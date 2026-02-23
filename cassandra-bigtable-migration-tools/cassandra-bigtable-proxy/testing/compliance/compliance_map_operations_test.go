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

func TestMapOperationAdditionTextText(t *testing.T) {
	t.Parallel()
	// 1. Initialize with an empty map
	err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, extra_info) VALUES (?, ?, ?)`,
		"User_Map_Add1", int64(25), map[string]string{}).Exec()
	require.NoError(t, err)

	// 2. Add key-value pairs
	err = session.Query(`UPDATE bigtabledevinstance.user_info SET extra_info = extra_info + ? WHERE name = ? AND age = ?`,
		map[string]string{"key1": "value1", "key2": "value2"}, "User_Map_Add1", int64(25)).Exec()
	require.NoError(t, err)

	// 3. Verify the result
	var extraInfo map[string]string
	err = session.Query(`SELECT extra_info FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`, "User_Map_Add1", int64(25)).Scan(&extraInfo)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"key1": "value1", "key2": "value2"}, extraInfo)
}
func TestMapOperationAdditionAscii(t *testing.T) {
	t.Parallel()
	// 1. Initialize with an empty map
	err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, map_ascii) VALUES (?, ?, ?)`,
		"map_ascii", int64(25), map[string]string{"key1": "old"}).Exec()
	require.NoError(t, err)

	// 2. Add key-value pairs
	err = session.Query(`UPDATE bigtabledevinstance.user_info SET map_ascii = map_ascii + ? WHERE name = ? AND age = ?`,
		map[string]string{"key1": "value1", "key2": "value2"}, "map_ascii", int64(25)).Exec()
	require.NoError(t, err)

	// 3. Verify the result
	var extraInfo map[string]string
	err = session.Query(`SELECT map_ascii FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`, "map_ascii", int64(25)).Scan(&extraInfo)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"key1": "value1", "key2": "value2"}, extraInfo)
}

func TestAsciiMapWithEmptyString(t *testing.T) {
	t.Parallel()
	err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, map_ascii) VALUES (?, ?, ?)`,
		"map_ascii_empty", int64(25), map[string]string{"": "value1", "key2": ""}).Exec()
	require.NoError(t, err)

	var extraInfo map[string]string
	err = session.Query(`SELECT map_ascii FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`, "map_ascii_empty", int64(25)).Scan(&extraInfo)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"": "value1", "key2": ""}, extraInfo)
}

func TestMapOperationAdditionTimestampText(t *testing.T) {
	t.Parallel()
	// 1. Initialize with an empty map
	err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, ts_text_map) VALUES (?, ?, ?)`,
		"User_Map_TS_Add1", int64(25), map[time.Time]string{}).Exec()
	require.NoError(t, err)

	// 2. Add key-value pairs
	newEvents := map[time.Time]string{
		parseTime(t, "2023-01-01T00:00:00Z"): "Event1",
		parseTime(t, "2023-02-01T00:00:00Z"): "Event2",
	}
	err = session.Query(`UPDATE bigtabledevinstance.user_info SET ts_text_map = ts_text_map + ? WHERE name = ? AND age = ?`,
		newEvents, "User_Map_TS_Add1", int64(25)).Exec()
	require.NoError(t, err)

	// 3. Verify the result
	var tsTextMap map[time.Time]string
	err = session.Query(`SELECT ts_text_map FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`, "User_Map_TS_Add1", int64(25)).Scan(&tsTextMap)
	require.NoError(t, err)
	assert.Equal(t, newEvents, tsTextMap)
}

func TestMapOperationSubtraction(t *testing.T) {
	t.Parallel()
	// 1. Initialize with a populated map
	initialMap := map[string]string{"key1": "value1", "key2": "value2", "key3": "value3"}
	err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, extra_info) VALUES (?, ?, ?)`,
		"User_Map_Sub1", int64(25), initialMap).Exec()
	require.NoError(t, err)

	// 2. Remove keys from the map
	keysToRemove := []string{"key1", "key3"}
	err = session.Query(`UPDATE bigtabledevinstance.user_info SET extra_info = extra_info - ? WHERE name = ? AND age = ?`,
		keysToRemove, "User_Map_Sub1", int64(25)).Exec()
	require.NoError(t, err)

	// 3. Verify the result
	var extraInfo map[string]string
	err = session.Query(`SELECT extra_info FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`, "User_Map_Sub1", int64(25)).Scan(&extraInfo)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"key2": "value2"}, extraInfo)
}

func TestMapOperationUpdate(t *testing.T) {
	t.Parallel()
	// 1. Initialize with a populated map
	initialMap := map[string]int{"score1": 100, "score2": 200}
	err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, map_text_int) VALUES (?, ?, ?)`,
		"User_Map_Update1", int64(25), initialMap).Exec()
	require.NoError(t, err)

	// 2. Update a single value in the map
	err = session.Query(`UPDATE bigtabledevinstance.user_info SET map_text_int['score1'] = ? WHERE name = ? AND age = ?`,
		150, "User_Map_Update1", int64(25)).Exec()
	require.NoError(t, err)

	// 3. Verify the result
	var mapTextInt map[string]int
	err = session.Query(`SELECT map_text_int FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`, "User_Map_Update1", int64(25)).Scan(&mapTextInt)
	require.NoError(t, err)
	assert.Equal(t, map[string]int{"score1": 150, "score2": 200}, mapTextInt)
}

func TestMapOperationDelete(t *testing.T) {
	t.Parallel()
	// 1. Initialize with a populated map
	initialMap := map[string]time.Time{
		"event1": parseTime(t, "2023-01-01T00:00:00Z"),
		"event2": parseTime(t, "2023-02-01T00:00:00Z"),
	}
	err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, map_text_ts) VALUES (?, ?, ?)`,
		"User_Map_Delete1", int64(25), initialMap).Exec()
	require.NoError(t, err)

	// 2. Delete a single key
	err = session.Query(`DELETE map_text_ts['event1'] FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`,
		"User_Map_Delete1", int64(25)).Exec()
	require.NoError(t, err)

	// 3. Verify the result
	var mapTextTs map[string]time.Time
	err = session.Query(`SELECT map_text_ts FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`, "User_Map_Delete1", int64(25)).Scan(&mapTextTs)
	require.NoError(t, err)
	assert.Equal(t, map[string]time.Time{"event2": parseTime(t, "2023-02-01T00:00:00Z")}, mapTextTs)
}

func TestComplexUpdateMapTextText(t *testing.T) {
	t.Parallel()
	// NOTE: The '+' operator in a CQL UPDATE on a map merges the maps. It overwrites existing keys and adds new ones.
	// It does NOT remove keys. This test validates the actual Cassandra behavior.

	// 1. Initialize map
	err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, extra_info) VALUES (?, ?, ?)`,
		"User_Map_Text1", int64(28), map[string]string{"key1": "value1", "key2": "value2"}).Exec()
	require.NoError(t, err)

	// 2. "Update" the map by adding another map
	err = session.Query(`UPDATE bigtabledevinstance.user_info SET extra_info = extra_info + ? WHERE name = ? AND age = ?`,
		map[string]string{"key2": "updated_value", "key3": "value3"}, "User_Map_Text1", int64(28)).Exec()
	require.NoError(t, err)

	// 3. Verify the result
	var extraInfo map[string]string
	err = session.Query(`SELECT extra_info FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`, "User_Map_Text1", int64(28)).Scan(&extraInfo)
	require.NoError(t, err)

	expectedMap := map[string]string{"key1": "value1", "key2": "updated_value", "key3": "value3"}
	assert.Equal(t, expectedMap, extraInfo, "Map merge did not produce the correct result")
}

func TestComplexUpdateMapTextInt(t *testing.T) {
	t.Parallel()
	// NOTE: The '+' operator in a CQL UPDATE on a map merges the maps. It does NOT remove keys.
	// This test validates the actual Cassandra behavior.

	// 1. Initialize map
	initialMap := map[string]int{"science": 85, "math": 75, "english": 65}
	err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, map_text_int) VALUES (?, ?, ?)`,
		"User_Map_Int2", int64(30), initialMap).Exec()
	require.NoError(t, err)

	// 2. Merge another map
	err = session.Query(`UPDATE bigtabledevinstance.user_info SET map_text_int = map_text_int + ? WHERE name = ? AND age = ?`,
		map[string]int{"math": 95, "history": 80}, "User_Map_Int2", int64(30)).Exec()
	require.NoError(t, err)

	// 3. Verify the result
	var mapTextInt map[string]int
	err = session.Query(`SELECT map_text_int FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`, "User_Map_Int2", int64(30)).Scan(&mapTextInt)
	require.NoError(t, err)

	expectedMap := map[string]int{"science": 85, "math": 95, "english": 65, "history": 80}
	assert.Equal(t, expectedMap, mapTextInt, "Map merge did not produce the correct result")
}

func TestMapReads(t *testing.T) {
	t.Parallel()
	// 1. Initialize map
	initialMap := map[string]string{"keyA": "valueA", "keyB": "valueB", "keyC": "valueC", "keyD": "valueD"}
	err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, extra_info) VALUES (?, ?, ?)`,
		"map_read_test", int64(90), initialMap).Exec()
	require.NoError(t, err)

	// 2. Read the entire map
	var extraInfo map[string]string
	err = session.Query(`SELECT extra_info FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`, "map_read_test", int64(90)).Scan(&extraInfo)
	require.NoError(t, err)
	assert.Equal(t, initialMap, extraInfo)

	// 3. Read the entire map with an alias
	var ei map[string]string
	err = session.Query(`SELECT extra_info as ei FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`, "map_read_test", int64(90)).Scan(&ei)
	require.NoError(t, err)
	assert.Equal(t, initialMap, ei)

	// 4. Read a single, existing key
	var valueA string
	err = session.Query(`SELECT extra_info['keyA'] FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`, "map_read_test", int64(90)).Scan(&valueA)
	require.NoError(t, err)
	assert.Equal(t, "valueA", valueA)

	// 5. Read a single, non-existent key (should return null, which scans as a zero-value string)
	var missingValue string
	err = session.Query(`SELECT extra_info['missingKey'] FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`, "map_read_test", int64(90)).Scan(&missingValue)
	require.NoError(t, err)
	assert.Equal(t, "", missingValue)
}

func TestComplexMapKeysSelection(t *testing.T) {
	t.Parallel()
	// 1. Initialize map
	initialMap := map[string]string{"info_key_one": "data_one", "info_key_two": "data_two", "info_key_three": "data_three"}
	err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, extra_info) VALUES (?, ?, ?)`,
		"map_multi_key_test", int64(45), initialMap).Exec()
	require.NoError(t, err)

	// 2. Select multiple keys with aliases
	var newage int64
	var name string
	var keyOneData, keyTwoData string
	err = session.Query(`SELECT age as newage, name, extra_info['info_key_one'] as key_one_data, extra_info['info_key_two'] as key_two_data FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`,
		"map_multi_key_test", int64(45)).Scan(&newage, &name, &keyOneData, &keyTwoData)
	require.NoError(t, err)
	assert.Equal(t, int64(45), newage)
	assert.Equal(t, "map_multi_key_test", name)
	assert.Equal(t, "data_one", keyOneData)
	assert.Equal(t, "data_two", keyTwoData)

	// 3. Select a mix of existing and non-existing keys
	var missingData, existingData string
	err = session.Query(`SELECT extra_info['nonexistent_key'] as missing_data, extra_info['info_key_one'] as existing_data FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`,
		"map_multi_key_test", int64(45)).Scan(&missingData, &existingData)
	require.NoError(t, err)
	assert.Equal(t, "", missingData)
	assert.Equal(t, "data_one", existingData)
}

func TestMapOperationWithContainsKeyClause(t *testing.T) {
	t.Parallel()
	// 1. Initialize record
	err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, map_text_int, extra_info) VALUES (?, ?,?, ?)`,
		"Johnatan", int64(71),
		map[string]int32{
			"test-key-one": 100,
			"test-key-two": 200,
		},
		map[string]string{"info_key_contains_one": "data_one", "info_key_contains_two": "data_two", "info_key_contains_three": "data_three"}).Exec()
	require.NoError(t, err)

	// 2. Test that CONTAINS KEY fails without ALLOW FILTERING
	t.Run("CONTAINS KEY without ALLOW FILTERING", func(t *testing.T) {
		t.Parallel()
		var extraInfo map[string]string
		err := session.Query(`SELECT extra_info FROM bigtabledevinstance.user_info WHERE extra_info CONTAINS KEY ?`, "info_key_contains_one").Scan(&extraInfo)
		if testTarget == TestTargetCassandra {
			require.Error(t, err)
			assert.Contains(t, err.Error(), "ALLOW FILTERING")
		} else {
			require.NoError(t, err)
			assert.Equal(t, map[string]string{
				"info_key_contains_one":   "data_one",
				"info_key_contains_two":   "data_two",
				"info_key_contains_three": "data_three",
			}, extraInfo)
		}
	})

	t.Run("CONTAINS KEY without ALLOW FILTERING", func(t *testing.T) {
		t.Parallel()
		var map_test_int map[string]int32
		err := session.Query(`SELECT map_text_int FROM bigtabledevinstance.user_info WHERE map_text_int CONTAINS KEY ?`, "test-key-one").Scan(&map_test_int)
		if testTarget == TestTargetCassandra {
			require.Error(t, err)
			assert.Contains(t, err.Error(), "ALLOW FILTERING")
		} else {
			assert.Equal(t, map[string]int32{
				"test-key-one": 100,
				"test-key-two": 200,
			}, map_test_int)
		}
	})

	t.Run("CONTAINS KEY adhoc", func(t *testing.T) {
		t.Parallel()
		var map_test_int map[string]int32
		err := session.Query(`SELECT map_text_int FROM bigtabledevinstance.user_info WHERE map_text_int CONTAINS KEY 'test-key-one' ALLOW FILTERING`).Scan(&map_test_int)
		require.NoError(t, err)
		assert.Equal(t, map[string]int32{
			"test-key-one": 100,
			"test-key-two": 200,
		}, map_test_int)
	})

	// 3. Test that CONTAINS KEY succeeds with ALLOW FILTERING
	t.Run("CONTAINS KEY with ALLOW FILTERING", func(t *testing.T) {
		t.Parallel()
		var extraInfo map[string]string
		err := session.Query(`SELECT extra_info FROM bigtabledevinstance.user_info WHERE extra_info CONTAINS KEY ? ALLOW FILTERING`, "info_key_contains_one").Scan(&extraInfo)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{
			"info_key_contains_one":   "data_one",
			"info_key_contains_two":   "data_two",
			"info_key_contains_three": "data_three",
		}, extraInfo)
	})

	// 4. Test that a query for a non-existent key returns no rows
	t.Run("CONTAINS KEY for non-existent key", func(t *testing.T) {
		t.Parallel()
		iter := session.Query(`SELECT extra_info FROM bigtabledevinstance.user_info WHERE extra_info CONTAINS KEY ? ALLOW FILTERING`, "non_existent_key").Iter()
		rowCount := iter.NumRows()
		require.NoError(t, iter.Close())
		assert.Equal(t, 0, rowCount, "Should return no rows for a key that doesn't exist")
	})
}
