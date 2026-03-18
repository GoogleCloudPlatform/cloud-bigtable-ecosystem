package types

type FuncAvg struct {
}

func (f *FuncAvg) GetValidClauses() []QueryClause {
	return []QueryClause{QueryClauseSelect}
}

func (f *FuncAvg) GetName() string {
	return "avg"
}

func (f *FuncAvg) GetCode() CqlFuncCode {
	return FuncCodeAvg
}

func (f *FuncAvg) GetParameterTypes() []CqlFuncParameter {
	return []CqlFuncParameter{
		*NewCqlFuncParameter(AllNumericTypes, true, false),
	}
}

func (f *FuncAvg) Apply(_ []DynamicValue, value *QueryParameterValues) (GoValue, error) {
	return nil, nil
}

func (f *FuncAvg) GetReturnType(args []DynamicValue) CqlDataType {
	if len(args) != 1 {
		return TypeDouble
	}
	dt := args[0].GetType()
	if dt.Code() == INT || dt.Code() == BIGINT {
		return TypeFloat
	}
	return dt
}
