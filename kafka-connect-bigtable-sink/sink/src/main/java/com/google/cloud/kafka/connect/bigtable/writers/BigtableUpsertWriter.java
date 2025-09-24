package com.google.cloud.kafka.connect.bigtable.writers;

import com.google.cloud.kafka.connect.bigtable.mapping.MutationData;
import com.google.cloud.kafka.connect.bigtable.utils.SinkResult;
import java.util.concurrent.CompletableFuture;

public class BigtableUpsertWriter implements BigtableWriter {
  private final BatchManager batchManager;

  public BigtableUpsertWriter(BatchManager batchManager) {
    this.batchManager = batchManager;
  }

  @Override
  public void flush() throws InterruptedException {
    batchManager.flush();
  }

  @Override
  public SinkResult<CompletableFuture<Void>> put(MutationData mutation) {
    return SinkResult.success(mutation.getRecord(), batchManager.put(mutation));
  }

  @Override
  public void close() throws Exception {
    batchManager.close();
  }
}
