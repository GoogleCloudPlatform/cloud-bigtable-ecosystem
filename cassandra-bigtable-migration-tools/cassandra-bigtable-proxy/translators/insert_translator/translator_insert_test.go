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

package insert_translator

import (
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/parser"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/testing/mockdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

type Want struct {
	Keyspace       types.Keyspace
	Table          types.TableName
	IfNotExists    bool
	Assignments    []types.Assignment
	UsingTimestamp types.DynamicValue
	AllParams      []*types.ParameterMetadata
}

func TestSelectTranslator_Translate(t *testing.T) {
	tests := []struct {
		name            string
		query           string
		sessionKeyspace types.Keyspace
		want            *Want
		wantErr         string
	}{
		{
			name:  "success with prepared query",
			query: "INSERT INTO test_keyspace.test_table (pk1, pk2) VALUES (?, ?)",
			want: &Want{
				Keyspace:    "test_keyspace",
				Table:       "test_table",
				IfNotExists: false,
				Assignments: []types.Assignment{
					types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk1"), types.NewParameterizedValue("value0")),
					types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk2"), types.NewParameterizedValue("value1")),
				},
				AllParams: []*types.ParameterMetadata{
					{
						Key:    "value0",
						Order:  0,
						Type:   types.TypeVarchar,
						Column: mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk1"),
					},
					{
						Key:    "value1",
						Order:  1,
						Type:   types.TypeVarchar,
						Column: mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk2"),
					},
				},
			},
		},
		{
			name:  "success with prepared query using named markers",
			query: "INSERT INTO test_keyspace.test_table (pk1, pk2) VALUES (:v_1, :v_2)",
			want: &Want{
				Keyspace:    "test_keyspace",
				Table:       "test_table",
				IfNotExists: false,
				Assignments: []types.Assignment{
					types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk1"), types.NewParameterizedValue("v_1")),
					types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk2"), types.NewParameterizedValue("v_2")),
				},
				AllParams: []*types.ParameterMetadata{
					{
						Key:     "v_1",
						Order:   0,
						Type:    types.TypeVarchar,
						IsNamed: true,
					},
					{
						Key:     "v_2",
						Order:   1,
						Type:    types.TypeVarchar,
						IsNamed: true,
					},
				},
			},
		},
		{
			name:  "duplicate named markers",
			query: "INSERT INTO test_keyspace.test_table (pk1, pk2, col_int) VALUES (:pk, :pk, :i)",
			want: &Want{
				Keyspace:    "test_keyspace",
				Table:       "test_table",
				IfNotExists: false,
				Assignments: []types.Assignment{
					types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk1"), types.NewParameterizedValue("pk")),
					types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk2"), types.NewParameterizedValue("pk")),
					types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_int"), types.NewParameterizedValue("i")),
				},
				AllParams: []*types.ParameterMetadata{
					{
						Key:     "pk",
						Order:   0,
						Type:    types.TypeVarchar,
						IsNamed: true,
					},
					{
						Key:     "i",
						Order:   1,
						Type:    types.TypeInt,
						IsNamed: true,
					},
				},
			},
		},
		{
			name:  "success with prepared query and USING TIMESTAMP",
			query: "INSERT INTO test_keyspace.test_table (pk2, col_blob, col_bool, list_text, col_int, col_bigint, pk1) VALUES (?, ?, ?, ?, ?, ?, ?) USING TIMESTAMP ?",
			want: &Want{
				Keyspace:    "test_keyspace",
				Table:       "test_table",
				IfNotExists: false,
				Assignments: []types.Assignment{
					types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk2"), types.NewParameterizedValue("value0")),
					types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_blob"), types.NewParameterizedValue("value1")),
					types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_bool"), types.NewParameterizedValue("value2")),
					types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "list_text"), types.NewParameterizedValue("value3")),
					types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_int"), types.NewParameterizedValue("value4")),
					types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_bigint"), types.NewParameterizedValue("value5")),
					types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk1"), types.NewParameterizedValue("value6")),
				},
				UsingTimestamp: types.NewParameterizedValue("value7"),
				AllParams: []*types.ParameterMetadata{
					{
						Key:    "value0",
						Order:  0,
						Type:   types.TypeVarchar,
						Column: mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk2"),
					},
					{
						Key:    "value1",
						Order:  1,
						Type:   types.TypeBlob,
						Column: mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_blob"),
					},
					{
						Key:    "value2",
						Order:  2,
						Type:   types.TypeBoolean,
						Column: mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_bool"),
					},
					{
						Key:    "value3",
						Order:  3,
						Type:   types.NewListType(types.TypeText),
						Column: mockdata.GetColumnOrDie("test_keyspace", "test_table", "list_text"),
					},
					{
						Key:    "value4",
						Order:  4,
						Type:   types.TypeInt,
						Column: mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_int"),
					},
					{
						Key:    "value5",
						Order:  5,
						Type:   types.TypeBigInt,
						Column: mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_bigint"),
					},
					{
						Key:    "value6",
						Order:  6,
						Type:   types.TypeVarchar,
						Column: mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk1"),
					},
					{
						Key:    "value7",
						Order:  7,
						Type:   types.TypeBigInt,
						Column: nil,
					},
				},
			},
		},
		{
			name:  "success with adhoc query and USING TIMESTAMP",
			query: "INSERT INTO test_keyspace.test_table (pk2, col_blob, col_bool, list_text, col_int, col_bigint, pk1) VALUES ('u123', 0x0003, true, ['item1', 'item2', 'item1'], 3, 8242842848, 'org1') USING TIMESTAMP 234242424",
			want: &Want{
				Keyspace:    "test_keyspace",
				Table:       "test_table",
				IfNotExists: false,
				Assignments: []types.Assignment{
					types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk2"), types.NewLiteralValue("u123")),
					types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_blob"), types.NewLiteralValue([]byte{0x00, 0x03})),
					types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_bool"), types.NewLiteralValue(true)),
					types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "list_text"), types.NewLiteralValue([]types.GoValue{"item1", "item2", "item1"})),
					types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_int"), types.NewLiteralValue(int32(3))),
					types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_bigint"), types.NewLiteralValue(int64(8242842848))),
					types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk1"), types.NewLiteralValue("org1")),
				},
				AllParams:      []*types.ParameterMetadata{},
				UsingTimestamp: types.NewLiteralValue(int64(234242424)),
			},
		},
		{
			name:    "fails if blob value a string",
			query:   "INSERT INTO test_keyspace.test_table (pk2, col_blob, pk1) VALUES ('u123', '0x0003', 'org1')",
			wantErr: "invalid literal for type blob:",
		},
		{
			name:  "session keyspace",
			query: "INSERT INTO test_table (pk1, pk2) VALUES (?, ?)",
			want: &Want{
				Keyspace:    "test_keyspace",
				Table:       "test_table",
				IfNotExists: false,
				Assignments: []types.Assignment{
					types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk1"), types.NewParameterizedValue("value0")),
					types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk2"), types.NewParameterizedValue("value1")),
				},
				AllParams: []*types.ParameterMetadata{
					{
						Key:    "value0",
						Order:  0,
						Type:   types.TypeVarchar,
						Column: mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk1"),
					},
					{
						Key:    "value1",
						Order:  1,
						Type:   types.TypeVarchar,
						Column: mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk2"),
					},
				},
			},
			sessionKeyspace: "test_keyspace",
		},
		{
			name:  "insert a map with special characters and key words",
			query: "INSERT INTO test_keyspace.test_table (pk1, pk2, map_text_text) VALUES ('abc', 'pkval', {'foo': 'bar', 'key:': ':value', 'k}': '{v:k}'})",
			want: &Want{
				Table:    "test_table",
				Keyspace: "test_keyspace",
				Assignments: []types.Assignment{
					types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk1"), types.NewLiteralValue("abc")),
					types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk2"), types.NewLiteralValue("pkval")),
					types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "map_text_text"), types.NewLiteralValue(map[types.GoValue]types.GoValue{"foo": "bar", "key:": ":value", "k}": "{v:k}"})),
				},
				AllParams: []*types.ParameterMetadata{},
			},
		},
		{
			name:  "escaped single quotes",
			query: "INSERT INTO test_keyspace.test_table (pk1, pk2, col_int) VALUES ('abc', 'pkva''l', 3)",
			want: &Want{
				Table:    "test_table",
				Keyspace: "test_keyspace",
				Assignments: []types.Assignment{
					types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk1"), types.NewLiteralValue("abc")),
					types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk2"), types.NewLiteralValue("pkva'l")),
					types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_int"), types.NewLiteralValue(int32(3))),
				},
				AllParams: []*types.ParameterMetadata{},
			},
		},
		{
			name:  "success with NULL literal",
			query: "INSERT INTO test_keyspace.test_table (pk1, pk2, col_int) VALUES ('abc', 'pkval', NULL)",
			want: &Want{
				Table:    "test_table",
				Keyspace: "test_keyspace",
				Assignments: []types.Assignment{
					types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk1"), types.NewLiteralValue("abc")),
					types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk2"), types.NewLiteralValue("pkval")),
					types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_int"), types.NewLiteralValue(nil)),
				},
				AllParams: []*types.ParameterMetadata{},
			},
		},
		{
			name:    "no keyspace",
			query:   "INSERT INTO test_table (pk1, pk2, col_int) VALUES ('abc', 'pkva''l', 3)",
			want:    nil,
			wantErr: "no keyspace specified",
		},
		{
			name:    "no such table",
			query:   "INSERT INTO test_keyspace.no_such_table (pk1, pk2, col_int) VALUES ('abc', 'pkva''l', 3)",
			want:    nil,
			wantErr: "table 'no_such_table' does not exist",
		},
		{
			name:            "no such keyspace",
			query:           "INSERT INTO no_such_keyspace.test_table (pk1, pk2, col_int) VALUES ('abc', 'pkva''l', 3)",
			want:            nil,
			wantErr:         "keyspace 'no_such_keyspace' does not exist",
			sessionKeyspace: "test_keyspace",
		},
		{
			name:            "not enough values",
			query:           "INSERT INTO test_keyspace.test_table (pk1, pk2, col_int) VALUES ('abc', 'foo')",
			want:            nil,
			wantErr:         "found mismatch between column count (3) value count (2)",
			sessionKeyspace: "test_keyspace",
		},
		{
			name:            "not enough columns",
			query:           "INSERT INTO test_keyspace.test_table (pk1, pk2) VALUES ('abc', 'foo', 3)",
			want:            nil,
			wantErr:         "found mismatch between column count (2) value count (3)",
			sessionKeyspace: "test_keyspace",
		},
		{
			name:    "invalid query syntax (should error)",
			query:   "INSERT INTO test_keyspace.test_table",
			want:    nil,
			wantErr: "parsing error",
		},
		{
			name:    "invalid query syntax (should error)",
			query:   "INSERT INTO test_keyspace.",
			want:    nil,
			wantErr: "parsing error",
		},

		{
			name:    "invalid query syntax (should error)",
			query:   "INSERT INTO .test_table",
			want:    nil,
			wantErr: "parsing error",
		},

		{
			name:    "invalid query syntax (should error)",
			query:   "INSERT INTO test_keyspace.test_table () VALUES ()",
			want:    nil,
			wantErr: "parsing error",
		},
		{
			name:    "invalid query syntax (should error)",
			query:   "INSERT INTO test_keyspace.test_table (col_ts, col_int, pk1) VALUES (?, ?, ?)",
			want:    nil,
			wantErr: "missing value for primary key `pk2`",
		},
		{
			name:    "insert list with null element (should error)",
			query:   "INSERT INTO test_keyspace.test_table (pk1, pk2, list_text) VALUES ('pk1', 'pk2', ['a', null])",
			wantErr: "collection items are not allowed to be null",
		},
		{
			name:    "insert set with null element (should error)",
			query:   "INSERT INTO test_keyspace.test_table (pk1, pk2, set_text) VALUES ('pk1', 'pk2', {'a', null})",
			wantErr: "collection items are not allowed to be null",
		},
		{
			name:    "insert map with null value (should error)",
			query:   "INSERT INTO test_keyspace.test_table (pk1, pk2, map_text_bool) VALUES ('pk1', 'pk2', {'a': null})",
			wantErr: "map values cannot be null",
		},
		{
			name:    "insert map with null key (should error)",
			query:   "INSERT INTO test_keyspace.test_table (pk1, pk2, map_text_bool) VALUES ('pk1', 'pk2', {null: true})",
			wantErr: "map keys cannot be null",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := NewInsertTranslator(mockdata.GetSchemaMappingConfig())
			got, err := tr.Translate(types.NewRawQuery(nil, tt.sessionKeyspace, tt.query, parser.NewParser(tt.query), types.QueryTypeInsert), tt.sessionKeyspace)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			gotInsert := got.(*types.PreparedInsertQuery)
			assert.Equal(t, tt.want.Keyspace, gotInsert.Keyspace())
			assert.Equal(t, tt.want.Table, gotInsert.Table())
			assert.Equal(t, tt.want.IfNotExists, gotInsert.IfNotExists)
			assert.Equal(t, tt.want.Assignments, gotInsert.Assignments)
			assert.ElementsMatch(t, tt.want.AllParams, gotInsert.Parameters().Ordered())
		})
	}
}
