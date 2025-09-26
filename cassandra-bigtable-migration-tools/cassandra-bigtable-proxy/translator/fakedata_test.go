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
		{Name: "column1", TypeInfo: u.ParseCqlTypeOrDie("varchar"), KeyType: u.KEY_TYPE_PARTITION, PkPrecedence: 1},
		{Name: "column10", TypeInfo: u.ParseCqlTypeOrDie("varchar"), KeyType: u.KEY_TYPE_CLUSTERING, PkPrecedence: 2},
		{Name: "column2", TypeInfo: u.ParseCqlTypeOrDie("blob"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "column3", TypeInfo: u.ParseCqlTypeOrDie("boolean"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "column5", TypeInfo: u.ParseCqlTypeOrDie("timestamp"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "column6", TypeInfo: u.ParseCqlTypeOrDie("int"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "column7", TypeInfo: u.ParseCqlTypeOrDie("set<varchar>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "column8", TypeInfo: u.ParseCqlTypeOrDie("map<text,boolean>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "column9", TypeInfo: u.ParseCqlTypeOrDie("bigint"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "blob_col", TypeInfo: u.ParseCqlTypeOrDie("blob"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "text_col", TypeInfo: u.ParseCqlTypeOrDie("varchar"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "bool_col", TypeInfo: u.ParseCqlTypeOrDie("boolean"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "timestamp_col", TypeInfo: u.ParseCqlTypeOrDie("timestamp"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "int_col", TypeInfo: u.ParseCqlTypeOrDie("int"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "set_text_col", TypeInfo: u.ParseCqlTypeOrDie("set<varchar>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "map_text_bool_col", TypeInfo: u.ParseCqlTypeOrDie("map<varchar,boolean>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "bigint_col", TypeInfo: u.ParseCqlTypeOrDie("bigint"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "float_col", TypeInfo: u.ParseCqlTypeOrDie("float"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "double_col", TypeInfo: u.ParseCqlTypeOrDie("double"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "map_text_text", TypeInfo: u.ParseCqlTypeOrDie("map<varchar,varchar>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "list_text", TypeInfo: u.ParseCqlTypeOrDie("list<varchar>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "counter_col", TypeInfo: u.ParseCqlTypeOrDie("counter"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "frozen_list_text", TypeInfo: u.ParseCqlTypeOrDie("frozen<list<text>>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "frozen_set_text", TypeInfo: u.ParseCqlTypeOrDie("frozen<set<text>>"), KeyType: u.KEY_TYPE_REGULAR},
	}

	intTableColumns := []*types.Column{
		{Name: "num", TypeInfo: u.ParseCqlTypeOrDie("int"), KeyType: u.KEY_TYPE_PARTITION, PkPrecedence: 1},
		{Name: "big_num", TypeInfo: u.ParseCqlTypeOrDie("bigint"), KeyType: u.KEY_TYPE_CLUSTERING, PkPrecedence: 2},
		{Name: "name", TypeInfo: u.ParseCqlTypeOrDie("varchar"), KeyType: u.KEY_TYPE_REGULAR},
	}

	userInfoColumns := []*types.Column{
		{Name: "name", TypeInfo: u.ParseCqlTypeOrDie("text"), KeyType: u.KEY_TYPE_PARTITION, PkPrecedence: 1},
		{Name: "age", TypeInfo: u.ParseCqlTypeOrDie("int"), KeyType: u.KEY_TYPE_CLUSTERING, PkPrecedence: 2},
	}

	nonPrimitiveTableColumns := []*types.Column{
		{Name: "pk_1_text", TypeInfo: u.ParseCqlTypeOrDie("text"), KeyType: u.KEY_TYPE_PARTITION, PkPrecedence: 1},
		{Name: "map_text_text", TypeInfo: u.ParseCqlTypeOrDie("map<varchar,varchar>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "map_text_int", TypeInfo: u.ParseCqlTypeOrDie("map<varchar,int>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "map_text_float", TypeInfo: u.ParseCqlTypeOrDie("map<varchar,float>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "map_text_double", TypeInfo: u.ParseCqlTypeOrDie("map<varchar,double>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "map_text_timestamp", TypeInfo: u.ParseCqlTypeOrDie("map<varchar,timestamp>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "map_timestamp_text", TypeInfo: u.ParseCqlTypeOrDie("map<timestamp,varchar>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "map_timestamp_int", TypeInfo: u.ParseCqlTypeOrDie("map<timestamp,int>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "map_timestamp_boolean", TypeInfo: u.ParseCqlTypeOrDie("map<timestamp,boolean>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "map_timestamp_timestamp", TypeInfo: u.ParseCqlTypeOrDie("map<timestamp,timestamp>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "map_timestamp_bigint", TypeInfo: u.ParseCqlTypeOrDie("map<timestamp,bigint>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "map_timestamp_float", TypeInfo: u.ParseCqlTypeOrDie("map<timestamp,float>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "map_timestamp_double", TypeInfo: u.ParseCqlTypeOrDie("map<timestamp,double>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "set_text", TypeInfo: u.ParseCqlTypeOrDie("set<varchar>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "set_boolean", TypeInfo: u.ParseCqlTypeOrDie("set<boolean>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "set_int", TypeInfo: u.ParseCqlTypeOrDie("set<int>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "set_float", TypeInfo: u.ParseCqlTypeOrDie("set<float>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "set_double", TypeInfo: u.ParseCqlTypeOrDie("set<double>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "set_bigint", TypeInfo: u.ParseCqlTypeOrDie("set<bigint>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "set_timestamp", TypeInfo: u.ParseCqlTypeOrDie("set<timestamp>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "map_text_boolean", TypeInfo: u.ParseCqlTypeOrDie("map<varchar,boolean>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "map_text_bigint", TypeInfo: u.ParseCqlTypeOrDie("map<varchar,bigint>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "list_text", TypeInfo: u.ParseCqlTypeOrDie("list<varchar>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "list_int", TypeInfo: u.ParseCqlTypeOrDie("list<int>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "list_float", TypeInfo: u.ParseCqlTypeOrDie("list<float>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "list_double", TypeInfo: u.ParseCqlTypeOrDie("list<double>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "list_boolean", TypeInfo: u.ParseCqlTypeOrDie("list<boolean>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "list_timestamp", TypeInfo: u.ParseCqlTypeOrDie("list<timestamp>"), KeyType: u.KEY_TYPE_REGULAR},
		{Name: "list_bigint", TypeInfo: u.ParseCqlTypeOrDie("list<bigint>"), KeyType: u.KEY_TYPE_REGULAR},
	}

	allTableConfigs := []*schemaMapping.TableConfig{
		schemaMapping.NewTableConfig("test_keyspace", "test_table", systemColumnFamily, intRowKeyEncoding, testTableColumns),
		schemaMapping.NewTableConfig("test_keyspace", "int_table", systemColumnFamily, intRowKeyEncoding, intTableColumns),
		schemaMapping.NewTableConfig("test_keyspace", "user_info", systemColumnFamily, intRowKeyEncoding, userInfoColumns),
		schemaMapping.NewTableConfig("test_keyspace", "non_primitive_table", systemColumnFamily, intRowKeyEncoding, nonPrimitiveTableColumns),
	}

	return schemaMapping.NewSchemaMappingConfig("schema_mapping", systemColumnFamily, zap.NewNop(), allTableConfigs)
}
