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
package bigtableclient

import (
	"encoding/base64"
	"fmt"
	"reflect"
	"slices"
	"sort"

	"cloud.google.com/go/bigtable"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/translator"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/message"
)

const (
	DefaultProfileId = "default"
)

// sortPrimaryKeys sorts the primary key columns of each table based on their precedence.
// The function takes a map where the keys are table names and the values are slices of columns.
// It returns the same map with the columns sorted by their primary key precedence.
//
// Parameters:
//   - pkMetadata: A map where keys are table names (strings) and values are slices of Column structs.
//     Each Column struct contains metadata about the columns, including primary key precedence.
//
// Returns:
// - A map with the same structure as the input, but with the columns sorted by primary key precedence.
func sortPrimaryKeys(keys []*types.Column) {
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].PkPrecedence < keys[j].PkPrecedence
	})
}

// GetProfileId returns the provided profile ID if it is not empty.
// If the provided profile ID is empty, it returns a default profile ID.
func GetProfileId(profileId string) string {
	if profileId != "" {
		return profileId
	}
	return DefaultProfileId
}

// GenerateAppliedRowsResult creates a RowsResult message to indicate whether a database operation was applied.
// It generates a single column row result with a boolean indicating the application status.
// it is specifically for if exists and if not exists queries
//
// Parameters:
//   - keyspace: A string representing the name of the keyspace in which the table resides.
//   - tableName: A string representing the name of the table where the operation was attempted.
//   - applied: A boolean flag indicating whether the operation was successfully applied.
//
// Returns: A pointer to a RowsResult object that contains metadata for a single boolean column denoting
//
//	the application status ([applied]) and the corresponding row data indicating true or false.
func GenerateAppliedRowsResult(keyspace string, tableName string, applied bool) *message.RowsResult {
	row := message.Column{0x00}
	if applied {
		row = message.Column{0x01}
	}
	return &message.RowsResult{
		Metadata: &message.RowsMetadata{
			ColumnCount: 1,
			Columns: []*message.ColumnMetadata{
				{
					Keyspace: keyspace,
					Table:    tableName,
					Name:     "[applied]",
					Type:     datatype.Boolean,
				},
			},
		},
		Data: message.RowSet{message.Row{row}},
	}
}

func decodeBase64(k string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(k)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

func isTableBigEndianEncoded(tableInfo *bigtable.TableInfo) bool {
	// row key schema is nil if no row key schema is set
	if tableInfo.RowKeySchema == nil {
		return false
	}
	for _, field := range tableInfo.RowKeySchema.Fields {
		switch v := field.FieldType.(type) {
		case bigtable.Int64Type:
			switch v.Encoding.(type) {
			case bigtable.BigEndianBytesEncoding:
				return true
			}
		}
	}
	return false
}

func createBigtableRowKeySchema(pmks []translator.CreateTablePrimaryKeyConfig, cols []message.ColumnMetadata, encodeIntRowKeysWithBigEndian bool) (*bigtable.StructType, error) {
	var rowKeySchemaFields []bigtable.StructField
	for _, key := range pmks {
		pmkIndex := slices.IndexFunc(cols, func(c message.ColumnMetadata) bool {
			return c.Name == key.Name
		})
		if pmkIndex == -1 {
			return nil, fmt.Errorf("missing primary key `%s` from columns definition", key)
		}
		part, err := createBigtableRowKeyField(cols[pmkIndex], encodeIntRowKeysWithBigEndian)
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

func createBigtableRowKeyField(col message.ColumnMetadata, encodeIntRowKeysWithBigEndian bool) (bigtable.StructField, error) {
	switch col.Type {
	case datatype.Varchar:
		return bigtable.StructField{FieldName: col.Name, FieldType: bigtable.StringType{Encoding: bigtable.StringUtf8BytesEncoding{}}}, nil
	case datatype.Int, datatype.Bigint, datatype.Timestamp:
		if encodeIntRowKeysWithBigEndian {
			return bigtable.StructField{FieldName: col.Name, FieldType: bigtable.Int64Type{Encoding: bigtable.BigEndianBytesEncoding{}}}, nil
		}
		return bigtable.StructField{FieldName: col.Name, FieldType: bigtable.Int64Type{Encoding: bigtable.Int64OrderedCodeBytesEncoding{}}}, nil
	default:
		return bigtable.StructField{}, fmt.Errorf("unhandled row key type %s", col.Type)
	}
}

// inferSQLType attempts to infer the Bigtable SQLType from a Go interface{} value.
// This is a basic implementation and might need enhancement based on actual data types and schema.
func inferSQLType(value interface{}) (bigtable.SQLType, error) {
	switch value.(type) {
	case string:
		return bigtable.StringSQLType{}, nil
	case []byte:
		return bigtable.BytesSQLType{}, nil
	case int, int8, int16, int32, int64:
		return bigtable.Int64SQLType{}, nil
	case float32:
		return bigtable.Float32SQLType{}, nil
	case float64:
		return bigtable.Float64SQLType{}, nil
	case bool:
		return bigtable.Int64SQLType{}, nil
	case []interface{}:
		return bigtable.ArraySQLType{}, nil
	default:
		v := reflect.ValueOf(value)
		if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
			return nil, fmt.Errorf("unsupported type for SQL parameter inference: %T", value)
		}
		elemType := v.Type().Elem()
		if elemType.Kind() == reflect.Interface {
			return nil, fmt.Errorf("cannot infer element type for empty interface slice")
		}
		zeroValue := reflect.Zero(elemType).Interface()
		elemSQLType, err := inferSQLType(zeroValue)
		if err != nil {
			return nil, fmt.Errorf("cannot infer type for array element: %w", err)
		}
		return bigtable.ArraySQLType{ElemType: elemSQLType}, nil

	}
}
