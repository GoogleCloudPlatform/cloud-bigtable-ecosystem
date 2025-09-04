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
		input    datatype.DataType
		expected bool
	}{
		{"Supported Type - Int", datatype.Int, true},
		{"Supported Type - Bigint", datatype.Bigint, true},
		{"Supported Type - Varchar", datatype.Varchar, true},
		{"Unsupported Type - Boolean", datatype.Boolean, false},
		{"Unsupported Type - Float", datatype.Float, false},
		{"Unsupported Type - Blob", datatype.Blob, false},
		{"Unsupported Type - List", datatype.NewListType(datatype.Int), false},
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
		{"Supported Type - Int", datatype.Int, true},
		{"Supported Type - Bigint", datatype.Bigint, true},
		{"Supported Type - Varchar", datatype.Varchar, true},
		{"Supported Type - Float", datatype.Float, true},
		{"Supported Type - Double", datatype.Double, true},
		{"Supported Type - Timestamp", datatype.Timestamp, true},
		{"Supported Type - Boolean", datatype.Boolean, true},
		{"Unsupported Type - Blob", datatype.Blob, false},
		{"Unsupported Type - UUID", datatype.Uuid, false},
		{"Unsupported Type - Map", datatype.NewMapType(datatype.Varchar, datatype.Int), false},
		{"Unsupported Type - Set", datatype.NewSetType(datatype.Varchar), false},
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
		input    datatype.DataType
		expected bool
	}{
		// --- Positive Cases: Primitive Types ---
		{"Supported Primitive - Int", datatype.Int, true},
		{"Supported Primitive - Bigint", datatype.Bigint, true},
		{"Supported Primitive - Blob", datatype.Blob, true},
		{"Supported Primitive - Boolean", datatype.Boolean, true},
		{"Supported Primitive - Double", datatype.Double, true},
		{"Supported Primitive - Float", datatype.Float, true},
		{"Supported Primitive - Timestamp", datatype.Timestamp, true},
		{"Supported Primitive - Varchar", datatype.Varchar, true},

		// --- Positive Cases: Collection Types ---
		{"Supported List", datatype.NewListType(datatype.Int), true},
		{"Supported Set", datatype.NewSetType(datatype.Varchar), true},
		{"Supported Map", datatype.NewMapType(datatype.Timestamp, datatype.Float), true},
		{"Supported Map with Text Key", datatype.NewMapType(datatype.Varchar, datatype.Bigint), true},

		// --- Negative Cases: Primitive Types ---
		{"Unsupported Primitive - UUID", datatype.Uuid, false},
		{"Unsupported Primitive - TimeUUID", datatype.Timeuuid, false},

		// --- Negative Cases: Collection Types ---
		{"Unsupported List Element", datatype.NewListType(datatype.Uuid), false},
		{"Unsupported Set Element", datatype.NewSetType(datatype.Blob), false},
		{"Unsupported Map Key", datatype.NewMapType(datatype.Blob, datatype.Varchar), false},
		{"Unsupported Map Value", datatype.NewMapType(datatype.Varchar, datatype.Uuid), false},
		{"Nested Collection - List of Maps", datatype.NewListType(datatype.NewMapType(datatype.Varchar, datatype.Int)), false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := IsSupportedColumnType(tc.input)
			assert.Equal(t, tc.expected, got)
		})
	}
}
