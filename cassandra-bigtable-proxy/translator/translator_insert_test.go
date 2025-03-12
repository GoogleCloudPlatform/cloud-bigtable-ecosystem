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

	"github.com/antlr4-go/antlr/v4"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	schemaMapping "github.com/ollionorg/cassandra-to-bigtable-proxy/schema-mapping"
	cql "github.com/ollionorg/cassandra-to-bigtable-proxy/translator/cqlparser"
	"go.uber.org/zap"
)

func parseInsertQuery() cql.IInsertContext {
	query := renameLiterals("INSERT INTO xobani_derived.user_info ( name ) VALUES ('Test')")
	lexer := cql.NewCqlLexer(antlr.NewInputStream(query))
	stream := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)
	p := cql.NewCqlParser(stream)
	insertObj := p.Insert()
	insertObj.KwInto()
	return insertObj
}

func parseFailedInsertQuery() cql.IInsertContext {
	query := renameLiterals("INSERT INTO xobani_derived.user_info ( name ) VALUES")
	lexer := cql.NewCqlLexer(antlr.NewInputStream(query))
	stream := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)
	p := cql.NewCqlParser(stream)
	insertObj := p.Insert()
	insertObj.KwInto()
	return insertObj
}

func Test_setParamsFromValues(t *testing.T) {
	var protocalV primitive.ProtocolVersion = 4
	insertObj := parseInsertQuery()
	response := make(map[string]interface{})
	val, _ := formatValues("Test", "text", protocalV)
	response["name"] = val
	var respValue []interface{}
	respValue = append(respValue, val)
	unencrypted := make(map[string]interface{})
	unencrypted["name"] = "Test"
	type args struct {
		input         cql.IInsertValuesSpecContext
		columns       []Column
		schemaMapping *schemaMapping.SchemaMappingConfig
		protocolV     primitive.ProtocolVersion
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
			name: "success",
			args: args{
				input: insertObj.InsertValuesSpec(),
				columns: []Column{
					{
						Name:         "name",
						ColumnFamily: "cf1",
						CQLType:      "text",
					},
				},
				schemaMapping: GetSchemaMappingConfig(),
				protocolV:     protocalV,
			},
			want:    response,
			want1:   respValue,
			want2:   unencrypted,
			wantErr: false,
		},
		{
			name: "failed",
			args: args{
				input: nil,
				columns: []Column{
					{
						Name:         "name",
						ColumnFamily: "cf1",
						CQLType:      "text",
					},
				},
				schemaMapping: GetSchemaMappingConfig(),
				protocolV:     protocalV,
			},
			want:    nil,
			want1:   nil,
			want2:   nil,
			wantErr: true,
		},
		{
			name: "failed",
			args: args{
				input: parseFailedInsertQuery().InsertValuesSpec(),
				columns: []Column{
					{
						Name:         "name",
						ColumnFamily: "cf1",
						CQLType:      "text",
					},
				},
				schemaMapping: GetSchemaMappingConfig(),
				protocolV:     protocalV,
			},
			want:    nil,
			want1:   nil,
			want2:   nil,
			wantErr: true,
		},
		{
			name: "failed",
			args: args{
				input:         parseFailedInsertQuery().InsertValuesSpec(),
				columns:       nil,
				schemaMapping: GetSchemaMappingConfig(),
				protocolV:     protocalV,
			},
			want:    nil,
			want1:   nil,
			want2:   nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, got2, err := setParamsFromValues(tt.args.input, tt.args.columns, tt.args.schemaMapping, tt.args.protocolV, "user_info", "test_keyspace")
			if (err != nil) != tt.wantErr {
				t.Errorf("setParamsFromValues() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("setParamsFromValues() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("setParamsFromValues() got1 = %v, want %v", got1, tt.want1)
			}
			if !reflect.DeepEqual(got2, tt.want2) {
				t.Errorf("setParamsFromValues() got2 = %v, want %v", got2, tt.want2)
			}
		})
	}
}

func TestTranslator_TranslateInsertQuerytoBigtable(t *testing.T) {
	var protocolV primitive.ProtocolVersion = 4
	type fields struct {
		Logger              *zap.Logger
		SchemaMappingConfig *schemaMapping.SchemaMappingConfig
	}
	type args struct {
		queryStr  string
		protocolV primitive.ProtocolVersion
	}

	// Define values and format them
	textValue := "test-text"
	blobValue := "0x0000000000000003"
	booleanValue := "true"
	timestampValue := "2024-08-12T12:34:56Z"
	intValue := "123"
	bigIntValue := "1234567890"
	column10 := "column10"

	formattedText, _ := formatValues(textValue, "text", protocolV)
	formattedBlob, _ := formatValues(blobValue, "blob", protocolV)
	formattedBoolean, _ := formatValues(booleanValue, "boolean", protocolV)
	formattedTimestamp, _ := formatValues(timestampValue, "timestamp", protocolV)
	formattedInt, _ := formatValues(intValue, "int", protocolV)
	formattedBigInt, _ := formatValues(bigIntValue, "bigint", protocolV)
	formattedcolumn10text, _ := formatValues(column10, "text", protocolV)

	values := []interface{}{
		formattedText,
		formattedBlob,
		formattedBoolean,
		formattedTimestamp,
		formattedInt,
		formattedBigInt,
		formattedcolumn10text,
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

	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *InsertQueryMap
		wantErr bool
	}{
		{
			name: "success",
			args: args{
				queryStr:  query,
				protocolV: protocolV,
			},
			fields: fields{
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			want: &InsertQueryMap{
				Query:     query,
				QueryType: "INSERT",
				Table:     "test_table",
				Keyspace:  "test_keyspace",
				Columns: []Column{
					{Name: "column1", ColumnFamily: "cf1", CQLType: "text", IsPrimaryKey: true},
					{Name: "column2", ColumnFamily: "cf1", CQLType: "blob", IsPrimaryKey: false},
					{Name: "column3", ColumnFamily: "cf1", CQLType: "boolean", IsPrimaryKey: false},
					{Name: "column5", ColumnFamily: "cf1", CQLType: "timestamp", IsPrimaryKey: false},
					{Name: "column6", ColumnFamily: "cf1", CQLType: "int", IsPrimaryKey: false},
					{Name: "column9", ColumnFamily: "cf1", CQLType: "bigint", IsPrimaryKey: false},
					{Name: "column10", ColumnFamily: "cf1", CQLType: "text", IsPrimaryKey: true},
				},
				Values:      values,
				Params:      response,
				ParamKeys:   []string{"column1", "column2", "column3", "column5", "column6", "column9", "column10"},
				PrimaryKeys: []string{"column1", "column10"}, // assuming column1 and column10 are primary keys
				RowKey:      "test-text#column10",            // assuming row key format
			},
			wantErr: false,
		},
		{
			name: "failed",
			args: args{
				queryStr:  "INSERT INTO test_keyspace.test_table",
				protocolV: protocolV,
			},
			fields: fields{
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "failed",
			args: args{
				queryStr:  "INSERT INTO test_keyspace.test_table (column1) VALUES ",
				protocolV: protocolV,
			},
			fields: fields{
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "failed",
			args: args{
				queryStr:  "INSERT INTO test_keyspace.test_table VALUES ('test')",
				protocolV: protocolV,
			},
			fields: fields{
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "failed because one primary column is not present",
			args: args{
				queryStr: "INSERT INTO test_keyspace.test_table (column1, column2, column3, column5, column6, column9) VALUES ('" +
					textValue + "', '" + blobValue + "', " + booleanValue + ", '" + timestampValue + "', " + intValue + ", " + bigIntValue + ")",
				protocolV: protocolV,
			},
			fields: fields{
				SchemaMappingConfig: GetSchemaMappingConfig(),
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
			got, err := tr.TranslateInsertQuerytoBigtable(tt.args.queryStr, tt.args.protocolV)
			if (err != nil) != tt.wantErr {
				t.Errorf("Translator.TranslateInsertQuerytoBigtable() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != nil && len(got.Params) > 0 && !reflect.DeepEqual(got.Params, tt.want.Params) {
				t.Errorf("Translator.TranslateInsertQuerytoBigtable() = %v, want %v", got.Params, tt.want.Params)
			}

			if got != nil && len(got.ParamKeys) > 0 && !reflect.DeepEqual(got.ParamKeys, tt.want.ParamKeys) {
				t.Errorf("Translator.TranslateInsertQuerytoBigtable() = %v, want %v", got.ParamKeys, tt.want.ParamKeys)
			}
			if got != nil && len(got.Values) > 0 && !reflect.DeepEqual(got.Values, tt.want.Values) {
				t.Errorf("Translator.TranslateInsertQuerytoBigtable() = %v, want %v", got.Values, tt.want.Values)
			}
			if got != nil && len(got.Columns) > 0 && !reflect.DeepEqual(got.Columns, tt.want.Columns) {
				t.Errorf("Translator.TranslateInsertQuerytoBigtable() = %v, want %v", got.Columns, tt.want.Columns)
			}
			if got != nil && len(got.RowKey) > 0 && !reflect.DeepEqual(got.RowKey, tt.want.RowKey) {
				t.Errorf("Translator.TranslateInsertQuerytoBigtable() = %v, want %v", got.RowKey, tt.want.RowKey)
			}

			if got != nil && !reflect.DeepEqual(got.Keyspace, tt.want.Keyspace) {
				t.Errorf("Translator.TranslateInsertQuerytoBigtable() = %v, want %v", got.Keyspace, tt.want.Keyspace)
			}
		})
	}
}

func TestTranslator_BuildInsertPrepareQuery(t *testing.T) {
	type fields struct {
		Logger              *zap.Logger
		SchemaMappingConfig *schemaMapping.SchemaMappingConfig
	}
	type args struct {
		columnsResponse []Column
		values          []*primitive.Value
		st              *InsertQueryMap
		protocolV       primitive.ProtocolVersion
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *InsertQueryMap
		wantErr bool
	}{
		{
			name: "Valid Input",
			fields: fields{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			args: args{
				values: []*primitive.Value{
					{Contents: []byte("123")},
				},
				st: &InsertQueryMap{
					Query:       "Insert into FROM test_table VALUES column1=?",
					QueryType:   "Insert",
					Table:       "test_table",
					Keyspace:    "test_keyspace",
					PrimaryKeys: []string{"column1"},
					RowKey:      "column1",
					VariableMetadata: []*message.ColumnMetadata{
						{Name: "column1"},
					},
				},
			},
			want: &InsertQueryMap{
				PrimaryKeys: []string{"column1"},
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
			got, err := tr.BuildInsertPrepareQuery(tt.args.columnsResponse, tt.args.values, tt.args.st, tt.args.protocolV)
			if (err != nil) != tt.wantErr {
				t.Errorf("Translator.BuildInsertPrepareQuery() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Translator.BuildInsertPrepareQuery() = %v, want %v", got, tt.want)
			}
		})
	}
}
