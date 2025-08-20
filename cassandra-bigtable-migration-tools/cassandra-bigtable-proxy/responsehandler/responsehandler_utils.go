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
	"reflect"
	"strconv"

	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/third_party/datastax/proxycore"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
)

// HandleTimestampMap encodes map data with timestamps as keys and appends it to a message.Row.
// It first decodes the map data, determines the map type for the given element type, and then
// encodes the map data into a byte slice which is appended to the message.Row.
//
// Parameters:
//   - mapData: A map with timestamps as keys and byte slices as values, representing the data to encode.
//   - mr: A pointer to a message.Row where the encoded data will be appended.
//   - elementType: A string indicating the data type of the map's values.
//   - protocolV: The Cassandra protocol version used for encoding.
//
// Returns: An error if the encoding fails or if the map type retrieval does not succeed.
func (th *TypeHandler) HandleTimestampMap(mapData map[string]interface{}, mr *message.Row, mapType datatype.MapType, protocolV primitive.ProtocolVersion) error {
	var bytes []byte
	var err error

	detailsField, err := th.decodeMapData(mapData, mapType.GetValueType(), protocolV)
	if err != nil {
		return err
	}

	bytes, err = proxycore.EncodeType(mapType, protocolV, detailsField)
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
//   - protocolV: The Cassandra protocol version used for decoding.
//
// Returns: An interface{} containing the decoded map data, or an error if decoding fails or
//
//	if any key can't be parsed to an int64.
func (th *TypeHandler) decodeMapData(mapData map[string]interface{}, elementType datatype.DataType, protocolV primitive.ProtocolVersion) (interface{}, error) {
	result := make(map[int64]interface{})

	for key, value := range mapData {
		byteArray, ok := value.([]byte)
		if !ok {
			return nil, fmt.Errorf("type assertion to []byte failed")
		}

		decodedValue, err := th.DecodeValue(byteArray, elementType, protocolV)
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
//   - protocolV: The Cassandra protocol version used for decoding.
//
// Returns: An interface{} containing the decoded value and an error if the decoding fails
//
//	or if the element type is unsupported.
func (th *TypeHandler) DecodeValue(byteArray []byte, elementType datatype.DataType, protocolV primitive.ProtocolVersion) (interface{}, error) {
	var decodedValue interface{}
	var err error

	switch elementType {
	case datatype.Boolean:
		decodedValue, err = HandlePrimitiveEncoding(elementType, byteArray, protocolV, false)
	case datatype.Int:
		decodedValue, err = HandlePrimitiveEncoding(elementType, byteArray, protocolV, false)
	case datatype.Bigint:
		decodedValue, err = proxycore.DecodeType(datatype.Bigint, protocolV, byteArray)
	case datatype.Float:
		decodedValue, err = proxycore.DecodeType(datatype.Float, protocolV, byteArray)
	case datatype.Double:
		decodedValue, err = proxycore.DecodeType(datatype.Double, protocolV, byteArray)
	case datatype.Varchar:
		decodedValue = string(byteArray)
	case datatype.Timestamp:
		decodedValue, err = proxycore.DecodeType(datatype.Bigint, protocolV, byteArray)
	default:
		return nil, fmt.Errorf("unsupported element type: %v", elementType)
	}

	if err != nil {
		return nil, fmt.Errorf("error decoding value for element type %v: %v", elementType, err)
	}

	return decodedValue, nil
}

// DecodeAndReturnBool decodes a byte array to a boolean value.
//
// Parameters:
//   - btBytes: The byte array to be decoded.
//   - pv: The Cassandra protocol version.
//
// Returns: The decoded boolean value and an error if any.
func decodeAndReturnBool(value interface{}, pv primitive.ProtocolVersion) (bool, error) {
	switch v := value.(type) {
	case []byte:
		bv, err := proxycore.DecodeType(datatype.Bigint, pv, v)
		if err != nil {
			return false, fmt.Errorf("failed to retrieve int in the DecodeAndReturnBool function: %v", err)
		}
		bigint := bv.(int64)
		if bigint > 0 {
			return true, nil
		}
		return false, nil
	case string:
		val, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return false, fmt.Errorf("error converting string to int64: %w", err)
		}
		if val > 0 {
			return true, nil
		}
		return false, nil
	default:
		return false, fmt.Errorf("unsupported type: %T", v)
	}
}

/**
* DecodeAndReturnInt is a function that decodes a value to an int32.
*
* Parameters:
*   - value: The value to be decoded.
*   - pv: The Cassandra protocol version.
*
* Returns: The decoded int32 value and an error if any.
 */
func decodeAndReturnInt(value interface{}, pv primitive.ProtocolVersion) (int32, error) {
	switch v := value.(type) {
	case []byte:
		val, err := proxycore.DecodeType(datatype.Bigint, pv, v)
		if err != nil {
			return 0, fmt.Errorf("failed to retrieve int in the DecodeAndReturnBool function: %v", err)
		}
		return int32(val.(int64)), nil
	case string:
		val, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("error converting string to int64: %w", err)
		}
		return int32(val), nil
	case int64:
		// should be safe to convert to int32 because this value should've been written with int32 constraints
		return int32(value.(int64)), nil
	default:
		return 0, fmt.Errorf("unsupported type: %T", v)
	}
}

/**
* DecodeAndReturnBigInt is a function that decodes a value to an int64.
*
* Parameters:
*   - value: The value to be decoded.
*   - pv: The Cassandra protocol version.
*
* Returns: The decoded int64 value and an error if any.
 */
func decodeAndReturnBigInt(value interface{}, pv primitive.ProtocolVersion) (int64, error) {
	switch v := value.(type) {
	case []byte:
		value, err := proxycore.DecodeType(datatype.Bigint, pv, v)
		if err != nil {
			return 0, fmt.Errorf("failed to retrieve int in the DecodeAndReturnBigInt function: %v", err)
		}
		return value.(int64), nil
	case string:
		val, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("error converting string to int64: %w", err)
		}
		return val, nil
	case int64:
		return value.(int64), nil
	default:
		return 0, fmt.Errorf("unsupported type: %T", v)
	}
}

/**
* DecodeAndReturnFloat is a function that decodes a value to an int64.
*
* Parameters:
*   - value: The value to be decoded.
*   - pv: The Cassandra protocol version.
*
* Returns: The decoded float32 value and an error if any.
 */
func decodeAndReturnFloat(value interface{}, pv primitive.ProtocolVersion) (float32, error) {
	switch v := value.(type) {
	case []byte:
		value, err := proxycore.DecodeType(datatype.Float, pv, v)
		if err != nil {
			return 0, fmt.Errorf("failed to retrieve int in the DecodeAndReturnFloat function: %v", err)
		}
		return value.(float32), nil
	case string:
		val, err := strconv.ParseFloat(v, 32)
		if err != nil {
			return 0, fmt.Errorf("error converting string to int64: %w", err)
		}
		return float32(val), nil
	default:
		return 0, fmt.Errorf("unsupported type: %T", v)
	}
}

/**
* DecodeAndReturnDouble is a function that decodes a value to an float64.
*
* Parameters:
*   - value: The value to be decoded.
*   - pv: The Cassandra protocol version.
*
* Returns: The decoded float64 value and an error if any.
 */
func decodeAndReturnDouble(value interface{}, pv primitive.ProtocolVersion) (float64, error) {
	switch v := value.(type) {
	case []byte:
		value, err := proxycore.DecodeType(datatype.Double, pv, v)
		if err != nil {
			return 0, fmt.Errorf("failed to retrieve int in the DecodeAndReturnDouble function: %v", err)
		}
		return value.(float64), nil
	case string:
		val, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0, fmt.Errorf("error converting string to int64: %w", err)
		}
		return val, nil
	default:
		return 0, fmt.Errorf("unsupported type: %T", v)
	}
}

/**
* HandlePrimitiveEncoding is a function that encodes a value based on the cqlType.
*
* Parameters:
*   - cqlType: A string representing the type of the value.
*   - value: The value to be encoded.
*   - protocalVersion: The Cassandra protocol version.
*
* Returns: The encoded value and an error if any.
 */
func HandlePrimitiveEncoding(dt datatype.DataType, value interface{}, protocalVersion primitive.ProtocolVersion, encode bool) (interface{}, error) {
	val := reflect.ValueOf(value)
	if !val.IsValid() {
		return value, nil
	}
	if val.Kind() == reflect.Slice {
		if val.Len() == 0 {
			return nil, nil
		}
	}
	var decodedValue interface{}
	var err error
	if dt == datatype.Boolean {
		decodedValue, err = decodeAndReturnBool(value, protocalVersion)
	} else if dt == datatype.Int {
		decodedValue, err = decodeAndReturnInt(value, protocalVersion)
	} else if dt == datatype.Bigint || dt == datatype.Timestamp || dt == datatype.Counter {
		decodedValue, err = decodeAndReturnBigInt(value, protocalVersion)
	} else if dt == datatype.Float {
		decodedValue, err = decodeAndReturnFloat(value, protocalVersion)
	} else if dt == datatype.Double {
		decodedValue, err = decodeAndReturnDouble(value, protocalVersion)
	} else if dt == datatype.Varchar {
		byteArray, okByte := value.([]byte)
		stringValue, okString := value.(string)
		if !okByte && !okString {
			return nil, fmt.Errorf("value is not a byte array or string")
		}
		if okByte {
			decodedValue = string(byteArray)
		} else {
			decodedValue = stringValue
		}
	} else {
		return nil, fmt.Errorf("unsupported primitive type: %s", dt.String())
	}
	if err != nil {
		return nil, err
	}
	if encode {
		encoded, _ := proxycore.EncodeType(dt, protocalVersion, decodedValue)
		return encoded, nil
	}
	return decodedValue, nil
}
