package types

import (
	"cloud.google.com/go/bigtable"
	"github.com/datastax/go-cassandra-native-protocol/message"
)

// PreparedDeleteQuery represents the mapping of a deleted query along with its translation details.
type PreparedDeleteQuery struct {
	keyspace        Keyspace  // Keyspace to which the table belongs
	table           TableName // Table involved in the query
	IfExists        bool
	cqlQuery        string // Original query string
	params          IQueryParameters
	RowKey          []DynamicValue
	SelectedColumns []SelectedColumn
}

func (p *PreparedDeleteQuery) IsIdempotent() bool {
	return true
}

func (p *PreparedDeleteQuery) QueryType() QueryType {
	return QueryTypeDelete
}

func (p *PreparedDeleteQuery) Parameters() IQueryParameters {
	return p.params
}

func (p *PreparedDeleteQuery) ResponseColumns() []*message.ColumnMetadata {
	return nil
}

func (p *PreparedDeleteQuery) SetBigtablePreparedQuery(s *bigtable.PreparedStatement) {
}

func (p *PreparedDeleteQuery) BigtableQuery() string {
	return ""
}

func NewPreparedDeleteQuery(keyspace Keyspace, table TableName, ifExists bool, cqlQuery string, rowKey []DynamicValue, params IQueryParameters, selectedColumns []SelectedColumn) *PreparedDeleteQuery {
	return &PreparedDeleteQuery{keyspace: keyspace, table: table, cqlQuery: cqlQuery, RowKey: rowKey, params: params, IfExists: ifExists, SelectedColumns: selectedColumns}
}

func (p *PreparedDeleteQuery) Keyspace() Keyspace {
	return p.keyspace
}

func (p *PreparedDeleteQuery) Table() TableName {
	return p.table
}

func (p *PreparedDeleteQuery) CqlQuery() string {
	return p.cqlQuery
}

type BoundDeleteQuery struct {
	cqlQuery string
	keyspace Keyspace
	table    TableName
	rowKey   RowKey
	IfExists bool
	Columns  []BoundSelectColumn
}

func (b *BoundDeleteQuery) CqlQuery() string {
	return b.cqlQuery
}

func (b *BoundDeleteQuery) BigtableQuery() string {
	return ""
}

func NewBoundDeleteQuery(keyspace Keyspace, table TableName, cqlQuery string, rowKey RowKey, ifExists bool, columns []BoundSelectColumn) *BoundDeleteQuery {
	return &BoundDeleteQuery{keyspace: keyspace, table: table, cqlQuery: cqlQuery, rowKey: rowKey, IfExists: ifExists, Columns: columns}
}

func (b *BoundDeleteQuery) AsBulkMutation() (IBigtableMutation, bool) {
	return b, true
}

func (b *BoundDeleteQuery) QueryType() QueryType {
	return QueryTypeDelete
}

func (b *BoundDeleteQuery) Keyspace() Keyspace {
	return b.keyspace
}

func (b *BoundDeleteQuery) Table() TableName {
	return b.table
}

func (b *BoundDeleteQuery) RowKey() RowKey {
	return b.rowKey
}
