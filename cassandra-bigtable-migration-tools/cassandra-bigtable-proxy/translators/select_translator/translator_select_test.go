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
package select_translator

import (
	"cloud.google.com/go/bigtable"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/parser"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/testing/mockdata"
	"github.com/stretchr/testify/require"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	"github.com/stretchr/testify/assert"
)

type Want struct {
	Keyspace        types.Keyspace
	Table           types.TableName
	TranslatedQuery string
	SelectClause    *types.SelectClause
	Conditions      []types.Condition
	CachedBTPrepare *bigtable.PreparedStatement
	OrderBy         types.OrderBy
	GroupByColumns  []string
	LimitValue      types.DynamicValue
	AllParams       []types.Placeholder
}

func TestTranslator_TranslateSelectQuerytoBigtable(t *testing.T) {

	tests := []struct {
		name            string
		query           string
		want            *Want
		wantErr         string
		sessionKeyspace types.Keyspace
	}{
		{
			name: "Select query with list contains key clause",
			query: `select pk1, col_int, col_bool from  test_keyspace.test_table
 where pk1 = 'test' AND col_bool=true
 AND col_ts <= '2015-05-03 13:30:54.234' AND col_int >= 123
 AND col_bigint > -10000000 LIMIT 20;`,
			want: &Want{
				TranslatedQuery: "SELECT pk1, TO_INT64(`cf1`['col_int']), TO_INT64(`cf1`['col_bool']) FROM test_table WHERE pk1 = 'test' AND TO_INT64(`cf1`['col_bool']) = 1 AND TIMESTAMP_FROM_UNIX_MILLIS(TO_INT64(`cf1`['col_ts'])) <= TIMESTAMP_FROM_UNIX_MILLIS(1430659854234) AND TO_INT64(`cf1`['col_int']) >= 123 AND TO_INT64(`cf1`['col_bigint']) > -10000000 LIMIT 20;",
				Table:           "test_table",
				Keyspace:        "test_keyspace",
				SelectClause: &types.SelectClause{
					Columns: []types.SelectedColumn{
						*types.NewSelectedColumn("pk1", "pk1", "", types.TypeVarchar),
						*types.NewSelectedColumn("col_int", "col_int", "", types.TypeInt),
						*types.NewSelectedColumn("col_bool", "col_bool", "", types.TypeBoolean),
					},
				},
				Conditions: []types.Condition{
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk1"),
						Operator: types.EQ,
						Value:    types.NewLiteralValue("test"),
					},
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_bool"),
						Operator: types.EQ,
						Value:    types.NewLiteralValue(true),
					},
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_ts"),
						Operator: types.LTE,
						Value:    types.NewLiteralValue(time.UnixMilli(1430659854234).Truncate(time.Nanosecond).UTC()),
					},
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_int"),
						Operator: types.GTE,
						Value:    types.NewLiteralValue(int32(123)),
					},
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_bigint"),
						Operator: types.GT,
						Value:    types.NewLiteralValue(int64(-10000000)),
					},
				},
				OrderBy: types.OrderBy{
					IsOrderBy: false,
				},
				LimitValue: types.NewLiteralValue(int32(20)),
				AllParams:  nil,
			},
			sessionKeyspace: "test_keyspace",
		},
		{
			name:  "Select query with list contains clause",
			query: `select col_int as name from test_keyspace.test_table where list_text CONTAINS 'test';`,
			want: &Want{
				TranslatedQuery: "SELECT TO_INT64(`cf1`['col_int']) as name FROM test_table WHERE ARRAY_INCLUDES(MAP_VALUES(`list_text`), 'test');",
				Table:           "test_table",
				Keyspace:        "test_keyspace",
				SelectClause: &types.SelectClause{
					Columns: []types.SelectedColumn{
						*types.NewSelectedColumn("col_int", "col_int", "name", types.TypeInt),
					},
				},
				Conditions: []types.Condition{
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "test_table", "list_text"),
						Operator: types.CONTAINS,
						Value:    types.NewLiteralValue("test"),
					},
				},
				OrderBy: types.OrderBy{
					IsOrderBy: false,
				},
				AllParams: nil,
			},
			sessionKeyspace: "test_keyspace",
		},
		{
			name:  "Select query with map contains key clause",
			query: `select col_int as name from test_keyspace.test_table where map_text_text CONTAINS KEY 'test';`,
			want: &Want{
				TranslatedQuery: "SELECT TO_INT64(`cf1`['col_int']) as name FROM test_table WHERE MAP_CONTAINS_KEY(`map_text_text`, 'test');",
				Table:           "test_table",
				Keyspace:        "test_keyspace",
				SelectClause: &types.SelectClause{
					Columns: []types.SelectedColumn{
						*types.NewSelectedColumn("col_int", "col_int", "name", types.TypeInt),
					},
				},
				Conditions: []types.Condition{
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "test_table", "map_text_text"),
						Operator: types.CONTAINS_KEY,
						Value:    types.NewLiteralValue("test"),
					},
				},
				OrderBy: types.OrderBy{
					IsOrderBy: false,
				},
				AllParams: nil,
			},
			sessionKeyspace: "test_keyspace",
		},
		{
			name:  "Select query with set contains clause",
			query: `select col_int as name from test_keyspace.test_table where set_text CONTAINS 'test';`,
			want: &Want{
				TranslatedQuery: "SELECT TO_INT64(`cf1`['col_int']) as name FROM test_table WHERE MAP_CONTAINS_KEY(`set_text`, 'test');",
				Table:           "test_table",
				Keyspace:        "test_keyspace",
				SelectClause: &types.SelectClause{
					Columns: []types.SelectedColumn{
						*types.NewSelectedColumn("col_int", "col_int", "name", types.TypeInt),
					},
				},
				Conditions: []types.Condition{
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "test_table", "set_text"),
						Operator: types.CONTAINS,
						Value:    types.NewLiteralValue("test"),
					},
				},
				OrderBy: types.OrderBy{
					IsOrderBy: false,
				},
				AllParams: nil,
			},
			sessionKeyspace: "test_keyspace",
		},
		{
			name:  "Writetime CqlQuery without as keyword",
			query: `select pk1, WRITETIME(col_int) from test_keyspace.test_table where pk1 = 'test';`,
			want: &Want{
				TranslatedQuery: "SELECT pk1, UNIX_MICROS(WRITE_TIMESTAMP(cf1, 'col_int')) FROM test_table WHERE pk1 = 'test';",
				Table:           "test_table",
				Keyspace:        "test_keyspace",
				SelectClause: &types.SelectClause{
					Columns: []types.SelectedColumn{
						*types.NewSelectedColumn("pk1", "pk1", "", types.TypeVarchar),
						*types.NewSelectedColumnFunction("WRITETIME(col_int)", "col_int", "", types.TypeBigInt, types.FuncCodeWriteTime),
					},
				},
				Conditions: []types.Condition{
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk1"),
						Operator: "=",
						Value:    types.NewLiteralValue("test"),
					},
				},
				OrderBy: types.OrderBy{
					IsOrderBy: false,
				},
				AllParams: nil,
			},
			sessionKeyspace: "test_keyspace",
		},
		{
			name:  "IN operator with integer values",
			query: `select pk1 from test_keyspace.test_table where col_int IN (1, 2, 3);`,
			want: &Want{
				TranslatedQuery: "SELECT pk1 FROM test_table WHERE TO_INT64(`cf1`['col_int']) IN UNNEST([1, 2, 3]);",
				Table:           "test_table",
				Keyspace:        "test_keyspace",
				SelectClause: &types.SelectClause{
					Columns: []types.SelectedColumn{
						*types.NewSelectedColumn("pk1", "pk1", "", types.TypeVarchar),
					},
				},
				Conditions: []types.Condition{
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_int"),
						Operator: "IN",
						Value:    types.NewLiteralValue([]any{int32(1), int32(2), int32(3)}),
					},
				},
				AllParams: nil,
			},
			sessionKeyspace: "test_keyspace",
		},
		{
			name:  "IN operator with bigint values",
			query: `select pk1 from test_keyspace.test_table where col_bigint IN (1234567890, 9876543210);`,
			want: &Want{
				TranslatedQuery: "SELECT pk1 FROM test_table WHERE TO_INT64(`cf1`['col_bigint']) IN UNNEST([1234567890, 9876543210]);",
				Table:           "test_table",
				Keyspace:        "test_keyspace",
				SelectClause: &types.SelectClause{
					Columns: []types.SelectedColumn{
						*types.NewSelectedColumn("pk1", "pk1", "", types.TypeVarchar),
					},
				},
				Conditions: []types.Condition{
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_bigint"),
						Operator: "IN",
						Value:    types.NewLiteralValue([]any{int64(1234567890), int64(9876543210)}),
					},
				},
				AllParams: nil,
			},
			sessionKeyspace: "test_keyspace",
		},
		{
			name:  "IN operator with float values",
			query: `select pk1 from test_keyspace.test_table where col_float IN (1.5, 2.5, 3.5);`,
			want: &Want{
				TranslatedQuery: "SELECT pk1 FROM test_table WHERE TO_FLOAT32(`cf1`['col_float']) IN UNNEST([1.5, 2.5, 3.5]);",
				Table:           "test_table",
				Keyspace:        "test_keyspace",
				SelectClause: &types.SelectClause{
					Columns: []types.SelectedColumn{
						*types.NewSelectedColumn("pk1", "pk1", "", types.TypeVarchar),
					},
				},
				Conditions: []types.Condition{
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_float"),
						Operator: "IN",
						Value:    types.NewLiteralValue([]any{float32(1.5), float32(2.5), float32(3.5)}),
					},
				},
				AllParams: nil,
			},
			sessionKeyspace: "test_keyspace",
		},
		{
			name:  "IN operator with double values",
			query: `select pk1 from test_keyspace.test_table where col_double IN (3.1415926535, 2.7182818284);`,
			want: &Want{
				TranslatedQuery: "SELECT pk1 FROM test_table WHERE TO_FLOAT64(`cf1`['col_double']) IN UNNEST([3.1415926535, 2.7182818284]);",
				Table:           "test_table",
				Keyspace:        "test_keyspace",
				SelectClause: &types.SelectClause{
					Columns: []types.SelectedColumn{
						*types.NewSelectedColumn("pk1", "pk1", "", types.TypeVarchar),
					},
				},
				Conditions: []types.Condition{
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_double"),
						Operator: "IN",
						Value:    types.NewLiteralValue([]any{3.1415926535, 2.7182818284}),
					},
				},
				AllParams: nil,
			},
			sessionKeyspace: "test_keyspace",
		},
		{
			name:  "IN operator with boolean values",
			query: `select pk1 from test_keyspace.test_table where col_bool IN (true, false);`,
			want: &Want{
				TranslatedQuery: "SELECT pk1 FROM test_table WHERE TO_INT64(`cf1`['col_bool']) IN UNNEST([1, 0]);",
				Table:           "test_table",
				Keyspace:        "test_keyspace",
				SelectClause: &types.SelectClause{
					Columns: []types.SelectedColumn{
						*types.NewSelectedColumn("pk1", "pk1", "", types.TypeVarchar),
					},
				},
				Conditions: []types.Condition{
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_bool"),
						Operator: "IN",
						Value:    types.NewLiteralValue([]any{true, false}),
					},
				},
				AllParams: nil,
			},
			sessionKeyspace: "test_keyspace",
		},
		{
			name:            "IN operator with mixed types (should error)",
			query:           `select pk1 from test_keyspace.test_table where col_int IN (1, 'two', 3);`,
			wantErr:         "error converting string to int32",
			want:            nil,
			sessionKeyspace: "test_keyspace",
		},
		{
			name:  "IN operator with prepared statement placeholder for Int",
			query: `select pk1 from test_keyspace.test_table where col_int IN ?;`,
			want: &Want{
				TranslatedQuery: "SELECT pk1 FROM test_table WHERE TO_INT64(`cf1`['col_int']) IN UNNEST(@value0);",
				Table:           "test_table",
				Keyspace:        "test_keyspace",
				SelectClause: &types.SelectClause{
					Columns: []types.SelectedColumn{
						*types.NewSelectedColumn("pk1", "pk1", "", types.TypeVarchar),
					},
				},
				AllParams: []types.Placeholder{"@value0"},
				Conditions: []types.Condition{
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_int"),
						Operator: "IN",
						Value:    types.NewParameterizedValue("@value0"),
					},
				},
			},
			sessionKeyspace: "test_keyspace",
		},
		{
			name:            "placeholder within an IN parenthesis is not legal",
			query:           `select pk1 from test_keyspace.test_table where col_int IN (?);`,
			wantErr:         "error converting string to int32",
			sessionKeyspace: "test_keyspace",
		},
		{
			name:  "IN operator with prepared statement placeholder for Bigint",
			query: `select pk1 from test_keyspace.test_table where col_bigint IN ?;`,
			want: &Want{
				TranslatedQuery: "SELECT pk1 FROM test_table WHERE TO_INT64(`cf1`['col_bigint']) IN UNNEST(@value0);",
				Table:           "test_table",
				Keyspace:        "test_keyspace",
				SelectClause: &types.SelectClause{
					Columns: []types.SelectedColumn{
						*types.NewSelectedColumn("pk1", "pk1", "", types.TypeVarchar),
					},
				},
				AllParams: []types.Placeholder{"@value0"},
				Conditions: []types.Condition{
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_bigint"),
						Operator: "IN",
						Value:    types.NewParameterizedValue("@value0"),
					},
				},
			},
			sessionKeyspace: "test_keyspace",
		},
		{
			name:  "IN operator with prepared statement placeholder for Float",
			query: `select pk1 from test_keyspace.test_table where col_float IN ?;`,
			want: &Want{
				TranslatedQuery: "SELECT pk1 FROM test_table WHERE TO_FLOAT32(`cf1`['col_float']) IN UNNEST(@value0);",
				Table:           "test_table",
				Keyspace:        "test_keyspace",
				SelectClause: &types.SelectClause{
					Columns: []types.SelectedColumn{
						*types.NewSelectedColumn("pk1", "pk1", "", types.TypeVarchar),
					},
				},
				AllParams: []types.Placeholder{"@value0"},
				Conditions: []types.Condition{
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_float"),
						Operator: "IN",
						Value:    types.NewParameterizedValue("@value0"),
					},
				},
			},
			sessionKeyspace: "test_keyspace",
		},
		{
			name:  "IN operator with prepared statement placeholder for Double",
			query: `select pk1 from test_keyspace.test_table where col_double IN ?;`,
			want: &Want{
				TranslatedQuery: "SELECT pk1 FROM test_table WHERE TO_FLOAT64(`cf1`['col_double']) IN UNNEST(@value0);",
				Table:           "test_table",
				Keyspace:        "test_keyspace",
				SelectClause: &types.SelectClause{
					Columns: []types.SelectedColumn{
						*types.NewSelectedColumn("pk1", "pk1", "", types.TypeVarchar),
					},
				},
				AllParams: []types.Placeholder{"@value0"},
				Conditions: []types.Condition{
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_double"),
						Operator: "IN",
						Value:    types.NewParameterizedValue("@value0"),
					},
				},
			},
			sessionKeyspace: "test_keyspace",
		},
		{
			name:  "IN operator with prepared statement placeholder for Boolean",
			query: `select pk1 from test_keyspace.test_table where col_bool IN ?;`,
			want: &Want{
				TranslatedQuery: "SELECT pk1 FROM test_table WHERE TO_INT64(`cf1`['col_bool']) IN UNNEST(@value0);",
				Table:           "test_table",
				Keyspace:        "test_keyspace",
				SelectClause: &types.SelectClause{
					Columns: []types.SelectedColumn{
						*types.NewSelectedColumn("pk1", "pk1", "", types.TypeVarchar),
					},
				},
				AllParams: []types.Placeholder{"@value0"},
				Conditions: []types.Condition{
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_bool"),
						Operator: "IN",
						Value:    types.NewParameterizedValue("@value0"),
					},
				},
			},
			sessionKeyspace: "test_keyspace",
		},
		{
			name:  "IN operator with prepared statement placeholder for Blob",
			query: `select pk1 from test_keyspace.test_table where col_blob IN ?;`,
			want: &Want{
				TranslatedQuery: "SELECT pk1 FROM test_table WHERE `cf1`['col_blob'] IN UNNEST(@value0);",
				Table:           "test_table",
				Keyspace:        "test_keyspace",
				SelectClause: &types.SelectClause{
					Columns: []types.SelectedColumn{
						*types.NewSelectedColumn("pk1", "pk1", "", types.TypeVarchar),
					},
				},
				AllParams: []types.Placeholder{"@value0"},
				Conditions: []types.Condition{
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_blob"),
						Operator: "IN",
						Value:    types.NewParameterizedValue("@value0"),
					},
				},
			},
			sessionKeyspace: "test_keyspace",
		},
		{
			name:            "IN operator with unsupported CQL type",
			query:           `select pk1 from test_keyspace.test_table where col_udt IN ?;`,
			wantErr:         "unknown column 'col_udt' in table test_keyspace.test_table",
			want:            nil,
			sessionKeyspace: "test_keyspace",
		},
		{
			name:  "Writetime CqlQuery with as keyword",
			query: `select pk1, WRITETIME(col_int) as name from test_keyspace.test_table where pk1 = 'test';`,
			want: &Want{
				TranslatedQuery: "SELECT pk1, UNIX_MICROS(WRITE_TIMESTAMP(cf1, 'col_int')) AS name FROM test_table WHERE pk1 = 'test';",
				Table:           "test_table",
				Keyspace:        "test_keyspace",
				SelectClause: &types.SelectClause{
					Columns: []types.SelectedColumn{
						*types.NewSelectedColumn("pk1", "pk1", "", types.TypeVarchar),
						*types.NewSelectedColumnFunction("WRITETIME(col_int)", "col_int", "name", types.TypeBigInt, types.FuncCodeWriteTime),
					},
				},
				Conditions: []types.Condition{
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk1"),
						Operator: "=",
						Value:    types.NewLiteralValue("test"),
					},
				},
				OrderBy: types.OrderBy{
					IsOrderBy: false,
				},
				AllParams: nil,
			},
			sessionKeyspace: "test_keyspace",
		},
		{
			name:  "As CqlQuery",
			query: `select col_int as name from test_keyspace.test_table where pk1 = 'test';`,
			want: &Want{
				TranslatedQuery: "SELECT TO_INT64(`cf1`['col_int']) as name FROM test_table WHERE pk1 = 'test';",
				Table:           "test_table",
				Keyspace:        "test_keyspace",
				SelectClause: &types.SelectClause{
					Columns: []types.SelectedColumn{
						*types.NewSelectedColumn("col_int", "col_int", "name", types.TypeInt),
					},
				},
				Conditions: []types.Condition{
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk1"),
						Operator: "=",
						Value:    types.NewLiteralValue("test"),
					},
				},
				OrderBy: types.OrderBy{
					IsOrderBy: false,
				},
				AllParams: nil,
			},
			sessionKeyspace: "test_keyspace",
		},
		{
			name:            "CqlQuery without Columns",
			query:           `select from test_keyspace.test_table where pk1 = 'test'`,
			wantErr:         "parsing error",
			want:            nil,
			sessionKeyspace: "test_keyspace",
		},
		{
			name:            "test for raw query when column name not exist in schema mapping table",
			query:           "select pk101, col_int, col_bool from  test_keyspace.test_table where pk1 = 'test' AND col_bool='true' AND col_ts <= '2015-05-03 13:30:54.234' AND col_int >= '123' AND col_bigint > '-10000000' LIMIT 20000;",
			want:            nil,
			wantErr:         "unknown column 'pk101' in table test_keyspace.test_table",
			sessionKeyspace: "test_keyspace",
		},
		{
			name:            "CqlQuery without keyspace name",
			query:           `select pk1, col_int from test_table where pk1 = '?' and pk1 in ('?', '?');`,
			wantErr:         "no keyspace specified",
			want:            nil,
			sessionKeyspace: "",
		},
		{
			name:            "MALFORMED QUERY",
			query:           "MALFORMED QUERY",
			wantErr:         "parsing error",
			want:            nil,
			sessionKeyspace: "test_keyspace",
		},
		{
			name:  "test for parameterized query success",
			query: "select pk1, col_int, col_bool from  test_keyspace.test_table where pk1 = ? AND col_bool=? AND col_ts <= ? AND col_int >= ? AND col_bigint > ? AND col_bigint < ? LIMIT 20000;",
			want: &Want{
				TranslatedQuery: "SELECT pk1, TO_INT64(`cf1`['col_int']), TO_INT64(`cf1`['col_bool']) FROM test_table WHERE pk1 = @value0 AND TO_INT64(`cf1`['col_bool']) = @value1 AND TIMESTAMP_FROM_UNIX_MILLIS(TO_INT64(`cf1`['col_ts'])) <= @value2 AND TO_INT64(`cf1`['col_int']) >= @value3 AND TO_INT64(`cf1`['col_bigint']) > @value4 AND TO_INT64(`cf1`['col_bigint']) < @value5 LIMIT 20000;",
				Table:           "test_table",
				Keyspace:        "test_keyspace",
				SelectClause: &types.SelectClause{
					Columns: []types.SelectedColumn{
						*types.NewSelectedColumn("pk1", "pk1", "", types.TypeVarchar),
						*types.NewSelectedColumn("col_int", "col_int", "", types.TypeInt),
						*types.NewSelectedColumn("col_bool", "col_bool", "", types.TypeBoolean),
					},
				},
				Conditions: []types.Condition{
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk1"),
						Operator: "=",
						Value:    types.NewParameterizedValue("@value0"),
					},
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_bool"),
						Operator: "=",
						Value:    types.NewParameterizedValue("@value1"),
					},
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_ts"),
						Operator: "<=",
						Value:    types.NewParameterizedValue("@value2"),
					},
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_int"),
						Operator: ">=",
						Value:    types.NewParameterizedValue("@value3"),
					},
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_bigint"),
						Operator: ">",
						Value:    types.NewParameterizedValue("@value4"),
					},
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_bigint"),
						Operator: "<",
						Value:    types.NewParameterizedValue("@value5"),
					},
				},
				OrderBy: types.OrderBy{
					IsOrderBy: false,
				},
				LimitValue: types.NewLiteralValue(int32(20000)),
				AllParams:  []types.Placeholder{"@value0", "@value1", "@value2", "@value3", "@value4", "@value5"},
			},
			sessionKeyspace: "test_keyspace",
		},
		{
			name:  "test for prepared query success",
			query: "select pk1, col_int, col_bool from test_keyspace.test_table where pk1 = ? AND col_int=? AND col_bool=? AND col_ts=? AND col_int=? AND col_bigint=?;",
			want: &Want{
				TranslatedQuery: "SELECT pk1, TO_INT64(`cf1`['col_int']), TO_INT64(`cf1`['col_bool']) FROM test_table WHERE pk1 = @value0 AND TO_INT64(`cf1`['col_int']) = @value1 AND TO_INT64(`cf1`['col_bool']) = @value2 AND TIMESTAMP_FROM_UNIX_MILLIS(TO_INT64(`cf1`['col_ts'])) = @value3 AND TO_INT64(`cf1`['col_int']) = @value4 AND TO_INT64(`cf1`['col_bigint']) = @value5;",
				Table:           "test_table",
				Keyspace:        "test_keyspace",
				SelectClause: &types.SelectClause{
					Columns: []types.SelectedColumn{
						*types.NewSelectedColumn("pk1", "pk1", "", types.TypeVarchar),
						*types.NewSelectedColumn("col_int", "col_int", "", types.TypeInt),
						*types.NewSelectedColumn("col_bool", "col_bool", "", types.TypeBoolean),
					},
				},
				Conditions: []types.Condition{
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk1"),
						Operator: "=",
						Value:    types.NewParameterizedValue("@value0"),
					},
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_int"),
						Operator: "=",
						Value:    types.NewParameterizedValue("@value1"),
					},
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_bool"),
						Operator: "=",
						Value:    types.NewParameterizedValue("@value2"),
					},
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_ts"),
						Operator: "=",
						Value:    types.NewParameterizedValue("@value3"),
					},
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_int"),
						Operator: "=",
						Value:    types.NewParameterizedValue("@value4"),
					},
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_bigint"),
						Operator: "=",
						Value:    types.NewParameterizedValue("@value5"),
					},
				},
				OrderBy: types.OrderBy{
					IsOrderBy: false,
				},
				AllParams: []types.Placeholder{"@value0", "@value1", "@value2", "@value3", "@value4", "@value5"},
			},
			sessionKeyspace: "test_keyspace",
		},
		{
			name:  "test for query without clause success",
			query: `select pk1, col_int, col_bool from test_keyspace.test_table ORDER BY pk1 LIMIT 20000;`,
			want: &Want{
				TranslatedQuery: "SELECT pk1, TO_INT64(`cf1`['col_int']), TO_INT64(`cf1`['col_bool']) FROM test_table ORDER BY pk1 asc LIMIT 20000;",
				Table:           "test_table",
				Keyspace:        "test_keyspace",
				SelectClause: &types.SelectClause{
					Columns: []types.SelectedColumn{
						*types.NewSelectedColumn("pk1", "pk1", "", types.TypeVarchar),
						*types.NewSelectedColumn("col_int", "col_int", "", types.TypeInt),
						*types.NewSelectedColumn("col_bool", "col_bool", "", types.TypeBoolean),
					},
				},
				LimitValue: types.NewLiteralValue(int32(20000)),
				AllParams:  nil,
				OrderBy: types.OrderBy{
					IsOrderBy: true,
					Columns: []types.OrderByColumn{
						{
							Column:    "pk1",
							Operation: types.Asc,
						},
					},
				},
			},
			sessionKeyspace: "test_keyspace",
		},
		{
			name:            "error at Columns parsing",
			query:           "select  from table;",
			want:            nil,
			wantErr:         "parsing error",
			sessionKeyspace: "test_keyspace",
		},
		{
			name:            "error at Condition parsing when column type not found",
			query:           "select * from test_keyspace.test_table where name='test';",
			want:            nil,
			wantErr:         "unknown column 'name' in table test_keyspace.test_table",
			sessionKeyspace: "test_keyspace",
		},
		{
			name:            "error at Condition parsing when value invalid",
			query:           "select * from test_keyspace.test_table where pk1=",
			want:            nil,
			wantErr:         "parsing error",
			sessionKeyspace: "test_keyspace",
		},
		{
			name:  "test with IN operator raw query",
			query: `select pk1, col_int from test_keyspace.test_table where pk1 = 'test' and pk1 in ('abc', 'xyz');`,
			want: &Want{
				TranslatedQuery: "SELECT pk1, TO_INT64(`cf1`['col_int']) FROM test_table WHERE pk1 = 'test' AND pk1 IN UNNEST(['abc', 'xyz']);",
				Keyspace:        "test_keyspace",
				Table:           "test_table",
				SelectClause: &types.SelectClause{
					Columns: []types.SelectedColumn{
						*types.NewSelectedColumn("pk1", "pk1", "", types.TypeVarchar),
						*types.NewSelectedColumn("col_int", "col_int", "", types.TypeInt),
					},
				},
				AllParams: nil,
				Conditions: []types.Condition{
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk1"),
						Operator: "=",
						Value:    types.NewLiteralValue("test"),
					},
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk1"),
						Operator: "IN",
						Value:    types.NewLiteralValue([]any{"abc", "xyz"}),
					},
				},
			},
			sessionKeyspace: "test_keyspace",
		},
		{
			name:  "test with IN operator prepared query",
			query: `select pk1, col_int from test_keyspace.test_table where pk1 = ? and pk1 in ?;`,
			want: &Want{
				TranslatedQuery: "SELECT pk1, TO_INT64(`cf1`['col_int']) FROM test_table WHERE pk1 = @value0 AND pk1 IN UNNEST(@value1);",
				Keyspace:        "test_keyspace",
				Table:           "test_table",
				SelectClause: &types.SelectClause{
					Columns: []types.SelectedColumn{
						*types.NewSelectedColumn("pk1", "pk1", "", types.TypeVarchar),
						*types.NewSelectedColumn("col_int", "col_int", "", types.TypeInt),
					},
				},
				AllParams: []types.Placeholder{"@value0", "@value1"},
				Conditions: []types.Condition{
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk1"),
						Operator: "=",
						Value:    types.NewParameterizedValue("@value0"),
					},
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk1"),
						Operator: "IN",
						Value:    types.NewParameterizedValue("@value1"),
					},
				},
			},
			sessionKeyspace: "test_keyspace",
		},
		{
			name:  "test with LIKE keyword raw query",
			query: `select pk1, col_int from test_keyspace.test_table where pk1 like 'test%';`,
			want: &Want{
				TranslatedQuery: "SELECT pk1, TO_INT64(`cf1`['col_int']) FROM test_table WHERE pk1 LIKE 'test%';",
				Keyspace:        "test_keyspace",
				Table:           "test_table",
				SelectClause: &types.SelectClause{
					Columns: []types.SelectedColumn{
						*types.NewSelectedColumn("pk1", "pk1", "", types.TypeVarchar),
						*types.NewSelectedColumn("col_int", "col_int", "", types.TypeInt),
					},
				},
				AllParams: nil,
				Conditions: []types.Condition{
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk1"),
						Operator: "LIKE",
						Value:    types.NewLiteralValue("test%"),
					},
				},
			},
		}, {
			name:  "test with BETWEEN operator raw query",
			query: `select pk1, col_int from test_keyspace.test_table where pk1 between 'te''st' and 'test2';`,
			want: &Want{
				TranslatedQuery: "SELECT pk1, TO_INT64(`cf1`['col_int']) FROM test_table WHERE pk1 BETWEEN 'te\\'st' AND 'test2';",
				Keyspace:        "test_keyspace",
				Table:           "test_table",
				SelectClause: &types.SelectClause{
					Columns: []types.SelectedColumn{
						*types.NewSelectedColumn("pk1", "pk1", "", types.TypeVarchar),
						*types.NewSelectedColumn("col_int", "col_int", "", types.TypeInt),
					},
				},
				AllParams: nil,
				Conditions: []types.Condition{
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk1"),
						Operator: "BETWEEN",
						Value:    types.NewLiteralValue("te'st"),
						Value2:   types.NewLiteralValue("test2"),
					},
				},
			},
		},
		{
			name:    "test with BETWEEN operator raw query with single value",
			query:   `select pk1, col_int from test_keyspace.test_table where pk1 between 'test';`,
			wantErr: "parsing error",
			want:    nil,
		},
		{
			name:  "test with LIKE keyword prepared query",
			query: `select pk1, col_int from test_keyspace.test_table where pk1 like ?;`,
			want: &Want{
				TranslatedQuery: "SELECT pk1, TO_INT64(`cf1`['col_int']) FROM test_table WHERE pk1 LIKE @value0;",
				Keyspace:        "test_keyspace",
				Table:           "test_table",
				SelectClause: &types.SelectClause{
					Columns: []types.SelectedColumn{
						*types.NewSelectedColumn("pk1", "pk1", "", types.TypeVarchar),
						*types.NewSelectedColumn("col_int", "col_int", "", types.TypeInt),
					},
				},
				AllParams: []types.Placeholder{"@value0"},
				Conditions: []types.Condition{
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk1"),
						Operator: "LIKE",
						Value:    types.NewParameterizedValue("@value0"),
					},
				},
			},
		},
		{
			name:  "test with BETWEEN operator prepared query",
			query: `select pk1, col_int from test_keyspace.test_table where pk1 between ? and ?;`,
			want: &Want{
				TranslatedQuery: "SELECT pk1, TO_INT64(`cf1`['col_int']) FROM test_table WHERE pk1 BETWEEN @value0 AND @value1;",
				Keyspace:        "test_keyspace",
				Table:           "test_table",
				SelectClause: &types.SelectClause{
					Columns: []types.SelectedColumn{
						*types.NewSelectedColumn("pk1", "pk1", "", types.TypeVarchar),
						*types.NewSelectedColumn("col_int", "col_int", "", types.TypeInt),
					},
				},
				AllParams: []types.Placeholder{"@value0", "@value1"},
				Conditions: []types.Condition{
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk1"),
						Operator: "BETWEEN",
						Value:    types.NewParameterizedValue("@value0"),
						Value2:   types.NewParameterizedValue("@value1"),
					},
				},
			},
		},
		{
			name:            "Empty CqlQuery",
			query:           "",
			wantErr:         "parsing error",
			want:            nil,
			sessionKeyspace: "test_keyspace",
		},
		{
			name:            "CqlQuery Without Select Object",
			query:           "UPDATE table_name SET pk1 = 'new_value1', col_int = 'new_value2' WHERE condition;",
			wantErr:         "mismatched input 'UPDATE' expecting 'SELECT'",
			want:            nil,
			sessionKeyspace: "test_keyspace",
		},
		{
			name:            "query with missing limit value",
			query:           `select pk1, col_int, col_bool from test_keyspace.test_table ORDER BY pk1 LIMIT;`,
			want:            nil,
			wantErr:         "parsing error",
			sessionKeyspace: "test_keyspace",
		},
		{
			name:            "query with non-existent column in ORDER BY",
			query:           `select pk1, col_int, col_bool from test_keyspace.test_table ORDER BY pk12343 LIMIT 100;`,
			want:            nil,
			wantErr:         "unknown column name 'pk12343' in table test_keyspace.test_table",
			sessionKeyspace: "test_keyspace",
		},
		{
			name:            "query with negative LIMIT value",
			query:           `select pk1, col_int, col_bool from test_keyspace.test_table ORDER BY pk1 LIMIT -100;`,
			want:            nil,
			wantErr:         "parsing error",
			sessionKeyspace: "test_keyspace",
		},
		{
			name:  "With keyspace in query, with default keyspace (should use query keyspace)",
			query: `select pk1 from test_keyspace.test_table where pk1 = 'abc';`,
			want: &Want{
				Keyspace:        "test_keyspace",
				Table:           "test_table",
				TranslatedQuery: "SELECT pk1 FROM test_table WHERE pk1 = 'abc';",
				SelectClause: &types.SelectClause{
					Columns: []types.SelectedColumn{
						*types.NewSelectedColumn("pk1", "pk1", "", types.TypeVarchar),
					},
				},
				AllParams:  nil,
				Conditions: []types.Condition{{Column: mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk1"), Operator: "=", Value: types.NewLiteralValue("abc")}},
			},
			sessionKeyspace: "other_keyspace",
		},
		{
			name:  "Without keyspace in query, with default keyspace (should use default)",
			query: `select pk1 from test_table where pk1 = 'abc';`,
			want: &Want{
				Keyspace:        "test_keyspace",
				Table:           "test_table",
				TranslatedQuery: "SELECT pk1 FROM test_table WHERE pk1 = 'abc';",
				SelectClause: &types.SelectClause{
					Columns: []types.SelectedColumn{
						*types.NewSelectedColumn("pk1", "pk1", "", types.TypeVarchar),
					},
				},
				AllParams:  nil,
				Conditions: []types.Condition{{Column: mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk1"), Operator: "=", Value: types.NewLiteralValue("abc")}},
			},
			sessionKeyspace: "test_keyspace",
		},
		{
			name:            "Invalid keyspace/table (not in schema)",
			query:           `select pk1 from invalid_keyspace.invalid_table where pk1 = 'abc';`,
			wantErr:         "keyspace 'invalid_keyspace' does not exist",
			want:            nil,
			sessionKeyspace: "test_keyspace",
		},
		{
			name:            "Parser returns nil/empty for table or keyspace (simulate parser edge cases)",
			query:           `select * from ;`,
			wantErr:         "parsing error",
			want:            nil,
			sessionKeyspace: "test_keyspace",
		},
		{
			name:            "CqlQuery with only table, no keyspace, and sessionKeyspace is empty (should error)",
			query:           `select pk1 from test_table;`,
			wantErr:         "no keyspace specified",
			want:            nil,
			sessionKeyspace: "",
		},
		{
			name:            "CqlQuery with complex WHERE conditions",
			query:           `select pk1, col_int from test_keyspace.test_table where pk1 = 'test' AND (col_int > 100 OR col_int < 50);`,
			wantErr:         "parsing error",
			sessionKeyspace: "test_keyspace",
		},
		{
			name:            "CqlQuery with multiple aggregate functions",
			query:           `select pk1, avg(col_int) as avg_col2, max(col_bool) as max_col3, min(col_int) as min_col4 from test_keyspace.test_table GROUP BY pk1;`,
			wantErr:         "invalid aggregate type: boolean",
			sessionKeyspace: "test_keyspace",
		},
		{
			name:  "CqlQuery with multiple aggregate functions",
			query: `select pk1 from test_keyspace.test_table LIMIT ?;`,
			// todo want success
			want: &Want{
				Keyspace:        "test_keyspace",
				Table:           "test_table",
				TranslatedQuery: "SELECT pk1 FROM test_table LIMIT @value0;",
				SelectClause: &types.SelectClause{
					Columns: []types.SelectedColumn{
						*types.NewSelectedColumn("pk1", "pk1", "", types.TypeVarchar),
					},
				},
				OrderBy:        types.OrderBy{},
				GroupByColumns: nil,
				LimitValue:     types.NewParameterizedValue("@value0"),
				AllParams:      []types.Placeholder{"@value0"},
			},
			sessionKeyspace: "test_keyspace",
		},
		{
			name:  "CqlQuery with multiple aggregate functions",
			query: `SELECT tags FROM test_keyspace.user_info WHERE tags CONTAINS ?;`,
			want: &Want{
				Keyspace:        "test_keyspace",
				Table:           "user_info",
				TranslatedQuery: "SELECT MAP_VALUES(`tags`) FROM user_info WHERE ARRAY_INCLUDES(MAP_VALUES(`tags`), @value0);",
				SelectClause: &types.SelectClause{
					Columns: []types.SelectedColumn{
						*types.NewSelectedColumn("tags", "tags", "", types.NewListType(types.TypeText)),
					},
				},
				Conditions: []types.Condition{
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "user_info", "tags"),
						Operator: types.CONTAINS,
						Value:    types.NewParameterizedValue("@value0"),
					},
				},
				OrderBy:        types.OrderBy{},
				GroupByColumns: nil,
				AllParams:      []types.Placeholder{"@value0"},
			},
			sessionKeyspace: "test_keyspace",
		},
		{
			name:            "where clause comparison functions not supported yet",
			query:           `SELECT count(*) FROM test_table WHERE name = ? AND col_ts < ToTimestamp(now());`,
			wantErr:         "parsing error",
			sessionKeyspace: "test_keyspace",
		},
		{
			name:  "where clause comparison functions not supported yet",
			query: `SELECT count(*) FROM test_table WHERE col_ts < '2025-12-02 10:30:00';`,
			want: &Want{
				Keyspace:        "test_keyspace",
				Table:           "test_table",
				TranslatedQuery: "SELECT count(*) FROM test_table WHERE TIMESTAMP_FROM_UNIX_MILLIS(TO_INT64(`cf1`['col_ts'])) < TIMESTAMP_FROM_UNIX_MILLIS(1764671400000);",
				SelectClause: &types.SelectClause{
					Columns: []types.SelectedColumn{
						*types.NewSelectedColumnFunction("system.count(*)", "*", "", types.TypeBigInt, types.FuncCodeCount),
					},
				},
				Conditions: []types.Condition{
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_ts"),
						Operator: types.LT,
						Value:    types.NewLiteralValue(time.Date(2025, 12, 2, 10, 30, 0, 0, time.UTC)),
					},
				},
				OrderBy:        types.OrderBy{},
				GroupByColumns: nil,
				AllParams:      nil,
			},
			sessionKeyspace: "test_keyspace",
		},
		{
			name:  "literal timestamp int",
			query: `SELECT count(*) FROM test_table WHERE col_ts < 1764671400000;`,
			want: &Want{
				Keyspace:        "test_keyspace",
				Table:           "test_table",
				TranslatedQuery: "SELECT count(*) FROM test_table WHERE TIMESTAMP_FROM_UNIX_MILLIS(TO_INT64(`cf1`['col_ts'])) < TIMESTAMP_FROM_UNIX_MILLIS(1764671400000);",
				SelectClause: &types.SelectClause{
					Columns: []types.SelectedColumn{
						*types.NewSelectedColumnFunction("system.count(*)", "*", "", types.TypeBigInt, types.FuncCodeCount),
					},
				},
				Conditions: []types.Condition{
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_ts"),
						Operator: types.LT,
						Value:    types.NewLiteralValue(time.Date(2025, 12, 2, 10, 30, 0, 0, time.UTC)),
					},
				},
				OrderBy:        types.OrderBy{},
				GroupByColumns: nil,
				AllParams:      nil,
			},
			sessionKeyspace: "test_keyspace",
		},
		{
			name:  "literal blob",
			query: `SELECT pk, name, val FROM blob_table WHERE pk=0x39383233666A61732C766D3266 AND val=0x706B6A787A ALLOW FILTERING`,
			want: &Want{
				Keyspace:        "test_keyspace",
				Table:           "blob_table",
				TranslatedQuery: "SELECT pk, name, `cf1`['val'] FROM blob_table WHERE pk = FROM_BASE64('OTgyM2ZqYXMsdm0yZg==') AND `cf1`['val'] = FROM_BASE64('cGtqeHo=');",
				SelectClause: &types.SelectClause{
					Columns: []types.SelectedColumn{
						*types.NewSelectedColumn("pk", "pk", "", types.TypeBlob),
						*types.NewSelectedColumn("name", "name", "", types.TypeVarchar),
						*types.NewSelectedColumn("val", "val", "", types.TypeBlob),
					},
				},
				Conditions: []types.Condition{
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "blob_table", "pk"),
						Operator: types.EQ,
						Value:    types.NewLiteralValue([]byte{0x39, 0x38, 0x32, 0x33, 0x66, 0x6a, 0x61, 0x73, 0x2c, 0x76, 0x6d, 0x32, 0x66}),
					},
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "blob_table", "val"),
						Operator: types.EQ,
						Value:    types.NewLiteralValue([]byte{0x70, 0x6b, 0x6a, 0x78, 0x7a}),
					},
				},
				OrderBy:        types.OrderBy{},
				GroupByColumns: nil,
				AllParams:      nil,
			},
			sessionKeyspace: "test_keyspace",
		},
		{
			name:  "literal ascii",
			query: `SELECT col_ascii FROM test_table WHERE pk1='foo' AND pk2='bar' AND col_ascii='fizz' ALLOW FILTERING`,
			want: &Want{
				Keyspace:        "test_keyspace",
				Table:           "test_table",
				TranslatedQuery: "SELECT `cf1`['col_ascii'] FROM test_table WHERE pk1 = 'foo' AND pk2 = 'bar' AND `cf1`['col_ascii'] = 'fizz';",
				SelectClause: &types.SelectClause{
					Columns: []types.SelectedColumn{
						*types.NewSelectedColumn("col_ascii", "col_ascii", "", types.TypeAscii),
					},
				},
				Conditions: []types.Condition{
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk1"),
						Operator: types.EQ,
						Value:    types.NewLiteralValue("foo"),
					},
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk2"),
						Operator: types.EQ,
						Value:    types.NewLiteralValue("bar"),
					},
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_ascii"),
						Operator: types.EQ,
						Value:    types.NewLiteralValue("fizz"),
					},
				},
				OrderBy:        types.OrderBy{},
				GroupByColumns: nil,
				AllParams:      nil,
			},
			sessionKeyspace: "test_keyspace",
		},
		{
			name:            "invalid ascii",
			query:           `SELECT col_ascii FROM test_table WHERE pk1='foo' AND pk2='bar' AND col_ascii='éàöñ' ALLOW FILTERING`,
			wantErr:         "string is not valid ascii",
			sessionKeyspace: "test_keyspace",
		},
		{
			name:  "literal blob lte",
			query: `SELECT pk, name, val FROM blob_table where pk <= 0x02 AND name='blob-comparison-operators' ALLOW FILTERING`,
			want: &Want{
				Keyspace:        "test_keyspace",
				Table:           "blob_table",
				TranslatedQuery: "SELECT pk, name, `cf1`['val'] FROM blob_table WHERE pk <= FROM_BASE64('Ag==') AND name = 'blob-comparison-operators';",
				SelectClause: &types.SelectClause{
					Columns: []types.SelectedColumn{
						*types.NewSelectedColumn("pk", "pk", "", types.TypeBlob),
						*types.NewSelectedColumn("name", "name", "", types.TypeVarchar),
						*types.NewSelectedColumn("val", "val", "", types.TypeBlob),
					},
				},
				Conditions: []types.Condition{
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "blob_table", "pk"),
						Operator: types.LTE,
						Value:    types.NewLiteralValue([]byte{0x02}),
					},
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "blob_table", "name"),
						Operator: types.EQ,
						Value:    types.NewLiteralValue("blob-comparison-operators"),
					},
				},
				OrderBy:        types.OrderBy{},
				GroupByColumns: nil,
				AllParams:      nil,
			},
			sessionKeyspace: "test_keyspace",
		},
		{
			name:  "Valid GROUP BY with aggregate and ORDER BY",
			query: `select pk1, count(col_int) from test_keyspace.test_table where pk1 = 'test' GROUP BY pk1 ORDER BY pk1;`,
			want: &Want{
				TranslatedQuery: "SELECT pk1, count(TO_INT64(`cf1`['col_int'])) FROM test_table WHERE pk1 = 'test' GROUP BY pk1 ORDER BY pk1 asc;",
				Table:           "test_table",
				Keyspace:        "test_keyspace",
				SelectClause: &types.SelectClause{
					Columns: []types.SelectedColumn{
						*types.NewSelectedColumn("pk1", "pk1", "", types.TypeVarchar),
						*types.NewSelectedColumnFunction("system.count(col_int)", "col_int", "", types.TypeBigInt, types.FuncCodeCount),
					},
				},
				Conditions: []types.Condition{
					{
						Column:   mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk1"),
						Operator: "=",
						Value:    types.NewLiteralValue("test"),
					},
				},
				AllParams: nil,
				GroupByColumns: []string{
					"pk1",
				},
				OrderBy: types.OrderBy{
					IsOrderBy: true,
					Columns: []types.OrderByColumn{
						{
							Column:    "pk1",
							Operation: types.Asc,
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := NewSelectTranslator(mockdata.GetSchemaMappingConfig())
			got, err := tr.Translate(types.NewRawQuery(nil, tt.sessionKeyspace, tt.query, parser.NewParser(tt.query), types.QueryTypeSelect), tt.sessionKeyspace)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			gotSelect := got.(*types.PreparedSelectQuery)
			assert.Equal(t, tt.want.Keyspace, gotSelect.Keyspace())
			assert.Equal(t, tt.want.Table, gotSelect.Table())
			assert.Equal(t, tt.want.TranslatedQuery, gotSelect.TranslatedQuery)
			assert.Equal(t, tt.want.SelectClause, gotSelect.SelectClause)
			assert.Equal(t, tt.want.Conditions, gotSelect.Conditions)
			assert.Equal(t, tt.want.CachedBTPrepare, gotSelect.CachedBTPrepare)
			assert.Equal(t, tt.want.LimitValue, gotSelect.LimitValue)
			assert.Equal(t, tt.want.OrderBy, gotSelect.OrderBy)
			assert.Equal(t, tt.want.GroupByColumns, gotSelect.GroupByColumns)
			assert.Equal(t, tt.want.AllParams, gotSelect.Params.AllKeys())
		})
	}
}
