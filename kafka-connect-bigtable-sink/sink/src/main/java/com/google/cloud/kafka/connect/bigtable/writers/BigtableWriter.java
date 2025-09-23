package com.google.cloud.kafka.connect.bigtable.writers;

import com.google.cloud.kafka.connect.bigtable.mapping.MutationData;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.Future;

public interface BigtableWriter {

  void Flush() throws InterruptedException;

  void Close() throws ExecutionException, InterruptedException;

  Future<Void> Put(MutationData mutation) ;
}
