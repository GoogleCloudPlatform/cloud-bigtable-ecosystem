package compliance

import (
	"fmt"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type timestampRow struct {
	region      string
	eventTime   time.Time
	measurement int
	endTime     time.Time
}

const TIME_FMT = "2006-01-02 15:04:05.000-0700"
const OUTPUT_FMT = "2006-01-02 15:04:05.000000-0700"

func readTimestampRow(q string) ([]*timestampRow, error) {
	got, err := cqlshScanToMap(q)
	if err != nil {
		return nil, err
	}
	var results []*timestampRow = nil
	for _, row := range got {
		m, err := strconv.Atoi(row["measurement"])
		if err != nil {
			return nil, err
		}
		eventTime, err := time.Parse(OUTPUT_FMT, row["event_time"])
		if err != nil {
			return nil, err
		}
		endTime, err := time.Parse(OUTPUT_FMT, row["end_time"])
		if err != nil {
			return nil, err
		}

		results = append(results, &timestampRow{
			region:      row["region"],
			eventTime:   eventTime.UTC(),
			measurement: m,
			endTime:     endTime.UTC(),
		})
	}
	return results, nil
}

func TestInsert(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		insertEventTime interface{}
		// provide an alternative type to read with so we can test writing with one type and reading with another
		selectEventTime interface{}
		wantTime        time.Time
	}{
		{
			name:            "0",
			insertEventTime: time.UnixMilli(0).UTC(),
			selectEventTime: time.UnixMilli(0).UTC(),
			wantTime:        time.UnixMilli(0).UTC(),
		},
		{
			name:            "0 int64",
			insertEventTime: 0,
			selectEventTime: 0,
			wantTime:        time.UnixMilli(0).UTC(),
		},
		{
			name:            "1",
			insertEventTime: time.UnixMilli(1).UTC(),
			selectEventTime: time.UnixMilli(1).UTC(),
			wantTime:        time.UnixMilli(1).UTC(),
		},
		{
			name:            "1000",
			insertEventTime: time.UnixMilli(1000).UTC(),
			selectEventTime: time.UnixMilli(1000).UTC(),
			wantTime:        time.UnixMilli(1000).UTC(),
		},
		{
			name:            "1000000",
			insertEventTime: time.UnixMilli(1000000).UTC(),
			selectEventTime: time.UnixMilli(1000000).UTC(),
			wantTime:        time.UnixMilli(1000000).UTC(),
		},
		{
			name:            "2025-10-05 06:29:01 time",
			insertEventTime: time.Date(2025, 10, 5, 6, 29, 1, 13, time.UTC),
			selectEventTime: time.Date(2025, 10, 5, 6, 29, 1, 13, time.UTC),
			wantTime:        time.Date(2025, 10, 5, 6, 29, 1, 13, time.UTC),
		},
		{
			name:            "write time read int",
			insertEventTime: time.Date(2025, 10, 5, 6, 29, 1, 13, time.UTC),
			selectEventTime: time.Date(2025, 10, 5, 6, 29, 1, 13, time.UTC).UnixMilli(),
			wantTime:        time.Date(2025, 10, 5, 6, 29, 1, 13, time.UTC),
		},
		{
			name:            "write int read time",
			insertEventTime: time.Date(2025, 10, 5, 6, 29, 1, 13, time.UTC).UnixMilli(),
			selectEventTime: time.Date(2025, 10, 5, 6, 29, 1, 13, time.UTC),
			wantTime:        time.Date(2025, 10, 5, 6, 29, 1, 13, time.UTC),
		},
		{
			name:            "write string read int",
			insertEventTime: time.Date(2025, 10, 5, 6, 29, 1, 13, time.UTC).Format(TIME_FMT),
			selectEventTime: time.Date(2025, 10, 5, 6, 29, 1, 13, time.UTC).UnixMilli(),
			wantTime:        time.Date(2025, 10, 5, 6, 29, 1, 13, time.UTC),
		},
		{
			name:            "write string read time",
			insertEventTime: time.Date(2025, 10, 5, 6, 29, 1, 13, time.UTC).Format(TIME_FMT),
			selectEventTime: time.Date(2025, 10, 5, 6, 29, 1, 13, time.UTC),
			wantTime:        time.Date(2025, 10, 5, 6, 29, 1, 13, time.UTC),
		},
		{
			name:            "write int read string",
			insertEventTime: time.Date(2025, 10, 5, 6, 29, 1, 13, time.UTC).UnixMilli(),
			selectEventTime: time.Date(2025, 10, 5, 6, 29, 1, 13, time.UTC).Format(TIME_FMT),
			wantTime:        time.Date(2025, 10, 5, 6, 29, 1, 13, time.UTC),
		},
		{
			name:            "2025-10-05 06:29:01 int64",
			insertEventTime: time.Date(2025, 10, 5, 6, 29, 1, 13, time.UTC).UnixMilli(),
			selectEventTime: time.Date(2025, 10, 5, 6, 29, 1, 13, time.UTC).UnixMilli(),
			wantTime:        time.Date(2025, 10, 5, 6, 29, 1, 13, time.UTC),
		},
		{
			name:            "2025-10-05 06:29:01 string",
			insertEventTime: time.Date(2025, 10, 5, 6, 29, 1, 13, time.UTC).Format(TIME_FMT),
			selectEventTime: time.Date(2025, 10, 5, 6, 29, 1, 13, time.UTC).Format(TIME_FMT),
			wantTime:        time.Date(2025, 10, 5, 6, 29, 1, 13, time.UTC),
		},
	}

	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			region := fmt.Sprintf("us-east%d", i+1)
			endTime := time.Now().UTC()
			measurement := rand.Int31()

			// we need to convert
			err := session.Query(`INSERT INTO bigtabledevinstance.timestamp_key (region, event_time, measurement, end_time) VALUES (?, ?, ?, ?)`).Bind(region, anyToTime(tc.insertEventTime), measurement, endTime).Exec()
			require.NoError(t, err)

			result := timestampRow{}
			err = session.Query(`SELECT region, event_time, measurement, end_time FROM bigtabledevinstance.timestamp_key WHERE region = ? AND event_time = ?`, region, anyToTime(tc.selectEventTime)).Scan(&result.region, &result.eventTime, &result.measurement, &result.endTime)
			require.NoError(t, err)
			assert.Equal(t, region, result.region)
			assert.Equal(t, tc.wantTime.Truncate(time.Millisecond), result.eventTime)
			assert.Equal(t, endTime.Truncate(time.Millisecond), result.endTime)
			assert.Equal(t, int(measurement), result.measurement)
		})

		t.Run("cqlsh_"+tc.name, func(t *testing.T) {
			t.Parallel()
			region := fmt.Sprintf("us-south-%d", i+1)
			measurement := rand.Int31()
			endTime := time.Now()

			_, err := cqlshExec(fmt.Sprintf("INSERT INTO bigtabledevinstance.timestamp_key (region, event_time, measurement, end_time) VALUES ('%s', '%s', %d, '%s')", region, anyToTime(tc.insertEventTime).Format(TIME_FMT), measurement, endTime.Format(TIME_FMT)))
			require.NoError(t, err)

			got, err := readTimestampRow(fmt.Sprintf("SELECT region, event_time, measurement, end_time FROM bigtabledevinstance.timestamp_key WHERE region = '%s' AND event_time = '%s'", region, anyToTime(tc.selectEventTime).Format(TIME_FMT)))
			require.NoError(t, err)
			require.Equal(t, []*timestampRow{
				{
					region:      region,
					eventTime:   tc.wantTime.UTC().Truncate(time.Millisecond),
					measurement: int(measurement),
					endTime:     endTime.UTC().Truncate(time.Millisecond),
				},
			}, got)
		})
	}

	t.Run("timestamp(now())", func(t *testing.T) {
		t.Parallel()
		region := fmt.Sprintf("us-east-2")

		_, err := cqlshExec("INSERT INTO bigtabledevinstance.timestamp_key (region, event_time, measurement, end_time) VALUES ('us-east-2', toTimestamp(now()), 2, toTimestamp(now()))")
		require.NoError(t, err)

		result := timestampRow{}
		err = session.Query(`SELECT region, event_time, measurement, end_time FROM bigtabledevinstance.timestamp_key WHERE region = ?`, region).Scan(&result.region, &result.eventTime, &result.measurement, &result.endTime)
		require.NoError(t, err)
		assert.Equal(t, region, result.region)
		assert.Equal(t, 2, result.measurement)
		assert.Equal(t, result.eventTime.UnixMilli(), result.endTime.UnixMilli(), "timestamps should be equal because they were both set with timestamp(now())")
		assert.Less(t, abs(result.eventTime.UnixMilli()), int64(1000), "timestamp should be within 1s of local time")
		assert.Less(t, abs(result.endTime.UnixMilli()), int64(1000), "timestamp should be within 1s of local time")
	})
}

func abs(t int64) int64 {
	delta := t - time.Now().UTC().UnixMilli()
	if delta < 0 {
		return delta * -1
	}
	return delta
}

func anyToTime(value interface{}) time.Time {
	switch v := value.(type) {
	case string:
		t, err := time.Parse(TIME_FMT, v)
		if err != nil {
			panic(err.Error())
		}
		return t.UTC()
	case time.Time:
		return v.UTC()
	case int64:
		return time.UnixMilli(v).UTC()
	case int:
		return time.UnixMilli(int64(v)).UTC()
	default:
		panic(fmt.Sprintf("unknown value type: %T", value))
	}
}
