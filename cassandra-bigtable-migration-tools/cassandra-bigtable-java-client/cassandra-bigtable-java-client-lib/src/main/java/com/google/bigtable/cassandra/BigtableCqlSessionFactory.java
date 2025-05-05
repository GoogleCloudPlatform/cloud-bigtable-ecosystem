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

import com.datastax.oss.driver.api.core.CqlSession;
import java.io.IOException;
import java.io.UncheckedIOException;
import java.net.InetSocketAddress;
import java.net.SocketAddress;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

public class BigtableCqlSessionFactory {

  private static final String BIGTABLE_CQLSESSION_NAME = "BigtableCqlSession";
  private static final String BIGTABLE_PROXY_LOCAL_DATACENTER = "bigtable-proxy-local-datacenter";
  private static final Logger LOGGER = LoggerFactory.getLogger(BigtableCqlSessionFactory.class);

  private final BigtableCqlConfiguration bigtableCqlConfiguration;

  public BigtableCqlSessionFactory(BigtableCqlConfiguration bigtableCqlConfiguration) {
    this.bigtableCqlConfiguration = bigtableCqlConfiguration;
  }

  public CqlSession newSession() {

    Proxy proxy = new ProxyFactory(bigtableCqlConfiguration).newProxy();

    try {
      SocketAddress address = proxy.start();

      LOGGER.info("Building CqlSession...");
      CqlSession delegate = CqlSession.builder()
          .withApplicationName(BIGTABLE_CQLSESSION_NAME)
          .addContactPoint((InetSocketAddress) address) // Connect to the proxy
          .withLocalDatacenter(BIGTABLE_PROXY_LOCAL_DATACENTER)
          .build();
      LOGGER.info("Built CqlSession");

      return new BigtableCqlSession(delegate, proxy);
    } catch (IOException e) {
      // If an exception occurs, make sure to stop the proxy if it was started.
      proxy.stop();
      throw new UncheckedIOException("Failed to build BigtableCqlSession", e);
    } catch (Exception e) {
      proxy.stop();
      LOGGER.error("Failed to build BigtableCqlSession", e);
      throw e;
    }
  }

}