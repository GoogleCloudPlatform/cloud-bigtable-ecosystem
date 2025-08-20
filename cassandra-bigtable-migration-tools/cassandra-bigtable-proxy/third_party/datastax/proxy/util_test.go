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
package proxy

import (
	"testing"
	"time"

	types "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	schemaMapping "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/schema-mapping"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/utilities"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

// TestUnixToISO tests the unixToISO function by checking if it correctly formats a known Unix timestamp.
func TestUnixToISO(t *testing.T) {
	// Known Unix timestamp and its corresponding ISO 8601 representation
	unixTimestamp := int64(1609459200) // 2021-01-01T00:00:00Z
	expectedISO := "2021-01-01T00:00:00Z"

	isoTimestamp := unixToISO(unixTimestamp)
	if isoTimestamp != expectedISO {
		t.Errorf("unixToISO(%d) = %s; want %s", unixTimestamp, isoTimestamp, expectedISO)
	}
}

// TestAddSecondsToCurrentTimestamp tests the addSecondsToCurrentTimestamp function by checking the format of the output.
func TestAddSecondsToCurrentTimestamp(t *testing.T) {
	secondsToAdd := int64(3600) // 1 hour
	result := addSecondsToCurrentTimestamp(secondsToAdd)

	// Parse the result to check if it's in correct ISO 8601 format
	if _, err := time.Parse(time.RFC3339, result); err != nil {
		t.Errorf("addSecondsToCurrentTimestamp(%d) returned an incorrectly formatted time: %s", secondsToAdd, result)
	}
}

// Mock ResponseHandler for testing
type MockResponseHandler struct {
	mock.Mock
}

// Mock function for BuildResponseForSystemQueries
func (m *MockResponseHandler) BuildResponseForSystemQueries(rows [][]interface{}, protocolV primitive.ProtocolVersion) ([]message.Row, error) {
	args := m.Called(rows, protocolV)
	return args.Get(0).([]message.Row), args.Error(1)
}

// Test case: Successful conversion of all metadata
func TestGetSystemQueryMetadataCache_Success(t *testing.T) {
	protocolVersion := primitive.ProtocolVersion4

	// Sample metadata rows for keyspace, table, and columns
	keyspaceMetadata := [][]interface{}{
		{"keyspace1", true, map[string]string{"class": "SimpleStrategy", "replication_factor": "1"}},
	}
	tableMetadata := [][]interface{}{
		{"keyspace1", "table1", "99p", 0.01, map[string]string{"keys": "ALL", "rows_per_partition": "NONE"}, []string{"compound"}},
	}
	columnMetadata := [][]interface{}{
		{"keyspace1", "table1", "column1", "none", "regular", 0, "text"},
	}

	// Call actual function
	result, err := getSystemQueryMetadataCache(keyspaceMetadata, tableMetadata, columnMetadata)

	// Ensure no error occurred
	assert.NoError(t, err, "Expected no error")
	assert.NotNil(t, result, "Expected non-nil result")

	assert.NotEmpty(t, result.KeyspaceSystemQueryMetadataCache[protocolVersion])
	assert.NotEmpty(t, result.TableSystemQueryMetadataCache[protocolVersion])
	assert.NotEmpty(t, result.ColumnsSystemQueryMetadataCache[protocolVersion])
}

// Test case: Failure in keyspace metadata conversion
func TestGetSystemQueryMetadataCache_KeyspaceFailure(t *testing.T) {
	protocolVersion := primitive.ProtocolVersion4

	// Sample metadata (invalid structure for failure simulation)
	keyspaceMetadata := [][]interface{}{
		{"keyspace1", true, make(chan int)}, // Invalid type to trigger failure
	}
	tableMetadata := [][]interface{}{}
	columnMetadata := [][]interface{}{}

	// Call function
	result, err := getSystemQueryMetadataCache(keyspaceMetadata, tableMetadata, columnMetadata)

	// Assertions
	assert.Error(t, err, "Expected error for keyspace metadata failure")
	assert.Nil(t, result.KeyspaceSystemQueryMetadataCache[protocolVersion])
}

// Test case: Failure in table metadata conversion
func TestGetSystemQueryMetadataCache_TableFailure(t *testing.T) {
	protocolVersion := primitive.ProtocolVersion4

	// Sample metadata (valid keyspace but invalid table metadata)
	keyspaceMetadata := [][]interface{}{
		{"keyspace1", true, map[string]string{"class": "SimpleStrategy", "replication_factor": "1"}},
	}
	tableMetadata := [][]interface{}{
		{"keyspace1", "table1", make(chan int)}, // Invalid type to trigger failure
	}
	columnMetadata := [][]interface{}{}

	// Call function
	result, err := getSystemQueryMetadataCache(keyspaceMetadata, tableMetadata, columnMetadata)

	// Assertions
	assert.Error(t, err, "Expected error for table metadata failure")
	assert.Nil(t, result.TableSystemQueryMetadataCache[protocolVersion])
}

// Test case: Failure in column metadata conversion
func TestGetSystemQueryMetadataCache_ColumnFailure(t *testing.T) {
	protocolVersion := primitive.ProtocolVersion4

	// Sample metadata (valid keyspace and table but invalid column metadata)
	keyspaceMetadata := [][]interface{}{
		{"keyspace1", true, map[string]string{"class": "SimpleStrategy", "replication_factor": "1"}},
	}
	tableMetadata := [][]interface{}{
		{"keyspace1", "table1", "99p", 0.01, map[string]string{"keys": "ALL", "rows_per_partition": "NONE"}, []string{"compound"}},
	}
	columnMetadata := [][]interface{}{
		{"keyspace1", "table1", "column1", "none", "regular", 0, make(chan int)}, // Invalid type to trigger failure
	}

	// Call function
	result, err := getSystemQueryMetadataCache(keyspaceMetadata, tableMetadata, columnMetadata)

	// Assertions
	assert.Error(t, err, "Expected error for column metadata failure")
	assert.Nil(t, result.ColumnsSystemQueryMetadataCache[protocolVersion])
}

// Test case: ConstructSystemMetadataRows function
func TestConstructSystemMetadataRows(t *testing.T) {
	tests := []struct {
		name          string
		metadata      *schemaMapping.SchemaMappingConfig
		expectedEmpty bool
		expectedError bool
	}{
		{
			name:          "Empty Metadata - Should Return Empty Cache",
			metadata:      schemaMapping.NewSchemaMappingConfig("cf", zap.NewNop(), nil),
			expectedEmpty: true,
			expectedError: false,
		},
		{
			name: "Valid Metadata - Should Return Populated Cache",
			metadata: schemaMapping.NewSchemaMappingConfig(
				"cf",
				zap.NewNop(),
				[]*schemaMapping.TableConfig{
					schemaMapping.NewTableConfig(
						"test_keyspace",
						"test_table",
						"cf",
						false,
						[]*types.Column{
							{
								Name:         "id",
								CQLType:      datatype.Uuid,
								KeyType:      "partition",
								IsPrimaryKey: true,
								ColumnFamily: "cf",
							},
						},
					),
				},
			),
			expectedEmpty: false,
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache, err := ConstructSystemMetadataRows(tt.metadata.GetAllTables())
			if tt.expectedError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// For empty metadata test case, we should only have system metadata
			if tt.expectedEmpty {
				// Check that we have the expected system metadata
				if len(cache.KeyspaceSystemQueryMetadataCache) == 0 {
					t.Error("Expected system keyspace metadata")
				}
				if len(cache.TableSystemQueryMetadataCache) == 0 {
					t.Error("Expected system table metadata")
				}
				if len(cache.ColumnsSystemQueryMetadataCache) == 0 {
					t.Error("Expected system column metadata")
				}
				return
			}

			// For non-empty metadata, verify the cache is populated
			if len(cache.KeyspaceSystemQueryMetadataCache) == 0 {
				t.Error("Expected non-empty keyspace metadata cache")
			}
			if len(cache.TableSystemQueryMetadataCache) == 0 {
				t.Error("Expected non-empty table metadata cache")
			}
			if len(cache.ColumnsSystemQueryMetadataCache) == 0 {
				t.Error("Expected non-empty column metadata cache")
			}
		})
	}
}

func TestGetKeyspaceMetadata(t *testing.T) {
	tests := []struct {
		name              string
		tableConfigs      []*schemaMapping.TableConfig
		expectedCount     int
		expectedKeyspaces []string
	}{
		{
			name:              "Empty Metadata",
			tableConfigs:      []*schemaMapping.TableConfig{},
			expectedCount:     0,
			expectedKeyspaces: []string{},
		},
		{
			name: "Single Keyspace",
			tableConfigs: []*schemaMapping.TableConfig{
				schemaMapping.NewTableConfig("test_keyspace", "test_table", "cf1", false, []*types.Column{
					{Name: "id", CQLType: datatype.Uuid, KeyType: utilities.KEY_TYPE_REGULAR},
				}),
			},
			expectedCount:     1,
			expectedKeyspaces: []string{"test_keyspace"},
		},
		{
			name: "Multiple Keyspaces",
			tableConfigs: []*schemaMapping.TableConfig{
				schemaMapping.NewTableConfig("keyspace1", "table1", "cf1", false, []*types.Column{
					{Name: "col1", CQLType: datatype.Varchar, KeyType: utilities.KEY_TYPE_REGULAR},
				}),
				schemaMapping.NewTableConfig("keyspace1", "table2", "cf1", false, []*types.Column{
					{Name: "col2", CQLType: datatype.Int, KeyType: utilities.KEY_TYPE_REGULAR},
				}),
				schemaMapping.NewTableConfig("keyspace2", "table2", "cf1", false, []*types.Column{
					{Name: "col2", CQLType: datatype.Int, KeyType: utilities.KEY_TYPE_REGULAR},
				}),
			},
			expectedCount:     2,
			expectedKeyspaces: []string{"keyspace1", "keyspace2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schemaConfig := schemaMapping.NewSchemaMappingConfig("cf1", zap.NewNop(), tt.tableConfigs)
			result := getKeyspaceMetadata(schemaConfig.GetAllTables())

			assert.Equal(t, tt.expectedCount, len(result), "Expected %d keyspaces, got %d", tt.expectedCount, len(result))
			for _, row := range result {
				assert.Equal(t, 3, len(row), "Each row should have 3 elements")
				keyspace := row[0].(string)
				assert.Contains(t, tt.expectedKeyspaces, keyspace, "Unexpected keyspace: %s", keyspace)
				assert.True(t, row[1].(bool), "durable_writes should be true")
				replication := row[2].(map[string]string)
				assert.Contains(t, replication, "class", "Replication map should contain 'class'")
			}
		})
	}
}

func TestGetTableMetadata(t *testing.T) {
	tests := []struct {
		name           string
		tableConfigs   []*schemaMapping.TableConfig
		expectedCount  int
		expectedTables []struct{ keyspace, table string }
	}{
		{
			name:           "Empty Metadata",
			tableConfigs:   []*schemaMapping.TableConfig{},
			expectedCount:  0,
			expectedTables: []struct{ keyspace, table string }{},
		},
		{
			name: "Single Table",
			tableConfigs: []*schemaMapping.TableConfig{
				schemaMapping.NewTableConfig("test_keyspace", "test_table", "cf1", false, []*types.Column{
					{Name: "id", CQLType: datatype.Uuid, KeyType: utilities.KEY_TYPE_REGULAR},
				}),
			},
			expectedCount: 1,
			expectedTables: []struct{ keyspace, table string }{
				{"test_keyspace", "test_table"},
			},
		},
		{
			name: "Multiple tables",
			tableConfigs: []*schemaMapping.TableConfig{
				schemaMapping.NewTableConfig("keyspace1", "table1", "cf1", false, []*types.Column{
					{Name: "col1", CQLType: datatype.Varchar, KeyType: utilities.KEY_TYPE_REGULAR},
				}),
				schemaMapping.NewTableConfig("keyspace1", "table2", "cf1", false, []*types.Column{
					{Name: "col2", CQLType: datatype.Int, KeyType: utilities.KEY_TYPE_REGULAR},
				}),
			},
			expectedCount: 2,
			expectedTables: []struct{ keyspace, table string }{
				{"keyspace1", "table1"},
				{"keyspace1", "table2"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schemaConfig := schemaMapping.NewSchemaMappingConfig("cf1", zap.NewNop(), tt.tableConfigs)
			result := getTableMetadata(schemaConfig.GetAllTables())

			assert.Equal(t, tt.expectedCount, len(result), "Expected %d tables, got %d", tt.expectedCount, len(result))
			for _, row := range result {
				assert.Equal(t, 6, len(row), "Each row should have 6 elements")
				keyspace, table := row[0].(string), row[1].(string)
				found := false
				for _, expected := range tt.expectedTables {
					if expected.keyspace == keyspace && expected.table == table {
						found = true
						break
					}
				}
				assert.True(t, found, "Unexpected table: %s.%s", keyspace, table)
			}
		})
	}
}

func TestGetColumnMetadata(t *testing.T) {
	tests := []struct {
		name            string
		tableConfigs    []*schemaMapping.TableConfig
		expectedCount   int
		expectedColumns []struct{ keyspace, table, column, kind string }
	}{
		{
			name:            "Empty Metadata",
			tableConfigs:    []*schemaMapping.TableConfig{},
			expectedCount:   0,
			expectedColumns: []struct{ keyspace, table, column, kind string }{},
		},
		{
			name: "Single Column",
			tableConfigs: []*schemaMapping.TableConfig{
				schemaMapping.NewTableConfig("test_keyspace", "test_table", "cf1", false, []*types.Column{
					{Name: "id", CQLType: datatype.Uuid, IsPrimaryKey: true, KeyType: "partition"},
				}),
			},
			expectedCount: 1,
			expectedColumns: []struct{ keyspace, table, column, kind string }{
				{"test_keyspace", "test_table", "id", "partition"},
			},
		},
		{
			name: "Multiple Columns",
			tableConfigs: []*schemaMapping.TableConfig{
				schemaMapping.NewTableConfig("keyspace1", "table1", "cf1", false, []*types.Column{
					{Name: "id", CQLType: datatype.Uuid, IsPrimaryKey: true, KeyType: "partition"},
					{Name: "name", CQLType: datatype.Varchar, IsPrimaryKey: true, KeyType: "clustering"},
					{Name: "age", CQLType: datatype.Int, IsPrimaryKey: false, KeyType: "regular"},
				}),
			},
			expectedCount: 3,
			expectedColumns: []struct{ keyspace, table, column, kind string }{
				{"keyspace1", "table1", "id", "partition"},
				{"keyspace1", "table1", "name", "clustering"},
				{"keyspace1", "table1", "age", "regular"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schemaConfig := schemaMapping.NewSchemaMappingConfig("cf1", zap.NewNop(), tt.tableConfigs)
			result := getColumnMetadata(schemaConfig.GetAllTables())

			assert.Equal(t, tt.expectedCount, len(result), "Expected %d columns, got %d", tt.expectedCount, len(result))
			for _, row := range result {
				assert.Equal(t, 7, len(row), "Each row should have 7 elements")
				keyspace, table, column, kind := row[0].(string), row[1].(string), row[2].(string), row[4].(string)
				found := false
				for _, expected := range tt.expectedColumns {
					if expected.keyspace == keyspace && expected.table == table &&
						expected.column == column && expected.kind == kind {
						found = true
						break
					}
				}
				assert.True(t, found, "Unexpected column: %s.%s.%s (kind: %s)", keyspace, table, column, kind)
			}
		})
	}
}
