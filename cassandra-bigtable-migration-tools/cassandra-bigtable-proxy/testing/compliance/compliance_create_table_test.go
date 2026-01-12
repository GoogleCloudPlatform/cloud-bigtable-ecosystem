package compliance

import (
	"context"
	"fmt"
	"testing"

	"cloud.google.com/go/bigtable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateIfNotExist(t *testing.T) {
	t.Parallel()
	var schemaVersion1 string
	require.NoError(t, session.Query("SELECT schema_version FROM system.local").Scan(&schemaVersion1))
	// creating a random table that definitely doesn't exist should be ok
	table := uniqueTableName("create_table_")
	defer cleanupTable(t, table)
	err := session.Query(fmt.Sprintf("CREATE TABLE %s (id TEXT PRIMARY KEY, name TEXT)", table)).Exec()
	assert.NoError(t, err)

	var schemaVersion2 string
	require.NoError(t, session.Query("SELECT schema_version FROM system.local").Scan(&schemaVersion2))
	assert.NotEqual(t, schemaVersion1, schemaVersion2, "schema version should change after successful DDL operations")

	// confirm system tables are updated
	systemTablesResult := make(map[string]any)
	err = session.Query("SELECT * FROM system_schema.tables WHERE table_name=?", table).MapScan(systemTablesResult)
	require.NoError(t, err)
	assert.Equal(t, systemTablesResult["keyspace_name"], "bigtabledevinstance")
	assert.Equal(t, systemTablesResult["table_name"], table)

	// confirm system columns are updated
	systemColumnsResult := make(map[string]any)
	err = session.Query("SELECT * FROM system_schema.columns WHERE table_name=? AND column_name=?", table, "id").MapScan(systemColumnsResult)
	require.NoError(t, err)
	assert.Equal(t, "bigtabledevinstance", systemColumnsResult["keyspace_name"])
	assert.Equal(t, table, systemColumnsResult["table_name"])
	assert.Equal(t, "id", systemColumnsResult["column_name"])
	assert.Equal(t, int(0), systemColumnsResult["position"])
	assert.Equal(t, "partition_key", systemColumnsResult["kind"])
	assert.Equal(t, "text", systemColumnsResult["type"])

	err = session.Query(fmt.Sprintf("INSERT INTO %s (id, name) VALUES (?, ?)", table), "user1", "larry").Exec()
	assert.NoError(t, err)
	err = session.Query(fmt.Sprintf("CREATE TABLE %s (id TEXT PRIMARY KEY, name TEXT)", table)).Exec()
	assert.Error(t, err)
	err = session.Query(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (id TEXT PRIMARY KEY, name TEXT)", table)).Exec()
	assert.NoError(t, err)
}

func TestCreateWhereBigtableTableExists(t *testing.T) {
	// skip this test because it's testing the behavior of the Proxy when the backing Bigtable table still exists - not relevant for Cassandra
	if testTarget == TestTargetCassandra {
		t.Skip()
		return
	}

	t.Parallel()
	table := uniqueTableName("create_table_")
	defer cleanupTable(t, table)

	ctx := context.Background()
	adminClient, err := bigtable.NewAdminClient(ctx, gcpProjectId, instanceId)
	require.NoError(t, err)

	err = adminClient.CreateTableFromConf(ctx, &bigtable.TableConf{
		TableID: table,
		ColumnFamilies: map[string]bigtable.Family{
			"cf1": {
				GCPolicy: bigtable.MaxVersionsPolicy(1),
			},
		},
		RowKeySchema: &bigtable.StructType{
			Fields:   []bigtable.StructField{{FieldName: "id", FieldType: bigtable.Int64Type{Encoding: bigtable.Int64OrderedCodeBytesEncoding{}}}},
			Encoding: bigtable.StructOrderedCodeBytesEncoding{},
		},
	})
	require.NoError(t, err)

	// table should still be creatable given that it's compatible
	err = session.Query(fmt.Sprintf("CREATE TABLE %s (id int PRIMARY KEY, name TEXT)", table)).Exec()
	require.NoError(t, err)

	// confirm it's usable
	require.NoError(t, session.Query(fmt.Sprintf("INSERT INTO %s (id, name) VALUES (?, ?)", table), 13, "foo").Exec())

	var id int32
	var name string
	require.NoError(t, session.Query(fmt.Sprintf("SELECT id, name FROM %s where id=?", table), int32(13)).Scan(&id, &name))
	assert.Equal(t, int32(13), id)
	assert.Equal(t, "foo", name)
}

func TestNegativeTestCasesForCreateTable(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name          string
		query         string
		expectedError string
		skipCassandra bool
	}{
		{
			name:          "Create table with no primary key",
			query:         "CREATE TABLE fail_no_primary_key (num INT, big_num BIGINT)",
			expectedError: "no primary key found in create table statement",
		},
		{
			name:          "Create table with same name as schema mapping table",
			query:         "CREATE TABLE schema_mapping (num INT PRIMARY KEY, big_num BIGINT)",
			expectedError: "table name cannot be the same as the configured schema mapping table name 'schema_mapping'",
			skipCassandra: true,
		},
		{
			name:          "multiple inline primary keys",
			query:         "CREATE TABLE fail_multiple_inline_pmk (num INT PRIMARY KEY, big_num BIGINT, PRIMARY KEY (num, big_num))",
			expectedError: "cannot specify both primary key clause and inline primary key",
		},
		{
			name:          "Create table with empty keys",
			query:         "CREATE TABLE fail_empty_primary_key (num INT, big_num BIGINT, PRIMARY KEY ())",
			expectedError: "parsing error",
		},
		{
			name:          "Create table no column definition for pmk",
			query:         "CREATE TABLE fail_missing_pmk_col (num INT, big_num BIGINT, PRIMARY KEY (foo))",
			expectedError: "primary key 'foo' has no column definition in create table statement",
		},
		{
			name:          "Create table with invalid key type",
			query:         "CREATE TABLE fail_invalid_pmk_type (k boolean, big_num BIGINT, PRIMARY KEY (k))",
			expectedError: "primary key cannot be of type boolean",
			skipCassandra: true,
		},
		{
			name:          "Create table with invalid column type",
			query:         "CREATE TABLE fail_invalid_col_type (num INT, big_num UUID, PRIMARY KEY (num))",
			expectedError: "column type 'uuid' is not supported",
			skipCassandra: true,
		},
		{
			// not allowed because it would clash with the default column because collection column families are the column name
			name:          "uses default column family as collection column name",
			query:         "CREATE TABLE uses_default_cf (num INT, cf1 map<text,text>, PRIMARY KEY (num))",
			expectedError: "counter and collection type columns cannot be named 'cf1' because it's reserved as the default column family",
			skipCassandra: true,
		},
		{
			// not allowed because it would clash with the default column because collection column families are the column name
			name:          "uses default column family as counter column name",
			query:         "CREATE TABLE uses_default_ctrf (num INT, cf1 counter, PRIMARY KEY (num))",
			expectedError: "counter and collection type columns cannot be named 'cf1' because it's reserved as the default column family",
			skipCassandra: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.skipCassandra && testTarget == TestTargetCassandra {
				t.Skip()
				return
			}
			t.Parallel()
			err := session.Query(tc.query).Exec()
			if testTarget == TestTargetCassandra {
				// we don't care about validating the cassandra error message, just that we got an error
				require.Error(t, err)
				return
			}
			require.Error(t, err, "Expected an error but got none")
			assert.Contains(t, err.Error(), tc.expectedError)
		})
	}
}
