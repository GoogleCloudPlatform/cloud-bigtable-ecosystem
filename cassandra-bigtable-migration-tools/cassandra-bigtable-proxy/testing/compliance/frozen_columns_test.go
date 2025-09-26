package compliance

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// todo frozen upsert via insert
// todo test selecting part of a frozen collection
func TestInsertAndSelectFrozenListInt(t *testing.T) {
	t.Parallel()

	testName := "test_user_list"
	testList := []int{10, 20, 30, 40}

	// Insert data
	err := session.Query(`INSERT INTO bigtabledevinstance.frozen_table (name, list_nums) VALUES (?, ?)`,
		testName, testList).Exec()
	require.NoError(t, err)

	// Retrieve and validate data
	var retrievedList []int
	err = session.Query(`SELECT list_nums FROM bigtabledevinstance.frozen_table WHERE name = ?`, testName).Scan(&retrievedList)
	require.NoError(t, err)

	assert.Equal(t, testList, retrievedList)
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

func TestInsertAndSelectFrozenSetText(t *testing.T) {
	t.Parallel()

	// Setup test data
	testName := "test_user_set"
	// A Go slice is used to represent a Cassandra set
	testSet := []string{"apple", "banana", "cherry"}

	// Insert data
	err := session.Query(`INSERT INTO bigtabledevinstance.frozen_table (name, set_text) VALUES (?, ?)`,
		testName, testSet).Exec()
	require.NoError(t, err)

	// Retrieve and validate data
	var retrievedSet []string
	err = session.Query(`SELECT set_text FROM bigtabledevinstance.frozen_table WHERE name = ?`, testName).Scan(&retrievedSet)
	require.NoError(t, err)
	assert.ElementsMatch(t, testSet, retrievedSet)
}

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
	require.NoError(t, err)

	// Retrieve and validate data
	var (
		retrievedList []int
		retrievedMap  map[string]string
		retrievedSet  []string
	)
	err = session.Query(`SELECT list_nums, map_text, set_text FROM bigtabledevinstance.frozen_table WHERE name = ?`,
		testName).Scan(&retrievedList, &retrievedMap, &retrievedSet)
	require.NoError(t, err)

	// Validate all fields
	assert.Equal(t, testList, retrievedList)
	assert.Equal(t, testMap, retrievedMap)
	assert.ElementsMatch(t, testSet, retrievedSet)
}

// TestFailAppendToFrozenList demonstrates that you cannot append to a frozen list.
func TestFailAppendToFrozenList(t *testing.T) {
	t.Parallel()
	testName := "fail_append_list"

	// Setup: Insert initial record
	err := session.Query(`INSERT INTO bigtabledevinstance.frozen_table (name, list_nums) VALUES (?, ?)`,
		testName, []int{1, 2}).Exec()
	require.NoError(t, err)

	// Attempt to append to the frozen list (this should fail)
	err = session.Query(`UPDATE bigtabledevinstance.frozen_table SET list_nums = list_nums + ? WHERE name = ?`,
		[]int{3}, testName).Exec()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot partially update frozen columns")
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

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot partially update frozen columns")
}

// TestFailUpdateKeyInFrozenMap demonstrates that you cannot update a key in a frozen map.
func TestFailUpdateKeyInFrozenMap(t *testing.T) {
	t.Parallel()
	testName := "fail_update_map_key"

	// Setup: Insert initial record
	err := session.Query(`INSERT INTO bigtabledevinstance.frozen_table (name, map_text) VALUES (?, ?)`,
		testName, map[string]string{"k1": "v1"}).Exec()
	require.NoError(t, err)

	// Attempt to update a single key in the frozen map (this should fail)
	err = session.Query(`UPDATE bigtabledevinstance.frozen_table SET map_text[?] = ? WHERE name = ?`,
		"k2", "v2", testName).Exec()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot partially update frozen columns")
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
	require.NoError(t, err)

	// "Modify" the list by replacing the *entire* value
	newList := []int{10, 20, 30, 40}
	err = session.Query(`UPDATE bigtabledevinstance.frozen_table SET list_nums = ? WHERE name = ?`,
		newList, testName).Exec()

	// This operation should succeed
	require.NoError(t, err)

	// Retrieve and validate the new list
	var retrievedList []int
	err = session.Query(`SELECT list_nums FROM bigtabledevinstance.frozen_table WHERE name = ?`, testName).Scan(&retrievedList)
	require.NoError(t, err)

	assert.Equal(t, newList, retrievedList)
}
