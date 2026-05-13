package types

import (
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/gocql/gocql"
)

type DynamicValue interface {
	GetValue(values *QueryParameterValues) (GoValue, error)
	IsIdempotent() bool
}

type ParameterizedValue struct {
	Parameter Parameter
}

func (p *ParameterizedValue) IsIdempotent() bool {
	return true
}

func (p *ParameterizedValue) GetValue(values *QueryParameterValues) (GoValue, error) {
	return values.GetValue(p.Parameter)
}

func NewParameterizedValue(parameter Parameter) DynamicValue {
	return &ParameterizedValue{Parameter: parameter}
}

type TimestampNowValue struct {
}

func (f *TimestampNowValue) IsIdempotent() bool {
	return false
}

func (f *TimestampNowValue) GetValue(values *QueryParameterValues) (GoValue, error) {
	return values.Time(), nil
}

func NewTimestampNowValue() DynamicValue {
	return &TimestampNowValue{}
}

type TimeuuidNowValue struct {
}

func (f *TimeuuidNowValue) IsIdempotent() bool {
	return false
}

func (f *TimeuuidNowValue) GetValue(values *QueryParameterValues) (GoValue, error) {
	// Cassandra's now() function typically returns a timeuuid (UUIDv1 or UUIDv7).
	// Using gocql.TimeUUID() generates a UUIDv1.
	u := gocql.TimeUUID()
	return primitive.UUID(u), nil
}

func NewTimeuuidNowValue() DynamicValue {
	return &TimeuuidNowValue{}
}

type LiteralValue struct {
	Value GoValue
}

func (l *LiteralValue) IsIdempotent() bool {
	return true
}

func (l *LiteralValue) GetValue(values *QueryParameterValues) (GoValue, error) {
	return l.Value, nil
}

func NewLiteralValue(value GoValue) DynamicValue {
	return &LiteralValue{Value: value}
}
