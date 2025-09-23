package com.google.cloud.kafka.connect.bigtable.writers;

import com.google.cloud.kafka.connect.bigtable.mapping.MutationData;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.Future;

public class BigtableUpsertWriter implements BigtableWriter {
  private final BatchManager batchManager;

  public BigtableUpsertWriter(BatchManager batchManager) {
    this.batchManager = batchManager;
  }

  @Override
  public void Flush() throws InterruptedException {
    batchManager.Flush();
  }

  @Override
  public void Close() throws ExecutionException, InterruptedException {
    batchManager.Close();
  }

  @Override
  public Future<Void> Put(MutationData mutation) {
    return batchManager.Put(mutation);
  }
}
