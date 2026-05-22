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

import static org.junit.Assert.*;

import java.time.Instant;
import java.util.*;
import org.apache.kafka.connect.data.Schema;
import org.apache.kafka.connect.data.SchemaAndValue;
import org.apache.kafka.connect.data.SchemaBuilder;
import org.apache.kafka.connect.data.Struct;
import org.apache.kafka.connect.errors.DataException;
import org.apache.kafka.connect.source.SourceRecord;
import org.junit.After;
import org.junit.Before;
import org.junit.Test;

public class ExtractTimestampTest {

  private ExtractTimestamp.Value<SourceRecord> valueSmt;
  private ExtractTimestamp.Key<SourceRecord> keySmt;

  @Before
  public void setUp() {
    valueSmt = new ExtractTimestamp.Value<>();
    keySmt = new ExtractTimestamp.Key<>();
  }

  @After
  public void tearDown() {
    valueSmt.close();
    keySmt.close();
  }

  @Test
  public void testParseTimestampToMillis() {
    long baseMillis = 1715698738123L;

    // Test different precisions with numeric input
    assertEquals(
        baseMillis,
        ExtractTimestamp.parseTimestampToMillis(
            new SchemaAndValue(Schema.INT64_SCHEMA, baseMillis * 1000000L),
            TimestampPrecision.NANOS));
    assertEquals(
        baseMillis,
        ExtractTimestamp.parseTimestampToMillis(
            new SchemaAndValue(Schema.INT64_SCHEMA, baseMillis * 1000L),
            TimestampPrecision.MICROS));
    assertEquals(
        baseMillis,
        ExtractTimestamp.parseTimestampToMillis(
            new SchemaAndValue(Schema.INT64_SCHEMA, baseMillis), TimestampPrecision.MILLIS));
    assertEquals(
        baseMillis - (baseMillis % 1000), // SECONDS loses precision
        ExtractTimestamp.parseTimestampToMillis(
            new SchemaAndValue(Schema.INT64_SCHEMA, baseMillis / 1000),
            TimestampPrecision.SECONDS));

    // Test different input types
    assertEquals(
        baseMillis,
        ExtractTimestamp.parseTimestampToMillis(
            new SchemaAndValue(Schema.STRING_SCHEMA, String.valueOf(baseMillis)),
            TimestampPrecision.MILLIS));
    assertEquals(
        baseMillis,
        ExtractTimestamp.parseTimestampToMillis(
            new SchemaAndValue(null, new Date(baseMillis)), TimestampPrecision.MILLIS));
    assertEquals(
        baseMillis,
        ExtractTimestamp.parseTimestampToMillis(
            new SchemaAndValue(null, Instant.ofEpochMilli(baseMillis)), TimestampPrecision.MILLIS));
  }

  @Test
  public void testAllTimestampPrecisionsImplemented() {
    SchemaAndValue value = new SchemaAndValue(Schema.INT64_SCHEMA, 123456789L);
    for (TimestampPrecision precision : TimestampPrecision.values()) {
      try {
        // We don't care about the output value, just that it doesn't throw IllegalStateException
        // (default case)
        ExtractTimestamp.parseTimestampToMillis(value, precision);
      } catch (IllegalStateException e) {
        fail("TimestampPrecision " + precision + " is not implemented in parseTimestampToMillis");
      }
    }
  }

  @Test(expected = IllegalArgumentException.class)
  public void testParseTimestampToMillisNullValue() {
    ExtractTimestamp.parseTimestampToMillis(null, TimestampPrecision.MILLIS);
  }

  @Test(expected = IllegalArgumentException.class)
  public void testParseTimestampToMillisNullInnerValue() {
    ExtractTimestamp.parseTimestampToMillis(
        new SchemaAndValue(Schema.OPTIONAL_INT64_SCHEMA, null), TimestampPrecision.MILLIS);
  }

  @Test(expected = IllegalArgumentException.class)
  public void testParseTimestampToMillisUnsupportedType() {
    ExtractTimestamp.parseTimestampToMillis(
        new SchemaAndValue(null, new Object()), TimestampPrecision.MILLIS);
  }

  @Test
  public void testExtractTimestampFromStructValue() {
    Map<String, String> configs = new HashMap<>();
    configs.put(ExtractTimestamp.TIMESTAMP_FIELD_CONFIG, "ts");
    configs.put(
        ExtractTimestamp.TIMESTAMP_FIELD_PRECISION_CONFIG, TimestampPrecision.MILLIS.name());
    valueSmt.configure(configs);

    Schema valueSchema = SchemaBuilder.struct().field("ts", Schema.INT64_SCHEMA).build();
    Struct value = new Struct(valueSchema).put("ts", 123456789L);

    SourceRecord record = new SourceRecord(null, null, "test", 0, null, null, valueSchema, value);
    SourceRecord transformed = valueSmt.apply(record);

    assertEquals(Long.valueOf(123456789L), transformed.timestamp());
  }

  @Test
  public void testExtractTimestampFromStructKey() {
    Map<String, String> configs = new HashMap<>();
    configs.put(ExtractTimestamp.TIMESTAMP_FIELD_CONFIG, "ts");
    configs.put(
        ExtractTimestamp.TIMESTAMP_FIELD_PRECISION_CONFIG, TimestampPrecision.MILLIS.name());
    keySmt.configure(configs);

    Schema keySchema = SchemaBuilder.struct().field("ts", Schema.INT64_SCHEMA).build();
    Struct key = new Struct(keySchema).put("ts", 123456789L);

    SourceRecord record = new SourceRecord(null, null, "test", 0, keySchema, key, null, null);
    SourceRecord transformed = keySmt.apply(record);

    assertEquals(Long.valueOf(123456789L), transformed.timestamp());
  }

  @Test
  public void testNestedFieldExtraction() {
    Map<String, String> configs = new HashMap<>();
    configs.put(ExtractTimestamp.TIMESTAMP_FIELD_CONFIG, "outer.inner");
    configs.put(
        ExtractTimestamp.TIMESTAMP_FIELD_PRECISION_CONFIG, TimestampPrecision.MILLIS.name());
    valueSmt.configure(configs);

    Schema innerSchema = SchemaBuilder.struct().field("inner", Schema.INT64_SCHEMA).build();
    Schema outerSchema = SchemaBuilder.struct().field("outer", innerSchema).build();

    Struct inner = new Struct(innerSchema).put("inner", 987654321L);
    Struct outer = new Struct(outerSchema).put("outer", inner);

    SourceRecord record = new SourceRecord(null, null, "test", 0, null, null, outerSchema, outer);
    SourceRecord transformed = valueSmt.apply(record);

    assertEquals(Long.valueOf(987654321L), transformed.timestamp());
  }

  @Test
  public void testNestedMapExtraction() {
    Map<String, String> configs = new HashMap<>();
    configs.put(ExtractTimestamp.TIMESTAMP_FIELD_CONFIG, "outer.inner");
    configs.put(
        ExtractTimestamp.TIMESTAMP_FIELD_PRECISION_CONFIG, TimestampPrecision.MILLIS.name());
    valueSmt.configure(configs);

    Map<String, Object> inner = Collections.singletonMap("inner", 987654321L);
    Map<String, Object> outer = Collections.singletonMap("outer", inner);

    SourceRecord record = new SourceRecord(null, null, "test", 0, null, null, null, outer);
    SourceRecord transformed = valueSmt.apply(record);

    assertEquals(Long.valueOf(987654321L), transformed.timestamp());
  }

  @Test
  public void testFormats() {
    long baseMillis = 1715698738000L;

    // SECONDS
    verifyFormat(baseMillis / 1000, TimestampPrecision.SECONDS, baseMillis);

    // MILLIS
    verifyFormat(baseMillis, TimestampPrecision.MILLIS, baseMillis);

    // MICROS
    verifyFormat(baseMillis * 1000, TimestampPrecision.MICROS, baseMillis);

    // NANOS
    verifyFormat(baseMillis * 1000000, TimestampPrecision.NANOS, baseMillis);
  }

  private void verifyFormat(long inputValue, TimestampPrecision format, long expectedMillis) {
    Map<String, String> configs = new HashMap<>();
    configs.put(ExtractTimestamp.TIMESTAMP_FIELD_CONFIG, "ts");
    configs.put(ExtractTimestamp.TIMESTAMP_FIELD_PRECISION_CONFIG, format.name());
    ExtractTimestamp.Value<SourceRecord> smt = new ExtractTimestamp.Value<>();
    smt.configure(configs);

    Schema schema = SchemaBuilder.struct().field("ts", Schema.INT64_SCHEMA).build();
    Struct value = new Struct(schema).put("ts", inputValue);
    SourceRecord record = new SourceRecord(null, null, "test", 0, null, null, schema, value);

    SourceRecord transformed = smt.apply(record);
    assertEquals(
        "Failed for format " + format, Long.valueOf(expectedMillis), transformed.timestamp());
  }

  @Test
  public void testInputTypes() {
    long expectedMillis = 1715698738123L;

    // Long
    verifyInputType(expectedMillis, expectedMillis);

    // Int
    verifyInputType(123, 123);

    // String (long)
    verifyInputType(String.valueOf(expectedMillis), expectedMillis);

    // Date
    verifyInputType(new Date(expectedMillis), expectedMillis);

    // Instant
    verifyInputType(Instant.ofEpochMilli(expectedMillis), expectedMillis);
  }

  private void verifyInputType(Object inputValue, long expectedMillis) {
    Map<String, String> configs = new HashMap<>();
    configs.put(ExtractTimestamp.TIMESTAMP_FIELD_CONFIG, "ts");
    configs.put(
        ExtractTimestamp.TIMESTAMP_FIELD_PRECISION_CONFIG, TimestampPrecision.MILLIS.name());
    valueSmt.configure(configs);

    Schema schema = null; // Schemaless test
    Map<String, Object> value = Collections.singletonMap("ts", inputValue);
    SourceRecord record = new SourceRecord(null, null, "test", 0, null, null, schema, value);

    SourceRecord transformed = valueSmt.apply(record);
    assertEquals(
        "Failed for input type " + inputValue.getClass().getName(),
        Long.valueOf(expectedMillis),
        transformed.timestamp());
  }

  @Test(expected = DataException.class)
  public void testMissingField() {
    Map<String, String> configs = new HashMap<>();
    configs.put(ExtractTimestamp.TIMESTAMP_FIELD_CONFIG, "missing");
    configs.put(
        ExtractTimestamp.TIMESTAMP_FIELD_PRECISION_CONFIG, TimestampPrecision.MILLIS.name());
    valueSmt.configure(configs);

    Schema schema = SchemaBuilder.struct().field("other", Schema.INT64_SCHEMA).build();
    Struct value = new Struct(schema).put("other", 123L);
    SourceRecord record = new SourceRecord(null, null, "test", 0, null, null, schema, value);

    valueSmt.apply(record);
  }

  @Test(expected = DataException.class)
  public void testNullField() {
    Map<String, String> configs = new HashMap<>();
    configs.put(ExtractTimestamp.TIMESTAMP_FIELD_CONFIG, "ts");
    configs.put(
        ExtractTimestamp.TIMESTAMP_FIELD_PRECISION_CONFIG, TimestampPrecision.MILLIS.name());
    valueSmt.configure(configs);

    Schema schema = SchemaBuilder.struct().field("ts", Schema.OPTIONAL_INT64_SCHEMA).build();
    Struct value = new Struct(schema).put("ts", null);
    SourceRecord record = new SourceRecord(null, null, "test", 0, null, null, schema, value);

    valueSmt.apply(record);
  }
}
