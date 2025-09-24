package com.google.cloud.kafka.connect.bigtable.writers;

import com.google.cloud.kafka.connect.bigtable.mapping.MutationData;
import com.google.cloud.kafka.connect.bigtable.utils.SinkResult;
import java.util.concurrent.CompletableFuture;

public interface BigtableWriter extends AutoCloseable {

  SinkResult<CompletableFuture<Void>> put(MutationData mutation);

  void flush() throws InterruptedException;
}
