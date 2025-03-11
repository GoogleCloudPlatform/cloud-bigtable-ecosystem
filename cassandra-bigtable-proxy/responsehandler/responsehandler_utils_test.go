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
	"math/big"
	"reflect"
	"testing"

	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	fakedata "github.com/ollionorg/cassandra-to-bigtable-proxy/fakedata"
	rh "github.com/ollionorg/cassandra-to-bigtable-proxy/responsehandler"
	schemaMapping "github.com/ollionorg/cassandra-to-bigtable-proxy/schema-mapping"
	"go.uber.org/zap"
)

func TestIsFirstCharDollar(t *testing.T) {
	testCases := []struct {
		input  string
		output bool
	}{
		{"$hello", true},
		{"hello$", false},
		{"$123", true},
		{"", false},
		{" ", false},
		{"\t$", false}, // Test non-printable characters
		{"\n$", false},
		{"\r$", false},
		{"\r\n$", false},
		{"$ \t", true}, // Test whitespace after $
		{"$ \n", true},
		{"$ \r", true},
		{"$ \r\n", true},
	}

	for _, tc := range testCases {
		result := rh.IsFirstCharDollar(tc.input)
		if result != tc.output {
			t.Errorf("IsFirstCharDollar(%q) = %v, want %v", tc.input, result, tc.output)
		}
	}
}

func TestGetMapField(t *testing.T) {
	type args struct {
		queryMetadata rh.QueryMetadata
		column        string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Success with map key",
			args: args{
				queryMetadata: rh.QueryMetadata{
					TableName:           "user_info",
					Query:               "SELECT name FROM user_info;",
					KeyspaceName:        "xobni_derived",
					IsStar:              false,
					DefaultColumnFamily: "cf1",
					SelectedColumns: []schemaMapping.SelectedColumns{
						{
							Name:   "map_with_key",
							MapKey: "name",
						},
					},
				},
				column: "map_with_key",
			},
			want: "name",
		},
		{
			name: "Empty with map key",
			args: args{
				queryMetadata: rh.QueryMetadata{
					TableName:           "user_info",
					Query:               "SELECT name FROM user_info;",
					KeyspaceName:        "xobni_derived",
					IsStar:              false,
					DefaultColumnFamily: "cf1",
					SelectedColumns: []schemaMapping.SelectedColumns{
						{
							Name:   "map_with_key",
							MapKey: "name",
						},
					},
				},
				column: "map_without_key",
			},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rh.GetMapField(tt.args.queryMetadata, tt.args.column); got != tt.want {
				t.Errorf("GetMapField() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTypeHandler_HandleTimestampMap(t *testing.T) {
	type fields struct {
		Logger              *zap.Logger
		SchemaMappingConfig *schemaMapping.SchemaMappingConfig
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
			name: "Success For Int",
			fields: fields{
				Logger:              zap.NewExample(),
				SchemaMappingConfig: fakedata.GetSchemaMappingConfig(),
			},
			args: args{
				mapData: map[string]interface{}{
					"1234567890": big.NewInt(1234567890).Bytes(),
				},
				mr: &message.Row{
					big.NewInt(1234567890).Bytes(),
				},
				elementType: "int",
				protocalV:   primitive.ProtocolVersion4,
			},
			wantErr: false,
		},
		{
			name: "Failed For float",
			fields: fields{
				Logger:              zap.NewExample(),
				SchemaMappingConfig: fakedata.GetSchemaMappingConfig(),
			},
			args: args{
				mapData: map[string]interface{}{
					"123.121": []byte(big.NewFloat(1.11).Text('f', -1)),
				},
				mr: &message.Row{
					[]byte(big.NewFloat(1.11).Text('f', -1)),
				},
				elementType: "float",
				protocalV:   primitive.ProtocolVersion4,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			th := &rh.TypeHandler{
				Logger:              tt.fields.Logger,
				SchemaMappingConfig: tt.fields.SchemaMappingConfig,
				ColumnMetadataCache: tt.fields.ColumnMetadataCache,
			}
			err := th.HandleTimestampMap(tt.args.mapData, tt.args.mr, tt.args.elementType, tt.args.protocalV)
			if (err != nil) != tt.wantErr {
				t.Errorf("TypeHandler.HandleTimestampMap() error = %v, wantErr %v", err, tt.wantErr)
			}
			if len(*tt.args.mr) == 0 {
				t.Errorf("TypeHandler.HandleTimestampMap() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetMapType(t *testing.T) {
	typeHandler := &rh.TypeHandler{}

	// Test cases
	tests := []struct {
		name          string
		elementType   string
		expectedType  datatype.DataType
		expectError   bool
		expectedError string
	}{
		{
			name:         "Valid type: boolean",
			elementType:  "boolean",
			expectedType: datatype.NewMapType(datatype.Timestamp, datatype.Boolean),
			expectError:  false,
		},
		{
			name:         "Valid type: int",
			elementType:  "int",
			expectedType: datatype.NewMapType(datatype.Timestamp, datatype.Int),
			expectError:  false,
		},
		{
			name:         "Valid type: bigint",
			elementType:  "bigint",
			expectedType: datatype.NewMapType(datatype.Timestamp, datatype.Bigint),
			expectError:  false,
		},
		{
			name:         "Valid type: float",
			elementType:  "float",
			expectedType: datatype.NewMapType(datatype.Timestamp, datatype.Float),
			expectError:  false,
		},
		{
			name:         "Valid type: double",
			elementType:  "double",
			expectedType: datatype.NewMapType(datatype.Timestamp, datatype.Double),
			expectError:  false,
		},
		{
			name:         "Valid type: string",
			elementType:  "string",
			expectedType: datatype.NewMapType(datatype.Timestamp, datatype.Varchar),
			expectError:  false,
		},
		{
			name:         "Valid type: text",
			elementType:  "text",
			expectedType: datatype.NewMapType(datatype.Timestamp, datatype.Varchar),
			expectError:  false,
		},
		{
			name:         "Valid type: timestamp",
			elementType:  "timestamp",
			expectedType: datatype.NewMapType(datatype.Timestamp, datatype.Bigint),
			expectError:  false,
		},
		{
			name:          "Invalid type: unsupported",
			elementType:   "unsupported",
			expectedType:  nil,
			expectError:   true,
			expectedError: "unsupported MAP element type: unsupported",
		},
	}

	// Execute test cases
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := typeHandler.GetMapType(test.elementType)

			if test.expectError {
				if err == nil {
					t.Errorf("Expected an error but got none")
				} else if err.Error() != test.expectedError {
					t.Errorf("Expected error: %v, but got: %v", test.expectedError, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if !reflect.DeepEqual(result, test.expectedType) {
					t.Errorf("Expected type: %+v, but got: %+v", test.expectedType, result)
				}
			}
		})
	}
}

func TestTypeHandler_decodeValue(t *testing.T) {
	type fields struct {
		Logger              *zap.Logger
		SchemaMappingConfig *schemaMapping.SchemaMappingConfig
		ColumnMetadataCache map[string]map[string]message.ColumnMetadata
	}
	type args struct {
		byteArray   []byte
		elementType string
		protocalV   primitive.ProtocolVersion
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    interface{}
		wantErr bool
	}{
		{
			name:   "Decode boolean value",
			fields: fields{},
			args: args{
				byteArray:   []byte{1}, // Represents true in boolean
				elementType: "boolean",
				protocalV:   primitive.ProtocolVersion3, // Example protocol version
			},
			want:    true,
			wantErr: false,
		},
		{
			name:   "Decode int value",
			fields: fields{},
			args: args{
				byteArray:   []byte{0, 0, 0, 42}, // Represents int 42
				elementType: "int",
				protocalV:   primitive.ProtocolVersion3,
			},
			want:    int32(42),
			wantErr: false,
		},
		{
			name:   "Decode bigint value",
			fields: fields{},
			args: args{
				byteArray:   []byte{0, 0, 0, 0, 0, 0, 0, 100}, // Represents bigint 100
				elementType: "bigint",
				protocalV:   primitive.ProtocolVersion3,
			},
			want:    int64(100),
			wantErr: false,
		},
		{
			name:   "Decode string value",
			fields: fields{},
			args: args{
				byteArray:   []byte("hello"),
				elementType: "string",
				protocalV:   primitive.ProtocolVersion3,
			},
			want:    "hello",
			wantErr: false,
		},
		{
			name:   "Decode unsupported type",
			fields: fields{},
			args: args{
				byteArray:   []byte{},
				elementType: "unsupported",
				protocalV:   primitive.ProtocolVersion3,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name:   "Decode with decoding error",
			fields: fields{},
			args: args{
				byteArray:   nil, // Simulates invalid input
				elementType: "int",
				protocalV:   primitive.ProtocolVersion3,
			},
			want:    nil,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			th := &rh.TypeHandler{
				Logger:              tt.fields.Logger,
				SchemaMappingConfig: tt.fields.SchemaMappingConfig,
				ColumnMetadataCache: tt.fields.ColumnMetadataCache,
			}
			got, err := th.DecodeValue(tt.args.byteArray, tt.args.elementType, tt.args.protocalV)
			if (err != nil) != tt.wantErr {
				t.Errorf("TypeHandler.DecodeValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TypeHandler.DecodeValue() = %v, want %v", got, tt.want)
			}
		})
	}
}
