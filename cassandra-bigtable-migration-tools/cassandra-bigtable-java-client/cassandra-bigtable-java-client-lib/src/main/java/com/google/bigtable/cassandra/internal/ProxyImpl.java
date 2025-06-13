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

package com.google.bigtable.cassandra.internal;

import com.datastax.oss.driver.api.core.CqlSession;
import com.google.bigtable.cassandra.BigtableCqlConfiguration;
import com.google.common.base.Preconditions;
import io.netty.channel.unix.DomainSocketAddress;
import java.io.BufferedReader;
import java.io.BufferedWriter;
import java.io.IOException;
import java.io.InputStream;
import java.io.InputStreamReader;
import java.net.SocketAddress;
import java.net.SocketException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.attribute.PosixFilePermission;
import java.nio.file.attribute.PosixFilePermissions;
import java.time.Duration;
import java.util.ArrayList;
import java.util.List;
import java.util.Properties;
import java.util.Set;
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
  private static final String PROJECT_VERSION_PROPERTY = "lib.version";
  private static final String PROXY_STARTUP_LOG_MESSAGE = "Starting to serve on listener";
  private static final String DATA_CENTER_ENV_VAR = "DATA_CENTER";
  private static final String PROXY_BINARY_NAME = "cassandra-to-bigtable-proxy";
  private static final String PROXY_CONFIG_FILENAME = "config.yaml";
  private static final String UDS_FILENAME = "cassandra-proxy.sock";
  private static final Set<PosixFilePermission> PERMS_700 = PosixFilePermissions.fromString("rwx------");
  private static final Set<PosixFilePermission> PERMS_600 = PosixFilePermissions.fromString("rw-------");
  private static final String BIGTABLE_PROXY_LOCAL_DATACENTER = "bigtable-proxy-local-datacenter";
  private static final Logger LOGGER = LoggerFactory.getLogger(ProxyImpl.class);

  private final BigtableCqlConfiguration bigtableCqlConfiguration;

  private Path tempProxyDir;
  private Path proxyExecutablePath;
  private Path proxyConfigFilePath;
  private Process proxyProcess;
  private CompletableFuture<Boolean> isProxyProcessListening;
  private boolean isProxyStarted;
  private CqlSession cqlSession;
  private SocketAddress proxyAddress;

  /**
   * Internal use only.
   */
  protected ProxyImpl(BigtableCqlConfiguration bigtableCqlConfiguration) {
    Preconditions.checkNotNull(bigtableCqlConfiguration,
        "Valid BigtableCqlConfig must be supplied");
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
    tempProxyDir = Files.createTempDirectory(null, PosixFilePermissions.asFileAttribute(PERMS_700));
    tempProxyDir.toFile().deleteOnExit();

    Path proxyExecutablePath = copyProxyBinaryToTempDir(tempProxyDir);

    ProxyConfig proxyConfig = ProxyConfigUtils.createProxyConfig(bigtableCqlConfiguration);
    Path proxyConfigFilePath = createProxyConfigFile(proxyConfig, tempProxyDir);

    this.proxyExecutablePath = proxyExecutablePath;
    this.proxyConfigFilePath = proxyConfigFilePath;
  }

  private Path createProxyConfigFile(ProxyConfig proxyConfig, Path tempProxyDir)
      throws IOException {
    String proxyConfigContents = proxyConfig.toYaml();

    Path configTempPath = tempProxyDir.resolve(PROXY_CONFIG_FILENAME);
    Files.createFile(configTempPath, PosixFilePermissions.asFileAttribute(PERMS_600));
    configTempPath.toFile().deleteOnExit();

    try (BufferedWriter writer = Files.newBufferedWriter(configTempPath)) {
      writer.write(proxyConfigContents);
      LOGGER.debug("Successfully created proxy config");
      return configTempPath;
    } catch (IOException e) {
      throw new IllegalStateException("Failed to create proxy config", e);
    }
  }

  private Path copyProxyBinaryToTempDir(Path tempProxyDir) throws IOException {
    if (OsUtils.isWindows()) {
      throw new IllegalStateException("Windows is currently not supported");
    }

    try (InputStream inputStream = ProxyImpl.class.getClassLoader()
        .getResourceAsStream(PROXY_BINARY_NAME)) {
      if (inputStream == null) {
        throw new IOException("Proxy binary not found. Expected to find: " + PROXY_BINARY_NAME);
      }

      Path targetProxyBinaryPath = tempProxyDir.resolve(PROXY_BINARY_NAME);
      Files.copy(inputStream, targetProxyBinaryPath);
      Files.setPosixFilePermissions(targetProxyBinaryPath, PERMS_700);
      targetProxyBinaryPath.toFile().deleteOnExit();

      return targetProxyBinaryPath;
    } catch (Exception e) {
      LOGGER.error("Error while deploying packaged proxy binary: " + e.getMessage());
      throw e;
    }
  }

  private void startProxy() throws IOException {
    Path udsPath = tempProxyDir.resolve(UDS_FILENAME);
    if (Files.exists(udsPath)) {
      throw new IllegalStateException("UDS path exists");
    }
    ProcessBuilder pb = createProxyProcessBuilder(udsPath.toString());
    startProxyProcess(pb);
    isProxyProcessListening = new CompletableFuture<>();
    startThreadToMonitorProxyProcess();
    startThreadToConsumeProxyOutputStream();
    startThreadToConsumeProxyErrorStream();
    waitForProxyProcessToStartup();
    if (!Files.isReadable(udsPath) || !Files.isWritable(udsPath)) {
      throw new IOException("Current user does not have read/write permissions to UDS");
    }
    proxyAddress = new DomainSocketAddress(udsPath.toString());
  }

  @Override
  public void stop() {
    Preconditions.checkState(isProxyStarted, "Proxy not started");
    Preconditions.checkNotNull(proxyProcess, "Proxy not started");

    LOGGER.debug("Stopping proxy process...");
    try {
      proxyProcess.destroy();
      if (!proxyProcess.waitFor(PROXY_SHUTDOWN_TIMEOUT_SECS.getSeconds(), TimeUnit.SECONDS)) {
        LOGGER.warn("Proxy process did not stop gracefully, forcing termination.");
        proxyProcess.destroyForcibly();
      }
    } catch (InterruptedException e) {
      LOGGER.warn(
          "Interrupted while waiting for proxy to stop, forcing termination." + e.getMessage());
      proxyProcess.destroyForcibly();
      Thread.currentThread().interrupt();
    }
    isProxyStarted = false;
    LOGGER.debug("Proxy process stopped.");
  }

  private ProcessBuilder createProxyProcessBuilder(String udsPath) {
    List<String> command = new ArrayList<>();
    command.add(proxyExecutablePath.toAbsolutePath().toString());

    if (proxyConfigFilePath == null || proxyConfigFilePath.toAbsolutePath().toString().trim()
        .isEmpty()) {
      throw new IllegalArgumentException("No proxy configuration file specified");
    }
    command.add("-f");
    command.add(proxyConfigFilePath.toAbsolutePath().toString());

    command.add("-u");
    command.add(USER_AGENT_PREFIX + getLibraryVersion());

    command.add("--use-unix-socket");
    command.add("--unix-socket-path");
    command.add(udsPath);
    command.add("--client-pid");
    command.add(OsUtils.getPid());
    command.add("--client-uid");
    command.add(OsUtils.getUid());

    ProcessBuilder pb = new ProcessBuilder(command);

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
    LOGGER.debug("Starting proxy");
    try {
      proxyProcess = pb.start();

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
            LOGGER.debug("Proxy started.");
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
    outputConsumerThread.setDaemon(true);
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
    errorOutputConsumerThread.setDaemon(true);
    errorOutputConsumerThread.start();
  }

  private void startThreadToMonitorProxyProcess() {
    Thread monitorProcessThread = new Thread(() -> {
      Logger proxyMonitorErrorLogger = LoggerFactory.getLogger("Proxy monitor");
      try {
        int exitCode = proxyProcess.waitFor();
        if (exitCode != 0) {
          proxyMonitorErrorLogger.warn("Proxy exited with error");
          if (cqlSession != null && !cqlSession.isClosed()) {
            proxyMonitorErrorLogger.warn("Closing CqlSession");
            cqlSession.forceCloseAsync();
          }
        }
      } catch (InterruptedException e) {
        throw new RuntimeException(e);
      }
    });
    monitorProcessThread.setDaemon(true);
    monitorProcessThread.start();
  }

  private void waitForProxyProcessToStartup() {
    LOGGER.debug("Waiting for proxy process to start");
    try {
      boolean proxyStarted = isProxyProcessListening.get(PROXY_STARTUP_TIMEOUT_SECS.getSeconds(),
          TimeUnit.SECONDS);
      if (!proxyStarted) {
        throw new IllegalStateException("Proxy process failed to start");
      }
    } catch (ExecutionException | InterruptedException | TimeoutException e) {
      LOGGER.error("Exception while waiting for proxy process to start: " + e.getMessage());
      throw new RuntimeException(e);
    }
  }

  public void setSession(CqlSession cqlSession) {
    this.cqlSession = cqlSession;
  }

}
