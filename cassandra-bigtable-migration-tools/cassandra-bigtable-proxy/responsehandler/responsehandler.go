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

package responsehandler

import (
	"fmt"
	"sort"

	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/third_party/datastax/proxycore"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/utilities"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"go.uber.org/zap"
)

const (
	ROWKEY             = "_key"
	CQL_TYPE_BIGINT    = "bigint"
	CQL_TYPE_TIMESTAMP = "timestamp"
	FUNC_NAME_COUNT    = "count"
)

type ResponseHandlerIface interface {
	BuildMetadata(rowMap []map[string]interface{}, query QueryMetadata) (cmd []*message.ColumnMetadata, err error)
	BuildResponseRow(rowMap map[string]interface{}, query QueryMetadata, cmd []*message.ColumnMetadata) (message.Row, error)
}

// BuildMetadata constructs metadata for given row data based on query specifications.
//
// Parameters:
//   - rowMap: A map where the key is a string representing unique columns and the value is another map
//     representing the column's data.
//   - query: QueryMetadata containing additional information about the query, such as alias mappings and table details.
//
// Returns:
//   - cmd: A slice of pointers to ColumnMetadata, which describe the columns' metadata like keyspace, table,
//     name, index, and datatype.
//   - mapKeyArr: A slice of strings representing keys of the map fields corresponding to the query column names.
//   - err: An error if any occurs during the operation, particularly during metadata retrieval.
//
// This function uses helper functions to extract unique keys, handle aliases, and determine column types
func (th *TypeHandler) BuildMetadata(rowMap []map[string]interface{}, query QueryMetadata) (cmd []*message.ColumnMetadata, err error) {
	tableConfig, err := th.SchemaMappingConfig.GetTableConfig(query.KeyspaceName, query.TableName)
	if err != nil {
		return nil, err
	}
	if query.IsStar {
		return tableConfig.GetMetadataForSelectedColumns(query.SelectedColumns)
	}
	uniqueColumns := ExtractUniqueKeys(rowMap, query)
	i := 0
	for _, column := range uniqueColumns {
		if column == ROWKEY {
			continue
		}
		var cqlType datatype.DataType
		columnObj := GetQueryColumn(query, i, column)
		if columnObj.FuncName == FUNC_NAME_COUNT {
			cqlType = datatype.Bigint
		} else if columnObj.IsWriteTimeColumn {
			cqlType = datatype.Timestamp
		} else {
			lookupColumn := column
			if columnObj.Alias != "" || columnObj.IsFunc || columnObj.MapKey != "" {
				lookupColumn = columnObj.ColumnName
			}
			cqlType, err = tableConfig.GetColumnDataType(lookupColumn)
			if err != nil {
				return nil, err
			}
		}
		if columnObj.MapKey != "" && cqlType.GetDataTypeCode() == primitive.DataTypeCodeMap {
			cqlType = cqlType.(datatype.MapType).GetValueType() // this gets the type of map value e.g map<varchar,int> -> datatype(int)
		}
		cmd = append(cmd, &message.ColumnMetadata{
			Keyspace: query.KeyspaceName,
			Table:    query.TableName,
			Name:     column,
			Index:    int32(i),
			Type:     cqlType,
		})
		i++
	}
	return cmd, nil
}

// BuildResponseRow constructs a message.Row from a given row map based on column metadata and query specification.
// It handles alias mappings, collections like sets, lists, and maps, and encodes values appropriately.
//
// Parameters:
//   - rowMap: A map with column names as keys and their corresponding data as values.
//   - query: QueryMetadata containing additional details about the query, such as alias mappings and protocol version.
//   - cmd: A slice of pointers to ColumnMetadata that describe details for each column, such as type and name.
//   - mapKeyArray: A slice of strings representing keys of the map fields corresponding to the query column names.
//
// Returns:
//   - mr: A message.Row containing the serialized row data.
//   - err: An error if any occurs during the construction of the row, especially in data retrieval or encoding.
func (th *TypeHandler) BuildResponseRow(rowMap map[string]interface{}, query QueryMetadata, cmd []*message.ColumnMetadata) (message.Row, error) {
	tableConfig, err := th.SchemaMappingConfig.GetTableConfig(query.KeyspaceName, query.TableName)
	if err != nil {
		return nil, err
	}
	var mr message.Row
	for index, metaData := range cmd {
		key := metaData.Name
		if rowMap[key] == nil {
			rowMap[key] = []byte{}
			mr = append(mr, nil)
			continue
		}
		value := rowMap[key]

		var cqlType datatype.DataType
		var err error
		col := GetQueryColumn(query, index, key)
		if col.FuncName == FUNC_NAME_COUNT {
			cqlType = datatype.Bigint
		} else if col.IsWriteTimeColumn {
			cqlType = datatype.Timestamp
		} else if col.IsFunc || col.MapKey != "" || col.IsAs {
			cqlType, err = tableConfig.GetColumnDataType(col.ColumnName)
			if err != nil {
				return nil, err
			}
		} else {
			cqlType, err = tableConfig.GetColumnDataType(key)
			if err != nil {
				return nil, err
			}
		}
		if err != nil {
			return nil, err
		}
		isCollection := utilities.IsCollection(cqlType)
		if col.IsFunc || query.IsGroupBy {
			if col.FuncName == FUNC_NAME_COUNT {
				cqlType = datatype.Bigint
				metaData.Type = datatype.Bigint
			}
			if value == nil {
				value = float64(0)
			}
			var dt datatype.DataType
			var val interface{}
			// For aggregate functions, it specifically handles:
			//   - int64 values: converts to bigint or int based on CQL type
			//   - float64 values: converts to bigint, int, float, or double based on CQL type.
			// Ensuring compatibility between BigTable and Cassandra type systems for aggregated results.
			switch v := value.(type) {
			case int64:
				switch cqlType {
				case datatype.Bigint, datatype.Counter:
					val = v
					dt = datatype.Bigint
				case datatype.Int:
					val = int32(v)
					dt = datatype.Int
				default:
					return nil, fmt.Errorf("invalid cqlType - value received type: %v, CqlType: %s", v, cqlType)
				}
			case float64:
				switch cqlType {
				case datatype.Bigint:
					val = int64(v)
					dt = datatype.Bigint
				case datatype.Int:
					val = int32(v)
					dt = datatype.Int
				case datatype.Float:
					val = float32(v)
					dt = datatype.Float
				case datatype.Double:
					val = v
					dt = datatype.Double
				default:
					return nil, fmt.Errorf("invalid cqlType - value recieved type: %v, CqlType: %s", v, cqlType)
				}
			case []byte:
				mr = append(mr, v)
				continue
			default:
				return nil, fmt.Errorf("unsupported value type received in Bigtable: %v, value: %v, type: %T", cqlType, value, v)
			}
			encoded, err := proxycore.EncodeType(dt, primitive.ProtocolVersion4, val)
			if err != nil {
				return nil, fmt.Errorf("failed to encode value: %v", err)
			}
			value = encoded

			mr = append(mr, value.([]byte))
			continue
		}
		if err != nil {
			return nil, err
		}
		if isCollection {
			if cqlType.GetDataTypeCode() == primitive.DataTypeCodeSet {
				setType := cqlType.(datatype.SetType)
				// creating array
				setval := []interface{}{}
				for _, val := range value.([]Maptype) {
					setval = append(setval, val.Key)
				}
				if len(setval) == 0 {
					mr = append(mr, nil)
					continue
				}
				err = th.HandleSetType(setval, &mr, setType, query.ProtocalV)
				if err != nil {
					return nil, err
				}
			} else if cqlType.GetDataTypeCode() == primitive.DataTypeCodeList {
				listType := cqlType.(datatype.ListType)
				listVal := []interface{}{}
				for _, val := range value.([]Maptype) {
					listVal = append(listVal, val.Value)
				}
				if len(listVal) == 0 {
					mr = append(mr, nil)
					continue
				}
				err = th.HandleListType(listVal, &mr, listType, query.ProtocalV)
				if err != nil {
					return nil, err
				}
			} else if cqlType.GetDataTypeCode() == primitive.DataTypeCodeMap {
				mapType := cqlType.(datatype.MapType)
				mapData := make(map[string]interface{})

				if col.MapKey != "" {
					if value == nil {
						mr = append(mr, nil)
						continue
					}
					mr = append(mr, value.([]byte))
					continue
				}

				for _, val := range value.([]Maptype) {
					mapData[val.Key] = val.Value
				}
				if len(mapData) == 0 {
					mr = append(mr, nil)
					continue
				}

				if mapType.GetKeyType() == datatype.Varchar {
					err = th.HandleMapType(mapData, &mr, mapType, query.ProtocalV)
				} else if mapType.GetKeyType() == datatype.Timestamp {
					err = th.HandleTimestampMap(mapData, &mr, mapType, query.ProtocalV)
				}
				if err != nil {
					th.Logger.Error("Error while Encoding json->bytes -> ", zap.Error(err))
					return nil, fmt.Errorf("failed to retrieve Map data: %v", err)
				}
			}
		} else {
			value, err = HandlePrimitiveEncoding(cqlType, value, query.ProtocalV, true)
			if err != nil {
				return nil, err
			}
			if value == nil {
				mr = append(mr, nil)
				continue
			}
			mr = append(mr, value.([]byte))
		}
	}
	return mr, nil
}

// HandleSetType converts a array or set type to a Cassandra  set type.
//
// Parameters:
//   - arr: arr of interface of type given by elementType.
//   - mr: Pointer to the message.Row to append the converted data.
//   - elementType: type of the elements in the array.
//   - protocalV: Cassandra protocol version.
//
// Returns: Cassandra datatype and an error if any.
func (th *TypeHandler) HandleSetType(arr []interface{}, mr *message.Row, setType datatype.SetType, protocalV primitive.ProtocolVersion) error {
	newArr := []interface{}{}
	for _, key := range arr {
		boolVal, err := HandlePrimitiveEncoding(setType.GetElementType(), key, protocalV, false)
		if err != nil {
			return fmt.Errorf("error while decoding primitive type: %w", err)
		}
		newArr = append(newArr, boolVal)
	}
	bytes, err := proxycore.EncodeType(setType, protocalV, newArr)
	if err != nil {
		th.Logger.Error("Error while Encoding -> ", zap.Error(err))
	}
	*mr = append(*mr, bytes)
	return nil
}

// HandleMapType converts a map type to a Cassandra map type.
//
// Parameters:
//   - mapData: map of string and interface{} of elementType of data.
//   - mr: Pointer to the message.Row to append the converted data.
//   - elementType: CQLType of the elements in the map.
//   - protocalV: Cassandra protocol version.
//
// Returns: Cassandra datatype and an error if any.
func (th *TypeHandler) HandleMapType(mapData map[string]interface{}, mr *message.Row, mapType datatype.MapType, protocalV primitive.ProtocolVersion) error {
	var bytes []byte

	maps := make(map[string]interface{})
	for key, value := range mapData {
		byteArray, ok := value.([]byte)
		if !ok {
			return fmt.Errorf("type assertion to []byte failed for key: %s", key)
		}
		bv, decodeErr := HandlePrimitiveEncoding(mapType.GetValueType(), byteArray, protocalV, false)
		if decodeErr != nil {
			return fmt.Errorf("error decoding map value for key %s: %w", key, decodeErr)
		}
		maps[key] = bv

	}

	bytes, err := proxycore.EncodeType(mapType, protocalV, maps)
	if err != nil {
		return fmt.Errorf("error while encoding map type: %w", err)
	}

	*mr = append(*mr, bytes)
	return nil
}

func (th *TypeHandler) HandleListType(listData []interface{}, mr *message.Row, listType datatype.ListType, protocalV primitive.ProtocolVersion) error {
	list := make([]interface{}, 0)
	for i, value := range listData {
		byteArray, ok := value.([]byte)
		if !ok {
			return fmt.Errorf("type assertion to []byte failed")
		}
		bv, decodeErr := HandlePrimitiveEncoding(listType.GetElementType(), byteArray, protocalV, false)
		if decodeErr != nil {
			return fmt.Errorf("error decoding list element at position %d: %w", i, decodeErr)
		}
		list = append(list, bv)
	}

	bytes, err := proxycore.EncodeType(listType, protocalV, list)
	if err != nil {
		return fmt.Errorf("error while encoding map type: %w", err)
	}

	*mr = append(*mr, bytes)
	return nil
}

// ExtractUniqueKeys extracts all unique keys from a nested map and returns them as a set of UniqueKeys
func ExtractUniqueKeys(rowMap []map[string]interface{}, query QueryMetadata) []string {
	columns := []string{}
	if query.IsStar {
		uniqueKeys := make(map[string]bool)
		for _, nestedMap := range rowMap {
			for key := range nestedMap {
				if !uniqueKeys[key] {
					columns = append(columns, key)
				}
				uniqueKeys[key] = true
			}
		}
		sort.Strings(columns)
		return columns
	}
	for _, column := range query.SelectedColumns {
		if column.Alias != "" {
			columns = append(columns, column.Alias)
		} else {
			columns = append(columns, column.Name)
		}
	}
	return columns
}

// GetQueryColumn retrieves a specific column from the QueryMetadata based on a key.
//
// Parameters:
// - query (QueryMetadata): The metadata object containing information about the selected columns.
// - index (int): The index of a specific column to check first for a match.
// - key (string): The key to match against the column's Alias or Name.
//
// Returns:
// - types: The column object that matches the key, or an empty object if no match is found.
func GetQueryColumn(query QueryMetadata, index int, key string) types.SelectedColumn {

	if len(query.SelectedColumns) > 0 {
		selectedColumn := query.SelectedColumns[index]
		if (selectedColumn.IsWriteTimeColumn && selectedColumn.Name == key) || (selectedColumn.IsWriteTimeColumn && selectedColumn.Alias == key) || selectedColumn.Name == key {
			return selectedColumn
		}

		for _, value := range query.SelectedColumns {
			if (value.IsWriteTimeColumn && value.Name == key) || (value.IsWriteTimeColumn && value.Alias == key) || (!value.IsWriteTimeColumn && value.Name == key) || (!value.IsWriteTimeColumn && value.Alias == key) {
				return value
			}
		}
	}

	return types.SelectedColumn{}
}

// BuildResponseForSystemQueries function to encode rows - [][]interface{} to cassandra supported response formate [][][]bytes
func BuildResponseForSystemQueries(rows [][]interface{}, protocalV primitive.ProtocolVersion) ([]message.Row, error) {
	var allRows []message.Row
	for _, row := range rows {
		var mr message.Row
		for _, val := range row {
			encodedByte, err := utilities.TypeConversion(val, protocalV)
			if err != nil {
				return allRows, err
			}
			mr = append(mr, encodedByte)
		}
		allRows = append(allRows, mr)
	}
	return allRows, nil
}
