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
package metadata

import (
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"reflect"
	"testing"
)

func getSchemaMappingConfig() *SchemaMetadata {
	return NewSchemaMetadata(
		"cf1",
		[]*TableSchema{
			NewTableConfig(
				"keyspace",
				"table",
				"cf1",
				types.OrderedCodeEncoding,
				[]*types.Column{
					{
						Name:         "column1",
						CQLType:      types.TypeVarchar,
						KeyType:      types.KeyTypeRegular,
						ColumnFamily: "cf1",
					},
					{
						Name:         "column2",
						CQLType:      types.TypeInt,
						KeyType:      types.KeyTypeRegular,
						ColumnFamily: "cf1",
					},
				},
			),
			NewTableConfig(
				"keyspace",
				"other",
				"cf1",
				types.OrderedCodeEncoding,
				[]*types.Column{
					{
						Name:         "id",
						CQLType:      types.TypeInt,
						KeyType:      "partition",
						IsPrimaryKey: true,
					},
					{
						Name:         "name",
						CQLType:      types.TypeVarchar,
						KeyType:      "clustering",
						IsPrimaryKey: true,
					},
				},
			),
		},
	)
}

func Test_GetColumn(t *testing.T) {
	columnExistsInDifferentTableArgs := struct {
		tableName  string
		columnName string
	}{
		tableName:  "table",
		columnName: "column3",
	}

	tests := []struct {
		name   string
		fields *SchemaMetadata
		args   struct {
			tableName  string
			columnName string
		}
		want    *types.Column
		wantErr bool
	}{
		{
			name:   "types.Columns exists",
			fields: getSchemaMappingConfig(),
			args: struct {
				tableName  string
				columnName string
			}{
				tableName:  "table",
				columnName: "column1",
			},
			want: &types.Column{
				Name:         "column1",
				ColumnFamily: "cf1",
				IsPrimaryKey: false,
				CQLType:      types.TypeVarchar,
			},
			wantErr: false,
		},
		{
			name:    "types.Columns exists in different table",
			fields:  getSchemaMappingConfig(),
			args:    columnExistsInDifferentTableArgs,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc, err := tt.fields.GetTableSchema("keyspace", types.TableName(tt.args.tableName))
			if err != nil {
				t.Errorf("table config error: %v", err)
				return
			}
			got, err := tc.GetColumn(types.ColumnName(tt.args.columnName))
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want.Name, got.Name)
				assert.Equal(t, tt.want.ColumnFamily, got.ColumnFamily)
				assert.Equal(t, tt.want.IsPrimaryKey, got.IsPrimaryKey)
				assert.Equal(t, tt.want.CQLType, got.CQLType)
			}
		})
	}
}

func Test_ListKeyspaces(t *testing.T) {
	tests := []struct {
		name     string
		tables   []*TableSchema
		expected []types.Keyspace
	}{
		{
			name: "Multiple keyspaces, unsorted input",
			tables: []*TableSchema{
				createTestTable("zeta", "t"),
				createTestTable("alpha", "t"),
				createTestTable("beta", "t"),
			},
			expected: []types.Keyspace{"alpha", "beta", "zeta"},
		},
		{
			name: "Single keyspace",
			tables: []*TableSchema{
				createTestTable("only", "t"),
			},
			expected: []types.Keyspace{"only"},
		},
		{
			name:     "No keyspaces",
			tables:   []*TableSchema{},
			expected: []types.Keyspace{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewSchemaMetadata("cf1", tt.tables)
			assert.Equal(t, tt.expected, cfg.ListKeyspaces())
		})
	}
}

func createTestTable(keyspace types.Keyspace, name types.TableName) *TableSchema {
	return NewTableConfig(keyspace, name, "cf1", types.OrderedCodeEncoding, []*types.Column{
		{
			Name:         "pk1",
			CQLType:      types.TypeText,
			IsPrimaryKey: true,
			PkPrecedence: 1,
			KeyType:      types.KeyTypePartition,
		},
		{
			Name:         "name",
			CQLType:      types.TypeText,
			IsPrimaryKey: false,
			PkPrecedence: 0,
			KeyType:      types.KeyTypeRegular,
		},
	})
}

func Test_sortPrimaryKeysData(t *testing.T) {
	type args struct {
		pkMetadata []*types.Column
	}
	tests := []struct {
		name string
		args args
		want []*types.Column
	}{
		{
			name: "Basic Sorting",
			args: args{
				pkMetadata: []*types.Column{
					{Name: "id", PkPrecedence: 2},
					{Name: "email", PkPrecedence: 1},
				},
			},
			want: []*types.Column{
				{Name: "email", PkPrecedence: 1},
				{Name: "id", PkPrecedence: 2},
			},
		},
		{
			name: "Already Sorted Data",
			args: args{
				pkMetadata: []*types.Column{
					{Name: "order_id", PkPrecedence: 1},
					{Name: "customer_id", PkPrecedence: 2},
				},
			},
			want: []*types.Column{
				{Name: "order_id", PkPrecedence: 1},
				{Name: "customer_id", PkPrecedence: 2},
			},
		},
		{
			name: "SameSchema Precedence Columns (Unchanged Order)",
			args: args{
				pkMetadata: []*types.Column{
					{Name: "product_id", PkPrecedence: 1},
					{Name: "category_id", PkPrecedence: 1},
				},
			},
			want: []*types.Column{
				{Name: "product_id", PkPrecedence: 1},
				{Name: "category_id", PkPrecedence: 1}, // Order should remain the same
			},
		},
		{
			name: "Single types.Columns (No Sorting Needed)",
			args: args{
				pkMetadata: []*types.Column{
					{Name: "category_id", PkPrecedence: 1},
				},
			},
			want: []*types.Column{
				{Name: "category_id", PkPrecedence: 1},
			},
		},
		{
			name: "Empty Slice (No Operation)",
			args: args{
				pkMetadata: []*types.Column{},
			},
			want: []*types.Column{},
		},
		{
			name: "Table With Nil Columns (Should Not Panic)",
			args: args{
				pkMetadata: nil,
			},
			want: nil,
		},
		{
			name: "Negative Precedence Columns (Still Sorted Correctly)",
			args: args{
				pkMetadata: []*types.Column{
					{Name: "col1", PkPrecedence: -1},
					{Name: "col2", PkPrecedence: -3},
					{Name: "col3", PkPrecedence: -2},
				},
			},
			want: []*types.Column{
				{Name: "col2", PkPrecedence: -3},
				{Name: "col3", PkPrecedence: -2},
				{Name: "col1", PkPrecedence: -1},
			},
		},
		{
			name: "Zero Precedence Columns (Sorted Normally)",
			args: args{
				pkMetadata: []*types.Column{
					{Name: "colA", PkPrecedence: 0},
					{Name: "colB", PkPrecedence: 2},
					{Name: "colC", PkPrecedence: 1},
				},
			},
			want: []*types.Column{
				{Name: "colA", PkPrecedence: 0},
				{Name: "colC", PkPrecedence: 1},
				{Name: "colB", PkPrecedence: 2},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sortPrimaryKeys(tt.args.pkMetadata)
			if !reflect.DeepEqual(tt.args.pkMetadata, tt.want) {
				t.Errorf("sortPrimaryKeys() = %v, want %v", tt.args.pkMetadata, tt.want)
			}
		})
	}
}

type MetadataListener struct {
	values []MetadataEvent
}

func NewMetadataListener() *MetadataListener {
	return &MetadataListener{}
}

func (m *MetadataListener) OnEvent(e MetadataEvent) {
	println("got event: " + e.EventType)
	m.values = append(m.values, e)
}

func Test_SyncTablesChanged(t *testing.T) {
	schemas := getSchemaMappingConfig()
	listener := NewMetadataListener()
	schemas.Subscribe(listener)

	schemas.SyncKeyspace("keyspace", []*TableSchema{
		NewTableConfig(
			"keyspace",
			"table",
			"cf1",
			types.OrderedCodeEncoding,
			[]*types.Column{
				{
					Name:         "column1",
					CQLType:      types.TypeVarchar,
					KeyType:      types.KeyTypeRegular,
					ColumnFamily: "cf1",
				},
				{
					Name:         "column2",
					CQLType:      types.TypeInt,
					KeyType:      types.KeyTypeRegular,
					ColumnFamily: "cf1",
				},
				{
					Name:         "column3", // new column
					CQLType:      types.TypeInt,
					KeyType:      types.KeyTypeRegular,
					ColumnFamily: "cf1",
				},
			},
		),
		NewTableConfig(
			"keyspace",
			"other",
			"cf1",
			types.OrderedCodeEncoding,
			[]*types.Column{
				{
					Name:         "id",
					CQLType:      types.TypeInt,
					KeyType:      "partition",
					IsPrimaryKey: true,
				},
				{
					Name:         "name",
					CQLType:      types.TypeVarchar,
					KeyType:      "clustering",
					IsPrimaryKey: true,
				},
			},
		),
	})

	assert.Equal(t, []MetadataEvent{
		{
			EventType: MetadataChangedEventType,
			Keyspace:  "keyspace",
			Table:     "table",
		},
	}, listener.values)
}

func Test_SyncTablesMixed(t *testing.T) {
	schemas := getSchemaMappingConfig()
	listener := NewMetadataListener()
	schemas.Subscribe(listener)

	schemas.SyncKeyspace("keyspace", []*TableSchema{
		NewTableConfig(
			"keyspace",
			"table",
			"cf1",
			types.OrderedCodeEncoding,
			[]*types.Column{
				{
					Name:         "column1",
					CQLType:      types.TypeVarchar,
					KeyType:      types.KeyTypeRegular,
					ColumnFamily: "cf1",
				},
				{
					Name:         "column2",
					CQLType:      types.TypeInt,
					KeyType:      types.KeyTypeRegular,
					ColumnFamily: "cf1",
				},
			},
		),
		NewTableConfig(
			"keyspace",
			"other",
			"cf1",
			types.OrderedCodeEncoding,
			[]*types.Column{
				{
					Name:         "id",
					CQLType:      types.TypeInt,
					KeyType:      "partition",
					IsPrimaryKey: true,
				},
				{
					Name:         "name2",
					CQLType:      types.TypeVarchar,
					KeyType:      "clustering",
					IsPrimaryKey: true,
				},
			},
		),
		NewTableConfig(
			"keyspace",
			"other2",
			"cf1",
			types.OrderedCodeEncoding,
			[]*types.Column{
				{
					Name:         "id",
					CQLType:      types.TypeInt,
					KeyType:      "partition",
					IsPrimaryKey: true,
				},
				{
					Name:         "name",
					CQLType:      types.TypeVarchar,
					KeyType:      "clustering",
					IsPrimaryKey: true,
				},
			},
		),
	})

	assert.ElementsMatch(t, []MetadataEvent{
		{
			EventType: MetadataChangedEventType,
			Keyspace:  "keyspace",
			Table:     "other",
		},
		{
			EventType: MetadataAddedEventType,
			Keyspace:  "keyspace",
			Table:     "other2",
		},
	}, listener.values)
}

func Test_SyncTablesDrop(t *testing.T) {
	schemas := getSchemaMappingConfig()
	listener := NewMetadataListener()
	schemas.Subscribe(listener)

	schemas.SyncKeyspace("keyspace", []*TableSchema{
		NewTableConfig(
			"keyspace",
			"table",
			"cf1",
			types.OrderedCodeEncoding,
			[]*types.Column{
				{
					Name:         "column1",
					CQLType:      types.TypeVarchar,
					KeyType:      types.KeyTypeRegular,
					ColumnFamily: "cf1",
				},
				{
					Name:         "column2",
					CQLType:      types.TypeInt,
					KeyType:      types.KeyTypeRegular,
					ColumnFamily: "cf1",
				},
			},
		),
	})

	assert.ElementsMatch(t, []MetadataEvent{
		{
			EventType: MetadataRemovedEventType,
			Keyspace:  "keyspace",
			Table:     "other",
		},
	}, listener.values)
}

func Test_CalculateSchemaVersion(t *testing.T) {
	schemas := getSchemaMappingConfig()
	v1, err := schemas.CalculateSchemaVersion()
	require.NoError(t, err)

	v2, err := schemas.CalculateSchemaVersion()
	require.NoError(t, err)

	// no changes happened, so we should have the same schema version
	assert.Equal(t, v1, v2)

	schemas.AddTables(
		[]*TableSchema{
			NewTableConfig(
				"keyspace",
				"table2",
				"cf1",
				types.OrderedCodeEncoding,
				[]*types.Column{
					{
						Name:         "column1",
						CQLType:      types.TypeVarchar,
						KeyType:      types.KeyTypeRegular,
						ColumnFamily: "cf1",
					},
					{
						Name:         "column2",
						CQLType:      types.TypeInt,
						KeyType:      types.KeyTypeRegular,
						ColumnFamily: "cf1",
					},
				},
			),
		},
	)

	v3, err := schemas.CalculateSchemaVersion()
	require.NoError(t, err)

	// we added a table, so the schema should be changed
	assert.NotEqual(t, v1, v3)

	// sync keyspace which will effectively drop the table we just added
	schemas.SyncKeyspace("keyspace", getSchemaMappingConfig().Tables())

	v4, err := schemas.CalculateSchemaVersion()
	require.NoError(t, err)

	// we dropped a table, so the schema should be changed
	assert.NotEqual(t, v3, v4)
	// we are back to v1 because we dropped the new table
	assert.Equal(t, v1, v4)

	// sync keyspace with the same schemas, so nothing should change
	schemas.SyncKeyspace("keyspace", getSchemaMappingConfig().Tables())

	v5, err := schemas.CalculateSchemaVersion()
	require.NoError(t, err)

	// we synced but no schemas changed so we should be back to the v1 version
	assert.Equal(t, v1, v5)
}
