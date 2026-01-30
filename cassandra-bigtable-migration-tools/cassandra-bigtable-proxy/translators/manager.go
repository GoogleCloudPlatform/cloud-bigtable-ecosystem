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

package translators

import (
	"fmt"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	schemaMapping "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/metadata"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/translators/alter_translator"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/translators/common"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/translators/create_translator"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/translators/delete_translator"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/translators/desc_translator"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/translators/drop_translator"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/translators/insert_translator"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/translators/select_translator"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/translators/truncate_translator"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/translators/update_translator"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/translators/use_translator"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"go.uber.org/zap"
)

type TranslatorManager struct {
	Logger              *zap.Logger
	SchemaMappingConfig *schemaMapping.SchemaMetadata
	translators         map[types.QueryType]types.IQueryTranslator
	config              *types.BigtableConfig
}

func NewTranslatorManager(logger *zap.Logger, schemaMappingConfig *schemaMapping.SchemaMetadata, config *types.BigtableConfig) *TranslatorManager {
	// add more translators here
	translators := []types.IQueryTranslator{
		select_translator.NewSelectTranslator(schemaMappingConfig),
		insert_translator.NewInsertTranslator(schemaMappingConfig),
		update_translator.NewUpdateTranslator(schemaMappingConfig),
		delete_translator.NewDeleteTranslator(schemaMappingConfig),
		truncate_translator.NewTruncateTranslator(schemaMappingConfig),
		create_translator.NewCreateTranslator(schemaMappingConfig, config),
		alter_translator.NewAlterTranslator(schemaMappingConfig),
		drop_translator.NewDropTranslator(schemaMappingConfig),
		use_translator.NewUseTranslator(schemaMappingConfig),
		desc_translator.NewDescTranslator(schemaMappingConfig),
	}

	var tm = make(map[types.QueryType]types.IQueryTranslator)
	for _, t := range translators {
		if _, has := tm[t.QueryType()]; has {
			panic(fmt.Errorf("translator of type %s already registered", t.QueryType().String()))
		}
		tm[t.QueryType()] = t
	}
	return &TranslatorManager{
		Logger:              logger,
		SchemaMappingConfig: schemaMappingConfig,
		translators:         tm,
		config:              config,
	}
}

func (t *TranslatorManager) TranslateQuery(q *types.RawQuery, sessionKeyspace types.Keyspace) (types.IPreparedQuery, error) {
	queryTranslator, err := t.getTranslator(q.QueryType())
	if err != nil {
		return nil, err
	}

	preparedQuery, err := queryTranslator.Translate(q, sessionKeyspace)

	if err != nil {
		return nil, err
	}

	t.Logger.Debug("translated query", zap.String("cql", q.RawCql()), zap.String("btql", preparedQuery.BigtableQuery()))

	// ensure user doesn't try to drop or corrupt the schema mapping table
	if !preparedQuery.Keyspace().IsSystemKeyspace() && preparedQuery.Table() == t.config.SchemaMappingTable {
		return nil, fmt.Errorf("table name cannot be the same as the configured schema mapping table name '%s'", t.config.SchemaMappingTable)
	}

	return preparedQuery, err
}

func (t *TranslatorManager) BindQuery(st types.IPreparedQuery, cassandraValues []*primitive.Value, namedValues map[string]*primitive.Value, pv primitive.ProtocolVersion) (types.IExecutableQuery, error) {
	values, err := common.BindQueryParams(st.Parameters(), cassandraValues, namedValues, pv)
	if err != nil {
		return nil, err
	}
	return t.BindQueryParameters(st, values, pv)
}

func (t *TranslatorManager) BindQueryParameters(st types.IPreparedQuery, values *types.QueryParameterValues, pv primitive.ProtocolVersion) (types.IExecutableQuery, error) {
	queryTranslator, err := t.getTranslator(st.QueryType())
	if err != nil {
		return nil, err
	}
	return queryTranslator.Bind(st, values, pv)
}
