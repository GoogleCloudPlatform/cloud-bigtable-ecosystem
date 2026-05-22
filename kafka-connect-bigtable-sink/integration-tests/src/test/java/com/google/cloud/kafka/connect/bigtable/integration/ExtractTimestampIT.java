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
package com.google.cloud.kafka.connect.bigtable.integration;

import static com.google.cloud.kafka.connect.bigtable.config.BigtableSinkConfig.DEFAULT_COLUMN_FAMILY_CONFIG;
import static com.google.cloud.kafka.connect.bigtable.config.BigtableSinkConfig.INSERT_MODE_CONFIG;
import static org.apache.kafka.connect.runtime.WorkerConfig.VALUE_CONVERTER_CLASS_CONFIG;
import static org.junit.Assert.assertEquals;
import static org.junit.Assert.assertNotNull;

import com.google.cloud.bigtable.data.v2.models.Row;
import com.google.cloud.bigtable.data.v2.models.RowCell;
import com.google.cloud.kafka.connect.bigtable.config.InsertMode;
import com.google.cloud.kafka.connect.bigtable.transformations.ExtractTimestamp;
import com.google.cloud.kafka.connect.bigtable.transformations.TimestampPrecision;
import com.google.cloud.kafka.connect.bigtable.util.TestDataUtil;
import com.google.protobuf.ByteString;
import java.nio.charset.StandardCharsets;
import java.util.Collections;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.ExecutionException;
import org.apache.kafka.connect.data.Schema;
import org.apache.kafka.connect.data.SchemaBuilder;
import org.apache.kafka.connect.data.Struct;
import org.apache.kafka.connect.json.JsonConverter;
import org.apache.kafka.connect.storage.StringConverter;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;

@RunWith(JUnit4.class)
public class ExtractTimestampIT extends BaseKafkaConnectBigtableIT {

  @Test
  public void testExtractTimestampFromValue() throws InterruptedException, ExecutionException {
    Map<String, String> props = baseConnectorProps();
    props.put(INSERT_MODE_CONFIG, InsertMode.UPSERT.name());
    props.put("transforms", "extractTimestamp");
    props.put("transforms.extractTimestamp.type", ExtractTimestamp.Value.class.getName());
    props.put("transforms.extractTimestamp." + ExtractTimestamp.TIMESTAMP_FIELD_CONFIG, "ts");
    props.put(
        "transforms.extractTimestamp." + ExtractTimestamp.TIMESTAMP_FIELD_PRECISION_CONFIG,
        TimestampPrecision.MILLIS.name());
    props.put(DEFAULT_COLUMN_FAMILY_CONFIG, "cf");
    props.put(VALUE_CONVERTER_CLASS_CONFIG, JsonConverter.class.getName());

    String testId = startSingleTopicConnector(props);
    createTablesAndColumnFamilies(Map.of(testId, Set.of("cf")));

    Schema schema =
        SchemaBuilder.struct()
            .field("id", Schema.STRING_SCHEMA)
            .field("ts", Schema.INT64_SCHEMA)
            .build();

    long timestamp = 1234567890L;
    Struct value = new Struct(schema).put("id", "val1").put("ts", timestamp);

    JsonConverter converter = new JsonConverter();
    converter.configure(Collections.singletonMap("schemas.enable", "true"), false);
    byte[] valueJson = converter.fromConnectData(testId, schema, value);

    String key = "key1";
    connect.kafka().produce(testId, key, new String(valueJson));

    waitUntilBigtableContainsNumberOfRows(testId, 1);
    Map<ByteString, Row> rows = readAllRows(bigtableData, testId);
    Row row = rows.get(ByteString.copyFrom(key.getBytes(StandardCharsets.UTF_8)));
    assertNotNull(row);

    List<RowCell> cells = row.getCells("cf", "id");
    assertEquals(1, cells.size());
    // Bigtable timestamps are in microseconds
    assertEquals(timestamp * 1000, cells.get(0).getTimestamp());
  }

  @Test
  public void testExtractTimestampFromNestedValue()
      throws InterruptedException, ExecutionException {
    Map<String, String> props = baseConnectorProps();
    props.put(INSERT_MODE_CONFIG, InsertMode.UPSERT.name());
    props.put("transforms", "extractTimestamp");
    props.put("transforms.extractTimestamp.type", ExtractTimestamp.Value.class.getName());
    props.put(
        "transforms.extractTimestamp." + ExtractTimestamp.TIMESTAMP_FIELD_CONFIG, "nested.ts");
    props.put(
        "transforms.extractTimestamp." + ExtractTimestamp.TIMESTAMP_FIELD_PRECISION_CONFIG,
        TimestampPrecision.SECONDS.name());
    props.put(DEFAULT_COLUMN_FAMILY_CONFIG, "cf");
    props.put(VALUE_CONVERTER_CLASS_CONFIG, JsonConverter.class.getName());

    String testId = startSingleTopicConnector(props);
    createTablesAndColumnFamilies(Map.of(testId, Set.of("cf", "nested")));

    Schema nestedSchema = SchemaBuilder.struct().field("ts", Schema.INT64_SCHEMA).build();
    Schema schema =
        SchemaBuilder.struct()
            .field("id", Schema.STRING_SCHEMA)
            .field("nested", nestedSchema)
            .build();

    long timestampSeconds = 1600000000L;
    Struct nestedValue = new Struct(nestedSchema).put("ts", timestampSeconds);
    Struct value = new Struct(schema).put("id", "val2").put("nested", nestedValue);

    JsonConverter converter = new JsonConverter();
    converter.configure(Collections.singletonMap("schemas.enable", "true"), false);
    byte[] valueJson = converter.fromConnectData(testId, schema, value);

    String key = "key2";
    connect.kafka().produce(testId, key, new String(valueJson));

    waitUntilBigtableContainsNumberOfRows(testId, 1);
    Map<ByteString, Row> rows = readAllRows(bigtableData, testId);
    Row row = rows.get(ByteString.copyFrom(key.getBytes(StandardCharsets.UTF_8)));
    assertNotNull(row);

    List<RowCell> cells = row.getCells("cf", "id");
    assertEquals(1, cells.size());
    // Bigtable timestamps are in microseconds.
    // timestampSeconds * 1000 (millis) * 1000 (micros)
    assertEquals(timestampSeconds * 1000 * 1000, cells.get(0).getTimestamp());
  }

  @Test
  public void testExtractTimestampFromKey() throws InterruptedException, ExecutionException {
    Map<String, String> props = baseConnectorProps();
    props.put(INSERT_MODE_CONFIG, InsertMode.UPSERT.name());
    props.put("transforms", "extractTimestamp");
    props.put("transforms.extractTimestamp.type", ExtractTimestamp.Key.class.getName());
    props.put("transforms.extractTimestamp." + ExtractTimestamp.TIMESTAMP_FIELD_CONFIG, "ts");
    props.put(DEFAULT_COLUMN_FAMILY_CONFIG, "cf");
    props.put(VALUE_CONVERTER_CLASS_CONFIG, StringConverter.class.getName());
    props.put("key.converter", JsonConverter.class.getName());
    props.put("key.converter.schemas.enable", "true");

    String testId = startSingleTopicConnector(props);
    createTablesAndColumnFamilies(Map.of(testId, Set.of("cf")));

    Schema keySchema = SchemaBuilder.struct().field("ts", Schema.INT64_SCHEMA).build();
    long timestamp = 9876543210L;
    Struct keyStruct = new Struct(keySchema).put("ts", timestamp);

    JsonConverter converter = new JsonConverter();
    converter.configure(Collections.singletonMap("schemas.enable", "true"), true);
    byte[] keyJson = converter.fromConnectData(testId, keySchema, keyStruct);

    String value = "some-value";
    connect.kafka().produce(testId, new String(keyJson), value);

    waitUntilBigtableContainsNumberOfRows(testId, 1);
    Map<ByteString, Row> rows = readAllRows(bigtableData, testId);
    // The row key will be the JSON string of the key
    Row row = rows.values().iterator().next();
    assertNotNull(row);

    List<RowCell> cells = row.getCells("cf", "KAFKA_VALUE");
    assertEquals(1, cells.size());
    assertEquals(timestamp * 1000, cells.get(0).getTimestamp());
  }

  @Test
  public void testExtractTimestampFromValueNoSchema() throws Exception {
    Map<String, String> props = baseConnectorProps();
    props.put(INSERT_MODE_CONFIG, InsertMode.UPSERT.name());
    props.put("transforms", "extractTimestamp");
    props.put("transforms.extractTimestamp.type", ExtractTimestamp.Value.class.getName());
    props.put(
        "transforms.extractTimestamp." + ExtractTimestamp.TIMESTAMP_FIELD_CONFIG, "createdAt");
    props.put(
        "transforms.extractTimestamp." + ExtractTimestamp.TIMESTAMP_FIELD_PRECISION_CONFIG,
        TimestampPrecision.SECONDS.name());
    props.put(DEFAULT_COLUMN_FAMILY_CONFIG, "cf");
    props.put(VALUE_CONVERTER_CLASS_CONFIG, JsonConverter.class.getName());
    props.put("value.converter.schemas.enable", "false");

    String testId = startSingleTopicConnector(props);
    createTablesAndColumnFamilies(Map.of(testId, Set.of("cf")));

    String json = TestDataUtil.readResource("json/order-no-schema.json");
    String key = "order1";
    connect.kafka().produce(testId, key, json);

    waitUntilBigtableContainsNumberOfRows(testId, 1);
    Map<ByteString, Row> rows = readAllRows(bigtableData, testId);
    Row row = rows.get(ByteString.copyFrom(key.getBytes(StandardCharsets.UTF_8)));
    assertNotNull(row);

    // createdAt: 1779122855 (seconds)
    long expectedTimestampMicros = 1779122855L * 1000 * 1000;

    List<RowCell> cells = row.getCells("cf", "KAFKA_VALUE");
    assertEquals(1, cells.size());
    assertEquals(expectedTimestampMicros, cells.get(0).getTimestamp());
  }
}
