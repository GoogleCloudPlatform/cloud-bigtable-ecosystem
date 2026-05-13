package compliance

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInsertAndValidationListText(t *testing.T) {
	t.Parallel()
	err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, list_text) VALUES (?, ?, ?)`,
		"Alice", int64(25), []string{"apple", "banana", "cherry"}).Exec()
	require.NoError(t, err, "Failed to insert record with list<text>")

	var listText []string
	err = session.Query(`SELECT list_text FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`, "Alice", int64(25)).Scan(&listText)
	require.NoError(t, err)
	assert.Equal(t, []string{"apple", "banana", "cherry"}, listText)
}
func TestInsertAndValidationListAscii(t *testing.T) {
	t.Parallel()
	err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, list_ascii) VALUES (?, ?, ?)`,
		"ascii_list_row", int64(25), []string{"apple", "banana", "cherry"}).Exec()
	require.NoError(t, err, "Failed to insert record with list<ascii>")

	var listText []string
	err = session.Query(`SELECT list_ascii FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`, "ascii_list_row", int64(25)).Scan(&listText)
	require.NoError(t, err)
	assert.Equal(t, []string{"apple", "banana", "cherry"}, listText)
}

func TestInsertListAsciiWithEmptyString(t *testing.T) {
	t.Parallel()
	err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, list_ascii) VALUES (?, ?, ?)`,
		"ascii_list_empty", int64(25), []string{"apple", ""}).Exec()
	require.NoError(t, err, "Failed to insert record with list<ascii>")

	var listText []string
	err = session.Query(`SELECT list_ascii FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`, "ascii_list_empty", int64(25)).Scan(&listText)
	require.NoError(t, err)
	assert.Equal(t, []string{"apple", ""}, listText)
}

func TestAppendElementsToListText(t *testing.T) {
	t.Parallel()
	err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, list_text) VALUES (?, ?, ?)`,
		"User_List1", int64(25), []string{"apple"}).Exec()
	require.NoError(t, err)

	err = session.Query(`UPDATE bigtabledevinstance.user_info SET list_text = list_text + ? WHERE name = ? AND age = ?`,
		[]string{"banana", "cherry"}, "User_List1", int64(25)).Exec()
	require.NoError(t, err)

	var listText []string
	err = session.Query(`SELECT list_text FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`, "User_List1", int64(25)).Scan(&listText)
	require.NoError(t, err)
	assert.Equal(t, []string{"apple", "banana", "cherry"}, listText, "List order or content after append is incorrect")
}

func TestReplaceElementInListIntByIndex(t *testing.T) {
	t.Parallel()
	err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, list_int) VALUES (?, ?, ?)`,
		"User_List2", int64(30), []int{100, 200, 300, 400, 500, 600}).Exec()
	require.NoError(t, err)

	err = session.Query(`UPDATE bigtabledevinstance.user_info SET list_int[4] = ? WHERE name = ? AND age = ?`,
		999, "User_List2", int64(30)).Exec()
	require.NoError(t, err)

	var listInt []int
	err = session.Query(`SELECT list_int FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`, "User_List2", int64(30)).Scan(&listInt)
	require.NoError(t, err)
	assert.Equal(t, []int{100, 200, 300, 400, 999, 600}, listInt, "List element was not replaced correctly")
}

func TestRemoveElementsByValueFromListInt(t *testing.T) {
	t.Parallel()
	// Note: The user name/age combination is duplicated from the previous test, but Cassandra will
	// simply overwrite the record since the primary key is the same.
	err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, list_int) VALUES (?, ?, ?)`,
		"User_List2", int64(25), []int{10, 20, 10, 30}).Exec()
	require.NoError(t, err)

	err = session.Query(`UPDATE bigtabledevinstance.user_info SET list_int = list_int - ? WHERE name = ? AND age = ?`,
		[]int{10}, "User_List2", int64(25)).Exec()
	require.NoError(t, err)

	var listInt []int
	err = session.Query(`SELECT list_int FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`, "User_List2", int64(25)).Scan(&listInt)
	require.NoError(t, err)
	assert.Equal(t, []int{20, 30}, listInt, "Removing elements by value failed")
}

func TestAppendAndPrependToListText(t *testing.T) {
	t.Parallel()
	err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, list_text) VALUES (?, ?, ?)`,
		"User_List3", int64(35), []string{"banana"}).Exec()
	require.NoError(t, err)

	// Prepend
	err = session.Query(`UPDATE bigtabledevinstance.user_info SET list_text = ? + list_text WHERE name = ? AND age = ?`,
		[]string{"apple"}, "User_List3", int64(35)).Exec()
	require.NoError(t, err)

	// Append
	err = session.Query(`UPDATE bigtabledevinstance.user_info SET list_text = list_text + ? WHERE name = ? AND age = ?`,
		[]string{"cherry"}, "User_List3", int64(35)).Exec()
	require.NoError(t, err)

	var listText []string
	err = session.Query(`SELECT list_text FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`, "User_List3", int64(35)).Scan(&listText)
	require.NoError(t, err)
	assert.Equal(t, []string{"apple", "banana", "cherry"}, listText, "List order after prepend and append is incorrect")
}

func TestCombinedForAllListTypes(t *testing.T) {
	t.Parallel()
	ts1 := time.UnixMilli(1735725600000).UTC()
	ts2 := time.UnixMilli(1738404000000).UTC()
	ts3 := time.UnixMilli(1741096800000).UTC()

	// 1. Insert initial record with all list types
	err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, list_text, list_int, list_bigint, list_float, list_double, list_boolean, list_timestamp) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"Combined_User", int64(50), []string{"alpha", "beta"}, []int{100, 200, 300}, []int64{10000000000, 20000000000, 30000000000},
		[]float32{1.1}, []float64{1.11, 2.22}, []bool{true, false, true}, []time.Time{ts1, ts2}).Exec()
	require.NoError(t, err)

	// 2. Perform updates
	err = session.Query(`UPDATE bigtabledevinstance.user_info SET list_float = list_float + ? WHERE name = ? AND age = ?`,
		[]float32{2.2, 3.3}, "Combined_User", int64(50)).Exec()
	require.NoError(t, err)

	err = session.Query(`UPDATE bigtabledevinstance.user_info SET list_timestamp[0] = ? WHERE name = ? AND age = ?`,
		ts3, "Combined_User", int64(50)).Exec()
	require.NoError(t, err)

	// 3. Validate final state
	var listText []string
	var listInt []int
	var listBigint []int64
	var listFloat []float32
	var listDouble []float64
	var listBoolean []bool
	var listTimestamp []time.Time
	err = session.Query(`SELECT list_text, list_int, list_bigint, list_float, list_double, list_boolean, list_timestamp FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`,
		"Combined_User", int64(50)).Scan(&listText, &listInt, &listBigint, &listFloat, &listDouble, &listBoolean, &listTimestamp)
	require.NoError(t, err)

	assert.Equal(t, []string{"alpha", "beta"}, listText)
	assert.Equal(t, []int{100, 200, 300}, listInt)
	assert.Equal(t, []int64{10000000000, 20000000000, 30000000000}, listBigint)
	assert.Equal(t, []float32{1.1, 2.2, 3.3}, listFloat)
	assert.Equal(t, []float64{1.11, 2.22}, listDouble)
	assert.Equal(t, []bool{true, false, true}, listBoolean)
	assert.Equal(t, []time.Time{ts3, ts2}, listTimestamp)
}

func TestDeleteElementsByIndexInAllListTypes(t *testing.T) {
	t.Parallel()
	ts1 := time.UnixMilli(1735725600000).UTC()
	ts2 := time.UnixMilli(1738404000000).UTC()
	ts3 := time.UnixMilli(1741096800000).UTC()

	// Use a different PK to avoid overwriting the previous test's data
	name, age := "Combined_User_Delete", int64(51)

	// 1. Insert initial record
	err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, list_text, list_int, list_bigint, list_float, list_double, list_boolean, list_timestamp) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		name, age, []string{"alpha", "beta", "gamma"}, []int{100, 200, 300}, []int64{10000000000, 20000000000, 30000000000},
		[]float32{1.1, 2.2, 3.3}, []float64{1.11, 2.22, 3.33}, []bool{true, false, true}, []time.Time{ts1, ts2, ts3}).Exec()
	require.NoError(t, err)

	// 2. Delete elements at various indices
	err = session.Query(`DELETE list_text[1],list_int[0],list_bigint[2],list_float[1],list_double[0],list_boolean[2],list_timestamp[1] FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`,
		name, age).Exec()
	require.NoError(t, err)

	// 3. Validate final state
	var listText []string
	var listInt []int
	var listBigint []int64
	var listFloat []float32
	var listDouble []float64
	var listBoolean []bool
	var listTimestamp []time.Time
	err = session.Query(`SELECT list_text, list_int, list_bigint, list_float, list_double, list_boolean, list_timestamp FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`,
		name, age).Scan(&listText, &listInt, &listBigint, &listFloat, &listDouble, &listBoolean, &listTimestamp)
	require.NoError(t, err)

	assert.Equal(t, []string{"alpha", "gamma"}, listText)
	assert.Equal(t, []int{200, 300}, listInt)
	assert.Equal(t, []int64{10000000000, 20000000000}, listBigint)
	assert.Equal(t, []float32{1.1, 3.3}, listFloat)
	assert.Equal(t, []float64{2.22, 3.33}, listDouble)
	assert.Equal(t, []bool{true, false}, listBoolean)
	assert.Equal(t, []time.Time{ts1, ts3}, listTimestamp)
}

func TestInsertLargeListInt(t *testing.T) {
	t.Parallel()
	// 1. Prepare a large list of integers
	largeList := make([]int, 101)
	for i := 0; i <= 100; i++ {
		largeList[i] = i
	}

	// 2. Insert and append
	err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, list_int) VALUES (?, ?, ?)`,
		"User_List5", int64(50), []int{0}).Exec()
	require.NoError(t, err)

	// Append the rest of the numbers (1-100)
	err = session.Query(`UPDATE bigtabledevinstance.user_info SET list_int = list_int + ? WHERE name = ? AND age = ?`,
		largeList[1:], "User_List5", int64(50)).Exec()
	require.NoError(t, err)

	// 3. Verify the final list
	var listInt []int
	err = session.Query(`SELECT list_int FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`, "User_List5", int64(50)).Scan(&listInt)
	require.NoError(t, err)
	assert.Equal(t, largeList, listInt)
}

func TestListReads(t *testing.T) {
	t.Parallel()
	err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, list_int) VALUES (?, ?, ?)`,
		"list_reads", int64(55), []int{33, 56, 55}).Exec()
	require.NoError(t, err)

	// Read without alias
	var name string
	var listInt []int
	err = session.Query(`SELECT name, list_int FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`, "list_reads", int64(55)).Scan(&name, &listInt)
	require.NoError(t, err)
	assert.Equal(t, "list_reads", name)
	assert.Equal(t, []int{33, 56, 55}, listInt)

	// Read with alias
	var n string
	var l []int
	err = session.Query(`SELECT name as n, list_int as l FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`, "list_reads", int64(55)).Scan(&n, &l)
	require.NoError(t, err)
	assert.Equal(t, "list_reads", n)
	assert.Equal(t, []int{33, 56, 55}, l)
}

func TestListReadWithContainsKeyClause(t *testing.T) {
	t.Parallel()
	// 1. Insert record
	err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, list_int, list_text) VALUES (?, ?, ?, ?)`,
		"Jack", int64(55), []int{100, 200, 300}, []string{"red", "blue", "green"}).Exec()
	require.NoError(t, err)

	// 2. Test that CONTAINS fails without ALLOW FILTERING
	t.Run("CONTAINS without ALLOW FILTERING", func(t *testing.T) {
		t.Parallel()
		var name string
		err := session.Query(`SELECT name FROM bigtabledevinstance.user_info WHERE list_text CONTAINS ?`, "red").Scan(&name)
		if testTarget == TestTargetCassandra {
			require.Error(t, err, "Expected an error for CONTAINS on a non-indexed column without ALLOW FILTERING")
			assert.Contains(t, err.Error(), "ALLOW FILTERING")
		} else {
			require.NoError(t, err)
			assert.Equal(t, "Jack", name)
		}
	})

	// 3. Test that CONTAINS succeeds with ALLOW FILTERING
	t.Run("CONTAINS with ALLOW FILTERING", func(t *testing.T) {
		t.Parallel()
		var name string
		var listInt []int
		err := session.Query(`SELECT name, list_int FROM bigtabledevinstance.user_info WHERE list_text CONTAINS ? ALLOW FILTERING`, "red").Scan(&name, &listInt)
		require.NoError(t, err)
		assert.Equal(t, "Jack", name)
		assert.Equal(t, []int{100, 200, 300}, listInt)
	})

	// 4. Test CONTAINS for a non-existent value
	t.Run("CONTAINS for non-existent value", func(t *testing.T) {
		t.Parallel()
		iter := session.Query(`SELECT name FROM bigtabledevinstance.user_info WHERE list_text CONTAINS ? ALLOW FILTERING`, "brown").Iter()
		assert.Equal(t, 0, iter.NumRows())
		require.NoError(t, iter.Close())
	})
}
