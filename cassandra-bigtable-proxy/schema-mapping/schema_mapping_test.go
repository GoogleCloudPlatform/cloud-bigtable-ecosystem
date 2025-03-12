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
	"reflect"
	"testing"

	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"go.uber.org/zap"
)

var systemColumnFamily = "cf1"
var tablesMetaData = map[string]map[string]map[string]*Column{
	"keyspace": {"table1": {
		"column1": {
			CQLType:      "text",
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

func TestSchemaMappingConfig_GetColumnType(t *testing.T) {
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

	columnExistsWant := &ColumnType{
		IsPrimaryKey: false,
		IsCollection: false,
	}

	tests := []struct {
		name   string
		fields SchemaMappingConfig
		args   struct {
			tableName  string
			columnName string
		}
		want    *ColumnType
		wantErr bool
	}{
		{
			name: "Column exists",
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
			name: "Column exists in different table",
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
			got, err := tt.fields.GetColumnType(tt.args.tableName, tt.args.columnName, "keyspace")
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

func TestSchemaMappingConfig_GetMetadataForColumns(t *testing.T) {
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
			name: "column exists",
			fields: SchemaMappingConfig{
				TablesMetaData: map[string]map[string]map[string]*Column{
					"keyspace": {"table1": {}},
				},
				SystemColumnFamily: "cf1",
			},
			args: struct {
				tableName   string
				columnNames []string
			}{
				tableName:   "table1",
				columnNames: []string{"bigtable_ttl_ts"},
			},
			wantErr: true,
		},
		{
			name: "column exists",
			fields: SchemaMappingConfig{
				TablesMetaData:     tablesMetaData,
				SystemColumnFamily: "cf1",
			},
			args: struct {
				tableName   string
				columnNames []string
			}{
				tableName:   "table1",
				columnNames: []string{"bigtable_ttl_ts"},
			},
			want: []*message.ColumnMetadata{
				{Keyspace: "test-keyspace", Name: "bigtable_ttl_ts", Table: "table1", Type: datatype.Int, Index: 0},
			},
			wantErr: false,
		},
		{
			name: "column exists",
			fields: SchemaMappingConfig{
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
			name: "column not exists",
			fields: SchemaMappingConfig{
				Logger:             zap.NewNop(),
				TablesMetaData:     tablesMetaData,
				SystemColumnFamily: "cf1",
			},
			args: struct {
				tableName   string
				columnNames []string
			}{
				tableName:   "table2",
				columnNames: []string{"column1"},
			},
			wantErr: true,
		},
		{
			name: "empty column input",
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.fields.GetMetadataForColumns(tt.args.tableName, tt.args.columnNames, "keyspace")
			if (err != nil) != tt.wantErr {
				t.Errorf("GetMetadataForColumns() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetMetadataForColumns() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSchemaMappingConfig_GetMetadataForSelectedColumns(t *testing.T) {
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

func TestGetPkByTableName(t *testing.T) {
	logger := zap.NewNop()

	// Sample data for testing
	pkMetadataCache := map[string]map[string][]Column{
		"keyspace1": {
			"table1": {
				{
					CQLType:      "text",
					ColumnName:   "id",
					ColumnType:   "testColumn",
					IsPrimaryKey: true,
					PkPrecedence: 1,
					IsCollection: false,
				},
			},
		},
	}

	expectedPkResponse := []Column{
		{
			CQLType:      "text",
			ColumnName:   "id",
			ColumnType:   "testColumn",
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
		want    []Column
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
func TestGetPkKeyType(t *testing.T) {
	logger := zap.NewNop()

	// Sample data for testing
	pkMetadataCache := map[string]map[string][]Column{
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
