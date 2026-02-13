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

package insert_translator

import (
	"fmt"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/translators/common"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
)

func (t *InsertTranslator) Translate(query *types.RawQuery, sessionKeyspace types.Keyspace) (types.IPreparedQuery, error) {
	insertObj, err := query.Parser().Insert()
	if err != nil {
		return nil, err
	}

	keyspaceName, tableName, err := common.ParseTableSpec(insertObj.TableSpec(), sessionKeyspace)
	if err != nil {
		return nil, err
	}

	table, err := t.schemaMappingConfig.GetTableSchema(keyspaceName, tableName)
	if err != nil {
		return nil, err
	}

	ifNotExists := insertObj.IfNotExist() != nil

	params := types.NewQueryParameterBuilder()

	columns, err := parseInsertColumns(insertObj.InsertColumnSpec(), table, params)
	if err != nil {
		return nil, err
	}

	values, err := parseInsertValues(insertObj.InsertValuesSpec(), columns, params)
	if err != nil {
		return nil, err
	}

	var usingTimestamp types.DynamicValue
	if insertObj.UsingTtlTimestamp() != nil {
		usingTimestamp, err = common.GetTimestampInfo(insertObj.UsingTtlTimestamp().Timestamp(), params)
		if err != nil {
			return nil, err
		}
	}

	var rowKeys []types.DynamicValue
	for _, key := range table.PrimaryKeys {
		var val types.DynamicValue
		for _, value := range values {
			if value.Column().Name == key.Name {
				val = value.Value()
				break
			}
		}
		if val == nil {
			return nil, fmt.Errorf("missing value for primary key `%s`", key.Name)
		}
		rowKeys = append(rowKeys, val)
	}

	builtParams, err := params.Build()
	if err != nil {
		return nil, err
	}

	st := types.NewPreparedInsertQuery(keyspaceName, tableName, ifNotExists, query.RawCql(), builtParams, values, rowKeys, usingTimestamp)

	return st, nil
}

func (t *InsertTranslator) Bind(st types.IPreparedQuery, values *types.QueryParameterValues, pv primitive.ProtocolVersion) (types.IExecutableQuery, error) {
	ist, ok := st.(*types.PreparedInsertQuery)
	if !ok {
		return nil, fmt.Errorf("cannot bind to %T", st)
	}
	tableConfig, err := t.schemaMappingConfig.GetTableSchema(ist.Keyspace(), ist.Table())
	if err != nil {
		return nil, err
	}

	rowKey, err := common.BindRowKey(tableConfig, ist.RowKeys, values)
	if err != nil {
		return nil, fmt.Errorf("key encoding failed: %w", err)
	}

	mutations := types.NewBigtableWriteMutation(ist.Keyspace(), ist.Table(), ist.CqlQuery(), types.IfSpec{IfNotExists: ist.IfNotExists}, types.QueryTypeInsert, rowKey)
	err = common.BindMutations(ist.Assignments, ist.UsingTimestamp, values, mutations)
	if err != nil {
		return nil, err
	}
	return mutations, nil
}
