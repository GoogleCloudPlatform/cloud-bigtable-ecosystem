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

import com.google.cloud.kafka.connect.bigtable.mapping.LogicalTypeUtils;
import com.google.cloud.kafka.connect.bigtable.mapping.SchemaUtils;
import java.util.Map;
import java.util.Optional;
import org.apache.kafka.connect.data.Schema;
import org.apache.kafka.connect.data.SchemaAndValue;
import org.apache.kafka.connect.data.Struct;
import org.apache.kafka.connect.errors.DataException;

public class SchemaParsingUtils {

  public static SchemaAndValue extractField(SchemaAndValue keySchemaAndValue, String[] fields) {
    return extractField(keySchemaAndValue, fields, 0);
  }

  /**
   * Extract possibly nested fields from the input value. If more than one field is provided, this
   * method will recursively iterate over the entire field array until it reaches the last field,
   * and then returns that field's schema and value.
   *
   * @param keySchemaAndValue {@link org.apache.kafka.connect.sink.SinkRecord SinkRecord's} key or
   *     some it's child with corresponding {@link Schema}.
   * @param fields Fields that need to be accessed before the target value is reached. This array
   *     represents a path to a field, where each element is a child of the previous element's
   *     field.
   * @param index Index of the field that is being extracted.
   * @return Extracted nested field.
   */
  private static SchemaAndValue extractField(
      SchemaAndValue keySchemaAndValue, String[] fields, int index) {
    Object value = keySchemaAndValue.value();
    Optional<Schema> schema = Optional.ofNullable(keySchemaAndValue.schema());
    ensureKeyElementIsNotNull(value);
    LogicalTypeUtils.logIfLogicalTypeUnsupported(schema);
    if (index >= fields.length) {
      return keySchemaAndValue;
    }
    String field = fields[index];
    if (value instanceof Struct) {
      // Note that getWithoutDefault() throws if such a field does not exist.
      Object fieldValue = ((Struct) value).getWithoutDefault(field);
      Schema fieldSchema = SchemaUtils.maybeExtractFieldSchema(schema, field).orElse(null);
      return extractField(new SchemaAndValue(fieldSchema, fieldValue), fields, index + 1);
    } else if (value instanceof Map<?, ?>) {
      Object fieldValue = ((Map<?, ?>) value).get(field);
      Schema fieldSchema = SchemaUtils.maybeExtractFieldSchema(schema, field).orElse(null);
      return extractField(new SchemaAndValue(fieldSchema, fieldValue), fields, index + 1);
    } else {
      throw new DataException(
          "Unexpected class `"
              + value.getClass().getName()
              + "` doesn't support extracting field `"
              + field
              + "` using a dot.");
    }
  }

  public static void ensureKeyElementIsNotNull(Object value) {
    if (value == null) {
      // Matching Confluent's sink behavior.
      throw new DataException("The extracted field value cannot be null.");
    }
  }
}
