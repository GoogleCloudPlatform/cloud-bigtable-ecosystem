/*
 * Copyright (C) 2025 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not
 * use this file except in compliance with the License. You may obtain a copy of
 * the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
 * WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
 * License for the specific language governing permissions and limitations under
 * the License.
 */

package compliance

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnsupportedFunctionInSelectQuery(t *testing.T) {
	t.Parallel()
	query := `SELECT xxxx(name) FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`
	err := session.Query(query, "Carls", int64(45)).Exec()

	require.Error(t, err, "Expected an error for an unsupported function, but got none")
	if testTarget != TestTargetCassandra {
		assert.Contains(t, err.Error(), "unknown function: 'xxxx'", "Error message did not match expected output")
	}
}

func TestSelectAndValidateDataFromTestTable(t *testing.T) {
	t.Parallel()
	// 1. Insert a record with various data types
	birthDate := time.UnixMicro(915148800000) // Corrected timestamp value
	err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, code, credited, balance, is_active, birth_date, zip_code, extra_info, tags) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"Carls", int64(45), 123, 1500.5, float32(500.0), true, birthDate, int64(12345),
		map[string]string{"info_key": "info_value"}, []string{"tag1", "tag2"},
	).Exec()
	require.NoError(t, err, "Failed to insert record")

	// 2. Perform a SELECT query to validate the inserted data
	var name string
	var age int64
	err = session.Query(`SELECT name, age FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`, "Carls", int64(45)).Scan(&name, &age)
	require.NoError(t, err, "Failed to select the inserted record")

	assert.Equal(t, "Carls", name)
	assert.Equal(t, int64(45), age)
}

func TestSelectAllRowsWithoutWhereClause(t *testing.T) {
	t.Parallel()
	// 1. Insert a record to ensure the table is not empty
	err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, code) VALUES (?, ?, ?)`,
		"Arena", int64(30), 999).Exec()
	require.NoError(t, err, "Failed to insert record")

	// 2. Select all rows and count them
	iter := session.Query(`SELECT name, age, code FROM bigtabledevinstance.user_info`).Iter()
	rowCount := iter.NumRows()
	require.NoError(t, iter.Close(), "Failed to close iterator after counting rows")

	// The test ensures that the query runs and returns at least the one record we inserted.
	assert.GreaterOrEqual(t, rowCount, 1, "Expected to retrieve at least one row")
}

func TestValidatingWritetimeFunctionality(t *testing.T) {
	t.Parallel()
	// 1. Insert records with specific timestamps
	tsAlice := int64(1734516831000000)
	tsBob := int64(2683260983500000)

	err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, code) VALUES (?, ?, ?) USING TIMESTAMP ?`,
		"Alice", int64(35), 999, tsAlice).Exec()
	require.NoError(t, err, "Failed to insert Alice's record")

	err = session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, code) VALUES (?, ?, ?) USING TIMESTAMP ?`,
		"Bob", int64(40), 123, tsBob).Exec()
	require.NoError(t, err, "Failed to insert Bob's record")

	// 2. Validate WRITETIME without an alias
	t.Run("writetime without alias", func(t *testing.T) {
		t.Parallel()
		var name string
		var age int64
		var code int
		var writeTime int64
		err := session.Query(`SELECT name, age, code, WRITETIME(code) FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`,
			"Alice", int64(35)).Scan(&name, &age, &code, &writeTime)
		require.NoError(t, err)
		assert.Equal(t, "Alice", name)
		assert.Equal(t, tsAlice, writeTime, "WRITETIME(code) did not match the insertion timestamp")
	})

	// 3. Validate WRITETIME with an alias
	t.Run("writetime with alias", func(t *testing.T) {
		t.Parallel()
		var name string
		var age int64
		var code int
		var codeTimestamp int64
		err := session.Query(`SELECT name, age, code, WRITETIME(code) as code_timestamp FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`,
			"Alice", int64(35)).Scan(&name, &age, &code, &codeTimestamp)
		require.NoError(t, err)
		assert.Equal(t, "Alice", name)
		assert.Equal(t, tsAlice, codeTimestamp, "Aliased WRITETIME(code) did not match")
	})
}

func TestSelectStarWithAllDatatypes(t *testing.T) {
	t.Parallel()
	// 1. Prepare complex data for insertion
	birthDate := time.UnixMicro(1672531200000)
	ts1 := time.UnixMicro(1672531200000).UTC()

	// use a long list to better expose any issues in results order
	textList := []string{"item1", "item2", "item3", "item0", "item9", "item1.0", "item01", "item00", "item"}

	// 2. Insert the comprehensive record
	err := session.Query(`
		INSERT INTO bigtabledevinstance.user_info (name, age, code, credited, balance, is_active, birth_date, zip_code, extra_info, tags, set_int, list_text, map_text_int, ts_text_map) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"TestUserStar", int64(25), 101, 1000.50, float32(500.25), true, birthDate, int64(54321),
		map[string]string{"key1": "value1"}, []string{"tag1", "tag2"}, []int{1, 2}, textList,
		map[string]int{"key1": 1}, map[time.Time]string{ts1: "value1"},
	).Exec()
	require.NoError(t, err, "Failed to insert the comprehensive record")

	// 3. Retrieve the record using SELECT * and validate using MapScan
	iter := session.Query(`SELECT * FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`, "TestUserStar", int64(25)).Iter()
	require.NotNil(t, iter)

	var gotColumns []string
	for _, col := range iter.Columns() {
		gotColumns = append(gotColumns, col.Name)
	}
	assert.Equal(t, []string{
		"age",
		"name",
		"balance",
		"birth_date",
		"code",
		"credited",
		"extra_info",
		"is_active",
		"list_ascii",
		"list_bigint",
		"list_boolean",
		"list_double",
		"list_float",
		"list_int",
		"list_text",
		"list_timestamp",
		"map_ascii",
		"map_text_bigint",
		"map_text_boolean",
		"map_text_double",
		"map_text_float",
		"map_text_int",
		"map_text_ts",
		"set_ascii",
		"set_bigint",
		"set_boolean",
		"set_double",
		"set_float",
		"set_int",
		"set_timestamp",
		"tags",
		"text_col",
		"ts_bigint_map",
		"ts_boolean_map",
		"ts_double_map",
		"ts_float_map",
		"ts_int_map",
		"ts_text_map",
		"ts_ts_map",
		"zip_code",
	}, gotColumns)

	resultMap := make(map[string]interface{})
	if !iter.MapScan(resultMap) {
		t.Fatal("MapScan failed to retrieve any data")
	}
	require.NoError(t, iter.Close(), "Failed to close iterator")

	// 4. Assert all values from the map
	assert.Equal(t, "TestUserStar", resultMap["name"])
	assert.Equal(t, int64(25), resultMap["age"])
	assert.Equal(t, 101, resultMap["code"])
	assert.Equal(t, 1000.50, resultMap["credited"])
	assert.Equal(t, float32(500.25), resultMap["balance"])
	assert.Equal(t, true, resultMap["is_active"])
	assert.Equal(t, birthDate.UTC(), resultMap["birth_date"].(time.Time).UTC())
	assert.Equal(t, int64(54321), resultMap["zip_code"])
	assert.Equal(t, map[string]string{"key1": "value1"}, resultMap["extra_info"])
	assert.ElementsMatch(t, []string{"tag1", "tag2"}, resultMap["tags"])
	assert.ElementsMatch(t, []int{1, 2}, resultMap["set_int"])
	assert.Equal(t, textList, resultMap["list_text"])
	assert.Equal(t, map[string]int{"key1": 1}, resultMap["map_text_int"])
	assert.Equal(t, map[time.Time]string{ts1: "value1"}, resultMap["ts_text_map"])
}

// These tests require ALLOW FILTERING as they don't operate on a primary key.
func TestSelectWithDifferentWhereOperators(t *testing.T) {
	t.Parallel()
	// 1. Insert boundary records
	require.NoError(t, session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, code) VALUES (?, ?, ?)`, "Della", int64(1), 987).Exec())
	require.NoError(t, session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, code) VALUES (?, ?, ?)`, "Andre", int64(2), 987).Exec())
	require.NoError(t, session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, code) VALUES (?, ?, ?)`, "Simon", int64(99999), 987).Exec())
	require.NoError(t, session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, code) VALUES (?, ?, ?)`, "Nivi", int64(99980), 987).Exec())

	// 2. Test >= operator
	iterGtEq := session.Query(`SELECT name FROM bigtabledevinstance.user_info WHERE age >= ? AND code=? ALLOW FILTERING`, 99980, 987).Iter()
	namesGtEq, err := iterGtEq.SliceMap()
	require.NoError(t, err)
	assert.ElementsMatch(t, []map[string]interface{}{{"name": "Simon"}, {"name": "Nivi"}}, namesGtEq)

	// 3. Test > operator
	iterGt := session.Query(`SELECT name FROM bigtabledevinstance.user_info WHERE age > ? AND code=? ALLOW FILTERING`, 99980, 987).Iter()
	namesGt, err := iterGt.SliceMap()
	require.NoError(t, err)
	assert.ElementsMatch(t, []map[string]interface{}{{"name": "Simon"}}, namesGt)

	// 4. Test <= operator
	iterLtEq := session.Query(`SELECT name FROM bigtabledevinstance.user_info WHERE age <= ? AND code=? ALLOW FILTERING`, 2, 987).Iter()
	namesLtEq, err := iterLtEq.SliceMap()
	require.NoError(t, err)
	assert.ElementsMatch(t, []map[string]interface{}{{"name": "Della"}, {"name": "Andre"}}, namesLtEq)

	// 5. Test < operator
	iterLt := session.Query(`SELECT name FROM bigtabledevinstance.user_info WHERE age < ? AND code=? ALLOW FILTERING`, 2, 987).Iter()
	namesLt, err := iterLt.SliceMap()
	require.NoError(t, err)
	assert.ElementsMatch(t, []map[string]interface{}{{"name": "Della"}}, namesLt)
}

func TestSelectWithBetweenOperator(t *testing.T) {
	t.Parallel()
	require.NoError(t, session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, code) VALUES (?, ?, ?)`, "Bob", int64(41220), 980).Exec())
	require.NoError(t, session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, code) VALUES (?, ?, ?)`, "Jack", int64(41230), 980).Exec())
	iter := session.Query(`SELECT name FROM bigtabledevinstance.user_info WHERE age BETWEEN ? AND ?`, int64(41225), int64(41235)).Iter()
	namesGtEq, err := iter.SliceMap()

	if testTarget == TestTargetCassandra {
		require.Error(t, err, "Expected an error for BETWEEN operator, but got none")
		return
	}
	assert.ElementsMatch(t, []map[string]interface{}{{"name": "Jack"}}, namesGtEq)
}

func TestSelectWithLikeOperator(t *testing.T) {
	t.Parallel()
	// 1. Insert test data
	require.NoError(t, session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, code) VALUES (?, ?, ?)`, "Silver Hunter", int64(1300), 988).Exec())

	// 2. Test that LIKE fails without a proper index
	iter := session.Query(`SELECT name, age FROM bigtabledevinstance.user_info WHERE name LIKE ?`, "Silver H%").Iter()
	results, err := iter.SliceMap()

	if testTarget == TestTargetCassandra {
		require.Error(t, err, "Expected an error for LIKE on a non-indexed column, but got none")
		assert.Contains(t, err.Error(), "LIKE restriction is only supported on properly indexed columns", "Error message did not match expected output")
	} else {
		assert.ElementsMatch(t, []map[string]interface{}{{"name": "Silver Hunter", "age": int64(1300)}}, results)
	}
}

func TestSelectToTimestampNow(t *testing.T) {
	t.Parallel()

	t1 := time.Date(2025, 12, 2, 9, 32, 12, 0, time.UTC)
	t2 := t1.Add(time.Second)
	require.NoError(t, session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, birth_date) VALUES (?, ?, ?)`, "select_now", int64(1), t1).Exec())
	require.NoError(t, session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, birth_date) VALUES (?, ?, ?)`, "select_now", int64(2), t2).Exec())

	var gotTime time.Time
	require.NoError(t, session.Query(`SELECT birth_date FROM bigtabledevinstance.user_info WHERE name = 'select_now' AND birth_date='2025-12-02 09:32:12'`).Scan(&gotTime))
	assert.Equal(t, t1, gotTime)

	require.NoError(t, session.Query(`SELECT birth_date FROM bigtabledevinstance.user_info WHERE name = 'select_now' AND birth_date > '2025-12-02 09:32:12'`).Scan(&gotTime))
	assert.Equal(t, t2, gotTime)

	require.NoError(t, session.Query(`SELECT birth_date FROM bigtabledevinstance.user_info WHERE name = 'select_now' AND birth_date < ?`, t2).Scan(&gotTime))
	assert.Equal(t, t1, gotTime)

	require.NoError(t, session.Query(`SELECT birth_date FROM bigtabledevinstance.user_info WHERE name = 'select_now' AND birth_date = ?`, t2).Scan(&gotTime))
	assert.Equal(t, t2, gotTime)

	var count int64
	require.NoError(t, session.Query(`SELECT count(*) FROM bigtabledevinstance.user_info WHERE name = 'select_now' AND birth_date < ?`, t1).Scan(&count))
	assert.Equal(t, int64(0), count)

	require.NoError(t, session.Query(`SELECT count(*) FROM bigtabledevinstance.user_info WHERE name = 'select_now' AND birth_date > ?`, t2).Scan(&count))
	assert.Equal(t, int64(0), count)

	require.NoError(t, session.Query(`SELECT birth_date FROM bigtabledevinstance.user_info WHERE name = 'select_now' AND birth_date >= ?`, t2).Scan(&gotTime))
	assert.Equal(t, t2, gotTime)

	require.NoError(t, session.Query(`SELECT birth_date FROM bigtabledevinstance.user_info WHERE name = 'select_now' AND birth_date <= ?`, t1).Scan(&gotTime))
	assert.Equal(t, t1, gotTime)
}
