package com.google.cloud.kafka.connect.bigtable.writers;

import com.google.api.gax.rpc.ApiException;
import com.google.cloud.bigtable.data.v2.BigtableDataClient;
import com.google.cloud.bigtable.data.v2.models.ConditionalRowMutation;
import com.google.cloud.bigtable.data.v2.models.Filters;
import com.google.cloud.kafka.connect.bigtable.mapping.MutationData;
import com.google.cloud.kafka.connect.bigtable.utils.SinkResult;
import java.util.concurrent.CompletableFuture;
import org.apache.kafka.connect.errors.ConnectException;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

public class BigtableInsertWriter implements BigtableWriter {

  private final Logger logger = LoggerFactory.getLogger(BigtableInsertWriter.class);

  private final BigtableDataClient bigtableData;

  public BigtableInsertWriter(BigtableDataClient bigtableData) {
    this.bigtableData = bigtableData;
  }


  @Override
  public void flush() {
// nothing to do because Put synchronously applies mutations
  }

  @Override
  public void close() {
    bigtableData.close();
  }

  public SinkResult<CompletableFuture<Void>> put(MutationData mutation) {
    ConditionalRowMutation insert =
        // We want to perform the mutation if and only if the row does not already exist.
        ConditionalRowMutation.create(
                mutation.getTargetTable(), mutation.getRowKey())
            // We first check if any cell of this row exists...
            .condition(Filters.FILTERS.pass())
            // ... and perform the mutation only if no cell exists.
            .otherwise(mutation.getInsertMutation());
    try {
      var insertSuccessful = !bigtableData.checkAndMutateRow(insert);
      if (insertSuccessful) {
        return SinkResult.success(mutation.getRecord(), CompletableFuture.completedFuture(null));
      } else {
        return SinkResult.success(mutation.getRecord(), CompletableFuture.failedFuture(
            new ConnectException("Insert failed since the row already existed.")));
      }
    } catch (ApiException e) {
      return SinkResult.success(mutation.getRecord(), CompletableFuture.failedFuture(e));
    }
  }
}
