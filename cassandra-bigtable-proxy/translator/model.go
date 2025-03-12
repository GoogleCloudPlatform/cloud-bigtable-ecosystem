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
	"cloud.google.com/go/bigtable"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/message"
	schemaMapping "github.com/ollionorg/cassandra-to-bigtable-proxy/schema-mapping"
	"go.uber.org/zap"
)

type Translator struct {
	Logger              *zap.Logger
	SchemaMappingConfig *schemaMapping.SchemaMappingConfig
}

// Select Query Models
type AsKeywordMeta struct {
	IsFunc   bool
	Name     string
	Alias    string
	DataType string
}

// SelectQueryMap represents the mapping of a select query along with its translation details.
type SelectQueryMap struct {
	Query            string // Original query string
	TranslatedQuery  string
	QueryType        string                    // Type of the query (e.g., SELECT)
	Table            string                    // Table involved in the query
	Keyspace         string                    // Keyspace to which the table belongs
	ColumnMeta       ColumnMeta                // Translator generated Metadata about the columns involved
	Clauses          []Clause                  // List of clauses in the query
	Limit            Limit                     // Limit clause details
	OrderBy          OrderBy                   // Order by clause details
	Params           map[string]interface{}    // Parameters for the query
	ParamKeys        []string                  // column_name of the parameters
	AliasMap         map[string]AsKeywordMeta  // Aliases used in the query
	PrimaryKeys      []string                  // Primary keys of the table
	Columns          []string                  //all columns mentioned in query
	Conditions       map[string]string         // List of conditions in the query
	ReturnMetadata   []*message.ColumnMetadata // Metadata of selected columns in Cassandra format
	VariableMetadata []*message.ColumnMetadata // Metadata of variable columns for prepared queries in Cassandra format
}

type OrderBy struct {
	IsOrderBy bool
	Column    string
	Operation string
}

type Limit struct {
	IsLimit bool
	Count   string
}

type ColumnMeta struct {
	Star   bool
	Column []schemaMapping.SelectedColumns
}

type Column struct {
	Name         string
	ColumnFamily string
	CQLType      string
	IsPrimaryKey bool
}
type IfSpec struct {
	IfExists    bool
	IfNotExists bool
}

type Clause struct {
	Column       string
	Operator     string
	Value        string
	IsPrimaryKey bool
}

type ClauseResponse struct {
	Clauses   []Clause
	Params    map[string]interface{}
	ParamKeys []string
}

type TimestampInfo struct {
	Timestamp         bigtable.Timestamp
	HasUsingTimestamp bool
	Index             int32
}
type ComplexUpdateMeta struct {
	Append           bool              // this is for map/set/list
	PrependList      bool              // this is for list
	Delete           bool              // this is for map/set/list
	UpdateListIndex  string            // this is for List index
	key              interface{}       // this key is for map key
	ExpectedDatatype datatype.DataType // this datatype has to be provided in case of change in expected datatype.
	Value            []byte            // this is value for setting at index for list
	ListDelete       bool              // this is for list = list - {value1, value2}
	ListDeleteValues [][]byte          // this stores the values to be deleted from list
}

// InsertQueryMap represents the mapping of an insert query along with its translation details.
type InsertQueryMap struct {
	Query                string                    // Original query string
	QueryType            string                    // Type of the query (e.g., INSERT)
	Table                string                    // Table involved in the query
	Keyspace             string                    // Keyspace to which the table belongs
	Columns              []Column                  // List of columns involved in the insert operation
	Values               []interface{}             // Values to be inserted
	Params               map[string]interface{}    // Parameters for the query
	ParamKeys            []string                  // Column names of the parameters
	PrimaryKeys          []string                  // Primary keys of the table
	RowKey               string                    // Unique row key for insert operation
	DeleteColumnFamilies []string                  // List of Column family which is type of collection type
	ReturnMetadata       []*message.ColumnMetadata // Metadata of all columns of that table in Cassandra format
	VariableMetadata     []*message.ColumnMetadata // Metadata of variable columns for prepared queries in Cassandra format
	TimestampInfo        TimestampInfo
	IfNotExists          bool
}

type ColumnsResponse struct {
	Columns       []Column
	ValueArr      []string
	ParamKeys     []string
	PrimayColumns []string
}

// DeleteQueryMap represents the mapping of a delete query along with its translation details.
type DeleteQueryMap struct {
	Query             string                    // Original query string
	QueryType         string                    // Type of the query (e.g., DELETE)
	Table             string                    // Table involved in the query
	Keyspace          string                    // Keyspace to which the table belongs
	Clauses           []Clause                  // List of clauses in the delete query
	Params            map[string]interface{}    // Parameters for the query
	ParamKeys         []string                  // Column names of the parameters
	PrimaryKeys       []string                  // Primary keys of the table
	RowKey            string                    // Unique rowkey which is required for delete operation
	ExecuteByMutation bool                      // Flag to indicate if the delete should be executed by mutation
	VariableMetadata  []*message.ColumnMetadata // Metadata of variable columns for prepared queries in Cassandra format
	ReturnMetadata    []*message.ColumnMetadata // Metadata of all columns of that table in Cassandra format
	TimestampInfo     TimestampInfo
	IfExists          bool
	SelectedColumns   []schemaMapping.SelectedColumns
}

// UpdateQueryMap represents the mapping of an update query along with its translation details.
type UpdateQueryMap struct {
	Query                 string // Original query string
	TranslatedQuery       string
	QueryType             string                    // Type of the query (e.g., UPDATE)
	Table                 string                    // Table involved in the query
	Keyspace              string                    // Keyspace to which the table belongs
	UpdateSetValues       []UpdateSetValue          // Values to be updated
	Clauses               []Clause                  // List of clauses in the update query
	Params                map[string]interface{}    // Parameters for the query
	ParamKeys             []string                  // Column names of the parameters
	PrimaryKeys           []string                  // Primary keys of the table
	Columns               []Column                  // List of columns in update query
	Values                []interface{}             // values for the update
	RowKey                string                    // Unique rowkey which is required for update operation
	DeleteColumnFamilies  []string                  // List of all collection type of columns
	DeleteColumQualifires []Column                  // List of all map key deletion in complex update
	ReturnMetadata        []*message.ColumnMetadata // Metadata of all columns of that table in Cassandra format
	VariableMetadata      []*message.ColumnMetadata // Metadata of variable columns for prepared queries in Cassandra format
	TimestampInfo         TimestampInfo
	IfExists              bool //
	ComplexUpdateMeta     map[string]*ComplexUpdateMeta
}

type UpdateSetValue struct {
	Column       string
	Value        string
	RawValue     interface{}
	ColumnFamily string
	CQLType      string
	Encrypted    interface{}
}

type UpdateSetResponse struct {
	UpdateSetValues []UpdateSetValue
	ParamKeys       []string
	Params          map[string]interface{}
}

type TableObj struct {
	TableName    string
	KeyspaceName string
}
