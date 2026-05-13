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
package com.google.cloud.aerospike.mapping;

import com.google.cloud.aerospike.RowBuilder;
import com.google.protobuf.ByteString;
import java.util.HashMap;
import java.util.Map;
import org.apache.kafka.connect.data.Schema;
import org.apache.kafka.connect.data.SchemaBuilder;
import org.apache.kafka.connect.data.Struct;

/**
 * This class lets the user create a Kafka Connect {@link Struct} that can be used as {@see
 * com.google.cloud.kafka.connect.bigtable.BigtableSinkConnector} input to write Bigtable cells
 * equivalent to input Aerospike values.
 */
public class SinkMutationBuilder extends RowBuilder<SinkMutationBuilder.CellSetter> {
  public SinkMutationBuilder() {
    super(new CellSetter());
  }

  public Struct getStruct() {
    return this.cellSetter.build();
  }

  /**
   * A {@link RowBuilder.CellSetter} implementation that creates a Kafka Connect {@link Struct} that
   * can be then consumed by {@see com.google.cloud.kafka.connect.bigtable.BigtableSinkConnector} to
   * write appropriate values to Bigtable.
   */
  public static class CellSetter implements RowBuilder.CellSetter {
    private final Map<String, Map<String, ByteString>> values;

    public CellSetter() {
      values = new HashMap<>();
    }

    public Struct build() {
      Map<String, Struct> familyStructs = new HashMap<>();
      for (Map.Entry<String, Map<String, ByteString>> family : values.entrySet()) {
        SchemaBuilder familySchemaBuilder = SchemaBuilder.struct();
        for (String columnQualifier : family.getValue().keySet()) {
          familySchemaBuilder.field(columnQualifier, SchemaBuilder.BYTES_SCHEMA);
        }
        Schema familySchema = familySchemaBuilder.build();
        Struct familyStruct = new Struct(familySchema);
        for (Map.Entry<String, ByteString> column : family.getValue().entrySet()) {
          familyStruct.put(column.getKey(), column.getValue().toByteArray());
        }
        familyStructs.put(family.getKey(), familyStruct);
      }

      SchemaBuilder schemaBuilder = SchemaBuilder.struct();
      for (Map.Entry<String, Struct> familyStruct : familyStructs.entrySet()) {
        schemaBuilder.field(familyStruct.getKey(), familyStruct.getValue().schema());
      }
      Struct result = new Struct(schemaBuilder.build());
      for (Map.Entry<String, Struct> familyStruct : familyStructs.entrySet()) {
        result.put(familyStruct.getKey(), familyStruct.getValue());
      }
      return result;
    }

    @Override
    public void setCell(String family, String qualifier, ByteString value) {
      Map<String, ByteString> familyMap =
          values.computeIfAbsent(family, ignored -> new HashMap<>());
      familyMap.put(qualifier, value);
    }
  }
}
