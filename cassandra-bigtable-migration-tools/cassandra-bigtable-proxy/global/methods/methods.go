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

/*
 * @file methods.go
 * @brief To avoid circular dependency, we have moved the methods to a separate file.
 */
package methods

import (
	"fmt"
	"strings"

	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
)

const (
	// Primitive types
	CassandraTypeText      = "text"
	CassandraTypeString    = "string"
	CassandraTypeBlob      = "blob"
	CassandraTypeTimestamp = "timestamp"
	CassandraTypeInt       = "int"
	CassandraTypeBigint    = "bigint"
	CassandraTypeBoolean   = "boolean"
	CassandraTypeUuid      = "uuid"
	CassandraTypeFloat     = "float"
	CassandraTypeDouble    = "double"
)

// GetCassandraColumnType() converts a string representation of a Cassandra data type into
// a corresponding DataType value. It supports a range of common Cassandra data types,
// including text, blob, timestamp, int, bigint, boolean, uuid, various map and list types.
//
// Parameters:
//   - c: A string representing the Cassandra column data type. This function expects
//     the data type in a specific format (e.g., "text", "int", "map<text, boolean>").
//
// Returns:
//   - datatype.DataType: The corresponding DataType value for the provided string.
//     This is used to represent the Cassandra data type in a structured format within Go.
//   - error: An error is returned if the provided string does not match any of the known
//     Cassandra data types. This helps in identifying unsupported or incorrectly specified
//     data types.
func GetCassandraColumnType(c string) (datatype.DataType, error) {
	choice := strings.ToLower(strings.ReplaceAll(c, " ", ""))
	if strings.HasSuffix(choice, ">") {
		if strings.HasPrefix(choice, "frozen<") {
			innerType, err := GetCassandraColumnType(choice[7 : len(choice)-1])
			if err != nil {
				return nil, fmt.Errorf("failed to extract type for '%s': %w", c, err)
			}
			// drop the frozen wrapper
			return innerType, nil
		} else if strings.HasPrefix(choice, "list<") {
			innerType, err := GetCassandraColumnType(choice[5 : len(choice)-1])
			if err != nil {
				return nil, fmt.Errorf("failed to extract type for '%s': %w", c, err)
			}
			return datatype.NewListType(innerType), nil
		} else if strings.HasPrefix(choice, "set<") {
			innerType, err := GetCassandraColumnType(choice[4 : len(choice)-1])
			if err != nil {
				return nil, fmt.Errorf("failed to extract type for '%s': %w", c, err)
			}
			return datatype.NewSetType(innerType), nil
		} else if strings.HasPrefix(choice, "map<") {
			parts := strings.SplitN(choice[4:len(choice)-1], ",", 2)
			if len(parts) != 2 {
				return nil, fmt.Errorf("malformed map type")
			}
			keyType, err := GetCassandraColumnType(parts[0])
			if err != nil {
				return nil, fmt.Errorf("failed to extract type for '%s': %w", c, err)
			}
			valueType, err := GetCassandraColumnType(parts[1])
			if err != nil {
				return nil, fmt.Errorf("failed to extract type for '%s': %w", c, err)
			}
			return datatype.NewMapType(keyType, valueType), nil
		}
	}
	switch choice {
	case CassandraTypeText, "varchar":
		return datatype.Varchar, nil
	case CassandraTypeBlob:
		return datatype.Blob, nil
	case CassandraTypeTimestamp:
		return datatype.Timestamp, nil
	case CassandraTypeInt:
		return datatype.Int, nil
	case CassandraTypeBigint:
		return datatype.Bigint, nil
	case CassandraTypeBoolean:
		return datatype.Boolean, nil
	case CassandraTypeUuid:
		return datatype.Uuid, nil
	case CassandraTypeFloat:
		return datatype.Float, nil
	case CassandraTypeDouble:
		return datatype.Double, nil
	default:
		return nil, fmt.Errorf("unsupported column type: %s", choice)
	}
}

func ConvertCQLDataTypeToString(cqlType datatype.DataType) (string, error) {
	if cqlType == nil {
		return "", fmt.Errorf("datatype is nil")
	}

	switch cqlType {
	case datatype.Varchar:
		return "varchar", nil
	case datatype.Blob:
		return "blob", nil
	case datatype.Timestamp:
		return "timestamp", nil
	case datatype.Int:
		return "int", nil
	case datatype.Bigint:
		return "bigint", nil
	case datatype.Boolean:
		return "boolean", nil
	case datatype.Uuid:
		return "uuid", nil
	case datatype.Float:
		return "float", nil
	case datatype.Double:
		return "double", nil
	default:
		typeCode := cqlType.GetDataTypeCode()

		// Check for specific type code value (0x0022 = 34 = Set) primitive.DataTypeCodeSet
		if typeCode == primitive.DataTypeCodeSet {
			setType, ok := cqlType.(datatype.SetType)
			if !ok {
				return "", fmt.Errorf("failed to assert set type for %v", cqlType)
			}
			elemTypeStr, elemErr := ConvertCQLDataTypeToString(setType.GetElementType())
			if elemErr != nil {
				return "", elemErr
			}
			return fmt.Sprintf("set<%s>", elemTypeStr), nil
		} else if typeCode == primitive.DataTypeCode(0x0020) { // List = 0x0020 = 32
			listType, ok := cqlType.(datatype.ListType)
			if !ok {
				return "", fmt.Errorf("failed to assert list type for %v", cqlType)
			}
			elemTypeStr, elemErr := ConvertCQLDataTypeToString(listType.GetElementType())
			if elemErr != nil {
				return "", elemErr
			}
			return fmt.Sprintf("list<%s>", elemTypeStr), nil
		} else if typeCode == primitive.DataTypeCode(0x0021) { // Map = 0x0021 = 33
			mapType, ok := cqlType.(datatype.MapType)
			if !ok {
				return "", fmt.Errorf("failed to assert map type for %v", cqlType)
			}
			keyTypeStr, keyErr := ConvertCQLDataTypeToString(mapType.GetKeyType())
			if keyErr != nil {
				return "", keyErr
			}
			valueTypeStr, valueErr := ConvertCQLDataTypeToString(mapType.GetValueType())
			if valueErr != nil {
				return "", valueErr
			}
			return fmt.Sprintf("map<%s,%s>", keyTypeStr, valueTypeStr), nil
		} else {
			// Fallback to type assertion (legacy approach)
			if mapType, ok := cqlType.(datatype.MapType); ok {
				keyTypeStr, keyErr := ConvertCQLDataTypeToString(mapType.GetKeyType())
				if keyErr != nil {
					return "", keyErr
				}
				valueTypeStr, valueErr := ConvertCQLDataTypeToString(mapType.GetValueType())
				if valueErr != nil {
					return "", valueErr
				}
				return fmt.Sprintf("map<%s,%s>", keyTypeStr, valueTypeStr), nil
			} else if listType, ok := cqlType.(datatype.ListType); ok {
				elemTypeStr, elemErr := ConvertCQLDataTypeToString(listType.GetElementType())
				if elemErr != nil {
					return "", elemErr
				}
				return fmt.Sprintf("list<%s>", elemTypeStr), nil
			} else if setType, ok := cqlType.(datatype.SetType); ok {
				elemTypeStr, elemErr := ConvertCQLDataTypeToString(setType.GetElementType())
				if elemErr != nil {
					return "", elemErr
				}
				return fmt.Sprintf("set<%s>", elemTypeStr), nil
			}
			return "", fmt.Errorf("unsupported data type: %v", cqlType)
		}
	}
}
