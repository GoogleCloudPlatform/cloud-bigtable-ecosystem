package types

import (
	"cloud.google.com/go/bigtable"
	"github.com/datastax/go-cassandra-native-protocol/message"
)

type TruncateTableStatementMap struct {
	cqlQuery string
	keyspace Keyspace
	table    TableName
}

func (c *TruncateTableStatementMap) InitialValues() map[Parameter]GoValue {
	return nil
}

func (c *TruncateTableStatementMap) Parameters() *QueryParameters {
	return nil
}

func (c *TruncateTableStatementMap) ResponseColumns() []*message.ColumnMetadata {
	return nil
}

func (c *TruncateTableStatementMap) SetBigtablePreparedQuery(s *bigtable.PreparedStatement) {
}

func (c *TruncateTableStatementMap) IsIdempotent() bool {
	return true
}

func (c *TruncateTableStatementMap) CqlQuery() string {
	return c.cqlQuery
}

func (c *TruncateTableStatementMap) BigtableQuery() string {
	return ""
}

func NewTruncateTableStatementMap(keyspace Keyspace, table TableName, cqlQuery string) *TruncateTableStatementMap {
	return &TruncateTableStatementMap{
		keyspace: keyspace,
		table:    table,
		cqlQuery: cqlQuery,
	}
}

func (c *TruncateTableStatementMap) AsBulkMutation() (IBigtableMutation, bool) {
	return nil, false
}

func (c *TruncateTableStatementMap) Keyspace() Keyspace {
	return c.keyspace
}

func (c *TruncateTableStatementMap) Table() TableName {
	return c.table
}

func (c *TruncateTableStatementMap) QueryType() QueryType {
	return QueryTypeTruncate
}

type CreateTablePrimaryKeyConfig struct {
	Name    ColumnName
	KeyType KeyType
}
