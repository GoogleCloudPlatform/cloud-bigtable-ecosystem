package types

import (
	"encoding/binary"
	"fmt"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/google/uuid"
	"time"
)

type CqlFuncCode int

const (
	FuncCodeUnknown CqlFuncCode = iota
	FuncCodeWriteTime
	FuncCodeCount
	FuncCodeAvg
	FuncCodeSum
	FuncCodeMin
	FuncCodeMax
	FuncCodeNow
	FuncCodeToTimestamp
)

func (c CqlFuncCode) String() string {
	switch c {
	case FuncCodeWriteTime:
		return "writetime"
	case FuncCodeCount:
		return "count"
	case FuncCodeAvg:
		return "avg"
	case FuncCodeSum:
		return "sum"
	case FuncCodeMin:
		return "min"
	case FuncCodeMax:
		return "max"
	case FuncCodeNow:
		return "now"
	case FuncCodeToTimestamp:
		return "totimestamp"
	default:
		return "unknown"
	}
}

type CqlFuncSpec struct {
	Code           CqlFuncCode
	ParameterTypes []CqlDataType
	ReturnType     CqlDataType
	Apply          func(args []GoValue) (GoValue, error)
}

func NewCqlFuncSpec(code CqlFuncCode, parameterTypes []CqlDataType, returnType CqlDataType, apply func(args []GoValue) (GoValue, error)) *CqlFuncSpec {
	return &CqlFuncSpec{Code: code, ParameterTypes: parameterTypes, ReturnType: returnType, Apply: apply}
}

var (
	FuncNow         = NewCqlFuncSpec(FuncCodeNow, nil, TypeTimeuuid, now)
	FuncToTimestamp = NewCqlFuncSpec(FuncCodeToTimestamp, []CqlDataType{TypeTimeuuid}, TypeTimestamp, toTimestamp)
)

func GetCqlFunc(code CqlFuncCode) (*CqlFuncSpec, error) {
	switch code {
	//case FuncCodeUnknown:
	//	return FuncUnknown, nil
	//case FuncCodeWriteTime:
	//	return FuncWriteTime, nil
	//case FuncCodeCount:
	//	return FuncCount, nil
	//case FuncCodeAvg:
	//	return FuncAvg, nil
	//case FuncCodeSum:
	//	return FuncSum, nil
	//case FuncCodeMin:
	//	return FuncMin, nil
	//case FuncCodeMax:
	//	return FuncMax, nil
	case FuncCodeNow:
		return FuncNow, nil
	case FuncCodeToTimestamp:
		return FuncToTimestamp, nil
	default:
		return nil, fmt.Errorf("unknown function code %s (%d)", code.String(), code)
	}
}

func now(_ []GoValue) (GoValue, error) {
	u, err := uuid.NewUUID()
	if err != nil {
		return nil, err
	}
	return primitive.UUID(u), nil
}
func toTimestamp(args []GoValue) (GoValue, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("invalid argments")
	}

	p, ok := args[0].(primitive.UUID)
	if !ok {
		return nil, fmt.Errorf("invalid argment: %T expected primitive.UUID", args[0])
	}
	u := uuid.UUID(p)
	if u.Version() == 1 {
		sec, nsec := u.Time().UnixTime()
		return time.Unix(sec, nsec).UTC(), nil
	}
	if u.Version() == 7 {
		return GetTimeFromUUIDv7(p)
	}
	return nil, fmt.Errorf("unsupported uuid version: %d", u.Version())
}

func GetTimeFromUUIDv7(p primitive.UUID) (time.Time, error) {
	id := uuid.UUID(p)
	// 1. Check the UUID version. UUIDv7 has the '7' in the 4 most significant bits
	//    of the 7th byte (index 6). This is a good sanity check.
	if id.Version() != 7 {
		return time.Time{}, fmt.Errorf("uuid version is %d, expected 7", id.Version())
	}

	// UUIDv7 structure:
	// Bits 0-47: 48-bit Unix Epoch time in milliseconds (6 bytes)
	// Bits 48-51: 4-bit version (7)
	// ... rest is reserved/random
	timestamp64 := binary.BigEndian.Uint64(id[:8])

	// 3. Shift right by 16 bits to discard the version/reserved bits and move
	//    the 48-bit millisecond timestamp to the least significant position.
	milliseconds := int64(timestamp64 >> 16)

	// 4. Convert milliseconds to time.
	// time.Unix takes seconds and nanoseconds, so we need to divide the milliseconds.
	seconds := milliseconds / 1000
	nanos := (milliseconds % 1000) * int64(time.Millisecond) // Remaining millis * 1,000,000 nanosec/millis

	return time.Unix(seconds, nanos).UTC(), nil
}
