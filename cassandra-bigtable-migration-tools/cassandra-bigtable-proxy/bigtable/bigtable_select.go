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

package bigtableclient

import (
	"cloud.google.com/go/bigtable"
	"context"
	"fmt"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/constants"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/responsehandler"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/third_party/datastax/proxycore"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/utilities"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/gocql/gocql"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"time"
)

// ExecutePreparedStatement -  Executes a prepared statement on Bigtable and returns the result.
// Parameters:
//   - ctx: Context for the operation, used for cancellation and deadlines.
//   - query: rh.QueryMetadata containing the query and parameters.
//   - preparedStmt: PreparedStatement object containing the prepared statement.
//
// Returns:
//   - *message.RowsResult: The result of the select statement.
//   - time.Duration: The total elapsed time for the operation.
//   - error: Error if the statement preparation or execution fails.
func (btc *BigtableAdapter) ExecutePreparedStatement(ctx context.Context, query *types.ExecutableSelectQuery) (*message.RowsResult, error) {
	if query.CachedBTPrepare == nil {
		return nil, fmt.Errorf("cannot execute select query because prepared bigtable query is nil")
	}

	params, err := BuildBigtableParams(query)
	if err != nil {
		btc.Logger.Error("Failed to bind parameters", zap.Any("params", params), zap.Error(err))
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	boundStmt, err := query.CachedBTPrepare.Bind(params)
	if err != nil {
		btc.Logger.Error("Failed to bind parameters", zap.Any("params", params), zap.Error(err))
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	var processingErr error
	var rows []types.GoRow
	executeErr := boundStmt.Execute(ctx, func(resultRow bigtable.ResultRow) bool {
		r, convertErr := btc.convertResultRow(resultRow, query) // Call the implemented helper
		if convertErr != nil {
			btc.Logger.Error("Failed to convert result row", zap.Error(convertErr), zap.String("btql", query.TranslatedQuery))
			processingErr = convertErr // Capture the error
			return false               // Stop execution
		}
		rows = append(rows, r)
		return true // Continue processing
	})
	if executeErr != nil {
		btc.Logger.Error("Failed to execute prepared statement", zap.Error(executeErr))
		return nil, fmt.Errorf("failed to execute prepared statement: %w", executeErr)
	}
	if processingErr != nil { // Check for error during row conversion/processing
		return nil, fmt.Errorf("failed during row processing: %w", processingErr)
	}

	return responsehandler.BuildRowsResultResponse(query, rows, query.ProtocolVersion)
}

func (btc *BigtableAdapter) convertResultRow(resultRow bigtable.ResultRow, query *types.ExecutableSelectQuery) (types.GoRow, error) {
	table, err := btc.schemaManager.Schemas().GetTableSchema(query.Keyspace(), query.Table())
	if err != nil {
		return nil, err
	}

	result := make(types.GoRow)
	for i, colMeta := range resultRow.Metadata.Columns {
		var val any
		key := colMeta.Name
		err := resultRow.GetByName(key, &val)
		if err != nil {
			return nil, err
		}

		// all scalars in system column family when "select *" is used
		if scalarsMap, ok := val.(map[string][]uint8); ok && key == string(table.SystemColumnFamily) {
			for k, val := range scalarsMap {
				key, err := decodeBase64(k)
				if err != nil {
					return nil, err
				}
				col, err := table.GetColumn(types.ColumnName(key))
				if err != nil {
					// the column may not exist in the table anymore - this happens when a column is dropped because we don't delete any data
					continue
				}
				goVal, err := decodeBigtableCellValue(col.CQLType, val)
				if err != nil {
					return nil, err
				}
				result[key] = goVal
			}
			continue
		}

		var expectedType types.CqlDataType
		if query.SelectClause.IsStar {
			col, err := table.GetColumn(types.ColumnName(key))
			if err != nil {
				return nil, err
			}
			expectedType = col.CQLType
		} else {
			// use the result name because that applies the correct alias
			key = query.ResultColumnMetadata[i].Name
			// use the selected column because that has the CQLType
			expectedType = query.SelectClause.Columns[i].ResultType
		}

		if _, ok := result[key]; ok {
			return nil, fmt.Errorf("result already set for column `%s`", key)
		}

		if key == "list_text" {
			btc.Logger.Log(zap.InfoLevel, "list_text", zap.Any("value", val))
		}

		goValue, err := rowValueToGoValue(val, expectedType)
		if err != nil {
			return nil, fmt.Errorf("failed to convert result for '%s': %w", key, err)
		}
		result[key] = goValue
	}

	return result, nil
}

func rowValueToGoValue(val any, expectedType types.CqlDataType) (types.GoValue, error) {
	switch v := val.(type) {
	case string:
		goVal, err := utilities.StringToGo(v, expectedType)
		if err != nil {
			return nil, err
		}
		return goVal, nil
	case []byte:
		switch expectedType.Code() {
		case types.UUID:
			return primitive.UUID(v), nil
		case types.TIMEUUID:
			return primitive.UUID(v), nil
		case types.TIMESTAMP:
			u, err := uuid.FromBytes(v)
			if err != nil {
				// If it's not a UUID, maybe it's raw bytes of a timestamp?
				// But in our case it's likely a UUID from totimestamp()
				return v, nil
			}
			if u.Version() == 1 {
				sec, nsec := u.Time().UnixTime()
				return time.Unix(sec, nsec).UTC(), nil
			}
			if u.Version() == 7 {
				return types.GetTimeFromUUIDv7(primitive.UUID(u))
			}
			return v, nil
		case types.BLOB:
			return v, nil
		default:
			return v, nil
		}
	case map[string]*int64:
		// counters are always a column family with a single column with an empty qualifier
		counterValue, ok := v[""]
		if ok {
			return *counterValue, nil
		} else {
			// default the counter to 0
			return int64(0), nil
		}
	case map[string][]uint8:
		if expectedType.Code() == types.LIST {
			lt := expectedType.(*types.ListType)
			decodedMap := make(map[string]types.GoValue)
			for k, listElement := range v {
				key, err := decodeBase64(k)
				if err != nil {
					return nil, err
				}
				goVal, err := decodeBigtableCellValue(lt.ElementType(), listElement)
				if err != nil {
					return nil, err
				}
				decodedMap[key] = goVal
			}
			// we need to sort the results by their decoded column qualifiers, to get the correct list order
			listValues := utilities.CreateKeyOrderedValueSlice(decodedMap)
			return listValues, nil
		} else if expectedType.Code() == types.SET {
			st := expectedType.(*types.SetType)
			var setValues []types.GoValue
			for k := range v {
				key, err := decodeBase64(k)
				if err != nil {
					return nil, err
				}
				goKey, err := utilities.StringToGo(key, st.ElementType())
				if err != nil {
					return nil, err
				}
				setValues = append(setValues, goKey)
			}
			return setValues, nil
		} else if expectedType.Code() == types.MAP {
			mt := expectedType.(*types.MapType)
			mapValue := make(map[types.GoValue]types.GoValue)
			for mapKey, mapVal := range v {
				key, err := decodeBase64(mapKey)
				if err != nil {
					return nil, err
				}
				goKey, err := utilities.StringToGo(key, mt.KeyType())
				if err != nil {
					return nil, err
				}
				goVal, err := decodeBigtableCellValue(mt.ValueType(), mapVal)
				if err != nil {
					return nil, err
				}
				mapValue[goKey] = goVal
			}
			return mapValue, nil
		} else {
			return nil, fmt.Errorf("unhandled collection response type: %s", expectedType.String())
		}
	case [][]byte: //specific case of listType column in select
		lt, ok := expectedType.(*types.ListType)
		if !ok {
			return nil, fmt.Errorf("expected list result type but got %s", expectedType.String())
		}
		var listValues []types.GoValue
		for _, listElement := range v {
			goVal, err := decodeBigtableCellValue(lt.ElementType(), listElement)
			if err != nil {
				return nil, err
			}
			listValues = append(listValues, goVal)
		}
		return listValues, nil
	case int64:
		// special case handling of ints because Bigtable often uses int64 instead of int32
		switch expectedType.Code() {
		case types.BIGINT, types.COUNTER:
			return v, nil
		case types.INT:
			return int32(v), nil
		case types.BOOLEAN:
			return v == 1, nil
		default:
			return nil, fmt.Errorf("unhandled type coersion: %T to %s", v, expectedType.String())
		}
	case float64:
		switch expectedType.Code() {
		case types.FLOAT:
			return float32(v), nil
		case types.DOUBLE:
			return v, nil
		default:
			return nil, fmt.Errorf("unhandled type coersion: %T to %s", v, expectedType.String())
		}
	case float32:
		switch expectedType.Code() {
		case types.FLOAT:
			return v, nil
		case types.DOUBLE:
			return float64(v), nil
		default:
			return nil, fmt.Errorf("unhandled type coersion: %T to %s", v, expectedType.String())
		}
	case time.Time:
		if expectedType.Code() == types.TIMEUUID {
			// This happens for now() which we translate to CURRENT_TIMESTAMP()
			// We must use the time from Bigtable to generate the UUID.
			return primitive.UUID(gocql.UUIDFromTime(v)), nil
		}
		return v, nil
	case nil:
		return nil, nil
	case [16]byte:
		return primitive.UUID(v), nil
	case primitive.UUID:
		return v, nil
	case []*string:
		return v, nil
	default:
		return nil, fmt.Errorf("unsupported Bigtable SQL type  %T", v)
	}

}

func decodeBigtableCellValue(t types.CqlDataType, val []byte) (types.GoValue, error) {
	return proxycore.DecodeType(t.BigtableStorageType().DataType(), constants.BigtableEncodingVersion, val)
}
