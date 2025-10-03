package compliance

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAlterTable(t *testing.T) {
	t.Parallel()
	table := "ddl_table_" + strings.ReplaceAll(uuid.New().String(), "-", "_")
	t.Logf("running test %s with random table name %s", t.Name(), table)
	err := session.Query(fmt.Sprintf("CREATE TABLE %s (id TEXT PRIMARY KEY, name TEXT)", table)).Exec()
	defer cleanupTable(t, table)
	assert.NoError(t, err)

	insertQuery := session.Query(fmt.Sprintf("INSERT INTO %s (id, name, age) VALUES (?, ?, ?)", table), "abc", "bob", 32)

	err = insertQuery.Exec()
	assert.Error(t, err, "insert should fail because there is no age column")

	err = session.Query(fmt.Sprintf("ALTER TABLE %s ADD age INT", table)).Exec()
	assert.NoError(t, err)

	err = insertQuery.Exec()
	assert.NoError(t, err)

	err = session.Query(fmt.Sprintf("ALTER TABLE %s DROP age", table)).Exec()
	assert.NoError(t, err)

	err = session.Query(fmt.Sprintf("ALTER TABLE %s ADD weight FLOAT", table)).Exec()
	assert.NoError(t, err)

	err = session.Query(fmt.Sprintf("INSERT INTO %s (id, name, weight) VALUES (?, ?, ?)", table), "abc", "bob", float32(190.5)).Exec()
	assert.NoError(t, err)

	err = session.Query(fmt.Sprintf("DROP TABLE  %s", table)).Exec()
	assert.NoError(t, err)
}

func TestNegativeTestCasesForAlterTable(t *testing.T) {
	t.Parallel()
	// This assumes a `session *gocql.Session` is available from a TestMain setup.
	// We also need a base table to run ALTER commands against.
	setupQuery := `
		CREATE TABLE IF NOT EXISTS alter_test_table (
			pk_part_one text,
			pk_part_two int,
			regular_col text,
			PRIMARY KEY (pk_part_one, pk_part_two)
		)
	`
	// Use require.NoError for setup, because if this fails, the rest of the tests are invalid.
	require.NoError(t, session.Query(setupQuery).Exec(), "Setup: failed to create base table for alter tests")

	testCases := []struct {
		name          string
		query         string
		expectedError string
		skipCassandra bool
	}{
		{
			name:          "Add a column that already exists",
			query:         "ALTER TABLE alter_test_table ADD regular_col text",
			expectedError: "already exists",
		},
		{
			name:          "Drop a column that does not exist",
			query:         "ALTER TABLE alter_test_table DROP non_existent_col",
			expectedError: "unknown column",
		},
		{
			name:          "Drop a primary key column",
			query:         "ALTER TABLE alter_test_table DROP pk_part_one",
			expectedError: "cannot drop primary key",
		},
		{
			name:          "Rename a primary key column",
			query:         "ALTER TABLE alter_test_table RENAME pk_part_two TO new_pk_name",
			expectedError: "rename operation in alter table command not supported",
			skipCassandra: true,
		},
		{
			name:          "Alter a table that does not exist",
			query:         "ALTER TABLE non_existent_table ADD some_col text",
			expectedError: "table non_existent_table does not exist",
		},
		{
			name:          "Alter a table in a non-existent keyspace",
			query:         "ALTER TABLE invalid_keyspace.alter_test_table ADD some_col text",
			expectedError: "keyspace 'invalid_keyspace' does not exist",
		},
		{
			name:          "Add a column with an unsupported data type",
			query:         "ALTER TABLE alter_test_table ADD new_col uuid",
			expectedError: "column type 'uuid' is not supported",
			skipCassandra: true,
		},
		{
			name:          "Alter columns are not supported by proxy",
			query:         "ALTER TABLE alter_test_table ALTER regular_col TYPE int",
			expectedError: "alter column type operations are not supported",
			skipCassandra: true,
		},
		{
			name:          "Alter property not supported by proxy",
			query:         "ALTER TABLE alter_test_table WITH int_row_key_encoding='big_endian'",
			expectedError: "table property operations are not supported",
			skipCassandra: true,
		},
		{
			name:          "Alter schema_mapping table not allowed",
			query:         "ALTER TABLE schema_mapping DROP all_your_data",
			expectedError: "cannot alter schema mapping table with configured name 'schema_mapping'",
			skipCassandra: true,
		},
	}

	for _, tc := range testCases {
		if tc.skipCassandra && testTarget == TestTargetCassandra {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := session.Query(tc.query).Exec()
			if testTarget == TestTargetCassandra {
				require.Error(t, err, "Expected an error but got none")
				// we don't care about validating the cassandra error message, just that we got an error
				require.Error(t, err)
				return
			}
			require.Error(t, err, "Expected an error but got none")
			assert.Contains(t, err.Error(), tc.expectedError, "Error message mismatch")
		})
	}
}
