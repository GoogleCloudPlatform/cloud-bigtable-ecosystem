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

	"github.com/stretchr/testify/assert"
)

func TestTranslateTruncateTableToBigtable(t *testing.T) {
	var tests = []struct {
		name            string
		query           string
		want            *TruncateTableStatementMap
		hasError        bool
		defaultKeyspace string
	}{
		{
			name:  "Truncate table with explicit keyspace",
			query: "TRUNCATE TABLE my_keyspace.my_table",
			want: &TruncateTableStatementMap{
				Table:     "my_table",
				Keyspace:  "my_keyspace",
				QueryType: "truncate",
			},
			hasError:        false,
			defaultKeyspace: "",
		},
		{
			name:  "Truncate table with explicit keyspace and default keyspace",
			query: "TRUNCATE TABLE my_keyspace.my_table;",
			want: &TruncateTableStatementMap{
				Table:     "my_table",
				Keyspace:  "my_keyspace",
				QueryType: "truncate",
			},
			hasError:        false,
			defaultKeyspace: "my_keyspace",
		},
		{
			name:  "Truncate table with default keyspace",
			query: "TRUNCATE TABLE my_table",
			want: &TruncateTableStatementMap{
				Table:     "my_table",
				Keyspace:  "my_keyspace",
				QueryType: "truncate",
			},
			hasError:        false,
			defaultKeyspace: "my_keyspace",
		},
		{
			name:            "Truncate table without keyspace and no default keyspace",
			query:           "TRUNCATE TABLE my_table",
			want:            nil,
			hasError:        true,
			defaultKeyspace: "",
		},
		{
			name:  "Truncate table without semicolon",
			query: "TRUNCATE TABLE my_keyspace.my_table",
			want: &TruncateTableStatementMap{
				Table:     "my_table",
				Keyspace:  "my_keyspace",
				QueryType: "truncate",
			},
			hasError:        false,
			defaultKeyspace: "my_keyspace",
		},
		{
			name:            "Truncate table with empty table name",
			query:           "TRUNCATE TABLE my_keyspace.;",
			want:            nil,
			hasError:        true,
			defaultKeyspace: "my_keyspace",
		},
		{
			name:            "Truncate table with empty keyspace(Should return error as query syntax is not valid)",
			query:           "TRUNCATE TABLE .my_table",
			want:            nil,
			hasError:        true,
			defaultKeyspace: "",
		},
		{
			name:  "no 'table' keyword because it's optional",
			query: "TRUNCATE my_keyspace.my_table",
			want: &TruncateTableStatementMap{
				Table:     "my_table",
				Keyspace:  "my_keyspace",
				QueryType: "truncate",
			},
			hasError:        false,
			defaultKeyspace: "my_keyspace",
		},
	}

	tr := &Translator{
		Logger:              nil,
		SchemaMappingConfig: nil,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tr.TranslateTruncateTableToBigtable(tt.query, tt.defaultKeyspace)
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
