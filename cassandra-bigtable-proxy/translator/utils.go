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
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/bigtable"
	"github.com/antlr4-go/antlr/v4"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/ollionorg/cassandra-to-bigtable-proxy/collectiondecoder"
	"github.com/ollionorg/cassandra-to-bigtable-proxy/proxycore"
	schemaMapping "github.com/ollionorg/cassandra-to-bigtable-proxy/schema-mapping"
	cql "github.com/ollionorg/cassandra-to-bigtable-proxy/translator/cqlparser"
	"github.com/ollionorg/cassandra-to-bigtable-proxy/utilities"
)

const (
	literalPlaceholder  = "_a1b2c3"
	ttlValuePlaceholder = "ttlValue"
	tsValuePlaceholder  = "tsValue"
	bigtableTSColumn    = "last_commit_ts"
	limitPlaceholder    = "limitValue"
	writetime           = "writetime"
	commitTsFn          = "PENDING_COMMIT_TIMESTAMP()"
	missingUndefined    = "<missing undefined>"
	questionMark        = "?"
	STAR                = "*"
	maxNanos            = int32(9999)
	referenceTime       = int64(1262304000000)
)

var (
	whereRegex          = regexp.MustCompile(`(?i)\bwhere\b`)
	orderByRegex        = regexp.MustCompile(`(?i)\border\s+by\b`)
	limitRegex          = regexp.MustCompile(`(?i)\blimit\b`)
	usingTimestampRegex = regexp.MustCompile(`using.*timestamp`)
	combinedRegex       = createCombinedRegex()
)

var (
	ErrEmptyTable           = errors.New("could not find table name to create spanner query")
	ErrParsingDelObj        = errors.New("error while parsing delete object")
	ErrParsingTs            = errors.New("timestamp could not be parsed")
	ErrTsNoValue            = errors.New("no value found for Timestamp")
	ErrEmptyTableOrKeyspace = errors.New("ToSpannerDelete: No table or keyspace name found in the query")
)

// convertMapString(): Function to convert map data into String
func convertMapString(dataStr string) (map[string]interface{}, error) {
	// Split the string into key-value pairs
	pairs := strings.Split(strings.Trim(dataStr, "{}"), ", ")

	// Initialize an empty map for storing data
	data := map[string]interface{}{}

	// // Extract key-value pairs and add them to the map
	for _, pair := range pairs {
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 {
			parts = strings.SplitN(pair, "=", 2)
			if len(parts) != 2 {
				// Handle the case where the split doesn't result in 2 parts
				return nil, fmt.Errorf("error while formatting %v", parts)
			}
		}

		v := strings.TrimSpace(strings.ToLower(parts[1]))
		k := strings.ReplaceAll(parts[0], `"`, ``)
		if v == "true" {
			data[k] = true
		} else {
			data[k] = false
		}

	}

	return data, nil
}

// convertToMap(): function to convert json tring to cassandra map type
func convertToMap(dataStr string, cqlType string) (map[string]interface{}, error) {
	// Split the string into key-value pairs
	dataStr = strings.ReplaceAll(dataStr, ": \"", ":\"")
	dataStr = strings.ReplaceAll(dataStr, "\"", "")
	pairs := strings.Split(strings.Trim(dataStr, "{}"), ", ")
	// Initialize an empty map for storing data
	switch cqlType {
	case "map<text, timestamp>":
		return mapOfStringToTimestamp(pairs)
	case "map<text, text>":
		return mapOfStringToString(pairs)
	default:
		return nil, fmt.Errorf("unhandled map type")
	}
}

// mapOfStringToString(): Converts a slice of key-value pairs into a map with string keys and string values.
func mapOfStringToString(pairs []string) (map[string]interface{}, error) {
	data := make(map[string]interface{})
	for _, pair := range pairs {
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 {
			parts = strings.SplitN(pair, "=", 2)
			if len(parts) != 2 {
				// Handle the case where the split doesn't result in 2 parts
				return nil, fmt.Errorf("error while formatting %v", parts)
			}

		}
		data[parts[0]] = parts[1]
	}
	return data, nil
}

// mapOfStringToTimestamp(): Converts a slice of key-value pairs into a map with string keys and timestamp values.
func mapOfStringToTimestamp(pairs []string) (map[string]interface{}, error) {
	data := make(map[string]interface{})
	for _, pair := range pairs {
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 {
			parts = strings.SplitN(pair, "=", 2)
			if len(parts) != 2 {
				// Handle the case where the split doesn't result in 2 parts
				return nil, fmt.Errorf("error while formatting %v", parts)
			}
		}
		timestamp, err := parseTimestamp(strings.TrimSpace(parts[1]))
		if err != nil {
			return nil, fmt.Errorf("error while typecasting to timestamp %v, -> %s", parts[1], err)
		}
		data[parts[0]] = timestamp
	}
	return data, nil
}

// hasUsingTimestamp(): Function to check if using timestamp exist in the query.
func hasUsingTimestamp(query string) bool {
	return usingTimestampRegex.MatchString(query)
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

// treetoString(): Function to parse string from ANTLR tree
func treetoString(tree antlr.Tree) string {
	if (reflect.TypeOf(tree) == reflect.TypeOf(&antlr.TerminalNodeImpl{})) {
		token := tree.(antlr.TerminalNode).GetSymbol()
		return token.GetText()

	}
	return ""
}

// hasWhere(): Function to check if where clause exist in the query.
func hasWhere(query string) bool {
	return whereRegex.MatchString(query)
}

// hasOrderBy(): Function to check if order by exist in the query.
func hasOrderBy(query string) bool {
	return orderByRegex.MatchString(query)
}

// hasLimit(): Function to check if limit exist in the query.
func hasLimit(query string) bool {
	return limitRegex.MatchString(query)
}

// formatValues(): Function to encode the cassandra values into bigEndian octal format.
func formatValues(value string, cqlType string, protocolV primitive.ProtocolVersion) ([]byte, error) {
	var iv interface{}
	var dt datatype.DataType

	switch cqlType {
	case "int":
		val, err := strconv.ParseInt(value, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("error converting string to int32: %w", err)
		}
		iv = int32(val)
		dt = datatype.Int

	case "bigint":
		val, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("error converting string to int64: %w", err)
		}
		iv = val
		dt = datatype.Bigint

	case "float":
		val, err := strconv.ParseFloat(value, 32)
		if err != nil {
			return nil, fmt.Errorf("error converting string to float32: %w", err)
		}
		iv = float32(val)
		dt = datatype.Float
	case "double":
		val, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, fmt.Errorf("error converting string to float64: %w", err)
		}
		iv = val
		dt = datatype.Double

	case "boolean":
		val, err := strconv.ParseBool(value)
		if err != nil {
			return nil, fmt.Errorf("error converting string to bool: %w", err)
		}
		iv = val
		dt = datatype.Boolean

	case "timestamp":
		val, err := parseTimestamp(value)
		if err != nil {
			return nil, fmt.Errorf("error converting string to timestamp: %w", err)
		}
		iv = val
		dt = datatype.Timestamp
	case "blob":
		iv = value
		dt = datatype.Blob
	case "text":
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

// primitivesToString(): Function used to  Convert primitive values into string
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

// stringToPrimitives(): Converts a string value to its corresponding primitive type based on the CQL type.
func stringToPrimitives(value string, cqlType string) (interface{}, error) {
	var iv interface{}

	switch cqlType {
	case "int":
		val, err := strconv.ParseInt(value, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("error converting string to int32: %w", err)
		}
		iv = int32(val)

	case "bigint":
		val, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("error converting string to int64: %w", err)
		}
		iv = val

	case "float":
		val, err := strconv.ParseFloat(value, 32)
		if err != nil {
			return nil, fmt.Errorf("error converting string to float32: %w", err)
		}
		iv = float32(val)

	case "double":
		val, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, fmt.Errorf("error converting string to float64: %w", err)
		}
		iv = val
	case "boolean":
		val, err := strconv.ParseBool(value)
		if err != nil {
			return nil, fmt.Errorf("error converting string to bool: %w", err)
		}
		iv = val

	case "timestamp":
		val, err := parseTimestamp(value)

		if err != nil {
			return nil, fmt.Errorf("error converting string to timestamp: %w", err)
		}
		iv = val
	case "blob":
		iv = value
	case "text":
		iv = value

	default:
		return nil, fmt.Errorf("unsupported CQL type: %s", cqlType)

	}
	return iv, nil
}

// createCombinedRegex(): constructs a regular expression that matches any of the specified keywords as whole words.
// The keywords are "time", "key", "type", and "json".
// The resulting regex will match these keywords only if they appear as complete words in the input string.
//
// Returns:
//
//	*regexp.Regexp: A compiled regular expression object.
func createCombinedRegex() *regexp.Regexp {
	keywords := []string{"time", "key", "type", "json"}
	combinedPattern := fmt.Sprintf("\\b(%s)\\b", regexp.QuoteMeta(keywords[0]))

	for _, keyword := range keywords[1:] {
		combinedPattern += fmt.Sprintf("|\\b%s\\b", regexp.QuoteMeta(keyword))
	}
	return regexp.MustCompile(combinedPattern)
}

// renameLiterals(): Function to find and replace the cql literals and add a temporary placeholder.
func renameLiterals(query string) string {
	return combinedRegex.ReplaceAllStringFunc(query, func(match string) string {
		return match + literalPlaceholder
	})
}

// getPrimaryKeys(): Function to get primary keys from schema mapping.
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

// preprocessMapString(): Preprocess the input string to convert it to valid JSON
func preprocessMapString(input string) string {
	input = strings.TrimPrefix(input, "{")
	input = strings.TrimSuffix(input, "}")
	pairs := strings.Split(input, ",")
	for i, pair := range pairs {
		kv := strings.Split(pair, ":")
		if len(kv) != 2 {
			kv = strings.Split(pair, "=")
		}
		if len(kv) == 2 {
			key := strings.TrimSpace(kv[0])
			value := strings.TrimSpace(kv[1])
			pairs[i] = fmt.Sprintf("\"%s\":\"%s\"", key, value)
		}
	}
	return "{" + strings.Join(pairs, ",") + "}"
}

// preprocessSetString(): Preprocess the set string to convert it to valid JSON
func preprocessSetString(input string) string {
	if (strings.HasPrefix(input, "{") && strings.HasSuffix(input, "}")) || (strings.HasPrefix(input, "[") && strings.HasSuffix(input, "]")) {
		input = input[1 : len(input)-1]
	}
	values := strings.Split(input, ",")
	for i, value := range values {
		value = strings.TrimSpace(value)
		if !strings.HasPrefix(value, "\"") && !strings.HasSuffix(value, "\"") && !strings.HasPrefix(value, "'") && !strings.HasSuffix(value, "'") {
			values[i] = fmt.Sprintf("\"%s\"", value)
		} else {
			values[i] = strings.Trim(value, "'")
			values[i] = fmt.Sprintf("\"%s\"", values[i])
		}
	}
	return "[" + strings.Join(values, ",") + "]"
}

// processCollectionColumnsForRawQueries(): processes columns that involve collection types (e.g., map, set)
// This function operates by iterating over each column provided. If the column represents a collection (e.g., map or set),
// the function decodes the corresponding value (expected to be a JSON string), converts it into the appropriate Go type
// (e.g., map[string]string or []string), and then creates new column-value pairs for each element in the collection.
// Non-collection columns are added directly to the result slices without modification.
//
// Parameters:
// - columns: A slice of Column structures, each representing a column in the database.
// - values: A slice of interface{} values corresponding to each column, containing the raw data to be processed.
// - tableName: The name of the table being processed.
// - t: A pointer to a Translator, which provides utility methods for working with table schemas and collections.
//
// Returns:
// - A slice of Columns with processed data, reflecting the transformed collections.
// - A slice of interface{} values representing the processed data for each column.
// - An error if any step of the processing fails.
func processCollectionColumnsForRawQueries(columns []Column, values []interface{}, tableName string, t *Translator, keySpace string, prependColumns []string) ([]Column, []interface{}, []string, []Column, map[string]*ComplexUpdateMeta, error) {
	var newColumns []Column
	var newValues []interface{}
	var delColumnFamily []string
	var delColumns []Column
	complexMeta := make(map[string]*ComplexUpdateMeta)

	for i, column := range columns {
		if t.IsCollection(tableName, column.Name, keySpace) {
			val := values[i].(string)
			if strings.Contains(val, column.Name) && (strings.Contains(val, "+") || strings.Contains(val, "-")) {

				if strings.Contains(val, "+") {
					trimmedStr := strings.TrimPrefix(val, column.Name)
					trimmedStr = strings.TrimPrefix(trimmedStr, "+")
					val = strings.TrimSpace(trimmedStr)
				} else if strings.Contains(val, "-") {
					cf, qualifiers, err := ExtractCFAndQualifiersforMap(val)
					if err != nil {
						return nil, nil, nil, nil, nil, fmt.Errorf("error reading map set value: %w", err)
					}
					for _, key := range qualifiers {
						delColumns = append(delColumns, Column{Name: key, ColumnFamily: cf})
					}
					continue
				}
			} else {
				delColumnFamily = append(delColumnFamily, column.Name)
			}
			colFamily := t.GetColumnFamily(tableName, column.Name, keySpace)
			switch column.CQLType {
			case "map<text,text>":
				var mapValue map[string]string
				formattedJsonStr := preprocessMapString(val)
				// Parse the JSON string to a map
				if err := json.Unmarshal([]byte(formattedJsonStr), &mapValue); err != nil {
					return nil, nil, nil, nil, nil, fmt.Errorf("error converting string to map<text,text>: %w", err)
				}
				for k, v := range mapValue {
					c := Column{
						Name:         k,
						ColumnFamily: colFamily,
						CQLType:      "text",
					}
					val, err := formatValues(v, "text", primitive.ProtocolVersion4)
					if err != nil {
						return nil, nil, nil, nil, nil, fmt.Errorf("error converting string to map<text,text> value: %w", err)
					}
					newValues = append(newValues, val)
					newColumns = append(newColumns, c)
				}
			case "map<text,int>":
				var mapValue map[string]string
				formattedJsonStr := preprocessMapString(val)
				if err := json.Unmarshal([]byte(formattedJsonStr), &mapValue); err != nil {
					return nil, nil, nil, nil, nil, fmt.Errorf("error converting string to map<text,int>: %w", err)
				}
				for k, v := range mapValue {
					c := Column{
						Name:         k,
						ColumnFamily: colFamily,
						CQLType:      "int",
					}
					val, err := formatValues(v, "int", primitive.ProtocolVersion4)
					if err != nil {
						return nil, nil, nil, nil, nil, fmt.Errorf("error converting string to map<text,int> value: %w", err)
					}
					newValues = append(newValues, val)
					newColumns = append(newColumns, c)
				}
			case "map<text,timestamp>":
				var mapValue map[string]string
				formattedJsonStr := preprocessMapString(val)
				if err := json.Unmarshal([]byte(formattedJsonStr), &mapValue); err != nil {
					return nil, nil, nil, nil, nil, fmt.Errorf("error converting string to map<text,timestamp>: %w", err)
				}
				for k, v := range mapValue {
					c := Column{
						Name:         k,
						ColumnFamily: colFamily,
						CQLType:      "timestamp",
					}
					val, err := formatValues(v, "timestamp", primitive.ProtocolVersion4)
					if err != nil {
						return nil, nil, nil, nil, nil, fmt.Errorf("error converting string to map<text,timestamp> value: %w", err)
					}
					newValues = append(newValues, val)
					newColumns = append(newColumns, c)
				}
			case "map<text,float>":
				var mapValue map[string]string
				formattedJsonStr := preprocessMapString(val)
				if err := json.Unmarshal([]byte(formattedJsonStr), &mapValue); err != nil {
					return nil, nil, nil, nil, nil, fmt.Errorf("error converting string to map<text,float>: %w", err)
				}
				for k, v := range mapValue {
					c := Column{
						Name:         k,
						ColumnFamily: colFamily,
						CQLType:      "float",
					}
					val, err := formatValues(v, "float", primitive.ProtocolVersion4)
					if err != nil {
						return nil, nil, nil, nil, nil, fmt.Errorf("error converting string to map<text,float> value: %w", err)
					}
					newValues = append(newValues, val)
					newColumns = append(newColumns, c)
				}
			case "map<text,double>":
				var mapValue map[string]string
				formattedJsonStr := preprocessMapString(val)

				if err := json.Unmarshal([]byte(formattedJsonStr), &mapValue); err != nil {
					return nil, nil, nil, nil, nil, fmt.Errorf("error converting string to map<text,double>: %w", err)
				}
				for k, v := range mapValue {
					c := Column{
						Name:         k,
						ColumnFamily: colFamily,
						CQLType:      "double",
					}
					val, err := formatValues(v, "double", primitive.ProtocolVersion4)
					if err != nil {
						return nil, nil, nil, nil, nil, fmt.Errorf("error converting string to map<text,double> value: %w", err)
					}
					newValues = append(newValues, val)
					newColumns = append(newColumns, c)
				}
			case "map<text,boolean>":
				var mapValue map[string]string
				formattedJsonStr := preprocessMapString(val)
				// Parse the JSON string to a map
				if err := json.Unmarshal([]byte(formattedJsonStr), &mapValue); err != nil {
					return nil, nil, nil, nil, nil, fmt.Errorf("error converting string to map<text,boolean>: %w", err)
				}
				for k, v := range mapValue {
					c := Column{
						Name:         k,
						ColumnFamily: colFamily,
						CQLType:      "boolean",
					}
					val, err := formatValues(v, "boolean", primitive.ProtocolVersion4)
					if err != nil {
						return nil, nil, nil, nil, nil, fmt.Errorf("error converting string to map<text,boolean> value: %w", err)
					}
					newValues = append(newValues, val)
					newColumns = append(newColumns, c)
				}

			case "map<timestamp,boolean>":
				var mapValue map[string]string
				formattedJsonStr := preprocessMapString(val)

				if err := json.Unmarshal([]byte(formattedJsonStr), &mapValue); err != nil {
					return nil, nil, nil, nil, nil, fmt.Errorf("error converting string to map<timestamp,boolean>: %w", err)
				}
				for k, v := range mapValue {
					c := Column{
						Name:         k,
						ColumnFamily: colFamily,
						CQLType:      "boolean",
					}
					val, err := formatValues(v, "boolean", primitive.ProtocolVersion4)
					if err != nil {
						return nil, nil, nil, nil, nil, fmt.Errorf("error converting string to map<timestamp,boolean> value: %w", err)
					}
					newValues = append(newValues, val)
					newColumns = append(newColumns, c)
				}
			case "map<timestamp,timestamp>":
				var mapValue map[string]string
				formattedJsonStr := preprocessMapString(val)

				if err := json.Unmarshal([]byte(formattedJsonStr), &mapValue); err != nil {
					return nil, nil, nil, nil, nil, fmt.Errorf("error converting string to map<timestamp,timestamp>: %w", err)
				}
				for k, v := range mapValue {
					c := Column{
						Name:         k,
						ColumnFamily: colFamily,
						CQLType:      "timestamp",
					}
					val, err := formatValues(v, "timestamp", primitive.ProtocolVersion4)
					if err != nil {
						return nil, nil, nil, nil, nil, fmt.Errorf("error converting string to map<timestamp,timestamp> value: %w", err)
					}
					newValues = append(newValues, val)
					newColumns = append(newColumns, c)
				}
			case "map<timestamp,text>":
				var mapValue map[string]string
				formattedJsonStr := preprocessMapString(val)

				if err := json.Unmarshal([]byte(formattedJsonStr), &mapValue); err != nil {
					return nil, nil, nil, nil, nil, fmt.Errorf("error converting string to map<timestamp,text>: %w", err)
				}
				for k, v := range mapValue {
					c := Column{
						Name:         k,
						ColumnFamily: colFamily,
						CQLType:      "text",
					}
					val, err := formatValues(v, "text", primitive.ProtocolVersion4)
					if err != nil {
						return nil, nil, nil, nil, nil, fmt.Errorf("error converting string to map<timestamp,text> value: %w", err)
					}
					newValues = append(newValues, val)
					newColumns = append(newColumns, c)
				}
			case "map<timestamp,int>":
				var mapValue map[string]string
				formattedJsonStr := preprocessMapString(val)
				if err := json.Unmarshal([]byte(formattedJsonStr), &mapValue); err != nil {
					return nil, nil, nil, nil, nil, fmt.Errorf("error converting string to map<timestamp,int>: %w", err)
				}
				for k, v := range mapValue {
					c := Column{
						Name:         k,
						ColumnFamily: colFamily,
						CQLType:      "int",
					}
					val, err := formatValues(v, "int", primitive.ProtocolVersion4)
					if err != nil {
						return nil, nil, nil, nil, nil, fmt.Errorf("error converting string to map<timestamp,int> value: %w", err)
					}
					newValues = append(newValues, val)
					newColumns = append(newColumns, c)
				}
			case "map<timestamp,bigint>":
				var mapValue map[string]string
				formattedJsonStr := preprocessMapString(val)
				if err := json.Unmarshal([]byte(formattedJsonStr), &mapValue); err != nil {
					return nil, nil, nil, nil, nil, fmt.Errorf("error converting string to map<timestamp,bigint>: %w", err)
				}
				for k, v := range mapValue {
					c := Column{
						Name:         k,
						ColumnFamily: colFamily,
						CQLType:      "bigint",
					}
					val, err := formatValues(v, "bigint", primitive.ProtocolVersion4)
					if err != nil {
						return nil, nil, nil, nil, nil, fmt.Errorf("error converting string to map<timestamp,bigint> value: %w", err)
					}
					newValues = append(newValues, val)
					newColumns = append(newColumns, c)
				}
			case "map<timestamp,float>":
				var mapValue map[string]string
				formattedJsonStr := preprocessMapString(val)

				if err := json.Unmarshal([]byte(formattedJsonStr), &mapValue); err != nil {
					return nil, nil, nil, nil, nil, fmt.Errorf("error converting string to map<timestamp,float>: %w", err)
				}
				for k, v := range mapValue {
					c := Column{
						Name:         k,
						ColumnFamily: colFamily,
						CQLType:      "float",
					}
					val, err := formatValues(v, "float", primitive.ProtocolVersion4)
					if err != nil {
						return nil, nil, nil, nil, nil, fmt.Errorf("error converting string to map<timestamp,float> value: %w", err)
					}
					newValues = append(newValues, val)
					newColumns = append(newColumns, c)
				}
			case "map<timestamp,double>":
				var mapValue map[string]string
				formattedJsonStr := preprocessMapString(val)

				if err := json.Unmarshal([]byte(formattedJsonStr), &mapValue); err != nil {
					return nil, nil, nil, nil, nil, fmt.Errorf("error converting string to map<timestamp,double>: %w", err)
				}
				for k, v := range mapValue {
					c := Column{
						Name:         k,
						ColumnFamily: colFamily,
						CQLType:      "double",
					}
					val, err := formatValues(v, "double", primitive.ProtocolVersion4)
					if err != nil {
						return nil, nil, nil, nil, nil, fmt.Errorf("error converting string to map<timestamp,double> value: %w", err)
					}
					newValues = append(newValues, val)
					newColumns = append(newColumns, c)
				}
			case "set<text>":
				var setValue []string
				formattedStr := preprocessSetString(val)
				// Parse the JSON string to a slice
				if err := json.Unmarshal([]byte(formattedStr), &setValue); err != nil {
					return nil, nil, nil, nil, nil, fmt.Errorf("error converting string to set<text>: %w", err)
				}
				for _, v := range setValue {
					c := Column{
						Name:         v,
						ColumnFamily: colFamily,
						CQLType:      "text",
					}
					// value parameter is empty, it's intentionally because column identifier has the value
					val, err := formatValues("", "text", primitive.ProtocolVersion4)
					if err != nil {
						return nil, nil, nil, nil, nil, fmt.Errorf("error converting string to set<text> value: %w", err)
					}
					newValues = append(newValues, val)
					newColumns = append(newColumns, c)
				}
			case "set<boolean>":
				var setValue []string
				formattedStr := preprocessSetString(val)
				// Parse the JSON string to a slice
				if err := json.Unmarshal([]byte(formattedStr), &setValue); err != nil {
					return nil, nil, nil, nil, nil, fmt.Errorf("error converting string to set<boolean>: %w", err)
				}
				for _, v := range setValue {
					c := Column{
						Name:         v,
						ColumnFamily: colFamily,
						CQLType:      "boolean",
					}
					// value parameter is empty, it's intentionally because column identifier has the value
					val, err := formatValues("", "text", primitive.ProtocolVersion4)
					if err != nil {
						return nil, nil, nil, nil, nil, fmt.Errorf("error converting string to set<boolean> value: %w", err)
					}
					newValues = append(newValues, val)
					newColumns = append(newColumns, c)
				}
			case "set<int>":
				var setValue []string
				formattedStr := preprocessSetString(val)
				// Parse the JSON string to a slice
				if err := json.Unmarshal([]byte(formattedStr), &setValue); err != nil {
					return nil, nil, nil, nil, nil, fmt.Errorf("error converting string to set<int>: %w", err)
				}
				for _, v := range setValue {
					c := Column{
						Name:         v,
						ColumnFamily: colFamily,
						CQLType:      "int",
					}
					// value parameter is empty, it's intentionally because column identifier has the value
					val, err := formatValues("", "text", primitive.ProtocolVersion4)
					if err != nil {
						return nil, nil, nil, nil, nil, fmt.Errorf("error converting string to set<int> value: %w", err)
					}
					newValues = append(newValues, val)
					newColumns = append(newColumns, c)
				}
			case "set<bigint>":
				var setValue []string
				formattedStr := preprocessSetString(val)
				// Parse the JSON string to a slice
				if err := json.Unmarshal([]byte(formattedStr), &setValue); err != nil {
					return nil, nil, nil, nil, nil, fmt.Errorf("error converting string to set<int>: %w", err)
				}
				for _, v := range setValue {
					c := Column{
						Name:         v,
						ColumnFamily: colFamily,
						CQLType:      "bigint",
					}
					// value parameter is empty, it's intentionally because column identifier has the value
					val, err := formatValues("", "text", primitive.ProtocolVersion4)
					if err != nil {
						return nil, nil, nil, nil, nil, fmt.Errorf("error converting string to set<int> value: %w", err)
					}
					newValues = append(newValues, val)
					newColumns = append(newColumns, c)
				}
			case "set<timestamp>":
				var setValue []string
				formattedStr := preprocessSetString(val)
				// Parse the JSON string to a slice
				if err := json.Unmarshal([]byte(formattedStr), &setValue); err != nil {
					return nil, nil, nil, nil, nil, fmt.Errorf("error converting string to set<timestamp>: %w", err)
				}
				for _, v := range setValue {
					c := Column{
						Name:         v,
						ColumnFamily: colFamily,
						CQLType:      "timestamp",
					}
					// value parameter is empty, it's intentionally because column identifier has the value
					val, err := formatValues("", "text", primitive.ProtocolVersion4)
					if err != nil {
						return nil, nil, nil, nil, nil, fmt.Errorf("error converting string to set<timestamp> value: %w", err)
					}
					newValues = append(newValues, val)
					newColumns = append(newColumns, c)
				}
			case "set<float>":
				var setValue []string
				formattedStr := preprocessSetString(val)
				// Parse the JSON string to a slice
				if err := json.Unmarshal([]byte(formattedStr), &setValue); err != nil {
					return nil, nil, nil, nil, nil, fmt.Errorf("error converting string to set<float>: %w", err)
				}
				for _, v := range setValue {
					c := Column{
						Name:         v,
						ColumnFamily: colFamily,
						CQLType:      "float",
					}
					// value parameter is empty, it's intentionally because column identifier has the value
					val, err := formatValues("", "text", primitive.ProtocolVersion4)
					if err != nil {
						return nil, nil, nil, nil, nil, fmt.Errorf("error converting string to set<float> value: %w", err)
					}
					newValues = append(newValues, val)
					newColumns = append(newColumns, c)
				}
			case "set<double>":
				var setValue []string
				formattedStr := preprocessSetString(val)
				// Parse the JSON string to a slice
				if err := json.Unmarshal([]byte(formattedStr), &setValue); err != nil {
					return nil, nil, nil, nil, nil, fmt.Errorf("error converting string to set<double>: %w", err)
				}
				for _, v := range setValue {
					c := Column{
						Name:         v,
						ColumnFamily: colFamily,
						CQLType:      "double",
					}
					// value parameter is empty, it's intentionally because column identifier has the value
					val, err := formatValues("", "text", primitive.ProtocolVersion4)
					if err != nil {
						return nil, nil, nil, nil, nil, fmt.Errorf("error converting string to set<double> value: %w", err)
					}
					newValues = append(newValues, val)
					newColumns = append(newColumns, c)
				}

			case "list<text>":

				if err := handleListType(val, colFamily, "text", &newValues, &newColumns, prependColumns, complexMeta); err != nil {
					return nil, nil, nil, nil, nil, fmt.Errorf("error processing list<text>: %w", err)
				}
			case "list<int>":

				if err := handleListType(val, colFamily, "int", &newValues, &newColumns, prependColumns, complexMeta); err != nil {
					return nil, nil, nil, nil, nil, fmt.Errorf("error processing list<int>: %w", err)
				}
			case "list<bigint>":

				if err := handleListType(val, colFamily, "bigint", &newValues, &newColumns, prependColumns, complexMeta); err != nil {
					return nil, nil, nil, nil, nil, fmt.Errorf("error processing list<int>: %w", err)
				}
			case "list<boolean>":

				if err := handleListType(val, colFamily, "boolean", &newValues, &newColumns, prependColumns, complexMeta); err != nil {
					return nil, nil, nil, nil, nil, fmt.Errorf("error processing list<int>: %w", err)
				}
			case "list<float>":

				if err := handleListType(val, colFamily, "float", &newValues, &newColumns, prependColumns, complexMeta); err != nil {
					return nil, nil, nil, nil, nil, fmt.Errorf("error processing list<int>: %w", err)
				}
			case "list<double>":

				if err := handleListType(val, colFamily, "double", &newValues, &newColumns, prependColumns, complexMeta); err != nil {
					return nil, nil, nil, nil, nil, fmt.Errorf("error processing list<int>: %w", err)
				}
			case "list<timestamp>":

				if err := handleListType(val, colFamily, "timestamp", &newValues, &newColumns, prependColumns, complexMeta); err != nil {
					return nil, nil, nil, nil, nil, fmt.Errorf("error processing list<int>: %w", err)
				}
			}
		} else {
			newColumns = append(newColumns, column)
			newValues = append(newValues, values[i])
		}
	}
	return newColumns, newValues, delColumnFamily, delColumns, complexMeta, nil
}

// processCollectionColumnsForPrepareQueries(): processes a set of columns and their associated values
// in preparation for generating queries. It handles various CQL collection types such as
// maps and sets, decoding them into Go-native types. This function is particularly useful
// when preparing data for operations like inserts or updates in a Cassandra-like database.
//
// Parameters:
// - columnsResponse: A slice of Column structures, each representing a column in the database.
// - values: A slice of pointers to primitive.Value, representing the values corresponding to each column.
// - tableName: The name of the table being processed.
// - protocolV: The protocol version used for decoding the values.
// - primaryKeys: A slice of strings representing the primary keys for the table.
// - t: A pointer to a Translator, which provides utility methods for working with table schemas and collections.
//
// Returns:
// - A slice of Columns with processed data.
// - A slice of interface{} values representing the processed data for each column.
// - A map[string]interface{} containing the unencrypted values of primary keys.
// - An integer indexEnd representing the last index processed in the columnsResponse.
// - An error if any step of the processing fails.
func processCollectionColumnsForPrepareQueries(columnsResponse []Column, values []*primitive.Value, tableName string, protocolV primitive.ProtocolVersion, primaryKeys []string, t *Translator, keySpace string, complexMeta map[string]*ComplexUpdateMeta) ([]Column, []interface{}, map[string]interface{}, int, []string, []Column, error) {
	var (
		newColumns       []Column
		newValues        []interface{}
		unencrypted      = make(map[string]interface{})
		indexEnd         int
		delColumnFamily  []string
		delColumns       []Column
		compleUpdateMeta *ComplexUpdateMeta
	)

	for i, column := range columnsResponse {
		if t.IsCollection(tableName, column.Name, keySpace) {

			colFamily := t.GetColumnFamily(tableName, column.Name, keySpace)
			if meta, ok := complexMeta[column.Name]; ok {
				if strings.Contains(column.CQLType, "list") {
					compleUpdateMeta = meta
					if compleUpdateMeta.UpdateListIndex != "" { // list index operation
						compleUpdateMeta.Value = values[i].Contents
						continue
					}
				}
				if meta.Append {
					if strings.Contains(column.CQLType, "map") && meta.key != "" { // for map append
						decodedValue, err := proxycore.DecodeType(meta.ExpectedDatatype, protocolV, values[i].Contents)
						if err != nil {
							return nil, nil, nil, 0, nil, nil, fmt.Errorf("error decoding string to %s value: %w", column.CQLType, err)
						}
						setKey, err := primitivesToString(meta.key)
						if err != nil {
							return nil, nil, nil, 0, nil, nil, fmt.Errorf("failed to convert/read key: %w", err)
						}
						setValue, err := primitivesToString(decodedValue)
						if err != nil {
							return nil, nil, nil, 0, nil, nil, fmt.Errorf("failed to convert value for key '%s': %w", setKey, err)
						}
						dt := strings.Split(column.CQLType, "map")
						cleaned := strings.Trim(dt[1], "<>")
						cleaned = strings.Trim(cleaned, " ")
						dts := strings.Split(cleaned, ",")
						valueType := dts[1]

						c := Column{
							Name:         setKey,
							ColumnFamily: colFamily,
							CQLType:      valueType,
						}
						val, _ := formatValues(setValue, valueType, protocolV)
						newValues = append(newValues, val)
						newColumns = append(newColumns, c)
						continue

					}

				}
				if meta.Delete && (strings.Contains(column.CQLType, "map") || strings.Contains(column.CQLType, "set")) {
					var err error
					expectedDt := meta.ExpectedDatatype
					if expectedDt == nil {
						expectedDt, err = utilities.GetCassandraColumnType(column.CQLType)
						if err != nil {
							return nil, nil, nil, 0, nil, nil, fmt.Errorf("invalid CQL Type %s err: %w", column.CQLType, err)
						}
					}
					decodedValue, err := collectiondecoder.DecodeCollection(expectedDt, protocolV, values[i].Contents)
					if err != nil {
						return nil, nil, nil, 0, nil, nil, fmt.Errorf("error decoding string to %s value: %w", column.CQLType, err)
					}
					setValue, ok := decodedValue.([]interface{})
					if !ok {
						return nil, nil, nil, 0, nil, nil, fmt.Errorf("unexpected type %T for set<%T>", decodedValue, "set")
					}
					for _, v := range setValue {
						setVal, err := primitivesToString(v)
						if err != nil {
							return nil, nil, nil, 0, nil, nil, fmt.Errorf("failed to convert value: %w", err)
						}
						delColumns = append(delColumns, Column{Name: setVal, ColumnFamily: column.Name})
					}
					continue
				}
			} else {
				delColumnFamily = append(delColumnFamily, column.Name)
			}

			dataType, _ := utilities.GetCassandraColumnType(column.CQLType)
			var decodedValue interface{}
			var err error
			switch column.CQLType {
			case "map<text,text>", "map<text,int>", "map<text,bigint>", "map<text,float>", "map<text,double>", "map<text,boolean>":
				decodedValue, err = proxycore.DecodeType(dataType, protocolV, values[i].Contents)
			case "map<text,timestamp>", "map<timestamp,timestamp>", "set<timestamp>", "list<timestamp>":

			default:
				decodedValue, err = collectiondecoder.DecodeCollection(dataType, protocolV, values[i].Contents)
			}
			if err != nil {
				return nil, nil, nil, 0, nil, nil, fmt.Errorf("error decoding string to %s value: %w", column.CQLType, err)

			}
			switch column.CQLType {
			case "map<text,text>":
				err := processTextMapColumn[string](
					decodedValue,
					protocolV,
					colFamily,
					"text",
					&newValues,
					&newColumns,
				)
				if err != nil {
					return nil, nil, nil, 0, nil, nil, err
				}
			case "map<text,int>":
				err := processTextMapColumn[int32](
					decodedValue,
					protocolV,
					colFamily,
					"int",
					&newValues,
					&newColumns,
				)
				if err != nil {
					return nil, nil, nil, 0, nil, nil, err
				}

			case "map<text,bigint>":
				err := processTextMapColumn[int64](
					decodedValue,
					protocolV,
					colFamily,
					"bigint",
					&newValues,
					&newColumns,
				)
				if err != nil {
					return nil, nil, nil, 0, nil, nil, err
				}

			case "map<text,timestamp>":
				// For timestamp, if special handling is needed, adjust dataType accordingly
				dataType := utilities.MapOfStrToBigInt
				decodedValue, err := proxycore.DecodeType(dataType, protocolV, values[i].Contents)
				if err != nil {
					return nil, nil, nil, 0, nil, nil, fmt.Errorf("error decoding string to %s value: %w", column.CQLType, err)
				}
				err = processTextMapColumn[int64](
					decodedValue,
					protocolV,
					colFamily,
					"bigint",
					&newValues,
					&newColumns,
				)
				if err != nil {
					return nil, nil, nil, 0, nil, nil, err
				}

			case "map<text,float>":
				err := processTextMapColumn[float32](
					decodedValue,
					protocolV,
					colFamily,
					"float",
					&newValues,
					&newColumns,
				)
				if err != nil {
					return nil, nil, nil, 0, nil, nil, err
				}

			case "map<text,double>":
				err := processTextMapColumn[float64](
					decodedValue,
					protocolV,
					colFamily,
					"double",
					&newValues,
					&newColumns,
				)
				if err != nil {
					return nil, nil, nil, 0, nil, nil, err
				}

			case "map<text,boolean>":
				err := processTextMapColumn[bool](
					decodedValue,
					protocolV,
					colFamily,
					"boolean",
					&newValues,
					&newColumns,
				)
				if err != nil {
					return nil, nil, nil, 0, nil, nil, err
				}

			case "map<timestamp,text>":
				err := processTimestampMapColumn[string](
					decodedValue,
					protocolV,
					colFamily,
					"text",
					&newValues,
					&newColumns,
				)
				if err != nil {
					return nil, nil, nil, 0, nil, nil, err
				}
			case "map<timestamp,int>":

				err := processTimestampMapColumn[int32](
					decodedValue,
					protocolV,
					colFamily,
					"int",
					&newValues,
					&newColumns,
				)
				if err != nil {
					return nil, nil, nil, 0, nil, nil, err
				}
			case "map<timestamp,bigint>":
				err := processTimestampMapColumn[int64](
					decodedValue,
					protocolV,
					colFamily,
					"bigint",
					&newValues,
					&newColumns,
				)
				if err != nil {
					return nil, nil, nil, 0, nil, nil, err
				}
			case "map<timestamp,timestamp>":
				dataType := utilities.MapOfTimeToTime
				decodedValue, err := collectiondecoder.DecodeCollection(dataType, protocolV, values[i].Contents)
				if err != nil {
					return nil, nil, nil, 0, nil, nil, fmt.Errorf("error decoding string to %s value: %w", column.CQLType, err)
				}
				decodedMap := make(map[int64]int64)
				if pointerMap, ok := decodedValue.(map[time.Time]time.Time); ok {
					for k, v := range pointerMap {
						key := k.UnixMilli()
						decodedMap[key] = v.UnixMilli()
					}
				}
				for k, v := range decodedMap {
					c := Column{
						Name:         strconv.FormatInt(k, 10),
						ColumnFamily: colFamily,
						CQLType:      "bigint",
					}
					val, _ := formatValues(strconv.FormatInt(v, 10), "bigint", protocolV)
					newValues = append(newValues, val)
					newColumns = append(newColumns, c)
				}
			case "map<timestamp,float>":
				err := processTimestampMapColumn[float32](
					decodedValue,
					protocolV,
					colFamily,
					"float",
					&newValues,
					&newColumns,
				)
				if err != nil {
					return nil, nil, nil, 0, nil, nil, err
				}
			case "map<timestamp,double>":
				err := processTimestampMapColumn[float64](
					decodedValue,
					protocolV,
					colFamily,
					"double",
					&newValues,
					&newColumns,
				)
				if err != nil {
					return nil, nil, nil, 0, nil, nil, err
				}
			case "map<timestamp,boolean>":
				err := processTimestampMapColumn[bool](
					decodedValue,
					protocolV,
					colFamily,
					"boolean",
					&newValues,
					&newColumns,
				)
				if err != nil {
					return nil, nil, nil, 0, nil, nil, err
				}
			case "set<text>":
				err := processSetColumnGeneric[string](
					decodedValue,
					protocolV,
					colFamily,
					"text",
					&newValues,
					&newColumns,
				)
				if err != nil {
					return nil, nil, nil, 0, nil, nil, err
				}
			case "set<boolean>":

				err := processSetColumnGeneric[bool](
					decodedValue,
					protocolV,
					colFamily,
					"boolean",
					&newValues,
					&newColumns,
				)
				if err != nil {
					return nil, nil, nil, 0, nil, nil, err
				}
			case "set<int>":
				err := processSetColumnGeneric[int32](
					decodedValue,
					protocolV,
					colFamily,
					"int",
					&newValues,
					&newColumns,
				)
				if err != nil {
					return nil, nil, nil, 0, nil, nil, err
				}
			case "set<bigint>":
				err := processSetColumnGeneric[int64](
					decodedValue,
					protocolV,
					colFamily,
					"bigint",
					&newValues,
					&newColumns,
				)
				if err != nil {
					return nil, nil, nil, 0, nil, nil, err
				}
			case "set<float>":
				err := processSetColumnGeneric[float32](
					decodedValue,
					protocolV,
					colFamily,
					"float",
					&newValues,
					&newColumns,
				)
				if err != nil {
					return nil, nil, nil, 0, nil, nil, err
				}
			case "set<double>":
				err := processSetColumnGeneric[float64](
					decodedValue,
					protocolV,
					colFamily,
					"double",
					&newValues,
					&newColumns,
				)
				if err != nil {
					return nil, nil, nil, 0, nil, nil, err
				}
			case "set<timestamp>":
				dataType := utilities.SetOfBigInt
				decodedValue, err := collectiondecoder.DecodeCollection(dataType, protocolV, values[i].Contents)
				if err != nil {
					return nil, nil, nil, 0, nil, nil, fmt.Errorf("error decoding string to %s value: %w", column.CQLType, err)
				}
				err = processSetColumnGeneric[int64](
					decodedValue,
					protocolV,
					colFamily,
					"bigint",
					&newValues,
					&newColumns,
				)
				if err != nil {
					return nil, nil, nil, 0, nil, nil, err
				}
			case "list<text>":
				err = processListColumnGeneric[string](
					decodedValue,
					protocolV,
					colFamily,
					"text",
					&newValues,
					&newColumns,
					compleUpdateMeta,
				)

				if err != nil {
					return nil, nil, nil, 0, nil, nil, err
				}
			case "list<int>":

				err = processListColumnGeneric[int32](
					decodedValue,
					protocolV,
					colFamily,
					"int",
					&newValues,
					&newColumns,
					compleUpdateMeta,
				)
				if err != nil {
					return nil, nil, nil, 0, nil, nil, err
				}
			case "list<bigint>":

				err = processListColumnGeneric[int64](
					decodedValue,
					protocolV,
					colFamily,
					"bigint",
					&newValues,
					&newColumns,
					compleUpdateMeta,
				)
				if err != nil {
					return nil, nil, nil, 0, nil, nil, err
				}
			case "list<float>":

				err = processListColumnGeneric[float32](
					decodedValue,
					protocolV,
					colFamily,
					"float",
					&newValues,
					&newColumns,
					compleUpdateMeta,
				)
				if err != nil {
					return nil, nil, nil, 0, nil, nil, err
				}
			case "list<double>":
				err = processListColumnGeneric[float64](
					decodedValue,
					protocolV,
					colFamily,
					"double",
					&newValues,
					&newColumns,
					compleUpdateMeta,
				)
				if err != nil {
					return nil, nil, nil, 0, nil, nil, err
				}
			case "list<boolean>":
				err = processListColumnGeneric[bool](
					decodedValue,
					protocolV,
					colFamily,
					"boolean",
					&newValues,
					&newColumns,
					compleUpdateMeta,
				)
				if err != nil {
					return nil, nil, nil, 0, nil, nil, err
				}
			case "list<timestamp>":
				dataType := utilities.ListOfBigInt
				decodedValue, err := collectiondecoder.DecodeCollection(dataType, protocolV, values[i].Contents)
				if err != nil {
					return nil, nil, nil, 0, nil, nil, fmt.Errorf("error decoding string to %s value: %w", column.CQLType, err)
				}
				err = processListColumnGeneric[int64](
					decodedValue,
					protocolV,
					colFamily,
					"bigint",
					&newValues,
					&newColumns,
					compleUpdateMeta,
				)
				if err != nil {
					return nil, nil, nil, 0, nil, nil, err
				}
			}
		} else {
			newColumns = append(newColumns, column)
			newValues = append(newValues, values[i].Contents)
			if utilities.KeyExistsInList(column.Name, primaryKeys) {
				val, _ := utilities.DecodeEncodedValues(values[i].Contents, column.CQLType, protocolV)
				unencrypted[column.Name] = val
			}
		}
		indexEnd = i
	}

	return newColumns, newValues, unencrypted, indexEnd, delColumnFamily, delColumns, nil
}

// generateRowKey(): generates a row key based on the primary keys of a given table and their corresponding values.
// This function works by first retrieving the primary keys for the specified table using the getPrimaryKeys function.
// It then iterates over the primary keys, checking if each key exists in the unencrypted map. If the key exists,
// the corresponding value is converted to a string and added to a slice. Finally, the slice of primary key values
// is joined into a single string, with each value separated by a "#" delimiter, and returned as the row key.
//
// Parameters:
// - schemaMapping: A pointer to the SchemaMappingConfig structure that holds metadata about the table, including primary keys.
// - tableName: The name of the table for which the row key is being generated.
// - unencrypted: A map containing the values of the primary keys. The keys of this map are the column names, and the values are the corresponding data.
//
// Returns:
// - A string representing the generated row key, which is a concatenation of the primary key values.
// - An error if the primary keys cannot be retrieved or any other issue occurs.
func generateRowKey(schemaMapping *schemaMapping.SchemaMappingConfig, tableName string, unencrypted map[string]interface{}, keySpace string) (string, error) {
	primaryKeys, err := getPrimaryKeys(schemaMapping, tableName, keySpace)
	if err != nil {
		return "", err
	}
	var primkeyvalues []string
	for _, key := range primaryKeys {
		if value, exists := unencrypted[key]; exists {
			v := fmt.Sprintf("%v", value)
			primkeyvalues = append(primkeyvalues, v)
		}
	}
	rowKey := strings.Join(primkeyvalues[:], "#")
	return rowKey, nil
}

// NewCqlParser(): Funtion to create lexer and parser object for the cql query
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

// buildWhereClause(): takes a slice of Clause structs and returns a string representing the WHERE clause of a bigtable SQL query.
// It iterates over the clauses and constructs the WHERE clause by combining the column name, operator, and value of each clause.
// If the operator is "IN", the value is wrapped with the UNNEST function.
// The constructed WHERE clause is returned as a string.
func buildWhereClause(clauses []Clause, t *Translator, tableName string, keySpace string) (string, error) {
	whereClause := ""
	columnFamily := t.SchemaMappingConfig.SystemColumnFamily
	for _, val := range clauses {
		column := "`" + val.Column + "`"
		value := val.Value
		if colMeta, ok := t.SchemaMappingConfig.TablesMetaData[keySpace][tableName][val.Column]; ok {
			// Check if the column is a primitive type and prepend the column family
			if !colMeta.IsCollection {
				var castErr error
				column, castErr = castColumns(colMeta, columnFamily)
				if castErr != nil {
					return "nil", castErr
				}

			}
			// else {
			// TODO:handle collection type of columns in where clause
			// }
		}

		if whereClause != "" {
			whereClause += " AND "
		}

		if val.Operator == "IN" {
			whereClause += fmt.Sprintf("%s IN UNNEST(%s)", column, val.Value)
		} else {
			whereClause += fmt.Sprintf("%s %s %s", column, val.Operator, value)
		}
	}

	if whereClause != "" {
		whereClause = " WHERE " + whereClause
	}
	return whereClause, nil
}

// castColumns(): constructs the corresponding Bigtable column reference string.
//
// Parameters:
//   - colMeta: Metadata for the column, including its name and type.
//   - columnFamily: The column family in Bigtable.
//
// Returns:
//   - nc: The Bigtable column reference string.
//   - err: An error if the conversion fails.
func castColumns(colMeta *schemaMapping.Column, columnFamily string) (string, error) {
	var nc string
	switch colMeta.ColumnType {
	case "int":
		nc = fmt.Sprintf("TO_INT32(%s['%s'])", columnFamily, colMeta.ColumnName)
	case "bigint":
		nc = fmt.Sprintf("TO_INT64(%s['%s'])", columnFamily, colMeta.ColumnName)
	case "float":
		nc = fmt.Sprintf("TO_FLOAT64(%s['%s'])", columnFamily, colMeta.ColumnName)
	case "double":
		nc = fmt.Sprintf("TO_FLOAT64(%s['%s'])", columnFamily, colMeta.ColumnName)
	case "boolean":
		nc = fmt.Sprintf("TO_BOOL(%s['%s'])", columnFamily, colMeta.ColumnName)
	case "timestamp":
		nc = fmt.Sprintf("TO_TIME(%s['%s'])", columnFamily, colMeta.ColumnName)
	case "blob":
		nc = fmt.Sprintf("TO_BLOB(%s['%s'])", columnFamily, colMeta.ColumnName)
	case "text":
		nc = fmt.Sprintf("%s['%s']", columnFamily, colMeta.ColumnName)
	default:
		return "", fmt.Errorf("unsupported CQL type: %s", colMeta.ColumnType)
	}
	return nc, nil
}

// parseWhereByClause(): parse Clauses from the Query
//
// Parameters:
//   - input: The Where Spec context from the antlr Parser.
//   - tableName - Table Name
//   - schemaMapping - JSON Config which maintains column and its datatypes info.
//
// Returns: ClauseResponse and an error if any.
func parseWhereByClause(input cql.IWhereSpecContext, tableName string, schemaMapping *schemaMapping.SchemaMappingConfig, keySpace string) (*ClauseResponse, error) {
	if input == nil {
		return nil, errors.New("no input parameters found for clauses")
	}

	elements := input.RelationElements().AllRelationElement()
	if elements == nil {
		return nil, errors.New("no input parameters found for clauses")
	}

	if len(elements) == 0 {
		return &ClauseResponse{}, nil
	}

	var clauses []Clause
	var response ClauseResponse
	var paramKeys []string

	params := make(map[string]interface{})

	for i, val := range elements {
		if val == nil {
			return nil, errors.New("could not parse column object")
		}
		s := strconv.Itoa(i + 1)
		placeholder := "value" + s
		operator := ""
		paramKeys = append(paramKeys, placeholder)
		colObj := val.OBJECT_NAME(0)
		if colObj == nil {
			return nil, errors.New("could not parse column object")
		}
		isInOperator := false

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
		} else {
			return nil, errors.New("no supported operator found")
		}

		colName := colObj.GetText()
		if colName == "" {
			return nil, errors.New("could not parse column name")
		}
		colName = strings.ReplaceAll(colName, literalPlaceholder, "")
		columnType, err := schemaMapping.GetColumnType(tableName, colName, keySpace)
		if err != nil {
			return nil, err
		}
		if columnType != nil && columnType.CQLType != "" {
			if !isInOperator {
				valConst := val.Constant()
				if valConst == nil {
					return nil, errors.New("could not parse value from query for one of the clauses")
				}
				value := val.Constant().GetText()
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

					case "int":
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
					case "bigint":
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
					case "float":
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
					case "double":
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
					case "boolean":
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
					case "blob":
					case "text", "timestamp":
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
				}
			}
			clause := &Clause{
				Column:       colName,
				Operator:     operator,
				Value:        "@" + placeholder,
				IsPrimaryKey: columnType.IsPrimaryKey,
			}
			clauses = append(clauses, *clause)
		}
	}
	response.Clauses = clauses
	response.Params = params
	response.ParamKeys = paramKeys
	return &response, nil
}

// GetColumnFamily(): The function returns the column family for given column, it returns default configured column family for
// primitive columns and the column name as column family for collection type of columns.
func (t *Translator) GetColumnFamily(tableName, columnName string, keySpace string) string {
	if colType, err := t.SchemaMappingConfig.GetColumnType(tableName, columnName, keySpace); err == nil {
		if strings.HasPrefix(colType.CQLType, "map<") {
			return columnName
		} else if strings.HasPrefix(colType.CQLType, "set<") {
			return columnName
		} else if strings.HasPrefix(colType.CQLType, "list<") {
			return columnName
		}
	}
	return t.SchemaMappingConfig.SystemColumnFamily
}

// IsMapKey(): Detects if the key is accessing a map element like map[key]
func IsMapKey(key string) bool {
	return strings.Contains(key, "[") && strings.Contains(key, "]")
}

// ExtractMapKey(): Extracts the map name and key from a string like map[key]
func ExtractMapKey(key string) (mapName, mapKey string) {
	openIdx := strings.Index(key, "[")
	closeIdx := strings.LastIndex(key, "]")
	if openIdx == -1 || closeIdx == -1 || closeIdx <= openIdx {
		return key, ""
	}
	mapName = key[:openIdx]
	mapKey = key[openIdx+1 : closeIdx]
	mapKey = strings.ReplaceAll(mapKey, "'", "")
	return mapName, mapKey
}

// ExtractCFAndQualifiersforMap(): Helper function to extract column family and qualifiers from map -{} stirng
func ExtractCFAndQualifiersforMap(value string) (string, []string, error) {
	splits := strings.Split(value, "-")
	if len(splits) < 2 {
		return "", nil, fmt.Errorf("invalid format")
	}
	columnFamily := strings.TrimSpace(splits[0])
	cleanedString := strings.Trim(splits[1], "{}")
	qualifiers := strings.Split(cleanedString, ",")
	if len(qualifiers) == 0 || columnFamily == "" {
		return "", nil, fmt.Errorf("invalid format")
	}
	return columnFamily, qualifiers, nil
}

// getTimestampValue(): getTTLValue parses Value for USING TIMESTAMP
//
// Parameters:
//   - spec: Using TTL TImestamp Spec Context from antlr parser.
//
// Returns: string as timestamp value and error if any
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

// GetTimestampInfo(): Extracts timestamp information from an insert query and converts it into a Bigtable timestamp
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

// convertToBigtableTimestamp(): Converts a string timestamp value into a Bigtable timestamp.
func convertToBigtableTimestamp(tsValue string, index int32) (TimestampInfo, error) {
	var timestampInfo TimestampInfo
	var ts64 int64
	var err error
	if tsValue != "" && !strings.Contains(tsValue, questionMark) {
		ts64, err = strconv.ParseInt(tsValue, 10, 64)
		if err != nil {
			return timestampInfo, err
		}
	}
	seconds := ts64 / 1_000_000
	microseconds := ts64 % 1_000_000
	t := time.Unix(seconds, microseconds*1_000)
	microsec := t.UnixMicro()
	timestampInfo.Timestamp = bigtable.Timestamp(microsec)
	timestampInfo.HasUsingTimestamp = true
	timestampInfo.Index = index
	return timestampInfo, nil
}

// GetTimestampInfoForRawDelete(): Extracts timestamp information from a raw delete query and converts it into a Bigtable timestamp.
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

// GetTimestampInfoByUpdate(): Extracts timestamp information from an update query and converts it into a Bigtable timestamp.
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

// getTimestampInfoForPrepareQuery(): Extracts timestamp information for a prepared query.
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
	timestampInfo.Timestamp = bigtable.Timestamp(timestamp)
	return timestampInfo, nil
}

// ProcessTimestamp(): Processes timestamp information for an insert query.
func ProcessTimestamp(st *InsertQueryMap, values []*primitive.Value) (TimestampInfo, error) {
	var timestampInfo TimestampInfo
	timestampInfo.HasUsingTimestamp = false
	if st.TimestampInfo.HasUsingTimestamp {
		return getTimestampInfoForPrepareQuery(values, st.TimestampInfo.Index, 0)
	}
	return timestampInfo, nil
}

// ProcessTimestampByUpdate(): Processes timestamp information for an update query.
func ProcessTimestampByUpdate(st *UpdateQueryMap, values []*primitive.Value) (TimestampInfo, []*primitive.Value, error) {
	var timestampInfo TimestampInfo
	var err error
	timestampInfo.HasUsingTimestamp = false
	if st.TimestampInfo.HasUsingTimestamp {
		timestampInfo, err = getTimestampInfoForPrepareQuery(values, st.TimestampInfo.Index, 0)
		if err != nil {
			return timestampInfo, values, err
		}
		values = values[1:]
	}
	return timestampInfo, values, nil
}

// ProcessTimestampByDelete(): Processes timestamp information for a delete query.
func ProcessTimestampByDelete(st *DeleteQueryMap, values []*primitive.Value) (TimestampInfo, []*primitive.Value, error) {
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

// GetAllColumns(): Retrieves all columns for a table from the schema mapping configuration.
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

// processTextMapColumn(): Processes a map column for a CQL query and converts it to the appropriate type.
func processTextMapColumn[V any](
	decodedValue interface{},
	protocolV primitive.ProtocolVersion,
	colFamily string,
	cqlType string,
	newValues *[]interface{},
	newColumns *[]Column,
) error {

	decodedMap := make(map[string]V)
	if pointerMap, ok := decodedValue.(map[*string]*V); ok {
		for k, v := range pointerMap {
			decodedMap[*k] = *v
		}
	} else {
		return fmt.Errorf("decodedValue is not of type map[string]%T", new(V))
	}

	for k, v := range decodedMap {
		c := Column{
			Name:         k,
			ColumnFamily: colFamily,
			CQLType:      cqlType,
		}
		valueStr, err := primitivesToString(v)
		if err != nil {
			return fmt.Errorf("failed to convert value for key '%s': %w", k, err)
		}

		val, _ := formatValues(valueStr, cqlType, protocolV)
		*newValues = append(*newValues, val)
		*newColumns = append(*newColumns, c)
	}
	return nil
}

// processTimestampMapColumn(): Processes a timestamp map column for a CQL query and converts it to the appropriate type.
func processTimestampMapColumn[V any](
	decodedValue interface{},
	protocolV primitive.ProtocolVersion,
	colFamily string,
	cqlType string,
	newValues *[]interface{},
	newColumns *[]Column,
) error {
	decodedMap := make(map[int64]V)
	if pointerMap, ok := decodedValue.(map[time.Time]V); ok {
		for k, v := range pointerMap {
			key := k.UnixMilli()
			decodedMap[key] = v
		}
	}
	for k, v := range decodedMap {
		c := Column{
			Name:         strconv.FormatInt(k, 10),
			ColumnFamily: colFamily,
			CQLType:      cqlType,
		}

		valueStr, err := primitivesToString(v)
		if err != nil {
			return fmt.Errorf("failed to convert value for key '%d': %w", k, err)
		}
		val, _ := formatValues(valueStr, cqlType, protocolV)
		*newValues = append(*newValues, val)
		*newColumns = append(*newColumns, c)
	}
	return nil
}

// processSetColumnGeneric(): Processes a set column for a CQL query and converts it to the appropriate type.
func processSetColumnGeneric[T any](
	decodedValue interface{},
	protocolV primitive.ProtocolVersion,
	colFamily string,
	cqlType string,
	newValues *[]interface{},
	newColumns *[]Column,
) error {

	// Assert the type of the decoded value
	setValue, ok := decodedValue.([]T)
	if !ok {
		return fmt.Errorf("unexpected type %T for set<%T>", decodedValue, new(T))
	}
	// Iterate over the elements in the set
	for _, elem := range setValue {
		valueStr, err := primitivesToString(elem)
		if err != nil {
			return fmt.Errorf("failed to convert element to string: %w", err)
		}
		c := Column{
			Name:         valueStr,
			ColumnFamily: colFamily,
			CQLType:      cqlType,
		}
		// The value parameter is intentionally empty because the column identifier has the value
		val, _ := formatValues("", "text", protocolV)
		*newValues = append(*newValues, val)
		*newColumns = append(*newColumns, c)
	}

	return nil
}

// processListColumnGeneric(): Processes a list column for a CQL query and converts it to the appropriate type.
func processListColumnGeneric[T any](
	decodedValue interface{},
	protocolV primitive.ProtocolVersion,
	colFamily string,
	cqlType string,
	newValues *[]interface{},
	newColumns *[]Column,
	complexMeta *ComplexUpdateMeta,
) error {

	// Assert the type of the decoded value
	listValues, ok := decodedValue.([]T)
	if !ok {
		return fmt.Errorf("unexpected type %T for set<%T>", decodedValue, new(T))
	}

	var listDelete [][]byte
	prepend := false
	// Iterate over the elements in the List
	for i, elem := range listValues {
		valueStr, err := primitivesToString(elem)
		if err != nil {
			return fmt.Errorf("failed to convert element to string: %w", err)
		}
		if complexMeta != nil && complexMeta.ListDelete {
			val, _ := formatValues(valueStr, cqlType, protocolV)
			listDelete = append(listDelete, val)
			continue
		}
		if complexMeta != nil {
			prepend = complexMeta.PrependList

		}

		encTime := getEncodedTimestamp(i, len(listValues), prepend)

		c := Column{
			Name:         string(encTime),
			ColumnFamily: colFamily,
			CQLType:      cqlType,
		}
		// The value parameter is intentionally empty because the column identifier has the value
		val, _ := formatValues(valueStr, cqlType, protocolV)
		*newValues = append(*newValues, val)
		*newColumns = append(*newColumns, c)
	}
	if len(listDelete) > 0 {
		complexMeta.ListDeleteValues = listDelete
	}
	return nil
}

// ValidateRequiredPrimaryKeys(): Validates that the primary keys in the required and actual lists match.
func ValidateRequiredPrimaryKeys(requiredKey []string, actualKey []string) bool {
	// Check if the length of slices is the same
	if len(requiredKey) != len(actualKey) {
		return false
	}

	// Sort both slices
	sort.Strings(requiredKey)
	sort.Strings(actualKey)

	// Compare the sorted slices element by element
	for i := range requiredKey {
		if requiredKey[i] != actualKey[i] {
			return false
		}
	}

	return true
}

// encodeTimestamp(): Encodes a timestamp value into BigEndian octal format.
func encodeTimestamp(millis int64, nanos int32) []byte {
	buf := make([]byte, 12) // 8 bytes for millis + 4 bytes for nanos
	binary.BigEndian.PutUint64(buf[0:8], uint64(millis))
	binary.BigEndian.PutUint32(buf[8:12], uint32(nanos))
	return buf
}

// handleListType(): Handles the processing of list-type columns in a CQL query and prepares them for conversion.
func handleListType(val string, colFamily string, cqlType string, newValues *[]interface{}, newColumns *[]Column, prependCol []string, complexMeta map[string]*ComplexUpdateMeta) error {
	var listValues []string
	formattedStr := preprocessSetString(val)

	// Parse the JSON string to a slice
	if err := json.Unmarshal([]byte(formattedStr), &listValues); err != nil {
		return fmt.Errorf("error converting string to list<%s>: %w", cqlType, err)
	}
	prepend := false
	// check if column is in prepend columns
	if valExistInArr(prependCol, colFamily) {
		prepend = true
	}
	var listDelete [][]byte
	for i, v := range listValues {
		if strings.Contains(v, ":") { // list index operation
			splitV := strings.Split(v, ":")
			v = splitV[1]
			index := splitV[0]
			val, err := formatValues(v, cqlType, primitive.ProtocolVersion4)
			if err != nil {
				return fmt.Errorf("error converting string to list<%s> value: %w", cqlType, err)
			}
			newComplexMeta := ComplexUpdateMeta{
				UpdateListIndex: index,
				Value:           val,
			}
			complexMeta[colFamily] = &newComplexMeta
			continue
		}

		if meta, ok := complexMeta[colFamily]; ok && meta.ListDelete {
			val, _ := formatValues(v, cqlType, primitive.ProtocolVersion4)
			listDelete = append(listDelete, val)
			continue
		}

		// Calculate encoded timestamp
		encTime := getEncodedTimestamp(i, len(listValues), prepend)
		// Create column
		c := Column{
			Name:         string(encTime),
			ColumnFamily: colFamily,
			CQLType:      cqlType,
		}
		// Format the value
		val, err := formatValues(v, cqlType, primitive.ProtocolVersion4)
		if err != nil {
			return fmt.Errorf("error converting string to list<%s> value: %w", cqlType, err)
		}

		// Append to the slices via pointers
		*newValues = append(*newValues, val)
		*newColumns = append(*newColumns, c)
	}
	if len(listDelete) > 0 {
		complexMeta[colFamily].ListDeleteValues = listDelete
	}

	return nil
}

// getEncodedTimestamp(): Generates an encoded timestamp based on the index and length of the list values
func getEncodedTimestamp(index int, totalLength int, prepend bool) []byte {

	now := time.Now().UnixMilli()

	if prepend {
		now = referenceTime - (now - referenceTime)
	}

	nanos := maxNanos - int32(totalLength) + int32(index)
	return encodeTimestamp(now, nanos)
}

// valExistInArr(): Checks if a given value exists in a slice of strings.
func valExistInArr(arr []string, val string) bool {
	for _, v := range arr {
		if v == val {
			return true
		}
	}
	return false
}

// TransformPrepareQueryForPrependList():  Transforms a prepared query for a prepend list operation.
func TransformPrepareQueryForPrependList(query string) (string, []string) {
	// Initialize variables
	var columns []string

	// Use case-insensitive search for ' SET ' and ' WHERE '
	queryUpper := strings.ToUpper(query)
	setIndex := strings.Index(queryUpper, " SET ")
	whereIndex := strings.Index(queryUpper, " WHERE ")

	// If 'SET' or 'WHERE' clauses are not found, return the original query
	if setIndex == -1 || whereIndex == -1 || setIndex >= whereIndex || !strings.Contains(query, "?") {
		return query, columns
	}

	// Extract parts of the query
	beforeSet := query[:setIndex+5] // Include ' SET '
	setClause := query[setIndex+5 : whereIndex]
	afterWhere := query[whereIndex:] // Include ' WHERE ' and the rest

	// Split the SET clause into individual assignments
	assignments := strings.Split(setClause, ",")
	var newAssignments []string

	// Regular expression to match the prepend pattern
	assignRegex := regexp.MustCompile(`(?i)\s*(\w+)\s*=\s*(['"]?\?['"]?)\s*\+\s*(\w+)\s*`)

	for _, assign := range assignments {
		assign = strings.TrimSpace(assign)
		matches := assignRegex.FindStringSubmatch(assign)

		if matches != nil {
			lhs := matches[1]
			questionMark := matches[2]
			rhsVariable := matches[3]

			// Check if LHS and RHS variable names are the same
			if lhs == rhsVariable {
				// Detected a prepend operation, transform it to append
				newAssign := fmt.Sprintf("%s = %s + %s", lhs, lhs, questionMark)
				newAssignments = append(newAssignments, newAssign)
				columns = append(columns, lhs)
				continue
			}
		}
		// If not a prepend operation, keep the original assignment
		newAssignments = append(newAssignments, assign)
	}

	// If no prepend was found, return the original query
	if len(columns) == 0 {
		return query, columns
	}

	// Reconstruct the modified query
	newSetClause := strings.Join(newAssignments, ", ")
	modifiedQuery := beforeSet + newSetClause + afterWhere

	return modifiedQuery, columns
}

// ExtractKeyValue(): ExtractValue extracts the value between curly braces, trims whitespace, and returns it.
// used in extraction of key in complex update of map and list
func ExtractKeyValue(input string) (string, error) {
	// Check if the input string starts with '{' and ends with '}'
	if len(input) < 2 || input[0] != '{' || input[len(input)-1] != '}' {
		return "", fmt.Errorf("invalid input format: expected '{<value>:?}'")
	}
	// Extract the content between the curly braces
	content := input[1 : len(input)-1]
	// Split the content by ':' to get the value part
	parts := strings.SplitN(content, ":", 2)
	if len(parts) == 0 {
		return "", fmt.Errorf("invalid input format: no value found")
	}
	// Extract the value and remove any whitespace
	value := strings.TrimSpace(parts[0])
	return value, nil
}

// ExtractListType(): ExtractValue extracts the value from a string formatted as "list<value>"
func ExtractListType(s string) (value string) {
	// Extract the value between "list<" and ">"
	value = strings.TrimPrefix(s, "list<")
	value = strings.TrimSuffix(value, ">")
	return
}

// ProcessComplexUpdate(): processes complex updates based on column data and value.
func (t *Translator) ProcessComplexUpdate(columns []Column, values []interface{}, tableName, keyspaceName string, prependColumns []string) (map[string]*ComplexUpdateMeta, error) {
	complexMeta := make(map[string]*ComplexUpdateMeta)

	for i, column := range columns {
		valueStr, ok := values[i].(string)
		if !ok || !containsOperationIndicators(valueStr, column.Name) {
			continue
		}

		if t.IsCollection(tableName, column.Name, keyspaceName) {
			meta, err := processColumnUpdate(valueStr, column, prependColumns)
			if err != nil {
				return nil, err
			}
			complexMeta[column.Name] = meta
		}
	}

	return complexMeta, nil
}

// containsOperationIndicators(): checks if the value string contains operation indicators.
func containsOperationIndicators(val, columnName string) bool {
	return strings.Contains(val, columnName) && (strings.Contains(val, "+") || strings.Contains(val, "-"))
}

// processColumnUpdate(): handles the logic to update a collection column.
func processColumnUpdate(val string, column Column, prependColumns []string) (*ComplexUpdateMeta, error) {
	appendOp, prependOp, listDelete := false, false, false
	var updateListIndex, key string
	var expectedDt datatype.DataType
	var err error

	if strings.Contains(val, "+") {
		if strings.Contains(column.CQLType, "map") {
			key, expectedDt, err = processMapUpdate(val, column)
			appendOp = true
		} else if strings.Contains(column.CQLType, "list") {
			prependOp, appendOp, updateListIndex, expectedDt, err = processListUpdate(val, column, prependColumns)
		}
	} else if strings.Contains(val, "-") {
		if strings.Contains(column.CQLType, "map") {
			expectedDt, err = getMapRemoveDatatype(column)
		} else if strings.Contains(column.CQLType, "list") {
			listDelete = true
		}
	}

	if err != nil {
		return nil, fmt.Errorf("invalid format in prepared complex update: %w", err)
	}

	return &ComplexUpdateMeta{
		Append:           appendOp,
		key:              key,
		ExpectedDatatype: expectedDt,
		PrependList:      prependOp,
		UpdateListIndex:  updateListIndex,
		Delete:           strings.Contains(val, "-"),
		ListDelete:       listDelete,
	}, nil
}

// processMapUpdate(): handles map column updates and returns relevant data.
func processMapUpdate(val string, column Column) (string, datatype.DataType, error) {
	if !strings.Contains(val, "{") { // this checks if its a single key update opperation like set map[key]=?
		return "", nil, nil
	}
	splits := strings.Split(val, "+")
	if len(splits) < 2 {
		return "", nil, fmt.Errorf("invalid format")
	}
	key, err := ExtractKeyValue(splits[1])
	if err != nil {
		return "", nil, err
	}
	expectedDt, err := getColumnType("map", column)
	if err != nil {
		return "", nil, err
	}
	return key, expectedDt, nil
}

// getMapRemoveDatatype(): determines the datatype for map removal.
func getMapRemoveDatatype(column Column) (datatype.DataType, error) {
	dts := extractMapTypes(column)
	setStr := fmt.Sprintf("set<%s>", dts[1])
	return utilities.GetCassandraColumnType(setStr)
}

// processListUpdate(): handles list column updates and returns relevant data.
func processListUpdate(val string, column Column, prependColumns []string) (bool, bool, string, datatype.DataType, error) {
	var prependOp, appendOp bool
	var updateIndex string
	var expectedDt datatype.DataType
	var err error

	splits := strings.Split(val, "+")
	if len(splits) < 2 {
		return false, false, "", nil, fmt.Errorf("invalid format")
	}

	if valExistInArr(prependColumns, column.Name) {
		prependOp = true
	} else if strings.Contains(splits[1], "{") {
		updateIndex, err = ExtractKeyValue(splits[1])
		if err != nil {
			return false, false, "", nil, err
		}
		expectedDt, err = getColumnType("list", column)
	} else {
		appendOp = true
	}
	return prependOp, appendOp, updateIndex, expectedDt, err
}

// getColumnType(): extracts types from map or list columns.
func getColumnType(typeName string, column Column) (datatype.DataType, error) {
	dts := extractTypes(typeName, column)
	if len(dts) == 1 {
		return utilities.GetCassandraColumnType(dts[0])
	}
	return utilities.GetCassandraColumnType(dts[1])
}

// extractMapTypes(): splits map types from column CQL type.
func extractMapTypes(column Column) []string {
	dt := strings.Split(column.CQLType, "map")
	cleaned := strings.Trim(dt[1], "<>")
	return strings.Split(cleaned, ",")
}

// extractTypes(): extractTypes splits types from map or list columns.
func extractTypes(typeName string, column Column) []string {
	dt := strings.Split(column.CQLType, typeName)
	cleaned := strings.Trim(dt[1], "<>")
	return strings.Split(cleaned, ",")
}

// ExtractWritetimeValue(): extractValue checks the format of the string and extracts the value-needed part.
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
