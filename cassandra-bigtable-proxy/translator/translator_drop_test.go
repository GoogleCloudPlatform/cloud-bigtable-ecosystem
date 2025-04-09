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

func TestTranslateDropTableToBigtable(t *testing.T) {
	var tests = []struct {
		query    string
		want     *DropTableStatementMap
		hasError bool
	}{
		{
			query: "DROP TABLE my_keyspace.my_table;",
			want: &DropTableStatementMap{
				Table:     "my_table",
				Keyspace:  "my_keyspace",
				QueryType: "drop",
				IfExists:  false,
			},
			hasError: false,
		},
		{
			query: "DROP TABLE IF EXISTS my_keyspace.my_table;",
			want: &DropTableStatementMap{
				Table:     "my_table",
				Keyspace:  "my_keyspace",
				QueryType: "drop",
				IfExists:  true,
			},
			hasError: false,
		},
	}

	tr := &Translator{
		Logger:              nil,
		SchemaMappingConfig: nil,
	}

	for _, tt := range tests {
		got, err := tr.TranslateDropTableToBigtable(tt.query)
		if tt.hasError {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
		}
		assert.True(t, (err != nil) == tt.hasError, tt.hasError)
		assert.Equal(t, tt.want, got)
	}
}
