package compliance

import (
	"testing"
	"time"

	"github.com/gocql/gocql"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBasicInsertUpdateDeleteValidation(t *testing.T) {
	t.Parallel()
	rowName := uuid.New().String()
	err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, code) VALUES (?, ?, ?)`, rowName, int64(30), 123).Exec()
	require.NoError(t, err, "Failed to insert record")

	var name string
	var age int64
	var code int
	err = session.Query(`SELECT name, age, code FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`, rowName, int64(30)).Scan(&name, &age, &code)
	require.NoError(t, err, "Failed to select newly inserted record")
	assert.Equal(t, rowName, name)
	assert.Equal(t, int64(30), age)
	assert.Equal(t, 123, code)

	err = session.Query(`UPDATE bigtabledevinstance.user_info SET code = ? WHERE name = ? AND age = ?`, 456, rowName, int64(30)).Exec()
	require.NoError(t, err, "Failed to update record")

	err = session.Query(`SELECT code FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`, rowName, int64(30)).Scan(&code)
	require.NoError(t, err, "Failed to select updated record")
	assert.Equal(t, 456, code)

	err = session.Query(`DELETE FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`, rowName, int64(30)).Exec()
	require.NoError(t, err, "Failed to delete record")

	err = session.Query(`SELECT name FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`, rowName, int64(30)).Scan(&name)
	require.Error(t, err, "Expected an error when selecting a deleted record")
	assert.Equal(t, gocql.ErrNotFound, err, "Expected error to be 'not found' after deletion")
}

func TestUpsertOperation(t *testing.T) {
	t.Parallel()
	pkName, pkAge := uuid.New().String(), int64(33)
	err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, code) VALUES (?, ?, ?)`, pkName, pkAge, 123).Exec()
	require.NoError(t, err, "Failed initial insert")

	err = session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, code) VALUES (?, ?, ?)`, pkName, pkAge, 456).Exec()
	require.NoError(t, err, "Failed second insert (upsert)")

	var code int
	err = session.Query(`SELECT code FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`, pkName, pkAge).Scan(&code)
	require.NoError(t, err, "Failed to select upserted record")
	assert.Equal(t, 456, code, "The code should be updated to the value from the second insert")
}

func TestInsertAndValidateCollectionData(t *testing.T) {
	t.Parallel()
	pkName, pkAge := uuid.New().String(), int64(25)
	tags := []string{"tag1", "tag2"}
	extraInfo := map[string]string{"info_key": "info_value"}

	err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, code, tags, extra_info) VALUES (?, ?, ?, ?, ?)`,
		pkName, pkAge, 456, tags, extraInfo).Exec()
	require.NoError(t, err)

	var retrievedTags []string
	var retrievedExtraInfo map[string]string
	err = session.Query(`SELECT tags, extra_info FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`, pkName, pkAge).Scan(&retrievedTags, &retrievedExtraInfo)
	require.NoError(t, err)
	assert.ElementsMatch(t, tags, retrievedTags)
	assert.Equal(t, extraInfo, retrievedExtraInfo)
}

// TestInsertWithTimestamps verifies INSERTs using the USING TIMESTAMP clause.
func TestInsertWithTimestamps(t *testing.T) {
	t.Parallel()
	nowMicro := time.Now().UnixMicro()

	t.Run("Future Timestamp", func(t *testing.T) {
		t.Parallel()
		pkName, pkAge := "Danial", int64(55)
		err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, code) VALUES (?, ?, ?) USING TIMESTAMP ?`,
			pkName, pkAge, 678, nowMicro+1000000).Exec() // 1 second in the future
		require.NoError(t, err)
		var code int
		err = session.Query(`SELECT code FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`, pkName, pkAge).Scan(&code)
		require.NoError(t, err, "Record with future timestamp should be selectable")
		assert.Equal(t, 678, code)
	})

	t.Run("Past Timestamp", func(t *testing.T) {
		t.Parallel()
		pkName, pkAge := uuid.New().String(), int64(34)
		err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, code) VALUES (?, ?, ?) USING TIMESTAMP ?`,
			pkName, pkAge, 678, nowMicro-1000000).Exec() // 1 second in the past
		require.NoError(t, err)
		var code int
		err = session.Query(`SELECT code FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`, pkName, pkAge).Scan(&code)
		require.NoError(t, err, "Record with past timestamp should be selectable")
		assert.Equal(t, 678, code)
	})
}

func TestInsertWithAllSupportedDatatypes(t *testing.T) {
	t.Parallel()
	pkName, pkAge := uuid.New().String(), int64(56)
	birthDate := time.UnixMilli(1734516444000)

	err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, code, credited, balance, is_active, birth_date, zip_code) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		pkName, pkAge, 678, 555.67, float32(23.43), true, birthDate, int64(411057)).Exec()
	require.NoError(t, err, "Failed to insert record with all datatypes")

	var code int
	var credited float64
	err = session.Query(`SELECT code, credited FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`, pkName, pkAge).Scan(&code, &credited)
	require.NoError(t, err)
	assert.Equal(t, 678, code)
	assert.Equal(t, 555.67, credited)
}

func TestInsertWithIfNotExists(t *testing.T) {
	t.Parallel()
	pkName, pkAge := uuid.New().String(), int64(56)

	// ensure row doesn't exist
	err := session.Query("DELETE FROM bigtabledevinstance.user_info WHERE name=? AND age=?", pkName, pkAge).Exec()
	require.NoError(t, err)

	// First insert should be applied
	err = session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, code, credited) VALUES (?, ?, ?, ?) IF NOT EXISTS`,
		pkName, pkAge, 678, 3445.0).Exec()
	require.NoError(t, err)

	// Second insert with the same PK should NOT be applied
	err = session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, code, credited) VALUES (?, ?, ?, ?) IF NOT EXISTS`,
		pkName, pkAge, 999, 8888.0).Exec()
	require.NoError(t, err)

	// Validate that the original data remains
	var credited float64
	err = session.Query(`SELECT credited FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`, pkName, pkAge).Scan(&credited)
	require.NoError(t, err)
	assert.Equal(t, 3445.0, credited, "Data from the first insert should be preserved")
}

func TestInsertWithSpecialCharacters(t *testing.T) {
	t.Parallel()
	t.Run("Special Chars", func(t *testing.T) {
		t.Parallel()
		pkName, pkAge := "@John#Doe!", int64(40)
		err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, code) VALUES (?, ?, ?)`, pkName, pkAge, 505).Exec()
		require.NoError(t, err)
		var name string
		err = session.Query(`SELECT name FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`, pkName, pkAge).Scan(&name)
		require.NoError(t, err)
		assert.Equal(t, pkName, name)
	})
	t.Run("Question Mark", func(t *testing.T) {
		t.Parallel()
		pkName, pkAge := "James?", int64(60)
		err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, code) VALUES (?, ?, ?)`, pkName, pkAge, 30).Exec()
		require.NoError(t, err)
		var name string
		err = session.Query(`SELECT name FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`, pkName, pkAge).Scan(&name)
		require.NoError(t, err)
		assert.Equal(t, pkName, name)
	})
}

func TestNegativeInsertCases(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name          string
		query         string
		params        []interface{}
		expectedError string
	}{
		{"Wrong Keyspace", `INSERT INTO randomkeyspace.user_info (name, age, code) VALUES (?, ?, ?)`, []interface{}{"Smith", int64(36), 45}, "keyspace 'randomkeyspace' does not exist"},
		{"Wrong Table", `INSERT INTO random_table (name, age, code) VALUES (?, ?, ?)`, []interface{}{"Smith", int64(36), 45}, "table random_table does not exist"},
		{"Wrong Column", `INSERT INTO user_info (name, age, random_column) VALUES (?, ?, ?)`, []interface{}{"Smith", int64(36), 123}, "undefined column name random_column in table bigtabledevinstance.user_info"},
		{"Missing PK", `INSERT INTO user_info (name, code, code) VALUES (?, ?, ?)`, []interface{}{"Smith", 724, 45}, "some primary key parts are missing: age"},
		{"Null PK", `INSERT INTO user_info (name, age, code) VALUES (?, ?, ?)`, []interface{}{nil, int64(36), 45}, "error building insert prepare query:failed to convert <nil> to BigInt for key name"},
	}

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
