package types

import (
	"fmt"

	"github.com/datastax/go-cassandra-native-protocol/datatype"
)

type CqlDataType interface {
	// String returns the canonical CQL string representation of the type.
	String() string

	DataType() datatype.DataType
	// isCDataType is an unexported marker method to ensure only types
	// from this package can implement the interface.
	isCDataType()

	IsCollection() bool
	IsAnyFrozen() bool
	Code() CqlTypeCode
}

type CqlTypeCode int

// Enumeration of all Cassandra scalar types.
const (
	// Scalars
	ASCII CqlTypeCode = iota
	VARCHAR
	BIGINT
	BLOB
	BOOLEAN
	COUNTER
	DATE
	DECIMAL
	DOUBLE
	FLOAT
	INET
	INT
	SMALLINT
	TEXT // Also used for VARCHAR
	TIME
	TIMESTAMP
	TIMEUUID
	TINYINT
	UUID
	VARINT
	// Collections
	LIST
	SET
	MAP
	// Other
	FROZEN
)

// ScalarType represents a primitive, single-value Cassandra type.
type ScalarType struct {
	code CqlTypeCode
	dt   datatype.DataType
	name string
}

func (s ScalarType) Code() CqlTypeCode {
	return s.code
}

func (s ScalarType) IsAnyFrozen() bool {
	return false
}

func (s ScalarType) Kind() CqlTypeCode {
	return s.code
}

func (s ScalarType) DataType() datatype.DataType {
	return s.dt
}

func (s ScalarType) isCDataType() {}

func (s ScalarType) String() string {
	return s.name
}

func (s ScalarType) IsCollection() bool {
	return false
}

func (s ScalarType) IsFrozen() bool {
	return false
}

// Pre-defined constants for common scalar types for convenience.
var (
	TypeAscii     = ScalarType{name: "ascii", code: ASCII, dt: datatype.Varchar}
	TypeVarchar   = ScalarType{name: "varchar", code: VARCHAR, dt: datatype.Varchar}
	TypeBigint    = ScalarType{name: "bigint", code: BIGINT, dt: datatype.Bigint}
	TypeBlob      = ScalarType{name: "blob", code: BLOB, dt: datatype.Blob}
	TypeBoolean   = ScalarType{name: "boolean", code: BOOLEAN, dt: datatype.Boolean}
	TypeCounter   = ScalarType{name: "counter", code: COUNTER, dt: datatype.Counter}
	TypeDate      = ScalarType{name: "date", code: DATE, dt: datatype.Date}
	TypeDecimal   = ScalarType{name: "decimal", code: DECIMAL, dt: datatype.Decimal}
	TypeDouble    = ScalarType{name: "double", code: DOUBLE, dt: datatype.Double}
	TypeFloat     = ScalarType{name: "float", code: FLOAT, dt: datatype.Float}
	TypeInet      = ScalarType{name: "inet", code: INET, dt: datatype.Inet}
	TypeInt       = ScalarType{name: "int", code: INT, dt: datatype.Int}
	TypeSmallint  = ScalarType{name: "smallint", code: SMALLINT, dt: datatype.Smallint}
	TypeText      = ScalarType{name: "text", code: TEXT, dt: datatype.Varchar}
	TypeTime      = ScalarType{name: "time", code: TIME, dt: datatype.Time}
	TypeTimestamp = ScalarType{name: "timestamp", code: TIMESTAMP, dt: datatype.Timestamp}
	TypeTimeuuid  = ScalarType{name: "timeuuid", code: TIMEUUID, dt: datatype.Timeuuid}
	TypeTinyint   = ScalarType{name: "tinyint", code: TINYINT, dt: datatype.Tinyint}
	TypeUuid      = ScalarType{name: "uuid", code: UUID, dt: datatype.Uuid}
	TypeVarint    = ScalarType{name: "varint", code: VARINT, dt: datatype.Varint}
)

type MapType struct {
	keyType   CqlDataType
	valueType CqlDataType
	dt        datatype.DataType
}

func (m MapType) Code() CqlTypeCode {
	return MAP
}

func (m MapType) IsAnyFrozen() bool {
	return m.keyType.IsAnyFrozen() || m.valueType.IsAnyFrozen()
}

func (m MapType) KeyType() CqlDataType {
	return m.keyType
}

func (m MapType) ValueType() CqlDataType {
	return m.valueType
}

func NewMapType(keyType CqlDataType, valueType CqlDataType) *MapType {
	return &MapType{keyType: keyType, valueType: valueType, dt: datatype.NewMapType(keyType.DataType(), valueType.DataType())}
}

func (m MapType) DataType() datatype.DataType {
	return m.dt
}

func (m MapType) isCDataType() {}

func (m MapType) String() string {
	return fmt.Sprintf("map<%s, %s>", m.keyType.String(), m.valueType.String())
}

func (m MapType) IsCollection() bool {
	return true
}

// ListType represents a Cassandra list<elementType>.
type ListType struct {
	elementType CqlDataType
	dt          datatype.DataType
}

func (l ListType) Code() CqlTypeCode {
	return LIST
}

func (l ListType) IsAnyFrozen() bool {
	return l.elementType.IsAnyFrozen()
}

func (l ListType) ElementType() CqlDataType {
	return l.elementType
}

func NewListType(elementType CqlDataType) *ListType {
	return &ListType{elementType: elementType, dt: datatype.NewListType(elementType.DataType())}
}

func (l ListType) DataType() datatype.DataType {
	return l.dt
}

func (l ListType) isCDataType() {}

func (l ListType) String() string {
	return fmt.Sprintf("list<%s>", l.elementType.String())
}

func (l ListType) IsCollection() bool {
	return true
}

// SetType represents a Cassandra set<elementType>.
type SetType struct {
	elementType CqlDataType
	dt          datatype.DataType
}

func (s SetType) Code() CqlTypeCode {
	return SET
}

func (s SetType) IsAnyFrozen() bool {
	return s.elementType.IsAnyFrozen()
}

func NewSetType(elementType CqlDataType) *SetType {
	return &SetType{elementType: elementType, dt: datatype.NewSetType(elementType.DataType())}
}

func (s SetType) DataType() datatype.DataType {
	return s.dt
}

func (s SetType) ElementType() CqlDataType {
	return s.elementType
}

func (s SetType) isCDataType() {}

func (s SetType) String() string {
	return fmt.Sprintf("set<%s>", s.elementType.String())
}

func (s SetType) IsCollection() bool {
	return true
}

type FrozenType struct {
	innerType CqlDataType
}

func (f FrozenType) Code() CqlTypeCode {
	return FROZEN
}

func (f FrozenType) IsAnyFrozen() bool {
	return true
}

func (f FrozenType) InnerType() CqlDataType {
	return f.innerType
}

func (f FrozenType) IsCollection() bool {
	return f.innerType.IsCollection()
}

func (f FrozenType) DataType() datatype.DataType {
	return f.innerType.DataType()
}

func (f FrozenType) isCDataType() {}

func (f FrozenType) String() string {
	return fmt.Sprintf("frozen<%s>", f.innerType.String())
}

func NewFrozenType(inner CqlDataType) *FrozenType {
	return &FrozenType{innerType: inner}
}
