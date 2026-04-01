package types

type FuncWritetime struct {
}

func (f *FuncWritetime) GetValidClauses() []QueryClause {
	return []QueryClause{QueryClauseSelect}
}

func (f *FuncWritetime) GetName() string {
	return "writetime"
}

func (f *FuncWritetime) GetCode() CqlFuncCode {
	return FuncCodeWriteTime
}

func (f *FuncWritetime) GetParameterTypes() []CqlFuncParameter {
	return []CqlFuncParameter{
		// todo what about collections?
		*NewCqlFuncParameter(AllScalarTypes, true, false),
	}
}

func (f *FuncWritetime) Apply(_ []DynamicValue, _ *QueryParameterValues) (GoValue, error) {
	return nil, nil
}

func (f *FuncWritetime) GetReturnType(_ []DynamicValue) CqlDataType {
	return TypeBigInt
}
