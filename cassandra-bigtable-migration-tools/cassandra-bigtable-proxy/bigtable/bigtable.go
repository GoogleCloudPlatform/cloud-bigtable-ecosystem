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
	"encoding/hex"
	"fmt"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	"strings"
	"time"

	"cloud.google.com/go/bigtable"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	btpb "cloud.google.com/go/bigtable/apiv2/bigtablepb"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/metadata"
	otelgo "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/otel"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"go.uber.org/zap"
)

type BigtableAdapter struct {
	clients       *types.BigtableClientManager
	Logger        *zap.Logger
	sqlClient     btpb.BigtableClient
	config        *types.BigtableConfig
	schemaManager *metadata.MetadataStore
}

func NewBigtableClient(clients *types.BigtableClientManager, logger *zap.Logger, config *types.BigtableConfig, schemaManager *metadata.MetadataStore) *BigtableAdapter {
	return &BigtableAdapter{
		clients:       clients,
		Logger:        logger,
		config:        config,
		schemaManager: schemaManager,
	}
}

func (btc *BigtableAdapter) Execute(ctx context.Context, query types.IExecutableQuery) (message.Message, error) {
	switch q := query.(type) {
	case *types.BoundDeleteQuery:
		return btc.DeleteRow(ctx, q)
	case *types.BigtableWriteMutation:
		return btc.mutateRow(ctx, q)
	case *types.ExecutableSelectQuery:
		return btc.ExecutePreparedStatement(ctx, q)
	case *types.CreateTableStatementMap:
		return btc.schemaManager.CreateTable(ctx, q)
	case *types.AlterTableStatementMap:
		return btc.schemaManager.AlterTable(ctx, q)
	case *types.TruncateTableStatementMap:
		err := btc.DropAllRows(ctx, q)
		return emptyRowsResult(), err
	case *types.DropTableQuery:
		return btc.schemaManager.DropTable(ctx, q)
	default:
		return nil, fmt.Errorf("unhandled prepared query type: %T", query)
	}
}

// mutateRow() - Applies mutations to a row in the specified Bigtable table.
//
// Parameters:
//   - ctx: Context for the operation, used for cancellation and deadlines.
//   - tableName: Columns of the table where the row exists.
//   - rowKey: GoRow key of the row to mutate.
//   - columns: Columns to mutate.
//   - values: Columns to set in the columns.
//   - deleteColumnFamilies: Columns families to delete.
//
// Returns:
//   - error: Error if the mutation fails.
func (btc *BigtableAdapter) mutateRow(ctx context.Context, input *types.BigtableWriteMutation) (message.Message, error) {
	otelgo.AddAnnotation(ctx, applyingBigtableMutation)
	mut := bigtable.NewMutation()

	btc.Logger.Info("mutating row", zap.String("key", hex.EncodeToString([]byte(input.RowKey()))))

	client, err := btc.clients.GetClient(input.Keyspace())
	if err != nil {
		return nil, err
	}

	tbl := client.Open(string(input.Table()))

	mutationCount, err := btc.buildMutation(ctx, tbl, input, mut)
	if err != nil {
		return nil, err
	}

	if input.IfSpec.IfExists || input.IfSpec.IfNotExists {
		predicateFilter := bigtable.CellsPerRowLimitFilter(1)
		matched := true
		conditionalMutation := bigtable.NewCondMutation(predicateFilter, mut, nil)
		if input.IfSpec.IfNotExists {
			conditionalMutation = bigtable.NewCondMutation(predicateFilter, nil, mut)
		}

		err := tbl.Apply(ctx, string(input.RowKey()), conditionalMutation, bigtable.GetCondMutationResult(&matched))
		otelgo.AddAnnotation(ctx, bigtableMutationApplied)
		if err != nil {
			return nil, err
		}

		return GenerateAppliedRowsResult(input.Keyspace(), input.Table(), input.IfSpec.IfExists == matched), nil
	}

	// no-op just return
	if mutationCount == 0 {
		return &message.VoidResult{}, nil
	}

	// If no conditions, apply the mutation directly
	err = tbl.Apply(ctx, string(input.RowKey()), mut)
	otelgo.AddAnnotation(ctx, bigtableMutationApplied)
	if err != nil {
		return nil, err
	}

	return &message.VoidResult{}, nil
}

func emptyRowsResult() *message.RowsResult {
	return &message.RowsResult{
		Metadata: &message.RowsMetadata{
			LastContinuousPage: true,
		},
	}
}

func (btc *BigtableAdapter) buildMutation(ctx context.Context, table *bigtable.Table, input *types.BigtableWriteMutation, mut *bigtable.Mutation) (int, error) {
	var mutationCount = 0

	var timestamp bigtable.Timestamp
	if input.UsingTimestamp != nil && input.UsingTimestamp.HasUsingTimestamp {
		timestamp = bigtable.Time(input.UsingTimestamp.Timestamp)
	} else {
		timestamp = bigtable.Time(time.Now())
	}

	for _, mutationOp := range input.Mutations() {
		switch m := mutationOp.(type) {
		case *types.WriteCellOp:
			mut.Set(string(m.Family), string(m.Column), timestamp, m.Bytes)
		case *types.DeleteColumnOp:
			mut.DeleteCellsInColumn(string(m.Column.Family), string(m.Column.Column))
		case *types.BigtableCounterOp:
			mut.AddIntToCell(string(m.Family), "", counterTimestamp, m.Value)
		case *types.DeleteCellsOp:
			mut.DeleteCellsInFamily(string(m.Family))
		case *types.BigtableSetIndexOp:
			reqTimestamp, err := btc.getIndexOpTimestamp(ctx, table, input.RowKey(), m.Family, int(m.Index))
			if err != nil {
				return 0, err
			}
			mut.Set(string(m.Family), reqTimestamp, timestamp, m.Value)
		case *types.BigtableDeleteListElementsOp:
			err := btc.setMutationForListDelete(ctx, table, input.RowKey(), m.Family, m.Values, mut)
			if err != nil {
				return 0, err
			}
		default:
			return 0, fmt.Errorf("unhandled mutation type %T", mutationOp)
		}
		mutationCount++
	}
	return mutationCount, nil
}

func (btc *BigtableAdapter) DropAllRows(ctx context.Context, data *types.TruncateTableStatementMap) error {
	_, err := btc.schemaManager.Schemas().GetTableSchema(data.Keyspace(), data.Table())
	if err != nil {
		return err
	}

	adminClient, err := btc.clients.GetAdmin(data.Keyspace())
	if err != nil {
		return err
	}

	btc.Logger.Info("truncate table: dropping all bigtable rows")
	err = adminClient.DropAllRows(ctx, string(data.Table()))
	if status.Code(err) == codes.NotFound {
		// Table doesn't exist in config, which is fine for a truncate.
		btc.Logger.Info("truncate table: table not found in bigtable, nothing to drop", zap.String("table", string(data.Table())))
		return nil
	}
	if err != nil {
		btc.Logger.Error("truncate table: failed", zap.Error(err))
		return err
	}
	btc.Logger.Info("truncate table: complete")
	return nil
}

// InsertRow - Inserts a row into the specified bigtable table.
//
// Parameters:
//   - ctx: Context for the operation, used for cancellation and deadlines.
//   - data: PreparedInsertQuery object containing the table, row key, columns, values, and deleteColumnFamilies.
//
// Returns:
//   - error: Error if the insertion fails.
func (btc *BigtableAdapter) InsertRow(ctx context.Context, input *types.BigtableWriteMutation) (message.Message, error) {
	return btc.mutateRow(ctx, input)
}

// UpdateRow - Updates a row in the specified bigtable table.
//
// Parameters:
//   - ctx: Context for the operation, used for cancellation and deadlines.
//   - data: PreparedUpdateQuery object containing the table, row key, columns, values, and DeleteColumnFamilies.
//
// Returns:
//   - error: Error if the update fails.
func (btc *BigtableAdapter) UpdateRow(ctx context.Context, input *types.BigtableWriteMutation) (message.Message, error) {
	return btc.mutateRow(ctx, input)
}

func (btc *BigtableAdapter) DeleteRow(ctx context.Context, deleteQueryData *types.BoundDeleteQuery) (message.Message, error) {
	otelgo.AddAnnotation(ctx, applyingDeleteMutation)
	client, err := btc.clients.GetClient(deleteQueryData.Keyspace())
	if err != nil {
		return nil, err
	}
	table := client.Open(string(deleteQueryData.Table()))
	mut := bigtable.NewMutation()

	err = btc.buildDeleteMutation(ctx, table, deleteQueryData, mut)
	if err != nil {
		return nil, err
	}
	if deleteQueryData.IfExists {
		predicateFilter := bigtable.CellsPerRowLimitFilter(1)
		conditionalMutation := bigtable.NewCondMutation(predicateFilter, mut, nil)
		matched := true
		err := table.Apply(ctx, string(deleteQueryData.RowKey()), conditionalMutation, bigtable.GetCondMutationResult(&matched))
		if err != nil {
			return nil, err
		}

		if !matched {
			return GenerateAppliedRowsResult(deleteQueryData.Keyspace(), deleteQueryData.Table(), false), nil
		} else {
			return GenerateAppliedRowsResult(deleteQueryData.Keyspace(), deleteQueryData.Table(), true), nil
		}
	} else {
		err := table.Apply(ctx, string(deleteQueryData.RowKey()), mut)
		if err != nil {
			return nil, err
		}
	}
	otelgo.AddAnnotation(ctx, deleteMutationApplied)
	return &message.VoidResult{}, nil
}

func (btc *BigtableAdapter) buildDeleteMutation(ctx context.Context, table *bigtable.Table, deleteQueryData *types.BoundDeleteQuery, mut *bigtable.Mutation) error {
	if len(deleteQueryData.Columns) > 0 {
		for _, column := range deleteQueryData.Columns {
			switch c := column.(type) {
			case *types.BoundIndexColumn:
				reqTimestamp, err := btc.getIndexOpTimestamp(ctx, table, deleteQueryData.RowKey(), c.Column().ColumnFamily, c.Index)
				if err != nil {
					return err
				}
				mut.DeleteCellsInColumn(string(c.Column().ColumnFamily), reqTimestamp)
			case *types.BoundKeyColumn:
				mut.DeleteCellsInColumn(string(c.Column().ColumnFamily), string(c.Key))
			default:
				return fmt.Errorf("unhandled delete query column %T", c)
			}
		}
	} else {
		mut.DeleteRow()
	}
	return nil
}

// ApplyBulkMutation - Applies bulk mutations to the specified bigtable table.
//
// Parameters:
//   - ctx: Context for the operation, used for cancellation and deadlines.
//   - tableName: Columns of the table to apply bulk mutations to.
//   - mutationData: Slice of MutationData objects containing mutation details.
//
// Returns:
//   - BulkOperationResponse: Response indicating the result of the bulk operation.
//   - error: Error if the bulk mutation fails.
func (btc *BigtableAdapter) ApplyBulkMutation(ctx context.Context, keyspace types.Keyspace, tableName types.TableName, mutationData []types.IBigtableMutation) (BulkOperationResponse, error) {
	client, err := btc.clients.GetClient(keyspace)
	if err != nil {
		return BulkOperationResponse{
			FailedRows: "All Rows are failed",
		}, err
	}

	table := client.Open(string(tableName))

	rowKeyToMutationMap := make(map[types.RowKey]*bigtable.Mutation)
	for _, md := range mutationData {
		rowKey := md.RowKey()
		btc.Logger.Info("mutating row BULK", zap.String("key", hex.EncodeToString([]byte(rowKey))))
		if _, exists := rowKeyToMutationMap[rowKey]; !exists {
			rowKeyToMutationMap[rowKey] = bigtable.NewMutation()
		}
		mut := rowKeyToMutationMap[rowKey]
		switch v := md.(type) {
		case *types.BigtableWriteMutation:
			_, err := btc.buildMutation(ctx, table, v, mut)
			if err != nil {
				return BulkOperationResponse{
					FailedRows: fmt.Sprintf("All Rows are failed because: %s", err.Error()),
				}, err
			}
		case *types.BoundDeleteQuery:
			err := btc.buildDeleteMutation(ctx, table, v, mut)
			if err != nil {
				return BulkOperationResponse{
					FailedRows: fmt.Sprintf("All Rows are failed because: %s", err.Error()),
				}, err
			}
		default:
			return BulkOperationResponse{
				FailedRows: fmt.Sprintf("All Rows are failed because: unsupported bulk operation: %T", v),
			}, fmt.Errorf("unhandled bulk mutation type %T", md)
		}
	}
	// create mutations from mutation data
	var mutations []*bigtable.Mutation
	var rowKeys []string

	for key, mutation := range rowKeyToMutationMap {
		mutations = append(mutations, mutation)
		rowKeys = append(rowKeys, string(key))
	}
	otelgo.AddAnnotation(ctx, applyingBulkMutation)

	errs, err := table.ApplyBulk(ctx, rowKeys, mutations)
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

// getIndexOpTimestamp() retrieves the timestamp qualifier for a given list index in a bigtable row.
//
// Parameters:
//   - ctx: Context for the operation, used for cancellation and deadlines.
//   - tableName: The name of the bigtable table.
//   - rowKey: The row key to look up.
//   - columnFamily: The column family where the list is stored.
//   - keyspace: The keyspace identifier.
//   - index: The index of the list element for which the timestamp is required.
//
// Returns:
//   - string: The timestamp qualifier if found.
//   - error: An error if the row does not exist, the index is out of bounds, or any other retrieval failure occurs.
func (btc *BigtableAdapter) getIndexOpTimestamp(ctx context.Context, table *bigtable.Table, rowKey types.RowKey, columnFamily types.ColumnFamily, index int) (string, error) {
	row, err := table.ReadRow(ctx, string(rowKey), bigtable.RowFilter(bigtable.ChainFilters(
		bigtable.FamilyFilter(string(columnFamily)),
		bigtable.LatestNFilter(1), // this filter is so that we fetch only the latest timestamp value
	)))
	if err != nil {
		btc.Logger.Error("Failed to read row", zap.String("RowKey", string(rowKey)), zap.Error(err))
		return "", err
	}
	columns := row[string(columnFamily)]
	if len(columns) <= 0 {
		return "", fmt.Errorf("no values found in list %s", string(columnFamily))
	}
	for i, item := range columns {
		if i == index {
			splits := strings.Split(item.Column, ":")
			qualifier := splits[1]
			return qualifier, nil
		}
	}
	return "", fmt.Errorf("index %d out of bounds for list size %d", index, len(columns))
}

// setMutationForListDelete() applies deletions to specific list elements in a bigtable row.
//
// This function identifies and removes the specified elements from a list stored in a column family.
//
// Parameters:
//   - ctx: Context for the operation, used for cancellation and deadlines.
//   - tableName: The name of the bigtable table.
//   - rowKey: The row key containing the list.
//   - columnFamily: The column family storing the list.
//   - keyspace: The keyspace identifier.
//   - deleteList: A slice of byte arrays representing the values to be deleted from the list.
//   - mut: The bigtable mutation object to which delete operations will be added.
//
// Returns:
//   - error: An error if the row does not exist or if list elements cannot be deleted.
func (btc *BigtableAdapter) setMutationForListDelete(ctx context.Context, table *bigtable.Table, rowKey types.RowKey, columnFamily types.ColumnFamily, deleteList []types.BigtableValue, mut *bigtable.Mutation) error {
	row, err := table.ReadRow(ctx, string(rowKey), bigtable.RowFilter(bigtable.ChainFilters(
		bigtable.FamilyFilter(string(columnFamily)),
		bigtable.LatestNFilter(1), // this filter is so that we fetch only the latest timestamp value
	)))
	if err != nil {
		btc.Logger.Error("Failed to read row", zap.String("RowKey", string(rowKey)), zap.Error(err))
		return err
	}

	if len(row[string(columnFamily)]) <= 0 {
		return fmt.Errorf("no values found in list %s", columnFamily)
	}
	for _, item := range row[string(columnFamily)] {
		for _, dItem := range deleteList {
			if bytes.Equal(dItem, item.Value) {
				splits := strings.Split(item.Column, ":")
				qualifier := splits[1]
				mut.DeleteCellsInColumn(string(columnFamily), qualifier)
			}
		}
	}
	return nil
}

// PrepareStatement prepares a query for execution using the bigtable SQL client.
func (btc *BigtableAdapter) PrepareStatement(ctx context.Context, query types.IPreparedQuery) (*bigtable.PreparedStatement, error) {
	// we don't use bigtable for system queries
	if query.Keyspace().IsSystemKeyspace() {
		return nil, nil
	}

	selectQuery, ok := query.(*types.PreparedSelectQuery)
	if !ok {
		// only select queries can be prepared in Bigtable at this time
		return nil, nil
	}
	client, err := btc.clients.GetClient(query.Keyspace())
	if err != nil {
		return nil, err
	}

	paramTypes, err := BuildParamTypes(selectQuery)
	if err != nil {
		btc.Logger.Error("Failed to prepare statement", zap.String("query", query.BigtableQuery()), zap.Error(err))
		return nil, err
	}
	preparedStatement, err := client.PrepareStatement(ctx, query.BigtableQuery(), paramTypes)
	if err != nil {
		btc.Logger.Error("Failed to prepare statement", zap.String("query", query.BigtableQuery()), zap.Error(err))
		return nil, fmt.Errorf("failed to prepare statement '%s': %w", query.CqlQuery(), err)
	}

	return preparedStatement, nil
}
