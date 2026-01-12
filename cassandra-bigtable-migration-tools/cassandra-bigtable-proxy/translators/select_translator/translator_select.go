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

package select_translator

import (
	"errors"
	"fmt"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/translators/common"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/utilities"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
)

func (t *SelectTranslator) Translate(query *types.RawQuery, sessionKeyspace types.Keyspace) (types.IPreparedQuery, error) {
	selectObj, err := query.Parser().Select_()
	if err != nil {
		return nil, err
	}

	keyspaceName, tableName, err := common.ParseTableSpec(selectObj.FromSpec().TableSpec(), sessionKeyspace)
	if err != nil {
		return nil, err
	}

	tableConfig, err := t.schemaMappingConfig.GetTableSchema(keyspaceName, tableName)
	if err != nil {
		return nil, err
	}

	selectClause, err := parseSelectClause(selectObj.SelectElements(), tableConfig)
	if err != nil {
		return nil, err
	}

	params := types.NewQueryParameters()

	conditions, err := common.ParseWhereClause(selectObj.WhereSpec(), tableConfig, params)
	if err != nil {
		return nil, err
	}

	var groupBy []string
	if selectObj.GroupSpec() != nil {
		groupBy = parseGroupByColumn(selectObj.GroupSpec())
	}
	var orderBy types.OrderBy
	if selectObj.OrderSpec() != nil {
		orderBy, err = parseOrderByFromSelect(selectObj.OrderSpec())
		if err != nil {
			// pass the original error to provide proper root cause of error.
			return nil, err
		}
	} else {
		orderBy.IsOrderBy = false
	}

	limitValue, err := parseLimitClause(selectObj.LimitSpec(), params)
	if err != nil {
		return nil, err
	}

	resultColumns := selectedColumnsToMetadata(tableConfig, selectClause)

	st := types.NewPreparedSelectQuery(keyspaceName, tableName, query.RawCql(), "", selectClause, conditions, params, orderBy, groupBy, limitValue, resultColumns)

	if !keyspaceName.IsSystemKeyspace() {
		translatedResult, err := createBigtableSql(t, st)
		if err != nil {
			return nil, err
		}
		st.TranslatedQuery = translatedResult
	} else {
		// we don't translate system queries because they aren't run against bigtable, and we don't have Bigtable SQL support for all the system queries that are run.
		st.TranslatedQuery = "system query"
	}
	return st, nil
}

func (t *SelectTranslator) Bind(st types.IPreparedQuery, values *types.QueryParameterValues, pv primitive.ProtocolVersion) (types.IExecutableQuery, error) {
	sst, ok := st.(*types.PreparedSelectQuery)
	if !ok {
		return nil, fmt.Errorf("cannot bind to %T", st)
	}

	if sst.LimitValue != nil {
		v, err := utilities.GetValueInt32(sst.LimitValue, values)
		if err != nil {
			return nil, err
		}
		if v <= 0 {
			return nil, errors.New("limit must be positive")
		}
	}
	query := types.NewExecutableSelectQuery(sst, pv, values)
	return query, nil
}
