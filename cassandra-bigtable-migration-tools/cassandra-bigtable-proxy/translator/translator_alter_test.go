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

func TestTranslateAlterTableToBigtable(t *testing.T) {

	userInfoTable := schemaMapping.NewTableConfig("test_keyspace", "user_info", "cf1", types.OrderedCodeEncoding, []*types.Column{
		{Name: "name", CQLType: datatype.Varchar, KeyType: utilities.KEY_TYPE_PARTITION, PkPrecedence: 1},
		{Name: "age", CQLType: datatype.Int, KeyType: utilities.KEY_TYPE_CLUSTERING, PkPrecedence: 2},
		{Name: "email", CQLType: datatype.Varchar, KeyType: utilities.KEY_TYPE_REGULAR, PkPrecedence: 0},
		{Name: "username", CQLType: datatype.Varchar, KeyType: utilities.KEY_TYPE_REGULAR, PkPrecedence: 0},
	})

	var tests = []struct {
		name            string
		query           string
		want            *AlterTableStatementMap
		tableConfig     *schemaMapping.TableConfig
		error           string
		defaultKeyspace string
	}{
		{
			name:        "Add column with explicit keyspace",
			query:       "ALTER TABLE test_keyspace.user_info ADD firstname text",
			tableConfig: userInfoTable,
			want: &AlterTableStatementMap{
				Table:     "user_info",
				Keyspace:  "test_keyspace",
				QueryType: "alter",
				AddColumns: []types.CreateColumn{{
					Name:  "firstname",
					Index: 0,
					Type:  datatype.Varchar,
				}},
			},
			error:           "",
			defaultKeyspace: "",
		},
		{
			name:        "Add column with default keyspace",
			query:       "ALTER TABLE user_info ADD firstname text",
			tableConfig: userInfoTable,
			want: &AlterTableStatementMap{
				Table:     "user_info",
				Keyspace:  "test_keyspace",
				QueryType: "alter",
				AddColumns: []types.CreateColumn{{
					Name:  "firstname",
					Index: 0,
					Type:  datatype.Varchar,
				}},
			},
			error:           "",
			defaultKeyspace: "test_keyspace",
		},
		{
			name:            "Add column without keyspace and no default keyspace (should error)",
			query:           "ALTER TABLE user_info ADD firstname text",
			tableConfig:     userInfoTable,
			want:            nil,
			error:           "missing keyspace. keyspace is required",
			defaultKeyspace: "",
		},
		{
			name:            "Add column with empty table name (should error)",
			query:           "ALTER TABLE . ADD firstname text",
			tableConfig:     userInfoTable,
			want:            nil,
			error:           "error while parsing alter statement",
			defaultKeyspace: "test_keyspace",
		},
		{
			name:        "Add multiple columns with explicit keyspace",
			query:       "ALTER TABLE test_keyspace.user_info ADD firstname text, number_of_cats int",
			tableConfig: userInfoTable,
			want: &AlterTableStatementMap{
				Table:     "user_info",
				Keyspace:  "test_keyspace",
				QueryType: "alter",
				AddColumns: []types.CreateColumn{{
					Name:  "firstname",
					Index: 0,
					Type:  datatype.Varchar,
				}, {
					Name:  "number_of_cats",
					Index: 1,
					Type:  datatype.Int,
				}},
			},
			error:           "",
			defaultKeyspace: "",
		},
		{
			name:        "Add multiple columns with default keyspace",
			query:       "ALTER TABLE user_info ADD firstname text, number_of_toes int",
			tableConfig: userInfoTable,
			want: &AlterTableStatementMap{
				Table:     "user_info",
				Keyspace:  "test_keyspace",
				QueryType: "alter",
				AddColumns: []types.CreateColumn{{
					Name:  "firstname",
					Index: 0,
					Type:  datatype.Varchar,
				}, {
					Name:  "number_of_toes",
					Index: 1,
					Type:  datatype.Int,
				}},
			},
			error:           "",
			defaultKeyspace: "test_keyspace",
		},
		{
			name:        "Drop column with explicit keyspace",
			query:       "ALTER TABLE test_keyspace.user_info DROP email",
			tableConfig: userInfoTable,
			want: &AlterTableStatementMap{
				Table:       "user_info",
				Keyspace:    "test_keyspace",
				QueryType:   "alter",
				DropColumns: []string{"email"},
			},
			error:           "",
			defaultKeyspace: "",
		},
		{
			name:        "Drop column with default keyspace",
			query:       "ALTER TABLE user_info DROP email",
			tableConfig: userInfoTable,
			want: &AlterTableStatementMap{
				Table:       "user_info",
				Keyspace:    "test_keyspace",
				QueryType:   "alter",
				DropColumns: []string{"email"},
			},
			error:           "",
			defaultKeyspace: "test_keyspace",
		},
		{
			name:            "Drop column without keyspace and no default keyspace (should error)",
			query:           "ALTER TABLE user_info DROP email",
			tableConfig:     userInfoTable,
			want:            nil,
			error:           "missing keyspace. keyspace is required",
			defaultKeyspace: "",
		},
		{
			name:        "Drop multiple columns with explicit keyspace",
			query:       "ALTER TABLE test_keyspace.user_info DROP email, username",
			tableConfig: userInfoTable,
			want: &AlterTableStatementMap{
				Table:       "user_info",
				Keyspace:    "test_keyspace",
				QueryType:   "alter",
				DropColumns: []string{"email", "username"},
			},
			error:           "",
			defaultKeyspace: "",
		},
		{
			name:        "Drop multiple columns with default keyspace",
			query:       "ALTER TABLE user_info DROP email, username",
			tableConfig: userInfoTable,
			want: &AlterTableStatementMap{
				Table:       "user_info",
				Keyspace:    "test_keyspace",
				QueryType:   "alter",
				DropColumns: []string{"email", "username"},
			},
			error:           "",
			defaultKeyspace: "test_keyspace",
		},
		{
			name:            "Drop multiple columns without keyspace and no default keyspace (should error)",
			query:           "ALTER TABLE user_info DROP firstname, lastname",
			tableConfig:     userInfoTable,
			want:            nil,
			error:           "missing keyspace. keyspace is required",
			defaultKeyspace: "",
		},
		{
			name:            "Rename column (not supported)",
			query:           "ALTER TABLE test_keyspace.user_info RENAME col1 TO col2",
			tableConfig:     userInfoTable,
			want:            nil,
			error:           "rename operation in alter table command not supported",
			defaultKeyspace: "",
		},
		{
			name:            "Add multiple columns but one already exists",
			query:           "ALTER TABLE test_keyspace.user_info ADD firstname text, age int",
			tableConfig:     userInfoTable,
			want:            nil,
			error:           "column 'age' already exists in table",
			defaultKeyspace: "",
		},
		{
			name:            "Drop primary key",
			query:           "ALTER TABLE test_keyspace.user_info DROP name",
			tableConfig:     userInfoTable,
			want:            nil,
			error:           "cannot drop primary key column: 'name'",
			defaultKeyspace: "",
		},
		{
			name:            "Alter type not supported",
			query:           "ALTER TABLE test_keyspace.user_info ALTER name int",
			tableConfig:     userInfoTable,
			want:            nil,
			error:           "alter column type operations are not supported",
			defaultKeyspace: "",
		},
		{
			name:            "Alter table properties not supported",
			query:           "ALTER TABLE test_keyspace.user_info WITH comment = 'bigtable was here'",
			tableConfig:     userInfoTable,
			want:            nil,
			error:           "table property operations are not supported",
			defaultKeyspace: "",
		},
		{
			name:            "add column with reserved keyword",
			query:           "ALTER TABLE test_keyspace.user_info add table varchar",
			tableConfig:     userInfoTable,
			want:            nil,
			error:           "cannot alter a table with reserved keyword as column name: 'table'",
			defaultKeyspace: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NotNil(t, tt.tableConfig, "tests must define a table config")
			smc := schemaMapping.NewSchemaMappingConfig("schema_mapping", "cf1", zap.NewNop(), []*schemaMapping.TableConfig{tt.tableConfig})
			tr := &Translator{
				Logger:              nil,
				SchemaMappingConfig: smc,
			}
			got, err := tr.TranslateAlterTableToBigtable(tt.query, tt.defaultKeyspace)
			if tt.error != "" {
				require.Error(t, err)
				assert.Equal(t, tt.error, err.Error())
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}
