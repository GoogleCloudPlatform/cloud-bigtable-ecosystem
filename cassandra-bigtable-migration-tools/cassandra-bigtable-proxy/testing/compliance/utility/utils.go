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

package utility

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"
)

// ConversionError represents an error that occurs during type conversion
type ConversionError struct {
	Value     interface{}
	Type      string
	Operation string
	Err       error
}

func (e *ConversionError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("failed to convert value '%v' to %s: %v", e.Value, e.Type, e.Err)
	}
	return fmt.Sprintf("failed to convert value '%v' to %s: unsupported type %T", e.Value, e.Type, e.Value)
}

// convertParams converts parameters to the correct Go types based on their expected Cassandra data types
func ConvertParams(t *testing.T, params []map[string]interface{}, fileName, query string) []interface{} {
	convertedParams := make([]any, len(params))
	for i, param := range params {
		// Convert value based on datatype
		convertedParams[i] = ConvertValue(t, param, fileName, query)
	}
	return convertedParams
}

// toBigIntTimestamp converts a value to a Unix microsecond timestamp
func toBigIntTimestamp(value interface{}) (int64, error) {
	switch v := value.(type) {
	case string:
		if v == "current" {
			return time.Now().UnixMicro(), nil
		} else if v == "future" {
			return time.Now().Add(time.Hour).UnixMicro(), nil
		} else if v == "past" {
			return time.Now().Add(-time.Hour).UnixMicro(), nil
		} else if ms, err := strconv.ParseInt(v, 10, 64); err == nil {
			return ms, nil
		} else {
			return 0, &ConversionError{Value: value, Type: "bigint timestamp", Operation: "parse string", Err: err}
		}
	case int, int64, float64:
		if result, err := toInt64(v); err == nil {
			return result, nil
		} else {
			return 0, &ConversionError{Value: value, Type: "bigint timestamp", Operation: "convert number", Err: err}
		}
	case time.Time:
		return v.UnixMicro(), nil
	default:
		return 0, &ConversionError{Value: value, Type: "bigint timestamp", Operation: "type conversion"}
	}
}

// ConvertValue converts a value to the corresponding Go type based on the Cassandra datatype
func ConvertValue(t *testing.T, param map[string]any, fileName, query string) any {
	value := param["value"]
	datatype := param["datatype"].(string)

	switch datatype {
	case "string", "text", "varchar":
		return fmt.Sprintf("%v", value)
	case "bigint":
		result, err := toInt64(value)
		if err != nil {
			LogTestFatal(t, fmt.Sprintf("Error in file '%s' for query '%s': %v", fileName, query, err))
			return nil
		}
		return result
	case "int":
		result, err := toInt(value)
		if err != nil {
			LogTestFatal(t, fmt.Sprintf("Error in file '%s' for query '%s': %v", fileName, query, err))
			return nil
		}
		return result
	case "double":
		result, err := toFloat64(value)
		if err != nil {
			LogTestFatal(t, fmt.Sprintf("Error in file '%s' for query '%s': %v", fileName, query, err))
			return nil
		}
		return result
	case "float":
		result, err := toFloat32(value)
		if err != nil {
			LogTestFatal(t, fmt.Sprintf("Error in file '%s' for query '%s': %v", fileName, query, err))
			return nil
		}
		return result
	case "boolean":
		result, err := toBool(value)
		if err != nil {
			LogTestFatal(t, fmt.Sprintf("Error in file '%s' for query '%s': %v", fileName, query, err))
			return nil
		}
		return result
	case "bigintTimestamp":
		result, err := toBigIntTimestamp(value)
		if err != nil {
			LogTestFatal(t, fmt.Sprintf("Error in file '%s' for query '%s': %v", fileName, query, err))
			return nil
		}
		return result
	case "timestamp":
		result, err := toTimestamp(value)
		if err != nil {
			LogTestFatal(t, fmt.Sprintf("Error in file '%s' for query '%s': %v", fileName, query, err))
			return nil
		}
		return result
	case "map<text,text>":
		return toMapStringString(value)
	case "map<text,int>":
		return toMapStringInt(value)
	case "map<text,bigint>":
		return toMapStringInt64(value)
	case "map<text,boolean>":
		return toMapStringBool(value)
	case "map<text,timestamp>":
		return toMapStringTimestamp(value)
	case "map<text,float>":
		return toMapStringFloat32(value)
	case "map<text,double>":
		return toMapStringFloat64(value)
	case "map<timestamp,text>":
		return toMapTimestampString(value)
	case "map<timestamp,boolean>":
		return toMapTimestampBool(value)
	case "map<timestamp,float>":
		return toMapTimestampFloat32(value)
	case "map<timestamp,double>":
		return toMapTimestampFloat64(value)
	case "map<timestamp,bigint>":
		return toMapTimestampInt64(value)
	case "map<timestamp,timestamp>":
		return toMapTimestampTimestamp(value)
	case "map<timestamp,int>":
		return toMapTimestampInt(value)
	case "set<text>":
		return toSetString(value)
	case "set<boolean>":
		return toSetBool(value)
	case "set<int>":
		return toSetInt(value)
	case "set<bigint>":
		return toSetInt64(value)
	case "set<float>":
		return toSetFloat32(value)
	case "set<double>":
		return toSetFloat64(value)
	case "set<timestamp>":
		return toSetTimestamp(value)
	case "list<text>":
		return toSetString(value)
	case "list<boolean>":
		return toSetBool(value)
	case "list<int>":
		return toSetInt(value)
	case "list<bigint>":
		return toSetInt64(value)
	case "list<float>":
		return toSetFloat32(value)
	case "list<double>":
		return toSetFloat64(value)
	case "list<timestamp>":
		return toSetTimestamp(value)
	case "createlist<text>":
		return ConvertCSVToList(value.(string))
	case "createlist<timestamp>":
		return ConvertCSVToList(value.(string))
	case "createlist<bigint>":
		return ConvertCSVToInt64List(value.(string))
	case "createlist<int>":
		return ConvertCSVToIntList(value.(string))
	case "createlist<float>":
		return ConvertCSVToFloat32List(value.(string))
	case "createlist<double>":
		return ConvertCSVToFloat64List(value.(string))
	default:
		LogTestFatal(t, fmt.Sprintf("There is no valid data, %s, type provided in the `%s` file for the query: `%s`", datatype, fileName, query))
		return nil
	}
}

// toInt64 converts an interface value to int64 and returns an error if conversion fails
func toInt64(value interface{}) (int64, error) {
	switch v := value.(type) {
	case int:
		return int64(v), nil
	case int64:
		return v, nil
	case float64:
		return int64(v), nil
	case string:
		ms, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, &ConversionError{Value: value, Type: "int64", Operation: "parse string", Err: err}
		}
		return ms, nil
	default:
		return 0, &ConversionError{Value: value, Type: "int64", Operation: "type conversion"}
	}
}

// toInt converts an interface value to int and returns an error if conversion fails
func toInt(value interface{}) (int, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case float64:
		return int(v), nil
	case string:
		i, err := strconv.Atoi(v)
		if err != nil {
			return 0, &ConversionError{Value: value, Type: "int", Operation: "parse string", Err: err}
		}
		return i, nil
	default:
		return 0, &ConversionError{Value: value, Type: "int", Operation: "type conversion"}
	}
}

// toFloat64 converts input to float64 and returns an error if conversion fails
func toFloat64(value interface{}) (float64, error) {
	switch v := value.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case string:
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0, &ConversionError{Value: value, Type: "float64", Operation: "parse string", Err: err}
		}
		return f, nil
	default:
		return 0, &ConversionError{Value: value, Type: "float64", Operation: "type conversion"}
	}
}

// toFloat32 converts input to float32 and returns an error if conversion fails
func toFloat32(value interface{}) (float32, error) {
	switch v := value.(type) {
	case float32:
		return v, nil
	case float64:
		return float32(v), nil
	case int:
		return float32(v), nil
	case int64:
		return float32(v), nil
	case string:
		f, err := strconv.ParseFloat(v, 32)
		if err != nil {
			return 0, &ConversionError{Value: value, Type: "float32", Operation: "parse string", Err: err}
		}
		return float32(f), nil
	default:
		return 0, &ConversionError{Value: value, Type: "float32", Operation: "type conversion"}
	}
}

// toBool converts input to boolean and returns an error if conversion fails
func toBool(value interface{}) (bool, error) {
	switch v := value.(type) {
	case bool:
		return v, nil
	case string:
		b, err := strconv.ParseBool(v)
		if err != nil {
			return false, &ConversionError{Value: value, Type: "bool", Operation: "parse string", Err: err}
		}
		return b, nil
	case int:
		return v != 0, nil
	case float64:
		return v != 0, nil
	default:
		return false, &ConversionError{Value: value, Type: "bool", Operation: "type conversion"}
	}
}

// toTimestamp converts input to time.Time
func toTimestamp(value interface{}) (time.Time, error) {
	switch v := value.(type) {
	case string:
		// Try parsing as Unix microseconds first
		if ms, err := strconv.ParseInt(v, 10, 64); err == nil {
			seconds := ms / 1_000_000
			microseconds := ms % 1_000_000
			return time.Unix(seconds, microseconds*1_000).UTC(), nil
		}

		// Try common datetime formats
		formats := []string{
			"2006-01-02 15:04:05",      // YYYY-MM-DD HH:MM:SS
			"2006-01-02T15:04:05Z",     // ISO 8601 UTC
			"2006-01-02T15:04:05",      // ISO 8601 without timezone
			time.RFC3339,               // ISO 8601 with timezone
			"2006-01-02",               // YYYY-MM-DD
			"01/02/2006 15:04:05",      // MM/DD/YYYY HH:MM:SS
			"01/02/2006",               // MM/DD/YYYY
			"02 Jan 2006 15:04:05",     // DD Mon YYYY HH:MM:SS
			"02 Jan 2006",              // DD Mon YYYY
			"Mon Jan 02 15:04:05 2006", // Unix date format
		}

		var lastErr error
		for _, format := range formats {
			if t, err := time.Parse(format, v); err == nil {
				return t.UTC(), nil
			} else {
				lastErr = err
			}
		}
		return time.Time{}, &ConversionError{Value: value, Type: "timestamp", Operation: "parse string", Err: lastErr}

	case int, int64, float64:
		if result, err := toInt64(v); err == nil {
			seconds := result / 1_000_000
			microseconds := result % 1_000_000
			return time.Unix(seconds, microseconds*1_000).UTC(), nil
		} else {
			return time.Time{}, &ConversionError{Value: value, Type: "timestamp", Operation: "convert number", Err: err}
		}
	case time.Time:
		// Ensure the time is in UTC
		return v.UTC(), nil
	default:
		return time.Time{}, &ConversionError{Value: value, Type: "timestamp", Operation: "type conversion"}
	}
}

// toMapStringString() converts map[string]interface{} to map[string]string.
// Non-string values are converted to strings using fmt.Sprintf.
func toMapStringString(value interface{}) map[string]string {
	result := make(map[string]string)
	if m, ok := value.(map[string]interface{}); ok {
		for k, v := range m {
			result[k] = fmt.Sprintf("%v", v)
		}
	}
	return result
}

// toMapStringInt converts map[string]interface{} to map[string]int
func toMapStringInt(value interface{}) map[string]int {
	result := make(map[string]int)
	if m, ok := value.(map[string]interface{}); ok {
		for k, v := range m {
			if intVal, err := toInt(v); err == nil {
				result[k] = intVal
			}
		}
	}
	return result
}

// toMapStringInt64 converts map[string]interface{} to map[string]int64
func toMapStringInt64(value interface{}) map[string]int64 {
	result := make(map[string]int64)
	if m, ok := value.(map[string]interface{}); ok {
		for k, v := range m {
			if int64Val, err := toInt64(v); err == nil {
				result[k] = int64Val
			}
		}
	}
	return result
}

// ConvertCSVToList takes a comma-separated string and returns a slice of strings
func ConvertCSVToList(input string) []string {
	// Use strings.Split to separate the input string by commas
	return strings.Split(input, ",")
}

// ConvertCSVToInt64List takes a comma-separated string of numbers
// and returns a slice of int64, handling conversion errors by skipping invalid values
func ConvertCSVToInt64List(input string) []int64 {
	// Split the input string by commas
	stringList := strings.Split(input, ",")

	// Create a slice to hold the int64 values with initial capacity
	int64List := make([]int64, 0, len(stringList))

	// Iterate over the split string elements
	for _, str := range stringList {
		// Trim any potential whitespace around the string
		str = strings.TrimSpace(str)

		// Convert the string to an int64, skip invalid values
		if num, err := strconv.ParseInt(str, 10, 64); err == nil {
			int64List = append(int64List, num)
		}
	}

	return int64List
}

func ConvertCSVToIntList(input string) []int64 {
	stringList := strings.Split(input, ",")
	intList := make([]int64, 0, len(stringList))

	for _, str := range stringList {
		str = strings.TrimSpace(str)
		if val, err := strconv.ParseInt(str, 10, 32); err == nil {
			intList = append(intList, val)
		}
	}

	return intList
}

// ConvertCSVToFloat64List takes a comma-separated string of numbers
// and returns a slice of float64, handling conversion errors by skipping invalid values
func ConvertCSVToFloat64List(input string) []float64 {
	// Split the input string into substrings
	stringList := strings.Split(input, ",")

	// Allocate a slice for float64 values with initial capacity
	floatList := make([]float64, 0, len(stringList))

	// Loop over each string element
	for _, str := range stringList {
		// Trim whitespace
		str = strings.TrimSpace(str)

		// Attempt to convert the string to float64
		if num, err := strconv.ParseFloat(str, 64); err == nil {
			floatList = append(floatList, num)
		}
	}
	return floatList
}

// ConvertCSVToFloat32List takes a comma-separated string of numbers
// and returns a slice of float32, handling conversion errors by skipping invalid values
func ConvertCSVToFloat32List(input string) []float32 {
	// Split the input string into substrings
	stringList := strings.Split(input, ",")

	// Allocate a slice for float32 values with initial capacity
	floatList := make([]float32, 0, len(stringList))

	// Loop over each string element
	for _, str := range stringList {
		// Trim whitespace
		str = strings.TrimSpace(str)

		// Attempt to convert the string to float32
		if num, err := strconv.ParseFloat(str, 32); err == nil {
			floatList = append(floatList, float32(num))
		}
	}
	return floatList
}

// toMapStringBool converts map[string]interface{} to map[string]bool
func toMapStringBool(value interface{}) map[string]bool {
	result := make(map[string]bool)
	if m, ok := value.(map[string]interface{}); ok {
		for k, v := range m {
			if boolVal, err := toBool(v); err == nil {
				result[k] = boolVal
			}
		}
	}
	return result
}

// toMapStringFloat32 converts map[string]interface{} to map[string]float32
func toMapStringFloat32(value interface{}) map[string]float32 {
	result := make(map[string]float32)
	if m, ok := value.(map[string]interface{}); ok {
		for k, v := range m {
			if floatVal, err := toFloat32(v); err == nil {
				result[k] = floatVal
			}
		}
	}
	return result
}

// toMapStringFloat64 converts map[string]interface{} to map[string]float64
func toMapStringFloat64(value interface{}) map[string]float64 {
	result := make(map[string]float64)
	if m, ok := value.(map[string]interface{}); ok {
		for k, v := range m {
			if floatVal, err := toFloat64(v); err == nil {
				result[k] = floatVal
			}
		}
	}
	return result
}

// toMapStringTimestamp converts map[string]interface{} to map[string]time.Time
func toMapStringTimestamp(value interface{}) map[string]time.Time {
	result := make(map[string]time.Time)
	if m, ok := value.(map[string]interface{}); ok {
		for k, v := range m {
			if timestamp, err := toTimestamp(v); err == nil {
				result[k] = timestamp
			}
		}
	}
	return result
}

// toMapTimestampString converts map[string]interface{} to map[time.Time]string
func toMapTimestampString(value interface{}) map[time.Time]string {
	result := make(map[time.Time]string)
	if m, ok := value.(map[string]interface{}); ok {
		for k, v := range m {
			timestamp, err := toTimestamp(k)
			if err != nil {
				fmt.Println("Error converting timestamp: ", err)
				continue
			}
			result[timestamp.UTC()] = fmt.Sprintf("%v", v)
		}
	}
	return result
}

// toMapTimestampBool converts map[string]interface{} to map[time.Time]bool
func toMapTimestampBool(value interface{}) map[time.Time]bool {
	result := make(map[time.Time]bool)
	if m, ok := value.(map[string]interface{}); ok {
		for k, v := range m {
			timestamp, err := toTimestamp(k)
			if err != nil {
				fmt.Println("Error converting timestamp: ", err)
				continue
			}
			if boolVal, err := toBool(v); err == nil {
				result[timestamp.UTC()] = boolVal
			}
		}
	}
	return result
}

// toMapTimestampFloat32 converts map[string]interface{} to map[time.Time]float32
func toMapTimestampFloat32(value interface{}) map[time.Time]float32 {
	result := make(map[time.Time]float32)
	if m, ok := value.(map[string]interface{}); ok {
		for k, v := range m {
			timestamp, err := toTimestamp(k)
			if err != nil {
				fmt.Println("Error converting timestamp: ", err)
				continue
			}
			if floatVal, err := toFloat32(v); err == nil {
				result[timestamp.UTC()] = floatVal
			}
		}
	}
	return result
}

// toMapTimestampFloat64 converts map[string]interface{} to map[time.Time]float64
func toMapTimestampFloat64(value interface{}) map[time.Time]float64 {
	result := make(map[time.Time]float64)
	if m, ok := value.(map[string]interface{}); ok {
		for k, v := range m {
			timestamp, err := toTimestamp(k)
			if err != nil {
				fmt.Println("Error converting timestamp: ", err)
				continue
			}
			if floatVal, err := toFloat64(v); err == nil {
				result[timestamp.UTC()] = floatVal
			}
		}
	}
	return result
}

// toMapTimestampInt64 converts map[string]interface{} to map[time.Time]int64
func toMapTimestampInt64(value interface{}) map[time.Time]int64 {
	result := make(map[time.Time]int64)
	if m, ok := value.(map[string]interface{}); ok {
		for k, v := range m {
			timestamp, err := toTimestamp(k)
			if err != nil {
				fmt.Println("Error converting timestamp: ", err)
				continue
			}
			if val, err := toInt64(v); err == nil {
				result[timestamp.UTC()] = val
			}
		}
	}
	return result
}

// toMapTimestampInt converts map[string]interface{} to map[time.Time]int
func toMapTimestampInt(value interface{}) map[time.Time]int {
	result := make(map[time.Time]int)
	if m, ok := value.(map[string]interface{}); ok {
		for k, v := range m {
			timestamp, err := toTimestamp(k)
			if err != nil {
				fmt.Println("Error converting timestamp: ", err)
				continue
			}
			if val, err := toInt(v); err == nil {
				result[timestamp.UTC()] = val
			}
		}
	}
	return result
}

// toMapTimestampTimestamp converts map[string]interface{} to map[time.Time]time.Time
func toMapTimestampTimestamp(value interface{}) map[time.Time]time.Time {
	result := make(map[time.Time]time.Time)
	if m, ok := value.(map[string]interface{}); ok {
		for k, v := range m {
			timestamp, err := toTimestamp(k)
			if err != nil {
				fmt.Println("Error converting timestamp: ", err)
				continue
			}
			// Handle empty or zero timestamps
			if v == nil || v == "" || v == 0 {
				result[timestamp.UTC()] = time.Time{}
				continue
			}
			if timestampVal, err := toTimestamp(v); err == nil {
				result[timestamp.UTC()] = timestampVal.UTC()
			}
		}
	}
	return result
}

// toSetString() converts []interface{} to []string by formatting each item as a string.
// Returns nil if conversion is not possible.
func toSetString(value interface{}) []string {
	if v, ok := value.([]interface{}); ok {
		var result []string
		for _, item := range v {
			result = append(result, fmt.Sprintf("%v", item))
		}
		return result
	}
	return nil
}

// toSetBool converts []interface{} to []bool by converting each item using toBool
func toSetBool(value interface{}) []bool {
	if v, ok := value.([]interface{}); ok {
		var result []bool
		for _, item := range v {
			if boolVal, err := toBool(item); err == nil {
				result = append(result, boolVal)
			}
		}
		return result
	}
	return nil
}

// toSetInt() converts []interface{} to []int by converting each item using toInt().
// Returns nil if conversion is not possible.
func toSetInt(value interface{}) []int {
	var result []int
	if items, ok := value.([]interface{}); ok {
		for _, item := range items {
			if val, err := toInt(item); err == nil {
				result = append(result, val)
			}
		}
	}
	return result
}

// toSetInt64() converts []interface{} to []int64 by converting each item using toInt64().
// Returns nil if conversion is not possible.
func toSetInt64(value interface{}) []int64 {
	var result []int64
	if items, ok := value.([]interface{}); ok {
		for _, item := range items {
			if val, err := toInt64(item); err == nil {
				result = append(result, val)
			}
		}
	}
	return result
}

// toSetFloat32 converts []interface{} to []float32 by converting each item using toFloat32
func toSetFloat32(value interface{}) []float32 {
	if v, ok := value.([]interface{}); ok {
		var result []float32
		for _, item := range v {
			if floatVal, err := toFloat32(item); err == nil {
				result = append(result, floatVal)
			}
		}
		return result
	}
	return nil
}

// toSetFloat64 converts []interface{} to []float64 by converting each item using toFloat64
func toSetFloat64(value interface{}) []float64 {
	if v, ok := value.([]interface{}); ok {
		var result []float64
		for _, item := range v {
			if floatVal, err := toFloat64(item); err == nil {
				result = append(result, floatVal)
			}
		}
		return result
	}
	return nil
}

// toSetTimestamp converts []interface{} to []time.Time by converting each item using toTimestamp
func toSetTimestamp(value interface{}) []time.Time {
	if v, ok := value.([]interface{}); ok {
		var result []time.Time
		for _, item := range v {
			if timestamp, err := toTimestamp(item); err == nil {
				result = append(result, timestamp)
			}
		}
		return result
	}
	return nil
}

// ConvertExpectedResult processes the expected results from test cases by converting
// values into appropriate Go types based on the specified 'datatype'.
// It handles primitive types (text, int, bigint, float, double, boolean, timestamp),
// map types (map<text, int>, map<timestamp, text>, etc.), and set types (set<int>, set<text>, etc.).
// This function ensures that the expected result format matches the structure of
// the actual database query results for accurate comparison during testing.
func ConvertExpectedResult(input []map[string]interface{}) []map[string]interface{} {
	if len(input) == 0 || input == nil {
		return nil
	}
	// Output slice
	convertedResult := make([]map[string]interface{}, 1)
	convertedResult[0] = make(map[string]interface{}) // Initialize the first map

	// Iterate through the input and process each field
	for _, field := range input {
		for key, value := range field {
			if key == "datatype" {
				continue // Skip the "datatype" key
			}
			if datatype, exists := field["datatype"].(string); exists {
				// Perform conversion based on datatype
				switch datatype {
				// Primitive Types
				case "text", "varchar":
					convertedResult[0][key] = fmt.Sprintf("%v", value) // Convert to string
				case "bigint":
					if val, err := toInt64(value); err == nil {
						convertedResult[0][key] = val
					}
				case "int":
					if val, err := toInt(value); err == nil {
						convertedResult[0][key] = val
					}
				case "double":
					if val, err := toFloat64(value); err == nil {
						convertedResult[0][key] = val
					}
				case "float":
					if val, err := toFloat32(value); err == nil {
						convertedResult[0][key] = val
					}
				case "boolean":
					if val, err := toBool(value); err == nil {
						convertedResult[0][key] = val
					}
				case "timestamp":
					if val, err := toTimestamp(value); err == nil {
						convertedResult[0][key] = val
					}

				// Map Types
				case "map<text,text>":
					convertedResult[0][key] = toMapStringString(value)
				case "map<text,int>":
					convertedResult[0][key] = toMapStringInt(value)
				case "map<text,bigint>":
					convertedResult[0][key] = toMapStringInt64(value)
				case "map<text,boolean>":
					convertedResult[0][key] = toMapStringBool(value)
				case "map<text,timestamp>":
					convertedResult[0][key] = toMapStringTimestamp(value)
				case "map<text,float>":
					convertedResult[0][key] = toMapStringFloat32(value)
				case "map<text,double>":
					convertedResult[0][key] = toMapStringFloat64(value)
				case "map<timestamp,text>":
					convertedResult[0][key] = toMapTimestampString(value)
				case "map<timestamp,boolean>":
					convertedResult[0][key] = toMapTimestampBool(value)
				case "map<timestamp,float>":
					convertedResult[0][key] = toMapTimestampFloat32(value)
				case "map<timestamp,double>":
					convertedResult[0][key] = toMapTimestampFloat64(value)
				case "map<timestamp,bigint>":
					convertedResult[0][key] = toMapTimestampInt64(value)
				case "map<timestamp,timestamp>":
					convertedResult[0][key] = toMapTimestampTimestamp(value)
				case "map<timestamp,int>":
					convertedResult[0][key] = toMapTimestampInt(value)

				// Set Types
				case "set<text>":
					convertedResult[0][key] = toSetString(value)
				case "set<boolean>":
					convertedResult[0][key] = toSetBool(value)
				case "set<int>":
					convertedResult[0][key] = toSetInt(value)
				case "set<bigint>":
					convertedResult[0][key] = toSetInt64(value)
				case "set<float>":
					convertedResult[0][key] = toSetFloat32(value)
				case "set<double>":
					convertedResult[0][key] = toSetFloat64(value)
				case "set<timestamp>":
					convertedResult[0][key] = toSetTimestamp(value)
				case "list<text>":
					convertedResult[0][key] = toSetString(value)
				case "list<boolean>":
					convertedResult[0][key] = toSetBool(value)
				case "list<int>":
					convertedResult[0][key] = toSetInt(value)
				case "list<bigint>":
					convertedResult[0][key] = toSetInt64(value)
				case "list<float>":
					convertedResult[0][key] = toSetFloat32(value)
				case "list<double>":
					convertedResult[0][key] = toSetFloat64(value)
				case "list<timestamp>":
					convertedResult[0][key] = toSetTimestamp(value)
				}
			}
		}
	}
	return convertedResult
}

// ConvertToMap converts a map[string]interface{} to map[string]interface{} with converted values
func ConvertToMap(t *testing.T, param map[string]any, fileName, query string) map[string]interface{} {
	convertedResult := make([]map[string]interface{}, 1)
	convertedResult[0] = make(map[string]interface{})

	for key, value := range param {
		if strings.HasPrefix(key, "bigint") {
			if val, err := toInt64(value); err == nil {
				convertedResult[0][key] = val
			} else {
				LogTestFatal(t, fmt.Sprintf("Error in file '%s' for query '%s': %v", fileName, query, err))
			}
		} else if strings.HasPrefix(key, "int") {
			if val, err := toInt(value); err == nil {
				convertedResult[0][key] = val
			} else {
				LogTestFatal(t, fmt.Sprintf("Error in file '%s' for query '%s': %v", fileName, query, err))
			}
		} else {
			convertedResult[0][key] = value
		}
	}

	return convertedResult[0]
}
