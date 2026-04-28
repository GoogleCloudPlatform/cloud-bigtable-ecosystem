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
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import com.google.cloud.bigtable.data.v2.models.Row;
import com.google.cloud.bigtable.data.v2.models.RowCell;
import com.google.protobuf.ByteString;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.StringJoiner;
import java.util.stream.Stream;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.MethodSource;
import org.mockito.Mockito;

public class AerospikeRecordTest {
  @Test
  public void testPutAtIndex0() {
    List<String> list = new java.util.ArrayList<>();
    AerospikeRecord.putAtIndex(list, 0, "value");
    assertEquals(List.of("value"), list);
  }

  @Test
  public void testPutAtIndex1() {
    List<String> list = new java.util.ArrayList<>();
    AerospikeRecord.putAtIndex(list, 1, "value");
    assertNull(list.get(0));
    assertEquals("value", list.get(1));
    assertEquals(2, list.size());
  }

  @Test
  public void testBuildRecursiveFlat() {
    Map<String, Object> result = new HashMap<>();
    List<String> path = List.of("binName", RowBuilder.STRING_TYPE);
    ByteString byteResult = ByteString.copyFromUtf8("value");
    AerospikeRecord.buildRecursive(result, path, byteResult);
    assertEquals(Map.of("binName", "value"), result);
  }

  @Test
  public void testBuildRecursiveList() {
    Map<String, Object> result = new HashMap<>();
    List<String> path = List.of("binName", "LIST", "0", RowBuilder.STRING_TYPE);
    ByteString byteResult = ByteString.copyFromUtf8("value");
    AerospikeRecord.buildRecursive(result, path, byteResult);
    assertEquals(Map.of("binName", List.of("value")), result);
  }

  @Test
  public void testBuildRecursiveMap() {
    Map<String, Object> result = new HashMap<>();
    List<String> path = List.of("binName", "MAP", "key1", RowBuilder.STRING_TYPE);
    ByteString byteResult = ByteString.copyFromUtf8("value");
    AerospikeRecord.buildRecursive(result, path, byteResult);
    assertInstanceOf(Map.class, result.get("binName"));
    assertEquals(Map.of("binName", Map.of("key1", "value")), result);
  }

  @Test
  public void testBuildRecursiveObject() {
    Map<String, Object> result = new HashMap<>();
    List<String> path = List.of("binName", "MAP", "key1", "LIST", "0", RowBuilder.STRING_TYPE);
    ByteString byteResult = ByteString.copyFromUtf8("value");
    AerospikeRecord.buildRecursive(result, path, byteResult);
    assertEquals(Map.of("binName", Map.of("key1", List.of("value"))), result);
  }

  static Stream<List<String>> provideInvalidPaths() {
    return Stream.of(
        List.of("binName"),
        List.of("binName", "", RowBuilder.STRING_TYPE),
        List.of("binName", "MAP"),
        List.of("binName", "MAP", "key"),
        List.of("binName", "MAP", RowBuilder.STRING_TYPE),
        List.of("binName", "LIST"),
        List.of("binName", "LIST", RowBuilder.STRING_TYPE),
        List.of("binName", "LIST", "0", "MAP", RowBuilder.STRING_TYPE));
  }

  @ParameterizedTest
  @MethodSource("provideInvalidPaths")
  public void testBuildRecursiveInvalidMapObject(List<String> path) {
    Map<String, Object> result = new HashMap<>();
    ByteString byteResult = ByteString.copyFromUtf8("value");
    assertThrows(
        IllegalArgumentException.class,
        () -> AerospikeRecord.buildRecursive(result, path, byteResult));
  }

  @Test
  public void testConstructor() {
    Row row = Mockito.mock(Row.class);
    Mockito.when(row.getCells(RowBuilder.SCALAR_FAMILY)).thenReturn(new ArrayList<>());
    RowCell value = Mockito.mock(RowCell.class);
    Mockito.when(row.getCells(RowBuilder.COMPLEX_OBJECT_FAMILY)).thenReturn(List.of(value));
    String qualifierStr =
        new StringJoiner(RowBuilder.DELIMITER)
            .add("binName")
            .add(RowBuilder.LIST_TYPE)
            .add("0")
            .add(RowBuilder.STRING_TYPE)
            .toString();
    ByteString qualifier = ByteString.copyFromUtf8(qualifierStr);
    Mockito.when(value.getQualifier()).thenReturn(qualifier);
    ByteString valueBytes = ByteString.copyFromUtf8("value");
    Mockito.when(value.getValue()).thenReturn(valueBytes);
    AerospikeRecord record = new AerospikeRecord(row);
    assertInstanceOf(List.class, record.bins.get("binName"));
    List<String> list = (List<String>) record.bins.get("binName");
    assertEquals(1, list.size());
    assertEquals("value", list.get(0));
  }

  @Test
  public void testConstructorInvalidSeparator() {
    Row row = Mockito.mock(Row.class);
    Mockito.when(row.getCells(RowBuilder.SCALAR_FAMILY)).thenReturn(new ArrayList<>());
    RowCell value = Mockito.mock(RowCell.class);
    Mockito.when(row.getCells(RowBuilder.COMPLEX_OBJECT_FAMILY)).thenReturn(List.of(value));
    String qualifierStr =
        new StringJoiner("X")
            .add("binName")
            .add(RowBuilder.LIST_TYPE)
            .add("0")
            .add(RowBuilder.STRING_TYPE)
            .toString();
    ByteString qualifier = ByteString.copyFromUtf8(qualifierStr);
    Mockito.when(value.getQualifier()).thenReturn(qualifier);
    ByteString valueBytes = ByteString.copyFromUtf8("value");
    Mockito.when(value.getValue()).thenReturn(valueBytes);
    RuntimeException thrown =
        assertThrows(IllegalArgumentException.class, () -> new AerospikeRecord(row));
    assertTrue(thrown.getMessage().contains("Invalid path"));
  }
}
