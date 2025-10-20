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
	"reflect"
	"testing"
	"time"

	types "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	schemaMapping "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/schema-mapping"
	cql "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/third_party/cqlparser"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/utilities"
	"github.com/antlr4-go/antlr/v4"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func createOrderedCodeKeyFromSliceOrDie(v []interface{}, e types.IntRowKeyEncodingType) string {
	result, err := createOrderedCodeKeyFromSlice(v, e)
	if err != nil {
		panic(err)
	}
	return result
}

func parseInsertQuery(query string) cql.IInsertContext {
	lexer := cql.NewCqlLexer(antlr.NewInputStream(query))
	stream := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)
	p := cql.NewCqlParser(stream)
	insertObj := p.Insert()
	insertObj.KwInto()
	return insertObj
}

func Test_setParamsFromValues(t *testing.T) {
	qctx := &types.QueryContext{
		Now:       time.Now(),
		ProtocolV: primitive.ProtocolVersion4,
	}
	response := make(map[string]interface{})
	val, _ := formatValues("Test", datatype.Varchar, qctx)
	specialCharVal, _ := formatValues("#!@#$%^&*()_+", datatype.Varchar, qctx)
	response["name"] = val
	specialCharResponse := make(map[string]interface{})
	specialCharResponse["name"] = specialCharVal
	var respValue []interface{}
	var emptyRespValue []interface{}
	respValue = append(respValue, val)
	specialCharRespValue := []interface{}{specialCharVal}
	unencrypted := make(map[string]interface{})
	unencrypted["name"] = "Test"
	unencryptedForSpecialChar := make(map[string]interface{})
	unencryptedForSpecialChar["name"] = "#!@#$%^&*()_+"
	type args struct {
		input           cql.IInsertValuesSpecContext
		columns         []*types.Column
		schemaMapping   *schemaMapping.SchemaMappingConfig
		isPreparedQuery bool
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]interface{}
		want1   []interface{}
		want2   map[string]interface{}
		wantErr bool
	}{
		{
			name: "success with special characters",
			args: args{
				input: parseInsertQuery("INSERT INTO xobani_derived.user_info ( name ) VALUES ('#!@#$%^&*()_+')").InsertValuesSpec(),
				columns: []*types.Column{
					{
						Name:         "name",
						ColumnFamily: "cf1",
						CQLType:      utilities.ParseCqlTypeOrDie("varchar"),
					},
				},
				schemaMapping:   GetSchemaMappingConfig(types.OrderedCodeEncoding),
				isPreparedQuery: false,
			},
			want:    specialCharResponse,
			want1:   specialCharRespValue,
			want2:   unencryptedForSpecialChar,
			wantErr: false,
		},
		{
			name: "success",
			args: args{
				input: parseInsertQuery("INSERT INTO xobani_derived.user_info ( name ) VALUES ('Test')").InsertValuesSpec(),
				columns: []*types.Column{
					{
						Name:         "name",
						ColumnFamily: "cf1",
						CQLType:      utilities.ParseCqlTypeOrDie("varchar"),
					},
				},
				schemaMapping:   GetSchemaMappingConfig(types.OrderedCodeEncoding),
				isPreparedQuery: false,
			},
			want:    response,
			want1:   respValue,
			want2:   unencrypted,
			wantErr: false,
		},
		{
			name: "success in prepare query",
			args: args{
				input: parseInsertQuery("INSERT INTO xobani_derived.user_info ( name ) VALUES ('?')").InsertValuesSpec(),
				columns: []*types.Column{
					{
						Name:         "name",
						ColumnFamily: "cf1",
						CQLType:      utilities.ParseCqlTypeOrDie("varchar"),
					},
				},
				schemaMapping:   GetSchemaMappingConfig(types.OrderedCodeEncoding),
				isPreparedQuery: true,
			},
			want:    make(map[string]interface{}),
			want1:   emptyRespValue,
			want2:   make(map[string]interface{}),
			wantErr: false,
		},
		{
			name: "failed",
			args: args{
				input: nil,
				columns: []*types.Column{
					{
						Name:         "name",
						ColumnFamily: "cf1",
						CQLType:      utilities.ParseCqlTypeOrDie("varchar"),
					},
				},
				schemaMapping:   GetSchemaMappingConfig(types.OrderedCodeEncoding),
				isPreparedQuery: false,
			},
			want:    nil,
			want1:   nil,
			want2:   nil,
			wantErr: true,
		},
		{
			name: "failed",
			args: args{
				input: parseInsertQuery("INSERT INTO xobani_derived.user_info ( name ) VALUES").InsertValuesSpec(),
				columns: []*types.Column{
					{
						Name:         "name",
						ColumnFamily: "cf1",
						CQLType:      utilities.ParseCqlTypeOrDie("varchar"),
					},
				},
				schemaMapping:   GetSchemaMappingConfig(types.OrderedCodeEncoding),
				isPreparedQuery: false,
			},
			want:    nil,
			want1:   nil,
			want2:   nil,
			wantErr: true,
		},
		{
			name: "failed",
			args: args{
				input:           parseInsertQuery("INSERT INTO xobani_derived.user_info ( name ) VALUES").InsertValuesSpec(),
				columns:         nil,
				schemaMapping:   GetSchemaMappingConfig(types.OrderedCodeEncoding),
				isPreparedQuery: false,
			},
			want:    nil,
			want1:   nil,
			want2:   nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc, _ := tt.args.schemaMapping.GetTableConfig("test_keyspace", "user_info")
			got, got1, got2, err := setParamsFromValues(tt.args.input, tt.args.columns, tc, tt.args.isPreparedQuery, qctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("setParamsFromValues() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("setParamsFromValues() got = %v, wantNewColumns %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("setParamsFromValues() got1 = %v, wantNewColumns %v", got1, tt.want1)
			}
			if !reflect.DeepEqual(got2, tt.want2) {
				t.Errorf("setParamsFromValues() got2 = %v, wantNewColumns %v", got2, tt.want2)
			}
		})
	}
}

func formatValueUnsafe(t *testing.T, value string, cqlType datatype.DataType, qctx *types.QueryContext) []byte {
	result, err := formatValues(value, cqlType, qctx)
	require.NoError(t, err)
	return result
}

func TestTranslator_TranslateInsertQuerytoBigtable(t *testing.T) {
	type fields struct {
		Logger              *zap.Logger
		SchemaMappingConfig *schemaMapping.SchemaMappingConfig
	}
	type args struct {
		queryStr        string
		isPreparedQuery bool
	}
	qctx := &types.QueryContext{
		Now:       time.Now(),
		ProtocolV: primitive.ProtocolVersion4,
	}

	// Define values and format them
	textValue := "test-text"
	blobValue := "0x0000000000000003"
	booleanValue := "true"
	timestampValue := "2024-08-12T12:34:56Z"
	intValue := "123"
	bigIntValue := "1234567890"
	column10 := "column10"

	formattedText, _ := formatValues(textValue, datatype.Varchar, qctx)
	formattedBlob, _ := formatValues(blobValue, datatype.Blob, qctx)
	formattedBoolean, _ := formatValues(booleanValue, datatype.Boolean, qctx)
	formattedTimestamp, _ := formatValues(timestampValue, datatype.Timestamp, qctx)
	formattedInt, _ := formatValues(intValue, datatype.Int, qctx)
	formattedBigInt, _ := formatValues(bigIntValue, datatype.Bigint, qctx)
	formattedcolumn10text, _ := formatValues(column10, datatype.Varchar, qctx)

	values := []interface{}{
		formattedBlob,
		formattedBoolean,
		formattedTimestamp,
		formattedInt,
		formattedBigInt,
	}

	response := map[string]interface{}{
		"column1":  formattedText,
		"column2":  formattedBlob,
		"column3":  formattedBoolean,
		"column5":  formattedTimestamp,
		"column6":  formattedInt,
		"column9":  formattedBigInt,
		"column10": formattedcolumn10text,
	}

	query := "INSERT INTO test_keyspace.test_table (column1, column2, column3, column5, column6, column9, column10) VALUES ('" +
		textValue + "', '" + blobValue + "', " + booleanValue + ", '" + timestampValue + "', " + intValue + ", " + bigIntValue + ", " + column10 + ")"

	preparedQuery := "INSERT INTO test_keyspace.test_table (column1, column2, column3, column5, column6, column9, column10) VALUES ('?', '?', '?', '?', '?', '?', '?')"
	// Query with USING TIMESTAMP
	preparedQueryWithTimestamp := "INSERT INTO test_keyspace.test_table (column1, column2, column3, column5, column6, column9, column10) VALUES (?, ?, ?, ?, ?, ?, ?) USING TIMESTAMP ?"

	now := time.Now().UTC()

	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *InsertQueryMapping
		wantErr bool
	}{
		{
			name: "success with prepared query and USING TIMESTAMP",
			args: args{
				queryStr:        preparedQueryWithTimestamp,
				isPreparedQuery: true,
			},
			fields: fields{
				SchemaMappingConfig: GetSchemaMappingConfig(types.OrderedCodeEncoding),
			},
			want: &InsertQueryMapping{
				Query:     preparedQueryWithTimestamp,
				QueryType: "INSERT",
				Table:     "test_table",
				Keyspace:  "test_keyspace",
				Columns: []*types.Column{
					{Name: "column1", ColumnFamily: "cf1", CQLType: utilities.ParseCqlTypeOrDie("varchar"), IsPrimaryKey: true},
					{Name: "column2", ColumnFamily: "cf1", CQLType: utilities.ParseCqlTypeOrDie("blob"), IsPrimaryKey: false},
					{Name: "column3", ColumnFamily: "cf1", CQLType: utilities.ParseCqlTypeOrDie("boolean"), IsPrimaryKey: false},
					{Name: "column5", ColumnFamily: "cf1", CQLType: utilities.ParseCqlTypeOrDie("timestamp"), IsPrimaryKey: false},
					{Name: "column6", ColumnFamily: "cf1", CQLType: utilities.ParseCqlTypeOrDie("int"), IsPrimaryKey: false},
					{Name: "column9", ColumnFamily: "cf1", CQLType: utilities.ParseCqlTypeOrDie("bigint"), IsPrimaryKey: false},
					{Name: "column10", ColumnFamily: "cf1", CQLType: utilities.ParseCqlTypeOrDie("varchar"), IsPrimaryKey: true},
				},
				Values:      nil, // undefined because this is a prepared query
				Params:      nil, // undefined because this is a prepared query
				ParamKeys:   []string{"column1", "column2", "column3", "column5", "column6", "column9", "column10"},
				PrimaryKeys: []string{"column1", "column10"},
				RowKey:      "", // undefined because this is a prepared query
				TimestampInfo: TimestampInfo{
					Timestamp:         0,
					HasUsingTimestamp: true,
					Index:             7,
				}, // Should include the custom timestamp parameter
			},
			wantErr: false,
		},
		{
			name: "success with prepared query",
			args: args{
				queryStr:        preparedQuery,
				isPreparedQuery: true,
			},
			fields: fields{
				SchemaMappingConfig: GetSchemaMappingConfig(types.OrderedCodeEncoding),
			},
			want: &InsertQueryMapping{
				Query:     preparedQuery,
				QueryType: "INSERT",
				Table:     "test_table",
				Keyspace:  "test_keyspace",
				Columns: []*types.Column{
					{Name: "column1", ColumnFamily: "cf1", CQLType: utilities.ParseCqlTypeOrDie("varchar"), IsPrimaryKey: true},
					{Name: "column2", ColumnFamily: "cf1", CQLType: utilities.ParseCqlTypeOrDie("blob"), IsPrimaryKey: false},
					{Name: "column3", ColumnFamily: "cf1", CQLType: utilities.ParseCqlTypeOrDie("boolean"), IsPrimaryKey: false},
					{Name: "column5", ColumnFamily: "cf1", CQLType: utilities.ParseCqlTypeOrDie("timestamp"), IsPrimaryKey: false},
					{Name: "column6", ColumnFamily: "cf1", CQLType: utilities.ParseCqlTypeOrDie("int"), IsPrimaryKey: false},
					{Name: "column9", ColumnFamily: "cf1", CQLType: utilities.ParseCqlTypeOrDie("bigint"), IsPrimaryKey: false},
					{Name: "column10", ColumnFamily: "cf1", CQLType: utilities.ParseCqlTypeOrDie("varchar"), IsPrimaryKey: true},
				},
				Values:      nil, // undefined because this is a prepared query
				Params:      nil, // undefined because this is a prepared query
				ParamKeys:   []string{"column1", "column2", "column3", "column5", "column6", "column9", "column10"},
				PrimaryKeys: []string{"column1", "column10"}, // assuming column1 and column10 are primary keys
				RowKey:      "",                              // assuming row key format
			},
			wantErr: false,
		},
		{
			name: "success",
			args: args{
				queryStr:        query,
				isPreparedQuery: false,
			},
			fields: fields{
				SchemaMappingConfig: GetSchemaMappingConfig(types.OrderedCodeEncoding),
			},
			want: &InsertQueryMapping{
				Query:     query,
				QueryType: "INSERT",
				Table:     "test_table",
				Keyspace:  "test_keyspace",
				Columns: []*types.Column{
					{Name: "column2", ColumnFamily: "cf1", CQLType: utilities.ParseCqlTypeOrDie("blob"), IsPrimaryKey: false},
					{Name: "column3", ColumnFamily: "cf1", CQLType: utilities.ParseCqlTypeOrDie("boolean"), IsPrimaryKey: false},
					{Name: "column5", ColumnFamily: "cf1", CQLType: utilities.ParseCqlTypeOrDie("timestamp"), IsPrimaryKey: false},
					{Name: "column6", ColumnFamily: "cf1", CQLType: utilities.ParseCqlTypeOrDie("int"), IsPrimaryKey: false},
					{Name: "column9", ColumnFamily: "cf1", CQLType: utilities.ParseCqlTypeOrDie("bigint"), IsPrimaryKey: false},
				},
				Values:      values,
				Params:      response,
				ParamKeys:   []string{"column1", "column2", "column3", "column5", "column6", "column9", "column10"},
				PrimaryKeys: []string{"column1", "column10"}, // assuming column1 and column10 are primary keys
				RowKey:      "test-text\x00\x01column10",     // assuming row key format
			},
			wantErr: false,
		},
		{
			name: "with keyspace in query, without default keyspace",
			args: args{
				queryStr:        "INSERT INTO test_keyspace.test_table (column1, column10) VALUES ('abc', 'pkval')",
				isPreparedQuery: false,
			},
			fields: fields{
				SchemaMappingConfig: GetSchemaMappingConfig(types.OrderedCodeEncoding),
			},
			want: &InsertQueryMapping{
				Query:     "INSERT INTO test_keyspace.test_table (column1, column10) VALUES ('abc', 'pkval')",
				QueryType: "INSERT",
				Table:     "test_table",
				Keyspace:  "test_keyspace",
				Params: map[string]interface{}{
					"column1":  []byte("abc"),
					"column10": []byte("pkval"),
				},
				PrimaryKeys: []string{"column1", "column10"},
				ParamKeys:   []string{"column1", "column10"},
				RowKey:      "abc\x00\x01pkval",
			},
			wantErr: false,
		},
		{
			name: "insert a map with special characters",
			args: args{
				queryStr:        "INSERT INTO test_keyspace.test_table (column1, column10, map_text_text) VALUES ('abc', 'pkval', {'foo': 'bar', 'key:': ':value', 'k}': '{v:k}'})",
				isPreparedQuery: false,
			},
			fields: fields{
				SchemaMappingConfig: GetSchemaMappingConfig(types.OrderedCodeEncoding),
			},
			want: &InsertQueryMapping{
				Query:     "INSERT INTO test_keyspace.test_table (column1, column10, map_text_text) VALUES ('abc', 'pkval', {'foo': 'bar', 'key:': ':value', 'k}': '{v:k}'})",
				QueryType: "INSERT",
				Table:     "test_table",
				Keyspace:  "test_keyspace",
				Params: map[string]interface{}{
					"column1":       []byte("abc"),
					"column10":      []byte("pkval"),
					"map_text_text": map[string]string{"foo": "bar", "key:": ":value", "k}": "{v:k}"},
				},
				Columns: []*types.Column{
					{
						Name:         "foo",
						ColumnFamily: "map_text_text",
						CQLType:      utilities.ParseCqlTypeOrDie("varchar"),
					},
					{
						Name:         "key:",
						ColumnFamily: "map_text_text",
						CQLType:      utilities.ParseCqlTypeOrDie("varchar"),
					},
					{
						Name:         "k}",
						ColumnFamily: "map_text_text",
						CQLType:      utilities.ParseCqlTypeOrDie("varchar"),
					},
				},
				Values: []interface{}{
					formatValueUnsafe(t, "bar", datatype.Varchar, qctx),
					formatValueUnsafe(t, ":value", datatype.Varchar, qctx),
					formatValueUnsafe(t, "{v:k}", datatype.Varchar, qctx),
				},
				DeleteColumnFamilies: []string{
					"map_text_text",
				},
				PrimaryKeys: []string{"column1", "column10"},
				ParamKeys:   []string{"column1", "column10", "map_text_text"},
				RowKey:      "abc\x00\x01pkval",
			},
			wantErr: false,
		},
		{
			name: "with keyspace in query, with default keyspace",
			args: args{
				queryStr:        "INSERT INTO test_keyspace.test_table (column1, column10) VALUES ('abc', 'pkval')",
				isPreparedQuery: false,
			},
			fields: fields{
				SchemaMappingConfig: GetSchemaMappingConfig(types.OrderedCodeEncoding),
			},
			want: &InsertQueryMapping{
				Query:     "INSERT INTO test_keyspace.test_table (column1, column10) VALUES ('abc', 'pkval')",
				QueryType: "INSERT",
				Table:     "test_table",
				Keyspace:  "test_keyspace",
				Params: map[string]interface{}{
					"column1":  []byte("abc"),
					"column10": []byte("pkval"),
				},
				Columns:     nil,
				PrimaryKeys: []string{"column1", "column10"},
				ParamKeys:   []string{"column1", "column10"},
				RowKey:      "abc\x00\x01pkval",
			},
			wantErr: false,
		},
		{
			name: "with double single quotes in a literal",
			args: args{
				queryStr:        "INSERT INTO test_keyspace.test_table (column1, column10, text_col) VALUES ('abc', 'pkval''s', 'text''s')",
				isPreparedQuery: false,
			},
			fields: fields{
				SchemaMappingConfig: GetSchemaMappingConfig(types.OrderedCodeEncoding),
			},
			want: &InsertQueryMapping{
				Query:     "INSERT INTO test_keyspace.test_table (column1, column10, text_col) VALUES ('abc', 'pkval''s', 'text''s')",
				QueryType: "INSERT",
				Table:     "test_table",
				Keyspace:  "test_keyspace",
				Params: map[string]interface{}{
					"column1":  []byte("abc"),
					"column10": []byte("pkval's"),
					"text_col": []byte("text's"),
				},
				Columns: []*types.Column{
					{
						Name:         "text_col",
						ColumnFamily: "cf1",
						CQLType:      utilities.ParseCqlTypeOrDie("varchar"),
					},
				},
				Values: []interface{}{
					formatValueUnsafe(t, "text's", datatype.Varchar, qctx),
				},
				PrimaryKeys: []string{"column1", "column10"},
				ParamKeys:   []string{"column1", "column10", "text_col"},
				RowKey:      "abc\x00\x01pkval's",
			},
			wantErr: false,
		},
		{
			name: "with empty key value",
			args: args{
				queryStr:        "INSERT INTO test_keyspace.test_table (column1, column10, text_col) VALUES ('', 'pkval''s', 'text''s')",
				isPreparedQuery: false,
			},
			fields: fields{
				SchemaMappingConfig: GetSchemaMappingConfig(types.OrderedCodeEncoding),
			},
			want: &InsertQueryMapping{
				Query:     "INSERT INTO test_keyspace.test_table (column1, column10, text_col) VALUES ('', 'pkval''s', 'text''s')",
				QueryType: "INSERT",
				Table:     "test_table",
				Keyspace:  "test_keyspace",
				Params: map[string]interface{}{
					"column1":  []byte(""),
					"column10": []byte("pkval's"),
					"text_col": []byte("text's"),
				},
				Columns: []*types.Column{
					{
						Name:         "text_col",
						ColumnFamily: "cf1",
						CQLType:      utilities.ParseCqlTypeOrDie("varchar"),
					},
				},
				Values: []interface{}{
					formatValueUnsafe(t, "text's", datatype.Varchar, qctx),
				},
				PrimaryKeys: []string{"column1", "column10"},
				ParamKeys:   []string{"column1", "column10", "text_col"},
				RowKey:      "\x00\x00\x00\x01pkval's",
			},
			wantErr: false,
		},
		{
			name: "without keyspace in query, with default keyspace",
			args: args{
				queryStr:        "INSERT INTO test_table (column1, column10) VALUES ('abc', 'pkval')",
				isPreparedQuery: false,
			},
			fields: fields{
				SchemaMappingConfig: GetSchemaMappingConfig(types.OrderedCodeEncoding),
			},
			want: &InsertQueryMapping{
				Query:     "INSERT INTO test_table (column1, column10) VALUES ('abc', 'pkval')",
				QueryType: "INSERT",
				Table:     "test_table",
				Keyspace:  "test_keyspace",
				Params: map[string]interface{}{
					"column1":  []byte("abc"),
					"column10": []byte("pkval"),
				},
				PrimaryKeys: []string{"column1", "column10"},
				ParamKeys:   []string{"column1", "column10"},
				RowKey:      "abc\x00\x01pkval",
			},
			wantErr: false,
		},
		{
			name: "without keyspace in query, with default keyspace",
			args: args{
				queryStr:        "INSERT INTO timestamp_row_keys (event_time, event_type) VALUES (toTimeStamp(now()), 'click')",
				isPreparedQuery: false,
			},
			fields: fields{
				SchemaMappingConfig: GetSchemaMappingConfig(types.OrderedCodeEncoding),
			},
			want: &InsertQueryMapping{
				Query:     "INSERT INTO timestamp_row_keys (event_time, event_type) VALUES (toTimeStamp(now()), 'click')",
				QueryType: "INSERT",
				Table:     "timestamp_row_keys",
				Keyspace:  "test_keyspace",
				Params: map[string]interface{}{
					"event_time": []byte("abc"),
					"event_type": []byte("pkval"),
				},
				PrimaryKeys: []string{"event_time"},
				ParamKeys:   []string{"event_time", "event_type"},
				RowKey:      createOrderedCodeKeyFromSliceOrDie([]interface{}{now.UnixMicro()}, types.OrderedCodeEncoding),
			},
			wantErr: false,
		},
		{
			name: "without keyspace in query, without default keyspace (should error)",
			args: args{
				queryStr:        "INSERT INTO test_table (column1) VALUES ('abc')",
				isPreparedQuery: false,
			},
			fields: fields{
				SchemaMappingConfig: GetSchemaMappingConfig(types.OrderedCodeEncoding),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid query syntax (should error)",
			args: args{
				queryStr:        "INSERT INTO test_keyspace.test_table",
				isPreparedQuery: false,
			},
			fields: fields{
				SchemaMappingConfig: GetSchemaMappingConfig(types.OrderedCodeEncoding),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "parser returns empty table (should error)",
			args: args{
				queryStr:        "INSERT INTO test_keyspace. VALUES ('abc')",
				isPreparedQuery: false,
			},
			fields: fields{
				SchemaMappingConfig: GetSchemaMappingConfig(types.OrderedCodeEncoding),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "parser returns empty keyspace (should error)",
			args: args{
				queryStr:        "INSERT INTO .test_table VALUES ('abc')",
				isPreparedQuery: false,
			},
			fields: fields{
				SchemaMappingConfig: GetSchemaMappingConfig(types.OrderedCodeEncoding),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "parser returns empty columns/values (should error)",
			args: args{
				queryStr:        "INSERT INTO test_keyspace.test_table () VALUES ()",
				isPreparedQuery: false,
			},
			fields: fields{
				SchemaMappingConfig: GetSchemaMappingConfig(types.OrderedCodeEncoding),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "keyspace does not exist (should error)",
			args: args{
				queryStr:        "INSERT INTO invalid_keyspace.test_table (column1) VALUES ('abc')",
				isPreparedQuery: false,
			},
			fields: fields{
				SchemaMappingConfig: GetSchemaMappingConfig(types.OrderedCodeEncoding),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "table does not exist (should error)",
			args: args{
				queryStr:        "INSERT INTO test_keyspace.invalid_table (column1) VALUES ('abc')",
				isPreparedQuery: false,
			},
			fields: fields{
				SchemaMappingConfig: GetSchemaMappingConfig(types.OrderedCodeEncoding),
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &Translator{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: tt.fields.SchemaMappingConfig,
			}
			got, err := tr.TranslateInsertQuerytoBigtable(tt.args.queryStr, tt.args.isPreparedQuery, "test_keyspace", qctx)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			// order of Columns is not deterministic so compare separately
			assert.ElementsMatch(t, tt.want.Columns, got.Columns)
			got.Columns = tt.want.Columns
			// order of Values is not deterministic so compare separately
			assert.ElementsMatch(t, tt.want.Values, got.Values)
			got.Values = tt.want.Values
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTranslator_BuildInsertPrepareQuery(t *testing.T) {
	qctx := &types.QueryContext{
		Now:       time.Now().UTC(),
		ProtocolV: primitive.ProtocolVersion4,
	}
	type fields struct {
		Logger              *zap.Logger
		SchemaMappingConfig *schemaMapping.SchemaMappingConfig
	}
	type args struct {
		columnsResponse []*types.Column
		values          []*primitive.Value
		st              *InsertQueryMapping
		protocolV       primitive.ProtocolVersion
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *InsertQueryMapping
		wantErr bool
	}{
		{
			name: "Valid Input",
			fields: fields{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: GetSchemaMappingConfig(types.OrderedCodeEncoding),
			},
			args: args{
				columnsResponse: []*types.Column{
					{
						Name:         "pk_1_text",
						ColumnFamily: "cf1",
						CQLType:      utilities.ParseCqlTypeOrDie("varchar"),
						IsPrimaryKey: true,
					},
				},
				values: []*primitive.Value{
					{Contents: []byte("")},
				},
				st: &InsertQueryMapping{
					Query:       "INSERT INTO test_keyspace.non_primitive_table(pk_1_text) VALUES (?)",
					QueryType:   "Insert",
					Table:       "non_primitive_table",
					Keyspace:    "test_keyspace",
					PrimaryKeys: []string{"pk_1_text"},
					RowKey:      "",
					Columns: []*types.Column{
						{
							Name:         "pk_1_text",
							ColumnFamily: "cf1",
							CQLType:      utilities.ParseCqlTypeOrDie("varchar"),
							IsPrimaryKey: true,
						},
					},
					Params: map[string]interface{}{
						"pk_1_text": &primitive.Value{Contents: []byte("123")},
					},
					ParamKeys:     []string{"pk_1_text"},
					IfNotExists:   false,
					TimestampInfo: TimestampInfo{},
					VariableMetadata: []*message.ColumnMetadata{
						{
							Name: "pk_1_text",
							Type: datatype.Varchar,
						},
					},
				},
				protocolV: 4,
			},
			want: &InsertQueryMapping{
				Query:       "INSERT INTO test_keyspace.test_table(pk_1_text) VALUES (?)",
				QueryType:   "Insert",
				Table:       "test_table",
				Keyspace:    "test_keyspace",
				PrimaryKeys: []string{"pk_1_text"},
				RowKey:      "",
				Columns: []*types.Column{
					{
						Name:         "pk_1_text",
						ColumnFamily: "cf1",
						CQLType:      utilities.ParseCqlTypeOrDie("varchar"),
						IsPrimaryKey: true,
					},
				},
				Values: []interface{}{
					&primitive.Value{Contents: []byte("123")},
				},
				Params: map[string]interface{}{
					"pk_1_text": &primitive.Value{Contents: []byte("123")},
				},
				ParamKeys:     []string{"pk_1_text"},
				IfNotExists:   false,
				TimestampInfo: TimestampInfo{},
				VariableMetadata: []*message.ColumnMetadata{
					{
						Name: "pk_1_text",
						Type: datatype.Varchar,
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &Translator{
				Logger:              tt.fields.Logger,
				SchemaMappingConfig: tt.fields.SchemaMappingConfig,
			}
			got, err := tr.BuildInsertPrepareQuery(tt.args.columnsResponse, tt.args.values, tt.args.st, qctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("Translator.BuildInsertPrepareQuery() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got.RowKey != tt.want.RowKey {
				t.Errorf("Translator.BuildInsertPrepareQuery() RowKey = %v, wantNewColumns %v", got.RowKey, tt.want.RowKey)
			}
		})
	}
}
