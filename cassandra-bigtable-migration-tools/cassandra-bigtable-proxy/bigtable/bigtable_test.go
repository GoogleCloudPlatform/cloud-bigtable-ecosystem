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

package bigtableclient

import (
	"context"
	"fmt"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/testing/bt_server_wrapper"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/testing/mockdata"
	"golang.org/x/exp/maps"
	"os"
	"testing"

	"cloud.google.com/go/bigtable"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	schemaMapping "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/metadata"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

var bts *bt_server_wrapper.BigtableTestServer
var bigtableConfig = types.BigtableConfig{
	ProjectID:          "my-project",
	Instances:          map[types.Keyspace]*types.InstanceMapping{"test_keyspace": {InstanceId: "bt1", Keyspace: "test_keyspace", AppProfileID: "default"}},
	SchemaMappingTable: "schema_mapping",
	Session: &types.Session{
		GrpcChannels: 3,
	},
	DefaultColumnFamily:      "cf1",
	DefaultIntRowKeyEncoding: types.OrderedCodeEncoding,
}

var mdStore *schemaMapping.MetadataStore
var btc *BigtableAdapter

const keyspace = types.Keyspace("test_keyspace")
const tableName = types.TableName("test_table")

func TestMain(m *testing.M) {
	bts = bt_server_wrapper.NewBigtableTestServer(bigtableConfig)
	bts.SetUp(0)
	ctx := context.Background()

	mdStore = schemaMapping.NewMetadataStore(zap.NewNop(), bts.Clients(), &bigtableConfig)
	err := mdStore.Initialize(ctx)
	if err != nil {
		fmt.Printf("failed to initialize md store: %v", err)
		os.Exit(1)
	}
	btc = NewBigtableClient(bts.Clients(), zap.NewNop(), &bigtableConfig, mdStore)

	_, err = mdStore.CreateTable(ctx, types.NewCreateTableStatementMap(keyspace, tableName, "ignored", false, []types.CreateColumn{
		{
			Name:     "pk1",
			Index:    0,
			TypeInfo: types.TypeText,
		},
		{
			Name:     "name",
			Index:    1,
			TypeInfo: types.TypeText,
		},
		{
			Name:     "tags",
			TypeInfo: types.NewListType(types.TypeText),
		},
	}, []types.CreateTablePrimaryKeyConfig{
		{
			Name:    "pk1",
			KeyType: types.KeyTypePartition,
		},
	}, types.OrderedCodeEncoding))
	if err != nil {
		fmt.Printf("failed to setup table: %v", err)
		os.Exit(1)
	}
	if len(mdStore.Schemas().Tables()) == 0 {
		fmt.Printf("failed to setup table: no tables")
		os.Exit(1)
	}

	defer bts.Close()
	os.Exit(m.Run())
}

func TestInsertRow(t *testing.T) {
	mdStore := schemaMapping.NewMetadataStore(zap.NewNop(), bts.Clients(), &bigtableConfig)
	mdStore.Schemas().AddTables(mockdata.GetSchemaMappingConfig().Tables())

	localBtc := NewBigtableClient(bts.Clients(), zap.NewNop(), &bigtableConfig, mdStore)

	tests := []struct {
		name          string
		keyspace      types.Keyspace
		table         types.TableName
		rowKey        types.RowKey
		ifNotExists   bool
		mutations     []types.IBigtableMutationOp
		expectedError string
		expectedValue map[string][]byte
	}{
		{
			name:     "insert row",
			keyspace: "test_keyspace",
			table:    "test_table",
			rowKey:   "row1",
			mutations: []types.IBigtableMutationOp{
				types.NewWriteCellOp("cf1", "col1", []byte("value1")),
			},
			expectedValue: map[string][]byte{
				"":     nil,
				"col1": []byte("value1"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mut := types.NewBigtableWriteMutation(tt.keyspace, tt.table, "ignored", types.IfSpec{IfNotExists: tt.ifNotExists}, types.QueryTypeInsert, tt.rowKey)
			mut.AddMutations(tt.mutations...)
			_, err := localBtc.InsertRow(t.Context(), mut)
			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				return
			}
			assert.NoError(t, err)
			client, err := btc.clients.GetClient(tt.keyspace)
			require.NoError(t, err)
			table := client.Open(string(tt.table))
			row, err := table.ReadRow(t.Context(), string(tt.rowKey))
			require.NoError(t, err)
			require.NotEmpty(t, row)
			require.Contains(t, row, "cf1")
			require.Equal(t, len(tt.expectedValue), len(row["cf1"]))
			for _, item := range row["cf1"] {
				columnName := item.Column[len("cf1:"):]
				assert.Equal(t, tt.expectedValue[columnName], item.Value)
			}
		})
	}
}

func TestDeleteRow(t *testing.T) {
	rowKey := types.RowKey("test-row")
	// Insert initial row
	initialData := types.NewBigtableWriteMutation(keyspace, tableName, "ignored", types.IfSpec{}, types.QueryTypeInsert, rowKey)
	initialData.AddMutations(types.NewWriteCellOp("cf1", "name", []byte("initial value")))

	_, err := btc.InsertRow(t.Context(), initialData)
	require.NoError(t, err)

	deleteQuery := types.NewBoundDeleteQuery(keyspace, tableName, "", rowKey, false, nil)
	_, err = btc.DeleteRow(t.Context(), deleteQuery)
	require.NoError(t, err)

	// Verify deletion
	tbl, err := btc.clients.GetTableClient(keyspace, tableName)
	require.NoError(t, err)

	row, err := tbl.ReadRow(t.Context(), string(rowKey))
	assert.NoError(t, err)
	assert.Empty(t, row)
}

func TestApplyBulkMutation(t *testing.T) {
	_, err := btc.ApplyBulkMutation(t.Context(), keyspace, tableName, []types.IBigtableMutation{
		types.NewBigtableWriteMutation(keyspace, tableName, "", types.IfSpec{}, types.QueryTypeInsert, "bulk1").
			AddMutations(types.NewWriteCellOp("cf1", "name", []byte("buddy"))),
		types.NewBigtableWriteMutation(keyspace, tableName, "", types.IfSpec{}, types.QueryTypeInsert, "bulk2").
			AddMutations(types.NewWriteCellOp("cf1", "name", []byte("santa"))),
		types.NewBigtableWriteMutation(keyspace, tableName, "", types.IfSpec{}, types.QueryTypeInsert, "bulk3").
			AddMutations(types.NewWriteCellOp("cf1", "name", []byte("walter"))),
		// update row1
		types.NewBigtableWriteMutation(keyspace, tableName, "", types.IfSpec{}, types.QueryTypeUpdate, "bulk1").
			AddMutations(types.NewWriteCellOp("cf1", "name", []byte("buddy-2"))),
		// delete bulk2
		types.NewBoundDeleteQuery(keyspace, tableName, "", "bulk2", false, nil),
	})
	require.NoError(t, err)

	tbl, err := btc.clients.GetTableClient(keyspace, tableName)
	require.NoError(t, err)

	// Verify mutations
	row1, err := tbl.ReadRow(t.Context(), "bulk1")
	assert.NoError(t, err)
	assert.NotEmpty(t, row1)
	assert.Equal(t, []byte(nil), row1["cf1"][0].Value) // empty row cell
	assert.Equal(t, []byte("buddy-2"), row1["cf1"][1].Value)

	row2, err := tbl.ReadRow(t.Context(), "bulk2")
	assert.NoError(t, err)
	assert.Empty(t, row2)

	row3, err := tbl.ReadRow(t.Context(), "bulk3")
	assert.NoError(t, err)
	assert.NotEmpty(t, row3)
	assert.Equal(t, []byte(nil), row3["cf1"][0].Value) // empty row cell
	assert.Equal(t, []byte("walter"), row3["cf1"][1].Value)
}

func TestMutateRowDeleteColumnFamily(t *testing.T) {
	const key = "delete-cf"
	var err error
	// Insert initial data
	insertData := types.NewBigtableWriteMutation(keyspace, tableName, "", types.IfSpec{}, types.QueryTypeInsert, key)
	insertData.AddMutations(types.NewWriteCellOp("cf1", "col1", []byte("v1")))
	insertData.AddMutations(types.NewWriteCellOp("tags", "0", []byte("tag1")))
	insertData.AddMutations(types.NewWriteCellOp("tags", "1", []byte("tag2")))
	_, err = btc.InsertRow(t.Context(), insertData)
	require.NoError(t, err)

	// Verify both column families are populated
	tbl, err := btc.clients.GetTableClient(keyspace, tableName)
	require.NoError(t, err)
	row, err := tbl.ReadRow(t.Context(), key)
	require.NoError(t, err)
	assert.ElementsMatch(t, maps.Keys(row), []string{"cf1", "tags"})

	// Delete cf2
	updateData := types.NewBigtableWriteMutation(keyspace, tableName, "", types.IfSpec{}, types.QueryTypeUpdate, key)
	updateData.AddMutations(types.NewDeleteCellsOp("tags"))
	_, err = btc.UpdateRow(t.Context(), updateData)
	require.NoError(t, err)

	// Verify deletion by reading the row
	row, err = tbl.ReadRow(t.Context(), key)
	require.NoError(t, err)
	assert.ElementsMatch(t, maps.Keys(row), []string{"cf1"}, "cf1 should still exist and tags should be deleted")
}

func TestMutateRowDeleteQualifiers(t *testing.T) {
	var err error
	const key = "delete-qualifiers"

	// Insert initial data with two columns
	insertData := types.NewBigtableWriteMutation(keyspace, tableName, "", types.IfSpec{}, types.QueryTypeInsert, key)
	insertData.AddMutations(types.NewWriteCellOp("cf1", "col1", []byte("v1")))
	insertData.AddMutations(types.NewWriteCellOp("cf1", "col2", []byte("v2")))
	_, err = btc.InsertRow(t.Context(), insertData)
	require.NoError(t, err)

	// Delete col1
	updateData := types.NewBigtableWriteMutation(keyspace, tableName, "", types.IfSpec{}, types.QueryTypeUpdate, key)
	updateData.AddMutations(types.NewDeleteColumnOp(types.BigtableColumn{Family: "cf1", Column: "col1"}))
	_, err = btc.UpdateRow(t.Context(), updateData)
	require.NoError(t, err)

	// Verify deletion by reading the row
	tbl, err := btc.clients.GetTableClient(keyspace, tableName)
	require.NoError(t, err)
	row, err := tbl.ReadRow(t.Context(), key, bigtable.RowFilter(bigtable.FamilyFilter("cf1")))
	require.NoError(t, err)
	var columns []string
	for _, item := range row["cf1"] {
		columns = append(columns, item.Column)
	}
	// 'cf1:' is an empty cell that we write to ensure the row isn't empty
	assert.ElementsMatch(t, []string{"cf1:col2", "cf1:"}, columns, "col2 should still exist")
}

func TestMutateRowIfExists(t *testing.T) {
	var err error
	const key1 = "if-exists-row-1"
	const key2 = "if-exists-row-2"

	// Insert initial data
	insertData := types.NewBigtableWriteMutation(keyspace, tableName, "", types.IfSpec{}, types.QueryTypeInsert, key1)
	insertData.AddMutations(types.NewWriteCellOp("cf1", "col1", []byte("v1")))
	_, err = btc.InsertRow(t.Context(), insertData)
	require.NoError(t, err)

	// Update the row when it exists
	updateData := types.NewBigtableWriteMutation(keyspace, tableName, "", types.IfSpec{IfExists: true}, types.QueryTypeUpdate, key1)
	updateData.AddMutations(types.NewWriteCellOp("cf1", "col1", []byte("v2")))
	res, err := btc.UpdateRow(t.Context(), updateData)
	require.NoError(t, err)
	assert.True(t, wasApplied(res))

	// Verify the update by reading the row
	tbl, err := btc.clients.GetTableClient(keyspace, tableName)
	require.NoError(t, err)

	row, err := tbl.ReadRow(t.Context(), key1, bigtable.RowFilter(bigtable.FamilyFilter("cf1")))
	require.NoError(t, err)
	assert.Equal(t, []byte(nil), row["cf1"][0].Value) // empty row cell
	assert.Equal(t, "v2", string(row["cf1"][1].Value), "value should be updated to v2")

	// Attempt to update a non-existent row
	updateDataNonExistent := types.NewBigtableWriteMutation(keyspace, tableName, "", types.IfSpec{IfExists: true}, types.QueryTypeUpdate, key2)
	updateDataNonExistent.AddMutations(types.NewWriteCellOp("cf1", "col1", []byte("v2")))
	res, err = btc.UpdateRow(t.Context(), updateDataNonExistent)
	require.NoError(t, err)
	assert.False(t, wasApplied(res))

	// Verify the non-existent row is not created
	row, err = tbl.ReadRow(t.Context(), key2)
	require.NoError(t, err)
	assert.Nil(t, row, "row2 should not exist")
}

func TestMutateRowIfNotExists(t *testing.T) {
	const key = "if-not-exists-row"
	// Insert a row when it does not exist
	insertData := types.NewBigtableWriteMutation(keyspace, tableName, "", types.IfSpec{IfNotExists: true}, types.QueryTypeInsert, key)
	insertData.AddMutations(types.NewWriteCellOp("cf1", "col1", []byte("v1")))
	res, err := btc.InsertRow(t.Context(), insertData)
	require.NoError(t, err)
	assert.True(t, wasApplied(res))

	// Verify the row is created
	tbl, err := btc.clients.GetTableClient(keyspace, tableName)
	require.NoError(t, err)
	row, err := tbl.ReadRow(t.Context(), key, bigtable.RowFilter(bigtable.FamilyFilter("cf1")))
	require.NoError(t, err)
	assert.Equal(t, []byte(nil), row["cf1"][0].Value) // empty row cell
	assert.Equal(t, "v1", string(row["cf1"][1].Value), "row1 should be created with value v1")

	// Attempt to insert the same row again
	res, err = btc.InsertRow(t.Context(), insertData)
	require.NoError(t, err)
	assert.False(t, wasApplied(res))

	// Verify the row is not updated
	row, err = tbl.ReadRow(t.Context(), key, bigtable.RowFilter(bigtable.FamilyFilter("cf1")))
	require.NoError(t, err)
	assert.Equal(t, []byte(nil), row["cf1"][0].Value) // empty row cell
	assert.Equal(t, "v1", string(row["cf1"][1].Value), "row1 should not be updated")
}

func TestMutateRowInvalidKeyspace(t *testing.T) {
	localMdStore := schemaMapping.NewMetadataStore(zap.NewNop(), bts.Clients(), &bigtableConfig)
	localBtc := NewBigtableClient(bts.Clients(), zap.NewNop(), &bigtableConfig, localMdStore)

	updateData := types.NewBigtableWriteMutation("invalid-keyspace", "any-table", "", types.IfSpec{}, types.QueryTypeUpdate, "row1")
	updateData.AddMutations(types.NewWriteCellOp("cf1", "col1", []byte("value")))
	_, err := localBtc.UpdateRow(t.Context(), updateData)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bigtable client not found for keyspace 'invalid-keyspace'")
}

func wasApplied(result message.Message) bool {
	rowsResult, ok := result.(*message.RowsResult)
	if !ok {
		return false
	}
	if len(rowsResult.Data) == 0 || len(rowsResult.Data[0]) == 0 {
		return false
	}
	// The CQL boolean type is represented as a single byte, 0 for false, 1 for true.
	return rowsResult.Data[0][0][0] == 0x01
}
