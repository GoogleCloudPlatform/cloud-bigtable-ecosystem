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
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/natefinch/lumberjack"
	"github.com/ollionorg/cassandra-to-bigtable-proxy/collectiondecoder"
	"github.com/ollionorg/cassandra-to-bigtable-proxy/proxycore"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	CustomWritetime = "writetime_"
)

type LoggerConfig struct {
	OutputType string `yaml:"outputType"`
	Filename   string `yaml:"fileName"`
	MaxSize    int    `yaml:"maxSize"`    // megabytes
	MaxBackups int    `yaml:"maxBackups"` // The value of MaxBackups determines how many previous log files are kept after a new log file is created due to the MaxSize or MaxAge limits.
	MaxAge     int    `yaml:"maxAge"`     // days
	Compress   bool   `yaml:"compress"`   // the rotated log files to be compressed to save disk space.
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

// formatTimestamp formats the Timestamp into time.
func FormatTimestamp(ts int64) (*time.Time, error) {
	if ts == 0 {
		return nil, errors.New("no valid timestamp found")
	}
	var formattedValue time.Time
	const secondsThreshold = int64(1e10)
	const millisecondsThreshold = int64(1e13)
	const microsecondsThreshold = int64(1e16)
	const nanosecondsThreshold = int64(1e18)

	switch {
	case ts < secondsThreshold:

		formattedValue = time.Unix(ts, 0)
	case ts >= secondsThreshold && ts < millisecondsThreshold:
		formattedValue = time.UnixMilli(ts)
	case ts >= millisecondsThreshold && ts < microsecondsThreshold:
		formattedValue = time.UnixMicro(ts)
	case ts >= microsecondsThreshold && ts < nanosecondsThreshold:
		seconds := ts / int64(time.Second)
		// Get the remaining nanoseconds
		nanoseconds := ts % int64(time.Second)
		formattedValue = time.Unix(seconds, nanoseconds)
	default:
		return nil, errors.New("no valid timestamp found")
	}

	return &formattedValue, nil
}

// keyExists checks if a key is present in a list of strings.
func KeyExistsInList(key string, list []string) bool {
	for _, item := range list {
		if item == key {
			return true // Key found
		}
	}
	return false // Key not found
}

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
	switch choice {
	case "text":
		return datatype.Varchar, nil
	case "blob":
		return datatype.Blob, nil
	case "timestamp":
		return datatype.Timestamp, nil
	case "int":
		return datatype.Int, nil
	case "bigint":
		return datatype.Bigint, nil
	case "boolean":
		return datatype.Boolean, nil
	case "uuid":
		return datatype.Uuid, nil
	case "float":
		return datatype.Float, nil
	case "double":
		return datatype.Double, nil
	case "map<text,boolean>":
		return MapOfStrToBool, nil
	case "map<text,int>":
		return MapOfStrToInt, nil
	case "map<text,bigint>":
		return MapOfStrToBigInt, nil
	case "map<text,float>":
		return MapOfStrToFloat, nil
	case "map<text,double>":
		return MapOfStrToDouble, nil
	case "map<text,text>":
		return MapOfStrToStr, nil
	case "map<text,timestamp>":
		return MapOfStrToTime, nil
	case "map<timestamp,text>":
		return MapOfTimeToStr, nil
	case "map<timestamp,boolean>":
		return MapOfTimeToBool, nil
	case "map<timestamp,float>":
		return MapOfTimeToFloat, nil
	case "map<timestamp,double>":
		return MapOfTimeToDouble, nil
	case "map<timestamp,bigint>":
		return MapOfTimeToBigInt, nil
	case "map<timestamp,int>":
		return MapOfTimeToInt, nil
	case "map<timestamp,timestamp>":
		return MapOfTimeToTime, nil
	case "list<text>", "frozen<list<text>>":
		return datatype.NewListType(datatype.Varchar), nil
	case "set<int>", "frozen<set<int>>":
		return SetOfInt, nil
	case "set<bigint>", "frozen<set<bigint>>":
		return SetOfBigInt, nil
	case "set<float>", "frozen<set<float>>":
		return SetOfFloat, nil
	case "set<double>", "frozen<set<double>>":
		return SetOfDouble, nil
	case "set<timestamp>", "frozen<set<timestamp>>":
		return SetOfTimeStamp, nil
	case "set<text>", "frozen<set<text>>":
		return SetOfStr, nil
	case "set<boolean>", "frozen<set<boolean>>":
		return SetOfBool, nil
	case "list<boolean>", "frozen<list<boolean>>":
		return ListOfBool, nil
	case "list<int>", "frozen<list<int>>":
		return ListOfInt, nil
	case "list<bigint>", "frozen<list<bigint>>":
		return ListOfBigInt, nil
	case "list<float>", "frozen<list<float>>":
		return ListOfFloat, nil
	case "list<double>", "frozen<list<double>>":
		return ListOfDouble, nil
	case "list<timestamp>", "frozen<list<timestamp>>":
		return ListOfTimeStamp, nil

	default:
		return nil, fmt.Errorf("%s", "Error in returning column type")
	}
}

// DecodeEncodedValues() Function used to decode the encoded values into its original format
func DecodeEncodedValues(value []byte, cqlType string, protocolV primitive.ProtocolVersion) (interface{}, error) {
	var dt datatype.DataType
	switch cqlType {
	case "int":
		dt = datatype.Int
	case "bigint":
		dt = datatype.Bigint
	case "float":
		dt = datatype.Float
	case "double":
		dt = datatype.Double
	case "boolean":
		dt = datatype.Boolean
	case "timestamp":
		dt = datatype.Timestamp
	case "blob":
		dt = datatype.Blob
	case "text":
		dt = datatype.Varchar
	default:
		return nil, fmt.Errorf("unsupported CQL type: %s", cqlType)
	}

	bd, err := proxycore.DecodeType(dt, protocolV, value)
	if err != nil {
		return nil, fmt.Errorf("error encoding value: %w", err)
	}

	return bd, nil
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
func DecodeBytesToCassandraColumnType(b []byte, choice datatype.PrimitiveType, protocolVersion primitive.ProtocolVersion) (interface{}, error) {
	switch choice.GetDataTypeCode() {
	case primitive.DataTypeCodeVarchar:
		return proxycore.DecodeType(datatype.Varchar, protocolVersion, b)
	case primitive.DataTypeCodeDouble:
		return proxycore.DecodeType(datatype.Double, protocolVersion, b)
	case primitive.DataTypeCodeFloat:
		bytes, err := proxycore.DecodeType(datatype.Float, protocolVersion, b)
		// casting result to float64 as float32 not supported by cassandra
		if err != nil {
			return nil, err
		}
		if res, ok := bytes.(float32); ok {
			float32Str := fmt.Sprintf("%f", res)
			// Convert string to float64
			f64, err := strconv.ParseFloat(float32Str, 64)
			if err != nil {
				return nil, fmt.Errorf("error converting string to float64: %s", err)
			}
			return float32(f64), err
		} else {
			return nil, fmt.Errorf("error while casting to float64")
		}
	case primitive.DataTypeCodeBigint:
		return proxycore.DecodeType(datatype.Bigint, protocolVersion, b)
	case primitive.DataTypeCodeTimestamp:
		return proxycore.DecodeType(datatype.Timestamp, protocolVersion, b)
	case primitive.DataTypeCodeInt:
		bytes, err := proxycore.DecodeType(datatype.Int, protocolVersion, b)
		// casting result to int64 as int32 not supported by cassandra
		if err != nil {
			return nil, err
		}
		if res, ok := bytes.(int32); ok {
			return int64(res), err
		} else {
			return nil, fmt.Errorf("error while casting to int64")
		}
	case primitive.DataTypeCodeBoolean:
		return proxycore.DecodeType(datatype.Boolean, protocolVersion, b)
	case primitive.DataTypeCodeUuid:
		decodedUuid, err := proxycore.DecodeType(datatype.Uuid, protocolVersion, b)
		if err != nil {
			return nil, err
		}
		if primitiveUuid, ok := decodedUuid.(primitive.UUID); ok {
			fmt.Printf("primitiveUuid.String() --- > %s", primitiveUuid.String())
			return primitiveUuid.String(), err
		}
		return nil, fmt.Errorf("unable to decode uuid")
	case primitive.DataTypeCodeDate:
		return proxycore.DecodeType(datatype.Date, protocolVersion, b)
	case primitive.DataTypeCodeBlob:
		return proxycore.DecodeType(datatype.Blob, protocolVersion, b)
	default:
		res, err := decodeNonPrimitive(choice, b)
		return res, err
	}
}

// decodeNonPrimitive() Decodes non-primitive types like list, list, and list from byte data based on the provided datatype choice. Returns the decoded collection or an error if unsupported.
func decodeNonPrimitive(choice datatype.PrimitiveType, b []byte) (interface{}, error) {
	var err error
	switch choice.String() {
	case "list<bigint>":
		decodedList, err := collectiondecoder.DecodeCollection(datatype.NewListType(datatype.Bigint), primitive.ProtocolVersion4, b)
		if err != nil {
			fmt.Println("Error decoding list:", err)
			return nil, err
		}
		return decodedList, err
	case "list<varchar>":
		decodedList, err := collectiondecoder.DecodeCollection(datatype.NewListType(datatype.Varchar), primitive.ProtocolVersion4, b)
		if err != nil {
			fmt.Println("Error decoding list:", err)
			return nil, err
		}
		return decodedList, err
	case "list<double>":
		decodedList, err := collectiondecoder.DecodeCollection(datatype.NewListType(datatype.Double), primitive.ProtocolVersion4, b)
		if err != nil {
			fmt.Println("Error decoding list:", err)
			return nil, err
		}
		return decodedList, err
	default:
		err = fmt.Errorf("unsupported Datatype to decode - %s", choice)
		return nil, err
	}
}

// SetupLogger() initializes a zap.Logger instance based on the provided log level and logger configuration.
// If loggerConfig specifies file output, it sets up a file-based logger. Otherwise, it defaults to console output.
// Returns the configured zap.Logger or an error if setup fails.
func SetupLogger(logLevel string, loggerConfig *LoggerConfig) (*zap.Logger, error) {
	level := getLogLevel(logLevel)

	if loggerConfig != nil && loggerConfig.OutputType == "file" {
		return setupFileLogger(level, loggerConfig)
	}

	return setupConsoleLogger(level)
}

// getLogLevel() translates a string log level to a zap.AtomicLevel.
// Supports "info", "debug", "error", and "warn" levels, defaulting to "info" if an unrecognized level is provided.
func getLogLevel(logLevel string) zap.AtomicLevel {
	level := zap.NewAtomicLevel()

	switch logLevel {
	case "info":
		level.SetLevel(zap.InfoLevel)
	case "debug":
		level.SetLevel(zap.DebugLevel)
	case "error":
		level.SetLevel(zap.ErrorLevel)
	case "warn":
		level.SetLevel(zap.WarnLevel)
	default:
		level.SetLevel(zap.InfoLevel)
	}

	return level
}

// setupFileLogger() configures a zap.Logger for file output using a lumberjack.Logger for log rotation.
// Accepts a zap.AtomicLevel and a LoggerConfig struct to customize log output and rotation settings.
// Returns the configured zap.Logger or an error if setup fails.
func setupFileLogger(level zap.AtomicLevel, loggerConfig *LoggerConfig) (*zap.Logger, error) {
	rotationalLogger := &lumberjack.Logger{
		Filename:   defaultIfEmpty(loggerConfig.Filename, "/var/log/cassandra-to-spanner-proxy/output.log"),
		MaxSize:    loggerConfig.MaxSize,                       // megabytes, default 100MB
		MaxAge:     defaultIfZero(loggerConfig.MaxAge, 3),      // setting default value to 3 days
		MaxBackups: defaultIfZero(loggerConfig.MaxBackups, 10), // setting default max backups to 10 files
		Compress:   loggerConfig.Compress,
	}

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
		zapcore.AddSync(rotationalLogger),
		level,
	)

	return zap.New(core), nil
}

// setupConsoleLogger() configures a zap.Logger for console output.
// Accepts a zap.AtomicLevel to set the logging level.
// Returns the configured zap.Logger or an error if setup fails.
func setupConsoleLogger(level zap.AtomicLevel) (*zap.Logger, error) {
	config := zap.Config{
		Encoding:         "json", // or "console"
		Level:            level,  // default log level
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "time",
			CallerKey:      "caller",
			LevelKey:       "level",
			NameKey:        "logger",
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder, // or zapcore.LowercaseColorLevelEncoder for console
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.StringDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
	}

	return config.Build()
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
func TypeConversion(s interface{}, protocalV primitive.ProtocolVersion) ([]byte, error) {
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

	default:
		err = fmt.Errorf("%v - %v", "Unknown Datatype Identified", s)
	}

	return bytes, err
}
