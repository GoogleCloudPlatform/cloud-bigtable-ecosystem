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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"cloud.google.com/go/bigtable"
	"google.golang.org/grpc"

	btpb "cloud.google.com/go/bigtable/apiv2/bigtablepb"
	"github.com/datastax/go-cassandra-native-protocol/message"
	otelgo "github.com/ollionorg/cassandra-to-bigtable-proxy/otel"
	rh "github.com/ollionorg/cassandra-to-bigtable-proxy/responsehandler"
	schemaMapping "github.com/ollionorg/cassandra-to-bigtable-proxy/schema-mapping"
	"github.com/ollionorg/cassandra-to-bigtable-proxy/translator"
	"github.com/ollionorg/cassandra-to-bigtable-proxy/utilities"
	"go.uber.org/zap"
)

// Events
const (
	applyingBigtableMutation       = "Applying Insert/Update Mutation"
	bigtableMutationApplied        = "Insert/Update Mutation Applied"
	callingBigtableSQLAPI          = "Calling Bigtable SQL API"
	bigtableSQLAPICallDone         = "Bigtable SQL API Call Done"
	applyingDeleteMutation         = "Applying Delete Mutation"
	deleteMutationApplied          = "Delete Mutation Applied"
	fetchingSchemaMappingConfig    = "Fetching Schema Mapping Configurations"
	schemaMappingConfigFetched     = "Schema Mapping Configurations Fetched"
	applyingBulkMutation           = "Applying Bulk Mutation"
	bulkMutationApplied            = "Bulk Mutation Applied"
	mutationTypeInsert             = "Insert"
	mutationTypeDelete             = "Delete"
	mutationTypeDeleteColumnFamily = "DeleteColumnFamilies"
	mutationTypeUpdate             = "Update"
)

// NewBigtableClient - Creates a new instance of BigtableClient.
//
// Parameters:
//   - client: Bigtable client.
//   - logger: Logger instance.
//   - sqlClient: Bigtable SQL client.
//   - config: BigtableConfig configuration object.
//   - responseHandler: TypeHandler for response handling.
//   - grpcConn: grpcConn for calling apis.
//   - schemaMapping: schema mapping for response handling.
//
// Returns:
//   - BigtableClient: New instance of BigtableClient
func NewBigtableClient(client map[string]*bigtable.Client, logger *zap.Logger, sqlClient btpb.BigtableClient, config BigtableConfig, responseHandler rh.ResponseHandlerIface, grpcConn *grpc.ClientConn, schemaMapping *schemaMapping.SchemaMappingConfig) *BigtableClient {
	return &BigtableClient{
		Clients:             client,
		Logger:              logger,
		SqlClient:           sqlClient,
		BigtableConfig:      config,
		ResponseHandler:     responseHandler,
		SchemaMappingConfig: schemaMapping,
		grpcConn:            grpcConn,
	}
}

// mutateRow() - Applies mutations to a row in the specified Bigtable table.
//
// Parameters:
//   - ctx: Context for the operation, used for cancellation and deadlines.
//   - tableName: Name of the table where the row exists.
//   - rowKey: Row key of the row to mutate.
//   - columns: Columns to mutate.
//   - values: Values to set in the columns.
//   - deleteColumnFamilies: Column families to delete.
//
// Returns:
//   - error: Error if the mutation fails.
func (btc *BigtableClient) mutateRow(ctx context.Context, tableName, rowKey string, columns []translator.Column, values []any, deleteColumnFamilies []string, deleteQualifiers []translator.Column, timestamp bigtable.Timestamp, ifSpec translator.IfSpec, keyspace string, complexUpdateMeta map[string]*translator.ComplexUpdateMeta) (*message.RowsResult, error) {
	otelgo.AddAnnotation(ctx, applyingBigtableMutation)
	mut := bigtable.NewMutation()

	client, ok := btc.Clients[keyspace]
	if !ok {
		return nil, fmt.Errorf("invalid keySpace - `%s`", keyspace)
	}

	tbl := client.Open(tableName)
	if timestamp == 0 {
		timestamp = bigtable.Timestamp(bigtable.Now().Time().UnixMicro())
	}

	// Delete column families
	for _, cf := range deleteColumnFamilies {
		mut.DeleteCellsInFamily(cf)
	}

	// Handle complex updates
	for cf, meta := range complexUpdateMeta {
		if meta.UpdateListIndex != "" {
			index, err := strconv.Atoi(meta.UpdateListIndex)
			if err != nil {
				return nil, err
			}
			reqTimestamp, err := btc.getIndexOpTimestamp(ctx, tableName, rowKey, cf, keyspace, index)
			if err != nil {
				return nil, err
			}
			mut.Set(cf, reqTimestamp, timestamp, meta.Value)
		}
		if meta.ListDelete {
			if err := btc.setMutationforListDelete(ctx, tableName, rowKey, cf, keyspace, meta.ListDeleteValues, mut); err != nil {
				return nil, err
			}
		}
	}

	// Delete specific column qualifiers
	for _, q := range deleteQualifiers {
		mut.DeleteCellsInColumn(q.ColumnFamily, q.Name)
	}

	// Set values for columns
	for i, column := range columns {
		if bv, ok := values[i].([]byte); ok {
			mut.Set(column.ColumnFamily, column.Name, timestamp, bv)
		} else {
			btc.Logger.Error("Value is not of type []byte", zap.String("column", column.Name), zap.Any("value", values[i]))
			return nil, fmt.Errorf("value for column %s is not of type []byte", column.Name)
		}
	}

	if ifSpec.IfExists || ifSpec.IfNotExists {
		predicateFilter := bigtable.CellsPerRowLimitFilter(1)
		matched := true
		conditionalMutation := bigtable.NewCondMutation(predicateFilter, mut, nil)
		if ifSpec.IfNotExists {
			conditionalMutation = bigtable.NewCondMutation(predicateFilter, nil, mut)
		}

		err := tbl.Apply(ctx, rowKey, conditionalMutation, bigtable.GetCondMutationResult(&matched))
		otelgo.AddAnnotation(ctx, bigtableMutationApplied)
		if err != nil {
			return nil, err
		}

		return GenerateAppliedRowsResult(keyspace, tableName, ifSpec.IfExists == matched), nil
	}

	// If no conditions, apply the mutation directly
	err := tbl.Apply(ctx, rowKey, mut)
	otelgo.AddAnnotation(ctx, bigtableMutationApplied)
	if err != nil {
		return nil, err
	}

	return &message.RowsResult{
		Metadata: &message.RowsMetadata{
			LastContinuousPage: true,
		},
	}, nil
}

// InsertRow - Inserts a row into the specified Bigtable table.
//
// Parameters:
//   - ctx: Context for the operation, used for cancellation and deadlines.
//   - data: InsertQueryMap object containing the table, row key, columns, values, and deleteColumnFamilies.
//
// Returns:
//   - error: Error if the insertion fails.
func (btc *BigtableClient) InsertRow(ctx context.Context, insertQueryData *translator.InsertQueryMap) (*message.RowsResult, error) {
	return btc.mutateRow(ctx, insertQueryData.Table, insertQueryData.RowKey, insertQueryData.Columns, insertQueryData.Values, insertQueryData.DeleteColumnFamilies, []translator.Column{}, insertQueryData.TimestampInfo.Timestamp, translator.IfSpec{IfNotExists: insertQueryData.IfNotExists}, insertQueryData.Keyspace, nil)
}

// UpdateRow - Updates a row in the specified Bigtable table.
//
// Parameters:
//   - ctx: Context for the operation, used for cancellation and deadlines.
//   - data: UpdateQueryMap object containing the table, row key, columns, values, and DeleteColumnFamilies.
//
// Returns:
//   - error: Error if the update fails.
func (btc *BigtableClient) UpdateRow(ctx context.Context, updateQueryData *translator.UpdateQueryMap) (*message.RowsResult, error) {
	return btc.mutateRow(ctx, updateQueryData.Table, updateQueryData.RowKey, updateQueryData.Columns, updateQueryData.Values, updateQueryData.DeleteColumnFamilies, updateQueryData.DeleteColumQualifires, updateQueryData.TimestampInfo.Timestamp, translator.IfSpec{IfExists: updateQueryData.IfExists}, updateQueryData.Keyspace, updateQueryData.ComplexUpdateMeta)
}

// DeleteRow - Deletes a row in the specified Bigtable table.
//
// Parameters:
//   - ctx: Context for the operation, used for cancellation and deadlines.
//   - queryData: DeleteQueryMap object containing the table and row key.
//
// Returns:
//   - error: Error if the deletion fails.
func (btc *BigtableClient) DeleteRow(ctx context.Context, deleteQueryData *translator.DeleteQueryMap) (*message.RowsResult, error) {
	otelgo.AddAnnotation(ctx, applyingDeleteMutation)
	client, ok := btc.Clients[deleteQueryData.Keyspace]
	if !ok {
		return nil, fmt.Errorf("invalid keySpace - `%s`", deleteQueryData.Keyspace)
	}
	tbl := client.Open(deleteQueryData.Table)
	mut := bigtable.NewMutation()

	if len(deleteQueryData.SelectedColumns) > 0 {
		for _, v := range deleteQueryData.SelectedColumns {
			if v.ListIndex != "" { //handle list delete Items
				index, err := strconv.Atoi(v.ListIndex)
				if err != nil {
					return nil, err
				}
				reqTimestamp, err := btc.getIndexOpTimestamp(ctx, deleteQueryData.Table, deleteQueryData.RowKey, v.Name, deleteQueryData.Keyspace, index)
				if err != nil {
					return nil, err
				}
				mut.DeleteCellsInColumn(v.Name, reqTimestamp)
			} else if v.MapKey != "" { // Handle map delete Items
				mut.DeleteCellsInColumn(v.Name, v.MapKey)
			}
		}
	} else {
		mut.DeleteRow()
	}
	if deleteQueryData.IfExists {
		predicateFilter := bigtable.CellsPerRowLimitFilter(1)
		conditionalMutation := bigtable.NewCondMutation(predicateFilter, mut, nil)
		matched := true
		if err := tbl.Apply(ctx, deleteQueryData.RowKey, conditionalMutation, bigtable.GetCondMutationResult(&matched)); err != nil {
			return nil, err
		}

		if !matched {
			return GenerateAppliedRowsResult(deleteQueryData.Keyspace, deleteQueryData.Table, false), nil
		} else {
			return GenerateAppliedRowsResult(deleteQueryData.Keyspace, deleteQueryData.Table, true), nil
		}
	} else {
		if err := tbl.Apply(ctx, deleteQueryData.RowKey, mut); err != nil {
			return nil, err
		}
	}
	otelgo.AddAnnotation(ctx, deleteMutationApplied)
	var response = message.RowsResult{
		Metadata: &message.RowsMetadata{
			LastContinuousPage: true,
		},
	}
	return &response, nil
}

// GetSchemaMappingConfigs - Retrieves schema mapping configurations from the specified config table.
//
// Parameters:
//   - ctx: Context for the operation, used for cancellation and deadlines.
//   - schemaMappingTable: Name of the table containing the configuration.
//
// Returns:
//   - map[string]map[string]*schemaMapping.Column: Table metadata.
//   - map[string][]schemaMapping.Column: Primary key metadata.
//   - error: Error if the retrieval fails.
func (btc *BigtableClient) GetSchemaMappingConfigs(ctx context.Context, keyspace, schemaMappingTable string) (map[string]map[string]*schemaMapping.Column, map[string][]schemaMapping.Column, error) {
	otelgo.AddAnnotation(ctx, fetchingSchemaMappingConfig)

	client, ok := btc.Clients[keyspace]
	if !ok {
		return nil, nil, fmt.Errorf("invalid keySpace - `%s`", keyspace)
	}

	table := client.Open(schemaMappingTable)
	filter := bigtable.LatestNFilter(1)

	tableMetadata := make(map[string]map[string]*schemaMapping.Column)
	pkMetadata := make(map[string][]schemaMapping.Column)

	err := table.ReadRows(ctx, bigtable.InfiniteRange(""), func(row bigtable.Row) bool {
		// Extract the row key and column values
		var tableName, columnName, columnType, KeyType string
		var isPrimaryKey, isCollection bool
		var pkPrecedence int64

		// Extract column values
		for _, item := range row["cf"] {
			switch item.Column {
			case "cf:TableName":
				tableName = string(item.Value)
			case "cf:ColumnName":
				columnName = string(item.Value)
			case "cf:ColumnType":
				columnType = string(item.Value)
			case "cf:IsPrimaryKey":
				isPrimaryKey = string(item.Value) == "true"
			case "cf:PK_Precedence":
				pkPrecedence, _ = json.Number(string(item.Value)).Int64()
			case "cf:IsCollection":
				isCollection = string(item.Value) == "true"
			case "cf:KeyType":
				KeyType = string(item.Value)
			}
		}
		cqlType, err := utilities.GetCassandraColumnType(columnType)
		if err != nil {
			btc.Logger.Error("Failed to read rows from Bigtable - possible issue with schema_mapping table:", zap.Error(err))
			return false
		}
		columnMetadata := message.ColumnMetadata{
			Keyspace: keyspace,
			Table:    tableName,
			Name:     columnName,
			Type:     cqlType,
		}

		// Create a new column struct
		column := schemaMapping.Column{
			ColumnName:   columnName,
			ColumnType:   columnType,
			IsPrimaryKey: isPrimaryKey,
			PkPrecedence: pkPrecedence,
			IsCollection: isCollection,
			Metadata:     columnMetadata,
			KeyType:      KeyType,
		}

		if _, exists := tableMetadata[tableName]; !exists {
			tableMetadata[tableName] = make(map[string]*schemaMapping.Column)
		}

		if _, exists := pkMetadata[tableName]; !exists {
			var pkSlice []schemaMapping.Column
			pkMetadata[tableName] = pkSlice
		}

		column.Metadata.Index = int32(len(tableMetadata[tableName]))
		tableMetadata[tableName][column.ColumnName] = &column
		if column.IsPrimaryKey {
			pkSlice := pkMetadata[tableName]
			pkSlice = append(pkSlice, column)
			pkMetadata[tableName] = pkSlice
		}

		return true
	}, bigtable.RowFilter(filter))

	if err != nil {
		btc.Logger.Error("Failed to read rows from Bigtable - possible issue with schema_mapping table:", zap.Error(err))
		return nil, nil, err
	}
	if len(tableMetadata) == 0 {
		return nil, nil, fmt.Errorf("config table %s empty", schemaMappingTable)
	}
	otelgo.AddAnnotation(ctx, schemaMappingConfigFetched)
	return tableMetadata, sortPkData(pkMetadata), nil
}

// DeleteRowsUsingTimestamp - Deletes specific column values in a row based on timestamp range in Bigtable.
//
// Parameters:
//   - ctx: Context for the operation, used for cancellation and deadlines.
//   - tableName: Name of the table where the row exists.
//   - columns: List of column names to delete.
//   - rowkey: The row key for identifying the row to modify.
//   - columnFamily: The column family containing the columns to delete.
//   - timestamp: The timestamp range up to which the cells should be deleted.
//   - keyspace: The keyspace (Bigtable instance) containing the table.
//
// Returns:
//   - error: Returns an error if the deletion operation fails.
func (btc *BigtableClient) DeleteRowsUsingTimestamp(ctx context.Context, tableName string, columns []string, rowkey string, columnFamily string, timestamp translator.TimestampInfo, keyspace string) error {
	client, ok := btc.Clients[keyspace]
	if !ok {
		return fmt.Errorf("invalid keySpace - `%s`", keyspace)
	}

	tbl := client.Open(tableName)
	mut := bigtable.NewMutation()
	for _, value := range columns {
		mut.DeleteTimestampRange(columnFamily, value, 0, timestamp.Timestamp)
	}
	if err := tbl.Apply(ctx, rowkey, mut); err != nil {
		btc.Logger.Error("failed to delete row", zap.String("RowKey", rowkey), zap.Error(err))
	}
	return nil
}

// ApplyBulkMutation - Applies bulk mutations to the specified Bigtable table.
//
// Parameters:
//   - ctx: Context for the operation, used for cancellation and deadlines.
//   - tableName: Name of the table to apply bulk mutations to.
//   - mutationData: Slice of MutationData objects containing mutation details.
//
// Returns:
//   - BulkOperationResponse: Response indicating the result of the bulk operation.
//   - error: Error if the bulk mutation fails.
func (btc *BigtableClient) ApplyBulkMutation(ctx context.Context, tableName string, mutationData []MutationData, keyspace string) (BulkOperationResponse, error) {
	rowKeyToMutationMap := make(map[string]*bigtable.Mutation)
	for _, md := range mutationData {
		if _, exists := rowKeyToMutationMap[md.RowKey]; !exists {
			rowKeyToMutationMap[md.RowKey] = bigtable.NewMutation()
		}
		mut := rowKeyToMutationMap[md.RowKey]
		switch md.MutationType {
		case mutationTypeInsert:
			{
				for _, column := range md.Columns {
					mut.Set(column.ColumnFamily, column.Name, bigtable.Now(), column.Contents)
				}
			}
		case mutationTypeDelete:
			{
				mut.DeleteRow()
			}
		case mutationTypeDeleteColumnFamily:
			{
				mut.DeleteCellsInFamily(md.ColumnFamily)
			}
		case mutationTypeUpdate:
			{
				for _, column := range md.Columns {
					mut.Set(column.ColumnFamily, column.Name, bigtable.Now(), column.Contents)
				}
			}
		default:
			return BulkOperationResponse{
				FailedRows: "",
			}, fmt.Errorf("invalid mutation type `%s`", md.MutationType)
		}
	}
	// create mutations from mutation data
	var mutations []*bigtable.Mutation
	var rowKeys []string

	for key, mutation := range rowKeyToMutationMap {
		mutations = append(mutations, mutation)
		rowKeys = append(rowKeys, key)
	}
	otelgo.AddAnnotation(ctx, applyingBulkMutation)

	client, ok := btc.Clients[keyspace]
	if !ok {
		return BulkOperationResponse{
			FailedRows: "All Rows are failed",
		}, fmt.Errorf("invalid keySpace - `%s`", keyspace)
	}

	tbl := client.Open(tableName)
	errs, err := tbl.ApplyBulk(ctx, rowKeys, mutations)

	if err != nil {
		return BulkOperationResponse{
			FailedRows: "All Rows are failed",
		}, fmt.Errorf("ApplyBulk: %w", err)
	}

	var failedRows []string
	for i, e := range errs {
		if e != nil {
			failedRows = append(failedRows, rowKeys[i])
		}
	}
	var res BulkOperationResponse
	if len(failedRows) > 0 {
		res = BulkOperationResponse{
			FailedRows: fmt.Sprintf("failed rowkeys: %v", failedRows),
		}
	} else {
		res = BulkOperationResponse{
			FailedRows: "",
		}
	}
	otelgo.AddAnnotation(ctx, bulkMutationApplied)
	return res, nil
}

// Close() gracefully shuts down the Bigtable client and gRPC connection.
//
// It iterates through all active Bigtable clients and closes them before closing the gRPC connection.
func (btc *BigtableClient) Close() {
	for _, clients := range btc.Clients {
		clients.Close()
	}
	btc.grpcConn.Close()
}

// getIndexOpTimestamp() retrieves the timestamp qualifier for a given list index in a Bigtable row.
//
// Parameters:
//   - ctx: Context for the operation, used for cancellation and deadlines.
//   - tableName: The name of the Bigtable table.
//   - rowKey: The row key to look up.
//   - columnFamily: The column family where the list is stored.
//   - keyspace: The keyspace identifier.
//   - index: The index of the list element for which the timestamp is required.
//
// Returns:
//   - string: The timestamp qualifier if found.
//   - error: An error if the row does not exist, the index is out of bounds, or any other retrieval failure occurs.
func (btc *BigtableClient) getIndexOpTimestamp(ctx context.Context, tableName, rowKey, columnFamily, keyspace string, index int) (string, error) {
	client, ok := btc.Clients[keyspace]
	if !ok {
		return "", fmt.Errorf("invalid keySpace - `%s`", keyspace)
	}
	tbl := client.Open(tableName)
	row, err := tbl.ReadRow(ctx, rowKey, bigtable.RowFilter(bigtable.ChainFilters(
		bigtable.FamilyFilter(columnFamily),
		bigtable.LatestNFilter(1), // this filter is so that we fetch only the latest timestamp value
	)))
	if err != nil {
		btc.Logger.Error("Failed to read row", zap.String("RowKey", rowKey), zap.Error(err))
		return "", err
	}

	if len(row[columnFamily]) <= 0 {
		return "", fmt.Errorf("no values found in list %s", columnFamily)
	}
	for i, item := range row[columnFamily] {
		if i == index {
			splits := strings.Split(item.Column, ":")
			qualifier := splits[1]
			return qualifier, nil
		}
	}
	return "", fmt.Errorf("index %d out of bounds for list size %d", index, len(row[columnFamily]))
}

// setMutationforListDelete() applies deletions to specific list elements in a Bigtable row.
//
// This function identifies and removes the specified elements from a list stored in a column family.
//
// Parameters:
//   - ctx: Context for the operation, used for cancellation and deadlines.
//   - tableName: The name of the Bigtable table.
//   - rowKey: The row key containing the list.
//   - columnFamily: The column family storing the list.
//   - keyspace: The keyspace identifier.
//   - deleteList: A slice of byte arrays representing the values to be deleted from the list.
//   - mut: The Bigtable mutation object to which delete operations will be added.
//
// Returns:
//   - error: An error if the row does not exist or if list elements cannot be deleted.
func (btc *BigtableClient) setMutationforListDelete(ctx context.Context, tableName, rowKey, columnFamily, keyspace string, deleteList [][]byte, mut *bigtable.Mutation) error {
	client, ok := btc.Clients[keyspace]
	if !ok {
		return fmt.Errorf("invalid keySpace - `%s`", keyspace)
	}

	tbl := client.Open(tableName)
	row, err := tbl.ReadRow(ctx, rowKey, bigtable.RowFilter(bigtable.ChainFilters(
		bigtable.FamilyFilter(columnFamily),
		bigtable.LatestNFilter(1), // this filter is so that we fetch only the latest timestamp value
	)))
	if err != nil {
		btc.Logger.Error("Failed to read row", zap.String("RowKey", rowKey), zap.Error(err))
		return err
	}

	if len(row[columnFamily]) <= 0 {
		return fmt.Errorf("no values found in list %s", columnFamily)
	}
	for _, item := range row[columnFamily] {
		for _, dItem := range deleteList {
			if bytes.Equal(dItem, item.Value) {
				splits := strings.Split(item.Column, ":")
				qualifier := splits[1]
				mut.DeleteCellsInColumn(columnFamily, qualifier)
			}
		}
	}
	return nil
}
