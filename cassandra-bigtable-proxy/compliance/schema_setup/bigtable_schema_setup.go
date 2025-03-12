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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"cloud.google.com/go/bigtable"
	"github.com/ollionorg/cassandra-to-bigtable-proxy/compliance/utility"
)

type TableSchema struct {
	Tables []struct {
		TableName      string   `json:"table_name"`
		ColumnFamilies []string `json:"column_families"`
		Columns        []struct {
			ColumnName   string `json:"ColumnName"`
			ColumnType   string `json:"ColumnType"`
			IsCollection string `json:"IsCollection"`
			IsPrimaryKey string `json:"IsPrimaryKey"`
			PKPrecedence string `json:"PK_Precedence"`
			TableName    string `json:"TableName"`
		} `json:"columns"`
	} `json:"tables"`
}

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
		if err := executeCBTCommandIgnoreExists(project, instance, fmt.Sprintf("createtable %s", table.TableName)); err != nil {
			return fmt.Errorf("failed to create table %s: %v", table.TableName, err)
		}

		// Create column families
		for _, family := range table.ColumnFamilies {
			if err := executeCBTCommandIgnoreExists(project, instance, fmt.Sprintf("createfamily %s %s", table.TableName, family)); err != nil {
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
	utility.LogInfo(fmt.Sprintf("Applying bulk mutaion applied to create schema mapping entries with batch size %d", batchSize))
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

// executeCBTCommandIgnoreExists runs a CBT command and ignores "AlreadyExists" errors for table/column family creation.
func executeCBTCommandIgnoreExists(project, instance, command string) error {
	cmd := exec.Command("cbt", "-project", project, "-instance", instance)
	args := strings.Fields(command)
	cmd.Args = append(cmd.Args, args...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(output), "AlreadyExists") {
			// Log and ignore the "AlreadyExists" error
			utility.LogInfo(fmt.Sprintf("Command skipped as resource already exists: %s", command))
			return nil
		}
		// Log and return other errors
		return fmt.Errorf("command failed: %s, output: %s, error: %v", command, output, err)
	}

	utility.LogInfo(fmt.Sprintf("Command executed successfully: %s", command))
	return nil
}

// checkAndCreateInstance checks if the Bigtable instance exists and creates it if it doesn't
func checkAndCreateInstance(project, instance, zone string) error {
	// Check if the instance exists
	cmd := exec.Command("gcloud", "bigtable", "instances", "describe", instance, "--project", project)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// Instance does not exist or an error occurred, attempt to create it
		utility.LogInfo(fmt.Sprintf("Bigtable instance '%s' does not exist or there was an error. Attempting to create it...\n", instance))

		createCmd := exec.Command(
			"gcloud", "bigtable", "instances", "create", instance,
			"--cluster-config", fmt.Sprintf("id=%s,zone=%s,nodes=%d", instance, zone, nodes),
			"--display-name", instance,
			"--project", project,
		)

		if createErr := createCmd.Run(); createErr != nil {
			return fmt.Errorf("failed to create Bigtable instance '%s': %v, error: %s", instance, createErr, stderr.String())
		}

		utility.LogInfo(fmt.Sprintf("Bigtable instance '%s' created successfully.\n", instance))
	} else {
		utility.LogInfo(fmt.Sprintf("Bigtable instance '%s' already exists.\n", instance))
	}

	return nil
}

// teardownBigtableInstance deletes the Bigtable instance to clean up resources.
func TeardownBigtableInstance(project, instance string) error {
	cmd := exec.Command("gcloud", "bigtable", "instances", "delete", instance, "--quiet", "--project", project)
	output, err := cmd.CombinedOutput()
	if err != nil {
		utility.LogTestError(fmt.Sprintf("Failed to delete Bigtable instance '%s': %v\nOutput: %s", instance, err, output))
	} else {
		utility.LogInfo(fmt.Sprintf("Successfully deleted Bigtable instance '%s'.", instance))
	}
	return err
}
