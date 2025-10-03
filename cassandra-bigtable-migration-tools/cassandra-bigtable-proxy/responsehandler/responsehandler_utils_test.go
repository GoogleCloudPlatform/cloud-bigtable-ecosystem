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
	"math/big"
	"reflect"
	"testing"

	schemaMapping "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/schema-mapping"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"go.uber.org/zap"
)

func TestTypeHandler_HandleTimestampMap(t *testing.T) {
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
			name: "Success For Int",
			fields: fields{
				Logger:              zap.NewExample(),
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			args: args{
				mapData: map[string]interface{}{
					"1234567890": []byte{0, 0, 0, 0, 0, 0, 4, 210},
				},
				mr: &message.Row{
					[]byte{0, 0, 0, 0, 0, 0, 4, 210},
				},
				mapType:   datatype.NewMapType(datatype.Timestamp, datatype.Int),
				protocalV: primitive.ProtocolVersion4,
			},
			wantErr: false,
		},
		{
			name: "Failed For float",
			fields: fields{
				Logger:              zap.NewExample(),
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			args: args{
				mapData: map[string]interface{}{
					"123.121": []byte(big.NewFloat(1.11).Text('f', -1)),
				},
				mr: &message.Row{
					[]byte(big.NewFloat(1.11).Text('f', -1)),
				},
				mapType:   datatype.NewMapType(datatype.Timestamp, datatype.Float),
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
			err := th.HandleTimestampMap(tt.args.mapData, tt.args.mr, tt.args.mapType, tt.args.protocalV)
			if (err != nil) != tt.wantErr {
				t.Errorf("TypeHandler.HandleTimestampMap() error = %v, wantErr %v", err, tt.wantErr)
			}
			if len(*tt.args.mr) == 0 {
				t.Errorf("TypeHandler.HandleTimestampMap() error = %v, wantErr %v", err, tt.wantErr)
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
		elementType datatype.DataType
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
				byteArray:   []byte{0, 0, 0, 0, 0, 0, 0, 1}, // Represents true in bigint
				elementType: datatype.Boolean,
				protocalV:   primitive.ProtocolVersion4, // Example protocol version
			},
			want:    true,
			wantErr: false,
		},
		{
			name:   "Decode int value",
			fields: fields{},
			args: args{
				byteArray:   []byte{0, 0, 0, 0, 0, 0, 0, 42}, // Represents int 42
				elementType: datatype.Int,
				protocalV:   primitive.ProtocolVersion4,
			},
			want:    int32(42),
			wantErr: false,
		},
		{
			name:   "Decode bigint value",
			fields: fields{},
			args: args{
				byteArray:   []byte{0, 0, 0, 0, 0, 0, 0, 100}, // Represents bigint 100
				elementType: datatype.Bigint,
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
				elementType: datatype.Varchar,
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
				elementType: datatype.NewCustomType("foo"),
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
				elementType: datatype.Int,
				protocalV:   primitive.ProtocolVersion3,
			},
			want:    nil,
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

func TestHandlePrimitiveEncoding(t *testing.T) {
	type args struct {
		cqlType         datatype.DataType
		value           interface{}
		protocalVersion primitive.ProtocolVersion
		encode          bool
	}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		{
			name: "Boolean true value as string input",
			args: args{
				cqlType:         datatype.Boolean,
				value:           "1",
				protocalVersion: primitive.ProtocolVersion(4),
				encode:          true,
			},
			want:    []byte{1},
			wantErr: false,
		},
		{
			name: "Boolean invalid value as string input",
			args: args{
				cqlType:         datatype.Boolean,
				value:           "true",
				protocalVersion: primitive.ProtocolVersion(4),
				encode:          true,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Boolean invalid value other than string and int",
			args: args{
				cqlType:         datatype.Boolean,
				value:           true,
				protocalVersion: primitive.ProtocolVersion(4),
				encode:          true,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Boolean false value as string input",
			args: args{
				cqlType:         datatype.Boolean,
				value:           "0",
				protocalVersion: primitive.ProtocolVersion(4),
				encode:          true,
			},
			want:    []byte{0},
			wantErr: false,
		},
		{
			name: "Boolean true value",
			args: args{
				cqlType:         datatype.Boolean,
				value:           []byte{0, 0, 0, 0, 0, 0, 0, 1},
				protocalVersion: primitive.ProtocolVersion(4),
				encode:          true,
			},
			want:    []byte{1},
			wantErr: false,
		},
		{
			name: "Boolean false value",
			args: args{
				cqlType:         datatype.Boolean,
				value:           []byte{0, 0, 0, 0, 0, 0, 0, 0},
				protocalVersion: primitive.ProtocolVersion(4),
				encode:          true,
			},
			want:    []byte{0},
			wantErr: false,
		},
		{
			name: "Nil value",
			args: args{
				cqlType:         datatype.Boolean,
				value:           nil,
				protocalVersion: primitive.ProtocolVersion(4),
				encode:          true,
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "Non-boolean type",
			args: args{
				cqlType:         datatype.Varchar,
				value:           []byte("some text"),
				protocalVersion: primitive.ProtocolVersion(4),
				encode:          true,
			},
			want:    []byte("some text"),
			wantErr: false,
		},
		{
			name: "Invalid byte data for boolean",
			args: args{
				cqlType:         datatype.Boolean,
				value:           []byte{2},
				protocalVersion: primitive.ProtocolVersion(4),
				encode:          true,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Int value",
			args: args{
				cqlType:         datatype.Int,
				value:           []byte{0, 0, 0, 0, 0, 0, 0, 12},
				protocalVersion: primitive.ProtocolVersion(4),
				encode:          true,
			},
			want:    []byte{0, 0, 0, 12},
			wantErr: false,
		},
		{
			name: "Int value as string input",
			args: args{
				cqlType:         datatype.Int,
				value:           "12",
				protocalVersion: primitive.ProtocolVersion(4),
				encode:          true,
			},
			want:    []byte{0, 0, 0, 12},
			wantErr: false,
		},
		{
			name: "Bigint value as string input",
			args: args{
				cqlType:         datatype.Bigint,
				value:           "12",
				protocalVersion: primitive.ProtocolVersion(4),
				encode:          true,
			},
			want:    []byte{0, 0, 0, 0, 0, 0, 0, 12},
			wantErr: false,
		},
		{
			name: "Counter value as string input",
			args: args{
				cqlType:         datatype.Counter,
				value:           "24",
				protocalVersion: primitive.ProtocolVersion(4),
				encode:          true,
			},
			want:    []byte{0, 0, 0, 0, 0, 0, 0, 24},
			wantErr: false,
		},
		{
			name: "Float value as string input",
			args: args{
				cqlType:         datatype.Float,
				value:           "12",
				protocalVersion: primitive.ProtocolVersion(4),
				encode:          true,
			},
			want:    []byte{65, 64, 0, 0},
			wantErr: false,
		},
		{
			name: "Invalid Float value as string input",
			args: args{
				cqlType:         datatype.Float,
				value:           true,
				protocalVersion: primitive.ProtocolVersion(4),
				encode:          true,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Invalid value for Bigint value",
			args: args{
				cqlType:         datatype.Bigint,
				value:           true,
				protocalVersion: primitive.ProtocolVersion(4),
				encode:          true,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Invalid value for Counter value",
			args: args{
				cqlType:         datatype.Counter,
				value:           true,
				protocalVersion: primitive.ProtocolVersion(4),
				encode:          true,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Invalid value for string value",
			args: args{
				cqlType:         datatype.Varchar,
				value:           true,
				protocalVersion: primitive.ProtocolVersion(4),
				encode:          true,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Valid value for string value",
			args: args{
				cqlType:         datatype.Varchar,
				value:           "some text",
				protocalVersion: primitive.ProtocolVersion(4),
				encode:          true,
			},
			want:    []byte("some text"),
			wantErr: false,
		},
		{
			name: "Invalid int value as non-string and non-byte value",
			args: args{
				cqlType:         datatype.Int,
				value:           true,
				protocalVersion: primitive.ProtocolVersion(4),
				encode:          true,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Int CQLType with nil value",
			args: args{
				cqlType:         datatype.Int,
				value:           nil,
				protocalVersion: primitive.ProtocolVersion(4),
				encode:          true,
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "Invalid type or unsupported type",
			args: args{
				cqlType:         datatype.Decimal,
				value:           "1.1",
				protocalVersion: primitive.ProtocolVersion(4),
				encode:          true,
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := HandlePrimitiveEncoding(tt.args.cqlType, tt.args.value, tt.args.protocalVersion, tt.args.encode)
			if (err != nil) != tt.wantErr {
				t.Errorf("HandlePrimitiveEncoding() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("HandlePrimitiveEncoding() = %v, want %v", got, tt.want)
			}
		})
	}
}
