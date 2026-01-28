package select_translator

import (
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCastColumns(t *testing.T) {
	tests := []struct {
		name    string
		colMeta *types.Column
		want    string
		wantErr bool
	}{
		{
			name: "integer type",
			colMeta: &types.Column{
				Name:         "age",
				ColumnFamily: "cf1",
				CQLType:      types.TypeInt,
			},
			want:    "TO_INT64(`cf1`['age'])",
			wantErr: false,
		},
		{
			name: "bigint type",
			colMeta: &types.Column{
				Name:         "timestamp",
				ColumnFamily: "cf1",
				CQLType:      types.TypeBigInt,
			},
			want:    "TO_INT64(`cf1`['timestamp'])",
			wantErr: false,
		},
		{
			name: "float type",
			colMeta: &types.Column{
				Name:         "price",
				ColumnFamily: "cf1",
				CQLType:      types.TypeFloat,
			},
			want:    "TO_FLOAT32(`cf1`['price'])",
			wantErr: false,
		},
		{
			name: "double type",
			colMeta: &types.Column{
				Name:         "value",
				ColumnFamily: "cf1",
				CQLType:      types.TypeDouble,
			},
			want:    "TO_FLOAT64(`cf1`['value'])",
			wantErr: false,
		},
		{
			name: "boolean type",
			colMeta: &types.Column{
				Name:         "active",
				ColumnFamily: "cf1",
				CQLType:      types.TypeBoolean,
			},
			want:    "TO_INT64(`cf1`['active'])",
			wantErr: false,
		},
		{
			name: "timestamp type",
			colMeta: &types.Column{
				Name:         "created_at",
				ColumnFamily: "cf1",
				CQLType:      types.TypeTimestamp,
			},
			want:    "TIMESTAMP_FROM_UNIX_MILLIS(TO_INT64(`cf1`['created_at']))",
			wantErr: false,
		},
		{
			name: "blob type",
			colMeta: &types.Column{
				Name:         "data",
				ColumnFamily: "cf1",
				CQLType:      types.TypeBlob,
			},
			want:    "`cf1`['data']",
			wantErr: false,
		},
		{
			name: "text type",
			colMeta: &types.Column{
				Name:         "name",
				ColumnFamily: "cf1",
				CQLType:      types.TypeVarchar,
			},
			want:    "`cf1`['name']",
			wantErr: false,
		},
		{
			name: "handle special characters in column name",
			colMeta: &types.Column{
				Name:         "special-name",
				ColumnFamily: "cf1",
				CQLType:      types.TypeVarchar,
			},
			want:    "`cf1`['special-name']",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := castScalarColumn(tt.colMeta)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
