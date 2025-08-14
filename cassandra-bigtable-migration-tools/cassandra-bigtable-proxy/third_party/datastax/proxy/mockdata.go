package proxy

import (
	types "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	schemaMapping "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/schema-mapping"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/message"
)

func GetSchemaMappingConfig() *schemaMapping.SchemaMappingConfig {
	return &schemaMapping.SchemaMappingConfig{
		Tables:             mockTableConfig,
		SystemColumnFamily: "cf1",
	}
}

var mockTableConfig = map[string]map[string]*schemaMapping.TableConfig{
	"test_keyspace": {
		"test_table": &schemaMapping.TableConfig{
			Keyspace: "test_keyspace",
			Name:     "test_table",
			Columns: map[string]*types.Column{
				"column1": &types.Column{
					Name:         "column1",
					CQLType:      datatype.Varchar,
					IsPrimaryKey: true,
					PkPrecedence: 1,
					Metadata: message.ColumnMetadata{
						Keyspace: "test_keyspace",
						Table:    "test_table",
						Name:     "column1",
						Index:    0,
						Type:     datatype.Varchar,
					},
				},
				"column2": &types.Column{
					Name:         "column2",
					CQLType:      datatype.Blob,
					IsPrimaryKey: false,
				},
				"column3": &types.Column{
					Name:         "column3",
					CQLType:      datatype.Boolean,
					IsPrimaryKey: false,
				},
				"column4": &types.Column{
					Name:         "column4",
					CQLType:      datatype.NewListType(datatype.Varchar),
					IsPrimaryKey: false,
					Metadata: message.ColumnMetadata{
						Keyspace: "test_keyspace",
						Table:    "test_table",
						Name:     "column4",
						Type:     datatype.NewListType(datatype.Varchar),
					},
				},
				"column5": &types.Column{
					Name:         "column5",
					CQLType:      datatype.Timestamp,
					IsPrimaryKey: false,
				},
				"column6": &types.Column{
					Name:         "column6",
					CQLType:      datatype.Int,
					IsPrimaryKey: false,
				},
				"column7": &types.Column{
					Name:         "column7",
					CQLType:      datatype.NewSetType(datatype.Varchar),
					IsPrimaryKey: false,
					Metadata: message.ColumnMetadata{
						Keyspace: "test_keyspace",
						Table:    "test_table",
						Name:     "column7",
						Type:     datatype.NewSetType(datatype.Varchar),
					},
				},
				"column8": &types.Column{
					Name:         "column8",
					CQLType:      datatype.NewMapType(datatype.Varchar, datatype.Boolean),
					IsPrimaryKey: false,
					Metadata: message.ColumnMetadata{
						Keyspace: "test_keyspace",
						Table:    "test_table",
						Name:     "column8",
						Type:     datatype.NewMapType(datatype.Varchar, datatype.Boolean),
					},
				},
				"column9": &types.Column{
					Name:         "column9",
					CQLType:      datatype.Bigint,
					IsPrimaryKey: false,
				},
				"column10": &types.Column{
					Name:         "column10",
					CQLType:      datatype.Varchar,
					IsPrimaryKey: true,
					PkPrecedence: 2,
				},
				"column11": &types.Column{
					Name:         "column11",
					CQLType:      datatype.NewSetType(datatype.Varchar),
					IsPrimaryKey: false,
				},
			},
			PrimaryKeys: []*types.Column{
				{
					Name:         "column1",
					CQLType:      datatype.Varchar,
					IsPrimaryKey: true,
					PkPrecedence: 1,
				},
				{
					Name:         "column10",
					CQLType:      datatype.Varchar,
					IsPrimaryKey: true,
					PkPrecedence: 2,
				},
			},
		},
		"user_info": &schemaMapping.TableConfig{
			Keyspace: "test_keyspace",
			Name:     "user_info",
			Columns: map[string]*types.Column{
				"name": &types.Column{
					Name:         "name",
					CQLType:      datatype.Varchar,
					IsPrimaryKey: true,
					PkPrecedence: 0,
					Metadata: message.ColumnMetadata{
						Keyspace: "user_info",
						Table:    "user_info",
						Name:     "name",
						Index:    0,
						Type:     datatype.Varchar,
					},
				},
				"age": &types.Column{
					Name:         "age",
					CQLType:      datatype.Varchar,
					IsPrimaryKey: false,
					PkPrecedence: 0,
					Metadata: message.ColumnMetadata{
						Keyspace: "user_info",
						Table:    "user_info",
						Name:     "age",
						Index:    1,
						Type:     datatype.Varchar,
					},
				},
			},
			PrimaryKeys: []*types.Column{
				{
					Name:         "name",
					CQLType:      datatype.Varchar,
					IsPrimaryKey: true,
					PkPrecedence: 0,
				},
			},
		},
	},
}
