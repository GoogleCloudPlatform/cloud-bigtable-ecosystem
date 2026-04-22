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

import static com.google.cloud.aerospike.MockUtil.assertTotalNumberOfInvocations;
import static org.junit.jupiter.api.Assertions.assertArrayEquals;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.times;
import static org.mockito.Mockito.verify;

import com.google.cloud.bigtable.data.v2.models.Mutation;
import com.google.protobuf.ByteString;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import org.apache.hadoop.hbase.util.Bytes;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

public class RowBuilderTest {
  private static final Long TIMESTAMP = 7312L;

  private Mutation bigtable;
  private RowBuilder<?> aerospike;

  @BeforeEach
  public void setUp() {
    bigtable = mock(Mutation.class);
    aerospike =
        new BigtableMutationBuilder(
            "table", ByteString.copyFromUtf8("rowkey"), bigtable, TIMESTAMP);
  }

  @Test
  public void testScalars() {
    // Test long serialization and deserialization
    for (long longValue : new long[] {-123456789L, 0L, 123456789L}) {
      ByteString longBytes = RowBuilder.toBytes(longValue);
      long deserializedLong = RowBuilder.integerFromBytes(longBytes);
      assertEquals(
          longValue, deserializedLong, "Long value mismatch after serialization/deserialization");
    }

    // Test double serialization and deserialization
    for (double doubleValue :
        new double[] {
          Double.NaN,
          Double.NEGATIVE_INFINITY,
          Double.POSITIVE_INFINITY,
          0,
          Double.MAX_VALUE,
          Double.MIN_VALUE,
          12345.6789
        }) {
      ByteString doubleBytes = RowBuilder.toBytes(doubleValue);
      double deserializedDouble = RowBuilder.doubleFromBytes(doubleBytes);
      assertEquals(
          doubleValue,
          deserializedDouble,
          "Double value mismatch after serialization/deserialization");
    }

    // Test boolean serialization and deserialization
    for (boolean booleanValue : new boolean[] {true, false}) {
      ByteString booleanBytes = RowBuilder.toBytes(booleanValue);
      boolean deserializedBoolean = RowBuilder.booleanFromBytes(booleanBytes);
      assertEquals(
          booleanValue,
          deserializedBoolean,
          "Boolean value mismatch after serialization/deserialization");
    }

    // Test string serialization and deserialization
    for (String stringValue : new String[] {"testString", "ꙮ", "العربية"}) {
      ByteString stringBytes = RowBuilder.toBytes(stringValue);
      String deserializedString = RowBuilder.stringFromBytes(stringBytes);
      assertEquals(
          stringValue,
          deserializedString,
          "String value mismatch after serialization/deserialization");
    }

    // Test byte array serialization and deserialization
    byte[] byteArrayValue = {1, 2, 3, 4, 5, 0, -128, 127, 0};
    ByteString byteArrayBytes = RowBuilder.toBytes(byteArrayValue);
    byte[] deserializedByteArray = RowBuilder.byteArrayFromBytes(byteArrayBytes);
    assertArrayEquals(
        byteArrayValue,
        deserializedByteArray,
        "Byte array value mismatch after serialization/deserialization");
  }

  @Test
  public void testAddValueLong() {
    aerospike.addValue("myBin", 1L);
    verify(bigtable, times(1))
        .setCell(
            RowBuilder.SCALAR_FAMILY,
            ByteString.copyFromUtf8("myBin__INT64"),
            TIMESTAMP,
            ByteString.copyFrom(Bytes.toBytes(1L)));
    assertTotalNumberOfInvocations(bigtable, 1);
  }

  @Test
  public void testAddValueDouble() {
    aerospike.addValue("myBin", 1.0);
    verify(bigtable, times(1))
        .setCell(
            RowBuilder.SCALAR_FAMILY,
            ByteString.copyFromUtf8("myBin__FLOAT64"),
            TIMESTAMP,
            ByteString.copyFrom(Bytes.toBytes(1.0)));
    assertTotalNumberOfInvocations(bigtable, 1);
  }

  @Test
  public void testAddValueBoolean() {
    aerospike.addValue("myBin", true);
    verify(bigtable, times(1))
        .setCell(
            RowBuilder.SCALAR_FAMILY,
            ByteString.copyFromUtf8("myBin__BOOL"),
            TIMESTAMP,
            ByteString.copyFrom(Bytes.toBytes(true)));
    assertTotalNumberOfInvocations(bigtable, 1);
  }

  @Test
  public void testAddValueString() {
    aerospike.addValue("myBin", "test");
    verify(bigtable, times(1))
        .setCell(
            RowBuilder.SCALAR_FAMILY,
            ByteString.copyFromUtf8("myBin__STRING"),
            TIMESTAMP,
            ByteString.copyFrom(Bytes.toBytes("test")));
    assertTotalNumberOfInvocations(bigtable, 1);
  }

  @Test
  public void testAddValueByteArray() {
    aerospike.addValue("myBin", new byte[] {1, 2, 3});
    verify(bigtable, times(1))
        .setCell(
            RowBuilder.SCALAR_FAMILY,
            ByteString.copyFromUtf8("myBin__BYTES"),
            TIMESTAMP,
            ByteString.copyFrom(new byte[] {1, 2, 3}));
    assertTotalNumberOfInvocations(bigtable, 1);
  }

  @Test
  public void testAddValueList() {
    aerospike.addValue("myBin", List.of(1L, 2.2, "o"));
    verify(bigtable, times(1))
        .setCell(
            RowBuilder.COMPLEX_OBJECT_FAMILY,
            ByteString.copyFromUtf8("myBin__LIST__0__INT64"),
            TIMESTAMP,
            ByteString.copyFrom(Bytes.toBytes(1L)));
    verify(bigtable, times(1))
        .setCell(
            RowBuilder.COMPLEX_OBJECT_FAMILY,
            ByteString.copyFromUtf8("myBin__LIST__1__FLOAT64"),
            TIMESTAMP,
            ByteString.copyFrom(Bytes.toBytes(2.2)));
    verify(bigtable, times(1))
        .setCell(
            RowBuilder.COMPLEX_OBJECT_FAMILY,
            ByteString.copyFromUtf8("myBin__LIST__2__STRING"),
            TIMESTAMP,
            ByteString.copyFrom(Bytes.toBytes("o")));
    assertTotalNumberOfInvocations(bigtable, 3);
  }

  @Test
  public void testAddValueListOfLists() {
    aerospike.addValue("myBin", List.of(List.of(1L), List.of(2.2, "o")));
    verify(bigtable, times(1))
        .setCell(
            RowBuilder.COMPLEX_OBJECT_FAMILY,
            ByteString.copyFromUtf8("myBin__LIST__0__LIST__0__INT64"),
            TIMESTAMP,
            ByteString.copyFrom(Bytes.toBytes(1L)));
    verify(bigtable, times(1))
        .setCell(
            RowBuilder.COMPLEX_OBJECT_FAMILY,
            ByteString.copyFromUtf8("myBin__LIST__1__LIST__0__FLOAT64"),
            TIMESTAMP,
            ByteString.copyFrom(Bytes.toBytes(2.2)));
    verify(bigtable, times(1))
        .setCell(
            RowBuilder.COMPLEX_OBJECT_FAMILY,
            ByteString.copyFromUtf8("myBin__LIST__1__LIST__1__STRING"),
            TIMESTAMP,
            ByteString.copyFrom(Bytes.toBytes("o")));
    assertTotalNumberOfInvocations(bigtable, 3);
  }

  @Test
  public void testAddValueMap() {
    aerospike.addValue("myBin", Map.of("key1", 1L, "key2", 2.2, "key3", "o", "STRING", "s"));
    verify(bigtable, times(1))
        .setCell(
            RowBuilder.COMPLEX_OBJECT_FAMILY,
            ByteString.copyFromUtf8("myBin__MAP__key1__INT64"),
            TIMESTAMP,
            ByteString.copyFrom(Bytes.toBytes(1L)));
    verify(bigtable, times(1))
        .setCell(
            RowBuilder.COMPLEX_OBJECT_FAMILY,
            ByteString.copyFromUtf8("myBin__MAP__key2__FLOAT64"),
            TIMESTAMP,
            ByteString.copyFrom(Bytes.toBytes(2.2)));
    verify(bigtable, times(1))
        .setCell(
            RowBuilder.COMPLEX_OBJECT_FAMILY,
            ByteString.copyFromUtf8("myBin__MAP__key3__STRING"),
            TIMESTAMP,
            ByteString.copyFrom(Bytes.toBytes("o")));
    verify(bigtable, times(1))
        .setCell(
            RowBuilder.COMPLEX_OBJECT_FAMILY,
            ByteString.copyFromUtf8("myBin__MAP__STRING__STRING"),
            TIMESTAMP,
            ByteString.copyFrom(Bytes.toBytes("s")));
    assertTotalNumberOfInvocations(bigtable, 4);
  }

  @Test
  public void testAddValueComplexObject() {
    aerospike.addValue(
        "myBin",
        Map.of(
            "k1", List.of("o", true),
            "k2", List.of(Map.of("nKey1", 1L, "nKey2", 2.2))));
    verify(bigtable, times(1))
        .setCell(
            RowBuilder.COMPLEX_OBJECT_FAMILY,
            ByteString.copyFromUtf8("myBin__MAP__k1__LIST__0__STRING"),
            TIMESTAMP,
            ByteString.copyFrom(Bytes.toBytes("o")));
    verify(bigtable, times(1))
        .setCell(
            RowBuilder.COMPLEX_OBJECT_FAMILY,
            ByteString.copyFromUtf8("myBin__MAP__k1__LIST__1__BOOL"),
            TIMESTAMP,
            ByteString.copyFrom(Bytes.toBytes(true)));
    verify(bigtable, times(1))
        .setCell(
            RowBuilder.COMPLEX_OBJECT_FAMILY,
            ByteString.copyFromUtf8("myBin__MAP__k2__LIST__0__MAP__nKey1__INT64"),
            TIMESTAMP,
            ByteString.copyFrom(Bytes.toBytes(1L)));
    verify(bigtable, times(1))
        .setCell(
            RowBuilder.COMPLEX_OBJECT_FAMILY,
            ByteString.copyFromUtf8("myBin__MAP__k2__LIST__0__MAP__nKey2__FLOAT64"),
            TIMESTAMP,
            ByteString.copyFrom(Bytes.toBytes(2.2)));
    assertTotalNumberOfInvocations(bigtable, 4);
  }

  @Test
  public void testCircularReferenceInMapThrows() {
    Map<Object, Object> map = new HashMap<>();
    map.put("key1", 1L);
    map.put("key2", 2.2);
    map.put("key3", "o");
    map.put("key4", map);
    RuntimeException thrown =
        assertThrows(RuntimeException.class, () -> aerospike.addValue("myBin", map));
    assertTrue(
        thrown.getMessage().contains("Circular reference detected."),
        "GOT: " + thrown.getMessage());
  }

  @Test
  public void testCircularReferenceInListThrows() {
    List<Object> list = new ArrayList<>();
    list.add(1L);
    list.add(list);
    RuntimeException thrown =
        assertThrows(RuntimeException.class, () -> aerospike.addValue("myBin", list));
    assertTrue(
        thrown.getMessage().contains("Circular reference detected."),
        "GOT: " + thrown.getMessage());
  }

  @Test
  public void testInvalidBinName() {
    RuntimeException thrown =
        assertThrows(
            RuntimeException.class, () -> aerospike.addValue("xd" + RowBuilder.DELIMITER, 1L));
    assertTrue(thrown.getMessage().contains("Invalid column id"), "GOT: " + thrown.getMessage());
  }

  @Test
  public void testInvalidMapKey() {
    Map<Object, Object> map = new HashMap<>();
    map.put("xd" + RowBuilder.DELIMITER, 1L);
    RuntimeException thrown =
        assertThrows(RuntimeException.class, () -> aerospike.addValue("myBin", map));
    assertTrue(thrown.getMessage().contains("Invalid column id"), "GOT: " + thrown.getMessage());
  }

  @Test
  public void testInvalidMapContainsNullValue() {
    Map<Object, Object> map = new HashMap<>();
    map.put("key1", null);
    RuntimeException thrown =
        assertThrows(RuntimeException.class, () -> aerospike.addValue("myBin", map));
    assertTrue(
        thrown.getMessage().contains("Unsupported type: null"), "GOT: " + thrown.getMessage());
  }

  @Test
  public void testInvalidMapContainsNullKey() {
    Map<Object, Object> map = new HashMap<>();
    map.put(null, "value");
    RuntimeException thrown =
        assertThrows(RuntimeException.class, () -> aerospike.addValue("myBin", map));
    assertTrue(
        thrown.getMessage().contains("Cannot add a map which contains a null key."),
        "GOT: " + thrown.getMessage());
  }

  @Test
  public void testInvalidMapContainsNonStringKey() {
    Map<Object, Object> map = new HashMap<>();
    map.put(1L, "value");
    RuntimeException thrown =
        assertThrows(RuntimeException.class, () -> aerospike.addValue("myBin", map));
    assertTrue(
        thrown.getMessage().contains("is not supported as a map key"),
        "GOT: " + thrown.getMessage());
  }

  @Test
  public void testInvalidListContainsNull() {
    List<Object> list = new ArrayList<>();
    list.add(null);
    RuntimeException thrown =
        assertThrows(RuntimeException.class, () -> aerospike.addValue("myBin", list));
    assertTrue(
        thrown.getMessage().contains("Unsupported type: null"), "GOT: " + thrown.getMessage());
  }
}
