package types

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCqlTypesEqual(t *testing.T) {
	tests := []struct {
		name     string
		a        CqlDataType
		b        CqlDataType
		expected bool
	}{
		{
			name:     "Same scalar types",
			a:        TypeInt,
			b:        TypeInt,
			expected: true,
		},
		{
			name:     "Different scalar types",
			a:        TypeInt,
			b:        TypeBigInt,
			expected: false,
		},
		{
			name:     "Varchar and Text are equal",
			a:        TypeVarchar,
			b:        TypeText,
			expected: true,
		},
		{
			name:     "Same list types",
			a:        NewListType(TypeInt),
			b:        NewListType(TypeInt),
			expected: true,
		},
		{
			name:     "Different list types",
			a:        NewListType(TypeInt),
			b:        NewListType(TypeBigInt),
			expected: false,
		},
		{
			name:     "Same set types",
			a:        NewSetType(TypeVarchar),
			b:        NewSetType(TypeVarchar),
			expected: true,
		},
		{
			name:     "Different set types",
			a:        NewSetType(TypeVarchar),
			b:        NewSetType(TypeText), // Should be true because Varchar == Text
			expected: true,
		},
		{
			name:     "Same map types",
			a:        NewMapType(TypeInt, TypeVarchar),
			b:        NewMapType(TypeInt, TypeVarchar),
			expected: true,
		},
		{
			name:     "Different map key types",
			a:        NewMapType(TypeInt, TypeVarchar),
			b:        NewMapType(TypeBigInt, TypeVarchar),
			expected: false,
		},
		{
			name:     "Different map value types",
			a:        NewMapType(TypeInt, TypeVarchar),
			b:        NewMapType(TypeInt, TypeBigInt),
			expected: false,
		},
		{
			name:     "Same frozen types",
			a:        NewFrozenType(NewListType(TypeInt)),
			b:        NewFrozenType(NewListType(TypeInt)),
			expected: true,
		},
		{
			name:     "Different frozen types",
			a:        NewFrozenType(NewListType(TypeInt)),
			b:        NewFrozenType(NewListType(TypeBigInt)),
			expected: false,
		},
		{
			name:     "Nil types",
			a:        nil,
			b:        nil,
			expected: true,
		},
		{
			name:     "One nil type",
			a:        TypeInt,
			b:        nil,
			expected: false,
		},
		{
			name:     "Deeply nested equal types",
			a:        NewMapType(TypeInt, NewListType(NewFrozenType(TypeVarchar))),
			b:        NewMapType(TypeInt, NewListType(NewFrozenType(TypeText))),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, CqlTypesEqual(tt.a, tt.b))
		})
	}
}
