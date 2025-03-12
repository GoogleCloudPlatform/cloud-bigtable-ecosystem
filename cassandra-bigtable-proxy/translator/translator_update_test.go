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

	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	schemaMapping "github.com/ollionorg/cassandra-to-bigtable-proxy/schema-mapping"
	"go.uber.org/zap"
)

func TestTranslator_TranslateUpdateQuerytoBigtable(t *testing.T) {
	type fields struct {
		Logger *zap.Logger
	}
	type args struct {
		query string
	}

	valueBlob := "0x0000000000000003"
	setBlob, err := formatValues(valueBlob, "blob", 4)
	if err != nil {
		t.Errorf("formatValues() error = %v", err)
	}

	valueTimestamp := "2024-08-12T12:34:56Z"
	setTimestamp, err := formatValues(valueTimestamp, "timestamp", 4)
	if err != nil {
		t.Errorf("formatValues() error = %v", err)
	}

	valueInt := "123"
	setInt, err := formatValues(valueInt, "int", 4)
	if err != nil {
		t.Errorf("formatValues() error = %v", err)
	}

	valueBigInt := "1234567890"
	setBigInt, err := formatValues(valueBigInt, "bigint", 4)
	if err != nil {
		t.Errorf("formatValues() error = %v", err)
	}

	setTrueBool, err := formatValues("true", "boolean", 4)
	if err != nil {
		t.Errorf("formatValues() error = %v", err)
	}

	setSetText := "{item1,item2}"

	valueMapTextBool := "{key1:true,key2:false}"

	// Define value1 and value2 as text for WHERE clause
	value1 := "testText"
	value2 := "column10"

	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *UpdateQueryMap
		wantErr bool
	}{
		{
			name: "update blob column",
			args: args{
				query: "UPDATE test_keyspace.test_table SET column2 = '0x0000000000000003' WHERE column1 = 'testText' AND column10 = 'column10';",
			},
			wantErr: false,
			want: &UpdateQueryMap{
				ParamKeys: []string{"set1", "value1", "value2"},
				Params: map[string]interface{}{
					"set1":   setBlob,
					"value1": value1,
					"value2": value2,
				},
				RowKey:   "testText#column10",
				Keyspace: "test_keyspace",
			},
		},
		{
			name: "update boolean column",
			args: args{
				query: "UPDATE test_keyspace.test_table SET column3 = true WHERE column1 = 'testText' AND column10 = 'column10';",
			},
			wantErr: false,
			want: &UpdateQueryMap{
				ParamKeys: []string{"set1", "value1", "value2"},
				Params: map[string]interface{}{
					"set1":   setTrueBool,
					"value1": value1,
					"value2": value2,
				},
				RowKey:   "testText#column10",
				Keyspace: "test_keyspace",
			},
		},
		{
			name: "update timestamp column",
			args: args{
				query: "UPDATE test_keyspace.test_table SET column5 = '2024-08-12T12:34:56Z' WHERE column1 = 'testText' AND column10 = 'column10';",
			},
			wantErr: false,
			want: &UpdateQueryMap{
				ParamKeys: []string{"set1", "value1", "value2"},
				Params: map[string]interface{}{
					"set1":   setTimestamp,
					"value1": value1,
					"value2": value2,
				},
				RowKey:   "testText#column10",
				Keyspace: "test_keyspace",
			},
		},
		{
			name: "update int column",
			args: args{
				query: "UPDATE test_keyspace.test_table SET column6 = 123 WHERE column1 = 'testText' AND column10 = 'column10';",
			},
			wantErr: false,
			want: &UpdateQueryMap{
				ParamKeys: []string{"set1", "value1", "value2"},
				Params: map[string]interface{}{
					"set1":   setInt,
					"value1": value1,
					"value2": value2,
				},
				RowKey:   "testText#column10",
				Keyspace: "test_keyspace",
			},
		},
		{
			name: "update set<text> column",
			args: args{
				query: "UPDATE test_keyspace.test_table SET column7 = {'item1', 'item2'} WHERE column1 = 'testText' AND column10 = 'column10';",
			},
			wantErr: false,
			want: &UpdateQueryMap{
				ParamKeys: []string{"set1", "value1", "value2"},
				Params: map[string]interface{}{
					"set1":   setSetText,
					"value1": value1,
					"value2": value2,
				},
				RowKey:   "testText#column10",
				Keyspace: "test_keyspace",
			},
		},
		{
			name: "update map<text,boolean> column",
			args: args{
				query: "UPDATE test_keyspace.test_table SET column8 = {'key1': true, 'key2': false} WHERE column1 = 'testText' AND column10 = 'column10';",
			},
			wantErr: false,
			want: &UpdateQueryMap{
				ParamKeys: []string{"set1", "value1", "value2"},
				Params: map[string]interface{}{
					"set1":   valueMapTextBool,
					"value1": value1,
					"value2": value2,
				},
				RowKey:   "testText#column10",
				Keyspace: "test_keyspace",
			},
		},
		{
			name: "update bigint column",
			args: args{
				query: "UPDATE test_keyspace.test_table SET column9 = 1234567890 WHERE column1 = 'testText' AND column10 = 'column10';",
			},
			wantErr: false,
			want: &UpdateQueryMap{
				ParamKeys: []string{"set1", "value1", "value2"},
				Params: map[string]interface{}{
					"set1":   setBigInt,
					"value1": value1,
					"value2": value2,
				},
				RowKey:   "testText#column10",
				Keyspace: "test_keyspace",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schemaMapping := &schemaMapping.SchemaMappingConfig{
				Logger:          tt.fields.Logger,
				TablesMetaData:  mockSchemaMappingConfig,
				PkMetadataCache: mockPkMetadata,
			}

			tr := &Translator{
				Logger:              tt.fields.Logger,
				SchemaMappingConfig: schemaMapping,
			}
			got, err := tr.TranslateUpdateQuerytoBigtable(tt.args.query)
			if (err != nil) != tt.wantErr {
				t.Errorf("Translator.TranslateUpdateQuerytoBigtable() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got != nil && len(got.Params) > 0 && !reflect.DeepEqual(got.Params, tt.want.Params) {
				t.Errorf("Translator.TranslateUpdateQuerytoBigtable() = %v, want %v", got.Params, tt.want.Params)
			}

			if got != nil && len(got.ParamKeys) > 0 && !reflect.DeepEqual(got.ParamKeys, tt.want.ParamKeys) {
				t.Errorf("Translator.TranslateUpdateQuerytoBigtable() = %v, want %v", got.ParamKeys, tt.want.ParamKeys)
			}
			if got != nil && len(got.RowKey) > 0 && !reflect.DeepEqual(got.RowKey, tt.want.RowKey) {
				t.Errorf("Translator.TranslateUpdateQuerytoBigtable() = %v, want %v", got.RowKey, tt.want.RowKey)
			}

			if got != nil && !reflect.DeepEqual(got.Keyspace, tt.want.Keyspace) {
				t.Errorf("Translator.TranslateUpdateQuerytoBigtable() = %v, want %v", got.Keyspace, tt.want.Keyspace)
			}

		})
	}
}

func TestTranslator_BuildUpdatePrepareQuery(t *testing.T) {
	type fields struct {
		Logger              *zap.Logger
		SchemaMappingConfig *schemaMapping.SchemaMappingConfig
	}
	type args struct {
		columnsResponse []Column
		values          []*primitive.Value
		st              *UpdateQueryMap
		protocolV       primitive.ProtocolVersion
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *UpdateQueryMap
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
					{Contents: []byte("")},
					{Contents: []byte("")},
				},
				columnsResponse: []Column{
					{
						Name:         "column1",
						ColumnFamily: "",
						CQLType:      "",
					},
				},
				st: &UpdateQueryMap{
					Query:       "Update column2=? FROM test_table where column1=?",
					QueryType:   "Update",
					Table:       "test_table",
					Keyspace:    "test_keyspace",
					PrimaryKeys: []string{"column1"},
					RowKey:      "column1",
					Clauses: []Clause{
						{
							Column:       "column1",
							Operator:     "=",
							Value:        "",
							IsPrimaryKey: true,
						},
					},
					VariableMetadata: []*message.ColumnMetadata{
						{Name: "column1"},
					},
				},
			},
			want: &UpdateQueryMap{
				Query:       "Update column2=? FROM test_table where column1=?",
				QueryType:   "Update",
				Keyspace:    "test_keyspace",
				PrimaryKeys: []string{"column1"},
				RowKey:      "",
				Table:       "test_table",
				Clauses: []Clause{
					{
						Column:       "column1",
						Operator:     "=",
						Value:        "",
						IsPrimaryKey: true,
					},
				},
				Columns: []Column{
					{
						Name:         "column1",
						ColumnFamily: "",
						CQLType:      "",
					},
				},
				Values: []interface{}{[]interface{}(nil)},
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
			got, err := tr.BuildUpdatePrepareQuery(tt.args.columnsResponse, tt.args.values, tt.args.st, tt.args.protocolV)
			if (err != nil) != tt.wantErr {
				t.Errorf("Translator.BuildUpdatePrepareQuery() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got == tt.want {
				t.Errorf("Translator.BuildUpdatePrepareQuery() = %v, want %v", got, tt.want)
			}
		})
	}
}
