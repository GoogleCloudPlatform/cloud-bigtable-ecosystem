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

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.json.JsonMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.google.cloud.bigtable.data.v2.models.Range;
import com.google.cloud.kafka.connect.bigtable.config.ConfigInterpolation;
import com.google.cloud.kafka.connect.bigtable.config.NullValueMode;
import com.google.cloud.kafka.connect.bigtable.util.ProtoUtil;
import com.google.protobuf.ByteString;
import org.apache.kafka.connect.data.Schema;
import org.apache.kafka.connect.data.SchemaAndValue;
import org.apache.kafka.connect.data.SchemaBuilder;
import org.apache.kafka.connect.data.Struct;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;

import java.math.BigDecimal;
import java.nio.charset.StandardCharsets;
import java.time.Instant;
import java.time.temporal.ChronoUnit;
import java.util.*;

import static com.google.cloud.kafka.connect.bigtable.util.MockUtil.assertTotalNumberOfInvocations;
import static com.google.cloud.kafka.connect.bigtable.util.NestedNullStructFactory.*;
import static org.junit.Assert.assertEquals;
import static org.junit.Assert.assertTrue;
import static org.mockito.Mockito.*;

@RunWith(JUnit4.class)
public class ValueMapperTest {

  private static final String DEFAULT_COLUMN_FAMILY = "COLUMN_FAMILY";
  private static final String DEFAULT_COLUMN = "COLUMN_QUALIFIER";
  private static final ByteString DEFAULT_COLUMN_BYTES =
      ByteString.copyFrom(DEFAULT_COLUMN.getBytes(StandardCharsets.UTF_8));
  private static final ByteString ROW_KEY =
      ByteString.copyFrom("ROW_KEY".getBytes(StandardCharsets.UTF_8));
  private static final String TARGET_TABLE_NAME = "table";
  private static final Long TIMESTAMP = 2024L;
  private static final Range.TimestampRange TIMESTAMP_RANGE =
      Range.TimestampRange.create(0, TIMESTAMP);
  private static final String DEFAULT_TOPIC = "topic";

  @Test
  public void testBoolean() {
    Boolean value = true;
    ByteString expected = ByteString.copyFrom(ByteUtils.toBytes(value));
    ValueMapper mapper =
        new TestValueMapper(DEFAULT_COLUMN_FAMILY, DEFAULT_COLUMN, NullValueMode.IGNORE, false);
    MutationDataBuilder mutationDataBuilder = getRecordMutationDataBuilder(mapper, value);
    verify(mutationDataBuilder, times(1))
        .setCell(DEFAULT_COLUMN_FAMILY, DEFAULT_COLUMN_BYTES, TIMESTAMP, expected);
    assertTotalNumberOfInvocations(mutationDataBuilder, 1);
  }

  @Test
  public void testString() {
    String value = "rrrrrrr";
    ByteString expected = ByteString.copyFrom(ByteUtils.toBytes(value));
    ValueMapper mapper =
        new TestValueMapper(DEFAULT_COLUMN_FAMILY, DEFAULT_COLUMN, NullValueMode.IGNORE, false);
    MutationDataBuilder mutationDataBuilder = getRecordMutationDataBuilder(mapper, value);
    verify(mutationDataBuilder, times(1))
        .setCell(DEFAULT_COLUMN_FAMILY, DEFAULT_COLUMN_BYTES, TIMESTAMP, expected);
    assertTotalNumberOfInvocations(mutationDataBuilder, 1);
  }

  @Test
  public void testLong() {
    Long value = 9223372036854775807L;
    ByteString expected = ByteString.copyFrom(ByteUtils.toBytes(value));
    ValueMapper mapper =
        new TestValueMapper(DEFAULT_COLUMN_FAMILY, DEFAULT_COLUMN, NullValueMode.IGNORE, false);
    MutationDataBuilder mutationDataBuilder = getRecordMutationDataBuilder(mapper, value);
    verify(mutationDataBuilder, times(1))
        .setCell(DEFAULT_COLUMN_FAMILY, DEFAULT_COLUMN_BYTES, TIMESTAMP, expected);
    assertTotalNumberOfInvocations(mutationDataBuilder, 1);
  }

  @Test
  public void testInteger() {
    Integer value = -2147483648;
    ByteString expected = ByteString.copyFrom(ByteUtils.toBytes(value));
    ValueMapper mapper =
        new TestValueMapper(DEFAULT_COLUMN_FAMILY, DEFAULT_COLUMN, NullValueMode.IGNORE, false);
    MutationDataBuilder mutationDataBuilder = getRecordMutationDataBuilder(mapper, value);
    verify(mutationDataBuilder, times(1))
        .setCell(DEFAULT_COLUMN_FAMILY, DEFAULT_COLUMN_BYTES, TIMESTAMP, expected);
    assertTotalNumberOfInvocations(mutationDataBuilder, 1);
  }

  @Test
  public void testShort() {
    Short value = 32767;
    ByteString expected = ByteString.copyFrom(ByteUtils.toBytes(value));
    ValueMapper mapper =
        new TestValueMapper(DEFAULT_COLUMN_FAMILY, DEFAULT_COLUMN, NullValueMode.IGNORE, false);
    MutationDataBuilder mutationDataBuilder = getRecordMutationDataBuilder(mapper, value);
    verify(mutationDataBuilder, times(1))
        .setCell(DEFAULT_COLUMN_FAMILY, DEFAULT_COLUMN_BYTES, TIMESTAMP, expected);
    assertTotalNumberOfInvocations(mutationDataBuilder, 1);
  }

  @Test
  public void testByte() {
    Byte value = -128;
    ByteString expected = ByteString.copyFrom(ByteUtils.toBytes(value));
    ValueMapper mapper =
        new TestValueMapper(DEFAULT_COLUMN_FAMILY, DEFAULT_COLUMN, NullValueMode.IGNORE, false);
    MutationDataBuilder mutationDataBuilder = getRecordMutationDataBuilder(mapper, value);
    verify(mutationDataBuilder, times(1))
        .setCell(DEFAULT_COLUMN_FAMILY, DEFAULT_COLUMN_BYTES, TIMESTAMP, expected);
    assertTotalNumberOfInvocations(mutationDataBuilder, 1);
  }

  @Test
  public void testBytes() {
    byte[] value = new byte[]{(byte) 37, (byte) 21};
    ByteString expected = ByteString.copyFrom(value);
    ValueMapper mapper =
        new TestValueMapper(DEFAULT_COLUMN_FAMILY, DEFAULT_COLUMN, NullValueMode.IGNORE, false);
    MutationDataBuilder mutationDataBuilder = getRecordMutationDataBuilder(mapper, value);
    verify(mutationDataBuilder, times(1))
        .setCell(DEFAULT_COLUMN_FAMILY, DEFAULT_COLUMN_BYTES, TIMESTAMP, expected);
    assertTotalNumberOfInvocations(mutationDataBuilder, 1);
  }

  @Test
  public void testFloat() {
    Float value = 128.37157f;
    ByteString expected = ByteString.copyFrom(ByteUtils.toBytes(value));
    ValueMapper mapper =
        new TestValueMapper(DEFAULT_COLUMN_FAMILY, DEFAULT_COLUMN, NullValueMode.IGNORE, false);
    MutationDataBuilder mutationDataBuilder = getRecordMutationDataBuilder(mapper, value);
    verify(mutationDataBuilder, times(1))
        .setCell(DEFAULT_COLUMN_FAMILY, DEFAULT_COLUMN_BYTES, TIMESTAMP, expected);
    assertTotalNumberOfInvocations(mutationDataBuilder, 1);
  }

  @Test
  public void testDouble() {
    Double value = 128.37157;
    ByteString expected = ByteString.copyFrom(ByteUtils.toBytes(value));
    ValueMapper mapper =
        new TestValueMapper(DEFAULT_COLUMN_FAMILY, DEFAULT_COLUMN, NullValueMode.IGNORE, false);
    MutationDataBuilder mutationDataBuilder = getRecordMutationDataBuilder(mapper, value);
    verify(mutationDataBuilder, times(1))
        .setCell(DEFAULT_COLUMN_FAMILY, DEFAULT_COLUMN_BYTES, TIMESTAMP, expected);
    assertTotalNumberOfInvocations(mutationDataBuilder, 1);
  }

  @Test
  public void testDoubleSpecial() {
    Double value = Double.NaN;
    ByteString expected = ByteString.copyFrom(ByteUtils.toBytes(value));
    ValueMapper mapper =
        new TestValueMapper(DEFAULT_COLUMN_FAMILY, DEFAULT_COLUMN, NullValueMode.IGNORE, false);
    MutationDataBuilder mutationDataBuilder = getRecordMutationDataBuilder(mapper, value);
    verify(mutationDataBuilder, times(1))
        .setCell(DEFAULT_COLUMN_FAMILY, DEFAULT_COLUMN_BYTES, TIMESTAMP, expected);
    assertTotalNumberOfInvocations(mutationDataBuilder, 1);
  }

  @Test
  public void testDate() {
    Date date = new Date(1732822801000L);
    ByteString expected = ByteString.copyFrom(ByteUtils.toBytes(date.getTime()));
    ValueMapper mapper =
        new TestValueMapper(DEFAULT_COLUMN_FAMILY, DEFAULT_COLUMN, NullValueMode.IGNORE, false);
    MutationDataBuilder mutationDataBuilder = getRecordMutationDataBuilder(mapper, date);
    verify(mutationDataBuilder, times(1))
        .setCell(DEFAULT_COLUMN_FAMILY, DEFAULT_COLUMN_BYTES, TIMESTAMP, expected);
    assertTotalNumberOfInvocations(mutationDataBuilder, 1);
  }

  @Test
  public void testDecimal() {
    BigDecimal value = new BigDecimal("0.300000000000000000000000000000001");
    ByteString expected = ByteString.copyFrom(ByteUtils.toBytes(value));
    ValueMapper mapper =
        new TestValueMapper(DEFAULT_COLUMN_FAMILY, DEFAULT_COLUMN, NullValueMode.IGNORE, false);
    MutationDataBuilder mutationDataBuilder = getRecordMutationDataBuilder(mapper, value);
    verify(mutationDataBuilder, times(1))
        .setCell(DEFAULT_COLUMN_FAMILY, DEFAULT_COLUMN_BYTES, TIMESTAMP, expected);
    assertTotalNumberOfInvocations(mutationDataBuilder, 1);
  }

  @Test
  public void testArray() {
    List<Object> value = List.of("1", 2, true);
    ValueMapper mapper =
        new TestValueMapper(DEFAULT_COLUMN_FAMILY, DEFAULT_COLUMN, NullValueMode.IGNORE, false);
    MutationDataBuilder mutationDataBuilder = getRecordMutationDataBuilder(mapper, value);
    verify(mutationDataBuilder, times(1))
        .setCell(
            DEFAULT_COLUMN_FAMILY,
            DEFAULT_COLUMN_BYTES,
            TIMESTAMP,
            ByteString.copyFrom("[\"1\",2,true]".getBytes(StandardCharsets.UTF_8)));
    assertTotalNumberOfInvocations(mutationDataBuilder, 1);
  }

  @Test
  public void testRootValueNeedsBothDefaultColumns() {
    Integer value = 123;
    for (ValueMapper mapper :
        List.of(
            new TestValueMapper(null, null, NullValueMode.WRITE, false),
            new TestValueMapper(DEFAULT_COLUMN_FAMILY, null, NullValueMode.WRITE, false),
            new TestValueMapper(null, DEFAULT_COLUMN, NullValueMode.WRITE, false))) {
      MutationDataBuilder mutationDataBuilder = getRecordMutationDataBuilder(mapper, value);
      verify(mutationDataBuilder, times(0))
          .setCell(
              DEFAULT_COLUMN_FAMILY,
              DEFAULT_COLUMN_BYTES,
              TIMESTAMP,
              ByteString.copyFrom(ByteUtils.toBytes(value)));
    }
    ValueMapper mapper =
        new TestValueMapper(DEFAULT_COLUMN_FAMILY, DEFAULT_COLUMN, NullValueMode.WRITE, false);
    MutationDataBuilder mutationDataBuilder = getRecordMutationDataBuilder(mapper, value);
    verify(mutationDataBuilder, times(1))
        .setCell(
            DEFAULT_COLUMN_FAMILY,
            DEFAULT_COLUMN_BYTES,
            TIMESTAMP,
            ByteString.copyFrom(ByteUtils.toBytes(value)));
    assertTotalNumberOfInvocations(mutationDataBuilder, 1);
  }

  @Test
  public void testValueNestedOnceNeedsOnlyDefaultColumnFamily() {
    String key = "key";
    Long value = 2L;
    Struct struct = new Struct(SchemaBuilder.struct().field(key, Schema.INT64_SCHEMA));
    struct.put(key, value);

    ValueMapper mapper = new TestValueMapper(DEFAULT_COLUMN_FAMILY, null, NullValueMode.WRITE, false);
    MutationDataBuilder mutationDataBuilder = getRecordMutationDataBuilder(mapper, struct);
    verify(mutationDataBuilder, times(1))
        .setCell(
            DEFAULT_COLUMN_FAMILY,
            ByteString.copyFrom(key.getBytes(StandardCharsets.UTF_8)),
            TIMESTAMP,
            ByteString.copyFrom(ByteUtils.toBytes(value)));

    assertTotalNumberOfInvocations(mutationDataBuilder, 1);
  }

  @Test
  public void testDefaultColumnFamilyInterpolation() {
    String topic = "TOPIC";
    String value = "value";
    ValueMapper mapper = new TestValueMapper("${topic}", DEFAULT_COLUMN, NullValueMode.WRITE, false);
    MutationDataBuilder mutationDataBuilder = getRecordMutationDataBuilder(mapper, value, topic);
    verify(mutationDataBuilder, times(1))
        .setCell(
            topic,
            DEFAULT_COLUMN_BYTES,
            TIMESTAMP,
            ByteString.copyFrom(value.getBytes(StandardCharsets.UTF_8)));
    assertTotalNumberOfInvocations(mutationDataBuilder, 1);
  }

  @Test
  public void testMultipleOperationsAtOnce() {
    String setColumnFamily = "setColumnFamily";
    String setColumn = "setColumn";
    String setRoot = "setRoot";
    String deleteColumnFamily = "deleteColumnFamily";
    String deleteColumn = "deleteColumn";
    String deleteRoot = "deleteRoot";

    Integer value = 789;
    Boolean rootValue = true;

    Struct createStruct =
        new Struct(SchemaBuilder.struct().field(setColumn, Schema.INT32_SCHEMA))
            .put(setColumn, value);
    Struct deleteStruct =
        new Struct(SchemaBuilder.struct().field(deleteColumn, Schema.OPTIONAL_INT8_SCHEMA))
            .put(deleteColumn, null);
    Struct struct =
        new Struct(
            SchemaBuilder.struct()
                .field(setColumnFamily, createStruct.schema())
                .field(setRoot, Schema.BOOLEAN_SCHEMA)
                .field(deleteColumnFamily, deleteStruct.schema())
                .field(deleteRoot, Schema.OPTIONAL_INT8_SCHEMA))
            .put(setColumnFamily, createStruct)
            .put(setRoot, rootValue)
            .put(deleteColumnFamily, deleteStruct)
            .put(deleteRoot, null);

    ValueMapper mapper =
        new TestValueMapper(DEFAULT_COLUMN_FAMILY, DEFAULT_COLUMN, NullValueMode.DELETE, false);
    MutationDataBuilder mutationDataBuilder = getRecordMutationDataBuilder(mapper, struct);
    verify(mutationDataBuilder, times(1))
        .setCell(
            setColumnFamily,
            ByteString.copyFrom(setColumn.getBytes(StandardCharsets.UTF_8)),
            TIMESTAMP,
            ByteString.copyFrom(ByteUtils.toBytes(value)));
    verify(mutationDataBuilder, times(1))
        .setCell(
            DEFAULT_COLUMN_FAMILY,
            ByteString.copyFrom(setRoot.getBytes(StandardCharsets.UTF_8)),
            TIMESTAMP,
            ByteString.copyFrom(ByteUtils.toBytes(rootValue)));
    verify(mutationDataBuilder, times(1))
        .deleteCells(
            deleteColumnFamily,
            ByteString.copyFrom(deleteColumn.getBytes(StandardCharsets.UTF_8)),
            Range.TimestampRange.create(0, TIMESTAMP));
    verify(mutationDataBuilder, times(1)).deleteFamily(deleteRoot);
    assertTotalNumberOfInvocations(mutationDataBuilder, 4);
  }

  @Test
  public void testMap() throws JsonProcessingException {
    Object outerMapKey = 123456;
    Object innerMapKey = "innerMapKey";
    String familyToBeDeleted = "familyToBeDeleted";
    String columnToBeDeleted = "columnToBeDeleted";
    Object innermostNullKey = "innermostNullKey";

    Object value = "value";
    Object valueKey = "valueKey";

    Map<Object, Object> innermostMap = new HashMap<>();
    Map<Object, Object> innerMap = new HashMap<>();
    Map<Object, Object> outerMap = new HashMap<>();

    outerMap.put(outerMapKey, innerMap);
    innerMap.put(innerMapKey, innermostMap);

    outerMap.put(valueKey, value);
    innerMap.put(valueKey, value);
    innermostMap.put(valueKey, value);

    outerMap.put(familyToBeDeleted, null);
    innerMap.put(columnToBeDeleted, null);
    innermostMap.put(innermostNullKey, null);

    ByteString expectedJsonificationBytes =
        ByteString.copyFrom(ValueMapper.getJsonMapper().writeValueAsBytes(outerMap));

    ValueMapper mapper =
        new TestValueMapper(DEFAULT_COLUMN_FAMILY, DEFAULT_COLUMN, NullValueMode.DELETE, false);
    MutationDataBuilder mutationDataBuilder = getRecordMutationDataBuilder(mapper, outerMap);
    assertTotalNumberOfInvocations(mutationDataBuilder, 1);
    verify(mutationDataBuilder, times(1))
        .setCell(
            DEFAULT_COLUMN_FAMILY, DEFAULT_COLUMN_BYTES, TIMESTAMP, expectedJsonificationBytes);
  }

  @Test
  public void testJsonificationOfNonJsonNativeTypes() {
    final String dateFieldName = "KafkaDate";
    final String timestampFieldName = "KafkaTimestamp";
    final String timeFieldName = "KafkaTime";
    final String decimalFieldName = "KafkaDecimal";
    final String bytesFieldName = "KafkaBytes";
    final Date timestamp = new Date(1488406838808L);
    final Date time = Date.from(Instant.EPOCH.plus(1234567890, ChronoUnit.MILLIS));
    final Date date = Date.from(Instant.EPOCH.plus(363, ChronoUnit.DAYS));
    final String decimalString = "0.30000000000000004";
    final Integer decimalScale = 17;
    final BigDecimal decimal = new BigDecimal(decimalString);
    final byte[] bytes = "bytes\0".getBytes(StandardCharsets.UTF_8);

    Struct structToBeJsonified =
        new Struct(
            SchemaBuilder.struct()
                .field(dateFieldName, org.apache.kafka.connect.data.Date.SCHEMA)
                .field(timestampFieldName, org.apache.kafka.connect.data.Timestamp.SCHEMA)
                .field(timeFieldName, org.apache.kafka.connect.data.Timestamp.SCHEMA)
                .field(
                    decimalFieldName,
                    org.apache.kafka.connect.data.Decimal.schema(decimalScale))
                .field(bytesFieldName, Schema.BYTES_SCHEMA)
                .build())
            .put(dateFieldName, date)
            .put(timestampFieldName, timestamp)
            .put(timeFieldName, time)
            .put(decimalFieldName, decimal)
            .put(bytesFieldName, bytes);

    String innerField = "innerField";
    String outerField = "outerField";
    Struct innerStruct =
        new Struct(SchemaBuilder.struct().field(innerField, structToBeJsonified.schema()))
            .put(innerField, structToBeJsonified);
    Struct outerStruct =
        new Struct(SchemaBuilder.struct().field(outerField, innerStruct.schema()))
            .put(outerField, innerStruct);

    String expectedStringification =
        "{\"KafkaDate\":363,\"KafkaTimestamp\":1488406838808,\"KafkaTime\":1234567890,\"KafkaDecimal\":\"apTXT0MABA==\",\"KafkaBytes\":\"Ynl0ZXMA\"}";
    ByteString expectedStringificationBytes =
        ByteString.copyFrom(expectedStringification.getBytes(StandardCharsets.UTF_8));

    ValueMapper mapper = new TestValueMapper(null, null, NullValueMode.DELETE, false);
    MutationDataBuilder mutationDataBuilder = getRecordMutationDataBuilder(mapper, outerStruct);
    verify(mutationDataBuilder, times(1))
        .setCell(
            outerField,
            ByteString.copyFrom(innerField.getBytes(StandardCharsets.UTF_8)),
            TIMESTAMP,
            expectedStringificationBytes);
    assertTotalNumberOfInvocations(mutationDataBuilder, 1);
  }

  @Test
  public void testStruct() {
    final String structFieldName = "struct";
    final ByteString structFieldNameBytes =
        ByteString.copyFrom(structFieldName.getBytes(StandardCharsets.UTF_8));
    final String valueFieldName = "value";
    final ByteString valueFieldNameBytes =
        ByteString.copyFrom(valueFieldName.getBytes(StandardCharsets.UTF_8));
    final String optionalFieldName = "optional";
    final ByteString optionalFieldNameBytes =
        ByteString.copyFrom(optionalFieldName.getBytes(StandardCharsets.UTF_8));
    final byte[] value = "value\0".getBytes(StandardCharsets.UTF_8);

    Schema innermostStructSchema =
        SchemaBuilder.struct()
            .field(valueFieldName, Schema.BYTES_SCHEMA)
            .field(optionalFieldName, Schema.OPTIONAL_INT8_SCHEMA)
            .build();
    Schema innerStructSchema =
        SchemaBuilder.struct()
            .field(structFieldName, innermostStructSchema)
            .field(valueFieldName, Schema.BYTES_SCHEMA)
            .field(optionalFieldName, Schema.OPTIONAL_INT8_SCHEMA)
            .build();
    Schema outerStructSchema =
        SchemaBuilder.struct()
            .field(structFieldName, innerStructSchema)
            .field(valueFieldName, Schema.BYTES_SCHEMA)
            .field(optionalFieldName, Schema.OPTIONAL_INT8_SCHEMA)
            .build();

    Struct innermostStruct = new Struct(innermostStructSchema);
    innermostStruct.put(valueFieldName, value);

    String expectedInnermostStringification = "{\"value\":\"dmFsdWUA\",\"optional\":null}";
    ByteString expectedInnermostStringificationBytes =
        ByteString.copyFrom(expectedInnermostStringification.getBytes(StandardCharsets.UTF_8));

    Struct innerStruct = new Struct(innerStructSchema);
    innerStruct.put(structFieldName, innermostStruct);
    innerStruct.put(valueFieldName, value);

    Struct struct = new Struct(outerStructSchema);
    struct.put(structFieldName, innerStruct);
    struct.put(valueFieldName, value);

    /*
    {
        struct: {
            struct: {
                optionalFieldName: null,
                valueFieldName: value,
            }
            optionalFieldName: null,
            valueFieldName: value,
        }
        optionalFieldName: null,
        valueFieldName: value,
    }
    */
    ValueMapper mapper =
        new TestValueMapper(DEFAULT_COLUMN_FAMILY, DEFAULT_COLUMN, NullValueMode.DELETE, false);
    MutationDataBuilder mutationDataBuilder = getRecordMutationDataBuilder(mapper, struct);
    verify(mutationDataBuilder, times(1))
        .setCell(DEFAULT_COLUMN_FAMILY, valueFieldNameBytes, TIMESTAMP, ByteString.copyFrom(value));
    verify(mutationDataBuilder, times(1)).deleteFamily(optionalFieldName);
    verify(mutationDataBuilder, times(1))
        .setCell(structFieldName, valueFieldNameBytes, TIMESTAMP, ByteString.copyFrom(value));
    verify(mutationDataBuilder, times(1))
        .deleteCells(
            structFieldName, optionalFieldNameBytes, Range.TimestampRange.create(0, TIMESTAMP));
    verify(mutationDataBuilder, times(1))
        .setCell(
            structFieldName,
            structFieldNameBytes,
            TIMESTAMP,
            expectedInnermostStringificationBytes);
    assertTotalNumberOfInvocations(mutationDataBuilder, 5);
    assertTrue(mutationDataBuilder.maybeBuild(TARGET_TABLE_NAME, ROW_KEY).isPresent());
  }

  @Test
  public void testEmptyStruct() {
    Schema emptyStructSchema = SchemaBuilder.struct().build();
    Struct emptyStruct = new Struct(emptyStructSchema);
    ValueMapper mapper =
        new TestValueMapper(DEFAULT_COLUMN_FAMILY, DEFAULT_COLUMN, NullValueMode.WRITE, false);
    MutationDataBuilder mutationDataBuilder = getRecordMutationDataBuilder(mapper, emptyStruct);
    assertTotalNumberOfInvocations(mutationDataBuilder, 0);
    assertTrue(mutationDataBuilder.maybeBuild(TARGET_TABLE_NAME, ROW_KEY).isEmpty());
  }

  @Test
  public void testSimpleCase1() {
    Integer value = 1;
    String innerField = "bar";
    String outerField = "foo";

    Struct innerStruct =
        new Struct(SchemaBuilder.struct().field(innerField, Schema.INT32_SCHEMA))
            .put(innerField, value);
    Struct outerStruct =
        new Struct(SchemaBuilder.struct().field(outerField, innerStruct.schema()))
            .put(outerField, innerStruct);

    ValueMapper mapper = new TestValueMapper(null, null, NullValueMode.IGNORE, false);
    MutationDataBuilder mutationDataBuilder = getRecordMutationDataBuilder(mapper, outerStruct);
    verify(mutationDataBuilder, times(1))
        .setCell(
            outerField,
            ByteString.copyFrom(innerField.getBytes(StandardCharsets.UTF_8)),
            TIMESTAMP,
            ByteString.copyFrom(ByteUtils.toBytes(value)));
    assertTotalNumberOfInvocations(mutationDataBuilder, 1);
    assertTrue(mutationDataBuilder.maybeBuild(TARGET_TABLE_NAME, ROW_KEY).isPresent());
  }

  @Test
  public void testSimpleCase2() {
    Integer value = 1;
    String innerField = "fizz";
    String middleField = "bar";
    String outerField = "foo";

    Struct innerStruct =
        new Struct(SchemaBuilder.struct().field(innerField, Schema.INT32_SCHEMA))
            .put(innerField, value);
    Struct middleStruct =
        new Struct(SchemaBuilder.struct().field(middleField, innerStruct.schema()))
            .put(middleField, innerStruct);
    Struct outerStruct =
        new Struct(SchemaBuilder.struct().field(outerField, middleStruct.schema()))
            .put(outerField, middleStruct);

    ValueMapper mapper = new TestValueMapper(null, null, NullValueMode.IGNORE, false);
    MutationDataBuilder mutationDataBuilder = getRecordMutationDataBuilder(mapper, outerStruct);
    verify(mutationDataBuilder, times(1))
        .setCell(
            outerField,
            ByteString.copyFrom(middleField.getBytes(StandardCharsets.UTF_8)),
            TIMESTAMP,
            ByteString.copyFrom(
                ("{\"" + innerField + "\":" + value + "}").getBytes(StandardCharsets.UTF_8)));
    assertTrue(mutationDataBuilder.maybeBuild(TARGET_TABLE_NAME, ROW_KEY).isPresent());
  }

  @Test
  public void testSimpleCase3() {
    Integer value = 1;
    String field = "foo";
    Struct struct =
        new Struct(SchemaBuilder.struct().field(field, Schema.INT32_SCHEMA)).put(field, value);

    ValueMapper mapper = new TestValueMapper(DEFAULT_COLUMN_FAMILY, null, NullValueMode.IGNORE, false);
    MutationDataBuilder mutationDataBuilder = getRecordMutationDataBuilder(mapper, struct);
    verify(mutationDataBuilder, times(1))
        .setCell(
            DEFAULT_COLUMN_FAMILY,
            ByteString.copyFrom(field.getBytes(StandardCharsets.UTF_8)),
            TIMESTAMP,
            ByteString.copyFrom(ByteUtils.toBytes(value)));
    assertTotalNumberOfInvocations(mutationDataBuilder, 1);
    assertTrue(mutationDataBuilder.maybeBuild(TARGET_TABLE_NAME, ROW_KEY).isPresent());
  }

  @Test
  public void testComplicatedCase() {
    String innerStructKey = "innerStructKey";
    String familyToBeDeleted = "familyToBeDeleted";
    String columnToBeDeleted = "columnToBeDeleted";
    ByteString columnToBeDeletedBytes =
        ByteString.copyFrom(columnToBeDeleted.getBytes(StandardCharsets.UTF_8));
    String innermostNullKey = "innermostNullKey";
    String outerStructKey = "outerStructKey";

    String value = "value";
    ByteString valueBytes = ByteString.copyFrom((value).getBytes(StandardCharsets.UTF_8));
    String valueKey = "valueKey";
    ByteString valueKeyBytes = ByteString.copyFrom((valueKey).getBytes(StandardCharsets.UTF_8));

    Struct innermostStruct =
        new Struct(
            SchemaBuilder.struct()
                .field(valueKey, Schema.STRING_SCHEMA)
                .field(innermostNullKey, Schema.OPTIONAL_INT8_SCHEMA))
            .put(valueKey, value)
            .put(innermostNullKey, null);
    Struct innerStruct =
        new Struct(
            SchemaBuilder.struct()
                .field(innerStructKey, innermostStruct.schema())
                .field(valueKey, Schema.STRING_SCHEMA)
                .field(columnToBeDeleted, Schema.OPTIONAL_INT8_SCHEMA))
            .put(innerStructKey, innermostStruct)
            .put(valueKey, value)
            .put(columnToBeDeleted, null);
    Struct outerStruct =
        new Struct(
            SchemaBuilder.struct()
                .field(outerStructKey, innerStruct.schema())
                .field(valueKey, Schema.STRING_SCHEMA)
                .field(familyToBeDeleted, Schema.OPTIONAL_INT8_SCHEMA))
            .put(outerStructKey, innerStruct)
            .put(valueKey, value)
            .put(familyToBeDeleted, null);

    /*
    {
        outerStructKey: {
            innerStructKey: {
                valueKey: value,
                innermostNullKey: null,
            }
            valueKey: value,
            columnToBeDeleted: null,
        }
        valueKey: value,
        familyToBeDeleted: null,
    }
     */
    String expectedJsonification = "{\"valueKey\":\"value\",\"innermostNullKey\":null}";
    ByteString expectedJsonificationBytes =
        ByteString.copyFrom(expectedJsonification.getBytes(StandardCharsets.UTF_8));

    ValueMapper mapper =
        new TestValueMapper(DEFAULT_COLUMN_FAMILY, DEFAULT_COLUMN, NullValueMode.DELETE, false);
    MutationDataBuilder mutationDataBuilder = getRecordMutationDataBuilder(mapper, outerStruct);
    verify(mutationDataBuilder, times(1)).deleteFamily(familyToBeDeleted);
    verify(mutationDataBuilder, times(1))
        .setCell(DEFAULT_COLUMN_FAMILY, valueKeyBytes, TIMESTAMP, valueBytes);
    verify(mutationDataBuilder, times(1))
        .deleteCells(
            outerStructKey.toString(),
            columnToBeDeletedBytes,
            Range.TimestampRange.create(0, TIMESTAMP));
    verify(mutationDataBuilder, times(1))
        .setCell(outerStructKey.toString(), valueKeyBytes, TIMESTAMP, valueBytes);
    verify(mutationDataBuilder, times(1))
        .setCell(
            outerStructKey.toString(),
            ByteString.copyFrom(innerStructKey.toString().getBytes(StandardCharsets.UTF_8)),
            TIMESTAMP,
            expectedJsonificationBytes);
    assertTotalNumberOfInvocations(mutationDataBuilder, 5);
  }

  @Test
  public void testNullModeIgnoreRoot() {
    ValueMapper mapper =
        new TestValueMapper(DEFAULT_COLUMN_FAMILY, DEFAULT_COLUMN, NullValueMode.IGNORE, false);
    MutationDataBuilder mutationDataBuilder = getRecordMutationDataBuilder(mapper, null);
    assertTotalNumberOfInvocations(mutationDataBuilder, 0);
    assertTrue(mutationDataBuilder.maybeBuild(TARGET_TABLE_NAME, ROW_KEY).isEmpty());
  }

  @Test
  public void testNullModeIgnoreNestedOnce() {
    ValueMapper mapper =
        new TestValueMapper(DEFAULT_COLUMN_FAMILY, DEFAULT_COLUMN, NullValueMode.IGNORE, false);
    MutationDataBuilder mutationDataBuilder =
        getRecordMutationDataBuilder(mapper, getStructWithNullOnNthNestingLevel(1));
    assertTotalNumberOfInvocations(mutationDataBuilder, 0);
    assertTrue(mutationDataBuilder.maybeBuild(TARGET_TABLE_NAME, ROW_KEY).isEmpty());
  }

  @Test
  public void testNullModeIgnoreNestedTwice() {
    ValueMapper mapper =
        new TestValueMapper(DEFAULT_COLUMN_FAMILY, DEFAULT_COLUMN, NullValueMode.IGNORE, false);
    MutationDataBuilder mutationDataBuilder =
        getRecordMutationDataBuilder(mapper, getStructWithNullOnNthNestingLevel(2));
    assertTotalNumberOfInvocations(mutationDataBuilder, 0);
    assertTrue(mutationDataBuilder.maybeBuild(TARGET_TABLE_NAME, ROW_KEY).isEmpty());
  }

  @Test
  public void testNullModeWriteRoot() {
    ValueMapper mapper =
        new TestValueMapper(DEFAULT_COLUMN_FAMILY, DEFAULT_COLUMN, NullValueMode.WRITE, false);
    MutationDataBuilder mutationDataBuilder = getRecordMutationDataBuilder(mapper, null);
    verify(mutationDataBuilder, times(1))
        .setCell(DEFAULT_COLUMN_FAMILY, DEFAULT_COLUMN_BYTES, TIMESTAMP, ByteString.empty());
    assertTotalNumberOfInvocations(mutationDataBuilder, 1);
    assertTrue(mutationDataBuilder.maybeBuild(TARGET_TABLE_NAME, ROW_KEY).isPresent());
  }

  @Test
  public void testNullModeWriteNestedOnce() {
    ValueMapper mapper =
        new TestValueMapper(DEFAULT_COLUMN_FAMILY, DEFAULT_COLUMN, NullValueMode.WRITE, false);
    MutationDataBuilder mutationDataBuilder =
        getRecordMutationDataBuilder(mapper, getStructWithNullOnNthNestingLevel(1));
    verify(mutationDataBuilder, times(1))
        .setCell(
            DEFAULT_COLUMN_FAMILY,
            NESTED_NULL_STRUCT_FIELD_NAME_BYTES,
            TIMESTAMP,
            ByteString.empty());
    assertTotalNumberOfInvocations(mutationDataBuilder, 1);
    assertTrue(mutationDataBuilder.maybeBuild(TARGET_TABLE_NAME, ROW_KEY).isPresent());
  }

  @Test
  public void testNullModeWriteNestedTwice() {
    ValueMapper mapper =
        new TestValueMapper(DEFAULT_COLUMN_FAMILY, DEFAULT_COLUMN, NullValueMode.WRITE, false);
    MutationDataBuilder mutationDataBuilder =
        getRecordMutationDataBuilder(mapper, getStructWithNullOnNthNestingLevel(2));
    verify(mutationDataBuilder, times(1))
        .setCell(
            NESTED_NULL_STRUCT_FIELD_NAME,
            NESTED_NULL_STRUCT_FIELD_NAME_BYTES,
            TIMESTAMP,
            ByteString.empty());
    assertTotalNumberOfInvocations(mutationDataBuilder, 1);
    assertTrue(mutationDataBuilder.maybeBuild(TARGET_TABLE_NAME, ROW_KEY).isPresent());
  }

  @Test
  public void testNullModeDeleteRoot() {
    ValueMapper mapper =
        new TestValueMapper(DEFAULT_COLUMN_FAMILY, DEFAULT_COLUMN, NullValueMode.DELETE, false);
    MutationDataBuilder mutationDataBuilder = getRecordMutationDataBuilder(mapper, null);
    verify(mutationDataBuilder, times(1)).deleteRow();
    assertTotalNumberOfInvocations(mutationDataBuilder, 1);
    assertTrue(mutationDataBuilder.maybeBuild(TARGET_TABLE_NAME, ROW_KEY).isPresent());
  }

  @Test
  public void testNullModeDeleteNestedOnce() {
    ValueMapper mapper =
        new TestValueMapper(DEFAULT_COLUMN_FAMILY, DEFAULT_COLUMN, NullValueMode.DELETE, false);
    MutationDataBuilder mutationDataBuilder =
        getRecordMutationDataBuilder(mapper, getStructWithNullOnNthNestingLevel(1));
    verify(mutationDataBuilder, times(1)).deleteFamily(NESTED_NULL_STRUCT_FIELD_NAME);
    assertTotalNumberOfInvocations(mutationDataBuilder, 1);
    assertTrue(mutationDataBuilder.maybeBuild(TARGET_TABLE_NAME, ROW_KEY).isPresent());
  }

  @Test
  public void testNullModeDeleteNestedTwice() {
    ValueMapper mapper =
        new TestValueMapper(DEFAULT_COLUMN_FAMILY, DEFAULT_COLUMN, NullValueMode.DELETE, false);
    MutationDataBuilder mutationDataBuilder =
        getRecordMutationDataBuilder(mapper, getStructWithNullOnNthNestingLevel(2));
    verify(mutationDataBuilder, times(1))
        .deleteCells(
            NESTED_NULL_STRUCT_FIELD_NAME, NESTED_NULL_STRUCT_FIELD_NAME_BYTES, TIMESTAMP_RANGE);
    assertTotalNumberOfInvocations(mutationDataBuilder, 1);
    assertTrue(mutationDataBuilder.maybeBuild(TARGET_TABLE_NAME, ROW_KEY).isPresent());
  }

  @Test
  public void testNullModeNestedThrice() {
    ValueMapper mapper =
        new TestValueMapper(DEFAULT_COLUMN_FAMILY, DEFAULT_COLUMN, NullValueMode.IGNORE, false);
    String expectedJsonification = "{\"struct\":null}";
    ByteString expectedJsonificationBytes =
        ByteString.copyFrom(expectedJsonification.getBytes(StandardCharsets.UTF_8));
    MutationDataBuilder mutationDataBuilder =
        getRecordMutationDataBuilder(mapper, getStructWithNullOnNthNestingLevel(3));
    verify(mutationDataBuilder, times(1))
        .setCell(
            NESTED_NULL_STRUCT_FIELD_NAME,
            NESTED_NULL_STRUCT_FIELD_NAME_BYTES,
            TIMESTAMP,
            expectedJsonificationBytes);
    assertTotalNumberOfInvocations(mutationDataBuilder, 1);
    assertTrue(mutationDataBuilder.maybeBuild(TARGET_TABLE_NAME, ROW_KEY).isPresent());
  }

  @Test
  public void testDefaultColumnFamilySubstitution() {
    ValueMapper mapper =
        new TestValueMapper(
            ConfigInterpolation.TOPIC_PLACEHOLDER, DEFAULT_COLUMN, NullValueMode.WRITE, false);
    MutationDataBuilder mutationDataBuilder = getRecordMutationDataBuilder(mapper, null);
    verify(mutationDataBuilder, times(1))
        .setCell(DEFAULT_TOPIC, DEFAULT_COLUMN_BYTES, TIMESTAMP, ByteString.empty());
    assertTotalNumberOfInvocations(mutationDataBuilder, 1);
    assertTrue(mutationDataBuilder.maybeBuild(TARGET_TABLE_NAME, ROW_KEY).isPresent());
  }

  @Test
  public void testExpandRootLevelArrays() throws JsonProcessingException {
    ValueMapper mapper =
        new TestValueMapper(
            "cf", "", NullValueMode.WRITE, true);

    Schema productSchema = SchemaBuilder.struct()
        .field("name", Schema.STRING_SCHEMA)
        .field("id", Schema.STRING_SCHEMA)
        .field("quantity", Schema.INT32_SCHEMA)
        .build();

    Schema schema = SchemaBuilder.struct().optional()
        .field("orderId", Schema.STRING_SCHEMA)
        .field("userId", Schema.STRING_SCHEMA)
        .field("products", SchemaBuilder.array(productSchema).build())
        .build();
    Struct productElement1 = new Struct(productSchema)
        .put("name", "Ball")
        .put("id", "PROD-123")
        .put("quantity", 5);
    Struct productElement2 = new Struct(productSchema)
        .put("name", "Car")
        .put("id", "PROD-456")
        .put("quantity", 1);
    Struct productElement3 = new Struct(productSchema)
        .put("name", "Tambourine")
        .put("id", "PROD-789")
        .put("quantity", 2);

    // use more than 9 elements to we can see what happens when the column qualifier is > 1 digits
    List<Struct> productList = Arrays.stream(new Struct[]{
        // 1
        productElement1,
        // 2
        productElement2,
        // 3
        productElement3,
        // 4
        productElement3,
        // 5
        productElement3,
        // 6
        productElement3,
        // 7
        productElement3,
        // 8
        productElement3,
        // 9
        productElement3,
        // 10
        productElement3,
        // 11
        productElement3,
        // 12
        productElement2
    }).toList();

    Struct value = new Struct(schema)
        .put("orderId", "ORD-999")
        .put("userId", "USER-42")
        .put("products", productList);


    MutationDataBuilder mutationDataBuilder = getRecordMutationDataBuilder(mapper, value);

    Optional<MutationData> mutationData = mutationDataBuilder.maybeBuild("my_table", ROW_KEY);
    assertTrue(mutationData.isPresent());
    String proto = ProtoUtil.toProto(mutationData.get()).replace("\n", "");

    JsonNode actual = new ObjectMapper().readTree(proto);
    assertEquals("projects/project/instances/instance/tables/my_table", actual.get("tableName").asText());
    assertEquals(ROW_KEY.toString(StandardCharsets.UTF_8), ProtoUtil.fromBase64(actual.get("rowKey").asText()));
    ArrayNode mutations = (ArrayNode) actual.get("mutations");
    assertEquals(15, mutations.size());
    assertEquals("cf", mutations.get(0).get("setCell").get("familyName").textValue());
    assertEquals("orderId", ProtoUtil.fromBase64(mutations.get(0).get("setCell").get("columnQualifier").textValue()));
    assertEquals(ProtoUtil.toBase64("ORD-999"), mutations.get(0).get("setCell").get("value").textValue());

    assertEquals("cf", mutations.get(1).get("setCell").get("familyName").textValue());
    assertEquals("userId", ProtoUtil.fromBase64(mutations.get(1).get("setCell").get("columnQualifier").textValue()));
    assertEquals(ProtoUtil.toBase64("USER-42"), mutations.get(1).get("setCell").get("value").textValue());

    // products
    assertEquals("products", mutations.get(2).get("deleteFromFamily").get("familyName").textValue());

    assertEquals("products", mutations.get(3).get("setCell").get("familyName").textValue());
    assertEquals("000000", ProtoUtil.fromBase64(mutations.get(3).get("setCell").get("columnQualifier").textValue()));
    JsonMapper jsonMapper = new JsonMapper();

    JsonNode product1Json = jsonMapper.readTree(ProtoUtil.fromBase64(mutations.get(3).get("setCell").get("value").textValue()));
    assertEquals("Ball", product1Json.get("name").asText());
    assertEquals("PROD-123", product1Json.get("id").asText());
    assertEquals(5, product1Json.get("quantity").asInt());

    assertEquals("000001", ProtoUtil.fromBase64(mutations.get(4).get("setCell").get("columnQualifier").textValue()));
    JsonNode product2Json = jsonMapper.readTree(ProtoUtil.fromBase64(mutations.get(4).get("setCell").get("value").textValue()));
    assertEquals("Car", product2Json.get("name").asText());
    assertEquals("PROD-456", product2Json.get("id").asText());
    assertEquals(1, product2Json.get("quantity").asInt());

    assertEquals("000002", ProtoUtil.fromBase64(mutations.get(5).get("setCell").get("columnQualifier").textValue()));
    JsonNode product3Json = jsonMapper.readTree(ProtoUtil.fromBase64(mutations.get(5).get("setCell").get("value").textValue()));
    assertEquals("Tambourine", product3Json.get("name").asText());
    assertEquals("PROD-789", product3Json.get("id").asText());
    assertEquals(2, product3Json.get("quantity").asInt());

    assertEquals("000011", ProtoUtil.fromBase64(mutations.get(14).get("setCell").get("columnQualifier").textValue()));
    JsonNode product12Json = jsonMapper.readTree(ProtoUtil.fromBase64(mutations.get(5).get("setCell").get("value").textValue()));
    assertEquals("Tambourine", product12Json.get("name").asText());
    assertEquals("PROD-789", product12Json.get("id").asText());
    assertEquals(2, product12Json.get("quantity").asInt());
  }

  private MutationDataBuilder getRecordMutationDataBuilder(ValueMapper mapper, Object kafkaValue) {
    return getRecordMutationDataBuilder(mapper, kafkaValue, DEFAULT_TOPIC);
  }

  private MutationDataBuilder getRecordMutationDataBuilder(
      ValueMapper mapper, Object kafkaValue, String topic) {
    // We use `null` in this test since our code for now uses  Schema only to warn the user when an
    // unsupported logical type is encountered.
    SchemaAndValue schemaAndValue = new SchemaAndValue(null, kafkaValue);
    return mapper.getRecordMutationDataBuilder(schemaAndValue, topic, TIMESTAMP);
  }

  private static class TestValueMapper extends ValueMapper {

    public TestValueMapper(
        String defaultColumnFamily, String defaultColumnQualifier, NullValueMode nullMode, boolean expandRootLevelArrays) {
      super(defaultColumnFamily, defaultColumnQualifier, nullMode, expandRootLevelArrays);
    }

    @Override
    protected MutationDataBuilder createMutationDataBuilder() {
      return spy(super.createMutationDataBuilder());
    }
  }
}
