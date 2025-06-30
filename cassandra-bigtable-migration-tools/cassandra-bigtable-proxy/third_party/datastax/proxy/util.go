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

package proxy

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	types "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/responsehandler"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/third_party/datastax/parser"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/translator"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/utilities"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
)

type TimeTrackInfo struct {
	// bigtableStart and bigtableEnd represent time taken by whole function including data encoding to cassandra
	start, bigtableStart, bigtableEnd time.Time
	ResponseProcessingTime            time.Duration
}

type SystemQueryMetadataCache struct {
	KeyspaceSystemQueryMetadataCache map[primitive.ProtocolVersion][]message.Row
	TableSystemQueryMetadataCache    map[primitive.ProtocolVersion][]message.Row
	ColumnsSystemQueryMetadataCache  map[primitive.ProtocolVersion][]message.Row
}

// Compile a regular expression that matches the WHERE clause with at least one space before and after it.
// The regex uses case-insensitive matching and captures everything after the WHERE clause.
// \s+ matches one or more spaces before and after WHERE.
// (.+) captures everything after WHERE and its trailing spaces.
var whereRegex = regexp.MustCompile(`(?i)\s+WHERE\s+(.+)`)

// addSecondsToCurrentTimestamp takes a number of seconds as input
// and returns the current Unix timestamp plus the input time in seconds.
func addSecondsToCurrentTimestamp(seconds int64) string {
	// Get the current time
	currentTime := time.Now()

	// Add the input seconds to the current time
	futureTime := currentTime.Add(time.Second * time.Duration(seconds))

	// Return the future time as a Unix timestamp (in seconds)
	return unixToISO(futureTime.Unix())
}

// unixToISO converts a Unix timestamp (in seconds) to an ISO 8601 formatted string.
func unixToISO(unixTimestamp int64) string {
	// Convert the Unix timestamp to a time.Time object
	t := time.Unix(unixTimestamp, 0).UTC()

	// Format the time as an ISO 8601 string
	return t.Format(time.RFC3339)
}

// ReplaceLimitValue replaces the limit placeholder in the query string with an actual value
// if the limit is specified in the query parameters.
func ReplaceLimitValue(query responsehandler.QueryMetadata) (responsehandler.QueryMetadata, error) {
	if query.Limit.IsLimit {
		if val, exists := query.Params[limitValue]; exists {
			if val, ok := val.(int64); ok {
				if val <= 0 {
					return query, fmt.Errorf("LIMIT must be strictly positive")
				}
				query.Query = strings.ReplaceAll(query.Query, "@"+limitValue, strconv.FormatInt(val, 10))
				delete(query.Params, limitValue)
				return query, nil
			} else {
				return query, fmt.Errorf("LIMIT must be strictly positive")
			}
		} else {
			return query, fmt.Errorf("limit values does not exist")
		}
	}
	return query, nil
}

// GetSystemQueryMetadataCache converts structured metadata rows into a SystemQueryMetadataCache.
// It encodes keyspace, table, and column metadata into a format compatible with Cassandra system queries.
//
// Parameters:
// - keyspaceMetadataRows: Slice of keyspace metadata in [][]interface{} format.
// - tableMetadataRows: Slice of table metadata in [][]interface{} format.
// - columnsMetadataRows: Slice of column metadata in [][]interface{} format.
//
// Returns:
// - *SystemQueryMetadataCache: A pointer to a structured metadata cache containing keyspaces, tables, and columns.
// - error: Returns an error if metadata conversion fails at any step.
func getSystemQueryMetadataCache(keyspaceMetadataRows, tableMetadataRows, columnsMetadataRows [][]interface{}) (*SystemQueryMetadataCache, error) {
	var err error
	protocolIV := primitive.ProtocolVersion4

	systemQueryMetadataCache := &SystemQueryMetadataCache{
		KeyspaceSystemQueryMetadataCache: make(map[primitive.ProtocolVersion][]message.Row),
		TableSystemQueryMetadataCache:    make(map[primitive.ProtocolVersion][]message.Row),
		ColumnsSystemQueryMetadataCache:  make(map[primitive.ProtocolVersion][]message.Row),
	}

	systemQueryMetadataCache.KeyspaceSystemQueryMetadataCache[protocolIV], err = responsehandler.BuildResponseForSystemQueries(keyspaceMetadataRows, protocolIV)
	if err != nil {
		return systemQueryMetadataCache, err
	}
	systemQueryMetadataCache.TableSystemQueryMetadataCache[protocolIV], err = responsehandler.BuildResponseForSystemQueries(tableMetadataRows, protocolIV)
	if err != nil {
		return systemQueryMetadataCache, err
	}
	systemQueryMetadataCache.ColumnsSystemQueryMetadataCache[protocolIV], err = responsehandler.BuildResponseForSystemQueries(columnsMetadataRows, protocolIV)
	if err != nil {
		return systemQueryMetadataCache, err
	}

	return systemQueryMetadataCache, nil
}

// getKeyspaceMetadata converts table metadata into keyspace metadata rows
func getKeyspaceMetadata(tableMetadata map[string]map[string]map[string]*types.Column) [][]interface{} {
	// Replication settings for system and example keyspaces, matching Cassandra output
	replicationMap := map[string]map[string]string{
		"system":                {"class": "org.apache.cassandra.locator.LocalStrategy"},
		"system_schema":         {"class": "org.apache.cassandra.locator.LocalStrategy"},
		"system_auth":           {"class": "org.apache.cassandra.locator.SimpleStrategy", "replication_factor": "1"},
		"system_distributed":    {"class": "org.apache.cassandra.locator.SimpleStrategy", "replication_factor": "3"},
		"system_traces":         {"class": "org.apache.cassandra.locator.SimpleStrategy", "replication_factor": "2"},
		"system_views":          {"class": "org.apache.cassandra.locator.SimpleStrategy", "replication_factor": "1"},
		"system_virtual_schema": {"class": "org.apache.cassandra.locator.LocalStrategy"},
	}

	keyspaceMetadataRows := [][]interface{}{}
	for keyspace := range tableMetadata {
		repl := map[string]string{"class": "org.apache.cassandra.locator.SimpleStrategy", "replication_factor": "1"}
		if val, ok := replicationMap[keyspace]; ok {
			repl = val
		}
		// Add keyspace metadata
		keyspaceMetadataRows = append(keyspaceMetadataRows, []interface{}{
			keyspace, true, repl,
		})
	}
	return keyspaceMetadataRows
}

// getTableMetadata converts table metadata into table metadata rows
func getTableMetadata(tableMetadata map[string]map[string]map[string]*types.Column) [][]interface{} {
	tableMetadataRows := [][]interface{}{}
	for keyspace, tables := range tableMetadata {
		for tableName := range tables {
			// Add table metadata
			tableMetadataRows = append(tableMetadataRows, []interface{}{
				keyspace, tableName, "99p", 0.01, map[string]string{
					"keys":               "ALL",
					"rows_per_partition": "NONE",
				},
				[]string{"compound"},
			})
		}
	}
	return tableMetadataRows
}

// getColumnMetadata converts table metadata into column metadata rows
func getColumnMetadata(tableMetadata map[string]map[string]map[string]*types.Column) [][]interface{} {
	columnsMetadataRows := [][]interface{}{}
	for keyspace, tables := range tableMetadata {
		for tableName, columns := range tables {
			for columnName, column := range columns {
				kind := utilities.KEY_TYPE_REGULAR
				if column.IsPrimaryKey {
					kind = utilities.KEY_TYPE_PARTITION
					if column.KeyType == utilities.KEY_TYPE_CLUSTERING {
						kind = utilities.KEY_TYPE_CLUSTERING
					}
				}
				// Add column metadata
				columnsMetadataRows = append(columnsMetadataRows, []interface{}{
					keyspace, tableName, columnName, "none", kind, 0, column.CQLType,
				})
			}
		}
	}
	return columnsMetadataRows
}

// ConstructSystemMetadataRows constructs system metadata rows for keyspaces, tables, and columns.
// It iterates through the provided table metadata and formats the data into a Cassandra-compatible structure.
// The resulting metadata is used for system queries in the Bigtable proxy.
//
// Parameters:
//   - tableMetadata: A nested map where the first level represents keyspaces, the second level represents tables,
//     and the third level represents columns within each table.
//
// Returns:
// - A pointer to a SystemQueryMetadataCache, which contains structured metadata for keyspaces, tables, and columns.
// - An error if any issue occurs while building the metadata cache.
func ConstructSystemMetadataRows(tableMetadata map[string]map[string]map[string]*types.Column) (*SystemQueryMetadataCache, error) {
	// Initialize the metadata map if it's nil
	if tableMetadata == nil {
		tableMetadata = make(map[string]map[string]map[string]*types.Column)
	}

	// Add system keyspaces to metadata
	addSystemKeyspacesToMetadata(tableMetadata)

	// Get metadata for system keyspaces
	keyspaceMetadata := getKeyspaceMetadata(tableMetadata)
	tableMetadataRows := getTableMetadata(tableMetadata)
	columnMetadata := getColumnMetadata(tableMetadata)

	// Get system query metadata cache
	systemQueryMetadataCache := &SystemQueryMetadataCache{
		KeyspaceSystemQueryMetadataCache: make(map[primitive.ProtocolVersion][]message.Row),
		TableSystemQueryMetadataCache:    make(map[primitive.ProtocolVersion][]message.Row),
		ColumnsSystemQueryMetadataCache:  make(map[primitive.ProtocolVersion][]message.Row),
	}

	protocolIV := primitive.ProtocolVersion4
	var err error

	systemQueryMetadataCache.KeyspaceSystemQueryMetadataCache[protocolIV], err = responsehandler.BuildResponseForSystemQueries(keyspaceMetadata, protocolIV)
	if err != nil {
		return nil, err
	}

	systemQueryMetadataCache.TableSystemQueryMetadataCache[protocolIV], err = responsehandler.BuildResponseForSystemQueries(tableMetadataRows, protocolIV)
	if err != nil {
		return nil, err
	}

	systemQueryMetadataCache.ColumnsSystemQueryMetadataCache[protocolIV], err = responsehandler.BuildResponseForSystemQueries(columnMetadata, protocolIV)
	if err != nil {
		return nil, err
	}

	return systemQueryMetadataCache, nil
}

// Add system keyspaces, tables, and columns to the schema mapping before system cache construction
func addSystemKeyspacesToMetadata(tableMetadata map[string]map[string]map[string]*types.Column) {
	// Replication settings for system and example keyspaces, matching Cassandra output
	replicationMap := map[string]map[string]string{
		"system":                {"class": "org.apache.cassandra.locator.LocalStrategy"},
		"system_schema":         {"class": "org.apache.cassandra.locator.LocalStrategy"},
		"system_auth":           {"class": "org.apache.cassandra.locator.SimpleStrategy", "replication_factor": "1"},
		"system_distributed":    {"class": "org.apache.cassandra.locator.SimpleStrategy", "replication_factor": "3"},
		"system_traces":         {"class": "org.apache.cassandra.locator.SimpleStrategy", "replication_factor": "2"},
		"system_views":          {"class": "org.apache.cassandra.locator.SimpleStrategy", "replication_factor": "1"},
		"system_virtual_schema": {"class": "org.apache.cassandra.locator.LocalStrategy"},
	}

	for ks := range replicationMap {
		if _, exists := tableMetadata[ks]; !exists {
			tableMetadata[ks] = make(map[string]map[string]*types.Column)
		}
		// Add system_schema tables
		if ks == "system_schema" {
			// Add tables table
			if _, exists := tableMetadata[ks]["tables"]; !exists {
				tableMetadata[ks]["tables"] = make(map[string]*types.Column)
			}
			for _, col := range parser.SystemSchemaTablesColumns {
				tableMetadata[ks]["tables"][col.Name] = &types.Column{
					ColumnName:   col.Name,
					CQLType:      col.Type,
					IsPrimaryKey: col.Name == "keyspace_name" || col.Name == "table_name",
					KeyType: func() string {
						if col.Name == "keyspace_name" || col.Name == "table_name" {
							return utilities.KEY_TYPE_PARTITION
						}
						return utilities.KEY_TYPE_REGULAR
					}(),
				}
			}

			// Add columns table
			if _, exists := tableMetadata[ks]["columns"]; !exists {
				tableMetadata[ks]["columns"] = make(map[string]*types.Column)
			}
			for _, col := range parser.SystemSchemaColumnsColumns {
				tableMetadata[ks]["columns"][col.Name] = &types.Column{
					ColumnName:   col.Name,
					CQLType:      col.Type,
					IsPrimaryKey: col.Name == "keyspace_name" || col.Name == "table_name" || col.Name == "column_name",
					KeyType: func() string {
						if col.Name == "keyspace_name" || col.Name == "table_name" || col.Name == "column_name" {
							return utilities.KEY_TYPE_PARTITION
						}
						return utilities.KEY_TYPE_REGULAR
					}(),
				}
			}

			// Add keyspaces table
			if _, exists := tableMetadata[ks]["keyspaces"]; !exists {
				tableMetadata[ks]["keyspaces"] = make(map[string]*types.Column)
			}
			for _, col := range parser.SystemSchemaKeyspacesColumns {
				tableMetadata[ks]["keyspaces"][col.Name] = &types.Column{
					ColumnName:   col.Name,
					CQLType:      col.Type,
					IsPrimaryKey: col.Name == "keyspace_name",
					KeyType: func() string {
						if col.Name == "keyspace_name" {
							return utilities.KEY_TYPE_PARTITION
						}
						return utilities.KEY_TYPE_REGULAR
					}(),
				}
			}
		}
		// Add system tables
		if ks == "system" {
			// Add local table
			if _, exists := tableMetadata[ks]["local"]; !exists {
				tableMetadata[ks]["local"] = make(map[string]*types.Column)
			}
			// Add columns for system.local
			columns := []struct {
				Name string
				Type datatype.DataType
			}{
				{"key", datatype.Varchar},
				{"bootstrapped", datatype.Varchar},
				{"broadcast_address", datatype.Inet},
				{"cluster_name", datatype.Varchar},
				{"cql_version", datatype.Varchar},
				{"data_center", datatype.Varchar},
				{"gossip_generation", datatype.Int},
				{"host_id", datatype.Uuid},
				{"listen_address", datatype.Inet},
				{"native_protocol_version", datatype.Varchar},
				{"partitioner", datatype.Varchar},
				{"rack", datatype.Varchar},
				{"release_version", datatype.Varchar},
				{"rpc_address", datatype.Inet},
				{"schema_version", datatype.Uuid},
				{"thrift_version", datatype.Varchar},
				{"tokens", datatype.NewSetType(datatype.Varchar)},
			}
			for _, col := range columns {
				tableMetadata[ks]["local"][col.Name] = &types.Column{
					ColumnName:   col.Name,
					CQLType:      col.Type,
					IsPrimaryKey: col.Name == "key",
					KeyType: func() string {
						if col.Name == "key" {
							return utilities.KEY_TYPE_PARTITION
						}
						return utilities.KEY_TYPE_REGULAR
					}(),
				}
			}

			// Add peers table
			if _, exists := tableMetadata[ks]["peers"]; !exists {
				tableMetadata[ks]["peers"] = make(map[string]*types.Column)
			}
			// Add columns for system.peers
			peerColumns := []struct {
				Name string
				Type datatype.DataType
			}{
				{"peer", datatype.Inet},
				{"data_center", datatype.Varchar},
				{"host_id", datatype.Uuid},
				{"preferred_ip", datatype.Inet},
				{"rack", datatype.Varchar},
				{"release_version", datatype.Varchar},
				{"rpc_address", datatype.Inet},
				{"schema_version", datatype.Uuid},
				{"tokens", datatype.NewSetType(datatype.Varchar)},
			}
			for _, col := range peerColumns {
				tableMetadata[ks]["peers"][col.Name] = &types.Column{
					ColumnName:   col.Name,
					CQLType:      col.Type,
					IsPrimaryKey: col.Name == "peer",
					KeyType: func() string {
						if col.Name == "peer" {
							return utilities.KEY_TYPE_PARTITION
						}
						return utilities.KEY_TYPE_REGULAR
					}(),
				}
			}

			// Add peers_v2 table
			if _, exists := tableMetadata[ks]["peers_v2"]; !exists {
				tableMetadata[ks]["peers_v2"] = make(map[string]*types.Column)
			}
			// Add columns for system.peers_v2
			peerV2Columns := []struct {
				Name string
				Type datatype.DataType
			}{
				{"peer", datatype.Inet},
				{"data_center", datatype.Varchar},
				{"host_id", datatype.Uuid},
				{"native_address", datatype.Inet},
				{"native_port", datatype.Int},
				{"preferred_ip", datatype.Inet},
				{"rack", datatype.Varchar},
				{"release_version", datatype.Varchar},
				{"rpc_address", datatype.Inet},
				{"schema_version", datatype.Uuid},
				{"tokens", datatype.NewSetType(datatype.Varchar)},
			}
			for _, col := range peerV2Columns {
				tableMetadata[ks]["peers_v2"][col.Name] = &types.Column{
					ColumnName:   col.Name,
					CQLType:      col.Type,
					IsPrimaryKey: col.Name == "peer",
					KeyType: func() string {
						if col.Name == "peer" {
							return utilities.KEY_TYPE_PARTITION
						}
						return utilities.KEY_TYPE_REGULAR
					}(),
				}
			}
		}
	}

	// Add system_virtual_schema tables and columns
	if _, exists := tableMetadata["system_virtual_schema"]; !exists {
		tableMetadata["system_virtual_schema"] = make(map[string]map[string]*types.Column)
	}
	// keyspaces
	tableMetadata["system_virtual_schema"]["keyspaces"] = make(map[string]*types.Column)
	for _, col := range parser.SystemVirtualSchemaKeyspaces {
		tableMetadata["system_virtual_schema"]["keyspaces"][col.Name] = &types.Column{
			ColumnName:   col.Name,
			CQLType:      col.Type,
			IsPrimaryKey: false,
			KeyType:      utilities.KEY_TYPE_REGULAR,
		}
	}
	// tables
	tableMetadata["system_virtual_schema"]["tables"] = make(map[string]*types.Column)
	for _, col := range parser.SystemVirtualSchemaTables {
		tableMetadata["system_virtual_schema"]["tables"][col.Name] = &types.Column{
			ColumnName:   col.Name,
			CQLType:      col.Type,
			IsPrimaryKey: false,
			KeyType:      utilities.KEY_TYPE_REGULAR,
		}
	}
	// columns
	tableMetadata["system_virtual_schema"]["columns"] = make(map[string]*types.Column)
	for _, col := range parser.SystemVirtualSchemaColumns {
		tableMetadata["system_virtual_schema"]["columns"][col.Name] = &types.Column{
			ColumnName:   col.Name,
			CQLType:      col.Type,
			IsPrimaryKey: false,
			KeyType:      utilities.KEY_TYPE_REGULAR,
		}
	}
}

// getTimestampMetadata appends a metadata entry for a timestamp column to a list of column metadata
// if a timestamp is used in the insert query.
//
// Parameters:
//   - insertQueryMetadata: An InsertQueryMapping containing information about the insert query, including
//     any timestamp information.
//   - columnMetadataList: A slice of pointers to ColumnMetadata representing the current list of column metadata.
//
// Returns: An updated slice of pointers to ColumnMetadata, including an entry for the timestamp column
//
//	if the query uses a timestamp.
func getTimestampMetadata(insertQueryMetadata translator.InsertQueryMapping, columnMetadataList []*message.ColumnMetadata) []*message.ColumnMetadata {
	if insertQueryMetadata.TimestampInfo.HasUsingTimestamp {
		metadata := message.ColumnMetadata{
			Keyspace: insertQueryMetadata.Keyspace,
			Table:    insertQueryMetadata.Table,
			Name:     TimestampColumnName,
			Index:    insertQueryMetadata.TimestampInfo.Index,
			Type:     datatype.Bigint,
		}
		columnMetadataList = append(columnMetadataList, &metadata)
	}
	return columnMetadataList
}

// getTimestampMetadataForUpdate prepends a metadata entry for a timestamp column to a list of column metadata
// if a timestamp is used in the update query.
//
// Parameters:
//   - updateQueryMetadata: An UpdateQueryMapping containing information about the update query, including
//     any timestamp information.
//   - columnMetadataList: A slice of pointers to ColumnMetadata representing the current list of column metadata.
//
// Returns: An updated slice of pointers to ColumnMetadata, with an entry for the timestamp column prepended
//
//	if the query uses a timestamp.
func getTimestampMetadataForUpdate(updateQueryMetadata translator.UpdateQueryMapping, columnMetadataList []*message.ColumnMetadata) []*message.ColumnMetadata {
	if updateQueryMetadata.TimestampInfo.HasUsingTimestamp {
		metadata := message.ColumnMetadata{
			Keyspace: updateQueryMetadata.Keyspace,
			Table:    updateQueryMetadata.Table,
			Name:     TimestampColumnName,
			Index:    updateQueryMetadata.TimestampInfo.Index,
			Type:     datatype.Bigint,
		}
		columnMetadataList = append([]*message.ColumnMetadata{&metadata}, columnMetadataList...)
	}
	return columnMetadataList
}
