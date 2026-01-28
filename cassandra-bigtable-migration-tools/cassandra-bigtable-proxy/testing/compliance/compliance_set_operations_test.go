package compliance

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetOperationAdditionSetText(t *testing.T) {
	t.Parallel()
	// 1. Initialize a user with a starting set
	err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, tags) VALUES (?, ?, ?)`,
		"User_Set_Add1", int64(25), []string{"tag0"}).Exec()
	require.NoError(t, err, "Failed to insert initial record")

	// 2. Add new elements to the set
	err = session.Query(`UPDATE bigtabledevinstance.user_info SET tags = tags + ? WHERE name = ? AND age = ?`,
		[]string{"tag1", "tag2"}, "User_Set_Add1", int64(25)).Exec()
	require.NoError(t, err, "Failed to add elements to the set")

	// 3. Verify the final state of the set
	var tags []string
	err = session.Query(`SELECT tags FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`,
		"User_Set_Add1", int64(25)).Scan(&tags)
	require.NoError(t, err, "Failed to select the updated record")

	assert.ElementsMatch(t, []string{"tag0", "tag1", "tag2"}, tags, "The set did not contain all expected elements")
}
func TestSetOperationAdditionSetAscii(t *testing.T) {
	t.Parallel()
	// 1. Initialize a user with a starting set
	err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, set_ascii) VALUES (?, ?, ?)`,
		"set_ascii_row", int64(25), []string{"tag0"}).Exec()
	require.NoError(t, err, "Failed to insert initial record")

	// 2. Add new elements to the set
	err = session.Query(`UPDATE bigtabledevinstance.user_info SET set_ascii = set_ascii + ? WHERE name = ? AND age = ?`,
		[]string{"tag1", "tag2"}, "set_ascii_row", int64(25)).Exec()
	require.NoError(t, err, "Failed to add elements to the set")

	// 3. Verify the final state of the set
	var set_ascii []string
	err = session.Query(`SELECT set_ascii FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`,
		"set_ascii_row", int64(25)).Scan(&set_ascii)
	require.NoError(t, err, "Failed to select the updated record")

	assert.ElementsMatch(t, []string{"tag0", "tag1", "tag2"}, set_ascii, "The set did not contain all expected elements")
}

func TestSetOperationSubtractionSetInt(t *testing.T) {
	t.Parallel()
	// 1. Initialize a user with a set of integers
	err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, set_int) VALUES (?, ?, ?)`,
		"User_Set_Sub1", int64(25), []int{10, 20, 30}).Exec()
	require.NoError(t, err, "Failed to insert initial record")

	// 2. Remove elements from the set
	err = session.Query(`UPDATE bigtabledevinstance.user_info SET set_int = set_int - ? WHERE name = ? AND age = ?`,
		[]int{10, 30}, "User_Set_Sub1", int64(25)).Exec()
	require.NoError(t, err, "Failed to remove elements from the set")

	// 3. Verify the final state of the set
	var setInt []int
	err = session.Query(`SELECT set_int FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`,
		"User_Set_Sub1", int64(25)).Scan(&setInt)
	require.NoError(t, err, "Failed to select the updated record")

	assert.Equal(t, []int{20}, setInt, "The set did not contain the correct remaining element")
}

func TestInsertElementsIntoSetText(t *testing.T) {
	t.Parallel()
	// 1. Initialize user with a set
	err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, tags) VALUES (?, ?, ?)`,
		"User_Set1", int64(25), []string{"tag0"}).Exec()
	require.NoError(t, err, "Failed to insert initial record")

	var tags []string
	err = session.Query(`SELECT tags FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`,
		"User_Set1", int64(25)).Scan(&tags)
	require.NoError(t, err, "Failed to select the initial record")
	assert.ElementsMatch(t, []string{"tag0"}, tags, "The set did not contain all expected tags")

	// 2. Add more elements
	err = session.Query(`UPDATE bigtabledevinstance.user_info SET tags = tags + ? WHERE name = ? AND age = ?`,
		[]string{"tag1", "tag2"}, "User_Set1", int64(25)).Exec()
	require.NoError(t, err, "Failed to add elements to the set")

	// 3. Verify the result
	err = session.Query(`SELECT tags FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`,
		"User_Set1", int64(25)).Scan(&tags)
	require.NoError(t, err, "Failed to select the updated record")

	assert.ElementsMatch(t, []string{"tag0", "tag1", "tag2"}, tags, "The set did not contain all expected tags")
}

func TestValidateUniqueConstraintInSetInt(t *testing.T) {
	t.Parallel()
	// 1. Initialize user with a set containing a single integer
	err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, set_int) VALUES (?, ?, ?)`,
		"User_Set2", int64(30), []int{42}).Exec()
	require.NoError(t, err, "Failed to insert initial record")

	// 2. Attempt to add duplicate values to the set
	err = session.Query(`UPDATE bigtabledevinstance.user_info SET set_int = set_int + ? WHERE name = ? AND age = ?`,
		[]int{42, 42}, "User_Set2", int64(30)).Exec()
	require.NoError(t, err, "Failed to update set with duplicate values")

	// 3. Verify the set still contains only one instance of the value
	var setInt []int
	err = session.Query(`SELECT set_int FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`,
		"User_Set2", int64(30)).Scan(&setInt)
	require.NoError(t, err, "Failed to select the updated record")

	assert.Equal(t, []int{42}, setInt, "The set should enforce uniqueness and contain only one instance of 42")
}

func TestValidateSetReads(t *testing.T) {
	t.Parallel()
	// 1. Initialize user with a set of integers
	err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, set_int) VALUES (?, ?, ?)`,
		"set_reads", int64(900), []int{28, 56, 2}).Exec()
	require.NoError(t, err, "Failed to insert record")

	// 2. Read the entire set without aliases
	var name string
	var setInt []int
	err = session.Query(`SELECT name, set_int FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`,
		"set_reads", int64(900)).Scan(&name, &setInt)
	require.NoError(t, err, "Failed to read set without aliases")
	assert.Equal(t, "set_reads", name)
	assert.ElementsMatch(t, []int{2, 28, 56}, setInt)

	// 3. Read the entire set with aliases
	var n string
	var s []int
	err = session.Query(`SELECT name as n, set_int as s FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`,
		"set_reads", int64(900)).Scan(&n, &s)
	require.NoError(t, err, "Failed to read set with aliases")
	assert.Equal(t, "set_reads", n)
	assert.ElementsMatch(t, []int{2, 28, 56}, s)
}

func TestValidateSetOperationsWithContainsClause(t *testing.T) {
	t.Parallel()
	// 1. Initialize user with a set of integers and a set of text
	err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, set_int, tags) VALUES (?, ?, ?, ?)`,
		"Jassie", int64(23), []int{1221, 1222}, []string{"earth", "moon", "sun"}).Exec()
	require.NoError(t, err, "Failed to insert record")

	// 2. Verify that querying with CONTAINS fails without ALLOW FILTERING
	t.Run("CONTAINS on existing element", func(t *testing.T) {
		t.Parallel()
		var tags []string
		err := session.Query(`SELECT tags FROM bigtabledevinstance.user_info WHERE tags CONTAINS ?`, "earth").Scan(&tags)
		if testTarget == TestTargetCassandra {
			require.Error(t, err, "Expected query to fail without ALLOW FILTERING")
			assert.Contains(t, err.Error(), "ALLOW FILTERING", "Error message should suggest ALLOW FILTERING")
		} else {
			assert.ElementsMatch(t, []string{"earth", "moon", "sun"}, tags)
		}
	})

	// 3. Verify that a query for a non-existent element also fails in the same way
	t.Run("CONTAINS on non-existent element", func(t *testing.T) {
		t.Parallel()
		var tags []string
		err := session.Query(`SELECT tags FROM bigtabledevinstance.user_info WHERE tags CONTAINS ?`, "jupiter").Scan(&tags)
		if testTarget == TestTargetCassandra {
			require.Error(t, err, "Expected query to fail without ALLOW FILTERING")
			assert.Contains(t, err.Error(), "ALLOW FILTERING", "Error message should suggest ALLOW FILTERING")
		} else {
			assert.ElementsMatch(t, []string{}, tags)
		}
	})

	t.Run("CONTAINS with ALLOW FILTERING", func(t *testing.T) {
		t.Parallel()
		var tags []string
		err := session.Query(`SELECT tags FROM bigtabledevinstance.user_info WHERE tags CONTAINS ? ALLOW FILTERING`, "moon").Scan(&tags)
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"earth", "moon", "sun"}, tags)
	})
}
