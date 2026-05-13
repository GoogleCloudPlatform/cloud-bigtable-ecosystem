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

import com.google.common.collect.Sets;
import com.google.protobuf.ByteString;
import java.util.List;
import java.util.Map;
import java.util.Map.Entry;
import java.util.Optional;
import java.util.Set;
import javax.annotation.Nonnull;
import org.apache.hadoop.hbase.util.Bytes;

/**
 * This class lets the user map Aerospike values to objects that can be used to store equivalent
 * values in Bigtable cells.
 *
 * <p>It makes use of appropriate columns as defined by the adapter specification using an interface
 * similar to Aerospike's (including proper handling of lists and maps).
 *
 * @param <T> Type of consumer of cells set with {@link RowBuilder#setCell} calls.
 */
public class RowBuilder<T extends RowBuilder.CellSetter> {
  public static final String INT64_TYPE = "INT64";
  public static final String FLOAT64_TYPE = "FLOAT64";
  public static final String BOOLEAN_TYPE = "BOOL";
  public static final String STRING_TYPE = "STRING";
  public static final String BYTES_TYPE = "BYTES";
  public static final String LIST_TYPE = "LIST";
  public static final String MAP_TYPE = "MAP";
  public static final String DELIMITER = "__";
  public static final String COMPLEX_OBJECT_FAMILY = "OBJECT";
  public static final String SCALAR_FAMILY = "SCALARS";

  protected final T cellSetter;

  /**
   * Converts a long value to a ByteString.
   *
   * @param value the long value to convert.
   * @return the ByteString representation of the value.
   */
  public static ByteString toBytes(long value) {
    return ByteString.copyFrom(Bytes.toBytes(value));
  }

  /**
   * Converts a ByteString to a long value.
   *
   * @param value the ByteString to convert.
   * @return the long value.
   */
  public static long integerFromBytes(ByteString value) {
    return Bytes.toLong(value.toByteArray());
  }

  /**
   * Converts a double value to a ByteString.
   *
   * @param value the double value to convert.
   * @return the ByteString representation of the value.
   */
  public static ByteString toBytes(double value) {
    return ByteString.copyFrom(Bytes.toBytes(value));
  }

  /**
   * Converts a ByteString to a double value.
   *
   * @param value the ByteString to convert.
   * @return the double value.
   */
  public static double doubleFromBytes(ByteString value) {
    return Bytes.toDouble(value.toByteArray());
  }

  /**
   * Converts a boolean value to a ByteString.
   *
   * @param value the boolean value to convert.
   * @return the ByteString representation of the value.
   */
  public static ByteString toBytes(boolean value) {
    return ByteString.copyFrom(Bytes.toBytes(value));
  }

  /**
   * Converts a ByteString to a boolean value.
   *
   * @param value the ByteString to convert.
   * @return the boolean value.
   */
  public static boolean booleanFromBytes(ByteString value) {
    return Bytes.toBoolean(value.toByteArray());
  }

  /**
   * Converts a string value to a ByteString.
   *
   * @param value the string value to convert.
   * @return the ByteString representation of the value.
   */
  public static ByteString toBytes(String value) {
    return ByteString.copyFrom(Bytes.toBytes(value));
  }

  /**
   * Converts a ByteString to a string value.
   *
   * @param value the ByteString to convert.
   * @return the string value.
   */
  public static String stringFromBytes(ByteString value) {
    return Bytes.toString(value.toByteArray());
  }

  /**
   * Converts a byte array value to a ByteString.
   *
   * @param value the bytes to convert.
   * @return the ByteString representation of the value.
   */
  public static ByteString toBytes(byte[] value) {
    return ByteString.copyFrom(value);
  }

  /**
   * Converts a ByteString to a string value.
   *
   * @param value the ByteString to convert.
   * @return the byte array value.
   */
  public static byte[] byteArrayFromBytes(ByteString value) {
    return value.toByteArray();
  }

  /**
   * Validates the column identifier to ensure it does not contain the delimiter.
   *
   * @param columnIdentifier the column identifier to validate.
   * @throws IllegalArgumentException if the column identifier contains the delimiter.
   */
  public static void validateColumnIdentifier(String columnIdentifier) {
    if (columnIdentifier.contains(DELIMITER)) {
      throw new IllegalArgumentException(
          "Invalid column id:"
              + columnIdentifier
              + "! ColumnId cannot contain \""
              + DELIMITER
              + "\"");
    }
  }

  protected RowBuilder(T cellSetter) {
    this.cellSetter = cellSetter;
  }

  protected void setCell(String columnFamily, String columnIdentifier, long value) {
    cellSetter.setCell(columnFamily, columnIdentifier + DELIMITER + INT64_TYPE, toBytes(value));
  }

  protected void setCell(String columnFamily, String columnIdentifier, double value) {
    cellSetter.setCell(columnFamily, columnIdentifier + DELIMITER + FLOAT64_TYPE, toBytes(value));
  }

  protected void setCell(String columnFamily, String columnIdentifier, byte[] value) {
    cellSetter.setCell(columnFamily, columnIdentifier + DELIMITER + BYTES_TYPE, toBytes(value));
  }

  protected void setCell(String columnFamily, String columnIdentifier, String value) {
    cellSetter.setCell(columnFamily, columnIdentifier + DELIMITER + STRING_TYPE, toBytes(value));
  }

  protected void setCell(String columnFamily, String columnIdentifier, boolean value) {
    cellSetter.setCell(columnFamily, columnIdentifier + DELIMITER + BOOLEAN_TYPE, toBytes(value));
  }

  /**
   * Adds a long value to the mutation for the specified column identifier.
   *
   * @param columnIdentifier the identifier of the column.
   * @param value the long value to add.
   * @return the updated {@link RowBuilder} object.
   */
  public RowBuilder<T> addValue(String columnIdentifier, long value) {
    validateColumnIdentifier(columnIdentifier);
    setCell(SCALAR_FAMILY, columnIdentifier, value);
    return this;
  }

  /**
   * Adds a double value to the mutation for the specified column identifier.
   *
   * @param columnIdentifier the identifier of the column.
   * @param value the double value to add.
   * @return the updated {@link RowBuilder} object.
   */
  public RowBuilder<T> addValue(String columnIdentifier, double value) {
    validateColumnIdentifier(columnIdentifier);
    setCell(SCALAR_FAMILY, columnIdentifier, value);
    return this;
  }

  /**
   * Adds a boolean value to the mutation for the specified column identifier.
   *
   * @param columnIdentifier the identifier of the column.
   * @param value the boolean value to add.
   * @return the updated {@link RowBuilder} object.
   */
  public RowBuilder<T> addValue(String columnIdentifier, boolean value) {
    validateColumnIdentifier(columnIdentifier);
    setCell(SCALAR_FAMILY, columnIdentifier, value);
    return this;
  }

  /**
   * Adds a string value to the mutation for the specified column identifier.
   *
   * @param columnIdentifier the identifier of the column.
   * @param value the string value to add.
   * @return the updated {@link RowBuilder} object.
   */
  public RowBuilder<T> addValue(String columnIdentifier, @Nonnull String value) {
    validateColumnIdentifier(columnIdentifier);
    setCell(SCALAR_FAMILY, columnIdentifier, value);
    return this;
  }

  /**
   * Adds a byte array value to the mutation for the specified column identifier.
   *
   * @param columnIdentifier the identifier of the column.
   * @param value the byte array value to add.
   * @return the updated {@link RowBuilder} object.
   */
  public RowBuilder<T> addValue(String columnIdentifier, @Nonnull byte[] value) {
    validateColumnIdentifier(columnIdentifier);
    setCell(SCALAR_FAMILY, columnIdentifier, value);
    return this;
  }

  /**
   * Adds a list of values to the mutation for the specified column identifier. The column
   * identifier format is: ${COLUMN_IDENTIFIER}__${INDEX}__${TYPE} where ${COLUMN_IDENTIFIER} is the
   * provided identifier, ${INDEX} is the index of the value in the list, and ${TYPE} is the type of
   * the value.
   *
   * @param columnIdentifier the identifier of the column.
   * @param value the list value to add.
   * @return the updated {@link RowBuilder} object.
   */
  public RowBuilder<T> addValue(String columnIdentifier, @Nonnull List<Object> value) {
    validateColumnIdentifier(columnIdentifier);
    addValueRecursive(columnIdentifier, value, Sets.newIdentityHashSet());
    return this;
  }

  /**
   * Adds a map of values to the mutation for the specified column identifier. The column identifier
   * format is: ${COLUMN_IDENTIFIER}__${KEY}__${TYPE} where ${COLUMN_IDENTIFIER} is the provided
   * identifier, ${KEY} is the key of the value in the map, and ${TYPE} is the type of the value.
   *
   * @param columnIdentifier the identifier of the column.
   * @param value the map value to add.
   * @return the updated {@link RowBuilder} object.
   */
  public RowBuilder<T> addValue(String columnIdentifier, @Nonnull Map<Object, Object> value) {
    validateColumnIdentifier(columnIdentifier);
    addValueRecursive(columnIdentifier, value, Sets.newIdentityHashSet());
    return this;
  }

  private void addValueRecursive(
      String columnIdentifier, List<Object> list, Set<Object> referenceSet) {
    if (referenceSet.contains(list)) {
      throw new RuntimeException("Circular reference detected.");
    } else {
      referenceSet.add(list);
    }
    for (int i = 0; i < list.size(); i++) {
      Object val = list.get(i);
      String newIdentifier = columnIdentifier + DELIMITER + LIST_TYPE + DELIMITER + i;
      addValueRecursive(val, newIdentifier, referenceSet);
    }
  }

  private void addValueRecursive(
      String columnIdentifier, Map<Object, Object> map, Set<Object> referenceSet) {
    if (referenceSet.contains(map)) {
      throw new RuntimeException("Circular reference detected.");
    } else {
      referenceSet.add(map);
    }
    for (Entry<Object, Object> entry : map.entrySet()) {
      Object val = entry.getValue();
      Object key = entry.getKey();
      if (key == null) {
        throw new RuntimeException("Cannot add a map which contains a null key.");
      }
      String mapKey;
      if (key instanceof String s) {
        mapKey = s;
      } else {
        throw new RuntimeException(
            "'" + key.getClass().getName() + "' is not supported as a map key.");
      }
      validateColumnIdentifier(mapKey);
      String newIdentifier = columnIdentifier + DELIMITER + MAP_TYPE + DELIMITER + mapKey;
      addValueRecursive(val, newIdentifier, referenceSet);
    }
  }

  private void addValueRecursive(Object val, String newIdentifier, Set<Object> referenceSet) {
    if (val instanceof Long || val instanceof Integer) {
      setCell(COMPLEX_OBJECT_FAMILY, newIdentifier, ((Number) val).longValue());
    } else if (val instanceof Double || val instanceof Float) {
      setCell(COMPLEX_OBJECT_FAMILY, newIdentifier, ((Number) val).doubleValue());
    } else if (val instanceof Boolean b) {
      setCell(COMPLEX_OBJECT_FAMILY, newIdentifier, b);
    } else if (val instanceof String s) {
      setCell(COMPLEX_OBJECT_FAMILY, newIdentifier, s);
    } else if (val instanceof byte[] b) {
      setCell(COMPLEX_OBJECT_FAMILY, newIdentifier, b);
    } else if (val instanceof List) {
      addValueRecursive(newIdentifier, (List<Object>) val, referenceSet);
    } else if (val instanceof Map) {
      addValueRecursive(newIdentifier, (Map<Object, Object>) val, referenceSet);
    } else {
      throw new RuntimeException(
          "Unsupported type: "
              + Optional.ofNullable(val).map(o -> o.getClass().getName()).orElse("null"));
    }
  }

  /**
   * An interface used by {@link RowBuilder} subclasses to connect logic defined in {@see
   * RowBuilder#setCell} methods with implementation-specific builder of an output object equivalent
   * to the input Aerospike values.
   */
  public interface CellSetter {
    /**
     * Set an appropriate cell in the internal builder of an output object.
     *
     * @param family Bigtable column family to be written to
     * @param qualifier Bigtable column qualifier to be written to
     * @param value Bigtable value to be written
     */
    void setCell(String family, String qualifier, ByteString value);
  }
}
