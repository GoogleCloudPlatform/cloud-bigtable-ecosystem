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

	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/stretchr/testify/assert"
)

func TestTranslateCreateTableToBigtable(t *testing.T) {
	tests := []struct {
		name            string
		query           string
		want            *CreateTableStatementMap
		hasError        bool
		defaultKeyspace string
	}{
		{
			name:  "success",
			query: "CREATE TABLE my_keyspace.my_table (user_id varchar, order_num int, name varchar, PRIMARY KEY (user_id, order_num))",
			want: &CreateTableStatementMap{
				Table:       "my_table",
				Keyspace:    "my_keyspace",
				QueryType:   "create",
				IfNotExists: false,
				Columns: []message.ColumnMetadata{
					{
						Keyspace: "my_keyspace",
						Table:    "my_table",
						Name:     "user_id",
						Index:    0,
						Type:     datatype.Varchar,
					},
					{
						Keyspace: "my_keyspace",
						Table:    "my_table",
						Name:     "order_num",
						Index:    1,
						Type:     datatype.Int,
					},
					{
						Keyspace: "my_keyspace",
						Table:    "my_table",
						Name:     "name",
						Index:    2,
						Type:     datatype.Varchar,
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
			hasError:        false,
			defaultKeyspace: "my_keyspace",
		},
		{
			name:  "if not exists",
			query: "CREATE TABLE IF NOT EXISTS my_keyspace.my_table (user_id varchar, order_num int, name varchar, PRIMARY KEY (user_id))",
			want: &CreateTableStatementMap{
				Table:       "my_table",
				Keyspace:    "my_keyspace",
				QueryType:   "create",
				IfNotExists: true,
				Columns: []message.ColumnMetadata{
					{
						Keyspace: "my_keyspace",
						Table:    "my_table",
						Name:     "user_id",
						Index:    0,
						Type:     datatype.Varchar,
					},
					{
						Keyspace: "my_keyspace",
						Table:    "my_table",
						Name:     "order_num",
						Index:    1,
						Type:     datatype.Int,
					},
					{
						Keyspace: "my_keyspace",
						Table:    "my_table",
						Name:     "name",
						Index:    2,
						Type:     datatype.Varchar,
					},
				},
				PrimaryKeys: []CreateTablePrimaryKeyConfig{
					{
						Name:    "user_id",
						KeyType: "partition",
					},
				},
			},
			hasError:        false,
			defaultKeyspace: "my_keyspace",
		},
		{
			name:  "single inline primary key",
			query: "CREATE TABLE cycling.cyclist_name (id UUID PRIMARY KEY, lastname varchar, firstname varchar);",
			want: &CreateTableStatementMap{
				Table:       "cyclist_name",
				Keyspace:    "cycling",
				QueryType:   "create",
				IfNotExists: false,
				Columns: []message.ColumnMetadata{
					{
						Keyspace: "cycling",
						Table:    "cyclist_name",
						Name:     "id",
						Index:    0,
						Type:     datatype.Uuid,
					},
					{
						Keyspace: "cycling",
						Table:    "cyclist_name",
						Name:     "lastname",
						Index:    1,
						Type:     datatype.Varchar,
					},
					{
						Keyspace: "cycling",
						Table:    "cyclist_name",
						Name:     "firstname",
						Index:    2,
						Type:     datatype.Varchar,
					},
				},
				PrimaryKeys: []CreateTablePrimaryKeyConfig{
					{
						Name:    "id",
						KeyType: "regular",
					},
				},
			},
			hasError:        false,
			defaultKeyspace: "cycling",
		},
		{
			name:  "composite primary key",
			query: "CREATE TABLE cycling.cyclist_composite (id text, lastname varchar, firstname varchar, PRIMARY KEY ((id, lastname), firstname)));",
			want: &CreateTableStatementMap{
				Table:       "cyclist_composite",
				Keyspace:    "cycling",
				QueryType:   "create",
				IfNotExists: false,
				Columns: []message.ColumnMetadata{
					{
						Keyspace: "cycling",
						Table:    "cyclist_composite",
						Name:     "id",
						Index:    0,
						Type:     datatype.Varchar,
					},
					{
						Keyspace: "cycling",
						Table:    "cyclist_composite",
						Name:     "lastname",
						Index:    1,
						Type:     datatype.Varchar,
					},
					{
						Keyspace: "cycling",
						Table:    "cyclist_composite",
						Name:     "firstname",
						Index:    2,
						Type:     datatype.Varchar,
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
			hasError:        false,
			defaultKeyspace: "cycling",
		},
		{
			name:  "with keyspace in query, without default keyspace",
			query: "CREATE TABLE test_keyspace.test_table (column1 varchar, column10 int, PRIMARY KEY (column1, column10))",
			want: &CreateTableStatementMap{
				Table:       "test_table",
				Keyspace:    "test_keyspace",
				QueryType:   "create",
				IfNotExists: false,
				Columns: []message.ColumnMetadata{
					{Keyspace: "test_keyspace", Table: "test_table", Name: "column1", Index: 0, Type: datatype.Varchar},
					{Keyspace: "test_keyspace", Table: "test_table", Name: "column10", Index: 1, Type: datatype.Int},
				},
				PrimaryKeys: []CreateTablePrimaryKeyConfig{
					{Name: "column1", KeyType: "partition"},
					{Name: "column10", KeyType: "clustering"},
				},
			},
			hasError:        false,
			defaultKeyspace: "test_keyspace",
		},
		{
			name:  "with keyspace in query, with default keyspace",
			query: "CREATE TABLE test_keyspace.test_table (column1 varchar, column10 int, PRIMARY KEY (column1, column10))",
			want: &CreateTableStatementMap{
				Table:       "test_table",
				Keyspace:    "test_keyspace",
				QueryType:   "create",
				IfNotExists: false,
				Columns: []message.ColumnMetadata{
					{Keyspace: "test_keyspace", Table: "test_table", Name: "column1", Index: 0, Type: datatype.Varchar},
					{Keyspace: "test_keyspace", Table: "test_table", Name: "column10", Index: 1, Type: datatype.Int},
				},
				PrimaryKeys: []CreateTablePrimaryKeyConfig{
					{Name: "column1", KeyType: "partition"},
					{Name: "column10", KeyType: "clustering"},
				},
			},
			hasError:        false,
			defaultKeyspace: "test_keyspace",
		},
		{
			name:  "without keyspace in query, with default keyspace",
			query: "CREATE TABLE test_table (column1 varchar, column10 int, PRIMARY KEY (column1, column10))",
			want: &CreateTableStatementMap{
				Table:       "test_table",
				Keyspace:    "my_keyspace",
				QueryType:   "create",
				IfNotExists: false,
				Columns: []message.ColumnMetadata{
					{Keyspace: "my_keyspace", Table: "test_table", Name: "column1", Index: 0, Type: datatype.Varchar},
					{Keyspace: "my_keyspace", Table: "test_table", Name: "column10", Index: 1, Type: datatype.Int},
				},
				PrimaryKeys: []CreateTablePrimaryKeyConfig{
					{Name: "column1", KeyType: "partition"},
					{Name: "column10", KeyType: "clustering"},
				},
			},
			hasError:        false,
			defaultKeyspace: "my_keyspace",
		},
		{
			name:            "without keyspace in query, without default keyspace (should error)",
			query:           "CREATE TABLE test_table (column1 varchar, column10 int, PRIMARY KEY (column1, column10))",
			want:            nil,
			hasError:        true,
			defaultKeyspace: "",
		},
		{
			name:            "parser returns empty table (should error)",
			query:           "CREATE TABLE test_keyspace. (column1 varchar, column10 int, PRIMARY KEY (column1, column10))",
			want:            nil,
			hasError:        true,
			defaultKeyspace: "test_keyspace",
		},
	}

	tr := &Translator{
		Logger:              nil,
		SchemaMappingConfig: nil,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tr.TranslateCreateTableToBigtable(tt.query, tt.defaultKeyspace)
			if tt.hasError {
				assert.Error(t, err)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
