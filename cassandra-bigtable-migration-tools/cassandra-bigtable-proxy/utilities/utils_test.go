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
package utilities

import (
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/third_party/datastax/proxycore"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsCollectionDataType(t *testing.T) {
	testCases := []struct {
		input datatype.DataType
		want  bool
	}{
		{datatype.Varchar, false},
		{datatype.Blob, false},
		{datatype.Bigint, false},
		{datatype.Boolean, false},
		{datatype.Date, false},
		{datatype.NewMapType(datatype.Varchar, datatype.Boolean), true},
		{datatype.NewListType(datatype.Int), true},
		{datatype.NewSetType(datatype.Varchar), true},
	}

	for _, tt := range testCases {
		t.Run(tt.input.String(), func(t *testing.T) {
			got := IsCollection(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDecodeBytesToCassandraColumnType(t *testing.T) {
	tests := []struct {
		name            string
		input           []byte
		dataType        datatype.PrimitiveType
		protocolVersion primitive.ProtocolVersion
		expected        any
		expectError     bool
		errorMessage    string
	}{
		{
			name:            "Decode varchar",
			input:           []byte("test string"),
			dataType:        datatype.Varchar,
			protocolVersion: primitive.ProtocolVersion4,
			expected:        "test string",
			expectError:     false,
		},
		{
			name: "Decode double",
			input: func() []byte {
				b, _ := proxycore.EncodeType(datatype.Double, primitive.ProtocolVersion4, float64(123.45))
				return b
			}(),
			dataType:        datatype.Double,
			protocolVersion: primitive.ProtocolVersion4,
			expected:        float64(123.45),
			expectError:     false,
		},
		{
			name: "Decode float",
			input: func() []byte {
				b, _ := proxycore.EncodeType(datatype.Float, primitive.ProtocolVersion4, float32(123.45))
				return b
			}(),
			dataType:        datatype.Float,
			protocolVersion: primitive.ProtocolVersion4,
			expected:        float32(123.45),
			expectError:     false,
		},
		{
			name: "Decode bigint",
			input: func() []byte {
				b, _ := proxycore.EncodeType(datatype.Bigint, primitive.ProtocolVersion4, int64(12345))
				return b
			}(),
			dataType:        datatype.Bigint,
			protocolVersion: primitive.ProtocolVersion4,
			expected:        int64(12345),
			expectError:     false,
		},
		{
			name: "Decode min bigint",
			input: func() []byte {
				b, _ := proxycore.EncodeType(datatype.Bigint, primitive.ProtocolVersion4, int64(math.MinInt64))
				return b
			}(),
			dataType:        datatype.Bigint,
			protocolVersion: primitive.ProtocolVersion4,
			expected:        int64(math.MinInt64),
			expectError:     false,
		},
		{
			name: "Decode int",
			input: func() []byte {
				b, _ := proxycore.EncodeType(datatype.Int, primitive.ProtocolVersion4, int32(12345))
				return b
			}(),
			dataType:        datatype.Int,
			protocolVersion: primitive.ProtocolVersion4,
			expected:        int64(12345), // Note: int32 is converted to int64
			expectError:     false,
		},
		{
			name: "Decode min int",
			input: func() []byte {
				b, _ := proxycore.EncodeType(datatype.Int, primitive.ProtocolVersion4, int32(math.MinInt32))
				return b
			}(),
			dataType:        datatype.Int,
			protocolVersion: primitive.ProtocolVersion4,
			expected:        int64(math.MinInt32), // Note: int32 is converted to int64
			expectError:     false,
		},
		{
			name: "Decode boolean true",
			input: func() []byte {
				b, _ := proxycore.EncodeType(datatype.Boolean, primitive.ProtocolVersion4, true)
				return b
			}(),
			dataType:        datatype.Boolean,
			protocolVersion: primitive.ProtocolVersion4,
			expected:        true,
			expectError:     false,
		},
		{
			name: "Decode list of varchar",
			input: func() []byte {
				b, _ := proxycore.EncodeType(ListOfStr.DataType(), primitive.ProtocolVersion4, []string{"test1", "test2"})
				return b
			}(),
			dataType:        ListOfStr.DataType(),
			protocolVersion: primitive.ProtocolVersion4,
			expected:        []string{"test1", "test2"},
			expectError:     false,
		},
		{
			name: "Decode list of bigint",
			input: func() []byte {
				b, _ := proxycore.EncodeType(ListOfBigInt.DataType(), primitive.ProtocolVersion4, []int64{123, 456})
				return b
			}(),
			dataType:        ListOfBigInt.DataType(),
			protocolVersion: primitive.ProtocolVersion4,
			expected:        []int64{123, 456},
			expectError:     false,
		},
		{
			name: "Decode list of double",
			input: func() []byte {
				b, _ := proxycore.EncodeType(ListOfDouble.DataType(), primitive.ProtocolVersion4, []float64{123.45, 456.78})
				return b
			}(),
			dataType:        ListOfDouble.DataType(),
			protocolVersion: primitive.ProtocolVersion4,
			expected:        []float64{123.45, 456.78},
			expectError:     false,
		},
		{
			name:            "Invalid int data",
			input:           []byte("invalid int"),
			dataType:        datatype.Int,
			protocolVersion: primitive.ProtocolVersion4,
			expectError:     true,
			errorMessage:    "cannot decode CQL int as *interface {} with ProtocolVersion OSS 4: cannot read int32: expected 4 bytes but got: 11",
		},
		{
			name:            "Unsupported list element type",
			input:           []byte("test"),
			dataType:        ListOfBool.DataType(), // List of boolean is not supported
			protocolVersion: primitive.ProtocolVersion4,
			expectError:     true,
			errorMessage:    "unsupported list element type to decode",
		},
		{
			name:            "Unsupported type",
			input:           []byte("test"),
			dataType:        datatype.Duration, // Duration type is not supported
			protocolVersion: primitive.ProtocolVersion4,
			expectError:     true,
			errorMessage:    "unsupported Datatype to decode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := DecodeBytesToCassandraColumnType(tt.input, tt.dataType, tt.protocolVersion)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestDecodeNonPrimitive(t *testing.T) {
	tests := []struct {
		name         string
		input        []byte
		dataType     datatype.PrimitiveType
		expected     interface{}
		expectError  bool
		errorMessage string
	}{
		{
			name: "Decode list of varchar",
			input: func() []byte {
				b, _ := proxycore.EncodeType(ListOfStr.DataType(), primitive.ProtocolVersion4, []string{"test1", "test2"})
				return b
			}(),
			dataType:    ListOfStr.DataType(),
			expected:    []string{"test1", "test2"},
			expectError: false,
		},
		{
			name: "Decode list of bigint",
			input: func() []byte {
				b, _ := proxycore.EncodeType(ListOfBigInt.DataType(), primitive.ProtocolVersion4, []int64{123, 456})
				return b
			}(),
			dataType:    ListOfBigInt.DataType(),
			expected:    []int64{123, 456},
			expectError: false,
		},
		{
			name: "Decode list of double",
			input: func() []byte {
				b, _ := proxycore.EncodeType(ListOfDouble.DataType(), primitive.ProtocolVersion4, []float64{123.45, 456.78})
				return b
			}(),
			dataType:    ListOfDouble.DataType(),
			expected:    []float64{123.45, 456.78},
			expectError: false,
		},
		{
			name:         "Unsupported list element type",
			input:        []byte("test"),
			dataType:     ListOfBool.DataType(), // List of boolean is not supported
			expectError:  true,
			errorMessage: "unsupported list element type to decode",
		},
		{
			name:         "Non-list type",
			input:        []byte("test"),
			dataType:     datatype.Varchar,
			expectError:  true,
			errorMessage: "unsupported Datatype to decode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := decodeNonPrimitive(tt.dataType, tt.input)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMessage != "" {
					assert.Contains(t, err.Error(), tt.errorMessage)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func Test_defaultIfZero(t *testing.T) {
	type args struct {
		value        int
		defaultValue int
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		{
			name: "Return actual value",
			args: args{
				value:        1,
				defaultValue: 1,
			},
			want: 1,
		},
		{
			name: "Return default value",
			args: args{
				value:        0,
				defaultValue: 11,
			},
			want: 11,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := defaultIfZero(tt.args.value, tt.args.defaultValue); got != tt.want {
				t.Errorf("defaultIfZero() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_defaultIfEmpty(t *testing.T) {
	type args struct {
		value        string
		defaultValue string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Return actual value",
			args: args{
				value:        "abcd",
				defaultValue: "",
			},
			want: "abcd",
		},
		{
			name: "Return default value",
			args: args{
				value:        "",
				defaultValue: "abcd",
			},
			want: "abcd",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := defaultIfEmpty(tt.args.value, tt.args.defaultValue); got != tt.want {
				t.Errorf("defaultIfEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTypeConversion(t *testing.T) {
	protocalV := primitive.ProtocolVersion4
	tests := []struct {
		name            string
		input           interface{}
		expected        []byte
		wantErr         bool
		protocalVersion primitive.ProtocolVersion
	}{
		{
			name:            "String",
			input:           "example string",
			expected:        []byte{'e', 'x', 'a', 'm', 'p', 'l', 'e', ' ', 's', 't', 'r', 'i', 'n', 'g'},
			protocalVersion: protocalV,
		},
		{
			name:            "Int64",
			input:           int64(12345),
			expected:        []byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x30, 0x39},
			protocalVersion: protocalV,
		},
		{
			name:            "Boolean",
			input:           true,
			expected:        []byte{0x01},
			protocalVersion: protocalV,
		},
		{
			name:            "Float64",
			input:           123.45,
			expected:        []byte{0x40, 0x5E, 0xDC, 0xCC, 0xCC, 0xCC, 0xCC, 0xCD},
			protocalVersion: protocalV,
		},
		{
			name:            "Timestamp",
			input:           time.Date(2021, time.April, 10, 12, 0, 0, 0, time.UTC),
			expected:        []byte{0x00, 0x00, 0x01, 0x78, 0xBB, 0xA7, 0x32, 0x00},
			protocalVersion: protocalV,
		},
		{
			name:            "Byte",
			input:           []byte{0x01, 0x02, 0x03, 0x04},
			expected:        []byte{0x01, 0x02, 0x03, 0x04},
			protocalVersion: protocalV,
		},
		{
			name:            "Map",
			input:           datatype.NewMapType(datatype.Varchar, datatype.Varchar),
			expected:        []byte{'m', 'a', 'p', '<', 'v', 'a', 'r', 'c', 'h', 'a', 'r', ',', 'v', 'a', 'r', 'c', 'h', 'a', 'r', '>'},
			protocalVersion: protocalV,
		},
		{
			name:            "String Error Case",
			input:           struct{}{},
			wantErr:         true,
			protocalVersion: protocalV,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TypeConversion(tt.input, tt.protocalVersion)
			if (err != nil) != tt.wantErr {
				t.Errorf("TypeConversion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("TypeConversion(%v) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestEncodeInt(t *testing.T) {
	tests := []struct {
		name string
		args struct {
			value interface{}
			pv    primitive.ProtocolVersion
		}
		want    []byte
		wantErr bool
	}{
		{
			name: "Valid string input",
			args: struct {
				value interface{}
				pv    primitive.ProtocolVersion
			}{
				value: "12",
				pv:    primitive.ProtocolVersion4,
			},
			want:    []byte{0, 0, 0, 0, 0, 0, 0, 12}, // Replace with the bytes you expect
			wantErr: false,
		},
		{
			name: "String parsing error",
			args: struct {
				value interface{}
				pv    primitive.ProtocolVersion
			}{
				value: "abc",
				pv:    primitive.ProtocolVersion4,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Valid int32 input",
			args: struct {
				value interface{}
				pv    primitive.ProtocolVersion
			}{
				value: int32(12),
				pv:    primitive.ProtocolVersion4,
			},
			want:    []byte{0, 0, 0, 0, 0, 0, 0, 12}, // Replace with the bytes you expect
			wantErr: false,
		},
		{
			name: "Valid []byte input",
			args: struct {
				value interface{}
				pv    primitive.ProtocolVersion
			}{
				value: []byte{0, 0, 0, 12}, // Replace with actual bytes representing an int32 value
				pv:    primitive.ProtocolVersion4,
			},
			want:    []byte{0, 0, 0, 0, 0, 0, 0, 12}, // Replace with the bytes you expect
			wantErr: false,
		},
		{
			name: "Unsupported type",
			args: struct {
				value interface{}
				pv    primitive.ProtocolVersion
			}{
				value: 12.34, // Unsupported float64 type.
				pv:    primitive.ProtocolVersion4,
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EncodeInt(tt.args.value, tt.args.pv)
			if (err != nil) != tt.wantErr {
				t.Errorf("EncodeInt() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("EncodeInt() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEncodeBool(t *testing.T) {
	tests := []struct {
		name string
		args struct {
			value interface{}
			pv    primitive.ProtocolVersion
		}
		want    []byte
		wantErr bool
	}{
		{
			name: "Valid string 'true'",
			args: struct {
				value interface{}
				pv    primitive.ProtocolVersion
			}{
				value: "true",
				pv:    primitive.ProtocolVersion4,
			},
			want:    []byte{0, 0, 0, 0, 0, 0, 0, 1},
			wantErr: false,
		},
		{
			name: "Valid string 'false'",
			args: struct {
				value interface{}
				pv    primitive.ProtocolVersion
			}{
				value: "false",
				pv:    primitive.ProtocolVersion4,
			},
			want:    []byte{0, 0, 0, 0, 0, 0, 0, 0},
			wantErr: false,
		},
		{
			name: "String parsing error",
			args: struct {
				value interface{}
				pv    primitive.ProtocolVersion
			}{
				value: "notabool",
				pv:    primitive.ProtocolVersion4,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Valid bool true",
			args: struct {
				value interface{}
				pv    primitive.ProtocolVersion
			}{
				value: true,
				pv:    primitive.ProtocolVersion4,
			},
			want:    []byte{0, 0, 0, 0, 0, 0, 0, 1},
			wantErr: false,
		},
		{
			name: "Valid bool false",
			args: struct {
				value interface{}
				pv    primitive.ProtocolVersion
			}{
				value: false,
				pv:    primitive.ProtocolVersion4,
			},
			want:    []byte{0, 0, 0, 0, 0, 0, 0, 0},
			wantErr: false,
		},
		{
			name: "Valid []byte input for true",
			args: struct {
				value interface{}
				pv    primitive.ProtocolVersion
			}{
				value: []byte{1},
				pv:    primitive.ProtocolVersion4,
			},
			want:    []byte{0, 0, 0, 0, 0, 0, 0, 1},
			wantErr: false,
		},
		{
			name: "Valid []byte input for false",
			args: struct {
				value interface{}
				pv    primitive.ProtocolVersion
			}{
				value: []byte{0},
				pv:    primitive.ProtocolVersion4,
			},
			want:    []byte{0, 0, 0, 0, 0, 0, 0, 0},
			wantErr: false,
		},
		{
			name: "Unsupported type",
			args: struct {
				value interface{}
				pv    primitive.ProtocolVersion
			}{
				value: 123,
				pv:    primitive.ProtocolVersion4,
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EncodeBool(tt.args.value, tt.args.pv)
			if (err != nil) != tt.wantErr {
				t.Errorf("EncodeBool() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("EncodeBool() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDataConversionInInsertionIfRequired(t *testing.T) {
	tests := []struct {
		name string
		args struct {
			value        interface{}
			pv           primitive.ProtocolVersion
			cqlType      string
			responseType string
		}
		want    interface{}
		wantErr bool
	}{
		{
			name: "Boolean to string true",
			args: struct {
				value        interface{}
				pv           primitive.ProtocolVersion
				cqlType      string
				responseType string
			}{
				value:        "true",
				pv:           primitive.ProtocolVersion4,
				cqlType:      "boolean",
				responseType: "string",
			},
			want:    "1",
			wantErr: false,
		},
		{
			name: "Boolean to string false",
			args: struct {
				value        interface{}
				pv           primitive.ProtocolVersion
				cqlType      string
				responseType string
			}{
				value:        "false",
				pv:           primitive.ProtocolVersion4,
				cqlType:      "boolean",
				responseType: "string",
			},
			want:    "0",
			wantErr: false,
		},
		{
			name: "Invalid boolean string",
			args: struct {
				value        interface{}
				pv           primitive.ProtocolVersion
				cqlType      string
				responseType string
			}{
				// value is not a valid boolean string
				value:        "notabool",
				pv:           primitive.ProtocolVersion4,
				cqlType:      "boolean",
				responseType: "string",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Boolean to EncodeBool",
			args: struct {
				value        interface{}
				pv           primitive.ProtocolVersion
				cqlType      string
				responseType string
			}{
				value:        true,
				pv:           primitive.ProtocolVersion4,
				cqlType:      "boolean",
				responseType: "default",
			},
			want:    []byte{0, 0, 0, 0, 0, 0, 0, 1}, // Expecting boolean encoding, replace as needed
			wantErr: false,
		},
		{
			name: "Int to string",
			args: struct {
				value        interface{}
				pv           primitive.ProtocolVersion
				cqlType      string
				responseType string
			}{
				value:        "123",
				pv:           primitive.ProtocolVersion4,
				cqlType:      "int",
				responseType: "string",
			},
			want:    "123",
			wantErr: false,
		},
		{
			name: "Invalid int string",
			args: struct {
				value        interface{}
				pv           primitive.ProtocolVersion
				cqlType      string
				responseType string
			}{
				// value is not a valid int string
				value:        "notanint",
				pv:           primitive.ProtocolVersion4,
				cqlType:      "int",
				responseType: "string",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Int to EncodeInt",
			args: struct {
				value        interface{}
				pv           primitive.ProtocolVersion
				cqlType      string
				responseType string
			}{
				value:        int32(12),
				pv:           primitive.ProtocolVersion4,
				cqlType:      "int",
				responseType: "default",
			},
			want:    []byte{0, 0, 0, 0, 0, 0, 0, 12}, // Expecting int encoding, replace as needed
			wantErr: false,
		},
		{
			name: "Unsupported cqlType",
			args: struct {
				value        interface{}
				pv           primitive.ProtocolVersion
				cqlType      string
				responseType string
			}{
				value:        "anything",
				pv:           primitive.ProtocolVersion4,
				cqlType:      "unsupported",
				responseType: "default",
			},
			want:    "anything",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DataConversionInInsertionIfRequired(tt.args.value, tt.args.pv, tt.args.cqlType, tt.args.responseType)
			if (err != nil) != tt.wantErr {
				t.Errorf("DataConversionInInsertionIfRequired() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DataConversionInInsertionIfRequired() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetClauseByValue(t *testing.T) {
	type args struct {
		clause []types.Clause
		value  string
	}
	tests := []struct {
		name    string
		args    args
		want    types.Clause
		wantErr bool
	}{
		{
			name: "Found clause",
			args: args{
				clause: []types.Clause{{Value: "@test"}},
				value:  "test",
			},
			want:    types.Clause{Value: "@test"},
			wantErr: false,
		},
		{
			name: "Clause not found",
			args: args{
				clause: []types.Clause{{Value: "@test"}},
				value:  "notfound",
			},
			want:    types.Clause{},
			wantErr: true,
		},
		{
			name: "Empty clause slice",
			args: args{
				clause: []types.Clause{},
				value:  "test",
			},
			want:    types.Clause{},
			wantErr: true,
		},
		{
			name: "Multiple clauses, found",
			args: args{
				clause: []types.Clause{{Value: "@test1"}, {Value: "@test2"}},
				value:  "test2",
			},
			want:    types.Clause{Value: "@test2"},
			wantErr: false,
		},
		{
			name: "Multiple clauses, not found",
			args: args{
				clause: []types.Clause{{Value: "@test1"}, {Value: "@test2"}},
				value:  "test3",
			},
			want:    types.Clause{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetClauseByValue(tt.args.clause, tt.args.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetClauseByValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetClauseByValue() = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestGetClauseByColumn(t *testing.T) {
	type args struct {
		clause []types.Clause
		column string
	}
	tests := []struct {
		name    string
		args    args
		want    types.Clause
		wantErr bool
	}{
		{
			name: "Existing column",
			args: args{
				clause: []types.Clause{
					{Column: "column1", Value: "value1"},
					{Column: "column2", Value: "value2"},
				},
				column: "column1",
			},
			want:    types.Clause{Column: "column1", Value: "value1"},
			wantErr: false,
		},
		{
			name: "Non-existing column",
			args: args{
				clause: []types.Clause{
					{Column: "column1", Value: "value1"},
					{Column: "column2", Value: "value2"},
				},
				column: "column3",
			},
			want:    types.Clause{},
			wantErr: true,
		},
		{
			name: "Empty clause slice",
			args: args{
				clause: []types.Clause{},
				column: "column1",
			},
			want:    types.Clause{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetClauseByColumn(tt.args.clause, tt.args.column)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetClauseByColumn() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetClauseByColumn() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsSupportedPrimaryKeyType(t *testing.T) {
	testCases := []struct {
		name     string
		input    types.CqlDataType
		expected bool
	}{
		{"Supported CQLType - Int", ParseCqlTypeOrDie("int"), true},
		{"Supported CQLType - Bigint", ParseCqlTypeOrDie("bigint"), true},
		{"Supported CQLType - Varchar", ParseCqlTypeOrDie("varchar"), true},
		{"Unsupported CQLType - Boolean", ParseCqlTypeOrDie("boolean"), false},
		{"Unsupported CQLType - Float", ParseCqlTypeOrDie("float"), false},
		{"Unsupported CQLType - Blob", ParseCqlTypeOrDie("blob"), false},
		{"Unsupported CQLType - List", ParseCqlTypeOrDie("list<int>"), false},
		{"Unsupported CQLType - Frozen", ParseCqlTypeOrDie("frozen<list<int>>"), false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := IsSupportedPrimaryKeyType(tc.input)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestIsSupportedCollectionElementType(t *testing.T) {
	testCases := []struct {
		name     string
		input    datatype.DataType
		expected bool
	}{
		{"Supported CQLType - Int", datatype.Int, true},
		{"Supported CQLType - Bigint", datatype.Bigint, true},
		{"Supported CQLType - Varchar", datatype.Varchar, true},
		{"Supported CQLType - Float", datatype.Float, true},
		{"Supported CQLType - Double", datatype.Double, true},
		{"Supported CQLType - Timestamp", datatype.Timestamp, true},
		{"Supported CQLType - Boolean", datatype.Boolean, true},
		{"Unsupported CQLType - Blob", datatype.Blob, false},
		{"Unsupported CQLType - UUID", datatype.Uuid, false},
		{"Unsupported CQLType - Map", datatype.NewMapType(datatype.Varchar, datatype.Int), false},
		{"Unsupported CQLType - Set", datatype.NewSetType(datatype.Varchar), false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := isSupportedCollectionElementType(tc.input)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestIsSupportedColumnType(t *testing.T) {
	testCases := []struct {
		name     string
		input    types.CqlDataType
		expected bool
	}{
		// --- Positive Cases: Primitive Types ---
		{"Supported Primitive - Int", ParseCqlTypeOrDie("int"), true},
		{"Supported Primitive - Bigint", ParseCqlTypeOrDie("bigint"), true},
		{"Supported Primitive - Blob", ParseCqlTypeOrDie("blob"), true},
		{"Supported Primitive - Boolean", ParseCqlTypeOrDie("boolean"), true},
		{"Supported Primitive - Double", ParseCqlTypeOrDie("double"), true},
		{"Supported Primitive - Float", ParseCqlTypeOrDie("float"), true},
		{"Supported Primitive - Timestamp", ParseCqlTypeOrDie("timestamp"), true},
		{"Supported Primitive - Varchar", ParseCqlTypeOrDie("varchar"), true},

		// --- Positive Cases: Collection Types ---
		{"Supported List", ParseCqlTypeOrDie("list<int>"), true},
		{"Supported Set", ParseCqlTypeOrDie("set<varchar>"), true},
		{"Supported Map", ParseCqlTypeOrDie("map<timestamp,float>"), true},
		{"Supported Map with Text Key", ParseCqlTypeOrDie("map<text,bigint>"), true},

		// --- Negative Cases: Primitive Types ---
		{"Unsupported Primitive - UUID", ParseCqlTypeOrDie("uuid"), false},
		{"Unsupported Primitive - TimeUUID", ParseCqlTypeOrDie("timeuuid"), false},

		// --- Negative Cases: Collection Types ---
		{"Unsupported List Element", ParseCqlTypeOrDie("list<uuid>"), false},
		{"Unsupported List Element", types.NewListType(types.NewListType(types.TypeInt)), false},
		{"Unsupported Set Element", ParseCqlTypeOrDie("set<blob>"), false},
		{"Unsupported Map Key", ParseCqlTypeOrDie("map<blob,text>"), false},
		{"Unsupported Map Value", ParseCqlTypeOrDie("map<text,uuid>"), false},
		{"Nested Collection - List of Maps", types.NewListType(types.NewMapType(types.TypeVarchar, types.TypeInt)), false},
		// --- Negative Cases: Frozen Types ---
		{"Frozen List", ParseCqlTypeOrDie("frozen<list<int>>"), false},
		{"Frozen Set", ParseCqlTypeOrDie("frozen<set<text>>"), false},
		{"Frozen Map", ParseCqlTypeOrDie("frozen<map<timestamp, float>>"), false},
		{"Frozen Map with Text Key", ParseCqlTypeOrDie("frozen<map<text, bigint>>"), false},
		{"List of frozen lists", ParseCqlTypeOrDie("list<frozen<list<bigint>>>"), false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := IsSupportedColumnType(tc.input)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestGetCassandraColumnType(t *testing.T) {
	const NO_ERROR_EXPECTED = ""
	testCases := []struct {
		input    string
		wantType types.CqlDataType
		wantErr  string
	}{
		{"text", types.TypeText, NO_ERROR_EXPECTED},
		{"blob", types.TypeBlob, NO_ERROR_EXPECTED},
		{"timestamp", types.TypeTimestamp, NO_ERROR_EXPECTED},
		{"TimeSTAmp", types.TypeTimestamp, NO_ERROR_EXPECTED},
		{"int", types.TypeInt, NO_ERROR_EXPECTED},
		{"float", types.TypeFloat, NO_ERROR_EXPECTED},
		{"double", types.TypeDouble, NO_ERROR_EXPECTED},
		{"bigint", types.TypeBigint, NO_ERROR_EXPECTED},
		{"boolean", types.TypeBoolean, NO_ERROR_EXPECTED},
		{"uuid", types.TypeUuid, NO_ERROR_EXPECTED},
		{"map<text, boolean>", types.NewMapType(types.TypeText, types.TypeBoolean), NO_ERROR_EXPECTED},
		{"map<varchar, boolean>", types.NewMapType(types.TypeVarchar, types.TypeBoolean), NO_ERROR_EXPECTED},
		{"map<text, text>", types.NewMapType(types.TypeText, types.TypeText), NO_ERROR_EXPECTED},
		{"map<text, varchar>", types.NewMapType(types.TypeText, types.TypeVarchar), NO_ERROR_EXPECTED},
		{"map<varchar,text>", types.NewMapType(types.TypeVarchar, types.TypeText), NO_ERROR_EXPECTED},
		{"map<varchar, varchar>", types.NewMapType(types.TypeVarchar, types.TypeVarchar), NO_ERROR_EXPECTED},
		{"maP<vARCHar, varcHAr>", types.NewMapType(types.TypeVarchar, types.TypeVarchar), NO_ERROR_EXPECTED},
		{"map<varchar>", nil, "expected exactly 2 types but found 1 in: 'map<varchar>'"},
		{"map<varchar,varchar,varchar>", nil, "expected exactly 2 types but found 3 in: 'map<varchar,varchar,varchar>'"},
		{"map<>", nil, "empty type definition in 'map<>'"},
		{"int<>", nil, "unexpected data type definition: 'int<>'"},
		{"list<text>", types.NewListType(types.TypeText), NO_ERROR_EXPECTED},
		{"list<varchar>", types.NewListType(types.TypeVarchar), NO_ERROR_EXPECTED},
		{"frozen<list<text>>", types.NewFrozenType(types.NewListType(types.TypeText)), NO_ERROR_EXPECTED},
		{"frozen<list<varchar>>", types.NewFrozenType(types.NewListType(types.TypeVarchar)), NO_ERROR_EXPECTED},
		{"set<text>", types.NewSetType(types.TypeText), NO_ERROR_EXPECTED},
		{"set<text", nil, "missing closing type bracket in: 'set<text'"},
		{"set", nil, "data type definition missing in: 'set'"},
		{"frozen", nil, "data type definition missing in: 'frozen'"},
		{"map", nil, "data type definition missing in: 'map'"},
		{"list", nil, "data type definition missing in: 'list'"},
		{"set<", nil, "failed"}, // we don't get a good parsing error from here but that's ok
		{"set<varchar>", types.NewSetType(types.TypeVarchar), NO_ERROR_EXPECTED},
		{"frozen<set<text>>", types.NewFrozenType(types.NewSetType(types.TypeText)), NO_ERROR_EXPECTED},
		{"frozen<set<varchar>>", types.NewFrozenType(types.NewSetType(types.TypeVarchar)), NO_ERROR_EXPECTED},
		{"unknown", nil, "unknown data type name: 'unknown'"},
		{"", nil, "unknown data type name: ''"},
		{"<>list", nil, "unknown data type name: ''"},
		{"<int>", nil, "unknown data type name: '<int'"},
		{"map<map<text,int>", nil, "expected exactly 2 types but found 1 in: 'map<map<text,int>'"},
		{"map<map<text,int>,text>", nil, "map key types must be scalar"},
		{"map<text, frozen<map<text,int>>>", types.NewMapType(types.TypeText, types.NewFrozenType(types.NewMapType(types.TypeText, types.TypeInt))), NO_ERROR_EXPECTED},
		// Future scope items below:
		{"map<text, int>", types.NewMapType(types.TypeText, types.TypeInt), NO_ERROR_EXPECTED},
		{"map<varchar, int>", types.NewMapType(types.TypeVarchar, types.TypeInt), NO_ERROR_EXPECTED},
		{"map<text, bigint>", types.NewMapType(types.TypeText, types.TypeBigint), NO_ERROR_EXPECTED},
		{"map<varchar, bigint>", types.NewMapType(types.TypeVarchar, types.TypeBigint), NO_ERROR_EXPECTED},
		{"map<text, float>", types.NewMapType(types.TypeText, types.TypeFloat), NO_ERROR_EXPECTED},
		{"map<varchar, float>", types.NewMapType(types.TypeVarchar, types.TypeFloat), NO_ERROR_EXPECTED},
		{"map<text, double>", types.NewMapType(types.TypeText, types.TypeDouble), NO_ERROR_EXPECTED},
		{"map<varchar, double>", types.NewMapType(types.TypeVarchar, types.TypeDouble), NO_ERROR_EXPECTED},
		{"map<text, timestamp>", types.NewMapType(types.TypeText, types.TypeTimestamp), NO_ERROR_EXPECTED},
		{"map<varchar, timestamp>", types.NewMapType(types.TypeVarchar, types.TypeTimestamp), NO_ERROR_EXPECTED},
		{"map<timestamp, text>", types.NewMapType(types.TypeTimestamp, types.TypeText), NO_ERROR_EXPECTED},
		{"map<timestamp, varchar>", types.NewMapType(types.TypeTimestamp, types.TypeVarchar), NO_ERROR_EXPECTED},
		{"map<timestamp, boolean>", types.NewMapType(types.TypeTimestamp, types.TypeBoolean), NO_ERROR_EXPECTED},
		{"map<timestamp, int>", types.NewMapType(types.TypeTimestamp, types.TypeInt), NO_ERROR_EXPECTED},
		{"map<timestamp, bigint>", types.NewMapType(types.TypeTimestamp, types.TypeBigint), NO_ERROR_EXPECTED},
		{"map<timestamp, float>", types.NewMapType(types.TypeTimestamp, types.TypeFloat), NO_ERROR_EXPECTED},
		{"map<timestamp, double>", types.NewMapType(types.TypeTimestamp, types.TypeDouble), NO_ERROR_EXPECTED},
		{"map<timestamp, timestamp>", types.NewMapType(types.TypeTimestamp, types.TypeTimestamp), NO_ERROR_EXPECTED},
		{"map<timestamp, sdfs>", nil, "failed"}, // we don't get a good error because the lexer doesn't recognize the invalid type
		{"map<intt, int>", nil, "failed"},       // we don't get a good error because the lexer doesn't recognize the invalid type
		{"set<asdfasdf>", nil, "failed"},        // we don't get a good error because the lexer doesn't recognize the invalid type
		{"list<asdfasdf>", nil, "failed"},       // we don't get a good error because the lexer doesn't recognize the invalid type
		{"frozen<asdfasdf>", nil, "failed"},     // we don't get a good error because the lexer doesn't recognize the invalid type
		{"set<int>", types.NewSetType(types.TypeInt), NO_ERROR_EXPECTED},
		{"set<bigint>", types.NewSetType(types.TypeBigint), NO_ERROR_EXPECTED},
		{"set<float>", types.NewSetType(types.TypeFloat), NO_ERROR_EXPECTED},
		{"set<double>", types.NewSetType(types.TypeDouble), NO_ERROR_EXPECTED},
		{"set<boolean>", types.NewSetType(types.TypeBoolean), NO_ERROR_EXPECTED},
		{"set<timestamp>", types.NewSetType(types.TypeTimestamp), NO_ERROR_EXPECTED},
		{"set<text>", types.NewSetType(types.TypeText), NO_ERROR_EXPECTED},
		{"set<varchar>", types.NewSetType(types.TypeVarchar), NO_ERROR_EXPECTED},
		{"set<frozen<list<varchar>>>", types.NewSetType(types.NewFrozenType(types.NewListType(types.TypeVarchar))), NO_ERROR_EXPECTED},
		{"set<frozen<map<text,int>>>", types.NewSetType(types.NewFrozenType(types.NewMapType(types.TypeText, types.TypeInt))), NO_ERROR_EXPECTED},
		{"list<frozen<map<text,int>>>", types.NewListType(types.NewFrozenType(types.NewMapType(types.TypeText, types.TypeInt))), NO_ERROR_EXPECTED},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			got, err := ParseCqlTypeString(tc.input)
			if tc.wantErr != "" {
				assert.Nil(t, got)
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantType, got)
		})
	}
}

func TestFromDataCode(t *testing.T) {

	// ## Test Cases for Primitive Types ##
	// These tests verify that each primitive type is correctly converted.
	primitiveTests := []struct {
		name     string
		input    datatype.DataType
		expected types.CqlDataType
	}{
		{"Ascii", datatype.Ascii, types.TypeAscii},
		{"Bigint", datatype.Bigint, types.TypeBigint},
		{"Blob", datatype.Blob, types.TypeBlob},
		{"Boolean", datatype.Boolean, types.TypeBoolean},
		{"Counter", datatype.Counter, types.TypeCounter},
		{"Decimal", datatype.Decimal, types.TypeDecimal},
		{"Double", datatype.Double, types.TypeDouble},
		{"Float", datatype.Float, types.TypeFloat},
		{"Int", datatype.Int, types.TypeInt},
		{"Text", datatype.Varchar, types.TypeVarchar},
		{"Timestamp", datatype.Timestamp, types.TypeTimestamp},
		{"Uuid", datatype.Uuid, types.TypeUuid},
		{"Varchar", datatype.Varchar, types.TypeVarchar},
		{"Varint", datatype.Varint, types.TypeVarint},
		{"Timeuuid", datatype.Timeuuid, types.TypeTimeuuid},
		{"Inet", datatype.Inet, types.TypeInet},
		{"Date", datatype.Date, types.TypeDate},
		{"Time", datatype.Time, types.TypeTime},
		{"Smallint", datatype.Smallint, types.TypeSmallint},
		{"Tinyint", datatype.Tinyint, types.TypeTinyint},
	}

	for _, tc := range primitiveTests {
		t.Run(fmt.Sprintf("Primitive_%s", tc.name), func(t *testing.T) {
			got, err := FromDataCode(tc.input)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, got)
		})
	}

	// ## Test Cases for Collection Types ##
	// These tests verify simple and nested collections.
	collectionTests := []struct {
		name     string
		input    datatype.DataType
		expected types.CqlDataType
	}{
		{
			name:     "Simple List",
			input:    datatype.NewListType(datatype.Int),
			expected: types.NewListType(types.TypeInt),
		},
		{
			name:     "Simple Set",
			input:    datatype.NewSetType(datatype.Varchar),
			expected: types.NewSetType(types.TypeVarchar),
		},
		{
			name:     "Simple Map",
			input:    datatype.NewMapType(datatype.Uuid, datatype.Boolean),
			expected: types.NewMapType(types.TypeUuid, types.TypeBoolean),
		},
		{
			name:     "Nested Collection List<Map<Int, Text>>",
			input:    datatype.NewListType(datatype.NewMapType(datatype.Int, datatype.Varchar)),
			expected: types.NewListType(types.NewMapType(types.TypeInt, types.TypeVarchar)),
		},
	}

	for _, tc := range collectionTests {
		t.Run(fmt.Sprintf("Collection_%s", tc.name), func(t *testing.T) {
			got, err := FromDataCode(tc.input)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, got)
		})
	}

	// ## Test Cases for Errors ##
	// These tests ensure the function fails gracefully.
	errorTests := []struct {
		name        string
		input       datatype.DataType
		expectedErr string
	}{
		{
			name:        "Unhandled Primitive Type",
			input:       datatype.NewCustomType("foobar"), // An invalid/unhandled code
			expectedErr: "unhandled type: custom(foobar)",
		},
		{
			name:  "List with unhandled element type",
			input: datatype.NewListType(datatype.NewCustomType("foobar")), // An invalid/unhandled code

			expectedErr: "unhandled type: custom(foobar)",
		},
		{
			name:        "Map with unhandled key type",
			input:       datatype.NewMapType(datatype.Varchar, datatype.NewCustomType("foobar")),
			expectedErr: "unhandled type: custom(foobar)",
		},
		{
			name:        "Map with unhandled key type",
			input:       datatype.NewMapType(datatype.NewCustomType("foobar"), datatype.Varchar),
			expectedErr: "unhandled type: custom(foobar)",
		},
	}

	for _, tc := range errorTests {
		t.Run(fmt.Sprintf("Error_%s", tc.name), func(t *testing.T) {
			_, err := FromDataCode(tc.input)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectedErr)
		})
	}
}
