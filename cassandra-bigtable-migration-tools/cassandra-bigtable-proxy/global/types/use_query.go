package types

import (
	"cloud.google.com/go/bigtable"
	"github.com/datastax/go-cassandra-native-protocol/message"
)

type UseTableStatementMap struct {
	cqlQuery string
	keyspace Keyspace
}

func (u *UseTableStatementMap) InitialValues() map[Parameter]GoValue {
	return nil
}

func (u *UseTableStatementMap) Parameters() *QueryParameters {
	return nil
}

func (u *UseTableStatementMap) ResponseColumns() []*message.ColumnMetadata {
	return nil
}

func (u *UseTableStatementMap) SetBigtablePreparedQuery(s *bigtable.PreparedStatement) {
}

func (u *UseTableStatementMap) IsIdempotent() bool {
	return true
}

func (u *UseTableStatementMap) Keyspace() Keyspace {
	return u.keyspace
}

func (u *UseTableStatementMap) Table() TableName {
	return ""
}

func (u *UseTableStatementMap) AsBulkMutation() (IBigtableMutation, bool) {
	return nil, false
}

func (u *UseTableStatementMap) CqlQuery() string {
	return u.cqlQuery
}

func (u *UseTableStatementMap) BigtableQuery() string {
	return ""
}

func (u *UseTableStatementMap) QueryType() QueryType {
	return QueryTypeUse
}

func NewUseTableStatementMap(keyspace Keyspace, cqlQuery string) *UseTableStatementMap {
	return &UseTableStatementMap{
		keyspace: keyspace,
		cqlQuery: cqlQuery,
	}
}
