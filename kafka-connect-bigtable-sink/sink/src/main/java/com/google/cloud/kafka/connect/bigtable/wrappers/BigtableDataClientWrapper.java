package com.google.cloud.kafka.connect.bigtable.wrappers;

import com.google.api.gax.batching.Batcher;
import com.google.api.gax.rpc.ServerStream;
import com.google.cloud.bigtable.data.v2.BigtableDataClient;
import com.google.cloud.bigtable.data.v2.models.ConditionalRowMutation;
import com.google.cloud.bigtable.data.v2.models.Query;
import com.google.cloud.bigtable.data.v2.models.Row;
import com.google.cloud.bigtable.data.v2.models.RowMutation;
import com.google.cloud.bigtable.data.v2.models.RowMutationEntry;
import javax.annotation.Nonnull;

public class BigtableDataClientWrapper implements BigtableDataClientInterface {

  private final BigtableDataClient dataClient;

  public BigtableDataClientWrapper(BigtableDataClient dataClient) {
    this.dataClient = dataClient;
  }

  @Override
  public Boolean checkAndMutateRow(ConditionalRowMutation mutation) {
    return dataClient.checkAndMutateRow(mutation);
  }

  @Override
  public Batcher<RowMutationEntry, Void> newBulkMutationBatcher(@Nonnull String tableId) {
    return dataClient.newBulkMutationBatcher(tableId);
  }

  @Override
  public Row readRow(String tableId, String rowKey) {
    return dataClient.readRow(tableId, rowKey);
  }

  @Override
  public void mutateRow(RowMutation rowMutation) {
    dataClient.mutateRow(rowMutation);
  }

  @Override
  public ServerStream<Row> readRows(Query query) {
    return dataClient.readRows(query);
  }

  @Override
  public void close() {
    dataClient.close();
  }
}
