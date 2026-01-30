package types

import (
	"cloud.google.com/go/bigtable"
	"github.com/datastax/go-cassandra-native-protocol/message"
)

type PreparedUpdateQuery struct {
	keyspace       Keyspace
	table          TableName
	IfExists       bool
	cqlQuery       string
	Values         []Assignment
	RowKeys        []DynamicValue
	params         IQueryParameters
	UsingTimestamp DynamicValue
}

func (p *PreparedUpdateQuery) IsIdempotent() bool {
	for _, v := range p.Values {
		if !v.IsIdempotentAssignment() {
			return false
		}
	}
	// note: if we allowed where clauses on non-primary key columns we'd have to check those too
	return true
}

func NewPreparedUpdateQuery(keyspace Keyspace, table TableName, ifExists bool, cqlQuery string, values []Assignment, rowKeys []DynamicValue, params IQueryParameters, usingTimestamp DynamicValue) *PreparedUpdateQuery {
	return &PreparedUpdateQuery{keyspace: keyspace, table: table, IfExists: ifExists, cqlQuery: cqlQuery, Values: values, RowKeys: rowKeys, params: params, UsingTimestamp: usingTimestamp}
}

func (p *PreparedUpdateQuery) BigtableQuery() string {
	return ""
}

func (p *PreparedUpdateQuery) Parameters() IQueryParameters {
	return p.params
}

func (p *PreparedUpdateQuery) ResponseColumns() []*message.ColumnMetadata {
	return nil
}

func (p *PreparedUpdateQuery) SetBigtablePreparedQuery(_ *bigtable.PreparedStatement) {
	// nothing to store
}

func (p *PreparedUpdateQuery) QueryType() QueryType {
	return QueryTypeUpdate
}

func (p *PreparedUpdateQuery) Keyspace() Keyspace {
	return p.keyspace
}

func (p *PreparedUpdateQuery) Table() TableName {
	return p.table
}

func (p *PreparedUpdateQuery) CqlQuery() string {
	return p.cqlQuery
}
