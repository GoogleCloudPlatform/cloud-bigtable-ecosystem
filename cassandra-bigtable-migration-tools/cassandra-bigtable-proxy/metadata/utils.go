/*
 * Copyright (C) 2025 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not
 * use this file except in compliance with the License. You may obtain a copy of
 * the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
 * WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
 * License for the specific language governing permissions and limitations under
 * the License.
 */

package metadata

import (
	"cloud.google.com/go/bigtable"
	"fmt"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"slices"
	"sort"

	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
)

func CreateTableMap(tables []*TableSchema) map[types.TableName]*TableSchema {
	var result = make(map[types.TableName]*TableSchema)
	for _, table := range tables {
		result[table.Name] = table
	}
	return result
}

// sortPrimaryKeys sorts the primary key columns of each table based on their precedence.
// The function takes a map where the keys are table names and the values are slices of columns.
// It returns the same map with the columns sorted by their primary key precedence.
//
// Parameters:
//   - pkMetadata: A map where keys are table names (strings) and values are slices of Columns structs.
//     Each Columns struct contains metadata about the columns, including primary key precedence.
//
// Returns:
// - A map with the same structure as the input, but with the columns sorted by primary key precedence.
func sortPrimaryKeys(keys []*types.Column) {
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].PkPrecedence < keys[j].PkPrecedence
	})
}

func detectTableEncoding(tableInfo *bigtable.TableInfo, defaultEncoding types.IntRowKeyEncodingType) types.IntRowKeyEncodingType {
	// row key schema is nil if no row key schema is set. This can also happen in
	// unit tests because they bigtable test instance doesn't use row key schemaManager
	if tableInfo.RowKeySchema == nil {
		return defaultEncoding
	}
	for _, field := range tableInfo.RowKeySchema.Fields {
		// assumes all int fields are encoded the same - which should be true if the tables are managed by this application
		switch v := field.FieldType.(type) {
		case bigtable.Int64Type:
			switch v.Encoding.(type) {
			case bigtable.BigEndianBytesEncoding:
				return types.BigEndianEncoding
			case bigtable.Int64OrderedCodeBytesEncoding:
				return types.OrderedCodeEncoding
			}
		}
	}
	return defaultEncoding
}

func createBigtableRowKeySchema(primaryKeys []types.CreateTablePrimaryKeyConfig, cols []types.CreateColumn, intRowKeyEncoding types.IntRowKeyEncodingType) (*bigtable.StructType, error) {
	var rowKeySchemaFields []bigtable.StructField
	for _, key := range primaryKeys {
		pmkIndex := slices.IndexFunc(cols, func(c types.CreateColumn) bool {
			return c.Name == key.Name
		})
		if pmkIndex == -1 {
			return nil, fmt.Errorf("missing primary key `%s` from columns definition", key)
		}
		part, err := createBigtableRowKeyField(cols[pmkIndex], intRowKeyEncoding)
		if err != nil {
			return nil, err
		}
		rowKeySchemaFields = append(rowKeySchemaFields, part)
	}
	return &bigtable.StructType{
		Fields:   rowKeySchemaFields,
		Encoding: bigtable.StructOrderedCodeBytesEncoding{},
	}, nil
}

func createBigtableRowKeyField(col types.CreateColumn, intRowKeyEncoding types.IntRowKeyEncodingType) (bigtable.StructField, error) {
	switch col.TypeInfo.DataType() {
	case datatype.Varchar, datatype.Ascii:
		return bigtable.StructField{FieldName: string(col.Name), FieldType: bigtable.StringType{Encoding: bigtable.StringUtf8BytesEncoding{}}}, nil
	case datatype.Blob, datatype.Uuid, datatype.Timeuuid:
		return bigtable.StructField{FieldName: string(col.Name), FieldType: bigtable.BytesType{Encoding: bigtable.RawBytesEncoding{}}}, nil
	case datatype.Timestamp:
		return bigtable.StructField{FieldName: string(col.Name), FieldType: bigtable.TimestampType{Encoding: bigtable.TimestampUnixMicrosInt64Encoding{UnixMicrosInt64Encoding: bigtable.Int64OrderedCodeBytesEncoding{}}}}, nil
	case datatype.Int, datatype.Bigint:
		switch intRowKeyEncoding {
		case types.OrderedCodeEncoding:
			return bigtable.StructField{FieldName: string(col.Name), FieldType: bigtable.Int64Type{Encoding: bigtable.Int64OrderedCodeBytesEncoding{}}}, nil
		case types.BigEndianEncoding:
			return bigtable.StructField{FieldName: string(col.Name), FieldType: bigtable.Int64Type{Encoding: bigtable.BigEndianBytesEncoding{}}}, nil
		}
		return bigtable.StructField{}, fmt.Errorf("unhandled int row key encoding %v", intRowKeyEncoding)
	default:
		return bigtable.StructField{}, fmt.Errorf("unhandled row key type %s", col.TypeInfo.String())
	}
}
