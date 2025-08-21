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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLimitAndOrderByOperations validates the behavior of LIMIT and ORDER BY clauses.
func TestLimitAndOrderByOperations(t *testing.T) {
	// 1. Insert a large number of records to create a diverse dataset.
	require.NoError(t, session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, code) VALUES (?, ?, ?)`, "Ram", int64(45), 123).Exec())
	require.NoError(t, session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, code) VALUES (?, ?, ?)`, "Ram", int64(11), 173).Exec())
	require.NoError(t, session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, code) VALUES (?, ?, ?)`, "Ram", int64(98), 1).Exec())
	require.NoError(t, session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, code) VALUES (?, ?, ?)`, "Ram", int64(245), 8433).Exec())
	require.NoError(t, session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, code) VALUES (?, ?, ?)`, "Ram", int64(90), 0).Exec())
	require.NoError(t, session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, code) VALUES (?, ?, ?)`, "Ram", int64(50), 150).Exec())
	require.NoError(t, session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, code) VALUES (?, ?, ?)`, "Ram", int64(40), 140).Exec())
	require.NoError(t, session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, code) VALUES (?, ?, ?)`, "Ram", int64(1001), 743).Exec())
	require.NoError(t, session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, code) VALUES (?, ?, ?)`, "Ram", int64(732), 10213).Exec())
	require.NoError(t, session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, code) VALUES (?, ?, ?)`, "Ram", int64(85), 1193).Exec())
	require.NoError(t, session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, code) VALUES (?, ?, ?)`, "Ram", int64(832), 934).Exec())
	require.NoError(t, session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, code) VALUES (?, ?, ?)`, "Ram", int64(10), 11).Exec())
	require.NoError(t, session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, code) VALUES (?, ?, ?)`, "Abc", int64(10), 2347).Exec())
	require.NoError(t, session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, code) VALUES (?, ?, ?)`, "Xyz", int64(10), 5847).Exec())
	require.NoError(t, session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, code) VALUES (?, ?, ?)`, "Ram", int64(93), 13).Exec())

	// 2. Run validation queries.
	t.Run("ORDER BY clustering key ASC with LIMIT", func(t *testing.T) {
		iter := session.Query(`SELECT name, age FROM bigtabledevinstance.user_info WHERE name = ? ORDER BY age LIMIT ?`, "Ram", 4).Iter()
		results, err := iter.SliceMap()
		require.NoError(t, err)
		// Expect the 4 smallest ages for "Ram"
		expected := []map[string]interface{}{
			{"name": "Ram", "age": int64(10)},
			{"name": "Ram", "age": int64(11)},
			{"name": "Ram", "age": int64(40)},
			{"name": "Ram", "age": int64(45)},
		}
		assert.Equal(t, expected, results)
	})

	t.Run("ORDER BY clustering key DESC with LIMIT", func(t *testing.T) {
		iter := session.Query(`SELECT name, age FROM bigtabledevinstance.user_info WHERE name = ? ORDER BY age DESC LIMIT ?`, "Ram", 4).Iter()
		results, err := iter.SliceMap()
		require.NoError(t, err)
		// Expect the 4 largest ages for "Ram"
		expected := []map[string]interface{}{
			{"name": "Ram", "age": int64(1001)},
			{"name": "Ram", "age": int64(832)},
			{"name": "Ram", "age": int64(732)},
			{"name": "Ram", "age": int64(245)},
		}
		assert.Equal(t, expected, results)
	})

	t.Run("ORDER BY partition key with filtering", func(t *testing.T) {
		// This query requires ALLOW FILTERING because it filters on a clustering key ('age')
		iter := session.Query(`SELECT name, age FROM bigtabledevinstance.user_info WHERE age = ? ORDER BY name LIMIT ? ALLOW FILTERING`, 10, 2).Iter()
		results, err := iter.SliceMap()
		require.NoError(t, err)
		// Expect the first 2 names alphabetically for age 10
		expected := []map[string]interface{}{
			{"name": "Abc", "age": int64(10)},
			{"name": "Ram", "age": int64(10)},
		}
		assert.Equal(t, expected, results)
	})

	t.Run("Invalid LIMIT values", func(t *testing.T) {
		err := session.Query(`SELECT name, age FROM bigtabledevinstance.user_info WHERE age = ? LIMIT ?`, 10, -3).Exec()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "LIMIT must be strictly positive")

		err = session.Query(`SELECT name, age FROM bigtabledevinstance.user_info WHERE age = ? LIMIT ?`, 10, 0).Exec()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "LIMIT must be strictly positive")
	})

	t.Run("Invalid ORDER BY syntax", func(t *testing.T) {
		// ORDER BY a number is not valid
		err := session.Query(`SELECT name, age FROM bigtabledevinstance.user_info WHERE age = ? ORDER BY 1 DESC`, 10).Exec()
		require.Error(t, err)
		if testTarget == TestTargetCassandra {
			assert.Contains(t, err.Error(), "no viable alternative at input '1'")
		} else {
			assert.Contains(t, err.Error(), "Order_by section not have proper values")
		}

		// ORDER BY a non-existent column
		err = session.Query(`SELECT name, age FROM bigtabledevinstance.user_info WHERE age = ? ORDER BY xyz`, 10).Exec()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Undefined column name xyz")
	})
}

// TestComprehensiveGroupByAndOrderBy validates that complex, unsupported aggregate queries fail as expected.
func TestComprehensiveGroupByAndOrderBy(t *testing.T) {
	// 1. Insert test data
	require.NoError(t, session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, code, credited, balance) VALUES (?, ?, ?, ?, ?)`, "CompreOne", int64(81), 100, 1000.0, float32(500.0)).Exec())
	require.NoError(t, session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, code, credited, balance) VALUES (?, ?, ?, ?, ?)`, "CompreTwo", int64(81), 200, 2000.0, float32(1000.0)).Exec())
	require.NoError(t, session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, code, credited, balance) VALUES (?, ?, ?, ?, ?)`, "CompreThree", int64(81), 300, 3000.0, float32(1500.0)).Exec())

	t.Run("ORDER BY aggregate alias", func(t *testing.T) {
		query := `SELECT age, name, SUM(code) AS total_code, AVG(balance) FROM bigtabledevinstance.user_info WHERE age = ? GROUP BY age, name ORDER BY name ASC,total_code DESC LIMIT 2`
		iter := session.Query(query, int64(81)).Iter()
		results, err := iter.SliceMap()
		if testTarget == TestTargetCassandra {
			require.Error(t, err)
			assert.Contains(t, err.Error(), "Undefined column name total_code in table bigtabledevinstance.user_info")
		} else {
			require.NoError(t, err)
			assert.ElementsMatch(t, []map[string]interface{}{
				{
					"age":                 int64(81),
					"name":                "CompreOne",
					"total_code":          100,
					"system.avg(balance)": float32(500.0),
				},
				{
					"age":                 int64(81),
					"name":                "CompreThree",
					"total_code":          300,
					"system.avg(balance)": float32(1500.0),
				},
			}, results)
		}
	})

	t.Run("Complex order by, group by and limit with aliases", func(t *testing.T) {
		query := `SELECT age AS age, name, SUM(code) AS total_code FROM bigtabledevinstance.user_info WHERE age = ? GROUP BY age, name ORDER BY age ASC, total_code DESC LIMIT 2`
		iter := session.Query(query, int64(81)).Iter()
		results, err := iter.SliceMap()
		if testTarget == TestTargetCassandra {
			require.Error(t, err)
			assert.Contains(t, err.Error(), "Undefined column name total_code in table bigtabledevinstance.user_info")
		} else {
			require.NoError(t, err)
			assert.ElementsMatch(t, []map[string]interface{}{
				{
					"age":        int64(81),
					"name":       "CompreThree",
					"total_code": 300,
				},
				{
					"age":        int64(81),
					"name":       "CompreTwo",
					"total_code": 200,
				},
			}, results)
		}
	})
	t.Run("Group by age, name; order by name alias and aggregate alias; limit 2", func(t *testing.T) {
		query := `SELECT age, name AS username, SUM(code) AS total_code, MAX(balance) AS max_balance FROM bigtabledevinstance.user_info WHERE age = ? GROUP BY age, name ORDER BY username ASC, max_balance DESC LIMIT 2`
		iter := session.Query(query, int64(81)).Iter()
		results, err := iter.SliceMap()
		if testTarget == TestTargetCassandra {
			require.Error(t, err)
			assert.Contains(t, err.Error(), "Undefined column name username in table bigtabledevinstance.user_info")
		} else {
			require.NoError(t, err)
			assert.ElementsMatch(t, []map[string]interface{}{
				{
					"age":         int64(81),
					"username":    "CompreOne",
					"total_code":  100,
					"max_balance": float32(500),
				},
				{
					"age":         int64(81),
					"username":    "CompreThree",
					"total_code":  300,
					"max_balance": float32(1500.0),
				},
			}, results)
		}
	})
	t.Run("Group by age, name; order by age asc, name desc; limit 2; count aggregate without AS", func(t *testing.T) {
		query := `SELECT age, name, COUNT(*) FROM bigtabledevinstance.user_info WHERE age = ? GROUP BY age, name ORDER BY age ASC, name DESC LIMIT 2`
		iter := session.Query(query, int64(81)).Iter()
		results, err := iter.SliceMap()
		if testTarget == TestTargetCassandra {
			require.Error(t, err)
			assert.Contains(t, err.Error(), "Order by is currently only supported on the clustered columns of the PRIMARY KEY, got age")
		} else {
			require.NoError(t, err)
			assert.ElementsMatch(t, []map[string]interface{}{
				{
					"age":             int64(81),
					"name":            "CompreThree",
					"system.count(*)": int64(1),
				},
				{
					"age":             int64(81),
					"name":            "CompreTwo",
					"system.count(*)": int64(1),
				},
			}, results)
		}
	})
}
