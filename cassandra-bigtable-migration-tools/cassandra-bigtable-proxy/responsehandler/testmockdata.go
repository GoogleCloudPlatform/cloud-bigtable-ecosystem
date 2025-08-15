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
package responsehandler

import (
	btpb "cloud.google.com/go/bigtable/apiv2/bigtablepb"
	types "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	schemaMapping "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/schema-mapping"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/message"
)

var mockTableConfig = map[string]map[string]map[string]*types.Column{
	"test_keyspace": {"test_table": {
		"column1": &types.Column{
			ColumnName:   "column1",
			CQLType:      datatype.Varchar,
			IsPrimaryKey: true,
			PkPrecedence: 1,
		},
		"column2": &types.Column{
			ColumnName:   "column2",
			CQLType:      datatype.Blob,
			IsPrimaryKey: false,
		},
		"column3": &types.Column{
			ColumnName:   "column3",
			CQLType:      datatype.Boolean,
			IsPrimaryKey: false,
		},
		"column4": &types.Column{
			ColumnName:   "column4",
			CQLType:      datatype.NewListType(datatype.Varchar),
			IsPrimaryKey: false,
			IsCollection: true,
		},
		"column5": &types.Column{
			ColumnName:   "column5",
			CQLType:      datatype.Timestamp,
			IsPrimaryKey: false,
		},
		"column6": &types.Column{
			ColumnName:   "column6",
			CQLType:      datatype.Int,
			IsPrimaryKey: false,
		},
		"column7": &types.Column{
			ColumnName:   "column7",
			CQLType:      datatype.NewSetType(datatype.Varchar),
			IsPrimaryKey: false,
			IsCollection: true,
		},
		"column8": &types.Column{
			ColumnName:   "column8",
			CQLType:      datatype.NewMapType(datatype.Varchar, datatype.Boolean),
			IsPrimaryKey: false,
			IsCollection: true,
		},
		"column9": &types.Column{
			ColumnName:   "column9",
			CQLType:      datatype.Bigint,
			IsPrimaryKey: false,
		},
		"column10": &types.Column{
			ColumnName:   "column10",
			CQLType:      datatype.Varchar,
			IsPrimaryKey: true,
			PkPrecedence: 2,
		},
		"column11": &types.Column{
			ColumnName:   "column11",
			CQLType:      datatype.NewSetType(datatype.Varchar),
			IsPrimaryKey: false,
			IsCollection: true,
		},
	},
		"user_info": {
			"name": &types.Column{
				ColumnName:   "name",
				CQLType:      datatype.Varchar,
				IsPrimaryKey: true,
				PkPrecedence: 0,
				IsCollection: false,
			},
		}},
}

var mockPkMetadata = map[string]map[string][]types.Column{
	"test_keyspace": {"test_table": {
		{
			ColumnName:   "column1",
			CQLType:      datatype.Varchar,
			IsPrimaryKey: true,
			PkPrecedence: 1,
		},
		{
			ColumnName:   "column10",
			CQLType:      datatype.Varchar,
			IsPrimaryKey: true,
			PkPrecedence: 2,
		},
	},
		"user_info": {
			{
				ColumnName:   "name",
				CQLType:      datatype.Varchar,
				IsPrimaryKey: true,
				PkPrecedence: 0,
			},
		}},
}

func GetSchemaMappingConfig() *schemaMapping.SchemaMappingConfig {
	return &schemaMapping.SchemaMappingConfig{
		// Logger:          tt.fields.Logger,
		TablesMetaData:      mockTableConfig,
		PkMetadataCache:     mockPkMetadata,
		SystemColumnFamily:  "cf1",
		CounterColumnFamily: "ctrf",
	}
}

// Response Handler
var (
	extraInfo = []Maptype{
		{Key: "make", Value: []byte("Tesla")},
		{Key: "model", Value: []byte("Model S")},
	}
	tags = []Maptype{
		{Key: "Black", Value: []byte("")},
		{Key: "Red", Value: []byte("")},
	}
	// list_text = []Maptype{
	// 	{Key: "Black", Value: []byte("")},
	// 	{Key: "Red", Value: []byte("")},
	// }
)
var ResponseHandler_Input_Result_Success = &btpb.ExecuteQueryResponse_Results{
	Results: &btpb.PartialResultSet{
		PartialRows: &btpb.PartialResultSet_ProtoRowsBatch{
			ProtoRowsBatch: &btpb.ProtoRowsBatch{
				BatchData: []byte("\x12\n\x12\b33#Jenny\x12\xc6\x01\"\xc3\x01\n\x15\"\x13\n\x05\x12\x03age\n\n\x12\b\x00\x00\x00\x00\x00\x00\x00!\n\x15\"\x13\n\t\x12\abalance\n\x06\x12\x04G\x83\x87\xd6\n\x1c\"\x1a\n\f\x12\nbirth_date\n\n\x12\b\x00\x00\x01\x8f\xc9\tw\xc0\n\x12\"\x10\n\x06\x12\x04code\n\x06\x12\x04\x00\x00\x04\xd2\n\x1a\"\x18\n\n\x12\bcredited\n\n\x12\b@È@\x00\x00\x00\x00\n\x14\"\x12\n\v\x12\tis_active\n\x03\x12\x01\x01\n\x13\"\x11\n\x06\x12\x04name\n\a\x12\x05Jenny\n\x1a\"\x18\n\n\x12\bzip_code\n\n\x12\b\x00\x00\x00\x00\x00\x06\xef\xfe\x12/\"-\n\x13\"\x11\n\x06\x12\x04make\n\a\x12\x05Tesla\n\x16\"\x14\n\a\x12\x05model\n\t\x12\aModel S\x12\"\" \n\x0f\"\r\n\a\x12\x05Black\n\x02\x12\x00\n\r\"\v\n\x05\x12\x03Red\n\x02\x12\x00"),
			},
		},
	},
}
var ResponseHandler_Input_CF_Success = []*btpb.ColumnMetadata{
	{Name: "_key"},
	{Name: "cf1"},
	{Name: "extra_info"},
	{Name: "tags"},
}
var ResponseHandler_Input_Query_Success = &QueryMetadata{
	TableName:           "user_info",
	Query:               "SELECT * FROM user_info;",
	KeyspaceName:        "xobni_derived",
	IsStar:              true,
	DefaultColumnFamily: "cf1",
}

var ResponseHandler_Input_Query_Selected_Select = QueryMetadata{
	TableName:           "user_info",
	Query:               "SELECT name FROM user_info;",
	KeyspaceName:        "xobni_derived",
	IsStar:              false,
	DefaultColumnFamily: "cf1",
	SelectedColumns: []schemaMapping.SelectedColumns{
		{
			Name: "name",
		},
	},
}

var ResponseHandler_Input_Query_Selected_Select_Map = QueryMetadata{
	TableName:           "user_info",
	Query:               "SELECT extra_info FROM user_info;",
	KeyspaceName:        "xobni_derived",
	IsStar:              false,
	DefaultColumnFamily: "cf1",
	SelectedColumns: []schemaMapping.SelectedColumns{
		{
			Name: "extra_info",
		},
	},
}
var ResponseHandler_Input_CF_Selected_Select = []*btpb.ColumnMetadata{
	{Name: "$col1"},
}
var ResponseHandler_Input_CF_Selected_Select_Map = []*btpb.ColumnMetadata{
	{Name: "extra_info"},
}
var ResponseHandler_Input_Result_Selected_Select = &btpb.ExecuteQueryResponse_Results{
	Results: &btpb.PartialResultSet{
		PartialRows: &btpb.PartialResultSet_ProtoRowsBatch{
			ProtoRowsBatch: &btpb.ProtoRowsBatch{
				BatchData: []byte("\x12\x06\x12\x04Lado"),
			},
		},
	},
}
var ResponseHandler_Input_Result_Selected_Select_Map = &btpb.ExecuteQueryResponse_Results{
	Results: &btpb.PartialResultSet{
		PartialRows: &btpb.PartialResultSet_ProtoRowsBatch{
			ProtoRowsBatch: &btpb.ProtoRowsBatch{
				BatchData: []byte("\x12D\"B\n\x14\"\x12\n\x06\x12\x04make\n\b\x12\x06Toyota\n\x16\"\x14\n\a\x12\x05model\n\t\x12\aCorolla\n\x12\"\x10\n\x06\x12\x04year\n\x06\x12\x042020"),
			},
		},
	},
}
var ResponseHandler_Success = map[string]map[string]interface{}{
	"0": {
		"_key":       []byte("33#Jenny"),
		"is_active":  []byte("\x01"),
		"birth_date": []byte("\x00\x00\x01\x8f\xc9\tw\xc0"),
		"code":       []byte("\x00\x00\x04\xd2"),
		"age":        []byte("\x00\x00\x00\x00\x00\x00\x00!"),
		"tags":       tags,
		"credited":   []byte("@È@\x00\x00\x00\x00"),
		"name":       []byte("Jenny"),
		"zip_code":   []byte("\x00\x00\x00\x00\x00\x06\xef\xfe"),
		"extra_info": extraInfo,
		"balance":    []byte("G\x83\x87\xd6"),
	},
}

var ResponseHandler_BuildMetatadata_Response = []*message.ColumnMetadata{
	{
		Keyspace: "xobni_derived",
		Table:    "user_info",
		Name:     "extra_info",
		Index:    int32(0),
		Type:     datatype.NewMapType(datatype.Varchar, datatype.Boolean),
	},
	{
		Keyspace: "xobni_derived",
		Table:    "user_info",
		Name:     "age",
		Index:    int32(1),
		Type:     datatype.Int,
	},
	{
		Keyspace: "xobni_derived",
		Table:    "user_info",
		Name:     "balance",
		Index:    int32(2),
		Type:     datatype.Float,
	},
	{
		Keyspace: "xobni_derived",
		Table:    "user_info",
		Name:     "credited",
		Index:    int32(3),
		Type:     datatype.Double,
	},
	{
		Keyspace: "xobni_derived",
		Table:    "user_info",
		Name:     "zip_code",
		Index:    int32(4),
		Type:     datatype.Bigint,
	},
	{
		Keyspace: "xobni_derived",
		Table:    "user_info",
		Name:     "name",
		Index:    int32(5),
		Type:     datatype.Varchar,
	},
	{
		Keyspace: "xobni_derived",
		Table:    "user_info",
		Name:     "tags",
		Index:    int32(6),
		Type:     datatype.NewSetType(datatype.Varchar),
	},
	{
		Keyspace: "xobni_derived",
		Table:    "user_info",
		Name:     "is_active",
		Index:    int32(7),
		Type:     datatype.Boolean,
	},
	{
		Keyspace: "xobni_derived",
		Table:    "user_info",
		Name:     "is_active",
		Index:    int32(8),
		Type:     datatype.Timestamp,
	},
	{
		Keyspace: "xobni_derived",
		Table:    "user_info",
		Name:     "code",
		Index:    int32(8),
		Type:     datatype.Int,
	},
}

var ResponseHandler_Selected_Select_Success = map[string]map[string]interface{}{
	"0": {
		"name": []byte("Lado"),
	},
}
var extra_info = []Maptype{
	{Key: "make", Value: []byte("Toyota")},
	{Key: "model", Value: []byte("Corolla")},
	{Key: "year", Value: []byte("2020")},
}
var ResponseHandler_Selected_Select_Success_Map = map[string]map[string]interface{}{
	"0": {
		"extra_info": extra_info,
	},
}

// Test case 5: Writetime column handling
var ResponseHandler_Input_Result_WithWritetime = &btpb.ExecuteQueryResponse_Results{
	Results: &btpb.PartialResultSet{
		PartialRows: &btpb.PartialResultSet_ProtoRowsBatch{
			ProtoRowsBatch: &btpb.ProtoRowsBatch{
				BatchData: []byte("\x12\n\x12\bklkrCCxl\x12\x0eb\f\b\xd3\xfb\xe4\xbe\x06\x10\x80\x8d\xb2\xda\x01"), // name and its writetime
			},
		},
	},
}

var ResponseHandler_Input_CF_WithWritetime = []*btpb.ColumnMetadata{
	{Name: "name"},
	{Name: "name_writetime"},
}

var ResponseHandler_Writetime_Success = map[string]map[string]interface{}{
	"0": {
		"name":           []byte("klkrCCxl"),
		"name_writetime": []byte("\x00\x060\x9a\x97\xa3\xd7\xd0"), // timestamp in bytes
	},
}

// Test case 6: Aggregate function handling
var ResponseHandler_Input_Result_WithAggregate = &btpb.ExecuteQueryResponse_Results{
	Results: &btpb.PartialResultSet{
		PartialRows: &btpb.PartialResultSet_ProtoRowsBatch{
			ProtoRowsBatch: &btpb.ProtoRowsBatch{
				BatchData: []byte("\x12\x08\x12\x06\x00\x00\x00\x00\x00\x00\x00\x05"), // count = 5
			},
		},
	},
}

var ResponseHandler_Input_CF_WithAggregate = []*btpb.ColumnMetadata{
	{Name: "count"},
}

var ResponseHandler_Aggregate_Success = map[string]map[string]interface{}{
	"0": {
		"count": []byte("\x00\x00\x00\x00\x00\x00\x00\x05"),
	},
}

// Test case 7: System column family handling
var ResponseHandler_Input_Result_SystemCF = &btpb.ExecuteQueryResponse_Results{
	Results: &btpb.PartialResultSet{
		PartialRows: &btpb.PartialResultSet_ProtoRowsBatch{
			ProtoRowsBatch: &btpb.ProtoRowsBatch{
				BatchData: []byte("\x12\x20\x12\x0btest_system\"\x11\n\x0f\x12\rSimpleStrategy"),
			},
		},
	},
}

var ResponseHandler_Input_CF_SystemCF = []*btpb.ColumnMetadata{
	{Name: "keyspace_name"},
	{Name: "strategy_class"},
}

var ResponseHandler_SystemCF_Success = map[string]map[string]interface{}{
	"0": {
		"keyspace_name":  []byte("test_system"),
		"strategy_class": []byte("SimpleStrategy"),
	},
}

// Test case 8: Nested array processing
var ResponseHandler_Input_Result_NestedArray = &btpb.ExecuteQueryResponse_Results{
	Results: &btpb.PartialResultSet{
		PartialRows: &btpb.PartialResultSet_ProtoRowsBatch{
			ProtoRowsBatch: &btpb.ProtoRowsBatch{
				BatchData: []byte("\x12\x30\x12.\n\x15\x12\x04key1\x12\x0bvalue1_nest\n\x15\x12\x04key2\x12\x0bvalue2_nest"),
			},
		},
	},
}

var ResponseHandler_Input_CF_NestedArray = []*btpb.ColumnMetadata{
	{Name: "nested_data"},
}

var nestedMapData = []Maptype{
	{Key: "key1", Value: []byte("value1_nest")},
	{Key: "key2", Value: []byte("value2_nest")},
}

var ResponseHandler_NestedArray_Success = map[string]map[string]interface{}{
	"0": {
		"nested_data": nestedMapData,
	},
}

// Note: Test cases 9 and 10 don't need fake data as they test empty/invalid batch data scenarios
