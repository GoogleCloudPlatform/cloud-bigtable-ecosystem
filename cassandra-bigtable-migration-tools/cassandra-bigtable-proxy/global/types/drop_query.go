package types

import (
	"cloud.google.com/go/bigtable"
	"github.com/datastax/go-cassandra-native-protocol/message"
)

type DropTableQuery struct {
	cqlQuery string
	keyspace Keyspace
	table    TableName
	IfExists bool
}

func (d *DropTableQuery) InitialValues() map[Parameter]GoValue {
	return nil
}

func (d *DropTableQuery) Parameters() *QueryParameters {
	return nil
}

func (d *DropTableQuery) ResponseColumns() []*message.ColumnMetadata {
	return nil
}

func (d *DropTableQuery) SetBigtablePreparedQuery(s *bigtable.PreparedStatement) {

}

func (d *DropTableQuery) IsIdempotent() bool {
	return d.IfExists
}

func NewDropTableQuery(keyspace Keyspace, table TableName, ifExists bool) *DropTableQuery {
	return &DropTableQuery{keyspace: keyspace, table: table, IfExists: ifExists}
}

func (d *DropTableQuery) CqlQuery() string {
	return d.cqlQuery
}

func (d *DropTableQuery) BigtableQuery() string {
	return ""
}

func (d *DropTableQuery) AsBulkMutation() (IBigtableMutation, bool) {
	return nil, false
}

func (d *DropTableQuery) Keyspace() Keyspace {
	return d.keyspace
}

func (d *DropTableQuery) Table() TableName {
	return d.table
}

func (d *DropTableQuery) QueryType() QueryType {
	return QueryTypeDrop
}
