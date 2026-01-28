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

package create_translator

import (
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/parser"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/testing/mockdata"
	"testing"

	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	u "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/utilities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTranslateCreateTableToBigtable(t *testing.T) {
	type Want struct {
		Keyspace          types.Keyspace
		Table             types.TableName
		IfNotExists       bool
		Columns           []types.CreateColumn
		PrimaryKeys       []types.CreateTablePrimaryKeyConfig
		IntRowKeyEncoding types.IntRowKeyEncodingType
	}

	tests := []struct {
		name                     string
		query                    string
		defaultIntRowKeyEncoding types.IntRowKeyEncodingType
		want                     *Want
		error                    string
		defaultKeyspace          types.Keyspace
	}{
		{
			name:                     "success",
			query:                    "CREATE TABLE my_keyspace.my_table (user_id varchar, order_num int, name varchar, PRIMARY KEY (user_id, order_num))",
			defaultIntRowKeyEncoding: types.OrderedCodeEncoding,
			want: &Want{
				Table:             "my_table",
				Keyspace:          "my_keyspace",
				IfNotExists:       false,
				IntRowKeyEncoding: types.OrderedCodeEncoding,
				Columns: []types.CreateColumn{
					{
						Name:     "user_id",
						Index:    0,
						TypeInfo: u.ParseCqlTypeOrDie("varchar"),
					},
					{
						Name:     "order_num",
						Index:    1,
						TypeInfo: u.ParseCqlTypeOrDie("int"),
					},
					{
						Name:     "name",
						Index:    2,
						TypeInfo: u.ParseCqlTypeOrDie("varchar"),
					},
				},
				PrimaryKeys: []types.CreateTablePrimaryKeyConfig{
					{
						Name:    "user_id",
						KeyType: "partition_key",
					},
					{
						Name:    "order_num",
						KeyType: "clustering",
					},
				},
			},
			error:           "",
			defaultKeyspace: "my_keyspace",
		},
		{
			name:                     "if not exists",
			query:                    "CREATE TABLE IF NOT EXISTS my_keyspace.my_table (user_id varchar, order_num int, name varchar, PRIMARY KEY (user_id))",
			defaultIntRowKeyEncoding: types.OrderedCodeEncoding,
			want: &Want{
				Table:             "my_table",
				Keyspace:          "my_keyspace",
				IntRowKeyEncoding: types.OrderedCodeEncoding,
				IfNotExists:       true,
				Columns: []types.CreateColumn{
					{
						Name:     "user_id",
						Index:    0,
						TypeInfo: u.ParseCqlTypeOrDie("varchar"),
					},
					{
						Name:     "order_num",
						Index:    1,
						TypeInfo: u.ParseCqlTypeOrDie("int"),
					},
					{
						Name:     "name",
						Index:    2,
						TypeInfo: u.ParseCqlTypeOrDie("varchar"),
					},
				},
				PrimaryKeys: []types.CreateTablePrimaryKeyConfig{
					{
						Name:    "user_id",
						KeyType: "partition_key",
					},
				},
			},
			error:           "",
			defaultKeyspace: "my_keyspace",
		},
		{
			name:                     "single inline primary key",
			query:                    "CREATE TABLE cycling.cyclist_name (id varchar PRIMARY KEY, lastname varchar, firstname varchar);",
			defaultIntRowKeyEncoding: types.OrderedCodeEncoding,
			want: &Want{
				Table:             "cyclist_name",
				Keyspace:          "cycling",
				IntRowKeyEncoding: types.OrderedCodeEncoding,
				IfNotExists:       false,
				Columns: []types.CreateColumn{
					{
						Name:     "id",
						Index:    0,
						TypeInfo: u.ParseCqlTypeOrDie("varchar"),
					},
					{
						Name:     "lastname",
						Index:    1,
						TypeInfo: u.ParseCqlTypeOrDie("varchar"),
					},
					{
						Name:     "firstname",
						Index:    2,
						TypeInfo: u.ParseCqlTypeOrDie("varchar"),
					},
				},
				PrimaryKeys: []types.CreateTablePrimaryKeyConfig{
					{
						Name:    "id",
						KeyType: types.KeyTypePartition,
					},
				},
			},
			error:           "",
			defaultKeyspace: "cycling",
		},
		{
			name:                     "composite primary key",
			query:                    "CREATE TABLE cycling.cyclist_composite (id text, lastname varchar, firstname varchar, PRIMARY KEY ((id, lastname), firstname)));",
			defaultIntRowKeyEncoding: types.OrderedCodeEncoding,
			want: &Want{
				Table:             "cyclist_composite",
				Keyspace:          "cycling",
				IntRowKeyEncoding: types.OrderedCodeEncoding,
				IfNotExists:       false,
				Columns: []types.CreateColumn{
					{
						Name:     "id",
						Index:    0,
						TypeInfo: u.ParseCqlTypeOrDie("text"),
					},
					{
						Name:     "lastname",
						Index:    1,
						TypeInfo: u.ParseCqlTypeOrDie("varchar"),
					},
					{
						Name:     "firstname",
						Index:    2,
						TypeInfo: u.ParseCqlTypeOrDie("varchar"),
					},
				},
				PrimaryKeys: []types.CreateTablePrimaryKeyConfig{
					{
						Name:    "id",
						KeyType: "partition_key",
					},
					{
						Name:    "lastname",
						KeyType: "partition_key",
					},
					{
						Name:    "firstname",
						KeyType: "clustering",
					},
				},
			},
			error:           "",
			defaultKeyspace: "cycling",
		},
		{
			name:                     "with keyspace in query, without default keyspace",
			query:                    "CREATE TABLE test_keyspace.test_table (column1 varchar, column10 int, PRIMARY KEY (column1, column10))",
			defaultIntRowKeyEncoding: types.OrderedCodeEncoding,
			want: &Want{
				Table:             "test_table",
				Keyspace:          "test_keyspace",
				IntRowKeyEncoding: types.OrderedCodeEncoding,
				IfNotExists:       false,
				Columns: []types.CreateColumn{
					{Name: "column1", Index: 0, TypeInfo: u.ParseCqlTypeOrDie("varchar")},
					{Name: "column10", Index: 1, TypeInfo: u.ParseCqlTypeOrDie("int")},
				},
				PrimaryKeys: []types.CreateTablePrimaryKeyConfig{
					{Name: "column1", KeyType: "partition_key"},
					{Name: "column10", KeyType: "clustering"},
				},
			},
			error:           "",
			defaultKeyspace: "test_keyspace",
		},
		{
			name:                     "with keyspace in query, with default keyspace",
			query:                    "CREATE TABLE test_keyspace.test_table (column1 varchar, column10 int, PRIMARY KEY (column1, column10))",
			defaultIntRowKeyEncoding: types.OrderedCodeEncoding,
			want: &Want{
				Table:             "test_table",
				Keyspace:          "test_keyspace",
				IntRowKeyEncoding: types.OrderedCodeEncoding,
				IfNotExists:       false,
				Columns: []types.CreateColumn{
					{Name: "column1", Index: 0, TypeInfo: u.ParseCqlTypeOrDie("varchar")},
					{Name: "column10", Index: 1, TypeInfo: u.ParseCqlTypeOrDie("int")},
				},
				PrimaryKeys: []types.CreateTablePrimaryKeyConfig{
					{Name: "column1", KeyType: "partition_key"},
					{Name: "column10", KeyType: "clustering"},
				},
			},
			error:           "",
			defaultKeyspace: "test_keyspace",
		},
		{
			name:                     "with ascii",
			query:                    "CREATE TABLE test_keyspace.test_table (column1 ascii, column10 int, test_col ascii, PRIMARY KEY (column1, column10))",
			defaultIntRowKeyEncoding: types.OrderedCodeEncoding,
			want: &Want{
				Table:             "test_table",
				Keyspace:          "test_keyspace",
				IntRowKeyEncoding: types.OrderedCodeEncoding,
				IfNotExists:       false,
				Columns: []types.CreateColumn{
					{Name: "column1", Index: 0, TypeInfo: types.TypeAscii},
					{Name: "column10", Index: 1, TypeInfo: types.TypeInt},
					{Name: "test_col", Index: 2, TypeInfo: types.TypeAscii},
				},
				PrimaryKeys: []types.CreateTablePrimaryKeyConfig{
					{Name: "column1", KeyType: "partition_key"},
					{Name: "column10", KeyType: "clustering"},
				},
			},
			error:           "",
			defaultKeyspace: "test_keyspace",
		},
		{
			name:                     "without keyspace in query, with default keyspace",
			query:                    "CREATE TABLE test_table (column1 varchar, column10 int, PRIMARY KEY (column1, column10))",
			defaultIntRowKeyEncoding: types.OrderedCodeEncoding,
			want: &Want{
				Table:             "test_table",
				Keyspace:          "my_keyspace",
				IntRowKeyEncoding: types.OrderedCodeEncoding,
				IfNotExists:       false,
				Columns: []types.CreateColumn{
					{Name: "column1", Index: 0, TypeInfo: u.ParseCqlTypeOrDie("varchar")},
					{Name: "column10", Index: 1, TypeInfo: u.ParseCqlTypeOrDie("int")},
				},
				PrimaryKeys: []types.CreateTablePrimaryKeyConfig{
					{Name: "column1", KeyType: "partition_key"},
					{Name: "column10", KeyType: "clustering"},
				},
			},
			error:           "",
			defaultKeyspace: "my_keyspace",
		},
		{
			name:  "with int_row_key_encoding option set to big_endian",
			query: "CREATE TABLE my_keyspace.test_table (column1 varchar, column10 int, PRIMARY KEY (column1, column10)) WITH int_row_key_encoding='big_endian'",
			// should be different from the int_row_key_encoding value
			defaultIntRowKeyEncoding: types.OrderedCodeEncoding,
			want: &Want{
				Table:             "test_table",
				Keyspace:          "my_keyspace",
				IntRowKeyEncoding: types.BigEndianEncoding,
				IfNotExists:       false,
				Columns: []types.CreateColumn{
					{Name: "column1", Index: 0, TypeInfo: u.ParseCqlTypeOrDie("varchar")},
					{Name: "column10", Index: 1, TypeInfo: u.ParseCqlTypeOrDie("int")},
				},
				PrimaryKeys: []types.CreateTablePrimaryKeyConfig{
					{Name: "column1", KeyType: "partition_key"},
					{Name: "column10", KeyType: "clustering"},
				},
			},
			error:           "",
			defaultKeyspace: "my_keyspace",
		},
		{
			name:  "with int_row_key_encoding option set to ordered_code",
			query: "CREATE TABLE my_keyspace.test_table (column1 varchar, column10 int, PRIMARY KEY (column1, column10)) WITH int_row_key_encoding='ordered_code'",
			// should be different from the int_row_key_encoding value
			defaultIntRowKeyEncoding: types.BigEndianEncoding,
			want: &Want{
				Table:             "test_table",
				Keyspace:          "my_keyspace",
				IntRowKeyEncoding: types.OrderedCodeEncoding,
				IfNotExists:       false,
				Columns: []types.CreateColumn{
					{Name: "column1", Index: 0, TypeInfo: u.ParseCqlTypeOrDie("varchar")},
					{Name: "column10", Index: 1, TypeInfo: u.ParseCqlTypeOrDie("int")},
				},
				PrimaryKeys: []types.CreateTablePrimaryKeyConfig{
					{Name: "column1", KeyType: "partition_key"},
					{Name: "column10", KeyType: "clustering"},
				},
			},
			error:           "",
			defaultKeyspace: "my_keyspace",
		},
		{
			name:  "with int_row_key_encoding option set to an unsupported value",
			query: "CREATE TABLE my_keyspace.test_table (column1 varchar, column10 int, PRIMARY KEY (column1, column10)) WITH int_row_key_encoding='pig_latin'",
			// should be different from the int_row_key_encoding value
			defaultIntRowKeyEncoding: types.BigEndianEncoding,
			want: &Want{
				Table:             "test_table",
				Keyspace:          "my_keyspace",
				IntRowKeyEncoding: types.OrderedCodeEncoding,
				IfNotExists:       false,
				Columns: []types.CreateColumn{
					{Name: "column1", Index: 0, TypeInfo: u.ParseCqlTypeOrDie("varchar")},
					{Name: "column10", Index: 1, TypeInfo: u.ParseCqlTypeOrDie("int")},
				},
				PrimaryKeys: []types.CreateTablePrimaryKeyConfig{
					{Name: "column1", KeyType: "partition_key"},
					{Name: "column10", KeyType: "clustering"},
				},
			},
			error:           "unsupported encoding 'pig_latin' for option 'int_row_key_encoding'",
			defaultKeyspace: "my_keyspace",
		},
		{
			name:            "without keyspace in query, without default keyspace (should error)",
			query:           "CREATE TABLE test_table (column1 varchar, column10 int, PRIMARY KEY (column1, column10))",
			want:            nil,
			error:           "no keyspace specified",
			defaultKeyspace: "",
		},
		{
			name:            "parser returns empty table (should error)",
			query:           "CREATE TABLE test_keyspace. (column1 varchar, column10 int, PRIMARY KEY (column1, column10))",
			want:            nil,
			error:           "parsing error",
			defaultKeyspace: "test_keyspace",
		},
		{
			name:            "parser returns error when primary key has no column definition",
			query:           "CREATE TABLE test_keyspace.table1 (column1 varchar, column10 int, PRIMARY KEY (column1, column99999))",
			want:            nil,
			error:           "primary key 'column99999' has no column definition in create table statement",
			defaultKeyspace: "test_keyspace",
		},
		{
			name:            "parser returns error when inline pmk conflicts",
			query:           "CREATE TABLE test_keyspace.table1 (column1 varchar PRIMARY KEY, column10 int, PRIMARY KEY (column1, column10))",
			want:            nil,
			error:           "cannot specify both primary key clause and inline primary key",
			defaultKeyspace: "test_keyspace",
		},
		{
			name:            "parser returns error when multiple inline primary keys",
			query:           "CREATE TABLE test_keyspace.table1 (column1 varchar PRIMARY KEY, column10 int PRIMARY KEY, column11 int)",
			want:            nil,
			error:           "multiple inline primary key columns not allowed",
			defaultKeyspace: "test_keyspace",
		},
		{
			name:            "parser returns error empty primary keys",
			query:           "CREATE TABLE test_keyspace.table1 (column1 varchar, column10 int, PRIMARY KEY ())",
			want:            nil,
			error:           "parsing error",
			defaultKeyspace: "test_keyspace",
		},
		{
			name:            "parser returns error when invalid key type is used inline",
			query:           "CREATE TABLE test_keyspace.table1 (column1 boolean PRIMARY KEY, column10 int, column11 int)",
			want:            nil,
			error:           "primary key cannot be of type boolean",
			defaultKeyspace: "test_keyspace",
		},
		{
			name:            "parser returns error when invalid key type is used in clause",
			query:           "CREATE TABLE test_keyspace.table1 (column1 boolean, column10 int, column11 int, PRIMARY KEY (column1))",
			want:            nil,
			error:           "primary key cannot be of type boolean",
			defaultKeyspace: "test_keyspace",
		},
		{
			name:            "parser returns error when invalid column type is used",
			query:           "CREATE TABLE test_keyspace.table1 (column1 int, column10 UUID, column11 int, PRIMARY KEY (column1))",
			want:            nil,
			error:           "column type 'uuid' is not supported",
			defaultKeyspace: "test_keyspace",
		},
		{
			name:            "parser returns error empty primary keys",
			query:           "CREATE TABLE test_keyspace.table1 ()",
			want:            nil,
			error:           "parsing error",
			defaultKeyspace: "test_keyspace",
		},
		{
			name:            "parser returns error empty primary keys",
			query:           "CREATE TABLE test_keyspace.table",
			want:            nil,
			error:           "parsing error",
			defaultKeyspace: "test_keyspace",
		},
		{
			name:            "parser returns error empty primary keys",
			query:           "CREATE TABLE test_keyspace.table",
			want:            nil,
			error:           "parsing error",
			defaultKeyspace: "test_keyspace",
		},
		{
			name:            "unsupported table options",
			query:           "CREATE TABLE test_keyspace.table1 (column1 varchar, column10 int, PRIMARY KEY (column1, column10)) WITH option_foo='bar'",
			want:            nil,
			error:           "unsupported table option: 'option_foo'",
			defaultKeyspace: "test_keyspace",
		},
		{
			name:            "same name as schema mapping table",
			query:           "CREATE TABLE test_keyspace.schema_mapping (column1 varchar, column10 int, PRIMARY KEY (column1, column10))",
			want:            nil,
			error:           "table name cannot be the same as the configured schema mapping table name 'schema_mapping'",
			defaultKeyspace: "test_keyspace",
		},
		{
			name:            "reserved table name",
			query:           "CREATE TABLE test_keyspace.table (column1 varchar, column10 int, PRIMARY KEY (column1, column10))",
			want:            nil,
			error:           "parsing error",
			defaultKeyspace: "test_keyspace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := NewCreateTranslator(mockdata.GetSchemaMappingConfig(), &types.BigtableConfig{
				SchemaMappingTable:       "schema_mapping",
				DefaultIntRowKeyEncoding: tt.defaultIntRowKeyEncoding,
			})
			query := types.NewRawQuery(nil, tt.defaultKeyspace, tt.query, parser.NewParser(tt.query), types.QueryTypeCreate)
			got, err := tr.Translate(query, tt.defaultKeyspace)
			if tt.error != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.error)
				assert.Nil(t, got)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, got)
			require.IsType(t, &types.CreateTableStatementMap{}, got)
			gotCreate := got.(*types.CreateTableStatementMap)
			assert.Equal(t, tt.want.Table, gotCreate.Table())
			assert.Equal(t, tt.want.Keyspace, gotCreate.Keyspace())
			assert.Equal(t, tt.want.IfNotExists, gotCreate.IfNotExists)
			assert.Equal(t, tt.want.Columns, gotCreate.Columns)
			assert.Equal(t, tt.want.PrimaryKeys, gotCreate.PrimaryKeys)
			assert.Equal(t, tt.want.IntRowKeyEncoding, gotCreate.IntRowKeyEncoding)
		})
	}
}
