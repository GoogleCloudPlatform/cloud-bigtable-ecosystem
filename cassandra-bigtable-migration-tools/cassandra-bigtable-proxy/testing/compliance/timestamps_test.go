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

	for i, tc := range tests {
		t.Run("cqlsh "+tc.name, func(t *testing.T) {
			t.Parallel()
			region := fmt.Sprintf("us-south-%d", i+1)
			inputFmt := "2006-01-02 15:04:05.000-0700"
			endTime := tc.inputTimestamp.Add(1 * time.Hour)

			_, err := cqlshExec(fmt.Sprintf("INSERT INTO bigtabledevinstance.timestamp_key (region, event_time, measurement, end_time) VALUES ('%s', '%s', 1.23, '%s') USING TIMESTAMP %d", region, tc.inputTimestamp.UTC().Format(inputFmt), endTime.UTC().Format(inputFmt), tc.inputTimestamp.UnixMilli()))
			require.NoError(t, err)

			got, err := cqlshScanToMap(fmt.Sprintf("SELECT event_time, WRITETIME(end_time), end_time FROM bigtabledevinstance.timestamp_key WHERE region = '%s' AND event_time = '%s'", region, tc.inputTimestamp.UTC().Format(inputFmt)))
			require.NoError(t, err)
			require.Equal(t, 1, len(got))
			outfmt := "2006-01-02 15:04:05.000000-0700"
			assert.Equal(t, map[string]string{
				"event_time":          tc.inputTimestamp.UTC().Format(outfmt),
				"WRITETIME(end_time)": tc.inputTimestamp.UTC().Format(outfmt),
				"end_time":            endTime.UTC().Format(outfmt),
			}, got[0])
		})
	}
}
