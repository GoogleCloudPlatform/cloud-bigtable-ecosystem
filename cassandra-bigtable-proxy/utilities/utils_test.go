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
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
)

// TestKeyExistsInList tests the keyExistsInList function with various inputs.
func TestKeyExistsInList(t *testing.T) {
	cases := []struct {
		name     string
		key      string
		list     []string
		expected bool
	}{
		{
			name:     "Key present in list",
			key:      "banana",
			list:     []string{"apple", "banana", "cherry"},
			expected: true,
		},
		{
			name:     "Key not present in list",
			key:      "mango",
			list:     []string{"apple", "banana", "cherry"},
			expected: false,
		},
		{
			name:     "Empty list",
			key:      "banana",
			list:     []string{},
			expected: false,
		},
		{
			name:     "Nil list",
			key:      "banana",
			list:     nil,
			expected: false,
		},
		{
			name:     "Key at the beginning",
			key:      "apple",
			list:     []string{"apple", "banana", "cherry"},
			expected: true,
		},
		{
			name:     "Key at the end",
			key:      "cherry",
			list:     []string{"apple", "banana", "cherry"},
			expected: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := KeyExistsInList(tc.key, tc.list)
			if got != tc.expected {
				t.Errorf("%s: expected %v, got %v", tc.name, tc.expected, got)
			}
		})
	}
}

func TestFormatTimestamp(t *testing.T) {
	tests := []struct {
		name          string
		input         int64
		expectedTime  time.Time
		expectedError error
	}{
		{
			name:          "Timestamp less than a second",
			input:         500, // Some value less than a second
			expectedTime:  time.Unix(500, 0),
			expectedError: nil,
		},
		{
			name:          "Timestamp in seconds",
			input:         1620655315, // Some value in seconds
			expectedTime:  time.Unix(1620655315, 0),
			expectedError: nil,
		},
		{
			name:          "Timestamp in milliseconds",
			input:         1620655315000, // Some value in milliseconds
			expectedTime:  time.Unix(1620655315, 0),
			expectedError: nil,
		},
		{
			name:          "Timestamp in microseconds",
			input:         1620655315000000, // Some value in microseconds
			expectedTime:  time.Unix(1620655315, 0),
			expectedError: nil,
		},
		{
			name:          "Timestamp in nanoseconds",
			input:         162065531000000000, // Some value in nanoseconds
			expectedTime:  time.Unix(162065531, 0),
			expectedError: nil,
		},
		{
			name:          "Zero timestamp",
			input:         0, // Invalid timestamp
			expectedTime:  time.Time{},
			expectedError: errors.New("no valid timestamp found"),
		},
		{
			name:          "Invalid timestamp",
			input:         2893187128318378367, // Invalid timestamp
			expectedTime:  time.Time{},
			expectedError: errors.New("no valid timestamp found"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := FormatTimestamp(test.input)

			if result != nil && !result.Equal(test.expectedTime) {
				t.Errorf("Expected time: %v, got: %v", test.expectedTime, result)
			}

			if !errorEquals(err, test.expectedError) {
				t.Errorf("Expected error: %v, got: %v", test.expectedError, err)
			}
		})
	}
}

func TestGetCassandraColumnType(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		wantType datatype.DataType
		wantErr  bool
	}{
		{"Text Type", "text", datatype.Varchar, false},
		{"Bolob Type", "blob", datatype.Blob, false},
		{"Timestamp Type", "timestamp", datatype.Timestamp, false},
		{"Int Type", "int", datatype.Int, false},
		{"Float Type", "float", datatype.Float, false},
		{"Double Type", "double", datatype.Double, false},
		{"Bigint Type", "bigint", datatype.Bigint, false},
		{"Boolean Type", "boolean", datatype.Boolean, false},
		{"Uuid Type", "uuid", datatype.Uuid, false},
		{"map<text, boolean> Type", "map<text, boolean>", datatype.NewMapType(datatype.Varchar, datatype.Boolean), false},
		{"map<text, text> Type", "map<text, text>", datatype.NewMapType(datatype.Varchar, datatype.Varchar), false},
		{"list<text> Type", "list<text>", datatype.NewListType(datatype.Varchar), false},
		{"frozen<list<text>> Type", "frozen<list<text>>", datatype.NewListType(datatype.Varchar), false},
		{"set<text> Type", "set<text>", datatype.NewSetType(datatype.Varchar), false},
		{"frozen<set<text>> Type", "frozen<set<text>>", datatype.NewSetType(datatype.Varchar), false},
		{"Invalid Type", "unknown", nil, true},
		// Future scope items below:
		{"map<text, int> Type", "map<text, int>", datatype.NewMapType(datatype.Varchar, datatype.Int), false},
		{"map<text, bigint> Type", "map<text, bigint>", datatype.NewMapType(datatype.Varchar, datatype.Bigint), false},
		{"map<text, float> Type", "map<text, float>", datatype.NewMapType(datatype.Varchar, datatype.Float), false},
		{"map<text, double> Type", "map<text, double>", datatype.NewMapType(datatype.Varchar, datatype.Double), false},
		{"map<text, timestamp> Type", "map<text, timestamp>", datatype.NewMapType(datatype.Varchar, datatype.Timestamp), false},
		{"map<timestamp, text> Type", "map<timestamp, text>", datatype.NewMapType(datatype.Timestamp, datatype.Varchar), false},
		{"map<timestamp, boolean> Type", "map<timestamp, boolean>", datatype.NewMapType(datatype.Timestamp, datatype.Boolean), false},
		{"map<timestamp, int> Type", "map<timestamp, int>", datatype.NewMapType(datatype.Timestamp, datatype.Int), false},
		{"map<timestamp, bigint> Type", "map<timestamp, bigint>", datatype.NewMapType(datatype.Timestamp, datatype.Bigint), false},
		{"map<timestamp, float> Type", "map<timestamp, float>", datatype.NewMapType(datatype.Timestamp, datatype.Float), false},
		{"map<timestamp, double> Type", "map<timestamp, double>", datatype.NewMapType(datatype.Timestamp, datatype.Double), false},
		{"map<timestamp, timestamp> Type", "map<timestamp, timestamp>", datatype.NewMapType(datatype.Timestamp, datatype.Timestamp), false},
		{"set<int> Type", "set<int>", datatype.NewSetType(datatype.Int), false},
		{"set<bigint> Type", "set<bigint>", datatype.NewSetType(datatype.Bigint), false},
		{"set<float> Type", "set<float>", datatype.NewSetType(datatype.Float), false},
		{"set<double> Type", "set<double>", datatype.NewSetType(datatype.Double), false},
		{"set<boolean> Type", "set<boolean>", datatype.NewSetType(datatype.Boolean), false},
		{"set<timestamp> Type", "set<timestamp>", datatype.NewSetType(datatype.Timestamp), false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotType, err := GetCassandraColumnType(tc.input)
			if (err != nil) != tc.wantErr {
				t.Errorf("getCassandraColumnType(%s) error = %v, wantErr %v", tc.input, err, tc.wantErr)
				return
			}

			if err == nil && !reflect.DeepEqual(gotType, tc.wantType) {
				t.Errorf("getCassandraColumnType(%s) = %v, want %v", tc.input, gotType, tc.wantType)
			}
		})
	}
}

func errorEquals(err1, err2 error) bool {
	if (err1 == nil && err2 != nil) || (err1 != nil && err2 == nil) {
		return false
	}
	if err1 != nil && err2 != nil && err1.Error() != err2.Error() {
		return false
	}
	return true
}

func TestDecodeEncodedValues(t *testing.T) {
	type args struct {
		value     []byte
		cqlType   string
		protocolV primitive.ProtocolVersion
	}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		{
			name: "Decode int value",
			args: args{
				value:     []byte{0, 0, 0, 10},
				cqlType:   "int",
				protocolV: primitive.ProtocolVersion4,
			},
			want:    int32(10),
			wantErr: false,
		},
		{
			name: "Decode timestamp value",
			args: args{
				value:     []byte{0, 0, 1, 96, 175, 4, 144, 0},
				cqlType:   "timestamp",
				protocolV: primitive.ProtocolVersion4,
			},
			want:    time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC),
			wantErr: false,
		},
		{
			name: "Decode bigint value",
			args: args{
				value:     []byte{0, 0, 0, 0, 0, 0, 0, 10},
				cqlType:   "bigint",
				protocolV: primitive.ProtocolVersion4,
			},
			want:    int64(10),
			wantErr: false,
		},
		{
			name: "Decode boolean value (true)",
			args: args{
				value:     []byte{1},
				cqlType:   "boolean",
				protocolV: primitive.ProtocolVersion4,
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "Decode float value",
			args: args{
				value:     []byte{0x40, 0x49, 0x0f, 0xdb},
				cqlType:   "float",
				protocolV: primitive.ProtocolVersion4,
			},
			want:    float32(3.141592653589793),
			wantErr: false,
		},
		{
			name: "Decode double value",
			args: args{
				value:     []byte{0x40, 0x09, 0x21, 0xfb, 0x54, 0x44, 0x2d, 0x18},
				cqlType:   "double",
				protocolV: primitive.ProtocolVersion4,
			},
			want:    float64(3.141592653589793),
			wantErr: false,
		},
		{
			name: "Decode blob value",
			args: args{
				value:     []byte{0x01, 0x02, 0x03, 0x04}, // represents a binary blob
				cqlType:   "blob",
				protocolV: primitive.ProtocolVersion4,
			},
			want:    []byte{0x01, 0x02, 0x03, 0x04},
			wantErr: false,
		},
		{
			name: "Decode text value",
			args: args{
				value:     []byte("hello"),
				cqlType:   "text",
				protocolV: primitive.ProtocolVersion4,
			},
			want:    "hello",
			wantErr: false,
		},
		{
			name: "Unsupported CQL type",
			args: args{
				value:     []byte{0, 0, 0, 0},
				cqlType:   "unsupported_type",
				protocolV: primitive.ProtocolVersion4,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Error in decoding",
			args: args{
				value:     []byte{0, 0},
				cqlType:   "int",
				protocolV: primitive.ProtocolVersion4,
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecodeEncodedValues(tt.args.value, tt.args.cqlType, tt.args.protocolV)
			if (err != nil) != tt.wantErr {
				t.Errorf("DecodeEncodedValues() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DecodeEncodedValues() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDecodeBytesToCassandraColumnType(t *testing.T) {
	bigIntList := []int64{}
	bigIntList = append(bigIntList, 12)

	type args struct {
		b               []byte
		choice          datatype.PrimitiveType
		protocolVersion primitive.ProtocolVersion
	}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		{
			name: "Decode varchar",
			args: args{
				b:               []byte("test string"),
				choice:          datatype.Varchar,
				protocolVersion: primitive.ProtocolVersion4,
			},
			want:    "test string",
			wantErr: false,
		},
		{
			name: "Decode double",
			args: args{
				b:               []byte{64, 9, 33, 251, 84, 68, 45, 24},
				choice:          datatype.Double,
				protocolVersion: primitive.ProtocolVersion4,
			},
			want:    float64(3.141592653589793),
			wantErr: false,
		},
		{
			name: "Decode double with float",
			args: args{
				b:               []byte{64, 9, 33, 251, 84, 68, 45, 24},
				choice:          datatype.Float,
				protocolVersion: primitive.ProtocolVersion4,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Decode float",
			args: args{
				b:               []byte{63, 133, 235, 81},
				choice:          datatype.Float,
				protocolVersion: primitive.ProtocolVersion4,
			},
			want:    float32(1.046244),
			wantErr: false,
		},
		{
			name: "Decode bigint",
			args: args{
				b:               []byte{0, 0, 0, 0, 0, 0, 0, 1},
				choice:          datatype.Bigint,
				protocolVersion: primitive.ProtocolVersion4,
			},
			want:    int64(1),
			wantErr: false,
		},
		{
			name: "Decode int",
			args: args{
				b:               []byte{0, 0, 0, 1},
				choice:          datatype.Int,
				protocolVersion: primitive.ProtocolVersion4,
			},
			want:    int64(1),
			wantErr: false,
		},
		{
			name: "Decode boolean",
			args: args{
				b:               []byte{1},
				choice:          datatype.Boolean,
				protocolVersion: primitive.ProtocolVersion4,
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "Decode uuid",
			args: args{
				b:               []byte{0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde, 0xf0, 0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde, 0xf0}, // Represents a UUID in bytes
				choice:          datatype.Uuid,
				protocolVersion: primitive.ProtocolVersion4,
			},
			want:    "12345678-9abc-def0-1234-56789abcdef0",
			wantErr: false,
		},
		{
			name: "Error in Decode uuid",
			args: args{
				b:               []byte{0x80, 0x00, 0x00, 0x00},
				choice:          datatype.Uuid,
				protocolVersion: primitive.ProtocolVersion4,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Error in Decode Int",
			args: args{
				b:               []byte("hello"),
				choice:          datatype.Int,
				protocolVersion: primitive.ProtocolVersion4,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Decode date",
			args: args{
				b:               []byte{0x80, 0x00, 0x00, 0x00},
				choice:          datatype.Date,
				protocolVersion: primitive.ProtocolVersion4,
			},
			want:    time.Unix(0, 0).UTC(),
			wantErr: false,
		},
		{
			name: "Decode blob",
			args: args{
				b:               []byte{0x01, 0x02, 0x03},
				choice:          datatype.Blob,
				protocolVersion: primitive.ProtocolVersion4,
			},
			want:    []byte{0x01, 0x02, 0x03},
			wantErr: false,
		},
		{
			name: "Decode Timestamp",
			args: args{
				b:               []byte{0, 0, 1, 96, 175, 4, 144, 0},
				choice:          datatype.Timestamp,
				protocolVersion: primitive.ProtocolVersion4,
			},
			want:    time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC),
			wantErr: false,
		},
		{
			name: "Should go in the default and throw error",
			args: args{
				b:               []byte{0, 0, 1, 96, 175, 4, 144, 0},
				choice:          datatype.Smallint,
				protocolVersion: primitive.ProtocolVersion4,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Should go in the default and run the bigint list",
			args: args{
				b:               []byte{0, 0, 0, 1, 0, 0, 0, 8, 0, 0, 0, 0, 0, 0, 0, 12},
				choice:          datatype.NewListType(datatype.Bigint),
				protocolVersion: primitive.ProtocolVersion4,
			},
			want:    bigIntList,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecodeBytesToCassandraColumnType(tt.args.b, tt.args.choice, tt.args.protocolVersion)
			if (err != nil) != tt.wantErr {
				t.Errorf("DecodeBytesToCassandraColumnType() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DecodeBytesToCassandraColumnType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSetupLogger(t *testing.T) {
	type args struct {
		logLevel     string
		loggerConfig *LoggerConfig
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "info log level",
			args: args{
				logLevel:     "info",
				loggerConfig: nil,
			},
			wantErr: false,
		},
		{
			name: "debug log level",
			args: args{
				logLevel:     "debug",
				loggerConfig: nil,
			},
			wantErr: false,
		},
		{
			name: "error log level",
			args: args{
				logLevel:     "error",
				loggerConfig: nil,
			},
			wantErr: false,
		},
		{
			name: "warn log level",
			args: args{
				logLevel:     "warn",
				loggerConfig: nil,
			},
			wantErr: false,
		},
		{
			name: "default log level",
			args: args{
				logLevel:     "default",
				loggerConfig: nil,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SetupLogger(tt.args.logLevel, tt.args.loggerConfig)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetupLogger() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got == nil {
				t.Errorf("SetupLogger() = %v", got)
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
