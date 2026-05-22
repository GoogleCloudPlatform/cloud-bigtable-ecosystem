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
package com.google.cloud.kafka.connect.bigtable.util;

import static org.junit.Assert.assertEquals;
import static org.junit.Assert.assertNull;

import java.util.Collections;
import java.util.HashMap;
import java.util.Map;
import org.apache.kafka.connect.data.Schema;
import org.apache.kafka.connect.data.SchemaAndValue;
import org.apache.kafka.connect.data.SchemaBuilder;
import org.apache.kafka.connect.data.Struct;
import org.apache.kafka.connect.errors.DataException;
import org.junit.Test;

public class SchemaParsingUtilsTest {

  @Test
  public void testExtractFieldFromStruct() {
    Schema schema = SchemaBuilder.struct().field("f1", Schema.STRING_SCHEMA).build();
    Struct struct = new Struct(schema).put("f1", "v1");
    SchemaAndValue input = new SchemaAndValue(schema, struct);

    SchemaAndValue result = SchemaParsingUtils.extractField(input, new String[] {"f1"});
    assertEquals(Schema.STRING_SCHEMA, result.schema());
    assertEquals("v1", result.value());
  }

  @Test
  public void testExtractFieldFromMap() {
    Map<String, Object> map = Collections.singletonMap("f1", "v1");
    SchemaAndValue input = new SchemaAndValue(null, map);

    SchemaAndValue result = SchemaParsingUtils.extractField(input, new String[] {"f1"});
    assertNull(result.schema());
    assertEquals("v1", result.value());
  }

  @Test
  public void testExtractFieldNestedStruct() {
    Schema innerSchema = SchemaBuilder.struct().field("inner", Schema.INT32_SCHEMA).build();
    Schema outerSchema = SchemaBuilder.struct().field("outer", innerSchema).build();

    Struct inner = new Struct(innerSchema).put("inner", 42);
    Struct outer = new Struct(outerSchema).put("outer", inner);
    SchemaAndValue input = new SchemaAndValue(outerSchema, outer);

    SchemaAndValue result = SchemaParsingUtils.extractField(input, new String[] {"outer", "inner"});
    assertEquals(Schema.INT32_SCHEMA, result.schema());
    assertEquals(42, result.value());
  }

  @Test
  public void testExtractFieldNestedMap() {
    Map<String, Object> inner = Collections.singletonMap("inner", 42);
    Map<String, Object> outer = Collections.singletonMap("outer", inner);
    SchemaAndValue input = new SchemaAndValue(null, outer);

    SchemaAndValue result = SchemaParsingUtils.extractField(input, new String[] {"outer", "inner"});
    assertEquals(42, result.value());
  }

  @Test
  public void testExtractFieldMixed() {
    Schema innerSchema = SchemaBuilder.struct().field("inner", Schema.INT32_SCHEMA).build();
    Struct inner = new Struct(innerSchema).put("inner", 42);

    Map<String, Object> outer = Collections.singletonMap("outer", inner);
    SchemaAndValue input = new SchemaAndValue(null, outer);

    SchemaAndValue result = SchemaParsingUtils.extractField(input, new String[] {"outer", "inner"});
    assertNull(result.schema());
    assertEquals(42, result.value());
  }

  @Test(expected = DataException.class)
  public void testExtractNestedFieldNotFound() {
    Schema innerSchema = SchemaBuilder.struct().field("inner", Schema.INT32_SCHEMA).build();
    Struct inner = new Struct(innerSchema).put("inner", 42);

    Map<String, Object> outer = Collections.singletonMap("outer", inner);
    SchemaAndValue input = new SchemaAndValue(null, outer);

    SchemaParsingUtils.extractField(input, new String[] {"outer", "not_found"});
  }

  @Test
  public void testExtractFieldEmptyPath() {
    SchemaAndValue input = new SchemaAndValue(Schema.STRING_SCHEMA, "v1");
    SchemaAndValue result = SchemaParsingUtils.extractField(input, new String[] {});
    assertEquals(input, result);
  }

  @Test(expected = DataException.class)
  public void testExtractFieldNullValue() {
    SchemaAndValue input = new SchemaAndValue(Schema.STRING_SCHEMA, null);
    SchemaParsingUtils.extractField(input, new String[] {"f1"});
  }

  @Test(expected = DataException.class)
  public void testExtractFieldMissingInStruct() {
    Schema schema = SchemaBuilder.struct().field("f1", Schema.STRING_SCHEMA).build();
    Struct struct = new Struct(schema).put("f1", "v1");
    SchemaAndValue input = new SchemaAndValue(schema, struct);

    // Struct.getWithoutDefault throws DataException (wrapped from SchemaException usually, but
    // Kafka Connect Struct throws DataException)
    SchemaParsingUtils.extractField(input, new String[] {"f2"});
  }

  @Test(expected = DataException.class)
  public void testExtractFieldMissingInMap() {
    Map<String, Object> map = new HashMap<>();
    SchemaAndValue input = new SchemaAndValue(null, map);

    // Map.get("f1") returns null, then next recursion calls ensureKeyElementIsNotNull(null)
    SchemaParsingUtils.extractField(input, new String[] {"f1"});
  }

  @Test(expected = DataException.class)
  public void testExtractFieldUnsupportedType() {
    SchemaAndValue input = new SchemaAndValue(Schema.STRING_SCHEMA, "not-a-struct-or-map");
    SchemaParsingUtils.extractField(input, new String[] {"f1"});
  }

  @Test
  public void testEnsureKeyElementIsNotNull() {
    SchemaParsingUtils.ensureKeyElementIsNotNull("not null");
  }

  @Test(expected = DataException.class)
  public void testEnsureKeyElementIsNotNullWithNull() {
    SchemaParsingUtils.ensureKeyElementIsNotNull(null);
  }
}
