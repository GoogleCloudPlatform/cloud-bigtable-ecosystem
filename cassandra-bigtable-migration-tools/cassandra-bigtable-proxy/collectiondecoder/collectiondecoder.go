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

package collectiondecoder

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/third_party/datastax/proxycore"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
)

// DecodeCollection decodes a collection (list, set, or map) of elements from the provided byte array.
// It reads the collection length, then iterates over the encoded elements to decode them based on their data type.
// Parameters:
// - encoded: The byte array containing the encoded collection data.
// - version: The Cassandra protocol version used for encoding.
// - dt: The data type of the collection elements.
// Returns:
// - An interface{} containing the decoded collection as a slice or map of the appropriate type.
// - An error if any decoding step fails.
func DecodeCollection(dt datatype.DataType, encoded []byte, qctx *types.QueryContext) (interface{}, error) {
	reader := bytes.NewReader(encoded)
	var length int32

	// Read the collection length (4 bytes)
	err := binary.Read(reader, binary.BigEndian, &length)
	if err != nil {
		return nil, err
	}

	switch dt.GetDataTypeCode() {
	case primitive.DataTypeCodeList:
		listType := dt.(datatype.ListType)
		return decodeListOrSet(listType.GetElementType(), qctx, reader, length)
	case primitive.DataTypeCodeSet:
		setType := dt.(datatype.SetType)
		return decodeListOrSet(setType.GetElementType(), qctx, reader, length)
	case primitive.DataTypeCodeMap:
		mapType := dt.(datatype.MapType)
		return decodeMap(mapType.GetValueType(), qctx, reader, mapType.GetKeyType(), length)
	default:
		return nil, fmt.Errorf("unsupported collection type: %v", dt.GetDataTypeCode())
	}
}

// decodeListOrSet decodes a list or set of elements from the provided byte reader.
// It reads each element's length and value, then decodes the value based on the specified element data type.
// Parameters:
// - reader: A byte reader positioned at the start of the encoded elements.
// - version: The Cassandra protocol version used for encoding.
// - elementType: The data type of the elements in the list or set.
// - length: The number of elements in the collection.
// Returns:
// - An interface{} containing the decoded elements as a slice of the appropriate type.
// - An error if any decoding step fails.
func decodeListOrSet(elementType datatype.DataType, qctx *types.QueryContext, reader *bytes.Reader, length int32) (interface{}, error) {
	decodedElements := make([]interface{}, length)
	for i := int32(0); i < length; i++ {
		var elementLength int32
		err := binary.Read(reader, binary.BigEndian, &elementLength)
		if err != nil {
			return nil, err
		}
		elementValue := make([]byte, elementLength)
		_, err = reader.Read(elementValue)
		if err != nil {
			return nil, err
		}
		decodedValue, err := proxycore.DecodeType(elementType, qctx.ProtocolV, elementValue)
		if err != nil {
			return nil, err
		}
		decodedElements[i] = decodedValue
	}
	return ConvertToTypedSlice(decodedElements, elementType)
}

// decodeMap decodes a map of key-value pairs from the provided byte reader.
// It reads each key's and value's length and value, then decodes them based on their respective data types.
// Parameters:
// - reader: A byte reader positioned at the start of the encoded key-value pairs.
// - version: The Cassandra protocol version used for encoding.
// - keyType: The data type of the map keys.
// - valueType: The data type of the map values.
// - length: The number of key-value pairs in the map.
// Returns:
// - An interface{} containing the decoded map as a map[interface{}]interface{} with keys and values of the appropriate types.
// - An error if any decoding step fails.
func decodeMap(valueType datatype.DataType, qctx *types.QueryContext, reader *bytes.Reader, keyType datatype.DataType, length int32) (interface{}, error) {
	decodedMap := make(map[interface{}]interface{}, length)
	for i := int32(0); i < length; i++ {
		var keyLength int32
		err := binary.Read(reader, binary.BigEndian, &keyLength)
		if err != nil {
			return nil, err
		}
		keyValue := make([]byte, keyLength)
		_, err = reader.Read(keyValue)
		if err != nil {
			return nil, err
		}
		decodedKey, err := proxycore.DecodeType(keyType, qctx.ProtocolV, keyValue)
		if err != nil {
			return nil, err
		}

		var valueLength int32
		err = binary.Read(reader, binary.BigEndian, &valueLength)
		if err != nil {
			return nil, err
		}
		value := make([]byte, valueLength)
		_, err = reader.Read(value)
		if err != nil {
			return nil, err
		}
		decodedValue, err := proxycore.DecodeType(valueType, qctx.ProtocolV, value)
		if err != nil {
			return nil, err
		}
		decodedMap[decodedKey] = decodedValue
	}
	return ConvertToTypedMap(decodedMap, keyType, valueType)
}

// ConvertToTypedSlice converts a slice of interface{} elements to a specific type based on the provided data type.
// It performs type assertions and creates a new slice of the appropriate type for the elements.
// Parameters:
// - decodedElements: A slice of interface{} containing the decoded elements.
// - dt: The data type of the elements in the slice.
// Returns:
// - An interface{} containing the converted slice of the appropriate type.
// - An error if the type conversion fails or if the data type is unsupported.
func ConvertToTypedSlice(decodedElements []interface{}, dt datatype.DataType) (interface{}, error) {
	switch dt.GetDataTypeCode() {
	case primitive.DataTypeCodeAscii, primitive.DataTypeCodeVarchar:
		typedCollection := make([]string, len(decodedElements))
		for i, elem := range decodedElements {
			typedCollection[i] = elem.(string)
		}
		return typedCollection, nil
	case primitive.DataTypeCodeBigint, primitive.DataTypeCodeCounter:
		typedCollection := make([]int64, len(decodedElements))
		for i, elem := range decodedElements {
			typedCollection[i] = elem.(int64)
		}
		return typedCollection, nil
	case primitive.DataTypeCodeInt:
		typedCollection := make([]int32, len(decodedElements))
		for i, elem := range decodedElements {
			typedCollection[i] = elem.(int32)
		}
		return typedCollection, nil
	case primitive.DataTypeCodeFloat:
		typedCollection := make([]float32, len(decodedElements))
		for i, elem := range decodedElements {
			typedCollection[i] = elem.(float32)
		}
		return typedCollection, nil
	case primitive.DataTypeCodeDouble:
		typedCollection := make([]float64, len(decodedElements))
		for i, elem := range decodedElements {
			typedCollection[i] = elem.(float64)
		}
		return typedCollection, nil
	case primitive.DataTypeCodeBoolean:
		typedCollection := make([]bool, len(decodedElements))
		for i, elem := range decodedElements {
			typedCollection[i] = elem.(bool)
		}
		return typedCollection, nil
	case primitive.DataTypeCodeTimestamp, primitive.DataTypeCodeDate, primitive.DataTypeCodeTime:
		typedCollection := make([]int64, len(decodedElements))
		for i, elem := range decodedElements {
			typedCollection[i] = elem.(int64)
		}
		return typedCollection, nil
	case primitive.DataTypeCodeUuid, primitive.DataTypeCodeTimeuuid:
		typedCollection := make([]string, len(decodedElements))
		for i, elem := range decodedElements {
			typedCollection[i] = elem.(string)
		}
		return typedCollection, nil
	case primitive.DataTypeCodeDecimal, primitive.DataTypeCodeVarint:
		// Assuming decimal and varint are represented as strings
		typedCollection := make([]string, len(decodedElements))
		for i, elem := range decodedElements {
			typedCollection[i] = elem.(string)
		}
		return typedCollection, nil
	case primitive.DataTypeCodeBlob, primitive.DataTypeCodeInet:
		// Assuming blob and inet are represented as byte slices
		typedCollection := make([][]byte, len(decodedElements))
		for i, elem := range decodedElements {
			typedCollection[i] = elem.([]byte)
		}
		return typedCollection, nil
	case primitive.DataTypeCodeSmallint:
		typedCollection := make([]int16, len(decodedElements))
		for i, elem := range decodedElements {
			typedCollection[i] = elem.(int16)
		}
		return typedCollection, nil
	case primitive.DataTypeCodeTinyint:
		typedCollection := make([]int8, len(decodedElements))
		for i, elem := range decodedElements {
			typedCollection[i] = elem.(int8)
		}
		return typedCollection, nil
	case primitive.DataTypeCodeDuration:
		// Assuming duration is represented as int64
		typedCollection := make([]int64, len(decodedElements))
		for i, elem := range decodedElements {
			typedCollection[i] = elem.(int64)
		}
		return typedCollection, nil
	default:
		// Unsupported type
		return nil, fmt.Errorf("unsupported data type: %v", dt.GetDataTypeCode())
	}
}

// ConvertToTypedMap converts a map of interface{} keys and values to a specific type based on the provided key and value data types.
// It performs type assertions and creates a new map of the appropriate types for the keys and values.
// Parameters:
// - decodedMap: A map[interface{}]interface{} containing the decoded key-value pairs.
// - keyType: The data type of the map keys.
// - valueType: The data type of the map values.
// Returns:
// - An interface{} containing the converted map of the appropriate types for keys and values.
// - An error if the type conversion fails or if the key or value data type is unsupported.
func ConvertToTypedMap(decodedMap map[interface{}]interface{}, keyType, valueType datatype.DataType) (interface{}, error) {
	switch keyType.GetDataTypeCode() {
	case primitive.DataTypeCodeAscii, primitive.DataTypeCodeVarchar:
		switch valueType.GetDataTypeCode() {
		case primitive.DataTypeCodeBigint, primitive.DataTypeCodeCounter:
			typedMap := make(map[string]int64, len(decodedMap))
			for key, value := range decodedMap {
				typedMap[key.(string)] = value.(int64)
			}
			return typedMap, nil
		case primitive.DataTypeCodeInt:
			typedMap := make(map[string]int32, len(decodedMap))
			for key, value := range decodedMap {
				typedMap[key.(string)] = value.(int32)
			}
			return typedMap, nil
		case primitive.DataTypeCodeFloat:
			typedMap := make(map[string]float32, len(decodedMap))
			for key, value := range decodedMap {
				typedMap[key.(string)] = value.(float32)
			}
			return typedMap, nil
		case primitive.DataTypeCodeDouble:
			typedMap := make(map[string]float64, len(decodedMap))
			for key, value := range decodedMap {
				typedMap[key.(string)] = value.(float64)
			}
			return typedMap, nil
		case primitive.DataTypeCodeBoolean:
			typedMap := make(map[string]bool, len(decodedMap))
			for key, value := range decodedMap {
				typedMap[key.(string)] = value.(bool)
			}
			return typedMap, nil
		case primitive.DataTypeCodeTimestamp, primitive.DataTypeCodeDate, primitive.DataTypeCodeTime:
			typedMap := make(map[string]int64, len(decodedMap))
			for key, value := range decodedMap {
				typedMap[key.(string)] = value.(int64)
			}
			return typedMap, nil
		case primitive.DataTypeCodeUuid, primitive.DataTypeCodeTimeuuid:
			typedMap := make(map[string]string, len(decodedMap))
			for key, value := range decodedMap {
				typedMap[key.(string)] = value.(string)
			}
			return typedMap, nil
		case primitive.DataTypeCodeDecimal, primitive.DataTypeCodeVarint:
			typedMap := make(map[string]string, len(decodedMap))
			for key, value := range decodedMap {
				typedMap[key.(string)] = value.(string)
			}
			return typedMap, nil
		case primitive.DataTypeCodeBlob, primitive.DataTypeCodeInet:
			typedMap := make(map[string][]byte, len(decodedMap))
			for key, value := range decodedMap {
				typedMap[key.(string)] = value.([]byte)
			}
			return typedMap, nil
		case primitive.DataTypeCodeSmallint:
			typedMap := make(map[string]int16, len(decodedMap))
			for key, value := range decodedMap {
				typedMap[key.(string)] = value.(int16)
			}
			return typedMap, nil
		case primitive.DataTypeCodeTinyint:
			typedMap := make(map[string]int8, len(decodedMap))
			for key, value := range decodedMap {
				typedMap[key.(string)] = value.(int8)
			}
			return typedMap, nil
		case primitive.DataTypeCodeDuration:
			typedMap := make(map[string]int64, len(decodedMap))
			for key, value := range decodedMap {
				typedMap[key.(string)] = value.(int64)
			}
			return typedMap, nil
		default:
			return nil, fmt.Errorf("unsupported map value data type: %v", valueType.GetDataTypeCode())
		}
	case primitive.DataTypeCodeTimestamp:
		switch valueType.GetDataTypeCode() {
		case primitive.DataTypeCodeAscii, primitive.DataTypeCodeVarchar:
			typedMap := make(map[time.Time]string, len(decodedMap))
			for key, value := range decodedMap {
				timestampKey, ok := key.(time.Time)
				if !ok {
					return nil, fmt.Errorf("invalid key type, expect time.Time but got %T", key)
				}
				stringValue, ok := value.(string)
				if !ok {
					return nil, fmt.Errorf("invalid value type, expect string but got %T", value)
				}
				typedMap[timestampKey] = stringValue
			}
			return typedMap, nil
		case primitive.DataTypeCodeInt:
			typedMap := make(map[time.Time]int32, len(decodedMap))
			for key, value := range decodedMap {
				timestampKey, ok := key.(time.Time)
				if !ok {
					return nil, fmt.Errorf("invalid key type, expect time.Time but got %T", key)
				}
				int32value, ok := value.(int32)
				if !ok {
					return nil, fmt.Errorf("invalid value type, expect int but got %T", value)
				}
				typedMap[timestampKey] = int32value
			}
			return typedMap, nil
		case primitive.DataTypeCodeBigint:
			typedMap := make(map[time.Time]int64, len(decodedMap))
			for key, value := range decodedMap {
				timestampKey, ok := key.(time.Time)
				if !ok {
					return nil, fmt.Errorf("invalid key type, expect time.Time but got %T", key)
				}
				int64value, ok := value.(int64)
				if !ok {
					return nil, fmt.Errorf("invalid value type, expect bigint but got %T", value)
				}
				typedMap[timestampKey] = int64value
			}
			return typedMap, nil
		case primitive.DataTypeCodeFloat:
			typedMap := make(map[time.Time]float32, len(decodedMap))
			for key, value := range decodedMap {
				timestampKey, ok := key.(time.Time)
				if !ok {
					return nil, fmt.Errorf("invalid key type, expect time.Time but got %T", key)
				}
				float32value, ok := value.(float32)
				if !ok {
					return nil, fmt.Errorf("invalid value type, expect float but got %T", value)
				}
				typedMap[timestampKey] = float32value
			}
			return typedMap, nil
		case primitive.DataTypeCodeDouble:
			typedMap := make(map[time.Time]float64, len(decodedMap))
			for key, value := range decodedMap {
				timestampKey, ok := key.(time.Time)
				if !ok {
					return nil, fmt.Errorf("invalid key type, expect time.Time but got %T", key)
				}
				float64value, ok := value.(float64)
				if !ok {
					return nil, fmt.Errorf("invalid value type, double string but got %T", value)
				}
				typedMap[timestampKey] = float64value
			}
			return typedMap, nil
		case primitive.DataTypeCodeBoolean:
			typedMap := make(map[time.Time]bool, len(decodedMap))
			for key, value := range decodedMap {
				timestampKey, ok := key.(time.Time)
				if !ok {
					return nil, fmt.Errorf("invalid key type, expect time.Time but got %T", key)
				}
				boolValue, ok := value.(bool)
				if !ok {
					return nil, fmt.Errorf("invalid value type, expect boolean but got %T", value)
				}
				typedMap[timestampKey] = boolValue
			}
			return typedMap, nil
		case primitive.DataTypeCodeTimestamp:
			typedMap := make(map[time.Time]time.Time, len(decodedMap))
			for key, value := range decodedMap {
				timestampKey, ok := key.(time.Time)
				if !ok {
					return nil, fmt.Errorf("invalid key type, expect time.Time but got %T", key)
				}
				timeValue, ok := value.(time.Time)
				if !ok {
					return nil, fmt.Errorf("invalid value type, expect timestamp but got %T", value)
				}
				typedMap[timestampKey] = timeValue
			}
			return typedMap, nil
		default:
			return nil, fmt.Errorf("unsupported map value data type: %v", valueType.GetDataTypeCode())
		}

	default:
		return nil, fmt.Errorf("unsupported map key data type: %v", keyType.GetDataTypeCode())
	}
}
