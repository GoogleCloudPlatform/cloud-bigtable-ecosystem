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
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"cloud.google.com/go/bigtable"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"

	rh "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/responsehandler"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/third_party/datastax/proxycore"
	"go.uber.org/zap"
)

// SelectStatement - Executes a select statement on Bigtable and returns the result.
// It uses the SQL API (Prepare/Execute flow) to execute the statement.
//
// Parameters:
//   - ctx: Context for the operation, used for cancellation and deadlines.
//   - query: rh.QueryMetadata containing the query and parameters.
//
// Returns:
//   - *message.RowsResult: The result of the select statement.
//   - time.Duration: The total elapsed time for the operation.
//   - error: Error if the select statement execution fails.
func (btc *BigtableClient) SelectStatement(ctx context.Context, query rh.QueryMetadata) (*message.RowsResult, time.Time, error) {
	preparedStmt, err := btc.PrepareStatement(ctx, query)
	if err != nil {
		btc.Logger.Error("Failed to prepare statement", zap.String("query", query.Query), zap.Error(err))
		return nil, time.Now(), fmt.Errorf("failed to prepare statement: %w", err)
	}
	return btc.ExecutePreparedStatement(ctx, query, preparedStmt)
}

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
func (btc *BigtableClient) ExecutePreparedStatement(ctx context.Context, query rh.QueryMetadata, preparedStmt *bigtable.PreparedStatement) (*message.RowsResult, time.Time, error) {
	var data message.RowSet
	var bigtableEnd time.Time
	var columnMetadata []*message.ColumnMetadata
	var processingErr error

	boundStmt, err := preparedStmt.Bind(query.Params)
	if err != nil {
		btc.Logger.Error("Failed to bind parameters", zap.Any("params", query.Params), zap.Error(err))
		return nil, time.Now(), fmt.Errorf("failed to bind parameters: %w", err)
	}

	allRowMaps := make([]map[string]interface{}, 0)
	executeErr := boundStmt.Execute(ctx, func(resultRow bigtable.ResultRow) bool {
		rowMap, convertErr := btc.convertResultRowToMap(resultRow, query) // Call the implemented helper
		if convertErr != nil {
			// Log the error and stop processing
			btc.Logger.Error("Failed to convert result row", zap.Error(convertErr), zap.String("btql", query.Query))
			processingErr = convertErr // Capture the error
			return false               // Stop execution
		}
		allRowMaps = append(allRowMaps, rowMap)
		return true // Continue processing
	})
	bigtableEnd = time.Now() // Record time after execution finishes or errors

	if executeErr != nil {
		btc.Logger.Error("Failed to execute prepared statement", zap.Error(executeErr))
		return nil, bigtableEnd, fmt.Errorf("failed to execute prepared statement: %w", executeErr)
	}
	if processingErr != nil { // Check for error during row conversion/processing
		return nil, bigtableEnd, fmt.Errorf("failed during row processing: %w", processingErr)
	}
	columnMetadata, err = btc.ResponseHandler.BuildMetadata(allRowMaps, query)
	if err != nil {
		btc.Logger.Error("Failed to build metadata from results", zap.Error(err))
		return nil, bigtableEnd, fmt.Errorf("failed to build metadata: %w", err)
	}

	for i := 0; i < len(allRowMaps); i++ {
		mr, buildErr := btc.ResponseHandler.BuildResponseRow(allRowMaps[i], query, columnMetadata)
		if buildErr != nil {
			btc.Logger.Error("Failed to build response row", zap.Int("rowIndex", i), zap.Error(buildErr))
			return nil, bigtableEnd, fmt.Errorf("failed to build response row %d: %w", i, buildErr)
		}
		if len(mr) != 0 {
			data = append(data, mr)
		}
	}

	if len(allRowMaps) == 0 && len(columnMetadata) == 0 {
		columnMetadata, err = btc.SchemaMappingConfig.GetMetadataForSelectedColumns(query.TableName, query.SelectedColumns, query.KeyspaceName)
		if err != nil {
			btc.Logger.Error("Error while fetching columnMetadata from config for empty result", zap.Error(err))
			columnMetadata = []*message.ColumnMetadata{}
		}
	}
	result := &message.RowsResult{
		Metadata: &message.RowsMetadata{
			ColumnCount: int32(len(columnMetadata)),
			Columns:     columnMetadata,
		},
		Data: data,
	}
	return result, bigtableEnd, nil
}

// convertResultRowToMap converts a bigtable.ResultRow into the map format expected by the ResponseHandler.
func (btc *BigtableClient) convertResultRowToMap(resultRow bigtable.ResultRow, query rh.QueryMetadata) (map[string]interface{}, error) {
	rowMap := make(map[string]interface{})
	var convertErr error

	// Iterate through the columns defined in the result row metadata
	for i, colMeta := range resultRow.Metadata.Columns {
		var val any
		colName := colMeta.Name
		err := resultRow.GetByName(colName, &val)
		if err != nil {
			return nil, err
		}
		// handle nil value cases
		if val == nil {
			// Skip columns with nil values
			continue
		}
		if strings.Contains(colName, "$col") {
			colName = query.SelectedColumns[i].Name
		}
		switch v := val.(type) {
		case string:
			rowMap[colName] = []byte(v)
		case []byte:
			rowMap[colName] = v
		case map[string]*int64:
			// flatten out the counter column family - we only get here when 'select *' is used
			if colName == btc.SchemaMappingConfig.CounterColumnFamily {
				for key, value := range v {
					decodedKey, err := decodeBase64(key)
					if err != nil {
						return nil, err
					}
					rowMap[decodedKey] = *value
				}
			}
		case map[string][]uint8: //specific case of select * column in select
			if colName == query.DefaultColumnFamily { // default column family e.g cf1
				for k, val := range v {
					key, err := decodeBase64(k)
					if err != nil {
						return nil, err
					}
					rowMap[key] = val
				}
			} else { // for all other collections types data
				newMap := make(map[string]interface{})
				for k, val := range v {
					key, err := decodeBase64(k)
					if err != nil {
						return nil, err
					}
					newMap[key] = val
				}
				keys := make([]string, 0, len(v))
				for key := range newMap {
					keys = append(keys, key)
				}
				// Sort the keys lexographically by comparing the strings
				slices.Sort(keys)
				decodedMap := make([]rh.Maptype, 0)
				for _, key := range keys {
					decodedMap = append(decodedMap, rh.Maptype{Key: key, Value: newMap[key]})
				}
				rowMap[colName] = decodedMap
				continue
			}
		case [][]byte: //specific case of listType column in select
			ListMap := make([]rh.Maptype, 0)
			for _, val := range v {
				ListMap = append(ListMap, rh.Maptype{Key: "", Value: val})
			}
			rowMap[colName] = ListMap
			continue
		case int64, float64:
			rowMap[colName] = v
		case float32:
			rowMap[colName] = float64(v)
		case time.Time:
			encoded, _ := proxycore.EncodeType(datatype.Timestamp, primitive.ProtocolVersion4, v.UnixMicro())
			rowMap[colName] = encoded
		case nil:
			rowMap[colName] = nil
		default:
			convertErr = fmt.Errorf("unsupported Bigtable SQL type  %T for column %s", v, colName)
		}
		if convertErr != nil {
			return nil, convertErr // Return error if conversion failed
		}
	}
	return rowMap, nil
}
