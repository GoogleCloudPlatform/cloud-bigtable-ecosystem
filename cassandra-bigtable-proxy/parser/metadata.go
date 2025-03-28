// Copyright (c) DataStax, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package parser

import (
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/message"
)

var (
	SystemLocalColumns = []*message.ColumnMetadata{
		{Keyspace: "system", Table: "local", Name: "key", Type: datatype.Varchar},
		{Keyspace: "system", Table: "local", Name: "rpc_address", Type: datatype.Inet},
		{Keyspace: "system", Table: "local", Name: "data_center", Type: datatype.Varchar},
		{Keyspace: "system", Table: "local", Name: "rack", Type: datatype.Varchar},
		{Keyspace: "system", Table: "local", Name: "tokens", Type: datatype.NewSetType(datatype.Varchar)},
		{Keyspace: "system", Table: "local", Name: "release_version", Type: datatype.Varchar},
		{Keyspace: "system", Table: "local", Name: "partitioner", Type: datatype.Varchar},
		{Keyspace: "system", Table: "local", Name: "cluster_name", Type: datatype.Varchar},
		{Keyspace: "system", Table: "local", Name: "cql_version", Type: datatype.Varchar},
		{Keyspace: "system", Table: "local", Name: "schema_version", Type: datatype.Uuid},
		{Keyspace: "system", Table: "local", Name: "native_protocol_version", Type: datatype.Varchar},
		{Keyspace: "system", Table: "local", Name: "host_id", Type: datatype.Uuid},
	}

	DseSystemLocalColumns = []*message.ColumnMetadata{
		{Keyspace: "system", Table: "local", Name: "key", Type: datatype.Varchar},
		{Keyspace: "system", Table: "local", Name: "rpc_address", Type: datatype.Inet},
		{Keyspace: "system", Table: "local", Name: "data_center", Type: datatype.Varchar},
		// The column "dse_version" is important for some DSE advance workloads esp. for graph to determine the graph
		// language.
		{Keyspace: "system", Table: "local", Name: "dse_version", Type: datatype.Varchar}, // DSE only
		{Keyspace: "system", Table: "local", Name: "rack", Type: datatype.Varchar},
		{Keyspace: "system", Table: "local", Name: "tokens", Type: datatype.NewSetType(datatype.Varchar)},
		{Keyspace: "system", Table: "local", Name: "release_version", Type: datatype.Varchar},
		{Keyspace: "system", Table: "local", Name: "partitioner", Type: datatype.Varchar},
		{Keyspace: "system", Table: "local", Name: "cluster_name", Type: datatype.Varchar},
		{Keyspace: "system", Table: "local", Name: "cql_version", Type: datatype.Varchar},
		{Keyspace: "system", Table: "local", Name: "schema_version", Type: datatype.Uuid},
		{Keyspace: "system", Table: "local", Name: "native_protocol_version", Type: datatype.Varchar},
		{Keyspace: "system", Table: "local", Name: "host_id", Type: datatype.Uuid},
	}

	SystemPeersColumns = []*message.ColumnMetadata{
		{Keyspace: "system", Table: "peers", Name: "peer", Type: datatype.Inet},
		{Keyspace: "system", Table: "peers", Name: "rpc_address", Type: datatype.Inet},
		{Keyspace: "system", Table: "peers", Name: "data_center", Type: datatype.Varchar},
		{Keyspace: "system", Table: "peers", Name: "rack", Type: datatype.Varchar},
		{Keyspace: "system", Table: "peers", Name: "tokens", Type: datatype.NewSetType(datatype.Varchar)},
		{Keyspace: "system", Table: "peers", Name: "release_version", Type: datatype.Varchar},
		{Keyspace: "system", Table: "peers", Name: "schema_version", Type: datatype.Uuid},
		{Keyspace: "system", Table: "peers", Name: "host_id", Type: datatype.Uuid},
	}

	DseSystemPeersColumns = []*message.ColumnMetadata{
		{Keyspace: "system", Table: "peers", Name: "peer", Type: datatype.Inet},
		{Keyspace: "system", Table: "peers", Name: "rpc_address", Type: datatype.Inet},
		{Keyspace: "system", Table: "peers", Name: "data_center", Type: datatype.Varchar},
		{Keyspace: "system", Table: "peers", Name: "dse_version", Type: datatype.Varchar}, // DSE only
		{Keyspace: "system", Table: "peers", Name: "rack", Type: datatype.Varchar},
		{Keyspace: "system", Table: "peers", Name: "tokens", Type: datatype.NewSetType(datatype.Varchar)},
		{Keyspace: "system", Table: "peers", Name: "release_version", Type: datatype.Varchar},
		{Keyspace: "system", Table: "peers", Name: "schema_version", Type: datatype.Uuid},
		{Keyspace: "system", Table: "peers", Name: "host_id", Type: datatype.Uuid},
	}

	SystemSchemaKeyspaces = []*message.ColumnMetadata{
		{Keyspace: "system", Table: "schema_keyspaces", Name: "keyspace_name", Type: datatype.Varchar},
		{Keyspace: "system", Table: "schema_keyspaces", Name: "durable_writes", Type: datatype.Boolean},
		{Keyspace: "system", Table: "schema_keyspaces", Name: "strategy_class", Type: datatype.Varchar},
		{Keyspace: "system", Table: "schema_keyspaces", Name: "strategy_options", Type: datatype.Varchar},
	}

	SystemSchemaColumnFamilies = []*message.ColumnMetadata{
		{Keyspace: "system", Table: "schema_columnfamilies", Name: "keyspace_name", Type: datatype.Varchar},
		{Keyspace: "system", Table: "schema_columnfamilies", Name: "columnfamily_name", Type: datatype.Varchar},
		{Keyspace: "system", Table: "schema_columnfamilies", Name: "bloom_filter_fp_chance", Type: datatype.Double},
		{Keyspace: "system", Table: "schema_columnfamilies", Name: "caching", Type: datatype.Varchar},
		{Keyspace: "system", Table: "schema_columnfamilies", Name: "cf_id", Type: datatype.Uuid},
		{Keyspace: "system", Table: "schema_columnfamilies", Name: "comment", Type: datatype.Varchar},
		{Keyspace: "system", Table: "schema_columnfamilies", Name: "compaction_strategy_class", Type: datatype.Varchar},
		{Keyspace: "system", Table: "schema_columnfamilies", Name: "compaction_strategy_options", Type: datatype.Varchar},
		{Keyspace: "system", Table: "schema_columnfamilies", Name: "comparator", Type: datatype.Varchar},
		{Keyspace: "system", Table: "schema_columnfamilies", Name: "compression_parameters", Type: datatype.Varchar},
		{Keyspace: "system", Table: "schema_columnfamilies", Name: "default_time_to_live", Type: datatype.Int},
		{Keyspace: "system", Table: "schema_columnfamilies", Name: "default_validator", Type: datatype.Varchar},
		{Keyspace: "system", Table: "schema_columnfamilies", Name: "dropped_columns", Type: datatype.NewMapType(datatype.Varchar, datatype.Bigint)},
		{Keyspace: "system", Table: "schema_columnfamilies", Name: "gc_grace_seconds", Type: datatype.Int},
		{Keyspace: "system", Table: "schema_columnfamilies", Name: "is_dense", Type: datatype.Boolean},
		{Keyspace: "system", Table: "schema_columnfamilies", Name: "key_validator", Type: datatype.Varchar},
		{Keyspace: "system", Table: "schema_columnfamilies", Name: "local_read_repair_chance", Type: datatype.Double},
		{Keyspace: "system", Table: "schema_columnfamilies", Name: "max_compaction_threshold", Type: datatype.Int},
		{Keyspace: "system", Table: "schema_columnfamilies", Name: "max_index_interval", Type: datatype.Int},
		{Keyspace: "system", Table: "schema_columnfamilies", Name: "memtable_flush_period_in_ms", Type: datatype.Int},
		{Keyspace: "system", Table: "schema_columnfamilies", Name: "min_compaction_threshold", Type: datatype.Int},
		{Keyspace: "system", Table: "schema_columnfamilies", Name: "min_index_interval", Type: datatype.Int},
		{Keyspace: "system", Table: "schema_columnfamilies", Name: "read_repair_chance", Type: datatype.Double},
		{Keyspace: "system", Table: "schema_columnfamilies", Name: "speculative_retry", Type: datatype.Varchar},
		{Keyspace: "system", Table: "schema_columnfamilies", Name: "subcomparator", Type: datatype.Varchar},
		{Keyspace: "system", Table: "schema_columnfamilies", Name: "type", Type: datatype.Varchar},
	}

	SystemSchemaColumns = []*message.ColumnMetadata{
		{Keyspace: "system", Table: "schema_columns", Name: "keyspace_name", Type: datatype.Varchar},
		{Keyspace: "system", Table: "schema_columns", Name: "columnfamily_name", Type: datatype.Varchar},
		{Keyspace: "system", Table: "schema_columns", Name: "column_name", Type: datatype.Varchar},
		{Keyspace: "system", Table: "schema_columns", Name: "component_index", Type: datatype.Int},
		{Keyspace: "system", Table: "schema_columns", Name: "index_name", Type: datatype.Varchar},
		{Keyspace: "system", Table: "schema_columns", Name: "index_options", Type: datatype.Varchar},
		{Keyspace: "system", Table: "schema_columns", Name: "index_type", Type: datatype.Varchar},
		{Keyspace: "system", Table: "schema_columns", Name: "type", Type: datatype.Varchar},
		{Keyspace: "system", Table: "schema_columns", Name: "validator", Type: datatype.Varchar},
	}

	SystemSchemaUsertypes = []*message.ColumnMetadata{
		{Keyspace: "system", Table: "schema_usertypes", Name: "keyspace_name", Type: datatype.Varchar},
		{Keyspace: "system", Table: "schema_usertypes", Name: "type_name", Type: datatype.Varchar},
		{Keyspace: "system", Table: "schema_usertypes", Name: "field_names", Type: datatype.NewListType(datatype.Varchar)},
		{Keyspace: "system", Table: "schema_usertypes", Name: "field_types", Type: datatype.NewListType(datatype.Varchar)},
	}
)

var SystemColumnsByName = map[string][]*message.ColumnMetadata{
	"local":                 SystemLocalColumns,
	"peers":                 SystemPeersColumns,
	"schema_keyspaces":      SystemSchemaKeyspaces,
	"schema_columnfamilies": SystemSchemaColumnFamilies,
	"schema_columns":        SystemSchemaColumns,
	"schema_usertypes":      SystemSchemaUsertypes,
}

func FindColumnMetadata(columns []*message.ColumnMetadata, name string) *message.ColumnMetadata {
	for _, column := range columns {
		if column.Name == name {
			return column
		}
	}
	return nil
}

var SystemSchemaTablesColumns = []*message.ColumnMetadata{
	{Keyspace: "system_schema", Table: "tables", Name: "keyspace_name", Type: datatype.Varchar, Index: 0},
	{Keyspace: "system_schema", Table: "tables", Name: "table_name", Type: datatype.Varchar, Index: 1},
	{Keyspace: "system_schema", Table: "tables", Name: "additional_write_policy", Type: datatype.Varchar, Index: 2},
	{Keyspace: "system_schema", Table: "tables", Name: "bloom_filter_fp_chance", Type: datatype.Double, Index: 3},
	{Keyspace: "system_schema", Table: "tables", Name: "caching", Type: datatype.NewMapType(datatype.Varchar, datatype.Varchar), Index: 4},
	{Keyspace: "system_schema", Table: "tables", Name: "flags", Type: datatype.NewSetType(datatype.Varchar), Index: 5},
}

var SystemSchemaKeyspacesColumns = []*message.ColumnMetadata{
	{Keyspace: "system_schema", Table: "keyspaces", Name: "keyspace_name", Type: datatype.Varchar, Index: 0},
	{Keyspace: "system_schema", Table: "keyspaces", Name: "durable_writes", Type: datatype.Boolean, Index: 1},
	{Keyspace: "system_schema", Table: "keyspaces", Name: "replication", Type: datatype.NewMapType(datatype.Varchar, datatype.Varchar), Index: 3},
}

var SystemSchemaColumnsColumns = []*message.ColumnMetadata{
	{Keyspace: "system_schema", Table: "columns", Name: "keyspace_name", Type: datatype.Varchar, Index: 0},
	{Keyspace: "system_schema", Table: "columns", Name: "table_name", Type: datatype.Varchar, Index: 1},
	{Keyspace: "system_schema", Table: "columns", Name: "column_name", Type: datatype.Varchar, Index: 2},
	{Keyspace: "system_schema", Table: "columns", Name: "clustering_order", Type: datatype.Varchar, Index: 3},
	{Keyspace: "system_schema", Table: "columns", Name: "kind", Type: datatype.Varchar, Index: 4},
	{Keyspace: "system_schema", Table: "columns", Name: "position", Type: datatype.Int, Index: 5},
	{Keyspace: "system_schema", Table: "columns", Name: "type", Type: datatype.Varchar, Index: 6},
}

var SystemSchemaFunctionsColumns = []*message.ColumnMetadata{
	{Keyspace: "system_schema", Table: "functions", Name: "keyspace_name", Type: datatype.Varchar},
	{Keyspace: "system_schema", Table: "functions", Name: "function_name", Type: datatype.Varchar},
	{Keyspace: "system_schema", Table: "functions", Name: "argument_types", Type: datatype.NewListType(datatype.Varchar)},
	{Keyspace: "system_schema", Table: "functions", Name: "argument_names", Type: datatype.NewListType(datatype.Varchar)},
	{Keyspace: "system_schema", Table: "functions", Name: "body", Type: datatype.Varchar},
	{Keyspace: "system_schema", Table: "functions", Name: "called_on_null_input", Type: datatype.Blob},
	{Keyspace: "system_schema", Table: "functions", Name: "deterministic", Type: datatype.Boolean},
	{Keyspace: "system_schema", Table: "functions", Name: "language", Type: datatype.Varchar},
	{Keyspace: "system_schema", Table: "functions", Name: "monotonic", Type: datatype.Boolean},
	{Keyspace: "system_schema", Table: "functions", Name: "monotonic_on", Type: datatype.NewListType(datatype.Varchar)},
	{Keyspace: "system_schema", Table: "functions", Name: "return_type", Type: datatype.Varchar},
}

var SystemSchemaAggregatesColumns = []*message.ColumnMetadata{
	{Keyspace: "system_schema", Table: "functions", Name: "keyspace_name", Type: datatype.Varchar},
	{Keyspace: "system_schema", Table: "functions", Name: "aggregate_name", Type: datatype.Varchar},
	{Keyspace: "system_schema", Table: "functions", Name: "argument_types", Type: datatype.NewListType(datatype.Varchar)},
	{Keyspace: "system_schema", Table: "functions", Name: "deterministic", Type: datatype.Boolean},
	{Keyspace: "system_schema", Table: "functions", Name: "final_func", Type: datatype.Varchar},
	{Keyspace: "system_schema", Table: "functions", Name: "initcond", Type: datatype.Varchar},
	{Keyspace: "system_schema", Table: "functions", Name: "return_type", Type: datatype.Varchar},
	{Keyspace: "system_schema", Table: "functions", Name: "state_func", Type: datatype.Varchar},
	{Keyspace: "system_schema", Table: "functions", Name: "state_type", Type: datatype.Varchar},
}

var SystemSchemaTriggersColumns = []*message.ColumnMetadata{
	{Keyspace: "system_schema", Table: "functions", Name: "keyspace_name", Type: datatype.Varchar},
	{Keyspace: "system_schema", Table: "functions", Name: "table_name", Type: datatype.Varchar},
	{Keyspace: "system_schema", Table: "functions", Name: "trigger_name", Type: datatype.Varchar},
	{Keyspace: "system_schema", Table: "functions", Name: "options", Type: datatype.NewMapType(datatype.Varchar, datatype.Varchar)},
}

var SystemSchemaIndexesColumns = []*message.ColumnMetadata{
	{Keyspace: "system_schema", Table: "functions", Name: "keyspace_name", Type: datatype.Varchar},
	{Keyspace: "system_schema", Table: "functions", Name: "table_name", Type: datatype.Varchar},
	{Keyspace: "system_schema", Table: "functions", Name: "index_name", Type: datatype.Varchar},
	{Keyspace: "system_schema", Table: "functions", Name: "kind", Type: datatype.Varchar},
	{Keyspace: "system_schema", Table: "functions", Name: "options", Type: datatype.NewMapType(datatype.Varchar, datatype.Varchar)},
}

var SystemSchemaViewsColumns = []*message.ColumnMetadata{
	{Keyspace: "system_schema", Table: "functions", Name: "keyspace_name", Type: datatype.Varchar},
	{Keyspace: "system_schema", Table: "functions", Name: "view_name", Type: datatype.Varchar},
	{Keyspace: "system_schema", Table: "functions", Name: "additional_write_policy", Type: datatype.Varchar},
	{Keyspace: "system_schema", Table: "functions", Name: "base_table_id", Type: datatype.Uuid},
	{Keyspace: "system_schema", Table: "functions", Name: "base_table_name", Type: datatype.Varchar},
	{Keyspace: "system_schema", Table: "functions", Name: "bloom_filter_fp_chance", Type: datatype.Double},
	{Keyspace: "system_schema", Table: "functions", Name: "caching", Type: datatype.NewMapType(datatype.Varchar, datatype.Varchar)},
	{Keyspace: "system_schema", Table: "functions", Name: "cdc", Type: datatype.Boolean},
	{Keyspace: "system_schema", Table: "functions", Name: "comment", Type: datatype.Varchar},

	{Keyspace: "system_schema", Table: "functions", Name: "compaction", Type: datatype.NewMapType(datatype.Varchar, datatype.Varchar)},
	{Keyspace: "system_schema", Table: "functions", Name: "compression", Type: datatype.NewMapType(datatype.Varchar, datatype.Varchar)},
	{Keyspace: "system_schema", Table: "functions", Name: "crc_check_chance", Type: datatype.Double},
	{Keyspace: "system_schema", Table: "functions", Name: "dclocal_read_repair_chance", Type: datatype.Double},
	{Keyspace: "system_schema", Table: "functions", Name: "default_time_to_live", Type: datatype.Int},
	{Keyspace: "system_schema", Table: "functions", Name: "extensions", Type: datatype.NewMapType(datatype.Varchar, datatype.Blob)},
	{Keyspace: "system_schema", Table: "functions", Name: "gc_grace_seconds", Type: datatype.Int},
	{Keyspace: "system_schema", Table: "functions", Name: "id", Type: datatype.Uuid},
	{Keyspace: "system_schema", Table: "functions", Name: "include_all_columns", Type: datatype.Boolean},
	{Keyspace: "system_schema", Table: "functions", Name: "max_index_interval", Type: datatype.Int},
	{Keyspace: "system_schema", Table: "functions", Name: "memtable_flush_period_in_ms", Type: datatype.Int},
	{Keyspace: "system_schema", Table: "functions", Name: "min_index_interval", Type: datatype.Int},

	{Keyspace: "system_schema", Table: "functions", Name: "nodesync", Type: datatype.NewMapType(datatype.Varchar, datatype.Varchar)},
	{Keyspace: "system_schema", Table: "functions", Name: "read_repair", Type: datatype.Varchar},
	{Keyspace: "system_schema", Table: "functions", Name: "read_repair_chance", Type: datatype.Double},
	{Keyspace: "system_schema", Table: "functions", Name: "speculative_retry", Type: datatype.Varchar},
	{Keyspace: "system_schema", Table: "functions", Name: "version", Type: datatype.Int},
	{Keyspace: "system_schema", Table: "functions", Name: "where_clause", Type: datatype.Varchar},
}

var SystemSchemaTypesColumns = []*message.ColumnMetadata{
	{Keyspace: "system_schema", Table: "types", Name: "keyspace_name", Type: datatype.Varchar},
	{Keyspace: "system_schema", Table: "types", Name: "type_name", Type: datatype.Varchar},
	{Keyspace: "system_schema", Table: "types", Name: "field_names", Type: datatype.NewListType(datatype.Varchar)},
	{Keyspace: "system_schema", Table: "types", Name: "field_types", Type: datatype.NewListType(datatype.Varchar)},
}
var SystemSchematablesColumn = map[string][]*message.ColumnMetadata{
	"tables":     SystemSchemaTablesColumns,
	"keyspaces":  SystemSchemaKeyspacesColumns,
	"columns":    SystemSchemaColumnsColumns,
	"functions":  SystemSchemaFunctionsColumns,
	"aggregates": SystemSchemaAggregatesColumns,
	"triggers":   SystemSchemaTriggersColumns,
	"indexes":    SystemSchemaIndexesColumns,
	"views":      SystemSchemaViewsColumns,
	"types":      SystemSchemaTypesColumns,
}

var SystemSchemaColumnsRow = map[string][][]interface{}{
	"columns": {
		{"system_schema", "columns", "kind", "none", "partition_key", 0, "text"},
	},
}

var SystemVirtualSchemaKeyspaces = []*message.ColumnMetadata{
	{Keyspace: "system_virtual_schema", Table: "keyspaces", Name: "keyspace_name", Type: datatype.Varchar},
}

var SystemVirtualSchemaTables = []*message.ColumnMetadata{
	{Keyspace: "system_virtual_schema", Table: "tables", Name: "keyspace_name", Type: datatype.Varchar},
	{Keyspace: "system_virtual_schema", Table: "tables", Name: "table_name", Type: datatype.Varchar},
	{Keyspace: "system_virtual_schema", Table: "tables", Name: "comment", Type: datatype.Varchar},
}

var SystemVirtualSchemaColumns = []*message.ColumnMetadata{
	{Keyspace: "system_virtual_schema", Table: "columns", Name: "keyspace_name", Type: datatype.Varchar},
	{Keyspace: "system_virtual_schema", Table: "columns", Name: "table_name", Type: datatype.Varchar},
	{Keyspace: "system_virtual_schema", Table: "columns", Name: "column_name", Type: datatype.Varchar},
	{Keyspace: "system_virtual_schema", Table: "columns", Name: "clustering_order", Type: datatype.Varchar},
	{Keyspace: "system_virtual_schema", Table: "columns", Name: "column_name_bytes", Type: datatype.Blob},
	{Keyspace: "system_virtual_schema", Table: "columns", Name: "kind", Type: datatype.Varchar},
	{Keyspace: "system_virtual_schema", Table: "columns", Name: "position", Type: datatype.Int},
	{Keyspace: "system_virtual_schema", Table: "columns", Name: "type", Type: datatype.Varchar},
}

var SystemVirtualSchemaColumn = map[string][]*message.ColumnMetadata{
	"keyspaces": SystemVirtualSchemaKeyspaces,
	"tables":    SystemVirtualSchemaTables,
	"columns":   SystemVirtualSchemaColumns,
}
