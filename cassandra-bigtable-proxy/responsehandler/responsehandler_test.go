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
package responsehandler_test

import (
	"reflect"
	"testing"

	btpb "cloud.google.com/go/bigtable/apiv2/bigtablepb"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/google/go-cmp/cmp"
	"github.com/ollionorg/cassandra-to-bigtable-proxy/fakedata"
	rh "github.com/ollionorg/cassandra-to-bigtable-proxy/responsehandler"
	"github.com/ollionorg/cassandra-to-bigtable-proxy/tableConfig"
	"go.uber.org/zap"
)

func TestTypeHandler_GetRows(t *testing.T) {
	type fields struct {
		Logger              *zap.Logger
		TableConfig         *tableConfig.TableConfig
		ColumnMetadataCache map[string]map[string]message.ColumnMetadata
	}
	type args struct {
		result *btpb.ExecuteQueryResponse_Results
		cf     []*btpb.ColumnMetadata
		query  rh.QueryMetadata
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    map[string]map[string]interface{}
		wantErr bool
	}{
		{
			name: "Test case 1: Successful row retrieval",
			fields: fields{
				Logger:              zap.NewNop(),
				TableConfig:         fakedata.GetTableConfig(),
				ColumnMetadataCache: map[string]map[string]message.ColumnMetadata{},
			},
			args: args{
				result: fakedata.ResponseHandler_Input_Result_Success,
				cf:     fakedata.ResponseHandler_Input_CF_Success,
				query: rh.QueryMetadata{
					TableName:           "user_info",
					Query:               "SELECT * FROM test_keyspace.user_info;",
					KeyspaceName:        "test_keyspace",
					IsStar:              true,
					DefaultColumnFamily: "cf1",
				},
			},
			want:    fakedata.ResponseHandler_Success,
			wantErr: false,
		},
		{
			name: "Test case 2: Empty result without error",
			fields: fields{
				Logger:              zap.NewNop(),
				TableConfig:         fakedata.GetTableConfig(),
				ColumnMetadataCache: map[string]map[string]message.ColumnMetadata{},
			},
			args: args{
				result: &btpb.ExecuteQueryResponse_Results{
					Results: &btpb.PartialResultSet{
						PartialRows: &btpb.PartialResultSet_ProtoRowsBatch{
							ProtoRowsBatch: &btpb.ProtoRowsBatch{
								BatchData: []byte(""),
							},
						},
					},
				},
				cf: fakedata.ResponseHandler_Input_CF_Success,
				query: rh.QueryMetadata{
					TableName:           "user_info",
					Query:               "SELECT * FROM test_keyspace.user_info;",
					KeyspaceName:        "test_keyspace",
					IsStar:              true,
					DefaultColumnFamily: "cf1",
				},
			},
			want:    map[string]map[string]interface{}{},
			wantErr: false,
		},
		{
			name: "Test case 3: selected select operation",
			fields: fields{
				Logger:              zap.NewNop(),
				TableConfig:         fakedata.GetTableConfig(),
				ColumnMetadataCache: map[string]map[string]message.ColumnMetadata{},
			},
			args: args{
				result: fakedata.ResponseHandler_Input_Result_Selected_Select,
				cf:     fakedata.ResponseHandler_Input_CF_Selected_Select,
				query: rh.QueryMetadata{
					TableName:           "user_info",
					Query:               "SELECT name FROM test_keyspace.user_info;",
					KeyspaceName:        "test_keyspace",
					IsStar:              false,
					DefaultColumnFamily: "cf1",
					SelectedColumns: []tableConfig.SelectedColumns{
						{
							Name: "name",
						},
					},
				},
			},
			want:    fakedata.ResponseHandler_Selected_Select_Success,
			wantErr: false,
		},
		{
			name: "Test case 4: selected select operation for map operation",
			fields: fields{
				Logger:              zap.NewNop(),
				TableConfig:         fakedata.GetTableConfig(),
				ColumnMetadataCache: map[string]map[string]message.ColumnMetadata{},
			},
			args: args{
				result: fakedata.ResponseHandler_Input_Result_Selected_Select_Map,
				cf:     fakedata.ResponseHandler_Input_CF_Selected_Select_Map,
				query: rh.QueryMetadata{
					TableName:           "user_info",
					Query:               "SELECT name FROM test_keyspace.user_info;",
					KeyspaceName:        "test_keyspace",
					IsStar:              false,
					DefaultColumnFamily: "cf1",
					SelectedColumns: []tableConfig.SelectedColumns{
						{
							Name: "name",
						},
					},
				},
			},
			want:    fakedata.ResponseHandler_Selected_Select_Success_Map,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			th := &rh.TypeHandler{
				Logger:              tt.fields.Logger,
				TableConfig:         tt.fields.TableConfig,
				ColumnMetadataCache: tt.fields.ColumnMetadataCache,
			}
			rows := make(map[string]map[string]interface{})
			count := 0
			got, err := th.GetRows(tt.args.result, tt.args.cf, tt.args.query, &count, rows)
			if (err != nil) != tt.wantErr {
				t.Errorf("TypeHandler.GetRows() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TypeHandler.GetRows() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractUniqueKeys(t *testing.T) {
	tests := []struct {
		name     string
		rowMap   map[string]map[string]interface{}
		expected map[string]struct{}
	}{
		{
			name: "Single nested map with unique keys",
			rowMap: map[string]map[string]interface{}{
				"row1": {"key1": 1, "key2": 2},
			},
			expected: map[string]struct{}{
				"key1": {},
				"key2": {},
			},
		},
		{
			name: "Multiple nested maps with overlapping keys",
			rowMap: map[string]map[string]interface{}{
				"row1": {"key1": 1, "key2": 2},
				"row2": {"key2": 3, "key3": 4},
			},
			expected: map[string]struct{}{
				"key1": {},
				"key2": {},
				"key3": {},
			},
		},
		{
			name: "Empty input map",
			rowMap: map[string]map[string]interface{}{
				"row1": {},
			},
			expected: map[string]struct{}{},
		},
		{
			name:     "Nil input map",
			rowMap:   nil,
			expected: map[string]struct{}{},
		},
		{
			name: "Nested maps with empty keys",
			rowMap: map[string]map[string]interface{}{
				"row1": {"": 1},
				"row2": {"key1": 2, "key2": 3},
			},
			expected: map[string]struct{}{
				"":     {},
				"key1": {},
				"key2": {},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rh.ExtractUniqueKeys(tt.rowMap)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("ExtractUniqueKeys() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestTypeHandler_HandleMapType(t *testing.T) {
	type fields struct {
		Logger              *zap.Logger
		TableConfig         *tableConfig.TableConfig
		ColumnMetadataCache map[string]map[string]message.ColumnMetadata
	}
	type args struct {
		mapData     map[string]interface{}
		mr          *message.Row
		elementType string
		protocalV   primitive.ProtocolVersion
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
				TableConfig:         nil,
				ColumnMetadataCache: nil,
			},
			args: args{
				mapData: map[string]interface{}{
					"key1": []byte{0x01},
					"key2": []byte{0x0012},
				},
				mr:          &message.Row{},
				elementType: "int",
				protocalV:   primitive.ProtocolVersion4,
			},
			wantErr: true,
		},
		{
			name: "Invalid bigint map decoded",
			fields: fields{
				Logger:              zap.NewNop(),
				TableConfig:         nil,
				ColumnMetadataCache: nil,
			},
			args: args{
				mapData: map[string]interface{}{
					"key1": []byte{0x01},
					"key2": []byte{0x0012},
				},
				mr:          &message.Row{},
				elementType: "bigint",
				protocalV:   primitive.ProtocolVersion4,
			},
			wantErr: true,
		},
		{
			name: "Invalid float map decoded",
			fields: fields{
				Logger:              zap.NewNop(),
				TableConfig:         nil,
				ColumnMetadataCache: nil,
			},
			args: args{
				mapData: map[string]interface{}{
					"key1": []byte{0x01},
					"key2": []byte{0x023},
				},
				mr:          &message.Row{},
				elementType: "float",
				protocalV:   primitive.ProtocolVersion4,
			},
			wantErr: true,
		},
		{
			name: "Invalid double map decoded",
			fields: fields{
				Logger:              zap.NewNop(),
				TableConfig:         nil,
				ColumnMetadataCache: nil,
			},
			args: args{
				mapData: map[string]interface{}{
					"key1": []byte{0x01},
					"key2": []byte{0x0012},
				},
				mr:          &message.Row{},
				elementType: "double",
				protocalV:   primitive.ProtocolVersion4,
			},
			wantErr: true,
		},
		{
			name: "Invalid timestamp map decoded",
			fields: fields{
				Logger:              zap.NewNop(),
				TableConfig:         nil,
				ColumnMetadataCache: nil,
			},
			args: args{
				mapData: map[string]interface{}{
					"key1": []byte{0x01},
					"key2": []byte{0x0012},
				},
				mr:          &message.Row{},
				elementType: "timestamp",
				protocalV:   primitive.ProtocolVersion4,
			},
			wantErr: true,
		},
		{
			name: "invalid integer map",
			fields: fields{
				Logger:              zap.NewNop(),
				TableConfig:         nil,
				ColumnMetadataCache: nil,
			},
			args: args{
				mapData: map[string]interface{}{
					"key1": "    ",
				},
				mr:          &message.Row{},
				elementType: "int",
				protocalV:   primitive.ProtocolVersion4,
			},
			wantErr: true,
		},
		{
			name: "invalid bigint map",
			fields: fields{
				Logger:              zap.NewNop(),
				TableConfig:         nil,
				ColumnMetadataCache: nil,
			},
			args: args{
				mapData: map[string]interface{}{
					"key1": "    ",
				},
				mr:          &message.Row{},
				elementType: "bigint",
				protocalV:   primitive.ProtocolVersion4,
			},
			wantErr: true,
		},
		{
			name: "invalid float map",
			fields: fields{
				Logger:              zap.NewNop(),
				TableConfig:         nil,
				ColumnMetadataCache: nil,
			},
			args: args{
				mapData: map[string]interface{}{
					"key1": "    ",
				},
				mr:          &message.Row{},
				elementType: "float",
				protocalV:   primitive.ProtocolVersion4,
			},
			wantErr: true,
		},
		{
			name: "invalid double map",
			fields: fields{
				Logger:              zap.NewNop(),
				TableConfig:         nil,
				ColumnMetadataCache: nil,
			},
			args: args{
				mapData: map[string]interface{}{
					"key1": "    ",
				},
				mr:          &message.Row{},
				elementType: "double",
				protocalV:   primitive.ProtocolVersion4,
			},
			wantErr: true,
		},
		{
			name: "invalid timestamp map",
			fields: fields{
				Logger:              zap.NewNop(),
				TableConfig:         nil,
				ColumnMetadataCache: nil,
			},
			args: args{
				mapData: map[string]interface{}{
					"key1": "    ",
				},
				mr:          &message.Row{},
				elementType: "timestamp",
				protocalV:   primitive.ProtocolVersion4,
			},
			wantErr: true,
		},
		{
			name: "invalid boolean map",
			fields: fields{
				Logger:              zap.NewNop(),
				TableConfig:         nil,
				ColumnMetadataCache: nil,
			},
			args: args{
				mapData: map[string]interface{}{
					"key1": "    ",
				},
				mr:          &message.Row{},
				elementType: "boolean",
				protocalV:   primitive.ProtocolVersion4,
			},
			wantErr: true,
		},
		{
			name: "Valid boolean map",
			fields: fields{
				Logger:              zap.NewNop(),
				TableConfig:         nil,
				ColumnMetadataCache: nil,
			},
			args: args{
				mapData: map[string]interface{}{
					"key1": []byte{0x01},
					"key2": []byte{0x00},
				},
				mr:          &message.Row{},
				elementType: "boolean",
				protocalV:   primitive.ProtocolVersion4,
			},
			wantErr: false,
		},
		{
			name: "Invalid boolean map - incorrect value type",
			fields: fields{
				Logger:              zap.NewNop(),
				TableConfig:         nil,
				ColumnMetadataCache: nil,
			},
			args: args{
				mapData: map[string]interface{}{
					"key1": "not a byte array",
				},
				mr:          &message.Row{},
				elementType: "boolean",
				protocalV:   primitive.ProtocolVersion4,
			},
			wantErr: true,
		},
		{
			name: "Valid string map",
			fields: fields{
				Logger:              zap.NewNop(),
				TableConfig:         nil,
				ColumnMetadataCache: nil,
			},
			args: args{
				mapData: map[string]interface{}{
					"key1": []byte("value1"),
					"key2": []byte("value2"),
				},
				mr:          &message.Row{},
				elementType: "string",
				protocalV:   primitive.ProtocolVersion4,
			},
			wantErr: false,
		},
		{
			name: "Valid integer map",
			fields: fields{
				Logger:              zap.NewNop(),
				TableConfig:         nil,
				ColumnMetadataCache: nil,
			},
			args: args{
				mapData: map[string]interface{}{
					"key1": []byte("1234"),
					"key2": []byte("4567"),
				},
				mr:          &message.Row{},
				elementType: "int",
				protocalV:   primitive.ProtocolVersion4,
			},
			wantErr: false,
		},
		{
			name: "Valid big int map",
			fields: fields{
				Logger:              zap.NewNop(),
				TableConfig:         nil,
				ColumnMetadataCache: nil,
			},
			args: args{
				mapData: map[string]interface{}{
					"key1": []byte("12345678"),
					"key2": []byte("45671234"),
				},
				mr:          &message.Row{},
				elementType: "bigint",
				protocalV:   primitive.ProtocolVersion4,
			},
			wantErr: false,
		},
		{
			name: "Valid timestamp map",
			fields: fields{
				Logger:              zap.NewNop(),
				TableConfig:         nil,
				ColumnMetadataCache: nil,
			},
			args: args{
				mapData: map[string]interface{}{
					"key1": []byte("17307156"),
				},
				mr:          &message.Row{},
				elementType: "timestamp",
				protocalV:   primitive.ProtocolVersion4,
			},
			wantErr: false,
		},
		{
			name: "Valid float map",
			fields: fields{
				Logger:              zap.NewNop(),
				TableConfig:         nil,
				ColumnMetadataCache: nil,
			},
			args: args{
				mapData: map[string]interface{}{
					"key1": []byte("1.71"),
				},
				mr:          &message.Row{},
				elementType: "float",
				protocalV:   primitive.ProtocolVersion4,
			},
			wantErr: false,
		},
		{
			name: "Valid double map",
			fields: fields{
				Logger:              zap.NewNop(),
				TableConfig:         nil,
				ColumnMetadataCache: nil,
			},
			args: args{
				mapData: map[string]interface{}{
					"key1": []byte("1.711212"),
				},
				mr:          &message.Row{},
				elementType: "double",
				protocalV:   primitive.ProtocolVersion4,
			},
			wantErr: false,
		},
		{
			name: "Unsupported element type",
			fields: fields{
				Logger:              zap.NewNop(),
				TableConfig:         nil,
				ColumnMetadataCache: nil,
			},
			args: args{
				mapData: map[string]interface{}{
					"key1": []byte("value1"),
				},
				mr:          &message.Row{},
				elementType: "unsupportedType",
				protocalV:   primitive.ProtocolVersion4,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			th := rh.TypeHandler{
				Logger:              tt.fields.Logger,
				TableConfig:         tt.fields.TableConfig,
				ColumnMetadataCache: tt.fields.ColumnMetadataCache,
			}
			if err := th.HandleMapType(tt.args.mapData, tt.args.mr, tt.args.elementType, tt.args.protocalV); (err != nil) != tt.wantErr {
				t.Errorf("TypeHandler.HandleMapType() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTypeHandler_HandleSetType(t *testing.T) {
	type fields struct {
		Logger              *zap.Logger
		TableConfig         *tableConfig.TableConfig
		ColumnMetadataCache map[string]map[string]message.ColumnMetadata
	}
	type args struct {
		arr         []interface{}
		mr          *message.Row
		elementType string
		protocalV   primitive.ProtocolVersion
		arrayType   string
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
				arr:         []interface{}{"a", "b", "c"},
				mr:          &message.Row{},
				elementType: "string",
				protocalV:   primitive.ProtocolVersion4,
				arrayType:   "list",
			},
			wantErr: false,
		},
		{
			name: "Handle int list",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				arr:         []interface{}{"11", "21", "51"},
				mr:          &message.Row{},
				elementType: "int",
				protocalV:   primitive.ProtocolVersion4,
				arrayType:   "list",
			},
			wantErr: false,
		},
		{
			name: "Handle int list Error",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				arr:         []interface{}{"     "},
				mr:          &message.Row{},
				elementType: "int",
				protocalV:   primitive.ProtocolVersion4,
				arrayType:   "list",
			},
			wantErr: true,
		},
		{
			name: "Handle float list Error",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				arr:         []interface{}{"     "},
				mr:          &message.Row{},
				elementType: "float",
				protocalV:   primitive.ProtocolVersion4,
				arrayType:   "list",
			},
			wantErr: true,
		},
		{
			name: "Handle double list Error",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				arr:         []interface{}{"     "},
				mr:          &message.Row{},
				elementType: "double",
				protocalV:   primitive.ProtocolVersion4,
				arrayType:   "list",
			},
			wantErr: true,
		},
		{
			name: "Handle timestamp list Error",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				arr:         []interface{}{"     "},
				mr:          &message.Row{},
				elementType: "timestamp",
				protocalV:   primitive.ProtocolVersion4,
				arrayType:   "list",
			},
			wantErr: true,
		},
		{
			name: "Handle int set",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				arr:         []interface{}{"11", "21", "51"},
				mr:          &message.Row{},
				elementType: "int",
				protocalV:   primitive.ProtocolVersion4,
				arrayType:   "set",
			},
			wantErr: false,
		},
		{
			name: "Handle bigint list",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				arr:         []interface{}{"1234567890", "2345678909", "3456789099"},
				mr:          &message.Row{},
				elementType: "bigint",
				protocalV:   primitive.ProtocolVersion4,
				arrayType:   "list",
			},
			wantErr: false,
		},
		{
			name: "Handle bigint list Error",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				arr:         []interface{}{"     "},
				mr:          &message.Row{},
				elementType: "bigint",
				protocalV:   primitive.ProtocolVersion4,
				arrayType:   "list",
			},
			wantErr: true,
		},
		{
			name: "Handle bigint set",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				arr:         []interface{}{"1234567890", "2345678909", "3456789099"},
				mr:          &message.Row{},
				elementType: "bigint",
				protocalV:   primitive.ProtocolVersion4,
				arrayType:   "set",
			},
			wantErr: false,
		},
		{
			name: "Handle float list",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				arr:         []interface{}{"12.123", "234.456343", "34.789"},
				mr:          &message.Row{},
				elementType: "float",
				protocalV:   primitive.ProtocolVersion4,
				arrayType:   "list",
			},
			wantErr: false,
		},
		{
			name: "Handle float set",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				arr:         []interface{}{"12.123", "234.456343", "34.789"},
				mr:          &message.Row{},
				elementType: "float",
				protocalV:   primitive.ProtocolVersion4,
				arrayType:   "set",
			},
			wantErr: false,
		},
		{
			name: "Handle double list",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				arr:         []interface{}{"12.1234567890", "234.4563433786723", "34.7897213512"},
				mr:          &message.Row{},
				elementType: "double",
				protocalV:   primitive.ProtocolVersion4,
				arrayType:   "list",
			},
			wantErr: false,
		},
		{
			name: "Handle double set",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				arr:         []interface{}{"12.1234567890", "234.4563433786723", "34.7897213512"},
				mr:          &message.Row{},
				elementType: "double",
				protocalV:   primitive.ProtocolVersion4,
				arrayType:   "set",
			},
			wantErr: false,
		},
		{
			name: "Handle timestamp list",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				arr:         []interface{}{"1730715681", "1699093281", "1667557281"},
				mr:          &message.Row{},
				elementType: "timestamp",
				protocalV:   primitive.ProtocolVersion4,
				arrayType:   "list",
			},
			wantErr: false,
		},
		{
			name: "Handle timestamp set",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				arr:         []interface{}{"1730715681", "1699093281", "1667557281"},
				mr:          &message.Row{},
				elementType: "timestamp",
				protocalV:   primitive.ProtocolVersion4,
				arrayType:   "set",
			},
			wantErr: false,
		},
		{
			name: "Handle string set",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				arr:         []interface{}{"a", "b", "c"},
				mr:          &message.Row{},
				elementType: "string",
				protocalV:   primitive.ProtocolVersion4,
				arrayType:   "set",
			},
			wantErr: false,
		},
		{
			name: "Handle boolean list",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				arr:         []interface{}{"true", "false", "true"},
				mr:          &message.Row{},
				elementType: "boolean",
				protocalV:   primitive.ProtocolVersion4,
				arrayType:   "list",
			},
			wantErr: false,
		},
		{
			name: "Handle boolean set",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				arr:         []interface{}{"true", "false", "true"},
				mr:          &message.Row{},
				elementType: "boolean",
				protocalV:   primitive.ProtocolVersion4,
				arrayType:   "set",
			},
			wantErr: false,
		},
		{
			name: "Unsuppoted datatype",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				arr:         []interface{}{"11", "21", "51"},
				mr:          &message.Row{},
				elementType: "xyz",
				protocalV:   primitive.ProtocolVersion4,
				arrayType:   "list",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			th := &rh.TypeHandler{
				Logger:              tt.fields.Logger,
				TableConfig:         tt.fields.TableConfig,
				ColumnMetadataCache: tt.fields.ColumnMetadataCache,
			}
			err := th.HandleSetType(tt.args.arr, tt.args.mr, tt.args.elementType, tt.args.protocalV)
			if (err != nil) != tt.wantErr {
				t.Errorf("TypeHandler.HandleSetType() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTypeHandler_BuildMetadata(t *testing.T) {
	type fields struct {
		Logger              *zap.Logger
		TableConfig         *tableConfig.TableConfig
		ColumnMetadataCache map[string]map[string]message.ColumnMetadata
	}
	type args struct {
		rowMap map[string]map[string]interface{}
		query  rh.QueryMetadata
	}
	tests := []struct {
		name          string
		fields        fields
		args          args
		wantCmd       []*message.ColumnMetadata
		wantMapKeyArr []string
		wantErr       bool
	}{
		{
			name: "Success",
			fields: fields{
				Logger:      zap.NewExample(),
				TableConfig: fakedata.GetTableConfig(),
			},
			args: args{
				rowMap: map[string]map[string]interface{}{
					"user1": {
						"name": "Bob",
					},
				},
				query: rh.QueryMetadata{
					TableName:           "user_info",
					Query:               "SELECT name FROM test_keyspace.user_info;",
					KeyspaceName:        "test_keyspace",
					IsStar:              false,
					DefaultColumnFamily: "cf1",
					SelectedColumns: []tableConfig.SelectedColumns{
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
			wantMapKeyArr: []string{""},
			wantErr:       false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			th := &rh.TypeHandler{
				Logger:              tt.fields.Logger,
				TableConfig:         tt.fields.TableConfig,
				ColumnMetadataCache: tt.fields.ColumnMetadataCache,
			}
			gotCmd, gotMapKeyArr, err := th.BuildMetadata(tt.args.rowMap, tt.args.query)
			if (err != nil) != tt.wantErr {
				t.Errorf("TypeHandler.BuildMetadata() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotCmd, tt.wantCmd) {
				t.Errorf("TypeHandler.BuildMetadata() gotCmd = %v, want %v", gotCmd, tt.wantCmd)
			}
			if !reflect.DeepEqual(gotMapKeyArr, tt.wantMapKeyArr) {
				t.Errorf("TypeHandler.BuildMetadata() gotMapKeyArr = %v, want %v", gotMapKeyArr, tt.wantMapKeyArr)
			}
		})
	}
}

func TestTypeHandler_BuildResponseRow(t *testing.T) {
	type fields struct {
		Logger              *zap.Logger
		TableConfig         *tableConfig.TableConfig
		ColumnMetadataCache map[string]map[string]message.ColumnMetadata
	}
	type args struct {
		rowMap      map[string]interface{}
		query       rh.QueryMetadata
		cmd         []*message.ColumnMetadata
		mapKeyArray []string
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
				Logger:      zap.NewExample(),
				TableConfig: fakedata.GetTableConfig(),
			},
			args: args{
				rowMap: map[string]interface{}{
					"name": []byte{0x01},
				},
				query: rh.QueryMetadata{
					TableName:           "user_info",
					Query:               "SELECT name FROM test_keyspace.test_keyspace.user_info;",
					KeyspaceName:        "test_keyspace",
					IsStar:              false,
					DefaultColumnFamily: "cf1",
					SelectedColumns: []tableConfig.SelectedColumns{
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
				mapKeyArray: []string{"name"},
			},
			want: message.Row{
				[]byte{0x01},
			},
			wantErr: false,
		},
		{
			name: "Success for map collection",
			fields: fields{
				Logger:      zap.NewExample(),
				TableConfig: fakedata.GetTableConfig(),
			},
			args: args{
				rowMap: map[string]interface{}{
					"column8": []rh.Maptype{
						{Key: "text", Value: true},
					},
				},
				query: rh.QueryMetadata{
					TableName:           "test_table",
					Query:               "SELECT column8 FROM test_table;",
					KeyspaceName:        "test_keyspace",
					IsStar:              false,
					DefaultColumnFamily: "cf1",
					SelectedColumns: []tableConfig.SelectedColumns{
						{
							Name: "column8",
						},
					},
				},
				cmd: []*message.ColumnMetadata{
					{
						Keyspace: "test_keyspace",
						Table:    "test_table",
						Name:     "column8",
						Index:    0,
						Type:     datatype.Varchar,
					},
				},
				mapKeyArray: []string{"column8"},
			},
			want: message.Row{
				[]byte{0x00, 0x00},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			th := &rh.TypeHandler{
				Logger:              tt.fields.Logger,
				TableConfig:         tt.fields.TableConfig,
				ColumnMetadataCache: tt.fields.ColumnMetadataCache,
			}
			got, err := th.BuildResponseRow(tt.args.rowMap, tt.args.query, tt.args.cmd, tt.args.mapKeyArray)
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
	query := rh.QueryMetadata{
		SelectedColumns: []tableConfig.SelectedColumns{
			{Name: "column1", Alias: "alias1", Is_WriteTime_Column: false},
			{Name: "column2", Alias: "alias2", Is_WriteTime_Column: true},
			{Name: "column3", Alias: "alias3", Is_WriteTime_Column: false},
		},
	}

	tests := []struct {
		name       string
		query      rh.QueryMetadata
		index      int
		key        string
		expected   tableConfig.SelectedColumns
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
			expected: tableConfig.SelectedColumns{},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := rh.GetQueryColumn(test.query, test.index, test.key)
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
		TableConfig         *tableConfig.TableConfig
		ColumnMetadataCache map[string]map[string]message.ColumnMetadata
	}
	type args struct {
		listData    []interface{}
		mr          *message.Row
		elementType string
		protocalV   primitive.ProtocolVersion
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
				listData:    []interface{}{[]byte{0x01}, []byte{0x00}, []byte{0x01}},
				mr:          &message.Row{},
				elementType: "boolean",
				protocalV:   primitive.ProtocolVersion4,
			},
			wantErr: false,
		},
		{
			name: "Handle int list",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				listData:    []interface{}{[]byte{0x00, 0x00, 0x00, 0x0b}, []byte{0x00, 0x00, 0x00, 0x15}, []byte{0x00, 0x00, 0x00, 0x33}},
				mr:          &message.Row{},
				elementType: "int",
				protocalV:   primitive.ProtocolVersion4,
			},
			wantErr: false,
		},
		{
			name: "Handle bigint list",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				listData:    []interface{}{[]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x0b}, []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x15}, []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x33}},
				mr:          &message.Row{},
				elementType: "bigint",
				protocalV:   primitive.ProtocolVersion4,
			},
			wantErr: false,
		},
		{
			name: "Handle float list",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				listData:    []interface{}{[]byte{0x3f, 0x80, 0x00, 0x00}, []byte{0x40, 0x00, 0x00, 0x00}, []byte{0x40, 0x40, 0x00, 0x00}},
				mr:          &message.Row{},
				elementType: "float",
				protocalV:   primitive.ProtocolVersion4,
			},
			wantErr: false,
		},
		{
			name: "Handle double list",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				listData:    []interface{}{[]byte{0x3f, 0xf0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, []byte{0x40, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, []byte{0x40, 0x09, 0x21, 0xfb, 0x54, 0x44, 0x2d, 0x18}},
				mr:          &message.Row{},
				elementType: "double",
				protocalV:   primitive.ProtocolVersion4,
			},
			wantErr: false,
		},
		{
			name: "Handle string list",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				listData:    []interface{}{[]byte("a"), []byte("b"), []byte("c")},
				mr:          &message.Row{},
				elementType: "string",
				protocalV:   primitive.ProtocolVersion4,
			},
			wantErr: false,
		},
		{
			name: "Handle timestamp list",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				listData:    []interface{}{[]byte{0x00, 0x00, 0x01, 0x63, 0x45, 0x67, 0x89, 0x0a}, []byte{0x00, 0x00, 0x01, 0x63, 0x45, 0x67, 0x89, 0x0b}, []byte{0x00, 0x00, 0x01, 0x63, 0x45, 0x67, 0x89, 0x0c}},
				mr:          &message.Row{},
				elementType: "timestamp",
				protocalV:   primitive.ProtocolVersion4,
			},
			wantErr: false,
		},
		{
			name: "Unsupported element type",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				listData:    []interface{}{[]byte("unsupported")},
				mr:          &message.Row{},
				elementType: "unsupportedType",
				protocalV:   primitive.ProtocolVersion4,
			},
			wantErr: true,
		},
		{
			name: "Invalid type assertion",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				listData:    []interface{}{"invalid"},
				mr:          &message.Row{},
				elementType: "int",
				protocalV:   primitive.ProtocolVersion4,
			},
			wantErr: true,
		},
		{
			name: "Error decoding boolean",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				listData:    []interface{}{[]byte{0x00, 0x00, 0x00}},
				mr:          &message.Row{},
				elementType: "boolean",
				protocalV:   primitive.ProtocolVersion4,
			},
			wantErr: true,
		},
		{
			name: "Error decoding int",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				listData:    []interface{}{[]byte{0x00, 0x00, 0x00}},
				mr:          &message.Row{},
				elementType: "int",
				protocalV:   primitive.ProtocolVersion4,
			},
			wantErr: true,
		},
		{
			name: "Error decoding bigint",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				listData:    []interface{}{[]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}},
				mr:          &message.Row{},
				elementType: "bigint",
				protocalV:   primitive.ProtocolVersion4,
			},
			wantErr: true,
		},
		{
			name: "Error decoding float",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				listData:    []interface{}{[]byte{0x3f, 0x80, 0x00}},
				mr:          &message.Row{},
				elementType: "float",
				protocalV:   primitive.ProtocolVersion4,
			},
			wantErr: true,
		},
		{
			name: "Error decoding double",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				listData:    []interface{}{[]byte{0x3f, 0xf0, 0x00, 0x00, 0x00, 0x00, 0x00}},
				mr:          &message.Row{},
				elementType: "double",
				protocalV:   primitive.ProtocolVersion4,
			},
			wantErr: true,
		},
		{
			name: "Error decoding timestamp",
			fields: fields{
				Logger: zap.NewExample(),
			},
			args: args{
				listData:    []interface{}{[]byte{0x00, 0x00, 0x01, 0x63, 0x45, 0x67, 0x89}},
				mr:          &message.Row{},
				elementType: "timestamp",
				protocalV:   primitive.ProtocolVersion4,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			th := &rh.TypeHandler{
				Logger:              tt.fields.Logger,
				TableConfig:         tt.fields.TableConfig,
				ColumnMetadataCache: tt.fields.ColumnMetadataCache,
			}
			if err := th.HandleListType(tt.args.listData, tt.args.mr, tt.args.elementType, tt.args.protocalV); (err != nil) != tt.wantErr {
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
			got, err := rh.BuildResponseForSystemQueries(tt.rows, protocolVersion)

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
