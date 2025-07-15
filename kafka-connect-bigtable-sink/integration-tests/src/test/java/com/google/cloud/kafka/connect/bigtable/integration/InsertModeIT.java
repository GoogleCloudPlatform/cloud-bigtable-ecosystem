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
package com.google.cloud.kafka.connect.bigtable.integration;

import static org.junit.Assert.assertEquals;

import com.google.cloud.bigtable.data.v2.models.Row;
import com.google.cloud.bigtable.data.v2.models.RowCell;
import com.google.cloud.kafka.connect.bigtable.config.BigtableErrorMode;
import com.google.cloud.kafka.connect.bigtable.config.BigtableSinkConfig;
import com.google.cloud.kafka.connect.bigtable.config.InsertMode;
import com.google.cloud.kafka.connect.bigtable.config.NullValueMode;
import com.google.cloud.kafka.connect.bigtable.util.JsonConverterFactory;
import com.google.protobuf.ByteString;
import java.nio.charset.StandardCharsets;
import java.util.AbstractMap;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.ExecutionException;
import java.util.stream.Collectors;
import org.apache.kafka.connect.data.Schema;
import org.apache.kafka.connect.data.SchemaAndValue;
import org.apache.kafka.connect.data.SchemaBuilder;
import org.apache.kafka.connect.data.Struct;
import org.apache.kafka.connect.runtime.ConnectorConfig;
import org.apache.kafka.connect.storage.Converter;
import org.apache.kafka.connect.storage.StringConverter;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;

@RunWith(JUnit4.class)
public class InsertModeIT extends BaseKafkaConnectBigtableIT {
  private static final String KEY1 = "key1";
  private static final String KEY2 = "key2";
  private static final String KEY3 = "key3";
  private static final ByteString KEY1_BYTES =
      ByteString.copyFrom(KEY1.getBytes(StandardCharsets.UTF_8));
  private static final ByteString KEY2_BYTES =
      ByteString.copyFrom(KEY2.getBytes(StandardCharsets.UTF_8));
  private static final ByteString KEY3_BYTES =
      ByteString.copyFrom(KEY2.getBytes(StandardCharsets.UTF_8));
  private static final String VALUE1 = "value1";
  private static final String VALUE2 = "value2";
  private static final String VALUE3 = "value3";
  private static final ByteString VALUE1_BYTES =
      ByteString.copyFrom(VALUE1.getBytes(StandardCharsets.UTF_8));
  private static final ByteString VALUE2_BYTES =
      ByteString.copyFrom(VALUE2.getBytes(StandardCharsets.UTF_8));
  private static final ByteString VALUE3_BYTES =
      ByteString.copyFrom(VALUE3.getBytes(StandardCharsets.UTF_8));

  @Test
  public void testInsert() throws InterruptedException, ExecutionException {
    String dlqTopic = createDlq();
    Map<String, String> props = baseConnectorProps();
    props.put(BigtableSinkConfig.INSERT_MODE_CONFIG, InsertMode.INSERT.name());
    props.put(BigtableSinkConfig.ERROR_MODE_CONFIG, BigtableErrorMode.IGNORE.name());
    configureDlq(props, dlqTopic);
    String testId = startSingleTopicConnector(props);
    createTablesAndColumnFamilies(testId);

    connect.kafka().produce(testId, KEY1, VALUE1);
    waitUntilBigtableContainsNumberOfRows(testId, 1);
    connect.kafka().produce(testId, KEY1, VALUE2);
    connect.kafka().produce(testId, KEY2, VALUE3);
    waitUntilBigtableContainsNumberOfRows(testId, 2);
    assertSingleDlqEntry(dlqTopic, KEY1, VALUE2, null);

    Map<ByteString, Row> rows = readAllRows(bigtableData, testId);
    Row row1 = rows.get(KEY1_BYTES);
    Row row2 = rows.get(KEY2_BYTES);
    assertEquals(1, row1.getCells().size());
    assertEquals(VALUE1_BYTES, row1.getCells().get(0).getValue());
    assertEquals(1, row2.getCells().size());
    assertEquals(VALUE3_BYTES, row2.getCells().get(0).getValue());

    assertConnectorAndAllTasksAreRunning(testId);
  }

  @Test
  public void testUpsert() throws InterruptedException, ExecutionException {
    Map<String, String> props = baseConnectorProps();
    props.put(BigtableSinkConfig.INSERT_MODE_CONFIG, InsertMode.UPSERT.name());
    String testId = startSingleTopicConnector(props);
    createTablesAndColumnFamilies(testId);

    connect.kafka().produce(testId, KEY1, VALUE1);
    waitUntilBigtableContainsNumberOfRows(testId, 1);
    connect.kafka().produce(testId, KEY1, VALUE2);
    connect.kafka().produce(testId, KEY2, VALUE3);
    waitUntilBigtableContainsNumberOfRows(testId, 2);

    Map<ByteString, Row> rows = readAllRows(bigtableData, testId);
    Row row1 = rows.get(KEY1_BYTES);
    Row row2 = rows.get(KEY2_BYTES);
    assertEquals(2, row1.getCells().size());
    assertEquals(
        Set.of(VALUE1_BYTES, VALUE2_BYTES),
        row1.getCells().stream().map(RowCell::getValue).collect(Collectors.toSet()));
    assertEquals(1, row2.getCells().size());
    assertEquals(VALUE3_BYTES, row2.getCells().get(0).getValue());
    assertConnectorAndAllTasksAreRunning(testId);
  }

  @Test
  public void testReplaceIfNewestWrites() throws InterruptedException, ExecutionException {
    Converter keyConverter = new StringConverter();
    Converter valueConverter = JsonConverterFactory.create(true, false);

    Map<String, String> props = baseConnectorProps();
    props.put(ConnectorConfig.VALUE_CONVERTER_CLASS_CONFIG, valueConverter.getClass().getName());
    props.put(BigtableSinkConfig.INSERT_MODE_CONFIG, InsertMode.REPLACE_IF_NEWEST.name());
    // Let's ignore `null`s to ensure that we observe `InsertMode.REPLACE_IF_NEWEST`'s behavior
    // rather than `NullValueMode.DELETE`'s.
    props.put(BigtableSinkConfig.VALUE_NULL_MODE_CONFIG, NullValueMode.IGNORE.name());

    String testId = startSingleTopicConnector(props);
    createTablesAndColumnFamilies(testId);

    String field1 = "f1";
    String field2 = "f2";

    Schema schema1 = SchemaBuilder.struct().field(field1, Schema.STRING_SCHEMA).build();
    Schema schema2 = SchemaBuilder.struct().field(field2, Schema.STRING_SCHEMA).build();

    Struct value1 = new Struct(schema1).put(field1, VALUE1);
    Struct value2 = new Struct(schema2).put(field2, VALUE2);

    SchemaAndValue schemaAndValue1 = new SchemaAndValue(value1.schema(), value1);
    SchemaAndValue schemaAndValue2 = new SchemaAndValue(value2.schema(), value2);

    SchemaAndValue schemaAndKey1 = new SchemaAndValue(Schema.STRING_SCHEMA, KEY1);
    SchemaAndValue schemaAndKey2 = new SchemaAndValue(Schema.STRING_SCHEMA, KEY2);
    SchemaAndValue schemaAndKey3 = new SchemaAndValue(Schema.STRING_SCHEMA, KEY3);

    long preexistingRowsTimestamp = 10000L;

    // Set initial values of the preexisting rows.
    sendRecords(
        testId,
        List.of(
            new AbstractMap.SimpleImmutableEntry<>(schemaAndKey1, schemaAndValue1),
            new AbstractMap.SimpleImmutableEntry<>(schemaAndKey2, schemaAndValue1)),
        keyConverter,
        valueConverter,
        preexistingRowsTimestamp);
    waitUntilBigtableContainsNumberOfRows(testId, 2);

    // Successfully try to replace a record using the same timestamp.
    sendRecords(
        testId,
        List.of(new AbstractMap.SimpleImmutableEntry<>(schemaAndKey1, schemaAndValue2)),
        keyConverter,
        valueConverter,
        preexistingRowsTimestamp);
    // Unsuccessfully try to replace a record using an earlier timestamp.
    sendRecords(
        testId,
        List.of(new AbstractMap.SimpleImmutableEntry<>(schemaAndKey2, schemaAndValue2)),
        keyConverter,
        valueConverter,
        preexistingRowsTimestamp - 1L);
    // Test writing to a row that didn't exist before using the lowest possible timestamp.
    sendRecords(
        testId,
        List.of(new AbstractMap.SimpleImmutableEntry<>(schemaAndKey3, schemaAndValue1)),
        keyConverter,
        valueConverter,
        0L);
    waitUntilBigtableContainsNumberOfRows(testId, 3);
    assertConnectorAndAllTasksAreRunning(testId);

    Map<ByteString, Row> rows = readAllRows(bigtableData, testId);
    Row row1 = rows.get(KEY1_BYTES);
    Row row2 = rows.get(KEY2_BYTES);
    Row row3 = rows.get(KEY3_BYTES);
    assertEquals(1, row1.getCells().size());
    assertEquals(VALUE2_BYTES, row1.getCells().get(0).getValue());
    assertEquals(ByteString.copyFromUtf8(field2), row1.getCells().get(0).getQualifier());
    assertEquals(1, row2.getCells().size());
    assertEquals(VALUE1_BYTES, row2.getCells().get(0).getValue());
    assertEquals(ByteString.copyFromUtf8(field1), row2.getCells().get(0).getQualifier());
    assertEquals(1, row3.getCells().size());
    assertEquals(VALUE1_BYTES, row3.getCells().get(0).getValue());
    assertEquals(ByteString.copyFromUtf8(field1), row3.getCells().get(0).getQualifier());
  }

  @Test
  public void testReplaceIfNewestDeletes() throws InterruptedException, ExecutionException {
    Converter keyConverter = new StringConverter();
    Converter valueConverter = new StringConverter();

    Map<String, String> props = baseConnectorProps();
    props.put(ConnectorConfig.VALUE_CONVERTER_CLASS_CONFIG, valueConverter.getClass().getName());
    props.put(BigtableSinkConfig.INSERT_MODE_CONFIG, InsertMode.REPLACE_IF_NEWEST.name());
    // `REPLACE_IF_NEWEST` mode empties the row before setting the new cells irregardless of
    // configured NullValueMode.
    props.put(BigtableSinkConfig.VALUE_NULL_MODE_CONFIG, NullValueMode.IGNORE.name());

    String testId = startSingleTopicConnector(props);
    createTablesAndColumnFamilies(testId);

    SchemaAndValue writeSchemaAndValue = new SchemaAndValue(Schema.STRING_SCHEMA, VALUE1);
    SchemaAndValue deleteSchemaAndValue = new SchemaAndValue(Schema.OPTIONAL_STRING_SCHEMA, null);

    SchemaAndValue schemaAndKey1 = new SchemaAndValue(Schema.STRING_SCHEMA, KEY1);
    SchemaAndValue schemaAndKey2 = new SchemaAndValue(Schema.STRING_SCHEMA, KEY2);
    SchemaAndValue schemaAndKey3 = new SchemaAndValue(Schema.STRING_SCHEMA, KEY3);

    long preexistingRowsTimestamp = 10000L;

    // Set initial values of the preexisting rows.
    sendRecords(
        testId,
        List.of(
            new AbstractMap.SimpleImmutableEntry<>(schemaAndKey1, writeSchemaAndValue),
            new AbstractMap.SimpleImmutableEntry<>(schemaAndKey2, writeSchemaAndValue)),
        keyConverter,
        valueConverter,
        preexistingRowsTimestamp);
    waitUntilBigtableContainsNumberOfRows(testId, 2);

    // Test deleting a row that didn't exist before.
    sendRecords(
        testId,
        List.of(new AbstractMap.SimpleImmutableEntry<>(schemaAndKey3, deleteSchemaAndValue)),
        keyConverter,
        valueConverter,
        preexistingRowsTimestamp);
    // Unsuccessfully try to delete a row using an earlier timestamp.
    sendRecords(
        testId,
        List.of(new AbstractMap.SimpleImmutableEntry<>(schemaAndKey2, deleteSchemaAndValue)),
        keyConverter,
        valueConverter,
        preexistingRowsTimestamp - 1L);
    // Successfully try to delete a row using the same timestamp.
    sendRecords(
        testId,
        List.of(new AbstractMap.SimpleImmutableEntry<>(schemaAndKey1, deleteSchemaAndValue)),
        keyConverter,
        valueConverter,
        preexistingRowsTimestamp);
    waitUntilBigtableContainsNumberOfRows(testId, 1);
    assertConnectorAndAllTasksAreRunning(testId);

    Map<ByteString, Row> rows = readAllRows(bigtableData, testId);
    Row row2 = rows.get(KEY2_BYTES);
    assertEquals(1, row2.getCells().size());
    assertEquals(VALUE1_BYTES, row2.getCells().get(0).getValue());
  }

  // This test ensures that deletion of row caused by REPLACE_IF_NEWEST works well when combined
  // with DELETE
  // null handling mode.
  @Test
  public void testReplaceIfNewestDeletesWorkWithNullDeletes()
      throws InterruptedException, ExecutionException {
    Converter keyConverter = new StringConverter();
    Converter valueConverter = JsonConverterFactory.create(true, false);

    Map<String, String> props = baseConnectorProps();
    props.put(ConnectorConfig.VALUE_CONVERTER_CLASS_CONFIG, valueConverter.getClass().getName());
    props.put(BigtableSinkConfig.INSERT_MODE_CONFIG, InsertMode.REPLACE_IF_NEWEST.name());
    props.put(BigtableSinkConfig.VALUE_NULL_MODE_CONFIG, NullValueMode.DELETE.name());

    String testId = startSingleTopicConnector(props);
    createTablesAndColumnFamilies(testId);

    SchemaAndValue writeSchemaAndValue = new SchemaAndValue(Schema.STRING_SCHEMA, VALUE1);

    SchemaAndValue deleteSchemaAndValue1 = new SchemaAndValue(Schema.OPTIONAL_STRING_SCHEMA, null);

    Schema schema2 =
        SchemaBuilder.struct().optional().field(testId, Schema.OPTIONAL_STRING_SCHEMA).build();
    SchemaAndValue deleteSchemaAndValue2 =
        new SchemaAndValue(schema2, new Struct(schema2).put(testId, null));

    Schema innerSchema3 =
        SchemaBuilder.struct()
            .optional()
            .field("KAFKA_CONNECT", Schema.OPTIONAL_STRING_SCHEMA)
            .build();
    Schema schema3 = SchemaBuilder.struct().optional().field(testId, innerSchema3).build();
    SchemaAndValue deleteSchemaAndValue3 =
        new SchemaAndValue(
            schema3,
            new Struct(schema3).put(testId, new Struct(innerSchema3).put("KAFKA_CONNECT", null)));

    SchemaAndValue schemaAndKey1 = new SchemaAndValue(Schema.STRING_SCHEMA, KEY1);
    SchemaAndValue schemaAndKey2 = new SchemaAndValue(Schema.STRING_SCHEMA, KEY2);
    SchemaAndValue schemaAndKey3 = new SchemaAndValue(Schema.STRING_SCHEMA, KEY3);
    SchemaAndValue nonexistentSchemaAndKey =
        new SchemaAndValue(Schema.STRING_SCHEMA, "nonexistent");

    long lowestPossibleTimestamp = 0L;
    long deleteTimestamp = 10000L;

    // Set initial values of the preexisting rows.
    sendRecords(
        testId,
        List.of(
            new AbstractMap.SimpleImmutableEntry<>(schemaAndKey1, writeSchemaAndValue),
            new AbstractMap.SimpleImmutableEntry<>(schemaAndKey2, writeSchemaAndValue),
            new AbstractMap.SimpleImmutableEntry<>(schemaAndKey3, writeSchemaAndValue)),
        keyConverter,
        valueConverter,
        lowestPossibleTimestamp);
    waitUntilBigtableContainsNumberOfRows(testId, 3);

    // Test that no kind of delete breaks on a nonexistent row.
    sendRecords(
        testId,
        List.of(
            new AbstractMap.SimpleImmutableEntry<>(nonexistentSchemaAndKey, deleteSchemaAndValue1),
            new AbstractMap.SimpleImmutableEntry<>(nonexistentSchemaAndKey, deleteSchemaAndValue2),
            new AbstractMap.SimpleImmutableEntry<>(nonexistentSchemaAndKey, deleteSchemaAndValue3)),
        keyConverter,
        valueConverter,
        deleteTimestamp);

    // Test that all kinds of delete work on existing rows.
    sendRecords(
        testId,
        List.of(
            new AbstractMap.SimpleImmutableEntry<>(schemaAndKey1, deleteSchemaAndValue1),
            new AbstractMap.SimpleImmutableEntry<>(schemaAndKey2, deleteSchemaAndValue2),
            new AbstractMap.SimpleImmutableEntry<>(schemaAndKey3, deleteSchemaAndValue3)),
        keyConverter,
        valueConverter,
        deleteTimestamp);

    waitUntilBigtableContainsNumberOfRows(testId, 0);
    assertConnectorAndAllTasksAreRunning(testId);
  }
}
