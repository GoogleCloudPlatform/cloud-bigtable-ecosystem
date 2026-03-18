package types

import (
	"encoding/binary"
	"fmt"
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
	FuncCodeMinTimeuuid
	FuncCodeMaxTimeuuid
	FuncCodeUUID
)

type CqlFunc interface {
	GetName() string
	GetCode() CqlFuncCode
	GetValidClauses() []QueryClause
	GetParameterTypes() []CqlFuncParameter
	Apply(args []DynamicValue, value *QueryParameterValues) (GoValue, error)
	GetReturnType(args []DynamicValue) CqlDataType
}

type CqlFuncParameter struct {
	Types       []CqlDataType
	AllowColumn bool
	AllowStar   bool
}

func NewCqlFuncParameter(Types []CqlDataType, allowColumn bool, allowStar bool) *CqlFuncParameter {
	return &CqlFuncParameter{Types: Types, AllowColumn: allowColumn, AllowStar: allowStar}
}

var (
	WriteTime   CqlFunc = &FuncWritetime{}
	Count       CqlFunc = &FuncCount{}
	Avg         CqlFunc = &FuncAvg{}
	Sum         CqlFunc = &FuncSum{}
	Min         CqlFunc = &FuncMin{}
	Max         CqlFunc = &FuncMax{}
	Now         CqlFunc = &FuncNow{}
	ToTimestamp CqlFunc = &FuncToTimestamp{}
	MinTimeuuid CqlFunc = &FuncMinTimeuuid{}
	MaxTimeuuid CqlFunc = &FuncMaxTimeuuid{}
)

func GetFunc(c CqlFuncCode) (CqlFunc, error) {
	switch c {
	case FuncCodeWriteTime:
		return WriteTime, nil
	case FuncCodeCount:
		return Count, nil
	case FuncCodeAvg:
		return Avg, nil
	case FuncCodeSum:
		return Sum, nil
	case FuncCodeMin:
		return Min, nil
	case FuncCodeMax:
		return Max, nil
	case FuncCodeNow:
		return Now, nil
	case FuncCodeToTimestamp:
		return ToTimestamp, nil
	case FuncCodeMinTimeuuid:
		return MinTimeuuid, nil
	case FuncCodeMaxTimeuuid:
		return MaxTimeuuid, nil
	default:
		return nil, fmt.Errorf("unknown func")
	}
}

func setUuidV1Time(t time.Time, id uuid.UUID) uuid.UUID {
	const epochOffset = 122192928000000000

	// Convert Unix time to 100-ns intervals
	unixIntervals := t.Unix()*10000000 + int64(t.Nanosecond()/100)

	// Add the offset
	uuidTime := uint64(unixIntervals + epochOffset)

	// 2. Breakdown the 60-bit timestamp into fields
	// UUID v1 Layout:
	//   Time Low (32 bits)
	//   Time Mid (16 bits)
	//   Time High & Version (16 bits)

	timeLow := uint32(uuidTime & 0xFFFFFFFF)
	timeMid := uint16((uuidTime >> 32) & 0xFFFF)
	timeHi := uint16((uuidTime >> 48) & 0x0FFF)

	// 3. Write bytes into the UUID structure (Big Endian)

	// Bytes 0-3: Time Low
	binary.BigEndian.PutUint32(id[0:4], timeLow)

	// Bytes 4-5: Time Mid
	binary.BigEndian.PutUint16(id[4:6], timeMid)

	// Bytes 6-7: Time High + Version
	// We combine the high time bits with the Version 1 identifier (0x1000)
	binary.BigEndian.PutUint16(id[6:8], timeHi|0x1000)
	return id
}

func getUuidArg(index int, args []DynamicValue, values *QueryParameterValues) (uuid.UUID, error) {
	value, err := args[index].GetValue(values)
	if err != nil {
		return [16]byte{}, err
	}

	u, ok := value.(uuid.UUID)
	if !ok {
		return [16]byte{}, fmt.Errorf("invalid argment: %T expected uuid", value)
	}
	return u, nil
}

func getTimeArg(index int, args []DynamicValue, values *QueryParameterValues) (time.Time, error) {
	value, err := args[index].GetValue(values)
	if err != nil {
		return time.Time{}, err
	}

	u, ok := value.(time.Time)
	if !ok {
		return time.Time{}, fmt.Errorf("invalid argment: %T expected time.Time", value)
	}
	return u, nil
}

func ValidateFunctionArgs(f *FunctionValue) error {
	if len(f.Func.GetParameterTypes()) != len(f.Args) {
		return fmt.Errorf("function '%s' expects %d args but got %d", f.Func.GetName(), len(f.Func.GetParameterTypes()), len(f.Args))
	}

	for i, arg := range f.Args {
		param := f.Func.GetParameterTypes()[i]
		star := IsSelectStar(arg)
		if !param.AllowStar && star {
			return fmt.Errorf("function '%s' doesn't allow '*' args for parameter %d", f.Func.GetName(), i)
		}
		if !param.AllowColumn && IsColumn(arg) {
			return fmt.Errorf("function '%s' doesn't allow column args for parameter %d", f.Func.GetName(), i)
		}

		// star is allowed so it's ok
		if star {
			continue
		}

		isValidType := false
		for _, paramType := range param.Types {
			if paramType == arg.GetType() {
				isValidType = true
				break
			}
		}
		if !isValidType {
			return fmt.Errorf("function '%s' parameter %d doesn't accept type %s", f.Func.GetName(), i, arg.GetType().String())
		}
	}
	return nil
}
