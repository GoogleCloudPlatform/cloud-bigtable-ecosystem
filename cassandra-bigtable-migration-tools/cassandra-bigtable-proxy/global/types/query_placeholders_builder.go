package types

import (
	"errors"
	"fmt"
)

type QueryParameterBuilder struct {
	metadata           map[Placeholder]*PlaceholderMetadata
	hasNamedParameters bool
}

func NewQueryParameterBuilder() *QueryParameterBuilder {
	return &QueryParameterBuilder{
		metadata: make(map[Placeholder]*PlaceholderMetadata),
	}
}

func (q *QueryParameterBuilder) getNextIndex() int {
	return len(q.metadata)
}
func (q *QueryParameterBuilder) getNextParameter() Placeholder {
	return Placeholder(fmt.Sprintf("value%d", q.getNextIndex()))
}

func (q *QueryParameterBuilder) AddPositionalParam(dt CqlDataType, c *Column) (Placeholder, error) {
	p := q.getNextParameter()
	_, exists := q.metadata[p]
	if exists {
		// could happen if named parameters are also used (which is not allowed in cql)
		return "", errors.New("unexpected positional param key already exists")
	}
	q.metadata[p] = newPlaceholderMetadata(p, q.getNextIndex(), false, dt, c)
	return p, nil
}

func (q *QueryParameterBuilder) AddNamedParam(p Placeholder, dt CqlDataType) error {
	existing, exists := q.metadata[p]
	if exists && existing.Type.Code() != dt.Code() {
		return fmt.Errorf("multiple named placeholders for '%s' with conflicting types: '%s' vs. '%s'", string(p), existing.Type.String(), dt.String())
	} else if exists {
		// we already have this param so we can return now
		return nil
	}
	q.metadata[p] = newPlaceholderMetadata(p, q.getNextIndex(), true, dt, nil)
	return nil
}

func (q *QueryParameterBuilder) Build() (IQueryParameters, error) {

	hasPositional := false
	hasNamed := false
	for _, md := range q.metadata {
		if md.IsNamed {
			hasNamed = true
		} else {
			hasPositional = true
		}
	}

	if hasPositional && hasNamed {
		return nil, fmt.Errorf("queries cannot have both named and positional parameters")
	}

	if hasNamed {
		return NewNamedQueryParameters(q.metadata), nil
	}

	ordered := make([]*PlaceholderMetadata, len(q.metadata))
	for _, metadata := range q.metadata {
		ordered[metadata.Order] = metadata
	}

	return NewPositionalQP(ordered), nil
}
