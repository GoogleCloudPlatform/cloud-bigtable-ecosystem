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
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/bigtable"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	constants "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/constants"
	types "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	otelgo "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/otel"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/responsehandler"
	rh "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/responsehandler"
	schemaMapping "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/schema-mapping"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/translator"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/utilities"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"go.uber.org/zap"
)

// Events
const (
	applyingBigtableMutation       = "Applying Insert/Update Mutation"
	bigtableMutationApplied        = "Insert/Update Mutation Applied"
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
	schemaMappingTableColumnFamily = "cf"
	// Cassandra doesn't have a time dimension to their counters, so we need to
	// use the same time for all counters
	counterTimestamp            = 0
	smColTableName              = "TableName"
	smColColumnName             = "ColumnName"
	smColColumnType             = "ColumnType"
	smColIsCollection           = "IsCollection"
	smColIsPrimaryKey           = "IsPrimaryKey"
	smColPKPrecedence           = "PK_Precedence"
	smColKeyType                = "KeyType"
	smFullQualifierTableName    = schemaMappingTableColumnFamily + ":" + smColTableName
	smFullQualifierColumnName   = schemaMappingTableColumnFamily + ":" + smColColumnName
	smFullQualifierColumnType   = schemaMappingTableColumnFamily + ":" + smColColumnType
	smFullQualifierIsPrimaryKey = schemaMappingTableColumnFamily + ":" + smColIsPrimaryKey
	smFullQualifierPKPrecedence = schemaMappingTableColumnFamily + ":" + smColPKPrecedence
	smFullQualifierKeyType      = schemaMappingTableColumnFamily + ":" + smColKeyType
)

type BigTableClientIface interface {
	ApplyBulkMutation(context.Context, string, []MutationData, string) (BulkOperationResponse, error)
	Close()
	DeleteRowNew(context.Context, *translator.DeleteQueryMapping) (*message.RowsResult, error)
	ReadTableConfigs(context.Context, string) ([]*schemaMapping.TableConfig, error)
	InsertRow(context.Context, *translator.InsertQueryMapping) (*message.RowsResult, error)
	SelectStatement(context.Context, rh.QueryMetadata) (*message.RowsResult, time.Time, error)
	AlterTable(ctx context.Context, data *translator.AlterTableStatementMap) error
	CreateTable(ctx context.Context, data *translator.CreateTableStatementMap) error
	DropAllRows(ctx context.Context, data *translator.TruncateTableStatementMap) error
	DropTable(ctx context.Context, data *translator.DropTableStatementMap) error
	UpdateRow(context.Context, *translator.UpdateQueryMapping) (*message.RowsResult, error)
	PrepareStatement(ctx context.Context, query rh.QueryMetadata) (*bigtable.PreparedStatement, error)
	ExecutePreparedStatement(ctx context.Context, query rh.QueryMetadata, preparedStmt *bigtable.PreparedStatement) (*message.RowsResult, time.Time, error)

	// Functions realted to updating the intance properties.
	LoadConfigs(rh *responsehandler.TypeHandler, schemaConfig *schemaMapping.SchemaMappingConfig)
}

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
var NewBigtableClient = func(client map[string]*bigtable.Client, adminClients map[string]*bigtable.AdminClient, logger *zap.Logger, config *types.BigtableConfig, responseHandler rh.ResponseHandlerIface, schemaMapping *schemaMapping.SchemaMappingConfig) BigTableClientIface {
	return &BigtableClient{
		Clients:             client,
		AdminClients:        adminClients,
		Logger:              logger,
		BigtableConfig:      config,
		ResponseHandler:     responseHandler,
		SchemaMappingConfig: schemaMapping,
	}
}

func (btc *BigtableClient) reloadSchemaMappings(ctx context.Context, keyspace string) error {
	tableConfigs, err := btc.ReadTableConfigs(ctx, keyspace)
	if err != nil {
		return fmt.Errorf("error when reloading schema mappings for %s.%s: %w", keyspace, btc.SchemaMappingConfig.SchemaMappingTableName, err)
	}
	err = btc.SchemaMappingConfig.ReplaceTables(keyspace, tableConfigs)
	if err != nil {
		return fmt.Errorf("error updating schema cache for keyspace %s: %w", keyspace, err)
	}
	return nil
}

// scan the schema mapping table to determine if the table exists
func (btc *BigtableClient) tableSchemaExists(ctx context.Context, client *bigtable.Client, tableName string) (bool, error) {
	table := client.Open(btc.SchemaMappingConfig.SchemaMappingTableName)
	exists := false
	err := table.ReadRows(ctx, bigtable.PrefixRange(tableName+"#"), func(row bigtable.Row) bool {
		exists = true
		return false
	}, bigtable.LimitRows(1))
	return exists, err
}

func (btc *BigtableClient) tableResourceExists(ctx context.Context, adminClient *bigtable.AdminClient, table string) (bool, error) {
	_, err := adminClient.TableInfo(ctx, table)
	// todo figure out which error message is for table doesn't exist yet or find better check
	if status.Code(err) == codes.NotFound || status.Code(err) == codes.InvalidArgument {
		return false, nil
	}
	// something went wrong
	if err != nil {
		return false, err
	}
	return true, nil
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
func (btc *BigtableClient) mutateRow(ctx context.Context, tableName, rowKey string, columns []types.Column, values []any, deleteColumnFamilies []string, deleteQualifiers []types.Column, timestamp bigtable.Timestamp, ifSpec translator.IfSpec, keyspace string, ComplexOperation map[string]*translator.ComplexOperation) (*message.RowsResult, error) {
	otelgo.AddAnnotation(ctx, applyingBigtableMutation)
	mut := bigtable.NewMutation()

	btc.Logger.Info("mutating row", zap.String("key", hex.EncodeToString([]byte(rowKey))))

	client, err := btc.getClient(keyspace)
	if err != nil {
		return nil, err
	}

	tbl := client.Open(tableName)
	if timestamp == 0 {
		timestamp = bigtable.Timestamp(bigtable.Now().Time().UnixMicro())
	}

	var mutationCount = 0
	// Delete column families
	for _, cf := range deleteColumnFamilies {
		mut.DeleteCellsInFamily(cf)
		mutationCount++
	}

	// Handle complex updates
	for cf, meta := range ComplexOperation {
		if meta.IncrementType != translator.None {
			var incrementValue = meta.IncrementValue
			if meta.IncrementType == translator.Decrement {
				incrementValue *= -1
			}
			mut.AddIntToCell(cf, "", counterTimestamp, incrementValue)
			mutationCount++
		}
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
			mutationCount++
		}
		if meta.ListDelete {
			if err := btc.setMutationforListDelete(ctx, tableName, rowKey, cf, keyspace, meta.ListDeleteValues, mut); err != nil {
				return nil, err
			}
			mutationCount++
		}
	}

	// Delete specific column qualifiers
	for _, q := range deleteQualifiers {
		mut.DeleteCellsInColumn(q.ColumnFamily, q.Name)
		mutationCount++
	}

	// Set values for columns
	for i, column := range columns {
		if bv, ok := values[i].([]byte); ok {
			mut.Set(column.ColumnFamily, column.Name, timestamp, bv)
			mutationCount++
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

	// no-op just return
	if mutationCount == 0 {
		return &message.RowsResult{
			Metadata: &message.RowsMetadata{
				LastContinuousPage: true,
			},
		}, nil
	}

	// If no conditions, apply the mutation directly
	err = tbl.Apply(ctx, rowKey, mut)
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

func (btc *BigtableClient) DropAllRows(ctx context.Context, data *translator.TruncateTableStatementMap) error {
	_, err := btc.SchemaMappingConfig.GetTableConfig(data.Keyspace, data.Table)
	if err != nil {
		return err
	}

	adminClient, err := btc.getAdminClient(data.Keyspace)
	if err != nil {
		return err
	}

	btc.Logger.Info("truncate table: dropping all bigtable rows")
	err = adminClient.DropAllRows(ctx, data.Table)
	if status.Code(err) == codes.NotFound {
		// Table doesn't exist in BigtableConfig, which is fine for a truncate.
		btc.Logger.Info("truncate table: table not found in bigtable, nothing to drop", zap.String("table", data.Table))
		return nil
	}
	if err != nil {
		btc.Logger.Error("truncate table: failed", zap.Error(err))
		return err
	}
	btc.Logger.Info("truncate table: complete")
	return nil
}

func (btc *BigtableClient) DropTable(ctx context.Context, data *translator.DropTableStatementMap) error {
	_, err := btc.SchemaMappingConfig.GetTableConfig(data.Keyspace, data.Table)
	if err != nil && !data.IfExists {
		return err
	}

	client, err := btc.getClient(data.Keyspace)
	if err != nil {
		return err
	}
	adminClient, err := btc.getAdminClient(data.Keyspace)
	if err != nil {
		return err
	}

	// first clean up table from schema mapping table because that's the SoT
	tbl := client.Open(btc.SchemaMappingConfig.SchemaMappingTableName)
	var deleteMuts []*bigtable.Mutation
	var rowKeysToDelete []string
	err = tbl.ReadRows(ctx, bigtable.PrefixRange(data.Table+"#"), func(row bigtable.Row) bool {
		mut := bigtable.NewMutation()
		mut.DeleteRow()
		deleteMuts = append(deleteMuts, mut)
		rowKeysToDelete = append(rowKeysToDelete, row.Key())
		return true
	})

	if err != nil {
		return err
	}

	btc.Logger.Info("drop table: deleting schema rows")
	_, err = tbl.ApplyBulk(ctx, rowKeysToDelete, deleteMuts)
	if err != nil {
		return err
	}

	// do a read to check if the table exists to save on admin API write quota
	exists, err := btc.tableResourceExists(ctx, adminClient, data.Table)
	if err != nil {
		return err
	}
	if exists {
		btc.Logger.Info("drop table: deleting bigtable table")
		err = adminClient.DeleteTable(ctx, data.Table)
	}
	if err != nil {
		return err
	}

	// this error behavior is done independently of the table resource existing or not because the schema mapping table is the SoT, not the table resource
	if len(rowKeysToDelete) == 0 && !data.IfExists {
		return fmt.Errorf("cannot delete table %s because it does not exist", data.Table)
	}

	// only reload schema mapping table if this operation changed it
	if len(rowKeysToDelete) > 0 {
		btc.Logger.Info("reloading schema mappings")
		err = btc.reloadSchemaMappings(ctx, data.Keyspace)
		if err != nil {
			return err
		}
	}
	return nil
}

func (btc *BigtableClient) createSchemaMappingTableMaybe(ctx context.Context, keyspace string) error {
	btc.Logger.Info("ensuring schema mapping table exists for keyspace", zap.String("schema_mapping_table", btc.SchemaMappingConfig.SchemaMappingTableName), zap.String("keyspace", keyspace))
	adminClient, err := btc.getAdminClient(keyspace)
	if err != nil {
		return err
	}

	// do a read to check if the table exists to save on admin API write quota
	exists, err := btc.tableResourceExists(ctx, adminClient, btc.SchemaMappingConfig.SchemaMappingTableName)
	if err != nil {
		return err
	}
	if !exists {
		btc.Logger.Info("schema mapping table not found. Automatically creating it...")
		err = adminClient.CreateTable(ctx, btc.SchemaMappingConfig.SchemaMappingTableName)
		if status.Code(err) == codes.AlreadyExists {
			// continue - maybe another Proxy instance raced, and created it instead
		} else if err != nil {
			btc.Logger.Error("failed to create schema mapping table", zap.Error(err))
			return err
		}
		btc.Logger.Info("schema mapping table created.")
	}

	err = adminClient.CreateColumnFamily(ctx, btc.SchemaMappingConfig.SchemaMappingTableName, schemaMappingTableColumnFamily)
	if status.Code(err) == codes.AlreadyExists {
		err = nil
	}
	if err != nil {
		return err
	}

	return nil
}

func (btc *BigtableClient) CreateTable(ctx context.Context, data *translator.CreateTableStatementMap) error {
	client, err := btc.getClient(data.Keyspace)
	if err != nil {
		return err
	}
	adminClient, err := btc.getAdminClient(data.Keyspace)
	if err != nil {
		return err
	}

	rowKeySchema, err := createBigtableRowKeySchema(data.PrimaryKeys, data.Columns, data.IntRowKeyEncoding)
	if err != nil {
		return err
	}

	columnFamilies, err := btc.addColumnFamilies(data.Columns)
	if err != nil {
		return err
	}

	// add default column family
	columnFamilies[btc.BigtableConfig.DefaultColumnFamily] = bigtable.Family{
		GCPolicy: bigtable.MaxVersionsPolicy(1),
	}

	// create the table conf first, here because it should exist before any reference to it in the schema mapping table is added, otherwise another concurrent request could try to load it and fail.
	btc.Logger.Info("creating bigtable table")
	err = adminClient.CreateTableFromConf(ctx, &bigtable.TableConf{
		TableID:        data.Table,
		ColumnFamilies: columnFamilies,
		RowKeySchema:   rowKeySchema,
	})
	// ignore already exists errors - the schema mapping table is the SoT
	if status.Code(err) == codes.AlreadyExists {
		err = nil
	} else if err != nil {
		btc.Logger.Error("failed to create bigtable table", zap.Error(err))
		return err
	}

	exists, err := btc.tableSchemaExists(ctx, client, data.Table)
	if err != nil {
		return err
	}

	if exists && !data.IfNotExists {
		return fmt.Errorf("cannot create table %s because it already exists", data.Table)
	}

	if !exists {
		btc.Logger.Info("updating table schema")
		err = btc.updateTableSchema(ctx, data.Keyspace, data.Table, data.PrimaryKeys, data.Columns, nil)
		if err != nil {
			return err
		}
		err = btc.reloadSchemaMappings(ctx, data.Keyspace)
		if err != nil {
			return err
		}
	}

	return nil
}

func (btc *BigtableClient) AlterTable(ctx context.Context, data *translator.AlterTableStatementMap) error {
	adminClient, err := btc.getAdminClient(data.Keyspace)
	if err != nil {
		return err
	}

	// passing nil in as pmks because we don't have access to them here and because primary keys can't be altered
	err = btc.updateTableSchema(ctx, data.Keyspace, data.Table, nil, data.AddColumns, data.DropColumns)
	if err != nil {
		return err
	}

	columnFamilies, err := btc.addColumnFamilies(data.AddColumns)
	if err != nil {
		return err
	}

	for family, config := range columnFamilies {
		err = adminClient.CreateColumnFamilyWithConfig(ctx, data.Table, family, config)
		if status.Code(err) == codes.AlreadyExists {
			// This can happen if the ALTER TABLE statement is run more than once.
			continue
		}

		if err != nil {
			return err
		}
	}

	btc.Logger.Info("reloading schema mappings")
	err = btc.reloadSchemaMappings(ctx, data.Keyspace)
	if err != nil {
		return err
	}

	return nil
}

func (btc *BigtableClient) addColumnFamilies(columns []types.CreateColumn) (map[string]bigtable.Family, error) {
	columnFamilies := make(map[string]bigtable.Family)
	for _, col := range columns {
		if !col.TypeInfo.IsCollection() && !col.TypeInfo.IsCounter() {
			continue
		}

		if col.Name == btc.BigtableConfig.DefaultColumnFamily {
			return nil, fmt.Errorf("counter and collection type columns cannot be named '%s' because it's reserved as the default column family", btc.BigtableConfig.DefaultColumnFamily)
		}

		if col.TypeInfo.IsCollection() {
			columnFamilies[col.Name] = bigtable.Family{
				GCPolicy: bigtable.MaxVersionsPolicy(1),
			}
		} else if col.TypeInfo.IsCounter() {
			columnFamilies[col.Name] = bigtable.Family{
				GCPolicy: bigtable.NoGcPolicy(),
				ValueType: bigtable.AggregateType{
					Input:      bigtable.Int64Type{},
					Aggregator: bigtable.SumAggregator{},
				},
			}
		}
	}
	return columnFamilies, nil
}

func (btc *BigtableClient) updateTableSchema(ctx context.Context, keyspace string, tableName string, pmks []translator.CreateTablePrimaryKeyConfig, addCols []types.CreateColumn, dropCols []string) error {
	client, err := btc.getClient(keyspace)
	if err != nil {
		return err
	}

	ts := bigtable.Now()
	var muts []*bigtable.Mutation
	var rowKeys []string
	sort.Slice(addCols, func(i, j int) bool {
		return addCols[i].Index < addCols[j].Index
	})
	for _, col := range addCols {
		mut := bigtable.NewMutation()
		mut.Set(schemaMappingTableColumnFamily, smColColumnName, ts, []byte(col.Name))
		mut.Set(schemaMappingTableColumnFamily, smColColumnType, ts, []byte(col.TypeInfo.RawType))
		isCollection := col.TypeInfo.IsCollection()
		// todo this is no longer used. We'll remove this later, in a few releases, but we'll keep it for now so users can roll back to earlier versions of the proxy if needed
		mut.Set(schemaMappingTableColumnFamily, smColIsCollection, ts, []byte(strconv.FormatBool(isCollection)))
		pmkIndex := slices.IndexFunc(pmks, func(c translator.CreateTablePrimaryKeyConfig) bool {
			return c.Name == col.Name
		})
		mut.Set(schemaMappingTableColumnFamily, smColIsPrimaryKey, ts, []byte(strconv.FormatBool(pmkIndex != -1)))
		if pmkIndex != -1 {
			pmkConfig := pmks[pmkIndex]
			mut.Set(schemaMappingTableColumnFamily, smColKeyType, ts, []byte(pmkConfig.KeyType))
		} else {
			// overkill, but overwrite any previous KeyType configs which could exist if the table was recreated with different columns
			mut.Set(schemaMappingTableColumnFamily, smColKeyType, ts, []byte(utilities.KEY_TYPE_REGULAR))
		}
		// +1 because we track PKPrecedence as 1 indexed for some reason
		mut.Set(schemaMappingTableColumnFamily, smColPKPrecedence, ts, []byte(strconv.Itoa(pmkIndex+1)))
		mut.Set(schemaMappingTableColumnFamily, smColTableName, ts, []byte(tableName))
		muts = append(muts, mut)
		rowKeys = append(rowKeys, tableName+"#"+col.Name)
	}
	// note: we only remove the column from the schema mapping table and don't actually delete any data from the data table
	for _, col := range dropCols {
		mut := bigtable.NewMutation()
		mut.DeleteRow()
		muts = append(muts, mut)
		rowKeys = append(rowKeys, tableName+"#"+col)
	}

	btc.Logger.Info("updating schema mapping table")
	table := client.Open(btc.SchemaMappingConfig.SchemaMappingTableName)
	_, err = table.ApplyBulk(ctx, rowKeys, muts)

	if err != nil {
		btc.Logger.Error("update schema mapping table failed")
		return err
	}

	return nil
}

// InsertRow - Inserts a row into the specified bigtable table.
//
// Parameters:
//   - ctx: Context for the operation, used for cancellation and deadlines.
//   - data: InsertQueryMapping object containing the table, row key, columns, values, and deleteColumnFamilies.
//
// Returns:
//   - error: Error if the insertion fails.
func (btc *BigtableClient) InsertRow(ctx context.Context, insertQueryData *translator.InsertQueryMapping) (*message.RowsResult, error) {
	return btc.mutateRow(ctx, insertQueryData.Table, insertQueryData.RowKey, insertQueryData.Columns, insertQueryData.Values, insertQueryData.DeleteColumnFamilies, []types.Column{}, insertQueryData.TimestampInfo.Timestamp, translator.IfSpec{IfNotExists: insertQueryData.IfNotExists}, insertQueryData.Keyspace, nil)
}

// UpdateRow - Updates a row in the specified bigtable table.
//
// Parameters:
//   - ctx: Context for the operation, used for cancellation and deadlines.
//   - data: UpdateQueryMapping object containing the table, row key, columns, values, and DeleteColumnFamilies.
//
// Returns:
//   - error: Error if the update fails.
func (btc *BigtableClient) UpdateRow(ctx context.Context, updateQueryData *translator.UpdateQueryMapping) (*message.RowsResult, error) {
	return btc.mutateRow(ctx, updateQueryData.Table, updateQueryData.RowKey, updateQueryData.Columns, updateQueryData.Values, updateQueryData.DeleteColumnFamilies, updateQueryData.DeleteColumQualifires, updateQueryData.TimestampInfo.Timestamp, translator.IfSpec{IfExists: updateQueryData.IfExists}, updateQueryData.Keyspace, updateQueryData.ComplexOperation)
}

// DeleteRowNew - Deletes a row in the specified bigtable table.
//
// Parameters:
//   - ctx: Context for the operation, used for cancellation and deadlines.
//   - queryData: DeleteQueryMapping object containing the table and row key.
//
// Returns:
//   - error: Error if the deletion fails.
func (btc *BigtableClient) DeleteRowNew(ctx context.Context, deleteQueryData *translator.DeleteQueryMapping) (*message.RowsResult, error) {
	otelgo.AddAnnotation(ctx, applyingDeleteMutation)
	client, err := btc.getClient(deleteQueryData.Keyspace)
	if err != nil {
		return nil, err
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

// ReadTableConfigs - Retrieves schema mapping configurations from the specified config table.
//
// Parameters:
//   - ctx: Context for the operation, used for cancellation and deadlines.
//   - schemaMappingTable: Name of the table containing the configuration.
//
// Returns:
//   - map[string]map[string]*Column: Table metadata.
//   - map[string][]Column: Primary key metadata.
//   - error: Error if the retrieval fails.
func (btc *BigtableClient) ReadTableConfigs(ctx context.Context, keyspace string) ([]*schemaMapping.TableConfig, error) {
	// if this is the first time we're getting table configs, ensure the schema mapping table exists
	if btc.SchemaMappingConfig == nil || btc.SchemaMappingConfig.CountTables() == 0 {
		err := btc.createSchemaMappingTableMaybe(ctx, keyspace)
		if err != nil {
			return nil, err
		}
	}

	otelgo.AddAnnotation(ctx, fetchingSchemaMappingConfig)
	client, err := btc.getClient(keyspace)
	if err != nil {
		return nil, err
	}

	table := client.Open(btc.SchemaMappingConfig.SchemaMappingTableName)
	filter := bigtable.LatestNFilter(1)

	allColumns := make(map[string][]*types.Column)

	var readErr error
	err = table.ReadRows(ctx, bigtable.InfiniteRange(""), func(row bigtable.Row) bool {
		// Extract the row key and column values
		var tableName, columnName, columnType, keyType string
		var isPrimaryKey bool
		var pkPrecedence int
		// Extract column values
		for _, item := range row[schemaMappingTableColumnFamily] {
			switch item.Column {
			case smFullQualifierTableName:
				tableName = string(item.Value)
			case smFullQualifierColumnName:
				columnName = string(item.Value)
			case smFullQualifierColumnType:
				columnType = string(item.Value)
			case smFullQualifierIsPrimaryKey:
				isPrimaryKey = string(item.Value) == "true"
			case smFullQualifierPKPrecedence:
				pkPrecedence, readErr = strconv.Atoi(string(item.Value))
				if readErr != nil {
					return false
				}
			case smFullQualifierKeyType:
				keyType = string(item.Value)
			}
		}
		dt, err := utilities.ParseCqlType(columnType)
		if err != nil {
			readErr = err
			return false
		}

		keyType = btc.parseKeyType(keyspace, tableName, columnName, keyType, pkPrecedence)

		// Create a new column struct
		column := &types.Column{
			Name:         columnName,
			TypeInfo:     dt,
			IsPrimaryKey: isPrimaryKey,
			PkPrecedence: pkPrecedence,
			KeyType:      keyType,
		}

		allColumns[tableName] = append(allColumns[tableName], column)

		return true
	}, bigtable.RowFilter(filter))

	// combine errors for simpler error handling
	if err == nil {
		err = readErr
	}

	if err != nil {
		btc.Logger.Error("Failed to read rows from bigtable - possible issue with schema_mapping table:", zap.Error(err))
		return nil, err
	}

	adminClient, err := btc.getAdminClient(keyspace)
	if err != nil {
		errorMessage := fmt.Sprintf("failed to get admin client for keyspace '%s'", keyspace)
		btc.Logger.Error(errorMessage, zap.Error(err))
		return nil, fmt.Errorf("%s: %w", errorMessage, err)
	}

	tableConfigs := make([]*schemaMapping.TableConfig, 0, len(allColumns))
	for tableName, tableColumns := range allColumns {
		var intRowKeyEncoding types.IntRowKeyEncodingType
		// if we already know what encoding the table has, just use that, so we don't have to do the extra network request
		if existingTable, err := btc.SchemaMappingConfig.GetTableConfig(keyspace, tableName); err == nil {
			intRowKeyEncoding = existingTable.IntRowKeyEncoding
		} else {
			btc.Logger.Info(fmt.Sprintf("loading table info for table %s.%s", keyspace, tableName))
			tableInfo, err := adminClient.TableInfo(ctx, tableName)
			if err != nil {
				return nil, err
			}
			intRowKeyEncoding = detectTableEncoding(tableInfo, types.OrderedCodeEncoding)
		}

		tableConfig := schemaMapping.NewTableConfig(keyspace, tableName, btc.BigtableConfig.DefaultColumnFamily, intRowKeyEncoding, tableColumns)
		tableConfigs = append(tableConfigs, tableConfig)
	}

	otelgo.AddAnnotation(ctx, schemaMappingConfigFetched)

	return tableConfigs, nil
}

// This parsing logic defaults the primary key types, because older versions of the proxy set keyType to an empty string. The only key type that we're guessing about is whether the key is a clustering key or not which, for the purposes of the proxy, doesn't matter.
func (btc *BigtableClient) parseKeyType(keyspace, table, column, keyType string, pkPrecedence int) string {
	keyType = strings.ToLower(keyType)

	// we used to write 'partition' for partition keys but Cassandra calls it 'partition_key'
	if keyType == "partition" {
		keyType = utilities.KEY_TYPE_PARTITION
	}

	if keyType == utilities.KEY_TYPE_PARTITION && pkPrecedence > 0 {
		return utilities.KEY_TYPE_PARTITION
	} else if keyType == utilities.KEY_TYPE_CLUSTERING && pkPrecedence > 1 {
		return utilities.KEY_TYPE_CLUSTERING
	} else if keyType == utilities.KEY_TYPE_REGULAR && pkPrecedence == 0 {
		return utilities.KEY_TYPE_REGULAR
	} else {
		// this is an unknown key type
		var defaultKeyType string
		if pkPrecedence <= 0 {
			// regular columns, that aren't part of the primary key have a precedence of 0.
			defaultKeyType = utilities.KEY_TYPE_REGULAR
		} else if pkPrecedence == 1 {
			// default to partition key because the first key is always a partition key.
			defaultKeyType = utilities.KEY_TYPE_PARTITION
		} else {
			// default to clustering key because clustering keys usually follow (this will be wrong in the case of a composite partition key), and it doesn't really matter for the purposes of the proxy.
			defaultKeyType = utilities.KEY_TYPE_CLUSTERING
		}
		btc.Logger.Warn(fmt.Sprintf("unknown key state KeyType='%s' and pkPrecedence of %d for %s.%s column %s. defaulting key type to '%s'", keyType, pkPrecedence,
			keyspace, table, column, defaultKeyType))

		return defaultKeyType
	}
}

// ApplyBulkMutation - Applies bulk mutations to the specified bigtable table.
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
		btc.Logger.Info("mutating row BULK", zap.String("key", hex.EncodeToString([]byte(md.RowKey))))
		if _, exists := rowKeyToMutationMap[md.RowKey]; !exists {
			rowKeyToMutationMap[md.RowKey] = bigtable.NewMutation()
		}
		mut := rowKeyToMutationMap[md.RowKey]
		switch md.MutationType {
		case mutationTypeInsert:
			{
				for _, column := range md.Columns {
					if md.Timestamp != 0 {
						mut.Set(column.ColumnFamily, column.Name, md.Timestamp, column.Contents)
					} else {
						mut.Set(column.ColumnFamily, column.Name, bigtable.Now(), column.Contents)
					}
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

	client, err := btc.getClient(keyspace)
	if err != nil {
		return BulkOperationResponse{
			FailedRows: "All Rows are failed",
		}, err
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

// Close() gracefully shuts down the bigtable client and gRPC connection.
//
// It iterates through all active bigtable clients and closes them before closing the gRPC connection.
func (btc *BigtableClient) Close() {
	for _, clients := range btc.Clients {
		clients.Close()
	}
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
func (btc *BigtableClient) getIndexOpTimestamp(ctx context.Context, tableName, rowKey, columnFamily, keyspace string, index int) (string, error) {
	client, err := btc.getClient(keyspace)
	if err != nil {
		return "", err
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

// setMutationforListDelete() applies deletions to specific list elements in a bigtable row.
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
func (btc *BigtableClient) setMutationforListDelete(ctx context.Context, tableName, rowKey, columnFamily, keyspace string, deleteList [][]byte, mut *bigtable.Mutation) error {
	client, err := btc.getClient(keyspace)
	if err != nil {
		return err
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

// LoadConfigs initializes the BigtableClient with the provided response handler
// and schema mapping configuration.
func (btc *BigtableClient) LoadConfigs(rh *responsehandler.TypeHandler, schemaConfig *schemaMapping.SchemaMappingConfig) {
	// Set the ResponseHandler for the BigtableClient to handle responses.
	btc.ResponseHandler = rh

	// Set the SchemaMappingConfig to define how data mapping will be handled in bigtable.
	btc.SchemaMappingConfig = schemaConfig
}

// PrepareStatement prepares a query for execution using the bigtable SQL client.
func (btc *BigtableClient) PrepareStatement(ctx context.Context, query rh.QueryMetadata) (*bigtable.PreparedStatement, error) {
	client, err := btc.getClient(query.KeyspaceName)
	if err != nil {
		return nil, err
	}

	paramTypes := make(map[string]bigtable.SQLType)
	for paramName, paramValue := range query.Params {
		// Infer SQLType from the Go type. This might need refinement based on schema.
		// For now, using a basic inference.
		// We need to pass the where clause to the prepare statement
		clause, _ := utilities.GetClauseByValue(query.Clauses, paramName)
		if clause.Operator == constants.MAP_CONTAINS_KEY || clause.Operator == constants.ARRAY_INCLUDES {
			paramTypes[paramName] = bigtable.BytesSQLType{}
			continue
		} else {
			sqlType, err := inferSQLType(paramValue)
			if err != nil {
				btc.Logger.Error("Failed to infer SQL type for parameter", zap.String("paramName", paramName), zap.Any("paramValue", paramValue), zap.Error(err))
				return nil, fmt.Errorf("failed to infer SQL type for parameter %s: %w", paramName, err)
			}
			paramTypes[paramName] = sqlType
		}
	}
	preparedStatement, err := client.PrepareStatement(ctx, query.Query, paramTypes)
	if err != nil {
		btc.Logger.Error("Failed to prepare statement", zap.String("query", query.Query), zap.Error(err))
		return nil, fmt.Errorf("failed to prepare statement: %w", err)
	}

	return preparedStatement, nil
}

// getClient retrieves a bigtable client for a given keyspace.
// It looks up the instance information associated with the keyspace in the InstancesMap,
// then returns the corresponding client from the Clients map.
//
// Parameters:
//   - keyspace: string representing the Cassandra keyspace name
//
// Returns:
//   - *bigtable.Client: the bigtable client instance associated with the keyspace
//   - error: returns an error if either the keyspace is not found in InstancesMap
//     or if no client exists for the corresponding bigtable instance
func (btc *BigtableClient) getClient(keyspace string) (*bigtable.Client, error) {
	instanceInfo, ok := btc.BigtableConfig.Instances[keyspace]
	if !ok {
		return nil, fmt.Errorf("keyspace not found: '%s'", keyspace)
	}
	client, ok := btc.Clients[instanceInfo.BigtableInstance]
	if !ok {
		return nil, fmt.Errorf("client not found for instance '%s' (from keyspace '%s')", instanceInfo.BigtableInstance, keyspace)
	}
	return client, nil
}

// getAdminClient retrieves the bigtable AdminClient associated with the given keyspace.
// It looks up the instance information for the provided keyspace in the InstancesMap,
// then fetches the corresponding AdminClient from the AdminClients map using the
// bigtable instance name. If the keyspace or the admin client is not found, it returns
// an error describing the missing resource.
//
// Parameters:
//   - keyspace: The keyspace name to look up.
//
// Returns:
//   - *bigtable.AdminClient: The admin client associated with the keyspace.
//   - error: An error if the keyspace or admin client is not found.
func (btc *BigtableClient) getAdminClient(keyspace string) (*bigtable.AdminClient, error) {
	instanceInfo, ok := btc.BigtableConfig.Instances[keyspace]
	if !ok {
		return nil, fmt.Errorf("keyspace not found: '%s'", keyspace)
	}
	Adminclient, ok := btc.AdminClients[instanceInfo.BigtableInstance]
	if !ok {
		return nil, fmt.Errorf("admin client not found for instance '%s' (from keyspace '%s')", instanceInfo.BigtableInstance, keyspace)
	}
	return Adminclient, nil
}
