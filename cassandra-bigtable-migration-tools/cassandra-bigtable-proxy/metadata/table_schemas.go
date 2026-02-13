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

package metadata

import (
	"fmt"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/utilities"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/google/uuid"
	"golang.org/x/exp/maps"
	"slices"
	"sort"
	"strings"
	"sync"
)

// SchemaMetadata contains the schema information for all tables, across
// all Bigtable instances, managed by this proxy.
type SchemaMetadata struct {
	mu                 sync.RWMutex
	tables             map[types.Keyspace]map[types.TableName]*TableSchema
	SystemColumnFamily types.ColumnFamily
	events             *utilities.EventPublisher[MetadataEvent]
}

type MetadataEventType string

const (
	MetadataRemovedEventType = MetadataEventType("removed")
	MetadataAddedEventType   = MetadataEventType("added")
	MetadataChangedEventType = MetadataEventType("changed")
)

type MetadataEvent struct {
	EventType MetadataEventType
	Keyspace  types.Keyspace
	Table     types.TableName
}

func NewMetadataEvent(eventType MetadataEventType, keyspace types.Keyspace, table types.TableName) MetadataEvent {
	return MetadataEvent{
		EventType: eventType,
		Keyspace:  keyspace,
		Table:     table,
	}
}

func (c *SchemaMetadata) Subscribe(s utilities.Subscriber[MetadataEvent]) {
	c.events.Register(s)
}

// NewSchemaMetadata is a constructor for SchemaMetadata. Please use this instead of direct initialization.
func NewSchemaMetadata(systemColumnFamily types.ColumnFamily, tableConfigs []*TableSchema) *SchemaMetadata {
	tablesMap := GroupTables(tableConfigs)
	return &SchemaMetadata{
		SystemColumnFamily: systemColumnFamily,
		tables:             tablesMap,
		events:             utilities.NewPublisher[MetadataEvent](),
	}
}

func (c *SchemaMetadata) Keyspaces() []types.Keyspace {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var results []types.Keyspace
	for keyspace := range c.tables {
		results = append(results, keyspace)
	}
	return results
}

func (c *SchemaMetadata) Tables() []*TableSchema {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var tables []*TableSchema
	for _, keyspace := range c.tables {
		for _, t := range keyspace {
			tables = append(tables, t)
		}
	}
	return tables
}

// CalculateSchemaVersion creates a deterministic UUID based on all keyspace and table schema information to be used
// for Cassandra schema_version. Schema Version must be consistent across all Proxy nodes. Most Cassandra clients care
// about all nodes achieving the same schema version.
func (c *SchemaMetadata) CalculateSchemaVersion() (primitive.UUID, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var builder strings.Builder

	sortedKeyspaces := make([]string, 0, len(c.tables))
	for k := range c.tables {
		if k.IsSystemKeyspace() {
			// we only care about user entities
			continue
		}
		sortedKeyspaces = append(sortedKeyspaces, string(k))
	}
	sort.Strings(sortedKeyspaces)

	for _, k := range sortedKeyspaces {
		keyspace := types.Keyspace(k)
		tables, ok := c.tables[keyspace]
		if !ok {
			return primitive.UUID{}, fmt.Errorf("failed to next schema version")
		}
		builder.WriteString(fmt.Sprintf("KS:%s|", k))

		sortedTableNames := make([]string, 0, len(tables))
		for _, t := range tables {
			sortedTableNames = append(sortedTableNames, string(t.Name))
		}
		sort.Strings(sortedTableNames)

		for _, t := range sortedTableNames {
			table, err := c.GetTableSchema(keyspace, types.TableName(t))
			if err != nil {
				return primitive.UUID{}, err
			}
			builder.WriteString(fmt.Sprintf("TBL:%s(", table.Name))

			sortedColumnNames := make([]string, 0, len(table.Columns))
			for name := range table.Columns {
				sortedColumnNames = append(sortedColumnNames, string(name))
			}
			sort.Strings(sortedColumnNames)

			for i, c := range sortedColumnNames {
				col := table.Columns[types.ColumnName(c)]
				builder.WriteString(fmt.Sprintf("%s:%s", col.Name, col.CQLType.String()))
				if i < len(sortedColumnNames)-1 {
					builder.WriteString(",")
				}
			}
			builder.WriteString(")")
		}
	}

	// Generate a Version 3 UUID (MD5-based)
	// We use a fixed Namespace UUID (like Cassandra)
	namespace, err := uuid.Parse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
	if err != nil {
		return primitive.UUID{}, err
	}
	return primitive.UUID(uuid.NewMD5(namespace, []byte(builder.String()))), nil
}

func (c *SchemaMetadata) ValidateKeyspace(keyspace types.Keyspace) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.tables[keyspace]
	if !ok {
		return fmt.Errorf("keyspace '%s' does not exist", keyspace)
	}
	return nil
}

func (c *SchemaMetadata) GetKeyspace(keyspace types.Keyspace) ([]*TableSchema, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	tables, ok := c.tables[keyspace]
	if !ok {
		return nil, fmt.Errorf("keyspace '%s' does not exist", keyspace)
	}
	var results []*TableSchema = nil
	for _, table := range tables {
		results = append(results, table)
	}
	return results, nil
}

func (c *SchemaMetadata) AddTables(tables []*TableSchema) {
	grouped := GroupTables(tables)
	for keyspace, keyspaceTables := range grouped {
		c.SyncKeyspace(keyspace, maps.Values(keyspaceTables))
	}
}

func (c *SchemaMetadata) SyncKeyspace(keyspace types.Keyspace, updates []*TableSchema) {
	c.mu.Lock()

	// ensure keyspace exists
	if _, exists := c.tables[keyspace]; !exists {
		c.tables[keyspace] = make(map[types.TableName]*TableSchema)
	}

	var events []MetadataEvent
	for _, table := range updates {
		existingTable, tableAlreadyExisted := c.tables[table.Keyspace][table.Name]
		// skip table if no changed detected
		if tableAlreadyExisted && existingTable.SameSchema(table) {
			continue
		}
		c.tables[table.Keyspace][table.Name] = table
		if tableAlreadyExisted {
			events = append(events, NewMetadataEvent(MetadataChangedEventType, table.Keyspace, table.Name))
		} else {
			events = append(events, NewMetadataEvent(MetadataAddedEventType, table.Keyspace, table.Name))
		}
	}

	// remove dropped tables
	keyspaceTables := c.tables[keyspace]
	for _, table := range keyspaceTables {
		var found *TableSchema
		for _, t := range updates {
			if t.SameTable(table) {
				found = t
				break
			}
		}
		if found == nil {
			delete(c.tables[keyspace], table.Name)
			events = append(events, NewMetadataEvent(MetadataRemovedEventType, table.Keyspace, table.Name))
		}
	}

	// unlock before sending events so down stream subscribers can read schemas
	c.mu.Unlock()
	for _, e := range events {
		c.events.SendEvent(e)
	}
}

// GetTableSchema finds the primary key columns of a specified table in a given keyspace.
//
// This method looks up the cached primary key metadata and returns the relevant columns.
//
// Parameters:
//   - keyspace: The name of the keyspace where the table resides.
//   - tableName: The name of the table for which primary key metadata is requested.
//
// Returns:
//   - []types.Column: A slice of types.Column structs representing the primary keys of the table.
//   - error: Returns an error if the primary key metadata is not found.
func (c *SchemaMetadata) GetTableSchema(k types.Keyspace, t types.TableName) (*TableSchema, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	keyspace, ok := c.tables[k]
	if !ok {
		return nil, fmt.Errorf("keyspace '%s' does not exist", k)
	}
	tableConfig, ok := keyspace[t]
	if !ok {
		return nil, fmt.Errorf("table '%s' does not exist", t)
	}
	return tableConfig, nil
}

func (c *SchemaMetadata) CountTables() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var result = 0
	for _, keyspaceTables := range c.tables {
		result += len(keyspaceTables)
	}
	return result
}

// ListKeyspaces returns a sorted list of all keyspace names in the schema mapping.
func (c *SchemaMetadata) ListKeyspaces() []types.Keyspace {
	c.mu.RLock()
	defer c.mu.RUnlock()
	keyspaces := make([]types.Keyspace, 0, len(c.tables))
	for ks := range c.tables {
		keyspaces = append(keyspaces, ks)
	}
	slices.SortFunc(keyspaces, func(a, b types.Keyspace) int {
		return strings.Compare(string(a), string(b))
	})
	return keyspaces
}

func GroupTables(tables []*TableSchema) map[types.Keyspace]map[types.TableName]*TableSchema {
	tablesMap := make(map[types.Keyspace]map[types.TableName]*TableSchema)
	for _, tableConfig := range tables {
		if _, exists := tablesMap[tableConfig.Keyspace]; !exists {
			tablesMap[tableConfig.Keyspace] = make(map[types.TableName]*TableSchema)
		}
		tablesMap[tableConfig.Keyspace][tableConfig.Name] = tableConfig
	}
	return tablesMap
}
