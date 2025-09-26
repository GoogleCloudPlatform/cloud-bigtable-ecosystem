package compliance

import (
	"testing"
	"time"

	"github.com/gocql/gocql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeleteOperationWithRecordValidation(t *testing.T) {
	t.Parallel()
	pkName, pkAge := "Michael", int64(45)

	err := session.Query(`INSERT INTO user_info (name, age, code, credited) VALUES (?, ?, ?, ?)`,
		pkName, pkAge, 987, 5000.0).Exec()
	require.NoError(t, err, "Setup: Failed to insert record")

	// Validate existence before delete
	var code int
	err = session.Query(`SELECT code FROM user_info WHERE name = ? AND age = ?`, pkName, pkAge).Scan(&code)
	require.NoError(t, err, "Setup: Failed to select record for pre-validation")
	assert.Equal(t, 987, code)

	// Perform the delete
	err = session.Query(`DELETE FROM user_info WHERE name = ? AND age = ?`, pkName, pkAge).Exec()
	require.NoError(t, err, "Delete operation failed")

	// Validate non-existence after delete
	err = session.Query(`SELECT code FROM user_info WHERE name = ? AND age = ?`, pkName, pkAge).Scan(&code)
	require.Error(t, err, "Expected an error when selecting a deleted record")
	assert.Equal(t, gocql.ErrNotFound, err, "Expected error to be 'not found' after deletion")
}

// Note: This test will need to be updated once the proxy supports this operation.
func TestDeleteOperationWithTimestampFails(t *testing.T) {
	t.Parallel()
	pkName, pkAge := "Jhon", int64(33)
	require.NoError(t, session.Query(`INSERT INTO user_info (name, age, code) VALUES (?, ?, ?)`, pkName, pkAge, 123).Exec())

	nowMicro := time.Now().UnixMicro()
	err := session.Query(`DELETE FROM user_info USING TIMESTAMP ? WHERE name = ? AND age = ?`,
		nowMicro, pkName, pkAge).Exec()

	if testTarget == TestTargetCassandra {
		require.NoError(t, err)
		return
	}

	require.Error(t, err, "Expected an error for DELETE USING TIMESTAMP")
	assert.Contains(t, err.Error(), "delete using timestamp is not allowed")
}

func TestDeleteSpecificRecordByPrimaryKey(t *testing.T) {
	t.Parallel()
	pkName, pkAge := "John", int64(30)
	require.NoError(t, session.Query(`INSERT INTO user_info (name, age, code) VALUES (?, ?, ?)`, pkName, pkAge, 123).Exec())

	err := session.Query(`DELETE FROM user_info WHERE name = ? AND age = ?`, pkName, pkAge).Exec()
	require.NoError(t, err)

	err = session.Query(`SELECT name FROM user_info WHERE name = ? AND age = ?`, pkName, pkAge).Scan(&pkName)
	require.Error(t, err)
	assert.Equal(t, gocql.ErrNotFound, err)
}

func TestDeleteNonExistentRecord(t *testing.T) {
	t.Parallel()
	pkName, pkAge := "NonExistent", int64(99)
	err := session.Query(`DELETE FROM user_info WHERE name = ? AND age = ?`, pkName, pkAge).Exec()
	require.NoError(t, err, "Deleting a non-existent record should not produce an error")
}

func TestDeleteRecordWithIfExists(t *testing.T) {
	t.Parallel()
	pkName, pkAge := "Emma", int64(28)
	require.NoError(t, session.Query(`INSERT INTO user_info (name, age, code, credited) VALUES (?, ?, ?, ?)`, pkName, pkAge, 112, 2500.0).Exec())

	// Delete existing record
	err := session.Query(`DELETE FROM user_info WHERE name = ? AND age = ? IF EXISTS`, pkName, pkAge).Exec()
	require.NoError(t, err)

	// Verify deletion
	err = session.Query(`SELECT name FROM user_info WHERE name = ? AND age = ?`, pkName, pkAge).Scan(&pkName)
	assert.Equal(t, gocql.ErrNotFound, err)

	// Delete non-existent record
	err = session.Query(`DELETE FROM user_info WHERE name = ? AND age = ? IF EXISTS`, "NonExistentName", int64(99)).Exec()
	require.NoError(t, err)
}

func TestNegativeDeleteCases(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name          string
		query         string
		params        []interface{}
		expectedError string
	}{
		{"With Non-PK Condition", `DELETE FROM user_info WHERE name = ? AND age = ? AND balance = ?`, []interface{}{"Oliver", int64(50), float32(100.0)}, "non PRIMARY KEY columns found in where clause: balance"},
		{"Missing PK Part", `DELETE FROM user_info WHERE name = ? IF EXISTS`, []interface{}{"Michael"}, "some primary key parts are missing: age"},
		{"Condition on Non-PK Only", `DELETE FROM user_info WHERE credited = ? IF EXISTS`, []interface{}{5000.0}, "non PRIMARY KEY columns found in where clause: credited"},
		{"Invalid Data TypeInfo", `DELETE FROM user_info WHERE name = ? AND age = ?`, []interface{}{"Michael", "invalid_age"}, "can not marshal string to bigint"},
		{"Invalid Table Name", `DELETE FROM non_existent_table WHERE name = ? AND age = ?`, []interface{}{"Michael", int64(45)}, "table non_existent_table does not exist"},
		{"Invalid Keyspace", `DELETE FROM invalid_keyspace.user_info WHERE name = ? AND age = ?`, []interface{}{"Michael", int64(45)}, "keyspace invalid_keyspace does not exist"},
	}

	// Insert a record needed for the "With Non-PK Condition" test
	require.NoError(t, session.Query(`INSERT INTO user_info (name, age, balance) VALUES (?, ?, ?)`, "Oliver", int64(50), float32(100.0)).Exec())

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := session.Query(tc.query, tc.params...).Exec()
			require.Error(t, err, "Expected query to fail")
			// we don't care about validating the cassandra error message, just that we got an error
			if testTarget == TestTargetCassandra {
				return
			}
			assert.Contains(t, err.Error(), tc.expectedError)
		})
	}
}
