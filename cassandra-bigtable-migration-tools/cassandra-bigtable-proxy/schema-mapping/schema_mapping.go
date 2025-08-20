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

type SchemaMappingConfig struct {
	Logger              *zap.Logger
	mu                  sync.RWMutex
	tables              map[string]map[string]*TableConfig
	SystemColumnFamily  string
	CounterColumnFamily string
}

func NewSchemaMappingConfig(systemColumnFamily string, counterColumnFamily string, logger *zap.Logger, tableConfigs []*TableConfig) *SchemaMappingConfig {
	tablesMap := make(map[string]map[string]*TableConfig)
	for _, tableConfig := range tableConfigs {
		if keyspace, exists := tablesMap[tableConfig.Keyspace]; !exists {
			keyspace = make(map[string]*TableConfig)
			tablesMap[tableConfig.Keyspace] = keyspace
		}
		tablesMap[tableConfig.Keyspace][tableConfig.Name] = tableConfig
	}
	return &SchemaMappingConfig{
		Logger:              logger,
		SystemColumnFamily:  systemColumnFamily,
		CounterColumnFamily: counterColumnFamily,
		tables:              tablesMap,
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

func (c *SchemaMappingConfig) UpdateTables(tableConfigs []*TableConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.tables == nil {
		c.tables = make(map[string]map[string]*TableConfig)
	}
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
		return nil, fmt.Errorf("keyspace %s does not exist", keySpace)
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
