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

package translator

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"cloud.google.com/go/bigtable"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/collectiondecoder"
	constants "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/constants"
	types "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	schemaMapping "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/schema-mapping"
	cql "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/third_party/cqlparser"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/third_party/datastax/proxycore"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/utilities"
	"github.com/antlr4-go/antlr/v4"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
)

const (
	literalPlaceholder = "_a1b2c3"
	tsValuePlaceholder = "tsValue"
	bigtableTSColumn   = "last_commit_ts"
	limitPlaceholder   = "limitValue"
	writetime          = "writetime"
	commitTsFn         = "PENDING_COMMIT_TIMESTAMP()"
	missingUndefined   = "<missing undefined>"
	missing            = "<missing"
	questionMark       = "?"
	STAR               = "*"
	maxNanos           = int32(9999)
	referenceTime      = int64(1262304000000)
	ifExists           = "ifexists"
)

var (
	whereRegex     = regexp.MustCompile(`(?i)\bwhere\b`)
	orderByRegex   = regexp.MustCompile(`(?i)\border\s+by\b`)
	groupByRegex   = regexp.MustCompile(`(?i)\bgroup\s+by\b`)
	limitRegex     = regexp.MustCompile(`(?i)\blimit\b`)
	combinedRegex  = createCombinedRegex()
	validTableName = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
)

var (
	ErrEmptyTable           = errors.New("could not find table name to create spanner query")
	ErrParsingDelObj        = errors.New("error while parsing delete object")
	ErrParsingTs            = errors.New("timestamp could not be parsed")
	ErrTsNoValue            = errors.New("no value found for Timestamp")
	ErrEmptyTableOrKeyspace = errors.New("ToBigtableDelete: No table or keyspace name found in the query")
)

// ComplexAssignment represents a complex update operation on a column.
// It contains the column name, operation type, and left/right operands for the operation.
type ComplexAssignment struct {
	Column    string
	Operation string
	Left      interface{}
	Right     interface{}
}

// hasUsingTimestamp checks if the query contains a USING TIMESTAMP clause.
// Returns true if the query includes timestamp specification.
func hasUsingTimestamp(query string) bool {
	return strings.Contains(strings.ToUpper(query), "USING TIMESTAMP")
}

// parseTimestamp(): Parse a timestamp string in various formats.
// Supported formats
// "2024-02-05T14:00:00Z",
// "2024-02-05 14:00:00",
// "2024/02/05 14:00:00",
// "1672522562",             // Unix timestamp (seconds)
// "1672522562000",          // Unix timestamp (milliseconds)
// "1672522562000000",       // Unix timestamp (microseconds)
func parseTimestamp(timestampStr string) (time.Time, error) {
	// Define multiple layouts to try
	layouts := []string{
		time.RFC3339,          // ISO 8601 format
		"2006-01-02 15:04:05", // Common date-time format
		"2006/01/02 15:04:05", // Common date-time format with slashes
	}

	var parsedTime time.Time
	var err error

	// Try to parse the timestamp using each layout
	for _, layout := range layouts {
		if timestampStr == "totimestamp(now())" {
			timestampStr = time.Now().Format(time.RFC3339)
		}
		parsedTime, err = time.Parse(layout, timestampStr)
		if err == nil {
			return parsedTime, nil
		}
	}

	// Try to parse as Unix timestamp (in seconds, milliseconds, or microseconds)
	if unixTime, err := strconv.ParseInt(timestampStr, 10, 64); err == nil {
		switch len(timestampStr) {
		case 10: // Seconds
			return time.Unix(unixTime, 0).UTC(), nil
		case 13: // Milliseconds
			return time.Unix(0, unixTime*int64(time.Millisecond)).UTC(), nil
		case 16: // Microseconds
			return time.Unix(0, unixTime*int64(time.Microsecond)).UTC(), nil
		}
	}

	// If all formats fail, return the last error
	return time.Time{}, err
}

// hasWhere checks if the query contains a WHERE clause.
// Returns true if the query includes a WHERE condition.
func hasWhere(query string) bool {
	return whereRegex.MatchString(query)
}

// hasOrderBy checks if the query contains an ORDER BY clause.
// Returns true if the query includes sorting specifications.
func hasOrderBy(query string) bool {
	return orderByRegex.MatchString(query)
}

// hasGroupBy checks if the query contains a GROUP BY clause.
// Returns true if the query includes grouping specifications.
func hasGroupBy(query string) bool {
	return groupByRegex.MatchString(query)
}

// hasLimit checks if the query contains a LIMIT clause.
// Returns true if the query includes a row limit specification.
func hasLimit(query string) bool {
	return limitRegex.MatchString(query)
}

// formatValues converts a value to its byte representation based on CQL type.
// Handles type conversion and encoding according to the protocol version.
// Returns error if value type is invalid or encoding fails.
func formatValues(value string, cqlType datatype.DataType, protocolV primitive.ProtocolVersion) ([]byte, error) {
	var iv interface{}
	var dt datatype.DataType

	switch cqlType {
	case datatype.Int:
		if strings.ToLower(value) == "null" {
			return nil, nil
		}
		return EncodeBigInt(value, protocolV)
	case datatype.Bigint:
		if strings.ToLower(value) == "null" {
			return nil, nil
		}
		val, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("error converting string to int64: %w", err)
		}
		iv = val
		dt = datatype.Bigint

	case datatype.Float:
		if strings.ToLower(value) == "null" {
			return nil, nil
		}
		val, err := strconv.ParseFloat(value, 32)
		if err != nil {
			return nil, fmt.Errorf("error converting string to float32: %w", err)
		}
		iv = float32(val)
		dt = datatype.Float
	case datatype.Double:
		if strings.ToLower(value) == "null" {
			return nil, nil
		}
		val, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, fmt.Errorf("error converting string to float64: %w", err)
		}
		iv = val
		dt = datatype.Double

	case datatype.Boolean:
		if strings.ToLower(value) == "null" {
			return nil, nil
		}
		return EncodeBool(value, protocolV)
	case datatype.Timestamp:
		if strings.ToLower(value) == "null" {
			return nil, nil
		}
		val, err := parseTimestamp(value)
		if err != nil {
			return nil, fmt.Errorf("error converting string to timestamp: %w", err)
		}
		iv = val
		dt = datatype.Timestamp
	case datatype.Blob:
		if strings.ToLower(value) == "null" {
			return nil, nil
		}
		iv = value
		dt = datatype.Blob
	case datatype.Varchar:
		if strings.ToLower(value) == "null" {
			return nil, nil
		}
		iv = value
		dt = datatype.Varchar

	default:
		return nil, fmt.Errorf("unsupported CQL type: %s", cqlType)
	}

	bd, err := proxycore.EncodeType(dt, protocolV, iv)
	if err != nil {
		return nil, fmt.Errorf("error encoding value: %w", err)
	}

	return bd, nil

}

// primitivesToString converts a primitive value to its string representation.
// Handles various data types and returns a formatted string.
// Returns error if value type is invalid or conversion fails.
func primitivesToString(val interface{}) (string, error) {
	switch v := val.(type) {
	case string:
		return v, nil
	case int32:
		return strconv.Itoa(int(v)), nil
	case int:
		return strconv.Itoa(v), nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32), nil
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	case bool:
		return strconv.FormatBool(v), nil
	default:
		return "", fmt.Errorf("unsupported type: %T", v)
	}
}

// stringToPrimitives converts a string value to its primitive type based on CQL type.
// Performs type conversion according to the specified CQL data type.
// Returns error if value type is invalid or conversion fails.
func stringToPrimitives(value string, cqlType datatype.DataType) (interface{}, error) {
	var iv interface{}

	switch cqlType {
	case datatype.Int:
		val, err := strconv.ParseInt(value, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("error converting string to int32: %w", err)
		}
		iv = int32(val)

	case datatype.Bigint, datatype.Counter:
		val, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("error converting string to int64: %w", err)
		}
		iv = val

	case datatype.Float:
		val, err := strconv.ParseFloat(value, 32)
		if err != nil {
			return nil, fmt.Errorf("error converting string to float32: %w", err)
		}
		iv = float32(val)

	case datatype.Double:
		val, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, fmt.Errorf("error converting string to float64: %w", err)
		}
		iv = val
	case datatype.Boolean:
		val, err := strconv.ParseBool(value)
		if err != nil {
			return nil, fmt.Errorf("error converting string to bool: %w", err)
		}
		if val {
			iv = int64(1)
		} else {
			iv = int64(0)
		}

	case datatype.Timestamp:
		val, err := parseTimestamp(value)

		if err != nil {
			return nil, fmt.Errorf("error converting string to timestamp: %w", err)
		}
		iv = val
	case datatype.Blob:
		iv = value
	case datatype.Varchar:
		iv = value

	default:
		return nil, fmt.Errorf("unsupported CQL type: %s", cqlType)

	}
	return iv, nil
}

// cqlTypeToEmptyPrimitive returns a keeping type value for a given CQL type.
func cqlTypeToEmptyPrimitive(cqlType datatype.DataType, isPrimaryKey bool) interface{} {
	switch cqlType {
	case datatype.Int:
		return int32(0)
	case datatype.Bigint:
		return int64(0)
	case datatype.Float:
		return float32(0)
	case datatype.Double:
		return float64(0)
	case datatype.Boolean:
		return false
	case datatype.Timestamp:
		return time.Time{}
	case datatype.Blob:
		return []byte{}
	case datatype.Varchar:
		if !isPrimaryKey {
			return []byte{}
		}
		return string("")
	}
	return nil
}

// createCombinedRegex creates a compiled regex pattern for query parsing.
// Combines multiple patterns for efficient query analysis.
func createCombinedRegex() *regexp.Regexp {
	keywords := []string{"time", "key", "type", "json"}
	combinedPattern := fmt.Sprintf("\\b(%s)\\b", regexp.QuoteMeta(keywords[0]))

	for _, keyword := range keywords[1:] {
		combinedPattern += fmt.Sprintf("|\\b%s\\b", regexp.QuoteMeta(keyword))
	}
	return regexp.MustCompile(combinedPattern)
}

// renameLiterals processes and renames literals in the query.
// Standardizes literal values for consistent processing.
func renameLiterals(query string) string {
	return combinedRegex.ReplaceAllStringFunc(query, func(match string) string {
		return match + literalPlaceholder
	})
}

// getPrimaryKeys retrieves the primary key columns for a given table.
// Returns the list of primary key column names from the schema mapping.
// Returns error if table doesn't exist or schema mapping fails.
func getPrimaryKeys(schemaMapping *schemaMapping.SchemaMappingConfig, tableName string, keySpace string) ([]string, error) {
	var primaryKeys []string
	pks, err := schemaMapping.GetPkByTableName(tableName, keySpace)
	if err != nil {
		return nil, err
	}

	for _, pk := range pks {
		primaryKeys = append(primaryKeys, pk.ColumnName)
	}

	return primaryKeys, nil
}

// processCollectionColumnsForRawQueries() processes collection columns in raw CQL queries.
// Handles list, set, and map collection types with their respective operations.
// For each column:
//   - Skips primary key columns
//   - For collection columns: processes based on type (list/set/map) and operation
//   - For non-collection columns: adds to output as-is
//
// Returns ProcessRawCollectionsOutput containing processed columns and values, or error if processing fails.
func processCollectionColumnsForRawQueries(input ProcessRawCollectionsInput) (*ProcessRawCollectionsOutput, error) {
	output := &ProcessRawCollectionsOutput{ComplexMeta: make(map[string]*ComplexOperation)}
	for i, column := range input.Columns {
		if column.IsPrimaryKey {
			continue
		}
		val := input.Values[i]
		if input.Translator.IsCollection(input.KeySpace, input.TableName, column.Name) {
			colFamily := input.Translator.GetColumnFamily(input.KeySpace, input.TableName, column.Name)
			switch column.CQLType.GetDataTypeCode() {
			case primitive.DataTypeCodeList:
				if err := handleListOperation(val, column, colFamily, input, output); err != nil {
					return nil, err
				}
			case primitive.DataTypeCodeSet:
				if err := handleSetOperation(val, column, colFamily, input, output); err != nil {
					return nil, err
				}
			case primitive.DataTypeCodeMap:
				if err := handleMapOperation(val, column, colFamily, input, output); err != nil {
					return nil, err
				}
			default:
				return nil, fmt.Errorf("column %s is not a collection type", column.Name)
			}
		} else if column.CQLType == datatype.Counter {
			if err := handleCounterOperation(val, column, input, output); err != nil {
				return nil, err
			}
		} else {
			output.NewColumns = append(output.NewColumns, column)
			output.NewValues = append(output.NewValues, val)
		}
	}
	return output, nil
}

// handleListOperation processes list operations in raw queries.
// Manages simple assignment, append, prepend, and index-based operations on list columns.
// Returns error if operation type is invalid or value type doesn't match expected type.
func handleListOperation(val interface{}, column types.Column, colFamily string, input ProcessRawCollectionsInput, output *ProcessRawCollectionsOutput) error {
	switch v := val.(type) {
	case ComplexAssignment:
		switch v.Operation {
		case "+":
			valueToProcess := v.Right
			if input.PrependColumns != nil && slices.Contains(input.PrependColumns, column.Name) {
				valueToProcess = v.Left
			}
			var listValues []string = valueToProcess.([]string)
			return addListElements(listValues, colFamily, column, input, output)
		case "-":
			keys, ok := v.Right.([]string)
			if !ok {
				return fmt.Errorf("expected []string for remove operation, got %T", v.Right)
			}
			return removeListElements(keys, colFamily, column, output)
		case "update_index":
			idx, ok := v.Left.(string)
			if !ok {
				return fmt.Errorf("expected string for index, got %T", v.Left)
			}
			listType, _ := column.CQLType.(datatype.ListType)
			dt := listType.GetElementType()

			return updateListIndex(idx, v.Right, colFamily, dt, output)
		default:
			return fmt.Errorf("unsupported list operation: %s", v.Operation)
		}
	default:
		// Simple assignment (replace)
		output.DelColumnFamily = append(output.DelColumnFamily, column.Name)
		var listValues []string
		if val != nil {
			switch v := val.(type) {
			case []string:
				listValues = v
			default:
				return fmt.Errorf("expected []string for list operation, got %T", val)
			}
		}

		return addListElements(listValues, colFamily, column, input, output)
	}
}

// addListElements adds elements to a list column in raw queries.
// Handles both append and prepend operations with type validation and conversion.
// Returns error if value type doesn't match list element type or conversion fails.
func addListElements(listValues []string, colFamily string, column types.Column, input ProcessRawCollectionsInput, output *ProcessRawCollectionsOutput) error {
	listType, ok := column.CQLType.(datatype.ListType)
	if !ok {
		return fmt.Errorf("failed to assert list type for %v", column.CQLType)
	}
	dt := listType.GetElementType()
	prepend := false
	if slices.Contains(input.PrependColumns, colFamily) {
		prepend = true
	}
	for i, v := range listValues {

		// Calculate encoded timestamp for the list element
		encTime := getEncodedTimestamp(i, len(listValues), prepend)

		// Create a new column identifier with the encoded timestamp as the name
		c := types.Column{
			Name:         string(encTime),
			ColumnFamily: colFamily,
			CQLType:      dt,
		}
		// Format the value
		formattedVal, err := formatValues(v, dt, primitive.ProtocolVersion4)
		if err != nil {
			return fmt.Errorf("error converting string to list<%s> value: %w", dt, err)
		}
		output.NewColumns = append(output.NewColumns, c)
		output.NewValues = append(output.NewValues, formattedVal)
	}
	return nil
}

// removeListElements removes elements from a list column in raw queries.
// Processes element removal by index with validation of index bounds.
// Returns error if index is invalid or out of bounds.
func removeListElements(keys []string, colFamily string, column types.Column, output *ProcessRawCollectionsOutput) error {
	var listDelete [][]byte
	listType, ok := column.CQLType.(datatype.ListType)
	if !ok {
		return fmt.Errorf("failed to assert list type for %v", column.CQLType)
	}

	listElementType := listType.GetElementType()

	output.ComplexMeta[colFamily] = &ComplexOperation{
		Delete:     true,
		ListDelete: true,
	}

	for _, col := range keys {
		formattedVal, err := formatValues(col, listElementType, primitive.ProtocolVersion4)
		if err != nil {
			return fmt.Errorf("error converting string to list<%s> value: %w", listElementType, err)
		}
		listDelete = append(listDelete, formattedVal)
	}
	if len(listDelete) > 0 {
		if meta, ok := output.ComplexMeta[colFamily]; ok {
			meta.ListDeleteValues = listDelete
		} else {
			output.ComplexMeta[colFamily] = &ComplexOperation{ListDeleteValues: listDelete}
		}
	}
	return nil
}

// updateListIndex updates a specific index in a list column.
// Handles type conversion and validation for the new value.
// Returns error if index is invalid or value type doesn't match list element type.
func updateListIndex(index string, value interface{}, colFamily string, dt datatype.DataType, output *ProcessRawCollectionsOutput) error {

	valStr, err := primitivesToString(value)
	if err != nil {
		return err
	}

	val, err := formatValues(valStr, dt, primitive.ProtocolVersion4)
	if err != nil {
		return err
	}
	output.ComplexMeta[colFamily] = &ComplexOperation{
		UpdateListIndex: index,
		Value:           val,
	}
	return nil
}

// handleSetOperation processes set operations in raw queries.
// Manages simple assignment, add, and remove operations on set columns.
// Returns error if operation type is invalid or value type doesn't match set element type.
func handleSetOperation(val interface{}, column types.Column, colFamily string, input ProcessRawCollectionsInput, output *ProcessRawCollectionsOutput) error {
	switch v := val.(type) {
	case ComplexAssignment:
		switch v.Operation {
		case "+":
			setValues, ok := v.Right.([]string)
			if !ok {
				return fmt.Errorf("expected []string for remove operation, got %T", v.Right)
			}
			return addSetElements(setValues, colFamily, column, input, output)
		case "-":
			keys, ok := v.Right.([]string)
			if !ok {
				return fmt.Errorf("expected []string for remove operation, got %T", v.Right)
			}
			return removeSetElements(keys, colFamily, column, output)
		default:
			return fmt.Errorf("unsupported set operation: %s", v.Operation)
		}
	default:
		// Simple assignment (replace)
		output.DelColumnFamily = append(output.DelColumnFamily, column.Name)
		return addSetElements(val.([]string), colFamily, column, input, output)
	}
}

// addSetElements adds elements to a set column in raw queries.
// Handles element addition with type validation and conversion.
// Returns error if value type doesn't match set element type or conversion fails.
func addSetElements(setValues []string, colFamily string, column types.Column, input ProcessRawCollectionsInput, output *ProcessRawCollectionsOutput) error {
	if output == nil {
		return fmt.Errorf("output cannot be nil")
	}

	valueType := column.CQLType.(datatype.SetType).GetElementType()
	for _, v := range setValues {
		// Convert value based on the element type
		valInInterface, err := DataConversionInInsertionIfRequired(v, primitive.ProtocolVersion4, valueType, "string")
		if err != nil {
			return fmt.Errorf("error converting string to set<%s> value: %w", valueType, err)
		}

		// For boolean values, we need to convert true/false to 1/0
		if valueType == datatype.Boolean {
			strVal, ok := valInInterface.(string)
			if !ok {
				return fmt.Errorf("expected string for boolean, got %T", valInInterface)
			}
			boolVal, err := strconv.ParseBool(strVal)
			if err != nil {
				return fmt.Errorf("invalid boolean string: %v", strVal)
			}
			if boolVal {
				v = "1"
			} else {
				v = "0"
			}
		}
		col := types.Column{
			Name:         v,
			ColumnFamily: colFamily,
			CQLType:      valueType,
		}
		// value parameter is intentionally empty because column identifier holds the value
		output.NewColumns = append(output.NewColumns, col)
		output.NewValues = append(output.NewValues, []byte(""))
	}
	return nil
}

// removeSetElements removes elements from a set column in raw queries.
// Processes element removal with validation of element existence.
// Returns error if element type doesn't match set element type.
func removeSetElements(keys []string, colFamily string, column types.Column, output *ProcessRawCollectionsOutput) error {
	for _, col := range keys {
		output.DelColumns = append(output.DelColumns, types.Column{
			Name:         col,
			ColumnFamily: colFamily,
		})
	}
	return nil
}

func handleCounterOperation(val interface{}, column types.Column, input ProcessRawCollectionsInput, output *ProcessRawCollectionsOutput) error {
	switch v := val.(type) {
	case ComplexAssignment:
		switch v.Operation {
		case "+", "-":
			str, ok := v.Right.(string)
			if !ok {
				return fmt.Errorf("expected string for counter operation, got %T", val)
			}
			intVal, err := strconv.ParseInt(str, 10, 64)
			if err != nil {
				return err
			}
			var op = Increment
			if v.Operation == "-" {
				op = Decrement
			}
			output.ComplexMeta[column.Name] = &ComplexOperation{
				IncrementType:  op,
				IncrementValue: intVal,
			}
			return nil
		default:
			return fmt.Errorf("unsupported counter operation: %s", v.Operation)
		}
	default:
		return fmt.Errorf("unsupported counter operation")
	}
}

// handleMapOperation processes map operations in raw queries.
// Manages simple assignment/replace complete map, add, remove, and update at index operations on map columns.
// Returns error if operation type is invalid or value type doesn't match map key/value types.
func handleMapOperation(val interface{}, column types.Column, colFamily string, input ProcessRawCollectionsInput, output *ProcessRawCollectionsOutput) error {
	mapType, ok := column.CQLType.(datatype.MapType)
	if !ok {
		return fmt.Errorf("failed to assert map type for %v", column.CQLType)
	}

	var keyType datatype.DataType = mapType.GetKeyType()
	var dt datatype.DataType = mapType.GetValueType()

	// Check if key type is VARCHAR or TIMESTAMP
	if keyType.GetDataTypeCode() == primitive.DataTypeCodeVarchar || keyType.GetDataTypeCode() == primitive.DataTypeCodeTimestamp {

		switch v := val.(type) {
		case ComplexAssignment:
			switch v.Operation {
			case "+":
				return addMapEntries(v.Right, colFamily, dt, column, input, output)
			case "-":
				return removeMapEntries(v.Right, colFamily, column, output)
			case "update_index":
				return updateMapIndex(v.Left, v.Right, dt, colFamily, output)
			default:
				return fmt.Errorf("unsupported map operation: %s", v.Operation)
			}
		default:
			// Simple assignment (replace)
			output.DelColumnFamily = append(output.DelColumnFamily, column.Name)
			return addMapEntries(val, colFamily, dt, column, input, output)
		}
	} else {
		return fmt.Errorf("unsupported map key type: %v", keyType)
	}
}

// addMapEntries adds key-value pairs to a map column in raw queries.
// Handles type validation and conversion for both keys and values.
// Returns error if key/value types don't match map types or conversion fails.
func addMapEntries(val interface{}, colFamily string, valDt datatype.DataType, column types.Column, input ProcessRawCollectionsInput, output *ProcessRawCollectionsOutput) error {
	mapValue, ok := val.(map[string]string)
	if !ok {
		return fmt.Errorf("expected map[string]interface{} for add operation, got %T", val)
	}

	cqlType := valDt
	if valDt == datatype.Boolean || valDt == datatype.Int {
		cqlType = datatype.Bigint
	}
	for k, v := range mapValue {
		col := types.Column{
			Name:         k,
			ColumnFamily: colFamily,
			CQLType:      cqlType,
		}
		valueFormatted, err := formatValues(v, valDt, primitive.ProtocolVersion4)
		if err != nil {
			return fmt.Errorf("error converting string to map<varchar,%s> value: %w", valDt, err)
		}
		output.NewColumns = append(output.NewColumns, col)
		output.NewValues = append(output.NewValues, valueFormatted)
	}
	return nil
}

// removeMapEntries removes key-value pairs from a map column in raw queries.
// Processes key removal with validation of key existence and type.
// Returns error if key type doesn't match map key type.
func removeMapEntries(val interface{}, colFamily string, column types.Column, output *ProcessRawCollectionsOutput) error {
	keys, ok := val.([]string)
	if !ok {
		return fmt.Errorf("expected []string for remove operation, got %T", val)
	}
	for _, col := range keys {
		output.DelColumns = append(output.DelColumns, types.Column{
			Name:         col,
			ColumnFamily: colFamily,
		})
	}
	return nil
}

// updateMapIndex updates a specific key in a map column.
// Handles type conversion and validation for both key and value.
// Returns error if key doesn't exist or value type doesn't match map value type.
func updateMapIndex(key interface{}, value interface{}, dt datatype.DataType, colFamily string, output *ProcessRawCollectionsOutput) error {
	k, ok := key.(string)
	if !ok {
		return fmt.Errorf("expected string for map key, got %T", key)
	}
	v, ok := value.(string)
	if !ok {
		return fmt.Errorf("expected string for map value, got %T", key)
	}

	val, err := formatValues(v, dt, primitive.ProtocolVersion4)
	if err != nil {
		return err
	}
	// For map index update, treat as a single entry add
	col := types.Column{
		Name:         k,
		ColumnFamily: colFamily,
		CQLType:      dt,
	}
	output.NewColumns = append(output.NewColumns, col)
	output.NewValues = append(output.NewValues, val)
	return nil
}

// processCollectionColumnsForPrepareQueries handles collection operations in prepared queries.
// Processes set, list, and map operations.
// Returns error if collection type is invalid or value encoding fails.
func processCollectionColumnsForPrepareQueries(input ProcessPrepareCollectionsInput) (*ProcessPrepareCollectionsOutput, error) {
	output := &ProcessPrepareCollectionsOutput{
		Unencrypted: make(map[string]interface{}),
	}

	for i, column := range input.ColumnsResponse {
		output.IndexEnd = i
		if column.IsPrimaryKey && slices.Contains(input.PrimaryKeys, column.Name) {
			val, _ := utilities.DecodeBytesToCassandraColumnType(input.Values[i].Contents, column.CQLType, input.ProtocolV)
			output.Unencrypted[column.Name] = val
			continue
		}
		// todo validate the column exists again, in case the column was dropped after the query was prepared.
		if input.Translator.IsCollection(input.KeySpace, input.TableName, column.Name) {

			colFamily := input.Translator.GetColumnFamily(input.KeySpace, input.TableName, column.Name)
			switch column.CQLType.GetDataTypeCode() {
			case primitive.DataTypeCodeList:
				if err := handlePrepareListOperation(input.Values[i].Contents, column, colFamily, input, output); err != nil {
					return nil, err
				}
				continue
			case primitive.DataTypeCodeSet:
				if err := handlePrepareSetOperation(input.Values[i], column, colFamily, input, output); err != nil {
					return nil, err
				}
				continue
			case primitive.DataTypeCodeMap:
				if err := handlePrepareMapOperation(input.Values[i], column, colFamily, input, output); err != nil {
					return nil, err
				}
				continue

			default:
				return nil, fmt.Errorf("column %s is not a collection type", column.Name)
			}
		} else if column.CQLType == datatype.Counter {
			decodedValue, err := proxycore.DecodeType(datatype.Counter, input.ProtocolV, input.Values[i].Contents)
			if err != nil {
				return nil, err
			}
			intVal, ok := decodedValue.(int64)
			if !ok {
				return nil, fmt.Errorf("failed to convert counter param")
			}
			// todo handle negatives?

			meta, ok := input.ComplexMeta[column.Name]
			if !ok {
				return nil, fmt.Errorf("unexpected state: no existing operation for counter param")
			}

			meta.IncrementValue = intVal
		} else {
			valInInterface, err := DataConversionInInsertionIfRequired(input.Values[i].Contents, input.ProtocolV, column.CQLType, "byte")
			if err != nil {
				return nil, fmt.Errorf("error converting primitives: %w", err)
			}
			input.Values[i].Contents = valInInterface.([]byte)
			output.NewColumns = append(output.NewColumns, column)
			output.NewValues = append(output.NewValues, input.Values[i].Contents)
		}
	}
	return output, nil
}

// handlePrepareListOperation processes list operations in prepared queries.
// Manages list operations with protocol version-specific value encoding.
// Returns error if value type doesn't match list element type or encoding fails.
func handlePrepareListOperation(val []byte, column types.Column, colFamily string, input ProcessPrepareCollectionsInput, output *ProcessPrepareCollectionsOutput) error {

	listType, ok := column.CQLType.(datatype.ListType)
	if !ok {
		return fmt.Errorf("failed to assert list type for %v", column.CQLType)
	}
	dt := listType.GetElementType()

	if meta, ok := input.ComplexMeta[column.Name]; ok {
		var complexUpdateMeta *ComplexOperation = meta

		if complexUpdateMeta.UpdateListIndex != "" {
			// list index operation e.g. list[index]=val
			valInInterface, err := DataConversionInInsertionIfRequired(val, input.ProtocolV, dt, "byte")
			if err != nil {
				return fmt.Errorf("error while encoding list<%s> value: %w", listType, err)
			}
			complexUpdateMeta.Value = valInInterface.([]byte)
		} else {
			return preprocessList(val, colFamily, complexUpdateMeta, dt, column, input, output)
		}
	} else {
		output.DelColumnFamily = append(output.DelColumnFamily, column.Name)
		return preprocessList(val, colFamily, nil, dt, column, input, output)
	}
	return nil
}

// preprocessList prepares list values for processing in prepared queries.
// Handles initial list value preparation, type validation, and conversion.
// Returns error if value type doesn't match list element type or conversion fails.
func preprocessList(val []byte, colFamily string, complexUpdateMeta *ComplexOperation, dt datatype.DataType, column types.Column, input ProcessPrepareCollectionsInput, output *ProcessPrepareCollectionsOutput) error {

	switch dt {
	case datatype.Varchar:
		return processList[string](val, colFamily, complexUpdateMeta, dt, column, input, output)
	case datatype.Int:
		return processList[int32](val, colFamily, complexUpdateMeta, dt, column, input, output)
	case datatype.Bigint, datatype.Timestamp:
		return processList[int64](val, colFamily, complexUpdateMeta, dt, column, input, output)
	case datatype.Float:
		return processList[float32](val, colFamily, complexUpdateMeta, dt, column, input, output)
	case datatype.Double:
		return processList[float64](val, colFamily, complexUpdateMeta, dt, column, input, output)
	case datatype.Boolean:
		return processList[bool](val, colFamily, complexUpdateMeta, dt, column, input, output)
	default:
		return fmt.Errorf("Invalid list value type")
	}
}

// processList() handles generic list processing operations in prepared queries.
// Manages list operations with type-specific handling and protocol version encoding.
// Returns error if value type doesn't match list element type or encoding fails.
func processList[V any](val []byte, colFamily string, complexUpdateMeta *ComplexOperation, dt datatype.DataType, column types.Column, input ProcessPrepareCollectionsInput, output *ProcessPrepareCollectionsOutput) error {
	listValDatatype := dt
	listDT := column.CQLType
	if dt == datatype.Timestamp {
		listValDatatype = datatype.Bigint
		listDT = utilities.ListOfBigInt
	}

	decodedValue, err := collectiondecoder.DecodeCollection(listDT, input.ProtocolV, val)
	if err != nil {
		return fmt.Errorf("error decoding string to %s value: %w", column.CQLType, err)
	}
	listValues, ok := decodedValue.([]V)
	if !ok {
		return fmt.Errorf("unexpected type %s:", listValDatatype)
	}

	var listDelete [][]byte
	prepend := false
	// Iterate over the elements in the List
	for i, elem := range listValues {
		valueStr, err := primitivesToString(elem)
		if err != nil {
			return fmt.Errorf("failed to convert element to string: %w", err)
		}
		if complexUpdateMeta != nil && complexUpdateMeta.ListDelete {
			val, _ := formatValues(valueStr, listValDatatype, input.ProtocolV)
			if err != nil {
				return err
			}
			listDelete = append(listDelete, val)
			continue
		}
		if complexUpdateMeta != nil {
			prepend = complexUpdateMeta.PrependList

		}

		encTime := getEncodedTimestamp(i, len(listValues), prepend)

		c := types.Column{
			Name:         string(encTime),
			ColumnFamily: colFamily,
			CQLType:      listValDatatype,
		}
		// The value parameter is intentionally empty because the column identifier has the value
		val, _ := formatValues(valueStr, listValDatatype, input.ProtocolV)
		output.NewValues = append(output.NewValues, val)
		output.NewColumns = append(output.NewColumns, c)
	}
	if len(listDelete) > 0 {
		complexUpdateMeta.ListDeleteValues = listDelete
	}
	return nil
}

// handlePrepareMapOperation processes map operations in prepared queries.
// Manages map operations, append, delete, and update at index and simple assignment.
// Returns error if value type doesn't match map key/value types or encoding fails.
func handlePrepareMapOperation(val *primitive.Value, column types.Column, colFamily string, input ProcessPrepareCollectionsInput, output *ProcessPrepareCollectionsOutput) error {
	mapType, ok := column.CQLType.(datatype.MapType)
	var keyType datatype.DataType = mapType.GetKeyType()
	var valueDatatype datatype.DataType = mapType.GetValueType()

	if !ok {
		return fmt.Errorf("failed to assert map type for %v", column.CQLType)
	}
	if meta, ok := input.ComplexMeta[column.Name]; ok {

		var compleUpdateMeta *ComplexOperation = meta
		if compleUpdateMeta.Append && compleUpdateMeta.mapKey != nil { //handling update for specific key e.g map[key]=val
			return processMapKeyAppend(column, compleUpdateMeta, input, val, output, valueDatatype, keyType)

		} else if compleUpdateMeta.Delete { //handling Delete for specific key e.g map=map-['key1','key2']

			expectedDt := compleUpdateMeta.ExpectedDatatype
			if expectedDt == nil {
				expectedDt = column.CQLType
			}

			if keyType == datatype.Timestamp {
				expectedDt = utilities.SetOfBigInt
			}
			return processDeleteOperationForMapAndSet(column, compleUpdateMeta, input, val, output, expectedDt)
		} else {
			if err := handleMapProcessing(val, column, colFamily, valueDatatype, input.ProtocolV, output, keyType, valueDatatype); err != nil {
				return err
			}
		}
	} else {
		// case of when new value is being set for collection type {e.g collection=newCollection}
		output.DelColumnFamily = append(output.DelColumnFamily, column.Name)
		if err := handleMapProcessing(val, column, colFamily, valueDatatype, input.ProtocolV, output, keyType, valueDatatype); err != nil {
			return err
		}
	}
	return nil
}

// handleMapProcessing processes map values for prepared queries.
// Handles map value preparation, calls processMap based on value type.
// Returns error if value type doesn't match map key/value types or encoding fails.
func handleMapProcessing(val *primitive.Value, column types.Column, colFamily string, valueDatatype datatype.DataType, protocolV primitive.ProtocolVersion, output *ProcessPrepareCollectionsOutput, keyType datatype.DataType, valueType datatype.DataType) error {
	switch valueDatatype {
	case datatype.Varchar:
		if err := processMap[string](val, column, colFamily, valueDatatype, protocolV, output, keyType, valueType); err != nil {
			return err
		}
	case datatype.Int:
		if err := processMap[int32](val, column, colFamily, valueDatatype, protocolV, output, keyType, valueType); err != nil {
			return err
		}
	case datatype.Bigint, datatype.Timestamp:
		if err := processMap[int64](val, column, colFamily, valueDatatype, protocolV, output, keyType, valueType); err != nil {
			return err
		}
	case datatype.Float:
		if err := processMap[float32](val, column, colFamily, valueDatatype, protocolV, output, keyType, valueType); err != nil {
			return err
		}
	case datatype.Double:
		if err := processMap[float64](val, column, colFamily, valueDatatype, protocolV, output, keyType, valueType); err != nil {
			return err
		}
	case datatype.Boolean:
		if err := processMap[bool](val, column, colFamily, valueDatatype, protocolV, output, keyType, valueType); err != nil {
			return err
		}
	}
	return nil
}

// processMap handles generic map processing operations in prepared queries.
// Manages map operations with type-specific handling map(timestamp,varchar) or map(varchar,timestamp)
// Returns error if value type doesn't match map key/value types or encoding fails.
func processMap[V any](
	val *primitive.Value,
	column types.Column,
	colFamily string,
	valueDatatype datatype.DataType,
	protocolV primitive.ProtocolVersion,
	output *ProcessPrepareCollectionsOutput,
	keyType datatype.DataType,
	valueType datatype.DataType,
) error {
	mapValDatatype := column.CQLType
	if keyType == datatype.Varchar && valueDatatype == datatype.Timestamp {
		mapValDatatype = utilities.MapOfStrToBigInt // Cassandra stores timestamps as bigint
	} else if keyType == datatype.Timestamp && valueDatatype == datatype.Timestamp {
		mapValDatatype = utilities.MapOfTimeToBigInt // Cassandra stores timestamps as bigint
	}

	if keyType == datatype.Varchar {
		return processVarcharMap[V](valueDatatype, mapValDatatype, protocolV, val, column, colFamily, output)

	} else if keyType == datatype.Timestamp {
		return processTimestampMap[V](valueDatatype, mapValDatatype, protocolV, val, column, colFamily, output)
	} else {
		return fmt.Errorf("Invalid map type")
	}
}

// processVarcharMap handles map operations with varchar keys in prepared queries.
// Processes varchar map operations.
// Returns error if key type isn't varchar or value encoding fails.
func processVarcharMap[V any](valueDatatype, mapValDatatype datatype.DataType, protocolV primitive.ProtocolVersion, val *primitive.Value, column types.Column, colFamily string, output *ProcessPrepareCollectionsOutput) error {
	decodedValue, err := proxycore.DecodeType(mapValDatatype, protocolV, val.Contents)
	if err != nil {
		return fmt.Errorf("error decoding string to %s value: %w", column.CQLType, err)
	}
	decodedMap := make(map[string]V)
	pointerMap, ok := decodedValue.(map[*string]*V)
	if ok {
		for k, v := range pointerMap {
			decodedMap[*k] = *v
		}
		for k, v := range decodedMap {

			c := types.Column{
				Name:         k,
				ColumnFamily: colFamily,
				CQLType:      valueDatatype,
			}
			valueStr, err := primitivesToString(v)
			if err != nil {
				return fmt.Errorf("failed to convert value for key '%s': %w", k, err)
			}

			val, _ := formatValues(valueStr, valueDatatype, protocolV)
			output.NewValues = append(output.NewValues, val)
			output.NewColumns = append(output.NewColumns, c)
		}
		return nil
	} else {
		return fmt.Errorf("error decoding string to %s value: %w", column.CQLType, err)
	}

}

func processTimestampMap[V any](valueDatatype, mapValDatatype datatype.DataType, protocolV primitive.ProtocolVersion, val *primitive.Value, column types.Column, colFamily string, output *ProcessPrepareCollectionsOutput) error {
	decodedValue, err := collectiondecoder.DecodeCollection(mapValDatatype, protocolV, val.Contents)
	if err != nil {
		return fmt.Errorf("error decoding string to %s value: %w", column.CQLType, err)
	}
	decodedMap := make(map[int64]V)
	pointerMap, ok := decodedValue.(map[time.Time]V)
	if ok {
		for k, v := range pointerMap {
			key := k.UnixMilli()
			decodedMap[key] = v
		}
		for k, v := range decodedMap {

			c := types.Column{
				Name:         strconv.FormatInt(k, 10),
				ColumnFamily: colFamily,
				CQLType:      valueDatatype,
			}
			valueStr, err := primitivesToString(v)
			if err != nil {
				return fmt.Errorf("failed to convert value for key '%v': %w", k, err)
			}

			val, _ := formatValues(valueStr, valueDatatype, protocolV)
			output.NewValues = append(output.NewValues, val)
			output.NewColumns = append(output.NewColumns, c)
		}
		return nil
	} else {
		return fmt.Errorf("error decoding string to %v value: %w", column.CQLType, err)
	}

}

// processDeleteOperationForMapAndSet handles delete operations for maps and sets in prepared queries.
// Processes deletion of map entries and set elements with type validation.
// Returns error if value type doesn't match collection element type or deletion fails.
func processDeleteOperationForMapAndSet(
	column types.Column,
	meta *ComplexOperation,
	input ProcessPrepareCollectionsInput,
	val *primitive.Value,
	output *ProcessPrepareCollectionsOutput,
	expectedDt datatype.DataType,
) error {
	var err error

	decodedValue, err := collectiondecoder.DecodeCollection(expectedDt, input.ProtocolV, val.Contents)
	if err != nil {
		return fmt.Errorf("error decoding string to %s value: %w", column.CQLType, err)
	}
	setValue, err := convertToInterfaceSlice(decodedValue)
	if err != nil {
		return err
	}
	for _, v := range setValue {
		setVal, err := primitivesToString(v)
		if err != nil {
			return fmt.Errorf("failed to convert value: %w", err)
		}
		output.DelColumns = append(output.DelColumns, types.Column{
			Name:         setVal,
			ColumnFamily: column.Name,
		})
	}
	return nil
}

// convertToInterfaceSlice converts a value to a slice of interfaces.
// Handles type conversion for collection operations with validation.
// Returns error if value cannot be converted to a slice.
func convertToInterfaceSlice(decodedValue interface{}) ([]interface{}, error) {
	val := reflect.ValueOf(decodedValue)
	if val.Kind() != reflect.Slice {
		return nil, fmt.Errorf("expected a slice type, got %T", decodedValue)
	}

	setValue := make([]interface{}, val.Len())
	for i := 0; i < val.Len(); i++ {
		setValue[i] = val.Index(i).Interface()
	}
	return setValue, nil
}

// processMapKeyAppend handles map key updates operations in prepared queries.
// Processes updation at index and simple assignment.
// Returns error if key/value types don't match map types or encoding fails.
func processMapKeyAppend(
	column types.Column,
	meta *ComplexOperation,
	input ProcessPrepareCollectionsInput,
	val *primitive.Value,
	output *ProcessPrepareCollectionsOutput,
	valueDatatype datatype.DataType,
	keyType datatype.DataType,
) error {
	expectedDT := meta.ExpectedDatatype
	//if expected datatype is timestamp, convert it to bigint
	if meta.ExpectedDatatype == datatype.Timestamp {
		expectedDT = datatype.Bigint
	}
	decodedValue, err := proxycore.DecodeType(expectedDT, input.ProtocolV, val.Contents)

	if err != nil {
		return fmt.Errorf("error decoding string to %s value: %w", column.CQLType, err)
	}
	mpKey := meta.mapKey

	//if key is timestamp, convert it to unix milliseconds
	if keyType == datatype.Timestamp {
		mp, err := parseTimestamp(mpKey.(string))
		if err != nil {
			return fmt.Errorf("error while typecasting to timestamp %v, -> %s", mpKey.(string), err)
		}
		mpKey = mp.UnixMilli()
	}

	setKey, err := primitivesToString(mpKey)
	if err != nil {
		return fmt.Errorf("failed to convert/read key: %w", err)
	}
	setValue, err := primitivesToString(decodedValue)
	if err != nil {
		return fmt.Errorf("failed to convert value for key '%s': %w", setKey, err)
	}

	c := types.Column{
		Name:         setKey,
		ColumnFamily: column.Name,
		CQLType:      valueDatatype,
	}
	formattedValue, err := formatValues(setValue, valueDatatype, input.ProtocolV)
	if err != nil {
		return fmt.Errorf("failed to format value: %w", err)
	}
	output.NewValues = append(output.NewValues, formattedValue)
	output.NewColumns = append(output.NewColumns, c)

	return nil
}

// handlePrepareSetOperation processes set operations in prepared queries.
// Manages set operations with protocol version-specific value encoding.
// Returns error if value type doesn't match set element type or encoding fails.
func handlePrepareSetOperation(val *primitive.Value, column types.Column, colFamily string, input ProcessPrepareCollectionsInput, output *ProcessPrepareCollectionsOutput) error {

	if meta, ok := input.ComplexMeta[column.Name]; ok {
		var complexUpdateMeta *ComplexOperation = meta

		//handling Delete for specific key e.g map=map-['key1','key2']
		if complexUpdateMeta.Delete {
			setType, ok := column.CQLType.(datatype.SetType)
			if !ok {
				return fmt.Errorf("failed to assert map type for %v", column.CQLType)
			}
			if setType.GetElementType() == datatype.Timestamp {
				setType = utilities.SetOfBigInt
			}
			return processDeleteOperationForMapAndSet(column, complexUpdateMeta, input, val, output, setType)
		}
		// handling set operations for append.
		return handleSetProcessing(val, column, colFamily, input.ProtocolV, output)
	} else {
		// case of when new value is being set for collection type {e.g collection=newCollection}
		output.DelColumnFamily = append(output.DelColumnFamily, column.Name)
		return handleSetProcessing(val, column, colFamily, input.ProtocolV, output)
	}
}

// handleSetProcessing processes set values for prepared queries.
// Handles set operations for append, delete, and update at index and simple assignment.
// Returns error if value type doesn't match set element type or encoding fails.
func handleSetProcessing(
	val *primitive.Value,
	column types.Column,
	colFamily string,
	protocolV primitive.ProtocolVersion,
	output *ProcessPrepareCollectionsOutput,
) error {
	setType, ok := column.CQLType.(datatype.SetType)
	if !ok {
		return fmt.Errorf("failed to assert set type for %v", column.CQLType)
	}
	valueDataType := setType.GetElementType()
	if valueDataType == datatype.Timestamp {
		valueDataType = datatype.Bigint
		setType = utilities.SetOfBigInt
	}
	switch valueDataType {
	case datatype.Varchar:
		return processSet[string](val, column, colFamily, protocolV, setType, valueDataType, output)
	case datatype.Int:
		return processSet[int32](val, column, colFamily, protocolV, setType, valueDataType, output)
	case datatype.Bigint, datatype.Timestamp:
		return processSet[int64](val, column, colFamily, protocolV, setType, valueDataType, output)
	case datatype.Float:
		return processSet[float32](val, column, colFamily, protocolV, setType, valueDataType, output)
	case datatype.Double:
		return processSet[float64](val, column, colFamily, protocolV, setType, valueDataType, output)
	case datatype.Boolean:
		return processSet[bool](val, column, colFamily, protocolV, setType, valueDataType, output)
	default:
		return fmt.Errorf("unexpected set element type: %T", column.CQLType)
	}
}

// processSet handles generic set processing operations in prepared queries.
// Manages set operations with type-specific handling and protocol version encoding.
// Returns error if value type doesn't match set element type or encoding fails.
func processSet[V any](
	val *primitive.Value,
	column types.Column,
	colFamily string,
	protocolV primitive.ProtocolVersion,
	setType datatype.DataType,
	valueDataType datatype.DataType,
	output *ProcessPrepareCollectionsOutput,

) error {

	decodedValue, err := collectiondecoder.DecodeCollection(setType, protocolV, val.Contents)
	if err != nil {
		return fmt.Errorf("error decoding string to %s value: %w", column.CQLType, err)
	}

	setValues, ok := decodedValue.([]V)
	if !ok {
		return fmt.Errorf("unexpected set element type: %T", decodedValue)
	}

	// Common processing logic for all types
	for _, elem := range setValues {
		valueStr, err := primitivesToString(elem)
		if err != nil {
			return fmt.Errorf("failed to convert element to string: %w", err)
		}
		valInInterface, err := DataConversionInInsertionIfRequired(valueStr, protocolV, valueDataType, "string")
		if err != nil {
			return fmt.Errorf("error while encoding set<%s> value: %w", valueDataType, err)
		}
		valueStr = valInInterface.(string)
		c := types.Column{
			Name:         valueStr,
			ColumnFamily: colFamily,
			CQLType:      valueDataType,
		}
		// The value parameter is intentionally empty because the column identifier has the value
		val, _ := formatValues("", datatype.Varchar, protocolV)
		output.NewValues = append(output.NewValues, val)
		output.NewColumns = append(output.NewColumns, c)
	}
	return nil
}

// buildWhereClause(): takes a slice of Clause structs and returns a string representing the WHERE clause of a bigtable SQL query.
// It iterates over the clauses and constructs the WHERE clause by combining the column name, operator, and value of each clause.
// If the operator is "IN", the value is wrapped with the UNNEST function.
// The constructed WHERE clause is returned as a string.
func buildWhereClause(clauses []types.Clause, t *Translator, tableName string, keySpace string) (string, error) {
	whereClause := ""
	columnFamily := t.SchemaMappingConfig.SystemColumnFamily
	for _, val := range clauses {
		column := "`" + val.Column + "`"
		value := val.Value
		if colMeta, ok := t.SchemaMappingConfig.TablesMetaData[keySpace][tableName][val.Column]; ok {
			// Check if the column is a primitive type and prepend the column family
			if !colMeta.IsCollection {
				var castErr error
				column, castErr = castColumns(colMeta, columnFamily, t.SchemaMappingConfig.CounterColumnFamily)
				if castErr != nil {
					return "", castErr
				}
			}
		}
		if whereClause != "" && val.Operator != constants.BETWEEN_AND {
			whereClause += " AND "
		}
		if val.Operator == constants.BETWEEN {
			whereClause += fmt.Sprintf("%s BETWEEN %s", column, val.Value)
		} else if val.Operator == constants.BETWEEN_AND {
			whereClause += fmt.Sprintf(" AND %s", val.Value)
		} else if val.Operator == constants.IN {
			whereClause += fmt.Sprintf("%s IN UNNEST(%s)", column, val.Value)
		} else if val.Operator == constants.MAP_CONTAINS_KEY {
			whereClause += fmt.Sprintf("MAP_CONTAINS_KEY(%s, %s)", column, value)
		} else if val.Operator == constants.ARRAY_INCLUDES {
			whereClause += fmt.Sprintf("ARRAY_INCLUDES(MAP_VALUES(%s), %s)", column, value)
		} else {
			whereClause += fmt.Sprintf("%s %s %s", column, val.Operator, value)
		}
	}

	if whereClause != "" {
		whereClause = " WHERE " + whereClause
	}
	return whereClause, nil
}

// castColumns handles column type casting in queries.
// Manages type conversion for column values with validation.
// Returns error if column type is invalid or conversion fails.
func castColumns(colMeta *types.Column, columnFamily string, counterColumnFamily string) (string, error) {
	var nc string
	switch colMeta.CQLType {
	case datatype.Int:
		if colMeta.IsPrimaryKey {
			nc = colMeta.ColumnName
		} else {
			nc = fmt.Sprintf("TO_INT64(%s['%s'])", columnFamily, colMeta.ColumnName)
		}
	case datatype.Bigint:
		if colMeta.IsPrimaryKey {
			nc = colMeta.ColumnName
		} else {
			nc = fmt.Sprintf("TO_INT64(%s['%s'])", columnFamily, colMeta.ColumnName)
		}
	case datatype.Float:
		nc = fmt.Sprintf("TO_FLOAT32(%s['%s'])", columnFamily, colMeta.ColumnName)
	case datatype.Double:
		nc = fmt.Sprintf("TO_FLOAT64(%s['%s'])", columnFamily, colMeta.ColumnName)
	case datatype.Boolean:
		nc = fmt.Sprintf("TO_INT64(%s['%s'])", columnFamily, colMeta.ColumnName)
	case datatype.Timestamp:
		nc = fmt.Sprintf("TO_TIME(%s['%s'])", columnFamily, colMeta.ColumnName)
	case datatype.Counter:
		nc = fmt.Sprintf("%s['%s']", counterColumnFamily, colMeta.ColumnName)
	case datatype.Blob:
		nc = fmt.Sprintf("TO_BLOB(%s['%s'])", columnFamily, colMeta.ColumnName)
	case datatype.Varchar:
		if colMeta.IsPrimaryKey {
			nc = colMeta.ColumnName
		} else {
			nc = fmt.Sprintf("%s['%s']", columnFamily, colMeta.ColumnName)
		}
	default:
		return "", fmt.Errorf("unsupported CQL type: %s", colMeta.CQLType)
	}
	return nc, nil
}

// parseWhereByClause parses the WHERE clause from a CQL query.
// Extracts and processes WHERE conditions with type validation.
// Returns error if clause parsing fails or invalid conditions are found.
func parseWhereByClause(input cql.IWhereSpecContext, tableName string, schemaMapping *schemaMapping.SchemaMappingConfig, keyspace string) (*QueryClauses, error) {
	if input == nil {
		return nil, errors.New("no input parameters found for clauses")
	}

	elements := input.RelationElements().AllRelationElement()
	if elements == nil {
		return nil, errors.New("no input parameters found for clauses")
	}

	if len(elements) == 0 {
		return &QueryClauses{}, nil
	}
	var clauses []types.Clause
	var response QueryClauses
	var paramKeys []string

	params := make(map[string]interface{})

	placeholderCount := 0
	for _, val := range elements {
		if val == nil {
			return nil, errors.New("could not parse column object")
		}
		s := strconv.Itoa(placeholderCount + 1)
		placeholderCount++
		placeholder := "value" + s
		secondPlaceholder := "value" + strconv.Itoa(placeholderCount+1) // It will be used for second value of BETWEEN clause
		operator := ""
		paramKeys = append(paramKeys, placeholder)

		isInOperator := false
		isBetweenOperator := false
		colName := ""
		value := ""
		if val.OPERATOR_EQ() != nil {
			operator = val.OPERATOR_EQ().GetText()
		} else if val.OPERATOR_GT() != nil {
			operator = val.OPERATOR_GT().GetSymbol().GetText()
		} else if val.OPERATOR_LT() != nil {
			operator = val.OPERATOR_LT().GetSymbol().GetText()
		} else if val.OPERATOR_GTE() != nil {
			operator = val.OPERATOR_GTE().GetSymbol().GetText()
		} else if val.OPERATOR_LTE() != nil {
			operator = val.OPERATOR_LTE().GetSymbol().GetText()
		} else if val.KwIn() != nil {
			operator = "IN"
			isInOperator = true
		} else if val.RelalationContains() != nil {
			operator = "CONTAINS"
			relContains := val.RelalationContains()
			colName = relContains.OBJECT_NAME().GetText()
			value = relContains.Constant().GetText()
		} else if val.RelalationContainsKey() != nil {
			operator = "CONTAINS KEY"
			relContainsKey := val.RelalationContainsKey()
			colName = relContainsKey.OBJECT_NAME().GetText()
			value = relContainsKey.Constant().GetText()
		} else if val.KwLike() != nil {
			operator = "LIKE"
		} else if val.KwBetween() != nil {
			operator = constants.BETWEEN
			isBetweenOperator = true
		} else {
			return nil, errors.New("no supported operator found")
		}

		if colName == "" {
			colObj := val.OBJECT_NAME(0)
			if colObj == nil {
				return nil, errors.New("could not parse column object")
			}
			colName = colObj.GetText()
			if colName == "" {
				return nil, errors.New("could not parse column name")
			}
		}
		colName = strings.ReplaceAll(colName, literalPlaceholder, "")
		columnType, err := schemaMapping.GetColumnType(keyspace, tableName, colName)
		if err != nil {
			return nil, err
		}
		if columnType != nil && columnType.CQLType != nil {
			if operator == constants.CONTAINS || operator == constants.CONTAINS_KEY {
				if value == "" {
					return nil, errors.New("could not parse value from query for one of the clauses")
				}
				value = strings.ReplaceAll(value, "'", "")
				typecode := columnType.CQLType.GetDataTypeCode()
				if value != questionMark {
					if operator == constants.CONTAINS {
						if typecode == primitive.DataTypeCodeList { // list
							operator = constants.ARRAY_INCLUDES
						} else if typecode == primitive.DataTypeCodeSet { // set
							operator = constants.MAP_CONTAINS_KEY
						} else {
							return nil, errors.New("CONTAINS are only supported for set and list")
						}
					} else {
						if typecode == primitive.DataTypeCodeMap { // map
							operator = constants.MAP_CONTAINS_KEY
						} else {
							return nil, errors.New("CONTAINS KEY are only supported for map")
						}
					}
					params[placeholder] = []byte(value)
				} else {
					if operator == "CONTAINS" {
						if typecode == primitive.DataTypeCodeSet { // set
							setType, _ := columnType.CQLType.(datatype.SetType)
							elementType := setType.GetElementType()
							params[placeholder] = cqlTypeToEmptyPrimitive(elementType, false)
							operator = constants.MAP_CONTAINS_KEY
						} else if typecode == primitive.DataTypeCodeList { // list
							listType, _ := columnType.CQLType.(datatype.ListType)
							elementType := listType.GetElementType()
							params[placeholder] = cqlTypeToEmptyPrimitive(elementType, false)
							operator = constants.ARRAY_INCLUDES
						} else {
							return nil, errors.New("CONTAINS are only supported for set and list")
						}
					} else {
						// If it's not contains, then it's a CONTAINS KEY operator
						if typecode == primitive.DataTypeCodeMap { // map
							// keyType := mapType.GetKeyType()
							params[placeholder] = cqlTypeToEmptyPrimitive(datatype.Boolean, false)
							operator = constants.MAP_CONTAINS_KEY
						} else {
							return nil, errors.New("CONTAINS KEY are only supported for map")
						}
					}
				}
			} else if isBetweenOperator {
				value := val.Constant(0).GetText()       // As per the testing val.Constant(0) values never be nil
				secondValue := val.Constant(1).GetText() // Same here, As per the testing val.Constant(1) values never be nil
				if value == "" || secondValue == "" || value == "<EOF>" || secondValue == "<EOF>" {
					return nil, errors.New("could not parse value from query for one of the clauses")
				}
				value = strings.ReplaceAll(value, "'", "")
				secondValue = strings.ReplaceAll(secondValue, "'", "")
				if value != questionMark {
					val, err := stringToPrimitives(value, columnType.CQLType)
					if err != nil {
						return nil, err
					}
					secondVal, err := stringToPrimitives(secondValue, columnType.CQLType)
					if err != nil {
						return nil, err
					}
					params[placeholder] = val
					params[secondPlaceholder] = secondVal
				} else {
					params[placeholder] = cqlTypeToEmptyPrimitive(columnType.CQLType, columnType.IsPrimaryKey)
					params[secondPlaceholder] = params[placeholder] // CQLType will be same for both the values in the between clause
				}
				placeholderCount++ // we need to increase the placeholder count to get the next placeholder as BETWEEN clause has two values
				paramKeys = append(paramKeys, secondPlaceholder)
			} else if !isInOperator && !isBetweenOperator {
				valConst := val.Constant(0)
				if valConst == nil {
					return nil, errors.New("could not parse value from query for one of the clauses")
				}
				value := val.Constant(0).GetText()
				if value == "" {
					return nil, errors.New("could not parse value from query for one of the clauses")
				}
				value = strings.ReplaceAll(value, "'", "")

				if value != questionMark {

					val, err := stringToPrimitives(value, columnType.CQLType)
					if err != nil {
						return nil, err
					}

					params[placeholder] = val
				} else {
					params[placeholder] = cqlTypeToEmptyPrimitive(columnType.CQLType, columnType.IsPrimaryKey)
				}
			} else {
				lower := strings.ToLower(val.GetText())
				if !strings.Contains(lower, "?") {
					valueFn := val.FunctionArgs()
					if valueFn == nil {
						return nil, errors.New("could not parse Function arguments")
					}
					value := valueFn.AllConstant()
					if value == nil {
						return nil, errors.New("could not parse all values inside IN operator")
					}
					switch columnType.CQLType {
					case datatype.Int:
						var allValues []int
						for _, inVal := range value {
							if inVal == nil {
								return nil, errors.New("could not parse value")
							}
							valueTxt := inVal.GetText()
							valueTxt = strings.ReplaceAll(valueTxt, "'", "")
							i, err := strconv.Atoi(valueTxt)
							if err != nil {
								return nil, err
							}
							allValues = append(allValues, i)
						}
						params[placeholder] = allValues
					case datatype.Bigint:
						var allValues []int64
						for _, inVal := range value {
							if inVal == nil {
								return nil, errors.New("could not parse value")
							}
							valueTxt := inVal.GetText()
							valueTxt = strings.ReplaceAll(valueTxt, "'", "")
							i, err := strconv.ParseInt(valueTxt, 10, 64)
							if err != nil {
								return nil, err
							}
							allValues = append(allValues, i)
						}
						params[placeholder] = allValues
					case datatype.Float:
						var allValues []float32
						for _, inVal := range value {
							if inVal == nil {
								return nil, errors.New("could not parse value")
							}
							valueTxt := inVal.GetText()
							valueTxt = strings.ReplaceAll(valueTxt, "'", "")
							i, err := strconv.ParseFloat(valueTxt, 32)

							if err != nil {
								return nil, err
							}
							allValues = append(allValues, float32(i))
						}
						params[placeholder] = allValues
					case datatype.Double:
						var allValues []float64
						for _, inVal := range value {
							if inVal == nil {
								return nil, errors.New("could not parse value")
							}
							valueTxt := inVal.GetText()
							valueTxt = strings.ReplaceAll(valueTxt, "'", "")
							i, err := strconv.ParseFloat(valueTxt, 64)
							if err != nil {
								return nil, err
							}
							allValues = append(allValues, i)
						}
						params[placeholder] = allValues
					case datatype.Boolean:
						var allValues []bool
						for _, inVal := range value {
							if inVal == nil {
								return nil, errors.New("could not parse value")
							}
							valueTxt := inVal.GetText()
							valueTxt = strings.ReplaceAll(valueTxt, "'", "")
							val := (valueTxt == "true")
							allValues = append(allValues, val)
						}
						params[placeholder] = allValues
					case datatype.Blob:
					case datatype.Varchar, datatype.Timestamp:
						var allValues []string
						for _, inVal := range value {
							if inVal == nil {
								return nil, errors.New("could not parse value")
							}
							valueTxt := inVal.GetText()
							valueTxt = strings.ReplaceAll(valueTxt, "'", "")
							allValues = append(allValues, valueTxt)
						}
						params[placeholder] = allValues

					default:
						err := errors.New("no correct Datatype found for column")
						return nil, err
					}
				} else {
					// Create an empty array placeholder value based on the CQL type
					switch columnType.CQLType {
					case datatype.Int:
						params[placeholder] = make([]int, 0)
					case datatype.Bigint:
						params[placeholder] = make([]int64, 0)
					case datatype.Float:
						params[placeholder] = make([]float32, 0)
					case datatype.Double:
						params[placeholder] = make([]float64, 0)
					case datatype.Boolean:
						params[placeholder] = make([]bool, 0)
					case datatype.Blob:
						params[placeholder] = make([][]byte, 0)
					case datatype.Varchar, datatype.Timestamp:
						params[placeholder] = make([]string, 0)
					default:
						return nil, fmt.Errorf("unsupported array CQL type: %s", columnType.CQLType)
					}
				}
			}
			clause := &types.Clause{
				Column:       colName,
				Operator:     operator,
				Value:        "@" + placeholder,
				IsPrimaryKey: columnType.IsPrimaryKey,
			}

			clauses = append(clauses, *clause)
			if isBetweenOperator {
				clause = &types.Clause{
					Column:       colName,
					Operator:     constants.BETWEEN_AND,
					Value:        "@" + secondPlaceholder,
					IsPrimaryKey: columnType.IsPrimaryKey,
				}
				clauses = append(clauses, *clause)
			}
		}
	}
	response.Clauses = clauses
	response.Params = params
	response.ParamKeys = paramKeys
	return &response, nil
}

// GetColumnFamily retrieves the column family for a given column.
// Returns the column family name from the schema mapping with validation.
// Returns default column family for primitive types and column name for collections.
func (t *Translator) GetColumnFamily(keyspace, tableName, columnName string) string {
	if colType, err := t.SchemaMappingConfig.GetColumnType(keyspace, tableName, columnName); err == nil {
		if colType.IsCollection {
			return columnName
		}
	}
	return t.SchemaMappingConfig.SystemColumnFamily
}

// getTimestampValue extracts timestamp value from a query.
// Parses timestamp specifications in USING TIMESTAMP clauses with validation.
// Returns error if timestamp format is invalid or parsing fails.
func getTimestampValue(spec cql.IUsingTtlTimestampContext) (string, error) {
	if spec == nil {
		return "", errors.New("invalid input")
	}

	spec.KwUsing()
	tsSpec := spec.Timestamp()

	if tsSpec == nil {
		return questionMark, nil
	}
	tsSpec.KwTimestamp()
	valLiteral := tsSpec.DecimalLiteral()
	var resp string

	if valLiteral == nil {
		return "", errors.New("no value found for Timestamp")
	}

	resp = valLiteral.GetText()
	resp = strings.ReplaceAll(resp, "'", "")
	return resp, nil
}

// GetTimestampInfo retrieves timestamp information from an insert query.
// Extracts and processes timestamp values with validation and conversion.
// Returns error if timestamp format is invalid or conversion fails.
func GetTimestampInfo(queryStr string, insertObj cql.IInsertContext, index int32) (TimestampInfo, error) {
	var timestampInfo TimestampInfo
	lowerQuery := strings.ToLower(queryStr)
	hasUsingTimestamp := hasUsingTimestamp(lowerQuery)
	if hasUsingTimestamp {
		tsSpec := insertObj.UsingTtlTimestamp()
		if tsSpec == nil {
			return timestampInfo, errors.New("error parsing timestamp spec")
		}
		tsValue, err := getTimestampValue(tsSpec)
		if err != nil {
			return timestampInfo, err
		}
		return convertToBigtableTimestamp(tsValue, index)
	}
	return timestampInfo, nil
}

// convertToBigtableTimestamp converts a timestamp to Bigtable format.
// Handles timestamp conversion and formatting with validation.
// Returns error if timestamp format is invalid or conversion fails.
func convertToBigtableTimestamp(tsValue string, index int32) (TimestampInfo, error) {
	var timestampInfo TimestampInfo
	unixTime := int64(0)
	var err error
	if tsValue != "" && !strings.Contains(tsValue, questionMark) {
		unixTime, err = strconv.ParseInt(tsValue, 10, 64)
		if err != nil {
			return timestampInfo, err
		}
	}
	t := time.Unix(unixTime, 0)
	switch len(tsValue) {
	case 13: // Milliseconds
		t = time.Unix(0, unixTime*int64(time.Millisecond))
	case 16: // Microseconds
		t = time.Unix(0, unixTime*int64(time.Microsecond))
	}
	microsec := t.UnixMicro()
	timestampInfo.Timestamp = bigtable.Timestamp(microsec)
	timestampInfo.HasUsingTimestamp = true
	timestampInfo.Index = index
	return timestampInfo, nil
}

// GetTimestampInfoForRawDelete retrieves timestamp information for delete operations.
// Extracts timestamp values from raw delete queries with validation.
// Returns error if timestamp format is invalid or extraction fails.
func GetTimestampInfoForRawDelete(queryStr string, deleteObj cql.IDelete_Context) (TimestampInfo, error) {
	var timestampInfo TimestampInfo
	lowerQuery := strings.ToLower(queryStr)
	hasUsingTimestamp := hasUsingTimestamp(lowerQuery)
	timestampInfo.HasUsingTimestamp = false
	if hasUsingTimestamp {
		tsSpec := deleteObj.UsingTimestampSpec()
		if tsSpec == nil {
			return timestampInfo, ErrParsingTs
		}

		timestampSpec := tsSpec.Timestamp()
		if timestampSpec == nil {
			return timestampInfo, ErrParsingDelObj
		}

		if tsSpec.KwUsing() != nil {
			tsSpec.KwUsing()
		}
		valLiteral := timestampSpec.DecimalLiteral()
		if valLiteral == nil {
			return timestampInfo, ErrTsNoValue
		}
		tsValue := valLiteral.GetText()
		return convertToBigtableTimestamp(tsValue, 0)
	}
	return timestampInfo, nil
}

// GetTimestampInfoByUpdate retrieves timestamp information for update operations.
// Extracts timestamp values from update queries with validation.
// Returns error if timestamp format is invalid or extraction fails.
func GetTimestampInfoByUpdate(queryStr string, updateObj cql.IUpdateContext) (TimestampInfo, error) {
	var timestampInfo TimestampInfo
	ttlTimestampObj := updateObj.UsingTtlTimestamp()
	lowerQuery := strings.ToLower(queryStr)
	hasUsingTimestamp := hasUsingTimestamp(lowerQuery)
	if ttlTimestampObj != nil && hasUsingTimestamp {
		tsValue, err := getTimestampValue(updateObj.UsingTtlTimestamp())
		if err != nil {
			return TimestampInfo{}, err
		}
		timestampInfo, err = convertToBigtableTimestamp(tsValue, 0)
		if err != nil {
			return TimestampInfo{}, err
		}
	}
	return timestampInfo, nil
}

// getTimestampInfoForPrepareQuery retrieves timestamp information from prepared queries.
// Extracts timestamp values from prepared statement parameters with validation.
// Returns error if timestamp format is invalid or extraction fails.
func getTimestampInfoForPrepareQuery(values []*primitive.Value, index int32, offset int32) (TimestampInfo, error) {
	var timestampInfo TimestampInfo
	timestampInfo.HasUsingTimestamp = true
	if values[index] == nil {
		err := fmt.Errorf("error processing timestamp column in prepare insert")
		return timestampInfo, err
	}
	vBytes := values[index].Contents
	decode, err := proxycore.DecodeType(datatype.Bigint, 4, vBytes)
	if err != nil {
		fmt.Println("error while decoding timestamp column in prepare insert:", err)
		return timestampInfo, err
	}
	timestamp := decode.(int64)
	var t time.Time
	if timestamp < 10000000000 { // Assuming it's in seconds
		t = time.Unix(timestamp, 0)
	} else if timestamp < 10000000000000 { // Assuming it's in milliseconds
		t = time.Unix(0, timestamp*int64(time.Millisecond))
	} else { // As, we are supporting till microseconds, we are not converting it to microseconds
		t = time.Unix(0, timestamp*int64(time.Microsecond))
	}
	microsec := t.UnixMicro()
	timestampInfo.Timestamp = bigtable.Timestamp(microsec)
	return timestampInfo, nil
}

// ProcessTimestamp processes timestamp values for insert operations.
// Handles timestamp processing and validation with type conversion.
// Returns error if timestamp format is invalid or processing fails.
func ProcessTimestamp(st *InsertQueryMapping, values []*primitive.Value) (TimestampInfo, error) {
	var timestampInfo TimestampInfo
	timestampInfo.HasUsingTimestamp = false
	if st.TimestampInfo.HasUsingTimestamp {
		return getTimestampInfoForPrepareQuery(values, st.TimestampInfo.Index, 0)
	}
	return timestampInfo, nil
}

// ProcessTimestampByUpdate processes timestamp values for update operations.
// Handles timestamp processing and validation with type conversion.
// Returns error if timestamp format is invalid or processing fails.
func ProcessTimestampByUpdate(st *UpdateQueryMapping, values []*primitive.Value) (TimestampInfo, []*primitive.Value, []*message.ColumnMetadata, error) {
	var timestampInfo TimestampInfo
	var err error
	timestampInfo.HasUsingTimestamp = false
	variableMetadata := st.VariableMetadata
	if st.TimestampInfo.HasUsingTimestamp {
		timestampInfo, err = getTimestampInfoForPrepareQuery(values, st.TimestampInfo.Index, 0)
		if err != nil {
			return timestampInfo, values, variableMetadata, err
		}
		values = values[1:]
		variableMetadata = st.VariableMetadata[1:]
	}
	return timestampInfo, values, variableMetadata, nil
}

// ProcessTimestampByDelete processes timestamp values for delete operations.
// Handles timestamp processing and validation with type conversion.
// Returns error if timestamp format is invalid or processing fails.
func ProcessTimestampByDelete(st *DeleteQueryMapping, values []*primitive.Value) (TimestampInfo, []*primitive.Value, error) {
	var timestampInfo TimestampInfo
	timestampInfo.HasUsingTimestamp = false
	var err error
	if st.TimestampInfo.HasUsingTimestamp {
		timestampInfo, err = getTimestampInfoForPrepareQuery(values, st.TimestampInfo.Index, 1)
		if err != nil {
			return timestampInfo, values, err
		}
		values = values[1:]
	}
	return timestampInfo, values, nil
}

// GetAllColumns retrieves all columns for a given table.
// Returns the list of column names and their types with validation.
// Returns error if table doesn't exist or schema mapping fails.
func (t *Translator) GetAllColumns(tableName string, keySpace string) ([]string, string, error) {
	tableData := t.SchemaMappingConfig.TablesMetaData[keySpace][tableName]
	if tableData == nil {
		return nil, "", errors.New("schema mapping not found")
	}
	var columns []string
	for _, value := range tableData {
		if !value.IsCollection {
			columns = append(columns, value.ColumnName)
		}
	}
	return columns, t.SchemaMappingConfig.SystemColumnFamily, nil
}

// ValidateRequiredPrimaryKeys validates primary key requirements.
// Checks if all required primary keys are present in the query with validation.
// Returns false if any required key is missing or invalid.
func ValidateRequiredPrimaryKeys(requiredKey []string, actualKey []string) bool {
	// Check if the length of slices is the same
	if len(requiredKey) != len(actualKey) {
		return false
	}

	// make a copy of the slices so sort doesn't mutate the original slices which other code probably relies on correct ordering of. Kinda lazy solution but these slices should be small
	requiredKeyTmp := make([]string, len(requiredKey))
	copy(requiredKeyTmp, requiredKey)
	actualKeyTmp := make([]string, len(actualKey))
	copy(actualKeyTmp, actualKey)

	// Sort both slices
	sort.Strings(requiredKeyTmp)
	sort.Strings(actualKeyTmp)

	// Compare the sorted slices element by element
	for i := range requiredKeyTmp {
		if requiredKeyTmp[i] != actualKeyTmp[i] {
			return false
		}
	}

	return true
}

// encodeTimestamp encodes a timestamp value into bytes.
// Converts timestamp values to byte representation with validation.
// Returns error if timestamp format is invalid or encoding fails.
func encodeTimestamp(millis int64, nanos int32) []byte {
	buf := make([]byte, 12) // 8 bytes for millis + 4 bytes for nanos
	binary.BigEndian.PutUint64(buf[0:8], uint64(millis))
	binary.BigEndian.PutUint32(buf[8:12], uint32(nanos))
	return buf
}

// getEncodedTimestamp generates encoded timestamp bytes.
// Creates timestamp byte representation for specific positions with validation.
// Returns error if position is invalid or encoding fails.
func getEncodedTimestamp(index int, totalLength int, prepend bool) []byte {
	now := time.Now().UnixMilli()
	if prepend {
		now = referenceTime - (now - referenceTime)
	}

	nanos := maxNanos - int32(totalLength) + int32(index)
	return encodeTimestamp(now, nanos)
}

// ExtractWritetimeValue extracts writetime value from a string.
// Parses and extracts writetime specifications with validation.
// Returns error if format is invalid or extraction fails.
func ExtractWritetimeValue(s string) (string, bool) {
	s = strings.ToLower(s)
	// Check if the input starts with "writetime(" and ends with ")"
	if strings.HasPrefix(s, "writetime(") && strings.HasSuffix(s, ")") {
		// Extract the content between "writetime(" and ")"
		value := s[len("writetime(") : len(s)-1]
		return value, true
	}
	// Return empty string and false if the format is incorrect
	return "", false
}

// convertAllValuesToRowKeyType converts values to row key types.
// Handles type conversion for row key values with validation.
// Returns error if value type is invalid or conversion fails.
func convertAllValuesToRowKeyType(primaryKeys []types.Column, values map[string]interface{}) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	for _, pmk := range primaryKeys {
		if !pmk.IsPrimaryKey {
			continue
		}

		value, exists := values[pmk.ColumnName]
		if !exists {
			return nil, fmt.Errorf("missing primary key `%s`", pmk.ColumnName)
		}
		switch pmk.CQLType {
		case datatype.Int:
			switch v := value.(type) {
			// bigtable row keys don't support int32 so convert all int32 values to int64
			case int:
				result[pmk.ColumnName] = int64(v)
			case int32:
				result[pmk.ColumnName] = int64(v)
			case int64:
				result[pmk.ColumnName] = v
			case string:
				i, err := strconv.Atoi(v)
				if err != nil {
					return nil, fmt.Errorf("failed to convert Int value %s for key %s", value.(string), pmk.ColumnName)
				}
				result[pmk.ColumnName] = int64(i)
			default:
				return nil, fmt.Errorf("failed to convert %T to Int for key %s", value, pmk.ColumnName)
			}
		case datatype.Bigint:
			switch v := value.(type) {
			case int:
				result[pmk.ColumnName] = int64(v)
			case int32:
				result[pmk.ColumnName] = int64(v)
			case int64:
				result[pmk.ColumnName] = value
			case string:
				i, err := strconv.ParseInt(v, 10, 0)
				if err != nil {
					return nil, fmt.Errorf("failed to convert BigInt value %s for key %s", value.(string), pmk.ColumnName)
				}
				result[pmk.ColumnName] = i
			default:
				return nil, fmt.Errorf("failed to convert %T to BigInt for key %s", value, pmk.ColumnName)
			}
		case datatype.Varchar:
			switch v := value.(type) {
			case string:
				// todo move this validation to all columns not just keys
				if !utf8.Valid([]byte(v)) {
					return nil, fmt.Errorf("invalid utf8 value provided for varchar row key field %s", pmk.ColumnName)
				}
				result[pmk.ColumnName] = v
			default:
				return nil, fmt.Errorf("failed to convert %T to BigInt for key %s", value, pmk.ColumnName)
			}
		case datatype.Blob:
			switch v := value.(type) {
			case string:
				result[pmk.ColumnName] = v
			default:
				return nil, fmt.Errorf("failed to convert %T to BigInt for key %s", value, pmk.ColumnName)
			}
		default:
			return nil, fmt.Errorf("unsupported primary key type %s for key %s", pmk.CQLType.String(), pmk.ColumnName)
		}
	}
	return result, nil
}

var kOrderedCodeEmptyField = []byte("\x00\x00")
var kOrderedCodeDelimiter = []byte("\x00\x01")

// createOrderedCodeKey creates an ordered row key.
// Generates a byte-encoded row key from primary key values with validation.
// Returns error if key type is invalid or encoding fails.
func createOrderedCodeKey(primaryKeys []types.Column, values map[string]interface{}, encodeIntValuesWithBigEndian bool) ([]byte, error) {
	fixedValues, err := convertAllValuesToRowKeyType(primaryKeys, values)
	if err != nil {
		return nil, err
	}

	var result []byte
	var trailingEmptyFields []byte
	for i, pmk := range primaryKeys {
		if i != pmk.PkPrecedence-1 {
			return nil, fmt.Errorf("wrong order for primary keys")
		}
		value, exists := fixedValues[pmk.ColumnName]
		if !exists {
			return nil, fmt.Errorf("missing primary key `%s`", pmk.ColumnName)
		}

		var orderEncodedField []byte
		var err error
		switch v := value.(type) {
		case int64:
			orderEncodedField, err = encodeInt64Key(v, encodeIntValuesWithBigEndian)
			if err != nil {
				return nil, err
			}
		case string:
			orderEncodedField, err = Append(nil, v)
			if err != nil {
				return nil, err
			}
			// the ordered code library always appends a delimiter to strings, but we have custom delimiter logic so remove it
			orderEncodedField = orderEncodedField[:len(orderEncodedField)-2]
		default:
			return nil, fmt.Errorf("unsupported row key type %T", value)
		}

		// Omit trailing empty fields from the encoding. We achieve this by holding
		// them back in a separate buffer until we hit a non-empty field.
		if len(orderEncodedField) == 0 {
			trailingEmptyFields = append(trailingEmptyFields, kOrderedCodeEmptyField...)
			trailingEmptyFields = append(trailingEmptyFields, kOrderedCodeDelimiter...)
			continue
		}

		if len(result) != 0 {
			result = append(result, kOrderedCodeDelimiter...)
		}

		// Since this field is non-empty, any empty fields we held back are not
		// trailing and should not be omitted. Add them in before appending the
		// latest field. Note that they will correctly end with a delimiter.
		result = append(result, trailingEmptyFields...)
		trailingEmptyFields = nil

		// Finally, append the non-empty field
		result = append(result, orderEncodedField...)
	}

	return result, nil
}

// encodeInt64Key encodes an int64 value for row keys.
// Converts int64 values to byte representation with validation.
// Returns error if value is invalid or encoding fails.
func encodeInt64Key(value int64, encodeIntValuesWithBigEndian bool) ([]byte, error) {
	// todo remove once ordered byte encoding is supported for ints
	// override ordered code value with BigEndian
	if encodeIntValuesWithBigEndian {

		var b bytes.Buffer
		err := binary.Write(&b, binary.BigEndian, value)
		if err != nil {
			return nil, err
		}

		result, err := Append(nil, b.String())
		if err != nil {
			return nil, err
		}

		return result[:len(result)-2], nil
	}

	return Append(nil, value)
}

// DataConversionInInsertionIfRequired performs data type conversion for insertions.
// Handles type conversion and validation for insert operations.
// Returns error if value type is invalid or conversion fails.
func DataConversionInInsertionIfRequired(value interface{}, pv primitive.ProtocolVersion, cqlType datatype.DataType, responseType string) (interface{}, error) {
	switch cqlType {
	case datatype.Boolean:
		switch responseType {
		case "string":
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
	case datatype.Bigint:
		switch responseType {
		case "string":
			val, err := strconv.ParseInt(value.(string), 10, 64)
			if err != nil {
				return nil, err
			}
			stringVal := strconv.FormatInt(val, 10)
			return stringVal, nil
		default:
			return EncodeBigInt(value, pv)
		}
	case datatype.Int:
		switch responseType {
		case "string":
			val, err := strconv.Atoi(value.(string))
			if err != nil {
				return nil, err
			}
			stringVal := strconv.Itoa(val)
			return stringVal, nil
		default:
			return EncodeInt(value, pv)
		}
	default:
		return value, nil
	}
}

// EncodeBool encodes boolean values to bytes.
// Converts boolean values to byte representation with validation.
// Returns error if value is invalid or encoding fails.
func EncodeBool(value interface{}, pv primitive.ProtocolVersion) ([]byte, error) {
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
		if vaInInterface == nil {
			return nil, nil
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

// EncodeBigInt encodes bigint values to bytes.
// Converts bigint values to byte representation with validation.
// Returns error if value is invalid or encoding fails.
func EncodeBigInt(value interface{}, pv primitive.ProtocolVersion) ([]byte, error) {
	var intVal int64
	var err error
	switch v := value.(type) {
	case string:
		intVal, err = strconv.ParseInt(v, 10, 64)
	case int32:
		intVal = int64(v)
	case int64:
		intVal = v
	case []byte:
		var decoded interface{}
		if len(v) == 4 {
			decoded, err = proxycore.DecodeType(datatype.Int, pv, v)
			if err != nil {
				return nil, err
			}
			intVal = int64(decoded.(int32))
		} else {
			decoded, err = proxycore.DecodeType(datatype.Bigint, pv, v)
			if err != nil {
				return nil, err
			}
			if decoded == nil {
				return nil, nil
			}
			intVal = decoded.(int64)
		}
	default:
		return nil, fmt.Errorf("unsupported type for bigint conversion: %v", value)
	}
	if err != nil {
		return nil, err
	}
	result, err := proxycore.EncodeType(datatype.Bigint, pv, intVal)
	if err != nil {
		return nil, fmt.Errorf("failed to encode bigint: %w", err)
	}
	return result, err
}

// EncodeInt encodes integer values to bytes.
// Converts integer values to byte representation with validation.
// Returns error if value is invalid or encoding fails.
func EncodeInt(value interface{}, pv primitive.ProtocolVersion) ([]byte, error) {
	var int32Val int32
	var err error
	switch v := value.(type) {
	case string:
		intVal, err := strconv.Atoi(v)
		if err != nil {
			return nil, err
		}
		int32Val = int32(intVal)
	case int32:
		int32Val = v
	case []byte:
		var decoded interface{}
		decoded, err = proxycore.DecodeType(datatype.Int, pv, v)
		if err != nil {
			return nil, err
		}
		if decoded == nil {
			return nil, nil
		}
		int32Val = decoded.(int32)
	default:
		return nil, fmt.Errorf("unsupported type for int conversion: %v", value)
	}
	if err != nil {
		return nil, err
	}
	result, err := proxycore.EncodeType(datatype.Bigint, pv, int64(int32Val))
	if err != nil {
		return nil, fmt.Errorf("failed to encode int: %w", err)
	}
	return result, err
}

// ProcessComplexUpdate processes complex update operations.
// Handles complex column updates including collections and counters with validation.
// Returns error if update type is invalid or processing fails.
func (t *Translator) ProcessComplexUpdate(columns []types.Column, values []interface{}, tableName, keyspaceName string, prependColumns []string) (map[string]*ComplexOperation, error) {
	complexMeta := make(map[string]*ComplexOperation)

	for i, column := range columns {
		val := values[i]
		// Only process ComplexAssignment types
		ca, ok := val.(ComplexAssignment)
		if !ok {
			continue
		}

		meta := &ComplexOperation{}

		switch column.CQLType.GetDataTypeCode() {
		case primitive.DataTypeCodeList:
			// a. + operation (append): only set append true
			if ca.Operation == "+" && ca.Left == column.Name {
				meta.Append = true
			}
			// b. + operation (prepend): only set prepend true
			if ca.Operation == "+" && ca.Right == column.Name {
				meta.PrependList = true
			}
			// c. marks[1]=? (update for index): set updateListIndex, set expected datatype as value type
			if ca.Operation == "update_index" {
				if idx, ok := ca.Left.(string); ok {
					meta.UpdateListIndex = idx
				}
				if listType, ok := column.CQLType.(datatype.ListType); ok {
					meta.ExpectedDatatype = listType.GetElementType()
				}
			}
			// d. - operation: just mark listDelete and delete as true
			if ca.Operation == "-" {
				meta.ListDelete = true
				meta.Delete = true
			}
			complexMeta[column.Name] = meta

		case primitive.DataTypeCodeMap:
			// 1. + operation: only mark append as true
			if ca.Operation == "+" {
				meta.Append = true
			}
			// 2. - operation: set expected datatype as SET<value_datatype>, mark delete as true
			if ca.Operation == "-" {
				meta.Delete = true
				// get map value datatype
				if mapType, ok := column.CQLType.(datatype.MapType); ok {
					meta.ExpectedDatatype = datatype.NewSetType(mapType.GetKeyType())
				}
			}
			// 3. map[key]=? (update for particular key): set updateMapKey, set expected datatype as value type, mark append as true
			if ca.Operation == "update_key" || ca.Operation == "update_index" {
				meta.Append = true
				if mapType, ok := column.CQLType.(datatype.MapType); ok {
					meta.ExpectedDatatype = mapType.GetValueType()
				}
				if key, ok := ca.Left.(string); ok {
					meta.mapKey = key
				}
			}
			complexMeta[column.Name] = meta
		case primitive.DataTypeCodeSet:
			if ca.Operation == "-" {
				meta.Delete = true
			}
			complexMeta[column.Name] = meta
		case primitive.DataTypeCodeCounter:
			if ca.Operation == "+" || ca.Operation == "-" {
				var op = Increment
				if ca.Operation == "-" {
					op = Decrement
				}
				meta.IncrementType = op
				meta.ExpectedDatatype = datatype.Bigint
				complexMeta[column.Name] = meta
			} else {
				return nil, fmt.Errorf("unsupported counter operation: `%s`", ca.Operation)
			}
		default:
			return nil, fmt.Errorf("column %s is not a collection type", column.Name)
		}
	}
	return complexMeta, nil
}

// getFromSpecElement extracts the FROM clause element from a query.
// Parses and returns the FROM specification element with validation.
// Returns error if FROM clause is invalid or parsing fails.
func getFromSpecElement(input cql.IFromSpecContext) (cql.IFromSpecElementContext, error) {
	if input == nil {
		return nil, errors.New("input context is nil")
	}
	fromSpec := input.FromSpecElement()
	if fromSpec == nil {
		return nil, errors.New("error while parsing fromSpec")
	}
	return fromSpec, nil
}

// getAllObjectNames extracts all object names from a FROM clause.
// Returns a list of all object names in the FROM specification with validation.
// Returns error if no objects found or parsing fails.
func getAllObjectNames(fromSpec cql.IFromSpecElementContext) ([]antlr.TerminalNode, error) {
	allObj := fromSpec.AllOBJECT_NAME()
	if allObj == nil {
		return nil, errors.New("error while parsing all objects from the fromSpec")
	}
	if len(allObj) == 0 {
		return nil, errors.New("could not find table and keyspace name")
	}
	return allObj, nil
}

// getTableAndKeyspaceObjects extracts table and keyspace names.
// Parses and returns the table and keyspace names from object names with validation.
// Returns error if names are invalid or parsing fails.
func getTableAndKeyspaceObjects(allObj []antlr.TerminalNode) (string, string, error) {
	var keyspaceName, tableName string

	if len(allObj) == 2 {
		keyspaceName = allObj[0].GetText()
		tableName = allObj[1].GetText()
	} else if len(allObj) == 1 {
		keyspaceName = ""
		tableName = allObj[0].GetText()
	} else {
		return "", "", errors.New("could not find table or keyspace name or some extra parameter provided")
	}

	if tableName == "" {
		return "", "", fmt.Errorf("table is missing")
	}

	return keyspaceName, tableName, nil
}

// NewCqlParser creates a new CQL parser instance.
// Initializes and returns a parser for CQL queries with validation.
// Returns error if query is invalid or parser initialization fails.
func NewCqlParser(cqlQuery string, isDebug bool) (*cql.CqlParser, error) {
	if cqlQuery == "" {
		return nil, fmt.Errorf("invalid input string")
	}

	lexer := cql.NewCqlLexer(antlr.NewInputStream(cqlQuery))
	stream := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)
	p := cql.NewCqlParser(stream)
	if p == nil {
		return nil, fmt.Errorf("error while creating parser object")
	}

	if !isDebug {
		p.RemoveErrorListeners()
	}
	return p, nil
}

// parseCqlValue parses a CQL value expression.
// Converts CQL value expressions to their corresponding Go types with validation.
// Returns error if expression is invalid or conversion fails.
func parseCqlValue(expr antlr.ParserRuleContext) (interface{}, error) {
	// If this is an expression context, check for subcontexts
	if e, ok := expr.(cql.IExpressionContext); ok {
		if m := e.AssignmentMap(); m != nil {
			return parseCqlValue(m)
		}
		if l := e.AssignmentList(); l != nil {
			return parseCqlValue(l)
		}
		if s := e.AssignmentSet(); s != nil {
			return parseCqlValue(s)
		}
		if c := e.Constant(); c != nil {
			return parseCqlConstant(c)
		}
		// Optionally handle tuple, function call, etc.
		return e.GetText(), nil // fallback
	}
	// Handle map context
	if m, ok := expr.(cql.IAssignmentMapContext); ok {
		result := make(map[string]string)
		constants := m.AllConstant()
		for i := 0; i+1 < len(constants); i += 2 {
			keyRaw, _ := parseCqlConstant(constants[i])
			val, err := parseCqlConstant(constants[i+1])
			if err != nil {
				return nil, err
			}
			key := fmt.Sprintf("%v", keyRaw)
			result[key] = fmt.Sprintf("%v", val)
		}
		return result, nil
	}
	// Handle list context
	if l, ok := expr.(cql.IAssignmentListContext); ok {
		var result []string
		for _, c := range l.AllConstant() {
			val, err := parseCqlConstant(c)
			if err != nil {
				return nil, err
			}
			result = append(result, fmt.Sprintf("%v", val))
		}
		return result, nil
	}
	// Handle set context
	if s, ok := expr.(cql.IAssignmentSetContext); ok {
		var result []string
		constants := s.AllConstant()
		if len(constants) == 0 {
			return result, nil
		}
		for _, c := range constants {
			if c == nil {
				continue
			}
			val, err := parseCqlConstant(c)
			if err != nil {
				return nil, err
			}
			result = append(result, fmt.Sprintf("%v", val))
		}
		return result, nil
	}
	return expr.GetText(), nil // fallback
}

// parseCqlConstant parses a CQL constant value.
// Converts CQL constant values to their corresponding Go types with validation.
// Returns error if constant is invalid or conversion fails.
func parseCqlConstant(c cql.IConstantContext) (interface{}, error) {

	if c.StringLiteral() != nil {
		return strings.Trim(c.StringLiteral().GetText(), "'"), nil
	}

	if c.DecimalLiteral() != nil {
		if c.DecimalLiteral().GetText() == "?" {
			return "?", nil
		}
		return strconv.Atoi(c.DecimalLiteral().GetText())
	}
	if c.FloatLiteral() != nil {
		return strconv.ParseFloat(c.FloatLiteral().GetText(), 64)
	}
	if c.BooleanLiteral() != nil {
		val := c.BooleanLiteral().GetText()
		if val == "true" {
			return true, nil
		} else if val == "false" {
			return false, nil
		}
		return nil, fmt.Errorf("unknown boolean literal: %s", val)
	}
	if c.KwNull() != nil {
		return nil, nil
	}
	return nil, fmt.Errorf("unknown constant: %s", c.GetText())
}
