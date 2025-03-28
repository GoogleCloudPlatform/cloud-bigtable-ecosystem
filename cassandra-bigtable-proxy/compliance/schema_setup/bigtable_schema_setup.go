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
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"cloud.google.com/go/bigtable"
	"github.com/ollionorg/cassandra-to-bigtable-proxy/compliance/utility"
)

const (
	nodes = 1
)

func SetupBigtableSchema(project, instance, zone, schemaFilePath string) error {
	// Step 1: Check if the Bigtable instance exists, create if necessary
	if err := checkAndCreateInstance(project, instance, zone); err != nil {
		return err
	}

	// Step 2: Read and parse schema.json
	data, err := os.ReadFile(schemaFilePath)
	if err != nil {
		return fmt.Errorf("failed to read schema file: %v", err)
	}

	var schema TableSchema
	if err := json.Unmarshal(data, &schema); err != nil {
		return fmt.Errorf("failed to parse schema JSON: %v", err)
	}

	ctx := context.Background()
	// Create admin client for schema operations
	adminClient, err := bigtable.NewAdminClient(ctx, project, instance)
	if err != nil {
		return fmt.Errorf("failed to create admin client: %v", err)
	}
	defer adminClient.Close()

	// Create schema_mapping table if it doesn't exist
	if err := executeBigtableOperation(ctx, adminClient, "createtable", "schema_mapping"); err != nil {
		return fmt.Errorf("failed to create schema_mapping table: %v", err)
	}

	// Create column family for schema_mapping table
	if err := executeBigtableOperation(ctx, adminClient, "createfamily", "schema_mapping", "cf"); err != nil {
		return fmt.Errorf("failed to create column family for schema_mapping table: %v", err)
	}

	// Create client for data operations
	client, err := bigtable.NewClient(ctx, project, instance)
	if err != nil {
		return fmt.Errorf("failed to create Bigtable client: %v", err)
	}
	defer client.Close()

	tbl := client.Open("schema_mapping")
	batchSize := 1000 // Batch size for mutations
	mutations := make([]*bigtable.Mutation, 0, batchSize)
	rowKeys := make([]string, 0, batchSize)

	// Step 3: Iterate over tables in schema and create Bigtable setup commands
	for _, table := range schema.Tables {
		// Create table
		if err := executeBigtableOperation(ctx, adminClient, "createtable", table.TableName); err != nil {
			return fmt.Errorf("failed to create table %s: %v", table.TableName, err)
		}

		// Create column families
		for _, family := range table.ColumnFamilies {
			if err := executeBigtableOperation(ctx, adminClient, "createfamily", table.TableName, family); err != nil {
				return fmt.Errorf("failed to create column family %s in table %s: %v", family, table.TableName, err)
			}
		}

		// Populate `schema_mapping` for non-configuration tables
		if table.TableName == "schema_mapping" {
			continue
		}
		for _, column := range table.Columns {
			rowKey := fmt.Sprintf("%s#%s", column.TableName, column.ColumnName)

			// Create a mutation for the row
			mut := bigtable.NewMutation()
			mut.Set("cf", "ColumnName", bigtable.Now(), []byte(column.ColumnName))
			mut.Set("cf", "ColumnType", bigtable.Now(), []byte(column.ColumnType))
			mut.Set("cf", "IsCollection", bigtable.Now(), []byte(column.IsCollection))
			mut.Set("cf", "IsPrimaryKey", bigtable.Now(), []byte(column.IsPrimaryKey))
			mut.Set("cf", "PK_Precedence", bigtable.Now(), []byte(column.PKPrecedence))
			mut.Set("cf", "KeyType", bigtable.Now(), []byte(column.KeyType))
			mut.Set("cf", "TableName", bigtable.Now(), []byte(column.TableName))

			mutations = append(mutations, mut)
			rowKeys = append(rowKeys, rowKey)

			// Send the batch when the batch size is reached
			if len(mutations) == batchSize {
				if err := applyMutations(ctx, tbl, rowKeys, mutations, int64(batchSize)); err != nil {
					return err
				}
				mutations = mutations[:0] // Reset the batch
				rowKeys = rowKeys[:0]
			}
		}
	}
	// Send any remaining mutations
	if len(mutations) > 0 {
		if err := applyMutations(ctx, tbl, rowKeys, mutations, int64(batchSize)); err != nil {
			return err
		}
	}

	utility.LogInfo("Bigtable schema setup completed successfully!")
	return nil
}

func applyMutations(ctx context.Context, tbl *bigtable.Table, rowKeys []string, mutations []*bigtable.Mutation, batchSize int64) error {
	utility.LogInfo(fmt.Sprintf("Applying bulk mutation to create schema mapping entries with batch size %d", batchSize))
	if len(rowKeys) != len(mutations) {
		return fmt.Errorf("row keys and mutations count mismatch")
	}

	// Perform the batch mutation
	errs, err := tbl.ApplyBulk(ctx, rowKeys, mutations)
	if err != nil {
		return fmt.Errorf("failed to apply bulk mutations: %v", err)
	}

	// Check for individual errors in the batch
	for i, batchErr := range errs {
		if batchErr != nil {
			return fmt.Errorf("failed to apply mutation for row %s: %v", rowKeys[i], batchErr)
		}
	}
	return nil
}

// executeBigtableOperation performs Bigtable operations (create table, column family)
// and ignores "AlreadyExists" errors for table/column family creation.
func executeBigtableOperation(ctx context.Context, adminClient *bigtable.AdminClient, operation string, params ...string) error {
	switch operation {
	case "createtable":
		if len(params) < 1 {
			return fmt.Errorf("missing table name in createtable operation")
		}
		tableName := params[0]
		// Create table with default column families
		err := adminClient.CreateTable(ctx, tableName)
		if err != nil {
			if strings.Contains(err.Error(), "AlreadyExists") {
				utility.LogInfo(fmt.Sprintf("Table already exists: %s", tableName))
				return nil
			}
			return fmt.Errorf("failed to create table %s: %v", tableName, err)
		}

	case "createfamily":
		if len(params) < 2 {
			return fmt.Errorf("missing table or column family name in createfamily operation")
		}
		tableName := params[0]
		columnFamily := params[1]
		err := adminClient.CreateColumnFamily(ctx, tableName, columnFamily)
		if err != nil {
			if strings.Contains(err.Error(), "AlreadyExists") {
				utility.LogInfo(fmt.Sprintf("Column family already exists: %s in table %s", columnFamily, tableName))
				return nil
			}
			return fmt.Errorf("failed to create column family %s in table %s: %v", columnFamily, tableName, err)
		}

	default:
		return fmt.Errorf("unsupported operation: %s", operation)
	}

	utility.LogInfo(fmt.Sprintf("Operation executed successfully: %s", operation))
	return nil
}

// checkAndCreateInstance checks if the Bigtable instance exists and creates it if it doesn't
func checkAndCreateInstance(project, instance, zone string) error {
	ctx := context.Background()
	adminClient, err := bigtable.NewInstanceAdminClient(ctx, project)
	if err != nil {
		return fmt.Errorf("failed to create instance admin client: %v", err)
	}
	defer adminClient.Close()

	// Check if instance exists
	_, err = adminClient.InstanceInfo(ctx, instance)
	if err != nil {
		// Instance does not exist, create it
		utility.LogInfo(fmt.Sprintf("Bigtable instance '%s' does not exist. Attempting to create it...\n", instance))

		// Create instance configuration
		conf := bigtable.InstanceWithClustersConfig{
			InstanceID:  instance,
			DisplayName: instance,
			Clusters: []bigtable.ClusterConfig{
				{
					ClusterID: instance,
					Zone:      zone,
					NumNodes:  nodes,
				},
			},
		}

		// Create instance
		err = adminClient.CreateInstanceWithClusters(ctx, &conf)
		if err != nil {
			return fmt.Errorf("failed to create Bigtable instance '%s': %v", instance, err)
		}

		utility.LogInfo(fmt.Sprintf("Bigtable instance '%s' created successfully.\n", instance))
	} else {
		utility.LogInfo(fmt.Sprintf("Bigtable instance '%s' already exists.\n", instance))
	}

	return nil
}

// teardownBigtableInstance deletes the Bigtable instance to clean up resources.
func TeardownBigtableInstance(project, instance string) error {
	ctx := context.Background()
	adminClient, err := bigtable.NewInstanceAdminClient(ctx, project)
	if err != nil {
		return fmt.Errorf("failed to create instance admin client: %v", err)
	}
	defer adminClient.Close()

	err = adminClient.DeleteInstance(ctx, instance)
	if err != nil {
		utility.LogTestError(fmt.Sprintf("Failed to delete Bigtable instance '%s': %v", instance, err))
	} else {
		utility.LogInfo(fmt.Sprintf("Successfully deleted Bigtable instance '%s'.", instance))
	}
	return err
}
