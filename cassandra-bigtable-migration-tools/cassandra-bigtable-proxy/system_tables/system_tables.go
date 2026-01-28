package system_tables

import (
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/metadata"
)

var (
	SystemSchemaTableTables = metadata.NewTableConfig("system_schema", "tables", "na", types.OrderedCodeEncoding, []*types.Column{
		{Name: "keyspace_name", CQLType: types.TypeVarchar, KeyType: types.KeyTypePartition},
		{Name: "table_name", CQLType: types.TypeVarchar, KeyType: types.KeyTypeClustering},
		{Name: "additional_write_policy", CQLType: types.TypeVarchar},
		{Name: "bloom_filter_fp_chance", CQLType: types.TypeDouble},
		{Name: "caching", CQLType: types.NewMapType(types.TypeVarchar, types.TypeVarchar)},
		{Name: "flags", CQLType: types.NewListType(types.TypeVarchar)},
	})
	SystemSchemaTableKeyspace = metadata.NewTableConfig("system_schema", "keyspaces", "na", types.OrderedCodeEncoding, []*types.Column{
		{Name: "keyspace_name", CQLType: types.TypeVarchar, KeyType: types.KeyTypePartition},
		{Name: "durable_writes", CQLType: types.TypeBoolean},
		{Name: "replication", CQLType: types.NewMapType(types.TypeVarchar, types.TypeVarchar)},
	},
	)
	SystemSchemaTableColumns = metadata.NewTableConfig("system_schema", "columns", "na", types.OrderedCodeEncoding, []*types.Column{
		{Name: "keyspace_name", CQLType: types.TypeVarchar, KeyType: types.KeyTypePartition},
		{Name: "table_name", CQLType: types.TypeVarchar, KeyType: types.KeyTypeClustering},
		{Name: "column_name", CQLType: types.TypeVarchar, KeyType: types.KeyTypeClustering},
		{Name: "clustering_order", CQLType: types.TypeVarchar},
		{Name: "kind", CQLType: types.TypeVarchar},
		{Name: "position", CQLType: types.TypeInt},
		{Name: "type", CQLType: types.TypeVarchar},
	})
	SystemSchemaTableFunctions = metadata.NewTableConfig("system_schema", "functions", "na", types.OrderedCodeEncoding, []*types.Column{
		{Name: "keyspace_name", CQLType: types.TypeVarchar, KeyType: types.KeyTypePartition},
		{Name: "function_name", CQLType: types.TypeVarchar, KeyType: types.KeyTypeClustering},
		{Name: "argument_types", CQLType: types.NewListType(types.TypeVarchar)},
		{Name: "argument_names", CQLType: types.NewListType(types.TypeVarchar)},
		{Name: "body", CQLType: types.TypeVarchar},
		{Name: "called_on_null_input", CQLType: types.TypeBlob},
		{Name: "deterministic", CQLType: types.TypeBoolean},
		{Name: "language", CQLType: types.TypeVarchar},
		{Name: "monotonic", CQLType: types.TypeBoolean},
		{Name: "monotonic_on", CQLType: types.NewListType(types.TypeVarchar)},
		{Name: "return_type", CQLType: types.TypeVarchar},
	})
	SystemSchemaTableAggregates = metadata.NewTableConfig("system_schema", "aggregates", "na", types.OrderedCodeEncoding, []*types.Column{
		{Name: "keyspace_name", CQLType: types.TypeVarchar, KeyType: types.KeyTypePartition},
		{Name: "aggregate_name", CQLType: types.TypeVarchar, KeyType: types.KeyTypeClustering},
		{Name: "argument_types", CQLType: types.NewListType(types.TypeVarchar)},
		{Name: "deterministic", CQLType: types.TypeBoolean},
		{Name: "final_func", CQLType: types.TypeVarchar},
		{Name: "initcond", CQLType: types.TypeVarchar},
		{Name: "return_type", CQLType: types.TypeVarchar},
		{Name: "state_func", CQLType: types.TypeVarchar},
		{Name: "state_type", CQLType: types.TypeVarchar},
	})
	SystemSchemaTableTriggers = metadata.NewTableConfig("system_schema", "triggers", "na", types.OrderedCodeEncoding, []*types.Column{
		{Name: "keyspace_name", CQLType: types.TypeVarchar, KeyType: types.KeyTypePartition},
		{Name: "table_name", CQLType: types.TypeVarchar, KeyType: types.KeyTypeClustering},
		{Name: "trigger_name", CQLType: types.TypeVarchar, KeyType: types.KeyTypeClustering},
		{Name: "options", CQLType: types.NewMapType(types.TypeVarchar, types.TypeVarchar)},
	})
	SystemSchemaTableIndexes = metadata.NewTableConfig("system_schema", "indexes", "na", types.OrderedCodeEncoding, []*types.Column{
		{Name: "keyspace_name", CQLType: types.TypeVarchar, KeyType: types.KeyTypePartition},
		{Name: "table_name", CQLType: types.TypeVarchar, KeyType: types.KeyTypeClustering},
		{Name: "index_name", CQLType: types.TypeVarchar, KeyType: types.KeyTypeClustering},
		{Name: "kind", CQLType: types.TypeVarchar},
		{Name: "options", CQLType: types.NewMapType(types.TypeVarchar, types.TypeVarchar)},
	})
	SystemSchemaTableViews = metadata.NewTableConfig("system_schema", "views", "na", types.OrderedCodeEncoding, []*types.Column{
		{Name: "keyspace_name", CQLType: types.TypeVarchar, KeyType: types.KeyTypePartition},
		{Name: "view_name", CQLType: types.TypeVarchar, KeyType: types.KeyTypeClustering},
		{Name: "additional_write_policy", CQLType: types.TypeVarchar},
		{Name: "base_table_id", CQLType: types.TypeUuid},
		{Name: "base_table_name", CQLType: types.TypeVarchar},
		{Name: "bloom_filter_fp_chance", CQLType: types.TypeDouble},
		{Name: "caching", CQLType: types.NewMapType(types.TypeVarchar, types.TypeVarchar)},
		{Name: "cdc", CQLType: types.TypeBoolean},
		{Name: "comment", CQLType: types.TypeVarchar},
		{Name: "compaction", CQLType: types.NewMapType(types.TypeVarchar, types.TypeVarchar)},
		{Name: "compression", CQLType: types.NewMapType(types.TypeVarchar, types.TypeVarchar)},
		{Name: "crc_check_chance", CQLType: types.TypeDouble},
		{Name: "dclocal_read_repair_chance", CQLType: types.TypeDouble},
		{Name: "default_time_to_live", CQLType: types.TypeInt},
		{Name: "extensions", CQLType: types.NewMapType(types.TypeVarchar, types.TypeBlob)},
		{Name: "gc_grace_seconds", CQLType: types.TypeInt},
		{Name: "id", CQLType: types.TypeUuid},
		{Name: "include_all_columns", CQLType: types.TypeBoolean},
		{Name: "max_index_interval", CQLType: types.TypeInt},
		{Name: "memtable_flush_period_in_ms", CQLType: types.TypeInt},
		{Name: "min_index_interval", CQLType: types.TypeInt},
		{Name: "nodesync", CQLType: types.NewMapType(types.TypeVarchar, types.TypeVarchar)},
		{Name: "read_repair", CQLType: types.TypeVarchar},
		{Name: "read_repair_chance", CQLType: types.TypeDouble},
		{Name: "speculative_retry", CQLType: types.TypeVarchar},
		{Name: "version", CQLType: types.TypeInt},
		{Name: "where_clause", CQLType: types.TypeVarchar},
	})
	SystemSchemaTableTypes = metadata.NewTableConfig("system_schema", "types", "na", types.OrderedCodeEncoding, []*types.Column{
		{Name: "keyspace_name", CQLType: types.TypeVarchar, KeyType: types.KeyTypePartition},
		{Name: "type_name", CQLType: types.TypeVarchar, KeyType: types.KeyTypeClustering},
		{Name: "field_names", CQLType: types.NewListType(types.TypeVarchar)},
		{Name: "field_types", CQLType: types.NewListType(types.TypeVarchar)},
	})
	SystemVirtualSchemaTableKeyspaces = metadata.NewTableConfig("system_virtual_schema", "keyspaces", "na", types.OrderedCodeEncoding, []*types.Column{
		{Name: "keyspace_name", CQLType: types.TypeVarchar, KeyType: types.KeyTypePartition},
	})
	SystemVirtualSchemaTableTables = metadata.NewTableConfig("system_virtual_schema", "tables", "na", types.OrderedCodeEncoding, []*types.Column{
		{Name: "keyspace_name", CQLType: types.TypeVarchar, KeyType: types.KeyTypePartition},
		{Name: "table_name", CQLType: types.TypeVarchar, KeyType: types.KeyTypeClustering},
		{Name: "additional_write_policy", CQLType: types.TypeVarchar},
		{Name: "bloom_filter_fp_chance", CQLType: types.TypeDouble},
		{Name: "caching", CQLType: types.NewMapType(types.TypeVarchar, types.TypeVarchar)},
		{Name: "flags", CQLType: types.NewListType(types.TypeVarchar)},
	})
	SystemVirtualSchemaTableTypes = metadata.NewTableConfig("system_virtual_schema", "types", "na", types.OrderedCodeEncoding, []*types.Column{
		{Name: "keyspace_name", CQLType: types.TypeVarchar, KeyType: types.KeyTypePartition},
		{Name: "type_name", CQLType: types.TypeVarchar, KeyType: types.KeyTypeClustering},
		{Name: "comment", CQLType: types.TypeVarchar},
	})
	SystemVirtualSchemaTableColumns = metadata.NewTableConfig("system_virtual_schema", "columns", "na", types.OrderedCodeEncoding, []*types.Column{
		{Name: "keyspace_name", CQLType: types.TypeVarchar, KeyType: types.KeyTypePartition},
		{Name: "table_name", CQLType: types.TypeVarchar, KeyType: types.KeyTypeClustering},
		{Name: "column_name", CQLType: types.TypeVarchar, KeyType: types.KeyTypeClustering},
		{Name: "clustering_order", CQLType: types.TypeVarchar},
		{Name: "column_name_bytes", CQLType: types.TypeBlob},
		{Name: "kind", CQLType: types.TypeVarchar},
		{Name: "position", CQLType: types.TypeInt},
		{Name: "type", CQLType: types.TypeVarchar},
	})
	SystemTableLocal = metadata.NewTableConfig("system", "local", "na", types.OrderedCodeEncoding, []*types.Column{
		{Name: "key", CQLType: types.TypeVarchar, KeyType: types.KeyTypePartition},
		{Name: "rpc_address", CQLType: types.TypeInet},
		{Name: "data_center", CQLType: types.TypeVarchar},
		{Name: "dse_version", CQLType: types.TypeVarchar},
		{Name: "rack", CQLType: types.TypeVarchar},
		{Name: "tokens", CQLType: types.NewSetType(types.TypeVarchar)},
		{Name: "release_version", CQLType: types.TypeVarchar},
		{Name: "partitioner", CQLType: types.TypeVarchar},
		{Name: "cluster_name", CQLType: types.TypeVarchar},
		{Name: "cql_version", CQLType: types.TypeVarchar},
		{Name: "schema_version", CQLType: types.TypeUuid},
		{Name: "native_protocol_version", CQLType: types.TypeVarchar},
		{Name: "host_id", CQLType: types.TypeUuid},
	})
	SystemTablePeers = metadata.NewTableConfig("system", "peers", "na", types.OrderedCodeEncoding, []*types.Column{
		{Name: "peer", CQLType: types.TypeInet, KeyType: types.KeyTypePartition},
		{Name: "rpc_address", CQLType: types.TypeInet},
		{Name: "data_center", CQLType: types.TypeVarchar},
		{Name: "dse_version", CQLType: types.TypeVarchar},
		{Name: "rack", CQLType: types.TypeVarchar},
		{Name: "tokens", CQLType: types.NewSetType(types.TypeVarchar)},
		{Name: "release_version", CQLType: types.TypeVarchar},
		{Name: "schema_version", CQLType: types.TypeUuid},
		{Name: "host_id", CQLType: types.TypeUuid},
	})
	SystemTablePeersV2 = metadata.NewTableConfig("system", "peers_v2", "na", types.OrderedCodeEncoding, []*types.Column{
		{Name: "peer", CQLType: types.TypeInet, KeyType: types.KeyTypePartition},
		{Name: "peer_port", CQLType: types.TypeInt, KeyType: types.KeyTypeClustering},
		{Name: "rpc_address", CQLType: types.TypeInet},
		{Name: "data_center", CQLType: types.TypeText},
		{Name: "dse_version", CQLType: types.TypeVarchar},
		{Name: "rack", CQLType: types.TypeVarchar},
		{Name: "tokens", CQLType: types.NewSetType(types.TypeVarchar)},
		{Name: "release_version", CQLType: types.TypeText},
		{Name: "schema_version", CQLType: types.TypeUuid},
		{Name: "host_id", CQLType: types.TypeUuid},
		{Name: "native_address", CQLType: types.TypeInet},
		{Name: "native_port", CQLType: types.TypeInt},
		{Name: "preferred_ip", CQLType: types.TypeInet},
		{Name: "preferred_port", CQLType: types.TypeInt},
	})
	SystemTableSchemaKeyspaces = metadata.NewTableConfig("system", "schema_keyspaces", "na", types.OrderedCodeEncoding, []*types.Column{
		{Name: "keyspace_name", CQLType: types.TypeVarchar, KeyType: types.KeyTypePartition},
		{Name: "durable_writes", CQLType: types.TypeBoolean},
		{Name: "strategy_class", CQLType: types.TypeVarchar},
		{Name: "strategy_options", CQLType: types.TypeVarchar},
	})
	SystemTableSchemaColumnFamilies = metadata.NewTableConfig("system", "schema_columnfamilies", "na", types.OrderedCodeEncoding, []*types.Column{
		{Name: "keyspace_name", CQLType: types.TypeVarchar, KeyType: types.KeyTypePartition},
		{Name: "columnfamily_name", CQLType: types.TypeVarchar, KeyType: types.KeyTypeClustering},
		{Name: "bloom_filter_fp_chance", CQLType: types.TypeDouble},
		{Name: "caching", CQLType: types.TypeVarchar},
		{Name: "cf_id", CQLType: types.TypeUuid},
		{Name: "comment", CQLType: types.TypeVarchar},
		{Name: "compaction_strategy_class", CQLType: types.TypeVarchar},
		{Name: "compaction_strategy_options", CQLType: types.TypeVarchar},
		{Name: "comparator", CQLType: types.TypeVarchar},
		{Name: "compression_parameters", CQLType: types.TypeVarchar},
		{Name: "default_time_to_live", CQLType: types.TypeInt},
		{Name: "default_validator", CQLType: types.TypeVarchar},
		{Name: "dropped_columns", CQLType: types.NewMapType(types.TypeVarchar, types.TypeBigInt)},
		{Name: "gc_grace_seconds", CQLType: types.TypeInt},
		{Name: "is_dense", CQLType: types.TypeBoolean},
		{Name: "key_validator", CQLType: types.TypeVarchar},
		{Name: "local_read_repair_chance", CQLType: types.TypeDouble},
		{Name: "max_compaction_threshold", CQLType: types.TypeInt},
		{Name: "max_index_interval", CQLType: types.TypeInt},
		{Name: "memtable_flush_period_in_ms", CQLType: types.TypeInt},
		{Name: "min_compaction_threshold", CQLType: types.TypeInt},
		{Name: "min_index_interval", CQLType: types.TypeInt},
		{Name: "read_repair_chance", CQLType: types.TypeDouble},
		{Name: "speculative_retry", CQLType: types.TypeVarchar},
		{Name: "subcomparator", CQLType: types.TypeVarchar},
		{Name: "type", CQLType: types.TypeVarchar},
	})
	SystemTableSchemaColumns = metadata.NewTableConfig("system", "schema_columns", "na", types.OrderedCodeEncoding, []*types.Column{
		{Name: "keyspace_name", CQLType: types.TypeVarchar, KeyType: types.KeyTypePartition},
		{Name: "columnfamily_name", CQLType: types.TypeVarchar, KeyType: types.KeyTypeClustering},
		{Name: "column_name", CQLType: types.TypeVarchar, KeyType: types.KeyTypeClustering},
		{Name: "component_index", CQLType: types.TypeInt},
		{Name: "index_name", CQLType: types.TypeVarchar},
		{Name: "index_options", CQLType: types.TypeVarchar},
		{Name: "index_type", CQLType: types.TypeVarchar},
		{Name: "type", CQLType: types.TypeVarchar},
		{Name: "validator", CQLType: types.TypeVarchar},
	})
	SystemTableSchemaUserTypes = metadata.NewTableConfig("system", "schema_usertypes", "na", types.OrderedCodeEncoding, []*types.Column{
		{Name: "keyspace_name", CQLType: types.TypeVarchar, KeyType: types.KeyTypePartition},
		{Name: "type_name", CQLType: types.TypeVarchar, KeyType: types.KeyTypeClustering},
		{Name: "field_names", CQLType: types.NewListType(types.TypeVarchar)},
		{Name: "field_types", CQLType: types.NewListType(types.TypeVarchar)},
	})
)

func getSystemTableConfigs() []*metadata.TableSchema {
	return []*metadata.TableSchema{
		SystemSchemaTableTables,
		SystemSchemaTableKeyspace,
		SystemSchemaTableColumns,
		SystemSchemaTableFunctions,
		SystemSchemaTableAggregates,
		SystemSchemaTableTriggers,
		SystemSchemaTableIndexes,
		SystemSchemaTableViews,
		SystemSchemaTableTypes,
		SystemVirtualSchemaTableKeyspaces,
		SystemVirtualSchemaTableTables,
		SystemVirtualSchemaTableTypes,
		SystemVirtualSchemaTableColumns,
		SystemTableLocal,
		SystemTablePeers,
		SystemTablePeersV2,
		SystemTableSchemaKeyspaces,
		SystemTableSchemaColumnFamilies,
		SystemTableSchemaColumns,
		SystemTableSchemaUserTypes,
	}
}
