package types

import (
	"fmt"
)

type DynamicValue interface {
	GetValue(values *QueryParameterValues) (GoValue, error)
	IsIdempotent() bool
	GetParameter() Parameter
	GetType() CqlDataType
}

type ParameterizedValue struct {
	parameter Parameter
	dataType  CqlDataType
}

func (p *ParameterizedValue) GetParameter() Parameter {
	return p.parameter
}

func (p *ParameterizedValue) GetType() CqlDataType {
	return p.dataType
}

func (p *ParameterizedValue) IsIdempotent() bool {
	return true
}

func (p *ParameterizedValue) GetValue(values *QueryParameterValues) (GoValue, error) {
	return values.GetValue(p.parameter)
}

func NewParameterizedValue(parameter Parameter, dt CqlDataType) DynamicValue {
	return &ParameterizedValue{parameter: parameter, dataType: dt}
}

type TimestampNowValue struct {
}

func (f *TimestampNowValue) IsIdempotent() bool {
	return false
}

func (f *TimestampNowValue) GetValue(values *QueryParameterValues) (GoValue, error) {
	return values.Time(), nil
}

type LiteralValue struct {
	Value GoValue
	dt    CqlDataType
}

func (l *LiteralValue) GetParameter() Parameter {
	return ""
}

func (l *LiteralValue) GetType() CqlDataType {
	return l.dt
}

func (l *LiteralValue) IsIdempotent() bool {
	return true
}

func (l *LiteralValue) GetValue(values *QueryParameterValues) (GoValue, error) {
	return l.Value, nil
}

func NewLiteralValue(value GoValue, dt CqlDataType) DynamicValue {
	return &LiteralValue{Value: value, dt: dt}
}

type FunctionValue struct {
	Parameter Parameter
	Func      CqlFunc
	Args      []DynamicValue
}

func (f FunctionValue) GetType() CqlDataType {
	return f.Func.GetReturnType(f.Args)
}

func (f FunctionValue) GetParameter() Parameter {
	return f.Parameter
}

func (f FunctionValue) GetValue(values *QueryParameterValues) (GoValue, error) {
	apply, err := f.Func.Apply(f.Args, values)
	if err != nil {
		return nil, err
	}
	return apply, nil
}

func (f FunctionValue) IsIdempotent() bool {
	return false
}

func NewFunctionValue(p Parameter, f CqlFunc, args ...DynamicValue) *FunctionValue {
	return &FunctionValue{
		Parameter: p,
		Func:      f,
		Args:      args,
	}
}

type ColumnValue struct {
	Column *Column
}

func (c ColumnValue) GetType() CqlDataType {
	return c.Column.CQLType
}

func (c ColumnValue) GetValue(values *QueryParameterValues) (GoValue, error) {
	return nil, fmt.Errorf("cannot get value on a column")
}

func (c ColumnValue) IsIdempotent() bool {
	return true
}

func (c ColumnValue) GetParameter() Parameter {
	return ""
}

func NewColumnValue(column *Column) DynamicValue {
	return &ColumnValue{Column: column}
}

type SelectStarValue struct {
}

func (c SelectStarValue) GetType() CqlDataType {
	return TypeText
}

func (c SelectStarValue) GetValue(values *QueryParameterValues) (GoValue, error) {
	return nil, fmt.Errorf("cannot get value on a select * value")
}

func (c SelectStarValue) IsIdempotent() bool {
	return true
}

func (c SelectStarValue) GetParameter() Parameter {
	return ""
}

func NewSelectStarValue() DynamicValue {
	return &SelectStarValue{}
}

type MapAccessValue struct {
	Column  *Column
	mapType *MapType
	// placeholders are NOT allowed
	MapKey ColumnQualifier
}

func NewMapAccessValue(column *Column, mapType *MapType, mapKey ColumnQualifier) DynamicValue {
	return &MapAccessValue{Column: column, mapType: mapType, MapKey: mapKey}
}

func (m MapAccessValue) GetValue(values *QueryParameterValues) (GoValue, error) {
	return nil, fmt.Errorf("GetValue not allowed")
}

func (m MapAccessValue) IsIdempotent() bool {
	return true
}

func (m MapAccessValue) GetParameter() Parameter {
	return ""
}

func (m MapAccessValue) GetType() CqlDataType {
	return m.mapType.valueType
}

type ListElementValue struct {
	Column   *Column
	listType *ListType
	// placeholders are NOT allowed
	ListIndex int64
}

func NewListElementValue(column *Column, listType *ListType, listIndex int64) DynamicValue {
	return &ListElementValue{Column: column, listType: listType, ListIndex: listIndex}
}

func (l ListElementValue) GetValue(values *QueryParameterValues) (GoValue, error) {
	return nil, fmt.Errorf("GetValue not allowed")
}

func (l ListElementValue) IsIdempotent() bool {
	return true
}

func (l ListElementValue) GetParameter() Parameter {
	return ""
}

func (l ListElementValue) GetType() CqlDataType {
	return l.listType.elementType
}

func IsSelectStar(v DynamicValue) bool {
	_, ok := v.(SelectStarValue)
	return ok
}

func IsColumn(v DynamicValue) bool {
	_, ok := v.(ColumnValue)
	return ok
}
