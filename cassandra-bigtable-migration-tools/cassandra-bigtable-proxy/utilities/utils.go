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

package utilities

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/collectiondecoder"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/third_party/datastax/proxycore"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
)

const (
	KEY_TYPE_PARTITION  = "partition_key"
	KEY_TYPE_CLUSTERING = "clustering"
	KEY_TYPE_REGULAR    = "regular"
)

// from https://cassandra.apache.org/doc/4.0/cassandra/cql/appendices.html#appendix-A
// a value of "true" means the keyword is truly reserved, while a value of "false" means the keyword is non-reserved/available in certain situations.
var reservedKeywords = map[string]bool{
	"ADD":          true,
	"AGGREGATE":    true,
	"ALL":          false,
	"ALLOW":        true,
	"ALTER":        true,
	"AND":          true,
	"ANY":          true,
	"APPLY":        true,
	"AS":           false,
	"ASC":          true,
	"ASCII":        false,
	"AUTHORIZE":    true,
	"BATCH":        true,
	"BEGIN":        true,
	"BIGINT":       false,
	"BLOB":         false,
	"BOOLEAN":      false,
	"BY":           true,
	"CLUSTERING":   false,
	"COLUMNFAMILY": true,
	"COMPACT":      false,
	"CONSISTENCY":  false,
	"COUNT":        false,
	"COUNTER":      false,
	"CREATE":       true,
	"CUSTOM":       false,
	"DECIMAL":      false,
	"DELETE":       true,
	"DESC":         true,
	"DISTINCT":     false,
	"DOUBLE":       false,
	"DROP":         true,
	"EACH_QUORUM":  true,
	"ENTRIES":      true,
	"EXISTS":       false,
	"FILTERING":    false,
	"FLOAT":        false,
	"FROM":         true,
	"FROZEN":       false,
	"FULL":         true,
	"GRANT":        true,
	"IF":           true,
	"IN":           true,
	"INDEX":        true,
	"INET":         true,
	"INFINITY":     true,
	"INSERT":       true,
	"INT":          false,
	"INTO":         true,
	"KEY":          false,
	"KEYSPACE":     true,
	"KEYSPACES":    true,
	"LEVEL":        false,
	"LIMIT":        true,
	"LIST":         false,
	"LOCAL_ONE":    true,
	"LOCAL_QUORUM": true,
	"MAP":          false,
	"MATERIALIZED": true,
	"MODIFY":       true,
	"NAN":          true,
	"NORECURSIVE":  true,
	"NOSUPERUSER":  false,
	"NOT":          true,
	"OF":           true,
	"ON":           true,
	"ONE":          true,
	"ORDER":        true,
	"PARTITION":    true,
	"PASSWORD":     true,
	"PER":          true,
	"PERMISSION":   false,
	"PERMISSIONS":  false,
	"PRIMARY":      true,
	"QUORUM":       true,
	"RENAME":       true,
	"REVOKE":       true,
	"SCHEMA":       true,
	"SELECT":       true,
	"SET":          true,
	"STATIC":       false,
	"STORAGE":      false,
	"SUPERUSER":    false,
	"TABLE":        true,
	"TEXT":         false,
	"TIME":         true,
	"TIMESTAMP":    false,
	"TIMEUUID":     false,
	"THREE":        true,
	"TO":           true,
	"TOKEN":        true,
	"TRUNCATE":     true,
	"TTL":          false,
	"TUPLE":        false,
	"TWO":          true,
	"TYPE":         false,
	"UNLOGGED":     true,
	"UPDATE":       true,
	"USE":          true,
	"USER":         false,
	"USERS":        false,
	"USING":        true,
	"UUID":         false,
	"VALUES":       false,
	"VARCHAR":      false,
	"VARINT":       false,
	"VIEW":         true,
	"WHERE":        true,
	"WITH":         true,
	"WRITETIME":    false,
	"BITSTRING":    false,
	"BYTE":         false,
	"COMPLEX":      false,
	"ENUM":         false,
	"INTERVAL":     false,
	"MACADDR":      false,
}

var (
	MapOfStrToStr     = datatype.NewMapType(datatype.Varchar, datatype.Varchar)
	MapOfStrToInt     = datatype.NewMapType(datatype.Varchar, datatype.Int)
	MapOfStrToBigInt  = datatype.NewMapType(datatype.Varchar, datatype.Bigint)
	MapOfStrToBool    = datatype.NewMapType(datatype.Varchar, datatype.Boolean)
	MapOfStrToFloat   = datatype.NewMapType(datatype.Varchar, datatype.Float)
	MapOfStrToDouble  = datatype.NewMapType(datatype.Varchar, datatype.Double)
	MapOfStrToTime    = datatype.NewMapType(datatype.Varchar, datatype.Timestamp)
	MapOfTimeToTime   = datatype.NewMapType(datatype.Timestamp, datatype.Timestamp)
	MapOfTimeToStr    = datatype.NewMapType(datatype.Timestamp, datatype.Varchar)
	MapOfTimeToInt    = datatype.NewMapType(datatype.Timestamp, datatype.Int)
	MapOfTimeToBigInt = datatype.NewMapType(datatype.Timestamp, datatype.Bigint)
	MapOfTimeToFloat  = datatype.NewMapType(datatype.Timestamp, datatype.Float)
	MapOfTimeToDouble = datatype.NewMapType(datatype.Timestamp, datatype.Double)
	MapOfTimeToBool   = datatype.NewMapType(datatype.Timestamp, datatype.Boolean)
	SetOfStr          = datatype.NewSetType(datatype.Varchar)
	SetOfInt          = datatype.NewSetType(datatype.Int)
	SetOfBigInt       = datatype.NewSetType(datatype.Bigint)
	SetOfBool         = datatype.NewSetType(datatype.Boolean)
	SetOfFloat        = datatype.NewSetType(datatype.Float)
	SetOfDouble       = datatype.NewSetType(datatype.Double)
	SetOfTimeStamp    = datatype.NewSetType(datatype.Timestamp)
	ListOfStr         = datatype.NewListType(datatype.Varchar)
	ListOfBool        = datatype.NewListType(datatype.Boolean)
	ListOfInt         = datatype.NewListType(datatype.Int)
	ListOfBigInt      = datatype.NewListType(datatype.Bigint)
	ListOfFloat       = datatype.NewListType(datatype.Float)
	ListOfDouble      = datatype.NewListType(datatype.Double)
	ListOfTimeStamp   = datatype.NewListType(datatype.Timestamp)
)

var (
	EncodedTrue, _  = proxycore.EncodeType(datatype.Boolean, primitive.ProtocolVersion4, true)
	EncodedFalse, _ = proxycore.EncodeType(datatype.Boolean, primitive.ProtocolVersion4, false)
)

// TODO: Move these variables to global level
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
	CassandraTypeCounter   = "counter"
)
const (
	Info  = "info"
	Debug = "debug"
	Error = "error"
	Warn  = "warn"
)

func IsCollectionColumn(c *types.Column) bool {
	return IsCollection(c.TypeInfo.DataType)
}

// IsCollection() checks if the provided data type is a collection type (list, set, or map).
func IsCollection(dt datatype.DataType) bool {
	switch dt.GetDataTypeCode() {
	case primitive.DataTypeCodeList, primitive.DataTypeCodeSet, primitive.DataTypeCodeMap:
		return true
	default:
		return false
	}
}

// DecodeBytesToCassandraColumnType(): Function to decode incoming bytes parameter
// for handleExecute scenario into corresponding go datatype
//
// Parameters:
//   - b: []byte
//   - choice:  datatype.DataType
//   - protocolVersion: primitive.ProtocolVersion
//
// Returns: (interface{}, error)
func DecodeBytesToCassandraColumnType(b []byte, choice datatype.PrimitiveType, protocolVersion primitive.ProtocolVersion) (any, error) {
	switch choice.GetDataTypeCode() {
	case primitive.DataTypeCodeVarchar:
		return proxycore.DecodeType(datatype.Varchar, protocolVersion, b)
	case primitive.DataTypeCodeDouble:
		return proxycore.DecodeType(datatype.Double, protocolVersion, b)
	case primitive.DataTypeCodeFloat:
		return proxycore.DecodeType(datatype.Float, protocolVersion, b)
	case primitive.DataTypeCodeBigint, primitive.DataTypeCodeCounter:
		return proxycore.DecodeType(datatype.Bigint, protocolVersion, b)
	case primitive.DataTypeCodeTimestamp:
		return proxycore.DecodeType(datatype.Timestamp, protocolVersion, b)
	case primitive.DataTypeCodeInt:
		var decodedInt int64
		if len(b) == 8 {
			decoded, err := proxycore.DecodeType(datatype.Bigint, protocolVersion, b)
			if err != nil {
				return nil, err
			}
			decodedInt = decoded.(int64)
		} else {
			decoded, err := proxycore.DecodeType(datatype.Int, protocolVersion, b)
			if err != nil {
				return nil, err
			}
			decodedInt = int64(decoded.(int32))
		}
		return decodedInt, nil
	case primitive.DataTypeCodeBoolean:
		return proxycore.DecodeType(datatype.Boolean, protocolVersion, b)
	case primitive.DataTypeCodeDate:
		return proxycore.DecodeType(datatype.Date, protocolVersion, b)
	case primitive.DataTypeCodeBlob:
		return proxycore.DecodeType(datatype.Blob, protocolVersion, b)
	default:
		res, err := decodeNonPrimitive(choice, b)
		return res, err
	}
}

func IsReservedCqlKeyword(s string) bool {
	// we're opting to treat reserved and "non-reserved" keywords the same, for simplicity
	_, found := reservedKeywords[strings.ToUpper(s)]
	return found
}

// decodeNonPrimitive() Decodes non-primitive types like list, list, and list from byte data based on the provided datatype choice. Returns the decoded collection or an error if unsupported.
func decodeNonPrimitive(choice datatype.PrimitiveType, b []byte) (any, error) {
	var err error
	// Check if it's a list type
	if choice.GetDataTypeCode() == primitive.DataTypeCodeList {
		// Get the element type
		listType := choice.(datatype.ListType)
		elementType := listType.GetElementType()

		// Now check the element type's code
		switch elementType.GetDataTypeCode() {
		case primitive.DataTypeCodeVarchar:
			decodedList, err := collectiondecoder.DecodeCollection(ListOfStr, primitive.ProtocolVersion4, b)
			if err != nil {
				return nil, err
			}
			return decodedList, err
		case primitive.DataTypeCodeInt:
			decodedList, err := collectiondecoder.DecodeCollection(ListOfInt, primitive.ProtocolVersion4, b)
			if err != nil {
				return nil, err
			}
			return decodedList, err
		case primitive.DataTypeCodeBigint:
			decodedList, err := collectiondecoder.DecodeCollection(ListOfBigInt, primitive.ProtocolVersion4, b)
			if err != nil {
				return nil, err
			}
			return decodedList, err
		case primitive.DataTypeCodeDouble:
			decodedList, err := collectiondecoder.DecodeCollection(ListOfDouble, primitive.ProtocolVersion4, b)
			if err != nil {
				return nil, err
			}
			return decodedList, err
		default:
			err = fmt.Errorf("unsupported list element type to decode - %v", elementType.GetDataTypeCode())
			return nil, err
		}
	}

	err = fmt.Errorf("unsupported Datatype to decode - %v", choice.GetDataTypeCode())
	return nil, err
}

// defaultIfEmpty() returns a default string value if the provided value is empty.
// Useful for setting default configuration values.
func defaultIfEmpty(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

// defaultIfZero() returns a default integer value if the provided value is zero.
// Useful for setting default configuration values.
func defaultIfZero(value, defaultValue int) int {
	if value == 0 {
		return defaultValue
	}
	return value
}

// TypeConversion() converts a Go data type to a Cassandra protocol-compliant byte array.
//
// Parameters:
//   - s: The data to be converted.
//   - protocalV: Cassandra protocol version.
//
// Returns: Byte array in Cassandra protocol format or an error if conversion fails.
func TypeConversion(s any, protocalV primitive.ProtocolVersion) ([]byte, error) {
	var bytes []byte
	var err error
	switch v := s.(type) {
	case string:
		bytes, err = proxycore.EncodeType(datatype.Varchar, protocalV, v)
	case time.Time:
		bytes, err = proxycore.EncodeType(datatype.Timestamp, protocalV, v)
	case []byte:
		bytes, err = proxycore.EncodeType(datatype.Blob, protocalV, v)
	case int64:
		bytes, err = proxycore.EncodeType(datatype.Bigint, protocalV, v)
	case int:
		bytes, err = proxycore.EncodeType(datatype.Int, protocalV, v)
	case bool:
		bytes, err = proxycore.EncodeType(datatype.Boolean, protocalV, v)
	case map[string]string:
		bytes, err = proxycore.EncodeType(MapOfStrToStr, protocalV, v)
	case float64:
		bytes, err = proxycore.EncodeType(datatype.Double, protocalV, v)
	case float32:
		bytes, err = proxycore.EncodeType(datatype.Float, protocalV, v)
	case []string:
		bytes, err = proxycore.EncodeType(SetOfStr, protocalV, v)
	case datatype.DataType:
		cqlTypeInString := fmt.Sprintf("%v", v)
		bytes, err = proxycore.EncodeType(datatype.Varchar, protocalV, cqlTypeInString)
	default:
		err = fmt.Errorf("%v - %v", "Unknown Datatype Identified", s)
	}

	return bytes, err
}

/*
DataConversionInInsertionIfRequired() converts a value to a byte array based on the provided Cassandra type and response type.
Parameters:
  - value: any
  - pv: primitive.ProtocolVersion
  - cqlType: string
*/
func DataConversionInInsertionIfRequired(value any, pv primitive.ProtocolVersion, cqlType string, responseType string) (any, error) {
	switch cqlType {
	case CassandraTypeBoolean:
		switch responseType {
		case CassandraTypeString:
			val, err := strconv.ParseBool(value.(string))
			if err != nil {
				return nil, err
			}
			if val {
				return "1", nil
			} else {
				return "0", nil
			}
		default:
			return EncodeBool(value, pv)
		}
	case CassandraTypeInt:
		switch responseType {
		case CassandraTypeString:
			val, err := strconv.ParseInt(value.(string), 10, 64)
			if err != nil {
				return nil, err
			}
			stringVal := strconv.FormatInt(val, 10)
			return stringVal, nil
		default:
			return EncodeInt(value, pv)
		}
	default:
		return value, nil
	}
}

/*
EncodeBool() encodes a boolean value to a byte array
Parameters:
  - value: any
  - pv: primitive.ProtocolVersion

Returns: []byte, error
*/
func EncodeBool(value any, pv primitive.ProtocolVersion) ([]byte, error) {
	switch v := value.(type) {
	case string:
		val, err := strconv.ParseBool(v)
		if err != nil {
			return nil, err
		}
		strVal := "0"
		if val {
			strVal = "1"
		}
		intVal, _ := strconv.ParseInt(strVal, 10, 64)
		bd, err := proxycore.EncodeType(datatype.Bigint, pv, intVal)
		return bd, err
	case bool:
		var valInBigint int64
		if v {
			valInBigint = 1
		} else {
			valInBigint = 0
		}
		bd, err := proxycore.EncodeType(datatype.Bigint, pv, valInBigint)
		return bd, err
	case []byte:
		vaInInterface, err := proxycore.DecodeType(datatype.Boolean, pv, v)
		if err != nil {
			return nil, err
		}
		if vaInInterface.(bool) {
			return proxycore.EncodeType(datatype.Bigint, pv, 1)
		} else {
			return proxycore.EncodeType(datatype.Bigint, pv, 0)
		}
	default:
		return nil, fmt.Errorf("unsupported type: %v", value)
	}
}

/*
EncodeInt() encodes an integer value to a byte array
Parameters:
  - value: any
  - pv: primitive.ProtocolVersion

Returns: []byte, error
*/
func EncodeInt(value any, pv primitive.ProtocolVersion) ([]byte, error) {
	switch v := value.(type) {
	case string:
		val, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, err
		}
		bd, err := proxycore.EncodeType(datatype.Bigint, pv, val)
		return bd, err
	case int32:
		bd, err := proxycore.EncodeType(datatype.Bigint, pv, int32(v))
		if err != nil {
			return nil, err
		}
		return bd, err
	case []byte:
		intVal, err := proxycore.DecodeType(datatype.Int, pv, v)
		if err != nil {
			return nil, err
		}
		return proxycore.EncodeType(datatype.Bigint, pv, intVal.(int32))
	default:
		return nil, fmt.Errorf("unsupported type: %v", value)
	}
}

/*
GetClauseByValue() returns the clause that matches the value
Parameters:
  - clause: []types.Clause
  - value: string

Returns: types.Clause, error
*/
func GetClauseByValue(clause []types.Clause, value string) (types.Clause, error) {
	for _, c := range clause {
		if c.Value == "@"+value {
			return c, nil
		}
	}
	return types.Clause{}, fmt.Errorf("clause not found")
}

/*
GetClauseByColumn() returns the clause that matches the column
Parameters:
  - clause: []types.Clause
  - column: string

Returns: types.Clause, error
*/
func GetClauseByColumn(clause []types.Clause, column string) (types.Clause, error) {
	for _, c := range clause {
		if c.Column == column {
			return c, nil
		}
	}
	return types.Clause{}, fmt.Errorf("clause not found")
}

func IsSupportedPrimaryKeyType(dt *types.CqlTypeInfo) bool {
	if dt.IsFrozen {
		return false
	}

	switch dt.DataType {
	case datatype.Int, datatype.Bigint, datatype.Varchar:
		return true
	default:
		return false
	}
}

func isSupportedCollectionElementType(dt datatype.DataType) bool {
	switch dt {
	case datatype.Int, datatype.Bigint, datatype.Varchar, datatype.Float, datatype.Double, datatype.Timestamp, datatype.Boolean:
		return true
	default:
		return false
	}
}

func IsSupportedColumnType(dt *types.CqlTypeInfo) bool {
	switch dt.DataType.GetDataTypeCode() {
	case primitive.DataTypeCodeInt, primitive.DataTypeCodeBigint, primitive.DataTypeCodeBlob, primitive.DataTypeCodeBoolean, primitive.DataTypeCodeDouble, primitive.DataTypeCodeFloat, primitive.DataTypeCodeTimestamp, primitive.DataTypeCodeText, primitive.DataTypeCodeVarchar, primitive.DataTypeCodeCounter:
		return true
	case primitive.DataTypeCodeMap:
		mapType := dt.DataType.(datatype.MapType)
		return isSupportedCollectionElementType(mapType.GetKeyType()) && isSupportedCollectionElementType(mapType.GetValueType())
	case primitive.DataTypeCodeSet:
		setType := dt.DataType.(datatype.SetType)
		return isSupportedCollectionElementType(setType.GetElementType())
	case primitive.DataTypeCodeList:
		listType := dt.DataType.(datatype.ListType)
		return isSupportedCollectionElementType(listType.GetElementType())
	default:
		return false
	}
}

var cqlGenericTypeRegex = regexp.MustCompile(`^(\w+)<(.+)>$`)

func ParseCqlTypeOrDie(typeStr string) *types.CqlTypeInfo {
	t, err := ParseCqlType(typeStr)
	if err != nil {
		panic(err)
	}
	return t
}

// ParseCqlType converts a string representation of a Cassandra data type into
// a corresponding DataType value. It supports a range of common Cassandra data types,
// including text, blob, timestamp, int, bigint, boolean, uuid, various map and list types.
//
// Parameters:
//   - typeStr: A string representing the Cassandra column data type. This function expects
//     the data type in a specific format (e.g., "text", "int", "map<text, boolean>").
//
// Returns:
//   - datatype.DataType: The corresponding DataType value for the provided string.
//     This is used to represent the Cassandra data type in a structured format within Go.
//   - bool: true if is frozen
//   - error: An error is returned if the provided string does not match any of the known
//     Cassandra data types. This helps in identifying unsupported or incorrectly specified
//     data types.
func ParseCqlType(typeStr string) (*types.CqlTypeInfo, error) {
	typeStr = strings.ToLower(strings.ReplaceAll(typeStr, " ", ""))

	matches := cqlGenericTypeRegex.FindStringSubmatch(typeStr)
	if matches != nil {
		switch matches[1] {
		case "frozen":
			innerType, err := ParseCqlType(matches[2])
			if err != nil {
				return nil, fmt.Errorf("failed to extract type for '%s': %w", typeStr, err)
			}
			if !innerType.IsCollection() {
				return nil, fmt.Errorf("failed to extract type for '%s': frozen types must be a collection", typeStr)
			}
			return &types.CqlTypeInfo{
				RawType:  typeStr,
				DataType: innerType.DataType,
				IsFrozen: true,
			}, nil
		case "list":
			innerType, err := ParseCqlType(matches[2])
			if err != nil {
				return nil, fmt.Errorf("failed to extract type for '%s': %w", typeStr, err)
			}
			return &types.CqlTypeInfo{
				RawType:  typeStr,
				DataType: datatype.NewListType(innerType.DataType),
				IsFrozen: false,
			}, nil
		case "set":
			innerType, err := ParseCqlType(matches[2])
			if err != nil {
				return nil, fmt.Errorf("failed to extract type for '%s': %w", typeStr, err)
			}
			return &types.CqlTypeInfo{
				RawType:  typeStr,
				DataType: datatype.NewSetType(innerType.DataType),
				IsFrozen: false,
			}, nil
		case "map":
			parts := strings.SplitN(matches[2], ",", 2)
			if len(parts) != 2 {
				return nil, fmt.Errorf("failed to extract type for '%s': malformed map type", typeStr)
			}
			keyType, err := ParseCqlType(parts[0])
			if err != nil {
				return nil, fmt.Errorf("failed to extract type for '%s': %w", typeStr, err)
			}
			valueType, err := ParseCqlType(parts[1])
			if err != nil {
				return nil, fmt.Errorf("failed to extract type for '%s': %w", typeStr, err)
			}
			return &types.CqlTypeInfo{
				RawType:  typeStr,
				DataType: datatype.NewMapType(keyType.DataType, valueType.DataType),
				IsFrozen: false,
			}, nil
		default:
			return nil, fmt.Errorf("unsupported generic column type: %s in type %s", matches[1], typeStr)
		}
	}

	switch typeStr {
	case "text", "varchar":
		return &types.CqlTypeInfo{RawType: typeStr, DataType: datatype.Varchar, IsFrozen: false}, nil
	case "blob":
		return &types.CqlTypeInfo{RawType: typeStr, DataType: datatype.Blob, IsFrozen: false}, nil
	case "timestamp":
		return &types.CqlTypeInfo{RawType: typeStr, DataType: datatype.Timestamp, IsFrozen: false}, nil
	case "int":
		return &types.CqlTypeInfo{RawType: typeStr, DataType: datatype.Int, IsFrozen: false}, nil
	case "bigint":
		return &types.CqlTypeInfo{RawType: typeStr, DataType: datatype.Bigint, IsFrozen: false}, nil
	case "boolean":
		return &types.CqlTypeInfo{RawType: typeStr, DataType: datatype.Boolean, IsFrozen: false}, nil
	case "uuid":
		return &types.CqlTypeInfo{RawType: typeStr, DataType: datatype.Uuid, IsFrozen: false}, nil
	case "float":
		return &types.CqlTypeInfo{RawType: typeStr, DataType: datatype.Float, IsFrozen: false}, nil
	case "double":
		return &types.CqlTypeInfo{RawType: typeStr, DataType: datatype.Double, IsFrozen: false}, nil
	case "counter":
		return &types.CqlTypeInfo{RawType: typeStr, DataType: datatype.Counter, IsFrozen: false}, nil
	default:
		return nil, fmt.Errorf("unsupported column type: %s", typeStr)
	}
}
