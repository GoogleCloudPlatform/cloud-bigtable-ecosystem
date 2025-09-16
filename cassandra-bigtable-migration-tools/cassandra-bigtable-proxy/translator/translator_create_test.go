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
	"testing"

	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	schemaMapping "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/schema-mapping"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/utilities"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestTranslateCreateTableToBigtable(t *testing.T) {
	tests := []struct {
		name                     string
		query                    string
		defaultIntRowKeyEncoding types.IntRowKeyEncodingType
		want                     *CreateTableStatementMap
		error                    string
		defaultKeyspace          string
	}{
		{
			name:                     "success",
			query:                    "CREATE TABLE my_keyspace.my_table (user_id varchar, order_num int, name varchar, PRIMARY KEY (user_id, order_num))",
			defaultIntRowKeyEncoding: types.OrderedCodeEncoding,
			want: &CreateTableStatementMap{
				Table:             "my_table",
				Keyspace:          "my_keyspace",
				QueryType:         "create",
				IfNotExists:       false,
				IntRowKeyEncoding: types.OrderedCodeEncoding,
				Columns: []types.CreateColumn{
					{
						Name:  "user_id",
						Index: 0,
						Type:  datatype.Varchar,
					},
					{
						Name:  "order_num",
						Index: 1,
						Type:  datatype.Int,
					},
					{
						Name:  "name",
						Index: 2,
						Type:  datatype.Varchar,
					},
				},
				PrimaryKeys: []CreateTablePrimaryKeyConfig{
					{
						Name:    "user_id",
						KeyType: "partition",
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
			want: &CreateTableStatementMap{
				Table:             "my_table",
				Keyspace:          "my_keyspace",
				QueryType:         "create",
				IntRowKeyEncoding: types.OrderedCodeEncoding,
				IfNotExists:       true,
				Columns: []types.CreateColumn{
					{
						Name:  "user_id",
						Index: 0,
						Type:  datatype.Varchar,
					},
					{
						Name:  "order_num",
						Index: 1,
						Type:  datatype.Int,
					},
					{
						Name:  "name",
						Index: 2,
						Type:  datatype.Varchar,
					},
				},
				PrimaryKeys: []CreateTablePrimaryKeyConfig{
					{
						Name:    "user_id",
						KeyType: "partition",
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
			want: &CreateTableStatementMap{
				Table:             "cyclist_name",
				Keyspace:          "cycling",
				QueryType:         "create",
				IntRowKeyEncoding: types.OrderedCodeEncoding,
				IfNotExists:       false,
				Columns: []types.CreateColumn{
					{
						Name:  "id",
						Index: 0,
						Type:  datatype.Varchar,
					},
					{
						Name:  "lastname",
						Index: 1,
						Type:  datatype.Varchar,
					},
					{
						Name:  "firstname",
						Index: 2,
						Type:  datatype.Varchar,
					},
				},
				PrimaryKeys: []CreateTablePrimaryKeyConfig{
					{
						Name:    "id",
						KeyType: utilities.KEY_TYPE_PARTITION,
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
			want: &CreateTableStatementMap{
				Table:             "cyclist_composite",
				Keyspace:          "cycling",
				QueryType:         "create",
				IntRowKeyEncoding: types.OrderedCodeEncoding,
				IfNotExists:       false,
				Columns: []types.CreateColumn{
					{
						Name:  "id",
						Index: 0,
						Type:  datatype.Varchar,
					},
					{
						Name:  "lastname",
						Index: 1,
						Type:  datatype.Varchar,
					},
					{
						Name:  "firstname",
						Index: 2,
						Type:  datatype.Varchar,
					},
				},
				PrimaryKeys: []CreateTablePrimaryKeyConfig{
					{
						Name:    "id",
						KeyType: "partition",
					},
					{
						Name:    "lastname",
						KeyType: "partition",
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
			want: &CreateTableStatementMap{
				Table:             "test_table",
				Keyspace:          "test_keyspace",
				QueryType:         "create",
				IntRowKeyEncoding: types.OrderedCodeEncoding,
				IfNotExists:       false,
				Columns: []types.CreateColumn{
					{Name: "column1", Index: 0, Type: datatype.Varchar},
					{Name: "column10", Index: 1, Type: datatype.Int},
				},
				PrimaryKeys: []CreateTablePrimaryKeyConfig{
					{Name: "column1", KeyType: "partition"},
					{Name: "column10", KeyType: "clustering"},
				},
			},
			error:           "",
			defaultKeyspace: "test_keyspace",
		},
		{
			name:                     "with a frozen column",
			query:                    "CREATE TABLE test_keyspace.test_table (column1 varchar, column10 int, frozen_col frozen<list<int>>, PRIMARY KEY (column1, column10))",
			defaultIntRowKeyEncoding: types.OrderedCodeEncoding,
			want: &CreateTableStatementMap{
				Table:             "test_table",
				Keyspace:          "test_keyspace",
				QueryType:         "create",
				IntRowKeyEncoding: types.OrderedCodeEncoding,
				IfNotExists:       false,
				Columns: []types.CreateColumn{
					{Name: "column1", Index: 0, Type: datatype.Varchar},
					{Name: "column10", Index: 1, Type: datatype.Int},
					{Name: "frozen_col", Index: 2, Type: datatype.NewListType(datatype.Int), IsFrozen: true},
				},
				PrimaryKeys: []CreateTablePrimaryKeyConfig{
					{Name: "column1", KeyType: "partition"},
					{Name: "column10", KeyType: "clustering"},
				},
			},
			error:           "",
			defaultKeyspace: "test_keyspace",
		},
		{
			name:                     "with a frozen column",
			query:                    "CREATE TABLE test_keyspace.test_table (column1 varchar, column10 int, frozen_col frozen<list<int>>, frozen_map frozen<map<text, text>>, PRIMARY KEY (column1, column10))",
			defaultIntRowKeyEncoding: types.OrderedCodeEncoding,
			want: &CreateTableStatementMap{
				Table:             "test_table",
				Keyspace:          "test_keyspace",
				QueryType:         "create",
				IntRowKeyEncoding: types.OrderedCodeEncoding,
				IfNotExists:       false,
				Columns: []types.CreateColumn{
					{Name: "column1", Index: 0, Type: datatype.Varchar},
					{Name: "column10", Index: 1, Type: datatype.Int},
					{Name: "frozen_col", Index: 2, Type: datatype.NewListType(datatype.Int), IsFrozen: true},
				},
				PrimaryKeys: []CreateTablePrimaryKeyConfig{
					{Name: "column1", KeyType: "partition"},
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
			want: &CreateTableStatementMap{
				Table:             "test_table",
				Keyspace:          "test_keyspace",
				QueryType:         "create",
				IntRowKeyEncoding: types.OrderedCodeEncoding,
				IfNotExists:       false,
				Columns: []types.CreateColumn{
					{Name: "column1", Index: 0, Type: datatype.Varchar},
					{Name: "column10", Index: 1, Type: datatype.Int},
				},
				PrimaryKeys: []CreateTablePrimaryKeyConfig{
					{Name: "column1", KeyType: "partition"},
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
			want: &CreateTableStatementMap{
				Table:             "test_table",
				Keyspace:          "my_keyspace",
				QueryType:         "create",
				IntRowKeyEncoding: types.OrderedCodeEncoding,
				IfNotExists:       false,
				Columns: []types.CreateColumn{
					{Name: "column1", Index: 0, Type: datatype.Varchar},
					{Name: "column10", Index: 1, Type: datatype.Int},
				},
				PrimaryKeys: []CreateTablePrimaryKeyConfig{
					{Name: "column1", KeyType: "partition"},
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
			want: &CreateTableStatementMap{
				Table:             "test_table",
				Keyspace:          "my_keyspace",
				QueryType:         "create",
				IntRowKeyEncoding: types.BigEndianEncoding,
				IfNotExists:       false,
				Columns: []types.CreateColumn{
					{Name: "column1", Index: 0, Type: datatype.Varchar},
					{Name: "column10", Index: 1, Type: datatype.Int},
				},
				PrimaryKeys: []CreateTablePrimaryKeyConfig{
					{Name: "column1", KeyType: "partition"},
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
			want: &CreateTableStatementMap{
				Table:             "test_table",
				Keyspace:          "my_keyspace",
				QueryType:         "create",
				IntRowKeyEncoding: types.OrderedCodeEncoding,
				IfNotExists:       false,
				Columns: []types.CreateColumn{
					{Name: "column1", Index: 0, Type: datatype.Varchar},
					{Name: "column10", Index: 1, Type: datatype.Int},
				},
				PrimaryKeys: []CreateTablePrimaryKeyConfig{
					{Name: "column1", KeyType: "partition"},
					{Name: "column10", KeyType: "clustering"},
				},
			},
			error:           "",
			defaultKeyspace: "my_keyspace",
		},
		{
			name:  "with int_row_key_encoding option set to an unsupported value",
			query: "CREATE TABLE my_keyspace.test_table (column1 varchar, column10 int, PRIMARY KEY (column1, column10)) WITH int_row_key_encoding='pig_latin'",
			// should be different than the int_row_key_encoding value
			defaultIntRowKeyEncoding: types.BigEndianEncoding,
			want: &CreateTableStatementMap{
				Table:             "test_table",
				Keyspace:          "my_keyspace",
				QueryType:         "create",
				IntRowKeyEncoding: types.OrderedCodeEncoding,
				IfNotExists:       false,
				Columns: []types.CreateColumn{
					{Name: "column1", Index: 0, Type: datatype.Varchar},
					{Name: "column10", Index: 1, Type: datatype.Int},
				},
				PrimaryKeys: []CreateTablePrimaryKeyConfig{
					{Name: "column1", KeyType: "partition"},
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
			error:           "missing keyspace. keyspace is required",
			defaultKeyspace: "",
		},
		{
			name:            "parser returns empty table (should error)",
			query:           "CREATE TABLE test_keyspace. (column1 varchar, column10 int, PRIMARY KEY (column1, column10))",
			want:            nil,
			error:           "invalid table name parsed from query",
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
			name:            "parser returns error when multiple inline pmks",
			query:           "CREATE TABLE test_keyspace.table1 (column1 varchar PRIMARY KEY, column10 int PRIMARY KEY, column11 int)",
			want:            nil,
			error:           "multiple inline primary key columns not allowed",
			defaultKeyspace: "test_keyspace",
		},
		{
			name:            "parser returns error empty pmks",
			query:           "CREATE TABLE test_keyspace.table1 (column1 varchar, column10 int, PRIMARY KEY ())",
			want:            nil,
			error:           "no primary key found in create table statement",
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
			name:            "parser returns error empty pmks",
			query:           "CREATE TABLE test_keyspace.table1 ()",
			want:            nil,
			error:           "malformed create table statement",
			defaultKeyspace: "test_keyspace",
		},
		{
			name:            "parser returns error empty pmks",
			query:           "CREATE TABLE test_keyspace.table",
			want:            nil,
			error:           "malformed create table statement",
			defaultKeyspace: "test_keyspace",
		},
		{
			name:            "parser returns error empty pmks",
			query:           "CREATE TABLE test_keyspace.table",
			want:            nil,
			error:           "malformed create table statement",
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
			error:           "cannot create a table with the configured schema mapping table name 'schema_mapping'",
			defaultKeyspace: "test_keyspace",
		},
		{
			name:            "reserved table name",
			query:           "CREATE TABLE test_keyspace.table (column1 varchar, column10 int, PRIMARY KEY (column1, column10))",
			want:            nil,
			error:           "cannot create a table with reserved keyword as name: 'table'",
			defaultKeyspace: "test_keyspace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &Translator{
				Logger:                   nil,
				SchemaMappingConfig:      schemaMapping.NewSchemaMappingConfig("schema_mapping", "cf1", zap.NewNop(), nil),
				DefaultIntRowKeyEncoding: tt.defaultIntRowKeyEncoding,
			}
			got, err := tr.TranslateCreateTableToBigtable(tt.query, tt.defaultKeyspace)
			if tt.error != "" {
				require.Error(t, err)
				assert.Equal(t, tt.error, err.Error())
				assert.Nil(t, got)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
