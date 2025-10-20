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

	types "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	schemaMapping "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/schema-mapping"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestTranslator_TranslateUpdateQuerytoBigtable(t *testing.T) {
	const NO_ERROR_EXPECTED = ""
	type fields struct {
		Logger *zap.Logger
	}
	type args struct {
		query string
	}

	valueBlob := "0x0000000000000003"
	setBlob, err := formatValues(valueBlob, datatype.Blob, 4)
	if err != nil {
		t.Errorf("formatValues() error = %v", err)
	}

	valueTimestamp := "2024-08-12T12:34:56Z"
	setTimestamp, err := formatValues(valueTimestamp, datatype.Timestamp, 4)
	if err != nil {
		t.Errorf("formatValues() error = %v", err)
	}

	valueInt := "123"
	setInt, err := formatValues(valueInt, datatype.Int, 4)
	if err != nil {
		t.Errorf("formatValues() error = %v", err)
	}

	valueBigInt := "1234567890"
	setBigInt, err := formatValues(valueBigInt, datatype.Bigint, 4)
	if err != nil {
		t.Errorf("formatValues() error = %v", err)
	}

	setTrueBool, err := formatValues("true", datatype.Boolean, 4)
	if err != nil {
		t.Errorf("formatValues() error = %v", err)
	}

	tests := []struct {
		name            string
		fields          fields
		sessionKeyspace string
		args            args
		want            *UpdateQueryMapping
		wantErr         string
	}{
		{
			name: "update blob column",
			args: args{
				query: "UPDATE test_keyspace.test_table SET column2 = '0x0000000000000003' WHERE column1 = 'testText' AND column10 = 'column10';",
			},
			wantErr: NO_ERROR_EXPECTED,
			want: &UpdateQueryMapping{
				ParamKeys: []string{"set1", "value1", "value2"},
				Params: map[string]interface{}{
					"set1":   setBlob,
					"value1": "testText",
					"value2": "column10",
				},
				RowKey:           "testText\x00\x01column10",
				Keyspace:         "test_keyspace",
				ComplexOperation: map[string]*ComplexOperation{},
			},
		},
		{
			name: "update boolean column",
			args: args{
				query: "UPDATE test_keyspace.test_table SET column3 = true WHERE column1 = 'testText' AND column10 = 'column10';",
			},
			wantErr: NO_ERROR_EXPECTED,
			want: &UpdateQueryMapping{
				ParamKeys: []string{"set1", "value1", "value2"},
				Params: map[string]interface{}{
					"set1":   setTrueBool,
					"value1": "testText",
					"value2": "column10",
				},
				RowKey:           "testText\x00\x01column10",
				Keyspace:         "test_keyspace",
				ComplexOperation: map[string]*ComplexOperation{},
			},
		},
		{
			name: "update timestamp column",
			args: args{
				query: "UPDATE test_keyspace.test_table SET column5 = '2024-08-12T12:34:56Z' WHERE column1 = 'testText' AND column10 = 'column10';",
			},
			wantErr: NO_ERROR_EXPECTED,
			want: &UpdateQueryMapping{
				ParamKeys: []string{"set1", "value1", "value2"},
				Params: map[string]interface{}{
					"set1":   setTimestamp,
					"value1": "testText",
					"value2": "column10",
				},
				RowKey:           "testText\x00\x01column10",
				Keyspace:         "test_keyspace",
				ComplexOperation: map[string]*ComplexOperation{},
			},
		},
		{
			name: "update int column",
			args: args{
				query: "UPDATE test_keyspace.test_table SET column6 = 123 WHERE column1 = 'testText' AND column10 = 'column10';",
			},
			wantErr: NO_ERROR_EXPECTED,
			want: &UpdateQueryMapping{
				ParamKeys: []string{"set1", "value1", "value2"},
				Params: map[string]interface{}{
					"set1":   setInt,
					"value1": "testText",
					"value2": "column10",
				},
				RowKey:           "testText\x00\x01column10",
				Keyspace:         "test_keyspace",
				ComplexOperation: map[string]*ComplexOperation{},
			},
		},
		{
			name: "update set<varchar> column",
			args: args{
				query: "UPDATE test_keyspace.test_table SET column7 = {'item1', 'item2'} WHERE column1 = 'testText' AND column10 = 'column10';",
			},
			wantErr: NO_ERROR_EXPECTED,
			want: &UpdateQueryMapping{
				ParamKeys: []string{"set1", "value1", "value2"},
				Params: map[string]interface{}{
					"set1":   []string{"item1", "item2"},
					"value1": "testText",
					"value2": "column10",
				},
				RowKey:           "testText\x00\x01column10",
				Keyspace:         "test_keyspace",
				ComplexOperation: map[string]*ComplexOperation{},
			},
		},
		{
			name: "update map<varchar,boolean> column",
			args: args{
				query: "UPDATE test_keyspace.test_table SET column8 = {'key1': true, 'key2': false} WHERE column1 = 'testText' AND column10 = 'column10';",
			},
			wantErr: NO_ERROR_EXPECTED,
			want: &UpdateQueryMapping{
				ParamKeys: []string{"set1", "value1", "value2"},
				Params: map[string]interface{}{
					"set1":   map[string]string{"key1": "true", "key2": "false"},
					"value1": "testText",
					"value2": "column10",
				},
				RowKey:           "testText\x00\x01column10",
				Keyspace:         "test_keyspace",
				ComplexOperation: map[string]*ComplexOperation{},
			},
		},
		{
			name: "update bigint column",
			args: args{
				query: "UPDATE test_keyspace.test_table SET column9 = 1234567890 WHERE column1 = 'testText' AND column10 = 'column10';",
			},
			wantErr: NO_ERROR_EXPECTED,
			want: &UpdateQueryMapping{
				ParamKeys: []string{"set1", "value1", "value2"},
				Params: map[string]interface{}{
					"set1":   setBigInt,
					"value1": "testText",
					"value2": "column10",
				},
				RowKey:           "testText\x00\x01column10",
				Keyspace:         "test_keyspace",
				ComplexOperation: map[string]*ComplexOperation{},
			},
		},
		{
			name: "with keyspace in query, without default keyspace",
			args: args{
				query: "UPDATE test_keyspace.test_table SET column2 = 'abc' WHERE column10 = 'pkval' AND column1 = 'abc';",
			},
			fields:  fields{},
			wantErr: NO_ERROR_EXPECTED,
			want: &UpdateQueryMapping{
				ParamKeys: []string{"set1", "value1", "value2"},
				Params: map[string]interface{}{
					"set1":   []byte("abc"),
					"value1": "pkval",
					"value2": "abc",
				},
				RowKey:           "abc\x00\x01pkval",
				Keyspace:         "test_keyspace",
				ComplexOperation: map[string]*ComplexOperation{},
			},
		},
		{
			name: "append to set column with + operator",
			args: args{
				query: "UPDATE test_keyspace.test_table SET column7 = column7 + {'item3'} WHERE column1 = 'testText' AND column10 = 'column10';",
			},
			wantErr: NO_ERROR_EXPECTED,
			want: &UpdateQueryMapping{
				ParamKeys: []string{"set1", "value1", "value2"},
				Params: map[string]interface{}{
					"set1": ComplexAssignment{
						Column:    "column7",
						Operation: "+",
						Left:      "column7",
						Right:     []string{"item3"},
					},
					"value1": "testText",
					"value2": "column10",
				},
				RowKey:           "testText\x00\x01column10",
				Keyspace:         "test_keyspace",
				ComplexOperation: map[string]*ComplexOperation{},
			},
		},
		{
			name: "subtract from set column with - operator",
			args: args{
				query: "UPDATE test_keyspace.test_table SET column7 = column7 - {'item2'} WHERE column1 = 'testText' AND column10 = 'column10';",
			},
			wantErr: NO_ERROR_EXPECTED,
			want: &UpdateQueryMapping{
				ParamKeys: []string{"set1", "value1", "value2"},
				Params: map[string]interface{}{
					"set1": ComplexAssignment{
						Column:    "column7",
						Operation: "-",
						Left:      "column7",
						Right:     []string{"item2"},
					},
					"value1": "testText",
					"value2": "column10",
				},
				RowKey:           "testText\x00\x01column10",
				Keyspace:         "test_keyspace",
				ComplexOperation: map[string]*ComplexOperation{},
			},
		},
		{
			name: "counter operation",
			args: args{
				query: "UPDATE test_keyspace.test_table SET counter_col = counter_col + 1 WHERE column1 = 'testText' AND column10 = 'column10';",
			},
			wantErr: NO_ERROR_EXPECTED,
			want: &UpdateQueryMapping{
				ParamKeys: []string{"set1", "value1", "value2"},
				Params: map[string]interface{}{
					"set1": ComplexAssignment{
						Column:    "counter_col",
						Operation: "+",
						Left:      "counter_col",
						Right:     "1",
					},
					"value1": "testText",
					"value2": "column10",
				},
				ComplexOperation: map[string]*ComplexOperation{
					"counter_col": {
						IncrementType:  Increment,
						IncrementValue: 1,
					},
				},
				RowKey:   "testText\x00\x01column10",
				Keyspace: "test_keyspace",
			},
		},
		{
			name: "counter operation with invalid operator",
			args: args{
				query: "UPDATE test_keyspace.test_table SET counter_col = counter_col * 1 WHERE column1 = 'testText' AND column10 = 'column10';",
			},
			wantErr: "unsupported counter operation",
		},
		{
			name: "counter operation decrement",
			args: args{
				query: "UPDATE test_keyspace.test_table SET counter_col = counter_col - 9 WHERE column1 = 'testText' AND column10 = 'column10';",
			},
			wantErr: NO_ERROR_EXPECTED,
			want: &UpdateQueryMapping{
				ParamKeys: []string{"set1", "value1", "value2"},
				Params: map[string]interface{}{
					"set1": ComplexAssignment{
						Column:    "counter_col",
						Operation: "-",
						Left:      "counter_col",
						Right:     "9",
					},
					"value1": "testText",
					"value2": "column10",
				},
				ComplexOperation: map[string]*ComplexOperation{
					"counter_col": {
						IncrementType:  Decrement,
						IncrementValue: 9,
					},
				},
				RowKey:   "testText\x00\x01column10",
				Keyspace: "test_keyspace",
			},
		},
		{
			name: "counter operation increment a negative value",
			args: args{
				query: "UPDATE test_keyspace.test_table SET counter_col = counter_col + -9 WHERE column1 = 'testText' AND column10 = 'column10';",
			},
			wantErr: NO_ERROR_EXPECTED,
			want: &UpdateQueryMapping{
				ParamKeys: []string{"set1", "value1", "value2"},
				Params: map[string]interface{}{
					"set1": ComplexAssignment{
						Column:    "counter_col",
						Operation: "+",
						Left:      "counter_col",
						Right:     "-9",
					},
					"value1": "testText",
					"value2": "column10",
				},
				ComplexOperation: map[string]*ComplexOperation{
					"counter_col": {
						IncrementType:  Increment,
						IncrementValue: -9,
					},
				},
				RowKey:   "testText\x00\x01column10",
				Keyspace: "test_keyspace",
			},
		},
		{
			name: "with keyspace in query, with default keyspace",
			args: args{
				query: "UPDATE test_keyspace.test_table SET column2 = 'abc' WHERE column10 = 'pkval' AND column1 = 'abc';",
			},
			fields:  fields{},
			wantErr: NO_ERROR_EXPECTED,
			want: &UpdateQueryMapping{
				ParamKeys: []string{"set1", "value1", "value2"},
				Params: map[string]interface{}{
					"set1":   []byte("abc"),
					"value1": "pkval",
					"value2": "abc",
				},
				RowKey:           "abc\x00\x01pkval",
				Keyspace:         "test_keyspace",
				ComplexOperation: map[string]*ComplexOperation{},
			},
		},
		{
			name: "update with empty list assignment",
			args: args{
				query: "UPDATE test_keyspace.test_table SET column7 = [] WHERE column1 = 'testText' AND column10 = 'column10';",
			},
			wantErr: NO_ERROR_EXPECTED,
			want: &UpdateQueryMapping{
				ParamKeys: []string{"set1", "value1", "value2"},
				Params: map[string]interface{}{
					"set1":   []string(nil),
					"value1": "testText",
					"value2": "column10",
				},
				RowKey:           "testText\x00\x01column10",
				Keyspace:         "test_keyspace",
				ComplexOperation: map[string]*ComplexOperation{},
			},
		},
		{
			name: "update with list assignment",
			args: args{
				query: "UPDATE test_keyspace.test_table SET column7 = ['item1', 'item2'] WHERE column1 = 'testText' AND column10 = 'column10';",
			},
			wantErr: NO_ERROR_EXPECTED,
			want: &UpdateQueryMapping{
				ParamKeys: []string{"set1", "value1", "value2"},
				Params: map[string]interface{}{
					"set1":   []string{"item1", "item2"},
					"value1": "testText",
					"value2": "column10",
				},
				RowKey:           "testText\x00\x01column10",
				Keyspace:         "test_keyspace",
				ComplexOperation: map[string]*ComplexOperation{},
			},
		},
		{
			name: "update with map assignment",
			args: args{
				query: "UPDATE test_keyspace.test_table SET column8 = {'key1': true, 'key2': false} WHERE column1 = 'testText' AND column10 = 'column10';",
			},
			want: &UpdateQueryMapping{
				ParamKeys: []string{"set1", "value1", "value2"},
				Params: map[string]interface{}{
					"set1":   map[string]string{"key1": "true", "key2": "false"},
					"value1": "testText",
					"value2": "column10",
				},
				RowKey:           "testText\x00\x01column10",
				Keyspace:         "test_keyspace",
				ComplexOperation: map[string]*ComplexOperation{},
			},
		},
		{
			name: "update with set assignment",
			args: args{
				query: "UPDATE test_keyspace.test_table SET column7 = {'item1', 'item2'} WHERE column1 = 'testText' AND column10 = 'column10';",
			},
			want: &UpdateQueryMapping{
				ParamKeys: []string{"set1", "value1", "value2"},
				Params: map[string]interface{}{
					"set1":   []string{"item1", "item2"},
					"value1": "testText",
					"value2": "column10",
				},
				RowKey:           "testText\x00\x01column10",
				Keyspace:         "test_keyspace",
				ComplexOperation: map[string]*ComplexOperation{},
			},
		},
		{
			name: "attempt to update primary key with collection (should error)",
			args: args{
				query: "UPDATE test_keyspace.test_table SET column1 = ['item1'] WHERE column10 = 'column10';",
			},
			wantErr: "primary key not allowed to assignments",
		},
		{
			name: "invalid collection syntax (should error)",
			args: args{
				query: "UPDATE test_keyspace.test_table SET column7 = ['item1', WHERE column1 = 'testText';",
			},
			// note - this exact error doesn't matter
			wantErr: "not allowed",
		},
		{
			name: "collection assignment to non-collection column (should error)",
			args: args{
				query: "UPDATE test_keyspace.test_table SET column6 = ['item1'] WHERE column1 = 'testText';",
			},
			wantErr: "invalid syntax",
		},
		{
			name: "without keyspace in query, with default keyspace",
			args: args{
				query: "UPDATE test_table SET column2 = 'abc' WHERE column10 = 'pkval' AND column1 = 'abc';",
			},
			sessionKeyspace: "test_keyspace",
			fields:          fields{},
			wantErr:         "",
			want: &UpdateQueryMapping{
				ParamKeys: []string{"set1", "value1", "value2"},
				Params: map[string]interface{}{
					"set1":   []byte("abc"),
					"value1": "pkval",
					"value2": "abc",
				},
				RowKey:           "abc\x00\x01pkval",
				Keyspace:         "test_keyspace",
				ComplexOperation: map[string]*ComplexOperation{},
			},
		},
		{
			name: "invalid index update missing bracket (should error)",
			args: args{
				query: "UPDATE test_keyspace.test_table SET column71 = 'newItem' WHERE column1 = 'testText';",
			},
			wantErr: "unknown column 'column71'",
		},
		{
			name: "without keyspace in query, without default keyspace (should error)",
			args: args{
				query: "UPDATE test_table SET column2 = 'abc' WHERE column10 = 'pkval' AND column1 = 'abc';",
			},
			fields:  fields{},
			wantErr: "invalid input parameters found for keyspace",
		},
		{
			name: "without keyspace in query, but with session keyspace is ok",
			args: args{
				query: "UPDATE test_table SET column2 = 'abc' WHERE column10 = 'pkval' AND column1 = 'abc';",
			},
			sessionKeyspace: "test_keyspace",
			fields:          fields{},
			want: &UpdateQueryMapping{
				ParamKeys: []string{"set1", "value1", "value2"},
				Params: map[string]interface{}{
					"set1":   []byte("abc"),
					"value1": "pkval",
					"value2": "abc",
				},
				RowKey:           "abc\x00\x01pkval",
				Keyspace:         "test_keyspace",
				ComplexOperation: map[string]*ComplexOperation{},
			},
		},
		{
			name: "invalid query syntax (should error)",
			args: args{
				query: "UPDATE test_keyspace.test_table",
			},
			fields:  fields{},
			wantErr: "error parsing",
		},
		{
			name: "parser returns empty table (should error)",
			args: args{
				query: "UPDATE test_keyspace. SET column1 = 'abc' WHERE column10 = 'pkval';",
			},
			fields:  fields{},
			wantErr: "does not exist",
		},
		{
			name: "parser returns empty keyspace (should error)",
			args: args{
				query: "UPDATE .test_table SET column1 = 'abc' WHERE column10 = 'pkval';",
			},
			fields:  fields{},
			wantErr: "invalid input parameters found for keyspace",
		},
		{
			name: "parser returns empty set clause (should error)",
			args: args{
				query: "UPDATE test_keyspace.test_table SET WHERE column10 = 'pkval';",
			},
			fields:  fields{},
			wantErr: " ",
		},
		{
			name: "keyspace does not exist (should error)",
			args: args{
				query: "UPDATE invalid_keyspace.test_table SET column1 = 'abc' WHERE column10 = 'pkval';",
			},
			fields:  fields{},
			wantErr: "keyspace 'invalid_keyspace' does not exist",
		},
		{
			name: "table does not exist (should error)",
			args: args{
				query: "UPDATE test_keyspace.invalid_table SET column1 = 'abc' WHERE column10 = 'pkval';",
			},
			fields:  fields{},
			wantErr: "table invalid_table does not exist",
		},
		{
			name: "missing primary key in where clause (should error)",
			args: args{
				query: "UPDATE test_keyspace.test_table SET column2 = 'abc' WHERE column1 = 'testText'", // Missing column10
			},
			fields:  fields{},
			wantErr: "some primary key parts are missing: column10",
			want:    nil,
		},
		{
			name: "missing all primary keys in where clause (should error)",
			args: args{
				query: "UPDATE test_keyspace.test_table SET column2 = 'abc' WHERE column3 = true", // No PK columns
			},
			fields:  fields{},
			wantErr: "some primary key parts are missing:",
			want:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schemaMappingConfig := GetSchemaMappingConfig(types.OrderedCodeEncoding)

			tr := &Translator{
				Logger:              tt.fields.Logger,
				SchemaMappingConfig: schemaMappingConfig,
			}
			got, err := tr.TranslateUpdateQuerytoBigtable(tt.args.query, false, tt.sessionKeyspace)
			if tt.wantErr != NO_ERROR_EXPECTED {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want.RowKey, got.RowKey)
			assert.Equal(t, tt.want.Keyspace, got.Keyspace)
			assert.Equal(t, tt.want.ComplexOperation, got.ComplexOperation)
			assert.Equal(t, tt.want.Params, got.Params)
		})
	}
}

func TestTranslator_BuildUpdatePrepareQuery(t *testing.T) {
	type fields struct {
		Logger              *zap.Logger
		SchemaMappingConfig *schemaMapping.SchemaMappingConfig
	}
	type args struct {
		columnsResponse []*types.Column
		values          []*primitive.Value
		st              *UpdateQueryMapping
		protocolV       primitive.ProtocolVersion
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *UpdateQueryMapping
		wantErr bool
	}{
		{
			name: "Valid Input",
			fields: fields{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: GetSchemaMappingConfig(types.OrderedCodeEncoding),
			},
			args: args{
				values: []*primitive.Value{
					{Contents: []byte("")},
					{Contents: []byte("")},
				},
				columnsResponse: []*types.Column{
					{
						Name:         "pk_1_text",
						ColumnFamily: "",
						CQLType:      types.TypeVarchar,
					},
				},
				st: &UpdateQueryMapping{
					Query:       "Update blob_col=? FROM non_primitive_table where pk_1_text=?",
					QueryType:   "Update",
					Table:       "non_primitive_table",
					Keyspace:    "test_keyspace",
					PrimaryKeys: []string{"pk_1_text"},
					RowKey:      "pk_1_text_value", // Example RowKey based on pk_1_text
					Clauses: []types.Clause{
						{
							Column:       "pk_1_text",
							Operator:     "=",
							Value:        "",
							IsPrimaryKey: true,
						},
					},
					VariableMetadata: []*message.ColumnMetadata{
						{
							Name: "blob_col",
							Type: datatype.Blob,
						},
						{
							Name: "pk_1_text",
							Type: datatype.Varchar,
						},
					},
				},
			},
			want: &UpdateQueryMapping{
				Query:       "Update blob_col=? FROM non_primitive_table where pk_1_text=?",
				QueryType:   "Update",
				Keyspace:    "test_keyspace",
				PrimaryKeys: []string{"pk_1_text"},
				RowKey:      "",
				Table:       "non_primitive_table",
				Clauses: []types.Clause{
					{
						Column:       "pk_1_text",
						Operator:     "=",
						Value:        "",
						IsPrimaryKey: true,
					},
				},
				Columns: []*types.Column{
					{
						Name:         "blob_col",
						ColumnFamily: "",
						CQLType:      types.TypeBlob,
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
			// Comparing specific fields as the whole struct comparison might fail due to dynamic parts
			if got != nil && tt.want != nil {
				if got.Query != tt.want.Query {
					t.Errorf("Translator.BuildUpdatePrepareQuery() Query = %v, wantNewColumns %v", got.Query, tt.want.Query)
				}
				if !reflect.DeepEqual(got.PrimaryKeys, tt.want.PrimaryKeys) {
					t.Errorf("Translator.BuildUpdatePrepareQuery() PrimaryKeys = %v, wantNewColumns %v", got.PrimaryKeys, tt.want.PrimaryKeys)
				}
				if len(got.Clauses) != len(tt.want.Clauses) {
					t.Errorf("Translator.BuildUpdatePrepareQuery() Clauses length mismatch = %d, wantNewColumns %d", len(got.Clauses), len(tt.want.Clauses))
				} else {
					for i := range got.Clauses {
						if !reflect.DeepEqual(got.Clauses[i], tt.want.Clauses[i]) {
							t.Errorf("Translator.BuildUpdatePrepareQuery() Clause[%d] = %v, wantNewColumns %v", i, got.Clauses[i], tt.want.Clauses[i])
						}
					}
				}
			} else if !(got == nil && tt.want == nil) {
				t.Errorf("Translator.BuildUpdatePrepareQuery() = %v, wantNewColumns %v", got, tt.want)
			}
		})
	}
}
