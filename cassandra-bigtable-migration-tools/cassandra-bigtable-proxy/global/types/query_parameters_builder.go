package types

import (
	"errors"
	"fmt"
)

type QueryParameterBuilder struct {
	metadata map[Parameter]*ParameterMetadata
}

func NewQueryParameterBuilder() *QueryParameterBuilder {
	return &QueryParameterBuilder{
		metadata: make(map[Parameter]*ParameterMetadata),
	}
}

func (q *QueryParameterBuilder) getNextIndex() int {
	return len(q.metadata)
}
func (q *QueryParameterBuilder) getNextParameter() Parameter {
	return Parameter(fmt.Sprintf("value%d", q.getNextIndex()))
}

func (q *QueryParameterBuilder) AddPositionalParam(dt CqlDataType, c *Column) (Parameter, error) {
	p := q.getNextParameter()
	_, exists := q.metadata[p]
	if exists {
		// could happen if named parameters are also used (which is not allowed in cql)
		return "", errors.New("unexpected positional param key already exists")
	}
	q.metadata[p] = newParameterMetadata(p, q.getNextIndex(), false, dt, c)
	return p, nil
}

func (q *QueryParameterBuilder) AddNamedParam(p Parameter, dt CqlDataType) error {
	existing, exists := q.metadata[p]
	if exists && existing.Type.Code() != dt.Code() {
		return fmt.Errorf("multiple named parameters for '%s' with conflicting types: '%s' vs. '%s'", string(p), existing.Type.String(), dt.String())
	} else if exists {
		// we already have this param so we can return now
		return nil
	}
	q.metadata[p] = newParameterMetadata(p, q.getNextIndex(), true, dt, nil)
	return nil
}

// Build - builds and validates the query parameters. An error is returned if both positional and named parameters are
// used (which Cassandra does not allow).
func (q *QueryParameterBuilder) Build() (*QueryParameters, error) {
	if len(q.metadata) > 0 {
		isNamed := false
		for _, metadata := range q.metadata {
			isNamed = metadata.IsNamed
			break
		}
		for _, md := range q.metadata {
			if md.IsNamed != isNamed {
				return nil, fmt.Errorf("cannot use both positional and named parameters in the same query")
			}
		}
	}

	ordered := make([]Parameter, len(q.metadata))
	for _, md := range q.metadata {
		ordered[md.Order] = md.Key
	}
	return NewQueryParameters(ordered, q.metadata), nil
}
