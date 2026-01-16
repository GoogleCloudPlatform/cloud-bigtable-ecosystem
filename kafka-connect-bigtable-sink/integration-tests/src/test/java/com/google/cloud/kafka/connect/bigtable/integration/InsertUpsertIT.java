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
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.google.cloud.bigtable.data.v2.models.Row;
import com.google.cloud.bigtable.data.v2.models.RowCell;
import com.google.cloud.kafka.connect.bigtable.config.BigtableErrorMode;
import com.google.cloud.kafka.connect.bigtable.config.InsertMode;
import com.google.cloud.kafka.connect.bigtable.mapping.ByteUtils;
import com.google.cloud.kafka.connect.bigtable.transformations.FlattenArrayElement;
import com.google.protobuf.ByteString;
import org.apache.kafka.connect.data.Schema;
import org.apache.kafka.connect.data.SchemaBuilder;
import org.apache.kafka.connect.data.Struct;
import org.apache.kafka.connect.errors.DataException;
import org.apache.kafka.connect.json.JsonConverter;
import org.apache.kafka.connect.storage.StringConverter;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;

import java.nio.charset.StandardCharsets;
import java.util.*;
import java.util.concurrent.ExecutionException;
import java.util.stream.Collectors;

import static com.google.cloud.kafka.connect.bigtable.config.BigtableSinkConfig.*;
import static org.apache.kafka.connect.runtime.WorkerConfig.KEY_CONVERTER_CLASS_CONFIG;
import static org.apache.kafka.connect.runtime.WorkerConfig.VALUE_CONVERTER_CLASS_CONFIG;
import static org.junit.Assert.*;

@RunWith(JUnit4.class)
public class InsertUpsertIT extends BaseKafkaConnectBigtableIT {
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
    props.put("transforms.createKey.fields", "orderId,userId");
    props.put(VALUE_CONVERTER_CLASS_CONFIG, JsonConverter.class.getName());

    String testId = startSingleTopicConnector(props);
    createTablesAndColumnFamilies(Map.of(testId, Set.of(testId, "cf")));

    String json = """
        {
          "schema": {
            "type": "struct",
            "name": "com.example.Order",
            "fields": [
              { "field": "orderId", "type": "string", "optional": false },
              { "field": "product", "type": "string", "optional": false },
              { "field": "quantity", "type": "int32", "optional": false }
            ]
          },
          "payload": {
            "orderId": "ORD-12345",
            "product": "ball",
            "quantity": 2
          }
        }""";
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
    props.put("transforms", "createKey,flattenElements");
    props.put("transforms.createKey.type", "org.apache.kafka.connect.transforms.ValueToKey");
    props.put("transforms.createKey.fields", "orderId,userId");
    props.put("value.converter.schemas.enable", "false");
    props.put(VALUE_CONVERTER_CLASS_CONFIG, JsonConverter.class.getName());

    String testId = startSingleTopicConnector(props);
    createTablesAndColumnFamilies(Map.of(testId, Set.of(testId, "cf")));

    String json = """
        {
            "orderId": "ORD-12345",
            "product": "ball",
            "quantity": 2
        }""";
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
    props.put("transforms", "createKey,flattenElements");
    props.put("transforms.createKey.type", "org.apache.kafka.connect.transforms.ValueToKey");
    props.put("transforms.createKey.fields", "orderId,userId");
    props.put(VALUE_CONVERTER_CLASS_CONFIG, JsonConverter.class.getName());
    configureDlq(props, dlqTopic);
    String testId = startSingleTopicConnector(props);
    createTablesAndColumnFamilies(Map.of(testId, Set.of(testId, "cf")));

    String json = """
        {
          "schema": {
            "type": "struct",
            "name": "com.example.Order",
            "fields": [
              { "field": "orderId", "type": "string", "optional": false },
              { "field": "product", "type": "string", "optional": false },
              { "field": "quantity", "type": "int32", "optional": false }
            ]
          },
          "payload": {
            "orderId": "ORD-12345",
            "product": "ball",
            "quantity": 2
          }
        }""";
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
    props.put("transforms", "createKey,flattenElements");
    props.put("transforms.createKey.type", "org.apache.kafka.connect.transforms.ValueToKey");
    props.put("transforms.createKey.fields", "orderId,userId");
    props.put(VALUE_CONVERTER_CLASS_CONFIG, JsonConverter.class.getName());
    configureDlq(props, dlqTopic);
    String testId = startSingleTopicConnector(props);
    createTablesAndColumnFamilies(Map.of(testId, Set.of(testId, "cf")));

    String json = """
        {
          "schema": {
            "type": "struct",
            "name": "com.example.Order",
            "fields": [
              { "field": "orderId", "type": "string", "optional": false },
              { "field": "product", "type": "string", "optional": true },
              { "field": "quantity", "type": "int32", "optional": false }
            ]
          },
          "payload": {
            "orderId": "ORD-12345",
            "product": null,
            "quantity": 2
          }
        }""";
    connect.kafka().produce(testId, KEY1, json);

    // null key values aren't allowed
    assertSingleDlqEntry(dlqTopic, KEY1, json, DataException.class);
  }

  @Test
  public void testUpsertWithRowKeyWithRootLevelArray() throws InterruptedException, ExecutionException, JsonProcessingException {
    Map<String, String> props = baseConnectorProps();
    props.put(INSERT_MODE_CONFIG, InsertMode.UPSERT.name());
    props.put(ROW_KEY_DEFINITION_CONFIG, "userId,orderId");
    props.put("transforms", "createKey,flattenElements");
    props.put("transforms.createKey.type", "org.apache.kafka.connect.transforms.ValueToKey");
    props.put("transforms.createKey.fields", "orderId,userId");
    props.put("transforms.flatten.type", "org.apache.kafka.connect.transforms.ExtractField$Value");
    props.put("transforms.flatten.field", "products");
    props.put("transforms.rename.type", "org.apache.kafka.connect.transforms.ReplaceField$Value");
    props.put("transforms.rename.renames", "list:products");
    props.put("transforms.flattenElements.type", FlattenArrayElement.class.getName());
    props.put("transforms.flattenElements." + FlattenArrayElement.ARRAY_FIELD_NAME, "products");
    props.put("transforms.flattenElements." + FlattenArrayElement.ARRAY_INNER_WRAPPER_FIELD_NAME, "list");
    props.put("transforms.flattenElements." + FlattenArrayElement.ARRAY_ELEMENT_WRAPPER_FIELD_NAME, "element");

    props.put(ROW_KEY_DELIMITER_CONFIG, "#");
    props.put(DEFAULT_COLUMN_FAMILY_CONFIG, "cf");
    props.put(ERROR_MODE_CONFIG, BigtableErrorMode.FAIL.name());
    props.put(KEY_CONVERTER_CLASS_CONFIG, StringConverter.class.getName());
    props.put(VALUE_CONVERTER_CLASS_CONFIG, JsonConverter.class.getName());

    String testId = startSingleTopicConnector(props);
    createTablesAndColumnFamilies(Map.of(testId, Set.of(testId, "cf", "products")));

    Schema productSchema = SchemaBuilder.struct()
        .field("name", Schema.STRING_SCHEMA)
        .field("id", Schema.STRING_SCHEMA)
        .field("quantity", Schema.INT32_SCHEMA)
        .build();

    Schema elementSchema = SchemaBuilder.struct().field("element", productSchema).build();

    Schema schema = SchemaBuilder.struct().optional()
        .field("orderId", Schema.STRING_SCHEMA)
        .field("userId", Schema.STRING_SCHEMA)
        .field("products",
            SchemaBuilder.struct().field("list", SchemaBuilder.array(elementSchema)).build()
        )
        .build();

    JsonConverter converter = new JsonConverter();
    converter.configure(Collections.singletonMap("schemas.enable", "true"), false);

    Struct productElement1 = new Struct(elementSchema).put("element", new Struct(productSchema)
        .put("name", "Ball")
        .put("id", "PROD-123")
        .put("quantity", 5)
    );
    Struct productElement2 = new Struct(elementSchema).put("element", new Struct(productSchema)
        .put("name", "Car")
        .put("id", "PROD-456")
        .put("quantity", 1)
    );
    Struct productElement3 = new Struct(elementSchema).put("element", new Struct(productSchema)
        .put("name", "Tambourine")
        .put("id", "PROD-789")
        .put("quantity", 2)
    );

    List<Struct> productList = Arrays.stream(new Struct[]{productElement1, productElement2, productElement3}).toList();

    Struct productsWrapper = new Struct(schema.field("products").schema())
        .put("list", productList);

    Struct value = new Struct(schema)
        .put("orderId", "ORD-999")
        .put("userId", "USER-42")
        .put("products", productsWrapper);


    byte[] schemaAsJson = converter.fromConnectData(testId, schema, value);
    System.out.println(new String(schemaAsJson));

    connect.kafka().produce(testId, KEY1, new String(schemaAsJson));

    waitUntilBigtableContainsNumberOfRows(testId, 1);
    Map<ByteString, Row> rows = readAllRows(bigtableData, testId);
    ByteString key = ByteString.copyFrom("USER-42#ORD-999".getBytes(StandardCharsets.UTF_8));
    Row row1 = rows.get(key);
    assertNotNull(row1);
    assertEquals(3, row1.getCells().size());

    List<RowCell> orderIdCells = row1.getCells("cf", "orderId");
    assertEquals(1, orderIdCells.size());
    assertEquals("ORD-999", orderIdCells.get(0).getValue().toString(StandardCharsets.UTF_8));

    List<RowCell> userIdCells = row1.getCells("cf", "userId");
    assertEquals(1, userIdCells.size());
    assertEquals("USER-42", userIdCells.get(0).getValue().toString(StandardCharsets.UTF_8));

    List<RowCell> productsCells = row1.getCells("cf", "products");
    ObjectMapper mapper = new ObjectMapper();
    ArrayNode productsJson = (ArrayNode) mapper.readTree(productsCells.get(0).getValue().toString(StandardCharsets.UTF_8));
    assertEquals(3, productsJson.size());

    // product 1
    assertEquals("Ball", productsJson.get(0).get("name").asText());
    assertEquals("PROD-123", productsJson.get(0).get("id").asText());
    assertEquals(5, productsJson.get(0).get("quantity").asInt());

    // product 2
    assertEquals("Car", productsJson.get(1).get("name").asText());
    assertEquals("PROD-456", productsJson.get(1).get("id").asText());
    assertEquals(1, productsJson.get(1).get("quantity").asInt());

    // product 3
    assertEquals("Tambourine", productsJson.get(2).get("name").asText());
    assertEquals("PROD-789", productsJson.get(2).get("id").asText());
    assertEquals(2, productsJson.get(2).get("quantity").asInt());
  }

  public JsonNode parseJson(String json) throws JsonProcessingException {
    ObjectMapper mapper = new ObjectMapper();
    return mapper.readTree(json);
  }

  public String minifyJson(String json) throws JsonProcessingException {
    ObjectMapper mapper = new ObjectMapper();
    Object obj = mapper.readValue(json, Object.class);
    return mapper.writeValueAsString(obj);
  }
}
