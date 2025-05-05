// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package com.google.bigtable.cassandra;

import com.google.bigtable.cassandra.BigtableCqlConfiguration;
import com.google.common.base.Preconditions;
import java.io.BufferedReader;
import java.io.BufferedWriter;
import java.io.FileWriter;
import java.io.IOException;
import java.io.InputStream;
import java.io.InputStreamReader;
import java.net.InetSocketAddress;
import java.net.ServerSocket;
import java.net.SocketAddress;
import java.net.SocketException;
import java.nio.charset.StandardCharsets;
import java.nio.file.FileSystems;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.attribute.PosixFilePermissions;
import java.time.Duration;
import java.util.ArrayList;
import java.util.List;
import java.util.Properties;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.TimeoutException;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * Internal use only.
 */
class ProxyImpl implements Proxy {

  private static final Duration PROXY_SHUTDOWN_TIMEOUT_SECS = Duration.ofSeconds(5);
  private static final Duration PROXY_STARTUP_TIMEOUT_SECS = Duration.ofSeconds(60);
  private static final String USER_AGENT_PREFIX = "java-cassandra-adapter/";
  private static final String PROJECT_PROPERTIES_FILENAME = "project.properties";
  private static final String LOOPBACK_ADDRESS = "127.0.0.1:%s";
  private static final String PROJECT_VERSION_PROPERTY = "lib.version";
  private static final String PROXY_STARTUP_LOG_MESSAGE = "Starting to serve on listener";
  private static final String DATA_CENTER_ENV_VAR = "DATA_CENTER";
  private static final String PROXY_BINARY_NAME = "cassandra-to-bigtable-proxy";
  private static final String PROXY_TEMP_DIR_PREFIX = "bigtable_cassandra_proxy_";
  private static final String WINDOWS_EXE_EXTENSION = ".exe";
  private static final String PROXY_CONFIG_FILENAME = "config.yaml";
  private static final int UNSPECIFIED_PORT = 0;
  private static final String PROXY_LOCAL_HOSTNAME = "localhost";
  private static final String PERMS_700 = "rwx------";
  private static final String BIGTABLE_PROXY_LOCAL_DATACENTER = "bigtable-proxy-local-datacenter";

  private static final Logger LOGGER = LoggerFactory.getLogger(ProxyImpl.class);

  private final BigtableCqlConfiguration bigtableCqlConfiguration;

  private Path proxyExecutablePath;
  private Path proxyConfigFilePath;
  private int proxyPort;
  private Process proxyProcess;
  private CompletableFuture<Boolean> isProxyProcessListening;
  private boolean isProxyStarted;
  private InetSocketAddress proxyAddress;

  /**
   * Internal use only.
   */
  protected ProxyImpl(BigtableCqlConfiguration bigtableCqlConfiguration) {
    Preconditions.checkNotNull(bigtableCqlConfiguration, "Valid BigtableCqlConfig must be supplied");
    this.bigtableCqlConfiguration = bigtableCqlConfiguration;
  }

  @Override
  public SocketAddress start() throws IOException {
    Preconditions.checkState(!isProxyStarted, "Proxy already started");

    LOGGER.debug("Setting up proxy");
    setupProxy();
    LOGGER.debug("Starting proxy");
    startProxy();
    isProxyStarted = true;
    return proxyAddress;
  }

  private void setupProxy() throws IOException {
    // TODO: handle potential race between finding a free port and another process taking it
    int proxyPort = findFreeProxyPort();

    // Create temp directory
    Path tempProxyDir = Files.createTempDirectory(PROXY_TEMP_DIR_PREFIX);
    LOGGER.debug("Proxy temp directory: " + tempProxyDir.toAbsolutePath());
    tempProxyDir.toFile().deleteOnExit();

    // Copy proxy binary to temp directory
    Path proxyExecutablePath = copyProxyBinaryToTempDir(tempProxyDir);

    // Write proxy config to temp directory
    ProxyConfig proxyConfig = ProxyConfigUtils.createProxyConfig(bigtableCqlConfiguration, proxyPort);
    Path proxyConfigFilePath = createProxyConfigFile(proxyConfig, tempProxyDir);

    this.proxyExecutablePath = proxyExecutablePath;
    this.proxyConfigFilePath = proxyConfigFilePath;
    this.proxyPort = proxyPort;
  }

  private int findFreeProxyPort() throws IOException {
    try (ServerSocket socket = new ServerSocket(UNSPECIFIED_PORT)) {
      if (socket.getLocalPort() == 0) {
        throw new IOException("Could not find a free port for the proxy");
      }
      return socket.getLocalPort();
    } catch (IOException e) {
      throw new IOException("Could not find a free port for the proxy", e);
    }
  }

  private Path createProxyConfigFile(ProxyConfig proxyConfig, Path tempProxyDir)
      throws IOException {
    // Serialize proxy config to YAML
    String proxyConfigContents = proxyConfig.toYaml();

    // Write proxy config to temp dir
    Path configTempPath = tempProxyDir.resolve(PROXY_CONFIG_FILENAME);
    Files.createFile(configTempPath);
    configTempPath.toFile().deleteOnExit();

    String proxyConfigFilePath = configTempPath.toAbsolutePath().toString();
    try (BufferedWriter writer = new BufferedWriter(new FileWriter(proxyConfigFilePath))) {
      writer.write(proxyConfigContents);
      LOGGER.debug("Successfully wrote proxy config to " + proxyConfigFilePath);
      return configTempPath;
    } catch (IOException e) {
      throw new IllegalStateException("Failed to write proxy config", e);
    }
  }

  private Path copyProxyBinaryToTempDir(Path tempProxyDir) throws IOException {
    // Copy proxy binary from resources
    try (InputStream inputStream = ProxyImpl.class.getClassLoader()
        .getResourceAsStream(PROXY_BINARY_NAME)) {
      if (inputStream == null) {
        throw new IOException("Proxy binary not found. Expected to find: " + PROXY_BINARY_NAME);
      }

      // Determine OS-specific binary name
      String os = System.getProperty("os.name").trim().toLowerCase();
      boolean isWindows = os.contains("win");
      String fullProxyBinaryName =
          isWindows ? PROXY_BINARY_NAME + WINDOWS_EXE_EXTENSION : PROXY_BINARY_NAME;

      // Copy proxy binary to temp directory
      Path targetProxyBinaryPath = tempProxyDir.resolve(fullProxyBinaryName);
      Files.copy(inputStream, targetProxyBinaryPath);
      targetProxyBinaryPath.toFile().deleteOnExit();

      // Set file permissions
      if (FileSystems.getDefault().supportedFileAttributeViews().contains("posix")) {
        // If Posix permissions are supported
        Files.setPosixFilePermissions(targetProxyBinaryPath, PosixFilePermissions.fromString(PERMS_700));
      } else {
        if (!targetProxyBinaryPath.toFile().setExecutable(true)) {
          throw new IOException("Failed to make proxy binary executable");
        }
      }

      return targetProxyBinaryPath;
    } catch (Exception e) {
      LOGGER.error("Error while deploying packaged proxy binary: " + e.getMessage());
      throw e;
    }
  }

  private void startProxy() throws IOException {
    ProcessBuilder pb = createProxyProcessBuilder();
    startProxyProcess(pb);
    isProxyProcessListening = new CompletableFuture<>();
    startThreadToConsumeProxyOutputStream();
    startThreadToConsumeProxyErrorStream();
    waitForProxyProcessToStartup();
    proxyAddress = new InetSocketAddress(PROXY_LOCAL_HOSTNAME, proxyPort);
  }

  @Override
  public void stop() {
    Preconditions.checkState(isProxyStarted, "Proxy not started");
    Preconditions.checkNotNull(proxyProcess, "Proxy not started");

    LOGGER.debug("Stopping proxy process...");
    try {
      // Try to stop the proxy with SIGINT. The Go process should trap this.
      proxyProcess.destroy();
      // Wait for graceful shutdown
      if (!proxyProcess.waitFor(PROXY_SHUTDOWN_TIMEOUT_SECS.getSeconds(), TimeUnit.SECONDS)) {
        LOGGER.warn("Proxy process did not stop gracefully, forcing termination.");
        proxyProcess.destroyForcibly(); // Forcefully terminate
      }
    } catch (InterruptedException e) {
      LOGGER.warn(
          "Interrupted while waiting for proxy to stop, forcing termination." + e.getMessage());
      proxyProcess.destroyForcibly();
      Thread.currentThread().interrupt(); // Restore interrupted status
    }
    isProxyStarted = false;
    LOGGER.debug("Proxy process stopped.");
  }

  private ProcessBuilder createProxyProcessBuilder() {
    // Build the command to start the proxy
    List<String> command = new ArrayList<>();
    command.add(proxyExecutablePath.toAbsolutePath().toString());

    // Add config file path argument
    if (proxyConfigFilePath == null || proxyConfigFilePath.toAbsolutePath().toString().trim().isEmpty()) {
      throw new IllegalArgumentException("No proxy configuration file specified");
    }
    command.add("-f");
    command.add(proxyConfigFilePath.toAbsolutePath().toString());

    // Add user agent argument
    command.add("-u");
    command.add(USER_AGENT_PREFIX + getLibraryVersion());

    // Add TCP bind port argument
    command.add("-t");
    command.add(LOOPBACK_ADDRESS);

    // Create ProcessBuilder
    ProcessBuilder pb = new ProcessBuilder(command);

    // Set Data Center
    pb.environment().put(DATA_CENTER_ENV_VAR, BIGTABLE_PROXY_LOCAL_DATACENTER);

    return pb;
  }

  private String getLibraryVersion() {
    final Properties properties = new Properties();
    try (InputStream resourceAsStream = ProxyImpl.class.getClassLoader()
        .getResourceAsStream(PROJECT_PROPERTIES_FILENAME)) {
      properties.load(resourceAsStream);
      return properties.getProperty(PROJECT_VERSION_PROPERTY);
    } catch (IOException e) {
      throw new IllegalStateException("Failed to get library version", e);
    }
  }

  private void startProxyProcess(ProcessBuilder pb) throws IOException {
    // Start the process
    LOGGER.debug("Starting proxy with command: " + pb.command());
    try {
      proxyProcess = pb.start();

      // Add shutdown hook to stop proxy
      Runtime.getRuntime().addShutdownHook(new Thread(() -> {
        if (proxyProcess != null && proxyProcess.isAlive()) {
          LOGGER.debug("Shutdown hook: Terminating proxy...");
          stop();
        }
      }));
    } catch (IOException e) {
      throw new IOException("Failed to start proxy process", e);
    }
  }

  private void startThreadToConsumeProxyOutputStream() {
    Thread outputConsumerThread = new Thread(() -> {
      Logger proxyProcessOutputLogger = LoggerFactory.getLogger("Proxy Output");
      try (BufferedReader reader = new BufferedReader(
          new InputStreamReader(proxyProcess.getInputStream(), StandardCharsets.UTF_8))) {
        String logLine;
        while ((logLine = reader.readLine()) != null) {
          proxyProcessOutputLogger.debug(logLine);
          if (logLine.contains(PROXY_STARTUP_LOG_MESSAGE)) {
            LOGGER.debug("Proxy started on port: " + proxyPort);
            isProxyProcessListening.complete(true);
          }
        }
      } catch (IOException e) {
        if (e instanceof SocketException && e.getMessage().contains("Socket closed")) {
          LOGGER.debug("Proxy process closed output stream");
        } else {
          LOGGER.error("Error reading proxy output: " + e.getMessage());
        }
        isProxyProcessListening.complete(false);
      }
    });
    outputConsumerThread.setDaemon(true);  // Make it a daemon thread
    outputConsumerThread.start();
  }

  private void startThreadToConsumeProxyErrorStream() {
    Thread errorOutputConsumerThread = new Thread(() -> {
      Logger proxyProcessErrorLogger = LoggerFactory.getLogger("Proxy Error");
      try (BufferedReader reader = new BufferedReader(
          new InputStreamReader(proxyProcess.getErrorStream(), StandardCharsets.UTF_8))) {
        String logLine;
        while ((logLine = reader.readLine()) != null) {
          proxyProcessErrorLogger.error(logLine);
        }
      } catch (IOException e) {
        if (e instanceof SocketException && e.getMessage().contains("Socket closed")) {
          LOGGER.debug("Proxy process closed error output stream");
        } else {
          LOGGER.error("Error reading proxy error output: " + e.getMessage());
        }
      }
    });
    errorOutputConsumerThread.setDaemon(true);  // Make it a daemon thread
    errorOutputConsumerThread.start();
  }

  private void waitForProxyProcessToStartup() {
    LOGGER.debug("Waiting for proxy process to start");
    try {
      boolean proxyStarted = isProxyProcessListening.get(PROXY_STARTUP_TIMEOUT_SECS.getSeconds(), TimeUnit.SECONDS);
      if (!proxyStarted) {
        throw new IllegalStateException("Proxy process failed to start");
      }
    } catch (ExecutionException | InterruptedException | TimeoutException e) {
      LOGGER.error("Exception while waiting for proxy process to start: " + e.getMessage());
      throw new RuntimeException(e);
    }
  }

}
