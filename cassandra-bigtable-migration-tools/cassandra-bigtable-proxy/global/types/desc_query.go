package types

import (
	"cloud.google.com/go/bigtable"
	"github.com/datastax/go-cassandra-native-protocol/message"
)

// DescribeQuery represents a parsed DESCRIBE/DESC statement
// Only one of the fields will be set to true/non-empty depending on the type of DESCRIBE
//   - Keyspaces: true for DESCRIBE KEYSPACES
//   - Tables: true for DESCRIBE TABLES
//   - table: set for DESCRIBE TABLE <keyspace.table>
//   - keyspace: set for DESCRIBE KEYSPACE <keyspace> or DESCRIBE <keyspace>
type DescribeQuery struct {
	cqlQuery string
	desc     IDescribeQueryVariant
}

func (a *DescribeQuery) Desc() IDescribeQueryVariant {
	return a.desc
}

func (a *DescribeQuery) Table() TableName {
	return ""
}

func NewDescribeQuery(cqlQuery string, desc IDescribeQueryVariant) *DescribeQuery {
	return &DescribeQuery{cqlQuery: cqlQuery, desc: desc}
}

func (a *DescribeQuery) InitialValues() map[Parameter]GoValue {
	return nil
}

func (a *DescribeQuery) CqlQuery() string {
	return a.cqlQuery
}

func (a *DescribeQuery) BigtableQuery() string {
	return ""
}

func (a *DescribeQuery) AsBulkMutation() (IBigtableMutation, bool) {
	return nil, false
}

func (a *DescribeQuery) Keyspace() Keyspace {
	return a.desc.Keyspace()
}

func (a *DescribeQuery) QueryType() QueryType {
	return QueryTypeDescribe
}

func (a *DescribeQuery) Parameters() *QueryParameters {
	return nil
}

func (a *DescribeQuery) ResponseColumns() []*message.ColumnMetadata {
	return nil
}

func (a *DescribeQuery) SetBigtablePreparedQuery(*bigtable.PreparedStatement) {
}

func (a *DescribeQuery) IsIdempotent() bool {
	return true
}

type IDescribeQueryVariant interface {
	// forces interface implementation
	isDescQuery()
	Keyspace() Keyspace
}

type DescribeKeyspacesQuery struct {
}

func (d DescribeKeyspacesQuery) isDescQuery() {
}

func (d DescribeKeyspacesQuery) Keyspace() Keyspace {
	return ""
}

func NewDescribeKeyspacesQuery() IDescribeQueryVariant {
	return &DescribeKeyspacesQuery{}
}

type DescribeTablesQuery struct {
}

func (d DescribeTablesQuery) isDescQuery() {
}

func (d DescribeTablesQuery) Keyspace() Keyspace {
	return ""
}

func NewDescribeTablesQuery() IDescribeQueryVariant {
	return &DescribeTablesQuery{}
}

type DescribeTableQuery struct {
	keyspace Keyspace
	table    TableName
}

func (d DescribeTableQuery) Table() TableName {
	return d.table
}

func (d DescribeTableQuery) isDescQuery() {
}

func (d DescribeTableQuery) Keyspace() Keyspace {
	return d.keyspace
}

func NewDescribeTableQuery(keyspace Keyspace, table TableName) IDescribeQueryVariant {
	return &DescribeTableQuery{keyspace: keyspace, table: table}
}

type DescribeKeyspaceQuery struct {
	keyspace Keyspace
}

func (d DescribeKeyspaceQuery) isDescQuery() {
}

func (d DescribeKeyspaceQuery) Keyspace() Keyspace {
	return d.keyspace
}

func NewDescribeKeyspaceQuery(keyspace Keyspace) IDescribeQueryVariant {
	return &DescribeKeyspaceQuery{keyspace: keyspace}
}
