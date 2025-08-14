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
	"slices"
	"sort"

	types "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/utilities"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"go.uber.org/zap"
)

const (
	LimitValue = "limitValue"
)

type SchemaMappingConfig struct {
	Logger             *zap.Logger
	Tables             map[string]map[string]*TableConfig
	SystemColumnFamily string
}

type TableConfig struct {
	Keyspace           string
	Name               string
	Columns            map[string]*types.Column
	PrimaryKeys        []*types.Column
	SystemColumnFamily string
}

type SelectedColumns struct {
	Name              string
	IsFunc            bool
	IsAs              bool
	FuncName          string
	Alias             string
	MapKey            string
	ListIndex         string
	ColumnName        string //this will be the column name for writetime function,aggregate function and Map key access
	KeyType           string
	IsWriteTimeColumn bool
}

func CreateTableConfig(systemColumnFamily string, mappings map[string]map[string]map[string]*types.Column) map[string]map[string]*TableConfig {
	results := make(map[string]map[string]*TableConfig)

	for keyspace, tables := range mappings {
		// add new keyspace
		if _, ok := results[keyspace]; !ok {
			results[keyspace] = make(map[string]*TableConfig)
		}
		for tableName, columns := range tables {
			// add new table
			if _, ok := results[keyspace][tableName]; !ok {
				results[keyspace][tableName] = &TableConfig{
					Keyspace:           keyspace,
					Name:               tableName,
					SystemColumnFamily: systemColumnFamily,
					Columns:            make(map[string]*types.Column),
				}
			}
			for columnName, column := range columns {
				results[keyspace][tableName].Columns[columnName] = column
			}
		}
	}

	return results
}

// GetTableConfig finds the primary key columns of a specified table in a given keyspace.
//
// This method looks up the cached primary key metadata and returns the relevant columns.
//
// Parameters:
//   - keySpace: The name of the keyspace where the table resides.
//   - tableName: The name of the table for which primary key metadata is requested.
//
// Returns:
//   - []types.Column: A slice of types.Column structs representing the primary keys of the table.
//   - error: Returns an error if the primary key metadata is not found.
func (c *SchemaMappingConfig) GetTableConfig(keySpace string, tableName string) (*TableConfig, error) {
	keyspace, ok := c.Tables[keySpace]
	if !ok {
		return nil, fmt.Errorf("keyspace %s does not exist", keySpace)
	}
	tableConfig, ok := keyspace[tableName]
	if !ok {
		return nil, fmt.Errorf("table %s does not exist", tableName)
	}
	return tableConfig, nil
}

func (tableConfig *TableConfig) GetPkByTableNameWithFilter(filterPrimaryKeys []string) []*types.Column {
	var result []*types.Column
	for _, pmk := range tableConfig.PrimaryKeys {
		if slices.Contains(filterPrimaryKeys, pmk.Name) {
			result = append(result, pmk)
		}
	}
	return result
}

// GetColumnFamily retrieves the column family for a given column.
// Returns the column family name from the schema mapping with validation.
// Returns default column family for primitive types and column name for collections.
func (tableConfig *TableConfig) GetColumnFamily(columnName string) string {
	colType, err := tableConfig.GetColumnType(columnName)
	if err == nil && utilities.IsCollection(colType) {
		return columnName
	}
	return tableConfig.SystemColumnFamily
}

func (tableConfig *TableConfig) GetColumn(columnName string) (*types.Column, error) {
	col, ok := tableConfig.Columns[columnName]
	if !ok {
		return nil, fmt.Errorf("undefined column name %s in table %s.%s", columnName, tableConfig.Keyspace, tableConfig.Name)
	}
	return col, nil
}

func (tableConfig *TableConfig) GetPrimaryKeys() []string {
	var primaryKeys []string
	for _, pk := range tableConfig.PrimaryKeys {
		primaryKeys = append(primaryKeys, pk.Name)
	}
	return primaryKeys
}

func (tableConfig *TableConfig) GetColumnType(columnName string) (datatype.DataType, error) {
	col, ok := tableConfig.Columns[columnName]
	if !ok {
		return nil, fmt.Errorf("undefined column name %s in table %s.%s", columnName, tableConfig.Keyspace, tableConfig.Name)
	}

	if col.CQLType == nil {
		return nil, fmt.Errorf("undefined column name %s in table %s.%s", columnName, tableConfig.Keyspace, tableConfig.Name)
	}

	return col.CQLType, nil
}

// GetMetadataForColumns() retrieves metadata for specific columns in a given table.
// This method is a part of the SchemaMappingConfig struct.
//
// Parameters:
//   - tableName: The name of the table for which column metadata is being requested.
//   - columnNames(optional):Accepts nil if no columnNames provided or else A slice of strings containing the names of the columns for which
//     metadata is required. If this slice is empty, metadata for all
//     columns in the table will be returned.
//
// Returns:
// - A slice of pointers to ColumnMetadata structs containing metadata for each requested column.
// - An error if the specified table is not found in the Columns.
func (tableConfig *TableConfig) GetMetadataForColumns(columnNames []string) ([]*message.ColumnMetadata, error) {
	if len(columnNames) == 0 {
		return getAllColumnsMetadata(tableConfig.Columns), nil
	}
	ff, err := tableConfig.getSpecificColumnsMetadata(columnNames)
	return ff, err
}

// GetMetadataForSelectedColumns retrieves metadata for specified columns in a given table.
// This method fetches metadata for the selected columns from the schema mapping configuration.
// If no columns are specified, metadata for all columns in the table is returned.
//
// Parameters:
//   - tableName: The name of the table for which column metadata is being requested.
//   - keySpace: The keyspace where the table resides.
//   - columnNames: A slice of SelectedColumns specifying the columns for which metadata is required.
//     If nil or empty, metadata for all columns in the table is returned.
//
// Returns:
//   - []*message.ColumnMetadata: A slice of pointers to ColumnMetadata structs containing metadata
//     for each requested column.
//   - error: Returns an error if the specified table is not found in Columns.
func (tableConfig *TableConfig) GetMetadataForSelectedColumns(columnNames []SelectedColumns) ([]*message.ColumnMetadata, error) {
	if len(columnNames) == 0 {
		return getAllColumnsMetadata(tableConfig.Columns), nil
	}
	return tableConfig.getSpecificColumnsMetadataForSelectedColumns(tableConfig.Columns, columnNames)
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

// getSpecificColumnsMetadataForSelectedColumns() generates column metadata for specifically selected columns.
// It handles regular columns, writetime columns, and special columns, and logs an error for invalid configurations.
//
// Parameters:
//   - columnsMap: A map where the keys are column names and the values are pointers to types.Column containing metadata.
//   - selectedColumns: A slice of SelectedColumns representing columns that have been selected for query.
//   - tableName: The name of the table from which columns are being selected.
//
// Returns:
//   - A slice of pointers to ColumnMetadata, representing the metadata for each selected column.
//   - An error if a column cannot be found in the map, or if there's an issue handling special columns.
func (tableConfig *TableConfig) getSpecificColumnsMetadataForSelectedColumns(columnsMap map[string]*types.Column, selectedColumns []SelectedColumns) ([]*message.ColumnMetadata, error) {
	var columnMetadataList []*message.ColumnMetadata
	var columnName string
	for i, columnMeta := range selectedColumns {
		columnName = columnMeta.Name

		if column, ok := columnsMap[columnName]; ok {
			columnMetadataList = append(columnMetadataList, cloneColumnMetadata(&column.Metadata, int32(i)))
		} else if columnMeta.IsWriteTimeColumn {
			metadata, err := handleSpecialColumn(columnsMap, getTimestampColumnName(columnMeta.Alias, columnMeta.ColumnName), int32(i), true)
			if err != nil {
				return nil, err
			}
			columnMetadataList = append(columnMetadataList, metadata)
		} else if isSpecialColumn(columnName) {
			metadata, err := handleSpecialColumn(columnsMap, columnName, int32(i), false)
			if err != nil {
				return nil, err
			}
			columnMetadataList = append(columnMetadataList, metadata)
		} else if columnMeta.IsFunc || columnMeta.MapKey != "" || columnMeta.IsWriteTimeColumn {
			metadata, err := tableConfig.handleSpecialSelectedColumn(columnsMap, columnMeta, int32(i))
			if err != nil {
				return nil, err
			}
			columnMetadataList = append(columnMetadataList, metadata)
		} else {
			return nil, fmt.Errorf("metadata not found for column `%s` in table `%s`", columnName, tableConfig.Name)
		}
		if columnMeta.IsAs {
			columnMetadataList[i].Name = columnMeta.Alias
		}
	}
	return columnMetadataList, nil
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

// getSpecificColumnsMetadata() retrieves metadata for specific columns in a given table.
//
// Parameters:
//   - columnsMap: column info for a given table.
//   - columnNames: column names for which the metadata is required.
//   - tableName: name of the table
//
// Returns:
// - A slice of pointers to ColumnMetadata structs containing metadata for each requested column.
// - An error
func (tableConfig *TableConfig) getSpecificColumnsMetadata(columnNames []string) ([]*message.ColumnMetadata, error) {
	var columnMetadataList []*message.ColumnMetadata
	for i, columnName := range columnNames {
		if column, ok := tableConfig.Columns[columnName]; ok {
			columnMetadataList = append(columnMetadataList, cloneColumnMetadata(&column.Metadata, int32(i)))
		} else if isSpecialColumn(columnName) {
			metadata, err := handleSpecialColumn(tableConfig.Columns, columnName, int32(i), false)
			if err != nil {
				return nil, err
			}
			columnMetadataList = append(columnMetadataList, metadata)
		} else {
			return nil, fmt.Errorf("metadata not found for column `%s` in table `%s`", columnName, tableConfig.Name)
		}
	}
	return columnMetadataList, nil
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

// handleSpecialSelectedColumn processes special column types and returns corresponding ColumnMetadata.
// It handles count functions, write time columns, aliased columns, function columns, and map columns.
//
// Parameters:
//   - columnsMap: A map of column names to their types.Column definitions
//   - columnSelected: Selected column configuration including special attributes
//   - index: The position index of the column
//   - tableName: Name of the table containing the column
//   - keySpace: Keyspace name containing the table
//
// Returns:
//   - *message.ColumnMetadata: Metadata for the processed column
//   - error: If the column is not found in the provided metadata
func (tableConfig *TableConfig) handleSpecialSelectedColumn(columnsMap map[string]*types.Column, columnSelected SelectedColumns, index int32) (*message.ColumnMetadata, error) {
	var cqlType datatype.DataType
	if columnSelected.FuncName == "count" || columnSelected.IsWriteTimeColumn {
		// For count function, the type is always bigint
		return &message.ColumnMetadata{
			Keyspace: tableConfig.Keyspace,
			Table:    tableConfig.Name,
			Type:     datatype.Bigint,
			Index:    index,
			Name:     columnSelected.Name,
		}, nil
	}
	lookupColumn := columnSelected.Name
	if columnSelected.Alias != "" || columnSelected.IsFunc || columnSelected.MapKey != "" {
		lookupColumn = columnSelected.ColumnName
	}
	column, ok := columnsMap[lookupColumn]
	if !ok {
		return nil, fmt.Errorf("special column %s not found in provided metadata", lookupColumn)
	}
	cqlType = column.Metadata.Type
	if columnSelected.MapKey != "" && cqlType.GetDataTypeCode() == primitive.DataTypeCodeMap {
		cqlType = cqlType.(datatype.MapType).GetValueType() // this gets the type of map value e.g map<varchar,int> -> datatype(int)
	}
	columnMd := &message.ColumnMetadata{
		Keyspace: tableConfig.Keyspace,
		Table:    tableConfig.Name,
		Type:     cqlType,
		Index:    index,
		Name:     columnSelected.Name,
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

// GetPkKeyType() returns the key type of a primary key column for a given table and keyspace.
// It takes the table name, keyspace name, and column name as input parameters.
// Returns the key type as a string if the column is a primary key, or an error if:
// - There's an error retrieving primary key information
// - The specified column is not a primary key in the table
func (tableConfig *TableConfig) GetPkKeyType(columnName string) (string, error) {
	for _, col := range tableConfig.PrimaryKeys {
		if col.Name == columnName {
			return col.KeyType, nil
		}
	}
	return "", fmt.Errorf("column %s is not a primary key in table %s", columnName, tableConfig.Name)
}

// ListKeyspaces returns a sorted list of all keyspace names in the schema mapping.
func (c *SchemaMappingConfig) ListKeyspaces() []string {
	keyspaces := make([]string, 0, len(c.Tables))
	for ks := range c.Tables {
		keyspaces = append(keyspaces, ks)
	}
	sort.Strings(keyspaces)
	return keyspaces
}
