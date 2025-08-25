package proxy

import (
	types "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	schemaMapping "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/schema-mapping"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/utilities"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"go.uber.org/zap"
)

func GetSchemaMappingConfig() *schemaMapping.SchemaMappingConfig {
	const systemColumnFamily = "cf1"

	testTableColumns := []*types.Column{
		{Name: "column1", CQLType: datatype.Varchar, KeyType: utilities.KEY_TYPE_PARTITION, PkPrecedence: 1},
		{Name: "column10", CQLType: datatype.Varchar, KeyType: utilities.KEY_TYPE_CLUSTERING, PkPrecedence: 2},
		{Name: "column2", CQLType: datatype.Blob, KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "column3", CQLType: datatype.Boolean, KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "column4", CQLType: datatype.NewListType(datatype.Varchar), KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "column5", CQLType: datatype.Timestamp, KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "column6", CQLType: datatype.Int, KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "column7", CQLType: datatype.NewSetType(datatype.Varchar), KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "column8", CQLType: datatype.NewMapType(datatype.Varchar, datatype.Boolean), KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "column9", CQLType: datatype.Bigint, KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "column11", CQLType: datatype.NewSetType(datatype.Varchar), KeyType: utilities.KEY_TYPE_REGULAR},
	}

	userInfoColumns := []*types.Column{
		{Name: "name", CQLType: datatype.Varchar, KeyType: utilities.KEY_TYPE_PARTITION, PkPrecedence: 0},
		{Name: "age", CQLType: datatype.Varchar, KeyType: utilities.KEY_TYPE_REGULAR},
	}

	allTableConfigs := []*schemaMapping.TableConfig{
		schemaMapping.NewTableConfig("test_keyspace", "test_table", systemColumnFamily, types.OrderedCodeEncoding, testTableColumns),
		schemaMapping.NewTableConfig("test_keyspace", "user_info", systemColumnFamily, types.OrderedCodeEncoding, userInfoColumns),
	}

	return schemaMapping.NewSchemaMappingConfig(systemColumnFamily, zap.NewNop(), allTableConfigs)
}
