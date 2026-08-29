package types

type FuncMin struct {
}

func (f *FuncMin) GetValidClauses() []QueryClause {
	return []QueryClause{QueryClauseSelect}
}

func (f *FuncMin) GetName() string {
	return "min"
}

func (f *FuncMin) GetCode() CqlFuncCode {
	return FuncCodeMin
}

func (f *FuncMin) GetParameterTypes() []CqlFuncParameter {
	return []CqlFuncParameter{
		*NewCqlFuncParameter(AllNumericTypes, true, false),
	}
}

func (f *FuncMin) Apply(_ []DynamicValue, value *QueryParameterValues) (GoValue, error) {
	return nil, nil
}

func (f *FuncMin) GetReturnType(args []DynamicValue) CqlDataType {
	if len(args) != 1 {
		return TypeDouble
	}
	return args[0].GetType()
}
