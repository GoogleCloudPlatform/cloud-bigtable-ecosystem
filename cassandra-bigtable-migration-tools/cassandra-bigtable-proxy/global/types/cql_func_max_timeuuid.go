package types

import (
	"github.com/google/uuid"
	"time"
)

type FuncMaxTimeuuid struct {
}

func (f *FuncMaxTimeuuid) GetValidClauses() []QueryClause {
	return []QueryClause{QueryClauseWhere, QueryClauseValues}
}

func (f *FuncMaxTimeuuid) GetName() string {
	return "maxTimeuuid"
}

func (f *FuncMaxTimeuuid) GetCode() CqlFuncCode {
	return FuncCodeMaxTimeuuid
}

func (f *FuncMaxTimeuuid) GetParameterTypes() []CqlFuncParameter {
	return []CqlFuncParameter{
		*NewCqlFuncParameter(
			[]CqlDataType{
				TypeTimestamp,
			},
			false,
			false),
	}
}

func (f *FuncMaxTimeuuid) Apply(args []DynamicValue, values *QueryParameterValues) (GoValue, error) {
	u, err := getTimeArg(0, args, values)
	if err != nil {
		return nil, err
	}
	return maxUUIDv1ForTime(u)
}

func (f *FuncMaxTimeuuid) GetReturnType(args []DynamicValue) CqlDataType {
	return TypeTimeuuid
}

// maxUUIDv1ForTime creates the maximum possible UUIDv1 for a given time.Time.
// This is achieved by setting the timestamp fields based on the input time,
// and setting the Clock Sequence and Node ID fields to their maximum values.
func maxUUIDv1ForTime(t time.Time) (uuid.UUID, error) {
	// For maxTimeuuid, Cassandra adds 9999 to the 100-nanosecond intervals timestamp.
	// 9999 * 100 nanoseconds.
	maxT := t.Add(9999 * 100 * time.Nanosecond)
	u := setUuidV1Time(maxT, [16]byte{})

	// --- Clock Sequence & Node ID Fields (Maxed Values) ---
	// clock_seq_hi_and_variant (byte 8, 8 bits)
	// The variant bits must be 10 (0x80 to 0xBF). We set the clock sequence to max.
	// 0xBF is 0b10111111 (variant 10 and max 6 bits for the clock sequence high part)
	u[8] = 0xBF

	// clock_seq_low (byte 9, 8 bits)
	u[9] = 0xFF

	// Node ID (bytes 10-15, 48 bits) - set to all F's (max)
	u[10] = 0xFF
	u[11] = 0xFF
	u[12] = 0xFF
	u[13] = 0xFF
	u[14] = 0xFF
	u[15] = 0xFF

	// Create the UUID from the byte array
	return u, nil
}
