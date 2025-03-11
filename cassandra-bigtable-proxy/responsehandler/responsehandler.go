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
	"slices"
	"strconv"
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
//	Returns an error if there is an issue unmarshaling the batch data or if no data is present.
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
	cf_len := len(cf)
	for i := 0; i < len(values); i += cf_len {
		rowMap := th.createRow(values, i, cf, query)
		id := fmt.Sprintf("%v", *rowCount)
		if rowMapData[id] == nil {
			rowMapData[id] = make(map[string]interface{})
		}
		if existingMap, ok := rowMapData[id]; ok {
			for k, v := range rowMap {
				existingMap[k] = v
			}
		} else {
			rowMapData[id] = rowMap
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
//	Special handling is included for key columns and writetime columns.
func (th *TypeHandler) createRow(values []*btpb.Value, start int, cf []*btpb.ColumnMetadata, query QueryMetadata) map[string]interface{} {
	cf_len := len(cf)
	var rowMap = make(map[string]interface{})
	for i := start; i < start+cf_len; i += 1 {
		isArray := IsArrayType(values[i])
		if !isArray && cf[start%cf_len].Name == "_key" {
			rowMap["_key"] = values[i].GetBytesValue()

		} else {
			pos := i % cf_len
			cf_name := cf[pos].Name
			cn := ""
			if !query.IsStar {
				if query.SelectedColumns[pos].Is_WriteTime_Column {
					if query.SelectedColumns[pos].Alias != "" {
						cn = query.SelectedColumns[pos].Alias
					} else {
						cn = query.SelectedColumns[pos].Name
					}
				} else {
					cn = query.SelectedColumns[pos].Name
				}
			}
			isWritetime := false
			if len(query.SelectedColumns) > pos {
				isWritetime = query.SelectedColumns[pos].Is_WriteTime_Column
			}
			th.processArray(values[i], &rowMap, cf_name, query, cn, isWritetime)
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
//   - cf_name: The name of the column family associated with the current processing context.
//   - query: QueryMetadata containing additional details about the query, such as default column family.
//   - cn: The column name string that may include an alias if specified in the query.
//   - isWritetime: A boolean flag indicating whether the current processing context is for a 'writetime' column.
//
// This function does not return a value; it directly modifies the provided rowMap with processed data.
func (th *TypeHandler) processArray(value *btpb.Value, rowMap *map[string]interface{}, cf_name string, query QueryMetadata, cn string, isWritetime bool) {
	if IsArrayType(value) {
		arr := value.GetArrayValue().Values
		length := len(arr)
		var key string
		var value []byte
		for i, v := range arr {
			if IsArrayType(v) {
				th.processArray(v, rowMap, cf_name, query, cn, isWritetime)
			} else if length == 2 && i == 0 {
				key = string(v.GetBytesValue())
			} else if length == 2 && i == 1 {
				value = v.GetBytesValue()
			}
		}
		if key != "" {
			if cf_name != th.SchemaMappingConfig.SystemColumnFamily {
				keyValMap := make(map[string]interface{})
				keyValMap[key] = value

				if (*rowMap)[cf_name] == nil {
					(*rowMap)[cf_name] = make([]Maptype, 0)
				}
				if existingMap, ok := (*rowMap)[cf_name].([]Maptype); ok {
					for k, v := range keyValMap {
						existingMap = append(existingMap, Maptype{Key: k, Value: v})
						// existingMap[k] = v
					}
					(*rowMap)[cf_name] = existingMap
				} else {
					(*rowMap)[cf_name] = keyValMap
				}
			} else {
				(*rowMap)[key] = value
			}
		}
	} else {
		cf := func() string {
			if IsFirstCharDollar(cf_name) {
				return query.DefaultColumnFamily
			}
			return cf_name
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
			if (*rowMap)[cf_name] == nil {
				(*rowMap)[cf_name] = make(map[string]interface{})
			}
			if existingMap, ok := (*rowMap)[cf_name].(map[string]interface{}); ok {
				for k, v := range keyValMap {
					existingMap[k] = v
				}
			} else {
				(*rowMap)[cf_name] = keyValMap
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
	UniqueColumns := ExtractUniqueKeys(rowMap)
	var aliasFound bool
	var aliasKeys []string
	i := 0
	if len(query.AliasMap) > 0 {
		aliasFound = true
		for k := range query.AliasMap {
			aliasKeys = append(aliasKeys, k)
		}
	}
	for column := range UniqueColumns {
		if column == rowkey {
			continue
		}
		var dt datatype.DataType
		var cqlType string
		var err error
		//checking if alias exists
		columnObj := GetQueryColumn(query, i, column)
		if columnObj.Is_WriteTime_Column {
			cqlType = "timestamp"
		} else if aliasFound && slices.Contains(aliasKeys, column) {
			cqlType, _, err = th.GetColumnMeta(query.AliasMap[column].Name, query.TableName, query.KeyspaceName)
		} else {
			cqlType, _, err = th.GetColumnMeta(column, query.TableName, query.KeyspaceName)
		}
		if err != nil {
			return nil, nil, err
		}
		cqlType = strings.ToLower(cqlType)
		dt, err = utilities.GetCassandraColumnType(cqlType)
		if err != nil {
			return nil, nil, err
		}
		mapKey := GetMapField(query, column)
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
	var aliasFound bool
	var aliasKeys []string
	if len(query.AliasMap) > 0 {
		aliasFound = true
		for k := range query.AliasMap {
			aliasKeys = append(aliasKeys, k)
		}
	}
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
		if col.Is_WriteTime_Column {
			cqlType = "timestamp"
			if aliasFound && slices.Contains(aliasKeys, key) {
				val := value.(map[string]interface{})
				value = val[key]
			}
		} else if aliasFound && slices.Contains(aliasKeys, key) {
			//checking if alias exists
			cqlType, isCollection, err = th.GetColumnMeta(query.AliasMap[key].Name, query.TableName, query.KeyspaceName)
			if !isCollection {
				val := value.(map[string]interface{})
				value = val[query.AliasMap[key].Name]
			}
		} else {
			cqlType, isCollection, err = th.GetColumnMeta(key, query.TableName, query.KeyspaceName)
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
			mr = append(mr, value.([]byte))
		}
	}
	return mr, nil
}

// GetColumnMeta retrieves the metadata for a column based on the provided query metadata.
// Parameters:
//   - columnName: Name of the column to get metadata for.
//   - tableName: Name of table in string.
//
// Returns: Column metadata as a string and an error if any.
func (th *TypeHandler) GetColumnMeta(columnName string, tableName string, keySpace string) (string, bool, error) {
	metaStr, err := th.SchemaMappingConfig.GetColumnType(tableName, columnName, keySpace)
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

	switch elementType {
	case "string", "text":
		dt = utilities.SetOfStr
		bytes, err = proxycore.EncodeType(dt, protocalV, arr)
	case "boolean":
		dt = utilities.SetOfBool
		newArr := []bool{}
		for _, key := range arr {
			boolVal, _ := strconv.ParseBool(key.(string))

			newArr = append(newArr, boolVal)
		}
		bytes, err = proxycore.EncodeType(dt, protocalV, newArr)

	case "int":
		dt = utilities.SetOfInt
		newArr := []int32{}
		for _, key := range arr {
			val, err := strconv.ParseInt(key.(string), 10, 32)
			if err != nil {
				return fmt.Errorf("error converting string to int32: %w", err)
			}
			boolVal := int32(val)
			newArr = append(newArr, boolVal)
		}
		bytes, err = proxycore.EncodeType(dt, protocalV, newArr)
	case "bigint":
		dt = utilities.SetOfBigInt
		newArr := []int64{}
		for _, key := range arr {

			val, err := strconv.ParseInt(key.(string), 10, 64)
			if err != nil {
				return fmt.Errorf("error converting string to int62: %w", err)
			}
			boolVal := int64(val)
			newArr = append(newArr, boolVal)
		}
		bytes, err = proxycore.EncodeType(dt, protocalV, newArr)

	case "float":
		dt = utilities.SetOfFloat
		newArr := []float32{}
		for _, key := range arr {
			val, err := strconv.ParseFloat(key.(string), 32)
			if err != nil {
				return fmt.Errorf("error converting string to float32: %w", err)
			}
			boolVal := float32(val)
			newArr = append(newArr, boolVal)
		}
		bytes, err = proxycore.EncodeType(dt, protocalV, newArr)

	case "double":
		dt = utilities.SetOfDouble
		newArr := []float64{}
		for _, key := range arr {
			val, err := strconv.ParseFloat(key.(string), 64)
			if err != nil {
				return fmt.Errorf("error converting string to float32: %w", err)
			}
			boolVal := float64(val)
			newArr = append(newArr, boolVal)
		}
		bytes, err = proxycore.EncodeType(dt, protocalV, newArr)

	case "timestamp":
		dt = utilities.SetOfTimeStamp
		newArr := []int64{}
		for _, key := range arr {
			val, err := strconv.ParseInt(key.(string), 10, 64)
			if err != nil {
				return fmt.Errorf("error converting string to float32: %w", err)
			}
			boolVal := int64(val)
			newArr = append(newArr, boolVal)
		}
		bytes, err = proxycore.EncodeType(dt, protocalV, newArr)

	default:
		return fmt.Errorf("unsupported array element type: %v", elementType)
	}

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

	switch elementType {
	case "boolean":
		detailsField := make(map[string]bool)
		for key, value := range mapData {
			byteArray, ok := value.([]byte)
			if !ok {
				return fmt.Errorf("type assertion to []byte failed for key: %s", key)
			}
			bv, decodeErr := proxycore.DecodeType(datatype.Boolean, protocalV, byteArray)
			if decodeErr != nil {
				return fmt.Errorf("error decoding boolean for key %s: %v", key, decodeErr)
			}
			detailsField[key] = bv.(bool)
		}
		mapType := utilities.MapOfStrToBool

		bytes, _ = proxycore.EncodeType(mapType, protocalV, detailsField)

	case "int":
		detailsField := make(map[string]int32)
		for key, value := range mapData {
			byteArray, ok := value.([]byte)
			if !ok {
				return fmt.Errorf("type assertion to []byte failed for key: %s", key)
			}
			bv, decodeErr := proxycore.DecodeType(datatype.Int, protocalV, byteArray)
			if decodeErr != nil {
				return fmt.Errorf("error decoding int for key %s: %v", key, decodeErr)
			}
			detailsField[key] = bv.(int32)

		}
		mapType := utilities.MapOfStrToInt
		bytes, _ = proxycore.EncodeType(mapType, protocalV, detailsField)

	case "bigint":
		detailsField := make(map[string]int64)
		for key, value := range mapData {
			byteArray, ok := value.([]byte)
			if !ok {
				return fmt.Errorf("type assertion to []byte failed for key: %s", key)
			}
			bv, decodeErr := proxycore.DecodeType(datatype.Bigint, protocalV, byteArray)
			if decodeErr != nil {
				return fmt.Errorf("error decoding bigint for key %s: %v", key, decodeErr)
			}
			detailsField[key] = bv.(int64)

		}
		mapType := utilities.MapOfStrToBigInt

		bytes, _ = proxycore.EncodeType(mapType, protocalV, detailsField)

	case "float":
		detailsField := make(map[string]float32)
		for key, value := range mapData {
			byteArray, ok := value.([]byte)
			if !ok {
				return fmt.Errorf("type assertion to []byte failed for key: %s", key)
			}
			bv, decodeErr := proxycore.DecodeType(datatype.Float, protocalV, byteArray)
			if decodeErr != nil {
				return fmt.Errorf("error decoding float for key %s: %v", key, decodeErr)
			}
			if bv == nil {
				return fmt.Errorf("decoded value is null for key %s", key)
			}
			detailsField[key] = bv.(float32)

		}
		mapType := utilities.MapOfStrToFloat

		bytes, _ = proxycore.EncodeType(mapType, protocalV, detailsField)

	case "double":
		detailsField := make(map[string]float64)
		for key, value := range mapData {
			byteArray, ok := value.([]byte)
			if !ok {
				return fmt.Errorf("type assertion to []byte failed for key: %s", key)
			}
			bv, decodeErr := proxycore.DecodeType(datatype.Double, protocalV, byteArray)
			if decodeErr != nil {
				return fmt.Errorf("error decoding double for key %s: %v", key, decodeErr)
			}
			detailsField[key] = bv.(float64)

		}
		mapType := utilities.MapOfStrToDouble

		bytes, _ = proxycore.EncodeType(mapType, protocalV, detailsField)

	case "string", "text":
		detailsField := make(map[string]string)

		for key, value := range mapData {
			byteArray, ok := value.([]byte)
			if !ok {
				return fmt.Errorf("type assertion to []byte failed for key: %s", key)
			}
			detailsField[key] = fmt.Sprintf("%v", string(byteArray))

		}
		mapType := utilities.MapOfStrToStr
		bytes, _ = proxycore.EncodeType(mapType, protocalV, detailsField)

	case "timestamp":
		detailsField := make(map[string]int64)
		for key, value := range mapData {
			byteArray, ok := value.([]byte)
			if !ok {
				return fmt.Errorf("type assertion to []byte failed for key: %s", key)
			}
			bv, decodeErr := proxycore.DecodeType(datatype.Bigint, protocalV, byteArray)
			if decodeErr != nil {
				return fmt.Errorf("error decoding timestamp for key %s: %v", key, decodeErr)
			}
			detailsField[key] = bv.(int64)

		}
		mapType := utilities.MapOfStrToBigInt

		bytes, _ = proxycore.EncodeType(mapType, protocalV, detailsField)

	default:
		return fmt.Errorf("unsupported MAP element type: %v", elementType)
	}

	*mr = append(*mr, bytes)
	return nil
}

func (th *TypeHandler) HandleListType(listData []interface{}, mr *message.Row, elementType string, protocalV primitive.ProtocolVersion) error {
	var bytes []byte

	switch elementType {
	case "boolean":
		detailsField := make([]bool, 0)
		for _, value := range listData {
			byteArray, ok := value.([]byte)
			if !ok {
				return fmt.Errorf("type assertion to []byte failed")
			}
			bv, decodeErr := proxycore.DecodeType(datatype.Boolean, protocalV, byteArray)
			if decodeErr != nil {
				return fmt.Errorf("error decoding boolean: %v", decodeErr)
			}
			detailsField = append(detailsField, bv.(bool))
		}
		listType := utilities.ListOfBool
		bytes, _ = proxycore.EncodeType(listType, protocalV, detailsField)

	case "int":
		detailsField := make([]int32, 0)
		for _, value := range listData {
			byteArray, ok := value.([]byte)
			if !ok {
				return fmt.Errorf("type assertion to []byte failed")
			}
			bv, decodeErr := proxycore.DecodeType(datatype.Int, protocalV, byteArray)
			if decodeErr != nil {
				return fmt.Errorf("error decoding int: %v", decodeErr)
			}
			detailsField = append(detailsField, bv.(int32))
		}
		listType := utilities.ListOfInt
		bytes, _ = proxycore.EncodeType(listType, protocalV, detailsField)

	case "bigint":
		detailsField := make([]int64, 0)
		for _, value := range listData {
			byteArray, ok := value.([]byte)
			if !ok {
				return fmt.Errorf("type assertion to []byte failed")
			}
			bv, decodeErr := proxycore.DecodeType(datatype.Bigint, protocalV, byteArray)
			if decodeErr != nil {
				return fmt.Errorf("error decoding bigint: %v", decodeErr)
			}
			detailsField = append(detailsField, bv.(int64))
		}
		listType := utilities.ListOfBigInt
		bytes, _ = proxycore.EncodeType(listType, protocalV, detailsField)

	case "float":
		detailsField := make([]float32, 0)
		for _, value := range listData {
			byteArray, ok := value.([]byte)
			if !ok {
				return fmt.Errorf("type assertion to []byte failed")
			}
			bv, decodeErr := proxycore.DecodeType(datatype.Float, protocalV, byteArray)
			if decodeErr != nil {
				return fmt.Errorf("error decoding float: %v", decodeErr)
			}
			if bv == nil {
				return fmt.Errorf("decoded value is null")
			}
			detailsField = append(detailsField, bv.(float32))
		}
		listType := utilities.ListOfFloat
		bytes, _ = proxycore.EncodeType(listType, protocalV, detailsField)

	case "double":
		detailsField := make([]float64, 0)
		for _, value := range listData {
			byteArray, ok := value.([]byte)
			if !ok {
				return fmt.Errorf("type assertion to []byte failed")
			}
			bv, decodeErr := proxycore.DecodeType(datatype.Double, protocalV, byteArray)
			if decodeErr != nil {
				return fmt.Errorf("error decoding double: %v", decodeErr)
			}
			detailsField = append(detailsField, bv.(float64))
		}
		listType := utilities.ListOfDouble
		bytes, _ = proxycore.EncodeType(listType, protocalV, detailsField)

	case "string", "text":
		detailsField := make([]string, 0)
		for _, value := range listData {
			byteArray, ok := value.([]byte)
			if !ok {
				return fmt.Errorf("type assertion to []byte failed")
			}
			detailsField = append(detailsField, string(byteArray))
		}
		listType := utilities.ListOfStr
		bytes, _ = proxycore.EncodeType(listType, protocalV, detailsField)

	case "timestamp":
		detailsField := make([]int64, 0)
		for _, value := range listData {
			byteArray, ok := value.([]byte)
			if !ok {
				return fmt.Errorf("type assertion to []byte failed")
			}
			bv, decodeErr := proxycore.DecodeType(datatype.Bigint, protocalV, byteArray)
			if decodeErr != nil {
				return fmt.Errorf("error decoding timestamp: %v", decodeErr)
			}
			detailsField = append(detailsField, bv.(int64))
		}
		listType := utilities.ListOfBigInt
		bytes, _ = proxycore.EncodeType(listType, protocalV, detailsField)

	default:
		return fmt.Errorf("unsupported LIST element type: %v", elementType)
	}

	*mr = append(*mr, bytes)
	return nil
}

// ExtractUniqueKeys extracts all unique keys from a nested map and returns them as a set of UniqueKeys
func ExtractUniqueKeys(rowMap map[string]map[string]interface{}) map[string]struct{} {
	uniqueKeys := make(map[string]struct{})

	for _, nestedMap := range rowMap {
		for key := range nestedMap {
			uniqueKeys[key] = struct{}{}
		}
	}

	return uniqueKeys
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
		if (selectedColumn.Is_WriteTime_Column && selectedColumn.Name == key) || (selectedColumn.Is_WriteTime_Column && selectedColumn.Alias == key) || selectedColumn.Name == key {
			return selectedColumn
		}

		for _, value := range query.SelectedColumns {
			if (value.Is_WriteTime_Column && value.Name == key) || (value.Is_WriteTime_Column && value.Alias == key) || (!value.Is_WriteTime_Column && value.Name == key) || (!value.Is_WriteTime_Column && value.Alias == key) {
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
