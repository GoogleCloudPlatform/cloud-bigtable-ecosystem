package types

type FuncSum struct {
}

func (f *FuncSum) GetValidClauses() []QueryClause {
	return []QueryClause{QueryClauseSelect}
}

func (f *FuncSum) GetName() string {
	return "sum"
}

func (f *FuncSum) GetCode() CqlFuncCode {
	return FuncCodeSum
}

func (f *FuncSum) GetParameterTypes() []CqlFuncParameter {
	return []CqlFuncParameter{
		*NewCqlFuncParameter(AllNumericTypes, true, false),
	}
}

func (f *FuncSum) Apply(_ []DynamicValue, value *QueryParameterValues) (GoValue, error) {
	return nil, nil
}

func (f *FuncSum) GetReturnType(args []DynamicValue) CqlDataType {
	if len(args) != 1 {
		return TypeDouble
	}
	return args[0].GetType()
}
