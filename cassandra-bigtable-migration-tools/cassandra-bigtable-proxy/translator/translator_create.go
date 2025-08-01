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

	methods "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/methods"
	cql "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/third_party/cqlparser"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/utilities"
	"github.com/antlr4-go/antlr/v4"
	"github.com/datastax/go-cassandra-native-protocol/message"
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
		return nil, errors.New("invalid input paramaters found for table")
	}

	if createTableObj != nil && createTableObj.Keyspace() != nil && createTableObj.Keyspace().GetText() != "" {
		keyspaceName = createTableObj.Keyspace().GetText()
	} else if sessionKeyspace != "" {
		keyspaceName = sessionKeyspace
	} else {
		return nil, errors.New("missing keyspace. keyspace is required")
	}

	var pmks []CreateTablePrimaryKeyConfig
	var columns []message.ColumnMetadata
	for i, col := range createTableObj.ColumnDefinitionList().AllColumnDefinition() {
		dt, err := methods.GetCassandraColumnType(col.DataType().GetText())

		if err != nil {
			return nil, err
		}

		columns = append(columns, message.ColumnMetadata{
			Table:    tableName,
			Keyspace: keyspaceName,
			Type:     dt,
			Name:     col.Column().GetText(),
			Index:    int32(i),
		})

		if col.PrimaryKeyColumn() != nil {
			pmks = append(pmks, CreateTablePrimaryKeyConfig{
				Name:    col.Column().GetText(),
				KeyType: utilities.KEY_TYPE_REGULAR,
			})
		}
	}

	if len(columns) == 0 {
		return nil, errors.New("no columns found in create table statement")
	}

	// nil if inline primary key definition used
	if createTableObj.ColumnDefinitionList().PrimaryKeyElement() != nil {
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
		} else {
			// this should never happen
			return nil, errors.New("unknown key type for table")
		}
	}

	if len(pmks) == 0 {
		return nil, errors.New("no primary key found in create table statement")
	}

	var stmt = CreateTableStatementMap{
		Table:       tableName,
		IfNotExists: createTableObj.IfNotExist() != nil,
		Keyspace:    keyspaceName,
		QueryType:   "create",
		Columns:     columns,
		PrimaryKeys: pmks,
	}

	return &stmt, nil
}
