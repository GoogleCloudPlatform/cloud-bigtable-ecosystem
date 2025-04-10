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
	"strconv"
	"strings"
	"time"

	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/natefinch/lumberjack"
	"github.com/ollionorg/cassandra-to-bigtable-proxy/collectiondecoder"
	"github.com/ollionorg/cassandra-to-bigtable-proxy/third_party/datastax/proxycore"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	KEY_TYPE_PARTITION  = "partition"
	KEY_TYPE_CLUSTERING = "clustering"
	KEY_TYPE_REGULAR    = "regular"
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
const (
	Info  = "info"
	Debug = "debug"
	Error = "error"
	Warn  = "warn"
)

// IsCollectionDataType() checks if the provided data type is a collection type (list, set, or map).
func IsCollectionDataType(dt datatype.DataType) bool {
	switch dt.GetDataTypeCode() {
	case primitive.DataTypeCodeList, primitive.DataTypeCodeSet, primitive.DataTypeCodeMap:
		return true
	default:
		return false
	}
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
	case primitive.DataTypeCodeBigint:
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
	case Info:
		level.SetLevel(zap.InfoLevel)
	case Debug:
		level.SetLevel(zap.DebugLevel)
	case Error:
		level.SetLevel(zap.ErrorLevel)
	case Warn:
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

	default:
		err = fmt.Errorf("%v - %v", "Unknown Datatype Identified", s)
	}

	return bytes, err
}

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
