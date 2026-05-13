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

import static com.google.cloud.kafka.connect.bigtable.config.BigtableSinkConfig.*;
import static org.apache.kafka.connect.runtime.WorkerConfig.KEY_CONVERTER_CLASS_CONFIG;
import static org.apache.kafka.connect.runtime.WorkerConfig.VALUE_CONVERTER_CLASS_CONFIG;
import static org.junit.Assert.assertArrayEquals;
import static org.junit.Assert.assertEquals;
import static org.junit.Assert.assertNotNull;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.google.cloud.bigtable.data.v2.models.Row;
import com.google.cloud.bigtable.data.v2.models.RowCell;
import com.google.cloud.kafka.connect.bigtable.config.BigtableErrorMode;
import com.google.cloud.kafka.connect.bigtable.config.BigtableSinkConfig;
import com.google.cloud.kafka.connect.bigtable.config.InsertMode;
import com.google.cloud.kafka.connect.bigtable.config.NullValueMode;
import com.google.cloud.kafka.connect.bigtable.transformations.ApplyJsonSchema;
import com.google.cloud.kafka.connect.bigtable.transformations.FlattenArrayElement;
import com.google.cloud.kafka.connect.bigtable.util.JsonConverterFactory;
import com.google.cloud.kafka.connect.bigtable.util.TestDataUtil;
import com.google.cloud.kafka.connect.bigtable.utils.ByteUtils;
import com.google.protobuf.ByteString;
import java.io.IOException;
import java.io.InputStream;
import java.nio.charset.StandardCharsets;
import java.time.Instant;
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
import org.apache.kafka.connect.errors.DataException;
import org.apache.kafka.connect.json.JsonConverter;
import org.apache.kafka.connect.runtime.ConnectorConfig;
import org.apache.kafka.connect.storage.Converter;
import org.apache.kafka.connect.storage.StringConverter;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;

@RunWith(JUnit4.class)
public class InsertModeIT extends BaseKafkaConnectBigtableIT {
  private static final ObjectMapper objectMapper = new ObjectMapper();

  private String readResource(String path) {
    try (InputStream is = getClass().getClassLoader().getResourceAsStream(path)) {
      if (is == null) {
        throw new IllegalArgumentException("Resource not found: " + path);
      }
      return new String(is.readAllBytes(), StandardCharsets.UTF_8);
    } catch (IOException e) {
      throw new RuntimeException(e);
    }
  }

  private static final String KEY1 = "key1";
  private static final String KEY2 = "key2";
  private static final String KEY3 = "key3";
  private static final ByteString KEY1_BYTES =
      ByteString.copyFrom(KEY1.getBytes(StandardCharsets.UTF_8));
  private static final ByteString KEY2_BYTES =
      ByteString.copyFrom(KEY2.getBytes(StandardCharsets.UTF_8));
  private static final ByteString KEY3_BYTES =
      ByteString.copyFrom(KEY3.getBytes(StandardCharsets.UTF_8));
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
    props.put(INSERT_MODE_CONFIG, InsertMode.INSERT.name());
    props.put(ERROR_MODE_CONFIG, BigtableErrorMode.IGNORE.name());
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
    props.put(INSERT_MODE_CONFIG, InsertMode.UPSERT.name());
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
  public void testUpsertWithRowKeyFromValue() throws InterruptedException, ExecutionException {
    Map<String, String> props = baseConnectorProps();
    props.put(INSERT_MODE_CONFIG, InsertMode.UPSERT.name());
    props.put(ROW_KEY_DEFINITION_CONFIG, "orderId,product");
    props.put(ROW_KEY_DELIMITER_CONFIG, "#");
    props.put(DEFAULT_COLUMN_FAMILY_CONFIG, "cf");
    props.put(ERROR_MODE_CONFIG, BigtableErrorMode.FAIL.name());
    props.put("transforms", "createKey");
    props.put("transforms.createKey.type", "org.apache.kafka.connect.transforms.ValueToKey");
    props.put("transforms.createKey.fields", "orderId,product");
    props.put(VALUE_CONVERTER_CLASS_CONFIG, JsonConverter.class.getName());

    String testId = startSingleTopicConnector(props);
    createTablesAndColumnFamilies(Map.of(testId, Set.of("cf")));

    String json = readResource("json/order-with-schema.json");
    connect.kafka().produce(testId, KEY1, json);

    waitUntilBigtableContainsNumberOfRows(testId, 1);
    Map<ByteString, Row> rows = readAllRows(bigtableData, testId);
    ByteString key = ByteString.copyFrom("ORD-12345#ball".getBytes(StandardCharsets.UTF_8));
    Row row1 = rows.get(key);
    assertNotNull(row1);
    assertEquals(3, row1.getCells().size());

    List<RowCell> orderIdCells = row1.getCells("cf", "orderId");
    assertEquals(1, orderIdCells.size());
    assertEquals("ORD-12345", orderIdCells.get(0).getValue().toString(StandardCharsets.UTF_8));

    List<RowCell> productCells = row1.getCells("cf", "product");
    assertEquals(1, productCells.size());
    assertEquals("ball", productCells.get(0).getValue().toString(StandardCharsets.UTF_8));

    List<RowCell> quantityCells = row1.getCells("cf", "quantity");
    assertEquals(1, quantityCells.size());
    assertArrayEquals(ByteUtils.toBytes(2), quantityCells.get(0).getValue().toByteArray());
  }

  @Test
  public void testUpsertWithRowKeyFromValueNoSchema()
      throws InterruptedException, ExecutionException, JsonProcessingException {
    Map<String, String> props = baseConnectorProps();
    props.put(INSERT_MODE_CONFIG, InsertMode.UPSERT.name());
    props.put(ROW_KEY_DEFINITION_CONFIG, "orderId,product");
    props.put(ROW_KEY_DELIMITER_CONFIG, "#");
    props.put(DEFAULT_COLUMN_FAMILY_CONFIG, "cf");
    props.put(ERROR_MODE_CONFIG, BigtableErrorMode.FAIL.name());
    props.put("transforms", "createKey");
    props.put("transforms.createKey.type", "org.apache.kafka.connect.transforms.ValueToKey");
    props.put("transforms.createKey.fields", "orderId,product");
    props.put("value.converter.schemas.enable", "false");
    props.put(VALUE_CONVERTER_CLASS_CONFIG, JsonConverter.class.getName());

    String testId = startSingleTopicConnector(props);
    createTablesAndColumnFamilies(Map.of(testId, Set.of("cf")));

    String json = readResource("json/order-no-schema.json");
    connect.kafka().produce(testId, KEY1, json);

    waitUntilBigtableContainsNumberOfRows(testId, 1);
    Map<ByteString, Row> rows = readAllRows(bigtableData, testId);
    ByteString key = ByteString.copyFrom("ORD-12345#ball".getBytes(StandardCharsets.UTF_8));
    Row row1 = rows.get(key);
    assertNotNull(row1);
    assertEquals(1, row1.getCells().size());

    List<RowCell> cells = row1.getCells("cf", "KAFKA_VALUE");
    assertEquals(1, cells.size());
    assertEquals(
        parseJson(json), parseJson(cells.get(0).getValue().toString(StandardCharsets.UTF_8)));
  }

  @Test
  public void testUpsertWithRowKeyFromValueMissingField()
      throws InterruptedException, ExecutionException {
    String dlqTopic = createDlq();
    Map<String, String> props = baseConnectorProps();
    props.put(INSERT_MODE_CONFIG, InsertMode.UPSERT.name());
    props.put(ROW_KEY_DEFINITION_CONFIG, "orderId,NO_SUCH_COLUMN");
    props.put(ROW_KEY_DELIMITER_CONFIG, "#");
    props.put(DEFAULT_COLUMN_FAMILY_CONFIG, "cf");
    props.put(ERROR_MODE_CONFIG, BigtableErrorMode.FAIL.name());
    props.put("transforms", "createKey");
    props.put("transforms.createKey.type", "org.apache.kafka.connect.transforms.ValueToKey");
    props.put("transforms.createKey.fields", "orderId,userId");
    props.put(VALUE_CONVERTER_CLASS_CONFIG, JsonConverter.class.getName());
    configureDlq(props, dlqTopic);
    String testId = startSingleTopicConnector(props);
    createTablesAndColumnFamilies(Map.of(testId, Set.of("cf")));

    String json = readResource("json/order-with-schema.json");
    connect.kafka().produce(testId, KEY1, json);

    assertSingleDlqEntry(dlqTopic, KEY1, json, DataException.class);
  }

  @Test
  public void testUpsertWithRowKeyFromValueNullValue()
      throws InterruptedException, ExecutionException {
    String dlqTopic = createDlq();
    Map<String, String> props = baseConnectorProps();
    props.put(INSERT_MODE_CONFIG, InsertMode.UPSERT.name());
    props.put(ROW_KEY_DEFINITION_CONFIG, "orderId,product");
    props.put(ROW_KEY_DELIMITER_CONFIG, "#");
    props.put(DEFAULT_COLUMN_FAMILY_CONFIG, "cf");
    props.put(ERROR_MODE_CONFIG, BigtableErrorMode.FAIL.name());
    props.put("transforms", "createKey");
    props.put("transforms.createKey.type", "org.apache.kafka.connect.transforms.ValueToKey");
    props.put("transforms.createKey.fields", "orderId,product");
    props.put(VALUE_CONVERTER_CLASS_CONFIG, JsonConverter.class.getName());
    configureDlq(props, dlqTopic);
    String testId = startSingleTopicConnector(props);
    createTablesAndColumnFamilies(Map.of(testId, Set.of("cf")));

    String json = readResource("json/order-with-null-product-schema.json");
    connect.kafka().produce(testId, KEY1, json);

    // null key values aren't allowed
    assertSingleDlqEntry(dlqTopic, KEY1, json, DataException.class);
  }

  @Test
  public void testUpsertAppliedSchema()
      throws InterruptedException, ExecutionException, JsonProcessingException {
    Map<String, String> props = baseConnectorProps();
    props.put(INSERT_MODE_CONFIG, InsertMode.UPSERT.name());
    props.put(ROW_KEY_DEFINITION_CONFIG, "userId,orderId");
    props.put("value.converter.schemas.enable", "false");
    props.put("transforms", "applySchema,createKey,flattenElements");
    props.put("transforms.applySchema.type", ApplyJsonSchema.class.getName() + "$Value");
    props.put("transforms.applySchema.schema.json", readResource("json/applied-schema.json"));
    props.put("transforms.createKey.type", "org.apache.kafka.connect.transforms.ValueToKey");
    props.put("transforms.createKey.fields", "userId,orderId");
    props.put("transforms.flattenElements.type", FlattenArrayElement.class.getName());
    props.put("transforms.flattenElements." + FlattenArrayElement.ARRAY_FIELD_NAME, "products");
    props.put(
        "transforms.flattenElements." + FlattenArrayElement.ARRAY_INNER_WRAPPER_FIELD_NAME, "list");
    props.put(
        "transforms.flattenElements." + FlattenArrayElement.ARRAY_ELEMENT_WRAPPER_FIELD_NAME,
        "element");
    props.put(EXPAND_ROOT_LEVEL_ARRAYS, "true");
    props.put(ROW_KEY_DELIMITER_CONFIG, "#");
    props.put(DEFAULT_COLUMN_FAMILY_CONFIG, "cf");
    props.put(ERROR_MODE_CONFIG, BigtableErrorMode.FAIL.name());
    props.put(KEY_CONVERTER_CLASS_CONFIG, StringConverter.class.getName());
    props.put(VALUE_CONVERTER_CLASS_CONFIG, JsonConverter.class.getName());

    String testId = startSingleTopicConnector(props);
    createTablesAndColumnFamilies(Map.of(testId, Set.of("cf", "products")));

    String json = readResource("json/expanded-order.json");
    connect.kafka().produce(testId, KEY1, json);

    waitUntilBigtableContainsNumberOfRows(testId, 1);
    Map<ByteString, Row> rows = readAllRows(bigtableData, testId);
    ByteString key = ByteString.copyFrom("USER-42#ORD-999".getBytes(StandardCharsets.UTF_8));
    Row row1 = rows.get(key);
    assertNotNull(row1);

    TestDataUtil.Order orderResult = TestDataUtil.extractExpandedOrderFromRow(row1);

    assertEquals("ORD-999", orderResult.orderId());
    assertEquals("USER-42", orderResult.userId());
    assertArrayEquals(
        new TestDataUtil.OrderProduct[] {
          new TestDataUtil.OrderProduct("Ball", "PROD-123", 5),
          new TestDataUtil.OrderProduct("Car", "PROD-456", 1),
          new TestDataUtil.OrderProduct("Tambourine", "PROD-789", 2)
        },
        orderResult.products());
  }

  @Test
  public void testUpsertWithRowKeyWithRootLevelArrayExpanded()
      throws InterruptedException, ExecutionException, JsonProcessingException {
    Map<String, String> props = baseConnectorProps();
    props.put(INSERT_MODE_CONFIG, InsertMode.UPSERT.name());
    props.put(ROW_KEY_DEFINITION_CONFIG, "userId,orderId");
    props.put(ROW_KEY_DELIMITER_CONFIG, "#");
    props.put("transforms", "createKey,flattenElements");
    props.put("transforms.createKey.type", "org.apache.kafka.connect.transforms.ValueToKey");
    props.put("transforms.createKey.fields", "userId,orderId");
    props.put("transforms.flattenElements.type", FlattenArrayElement.class.getName());
    props.put("transforms.flattenElements." + FlattenArrayElement.ARRAY_FIELD_NAME, "products");
    props.put(
        "transforms.flattenElements." + FlattenArrayElement.ARRAY_INNER_WRAPPER_FIELD_NAME, "list");
    props.put(
        "transforms.flattenElements." + FlattenArrayElement.ARRAY_ELEMENT_WRAPPER_FIELD_NAME,
        "element");

    props.put(EXPAND_ROOT_LEVEL_ARRAYS, "true");
    props.put(DEFAULT_COLUMN_FAMILY_CONFIG, "cf");
    props.put(ERROR_MODE_CONFIG, BigtableErrorMode.FAIL.name());
    props.put(KEY_CONVERTER_CLASS_CONFIG, StringConverter.class.getName());
    props.put(VALUE_CONVERTER_CLASS_CONFIG, JsonConverter.class.getName());

    String testId = startSingleTopicConnector(props);
    createTablesAndColumnFamilies(Map.of(testId, Set.of("cf", "products")));

    TestDataUtil.Order order1 =
        new TestDataUtil.Order(
            "ORD-999",
            "USER-42",
            new TestDataUtil.OrderProduct[] {
              new TestDataUtil.OrderProduct("Ball", "PROD-123", 5),
              new TestDataUtil.OrderProduct("Car", "PROD-456", 1),
              new TestDataUtil.OrderProduct("Tambourine", "PROD-789", 2)
            });

    TestDataUtil.writeOrder(connect, testId, KEY1, order1);

    waitUntilBigtableContainsNumberOfRows(testId, 1);
    assertArrayEquals(new String[] {"USER-42#ORD-999"}, readAllRowKeys(bigtableData, testId));
    Map<ByteString, Row> rows = readAllRows(bigtableData, testId);
    ByteString key = ByteString.copyFrom("USER-42#ORD-999".getBytes(StandardCharsets.UTF_8));
    Row row1 = rows.get(key);
    assertNotNull(row1);

    TestDataUtil.Order orderResult = TestDataUtil.extractExpandedOrderFromRow(row1);

    assertEquals(order1.orderId(), orderResult.orderId());
    assertEquals(order1.userId(), orderResult.userId());
    assertArrayEquals(order1.products(), orderResult.products());

    // overwrite the first order with a smaller products collection to ensure the existing array is
    // deleted
    TestDataUtil.Order order2 =
        new TestDataUtil.Order(
            order1.orderId(),
            order1.userId(),
            new TestDataUtil.OrderProduct[] {
              new TestDataUtil.OrderProduct("Drum", "PROD-ABC", 1),
              new TestDataUtil.OrderProduct("Balloons", "PROD-DEF", 5),
            });

    Instant writeTime2 = Instant.now();
    TestDataUtil.writeOrder(connect, testId, KEY1, order2);

    waitUntilBigtableWriteTimeLaterThan(testId, writeTime2);
    assertArrayEquals(new String[] {"USER-42#ORD-999"}, readAllRowKeys(bigtableData, testId));
    Map<ByteString, Row> rows2 = readAllRows(bigtableData, testId);
    Row row2 = rows2.get(key);
    assertNotNull(row2);

    TestDataUtil.Order orderResult2 = TestDataUtil.extractExpandedOrderFromRow(row2);

    assertEquals(order2.orderId(), orderResult2.orderId());
    assertEquals(order2.userId(), orderResult2.userId());
    // must explicitly compare arrays
    assertArrayEquals(order2.products(), orderResult2.products());
  }

  @Test
  public void testRootLevelArrayExpandedWriteNull()
      throws InterruptedException, ExecutionException, JsonProcessingException {
    Map<String, String> props = baseConnectorProps();
    props.put(INSERT_MODE_CONFIG, InsertMode.UPSERT.name());
    props.put(ROW_KEY_DEFINITION_CONFIG, "userId,orderId");
    props.put(ROW_KEY_DELIMITER_CONFIG, "#");
    props.put("transforms", "createKey,flattenElements");
    props.put("transforms.createKey.type", "org.apache.kafka.connect.transforms.ValueToKey");
    props.put("transforms.createKey.fields", "userId,orderId");
    props.put("transforms.flattenElements.type", FlattenArrayElement.class.getName());
    props.put("transforms.flattenElements." + FlattenArrayElement.ARRAY_FIELD_NAME, "products");
    props.put(
        "transforms.flattenElements." + FlattenArrayElement.ARRAY_INNER_WRAPPER_FIELD_NAME, "list");
    props.put(
        "transforms.flattenElements." + FlattenArrayElement.ARRAY_ELEMENT_WRAPPER_FIELD_NAME,
        "element");

    props.put(EXPAND_ROOT_LEVEL_ARRAYS, "true");
    props.put(VALUE_NULL_MODE_CONFIG, NullValueMode.WRITE.name());
    props.put(DEFAULT_COLUMN_FAMILY_CONFIG, "cf");
    props.put(ERROR_MODE_CONFIG, BigtableErrorMode.FAIL.name());
    props.put(KEY_CONVERTER_CLASS_CONFIG, StringConverter.class.getName());
    props.put(VALUE_CONVERTER_CLASS_CONFIG, JsonConverter.class.getName());

    String testId = startSingleTopicConnector(props);
    createTablesAndColumnFamilies(Map.of(testId, Set.of("cf", "products")));

    TestDataUtil.Order order1 = new TestDataUtil.Order("ORD-999", "USER-42", null);

    TestDataUtil.writeOrder(connect, testId, KEY1, order1);

    waitUntilBigtableContainsNumberOfRows(testId, 1);
    assertArrayEquals(new String[] {"USER-42#ORD-999"}, readAllRowKeys(bigtableData, testId));
    Map<ByteString, Row> rows = readAllRows(bigtableData, testId);
    ByteString key = ByteString.copyFrom("USER-42#ORD-999".getBytes(StandardCharsets.UTF_8));
    Row row1 = rows.get(key);
    assertNotNull(row1);

    TestDataUtil.Order orderResult = TestDataUtil.extractExpandedOrderFromRow(row1);

    assertEquals(order1.orderId(), orderResult.orderId());
    assertEquals(order1.userId(), orderResult.userId());
    assertArrayEquals(new TestDataUtil.OrderProduct[0], orderResult.products());
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
  // with DELETE null handling mode.
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

  public JsonNode parseJson(String json) throws JsonProcessingException {
    return objectMapper.readTree(json);
  }
}
