package types

import (
	"github.com/google/uuid"
	"time"
)

type FuncToTimestamp struct {
}

func (f *FuncToTimestamp) GetValidClauses() []QueryClause {
	return []QueryClause{QueryClauseWhere, QueryClauseValues}
}

func (f *FuncToTimestamp) GetName() string {
	return "toTimestamp"
}

func (f *FuncToTimestamp) GetCode() CqlFuncCode {
	return FuncCodeToTimestamp
}

func (f *FuncToTimestamp) GetParameterTypes() []CqlFuncParameter {
	return []CqlFuncParameter{
		*NewCqlFuncParameter(
			[]CqlDataType{
				TypeTimeuuid,
			},
			false,
			false,
		),
	}
}

func (f *FuncToTimestamp) Apply(args []DynamicValue, values *QueryParameterValues) (GoValue, error) {
	u, err := getUuidArg(0, args, values)
	if err != nil {
		return nil, err
	}
	return getTimeFromUUID(u)
}

func (f *FuncToTimestamp) GetReturnType(_ []DynamicValue) CqlDataType {
	return TypeTimeuuid
}

func getTimeFromUUID(id uuid.UUID) (time.Time, error) {
	sec, nsec := id.Time().UnixTime()
	return time.Unix(sec, nsec).UTC(), nil
}
