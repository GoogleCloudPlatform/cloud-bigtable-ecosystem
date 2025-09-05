/*
 * Copyright (C) 2025 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not
 * use this file except in compliance with the License. You may obtain a copy of
 * the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
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
	"strings"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/constants"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	schemaMapping "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/schema-mapping"
	cql "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/third_party/cqlparser"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/utilities"
	"github.com/antlr4-go/antlr/v4"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/stretchr/testify/assert"
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
		name            string
		fields          fields
		args            args
		want            *SelectQueryMap
		wantErr         bool
		defaultKeyspace string
	}{
		{
			name: "Select query with list contains key clause",
			args: args{
				query: `select column2 as name from test_keyspace.test_table where list_text CONTAINS 'test';`,
			},
			wantErr: false,
			want: &SelectQueryMap{
				Query:           "select column2 as name from test_keyspace.test_table where list_text CONTAINS 'test'",
				QueryType:       "select",
				TranslatedQuery: "SELECT cf1['column2'] as name FROM test_table WHERE ARRAY_INCLUDES(MAP_VALUES(`list_text`), @value1);",
				Table:           "test_table",
				Keyspace:        "test_keyspace",
				Clauses: []types.Clause{
					{
						Column:   "list_text",
						Operator: constants.ARRAY_INCLUDES,
						Value:    "@value1",
					},
				},
				Limit: Limit{
					IsLimit: false,
				},
				OrderBy: OrderBy{
					IsOrderBy: false,
				},
				Params: map[string]interface{}{
					"value1": []byte("test"),
				},
				ParamKeys: []string{"value1"},
			},
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "Select query with map contains key clause",
			args: args{
				query: `select column2 as name from test_keyspace.test_table where column8 CONTAINS KEY 'test';`,
			},
			wantErr: false,
			want: &SelectQueryMap{
				Query:           "select column2 as name from test_keyspace.test_table where column8 CONTAINS KEY 'test'",
				QueryType:       "select",
				TranslatedQuery: "SELECT cf1['column2'] as name FROM test_table WHERE MAP_CONTAINS_KEY(`column8`, @value1);",
				Table:           "test_table",
				Keyspace:        "test_keyspace",
				Clauses: []types.Clause{
					{
						Column:   "column8",
						Operator: "MAP_CONTAINS_KEY",
						Value:    "@value1",
					},
				},
				Limit: Limit{
					IsLimit: false,
				},
				OrderBy: OrderBy{
					IsOrderBy: false,
				},
				Params: map[string]interface{}{
					"value1": []byte("test"),
				},
				ParamKeys: []string{"value1"},
			},
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "Select query with set contains key clause",
			args: args{
				query: `select column2 as name from test_keyspace.test_table where column7 CONTAINS 'test';`,
			},
			wantErr: false,
			want: &SelectQueryMap{
				Query:           "select column2 as name from test_keyspace.test_table where column7 CONTAINS 'test'",
				QueryType:       "select",
				TranslatedQuery: "SELECT cf1['column2'] as name FROM test_table WHERE MAP_CONTAINS_KEY(`column7`, @value1);",
				Table:           "test_table",
				Keyspace:        "test_keyspace",
				Clauses: []types.Clause{
					{
						Column:   "column7",
						Operator: "MAP_CONTAINS_KEY", // We are considering set as map internally
						Value:    "@value1",
					},
				},
				Limit: Limit{
					IsLimit: false,
				},
				OrderBy: OrderBy{
					IsOrderBy: false,
				},
				Params: map[string]interface{}{
					"value1": []byte("test"),
				},
				ParamKeys: []string{"value1"},
			},
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "Writetime Query without as keyword",
			args: args{
				query: `select column1, WRITETIME(column2) from test_keyspace.test_table where column1 = 'test';`,
			},
			wantErr: false,
			want: &SelectQueryMap{
				Query:           "SELECT column1,WRITE_TIMESTAMP(cf1, 'column2') FROM test_table WHERE column1 = @value1;",
				QueryType:       "select",
				TranslatedQuery: "SELECT column1,WRITE_TIMESTAMP(cf1, 'column2') FROM test_table WHERE column1 = @value1;",
				Table:           "test_table",
				Keyspace:        "test_keyspace",
				Clauses: []types.Clause{
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
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "IN operator with integer values",
			args: args{
				query: `select column1 from test_keyspace.test_table where column6 IN (1, 2, 3);`,
			},
			wantErr: false,
			want: &SelectQueryMap{
				Query:           `select column1 from test_keyspace.test_table where column6 IN (1, 2, 3);`,
				QueryType:       "select",
				TranslatedQuery: "SELECT column1 FROM test_table WHERE TO_INT64(cf1['column6']) IN UNNEST(@value1);",
				Table:           "test_table",
				Keyspace:        "test_keyspace",
				Clauses: []types.Clause{
					{
						Column:   "column6",
						Operator: "IN",
						Value:    "@value1",
					},
				},
				Params: map[string]interface{}{
					"value1": []int{1, 2, 3},
				},
				ParamKeys: []string{"value1"},
			},
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "IN operator with bigint values",
			args: args{
				query: `select column1 from test_keyspace.test_table where column9 IN (1234567890, 9876543210);`,
			},
			wantErr: false,
			want: &SelectQueryMap{
				Query:           `select column1 from test_keyspace.test_table where column9 IN (1234567890, 9876543210);`,
				QueryType:       "select",
				TranslatedQuery: `SELECT column1 FROM test_table WHERE TO_INT64(cf1['column9']) IN UNNEST(@value1);`,
				Table:           "test_table",
				Keyspace:        "test_keyspace",
				Clauses: []types.Clause{
					{
						Column:   "column9",
						Operator: "IN",
						Value:    "@value1",
					},
				},
				Params: map[string]interface{}{
					"value1": []int64{1234567890, 9876543210},
				},
				ParamKeys: []string{"value1"},
			},
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "IN operator with float values",
			args: args{
				query: `select column1 from test_keyspace.test_table where float_col IN (1.5, 2.5, 3.5);`,
			},
			wantErr: false,
			want: &SelectQueryMap{
				Query:           `select column1 from test_keyspace.test_table where float_col IN (1.5, 2.5, 3.5);`,
				QueryType:       "select",
				TranslatedQuery: "SELECT column1 FROM test_table WHERE TO_FLOAT32(cf1['float_col']) IN UNNEST(@value1);",
				Table:           "test_table",
				Keyspace:        "test_keyspace",
				Clauses: []types.Clause{
					{
						Column:   "float_col",
						Operator: "IN",
						Value:    "@value1",
					},
				},
				Params: map[string]interface{}{
					"value1": []float32{1.5, 2.5, 3.5},
				},
				ParamKeys: []string{"value1"},
			},
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "IN operator with double values",
			args: args{
				query: `select column1 from test_keyspace.test_table where double_col IN (3.1415926535, 2.7182818284);`,
			},
			wantErr: false,
			want: &SelectQueryMap{
				Query:           `select column1 from test_keyspace.test_table where double_col IN (3.1415926535, 2.7182818284);`,
				QueryType:       "select",
				TranslatedQuery: "SELECT column1 FROM test_table WHERE TO_FLOAT64(cf1['double_col']) IN UNNEST(@value1);",
				Table:           "test_table",
				Keyspace:        "test_keyspace",
				Clauses: []types.Clause{
					{
						Column:   "double_col",
						Operator: "IN",
						Value:    "@value1",
					},
				},
				Params: map[string]interface{}{
					"value1": []float64{3.1415926535, 2.7182818284},
				},
				ParamKeys: []string{"value1"},
			},
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "IN operator with boolean values",
			args: args{
				query: `select column1 from test_keyspace.test_table where column3 IN (true, false);`,
			},
			wantErr: false,
			want: &SelectQueryMap{
				Query:           `select column1 from test_keyspace.test_table where column3 IN (true, false);`,
				QueryType:       "select",
				TranslatedQuery: "SELECT column1 FROM test_table WHERE TO_INT64(cf1['column3']) IN UNNEST(@value1);",
				Table:           "test_table",
				Keyspace:        "test_keyspace",
				Clauses: []types.Clause{
					{
						Column:   "column3",
						Operator: "IN",
						Value:    "@value1",
					},
				},
				Params: map[string]interface{}{
					"value1": []bool{true, false},
				},
				ParamKeys: []string{"value1"},
			},
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "IN operator with mixed types (should error)",
			args: args{
				query: `select column1 from test_keyspace.test_table where int_column IN (1, 'two', 3);`,
			},
			wantErr:         true,
			want:            nil,
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "IN operator with prepared statement placeholder for Int",
			args: args{
				query: `select column1 from test_keyspace.test_table where column6 IN (?);`,
			},
			wantErr: false,
			want: &SelectQueryMap{
				TranslatedQuery: "SELECT column1 FROM test_table WHERE TO_INT64(cf1['column6']) IN UNNEST(@value1);",
				Keyspace:        "test_keyspace",
				Params: map[string]interface{}{
					"value1": []int{},
				},
				ParamKeys: []string{"value1"},
				Clauses: []types.Clause{
					{
						Column:   "column6",
						Operator: "IN",
						Value:    "@value1",
					},
				},
			},
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "IN operator with prepared statement placeholder for Bigint",
			args: args{
				query: `select column1 from test_keyspace.test_table where column9 IN (?);`,
			},
			wantErr: false,
			want: &SelectQueryMap{
				TranslatedQuery: "SELECT column1 FROM test_table WHERE TO_INT64(cf1['column9']) IN UNNEST(@value1);",
				Keyspace:        "test_keyspace",
				Params: map[string]interface{}{
					"value1": []int64{},
				},
				ParamKeys: []string{"value1"},
				Clauses: []types.Clause{
					{
						Column:   "column9",
						Operator: "IN",
						Value:    "@value1",
					},
				},
			},
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "IN operator with prepared statement placeholder for Float",
			args: args{
				query: `select column1 from test_keyspace.test_table where float_col IN (?);`,
			},
			wantErr: false,
			want: &SelectQueryMap{
				TranslatedQuery: "SELECT column1 FROM test_table WHERE TO_FLOAT32(cf1['float_col']) IN UNNEST(@value1);",
				Keyspace:        "test_keyspace",
				Params: map[string]interface{}{
					"value1": []float32{},
				},
				ParamKeys: []string{"value1"},
				Clauses: []types.Clause{
					{
						Column:   "float_col",
						Operator: "IN",
						Value:    "@value1",
					},
				},
			},
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "IN operator with prepared statement placeholder for Double",
			args: args{
				query: `select column1 from test_keyspace.test_table where double_col IN (?);`,
			},
			wantErr: false,
			want: &SelectQueryMap{
				TranslatedQuery: "SELECT column1 FROM test_table WHERE TO_FLOAT64(cf1['double_col']) IN UNNEST(@value1);",
				Keyspace:        "test_keyspace",
				Params: map[string]interface{}{
					"value1": []float64{},
				},
				ParamKeys: []string{"value1"},
				Clauses: []types.Clause{
					{
						Column:   "double_col",
						Operator: "IN",
						Value:    "@value1",
					},
				},
			},
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "IN operator with prepared statement placeholder for Boolean",
			args: args{
				query: `select column1 from test_keyspace.test_table where column3 IN (?);`,
			},
			wantErr: false,
			want: &SelectQueryMap{
				TranslatedQuery: "SELECT column1 FROM test_table WHERE TO_INT64(cf1['column3']) IN UNNEST(@value1);",
				Keyspace:        "test_keyspace",
				Params: map[string]interface{}{
					"value1": []bool{},
				},
				ParamKeys: []string{"value1"},
				Clauses: []types.Clause{
					{
						Column:   "column3",
						Operator: "IN",
						Value:    "@value1",
					},
				},
			},
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "IN operator with prepared statement placeholder for Blob",
			args: args{
				query: `select column1 from test_keyspace.test_table where column2 IN (?);`,
			},
			wantErr: false,
			want: &SelectQueryMap{
				TranslatedQuery: "SELECT column1 FROM test_table WHERE TO_BLOB(cf1['column2']) IN UNNEST(@value1);",
				Keyspace:        "test_keyspace",
				Params: map[string]interface{}{
					"value1": [][]byte{},
				},
				ParamKeys: []string{"value1"},
				Clauses: []types.Clause{
					{
						Column:   "column2",
						Operator: "IN",
						Value:    "@value1",
					},
				},
			},
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "IN operator with unsupported CQL type",
			args: args{
				query: `select column1 from test_keyspace.test_table where custom_column IN (?);`,
			},
			wantErr:         true,
			want:            nil,
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "Writetime Query with as keyword",
			args: args{
				query: `select column1, WRITETIME(column2) as name from test_keyspace.test_table where column1 = 'test';`,
			},
			wantErr: false,
			want: &SelectQueryMap{
				Query:           "SELECT column1,WRITE_TIMESTAMP(cf1, 'column2') as name FROM test_table WHERE column1 = @value1;",
				QueryType:       "select",
				TranslatedQuery: "SELECT column1,WRITE_TIMESTAMP(cf1, 'column2') as name FROM test_table WHERE column1 = @value1;",
				Table:           "test_table",
				Keyspace:        "test_keyspace",
				Clauses: []types.Clause{
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
			defaultKeyspace: "test_keyspace",
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
				TranslatedQuery: "SELECT cf1['column2'] as name FROM test_table WHERE column1 = @value1;",
				Table:           "test_table",
				Keyspace:        "test_keyspace",
				Clauses: []types.Clause{
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
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "Query without Columns",
			args: args{
				query: `select from key_space.test_table where column1 = 'test'`,
			},
			wantErr:         true,
			want:            nil,
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "test for raw query when column name not exist in schema mapping table",
			args: args{
				query: "select column101, column2, column3 from  key_space.test_table where column1 = 'test' AND column3='true' AND column5 <= '2015-05-03 13:30:54.234' AND column6 >= '123' AND column9 > '-10000000' LIMIT 20000;",
			},
			want:            nil,
			wantErr:         true,
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "Query without keyspace name",
			args: args{
				query: `select column1, column2 from test_table where column1 = '?' and column1 in ('?', '?');`,
			},
			wantErr:         true,
			want:            nil,
			defaultKeyspace: "",
		},
		{
			name: "MALFORMED QUERY",
			args: args{
				query: "MALFORMED QUERY",
			},
			wantErr:         true,
			want:            nil,
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "test for raw query success",
			args: args{
				query: "select column1, column2, column3 from  test_keyspace.test_table where column1 = 'test' AND column3='true' AND column5 <= '2015-05-03 13:30:54.234' AND column6 >= '123' AND column9 > '-10000000' AND column9 < '10' LIMIT 20000;",
			},
			want: &SelectQueryMap{
				Query:           inputRawQuery,
				QueryType:       "select",
				TranslatedQuery: "SELECT column1,cf1['column2'],cf1['column3'] FROM test_table WHERE column1 = @value1 AND TO_INT64(cf1['column3']) = @value2 AND TO_TIME(cf1['column5']) <= @value3 AND TO_INT64(cf1['column6']) >= @value4 AND TO_INT64(cf1['column9']) > @value5 AND TO_INT64(cf1['column9']) < @value6 LIMIT 20000;",
				Table:           "test_table",
				Keyspace:        "test_keyspace",
				Clauses: []types.Clause{
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
					{
						Column:   "column9",
						Operator: "<",
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
				Params: map[string]interface{}{
					"value1": "test",
					"value2": int64(1),
					"value3": timeStamp,
					"value4": int32(123),
					"value5": int64(-10000000),
					"value6": int64(10),
				},
				ParamKeys: []string{"value1", "value2", "value3", "value4", "value5", "value6"},
			},
			wantErr:         false,
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "test for prepared query success",
			args: args{
				query: inputPreparedQuery,
			},
			want: &SelectQueryMap{
				Query:           inputPreparedQuery,
				QueryType:       "select",
				TranslatedQuery: "SELECT column1,cf1['column2'],cf1['column3'] FROM test_table WHERE column1 = @value1 AND TO_BLOB(cf1['column2']) = @value2 AND TO_INT64(cf1['column3']) = @value3 AND TO_TIME(cf1['column5']) = @value4 AND TO_INT64(cf1['column6']) = @value5 AND TO_INT64(cf1['column9']) = @value6;",
				Table:           "test_table",
				Keyspace:        "test_keyspace",
				Clauses: []types.Clause{
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
				Params: map[string]interface{}{"value1": "", "value2": []uint8{}, "value3": false, "value4": time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC), "value5": int32(0), "value6": int64(0)},
				Limit: Limit{
					IsLimit: true,
					Count:   "2000",
				},
				OrderBy: OrderBy{
					IsOrderBy: false,
				},
				ParamKeys: []string{"value1", "value2", "value3", "value4", "value5", "value6"},
			},
			wantErr:         false,
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "test for query without clause success",
			args: args{
				query: `select column1, column2, column3 from test_keyspace.test_table ORDER BY column1 LIMIT 20000;`,
			},
			want: &SelectQueryMap{
				Query:           `select column1, column2, column3 from test_keyspace.test_table ORDER BY column1 LIMIT 20000;`,
				QueryType:       "select",
				TranslatedQuery: "SELECT column1,cf1['column2'],cf1['column3'] FROM test_table ORDER BY column1 asc LIMIT 20000;",
				Table:           "test_table",
				Keyspace:        "test_keyspace",
				ColumnMeta: ColumnMeta{
					Star:   false,
					Column: []types.SelectedColumn{{Name: "column1"}, {Name: "column2"}, {Name: "column3"}},
				},
				Limit: Limit{
					IsLimit: true,
					Count:   "2000",
				},
				OrderBy: OrderBy{
					IsOrderBy: true,
					Columns: []OrderByColumn{
						{
							Column:    "column1",
							Operation: Asc,
						},
					},
				},
			},
			wantErr:         false,
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "error at Column parsing",
			args: args{
				query: "select  from table;",
			},
			want:            nil,
			wantErr:         true,
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "error at Clause parsing when column type not found",
			args: args{
				query: "select * from test_keyspace.table_name where name=test;",
			},
			want:            nil,
			wantErr:         true,
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "error at Clause parsing when value invalid",
			args: args{
				query: "select * from test_keyspace.test_table where column1=",
			},
			want:            nil,
			wantErr:         true,
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "test with IN operator raw query",
			args: args{
				query: `select column1, column2 from test_keyspace.test_table where column1 = 'test' and column1 in ('abc', 'xyz');`,
			},
			wantErr: false,
			want: &SelectQueryMap{
				TranslatedQuery: "SELECT column1,cf1['column2'] FROM test_table WHERE column1 = @value1 AND column1 IN UNNEST(@value2);",
				Keyspace:        "test_keyspace",
				Params: map[string]interface{}{
					"value1": "test",
					"value2": []string{"abc", "xyz"},
				},
				ParamKeys: []string{"value1", "value2"},
				Clauses: []types.Clause{
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
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "test with IN operator prepared query",
			args: args{
				query: `select column1, column2 from test_keyspace.test_table where column1 = '?' and column1 in ('?', '?');`,
			},
			wantErr: false,
			want: &SelectQueryMap{
				TranslatedQuery: "SELECT column1,cf1['column2'] FROM test_table WHERE column1 = @value1 AND column1 IN UNNEST(@value2);",
				Keyspace:        "test_keyspace",
				Params:          map[string]interface{}{"value1": "", "value2": []string{}},
				ParamKeys:       []string{"value1", "value2"},
				Clauses: []types.Clause{
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
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "test with LIKE keyword raw query",
			args: args{
				query: `select column1, column2 from test_keyspace.test_table where column1 like 'test%';`,
			},
			wantErr: false,
			want: &SelectQueryMap{
				TranslatedQuery: "SELECT column1,cf1['column2'] FROM test_table WHERE column1 LIKE @value1;",
				Keyspace:        "test_keyspace",
				Params:          map[string]interface{}{"value1": "test%"},
				ParamKeys:       []string{"value1"},
				Clauses: []types.Clause{
					{
						Column:       "column1",
						Operator:     "LIKE",
						Value:        "@value1",
						IsPrimaryKey: true,
					},
				},
			},
		},
		{
			name: "test with BETWEEN operator raw query",
			args: args{
				query: `select column1, column2 from test_keyspace.test_table where column1 between 'te''st' and 'test2';`,
			},
			wantErr: false,
			want: &SelectQueryMap{
				TranslatedQuery: "SELECT column1,cf1['column2'] FROM test_table WHERE column1 BETWEEN @value1 AND @value2;",
				Keyspace:        "test_keyspace",
				Params:          map[string]interface{}{"value1": "te'st", "value2": "test2"},
				ParamKeys:       []string{"value1", "value2"},
				Clauses: []types.Clause{
					{
						Column:       "column1",
						Operator:     "BETWEEN",
						Value:        "@value1",
						IsPrimaryKey: true,
					},
					{
						Column:       "column1",
						Operator:     "BETWEEN-AND",
						Value:        "@value2",
						IsPrimaryKey: true,
					},
				},
			},
		},
		{
			name: "test with BETWEEN operator raw query with single value",
			args: args{
				query: `select column1, column2 from test_keyspace.test_table where column1 between 'test';`,
			},
			wantErr: true,
			want:    nil,
		},
		{
			name: "test with LIKE keyword prepared query",
			args: args{
				query: `select column1, column2 from test_keyspace.test_table where column1 like '?';`,
			},
			wantErr: false,
			want: &SelectQueryMap{
				TranslatedQuery: "SELECT column1,cf1['column2'] FROM test_table WHERE column1 LIKE @value1;",
				Keyspace:        "test_keyspace",
				Params:          map[string]interface{}{"value1": ""},
				ParamKeys:       []string{"value1"},
				Clauses: []types.Clause{
					{
						Column:       "column1",
						Operator:     "LIKE",
						Value:        "@value1",
						IsPrimaryKey: true,
					},
				},
			},
		},
		{
			name: "test with BETWEEN operator prepared query",
			args: args{
				query: `select column1, column2 from test_keyspace.test_table where column1 between ? and ?;`,
			},
			wantErr: false,
			want: &SelectQueryMap{
				TranslatedQuery: "SELECT column1,cf1['column2'] FROM test_table WHERE column1 BETWEEN @value1 AND @value2;",
				Keyspace:        "test_keyspace",
				Params:          map[string]interface{}{"value1": "", "value2": ""},
				ParamKeys:       []string{"value1", "value2"},
				Clauses: []types.Clause{
					{
						Column:       "column1",
						Operator:     "BETWEEN",
						Value:        "@value1",
						IsPrimaryKey: true,
					},
					{
						Column:       "column1",
						Operator:     "BETWEEN-AND",
						Value:        "@value2",
						IsPrimaryKey: true,
					},
				},
			},
		},
		{
			name: "test with BETWEEN operator prepared query with single value",
			args: args{
				query: `select column1, column2 from test_keyspace.test_table where column1 between ?;`,
			},
			wantErr: true,
			want:    nil,
		},
		{
			name: "test with LIKE keyword raw query",
			args: args{
				query: `select column1, column2 from test_keyspace.test_table where column1 like 'test%';`,
			},
			wantErr: false,
			want: &SelectQueryMap{
				TranslatedQuery: "SELECT column1,cf1['column2'] FROM test_table WHERE column1 LIKE @value1;",
				Keyspace:        "test_keyspace",
				Params:          map[string]interface{}{"value1": "test%"},
				ParamKeys:       []string{"value1"},
				Clauses: []types.Clause{
					{
						Column:       "column1",
						Operator:     "LIKE",
						Value:        "@value1",
						IsPrimaryKey: true,
					},
				},
			},
		},
		{
			name: "test with BETWEEN operator raw query",
			args: args{
				query: `select column1, column2 from test_keyspace.test_table where column1 between 'test' and 'test2';`,
			},
			wantErr: false,
			want: &SelectQueryMap{
				TranslatedQuery: "SELECT column1,cf1['column2'] FROM test_table WHERE column1 BETWEEN @value1 AND @value2;",
				Keyspace:        "test_keyspace",
				Params:          map[string]interface{}{"value1": "test", "value2": "test2"},
				ParamKeys:       []string{"value1", "value2"},
				Clauses: []types.Clause{
					{
						Column:       "column1",
						Operator:     "BETWEEN",
						Value:        "@value1",
						IsPrimaryKey: true,
					},
					{
						Column:       "column1",
						Operator:     "BETWEEN-AND",
						Value:        "@value2",
						IsPrimaryKey: true,
					},
				},
			},
		},
		{
			name: "test with BETWEEN operator raw query without any value",
			args: args{
				query: `select column1, column2 from test_keyspace.test_table where column1 between`,
			},
			wantErr: true,
			want:    nil,
		},
		{
			name: "Empty Query",
			args: args{
				query: "",
			},
			wantErr:         true,
			want:            nil,
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "Query Without Select Object",
			args: args{
				query: "UPDATE table_name SET column1 = 'new_value1', column2 = 'new_value2' WHERE condition;",
			},
			wantErr:         true,
			want:            nil,
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "query with missing limit value",
			args: args{
				query: `select column1, column2, column3 from test_keyspace.test_table ORDER BY column1 LIMIT;`,
			},
			want:            nil,
			wantErr:         true,
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "query with LIMIT before ORDER BY",
			args: args{
				query: `select column1, column2, column3 from test_keyspace.test_table LIMIT 100 ORDER BY column1;`,
			},
			want:            nil,
			wantErr:         true,
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "query with non-existent column in ORDER BY",
			args: args{
				query: `select column1, column2, column3 from test_keyspace.test_table ORDER BY column12343 LIMIT 100;`,
			},
			want:            nil,
			wantErr:         true,
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "query with negative LIMIT value",
			args: args{
				query: `select column1, column2, column3 from test_keyspace.test_table ORDER BY column1 LIMIT -100;`,
			},
			want:            nil,
			wantErr:         true,
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "query with duplicate negative LIMIT value (potential duplicate test case)",
			args: args{
				query: `select column1, column2, column3 from test_keyspace.test_table ORDER BY column1 LIMIT -100;`,
			},
			want:            nil,
			wantErr:         true,
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "With keyspace in query, with default keyspace (should use query keyspace)",
			args: args{
				query: `select column1 from test_keyspace.test_table where column1 = 'abc';`,
			},
			wantErr: false,
			want: &SelectQueryMap{
				Keyspace:        "test_keyspace",
				Table:           "test_table",
				TranslatedQuery: "SELECT column1 FROM test_table WHERE column1 = @value1;",
				Params:          map[string]interface{}{"value1": "abc"},
				ParamKeys:       []string{"value1"},
				Clauses:         []types.Clause{{Column: "column1", Operator: "=", Value: "@value1", IsPrimaryKey: true}},
			},
			defaultKeyspace: "other_keyspace",
		},
		{
			name: "Without keyspace in query, with default keyspace (should use default)",
			args: args{
				query: `select column1 from test_table where column1 = 'abc';`,
			},
			wantErr: false,
			want: &SelectQueryMap{
				Keyspace:        "test_keyspace",
				Table:           "test_table",
				TranslatedQuery: "SELECT column1 FROM test_table WHERE column1 = @value1;",
				Params:          map[string]interface{}{"value1": "abc"},
				ParamKeys:       []string{"value1"},
				Clauses:         []types.Clause{{Column: "column1", Operator: "=", Value: "@value1", IsPrimaryKey: true}},
			},
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "Without keyspace in query, without default keyspace (should error)",
			args: args{
				query: `select column1 from test_table where column1 = 'abc';`,
			},
			wantErr:         true,
			want:            nil,
			defaultKeyspace: "",
		},
		{
			name: "With keyspace in query, with default keyspace (should ignore default)",
			args: args{
				query: `select column1 from test_keyspace.test_table where column1 = 'abc';`,
			},
			wantErr: false,
			want: &SelectQueryMap{
				Keyspace:        "test_keyspace",
				Table:           "test_table",
				TranslatedQuery: "SELECT column1 FROM test_table WHERE column1 = @value1;",
				Params:          map[string]interface{}{"value1": "abc"},
				ParamKeys:       []string{"value1"},
				Clauses:         []types.Clause{{Column: "column1", Operator: "=", Value: "@value1", IsPrimaryKey: true}},
			},
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "Invalid keyspace/table (not in schema)",
			args: args{
				query: `select column1 from invalid_keyspace.invalid_table where column1 = 'abc';`,
			},
			wantErr:         true,
			want:            nil,
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "Parser returns nil/empty for table or keyspace (simulate parser edge cases)",
			args: args{
				query: `select from ;`,
			},
			wantErr:         true,
			want:            nil,
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "Query with only table, no keyspace, and defaultKeyspace is empty (should error)",
			args: args{
				query: `select column1 from test_table;`,
			},
			wantErr:         true,
			want:            nil,
			defaultKeyspace: "",
		},
		{
			name: "Query with complex GROUP BY and HAVING",
			args: args{
				query: `select column1, count(column2) as count_col2 from test_keyspace.test_table GROUP BY column1 HAVING count(column2) > 5;`,
			},
			wantErr: false,
			want: &SelectQueryMap{
				Query:           `select column1, count(column2) as count_col2 from test_keyspace.test_table GROUP BY column1 HAVING count(column2) > 5;`,
				QueryType:       "select",
				TranslatedQuery: "SELECT column1,count(TO_BLOB(cf1['column2'])) as count_col2 FROM test_table GROUP BY column1;",
				Table:           "test_table",
				Keyspace:        "test_keyspace",
				ColumnMeta: ColumnMeta{
					Star: false,
					Column: []types.SelectedColumn{
						{Name: "column1"},
						{Name: "count_col2", IsFunc: true, FuncName: "count", ColumnName: "column2", IsAs: true, Alias: "count_col2"},
					},
				},
				GroupByColumns: []string{},
			},
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "Query with complex WHERE conditions",
			args: args{
				query: `select column1, column2 from test_keyspace.test_table where column1 = 'test' AND (column2 > 100 OR column2 < 50) AND column3 IN ('a', 'b', 'c');`,
			},
			wantErr:         true,
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "Query with multiple aggregate functions",
			args: args{
				query: `select column1, avg(column2) as avg_col2, max(column3) as max_col3, min(column4) as min_col4 from test_keyspace.test_table GROUP BY column1;`,
			},
			wantErr:         true,
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "Query with LIMIT and OFFSET",
			args: args{
				query: `select column1, column2 from test_keyspace.test_table LIMIT 10 OFFSET 20;`,
			},
			wantErr: false,
			want: &SelectQueryMap{
				Query:           `select column1, column2 from test_keyspace.test_table LIMIT 10 OFFSET 20;`,
				QueryType:       "select",
				TranslatedQuery: "SELECT column1,cf1['column2'] FROM test_table LIMIT 10;",
				Table:           "test_table",
				Keyspace:        "test_keyspace",
				Limit: Limit{
					IsLimit: true,
					Count:   "10",
				},
			},
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "Invalid ORDER BY with non-grouped column",
			args: args{
				query: `select column1, column2, column3 from test_keyspace.test_table where column1 = 'test' AND column3='true' AND column5 = '2015-05-03 13:30:54.234' AND column6 = '123' AND column9 = '-10000000' LIMIT 20000 ORDER BY column12343;`,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Valid GROUP BY with aggregate and ORDER BY",
			args: args{
				query: `select column1, column2, column3 from test_keyspace.test_table where column1 = 'test' AND column3='true' AND column5 = '2015-05-03 13:30:54.234' AND column6 = '123' AND column9 = '-10000000' LIMIT 20000 ORDER BY column12343;`,
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schemaMappingConfig := GetSchemaMappingConfig(types.OrderedCodeEncoding)
			tr := &Translator{
				Logger:              tt.fields.Logger,
				SchemaMappingConfig: schemaMappingConfig,
			}
			got, err := tr.TranslateSelectQuerytoBigtable(tt.args.query, tt.defaultKeyspace)
			if (err != nil) != tt.wantErr {
				t.Errorf("Translator.TranslateSelectQuerytoBigtable() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.want != nil {
				assert.Equal(t, tt.want.TranslatedQuery, got.TranslatedQuery)
				assert.Equal(t, tt.want.Params, got.Params)
				assert.Equal(t, tt.want.ParamKeys, got.ParamKeys)
				assert.Equal(t, tt.want.Clauses, got.Clauses)
				assert.Equal(t, tt.want.Keyspace, got.Keyspace)
			}
		})
	}
}

func Test_GetBigtableSelectQuery(t *testing.T) {
	schemaMap := GetSchemaMappingConfig(types.OrderedCodeEncoding)
	tr := &Translator{
		Logger:              zap.NewNop(),
		SchemaMappingConfig: schemaMap,
	}

	type args struct {
		data *SelectQueryMap
	}
	tests := []struct {
		name       string
		args       args
		want       string
		wantErr    bool
		translator *Translator
	}{
		{
			name: "It should fail because order by not support map data type",
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
						Columns: []OrderByColumn{
							{
								Column:    "map_text_text",
								Operation: Asc,
							},
						},
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
					Keyspace:  "test_keyspace",
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
					},
				},
			},
			want:    "SELECT * FROM test_table LIMIT 1000;",
			wantErr: false,
		},
		{
			name: "GROUP BY with multiple columns",
			args: args{
				data: &SelectQueryMap{
					QueryType: "select",
					Table:     "test_table",
					Keyspace:  "test_keyspace",
					ColumnMeta: ColumnMeta{
						Star: false,
						Column: []types.SelectedColumn{
							{Name: "column1"},
							{Name: "column6"},
						},
					},
					GroupByColumns: []string{"column1", "column6"},
				},
			},
			want:    "SELECT column1,TO_INT64(cf1['column6']) FROM test_table GROUP BY column1,TO_INT64(cf1['column6']);",
			wantErr: false,
		},
		{
			name: "GROUP BY with unsupported collection type",
			args: args{
				data: &SelectQueryMap{
					QueryType: "select",
					Table:     "test_table",
					Keyspace:  "test_keyspace",
					ColumnMeta: ColumnMeta{
						Star: false,
						Column: []types.SelectedColumn{
							{Name: "column1"},
							{Name: "map_text_text"},
						},
					},
					GroupByColumns: []string{"column1", "map_text_text"},
				},
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "error",
			args: args{
				data: &SelectQueryMap{},
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "All clauses: WHERE, GROUP BY, ORDER BY, LIMIT, AGGREGATE, AS",
			args: args{
				data: &SelectQueryMap{
					QueryType: "select",
					Table:     "test_table",
					Keyspace:  "test_keyspace",
					ColumnMeta: ColumnMeta{
						Star: false,
						Column: []types.SelectedColumn{
							{Name: "column1", ColumnName: "column1"},
							{Name: "SUM(column2)", IsFunc: true, FuncName: "SUM", ColumnName: "column2", IsAs: true, Alias: "total"},
							{Name: "column3", ColumnName: "column3"},
						},
					},
					Clauses: []types.Clause{
						{Column: "column3", Operator: "=", Value: "10"},
					},
					GroupByColumns: []string{"column1", "column3"},
					OrderBy: OrderBy{
						IsOrderBy: true,
						Columns: []OrderByColumn{
							{Column: "column3", Operation: Desc},
							{Column: "column1", Operation: Asc},
						},
					},
					Limit: Limit{IsLimit: true, Count: "100"},
				},
			},
			want:    "SELECT column1,SUM(TO_INT64(cf1['column2'])) as total,column3 FROM test_table WHERE column3 = 10 GROUP BY column1,column3 ORDER BY column3 desc, column1 asc LIMIT 100;",
			wantErr: false,
			translator: func() *Translator {
				// REFACTOR: Use constructor functions
				cols := []*types.Column{
					{Name: "column1", CQLType: datatype.Varchar, KeyType: utilities.KEY_TYPE_PARTITION},
					{Name: "column2", CQLType: datatype.Bigint, KeyType: utilities.KEY_TYPE_REGULAR},
					{Name: "column3", CQLType: datatype.Bigint, KeyType: utilities.KEY_TYPE_PARTITION},
				}
				tableCfg := schemaMapping.NewTableConfig("test_keyspace", "test_table", "cf1", types.OrderedCodeEncoding, cols)
				schemaCfg := schemaMapping.NewSchemaMappingConfig("schema_mappings", "cf1", zap.NewNop(), []*schemaMapping.TableConfig{tableCfg})
				return &Translator{
					Logger:              zap.NewNop(),
					SchemaMappingConfig: schemaCfg,
				}
			}(),
		},
		{
			name: "Multiple aggregates with GROUP BY and ORDER BY",
			args: args{
				data: &SelectQueryMap{
					QueryType: "select",
					Table:     "test_table",
					Keyspace:  "test_keyspace",
					ColumnMeta: ColumnMeta{
						Star: false,
						Column: []types.SelectedColumn{
							{Name: "column1", ColumnName: "column1"},
							{Name: "AVG(column2)", IsFunc: true, FuncName: "AVG", ColumnName: "column2", IsAs: true, Alias: "avg_value"},
							{Name: "MAX(column3)", IsFunc: true, FuncName: "MAX", ColumnName: "column3", IsAs: true, Alias: "max_value"},
						},
					},
					GroupByColumns: []string{"column1"},
					OrderBy: OrderBy{
						IsOrderBy: true,
						Columns: []OrderByColumn{
							{Column: "avg_value", Operation: Desc},
						},
					},
				},
			},
			want:    "SELECT column1,AVG(TO_INT64(cf1['column2'])) as avg_value,MAX(TO_INT64(cf1['column3'])) as max_value FROM test_table GROUP BY column1 ORDER BY avg_value desc;",
			wantErr: false,
			translator: func() *Translator {
				// REFACTOR: Use constructor functions
				cols := []*types.Column{
					{Name: "column1", CQLType: datatype.Varchar, KeyType: utilities.KEY_TYPE_PARTITION},
					{Name: "column2", CQLType: datatype.Bigint, KeyType: utilities.KEY_TYPE_REGULAR},
					{Name: "column3", CQLType: datatype.Bigint, KeyType: utilities.KEY_TYPE_REGULAR},
				}
				tableCfg := schemaMapping.NewTableConfig("test_keyspace", "test_table", "cf1", types.OrderedCodeEncoding, cols)
				schemaCfg := schemaMapping.NewSchemaMappingConfig("schema_mappings", "cf1", zap.NewNop(), []*schemaMapping.TableConfig{tableCfg})
				return &Translator{
					Logger:              zap.NewNop(),
					SchemaMappingConfig: schemaCfg,
				}
			}(),
		},
		{
			name: "COUNT with WHERE and LIMIT",
			args: args{
				data: &SelectQueryMap{
					QueryType: "select",
					Table:     "test_table",
					Keyspace:  "test_keyspace",
					ColumnMeta: ColumnMeta{
						Star: false,
						Column: []types.SelectedColumn{
							{Name: "COUNT(*)", IsFunc: true, FuncName: "COUNT", ColumnName: "*", IsAs: true, Alias: "total_count"},
						},
					},
					Clauses: []types.Clause{
						{Column: "column1", Operator: ">", Value: "5"},
					},
					Limit: Limit{IsLimit: true, Count: "50"},
				},
			},
			want:    "SELECT count(*) as total_count FROM test_table WHERE column1 > 5 LIMIT 50;",
			wantErr: false,
			translator: func() *Translator {
				// REFACTOR: Use constructor functions
				cols := []*types.Column{
					{Name: "column1", CQLType: datatype.Bigint, KeyType: utilities.KEY_TYPE_PARTITION},
				}
				tableCfg := schemaMapping.NewTableConfig("test_keyspace", "test_table", "cf1", types.OrderedCodeEncoding, cols)
				schemaCfg := schemaMapping.NewSchemaMappingConfig("schema_mappings", "cf1", zap.NewNop(), []*schemaMapping.TableConfig{tableCfg})
				return &Translator{
					Logger:              zap.NewNop(),
					SchemaMappingConfig: schemaCfg,
				}
			}(),
		},
		{
			name: "MIN and MAX with WHERE and ORDER BY",
			args: args{
				data: &SelectQueryMap{
					QueryType: "select",
					Table:     "test_table",
					Keyspace:  "test_keyspace",
					ColumnMeta: ColumnMeta{
						Star: false,
						Column: []types.SelectedColumn{
							{Name: "MIN(column2)", IsFunc: true, FuncName: "MIN", ColumnName: "column2", IsAs: true, Alias: "min_value"},
							{Name: "MAX(column2)", IsFunc: true, FuncName: "MAX", ColumnName: "column2", IsAs: true, Alias: "max_value"},
						},
					},
					Clauses: []types.Clause{
						{Column: "column1", Operator: "IN", Value: "@values"},
					},
					OrderBy: OrderBy{
						IsOrderBy: true,
						Columns: []OrderByColumn{
							{Column: "min_value", Operation: Asc},
						},
					},
				},
			},
			want:    "SELECT MIN(TO_INT64(cf1['column2'])) as min_value,MAX(TO_INT64(cf1['column2'])) as max_value FROM test_table WHERE column1 IN UNNEST(@values) ORDER BY min_value asc;",
			wantErr: false,
			translator: func() *Translator {
				// REFACTOR: Use constructor functions
				cols := []*types.Column{
					{Name: "column1", CQLType: datatype.Varchar, KeyType: utilities.KEY_TYPE_PARTITION},
					{Name: "column2", CQLType: datatype.Bigint, KeyType: utilities.KEY_TYPE_REGULAR},
				}
				tableCfg := schemaMapping.NewTableConfig("test_keyspace", "test_table", "cf1", types.OrderedCodeEncoding, cols)
				schemaCfg := schemaMapping.NewSchemaMappingConfig("schema_mappings", "cf1", zap.NewNop(), []*schemaMapping.TableConfig{tableCfg})
				return &Translator{
					Logger:              zap.NewNop(),
					SchemaMappingConfig: schemaCfg,
				}
			}(),
		},
		{
			name: "SUM with GROUP BY",
			args: args{
				data: &SelectQueryMap{
					QueryType: "select",
					Table:     "test_table",
					Keyspace:  "test_keyspace",
					ColumnMeta: ColumnMeta{
						Star: false,
						Column: []types.SelectedColumn{
							{Name: "column1", ColumnName: "column1"},
							{Name: "SUM(column2)", IsFunc: true, FuncName: "SUM", ColumnName: "column2", IsAs: true, Alias: "total_sum"},
						},
					},
					GroupByColumns: []string{"column1"},
				},
			},
			want:    "SELECT column1,SUM(TO_INT64(cf1['column2'])) as total_sum FROM test_table GROUP BY column1;",
			wantErr: false,
			translator: func() *Translator {
				// REFACTOR: Use constructor functions
				cols := []*types.Column{
					{Name: "column1", CQLType: datatype.Varchar, KeyType: utilities.KEY_TYPE_PARTITION},
					{Name: "column2", CQLType: datatype.Bigint, KeyType: utilities.KEY_TYPE_REGULAR},
				}
				tableCfg := schemaMapping.NewTableConfig("test_keyspace", "test_table", "cf1", types.OrderedCodeEncoding, cols)
				schemaCfg := schemaMapping.NewSchemaMappingConfig("schema_mappings", "cf1", zap.NewNop(), []*schemaMapping.TableConfig{tableCfg})
				return &Translator{
					Logger:              zap.NewNop(),
					SchemaMappingConfig: schemaCfg,
				}
			}(),
		},
		{
			name: "Invalid ORDER BY with non-grouped column",
			args: args{
				data: &SelectQueryMap{
					QueryType: "select",
					Table:     "test_table",
					Keyspace:  "test_keyspace",
					ColumnMeta: ColumnMeta{
						Star: false,
						Column: []types.SelectedColumn{
							{Name: "name", ColumnName: "name"},
							{Name: "age", ColumnName: "age"},
							{Name: "MAX(code)", IsFunc: true, FuncName: "MAX", ColumnName: "code", IsAs: true, Alias: "max_code"},
						},
					},
					GroupByColumns: []string{"name", "age"},
					OrderBy: OrderBy{
						IsOrderBy: true,
						Columns: []OrderByColumn{
							{Column: "name", Operation: Asc},
							{Column: "code", Operation: Desc},
						},
					},
				},
			},
			want:    "SELECT name,TO_INT64(cf1['age']),MAX(TO_INT64(cf1['code'])) as max_code FROM test_table GROUP BY name,TO_INT64(cf1['age']) ORDER BY name asc, TO_INT64(cf1['code']) desc;",
			wantErr: false,
			translator: func() *Translator {
				// REFACTOR: Use constructor functions
				cols := []*types.Column{
					{Name: "name", CQLType: datatype.Varchar, KeyType: utilities.KEY_TYPE_PARTITION},
					{Name: "age", CQLType: datatype.Bigint, KeyType: utilities.KEY_TYPE_REGULAR},
					{Name: "code", CQLType: datatype.Bigint, KeyType: utilities.KEY_TYPE_REGULAR},
				}
				tableCfg := schemaMapping.NewTableConfig("test_keyspace", "test_table", "cf1", types.OrderedCodeEncoding, cols)
				schemaCfg := schemaMapping.NewSchemaMappingConfig("schema_mappings", "cf1", zap.NewNop(), []*schemaMapping.TableConfig{tableCfg})
				return &Translator{
					Logger:              zap.NewNop(),
					SchemaMappingConfig: schemaCfg,
				}
			}(),
		},
		{
			name: "Invalid SELECT with non-grouped column",
			args: args{
				data: &SelectQueryMap{
					QueryType: "select",
					Table:     "test_table",
					Keyspace:  "test_keyspace",
					ColumnMeta: ColumnMeta{
						Star: false,
						Column: []types.SelectedColumn{
							{Name: "name", ColumnName: "name"},
							{Name: "age", ColumnName: "age"},
							{Name: "code", ColumnName: "code"},
						},
					},
					GroupByColumns: []string{"name", "age"},
					OrderBy: OrderBy{
						IsOrderBy: true,
						Columns: []OrderByColumn{
							{Column: "name", Operation: Asc},
						},
					},
				},
			},
			want:    "SELECT name,TO_INT64(cf1['age']),TO_INT64(cf1['code']) FROM test_table GROUP BY name,TO_INT64(cf1['age']) ORDER BY name asc;",
			wantErr: false,
			translator: func() *Translator {
				// REFACTOR: Use constructor functions
				cols := []*types.Column{
					{Name: "name", CQLType: datatype.Varchar, KeyType: utilities.KEY_TYPE_PARTITION},
					{Name: "age", CQLType: datatype.Bigint, KeyType: utilities.KEY_TYPE_REGULAR},
					{Name: "code", CQLType: datatype.Bigint, KeyType: utilities.KEY_TYPE_REGULAR},
				}
				tableCfg := schemaMapping.NewTableConfig("test_keyspace", "test_table", "cf1", types.OrderedCodeEncoding, cols)
				schemaCfg := schemaMapping.NewSchemaMappingConfig("schema_mappings", "cf1", zap.NewNop(), []*schemaMapping.TableConfig{tableCfg})
				return &Translator{
					Logger:              zap.NewNop(),
					SchemaMappingConfig: schemaCfg,
				}
			}(),
		},
		{
			name: "Valid GROUP BY with aggregate and ORDER BY",
			args: args{
				data: &SelectQueryMap{
					QueryType: "select",
					Table:     "test_table",
					Keyspace:  "test_keyspace",
					ColumnMeta: ColumnMeta{
						Star: false,
						Column: []types.SelectedColumn{
							{Name: "name", ColumnName: "name"},
							{Name: "age", ColumnName: "age"},
							{Name: "MAX(code)", IsFunc: true, FuncName: "MAX", ColumnName: "code", IsAs: true, Alias: "max_code"},
						},
					},
					GroupByColumns: []string{"name", "age"},
					OrderBy: OrderBy{
						IsOrderBy: true,
						Columns: []OrderByColumn{
							{Column: "name", Operation: Asc},
							{Column: "max_code", Operation: Desc},
						},
					},
				},
			},
			want:    "SELECT name,TO_INT64(cf1['age']),MAX(TO_INT64(cf1['code'])) as max_code FROM test_table GROUP BY name,TO_INT64(cf1['age']) ORDER BY name asc, max_code desc;",
			wantErr: false,
			translator: func() *Translator {
				// REFACTOR: Use constructor functions
				cols := []*types.Column{
					{Name: "name", CQLType: datatype.Varchar, KeyType: utilities.KEY_TYPE_PARTITION},
					{Name: "age", CQLType: datatype.Bigint, KeyType: utilities.KEY_TYPE_REGULAR},
					{Name: "code", CQLType: datatype.Bigint, KeyType: utilities.KEY_TYPE_REGULAR},
				}
				tableCfg := schemaMapping.NewTableConfig("test_keyspace", "test_table", "cf1", types.OrderedCodeEncoding, cols)
				schemaCfg := schemaMapping.NewSchemaMappingConfig("schema_mappings", "cf1", zap.NewNop(), []*schemaMapping.TableConfig{tableCfg})
				return &Translator{
					Logger:              zap.NewNop(),
					SchemaMappingConfig: schemaCfg,
				}
			}(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trToUse := tr
			if tt.translator != nil {
				trToUse = tt.translator
			}
			got, err := getBigtableSelectQuery(trToUse, tt.args.data)
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
	tests := []struct {
		name    string
		query   string
		want    *TableObj
		wantErr bool
	}{
		{
			name:  "Valid keyspace.table format",
			query: "SELECT * FROM test_keyspace.test_table",
			want: &TableObj{
				TableName:    "test_table",
				KeyspaceName: "test_keyspace",
			},
			wantErr: false,
		},
		{
			name:  "Missing keyspace",
			query: "SELECT * FROM test_table",
			want: &TableObj{
				TableName:    "test_table",
				KeyspaceName: "",
			},
			wantErr: false,
		},
		{
			name:    "Missing table",
			query:   "SELECT * FROM ",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Empty query",
			query:   "",
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewCqlParser(tt.query, false)
			if err != nil {
				if !tt.wantErr {
					t.Fatalf("Failed to create parser: %v", err)
				}
				return
			}

			var fromSpec cql.IFromSpecContext
			if selectStmt := p.Select_(); selectStmt != nil {
				fromSpec = selectStmt.FromSpec()
			}

			got, err := parseTableFromSelect(fromSpec)
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
		name    string
		args    args
		want    OrderBy
		wantErr bool
	}{
		{
			name: "Empty Input",
			args: args{
				input: nil,
			},
			want: OrderBy{
				IsOrderBy: false,
			},
			wantErr: false,
		},
		{
			name: "Single Column Order By",
			args: args{
				input: &mockOrderSpecContext{
					orderSpecElements: []cql.IOrderSpecElementContext{
						&mockOrderSpecElementContext{
							objectName: "column1",
							isDesc:     false,
						},
					},
				},
			},
			want: OrderBy{
				IsOrderBy: true,
				Columns: []OrderByColumn{
					{
						Column:    "column1",
						Operation: Asc,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Multiple Columns Order By",
			args: args{
				input: &mockOrderSpecContext{
					orderSpecElements: []cql.IOrderSpecElementContext{
						&mockOrderSpecElementContext{
							objectName: "column1",
							isDesc:     false,
						},
						&mockOrderSpecElementContext{
							objectName: "column2",
							isDesc:     true,
						},
					},
				},
			},
			want: OrderBy{
				IsOrderBy: true,
				Columns: []OrderByColumn{
					{
						Column:    "column1",
						Operation: Asc,
					},
					{
						Column:    "column2",
						Operation: Desc,
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseOrderByFromSelect(tt.args.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseOrderByFromSelect() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseOrderByFromSelect() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Mock implementations for testing
type mockOrderSpecContext struct {
	cql.IOrderSpecContext
	orderSpecElements []cql.IOrderSpecElementContext
}

func (m *mockOrderSpecContext) AllOrderSpecElement() []cql.IOrderSpecElementContext {
	return m.orderSpecElements
}

type mockOrderSpecElementContext struct {
	cql.IOrderSpecElementContext
	objectName string
	isDesc     bool
}

func (m *mockOrderSpecElementContext) OBJECT_NAME() antlr.TerminalNode {
	return &mockTerminalNode{text: m.objectName}
}

func (m *mockOrderSpecElementContext) KwDesc() cql.IKwDescContext {
	if m.isDesc {
		return &mockKwDescContext{}
	}
	return nil
}

type mockKwDescContext struct {
	cql.IKwDescContext
}

func Test_processFunctionColumn(t *testing.T) {
	tests := []struct {
		name           string
		columnMetadata types.SelectedColumn
		tableName      string
		keySpace       string
		inputColumns   []string
		wantColumns    []string
		wantErr        bool
		errMsg         string
	}{
		{
			name: "COUNT(*)",
			columnMetadata: types.SelectedColumn{
				FuncName:   "count",
				ColumnName: "*",
				IsFunc:     true,
			},
			tableName:    "user_info",
			keySpace:     "test_keyspace",
			inputColumns: []string{},
			wantColumns: []string{
				"count(*)",
			},
			wantErr: false,
		},
		{
			name: "AVG with numeric column",
			columnMetadata: types.SelectedColumn{
				FuncName:   "avg",
				ColumnName: "age",
				IsFunc:     true,
			},
			tableName:    "user_info",
			keySpace:     "test_keyspace",
			inputColumns: []string{},
			wantColumns: []string{
				"avg(TO_INT64(cf1['age']))",
			},
			wantErr: false,
		},
		{
			name: "SUM with alias",
			columnMetadata: types.SelectedColumn{
				FuncName:   "sum",
				ColumnName: "balance",
				IsFunc:     true,
				IsAs:       true,
				Alias:      "total_balance",
			},
			tableName:    "user_info",
			keySpace:     "test_keyspace",
			inputColumns: []string{},
			wantColumns: []string{
				"sum(TO_FLOAT32(cf1['balance'])) as total_balance",
			},
			wantErr: false,
		},
		{
			name: "Invalid function",
			columnMetadata: types.SelectedColumn{
				FuncName:   "invalid_func",
				ColumnName: "age",
				IsFunc:     true,
			},
			tableName:    "user_info",
			keySpace:     "test_keyspace",
			inputColumns: []string{},
			wantErr:      true,
			errMsg:       "unknown function 'invalid_func'",
		},
		{
			name: "Non-numeric column in aggregate",
			columnMetadata: types.SelectedColumn{
				FuncName:   "sum",
				ColumnName: "name",
				IsFunc:     true,
			},
			tableName:    "user_info",
			keySpace:     "test_keyspace",
			inputColumns: []string{},
			wantErr:      true,
			errMsg:       "column not supported for aggregate",
		},
		{
			name: "AVG with float column",
			columnMetadata: types.SelectedColumn{
				FuncName:   "avg",
				ColumnName: "balance",
				IsFunc:     true,
			},
			tableName:    "user_info",
			keySpace:     "test_keyspace",
			inputColumns: []string{},
			wantColumns: []string{
				"avg(TO_FLOAT32(cf1['balance']))",
			},
			wantErr: false,
		},
		{
			name: "MIN with int column",
			columnMetadata: types.SelectedColumn{
				FuncName:   "min",
				ColumnName: "code",
				IsFunc:     true,
			},
			tableName:    "user_info",
			keySpace:     "test_keyspace",
			inputColumns: []string{},
			wantColumns: []string{
				"min(TO_INT64(cf1['code']))",
			},
			wantErr: false,
		},
		{
			name: "MAX with alias",
			columnMetadata: types.SelectedColumn{
				FuncName:   "max",
				ColumnName: "age",
				IsFunc:     true,
				IsAs:       true,
				Alias:      "max_age",
			},
			tableName:    "user_info",
			keySpace:     "test_keyspace",
			inputColumns: []string{},
			wantColumns: []string{
				"max(TO_INT64(cf1['age'])) as max_age",
			},
			wantErr: false,
		},
		{
			name: "Missing column metadata",
			columnMetadata: types.SelectedColumn{
				FuncName:   "avg",
				ColumnName: "nonexistent",
				IsFunc:     true,
			},
			tableName:    "user_info",
			keySpace:     "test_keyspace",
			inputColumns: []string{},
			wantErr:      true,
			errMsg:       "column metadata not found for column 'nonexistent' in table 'user_info' and keyspace 'test_keyspace'",
		},
		{
			name: "Empty function name",
			columnMetadata: types.SelectedColumn{
				FuncName:   "",
				ColumnName: "age",
				IsFunc:     true,
			},
			tableName:    "user_info",
			keySpace:     "test_keyspace",
			inputColumns: []string{},
			wantErr:      true,
			errMsg:       "unknown function ''",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// REFACTOR: Use constructor functions
			cols := []*types.Column{
				{Name: "age", CQLType: datatype.Bigint, KeyType: utilities.KEY_TYPE_REGULAR},
				{Name: "balance", CQLType: datatype.Float, KeyType: utilities.KEY_TYPE_REGULAR},
				{Name: "name", CQLType: datatype.Varchar, KeyType: utilities.KEY_TYPE_REGULAR},
				{Name: "code", CQLType: datatype.Int, KeyType: utilities.KEY_TYPE_REGULAR},
			}
			tableCfg := schemaMapping.NewTableConfig("test_keyspace", "user_info", "cf1", types.OrderedCodeEncoding, cols)
			schemaCfg := schemaMapping.NewSchemaMappingConfig("schema_mappings", "cf1", zap.NewNop(), []*schemaMapping.TableConfig{tableCfg})
			translator := &Translator{SchemaMappingConfig: schemaCfg}

			tc, err := translator.SchemaMappingConfig.GetTableConfig(tt.keySpace, tt.tableName)
			if err != nil {
				t.Errorf("table config should exist for %s.%s", tt.keySpace, tt.tableName)
				return
			}
			gotColumns, err := processFunctionColumn(translator, tt.columnMetadata, tc, tt.inputColumns)

			if tt.wantErr {
				if err == nil {
					t.Errorf("processFunctionColumn() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if err.Error() != tt.errMsg {
					t.Errorf("processFunctionColumn() error = %v, wantErr %v", err, tt.errMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("processFunctionColumn() unexpected error = %v", err)
				return
			}

			if !reflect.DeepEqual(gotColumns, tt.wantColumns) {
				t.Errorf("processFunctionColumn() gotColumns = %v, want %v", gotColumns, tt.wantColumns)
			}

		})
	}
}

func Test_parseColumnsFromSelectWithParser(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		want    ColumnMeta
		wantErr bool
		err     string
	}{
		{
			name:    "star query",
			query:   "SELECT * FROM test_table",
			want:    ColumnMeta{Star: true},
			wantErr: false,
		},
		{
			name:  "single column",
			query: "SELECT pk_1_text FROM test_table",
			want: ColumnMeta{
				Column: []types.SelectedColumn{
					{Name: "pk_1_text", ColumnName: "pk_1_text"},
				},
			},
			wantErr: false,
		},
		{
			name:  "multiple columns",
			query: "SELECT pk_1_text, blob_col, bool_col FROM test_table",
			want: ColumnMeta{
				Column: []types.SelectedColumn{
					{Name: "pk_1_text", ColumnName: "pk_1_text"},
					{Name: "blob_col", ColumnName: "blob_col"},
					{Name: "bool_col", ColumnName: "bool_col"},
				},
			},
			wantErr: false,
		},
		{
			name:  "column with alias",
			query: "SELECT pk_1_text AS alias1 FROM test_table",
			want: ColumnMeta{
				Column: []types.SelectedColumn{
					{Name: "pk_1_text", IsAs: true, Alias: "alias1", ColumnName: "pk_1_text"},
				},
			},
			wantErr: false,
		},
		{
			name:  "function with star",
			query: "SELECT COUNT(*) FROM test_table",
			want: ColumnMeta{
				Column: []types.SelectedColumn{
					{Name: "system.count(*)", IsFunc: true, FuncName: "count", Alias: "", ColumnName: "*"},
				},
			},
			wantErr: false,
		},
		{
			name:  "writetime function",
			query: "SELECT name,WRITETIME(pk_1_text) AS wt FROM test_table",
			want: ColumnMeta{
				Column: []types.SelectedColumn{
					{Name: "name", ColumnName: "name"},
					{
						Name:              "writetime(pk_1_text)",
						Alias:             "wt",
						IsAs:              true,
						ColumnName:        "pk_1_text",
						IsWriteTimeColumn: true,
					},
				},
			},
			wantErr: false,
		},
		{
			name:  "map access",
			query: "SELECT map_col['key1'] FROM test_table",
			want: ColumnMeta{
				Column: []types.SelectedColumn{
					{Name: "map_col['key1']", MapKey: "key1", Alias: "", ColumnName: "map_col"},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewCqlParser(tt.query, false)
			if err != nil {
				t.Fatalf("Failed to create parser: %v", err)
			}

			selectElements := p.Select_().SelectElements()

			got, err := parseColumnsFromSelect(selectElements)

			if (err != nil) != tt.wantErr {
				t.Errorf("parseColumnsFromSelect() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseColumnsFromSelect() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_parseGroupByColumn(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected []string
	}{
		{
			name:     "Single column",
			query:    "select * from table group by col1",
			expected: []string{"col1"},
		},
		{
			name:     "Multiple columns",
			query:    "select * from table group by col1, col2, col3",
			expected: []string{"col1", "col2", "col3"},
		},
		{
			name:     "With semicolon",
			query:    "select * from table group by col1;",
			expected: []string{"col1"},
		},
		{
			name:     "With ORDER BY",
			query:    "select * from table group by col1 order by col2",
			expected: []string{"col1"},
		},
		{
			name:     "With LIMIT",
			query:    "select * from table group by col1 limit 10",
			expected: []string{"col1"},
		},
		{
			name:     "No GROUP BY",
			query:    "select * from table",
			expected: nil,
		},
		{
			name:     "Malformed GROUP BY",
			query:    "select * from table group by",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewCqlParser(tt.query, false)
			if err != nil {
				if tt.expected == nil {
					return
				}
				t.Fatalf("Failed to create parser: %v", err)
			}

			var groupSpec cql.IGroupSpecContext
			if selectStmt := p.Select_(); selectStmt != nil {
				groupSpec = selectStmt.GroupSpec()
			}

			got := parseGroupByColumn(groupSpec)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("parseGroupByColumn() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func Test_dtAllowedInAggregate(t *testing.T) {
	tests := []struct {
		dataType string
		expected bool
	}{
		{"int", true},
		{"bigint", true},
		{"float", true},
		{"double", true},
		{"text", false},
		{"boolean", false},
		{"timestamp", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.dataType, func(t *testing.T) {
			got := dtAllowedInAggregate(tt.dataType)
			if got != tt.expected {
				t.Errorf("dtAllowedInAggregate(%q) = %v, want %v", tt.dataType, got, tt.expected)
			}
		})
	}
}

func Test_parseLimitFromSelect(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		want    Limit
		wantErr bool
		errMsg  string
	}{
		{
			name:  "Valid numeric limit",
			query: "SELECT * FROM test_table LIMIT 10",
			want:  Limit{IsLimit: true, Count: "10"},
		},
		{
			name:  "Placeholder limit",
			query: "SELECT * FROM test_table LIMIT ?",
			want:  Limit{IsLimit: true, Count: "?"},
		},
		{
			name:  "No limit",
			query: "SELECT * FROM test_table",
			want:  Limit{IsLimit: false},
		},
		{
			name:    "Invalid limit (negative)",
			query:   "SELECT * FROM test_table LIMIT -10",
			want:    Limit{},
			wantErr: true,
			errMsg:  "no viable alternative at input '-10'",
		},
		{
			name:    "Invalid limit (zero)",
			query:   "SELECT * FROM test_table LIMIT 0",
			want:    Limit{},
			wantErr: true,
			errMsg:  "no viable alternative at input '0'",
		},
		{
			name:    "Invalid limit (non-numeric)",
			query:   "SELECT * FROM test_table LIMIT abc",
			want:    Limit{},
			wantErr: true,
			errMsg:  "no viable alternative at input 'abc'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewCqlParser(tt.query, false)
			if err != nil {
				t.Fatalf("Failed to create parser: %v", err)
			}

			var limitSpec cql.ILimitSpecContext
			if selectStmt := p.Select_(); selectStmt != nil {
				limitSpec = selectStmt.LimitSpec()
			}

			got, err := parseLimitFromSelect(limitSpec)

			if (err != nil) != tt.wantErr {
				t.Errorf("parseLimitFromSelect() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				if err == nil || !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("parseLimitFromSelect() error = %v, wantErr containing %q", err, tt.errMsg)
				}
				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseLimitFromSelect() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_funcAllowedInAggregate(t *testing.T) {
	tests := []struct {
		funcName string
		expected bool
	}{
		{"avg", true},
		{"sum", true},
		{"min", true},
		{"max", true},
		{"count", true},
		{"AVG", true}, // case insensitive
		{"Sum", true}, // case insensitive
		{"unknown", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.funcName, func(t *testing.T) {
			got := funcAllowedInAggregate(tt.funcName)
			if got != tt.expected {
				t.Errorf("funcAllowedInAggregate(%q) = %v, want %v", tt.funcName, got, tt.expected)
			}
		})
	}
}

func TestProcessSetStrings(t *testing.T) {
	tests := []struct {
		name            string
		selectedColumns []types.SelectedColumn
		tableName       string
		keySpace        string
		isGroupBy       bool
		wantColumns     []string
		wantErr         bool
	}{
		{
			name: "Simple column selection",
			selectedColumns: []types.SelectedColumn{
				{Name: "name"},
				{Name: "age"},
			},
			tableName: "user_info",
			keySpace:  "test_keyspace",
			isGroupBy: false,
			wantColumns: []string{
				"cf1['name']",
				"cf1['age']",
			},
			wantErr: false,
		},
		{
			name: "Column with alias",
			selectedColumns: []types.SelectedColumn{
				{Name: "name", IsAs: true, Alias: "username"},
				{Name: "age"},
			},
			tableName: "user_info",
			keySpace:  "test_keyspace",
			isGroupBy: false,
			wantColumns: []string{
				"cf1['name'] as username",
				"cf1['age']",
			},
			wantErr: false,
		},
		{
			name: "Aggregate function",
			selectedColumns: []types.SelectedColumn{
				{
					Name:       "count_age",
					IsFunc:     true,
					FuncName:   "count",
					ColumnName: "age",
					IsAs:       true,
					Alias:      "age_count",
				},
			},
			tableName:   "user_info",
			keySpace:    "test_keyspace",
			isGroupBy:   false,
			wantColumns: []string{"count(TO_INT64(cf1['age'])) as age_count"},
			wantErr:     false,
		},
		{
			name: "Invalid column",
			selectedColumns: []types.SelectedColumn{
				{Name: "invalid_column"},
			},
			tableName: "user_info",
			keySpace:  "test_keyspace",
			isGroupBy: false,
			wantErr:   true,
		},
		{
			name: "Regular column with alias",
			selectedColumns: []types.SelectedColumn{
				{Name: "age", IsAs: true, Alias: "user_age"},
			},
			tableName:   "user_info",
			keySpace:    "test_keyspace",
			isGroupBy:   false,
			wantColumns: []string{"cf1['age'] as user_age"},
			wantErr:     false,
		},
		{
			name: "Collection column with alias",
			selectedColumns: []types.SelectedColumn{
				{Name: "map_col", IsAs: true, Alias: "renamed_map"},
			},
			tableName:   "user_info",
			keySpace:    "test_keyspace",
			isGroupBy:   false,
			wantColumns: []string{"`map_col` as renamed_map"},
			wantErr:     false,
		},
		{
			name: "Column with alias in GROUP BY",
			selectedColumns: []types.SelectedColumn{
				{Name: "age", IsAs: true, Alias: "user_age"},
			},
			tableName:   "user_info",
			keySpace:    "test_keyspace",
			isGroupBy:   true,
			wantColumns: []string{"TO_INT64(cf1['age']) as user_age"},
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// REFACTOR: Use constructor functions
			cols := []*types.Column{
				{Name: "name", CQLType: datatype.Varchar, KeyType: utilities.KEY_TYPE_REGULAR},
				{Name: "age", CQLType: datatype.Bigint, KeyType: utilities.KEY_TYPE_REGULAR},
				{Name: "code", CQLType: datatype.Int, KeyType: utilities.KEY_TYPE_REGULAR},
				{Name: "map_col", CQLType: datatype.NewMapType(datatype.Varchar, datatype.Varchar), KeyType: utilities.KEY_TYPE_REGULAR},
			}
			tableCfg := schemaMapping.NewTableConfig("test_keyspace", "user_info", "cf1", types.OrderedCodeEncoding, cols)
			schemaCfg := schemaMapping.NewSchemaMappingConfig("schema_mappings", "cf1", zap.NewNop(), []*schemaMapping.TableConfig{tableCfg})
			tr := &Translator{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: schemaCfg,
			}

			tc, err := tr.SchemaMappingConfig.GetTableConfig(tt.keySpace, tt.tableName)
			if err != nil {
				t.Errorf("table config should exist for %s.%s", tt.keySpace, tt.tableName)
				return
			}
			gotColumns, err := processSetStrings(tr, tc, tt.selectedColumns, tt.isGroupBy)

			if (err != nil) != tt.wantErr {
				t.Errorf("processStrings() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if !reflect.DeepEqual(gotColumns, tt.wantColumns) {
					t.Errorf("processStrings() gotColumns = %v, want %v", gotColumns, tt.wantColumns)
				}
			}
		})
	}
}

func Test_processAsColumn(t *testing.T) {
	tests := []struct {
		name           string
		columnMetadata types.SelectedColumn
		tableName      string
		columnFamily   string
		colMeta        *types.Column
		columns        []string
		isGroupBy      bool
		want           []string
	}{
		{
			name: "Non-collection column with GROUP BY",
			columnMetadata: types.SelectedColumn{
				Name:  "pk_1_text",
				Alias: "col1",
			},
			tableName:    "test_table",
			columnFamily: "cf1",
			colMeta: &types.Column{
				Name:    "pk_1_text",
				CQLType: datatype.Varchar,
			},
			columns:   []string{},
			isGroupBy: true,
			want:      []string{"cf1['pk_1_text'] as col1"},
		},
		{
			name: "Non-collection column without GROUP BY",
			columnMetadata: types.SelectedColumn{
				Name:  "pk_1_text",
				Alias: "col1",
			},
			tableName:    "test_table",
			columnFamily: "cf1",
			colMeta: &types.Column{
				Name:    "pk_1_text",
				CQLType: datatype.Varchar,
			},
			columns:   []string{},
			isGroupBy: false,
			want:      []string{"cf1['pk_1_text'] as col1"},
		},
		{
			name: "counter column",
			columnMetadata: types.SelectedColumn{
				Name:  "likes",
				Alias: "l",
			},
			tableName:    "test_table",
			columnFamily: "cf1",
			colMeta: &types.Column{
				Name:    "likes",
				CQLType: datatype.Counter,
			},
			columns:   []string{},
			isGroupBy: false,
			want:      []string{"likes[''] as l"},
		},
		{
			name: "Collection column without GROUP BY",
			columnMetadata: types.SelectedColumn{
				Name:  "map_column",
				Alias: "map1",
			},
			tableName:    "test_table",
			columnFamily: "cf1",
			colMeta: &types.Column{
				CQLType: datatype.NewMapType(datatype.Varchar, datatype.Varchar),
			},
			columns:   []string{},
			isGroupBy: false,
			want:      []string{"`map_column` as map1"},
		},
		{
			name: "With existing columns",
			columnMetadata: types.SelectedColumn{
				Name:  "pk_1_text",
				Alias: "col1",
			},
			tableName:    "test_table",
			columnFamily: "cf1",
			colMeta: &types.Column{
				CQLType: datatype.Varchar,
			},
			columns:   []string{"existing_column"},
			isGroupBy: false,
			want:      []string{"existing_column", "cf1['pk_1_text'] as col1"},
		},
		{
			name: "WriteTime column",
			columnMetadata: types.SelectedColumn{
				Name:              "test_table",
				Alias:             "wt",
				IsWriteTimeColumn: true,
			},
			tableName:    "test_table",
			columnFamily: "cf1",
			colMeta: &types.Column{
				CQLType: datatype.Varchar,
			},
			columns:   []string{},
			isGroupBy: false,
			want:      []string{"cf1['test_table'] as wt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := processAsColumn(tt.columnMetadata, tt.columnFamily, tt.colMeta, tt.columns, tt.isGroupBy)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("processAsColumn() = %v, want %v", got, tt.want)
			}
		})
	}
}
