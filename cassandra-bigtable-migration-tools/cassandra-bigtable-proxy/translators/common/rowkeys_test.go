package common

import (
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	schemaMapping "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/metadata"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/testing/mockdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestBindRowKeyTimestampPrecision(t *testing.T) {
	micro := time.UnixMicro(1767727307246309)
	millis := time.UnixMilli(1767727307246)
	table := mockdata.GetTableOrDie("test_keyspace", "timestamp_key")

	rowKeyMicro, err := BindRowKey(table, []types.DynamicValue{types.NewLiteralValue(micro)}, types.NewQueryParameterValues(types.NewQueryParameters(), time.Now()))
	require.NoError(t, err)

	rowKeyMillis, err := BindRowKey(table, []types.DynamicValue{types.NewLiteralValue(millis)}, types.NewQueryParameterValues(types.NewQueryParameters(), time.Now()))
	require.NoError(t, err)

	// ensure microseconds are truncated because Bigtable uses microseconds for keys but Cassandra uses milliseconds
	assert.Equal(t, rowKeyMillis, rowKeyMicro)
}

func TestBindRowKey(t *testing.T) {
	tests := []struct {
		name   string
		table  *schemaMapping.TableSchema
		values []types.GoValue
		want   types.RowKey
		err    string
	}{
		{
			name:  "success",
			table: mockdata.GetTableOrDie("test_keyspace", "test_table"),
			values: []types.GoValue{
				"abc",
				"def",
			},
			want: "abc\x00\x01def",
		},
		{
			name:  "mixed types",
			table: mockdata.GetTableOrDie("test_keyspace", "user_info"),
			values: []types.GoValue{
				"bobby",
				int64(14242),
			},
			want: "bobby\x00\x01\xe07\xa2",
		},
		{
			name:  "missing value",
			table: mockdata.GetTableOrDie("test_keyspace", "test_table"),
			values: []types.GoValue{
				"def",
			},
			want: "",
			err:  "wrong number of primary keys: want 2 got 1",
		},
		{
			name:  "timestamp key",
			table: mockdata.GetTableOrDie("test_keyspace", "timestamp_key"),
			values: []types.GoValue{
				time.UnixMicro(1767727307246309),
			},
			want: "\xff\x06G\xbd\x165I\xb0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var values []types.DynamicValue
			for _, v := range tt.values {
				values = append(values, types.NewLiteralValue(v))
			}
			rowKey, err := BindRowKey(tt.table, values, types.NewQueryParameterValues(types.NewQueryParameters(), time.Now()))
			if tt.err != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, rowKey)
		})
	}
}
