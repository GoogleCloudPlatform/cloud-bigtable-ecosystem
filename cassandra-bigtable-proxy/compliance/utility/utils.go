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
	"time"
)

// convertParams converts parameters to the correct Go types based on their expected Cassandra data types
func ConvertParams(params []map[string]interface{}) []interface{} {
	convertedParams := make([]interface{}, len(params))
	for i, param := range params {
		value := param["value"]
		datatype := param["datatype"].(string)

		// Convert value based on datatype
		convertedParams[i] = ConvertValue(value, datatype)
	}
	return convertedParams
}

func toBigIntTimestamp(value interface{}) int64 {
	switch v := value.(type) {
	case string:
		if v == "current" {
			unixTimestamp := time.Now().UnixMicro()
			return unixTimestamp
		} else if v == "future" {
			unixTimestamp := time.Now().Add(time.Hour).UnixMicro()
			return unixTimestamp
		} else if v == "past" {
			unixTimestamp := time.Now().Add(-time.Hour).UnixMicro()
			return unixTimestamp
		} else if ms, err := strconv.ParseInt(v, 10, 64); err == nil {
			return ms
		}

	case int, int64, float64:
		return toInt64(v)
	case time.Time:
		return toInt64(v)
	default:
		return time.Now().UnixMicro()
	}
	return time.Now().UnixMicro()
}

// convertValue converts a value to the corresponding Go type based on the Cassandra datatype.
func ConvertValue(value interface{}, datatype string) interface{} {
	switch datatype {
	case "string":
		return fmt.Sprintf("%v", value)
	case "bigint":
		return toInt64(value)
	case "int":
		return toInt(value)
	case "double":
		return toFloat64(value)
	case "float":
		return toFloat32(value)
	case "boolean":
		return toBool(value)
	case "bigintTimestamp":
		return toBigIntTimestamp(value)
	case "timestamp":
		return toTimestampWrite(value)
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
		return value // Return as-is if no specific conversion is defined
	}
}

// toInt64() converts various types (int, int64, float64, string) to int64.
// If the input is a string, it attempts to parse it as an integer.
// Returns 0 if the conversion fails or the type is unsupported.
func toInt64(value interface{}) int64 {
	switch v := value.(type) {
	case int:
		return int64(v)
	case int64:
		return v
	case float64:
		return int64(v)
	case string:
		if ms, err := strconv.ParseInt(v, 10, 64); err == nil {
			return ms
		}
		return 0
	default:
		return 0
	}
}

// toInt() converts float64 or int to int.
// Returns 0 if the conversion fails or the type is unsupported.
func toInt(value interface{}) int {
	switch v := value.(type) {
	case int:
		return v
	case float64:
		return int(v)
	default:
		return 0
	}
}

// toFloat64() converts input to float64 if possible, returns 0.0 otherwise.
func toFloat64(value interface{}) float64 {
	if v, ok := value.(float64); ok {
		return v
	}
	return 0.0
}

// toFloat32() converts float64 to float32 if possible, returns 0.0 otherwise.
func toFloat32(value interface{}) float32 {
	if v, ok := value.(float64); ok {
		return float32(v)
	}
	return 0.0
}

// toBool() converts input to boolean if the type is bool, returns false otherwise.
func toBool(value interface{}) bool {
	if v, ok := value.(bool); ok {
		return v
	}
	return false
}

// toTimestampWrite() returns a Unix microsecond timestamp based on input.
// - "current" returns the current time.
// - "future" returns the current time + 1 hour.
// - "past" returns the current time - 1 hour.
// If input is an epoch string or int, it returns the parsed timestamp.
// Returns the current timestamp if the input type is unsupported.
func toTimestampWrite(value interface{}) time.Time {
	switch v := value.(type) {
	case string:
		if v == "current" {
			unixTimestamp := time.Now()
			return unixTimestamp
		} else if v == "future" {
			unixTimestamp := time.Now().Add(time.Hour)
			return unixTimestamp
		} else if v == "past" {
			unixTimestamp := time.Now().Add(-time.Hour)
			return unixTimestamp
		} else if ms, err := strconv.ParseInt(v, 10, 64); err == nil {
			seconds := ms / 1_000_000
			microseconds := ms % 1_000_000
			t := time.Unix(seconds, microseconds*1_000)
			return t
		} else if ms, err := time.Parse(v, time.RFC3339); err == nil {
			return ms
		}

	case int, int64, float64:
		seconds := toInt64(v) / 1_000_000
		microseconds := toInt64(v) % 1_000_000
		t := time.Unix(seconds, microseconds*1_000)
		return t
	case time.Time:
		return v
	default:
		return time.Now()
	}
	return time.Now()
}

// toTimestamp() converts input (string, int, float64) to time.Time.
// If the input is invalid or unsupported, returns epoch 0 (Jan 1, 1970).
func toTimestamp(value interface{}) time.Time {
	switch v := value.(type) {
	case string:
		if ms, err := strconv.ParseInt(v, 10, 64); err == nil {
			seconds := ms / 1_000_000
			microseconds := ms % 1_000_000
			t := time.Unix(seconds, microseconds*1_000)
			return t
		}
	case int, int64, float64:
		// If the input is already in epoch milliseconds
		ms := toInt64(v)
		seconds := ms / 1_000_000
		microseconds := ms % 1_000_000
		t := time.Unix(seconds, microseconds*1_000)
		return t
	case time.Time:
		// Convert time.Time to epoch milliseconds
		return time.Unix(v.UnixMicro(), 0)
	default:
		return time.Unix(toInt64(0), 0) // Return 0 for unsupported types
	}
	return time.Unix(toInt64(0), 0)
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

// toMapStringInt() converts map[string]interface{} to map[string]int.
func toMapStringInt(value interface{}) map[string]int {
	result := make(map[string]int)
	if m, ok := value.(map[string]interface{}); ok {
		for k, v := range m {
			result[k] = toInt(v)
		}
	}
	return result
}

// toMapStringInt64() converts map[string]interface{} to map[string]int64.
func toMapStringInt64(value interface{}) map[string]int64 {
	result := make(map[string]int64)
	if m, ok := value.(map[string]interface{}); ok {
		for k, v := range m {
			result[k] = toInt64(v)
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
// and returns a slice of int64, without explicit error handling.
func ConvertCSVToInt64List(input string) []int64 {
	// Split the input string by commas
	stringList := strings.Split(input, ",")

	// Create a slice to hold the int64 values
	int64List := make([]int64, len(stringList))

	// Iterate over the split string elements
	for i, str := range stringList {
		// Trim any potential whitespace around the string
		str = strings.TrimSpace(str)

		// Convert the string to an int64; assume the conversion is always valid
		num, _ := strconv.ParseInt(str, 10, 64)

		// Assign the int64 to the slice
		int64List[i] = num
	}

	return int64List
}

func ConvertCSVToIntList(input string) []int64 {
	// Split the input string by commas
	stringList := strings.Split(input, ",")

	// Create a slice to hold the int64 values
	intList := make([]int64, len(stringList))

	// Iterate over the split string elements
	for i, str := range stringList {
		// Trim any potential whitespace around the string
		str = strings.TrimSpace(str)

		// Convert the string to an int64; assume the conversion is always valid
		num, _ := strconv.ParseInt(str, 10, 32)

		// Assign the int64 to the slice
		intList[i] = num
	}

	return intList
}

func ConvertCSVToFloat64List(input string) []float64 {
	// Split the input string into substrings
	stringList := strings.Split(input, ",")
	// Allocate a slice for float64 values
	floatList := make([]float64, len(stringList))
	// Loop over each string element
	for i, str := range stringList {
		// Trim whitespace
		str = strings.TrimSpace(str)
		// Attempt to convert the string to float64
		num, err := strconv.ParseFloat(str, 64)
		if err != nil {
			fmt.Printf("Error converting '%s' to float64: %v\n", str, err)
		}
		// Store the float64 in the slice
		floatList[i] = num
	}
	return floatList
}

func ConvertCSVToFloat32List(input string) []float32 {
	// Split the input string into substrings by the comma delimiter
	stringList := strings.Split(input, ",")
	// Allocate a slice for float32 values
	floatList := make([]float32, len(stringList))
	// Loop over each string element
	for i, str := range stringList {
		// Trim any whitespace from the string
		str = strings.TrimSpace(str)
		// Convert the string to a float32 directly with ParseFloat; we still need float64 as type but specify 32 bits for float size
		num, err := strconv.ParseFloat(str, 32)
		if err != nil {
			// Handle errors in conversion, e.g., logging or setting a default value
			fmt.Printf("Error converting '%s' to float32: %v\n", str, err)
			continue // You may want to handle this differently depending on your needs
		}
		// Assign the converted number (type-casted to float32) to the slice
		floatList[i] = float32(num)
	}
	return floatList
}

// toMapStringBool() converts map[string]interface{} to map[string]bool.
func toMapStringBool(value interface{}) map[string]bool {
	result := make(map[string]bool)
	if m, ok := value.(map[string]interface{}); ok {
		for k, v := range m {
			result[k] = toBool(v)
		}
	}
	return result
}

// toMapStringFloat32() converts map[string]interface{} to map[string]float32.
func toMapStringFloat32(value interface{}) map[string]float32 {
	result := make(map[string]float32)
	if m, ok := value.(map[string]interface{}); ok {
		for k, v := range m {
			result[k] = toFloat32(v)
		}
	}
	return result
}

// toMapStringFloat64() converts map[string]interface{} to map[string]float64.
// If the value is not convertible to float64, it defaults to 0.0.
func toMapStringFloat64(value interface{}) map[string]float64 {
	result := make(map[string]float64)
	if m, ok := value.(map[string]interface{}); ok {
		for k, v := range m {
			result[k] = toFloat64(v)
		}
	}
	return result
}

// toMapStringTimestamp() converts map[string]interface{} to map[string]time.Time.
// Values are parsed as timestamps using toTimestamp().
func toMapStringTimestamp(value interface{}) map[string]time.Time {
	result := make(map[string]time.Time)
	if m, ok := value.(map[string]interface{}); ok {
		for k, v := range m {
			timestamp := toTimestamp(v)
			result[k] = timestamp
		}
	}
	return result
}

// toMapTimestampString() converts map[string]interface{} to map[time.Time]string.
// Keys are parsed as Unix timestamps (in seconds) and converted to time.Time.
func toMapTimestampString(value interface{}) map[time.Time]string {
	result := make(map[time.Time]string)
	if m, ok := value.(map[string]interface{}); ok {
		for k, v := range m {
			unixTimestamp, _ := strconv.ParseInt(k, 10, 64)
			parsedTime := time.Unix(unixTimestamp, 0).UTC()
			result[parsedTime] = fmt.Sprintf("%v", v)
		}
	}
	return result
}

// toMapTimestampBool() converts map[string]interface{} to map[time.Time]bool.
// Keys are parsed as Unix timestamps, and values are converted to bool.
func toMapTimestampBool(value interface{}) map[time.Time]bool {
	result := make(map[time.Time]bool)
	if m, ok := value.(map[string]interface{}); ok {
		for k, v := range m {
			unixTimestamp, _ := strconv.ParseInt(k, 10, 64)
			parsedTime := time.Unix(unixTimestamp, 0).UTC()
			result[parsedTime] = toBool(v)
		}
	}
	return result
}

// toMapTimestampFloat32() converts map[string]interface{} to map[time.Time]float32.
// Keys are parsed as Unix timestamps, and values are converted to float32.
func toMapTimestampFloat32(value interface{}) map[time.Time]float32 {
	result := make(map[time.Time]float32)
	if m, ok := value.(map[string]interface{}); ok {
		for k, v := range m {
			unixTimestamp, _ := strconv.ParseInt(k, 10, 64)
			parsedTime := time.Unix(unixTimestamp, 0).UTC()
			result[parsedTime] = toFloat32(v)
		}
	}
	return result
}

// toMapTimestampFloat64() converts map[string]interface{} to map[time.Time]float64.
// Keys are parsed as Unix timestamps, and values are converted to float64.
func toMapTimestampFloat64(value interface{}) map[time.Time]float64 {
	result := make(map[time.Time]float64)
	if m, ok := value.(map[string]interface{}); ok {
		for k, v := range m {
			unixTimestamp, _ := strconv.ParseInt(k, 10, 64)
			parsedTime := time.Unix(unixTimestamp, 0).UTC()
			result[parsedTime] = toFloat64(v)
		}
	}
	return result
}

// toMapTimestampInt() converts map[string]interface{} to map[time.Time]int.
// Keys are parsed as Unix timestamps, and values are converted to int.
func toMapTimestampInt(value interface{}) map[time.Time]int {
	result := make(map[time.Time]int)
	if m, ok := value.(map[string]interface{}); ok {
		for k, v := range m {
			unixTimestamp, _ := strconv.ParseInt(k, 10, 64)
			parsedTime := time.Unix(unixTimestamp, 0).UTC()
			result[parsedTime] = toInt(v)
		}
	}
	return result
}

// toMapTimestampInt64() converts map[string]interface{} to map[time.Time]int64.
// Keys are parsed as Unix timestamps, and values are converted to int64.
func toMapTimestampInt64(value interface{}) map[time.Time]int64 {
	result := make(map[time.Time]int64)
	if m, ok := value.(map[string]interface{}); ok {
		for k, v := range m {
			unixTimestamp, _ := strconv.ParseInt(k, 10, 64)
			parsedTime := time.Unix(unixTimestamp, 0).UTC()
			result[parsedTime] = toInt64(v)
		}
	}
	return result
}

// toMapTimestampTimestamp() converts map[string]interface{} to map[time.Time]time.Time.
// Both keys and values are parsed as Unix timestamps and converted to time.Time.
func toMapTimestampTimestamp(value interface{}) map[time.Time]time.Time {
	result := make(map[time.Time]time.Time)
	if m, ok := value.(map[string]interface{}); ok {
		for k, v := range m {
			unixTimestamp, _ := strconv.ParseInt(k, 10, 64)
			parsedTime := time.Unix(unixTimestamp, 0).UTC()
			result[parsedTime] = time.Unix(toInt64(v), 0).UTC()
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

// toSetBool() converts []interface{} to []bool by casting each item to bool.
// Returns nil if conversion is not possible.
func toSetBool(value interface{}) []bool {
	if v, ok := value.([]interface{}); ok {
		var result []bool
		for _, item := range v {
			if b, ok := item.(bool); ok {
				result = append(result, b)
			}
		}
		return result
	}
	return nil
}

// toSetInt() converts []interface{} to []int by converting each item using toInt().
// Returns nil if conversion is not possible.
func toSetInt(value interface{}) []int {
	if v, ok := value.([]interface{}); ok {
		var result []int
		for _, item := range v {
			result = append(result, toInt(item))
		}
		return result
	}
	return nil
}

// toSetInt64() converts []interface{} to []int64 by converting each item using toInt64().
// Returns nil if conversion is not possible.
func toSetInt64(value interface{}) []int64 {
	if v, ok := value.([]interface{}); ok {
		var result []int64
		for _, item := range v {
			result = append(result, toInt64(item))
		}
		return result
	}
	return nil
}

// toSetFloat32() converts []interface{} to []float32 by converting each item using toFloat32().
// Returns nil if conversion is not possible.
func toSetFloat32(value interface{}) []float32 {
	if v, ok := value.([]interface{}); ok {
		var result []float32
		for _, item := range v {
			result = append(result, toFloat32(item))
		}
		return result
	}
	return nil
}

// toSetFloat64() converts []interface{} to []float64 by converting each item using toFloat64().
// Returns nil if conversion is not possible.
func toSetFloat64(value interface{}) []float64 {
	if v, ok := value.([]interface{}); ok {
		var result []float64
		for _, item := range v {
			result = append(result, toFloat64(item))
		}
		return result
	}
	return nil
}

// toSetTimestamp() converts []interface{} to []time.Time by converting each item using toTimestamp().
// Returns nil if conversion is not possible.
func toSetTimestamp(value interface{}) []time.Time {
	if v, ok := value.([]interface{}); ok {
		var result []time.Time
		for _, item := range v {
			timestamp_val := toTimestamp(item)
			result = append(result, timestamp_val)
		}
		return result
	}
	return nil
}

// ConvertExpectedResult() processes the expected results from test cases by converting
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
				case "text":
					convertedResult[0][key] = fmt.Sprintf("%v", value) // Convert to string
				case "bigint":
					convertedResult[0][key] = toInt64(value)
				case "int":
					convertedResult[0][key] = toInt(value)
				case "double":
					convertedResult[0][key] = toFloat64(value)
				case "float":
					convertedResult[0][key] = toFloat32(value)
				case "boolean":
					convertedResult[0][key] = toBool(value)
				case "timestamp":
					convertedResult[0][key] = toTimestamp(value)

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
