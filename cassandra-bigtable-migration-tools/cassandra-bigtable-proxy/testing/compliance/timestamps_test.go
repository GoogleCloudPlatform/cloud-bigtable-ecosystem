package compliance

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInsert(t *testing.T) {
	t.Parallel()

	tests := []struct {
		inputTimestamp time.Time
	}{
		{
			inputTimestamp: time.UnixMilli(1),
		},
		{
			inputTimestamp: time.UnixMilli(1000),
		},
		{
			inputTimestamp: time.UnixMilli(1000000),
		},
		{
			inputTimestamp: time.Date(2025, 10, 5, 6, 29, 1, 13, time.UTC),
		},
	}

	for i, tc := range tests {
		t.Run(fmt.Sprintf("timestamp column test %v", tc.inputTimestamp), func(t *testing.T) {
			t.Parallel()
			keyName := "timestamp_test"
			keyAge := int64(i)

			err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, birth_date) VALUES (?, ?, ?) USING TIMESTAMP ?`,
				keyName, keyAge, tc.inputTimestamp, tc.inputTimestamp.UnixMicro()).Exec()
			require.NoError(t, err)

			var gotTimestamp int64
			var gotWriteTimestamp int64
			err = session.Query(`SELECT birth_date, WRITETIME(birth_date) FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`, keyName, keyAge).Scan(&gotTimestamp, &gotWriteTimestamp)
			require.NoError(t, err)
			assert.Equal(t, tc.inputTimestamp.UnixMilli(), gotTimestamp)
			assert.Equal(t, tc.inputTimestamp.UnixMicro(), gotWriteTimestamp)
		})
	}
}
