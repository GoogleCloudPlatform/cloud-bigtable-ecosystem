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
	"testing"
	"time"

	"cloud.google.com/go/bigtable"
	"github.com/antlr4-go/antlr/v4"

	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/ollionorg/cassandra-to-bigtable-proxy/proxycore"
	schemaMapping "github.com/ollionorg/cassandra-to-bigtable-proxy/schema-mapping"
	"go.uber.org/zap"
)

func Test_convertMapString(t *testing.T) {
	type args struct {
		dataStr string
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]interface{}
		wantErr bool
	}{
		{
			name: "sucess",
			args: args{
				dataStr: "{i3=true, 5023.0=true}",
			},
			want: map[string]interface{}{
				"i3":     true,
				"5023.0": true,
			},
			wantErr: false,
		},
		{
			name: "error",
			args: args{
				dataStr: "test",
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := convertMapString(tt.args.dataStr)
			if (err != nil) != tt.wantErr {
				t.Errorf("convertMapString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("convertMapString() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Helper function to create a mock TerminalNode
func createMockTerminalNode(token antlr.Token) antlr.Tree {
	return antlr.NewTerminalNodeImpl(token)
}

func Test_treetoString(t *testing.T) {
	tests := []struct {
		name   string
		input  antlr.Tree
		expect string
	}{
		{
			name:   "TerminalNode",
			input:  createMockTerminalNode(antlr.NewCommonToken(&antlr.TokenSourceCharStreamPair{}, 1, 1, 1, 1)),
			expect: "", // Modify the expected output accordingly
		},
		{
			name:   "NonTerminalNode",
			input:  antlr.NewErrorNodeImpl(nil),
			expect: "", // Modify the expected output accordingly
		},
		{
			name:   "NilInput",
			input:  nil,
			expect: "", // Modify the expected output accordingly
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := treetoString(tt.input)

			if result != tt.expect {
				t.Errorf("Test '%s' failed: expected '%s', got '%s'", tt.name, tt.expect, result)
			}
		})
	}
}

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
			name:     "Unix timestamp (seconds)",
			input:    "1672522562",
			expected: time.Unix(1672522562, 0),
		},
		{
			name:     "Unix timestamp (milliseconds)",
			input:    "1672522562000",
			expected: time.Unix(0, 1672522562000*int64(time.Millisecond)),
		},
		{
			name:     "Unix timestamp (microseconds)",
			input:    "1672522562000000",
			expected: time.Unix(0, 1672522562000000*int64(time.Microsecond)),
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

// TestConvertMap tests the convertMap function with various inputs.
func TestConvertToMap(t *testing.T) {
	cases := []struct {
		name     string
		dataStr  string
		cqlType  string
		expected map[string]interface{}
		wantErr  bool
	}{
		{
			name:    "Valid map<text, timestamp>",
			dataStr: `{"key1":"2024-02-05T14:00:00Z", "key2":"2024-02-05T14:00:00Z"}`,
			cqlType: "map<text, timestamp>",
			expected: map[string]interface{}{
				"key1": time.Date(2024, 2, 5, 14, 0, 0, 0, time.UTC),
				"key2": time.Date(2024, 2, 5, 14, 0, 0, 0, time.UTC),
			},
		},
		{
			name:    "Valid map<text, timestamp> Case 2",
			dataStr: `{"key1":"2023-01-30T09:00:00Z"}`,
			cqlType: "map<text, timestamp>",
			expected: map[string]interface{}{
				"key1": time.Date(2023, time.January, 30, 9, 0, 0, 0, time.UTC),
			},
		},
		{
			name:    "Valid map<text, timestamp> time in unix seconds",
			dataStr: `{"key1":1675075200}`,
			cqlType: "map<text, timestamp>",
			expected: map[string]interface{}{
				"key1": time.Date(2023, time.January, 30, 10, 40, 0, 0, time.UTC),
			},
		},
		{
			name:    "Valid map<text, timestamp> time in unix miliseconds",
			dataStr: `{"key1":1675075200}`,
			cqlType: "map<text, timestamp>",
			expected: map[string]interface{}{
				"key1": time.Date(2023, time.January, 30, 10, 40, 0, 0, time.UTC),
			},
		},
		{
			name:    "Valid map<text, timestamp> time in unix miliseconds and equal operator",
			dataStr: `{"key1"=1675075200}`,
			cqlType: "map<text, timestamp>",
			expected: map[string]interface{}{
				"key1": time.Date(2023, time.January, 30, 10, 40, 0, 0, time.UTC),
			},
		},
		{
			name:    "Valid map<text, text> with equal ",
			dataStr: `{"key1"="1675075200"}`,
			cqlType: "map<text, text>",
			expected: map[string]interface{}{
				"key1": "1675075200",
			},
		},
		{
			name:    "Valid map<text, text> with colon",
			dataStr: `{"key1":"1675075200"}`,
			cqlType: "map<text, text>",
			expected: map[string]interface{}{
				"key1": "1675075200",
			},
		},
		{
			name:    "Invalid pair formate",
			dataStr: `{"key11675075200, "sdfdgdgdf"}`,
			cqlType: "map<text, text>",
			wantErr: true,
		},
		{
			name:    "Invalid timestamp format",
			dataStr: `{"key1": "invalid-timestamp"}`,
			cqlType: "map<text, timestamp>",
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := convertToMap(tc.dataStr, tc.cqlType)
			if (err != nil) != tc.wantErr {
				t.Errorf("convertMap() error = %v, wantErr %v", err, tc.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tc.expected) && !tc.wantErr {
				t.Errorf("convertMap() = %v, want %v", got, tc.expected)
			}
		})
	}
}

func TestMapOfStringToString(t *testing.T) {
	cases := []struct {
		name     string
		pairs    []string
		expected map[string]interface{}
		wantErr  bool
	}{
		{
			name:  "Valid pairs with colon separator",
			pairs: []string{"key1:value1", "key2:value2"},
			expected: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
			wantErr: false,
		},
		{
			name:  "Valid pairs with equal separator",
			pairs: []string{"key1=value1", "key2=value2"},
			expected: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
			wantErr: false,
		},
		{
			name:    "Invalid pair format",
			pairs:   []string{"key1-value1"},
			wantErr: true,
		},
		{
			name:  "Mixed separators",
			pairs: []string{"key1:value1", "key2=value2"},
			expected: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
			wantErr: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := mapOfStringToString(tc.pairs)
			if (err != nil) != tc.wantErr {
				t.Errorf("mapOfStringToString() error = %v, wantErr %v", err, tc.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tc.expected) && !tc.wantErr {
				t.Errorf("mapOfStringToString() got = %v, want %v", got, tc.expected)
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
			want:    []string{"column6", "column9", "column10", "column2", "column3", "column5", "column1"},
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

func TestExtractCFAndQualifiersforMap(t *testing.T) {
	tests := []struct {
		input         string
		expectedCF    string
		expectedQuals []string
		expectedErr   bool
	}{
		{"cf-{q1,q2,q3}", "cf", []string{"q1", "q2", "q3"}, false},
		{"cf-{q1}", "cf", []string{"q1"}, false},
		{"cf-{}", "cf", []string{""}, false},
		{"cf", "", nil, true},
		{"-{}", "", nil, true},
		{"cf-{,}", "cf", []string{"", ""}, false},
		{"cf-{q1, q2 , q3}", "cf", []string{"q1", " q2 ", " q3"}, false},
		{"", "", nil, true},
	}

	for _, tt := range tests {
		cf, quals, err := ExtractCFAndQualifiersforMap(tt.input)
		if (err != nil) != tt.expectedErr {
			t.Errorf("ExtractCFAndQualifiersforMap(%v) error = %v, expectedErr %v", tt.input, err, tt.expectedErr)
		}
		if cf != tt.expectedCF {
			t.Errorf("ExtractCFAndQualifiersforMap(%v) cf = %v, expected %v", tt.input, cf, tt.expectedCF)
		}
		if !reflect.DeepEqual(quals, tt.expectedQuals) {
			t.Errorf("ExtractCFAndQualifiersforMap(%v) quals = %v, expected %v", tt.input, quals, tt.expectedQuals)
		}
	}
}

func Test_processCollectionColumnsForRawQueries(t *testing.T) {
	trueVal, _ := formatValues("true", "boolean", primitive.ProtocolVersion4)
	textVal, _ := formatValues("test", "text", primitive.ProtocolVersion4)
	emptyVal, _ := formatValues("", "text", primitive.ProtocolVersion4)
	intVal, _ := formatValues("10", "int", primitive.ProtocolVersion4)
	floatVal, _ := formatValues("10.10", "float", primitive.ProtocolVersion4)
	doubleVal, _ := formatValues("10.10101", "double", primitive.ProtocolVersion4)
	timestampVal, _ := formatValues("1234567890", "timestamp", primitive.ProtocolVersion4)
	bigintVal, _ := formatValues("1234567890", "bigint", primitive.ProtocolVersion4)
	type args struct {
		columns   []Column
		values    []interface{}
		tableName string
		t         *Translator
	}
	tests := []struct {
		name    string
		args    args
		want    []Column
		want1   []interface{}
		want2   []string
		want3   []Column
		wantErr bool
	}{
		{
			name: "Valid Input For Text Boolean",
			args: args{
				columns:   []Column{{Name: "column8", ColumnFamily: "column8", CQLType: "map<text,boolean>"}},
				values:    []interface{}{"{test:true}"},
				tableName: "test_table",
				t: &Translator{
					Logger:              zap.NewNop(),
					SchemaMappingConfig: GetSchemaMappingConfig(),
				},
			},
			want: []Column{
				{Name: "test", ColumnFamily: "column8", CQLType: "boolean"},
			},
			want1: []interface{}{trueVal},
			want2: []string{
				"column8",
			},
			want3:   []Column{},
			wantErr: false,
		},
		{
			name: "Valid Input For Text Text",
			args: args{
				columns:   []Column{{Name: "map_text_text", ColumnFamily: "map_text_text", CQLType: "map<text,text>"}},
				values:    []interface{}{"{test:test}"},
				tableName: "non_primitive_table",
				t: &Translator{
					Logger:              zap.NewNop(),
					SchemaMappingConfig: GetSchemaMappingConfig(),
				},
			},
			want: []Column{
				{Name: "test", ColumnFamily: "map_text_text", CQLType: "text"},
			},
			want1: []interface{}{textVal},
			want2: []string{
				"map_text_text",
			},
			want3:   []Column{},
			wantErr: false,
		},
		{
			name: "Valid Input For Text Int",
			args: args{
				columns:   []Column{{Name: "map_text_int", ColumnFamily: "map_text_int", CQLType: "map<text,int>"}},
				values:    []interface{}{"{test:10}"},
				tableName: "non_primitive_table",
				t: &Translator{
					Logger:              zap.NewNop(),
					SchemaMappingConfig: GetSchemaMappingConfig(),
				},
			},
			want: []Column{
				{Name: "test", ColumnFamily: "map_text_int", CQLType: "int"},
			},
			want1: []interface{}{intVal},
			want2: []string{
				"map_text_int",
			},
			want3:   []Column{},
			wantErr: false,
		},
		{
			name: "Valid Input For Text Float",
			args: args{
				columns:   []Column{{Name: "map_text_float", ColumnFamily: "map_text_float", CQLType: "map<text,float>"}},
				values:    []interface{}{"{test:10.10}"},
				tableName: "non_primitive_table",
				t: &Translator{
					Logger:              zap.NewNop(),
					SchemaMappingConfig: GetSchemaMappingConfig(),
				},
			},
			want: []Column{
				{Name: "test", ColumnFamily: "map_text_float", CQLType: "float"},
			},
			want1: []interface{}{floatVal},
			want2: []string{
				"map_text_float",
			},
			want3:   []Column{},
			wantErr: false,
		},
		{
			name: "Valid Input For Text Double",
			args: args{
				columns:   []Column{{Name: "map_text_double", ColumnFamily: "map_text_double", CQLType: "map<text,double>"}},
				values:    []interface{}{"{test:10.10101}"},
				tableName: "non_primitive_table",
				t: &Translator{
					Logger:              zap.NewNop(),
					SchemaMappingConfig: GetSchemaMappingConfig(),
				},
			},
			want: []Column{
				{Name: "test", ColumnFamily: "map_text_double", CQLType: "double"},
			},
			want1: []interface{}{doubleVal},
			want2: []string{
				"map_text_double",
			},
			want3:   []Column{},
			wantErr: false,
		},
		{
			name: "Valid Input For Text Timestamp",
			args: args{
				columns:   []Column{{Name: "map_text_timestamp", ColumnFamily: "map_text_timestamp", CQLType: "map<text,timestamp>"}},
				values:    []interface{}{"{test:1234567890}"},
				tableName: "non_primitive_table",
				t: &Translator{
					Logger:              zap.NewNop(),
					SchemaMappingConfig: GetSchemaMappingConfig(),
				},
			},
			want: []Column{
				{Name: "test", ColumnFamily: "map_text_timestamp", CQLType: "timestamp"},
			},
			want1: []interface{}{timestampVal},
			want2: []string{
				"map_text_timestamp",
			},
			want3:   []Column{},
			wantErr: false,
		},
		{
			name: "Valid Input For Timestamp Text",
			args: args{
				columns:   []Column{{Name: "map_timestamp_text", ColumnFamily: "map_timestamp_text", CQLType: "map<timestamp,text>"}},
				values:    []interface{}{"{1234567890:test}"},
				tableName: "non_primitive_table",
				t: &Translator{
					Logger:              zap.NewNop(),
					SchemaMappingConfig: GetSchemaMappingConfig(),
				},
			},
			want: []Column{
				{Name: "1234567890", ColumnFamily: "map_timestamp_text", CQLType: "text"},
			},
			want1: []interface{}{textVal},
			want2: []string{
				"map_timestamp_text",
			},
			want3:   []Column{},
			wantErr: false,
		},
		{
			name: "Valid Input For Timestamp Int",
			args: args{
				columns:   []Column{{Name: "map_timestamp_int", ColumnFamily: "map_timestamp_int", CQLType: "map<timestamp,int>"}},
				values:    []interface{}{"{1234567890:10}"},
				tableName: "non_primitive_table",
				t: &Translator{
					Logger:              zap.NewNop(),
					SchemaMappingConfig: GetSchemaMappingConfig(),
				},
			},
			want: []Column{
				{Name: "1234567890", ColumnFamily: "map_timestamp_int", CQLType: "int"},
			},
			want1: []interface{}{intVal},
			want2: []string{
				"map_timestamp_int",
			},
			want3:   []Column{},
			wantErr: false,
		},
		{
			name: "Valid Input For Timestamp Boolean",
			args: args{
				columns:   []Column{{Name: "map_timestamp_boolean", ColumnFamily: "map_timestamp_boolean", CQLType: "map<timestamp,boolean>"}},
				values:    []interface{}{"{1234567890:true}"},
				tableName: "non_primitive_table",
				t: &Translator{
					Logger:              zap.NewNop(),
					SchemaMappingConfig: GetSchemaMappingConfig(),
				},
			},
			want: []Column{
				{Name: "1234567890", ColumnFamily: "map_timestamp_boolean", CQLType: "boolean"},
			},
			want1: []interface{}{trueVal},
			want2: []string{
				"map_timestamp_boolean",
			},
			want3:   []Column{},
			wantErr: false,
		},
		{
			name: "Valid Input For Timestamp Timestamp",
			args: args{
				columns:   []Column{{Name: "map_timestamp_timestamp", ColumnFamily: "map_timestamp_timestamp", CQLType: "map<timestamp,timestamp>"}},
				values:    []interface{}{"{1234567890:1234567890}"},
				tableName: "non_primitive_table",
				t: &Translator{
					Logger:              zap.NewNop(),
					SchemaMappingConfig: GetSchemaMappingConfig(),
				},
			},
			want: []Column{
				{Name: "1234567890", ColumnFamily: "map_timestamp_timestamp", CQLType: "timestamp"},
			},
			want1: []interface{}{timestampVal},
			want2: []string{
				"map_timestamp_timestamp",
			},
			want3:   []Column{},
			wantErr: false,
		},
		{
			name: "Valid Input For Timestamp Bigint",
			args: args{
				columns:   []Column{{Name: "map_timestamp_bigint", ColumnFamily: "map_timestamp_bigint", CQLType: "map<timestamp,bigint>"}},
				values:    []interface{}{"{1234567890:1234567890}"},
				tableName: "non_primitive_table",
				t: &Translator{
					Logger:              zap.NewNop(),
					SchemaMappingConfig: GetSchemaMappingConfig(),
				},
			},
			want: []Column{
				{Name: "1234567890", ColumnFamily: "map_timestamp_bigint", CQLType: "bigint"},
			},
			want1: []interface{}{bigintVal},
			want2: []string{
				"map_timestamp_bigint",
			},
			want3:   []Column{},
			wantErr: false,
		},
		{
			name: "Valid Input For Timestamp Float",
			args: args{
				columns:   []Column{{Name: "map_timestamp_float", ColumnFamily: "map_timestamp_float", CQLType: "map<timestamp,float>"}},
				values:    []interface{}{"{1234567890:10.10}"},
				tableName: "non_primitive_table",
				t: &Translator{
					Logger:              zap.NewNop(),
					SchemaMappingConfig: GetSchemaMappingConfig(),
				},
			},
			want: []Column{
				{Name: "1234567890", ColumnFamily: "map_timestamp_float", CQLType: "float"},
			},
			want1: []interface{}{floatVal},
			want2: []string{
				"map_timestamp_float",
			},
			want3:   []Column{},
			wantErr: false,
		},
		{
			name: "Valid Input For Timestamp Double",
			args: args{
				columns:   []Column{{Name: "map_timestamp_double", ColumnFamily: "map_timestamp_double", CQLType: "map<timestamp,double>"}},
				values:    []interface{}{"{1234567890:10.10101}"},
				tableName: "non_primitive_table",
				t: &Translator{
					Logger:              zap.NewNop(),
					SchemaMappingConfig: GetSchemaMappingConfig(),
				},
			},
			want: []Column{
				{Name: "1234567890", ColumnFamily: "map_timestamp_double", CQLType: "double"},
			},
			want1: []interface{}{doubleVal},
			want2: []string{
				"map_timestamp_double",
			},
			want3:   []Column{},
			wantErr: false,
		},
		{
			name: "Valid Input For Set<Text>",
			args: args{
				columns:   []Column{{Name: "set_text", ColumnFamily: "set_text", CQLType: "set<text>"}},
				values:    []interface{}{"{'test'}"},
				tableName: "non_primitive_table",
				t: &Translator{
					Logger:              zap.NewNop(),
					SchemaMappingConfig: GetSchemaMappingConfig(),
				},
			},
			want: []Column{
				{Name: "test", ColumnFamily: "set_text", CQLType: "text"},
			},
			want1: []interface{}{emptyVal},
			want2: []string{
				"set_text",
			},
			want3:   []Column{},
			wantErr: false,
		},
		{
			name: "Valid Input For Set<Boolean>",
			args: args{
				columns:   []Column{{Name: "set_boolean", ColumnFamily: "set_boolean", CQLType: "set<boolean>"}},
				values:    []interface{}{"{'true'}"},
				tableName: "non_primitive_table",
				t: &Translator{
					Logger:              zap.NewNop(),
					SchemaMappingConfig: GetSchemaMappingConfig(),
				},
			},
			want: []Column{
				{Name: "true", ColumnFamily: "set_boolean", CQLType: "boolean"},
			},
			want1: []interface{}{emptyVal},
			want2: []string{
				"set_boolean",
			},
			want3:   []Column{},
			wantErr: false,
		},
		{
			name: "Valid Input For Set<Int>",
			args: args{
				columns:   []Column{{Name: "set_int", ColumnFamily: "set_int", CQLType: "set<int>"}},
				values:    []interface{}{"{'10'}"},
				tableName: "non_primitive_table",
				t: &Translator{
					Logger:              zap.NewNop(),
					SchemaMappingConfig: GetSchemaMappingConfig(),
				},
			},
			want: []Column{
				{Name: "10", ColumnFamily: "set_int", CQLType: "int"},
			},
			want1: []interface{}{emptyVal},
			want2: []string{
				"set_int",
			},
			want3:   []Column{},
			wantErr: false,
		},
		{
			name: "Valid Input For Set<Float>",
			args: args{
				columns:   []Column{{Name: "set_float", ColumnFamily: "set_float", CQLType: "set<float>"}},
				values:    []interface{}{"{10.10}"},
				tableName: "non_primitive_table",
				t: &Translator{
					Logger:              zap.NewNop(),
					SchemaMappingConfig: GetSchemaMappingConfig(),
				},
			},
			want: []Column{
				{Name: "10.10", ColumnFamily: "set_float", CQLType: "float"},
			},
			want1: []interface{}{emptyVal},
			want2: []string{
				"set_float",
			},
			want3:   []Column{},
			wantErr: false,
		},
		{
			name: "Valid Input For Set<Double>",
			args: args{
				columns:   []Column{{Name: "set_double", ColumnFamily: "set_double", CQLType: "set<double>"}},
				values:    []interface{}{"{10.10101}"},
				tableName: "non_primitive_table",
				t: &Translator{
					Logger:              zap.NewNop(),
					SchemaMappingConfig: GetSchemaMappingConfig(),
				},
			},
			want: []Column{
				{Name: "10.10101", ColumnFamily: "set_double", CQLType: "double"},
			},
			want1: []interface{}{emptyVal},
			want2: []string{
				"set_double",
			},
			want3:   []Column{},
			wantErr: false,
		},
		{
			name: "Valid Input For Set<BigInt>",
			args: args{
				columns:   []Column{{Name: "set_bigint", ColumnFamily: "set_bigint", CQLType: "set<bigint>"}},
				values:    []interface{}{"{1234567890123}"},
				tableName: "non_primitive_table",
				t: &Translator{
					Logger:              zap.NewNop(),
					SchemaMappingConfig: GetSchemaMappingConfig(),
				},
			},
			want: []Column{
				{Name: "1234567890123", ColumnFamily: "set_bigint", CQLType: "bigint"},
			},
			want1: []interface{}{emptyVal},
			want2: []string{
				"set_bigint",
			},
			want3:   []Column{},
			wantErr: false,
		},
		{
			name: "Valid Input For Set<Timestamp>",
			args: args{
				columns:   []Column{{Name: "set_timestamp", ColumnFamily: "set_timestamp", CQLType: "set<timestamp>"}},
				values:    []interface{}{"{'1234567890'}"},
				tableName: "non_primitive_table",
				t: &Translator{
					Logger:              zap.NewNop(),
					SchemaMappingConfig: GetSchemaMappingConfig(),
				},
			},
			want: []Column{
				{Name: "1234567890", ColumnFamily: "set_timestamp", CQLType: "timestamp"},
			},
			want1: []interface{}{emptyVal},
			want2: []string{
				"set_timestamp",
			},
			want3:   []Column{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, got2, _, _, err := processCollectionColumnsForRawQueries(tt.args.columns, tt.args.values, tt.args.tableName, tt.args.t, "test_keyspace", nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("processCollectionColumnsForRawQueries() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("processCollectionColumnsForRawQueries() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("processCollectionColumnsForRawQueries() got1 = %v, want %v", got1, tt.want1)
			}
			if !reflect.DeepEqual(got2, tt.want2) {
				t.Errorf("processCollectionColumnsForRawQueries() got2 = %v, want %v", got2, tt.want2)
			}
		})
	}
}

func TestStringToPrimitives(t *testing.T) {
	tests := []struct {
		value    string
		cqlType  string
		expected interface{}
		hasError bool
	}{
		{"123", "int", int32(123), false},
		{"not_an_int", "int", nil, true},
		{"123456789", "bigint", int64(123456789), false},
		{"not_a_bigint", "bigint", nil, true},
		{"3.14", "float", float32(3.14), false},
		{"not_a_float", "float", nil, true},
		{"3.1415926535", "double", float64(3.1415926535), false},
		{"not_a_double", "double", nil, true},
		{"true", "boolean", true, false},
		{"false", "boolean", false, false},
		{"not_a_boolean", "boolean", nil, true},
		{"blob_data", "blob", "blob_data", false},
		{"hello", "text", "hello", false},
		{"123", "unsupported_type", nil, true},
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
		cqlType   string
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
			args:    args{"abc", "int", primitive.ProtocolVersion4},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Invalid bigint",
			args:    args{"abc", "bigint", primitive.ProtocolVersion4},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Invalid float",
			args:    args{"abc", "float", primitive.ProtocolVersion4},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Invalid double",
			args:    args{"abc", "double", primitive.ProtocolVersion4},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Invalid boolean",
			args:    args{"abc", "boolean", primitive.ProtocolVersion4},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Invalid timestamp",
			args:    args{"abc", "timestamp", primitive.ProtocolVersion4},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Unsupported type",
			args:    args{"123", "unsupported", primitive.ProtocolVersion4},
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
	textValue, _ := formatValues("test", "text", primitive.ProtocolVersion4)
	trueVal, _ := formatValues("true", "boolean", primitive.ProtocolVersion4)

	valuesTextBool := map[string]bool{"test": true}
	textBytesTextBool, _ := proxycore.EncodeType(mapTypeTextBool, primitive.ProtocolVersion4, valuesTextBool)

	valuesTextInt := map[string]int{"test": 42}
	textBytesTextInt, _ := proxycore.EncodeType(mapTypeTextInt, primitive.ProtocolVersion4, valuesTextInt)
	intValue, _ := formatValues("42", "int", primitive.ProtocolVersion4)

	valuesTextFloat := map[string]float32{"test": 3.14}
	textBytesTextFloat, _ := proxycore.EncodeType(mapTypeTextFloat, primitive.ProtocolVersion4, valuesTextFloat)
	floatValue, _ := formatValues("3.14", "float", primitive.ProtocolVersion4)

	valuesTextDouble := map[string]float64{"test": 6.283}
	textBytesTextDouble, _ := proxycore.EncodeType(mapTypeTextDouble, primitive.ProtocolVersion4, valuesTextDouble)
	doubleValue, _ := formatValues("6.283", "double", primitive.ProtocolVersion4)

	valuesTextTimestamp := map[string]time.Time{"test": time.Unix(1633046400, 0)} // Example timestamp
	textBytesTextTimestamp, _ := proxycore.EncodeType(mapTypeTextTimestamp, primitive.ProtocolVersion4, valuesTextTimestamp)
	timestampValue, _ := formatValues("1633046400000", "timestamp", primitive.ProtocolVersion4) // Example in milliseconds

	valuesTimestampBoolean := map[time.Time]bool{
		time.Unix(1633046400, 0): true,
	}
	textBytesTimestampBoolean, _ := proxycore.EncodeType(mapTypeTimestampBoolean, primitive.ProtocolVersion4, valuesTimestampBoolean)
	timestampBooleanValue, _ := formatValues("true", "boolean", primitive.ProtocolVersion4)

	valuesTimestampText := map[time.Time]string{
		time.Unix(1633046400, 0): "example_text", // Example timestamp as key with text value
	}
	textBytesTimestampText, _ := proxycore.EncodeType(mapTypeTimestampText, primitive.ProtocolVersion4, valuesTimestampText)
	timestampTextValue, _ := formatValues("example_text", "text", primitive.ProtocolVersion4)

	valuesTimestampInt := map[time.Time]int{
		time.Unix(1633046400, 0): 42, // Example timestamp as key with int value
	}
	textBytesTimestampInt, _ := proxycore.EncodeType(mapTypeTimestampInt, primitive.ProtocolVersion4, valuesTimestampInt)
	timestampIntValue, _ := formatValues("42", "int", primitive.ProtocolVersion4)

	valuesTimestampFloat := map[time.Time]float32{
		time.Unix(1633046400, 0): 3.14, // Example timestamp as key with float value
	}
	textBytesTimestampFloat, _ := proxycore.EncodeType(mapTypeTimestampFloat, primitive.ProtocolVersion4, valuesTimestampFloat)
	timestampFloatValue, _ := formatValues("3.14", "float", primitive.ProtocolVersion4)

	valuesTimestampBigint := map[time.Time]int64{
		time.Unix(1633046400, 0): 1234567890123, // Example timestamp as key with bigint value
	}
	textBytesTimestampBigint, _ := proxycore.EncodeType(mapTypeTimestampBigint, primitive.ProtocolVersion4, valuesTimestampBigint)
	timestampBigintValue, _ := formatValues("1234567890123", "bigint", primitive.ProtocolVersion4)

	valuesTimestampDouble := map[time.Time]float64{
		time.Unix(1633046400, 0): 6.283, // Example timestamp as key with double value
	}
	textBytesTimestampDouble, _ := proxycore.EncodeType(mapTypeTimestampDouble, primitive.ProtocolVersion4, valuesTimestampDouble)
	timestampDoubleValue, _ := formatValues("6.283", "double", primitive.ProtocolVersion4)

	valuesTimestampTimestamp := map[time.Time]time.Time{
		time.Unix(1633046400, 0): time.Unix(1633126400, 0), // Example timestamp as key with timestamp value
	}
	textBytesTimestampTimestamp, _ := proxycore.EncodeType(mapTypeTimestampTimestamp, primitive.ProtocolVersion4, valuesTimestampTimestamp)
	timestampTimestampValue, _ := formatValues("1633126400000", "timestamp", primitive.ProtocolVersion4) // Example in milliseconds

	valuesTextBigint := map[string]int64{"test": 1234567890123}
	textBytesTextBigint, _ := proxycore.EncodeType(mapTypeTextBigint, primitive.ProtocolVersion4, valuesTextBigint)
	bigintValue, _ := formatValues("1234567890123", "bigint", primitive.ProtocolVersion4)

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

	emptyVal, _ := formatValues("", "text", primitive.ProtocolVersion4)
	tests := []struct {
		name        string
		columns     []Column
		values      []*primitive.Value
		tableName   string
		protocolV   primitive.ProtocolVersion
		primaryKeys []string
		translator  *Translator
		want        []Column
		want1       []interface{}
		want2       map[string]interface{}
		want3       int
		want4       []string
		wantErr     bool
	}{
		{
			name: "Valid Input For Timestamp Float",
			columns: []Column{
				{Name: "map_timestamp_float", ColumnFamily: "map_timestamp_float", CQLType: "map<timestamp,float>"},
			},
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
			want: []Column{
				{Name: "1633046400000", ColumnFamily: "map_timestamp_float", CQLType: "float"},
			},
			want1:   []interface{}{timestampFloatValue},
			want2:   map[string]interface{}{},
			want3:   0,
			want4:   []string{"map_timestamp_float"},
			wantErr: false,
		},
		{
			name: "Valid Input For Text Timestamp",
			columns: []Column{
				{Name: "map_text_timestamp", ColumnFamily: "map_text_timestamp", CQLType: "map<text,timestamp>"},
			},
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
			want: []Column{
				{Name: "test", ColumnFamily: "map_text_timestamp", CQLType: "bigint"},
			},
			want1:   []interface{}{timestampValue},
			want2:   map[string]interface{}{},
			want3:   0,
			want4:   []string{"map_text_timestamp"},
			wantErr: false,
		},
		{
			name: "Valid Input For Timestamp Text",
			columns: []Column{
				{Name: "map_timestamp_text", ColumnFamily: "map_timestamp_text", CQLType: "map<timestamp,text>"},
			},
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
			want: []Column{
				{Name: "1633046400000", ColumnFamily: "map_timestamp_text", CQLType: "text"},
			},
			want1:   []interface{}{timestampTextValue},
			want2:   map[string]interface{}{},
			want3:   0,
			want4:   []string{"map_timestamp_text"},
			wantErr: false,
		},
		{
			name: "Valid Input For Set Timestamp",
			columns: []Column{
				{Name: "set_timestamp", ColumnFamily: "set_timestamp", CQLType: "set<timestamp>"},
			},
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
			want: []Column{
				{Name: "1633046400", ColumnFamily: "set_timestamp", CQLType: "bigint"},
			},
			want1:   []interface{}{emptyVal},
			want2:   map[string]interface{}{},
			want3:   0,
			want4:   []string{"set_timestamp"},
			wantErr: false,
		},
		{
			name: "Valid Input For Set Double",
			columns: []Column{
				{Name: "set_double", ColumnFamily: "set_double", CQLType: "set<double>"},
			},
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
			want: []Column{
				{Name: "6.283", ColumnFamily: "set_double", CQLType: "double"},
			},
			want1:   []interface{}{emptyVal},
			want2:   map[string]interface{}{},
			want3:   0,
			want4:   []string{"set_double"},
			wantErr: false,
		},
		{
			name: "Valid Input For Set Float",
			columns: []Column{
				{Name: "set_float", ColumnFamily: "set_float", CQLType: "set<float>"},
			},
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
			want: []Column{
				{Name: "6.283", ColumnFamily: "set_float", CQLType: "float"},
			},
			want1:   []interface{}{emptyVal},
			want2:   map[string]interface{}{},
			want3:   0,
			want4:   []string{"set_float"},
			wantErr: false,
		},
		{
			name: "Valid Input For Set Text",
			columns: []Column{
				{Name: "set_text", ColumnFamily: "set_text", CQLType: "set<text>"},
			},
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
			want: []Column{
				{Name: "test", ColumnFamily: "set_text", CQLType: "text"},
			},
			want1:   []interface{}{emptyVal},
			want2:   map[string]interface{}{},
			want3:   0,
			want4:   []string{"set_text"},
			wantErr: false,
		},
		{
			name: "Valid Input For Set BigInt",
			columns: []Column{
				{Name: "set_bigint", ColumnFamily: "set_bigint", CQLType: "set<bigint>"},
			},
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
			want: []Column{
				{Name: "12372432764", ColumnFamily: "set_bigint", CQLType: "bigint"},
			},
			want1:   []interface{}{emptyVal},
			want2:   map[string]interface{}{},
			want3:   0,
			want4:   []string{"set_bigint"},
			wantErr: false,
		},
		{
			name: "Valid Input For Set Int",
			columns: []Column{
				{Name: "set_int", ColumnFamily: "set_int", CQLType: "set<int>"},
			},
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
			want: []Column{
				{Name: "12", ColumnFamily: "set_int", CQLType: "int"},
			},
			want1:   []interface{}{emptyVal},
			want2:   map[string]interface{}{},
			want3:   0,
			want4:   []string{"set_int"},
			wantErr: false,
		},
		{
			name: "Valid Input For Timestamp Timestamp",
			columns: []Column{
				{Name: "map_timestamp_timestamp", ColumnFamily: "map_timestamp_timestamp", CQLType: "map<timestamp,timestamp>"},
			},
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
			want: []Column{
				{Name: "1633046400000", ColumnFamily: "map_timestamp_timestamp", CQLType: "bigint"},
			},
			want1:   []interface{}{timestampTimestampValue},
			want2:   map[string]interface{}{},
			want3:   0,
			want4:   []string{"map_timestamp_timestamp"},
			wantErr: false,
		},
		{
			name: "Valid Input For Text Bigint",
			columns: []Column{
				{Name: "map_text_bigint", ColumnFamily: "map_text_bigint", CQLType: "map<text,bigint>"},
			},
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
			want: []Column{
				{Name: "test", ColumnFamily: "map_text_bigint", CQLType: "bigint"},
			},
			want1:   []interface{}{bigintValue},
			want2:   map[string]interface{}{},
			want3:   0,
			want4:   []string{"map_text_bigint"},
			wantErr: false,
		},
		{
			name: "Valid Input For Timestamp Int",
			columns: []Column{
				{Name: "map_timestamp_int", ColumnFamily: "map_timestamp_int", CQLType: "map<timestamp,int>"},
			},
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
			want: []Column{
				{Name: "1633046400000", ColumnFamily: "map_timestamp_int", CQLType: "int"},
			},
			want1:   []interface{}{timestampIntValue},
			want2:   map[string]interface{}{},
			want3:   0,
			want4:   []string{"map_timestamp_int"},
			wantErr: false,
		},
		{
			name: "Valid Input For Set Boolean",
			columns: []Column{
				{Name: "set_boolean", ColumnFamily: "set_boolean", CQLType: "set<boolean>"},
			},
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
			want: []Column{
				{Name: "true", ColumnFamily: "set_boolean", CQLType: "boolean"},
			},
			want1:   []interface{}{emptyVal},
			want2:   map[string]interface{}{},
			want3:   0,
			want4:   []string{"set_boolean"},
			wantErr: false,
		},
		{
			name: "Valid Input For Text Boolean",
			columns: []Column{
				{Name: "map_text_boolean", ColumnFamily: "map_text_boolean", CQLType: "map<text,boolean>"},
			},
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
			want: []Column{
				{Name: "test", ColumnFamily: "map_text_boolean", CQLType: "boolean"},
			},
			want1:   []interface{}{trueVal},
			want2:   map[string]interface{}{},
			want3:   0,
			want4:   []string{"map_text_boolean"},
			wantErr: false,
		},
		{
			name: "Valid Input For Text Text",
			columns: []Column{
				{Name: "map_text_text", ColumnFamily: "map_text_text", CQLType: "map<text,text>"},
			},
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
			want: []Column{
				{Name: "test", ColumnFamily: "map_text_text", CQLType: "text"},
			},
			want1:   []interface{}{textValue},
			want2:   map[string]interface{}{},
			want3:   0,
			want4:   []string{"map_text_text"},
			wantErr: false,
		},
		{
			name: "Valid Input For Text Int",
			columns: []Column{
				{Name: "map_text_int", ColumnFamily: "map_text_int", CQLType: "map<text,int>"},
			},
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
			want: []Column{
				{Name: "test", ColumnFamily: "map_text_int", CQLType: "int"},
			},
			want1:   []interface{}{intValue},
			want2:   map[string]interface{}{},
			want3:   0,
			want4:   []string{"map_text_int"},
			wantErr: false,
		},
		{
			name: "Valid Input For Text Float",
			columns: []Column{
				{Name: "map_text_float", ColumnFamily: "map_text_float", CQLType: "map<text,float>"},
			},
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
			want: []Column{
				{Name: "test", ColumnFamily: "map_text_float", CQLType: "float"},
			},
			want1:   []interface{}{floatValue},
			want2:   map[string]interface{}{},
			want3:   0,
			want4:   []string{"map_text_float"},
			wantErr: false,
		},
		{
			name: "Valid Input For Text Double",
			columns: []Column{
				{Name: "map_text_double", ColumnFamily: "map_text_double", CQLType: "map<text,double>"},
			},
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
			want: []Column{
				{Name: "test", ColumnFamily: "map_text_double", CQLType: "double"},
			},
			want1:   []interface{}{doubleValue},
			want2:   map[string]interface{}{},
			want3:   0,
			want4:   []string{"map_text_double"},
			wantErr: false,
		},
		{
			name: "Valid Input For Timestamp Boolean",
			columns: []Column{
				{Name: "map_timestamp_boolean", ColumnFamily: "map_timestamp_boolean", CQLType: "map<timestamp,boolean>"},
			},
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
			want: []Column{
				{Name: "1633046400000", ColumnFamily: "map_timestamp_boolean", CQLType: "boolean"},
			},
			want1:   []interface{}{timestampBooleanValue},
			want2:   map[string]interface{}{},
			want3:   0,
			want4:   []string{"map_timestamp_boolean"},
			wantErr: false,
		},
		{
			name: "Valid Input For Timestamp Double",
			columns: []Column{
				{Name: "map_timestamp_double", ColumnFamily: "map_timestamp_double", CQLType: "map<timestamp,double>"},
			},
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
			want: []Column{
				{Name: "1633046400000", ColumnFamily: "map_timestamp_double", CQLType: "double"},
			},
			want1:   []interface{}{timestampDoubleValue},
			want2:   map[string]interface{}{},
			want3:   0,
			want4:   []string{"map_timestamp_double"},
			wantErr: false,
		},
		{
			name: "Valid Input For Timestamp Bigint",
			columns: []Column{
				{Name: "map_timestamp_bigint", ColumnFamily: "map_timestamp_bigint", CQLType: "map<timestamp,bigint>"},
			},
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
			want: []Column{
				{Name: "1633046400000", ColumnFamily: "map_timestamp_bigint", CQLType: "bigint"},
			},
			want1:   []interface{}{timestampBigintValue},
			want2:   map[string]interface{}{},
			want3:   0,
			want4:   []string{"map_timestamp_bigint"},
			wantErr: false,
		},
		{
			name: "Invalid Input For Set Bigint",
			columns: []Column{
				{Name: "set_bigint", ColumnFamily: "set_bigint", CQLType: "set<bigint>"},
			},
			values: []*primitive.Value{
				{Contents: setBytesBoolean}, // Invalid type
			},
			tableName:   "non_primitive_table",
			protocolV:   primitive.ProtocolVersion4,
			primaryKeys: []string{},
			translator: &Translator{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			want:    nil,
			want1:   nil,
			want2:   nil,
			want3:   0,
			want4:   nil,
			wantErr: true,
		},
		{
			name: "Invalid Input For Text Bigint",
			columns: []Column{
				{Name: "map_text_bigint", ColumnFamily: "map_text_bigint", CQLType: "map<text,bigint>"},
			},
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
			want:    nil,
			want1:   nil,
			want2:   nil,
			want3:   0,
			want4:   nil,
			wantErr: true,
		},
		{
			name: "Invalid Input For Set Int",
			columns: []Column{
				{Name: "set_int", ColumnFamily: "set_int", CQLType: "set<int>"},
			},
			values: []*primitive.Value{
				{Contents: setBytesBoolean}, // Invalid type
			},
			tableName:   "non_primitive_table",
			protocolV:   primitive.ProtocolVersion4,
			primaryKeys: []string{},
			translator: &Translator{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			want:    nil,
			want1:   nil,
			want2:   nil,
			want3:   0,
			want4:   nil,
			wantErr: true,
		},
		{
			name: "Invalid Input For Set Float",
			columns: []Column{
				{Name: "set_float", ColumnFamily: "set_float", CQLType: "set<float>"},
			},
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
			want:    nil,
			want1:   nil,
			want2:   nil,
			want3:   0,
			want4:   nil,
			wantErr: true,
		},
		{
			name: "Invalid Input For Set Double",
			columns: []Column{
				{Name: "set_double", ColumnFamily: "set_double", CQLType: "set<double>"},
			},
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
			want:    nil,
			want1:   nil,
			want2:   nil,
			want3:   0,
			want4:   nil,
			wantErr: true,
		},
		{
			name: "Invalid Input For Set Timestamp",
			columns: []Column{
				{Name: "set_timestamp", ColumnFamily: "set_timestamp", CQLType: "set<timestamp>"},
			},
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
			want:    nil,
			want1:   nil,
			want2:   nil,
			want3:   0,
			want4:   nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, got2, got3, got4, _, err := processCollectionColumnsForPrepareQueries(tt.columns, tt.values, tt.tableName, tt.protocolV, tt.primaryKeys, tt.translator, "test_keyspace", nil)

			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("got1 = %v, want %v", got1, tt.want1)
			}
			if !reflect.DeepEqual(got2, tt.want2) {
				t.Errorf("got2 = %v, want %v", got2, tt.want2)
			}
			if got3 != tt.want3 {
				t.Errorf("processCollectionColumnsForPrepareQueries() got3 = %v, want %v", got3, tt.want3)
			}
			if !reflect.DeepEqual(got4, tt.want4) {
				t.Errorf("processCollectionColumnsForPrepareQueries() got4 = %v, want %v", got4, tt.want4)
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
			name:  "Valid timestamp input",
			input: "1634232345000000",
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
		columns        []Column
		values         []interface{}
		prependColumns []string
		expectedMeta   map[string]*ComplexUpdateMeta
		expectedErr    error
	}{
		{
			name: "successful collection update for map and list",
			columns: []Column{
				{Name: "column8", CQLType: "map<text, boolean>"},
				{Name: "list_text", CQLType: "list<text>"},
			},
			values: []interface{}{
				"column8+{key:?}",
				"list_text+?",
			},
			prependColumns: []string{"list_text"},
			expectedMeta: map[string]*ComplexUpdateMeta{
				"column8": {
					Append:           true,
					key:              "key",
					ExpectedDatatype: datatype.Boolean,
				},
				"list_text": {
					key:         "",
					Append:      false,
					PrependList: true,
				},
			},
			expectedErr: nil,
		},
		{
			name: "non-collection column should be skipped",
			columns: []Column{
				{Name: "column1", CQLType: "text"},
			},
			values: []interface{}{
				"column1+value",
			},
			prependColumns: []string{},
			expectedMeta:   map[string]*ComplexUpdateMeta{},
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
					if !compareComplexUpdateMeta(expectedComplexUpdate, actualComplexUpdate) {
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

func TestTransformPrepareQueryForPrependList(t *testing.T) {
	tests := []struct {
		name            string
		query           string
		expectedQuery   string
		expectedColumns []string
	}{
		{
			name:            "No WHERE clause",
			query:           "UPDATE table SET column1 = ?",
			expectedQuery:   "UPDATE table SET column1 = ?",
			expectedColumns: nil,
		},
		{
			name:            "No prepend operation",
			query:           "UPDATE table SET column1 = ? WHERE id = ?",
			expectedQuery:   "UPDATE table SET column1 = ? WHERE id = ?",
			expectedColumns: nil,
		},
		{
			name:            "Prepend operation detected",
			query:           "UPDATE table SET column1 = ? + column1 WHERE id = ?",
			expectedQuery:   "UPDATE table SET column1 = column1 + ? WHERE id = ?",
			expectedColumns: []string{"column1"},
		},
		{
			name:            "Multiple assignments with prepend",
			query:           "UPDATE table SET column1 = ? + column1, column2 = ? WHERE id = ?",
			expectedQuery:   "UPDATE table SET column1 = column1 + ?, column2 = ? WHERE id = ?",
			expectedColumns: []string{"column1"},
		},
		{
			name:            "Multiple prepend operations",
			query:           "UPDATE table SET column1 = ? + column1, column2 = ? + column2 WHERE id = ?",
			expectedQuery:   "UPDATE table SET column1 = column1 + ?, column2 = column2 + ? WHERE id = ?",
			expectedColumns: []string{"column1", "column2"},
		},
		{
			name:            "Prepend operation with different variable names",
			query:           "UPDATE table SET column1 = ? + column2 WHERE id = ?",
			expectedQuery:   "UPDATE table SET column1 = ? + column2 WHERE id = ?",
			expectedColumns: nil,
		},
		{
			name:            "Prepend operation with quotes",
			query:           `UPDATE table SET column1 = '?' + column1 WHERE id = ?`,
			expectedQuery:   `UPDATE table SET column1 = column1 + '?' WHERE id = ?`,
			expectedColumns: []string{"column1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modifiedQuery, columns := TransformPrepareQueryForPrependList(tt.query)
			if modifiedQuery != tt.expectedQuery {
				t.Errorf("TransformPrepareQueryForPrependList() modifiedQuery = %v, want %v", modifiedQuery, tt.expectedQuery)
			}
			if !reflect.DeepEqual(columns, tt.expectedColumns) {
				t.Errorf("TransformPrepareQueryForPrependList() columns = %v, want %v", columns, tt.expectedColumns)
			}
		})
	}
}

// compareComplexUpdateMeta checks if two ComplexUpdateMeta structures are equal.
func compareComplexUpdateMeta(expected, actual *ComplexUpdateMeta) bool {
	return expected.Append == actual.Append &&
		expected.key == actual.key &&
		expected.PrependList == actual.PrependList &&
		expected.UpdateListIndex == actual.UpdateListIndex &&
		expected.Delete == actual.Delete &&
		expected.ListDelete == actual.ListDelete &&
		reflect.DeepEqual(expected.ExpectedDatatype, actual.ExpectedDatatype)
}
