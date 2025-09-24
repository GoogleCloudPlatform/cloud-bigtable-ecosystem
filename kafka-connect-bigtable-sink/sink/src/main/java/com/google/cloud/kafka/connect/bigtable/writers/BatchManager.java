package com.google.cloud.kafka.connect.bigtable.writers;

import com.google.api.core.ApiFuture;
import com.google.api.core.ApiFutures;
import com.google.api.gax.batching.Batcher;
import com.google.cloud.bigtable.data.v2.BigtableDataClient;
import com.google.cloud.bigtable.data.v2.models.RowMutationEntry;
import com.google.cloud.bigtable.data.v2.models.TableId;
import com.google.cloud.kafka.connect.bigtable.mapping.MutationData;
import com.google.cloud.kafka.connect.bigtable.utils.Utils;
import java.util.HashMap;
import java.util.Map;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.ExecutionException;
import java.util.stream.Collectors;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

public class BatchManager {
  private final Logger logger = LoggerFactory.getLogger(BatchManager.class);

  protected final Map<TableId, Batcher<RowMutationEntry, Void>> batchers;
  private final BigtableDataClient bigtableData;

  public BatchManager(BigtableDataClient bigtableData) {
    this.bigtableData = bigtableData;
    this.batchers = new HashMap<>();
  }

  public void flush() throws InterruptedException {
    logger.info("Flushing all batches...");
    // asynchronously trigger all records to send
    for (Batcher<RowMutationEntry, Void> value : batchers.values()) {
      value.sendOutstanding();
    }
    // synchronously wait until all records are flushed
    for (Batcher<RowMutationEntry, Void> value : batchers.values()) {
      value.flush();
    }

    logger.info("Flushed all batches.");
  }

  public void close() throws ExecutionException, InterruptedException {
    try {
      Iterable<ApiFuture<Void>> closeFutures =
          batchers.values().stream().map(Batcher::closeAsync).collect(Collectors.toList());
      ApiFutures.allAsList(closeFutures).get();
    } finally {
      batchers.clear();
    }
    bigtableData.close();
  }

  private Batcher<RowMutationEntry, Void> getBatcher(TableId tableId){
    return
        batchers.computeIfAbsent(tableId, bigtableData::newBulkMutationBatcher);
  }

  public CompletableFuture<Void> put(MutationData mutationData) {
    var batcher = getBatcher(mutationData.getTargetTable());
    ApiFuture<Void> fut = batcher.add(mutationData.getUpsertMutation());
    return Utils.toCompletableFuture(fut);
  }
}
