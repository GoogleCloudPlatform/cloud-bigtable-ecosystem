package types

import (
	"fmt"
	"golang.org/x/exp/maps"
)

type NamedQueryParameters struct {
	params map[Placeholder]*PlaceholderMetadata
}

func (n *NamedQueryParameters) GetMetadataByIndex(index int) (*PlaceholderMetadata, error) {
	for _, md := range n.params {
		if md.Order == index {
			return md, nil
		}
	}
	return nil, fmt.Errorf("no placeholder for index %d", index)
}

func (n *NamedQueryParameters) Metadata() []*PlaceholderMetadata {
	return maps.Values(n.params)
}

func (n *NamedQueryParameters) GetMetadata(placeholder Placeholder) (*PlaceholderMetadata, error) {
	metadata, ok := n.params[placeholder]
	if !ok {
		return nil, fmt.Errorf("missing named parameter '%s'", placeholder)
	}
	return metadata, nil
}

func (n *NamedQueryParameters) Params() map[Placeholder]*PlaceholderMetadata {
	return n.params
}

func NewNamedQueryParameters(params map[Placeholder]*PlaceholderMetadata) IQueryParameters {
	return &NamedQueryParameters{params: params}
}

func (n *NamedQueryParameters) Count() int {
	return len(n.params)
}
