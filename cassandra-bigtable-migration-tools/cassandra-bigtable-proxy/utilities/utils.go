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
	cql "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/third_party/cqlparser"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/third_party/datastax/proxycore"
	"github.com/antlr4-go/antlr/v4"
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
	MapOfStrToStr     = types.NewMapType(types.TypeVarchar, types.TypeVarchar)
	MapOfStrToInt     = types.NewMapType(types.TypeVarchar, types.TypeInt)
	MapOfStrToBigInt  = types.NewMapType(types.TypeVarchar, types.TypeBigint)
	MapOfStrToBool    = types.NewMapType(types.TypeVarchar, types.TypeBoolean)
	MapOfStrToFloat   = types.NewMapType(types.TypeVarchar, types.TypeFloat)
	MapOfStrToDouble  = types.NewMapType(types.TypeVarchar, types.TypeDouble)
	MapOfStrToTime    = types.NewMapType(types.TypeVarchar, types.TypeTimestamp)
	MapOfTimeToTime   = types.NewMapType(types.TypeTimestamp, types.TypeTimestamp)
	MapOfTimeToStr    = types.NewMapType(types.TypeTimestamp, types.TypeVarchar)
	MapOfTimeToInt    = types.NewMapType(types.TypeTimestamp, types.TypeInt)
	MapOfTimeToBigInt = types.NewMapType(types.TypeTimestamp, types.TypeBigint)
	MapOfTimeToFloat  = types.NewMapType(types.TypeTimestamp, types.TypeFloat)
	MapOfTimeToDouble = types.NewMapType(types.TypeTimestamp, types.TypeDouble)
	MapOfTimeToBool   = types.NewMapType(types.TypeTimestamp, types.TypeBoolean)
	SetOfStr          = types.NewSetType(types.TypeVarchar)
	SetOfInt          = types.NewSetType(types.TypeInt)
	SetOfBigInt       = types.NewSetType(types.TypeBigint)
	SetOfBool         = types.NewSetType(types.TypeBoolean)
	SetOfFloat        = types.NewSetType(types.TypeFloat)
	SetOfDouble       = types.NewSetType(types.TypeDouble)
	SetOfTimeStamp    = types.NewSetType(types.TypeTimestamp)
	ListOfStr         = types.NewListType(types.TypeVarchar)
	ListOfBool        = types.NewListType(types.TypeBoolean)
	ListOfInt         = types.NewListType(types.TypeInt)
	ListOfBigInt      = types.NewListType(types.TypeBigint)
	ListOfFloat       = types.NewListType(types.TypeFloat)
	ListOfDouble      = types.NewListType(types.TypeDouble)
	ListOfTimeStamp   = types.NewListType(types.TypeTimestamp)
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

func FromDataCode(dt datatype.DataType) (types.CqlDataType, error) {
	switch dt.GetDataTypeCode() {
	case primitive.DataTypeCodeAscii:
		return types.TypeAscii, nil
	case primitive.DataTypeCodeBigint:
		return types.TypeBigint, nil
	case primitive.DataTypeCodeBlob:
		return types.TypeBlob, nil
	case primitive.DataTypeCodeBoolean:
		return types.TypeBoolean, nil
	case primitive.DataTypeCodeCounter:
		return types.TypeCounter, nil
	case primitive.DataTypeCodeDecimal:
		return types.TypeDecimal, nil
	case primitive.DataTypeCodeDouble:
		return types.TypeDouble, nil
	case primitive.DataTypeCodeFloat:
		return types.TypeFloat, nil
	case primitive.DataTypeCodeInt:
		return types.TypeInt, nil
	case primitive.DataTypeCodeText:
		return types.TypeText, nil
	case primitive.DataTypeCodeTimestamp:
		return types.TypeTimestamp, nil
	case primitive.DataTypeCodeUuid:
		return types.TypeUuid, nil
	case primitive.DataTypeCodeVarchar:
		return types.TypeVarchar, nil
	case primitive.DataTypeCodeVarint:
		return types.TypeVarint, nil
	case primitive.DataTypeCodeTimeuuid:
		return types.TypeTimeuuid, nil
	case primitive.DataTypeCodeInet:
		return types.TypeInet, nil
	case primitive.DataTypeCodeDate:
		return types.TypeDate, nil
	case primitive.DataTypeCodeTime:
		return types.TypeTime, nil
	case primitive.DataTypeCodeSmallint:
		return types.TypeSmallint, nil
	case primitive.DataTypeCodeTinyint:
		return types.TypeTinyint, nil
	case primitive.DataTypeCodeList:
		lt, ok := dt.(datatype.ListType)
		if !ok {
			return nil, fmt.Errorf("unhandled type %s", dt.String())
		}
		et, err := FromDataCode(lt.GetElementType())
		if err != nil {
			return nil, err
		}
		return types.NewListType(et), nil
	case primitive.DataTypeCodeMap:
		mt, ok := dt.(datatype.MapType)
		if !ok {
			return nil, fmt.Errorf("unhandled type %s", dt.String())
		}
		kt, err := FromDataCode(mt.GetKeyType())
		if err != nil {
			return nil, err
		}
		vt, err := FromDataCode(mt.GetValueType())
		if err != nil {
			return nil, err
		}
		return types.NewMapType(kt, vt), nil
	case primitive.DataTypeCodeSet:
		st, ok := dt.(datatype.SetType)
		if !ok {
			return nil, fmt.Errorf("unhandled type %s", dt.String())
		}
		et, err := FromDataCode(st.GetElementType())
		if err != nil {
			return nil, err
		}
		return types.NewSetType(et), nil
	default:
		return nil, fmt.Errorf("unhandled type: %s", dt.String())
	}
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
			decodedList, err := collectiondecoder.DecodeCollection(ListOfStr.DataType(), primitive.ProtocolVersion4, b)
			if err != nil {
				return nil, err
			}
			return decodedList, err
		case primitive.DataTypeCodeInt:
			decodedList, err := collectiondecoder.DecodeCollection(ListOfInt.DataType(), primitive.ProtocolVersion4, b)
			if err != nil {
				return nil, err
			}
			return decodedList, err
		case primitive.DataTypeCodeBigint:
			decodedList, err := collectiondecoder.DecodeCollection(ListOfBigInt.DataType(), primitive.ProtocolVersion4, b)
			if err != nil {
				return nil, err
			}
			return decodedList, err
		case primitive.DataTypeCodeDouble:
			decodedList, err := collectiondecoder.DecodeCollection(ListOfDouble.DataType(), primitive.ProtocolVersion4, b)
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
		bytes, err = proxycore.EncodeType(MapOfStrToStr.DataType(), protocalV, v)
	case float64:
		bytes, err = proxycore.EncodeType(datatype.Double, protocalV, v)
	case float32:
		bytes, err = proxycore.EncodeType(datatype.Float, protocalV, v)
	case []string:
		bytes, err = proxycore.EncodeType(SetOfStr.DataType(), protocalV, v)
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

func IsSupportedPrimaryKeyType(dt types.CqlDataType) bool {
	if dt.IsAnyFrozen() {
		return false
	}

	switch dt.DataType() {
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

func IsSupportedColumnType(dt types.CqlDataType) bool {
	if dt.IsAnyFrozen() {
		return false
	}

	switch dt.DataType().GetDataTypeCode() {
	case primitive.DataTypeCodeInt, primitive.DataTypeCodeBigint, primitive.DataTypeCodeBlob, primitive.DataTypeCodeBoolean, primitive.DataTypeCodeDouble, primitive.DataTypeCodeFloat, primitive.DataTypeCodeTimestamp, primitive.DataTypeCodeText, primitive.DataTypeCodeVarchar, primitive.DataTypeCodeCounter:
		return true
	case primitive.DataTypeCodeMap:
		mapType := dt.DataType().(datatype.MapType)
		return isSupportedCollectionElementType(mapType.GetKeyType()) && isSupportedCollectionElementType(mapType.GetValueType())
	case primitive.DataTypeCodeSet:
		setType := dt.DataType().(datatype.SetType)
		return isSupportedCollectionElementType(setType.GetElementType())
	case primitive.DataTypeCodeList:
		listType := dt.DataType().(datatype.ListType)
		return isSupportedCollectionElementType(listType.GetElementType())
	default:
		return false
	}
}

var cqlGenericTypeRegex = regexp.MustCompile(`^(\w+)<(.+)>$`)

func ParseCqlTypeOrDie(typeStr string) types.CqlDataType {
	t, err := ParseCqlTypeString(typeStr)
	if err != nil {
		panic(err)
	}
	return t
}

// ParseCqlTypeString converts a string representation of a Cassandra data type into
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
func ParseCqlTypeString(input string) (types.CqlDataType, error) {
	input = strings.ToLower(strings.ReplaceAll(input, " ", ""))
	lexer := cql.NewCqlLexer(antlr.NewInputStream(input))
	stream := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)
	p := cql.NewCqlParser(stream)
	dataTypeTree := p.DataType()
	return ParseCqlType(dataTypeTree)
}

// ParseCqlType - parses a DataType out behaving exactly like Cassandra might. Does not validate that we support the type in this Proxy.
func ParseCqlType(dtc cql.IDataTypeContext) (types.CqlDataType, error) {
	dataTypeName := dtc.DataTypeName().GetText()
	switch strings.ToLower(dataTypeName) {
	case "frozen":
		err := validateDataTypeDefinition(dtc, 1)
		if err != nil {
			return nil, err
		}
		innerType, err := ParseCqlType(dtc.DataTypeDefinition().DataType(0))
		if err != nil {
			return nil, fmt.Errorf("failed to extract type for '%s': %w", dtc.GetText(), err)
		}
		if !innerType.IsCollection() {
			return nil, fmt.Errorf("frozen types must be a collection: '%s'", dtc.GetText())
		}
		return types.NewFrozenType(innerType), nil
	case "list":
		err := validateDataTypeDefinition(dtc, 1)
		if err != nil {
			return nil, err
		}
		innerType, err := ParseCqlType(dtc.DataTypeDefinition().DataType(0))
		if err != nil {
			return nil, fmt.Errorf("failed to extract type for '%s': %w", dtc.GetText(), err)
		}
		if innerType.IsCollection() && innerType.Code() != types.FROZEN {
			return nil, fmt.Errorf("lists cannot contain collections unless they are frozen")
		}
		return types.NewListType(innerType), nil
	case "set":
		err := validateDataTypeDefinition(dtc, 1)
		if err != nil {
			return nil, err
		}
		innerType, err := ParseCqlType(dtc.DataTypeDefinition().DataType(0))
		if err != nil {
			return nil, fmt.Errorf("failed to extract type for '%s': %w", dtc.GetText(), err)
		}
		if innerType.IsCollection() && innerType.Code() != types.FROZEN {
			return nil, fmt.Errorf("sets cannot contain collections unless they are frozen")
		}
		return types.NewSetType(innerType), nil
	case "map":
		err := validateDataTypeDefinition(dtc, 2)
		if err != nil {
			return nil, err
		}
		keyType, err := ParseCqlType(dtc.DataTypeDefinition().DataType(0))
		if err != nil {
			return nil, fmt.Errorf("failed to extract key type for '%s': %w", dtc.GetText(), err)
		}
		if keyType.IsCollection() {
			return nil, fmt.Errorf("map key types must be scalar")
		}
		valueType, err := ParseCqlType(dtc.DataTypeDefinition().DataType(1))
		if err != nil {
			return nil, fmt.Errorf("failed to extract value type for '%s': %w", dtc.GetText(), err)
		}
		if valueType.IsCollection() && valueType.Code() != types.FROZEN {
			return nil, fmt.Errorf("map values cannot be collections unless they are frozen")
		}
		if err != nil {
			return nil, fmt.Errorf("failed to extract type for '%s': %w", dtc.GetText(), err)
		}
		return types.NewMapType(keyType, valueType), nil
	case "text":
		err := validateNoDataTypeDefinition(dtc)
		if err != nil {
			return nil, err
		}
		return types.TypeText, nil
	case "varchar":
		err := validateNoDataTypeDefinition(dtc)
		if err != nil {
			return nil, err
		}
		return types.TypeVarchar, nil
	case "blob":
		err := validateNoDataTypeDefinition(dtc)
		if err != nil {
			return nil, err
		}
		return types.TypeBlob, nil
	case "timestamp":
		err := validateNoDataTypeDefinition(dtc)
		if err != nil {
			return nil, err
		}
		return types.TypeTimestamp, nil
	case "int":
		err := validateNoDataTypeDefinition(dtc)
		if err != nil {
			return nil, err
		}
		return types.TypeInt, nil
	case "bigint":
		err := validateNoDataTypeDefinition(dtc)
		if err != nil {
			return nil, err
		}
		return types.TypeBigint, nil
	case "boolean":
		err := validateNoDataTypeDefinition(dtc)
		if err != nil {
			return nil, err
		}
		return types.TypeBoolean, nil
	case "uuid":
		err := validateNoDataTypeDefinition(dtc)
		if err != nil {
			return nil, err
		}
		return types.TypeUuid, nil
	case "float":
		err := validateNoDataTypeDefinition(dtc)
		if err != nil {
			return nil, err
		}
		return types.TypeFloat, nil
	case "double":
		err := validateNoDataTypeDefinition(dtc)
		if err != nil {
			return nil, err
		}
		return types.TypeDouble, nil
	case "decimal":
		err := validateNoDataTypeDefinition(dtc)
		if err != nil {
			return nil, err
		}
		return types.TypeDecimal, nil
	case "counter":
		err := validateNoDataTypeDefinition(dtc)
		if err != nil {
			return nil, err
		}
		return types.TypeCounter, nil
	case "timeuuid":
		err := validateNoDataTypeDefinition(dtc)
		if err != nil {
			return nil, err
		}
		return types.TypeTimeuuid, nil
	case "ascii":
		err := validateNoDataTypeDefinition(dtc)
		if err != nil {
			return nil, err
		}
		return types.TypeAscii, nil
	case "varint":
		err := validateNoDataTypeDefinition(dtc)
		if err != nil {
			return nil, err
		}
		return types.TypeVarint, nil
	case "date":
		err := validateNoDataTypeDefinition(dtc)
		if err != nil {
			return nil, err
		}
		return types.TypeDate, nil
	case "smallint":
		err := validateNoDataTypeDefinition(dtc)
		if err != nil {
			return nil, err
		}
		return types.TypeSmallint, nil
	case "tinyint":
		err := validateNoDataTypeDefinition(dtc)
		if err != nil {
			return nil, err
		}
		return types.TypeTinyint, nil
	default:
		return nil, fmt.Errorf("unknown data type name: '%s' in type '%s'", dataTypeName, dtc)
	}
}

func validateNoDataTypeDefinition(dt cql.IDataTypeContext) error {
	if dt.DataTypeDefinition() != nil {
		return fmt.Errorf("unexpected data type definition: '%s'", dt.GetText())
	}
	return nil
}
func validateDataTypeDefinition(dt cql.IDataTypeContext, expectedTypeCount int) error {
	def := dt.DataTypeDefinition()
	if def == nil {
		return fmt.Errorf("data type definition missing in: '%s'", dt.GetText())
	}
	if def.GetText() == "<>" {
		return fmt.Errorf("empty type definition in '%s'", dt.GetText())
	}
	if len(def.AllDataType()) != expectedTypeCount {
		return fmt.Errorf("expected exactly %d types but found %d in: '%s'", expectedTypeCount, len(def.AllDataType()), dt.GetText())
	}
	if def.SyntaxBracketLa() == nil {
		return fmt.Errorf("missing opening type bracket in: '%s'", dt.GetText())
	}
	if def.SyntaxBracketRa() == nil {
		return fmt.Errorf("missing closing type bracket in: '%s'", dt.GetText())
	}
	return nil
}
