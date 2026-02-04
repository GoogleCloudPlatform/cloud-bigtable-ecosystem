package types

import (
	"cloud.google.com/go/bigtable"
	"github.com/datastax/go-cassandra-native-protocol/message"
)

type AlterTableStatementMap struct {
	cqlQuery    string
	keyspace    Keyspace
	table       TableName
	IfNotExists bool
	AddColumns  []CreateColumn
	DropColumns []ColumnName
}

func (a *AlterTableStatementMap) InitialValues() map[Parameter]GoValue {
	return nil
}

func (a *AlterTableStatementMap) Parameters() *QueryParameters {
	return nil
}

func (a *AlterTableStatementMap) ResponseColumns() []*message.ColumnMetadata {
	return nil
}

func (a *AlterTableStatementMap) SetBigtablePreparedQuery(s *bigtable.PreparedStatement) {
}

func (a *AlterTableStatementMap) IsIdempotent() bool {
	return false
}

func (a *AlterTableStatementMap) CqlQuery() string {
	return a.cqlQuery
}

func (a *AlterTableStatementMap) BigtableQuery() string {
	return ""
}

func NewAlterTableStatementMap(keyspace Keyspace, table TableName, cqlQuery string, ifNotExists bool, addColumns []CreateColumn, dropColumns []ColumnName) *AlterTableStatementMap {
	return &AlterTableStatementMap{
		keyspace:    keyspace,
		table:       table,
		cqlQuery:    cqlQuery,
		IfNotExists: ifNotExists,
		AddColumns:  addColumns,
		DropColumns: dropColumns,
	}
}

func (a *AlterTableStatementMap) AsBulkMutation() (IBigtableMutation, bool) {
	return nil, false
}

func (a *AlterTableStatementMap) Keyspace() Keyspace {
	return a.keyspace
}

func (a *AlterTableStatementMap) Table() TableName {
	return a.table
}

func (a *AlterTableStatementMap) QueryType() QueryType {
	return QueryTypeAlter
}
