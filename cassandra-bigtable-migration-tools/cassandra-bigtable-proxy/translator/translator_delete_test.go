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
	"strings"
	"testing"

	types "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	schemaMapping "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/schema-mapping"
	cql "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/third_party/cqlparser"
	"github.com/antlr4-go/antlr/v4"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func createSchemaMappingConfigs() *schemaMapping.SchemaMappingConfig {
	return &schemaMapping.SchemaMappingConfig{
		TablesMetaData:  mockSchemaMappingConfig,
		PkMetadataCache: mockPkMetadata,
	}
}

func TestTranslator_TranslateDeleteQuerytoBigtable(t *testing.T) {
	var protocalV primitive.ProtocolVersion = 4
	params := make(map[string]interface{})
	formattedValue, _ := formatValues("test", datatype.Varchar, protocalV)
	formattedValue2, _ := formatValues("15", datatype.Int, protocalV)
	params["value1"] = formattedValue
	params2 := make(map[string]interface{})
	params2["value1"] = formattedValue
	params2["value2"] = formattedValue2

	type fields struct {
		Logger              *zap.Logger
		SchemaMappingConfig *schemaMapping.SchemaMappingConfig
	}
	type args struct {
		queryStr string
	}

	tests := []struct {
		name                         string
		fields                       fields
		args                         args
		want                         *DeleteQueryMapping
		encodeIntValuesWithBigEndian bool
		wantErr                      bool
		defaultKeyspace              string
	}{
		{
			name: "simple DELETE query without WHERE clause",
			args: args{
				queryStr: "DELETE FROM test_keyspace.user_info",
			},
			fields: fields{
				SchemaMappingConfig: createSchemaMappingConfigs(),
			},
			want:            nil,
			wantErr:         true,
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "DELETE query with single WHERE clause missing primary key",
			args: args{
				queryStr: "DELETE FROM test_keyspace.user_info WHERE name='test'",
			},
			fields: fields{
				SchemaMappingConfig: createSchemaMappingConfigs(),
			},
			wantErr:         true,
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "DELETE query with multiple WHERE clauses",
			args: args{
				queryStr: "DELETE FROM test_keyspace.user_info WHERE name='test' AND age=15",
			},
			fields: fields{
				SchemaMappingConfig: createSchemaMappingConfigs(),
			},
			want: &DeleteQueryMapping{
				Query:     "DELETE FROM test_keyspace.user_info WHERE name='test' AND age=15",
				QueryType: "DELETE",
				Table:     "user_info",
				Keyspace:  "test_keyspace",
				Clauses: []types.Clause{
					{
						Column:       "name",
						Operator:     "=",
						Value:        "test",
						IsPrimaryKey: true,
					},
					{
						Column:       "age",
						Operator:     "=",
						Value:        "15",
						IsPrimaryKey: true,
					},
				},
				Params:      params2,
				ParamKeys:   []string{"value1", "value2"},
				PrimaryKeys: []string{"name", "age"}, // assuming primary keys
				RowKey:      "test\x00\x01\x8f",
			},
			encodeIntValuesWithBigEndian: false,
			wantErr:                      false,
			defaultKeyspace:              "test_keyspace",
		},
		// todo remove once we support ordered code ints
		{
			name: "DELETE query with multiple WHERE clauses with big endian int key encoding",
			args: args{
				queryStr: "DELETE FROM test_keyspace.user_info WHERE name='test' AND age=15",
			},
			fields: fields{
				SchemaMappingConfig: createSchemaMappingConfigs(),
			},
			want: &DeleteQueryMapping{
				Query:     "DELETE FROM test_keyspace.user_info WHERE name='test' AND age=15",
				QueryType: "DELETE",
				Table:     "user_info",
				Keyspace:  "test_keyspace",
				Clauses: []types.Clause{
					{
						Column:       "name",
						Operator:     "=",
						Value:        "test",
						IsPrimaryKey: true,
					},
					{
						Column:       "age",
						Operator:     "=",
						Value:        "15",
						IsPrimaryKey: true,
					},
				},
				Params:      params2,
				ParamKeys:   []string{"value1", "value2"},
				PrimaryKeys: []string{"name", "age"}, // assuming primary keys
				RowKey:      "test\x00\x01\x00\xff\x00\xff\x00\xff\x00\xff\x00\xff\x00\xff\x00\xff\x0f",
			},
			encodeIntValuesWithBigEndian: true,
			wantErr:                      false,
			defaultKeyspace:              "test_keyspace",
		},
		{
			name: "DELETE query with missing keyspace or table",
			args: args{
				queryStr: "DELETE FROM .user_info WHERE name='test' AND age=15",
			},
			fields: fields{
				SchemaMappingConfig: createSchemaMappingConfigs(),
			},
			want:            nil,
			wantErr:         true,
			defaultKeyspace: "",
		},
		{
			name: "DELETE query with incorrect keyword positions",
			args: args{
				queryStr: "DELETE test_keyspace.user_info WHERE name='test' AND age=15",
			},
			fields: fields{
				SchemaMappingConfig: createSchemaMappingConfigs(),
			},
			want:            nil,
			wantErr:         true,
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "DELETE query with ifExists condition",
			args: args{
				queryStr: "DELETE FROM test_keyspace.user_info WHERE name='test' AND age=15 IF EXISTS",
			},
			fields: fields{
				SchemaMappingConfig: createSchemaMappingConfigs(),
			},
			want: &DeleteQueryMapping{
				Query:     "DELETE FROM test_keyspace.user_info WHERE name='test' AND age=15 IF EXISTS",
				QueryType: "DELETE",
				Table:     "user_info",
				Keyspace:  "test_keyspace",
				Clauses: []types.Clause{
					{
						Column:       "name",
						Operator:     "=",
						Value:        "test",
						IsPrimaryKey: true,
					},
					{
						Column:       "age",
						Operator:     "=",
						Value:        "15",
						IsPrimaryKey: true,
					},
				},
				Params:      params2,
				ParamKeys:   []string{"value1", "value2"},
				PrimaryKeys: []string{"name", "age"}, // assuming primary keys
				RowKey:      "test\x00\x01\x8f",
				IfExists:    true,
			},
			wantErr:         false,
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "non-existent keyspace/table",
			args: args{
				queryStr: "DELETE FROM non_existent_keyspace.non_existent_table WHERE name='test'",
			},
			fields: fields{
				SchemaMappingConfig: createSchemaMappingConfigs(),
			},
			want:            nil,
			wantErr:         true,
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "with keyspace in query, without default keyspace",
			args: args{
				queryStr: "DELETE FROM test_keyspace.test_table WHERE column1 = 'abc' AND column10 = 'pkval';",
			},
			fields: fields{
				SchemaMappingConfig: createSchemaMappingConfigs(),
			},
			want: &DeleteQueryMapping{
				Query:     "DELETE FROM test_keyspace.test_table WHERE column1 = 'abc' AND column10 = 'pkval';",
				QueryType: "DELETE",
				Table:     "test_table",
				Keyspace:  "test_keyspace",
				Clauses: []types.Clause{
					{
						Column:       "column1",
						Operator:     "=",
						Value:        "abc",
						IsPrimaryKey: true,
					},
					{
						Column:       "column10",
						Operator:     "=",
						Value:        "pkval",
						IsPrimaryKey: true,
					},
				},
				Params: map[string]interface{}{
					"value1": []byte("abc"),
					"value2": []byte("pkval"),
				},
				ParamKeys:   []string{"value1", "value2"},
				PrimaryKeys: []string{"column1", "column10"},
				RowKey:      "abc\x00\x01pkval",
			},
			wantErr:         false,
			defaultKeyspace: "",
		},
		{
			name: "with keyspace in query, with default keyspace",
			args: args{
				queryStr: "DELETE FROM test_keyspace.test_table WHERE column1 = 'abc' AND column10 = 'pkval';",
			},
			fields: fields{
				SchemaMappingConfig: createSchemaMappingConfigs(),
			},
			want: &DeleteQueryMapping{
				Query:     "DELETE FROM test_keyspace.test_table WHERE column1 = 'abc' AND column10 = 'pkval';",
				QueryType: "DELETE",
				Table:     "test_table",
				Keyspace:  "test_keyspace",
				Clauses: []types.Clause{
					{
						Column:       "column1",
						Operator:     "=",
						Value:        "abc",
						IsPrimaryKey: true,
					},
					{
						Column:       "column10",
						Operator:     "=",
						Value:        "pkval",
						IsPrimaryKey: true,
					},
				},
				Params: map[string]interface{}{
					"value1": []byte("abc"),
					"value2": []byte("pkval"),
				},
				ParamKeys:   []string{"value1", "value2"},
				PrimaryKeys: []string{"column1", "column10"},
				RowKey:      "abc\x00\x01pkval",
			},
			wantErr:         false,
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "without keyspace in query, with default keyspace",
			args: args{
				queryStr: "DELETE FROM test_table WHERE column1 = 'abc' AND column10 = 'pkval';",
			},
			fields: fields{
				SchemaMappingConfig: createSchemaMappingConfigs(),
			},
			want: &DeleteQueryMapping{
				Query:     "DELETE FROM test_table WHERE column1 = 'abc' AND column10 = 'pkval';",
				QueryType: "DELETE",
				Table:     "test_table",
				Keyspace:  "test_keyspace",
				Clauses: []types.Clause{
					{
						Column:       "column1",
						Operator:     "=",
						Value:        "abc",
						IsPrimaryKey: true,
					},
					{
						Column:       "column10",
						Operator:     "=",
						Value:        "pkval",
						IsPrimaryKey: true,
					},
				},
				Params: map[string]interface{}{
					"value1": []byte("abc"),
					"value2": []byte("pkval"),
				},
				ParamKeys:   []string{"value1", "value2"},
				PrimaryKeys: []string{"column1", "column10"},
				RowKey:      "abc\x00\x01pkval",
			},
			wantErr:         false,
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "without keyspace in query, without default keyspace (should error)",
			args: args{
				queryStr: "DELETE FROM test_table WHERE column1 = 'abc' AND column10 = 'pkval';",
			},
			fields: fields{
				SchemaMappingConfig: createSchemaMappingConfigs(),
			},
			want:            nil,
			wantErr:         true,
			defaultKeyspace: "",
		},
		{
			name: "invalid query syntax (should error)",
			args: args{
				queryStr: "DELETE FROM test_keyspace.test_table",
			},
			fields: fields{
				SchemaMappingConfig: createSchemaMappingConfigs(),
			},
			want:            nil,
			wantErr:         true,
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "parser returns empty table (should error)",
			args: args{
				queryStr: "DELETE FROM test_keyspace. WHERE column1 = 'abc';",
			},
			fields: fields{
				SchemaMappingConfig: createSchemaMappingConfigs(),
			},
			want:            nil,
			wantErr:         true,
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "parser returns empty keyspace (should error)",
			args: args{
				queryStr: "DELETE FROM .test_table WHERE column1 = 'abc';",
			},
			fields: fields{
				SchemaMappingConfig: createSchemaMappingConfigs(),
			},
			want:            nil,
			wantErr:         true,
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "keyspace does not exist (should error)",
			args: args{
				queryStr: "DELETE FROM invalid_keyspace.test_table WHERE column1 = 'abc';",
			},
			fields: fields{
				SchemaMappingConfig: createSchemaMappingConfigs(),
			},
			want:            nil,
			wantErr:         true,
			defaultKeyspace: "test_keyspace",
		},
		{
			name: "table does not exist (should error)",
			args: args{
				queryStr: "DELETE FROM test_keyspace.invalid_table WHERE column1 = 'abc';",
			},
			fields: fields{
				SchemaMappingConfig: createSchemaMappingConfigs(),
			},
			want:            nil,
			wantErr:         true,
			defaultKeyspace: "test_keyspace",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &Translator{
				Logger:                       tt.fields.Logger,
				EncodeIntValuesWithBigEndian: tt.encodeIntValuesWithBigEndian,
				SchemaMappingConfig:          tt.fields.SchemaMappingConfig,
			}
			got, err := tr.TranslateDeleteQuerytoBigtable(tt.args.queryStr, false, tt.defaultKeyspace)
			if (err != nil) != tt.wantErr {
				t.Errorf("Translator.TranslateDeleteQuerytoBigtable() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTranslator_BuildDeletePrepareQuery(t *testing.T) {
	type fields struct {
		Logger              *zap.Logger
		SchemaMappingConfig *schemaMapping.SchemaMappingConfig
	}
	type args struct {
		values                 []*primitive.Value
		st                     *DeleteQueryMapping
		variableColumnMetadata []*message.ColumnMetadata
		protocolV              primitive.ProtocolVersion
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    string
		want1   TimestampInfo
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
					{Contents: []byte("abc")},
				},
				st: &DeleteQueryMapping{
					Query:       "DELETE FROM test_table WHERE column1=? AND column10=?",
					QueryType:   "DELETE",
					Table:       "test_table",
					Keyspace:    "test_keyspace",
					PrimaryKeys: []string{"column1", "column10"},
					RowKey:      "column1",
					VariableMetadata: []*message.ColumnMetadata{
						{Name: "column1"},
						{Name: "column10"},
					},
				},
				variableColumnMetadata: []*message.ColumnMetadata{
					{
						Keyspace: "test_keyspace",
						Table:    "test_table",
						Name:     "column1",
						Index:    0,
						Type:     datatype.Varchar,
					},
					{
						Keyspace: "test_keyspace",
						Table:    "test_table",
						Name:     "column10",
						Index:    1,
						Type:     datatype.Varchar,
					},
				},
			},
			want:    "123\x00\x01abc",
			want1:   TimestampInfo{},
			wantErr: false,
		},
		{
			name: "Invalid input with wrong value map",
			fields: fields{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			args: args{
				values: []*primitive.Value{
					{Contents: []byte("123")},
				},
				st: &DeleteQueryMapping{
					Query:       "DELETE FROM test_table WHERE column1=?",
					QueryType:   "DELETE",
					Table:       "test_table",
					Keyspace:    "test_keyspace",
					PrimaryKeys: []string{"column1", "column2"},
					RowKey:      "column1",
					VariableMetadata: []*message.ColumnMetadata{
						{Name: "column1"},
					},
				},
				variableColumnMetadata: []*message.ColumnMetadata{{
					Keyspace: "test_keyspace",
					Table:    "test_table",
					Name:     "column1",
					Index:    0,
					Type:     datatype.Varchar,
				}},
			},
			want:    "",
			want1:   TimestampInfo{},
			wantErr: true,
		},
		{
			name: "Invalid Input with wrong parameter",
			fields: fields{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			args: args{
				values: []*primitive.Value{},
				st: &DeleteQueryMapping{
					Query:       "DELETE FROM test_table WHERE column1=?",
					QueryType:   "DELETE",
					Table:       "test_table",
					Keyspace:    "test_keyspace",
					PrimaryKeys: []string{"column1", "column1"},
					RowKey:      "column1#column1",
					VariableMetadata: []*message.ColumnMetadata{
						{Name: "column1"},
					},
				},
				variableColumnMetadata: []*message.ColumnMetadata{},
			},
			want:    "",
			want1:   TimestampInfo{},
			wantErr: true,
		},
		{
			name: "Invalid Input with timestamp info",
			fields: fields{
				Logger:              zap.NewNop(),
				SchemaMappingConfig: GetSchemaMappingConfig(),
			},
			args: args{
				values: []*primitive.Value{
					{Contents: []byte("123")},
				},
				st: &DeleteQueryMapping{
					Query:       "DELETE FROM test_table WHERE column1=?",
					QueryType:   "DELETE",
					Table:       "test_table",
					Keyspace:    "test_keyspace",
					PrimaryKeys: []string{"column"},
					RowKey:      "column",
					VariableMetadata: []*message.ColumnMetadata{
						{Name: "column1"},
					},
					TimestampInfo: TimestampInfo{
						HasUsingTimestamp: true,
						Timestamp:         0,
						Index:             0,
					},
				},
				variableColumnMetadata: []*message.ColumnMetadata{{
					Keyspace: "test_keyspace",
					Table:    "test_table",
					Name:     "column1",
					Index:    0,
					Type:     datatype.Varchar,
				}},
			},
			want:    "",
			want1:   TimestampInfo{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &Translator{
				Logger:              tt.fields.Logger,
				SchemaMappingConfig: tt.fields.SchemaMappingConfig,
			}
			got, got1, err := tr.BuildDeletePrepareQuery(tt.args.values, tt.args.st, tt.args.variableColumnMetadata, tt.args.protocolV)
			if tt.wantErr {
				assert.Error(t, err, "error expected")
			} else {
				assert.NoErrorf(t, err, "unexpected error")
			}
			if got != tt.want {
				t.Errorf("Translator.BuildDeletePrepareQuery() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("Translator.BuildDeletePrepareQuery() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

// newTestTableConfig returns a minimal SchemaMappingConfig for testing.
func newTestTableConfig() *schemaMapping.SchemaMappingConfig {
	tc := &schemaMapping.SchemaMappingConfig{
		TablesMetaData: map[string]map[string]map[string]*types.Column{
			"test_keyspace": {
				"testtable": {
					"col1": {CQLType: datatype.Varchar, IsPrimaryKey: false},
					"col2": {CQLType: datatype.Int, IsPrimaryKey: false},
				},
			},
		},
		PkMetadataCache: map[string]map[string][]types.Column{
			"testtable": {
				"test_keyspace": {
					{CQLType: datatype.Varchar, IsPrimaryKey: false, ColumnName: "col1"},
					{CQLType: datatype.Int, IsPrimaryKey: false, ColumnName: "col2"},
				},
			},
		},
	}
	return tc
}

// setupDeleteColumnList is used to generate the DeleteColumnList context.
func setupDeleteColumnList(query string) interface{} {
	lexer := cql.NewCqlLexer(antlr.NewInputStream(query))
	stream := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)
	p := cql.NewCqlParser(stream)

	deleteObj := p.Delete_()
	return deleteObj.DeleteColumnList()
}

func TestParseDeleteColumns(t *testing.T) {
	tc := newTestTableConfig()
	keyspace := "test_keyspace"
	table := "testtable"

	tests := []struct {
		name         string
		query        string
		expectNil    bool
		expectedCols []schemaMapping.SelectedColumns
		wantErr      bool
	}{
		{
			name:         "Nil delete column list",
			query:        "DELETE FROM test_keyspace.testtable",
			expectNil:    true,
			expectedCols: nil,
			wantErr:      false,
		},
		{
			name:      "Single delete column",
			query:     "DELETE col1 FROM test_keyspace.testtable",
			expectNil: false,
			expectedCols: []schemaMapping.SelectedColumns{
				{
					Name: "col1",
				},
			},
			wantErr: false,
		},
		{
			name:      "Multiple delete columns",
			query:     "DELETE col1, col2 FROM test_keyspace.testtable",
			expectNil: false,
			expectedCols: []schemaMapping.SelectedColumns{
				{
					Name: "col1",
				},
				{
					Name: "col2",
				},
			},
			wantErr: false,
		},
		{
			name:      "Delete column with list index",
			query:     "DELETE col1[1] FROM test_keyspace.testtable",
			expectNil: false,
			expectedCols: []schemaMapping.SelectedColumns{
				{
					Name:      "col1",
					ListIndex: "1",
				},
			},
			wantErr: false,
		},
		{
			name:      "Delete column with map key",
			query:     "DELETE col1['key'] FROM test_keyspace.testtable",
			expectNil: false,
			expectedCols: []schemaMapping.SelectedColumns{
				{
					Name:   "col1",
					MapKey: "key",
				},
			},
			wantErr: false,
		},
		{
			name: "Column missing in schema mapping table",
			// using a column name that doesn't exist in schema mapping table.
			query:     "DELETE unknown FROM test_keyspace.testtable",
			expectNil: false,
			// parseDeleteColumns will call tableConf.GetColumnType and error out.
			expectedCols: nil,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := setupDeleteColumnList(tt.query)
			var deleteColumnList cql.IDeleteColumnListContext
			if raw != nil {
				var ok bool
				deleteColumnList, ok = raw.(cql.IDeleteColumnListContext)
				if !ok {
					t.Fatalf("returned object does not implement IDeleteColumnListContext")
				}
			}
			cols, err := parseDeleteColumns(deleteColumnList, table, tc, keyspace)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseDeleteColumns() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.expectNil {
				if cols != nil {
					t.Fatalf("expected nil columns, got %v", cols)
				}
				return
			}
			if len(cols) != len(tt.expectedCols) {
				t.Fatalf("expected %d columns, got %d", len(tt.expectedCols), len(cols))
			}
			for i := range cols {
				got := cols[i]
				exp := tt.expectedCols[i]
				if got.Name != exp.Name {
					t.Errorf("expected column name '%s', got '%s'", exp.Name, got.Name)
				}
				// Normalize values: remove any quotes from map keys.
				gotMapKey := strings.Trim(got.MapKey, "'")
				expMapKey := strings.Trim(exp.MapKey, "'")
				if gotMapKey != expMapKey {
					t.Errorf("expected map key '%s', got '%s'", expMapKey, gotMapKey)
				}
				if got.ListIndex != exp.ListIndex {
					t.Errorf("expected list index '%s', got '%s'", exp.ListIndex, got.ListIndex)
				}
			}
		})
	}
}

func TestFindFirstMissingKey(t *testing.T) {
	tests := []struct {
		name             string
		primaryKeys      []string
		primaryKeysFound []string
		want             string
	}{
		{
			name:             "Empty slices",
			primaryKeys:      []string{},
			primaryKeysFound: []string{},
			want:             "",
		},
		{
			name:             "All keys present",
			primaryKeys:      []string{"id", "name", "age"},
			primaryKeysFound: []string{"id", "name", "age"},
			want:             "",
		},
		{
			name:             "One key missing",
			primaryKeys:      []string{"id", "name", "age"},
			primaryKeysFound: []string{"id", "age"},
			want:             "name",
		},
		{
			name:             "Multiple keys missing, return first",
			primaryKeys:      []string{"id", "name", "age", "email"},
			primaryKeysFound: []string{"id", "email"},
			want:             "name",
		},
		{
			name:             "Different order but all present",
			primaryKeys:      []string{"id", "name", "age"},
			primaryKeysFound: []string{"age", "id", "name"},
			want:             "",
		},
		{
			name:             "Extra found keys",
			primaryKeys:      []string{"id", "name"},
			primaryKeysFound: []string{"id", "name", "extra"},
			want:             "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findFirstMissingKey(tt.primaryKeys, tt.primaryKeysFound)
			if got != tt.want {
				t.Errorf("findFirstMissingKey() = %v, want %v", got, tt.want)
			}
		})
	}
}
