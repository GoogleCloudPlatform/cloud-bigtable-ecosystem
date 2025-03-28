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
	"errors"
	"reflect"
	"testing"

	schemaMapping "github.com/ollionorg/cassandra-to-bigtable-proxy/schema-mapping"
	cql "github.com/ollionorg/cassandra-to-bigtable-proxy/translator/cqlparser"
	"go.uber.org/zap"
)

func TestTranslator_TranslateSelectQuerytoBigtable(t *testing.T) {
	type fields struct {
		Logger *zap.Logger
	}
	type args struct {
		query string
	}

	inputRawQuery := `select column1, column2, column3 from  test_keyspace.test_table
	where column1 = 'test' AND column3='true'
	AND column5 <= '2015-05-03 13:30:54.234' AND column6 >= '123'
	AND column9 > '-10000000' LIMIT 20000;;`
	timeStamp, _ := parseTimestamp("2015-05-03 13:30:54.234")

	inputPreparedQuery := `select column1, column2, column3 from  test_keyspace.test_table
	where column1 = '?' AND column2='?' AND column3='?'
	AND column5='?' AND column6='?'
	AND column9='?';`
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *SelectQueryMap
		wantErr bool
	}{
		{
			name: "Writetime Query without as keyword",
			args: args{
				query: `select column1, WRITETIME(column2) from test_keyspace.test_table where column1 = 'test';`,
			},
			wantErr: false,
			want: &SelectQueryMap{
				Query:           "SELECT cf1['column1'],WRITE_TIMESTAMP(cf1, 'column2') FROM test_table WHERE cf1['column1'] = @value1;",
				QueryType:       "select",
				TranslatedQuery: "SELECT cf1['column1'],WRITE_TIMESTAMP(cf1, 'column2') FROM test_table WHERE cf1['column1'] = @value1;",
				Table:           "test_table",
				Keyspace:        "test_keyspace",
				Clauses: []Clause{
					{
						Column:       "column1",
						Operator:     "=",
						Value:        "@value1",
						IsPrimaryKey: true,
					},
				},
				Limit: Limit{
					IsLimit: false,
				},
				OrderBy: OrderBy{
					IsOrderBy: false,
				},
				Params: map[string]interface{}{
					"value1": "test",
				},
				ParamKeys: []string{"value1"},
			},
		},
		{
			name: "Writetime Query with as keyword",
			args: args{
				query: `select column1, WRITETIME(column2) as name from test_keyspace.test_table where column1 = 'test';`,
			},
			wantErr: false,
			want: &SelectQueryMap{
				Query:           "SELECT cf1['column1'],WRITE_TIMESTAMP(cf1, 'column2') as name FROM test_table WHERE cf1['column1'] = @value1;",
				QueryType:       "select",
				TranslatedQuery: "SELECT cf1['column1'],WRITE_TIMESTAMP(cf1, 'column2') as name FROM test_table WHERE cf1['column1'] = @value1;",
				Table:           "test_table",
				Keyspace:        "test_keyspace",
				Clauses: []Clause{
					{
						Column:       "column1",
						Operator:     "=",
						Value:        "@value1",
						IsPrimaryKey: true,
					},
				},
				Limit: Limit{
					IsLimit: false,
				},
				OrderBy: OrderBy{
					IsOrderBy: false,
				},
				Params: map[string]interface{}{
					"value1": "test",
				},
				ParamKeys: []string{"value1"},
			},
		},
		{
			name: "As Query",
			args: args{
				query: `select column2 as name from test_keyspace.test_table where column1 = 'test';`,
			},
			wantErr: false,
			want: &SelectQueryMap{
				Query:           "select column2 as name from test_keyspace.test_table where column1 = 'test'",
				QueryType:       "select",
				TranslatedQuery: "SELECT cf1['column2'] as name FROM test_table WHERE cf1['column1'] = @value1;",
				Table:           "test_table",
				Keyspace:        "test_keyspace",
				Clauses: []Clause{
					{
						Column:       "column1",
						Operator:     "=",
						Value:        "@value1",
						IsPrimaryKey: true,
					},
				},
				Limit: Limit{
					IsLimit: false,
				},
				OrderBy: OrderBy{
					IsOrderBy: false,
				},
				Params: map[string]interface{}{
					"value1": "test",
				},
				ParamKeys: []string{"value1"},
			},
		},
		{
			name: "Query without Columns",
			args: args{
				query: `select from key_space.test_table where column1 = 'test'`,
			},
			wantErr: true,
			want:    nil,
		},
		{
			name: "test for raw query when column name not exist in schema mapping table",
			args: args{
				query: "select column101, column2, column3 from  key_space.test_table where column1 = 'test' AND column3='true' AND column5 <= '2015-05-03 13:30:54.234' AND column6 >= '123' AND column9 > '-10000000' LIMIT 20000;",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Query without keyspace name",
			args: args{
				query: `select column1, column2 from test_table where column1 = '?' and column1 in ('?', '?');`,
			},
			wantErr: true,
			want:    nil,
		},
		{
			name: "MALFORMED QUERY",
			args: args{
				query: "MALFORMED QUERY",
			},
			wantErr: true,
			want:    nil,
		},
		{
			name: "test for raw query success",
			args: args{
				query: inputRawQuery,
			},
			want: &SelectQueryMap{
				Query:           inputRawQuery,
				QueryType:       "select",
				TranslatedQuery: "SELECT cf1['column1'],cf1['column2'],cf1['column3'] FROM test_table WHERE cf1['column1'] = @value1 AND TO_BOOL(cf1['column3']) = @value2 AND TO_TIME(cf1['column5']) <= @value3 AND TO_INT32(cf1['column6']) >= @value4 AND TO_INT64(cf1['column9']) > @value5 LIMIT 20000;",
				Table:           "test_table",
				Keyspace:        "test_keyspace",
				Clauses: []Clause{
					{
						Column:       "column1",
						Operator:     "=",
						Value:        "@value1",
						IsPrimaryKey: true,
					},
					{
						Column:   "column3",
						Operator: "=",
						Value:    "@value2",
					},
					{
						Column:   "column5",
						Operator: "<=",
						Value:    "@value3",
					},
					{
						Column:   "column6",
						Operator: ">=",
						Value:    "@value4",
					},
					{
						Column:   "column9",
						Operator: ">",
						Value:    "@value5",
					},
				},
				Limit: Limit{
					IsLimit: true,
					Count:   "2000",
				},
				OrderBy: OrderBy{
					IsOrderBy: false,
				},
				Params: map[string]interface{}{
					"value1": "test",
					"value2": true,
					"value3": timeStamp,
					"value4": int32(123),
					"value5": int64(-10000000),
				},
				ParamKeys: []string{"value1", "value2", "value3", "value4", "value5"},
			},
			wantErr: false,
		},
		{
			name: "test for prepared query success",
			args: args{
				query: inputPreparedQuery,
			},
			want: &SelectQueryMap{
				Query:           inputPreparedQuery,
				QueryType:       "select",
				TranslatedQuery: "SELECT cf1['column1'],cf1['column2'],cf1['column3'] FROM test_table WHERE cf1['column1'] = @value1 AND TO_BLOB(cf1['column2']) = @value2 AND TO_BOOL(cf1['column3']) = @value3 AND TO_TIME(cf1['column5']) = @value4 AND TO_INT32(cf1['column6']) = @value5 AND TO_INT64(cf1['column9']) = @value6;",
				Table:           "test_table",
				Keyspace:        "test_keyspace",
				Clauses: []Clause{
					{
						Column:       "column1",
						Operator:     "=",
						Value:        "@value1",
						IsPrimaryKey: true,
					},
					{
						Column:   "column2",
						Operator: "=",
						Value:    "@value2",
					},
					{
						Column:   "column3",
						Operator: "=",
						Value:    "@value3",
					},
					{
						Column:   "column5",
						Operator: "=",
						Value:    "@value4",
					},
					{
						Column:   "column6",
						Operator: "=",
						Value:    "@value5",
					},
					{
						Column:   "column9",
						Operator: "=",
						Value:    "@value6",
					},
				},
				Limit: Limit{
					IsLimit: true,
					Count:   "2000",
				},
				OrderBy: OrderBy{
					IsOrderBy: false,
				},
				ParamKeys: []string{"value1", "value2", "value3", "value4", "value5", "value6"},
			},
			wantErr: false,
		},
		{
			name: "test for query without clause success",
			args: args{
				query: `select column1, column2, column3 from test_keyspace.test_table ORDER BY column1 LIMIT 20000;`,
			},
			want: &SelectQueryMap{
				Query:           `select column1, column2, column3 from test_keyspace.test_table ORDER BY column1 LIMIT 20000;`,
				QueryType:       "select",
				TranslatedQuery: "SELECT cf1['column1'],cf1['column2'],cf1['column3'] FROM test_table ORDER BY cf1['column1'] asc LIMIT 20000;",
				Table:           "test_table",
				Keyspace:        "test_keyspace",
				ColumnMeta: ColumnMeta{
					Star:   false,
					Column: []schemaMapping.SelectedColumns{{Name: "column1"}, {Name: "column2"}, {Name: "column3"}},
				},
				Limit: Limit{
					IsLimit: true,
					Count:   "2000",
				},
				OrderBy: OrderBy{
					IsOrderBy: true,
					Column:    "column1",
					Operation: "asc",
				},
			},
			wantErr: false,
		},
		{
			name: "error at Column parsing",
			args: args{
				query: "select  from table;",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "error at Clause parsing when column type not found",
			args: args{
				query: "select * from test_keyspace.table_name where name=test;",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "error at Clause parsing when value invalid",
			args: args{
				query: "select * from test_keyspace.test_table where column1=",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "test with IN operator raw query",
			args: args{
				query: `select column1, column2 from test_keyspace.test_table where column1 = 'test' and column1 in ('abc', 'xyz');`,
			},
			wantErr: false,
			want: &SelectQueryMap{
				TranslatedQuery: "SELECT cf1['column1'],cf1['column2'] FROM test_table WHERE cf1['column1'] = @value1 AND cf1['column1'] IN UNNEST(@value2);",
				Keyspace:        "test_keyspace",
				Params: map[string]interface{}{
					"value1": "test",
					"value2": []string{"abc", "xyz"},
				},
				ParamKeys: []string{"value1", "value2"},
				Clauses: []Clause{
					{
						Column:       "column1",
						Operator:     "=",
						Value:        "@value1",
						IsPrimaryKey: true,
					},
					{
						Column:       "column1",
						Operator:     "IN",
						Value:        "@value2",
						IsPrimaryKey: true,
					},
				},
			},
		},
		{
			name: "test with IN operator prepared query",
			args: args{
				query: `select column1, column2 from test_keyspace.test_table where column1 = '?' and column1 in ('?', '?');`,
			},
			wantErr: false,
			want: &SelectQueryMap{
				TranslatedQuery: "SELECT cf1['column1'],cf1['column2'] FROM test_table WHERE cf1['column1'] = @value1 AND cf1['column1'] IN UNNEST(@value2);",
				Keyspace:        "test_keyspace",
				Params:          map[string]interface{}{},
				ParamKeys:       []string{"value1", "value2"},
				Clauses: []Clause{
					{
						Column:       "column1",
						Operator:     "=",
						Value:        "@value1",
						IsPrimaryKey: true,
					},
					{
						Column:       "column1",
						Operator:     "IN",
						Value:        "@value2",
						IsPrimaryKey: true,
					},
				},
			},
		},
		{
			name: "Empty Query",
			args: args{
				query: "",
			},
			wantErr: true,
			want:    nil,
		},
		{
			name: "Query Without Select Object",
			args: args{
				query: "UPDATE table_name SET column1 = 'new_value1', column2 = 'new_value2' WHERE condition;",
			},
			wantErr: true,
			want:    nil,
		},
		{
			name: "query with missing limit value",
			args: args{
				query: `select column1, column2, column3 from test_keyspace.test_table ORDER BY column1 LIMIT;`,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "query with LIMIT before ORDER BY",
			args: args{
				query: `select column1, column2, column3 from test_keyspace.test_table LIMIT 100 ORDER BY column1;`,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "query with non-existent column in ORDER BY",
			args: args{
				query: `select column1, column2, column3 from test_keyspace.test_table ORDER BY column12343 LIMIT 100;`,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "query with negative LIMIT value",
			args: args{
				query: `select column1, column2, column3 from test_keyspace.test_table ORDER BY column1 LIMIT -100;`,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "query with duplicate negative LIMIT value (potential duplicate test case)",
			args: args{
				query: `select column1, column2, column3 from test_keyspace.test_table ORDER BY column1 LIMIT -100;`,
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schemaMapping := &schemaMapping.SchemaMappingConfig{
				Logger:             tt.fields.Logger,
				TablesMetaData:     mockSchemaMappingConfig,
				PkMetadataCache:    mockPkMetadata,
				SystemColumnFamily: "cf1",
			}
			tr := &Translator{
				Logger:              tt.fields.Logger,
				SchemaMappingConfig: schemaMapping,
			}
			got, err := tr.TranslateSelectQuerytoBigtable(tt.args.query)
			if (err != nil) != tt.wantErr {
				t.Errorf("Translator.TranslateSelectQuerytoBigtable() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != nil && !reflect.DeepEqual(got.TranslatedQuery, tt.want.TranslatedQuery) {
				t.Errorf("Translator.TranslateSelectQuerytoBigtable() = %v, want %v", got.TranslatedQuery, tt.want.TranslatedQuery)
			}

			if got != nil && len(got.Params) > 0 && !reflect.DeepEqual(got.Params, tt.want.Params) {
				t.Errorf("Translator.TranslateSelectQuerytoBigtable() = %v, want %v", got.Params, tt.want.Params)
			}

			if got != nil && len(got.ParamKeys) > 0 && !reflect.DeepEqual(got.ParamKeys, tt.want.ParamKeys) {
				t.Errorf("Translator.TranslateSelectQuerytoBigtable() = %v, want %v", got.ParamKeys, tt.want.ParamKeys)
			}

			if got != nil && len(got.Clauses) > 0 && !reflect.DeepEqual(got.Clauses, tt.want.Clauses) {
				t.Errorf("Translator.TranslateSelectQuerytoBigtable() = %v, want %v", got.Clauses, tt.want.Clauses)
			}

			if got != nil && len(got.Keyspace) > 0 && !reflect.DeepEqual(got.Keyspace, tt.want.Keyspace) {
				t.Errorf("Translator.TranslateSelectQuerytoBigtable() = %v, want %v", got.Keyspace, tt.want.Keyspace)
			}
		})
	}
}

func Test_getBigtableSelectQuery(t *testing.T) {
	schemaMapping := &schemaMapping.SchemaMappingConfig{
		Logger:         zap.NewNop(),
		TablesMetaData: mockSchemaMappingConfig,
	}
	tr := &Translator{
		Logger:              zap.NewNop(),
		SchemaMappingConfig: schemaMapping,
	}

	type args struct {
		data *SelectQueryMap
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "It should faile because order by not support map data type",
			args: args{
				data: &SelectQueryMap{
					QueryType: "select",
					Table:     "test_table",
					Keyspace:  "test_keyspace",
					ColumnMeta: ColumnMeta{
						Star: true,
					},
					Limit: Limit{
						IsLimit: true,
						Count:   "1000",
					},
					OrderBy: OrderBy{
						IsOrderBy: true,
						Column:    "map_text_text",
						Operation: "asc",
					},
				},
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "It should pass with @limit value",
			args: args{
				data: &SelectQueryMap{
					QueryType: "select",
					Table:     "test_table",
					Keyspace:  "test_keyspace",
					ColumnMeta: ColumnMeta{
						Star: true,
					},
					Limit: Limit{
						IsLimit: true,
						Count:   "?",
					},
				},
			},
			want:    "SELECT * FROM test_table LIMIT @limitValue;",
			wantErr: false,
		},
		{
			name: "success",
			args: args{
				data: &SelectQueryMap{
					QueryType: "select",
					Table:     "test_table",
					ColumnMeta: ColumnMeta{
						Star: true,
					},
					Limit: Limit{
						IsLimit: true,
						Count:   "1000",
					},
					OrderBy: OrderBy{
						IsOrderBy: false,
						Column:    "",
						Operation: "",
					},
				},
			},
			want:    "SELECT * FROM test_table LIMIT 1000;",
			wantErr: false,
		},
		{
			name: "error",
			args: args{
				data: &SelectQueryMap{},
			},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getBigtableSelectQuery(tr, tt.args.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("getBigtableSelectQuery() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("getBigtableSelectQuery() = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestInferDataType(t *testing.T) {
	tests := []struct {
		methodName   string
		expectedType string
		expectedErr  error
	}{
		{"count", "bigint", nil},
		{"round", "float", nil},
		{"unknown", "", errors.New("unknown function 'unknown'")},
	}

	for _, test := range tests {
		t.Run(test.methodName, func(t *testing.T) {
			result, err := inferDataType(test.methodName)
			if result != test.expectedType {
				t.Errorf("Expected type %s, got %s", test.expectedType, result)
			}
			if (err != nil && test.expectedErr == nil) || (err == nil && test.expectedErr != nil) {
				t.Errorf("Expected error %v, got %v", test.expectedErr, err)
			}

			if err != nil && test.expectedErr != nil && err.Error() != test.expectedErr.Error() {
				t.Errorf("Expected error message %v, got %v", test.expectedErr.Error(), err.Error())
			}
		})
	}
}

func Test_parseTableFromSelect(t *testing.T) {
	type args struct {
		input cql.IFromSpecContext
	}
	tests := []struct {
		name    string
		args    args
		want    *TableObj
		wantErr bool
	}{
		{
			name: "Invalid input",
			args: args{
				input: nil,
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTableFromSelect(tt.args.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseTableFromSelect() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseTableFromSelect() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_parseOrderByFromSelect(t *testing.T) {
	type args struct {
		input cql.IOrderSpecContext
	}
	tests := []struct {
		name string
		args args
		want OrderBy
	}{
		{
			name: "Invalid input",
			args: args{
				input: nil,
			},
			want: OrderBy{
				IsOrderBy: false,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseOrderByFromSelect(tt.args.input); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseOrderByFromSelect() = %v, want %v", got, tt.want)
			}
		})
	}
}
