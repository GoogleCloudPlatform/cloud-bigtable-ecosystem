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
package responsehandler

import (
	"reflect"
	"testing"

	types "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	schemaMapping "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/schema-mapping"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/third_party/datastax/proxycore"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/google/go-cmp/cmp"
	"go.uber.org/zap"
)

func TestExtractUniqueKeys(t *testing.T) {
	tests := []struct {
		name     string
		rowMap   []map[string]interface{}
		query    QueryMetadata
		expected []string
	}{
		{
			name: "Single nested map with unique keys",
			rowMap: []map[string]interface{}{
				{"key1": 1, "key2": 2},
			},
			query: QueryMetadata{
				IsStar: true,
			},
			expected: []string{"key1", "key2"},
		},
		{
			name: "Multiple nested maps with overlapping keys",
			rowMap: []map[string]interface{}{
				{"key1": 1, "key2": 2},
				{"key2": 3, "key3": 4},
			},
			query: QueryMetadata{
				IsStar: true,
			},
			expected: []string{"key1", "key2", "key3"},
		},
		{
			name: "Empty input map",
			rowMap: []map[string]interface{}{
				{},
			},
			query: QueryMetadata{
				IsStar: true,
			},
			expected: []string{},
		},
		{
			name:   "Nil input map",
			rowMap: nil,
			query: QueryMetadata{
				IsStar: true,
			},
			expected: []string{},
		},
		{
			name: "Nested maps with empty keys",
			rowMap: []map[string]interface{}{
				{"": 1},
				{"key1": 2, "key2": 3},
			},
			query: QueryMetadata{
				IsStar: true,
			},
			expected: []string{"", "key1", "key2"},
		},
		{
			name: "Test case 5: selected columns",
			rowMap: []map[string]interface{}{
				{"key1": 1, "key2": 2},
			},
			query: QueryMetadata{
				SelectedColumns: []schemaMapping.SelectedColumns{
					{
						Name: "key1",
					},
				},
			},
			expected: []string{"key1"},
		},
		{
			name: "Test case 6: selected columns with alias",
			rowMap: []map[string]interface{}{
				{"key1": 1, "key2": 2},
			},
			query: QueryMetadata{
				SelectedColumns: []schemaMapping.SelectedColumns{
					{
						Name:  "key1",
						Alias: "key1_alias",
					},
				},
			},
			expected: []string{"key1_alias"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractUniqueKeys(tt.rowMap, tt.query)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("ExtractUniqueKeys() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestTypeHandler_HandleMapType(t *testing.T) {
	type fields struct {
		Logger              *zap.Logger
		SchemaMappingConfig *schemaMapping.SchemaMappingConfig
		ColumnMetadataCache map[string]map[string]message.ColumnMetadata
	}
	type args struct {
		mapData   map[string]interface{}
		mr        *message.Row
		mapType   datatype.MapType
		protocalV primitive.ProtocolVersion
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "Invalid interger map decoded",
			fields: fields{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: nil,
				ColumnMetadataCache: nil,
			},
			args: args{
				mapData: map[string]interface{}{
					"key1": []byte{0x01},
					"key2": []byte{0x0012},
				},
				mr:        &message.Row{},
				mapType:   datatype.NewMapType(datatype.Varchar, datatype.Int),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: true,
		},
		{
			name: "Invalid bigint map decoded",
			fields: fields{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: nil,
				ColumnMetadataCache: nil,
			},
			args: args{
				mapData: map[string]interface{}{
					"key1": []byte{0x01},
					"key2": []byte{0x0012},
				},
				mr:        &message.Row{},
				mapType:   datatype.NewMapType(datatype.Varchar, datatype.Bigint),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: true,
		},
		{
			name: "Invalid float map decoded",
			fields: fields{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: nil,
				ColumnMetadataCache: nil,
			},
			args: args{
				mapData: map[string]interface{}{
					"key1": []byte{0x01},
					"key2": []byte{0x023},
				},
				mr:        &message.Row{},
				mapType:   datatype.NewMapType(datatype.Varchar, datatype.Float),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: true,
		},
		{
			name: "Invalid double map decoded",
			fields: fields{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: nil,
				ColumnMetadataCache: nil,
			},
			args: args{
				mapData: map[string]interface{}{
					"key1": []byte{0x01},
					"key2": []byte{0x0012},
				},
				mr:        &message.Row{},
				mapType:   datatype.NewMapType(datatype.Varchar, datatype.Double),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: true,
		},
		{
			name: "Invalid timestamp map decoded",
			fields: fields{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: nil,
				ColumnMetadataCache: nil,
			},
			args: args{
				mapData: map[string]interface{}{
					"key1": []byte{0x01},
					"key2": []byte{0x0012},
				},
				mr:        &message.Row{},
				mapType:   datatype.NewMapType(datatype.Varchar, datatype.Timestamp),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: true,
		},
		{
			name: "invalid integer map",
			fields: fields{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: nil,
				ColumnMetadataCache: nil,
			},
			args: args{
				mapData: map[string]interface{}{
					"key1": "    ",
				},
				mr:        &message.Row{},
				mapType:   datatype.NewMapType(datatype.Varchar, datatype.Int),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: true,
		},
		{
			name: "invalid bigint map",
			fields: fields{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: nil,
				ColumnMetadataCache: nil,
			},
			args: args{
				mapData: map[string]interface{}{
					"key1": "    ",
				},
				mr:        &message.Row{},
				mapType:   datatype.NewMapType(datatype.Varchar, datatype.Bigint),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: true,
		},
		{
			name: "invalid float map",
			fields: fields{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: nil,
				ColumnMetadataCache: nil,
			},
			args: args{
				mapData: map[string]interface{}{
					"key1": "    ",
				},
				mr:        &message.Row{},
				mapType:   datatype.NewMapType(datatype.Varchar, datatype.Float),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: true,
		},
		{
			name: "invalid double map",
			fields: fields{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: nil,
				ColumnMetadataCache: nil,
			},
			args: args{
				mapData: map[string]interface{}{
					"key1": "    ",
				},
				mr:        &message.Row{},
				mapType:   datatype.NewMapType(datatype.Varchar, datatype.Double),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: true,
		},
		{
			name: "invalid timestamp map",
			fields: fields{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: nil,
				ColumnMetadataCache: nil,
			},
			args: args{
				mapData: map[string]interface{}{
					"key1": "    ",
				},
				mr:        &message.Row{},
				mapType:   datatype.NewMapType(datatype.Varchar, datatype.Timestamp),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: true,
		},
		{
			name: "invalid boolean map",
			fields: fields{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: nil,
				ColumnMetadataCache: nil,
			},
			args: args{
				mapData: map[string]interface{}{
					"key1": "    ",
				},
				mr:        &message.Row{},
				mapType:   datatype.NewMapType(datatype.Varchar, datatype.Boolean),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: true,
		},
		{
			name: "Valid boolean map",
			fields: fields{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: nil,
				ColumnMetadataCache: nil,
			},
			args: args{
				mapData: map[string]interface{}{
					"key1": []byte{0, 0, 0, 0, 0, 0, 0, 1},
					"key2": []byte{0, 0, 0, 0, 0, 0, 0, 0},
				},
				mr:        &message.Row{},
				mapType:   datatype.NewMapType(datatype.Varchar, datatype.Boolean),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: false,
		},
		{
			name: "Invalid boolean map - incorrect value type",
			fields: fields{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: nil,
				ColumnMetadataCache: nil,
			},
			args: args{
				mapData: map[string]interface{}{
					"key1": "not a byte array",
				},
				mr:        &message.Row{},
				mapType:   datatype.NewMapType(datatype.Varchar, datatype.Boolean),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: true,
		},
		{
			name: "Valid string map",
			fields: fields{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: nil,
				ColumnMetadataCache: nil,
			},
			args: args{
				mapData: map[string]interface{}{
					"key1": []byte("value1"),
					"key2": []byte("value2"),
				},
				mr:        &message.Row{},
				mapType:   datatype.NewMapType(datatype.Varchar, datatype.Varchar),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: false,
		},
		{
			name: "Valid integer map",
			fields: fields{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: nil,
				ColumnMetadataCache: nil,
			},
			args: args{
				mapData: map[string]interface{}{
					"key1": []byte{0, 0, 0, 0, 0, 0, 0, 10},
					"key2": []byte{0, 0, 0, 0, 0, 0, 0, 12},
				},
				mr:        &message.Row{},
				mapType:   datatype.NewMapType(datatype.Varchar, datatype.Int),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: false,
		},
		{
			name: "Valid big int map",
			fields: fields{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: nil,
				ColumnMetadataCache: nil,
			},
			args: args{
				mapData: map[string]interface{}{
					"key1": []byte("12345678"),
					"key2": []byte("45671234"),
				},
				mr:        &message.Row{},
				mapType:   datatype.NewMapType(datatype.Varchar, datatype.Bigint),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: false,
		},
		{
			name: "Valid timestamp map",
			fields: fields{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: nil,
				ColumnMetadataCache: nil,
			},
			args: args{
				mapData: map[string]interface{}{
					"key1": []byte("17307156"),
				},
				mr:        &message.Row{},
				mapType:   datatype.NewMapType(datatype.Varchar, datatype.Timestamp),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: false,
		},
		{
			name: "Valid float map",
			fields: fields{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: nil,
				ColumnMetadataCache: nil,
			},
			args: args{
				mapData: map[string]interface{}{
					"key1": []byte("1.71"),
				},
				mr:        &message.Row{},
				mapType:   datatype.NewMapType(datatype.Varchar, datatype.Float),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: false,
		},
		{
			name: "Valid double map",
			fields: fields{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: nil,
				ColumnMetadataCache: nil,
			},
			args: args{
				mapData: map[string]interface{}{
					"key1": []byte("1.711212"),
				},
				mr:        &message.Row{},
				mapType:   datatype.NewMapType(datatype.Varchar, datatype.Double),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: false,
		},
		{
			name: "Unsupported element type",
			fields: fields{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: nil,
				ColumnMetadataCache: nil,
			},
			args: args{
				mapData: map[string]interface{}{
					"key1": []byte("value1"),
				},
				mr:        &message.Row{},
				mapType:   datatype.NewMapType(datatype.Varchar, datatype.NewCustomType("foo")),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			th := TypeHandler{
				Logger:              tt.fields.Logger,
				SchemaMappingConfig: tt.fields.SchemaMappingConfig,
				ColumnMetadataCache: tt.fields.ColumnMetadataCache,
			}
			if err := th.HandleMapType(tt.args.mapData, tt.args.mr, tt.args.mapType, tt.args.protocalV); (err != nil) != tt.wantErr {
				t.Errorf("TypeHandler.HandleMapType() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTypeHandler_HandleSetType(t *testing.T) {
	type fields struct {
		Logger              *zap.Logger
		SchemaMappingConfig *schemaMapping.SchemaMappingConfig
		ColumnMetadataCache map[string]map[string]message.ColumnMetadata
	}
	type args struct {
		arr       []interface{}
		mr        *message.Row
		setType   datatype.SetType
		protocalV primitive.ProtocolVersion
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "Handle string list",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				arr:       []interface{}{[]byte("a"), []byte("b"), []byte("c")},
				mr:        &message.Row{},
				setType:   datatype.NewSetType(datatype.Varchar),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: false,
		},
		{
			name: "Handle int list",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				arr:       []interface{}{[]byte{0, 0, 0, 0, 0, 0, 0, 10}, []byte{0, 0, 0, 0, 0, 0, 0, 12}},
				mr:        &message.Row{},
				setType:   datatype.NewSetType(datatype.Int),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: false,
		},
		{
			name: "Handle int list Error",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				arr:       []interface{}{"     "},
				mr:        &message.Row{},
				setType:   datatype.NewSetType(datatype.Int),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: true,
		},
		{
			name: "Handle float list Error",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				arr:       []interface{}{"     "},
				mr:        &message.Row{},
				setType:   datatype.NewSetType(datatype.Float),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: true,
		},
		{
			name: "Handle double list Error",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				arr:       []interface{}{"     "},
				mr:        &message.Row{},
				setType:   datatype.NewSetType(datatype.Double),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: true,
		},
		{
			name: "Handle timestamp list Error",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				arr:       []interface{}{"     "},
				mr:        &message.Row{},
				setType:   datatype.NewSetType(datatype.Timestamp),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: true,
		},
		{
			name: "Handle int set",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				arr:       []interface{}{[]byte{0, 0, 0, 0, 0, 0, 0, 10}, []byte{0, 0, 0, 0, 0, 0, 0, 12}},
				mr:        &message.Row{},
				setType:   datatype.NewSetType(datatype.Int),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: false,
		},
		{
			name: "Handle bigint list",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				arr:       []interface{}{[]byte{0, 0, 0, 0, 0, 0, 0, 10}, []byte{0, 0, 0, 0, 0, 0, 0, 12}},
				mr:        &message.Row{},
				setType:   datatype.NewSetType(datatype.Bigint),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: false,
		},
		{
			name: "Handle bigint list Error",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				arr:       []interface{}{"     "},
				mr:        &message.Row{},
				setType:   datatype.NewSetType(datatype.Bigint),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: true,
		},
		{
			name: "Handle bigint set",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				arr:       []interface{}{[]byte{0, 0, 0, 0, 0, 0, 0, 10}, []byte{0, 0, 0, 0, 0, 0, 0, 12}},
				mr:        &message.Row{},
				setType:   datatype.NewSetType(datatype.Bigint),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: false,
		},
		{
			name: "Handle float list",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				arr:       []interface{}{[]byte{0, 0, 0, 10}, []byte{0, 0, 0, 12}},
				mr:        &message.Row{},
				setType:   datatype.NewSetType(datatype.Float),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: false,
		},
		{
			name: "Handle float set",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				arr:       []interface{}{[]byte{0, 0, 0, 10}, []byte{0, 0, 0, 12}},
				mr:        &message.Row{},
				setType:   datatype.NewSetType(datatype.Float),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: false,
		},
		{
			name: "Handle double list",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				arr:       []interface{}{[]byte{0, 0, 0, 0, 0, 0, 0, 10}, []byte{0, 0, 0, 0, 0, 0, 0, 12}},
				mr:        &message.Row{},
				setType:   datatype.NewSetType(datatype.Double),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: false,
		},
		{
			name: "Handle double set",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				arr:       []interface{}{[]byte{0, 0, 0, 0, 0, 0, 0, 10}, []byte{0, 0, 0, 0, 0, 0, 0, 12}},
				mr:        &message.Row{},
				setType:   datatype.NewSetType(datatype.Double),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: false,
		},
		{
			name: "Handle timestamp list",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				arr:       []interface{}{[]byte{0, 0, 0, 0, 0, 0, 0, 10}, []byte{0, 0, 0, 0, 0, 0, 0, 12}},
				mr:        &message.Row{},
				setType:   datatype.NewSetType(datatype.Timestamp),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: false,
		},
		{
			name: "Handle timestamp set",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				arr:       []interface{}{[]byte{0, 0, 0, 0, 0, 0, 0, 10}, []byte{0, 0, 0, 0, 0, 0, 0, 12}},
				mr:        &message.Row{},
				setType:   datatype.NewSetType(datatype.Timestamp),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: false,
		},
		{
			name: "Handle string set",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				arr:       []interface{}{[]byte("a"), []byte("b"), []byte("c")},
				mr:        &message.Row{},
				setType:   datatype.NewSetType(datatype.Varchar),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: false,
		},
		{
			name: "Handle boolean list",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				arr:       []interface{}{[]byte{0, 0, 0, 0, 0, 0, 0, 1}, []byte{0, 0, 0, 0, 0, 0, 0, 0}, []byte{0, 0, 0, 0, 0, 0, 0, 1}},
				mr:        &message.Row{},
				setType:   datatype.NewSetType(datatype.Boolean),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: false,
		},
		{
			name: "Handle boolean set",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				arr:       []interface{}{[]byte{0, 0, 0, 0, 0, 0, 0, 1}, []byte{0, 0, 0, 0, 0, 0, 0, 0}, []byte{0, 0, 0, 0, 0, 0, 0, 1}},
				mr:        &message.Row{},
				setType:   datatype.NewSetType(datatype.Boolean),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: false,
		},
		{
			name: "Unsupported datatype",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				arr:       []interface{}{"11", "21", "51"},
				mr:        &message.Row{},
				setType:   datatype.NewSetType(datatype.NewCustomType("foo")),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			th := &TypeHandler{
				Logger:              tt.fields.Logger,
				SchemaMappingConfig: tt.fields.SchemaMappingConfig,
				ColumnMetadataCache: tt.fields.ColumnMetadataCache,
			}
			err := th.HandleSetType(tt.args.arr, tt.args.mr, tt.args.setType, tt.args.protocalV)
			if (err != nil) != tt.wantErr {
				t.Errorf("TypeHandler.HandleSetType() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTypeHandler_BuildMetadata(t *testing.T) {
	type fields struct {
		Logger              *zap.Logger
		SchemaMappingConfig *schemaMapping.SchemaMappingConfig
		ColumnMetadataCache map[string]map[string]message.ColumnMetadata
	}
	type args struct {
		rowMap []map[string]interface{}
		query  QueryMetadata
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantCmd []*message.ColumnMetadata
		wantErr bool
	}{
		{
			name: "Success",
			fields: fields{
				Logger:              zap.NewExample(),
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			args: args{
				rowMap: []map[string]interface{}{
					{
						"name": "Bob",
					},
				},
				query: QueryMetadata{
					TableName:           "user_info",
					Query:               "SELECT name FROM test_keyspace.user_info;",
					KeyspaceName:        "test_keyspace",
					IsStar:              false,
					DefaultColumnFamily: "cf1",
					SelectedColumns: []schemaMapping.SelectedColumns{
						{
							Name: "name",
						},
					},
				},
			},
			wantCmd: []*message.ColumnMetadata{
				{
					Keyspace: "test_keyspace",
					Table:    "user_info",
					Name:     "name",
					Index:    0,
					Type:     datatype.Varchar,
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			th := &TypeHandler{
				Logger:              tt.fields.Logger,
				SchemaMappingConfig: tt.fields.SchemaMappingConfig,
				ColumnMetadataCache: tt.fields.ColumnMetadataCache,
			}
			gotCmd, err := th.BuildMetadata(tt.args.rowMap, tt.args.query)
			if (err != nil) != tt.wantErr {
				t.Errorf("TypeHandler.BuildMetadata() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotCmd, tt.wantCmd) {
				t.Errorf("TypeHandler.BuildMetadata() gotCmd = %v, want %v", gotCmd, tt.wantCmd)
			}
		})
	}
}
func TestTypeHandler_BuildResponseRow(t *testing.T) {
	// Pre-compute expected encoded bytes for aggregate function test cases.
	encodedInt, err := proxycore.EncodeType(datatype.Bigint, primitive.ProtocolVersion4, int64(10))
	if err != nil {
		t.Fatalf("failed to encode aggregate int64 value: %v", err)
	}
	encodedAlias, err := proxycore.EncodeType(datatype.Bigint, primitive.ProtocolVersion4, int64(20))
	if err != nil {
		t.Fatalf("failed to encode aggregate alias int64 value: %v", err)
	}
	var expectedAggAlias message.Row = []message.Column{encodedAlias}
	var expectedAggInt message.Row = []message.Column{encodedInt}

	type fields struct {
		Logger              *zap.Logger
		SchemaMappingConfig *schemaMapping.SchemaMappingConfig
		ColumnMetadataCache map[string]map[string]message.ColumnMetadata
	}
	type args struct {
		rowMap map[string]interface{}
		query  QueryMetadata
		cmd    []*message.ColumnMetadata
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    message.Row
		wantErr bool
	}{
		{
			name: "Success for string data type",
			fields: fields{
				Logger:              zap.NewExample(),
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			args: args{
				rowMap: map[string]interface{}{
					"name": []byte{0x01},
				},
				query: QueryMetadata{
					TableName:           "user_info",
					Query:               "SELECT name FROM test_keyspace.user_info;",
					KeyspaceName:        "test_keyspace",
					IsStar:              false,
					DefaultColumnFamily: "cf1",
					SelectedColumns: []schemaMapping.SelectedColumns{
						{
							Name: "name",
						},
					},
				},
				cmd: []*message.ColumnMetadata{
					{
						Keyspace: "test_keyspace",
						Table:    "user_info",
						Name:     "name",
						Index:    0,
						Type:     datatype.Varchar,
					},
				},
				// mapKeyArray: []string{"name"},
			},
			want:    message.Row{[]byte{0x01}},
			wantErr: false,
		},
		{
			name: "Success for map with key collection",
			fields: fields{
				Logger:              zap.NewExample(),
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			args: args{
				rowMap: map[string]interface{}{
					"column8['mapKey']": []byte("mapKeyValue"),
				},
				query: QueryMetadata{
					TableName:           "test_table",
					Query:               "SELECT column8['mapKey'] FROM test_table;",
					KeyspaceName:        "test_keyspace",
					IsStar:              false,
					DefaultColumnFamily: "cf1",
					SelectedColumns: []schemaMapping.SelectedColumns{
						{
							Name: "column8['mapKey']", MapKey: "mapKey", ColumnName: "column8",
						},
					},
				},
				cmd: []*message.ColumnMetadata{
					{
						Keyspace: "test_keyspace",
						Table:    "test_table",
						Name:     "column8['mapKey']",
						Index:    0,
						Type:     datatype.Varchar,
					},
				},
			},
			want:    message.Row{[]byte("mapKeyValue")},
			wantErr: false,
		},
		{
			name: "Aggregate count with int64 value",
			fields: fields{
				Logger:              zap.NewExample(),
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			args: args{
				rowMap: map[string]interface{}{
					"id": int64(10),
				},
				query: QueryMetadata{
					TableName:           "user_info",
					Query:               "SELECT count(id) FROM test_keyspace.user_info;",
					KeyspaceName:        "test_keyspace",
					IsStar:              false,
					DefaultColumnFamily: "cf1",
					SelectedColumns: []schemaMapping.SelectedColumns{
						{
							Name:     "id",
							IsFunc:   true,
							FuncName: "count",
						},
					},
				},
				cmd: []*message.ColumnMetadata{
					{
						Keyspace: "test_keyspace",
						Table:    "user_info",
						Name:     "id",
						Index:    0,
						// Type will be overridden in the aggregate branch.
						Type: datatype.Varchar,
					},
				},
			},
			want:    message.Row(expectedAggInt),
			wantErr: false,
		},
		{
			name: "Aggregate count with float64 value",
			fields: fields{
				Logger:              zap.NewExample(),
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			args: args{
				// Even if the value is provided as float64, for count, it will be converted to int64.
				rowMap: map[string]interface{}{
					"id": float64(10),
				},
				query: QueryMetadata{
					TableName:           "user_info",
					Query:               "SELECT count(id) FROM test_keyspace.user_info;",
					KeyspaceName:        "test_keyspace",
					IsStar:              false,
					DefaultColumnFamily: "cf1",
					SelectedColumns: []schemaMapping.SelectedColumns{
						{
							Name:     "id",
							IsFunc:   true,
							FuncName: "count",
						},
					},
				},
				cmd: []*message.ColumnMetadata{
					{
						Keyspace: "test_keyspace",
						Table:    "user_info",
						Name:     "id",
						Index:    0,
						Type:     datatype.Varchar,
					},
				},
			},
			want:    message.Row(expectedAggInt),
			wantErr: false,
		},
		{
			name: "Aggregate count with alias mapping",
			fields: fields{
				Logger:              zap.NewExample(),
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			args: args{
				// When an alias is provided, the rowMap value is expected to be a nested map.
				rowMap: map[string]interface{}{
					"id": int64(20),
				},
				query: QueryMetadata{
					TableName:           "user_info",
					Query:               "SELECT count(id) as id FROM test_keyspace.user_info;",
					KeyspaceName:        "test_keyspace",
					IsStar:              false,
					DefaultColumnFamily: "cf1",
					SelectedColumns: []schemaMapping.SelectedColumns{
						{
							Name:     "id",
							IsFunc:   true,
							FuncName: "count",
							Alias:    "id",
						},
					},
				},
				cmd: []*message.ColumnMetadata{
					{
						Keyspace: "test_keyspace",
						Table:    "user_info",
						Name:     "id",
						Index:    0,
						Type:     datatype.Varchar,
					},
				},
			},
			want:    message.Row(expectedAggAlias),
			wantErr: false,
		},
		{
			name: "Success in writetime query",
			fields: fields{
				Logger:              zap.NewExample(),
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			args: args{
				rowMap: map[string]interface{}{
					"abcd": []byte{0, 0, 0, 0, 0, 0, 0, 12},
				},
				query: QueryMetadata{
					TableName:           "test_table",
					Query:               "SELECT writetime(column5) as abcd FROM test_table;",
					KeyspaceName:        "test_keyspace",
					IsStar:              false,
					DefaultColumnFamily: "cf1",
					SelectedColumns: []schemaMapping.SelectedColumns{
						{
							Name:              "writetime(column5)",
							ColumnName:        "column5",
							IsWriteTimeColumn: true,
							Alias:             "abcd",
						},
					},
				},
				cmd: []*message.ColumnMetadata{
					{
						Keyspace: "test_keyspace",
						Table:    "test_table",
						Name:     "abcd",
						Index:    0,
						Type:     datatype.Timestamp,
					},
				},
			},
			want: message.Row{
				[]byte{0, 0, 0, 0, 0, 0, 0, 12},
			},
			wantErr: false,
		},
		{
			name: "Success in simple `as` keyword",
			fields: fields{
				Logger:              zap.NewExample(),
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			args: args{
				rowMap: map[string]interface{}{
					"abcd": []byte{0, 0, 0, 0, 0, 0, 0, 12},
				},
				query: QueryMetadata{
					TableName:           "test_table",
					Query:               "SELECT column5 as abcd FROM test_table;",
					KeyspaceName:        "test_keyspace",
					IsStar:              false,
					DefaultColumnFamily: "cf1",
					SelectedColumns: []schemaMapping.SelectedColumns{
						{
							Name:       "column5",
							Alias:      "abcd",
							ColumnName: "column5",
							IsAs:       true,
						},
					},
				},
				cmd: []*message.ColumnMetadata{
					{
						Keyspace: "test_keyspace",
						Table:    "test_table",
						Name:     "abcd",
						Index:    0,
						Type:     datatype.Timestamp,
					},
				},
			},
			want: message.Row{
				[]byte{0, 0, 0, 0, 0, 0, 0, 12},
			},
			wantErr: false,
		},
		{
			name: "Success in set data types",
			fields: fields{
				Logger:              zap.NewExample(),
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			args: args{
				rowMap: map[string]interface{}{
					"column11": []Maptype{
						{Key: "tag1", Value: ""},
						{Key: "tag2", Value: ""},
					},
				},
				query: QueryMetadata{
					TableName:           "test_table",
					Query:               "SELECT column11 FROM test_table;",
					KeyspaceName:        "test_keyspace",
					IsStar:              false,
					DefaultColumnFamily: "cf1",
					SelectedColumns: []schemaMapping.SelectedColumns{
						{
							Name: "column11",
						},
					},
				},
				cmd: []*message.ColumnMetadata{
					{
						Keyspace: "test_keyspace",
						Table:    "test_table",
						Name:     "column11",
						Index:    0,
						Type:     datatype.NewSetType(datatype.Varchar),
					},
				},
			},
			want: message.Row{
				[]byte{0, 2, 0, 0, 0, 4, 116, 97, 103, 49, 0, 0, 0, 4, 116, 97, 103, 50},
			},
			wantErr: false,
		},
		{
			name: "Failure case in set data types",
			fields: fields{
				Logger: zap.NewExample(),
				SchemaMappingConfig: &schemaMapping.SchemaMappingConfig{
					Logger: nil,
					Tables: map[string]map[string]*schemaMapping.TableConfig{
						"test_keyspace": {"test_table": &schemaMapping.TableConfig{
							Keyspace: "test_keyspace",
							Name:     "user_info",
							Columns: map[string]*types.Column{
								"column1": {
									Name:         "column1",
									CQLType:      datatype.Varchar,
									IsPrimaryKey: true,
									PkPrecedence: 1,
								},
								"column7": {
									Name:         "column7",
									CQLType:      datatype.NewSetType(datatype.NewCustomType("foo")),
									IsPrimaryKey: false,
									PkPrecedence: 1,
								},
							},
							PrimaryKeys: []*types.Column{
								{
									Name:         "column1",
									CQLType:      datatype.Varchar,
									IsPrimaryKey: true,
									PkPrecedence: 1,
								},
							},
						},
						},
					},
					SystemColumnFamily: "cf1",
				},
			},
			args: args{
				rowMap: map[string]interface{}{
					"column7": []Maptype{
						{Key: "tag1", Value: ""},
						{Key: "tag2", Value: ""},
					},
				},
				query: QueryMetadata{
					TableName:           "test_table",
					Query:               "SELECT column7 FROM test_table;",
					KeyspaceName:        "test_keyspace",
					IsStar:              false,
					DefaultColumnFamily: "cf1",
					SelectedColumns: []schemaMapping.SelectedColumns{
						{
							Name: "column7",
						},
					},
				},
				cmd: []*message.ColumnMetadata{
					{
						Keyspace: "test_keyspace",
						Table:    "test_table",
						Name:     "column7",
						Index:    0,
						Type:     datatype.NewSetType(datatype.NewCustomType("foo")),
					},
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Success case in list data types",
			fields: fields{
				Logger:              zap.NewExample(),
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			args: args{
				rowMap: map[string]interface{}{
					"column4": []Maptype{
						{Key: "key1", Value: []byte("tage1")},
						{Key: "key2", Value: []byte("tage2")},
					},
				},
				query: QueryMetadata{
					TableName:           "test_table",
					Query:               "SELECT column4 FROM test_table;",
					KeyspaceName:        "test_keyspace",
					IsStar:              false,
					DefaultColumnFamily: "cf1",
					SelectedColumns: []schemaMapping.SelectedColumns{
						{
							Name: "column4",
						},
					},
				},
				cmd: []*message.ColumnMetadata{
					{
						Keyspace: "test_keyspace",
						Table:    "test_table",
						Name:     "column4",
						Index:    0,
						Type:     datatype.NewListType(datatype.Varchar),
					},
				},
			},
			want: message.Row{
				[]byte{0, 2, 0, 0, 0, 5, 116, 97, 103, 101, 49, 0, 0, 0, 5, 116, 97, 103, 101, 50},
			},
			wantErr: false,
		},
		{
			name: "Failure case in list data types",
			fields: fields{
				Logger:              zap.NewExample(),
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			args: args{
				rowMap: map[string]interface{}{
					"column4": []Maptype{
						{Key: "tag1", Value: ""},
						{Key: "tag2", Value: ""},
					},
				},
				query: QueryMetadata{
					TableName:           "test_table",
					Query:               "SELECT column4 FROM test_table;",
					KeyspaceName:        "test_keyspace",
					IsStar:              false,
					DefaultColumnFamily: "cf1",
					SelectedColumns: []schemaMapping.SelectedColumns{
						{
							Name: "column4",
						},
					},
				},
				cmd: []*message.ColumnMetadata{
					{
						Keyspace: "test_keyspace",
						Table:    "test_table",
						Name:     "column4",
						Index:    0,
						Type:     datatype.NewListType(datatype.Varchar),
					},
				},
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			th := &TypeHandler{
				Logger:              tt.fields.Logger,
				SchemaMappingConfig: tt.fields.SchemaMappingConfig,
				ColumnMetadataCache: tt.fields.ColumnMetadataCache,
			}
			got, err := th.BuildResponseRow(tt.args.rowMap, tt.args.query, tt.args.cmd)
			if (err != nil) != tt.wantErr {
				t.Errorf("TypeHandler.BuildResponseRow() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TypeHandler.BuildResponseRow() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetQueryColumn(t *testing.T) {
	query := QueryMetadata{
		SelectedColumns: []schemaMapping.SelectedColumns{
			{Name: "column1", Alias: "alias1", IsWriteTimeColumn: false},
			{Name: "column2", Alias: "alias2", IsWriteTimeColumn: true},
			{Name: "column3", Alias: "alias3", IsWriteTimeColumn: false},
		},
	}

	tests := []struct {
		name       string
		query      QueryMetadata
		index      int
		key        string
		expected   schemaMapping.SelectedColumns
		expectFail bool
	}{
		{
			name:     "Match by Name at index",
			query:    query,
			index:    0,
			key:      "column1",
			expected: query.SelectedColumns[0],
		},
		{
			name:     "Match by Alias at index (Write Time Column)",
			query:    query,
			index:    1,
			key:      "alias2",
			expected: query.SelectedColumns[1],
		},
		{
			name:     "Match by Name in iteration",
			query:    query,
			index:    2,
			key:      "column2",
			expected: query.SelectedColumns[1],
		},
		{
			name:     "Not Match by Name in iteration",
			query:    query,
			index:    2,
			key:      "random-name",
			expected: schemaMapping.SelectedColumns{},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := GetQueryColumn(test.query, test.index, test.key)
			if !reflect.DeepEqual(result, test.expected) {
				if !test.expectFail {
					t.Errorf("Expected %+v, but got %+v", test.expected, result)
				}
			} else if test.expectFail {
				t.Errorf("Expected failure, but test passed for %+v", test.key)
			}
		})
	}
}
func TestTypeHandler_HandleListType(t *testing.T) {
	type fields struct {
		Logger              *zap.Logger
		SchemaMappingConfig *schemaMapping.SchemaMappingConfig
		ColumnMetadataCache map[string]map[string]message.ColumnMetadata
	}
	type args struct {
		listData  []interface{}
		mr        *message.Row
		listType  datatype.ListType
		protocalV primitive.ProtocolVersion
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "Handle boolean list",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				listData:  []interface{}{[]byte{0, 0, 0, 0, 0, 0, 0, 1}, []byte{0, 0, 0, 0, 0, 0, 0, 0}, []byte{0, 0, 0, 0, 0, 0, 0, 1}},
				mr:        &message.Row{},
				listType:  datatype.NewListType(datatype.Boolean),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: false,
		},
		{
			name: "Handle int list",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				listData:  []interface{}{[]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x0b}, []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x15}, []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x33}},
				mr:        &message.Row{},
				listType:  datatype.NewListType(datatype.Int),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: false,
		},
		{
			name: "Handle bigint list",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				listData:  []interface{}{[]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x0b}, []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x15}, []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x33}},
				mr:        &message.Row{},
				listType:  datatype.NewListType(datatype.Bigint),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: false,
		},
		{
			name: "Handle float list",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				listData:  []interface{}{[]byte{0x3f, 0x80, 0x00, 0x00}, []byte{0x40, 0x00, 0x00, 0x00}, []byte{0x40, 0x40, 0x00, 0x00}},
				mr:        &message.Row{},
				listType:  datatype.NewListType(datatype.Float),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: false,
		},
		{
			name: "Handle double list",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				listData:  []interface{}{[]byte{0x3f, 0xf0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, []byte{0x40, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, []byte{0x40, 0x09, 0x21, 0xfb, 0x54, 0x44, 0x2d, 0x18}},
				mr:        &message.Row{},
				listType:  datatype.NewListType(datatype.Double),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: false,
		},
		{
			name: "Handle string list",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				listData:  []interface{}{[]byte("a"), []byte("b"), []byte("c")},
				mr:        &message.Row{},
				listType:  datatype.NewListType(datatype.Varchar),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: false,
		},
		{
			name: "Handle timestamp list",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				listData:  []interface{}{[]byte{0x00, 0x00, 0x01, 0x63, 0x45, 0x67, 0x89, 0x0a}, []byte{0x00, 0x00, 0x01, 0x63, 0x45, 0x67, 0x89, 0x0b}, []byte{0x00, 0x00, 0x01, 0x63, 0x45, 0x67, 0x89, 0x0c}},
				mr:        &message.Row{},
				listType:  datatype.NewListType(datatype.Timestamp),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: false,
		},
		{
			name: "Unsupported element type",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				listData:  []interface{}{[]byte("unsupported")},
				mr:        &message.Row{},
				listType:  datatype.NewListType(datatype.NewCustomType("foo")),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: true,
		},
		{
			name: "Invalid type assertion",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				listData:  []interface{}{"invalid"},
				mr:        &message.Row{},
				listType:  datatype.NewListType(datatype.Int),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: true,
		},
		{
			name: "Error decoding boolean",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				listData:  []interface{}{[]byte{0x00, 0x00, 0x00}},
				mr:        &message.Row{},
				listType:  datatype.NewListType(datatype.Boolean),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: true,
		},
		{
			name: "Error decoding int",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				listData:  []interface{}{[]byte{0x00, 0x00, 0x00}},
				mr:        &message.Row{},
				listType:  datatype.NewListType(datatype.Int),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: true,
		},
		{
			name: "Error decoding bigint",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				listData:  []interface{}{[]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}},
				mr:        &message.Row{},
				listType:  datatype.NewListType(datatype.Bigint),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: true,
		},
		{
			name: "Error decoding float",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				listData:  []interface{}{[]byte{0x3f, 0x80, 0x00}},
				mr:        &message.Row{},
				listType:  datatype.NewListType(datatype.Float),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: true,
		},
		{
			name: "Error decoding double",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				listData:  []interface{}{[]byte{0x3f, 0xf0, 0x00, 0x00, 0x00, 0x00, 0x00}},
				mr:        &message.Row{},
				listType:  datatype.NewListType(datatype.Double),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: true,
		},
		{
			name: "Error decoding timestamp",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				listData:  []interface{}{[]byte{0x00, 0x00, 0x01, 0x63, 0x45, 0x67, 0x89}},
				mr:        &message.Row{},
				listType:  datatype.NewListType(datatype.Timestamp),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			th := &TypeHandler{
				Logger:              tt.fields.Logger,
				SchemaMappingConfig: tt.fields.SchemaMappingConfig,
				ColumnMetadataCache: tt.fields.ColumnMetadataCache,
			}
			if err := th.HandleListType(tt.args.listData, tt.args.mr, tt.args.listType, tt.args.protocalV); (err != nil) != tt.wantErr {
				t.Errorf("TypeHandler.HandleListType() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBuildResponseForSystemQueries(t *testing.T) {
	protocolVersion := primitive.ProtocolVersion4

	tests := []struct {
		name         string
		rows         [][]interface{}
		want         []message.Row
		wantErr      bool
		expectedRows int
	}{
		{
			name: "Valid Keyspace Metadata Encoding",
			rows: [][]any{
				{"keyspace1", true, map[string]string{"class": "SimpleStrategy", "replication_factor": "1"}},
			},
			wantErr:      false,
			expectedRows: 3,
		},
		{
			name: "Valid Table Metadata Encoding",
			rows: [][]any{
				{"keyspace1", "table1", "99p", 0.01, map[string]string{"keys": "ALL", "rows_per_partition": "NONE"}, []string{"compound"}},
			},
			wantErr:      false,
			expectedRows: 6,
		},
		{
			name: "Valid Column Metadata Encoding",
			rows: [][]any{
				{"keyspace1", "table1", "column1", "none", "regular", 0, "text"},
			},
			wantErr:      false,
			expectedRows: 7,
		},
		{
			name: "Failure Case - Invalid Data Type",
			rows: [][]any{
				{"keyspace1", make(chan int)}, // Passing an unsupported type to cause failure
			},
			wantErr:      true,
			expectedRows: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildResponseForSystemQueries(tt.rows, protocolVersion)

			if (err != nil) != tt.wantErr {
				t.Errorf("BuildResponseForSystemQueries() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				// Use cmp.Equal with custom comparer
				totalReturnedRows := len(got[0])
				if tt.expectedRows != totalReturnedRows {
					t.Errorf("Mismatch in encoded system query metadata response:\n%s", cmp.Diff(tt.want, got, customComparer))
				}
			}
		})
	}
}

// Custom comparer to ignore ordering of map keys
var customComparer = cmp.FilterValues(func(x, y interface{}) bool {
	_, xOk := x.(map[string]string)
	_, yOk := y.(map[string]string)
	return xOk && yOk
}, cmp.Comparer(func(x, y map[string]string) bool {
	if len(x) != len(y) {
		return false
	}
	for k, v := range x {
		if y[k] != v {
			return false
		}
	}
	return true
}))
