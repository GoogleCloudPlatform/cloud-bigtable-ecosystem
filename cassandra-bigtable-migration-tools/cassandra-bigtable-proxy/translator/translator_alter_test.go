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

func TestTranslateAlterTableToBigtable(t *testing.T) {
	var tests = []struct {
		name            string
		query           string
		want            *AlterTableStatementMap
		hasError        bool
		defaultKeyspace string
	}{
		// Add column with explicit keyspace
		{
			name:  "Add column with explicit keyspace",
			query: "ALTER TABLE my_keyspace.my_table ADD firstname text",
			want: &AlterTableStatementMap{
				Table:     "my_table",
				Keyspace:  "my_keyspace",
				QueryType: "alter",
				AddColumns: []message.ColumnMetadata{{
					Keyspace: "my_keyspace",
					Table:    "my_table",
					Name:     "firstname",
					Index:    0,
					Type:     datatype.Varchar,
				}},
			},
			hasError:        false,
			defaultKeyspace: "",
		},
		// Add column with default keyspace
		{
			name:  "Add column with default keyspace",
			query: "ALTER TABLE my_table ADD firstname text",
			want: &AlterTableStatementMap{
				Table:     "my_table",
				Keyspace:  "test_keyspace",
				QueryType: "alter",
				AddColumns: []message.ColumnMetadata{{
					Keyspace: "test_keyspace",
					Table:    "my_table",
					Name:     "firstname",
					Index:    0,
					Type:     datatype.Varchar,
				}},
			},
			hasError:        false,
			defaultKeyspace: "test_keyspace",
		},
		// Add column without keyspace and no default keyspace
		{
			name:            "Add column without keyspace and no default keyspace (should error)",
			query:           "ALTER TABLE my_table ADD firstname text",
			want:            nil,
			hasError:        true,
			defaultKeyspace: "",
		},
		// Add column with empty table name
		{
			name:            "Add column with empty table name (should error)",
			query:           "ALTER TABLE . ADD firstname text",
			want:            nil,
			hasError:        true,
			defaultKeyspace: "test_keyspace",
		},
		// Add multiple columns with explicit keyspace
		{
			name:  "Add multiple columns with explicit keyspace",
			query: "ALTER TABLE my_keyspace.my_table ADD firstname text, age int",
			want: &AlterTableStatementMap{
				Table:     "my_table",
				Keyspace:  "my_keyspace",
				QueryType: "alter",
				AddColumns: []message.ColumnMetadata{{
					Keyspace: "my_keyspace",
					Table:    "my_table",
					Name:     "firstname",
					Index:    0,
					Type:     datatype.Varchar,
				}, {
					Keyspace: "my_keyspace",
					Table:    "my_table",
					Name:     "age",
					Index:    1,
					Type:     datatype.Int,
				}},
			},
			hasError:        false,
			defaultKeyspace: "",
		},
		// Add multiple columns with default keyspace
		{
			name:  "Add multiple columns with default keyspace",
			query: "ALTER TABLE my_table ADD firstname text, age int",
			want: &AlterTableStatementMap{
				Table:     "my_table",
				Keyspace:  "test_keyspace",
				QueryType: "alter",
				AddColumns: []message.ColumnMetadata{{
					Keyspace: "test_keyspace",
					Table:    "my_table",
					Name:     "firstname",
					Index:    0,
					Type:     datatype.Varchar,
				}, {
					Keyspace: "test_keyspace",
					Table:    "my_table",
					Name:     "age",
					Index:    1,
					Type:     datatype.Int,
				}},
			},
			hasError:        false,
			defaultKeyspace: "test_keyspace",
		},
		// Drop column with explicit keyspace
		{
			name:  "Drop column with explicit keyspace",
			query: "ALTER TABLE my_keyspace.my_table DROP firstname",
			want: &AlterTableStatementMap{
				Table:       "my_table",
				Keyspace:    "my_keyspace",
				QueryType:   "alter",
				DropColumns: []string{"firstname"},
			},
			hasError:        false,
			defaultKeyspace: "",
		},
		// Drop column with default keyspace
		{
			name:  "Drop column with default keyspace",
			query: "ALTER TABLE my_table DROP firstname",
			want: &AlterTableStatementMap{
				Table:       "my_table",
				Keyspace:    "test_keyspace",
				QueryType:   "alter",
				DropColumns: []string{"firstname"},
			},
			hasError:        false,
			defaultKeyspace: "test_keyspace",
		},
		// Drop column without keyspace and no default keyspace
		{
			name:            "Drop column without keyspace and no default keyspace (should error)",
			query:           "ALTER TABLE my_table DROP firstname",
			want:            nil,
			hasError:        true,
			defaultKeyspace: "",
		},
		// Drop multiple columns with explicit keyspace
		{
			name:  "Drop multiple columns with explicit keyspace",
			query: "ALTER TABLE my_keyspace.my_table DROP firstname, lastname",
			want: &AlterTableStatementMap{
				Table:       "my_table",
				Keyspace:    "my_keyspace",
				QueryType:   "alter",
				DropColumns: []string{"firstname", "lastname"},
			},
			hasError:        false,
			defaultKeyspace: "",
		},
		// Drop multiple columns with default keyspace
		{
			name:  "Drop multiple columns with default keyspace",
			query: "ALTER TABLE my_table DROP firstname, lastname",
			want: &AlterTableStatementMap{
				Table:       "my_table",
				Keyspace:    "test_keyspace",
				QueryType:   "alter",
				DropColumns: []string{"firstname", "lastname"},
			},
			hasError:        false,
			defaultKeyspace: "test_keyspace",
		},
		// Drop multiple columns without keyspace and no default keyspace
		{
			name:            "Drop multiple columns without keyspace and no default keyspace (should error)",
			query:           "ALTER TABLE my_table DROP firstname, lastname",
			want:            nil,
			hasError:        true,
			defaultKeyspace: "",
		},
		// Rename column (not supported)
		{
			name:            "Rename column (not supported)",
			query:           "ALTER TABLE my_keyspace.my_table RENAME col1 TO col2",
			want:            nil,
			hasError:        true,
			defaultKeyspace: "",
		},
	}

	tr := &Translator{
		Logger:              nil,
		SchemaMappingConfig: nil,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tr.TranslateAlterTableToBigtable(tt.query, tt.defaultKeyspace)
			if tt.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.True(t, (err != nil) == tt.hasError, tt.hasError)
			assert.Equal(t, tt.want, got)
		})
	}
}
