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
	"slices"
	"strings"

	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"golang.org/x/exp/maps"
)

// TableSchema contains all schema information about a single table
type TableSchema struct {
	Keyspace           types.Keyspace
	Name               types.TableName
	Columns            map[types.ColumnName]*types.Column
	PrimaryKeys        []*types.Column
	SystemColumnFamily types.ColumnFamily
	// this is for backwards compatability. We had to launch with only big endian encoding, but we prefer ordered code encoding (added in a later version) because it allows negatives.
	IntRowKeyEncoding types.IntRowKeyEncodingType
}

// NewTableConfig is a constructor for TableSchema. Please use this instead of direct initialization.
func NewTableConfig(
	keyspace types.Keyspace, table types.TableName,
	systemColumnFamily types.ColumnFamily,
	intRowKeyEncoding types.IntRowKeyEncodingType,
	columns []*types.Column,
) *TableSchema {
	columnMap := make(map[types.ColumnName]*types.Column)
	var pks []*types.Column = nil
	for i, column := range columns {

		var cf types.ColumnFamily
		if column.IsPrimaryKey {
			// primary keys are stored in the row key, so they have no column family
			cf = ""
		} else if column.CQLType.IsCollection() || column.CQLType.Code() == types.COUNTER {
			// collections and counters are stored at the column family level, so their name is their column family
			cf = types.ColumnFamily(column.Name)
		} else {
			// all scalars that aren't primary keys are stored in the configured system column family
			cf = systemColumnFamily
		}
		column.ColumnFamily = cf
		column.Metadata = message.ColumnMetadata{
			Keyspace: string(keyspace),
			Table:    string(table),
			Name:     string(column.Name),
			Index:    int32(i),
			Type:     column.CQLType.DataType(),
		}
		columnMap[column.Name] = column
		if column.KeyType == types.KeyTypePartition || column.KeyType == types.KeyTypeClustering {
			// this field is redundant - make sure it's in sync with other fields
			column.IsPrimaryKey = true
			pks = append(pks, column)
		}
	}
	sortPrimaryKeys(pks)

	return &TableSchema{
		Keyspace:           keyspace,
		Name:               table,
		Columns:            columnMap,
		PrimaryKeys:        pks,
		SystemColumnFamily: systemColumnFamily,
		IntRowKeyEncoding:  intRowKeyEncoding,
	}
}

func (t *TableSchema) SameTable(other *TableSchema) bool {
	if other == nil {
		return false
	}
	return t.Keyspace == other.Keyspace && t.Name == other.Name
}

func (t *TableSchema) SameSchema(other *TableSchema) bool {
	if other == nil {
		return false
	}

	if len(t.PrimaryKeys) != len(other.PrimaryKeys) || len(t.Columns) != len(other.Columns) {
		return false
	}

	for i, key := range t.PrimaryKeys {
		otherKey := other.PrimaryKeys[i]
		if key.Name != otherKey.Name || key.CQLType != otherKey.CQLType {
			return false
		}
	}
	for _, col := range t.Columns {
		otherCol, ok := other.Columns[col.Name]
		if !ok {
			return false
		}
		if col.Name != otherCol.Name || col.CQLType != otherCol.CQLType {
			return false
		}
	}
	return true
}

func (t *TableSchema) AllColumns() []*types.Column {
	return maps.Values(t.Columns)
}

func (t *TableSchema) GetPkByTableNameWithFilter(filterPrimaryKeys []types.ColumnName) []*types.Column {
	var result []*types.Column
	for _, pmk := range t.PrimaryKeys {
		if slices.Contains(filterPrimaryKeys, pmk.Name) {
			result = append(result, pmk)
		}
	}
	return result
}

func (t *TableSchema) GetCassandraPositionForColumn(column types.ColumnName) int {
	col, err := t.GetColumn(column)
	if err != nil {
		return -1
	}
	// regular columns all have a position of -1 in cassandra - position is only used to track primary key position/index.
	if !col.IsPrimaryKey {
		return -1
	}
	// we need to return the position/index of the column _within_ it's key type.
	// Example: given PRIMARY KEY((org, user), email, name) org=0, user=1, email=0 and name=1
	result := 0
	for _, key := range t.PrimaryKeys {
		if key.KeyType != col.KeyType {
			continue
		}
		if key.Name == col.Name {
			return result
		}
		result++
	}
	// this shouldn't happen
	return 0
}

// GetColumnFamily retrieves the column family for a given column.
// Returns the column family name from the schema mapping with validation.
// Returns default column family for primitive types and column name for collections.
func (t *TableSchema) GetColumnFamily(columnName types.ColumnName) types.ColumnFamily {
	colType, err := t.GetColumnType(columnName)
	if err == nil && colType.IsCollection() {
		return types.ColumnFamily(columnName)
	}
	return t.SystemColumnFamily
}

func (t *TableSchema) HasColumn(columnName types.ColumnName) bool {
	_, ok := t.Columns[columnName]
	return ok
}

func (t *TableSchema) Describe() string {
	cols := maps.Values(t.Columns)
	// sort by metadata index for consistent output
	slices.SortFunc(cols, func(a, b *types.Column) int {
		if a.IsPrimaryKey && b.IsPrimaryKey {
			return a.PkPrecedence - b.PkPrecedence
		} else if a.IsPrimaryKey {
			return -1
		} else if b.IsPrimaryKey {
			return 1
		} else {
			return int(a.Metadata.Index - b.Metadata.Index)
		}
	})
	var colNames []types.ColumnName = nil
	for _, col := range cols {
		colNames = append(colNames, col.Name)
	}

	// First collect all column definitions with their data types
	var colDefs []string
	for _, colName := range colNames {
		col := t.Columns[colName]
		colDefs = append(colDefs, fmt.Sprintf("%s %s", colName, strings.ToUpper(col.CQLType.String())))
	}

	var pkCols []string = nil
	var clusteringCols []string = nil
	for _, key := range t.PrimaryKeys {
		if key.KeyType == types.KeyTypePartition {
			pkCols = append(pkCols, string(key.Name))
		} else {
			clusteringCols = append(clusteringCols, string(key.Name))
		}
	}

	// Build primary key clause
	pkClause := ""
	if len(pkCols) == 1 && len(clusteringCols) == 0 {
		pkClause = fmt.Sprintf("PRIMARY KEY (%s)", pkCols[0])
	} else if len(pkCols) == 1 && len(clusteringCols) > 0 {
		pkClause = fmt.Sprintf("PRIMARY KEY (%s, %s)", pkCols[0], strings.Join(clusteringCols, ", "))
	} else {
		pkClause = fmt.Sprintf("PRIMARY KEY ((%s), %s)",
			strings.Join(pkCols, ", "),
			strings.Join(clusteringCols, ", "))
	}

	createTableStmt := fmt.Sprintf("CREATE TABLE %s.%s (\n    %s,\n    %s\n);",
		t.Keyspace, t.Name,
		strings.Join(colDefs, ",\n    "),
		pkClause)
	return createTableStmt
}

func (t *TableSchema) GetColumnWithFallbacks(columns ...string) (*types.Column, bool) {
	for _, c := range columns {
		col, ok := t.Columns[types.ColumnName(c)]
		if ok {
			return col, true
		}
	}
	return nil, false
}
func (t *TableSchema) GetColumn(columnName types.ColumnName) (*types.Column, error) {
	col, ok := t.Columns[columnName]
	if !ok {
		return nil, fmt.Errorf("unknown column '%s' in table %s.%s", columnName, t.Keyspace, t.Name)
	}
	return col, nil
}

func (t *TableSchema) GetPrimaryKeys() []types.ColumnName {
	var primaryKeys []types.ColumnName
	for _, pk := range t.PrimaryKeys {
		primaryKeys = append(primaryKeys, pk.Name)
	}
	return primaryKeys
}

func (t *TableSchema) GetColumnDataType(columnName types.ColumnName) (datatype.DataType, error) {
	col, err := t.GetColumnType(columnName)
	if err != nil {
		return nil, err
	}
	return col.DataType(), nil
}
func (t *TableSchema) GetColumnType(columnName types.ColumnName) (types.CqlDataType, error) {
	col, ok := t.Columns[columnName]
	if !ok {
		return nil, fmt.Errorf("undefined column name %s in table %s.%s", columnName, t.Keyspace, t.Name)
	}

	if col.CQLType == nil {
		return nil, fmt.Errorf("undefined column name %s in table %s.%s", columnName, t.Keyspace, t.Name)
	}
	return col.CQLType, nil
}

func (t *TableSchema) GetMetadata() []*message.ColumnMetadata {
	var results []*message.ColumnMetadata
	for _, c := range t.Columns {
		results = append(results, &c.Metadata)
	}
	return results
}

func (t *TableSchema) CreateTableMetadata() TableMetaData {
	return TableMetaData{
		KeyspaceName:          string(t.Keyspace),
		TableName:             string(t.Name),
		AdditionalWritePolicy: "99p",
		BloomFilterFpChance:   0.01,
		Caching: map[string]string{
			"keys":               "ALL",
			"rows_per_partition": "NONE",
		},
		Flags: []string{"compound"},
	}
}

type TableMetaData struct {
	KeyspaceName          string
	TableName             string
	AdditionalWritePolicy string
	BloomFilterFpChance   float64
	Caching               map[string]string
	Flags                 []string
}
