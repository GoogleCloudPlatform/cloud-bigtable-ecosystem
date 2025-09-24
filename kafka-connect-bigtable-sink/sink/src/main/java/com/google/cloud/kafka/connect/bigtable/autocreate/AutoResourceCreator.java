package com.google.cloud.kafka.connect.bigtable.autocreate;

import com.google.cloud.kafka.connect.bigtable.mapping.MutationData;
import com.google.cloud.kafka.connect.bigtable.utils.SinkResult;
import com.google.cloud.kafka.connect.bigtable.utils.SinkResult.FailureSinkResult;
import java.util.Collection;

public interface AutoResourceCreator extends AutoCloseable {

  Collection<FailureSinkResult<MutationData>> autoCreateTablesAndHandleErrors(
      Collection<MutationData> mutations);

  Collection<FailureSinkResult<MutationData>> autoCreateColumnFamiliesAndHandleErrors(
      Collection<MutationData> mutations);
}
