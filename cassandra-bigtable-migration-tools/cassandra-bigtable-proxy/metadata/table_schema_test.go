package metadata

import (
	"testing"

	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	"github.com/stretchr/testify/assert"
)

func TestTableConfig_Describe(t *testing.T) {
	tests := []struct {
		name  string
		table *TableSchema
		want  string
	}{
		{
			name: "Success",
			table: NewTableConfig("keyspace1", "table", "cf1", types.OrderedCodeEncoding, []*types.Column{
				{
					Name:         "name",
					ColumnFamily: "cf1",
					CQLType:      types.TypeVarchar,
					IsPrimaryKey: false,
					PkPrecedence: 0,
					KeyType:      types.KeyTypeRegular,
				},
				{
					Name:         "org_id",
					ColumnFamily: "cf1",
					CQLType:      types.TypeBigInt,
					IsPrimaryKey: true,
					PkPrecedence: 1,
					KeyType:      types.KeyTypePartition,
				},
				{
					Name:         "user_id",
					ColumnFamily: "cf1",
					CQLType:      types.TypeBigInt,
					IsPrimaryKey: true,
					PkPrecedence: 2,
					KeyType:      types.KeyTypeClustering,
				},
			}),
			want: "CREATE TABLE keyspace1.table (\n    org_id BIGINT,\n    user_id BIGINT,\n    name VARCHAR,\n    PRIMARY KEY (org_id, user_id)\n);",
		},
		{
			name: "two partition keys",
			table: NewTableConfig("keyspace1", "table", "cf1", types.OrderedCodeEncoding, []*types.Column{
				{
					Name:         "org_id",
					ColumnFamily: "cf1",
					CQLType:      types.TypeBigInt,
					IsPrimaryKey: true,
					PkPrecedence: 1,
					KeyType:      types.KeyTypePartition,
				},
				{
					Name:         "user_id",
					ColumnFamily: "cf1",
					CQLType:      types.TypeBigInt,
					IsPrimaryKey: true,
					PkPrecedence: 2,
					KeyType:      types.KeyTypePartition,
				},
				{
					Name:         "group_id",
					ColumnFamily: "cf1",
					CQLType:      types.TypeBigInt,
					IsPrimaryKey: true,
					PkPrecedence: 3,
					KeyType:      types.KeyTypeClustering,
				},
				{
					Name:         "name",
					ColumnFamily: "cf1",
					CQLType:      types.TypeVarchar,
					IsPrimaryKey: false,
					PkPrecedence: 0,
					KeyType:      types.KeyTypeRegular,
				},
			}),
			want: "CREATE TABLE keyspace1.table (\n    org_id BIGINT,\n    user_id BIGINT,\n    group_id BIGINT,\n    name VARCHAR,\n    PRIMARY KEY ((org_id, user_id), group_id)\n);",
		},
		{
			name: "one partition key",
			table: NewTableConfig("keyspace1", "table", "cf1", types.OrderedCodeEncoding, []*types.Column{
				{
					Name:         "org_id",
					ColumnFamily: "cf1",
					CQLType:      types.TypeBigInt,
					IsPrimaryKey: true,
					PkPrecedence: 1,
					KeyType:      types.KeyTypePartition,
				},
				{
					Name:         "user_id",
					ColumnFamily: "cf1",
					CQLType:      types.TypeBigInt,
					IsPrimaryKey: false,
					PkPrecedence: 0,
					KeyType:      types.KeyTypeRegular,
				},
				{
					Name:         "name",
					ColumnFamily: "cf1",
					CQLType:      types.TypeVarchar,
					IsPrimaryKey: false,
					PkPrecedence: 0,
					KeyType:      types.KeyTypeRegular,
				},
			}),
			want: "CREATE TABLE keyspace1.table (\n    org_id BIGINT,\n    user_id BIGINT,\n    name VARCHAR,\n    PRIMARY KEY (org_id)\n);",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.table.Describe()
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_SameTable(t *testing.T) {
	tests := []struct {
		name  string
		table *TableSchema
		other *TableSchema
		want  bool
	}{
		{
			name:  "Success",
			table: NewTableConfig("k1", "t1", "cf1", types.OrderedCodeEncoding, nil),
			other: NewTableConfig("k1", "t1", "cf1", types.OrderedCodeEncoding, nil),
			want:  true,
		},
		{
			name:  "Different name",
			table: NewTableConfig("k1", "t1", "cf1", types.OrderedCodeEncoding, nil),
			other: NewTableConfig("k1", "t2", "cf1", types.OrderedCodeEncoding, nil),
			want:  false,
		},
		{
			name:  "Different keyspace",
			table: NewTableConfig("k1", "t1", "cf1", types.OrderedCodeEncoding, nil),
			other: NewTableConfig("k2", "t1", "cf1", types.OrderedCodeEncoding, nil),
			want:  false,
		},
		{
			name:  "nil other",
			table: NewTableConfig("k1", "t1", "cf1", types.OrderedCodeEncoding, nil),
			other: nil,
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.table.SameTable(tt.other)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_SameTableSchema(t *testing.T) {
	tests := []struct {
		name  string
		table *TableSchema
		other *TableSchema
		want  bool
	}{
		{
			name: "Success",
			table: NewTableConfig("k1", "t1", "cf1", types.OrderedCodeEncoding, []*types.Column{
				{
					Name:         "c1",
					ColumnFamily: "",
					CQLType:      types.TypeText,
					IsPrimaryKey: false,
					PkPrecedence: 1,
					KeyType:      types.KeyTypePartition,
				},
				{
					Name:         "c2",
					ColumnFamily: "cf1",
					CQLType:      types.TypeInt,
					IsPrimaryKey: false,
					PkPrecedence: 1,
					KeyType:      types.KeyTypeRegular,
				},
			}),
			other: NewTableConfig("k1", "t1", "cf1", types.OrderedCodeEncoding, []*types.Column{
				{
					Name:         "c1",
					ColumnFamily: "",
					CQLType:      types.TypeText,
					IsPrimaryKey: false,
					PkPrecedence: 1,
					KeyType:      types.KeyTypePartition,
				},
				{
					Name:         "c2",
					ColumnFamily: "cf1",
					CQLType:      types.TypeInt,
					IsPrimaryKey: false,
					PkPrecedence: 1,
					KeyType:      types.KeyTypeRegular,
				},
			}),
			want: true,
		},
		{
			name: "Different column name",
			table: NewTableConfig("k1", "t1", "cf1", types.OrderedCodeEncoding, []*types.Column{
				{
					Name:         "c1",
					ColumnFamily: "",
					CQLType:      types.TypeText,
					IsPrimaryKey: false,
					PkPrecedence: 1,
					KeyType:      types.KeyTypePartition,
				},
				{
					Name:         "c2",
					ColumnFamily: "cf1",
					CQLType:      types.TypeInt,
					IsPrimaryKey: false,
					PkPrecedence: 1,
					KeyType:      types.KeyTypeRegular,
				},
			}),
			other: NewTableConfig("k1", "t1", "cf1", types.OrderedCodeEncoding, []*types.Column{
				{
					Name:         "c1",
					ColumnFamily: "",
					CQLType:      types.TypeText,
					IsPrimaryKey: false,
					PkPrecedence: 1,
					KeyType:      types.KeyTypePartition,
				},
				{
					Name:         "WRONG",
					ColumnFamily: "cf1",
					CQLType:      types.TypeInt,
					IsPrimaryKey: false,
					PkPrecedence: 1,
					KeyType:      types.KeyTypeRegular,
				},
			}),
			want: false,
		},
		{
			name: "Different column type",
			table: NewTableConfig("k1", "t1", "cf1", types.OrderedCodeEncoding, []*types.Column{
				{
					Name:         "c1",
					ColumnFamily: "",
					CQLType:      types.TypeText,
					IsPrimaryKey: false,
					PkPrecedence: 1,
					KeyType:      types.KeyTypePartition,
				},
				{
					Name:         "c2",
					ColumnFamily: "cf1",
					CQLType:      types.TypeInt,
					IsPrimaryKey: false,
					PkPrecedence: 1,
					KeyType:      types.KeyTypeRegular,
				},
			}),
			other: NewTableConfig("k1", "t1", "cf1", types.OrderedCodeEncoding, []*types.Column{
				{
					Name:         "c1",
					ColumnFamily: "",
					CQLType:      types.TypeText,
					IsPrimaryKey: false,
					PkPrecedence: 1,
					KeyType:      types.KeyTypePartition,
				},
				{
					Name:         "c2",
					ColumnFamily: "cf1",
					CQLType:      types.TypeBoolean,
					IsPrimaryKey: false,
					PkPrecedence: 1,
					KeyType:      types.KeyTypeRegular,
				},
			}),
			want: false,
		},
		{
			name: "Same schema with collections", // testing collections because comparing those types can't be done with '=='
			table: NewTableConfig("k1", "t1", "cf1", types.OrderedCodeEncoding, []*types.Column{
				{
					Name:         "c1",
					ColumnFamily: "",
					CQLType:      types.TypeText,
					IsPrimaryKey: false,
					PkPrecedence: 1,
					KeyType:      types.KeyTypePartition,
				},
				{
					Name:         "c2",
					ColumnFamily: "cf1",
					CQLType:      types.TypeInt,
					IsPrimaryKey: false,
					PkPrecedence: 1,
					KeyType:      types.KeyTypeRegular,
				},
				{
					Name:         "map_text",
					ColumnFamily: "map_text",
					CQLType:      types.NewMapType(types.TypeText, types.TypeText),
					KeyType:      types.KeyTypeRegular,
				},
			}),
			other: NewTableConfig("k1", "t1", "cf1", types.OrderedCodeEncoding, []*types.Column{
				{
					Name:         "c1",
					ColumnFamily: "",
					CQLType:      types.TypeText,
					IsPrimaryKey: false,
					PkPrecedence: 1,
					KeyType:      types.KeyTypePartition,
				},
				{
					Name:         "c2",
					ColumnFamily: "cf1",
					CQLType:      types.TypeInt,
					IsPrimaryKey: false,
					PkPrecedence: 1,
					KeyType:      types.KeyTypeRegular,
				},
				{
					Name:         "map_text",
					ColumnFamily: "map_text",
					CQLType:      types.NewMapType(types.TypeText, types.TypeText),
					KeyType:      types.KeyTypeRegular,
				},
			}),
			want: true,
		},
		{
			name: "nil other",
			table: NewTableConfig("k1", "t1", "cf1", types.OrderedCodeEncoding, []*types.Column{
				{
					Name:         "c1",
					ColumnFamily: "",
					CQLType:      types.TypeText,
					IsPrimaryKey: false,
					PkPrecedence: 1,
					KeyType:      types.KeyTypePartition,
				},
				{
					Name:         "c2",
					ColumnFamily: "",
					CQLType:      types.TypeInt,
					IsPrimaryKey: false,
					PkPrecedence: 1,
					KeyType:      types.KeyTypeRegular,
				},
			}),
			other: nil,
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.table.SameSchema(tt.other)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTableConfig_GetMetadata(t *testing.T) {
	table := NewTableConfig("keyspace1", "table1", "cf1", types.OrderedCodeEncoding, []*types.Column{
		{
			Name:         "zebra",
			ColumnFamily: "cf1",
			CQLType:      types.TypeVarchar,
			IsPrimaryKey: false,
			PkPrecedence: 0,
			KeyType:      types.KeyTypeRegular,
		},
		{
			Name:         "b_ck2",
			ColumnFamily: "",
			CQLType:      types.TypeInt,
			IsPrimaryKey: true,
			PkPrecedence: 4,
			KeyType:      types.KeyTypeClustering,
		},
		{
			Name:         "apple",
			ColumnFamily: "cf1",
			CQLType:      types.TypeInt,
			IsPrimaryKey: false,
			PkPrecedence: 0,
			KeyType:      types.KeyTypeRegular,
		},
		{
			Name:         "m_pk2",
			ColumnFamily: "",
			CQLType:      types.TypeVarchar,
			IsPrimaryKey: true,
			PkPrecedence: 2,
			KeyType:      types.KeyTypePartition,
		},
		{
			Name:         "mango",
			ColumnFamily: "mango",
			CQLType:      types.NewListType(types.TypeText),
			IsPrimaryKey: false,
			PkPrecedence: 0,
			KeyType:      types.KeyTypeRegular,
		},
		{
			Name:         "z_pk1",
			ColumnFamily: "",
			CQLType:      types.TypeBigInt,
			IsPrimaryKey: true,
			PkPrecedence: 1,
			KeyType:      types.KeyTypePartition,
		},
		{
			Name:         "x_ck1",
			ColumnFamily: "",
			CQLType:      types.TypeVarchar,
			IsPrimaryKey: true,
			PkPrecedence: 3,
			KeyType:      types.KeyTypeClustering,
		},
		{
			Name:         "banana",
			ColumnFamily: "cf1",
			CQLType:      types.TypeBoolean,
			IsPrimaryKey: false,
			PkPrecedence: 0,
			KeyType:      types.KeyTypeRegular,
		},
	})

	wantColumnNames := []string{
		"z_pk1",
		"m_pk2",
		"x_ck1",
		"b_ck2",
		"apple",
		"banana",
		"mango",
		"zebra",
	}

	// Verify deterministic ordering across multiple calls despite map backing
	for i := 0; i < 10; i++ {
		gotMetadata := table.GetMetadata()
		var gotNames []string
		for _, md := range gotMetadata {
			gotNames = append(gotNames, md.Name)
		}
		assert.Equal(t, wantColumnNames, gotNames)

		gotCols := table.AllColumns()
		var gotColNames []string
		for _, col := range gotCols {
			gotColNames = append(gotColNames, string(col.Name))
		}
		assert.Equal(t, wantColumnNames, gotColNames)
	}
}
