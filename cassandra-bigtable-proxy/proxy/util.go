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

	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/ollionorg/cassandra-to-bigtable-proxy/responsehandler"
	schemaMapping "github.com/ollionorg/cassandra-to-bigtable-proxy/schema-mapping"
	"github.com/ollionorg/cassandra-to-bigtable-proxy/translator"
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

const PARTITION_KEY = "partition_key"
const CLUSTERING = "clustering"
const REGULAR = "regular"

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

// ConstructSystemMetadataRows constructs system metadata rows for keyspaces, tables, and columns.
// It iterates through the provided table metadata and formats the data into a Cassandra-compatible structure.
// The resulting metadata is used for system queries in the Bigtable proxy.
//
// Parameters:
//   - tableMetaData: A nested map where the first level represents keyspaces, the second level represents tables,
//     and the third level represents columns within each table.
//
// Returns:
// - A pointer to a SystemQueryMetadataCache, which contains structured metadata for keyspaces, tables, and columns.
// - An error if any issue occurs while building the metadata cache.
func ConstructSystemMetadataRows(tableMetaData map[string]map[string]map[string]*schemaMapping.Column) (*SystemQueryMetadataCache, error) {

	keyspaceMetadataRows := [][]interface{}{}
	tableMetadataRows := [][]interface{}{}
	columnsMetadataRows := [][]interface{}{}

	// Iterate through keyspaces (instances)
	for keyspace, tables := range tableMetaData {

		// Add keyspace metadata
		keyspaceMetadataRows = append(keyspaceMetadataRows, []interface{}{
			keyspace, true, map[string]string{
				"class":              "org.apache.cassandra.locator.SimpleStrategy",
				"replication_factor": "1",
			},
		})

		// Iterate through tables
		for tableName, columns := range tables {

			// Add table metadata
			tableMetadataRows = append(tableMetadataRows, []interface{}{
				keyspace, tableName, "99p", 0.01, map[string]string{
					"keys":               "ALL",
					"rows_per_partition": "NONE",
				},
				[]string{"compound"},
			})

			// Iterate through columns
			for columnName, column := range columns {
				kind := REGULAR
				if column.IsPrimaryKey {
					kind = PARTITION_KEY
					if column.KeyType == CLUSTERING {
						kind = CLUSTERING
					}
				}

				// Add column metadata
				columnsMetadataRows = append(columnsMetadataRows, []interface{}{
					keyspace, tableName, columnName, "none", kind, 0, column.ColumnType,
				})
			}
		}
	}

	columnsMetadataRows = append(columnsMetadataRows, []interface{}{"system_schema", "columns", "kind", "none", PARTITION_KEY, 0, "text"})
	return getSystemQueryMetadataCache(keyspaceMetadataRows, tableMetadataRows, columnsMetadataRows)
}

// getTimestampMetadata appends a metadata entry for a timestamp column to a list of column metadata
// if a timestamp is used in the insert query.
//
// Parameters:
//   - insertQueryMetadata: An InsertQueryMap containing information about the insert query, including
//     any timestamp information.
//   - columnMetadataList: A slice of pointers to ColumnMetadata representing the current list of column metadata.
//
// Returns: An updated slice of pointers to ColumnMetadata, including an entry for the timestamp column
//
//	if the query uses a timestamp.
func getTimestampMetadata(insertQueryMetadata translator.InsertQueryMap, columnMetadataList []*message.ColumnMetadata) []*message.ColumnMetadata {
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
//   - updateQueryMetadata: An UpdateQueryMap containing information about the update query, including
//     any timestamp information.
//   - columnMetadataList: A slice of pointers to ColumnMetadata representing the current list of column metadata.
//
// Returns: An updated slice of pointers to ColumnMetadata, with an entry for the timestamp column prepended
//
//	if the query uses a timestamp.
func getTimestampMetadataForUpdate(updateQueryMetadata translator.UpdateQueryMap, columnMetadataList []*message.ColumnMetadata) []*message.ColumnMetadata {
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
