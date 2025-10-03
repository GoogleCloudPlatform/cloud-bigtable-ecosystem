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
	u "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/utilities"
	"go.uber.org/zap"
)

// GetSchemaMappingConfig builds and returns the mock schema configuration using constructors.
func GetSchemaMappingConfig(intRowKeyEncoding types.IntRowKeyEncodingType) *schemaMapping.SchemaMappingConfig {
	const systemColumnFamily = "cf1"

	testTableColumns := []*types.Column{
		{Name: "column1", CQLType: u.ParseCqlTypeOrDie("varchar"), KeyType: u.KEY_TYPE_PARTITION, PkPrecedence: 1},
		{Name: "column10", CQLType: u.ParseCqlTypeOrDie("varchar"), KeyType: u.KEY_TYPE_CLUSTERING, PkPrecedence: 2},
		{Name: "column2", CQLType: u.ParseCqlTypeOrDie("blob"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "column3", CQLType: u.ParseCqlTypeOrDie("boolean"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "column5", CQLType: u.ParseCqlTypeOrDie("timestamp"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "column6", CQLType: u.ParseCqlTypeOrDie("int"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "column7", CQLType: u.ParseCqlTypeOrDie("set<varchar>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "column8", CQLType: u.ParseCqlTypeOrDie("map<text,boolean>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "column9", CQLType: u.ParseCqlTypeOrDie("bigint"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "blob_col", CQLType: u.ParseCqlTypeOrDie("blob"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "text_col", CQLType: u.ParseCqlTypeOrDie("varchar"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "bool_col", CQLType: u.ParseCqlTypeOrDie("boolean"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "timestamp_col", CQLType: u.ParseCqlTypeOrDie("timestamp"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "int_col", CQLType: u.ParseCqlTypeOrDie("int"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "set_text_col", CQLType: u.ParseCqlTypeOrDie("set<varchar>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "map_text_bool_col", CQLType: u.ParseCqlTypeOrDie("map<varchar,boolean>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "bigint_col", CQLType: u.ParseCqlTypeOrDie("bigint"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "float_col", CQLType: u.ParseCqlTypeOrDie("float"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "double_col", CQLType: u.ParseCqlTypeOrDie("double"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "map_text_text", CQLType: u.ParseCqlTypeOrDie("map<varchar,varchar>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "list_text", CQLType: u.ParseCqlTypeOrDie("list<varchar>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "counter_col", CQLType: u.ParseCqlTypeOrDie("counter"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "frozen_list_text", CQLType: u.ParseCqlTypeOrDie("frozen<list<text>>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "frozen_set_text", CQLType: u.ParseCqlTypeOrDie("frozen<set<text>>"), KeyType: u.KEY_TYPE_REGULAR},
	}

	intTableColumns := []*types.Column{
		{Name: "num", CQLType: u.ParseCqlTypeOrDie("int"), KeyType: u.KEY_TYPE_PARTITION, PkPrecedence: 1},
		{Name: "big_num", CQLType: u.ParseCqlTypeOrDie("bigint"), KeyType: u.KEY_TYPE_CLUSTERING, PkPrecedence: 2},
		{Name: "name", CQLType: u.ParseCqlTypeOrDie("varchar"), KeyType: u.KEY_TYPE_REGULAR},
	}

	userInfoColumns := []*types.Column{
		{Name: "name", CQLType: u.ParseCqlTypeOrDie("text"), KeyType: u.KEY_TYPE_PARTITION, PkPrecedence: 1},
		{Name: "age", CQLType: u.ParseCqlTypeOrDie("int"), KeyType: u.KEY_TYPE_CLUSTERING, PkPrecedence: 2},
	}

	nonPrimitiveTableColumns := []*types.Column{
		{Name: "pk_1_text", CQLType: u.ParseCqlTypeOrDie("text"), KeyType: u.KEY_TYPE_PARTITION, PkPrecedence: 1},
		{Name: "map_text_text", CQLType: u.ParseCqlTypeOrDie("map<varchar,varchar>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "map_text_int", CQLType: u.ParseCqlTypeOrDie("map<varchar,int>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "map_text_float", CQLType: u.ParseCqlTypeOrDie("map<varchar,float>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "map_text_double", CQLType: u.ParseCqlTypeOrDie("map<varchar,double>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "map_text_timestamp", CQLType: u.ParseCqlTypeOrDie("map<varchar,timestamp>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "map_timestamp_text", CQLType: u.ParseCqlTypeOrDie("map<timestamp,varchar>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "map_timestamp_int", CQLType: u.ParseCqlTypeOrDie("map<timestamp,int>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "map_timestamp_boolean", CQLType: u.ParseCqlTypeOrDie("map<timestamp,boolean>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "map_timestamp_timestamp", CQLType: u.ParseCqlTypeOrDie("map<timestamp,timestamp>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "map_timestamp_bigint", CQLType: u.ParseCqlTypeOrDie("map<timestamp,bigint>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "map_timestamp_float", CQLType: u.ParseCqlTypeOrDie("map<timestamp,float>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "map_timestamp_double", CQLType: u.ParseCqlTypeOrDie("map<timestamp,double>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "set_text", CQLType: u.ParseCqlTypeOrDie("set<varchar>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "set_boolean", CQLType: u.ParseCqlTypeOrDie("set<boolean>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "set_int", CQLType: u.ParseCqlTypeOrDie("set<int>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "set_float", CQLType: u.ParseCqlTypeOrDie("set<float>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "set_double", CQLType: u.ParseCqlTypeOrDie("set<double>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "set_bigint", CQLType: u.ParseCqlTypeOrDie("set<bigint>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "set_timestamp", CQLType: u.ParseCqlTypeOrDie("set<timestamp>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "map_text_boolean", CQLType: u.ParseCqlTypeOrDie("map<varchar,boolean>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "map_text_bigint", CQLType: u.ParseCqlTypeOrDie("map<varchar,bigint>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "list_text", CQLType: u.ParseCqlTypeOrDie("list<varchar>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "list_int", CQLType: u.ParseCqlTypeOrDie("list<int>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "list_float", CQLType: u.ParseCqlTypeOrDie("list<float>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "list_double", CQLType: u.ParseCqlTypeOrDie("list<double>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "list_boolean", CQLType: u.ParseCqlTypeOrDie("list<boolean>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "list_timestamp", CQLType: u.ParseCqlTypeOrDie("list<timestamp>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "list_bigint", CQLType: u.ParseCqlTypeOrDie("list<bigint>"), KeyType: u.KEY_TYPE_REGULAR},
	}

	allTableConfigs := []*schemaMapping.TableConfig{
		schemaMapping.NewTableConfig("test_keyspace", "test_table", systemColumnFamily, intRowKeyEncoding, testTableColumns),
		schemaMapping.NewTableConfig("test_keyspace", "int_table", systemColumnFamily, intRowKeyEncoding, intTableColumns),
		schemaMapping.NewTableConfig("test_keyspace", "user_info", systemColumnFamily, intRowKeyEncoding, userInfoColumns),
		schemaMapping.NewTableConfig("test_keyspace", "non_primitive_table", systemColumnFamily, intRowKeyEncoding, nonPrimitiveTableColumns),
	}

	return schemaMapping.NewSchemaMappingConfig("schema_mapping", systemColumnFamily, zap.NewNop(), allTableConfigs)
}
