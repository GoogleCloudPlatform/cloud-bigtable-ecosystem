package compliance

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBasicCounterValidation(t *testing.T) {
	pkUser, pkId := "user123", 30

	// Initialize the counter with + 0. The row is created on the first update.
	require.NoError(t, session.Query(`UPDATE social_posts SET likes = likes + ? WHERE user_id = ? AND id = ?`, 0, pkUser, pkId).Exec())

	// Validate initial state
	var likes, views int64
	require.NoError(t, session.Query(`SELECT likes, views FROM social_posts WHERE user_id = ? AND id = ?`, pkUser, pkId).Scan(&likes, &views))
	assert.Equal(t, int64(0), likes, "Likes should initialize to 0")
	assert.Equal(t, int64(0), views, "Un-initialized counter (views) should default to 0")

	// Increment the counter
	require.NoError(t, session.Query(`UPDATE social_posts SET likes = likes + ? WHERE user_id = ? AND id = ?`, 2, pkUser, pkId).Exec())

	// Validate the increment
	require.NoError(t, session.Query(`SELECT likes FROM social_posts WHERE user_id = ? AND id = ?`, pkUser, pkId).Scan(&likes))
	assert.Equal(t, int64(2), likes, "Likes should be incremented to 2")

	// Validate with SELECT *
	results := make(map[string]interface{})
	require.NoError(t, session.Query(`SELECT * FROM social_posts WHERE user_id = ? AND id = ?`, pkUser, pkId).MapScan(results))
	assert.Equal(t, pkUser, results["user_id"])
	assert.Equal(t, pkId, results["id"])
	assert.Equal(t, int64(2), results["likes"])
	assert.Equal(t, int64(0), results["views"])
}

func TestIncrementAndDecrementCounter(t *testing.T) {
	pkUser, pkId := "userABC", 1

	// Sequence of updates
	require.NoError(t, session.Query(`UPDATE social_posts SET likes = likes + ? WHERE user_id = ? AND id = ?`, 2, pkUser, pkId).Exec())
	// subtract a negative should increase
	require.NoError(t, session.Query(`UPDATE social_posts SET likes = likes - ? WHERE user_id = ? AND id = ?`, -3, pkUser, pkId).Exec())
	// add a negative should decrease
	require.NoError(t, session.Query(`UPDATE social_posts SET likes = likes + ? WHERE user_id = ? AND id = ?`, -1, pkUser, pkId).Exec())
	require.NoError(t, session.Query(`UPDATE social_posts SET views = views - ? WHERE user_id = ? AND id = ?`, 10, pkUser, pkId).Exec())

	// Validate after initial updates
	var likes, views int64
	err := session.Query(`SELECT likes, views FROM social_posts WHERE user_id = ? AND id = ?`, pkUser, pkId).Scan(&likes, &views)
	require.NoError(t, err)
	assert.Equal(t, int64(4), likes)
	assert.Equal(t, int64(-10), views)

	// Add a negative value
	require.NoError(t, session.Query(`UPDATE social_posts SET likes = likes + ? WHERE user_id = ? AND id = ?`, -1, pkUser, pkId).Exec())
	err = session.Query(`SELECT likes FROM social_posts WHERE user_id = ? AND id = ?`, pkUser, pkId).Scan(&likes)
	require.NoError(t, err)
	assert.Equal(t, int64(3), likes)

	// Subtract a negative value (should be an addition)
	require.NoError(t, session.Query(`UPDATE social_posts SET likes = likes - ? WHERE user_id = ? AND id = ?`, -1, pkUser, pkId).Exec())
	err = session.Query(`SELECT likes FROM social_posts WHERE user_id = ? AND id = ?`, pkUser, pkId).Scan(&likes)
	require.NoError(t, err)
	assert.Equal(t, int64(4), likes)

	// Subtract a large value to make the counter negative
	require.NoError(t, session.Query(`UPDATE social_posts SET likes = likes - ? WHERE user_id = ? AND id = ?`, 99, pkUser, pkId).Exec())
	err = session.Query(`SELECT likes FROM social_posts WHERE user_id = ? AND id = ?`, pkUser, pkId).Scan(&likes)
	require.NoError(t, err)
	assert.Equal(t, int64(-95), likes)
}

func TestAggregateQueriesOnCounters(t *testing.T) {
	pkUser := "user_aggregate_test"
	require.NoError(t, session.Query(`UPDATE social_posts SET likes = likes + ? WHERE user_id = ? AND id = ?`, 2, pkUser, 1).Exec())
	require.NoError(t, session.Query(`UPDATE social_posts SET likes = likes + ? WHERE user_id = ? AND id = ?`, 50, pkUser, 2).Exec())
	// Two updates on the same counter
	require.NoError(t, session.Query(`UPDATE social_posts SET likes = likes + ? WHERE user_id = ? AND id = ?`, 1, pkUser, 3).Exec())
	require.NoError(t, session.Query(`UPDATE social_posts SET likes = likes + ? WHERE user_id = ? AND id = ?`, 10, pkUser, 3).Exec())

	t.Run("Aggregates without aliases", func(t *testing.T) {
		var maxLikes, minLikes, sumLikes int64
		require.NoError(t, session.Query(`SELECT max(likes), min(likes), sum(likes) FROM social_posts WHERE user_id = ?`, pkUser).Scan(&maxLikes, &minLikes, &sumLikes))
		assert.Equal(t, int64(50), maxLikes, "MAX is incorrect")
		assert.Equal(t, int64(2), minLikes, "MIN is incorrect")
		assert.Equal(t, int64(63), sumLikes, "SUM is incorrect") // 2 + 50 + (1+10) = 63
	})

	t.Run("Aggregates with aliases", func(t *testing.T) {
		results := make(map[string]interface{})
		require.NoError(t, session.Query(`SELECT max(likes) as mxl, min(likes), sum(likes) as sml FROM social_posts WHERE user_id = ?`, pkUser).MapScan(results))
		assert.Equal(t, int64(50), results["mxl"])
		assert.Equal(t, int64(2), results["system.min(likes)"])
		assert.Equal(t, int64(63), results["sml"])
	})
}

func TestGroupByAndOrderByCounters(t *testing.T) {
	// cassandra doesn't support grouping or ordering by a counter
	if testTarget == TestTargetCassandra {
		t.Skip()
		return
	}

	pkUser := "gbob" // group by order by
	require.NoError(t, session.Query(`UPDATE social_posts SET likes = likes + ? WHERE user_id = ? AND id = ?`, 2, pkUser, 1).Exec())
	require.NoError(t, session.Query(`UPDATE social_posts SET likes = likes + ? WHERE user_id = ? AND id = ?`, 3, pkUser, 2).Exec())
	require.NoError(t, session.Query(`UPDATE social_posts SET likes = likes + ? WHERE user_id = ? AND id = ?`, 2, pkUser, 3).Exec())
	require.NoError(t, session.Query(`UPDATE social_posts SET likes = likes + ? WHERE user_id = ? AND id = ?`, 5, pkUser, 4).Exec())

	t.Run("ORDER BY on counter", func(t *testing.T) {
		scanner := session.Query(`SELECT likes FROM social_posts WHERE user_id = ? ORDER BY likes`, pkUser).Iter().Scanner()
		var results []int32 = nil
		for scanner.Next() {
			var likes int32
			err := scanner.Scan(
				&likes,
			)
			require.NoError(t, err)
			results = append(results, likes)
		}
		assert.ElementsMatch(t, []int32{2, 2, 3, 5}, results)
	})

	t.Run("ORDER BY on alias of counter", func(t *testing.T) {
		scanner := session.Query(`SELECT likes as l FROM social_posts WHERE user_id = ? ORDER BY l desc`, pkUser).Iter().Scanner()
		var results []int32 = nil
		for scanner.Next() {
			var likes int32
			err := scanner.Scan(
				&likes,
			)
			require.NoError(t, err)
			results = append(results, likes)
		}
		assert.ElementsMatch(t, []int32{5, 3, 2, 2}, results)
	})

	t.Run("GROUP BY on counter", func(t *testing.T) {
		scanner := session.Query(`SELECT likes FROM social_posts WHERE user_id = ? GROUP BY likes`, pkUser).Iter().Scanner()
		var results []int32 = nil
		for scanner.Next() {
			var likes int32
			err := scanner.Scan(
				&likes,
			)
			require.NoError(t, err)
			results = append(results, likes)
		}
		assert.ElementsMatch(t, []int32{2, 3, 5}, results)
	})
}

func TestSetCounterToNull(t *testing.T) {
	// Cassandra doesn't support setting a counter to null
	if testTarget == TestTargetCassandra {
		t.Skip()
		return
	}

	pkUser, pkId := "user_null_test", 1

	// 1. Initialize the counter
	require.NoError(t, session.Query(`UPDATE social_posts SET likes = likes + ? WHERE user_id = ? AND id = ?`, 10, pkUser, pkId).Exec())

	// 2. Validate it's set
	var likes int64
	require.NoError(t, session.Query(`SELECT likes FROM social_posts WHERE user_id = ? AND id = ?`, pkUser, pkId).Scan(&likes))
	assert.Equal(t, int64(10), likes)

	// 3. Set counter to null via placeholder
	require.NoError(t, session.Query(`UPDATE social_posts SET likes = ? WHERE user_id = ? AND id = ?`, nil, pkUser, pkId).Exec())

	// 4. Validate it's null (which defaults to 0 in Cassandra for counters)
	require.NoError(t, session.Query(`SELECT likes FROM social_posts WHERE user_id = ? AND id = ?`, pkUser, pkId).Scan(&likes))
	assert.Equal(t, int64(0), likes, "Counter should be 0 after being set to null")

	// 5. Set counter to null via literal
	require.NoError(t, session.Query(`UPDATE social_posts SET likes = likes + ? WHERE user_id = ? AND id = ?`, 10, pkUser, pkId).Exec())
	require.NoError(t, session.Query(`UPDATE social_posts SET likes = NULL WHERE user_id = ? AND id = ?`, pkUser, pkId).Exec())

	// 6. Validate it's null again
	require.NoError(t, session.Query(`SELECT likes FROM social_posts WHERE user_id = ? AND id = ?`, pkUser, pkId).Scan(&likes))
	assert.Equal(t, int64(0), likes, "Counter should be 0 after being set to NULL literal")
}
