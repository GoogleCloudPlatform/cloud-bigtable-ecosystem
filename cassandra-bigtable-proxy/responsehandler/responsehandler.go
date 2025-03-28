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
	"strings"

	btpb "cloud.google.com/go/bigtable/apiv2/bigtablepb"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/ollionorg/cassandra-to-bigtable-proxy/proxycore"
	schemaMapping "github.com/ollionorg/cassandra-to-bigtable-proxy/schema-mapping"
	"github.com/ollionorg/cassandra-to-bigtable-proxy/utilities"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

const (
	rowkey = "_key"
)

type ResponseHandlerIface interface {
	GetRows(result *btpb.ExecuteQueryResponse_Results, cf []*btpb.ColumnMetadata, query QueryMetadata, rowCount *int, rowMapData map[string]map[string]interface{}) (map[string]map[string]interface{}, error)
	BuildMetadata(rowMap map[string]map[string]interface{}, query QueryMetadata) (cmd []*message.ColumnMetadata, mapKeyArr []string, err error)
	BuildResponseRow(rowMap map[string]interface{}, query QueryMetadata, cmd []*message.ColumnMetadata, mapKeyArray []string) (message.Row, error)
}

// GetRows processes ExecuteQueryResponse_Results and constructs a structured map of row data.
//
// Parameters:
//   - result: A pointer to ExecuteQueryResponse_Results containing the query results.
//   - cf: A slice of ColumnMetadata that describes the metadata of the columns returned in the query.
//   - query: QueryMetadata containing additional information about the executed query.
//
// Returns: A map where each key is a string representing a unique index for each row,
//
//	and the value is another map representing the column data for that row.
//	Returns an error if there is an issue unmarshalling the batch data or if no data is present.
func (th *TypeHandler) GetRows(result *btpb.ExecuteQueryResponse_Results, cf []*btpb.ColumnMetadata, query QueryMetadata, rowCount *int, rowMapData map[string]map[string]interface{}) (map[string]map[string]interface{}, error) {
	batch := result.Results.GetProtoRowsBatch()
	if batch == nil {
		return rowMapData, nil
	}
	protoRows := &btpb.ProtoRows{}
	err := proto.Unmarshal(batch.BatchData, protoRows)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal protorows: %v", err)
	}
	values := protoRows.Values
	cfLen := len(cf)
	for i := 0; i < len(values); i += cfLen {
		rowMap := th.createRow(values, i, cf, query)
		id := fmt.Sprintf("%v", *rowCount)
		if rowMapData[id] == nil {
			rowMapData[id] = make(map[string]interface{})
		}
		if existingMap, ok := rowMapData[id]; ok {
			for k, v := range rowMap {
				existingMap[k] = v
			}
		}

		(*rowCount)++
	}
	return rowMapData, nil
}

// createRow constructs a map representing a single row of data from the given values and column metadata.
//
// Parameters:
//   - values: A slice of Value pointers representing the values of the current row.
//   - start: An integer indicating the starting index in the values slice for the current row.
//   - cf: A slice of ColumnMetadata that provides metadata about the columns.
//   - query: QueryMetadata containing additional information about the executed query, such as selected columns.
//
// Returns: A map where keys are column names and values are the data from the corresponding value in the row.
//
//	Special handling is included for key columns and write time columns.
func (th *TypeHandler) createRow(values []*btpb.Value, start int, cf []*btpb.ColumnMetadata, query QueryMetadata) map[string]interface{} {
	cfLen := len(cf)
	var rowMap = make(map[string]interface{})
	for i := start; i < start+cfLen; i += 1 {
		isArray := IsArrayType(values[i])
		if !isArray && cf[start%cfLen].Name == rowkey {
			rowMap[rowkey] = values[i].GetBytesValue()

		} else {
			pos := i % cfLen
			cn := ""
			if !query.IsStar {
				if query.SelectedColumns[pos].IsWriteTimeColumn {
					if query.SelectedColumns[pos].Alias != "" {
						cn = query.SelectedColumns[pos].Alias
					} else {
						cn = query.SelectedColumns[pos].Name
					}
				} else {
					cn = query.SelectedColumns[pos].Name
				}
			}
			isWriteTime := false
			if len(query.SelectedColumns) > pos {
				isWriteTime = query.SelectedColumns[pos].IsWriteTimeColumn
			}
			th.processArray(values[i], &rowMap, cf[pos].Name, query, cn, isWriteTime)
		}
	}
	return rowMap
}

// IsArrayType determines if a given Value is of an array type.
//
// Parameters:
//   - value: A pointer to a Value, which represents a single data entry that may encapsulate different kinds of data types.
//
// Returns: A boolean indicating whether the provided Value is of type ArrayValue.
//
//	Returns true if the Value is an array; otherwise, returns false.
func IsArrayType(value *btpb.Value) bool {
	_, ok := value.Kind.(*btpb.Value_ArrayValue)
	return ok
}

// processArray processes a Value to update a given row map with array or single value data.
// This function handles nested arrays, key-value pairs, and special write-time handling.
//
// Parameters:
//   - value: A pointer to a Value, which represents the data entry to be processed, possibly as an array.
//   - rowMap: A pointer to a map storing row data, with keys as column names and values as the associated data.
//   - cfName: The name of the column family associated with the current processing context.
//   - query: QueryMetadata containing additional details about the query, such as default column family.
//   - cn: The column name string that may include an alias if specified in the query.
//   - isWritetime: A boolean flag indicating whether the current processing context is for a 'writetime' column.
//
// This function does not return a value; it directly modifies the provided rowMap with processed data.
func (th *TypeHandler) processArray(value *btpb.Value, rowMap *map[string]interface{}, cfName string, query QueryMetadata, cn string, isWritetime bool) {
	if IsArrayType(value) {
		arr := value.GetArrayValue().Values
		length := len(arr)
		var key string
		var byteValue []byte
		for i, v := range arr {
			if IsArrayType(v) {
				th.processArray(v, rowMap, cfName, query, cn, isWritetime)
			} else if length == 2 && i == 0 {
				key = string(v.GetBytesValue())
			} else if length == 2 && i == 1 {
				byteValue = v.GetBytesValue()
			}
		}
		if key != "" {
			if cfName != th.SchemaMappingConfig.SystemColumnFamily {
				keyValMap := make(map[string]interface{})
				keyValMap[key] = byteValue

				if (*rowMap)[cfName] == nil {
					(*rowMap)[cfName] = make([]Maptype, 0)
				}
				if existingMap, ok := (*rowMap)[cfName].([]Maptype); ok {
					for k, v := range keyValMap {
						existingMap = append(existingMap, Maptype{Key: k, Value: v})
					}
					(*rowMap)[cfName] = existingMap
				} else {
					(*rowMap)[cfName] = keyValMap
				}
			} else {
				(*rowMap)[key] = byteValue
			}
		}
	} else {
		cf := func() string {
			if HasDollarSymbolPrefix(cfName) {
				return query.DefaultColumnFamily
			}
			return cfName
		}()
		if cf != th.SchemaMappingConfig.SystemColumnFamily {
			keyValMap := make(map[string]interface{})
			if isWritetime {
				timestamp := value.GetTimestampValue().AsTime().UnixMicro()
				encoded, _ := proxycore.EncodeType(datatype.Timestamp, primitive.ProtocolVersion4, timestamp)
				keyValMap[cn] = encoded
			} else {
				keyValMap[cn] = value.GetBytesValue()
			}
			if (*rowMap)[cfName] == nil {
				(*rowMap)[cfName] = make(map[string]interface{})
			}
			if existingMap, ok := (*rowMap)[cfName].(map[string]interface{}); ok {
				for k, v := range keyValMap {
					existingMap[k] = v
				}
			} else {
				(*rowMap)[cfName] = keyValMap
			}
		} else {
			if isWritetime {
				timestamp := value.GetTimestampValue().AsTime().UnixMicro()
				encoded, _ := proxycore.EncodeType(datatype.Timestamp, primitive.ProtocolVersion4, timestamp)
				(*rowMap)[cn] = encoded
			} else {
				(*rowMap)[cn] = value.GetBytesValue()
			}
		}
	}
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
func (th *TypeHandler) BuildMetadata(rowMap map[string]map[string]interface{}, query QueryMetadata) (cmd []*message.ColumnMetadata, mapKeyArr []string, err error) {
	uniqueColumns := ExtractUniqueKeys(rowMap, query)
	i := 0
	for index := range uniqueColumns {
		column := uniqueColumns[index]
		if column == rowkey {
			continue
		}

		var cqlType string
		var err error
		//checking if alias exists
		columnObj := GetQueryColumn(query, i, column)
		if columnObj.IsWriteTimeColumn {
			cqlType = "timestamp"
		} else {
			lookupColumn := column
			if len(query.AliasMap) > 0 {
				if alias, exists := query.AliasMap[column]; exists {
					lookupColumn = alias.Name
				}
			}
			cqlType, _, err = th.GetColumnMeta(query.KeyspaceName, query.TableName, lookupColumn)
			if err != nil {
				return nil, nil, err
			}
		}

		dt, err := utilities.GetCassandraColumnType(cqlType)
		if err != nil {
			return nil, nil, err
		}

		mapKey := GetMapKeyForColumn(query, column)
		cmd = append(cmd, &message.ColumnMetadata{
			Keyspace: query.KeyspaceName,
			Table:    query.TableName,
			Name:     column,
			Index:    int32(i),
			Type:     dt,
		})
		mapKeyArr = append(mapKeyArr, mapKey)
		i++
	}
	return cmd, mapKeyArr, nil
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
func (th *TypeHandler) BuildResponseRow(rowMap map[string]interface{}, query QueryMetadata, cmd []*message.ColumnMetadata, mapKeyArray []string) (message.Row, error) {
	var mr message.Row
	for index, metaData := range cmd {
		key := metaData.Name
		mapKey := mapKeyArray[index]
		if rowMap[key] == nil {
			rowMap[key] = []byte{}
			mr = append(mr, []byte{})
			continue
		}
		value := rowMap[key]

		var cqlType string
		var err error
		var isCollection bool
		col := GetQueryColumn(query, index, key)
		if col.IsWriteTimeColumn {
			cqlType = "timestamp"
			if _, exists := query.AliasMap[key]; exists {
				val := value.(map[string]interface{})
				value = val[key]
			}
		} else if aliasKey, exists := query.AliasMap[key]; exists {
			cqlType, isCollection, err = th.GetColumnMeta(query.KeyspaceName, query.TableName, aliasKey.Name)
			if !isCollection {
				val := value.(map[string]interface{})
				value = val[aliasKey.Name]
			}
		} else {
			cqlType, isCollection, err = th.GetColumnMeta(query.KeyspaceName, query.TableName, key)
		}
		if err != nil {
			return nil, err
		}
		cqlType = strings.ToLower(cqlType)

		if isCollection {
			if strings.Contains(cqlType, "set") {
				// creating array
				setval := []interface{}{}
				for _, val := range value.([]Maptype) {
					setval = append(setval, val.Key)
				}
				startPos := strings.Index(cqlType, "<")
				endPos := strings.Index(cqlType, ">")
				content := cqlType[startPos+1 : endPos]
				elementType := strings.ToLower(strings.Trim(content, " "))

				err = th.HandleSetType(setval, &mr, elementType, query.ProtocalV)
				if err != nil {
					return nil, err
				}
			} else if strings.Contains(cqlType, "list") {
				listVal := []interface{}{}

				for _, val := range value.([]Maptype) {
					listVal = append(listVal, val.Value)
				}

				startPos := strings.Index(cqlType, "<")
				endPos := strings.Index(cqlType, ">")
				content := cqlType[startPos+1 : endPos]
				elementType := strings.ToLower(strings.Trim(content, " "))
				err = th.HandleListType(listVal, &mr, elementType, query.ProtocalV)
				if err != nil {
					return nil, err
				}
			} else if strings.Contains(cqlType, "map") {
				mapData := make(map[string]interface{})
				for _, val := range value.([]Maptype) {
					if mapKey != "" {
						if mapKey == val.Key {
							mapData[val.Key] = val.Value
						}
					} else {
						mapData[val.Key] = val.Value
					}
				}
				startPos := strings.Index(cqlType, ",")
				key := cqlType[4:startPos] // map< will have 4 characters at first
				endPos := strings.Index(cqlType, ">")
				content := cqlType[startPos+1 : endPos]
				elementType := strings.ToLower(strings.Trim(content, " "))
				if strings.ToLower(key) == "text" {
					err = th.HandleMapType(mapData, &mr, elementType, query.ProtocalV)
				} else if strings.ToLower(key) == "timestamp" {
					err = th.HandleTimestampMap(mapData, &mr, elementType, query.ProtocalV)
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
			mr = append(mr, value.([]byte))
		}
	}
	return mr, nil
}

// GetColumnMeta retrieves the metadata for a specified column within a given keyspace and table.
//
// Parameters:
//   - keyspace: The name of the keyspace where the table resides.
//   - tableName: The name of the table containing the column.
//   - columnName: The name of the column for which metadata is retrieved.
//
// Returns:
//   - A string representing the CQL type of the column.
//   - A boolean indicating whether the column is a collection type.
//   - An error if the metadata retrieval fails.
func (th *TypeHandler) GetColumnMeta(keyspace, tableName, columnName string) (string, bool, error) {
	metaStr, err := th.SchemaMappingConfig.GetColumnType(keyspace, tableName, columnName)
	if err != nil {
		return "", false, err
	}
	return metaStr.CQLType, metaStr.IsCollection, nil
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
func (th *TypeHandler) HandleSetType(arr []interface{}, mr *message.Row, elementType string, protocalV primitive.ProtocolVersion) error {
	var bytes []byte
	var err error
	var dt datatype.DataType

	dt, err = getSetDataType(elementType)
	if err != nil {
		return fmt.Errorf("error while getting set data type: %w", err)
	}
	newArr := []interface{}{}
	for _, key := range arr {
		boolVal, err := HandlePrimitiveEncoding(elementType, key, protocalV, false)
		if err != nil {
			return fmt.Errorf("error while decoding primitive type: %w", err)
		}
		newArr = append(newArr, boolVal)
	}
	bytes, err = proxycore.EncodeType(dt, protocalV, newArr)

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
//   - elementType: Type of the elements in the map.
//   - protocalV: Cassandra protocol version.
//
// Returns: Cassandra datatype and an error if any.
func (th *TypeHandler) HandleMapType(mapData map[string]interface{}, mr *message.Row, elementType string, protocalV primitive.ProtocolVersion) error {
	var bytes []byte

	maps := make(map[string]interface{})
	for key, value := range mapData {
		byteArray, ok := value.([]byte)
		if !ok {
			return fmt.Errorf("type assertion to []byte failed for key: %s", key)
		}
		bv, decodeErr := HandlePrimitiveEncoding(elementType, byteArray, protocalV, false)
		if decodeErr != nil {
			return fmt.Errorf("error decoding boolean for key %s: %v", key, decodeErr)
		}
		maps[key] = bv

	}

	mapType, err := getMapDataType(elementType)
	if err != nil {
		return fmt.Errorf("error while getting map data type: %w", err)
	}
	bytes, err = proxycore.EncodeType(mapType, protocalV, maps)
	if err != nil {
		return fmt.Errorf("error while encoding map type: %w", err)
	}

	*mr = append(*mr, bytes)
	return nil
}

func (th *TypeHandler) HandleListType(listData []interface{}, mr *message.Row, elementType string, protocalV primitive.ProtocolVersion) error {
	var bytes []byte

	list := make([]interface{}, 0)
	for _, value := range listData {
		byteArray, ok := value.([]byte)
		if !ok {
			return fmt.Errorf("type assertion to []byte failed")
		}
		bv, decodeErr := HandlePrimitiveEncoding(elementType, byteArray, protocalV, false)
		if decodeErr != nil {
			return fmt.Errorf("error decoding boolean: %v", decodeErr)
		}
		list = append(list, bv)

	}

	listType, err := getListDataType(elementType)
	if err != nil {
		return fmt.Errorf("error while getting list data type: %w", err)
	}
	bytes, err = proxycore.EncodeType(listType, protocalV, list)
	if err != nil {
		return fmt.Errorf("error while encoding map type: %w", err)
	}

	*mr = append(*mr, bytes)
	return nil
}

// ExtractUniqueKeys extracts all unique keys from a nested map and returns them as a set of UniqueKeys
func ExtractUniqueKeys(rowMap map[string]map[string]interface{}, query QueryMetadata) []string {
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
// - schemaMapping.SelectedColumns: The column object that matches the key, or an empty object if no match is found.
func GetQueryColumn(query QueryMetadata, index int, key string) schemaMapping.SelectedColumns {

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

	return schemaMapping.SelectedColumns{}
}

// function to encode rows - [][]interface{} to cassandra supported response formate [][][]bytes
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

func getSetDataType(cqlType string) (datatype.SetType, error) {
	switch cqlType {
	case "string", "text":
		return utilities.SetOfStr, nil
	case "int":
		return utilities.SetOfInt, nil
	case "bigint":
		return utilities.SetOfBigInt, nil
	case "float":
		return utilities.SetOfFloat, nil
	case "double":
		return utilities.SetOfDouble, nil
	case "timestamp":
		return utilities.SetOfTimeStamp, nil
	case "boolean":
		return utilities.SetOfBool, nil
	default:
		return nil, fmt.Errorf("unsupported array element type: %v", cqlType)
	}
}

func getMapDataType(cqlType string) (datatype.MapType, error) {
	switch cqlType {
	case "string", "text":
		return utilities.MapOfStrToStr, nil
	case "int":
		return utilities.MapOfStrToInt, nil
	case "bigint":
		return utilities.MapOfStrToBigInt, nil
	case "float":
		return utilities.MapOfStrToFloat, nil
	case "double":
		return utilities.MapOfStrToDouble, nil
	case "timestamp":
		return utilities.MapOfStrToBigInt, nil
	case "boolean":
		return utilities.MapOfStrToBool, nil
	default:
		return nil, fmt.Errorf("unsupported array element type: %v", cqlType)
	}
}
func getListDataType(cqlType string) (datatype.ListType, error) {
	switch cqlType {
	case "string", "text":
		return utilities.ListOfStr, nil
	case "int":
		return utilities.ListOfInt, nil
	case "bigint":
		return utilities.ListOfBigInt, nil
	case "float":
		return utilities.ListOfFloat, nil
	case "double":
		return utilities.ListOfDouble, nil
	case "timestamp":
		return utilities.ListOfBigInt, nil
	case "boolean":
		return utilities.ListOfBool, nil
	default:
		return nil, fmt.Errorf("unsupported array element type: %v", cqlType)
	}
}
