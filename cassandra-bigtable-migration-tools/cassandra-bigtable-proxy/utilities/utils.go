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
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	cql "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/third_party/cqlparser"
	"github.com/antlr4-go/antlr/v4"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
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
		return types.TypeBigInt, nil
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

func IsSupportedPrimaryKeyType(dt types.CqlDataType) bool {
	if dt.IsAnyFrozen() {
		return false
	}

	switch dt.DataType() {
	case datatype.Int, datatype.Bigint, datatype.Varchar, datatype.Timestamp, datatype.Blob, datatype.Ascii:
		return true
	default:
		return false
	}
}

func ParseBigInt(value string) (int64, error) {
	val, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("error converting string to int64: %w", err)
	}
	return val, err
}

func GoToString(value types.GoValue) (string, error) {
	if value == nil {
		return "null", nil
	}

	switch v := value.(type) {
	case string:
		escaped := strings.ReplaceAll(v, "'", "\\'")
		return "'" + escaped + "'", nil
	case int32:
		return strconv.Itoa(int(v)), nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32), nil
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	case bool:
		if v {
			return "1", nil
		} else {
			return "0", nil
		}
	case time.Time:
		return fmt.Sprintf("TIMESTAMP_FROM_UNIX_MILLIS(%d)", v.UnixMilli()), nil
	case []uint8:
		encoded := base64.StdEncoding.EncodeToString(v)
		// note: Bigtable limits SQL strings to 1,024k characters while Cassandra has no official limit. We only encode blobs to base64 when a literal is used. Placeholders will not have this issue.
		return fmt.Sprintf("FROM_BASE64('%s')", encoded), nil
	case []interface{}:
		var values []string
		for _, vi := range v {
			s, err := GoToString(vi)
			if err != nil {
				return "", err
			}
			values = append(values, s)
		}
		return fmt.Sprintf("[%s]", strings.Join(values, ", ")), nil
	default:
		return "", fmt.Errorf("unhandled go to string conversion for type %T", v)
	}
}

func StringToGo(value string, cqlType types.CqlDataType) (types.GoValue, error) {
	switch cqlType.DataType() {
	case datatype.Int:
		val, err := strconv.ParseInt(value, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("error converting string to int32: %w", err)
		}
		return int32(val), nil
	case datatype.Bigint, datatype.Counter:
		val, err := ParseBigInt(value)
		if err != nil {
			return nil, err
		}
		return val, nil
	case datatype.Float:
		val, err := strconv.ParseFloat(value, 32)
		if err != nil {
			return nil, fmt.Errorf("error converting string to float32: %w", err)
		}
		return float32(val), nil
	case datatype.Double:
		val, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, fmt.Errorf("error converting string to float64: %w", err)
		}
		return val, nil
	case datatype.Boolean:
		val, err := strconv.ParseBool(value)
		if err != nil {
			return nil, fmt.Errorf("error converting string to bool: %w", err)
		}
		return val, nil
	case datatype.Timestamp:
		val, err := parseCqlTimestamp(value)
		if err != nil {
			return nil, fmt.Errorf("error converting string to timestamp: %w", err)
		}
		return val, nil
	case datatype.Blob:
		// remove the '0x' prefix that CQL requires for blob literals
		if strings.HasPrefix(value, "0x") || strings.HasPrefix(value, "0X") {
			value = value[2:]
		}
		hexBytes, err := hex.DecodeString(value)
		if err != nil {
			return nil, err
		}
		return hexBytes, nil
	case datatype.Ascii:
		if err := ValidateData(value, cqlType); err != nil {
			return nil, err
		}
		return value, nil
	case datatype.Varchar:
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported CQL type: %s", cqlType.String())

	}
}

func ValidateData(value types.GoValue, dt types.CqlDataType) error {
	switch dt.Code() {
	case types.ASCII:
		s, ok := value.(string)
		if !ok {
			return fmt.Errorf("invalid data type for ascii: %T", value)
		}
		if err := validateASCII(s); err != nil {
			return err
		}
		return nil
	}
	return nil
}

func validateASCII(s string) error {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return fmt.Errorf("string is not valid ascii")
		}
	}
	return nil
}

func CreateKeyOrderedValueSlice[T any](data map[string]T) []T {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	result := make([]T, 0, len(keys))
	for _, k := range keys {
		result = append(result, data[k])
	}

	return result
}

var cqlTimestampFormats = []string{
	time.RFC3339,
	"2006-01-02 15:04:05",
	"2006-01-02 15:04-0700",
	"2006-01-02 15:04:05-0700",
	"2006-01-02 15:04:05.000-0700",
	"2006-01-02T15:04-0700",
	"2006-01-02T15:04:05-0700",
	"2006-01-02T15:04:05.000-0700",
	// it's possible to specify just the date - this will set the time to 00:00:00
	"2006-01-02",
}

// parseCqlTimestamp(): Parse a timestamp string in various formats.
// https://cassandra.apache.org/doc/4.1/cassandra/cql/types.html
func parseCqlTimestamp(timestampStr string) (time.Time, error) {
	// Try to parse the timestamp using each layout
	for _, format := range cqlTimestampFormats {
		parsedTime, err := time.Parse(format, timestampStr)
		if err == nil {
			return parsedTime.UTC(), nil
		}
	}

	if unixTime, err := strconv.ParseInt(timestampStr, 10, 64); err == nil {
		return time.UnixMilli(unixTime).UTC(), nil
	} else if floatTime, err := strconv.ParseFloat(timestampStr, 64); err == nil {
		unixTime := int64(floatTime)
		return time.UnixMilli(unixTime).UTC(), nil
	}

	return time.Time{}, fmt.Errorf("unable to parse timestamp for value '%s'", timestampStr)
}

func isSupportedCollectionElementType(dt datatype.DataType) bool {
	switch dt {
	case datatype.Int, datatype.Bigint, datatype.Varchar, datatype.Float, datatype.Double, datatype.Timestamp, datatype.Boolean, datatype.Ascii:
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
	case primitive.DataTypeCodeInt, primitive.DataTypeCodeBigint, primitive.DataTypeCodeBlob, primitive.DataTypeCodeBoolean, primitive.DataTypeCodeDouble, primitive.DataTypeCodeFloat, primitive.DataTypeCodeTimestamp, primitive.DataTypeCodeText, primitive.DataTypeCodeVarchar, primitive.DataTypeCodeCounter, primitive.DataTypeCodeAscii:
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
		return types.TypeBigInt, nil
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

func GetValueInt32(value types.DynamicValue, values *types.QueryParameterValues) (int32, error) {
	v, err := value.GetValue(values)
	if err != nil {
		return 0, err
	}
	intVal, ok := v.(int32)
	if !ok {
		return 0, fmt.Errorf("query value is a %T, not an int32", v)
	}
	return intVal, nil
}

func GetValueInt64(value types.DynamicValue, values *types.QueryParameterValues) (int64, error) {
	v, err := value.GetValue(values)
	if err != nil {
		return 0, err
	}
	intVal, ok := v.(int64)
	if !ok {
		return 0, fmt.Errorf("query value is a %T, not an int64", v)
	}
	return intVal, nil
}

func GetValueSlice(value types.DynamicValue, values *types.QueryParameterValues) ([]types.GoValue, error) {
	v, err := value.GetValue(values)
	if err != nil {
		return nil, err
	}

	val := reflect.ValueOf(v)

	if val.Kind() != reflect.Slice {
		return nil, fmt.Errorf("query value is a %T, not a slice", v)
	}

	length := val.Len()
	result := make([]types.GoValue, length)

	for i := 0; i < length; i++ {
		result[i] = val.Index(i).Interface()
	}

	return result, nil
}

func GetValueMap(value types.DynamicValue, values *types.QueryParameterValues) (map[types.GoValue]types.GoValue, error) {
	v, err := value.GetValue(values)
	if err != nil {
		return nil, err
	}
	val := reflect.ValueOf(v)

	if val.Kind() != reflect.Map {
		return nil, fmt.Errorf("value is a %T, not a map", v)
	}

	result := make(map[types.GoValue]types.GoValue, val.Len())

	// 3. Iterate over the keys of the original map
	iter := val.MapRange()
	for iter.Next() {
		// Get the reflection Parameter for the key and the value
		keyVal := iter.Key()
		valueVal := iter.Value()

		// 4. Use .Interface() to convert the concrete key/value to an any (interface{})
		keyAny := keyVal.Interface()
		valueAny := valueVal.Interface()

		// 5. Add to the new map[any]any
		result[keyAny] = valueAny
	}
	return result, nil
}
