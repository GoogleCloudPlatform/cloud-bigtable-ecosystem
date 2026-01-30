package types

import (
	"cloud.google.com/go/bigtable"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
)

// SelectedColumn describes a column that was selected as part of a query. It's
// an output of query translating, and is also used for response construction.
type SelectedColumn struct {
	// Sql is the original value of the selected column, including functions, not including alias. e.g. "region" or "count(*)"
	Sql  string
	Func CqlFuncCode
	// placeholders are NOT allowed
	MapKey ColumnQualifier
	// placeholders are NOT allowed
	ListIndex int64
	// ColumnName is the name of the underlying column in a function, or map key
	// access. e.g. the column name of "max(price)" is "price"
	ColumnName ColumnName
	Alias      string
	ResultType CqlDataType
}

func NewSelectedColumn(sql string, columnName ColumnName, alias string, resultType CqlDataType) *SelectedColumn {
	return &SelectedColumn{Sql: sql, ColumnName: columnName, Alias: alias, ResultType: resultType, ListIndex: -1}
}

func NewSelectedColumnListElement(sql string, columnName ColumnName, alias string, resultType CqlDataType, listIndex int64) *SelectedColumn {
	return &SelectedColumn{Sql: sql, Alias: alias, ColumnName: columnName, ResultType: resultType, ListIndex: listIndex}
}
func NewSelectedColumnMapElement(sql string, columnName ColumnName, alias string, resultType CqlDataType, key ColumnQualifier) *SelectedColumn {
	return &SelectedColumn{Sql: sql, Alias: alias, ColumnName: columnName, ResultType: resultType, MapKey: key, ListIndex: -1}
}
func NewSelectedColumnFunction(sql string, columnName ColumnName, alias string, resultType CqlDataType, funcCode CqlFuncCode) *SelectedColumn {
	return &SelectedColumn{Sql: sql, Alias: alias, ColumnName: columnName, ResultType: resultType, Func: funcCode, ListIndex: -1}
}

// PreparedSelectQuery represents the mapping of a select query along with its translation details.
type PreparedSelectQuery struct {
	keyspace             Keyspace      // Keyspace to which the table belongs
	table                TableName     // Table involved in the query
	cqlQuery             string        // Original query string
	TranslatedQuery      string        // btql
	SelectClause         *SelectClause // Translator generated Metadata about the columns involved
	Conditions           []Condition
	Params               IQueryParameters // Parameters for the query
	LimitValue           DynamicValue
	CachedBTPrepare      *bigtable.PreparedStatement // prepared statement object for bigtable
	OrderBy              OrderBy                     // Order by clause details
	GroupByColumns       []string                    // Group by Columns - could be a column name or a column index
	ResultColumnMetadata []*message.ColumnMetadata
}

type SelectClause struct {
	// if IsStar is true, Columns will be nil/empty
	IsStar  bool
	Columns []SelectedColumn
}

func (p *PreparedSelectQuery) IsIdempotent() bool {
	return true
}

func NewPreparedSelectQuery(keyspace Keyspace, table TableName, cqlQuery string, translatedQuery string, selectClause *SelectClause, conditions []Condition, params IQueryParameters, orderBy OrderBy, groupByColumns []string, limitValue DynamicValue, resultColumnMetadata []*message.ColumnMetadata) *PreparedSelectQuery {
	return &PreparedSelectQuery{
		keyspace:             keyspace,
		table:                table,
		cqlQuery:             cqlQuery,
		TranslatedQuery:      translatedQuery,
		SelectClause:         selectClause,
		Conditions:           conditions,
		Params:               params,
		OrderBy:              orderBy,
		GroupByColumns:       groupByColumns,
		ResultColumnMetadata: resultColumnMetadata,
		LimitValue:           limitValue,
	}
}

func (p *PreparedSelectQuery) Parameters() IQueryParameters {
	return p.Params
}

func (p *PreparedSelectQuery) ResponseColumns() []*message.ColumnMetadata {
	return p.ResultColumnMetadata
}

func (p *PreparedSelectQuery) SetBigtablePreparedQuery(s *bigtable.PreparedStatement) {
	p.CachedBTPrepare = s
}

func (p *PreparedSelectQuery) BigtableQuery() string {
	return p.TranslatedQuery
}

func (p *PreparedSelectQuery) QueryType() QueryType {
	return QueryTypeSelect
}

func (p *PreparedSelectQuery) Keyspace() Keyspace {
	return p.keyspace
}

func (p *PreparedSelectQuery) Table() TableName {
	return p.table
}

func (p *PreparedSelectQuery) CqlQuery() string {
	return p.cqlQuery
}

type ExecutableSelectQuery struct {
	keyspace             Keyspace
	table                TableName
	cqlQuery             string
	TranslatedQuery      string
	SelectClause         *SelectClause
	Conditions           []Condition
	CachedBTPrepare      *bigtable.PreparedStatement
	OrderBy              OrderBy
	Limit                DynamicValue
	GroupByColumns       []string
	ResultColumnMetadata []*message.ColumnMetadata
	ProtocolVersion      primitive.ProtocolVersion
	Values               *QueryParameterValues
}

func NewExecutableSelectQuery(query *PreparedSelectQuery, protocolVersion primitive.ProtocolVersion, values *QueryParameterValues) *ExecutableSelectQuery {
	return &ExecutableSelectQuery{
		keyspace:             query.keyspace,
		table:                query.table,
		cqlQuery:             query.cqlQuery,
		TranslatedQuery:      query.TranslatedQuery,
		SelectClause:         query.SelectClause,
		Conditions:           query.Conditions,
		CachedBTPrepare:      query.CachedBTPrepare,
		OrderBy:              query.OrderBy,
		GroupByColumns:       query.GroupByColumns,
		ResultColumnMetadata: query.ResultColumnMetadata,
		ProtocolVersion:      protocolVersion,
		Limit:                query.LimitValue,
		Values:               values,
	}
}

func (b *ExecutableSelectQuery) CqlQuery() string {
	return b.cqlQuery
}

func (b *ExecutableSelectQuery) BigtableQuery() string {
	return b.TranslatedQuery
}

func (b *ExecutableSelectQuery) AsBulkMutation() (IBigtableMutation, bool) {
	return nil, false
}

func (b *ExecutableSelectQuery) QueryType() QueryType {
	return QueryTypeSelect
}

func (b *ExecutableSelectQuery) Keyspace() Keyspace {
	return b.keyspace
}

func (b *ExecutableSelectQuery) Table() TableName {
	return b.table
}
