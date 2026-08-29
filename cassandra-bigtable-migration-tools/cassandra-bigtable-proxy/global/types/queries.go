package types

type OrderOperation string

const (
	Asc  OrderOperation = "asc"
	Desc OrderOperation = "desc"
)

type OrderBy struct {
	IsOrderBy bool
	Columns   []OrderByColumn
}

type OrderByColumn struct {
	Column    string
	Operation OrderOperation
}

type IfSpec struct {
	IfExists    bool
	IfNotExists bool
}

type UpdateSetResponse struct {
	Assignments []Assignment
}

type BoundSelectColumn interface {
	Column() *Column
}

type BoundIndexColumn struct {
	column *Column
	Index  int
}

func NewBoundIndexColumn(column *Column, index int) *BoundIndexColumn {
	return &BoundIndexColumn{column: column, Index: index}
}

func (b *BoundIndexColumn) Column() *Column {
	return b.column
}

type BoundKeyColumn struct {
	column *Column
	Key    ColumnQualifier
}

func NewBoundKeyColumn(column *Column, key ColumnQualifier) *BoundKeyColumn {
	return &BoundKeyColumn{column: column, Key: key}
}

func (b *BoundKeyColumn) Column() *Column {
	return b.column
}

type IBigtableMutation interface {
	Keyspace() Keyspace
	Table() TableName
	RowKey() RowKey
}

type Operator string

// TODO: we need to move all the constants here.
// Just to keep the code clean, we have defined the constants here.
// It will help in reducing the number of magic strings in the code also to avoid the circular dependency.
const (
	EQ           Operator = "="
	GT           Operator = ">"
	LT           Operator = "<"
	GTE          Operator = ">="
	LTE          Operator = "<="
	BETWEEN      Operator = "BETWEEN"
	LIKE         Operator = "LIKE"
	IN           Operator = "IN"
	CONTAINS     Operator = "CONTAINS"
	CONTAINS_KEY Operator = "CONTAINS KEY"
)

func FlipOperator(op Operator) Operator {
	switch op {
	case GT:
		return LT
	case LT:
		return GT
	case GTE:
		return LTE
	case LTE:
		return LTE
	default:
		return op
	}
}

type ArithmeticOperator string

const (
	PLUS  ArithmeticOperator = "+"
	MINUS ArithmeticOperator = "-"
)

type Condition struct {
	Column   *Column
	Operator Operator
	// points to a placeholder
	Value DynamicValue
	// used for BETWEEN operator
	Value2 DynamicValue
}

type Assignment interface {
	Column() *Column
	IsIdempotentAssignment() bool
	Value() DynamicValue
}

type ComplexAssignmentAppend struct {
	column    *Column
	Operator  ArithmeticOperator
	value     DynamicValue
	IsPrepend bool
}

func (c ComplexAssignmentAppend) Value() DynamicValue {
	return c.value
}

func (c ComplexAssignmentAppend) IsIdempotentAssignment() bool {
	code := c.column.CQLType.Code()
	// list appends are never idempotent
	return c.value.IsIdempotent() && (code == MAP || code == SET)
}

func NewComplexAssignmentAppend(column *Column, op ArithmeticOperator, value DynamicValue, isPrepend bool) Assignment {
	return &ComplexAssignmentAppend{column: column, Operator: op, value: value, IsPrepend: isPrepend}
}

func (c ComplexAssignmentAppend) Column() *Column {
	return c.column
}

type AssignmentCounterIncrement struct {
	column *Column
	Op     ArithmeticOperator
	value  DynamicValue
}

func (c AssignmentCounterIncrement) Value() DynamicValue {
	return c.value
}

func (c AssignmentCounterIncrement) IsIdempotentAssignment() bool {
	return false
}

func NewAssignmentCounterIncrement(column *Column, op ArithmeticOperator, value DynamicValue) Assignment {
	return &AssignmentCounterIncrement{column: column, Op: op, value: value}
}

func (c AssignmentCounterIncrement) Column() *Column {
	return c.column
}

type ComplexAssignmentUpdateListIndex struct {
	column *Column
	// cassandra requires a literal, so no need to handle a parameter for the index
	Index int64
	value DynamicValue
}

func (c ComplexAssignmentUpdateListIndex) Value() DynamicValue {
	return c.value
}

func (c ComplexAssignmentUpdateListIndex) IsIdempotentAssignment() bool {
	return c.value.IsIdempotent()
}

func NewComplexAssignmentUpdateListIndex(column *Column, index int64, value DynamicValue) Assignment {
	return &ComplexAssignmentUpdateListIndex{column: column, Index: index, value: value}
}

func (c ComplexAssignmentUpdateListIndex) Column() *Column {
	return c.column
}

type ComplexAssignmentUpdateMapValue struct {
	column *Column
	// cassandra requires a literal, so no need to handle a parameter for the key
	Key   GoValue
	value DynamicValue
}

func (c ComplexAssignmentUpdateMapValue) Value() DynamicValue {
	return c.value
}

func (c ComplexAssignmentUpdateMapValue) IsIdempotentAssignment() bool {
	return c.value.IsIdempotent()
}

func NewComplexAssignmentUpdateMapValue(column *Column, index GoValue, value DynamicValue) Assignment {
	return &ComplexAssignmentUpdateMapValue{column: column, Key: index, value: value}
}

func (c ComplexAssignmentUpdateMapValue) Column() *Column {
	return c.column
}

type ComplexAssignmentSet struct {
	column *Column
	value  DynamicValue
}

func (c ComplexAssignmentSet) Value() DynamicValue {
	return c.value
}

func (c ComplexAssignmentSet) IsIdempotentAssignment() bool {
	return c.value.IsIdempotent()
}

func NewComplexAssignmentSet(column *Column, value DynamicValue) Assignment {
	return &ComplexAssignmentSet{column: column, value: value}
}

func (c ComplexAssignmentSet) Column() *Column {
	return c.column
}
