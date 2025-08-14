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
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/message"
)

func getMockTableConfigs(encodeIntRowKeysWithBigEndian bool) map[string]map[string]*schemaMapping.TableConfig {
	var result = map[string]map[string]*schemaMapping.TableConfig{"test_keyspace": {
		"test_table": &schemaMapping.TableConfig{
			Keyspace:                      "test_keyspace",
			Name:                          "test_table",
			SystemColumnFamily:            "cf1",
			EncodeIntRowKeysWithBigEndian: encodeIntRowKeysWithBigEndian,
			Columns: map[string]*types.Column{
				"column1": {
					Name:         "column1",
					CQLType:      datatype.Varchar,
					IsPrimaryKey: true,
					PkPrecedence: 1,
					Metadata: message.ColumnMetadata{
						Table: "test_table",
						Name:  "column1",
						Index: 0,
						Type:  datatype.Varchar,
					},
				},
				"column2": {
					Name:         "column2",
					CQLType:      datatype.Blob,
					IsPrimaryKey: false,
					Metadata: message.ColumnMetadata{
						Table: "test_table",
						Name:  "column2",
						Index: 1,
						Type:  datatype.Blob,
					},
				},
				"column3": {
					Name:         "column3",
					CQLType:      datatype.Boolean,
					IsPrimaryKey: false,
					Metadata: message.ColumnMetadata{
						Table: "test_table",
						Name:  "column3",
						Index: 3,
						Type:  datatype.Boolean,
					},
				},
				"column5": {
					Name:         "column5",
					CQLType:      datatype.Timestamp,
					IsPrimaryKey: false,
					Metadata: message.ColumnMetadata{
						Table: "test_table",
						Name:  "column5",
						Index: 4,
						Type:  datatype.Timestamp,
					},
				},
				"column6": {
					Name:         "column6",
					CQLType:      datatype.Int,
					IsPrimaryKey: false,
					Metadata: message.ColumnMetadata{
						Table: "test_table",
						Name:  "column6",
						Index: 5,
						Type:  datatype.Int,
					},
				},
				"column7": {
					Name:         "column7",
					CQLType:      datatype.NewSetType(datatype.Varchar),
					IsPrimaryKey: false,
					Metadata: message.ColumnMetadata{
						Table: "test_table",
						Name:  "column7",
						Index: 6,
						Type:  datatype.NewSetType(datatype.Varchar),
					},
				},
				"column8": {
					Name:         "column8",
					CQLType:      datatype.NewMapType(datatype.Varchar, datatype.Boolean),
					IsPrimaryKey: false,
					Metadata: message.ColumnMetadata{
						Table: "test_table",
						Name:  "column8",
						Index: 7,
						Type:  datatype.NewMapType(datatype.Varchar, datatype.Boolean),
					},
				},
				"column9": {
					Name:         "column9",
					CQLType:      datatype.Bigint,
					IsPrimaryKey: false,
					Metadata: message.ColumnMetadata{
						Table: "test_table",
						Name:  "column9",
						Index: 8,
						Type:  datatype.Bigint,
					},
				},
				"column10": {
					Name:         "column10",
					CQLType:      datatype.Varchar,
					IsPrimaryKey: true,
					PkPrecedence: 2,
					Metadata: message.ColumnMetadata{
						Table: "test_table",
						Name:  "pk_2_text",
						Index: 1,
						Type:  datatype.Varchar,
					},
				},
				"blob_col": {
					Name:         "blob_col",
					CQLType:      datatype.Blob,
					IsPrimaryKey: false,
					Metadata: message.ColumnMetadata{
						Table: "test_table",
						Name:  "blob_col",
						Index: 2,
						Type:  datatype.Blob,
					},
				},
				"bool_col": {
					Name:         "bool_col",
					CQLType:      datatype.Boolean,
					IsPrimaryKey: false,
					Metadata: message.ColumnMetadata{
						Table: "test_table",
						Name:  "bool_col",
						Index: 3,
						Type:  datatype.Boolean,
					},
				},
				"timestamp_col": {
					Name:         "timestamp_col",
					CQLType:      datatype.Timestamp,
					IsPrimaryKey: false,
					Metadata: message.ColumnMetadata{
						Table: "test_table",
						Name:  "timestamp_col",
						Index: 4,
						Type:  datatype.Timestamp,
					},
				},
				"int_col": {
					Name:         "int_col",
					CQLType:      datatype.Int,
					IsPrimaryKey: false,
					Metadata: message.ColumnMetadata{
						Table: "test_table",
						Name:  "int_col",
						Index: 5,
						Type:  datatype.Int,
					},
				},
				"set_text_col": {
					Name:         "set_text_col",
					CQLType:      datatype.NewSetType(datatype.Varchar),
					IsPrimaryKey: false,
					Metadata: message.ColumnMetadata{
						Table: "test_table",
						Name:  "set_text_col",
						Index: 6,
						Type:  datatype.NewSetType(datatype.Varchar),
					},
				},
				"map_text_bool_col": {
					Name:         "map_text_bool_col",
					CQLType:      datatype.NewMapType(datatype.Varchar, datatype.Boolean),
					IsPrimaryKey: false,
					Metadata: message.ColumnMetadata{
						Table: "test_table",
						Name:  "map_text_bool_col",
						Index: 7,
						Type:  datatype.NewMapType(datatype.Varchar, datatype.Boolean),
					},
				},
				"bigint_col": {
					Name:         "bigint_col",
					CQLType:      datatype.Bigint,
					IsPrimaryKey: false,
					Metadata: message.ColumnMetadata{
						Table: "test_table",
						Name:  "bigint_col",
						Index: 8,
						Type:  datatype.Bigint,
					},
				},
				"float_col": {
					Name:         "float_col",
					CQLType:      datatype.Float,
					IsPrimaryKey: false,
					Metadata: message.ColumnMetadata{
						Table: "test_table",
						Name:  "float_col",
						Index: 9,
						Type:  datatype.Float,
					},
				},
				"double_col": {
					Name:         "double_col",
					CQLType:      datatype.Double,
					IsPrimaryKey: false,
					Metadata: message.ColumnMetadata{
						Table: "test_table",
						Name:  "double_col",
						Index: 10,
						Type:  datatype.Double,
					},
				},
				"map_text_text": {
					Name:         "map_text_text",
					CQLType:      datatype.NewMapType(datatype.Varchar, datatype.Varchar),
					IsPrimaryKey: false,
					Metadata: message.ColumnMetadata{
						Table: "non_primitive_table",
						Name:  "map_text_text",
						Index: 0,
						Type:  datatype.NewMapType(datatype.Varchar, datatype.Varchar),
					},
				},
				"list_text": {
					Name:         "list_text",
					CQLType:      datatype.NewListType(datatype.Varchar),
					IsPrimaryKey: false,
					Metadata: message.ColumnMetadata{
						Table: "non_primitive_table",
						Name:  "list_text",
						Index: 21,
						Type:  datatype.NewListType(datatype.Varchar),
					},
				},
			},
			PrimaryKeys: []*types.Column{
				{
					Name:         "column1",
					CQLType:      datatype.Varchar,
					IsPrimaryKey: true,
					PkPrecedence: 1,
					Metadata: message.ColumnMetadata{
						Table: "test_table",
						Name:  "column1",
						Index: 0,
						Type:  datatype.Varchar,
					},
				},
				{
					Name:         "column10",
					CQLType:      datatype.Varchar,
					IsPrimaryKey: true,
					PkPrecedence: 2,
					Metadata: message.ColumnMetadata{
						Table: "test_table",
						Name:  "column10",
						Index: 9,
						Type:  datatype.Varchar,
					},
				},
			},
		},
		"int_table": &schemaMapping.TableConfig{
			Keyspace: "test_keyspace",
			Name:     "int_table",
			Columns: map[string]*types.Column{
				"num": {
					Name:         "num",
					CQLType:      datatype.Int,
					IsPrimaryKey: true,
					PkPrecedence: 1,
					Metadata: message.ColumnMetadata{
						Table: "int_table",
						Name:  "num",
						Index: 0,
						Type:  datatype.Int,
					},
				},
				"big_num": {
					Name:         "big_num",
					CQLType:      datatype.Bigint,
					IsPrimaryKey: true,
					PkPrecedence: 2,
					Metadata: message.ColumnMetadata{
						Table: "int_table",
						Name:  "big_num",
						Index: 0,
						Type:  datatype.Bigint,
					},
				},
				"name": {
					Name:         "name",
					CQLType:      datatype.Varchar,
					IsPrimaryKey: false,
					PkPrecedence: 1,
					Metadata: message.ColumnMetadata{
						Table: "int_table",
						Name:  "name",
						Index: 0,
						Type:  datatype.Varchar,
					},
				},
			},
			PrimaryKeys: []*types.Column{
				{
					Name:         "num",
					CQLType:      datatype.Int,
					IsPrimaryKey: true,
					PkPrecedence: 1,
					Metadata: message.ColumnMetadata{
						Table: "test_table",
						Name:  "num",
						Index: 0,
						Type:  datatype.Int,
					},
				},
				{
					Name:         "big_num",
					CQLType:      datatype.Bigint,
					IsPrimaryKey: true,
					PkPrecedence: 2,
					Metadata: message.ColumnMetadata{
						Table: "test_table",
						Name:  "big_num",
						Index: 9,
						Type:  datatype.Bigint,
					},
				},
			},
		},
		"user_info": &schemaMapping.TableConfig{
			Keyspace:                      "test_keyspace",
			Name:                          "user_info",
			SystemColumnFamily:            "cf1",
			EncodeIntRowKeysWithBigEndian: encodeIntRowKeysWithBigEndian,
			Columns: map[string]*types.Column{
				"name": {
					Name:         "name",
					CQLType:      datatype.Varchar,
					IsPrimaryKey: true,
					PkPrecedence: 1,
					Metadata: message.ColumnMetadata{
						Table: "user_info",
						Name:  "name",
						Index: 0,
						Type:  datatype.Varchar,
					},
				},
				"age": {
					Name:         "age",
					CQLType:      datatype.Int,
					IsPrimaryKey: true,
					PkPrecedence: 2,
					Metadata: message.ColumnMetadata{
						Table: "user_info",
						Name:  "age",
						Index: 1,
						Type:  datatype.Int,
					},
				},
			},
			PrimaryKeys: []*types.Column{
				{
					Name:         "name",
					CQLType:      datatype.Varchar,
					IsPrimaryKey: true,
					PkPrecedence: 1,
					Metadata: message.ColumnMetadata{
						Table: "user_info",
						Name:  "name",
						Index: 0,
						Type:  datatype.Varchar,
					},
				},
				{
					Name:         "age",
					CQLType:      datatype.Int,
					IsPrimaryKey: true,
					PkPrecedence: 2,
					Metadata: message.ColumnMetadata{
						Table: "user_info",
						Name:  "age",
						Index: 1,
						Type:  datatype.Int,
					},
				},
			},
		},
		"non_primitive_table": &schemaMapping.TableConfig{
			Keyspace:                      "test_keyspace",
			Name:                          "non_primitive_table",
			EncodeIntRowKeysWithBigEndian: encodeIntRowKeysWithBigEndian,
			Columns: map[string]*types.Column{
				"map_text_text": {
					Name:         "map_text_text",
					CQLType:      datatype.NewMapType(datatype.Varchar, datatype.Varchar),
					IsPrimaryKey: false,
					Metadata: message.ColumnMetadata{
						Table: "non_primitive_table",
						Name:  "map_text_text",
						Index: 0,
						Type:  datatype.NewMapType(datatype.Varchar, datatype.Varchar),
					},
				},
				"map_text_int": {
					Name:         "map_text_int",
					CQLType:      datatype.NewMapType(datatype.Varchar, datatype.Int),
					IsPrimaryKey: false,
					Metadata: message.ColumnMetadata{
						Table: "non_primitive_table",
						Name:  "map_text_int",
						Index: 1,
						Type:  datatype.NewMapType(datatype.Varchar, datatype.Int),
					},
				},
				"pk_1_text": {
					Name:         "pk_1_text",
					CQLType:      datatype.Varchar,
					IsPrimaryKey: true,
					PkPrecedence: 1,
					Metadata: message.ColumnMetadata{
						Table: "non_primitive_table",
						Name:  "pk_1_text",
						Index: 0,
						Type:  datatype.Varchar,
					},
				},
				"map_text_float": {
					Name:         "map_text_float",
					CQLType:      datatype.NewMapType(datatype.Varchar, datatype.Float),
					IsPrimaryKey: false,
					Metadata: message.ColumnMetadata{
						Table: "non_primitive_table",
						Name:  "map_text_float",
						Index: 2,
						Type:  datatype.NewMapType(datatype.Varchar, datatype.Float),
					},
				},
				"map_text_double": {
					Name:         "map_text_double",
					CQLType:      datatype.NewMapType(datatype.Varchar, datatype.Double),
					IsPrimaryKey: false,
					Metadata: message.ColumnMetadata{
						Table: "non_primitive_table",
						Name:  "map_text_double",
						Index: 3,
						Type:  datatype.NewMapType(datatype.Varchar, datatype.Double),
					},
				},
				"map_text_timestamp": {
					Name:         "map_text_timestamp",
					CQLType:      datatype.NewMapType(datatype.Varchar, datatype.Timestamp),
					IsPrimaryKey: false,
					Metadata: message.ColumnMetadata{
						Table: "non_primitive_table",
						Name:  "map_text_timestamp",
						Index: 4,
						Type:  datatype.NewMapType(datatype.Varchar, datatype.Timestamp),
					},
				},
				"map_timestamp_text": {
					Name:         "map_timestamp_text",
					CQLType:      datatype.NewMapType(datatype.Timestamp, datatype.Varchar),
					IsPrimaryKey: false,
					Metadata: message.ColumnMetadata{
						Table: "non_primitive_table",
						Name:  "map_timestamp_text",
						Index: 5,
						Type:  datatype.NewMapType(datatype.Timestamp, datatype.Varchar),
					},
				},
				"map_timestamp_int": {
					Name:         "map_timestamp_int",
					CQLType:      datatype.NewMapType(datatype.Timestamp, datatype.Int),
					IsPrimaryKey: false,
					Metadata: message.ColumnMetadata{
						Table: "non_primitive_table",
						Name:  "map_timestamp_int",
						Index: 6,
						Type:  datatype.NewMapType(datatype.Timestamp, datatype.Int),
					},
				},
				"map_timestamp_boolean": {
					Name:         "map_timestamp_boolean",
					CQLType:      datatype.NewMapType(datatype.Timestamp, datatype.Boolean),
					IsPrimaryKey: false,
					Metadata: message.ColumnMetadata{
						Table: "non_primitive_table",
						Name:  "map_timestamp_boolean",
						Index: 7,
						Type:  datatype.NewMapType(datatype.Timestamp, datatype.Boolean),
					},
				},
				"map_timestamp_timestamp": {
					Name:         "map_timestamp_timestamp",
					CQLType:      datatype.NewMapType(datatype.Timestamp, datatype.Timestamp),
					IsPrimaryKey: false,
					Metadata: message.ColumnMetadata{
						Table: "non_primitive_table",
						Name:  "map_timestamp_timestamp",
						Index: 8,
						Type:  datatype.NewMapType(datatype.Timestamp, datatype.Timestamp),
					},
				},
				"map_timestamp_bigint": {
					Name:         "map_timestamp_bigint",
					CQLType:      datatype.NewMapType(datatype.Timestamp, datatype.Bigint),
					IsPrimaryKey: false,
					Metadata: message.ColumnMetadata{
						Table: "non_primitive_table",
						Name:  "map_timestamp_bigint",
						Index: 9,
						Type:  datatype.NewMapType(datatype.Timestamp, datatype.Bigint),
					},
				},
				"map_timestamp_float": {
					Name:         "map_timestamp_float",
					CQLType:      datatype.NewMapType(datatype.Timestamp, datatype.Float),
					IsPrimaryKey: false,
					Metadata: message.ColumnMetadata{
						Table: "non_primitive_table",
						Name:  "map_timestamp_float",
						Index: 10,
						Type:  datatype.NewMapType(datatype.Timestamp, datatype.Float),
					},
				},
				"map_timestamp_double": {
					Name:         "map_timestamp_double",
					CQLType:      datatype.NewMapType(datatype.Timestamp, datatype.Double),
					IsPrimaryKey: false,
					Metadata: message.ColumnMetadata{
						Table: "non_primitive_table",
						Name:  "map_timestamp_double",
						Index: 11,
						Type:  datatype.NewMapType(datatype.Timestamp, datatype.Double),
					},
				},
				"set_text": {
					Name:         "set_text",
					CQLType:      datatype.NewSetType(datatype.Varchar),
					IsPrimaryKey: false,
					Metadata: message.ColumnMetadata{
						Table: "non_primitive_table",
						Name:  "set_text",
						Index: 12,
						Type:  datatype.NewSetType(datatype.Varchar),
					},
				},
				"set_boolean": {
					Name:         "set_boolean",
					CQLType:      datatype.NewSetType(datatype.Boolean),
					IsPrimaryKey: false,
					Metadata: message.ColumnMetadata{
						Table: "non_primitive_table",
						Name:  "set_boolean",
						Index: 13,
						Type:  datatype.NewSetType(datatype.Boolean),
					},
				},
				"set_int": {
					Name:         "set_int",
					CQLType:      datatype.NewSetType(datatype.Int),
					IsPrimaryKey: false,
					Metadata: message.ColumnMetadata{
						Table: "non_primitive_table",
						Name:  "set_int",
						Index: 14,
						Type:  datatype.NewSetType(datatype.Int),
					},
				},
				"set_float": {
					Name:         "set_float",
					CQLType:      datatype.NewSetType(datatype.Float),
					IsPrimaryKey: false,
					Metadata: message.ColumnMetadata{
						Table: "non_primitive_table",
						Name:  "set_float",
						Index: 15,
						Type:  datatype.NewSetType(datatype.Float),
					},
				},
				"set_double": {
					Name:         "set_double",
					CQLType:      datatype.NewSetType(datatype.Double),
					IsPrimaryKey: false,
					Metadata: message.ColumnMetadata{
						Table: "non_primitive_table",
						Name:  "set_double",
						Index: 16,
						Type:  datatype.NewSetType(datatype.Double),
					},
				},
				"set_bigint": {
					Name:         "set_bigint",
					CQLType:      datatype.NewSetType(datatype.Bigint),
					IsPrimaryKey: false,
					Metadata: message.ColumnMetadata{
						Table: "non_primitive_table",
						Name:  "set_bigint",
						Index: 17,
						Type:  datatype.NewSetType(datatype.Bigint),
					},
				},
				"set_timestamp": {
					Name:         "set_timestamp",
					CQLType:      datatype.NewSetType(datatype.Timestamp),
					IsPrimaryKey: false,
					Metadata: message.ColumnMetadata{
						Table: "non_primitive_table",
						Name:  "set_timestamp",
						Index: 18,
						Type:  datatype.NewSetType(datatype.Timestamp),
					},
				},
				"map_text_boolean": {
					Name:         "map_text_boolean",
					CQLType:      datatype.NewMapType(datatype.Varchar, datatype.Boolean),
					IsPrimaryKey: false,
					Metadata: message.ColumnMetadata{
						Table: "non_primitive_table",
						Name:  "map_text_boolean",
						Index: 19,
						Type:  datatype.NewMapType(datatype.Varchar, datatype.Boolean),
					},
				},
				"map_text_bigint": {
					Name:         "map_text_bigint",
					CQLType:      datatype.NewMapType(datatype.Varchar, datatype.Bigint),
					IsPrimaryKey: false,
					Metadata: message.ColumnMetadata{
						Table: "non_primitive_table",
						Name:  "map_text_bigint",
						Index: 20,
						Type:  datatype.NewMapType(datatype.Varchar, datatype.Bigint),
					},
				},
				"list_text": {
					Name:         "list_text",
					CQLType:      datatype.NewListType(datatype.Varchar),
					IsPrimaryKey: false,
					Metadata: message.ColumnMetadata{
						Table: "non_primitive_table",
						Name:  "list_text",
						Index: 21,
						Type:  datatype.NewListType(datatype.Varchar),
					},
				},
				"list_int": {
					Name:         "list_int",
					CQLType:      datatype.NewListType(datatype.Int),
					IsPrimaryKey: false,
					Metadata: message.ColumnMetadata{
						Table: "non_primitive_table",
						Name:  "list_int",
						Index: 22,
						Type:  datatype.NewListType(datatype.Int),
					},
				},
				"list_float": {
					Name:         "list_float",
					CQLType:      datatype.NewListType(datatype.Float),
					IsPrimaryKey: false,
					Metadata: message.ColumnMetadata{
						Table: "non_primitive_table",
						Name:  "list_float",
						Index: 23,
						Type:  datatype.NewListType(datatype.Float),
					},
				},
				"list_double": {
					Name:         "list_double",
					CQLType:      datatype.NewListType(datatype.Double),
					IsPrimaryKey: false,
					Metadata: message.ColumnMetadata{
						Table: "non_primitive_table",
						Name:  "list_double",
						Index: 24,
						Type:  datatype.NewListType(datatype.Double),
					},
				},
				"list_boolean": {
					Name:         "list_boolean",
					CQLType:      datatype.NewListType(datatype.Boolean),
					IsPrimaryKey: false,
					Metadata: message.ColumnMetadata{
						Table: "non_primitive_table",
						Name:  "list_boolean",
						Index: 25,
						Type:  datatype.NewListType(datatype.Boolean),
					},
				},
				"list_timestamp": {
					Name:         "list_timestamp",
					CQLType:      datatype.NewListType(datatype.Timestamp),
					IsPrimaryKey: false,
					Metadata: message.ColumnMetadata{
						Table: "non_primitive_table",
						Name:  "list_timestamp",
						Index: 26,
						Type:  datatype.NewListType(datatype.Timestamp),
					},
				},
				"list_bigint": {
					Name:         "list_bigint",
					CQLType:      datatype.NewListType(datatype.Bigint),
					IsPrimaryKey: false,
					Metadata: message.ColumnMetadata{
						Table: "non_primitive_table",
						Name:  "list_bigint",
						Index: 27,
						Type:  datatype.NewListType(datatype.Bigint),
					},
				},
			},
			// todo fill out pmks
			PrimaryKeys: []*types.Column{},
		},
	},
	}
	return result
}

func GetSchemaMappingConfig(encodeIntRowKeysWithBigEndian bool) *schemaMapping.SchemaMappingConfig {
	return &schemaMapping.SchemaMappingConfig{
		Tables:             getMockTableConfigs(encodeIntRowKeysWithBigEndian),
		SystemColumnFamily: "cf1",
	}
}
