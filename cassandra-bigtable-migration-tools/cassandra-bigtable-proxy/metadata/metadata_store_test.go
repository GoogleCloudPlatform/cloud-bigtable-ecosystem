package metadata

import (
	"cloud.google.com/go/bigtable"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/testing/bt_server_wrapper"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"os"
	"slices"
	"testing"
)

var ks1 = types.Keyspace("ks1")

var createTableStatement = types.NewCreateTableStatementMap(
	ks1,
	"create_table_test",
	"ignored",
	false,
	[]types.CreateColumn{
		{
			Name:     "org",
			Index:    0,
			TypeInfo: types.TypeVarchar,
		},
		{
			Name:     "id",
			Index:    1,
			TypeInfo: types.TypeBigInt,
		},
		{
			Name:     "name",
			Index:    2,
			TypeInfo: types.TypeVarchar,
		},
		{
			Name:     "zipcode",
			Index:    3,
			TypeInfo: types.TypeInt,
		},
	},
	[]types.CreateTablePrimaryKeyConfig{
		{
			Name:    "org",
			KeyType: "partition_key",
		},
		{
			Name:    "id",
			KeyType: "clustering",
		},
	},
	types.OrderedCodeEncoding,
)

var bts *bt_server_wrapper.BigtableTestServer
var bigtableConfig = types.BigtableConfig{
	ProjectID:          "my-project",
	Instances:          map[types.Keyspace]*types.InstanceMapping{ks1: {InstanceId: "bt1", Keyspace: ks1, AppProfileID: "default"}},
	SchemaMappingTable: "schema_mapping",
	Session: &types.Session{
		GrpcChannels: 3,
	},
	DefaultColumnFamily:      "cf1",
	DefaultIntRowKeyEncoding: types.OrderedCodeEncoding,
}

func TestMain(m *testing.M) {
	bts = bt_server_wrapper.NewBigtableTestServer(bigtableConfig)
	bts.SetUp(0)
	defer bts.Close()
	os.Exit(m.Run())
}

func TestCreateTable(t *testing.T) {
	store := NewMetadataStore(zap.NewNop(), bts.Clients(), &bigtableConfig)

	err := store.Initialize(t.Context())
	require.NoError(t, err)

	exists, err := store.tableResourceExists(t.Context(), createTableStatement.Keyspace(), createTableStatement.Table())
	require.NoError(t, err)
	require.False(t, exists)

	response, err := store.CreateTable(t.Context(), createTableStatement)
	require.NoError(t, err)
	assert.Equal(t, "ks1", response.Keyspace)
	assert.Equal(t, primitive.SchemaChangeType("CREATED"), response.ChangeType)

	table, err := store.Schemas().GetTableSchema(createTableStatement.Keyspace(), createTableStatement.Table())
	require.NoError(t, err)

	expectedColumns := map[types.ColumnName]*types.Column{
		"org": {
			Name:         "org",
			CQLType:      types.TypeVarchar,
			IsPrimaryKey: true,
			PkPrecedence: 1,
			KeyType:      "partition_key",
			Metadata: message.ColumnMetadata{
				Keyspace: string(ks1),
				Table:    "create_table_test",
				Name:     "org",
				Index:    2,
				Type:     datatype.Varchar,
			},
		},
		"id": {
			Name:         "id",
			CQLType:      types.TypeBigInt,
			IsPrimaryKey: true,
			PkPrecedence: 2,
			KeyType:      "clustering",
			Metadata: message.ColumnMetadata{
				Keyspace: string(ks1),
				Table:    "create_table_test",
				Name:     "id",
				Index:    0,
				Type:     datatype.Bigint,
			},
		},
		"name": {
			Name:         "name",
			ColumnFamily: "cf1",
			CQLType:      types.TypeVarchar,
			IsPrimaryKey: false,
			PkPrecedence: 0,
			KeyType:      types.KeyTypeRegular,
			Metadata: message.ColumnMetadata{
				Keyspace: string(ks1),
				Table:    "create_table_test",
				Name:     "name",
				Index:    1,
				Type:     datatype.Varchar,
			},
		},
		"zipcode": {
			Name:         "zipcode",
			ColumnFamily: "cf1",
			CQLType:      types.TypeInt,
			IsPrimaryKey: false,
			PkPrecedence: 0,
			KeyType:      types.KeyTypeRegular,
			Metadata: message.ColumnMetadata{
				Keyspace: string(ks1),
				Table:    "create_table_test",
				Name:     "zipcode",
				Index:    3,
				Type:     datatype.Int,
			},
		},
	}
	for name, expectedCol := range expectedColumns {
		got, err := table.GetColumn(name)
		assert.NoError(t, err)
		assert.Equal(t, expectedCol, got)
	}

	exists, err = store.tableResourceExists(t.Context(), createTableStatement.Keyspace(), createTableStatement.Table())
	require.NoError(t, err)
	require.True(t, exists)
}

func TestCanLoadBadTableConfig(t *testing.T) {
	tableName := types.TableName("bad_key_type_table")
	// WARNING: do NOT change this mutation. It captures a real world schema that we need to handle.
	rks, mutations := toMutations(tableName, []map[string][]byte{
		{
			smColColumnName:   []byte("column1"),
			smColColumnType:   []byte("varchar"),
			smColIsCollection: []byte("false"),
			smColIsPrimaryKey: []byte("true"),
			smColKeyType:      []byte("regular"), // wrong key type but PkPrecedence is > 0
			smColPKPrecedence: []byte("1"),
			smColTableName:    []byte(tableName),
		},
		{
			smColColumnName:   []byte("column2"),
			smColColumnType:   []byte("float"),
			smColIsCollection: []byte("false"),
			smColIsPrimaryKey: []byte("false"),
			smColKeyType:      []byte(""), // missing key type
			smColPKPrecedence: []byte("0"),
			smColTableName:    []byte(tableName),
		},
		{
			smColColumnName:   []byte("column3"),
			smColColumnType:   []byte("int"),
			smColIsCollection: []byte("false"),
			smColIsPrimaryKey: []byte("false"),
			smColKeyType:      []byte(""), // missing key type
			smColPKPrecedence: []byte("0"),
			smColTableName:    []byte(tableName),
		},
		{
			smColColumnName:   []byte("column4"),
			smColColumnType:   []byte("double"),
			smColIsCollection: []byte("false"),
			smColIsPrimaryKey: []byte("false"),
			smColKeyType:      []byte(""), // missing key type
			smColPKPrecedence: []byte("0"),
			smColTableName:    []byte(tableName),
		},
		{
			smColColumnName:   []byte("column5"),
			smColColumnType:   []byte("double"),
			smColIsCollection: []byte("false"),
			smColIsPrimaryKey: []byte("false"),
			smColKeyType:      []byte(""), // missing key type
			smColPKPrecedence: []byte("0"),
			smColTableName:    []byte(tableName),
		},
	})

	admin, err := bts.Clients().GetAdmin(ks1)
	require.NoError(t, err)

	client, err := bts.Clients().GetClient(ks1)
	require.NoError(t, err)

	table := client.OpenTable(string(bigtableConfig.SchemaMappingTable))

	_, err = table.ApplyBulk(t.Context(), rks, mutations)
	require.NoError(t, err)

	err = admin.CreateTable(t.Context(), string(tableName))
	require.NoError(t, err)

	store := NewMetadataStore(zap.NewNop(), bts.Clients(), &bigtableConfig)
	err = store.Initialize(t.Context())
	if err != nil {
		return
	}
	require.NoError(t, err)

	allTables := store.schemas.Tables()
	index := slices.IndexFunc(allTables, func(config *TableSchema) bool {
		return config.Name == tableName
	})
	require.NotEqual(t, -1, index)
	tc := allTables[index]
	assert.Equal(t, tableName, tc.Name)
	assert.Equal(t, ks1, tc.Keyspace)
	assert.Equal(t, []types.ColumnName{"column1"}, tc.GetPrimaryKeys())
	pkCol, err := tc.GetColumn("column1")
	require.NoError(t, err)
	assert.Equal(t, types.ColumnName("column1"), pkCol.Name)
	assert.Equal(t, 1, pkCol.PkPrecedence)
	assert.Equal(t, types.KeyTypePartition, pkCol.KeyType)
	assert.Equal(t, true, pkCol.IsPrimaryKey)
}

func toMutations(tableName types.TableName, data []map[string][]byte) ([]string, []*bigtable.Mutation) {
	var muts []*bigtable.Mutation
	var rowKeys []string
	ts := bigtable.Now()
	for _, d := range data {
		mut := bigtable.NewMutation()
		for k, v := range d {
			mut.Set(schemaMappingTableColumnFamily, k, ts, v)
		}
		muts = append(muts, mut)
		rowKeys = append(rowKeys, string(tableName)+"#"+string(d[smColColumnName]))
	}
	return rowKeys, muts
}

// note: the bttest instance ignores RowKeySchema, so it will always be nil which breaks ReadTableConfigs() ability to infer key encoding.
func TestCreateTableWithEncodeIntRowKeysWithBigEndian(t *testing.T) {
	var err error
	store := NewMetadataStore(zap.NewNop(), bts.Clients(), &bigtableConfig)
	err = store.Initialize(t.Context())
	require.NoError(t, err)

	tableName := types.TableName("big_endian_table")
	createTableStmt := types.NewCreateTableStatementMap(ks1, tableName, "ignored", false, createTableStatement.Columns, createTableStatement.PrimaryKeys, types.BigEndianEncoding)
	_, err = store.CreateTable(t.Context(), createTableStmt)
	require.NoError(t, err)

	lastCreateTableReq := bts.LastCreateTableReq()
	assert.NotNil(t, lastCreateTableReq)
	assert.Equal(t, string(tableName), lastCreateTableReq.TableId)
	field := lastCreateTableReq.Table.RowKeySchema.Fields[1]
	assert.Equal(t, "id", field.FieldName)
	assert.Equal(t, "big_endian_bytes:{}", field.Type.GetInt64Type().GetEncoding().String())
}

func TestCreateTableWithEncodeIntRowKeysWithOrderedCode(t *testing.T) {
	var err error
	store := NewMetadataStore(zap.NewNop(), bts.Clients(), &bigtableConfig)
	err = store.Initialize(t.Context())
	require.NoError(t, err)

	tableName := types.TableName("ordered_code_table")
	createTableStmt := types.NewCreateTableStatementMap(ks1, tableName, "ignored", false, createTableStatement.Columns, createTableStatement.PrimaryKeys, types.OrderedCodeEncoding)
	_, err = store.CreateTable(t.Context(), createTableStmt)
	require.NoError(t, err)

	lastCreateTableReq := bts.LastCreateTableReq()
	assert.NotNil(t, lastCreateTableReq)
	assert.Equal(t, string(tableName), lastCreateTableReq.TableId)
	field := lastCreateTableReq.Table.RowKeySchema.Fields[1]
	assert.Equal(t, "id", field.FieldName)
	assert.Equal(t, "ordered_code_bytes:{}", field.Type.GetInt64Type().GetEncoding().String())
}

func TestAlterTable(t *testing.T) {
	var err error
	store := NewMetadataStore(zap.NewNop(), bts.Clients(), &bigtableConfig)
	err = store.Initialize(t.Context())
	require.NoError(t, err)

	tableName := types.TableName("alter_table_test")
	exists, err := store.tableResourceExists(t.Context(), ks1, tableName)
	require.NoError(t, err)
	require.False(t, exists)

	createTable := types.NewCreateTableStatementMap(createTableStatement.Keyspace(), tableName, "ignored", false, createTableStatement.Columns, createTableStatement.PrimaryKeys, createTableStatement.IntRowKeyEncoding)
	_, err = store.CreateTable(t.Context(), createTable)
	require.NoError(t, err)

	addColumns := []types.CreateColumn{
		{
			Name:     "zodiac",
			TypeInfo: types.TypeVarchar,
		},
	}

	alter := types.NewAlterTableStatementMap(createTableStatement.Keyspace(), tableName, "ignored", false, addColumns, []types.ColumnName{
		"zipcode",
	})
	response, err := store.AlterTable(t.Context(), alter)
	require.NoError(t, err)
	assert.Equal(t, primitive.SchemaChangeType("UPDATED"), response.ChangeType)

	require.NoError(t, err)
	table, err := store.Schemas().GetTableSchema(alter.Keyspace(), alter.Table())
	require.NoError(t, err)

	expectedColumns := map[types.ColumnName]*types.Column{
		"org": {
			Name:         "org",
			CQLType:      types.TypeVarchar,
			IsPrimaryKey: true,
			PkPrecedence: 1,
			KeyType:      "partition_key",
			Metadata: message.ColumnMetadata{
				Keyspace: string(ks1),
				Table:    string(tableName),
				Name:     "org",
				Index:    2,
				Type:     datatype.Varchar,
			},
		},
		"id": {
			Name:         "id",
			CQLType:      types.TypeBigInt,
			IsPrimaryKey: true,
			PkPrecedence: 2,
			KeyType:      "clustering",
			Metadata: message.ColumnMetadata{
				Keyspace: string(ks1),
				Table:    string(tableName),
				Name:     "id",
				Index:    0,
				Type:     datatype.Bigint,
			},
		},
		"name": {
			Name:         "name",
			ColumnFamily: "cf1",
			CQLType:      types.TypeVarchar,
			IsPrimaryKey: false,
			PkPrecedence: 0,
			KeyType:      types.KeyTypeRegular,
			Metadata: message.ColumnMetadata{
				Keyspace: string(ks1),
				Table:    string(tableName),
				Name:     "name",
				Index:    1,
				Type:     datatype.Varchar,
			},
		},
		"zodiac": {
			Name:         "zodiac",
			ColumnFamily: "cf1",
			CQLType:      types.TypeVarchar,
			IsPrimaryKey: false,
			PkPrecedence: 0,
			KeyType:      types.KeyTypeRegular,
			Metadata: message.ColumnMetadata{
				Keyspace: string(ks1),
				Table:    string(tableName),
				Name:     "zodiac",
				Index:    3,
				Type:     datatype.Varchar,
			},
		},
	}
	for name, expectedCol := range expectedColumns {
		got, err := table.GetColumn(name)
		assert.NoError(t, err)
		assert.Equal(t, expectedCol, got)
	}
}
