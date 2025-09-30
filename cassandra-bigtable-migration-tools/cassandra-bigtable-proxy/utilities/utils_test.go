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
				b, _ := proxycore.EncodeType(ListOfStr, primitive.ProtocolVersion4, []string{"test1", "test2"})
				return b
			}(),
			dataType:        ListOfStr,
			protocolVersion: primitive.ProtocolVersion4,
			expected:        []string{"test1", "test2"},
			expectError:     false,
		},
		{
			name: "Decode list of bigint",
			input: func() []byte {
				b, _ := proxycore.EncodeType(ListOfBigInt, primitive.ProtocolVersion4, []int64{123, 456})
				return b
			}(),
			dataType:        ListOfBigInt,
			protocolVersion: primitive.ProtocolVersion4,
			expected:        []int64{123, 456},
			expectError:     false,
		},
		{
			name: "Decode list of double",
			input: func() []byte {
				b, _ := proxycore.EncodeType(ListOfDouble, primitive.ProtocolVersion4, []float64{123.45, 456.78})
				return b
			}(),
			dataType:        ListOfDouble,
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
			dataType:        ListOfBool, // List of boolean is not supported
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
				b, _ := proxycore.EncodeType(ListOfStr, primitive.ProtocolVersion4, []string{"test1", "test2"})
				return b
			}(),
			dataType:    ListOfStr,
			expected:    []string{"test1", "test2"},
			expectError: false,
		},
		{
			name: "Decode list of bigint",
			input: func() []byte {
				b, _ := proxycore.EncodeType(ListOfBigInt, primitive.ProtocolVersion4, []int64{123, 456})
				return b
			}(),
			dataType:    ListOfBigInt,
			expected:    []int64{123, 456},
			expectError: false,
		},
		{
			name: "Decode list of double",
			input: func() []byte {
				b, _ := proxycore.EncodeType(ListOfDouble, primitive.ProtocolVersion4, []float64{123.45, 456.78})
				return b
			}(),
			dataType:    ListOfDouble,
			expected:    []float64{123.45, 456.78},
			expectError: false,
		},
		{
			name:         "Unsupported list element type",
			input:        []byte("test"),
			dataType:     ListOfBool, // List of boolean is not supported
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
		input    *types.CqlTypeInfo
		expected bool
	}{
		{"Supported TypeInfo - Int", ParseCqlTypeOrDie("int"), true},
		{"Supported TypeInfo - Bigint", ParseCqlTypeOrDie("bigint"), true},
		{"Supported TypeInfo - Varchar", ParseCqlTypeOrDie("varchar"), true},
		{"Unsupported TypeInfo - Boolean", ParseCqlTypeOrDie("boolean"), false},
		{"Unsupported TypeInfo - Float", ParseCqlTypeOrDie("float"), false},
		{"Unsupported TypeInfo - Blob", ParseCqlTypeOrDie("blob"), false},
		{"Unsupported TypeInfo - List", ParseCqlTypeOrDie("list<int>"), false},
		{"Unsupported TypeInfo - Frozen", ParseCqlTypeOrDie("frozen<list<int>>"), false},
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
		{"Supported TypeInfo - Int", datatype.Int, true},
		{"Supported TypeInfo - Bigint", datatype.Bigint, true},
		{"Supported TypeInfo - Varchar", datatype.Varchar, true},
		{"Supported TypeInfo - Float", datatype.Float, true},
		{"Supported TypeInfo - Double", datatype.Double, true},
		{"Supported TypeInfo - Timestamp", datatype.Timestamp, true},
		{"Supported TypeInfo - Boolean", datatype.Boolean, true},
		{"Unsupported TypeInfo - Blob", datatype.Blob, false},
		{"Unsupported TypeInfo - UUID", datatype.Uuid, false},
		{"Unsupported TypeInfo - Map", datatype.NewMapType(datatype.Varchar, datatype.Int), false},
		{"Unsupported TypeInfo - Set", datatype.NewSetType(datatype.Varchar), false},
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
		input    *types.CqlTypeInfo
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
		{"Unsupported List Element", ParseCqlTypeOrDie("list<list<text>>"), false},
		{"Unsupported Set Element", ParseCqlTypeOrDie("set<blob>"), false},
		{"Unsupported Map Key", ParseCqlTypeOrDie("map<blob,text>"), false},
		{"Unsupported Map Value", ParseCqlTypeOrDie("map<text,uuid>"), false},
		{"Nested Collection - List of Maps", ParseCqlTypeOrDie("list<map<text,int>>"), false},
		// --- Negative Cases: Frozen Types ---
		{"Frozen List", ParseCqlTypeOrDie("frozen<list<int>>"), false},
		{"Frozen Set", ParseCqlTypeOrDie("frozen<set<text>>"), false},
		{"Frozen Map", ParseCqlTypeOrDie("frozen<map<timestamp, float>>"), false},
		{"Frozen Map with Text Key", ParseCqlTypeOrDie("frozen<map<text, bigint>>"), false},
		{"List of frozen lists", ParseCqlTypeOrDie("list<frozen<list<bigint>>>"), false},

		// --- Negative Cases: UDT ---
		{"udt", types.NewCqlTypeInfoFromType(datatype.NewCustomType("foobar")), false},
		{"list of frozen udt", types.NewCqlTypeInfoFromType(datatype.NewListType(datatype.NewCustomType("foobar"))), false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := IsSupportedColumnType(tc.input)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestGetCassandraColumnType(t *testing.T) {
	testCases := []struct {
		input    string
		wantType *types.CqlTypeInfo
		wantErr  string
	}{
		{"text", types.NewCqlTypeInfo("text", datatype.Varchar, false), ""},
		{"blob", types.NewCqlTypeInfo("blob", datatype.Blob, false), ""},
		{"timestamp", types.NewCqlTypeInfo("timestamp", datatype.Timestamp, false), ""},
		{"int", types.NewCqlTypeInfo("int", datatype.Int, false), ""},
		{"float", types.NewCqlTypeInfo("float", datatype.Float, false), ""},
		{"double", types.NewCqlTypeInfo("double", datatype.Double, false), ""},
		{"bigint", types.NewCqlTypeInfo("bigint", datatype.Bigint, false), ""},
		{"boolean", types.NewCqlTypeInfo("boolean", datatype.Boolean, false), ""},
		{"uuid", types.NewCqlTypeInfo("uuid", datatype.Uuid, false), ""},
		{"map<text, boolean>", types.NewCqlTypeInfo("map<text,boolean>", datatype.NewMapType(datatype.Varchar, datatype.Boolean), false), ""},
		{"map<varchar, boolean>", types.NewCqlTypeInfo("map<varchar,boolean>", datatype.NewMapType(datatype.Varchar, datatype.Boolean), false), ""},
		{"map<text, text>", types.NewCqlTypeInfo("map<text,text>", datatype.NewMapType(datatype.Varchar, datatype.Varchar), false), ""},
		{"map<text, varchar>", types.NewCqlTypeInfo("map<text,varchar>", datatype.NewMapType(datatype.Varchar, datatype.Varchar), false), ""},
		{"map<varchar,text>", types.NewCqlTypeInfo("map<varchar,text>", datatype.NewMapType(datatype.Varchar, datatype.Varchar), false), ""},
		{"map<varchar, varchar>", types.NewCqlTypeInfo("map<varchar,varchar>", datatype.NewMapType(datatype.Varchar, datatype.Varchar), false), ""},
		{"map<varchar>", nil, "malformed map type"},
		{"map<varchar,varchar,varchar>", nil, "unsupported column type"},
		{"map<>", nil, "unsupported column type"},
		{"int<>", nil, "unsupported column type"},
		{"list<text>", types.NewCqlTypeInfo("list<text>", datatype.NewListType(datatype.Varchar), false), ""},
		{"list<varchar>", types.NewCqlTypeInfo("list<varchar>", datatype.NewListType(datatype.Varchar), false), ""},
		{"frozen<list<text>>", types.NewCqlTypeInfo("frozen<list<text>>", datatype.NewListType(datatype.Varchar), true), ""},
		{"frozen<list<varchar>>", types.NewCqlTypeInfo("frozen<list<varchar>>", datatype.NewListType(datatype.Varchar), true), ""},
		{"set<text>", types.NewCqlTypeInfo("set<text>", datatype.NewSetType(datatype.Varchar), false), ""},
		{"set<text", nil, "unsupported column type"},
		{"set<", nil, "unsupported column type"},
		{"set<varchar>", types.NewCqlTypeInfo("set<varchar>", datatype.NewSetType(datatype.Varchar), false), ""},
		{"frozen<set<text>>", types.NewCqlTypeInfo("frozen<set<text>>", datatype.NewSetType(datatype.Varchar), true), ""},
		{"frozen<set<varchar>>", types.NewCqlTypeInfo("frozen<set<varchar>>", datatype.NewSetType(datatype.Varchar), true), ""},
		{"unknown", nil, "unsupported column type"},
		{"", nil, "unsupported column type"},
		{"<>list", nil, "unsupported column type"},
		{"<int>", nil, "unsupported column type"},
		{"map<map<text,int>", nil, "unsupported column type"},
		// Future scope items below:
		{"map<text, int>", types.NewCqlTypeInfo("map<text,int>", datatype.NewMapType(datatype.Varchar, datatype.Int), false), ""},
		{"map<varchar, int>", types.NewCqlTypeInfo("map<varchar,int>", datatype.NewMapType(datatype.Varchar, datatype.Int), false), ""},
		{"map<text, bigint>", types.NewCqlTypeInfo("map<text,bigint>", datatype.NewMapType(datatype.Varchar, datatype.Bigint), false), ""},
		{"map<varchar, bigint>", types.NewCqlTypeInfo("map<varchar,bigint>", datatype.NewMapType(datatype.Varchar, datatype.Bigint), false), ""},
		{"map<text, float>", types.NewCqlTypeInfo("map<text,float>", datatype.NewMapType(datatype.Varchar, datatype.Float), false), ""},
		{"map<varchar, float>", types.NewCqlTypeInfo("map<varchar,float>", datatype.NewMapType(datatype.Varchar, datatype.Float), false), ""},
		{"map<text, double>", types.NewCqlTypeInfo("map<text,double>", datatype.NewMapType(datatype.Varchar, datatype.Double), false), ""},
		{"map<varchar, double>", types.NewCqlTypeInfo("map<varchar,double>", datatype.NewMapType(datatype.Varchar, datatype.Double), false), ""},
		{"map<text, timestamp>", types.NewCqlTypeInfo("map<text,timestamp>", datatype.NewMapType(datatype.Varchar, datatype.Timestamp), false), ""},
		{"map<varchar, timestamp>", types.NewCqlTypeInfo("map<varchar,timestamp>", datatype.NewMapType(datatype.Varchar, datatype.Timestamp), false), ""},
		{"map<timestamp, text>", types.NewCqlTypeInfo("map<timestamp,text>", datatype.NewMapType(datatype.Timestamp, datatype.Varchar), false), ""},
		{"map<timestamp, varchar>", types.NewCqlTypeInfo("map<timestamp,varchar>", datatype.NewMapType(datatype.Timestamp, datatype.Varchar), false), ""},
		{"map<timestamp, boolean>", types.NewCqlTypeInfo("map<timestamp,boolean>", datatype.NewMapType(datatype.Timestamp, datatype.Boolean), false), ""},
		{"map<timestamp, int>", types.NewCqlTypeInfo("map<timestamp,int>", datatype.NewMapType(datatype.Timestamp, datatype.Int), false), ""},
		{"map<timestamp, bigint>", types.NewCqlTypeInfo("map<timestamp,bigint>", datatype.NewMapType(datatype.Timestamp, datatype.Bigint), false), ""},
		{"map<timestamp, float>", types.NewCqlTypeInfo("map<timestamp,float>", datatype.NewMapType(datatype.Timestamp, datatype.Float), false), ""},
		{"map<timestamp, double>", types.NewCqlTypeInfo("map<timestamp,double>", datatype.NewMapType(datatype.Timestamp, datatype.Double), false), ""},
		{"map<timestamp, timestamp>", types.NewCqlTypeInfo("map<timestamp,timestamp>", datatype.NewMapType(datatype.Timestamp, datatype.Timestamp), false), ""},
		{"set<int>", types.NewCqlTypeInfo("set<int>", datatype.NewSetType(datatype.Int), false), ""},
		{"set<bigint>", types.NewCqlTypeInfo("set<bigint>", datatype.NewSetType(datatype.Bigint), false), ""},
		{"set<float>", types.NewCqlTypeInfo("set<float>", datatype.NewSetType(datatype.Float), false), ""},
		{"set<double>", types.NewCqlTypeInfo("set<double>", datatype.NewSetType(datatype.Double), false), ""},
		{"set<boolean>", types.NewCqlTypeInfo("set<boolean>", datatype.NewSetType(datatype.Boolean), false), ""},
		{"set<timestamp>", types.NewCqlTypeInfo("set<timestamp>", datatype.NewSetType(datatype.Timestamp), false), ""},
		{"set<text>", types.NewCqlTypeInfo("set<text>", datatype.NewSetType(datatype.Varchar), false), ""},
		{"set<varchar>", types.NewCqlTypeInfo("set<varchar>", datatype.NewSetType(datatype.Varchar), false), ""},
		{"set<frozen<list<varchar>>>", types.NewCqlTypeInfo("set<frozen<list<varchar>>>", datatype.NewSetType(datatype.NewListType(datatype.Varchar)), true), ""},
		{"set<frozen<map<text,int>>>", types.NewCqlTypeInfo("set<frozen<map<text,int>>>", datatype.NewSetType(datatype.NewMapType(datatype.Varchar, datatype.Int)), true), ""},
		{"list<frozen<map<text,int>>>", types.NewCqlTypeInfo("list<frozen<map<text,int>>>", datatype.NewListType(datatype.NewMapType(datatype.Varchar, datatype.Int)), true), ""},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			gotType, err := ParseCqlType(tc.input)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantType, gotType)
		})
	}
}
