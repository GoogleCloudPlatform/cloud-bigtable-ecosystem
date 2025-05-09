package com.google.cloud.kafka.connect.bigtable.wrappers;

import com.google.api.gax.batching.Batcher;
import com.google.api.gax.rpc.ServerStream;
import com.google.cloud.bigtable.data.v2.models.ConditionalRowMutation;
import com.google.cloud.bigtable.data.v2.models.Query;
import com.google.cloud.bigtable.data.v2.models.Row;
import com.google.cloud.bigtable.data.v2.models.RowMutation;
import com.google.cloud.bigtable.data.v2.models.RowMutationEntry;
import javax.annotation.Nonnull;

/**
 * This is an interface for com.google.cloud.bigtable.data.v2.BigtableDataClient which makes mocking
 * easier.
 */
public interface BigtableDataClientInterface {

  Boolean checkAndMutateRow(ConditionalRowMutation mutation);

  Batcher<RowMutationEntry, Void> newBulkMutationBatcher(@Nonnull String tableId);

  Row readRow(String tableId, String rowKey);

  void mutateRow(RowMutation rowMutation);

  ServerStream<Row> readRows(Query query);

  void close();
}
