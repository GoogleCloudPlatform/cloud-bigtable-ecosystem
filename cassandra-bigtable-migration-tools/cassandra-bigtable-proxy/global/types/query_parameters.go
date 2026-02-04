package types

import (
	"fmt"
	"time"
)

// Parameter - represents a query parameter, either named or positional. For named parameters, the exact name,
// provided in the query is used, without the leading colon symbol. For positional parameters, "value$INDEX" is used,
// where $INDEX is the index (zero indexed) of the parameter.
type Parameter string

type ParameterMetadata struct {
	Key     Parameter
	Order   int // zero indexed
	IsNamed bool
	Type    CqlDataType
	// can be null if the placeholder is not related to a specific column
	Column *Column
}

func newParameterMetadata(
	key Parameter,
	order int,
	isNamed bool,
	tpe CqlDataType,
	column *Column,
) *ParameterMetadata {
	return &ParameterMetadata{Key: key, Order: order, IsNamed: isNamed, Type: tpe, Column: column}
}

type QueryParameters struct {
	ordered []Parameter
	params  map[Parameter]*ParameterMetadata
}

func NewQueryParameters(ordered []Parameter, params map[Parameter]*ParameterMetadata) *QueryParameters {
	return &QueryParameters{ordered: ordered, params: params}
}

func NewEmptyQueryParameters() *QueryParameters {
	return NewQueryParameters(nil, nil)
}

func (q *QueryParameters) GetMetadataByIndex(index int) (*ParameterMetadata, error) {
	if index < 0 || index >= len(q.ordered) {
		return nil, fmt.Errorf("placeholder index %d is out of bounds (0-%d)", index, len(q.ordered))
	}
	return q.GetMetadata(q.ordered[index])
}

func (q *QueryParameters) GetMetadata(p Parameter) (*ParameterMetadata, error) {
	md, ok := q.params[p]
	if !ok {
		return nil, fmt.Errorf("no query param for '%s'", p)
	}
	return md, nil
}

func (q *QueryParameters) Count() int {
	return len(q.ordered)
}

func (q *QueryParameters) Ordered() []*ParameterMetadata {
	result := make([]*ParameterMetadata, len(q.ordered))
	for _, metadata := range q.params {
		result[metadata.Order] = metadata
	}
	return result
}

type QueryParameterValues struct {
	params *QueryParameters
	time   time.Time
	values map[Parameter]GoValue
}

func (q *QueryParameterValues) Params() *QueryParameters {
	return q.params
}

func (q *QueryParameterValues) Time() time.Time {
	return q.time
}

func NewQueryParameterValues(params *QueryParameters, t time.Time) *QueryParameterValues {
	return &QueryParameterValues{params: params, values: make(map[Parameter]GoValue), time: t}
}

func (q *QueryParameterValues) SetValue(p Parameter, value any) error {
	q.values[p] = value
	return nil
}

func (q *QueryParameterValues) GetValue(p Parameter) (GoValue, error) {
	if v, ok := q.values[p]; ok {
		return v, nil
	}
	return nil, fmt.Errorf("no query param for %s", p)
}

func (q *QueryParameterValues) AllValuesSet() bool {
	return len(q.values) == q.params.Count()
}
func (q *QueryParameterValues) AsMap() map[Parameter]GoValue {
	return q.values
}
