package com.google.cloud.kafka.connect.bigtable.writers;

import com.google.cloud.bigtable.data.v2.BigtableDataClient;
import com.google.cloud.kafka.connect.bigtable.config.BigtableSinkTaskConfig;

public class BigtableWriterFactory {

  public static BigtableWriter GetWriter(BigtableSinkTaskConfig config,
      BigtableDataClient bigtableDataClient) {

    switch (config.getInsertMode()) {
      case INSERT:
        return new BigtableInsertWriter(bigtableDataClient);
      case UPSERT:
        return new BigtableUpsertWriter(new BatchManager(bigtableDataClient));
      default:
        throw new IllegalStateException("Unexpected value: " + config.getInsertMode());
    }
  }
}
