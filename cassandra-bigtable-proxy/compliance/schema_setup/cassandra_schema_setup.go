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

package schema_setup

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/gocql/gocql"
	"github.com/ollionorg/cassandra-to-bigtable-proxy/compliance/utility"
)

// Column represents the definition of a single column in the table schema
type Column struct {
	ColumnName   string `json:"ColumnName"`
	ColumnType   string `json:"ColumnType"`
	IsPrimaryKey string `json:"IsPrimaryKey"`
	PKPrecedence string `json:"PK_Precedence"`
}

// Table represents the schema definition for a single table
type Table struct {
	TableName string   `json:"table_name"`
	Columns   []Column `json:"columns"`
}

// CassandraTableSchema represents the schema for multiple Cassandra tables
type CassandraTableSchema struct {
	Keyspace string  `json:"keyspace"`
	Tables   []Table `json:"tables"`
}

// SetupCassandraSchema sets up the Cassandra keyspace and tables
func SetupCassandraSchema(session *gocql.Session, schemaFilePath, keyspace string) error {
	// Step 1: Parse the schema file
	schema, err := parseSchema(schemaFilePath)
	if err != nil {
		return err
	}

	// Step 2: Create keyspace
	if err := createKeyspace(session, keyspace); err != nil {
		return err
	}

	// Step 3: Iterate over tables and create them
	for _, table := range schema.Tables {
		// Skip "schema_mapping" table
		if table.TableName == "schema_mapping" {
			continue
		}

		// Generate the CREATE TABLE statement
		createStmt, err := generateCassandraCreateTableStatement(table, keyspace)
		if err != nil {
			return fmt.Errorf("failed to generate CREATE TABLE statement for table %s: %v", table.TableName, err)
		}

		// Execute the statement
		utility.LogInfo(fmt.Sprintf("Executing: %s", createStmt))
		if err := session.Query(createStmt).Exec(); err != nil {
			return fmt.Errorf("failed to execute CREATE TABLE statement for table %s: %v", table.TableName, err)
		}
	}

	utility.LogInfo("Cassandra schema setup completed successfully!")
	return nil
}

// parseSchema parses the schema JSON file and returns the CassandraTableSchema
func parseSchema(schemaFilePath string) (*CassandraTableSchema, error) {
	data, err := os.ReadFile(schemaFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema file: %v", err)
	}

	var schema CassandraTableSchema
	if err := json.Unmarshal(data, &schema); err != nil {
		return nil, fmt.Errorf("failed to parse schema JSON: %v", err)
	}

	return &schema, nil
}

// createKeyspace creates a keyspace in Cassandra
func createKeyspace(session *gocql.Session, keyspace string) error {
	ddlStatementKeyspace := fmt.Sprintf(
		"CREATE KEYSPACE IF NOT EXISTS %s WITH REPLICATION = { 'class' : 'SimpleStrategy', 'replication_factor' : '1' };",
		keyspace,
	)
	utility.LogInfo(fmt.Sprintf("Creating keyspace: %s", ddlStatementKeyspace))
	if err := session.Query(ddlStatementKeyspace).Exec(); err != nil {
		return fmt.Errorf("could not create keyspace %s: %v", keyspace, err)
	}
	return nil
}

// generateCassandraCreateTableStatement generates the CREATE TABLE statement for a single table
func generateCassandraCreateTableStatement(table Table, keyspace string) (string, error) {
	primaryKeys := map[int][]string{}
	columnDefinitions := []string{}

	for _, column := range table.Columns {
		columnDef := fmt.Sprintf("%s %s", column.ColumnName, column.ColumnType)
		columnDefinitions = append(columnDefinitions, columnDef)

		if column.IsPrimaryKey == "true" {
			precedence, err := strconv.Atoi(column.PKPrecedence)
			if err != nil {
				return "", fmt.Errorf("invalid precedence value for column %s in table %s", column.ColumnName, table.TableName)
			}
			primaryKeys[precedence] = append(primaryKeys[precedence], column.ColumnName)
		}
	}

	primaryKeyDef, err := generateCassandraPrimaryKey(primaryKeys)
	if err != nil {
		return "", fmt.Errorf("failed to generate primary key for table %s: %v", table.TableName, err)
	}

	createStmt := fmt.Sprintf(
		"CREATE TABLE IF NOT EXISTS %s.%s (%s, %s);",
		keyspace,
		table.TableName,
		strings.Join(columnDefinitions, ", "),
		primaryKeyDef,
	)

	return createStmt, nil
}

// generateCassandraPrimaryKey generates the PRIMARY KEY definition for a table
func generateCassandraPrimaryKey(primaryKeys map[int][]string) (string, error) {
	var partitionKeys []string
	var clusteringKeys []string

	for precedence, keys := range primaryKeys {
		if precedence == 1 {
			partitionKeys = append(partitionKeys, keys...)
		} else {
			clusteringKeys = append(clusteringKeys, keys...)
		}
	}

	if len(partitionKeys) == 0 {
		return "", fmt.Errorf("no partition key defined")
	}

	partitionKeyDef := fmt.Sprintf("(%s)", strings.Join(partitionKeys, ", "))
	if len(clusteringKeys) > 0 {
		return fmt.Sprintf("PRIMARY KEY (%s, %s)", partitionKeyDef, strings.Join(clusteringKeys, ", ")), nil
	}

	return fmt.Sprintf("PRIMARY KEY (%s)", partitionKeyDef), nil
}
