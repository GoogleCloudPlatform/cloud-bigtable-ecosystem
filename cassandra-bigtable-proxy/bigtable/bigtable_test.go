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
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"cloud.google.com/go/bigtable"
	"cloud.google.com/go/bigtable/bttest"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/message"
	schemaMapping "github.com/ollionorg/cassandra-to-bigtable-proxy/schema-mapping"
	"github.com/ollionorg/cassandra-to-bigtable-proxy/translator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var conn *grpc.ClientConn

func setupServer() *bttest.Server {
	btt, err := bttest.NewServer("localhost:0")
	if err != nil {
		fmt.Printf("Failed to setup server: %v", err)
		os.Exit(1)
	}
	conn, err = grpc.NewClient(btt.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fmt.Printf("Failed to setup grpc: %v", err)
		os.Exit(1)
	}
	return btt

}
func getClient(conn *grpc.ClientConn) (map[string]*bigtable.Client, context.Context, error) {
	ctx := context.Background()
	client, err := bigtable.NewClient(ctx, "project", "instance", option.WithGRPCConn(conn))
	if err != nil {
		fmt.Printf("Failed to create Bigtable client: %v", err)
		return nil, nil, err
	}
	return map[string]*bigtable.Client{"keyspace": client}, ctx, nil
}

func TestMain(m *testing.M) {
	btt := setupServer()
	defer btt.Close()
	os.Exit(m.Run())
}

func TestInsertRow(t *testing.T) {
	client, ctx, err := getClient(conn)
	if err != nil {
		t.Fatalf("Failed to create Bigtable client: %v", err)
	}

	adminClient, err := bigtable.NewAdminClient(ctx, "project", "instance", option.WithGRPCConn(conn))
	if err != nil {
		t.Fatalf("Failed to create Bigtable admin client: %v", err)
	}

	// Create table
	err = adminClient.CreateTable(ctx, "test-table-insert")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	err = adminClient.CreateColumnFamily(ctx, "test-table-insert", "cf1")
	if err != nil {
		t.Fatalf("Failed to create column family: %v", err)
	}
	btc := NewBigtableClient(client, zap.NewNop(), nil, BigtableConfig{}, nil, nil, nil)

	tests := []struct {
		name          string
		data          *translator.InsertQueryMap
		expectedError error
		expectedValue *message.RowsResult
	}{
		{
			name: "Insert new row",
			data: &translator.InsertQueryMap{
				Table:    "test-table-insert",
				RowKey:   "row1",
				Columns:  []translator.Column{{ColumnFamily: "cf1", Name: "col1"}},
				Values:   []interface{}{[]byte("value1")},
				Keyspace: "keyspace",
			},
			expectedError: nil,
		},
		{
			name: "Insert row with IfNotExists where row doesn't exist",
			data: &translator.InsertQueryMap{
				Table:       "test-table-insert",
				RowKey:      "row2",
				Columns:     []translator.Column{{ColumnFamily: "cf1", Name: "col1"}},
				Values:      []interface{}{[]byte("value2")},
				Keyspace:    "keyspace",
				IfNotExists: true,
			},
			expectedError: nil,
		},
		{
			name: "Insert row with IfNotExists where row exists",
			data: &translator.InsertQueryMap{
				Table:       "test-table-insert",
				RowKey:      "row1",
				Columns:     []translator.Column{{ColumnFamily: "cf1", Name: "col1"}},
				Values:      []interface{}{[]byte("value1")},
				Keyspace:    "keyspace",
				IfNotExists: true,
			},
			expectedError: nil,
		},
		{
			name: "Insert with invalid keyspace",
			data: &translator.InsertQueryMap{
				Table:    "test-table-insert",
				RowKey:   "row3",
				Columns:  []translator.Column{{ColumnFamily: "cf1", Name: "col1"}},
				Values:   []interface{}{[]byte("value3")},
				Keyspace: "invalid-keyspace",
			},
			expectedError: fmt.Errorf("invalid keySpace - `invalid-keyspace`"),
		},
		{
			name: "Delete an entire column family",
			data: &translator.InsertQueryMap{
				Table:                "test-table-insert",
				RowKey:               "row3",
				DeleteColumnFamilies: []string{"cf1"},
				Columns:              []translator.Column{{ColumnFamily: "cf1", Name: "col1"}},
				Values:               []interface{}{[]byte("value3")},
				Keyspace:             "keyspace",
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
	client, ctx, err := getClient(conn)
	if err != nil {
		t.Fatalf("Failed to create Bigtable client: %v", err)
	}

	adminClient, err := bigtable.NewAdminClient(ctx, "project", "instance", option.WithGRPCConn(conn))
	if err != nil {
		t.Fatalf("Failed to create Bigtable admin client: %v", err)
	}

	// Create table
	err = adminClient.CreateTable(ctx, "test-table-update")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	err = adminClient.CreateColumnFamily(ctx, "test-table-update", "cf1")
	if err != nil {
		t.Fatalf("Failed to create column family: %v", err)
	}

	bigtableCl := NewBigtableClient(client, zap.NewNop(), nil, BigtableConfig{}, nil, nil, nil)

	// Insert initial row
	initialData := &translator.InsertQueryMap{
		Table:                "test-table-update",
		RowKey:               "test-row",
		Columns:              []translator.Column{{ColumnFamily: "cf1", Name: "col1"}},
		Values:               []interface{}{[]byte("initial value")},
		DeleteColumnFamilies: []string{},
		Keyspace:             "keyspace",
	}
	_, err = bigtableCl.InsertRow(ctx, initialData)
	assert.NoError(t, err)

	// Update the row
	updateData := &translator.UpdateQueryMap{
		Table:                "test-table-update",
		RowKey:               "test-row",
		Columns:              []translator.Column{{ColumnFamily: "cf1", Name: "col1"}},
		Values:               []interface{}{[]byte("updated value")},
		DeleteColumnFamilies: []string{},
		Keyspace:             "keyspace",
	}
	_, err = bigtableCl.UpdateRow(ctx, updateData)
	assert.NoError(t, err)

	tbl := client["keyspace"].Open("test-table-update")
	row, err := tbl.ReadRow(ctx, "test-row", bigtable.RowFilter(bigtable.FamilyFilter("cf1")))
	assert.NoError(t, err)
	assert.Equal(t, "updated value", string(row["cf1"][0].Value))
}

func TestDeleteRow(t *testing.T) {
	client, ctx, err := getClient(conn)
	if err != nil {
		t.Fatalf("Failed to create Bigtable client: %v", err)
	}

	adminClient, err := bigtable.NewAdminClient(ctx, "project", "instance", option.WithGRPCConn(conn))
	if err != nil {
		t.Fatalf("Failed to create Bigtable admin client: %v", err)
	}

	// Create table
	err = adminClient.CreateTable(ctx, "test-table-delete")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	err = adminClient.CreateColumnFamily(ctx, "test-table-delete", "cf1")
	if err != nil {
		t.Fatalf("Failed to create column family: %v", err)
	}

	btc := NewBigtableClient(client, zap.NewNop(), nil, BigtableConfig{}, nil, nil, nil)

	// Insert initial row
	initialData := &translator.InsertQueryMap{
		Table:                "test-table-delete",
		RowKey:               "test-row",
		Columns:              []translator.Column{{ColumnFamily: "cf1", Name: "col1"}},
		Values:               []interface{}{[]byte("initial value")},
		DeleteColumnFamilies: []string{},
		Keyspace:             "keyspace",
	}
	_, err = btc.InsertRow(ctx, initialData)
	assert.NoError(t, err)

	// Delete the row
	deleteData := &translator.DeleteQueryMap{
		Table:    "test-table-delete",
		RowKey:   "test-row",
		Keyspace: "keyspace",
	}
	_, err = btc.DeleteRow(ctx, deleteData)
	assert.NoError(t, err)

	// Verify deletion
	tbl := client["keyspace"].Open("test-table-delete")
	row, err := tbl.ReadRow(ctx, "test-row")
	assert.NoError(t, err)
	assert.Empty(t, row)
}

func TestGetTableConfigs(t *testing.T) {
	client, ctx, err := getClient(conn)
	if err != nil {
		t.Fatalf("Failed to create Bigtable client: %v", err)
	}

	adminClient, err := bigtable.NewAdminClient(ctx, "project", "instance", option.WithGRPCConn(conn))
	if err != nil {
		t.Fatalf("Failed to create Bigtable admin client: %v", err)
	}

	// Create table
	err = adminClient.CreateTable(ctx, "config-table")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	err = adminClient.CreateColumnFamily(ctx, "config-table", "cf")
	if err != nil {
		t.Fatalf("Failed to create column family: %v", err)
	}

	// Insert test data
	tbl := client["keyspace"].Open("config-table")
	mut := bigtable.NewMutation()
	mut.Set("cf", "TableName", bigtable.Now(), []byte("TestTable"))
	mut.Set("cf", "ColumnName", bigtable.Now(), []byte("TestColumn"))
	mut.Set("cf", "ColumnType", bigtable.Now(), []byte("text"))
	mut.Set("cf", "IsPrimaryKey", bigtable.Now(), []byte("true"))
	mut.Set("cf", "PK_Precedence", bigtable.Now(), []byte(json.Number("1").String()))
	mut.Set("cf", "IsCollection", bigtable.Now(), []byte("false"))

	err = tbl.Apply(ctx, "row1", mut)
	assert.NoError(t, err)

	btc := NewBigtableClient(client, zap.NewNop(), nil, BigtableConfig{}, nil, nil, nil)

	tableMetadata, pkMetadata, err := btc.GetSchemaMappingConfigs(ctx, "keyspace", "config-table")

	assert.NoError(t, err)

	var expectedPkMetadata = map[string][]schemaMapping.Column{
		"TestTable": {
			{
				ColumnName:   "TestColumn",
				ColumnType:   "text",
				IsPrimaryKey: true,
				PkPrecedence: 1,
				IsCollection: false,
				Metadata: message.ColumnMetadata{
					Keyspace: "keyspace",
					Table:    "TestTable",
					Name:     "TestColumn",
					Index:    0,
					Type:     datatype.Varchar,
				},
			},
		},
	}

	expectedTableMetaData := map[string]map[string]*schemaMapping.Column{
		"TestTable": {
			"TestColumn": {
				ColumnName:   "TestColumn",
				ColumnType:   "text",
				IsPrimaryKey: true,
				PkPrecedence: 1,
				IsCollection: false,
				Metadata: message.ColumnMetadata{
					Keyspace: "keyspace",
					Table:    "TestTable",
					Name:     "TestColumn",
					Index:    0,
					Type:     datatype.Varchar,
				},
			},
		},
	}

	assert.Equal(t, expectedTableMetaData, tableMetadata)
	assert.Equal(t, expectedPkMetadata, pkMetadata)
}

func TestApplyBulkMutation(t *testing.T) {
	client, ctx, err := getClient(conn)
	if err != nil {
		t.Fatalf("Failed to create Bigtable client: %v", err)
	}

	adminClient, err := bigtable.NewAdminClient(ctx, "project", "instance", option.WithGRPCConn(conn))
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

	btc := NewBigtableClient(client, zap.NewNop(), nil, BigtableConfig{}, nil, nil, nil)

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
	resp, err := btc.ApplyBulkMutation(ctx, "test-table-bulk-mutation", mutationData, "keyspace")
	assert.NoError(t, err)
	assert.Empty(t, resp.FailedRows)

	// Verify mutations
	tbl := client["keyspace"].Open("test-table-bulk-mutation")
	row1, err := tbl.ReadRow(ctx, "test-row1")
	assert.NoError(t, err)
	assert.NotEmpty(t, row1)
	assert.Equal(t, []byte("updated-value1"), row1["cf1"][0].Value)

	row2, err := tbl.ReadRow(ctx, "test-row2")
	assert.NoError(t, err)
	assert.Empty(t, row2)
}
func TestDeleteRowsUsingTimestamp(t *testing.T) {
	client, ctx, err := getClient(conn)
	if err != nil {
		t.Fatalf("Failed to create Bigtable client: %v", err)
	}

	adminClient, err := bigtable.NewAdminClient(ctx, "project", "instance", option.WithGRPCConn(conn))
	if err != nil {
		t.Fatalf("Failed to create Bigtable admin client: %v", err)
	}
	// defer adminClient.Close()

	btc := NewBigtableClient(client, zap.NewNop(), nil, BigtableConfig{}, nil, nil, nil)

	// Define test table and data
	tableName := "test-table-delete-timestamp"
	columnFamily := "cf1"
	rowKey := "test-row"
	columns := []string{"col1", "col2"}
	timestamp := translator.TimestampInfo{Timestamp: bigtable.Now()}

	err = adminClient.CreateTable(ctx, tableName)
	assert.NoError(t, err)
	err = adminClient.CreateColumnFamily(ctx, tableName, columnFamily)
	assert.NoError(t, err)

	tbl := client["keyspace"].Open(tableName)
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
	err = btc.DeleteRowsUsingTimestamp(ctx, tableName, columns, rowKey, columnFamily, timestamp, "keyspace")
	assert.NoError(t, err)

	// Verify that the columns are deleted
	row, err = tbl.ReadRow(ctx, rowKey)
	assert.NoError(t, err)
	assert.Empty(t, row[columnFamily], "Expected columns to be deleted")

	// Test Case 2: Invalid keyspace
	timestamp.Timestamp = bigtable.Now()
	err = btc.DeleteRowsUsingTimestamp(ctx, tableName, columns, rowKey, columnFamily, timestamp, "invalid-keyspace")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid keySpace - `invalid-keyspace`")

	// Test Case 3: Attempt to delete non-existent columns (should not error)
	err = btc.DeleteRowsUsingTimestamp(ctx, tableName, []string{"nonexistent-col"}, rowKey, columnFamily, timestamp, "keyspace")
	assert.NoError(t, err)

	// Verify that nothing breaks or changes for non-existent columns
	row, err = tbl.ReadRow(ctx, rowKey)
	assert.NoError(t, err)
	assert.Empty(t, row[columnFamily], "Row should remain empty as no valid columns existed to delete")
}

func TestMutateRowDeleteColumnFamily(t *testing.T) {
	client, ctx, err := getClient(conn)
	if err != nil {
		t.Fatalf("Failed to create Bigtable client: %v", err)
	}

	adminClient, err := bigtable.NewAdminClient(ctx, "project", "instance", option.WithGRPCConn(conn))
	if err != nil {
		t.Fatalf("Failed to create Bigtable admin client: %v", err)
	}
	// defer adminClient.Close()

	tableName := "test-table-delete-cf"
	require.NoError(t, adminClient.CreateTable(ctx, tableName))
	require.NoError(t, adminClient.CreateColumnFamily(ctx, tableName, "cf1"))
	require.NoError(t, adminClient.CreateColumnFamily(ctx, tableName, "cf2"))

	btClient := NewBigtableClient(client, zap.NewNop(), nil, BigtableConfig{}, nil, nil, nil)

	// Insert initial data
	insertData := &translator.InsertQueryMap{
		Table:    tableName,
		RowKey:   "row1",
		Columns:  []translator.Column{{ColumnFamily: "cf1", Name: "col1"}, {ColumnFamily: "cf2", Name: "col2"}},
		Values:   []interface{}{[]byte("v1"), []byte("v2")},
		Keyspace: "keyspace",
	}
	_, err = btClient.InsertRow(ctx, insertData)
	require.NoError(t, err)

	// Delete cf2
	updateData := &translator.UpdateQueryMap{
		Table:                tableName,
		RowKey:               "row1",
		DeleteColumnFamilies: []string{"cf2"},
		Keyspace:             "keyspace",
	}
	_, err = btClient.UpdateRow(ctx, updateData)
	require.NoError(t, err)

	// Verify deletion by reading the row
	tbl := client["keyspace"].Open(tableName)
	row, err := tbl.ReadRow(ctx, "row1")
	require.NoError(t, err)
	assert.NotContains(t, row, "cf2", "cf2 should be deleted")
	assert.Contains(t, row, "cf1", "cf1 should still exist")
}

func TestMutateRowDeleteQualifiers(t *testing.T) {
	client, ctx, err := getClient(conn)
	require.NoError(t, err)

	adminClient, err := bigtable.NewAdminClient(ctx, "project", "instance", option.WithGRPCConn(conn))
	require.NoError(t, err)

	tableName := "test-table-delete-qualifiers"
	require.NoError(t, adminClient.CreateTable(ctx, tableName))
	require.NoError(t, adminClient.CreateColumnFamily(ctx, tableName, "cf1"))

	btClient := NewBigtableClient(client, zap.NewNop(), nil, BigtableConfig{}, nil, nil, nil)

	// Insert initial data with two columns
	insertData := &translator.InsertQueryMap{
		Table:    tableName,
		RowKey:   "row1",
		Columns:  []translator.Column{{ColumnFamily: "cf1", Name: "col1"}, {ColumnFamily: "cf1", Name: "col2"}},
		Values:   []interface{}{[]byte("v1"), []byte("v2")},
		Keyspace: "keyspace",
	}
	_, err = btClient.InsertRow(ctx, insertData)
	require.NoError(t, err)

	// Delete col1
	updateData := &translator.UpdateQueryMap{
		Table:                 tableName,
		RowKey:                "row1",
		DeleteColumQualifires: []translator.Column{{ColumnFamily: "cf1", Name: "col1"}},
		Keyspace:              "keyspace",
	}
	_, err = btClient.UpdateRow(ctx, updateData)
	require.NoError(t, err)

	// Verify deletion by reading the row
	tbl := client["keyspace"].Open(tableName)
	row, err := tbl.ReadRow(ctx, "row1", bigtable.RowFilter(bigtable.FamilyFilter("cf1")))
	require.NoError(t, err)
	cf := row["cf1"]
	for _, cell := range cf {
		assert.NotEqual(t, "cf1:col1", cell.Column, "col1 should be deleted")
	}
	assert.Equal(t, "cf1:col2", cf[0].Column, "col2 should still exist")
}

func TestMutateRowIfExists(t *testing.T) {
	client, ctx, err := getClient(conn)
	require.NoError(t, err)

	adminClient, err := bigtable.NewAdminClient(ctx, "project", "instance", option.WithGRPCConn(conn))
	require.NoError(t, err)

	tableName := "test-table-if-exists"
	require.NoError(t, adminClient.CreateTable(ctx, tableName))
	require.NoError(t, adminClient.CreateColumnFamily(ctx, tableName, "cf1"))

	btClient := NewBigtableClient(client, zap.NewNop(), nil, BigtableConfig{}, nil, nil, nil)

	// Insert initial data
	insertData := &translator.InsertQueryMap{
		Table:    tableName,
		RowKey:   "row1",
		Columns:  []translator.Column{{ColumnFamily: "cf1", Name: "col1"}},
		Values:   []interface{}{[]byte("v1")},
		Keyspace: "keyspace",
	}
	_, err = btClient.InsertRow(ctx, insertData)
	require.NoError(t, err)

	// Update the row when it exists
	updateData := &translator.UpdateQueryMap{
		Table:    tableName,
		RowKey:   "row1",
		Columns:  []translator.Column{{ColumnFamily: "cf1", Name: "col1"}},
		Values:   []interface{}{[]byte("v2")},
		IfExists: true,
		Keyspace: "keyspace",
	}
	_, err = btClient.UpdateRow(ctx, updateData)
	require.NoError(t, err)

	// Verify the update by reading the row
	tbl := client["keyspace"].Open(tableName)
	row, err := tbl.ReadRow(ctx, "row1", bigtable.RowFilter(bigtable.FamilyFilter("cf1")))
	require.NoError(t, err)
	assert.Equal(t, "v2", string(row["cf1"][0].Value), "value should be updated to v2")

	// Attempt to update a non-existent row
	updateData.RowKey = "row2"
	_, err = btClient.UpdateRow(ctx, updateData)
	require.NoError(t, err)

	// Verify the non-existent row is not created
	row, err = tbl.ReadRow(ctx, "row2")
	require.NoError(t, err)
	assert.Nil(t, row, "row2 should not exist")
}

func TestMutateRowIfNotExists(t *testing.T) {
	client, ctx, err := getClient(conn)
	require.NoError(t, err)

	adminClient, err := bigtable.NewAdminClient(ctx, "project", "instance", option.WithGRPCConn(conn))
	require.NoError(t, err)

	tableName := "test-table-if-not-exists"
	require.NoError(t, adminClient.CreateTable(ctx, tableName))
	require.NoError(t, adminClient.CreateColumnFamily(ctx, tableName, "cf1"))

	btClient := NewBigtableClient(client, zap.NewNop(), nil, BigtableConfig{}, nil, nil, nil)

	// Insert a row when it does not exist
	InsertData := &translator.InsertQueryMap{
		Table:       tableName,
		RowKey:      "row1",
		Columns:     []translator.Column{{ColumnFamily: "cf1", Name: "col1"}},
		Values:      []interface{}{[]byte("v1")},
		IfNotExists: true,
		Keyspace:    "keyspace",
	}
	_, err = btClient.InsertRow(ctx, InsertData)
	require.NoError(t, err)

	// Verify the row is created
	tbl := client["keyspace"].Open(tableName)
	row, err := tbl.ReadRow(ctx, "row1", bigtable.RowFilter(bigtable.FamilyFilter("cf1")))
	require.NoError(t, err)
	assert.Equal(t, "v1", string(row["cf1"][0].Value), "row1 should be created with value v1")

	// Attempt to insert the same row again
	_, err = btClient.InsertRow(ctx, InsertData)
	require.NoError(t, err)

	// Verify the row is not updated
	row, err = tbl.ReadRow(ctx, "row1", bigtable.RowFilter(bigtable.FamilyFilter("cf1")))
	require.NoError(t, err)
	assert.Equal(t, "v1", string(row["cf1"][0].Value), "row1 should not be updated")
}

func TestMutateRowNonByteValue(t *testing.T) {
	client, ctx, err := getClient(conn)
	require.NoError(t, err)

	adminClient, err := bigtable.NewAdminClient(ctx, "project", "instance", option.WithGRPCConn(conn))
	require.NoError(t, err)

	tableName := "test-table-non-byte"
	require.NoError(t, adminClient.CreateTable(ctx, tableName))
	require.NoError(t, adminClient.CreateColumnFamily(ctx, tableName, "cf1"))

	btClient := NewBigtableClient(client, zap.NewNop(), nil, BigtableConfig{}, nil, nil, nil)

	updateData := &translator.UpdateQueryMap{
		Table:    tableName,
		RowKey:   "row1",
		Columns:  []translator.Column{{ColumnFamily: "cf1", Name: "col1"}},
		Values:   []interface{}{"invalid-value"}, // string instead of []byte
		Keyspace: "keyspace",
	}
	_, err = btClient.UpdateRow(ctx, updateData)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not of type []byte")
}

func TestMutateRowInvalidKeyspace(t *testing.T) {
	client, ctx, err := getClient(conn)
	require.NoError(t, err)

	btClient := NewBigtableClient(client, zap.NewNop(), nil, BigtableConfig{}, nil, nil, nil)

	updateData := &translator.UpdateQueryMap{
		Table:    "any-table",
		RowKey:   "row1",
		Columns:  []translator.Column{{ColumnFamily: "cf1", Name: "col1"}},
		Values:   []interface{}{[]byte("value")},
		Keyspace: "invalid-keyspace",
	}
	_, err = btClient.UpdateRow(ctx, updateData)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid keySpace - `invalid-keyspace`")
}

func TestComplexUpdateWithListIndex(t *testing.T) {
	client, ctx, err := getClient(conn)
	require.NoError(t, err)

	adminClient, err := bigtable.NewAdminClient(ctx, "project", "instance", option.WithGRPCConn(conn))
	require.NoError(t, err)

	tableName := "test-table-complex-update-list-index"
	require.NoError(t, adminClient.CreateTable(ctx, tableName))
	require.NoError(t, adminClient.CreateColumnFamily(ctx, tableName, "cf1"))
	require.NoError(t, adminClient.CreateColumnFamily(ctx, tableName, "list"))

	btClient := NewBigtableClient(client, zap.NewNop(), nil, BigtableConfig{}, nil, nil, nil)

	// Insert initial data with a list in cf1
	insertData := &translator.InsertQueryMap{
		Table:    tableName,
		RowKey:   "row1",
		Columns:  []translator.Column{{ColumnFamily: "list", Name: "timestamp1"}, {ColumnFamily: "list", Name: "timestamp2"}},
		Values:   []interface{}{[]byte("v1"), []byte("v2")},
		Keyspace: "keyspace",
	}
	_, err = btClient.InsertRow(ctx, insertData)
	require.NoError(t, err)

	// Perform a complex update with list index
	complexUpdateMeta := map[string]*translator.ComplexUpdateMeta{
		"list": {
			UpdateListIndex: "1", // Update the second item in the list
			Value:           []byte("updated-v2"),
		},
	}
	updateData := &translator.UpdateQueryMap{
		Table:             tableName,
		RowKey:            "row1",
		ComplexUpdateMeta: complexUpdateMeta,
		Keyspace:          "keyspace",
		Columns:           []translator.Column{},
		Values:            []interface{}{},
	}
	_, err = btClient.UpdateRow(ctx, updateData)
	require.NoError(t, err)

	// Verify the update by reading the row
	tbl := client["keyspace"].Open(tableName)
	row, err := tbl.ReadRow(ctx, "row1", bigtable.RowFilter(bigtable.FamilyFilter("list")))
	require.NoError(t, err)
	cf := row["list"]
	assert.Equal(t, "updated-v2", string(cf[1].Value), "second item in the list should be updated")
}

func TestComplexUpdateWithListDeletion(t *testing.T) {
	client, ctx, err := getClient(conn)
	require.NoError(t, err)

	adminClient, err := bigtable.NewAdminClient(ctx, "project", "instance", option.WithGRPCConn(conn))
	require.NoError(t, err)

	tableName := "test-table-complex-update-list-delete"
	require.NoError(t, adminClient.CreateTable(ctx, tableName))
	require.NoError(t, adminClient.CreateColumnFamily(ctx, tableName, "cf1"))
	require.NoError(t, adminClient.CreateColumnFamily(ctx, tableName, "list"))

	btClient := NewBigtableClient(client, zap.NewNop(), nil, BigtableConfig{}, nil, nil, nil)

	// Insert initial data with a list in the "list" column family
	insertData := &translator.InsertQueryMap{
		Table:    tableName,
		RowKey:   "row1",
		Columns:  []translator.Column{{ColumnFamily: "list", Name: "timestamp1"}, {ColumnFamily: "list", Name: "timestamp2"}},
		Values:   []interface{}{[]byte("v1"), []byte("v2")},
		Keyspace: "keyspace",
	}
	_, err = btClient.InsertRow(ctx, insertData)
	require.NoError(t, err)

	// Perform a complex update with list deletion
	complexUpdateMeta := map[string]*translator.ComplexUpdateMeta{
		"list": {
			ListDelete:       true,
			ListDeleteValues: [][]byte{[]byte("v1")}, // Delete the first item in the list
		},
	}
	updateData := &translator.UpdateQueryMap{
		Table:             tableName,
		RowKey:            "row1",
		ComplexUpdateMeta: complexUpdateMeta,
		Keyspace:          "keyspace",
		Columns:           []translator.Column{},
		Values:            []interface{}{},
	}
	_, err = btClient.UpdateRow(ctx, updateData)
	require.NoError(t, err)

	// Verify the deletion by reading the row
	tbl := client["keyspace"].Open(tableName)
	row, err := tbl.ReadRow(ctx, "row1", bigtable.RowFilter(bigtable.FamilyFilter("list")))
	require.NoError(t, err)
	cf := row["list"]
	assert.Equal(t, 1, len(cf), "one item should remain in the list")
	assert.Equal(t, "v2", string(cf[0].Value), "remaining item should be v2")
}

func TestComplexUpdateInvalidKeyspace(t *testing.T) {
	client, ctx, err := getClient(conn)
	require.NoError(t, err)

	btClient := NewBigtableClient(client, zap.NewNop(), nil, BigtableConfig{}, nil, nil, nil)

	// Attempt to perform a complex update with an invalid keyspace
	complexUpdateMeta := map[string]*translator.ComplexUpdateMeta{
		"list": {
			UpdateListIndex: "0",
			Value:           []byte("updated-v1"),
		},
	}
	updateData := &translator.UpdateQueryMap{
		Table:             "any-table",
		RowKey:            "row1",
		ComplexUpdateMeta: complexUpdateMeta,
		Keyspace:          "invalid-keyspace",
		Columns:           []translator.Column{},
		Values:            []interface{}{},
	}
	_, err = btClient.UpdateRow(ctx, updateData)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid keySpace - `invalid-keyspace`")
}

func TestComplexUpdateOutOfBoundsIndex(t *testing.T) {
	client, ctx, err := getClient(conn)
	require.NoError(t, err)

	adminClient, err := bigtable.NewAdminClient(ctx, "project", "instance", option.WithGRPCConn(conn))
	require.NoError(t, err)
	defer adminClient.Close()

	tableName := "test-table-complex-update-out-of-bounds"
	require.NoError(t, adminClient.CreateTable(ctx, tableName))
	require.NoError(t, adminClient.CreateColumnFamily(ctx, tableName, "cf1"))
	require.NoError(t, adminClient.CreateColumnFamily(ctx, tableName, "list"))

	btClient := NewBigtableClient(client, zap.NewNop(), nil, BigtableConfig{}, nil, nil, nil)

	// Insert initial data with a list in the "list" column family
	insertData := &translator.InsertQueryMap{
		Table:    tableName,
		RowKey:   "row1",
		Columns:  []translator.Column{{ColumnFamily: "list", Name: "timestamp1"}},
		Values:   []interface{}{[]byte("v1")},
		Keyspace: "keyspace",
	}
	_, err = btClient.InsertRow(ctx, insertData)
	require.NoError(t, err)

	// Attempt to perform a complex update with an out-of-bounds index
	complexUpdateMeta := map[string]*translator.ComplexUpdateMeta{
		"list": {
			UpdateListIndex: "1", // Index 1 is out of bounds for a list of size 1
			Value:           []byte("updated-v2"),
		},
	}
	updateData := &translator.UpdateQueryMap{
		Table:             tableName,
		RowKey:            "row1",
		ComplexUpdateMeta: complexUpdateMeta,
		Keyspace:          "keyspace",
		Columns:           []translator.Column{},
		Values:            []interface{}{},
	}
	_, err = btClient.UpdateRow(ctx, updateData)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "index 1 out of bounds")
}
