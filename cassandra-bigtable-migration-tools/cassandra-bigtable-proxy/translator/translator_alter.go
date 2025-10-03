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

	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	cql "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/third_party/cqlparser"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/utilities"
	"github.com/antlr4-go/antlr/v4"
)

func (t *Translator) TranslateAlterTableToBigtable(query, sessionKeyspace string) (*AlterTableStatementMap, error) {
	lexer := cql.NewCqlLexer(antlr.NewInputStream(query))
	stream := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)
	p := cql.NewCqlParser(stream)

	alterTable := p.AlterTable()

	if alterTable == nil || alterTable.Table() == nil {
		return nil, errors.New("error while parsing alter statement")
	}

	var tableName, keyspaceName string

	if alterTable != nil && alterTable.Table() != nil && alterTable.Table().GetText() != "" {
		tableName = alterTable.Table().GetText()
		if !validTableName.MatchString(tableName) {
			return nil, errors.New("invalid table name parsed from query")
		}
	} else {
		return nil, errors.New("invalid input paramaters found for table")
	}

	if tableName == t.SchemaMappingConfig.SchemaMappingTableName {
		return nil, fmt.Errorf("cannot alter schema mapping table with configured name '%s'", tableName)
	}

	if alterTable != nil && alterTable.Keyspace() != nil && alterTable.Keyspace().GetText() != "" {
		keyspaceName = alterTable.Keyspace().GetText()
	} else if sessionKeyspace != "" {
		keyspaceName = sessionKeyspace
	} else {
		return nil, errors.New("missing keyspace. keyspace is required")
	}

	tableConfig, err := t.SchemaMappingConfig.GetTableConfig(keyspaceName, tableName)
	if err != nil {
		return nil, err
	}

	if alterTable.AlterTableOperation().AlterTableAlterColumnTypes() != nil {
		return nil, errors.New("alter column type operations are not supported")
	}

	if alterTable.AlterTableOperation().AlterTableWith() != nil {
		return nil, errors.New("table property operations are not supported")
	}

	var dropColumns []string
	if alterTable.AlterTableOperation().AlterTableDropColumns() != nil {
		for _, dropColumn := range alterTable.AlterTableOperation().AlterTableDropColumns().AlterTableDropColumnList().AllColumn() {
			dropColumns = append(dropColumns, dropColumn.GetText())
		}
	}
	var addColumns []types.CreateColumn
	if alterTable.AlterTableOperation().AlterTableAdd() != nil {
		for i, addColumn := range alterTable.AlterTableOperation().AlterTableAdd().AlterTableColumnDefinition().AllColumn() {
			dt, err := utilities.ParseCqlType(alterTable.AlterTableOperation().AlterTableAdd().AlterTableColumnDefinition().DataType(i))
			if err != nil {
				return nil, err
			}

			addColumns = append(addColumns, types.CreateColumn{
				TypeInfo: dt,
				Name:     addColumn.GetText(),
				Index:    int32(i),
			})
		}
	}

	if alterTable.AlterTableOperation().AlterTableRename() != nil && len(alterTable.AlterTableOperation().AlterTableRename().AllColumn()) != 0 {
		return nil, errors.New("rename operation in alter table command not supported")
	}

	for _, dropColumn := range dropColumns {
		column, err := tableConfig.GetColumn(dropColumn)
		if err != nil {
			return nil, fmt.Errorf("cannot drop column: %w", err)
		}
		if column.IsPrimaryKey {
			return nil, fmt.Errorf("cannot drop primary key column: '%s'", column.Name)
		}
	}
	for _, addColumn := range addColumns {
		if !utilities.IsSupportedColumnType(addColumn.TypeInfo) {
			return nil, fmt.Errorf("column type '%s' is not supported", addColumn.TypeInfo.String())
		}
		if utilities.IsReservedCqlKeyword(addColumn.Name) {
			return nil, fmt.Errorf("cannot alter a table with reserved keyword as column name: '%s'", addColumn.Name)
		}
		if tableConfig.HasColumn(addColumn.Name) {
			return nil, fmt.Errorf("column '%s' already exists in table", addColumn.Name)
		}
	}

	var stmt = AlterTableStatementMap{
		Table:       tableName,
		Keyspace:    keyspaceName,
		QueryType:   "alter",
		DropColumns: dropColumns,
		AddColumns:  addColumns,
	}

	return &stmt, nil
}
