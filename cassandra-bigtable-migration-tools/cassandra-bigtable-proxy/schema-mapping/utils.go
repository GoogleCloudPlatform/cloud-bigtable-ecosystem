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

package schemaMapping

import (
	"fmt"
	"sort"

	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/message"
)

func CreateTableMap(tables []*TableConfig) map[string]*TableConfig {
	var result = make(map[string]*TableConfig)
	for _, table := range tables {
		result[table.Name] = table
	}
	return result
}

// getTimestampColumnName() constructs the appropriate name for a column used in a writetime function.
//
// Parameters:
//   - aliasName: A string representing the alias of the column, if any.
//   - columnName: The actual name of the column for which the writetime function is being constructed.
//
// Returns: A string which is either the alias name or the expression "writetime(columnName)" if no alias is provided.
func getTimestampColumnName(aliasName string, columnName string) string {
	if aliasName == "" {
		return "writetime(" + columnName + ")"
	}
	return aliasName
}

// getAllColumnsMetadata() retrieves metadata for all columns in a given table.
//
// Parameters:
//   - columnsMap: column info for a given table.
//
// Returns:
// - A slice of pointers to ColumnMetadata structs containing metadata for each requested column.
// - An error
func getAllColumnsMetadata(columnsMap map[string]*types.Column) []*message.ColumnMetadata {
	var columnMetadataList []*message.ColumnMetadata
	for _, column := range columnsMap {
		columnMd := column.Metadata.Clone()
		columnMetadataList = append(columnMetadataList, columnMd)
	}
	sort.Slice(columnMetadataList, func(i, j int) bool {
		return columnMetadataList[i].Index < columnMetadataList[j].Index
	})
	return columnMetadataList
}

// handleSpecialColumn() retrieves metadata for special columns.
//
// Parameters:
//   - columnsMap: column info for a given table.
//   - columnName: column name for which the metadata is required.
//   - index: Index for the column
//
// Returns:
// - Pointers to ColumnMetadata structs containing metadata for each requested column.
// - An error
func handleSpecialColumn(columnsMap map[string]*types.Column, columnName string, index int32, isWriteTimeFunction bool) (*message.ColumnMetadata, error) {
	// Validate if the column is a special column
	if !isSpecialColumn(columnName) && !isWriteTimeFunction {
		return nil, fmt.Errorf("invalid special column: %s", columnName)
	}

	// Retrieve the first available column in the map
	var columnMd *message.ColumnMetadata
	for _, column := range columnsMap {
		columnMd = column.Metadata.Clone()
		columnMd.Index = index
		columnMd.Name = columnName
		columnMd.Type = datatype.Bigint
		break
	}

	// No matching column found
	if columnMd == nil {
		return nil, fmt.Errorf("special column %s not found in provided metadata", columnName)
	}

	return columnMd, nil
}

// cloneColumnMetadata() clones the metadata from cache.
//
// Parameters:
//   - metadata: Column metadata from cache
//   - index: Index for the column
//
// Returns:
// - Pointers to ColumnMetadata structs containing metadata for each requested column.
func cloneColumnMetadata(metadata *message.ColumnMetadata, index int32) *message.ColumnMetadata {
	columnMd := metadata.Clone()
	columnMd.Index = index
	return columnMd
}

// isSpecialColumn() to check if its a special column.
//
// Parameters:
//   - columnName: name of special column
//
// Returns:
// - boolean
func isSpecialColumn(columnName string) bool {
	return columnName == LimitValue
}

// sortPrimaryKeys sorts the primary key columns of each table based on their precedence.
// The function takes a map where the keys are table names and the values are slices of columns.
// It returns the same map with the columns sorted by their primary key precedence.
//
// Parameters:
//   - pkMetadata: A map where keys are table names (strings) and values are slices of Column structs.
//     Each Column struct contains metadata about the columns, including primary key precedence.
//
// Returns:
// - A map with the same structure as the input, but with the columns sorted by primary key precedence.
func sortPrimaryKeys(keys []*types.Column) {
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].PkPrecedence < keys[j].PkPrecedence
	})
}
