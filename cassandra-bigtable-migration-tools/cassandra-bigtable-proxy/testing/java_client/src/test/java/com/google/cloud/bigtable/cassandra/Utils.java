package com.google.cloud.bigtable.cassandra;

import com.datastax.oss.driver.api.core.CqlSession;
import com.datastax.oss.driver.api.core.CqlSessionBuilder;

import java.net.InetSocketAddress;

public class Utils {

  public static CqlSession createClient() {
    return createClient(null);
  }

  public static CqlSession createClient(String keyspace) {
    CqlSessionBuilder cqlSessionBuilder = CqlSession.builder()
        .addContactPoint(new InetSocketAddress("127.0.0.1", 9042))
        .withLocalDatacenter("datacenter1");

    if (keyspace != null) {
      cqlSessionBuilder
          .withKeyspace(keyspace);
    }

    return cqlSessionBuilder.build();
  }
}
