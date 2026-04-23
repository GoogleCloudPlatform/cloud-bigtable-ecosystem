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

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertInstanceOf;

import com.google.cloud.aerospike.values.BooleanValue;
import com.google.cloud.aerospike.values.BytesValue;
import com.google.cloud.aerospike.values.FloatValue;
import com.google.cloud.aerospike.values.IntegerValue;
import com.google.cloud.aerospike.values.ListValue;
import com.google.cloud.aerospike.values.MapValue;
import com.google.cloud.aerospike.values.StringValue;
import com.google.cloud.aerospike.values.Value;
import com.google.common.collect.ImmutableMap;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.util.Base64;
import java.util.List;
import java.util.Map;
import java.util.Set;
import org.junit.jupiter.api.Test;

public class SerializationTest {
  @Test
  public void testBoolean() {
    for (Map.Entry<Boolean, String> test : Map.of(true, "true", false, "false").entrySet()) {
      assertEquals(
          test.getValue(),
          Value.getJsonAdapter(BooleanValue.class).toJson(new BooleanValue(test.getKey())));
    }
  }

  @Test
  public void testLong() {
    for (Map.Entry<Long, String> test :
        Map.of(Long.MIN_VALUE, "-9223372036854775808", 0L, "0", 123456789L, "123456789")
            .entrySet()) {
      assertEquals(
          test.getValue(),
          Value.getJsonAdapter(IntegerValue.class).toJson(new IntegerValue(test.getKey())));
    }
  }

  @Test
  public void testDouble() {
    for (Map.Entry<Double, String> test :
        Map.of(
                Double.NaN,
                "\"NaN\"",
                Double.NEGATIVE_INFINITY,
                "\"-Infinity\"",
                Double.POSITIVE_INFINITY,
                "\"Infinity\"",
                0.0,
                "\"0.0\"",
                Double.MAX_VALUE,
                "\"1.7976931348623157E308\"",
                Double.MIN_VALUE,
                "\"4.9E-324\"",
                12345.6789,
                "\"12345.6789\"")
            .entrySet()) {
      assertEquals(
          test.getValue(),
          Value.getJsonAdapter(FloatValue.class).toJson(new FloatValue(test.getKey())));
    }
  }

  @Test
  public void testString() {
    for (Map.Entry<String, String> test :
        Map.of("test", "\"test\"", "ꙮ", "\"ꙮ\"", "العربية", "\"العربية\"", "\"", "\"\\\"\"")
            .entrySet()) {
      assertEquals(
          test.getValue(),
          Value.getJsonAdapter(StringValue.class).toJson(new StringValue(test.getKey())));
    }
  }

  @Test
  public void testBytes() {
    byte[] test = {1, 2, 3, 4, 5, 0, -128, 127, 0};
    String expected = "\"" + Base64.getEncoder().encodeToString(test) + "\"";
    assertEquals(expected, Value.getJsonAdapter(BytesValue.class).toJson(new BytesValue(test)));
  }

  @Test
  public void testList() {
    ListValue test = new ListValue(List.of(1L, 2.2, "o"));
    assertEquals("[1,\"2.2\",\"o\"]", Value.getJsonAdapter(ListValue.class).toJson(test));
  }

  @Test
  public void testListOfLists() {
    ListValue test = new ListValue(List.of(List.of(1L), List.of(2.2, "o")));
    assertEquals("[[1],[\"2.2\",\"o\"]]", Value.getJsonAdapter(ListValue.class).toJson(test));
  }

  @Test
  public void testMap() {
    byte[] bytesValue = "bytes".getBytes(StandardCharsets.UTF_8);
    // ImmutableMap preservers "insertion" order, we rely on it.
    MapValue test =
        new MapValue(ImmutableMap.of("key1", 1L, "key2", 2.2, "key3", "o", "key4", bytesValue));
    assertEquals(
        "{\"key1\":1,\"key2\":\"2.2\",\"key3\":\"o\",\"key4\":\""
            + Base64.getEncoder().encodeToString(bytesValue)
            + "\"}",
        Value.getJsonAdapter(MapValue.class).toJson(test));
  }

  @Test
  public void testComplex() {
    // ImmutableMap preservers "insertion" order, we rely on it.
    MapValue test =
        new MapValue(
            ImmutableMap.of(
                "k1", List.of("o", true),
                "k2", List.of(ImmutableMap.of("nKey1", 1L, "nKey2", 2.2, "nKey3", "str"))));

    assertEquals(
        "{\"k1\":[\"o\",true],\"k2\":[{\"nKey1\":1,\"nKey2\":\"2.2\",\"nKey3\":\"str\"}]}",
        Value.getJsonAdapter(MapValue.class).toJson(test));
  }

  @Test
  public void testReadRecordResult() throws IOException {
    byte digest[] = new byte[] {-128, 0, 1, 2, 3, 127};
    ReadRecordResult test = new ReadRecordResult(digest);

    test.bins.putAll(
        Map.of(
            "map", new MapValue(Map.of("k1", false)),
            "list", new ListValue(List.of(true)),
            "bool", new BooleanValue(true),
            "bytes", new BytesValue(new byte[] {1, 2, 3}),
            "double", new FloatValue(2.2),
            "long", new IntegerValue(1L),
            "string", new StringValue("o")));

    Map<?, ?> map = Value.getJsonAdapter(Map.class).fromJson(test.toJson());
    // We cannot immediately `assertEquals(expected, map)`, because bins are deserialized to String.
    assertEquals(Set.of("digest", "bins"), map.keySet());
    assertEquals(Base64.getEncoder().encodeToString(digest), map.get("digest"));
    assertInstanceOf(String.class, map.get("bins"));
    Map<?, ?> bins = Value.getJsonAdapter(Map.class).fromJson((String) map.get("bins"));
    assertEquals(
        Map.of(
            "map",
            Map.of("k1", false),
            "list",
            List.of(true),
            "bool",
            true,
            "bytes",
            Base64.getEncoder().encodeToString(new byte[] {1, 2, 3}),
            "double",
            "2.2",
            "long",
            1.0,
            "string",
            "o"),
        bins);
  }
}
