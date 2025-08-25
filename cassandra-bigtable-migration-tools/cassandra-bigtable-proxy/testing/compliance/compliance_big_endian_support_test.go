package compliance

import (
	"testing"

	"github.com/gocql/gocql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBigEndianDataSuccessCase(t *testing.T) {
	if testTarget == TestTargetCassandra {
		t.Skip()
		return
	}
	t.Parallel()
	err := session.Query(`INSERT INTO orders_big_endian_encoded (user_id, order_num, name) VALUES (?, ?, ?)`, "valid_int", int64(30), "foo").Exec()
	require.NoError(t, err)

	var user_id string
	var order_num int64
	var name string
	err = session.Query(`SELECT user_id, order_num, name FROM orders_big_endian_encoded WHERE user_id = ? AND order_num = ?`, "valid_int", int64(30)).Scan(&user_id, &order_num, &name)
	require.NoError(t, err, "Failed to select newly inserted record")
	assert.Equal(t, "valid_int", user_id)
	assert.Equal(t, int64(30), order_num)
	assert.Equal(t, "foo", name)
}

func TestBigEndianDataFailsOnNegativeIntKeys(t *testing.T) {
	if testTarget == TestTargetCassandra {
		t.Skip()
		return
	}
	t.Parallel()
	err := session.Query(`INSERT INTO orders_big_endian_encoded (user_id, order_num, name) VALUES (?, ?, ?)`, "invalid_int", int64(-1), "bad").Exec()
	require.Error(t, err)
	require.Contains(t, err.Error(), "row keys with big endian encoding cannot contain negative integer values")

	var name string
	err = session.Query(`SELECT  name FROM orders_big_endian_encoded WHERE user_id = ? AND order_num = ?`, "invalid_int", int64(30)).Scan(&name)
	assert.Equal(t, gocql.ErrNotFound, err, "not found because the insert should have failed")
}
