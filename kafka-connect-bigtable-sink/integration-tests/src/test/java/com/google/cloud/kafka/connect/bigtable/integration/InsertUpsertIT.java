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

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.google.cloud.bigtable.data.v2.models.Row;
import com.google.cloud.bigtable.data.v2.models.RowCell;
import com.google.cloud.kafka.connect.bigtable.config.BigtableErrorMode;
import com.google.cloud.kafka.connect.bigtable.config.InsertMode;
import com.google.cloud.kafka.connect.bigtable.config.NullValueMode;
import com.google.cloud.kafka.connect.bigtable.mapping.ByteUtils;
import com.google.cloud.kafka.connect.bigtable.transformations.ApplyJsonSchema;
import com.google.cloud.kafka.connect.bigtable.transformations.FlattenArrayElement;
import com.google.cloud.kafka.connect.bigtable.util.TestDataUtil;
import com.google.protobuf.ByteString;
import org.apache.kafka.connect.errors.DataException;
import org.apache.kafka.connect.json.JsonConverter;
import org.apache.kafka.connect.storage.StringConverter;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;

import java.io.IOException;
import java.io.InputStream;
import java.nio.charset.StandardCharsets;
import java.time.Instant;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.ExecutionException;
import java.util.stream.Collectors;

import static com.google.cloud.kafka.connect.bigtable.config.BigtableSinkConfig.*;
import static org.apache.kafka.connect.runtime.WorkerConfig.KEY_CONVERTER_CLASS_CONFIG;
import static org.apache.kafka.connect.runtime.WorkerConfig.VALUE_CONVERTER_CLASS_CONFIG;
import static org.junit.Assert.*;

@RunWith(JUnit4.class)
public class InsertUpsertIT extends BaseKafkaConnectBigtableIT {
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
  private static final ByteString KEY1_BYTES =
      ByteString.copyFrom(KEY1.getBytes(StandardCharsets.UTF_8));
  private static final ByteString KEY2_BYTES =
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
  public void testUpsertWithRowKeyFromValueNoSchema() throws InterruptedException, ExecutionException, JsonProcessingException {
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
    assertEquals(parseJson(json), parseJson(cells.get(0).getValue().toString(StandardCharsets.UTF_8)));
  }

  @Test
  public void testUpsertWithRowKeyFromValueMissingField() throws InterruptedException, ExecutionException {
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
  public void testUpsertWithRowKeyFromValueNullValue() throws InterruptedException, ExecutionException {
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
  public void testUpsertAppliedSchema() throws InterruptedException, ExecutionException, JsonProcessingException {
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
    props.put("transforms.flattenElements." + FlattenArrayElement.ARRAY_INNER_WRAPPER_FIELD_NAME, "list");
    props.put("transforms.flattenElements." + FlattenArrayElement.ARRAY_ELEMENT_WRAPPER_FIELD_NAME, "element");
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
    assertArrayEquals(new TestDataUtil.OrderProduct[]{
        new TestDataUtil.OrderProduct("Ball", "PROD-123", 5),
        new TestDataUtil.OrderProduct("Car", "PROD-456", 1),
        new TestDataUtil.OrderProduct("Tambourine", "PROD-789", 2)
    }, orderResult.products());
  }

  @Test
  public void testUpsertWithRowKeyWithRootLevelArrayExpanded() throws InterruptedException, ExecutionException, JsonProcessingException {
    Map<String, String> props = baseConnectorProps();
    props.put(INSERT_MODE_CONFIG, InsertMode.UPSERT.name());
    props.put(ROW_KEY_DEFINITION_CONFIG, "userId,orderId");
    props.put(ROW_KEY_DELIMITER_CONFIG, "#");
    props.put("transforms", "createKey,flattenElements");
    props.put("transforms.createKey.type", "org.apache.kafka.connect.transforms.ValueToKey");
    props.put("transforms.createKey.fields", "userId,orderId");
    props.put("transforms.flattenElements.type", FlattenArrayElement.class.getName());
    props.put("transforms.flattenElements." + FlattenArrayElement.ARRAY_FIELD_NAME, "products");
    props.put("transforms.flattenElements." + FlattenArrayElement.ARRAY_INNER_WRAPPER_FIELD_NAME, "list");
    props.put("transforms.flattenElements." + FlattenArrayElement.ARRAY_ELEMENT_WRAPPER_FIELD_NAME, "element");

    props.put(EXPAND_ROOT_LEVEL_ARRAYS, "true");
    props.put(DEFAULT_COLUMN_FAMILY_CONFIG, "cf");
    props.put(ERROR_MODE_CONFIG, BigtableErrorMode.FAIL.name());
    props.put(KEY_CONVERTER_CLASS_CONFIG, StringConverter.class.getName());
    props.put(VALUE_CONVERTER_CLASS_CONFIG, JsonConverter.class.getName());

    String testId = startSingleTopicConnector(props);
    createTablesAndColumnFamilies(Map.of(testId, Set.of("cf", "products")));

    TestDataUtil.Order order1 = new TestDataUtil.Order("ORD-999", "USER-42", new TestDataUtil.OrderProduct[]{
        new TestDataUtil.OrderProduct("Ball", "PROD-123", 5),
        new TestDataUtil.OrderProduct("Car", "PROD-456", 1),
        new TestDataUtil.OrderProduct("Tambourine", "PROD-789", 2)
    });

    TestDataUtil.writeOrder(connect, testId, KEY1, order1);

    waitUntilBigtableContainsNumberOfRows(testId, 1);
    assertArrayEquals(new String[]{"USER-42#ORD-999"}, readAllRowKeys(bigtableData, testId));
    Map<ByteString, Row> rows = readAllRows(bigtableData, testId);
    ByteString key = ByteString.copyFrom("USER-42#ORD-999".getBytes(StandardCharsets.UTF_8));
    Row row1 = rows.get(key);
    assertNotNull(row1);

    TestDataUtil.Order orderResult = TestDataUtil.extractExpandedOrderFromRow(row1);

    assertEquals(order1.orderId(), orderResult.orderId());
    assertEquals(order1.userId(), orderResult.userId());
    assertArrayEquals(order1.products(), orderResult.products());

    // overwrite the first order with a smaller products collection to ensure the existing array is deleted
    TestDataUtil.Order order2 = new TestDataUtil.Order(order1.orderId(), order1.userId(), new TestDataUtil.OrderProduct[]{
        new TestDataUtil.OrderProduct("Drum", "PROD-ABC", 1),
        new TestDataUtil.OrderProduct("Balloons", "PROD-DEF", 5),
    });

    Instant writeTime2 = Instant.now();
    TestDataUtil.writeOrder(connect, testId, KEY1, order2);

    waitUntilBigtableWriteTimeLaterThan(testId, writeTime2);
    assertArrayEquals(new String[]{"USER-42#ORD-999"}, readAllRowKeys(bigtableData, testId));
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
  public void testRootLevelArrayExpandedWriteNull() throws InterruptedException, ExecutionException, JsonProcessingException {
    Map<String, String> props = baseConnectorProps();
    props.put(INSERT_MODE_CONFIG, InsertMode.UPSERT.name());
    props.put(ROW_KEY_DEFINITION_CONFIG, "userId,orderId");
    props.put(ROW_KEY_DELIMITER_CONFIG, "#");
    props.put("transforms", "createKey,flattenElements");
    props.put("transforms.createKey.type", "org.apache.kafka.connect.transforms.ValueToKey");
    props.put("transforms.createKey.fields", "userId,orderId");
    props.put("transforms.flattenElements.type", FlattenArrayElement.class.getName());
    props.put("transforms.flattenElements." + FlattenArrayElement.ARRAY_FIELD_NAME, "products");
    props.put("transforms.flattenElements." + FlattenArrayElement.ARRAY_INNER_WRAPPER_FIELD_NAME, "list");
    props.put("transforms.flattenElements." + FlattenArrayElement.ARRAY_ELEMENT_WRAPPER_FIELD_NAME, "element");

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
    assertArrayEquals(new String[]{"USER-42#ORD-999"}, readAllRowKeys(bigtableData, testId));
    Map<ByteString, Row> rows = readAllRows(bigtableData, testId);
    ByteString key = ByteString.copyFrom("USER-42#ORD-999".getBytes(StandardCharsets.UTF_8));
    Row row1 = rows.get(key);
    assertNotNull(row1);

    TestDataUtil.Order orderResult = TestDataUtil.extractExpandedOrderFromRow(row1);

    assertEquals(order1.orderId(), orderResult.orderId());
    assertEquals(order1.userId(), orderResult.userId());
    assertArrayEquals(new TestDataUtil.OrderProduct[0], orderResult.products());
  }

  public JsonNode parseJson(String json) throws JsonProcessingException {
    return objectMapper.readTree(json);
  }
}
