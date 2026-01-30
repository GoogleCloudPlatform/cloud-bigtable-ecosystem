package types

import (
	"fmt"
	"time"
)

type Placeholder string

type PlaceholderMetadata struct {
	Key     Placeholder
	Order   int
	IsNamed bool
	Type    CqlDataType
	// can be null if the placeholder is not related to a specific column
	Column *Column
}

func newPlaceholderMetadata(
	key Placeholder,
	order int,
	isNamed bool,
	tpe CqlDataType,
	column *Column,
) *PlaceholderMetadata {
	return &PlaceholderMetadata{Key: key, Order: order, IsNamed: isNamed, Type: tpe, Column: column}
}

type IQueryParameters interface {
	Count() int
	GetMetadata(placeholder Placeholder) (*PlaceholderMetadata, error)
	GetMetadataByIndex(index int) (*PlaceholderMetadata, error)
	Metadata() []*PlaceholderMetadata
}

func NewEmptyQueryParameters() IQueryParameters {
	return &PositionalQueryParameters{ordered: nil}
}

type QueryParameterValues struct {
	params IQueryParameters
	time   time.Time
	values map[Placeholder]GoValue
}

func (q *QueryParameterValues) Params() IQueryParameters {
	return q.params
}

func (q *QueryParameterValues) Time() time.Time {
	return q.time
}

func NewQueryParameterValues(params IQueryParameters, t time.Time) *QueryParameterValues {
	return &QueryParameterValues{params: params, values: make(map[Placeholder]GoValue), time: t}
}

func (q *QueryParameterValues) SetValue(p Placeholder, value any) error {
	q.values[p] = value
	return nil
}

func (q *QueryParameterValues) GetValue(p Placeholder) (GoValue, error) {
	if v, ok := q.values[p]; ok {
		return v, nil
	}
	return nil, fmt.Errorf("no query param for %s", p)
}

func (q *QueryParameterValues) AllValuesSet() bool {
	return len(q.values) == q.params.Count()
}
func (q *QueryParameterValues) AsMap() map[Placeholder]GoValue {
	return q.values
}
