package types

import (
	"fmt"
	"reflect"
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
	Column     *Column
	IsInternal bool
}

func newParameterMetadata(
	key Parameter,
	order int,
	isNamed bool,
	tpe CqlDataType,
	column *Column,
	isInternal bool,
) *ParameterMetadata {
	return &ParameterMetadata{Key: key, Order: order, IsNamed: isNamed, Type: tpe, Column: column, IsInternal: isInternal}
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

func (q *QueryParameters) CountUserParameters() int {
	result := 0
	for _, metadata := range q.params {
		if metadata.IsInternal {
			continue
		}
		result++
	}
	return result
}

func (q *QueryParameters) Ordered() []*ParameterMetadata {
	result := make([]*ParameterMetadata, len(q.ordered))
	for _, metadata := range q.params {
		result[metadata.Order] = metadata
	}
	return result
}

func (q *QueryParameters) OrderedUserParams() []*ParameterMetadata {
	ordered := q.Ordered()
	result := make([]*ParameterMetadata, 0, len(ordered))
	for _, p := range ordered {
		// don't return internal params because we don't want the user to bind to them
		if p.IsInternal {
			continue
		}
		result = append(result, p)
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
func (q *QueryParameterValues) GetValueInt32(dv DynamicValue) (int32, error) {
	value, err := dv.GetValue(q)
	if err != nil {
		return 0, err
	}
	intVal, ok := value.(int32)
	if !ok {
		return 0, fmt.Errorf("query value is a %T, not an int32", value)
	}
	return intVal, nil
}

func (q *QueryParameterValues) GetValueInt64(dv DynamicValue) (int64, error) {
	v, err := dv.GetValue(q)
	if err != nil {
		return 0, err
	}
	intVal, ok := v.(int64)
	if !ok {
		return 0, fmt.Errorf("query value is a %T, not an int64", v)
	}
	return intVal, nil
}

func (q *QueryParameterValues) GetValueTime(dv DynamicValue) (time.Time, error) {
	v, err := dv.GetValue(q)
	if err != nil {
		return time.Time{}, err
	}
	t, ok := v.(time.Time)
	if !ok {
		return time.Time{}, fmt.Errorf("query value is a %T, not a time", v)
	}
	return t, nil
}

func (q *QueryParameterValues) GetValueSlice(dv DynamicValue) ([]GoValue, error) {
	v, err := dv.GetValue(q)
	if err != nil {
		return nil, err
	}

	val := reflect.ValueOf(v)

	if val.Kind() != reflect.Slice {
		return nil, fmt.Errorf("query value is a %T, not a slice", v)
	}

	length := val.Len()
	result := make([]GoValue, length)

	for i := 0; i < length; i++ {
		result[i] = val.Index(i).Interface()
	}

	return result, nil
}

func (q *QueryParameterValues) GetValueMap(dv DynamicValue) (map[GoValue]GoValue, error) {
	v, err := dv.GetValue(q)
	if err != nil {
		return nil, err
	}
	val := reflect.ValueOf(v)

	if val.Kind() != reflect.Map {
		return nil, fmt.Errorf("value is a %T, not a map", v)
	}

	result := make(map[GoValue]GoValue, val.Len())

	// 3. Iterate over the keys of the original map
	iter := val.MapRange()
	for iter.Next() {
		// Get the reflection Placeholder for the key and the value
		keyVal := iter.Key()
		valueVal := iter.Value()

		// 4. Use .Interface() to convert the concrete key/value to an any (interface{})
		keyAny := keyVal.Interface()
		valueAny := valueVal.Interface()

		// 5. Add to the new map[any]any
		result[keyAny] = valueAny
	}
	return result, nil
}
