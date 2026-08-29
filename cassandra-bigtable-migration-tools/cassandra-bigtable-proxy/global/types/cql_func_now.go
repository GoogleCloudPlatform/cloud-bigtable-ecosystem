package types

import (
	"fmt"
	"github.com/google/uuid"
)

type FuncNow struct {
}

func (f *FuncNow) GetValidClauses() []QueryClause {
	return []QueryClause{QueryClauseWhere, QueryClauseValues}
}

func (f *FuncNow) GetName() string {
	return "now"
}

func (f *FuncNow) GetCode() CqlFuncCode {
	return FuncCodeNow
}

func (f *FuncNow) GetParameterTypes() []CqlFuncParameter {
	return []CqlFuncParameter{}
}

func (f *FuncNow) Apply(_ []DynamicValue, value *QueryParameterValues) (GoValue, error) {
	id, err := uuid.NewUUID()
	if err != nil {
		return nil, err
	}
	if id.Version() != 1 {
		return nil, fmt.Errorf("internal: timeuuid must be v1")
	}
	// sync with query "now"
	setUuidV1Time(value.time, id)
	return id, nil
}

func (f *FuncNow) GetReturnType(_ []DynamicValue) CqlDataType {
	return TypeTimeuuid
}
