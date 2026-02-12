package metadata

import (
	"cloud.google.com/go/bigtable"
	"context"
	"fmt"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/utilities"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type MetadataStore struct {
	logger  *zap.Logger
	clients *types.BigtableClientManager
	schemas *SchemaMetadata
	config  *types.BigtableConfig
}

func NewMetadataStore(logger *zap.Logger, clients *types.BigtableClientManager, config *types.BigtableConfig) *MetadataStore {
	return &MetadataStore{logger: logger, clients: clients, config: config, schemas: NewSchemaMetadata(config.DefaultColumnFamily, nil)}
}

func (b *MetadataStore) Initialize(ctx context.Context) error {
	for keyspace := range b.config.Instances {
		err := b.createSchemaMappingTableMaybe(ctx, keyspace)
		if err != nil {
			return err
		}

		err = b.ReloadKeyspaceSchemas(ctx, keyspace)
		if err != nil {
			return err
		}
	}

	if b.config.MetadataRefreshInterval > 0 {
		go b.refreshLoop(ctx)
	} else {
		b.logger.Info("periodic metadata refresh disabled because interval is 0.")
	}
	return nil
}

func (b *MetadataStore) refreshLoop(ctx context.Context) {
	b.logger.Info("starting periodic metadata refresh loop", zap.Int("interval_seconds", b.config.MetadataRefreshInterval))
	ticker := time.NewTicker(time.Duration(b.config.MetadataRefreshInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			b.logger.Info("stopping periodic metadata refresh loop")
			return
		case <-ticker.C:
			b.logger.Debug("starting periodic metadata refresh")
			var wg sync.WaitGroup
			for keyspace := range b.config.Instances {
				wg.Add(1)
				go func(ks types.Keyspace) {
					defer wg.Done()
					err := b.ReloadKeyspaceSchemas(ctx, ks)
					if err != nil {
						b.logger.Error("failed to periodically reload keyspace schemas", zap.String("keyspace", string(ks)), zap.Error(err))
					}
				}(keyspace)
			}
			wg.Wait()
		}
	}
}

func (b *MetadataStore) Schemas() *SchemaMetadata {
	return b.schemas
}

const (
	schemaMappingTableColumnFamily = "cf"
	// Cassandra doesn't have a time dimension to their counters, so we need to
	// use the same time for all counters
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

type TableResult struct {
	Config *TableSchema
	Error  error
}

func (b *MetadataStore) readKeyspace(ctx context.Context, keyspace types.Keyspace) ([]*TableSchema, error) {
	client, err := b.clients.GetClient(keyspace)
	if err != nil {
		return nil, err
	}

	table := client.Open(string(b.config.SchemaMappingTable))
	filter := bigtable.LatestNFilter(1)

	tables := make(map[types.TableName][]*types.Column)

	var readErr error
	err = table.ReadRows(ctx, bigtable.InfiniteRange(""), func(row bigtable.Row) bool {
		// Extract the row key and column values
		var tableName types.TableName
		var columnName, columnType, keyType string
		var isPrimaryKey bool
		var pkPrecedence int
		// Extract column values
		for _, item := range row[schemaMappingTableColumnFamily] {
			switch item.Column {
			case smFullQualifierTableName:
				tableName = types.TableName(item.Value)
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
		dt, err := utilities.ParseCqlTypeString(columnType)
		if err != nil {
			readErr = err
			return false
		}

		parsedKeyType := b.parseKeyType(keyspace, tableName, columnName, keyType, pkPrecedence)

		// Create a new column struct
		column := &types.Column{
			Name:         types.ColumnName(columnName),
			CQLType:      dt,
			IsPrimaryKey: isPrimaryKey,
			PkPrecedence: pkPrecedence,
			KeyType:      parsedKeyType,
		}

		tables[tableName] = append(tables[tableName], column)

		return true
	}, bigtable.RowFilter(filter))

	// combine errors for simpler error handling
	if err == nil {
		err = readErr
	}

	if err != nil {
		b.logger.Error("Failed to read rows from bigtable - possible issue with schema_mapping table:", zap.Error(err))
		return nil, err
	}

	adminClient, err := b.clients.GetAdmin(keyspace)
	if err != nil {
		errorMessage := fmt.Sprintf("failed to get admin client for keyspace '%s'", keyspace)
		b.logger.Error(errorMessage, zap.Error(err))
		return nil, fmt.Errorf("%s: %w", errorMessage, err)
	}

	// Create a channel to collect results and errors from the goroutines.
	// Buffer size set to the total number of tables to avoid blocking senders.
	resultCh := make(chan TableResult, len(tables))
	var wg sync.WaitGroup
	for tableName, tableColumns := range tables {
		tn := tableName
		tc := tableColumns
		// if we already know what encoding the table has, just use that, so we don't have to do the extra network request
		if existingTable, err := b.schemas.GetTableSchema(keyspace, tableName); err == nil {
			tableConfig := NewTableConfig(keyspace, tn, b.config.DefaultColumnFamily, existingTable.IntRowKeyEncoding, tc)
			resultCh <- TableResult{Config: tableConfig, Error: nil}
			continue
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			b.logger.Info(fmt.Sprintf("loading table info for table %s.%s", keyspace, tn))
			tableInfo, err := adminClient.TableInfo(ctx, string(tn))
			if err != nil {
				// Send the error back through the channel
				resultCh <- TableResult{Error: fmt.Errorf("TableInfo failed for %s: %w", tn, err)}
				return
			}
			intRowKeyEncoding := detectTableEncoding(tableInfo, types.OrderedCodeEncoding)
			tableConfig := NewTableConfig(keyspace, tn, b.config.DefaultColumnFamily, intRowKeyEncoding, tc)
			resultCh <- TableResult{Config: tableConfig, Error: nil}
		}()
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	tableConfigs := make([]*TableSchema, 0, len(tables))
	for result := range resultCh {
		if result.Error != nil {
			return nil, result.Error
		}
		tableConfigs = append(tableConfigs, result.Config)
	}

	return tableConfigs, nil
}

// This parsing logic defaults the primary key types, because older versions of the proxy set keyType to an empty string. The only key type that we're guessing about is whether the key is a clustering key or not which, for the purposes of the proxy, doesn't matter.
func (b *MetadataStore) parseKeyType(keyspace types.Keyspace, table types.TableName, column, keyType string, pkPrecedence int) types.KeyType {
	keyType = strings.ToLower(keyType)

	// we used to write 'partition' for partition keys but Cassandra calls it 'partition_key'
	if keyType == "partition" {
		keyType = string(types.KeyTypePartition)
	}

	if keyType == string(types.KeyTypePartition) && pkPrecedence > 0 {
		return types.KeyTypePartition
	} else if keyType == string(types.KeyTypeClustering) && pkPrecedence > 1 {
		return types.KeyTypeClustering
	} else if keyType == string(types.KeyTypeRegular) && pkPrecedence == 0 {
		return types.KeyTypeRegular
	} else {
		// this is an unknown key type
		var defaultKeyType types.KeyType
		if pkPrecedence <= 0 {
			// regular columns, that aren't part of the primary key have a precedence of 0.
			defaultKeyType = types.KeyTypeRegular
		} else if pkPrecedence == 1 {
			// default to partition key because the first key is always a partition key.
			defaultKeyType = types.KeyTypePartition
		} else {
			// default to clustering key because clustering keys usually follow (this will be wrong in the case of a composite partition key), and it doesn't really matter for the purposes of the proxy.
			defaultKeyType = types.KeyTypeClustering
		}
		b.logger.Warn(fmt.Sprintf("unknown key state KeyType='%s' and pkPrecedence of %d for %s.%s column %s. defaulting key type to '%s'", keyType, pkPrecedence,
			keyspace, table, column, defaultKeyType))

		return defaultKeyType
	}
}

func (b *MetadataStore) updateTableSchema(ctx context.Context, keyspace types.Keyspace, tableName types.TableName, pmks []types.CreateTablePrimaryKeyConfig, addCols []types.CreateColumn, dropCols []types.ColumnName) error {
	client, err := b.clients.GetClient(keyspace)
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
		mut.Set(schemaMappingTableColumnFamily, smColColumnType, ts, []byte(col.TypeInfo.String()))
		isCollection := col.TypeInfo.IsCollection()
		// todo this is no longer used. We'll remove this later, in a few releases, but we'll keep it for now so users can roll back to earlier versions of the proxy if needed
		mut.Set(schemaMappingTableColumnFamily, smColIsCollection, ts, []byte(strconv.FormatBool(isCollection)))
		pmkIndex := slices.IndexFunc(pmks, func(c types.CreateTablePrimaryKeyConfig) bool {
			return c.Name == col.Name
		})
		mut.Set(schemaMappingTableColumnFamily, smColIsPrimaryKey, ts, []byte(strconv.FormatBool(pmkIndex != -1)))
		if pmkIndex != -1 {
			pmkConfig := pmks[pmkIndex]
			mut.Set(schemaMappingTableColumnFamily, smColKeyType, ts, []byte(pmkConfig.KeyType))
		} else {
			// overkill, but overwrite any previous KeyType configs which could exist if the table was recreated with different columns
			mut.Set(schemaMappingTableColumnFamily, smColKeyType, ts, []byte(types.KeyTypeRegular))
		}
		// +1 because we track PKPrecedence as 1 indexed for some reason
		mut.Set(schemaMappingTableColumnFamily, smColPKPrecedence, ts, []byte(strconv.Itoa(pmkIndex+1)))
		mut.Set(schemaMappingTableColumnFamily, smColTableName, ts, []byte(tableName))
		muts = append(muts, mut)
		rowKeys = append(rowKeys, string(tableName)+"#"+string(col.Name))
	}
	// note: we only remove the column from the schema mapping table and don't actually delete any data from the data table
	for _, col := range dropCols {
		mut := bigtable.NewMutation()
		mut.DeleteRow()
		muts = append(muts, mut)
		rowKeys = append(rowKeys, fmt.Sprintf("%s#%s", tableName, col))
	}

	b.logger.Info("updating schema mapping table")
	table := client.Open(string(b.config.SchemaMappingTable))
	_, err = table.ApplyBulk(ctx, rowKeys, muts)

	if err != nil {
		b.logger.Error("update schema mapping table failed")
		return err
	}

	return nil
}

func (b *MetadataStore) AlterTable(ctx context.Context, data *types.AlterTableStatementMap) (*message.SchemaChangeResult, error) {
	adminClient, err := b.clients.GetAdmin(data.Keyspace())
	if err != nil {
		return nil, err
	}

	// passing nil in as pmks because we don't have access to them here and because primary keys can't be altered
	err = b.updateTableSchema(ctx, data.Keyspace(), data.Table(), nil, data.AddColumns, data.DropColumns)
	if err != nil {
		return nil, err
	}

	columnFamilies, err := b.addColumnFamilies(data.AddColumns)
	if err != nil {
		return nil, err
	}

	for family, config := range columnFamilies {
		err = adminClient.CreateColumnFamilyWithConfig(ctx, string(data.Table()), family, config)
		if status.Code(err) == codes.AlreadyExists {
			// This can happen if the ALTER TABLE statement is run more than once.
			continue
		}

		if err != nil {
			return nil, err
		}
	}

	b.logger.Info("reloading schema mappings")
	err = b.ReloadKeyspaceSchemas(ctx, data.Keyspace())
	if err != nil {
		return nil, err
	}
	return &message.SchemaChangeResult{
		ChangeType: primitive.SchemaChangeTypeUpdated,
		Target:     primitive.SchemaChangeTargetTable,
		Keyspace:   string(data.Keyspace()),
		Object:     string(data.Table()),
		Arguments:  nil,
	}, nil
}

func (b *MetadataStore) DropTable(ctx context.Context, data *types.DropTableQuery) (*message.SchemaChangeResult, error) {
	_, err := b.schemas.GetTableSchema(data.Keyspace(), data.Table())
	if err != nil && !data.IfExists {
		return nil, err
	}

	client, err := b.clients.GetClient(data.Keyspace())
	if err != nil {
		return nil, err
	}
	adminClient, err := b.clients.GetAdmin(data.Keyspace())
	if err != nil {
		return nil, err
	}

	// first clean up table from schema mapping table because that'b the SoT
	tbl := client.Open(string(b.config.SchemaMappingTable))
	var deleteMuts []*bigtable.Mutation
	var rowKeysToDelete []string
	err = tbl.ReadRows(ctx, bigtable.PrefixRange(string(data.Table())+"#"), func(row bigtable.Row) bool {
		mut := bigtable.NewMutation()
		mut.DeleteRow()
		deleteMuts = append(deleteMuts, mut)
		rowKeysToDelete = append(rowKeysToDelete, row.Key())
		return true
	})

	if err != nil {
		return nil, err
	}

	b.logger.Info("drop table: deleting schema rows")
	_, err = tbl.ApplyBulk(ctx, rowKeysToDelete, deleteMuts)
	if err != nil {
		return nil, err
	}

	// do a read to check if the table exists to save on admin API write quota
	exists, err := b.tableResourceExists(ctx, data.Keyspace(), data.Table())
	if err != nil {
		return nil, err
	}
	if exists {
		b.logger.Info("drop table: deleting bigtable table")
		err = adminClient.DeleteTable(ctx, string(data.Table()))
	}
	if err != nil {
		return nil, err
	}

	// this error behavior is done independently of the table resource existing or not because the schema mapping table is the SoT, not the table resource
	if len(rowKeysToDelete) == 0 && !data.IfExists {
		return nil, fmt.Errorf("cannot delete table %s because it does not exist", data.Table())
	}

	// only reload schema mapping table if this operation changed it
	if len(rowKeysToDelete) > 0 {
		b.logger.Info("reloading schema mappings")
		err = b.ReloadKeyspaceSchemas(ctx, data.Keyspace())
		if err != nil {
			return nil, err
		}
	}
	return &message.SchemaChangeResult{
		ChangeType: primitive.SchemaChangeTypeDropped,
		Target:     primitive.SchemaChangeTargetTable,
		Keyspace:   string(data.Keyspace()),
		Object:     string(data.Table()),
		Arguments:  nil,
	}, nil
}

func (b *MetadataStore) createSchemaMappingTableMaybe(ctx context.Context, keyspace types.Keyspace) error {
	b.logger.Info("ensuring schema mapping table exists for keyspace", zap.String("schema_mapping_table", string(b.config.SchemaMappingTable)), zap.String("keyspace", string(keyspace)))
	adminClient, err := b.clients.GetAdmin(keyspace)
	if err != nil {
		return err
	}

	// do a read to check if the table exists to save on admin API write quota
	exists, err := b.tableResourceExists(ctx, keyspace, b.config.SchemaMappingTable)
	if err != nil {
		return err
	}
	if !exists {
		b.logger.Info("schema mapping table not found. Automatically creating it...")
		err = adminClient.CreateTable(ctx, string(b.config.SchemaMappingTable))
		if status.Code(err) == codes.AlreadyExists {
			// continue - maybe another Proxy instance raced, and created it instead
		} else if err != nil {
			b.logger.Error("failed to create schema mapping table", zap.Error(err))
			return err
		}
		b.logger.Info("schema mapping table created.")
	}

	err = adminClient.CreateColumnFamily(ctx, string(b.config.SchemaMappingTable), schemaMappingTableColumnFamily)
	if status.Code(err) == codes.AlreadyExists {
		err = nil
	}
	if err != nil {
		return err
	}

	return nil
}

func (b *MetadataStore) CreateTable(ctx context.Context, data *types.CreateTableStatementMap) (*message.SchemaChangeResult, error) {
	client, err := b.clients.GetClient(data.Keyspace())
	if err != nil {
		return nil, err
	}
	adminClient, err := b.clients.GetAdmin(data.Keyspace())
	if err != nil {
		return nil, err
	}

	rowKeySchema, err := createBigtableRowKeySchema(data.PrimaryKeys, data.Columns, data.IntRowKeyEncoding)
	if err != nil {
		return nil, err
	}

	columnFamilies, err := b.addColumnFamilies(data.Columns)
	if err != nil {
		return nil, err
	}

	// add default column family
	columnFamilies[string(b.config.DefaultColumnFamily)] = bigtable.Family{
		GCPolicy: bigtable.MaxVersionsPolicy(1),
	}

	// create the table conf first, here because it should exist before any reference to it in the schema mapping table is added, otherwise another concurrent request could try to load it and fail.
	b.logger.Info("creating bigtable table")
	err = adminClient.CreateTableFromConf(ctx, &bigtable.TableConf{
		TableID:        string(data.Table()),
		ColumnFamilies: columnFamilies,
		RowKeySchema:   rowKeySchema,
	})
	// ignore already exists errors - the schema mapping table is the SoT
	if status.Code(err) == codes.AlreadyExists {
		err = nil
	} else if err != nil {
		b.logger.Error("failed to create bigtable table", zap.Error(err))
		return nil, err
	}

	exists, err := b.tableSchemaExists(ctx, client, data.Table())
	if err != nil {
		return nil, err
	}

	if exists && !data.IfNotExists {
		return nil, fmt.Errorf("cannot create table %s because it already exists", data.Table())
	}

	if !exists {
		b.logger.Info("updating table schema")
		err = b.updateTableSchema(ctx, data.Keyspace(), data.Table(), data.PrimaryKeys, data.Columns, nil)
		if err != nil {
			return nil, err
		}
		err = b.ReloadKeyspaceSchemas(ctx, data.Keyspace())
		if err != nil {
			return nil, err
		}
	}

	return &message.SchemaChangeResult{
		ChangeType: primitive.SchemaChangeTypeCreated,
		Target:     primitive.SchemaChangeTargetTable,
		Keyspace:   string(data.Keyspace()),
		Object:     string(data.Table()),
		Arguments:  nil,
	}, nil
}

func (b *MetadataStore) addColumnFamilies(columns []types.CreateColumn) (map[string]bigtable.Family, error) {
	columnFamilies := make(map[string]bigtable.Family)
	for _, col := range columns {
		if !col.TypeInfo.IsCollection() && col.TypeInfo.Code() != types.COUNTER {
			continue
		}

		if string(col.Name) == string(b.config.DefaultColumnFamily) {
			return nil, fmt.Errorf("counter and collection type columns cannot be named '%s' because it's reserved as the default column family", b.config.DefaultColumnFamily)
		}

		if col.TypeInfo.IsCollection() {
			columnFamilies[string(col.Name)] = bigtable.Family{
				GCPolicy: bigtable.MaxVersionsPolicy(1),
			}
		} else if col.TypeInfo.Code() == types.COUNTER {
			columnFamilies[string(col.Name)] = bigtable.Family{
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

func (b *MetadataStore) tableResourceExists(ctx context.Context, keyspace types.Keyspace, table types.TableName) (bool, error) {
	adminClient, err := b.clients.GetAdmin(keyspace)
	if err != nil {
		return false, err
	}
	_, err = adminClient.TableInfo(ctx, string(table))
	// todo figure out which error message is for table doesn't exist yet or find better check
	if status.Code(err) == codes.NotFound || status.Code(err) == codes.InvalidArgument {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// scan the schema mapping table to determine if the table exists
func (b *MetadataStore) tableSchemaExists(ctx context.Context, client *bigtable.Client, tableName types.TableName) (bool, error) {
	table := client.Open(string(b.config.SchemaMappingTable))
	exists := false
	err := table.ReadRows(ctx, bigtable.PrefixRange(string(tableName)+"#"), func(row bigtable.Row) bool {
		exists = true
		return false
	}, bigtable.LimitRows(1))
	return exists, err
}

func (b *MetadataStore) ReloadKeyspaceSchemas(ctx context.Context, keyspace types.Keyspace) error {
	b.logger.Info("reloading keyspace schemas", zap.String("keyspace", string(keyspace)))
	tableConfigs, err := b.readKeyspace(ctx, keyspace)
	if err != nil {
		return fmt.Errorf("error when reloading schema mappings for %s.%s: %w", keyspace, b.config.SchemaMappingTable, err)
	}
	changeCount := b.schemas.SyncKeyspace(keyspace, tableConfigs)
	b.logger.Info("successfully synced keyspace schemas", zap.String("keyspace", string(keyspace)), zap.Int("changes", changeCount))
	return nil
}
