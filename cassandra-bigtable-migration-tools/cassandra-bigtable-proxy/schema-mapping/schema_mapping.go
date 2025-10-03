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

package schemaMapping

import (
	"fmt"
	"sort"
	"sync"

	"go.uber.org/zap"
)

const (
	LimitValue = "limitValue"
)

// SchemaMappingConfig contains the schema information for all tables, across
// all Bigtable instances, managed by this proxy.
type SchemaMappingConfig struct {
	Logger                 *zap.Logger
	mu                     sync.RWMutex
	tables                 map[string]map[string]*TableConfig
	SystemColumnFamily     string
	SchemaMappingTableName string
}

// NewSchemaMappingConfig is a constructor for SchemaMappingConfig. Please use this instead of direct initialization.
func NewSchemaMappingConfig(schemaMappingTableName, systemColumnFamily string, logger *zap.Logger, tableConfigs []*TableConfig) *SchemaMappingConfig {
	tablesMap := make(map[string]map[string]*TableConfig)
	for _, tableConfig := range tableConfigs {
		if keyspace, exists := tablesMap[tableConfig.Keyspace]; !exists {
			keyspace = make(map[string]*TableConfig)
			tablesMap[tableConfig.Keyspace] = keyspace
		}
		tablesMap[tableConfig.Keyspace][tableConfig.Name] = tableConfig
	}
	return &SchemaMappingConfig{
		Logger:                 logger,
		SchemaMappingTableName: schemaMappingTableName,
		SystemColumnFamily:     systemColumnFamily,
		tables:                 tablesMap,
	}
}

// GetAllTables DEPRECATED - will be removed in the future
func (c *SchemaMappingConfig) GetAllTables() map[string]map[string]*TableConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// make a new shallow copy to ensure that read is thread safe
	tablesCopy := make(map[string]map[string]*TableConfig, len(c.tables))

	for keyspace, tables := range c.tables {
		innerCopy := make(map[string]*TableConfig, len(tables))
		for name, config := range tables {
			innerCopy[name] = config
		}
		tablesCopy[keyspace] = innerCopy
	}

	return tablesCopy
}

func (c *SchemaMappingConfig) GetKeyspace(keyspace string) ([]*TableConfig, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	tables, ok := c.tables[keyspace]
	if !ok {
		return nil, fmt.Errorf("keyspace '%s' does not exist", keyspace)
	}
	var results []*TableConfig = nil
	for _, table := range tables {
		results = append(results, table)
	}
	return results, nil
}

func (c *SchemaMappingConfig) ReplaceTables(keyspace string, tableConfigs []*TableConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// clear the keyspace
	c.tables[keyspace] = make(map[string]*TableConfig)
	for _, tableConfig := range tableConfigs {
		if tableConfig.Keyspace != keyspace {
			return fmt.Errorf("cannot replace table with keyspace '%s' because we're only updating keyspace '%s'", tableConfig.Keyspace, keyspace)
		}
		c.tables[keyspace][tableConfig.Name] = tableConfig
	}
	return nil
}

func (c *SchemaMappingConfig) UpdateTables(tableConfigs []*TableConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, tableConfig := range tableConfigs {
		if _, exists := c.tables[tableConfig.Keyspace]; !exists {
			c.tables[tableConfig.Keyspace] = make(map[string]*TableConfig)
		}
		c.tables[tableConfig.Keyspace][tableConfig.Name] = tableConfig
	}
}

// GetTableConfig finds the primary key columns of a specified table in a given keyspace.
//
// This method looks up the cached primary key metadata and returns the relevant columns.
//
// Parameters:
//   - keySpace: The name of the keyspace where the table resides.
//   - tableName: The name of the table for which primary key metadata is requested.
//
// Returns:
//   - []types.Column: A slice of types.Column structs representing the primary keys of the table.
//   - error: Returns an error if the primary key metadata is not found.
func (c *SchemaMappingConfig) GetTableConfig(keySpace string, tableName string) (*TableConfig, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	keyspace, ok := c.tables[keySpace]
	if !ok {
		return nil, fmt.Errorf("keyspace '%s' does not exist", keySpace)
	}
	tableConfig, ok := keyspace[tableName]
	if !ok {
		return nil, fmt.Errorf("table %s does not exist", tableName)
	}
	return tableConfig, nil
}

func (c *SchemaMappingConfig) CountTables() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var result = 0
	for _, keyspaceTables := range c.tables {
		result += len(keyspaceTables)
	}
	return result
}

// ListKeyspaces returns a sorted list of all keyspace names in the schema mapping.
func (c *SchemaMappingConfig) ListKeyspaces() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	keyspaces := make([]string, 0, len(c.tables))
	for ks := range c.tables {
		keyspaces = append(keyspaces, ks)
	}
	sort.Strings(keyspaces)
	return keyspaces
}
