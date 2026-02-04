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

package update_translator

import (
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/parser"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/testing/mockdata"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type Want struct {
	Keyspace  types.Keyspace
	Table     types.TableName
	IfExists  bool
	Values    []types.Assignment
	RowKeys   []types.DynamicValue
	AllParams []*types.ParameterMetadata
}

func TestTranslator_TranslateUpdateQuerytoBigtable(t *testing.T) {
	tests := []struct {
		name            string
		sessionKeyspace types.Keyspace
		query           string
		want            *Want
		wantErr         string
	}{
		{
			name:  "update blob column",
			query: "UPDATE test_keyspace.test_table SET col_blob = 0x00000000000003 WHERE pk1 = 'testText' AND pk2 = 'pk2';",
			want: &Want{
				Keyspace: "test_keyspace",
				Table:    "test_table",
				IfExists: false,
				Values: []types.Assignment{
					types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_blob"), types.NewLiteralValue([]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03})),
				},
				RowKeys: []types.DynamicValue{
					types.NewLiteralValue("testText"),
					types.NewLiteralValue("pk2"),
				},
				AllParams: []*types.ParameterMetadata{},
			},
		},
		{
			name:  "if exists",
			query: "UPDATE test_keyspace.test_table SET col_blob = 0x00000000000003 WHERE pk1 = 'testText' AND pk2 = 'pk2' IF EXISTS;",
			want: &Want{
				Keyspace: "test_keyspace",
				Table:    "test_table",
				IfExists: true,
				Values: []types.Assignment{
					types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_blob"), types.NewLiteralValue([]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03})),
				},
				RowKeys: []types.DynamicValue{
					types.NewLiteralValue("testText"),
					types.NewLiteralValue("pk2"),
				},
				AllParams: []*types.ParameterMetadata{},
			},
		},
		{
			name:  "prepared update blob column",
			query: "UPDATE test_keyspace.test_table SET col_blob = ? WHERE pk1 = ? AND pk2 = ?;",
			want: &Want{
				Keyspace: "test_keyspace",
				Table:    "test_table",
				IfExists: false,
				AllParams: []*types.ParameterMetadata{
					{
						Key:    "value0",
						Order:  0,
						Type:   types.TypeBlob,
						Column: mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_blob"),
					},
					{
						Key:    "value1",
						Order:  1,
						Type:   types.TypeVarchar,
						Column: mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk1"),
					},
					{
						Key:    "value2",
						Order:  2,
						Type:   types.TypeVarchar,
						Column: mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk2"),
					},
				},
				Values: []types.Assignment{
					types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_blob"), types.NewParameterizedValue("value0")),
				},
				RowKeys: []types.DynamicValue{
					types.NewParameterizedValue("value1"),
					types.NewParameterizedValue("value2"),
				},
			},
		},
		{
			name:  "prepared update with named markers",
			query: "UPDATE test_keyspace.test_table SET col_blob = :v1 WHERE pk1 = :v2 AND pk2 = :v3;",
			want: &Want{
				Keyspace: "test_keyspace",
				Table:    "test_table",
				IfExists: false,
				AllParams: []*types.ParameterMetadata{
					{
						Key:     "v1",
						Order:   0,
						Type:    types.TypeBlob,
						IsNamed: true,
					},
					{
						Key:     "v2",
						Order:   1,
						Type:    types.TypeVarchar,
						IsNamed: true,
					},
					{
						Key:     "v3",
						Order:   2,
						Type:    types.TypeVarchar,
						IsNamed: true,
					},
				},
				Values: []types.Assignment{
					types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_blob"), types.NewParameterizedValue("v1")),
				},
				RowKeys: []types.DynamicValue{
					types.NewParameterizedValue("v2"),
					types.NewParameterizedValue("v3"),
				},
			},
		},
		{
			name:  "prepared update blob column with reverse pk order",
			query: "UPDATE test_keyspace.test_table SET col_blob = ? WHERE pk2 = ? AND pk1 = ?;",
			want: &Want{
				Keyspace: "test_keyspace",
				Table:    "test_table",
				IfExists: false,
				AllParams: []*types.ParameterMetadata{
					{
						Key:    "value0",
						Order:  0,
						Type:   types.TypeBlob,
						Column: mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_blob"),
					},
					{
						Key:    "value1",
						Order:  1,
						Type:   types.TypeVarchar,
						Column: mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk2"),
					},
					{
						Key:    "value2",
						Order:  2,
						Type:   types.TypeVarchar,
						Column: mockdata.GetColumnOrDie("test_keyspace", "test_table", "pk1"),
					},
				},
				Values: []types.Assignment{
					types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_blob"), types.NewParameterizedValue("value0")),
				},
				RowKeys: []types.DynamicValue{
					types.NewParameterizedValue("value2"),
					types.NewParameterizedValue("value1"),
				},
			},
		},
		{
			name:  "update boolean column",
			query: "UPDATE test_keyspace.test_table SET col_bool = true WHERE pk1 = 'testText' AND pk2 = 'pk2';",
			want: &Want{
				Keyspace: "test_keyspace",
				Table:    "test_table",
				IfExists: false,
				Values: []types.Assignment{
					types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_bool"), types.NewLiteralValue(true)),
				},
				RowKeys: []types.DynamicValue{
					types.NewLiteralValue("testText"),
					types.NewLiteralValue("pk2"),
				},
				AllParams: []*types.ParameterMetadata{},
			},
		},
		{
			name:  "update timestamp column",
			query: "UPDATE test_keyspace.test_table SET col_ts = '2024-08-12T12:34:56Z' WHERE pk1 = 'testText' AND pk2 = 'pk2';",
			want: &Want{
				Keyspace: "test_keyspace",
				Table:    "test_table",
				Values: []types.Assignment{
					types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_ts"), types.NewLiteralValue(time.Date(2024, 8, 12, 12, 34, 56, 0, time.UTC))),
				},
				RowKeys: []types.DynamicValue{
					types.NewLiteralValue("testText"),
					types.NewLiteralValue("pk2"),
				},
				AllParams: []*types.ParameterMetadata{},
			},
		},
		{
			name:  "update int column",
			query: "UPDATE test_keyspace.test_table SET col_int = 123 WHERE pk1 = 'testText' AND pk2 = 'pk2';",
			want: &Want{
				Keyspace: "test_keyspace",
				Table:    "test_table",
				Values: []types.Assignment{
					types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_int"), types.NewLiteralValue(int32(123))),
				},
				RowKeys: []types.DynamicValue{
					types.NewLiteralValue("testText"),
					types.NewLiteralValue("pk2"),
				},
				AllParams: []*types.ParameterMetadata{},
			},
		},
		{
			name:  "update set<varchar> column",
			query: "UPDATE test_keyspace.test_table SET set_text = {'item1', 'item2'} WHERE pk1 = 'testText' AND pk2 = 'pk2';",
			want: &Want{
				Keyspace: "test_keyspace",
				Table:    "test_table",
				Values: []types.Assignment{
					types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "set_text"), types.NewLiteralValue([]types.GoValue{"item1", "item2"})),
				},
				RowKeys: []types.DynamicValue{
					types.NewLiteralValue("testText"),
					types.NewLiteralValue("pk2"),
				},
				AllParams: []*types.ParameterMetadata{},
			},
		},
		{
			name:  "update map<varchar,boolean> column",
			query: "UPDATE test_keyspace.test_table SET map_text_bool = {'key1': true, 'key2': false} WHERE pk1 = 'testText' AND pk2 = 'pk2';",
			want: &Want{
				Keyspace: "test_keyspace",
				Table:    "test_table",
				Values: []types.Assignment{
					types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "map_text_bool"), types.NewLiteralValue(map[types.GoValue]types.GoValue{"key1": true, "key2": false})),
				},
				RowKeys: []types.DynamicValue{
					types.NewLiteralValue("testText"),
					types.NewLiteralValue("pk2"),
				},
				AllParams: []*types.ParameterMetadata{},
			},
		},
		{
			name:  "update bigint column",
			query: "UPDATE test_keyspace.test_table SET col_bigint = 1234567890 WHERE pk1 = 'testText' AND pk2 = 'pk2';",
			want: &Want{
				Keyspace: "test_keyspace",
				Table:    "test_table",
				Values: []types.Assignment{
					types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_bigint"), types.NewLiteralValue(int64(1234567890))),
				},
				RowKeys: []types.DynamicValue{
					types.NewLiteralValue("testText"),
					types.NewLiteralValue("pk2"),
				},
				AllParams: []*types.ParameterMetadata{},
			},
		},
		{
			name:  "with keyspace in query, without default keyspace",
			query: "UPDATE test_keyspace.test_table SET col_blob = 0x01 WHERE pk2 = 'pkval' AND pk1 = 'abc';",
			want: &Want{
				Keyspace: "test_keyspace",
				Table:    "test_table",
				Values: []types.Assignment{
					types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_blob"), types.NewLiteralValue([]byte{0x01})),
				},
				RowKeys: []types.DynamicValue{
					types.NewLiteralValue("abc"),
					types.NewLiteralValue("pkval"),
				},
				AllParams: []*types.ParameterMetadata{},
			},
		},
		{
			name:  "append to set column with + operator",
			query: "UPDATE test_keyspace.test_table SET set_text = set_text + {'item3'} WHERE pk1 = 'testText' AND pk2 = 'pk2';",
			want: &Want{
				Keyspace: "test_keyspace",
				Table:    "test_table",
				Values: []types.Assignment{
					types.NewComplexAssignmentAppend(mockdata.GetColumnOrDie("test_keyspace", "test_table", "set_text"), types.PLUS, types.NewLiteralValue([]types.GoValue{"item3"}), false),
				},
				RowKeys: []types.DynamicValue{
					types.NewLiteralValue("testText"),
					types.NewLiteralValue("pk2"),
				},
				AllParams: []*types.ParameterMetadata{},
			},
		},
		{
			name:  "subtract from set column with - operator",
			query: "UPDATE test_keyspace.test_table SET set_text = set_text - {'item2'} WHERE pk1 = 'testText' AND pk2 = 'pk2';",
			want: &Want{
				Keyspace: "test_keyspace",
				Table:    "test_table",
				Values: []types.Assignment{
					types.NewComplexAssignmentAppend(mockdata.GetColumnOrDie("test_keyspace", "test_table", "set_text"), types.MINUS, types.NewLiteralValue([]types.GoValue{"item2"}), false),
				},
				RowKeys: []types.DynamicValue{
					types.NewLiteralValue("testText"),
					types.NewLiteralValue("pk2"),
				},
				AllParams: []*types.ParameterMetadata{},
			},
		},
		{
			name:  "counter operation",
			query: "UPDATE test_keyspace.test_table SET col_counter = col_counter + 1 WHERE pk1 = 'testText' AND pk2 = 'pk2';",
			want: &Want{
				Keyspace: "test_keyspace",
				Table:    "test_table",
				Values: []types.Assignment{
					types.NewAssignmentCounterIncrement(mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_counter"), types.PLUS, types.NewLiteralValue(int64(1))),
				},
				RowKeys: []types.DynamicValue{
					types.NewLiteralValue("testText"),
					types.NewLiteralValue("pk2"),
				},
				AllParams: []*types.ParameterMetadata{},
			},
		},
		{
			name:    "counter operation with invalid operator",
			query:   "UPDATE test_keyspace.test_table SET col_counter = col_counter * 1 WHERE pk1 = 'testText' AND pk2 = 'pk2';",
			wantErr: "parsing error",
		},
		{
			name:  "counter operation decrement",
			query: "UPDATE test_keyspace.test_table SET col_counter = col_counter - 9 WHERE pk1 = 'testText' AND pk2 = 'pk2';",
			want: &Want{
				Keyspace: "test_keyspace",
				Table:    "test_table",
				Values: []types.Assignment{
					types.NewAssignmentCounterIncrement(mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_counter"), types.MINUS, types.NewLiteralValue(int64(9))),
				},
				RowKeys: []types.DynamicValue{
					types.NewLiteralValue("testText"),
					types.NewLiteralValue("pk2"),
				},
				AllParams: []*types.ParameterMetadata{},
			},
		},
		{
			name:  "counter operation increment a negative value",
			query: "UPDATE test_keyspace.test_table SET col_counter = col_counter + -9 WHERE pk1 = 'testText' AND pk2 = 'pk2';",
			want: &Want{
				Keyspace: "test_keyspace",
				Table:    "test_table",
				Values: []types.Assignment{
					types.NewAssignmentCounterIncrement(mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_counter"), types.PLUS, types.NewLiteralValue(int64(-9))),
				},
				RowKeys: []types.DynamicValue{
					types.NewLiteralValue("testText"),
					types.NewLiteralValue("pk2"),
				},
				AllParams: []*types.ParameterMetadata{},
			},
		},
		{
			name:  "with keyspace in query, with default keyspace",
			query: "UPDATE test_keyspace.test_table SET col_blob = 0x01 WHERE pk2 = 'pkval' AND pk1 = 'abc';",
			want: &Want{
				Keyspace: "test_keyspace",
				Table:    "test_table",
				Values: []types.Assignment{
					types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_blob"), types.NewLiteralValue([]byte{0x01})),
				},
				RowKeys: []types.DynamicValue{
					types.NewLiteralValue("abc"),
					types.NewLiteralValue("pkval"),
				},
				AllParams: []*types.ParameterMetadata{},
			},
		},
		{
			name:    "assign list to set",
			query:   "UPDATE test_keyspace.test_table SET set_text = ['item1'] WHERE pk1 = 'testText' AND pk2 = 'pk2';",
			wantErr: "cannot parse list value for non-list type: set<text>",
		},
		{
			name:  "update with list assignment",
			query: "UPDATE test_keyspace.test_table SET set_text = {'item1', 'item2'} WHERE pk1 = 'testText' AND pk2 = 'pk2';",
			want: &Want{
				Keyspace: "test_keyspace",
				Table:    "test_table",
				Values: []types.Assignment{
					types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "set_text"), types.NewLiteralValue([]types.GoValue{"item1", "item2"})),
				},
				RowKeys: []types.DynamicValue{
					types.NewLiteralValue("testText"),
					types.NewLiteralValue("pk2"),
				},
				AllParams: []*types.ParameterMetadata{},
			},
		},
		{
			name:  "update with map assignment",
			query: "UPDATE test_keyspace.test_table SET map_text_bool = {'key1': true, 'key2': false} WHERE pk1 = 'testText' AND pk2 = 'pk2';",
			want: &Want{
				Keyspace: "test_keyspace",
				Table:    "test_table",
				Values: []types.Assignment{
					types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "map_text_bool"), types.NewLiteralValue(map[types.GoValue]types.GoValue{"key1": true, "key2": false})),
				},
				RowKeys: []types.DynamicValue{
					types.NewLiteralValue("testText"),
					types.NewLiteralValue("pk2"),
				},
				AllParams: []*types.ParameterMetadata{},
			},
		},
		{
			name:  "update with set assignment",
			query: "UPDATE test_keyspace.test_table SET set_text = {'item1', 'item2'} WHERE pk1 = 'testText' AND pk2 = 'pk2';",
			want: &Want{
				Keyspace: "test_keyspace",
				Table:    "test_table",
				Values: []types.Assignment{
					types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "set_text"), types.NewLiteralValue([]types.GoValue{"item1", "item2"})),
				},
				RowKeys: []types.DynamicValue{
					types.NewLiteralValue("testText"),
					types.NewLiteralValue("pk2"),
				},
				AllParams: []*types.ParameterMetadata{},
			},
		},
		{
			name:    "attempt to update primary key with collection (should error)",
			query:   "UPDATE test_keyspace.test_table SET pk1 = ['item1'] WHERE pk2 = 'pk2';",
			wantErr: "cannot parse list value for non-list type: varchar",
		},
		{
			name:    "invalid collection syntax (should error)",
			query:   "UPDATE test_keyspace.test_table SET set_text = ['item1', WHERE pk1 = 'testText';",
			wantErr: "parsing error",
		},
		{
			name:    "collection assignment to non-collection column (should error)",
			query:   "UPDATE test_keyspace.test_table SET col_blob = ['item1'] WHERE pk1 = 'testText';",
			wantErr: "cannot parse list value for non-list type: blob",
		},
		{
			name:            "without keyspace in query, with default keyspace",
			query:           "UPDATE test_table SET col_blob = 0x01 WHERE pk2 = 'pkval' AND pk1 = 'abc';",
			sessionKeyspace: "test_keyspace",
			wantErr:         "",
			want: &Want{
				Keyspace: "test_keyspace",
				Table:    "test_table",
				Values: []types.Assignment{
					types.NewComplexAssignmentSet(mockdata.GetColumnOrDie("test_keyspace", "test_table", "col_blob"), types.NewLiteralValue([]byte{0x01})),
				},
				RowKeys: []types.DynamicValue{
					types.NewLiteralValue("abc"),
					types.NewLiteralValue("pkval"),
				},
				AllParams: []*types.ParameterMetadata{},
			},
		},
		{
			name:    "invalid index update missing bracket (should error)",
			query:   "UPDATE test_keyspace.test_table SET column71 = 'newItem' WHERE pk1 = 'testText';",
			wantErr: "unknown column 'column71'",
		},
		{
			name:    "without keyspace in query, without default keyspace (should error)",
			query:   "UPDATE test_table SET col_blob = 'abc' WHERE pk2 = 'pkval' AND pk1 = 'abc';",
			wantErr: "no keyspace specified",
		},
		{
			name:    "invalid query syntax (should error)",
			query:   "UPDATE test_keyspace.test_table",
			wantErr: "parsing error",
		},
		{
			name:    "parser returns empty table (should error)",
			query:   "UPDATE test_keyspace. SET pk1 = 'abc' WHERE pk2 = 'pkval';",
			wantErr: "parsing error",
		},
		{
			name:    "parser returns empty keyspace (should error)",
			query:   "UPDATE .test_table SET pk1 = 'abc' WHERE pk2 = 'pkval';",
			wantErr: "parsing error",
		},
		{
			name:    "parser returns empty set clause (should error)",
			query:   "UPDATE test_keyspace.test_table SET WHERE pk2 = 'pkval';",
			wantErr: "parsing error",
		},
		{
			name:    "keyspace does not exist (should error)",
			query:   "UPDATE invalid_keyspace.test_table SET pk1 = 'abc' WHERE pk2 = 'pkval';",
			wantErr: "keyspace 'invalid_keyspace' does not exist",
		},
		{
			name:    "table does not exist (should error)",
			query:   "UPDATE test_keyspace.invalid_table SET pk1 = 'abc' WHERE pk2 = 'pkval';",
			wantErr: "table 'invalid_table' does not exist",
		},
		{
			name:    "missing primary key in where clause (should error)",
			query:   "UPDATE test_keyspace.test_table SET col_blob = 0x01 WHERE pk1 = 'testText'", // Missing pk2
			wantErr: "all primary keys must be included in the where clause. missing `pk2`",
			want:    nil,
		},
		{
			name:    "missing all primary keys in where clause (should error)",
			query:   "UPDATE test_keyspace.test_table SET col_blob = 0x01 WHERE column3 = true", // No PK columns
			wantErr: "unknown column 'column3' in table test_keyspace.test_table",
			want:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := NewUpdateTranslator(mockdata.GetSchemaMappingConfig())
			got, err := tr.Translate(types.NewRawQuery(nil, tt.sessionKeyspace, tt.query, parser.NewParser(tt.query), types.QueryTypeUpdate), tt.sessionKeyspace)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			gotUpdate := got.(*types.PreparedUpdateQuery)
			assert.Equal(t, tt.want.Keyspace, gotUpdate.Keyspace())
			assert.Equal(t, tt.want.Table, gotUpdate.Table())
			assert.Equal(t, tt.want.IfExists, gotUpdate.IfExists)
			assert.Equal(t, tt.want.Values, gotUpdate.Values)
			assert.Equal(t, tt.want.RowKeys, gotUpdate.RowKeys)
			assert.ElementsMatch(t, tt.want.AllParams, gotUpdate.Parameters().Ordered())
		})
	}
}
