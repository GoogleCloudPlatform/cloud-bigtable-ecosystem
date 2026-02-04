package common

import (
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/testing/mockdata"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_bindValues(t *testing.T) {
	tests := []struct {
		name             string
		params           *types.QueryParameters
		positionalValues []*primitive.Value
		namedValues      map[string]*primitive.Value
		pv               primitive.ProtocolVersion
		want             map[types.Parameter]types.GoValue
		err              string
	}{
		{
			name: "success: positional",
			params: func() *types.QueryParameters {
				p := types.NewQueryParameterBuilder()
				_, _ = p.AddPositionalParam(types.TypeVarchar, nil)
				_, _ = p.AddPositionalParam(types.TypeVarchar, nil)
				result, _ := p.Build()
				return result
			}(),
			positionalValues: []*primitive.Value{
				mockdata.EncodePrimitiveValueOrDie("abc", types.TypeText, primitive.ProtocolVersion4),
				mockdata.EncodePrimitiveValueOrDie("def", types.TypeText, primitive.ProtocolVersion4),
			},
			want: map[types.Parameter]types.GoValue{
				"value0": "abc",
				"value1": "def",
			},
			err: "",
		},
		{
			name: "success: empty",
			params: func() *types.QueryParameters {
				p := types.NewQueryParameterBuilder()
				result, _ := p.Build()
				return result
			}(),
			namedValues: map[string]*primitive.Value{},
			want:        map[types.Parameter]types.GoValue{},
			err:         "",
		},
		{
			name: "success: named",
			params: func() *types.QueryParameters {
				p := types.NewQueryParameterBuilder()
				_ = p.AddNamedParam("v1", types.TypeVarchar)
				_ = p.AddNamedParam("v2", types.TypeVarchar)
				result, _ := p.Build()
				return result
			}(),
			namedValues: map[string]*primitive.Value{
				"v1": mockdata.EncodePrimitiveValueOrDie("abc", types.TypeText, primitive.ProtocolVersion4),
				"v2": mockdata.EncodePrimitiveValueOrDie("def", types.TypeText, primitive.ProtocolVersion4),
			},
			want: map[types.Parameter]types.GoValue{
				"v1": "abc",
				"v2": "def",
			},
			err: "",
		},
		{
			name: "success: named params but got positional",
			params: func() *types.QueryParameters {
				p := types.NewQueryParameterBuilder()
				_ = p.AddNamedParam("v1", types.TypeVarchar)
				_ = p.AddNamedParam("v2", types.TypeVarchar)
				result, _ := p.Build()
				return result
			}(),
			positionalValues: []*primitive.Value{
				mockdata.EncodePrimitiveValueOrDie("abc", types.TypeText, primitive.ProtocolVersion4),
				mockdata.EncodePrimitiveValueOrDie("def", types.TypeText, primitive.ProtocolVersion4),
			},
			want: map[types.Parameter]types.GoValue{
				"v1": "abc",
				"v2": "def",
			},
			err: "",
		},
		{
			name: "fail: both positional and named param values",
			params: func() *types.QueryParameters {
				p := types.NewQueryParameterBuilder()
				_ = p.AddNamedParam("v1", types.TypeVarchar)
				_ = p.AddNamedParam("v2", types.TypeVarchar)
				result, _ := p.Build()
				return result
			}(),
			positionalValues: []*primitive.Value{
				mockdata.EncodePrimitiveValueOrDie("abc", types.TypeText, primitive.ProtocolVersion4),
				mockdata.EncodePrimitiveValueOrDie("def", types.TypeText, primitive.ProtocolVersion4),
			},
			namedValues: map[string]*primitive.Value{
				"v1": mockdata.EncodePrimitiveValueOrDie("abc", types.TypeText, primitive.ProtocolVersion4),
				"v2": mockdata.EncodePrimitiveValueOrDie("def", types.TypeText, primitive.ProtocolVersion4),
			},
			want: nil,
			err:  "cannot bind both named and positional parameters",
		},
		{
			name: "fail: named values to positional params",
			params: func() *types.QueryParameters {
				p := types.NewQueryParameterBuilder()
				_, _ = p.AddPositionalParam(types.TypeVarchar, nil)
				_, _ = p.AddPositionalParam(types.TypeVarchar, nil)
				result, _ := p.Build()
				return result
			}(),
			namedValues: map[string]*primitive.Value{
				"v1": mockdata.EncodePrimitiveValueOrDie("abc", types.TypeText, primitive.ProtocolVersion4),
				"v2": mockdata.EncodePrimitiveValueOrDie("def", types.TypeText, primitive.ProtocolVersion4),
			},
			want: nil,
			err:  "no query param for '",
		},
		{
			name: "too many input values",
			params: func() *types.QueryParameters {
				p := types.NewQueryParameterBuilder()
				_, _ = p.AddPositionalParam(types.TypeVarchar, nil)
				_, _ = p.AddPositionalParam(types.TypeVarchar, nil)
				result, _ := p.Build()
				return result
			}(),
			positionalValues: []*primitive.Value{
				mockdata.EncodePrimitiveValueOrDie("abc", types.TypeText, primitive.ProtocolVersion4),
				mockdata.EncodePrimitiveValueOrDie("def", types.TypeText, primitive.ProtocolVersion4),
				// unexpected value
				mockdata.EncodePrimitiveValueOrDie("xyz", types.TypeText, primitive.ProtocolVersion4),
			},
			want: nil,
			err:  "expected 2 prepared positional values but got 3",
		},
		{
			name: "too few input values",
			params: func() *types.QueryParameters {
				p := types.NewQueryParameterBuilder()
				_, _ = p.AddPositionalParam(types.TypeVarchar, nil)
				_, _ = p.AddPositionalParam(types.TypeVarchar, nil)
				result, _ := p.Build()
				return result
			}(),
			positionalValues: []*primitive.Value{
				mockdata.EncodePrimitiveValueOrDie("abc", types.TypeText, primitive.ProtocolVersion4),
			},
			want: nil,
			err:  "expected 2 prepared positional values but got 1",
		},
		{
			name: "wrong input type",
			params: func() *types.QueryParameters {
				p := types.NewQueryParameterBuilder()
				_, _ = p.AddPositionalParam(types.NewListType(types.TypeBigInt), nil)
				result, _ := p.Build()
				return result
			}(),
			positionalValues: []*primitive.Value{
				mockdata.EncodePrimitiveValueOrDie("abcdefgh", types.TypeText, primitive.ProtocolVersion4),
			},
			want: nil,
			err:  "cannot decode CQL list<bigint>",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values, err := BindQueryParams(tt.params, tt.positionalValues, tt.namedValues, tt.pv)
			if tt.err != "" {
				assert.Nil(t, values)
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, values.AsMap())
		})
	}
}
