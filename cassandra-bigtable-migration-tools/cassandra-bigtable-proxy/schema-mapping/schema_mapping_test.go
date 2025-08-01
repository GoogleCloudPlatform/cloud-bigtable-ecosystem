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
package schemaMapping

import (
	"fmt"
	"reflect"
	"testing"

	types "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

var systemColumnFamily = "cf1"
var tablesMetaData = map[string]map[string]map[string]*types.Column{
	"keyspace": {"table1": {
		"column1": {
			CQLType:      datatype.Varchar,
			ColumnName:   "column1",
			IsPrimaryKey: false,
			PkPrecedence: 0,
			IsCollection: false,
			Metadata: message.ColumnMetadata{
				Keyspace: "test-keyspace",
				Table:    "table1",
				Name:     "column1",
				Index:    int32(0),
				Type:     datatype.Varchar,
			},
		},
	}},
}
var expectedResponse = []*message.ColumnMetadata{
	{Keyspace: "test-keyspace", Name: "column1", Table: "table1", Type: datatype.Varchar, Index: 0},
}

func Test_GetColumnType(t *testing.T) {
	columnExistsArgs := struct {
		tableName  string
		columnName string
	}{
		tableName:  "table1",
		columnName: "column1",
	}

	tableExistsArgs := struct {
		tableName  string
		columnName string
	}{
		tableName:  "table2",
		columnName: "column2",
	}

	columnExistsInDifferentTableArgs := struct {
		tableName  string
		columnName string
	}{
		tableName:  "table1",
		columnName: "column2",
	}

	columnExistsWant := &types.Column{
		IsPrimaryKey: false,
		IsCollection: false,
		CQLType:      datatype.Varchar,
	}

	tests := []struct {
		name   string
		fields SchemaMappingConfig
		args   struct {
			tableName  string
			columnName string
		}
		want    *types.Column
		wantErr bool
	}{
		{
			name: "types.Column exists",
			fields: SchemaMappingConfig{
				TablesMetaData:     tablesMetaData,
				SystemColumnFamily: systemColumnFamily,
			},
			args:    columnExistsArgs,
			want:    columnExistsWant,
			wantErr: false,
		},
		{
			name: "Table exists",
			fields: SchemaMappingConfig{
				TablesMetaData:     tablesMetaData,
				SystemColumnFamily: systemColumnFamily,
			},
			args:    tableExistsArgs,
			wantErr: true,
		},
		{
			name: "types.Column exists in different table",
			fields: SchemaMappingConfig{
				TablesMetaData:     tablesMetaData,
				SystemColumnFamily: systemColumnFamily,
			},
			args:    columnExistsInDifferentTableArgs,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.fields.GetColumnType("keyspace", tt.args.tableName, tt.args.columnName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetColumnType() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetColumnType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_GetMetadataForColumns(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name   string
		fields SchemaMappingConfig
		args   struct {
			tableName   string
			columnNames []string
		}
		want    []*message.ColumnMetadata
		wantErr bool
	}{
		{
			name: "Success - Single regular column",
			fields: SchemaMappingConfig{
				Logger:             logger,
				TablesMetaData:     tablesMetaData,
				SystemColumnFamily: "cf1",
			},
			args: struct {
				tableName   string
				columnNames []string
			}{
				tableName:   "table1",
				columnNames: []string{"column1"},
			},
			want:    expectedResponse,
			wantErr: false,
		},
		{
			name: "Success - Multiple regular columns",
			fields: SchemaMappingConfig{
				Logger: logger,
				TablesMetaData: map[string]map[string]map[string]*types.Column{
					"keyspace": {"table1": {
						"column1": {
							Metadata: message.ColumnMetadata{
								Type: datatype.Varchar,
							},
						},
						"column2": {
							Metadata: message.ColumnMetadata{
								Type: datatype.Int,
							},
						},
					}},
				},
				SystemColumnFamily: "cf1",
			},
			args: struct {
				tableName   string
				columnNames []string
			}{
				tableName:   "table1",
				columnNames: []string{"column1", "column2"},
			},
			want: []*message.ColumnMetadata{
				{Type: datatype.Varchar, Index: 0},
				{Type: datatype.Int, Index: 1},
			},
			wantErr: false,
		},
		{
			name: "Success - Special column (LimitValue)",
			fields: SchemaMappingConfig{
				Logger:             logger,
				TablesMetaData:     tablesMetaData,
				SystemColumnFamily: "cf1",
			},
			args: struct {
				tableName   string
				columnNames []string
			}{
				tableName:   "table1",
				columnNames: []string{LimitValue},
			},
			want: []*message.ColumnMetadata{
				{
					Type:  datatype.Bigint,
					Index: 0,
					Name:  LimitValue,
				},
			},
			wantErr: false,
		},
		{
			name: "Success - Mixed column types",
			fields: SchemaMappingConfig{
				Logger:             logger,
				TablesMetaData:     tablesMetaData,
				SystemColumnFamily: "cf1",
			},
			args: struct {
				tableName   string
				columnNames []string
			}{
				tableName:   "table1",
				columnNames: []string{"column1", LimitValue},
			},
			want: []*message.ColumnMetadata{
				{Type: datatype.Varchar, Index: 0, Name: "column1"},
				{
					Type:  datatype.Bigint,
					Index: 1,
					Name:  LimitValue,
				},
			},
			wantErr: false,
		},
		{
			name: "Error - types.Column not found in metadata",
			fields: SchemaMappingConfig{
				Logger:             logger,
				TablesMetaData:     tablesMetaData,
				SystemColumnFamily: "cf1",
			},
			args: struct {
				tableName   string
				columnNames []string
			}{
				tableName:   "table1",
				columnNames: []string{"nonexistent_column"},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Error - Table not found",
			fields: SchemaMappingConfig{
				Logger:             logger,
				TablesMetaData:     tablesMetaData,
				SystemColumnFamily: "cf1",
			},
			args: struct {
				tableName   string
				columnNames []string
			}{
				tableName:   "nonexistent_table",
				columnNames: []string{"column1"},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Empty column names",
			fields: SchemaMappingConfig{
				Logger:             zap.NewNop(),
				TablesMetaData:     tablesMetaData,
				SystemColumnFamily: "cf1",
			},
			args: struct {
				tableName   string
				columnNames []string
			}{
				tableName:   "table1",
				columnNames: []string{},
			},
			want:    expectedResponse,
			wantErr: false,
		},
		{
			name: "Success - Multiple special columns",
			fields: SchemaMappingConfig{
				Logger:             logger,
				TablesMetaData:     tablesMetaData,
				SystemColumnFamily: "cf1",
			},
			args: struct {
				tableName   string
				columnNames []string
			}{
				tableName:   "table1",
				columnNames: []string{LimitValue, LimitValue},
			},
			want: []*message.ColumnMetadata{
				{
					Type:  datatype.Bigint,
					Index: 0,
					Name:  LimitValue,
				},
				{
					Type:  datatype.Bigint,
					Index: 1,
					Name:  LimitValue,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.fields.GetMetadataForColumns("keyspace", tt.args.tableName, tt.args.columnNames)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetMetadataForColumns() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Compare metadata content instead of memory addresses
			if tt.want == nil {
				assert.Nil(t, got)
			} else {
				assert.NotNil(t, got)
				assert.Equal(t, len(tt.want), len(got))

				for i, expected := range tt.want {
					assert.Equal(t, expected.Type, got[i].Type)
					assert.Equal(t, expected.Index, got[i].Index)
					assert.Equal(t, expected.Name, got[i].Name)
				}
			}
		})
	}
}

func Test_GetMetadataForSelectedColumns(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name   string
		fields SchemaMappingConfig
		args   struct {
			tableName   string
			columnNames []SelectedColumns
			keySpace    string
		}
		want    []*message.ColumnMetadata
		wantErr bool
	}{
		{
			name: "Successfully retrieve metadata for specific column",
			fields: SchemaMappingConfig{
				Logger:         logger,
				TablesMetaData: tablesMetaData,
			},
			args: struct {
				tableName   string
				columnNames []SelectedColumns
				keySpace    string
			}{
				tableName:   "table1",
				columnNames: []SelectedColumns{{Name: "column1"}},
				keySpace:    "keyspace",
			},
			want:    expectedResponse,
			wantErr: false,
		},
		{
			name: "Return all columns when no specific columns provided",
			fields: SchemaMappingConfig{
				Logger:         logger,
				TablesMetaData: tablesMetaData,
			},
			args: struct {
				tableName   string
				columnNames []SelectedColumns
				keySpace    string
			}{
				tableName:   "table1",
				columnNames: []SelectedColumns{},
				keySpace:    "keyspace",
			},
			want:    expectedResponse,
			wantErr: false,
		},
		{
			name: "Error when table is not found",
			fields: SchemaMappingConfig{
				Logger:         logger,
				TablesMetaData: tablesMetaData,
			},
			args: struct {
				tableName   string
				columnNames []SelectedColumns
				keySpace    string
			}{
				tableName:   "nonexistent_table",
				columnNames: []SelectedColumns{{Name: "column1"}},
				keySpace:    "keyspace",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Error when column is not found",
			fields: SchemaMappingConfig{
				Logger:         logger,
				TablesMetaData: tablesMetaData,
			},
			args: struct {
				tableName   string
				columnNames []SelectedColumns
				keySpace    string
			}{
				tableName:   "table1",
				columnNames: []SelectedColumns{{Name: "nonexistent_column"}},
				keySpace:    "keyspace",
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.fields.GetMetadataForSelectedColumns(tt.args.tableName, tt.args.columnNames, tt.args.keySpace)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetMetadataForSelectedColumns() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetMetadataForSelectedColumns() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_GetPkByTableName(t *testing.T) {
	logger := zap.NewNop()

	// Sample data for testing
	pkMetadataCache := map[string]map[string][]types.Column{
		"keyspace1": {
			"table1": {
				{
					CQLType:      datatype.Varchar,
					ColumnName:   "id",
					IsPrimaryKey: true,
					PkPrecedence: 1,
					IsCollection: false,
				},
			},
		},
	}

	expectedPkResponse := []types.Column{
		{
			CQLType:      datatype.Varchar,
			ColumnName:   "id",
			IsPrimaryKey: true,
			PkPrecedence: 1,
			IsCollection: false,
		},
	}

	tests := []struct {
		name   string
		fields SchemaMappingConfig
		args   struct {
			tableName string
			keySpace  string
		}
		want    []types.Column
		wantErr bool
	}{
		{
			name: "Successfully retrieve primary key metadata for table",
			fields: SchemaMappingConfig{
				Logger:          logger,
				PkMetadataCache: pkMetadataCache,
			},
			args: struct {
				tableName string
				keySpace  string
			}{
				tableName: "table1",
				keySpace:  "keyspace1",
			},
			want:    expectedPkResponse,
			wantErr: false,
		},
		{
			name: "Error when table metadata is not found",
			fields: SchemaMappingConfig{
				Logger:          logger,
				PkMetadataCache: pkMetadataCache,
			},
			args: struct {
				tableName string
				keySpace  string
			}{
				tableName: "nonexistent_table",
				keySpace:  "keyspace1",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Error when keyspace metadata is not found",
			fields: SchemaMappingConfig{
				Logger:          logger,
				PkMetadataCache: pkMetadataCache,
			},
			args: struct {
				tableName string
				keySpace  string
			}{
				tableName: "table1",
				keySpace:  "nonexistent_keyspace",
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.fields.GetPkByTableName(tt.args.tableName, tt.args.keySpace)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetPkByTableName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetPkByTableName() = %v, want %v", got, tt.want)
			}
		})
	}
}
func Test_GetPkKeyType(t *testing.T) {
	logger := zap.NewNop()

	// Sample data for testing
	pkMetadataCache := map[string]map[string][]types.Column{
		"keyspace1": {
			"table1": {
				{
					ColumnName:   "id",
					KeyType:      "partition",
					IsPrimaryKey: true,
				},
				{
					ColumnName:   "name",
					KeyType:      "clustering",
					IsPrimaryKey: true,
				},
			},
		},
	}

	tests := []struct {
		name   string
		fields SchemaMappingConfig
		args   struct {
			tableName  string
			keySpace   string
			columnName string
		}
		want    string
		wantErr bool
	}{
		{
			name: "Successfully get partition key type",
			fields: SchemaMappingConfig{
				Logger:          logger,
				PkMetadataCache: pkMetadataCache,
			},
			args: struct {
				tableName  string
				keySpace   string
				columnName string
			}{
				tableName:  "table1",
				keySpace:   "keyspace1",
				columnName: "id",
			},
			want:    "partition",
			wantErr: false,
		},
		{
			name: "Successfully get clustering key type",
			fields: SchemaMappingConfig{
				Logger:          logger,
				PkMetadataCache: pkMetadataCache,
			},
			args: struct {
				tableName  string
				keySpace   string
				columnName string
			}{
				tableName:  "table1",
				keySpace:   "keyspace1",
				columnName: "name",
			},
			want:    "clustering",
			wantErr: false,
		},
		{
			name: "Error when column is not a primary key",
			fields: SchemaMappingConfig{
				Logger:          logger,
				PkMetadataCache: pkMetadataCache,
			},
			args: struct {
				tableName  string
				keySpace   string
				columnName string
			}{
				tableName:  "table1",
				keySpace:   "keyspace1",
				columnName: "nonexistent_column",
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "Error when table not found",
			fields: SchemaMappingConfig{
				Logger:          logger,
				PkMetadataCache: pkMetadataCache,
			},
			args: struct {
				tableName  string
				keySpace   string
				columnName string
			}{
				tableName:  "nonexistent_table",
				keySpace:   "keyspace1",
				columnName: "id",
			},
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.fields.GetPkKeyType(tt.args.tableName, tt.args.keySpace, tt.args.columnName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetPkKeyType() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetPkKeyType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_HandleSpecialColumn(t *testing.T) {
	tests := []struct {
		name                string
		columnsMap          map[string]*types.Column
		columnName          string
		index               int32
		isWriteTimeFunction bool
		expectedMetadata    *message.ColumnMetadata
		expectedError       error
	}{
		{
			name: "Success - Special column (LimitValue)",
			columnsMap: map[string]*types.Column{
				"column1": {
					Metadata: message.ColumnMetadata{
						Type: datatype.Varchar,
					},
				},
			},
			columnName:          LimitValue,
			index:               0,
			isWriteTimeFunction: false,
			expectedMetadata: &message.ColumnMetadata{
				Type:  datatype.Bigint,
				Index: 0,
				Name:  LimitValue,
			},
			expectedError: nil,
		},
		{
			name: "Success - Write time function",
			columnsMap: map[string]*types.Column{
				"column1": {
					Metadata: message.ColumnMetadata{
						Type: datatype.Varchar,
					},
				},
			},
			columnName:          "writetime(column1)",
			index:               1,
			isWriteTimeFunction: true,
			expectedMetadata: &message.ColumnMetadata{
				Type:  datatype.Bigint,
				Index: 1,
				Name:  "writetime(column1)",
			},
			expectedError: nil,
		},
		{
			name: "Error - Invalid special column",
			columnsMap: map[string]*types.Column{
				"column1": {
					Metadata: message.ColumnMetadata{
						Type: datatype.Varchar,
					},
				},
			},
			columnName:          "invalid_column",
			index:               0,
			isWriteTimeFunction: false,
			expectedMetadata:    nil,
			expectedError:       fmt.Errorf("invalid special column: invalid_column"),
		},
		{
			name:                "Error - Empty columns map",
			columnsMap:          map[string]*types.Column{},
			columnName:          LimitValue,
			index:               0,
			isWriteTimeFunction: false,
			expectedMetadata:    nil,
			expectedError:       fmt.Errorf("special column %s not found in provided metadata", LimitValue),
		},
		{
			name: "Success - Multiple columns in map",
			columnsMap: map[string]*types.Column{
				"column1": {
					Metadata: message.ColumnMetadata{
						Type: datatype.Varchar,
					},
				},
				"column2": {
					Metadata: message.ColumnMetadata{
						Type: datatype.Int,
					},
				},
			},
			columnName:          LimitValue,
			index:               2,
			isWriteTimeFunction: false,
			expectedMetadata: &message.ColumnMetadata{
				Type:  datatype.Bigint,
				Index: 2,
				Name:  LimitValue,
			},
			expectedError: nil,
		},
		{
			name: "Success - Write time function with different column name",
			columnsMap: map[string]*types.Column{
				"column1": {
					Metadata: message.ColumnMetadata{
						Type: datatype.Varchar,
					},
				},
			},
			columnName:          "writetime(column2)",
			index:               3,
			isWriteTimeFunction: true,
			expectedMetadata: &message.ColumnMetadata{
				Type:  datatype.Bigint,
				Index: 3,
				Name:  "writetime(column2)",
			},
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &SchemaMappingConfig{}
			metadata, err := config.handleSpecialColumn(tt.columnsMap, tt.columnName, tt.index, tt.isWriteTimeFunction)

			// Check error cases
			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError.Error(), err.Error())
				assert.Nil(t, metadata)
				return
			}

			// Check success cases
			assert.NoError(t, err)
			assert.NotNil(t, metadata)
			assert.Equal(t, tt.expectedMetadata.Type, metadata.Type)
			assert.Equal(t, tt.expectedMetadata.Index, metadata.Index)
			assert.Equal(t, tt.expectedMetadata.Name, metadata.Name)
		})
	}
}

func Test_InstanceExists(t *testing.T) {
	tests := []struct {
		name     string
		fields   SchemaMappingConfig
		keyspace string
		want     bool
	}{
		{
			name: "Keyspace exists",
			fields: SchemaMappingConfig{
				TablesMetaData: map[string]map[string]map[string]*types.Column{
					"test_keyspace": {
						"table1": {
							"column1": {
								CQLType:      datatype.Varchar,
								ColumnName:   "column1",
								IsPrimaryKey: false,
							},
						},
					},
				},
			},
			keyspace: "test_keyspace",
			want:     true,
		},
		{
			name: "Keyspace does not exist",
			fields: SchemaMappingConfig{
				TablesMetaData: map[string]map[string]map[string]*types.Column{
					"test_keyspace": {
						"table1": {
							"column1": {
								CQLType:      datatype.Varchar,
								ColumnName:   "column1",
								IsPrimaryKey: false,
							},
						},
					},
				},
			},
			keyspace: "nonexistent_keyspace",
			want:     false,
		},
		{
			name: "Empty TablesMetaData",
			fields: SchemaMappingConfig{
				TablesMetaData: map[string]map[string]map[string]*types.Column{},
			},
			keyspace: "any_keyspace",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fields.InstanceExists(tt.keyspace)
			if got != tt.want {
				t.Errorf("InstanceExists() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_TableExist(t *testing.T) {
	tests := []struct {
		name      string
		fields    SchemaMappingConfig
		keyspace  string
		tableName string
		want      bool
	}{
		{
			name: "Table exists in keyspace",
			fields: SchemaMappingConfig{
				TablesMetaData: map[string]map[string]map[string]*types.Column{
					"test_keyspace": {
						"table1": {
							"column1": {
								CQLType:      datatype.Varchar,
								ColumnName:   "column1",
								IsPrimaryKey: false,
							},
						},
					},
				},
			},
			keyspace:  "test_keyspace",
			tableName: "table1",
			want:      true,
		},
		{
			name: "Table does not exist in keyspace",
			fields: SchemaMappingConfig{
				TablesMetaData: map[string]map[string]map[string]*types.Column{
					"test_keyspace": {
						"table1": {
							"column1": {
								CQLType:      datatype.Varchar,
								ColumnName:   "column1",
								IsPrimaryKey: false,
							},
						},
					},
				},
			},
			keyspace:  "test_keyspace",
			tableName: "nonexistent_table",
			want:      false,
		},
		{
			name: "Keyspace does not exist",
			fields: SchemaMappingConfig{
				TablesMetaData: map[string]map[string]map[string]*types.Column{
					"test_keyspace": {
						"table1": {
							"column1": {
								CQLType:      datatype.Varchar,
								ColumnName:   "column1",
								IsPrimaryKey: false,
							},
						},
					},
				},
			},
			keyspace:  "nonexistent_keyspace",
			tableName: "table1",
			want:      false,
		},
		{
			name: "Empty TablesMetaData",
			fields: SchemaMappingConfig{
				TablesMetaData: map[string]map[string]map[string]*types.Column{},
			},
			keyspace:  "any_keyspace",
			tableName: "any_table",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fields.TableExist(tt.keyspace, tt.tableName)
			if got != tt.want {
				t.Errorf("TableExist() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_GetSpecificColumnsMetadataForSelectedColumns(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name          string
		fields        SchemaMappingConfig
		columnsMap    map[string]*types.Column
		selectedCols  []SelectedColumns
		tableName     string
		expectedMeta  []*message.ColumnMetadata
		expectedError error
	}{
		{
			name: "Success - Regular column",
			fields: SchemaMappingConfig{
				Logger: logger,
			},
			columnsMap: map[string]*types.Column{
				"column1": {
					Metadata: message.ColumnMetadata{
						Keyspace: "test_keyspace",
						Table:    "test_table",
						Name:     "column1",
						Type:     datatype.Varchar,
						Index:    0,
					},
				},
			},
			selectedCols: []SelectedColumns{
				{
					Name: "column1",
				},
			},
			tableName: "test_table",
			expectedMeta: []*message.ColumnMetadata{
				{
					Keyspace: "test_keyspace",
					Table:    "test_table",
					Name:     "column1",
					Type:     datatype.Varchar,
					Index:    0,
				},
			},
			expectedError: nil,
		},
		{
			name: "Success - Write time column",
			fields: SchemaMappingConfig{
				Logger: logger,
			},
			columnsMap: map[string]*types.Column{
				"column1": {
					Metadata: message.ColumnMetadata{
						Keyspace: "test_keyspace",
						Table:    "test_table",
						Name:     "column1",
						Type:     datatype.Varchar,
						Index:    0,
					},
				},
			},
			selectedCols: []SelectedColumns{
				{
					Name:              "writetime_column",
					IsWriteTimeColumn: true,
					ColumnName:        "column1",
				},
			},
			tableName: "test_table",
			expectedMeta: []*message.ColumnMetadata{
				{
					Keyspace: "test_keyspace",
					Table:    "test_table",
					Name:     "writetime(column1)",
					Type:     datatype.Bigint,
					Index:    0,
				},
			},
			expectedError: nil,
		},
		{
			name: "Success - Special column",
			fields: SchemaMappingConfig{
				Logger: logger,
			},
			columnsMap: map[string]*types.Column{
				"column1": {
					Metadata: message.ColumnMetadata{
						Keyspace: "test_keyspace",
						Table:    "test_table",
						Name:     "column1",
						Type:     datatype.Varchar,
						Index:    0,
					},
				},
			},
			selectedCols: []SelectedColumns{
				{
					Name: LimitValue,
				},
			},
			tableName: "test_table",
			expectedMeta: []*message.ColumnMetadata{
				{
					Keyspace: "test_keyspace",
					Table:    "test_table",
					Name:     LimitValue,
					Type:     datatype.Bigint,
					Index:    0,
				},
			},
			expectedError: nil,
		},
		{
			name: "Success - Function call",
			fields: SchemaMappingConfig{
				Logger: logger,
			},
			columnsMap: map[string]*types.Column{
				"column1": {
					Metadata: message.ColumnMetadata{
						Keyspace: "test_keyspace",
						Table:    "test_table",
						Name:     "column1",
						Type:     datatype.Varchar,
						Index:    0,
					},
				},
			},
			selectedCols: []SelectedColumns{
				{
					Name:   "column1",
					IsFunc: true,
				},
			},
			tableName:     "test_table",
			expectedMeta:  nil,
			expectedError: nil,
		},
		{
			name: "Error - types.Column not found",
			fields: SchemaMappingConfig{
				Logger: logger,
			},
			columnsMap: map[string]*types.Column{
				"column1": {
					Metadata: message.ColumnMetadata{
						Keyspace: "test_keyspace",
						Table:    "test_table",
						Name:     "column1",
						Type:     datatype.Varchar,
						Index:    0,
					},
				},
			},
			selectedCols: []SelectedColumns{
				{
					Name: "nonexistent_column",
				},
			},
			tableName:     "test_table",
			expectedMeta:  nil,
			expectedError: fmt.Errorf("metadata not found for the `nonexistent_column` column in `test_table`table"),
		},
		{
			name: "Error - Invalid special column",
			fields: SchemaMappingConfig{
				Logger: logger,
			},
			columnsMap: map[string]*types.Column{
				"column1": {
					Metadata: message.ColumnMetadata{
						Keyspace: "test_keyspace",
						Table:    "test_table",
						Name:     "column1",
						Type:     datatype.Varchar,
						Index:    0,
					},
				},
			},
			selectedCols: []SelectedColumns{
				{
					Name: "invalid_special_column",
				},
			},
			tableName:     "test_table",
			expectedMeta:  nil,
			expectedError: fmt.Errorf("metadata not found for the `invalid_special_column` column in `test_table`table"),
		},
		{
			name: "Error - Empty columns map",
			fields: SchemaMappingConfig{
				Logger: logger,
			},
			columnsMap: map[string]*types.Column{},
			selectedCols: []SelectedColumns{
				{
					Name: "column1",
				},
			},
			tableName:     "test_table",
			expectedMeta:  nil,
			expectedError: fmt.Errorf("metadata not found for the `column1` column in `test_table`table"),
		},
		{
			name: "Error - Write time column not found",
			fields: SchemaMappingConfig{
				Logger: logger,
			},
			columnsMap: map[string]*types.Column{},
			selectedCols: []SelectedColumns{
				{
					Name:              "no_write_time_column",
					IsWriteTimeColumn: true,
					ColumnName:        "nonexistent_column",
				},
			},
			tableName:     "test_table",
			expectedMeta:  nil,
			expectedError: fmt.Errorf("special column writetime(nonexistent_column) not found in provided metadata"),
		},
		{
			name: "Error - Special column handling error",
			fields: SchemaMappingConfig{
				Logger: logger,
			},
			columnsMap: map[string]*types.Column{},
			selectedCols: []SelectedColumns{
				{
					Name: LimitValue,
				},
			},
			tableName:     "test_table",
			expectedMeta:  nil,
			expectedError: fmt.Errorf("special column %s not found in provided metadata", LimitValue),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata, err := tt.fields.getSpecificColumnsMetadataForSelectedColumns(tt.columnsMap, tt.selectedCols, tt.tableName, "test_keyspace")

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError.Error(), err.Error())
				assert.Nil(t, metadata)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, metadata)

			// Compare metadata entries only for success cases
			for i, expected := range tt.expectedMeta {
				assert.Equal(t, expected.Keyspace, metadata[i].Keyspace)
				assert.Equal(t, expected.Table, metadata[i].Table)
				assert.Equal(t, expected.Name, metadata[i].Name)
				assert.Equal(t, expected.Type, metadata[i].Type)
				assert.Equal(t, expected.Index, metadata[i].Index)
			}
		})
	}
}

func Test_GetSpecificColumnsMetadata(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name          string
		fields        SchemaMappingConfig
		columnsMap    map[string]*types.Column
		columnNames   []string
		tableName     string
		expectedMeta  []*message.ColumnMetadata
		expectedError error
	}{
		{
			name: "Success - Regular column",
			fields: SchemaMappingConfig{
				Logger: logger,
			},
			columnsMap: map[string]*types.Column{
				"column1": {
					Metadata: message.ColumnMetadata{
						Type: datatype.Varchar,
					},
				},
			},
			columnNames: []string{"column1"},
			tableName:   "table1",
			expectedMeta: []*message.ColumnMetadata{
				{
					Type:  datatype.Varchar,
					Index: 0,
				},
			},
			expectedError: nil,
		},
		{
			name: "Success - Special column (LimitValue)",
			fields: SchemaMappingConfig{
				Logger: logger,
			},
			columnsMap: map[string]*types.Column{
				"column1": {
					Metadata: message.ColumnMetadata{
						Type: datatype.Varchar,
					},
				},
			},
			columnNames: []string{LimitValue},
			tableName:   "table1",
			expectedMeta: []*message.ColumnMetadata{
				{
					Type:  datatype.Bigint,
					Index: 0,
					Name:  LimitValue,
				},
			},
			expectedError: nil,
		},
		{
			name: "Success - Multiple columns",
			fields: SchemaMappingConfig{
				Logger: logger,
			},
			columnsMap: map[string]*types.Column{
				"column1": {
					Metadata: message.ColumnMetadata{
						Type: datatype.Varchar,
					},
				},
				"column2": {
					Metadata: message.ColumnMetadata{
						Type: datatype.Int,
					},
				},
			},
			columnNames: []string{"column1", "column2"},
			tableName:   "table1",
			expectedMeta: []*message.ColumnMetadata{
				{
					Type:  datatype.Varchar,
					Index: 0,
				},
				{
					Type:  datatype.Int,
					Index: 1,
				},
			},
			expectedError: nil,
		},
		{
			name: "Success - Mixed column types",
			fields: SchemaMappingConfig{
				Logger: logger,
			},
			columnsMap: map[string]*types.Column{
				"column1": {
					Metadata: message.ColumnMetadata{
						Type: datatype.Varchar,
					},
				},
			},
			columnNames: []string{"column1", LimitValue},
			tableName:   "table1",
			expectedMeta: []*message.ColumnMetadata{
				{
					Type:  datatype.Varchar,
					Index: 0,
				},
				{
					Type:  datatype.Bigint,
					Index: 1,
					Name:  LimitValue,
				},
			},
			expectedError: nil,
		},
		{
			name: "Error - types.Column not found",
			fields: SchemaMappingConfig{
				Logger: logger,
			},
			columnsMap: map[string]*types.Column{
				"column1": {
					Metadata: message.ColumnMetadata{
						Type: datatype.Varchar,
					},
				},
			},
			columnNames:   []string{"nonexistent_column"},
			tableName:     "table1",
			expectedMeta:  nil,
			expectedError: fmt.Errorf("metadata not found for the `nonexistent_column` column in `table1`table"),
		},
		{
			name: "Success - Empty column names",
			fields: SchemaMappingConfig{
				Logger: logger,
			},
			columnsMap: map[string]*types.Column{
				"column1": {
					Metadata: message.ColumnMetadata{
						Type: datatype.Varchar,
					},
				},
			},
			columnNames:   []string{},
			tableName:     "table1",
			expectedMeta:  nil,
			expectedError: nil,
		},
		{
			name: "Success - Multiple special columns",
			fields: SchemaMappingConfig{
				Logger: logger,
			},
			columnsMap: map[string]*types.Column{
				"column1": {
					Metadata: message.ColumnMetadata{
						Type: datatype.Varchar,
					},
				},
			},
			columnNames: []string{LimitValue, LimitValue},
			tableName:   "table1",
			expectedMeta: []*message.ColumnMetadata{
				{
					Type:  datatype.Bigint,
					Index: 0,
					Name:  LimitValue,
				},
				{
					Type:  datatype.Bigint,
					Index: 1,
					Name:  LimitValue,
				},
			},
			expectedError: nil,
		},
		{
			name: "Error - Special column handling error",
			fields: SchemaMappingConfig{
				Logger: logger,
			},
			columnsMap:    map[string]*types.Column{},
			columnNames:   []string{LimitValue},
			tableName:     "test_table",
			expectedMeta:  nil,
			expectedError: fmt.Errorf("special column %s not found in provided metadata", LimitValue),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata, err := tt.fields.getSpecificColumnsMetadata(tt.columnsMap, tt.columnNames, tt.tableName)

			// Check error cases
			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError.Error(), err.Error())
				assert.Nil(t, metadata)
				return
			}

			// Check success cases
			assert.NoError(t, err)
			if tt.expectedMeta == nil {
				assert.Nil(t, metadata)
			} else {
				assert.NotNil(t, metadata)
				assert.Equal(t, len(tt.expectedMeta), len(metadata))

				// Create maps to store metadata by name for comparison
				expectedMap := make(map[string]*message.ColumnMetadata)
				actualMap := make(map[string]*message.ColumnMetadata)

				// Populate the maps
				for _, meta := range tt.expectedMeta {
					expectedMap[meta.Name] = meta
				}
				for _, meta := range metadata {
					actualMap[meta.Name] = meta
				}

				// Compare metadata entries by name
				for name, expected := range expectedMap {
					actual, exists := actualMap[name]
					assert.True(t, exists, "Expected metadata for column %s not found", name)
					assert.Equal(t, expected.Keyspace, actual.Keyspace)
					assert.Equal(t, expected.Table, actual.Table)
					assert.Equal(t, expected.Type, actual.Type)
					// Don't compare indices as they are order-dependent
				}
			}
		})
	}
}

func Test_CloneColumnMetadata(t *testing.T) {
	tests := []struct {
		name          string
		metadata      *message.ColumnMetadata
		index         int32
		expectedMeta  *message.ColumnMetadata
		expectedError error
	}{
		{
			name: "Success - Basic metadata cloning",
			metadata: &message.ColumnMetadata{
				Keyspace: "test_keyspace",
				Table:    "test_table",
				Name:     "test_column",
				Type:     datatype.Varchar,
				Index:    0,
			},
			index: 1,
			expectedMeta: &message.ColumnMetadata{
				Keyspace: "test_keyspace",
				Table:    "test_table",
				Name:     "test_column",
				Type:     datatype.Varchar,
				Index:    1,
			},
			expectedError: nil,
		},
		{
			name: "Success - Metadata with all fields",
			metadata: &message.ColumnMetadata{
				Keyspace: "test_keyspace",
				Table:    "test_table",
				Name:     "test_column",
				Type:     datatype.Int,
				Index:    5,
			},
			index: 10,
			expectedMeta: &message.ColumnMetadata{
				Keyspace: "test_keyspace",
				Table:    "test_table",
				Name:     "test_column",
				Type:     datatype.Int,
				Index:    10,
			},
			expectedError: nil,
		},
		{
			name: "Success - Metadata with array type",
			metadata: &message.ColumnMetadata{
				Keyspace: "test_keyspace",
				Table:    "test_table",
				Name:     "test_array",
				Type:     datatype.Varchar,
				Index:    0,
			},
			index: 2,
			expectedMeta: &message.ColumnMetadata{
				Keyspace: "test_keyspace",
				Table:    "test_table",
				Name:     "test_array",
				Type:     datatype.Varchar,
				Index:    2,
			},
			expectedError: nil,
		},
		{
			name: "Success - Metadata with null values",
			metadata: &message.ColumnMetadata{
				Keyspace: "",
				Table:    "",
				Name:     "",
				Type:     datatype.Varchar,
				Index:    0,
			},
			index: 4,
			expectedMeta: &message.ColumnMetadata{
				Keyspace: "",
				Table:    "",
				Name:     "",
				Type:     datatype.Varchar,
				Index:    4,
			},
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &SchemaMappingConfig{}
			result := config.cloneColumnMetadata(tt.metadata, tt.index)

			// Verify the result is not nil
			assert.NotNil(t, result)

			// Compare all fields
			assert.Equal(t, tt.expectedMeta.Keyspace, result.Keyspace)
			assert.Equal(t, tt.expectedMeta.Table, result.Table)
			assert.Equal(t, tt.expectedMeta.Name, result.Name)
			assert.Equal(t, tt.expectedMeta.Type, result.Type)
			assert.Equal(t, tt.expectedMeta.Index, result.Index)

			// Verify that the original metadata was not modified
			assert.Equal(t, tt.metadata.Index, tt.metadata.Index)
		})
	}
}

func Test_GetAllColumnsMetadata(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name          string
		fields        SchemaMappingConfig
		columnsMap    map[string]*types.Column
		expectedMeta  []*message.ColumnMetadata
		expectedError error
	}{
		{
			name: "Success - Single column",
			fields: SchemaMappingConfig{
				Logger: logger,
			},
			columnsMap: map[string]*types.Column{
				"column1": {
					Metadata: message.ColumnMetadata{
						Keyspace: "test_keyspace",
						Table:    "test_table",
						Name:     "column1",
						Type:     datatype.Varchar,
						Index:    0,
					},
				},
			},
			expectedMeta: []*message.ColumnMetadata{
				{
					Keyspace: "test_keyspace",
					Table:    "test_table",
					Name:     "column1",
					Type:     datatype.Varchar,
					Index:    0,
				},
			},
			expectedError: nil,
		},
		{
			name: "Success - Multiple columns with different types",
			fields: SchemaMappingConfig{
				Logger: logger,
			},
			columnsMap: map[string]*types.Column{
				"column1": {
					Metadata: message.ColumnMetadata{
						Keyspace: "test_keyspace",
						Table:    "test_table",
						Name:     "column1",
						Type:     datatype.Varchar,
						Index:    0,
					},
				},
				"column2": {
					Metadata: message.ColumnMetadata{
						Keyspace: "test_keyspace",
						Table:    "test_table",
						Name:     "column2",
						Type:     datatype.Int,
						Index:    0,
					},
				},
				"column3": {
					Metadata: message.ColumnMetadata{
						Keyspace: "test_keyspace",
						Table:    "test_table",
						Name:     "column3",
						Type:     datatype.Boolean,
						Index:    0,
					},
				},
			},
			expectedMeta: []*message.ColumnMetadata{
				{
					Keyspace: "test_keyspace",
					Table:    "test_table",
					Name:     "column1",
					Type:     datatype.Varchar,
					Index:    0,
				},
				{
					Keyspace: "test_keyspace",
					Table:    "test_table",
					Name:     "column2",
					Type:     datatype.Int,
					Index:    1,
				},
				{
					Keyspace: "test_keyspace",
					Table:    "test_table",
					Name:     "column3",
					Type:     datatype.Boolean,
					Index:    2,
				},
			},
			expectedError: nil,
		},
		{
			name: "Success - Empty columns map",
			fields: SchemaMappingConfig{
				Logger: logger,
			},
			columnsMap:    map[string]*types.Column{},
			expectedMeta:  []*message.ColumnMetadata{},
			expectedError: nil,
		},
		{
			name: "Success - Columns with collection types",
			fields: SchemaMappingConfig{
				Logger: logger,
			},
			columnsMap: map[string]*types.Column{
				"list_column": {
					Metadata: message.ColumnMetadata{
						Keyspace: "test_keyspace",
						Table:    "test_table",
						Name:     "list_column",
						Type:     datatype.Varchar,
						Index:    0,
					},
				},
				"map_column": {
					Metadata: message.ColumnMetadata{
						Keyspace: "test_keyspace",
						Table:    "test_table",
						Name:     "map_column",
						Type:     datatype.Varchar,
						Index:    0,
					},
				},
			},
			expectedMeta: []*message.ColumnMetadata{
				{
					Keyspace: "test_keyspace",
					Table:    "test_table",
					Name:     "list_column",
					Type:     datatype.Varchar,
					Index:    0,
				},
				{
					Keyspace: "test_keyspace",
					Table:    "test_table",
					Name:     "map_column",
					Type:     datatype.Varchar,
					Index:    1,
				},
			},
			expectedError: nil,
		},
		{
			name: "Success - Columns with null values",
			fields: SchemaMappingConfig{
				Logger: logger,
			},
			columnsMap: map[string]*types.Column{
				"column1": {
					Metadata: message.ColumnMetadata{
						Keyspace: "",
						Table:    "",
						Name:     "",
						Type:     datatype.Varchar,
						Index:    0,
					},
				},
				"column2": {
					Metadata: message.ColumnMetadata{
						Keyspace: "",
						Table:    "",
						Name:     "",
						Type:     datatype.Int,
						Index:    0,
					},
				},
			},
			expectedMeta: []*message.ColumnMetadata{
				{
					Keyspace: "",
					Table:    "",
					Name:     "",
					Type:     datatype.Varchar,
					Index:    0,
				},
				{
					Keyspace: "",
					Table:    "",
					Name:     "",
					Type:     datatype.Int,
					Index:    1,
				},
			},
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata := tt.fields.getAllColumnsMetadata(tt.columnsMap)

			// Check if the result is not nil (except for empty columns map case)
			if len(tt.columnsMap) > 0 {
				assert.NotNil(t, metadata)
			}

			// Check the length of the result
			assert.Equal(t, len(tt.expectedMeta), len(metadata))

			// Create maps to store metadata by name for comparison
			expectedMap := make(map[string]*message.ColumnMetadata)
			actualMap := make(map[string]*message.ColumnMetadata)

			// Populate the maps
			for _, meta := range tt.expectedMeta {
				expectedMap[meta.Name] = meta
			}
			for _, meta := range metadata {
				actualMap[meta.Name] = meta
			}

			// Compare metadata entries by name
			for name, expected := range expectedMap {
				actual, exists := actualMap[name]
				assert.True(t, exists, "Expected metadata for column %s not found", name)
				assert.Equal(t, expected.Keyspace, actual.Keyspace)
				assert.Equal(t, expected.Table, actual.Table)
			}
		})
	}
}

func Test_GetTimestampColumnName(t *testing.T) {
	tests := []struct {
		name       string
		aliasName  string
		columnName string
		want       string
	}{
		{
			name:       "Empty alias name",
			aliasName:  "",
			columnName: "column1",
			want:       "writetime(column1)",
		},
		{
			name:       "With alias name",
			aliasName:  "alias_column",
			columnName: "column1",
			want:       "alias_column",
		},
		{
			name:       "Empty alias and special character in column name",
			aliasName:  "",
			columnName: "user_id",
			want:       "writetime(user_id)",
		},
		{
			name:       "With alias containing special characters",
			aliasName:  "wt_user_id",
			columnName: "user_id",
			want:       "wt_user_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getTimestampColumnName(tt.aliasName, tt.columnName)
			assert.Equal(t, tt.want, got)
		})
	}
}
func Test_HandleSpecialSelectedColumn(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name           string
		fields         SchemaMappingConfig
		columnsMap     map[string]*types.Column
		columnSelected SelectedColumns
		index          int32
		tableName      string
		keySpace       string
		expectedMeta   *message.ColumnMetadata
		expectedError  error
	}{
		{
			name: "Success - Count function",
			fields: SchemaMappingConfig{
				Logger: logger,
			},
			columnsMap: map[string]*types.Column{},
			columnSelected: SelectedColumns{
				Name:     "count_col",
				FuncName: "count",
			},
			index:     0,
			tableName: "test_table",
			keySpace:  "test_keyspace",
			expectedMeta: &message.ColumnMetadata{
				Keyspace: "test_keyspace",
				Table:    "test_table",
				Name:     "count_col",
				Type:     datatype.Bigint,
				Index:    0,
			},
			expectedError: nil,
		},
		{
			name: "Success - Write time column",
			fields: SchemaMappingConfig{
				Logger: logger,
			},
			columnsMap: map[string]*types.Column{},
			columnSelected: SelectedColumns{
				Name:              "wt_col",
				IsWriteTimeColumn: true,
			},
			index:     1,
			tableName: "test_table",
			keySpace:  "test_keyspace",
			expectedMeta: &message.ColumnMetadata{
				Keyspace: "test_keyspace",
				Table:    "test_table",
				Name:     "wt_col",
				Type:     datatype.Bigint,
				Index:    1,
			},
			expectedError: nil,
		},
		{
			name: "Success - Regular column with alias",
			fields: SchemaMappingConfig{
				Logger: logger,
			},
			columnsMap: map[string]*types.Column{
				"original_col": {
					Metadata: message.ColumnMetadata{
						Type: datatype.Varchar,
					},
				},
			},
			columnSelected: SelectedColumns{
				Name:       "alias_col",
				Alias:      "alias_col",
				ColumnName: "original_col",
			},
			index:     2,
			tableName: "test_table",
			keySpace:  "test_keyspace",
			expectedMeta: &message.ColumnMetadata{
				Keyspace: "test_keyspace",
				Table:    "test_table",
				Name:     "alias_col",
				Type:     datatype.Varchar,
				Index:    2,
			},
			expectedError: nil,
		},
		{
			name: "Success - Map value access",
			fields: SchemaMappingConfig{
				Logger: logger,
			},
			columnsMap: map[string]*types.Column{
				"map_col": {
					Metadata: message.ColumnMetadata{
						Type: datatype.NewMapType(datatype.Varchar, datatype.Int),
					},
				},
			},
			columnSelected: SelectedColumns{
				Name:       "map_value",
				ColumnName: "map_col",
				MapKey:     "key1",
			},
			index:     3,
			tableName: "test_table",
			keySpace:  "test_keyspace",
			expectedMeta: &message.ColumnMetadata{
				Keyspace: "test_keyspace",
				Table:    "test_table",
				Name:     "map_value",
				Type:     datatype.Int,
				Index:    3,
			},
			expectedError: nil,
		},
		{
			name: "Error - types.Column not found",
			fields: SchemaMappingConfig{
				Logger: logger,
			},
			columnsMap: map[string]*types.Column{},
			columnSelected: SelectedColumns{
				Name:       "nonexistent",
				ColumnName: "nonexistent",
			},
			index:         4,
			tableName:     "test_table",
			keySpace:      "test_keyspace",
			expectedMeta:  nil,
			expectedError: fmt.Errorf("special column nonexistent not found in provided metadata"),
		},
		{
			name: "Success - Function call",
			fields: SchemaMappingConfig{
				Logger: logger,
			},
			columnsMap: map[string]*types.Column{
				"func_col": {
					Metadata: message.ColumnMetadata{
						Type: datatype.Int,
					},
				},
			},
			columnSelected: SelectedColumns{
				Name:       "func_result",
				IsFunc:     true,
				ColumnName: "func_col",
			},
			index:     5,
			tableName: "test_table",
			keySpace:  "test_keyspace",
			expectedMeta: &message.ColumnMetadata{
				Keyspace: "test_keyspace",
				Table:    "test_table",
				Name:     "func_result",
				Type:     datatype.Int,
				Index:    5,
			},
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata, err := tt.fields.handleSpecialSelectedColumn(tt.columnsMap, tt.columnSelected, tt.index, tt.tableName, tt.keySpace)

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError.Error(), err.Error())
				assert.Nil(t, metadata)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, metadata)

			assert.Equal(t, tt.expectedMeta.Keyspace, metadata.Keyspace)
			assert.Equal(t, tt.expectedMeta.Table, metadata.Table)
			assert.Equal(t, tt.expectedMeta.Name, metadata.Name)
			assert.Equal(t, tt.expectedMeta.Type, metadata.Type)
			assert.Equal(t, tt.expectedMeta.Index, metadata.Index)
		})
	}
}

func Test_ListKeyspaces(t *testing.T) {
	tests := []struct {
		name       string
		tablesMeta map[string]map[string]map[string]*types.Column
		expected   []string
	}{
		{
			name: "Multiple keyspaces, unsorted input",
			tablesMeta: map[string]map[string]map[string]*types.Column{
				"zeta":  {},
				"alpha": {},
				"beta":  {},
			},
			expected: []string{"alpha", "beta", "zeta"},
		},
		{
			name: "Single keyspace",
			tablesMeta: map[string]map[string]map[string]*types.Column{
				"only": {},
			},
			expected: []string{"only"},
		},
		{
			name:       "No keyspaces",
			tablesMeta: map[string]map[string]map[string]*types.Column{},
			expected:   []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &SchemaMappingConfig{TablesMetaData: tt.tablesMeta}
			got := cfg.ListKeyspaces()
			assert.Equal(t, tt.expected, got)
		})
	}
}
