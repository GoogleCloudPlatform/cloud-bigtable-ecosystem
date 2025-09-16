package compliance

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInsertAndSelectFrozenListInt tests the frozen<list<int>> column.
func TestInsertAndSelectFrozenListInt(t *testing.T) {
	t.Parallel()

	// Setup test data
	testName := "test_user_list"
	testList := []int{10, 20, 30, 40}

	// Insert data
	err := session.Query(`INSERT INTO bigtabledevinstance.frozen_table (name, list_nums) VALUES (?, ?)`,
		testName, testList).Exec()
	require.NoError(t, err, "Failed to insert record with frozen<list<int>>")

	// Retrieve and validate data
	var retrievedList []int
	err = session.Query(`SELECT list_nums FROM bigtabledevinstance.frozen_table WHERE name = ?`, testName).Scan(&retrievedList)
	require.NoError(t, err, "Failed to select record with frozen<list<int>>")

	assert.Equal(t, testList, retrievedList, "Retrieved list does not match inserted list")
}

// TestInsertAndSelectFrozenMapText tests the frozen<map<text, text>> column.
func TestInsertAndSelectFrozenMapText(t *testing.T) {
	t.Parallel()

	// Setup test data
	testName := "test_user_map"
	testMap := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	// Insert data
	err := session.Query(`INSERT INTO bigtabledevinstance.frozen_table (name, map_text) VALUES (?, ?)`,
		testName, testMap).Exec()
	require.NoError(t, err, "Failed to insert record with frozen<map<text, text>>")

	// Retrieve and validate data
	var retrievedMap map[string]string
	err = session.Query(`SELECT map_text FROM bigtabledevinstance.frozen_table WHERE name = ?`, testName).Scan(&retrievedMap)
	require.NoError(t, err, "Failed to select record with frozen<map<text, text>>")

	assert.Equal(t, testMap, retrievedMap, "Retrieved map does not match inserted map")
}

// TestInsertAndSelectFrozenSetText tests the frozen<set<text>> column.
func TestInsertAndSelectFrozenSetText(t *testing.T) {
	t.Parallel()

	// Setup test data
	testName := "test_user_set"
	// A Go slice is used to represent a Cassandra set
	testSet := []string{"apple", "banana", "cherry"}

	// Insert data
	err := session.Query(`INSERT INTO bigtabledevinstance.frozen_table (name, set_text) VALUES (?, ?)`,
		testName, testSet).Exec()
	require.NoError(t, err, "Failed to insert record with frozen<set<text>>")

	// Retrieve and validate data
	var retrievedSet []string
	err = session.Query(`SELECT set_text FROM bigtabledevinstance.frozen_table WHERE name = ?`, testName).Scan(&retrievedSet)
	require.NoError(t, err, "Failed to select record with frozen<set<text>>")

	// IMPORTANT: Sets are unordered. Use ElementsMatch to compare, not Equal.
	assert.ElementsMatch(t, testSet, retrievedSet, "Retrieved set does not match inserted set")
}

// TestInsertAndSelectAllFrozenTypes tests inserting and retrieving all columns at once.
func TestInsertAndSelectAllFrozenTypes(t *testing.T) {
	t.Parallel()

	// Setup test data
	testName := "test_user_all"
	testList := []int{1, 2, 3}
	testMap := map[string]string{"env": "prod", "region": "us-east-1"}
	testSet := []string{"monitoring", "logging"}

	// Insert data
	err := session.Query(`INSERT INTO bigtabledevinstance.frozen_table (name, list_nums, map_text, set_text) VALUES (?, ?, ?, ?)`,
		testName, testList, testMap, testSet).Exec()
	require.NoError(t, err, "Failed to insert record with all frozen types")

	// Retrieve and validate data
	var (
		retrievedList []int
		retrievedMap  map[string]string
		retrievedSet  []string
	)
	err = session.Query(`SELECT list_nums, map_text, set_text FROM bigtabledevinstance.frozen_table WHERE name = ?`,
		testName).Scan(&retrievedList, &retrievedMap, &retrievedSet)
	require.NoError(t, err, "Failed to select record with all frozen types")

	// Validate all fields
	assert.Equal(t, testList, retrievedList, "List data mismatch")
	assert.Equal(t, testMap, retrievedMap, "Map data mismatch")
	assert.ElementsMatch(t, testSet, retrievedSet, "Set data mismatch")
}

// TestFailAppendToFrozenList demonstrates that you cannot append to a frozen list.
func TestFailAppendToFrozenList(t *testing.T) {
	t.Parallel()
	testName := "fail_append_list"

	// Setup: Insert initial record
	err := session.Query(`INSERT INTO bigtabledevinstance.frozen_table (name, list_nums) VALUES (?, ?)`,
		testName, []int{1, 2}).Exec()
	require.NoError(t, err, "Failed to insert initial record")

	// Attempt to append to the frozen list (this should fail)
	err = session.Query(`UPDATE bigtabledevinstance.frozen_table SET list_nums = list_nums + ? WHERE name = ?`,
		[]int{3}, testName).Exec()

	// We expect an error
	require.Error(t, err, "Expected an error when appending to a frozen list")
	// Check for Cassandra's specific error message
	assert.Contains(t, err.Error(), "Invalid operation (APPEND) for frozen list", "Error message did not indicate invalid frozen operation")
}

// TestFailAddToFrozenSet demonstrates that you cannot add elements to a frozen set.
func TestFailAddToFrozenSet(t *testing.T) {
	t.Parallel()
	testName := "fail_add_set"

	// Setup: Insert initial record
	err := session.Query(`INSERT INTO bigtabledevinstance.frozen_table (name, set_text) VALUES (?, ?)`,
		testName, []string{"a", "b"}).Exec()
	require.NoError(t, err, "Failed to insert initial record")

	// Attempt to add to the frozen set (this should fail)
	err = session.Query(`UPDATE bigtabledevinstance.frozen_table SET set_text = set_text + ? WHERE name = ?`,
		[]string{"c"}, testName).Exec()

	require.Error(t, err, "Expected an error when adding to a frozen set")
	assert.Contains(t, err.Error(), "Invalid operation (ADD) for frozen set", "Error message did not indicate invalid frozen operation")
}

// TestFailUpdateKeyInFrozenMap demonstrates that you cannot update a key in a frozen map.
func TestFailUpdateKeyInFrozenMap(t *testing.T) {
	t.Parallel()
	testName := "fail_update_map_key"

	// Setup: Insert initial record
	err := session.Query(`INSERT INTO bigtabledevinstance.frozen_table (name, map_text) VALUES (?, ?)`,
		testName, map[string]string{"k1": "v1"}).Exec()
	require.NoError(t, err, "Failed to insert initial record")

	// Attempt to update a single key in the frozen map (this should fail)
	err = session.Query(`UPDATE bigtabledevinstance.frozen_table SET map_text[?] = ? WHERE name = ?`,
		"k2", "v2", testName).Exec()

	require.Error(t, err, "Expected an error when updating a key in a frozen map")
	assert.Contains(t, err.Error(), "Invalid operation (SET) for frozen map", "Error message did not indicate invalid frozen operation")
}

// TestSuccessReplaceEntireFrozenCollection demonstrates the *correct* way to "update"
// a frozen collection: by replacing it entirely.
func TestSuccessReplaceEntireFrozenCollection(t *testing.T) {
	t.Parallel()
	testName := "success_replace_list"

	// Setup: Insert initial record
	initialList := []int{10, 20}
	err := session.Query(`INSERT INTO bigtabledevinstance.frozen_table (name, list_nums) VALUES (?, ?)`,
		testName, initialList).Exec()
	require.NoError(t, err, "Failed to insert initial record")

	// "Modify" the list by replacing the *entire* value
	newList := []int{10, 20, 30, 40}
	err = session.Query(`UPDATE bigtabledevinstance.frozen_table SET list_nums = ? WHERE name = ?`,
		newList, testName).Exec()

	// This operation should succeed
	require.NoError(t, err, "Failed to replace entire frozen list")

	// Retrieve and validate the new list
	var retrievedList []int
	err = session.Query(`SELECT list_nums FROM bigtabledevinstance.frozen_table WHERE name = ?`, testName).Scan(&retrievedList)
	require.NoError(t, err, "Failed to select replaced list")

	assert.Equal(t, newList, retrievedList, "Retrieved list does not match the new, replaced list")
}
