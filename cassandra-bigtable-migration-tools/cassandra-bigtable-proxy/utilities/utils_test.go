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
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCqlTimestamp(t *testing.T) {
	// Define a helper to create times safely without error checking in the struct
	mustParse := func(layout, value string) time.Time {
		t, err := time.Parse(layout, value)
		if err != nil {
			panic(err)
		}
		return t
	}

	tests := []struct {
		name    string
		input   string
		want    time.Time
		wantErr bool
	}{
		{
			name:    "Standard RFC3339",
			input:   "2023-11-27T10:30:00Z",
			want:    mustParse(time.RFC3339, "2023-11-27T10:30:00Z"),
			wantErr: false,
		},
		{
			name:  "Simple Date (YYYY-MM-DD)",
			input: "2023-11-27",
			// Assuming simple date defaults to midnight UTC
			want:    mustParse("2006-01-02", "2023-11-27"),
			wantErr: false,
		},
		{
			name:    "Date with Space and Time",
			input:   "2023-11-27 10:30:00",
			want:    mustParse("2006-01-02 15:04:05", "2023-11-27 10:30:00"),
			wantErr: false,
		},
		{
			name:    "Date with Milliseconds",
			input:   "2023-11-27 10:30:00.555",
			want:    mustParse("2006-01-02 15:04:05.000", "2023-11-27 10:30:00.555"),
			wantErr: false,
		},
		{
			name:    "Invalid Format - Nonsense String",
			input:   "not-a-timestamp",
			want:    time.Time{},
			wantErr: true,
		},
		{
			name:    "Invalid Date - Month out of range",
			input:   "2023-13-01 10:00:00",
			want:    time.Time{},
			wantErr: true,
		},
		{
			name:    "Empty String",
			input:   "",
			want:    time.Time{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCqlTimestamp(tt.input)

			// 1. Check Error State
			if (err != nil) != tt.wantErr {
				t.Errorf("parseCqlTimestamp() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// 2. Check Time Equality
			// We use .Equal() rather than == to handle monotonic clock differences
			// and timezone pointer differences.
			if !tt.wantErr && !got.Equal(tt.want) {
				t.Errorf("parseCqlTimestamp() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStringToPrimitives(t *testing.T) {
	tests := []struct {
		value    string
		cqlType  types.CqlDataType
		expected interface{}
		hasError bool
	}{
		{"123", types.TypeInt, int32(123), false},
		{"not_an_int", types.TypeInt, nil, true},
		{"123456789", types.TypeBigInt, int64(123456789), false},
		{"not_a_bigint", types.TypeBigInt, nil, true},
		{"3.14", types.TypeFloat, float32(3.14), false},
		{"not_a_float", types.TypeFloat, nil, true},
		{"3.1415926535", types.TypeDouble, 3.1415926535, false},
		{"not_a_double", types.TypeDouble, nil, true},
		{"true", types.TypeBoolean, true, false},
		{"false", types.TypeBoolean, false, false},
		{"not_a_boolean", types.TypeBoolean, nil, true},
		{"0x013A", types.TypeBlob, []byte{0x1, 0x3a}, false},
		{"hello", types.TypeVarchar, "hello", false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_%s", tt.cqlType, tt.value), func(t *testing.T) {
			result, err := StringToGo(tt.value, tt.cqlType)
			if tt.hasError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
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
	testCases := []struct {
		input    string
		wantType types.CqlDataType
		wantErr  string
	}{
		{
			input:    "text",
			wantType: types.TypeText,
		},
		{
			input:    "blob",
			wantType: types.TypeBlob,
		},
		{
			input:    "timestamp",
			wantType: types.TypeTimestamp,
		},
		{
			input:    "TimeSTAmp",
			wantType: types.TypeTimestamp,
		},
		{
			input:    "int",
			wantType: types.TypeInt,
		},
		{
			input:    "float",
			wantType: types.TypeFloat,
		},
		{
			input:    "double",
			wantType: types.TypeDouble,
		},
		{
			input:    "bigint",
			wantType: types.TypeBigInt,
		},
		{
			input:    "boolean",
			wantType: types.TypeBoolean,
		},
		{
			input:    "uuid",
			wantType: types.TypeUuid,
		},
		{
			input:    "map<text, boolean>",
			wantType: types.NewMapType(types.TypeText, types.TypeBoolean),
		},
		{
			input:    "map<varchar, boolean>",
			wantType: types.NewMapType(types.TypeVarchar, types.TypeBoolean),
		},
		{
			input:    "map<text, text>",
			wantType: types.NewMapType(types.TypeText, types.TypeText),
		},
		{
			input:    "map<text, varchar>",
			wantType: types.NewMapType(types.TypeText, types.TypeVarchar),
		},
		{
			input:    "map<varchar,text>",
			wantType: types.NewMapType(types.TypeVarchar, types.TypeText),
		},
		{
			input:    "map<varchar, varchar>",
			wantType: types.NewMapType(types.TypeVarchar, types.TypeVarchar),
		},
		{
			input:    "maP<vARCHar, varcHAr>",
			wantType: types.NewMapType(types.TypeVarchar, types.TypeVarchar),
		},
		{"map<varchar>", nil, "expected exactly 2 types but found 1 in: 'map<varchar>'"},
		{"map<varchar,varchar,varchar>", nil, "expected exactly 2 types but found 3 in: 'map<varchar,varchar,varchar>'"},
		{"map<>", nil, "empty type definition in 'map<>'"},
		{"int<>", nil, "unexpected data type definition: 'int<>'"},
		{
			input:    "list<text>",
			wantType: types.NewListType(types.TypeText),
		},
		{
			input:    "list<varchar>",
			wantType: types.NewListType(types.TypeVarchar),
		},
		{
			input:    "frozen<list<text>>",
			wantType: types.NewFrozenType(types.NewListType(types.TypeText)),
		},
		{
			input:    "frozen<list<varchar>>",
			wantType: types.NewFrozenType(types.NewListType(types.TypeVarchar)),
		},
		{
			input:    "set<text>",
			wantType: types.NewSetType(types.TypeText),
		},
		{"set<text", nil, "missing closing type bracket in: 'set<text'"},
		{"set", nil, "data type definition missing in: 'set'"},
		{"frozen", nil, "data type definition missing in: 'frozen'"},
		{"map", nil, "data type definition missing in: 'map'"},
		{"list", nil, "data type definition missing in: 'list'"},
		{"set<", nil, "failed"}, // we don't get a good parsing error from here but that's ok
		{
			input:    "set<varchar>",
			wantType: types.NewSetType(types.TypeVarchar),
		},
		{
			input:    "frozen<set<text>>",
			wantType: types.NewFrozenType(types.NewSetType(types.TypeText)),
		},
		{
			input:    "frozen<set<varchar>>",
			wantType: types.NewFrozenType(types.NewSetType(types.TypeVarchar)),
		},
		{"unknown", nil, "unknown data type name: 'unknown'"},
		{"", nil, "unknown data type name: ''"},
		{"<>list", nil, "unknown data type name: ''"},
		{"<int>", nil, "unknown data type name: '<int'"},
		{"map<map<text,int>", nil, "expected exactly 2 types but found 1 in: 'map<map<text,int>'"},
		{"map<map<text,int>,text>", nil, "map key types must be scalar"},
		{
			input:    "map<text, frozen<map<text,int>>>",
			wantType: types.NewMapType(types.TypeText, types.NewFrozenType(types.NewMapType(types.TypeText, types.TypeInt))),
		},
		// Future scope items below:
		{
			input:    "map<text, int>",
			wantType: types.NewMapType(types.TypeText, types.TypeInt),
		},
		{
			input:    "map<varchar, int>",
			wantType: types.NewMapType(types.TypeVarchar, types.TypeInt),
		},
		{
			input:    "map<text, bigint>",
			wantType: types.NewMapType(types.TypeText, types.TypeBigInt),
		},
		{
			input:    "map<varchar, bigint>",
			wantType: types.NewMapType(types.TypeVarchar, types.TypeBigInt),
		},
		{
			input:    "map<text, float>",
			wantType: types.NewMapType(types.TypeText, types.TypeFloat),
		},
		{
			input:    "map<varchar, float>",
			wantType: types.NewMapType(types.TypeVarchar, types.TypeFloat),
		},
		{
			input:    "map<text, double>",
			wantType: types.NewMapType(types.TypeText, types.TypeDouble),
		},
		{
			input:    "map<varchar, double>",
			wantType: types.NewMapType(types.TypeVarchar, types.TypeDouble),
		},
		{
			input:    "map<text, timestamp>",
			wantType: types.NewMapType(types.TypeText, types.TypeTimestamp),
		},
		{
			input:    "map<varchar, timestamp>",
			wantType: types.NewMapType(types.TypeVarchar, types.TypeTimestamp),
		},
		{
			input:    "map<timestamp, text>",
			wantType: types.NewMapType(types.TypeTimestamp, types.TypeText),
		},
		{
			input:    "map<timestamp, varchar>",
			wantType: types.NewMapType(types.TypeTimestamp, types.TypeVarchar),
		},
		{
			input:    "map<timestamp, boolean>",
			wantType: types.NewMapType(types.TypeTimestamp, types.TypeBoolean),
		},
		{
			input:    "map<timestamp, int>",
			wantType: types.NewMapType(types.TypeTimestamp, types.TypeInt),
		},
		{
			input:    "map<timestamp, bigint>",
			wantType: types.NewMapType(types.TypeTimestamp, types.TypeBigInt),
		},
		{
			input:    "map<timestamp, float>",
			wantType: types.NewMapType(types.TypeTimestamp, types.TypeFloat),
		},
		{
			input:    "map<timestamp, double>",
			wantType: types.NewMapType(types.TypeTimestamp, types.TypeDouble),
		},
		{
			input:    "map<timestamp, timestamp>",
			wantType: types.NewMapType(types.TypeTimestamp, types.TypeTimestamp),
		},
		{
			input:   "map<timestamp, sdfs>",
			wantErr: "failed", // we don't get a good error because the lexer doesn't recognize the invalid type
		},
		{
			input:   "map<intt, int>",
			wantErr: "failed", // we don't get a good error because the lexer doesn't recognize the invalid type
		},
		{
			input:   "set<asdfasdf>",
			wantErr: "failed", // we don't get a good error because the lexer doesn't recognize the invalid type
		},
		{
			input:   "list<asdfasdf>",
			wantErr: "failed", // we don't get a good error because the lexer doesn't recognize the invalid type
		},
		{
			input:   "frozen<asdfasdf>",
			wantErr: "failed", // we don't get a good error because the lexer doesn't recognize the invalid type
		},
		{
			input:    "set<int>",
			wantType: types.NewSetType(types.TypeInt),
		},
		{
			input:    "set<bigint>",
			wantType: types.NewSetType(types.TypeBigInt),
		},
		{
			input:    "set<float>",
			wantType: types.NewSetType(types.TypeFloat),
		},
		{
			input:    "set<double>",
			wantType: types.NewSetType(types.TypeDouble),
		},
		{
			input:    "set<boolean>",
			wantType: types.NewSetType(types.TypeBoolean),
		},
		{
			input:    "set<timestamp>",
			wantType: types.NewSetType(types.TypeTimestamp),
		},
		{
			input:    "set<text>",
			wantType: types.NewSetType(types.TypeText),
		},
		{
			input:    "set<varchar>",
			wantType: types.NewSetType(types.TypeVarchar),
		},
		{
			input:    "set<frozen<list<varchar>>>",
			wantType: types.NewSetType(types.NewFrozenType(types.NewListType(types.TypeVarchar))),
		},
		{
			input:    "set<frozen<map<text,int>>>",
			wantType: types.NewSetType(types.NewFrozenType(types.NewMapType(types.TypeText, types.TypeInt))),
		},
		{
			input:    "list<frozen<map<text,int>>>",
			wantType: types.NewListType(types.NewFrozenType(types.NewMapType(types.TypeText, types.TypeInt))),
		},
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
		{"Bigint", datatype.Bigint, types.TypeBigInt},
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
		{"timestamp", datatype.Time, types.TypeTime},
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

	// ## Test Cases for errs ##
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
