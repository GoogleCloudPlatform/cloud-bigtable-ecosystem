package compliance

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

/*
timestamp_key Schema:
CREATE TABLE IF NOT EXISTS bigtabledevinstance.timestamp_key (

	region text,
	event_time timestamp,
	measurement float,
	end_time timestamp,
	PRIMARY KEY (region, event_time)

);
*/
func TestInsert(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		inputTimestamp time.Time
	}{
		{
			name:           "0",
			inputTimestamp: time.UnixMilli(0),
		},
		{
			name:           "1",
			inputTimestamp: time.UnixMilli(1),
		},
		{
			name:           "1000",
			inputTimestamp: time.UnixMilli(1000),
		},
		{
			name:           "1000000",
			inputTimestamp: time.UnixMilli(1000000),
		},
		{
			name:           "2025-10-05 06:29:01",
			inputTimestamp: time.Date(2025, 10, 5, 6, 29, 1, 13, time.UTC),
		},
	}

	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			region := fmt.Sprintf("us-east-%d", i+1)
			endTime := tc.inputTimestamp.Add(1 * time.Hour)

			err := session.Query(`INSERT INTO bigtabledevinstance.timestamp_key (region, event_time, measurement, end_time) VALUES (?, ?, ?, ?) USING TIMESTAMP ?`,
				region, tc.inputTimestamp, float32(3.14), endTime, tc.inputTimestamp.UnixMilli()).Exec()
			require.NoError(t, err)

			var gotEventTime int64
			var gotWriteTimestamp int64
			var gotEndTime int64
			err = session.Query(`SELECT event_time, WRITETIME(end_time), end_time FROM bigtabledevinstance.timestamp_key WHERE region = ? AND event_time = ?`, region, tc.inputTimestamp).Scan(&gotEventTime, &gotWriteTimestamp, &gotEndTime)
			require.NoError(t, err)
			assert.Equal(t, tc.inputTimestamp.UnixMilli(), gotEventTime)
			assert.Equal(t, tc.inputTimestamp.UnixMilli(), gotWriteTimestamp)
			assert.Equal(t, endTime, endTime)
		})
	}
}
