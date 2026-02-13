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

package common

import (
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	schemaMapping "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/metadata"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/testing/mockdata"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"math"
	"testing"
)

func Test_formatValues(t *testing.T) {
	type args struct {
		value     string
		cqlType   types.CqlDataType
		protocolV primitive.ProtocolVersion
	}
	tests := []struct {
		name    string
		args    args
		want    []byte
		wantErr bool
	}{
		{
			name:    "Invalid int",
			args:    args{"abc", types.TypeInt, primitive.ProtocolVersion4},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Invalid bigint",
			args:    args{"abc", types.TypeBigInt, primitive.ProtocolVersion4},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Invalid float",
			args:    args{"abc", types.TypeFloat, primitive.ProtocolVersion4},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Invalid double",
			args:    args{"abc", types.TypeDouble, primitive.ProtocolVersion4},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Invalid boolean",
			args:    args{"abc", types.TypeBoolean, primitive.ProtocolVersion4},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Invalid timestamp",
			args:    args{"abc", types.TypeTimestamp, primitive.ProtocolVersion4},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EncodeScalarForBigtable(tt.args.value, tt.args.cqlType)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_validateRequiredPrimaryKeys(t *testing.T) {
	tests := []struct {
		name        string
		table       *schemaMapping.TableSchema
		assignments []types.Assignment
		err         string
	}{
		{
			name:  "success",
			table: mockdata.GetTableOrDie("test_keyspace", "test_table"),
			assignments: []types.Assignment{
				types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk1"), types.NewParameterizedValue("value1")),
				types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk2"), types.NewParameterizedValue("value2")),
			},
			err: "",
		},
		{
			name:  "missing pk2",
			table: mockdata.GetTableOrDie("test_keyspace", "test_table"),
			assignments: []types.Assignment{
				types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk1"), types.NewParameterizedValue("value1")),
			},
			err: "missing primary key: 'pk2'",
		},
		{
			name:  "extra columns ok",
			table: mockdata.GetTableOrDie("test_keyspace", "test_table"),
			assignments: []types.Assignment{
				types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk1"), types.NewParameterizedValue("value1")),
				types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk2"), types.NewParameterizedValue("value2")),
				types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_blob"), types.NewParameterizedValue("value3")),
			},
			err: "",
		},
		{
			name:  "out of order ok",
			table: mockdata.GetTableOrDie("test_keyspace", "test_table"),
			assignments: []types.Assignment{
				types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_blob"), types.NewParameterizedValue("value0")),
				types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk2"), types.NewParameterizedValue("value1")),
				types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk1"), types.NewParameterizedValue("value2")),
			},
			err: "",
		},
		{
			name:  "mixed types ok",
			table: mockdata.GetTableOrDie("test_keyspace", "user_info"),
			assignments: []types.Assignment{
				types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "user_info", "age"), types.NewParameterizedValue("value1")),
				types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "user_info", "name"), types.NewParameterizedValue("value2")),
			},
			err: "",
		},
		{
			name:        "no params",
			table:       mockdata.GetTableOrDie("test_keyspace", "test_table"),
			assignments: []types.Assignment{},
			err:         "missing primary key: 'pk1'",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRequiredPrimaryKeys(tt.table, tt.assignments)
			if tt.err != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCreateOrderedCodeKey(t *testing.T) {
	tests := []struct {
		name        string
		tableConfig *schemaMapping.TableSchema
		values      []*types.TypedGoValue
		rowKey      []types.DynamicValue
		want        types.RowKey
		wantErr     string
	}{
		{
			name: "simple string",
			tableConfig: schemaMapping.NewTableConfig("keyspace", "table", "cf1", types.OrderedCodeEncoding, []*types.Column{
				{Name: "user_id", CQLType: types.TypeVarchar, KeyType: types.KeyTypePartition, PkPrecedence: 1},
			}),
			values: []*types.TypedGoValue{types.NewTypedGoValue("user1", types.TypeVarchar)},
			rowKey: []types.DynamicValue{types.NewParameterizedValue("value0")},
			want:   "user1",
		},
		{
			name: "int nonzero",
			tableConfig: schemaMapping.NewTableConfig("keyspace", "table", "cf1", types.OrderedCodeEncoding, []*types.Column{
				{Name: "user_id", CQLType: types.TypeBigInt, KeyType: types.KeyTypePartition, PkPrecedence: 1},
			}),
			values: []*types.TypedGoValue{types.NewTypedGoValue(int64(1), types.TypeBigInt)},
			rowKey: []types.DynamicValue{types.NewParameterizedValue("value0")},
			want:   "\x81",
		},
		{
			name: "int32 nonzero",
			tableConfig: schemaMapping.NewTableConfig("keyspace", "table", "cf1", types.OrderedCodeEncoding, []*types.Column{
				{Name: "user_id", CQLType: types.TypeInt, KeyType: types.KeyTypePartition, PkPrecedence: 1},
			}),
			values: []*types.TypedGoValue{types.NewTypedGoValue(int32(1), types.TypeInt)},
			rowKey: []types.DynamicValue{types.NewParameterizedValue("value0")},
			want:   "\x81",
		},
		{
			name: "int32 nonzero big endian",
			tableConfig: schemaMapping.NewTableConfig("keyspace", "table", "cf1", types.BigEndianEncoding, []*types.Column{
				{Name: "user_id", CQLType: types.TypeInt, KeyType: types.KeyTypePartition, PkPrecedence: 1},
			}),
			values: []*types.TypedGoValue{types.NewTypedGoValue(int32(1), types.TypeInt)},
			rowKey: []types.DynamicValue{types.NewParameterizedValue("value0")},
			want:   "\x00\xff\x00\xff\x00\xff\x00\xff\x00\xff\x00\xff\x00\xff\x01",
		},
		{
			name: "int32 max",
			tableConfig: schemaMapping.NewTableConfig("keyspace", "table", "cf1", types.BigEndianEncoding, []*types.Column{
				{Name: "user_id", CQLType: types.TypeInt, KeyType: types.KeyTypePartition, PkPrecedence: 1},
			}),
			values: []*types.TypedGoValue{types.NewTypedGoValue(int64(2147483647), types.TypeBigInt)},
			rowKey: []types.DynamicValue{types.NewParameterizedValue("value0")},
			want:   "\x00\xff\x00\xff\x00\xff\x00\xff\x7f\xff\xff\xff",
		},
		{
			name: "int64 max",
			tableConfig: schemaMapping.NewTableConfig("keyspace", "table", "cf1", types.BigEndianEncoding, []*types.Column{
				{Name: "user_id", CQLType: types.TypeBigInt, KeyType: types.KeyTypePartition, PkPrecedence: 1},
			}),
			values: []*types.TypedGoValue{types.NewTypedGoValue(int64(9223372036854775807), types.TypeBigInt)},
			rowKey: []types.DynamicValue{types.NewParameterizedValue("value0")},
			want:   "\x7f\xff\xff\xff\xff\xff\xff\xff",
		},
		{
			name: "negative int",
			tableConfig: schemaMapping.NewTableConfig("keyspace", "table", "cf1", types.OrderedCodeEncoding, []*types.Column{
				{Name: "user_id", CQLType: types.TypeBigInt, KeyType: types.KeyTypePartition, PkPrecedence: 1},
			}),
			values: []*types.TypedGoValue{types.NewTypedGoValue(int64(-1), types.TypeBigInt)},
			rowKey: []types.DynamicValue{types.NewParameterizedValue("value0")},
			want:   "\x7f",
		},
		{
			name: "negative int big endian fails",
			tableConfig: schemaMapping.NewTableConfig("keyspace", "table", "cf1", types.BigEndianEncoding, []*types.Column{
				{Name: "user_id", CQLType: types.TypeBigInt, KeyType: types.KeyTypePartition, PkPrecedence: 1},
			}),
			values:  []*types.TypedGoValue{types.NewTypedGoValue(int64(-1), types.TypeBigInt)},
			rowKey:  []types.DynamicValue{types.NewParameterizedValue("value0")},
			want:    "",
			wantErr: "row keys with big endian encoding cannot contain negative integer values",
		},
		{
			name: "int zero",
			tableConfig: schemaMapping.NewTableConfig("keyspace", "table", "cf1", types.OrderedCodeEncoding, []*types.Column{
				{Name: "user_id", CQLType: types.TypeBigInt, KeyType: types.KeyTypePartition, PkPrecedence: 1},
			}),
			values: []*types.TypedGoValue{types.NewTypedGoValue(int64(0), types.TypeBigInt)},
			rowKey: []types.DynamicValue{types.NewParameterizedValue("value0")},
			want:   "\x80",
		},
		{
			name: "int64 minvalue",
			tableConfig: schemaMapping.NewTableConfig("keyspace", "table", "cf1", types.OrderedCodeEncoding, []*types.Column{
				{Name: "user_id", CQLType: types.TypeBigInt, KeyType: types.KeyTypePartition, PkPrecedence: 1},
			}),
			values: []*types.TypedGoValue{types.NewTypedGoValue(int64(math.MinInt64), types.TypeBigInt)},
			rowKey: []types.DynamicValue{types.NewParameterizedValue("value0")},
			want:   "\x00\xff\x3f\x80\x00\xff\x00\xff\x00\xff\x00\xff\x00\xff\x00\xff\x00\xff",
		},
		{
			name: "int64 negative value with leading null byte",
			tableConfig: schemaMapping.NewTableConfig("keyspace", "table", "cf1", types.OrderedCodeEncoding, []*types.Column{
				{Name: "user_id", CQLType: types.TypeBigInt, KeyType: types.KeyTypePartition, PkPrecedence: 1},
			}),
			values: []*types.TypedGoValue{types.NewTypedGoValue(int64(-922337203685473), types.TypeBigInt)},
			rowKey: []types.DynamicValue{types.NewParameterizedValue("value0")},
			want:   "\x00\xff\xfc\xb9\x23\xa2\x9c\x77\x9f",
		},
		{
			name: "int32 minvalue",
			tableConfig: schemaMapping.NewTableConfig("keyspace", "table", "cf1", types.OrderedCodeEncoding, []*types.Column{
				{Name: "user_id", CQLType: types.TypeInt, KeyType: types.KeyTypePartition, PkPrecedence: 1},
			}),
			values: []*types.TypedGoValue{types.NewTypedGoValue(int64(math.MinInt32), types.TypeInt)},
			rowKey: []types.DynamicValue{types.NewParameterizedValue("value0")},
			want:   "\x07\x80\x00\xff\x00\xff\x00\xff",
		},
		{
			name: "int minvalue combined",
			tableConfig: schemaMapping.NewTableConfig("keyspace", "table", "cf1", types.OrderedCodeEncoding, []*types.Column{
				{Name: "user_id", CQLType: types.TypeBigInt, KeyType: types.KeyTypePartition, PkPrecedence: 1},
				{Name: "other_id", CQLType: types.TypeInt, KeyType: types.KeyTypePartition, PkPrecedence: 2},
				{Name: "yet_another_id", CQLType: types.TypeVarchar, KeyType: types.KeyTypePartition, PkPrecedence: 3},
			}),
			values: []*types.TypedGoValue{
				types.NewTypedGoValue(int64(math.MinInt64), types.TypeInt),
				types.NewTypedGoValue(int64(math.MinInt32), types.TypeInt),
				types.NewTypedGoValue("id123", types.TypeVarchar),
			},
			rowKey: []types.DynamicValue{
				types.NewParameterizedValue("value0"),
				types.NewParameterizedValue("value1"),
				types.NewParameterizedValue("value2"),
			},
			want: "\x00\xff\x3f\x80\x00\xff\x00\xff\x00\xff\x00\xff\x00\xff\x00\xff\x00\xff\x00\x01\x07\x80\x00\xff\x00\xff\x00\xff\x00\x01\x69\x64\x31\x32\x33",
		},
		{
			name: "int mixed",
			tableConfig: schemaMapping.NewTableConfig("keyspace", "table", "cf1", types.OrderedCodeEncoding, []*types.Column{
				{Name: "user_id", CQLType: types.TypeBigInt, KeyType: types.KeyTypePartition, PkPrecedence: 1},
				{Name: "other_id", CQLType: types.TypeInt, KeyType: types.KeyTypePartition, PkPrecedence: 2},
			}),
			rowKey: []types.DynamicValue{
				types.NewLiteralValue(int64(-43232545)),
				types.NewLiteralValue(int64(-12451)),
			},
			want: "\x0d\x6c\x52\xdf\x00\x01\x1f\xcf\x5d",
		},
		{
			name: "int zero big endian",
			tableConfig: schemaMapping.NewTableConfig("keyspace", "table", "cf1", types.BigEndianEncoding, []*types.Column{
				{Name: "user_id", CQLType: types.TypeBigInt, KeyType: types.KeyTypePartition, PkPrecedence: 1},
			}),
			rowKey: []types.DynamicValue{
				types.NewLiteralValue(int64(0)),
			},
			want: "\x00\xff\x00\xff\x00\xff\x00\xff\x00\xff\x00\xff\x00\xff\x00\xff",
		},
		{
			name: "compound key",
			tableConfig: schemaMapping.NewTableConfig("keyspace", "table", "cf1", types.OrderedCodeEncoding, []*types.Column{
				{Name: "user_id", CQLType: types.TypeVarchar, KeyType: types.KeyTypePartition, PkPrecedence: 1},
				{Name: "team_num", CQLType: types.TypeBigInt, KeyType: types.KeyTypeClustering, PkPrecedence: 2},
				{Name: "city", CQLType: types.TypeVarchar, KeyType: types.KeyTypeClustering, PkPrecedence: 3},
			}),
			values: []*types.TypedGoValue{
				types.NewTypedGoValue("user1", types.TypeVarchar),
				types.NewTypedGoValue(int64(1), types.TypeBigInt),
				types.NewTypedGoValue("new york", types.TypeVarchar),
			},
			rowKey: []types.DynamicValue{
				types.NewParameterizedValue("value0"),
				types.NewParameterizedValue("value1"),
				types.NewParameterizedValue("value2"),
			},
			want: "user1\x00\x01\x81\x00\x01new york",
		},
		{
			name: "compound key big endian",
			tableConfig: schemaMapping.NewTableConfig("keyspace", "table", "cf1", types.BigEndianEncoding, []*types.Column{
				{Name: "user_id", CQLType: types.TypeVarchar, KeyType: types.KeyTypePartition, PkPrecedence: 1},
				{Name: "team_num", CQLType: types.TypeBigInt, KeyType: types.KeyTypeClustering, PkPrecedence: 2},
				{Name: "city", CQLType: types.TypeVarchar, KeyType: types.KeyTypeClustering, PkPrecedence: 3},
			}),
			rowKey: []types.DynamicValue{
				types.NewLiteralValue("user1"),
				types.NewLiteralValue(int64(1)),
				types.NewLiteralValue("new york"),
			},
			want: "user1\x00\x01\x00\xff\x00\xff\x00\xff\x00\xff\x00\xff\x00\xff\x00\xff\x01\x00\x01new york",
		},
		{
			name: "unhandled int row key encoding type",
			tableConfig: schemaMapping.NewTableConfig("keyspace", "table", "cf1", 4 /*unhandled type*/, []*types.Column{
				{Name: "user_id", CQLType: types.TypeVarchar, KeyType: types.KeyTypePartition, PkPrecedence: 1},
				{Name: "team_num", CQLType: types.TypeBigInt, KeyType: types.KeyTypeClustering, PkPrecedence: 2},
				{Name: "city", CQLType: types.TypeVarchar, KeyType: types.KeyTypeClustering, PkPrecedence: 3},
			}),
			rowKey: []types.DynamicValue{
				types.NewLiteralValue("user1"),
				types.NewLiteralValue(int64(1)),
				types.NewLiteralValue("new york"),
			},
			want:    "",
			wantErr: "unhandled int encoding type",
		},
		{
			name: "compound key with trailing empty",
			tableConfig: schemaMapping.NewTableConfig("keyspace", "table", "cf1", types.OrderedCodeEncoding, []*types.Column{
				{Name: "user_id", CQLType: types.TypeVarchar, KeyType: types.KeyTypePartition, PkPrecedence: 1},
				{Name: "team_num", CQLType: types.TypeBigInt, KeyType: types.KeyTypeClustering, PkPrecedence: 2},
				{Name: "city", CQLType: types.TypeVarchar, KeyType: types.KeyTypeClustering, PkPrecedence: 3},
				{Name: "borough", CQLType: types.TypeVarchar, KeyType: types.KeyTypeClustering, PkPrecedence: 4},
			}),
			values: []*types.TypedGoValue{
				types.NewTypedGoValue("user3", types.TypeVarchar),
				types.NewTypedGoValue(int64(3), types.TypeBigInt),
				types.NewTypedGoValue("", types.TypeVarchar),
				types.NewTypedGoValue("", types.TypeVarchar),
			},
			rowKey: []types.DynamicValue{
				types.NewParameterizedValue("value0"),
				types.NewParameterizedValue("value1"),
				types.NewParameterizedValue("value2"),
				types.NewParameterizedValue("value3"),
			},
			want: "user3\x00\x01\x83",
		},
		{
			name: "compound key with trailing empty big endian",
			tableConfig: schemaMapping.NewTableConfig("keyspace", "table", "cf1", types.BigEndianEncoding, []*types.Column{
				{Name: "user_id", CQLType: types.TypeVarchar, KeyType: types.KeyTypePartition, PkPrecedence: 1},
				{Name: "team_num", CQLType: types.TypeBigInt, KeyType: types.KeyTypeClustering, PkPrecedence: 2},
				{Name: "city", CQLType: types.TypeVarchar, KeyType: types.KeyTypeClustering, PkPrecedence: 3},
				{Name: "borough", CQLType: types.TypeVarchar, KeyType: types.KeyTypeClustering, PkPrecedence: 4},
			}),
			values: []*types.TypedGoValue{
				types.NewTypedGoValue("user3", types.TypeVarchar),
				types.NewTypedGoValue(int64(3), types.TypeBigInt),
				types.NewTypedGoValue("", types.TypeVarchar),
				types.NewTypedGoValue("", types.TypeVarchar),
			},
			rowKey: []types.DynamicValue{
				types.NewParameterizedValue("value0"),
				types.NewParameterizedValue("value1"),
				types.NewParameterizedValue("value2"),
				types.NewParameterizedValue("value3"),
			},
			want: "user3\x00\x01\x00\xff\x00\xff\x00\xff\x00\xff\x00\xff\x00\xff\x00\xff\x03",
		},
		{
			name: "compound key with empty middle",
			tableConfig: schemaMapping.NewTableConfig("keyspace", "table", "cf1", types.OrderedCodeEncoding, []*types.Column{
				{Name: "user_id", CQLType: types.TypeBlob, KeyType: types.KeyTypePartition, PkPrecedence: 1},
				{Name: "team_id", CQLType: types.TypeBlob, KeyType: types.KeyTypeClustering, PkPrecedence: 2},
				{Name: "city", CQLType: types.TypeBlob, KeyType: types.KeyTypeClustering, PkPrecedence: 3},
			}),
			values: []*types.TypedGoValue{
				types.NewTypedGoValue("\xa2", types.TypeBlob),
				types.NewTypedGoValue("", types.TypeBlob),
				types.NewTypedGoValue("\xb7", types.TypeBlob),
			},
			rowKey: []types.DynamicValue{
				types.NewParameterizedValue("value0"),
				types.NewParameterizedValue("value1"),
				types.NewParameterizedValue("value2"),
			},
			want: "\xa2\x00\x01\x00\x00\x00\x01\xb7",
		},
		{
			name: "bytes with delimiter",
			tableConfig: schemaMapping.NewTableConfig("keyspace", "table", "cf1", types.OrderedCodeEncoding, []*types.Column{
				{Name: "user_id", CQLType: types.TypeBlob, KeyType: types.KeyTypePartition, PkPrecedence: 1},
			}),
			rowKey: []types.DynamicValue{
				types.NewLiteralValue("\x80\x00\x01\x81"),
			},
			want: "\x80\x00\xff\x01\x81",
		},
		{
			name: "compound key with 2 empty middle fields",
			tableConfig: schemaMapping.NewTableConfig("keyspace", "table", "cf1", types.OrderedCodeEncoding, []*types.Column{
				{Name: "user_id", CQLType: types.TypeBlob, KeyType: types.KeyTypePartition, PkPrecedence: 1},
				{Name: "team_num", CQLType: types.TypeBlob, KeyType: types.KeyTypeClustering, PkPrecedence: 2},
				{Name: "city", CQLType: types.TypeBlob, KeyType: types.KeyTypeClustering, PkPrecedence: 3},
				{Name: "borough", CQLType: types.TypeBlob, KeyType: types.KeyTypeClustering, PkPrecedence: 4},
			}),
			rowKey: []types.DynamicValue{
				types.NewLiteralValue("\xa2"),
				types.NewLiteralValue(""),
				types.NewLiteralValue(""),
				types.NewLiteralValue("\xb7"),
			},
			want: "\xa2\x00\x01\x00\x00\x00\x01\x00\x00\x00\x01\xb7",
		},
		{
			name: "byte strings",
			tableConfig: schemaMapping.NewTableConfig("keyspace", "table", "cf1", types.OrderedCodeEncoding, []*types.Column{
				{Name: "user_id", CQLType: types.TypeBlob, KeyType: types.KeyTypePartition, PkPrecedence: 1},
				{Name: "city", CQLType: types.TypeBlob, KeyType: types.KeyTypeClustering, PkPrecedence: 2},
			}),
			rowKey: []types.DynamicValue{
				types.NewLiteralValue("\xa5"),
				types.NewLiteralValue("\x90"),
			},
			want: "\xa5\x00\x01\x90",
		},
		{
			name: "empty first value",
			tableConfig: schemaMapping.NewTableConfig("keyspace", "table", "cf1", types.OrderedCodeEncoding, []*types.Column{
				{Name: "user_id", CQLType: types.TypeVarchar, KeyType: types.KeyTypePartition, PkPrecedence: 1},
				{Name: "city", CQLType: types.TypeBlob, KeyType: types.KeyTypeClustering, PkPrecedence: 2},
			}),
			rowKey: []types.DynamicValue{
				types.NewLiteralValue(""),
				types.NewLiteralValue("\xaa"),
			},
			want: "\x00\x00\x00\x01\xaa",
		},
		{
			name: "null escaped",
			tableConfig: schemaMapping.NewTableConfig("keyspace", "table", "cf1", types.OrderedCodeEncoding, []*types.Column{
				{Name: "user_id", CQLType: types.TypeVarchar, KeyType: types.KeyTypePartition, PkPrecedence: 1},
				{Name: "city", CQLType: types.TypeVarchar, KeyType: types.KeyTypeClustering, PkPrecedence: 2},
				{Name: "borough", CQLType: types.TypeVarchar, KeyType: types.KeyTypeClustering, PkPrecedence: 3},
			}),
			rowKey: []types.DynamicValue{
				types.NewLiteralValue("nn"),
				types.NewLiteralValue("t\x00t"),
				types.NewLiteralValue("end"),
			},
			want: "nn\x00\x01t\x00\xfft\x00\x01end",
		},
		{
			name: "null escaped (big endian)",
			tableConfig: schemaMapping.NewTableConfig("keyspace", "table", "cf1", types.BigEndianEncoding, []*types.Column{
				{Name: "user_id", CQLType: types.TypeVarchar, KeyType: types.KeyTypePartition, PkPrecedence: 1},
				{Name: "team_num", CQLType: types.TypeBigInt, KeyType: types.KeyTypeClustering, PkPrecedence: 2},
				{Name: "city", CQLType: types.TypeVarchar, KeyType: types.KeyTypeClustering, PkPrecedence: 3},
			}),
			rowKey: []types.DynamicValue{
				types.NewLiteralValue("abcd"),
				types.NewLiteralValue(int64(45)),
				types.NewLiteralValue("name"),
			},
			want: "abcd\x00\x01\x00\xff\x00\xff\x00\xff\x00\xff\x00\xff\x00\xff\x00\xff\x2d\x00\x01name",
		},
		{
			name: "null char",
			tableConfig: schemaMapping.NewTableConfig("keyspace", "table", "cf1", types.OrderedCodeEncoding, []*types.Column{
				{Name: "user_id", CQLType: types.TypeVarchar, KeyType: types.KeyTypePartition, PkPrecedence: 1},
			}),
			rowKey: []types.DynamicValue{
				types.NewLiteralValue("\x00\x01"),
			},
			want: "\x00\xff\x01",
		},
		{
			name: "missing parameter",
			tableConfig: schemaMapping.NewTableConfig("keyspace", "table", "cf1", types.OrderedCodeEncoding, []*types.Column{
				{Name: "user_id", CQLType: types.TypeVarchar, KeyType: types.KeyTypePartition, PkPrecedence: 1},
			}),
			values: []*types.TypedGoValue{},
			rowKey: []types.DynamicValue{
				types.NewParameterizedValue("value1"),
			},
			want:    "",
			wantErr: "no query param for value1",
		},
		{
			name: "null value",
			tableConfig: schemaMapping.NewTableConfig("keyspace", "table", "cf1", types.OrderedCodeEncoding, []*types.Column{
				{Name: "user_id", CQLType: types.TypeVarchar, KeyType: types.KeyTypePartition, PkPrecedence: 1},
			}),
			rowKey: []types.DynamicValue{
				types.NewLiteralValue(nil),
			},
			want:    "",
			wantErr: "value cannot be null for primary key 'user_id'",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values, err := mockdata.CreateQueryParams(tt.values)
			require.NoError(t, err)
			got, err := BindRowKey(tt.tableConfig, tt.rowKey, values)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestEncodeBool(t *testing.T) {
	trueValue := true
	falseValue := false
	tests := []struct {
		name    string
		value   interface{}
		want    types.BigtableValue
		wantErr string
	}{
		{
			name:  "true",
			value: true,
			want:  []byte{0, 0, 0, 0, 0, 0, 0, 1},
		},
		{
			name:  "false",
			value: false,
			want:  []byte{0, 0, 0, 0, 0, 0, 0, 0},
		},
		{
			name:  "true pointer",
			value: &trueValue,
			want:  []byte{0, 0, 0, 0, 0, 0, 0, 1},
		},
		{
			name:  "false pointer",
			value: &falseValue,
			want:  []byte{0, 0, 0, 0, 0, 0, 0, 0},
		},
		{
			name:    "strings not supported",
			value:   "true",
			want:    nil,
			wantErr: "unsupported type: string",
		},
		{
			name:    "Unsupported type",
			value:   1,
			want:    nil,
			wantErr: "unsupported type: int",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := encodeBoolForBigtable(tt.value)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestEncodeInt(t *testing.T) {
	tests := []struct {
		name    string
		value   interface{}
		want    []byte
		wantErr string
	}{
		{
			name:  "Valid string input",
			value: int64(12),
			want:  []byte{0, 0, 0, 0, 0, 0, 0, 12},
		},
		{
			name:    "String parsing error",
			value:   "12",
			want:    nil,
			wantErr: "unsupported type for bigint: string",
		},
		{
			name:  "Valid int32 input",
			value: int32(12),
			want:  []byte{0, 0, 0, 0, 0, 0, 0, 12},
		},
		{
			name:    "Valid []byte input",
			value:   []byte{0, 0, 0, 12},
			wantErr: "unsupported type for bigint: []uint8",
		},
		{
			name:    "Unsupported type",
			value:   12.34,
			want:    nil,
			wantErr: "unsupported type for bigint: float64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := encodeBigIntForBigtable(tt.value)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTrimQuotes(t *testing.T) {
	tests := []struct {
		value string
		want  string
	}{
		{
			value: `foo`,
			want:  `foo`,
		},
		{
			value: `f`,
			want:  `f`,
		},
		{
			value: `fo`,
			want:  `fo`,
		},
		{
			value: ``,
			want:  ``,
		},
		{
			value: `''`,
			want:  ``,
		},
		{
			value: `'foo'`,
			want:  `foo`,
		},
		{
			value: `'''foo'`,
			want:  `'foo`,
		},
		{
			value: `'foo'''`,
			want:  `foo'`,
		},
		// only trim the outermost quotes
		{
			value: `'sister''s'`,
			want:  `sister's`,
		},
		// should keep inner quotes
		{
			value: `'"foo"'`,
			want:  `"foo"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got := TrimQuotes(tt.value)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRelationFragment(t *testing.T) {
	input := ":my_marker_1"

	p, errs := setupParser(input)

	m := p.Marker()
	assert.Empty(t, errs.Errors)

	qp := types.NewQueryParameterBuilder()
	col := &types.Column{
		Name:         "Foo",
		ColumnFamily: "cf",
		CQLType:      types.TypeText,
		IsPrimaryKey: false,
		PkPrecedence: 0,
		KeyType:      types.KeyTypeRegular,
		Metadata:     message.ColumnMetadata{},
	}
	result, err := ParseMarker(m, types.TypeText, qp, col)
	require.NoError(t, err)
	param, ok := result.(*types.ParameterizedValue)
	assert.True(t, ok)
	assert.Equal(t, types.Parameter("my_marker_1"), param.Parameter)

	builtParams, err := qp.Build()
	assert.NoError(t, err)

	metadata, err := builtParams.GetMetadata(param.Parameter)
	require.NoError(t, err)
	assert.Equal(t, types.Parameter("my_marker_1"), metadata.Key)
	assert.Equal(t, types.TypeText, metadata.Type)
	assert.Equal(t, true, metadata.IsNamed)
	assert.Equal(t, (*types.Column)(nil), metadata.Column)
}
