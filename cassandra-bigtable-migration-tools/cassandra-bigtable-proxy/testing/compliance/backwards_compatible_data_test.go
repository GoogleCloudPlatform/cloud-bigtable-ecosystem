package compliance

import (
	"cloud.google.com/go/bigtable"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

// DO NOT CHANGE VALUES - these values are hardcoded to ensure we keep backwards compatibility between versions
func TestBackwardsCompatibleData(t *testing.T) {
	t.Parallel()
	if testTarget == TestTargetCassandra {
		t.Skip()
		return
	}

	const IGNORED_COLUMN_NAME = "ignored"
	const WRITE_TIMESTAMP = 1764084624204000

	testCases := []struct {
		name             string
		column           string
		value            any
		ignoreColumnName bool
		want             bigtable.Row
	}{
		{
			name:   "int column",
			column: "int_col",
			value:  100,
			want: bigtable.Row{
				"cf1": []bigtable.ReadItem{
					{
						Column: "cf1:int_col",
						Value:  []uint8{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x64},
					},
				},
			},
		},
		{
			name:   "bigint column",
			column: "bigint_col",
			value:  100,
			want: bigtable.Row{
				"cf1": []bigtable.ReadItem{
					{
						Column: "cf1:bigint_col",
						// 100 in 8-byte Big Endian
						Value:  []uint8{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x64},
						Labels: nil,
					},
				},
			},
		},
		{
			name:   "text column",
			column: "text_col",
			value:  "hello",
			want: bigtable.Row{
				"cf1": []bigtable.ReadItem{
					{
						Column: "cf1:text_col",
						Value:  []byte("hello"),
					},
				},
			},
		},
		{
			name:   "float column",
			column: "float_col",
			value:  float32(1.5),
			want: bigtable.Row{
				"cf1": []bigtable.ReadItem{
					{
						Column: "cf1:float_col",
						// 1.5 in IEEE 754 4-byte Big Endian
						Value:  []uint8{0x3F, 0xC0, 0x00, 0x00},
						Labels: nil,
					},
				},
			},
		},
		{
			name:   "boolean false",
			column: "bool_col",
			value:  false,
			want: bigtable.Row{
				"cf1": []bigtable.ReadItem{
					{
						Column: "cf1:bool_col",
						Value:  []uint8{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
					},
				},
			},
		},
		{
			name:   "boolean true",
			column: "bool_col",
			value:  true,
			want: bigtable.Row{
				"cf1": []bigtable.ReadItem{
					{
						Column: "cf1:bool_col",
						Value:  []uint8{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x1},
					},
				},
			},
		},
		{
			name:   "double column",
			column: "double_col",
			value:  1.5,
			want: bigtable.Row{
				"cf1": []bigtable.ReadItem{
					{
						Column: "cf1:double_col",
						// 1.5 in IEEE 754 8-byte Big Endian
						Value:  []uint8{0x3F, 0xF8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
						Labels: nil,
					},
				},
			},
		},
		{
			name:   "timestamp column",
			column: "time_col",
			value:  time.UnixMicro(WRITE_TIMESTAMP + 500),
			want: bigtable.Row{
				"cf1": []bigtable.ReadItem{
					{
						Column: "cf1:time_col",
						Value:  []uint8{0x0, 0x0, 0x1, 0x9a, 0xbb, 0xa3, 0x2b, 0x4c},
					},
				},
			},
		},

		// --- MAPS ---
		{
			name:   "map<text, text>",
			column: "map_text_text",
			value:  map[string]string{"abc": "foo", "def": "bar"},
			want: bigtable.Row{
				"map_text_text": []bigtable.ReadItem{
					{
						Column: "map_text_text:abc",
						Value:  []byte("foo"),
					},
					{
						Column: "map_text_text:def",
						Value:  []byte("bar"),
					},
				},
			},
		},
		{
			name:   "map<text, int>",
			column: "map_text_int",
			value:  map[string]int32{"abc": 43, "def": 12},
			want: bigtable.Row{
				"map_text_int": []bigtable.ReadItem{
					{
						Column: "map_text_int:abc",
						Value:  []uint8{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x2b},
					},
					{
						Column: "map_text_int:def",
						Value:  []uint8{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0xc},
					},
				},
			},
		},
		{
			name:   "map<text, bigint>",
			column: "map_text_bigint",
			value:  map[string]int64{"abc": 43, "def": 12},
			want: bigtable.Row{
				"map_text_bigint": []bigtable.ReadItem{
					{
						Column: "map_text_bigint:abc",
						Value:  []uint8{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x2b},
					},
					{
						Column: "map_text_bigint:def",
						Value:  []uint8{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0xc},
					},
				},
			},
		},
		{
			name:   "map<text, boolean>",
			column: "map_text_boolean",
			value:  map[string]bool{"abc": true, "def": false},
			want: bigtable.Row{
				"map_text_boolean": []bigtable.ReadItem{
					{
						Column: "map_text_boolean:abc",
						Value:  []uint8{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x1},
					},
					{
						Column: "map_text_boolean:def",
						Value:  []uint8{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
					},
				},
			},
		},
		{
			name:   "map<text, ts>",
			column: "map_text_ts",
			value:  map[string]time.Time{"abc": time.UnixMicro(WRITE_TIMESTAMP)},
			want: bigtable.Row{
				"map_text_ts": []bigtable.ReadItem{
					{
						Column: "map_text_ts:abc",
						Value:  []uint8{0x0, 0x0, 0x1, 0x9a, 0xbb, 0xa3, 0x2b, 0x4c},
					},
				},
			},
		},
		{
			name:   "map<text, float>",
			column: "map_text_float",
			value:  map[string]float32{"abc": 99.9},
			want: bigtable.Row{
				"map_text_float": []bigtable.ReadItem{
					{
						Column: "map_text_float:abc",
						Value:  []uint8{0x42, 0xc7, 0xcc, 0xcd},
					},
				},
			},
		},
		{
			name:   "map<text, double>",
			column: "map_text_double",
			value:  map[string]float64{"abc": 99.9},
			want: bigtable.Row{
				"map_text_double": []bigtable.ReadItem{
					{
						Column: "map_text_double:abc",
						Value:  []uint8{0x40, 0x58, 0xf9, 0x99, 0x99, 0x99, 0x99, 0x9a},
					},
				},
			},
		},
		{
			name:   "map<timestamp, double>",
			column: "ts_double_map",
			value:  map[time.Time]float64{time.UnixMicro(WRITE_TIMESTAMP): 99.9},
			want: bigtable.Row{
				"ts_double_map": []bigtable.ReadItem{
					{
						Column: "ts_double_map:1764084624204",
						Value:  []uint8{0x40, 0x58, 0xf9, 0x99, 0x99, 0x99, 0x99, 0x9a},
					},
				},
			},
		},

		// --- SETS ---
		{
			name:   "set<text>",
			column: "set_text",
			value:  []string{"tag1", "tag2"},
			want: bigtable.Row{
				"set_text": []bigtable.ReadItem{
					{
						Column: "set_text:tag1",
						Value:  nil,
					},
					{
						Column: "set_text:tag2",
						Value:  nil,
					},
				},
			},
		},
		{
			name:   "set<boolean>",
			column: "set_boolean",
			value:  []bool{false, true},
			want: bigtable.Row{
				"set_boolean": []bigtable.ReadItem{
					{
						Column: "set_boolean:0",
						Value:  nil,
					},
					{
						Column: "set_boolean:1",
						Value:  nil,
					},
				},
			},
		},
		{
			name:   "set<int>",
			column: "set_int",
			value:  []int32{1, 3, 2},
			want: bigtable.Row{
				"set_int": []bigtable.ReadItem{
					{
						Column: "set_int:1",
						Value:  nil,
					},
					{
						Column: "set_int:2",
						Value:  nil,
					},
					{
						Column: "set_int:3",
						Value:  nil,
					},
				},
			},
		},

		{
			name:   "set<bigint>",
			column: "set_bigint",
			value:  []int64{1, 3, 2},
			want: bigtable.Row{
				"set_bigint": []bigtable.ReadItem{
					{
						Column: "set_bigint:1",
						Value:  nil,
					},
					{
						Column: "set_bigint:2",
						Value:  nil,
					},
					{
						Column: "set_bigint:3",
						Value:  nil,
					},
				},
			},
		},
		{
			name:   "set<float>",
			column: "set_float",
			value:  []float32{1.2, 3.2},
			want: bigtable.Row{
				"set_float": []bigtable.ReadItem{
					{
						Column: "set_float:1.2",
						Value:  nil,
					},
					{
						Column: "set_float:3.2",
						Value:  nil,
					},
				},
			},
		},
		{
			name:   "set<double>",
			column: "set_double",
			value:  []float64{1.1, 2.2},
			want: bigtable.Row{
				"set_double": []bigtable.ReadItem{
					{
						Column: "set_double:1.1",
						Value:  nil,
					},
					{
						Column: "set_double:2.2",
						Value:  nil,
					},
				},
			},
		},
		{
			name:   "set<timestamp>",
			column: "set_timestamp",
			value:  []time.Time{time.UnixMicro(WRITE_TIMESTAMP), time.UnixMicro(WRITE_TIMESTAMP - 1000)},
			want: bigtable.Row{
				"set_timestamp": []bigtable.ReadItem{
					{
						Column: "set_timestamp:1764084624203",
						Value:  nil,
					},
					{
						Column: "set_timestamp:1764084624204",
						Value:  nil,
					},
				},
			},
		},

		// --- LISTS ---
		{
			name:             "list<text>",
			column:           "list_text",
			value:            []string{"item1", "item2"},
			ignoreColumnName: true,
			want: bigtable.Row{
				"list_text": []bigtable.ReadItem{
					{
						Column: IGNORED_COLUMN_NAME,
						Value:  []byte("item1"),
					},
					{
						Column: IGNORED_COLUMN_NAME,
						Value:  []byte("item2"),
					},
				},
			},
		},
		{
			name:             "list<int>",
			column:           "list_int",
			value:            []int32{4, 2},
			ignoreColumnName: true,
			want: bigtable.Row{
				"list_int": []bigtable.ReadItem{
					{
						Column: IGNORED_COLUMN_NAME,
						Value:  []byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x4},
					},
					{
						Column: IGNORED_COLUMN_NAME,
						Value:  []byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x2},
					},
				},
			},
		},
		{
			name:             "list<bigint>",
			column:           "list_bigint",
			value:            []int64{24, 929},
			ignoreColumnName: true,
			want: bigtable.Row{
				"list_bigint": []bigtable.ReadItem{
					{
						Column: IGNORED_COLUMN_NAME,
						Value:  []byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x18},
					},
					{
						Column: IGNORED_COLUMN_NAME,
						Value:  []byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x3, 0xa1},
					},
				},
			},
		},
		{
			name:             "list<float>",
			column:           "list_float",
			value:            []float32{1.2, 3.2},
			ignoreColumnName: true,
			want: bigtable.Row{
				"list_float": []bigtable.ReadItem{
					{
						Column: IGNORED_COLUMN_NAME,
						Value:  []uint8{0x3f, 0x99, 0x99, 0x9a},
					},
					{
						Column: IGNORED_COLUMN_NAME,
						Value:  []uint8{0x40, 0x4c, 0xcc, 0xcd},
					},
				},
			},
		},
		{
			name:             "list<boolean>",
			column:           "list_boolean",
			value:            []bool{true, true, false},
			ignoreColumnName: true,
			want: bigtable.Row{
				"list_boolean": []bigtable.ReadItem{
					{
						Column: IGNORED_COLUMN_NAME,
						Value:  []uint8{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x1},
					},
					{
						Column: IGNORED_COLUMN_NAME,
						Value:  []uint8{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x1},
					},
					{
						Column: IGNORED_COLUMN_NAME,
						Value:  []uint8{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
					},
				},
			},
		},
		{
			name:             "list<double>",
			column:           "list_double",
			value:            []float64{1.1, 2.2},
			ignoreColumnName: true,
			want: bigtable.Row{
				"list_double": []bigtable.ReadItem{
					{
						Column: IGNORED_COLUMN_NAME,
						Value:  []uint8{0x3f, 0xf1, 0x99, 0x99, 0x99, 0x99, 0x99, 0x9a},
					},
					{
						Column: IGNORED_COLUMN_NAME,
						Value:  []uint8{0x40, 0x1, 0x99, 0x99, 0x99, 0x99, 0x99, 0x9a},
					},
				},
			},
		},
		{
			name:             "list<timestamp>",
			column:           "list_timestamp",
			value:            []time.Time{time.UnixMicro(WRITE_TIMESTAMP), time.UnixMicro(WRITE_TIMESTAMP - 1000)},
			ignoreColumnName: true,
			want: bigtable.Row{
				"list_timestamp": []bigtable.ReadItem{
					{
						Column: IGNORED_COLUMN_NAME,
						Value:  []uint8{0x0, 0x0, 0x1, 0x9a, 0xbb, 0xa3, 0x2b, 0x4c},
					},
					{
						Column: IGNORED_COLUMN_NAME,
						Value:  []uint8{0x0, 0x0, 0x1, 0x9a, 0xbb, 0xa3, 0x2b, 0x4b},
					},
				},
			},
		},
	}

	table := client.Open("all_columns")

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rowKey := fmt.Sprintf("key%d", i)
			// dynamically set the column
			query := fmt.Sprintf(`INSERT INTO all_columns (name, %s) VALUES (?, ?) USING TIMESTAMP ?`, tc.column)
			err := session.Query(query, rowKey, tc.value, WRITE_TIMESTAMP).Exec()
			require.NoError(t, err)

			emptyRowCellFilter := bigtable.ChainFilters(bigtable.ColumnFilter(".+"), bigtable.LatestNFilter(1))
			row, err := table.ReadRow(t.Context(), rowKey, bigtable.RowFilter(emptyRowCellFilter))
			require.NoError(t, err)

			for _, cells := range row {
				for i := range cells {
					cv := cells[i]
					if tc.ignoreColumnName {
						cv.Column = IGNORED_COLUMN_NAME
					}
					cells[i] = cv
				}
			}

			for _, cells := range tc.want {
				for i := range cells {
					cv := cells[i]
					// set row key, so we don't have to worry about that
					cv.Row = rowKey
					cv.Timestamp = WRITE_TIMESTAMP
					cells[i] = cv
				}
			}

			assert.Equal(t, tc.want, row)
		})
	}
}
