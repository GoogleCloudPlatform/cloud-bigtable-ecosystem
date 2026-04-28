/*
 * Copyright 2025 Google LLC
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
package com.google.cloud.aerospike;

import com.google.cloud.aerospike.config.Config;
import com.google.cloud.aerospike.exception.InvalidMessageException;
import com.google.cloud.aerospike.mapping.SinkMutationBuilder;
import java.util.Base64;
import java.util.List;
import java.util.Map;
import org.apache.kafka.common.InvalidRecordException;
import org.apache.kafka.common.config.ConfigDef;
import org.apache.kafka.connect.connector.ConnectRecord;
import org.apache.kafka.connect.data.Schema;
import org.apache.kafka.connect.data.SchemaBuilder;
import org.apache.kafka.connect.data.Struct;
import org.apache.kafka.connect.transforms.Transformation;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * Transform a {@link org.apache.kafka.connect.json.JsonConverter}-deserialized Aerospike Connect <a
 * href="https://aerospike.com/docs/connectors/streaming/kafka/outbound/formats/json-serialization-format">
 * JSON-formatted XDR message</a> into {@see
 * com.google.cloud.kafka.connect.bigtable.BigtableSinkConnector} input to stream changes from
 * Aerospike into Cloud Bigtable.
 *
 * <p>It is a standard Kafka Connect Single Message Transformation, so when it (or the deserializer)
 * throws an exception:
 *
 * <ul>
 *   <li>if DLQ is configured for the sink, the message is sent to DLQ with information about the
 *       exception attached in the message's headers,
 *   <li>otherwise, the sink task stops and requires manual intervention.
 * </ul>
 */
public class MapAerospikeConnectJsonToBigtableSinkInput<R extends ConnectRecord<R>>
    implements Transformation<R> {
  public static final String TIMESTAMP_FIELD = "lut";
  public static final String TYPE_FIELD = "msg";
  public static final String BINS_FIELD = "bins";
  public static final String BIN_NAME_FIELD = "name";
  public static final String BIN_TYPE_FIELD = "type";
  public static final String BIN_VALUE_FIELD = "value";
  public static final String KEY_FIELD = "key";
  public static final int KEY_FIELD_DIGEST_IDX = 2;
  public static final int DIGEST_LENGTH = 20;
  private static final Logger log =
      LoggerFactory.getLogger(MapAerospikeConnectJsonToBigtableSinkInput.class);

  private Config config;

  public enum MessageType {
    WRITE,
    DELETE,
  }

  public enum ValueType {
    STR,
    BOOL,
    INT,
    FLOAT,
    BLOB,
    LIST,
    MAP,
    GEOJSON,
  }

  @Override
  public ConfigDef config() {
    return Config.getDefinition();
  }

  @Override
  public void configure(Map<String, ?> configs) {
    this.config = new Config(configs);
  }

  @Override
  public void close() {}

  /**
   * The transformation function.
   *
   * @param record a {@link org.apache.kafka.connect.json.JsonConverter}-deserialized Aerospike
   *     Connect JSON-formatted XDR message
   * @return {@code null} if the input message has last update timestamp lower than value configured
   *     by {@link Config#START_LUT_MILLIS}, an {@link R} mapped to a {@see
   *     com.google.cloud.kafka.connect.bigtable.BigtableSinkConnector} input equivalent to
   *     Aerospike values from {@code record}.
   * @throws InvalidMessageException if the message is invalid.
   */
  @Override
  public R apply(R record) {
    Object value = record.value();

    if (!(value instanceof Map<?, ?> msg)) {
      throw new InvalidMessageException("Message is not a JSON object.");
    }
    byte[] digest = getDigest(msg);
    long timestamp = getTimestamp(msg);
    Long minimalAllowedTimestamp = config.getLong(Config.START_LUT_MILLIS);
    if (timestamp < minimalAllowedTimestamp) {
      log.trace(
          "Dropping an XDR record with (base64-encoded) digest '{}' due to a too low LUT of {},"
              + " which is lower than minimal LUT of {}",
          Base64.getEncoder().encodeToString(digest),
          timestamp,
          minimalAllowedTimestamp);
      return null;
    }

    MessageType messageType = getMessageType(msg);

    return switch (messageType) {
      case WRITE -> applyWrite(record, msg, digest, timestamp);
      case DELETE -> applyDelete(record, digest, timestamp);
    };
  }

  private static <T extends ConnectRecord<T>> T newRecord(
      ConnectRecord<T> original, byte[] key, Schema schema, Object value, long timestamp) {
    return original.newRecord(
        original.topic(),
        original.kafkaPartition(),
        Schema.BYTES_SCHEMA,
        key,
        schema,
        value,
        timestamp,
        original.headers());
  }

  private static <T extends ConnectRecord<T>> T applyDelete(
      ConnectRecord<T> original, byte[] digest, long timestamp) {
    Schema valueSchema = SchemaBuilder.struct().optional().build();
    return newRecord(original, digest, valueSchema, null, timestamp);
  }

  private static <T extends ConnectRecord<T>> T applyWrite(
      ConnectRecord<T> original, Map<?, ?> msg, byte[] digest, long timestamp) {
    List<?> bins =
        getAndCastMapValue(
            msg,
            BINS_FIELD,
            List.class,
            "Field '" + BINS_FIELD + "' is missing.",
            "Field " + "'" + BINS_FIELD + "' is not a JSON array.");

    SinkMutationBuilder smb = new SinkMutationBuilder();
    for (Object o : bins) {
      addBinValue(smb, o);
    }

    Struct value = smb.getStruct();
    return newRecord(original, digest, value.schema(), value, timestamp);
  }

  private static void addBinValue(SinkMutationBuilder smb, Object binObject) {
    if (!(binObject instanceof Map<?, ?> bin)) {
      throw new InvalidMessageException("Field '" + BINS_FIELD + "' element is not a JSON object.");
    }
    Object binValue = bin.get(BIN_VALUE_FIELD);
    String binName =
        getAndCastMapValue(
            bin,
            BIN_NAME_FIELD,
            String.class,
            "Some bin is missing '" + BIN_NAME_FIELD + "' field.",
            "Some bin's '" + BIN_NAME_FIELD + "' is not a string.");
    String binTypeString =
        getAndCastMapValue(
            bin,
            BIN_TYPE_FIELD,
            String.class,
            "Some bin is missing '" + BIN_TYPE_FIELD + "' field.",
            "Some bin's '" + BIN_TYPE_FIELD + "' is not a string.");
    ValueType valueType = getEnum(ValueType.class, binTypeString);

    try {
      if (binValue == null) {
        throw new InvalidMessageException("'null' bin values are not supported.");
      }
      switch (valueType) {
        case STR -> smb.addValue(binName, (String) binValue);
        case BOOL -> smb.addValue(binName, (Boolean) binValue);
        case INT -> smb.addValue(binName, ((Number) binValue).longValue());
        case FLOAT -> smb.addValue(binName, ((Number) binValue).doubleValue());
        case BLOB -> smb.addValue(binName, Base64.getDecoder().decode((String) binValue));
        case LIST -> smb.addValue(binName, (List<Object>) binValue);
        case MAP -> smb.addValue(binName, (Map<Object, Object>) binValue);
        case GEOJSON ->
            throw new InvalidMessageException(
                "Value type '" + ValueType.GEOJSON + "' is not supported.");
      }
    } catch (ClassCastException e) {
      throw new InvalidMessageException(
          "Unexpected value type '"
              + binValue.getClass().getName()
              + "' encountered for type hint '"
              + valueType
              + "'.",
          e);
    }
  }

  private static long getTimestamp(Map<?, ?> msg) {
    return getAndCastMapValue(
            msg,
            TIMESTAMP_FIELD,
            Number.class,
            "Field '" + TIMESTAMP_FIELD + "' is missing.",
            "Field " + "'" + TIMESTAMP_FIELD + "' is not numeric.")
        .longValue();
  }

  private static MessageType getMessageType(Map<?, ?> msg) {
    String messageType =
        getAndCastMapValue(
            msg,
            TYPE_FIELD,
            String.class,
            "Field '" + TYPE_FIELD + "' is missing.",
            "Field " + "'" + TYPE_FIELD + "' is not a string.");
    return getEnum(MessageType.class, messageType);
  }

  private static byte[] getDigest(Map<?, ?> msg) {
    List<?> keyArray =
        getAndCastMapValue(
            msg,
            KEY_FIELD,
            List.class,
            "Field '" + KEY_FIELD + "' is missing.",
            "Field '" + KEY_FIELD + "' is not a JSON array.");
    if (keyArray.size() != 4) {
      throw new InvalidMessageException(
          "Field '" + KEY_FIELD + "' does not have exactly 4 fields.");
    }

    Object maybeDigestB64 = keyArray.get(KEY_FIELD_DIGEST_IDX);
    try {
      if (maybeDigestB64 instanceof String digestB64) {
        byte[] digest = Base64.getDecoder().decode(digestB64);
        if (digest.length != DIGEST_LENGTH) {
          throw new IllegalArgumentException();
        }
        return digest;
      } else {
        throw new IllegalArgumentException();
      }
    } catch (IllegalArgumentException e) {
      throw new InvalidRecordException(
          "Third element of '" + KEY_FIELD + "' is not a valid digest.");
    }
  }

  private static <T> T getAndCastMapValue(
      Map<?, ?> map,
      Object key,
      Class<T> klass,
      String missingErrorMessage,
      String castErrorMessage) {
    Object o = map.get(key);
    if (o == null) {
      throw new InvalidMessageException(missingErrorMessage);
    }
    try {
      return klass.cast(o);
    } catch (ClassCastException e) {
      throw new InvalidMessageException(castErrorMessage, e);
    }
  }

  private static <T extends Enum<T>> T getEnum(Class<T> cls, String s) {
    try {
      return Enum.valueOf(cls, s.toUpperCase());
    } catch (NullPointerException | IllegalArgumentException e) {
      throw new InvalidMessageException("Invalid " + cls.getName() + " value: '" + s + "'.", e);
    }
  }
}
