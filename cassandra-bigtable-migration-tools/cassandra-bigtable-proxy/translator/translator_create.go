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

package translator

import (
	"errors"
	"fmt"
	"slices"

	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	cql "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/third_party/cqlparser"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/utilities"
	"github.com/antlr4-go/antlr/v4"
)

const (
	intRowKeyEncodingOptionName             = "int_row_key_encoding"
	intRowKeyEncodingOptionValueBigEndian   = "big_endian"
	intRowKeyEncodingOptionValueOrderedCode = "ordered_code"
)

func (t *Translator) TranslateCreateTableToBigtable(query, sessionKeyspace string) (*CreateTableStatementMap, error) {
	lexer := cql.NewCqlLexer(antlr.NewInputStream(query))
	stream := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)
	p := cql.NewCqlParser(stream)

	createTableObj := p.CreateTable()

	if createTableObj == nil || createTableObj.Table() == nil {
		return nil, errors.New("error while parsing create object")
	}

	var tableName, keyspaceName string

	if createTableObj != nil && createTableObj.Table() != nil && createTableObj.Table().GetText() != "" {
		tableName = createTableObj.Table().GetText()
		if !validTableName.MatchString(tableName) {
			return nil, errors.New("invalid table name parsed from query")
		}
	} else {
		return nil, errors.New("invalid input parameters found for table")
	}

	if tableName == t.SchemaMappingConfig.SchemaMappingTableName {
		return nil, fmt.Errorf("cannot create a table with the configured schema mapping table name '%s'", tableName)
	}

	if createTableObj != nil && createTableObj.Keyspace() != nil && createTableObj.Keyspace().GetText() != "" {
		keyspaceName = createTableObj.Keyspace().GetText()
	} else if sessionKeyspace != "" {
		keyspaceName = sessionKeyspace
	} else {
		return nil, errors.New("missing keyspace. keyspace is required")
	}

	var pmks []CreateTablePrimaryKeyConfig
	var columns []types.CreateColumn

	if createTableObj.ColumnDefinitionList().GetText() == "" {
		return nil, errors.New("malformed create table statement")
	}

	for i, col := range createTableObj.ColumnDefinitionList().AllColumnDefinition() {
		dt, isFrozen, err := utilities.GetCassandraColumnType(col.DataType().GetText())

		if err != nil {
			return nil, err
		}

		if !utilities.IsSupportedColumnType(dt) {
			return nil, fmt.Errorf("column type '%s' is not supported", dt.String())
		}

		columns = append(columns, types.CreateColumn{
			Type:     dt,
			IsFrozen: isFrozen,
			Name:     col.Column().GetText(),
			Index:    int32(i),
		})

		if col.PrimaryKeyColumn() != nil {
			pmks = append(pmks, CreateTablePrimaryKeyConfig{
				Name:    col.Column().GetText(),
				KeyType: utilities.KEY_TYPE_PARTITION,
			})
		}
	}

	if len(pmks) > 1 {
		return nil, errors.New("multiple inline primary key columns not allowed")
	}

	if len(columns) == 0 {
		return nil, errors.New("no columns found in create table statement")
	}

	rowKeyEncoding := t.DefaultIntRowKeyEncoding
	for optionName, optionValue := range createOptionsMap(createTableObj.WithElement()) {
		switch optionName {
		case intRowKeyEncodingOptionName:
			optionValue = trimQuotes(optionValue)
			switch optionValue {
			case intRowKeyEncodingOptionValueBigEndian:
				rowKeyEncoding = types.BigEndianEncoding
			case intRowKeyEncodingOptionValueOrderedCode:
				rowKeyEncoding = types.OrderedCodeEncoding
			default:
				return nil, fmt.Errorf("unsupported encoding '%s' for option '%s'", optionValue, optionName)
			}
		default:
			// fail fast, so the user know we don't support the option rather than silently ignoring it.
			return nil, fmt.Errorf("unsupported table option: '%s'", optionName)
		}
	}

	// nil if inline primary key definition used
	if createTableObj.ColumnDefinitionList().PrimaryKeyElement() != nil {
		if len(pmks) > 0 {
			return nil, errors.New("cannot specify both primary key clause and inline primary key")
		}
		singleKey := createTableObj.ColumnDefinitionList().PrimaryKeyElement().PrimaryKeyDefinition().SinglePrimaryKey()
		compoundKey := createTableObj.ColumnDefinitionList().PrimaryKeyElement().PrimaryKeyDefinition().CompoundKey()
		compositeKey := createTableObj.ColumnDefinitionList().PrimaryKeyElement().PrimaryKeyDefinition().CompositeKey()
		if singleKey != nil {
			pmks = []CreateTablePrimaryKeyConfig{
				{
					Name:    singleKey.GetText(),
					KeyType: utilities.KEY_TYPE_PARTITION,
				},
			}
		} else if compoundKey != nil {
			pmks = append(pmks, CreateTablePrimaryKeyConfig{
				Name:    compoundKey.PartitionKey().GetText(),
				KeyType: utilities.KEY_TYPE_PARTITION,
			})
			for _, clusterKey := range compoundKey.ClusteringKeyList().AllClusteringKey() {
				pmks = append(pmks, CreateTablePrimaryKeyConfig{
					Name:    clusterKey.Column().GetText(),
					KeyType: utilities.KEY_TYPE_CLUSTERING,
				})
			}
		} else if compositeKey != nil {
			for _, partitionKey := range compositeKey.PartitionKeyList().AllPartitionKey() {
				pmks = append(pmks, CreateTablePrimaryKeyConfig{
					Name:    partitionKey.Column().GetText(),
					KeyType: utilities.KEY_TYPE_PARTITION,
				})
			}
			for _, clusterKey := range compositeKey.ClusteringKeyList().AllClusteringKey() {
				pmks = append(pmks, CreateTablePrimaryKeyConfig{
					Name:    clusterKey.Column().GetText(),
					KeyType: utilities.KEY_TYPE_CLUSTERING,
				})
			}
		}
	}

	if utilities.IsReservedCqlKeyword(tableName) {
		return nil, fmt.Errorf("cannot create a table with reserved keyword as name: '%s'", tableName)
	}

	for _, column := range columns {
		if utilities.IsReservedCqlKeyword(column.Name) {
			return nil, fmt.Errorf("cannot create a table with reserved keyword as column name: '%s'", column.Name)
		}
	}

	if len(pmks) == 0 {
		return nil, errors.New("no primary key found in create table statement")
	}

	for _, pmk := range pmks {
		colIndex := slices.IndexFunc(columns, func(col types.CreateColumn) bool {
			return col.Name == pmk.Name
		})
		if colIndex == -1 {
			return nil, fmt.Errorf("primary key '%s' has no column definition in create table statement", pmk.Name)
		}
		col := columns[colIndex]
		if !utilities.IsSupportedPrimaryKeyType(col.Type) {
			return nil, fmt.Errorf("primary key cannot be of type %s", col.Type.String())
		}
	}

	var stmt = CreateTableStatementMap{
		Table:             tableName,
		IfNotExists:       createTableObj.IfNotExist() != nil,
		Keyspace:          keyspaceName,
		QueryType:         "create",
		Columns:           columns,
		PrimaryKeys:       pmks,
		IntRowKeyEncoding: rowKeyEncoding,
	}

	return &stmt, nil
}

func createOptionsMap(withElement cql.IWithElementContext) map[string]string {
	results := make(map[string]string)
	// it's ok if the input is null since options are optional
	if withElement == nil || withElement.TableOptions() == nil {
		return results
	}

	for _, option := range withElement.TableOptions().AllTableOptionItem() {
		optionName := option.TableOptionName().GetText()
		optionValue := option.TableOptionValue().GetText()
		// note: this technically allows options to overwrite each other but that's probably ok
		results[optionName] = optionValue
	}
	return results
}
