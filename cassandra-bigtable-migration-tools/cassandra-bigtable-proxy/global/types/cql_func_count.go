package types

type FuncCount struct {
}

func (f *FuncCount) GetValidClauses() []QueryClause {
	return []QueryClause{QueryClauseSelect}
}

func (f *FuncCount) GetName() string {
	return "count"
}

func (f *FuncCount) GetCode() CqlFuncCode {
	return FuncCodeCount
}

func (f *FuncCount) GetParameterTypes() []CqlFuncParameter {
	return []CqlFuncParameter{
		*NewCqlFuncParameter(AllScalarTypes, true, true),
	}
}

func (f *FuncCount) Apply(_ []DynamicValue, _ *QueryParameterValues) (GoValue, error) {
	return nil, nil
}

func (f *FuncCount) GetReturnType(_ []DynamicValue) CqlDataType {
	return TypeBigInt
}
