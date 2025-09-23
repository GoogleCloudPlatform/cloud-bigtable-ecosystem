package com.google.cloud.kafka.connect.bigtable;

import static com.google.cloud.kafka.connect.bigtable.utils.Utils.getTimestampMicros;

import com.google.cloud.bigtable.data.v2.models.TableId;
import com.google.cloud.kafka.connect.bigtable.config.BigtableSinkTaskConfig;
import com.google.cloud.kafka.connect.bigtable.config.ConfigInterpolation;
import com.google.cloud.kafka.connect.bigtable.mapping.MutationData;
import com.google.cloud.kafka.connect.bigtable.mapping.MutationDataBuilder;
import com.google.common.annotations.VisibleForTesting;
import com.google.protobuf.ByteString;
import java.util.Collection;
import java.util.LinkedList;
import java.util.Map;
import java.util.Optional;
import org.apache.kafka.connect.data.SchemaAndValue;
import org.apache.kafka.connect.errors.DataException;
import org.apache.kafka.connect.sink.SinkRecord;

public class RecordPreparer {

  private final String tableNameFormat;

  public RecordPreparer(String tableNameFormat) {
    this.tableNameFormat = tableNameFormat;
  }

  /**
   * Generate mutation for a single input record.
   *
   * @param record Input record.
   * @return {@link Optional#empty()} if the input record requires no write to Cloud Bigtable,
   *     {@link Optional} containing mutation that it needs to be written to Cloud Bigtable
   *     otherwise.
   */
  @VisibleForTesting
  public Optional<MutationData> createRecordMutationData(SinkRecord record) {
    TableId recordTableId = getTableName(record);
    SchemaAndValue kafkaKey = new SchemaAndValue(record.keySchema(), record.key());
    ByteString rowKey = ByteString.copyFrom(keyMapper.getKey(kafkaKey));
    if (rowKey.isEmpty()) {
      throw new DataException(
          "The record's key converts into an illegal empty Cloud Bigtable row key.");
    }
    SchemaAndValue kafkaValue = new SchemaAndValue(record.valueSchema(), record.value());
    long timestamp = getTimestampMicros(record);
    MutationDataBuilder mutationDataBuilder =
        valueMapper.getRecordMutationDataBuilder(kafkaValue, record.topic(), timestamp);

    return mutationDataBuilder.maybeBuild(recordTableId, rowKey, record);
  }

  /**
   * Get table name the input record's mutation will be written to.
   *
   * @param record Input record.
   * @return Cloud Bigtable table name the input record's mutation will be written to.
   */
  @VisibleForTesting
  TableId getTableName(SinkRecord record) {
    return TableId.of(ConfigInterpolation.replace(tableNameFormat, record.topic()));
  }
}
