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
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/utilities"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func getSchemaMappingConfig() *SchemaMappingConfig {
	return NewSchemaMappingConfig(
		"cf1",
		zap.NewNop(),
		[]*TableConfig{
			NewTableConfig(
				"keyspace",
				"table1",
				"cf1",
				types.OrderedCodeEncoding,
				[]*types.Column{
					{
						Name:         "column1",
						CQLType:      datatype.Varchar,
						KeyType:      utilities.KEY_TYPE_REGULAR,
						ColumnFamily: "cf1",
					},
					{
						Name:         "column2",
						CQLType:      datatype.Int,
						KeyType:      utilities.KEY_TYPE_REGULAR,
						ColumnFamily: "cf1",
					},
				},
			),
			NewTableConfig(
				"keyspace",
				"table2",
				"cf1",
				types.OrderedCodeEncoding,
				[]*types.Column{
					{
						Name:         "id",
						CQLType:      datatype.Int,
						KeyType:      "partition",
						IsPrimaryKey: true,
					},
					{
						Name:         "name",
						CQLType:      datatype.Varchar,
						KeyType:      "clustering",
						IsPrimaryKey: true,
					},
				},
			),
		},
	)
}

var expectedResponse = []*message.ColumnMetadata{
	{Keyspace: "keyspace", Name: "column1", Table: "table1", Type: datatype.Varchar, Index: 0},
}

func Test_GetColumn(t *testing.T) {
	columnExistsInDifferentTableArgs := struct {
		tableName  string
		columnName string
	}{
		tableName:  "table1",
		columnName: "column3",
	}

	tests := []struct {
		name   string
		fields *SchemaMappingConfig
		args   struct {
			tableName  string
			columnName string
		}
		want    *types.Column
		wantErr bool
	}{
		{
			name:   "types.Column exists",
			fields: getSchemaMappingConfig(),
			args: struct {
				tableName  string
				columnName string
			}{
				tableName:  "table1",
				columnName: "column1",
			},
			want: &types.Column{
				Name:         "column1",
				ColumnFamily: "cf1",
				IsPrimaryKey: false,
				CQLType:      datatype.Varchar,
			},
			wantErr: false,
		},
		{
			name:    "types.Column exists in different table",
			fields:  getSchemaMappingConfig(),
			args:    columnExistsInDifferentTableArgs,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc, err := tt.fields.GetTableConfig("keyspace", tt.args.tableName)
			if err != nil {
				t.Errorf("table config error: %v", err)
				return
			}
			got, err := tc.GetColumn(tt.args.columnName)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want.Name, got.Name)
				assert.Equal(t, tt.want.ColumnFamily, got.ColumnFamily)
				assert.Equal(t, tt.want.IsPrimaryKey, got.IsPrimaryKey)
				assert.Equal(t, tt.want.CQLType, got.CQLType)
			}
		})
	}
}

func Test_GetMetadataForColumns(t *testing.T) {
	tests := []struct {
		name   string
		fields *SchemaMappingConfig
		args   struct {
			tableName   string
			columnNames []string
		}
		want    []*message.ColumnMetadata
		wantErr bool
	}{
		{
			name:   "Success - Single regular column",
			fields: getSchemaMappingConfig(),
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
			name:   "Success - Multiple regular columns",
			fields: getSchemaMappingConfig(),
			args: struct {
				tableName   string
				columnNames []string
			}{
				tableName:   "table1",
				columnNames: []string{"column1", "column2"},
			},
			want: []*message.ColumnMetadata{
				{Name: "column1", Type: datatype.Varchar, Index: 0},
				{Name: "column2", Type: datatype.Int, Index: 1},
			},
			wantErr: false,
		},
		{
			name:   "Success - Special column (LimitValue)",
			fields: getSchemaMappingConfig(),
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
			name:   "Success - Mixed column types",
			fields: getSchemaMappingConfig(),
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
			name:   "Error - types.Column not found in metadata",
			fields: getSchemaMappingConfig(),
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
			name:   "Empty column names",
			fields: getSchemaMappingConfig(),
			args: struct {
				tableName   string
				columnNames []string
			}{
				tableName:   "table1",
				columnNames: []string{},
			},
			want: []*message.ColumnMetadata{
				{Keyspace: "keyspace", Name: "column1", Table: "table1", Type: datatype.Varchar, Index: 0},
				{Keyspace: "keyspace", Name: "column2", Table: "table1", Type: datatype.Int, Index: 1},
			},
			wantErr: false,
		},
		{
			name:   "Success - Multiple special columns",
			fields: getSchemaMappingConfig(),
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
			tc, err := tt.fields.GetTableConfig("keyspace", tt.args.tableName)
			if err != nil {
				t.Errorf("table config error: %v", err)
				return
			}
			got, err := tc.GetMetadataForColumns(tt.args.columnNames)
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
	tests := []struct {
		name   string
		fields *SchemaMappingConfig
		args   struct {
			tableName   string
			columnNames []types.SelectedColumn
			keySpace    string
		}
		want    []*message.ColumnMetadata
		wantErr bool
	}{
		{
			name:   "Successfully retrieve metadata for specific column",
			fields: getSchemaMappingConfig(),
			args: struct {
				tableName   string
				columnNames []types.SelectedColumn
				keySpace    string
			}{
				tableName:   "table1",
				columnNames: []types.SelectedColumn{{Name: "column1"}},
				keySpace:    "keyspace",
			},
			want:    expectedResponse,
			wantErr: false,
		},
		{
			name:   "Return all columns when no specific columns provided",
			fields: getSchemaMappingConfig(),
			args: struct {
				tableName   string
				columnNames []types.SelectedColumn
				keySpace    string
			}{
				tableName:   "table1",
				columnNames: []types.SelectedColumn{},
				keySpace:    "keyspace",
			},
			want: []*message.ColumnMetadata{
				{Keyspace: "keyspace", Name: "column1", Table: "table1", Type: datatype.Varchar, Index: 0},
				{Keyspace: "keyspace", Name: "column2", Table: "table1", Type: datatype.Int, Index: 1},
			},
			wantErr: false,
		},
		{
			name:   "Error when column is not found",
			fields: getSchemaMappingConfig(),
			args: struct {
				tableName   string
				columnNames []types.SelectedColumn
				keySpace    string
			}{
				tableName:   "table1",
				columnNames: []types.SelectedColumn{{Name: "nonexistent_column"}},
				keySpace:    "keyspace",
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc, err := tt.fields.GetTableConfig("keyspace", tt.args.tableName)
			if err != nil {
				t.Errorf("table config error: %v", err)
				return
			}
			got, err := tc.GetMetadataForSelectedColumns(tt.args.columnNames)
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

func Test_GetPkKeyType(t *testing.T) {
	tests := []struct {
		name   string
		fields *SchemaMappingConfig
		args   struct {
			tableName  string
			keySpace   string
			columnName string
		}
		want    string
		wantErr bool
	}{
		{
			name:   "Successfully get partition key type",
			fields: getSchemaMappingConfig(),
			args: struct {
				tableName  string
				keySpace   string
				columnName string
			}{
				tableName:  "table2",
				keySpace:   "keyspace",
				columnName: "id",
			},
			want:    "partition",
			wantErr: false,
		},
		{
			name:   "Successfully get clustering key type",
			fields: getSchemaMappingConfig(),
			args: struct {
				tableName  string
				keySpace   string
				columnName string
			}{
				tableName:  "table2",
				keySpace:   "keyspace",
				columnName: "name",
			},
			want:    "clustering",
			wantErr: false,
		},
		{
			name:   "Error when column is not a primary key",
			fields: getSchemaMappingConfig(),
			args: struct {
				tableName  string
				keySpace   string
				columnName string
			}{
				tableName:  "table2",
				keySpace:   "keyspace",
				columnName: "nonexistent_column",
			},
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc, err := tt.fields.GetTableConfig(tt.args.keySpace, tt.args.tableName)
			if err != nil {
				t.Errorf("table config error: %v", err)
				return
			}
			got, err := tc.GetPkKeyType(tt.args.columnName)
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
			metadata, err := handleSpecialColumn(tt.columnsMap, tt.columnName, tt.index, tt.isWriteTimeFunction)

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

func Test_GetSpecificColumnsMetadataForSelectedColumns(t *testing.T) {
	tests := []struct {
		name          string
		fields        *SchemaMappingConfig
		columnsMap    map[string]*types.Column
		selectedCols  []types.SelectedColumn
		tableName     string
		expectedMeta  []*message.ColumnMetadata
		expectedError error
	}{
		{
			name:   "Success - Regular column",
			fields: getSchemaMappingConfig(),
			columnsMap: map[string]*types.Column{
				"column1": {
					Metadata: message.ColumnMetadata{
						Keyspace: "keyspace",
						Table:    "table1",
						Name:     "column1",
						Type:     datatype.Varchar,
						Index:    0,
					},
				},
			},
			selectedCols: []types.SelectedColumn{
				{
					Name: "column1",
				},
			},
			tableName: "table1",
			expectedMeta: []*message.ColumnMetadata{
				{
					Keyspace: "keyspace",
					Table:    "table1",
					Name:     "column1",
					Type:     datatype.Varchar,
					Index:    0,
				},
			},
			expectedError: nil,
		},
		{
			name:   "Success - Write time column",
			fields: getSchemaMappingConfig(),
			columnsMap: map[string]*types.Column{
				"column1": {
					Metadata: message.ColumnMetadata{
						Keyspace: "keyspace",
						Table:    "table1",
						Name:     "column1",
						Type:     datatype.Varchar,
						Index:    0,
					},
				},
			},
			selectedCols: []types.SelectedColumn{
				{
					Name:              "writetime_column",
					IsWriteTimeColumn: true,
					ColumnName:        "column1",
				},
			},
			tableName: "table1",
			expectedMeta: []*message.ColumnMetadata{
				{
					Keyspace: "keyspace",
					Table:    "table1",
					Name:     "writetime(column1)",
					Type:     datatype.Bigint,
					Index:    0,
				},
			},
			expectedError: nil,
		},
		{
			name:   "Success - Special column",
			fields: getSchemaMappingConfig(),
			columnsMap: map[string]*types.Column{
				"column1": {
					Metadata: message.ColumnMetadata{
						Keyspace: "keyspace",
						Table:    "table1",
						Name:     "column1",
						Type:     datatype.Varchar,
						Index:    0,
					},
				},
			},
			selectedCols: []types.SelectedColumn{
				{
					Name: LimitValue,
				},
			},
			tableName: "table1",
			expectedMeta: []*message.ColumnMetadata{
				{
					Keyspace: "keyspace",
					Table:    "table1",
					Name:     LimitValue,
					Type:     datatype.Bigint,
					Index:    0,
				},
			},
			expectedError: nil,
		},
		{
			name:   "Success - Function call",
			fields: getSchemaMappingConfig(),
			columnsMap: map[string]*types.Column{
				"column1": {
					Metadata: message.ColumnMetadata{
						Keyspace: "keyspace",
						Table:    "table1",
						Name:     "column1",
						Type:     datatype.Varchar,
						Index:    0,
					},
				},
			},
			selectedCols: []types.SelectedColumn{
				{
					Name:   "column1",
					IsFunc: true,
				},
			},
			tableName:     "table1",
			expectedMeta:  nil,
			expectedError: nil,
		},
		{
			name:   "Error - types.Column not found",
			fields: getSchemaMappingConfig(),
			columnsMap: map[string]*types.Column{
				"column1": {
					Metadata: message.ColumnMetadata{
						Keyspace: "keyspace",
						Table:    "table1",
						Name:     "column1",
						Type:     datatype.Varchar,
						Index:    0,
					},
				},
			},
			selectedCols: []types.SelectedColumn{
				{
					Name: "nonexistent_column",
				},
			},
			tableName:     "table1",
			expectedMeta:  nil,
			expectedError: fmt.Errorf("metadata not found for column `nonexistent_column` in table `table1`"),
		},
		{
			name:   "Error - Invalid special column",
			fields: getSchemaMappingConfig(),
			columnsMap: map[string]*types.Column{
				"column1": {
					Metadata: message.ColumnMetadata{
						Keyspace: "keyspace",
						Table:    "table1",
						Name:     "column1",
						Type:     datatype.Varchar,
						Index:    0,
					},
				},
			},
			selectedCols: []types.SelectedColumn{
				{
					Name: "invalid_special_column",
				},
			},
			tableName:     "table1",
			expectedMeta:  nil,
			expectedError: fmt.Errorf("metadata not found for column `invalid_special_column` in table `table1`"),
		},
		{
			name:       "Error - Empty columns map",
			fields:     getSchemaMappingConfig(),
			columnsMap: map[string]*types.Column{},
			selectedCols: []types.SelectedColumn{
				{
					Name: "column1",
				},
			},
			tableName:     "table1",
			expectedMeta:  nil,
			expectedError: fmt.Errorf("metadata not found for column `column1` in table `table1`"),
		},
		{
			name:       "Error - Write time column not found",
			fields:     getSchemaMappingConfig(),
			columnsMap: map[string]*types.Column{},
			selectedCols: []types.SelectedColumn{
				{
					Name:              "no_write_time_column",
					IsWriteTimeColumn: true,
					ColumnName:        "nonexistent_column",
				},
			},
			tableName:     "table1",
			expectedMeta:  nil,
			expectedError: fmt.Errorf("special column writetime(nonexistent_column) not found in provided metadata"),
		},
		{
			name:       "Error - Special column handling error",
			fields:     getSchemaMappingConfig(),
			columnsMap: map[string]*types.Column{},
			selectedCols: []types.SelectedColumn{
				{
					Name: LimitValue,
				},
			},
			tableName:     "table1",
			expectedMeta:  nil,
			expectedError: fmt.Errorf("special column %s not found in provided metadata", LimitValue),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			tc, err := tt.fields.GetTableConfig("keyspace", tt.tableName)
			if err != nil {
				t.Errorf("table config error: %v", err)
				return
			}
			metadata, err := tc.getSpecificColumnsMetadataForSelectedColumns(tt.columnsMap, tt.selectedCols)

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
	tests := []struct {
		name          string
		fields        *SchemaMappingConfig
		columnNames   []string
		tableName     string
		expectedMeta  []*message.ColumnMetadata
		expectedError error
	}{
		{
			name:        "Success - Regular column",
			fields:      getSchemaMappingConfig(),
			columnNames: []string{"column1"},
			tableName:   "table1",
			expectedMeta: []*message.ColumnMetadata{
				{
					Name:     "column1",
					Table:    "table1",
					Keyspace: "keyspace",
					Type:     datatype.Varchar,
					Index:    0,
				},
			},
			expectedError: nil,
		},
		{
			name:        "Success - Special column (LimitValue)",
			fields:      getSchemaMappingConfig(),
			columnNames: []string{LimitValue},
			tableName:   "table1",
			expectedMeta: []*message.ColumnMetadata{
				{
					Type:     datatype.Bigint,
					Index:    0,
					Name:     LimitValue,
					Keyspace: "keyspace",
					Table:    "table1"},
			},
			expectedError: nil,
		},
		{
			name:        "Success - Multiple columns",
			fields:      getSchemaMappingConfig(),
			columnNames: []string{"column1", "column2"},
			tableName:   "table1",
			expectedMeta: []*message.ColumnMetadata{
				{
					Name:     "column1",
					Keyspace: "keyspace",
					Table:    "table1",
					Type:     datatype.Varchar,
					Index:    0,
				},
				{
					Name:     "column2",
					Keyspace: "keyspace",
					Table:    "table1",
					Type:     datatype.Int,
					Index:    1,
				},
			},
			expectedError: nil,
		},
		{
			name:        "Success - Mixed column types",
			fields:      getSchemaMappingConfig(),
			columnNames: []string{"column1", LimitValue},
			tableName:   "table1",
			expectedMeta: []*message.ColumnMetadata{
				{
					Name:     "column1",
					Keyspace: "keyspace",
					Table:    "table1",
					Type:     datatype.Varchar,
					Index:    0,
				},
				{
					Name:     LimitValue,
					Keyspace: "keyspace",
					Table:    "table1",
					Type:     datatype.Bigint,
					Index:    1,
				},
			},
			expectedError: nil,
		},
		{
			name:          "Error - types.Column not found",
			fields:        getSchemaMappingConfig(),
			columnNames:   []string{"nonexistent_column"},
			tableName:     "table1",
			expectedMeta:  nil,
			expectedError: fmt.Errorf("metadata not found for column `nonexistent_column` in table `table1`"),
		},
		{
			name:          "Success - Empty column names",
			fields:        getSchemaMappingConfig(),
			columnNames:   []string{},
			tableName:     "table1",
			expectedMeta:  nil,
			expectedError: nil,
		},
		{
			name:        "Success - Multiple special columns",
			fields:      getSchemaMappingConfig(),
			columnNames: []string{LimitValue, LimitValue},
			tableName:   "table1",
			expectedMeta: []*message.ColumnMetadata{
				{
					Table:    "table1",
					Keyspace: "keyspace",
					Type:     datatype.Bigint,
					Index:    0,
					Name:     LimitValue,
				},
				{
					Table:    "table1",
					Keyspace: "keyspace",
					Type:     datatype.Bigint,
					Index:    1,
					Name:     LimitValue,
				},
			},
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc, err := tt.fields.GetTableConfig("keyspace", tt.tableName)
			if err != nil {
				t.Errorf("table config error: %v", err)
				return
			}
			metadata, err := tc.getSpecificColumnsMetadata(tt.columnNames)

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
					assert.True(t, exists, "Expected metadata for column '%s' not found", name)
					assert.NotNil(t, actual, "nil metadata result for column '%s'", name)
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
			result := cloneColumnMetadata(tt.metadata, tt.index)

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
	tests := []struct {
		name          string
		fields        *SchemaMappingConfig
		columnsMap    map[string]*types.Column
		expectedMeta  []*message.ColumnMetadata
		expectedError error
	}{
		{
			name:   "Success - Single column",
			fields: getSchemaMappingConfig(),
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
			name:   "Success - Multiple columns with different types",
			fields: getSchemaMappingConfig(),
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
			name:          "Success - Empty columns map",
			fields:        getSchemaMappingConfig(),
			columnsMap:    map[string]*types.Column{},
			expectedMeta:  []*message.ColumnMetadata{},
			expectedError: nil,
		},
		{
			name:   "Success - Columns with collection types",
			fields: getSchemaMappingConfig(),
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
			name:   "Success - Columns with null values",
			fields: getSchemaMappingConfig(),
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
			metadata := getAllColumnsMetadata(tt.columnsMap)

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
	tests := []struct {
		name           string
		fields         *SchemaMappingConfig
		columnsMap     map[string]*types.Column
		columnSelected types.SelectedColumn
		index          int32
		tableName      string
		keySpace       string
		expectedMeta   *message.ColumnMetadata
		expectedError  error
	}{
		{
			name:       "Success - Count function",
			fields:     getSchemaMappingConfig(),
			columnsMap: map[string]*types.Column{},
			columnSelected: types.SelectedColumn{
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
			name:       "Success - Write time column",
			fields:     getSchemaMappingConfig(),
			columnsMap: map[string]*types.Column{},
			columnSelected: types.SelectedColumn{
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
			name:   "Success - Regular column with alias",
			fields: getSchemaMappingConfig(),
			columnsMap: map[string]*types.Column{
				"original_col": {
					Metadata: message.ColumnMetadata{
						Type: datatype.Varchar,
					},
				},
			},
			columnSelected: types.SelectedColumn{
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
			name:   "Success - Map value access",
			fields: getSchemaMappingConfig(),
			columnsMap: map[string]*types.Column{
				"map_col": {
					Metadata: message.ColumnMetadata{
						Type: datatype.NewMapType(datatype.Varchar, datatype.Int),
					},
				},
			},
			columnSelected: types.SelectedColumn{
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
			name:       "Error - types.Column not found",
			fields:     getSchemaMappingConfig(),
			columnsMap: map[string]*types.Column{},
			columnSelected: types.SelectedColumn{
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
			name:   "Success - Function call",
			fields: getSchemaMappingConfig(),
			columnsMap: map[string]*types.Column{
				"func_col": {
					Metadata: message.ColumnMetadata{
						Type: datatype.Int,
					},
				},
			},
			columnSelected: types.SelectedColumn{
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
			tc := TableConfig{
				Keyspace: tt.keySpace,
				Name:     tt.tableName,
			}
			metadata, err := tc.handleSpecialSelectedColumn(tt.columnsMap, tt.columnSelected, tt.index)

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
		name     string
		tables   map[string]map[string]*TableConfig
		expected []string
	}{
		{
			name: "Multiple keyspaces, unsorted input",
			tables: map[string]map[string]*TableConfig{
				"zeta":  {},
				"alpha": {},
				"beta":  {},
			},
			expected: []string{"alpha", "beta", "zeta"},
		},
		{
			name: "Single keyspace",
			tables: map[string]map[string]*TableConfig{
				"only": {},
			},
			expected: []string{"only"},
		},
		{
			name:     "No keyspaces",
			tables:   map[string]map[string]*TableConfig{},
			expected: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &SchemaMappingConfig{tables: tt.tables}
			got := cfg.ListKeyspaces()
			assert.Equal(t, tt.expected, got)
		})
	}
}

func Test_sortPrimaryKeysData(t *testing.T) {
	type args struct {
		pkMetadata []*types.Column
	}
	tests := []struct {
		name string
		args args
		want []*types.Column
	}{
		{
			name: "Basic Sorting",
			args: args{
				pkMetadata: []*types.Column{
					{Name: "id", PkPrecedence: 2},
					{Name: "email", PkPrecedence: 1},
				},
			},
			want: []*types.Column{
				{Name: "email", PkPrecedence: 1},
				{Name: "id", PkPrecedence: 2},
			},
		},
		{
			name: "Already Sorted Data",
			args: args{
				pkMetadata: []*types.Column{
					{Name: "order_id", PkPrecedence: 1},
					{Name: "customer_id", PkPrecedence: 2},
				},
			},
			want: []*types.Column{
				{Name: "order_id", PkPrecedence: 1},
				{Name: "customer_id", PkPrecedence: 2},
			},
		},
		{
			name: "Same Precedence Values (Unchanged Order)",
			args: args{
				pkMetadata: []*types.Column{
					{Name: "product_id", PkPrecedence: 1},
					{Name: "category_id", PkPrecedence: 1},
				},
			},
			want: []*types.Column{
				{Name: "product_id", PkPrecedence: 1},
				{Name: "category_id", PkPrecedence: 1}, // Order should remain the same
			},
		},
		{
			name: "Single types.Column (No Sorting Needed)",
			args: args{
				pkMetadata: []*types.Column{
					{Name: "category_id", PkPrecedence: 1},
				},
			},
			want: []*types.Column{
				{Name: "category_id", PkPrecedence: 1},
			},
		},
		{
			name: "Empty Slice (No Operation)",
			args: args{
				pkMetadata: []*types.Column{},
			},
			want: []*types.Column{},
		},
		{
			name: "Table With Nil Columns (Should Not Panic)",
			args: args{
				pkMetadata: nil,
			},
			want: nil,
		},
		{
			name: "Negative Precedence Values (Still Sorted Correctly)",
			args: args{
				pkMetadata: []*types.Column{
					{Name: "col1", PkPrecedence: -1},
					{Name: "col2", PkPrecedence: -3},
					{Name: "col3", PkPrecedence: -2},
				},
			},
			want: []*types.Column{
				{Name: "col2", PkPrecedence: -3},
				{Name: "col3", PkPrecedence: -2},
				{Name: "col1", PkPrecedence: -1},
			},
		},
		{
			name: "Zero Precedence Values (Sorted Normally)",
			args: args{
				pkMetadata: []*types.Column{
					{Name: "colA", PkPrecedence: 0},
					{Name: "colB", PkPrecedence: 2},
					{Name: "colC", PkPrecedence: 1},
				},
			},
			want: []*types.Column{
				{Name: "colA", PkPrecedence: 0},
				{Name: "colC", PkPrecedence: 1},
				{Name: "colB", PkPrecedence: 2},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sortPrimaryKeys(tt.args.pkMetadata)
			if !reflect.DeepEqual(tt.args.pkMetadata, tt.want) {
				t.Errorf("sortPrimaryKeys() = %v, want %v", tt.args.pkMetadata, tt.want)
			}
		})
	}
}
