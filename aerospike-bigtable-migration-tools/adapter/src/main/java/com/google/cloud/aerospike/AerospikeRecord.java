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

import com.google.cloud.bigtable.data.v2.models.Row;
import com.google.cloud.bigtable.data.v2.models.RowCell;
import com.google.common.primitives.Bytes;
import com.google.protobuf.ByteString;
import gnu.crypto.hash.RipeMD160;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Base64;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;

/**
 * Represents a record in Aerospike, constructed from a Bigtable {@link Row}. This class maps
 * Bigtable cells to Aerospike bins based on predefined types.
 */
public class AerospikeRecord {
  // https://github.com/aerospike/aerospike-server/blob/8.0.0.8/as/include/base/datamodel.h#L131
  private static final byte PARTICLE_TYPE_STRING = 3;

  /** A map of bin names to their corresponding values. */
  public final Map<String, Object> bins;

  /**
   * Utility function for generating an aerospike record identifier which is used as a row key in
   * Bigtable. This identifier is a Base64 encoded hash of three concatenated components: set name,
   * key particle type, and key value. This utility function is used only for the most common type
   * of keys - string. For other types of keys, we advise using
   * com.aerospike.client.util.Crypto.computeDigest from the official java Aerospike Client api.
   *
   * @param set Aerospike set name
   * @param key Aerospike row key
   * @return aerospike record identifier encoded to Base64
   */
  public static String getRowKeyFromAerospikeKey(String set, String key) {
    // Convert strings to UTF-8 byte arrays
    byte[] setBytes = set.getBytes(StandardCharsets.UTF_8);
    byte[] keyBytes = key.getBytes(StandardCharsets.UTF_8);
    byte[] result = Bytes.concat(setBytes, new byte[] {PARTICLE_TYPE_STRING}, keyBytes);

    RipeMD160 hash = new RipeMD160();
    hash.update(result, 0, result.length);

    return Base64.getEncoder().encodeToString(hash.digest());
  }

  protected static <T> void extendListWithNulls(List<T> list, int targetSize) {
    if (targetSize < 0) {
      throw new IllegalArgumentException("targetSize must not be negative");
    }
    while (list.size() < targetSize) {
      list.add(null);
    }
  }

  protected static <T> void putAtIndex(List<T> list, int idx, T val) {
    if (idx < 0) {
      throw new IndexOutOfBoundsException("Index cannot be negative: " + idx);
    }
    extendListWithNulls(list, idx + 1);
    list.set(idx, val);
  }

  /**
   * Places the value in the result map with the provided key. Assumes remainingPathParts contain
   * [key, type, ...] where: key is a string key of the map, type is one of the value types defined
   * in {@link RowBuilder}, the rest of the path parts are either empty or represent further nested
   * structure.
   *
   * <p>If the type is a list or a map, it will recursively build the structure using the remaining
   * path parts. If the type is a scalar, it will convert the value to the appropriate type
   *
   * @param result result {@link Map} to be filled in
   * @param remainingPathParts Bigtable column name's parts not yet consumed by this function
   * @param value Bigtable value
   */
  protected static void buildRecursive(
      Map<String, Object> result, List<String> remainingPathParts, ByteString value) {
    if (remainingPathParts.size() <= 1) {
      throw new IllegalArgumentException("Invalid path: " + remainingPathParts);
    }
    String binName = remainingPathParts.get(0);
    String type = remainingPathParts.get(1);
    Object sub = result.get(binName);
    if (remainingPathParts.size() == 2) {
      result.put(binName, scalarFromBytes(type, value));
    } else if (type.equals(RowBuilder.LIST_TYPE) && (sub == null || sub instanceof List)) {
      List<Object> subList = (List<Object>) sub;
      if (subList == null) {
        subList = new ArrayList<>();
        result.put(binName, subList);
      }
      buildRecursive(subList, remainingPathParts.subList(2, remainingPathParts.size()), value);
    } else if (type.equals(RowBuilder.MAP_TYPE) && (sub == null || sub instanceof Map)) {
      Map<String, Object> subMap = (Map<String, Object>) sub;
      if (subMap == null) {
        subMap = new HashMap<>();
        result.put(binName, subMap);
      }
      buildRecursive(subMap, remainingPathParts.subList(2, remainingPathParts.size()), value);
    } else {
      throw new IllegalArgumentException(
          String.format(
              "Invalid combination of annotated type '%s' and actual type '%s' for bin '%s' in"
                  + " path '%s'.",
              type,
              Optional.ofNullable(sub).map(o -> o.getClass().getName()).orElse("null"),
              binName,
              remainingPathParts));
    }
  }

  /**
   * Places the value at the provided index, filling gaps in the result list with nulls. Assumes
   * remainingPathParts contain [index, type, ...] where: index is a string representation of an
   * unsigned integer, type is one of the value types defined in {@link RowBuilder}, the rest of the
   * path parts are either empty or represent further nested structure.
   *
   * <p>If the type is a list or a map, it will recursively build the structure using the remaining
   * path parts. If the type is a scalar, it will convert the value to the appropriate type.
   *
   * @param result result {@link List} to be filled in
   * @param remainingPathParts Bigtable column name's parts not yet consumed by this function
   * @param value Bigtable value
   */
  private static void buildRecursive(
      List<Object> result, List<String> remainingPathParts, ByteString value) {
    if (remainingPathParts.size() <= 1) {
      throw new IllegalArgumentException("Invalid path: " + remainingPathParts);
    }
    String indexPathPart = remainingPathParts.get(0);
    String type = remainingPathParts.get(1);
    int idx;
    try {
      idx = Integer.parseUnsignedInt(indexPathPart);
    } catch (NumberFormatException e) {
      throw new IllegalArgumentException("Invalid index: " + indexPathPart);
    }
    extendListWithNulls(result, idx + 1);
    Object sub = result.get(idx);

    if (remainingPathParts.size() == 2) {
      putAtIndex(result, idx, scalarFromBytes(type, value));
    } else if (type.equals(RowBuilder.LIST_TYPE) && (sub == null || sub instanceof List)) {
      List<Object> subList = (List<Object>) sub;
      if (subList == null) {
        subList = new ArrayList<>();
        putAtIndex(result, idx, subList);
      }
      buildRecursive(subList, remainingPathParts.subList(2, remainingPathParts.size()), value);
    } else if (type.equals(RowBuilder.MAP_TYPE) && (sub == null || sub instanceof Map)) {
      Map<String, Object> subMap = (Map<String, Object>) sub;
      if (subMap == null) {
        subMap = new HashMap<>();
        result.set(idx, subMap);
      }
      buildRecursive(subMap, remainingPathParts.subList(2, remainingPathParts.size()), value);
    } else {
      throw new IllegalArgumentException(
          String.format(
              "Invalid combination of annotated type '%s' and actual type '%s' for index '%s' in"
                  + " path '%s'.",
              type,
              Optional.ofNullable(sub).map(o -> o.getClass().getName()).orElse("null"),
              indexPathPart,
              remainingPathParts));
    }
  }

  public static Object scalarFromBytes(String type, ByteString rawValue) {
    return switch (type) {
      case RowBuilder.BOOLEAN_TYPE -> RowBuilder.booleanFromBytes(rawValue);
      case RowBuilder.INT64_TYPE -> RowBuilder.integerFromBytes(rawValue);
      case RowBuilder.FLOAT64_TYPE -> RowBuilder.doubleFromBytes(rawValue);
      case RowBuilder.STRING_TYPE -> RowBuilder.stringFromBytes(rawValue);
      case RowBuilder.BYTES_TYPE -> rawValue.toByteArray();
      default -> throw new IllegalArgumentException("Unknown scalar type: " + type);
    };
  }

  /**
   * Constructs an {@code AerospikeRecord} from a Bigtable {@link Row}. The rows are expected to
   * have been created using {@link RowBuilder}, which stores the bin names and types in the
   * qualifiers of the cells. For the exact format of the qualifiers, see {@link RowBuilder}
   * `addValue` methods.
   *
   * @param row the Bigtable row to be converted
   */
  public AerospikeRecord(Row row) {
    bins = new HashMap<>();
    for (RowCell cell : row.getCells(RowBuilder.SCALAR_FAMILY)) {
      String[] splitQualifier = cell.getQualifier().toStringUtf8().split(RowBuilder.DELIMITER);
      if (splitQualifier.length != 2) {
        throw new IllegalArgumentException(
            "Invalid qualifier format: " + cell.getQualifier().toStringUtf8());
      }
      String binName = splitQualifier[0];
      String type = splitQualifier[1];
      bins.put(binName, scalarFromBytes(type, cell.getValue()));
    }
    for (RowCell cell : row.getCells(RowBuilder.COMPLEX_OBJECT_FAMILY)) {
      String[] splitQualifier = cell.getQualifier().toStringUtf8().split(RowBuilder.DELIMITER);
      buildRecursive(bins, Arrays.asList(splitQualifier), cell.getValue());
    }
  }
}
