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

package delete_translator

import (
	"fmt"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/translators/common"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
)

func (t *DeleteTranslator) Translate(query *types.RawQuery, sessionKeyspace types.Keyspace) (types.IPreparedQuery, error) {
	deleteObj, err := query.Parser().Delete_()
	if err != nil {
		return nil, err
	}

	keyspaceName, tableName, err := common.ParseTableSpec(deleteObj.FromSpec().TableSpec(), sessionKeyspace)
	if err != nil {
		return nil, err
	}

	table, err := t.schemaMappingConfig.GetTableSchema(keyspaceName, tableName)
	if err != nil {
		return nil, err
	}

	var ifExist = deleteObj.IfExist() != nil

	params := types.NewQueryParameterBuilder()

	selectedColumns, err := parseDeleteColumns(deleteObj.DeleteColumnList(), table)
	if err != nil {
		return nil, err
	}

	conditions, err := common.ParseWhereClause(deleteObj.WhereSpec(), table, params)
	if err != nil {
		return nil, err
	}

	if deleteObj.UsingTimestampSpec() != nil {
		return nil, fmt.Errorf("delete USING TIMESTAMP not supported yet")
	}

	rowKeys, err := common.ConvertStrictConditionsToRowKeyValues(table, conditions)
	if err != nil {
		return nil, err
	}

	builtParams, err := params.Build()
	if err != nil {
		return nil, err
	}

	st := types.NewPreparedDeleteQuery(keyspaceName, tableName, ifExist, query.RawCql(), rowKeys, builtParams, selectedColumns)

	return st, nil
}

func (t *DeleteTranslator) Bind(st types.IPreparedQuery, values *types.QueryParameterValues, pv primitive.ProtocolVersion) (types.IExecutableQuery, error) {
	dst, ok := st.(*types.PreparedDeleteQuery)
	if !ok {
		return nil, fmt.Errorf("cannot bind to %T", st)
	}
	tableConfig, err := t.schemaMappingConfig.GetTableSchema(dst.Keyspace(), dst.Table())
	if err != nil {
		return nil, err
	}

	rowKey, err := common.BindRowKey(tableConfig, dst.RowKey, values)
	if err != nil {
		return nil, fmt.Errorf("key encoding failed: %w", err)
	}

	cols, err := common.BindSelectColumns(tableConfig, dst.SelectedColumns)
	if err != nil {
		return nil, err
	}
	return types.NewBoundDeleteQuery(dst.Keyspace(), dst.Table(), dst.CqlQuery(), rowKey, dst.IfExists, cols), nil
}
