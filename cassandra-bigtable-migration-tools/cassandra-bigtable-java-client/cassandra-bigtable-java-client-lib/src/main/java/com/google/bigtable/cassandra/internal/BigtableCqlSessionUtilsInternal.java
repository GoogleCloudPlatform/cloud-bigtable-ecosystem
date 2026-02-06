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
import com.google.bigtable.cassandra.BigtableCqlSessionFactory;
import java.io.IOException;
import java.io.UncheckedIOException;
import java.nio.file.Files;
import java.nio.file.Paths;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import io.netty.channel.unix.DomainSocketAddress;

/**
 * Internal use only. Use {@link BigtableCqlSessionFactory} instead.
 */
public final class BigtableCqlSessionUtilsInternal {

  private static final String BIGTABLE_CQL_SESSION_NAME = "BigtableCqlSession";
  private static final String BIGTABLE_PROXY_LOCAL_DATACENTER = "bigtable-proxy-local-datacenter";
  private static final Logger LOGGER = LoggerFactory.getLogger(
      BigtableCqlSessionUtilsInternal.class);

  private BigtableCqlSessionUtilsInternal() {
  }

  /**
   * Internal use only. Use {@link BigtableCqlSessionFactory#newSession()}  instead.
   */
  public static CqlSession newSession(BigtableCqlConfiguration bigtableCqlConfiguration) {
    if (OsUtils.isWindows()) {
      throw new IllegalStateException("Windows is currently not supported");
    }

    Proxy proxy = new ProxyFactory(bigtableCqlConfiguration).newProxy();
    BigtableCqlSessionNodeStateListener nodeStateListener = new BigtableCqlSessionNodeStateListener();

    try {
      DomainSocketAddress udsAddress = (DomainSocketAddress) proxy.start();
      LOGGER.info("Starting CqlSession...");
      CqlSession delegate = new UdsCqlSessionBuilder()
          .withApplicationName(BIGTABLE_CQL_SESSION_NAME)
          .addContactEndPoint(new UdsEndpoint(udsAddress))
          .withLocalDatacenter(BIGTABLE_PROXY_LOCAL_DATACENTER)
          .withNodeStateListener(nodeStateListener)
          .withKeyspace(bigtableCqlConfiguration.getDefaultKeyspace().orElse(null))
          .build();
      Files.delete(Paths.get(udsAddress.path()));

      BigtableCqlSession bigtableCqlSession = new BigtableCqlSession(delegate, proxy);
      proxy.setSession(bigtableCqlSession);
      nodeStateListener.initialize(bigtableCqlSession);

      LOGGER.info("Started CqlSession.");
      return bigtableCqlSession;
    } catch (IOException e) {
      proxy.stop();
      throw new UncheckedIOException("Failed to start CqlSession", e);
    } catch (Exception e) {
      proxy.stop();
      LOGGER.error("Failed to start CqlSession", e);
      throw e;
    }
  }

}
