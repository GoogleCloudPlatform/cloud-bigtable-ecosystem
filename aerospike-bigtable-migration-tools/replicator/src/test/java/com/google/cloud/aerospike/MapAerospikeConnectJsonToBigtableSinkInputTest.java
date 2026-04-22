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

import static org.junit.jupiter.api.Assertions.assertArrayEquals;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertInstanceOf;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertThrows;

import com.google.cloud.aerospike.config.Config;
import com.google.cloud.aerospike.exception.InvalidMessageException;
import com.google.cloud.aerospike.util.AerospikeConnectXdrJson;
import com.google.protobuf.ByteString;
import java.nio.charset.StandardCharsets;
import java.util.Base64;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.atomic.AtomicInteger;
import org.apache.kafka.common.config.ConfigException;
import org.apache.kafka.common.record.TimestampType;
import org.apache.kafka.connect.data.Field;
import org.apache.kafka.connect.data.Schema;
import org.apache.kafka.connect.data.SchemaBuilder;
import org.apache.kafka.connect.data.Struct;
import org.apache.kafka.connect.sink.SinkRecord;
import org.junit.jupiter.api.Test;

public class MapAerospikeConnectJsonToBigtableSinkInputTest {
  public static final String TOPIC = "topic";
  public static final int PARTITION = 0;
  // It makes sure that no two messages returned by record() are identical.
  private static final AtomicInteger KAFKA_OFFSET = new AtomicInteger(0);
  private static final long DEFAULT_START_LUT_MILLIS = 1000L;
  private static final byte[] DEFAULT_DIGEST = new byte[20];

  private static final MapAerospikeConnectJsonToBigtableSinkInput<SinkRecord>
      AEROSPIKE_CONNECT_JSON_TO_BIGTABLE_SINK_INPUT_TRANSFORMER =
          transformation(DEFAULT_START_LUT_MILLIS);

  @Test
  public void testInvalidConfig() {
    assertThrows(
        ConfigException.class,
        () -> new MapAerospikeConnectJsonToBigtableSinkInput<>().configure(Map.of()));
    assertThrows(ConfigException.class, () -> transformation(-1L));
  }

  @Test
  public void invalidValueShape() {
    SinkRecord struct =
        record(new Struct(SchemaBuilder.struct().build()), DEFAULT_START_LUT_MILLIS);
    assertThrows(
        InvalidMessageException.class,
        () -> AEROSPIKE_CONNECT_JSON_TO_BIGTABLE_SINK_INPUT_TRANSFORMER.apply(struct));
  }

  @Test
  public void testIgnoredLutFilters() {
    SinkRecord early =
        record(
            AerospikeConnectXdrJson.deleteMessageMap(DEFAULT_START_LUT_MILLIS - 1, DEFAULT_DIGEST),
            null);
    SinkRecord minimal =
        record(
            AerospikeConnectXdrJson.deleteMessageMap(DEFAULT_START_LUT_MILLIS, DEFAULT_DIGEST),
            null);

    assertNull(AEROSPIKE_CONNECT_JSON_TO_BIGTABLE_SINK_INPUT_TRANSFORMER.apply(early));
    assertNotNull(AEROSPIKE_CONNECT_JSON_TO_BIGTABLE_SINK_INPUT_TRANSFORMER.apply(minimal));
  }

  @Test
  public void testDelete() {
    SinkRecord delete =
        record(
            AerospikeConnectXdrJson.deleteMessageMap(DEFAULT_START_LUT_MILLIS, DEFAULT_DIGEST),
            123456L);
    SinkRecord result = AEROSPIKE_CONNECT_JSON_TO_BIGTABLE_SINK_INPUT_TRANSFORMER.apply(delete);
    assertEquals(DEFAULT_START_LUT_MILLIS, result.timestamp());
    assertArrayEquals(DEFAULT_DIGEST, assertInstanceOf(byte[].class, result.key()));
    assertNull(result.value());
  }

  @Test
  public void testWrite() {
    Map<?, ?> value =
        AerospikeConnectXdrJson.writeMessageMap(
            DEFAULT_START_LUT_MILLIS,
            DEFAULT_DIGEST,
            List.of(
                new AerospikeConnectXdrJson.Bin(
                    "myString",
                    MapAerospikeConnectJsonToBigtableSinkInput.ValueType.STR.name(),
                    "a string value"),
                new AerospikeConnectXdrJson.Bin(
                    "myBlob",
                    MapAerospikeConnectJsonToBigtableSinkInput.ValueType.BLOB.name(),
                    Base64.getEncoder()
                        .encodeToString(
                            "ABCDEFGHIJKLMNOPQRSTUVWXYZ".getBytes(StandardCharsets.UTF_8))),
                new AerospikeConnectXdrJson.Bin(
                    "myBoolean",
                    MapAerospikeConnectJsonToBigtableSinkInput.ValueType.BOOL.name(),
                    true),
                new AerospikeConnectXdrJson.Bin(
                    "myDouble",
                    MapAerospikeConnectJsonToBigtableSinkInput.ValueType.FLOAT.name(),
                    1.23),
                new AerospikeConnectXdrJson.Bin(
                    "myDoubleInfinity",
                    MapAerospikeConnectJsonToBigtableSinkInput.ValueType.FLOAT.name(),
                    Double.POSITIVE_INFINITY),
                new AerospikeConnectXdrJson.Bin(
                    "myInt",
                    MapAerospikeConnectJsonToBigtableSinkInput.ValueType.INT.name(),
                    Long.MAX_VALUE),
                new AerospikeConnectXdrJson.Bin(
                    "myList",
                    MapAerospikeConnectJsonToBigtableSinkInput.ValueType.LIST.name(),
                    List.of("abc", "def", "ghi", "jkl")),
                new AerospikeConnectXdrJson.Bin(
                    "myMap",
                    MapAerospikeConnectJsonToBigtableSinkInput.ValueType.MAP.name(),
                    Map.of("f", 3.1415, "i", 42, "l", List.of(3, 2, 1, 0)))));
    SinkRecord input = record(value, 123456L);
    SinkRecord output = AEROSPIKE_CONNECT_JSON_TO_BIGTABLE_SINK_INPUT_TRANSFORMER.apply(input);
    assertEquals(DEFAULT_START_LUT_MILLIS, output.timestamp());
    assertArrayEquals(DEFAULT_DIGEST, assertInstanceOf(byte[].class, output.key()));
    Struct outputValue = assertInstanceOf(Struct.class, output.value());
    assertEquals(outputValue.schema(), output.valueSchema());

    Map<String, Map<String, ByteString>> parsedResult = parseResult(outputValue);
    assertEquals(
        Map.of(
            RowBuilder.SCALAR_FAMILY,
                Map.of(
                    "myString__STRING", RowBuilder.toBytes("a string value"),
                    "myBlob__BYTES",
                        RowBuilder.toBytes(
                            "ABCDEFGHIJKLMNOPQRSTUVWXYZ".getBytes(StandardCharsets.UTF_8)),
                    "myBoolean__BOOL", RowBuilder.toBytes(true),
                    "myDouble__FLOAT64", RowBuilder.toBytes(1.23),
                    "myDoubleInfinity__FLOAT64", RowBuilder.toBytes(Double.POSITIVE_INFINITY),
                    "myInt__INT64", RowBuilder.toBytes(Long.MAX_VALUE)),
            RowBuilder.COMPLEX_OBJECT_FAMILY,
                Map.of(
                    "myList__LIST__0__STRING", RowBuilder.toBytes("abc"),
                    "myList__LIST__1__STRING", RowBuilder.toBytes("def"),
                    "myList__LIST__2__STRING", RowBuilder.toBytes("ghi"),
                    "myList__LIST__3__STRING", RowBuilder.toBytes("jkl"),
                    "myMap__MAP__f__FLOAT64", RowBuilder.toBytes(3.1415),
                    "myMap__MAP__i__INT64", RowBuilder.toBytes(42),
                    "myMap__MAP__l__LIST__0__INT64", RowBuilder.toBytes(3),
                    "myMap__MAP__l__LIST__1__INT64", RowBuilder.toBytes(2),
                    "myMap__MAP__l__LIST__2__INT64", RowBuilder.toBytes(1),
                    "myMap__MAP__l__LIST__3__INT64", RowBuilder.toBytes(0))),
        parsedResult);
  }

  @Test
  public void testUnsupportedBinType() {
    Map<?, ?> value =
        AerospikeConnectXdrJson.writeMessageMap(
            DEFAULT_START_LUT_MILLIS,
            DEFAULT_DIGEST,
            List.of(
                // GeoJSON is the only known data type, which we explicitly don't support.
                new AerospikeConnectXdrJson.Bin(
                    "geo",
                    MapAerospikeConnectJsonToBigtableSinkInput.ValueType.GEOJSON.name(),
                    Map.of("type", "Point", "coordinates", List.of(1.30824, 103.91327)))));
    SinkRecord record = record(value, null);
    assertThrows(
        InvalidMessageException.class,
        () -> AEROSPIKE_CONNECT_JSON_TO_BIGTABLE_SINK_INPUT_TRANSFORMER.apply(record));
  }

  public static Map<String, Map<String, ByteString>> parseResult(Struct root) {
    Map<String, Map<String, ByteString>> parsed = new HashMap<>();
    Schema rootSchema = root.schema();
    for (Field rootField : rootSchema.fields()) {
      String rootFieldName = rootField.name();
      Struct child = root.getStruct(rootFieldName);
      Schema childSchema = child.schema();

      HashMap<String, ByteString> childParsed = new HashMap<>();
      for (Field childField : childSchema.fields()) {
        String childFieldName = childField.name();
        ByteString value = ByteString.copyFrom(child.getBytes(childFieldName));

        assertFalse(childParsed.containsKey(childFieldName));
        childParsed.put(childFieldName, value);
      }
      assertFalse(parsed.containsKey(rootFieldName));
      parsed.put(rootFieldName, childParsed);
    }
    return parsed;
  }

  public static SinkRecord record(Object value, Long timestamp) {
    return new SinkRecord(
        TOPIC,
        PARTITION,
        null,
        null,
        null,
        value,
        KAFKA_OFFSET.incrementAndGet(),
        timestamp,
        TimestampType.NO_TIMESTAMP_TYPE);
  }

  public static MapAerospikeConnectJsonToBigtableSinkInput<SinkRecord> transformation(
      long startLutMillis) {
    MapAerospikeConnectJsonToBigtableSinkInput<SinkRecord> r =
        new MapAerospikeConnectJsonToBigtableSinkInput<>();
    r.configure(Map.of(Config.START_LUT_MILLIS, String.valueOf(startLutMillis)));
    return r;
  }
}
