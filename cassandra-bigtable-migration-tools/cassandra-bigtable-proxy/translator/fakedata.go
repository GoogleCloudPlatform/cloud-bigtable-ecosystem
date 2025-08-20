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
	types "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	schemaMapping "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/schema-mapping"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/utilities"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"go.uber.org/zap"
)

// GetSchemaMappingConfig builds and returns the mock schema configuration using constructors.
func GetSchemaMappingConfig(encodeIntRowKeysWithBigEndian bool) *schemaMapping.SchemaMappingConfig {
	const systemColumnFamily = "cf1"

	testTableColumns := []*types.Column{
		{Name: "column1", CQLType: datatype.Varchar, KeyType: utilities.KEY_TYPE_PARTITION, PkPrecedence: 1},
		{Name: "column10", CQLType: datatype.Varchar, KeyType: utilities.KEY_TYPE_CLUSTERING, PkPrecedence: 2},
		{Name: "column2", CQLType: datatype.Blob, KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "column3", CQLType: datatype.Boolean, KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "column5", CQLType: datatype.Timestamp, KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "column6", CQLType: datatype.Int, KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "column7", CQLType: datatype.NewSetType(datatype.Varchar), KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "column8", CQLType: datatype.NewMapType(datatype.Varchar, datatype.Boolean), KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "column9", CQLType: datatype.Bigint, KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "blob_col", CQLType: datatype.Blob, KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "bool_col", CQLType: datatype.Boolean, KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "timestamp_col", CQLType: datatype.Timestamp, KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "int_col", CQLType: datatype.Int, KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "set_text_col", CQLType: datatype.NewSetType(datatype.Varchar), KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "map_text_bool_col", CQLType: datatype.NewMapType(datatype.Varchar, datatype.Boolean), KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "bigint_col", CQLType: datatype.Bigint, KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "float_col", CQLType: datatype.Float, KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "double_col", CQLType: datatype.Double, KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "map_text_text", CQLType: datatype.NewMapType(datatype.Varchar, datatype.Varchar), KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "list_text", CQLType: datatype.NewListType(datatype.Varchar), KeyType: utilities.KEY_TYPE_REGULAR},
	}

	intTableColumns := []*types.Column{
		{Name: "num", CQLType: datatype.Int, KeyType: utilities.KEY_TYPE_PARTITION, PkPrecedence: 1},
		{Name: "big_num", CQLType: datatype.Bigint, KeyType: utilities.KEY_TYPE_CLUSTERING, PkPrecedence: 2},
		{Name: "name", CQLType: datatype.Varchar, KeyType: utilities.KEY_TYPE_REGULAR},
	}

	userInfoColumns := []*types.Column{
		{Name: "name", CQLType: datatype.Varchar, KeyType: utilities.KEY_TYPE_PARTITION, PkPrecedence: 1},
		{Name: "age", CQLType: datatype.Int, KeyType: utilities.KEY_TYPE_CLUSTERING, PkPrecedence: 2},
	}

	nonPrimitiveTableColumns := []*types.Column{
		{Name: "pk_1_text", CQLType: datatype.Varchar, KeyType: utilities.KEY_TYPE_PARTITION, PkPrecedence: 1},
		{Name: "map_text_text", CQLType: datatype.NewMapType(datatype.Varchar, datatype.Varchar), KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "map_text_int", CQLType: datatype.NewMapType(datatype.Varchar, datatype.Int), KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "map_text_float", CQLType: datatype.NewMapType(datatype.Varchar, datatype.Float), KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "map_text_double", CQLType: datatype.NewMapType(datatype.Varchar, datatype.Double), KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "map_text_timestamp", CQLType: datatype.NewMapType(datatype.Varchar, datatype.Timestamp), KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "map_timestamp_text", CQLType: datatype.NewMapType(datatype.Timestamp, datatype.Varchar), KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "map_timestamp_int", CQLType: datatype.NewMapType(datatype.Timestamp, datatype.Int), KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "map_timestamp_boolean", CQLType: datatype.NewMapType(datatype.Timestamp, datatype.Boolean), KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "map_timestamp_timestamp", CQLType: datatype.NewMapType(datatype.Timestamp, datatype.Timestamp), KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "map_timestamp_bigint", CQLType: datatype.NewMapType(datatype.Timestamp, datatype.Bigint), KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "map_timestamp_float", CQLType: datatype.NewMapType(datatype.Timestamp, datatype.Float), KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "map_timestamp_double", CQLType: datatype.NewMapType(datatype.Timestamp, datatype.Double), KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "set_text", CQLType: datatype.NewSetType(datatype.Varchar), KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "set_boolean", CQLType: datatype.NewSetType(datatype.Boolean), KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "set_int", CQLType: datatype.NewSetType(datatype.Int), KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "set_float", CQLType: datatype.NewSetType(datatype.Float), KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "set_double", CQLType: datatype.NewSetType(datatype.Double), KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "set_bigint", CQLType: datatype.NewSetType(datatype.Bigint), KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "set_timestamp", CQLType: datatype.NewSetType(datatype.Timestamp), KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "map_text_boolean", CQLType: datatype.NewMapType(datatype.Varchar, datatype.Boolean), KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "map_text_bigint", CQLType: datatype.NewMapType(datatype.Varchar, datatype.Bigint), KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "list_text", CQLType: datatype.NewListType(datatype.Varchar), KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "list_int", CQLType: datatype.NewListType(datatype.Int), KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "list_float", CQLType: datatype.NewListType(datatype.Float), KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "list_double", CQLType: datatype.NewListType(datatype.Double), KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "list_boolean", CQLType: datatype.NewListType(datatype.Boolean), KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "list_timestamp", CQLType: datatype.NewListType(datatype.Timestamp), KeyType: utilities.KEY_TYPE_REGULAR},
		{Name: "list_bigint", CQLType: datatype.NewListType(datatype.Bigint), KeyType: utilities.KEY_TYPE_REGULAR},
	}

	allTableConfigs := []*schemaMapping.TableConfig{
		schemaMapping.NewTableConfig("test_keyspace", "test_table", systemColumnFamily, encodeIntRowKeysWithBigEndian, testTableColumns),
		schemaMapping.NewTableConfig("test_keyspace", "int_table", systemColumnFamily, encodeIntRowKeysWithBigEndian, intTableColumns),
		schemaMapping.NewTableConfig("test_keyspace", "user_info", systemColumnFamily, encodeIntRowKeysWithBigEndian, userInfoColumns),
		schemaMapping.NewTableConfig("test_keyspace", "non_primitive_table", systemColumnFamily, encodeIntRowKeysWithBigEndian, nonPrimitiveTableColumns),
	}

	return schemaMapping.NewSchemaMappingConfig(systemColumnFamily, zap.NewNop(), allTableConfigs)
}
