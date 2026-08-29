package types

type FuncMax struct {
}

func (f *FuncMax) GetValidClauses() []QueryClause {
	return []QueryClause{QueryClauseSelect}
}

func (f *FuncMax) GetName() string {
	return "max"
}

func (f *FuncMax) GetCode() CqlFuncCode {
	return FuncCodeMax
}

func (f *FuncMax) GetParameterTypes() []CqlFuncParameter {
	return []CqlFuncParameter{
		*NewCqlFuncParameter(AllNumericTypes, true, false),
	}
}

func (f *FuncMax) Apply(_ []DynamicValue, value *QueryParameterValues) (GoValue, error) {
	return nil, nil
}

func (f *FuncMax) GetReturnType(args []DynamicValue) CqlDataType {
	if len(args) != 1 {
		return TypeDouble
	}
	return args[0].GetType()
}
