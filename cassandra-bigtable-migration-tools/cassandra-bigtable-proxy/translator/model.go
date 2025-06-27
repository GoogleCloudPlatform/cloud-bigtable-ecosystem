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
	types "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	schemaMapping "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/schema-mapping"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"go.uber.org/zap"
)

type Translator struct {
	Logger *zap.Logger
	// todo remove once we support ordered code ints
	EncodeIntValuesWithBigEndian bool
	SchemaMappingConfig          *schemaMapping.SchemaMappingConfig
}

// SelectQueryMap represents the mapping of a select query along with its translation details.
type SelectQueryMap struct {
	Query            string // Original query string
	TranslatedQuery  string
	QueryType        string                      // Type of the query (e.g., SELECT)
	Table            string                      // Table involved in the query
	Keyspace         string                      // Keyspace to which the table belongs
	ColumnMeta       ColumnMeta                  // Translator generated Metadata about the columns involved
	Clauses          []types.Clause              // List of clauses in the query
	Limit            Limit                       // Limit clause details
	OrderBy          OrderBy                     // Order by clause details
	GroupByColumns   []string                    // Group by Columns
	Params           map[string]interface{}      // Parameters for the query
	ParamKeys        []string                    // column_name of the parameters
	PrimaryKeys      []string                    // Primary keys of the table
	Columns          []string                    //all columns mentioned in query
	Conditions       map[string]string           // List of conditions in the query
	ReturnMetadata   []*message.ColumnMetadata   // Metadata of selected columns in Cassandra format
	VariableMetadata []*message.ColumnMetadata   // Metadata of variable columns for prepared queries in Cassandra format
	CachedBTPrepare  *bigtable.PreparedStatement // prepared statement object for bigtable
	ParamTypes       map[string]datatype.DataType
}

type OrderOperation string

const (
	Asc  OrderOperation = "asc"
	Desc OrderOperation = "desc"
)

type QueryTypesEnum string

const (
	INSERT QueryTypesEnum = "INSERT"
	SELECT QueryTypesEnum = "SELECT"
	UPDATE QueryTypesEnum = "UPDATE"
	DELETE QueryTypesEnum = "DELETE"
)

type OrderBy struct {
	IsOrderBy bool
	Columns   []OrderByColumn
}

type OrderByColumn struct {
	Column    string
	Operation OrderOperation
}

type Limit struct {
	IsLimit bool
	Count   string
}

type ColumnMeta struct {
	Star   bool
	Column []schemaMapping.SelectedColumns
}

type IfSpec struct {
	IfExists    bool
	IfNotExists bool
}

// This struct will be useful in combining all the clauses into one.
type QueryClauses struct {
	Clauses   []types.Clause
	Params    map[string]interface{}
	ParamKeys []string
}

type TimestampInfo struct {
	Timestamp         bigtable.Timestamp
	HasUsingTimestamp bool
	Index             int32
}

type IncrementOperationType int

const (
	None IncrementOperationType = iota
	Increment
	Decrement
)

type ComplexOperation struct {
	Append           bool                   // this is for map/set/list
	PrependList      bool                   // this is for list
	Delete           bool                   // this is for map/set/list
	IncrementType    IncrementOperationType // for incrementing a counter
	IncrementValue   int64                  // how much to increment a counter by
	UpdateListIndex  string                 // this is for List index
	ExpectedDatatype datatype.DataType      // this datatype has to be provided in case of change in want datatype.
	mapKey           interface{}            // this key is for map key
	Value            []byte                 // this is value for setting at index for list
	ListDelete       bool                   // this is for list = list - {value1, value2}
	ListDeleteValues [][]byte               // this stores the values to be deleted from list
}

// InsertQueryMapping represents the mapping of an insert query along with its translation details.
type InsertQueryMapping struct {
	Query                string                    // Original query string
	QueryType            QueryTypesEnum            // Type of the query (e.g., INSERT)
	Table                string                    // Table involved in the query
	Keyspace             string                    // Keyspace to which the table belongs
	Columns              []types.Column            // List of columns involved in the insert operation
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
	Columns       []types.Column
	ParamKeys     []string
	PrimayColumns []string
}

// DeleteQueryMapping represents the mapping of a delete query along with its translation details.
type DeleteQueryMapping struct {
	Query             string                    // Original query string
	QueryType         QueryTypesEnum            // Type of the query (e.g., DELETE)
	Table             string                    // Table involved in the query
	Keyspace          string                    // Keyspace to which the table belongs
	Clauses           []types.Clause            // List of clauses in the delete query
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

type CreateTableStatementMap struct {
	QueryType   string
	Keyspace    string
	Table       string
	IfNotExists bool
	Columns     []message.ColumnMetadata
	PrimaryKeys []CreateTablePrimaryKeyConfig
}

type CreateTablePrimaryKeyConfig struct {
	Name    string
	KeyType string
}

type AlterTableStatementMap struct {
	QueryType   string
	Keyspace    string
	Table       string
	IfNotExists bool
	AddColumns  []message.ColumnMetadata
	DropColumns []string
}

type DropTableStatementMap struct {
	QueryType string
	Keyspace  string
	Table     string
	IfExists  bool
}

type TruncateTableStatementMap struct {
	QueryType string
	Keyspace  string
	Table     string
}

// UpdateQueryMapping represents the mapping of an update query along with its translation details.
type UpdateQueryMapping struct {
	Query                 string // Original query string
	TranslatedQuery       string
	QueryType             QueryTypesEnum            // Type of the query (e.g., UPDATE)
	Table                 string                    // Table involved in the query
	Keyspace              string                    // Keyspace to which the table belongs
	UpdateSetValues       []UpdateSetValue          // Values to be updated
	Clauses               []types.Clause            // List of clauses in the update query
	Params                map[string]interface{}    // Parameters for the query
	ParamKeys             []string                  // Column names of the parameters
	PrimaryKeys           []string                  // Primary keys of the table
	Columns               []types.Column            // List of columns in update query
	Values                []interface{}             // values for the update
	RowKey                string                    // Unique rowkey which is required for update operation
	DeleteColumnFamilies  []string                  // List of all collection type of columns
	DeleteColumQualifires []types.Column            // List of all map key deletion in complex update
	ReturnMetadata        []*message.ColumnMetadata // Metadata of all columns of that table in Cassandra format
	VariableMetadata      []*message.ColumnMetadata // Metadata of variable columns for prepared queries in Cassandra format
	TimestampInfo         TimestampInfo
	IfExists              bool
	ComplexOperation      map[string]*ComplexOperation
}

type UpdateSetValue struct {
	Column       string
	Value        string
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

// ProcessRawCollectionsInput holds the parameters for processCollectionColumnsForRawQueries.
type ProcessRawCollectionsInput struct {
	Columns        []types.Column
	Values         []interface{}
	TableName      string
	Translator     *Translator
	KeySpace       string
	PrependColumns []string
}

// ProcessRawCollectionsOutput holds the results from processCollectionColumnsForRawQueries.
type ProcessRawCollectionsOutput struct {
	NewColumns      []types.Column
	NewValues       []interface{}
	DelColumnFamily []string
	DelColumns      []types.Column
	ComplexMeta     map[string]*ComplexOperation
}

// ProcessPrepareCollectionsInput holds the parameters for processCollectionColumnsForPrepareQueries.
type ProcessPrepareCollectionsInput struct {
	ColumnsResponse []types.Column
	Values          []*primitive.Value
	TableName       string
	ProtocolV       primitive.ProtocolVersion
	PrimaryKeys     []string
	Translator      *Translator
	KeySpace        string
	ComplexMeta     map[string]*ComplexOperation
	PrependColumns  []string
}

// ProcessPrepareCollectionsOutput holds the results from processCollectionColumnsForPrepareQueries.
type ProcessPrepareCollectionsOutput struct {
	NewColumns      []types.Column
	NewValues       []interface{}
	Unencrypted     map[string]interface{}
	IndexEnd        int
	DelColumnFamily []string
	DelColumns      []types.Column
	ComplexMeta     map[string]*ComplexOperation
}
