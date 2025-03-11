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
	"strconv"

	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/ollionorg/cassandra-to-bigtable-proxy/proxycore"
	"github.com/ollionorg/cassandra-to-bigtable-proxy/utilities"
)

// HandleTimestampMap encodes map data with timestamps as keys and appends it to a message.Row.
// It first decodes the map data, determines the map type for the given element type, and then
// encodes the map data into a byte slice which is appended to the message.Row.
//
// Parameters:
//   - mapData: A map with timestamps as keys and byte slices as values, representing the data to encode.
//   - mr: A pointer to a message.Row where the encoded data will be appended.
//   - elementType: A string indicating the data type of the map's values.
//   - protocalV: The Cassandra protocol version used for encoding.
//
// Returns: An error if the encoding fails or if the map type retrieval does not succeed.
func (th *TypeHandler) HandleTimestampMap(mapData map[string]interface{}, mr *message.Row, elementType string, protocalV primitive.ProtocolVersion) error {
	var bytes []byte
	var err error

	detailsField, err := th.decodeMapData(mapData, elementType, protocalV)
	if err != nil {
		return err
	}

	mapType, err := th.GetMapType(elementType)
	if err != nil {
		return err
	}

	bytes, err = proxycore.EncodeType(mapType, protocalV, detailsField)
	if err != nil {
		return fmt.Errorf("error encoding map data: %v", err)
	}

	*mr = append(*mr, bytes)
	return nil
}

// decodeMapData converts map data with string keys to a map with int64 keys, and decodes the values
// from byte slices to the specified element type. It interprets the string keys as timestamps.
//
// Parameters:
//   - mapData: A map with string keys and byte slice values, representing data to decode.
//   - elementType: A string specifying the type of the values in the map.
//   - protocalV: The Cassandra protocol version used for decoding.
//
// Returns: An interface{} containing the decoded map data, or an error if decoding fails or
//
//	if any key can't be parsed to an int64.
func (th *TypeHandler) decodeMapData(mapData map[string]interface{}, elementType string, protocalV primitive.ProtocolVersion) (interface{}, error) {
	result := make(map[int64]interface{})

	for key, value := range mapData {
		byteArray, ok := value.([]byte)
		if !ok {
			return nil, fmt.Errorf("type assertion to []byte failed")
		}

		decodedValue, err := th.DecodeValue(byteArray, elementType, protocalV)
		if err != nil {
			return nil, err
		}

		// Convert key as string to timestamp
		bigintKey, err := strconv.ParseInt(key, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("unable to parse key to int64")
		}
		result[bigintKey] = decodedValue
	}

	return result, nil
}

// DecodeValue decodes a byte slice into a specified data type based on the element type and protocol version.
//
// Parameters:
//   - byteArray: A byte slice representing the encoded data.
//   - elementType: A string indicating the desired type to decode the byte slice into.
//   - protocalV: The Cassandra protocol version used for decoding.
//
// Returns: An interface{} containing the decoded value and an error if the decoding fails
//
//	or if the element type is unsupported.
func (th *TypeHandler) DecodeValue(byteArray []byte, elementType string, protocalV primitive.ProtocolVersion) (interface{}, error) {
	var decodedValue interface{}
	var err error

	switch elementType {
	case "boolean":
		decodedValue, err = proxycore.DecodeType(datatype.Boolean, protocalV, byteArray)
	case "int":
		decodedValue, err = proxycore.DecodeType(datatype.Int, protocalV, byteArray)
	case "bigint":
		decodedValue, err = proxycore.DecodeType(datatype.Bigint, protocalV, byteArray)
	case "float":
		decodedValue, err = proxycore.DecodeType(datatype.Float, protocalV, byteArray)
	case "double":
		decodedValue, err = proxycore.DecodeType(datatype.Double, protocalV, byteArray)
	case "string", "text":
		decodedValue = string(byteArray)
	case "timestamp":
		decodedValue, err = proxycore.DecodeType(datatype.Bigint, protocalV, byteArray)
	default:
		return nil, fmt.Errorf("unsupported element type: %v", elementType)
	}

	if err != nil {
		return nil, fmt.Errorf("error decoding value for element type %v: %v", elementType, err)
	}

	return decodedValue, nil
}

// GetMapType retrieves the specified Cassandra map data type based on the given element type string.
//
// Parameters:
//   - elementType: A string indicating the type of values in the map.
//
// Returns: A datatype.DataType representing the map type and an error if the element type is unsupported.
func (th *TypeHandler) GetMapType(elementType string) (datatype.DataType, error) {
	switch elementType {
	case "boolean":
		return utilities.MapOfTimeToBool, nil
	case "int":
		return utilities.MapOfTimeToInt, nil
	case "bigint":
		return utilities.MapOfTimeToBigInt, nil
	case "float":
		return utilities.MapOfTimeToFloat, nil
	case "double":
		return utilities.MapOfTimeToDouble, nil
	case "string", "text":
		return utilities.MapOfTimeToStr, nil
	case "timestamp":
		return utilities.MapOfTimeToBigInt, nil
	default:
		return nil, fmt.Errorf("unsupported MAP element type: %v", elementType)
	}
}

// IsFirstCharDollar checks if the first character of a given string is a dollar sign.
//
// Parameters:
//   - s: The string to check.
//
// Returns: A boolean indicating whether the first character is a dollar sign.
func IsFirstCharDollar(s string) bool {
	if len(s) == 0 {
		return false
	}
	return s[0] == '$'
}

// GetMapField retrieves the map key associated with a specific column from the query metadata.
//
// Parameters:
//   - queryMetadata: The QueryMetadata containing information about selected columns.
//   - column: A string representing the column name for which to find the associated map key.
//
// Returns: A string containing the map key associated with the given column name, or an empty string if not found.
func GetMapField(queryMetadata QueryMetadata, column string) string {
	for _, value := range queryMetadata.SelectedColumns {
		if value.Name == column {
			return value.MapKey
		}
	}
	return ""
}
