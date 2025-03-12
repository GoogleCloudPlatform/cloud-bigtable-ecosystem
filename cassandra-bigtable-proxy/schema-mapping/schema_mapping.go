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

	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"go.uber.org/zap"
)

const (
	limitValue = "limitValue"
)

type Column struct {
	CQLType      string
	ColumnName   string
	ColumnType   string
	IsPrimaryKey bool
	PkPrecedence int64
	IsCollection bool
	KeyType      string
	Metadata     message.ColumnMetadata
}

type SchemaMappingConfig struct {
	Logger             *zap.Logger
	TablesMetaData     map[string]map[string]map[string]*Column
	PkMetadataCache    map[string]map[string][]Column
	SystemColumnFamily string
}

type ColumnType struct {
	CQLType      string
	IsPrimaryKey bool
	ColumnFamily string
	IsCollection bool
	KeyType      string
}

type SelectedColumns struct {
	FormattedColumn     string
	Name                string
	IsFunc              bool
	IsAs                bool
	FuncName            string
	Alias               string
	MapKey              string
	ListIndex           string
	WriteTime_Column    string
	KeyType             string
	Is_WriteTime_Column bool
}

// GetPkByTableName() searches for and retrieves the primary keys of a specific table
//
// Parameters:
//   - tableName: A string representing the name of the table for which the metadata
//     is requested.
//
// Returns:
//   - []Column: Array of Column details which are Primary Keys.
//   - An error: Returns error if function not able to search any primary keys
func (c *SchemaMappingConfig) GetPkByTableName(tableName string, keySpace string) ([]Column, error) {
	pkMeta, ok := c.PkMetadataCache[keySpace][tableName]
	if !ok {
		return nil, fmt.Errorf("could not find table metadata")
	}
	return pkMeta, nil
}

// GetColumnType() retrieves the column types for a specified column in a specified table.
// This method is a part of the SchemaMappingConfig struct and is used to map column types
// from Cassandra (CQL) to Google Cloud Bigtable.
//
// Parameters:
// - tableName: A string representing the name of the table.
// - columnName: A string representing the name of the column within the specified table.
//
// Returns:
//   - A pointer to a ColumnType struct that contains both the CQL type and the corresponding
//     cassandra type for the specified column.
//   - An error if the function is unable to determine the corresponding cassandra type.
func (c *SchemaMappingConfig) GetColumnType(tableName string, columnName string, keySpace string) (*ColumnType, error) {

	td, ok := c.TablesMetaData[keySpace][tableName]
	if !ok {
		return nil, fmt.Errorf("could not find table metadata")
	}

	col, ok := td[columnName]
	if !ok {
		return nil, fmt.Errorf("could not find column metadata")
	}

	return &ColumnType{
		CQLType:      col.ColumnType,
		IsPrimaryKey: col.IsPrimaryKey,
		IsCollection: col.IsCollection,
		KeyType:      col.KeyType,
	}, nil
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
// - An error if the specified table is not found in the TablesMetaData.
func (c *SchemaMappingConfig) GetMetadataForColumns(tableName string, columnNames []string, keySpace string) ([]*message.ColumnMetadata, error) {
	columnsMap, ok := c.TablesMetaData[keySpace][tableName]
	if !ok {
		c.Logger.Error("Table not found in Column MetaData Cache")
		return nil, fmt.Errorf("table %s not found", tableName)
	}

	if len(columnNames) == 0 {
		return c.getAllColumnsMetadata(columnsMap), nil
	}

	return c.getSpecificColumnsMetadata(columnsMap, columnNames, tableName)
}

// GetMetadataForSelectedColumns() simply fetching column metadata for selected columns .
//
// Parameters:
//   - tableName: The name of the table for which column metadata is being requested.
//   - columnNames(optional): []SelectedColumns - Accepts nil if no columnNames provided or else A slice of strings containing the names of the columns for which
//     metadata is required. If this slice is empty, metadata for all
//     columns in the table will be returned.
//
// Returns:
// - A slice of pointers to ColumnMetadata structs containing metadata for each requested column.
// - An error if the specified table is not found in the TablesMetaData.
func (c *SchemaMappingConfig) GetMetadataForSelectedColumns(tableName string, columnNames []SelectedColumns, keySpace string) ([]*message.ColumnMetadata, error) {
	columnsMap, ok := c.TablesMetaData[keySpace][tableName]
	if !ok {
		c.Logger.Error("Table not found in Column MetaData Cache")
		return nil, fmt.Errorf("table %s not found", tableName)
	}

	if len(columnNames) == 0 {
		return c.getAllColumnsMetadata(columnsMap), nil
	}
	return c.getSpecificColumnsMetadataForSelectedColumns(columnsMap, columnNames, tableName)
}

// getTimestampColumnName() constructs the appropriate name for a column used in a writetime function.
//
// Parameters:
//   - aliasName: A string representing the alias of the column, if any.
//   - columnName: The actual name of the column for which the writetime function is being constructed.
//
// Returns: A string which is either the alias name or the expression "writetime(columnName)"
//
//	if no alias is provided.
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
//   - columnsMap: A map where the keys are column names and the values are pointers to Column containing metadata.
//   - selectedColumns: A slice of SelectedColumns representing columns that have been selected for query.
//   - tableName: The name of the table from which columns are being selected.
//
// Returns:
//   - A slice of pointers to ColumnMetadata, representing the metadata for each selected column.
//   - An error if a column cannot be found in the map, or if there's an issue handling special columns.
func (c *SchemaMappingConfig) getSpecificColumnsMetadataForSelectedColumns(columnsMap map[string]*Column, selectedColumns []SelectedColumns, tableName string) ([]*message.ColumnMetadata, error) {
	var columnMetadataList []*message.ColumnMetadata
	var columnName string
	for i, columnMeta := range selectedColumns {
		columnName = columnMeta.Name

		if column, ok := columnsMap[columnName]; ok {
			columnMetadataList = append(columnMetadataList, c.cloneColumnMetadata(&column.Metadata, int32(i)))
		} else if columnMeta.Is_WriteTime_Column {
			metadata, err := c.handleSpecialColumn(columnsMap, getTimestampColumnName(columnMeta.Alias, columnMeta.WriteTime_Column), int32(i), true)
			if err != nil {
				return nil, err
			}
			columnMetadataList = append(columnMetadataList, metadata)
		} else if isSpecialColumn(columnName) {
			metadata, err := c.handleSpecialColumn(columnsMap, columnName, int32(i), false)
			if err != nil {
				return nil, err
			}
			columnMetadataList = append(columnMetadataList, metadata)
		} else if columnMeta.IsFunc {
			c.Logger.Debug("Identified a function call", zap.String("columnName", columnName))
		} else {
			errMsg := fmt.Sprintf("table = `%s` column name = `%s` not found", tableName, columnName)
			c.Logger.Error(errMsg)
			return nil, fmt.Errorf("%s", errMsg)
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
func (c *SchemaMappingConfig) getAllColumnsMetadata(columnsMap map[string]*Column) []*message.ColumnMetadata {
	var columnMetadataList []*message.ColumnMetadata
	var i int32 = 0
	for _, column := range columnsMap {
		columnMt := column.Metadata
		columnMd := columnMt.Clone()
		columnMd.Index = i
		columnMetadataList = append(columnMetadataList, columnMd)
		i++
	}
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
func (c *SchemaMappingConfig) getSpecificColumnsMetadata(columnsMap map[string]*Column, columnNames []string, tableName string) ([]*message.ColumnMetadata, error) {
	var columnMetadataList []*message.ColumnMetadata
	for i, columnName := range columnNames {
		if column, ok := columnsMap[columnName]; ok {
			columnMetadataList = append(columnMetadataList, c.cloneColumnMetadata(&column.Metadata, int32(i)))
		} else if isSpecialColumn(columnName) {
			metadata, err := c.handleSpecialColumn(columnsMap, columnName, int32(i), false)
			if err != nil {
				return nil, err
			}
			columnMetadataList = append(columnMetadataList, metadata)
		} else {
			errMsg := fmt.Sprintf("table = `%s` column name = `%s` not found", tableName, columnName)
			c.Logger.Error(errMsg)
			return nil, fmt.Errorf("%s", errMsg)
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
func (c *SchemaMappingConfig) handleSpecialColumn(columnsMap map[string]*Column, columnName string, index int32, iswriteTimeFunction bool) (*message.ColumnMetadata, error) {
	var expectedType datatype.DataType
	if columnName == limitValue {
		expectedType = datatype.Bigint
	} else if iswriteTimeFunction {
		expectedType = datatype.Bigint
	}
	for _, column := range columnsMap {
		columnMd := column.Metadata.Clone()
		columnMd.Index = index
		columnMd.Name = columnName
		columnMd.Type = expectedType
		return columnMd, nil
	}
	return nil, fmt.Errorf("special column %s not found", columnName)
}

// cloneColumnMetadata() clones the metadata from cache.
//
// Parameters:
//   - metadata: Column metadata from cache
//   - index: Index for the column
//
// Returns:
// - Pointers to ColumnMetadata structs containing metadata for each requested column.
func (c *SchemaMappingConfig) cloneColumnMetadata(metadata *message.ColumnMetadata, index int32) *message.ColumnMetadata {
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
	return columnName == limitValue
}

func (c *SchemaMappingConfig) InstanceExist(keyspace string) bool {
	_, ok := c.TablesMetaData[keyspace]
	return ok
}

func (c *SchemaMappingConfig) TableExist(keyspace string, tableName string) bool {
	_, ok := c.TablesMetaData[keyspace][tableName]
	return ok
}

// GetPkKeyType() returns the key type of a primary key column for a given table and keyspace.
// It takes the table name, keyspace name, and column name as input parameters.
// Returns the key type as a string if the column is a primary key, or an error if:
// - There's an error retrieving primary key information
// - The specified column is not a primary key in the table
func (c *SchemaMappingConfig) GetPkKeyType(tableName string, keySpace string, columnName string) (string, error) {
	pkColumns, err := c.GetPkByTableName(tableName, keySpace)
	if err != nil {
		return "", err
	}
	for _, col := range pkColumns {
		if col.ColumnName == columnName {
			return col.KeyType, nil
		}
	}
	return "", fmt.Errorf("column %s is not a primary key in table %s", columnName, tableName)
}
