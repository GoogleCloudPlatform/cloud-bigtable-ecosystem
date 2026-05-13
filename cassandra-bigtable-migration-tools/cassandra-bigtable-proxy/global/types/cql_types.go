package types

import (
	"cloud.google.com/go/bigtable"
	"fmt"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"net"
	"reflect"
	"time"
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
	BigtableStorageType() CqlDataType
	BigtableSqlType() bigtable.SQLType
	GoType() reflect.Type
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
	code      CqlTypeCode
	dt        datatype.DataType
	name      string
	btSqlType bigtable.SQLType
	goType    reflect.Type
}

func (s ScalarType) BigtableSqlType() bigtable.SQLType {
	return s.btSqlType
}

func newScalarType(name string, code CqlTypeCode, dt datatype.DataType, goType reflect.Type, btSqlType bigtable.SQLType) *ScalarType {
	return &ScalarType{code: code, dt: dt, name: name, goType: goType, btSqlType: btSqlType}
}

func (s ScalarType) GoType() reflect.Type {
	return s.goType
}

func (s ScalarType) BigtableStorageType() CqlDataType {
	switch s.code {
	case BOOLEAN:
		return TypeBigInt
	case INT, TINYINT:
		return TypeBigInt
	default:
		return s
	}
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
	TypeAscii     CqlDataType = newScalarType("ascii", ASCII, datatype.Ascii, reflect.TypeOf(""), bigtable.StringSQLType{})
	TypeVarchar   CqlDataType = newScalarType("varchar", VARCHAR, datatype.Varchar, reflect.TypeOf(""), bigtable.StringSQLType{})
	TypeBigInt    CqlDataType = newScalarType("bigint", BIGINT, datatype.Bigint, reflect.TypeOf(int64(0)), bigtable.Int64SQLType{})
	TypeBlob      CqlDataType = newScalarType("blob", BLOB, datatype.Blob, reflect.TypeOf(""), bigtable.BytesSQLType{})
	TypeBoolean   CqlDataType = newScalarType("boolean", BOOLEAN, datatype.Boolean, reflect.TypeOf(false), bigtable.Int64SQLType{})
	TypeCounter   CqlDataType = newScalarType("counter", COUNTER, datatype.Counter, reflect.TypeOf(int64(0)), bigtable.Int64SQLType{})
	TypeDate      CqlDataType = newScalarType("date", DATE, datatype.Date, reflect.TypeOf(time.Time{}), bigtable.TimestampSQLType{})
	TypeDecimal   CqlDataType = newScalarType("decimal", DECIMAL, datatype.Decimal, reflect.TypeOf(0.0), bigtable.Float64SQLType{})
	TypeDouble    CqlDataType = newScalarType("double", DOUBLE, datatype.Double, reflect.TypeOf(float64(0.0)), bigtable.Float64SQLType{})
	TypeFloat     CqlDataType = newScalarType("float", FLOAT, datatype.Float, reflect.TypeOf(float32(0.0)), bigtable.Float32SQLType{})
	TypeInet      CqlDataType = newScalarType("inet", INET, datatype.Inet, reflect.TypeOf(net.IP{}), bigtable.StringSQLType{})
	TypeInt       CqlDataType = newScalarType("int", INT, datatype.Int, reflect.TypeOf(int32(1)), bigtable.Int64SQLType{})
	TypeSmallint  CqlDataType = newScalarType("smallint", SMALLINT, datatype.Smallint, reflect.TypeOf(int16(1)), bigtable.Int64SQLType{})
	TypeText      CqlDataType = newScalarType("text", TEXT, datatype.Varchar, reflect.TypeOf(""), bigtable.StringSQLType{})
	TypeTime      CqlDataType = newScalarType("time", TIME, datatype.Time, reflect.TypeOf(time.Time{}), bigtable.TimestampSQLType{})
	TypeTimestamp CqlDataType = newScalarType("timestamp", TIMESTAMP, datatype.Timestamp, reflect.TypeOf(time.Time{}), bigtable.TimestampSQLType{})
	TypeTimeuuid  CqlDataType = newScalarType("timeuuid", TIMEUUID, datatype.Timeuuid, reflect.TypeOf(primitive.UUID{}), bigtable.BytesSQLType{})
	TypeTinyint   CqlDataType = newScalarType("tinyint", TINYINT, datatype.Tinyint, reflect.TypeOf(int8(1)), bigtable.Int64SQLType{})
	TypeUuid      CqlDataType = newScalarType("uuid", UUID, datatype.Uuid, reflect.TypeOf(primitive.UUID{}), bigtable.BytesSQLType{})
	TypeVarint    CqlDataType = newScalarType("varint", VARINT, datatype.Varint, reflect.TypeOf(1), bigtable.StringSQLType{})
)

type MapType struct {
	keyType   CqlDataType
	valueType CqlDataType
	dt        datatype.DataType
	goType    reflect.Type
}

func (m MapType) BigtableSqlType() bigtable.SQLType {
	return bigtable.MapSQLType{
		KeyType:   m.keyType.BigtableSqlType(),
		ValueType: m.valueType.BigtableSqlType(),
	}
}

func (m MapType) GoType() reflect.Type {
	return m.goType
}

func (m MapType) BigtableStorageType() CqlDataType {
	kt := m.keyType.BigtableStorageType()
	vt := m.valueType.BigtableStorageType()
	if m.keyType == kt && m.valueType == vt {
		return m
	}
	return NewMapType(kt, vt)
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
	return &MapType{
		keyType:   keyType,
		valueType: valueType,
		dt:        datatype.NewMapType(keyType.DataType(), valueType.DataType()),
		goType:    reflect.MapOf(keyType.GoType(), valueType.GoType()),
	}
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
	goType      reflect.Type
}

func (l ListType) BigtableSqlType() bigtable.SQLType {
	return bigtable.ArraySQLType{ElemType: l.elementType.BigtableSqlType()}
}

func (l ListType) GoType() reflect.Type {
	return l.goType
}

func (l ListType) BigtableStorageType() CqlDataType {
	dt := l.elementType.BigtableStorageType()
	if dt != l.elementType {
		return NewListType(dt)
	}
	return l
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
	return &ListType{
		elementType: elementType,
		dt:          datatype.NewListType(elementType.DataType()),
		goType:      reflect.SliceOf(elementType.GoType()),
	}
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
	goType      reflect.Type
}

func (s SetType) BigtableSqlType() bigtable.SQLType {
	return bigtable.ArraySQLType{ElemType: s.elementType.BigtableSqlType()}
}

func (s SetType) GoType() reflect.Type {
	return s.goType
}

func (s SetType) BigtableStorageType() CqlDataType {
	dt := s.elementType.BigtableStorageType()
	if dt != s.elementType {
		return NewSetType(dt)
	}
	return s
}

func (s SetType) Code() CqlTypeCode {
	return SET
}

func (s SetType) IsAnyFrozen() bool {
	return s.elementType.IsAnyFrozen()
}

func NewSetType(elementType CqlDataType) *SetType {
	return &SetType{
		elementType: elementType,
		dt:          datatype.NewSetType(elementType.DataType()),
		goType:      reflect.SliceOf(elementType.GoType()),
	}
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

func (f FrozenType) BigtableSqlType() bigtable.SQLType {
	return bigtable.BytesSQLType{}
}

func (f FrozenType) GoType() reflect.Type {
	return f.innerType.GoType()
}

func (f FrozenType) BigtableStorageType() CqlDataType {
	return f
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
	return false
}

func (f FrozenType) DataType() datatype.DataType {
	return f.innerType.DataType()
}

func (f FrozenType) isCDataType() {}

func (f FrozenType) String() string {
	return fmt.Sprintf("frozen<%s>", f.innerType.String())
}

func NewFrozenType(inner CqlDataType) *FrozenType {
	return &FrozenType{
		innerType: inner,
	}
}
