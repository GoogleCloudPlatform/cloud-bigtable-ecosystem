package select_translator

import (
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	schemaMapping "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/metadata"
	"go.uber.org/zap"
)

type SelectTranslator struct {
	schemaMappingConfig *schemaMapping.SchemaMetadata
	logger              *zap.Logger
}

func (t *SelectTranslator) QueryType() types.QueryType {
	return types.QueryTypeSelect
}

func NewSelectTranslator(schemaMappingConfig *schemaMapping.SchemaMetadata, logger *zap.Logger) types.IQueryTranslator {
	return &SelectTranslator{schemaMappingConfig: schemaMappingConfig, logger: logger}
}
