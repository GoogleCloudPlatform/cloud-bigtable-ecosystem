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
	"errors"
	"fmt"
	"os"
	"slices"
	"testing"
	"time"

	"cloud.google.com/go/bigtable"
	"cloud.google.com/go/bigtable/admin/apiv2/adminpb"
	"cloud.google.com/go/bigtable/bttest"
	types "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	schemaMapping "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/schema-mapping"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/translator"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/utilities"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

var conn *grpc.ClientConn
var lastCreateTableReq *adminpb.CreateTableRequest

var bigtableConfig = &types.BigtableConfig{
	ProjectID:          "my-project",
	Instances:          map[string]*types.InstancesMapping{"ks1": {BigtableInstance: "bt1", Keyspace: "ks1", AppProfileID: "default"}},
	SchemaMappingTable: "schema_mapping",
	Session: &types.Session{
		GrpcChannels: 3,
	},
	DefaultColumnFamily:      "cf1",
	DefaultIntRowKeyEncoding: types.OrderedCodeEncoding,
}

func setupServer() *bttest.Server {
	btt, err := bttest.NewServer("localhost:0")
	if err != nil {
		fmt.Printf("Failed to setup server: %v", err)
		os.Exit(1)
	}
	conn, err = grpc.NewClient(btt.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithUnaryInterceptor(interceptCreateTableRequests))
	if err != nil {
		fmt.Printf("Failed to setup grpc: %v", err)
		os.Exit(1)
	}
	return btt

}

// interceptCreateTableRequests intercepts create table requests, so we can perform extra assertions on them
func interceptCreateTableRequests(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	if method == "/google.bigtable.admin.v2.BigtableTableAdmin/CreateTable" {
		ctr, ok := req.(*adminpb.CreateTableRequest)
		if ok {
			lastCreateTableReq = ctr
		} else {
			lastCreateTableReq = nil
			return errors.New("unary interceptor: unexpected request type for CreateTable")
		}
	}

	err := invoker(ctx, method, req, reply, cc, opts...) // Invoke the actual RPC

	// Post-RPC logic (e.g., error handling, metrics)
	if err != nil {
		fmt.Printf("Unary call to %s failed: %v\n", method, err)
	}
	return err
}

func getClient(conn *grpc.ClientConn) (map[string]*bigtable.Client, map[string]*bigtable.AdminClient, context.Context, error) {
	ctx := context.Background()
	client, err := bigtable.NewClient(ctx, "project", "bt1", option.WithGRPCConn(conn))
	if err != nil {
		fmt.Printf("Failed to create Bigtable client: %v", err)
		return nil, nil, nil, err
	}
	adminClient, err := bigtable.NewAdminClient(ctx, "project", "bt1", option.WithGRPCConn(conn))
	if err != nil {
		fmt.Printf("Failed to create Bigtable admin client: %v", err)
	}
	return map[string]*bigtable.Client{"bt1": client}, map[string]*bigtable.AdminClient{"bt1": adminClient}, ctx, nil
}

func TestMain(m *testing.M) {
	btt := setupServer()
	defer btt.Close()
	os.Exit(m.Run())
}

func TestInsertRow(t *testing.T) {
	client, adminClients, ctx, err := getClient(conn)
	if err != nil {
		t.Fatalf("Failed to create Bigtable client: %v", err)
	}

	adminClient := adminClients["bt1"]

	// Create table
	err = adminClient.CreateTable(ctx, "test-table-insert")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	err = adminClient.CreateColumnFamily(ctx, "test-table-insert", "cf1")
	if err != nil {
		t.Fatalf("Failed to create column family: %v", err)
	}
	btc := NewBigtableClient(client, adminClients, zap.NewNop(), bigtableConfig, nil, schemaMapping.NewSchemaMappingConfig("schema_mapping", "cf1", zap.NewNop(), nil))

	tests := []struct {
		name          string
		data          *translator.InsertQueryMapping
		expectedError error
		expectedValue *message.RowsResult
	}{
		{
			name: "Insert new row",
			data: &translator.InsertQueryMapping{
				Table:    "test-table-insert",
				RowKey:   "row1",
				Columns:  []types.Column{{ColumnFamily: "cf1", Name: "col1"}},
				Values:   []interface{}{[]byte("value1")},
				Keyspace: "ks1",
			},
			expectedError: nil,
		},
		{
			name: "Insert row with IfNotExists where row doesn't exist",
			data: &translator.InsertQueryMapping{
				Table:       "test-table-insert",
				RowKey:      "row2",
				Columns:     []types.Column{{ColumnFamily: "cf1", Name: "col1"}},
				Values:      []interface{}{[]byte("value2")},
				Keyspace:    "ks1",
				IfNotExists: true,
			},
			expectedError: nil,
		},
		{
			name: "Insert row with IfNotExists where row exists",
			data: &translator.InsertQueryMapping{
				Table:       "test-table-insert",
				RowKey:      "row1",
				Columns:     []types.Column{{ColumnFamily: "cf1", Name: "col1"}},
				Values:      []interface{}{[]byte("value1")},
				Keyspace:    "ks1",
				IfNotExists: true,
			},
			expectedError: nil,
		},
		{
			name: "Insert with invalid keyspace",
			data: &translator.InsertQueryMapping{
				Table:    "test-table-insert",
				RowKey:   "row3",
				Columns:  []types.Column{{ColumnFamily: "cf1", Name: "col1"}},
				Values:   []interface{}{[]byte("value3")},
				Keyspace: "invalid-keyspace",
			},
			expectedError: fmt.Errorf("keyspace not found: 'invalid-keyspace'"),
		},
		{
			name: "Delete an entire column family",
			data: &translator.InsertQueryMapping{
				Table:                "test-table-insert",
				RowKey:               "row3",
				DeleteColumnFamilies: []string{"cf1"},
				Columns:              []types.Column{{ColumnFamily: "cf1", Name: "col1"}},
				Values:               []interface{}{[]byte("value3")},
				Keyspace:             "ks1",
			},
			expectedError: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := btc.InsertRow(context.Background(), tt.data)
			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUpdateRow(t *testing.T) {
	client, adminClients, ctx, err := getClient(conn)
	if err != nil {
		t.Fatalf("Failed to create Bigtable client: %v", err)
	}

	adminClient := adminClients["bt1"]

	// Create table
	err = adminClient.CreateTable(ctx, "test-table-update")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	err = adminClient.CreateColumnFamily(ctx, "test-table-update", "cf1")
	if err != nil {
		t.Fatalf("Failed to create column family: %v", err)
	}

	btc := NewBigtableClient(client, adminClients, zap.NewNop(), bigtableConfig, nil, schemaMapping.NewSchemaMappingConfig("schema_mapping", "cf1", zap.NewNop(), nil))

	// Insert initial row
	initialData := &translator.InsertQueryMapping{
		Table:                "test-table-update",
		RowKey:               "test-row",
		Columns:              []types.Column{{ColumnFamily: "cf1", Name: "col1"}},
		Values:               []interface{}{[]byte("initial value")},
		DeleteColumnFamilies: []string{},
		Keyspace:             "ks1",
	}
	_, err = btc.InsertRow(ctx, initialData)
	assert.NoError(t, err)

	// Update the row
	updateData := &translator.UpdateQueryMapping{
		Table:                "test-table-update",
		RowKey:               "test-row",
		Columns:              []types.Column{{ColumnFamily: "cf1", Name: "col1"}},
		Values:               []interface{}{[]byte("updated value")},
		DeleteColumnFamilies: []string{},
		Keyspace:             "ks1",
	}
	_, err = btc.UpdateRow(ctx, updateData)
	assert.NoError(t, err)

	tbl := client["bt1"].Open("test-table-update")
	row, err := tbl.ReadRow(ctx, "test-row", bigtable.RowFilter(bigtable.FamilyFilter("cf1")))
	assert.NoError(t, err)
	assert.Equal(t, "updated value", string(row["cf1"][0].Value))
}

func TestDeleteRow(t *testing.T) {
	client, adminClients, ctx, err := getClient(conn)
	if err != nil {
		t.Fatalf("Failed to create Bigtable client: %v", err)
	}

	adminClient := adminClients["bt1"]

	// Create table
	err = adminClient.CreateTable(ctx, "test-table-delete")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	err = adminClient.CreateColumnFamily(ctx, "test-table-delete", "cf1")
	if err != nil {
		t.Fatalf("Failed to create column family: %v", err)
	}

	btc := NewBigtableClient(client, adminClients, zap.NewNop(), bigtableConfig, nil, &schemaMapping.SchemaMappingConfig{})

	// Insert initial row
	initialData := &translator.InsertQueryMapping{
		Table:                "test-table-delete",
		RowKey:               "test-row",
		Columns:              []types.Column{{ColumnFamily: "cf1", Name: "col1"}},
		Values:               []interface{}{[]byte("initial value")},
		DeleteColumnFamilies: []string{},
		Keyspace:             "ks1",
	}
	_, err = btc.InsertRow(ctx, initialData)
	assert.NoError(t, err)

	// Delete the row
	deleteData := &translator.DeleteQueryMapping{
		Table:    "test-table-delete",
		RowKey:   "test-row",
		Keyspace: "ks1",
	}
	_, err = btc.DeleteRowNew(ctx, deleteData)
	assert.NoError(t, err)

	// Verify deletion
	tbl := client["bt1"].Open("test-table-delete")
	row, err := tbl.ReadRow(ctx, "test-row")
	assert.NoError(t, err)
	assert.Empty(t, row)
}

func TestApplyBulkMutation(t *testing.T) {
	client, adminClients, ctx, err := getClient(conn)
	if err != nil {
		t.Fatalf("Failed to create Bigtable client: %v", err)
	}

	adminClient, err := bigtable.NewAdminClient(ctx, "project", "bt1", option.WithGRPCConn(conn))
	if err != nil {
		t.Fatalf("Failed to create Bigtable admin client: %v", err)
	}

	// Create table
	err = adminClient.CreateTable(ctx, "test-table-bulk-mutation")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	err = adminClient.CreateColumnFamily(ctx, "test-table-bulk-mutation", "cf1")
	if err != nil {
		t.Fatalf("Failed to create column family: %v", err)
	}

	btc := NewBigtableClient(client, adminClients, zap.NewNop(), bigtableConfig, nil, &schemaMapping.SchemaMappingConfig{})

	// Prepare bulk mutation data
	mutationData := []MutationData{
		{
			RowKey:       "test-row1",
			MutationType: "Insert",
			Columns: []ColumnData{
				{ColumnFamily: "cf1", Name: "col1", Contents: []byte("value1")},
			},
		},
		{
			RowKey:       "test-row2",
			MutationType: "Insert",
			Columns: []ColumnData{
				{ColumnFamily: "cf1", Name: "col1", Contents: []byte("value2")},
			},
		},
		{
			RowKey:       "test-row1",
			MutationType: "Update",
			Columns: []ColumnData{
				{ColumnFamily: "cf1", Name: "col1", Contents: []byte("updated-value1")},
			},
		},
		{
			RowKey:       "test-row2",
			MutationType: "Delete",
		},
	}

	// Apply bulk mutation
	resp, err := btc.ApplyBulkMutation(ctx, "test-table-bulk-mutation", mutationData, "ks1")
	assert.NoError(t, err)
	assert.Empty(t, resp.FailedRows)

	// Verify mutations
	tbl := client["bt1"].Open("test-table-bulk-mutation")
	row1, err := tbl.ReadRow(ctx, "test-row1")
	assert.NoError(t, err)
	assert.NotEmpty(t, row1)
	assert.Equal(t, []byte("updated-value1"), row1["cf1"][0].Value)

	row2, err := tbl.ReadRow(ctx, "test-row2")
	assert.NoError(t, err)
	assert.Empty(t, row2)
}

func TestDeleteRowsUsingTimestamp(t *testing.T) {
	client, adminClients, ctx, err := getClient(conn)
	if err != nil {
		t.Fatalf("Failed to create Bigtable client: %v", err)
	}

	adminClient, err := bigtable.NewAdminClient(ctx, "project", "bt1", option.WithGRPCConn(conn))
	if err != nil {
		t.Fatalf("Failed to create Bigtable admin client: %v", err)
	}

	btc := NewBigtableClient(client, adminClients, zap.NewNop(), bigtableConfig, nil, &schemaMapping.SchemaMappingConfig{})

	// Define test table and data
	tableName := "test-table-delete-timestamp"
	columnFamily := "cf1"
	rowKey := "test-row"
	// columns := []string{"col1", "col2"}
	timestamp := translator.TimestampInfo{Timestamp: bigtable.Now()}

	err = adminClient.CreateTable(ctx, tableName)
	assert.NoError(t, err)
	err = adminClient.CreateColumnFamily(ctx, tableName, columnFamily)
	assert.NoError(t, err)

	tbl := client["bt1"].Open(tableName)
	mut := bigtable.NewMutation()
	mut.Set(columnFamily, "col1", bigtable.Now(), []byte("value1"))
	mut.Set(columnFamily, "col2", bigtable.Now(), []byte("value2"))
	err = tbl.Apply(ctx, rowKey, mut)
	assert.NoError(t, err)

	// Verify that the row is present before deletion
	row, err := tbl.ReadRow(ctx, rowKey)
	assert.NoError(t, err)
	assert.NotEmpty(t, row[columnFamily], "Expected columns to be present before deletion")
	// Test Case 1: Successful deletion of columns using a timestamp
	timestamp.Timestamp = bigtable.Timestamp(time.Now().Day())
	deleteData := &translator.DeleteQueryMapping{
		Table:         tableName,
		RowKey:        rowKey,
		Keyspace:      "ks1",
		TimestampInfo: timestamp,
	}
	_, err = btc.DeleteRowNew(ctx, deleteData)
	assert.NoError(t, err)

	// Verify that the columns are deleted
	row, err = tbl.ReadRow(ctx, rowKey)
	assert.NoError(t, err)
	assert.Empty(t, row[columnFamily], "Expected columns to be deleted")

	// Test Case 2: Invalid keyspace
	deleteData.TimestampInfo.Timestamp = bigtable.Now()
	deleteData.Keyspace = "invalid-keyspace"
	_, err = btc.DeleteRowNew(ctx, deleteData)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "keyspace not found: 'invalid-keyspace'")

	// Test Case 3: Attempt to delete non-existent columns (should not error)
	deleteData.Keyspace = "ks1"
	_, err = btc.DeleteRowNew(ctx, deleteData)
	assert.NoError(t, err)

	// Verify that nothing breaks or changes for non-existent columns
	row, err = tbl.ReadRow(ctx, rowKey)
	assert.NoError(t, err)
	assert.Empty(t, row[columnFamily], "Row should remain empty as no valid columns existed to delete")
}

func TestMutateRowDeleteColumnFamily(t *testing.T) {
	client, adminClients, ctx, err := getClient(conn)
	if err != nil {
		t.Fatalf("Failed to create Bigtable client: %v", err)
	}

	adminClient, err := bigtable.NewAdminClient(ctx, "project", "bt1", option.WithGRPCConn(conn))
	if err != nil {
		t.Fatalf("Failed to create Bigtable admin client: %v", err)
	}

	tableName := "test-table-delete-cf"
	require.NoError(t, adminClient.CreateTable(ctx, tableName))
	require.NoError(t, adminClient.CreateColumnFamily(ctx, tableName, "cf1"))
	require.NoError(t, adminClient.CreateColumnFamily(ctx, tableName, "cf2"))

	btc := NewBigtableClient(client, adminClients, zap.NewNop(), bigtableConfig, nil, schemaMapping.NewSchemaMappingConfig("schema_mapping", "cf1", zap.NewNop(), nil))

	// Insert initial data
	insertData := &translator.InsertQueryMapping{
		Table:    tableName,
		RowKey:   "row1",
		Columns:  []types.Column{{ColumnFamily: "cf1", Name: "col1"}, {ColumnFamily: "cf2", Name: "col2"}},
		Values:   []interface{}{[]byte("v1"), []byte("v2")},
		Keyspace: "ks1",
	}
	_, err = btc.InsertRow(ctx, insertData)
	require.NoError(t, err)

	// Delete cf2
	updateData := &translator.UpdateQueryMapping{
		Table:                tableName,
		RowKey:               "row1",
		DeleteColumnFamilies: []string{"cf2"},
		Keyspace:             "ks1",
	}
	_, err = btc.UpdateRow(ctx, updateData)
	require.NoError(t, err)

	// Verify deletion by reading the row
	tbl := client["bt1"].Open(tableName)
	row, err := tbl.ReadRow(ctx, "row1")
	require.NoError(t, err)
	assert.NotContains(t, row, "cf2", "cf2 should be deleted")
	assert.Contains(t, row, "cf1", "cf1 should still exist")
}

func TestMutateRowDeleteQualifiers(t *testing.T) {
	client, adminClients, ctx, err := getClient(conn)
	require.NoError(t, err)

	adminClient, err := bigtable.NewAdminClient(ctx, "project", "bt1", option.WithGRPCConn(conn))
	require.NoError(t, err)

	tableName := "test-table-delete-qualifiers"
	require.NoError(t, adminClient.CreateTable(ctx, tableName))
	require.NoError(t, adminClient.CreateColumnFamily(ctx, tableName, "cf1"))

	btc := NewBigtableClient(client, adminClients, zap.NewNop(), bigtableConfig, nil, schemaMapping.NewSchemaMappingConfig("schema_mapping", "cf1", zap.NewNop(), nil))

	// Insert initial data with two columns
	insertData := &translator.InsertQueryMapping{
		Table:    tableName,
		RowKey:   "row1",
		Columns:  []types.Column{{ColumnFamily: "cf1", Name: "col1"}, {ColumnFamily: "cf1", Name: "col2"}},
		Values:   []interface{}{[]byte("v1"), []byte("v2")},
		Keyspace: "ks1",
	}
	_, err = btc.InsertRow(ctx, insertData)
	require.NoError(t, err)

	// Delete col1
	updateData := &translator.UpdateQueryMapping{
		Table:                 tableName,
		RowKey:                "row1",
		DeleteColumQualifires: []types.Column{{ColumnFamily: "cf1", Name: "col1"}},
		Keyspace:              "ks1",
	}
	_, err = btc.UpdateRow(ctx, updateData)
	require.NoError(t, err)

	// Verify deletion by reading the row
	tbl := client["bt1"].Open(tableName)
	row, err := tbl.ReadRow(ctx, "row1", bigtable.RowFilter(bigtable.FamilyFilter("cf1")))
	require.NoError(t, err)
	cf := row["cf1"]
	for _, cell := range cf {
		assert.NotEqual(t, "cf1:col1", cell.Column, "col1 should be deleted")
	}
	assert.Equal(t, "cf1:col2", cf[0].Column, "col2 should still exist")
}

func TestMutateRowIfExists(t *testing.T) {
	client, adminClients, ctx, err := getClient(conn)
	require.NoError(t, err)

	adminClient, err := bigtable.NewAdminClient(ctx, "project", "bt1", option.WithGRPCConn(conn))
	require.NoError(t, err)

	tableName := "test-table-if-exists"
	require.NoError(t, adminClient.CreateTable(ctx, tableName))
	require.NoError(t, adminClient.CreateColumnFamily(ctx, tableName, "cf1"))

	btc := NewBigtableClient(client, adminClients, zap.NewNop(), bigtableConfig, nil, schemaMapping.NewSchemaMappingConfig("schema_mapping", "cf1", zap.NewNop(), nil))

	// Insert initial data
	insertData := &translator.InsertQueryMapping{
		Table:    tableName,
		RowKey:   "row1",
		Columns:  []types.Column{{ColumnFamily: "cf1", Name: "col1"}},
		Values:   []interface{}{[]byte("v1")},
		Keyspace: "ks1",
	}
	_, err = btc.InsertRow(ctx, insertData)
	require.NoError(t, err)

	// Update the row when it exists
	updateData := &translator.UpdateQueryMapping{
		Table:    tableName,
		RowKey:   "row1",
		Columns:  []types.Column{{ColumnFamily: "cf1", Name: "col1"}},
		Values:   []interface{}{[]byte("v2")},
		IfExists: true,
		Keyspace: "ks1",
	}
	_, err = btc.UpdateRow(ctx, updateData)
	require.NoError(t, err)

	// Verify the update by reading the row
	tbl := client["bt1"].Open(tableName)
	row, err := tbl.ReadRow(ctx, "row1", bigtable.RowFilter(bigtable.FamilyFilter("cf1")))
	require.NoError(t, err)
	assert.Equal(t, "v2", string(row["cf1"][0].Value), "value should be updated to v2")

	// Attempt to update a non-existent row
	updateData.RowKey = "row2"
	_, err = btc.UpdateRow(ctx, updateData)
	require.NoError(t, err)

	// Verify the non-existent row is not created
	row, err = tbl.ReadRow(ctx, "row2")
	require.NoError(t, err)
	assert.Nil(t, row, "row2 should not exist")
}

func TestMutateRowIfNotExists(t *testing.T) {
	client, adminClients, ctx, err := getClient(conn)
	require.NoError(t, err)

	adminClient, err := bigtable.NewAdminClient(ctx, "project", "bt1", option.WithGRPCConn(conn))
	require.NoError(t, err)

	tableName := "test-table-if-not-exists"
	require.NoError(t, adminClient.CreateTable(ctx, tableName))
	require.NoError(t, adminClient.CreateColumnFamily(ctx, tableName, "cf1"))

	btc := NewBigtableClient(client, adminClients, zap.NewNop(), bigtableConfig, nil, schemaMapping.NewSchemaMappingConfig("schema_mapping", "cf1", zap.NewNop(), nil))

	// Insert a row when it does not exist
	InsertData := &translator.InsertQueryMapping{
		Table:       tableName,
		RowKey:      "row1",
		Columns:     []types.Column{{ColumnFamily: "cf1", Name: "col1"}},
		Values:      []interface{}{[]byte("v1")},
		IfNotExists: true,
		Keyspace:    "ks1",
	}
	_, err = btc.InsertRow(ctx, InsertData)
	require.NoError(t, err)

	// Verify the row is created
	tbl := client["bt1"].Open(tableName)
	row, err := tbl.ReadRow(ctx, "row1", bigtable.RowFilter(bigtable.FamilyFilter("cf1")))
	require.NoError(t, err)
	assert.Equal(t, "v1", string(row["cf1"][0].Value), "row1 should be created with value v1")

	// Attempt to insert the same row again
	_, err = btc.InsertRow(ctx, InsertData)
	require.NoError(t, err)

	// Verify the row is not updated
	row, err = tbl.ReadRow(ctx, "row1", bigtable.RowFilter(bigtable.FamilyFilter("cf1")))
	require.NoError(t, err)
	assert.Equal(t, "v1", string(row["cf1"][0].Value), "row1 should not be updated")
}

func TestMutateRowNonByteValue(t *testing.T) {
	client, adminClients, ctx, err := getClient(conn)
	require.NoError(t, err)

	adminClient, err := bigtable.NewAdminClient(ctx, "project", "bt1", option.WithGRPCConn(conn))
	require.NoError(t, err)

	tableName := "test-table-non-byte"
	require.NoError(t, adminClient.CreateTable(ctx, tableName))
	require.NoError(t, adminClient.CreateColumnFamily(ctx, tableName, "cf1"))

	btc := NewBigtableClient(client, adminClients, zap.NewNop(), bigtableConfig, nil, schemaMapping.NewSchemaMappingConfig("schema_mapping", "cf1", zap.NewNop(), nil))

	updateData := &translator.UpdateQueryMapping{
		Table:    tableName,
		RowKey:   "row1",
		Columns:  []types.Column{{ColumnFamily: "cf1", Name: "col1"}},
		Values:   []interface{}{"invalid-value"}, // string instead of []byte
		Keyspace: "ks1",
	}
	_, err = btc.UpdateRow(ctx, updateData)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not of type []byte")
}

func TestMutateRowInvalidKeyspace(t *testing.T) {
	client, adminClients, ctx, err := getClient(conn)
	require.NoError(t, err)

	btc := NewBigtableClient(client, adminClients, zap.NewNop(), bigtableConfig, nil, schemaMapping.NewSchemaMappingConfig("schema_mapping", "cf1", zap.NewNop(), nil))

	updateData := &translator.UpdateQueryMapping{
		Table:    "any-table",
		RowKey:   "row1",
		Columns:  []types.Column{{ColumnFamily: "cf1", Name: "col1"}},
		Values:   []interface{}{[]byte("value")},
		Keyspace: "invalid-keyspace",
	}
	_, err = btc.UpdateRow(ctx, updateData)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "keyspace not found: 'invalid-keyspace'")
}

func TestComplexUpdateWithListIndex(t *testing.T) {
	client, adminClients, ctx, err := getClient(conn)
	require.NoError(t, err)

	adminClient, err := bigtable.NewAdminClient(ctx, "project", "bt1", option.WithGRPCConn(conn))
	require.NoError(t, err)

	tableName := "test-table-complex-update-list-index"
	require.NoError(t, adminClient.CreateTable(ctx, tableName))
	require.NoError(t, adminClient.CreateColumnFamily(ctx, tableName, "cf1"))
	require.NoError(t, adminClient.CreateColumnFamily(ctx, tableName, "list"))

	btc := NewBigtableClient(client, adminClients, zap.NewNop(), bigtableConfig, nil, schemaMapping.NewSchemaMappingConfig("schema_mapping", "cf1", zap.NewNop(), nil))

	// Insert initial data with a list in cf1
	insertData := &translator.InsertQueryMapping{
		Table:    tableName,
		RowKey:   "row1",
		Columns:  []types.Column{{ColumnFamily: "list", Name: "timestamp1"}, {ColumnFamily: "list", Name: "timestamp2"}},
		Values:   []interface{}{[]byte("v1"), []byte("v2")},
		Keyspace: "ks1",
	}
	_, err = btc.InsertRow(ctx, insertData)
	require.NoError(t, err)

	// Perform a complex update with list index
	ComplexOperation := map[string]*translator.ComplexOperation{
		"list": {
			UpdateListIndex: "1", // Update the second item in the list
			Value:           []byte("updated-v2"),
		},
	}
	updateData := &translator.UpdateQueryMapping{
		Table:            tableName,
		RowKey:           "row1",
		ComplexOperation: ComplexOperation,
		Keyspace:         "ks1",
		Columns:          []types.Column{},
		Values:           []interface{}{},
	}
	_, err = btc.UpdateRow(ctx, updateData)
	require.NoError(t, err)

	// Verify the update by reading the row
	tbl := client["bt1"].Open(tableName)
	row, err := tbl.ReadRow(ctx, "row1", bigtable.RowFilter(bigtable.FamilyFilter("list")))
	require.NoError(t, err)
	cf := row["list"]
	assert.Equal(t, "updated-v2", string(cf[1].Value), "second item in the list should be updated")
}

func TestComplexUpdateWithListDeletion(t *testing.T) {
	client, adminClients, ctx, err := getClient(conn)
	require.NoError(t, err)

	adminClient, err := bigtable.NewAdminClient(ctx, "project", "bt1", option.WithGRPCConn(conn))
	require.NoError(t, err)

	tableName := "test-table-complex-update-list-delete"
	require.NoError(t, adminClient.CreateTable(ctx, tableName))
	require.NoError(t, adminClient.CreateColumnFamily(ctx, tableName, "cf1"))
	require.NoError(t, adminClient.CreateColumnFamily(ctx, tableName, "list"))

	btc := NewBigtableClient(client, adminClients, zap.NewNop(), bigtableConfig, nil, schemaMapping.NewSchemaMappingConfig("schema_mapping", "cf1", zap.NewNop(), nil))

	// Insert initial data with a list in the "list" column family
	insertData := &translator.InsertQueryMapping{
		Table:    tableName,
		RowKey:   "row1",
		Columns:  []types.Column{{ColumnFamily: "list", Name: "timestamp1"}, {ColumnFamily: "list", Name: "timestamp2"}},
		Values:   []interface{}{[]byte("v1"), []byte("v2")},
		Keyspace: "ks1",
	}
	_, err = btc.InsertRow(ctx, insertData)
	require.NoError(t, err)

	// Perform a complex update with list deletion
	ComplexOperation := map[string]*translator.ComplexOperation{
		"list": {
			ListDelete:       true,
			ListDeleteValues: [][]byte{[]byte("v1")}, // Delete the first item in the list
		},
	}
	updateData := &translator.UpdateQueryMapping{
		Table:            tableName,
		RowKey:           "row1",
		ComplexOperation: ComplexOperation,
		Keyspace:         "ks1",
		Columns:          []types.Column{},
		Values:           []interface{}{},
	}
	_, err = btc.UpdateRow(ctx, updateData)
	require.NoError(t, err)

	// Verify the deletion by reading the row
	tbl := client["bt1"].Open(tableName)
	row, err := tbl.ReadRow(ctx, "row1", bigtable.RowFilter(bigtable.FamilyFilter("list")))
	require.NoError(t, err)
	cf := row["list"]
	assert.Equal(t, 1, len(cf), "one item should remain in the list")
	assert.Equal(t, "v2", string(cf[0].Value), "remaining item should be v2")
}

func TestComplexUpdateInvalidKeyspace(t *testing.T) {
	client, adminClients, ctx, err := getClient(conn)
	require.NoError(t, err)

	btc := NewBigtableClient(client, adminClients, zap.NewNop(), bigtableConfig, nil, schemaMapping.NewSchemaMappingConfig("schema_mapping", "cf1", zap.NewNop(), nil))

	// Attempt to perform a complex update with an invalid keyspace
	ComplexOperation := map[string]*translator.ComplexOperation{
		"list": {
			UpdateListIndex: "0",
			Value:           []byte("updated-v1"),
		},
	}
	updateData := &translator.UpdateQueryMapping{
		Table:            "any-table",
		RowKey:           "row1",
		ComplexOperation: ComplexOperation,
		Keyspace:         "invalid-keyspace",
		Columns:          []types.Column{},
		Values:           []interface{}{},
	}
	_, err = btc.UpdateRow(ctx, updateData)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "keyspace not found: 'invalid-keyspace'")
}

func TestComplexUpdateOutOfBoundsIndex(t *testing.T) {
	client, adminClients, ctx, err := getClient(conn)
	require.NoError(t, err)

	adminClient := adminClients["bt1"]

	tableName := "test-table-complex-update-out-of-bounds"
	require.NoError(t, adminClient.CreateTable(ctx, tableName))
	require.NoError(t, adminClient.CreateColumnFamily(ctx, tableName, "cf1"))
	require.NoError(t, adminClient.CreateColumnFamily(ctx, tableName, "list"))

	btc := NewBigtableClient(client, adminClients, zap.NewNop(), bigtableConfig, nil, schemaMapping.NewSchemaMappingConfig("schema_mapping", "cf1", zap.NewNop(), nil))

	// Insert initial data with a list in the "list" column family
	insertData := &translator.InsertQueryMapping{
		Table:    tableName,
		RowKey:   "row1",
		Columns:  []types.Column{{ColumnFamily: "list", Name: "timestamp1"}},
		Values:   []interface{}{[]byte("v1")},
		Keyspace: "ks1",
	}
	_, err = btc.InsertRow(ctx, insertData)
	require.NoError(t, err)

	// Attempt to perform a complex update with an out-of-bounds index
	ComplexOperation := map[string]*translator.ComplexOperation{
		"list": {
			UpdateListIndex: "1", // Index 1 is out of bounds for a list of size 1
			Value:           []byte("updated-v2"),
		},
	}
	updateData := &translator.UpdateQueryMapping{
		Table:            tableName,
		RowKey:           "row1",
		ComplexOperation: ComplexOperation,
		Keyspace:         "ks1",
		Columns:          []types.Column{},
		Values:           []interface{}{},
	}
	_, err = btc.UpdateRow(ctx, updateData)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "index 1 out of bounds")
}

var testCreateTableStatementMap = translator.CreateTableStatementMap{
	QueryType:         "create",
	Keyspace:          "ks1",
	Table:             "create_table_test",
	IfNotExists:       false,
	IntRowKeyEncoding: types.OrderedCodeEncoding,
	Columns: []types.CreateColumn{
		{
			Name:     "org",
			Index:    0,
			TypeInfo: types.NewCqlTypeInfoFromType(datatype.Varchar),
		},
		{
			Name:     "id",
			Index:    1,
			TypeInfo: types.NewCqlTypeInfoFromType(datatype.Bigint),
		},
		{
			Name:     "name",
			Index:    2,
			TypeInfo: types.NewCqlTypeInfoFromType(datatype.Varchar),
		},
		{
			Name:     "zipcode",
			Index:    3,
			TypeInfo: types.NewCqlTypeInfoFromType(datatype.Int),
		},
	},
	PrimaryKeys: []translator.CreateTablePrimaryKeyConfig{
		{
			Name:    "org",
			KeyType: "partition_key",
		},
		{
			Name:    "id",
			KeyType: "clustering",
		},
	},
}

func TestCreateTable(t *testing.T) {
	client, adminClients, ctx, err := getClient(conn)
	require.NoError(t, err)

	btc := NewBigtableClient(client, adminClients, zap.NewNop(), bigtableConfig, nil, schemaMapping.NewSchemaMappingConfig("schema_mapping", "cf1", zap.NewNop(), nil))

	// force set up the schema mappings table
	_, err = btc.ReadTableConfigs(ctx, "ks1")
	require.NoError(t, err)

	// table should not exist yet
	info, err := adminClients["bt1"].TableInfo(ctx, "create_table_test")
	require.Error(t, err)
	require.Equal(t, codes.NotFound, status.Code(err))
	require.Nil(t, info)

	err = btc.CreateTable(ctx, &testCreateTableStatementMap)
	require.NoError(t, err)

	tables, err := btc.ReadTableConfigs(ctx, "ks1")
	require.NoError(t, err)
	tableMap := schemaMapping.CreateTableMap(tables)

	require.Equal(t, map[string]*types.Column{
		"org": {
			Name:         "org",
			TypeInfo:     types.NewCqlTypeInfoFromType(datatype.Varchar),
			IsPrimaryKey: true,
			PkPrecedence: 1,
			KeyType:      "partition_key",
			Metadata: message.ColumnMetadata{
				Keyspace: "ks1",
				Table:    "create_table_test",
				Name:     "org",
				Index:    2,
				Type:     datatype.Varchar,
			},
		},
		"id": {
			Name:         "id",
			TypeInfo:     types.NewCqlTypeInfoFromType(datatype.Bigint),
			IsPrimaryKey: true,
			PkPrecedence: 2,
			KeyType:      "clustering",
			Metadata: message.ColumnMetadata{
				Keyspace: "ks1",
				Table:    "create_table_test",
				Name:     "id",
				Index:    0,
				Type:     datatype.Bigint,
			},
		},
		"name": {
			Name:         "name",
			TypeInfo:     types.NewCqlTypeInfoFromType(datatype.Varchar),
			IsPrimaryKey: false,
			PkPrecedence: 0,
			KeyType:      utilities.KEY_TYPE_REGULAR,
			Metadata: message.ColumnMetadata{
				Keyspace: "ks1",
				Table:    "create_table_test",
				Name:     "name",
				Index:    1,
				Type:     datatype.Varchar,
			},
		},
		"zipcode": {
			Name:         "zipcode",
			TypeInfo:     types.NewCqlTypeInfoFromType(datatype.Int),
			IsPrimaryKey: false,
			PkPrecedence: 0,
			KeyType:      utilities.KEY_TYPE_REGULAR,
			Metadata: message.ColumnMetadata{
				Keyspace: "ks1",
				Table:    "create_table_test",
				Name:     "zipcode",
				Index:    3,
				Type:     datatype.Int,
			},
		},
	}, tableMap["create_table_test"].Columns)
	info, err = adminClients["bt1"].TableInfo(ctx, "create_table_test")
	require.NoError(t, err)
	require.NotNil(t, info)
}

func TestCanLoadBadTableConfig(t *testing.T) {
	client, adminClients, ctx, err := getClient(conn)
	require.NoError(t, err)

	btc := NewBigtableClient(client, adminClients, zap.NewNop(), bigtableConfig, nil, schemaMapping.NewSchemaMappingConfig("schema_mapping", "cf1", zap.NewNop(), nil))
	_, err = btc.ReadTableConfigs(ctx, "ks1")
	require.NoError(t, err)

	sm := client["bt1"].Open("schema_mapping")

	tableName := "bad_key_type_table"
	// WARNING: do NOT change this mutation. It captures a real world schema that we need to handle.
	rks, muts := toMuts(tableName, []map[string][]byte{
		map[string][]byte{
			smColColumnName:   []byte("column1"),
			smColColumnType:   []byte("varchar"),
			smColIsCollection: []byte("false"),
			smColIsPrimaryKey: []byte("true"),
			smColKeyType:      []byte("regular"), // wrong key type but PkPrecedence is > 0
			smColPKPrecedence: []byte("1"),
			smColTableName:    []byte(tableName),
		},
		map[string][]byte{
			smColColumnName:   []byte("column2"),
			smColColumnType:   []byte("float"),
			smColIsCollection: []byte("false"),
			smColIsPrimaryKey: []byte("false"),
			smColKeyType:      []byte(""), // missing key type
			smColPKPrecedence: []byte("0"),
			smColTableName:    []byte(tableName),
		},
		map[string][]byte{
			smColColumnName:   []byte("column3"),
			smColColumnType:   []byte("int"),
			smColIsCollection: []byte("false"),
			smColIsPrimaryKey: []byte("false"),
			smColKeyType:      []byte(""), // missing key type
			smColPKPrecedence: []byte("0"),
			smColTableName:    []byte(tableName),
		},
		map[string][]byte{
			smColColumnName:   []byte("column4"),
			smColColumnType:   []byte("double"),
			smColIsCollection: []byte("false"),
			smColIsPrimaryKey: []byte("false"),
			smColKeyType:      []byte(""), // missing key type
			smColPKPrecedence: []byte("0"),
			smColTableName:    []byte(tableName),
		},
		map[string][]byte{
			smColColumnName:   []byte("column5"),
			smColColumnType:   []byte("double"),
			smColIsCollection: []byte("false"),
			smColIsPrimaryKey: []byte("false"),
			smColKeyType:      []byte(""), // missing key type
			smColPKPrecedence: []byte("0"),
			smColTableName:    []byte(tableName),
		},
	})
	_, err = sm.ApplyBulk(ctx, rks, muts)
	require.NoError(t, err)

	err = adminClients["bt1"].CreateTable(ctx, tableName)
	require.NoError(t, err)

	tableConfigs, err := btc.ReadTableConfigs(ctx, "ks1")
	require.NoError(t, err)

	index := slices.IndexFunc(tableConfigs, func(config *schemaMapping.TableConfig) bool {
		return config.Name == tableName
	})
	require.NotEqual(t, -1, index)
	tc := tableConfigs[index]
	assert.Equal(t, tableName, tc.Name)
	assert.Equal(t, "ks1", tc.Keyspace)
	assert.Equal(t, []string{"column1"}, tc.GetPrimaryKeys())
	pkCol, err := tc.GetColumn("column1")
	require.NoError(t, err)
	assert.Equal(t, "column1", pkCol.Name)
	assert.Equal(t, 1, pkCol.PkPrecedence)
	assert.Equal(t, utilities.KEY_TYPE_PARTITION, pkCol.KeyType)
	assert.Equal(t, true, pkCol.IsPrimaryKey)
}

func toMuts(tableName string, data []map[string][]byte) ([]string, []*bigtable.Mutation) {
	var muts []*bigtable.Mutation
	var rowKeys []string
	ts := bigtable.Now()
	for _, d := range data {
		mut := bigtable.NewMutation()
		for k, v := range d {
			mut.Set(schemaMappingTableColumnFamily, k, ts, v)
		}
		muts = append(muts, mut)
		rowKeys = append(rowKeys, tableName+"#"+string(d[smColColumnName]))
	}
	return rowKeys, muts
}

// note: the bttest instance ignores RowKeySchema, so it will always be nil which breaks ReadTableConfigs() ability to infer key encoding.
func TestCreateTableWithEncodeIntRowKeysWithBigEndianTrue(t *testing.T) {
	client, adminClients, ctx, err := getClient(conn)
	require.NoError(t, err)

	btc := NewBigtableClient(client, adminClients, zap.NewNop(), bigtableConfig, nil, schemaMapping.NewSchemaMappingConfig("schema_mapping", "cf1", zap.NewNop(), nil))

	// force set up the schema mappings table
	_, err = btc.ReadTableConfigs(ctx, "ks1")
	require.NoError(t, err)

	tableName := "big_endian_table"
	createTableStmt := testCreateTableStatementMap
	createTableStmt.Table = tableName
	createTableStmt.IntRowKeyEncoding = types.BigEndianEncoding
	err = btc.CreateTable(ctx, &createTableStmt)
	require.NoError(t, err)

	assert.NotNil(t, lastCreateTableReq)
	assert.Equal(t, tableName, lastCreateTableReq.TableId)
	field := lastCreateTableReq.Table.RowKeySchema.Fields[1]
	assert.Equal(t, "id", field.FieldName)
	assert.Equal(t, "big_endian_bytes:{}", field.Type.GetInt64Type().GetEncoding().String())
}

func TestCreateTableWithEncodeIntRowKeysWithBigEndianFalse(t *testing.T) {
	client, adminClients, ctx, err := getClient(conn)
	require.NoError(t, err)

	btc := NewBigtableClient(client, adminClients, zap.NewNop(), bigtableConfig, nil, schemaMapping.NewSchemaMappingConfig("schema_mapping", "cf1", zap.NewNop(), nil))

	// force set up the schema mappings table
	_, err = btc.ReadTableConfigs(ctx, "ks1")
	require.NoError(t, err)

	tableName := "ordered_bytes_encoded_table"
	createTableStmt := testCreateTableStatementMap
	createTableStmt.Table = tableName
	createTableStmt.IntRowKeyEncoding = types.OrderedCodeEncoding
	err = btc.CreateTable(ctx, &createTableStmt)
	require.NoError(t, err)

	assert.NotNil(t, lastCreateTableReq)
	assert.Equal(t, tableName, lastCreateTableReq.TableId)
	field := lastCreateTableReq.Table.RowKeySchema.Fields[1]
	assert.Equal(t, "id", field.FieldName)
	assert.Equal(t, "ordered_code_bytes:{}", field.Type.GetInt64Type().GetEncoding().String())
}

func TestAlterTable(t *testing.T) {
	client, adminClients, ctx, err := getClient(conn)
	require.NoError(t, err)

	btc := NewBigtableClient(client, adminClients, zap.NewNop(), bigtableConfig, nil, schemaMapping.NewSchemaMappingConfig("schema_mapping", "cf1", zap.NewNop(), nil))

	// force set up the schema mappings table
	_, err = btc.ReadTableConfigs(ctx, "ks1")
	require.NoError(t, err)

	// table should not exist yet
	info, err := adminClients["bt1"].TableInfo(ctx, "alter_table_test")
	require.Error(t, err)
	require.Equal(t, codes.NotFound, status.Code(err))
	require.Nil(t, info)

	createTable := testCreateTableStatementMap
	createTable.Table = "alter_table_test"
	err = btc.CreateTable(ctx, &createTable)
	require.NoError(t, err)

	err = btc.AlterTable(ctx, &translator.AlterTableStatementMap{
		QueryType:   "alter",
		Keyspace:    "ks1",
		Table:       "alter_table_test",
		IfNotExists: false,
		AddColumns: []types.CreateColumn{
			{
				Name:     "zodiac",
				TypeInfo: types.NewCqlTypeInfoFromType(datatype.Varchar),
			},
		},
		DropColumns: []string{
			"zipcode",
		},
	})
	require.NoError(t, err)

	tables, err := btc.ReadTableConfigs(ctx, "ks1")
	require.NoError(t, err)
	tableMap := schemaMapping.CreateTableMap(tables)

	require.Equal(t, tableMap["alter_table_test"].Columns, map[string]*types.Column{
		"org": {
			Name:         "org",
			TypeInfo:     types.NewCqlTypeInfoFromType(datatype.Varchar),
			IsPrimaryKey: true,
			PkPrecedence: 1,
			KeyType:      "partition_key",
			Metadata: message.ColumnMetadata{
				Keyspace: "ks1",
				Table:    "alter_table_test",
				Name:     "org",
				Index:    2,
				Type:     datatype.Varchar,
			},
		},
		"id": {
			Name:         "id",
			TypeInfo:     types.NewCqlTypeInfoFromType(datatype.Bigint),
			IsPrimaryKey: true,
			PkPrecedence: 2,
			KeyType:      "clustering",
			Metadata: message.ColumnMetadata{
				Keyspace: "ks1",
				Table:    "alter_table_test",
				Name:     "id",
				Index:    0,
				Type:     datatype.Bigint,
			},
		},
		"name": {
			Name:         "name",
			TypeInfo:     types.NewCqlTypeInfoFromType(datatype.Varchar),
			IsPrimaryKey: false,
			PkPrecedence: 0,
			KeyType:      utilities.KEY_TYPE_REGULAR,
			Metadata: message.ColumnMetadata{
				Keyspace: "ks1",
				Table:    "alter_table_test",
				Name:     "name",
				Index:    1,
				Type:     datatype.Varchar,
			},
		},
		"zodiac": {
			Name:         "zodiac",
			TypeInfo:     types.NewCqlTypeInfoFromType(datatype.Varchar),
			IsPrimaryKey: false,
			PkPrecedence: 0,
			KeyType:      utilities.KEY_TYPE_REGULAR,
			Metadata: message.ColumnMetadata{
				Keyspace: "ks1",
				Table:    "alter_table_test",
				Name:     "zodiac",
				Index:    3,
				Type:     datatype.Varchar,
			},
		},
	})
}

func TestDropTable(t *testing.T) {
	client, adminClients, ctx, err := getClient(conn)
	require.NoError(t, err)

	btc := NewBigtableClient(client, adminClients, zap.NewNop(), bigtableConfig, nil, schemaMapping.NewSchemaMappingConfig("schema_mapping", "cf1", zap.NewNop(), nil))

	// force set up the schema mappings table
	_, err = btc.ReadTableConfigs(ctx, "ks1")
	require.NoError(t, err)

	// table should not exist yet
	info, err := adminClients["bt1"].TableInfo(ctx, "drop_table_test")
	require.Error(t, err)
	require.Equal(t, codes.NotFound, status.Code(err))
	require.Nil(t, info)

	createTable := testCreateTableStatementMap
	createTable.Table = "drop_table_test"
	err = btc.CreateTable(ctx, &createTable)
	require.NoError(t, err)

	tables, err := btc.ReadTableConfigs(ctx, "ks1")
	require.NoError(t, err)
	tableMap := schemaMapping.CreateTableMap(tables)
	require.NotNil(t, tableMap["drop_table_test"])

	err = btc.DropTable(ctx, &translator.DropTableStatementMap{
		QueryType: "drop",
		Keyspace:  "ks1",
		Table:     "drop_table_test",
		IfExists:  false,
	})
	require.NoError(t, err)

	tables, err = btc.ReadTableConfigs(ctx, "ks1")
	require.NoError(t, err)
	tableMap = schemaMapping.CreateTableMap(tables)
	require.Nil(t, tableMap["drop_table_test"])

	// table should be cleaned up
	info, err = adminClients["bt1"].TableInfo(ctx, "drop_table_test")
	require.Error(t, err)
	require.Equal(t, codes.NotFound, status.Code(err))
	require.Nil(t, info)
}

func TestInferSQLType(t *testing.T) {
	tests := []struct {
		name          string
		input         interface{}
		expectedType  bigtable.SQLType
		expectedError string
	}{
		{
			name:         "string value",
			input:        "test",
			expectedType: bigtable.StringSQLType{},
		},
		{
			name:         "byte slice value",
			input:        []byte("test"),
			expectedType: bigtable.BytesSQLType{},
		},
		{
			name:         "integer value",
			input:        42,
			expectedType: bigtable.Int64SQLType{},
		},
		{
			name:         "int32 value",
			input:        int32(42),
			expectedType: bigtable.Int64SQLType{},
		},
		{
			name:         "int64 value",
			input:        int64(42),
			expectedType: bigtable.Int64SQLType{},
		},
		{
			name:         "float32 value",
			input:        float32(3.14),
			expectedType: bigtable.Float32SQLType{},
		},
		{
			name:         "float64 value",
			input:        float64(3.14),
			expectedType: bigtable.Float64SQLType{},
		},
		{
			name:         "boolean value",
			input:        true,
			expectedType: bigtable.Int64SQLType{},
		},
		{
			name:         "interface slice",
			input:        []interface{}{1, 2, 3},
			expectedType: bigtable.ArraySQLType{},
		},
		{
			name:          "unsupported struct type",
			input:         struct{}{},
			expectedError: "unsupported type for SQL parameter inference: struct {}",
		},
		{
			name:         "empty interface slice",
			input:        []interface{}{},
			expectedType: bigtable.ArraySQLType{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sqlType, err := inferSQLType(tt.input)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError, err.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedType, sqlType)
			}
		})
	}
}
func TestBigtableClient_getClient(t *testing.T) {
	// Setup a dummy bigtable.Client and BigtableClient struct
	dummyClient := &bigtable.Client{}
	clients := map[string]*bigtable.Client{
		"bt1": dummyClient,
	}
	btc := &BigtableClient{
		BigtableConfig: bigtableConfig,
		Clients:        clients,
	}

	t.Run("returns client when keyspace and instance exist", func(t *testing.T) {
		client, err := btc.getClient("ks1")
		assert.NoError(t, err)
		assert.Equal(t, dummyClient, client)
	})

	t.Run("returns error when keyspace does not exist", func(t *testing.T) {
		client, err := btc.getClient("missing")
		assert.Nil(t, client)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "keyspace not found: 'missing'")
	})

	t.Run("returns error when client for instance does not exist", func(t *testing.T) {
		btc2 := &BigtableClient{
			BigtableConfig: bigtableConfig,
			Clients:        map[string]*bigtable.Client{}, // no client for inst2
		}
		client, err := btc2.getClient("ks2")
		assert.Nil(t, client)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "keyspace not found: 'ks2'")
	})
}
func TestBigtableClient_getAdminClient(t *testing.T) {
	// Setup a dummy bigtable.AdminClient and BigtableClient struct
	dummyAdminClient := &bigtable.AdminClient{}
	adminClients := map[string]*bigtable.AdminClient{
		"bt1": dummyAdminClient,
	}
	btc := &BigtableClient{
		BigtableConfig: bigtableConfig,
		AdminClients:   adminClients,
	}

	t.Run("returns admin client when keyspace and instance exist", func(t *testing.T) {
		client, err := btc.getAdminClient("ks1")
		assert.NoError(t, err)
		assert.Equal(t, dummyAdminClient, client)
	})

	t.Run("returns error when keyspace does not exist", func(t *testing.T) {
		client, err := btc.getAdminClient("missing")
		assert.Nil(t, client)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "keyspace not found: 'missing'")
	})

	t.Run("returns error when admin client for instance does not exist", func(t *testing.T) {
		btc2 := &BigtableClient{
			BigtableConfig: bigtableConfig,
			AdminClients:   map[string]*bigtable.AdminClient{}, // no admin client for inst2
		}
		client, err := btc2.getAdminClient("ks2")
		assert.Nil(t, client)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "keyspace not found: 'ks2'")
	})
}
