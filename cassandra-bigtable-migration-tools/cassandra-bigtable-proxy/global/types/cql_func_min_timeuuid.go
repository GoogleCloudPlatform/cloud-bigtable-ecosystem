package types

import (
	"github.com/google/uuid"
	"time"
)

type FuncMinTimeuuid struct {
}

func (f *FuncMinTimeuuid) GetValidClauses() []QueryClause {
	return []QueryClause{QueryClauseWhere, QueryClauseValues}
}

func (f *FuncMinTimeuuid) GetName() string {
	return "minTimeuuid"
}

func (f *FuncMinTimeuuid) GetCode() CqlFuncCode {
	return FuncCodeMinTimeuuid
}

func (f *FuncMinTimeuuid) GetParameterTypes() []CqlFuncParameter {
	return []CqlFuncParameter{
		*NewCqlFuncParameter(
			[]CqlDataType{
				TypeTimestamp,
			},
			false,
			false),
	}
}

func (f *FuncMinTimeuuid) Apply(args []DynamicValue, values *QueryParameterValues) (GoValue, error) {
	u, err := getTimeArg(0, args, values)
	if err != nil {
		return nil, err
	}
	return minUUIDv1ForTime(u)
}

func (f *FuncMinTimeuuid) GetReturnType(args []DynamicValue) CqlDataType {
	return TypeTimeuuid
}

func minUUIDv1ForTime(t time.Time) (uuid.UUID, error) {
	u := setUuidV1Time(t, [16]byte{})

	// --- Clock Sequence & Node ID Fields (Minned Values) ---

	// clock_seq_hi_and_variant (byte 8, 8 bits)
	// The variant bits must be 10 (0x80 to 0xBF). We set the clock sequence to min (0).
	// 0x80 is 0b10000000 (variant 10 and min 6 bits for the clock sequence high part)
	u[8] = 0x80

	// clock_seq_low (byte 9, 8 bits)
	u[9] = 0x80 // Min 8 bits

	// Node ID (bytes 10-15, 48 bits) - set to all 0's (min)
	u[10] = 0x80
	u[11] = 0x80
	u[12] = 0x80
	u[13] = 0x80
	u[14] = 0x80
	u[15] = 0x80

	// Create the UUID from the byte array
	return u, nil
}
