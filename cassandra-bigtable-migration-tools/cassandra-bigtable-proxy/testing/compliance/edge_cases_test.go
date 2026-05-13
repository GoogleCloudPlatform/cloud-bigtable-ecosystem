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
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/gocql/gocql"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ensures that various int row keys are handled correctly in all CRUD operations
func TestIntRowKeys(t *testing.T) {
	t.Parallel()
	tests := []struct {
		i32 int32
		i64 int64
	}{
		{i32: 0, i64: 0},
		{i32: 1, i64: 1},
		{i32: -1, i64: -1},
		{i32: math.MinInt32, i64: math.MinInt64},
		{i32: math.MaxInt32, i64: math.MaxInt64},
		{i32: math.MaxInt32, i64: math.MinInt64},
		{i32: math.MinInt32, i64: math.MaxInt64},
		// min int64
		{i32: -1, i64: math.MinInt64},
		{i32: -1, i64: math.MinInt64 + 1},
		{i32: -1, i64: math.MinInt64 + 2},
		{i32: -1, i64: math.MinInt64 + 100},
		{i32: -1, i64: math.MinInt64 + 1000},
		{i32: -1, i64: math.MinInt64 + 10000},
		{i32: -1, i64: math.MinInt64 + 100000},

		// max int64
		{i32: -1, i64: math.MaxInt64},
		{i32: -1, i64: math.MaxInt64 - 1},
		{i32: -1, i64: math.MaxInt64 - 2},
		{i32: -1, i64: math.MaxInt64 - 100},
		{i32: -1, i64: math.MaxInt64 - 1000},
		{i32: -1, i64: math.MaxInt64 - 10000},
		{i32: -1, i64: math.MaxInt64 - 100000},

		// min int32
		{i32: math.MinInt32, i64: 1},
		{i32: math.MinInt32 + 1, i64: 1},
		{i32: math.MinInt32 + 2, i64: 1},
		{i32: math.MinInt32 + 100, i64: 1},
		{i32: math.MinInt32 + 1000, i64: 1},
		{i32: math.MinInt32 + 10000, i64: 1},
		{i32: math.MinInt32 + 100000, i64: 1},

		// max int32
		{i32: math.MaxInt32, i64: 1},
		{i32: math.MaxInt32 - 1, i64: 1},
		{i32: math.MaxInt32 - 2, i64: 1},
		{i32: math.MaxInt32 - 100, i64: 1},
		{i32: math.MaxInt32 - 1000, i64: 1},
		{i32: math.MaxInt32 - 10000, i64: 1},
		{i32: math.MaxInt32 - 100000, i64: 1},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("int keys i32 %d and i64 %d", tc.i32, tc.i64), func(t *testing.T) {
			t.Parallel()
			name := uuid.New().String()
			require.NoError(t, session.Query("insert into bigtabledevinstance.multiple_int_keys (user_id, order_num, name) values (?, ?, ?)", tc.i64, tc.i32, name).Exec())

			var gotUserId int64
			var gotOrderNum int32
			var gotName string
			query := session.Query("select user_id, order_num, name from bigtabledevinstance.multiple_int_keys where user_id=? and order_num=?", tc.i64, tc.i32)
			require.NoError(t, query.Scan(&gotUserId, &gotOrderNum, &gotName))
			assert.Equal(t, name, gotName)
			assert.Equal(t, tc.i64, gotUserId)
			assert.Equal(t, tc.i32, gotOrderNum)

			name = name + "2"
			require.NoError(t, session.Query("update bigtabledevinstance.multiple_int_keys set name=? where user_id=? and order_num=?", name, tc.i64, tc.i32).Exec())

			require.NoError(t, query.Scan(&gotUserId, &gotOrderNum, &gotName))
			assert.Equal(t, name, gotName)
			assert.Equal(t, tc.i64, gotUserId)
			assert.Equal(t, tc.i32, gotOrderNum)

			require.NoError(t, session.Query("delete from bigtabledevinstance.multiple_int_keys where user_id=? and order_num=?", tc.i64, tc.i32).Exec())
			err := query.Scan()
			assert.Equal(t, gocql.ErrNotFound, err, "The record should be deleted")
		})
	}
}

func TestLexicographicOrder(t *testing.T) {
	if testTarget == TestTargetCassandra {
		t.Skip()
		return
	}
	require.NoError(t, session.Query("CREATE TABLE IF NOT EXISTS lex_test_ordered_code (org BIGINT, id INT, username TEXT, row_index INT, PRIMARY KEY (org, id, username))").Exec())
	require.NoError(t, session.Query("TRUNCATE TABLE lex_test_ordered_code").Exec())

	orderedValues := []map[string]interface{}{
		{"org": math.MinInt64, "id": math.MinInt32, "username": ""},
		{"org": math.MinInt64, "id": math.MinInt32 + 1, "username": ""},
		{"org": math.MinInt64, "id": math.MinInt32 + 1, "username": "a"},
		{"org": math.MinInt64, "id": math.MinInt32 + 1, "username": "b"},
		{"org": math.MinInt64 + 1, "id": math.MinInt32, "username": ""},
		{"org": -1000, "id": math.MinInt32, "username": ""},
		{"org": -1, "id": math.MinInt32, "username": ""},
		{"org": 0, "id": math.MinInt32, "username": ""},
		{"org": 1, "id": math.MinInt32, "username": ""},
		{"org": 1000, "id": math.MinInt32, "username": ""},
		{"org": 99999, "id": math.MinInt32, "username": ""},
		{"org": math.MaxInt64 - 1, "id": math.MinInt32, "username": ""},
		{"org": math.MaxInt64, "id": math.MinInt32, "username": ""},
		{"org": math.MaxInt64, "id": math.MinInt32 + 1, "username": ""},
		{"org": math.MaxInt64, "id": -1000, "username": ""},
		{"org": math.MaxInt64, "id": -1, "username": ""},
		{"org": math.MaxInt64, "id": 0, "username": ""},
		{"org": math.MaxInt64, "id": 0, "username": "D"},
		{"org": math.MaxInt64, "id": 0, "username": "a"},
		{"org": math.MaxInt64, "id": 0, "username": "b"},
		{"org": math.MaxInt64, "id": 1, "username": ""},
		{"org": math.MaxInt64, "id": 1000, "username": ""},
		{"org": math.MaxInt64, "id": 99999, "username": ""},
		{"org": math.MaxInt64, "id": math.MaxInt32 - 1, "username": ""},
		{"org": math.MaxInt64, "id": math.MaxInt32, "username": ""},
		{"org": math.MaxInt64, "id": math.MaxInt32, "username": "10a"},
		{"org": math.MaxInt64, "id": math.MaxInt32, "username": "A"},
		{"org": math.MaxInt64, "id": math.MaxInt32, "username": "Aa"},
		{"org": math.MaxInt64, "id": math.MaxInt32, "username": "Z"},
		{"org": math.MaxInt64, "id": math.MaxInt32, "username": "a"},
		{"org": math.MaxInt64, "id": math.MaxInt32, "username": "b"},
		{"org": math.MaxInt64, "id": math.MaxInt32, "username": "c"},
		{"org": math.MaxInt64, "id": math.MaxInt32, "username": "d"},
		{"org": math.MaxInt64, "id": math.MaxInt32, "username": "defghi"},
		{"org": math.MaxInt64, "id": math.MaxInt32, "username": "dz"},
		{"org": math.MaxInt64, "id": math.MaxInt32, "username": "y"},
		{"org": math.MaxInt64, "id": math.MaxInt32, "username": "z"},
	}

	testLexOrder(t, orderedValues, "lex_test_ordered_code")
}

func testPreparedCrudWithQuotes(t *testing.T, nameValue, textValue, updatedTextValue string, age int64) {
	err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, text_col) VALUES (?, ?, ?)`, nameValue, age, textValue).Exec()
	require.NoError(t, err)

	selectQuery := session.Query(`SELECT name, text_col FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`, nameValue, age)
	var name string
	var textCol string
	err = selectQuery.Scan(&name, &textCol)
	require.NoError(t, err)
	assert.Equal(t, nameValue, name)
	assert.Equal(t, textValue, textCol)

	err = session.Query(`UPDATE bigtabledevinstance.user_info SET text_col=? WHERE name = ? AND age = ?`, updatedTextValue, nameValue, age).Exec()
	require.NoError(t, err)

	err = selectQuery.Scan(&name, &textCol)
	require.NoError(t, err)
	assert.Equal(t, nameValue, name)
	assert.Equal(t, updatedTextValue, textCol)

	err = session.Query(`DELETE FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`, nameValue, age).Exec()
	require.NoError(t, err)

	err = selectQuery.Scan(&name, &textCol)
	assert.Equal(t, gocql.ErrNotFound, err)
}

func TestPreparedQueryWithTwoSingleQuotes(t *testing.T) {
	t.Parallel()
	testPreparedCrudWithQuotes(t, "Jame''s?", "don''t", "won''t", 59)
}

func TestPreparedQueryWithOneSingleQuote(t *testing.T) {
	t.Parallel()
	testPreparedCrudWithQuotes(t, "Jame's?", "don't", "won't", 591)
}

func TestUnpreparedCrudTwoSingleQuotes(t *testing.T) {
	t.Parallel()

	_, err := cqlshExec(`INSERT INTO bigtabledevinstance.user_info (name, age, text_col) VALUES ('cqlsh_''person', 80, '25''s')`)
	require.NoError(t, err)

	results, err := cqlshScanToMap(`SELECT name, age, text_col FROM bigtabledevinstance.user_info WHERE name='cqlsh_''person' AND age=80`)
	require.NoError(t, err)
	assert.Equal(t, []map[string]string{{"age": "80", "name": "cqlsh_'person", "text_col": "25's"}}, results)

	_, err = cqlshExec(`UPDATE bigtabledevinstance.user_info SET text_col='don''t' WHERE name='cqlsh_''person' AND age=80`)
	require.NoError(t, err)
	results, err = cqlshScanToMap(`SELECT name, age, text_col FROM bigtabledevinstance.user_info WHERE name='cqlsh_''person' AND age=80`)
	require.NoError(t, err)
	assert.Equal(t, []map[string]string{{"age": "80", "name": "cqlsh_'person", "text_col": "don't"}}, results)

	_, err = cqlshExec(`DELETE FROM bigtabledevinstance.user_info WHERE name='cqlsh_''person' AND age=80`)
	require.NoError(t, err)
	results, err = cqlshScanToMap(`SELECT * FROM bigtabledevinstance.user_info WHERE name='cqlsh_''person' AND age=80`)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestReadUndefinedPrimitives(t *testing.T) {
	t.Parallel()

	// write with no primitive columns
	require.NoError(t, session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, extra_info) VALUES ('undefined_primitives_test', 1, {'foo': 'bar'})`).Exec())

	var code int32
	var credited float64
	var textCol string
	var balance float32
	var isActive bool
	var birthDate time.Time
	var zipCode int64
	// Ensure that scalar values, not written to a row, are still readable and have the correct defaults
	require.NoError(t, session.Query(`SELECT code, credited, text_col, balance, is_active, birth_date, zip_code FROM bigtabledevinstance.user_info WHERE name='undefined_primitives_test' AND age=1`).Scan(
		&code,
		&credited,
		&textCol,
		&balance,
		&isActive,
		&birthDate,
		&zipCode,
	))
	assert.Equal(t, int32(0), code)
	assert.Equal(t, float64(0), credited)
	assert.Equal(t, "", textCol)
	assert.Equal(t, float32(0), balance)
	assert.Equal(t, false, isActive)
	assert.Equal(t, time.Time{}, birthDate)
	assert.Equal(t, int64(0), zipCode)
}
