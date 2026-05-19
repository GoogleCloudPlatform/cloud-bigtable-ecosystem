/*
 * Copyright 2026 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package com.google.cloud.kafka.connect.bigtable.transformations;

import com.google.cloud.kafka.connect.bigtable.util.ConfigUtils;
import com.google.cloud.kafka.connect.bigtable.util.SchemaParsingUtils;
import com.google.common.annotations.VisibleForTesting;
import java.util.Map;
import org.apache.kafka.common.config.ConfigDef;
import org.apache.kafka.connect.connector.ConnectRecord;
import org.apache.kafka.connect.data.SchemaAndValue;
import org.apache.kafka.connect.transforms.Transformation;
import org.apache.kafka.connect.transforms.util.SimpleConfig;

public abstract class ExtractTimestamp<R extends ConnectRecord<R>> implements Transformation<R> {

  public static final String TIMESTAMP_FIELD_CONFIG = "timestamp.field";
  public static final String TIMESTAMP_FIELD_PRECISION_CONFIG = "timestamp.field.precision";

  public static final ConfigDef CONFIG_DEF =
      new ConfigDef()
          .define(
              TIMESTAMP_FIELD_CONFIG,
              ConfigDef.Type.STRING,
              ConfigDef.Importance.HIGH,
              "The name of the timestamp field. Non-root fields can be referenced by specifying the"
                  + " field path, with periods separating each field. If the field cannot be found,"
                  + " or if the value is null, the message is failed. The field may be a numeric,"
                  + " string or date type.")
          .define(
              TIMESTAMP_FIELD_PRECISION_CONFIG,
              ConfigDef.Type.STRING,
              TimestampPrecision.MILLIS.name(),
              ConfigUtils.enumValidator(TimestampPrecision.values()),
              ConfigDef.Importance.HIGH,
              "The precision of the timestamp field. Defaults to MILLIS. This only effects the"
                  + " output for numeric fields. Ignore this config if your field is a date type."
                  + " Supported values are NANOS, MICROS, MILLIS and SECONDS. Use the value that"
                  + " matches the field's precision. Example: if your field has epoch millisecond"
                  + " values, use the MILLIS config value.");

  private String[] fieldPath;
  private TimestampPrecision timestampPrecision;

  @Override
  public void configure(Map<String, ?> configs) {
    SimpleConfig config = new SimpleConfig(CONFIG_DEF, configs);
    this.fieldPath = config.getString(TIMESTAMP_FIELD_CONFIG).split("\\.");
    this.timestampPrecision =
        TimestampPrecision.valueOf(
            config.getString(TIMESTAMP_FIELD_PRECISION_CONFIG).toUpperCase());
  }

  @Override
  public R apply(R record) {
    SchemaAndValue timestampField =
        SchemaParsingUtils.extractField(getOperatingValue(record), fieldPath);
    long parsedTimestampMillis =
        ExtractTimestamp.parseTimestampToMillis(timestampField, timestampPrecision);
    return record.newRecord(
        record.topic(),
        record.kafkaPartition(),
        record.keySchema(),
        record.key(),
        record.valueSchema(),
        record.value(),
        parsedTimestampMillis);
  }

  @Override
  public ConfigDef config() {
    return CONFIG_DEF;
  }

  @Override
  public void close() {}

  protected abstract SchemaAndValue getOperatingValue(R record);

  @VisibleForTesting
  static long parseTimestampToMillis(SchemaAndValue value, TimestampPrecision timestampPrecision) {
    if (value == null || value.value() == null) {
      throw new IllegalArgumentException("Cannot parse timestamp value of null");
    }

    Object rawValue = value.value();

    // Handle native Connect logical Timestamp/Date objects directly
    if (rawValue instanceof java.util.Date) {
      return ((java.util.Date) rawValue).getTime();
    }
    if (rawValue instanceof java.time.Instant) {
      return ((java.time.Instant) rawValue).toEpochMilli();
    }

    // Extract the epoch number safely (supports both Schema-based and Schemaless records)
    long epochValue;
    if (rawValue instanceof Number) {
      epochValue = ((Number) rawValue).longValue();
    } else if (rawValue instanceof String) {
      String strVal = ((String) rawValue).trim();
      epochValue = Long.parseLong(strVal);
    } else {
      throw new IllegalArgumentException(
          "Unsupported timestamp payload type: " + rawValue.getClass().getName());
    }

    // Resolve to target milliseconds resolution
    switch (timestampPrecision) {
      case NANOS:
        return epochValue / 1_000_000L;
      case MICROS:
        return epochValue / 1000L;
      case MILLIS:
        return epochValue;
      case SECONDS:
        return epochValue * 1000L;
      default:
        throw new IllegalStateException("Unexpected timestamp precision: " + timestampPrecision);
    }
  }

  // Boilerplate for Key/Value distinct implementations
  public static class Key<R extends ConnectRecord<R>> extends ExtractTimestamp<R> {

    @Override
    protected SchemaAndValue getOperatingValue(R record) {
      return new SchemaAndValue(record.keySchema(), record.key());
    }
  }

  public static class Value<R extends ConnectRecord<R>> extends ExtractTimestamp<R> {
    @Override
    protected SchemaAndValue getOperatingValue(R record) {
      return new SchemaAndValue(record.valueSchema(), record.value());
    }
  }
}
