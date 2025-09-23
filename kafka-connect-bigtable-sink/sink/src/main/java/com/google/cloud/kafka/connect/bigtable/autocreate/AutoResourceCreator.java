package com.google.cloud.kafka.connect.bigtable.autocreate;

import com.google.cloud.kafka.connect.bigtable.mapping.MutationData;
import java.util.Collection;
import java.util.Map;
import org.apache.kafka.connect.sink.SinkRecord;

public interface AutoResourceCreator {

  Collection<MutationData> CreateResources(Collection<MutationData> mutations);

  void Close();
}
