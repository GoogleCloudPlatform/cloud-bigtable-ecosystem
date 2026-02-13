/*
 * Copyright 2024 Google LLC
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
package com.google.cloud.kafka.connect.bigtable.mapping;

import static com.google.cloud.kafka.connect.bigtable.mapping.LogicalTypeUtils.logIfLogicalTypeUnsupported;

import com.fasterxml.jackson.core.JsonGenerator;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.SerializerProvider;
import com.fasterxml.jackson.databind.module.SimpleModule;
import com.fasterxml.jackson.databind.ser.std.StdSerializer;
import com.google.cloud.bigtable.data.v2.models.Range;
import com.google.cloud.kafka.connect.bigtable.config.ConfigInterpolation;
import com.google.cloud.kafka.connect.bigtable.config.NullValueMode;
import com.google.common.annotations.VisibleForTesting;
import com.google.protobuf.ByteString;

import java.io.IOException;
import java.math.BigDecimal;
import java.nio.ByteBuffer;
import java.nio.charset.StandardCharsets;
import java.util.AbstractMap;
import java.util.Date;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.stream.Collectors;
import javax.annotation.Nonnull;

import com.google.protobuf.MapEntry;
import org.apache.kafka.common.utils.Utils;
import org.apache.kafka.connect.data.Decimal;
import org.apache.kafka.connect.data.Field;
import org.apache.kafka.connect.data.Schema;
import org.apache.kafka.connect.data.SchemaAndValue;
import org.apache.kafka.connect.data.Struct;
import org.apache.kafka.connect.data.Time;
import org.apache.kafka.connect.data.Timestamp;
import org.apache.kafka.connect.errors.DataException;

/**
 * A class responsible for converting Kafka {@link org.apache.kafka.connect.sink.SinkRecord
 * SinkRecord(s)} into Cloud Bigtable {@link com.google.cloud.bigtable.data.v2.models.Mutation
 * Mutation(s)}.
 */
public class ValueMapper {
  private static final int ROOT_LEVEL_ARRAY_INDEX_PADDING_WIDTH = 6;
  private static final String ROOT_LEVEL_ARRAY_INDEX_FORMAT = "%0" + ROOT_LEVEL_ARRAY_INDEX_PADDING_WIDTH + "d";
  public final String defaultColumnFamilyTemplate;
  public final ByteString defaultColumnQualifier;
  private final NullValueMode nullMode;
  private final boolean expandRootLevelArrays;
  private static final ObjectMapper jsonMapper = getJsonMapper();

  /**
   * The main constructor.
   *
   * @param defaultColumnFamily    Default column family as per {@link
   *                               com.google.cloud.kafka.connect.bigtable.config.BigtableSinkConfig#DEFAULT_COLUMN_FAMILY_CONFIG}.
   * @param defaultColumnQualifier Default column as per {@link
   *                               com.google.cloud.kafka.connect.bigtable.config.BigtableSinkConfig#ROW_KEY_DELIMITER_CONFIG}.
   */
  public ValueMapper(
      String defaultColumnFamily, String defaultColumnQualifier, @Nonnull NullValueMode nullMode, boolean expandRootLevelArrays) {
    this.defaultColumnFamilyTemplate =
        Utils.isBlank(defaultColumnFamily) ? null : defaultColumnFamily;
    this.defaultColumnQualifier =
        Utils.isBlank(defaultColumnQualifier)
            ? null
            : ByteString.copyFrom(defaultColumnQualifier.getBytes(StandardCharsets.UTF_8));
    this.nullMode = nullMode;
    this.expandRootLevelArrays = expandRootLevelArrays;
  }

  /**
   * Creates a {@link MutationDataBuilder} that can be used to create a {@link MutationData}
   * representing the input Kafka Connect value as Cloud Bigtable mutations that need to be applied.
   *
   * @param kafkaValueAndSchema The value to be converted into Cloud Bigtable {@link
   *                            com.google.cloud.bigtable.data.v2.models.Mutation Mutation(s)} and its optional {@link
   *                            Schema}.
   * @param topic               The name of Kafka topic this value originates from.
   * @param timestampMicros     The timestamp the mutations will be created at in microseconds.
   */
  public MutationDataBuilder getRecordMutationDataBuilder(
      SchemaAndValue kafkaValueAndSchema, String topic, long timestampMicros) {
    Object rootKafkaValue = kafkaValueAndSchema.value();
    Optional<Schema> rootKafkaSchema = Optional.ofNullable(kafkaValueAndSchema.schema());
    logIfLogicalTypeUnsupported(rootKafkaSchema);
    MutationDataBuilder mutationDataBuilder = createMutationDataBuilder();
    if (rootKafkaValue == null && nullMode == NullValueMode.IGNORE) {
      // Do nothing
    } else if (rootKafkaValue == null && nullMode == NullValueMode.DELETE) {
      mutationDataBuilder.deleteRow();
    } else if (rootKafkaValue instanceof Struct) {
      for (Map.Entry<Object, SchemaAndValue> field :
          getChildren((Struct) rootKafkaValue, rootKafkaSchema)) {
        String kafkaFieldName = field.getKey().toString();
        Object kafkaFieldValue = field.getValue().value();
        Optional<Schema> kafkaFieldSchema = Optional.ofNullable(field.getValue().schema());
        logIfLogicalTypeUnsupported(kafkaFieldSchema);
        if (kafkaFieldValue == null && nullMode == NullValueMode.IGNORE) {
          continue;
        } else if (kafkaFieldValue == null && nullMode == NullValueMode.DELETE) {
          mutationDataBuilder.deleteFamily(kafkaFieldName);
        } else if (expandRootLevelArrays && kafkaFieldValue instanceof List) {
          writeRootLevelArray(mutationDataBuilder, kafkaFieldName, kafkaFieldValue, timestampMicros);
        } else if (kafkaFieldValue instanceof Struct) {
          for (Map.Entry<Object, SchemaAndValue> subfield :
              getChildren((Struct) kafkaFieldValue, kafkaFieldSchema)) {
            ByteString kafkaSubfieldName =
                ByteString.copyFrom(subfield.getKey().toString().getBytes(StandardCharsets.UTF_8));
            Object kafkaSubfieldValue = subfield.getValue().value();
            Optional<Schema> kafkaSubfieldSchema =
                Optional.ofNullable(subfield.getValue().schema());
            logIfLogicalTypeUnsupported(kafkaSubfieldSchema);
            if (kafkaSubfieldValue == null && nullMode == NullValueMode.IGNORE) {
              continue;
            } else if (kafkaSubfieldValue == null && nullMode == NullValueMode.DELETE) {
              mutationDataBuilder.deleteCells(
                  kafkaFieldName,
                  kafkaSubfieldName,
                  Range.TimestampRange.create(0, timestampMicros));
            } else {
              mutationDataBuilder.setCell(
                  kafkaFieldName,
                  kafkaSubfieldName,
                  timestampMicros,
                  ByteString.copyFrom(serialize(kafkaSubfieldValue)));
            }
          }
        } else {
          if (defaultColumnFamilyTemplate != null) {
            mutationDataBuilder.setCell(
                getDefaultColumnFamily(topic),
                ByteString.copyFrom(kafkaFieldName.getBytes(StandardCharsets.UTF_8)),
                timestampMicros,
                ByteString.copyFrom(serialize(kafkaFieldValue)));
          }
        }
      }
    } else {
      if (defaultColumnFamilyTemplate != null && defaultColumnQualifier != null) {
        mutationDataBuilder.setCell(
            getDefaultColumnFamily(topic),
            defaultColumnQualifier,
            timestampMicros,
            ByteString.copyFrom(serialize(rootKafkaValue)));
      }
    }
    return mutationDataBuilder;
  }

  private void writeRootLevelArray(MutationDataBuilder mutationDataBuilder, String columnFamily, Object value, long timestampMicros) {
    List<?> listValue = (List<?>) value;
    // clear the previous value in case it has more elements than this one
    mutationDataBuilder.deleteFamily(columnFamily);
    for (int i = 0; i < listValue.size(); i++) {
      Object o = listValue.get(i);
      mutationDataBuilder.setCell(
          columnFamily,
          // pad element names with leading zeros for easier sorting: '000001'
          ByteString.copyFrom(String.format(ROOT_LEVEL_ARRAY_INDEX_FORMAT, i).getBytes(StandardCharsets.UTF_8)),
          timestampMicros,
          ByteString.copyFrom(serialize(o)));
    }
  }

  @VisibleForTesting
  // Method only needed for use in tests. It could be inlined otherwise.
  protected MutationDataBuilder createMutationDataBuilder() {
    return new MutationDataBuilder();
  }

  protected String getDefaultColumnFamily(String topic) {
    return ConfigInterpolation.replace(defaultColumnFamilyTemplate, topic);
  }

  /**
   * @param struct {@link Struct} whose children we want to list
   * @return {@link List} of pairs of field names and values (with optional schemas) of {@code
   * struct}'s fields.
   */
  private static List<Map.Entry<Object, SchemaAndValue>> getChildren(
      Struct struct, Optional<Schema> schema) {
    return struct.schema().fields().stream()
        .map(Field::name)
        .map(
            f ->
                new AbstractMap.SimpleImmutableEntry<>(
                    (Object) f,
                    new SchemaAndValue(
                        SchemaUtils.maybeExtractFieldSchema(schema, f).orElse(null),
                        struct.get(f))))
        .collect(Collectors.toList());
  }

  /**
   * @param value Input value.
   * @return Input value's serialization's bytes that will be written to Cloud Bigtable as a cell's
   * value.
   */
  private static byte[] serialize(Object value) {
    if (value == null) {
      return new byte[0];
    } else if (value instanceof byte[]) {
      return (byte[]) value;
    } else if (value instanceof ByteBuffer) {
      return serialize(((ByteBuffer) value).array());
    } else if (value instanceof Integer) {
      return ByteUtils.toBytes((Integer) value);
    } else if (value instanceof Long) {
      return ByteUtils.toBytes((Long) value);
    } else if (value instanceof Short) {
      return ByteUtils.toBytes((Short) value);
    } else if (value instanceof Byte) {
      return ByteUtils.toBytes((Byte) value);
    } else if (value instanceof Float) {
      return ByteUtils.toBytes((Float) value);
    } else if (value instanceof Double) {
      return ByteUtils.toBytes((Double) value);
    } else if (value instanceof Boolean) {
      return ByteUtils.toBytes((Boolean) value);
    } else if (value instanceof String) {
      return ByteUtils.toBytes((String) value);
    } else if (value instanceof Character) {
      return serialize(Character.toString((Character) value));
    } else if (value instanceof Date) {
      // Note that the value might have different Kafka Connect schema: Date, Time or Timestamp.
      return serialize(((Date) value).getTime());
    } else if (value instanceof BigDecimal) {
      return ByteUtils.toBytes((BigDecimal) value);
    } else if (value instanceof Map || value instanceof Struct || value instanceof List) {
      try {
        return jsonMapper.writeValueAsBytes(value);
      } catch (JsonProcessingException e) {
        throw new DataException("Failed to deserialize a(n) " + value.getClass().getName(), e);
      }
    } else {
      throw new DataException(
          "Unsupported serialization of an unexpected class `"
              + value.getClass().getName()
              + "` in value.");
    }
  }

  /**
   * @return {@link ObjectMapper} that can serialize all the Kafka Connect types.
   */
  @VisibleForTesting
  protected static ObjectMapper getJsonMapper() {
    ObjectMapper mapper = new ObjectMapper();
    SimpleModule mapperModule = new SimpleModule("KafkaConnectSerializer");
    mapperModule.addSerializer(Struct.class, new StructJsonSerializer(Struct.class));
    mapper.registerModule(mapperModule);
    return mapper;
  }

  private static class StructJsonSerializer extends StdSerializer<Struct> {
    protected StructJsonSerializer(Class<Struct> t) {
      super(t);
    }

    @Override
    public void serialize(Struct value, JsonGenerator gen, SerializerProvider provider)
        throws IOException {
      Schema schema = value.schema();
      logIfLogicalTypeUnsupported(Optional.ofNullable(schema));
      gen.writeStartObject();
      for (Field field : schema.fields()) {
        String fieldName = field.name();
        Schema fieldSchema = field.schema();
        logIfLogicalTypeUnsupported(Optional.ofNullable(fieldSchema));
        Object logicalFieldValue = value.getWithoutDefault(fieldName);
        Object physicalFieldValue = convertToPhysicalValue(logicalFieldValue, fieldSchema);
        gen.writeObjectField(fieldName, physicalFieldValue);
      }
      gen.writeEndObject();
    }

    private Object convertToPhysicalValue(Object logicalFieldValue, Schema fieldSchema) {
      if (logicalFieldValue instanceof Date) {
        Date date = (Date) logicalFieldValue;
        String logicalName = Optional.ofNullable(fieldSchema).map(Schema::name).orElse("");
        switch (logicalName) {
          case Timestamp.LOGICAL_NAME:
            return Timestamp.fromLogical(fieldSchema, date);
          case Time.LOGICAL_NAME:
            return Time.fromLogical(fieldSchema, date);
          case org.apache.kafka.connect.data.Date.LOGICAL_NAME:
            return org.apache.kafka.connect.data.Date.fromLogical(fieldSchema, date);
        }
      } else if (logicalFieldValue instanceof BigDecimal) {
        return Decimal.fromLogical(fieldSchema, (BigDecimal) logicalFieldValue);
      }
      return logicalFieldValue;
    }
  }
}
