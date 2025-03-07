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

	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	schemaMapping "github.com/ollionorg/cassandra-to-bigtable-proxy/schema-mapping"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func Test_formatePrepareQuery(t *testing.T) {
	type args struct {
		query string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "USING Question Mark in update",
			args: args{query: "UPDATE user_profiles USING TIMESTAMP ? SET name = ?, email = ? WHERE user_id = ? using ttl  ?"},
			want: "UPDATE user_profiles USING TIMESTAMP '?' SET name = '?', email = '?' WHERE user_id = '?' using ttl  '?'",
		},
		{
			name: "USING Question Mark in insert",
			args: args{query: "UPDATE user_profiles USING TIMESTAMP ? SET name = ?, email = ? WHERE user_id = ? using ttl  ?"},
			want: "UPDATE user_profiles USING TIMESTAMP '?' SET name = '?', email = '?' WHERE user_id = '?' using ttl  '?'",
		},
		{
			name: "USING Question Mark in Insert",
			args: args{query: "INSERT INTO user_profiles (user_id, name, email) VALUES (?, ?, ?) USING TTL ? "},
			want: "INSERT INTO user_profiles (user_id, name, email) VALUES ('?', '?', '?') USING TTL '?' ",
		},
		{
			name: "Single Question Mark",
			args: args{query: "SELECT * FROM utest WHERE uid = ?"},
			want: "SELECT * FROM utest WHERE uid = '?'",
		},
		{
			name: "Multiple Question Marks",
			args: args{query: "SELECT * FROM utest WHERE uid1 = ? AND uid2 = ?"},
			want: "SELECT * FROM utest WHERE uid1 = '?' AND uid2 = '?'",
		},
		{
			name: "No Question Marks",
			args: args{query: "SELECT * FROM utest"},
			want: "SELECT * FROM utest",
		},
		{
			name: "Empty String",
			args: args{query: ""},
			want: "",
		},
		{
			name: "Escaped Question Mark",
			args: args{query: "SELECT * FROM utest WHERE uid = \\?"},
			want: "SELECT * FROM utest WHERE uid = \\?",
		},
		{
			name: "Already Formatted Question Marks",
			args: args{query: "SELECT * FROM utest WHERE uid = '?'"},
			want: "SELECT * FROM utest WHERE uid = '?'",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatePrepareQuery(tt.args.query); got != tt.want {
				t.Errorf("formatePrepareQuery() = %v, want %v", got, tt.want)
			}
		})
	}
}

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

// TestExtractAfterWhere tests the extractAfterWhere function with various SQL queries.
func TestExtractAfterWhere(t *testing.T) {
	tests := []struct {
		name     string
		sqlQuery string
		want     string
		wantErr  bool
	}{
		{
			name:     "Valid query with uppercase WHERE",
			sqlQuery: "SELECT * FROM table WHERE column = 'value';",
			want:     "column = 'value';",
			wantErr:  false,
		},
		{
			name:     "Valid query with lowercase where",
			sqlQuery: "SELECT * FROM table where column = 'value';",
			want:     "column = 'value';",
			wantErr:  false,
		},
		{
			name:     "No WHERE clause",
			sqlQuery: "SELECT * FROM table;",
			want:     "",
			wantErr:  true,
		},
		{
			name:     "Mixed case WHERE",
			sqlQuery: "SELECT * FROM table WhErE XOZO = 'value';",
			want:     "XOZO = 'value';",
			wantErr:  false,
		},
		{
			name:     "Complex query with subquery",
			sqlQuery: "UPDATE table SET column = 'value' WHERE ID IN (SELECT id FROM table2 WHERE column2 = 'value2');",
			want:     "ID IN (SELECT id FROM table2 WHERE column2 = 'value2');",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractAfterWhere(tt.sqlQuery)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractAfterWhere() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("extractAfterWhere() = %v, want %v", got, tt.want)
			}
		})
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
	protocolVersion := primitive.ProtocolVersion4

	// Sample input metadata
	sampleTableMetadata := map[string]map[string]map[string]*schemaMapping.Column{
		"keyspace1": {
			"users": {
				"id":    {ColumnName: "id", ColumnType: "uuid", IsPrimaryKey: true},
				"email": {ColumnName: "email", ColumnType: "text", IsPrimaryKey: false},
			},
		},
	}

	tests := []struct {
		name        string
		tableMeta   map[string]map[string]map[string]*schemaMapping.Column
		expectEmpty bool
	}{
		{
			name:        "Success Case - Valid Metadata",
			tableMeta:   sampleTableMetadata,
			expectEmpty: false,
		},
		{
			name:        "Empty Metadata - Should Return Empty Cache",
			tableMeta:   map[string]map[string]map[string]*schemaMapping.Column{},
			expectEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ConstructSystemMetadataRows(tt.tableMeta)

			assert.NoError(t, err)
			if tt.expectEmpty {
				assert.Empty(t, got.KeyspaceSystemQueryMetadataCache[protocolVersion])
				assert.Empty(t, got.TableSystemQueryMetadataCache[protocolVersion])
				assert.NotEmpty(t, got.ColumnsSystemQueryMetadataCache[protocolVersion])
			} else {
				assert.NotEmpty(t, got.KeyspaceSystemQueryMetadataCache[protocolVersion])
				assert.NotEmpty(t, got.TableSystemQueryMetadataCache[protocolVersion])
				assert.NotEmpty(t, got.ColumnsSystemQueryMetadataCache[protocolVersion])
			}
		})
	}
}
