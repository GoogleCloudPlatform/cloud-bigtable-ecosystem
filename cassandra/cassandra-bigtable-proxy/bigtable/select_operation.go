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
	"io"
	"net/url"
	"reflect"
	"strconv"
	"time"

	btpb "cloud.google.com/go/bigtable/apiv2/bigtablepb"
	"github.com/datastax/go-cassandra-native-protocol/message"
	rh "github.com/ollionorg/cassandra-to-bigtable-proxy/responsehandler"
	"go.uber.org/zap"
	"google.golang.org/grpc/metadata"
)

// SelectStatement - Executes a select statement on Bigtable and returns the result.
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
	var data message.RowSet
	var bigtableEnd time.Time

	rowMap, err := btc.ExecuteBigtableQuery(ctx, query)
	bigtableEnd = time.Now()
	if err != nil {
		return nil, bigtableEnd, err
	}
	columnMetadata, mapKeyArray, err := btc.ResponseHandler.BuildMetadata(rowMap, query)
	if err != nil {
		return nil, bigtableEnd, err
	}

	for i := range len(rowMap) {
		lastRow := i == len(rowMap)-1
		mr, err := btc.ResponseHandler.BuildResponseRow(rowMap[strconv.Itoa(i)], query, columnMetadata, mapKeyArray, lastRow)

		if err != nil {
			return nil, bigtableEnd, err
		}
		if len(mr) != 0 {
			data = append(data, mr)
		}
	}

	if len(columnMetadata) == 0 {
		//TODO Ensure ColumnMetadata  returned is in same order as columns in table
		columnMetadata, err = btc.SchemaMappingConfig.GetMetadataForSelectedColumns(query.TableName, query.SelectedColumns, query.KeyspaceName)
		if err != nil {
			btc.Logger.Error("error while fetching columnMetadata from config -", zap.Error(err))
			return nil, bigtableEnd, err
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

// ExecuteBigtableQuery() - Executes a Bigtable query using the provided context and query metadata.
//
// Parameters:
//   - ctx: Context for the operation, used for cancellation and deadlines.
//   - query: rh.QueryMetadata containing the query and parameters.
//
// Returns:
//   - map[string]map[string]interface{}: Retrieved rows from the Bigtable query mapped by row keys.
//   - error: Error if the query execution fails.
func (btc *BigtableClient) ExecuteBigtableQuery(ctx context.Context, query rh.QueryMetadata) (map[string]map[string]interface{}, error) {

	_, ok := btc.Clients[query.KeyspaceName]
	if !ok {
		return nil, fmt.Errorf("invalid keySpace")
	}
	var instanceName string = fmt.Sprintf("projects/%s/instances/%s", btc.BigtableConfig.GCPProjectID, query.KeyspaceName)
	var appProfileId string = GetProfileId(btc.BigtableConfig.AppProfileID)

	// Construct the x-goog-request-params header
	paramHeaders := fmt.Sprintf("name=%s&app_profile_id=%s", url.QueryEscape(instanceName), appProfileId)
	md := metadata.Pairs("x-goog-request-params", paramHeaders)
	ctxMD := metadata.NewOutgoingContext(ctx, md)

	newParams, err := constructRequestParams(query.Params)
	if err != nil {
		return nil, fmt.Errorf("error constructing params: %v", err)
	}

	req := &btpb.ExecuteQueryRequest{
		InstanceName: instanceName,
		Query:        query.Query,
		DataFormat: &btpb.ExecuteQueryRequest_ProtoFormat{
			ProtoFormat: &btpb.ProtoFormat{},
		},
		Params: newParams,
	}
	btc.Logger.Info("ExecuteQuery", zap.Any("req", req))

	// Call the gRPC method using the context with metadata
	stream, err := btc.SqlClient.ExecuteQuery(ctxMD, req)
	if err != nil {
		return nil, fmt.Errorf("could not execute query: %v", err)
	}

	var rowMapData = make(map[string]map[string]interface{})
	var rowCount = 0
	var cfs []*btpb.ColumnMetadata
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			return rowMapData, nil
		}
		if err != nil {
			return nil, err
		}
		switch r := resp.Response.(type) {
		case *btpb.ExecuteQueryResponse_Metadata:
			cfs = resp.GetMetadata().GetProtoSchema().GetColumns()
		case *btpb.ExecuteQueryResponse_Results:
			rowMapData, err = btc.ResponseHandler.GetRows(r, cfs, query, &rowCount, rowMapData)
			btc.Logger.Info("ExecuteQueryResponse_Results", zap.Any("rowMapData", rowMapData))
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unknown response type")
		}
	}
}

// constructRequestValues - Constructs a btpb.Value based on the type of the input value.
//
// Parameters:
//   - value: interface{} representing the input value.
//
// Returns:
//   - *btpb.Value: Constructed btpb.Value suitable for Bigtable requests.
//   - error: Error if the value type is unsupported.
func constructRequestValues(value interface{}) (*btpb.Value, error) {
	switch v := value.(type) {
	case string:
		return &btpb.Value{
			Kind: &btpb.Value_StringValue{StringValue: v},
			Type: &btpb.Type{Kind: &btpb.Type_StringType{}},
		}, nil
	case int32:
		return &btpb.Value{
			Kind: &btpb.Value_IntValue{IntValue: int64(v)},
			Type: &btpb.Type{Kind: &btpb.Type_Int64Type{}},
		}, nil
	case int64:
		return &btpb.Value{
			Kind: &btpb.Value_IntValue{IntValue: v},
			Type: &btpb.Type{Kind: &btpb.Type_Int64Type{}},
		}, nil
	case float64:
		return &btpb.Value{
			Kind: &btpb.Value_FloatValue{FloatValue: v},
			Type: &btpb.Type{Kind: &btpb.Type_Float64Type{}},
		}, nil
	case float32:
		return &btpb.Value{
			Kind: &btpb.Value_FloatValue{FloatValue: float64(v)},
			Type: &btpb.Type{Kind: &btpb.Type_Float32Type{}},
		}, nil
	default:
		val := reflect.ValueOf(value)

		// Return early if value is not a slice
		if val.Kind() != reflect.Slice {
			return nil, fmt.Errorf("unsupported type: %T", value)
		}

		// Return early if slice is empty
		if val.Len() == 0 {
			return &btpb.Value{
				Kind: &btpb.Value_ArrayValue{ArrayValue: &btpb.ArrayValue{Values: []*btpb.Value{}}},
				Type: &btpb.Type{Kind: &btpb.Type_ArrayType{ArrayType: &btpb.Type_Array{ElementType: nil}}},
			}, nil
		}

		// Process array values
		arrayValues := make([]*btpb.Value, val.Len())
		var elementType *btpb.Type

		for i := 0; i < val.Len(); i++ {
			elem := val.Index(i).Interface()
			btpbValue, err := constructRequestValues(elem)
			if err != nil {
				return nil, fmt.Errorf("unsupported element type in array: %v", err)
			}

			// Ensure homogeneous array
			if elementType == nil {
				elementType = btpbValue.Type
			} else if !reflect.DeepEqual(elementType, btpbValue.Type) {
				return nil, fmt.Errorf("heterogeneous array detected: elements must be of the same type")
			}

			arrayValues[i] = btpbValue
		}

		return &btpb.Value{
			Kind: &btpb.Value_ArrayValue{ArrayValue: &btpb.ArrayValue{Values: arrayValues}},
			Type: &btpb.Type{Kind: &btpb.Type_ArrayType{ArrayType: &btpb.Type_Array{ElementType: elementType}}},
		}, nil
	}
}

// constructRequestParams - Transforms a map of input parameters into a map of btpb.Value suitable for Bigtable requests.
//
// Parameters:
//   - inputParams: map[string]interface{} containing the input parameters.
//
// Returns:
//   - map[string]*btpb.Value: Transformed parameters suitable for Bigtable requests.
//   - error: Error if any value type is unsupported.
func constructRequestParams(inputParams map[string]interface{}) (map[string]*btpb.Value, error) {
	newParams := make(map[string]*btpb.Value)
	for key, value := range inputParams {
		btpbValue, err := constructRequestValues(value)
		if err != nil {
			return nil, fmt.Errorf("unsupported type for key %s: %v", key, err)
		}
		newParams[key] = btpbValue
	}
	return newParams, nil
}
