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

	err = session.Query(`UPDATE bigtabledevinstance.user_info SET code = ? WHERE "name" = ? AND age = ?`, 456, rowName, int64(30)).Exec()
	require.NoError(t, err, "Failed to update record")

	err = session.Query(`SELECT code FROM bigtabledevinstance.user_info WHERE "name" = ? AND age = ?`, rowName, int64(30)).Scan(&code)
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
	err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, code, text_col) VALUES (?, ?, ?, ?)`, pkName, pkAge, 123, "abc").Exec()
	require.NoError(t, err)

	err = session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, code, text_col) VALUES (?, ?, ?, ?)`, pkName, pkAge, 456, "abc").Exec()
	require.NoError(t, err)

	var code int
	var textCol string
	err = session.Query(`SELECT code, text_col FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`, pkName, pkAge).Scan(&code, &textCol)
	require.NoError(t, err)
	assert.Equal(t, 456, code)
	assert.Equal(t, "abc", textCol)
}
func TestEmptyString(t *testing.T) {
	t.Parallel()
	pkName := ""
	pkAge := int64(33)
	err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, text_col) VALUES (?, ?, ?)`, pkName, pkAge, "").Exec()
	require.NoError(t, err)

	var name string
	var textCol string
	err = session.Query(`SELECT name, text_col FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`, pkName, pkAge).Scan(&name, &textCol)
	require.NoError(t, err)
	assert.Equal(t, "", name)
	assert.Equal(t, "", textCol)
}

func TestEmptyStringLiteral(t *testing.T) {
	t.Parallel()
	err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, text_col) VALUES ('', 22, '')`).Exec()
	require.NoError(t, err)

	var name string
	var textCol string
	err = session.Query(`SELECT name, text_col FROM bigtabledevinstance.user_info WHERE name = '' AND age = 22 and text_col=''`).Scan(&name, &textCol)
	require.NoError(t, err)
	assert.Equal(t, "", name)
	assert.Equal(t, "", textCol)
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

func TestAscii(t *testing.T) {
	t.Parallel()

	t.Run("literal", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, session.Query("INSERT INTO all_columns (name, ascii_col) VALUES ('ascii-literal', 'valid-ascii-value')").Exec())

		var got string
		require.NoError(t, session.Query(`SELECT ascii_col FROM all_columns WHERE name ='ascii-literal'`).Scan(&got))
		assert.Equal(t, "valid-ascii-value", got)

		require.NoError(t, session.Query("UPDATE all_columns SET ascii_col='valid-ascii-value-2' WHERE name='ascii-literal'").Exec())

		require.NoError(t, session.Query(`SELECT ascii_col FROM all_columns WHERE name ='ascii-literal'`).Scan(&got))
		assert.Equal(t, "valid-ascii-value-2", got)
	})

	t.Run("placeholder", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, session.Query("INSERT INTO all_columns (name, ascii_col) VALUES ( ?, ?)", "ascii-placeholder", "valid-ascii-value").Exec())

		var got string
		require.NoError(t, session.Query(`SELECT ascii_col FROM all_columns WHERE name = ?`, "ascii-placeholder").Scan(&got))
		assert.Equal(t, "valid-ascii-value", got)

		require.NoError(t, session.Query("UPDATE all_columns SET ascii_col=? WHERE name= ?", "valid-ascii-value-2", "ascii-placeholder").Exec())

		require.NoError(t, session.Query(`SELECT ascii_col FROM all_columns WHERE name = ?`, "ascii-placeholder").Scan(&got))
		assert.Equal(t, "valid-ascii-value-2", got)
	})

	t.Run("primary key placeholder", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, session.Query("INSERT INTO ascii_key (id, measurement) VALUES ( ?, ?)", "ascii-key-placeholder", 1).Exec())

		var got int32
		require.NoError(t, session.Query(`SELECT measurement FROM ascii_key WHERE id = ?`, "ascii-key-placeholder").Scan(&got))
		assert.Equal(t, int32(1), got)

		require.NoError(t, session.Query("UPDATE ascii_key SET measurement=? WHERE id= ?", 2, "ascii-key-placeholder").Exec())

		require.NoError(t, session.Query(`SELECT measurement FROM ascii_key WHERE id = ?`, "ascii-key-placeholder").Scan(&got))
		assert.Equal(t, int32(2), got)
	})

	t.Run("primary key literal", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, session.Query("INSERT INTO ascii_key (id, measurement) VALUES ('ascii-key-literal', 1)").Exec())

		var got int32
		require.NoError(t, session.Query(`SELECT measurement FROM ascii_key WHERE id = 'ascii-key-literal'`).Scan(&got))
		assert.Equal(t, int32(1), got)

		require.NoError(t, session.Query("UPDATE ascii_key SET measurement=2 WHERE id= 'ascii-key-literal'").Exec())

		require.NoError(t, session.Query(`SELECT measurement FROM ascii_key WHERE id = 'ascii-key-literal'`).Scan(&got))
		assert.Equal(t, int32(2), got)
	})

	t.Run("primary key invalid literal", func(t *testing.T) {
		t.Parallel()
		err := session.Query("INSERT INTO ascii_key (id, measurement) VALUES ('éàöñ', 1)").Exec()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "string is not valid ascii")
	})

	t.Run("primary key invalid placeholder", func(t *testing.T) {
		t.Parallel()
		err := session.Query("INSERT INTO ascii_key (id, measurement) VALUES (?, ?)", "éàöñ", 1).Exec()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "string is not valid ascii")
	})

	t.Run("invalid value ascii literal", func(t *testing.T) {
		t.Parallel()

		err := session.Query("INSERT INTO all_columns (name, ascii_col) VALUES ('ascii-invalid', 'éàöñ')").Exec()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "string is not valid ascii")
	})

	t.Run("invalid value ascii placeholder", func(t *testing.T) {
		t.Parallel()

		err := session.Query("INSERT INTO all_columns (name, ascii_col) VALUES ('ascii-invalid', ?)", "éàöñ").Exec()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "string is not valid ascii")
	})
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
		{"Wrong Table", `INSERT INTO random_table (name, age, code) VALUES (?, ?, ?)`, []interface{}{"Smith", int64(36), 45}, "table 'random_table' does not exist"},
		{"Wrong Columns", `INSERT INTO user_info (name, age, random_column) VALUES (?, ?, ?)`, []interface{}{"Smith", int64(36), 123}, "unknown column 'random_column' in table bigtabledevinstance.user_info"},
		{"Missing PK", `INSERT INTO user_info (name, code, code) VALUES (?, ?, ?)`, []interface{}{"Smith", 724, 45}, "missing value for primary key `age`"},
		{"Null PK", `INSERT INTO user_info (name, age, code) VALUES (?, ?, ?)`, []interface{}{nil, int64(36), 45}, "value cannot be null for primary key 'name'"},
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

func TestPreparedTimestampNowAlwaysUsesCurrentTimestamp(t *testing.T) {
	t.Parallel()
	var err error

	// execute the same prepared query twice, with a 1 second delay after the first to ensure ToTimestamp(now()) is respected in prepared queries and not just a static time when the query was first prepared
	insertQuery := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, birth_date) VALUES (?, ?, ToTimestamp(now()))`)
	require.NoError(t, insertQuery.Bind("timestampNowTest", int64(1)).Exec())
	time.Sleep(time.Second) // Pause for 1 second
	require.NoError(t, insertQuery.Bind("timestampNowTest", int64(2)).Exec())

	selectQuery := session.Query(`SELECT birth_date FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`)
	var t1 time.Time
	err = selectQuery.Bind("timestampNowTest", int64(1)).Scan(&t1)
	require.NoError(t, err)

	var t2 time.Time
	err = selectQuery.Bind("timestampNowTest", int64(2)).Scan(&t2)
	require.NoError(t, err)

	assert.Less(t, t1.UnixMilli(), t2.UnixMilli(), "t1 must be less than t2")
	diff := t2.UnixMilli() - t1.UnixMilli()
	assert.GreaterOrEqual(t, diff, int64(1000), "at least one second should have passed")

	// confirm the timestamp is recent - give a generous buffer to reduce flakiness
	assert.Less(t, time.Now().UnixMilli()-t2.UnixMilli(), int64(5000), "t2 should be recent")
}

type TimestampEvent struct {
	region      string
	eventTime   time.Time
	measurement int32
	endTime     time.Time
}

func TestTimestampInKey(t *testing.T) {
	t.Parallel()

	t.Run("base case", func(t *testing.T) {
		t.Parallel()

		input := TimestampEvent{
			region:      "us-east-1",
			eventTime:   time.Now().UTC(),
			measurement: 123,
			endTime:     time.Now().UTC().Add(time.Hour * 3),
		}
		err := session.Query(`INSERT INTO timestamp_key (region, event_time, measurement, end_time) VALUES (?, ?, ?, ?)`,
			input.region, input.eventTime, input.measurement, input.endTime).Exec()

		require.NoError(t, err)

		var got = TimestampEvent{}
		err = session.Query(`
        SELECT region, event_time, measurement, end_time
        FROM timestamp_key
        WHERE region = ? AND event_time = ?`,
			input.region,
			input.eventTime,
		).Scan(
			&got.region,
			&got.eventTime,
			&got.measurement,
			&got.endTime,
		)

		input.eventTime = input.eventTime.Truncate(time.Millisecond)
		input.endTime = input.endTime.Truncate(time.Millisecond)

		require.NoError(t, err)
		assert.Equal(t, input, got)
	})
}

func TestInsertEmptyRow(t *testing.T) {
	t.Parallel()

	pkName := uuid.New().String()
	pkAge := int64(100)

	err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age) VALUES (?, ?)`,
		pkName, pkAge).Exec()
	require.NoError(t, err)

	var name string
	var age int64
	var code *int
	var credited *float64
	var textCol *string
	var tags []string
	var extraInfo map[string]string
	var listText []string
	err = session.Query(`SELECT name, age, code, credited, text_col, tags, extra_info, list_text FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`, pkName, pkAge).
		Scan(&name, &age, &code, &credited, &textCol, &tags, &extraInfo, &listText)
	require.NoError(t, err)

	assert.Equal(t, pkName, name)
	assert.Equal(t, pkAge, age)
	assert.Nil(t, code)
	assert.Nil(t, credited)
	assert.Nil(t, textCol)
	assert.Empty(t, tags)
	assert.Empty(t, extraInfo)
	assert.Empty(t, listText)
}

func TestInsertNullValues(t *testing.T) {
	t.Parallel()

	t.Run("Placeholders", func(t *testing.T) {
		t.Parallel()
		pkName := uuid.New().String()
		pkAge := int64(100)

		// Insert nulls using placeholders
		err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, code, credited, text_col, tags, extra_info, list_text) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			pkName, pkAge, nil, nil, nil, nil, nil, nil).Exec()
		require.NoError(t, err)

		var code *int
		var credited *float64
		var textCol *string
		var tags []string
		var extraInfo map[string]string
		var listText []string

		err = session.Query(`SELECT code, credited, text_col, tags, extra_info, list_text FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`, pkName, pkAge).
			Scan(&code, &credited, &textCol, &tags, &extraInfo, &listText)
		require.NoError(t, err)

		assert.Nil(t, code, "code should be null")
		assert.Nil(t, credited, "credited should be null")
		assert.Nil(t, textCol, "text_col should be null")
		assert.Empty(t, tags, "tags should be empty/null")
		assert.Empty(t, extraInfo, "extra_info should be empty/null")
		assert.Empty(t, listText, "list_text should be empty/null")
	})

	t.Run("Literals", func(t *testing.T) {
		t.Parallel()
		pkName := uuid.New().String()
		pkAge := int64(101)

		// Insert nulls using NULL literal
		err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, code, credited, text_col, tags, extra_info, list_text) VALUES (?, ?, NULL, NULL, NULL, NULL, NULL, NULL)`,
			pkName, pkAge).Exec()
		require.NoError(t, err)

		var code *int
		var credited *float64
		var textCol *string
		var tags []string
		var extraInfo map[string]string
		var listText []string

		err = session.Query(`SELECT code, credited, text_col, tags, extra_info, list_text FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`, pkName, pkAge).
			Scan(&code, &credited, &textCol, &tags, &extraInfo, &listText)
		require.NoError(t, err)

		assert.Nil(t, code, "code should be null")
		assert.Nil(t, credited, "credited should be null")
		assert.Nil(t, textCol, "text_col should be null")
		assert.Empty(t, tags, "tags should be empty/null")
		assert.Empty(t, extraInfo, "extra_info should be empty/null")
		assert.Empty(t, listText, "list_text should be empty/null")
	})

	t.Run("list element", func(t *testing.T) {
		t.Parallel()
		pkName := uuid.New().String()
		pkAge := int64(101)

		err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, list_text) VALUES (?, ?, ['foo', NULL])`, pkName, pkAge).Exec()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "collection items are not allowed to be null")
	})

	t.Run("set element", func(t *testing.T) {
		t.Parallel()
		pkName := uuid.New().String()
		pkAge := int64(102)

		err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, tags) VALUES (?, ?, {'foo', NULL})`, pkName, pkAge).Exec()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "collection items are not allowed to be null")
	})

	t.Run("map elements", func(t *testing.T) {
		t.Parallel()
		pkName := uuid.New().String()
		pkAge := int64(103)

		// null value
		err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, extra_info) VALUES (?, ?, {'k': 'v', 'foo': NULL})`, pkName, pkAge).Exec()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "map values cannot be null")
		// null key
		err = session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, extra_info) VALUES (?, ?, {'k': 'v', NULL: 'foo'})`, pkName, pkAge).Exec()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "map keys cannot be null")
		// null key and value
		err = session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, extra_info) VALUES (?, ?, {'k': 'v', NULL: NULL})`, pkName, pkAge).Exec()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "map keys cannot be null")
	})
}
