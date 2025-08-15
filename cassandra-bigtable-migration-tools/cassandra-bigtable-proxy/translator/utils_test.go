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

package translator

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/bigtable"
	types "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	schemaMapping "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/schema-mapping"
	cql "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/third_party/cqlparser"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/third_party/datastax/proxycore"
	"github.com/antlr4-go/antlr/v4"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func Test_hasWhere(t *testing.T) {
	type args struct {
		query string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "success",
			args: args{
				query: "select * from table where id = 1;",
			},
			want: true,
		},
		{
			name: "no where clause",
			args: args{
				query: "select * from table;",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasWhere(tt.args.query); got != tt.want {
				t.Errorf("hasWhere() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestParseTimestamp tests the parseTimestamp function with various timestamp formats.
func TestParseTimestamp(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected time.Time
		wantErr  bool
	}{
		{
			name:     "ISO 8601 format",
			input:    "2024-02-05T14:00:00Z",
			expected: time.Date(2024, 2, 5, 14, 0, 0, 0, time.UTC),
		},
		{
			name:     "Common date-time format",
			input:    "2024-02-05 14:00:00",
			expected: time.Date(2024, 2, 5, 14, 0, 0, 0, time.UTC),
		},
		{
			name:     "Unix timestamp",
			input:    "1672522562000",
			expected: time.Unix(1672522562, 0).UTC(),
		},
		{
			name:     "Unix timestamp epoch",
			input:    "0",
			expected: time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "Unix timestamp negative",
			input:    "-10000",
			expected: time.Date(1969, 12, 31, 23, 59, 50, 0, time.UTC),
		},
		{
			name:    "Invalid format",
			input:   "invalid-timestamp",
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseTimestamp(tc.input)
			if (err != nil) != tc.wantErr {
				t.Errorf("parseTimestamp() error = %v, wantErr %v", err, tc.wantErr)
				return
			}

			// Allow a small margin of error for floating-point timestamp comparisons
			if !tc.wantErr {
				delta := got.Sub(tc.expected)
				if delta > time.Millisecond || delta < -time.Millisecond {
					t.Errorf("parseTimestamp() = %v, want %v (delta: %v)", got, tc.expected, delta)
				}
			}
		})
	}
}

func TestPrimitivesToString(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected string
		err      bool
	}{
		{"hello", "hello", false},
		{int32(123), "123", false},
		{int(456), "456", false},
		{int64(789), "789", false},
		{float32(1.23), "1.23", false},
		{float64(4.56), "4.56", false},
		{true, "true", false},
		{false, "false", false},
		{complex(1, 1), "", true}, // unsupported type
	}

	for _, test := range tests {
		output, err := primitivesToString(test.input)
		if (err != nil) != test.err {
			t.Errorf("primitivesToString(%v) unexpected error status: %v", test.input, err)
			continue
		}
		if output != test.expected {
			t.Errorf("primitivesToString(%v) = %v; want %v", test.input, output, test.expected)
		}
	}
}

func TestTranslator_GetAllColumns(t *testing.T) {
	type fields struct {
		Logger              *zap.Logger
		SchemaMappingConfig *schemaMapping.SchemaMappingConfig
	}
	type args struct {
		tableName string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []string
		want1   string
		wantErr bool
	}{
		{
			name: "Valid Input",
			fields: fields{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			args: args{
				tableName: "test_table",
			},
			want:    []string{"bigint_col", "blob_col", "bool_col", "column1", "column10", "column2", "column3", "column5", "column6", "column9", "double_col", "float_col", "int_col", "timestamp_col"},
			want1:   "cf1",
			wantErr: false,
		},
		{
			name: "Valid Input",
			fields: fields{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			args: args{
				tableName: "test_table123",
			},
			want:    nil,
			want1:   "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &Translator{
				Logger:              tt.fields.Logger,
				SchemaMappingConfig: tt.fields.SchemaMappingConfig,
			}
			got, got1, err := tr.GetAllColumns(tt.args.tableName, "test_keyspace")
			sort.Strings(got)
			sort.Strings(tt.want)
			if (err != nil) != tt.wantErr {
				t.Errorf("Translator.GetAllColumns() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Translator.GetAllColumns() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("Translator.GetAllColumns() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestStringToPrimitives(t *testing.T) {
	tests := []struct {
		value    string
		cqlType  datatype.DataType
		expected interface{}
		hasError bool
	}{
		{"123", datatype.Int, int32(123), false},
		{"not_an_int", datatype.Int, nil, true},
		{"123456789", datatype.Bigint, int64(123456789), false},
		{"not_a_bigint", datatype.Bigint, nil, true},
		{"3.14", datatype.Float, float32(3.14), false},
		{"not_a_float", datatype.Float, nil, true},
		{"3.1415926535", datatype.Double, float64(3.1415926535), false},
		{"not_a_double", datatype.Double, nil, true},
		{"true", datatype.Boolean, int64(1), false},
		{"false", datatype.Boolean, int64(0), false},
		{"not_a_boolean", datatype.Boolean, nil, true},
		{"blob_data", datatype.Blob, "blob_data", false},
		{"hello", datatype.Varchar, "hello", false},
		{"123", nil, nil, true},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_%s", tt.cqlType, tt.value), func(t *testing.T) {
			result, err := stringToPrimitives(tt.value, tt.cqlType)
			if (err != nil) != tt.hasError {
				t.Errorf("expected error: %v, got error: %v", tt.hasError, err)
			}
			if result != tt.expected {
				t.Errorf("expected result: %v, got result: %v", tt.expected, result)
			}
		})
	}
}

func Test_formatValues(t *testing.T) {
	type args struct {
		value     string
		cqlType   datatype.DataType
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
			args:    args{"abc", datatype.Int, primitive.ProtocolVersion4},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Invalid bigint",
			args:    args{"abc", datatype.Bigint, primitive.ProtocolVersion4},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Invalid float",
			args:    args{"abc", datatype.Float, primitive.ProtocolVersion4},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Invalid double",
			args:    args{"abc", datatype.Double, primitive.ProtocolVersion4},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Invalid boolean",
			args:    args{"abc", datatype.Boolean, primitive.ProtocolVersion4},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Invalid timestamp",
			args:    args{"abc", datatype.Timestamp, primitive.ProtocolVersion4},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Unsupported type",
			args:    args{"123", nil, primitive.ProtocolVersion4},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := formatValues(tt.args.value, tt.args.cqlType, tt.args.protocolV)
			if (err != nil) != tt.wantErr {
				t.Errorf("formatValues() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("formatValues() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_processCollectionColumnsForPrepareQueries(t *testing.T) {
	mapTypeTextText := datatype.NewMapType(datatype.Varchar, datatype.Varchar)
	mapTypeTextBool := datatype.NewMapType(datatype.Varchar, datatype.Boolean)
	mapTypeTextInt := datatype.NewMapType(datatype.Varchar, datatype.Int)
	mapTypeTextFloat := datatype.NewMapType(datatype.Varchar, datatype.Float)
	mapTypeTextDouble := datatype.NewMapType(datatype.Varchar, datatype.Double)
	mapTypeTextTimestamp := datatype.NewMapType(datatype.Varchar, datatype.Timestamp)
	mapTypeTimestampBoolean := datatype.NewMapType(datatype.Timestamp, datatype.Boolean)
	mapTypeTimestampText := datatype.NewMapType(datatype.Timestamp, datatype.Varchar)
	mapTypeTimestampInt := datatype.NewMapType(datatype.Timestamp, datatype.Int)
	mapTypeTimestampFloat := datatype.NewMapType(datatype.Timestamp, datatype.Float)
	mapTypeTimestampBigint := datatype.NewMapType(datatype.Timestamp, datatype.Bigint)
	mapTypeTimestampDouble := datatype.NewMapType(datatype.Timestamp, datatype.Double)
	mapTypeTimestampTimestamp := datatype.NewMapType(datatype.Timestamp, datatype.Timestamp)
	mapTypeTextBigint := datatype.NewMapType(datatype.Varchar, datatype.Bigint)
	setTypeBoolean := datatype.NewSetType(datatype.Boolean)
	setTypeInt := datatype.NewSetType(datatype.Int)
	setTypeBigint := datatype.NewSetType(datatype.Bigint)
	setTypeText := datatype.NewSetType(datatype.Varchar)
	setTypeFloat := datatype.NewSetType(datatype.Float)
	setTypeDouble := datatype.NewSetType(datatype.Double)
	setTypeTimestamp := datatype.NewSetType(datatype.Timestamp)

	valuesTextText := map[string]string{"test": "test"}
	textBytesTextText, _ := proxycore.EncodeType(mapTypeTextText, primitive.ProtocolVersion4, valuesTextText)
	textValue, _ := formatValues("test", datatype.Varchar, primitive.ProtocolVersion4)
	trueVal, _ := formatValues("true", datatype.Boolean, primitive.ProtocolVersion4)

	valuesTextBool := map[string]bool{"test": true}
	textBytesTextBool, _ := proxycore.EncodeType(mapTypeTextBool, primitive.ProtocolVersion4, valuesTextBool)

	valuesTextInt := map[string]int{"test": 42}
	textBytesTextInt, _ := proxycore.EncodeType(mapTypeTextInt, primitive.ProtocolVersion4, valuesTextInt)
	intValue, _ := formatValues("42", datatype.Int, primitive.ProtocolVersion4)

	valuesTextFloat := map[string]float32{"test": 3.14}
	textBytesTextFloat, _ := proxycore.EncodeType(mapTypeTextFloat, primitive.ProtocolVersion4, valuesTextFloat)
	floatValue, _ := formatValues("3.14", datatype.Float, primitive.ProtocolVersion4)

	valuesTextDouble := map[string]float64{"test": 6.283}
	textBytesTextDouble, _ := proxycore.EncodeType(mapTypeTextDouble, primitive.ProtocolVersion4, valuesTextDouble)
	doubleValue, _ := formatValues("6.283", datatype.Double, primitive.ProtocolVersion4)

	valuesTextTimestamp := map[string]time.Time{"test": time.Unix(1633046400, 0)} // Example timestamp
	textBytesTextTimestamp, _ := proxycore.EncodeType(mapTypeTextTimestamp, primitive.ProtocolVersion4, valuesTextTimestamp)
	timestampValue, _ := formatValues("1633046400000", datatype.Timestamp, primitive.ProtocolVersion4) // Example in milliseconds

	valuesTimestampBoolean := map[time.Time]bool{
		time.Unix(1633046400, 0): true,
	}
	textBytesTimestampBoolean, _ := proxycore.EncodeType(mapTypeTimestampBoolean, primitive.ProtocolVersion4, valuesTimestampBoolean)
	timestampBooleanValue, _ := formatValues("true", datatype.Boolean, primitive.ProtocolVersion4)

	valuesTimestampText := map[time.Time]string{
		time.Unix(1633046400, 0): "example_text", // Example timestamp as key with text value
	}
	textBytesTimestampText, _ := proxycore.EncodeType(mapTypeTimestampText, primitive.ProtocolVersion4, valuesTimestampText)
	timestampTextValue, _ := formatValues("example_text", datatype.Varchar, primitive.ProtocolVersion4)

	valuesTimestampInt := map[time.Time]int{
		time.Unix(1633046400, 0): 42, // Example timestamp as key with int value
	}
	textBytesTimestampInt, _ := proxycore.EncodeType(mapTypeTimestampInt, primitive.ProtocolVersion4, valuesTimestampInt)
	timestampIntValue, _ := formatValues("42", datatype.Int, primitive.ProtocolVersion4)

	valuesTimestampFloat := map[time.Time]float32{
		time.Unix(1633046400, 0): 3.14, // Example timestamp as key with float value
	}
	textBytesTimestampFloat, _ := proxycore.EncodeType(mapTypeTimestampFloat, primitive.ProtocolVersion4, valuesTimestampFloat)
	timestampFloatValue, _ := formatValues("3.14", datatype.Float, primitive.ProtocolVersion4)

	valuesTimestampBigint := map[time.Time]int64{
		time.Unix(1633046400, 0): 1234567890123, // Example timestamp as key with bigint value
	}
	textBytesTimestampBigint, _ := proxycore.EncodeType(mapTypeTimestampBigint, primitive.ProtocolVersion4, valuesTimestampBigint)
	timestampBigintValue, _ := formatValues("1234567890123", datatype.Bigint, primitive.ProtocolVersion4)

	valuesTimestampDouble := map[time.Time]float64{
		time.Unix(1633046400, 0): 6.283, // Example timestamp as key with double value
	}
	textBytesTimestampDouble, _ := proxycore.EncodeType(mapTypeTimestampDouble, primitive.ProtocolVersion4, valuesTimestampDouble)
	timestampDoubleValue, _ := formatValues("6.283", datatype.Double, primitive.ProtocolVersion4)

	valuesTimestampTimestamp := map[time.Time]time.Time{
		time.Unix(1633046400, 0): time.Unix(1633126400, 0), // Example timestamp as key with timestamp value
	}
	textBytesTimestampTimestamp, _ := proxycore.EncodeType(mapTypeTimestampTimestamp, primitive.ProtocolVersion4, valuesTimestampTimestamp)
	timestampTimestampValue, _ := formatValues("1633126400000", datatype.Timestamp, primitive.ProtocolVersion4) // Example in milliseconds

	valuesTextBigint := map[string]int64{"test": 1234567890123}
	textBytesTextBigint, _ := proxycore.EncodeType(mapTypeTextBigint, primitive.ProtocolVersion4, valuesTextBigint)
	bigintValue, _ := formatValues("1234567890123", datatype.Bigint, primitive.ProtocolVersion4)

	valuesSetBoolean := []bool{true}
	valuesSetInt := []int32{12}
	valuesSetBigInt := []int64{12372432764}
	valuesSetText := []string{"test"}
	valuesSetFloat := []float32{6.283}
	valuesSetDouble := []float64{6.283}
	valuesSetTimestamp := []int64{1633046400}

	setBytesBoolean, _ := proxycore.EncodeType(setTypeBoolean, primitive.ProtocolVersion4, valuesSetBoolean)
	setBytesInt, _ := proxycore.EncodeType(setTypeInt, primitive.ProtocolVersion4, valuesSetInt)
	setBytesBigInt, _ := proxycore.EncodeType(setTypeBigint, primitive.ProtocolVersion4, valuesSetBigInt)
	setBytesText, _ := proxycore.EncodeType(setTypeText, primitive.ProtocolVersion4, valuesSetText)
	setBytesFloat, _ := proxycore.EncodeType(setTypeFloat, primitive.ProtocolVersion4, valuesSetFloat)
	setBytesDouble, _ := proxycore.EncodeType(setTypeDouble, primitive.ProtocolVersion4, valuesSetDouble)
	setBytesTimestamp, _ := proxycore.EncodeType(setTypeTimestamp, primitive.ProtocolVersion4, valuesSetTimestamp)

	emptyVal, _ := formatValues("", datatype.Varchar, primitive.ProtocolVersion4)
	listTextType := datatype.NewListType(datatype.Varchar)
	valuesListText := []string{"test"}
	listBytesText, _ := proxycore.EncodeType(listTextType, primitive.ProtocolVersion4, valuesListText)

	listIntType := datatype.NewListType(datatype.Int)
	valuesListInt := []int32{42}
	listBytesInt, _ := proxycore.EncodeType(listIntType, primitive.ProtocolVersion4, valuesListInt)

	listBigintType := datatype.NewListType(datatype.Bigint)
	valuesListBigint := []int64{1234567890123}
	listBytesBigint, _ := proxycore.EncodeType(listBigintType, primitive.ProtocolVersion4, valuesListBigint)

	listBoolType := datatype.NewListType(datatype.Boolean)
	valuesListBool := []bool{true}
	listBytesBool, _ := proxycore.EncodeType(listBoolType, primitive.ProtocolVersion4, valuesListBool)

	listDoubleType := datatype.NewListType(datatype.Double)
	valuesListDouble := []float64{6.283}
	listBytesDouble, _ := proxycore.EncodeType(listDoubleType, primitive.ProtocolVersion4, valuesListDouble)

	listFloatType := datatype.NewListType(datatype.Float)
	valuesListFloat := []float32{3.14}
	listBytesFloat, _ := proxycore.EncodeType(listFloatType, primitive.ProtocolVersion4, valuesListFloat)

	listTimestampType := datatype.NewListType(datatype.Timestamp)
	valuesListTimestamp := []int64{1633046400000}
	listBytesTimestamp, _ := proxycore.EncodeType(listTimestampType, primitive.ProtocolVersion4, valuesListTimestamp)

	floatVal, _ := formatValues("3.14", datatype.Float, primitive.ProtocolVersion4)
	doubleVal, _ := formatValues("6.283", datatype.Double, primitive.ProtocolVersion4)
	timestampVal, _ := formatValues("1633046400000", datatype.Timestamp, primitive.ProtocolVersion4)

	tests := []struct {
		name             string
		columns          []types.Column
		variableMetadata []*message.ColumnMetadata
		values           []*primitive.Value
		tableName        string
		protocolV        primitive.ProtocolVersion
		primaryKeys      []string
		translator       *Translator
		want             []types.Column
		want1            []interface{}
		want2            map[string]interface{}
		want3            int
		want4            []string
		wantErr          bool
	}{
		{
			name: "Valid Input For Timestamp Float",
			columns: []types.Column{
				{Name: "map_timestamp_float", ColumnFamily: "map_timestamp_float", CQLType: datatype.NewMapType(datatype.Timestamp, datatype.Float)},
			},
			variableMetadata: []*message.ColumnMetadata{},
			values: []*primitive.Value{
				{Contents: textBytesTimestampFloat},
			},

			tableName:   "non_primitive_table",
			protocolV:   primitive.ProtocolVersion4,
			primaryKeys: []string{},
			translator: &Translator{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			want: []types.Column{
				{Name: "1633046400000", ColumnFamily: "map_timestamp_float", CQLType: datatype.Float},
			},
			want1:   []interface{}{timestampFloatValue},
			want2:   map[string]interface{}{},
			want3:   0,
			want4:   []string{"map_timestamp_float"},
			wantErr: false,
		},
		{
			name: "Valid Input For Text Timestamp",
			columns: []types.Column{
				{Name: "map_text_timestamp", ColumnFamily: "map_text_timestamp", CQLType: datatype.NewMapType(datatype.Varchar, datatype.Timestamp)},
			},
			variableMetadata: []*message.ColumnMetadata{},
			values: []*primitive.Value{
				{Contents: textBytesTextTimestamp},
			},
			tableName:   "non_primitive_table",
			protocolV:   primitive.ProtocolVersion4,
			primaryKeys: []string{},
			translator: &Translator{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			want: []types.Column{
				{Name: "test", ColumnFamily: "map_text_timestamp", CQLType: datatype.Timestamp},
			},
			want1:   []interface{}{timestampValue},
			want2:   map[string]interface{}{},
			want3:   0,
			want4:   []string{"map_text_timestamp"},
			wantErr: false,
		},
		{
			name: "Valid Input For Timestamp Text",
			columns: []types.Column{
				{Name: "map_timestamp_text", ColumnFamily: "map_timestamp_text", CQLType: datatype.NewMapType(datatype.Timestamp, datatype.Varchar)},
			},
			variableMetadata: []*message.ColumnMetadata{},
			values: []*primitive.Value{
				{Contents: textBytesTimestampText},
			},
			tableName:   "non_primitive_table",
			protocolV:   primitive.ProtocolVersion4,
			primaryKeys: []string{},
			translator: &Translator{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			want: []types.Column{
				{Name: "1633046400000", ColumnFamily: "map_timestamp_text", CQLType: datatype.Varchar},
			},
			want1:   []interface{}{timestampTextValue},
			want2:   map[string]interface{}{},
			want3:   0,
			want4:   []string{"map_timestamp_text"},
			wantErr: false,
		},
		{
			name: "Valid Input For Set Timestamp",
			columns: []types.Column{
				{Name: "set_timestamp", ColumnFamily: "set_timestamp", CQLType: datatype.NewSetType(datatype.Timestamp)},
			},
			variableMetadata: []*message.ColumnMetadata{},
			values: []*primitive.Value{
				{Contents: setBytesTimestamp},
			},
			tableName:   "non_primitive_table",
			protocolV:   primitive.ProtocolVersion4,
			primaryKeys: []string{},
			translator: &Translator{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			want: []types.Column{
				{Name: "1633046400", ColumnFamily: "set_timestamp", CQLType: datatype.Bigint},
			},
			want1:   []interface{}{emptyVal},
			want2:   map[string]interface{}{},
			want3:   0,
			want4:   []string{"set_timestamp"},
			wantErr: false,
		},
		{
			name: "Valid Input For Set Double",
			columns: []types.Column{
				{Name: "set_double", ColumnFamily: "set_double", CQLType: datatype.NewSetType(datatype.Double)},
			},
			variableMetadata: []*message.ColumnMetadata{},
			values: []*primitive.Value{
				{Contents: setBytesDouble},
			},
			tableName:   "non_primitive_table",
			protocolV:   primitive.ProtocolVersion4,
			primaryKeys: []string{},
			translator: &Translator{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			want: []types.Column{
				{Name: "6.283", ColumnFamily: "set_double", CQLType: datatype.Double},
			},
			want1:   []interface{}{emptyVal},
			want2:   map[string]interface{}{},
			want3:   0,
			want4:   []string{"set_double"},
			wantErr: false,
		},
		{
			name: "Valid Input For Set Float",
			columns: []types.Column{
				{Name: "set_float", ColumnFamily: "set_float", CQLType: datatype.NewSetType(datatype.Float)},
			},
			variableMetadata: []*message.ColumnMetadata{},
			values: []*primitive.Value{
				{Contents: setBytesFloat},
			},
			tableName:   "non_primitive_table",
			protocolV:   primitive.ProtocolVersion4,
			primaryKeys: []string{},
			translator: &Translator{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			want: []types.Column{
				{Name: "6.283", ColumnFamily: "set_float", CQLType: datatype.Float},
			},
			want1:   []interface{}{emptyVal},
			want2:   map[string]interface{}{},
			want3:   0,
			want4:   []string{"set_float"},
			wantErr: false,
		},
		{
			name: "Valid Input For Set Text",
			columns: []types.Column{
				{Name: "set_text", ColumnFamily: "set_text", CQLType: datatype.NewSetType(datatype.Varchar)},
			},
			variableMetadata: []*message.ColumnMetadata{},
			values: []*primitive.Value{
				{Contents: setBytesText},
			},
			tableName:   "non_primitive_table",
			protocolV:   primitive.ProtocolVersion4,
			primaryKeys: []string{},
			translator: &Translator{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			want: []types.Column{
				{Name: "test", ColumnFamily: "set_text", CQLType: datatype.Varchar},
			},
			want1:   []interface{}{emptyVal},
			want2:   map[string]interface{}{},
			want3:   0,
			want4:   []string{"set_text"},
			wantErr: false,
		},
		{
			name: "Valid Input For Set BigInt",
			columns: []types.Column{
				{Name: "set_bigint", ColumnFamily: "set_bigint", CQLType: datatype.NewSetType(datatype.Bigint)},
			},
			variableMetadata: []*message.ColumnMetadata{},
			values: []*primitive.Value{
				{Contents: setBytesBigInt},
			},
			tableName:   "non_primitive_table",
			protocolV:   primitive.ProtocolVersion4,
			primaryKeys: []string{},
			translator: &Translator{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			want: []types.Column{
				{Name: "12372432764", ColumnFamily: "set_bigint", CQLType: datatype.Bigint},
			},
			want1:   []interface{}{emptyVal},
			want2:   map[string]interface{}{},
			want3:   0,
			want4:   []string{"set_bigint"},
			wantErr: false,
		},
		{
			name: "Valid Input For Set Int",
			columns: []types.Column{
				{Name: "set_int", ColumnFamily: "set_int", CQLType: datatype.NewSetType(datatype.Int)},
			},
			variableMetadata: []*message.ColumnMetadata{},
			values: []*primitive.Value{
				{Contents: setBytesInt},
			},
			tableName:   "non_primitive_table",
			protocolV:   primitive.ProtocolVersion4,
			primaryKeys: []string{},
			translator: &Translator{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			want: []types.Column{
				{Name: "12", ColumnFamily: "set_int", CQLType: datatype.Int},
			},
			want1:   []interface{}{emptyVal},
			want2:   map[string]interface{}{},
			want3:   0,
			want4:   []string{"set_int"},
			wantErr: false,
		},
		{
			name: "Valid Input For Timestamp Timestamp",
			columns: []types.Column{
				{Name: "map_timestamp_timestamp", ColumnFamily: "map_timestamp_timestamp", CQLType: datatype.NewMapType(datatype.Timestamp, datatype.Timestamp)},
			},
			variableMetadata: []*message.ColumnMetadata{},
			values: []*primitive.Value{
				{Contents: textBytesTimestampTimestamp},
			},
			tableName:   "non_primitive_table",
			protocolV:   primitive.ProtocolVersion4,
			primaryKeys: []string{},
			translator: &Translator{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			want: []types.Column{
				{Name: "1633046400000", ColumnFamily: "map_timestamp_timestamp", CQLType: datatype.Timestamp},
			},
			want1:   []interface{}{timestampTimestampValue},
			want2:   map[string]interface{}{},
			want3:   0,
			want4:   []string{"map_timestamp_timestamp"},
			wantErr: false,
		},
		{
			name: "Valid Input For Text Bigint",
			columns: []types.Column{
				{Name: "map_text_bigint", ColumnFamily: "map_text_bigint", CQLType: datatype.NewMapType(datatype.Varchar, datatype.Bigint)},
			},
			variableMetadata: []*message.ColumnMetadata{},
			values: []*primitive.Value{
				{Contents: textBytesTextBigint},
			},
			tableName:   "non_primitive_table",
			protocolV:   primitive.ProtocolVersion4,
			primaryKeys: []string{},
			translator: &Translator{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			want: []types.Column{
				{Name: "test", ColumnFamily: "map_text_bigint", CQLType: datatype.Bigint},
			},
			want1:   []interface{}{bigintValue},
			want2:   map[string]interface{}{},
			want3:   0,
			want4:   []string{"map_text_bigint"},
			wantErr: false,
		},
		{
			name: "Valid Input For Timestamp Int",
			columns: []types.Column{
				{Name: "map_timestamp_int", ColumnFamily: "map_timestamp_int", CQLType: datatype.NewMapType(datatype.Timestamp, datatype.Int)},
			},
			variableMetadata: []*message.ColumnMetadata{},
			values: []*primitive.Value{
				{Contents: textBytesTimestampInt},
			},
			tableName:   "non_primitive_table",
			protocolV:   primitive.ProtocolVersion4,
			primaryKeys: []string{},
			translator: &Translator{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			want: []types.Column{
				{Name: "1633046400000", ColumnFamily: "map_timestamp_int", CQLType: datatype.Int},
			},
			want1:   []interface{}{timestampIntValue},
			want2:   map[string]interface{}{},
			want3:   0,
			want4:   []string{"map_timestamp_int"},
			wantErr: false,
		},
		{
			name: "Valid Input For Set Boolean",
			columns: []types.Column{
				{Name: "set_boolean", ColumnFamily: "set_boolean", CQLType: datatype.NewSetType(datatype.Boolean)},
			},
			variableMetadata: []*message.ColumnMetadata{},
			values: []*primitive.Value{
				{Contents: setBytesBoolean},
			},
			tableName:   "non_primitive_table",
			protocolV:   primitive.ProtocolVersion4,
			primaryKeys: []string{},
			translator: &Translator{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			want: []types.Column{
				{Name: "1", ColumnFamily: "set_boolean", CQLType: datatype.Boolean},
			},
			want1:   []interface{}{emptyVal},
			want2:   map[string]interface{}{},
			want3:   0,
			want4:   []string{"set_boolean"},
			wantErr: false,
		},
		{
			name: "Valid Input For Text Boolean",
			columns: []types.Column{
				{Name: "map_text_boolean", ColumnFamily: "map_text_boolean", CQLType: datatype.NewMapType(datatype.Varchar, datatype.Boolean)},
			},
			variableMetadata: []*message.ColumnMetadata{},
			values: []*primitive.Value{
				{Contents: textBytesTextBool},
			},
			tableName:   "non_primitive_table",
			protocolV:   primitive.ProtocolVersion4,
			primaryKeys: []string{},
			translator: &Translator{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			want: []types.Column{
				{Name: "test", ColumnFamily: "map_text_boolean", CQLType: datatype.Boolean},
			},
			want1:   []interface{}{trueVal},
			want2:   map[string]interface{}{},
			want3:   0,
			want4:   []string{"map_text_boolean"},
			wantErr: false,
		},
		{
			name: "Valid Input For Text Text",
			columns: []types.Column{
				{Name: "map_text_text", ColumnFamily: "map_text_text", CQLType: datatype.NewMapType(datatype.Varchar, datatype.Varchar)},
			},
			variableMetadata: []*message.ColumnMetadata{},
			values: []*primitive.Value{
				{Contents: textBytesTextText},
			},
			tableName:   "non_primitive_table",
			protocolV:   primitive.ProtocolVersion4,
			primaryKeys: []string{},
			translator: &Translator{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			want: []types.Column{
				{Name: "test", ColumnFamily: "map_text_text", CQLType: datatype.Varchar},
			},
			want1:   []interface{}{textValue},
			want2:   map[string]interface{}{},
			want3:   0,
			want4:   []string{"map_text_text"},
			wantErr: false,
		},
		{
			name: "Valid Input For Text Int",
			columns: []types.Column{
				{Name: "map_text_int", ColumnFamily: "map_text_int", CQLType: datatype.NewMapType(datatype.Varchar, datatype.Int)},
			},
			variableMetadata: []*message.ColumnMetadata{},
			values: []*primitive.Value{
				{Contents: textBytesTextInt},
			},
			tableName:   "non_primitive_table",
			protocolV:   primitive.ProtocolVersion4,
			primaryKeys: []string{},
			translator: &Translator{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			want: []types.Column{
				{Name: "test", ColumnFamily: "map_text_int", CQLType: datatype.Int},
			},
			want1:   []interface{}{intValue},
			want2:   map[string]interface{}{},
			want3:   0,
			want4:   []string{"map_text_int"},
			wantErr: false,
		},
		{
			name: "Valid Input For Text Float",
			columns: []types.Column{
				{Name: "map_text_float", ColumnFamily: "map_text_float", CQLType: datatype.NewMapType(datatype.Varchar, datatype.Float)},
			},
			variableMetadata: []*message.ColumnMetadata{},
			values: []*primitive.Value{
				{Contents: textBytesTextFloat},
			},
			tableName:   "non_primitive_table",
			protocolV:   primitive.ProtocolVersion4,
			primaryKeys: []string{},
			translator: &Translator{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			want: []types.Column{
				{Name: "test", ColumnFamily: "map_text_float", CQLType: datatype.Float},
			},
			want1:   []interface{}{floatValue},
			want2:   map[string]interface{}{},
			want3:   0,
			want4:   []string{"map_text_float"},
			wantErr: false,
		},
		{
			name: "Valid Input For Text Double",
			columns: []types.Column{
				{Name: "map_text_double", ColumnFamily: "map_text_double", CQLType: datatype.NewMapType(datatype.Varchar, datatype.Double)},
			},
			variableMetadata: []*message.ColumnMetadata{},
			values: []*primitive.Value{
				{Contents: textBytesTextDouble},
			},
			tableName:   "non_primitive_table",
			protocolV:   primitive.ProtocolVersion4,
			primaryKeys: []string{},
			translator: &Translator{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			want: []types.Column{
				{Name: "test", ColumnFamily: "map_text_double", CQLType: datatype.Double},
			},
			want1:   []interface{}{doubleValue},
			want2:   map[string]interface{}{},
			want3:   0,
			want4:   []string{"map_text_double"},
			wantErr: false,
		},
		{
			name: "Valid Input For Timestamp Boolean",
			columns: []types.Column{
				{Name: "map_timestamp_boolean", ColumnFamily: "map_timestamp_boolean", CQLType: datatype.NewMapType(datatype.Timestamp, datatype.Boolean)},
			},
			variableMetadata: []*message.ColumnMetadata{},
			values: []*primitive.Value{
				{Contents: textBytesTimestampBoolean},
			},
			tableName:   "non_primitive_table",
			protocolV:   primitive.ProtocolVersion4,
			primaryKeys: []string{},
			translator: &Translator{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			want: []types.Column{
				{Name: "1633046400000", ColumnFamily: "map_timestamp_boolean", CQLType: datatype.Boolean},
			},
			want1:   []interface{}{timestampBooleanValue},
			want2:   map[string]interface{}{},
			want3:   0,
			want4:   []string{"map_timestamp_boolean"},
			wantErr: false,
		},
		{
			name: "Valid Input For Timestamp Double",
			columns: []types.Column{
				{Name: "map_timestamp_double", ColumnFamily: "map_timestamp_double", CQLType: datatype.NewMapType(datatype.Timestamp, datatype.Double)},
			},
			variableMetadata: []*message.ColumnMetadata{},
			values: []*primitive.Value{
				{Contents: textBytesTimestampDouble},
			},
			tableName:   "non_primitive_table",
			protocolV:   primitive.ProtocolVersion4,
			primaryKeys: []string{},
			translator: &Translator{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			want: []types.Column{
				{Name: "1633046400000", ColumnFamily: "map_timestamp_double", CQLType: datatype.Double},
			},
			want1:   []interface{}{timestampDoubleValue},
			want2:   map[string]interface{}{},
			want3:   0,
			want4:   []string{"map_timestamp_double"},
			wantErr: false,
		},
		{
			name: "Valid Input For Timestamp Bigint",
			columns: []types.Column{
				{Name: "map_timestamp_bigint", ColumnFamily: "map_timestamp_bigint", CQLType: datatype.NewMapType(datatype.Timestamp, datatype.Bigint)},
			},
			variableMetadata: []*message.ColumnMetadata{},
			values: []*primitive.Value{
				{Contents: textBytesTimestampBigint},
			},
			tableName:   "non_primitive_table",
			protocolV:   primitive.ProtocolVersion4,
			primaryKeys: []string{},
			translator: &Translator{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			want: []types.Column{
				{Name: "1633046400000", ColumnFamily: "map_timestamp_bigint", CQLType: datatype.Bigint},
			},
			want1:   []interface{}{timestampBigintValue},
			want2:   map[string]interface{}{},
			want3:   0,
			want4:   []string{"map_timestamp_bigint"},
			wantErr: false,
		},
		{
			name: "Valid Input For List<text>",
			columns: []types.Column{
				{Name: "list_text", ColumnFamily: "list_text", CQLType: datatype.NewListType(datatype.Varchar)},
			},
			values: []*primitive.Value{
				{Contents: listBytesText},
			},
			tableName:   "non_primitive_table",
			protocolV:   primitive.ProtocolVersion4,
			primaryKeys: []string{},
			translator: &Translator{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			want: []types.Column{
				{Name: time.Now().Format("20060102150405.000"), ColumnFamily: "list_text", CQLType: datatype.Varchar},
			},
			want1:   []interface{}{textValue},
			want2:   map[string]interface{}{},
			want3:   0,
			want4:   []string{"list_text"},
			wantErr: false,
		},
		{
			name: "Valid Input For List<int>",
			columns: []types.Column{
				{Name: "list_int", ColumnFamily: "list_int", CQLType: datatype.NewListType(datatype.Int)},
			},
			values: []*primitive.Value{
				{Contents: listBytesInt},
			},
			tableName:   "non_primitive_table",
			protocolV:   primitive.ProtocolVersion4,
			primaryKeys: []string{},
			translator: &Translator{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			want: []types.Column{
				{Name: time.Now().Format("20060102150405.000"), ColumnFamily: "list_int", CQLType: datatype.Int},
			},
			want1:   []interface{}{intValue},
			want2:   map[string]interface{}{},
			want3:   0,
			want4:   []string{"list_int"},
			wantErr: false,
		},
		{
			name: "Valid Input For List<bigint>",
			columns: []types.Column{
				{Name: "list_bigint", ColumnFamily: "list_bigint", CQLType: datatype.NewListType(datatype.Bigint)},
			},
			values: []*primitive.Value{
				{Contents: listBytesBigint},
			},
			tableName:   "non_primitive_table",
			protocolV:   primitive.ProtocolVersion4,
			primaryKeys: []string{},
			translator: &Translator{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			want: []types.Column{
				{Name: time.Now().Format("20060102150405.000"), ColumnFamily: "list_bigint", CQLType: datatype.Bigint},
			},
			want1:   []interface{}{bigintValue},
			want2:   map[string]interface{}{},
			want3:   0,
			want4:   []string{"list_bigint"},
			wantErr: false,
		},
		{
			name: "Valid Input For List<boolean>",
			columns: []types.Column{
				{Name: "list_boolean", ColumnFamily: "list_boolean", CQLType: datatype.NewListType(datatype.Boolean)},
			},
			values: []*primitive.Value{
				{Contents: listBytesBool},
			},
			tableName:   "non_primitive_table",
			protocolV:   primitive.ProtocolVersion4,
			primaryKeys: []string{},
			translator: &Translator{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			want: []types.Column{
				{Name: time.Now().Format("20060102150405.000"), ColumnFamily: "list_boolean", CQLType: datatype.Boolean},
			},
			want1:   []interface{}{trueVal},
			want2:   map[string]interface{}{},
			want3:   0,
			want4:   []string{"list_boolean"},
			wantErr: false,
		},
		{
			name: "Valid Input For List<float>",
			columns: []types.Column{
				{Name: "list_float", ColumnFamily: "list_float", CQLType: datatype.NewListType(datatype.Float)},
			},
			values: []*primitive.Value{
				{Contents: listBytesFloat},
			},
			tableName:   "non_primitive_table",
			protocolV:   primitive.ProtocolVersion4,
			primaryKeys: []string{},
			translator: &Translator{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			want: []types.Column{
				{Name: time.Now().Format("20060102150405.000"), ColumnFamily: "list_float", CQLType: datatype.Float},
			},
			want1:   []interface{}{floatVal},
			want2:   map[string]interface{}{},
			want3:   0,
			want4:   []string{"list_float"},
			wantErr: false,
		},
		{
			name: "Valid Input For List<double>",
			columns: []types.Column{
				{Name: "list_double", ColumnFamily: "list_double", CQLType: datatype.NewListType(datatype.Double)},
			},
			values: []*primitive.Value{
				{Contents: listBytesDouble},
			},
			tableName:   "non_primitive_table",
			protocolV:   primitive.ProtocolVersion4,
			primaryKeys: []string{},
			translator: &Translator{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			want: []types.Column{
				{Name: time.Now().Format("20060102150405.000"), ColumnFamily: "list_double", CQLType: datatype.Double},
			},
			want1:   []interface{}{doubleVal},
			want2:   map[string]interface{}{},
			want3:   0,
			want4:   []string{"list_double"},
			wantErr: false,
		},
		{
			name: "Valid Input For List<timestamp>",
			columns: []types.Column{
				{Name: "list_timestamp", ColumnFamily: "list_timestamp", CQLType: datatype.NewListType(datatype.Timestamp)},
			},
			values: []*primitive.Value{
				{Contents: listBytesTimestamp},
			},
			tableName:   "non_primitive_table",
			protocolV:   primitive.ProtocolVersion4,
			primaryKeys: []string{},
			translator: &Translator{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			want: []types.Column{
				{Name: time.Now().Format("20060102150405.000"), ColumnFamily: "list_timestamp", CQLType: datatype.Bigint},
			},
			want1:   []interface{}{timestampVal},
			want2:   map[string]interface{}{},
			want3:   0,
			want4:   []string{"list_timestamp"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := ProcessPrepareCollectionsInput{
				ColumnsResponse: tt.columns,
				Values:          tt.values,
				TableName:       tt.tableName,
				ProtocolV:       tt.protocolV,
				PrimaryKeys:     tt.primaryKeys,
				Translator:      tt.translator,
				KeySpace:        "test_keyspace",
				ComplexMeta:     nil, // Assuming nil for these tests, adjust if needed
			}
			output, err := processCollectionColumnsForPrepareQueries(input)

			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return // Don't check results if an error was expected
			}
			// For list types, normalize Names for comparison as its a timestamp value based on time.Now()
			if strings.Contains(tt.name, "List") {
				// Normalize both output and expected Names for comparison
				for i := range output.NewColumns {
					output.NewColumns[i].Name = fmt.Sprintf("list_index_%d", i)
				}
				for i := range tt.want {
					tt.want[i].Name = fmt.Sprintf("list_index_%d", i)
				}
			}

			if !reflect.DeepEqual(output.NewColumns, tt.want) {
				t.Errorf("output.NewColumns = %v, want %v", output.NewColumns, tt.want)
			}
			if !reflect.DeepEqual(output.NewValues, tt.want1) {
				t.Errorf("output.NewValues = %v, want %v", output.NewValues, tt.want1)
			}
			if !reflect.DeepEqual(output.Unencrypted, tt.want2) {
				t.Errorf("output.Unencrypted = %v, want %v", output.Unencrypted, tt.want2)
			}
			if output.IndexEnd != tt.want3 {
				t.Errorf("processCollectionColumnsForPrepareQueries() output.IndexEnd = %v, want %v", output.IndexEnd, tt.want3)
			}
			if !reflect.DeepEqual(output.DelColumnFamily, tt.want4) {
				t.Errorf("processCollectionColumnsForPrepareQueries() output.DelColumnFamily = %v, want %v", output.DelColumnFamily, tt.want4)
			}
		})
	}
}

func TestConvertToBigtableTimestamp(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    TimestampInfo
		expectError bool
	}{
		{
			name:  "Valid timestamp input in nano second",
			input: "1634232345000000",
			expected: TimestampInfo{
				Timestamp:         bigtable.Time(time.Unix(1634232345, 0)),
				HasUsingTimestamp: true,
				Index:             0,
			},
			expectError: false,
		},
		{
			name:  "Valid timestamp input in micro second",
			input: "1634232345000",
			expected: TimestampInfo{
				Timestamp:         bigtable.Time(time.Unix(1634232345, 0)),
				HasUsingTimestamp: true,
				Index:             0,
			},
			expectError: false,
		},
		{
			name:  "Valid timestamp input in seconds",
			input: "1634232345",
			expected: TimestampInfo{
				Timestamp:         bigtable.Time(time.Unix(1634232345, 0)),
				HasUsingTimestamp: true,
				Index:             0,
			},
			expectError: false,
		},
		{
			name:  "Empty input",
			input: "",
			expected: TimestampInfo{
				Timestamp:         bigtable.Timestamp(0),
				HasUsingTimestamp: true,
				Index:             0,
			},
			expectError: false,
		},
		{
			name:  "Input contains question mark",
			input: "1634232345?",
			expected: TimestampInfo{
				Timestamp:         bigtable.Timestamp(0),
				HasUsingTimestamp: true,
				Index:             0,
			},
			expectError: false,
		},
		{
			name:        "Invalid input",
			input:       "invalid",
			expected:    TimestampInfo{},
			expectError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := convertToBigtableTimestamp(test.input, 0)

			if (err != nil) != test.expectError {
				t.Errorf("Unexpected error status: got %v, expected error %v", err, test.expectError)
			}

			if !test.expectError && result != test.expected {
				t.Errorf("Unexpected result: got %+v, expected %+v", result, test.expected)
			}
		})
	}
}

func Test_validateRequiredPrimaryKeys(t *testing.T) {
	type args struct {
		requiredKey []string
		actualKey   []string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Equal slices with different order",
			args: args{
				requiredKey: []string{"key1", "key2", "key3"},
				actualKey:   []string{"key3", "key2", "key1"},
			},
			want: true,
		},
		{
			name: "Equal slices with same order",
			args: args{
				requiredKey: []string{"key1", "key2", "key3"},
				actualKey:   []string{"key1", "key2", "key3"},
			},
			want: true,
		},
		{
			name: "Unequal slices with different elements",
			args: args{
				requiredKey: []string{"key1", "key2", "key3"},
				actualKey:   []string{"key1", "key4", "key3"},
			},
			want: false,
		},
		{
			name: "Unequal slices with different lengths",
			args: args{
				requiredKey: []string{"key1", "key2"},
				actualKey:   []string{"key1", "key2", "key3"},
			},
			want: false,
		},
		{
			name: "Both slices empty",
			args: args{
				requiredKey: []string{},
				actualKey:   []string{},
			},
			want: true,
		},
		{
			name: "One slice empty, one not",
			args: args{
				requiredKey: []string{"key1"},
				actualKey:   []string{},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateRequiredPrimaryKeys(tt.args.requiredKey, tt.args.actualKey); got != tt.want {
				t.Errorf("validateRequiredPrimaryKeys() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProcessComplexUpdate(t *testing.T) {
	translator := &Translator{
		SchemaMappingConfig: GetSchemaMappingConfig(),
	}

	tests := []struct {
		name           string
		columns        []types.Column
		values         []interface{}
		prependColumns []string
		expectedMeta   map[string]*ComplexOperation
		expectedErr    error
	}{
		{
			name: "successful collection update for map and list",
			columns: []types.Column{
				{Name: "map_text_bool_col", CQLType: datatype.NewMapType(datatype.Varchar, datatype.Boolean)},
				{Name: "list_text", CQLType: datatype.NewListType(datatype.Varchar)},
			},
			values: []interface{}{
				ComplexAssignment{
					Left:      "map_text_bool_col",
					Operation: "+",
					Right:     "{key:?}",
				},
				ComplexAssignment{
					Left:      "list_text",
					Operation: "+",
					Right:     "?",
				},
			},
			prependColumns: []string{"list_text"},
			expectedMeta: map[string]*ComplexOperation{
				"map_text_bool_col": {
					Append: true,
				},
				"list_text": {
					Append:      true,
					PrependList: false,
				},
			},
			expectedErr: nil,
		},
		{
			name: "non-collection column should be skipped",
			columns: []types.Column{
				{Name: "pk_1_text", CQLType: datatype.Varchar},
			},
			values: []interface{}{
				"pk_1_text+value",
			},
			prependColumns: []string{},
			expectedMeta:   map[string]*ComplexOperation{},
			expectedErr:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			complexMeta, err := translator.ProcessComplexUpdate(tt.columns, tt.values, "test_table", "test_keyspace", tt.prependColumns)

			if err != tt.expectedErr {
				t.Errorf("expected error %v, got %v", tt.expectedErr, err)
			}

			if len(complexMeta) != len(tt.expectedMeta) {
				t.Errorf("expected length %d, got %d", len(tt.expectedMeta), len(complexMeta))
			}

			for key, expectedComplexUpdate := range tt.expectedMeta {
				actualComplexUpdate, exists := complexMeta[key]
				if !exists {
					t.Errorf("expected key %s to exist in result", key)
				} else {
					if !compareComplexOperation(expectedComplexUpdate, actualComplexUpdate) {
						t.Errorf("expected meta for key %s: %v, got: %v", key, expectedComplexUpdate, actualComplexUpdate)
					}
				}
			}
		})
	}
}

func TestExtractWritetimeValue(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name  string
		args  args
		want  string
		want1 bool
	}{
		{
			name:  "Valid writetime statement",
			args:  args{s: "writetime(column)"},
			want:  "column",
			want1: true,
		},
		{
			name:  "Invalid missing closing parenthesis",
			args:  args{s: "writetime(column"},
			want:  "",
			want1: false,
		},
		{
			name:  "Invalid missing opening parenthesis",
			args:  args{s: "writetime)"},
			want:  "",
			want1: false,
		},
		{
			name:  "Completely invalid string",
			args:  args{s: "some random string"},
			want:  "",
			want1: false,
		},
		{
			name:  "Empty string",
			args:  args{s: ""},
			want:  "",
			want1: false,
		},
		{
			name:  "Case insensitivity",
			args:  args{s: "WriteTime(test)"},
			want:  "test",
			want1: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := ExtractWritetimeValue(tt.args.s)
			if got != tt.want {
				t.Errorf("ExtractWritetimeValue() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("ExtractWritetimeValue() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestCastColumns(t *testing.T) {
	tests := []struct {
		name         string
		colMeta      *types.Column
		columnFamily string
		want         string
		wantErr      bool
	}{
		{
			name: "integer type",
			colMeta: &types.Column{
				ColumnName: "age",
				CQLType:    datatype.Int,
			},
			columnFamily: "cf1",
			want:         "TO_INT64(cf1['age'])",
			wantErr:      false,
		},
		{
			name: "bigint type",
			colMeta: &types.Column{
				ColumnName: "timestamp",
				CQLType:    datatype.Bigint,
			},
			columnFamily: "cf1",
			want:         "TO_INT64(cf1['timestamp'])",
			wantErr:      false,
		},
		{
			name: "float type",
			colMeta: &types.Column{
				ColumnName: "price",
				CQLType:    datatype.Float,
			},
			columnFamily: "cf1",
			want:         "TO_FLOAT32(cf1['price'])",
			wantErr:      false,
		},
		{
			name: "double type",
			colMeta: &types.Column{
				ColumnName: "value",
				CQLType:    datatype.Double,
			},
			columnFamily: "cf1",
			want:         "TO_FLOAT64(cf1['value'])",
			wantErr:      false,
		},
		{
			name: "boolean type",
			colMeta: &types.Column{
				ColumnName: "active",
				CQLType:    datatype.Boolean,
			},
			columnFamily: "cf1",
			want:         "TO_INT64(cf1['active'])",
			wantErr:      false,
		},
		{
			name: "timestamp type",
			colMeta: &types.Column{
				ColumnName: "created_at",
				CQLType:    datatype.Timestamp,
			},
			columnFamily: "cf1",
			want:         "TO_TIME(cf1['created_at'])",
			wantErr:      false,
		},
		{
			name: "blob type",
			colMeta: &types.Column{
				ColumnName: "data",
				CQLType:    datatype.Blob,
			},
			columnFamily: "cf1",
			want:         "TO_BLOB(cf1['data'])",
			wantErr:      false,
		},
		{
			name: "text type",
			colMeta: &types.Column{
				ColumnName: "name",
				CQLType:    datatype.Varchar,
			},
			columnFamily: "cf1",
			want:         "cf1['name']",
			wantErr:      false,
		},
		{
			name: "unsupported type",
			colMeta: &types.Column{
				ColumnName: "unsupported",
				CQLType:    nil,
			},
			columnFamily: "cf1",
			want:         "",
			wantErr:      true,
		},
		{
			name: "handle special characters in column name",
			colMeta: &types.Column{
				ColumnName: "special-name",
				CQLType:    datatype.Varchar,
			},
			columnFamily: "cf1",
			want:         "cf1['special-name']",
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := castColumns(tt.colMeta, tt.columnFamily, "ctrf")
			if (err != nil) != tt.wantErr {
				t.Errorf("castColumns() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("castColumns() = %v, want %v", got, tt.want)
			}
		})
	}
}

// compareComplexOperation checks if two ComplexOperation structures are equal.
func compareComplexOperation(expected, actual *ComplexOperation) bool {
	return expected.Append == actual.Append &&
		expected.mapKey == actual.mapKey &&
		expected.PrependList == actual.PrependList &&
		expected.UpdateListIndex == actual.UpdateListIndex &&
		expected.Delete == actual.Delete &&
		expected.ListDelete == actual.ListDelete &&
		reflect.DeepEqual(expected.ExpectedDatatype, actual.ExpectedDatatype)
}

func TestTranslator_CreateOrderedCodeKey(t *testing.T) {
	tests := []struct {
		name                         string
		pmks                         []types.Column
		values                       map[string]interface{}
		want                         []byte
		encodeIntValuesWithBigEndian bool
		wantErr                      bool
	}{
		{
			name: "simple string",
			pmks: []types.Column{
				{
					CQLType:      datatype.Varchar,
					ColumnName:   "user_id",
					IsPrimaryKey: true,
					PkPrecedence: 1,
				},
			},
			values: map[string]interface{}{
				"user_id": "user1",
			},
			want:                         []byte("user1"),
			encodeIntValuesWithBigEndian: false,
			wantErr:                      false,
		},
		{
			name: "int nonzero",
			pmks: []types.Column{
				{
					CQLType:      datatype.Bigint,
					ColumnName:   "user_id",
					IsPrimaryKey: true,
					PkPrecedence: 1,
				},
			},
			values: map[string]interface{}{
				"user_id": int64(1),
			},
			want:                         []byte("\x81"),
			encodeIntValuesWithBigEndian: false,
			wantErr:                      false,
		},
		{
			name: "int32 nonzero",
			pmks: []types.Column{
				{
					CQLType:      datatype.Int,
					ColumnName:   "user_id",
					IsPrimaryKey: true,
					PkPrecedence: 1,
				},
			},
			values: map[string]interface{}{
				"user_id": int32(1),
			},
			want:                         []byte("\x81"),
			encodeIntValuesWithBigEndian: false,
			wantErr:                      false,
		},
		{
			name: "int32 nonzero big endian",
			pmks: []types.Column{
				{
					CQLType:      datatype.Int,
					ColumnName:   "user_id",
					IsPrimaryKey: true,
					PkPrecedence: 1,
				},
			},
			values: map[string]interface{}{
				"user_id": int32(1),
			},
			want:                         []byte("\x00\xff\x00\xff\x00\xff\x00\xff\x00\xff\x00\xff\x00\xff\x01"),
			encodeIntValuesWithBigEndian: true,
			wantErr:                      false,
		},
		{
			name: "int32 max",
			pmks: []types.Column{
				{
					CQLType:      datatype.Int,
					ColumnName:   "user_id",
					IsPrimaryKey: true,
					PkPrecedence: 1,
				},
			},
			values: map[string]interface{}{
				"user_id": int32(2147483647),
			},
			want:                         []byte("\x00\xff\x00\xff\x00\xff\x00\xff\x7f\xff\xff\xff"),
			encodeIntValuesWithBigEndian: true,
			wantErr:                      false,
		},
		{
			name: "int64 max",
			pmks: []types.Column{
				{
					CQLType:      datatype.Bigint,
					ColumnName:   "user_id",
					IsPrimaryKey: true,
					PkPrecedence: 1,
				},
			},
			values: map[string]interface{}{
				"user_id": int64(9223372036854775807),
			},
			want:                         []byte("\x7f\xff\xff\xff\xff\xff\xff\xff"),
			encodeIntValuesWithBigEndian: true,
			wantErr:                      false,
		},
		{
			name: "negative int",
			pmks: []types.Column{
				{
					CQLType:      datatype.Bigint,
					ColumnName:   "user_id",
					IsPrimaryKey: true,
					PkPrecedence: 1,
				},
			},
			values: map[string]interface{}{
				"user_id": int64(-1),
			},
			want:                         []byte("\x7f"),
			encodeIntValuesWithBigEndian: false,
			wantErr:                      false,
		},
		{
			name: "negative int big endian fails",
			pmks: []types.Column{
				{
					CQLType:      datatype.Bigint,
					ColumnName:   "user_id",
					IsPrimaryKey: true,
					PkPrecedence: 1,
				},
			},
			values: map[string]interface{}{
				"user_id": int64(-1),
			},
			want:                         nil,
			encodeIntValuesWithBigEndian: true,
			wantErr:                      true,
		},
		{
			name: "int zero",
			pmks: []types.Column{
				{
					CQLType:      datatype.Bigint,
					ColumnName:   "user_id",
					IsPrimaryKey: true,
					PkPrecedence: 1,
				},
			},
			values: map[string]interface{}{
				"user_id": int64(0),
			},
			want:                         []byte("\x80"),
			encodeIntValuesWithBigEndian: false,
			wantErr:                      false,
		},
		{
			name: "int zero big endian",
			pmks: []types.Column{
				{
					CQLType:      datatype.Bigint,
					ColumnName:   "user_id",
					IsPrimaryKey: true,
					PkPrecedence: 1,
				},
			},
			values: map[string]interface{}{
				"user_id": int64(0),
			},
			want:                         []byte("\x00\xff\x00\xff\x00\xff\x00\xff\x00\xff\x00\xff\x00\xff\x00\xff"),
			encodeIntValuesWithBigEndian: true,
			wantErr:                      false,
		},
		{
			name: "compound key",
			pmks: []types.Column{
				{
					CQLType:      datatype.Varchar,
					ColumnName:   "user_id",
					IsPrimaryKey: true,
					PkPrecedence: 1,
				},
				{
					CQLType:      datatype.Bigint,
					ColumnName:   "team_num",
					IsPrimaryKey: true,
					PkPrecedence: 2,
				},
				{
					CQLType:      datatype.Varchar,
					ColumnName:   "city",
					IsPrimaryKey: true,
					PkPrecedence: 3,
				},
			},
			values: map[string]interface{}{
				"user_id":  "user1",
				"team_num": int64(1),
				"city":     "new york",
			},
			want:                         []byte("user1\x00\x01\x81\x00\x01new york"),
			encodeIntValuesWithBigEndian: false,
			wantErr:                      false,
		},
		{
			name: "compound key big endian",
			pmks: []types.Column{
				{
					CQLType:      datatype.Varchar,
					ColumnName:   "user_id",
					IsPrimaryKey: true,
					PkPrecedence: 1,
				},
				{
					CQLType:      datatype.Bigint,
					ColumnName:   "team_num",
					IsPrimaryKey: true,
					PkPrecedence: 2,
				},
				{
					CQLType:      datatype.Varchar,
					ColumnName:   "city",
					IsPrimaryKey: true,
					PkPrecedence: 3,
				},
			},
			values: map[string]interface{}{
				"user_id":  "user1",
				"team_num": int64(1),
				"city":     "new york",
			},
			want:                         []byte("user1\x00\x01\x00\xff\x00\xff\x00\xff\x00\xff\x00\xff\x00\xff\x00\xff\x01\x00\x01new york"),
			encodeIntValuesWithBigEndian: true,
			wantErr:                      false,
		},
		{
			name: "compound key with trailing empty",
			pmks: []types.Column{
				{
					CQLType:      datatype.Varchar,
					ColumnName:   "user_id",
					IsPrimaryKey: true,
					PkPrecedence: 1,
				},
				{
					CQLType:      datatype.Bigint,
					ColumnName:   "team_num",
					IsPrimaryKey: true,
					PkPrecedence: 2,
				},
				{
					CQLType:      datatype.Varchar,
					ColumnName:   "city",
					IsPrimaryKey: true,
					PkPrecedence: 3,
				},
				{
					CQLType:      datatype.Varchar,
					ColumnName:   "borough",
					IsPrimaryKey: true,
					PkPrecedence: 4,
				},
			},
			values: map[string]interface{}{
				"user_id":  "user3",
				"team_num": int64(3),
				"city":     "",
				"borough":  "",
			},
			want:                         []byte("user3\x00\x01\x83"),
			encodeIntValuesWithBigEndian: false,
			wantErr:                      false,
		},
		{
			name: "compound key with trailing empty big endian",
			pmks: []types.Column{
				{
					CQLType:      datatype.Varchar,
					ColumnName:   "user_id",
					IsPrimaryKey: true,
					PkPrecedence: 1,
				},
				{
					CQLType:      datatype.Bigint,
					ColumnName:   "team_num",
					IsPrimaryKey: true,
					PkPrecedence: 2,
				},
				{
					CQLType:      datatype.Varchar,
					ColumnName:   "city",
					IsPrimaryKey: true,
					PkPrecedence: 3,
				},
				{
					CQLType:      datatype.Varchar,
					ColumnName:   "borough",
					IsPrimaryKey: true,
					PkPrecedence: 4,
				},
			},
			values: map[string]interface{}{
				"user_id":  "user3",
				"team_num": int64(3),
				"city":     "",
				"borough":  "",
			},
			want:                         []byte("user3\x00\x01\x00\xff\x00\xff\x00\xff\x00\xff\x00\xff\x00\xff\x00\xff\x03"),
			encodeIntValuesWithBigEndian: true,
			wantErr:                      false,
		},
		{
			name: "compound key with empty middle",
			pmks: []types.Column{
				{
					CQLType:      datatype.Blob,
					ColumnName:   "user_id",
					IsPrimaryKey: true,
					PkPrecedence: 1,
				},
				{
					CQLType:      datatype.Blob,
					ColumnName:   "team_id",
					IsPrimaryKey: true,
					PkPrecedence: 2,
				},
				{
					CQLType:      datatype.Blob,
					ColumnName:   "city",
					IsPrimaryKey: true,
					PkPrecedence: 3,
				},
			},
			values: map[string]interface{}{
				"user_id": "\xa2",
				"team_id": "",
				"city":    "\xb7",
			},
			want:                         []byte("\xa2\x00\x01\x00\x00\x00\x01\xb7"),
			encodeIntValuesWithBigEndian: false,
			wantErr:                      false,
		},
		{
			name: "bytes with delimiter",
			pmks: []types.Column{
				{
					CQLType:      datatype.Blob,
					ColumnName:   "user_id",
					IsPrimaryKey: true,
					PkPrecedence: 1,
				},
			},
			values: map[string]interface{}{
				"user_id": "\x80\x00\x01\x81",
			},
			want:                         []byte("\x80\x00\xff\x01\x81"),
			encodeIntValuesWithBigEndian: false,
			wantErr:                      false,
		},
		{
			name: "compound key with 2 empty middle fields",
			pmks: []types.Column{
				{
					CQLType:      datatype.Blob,
					ColumnName:   "user_id",
					IsPrimaryKey: true,
					PkPrecedence: 1,
				},
				{
					CQLType:      datatype.Blob,
					ColumnName:   "team_num",
					IsPrimaryKey: true,
					PkPrecedence: 2,
				},
				{
					CQLType:      datatype.Blob,
					ColumnName:   "city",
					IsPrimaryKey: true,
					PkPrecedence: 3,
				},
				{
					CQLType:      datatype.Blob,
					ColumnName:   "borough",
					IsPrimaryKey: true,
					PkPrecedence: 4,
				},
			},
			values: map[string]interface{}{
				"user_id":  "\xa2",
				"team_num": "",
				"city":     "",
				"borough":  "\xb7",
			},
			want:                         []byte("\xa2\x00\x01\x00\x00\x00\x01\x00\x00\x00\x01\xb7"),
			encodeIntValuesWithBigEndian: false,
			wantErr:                      false,
		},
		{
			name: "byte strings",
			pmks: []types.Column{
				{
					CQLType:      datatype.Blob,
					ColumnName:   "user_id",
					IsPrimaryKey: true,
					PkPrecedence: 1,
				},
				{
					CQLType:      datatype.Blob,
					ColumnName:   "city",
					IsPrimaryKey: true,
					PkPrecedence: 2,
				},
			},
			values: map[string]interface{}{
				"user_id": "\xa5",
				"city":    "\x90",
			},
			want:                         []byte("\xa5\x00\x01\x90"),
			encodeIntValuesWithBigEndian: false,
			wantErr:                      false,
		},
		{
			name: "empty first value",
			pmks: []types.Column{
				{
					CQLType:      datatype.Varchar,
					ColumnName:   "user_id",
					IsPrimaryKey: true,
					PkPrecedence: 1,
				},
				{
					CQLType:      datatype.Blob,
					ColumnName:   "city",
					IsPrimaryKey: true,
					PkPrecedence: 2,
				},
			},
			values: map[string]interface{}{
				"user_id": "",
				"city":    "\xaa",
			},
			want:                         []byte("\x00\x00\x00\x01\xaa"),
			encodeIntValuesWithBigEndian: false,
			wantErr:                      false,
		},
		{
			name: "null escaped",
			pmks: []types.Column{
				{
					CQLType:      datatype.Varchar,
					ColumnName:   "user_id",
					IsPrimaryKey: true,
					PkPrecedence: 1,
				},
				{
					CQLType:      datatype.Varchar,
					ColumnName:   "city",
					IsPrimaryKey: true,
					PkPrecedence: 2,
				},
				{
					CQLType:      datatype.Varchar,
					ColumnName:   "borough",
					IsPrimaryKey: true,
					PkPrecedence: 3,
				},
			},
			values: map[string]interface{}{
				"user_id": "nn",
				"city":    "t\x00t",
				"borough": "end",
			},
			want:                         []byte("nn\x00\x01t\x00\xfft\x00\x01end"),
			encodeIntValuesWithBigEndian: false,
			wantErr:                      false,
		},
		{
			name: "null escaped",
			pmks: []types.Column{
				{
					CQLType:      datatype.Varchar,
					ColumnName:   "user_id",
					IsPrimaryKey: true,
					PkPrecedence: 1,
				},
				{
					CQLType:      datatype.Bigint,
					ColumnName:   "team_num",
					IsPrimaryKey: true,
					PkPrecedence: 2,
				},
				{
					CQLType:      datatype.Varchar,
					ColumnName:   "city",
					IsPrimaryKey: true,
					PkPrecedence: 3,
				},
			},
			values: map[string]interface{}{
				"user_id":  "abcd",
				"team_num": int64(45),
				"city":     "name",
			},
			want:                         []byte("abcd\x00\x01\x00\xff\x00\xff\x00\xff\x00\xff\x00\xff\x00\xff\x00\xff\x2d\x00\x01name"),
			encodeIntValuesWithBigEndian: true,
			wantErr:                      false,
		},
		{
			name: "invalid utf8 varchar returns error",
			pmks: []types.Column{
				{
					CQLType:      datatype.Varchar,
					ColumnName:   "user_id",
					IsPrimaryKey: true,
					PkPrecedence: 1,
				},
			},
			values: map[string]interface{}{
				"user_id": string([]uint8{182}),
			},
			want:                         nil,
			encodeIntValuesWithBigEndian: true,
			wantErr:                      true,
		},
		{
			name: "null char",
			pmks: []types.Column{
				{
					CQLType:      datatype.Varchar,
					ColumnName:   "user_id",
					IsPrimaryKey: true,
					PkPrecedence: 1,
				},
			},
			values: map[string]interface{}{
				"user_id": "\x00\x01",
			},
			want:                         []byte("\x00\xff\x01"),
			encodeIntValuesWithBigEndian: false,
			wantErr:                      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := createOrderedCodeKey(tt.pmks, tt.values, tt.encodeIntValuesWithBigEndian)
			if (err != nil) != tt.wantErr {
				t.Errorf("createOrderedCodeKey() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.want, got)
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
			got, err := EncodeBigInt(tt.args.value, tt.args.pv)
			if (err != nil) != tt.wantErr {
				t.Errorf("EncodeBigInt() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("EncodeBigInt() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProcessCollectionColumnsForPrepareQueries_ComplexMetaAndNonCollection(t *testing.T) {
	translator := &Translator{
		Logger:              zap.NewNop(),
		SchemaMappingConfig: GetSchemaMappingConfig(),
	}
	protocolV := primitive.ProtocolVersion4
	tableName := "non_primitive_table"
	keySpace := "test_keyspace"

	// --- Helper data ---
	textValueBytes, _ := proxycore.EncodeType(datatype.Varchar, protocolV, "testValue")
	textValue2Bytes, _ := proxycore.EncodeType(datatype.Varchar, protocolV, "testValue2")
	textValue3Bytes, _ := proxycore.EncodeType(datatype.Varchar, protocolV, "newValue")
	intValueBytes, _ := proxycore.EncodeType(datatype.Int, protocolV, int32(123))

	// Set data
	setTextType := datatype.NewSetType(datatype.Varchar)
	setValue := []string{"elem1", "elem2"}
	setValueBytes, _ := proxycore.EncodeType(setTextType, protocolV, setValue)

	// --- Test Cases ---
	tests := []struct {
		name            string
		columnsResponse []types.Column
		values          []*primitive.Value
		complexMeta     map[string]*ComplexOperation
		primaryKeys     []string
		// Expected outputs
		wantNewColumns   []types.Column
		wantNewValues    []interface{}
		wantUnencrypted  map[string]interface{}
		wantIndexEnd     int
		wantDelColFamily []string
		wantDelColumns   []types.Column
		wantErr          bool
	}{
		{
			name: "Non-collection column (text)",
			columnsResponse: []types.Column{
				{Name: "pk_1_text", CQLType: datatype.Varchar, ColumnFamily: "cf1"},
			},
			values: []*primitive.Value{
				{Contents: textValueBytes},
			},
			complexMeta: map[string]*ComplexOperation{},
			primaryKeys: []string{"pk_1_text"},
			wantNewColumns: []types.Column{
				{Name: "pk_1_text", CQLType: datatype.Varchar, ColumnFamily: "cf1"},
			},
			wantNewValues:    []interface{}{textValueBytes},
			wantUnencrypted:  map[string]interface{}{},
			wantIndexEnd:     0,
			wantDelColFamily: nil,
			wantDelColumns:   nil,
			wantErr:          false,
		},
		{
			name: "Non-collection column (int)",
			columnsResponse: []types.Column{
				{Name: "column_int", CQLType: datatype.Int, ColumnFamily: "cf1"},
			},
			values: []*primitive.Value{
				{Contents: intValueBytes},
			},
			complexMeta: map[string]*ComplexOperation{},
			primaryKeys: []string{},
			wantNewColumns: []types.Column{
				{Name: "column_int", CQLType: datatype.Int, ColumnFamily: "cf1"},
			},

			wantNewValues:    []interface{}{[]byte{0, 0, 0, 0, 0, 0, 0, 123}},
			wantUnencrypted:  map[string]interface{}{},
			wantIndexEnd:     0,
			wantDelColFamily: nil,
			wantDelColumns:   nil,
			wantErr:          false,
		},
		{
			name: "Map append for specific key",
			columnsResponse: []types.Column{
				{Name: "map_text_text", CQLType: datatype.NewMapType(datatype.Varchar, datatype.Varchar), ColumnFamily: "map_text_text"},
			},
			values: []*primitive.Value{
				{Contents: textValue2Bytes},
			},
			complexMeta: map[string]*ComplexOperation{
				"map_text_text": {
					Append:           true,
					mapKey:           "newKey",
					ExpectedDatatype: datatype.Varchar,
				},
			},
			primaryKeys: []string{},
			wantNewColumns: []types.Column{
				{Name: "newKey", ColumnFamily: "map_text_text", CQLType: datatype.Varchar},
			},
			wantNewValues:    []interface{}{textValue2Bytes},
			wantUnencrypted:  map[string]interface{}{},
			wantIndexEnd:     0,
			wantDelColFamily: nil,
			wantDelColumns:   nil,
			wantErr:          false,
		},
		{
			name: "Map delete",
			columnsResponse: []types.Column{
				{Name: "map_text_text", CQLType: datatype.NewMapType(datatype.Varchar, datatype.Varchar), ColumnFamily: "map_text_text"},
			},
			values: []*primitive.Value{
				// Value contains the keys to delete, encoded as a set<text>
				{Contents: setValueBytes},
			},
			complexMeta: map[string]*ComplexOperation{
				"map_text_text": {
					Delete:           true,
					ExpectedDatatype: setTextType, // Expecting a set of keys to delete
				},
			},
			primaryKeys:      []string{},
			wantNewColumns:   nil, // No new columns added
			wantNewValues:    []interface{}{},
			wantUnencrypted:  map[string]interface{}{},
			wantIndexEnd:     0,
			wantDelColFamily: nil, // Delete specific keys, not the whole family
			wantDelColumns: []types.Column{
				{Name: "elem1", ColumnFamily: "map_text_text"},
				{Name: "elem2", ColumnFamily: "map_text_text"},
			},
			wantErr: false,
		},
		{
			name: "List update by index",
			columnsResponse: []types.Column{
				{Name: "list_text", CQLType: datatype.NewListType(datatype.Varchar), ColumnFamily: "list_text"},
			},
			values: []*primitive.Value{
				{Contents: textValue3Bytes}, // The new value for the specific index
			},
			complexMeta: map[string]*ComplexOperation{
				"list_text": {
					UpdateListIndex: "1", // Update index 1
				},
			},
			primaryKeys:      []string{},
			wantNewColumns:   nil, // Update by index doesn't add new columns here, it modifies the meta
			wantNewValues:    []interface{}{},
			wantUnencrypted:  map[string]interface{}{},
			wantIndexEnd:     0,
			wantDelColFamily: nil,
			wantDelColumns:   nil,
			wantErr:          false,
			// We also need to check if complexMeta["list_text"].Value was updated, but that's harder in this structure
		},
		{
			name: "Set delete elements",
			columnsResponse: []types.Column{
				{Name: "set_text", CQLType: datatype.NewSetType(datatype.Varchar), ColumnFamily: "set_text"},
			},
			values: []*primitive.Value{
				{Contents: setValueBytes}, // The set containing elements to delete
			},
			complexMeta: map[string]*ComplexOperation{
				"set_text": {
					Delete:           true,
					ExpectedDatatype: setTextType,
				},
			},
			primaryKeys:      []string{},
			wantNewColumns:   nil,
			wantNewValues:    []interface{}{},
			wantUnencrypted:  map[string]interface{}{},
			wantIndexEnd:     0,
			wantDelColFamily: nil, // Deleting specific elements
			wantDelColumns: []types.Column{
				{Name: "elem1", ColumnFamily: "set_text"},
				{Name: "elem2", ColumnFamily: "set_text"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy of complexMeta for each test run
			currentComplexMeta := make(map[string]*ComplexOperation)
			for k, v := range tt.complexMeta {
				metaCopy := *v // Shallow copy is enough for this test structure
				currentComplexMeta[k] = &metaCopy
			}

			input := ProcessPrepareCollectionsInput{
				ColumnsResponse: tt.columnsResponse,
				Values:          tt.values,
				TableName:       tableName,
				ProtocolV:       protocolV,
				PrimaryKeys:     tt.primaryKeys,
				Translator:      translator,
				KeySpace:        keySpace,
				ComplexMeta:     currentComplexMeta,
			}
			output, err := processCollectionColumnsForPrepareQueries(input)

			if (err != nil) != tt.wantErr {
				t.Errorf("processCollectionColumnsForPrepareQueries() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return // Don't check results if an error was expected
			}

			// Sort slices of columns before comparing for deterministic results
			sort.Slice(output.NewColumns, func(i, j int) bool { return output.NewColumns[i].Name < output.NewColumns[j].Name })
			sort.Slice(tt.wantNewColumns, func(i, j int) bool { return tt.wantNewColumns[i].Name < tt.wantNewColumns[j].Name })
			sort.Slice(output.DelColumns, func(i, j int) bool { return output.DelColumns[i].Name < output.DelColumns[j].Name })
			sort.Slice(tt.wantDelColumns, func(i, j int) bool { return tt.wantDelColumns[i].Name < tt.wantDelColumns[j].Name })
			sort.Strings(output.DelColumnFamily)
			sort.Strings(tt.wantDelColFamily)

			// For list types, don't compare Names directly, normalize them for comparison
			if strings.Contains(tt.name, "List") {
				// Normalize the Name fields for comparison
				for i := range output.NewColumns {
					output.NewColumns[i].Name = fmt.Sprintf("list_index_%d", i)
				}
				for i := range tt.wantNewColumns {
					tt.wantNewColumns[i].Name = fmt.Sprintf("list_index_%d", i)
				}
			}

			if !reflect.DeepEqual(output.NewColumns, tt.wantNewColumns) {
				t.Errorf("processCollectionColumnsForPrepareQueries() output.NewColumns = %v, want %v", output.NewColumns, tt.wantNewColumns)
			}
			// Comparing slices of interfaces containing byte slices requires careful comparison
			if len(output.NewValues) != len(tt.wantNewValues) {
				t.Errorf("processCollectionColumnsForPrepareQueries() output.NewValues length = %d, want %d", len(output.NewValues), len(tt.wantNewValues))
			} else {
				// Simple byte comparison for this test setup
				for i := range output.NewValues {
					gotBytes, okGot := output.NewValues[i].([]byte)
					wantBytes, okWant := tt.wantNewValues[i].([]byte)
					if !okGot || !okWant || !reflect.DeepEqual(gotBytes, wantBytes) {
						t.Errorf("processCollectionColumnsForPrepareQueries() output.NewValues[%d] = %v, want %v", i, output.NewValues[i], tt.wantNewValues[i])
					}
				}
			}
			if !reflect.DeepEqual(output.Unencrypted, tt.wantUnencrypted) {
				t.Errorf("processCollectionColumnsForPrepareQueries() output.Unencrypted = %v, want %v", output.Unencrypted, tt.wantUnencrypted)
			}
			if output.IndexEnd != tt.wantIndexEnd {
				t.Errorf("processCollectionColumnsForPrepareQueries() output.IndexEnd = %v, want %v", output.IndexEnd, tt.wantIndexEnd)
			}
			if !reflect.DeepEqual(output.DelColumnFamily, tt.wantDelColFamily) {
				t.Errorf("processCollectionColumnsForPrepareQueries() output.DelColumnFamily = %v, want %v", output.DelColumnFamily, tt.wantDelColFamily)
			}
			if !reflect.DeepEqual(output.DelColumns, tt.wantDelColumns) {
				t.Errorf("processCollectionColumnsForPrepareQueries() output.DelColumns = %v, want %v", output.DelColumns, tt.wantDelColumns)
			}

			// Specific checks for complex meta modifications
			if tt.name == "List update by index" {
				meta, ok := currentComplexMeta["list_text"]
				if !ok || meta.UpdateListIndex != "1" || !reflect.DeepEqual(meta.Value, textValue3Bytes) {
					t.Errorf("List update by index: complexMeta not updated correctly. Got: %+v", meta)
				}
			}
			if tt.name == "List delete elements" {
				meta, ok := currentComplexMeta["list_text"]
				// Assuming listValueBytes corresponds to ["testValue", "testValue2"]
				expectedDeleteValues := [][]byte{textValueBytes, textValue2Bytes}
				if !ok || !meta.ListDelete || len(meta.ListDeleteValues) != len(expectedDeleteValues) {
					t.Errorf("List delete elements: complexMeta not updated correctly. Got: %+v", meta)
				} else {
					// Sort before comparing byte slices within the slice
					sort.Slice(meta.ListDeleteValues, func(i, j int) bool {
						return string(meta.ListDeleteValues[i]) < string(meta.ListDeleteValues[j])
					})
					sort.Slice(expectedDeleteValues, func(i, j int) bool {
						return string(expectedDeleteValues[i]) < string(expectedDeleteValues[j])
					})
					if !reflect.DeepEqual(meta.ListDeleteValues, expectedDeleteValues) {
						t.Errorf("List delete elements: ListDeleteValues mismatch. Got: %v, Want: %v", meta.ListDeleteValues, expectedDeleteValues)
					}
				}
			}
		})
	}
}

func TestProcessComplexUpdate_SuccessfulCases(t *testing.T) {
	translator := &Translator{
		SchemaMappingConfig: &schemaMapping.SchemaMappingConfig{
			TablesMetaData: map[string]map[string]map[string]*types.Column{
				"keyspace1": {
					"table1": {
						"map_col": {
							ColumnName:   "map_col",
							CQLType:      datatype.NewMapType(datatype.Varchar, datatype.Varchar),
							IsCollection: true,
						},
						"list_col": {
							ColumnName:   "list_col",
							CQLType:      datatype.NewListType(datatype.Varchar),
							IsCollection: true,
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name           string
		columns        []types.Column
		values         []interface{}
		tableName      string
		keyspaceName   string
		prependColumns []string
		wantMeta       map[string]*ComplexOperation
		wantErr        bool
	}{
		{
			name: "map append operation",
			columns: []types.Column{
				{Name: "map_col", CQLType: datatype.NewMapType(datatype.Varchar, datatype.Varchar)},
			},
			// values:         []interface{}{"map_col+{key:?}"},
			values: []interface{}{
				ComplexAssignment{
					Column:    "map_col",
					Operation: "+",
					Left:      "key",
				},
			},
			tableName:      "table1",
			keyspaceName:   "keyspace1",
			prependColumns: []string{},
			wantMeta: map[string]*ComplexOperation{
				"map_col": {
					Append: true,
				},
			},
			wantErr: false,
		},
		{
			name: "list prepend operation",
			columns: []types.Column{
				{Name: "list_col", CQLType: datatype.NewListType(datatype.Varchar)},
			},
			values: []interface{}{
				ComplexAssignment{
					Column:    "list_col",
					Operation: "+",
					Left:      "key",
					Right:     "list_col",
				},
			},
			tableName:      "table1",
			keyspaceName:   "keyspace1",
			prependColumns: []string{"list_col"},
			wantMeta: map[string]*ComplexOperation{
				"list_col": {
					PrependList: true,
					mapKey:      nil,
				},
			},
			wantErr: false,
		},
		{
			name: "multiple operations",
			columns: []types.Column{
				{Name: "map_col", CQLType: datatype.NewMapType(datatype.Varchar, datatype.Varchar)},
				{Name: "list_col", CQLType: datatype.NewListType(datatype.Varchar)},
			},
			// values: []interface{}{
			// 	"map_col+{key:?}",
			// 	"list_col+?",
			// },
			values: []interface{}{
				ComplexAssignment{
					Column:    "map_col",
					Operation: "+",
					Left:      "key",
					Right:     "map_col",
				},
				ComplexAssignment{
					Column:    "list_col",
					Operation: "+",
					Left:      "key",
					Right:     "list_col",
				},
			},
			tableName:      "table1",
			keyspaceName:   "keyspace1",
			prependColumns: []string{"list_col"},
			wantMeta: map[string]*ComplexOperation{
				"map_col": {
					Append: true,
					mapKey: nil,
				},
				"list_col": {
					PrependList: true,
					mapKey:      nil,
				},
			},
			wantErr: false,
		},
		{
			name: "non-collection column operation",
			columns: []types.Column{
				{Name: "normal_col", CQLType: datatype.Varchar},
			},
			values:         []interface{}{"normal_col+value"},
			tableName:      "table1",
			keyspaceName:   "keyspace1",
			prependColumns: []string{},
			wantMeta:       map[string]*ComplexOperation{},
			wantErr:        false,
		},
		{
			name: "skip invalid value type",
			columns: []types.Column{
				{Name: "map_col", CQLType: datatype.NewMapType(datatype.Varchar, datatype.Varchar)},
			},
			values:         []interface{}{123}, // Not a string
			tableName:      "table1",
			keyspaceName:   "keyspace1",
			prependColumns: []string{},
			wantMeta:       map[string]*ComplexOperation{},
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMeta, err := translator.ProcessComplexUpdate(tt.columns, tt.values, tt.tableName, tt.keyspaceName, tt.prependColumns)

			if (err != nil) != tt.wantErr {
				t.Errorf("ProcessComplexUpdate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			// Compare metadata using custom comparison
			if len(gotMeta) != len(tt.wantMeta) {
				t.Errorf("ProcessComplexUpdate() metadata length = %d, want %d", len(gotMeta), len(tt.wantMeta))
				return
			}
			for k, got := range gotMeta {
				want, exists := tt.wantMeta[k]
				if !exists {
					t.Errorf("ProcessComplexUpdate() unexpected key %s in result", k)
					continue
				}
				if !compareComplexOperation(got, want) {
					t.Errorf("ProcessComplexUpdate() metadata mismatch for key %s:\ngot  = %+v\nwant = %+v", k, got, want)
				}
			}
		})
	}
}

// --- Mocks for ANTLR interfaces ---
type mockFromSpecContext struct {
	cql.IFromSpecContext
	fromSpecElement cql.IFromSpecElementContext
}

func (m *mockFromSpecContext) FromSpecElement() cql.IFromSpecElementContext {
	return m.fromSpecElement
}

type mockFromSpecElementContext struct {
	cql.IFromSpecElementContext
	objectNames []antlr.TerminalNode
}

func (m *mockFromSpecElementContext) AllOBJECT_NAME() []antlr.TerminalNode {
	return m.objectNames
}

type mockTerminalNode struct {
	antlr.TerminalNode
	text string
}

func (m *mockTerminalNode) GetText() string {
	return m.text
}

// Tests for getFromSpecElement
func Test_getFromSpecElement(t *testing.T) {
	tests := []struct {
		name       string
		ctx        cql.IFromSpecContext
		want       cql.IFromSpecElementContext
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:    "Valid fromSpecElement",
			ctx:     &mockFromSpecContext{fromSpecElement: &mockFromSpecElementContext{}},
			want:    &mockFromSpecElementContext{},
			wantErr: false,
		},
		{
			name:    "Nil fromSpecElement",
			ctx:     &mockFromSpecContext{fromSpecElement: nil},
			want:    nil,
			wantErr: true,
		},
		{
			name:       "Nil input context",
			ctx:        nil,
			want:       nil,
			wantErr:    true,
			wantErrMsg: "input context is nil",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := getFromSpecElement(tt.ctx)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
				if tt.wantErrMsg != "" {
					assert.Equal(t, tt.wantErrMsg, err.Error())
				}
			} else {
				assert.NoError(t, err)
				// Can't use reflect.DeepEqual for interfaces with methods, so just check type
				assert.IsType(t, tt.want, result)
			}
		})
	}
}

// Tests for getAllObjectNames
func Test_getAllObjectNames(t *testing.T) {
	tests := []struct {
		name     string
		fromSpec cql.IFromSpecElementContext
		want     []antlr.TerminalNode
		wantErr  bool
	}{
		{
			name: "Both keyspace and table present",
			fromSpec: &mockFromSpecElementContext{objectNames: []antlr.TerminalNode{
				&mockTerminalNode{text: "ks"},
				&mockTerminalNode{text: "tbl"},
			}},
			want: []antlr.TerminalNode{
				&mockTerminalNode{text: "ks"},
				&mockTerminalNode{text: "tbl"},
			},
			wantErr: false,
		},
		{
			name: "Only table present",
			fromSpec: &mockFromSpecElementContext{objectNames: []antlr.TerminalNode{
				&mockTerminalNode{text: "tbl"},
			}},
			want: []antlr.TerminalNode{
				&mockTerminalNode{text: "tbl"},
			},
			wantErr: false,
		},
		{
			name:     "No object names (empty)",
			fromSpec: &mockFromSpecElementContext{objectNames: []antlr.TerminalNode{}},
			want:     nil,
			wantErr:  true,
		},
		{
			name:     "Nil object names",
			fromSpec: &mockFromSpecElementContext{objectNames: nil},
			want:     nil,
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := getAllObjectNames(tt.fromSpec)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, len(tt.want), len(result))
				for i := range tt.want {
					assert.Equal(t, tt.want[i].GetText(), result[i].GetText())
				}
			}
		})
	}
}

// Tests for getTableAndKeyspaceObjects
func Test_getTableAndKeyspaceObjects(t *testing.T) {
	tests := []struct {
		name         string
		objs         []antlr.TerminalNode
		wantKeyspace string
		wantTable    string
		wantErr      bool
		wantErrMsg   string
	}{
		{
			name: "Both keyspace and table present",
			objs: []antlr.TerminalNode{
				&mockTerminalNode{text: "ks"},
				&mockTerminalNode{text: "tbl"},
			},
			wantKeyspace: "ks",
			wantTable:    "tbl",
			wantErr:      false,
		},
		{
			name: "Only table present",
			objs: []antlr.TerminalNode{
				&mockTerminalNode{text: "tbl"},
			},
			wantKeyspace: "",
			wantTable:    "tbl",
			wantErr:      false,
		},
		{
			name: "Missing table name (empty string)",
			objs: []antlr.TerminalNode{
				&mockTerminalNode{text: "ks"},
				&mockTerminalNode{text: ""},
			},
			wantKeyspace: "",
			wantTable:    "",
			wantErr:      true,
			wantErrMsg:   "table is missing",
		},
		{
			name: "Extra parameters (more than 2 objects)",
			objs: []antlr.TerminalNode{
				&mockTerminalNode{text: "ks"},
				&mockTerminalNode{text: "tbl"},
				&mockTerminalNode{text: "extra"},
			},
			wantKeyspace: "",
			wantTable:    "",
			wantErr:      true,
		},
		{
			name:         "No objects (empty slice)",
			objs:         []antlr.TerminalNode{},
			wantKeyspace: "",
			wantTable:    "",
			wantErr:      true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keyspace, table, err := getTableAndKeyspaceObjects(tt.objs)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Equal(t, tt.wantKeyspace, keyspace)
				assert.Equal(t, tt.wantTable, table)
				if tt.wantErrMsg != "" {
					assert.Equal(t, tt.wantErrMsg, err.Error())
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantKeyspace, keyspace)
				assert.Equal(t, tt.wantTable, table)
			}
		})
	}
}

func TestAddSetElements(t *testing.T) {
	tests := []struct {
		name        string
		setValues   []string
		colFamily   string
		column      types.Column
		input       ProcessRawCollectionsInput
		output      *ProcessRawCollectionsOutput
		expectedErr bool
		validate    func(t *testing.T, output *ProcessRawCollectionsOutput)
	}{
		{
			name:      "Add single string element to set",
			setValues: []string{"value1"},
			colFamily: "test_family",
			column: types.Column{
				Name:    "test_set",
				CQLType: datatype.NewSetType(datatype.Varchar),
			},
			input:  ProcessRawCollectionsInput{},
			output: &ProcessRawCollectionsOutput{},
			validate: func(t *testing.T, output *ProcessRawCollectionsOutput) {
				assert.Len(t, output.NewColumns, 1)
				assert.Len(t, output.NewValues, 1)
				assert.Equal(t, "value1", output.NewColumns[0].Name)
				assert.Equal(t, "test_family", output.NewColumns[0].ColumnFamily)
				assert.Equal(t, datatype.Varchar, output.NewColumns[0].CQLType)
				assert.Empty(t, output.NewValues[0])
			},
		},
		{
			name:      "Add multiple string elements to set",
			setValues: []string{"value1", "value2", "value3"},
			colFamily: "test_family",
			column: types.Column{
				Name:    "test_set",
				CQLType: datatype.NewSetType(datatype.Varchar),
			},
			input:  ProcessRawCollectionsInput{},
			output: &ProcessRawCollectionsOutput{},
			validate: func(t *testing.T, output *ProcessRawCollectionsOutput) {
				assert.Len(t, output.NewColumns, 3)
				assert.Len(t, output.NewValues, 3)
				expectedValues := []string{"value1", "value2", "value3"}
				for i, val := range expectedValues {
					assert.Equal(t, val, output.NewColumns[i].Name)
					assert.Equal(t, "test_family", output.NewColumns[i].ColumnFamily)
					assert.Equal(t, datatype.Varchar, output.NewColumns[i].CQLType)
					assert.Empty(t, output.NewValues[i])
				}
			},
		},
		{
			name:      "Add boolean elements to set",
			setValues: []string{"true", "false"},
			colFamily: "test_family",
			column: types.Column{
				Name:    "test_set",
				CQLType: datatype.NewSetType(datatype.Boolean),
			},
			input:  ProcessRawCollectionsInput{},
			output: &ProcessRawCollectionsOutput{},
			validate: func(t *testing.T, output *ProcessRawCollectionsOutput) {
				assert.Len(t, output.NewColumns, 2)
				assert.Len(t, output.NewValues, 2)
				expectedValues := []string{"1", "0"}
				for i, val := range expectedValues {
					assert.Equal(t, val, output.NewColumns[i].Name)
					assert.Equal(t, "test_family", output.NewColumns[i].ColumnFamily)
					assert.Equal(t, datatype.Boolean, output.NewColumns[i].CQLType)
					assert.Empty(t, output.NewValues[i])
				}
			},
		},
		{
			name:      "Add elements to empty set",
			setValues: []string{},
			colFamily: "test_family",
			column: types.Column{
				Name:    "test_set",
				CQLType: datatype.NewSetType(datatype.Varchar),
			},
			input:  ProcessRawCollectionsInput{},
			output: &ProcessRawCollectionsOutput{},
			validate: func(t *testing.T, output *ProcessRawCollectionsOutput) {
				assert.Empty(t, output.NewColumns)
				assert.Empty(t, output.NewValues)
			},
		},
		{
			name:      "Add elements with empty column family",
			setValues: []string{"value1"},
			colFamily: "",
			column: types.Column{
				Name:    "test_set",
				CQLType: datatype.NewSetType(datatype.Varchar),
			},
			input:  ProcessRawCollectionsInput{},
			output: &ProcessRawCollectionsOutput{},
			validate: func(t *testing.T, output *ProcessRawCollectionsOutput) {
				assert.Len(t, output.NewColumns, 1)
				assert.Len(t, output.NewValues, 1)
				assert.Equal(t, "value1", output.NewColumns[0].Name)
				assert.Empty(t, output.NewColumns[0].ColumnFamily)
				assert.Equal(t, datatype.Varchar, output.NewColumns[0].CQLType)
				assert.Empty(t, output.NewValues[0])
			},
		},
		{
			name:      "Add elements with invalid boolean value",
			setValues: []string{"invalid"},
			colFamily: "test_family",
			column: types.Column{
				Name:    "test_set",
				CQLType: datatype.NewSetType(datatype.Boolean),
			},
			input:       ProcessRawCollectionsInput{},
			output:      &ProcessRawCollectionsOutput{},
			expectedErr: true,
		},
		{
			name:      "Add elements with nil output",
			setValues: []string{"value1"},
			colFamily: "test_family",
			column: types.Column{
				Name:    "test_set",
				CQLType: datatype.NewSetType(datatype.Varchar),
			},
			input:       ProcessRawCollectionsInput{},
			output:      nil,
			expectedErr: true,
		},
		{
			name:      "Add elements with different data types",
			setValues: []string{"value1", "123", "true"},
			colFamily: "test_family",
			column: types.Column{
				Name:    "test_set",
				CQLType: datatype.NewSetType(datatype.Int),
			},
			input:       ProcessRawCollectionsInput{},
			output:      &ProcessRawCollectionsOutput{},
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := addSetElements(tt.setValues, tt.colFamily, tt.column, tt.input, tt.output)

			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, tt.output)
				}
			}
		})
	}
}

func TestHandleListOperation(t *testing.T) {
	tests := []struct {
		name      string
		column    types.Column
		input     ProcessRawCollectionsInput
		operation interface{}
		wantErr   bool
		validate  func(t *testing.T, output *ProcessRawCollectionsOutput)
	}{
		{
			name: "Add operation with prepend",
			column: types.Column{
				Name:    "mylist",
				CQLType: datatype.NewListType(datatype.Int),
			},
			input: ProcessRawCollectionsInput{
				PrependColumns: []string{"mylist"},
			},
			operation: ComplexAssignment{Operation: "+", Left: []string{"1", "2"}, Right: []string{"3", "4"}},
			validate: func(t *testing.T, output *ProcessRawCollectionsOutput) {
				assert.Len(t, output.NewValues, 2)
				assert.Len(t, output.NewColumns, 2)
				for _, col := range output.NewColumns {
					assert.Equal(t, "mylist", col.ColumnFamily)
					assert.Equal(t, datatype.Int, col.CQLType)
				}
			},
		},
		{
			name: "Remove operation",
			column: types.Column{
				Name:    "mylist",
				CQLType: datatype.NewListType(datatype.Int),
			},
			operation: ComplexAssignment{Operation: "-", Right: []string{"3", "4"}},
			input:     ProcessRawCollectionsInput{},
			validate: func(t *testing.T, output *ProcessRawCollectionsOutput) {
				assert.NotNil(t, output.ComplexMeta["mylist"])
				assert.True(t, output.ComplexMeta["mylist"].ListDelete)
				assert.True(t, output.ComplexMeta["mylist"].Delete)
			},
		},
		{
			name: "Update index operation",
			column: types.Column{
				Name:    "mylist",
				CQLType: datatype.NewListType(datatype.Int),
			},
			operation: ComplexAssignment{Operation: "update_index", Left: "1", Right: "123"},
			input:     ProcessRawCollectionsInput{},
			validate: func(t *testing.T, output *ProcessRawCollectionsOutput) {
				assert.NotNil(t, output.ComplexMeta["mylist"])
				assert.Equal(t, "1", output.ComplexMeta["mylist"].UpdateListIndex)
			},
		},
		{
			name: "Simple assignment",
			column: types.Column{
				Name:    "mylist",
				CQLType: datatype.NewListType(datatype.Int),
			},
			operation: []string{"1", "2"},
			input:     ProcessRawCollectionsInput{},
			validate: func(t *testing.T, output *ProcessRawCollectionsOutput) {
				assert.Len(t, output.NewValues, 2)
				assert.Len(t, output.NewColumns, 2)
				assert.Len(t, output.DelColumnFamily, 1)
				assert.Equal(t, "mylist", output.DelColumnFamily[0])
			},
		},
		{
			name: "Invalid operation",
			column: types.Column{
				Name:    "mylist",
				CQLType: datatype.NewListType(datatype.Int),
			},
			operation: ComplexAssignment{Operation: "invalid"},
			input:     ProcessRawCollectionsInput{},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := &ProcessRawCollectionsOutput{
				ComplexMeta: make(map[string]*ComplexOperation),
			}
			err := handleListOperation(tt.operation, tt.column, tt.column.Name, tt.input, output)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			if tt.validate != nil {
				tt.validate(t, output)
			}
		})
	}
}

func TestHandleSetOperation(t *testing.T) {
	tests := []struct {
		name      string
		column    types.Column
		input     ProcessRawCollectionsInput
		operation interface{}
		wantErr   bool
		validate  func(t *testing.T, output *ProcessRawCollectionsOutput)
	}{
		{
			name: "Add elements to set",
			column: types.Column{
				Name:    "myset",
				CQLType: datatype.NewSetType(datatype.Int),
			},
			operation: ComplexAssignment{Operation: "+", Right: []string{"1", "2", "3"}},
			input:     ProcessRawCollectionsInput{},
			validate: func(t *testing.T, output *ProcessRawCollectionsOutput) {
				assert.Len(t, output.NewColumns, 3)
				assert.Len(t, output.NewValues, 3)
				for i, col := range output.NewColumns {
					assert.Equal(t, "myset", col.ColumnFamily)
					assert.Equal(t, datatype.Int, col.CQLType)
					assert.Equal(t, fmt.Sprintf("%d", i+1), col.Name)
				}
			},
		},
		{
			name: "Remove elements from set",
			column: types.Column{
				Name:    "myset",
				CQLType: datatype.NewSetType(datatype.Int),
			},
			operation: ComplexAssignment{Operation: "-", Right: []string{"1", "2"}},
			input:     ProcessRawCollectionsInput{},
			validate: func(t *testing.T, output *ProcessRawCollectionsOutput) {
				assert.Len(t, output.DelColumns, 2)
				for _, col := range output.DelColumns {
					assert.Equal(t, "myset", col.ColumnFamily)
				}
			},
		},
		{
			name: "Simple assignment to set",
			column: types.Column{
				Name:    "myset",
				CQLType: datatype.NewSetType(datatype.Int),
			},
			operation: []string{"1", "2", "3"},
			input:     ProcessRawCollectionsInput{},
			validate: func(t *testing.T, output *ProcessRawCollectionsOutput) {
				assert.Len(t, output.NewColumns, 3)
				assert.Len(t, output.NewValues, 3)
				assert.Len(t, output.DelColumnFamily, 1)
				assert.Equal(t, "myset", output.DelColumnFamily[0])
			},
		},
		{
			name: "Invalid operation",
			column: types.Column{
				Name:    "myset",
				CQLType: datatype.NewSetType(datatype.Int),
			},
			operation: ComplexAssignment{Operation: "invalid"},
			input:     ProcessRawCollectionsInput{},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := &ProcessRawCollectionsOutput{
				ComplexMeta: make(map[string]*ComplexOperation),
			}
			err := handleSetOperation(tt.operation, tt.column, tt.column.Name, tt.input, output)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			if tt.validate != nil {
				tt.validate(t, output)
			}
		})
	}
}

func TestHandleMapOperation(t *testing.T) {
	tests := []struct {
		name      string
		column    types.Column
		input     ProcessRawCollectionsInput
		operation interface{}
		wantErr   bool
		validate  func(t *testing.T, output *ProcessRawCollectionsOutput)
	}{
		{
			name: "Add entries to map",
			column: types.Column{
				Name:    "mymap",
				CQLType: datatype.NewMapType(datatype.Varchar, datatype.Int),
			},
			operation: ComplexAssignment{Operation: "+", Right: map[string]string{"key1": "1", "key2": "2"}},
			input:     ProcessRawCollectionsInput{},
			validate: func(t *testing.T, output *ProcessRawCollectionsOutput) {
				assert.Len(t, output.NewColumns, 2)
				assert.Len(t, output.NewValues, 2)
				for _, col := range output.NewColumns {
					assert.Equal(t, "mymap", col.ColumnFamily)
					assert.Contains(t, []string{"key1", "key2"}, col.Name)
				}
			},
		},
		{
			name: "Remove entries from map",
			column: types.Column{
				Name:    "mymap",
				CQLType: datatype.NewMapType(datatype.Varchar, datatype.Int),
			},
			operation: ComplexAssignment{Operation: "-", Right: []string{"key1", "key2"}},
			input:     ProcessRawCollectionsInput{},
			validate: func(t *testing.T, output *ProcessRawCollectionsOutput) {
				assert.Len(t, output.DelColumns, 2)
				for _, col := range output.DelColumns {
					assert.Equal(t, "mymap", col.ColumnFamily)
					assert.Contains(t, []string{"key1", "key2"}, col.Name)
				}
			},
		},
		{
			name: "Update map index",
			column: types.Column{
				Name:    "mymap",
				CQLType: datatype.NewMapType(datatype.Varchar, datatype.Int),
			},
			operation: ComplexAssignment{Operation: "update_index", Left: "key1", Right: "99"},
			input:     ProcessRawCollectionsInput{},
			validate: func(t *testing.T, output *ProcessRawCollectionsOutput) {
				assert.Len(t, output.NewColumns, 1)
				assert.Equal(t, "key1", output.NewColumns[0].Name)
				assert.Equal(t, "mymap", output.NewColumns[0].ColumnFamily)
			},
		},
		{
			name: "Simple assignment to map",
			column: types.Column{
				Name:    "mymap",
				CQLType: datatype.NewMapType(datatype.Varchar, datatype.Int),
			},
			operation: map[string]string{"key1": "1", "key2": "2"},
			input:     ProcessRawCollectionsInput{},
			validate: func(t *testing.T, output *ProcessRawCollectionsOutput) {
				assert.Len(t, output.NewColumns, 2)
				assert.Len(t, output.NewValues, 2)
				assert.Len(t, output.DelColumnFamily, 1)
				assert.Equal(t, "mymap", output.DelColumnFamily[0])
			},
		},
		{
			name: "Invalid map key type",
			column: types.Column{
				Name:    "mymap",
				CQLType: datatype.NewMapType(datatype.Int, datatype.Int),
			},
			operation: ComplexAssignment{Operation: "+", Right: map[string]string{"key1": "1"}},
			input:     ProcessRawCollectionsInput{},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := &ProcessRawCollectionsOutput{
				ComplexMeta: make(map[string]*ComplexOperation),
			}
			err := handleMapOperation(tt.operation, tt.column, tt.column.Name, tt.input, output)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			if tt.validate != nil {
				tt.validate(t, output)
			}
		})
	}
}

func TestProcessCollectionColumnsForRawQueries(t *testing.T) {
	// Mock key data types for columns
	colList := types.Column{
		Name:         "list_text",
		CQLType:      datatype.NewListType(datatype.Varchar),
		IsCollection: true,
	}
	colSet := types.Column{
		Name:    "column7",
		CQLType: datatype.NewSetType(datatype.Varchar),
	}
	colMap := types.Column{
		Name:    "map_text_text",
		CQLType: datatype.NewMapType(datatype.Varchar, datatype.Varchar),
	}

	// Mock inputs
	inputs := ProcessRawCollectionsInput{
		Columns: []types.Column{colList, colSet, colMap /* add more as needed */},
		Values: []interface{}{
			[]string{"hi", "hello"},
			[]string{"alpha", "beta"},
			map[string]string{"k1": "v1", "k2": "v2"},
		},
		TableName:      "test_table",
		KeySpace:       "test_keyspace",
		PrependColumns: []string{"mylist"},
		Translator: &Translator{
			Logger:              zap.NewExample(), // or zap.NewNop() for silent logs
			SchemaMappingConfig: GetSchemaMappingConfig(),
		},
	}

	output, err := processCollectionColumnsForRawQueries(inputs)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}

	// Verify that output updated accordingly
	if len(output.NewColumns) == 0 || len(output.NewValues) == 0 {
		t.Errorf("Expected non-empty NewColumns and NewValues")
	}
}

func TestConvertAllValuesToRowKeyType(t *testing.T) {
	pkCols := []types.Column{
		{
			ColumnName:   "id_int",
			CQLType:      datatype.Int,
			IsPrimaryKey: true,
		},
		{
			ColumnName:   "id_bigint",
			CQLType:      datatype.Bigint,
			IsPrimaryKey: true,
		},
		{
			ColumnName:   "name_varchar",
			CQLType:      datatype.Varchar,
			IsPrimaryKey: true,
		},
		{
			ColumnName:   "blob_pk",
			CQLType:      datatype.Blob,
			IsPrimaryKey: true,
		},
	}

	values := map[string]interface{}{
		"id_int":       "123",
		"id_bigint":    "987654321",
		"name_varchar": "validUTF8",
		"blob_pk":      "blob_data",
	}

	result, err := convertAllValuesToRowKeyType(pkCols, values)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Check results
	if result["id_int"] != int64(123) {
		t.Errorf("Expected 123, got %v", result["id_int"])
	}
	if result["id_bigint"] != int64(987654321) {
		t.Errorf("Expected 987654321, got %v", result["id_bigint"])
	}
	if result["name_varchar"] != "validUTF8" {
		t.Errorf("Expected 'validUTF8', got %v", result["name_varchar"])
	}
	if result["blob_pk"] != "blob_data" {
		t.Errorf("Expected 'blob_data', got %v", result["blob_pk"])
	}

	// Test with non-string unsupported type for varchar
	valuesInvalid := map[string]interface{}{
		"name_varchar": 12345,
	}
	_, err = convertAllValuesToRowKeyType(pkCols, valuesInvalid)
	if err == nil {
		t.Errorf("Expected error for invalid varchar input")
	}

	// Test missing key
	incompleteValues := map[string]interface{}{
		"id_int": "123",
		// missing "id_bigint"
	}
	_, err = convertAllValuesToRowKeyType(pkCols, incompleteValues)
	if err == nil {
		t.Errorf("Expected error for missing primary key")
	}
}

func TestCqlTypeToEmptyPrimitive(t *testing.T) {
	tests := []struct {
		name         string
		cqlType      datatype.DataType
		isPrimaryKey bool
		expected     interface{}
	}{
		{
			name:         "Int type",
			cqlType:      datatype.Int,
			isPrimaryKey: false,
			expected:     int32(0),
		},
		{
			name:         "Bigint type",
			cqlType:      datatype.Bigint,
			isPrimaryKey: false,
			expected:     int64(0),
		},
		{
			name:         "Float type",
			cqlType:      datatype.Float,
			isPrimaryKey: false,
			expected:     float32(0),
		},
		{
			name:         "Double type",
			cqlType:      datatype.Double,
			isPrimaryKey: false,
			expected:     float64(0),
		},
		{
			name:         "Boolean type",
			cqlType:      datatype.Boolean,
			isPrimaryKey: false,
			expected:     false,
		},
		{
			name:         "Timestamp type",
			cqlType:      datatype.Timestamp,
			isPrimaryKey: false,
			expected:     time.Time{},
		},
		{
			name:         "Blob type",
			cqlType:      datatype.Blob,
			isPrimaryKey: false,
			expected:     []byte{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cqlTypeToEmptyPrimitive(tt.cqlType, tt.isPrimaryKey)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("For cqlType %v and isPrimaryKey %v, expected %v (%T), but got %v (%T)",
					tt.cqlType, tt.isPrimaryKey, tt.expected, tt.expected, result, result)
			}
		})
	}
}

func TestRenameLiterals(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected string
	}{
		{
			name:     "No literals",
			query:    `SELECT * FROM table`,
			expected: `SELECT * FROM table`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renameLiterals(tt.query)
			if result != tt.expected {
				t.Errorf("Expected: %s, Got: %s", tt.expected, result)
			}
		})
	}
}
