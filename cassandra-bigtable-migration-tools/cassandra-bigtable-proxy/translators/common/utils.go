package common

import (
	"errors"
	"fmt"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	schemaMapping "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/metadata"
	cql "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/third_party/cqlparser"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/third_party/datastax/proxycore"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/utilities"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/google/uuid"
	"regexp"
	"strings"
	"time"
)

var (
	validTableName = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
)

func ParseArithmeticOperator(a cql.IArithmeticOperatorContext) (types.ArithmeticOperator, error) {
	if a == nil {
		return "", fmt.Errorf("nil arithmetic operator")
	}
	if a.PLUS() != nil {
		return types.PLUS, nil
	} else if a.MINUS() != nil {
		return types.MINUS, nil
	}
	return "", fmt.Errorf("unsupported arithmetic operator: `%s`", a.GetText())
}

func ExtractDecimalLiteral(d cql.IDecimalLiteralContext, cqlType types.CqlDataType) (types.DynamicValue, error) {
	if d == nil {
		return nil, fmt.Errorf("decimal literal missing")
	}
	val, err := GetDecimalLiteral(d, cqlType)
	if err != nil {
		return nil, err
	}
	return types.NewLiteralValue(val), err
}

func ParseMarker(m cql.IMarkerContext, dt types.CqlDataType, params *types.QueryParameterBuilder, col *types.Column) (types.DynamicValue, error) {
	var marker types.Parameter = ""
	var err error
	if m.NAMED_MARK() != nil {
		// named markers start with colons in CQL - we need to drop them
		marker = types.Parameter(strings.TrimPrefix(m.NAMED_MARK().GetText(), ":"))
		err = params.AddNamedParam(marker, dt)
	} else {
		marker, err = params.AddPositionalParam(dt, col)
	}
	if err != nil {
		return nil, err
	}
	return types.NewParameterizedValue(marker), nil
}

func GetDecimalLiteral(d cql.IDecimalLiteralContext, cqlType types.CqlDataType) (types.GoValue, error) {
	if d.DECIMAL_LITERAL() == nil {
		return nil, fmt.Errorf("missing decimal literal value")
	}
	return utilities.StringToGo(d.DECIMAL_LITERAL().GetText(), cqlType)
}

func ParseBigInt(d cql.IDecimalLiteralContext) (int64, error) {
	if d == nil || d.DECIMAL_LITERAL() == nil {
		return 0, fmt.Errorf("missing decimal literal value")
	}
	return utilities.ParseBigInt(d.GetText())
}

func ParseOperator(op cql.ICompareOperatorContext) (types.Operator, error) {
	if op == nil {
		return "", fmt.Errorf("nil operator")
	}
	if op.OPERATOR_EQ() != nil {
		return types.EQ, nil
	}
	if op.OPERATOR_LT() != nil {
		return types.LT, nil
	}
	if op.OPERATOR_GT() != nil {
		return types.GT, nil
	}
	if op.OPERATOR_LTE() != nil {
		return types.LTE, nil
	}
	if op.OPERATOR_GTE() != nil {
		return types.GTE, nil
	}
	return "", fmt.Errorf("unknown operator type: `%s`", op.GetText())
}

func ParseWhereClause(input cql.IWhereSpecContext, tableConfig *schemaMapping.TableSchema, params *types.QueryParameterBuilder) ([]types.Condition, error) {
	if input == nil {
		return nil, nil
	}

	var conditions []types.Condition
	for _, val := range input.RelationElements().AllRelationElement() {
		var err error
		var condition types.Condition
		if val.RelationCompare() != nil {
			condition, err = parseWhereCompare(val.RelationCompare(), tableConfig, params)
		} else if val.RelationLike() != nil {
			condition, err = parseWhereLike(val.RelationLike(), tableConfig, params)
		} else if val.RelationContainsKey() != nil {
			condition, err = parseWhereContainsKey(val.RelationContainsKey(), tableConfig, params)
		} else if val.RelationContains() != nil {
			condition, err = parseWhereContains(val.RelationContains(), tableConfig, params)
		} else if val.RelationBetween() != nil {
			condition, err = parseWhereBetween(val.RelationBetween(), tableConfig, params)
		} else if val.RelationIn() != nil {
			condition, err = parseWhereIn(val.RelationIn(), tableConfig, params)
		} else {
			return nil, fmt.Errorf("unsupported condition type: `%s`", val.GetText())
		}
		if err != nil {
			return nil, err
		}
		conditions = append(conditions, condition)
	}
	return conditions, nil
}

func parseWhereCompare(compare cql.IRelationCompareContext, tableConfig *schemaMapping.TableSchema, params *types.QueryParameterBuilder) (types.Condition, error) {
	op, err := ParseOperator(compare.CompareOperator())
	if err != nil {
		return types.Condition{}, err
	}

	column, err := ParseColumnContext(tableConfig, compare.Column())
	if err != nil {
		return types.Condition{}, err
	}

	value, err := ParseConstantValue(compare.Constant(), column.CQLType, params, column)
	if err != nil {
		return types.Condition{}, err
	}

	return types.Condition{
		Column:   column,
		Operator: op,
		Value:    value,
	}, nil
}
func parseWhereLike(like cql.IRelationLikeContext, tableConfig *schemaMapping.TableSchema, params *types.QueryParameterBuilder) (types.Condition, error) {
	column, err := ParseColumnContext(tableConfig, like.Column())
	if err != nil {
		return types.Condition{}, err
	}
	value, err := ParseConstantValue(like.Constant(), column.CQLType, params, column)
	if err != nil {
		return types.Condition{}, err
	}
	return types.Condition{
		Column:   column,
		Operator: types.LIKE,
		Value:    value,
	}, nil
}
func parseWhereContainsKey(containsKey cql.IRelationContainsKeyContext, tableConfig *schemaMapping.TableSchema, params *types.QueryParameterBuilder) (types.Condition, error) {
	column, err := ParseColumnContext(tableConfig, containsKey.Column())
	if err != nil {
		return types.Condition{}, err
	}
	if column.CQLType.Code() != types.MAP {
		return types.Condition{}, errors.New("CONTAINS KEY are only supported for map")
	}
	keyType := column.CQLType.(*types.MapType).KeyType()
	value, err := ParseConstantValue(containsKey.Constant(), keyType, params, column)
	if err != nil {
		return types.Condition{}, err
	}
	return types.Condition{
		Column:   column,
		Operator: types.CONTAINS_KEY,
		Value:    value,
	}, nil
}

func parseWhereContains(contains cql.IRelationContainsContext, tableConfig *schemaMapping.TableSchema, params *types.QueryParameterBuilder) (types.Condition, error) {
	column, err := ParseColumnContext(tableConfig, contains.Column())
	if err != nil {
		return types.Condition{}, err
	}
	var elementType types.CqlDataType
	if column.CQLType.Code() == types.LIST {
		elementType = column.CQLType.(*types.ListType).ElementType()
	} else if column.CQLType.Code() == types.SET {
		elementType = column.CQLType.(*types.SetType).ElementType()
	} else {
		return types.Condition{}, fmt.Errorf("CONTAINS are only supported for set and list type columns, not %s", column.CQLType.String())
	}

	value, err := ParseConstantValue(contains.Constant(), elementType, params, column)
	if err != nil {
		return types.Condition{}, err
	}
	return types.Condition{
		Column:   column,
		Operator: types.CONTAINS,
		Value:    value,
	}, nil
}

func parseWhereBetween(between cql.IRelationBetweenContext, tableConfig *schemaMapping.TableSchema, params *types.QueryParameterBuilder) (types.Condition, error) {
	column, err := ParseColumnContext(tableConfig, between.Column())
	if err != nil {
		return types.Condition{}, err
	}

	if len(between.AllConstant()) != 2 {
		return types.Condition{}, fmt.Errorf("BETWEEN condition must have exactly 2 values")
	}

	v1, err := ParseConstantValue(between.Constant(0), column.CQLType, params, column)
	if err != nil {
		return types.Condition{}, err
	}
	v2, err := ParseConstantValue(between.Constant(1), column.CQLType, params, column)
	if err != nil {
		return types.Condition{}, err
	}
	return types.Condition{
		Column:   column,
		Operator: types.BETWEEN,
		Value:    v1,
		Value2:   v2,
	}, nil
}

func parseWhereIn(whereIn cql.IRelationInContext, tableConfig *schemaMapping.TableSchema, params *types.QueryParameterBuilder) (types.Condition, error) {
	column, err := ParseColumnContext(tableConfig, whereIn.Column())
	if err != nil {
		return types.Condition{}, err
	}
	value, err := ParseTupleValue(whereIn.TupleValue(), types.NewListType(column.CQLType), params, column)
	if err != nil {
		return types.Condition{}, err
	}
	return types.Condition{
		Column:   column,
		Operator: types.IN,
		Value:    value,
	}, nil
}

func ParseTupleValue(tuple cql.ITupleValueContext, lt *types.ListType, params *types.QueryParameterBuilder, column *types.Column) (types.DynamicValue, error) {
	if tuple.Marker() != nil {
		return ParseMarker(tuple.Marker(), lt, params, column)
	}

	valueFn := tuple.FunctionArgs()
	all := valueFn.AllConstant()
	if all == nil || len(all) == 0 {
		return nil, errors.New("failed to parse values for IN operator")
	}
	var inValues []any
	for _, v := range all {
		parsed, err := utilities.StringToGo(TrimQuotes(v.GetText()), lt.ElementType())
		if err != nil {
			return nil, err
		}
		inValues = append(inValues, parsed)
	}
	return types.NewLiteralValue(inValues), nil
}

func ParseValueAny(v cql.IValueAnyContext, dt types.CqlDataType, params *types.QueryParameterBuilder, column *types.Column) (types.DynamicValue, error) {
	if v.Marker() != nil {
		return ParseMarker(v.Marker(), dt, params, column)
	}
	// todo handle tuple
	if v.Constant() != nil {
		return ParseConstantValue(v.Constant(), dt, params, column)
	}
	if v.ValueList() != nil {
		value, err := ParseListValue(v.ValueList(), dt)
		if err != nil {
			return nil, err
		}
		return types.NewLiteralValue(value), nil
	}
	if v.ValueMap() != nil {
		value, err := ParseCqlMapAssignment(v.ValueMap(), dt)
		if err != nil {
			return nil, err
		}
		return types.NewLiteralValue(value), nil
	}
	if v.ValueSet() != nil {
		value, err := ParseCqlSetAssignment(v.ValueSet(), dt)
		if err != nil {
			return nil, err
		}
		return types.NewLiteralValue(value), nil
	}
	// todo more robust function handling
	if v.FunctionCall() != nil {
		functionName := strings.ToLower(v.FunctionCall().OBJECT_NAME().GetText())
		if functionName == "totimestamp" {
			fnArgs := v.FunctionCall().FunctionArgs().AllFunctionCall()
			if len(fnArgs) == 1 {
				innerFn := strings.ToLower(fnArgs[0].OBJECT_NAME().GetText())
				if innerFn == "now" {
					return types.NewTimestampNowValue(), nil
				}
			}
		}
		if functionName == "now" && dt.Code() == types.TIMEUUID {
			return types.NewTimeuuidNowValue(), nil
		}
		return nil, fmt.Errorf("unsupported function call: `%s`", v.GetText())
	}
	return nil, fmt.Errorf("unhandled value set `%s`", v.GetText())
}

func ParseConstantValue(v cql.IConstantContext, dt types.CqlDataType, params *types.QueryParameterBuilder, col *types.Column) (types.DynamicValue, error) {
	if v.Marker() != nil {
		return ParseMarker(v.Marker(), dt, params, col)
	}
	goValue, err := ParseCqlConstant(v, dt)
	if err != nil {
		return nil, err
	}

	return types.NewLiteralValue(goValue), nil
}

func ParseColumnContext(table *schemaMapping.TableSchema, r cql.IColumnContext) (*types.Column, error) {
	if r == nil {
		return nil, fmt.Errorf("nil column")
	}

	var col string
	if r.OBJECT_NAME() != nil {
		col = TrimDoubleQuotes(r.OBJECT_NAME().GetText())
	} else if r.K_KEY() != nil { // hack to handle unquoted `key` column reference that cqlsh does
		col = "key"
	} else if r.KwType() != nil { // hack to handle unquoted `type` column reference that java client does
		col = "type"
	} else {
		return nil, fmt.Errorf("unknown column form: `%s`", r.GetText())
	}

	return table.GetColumn(types.ColumnName(col))
}

func TrimQuotes(s string) string {
	if len(s) < 2 {
		return s
	}
	if s[0] == '\'' && s[len(s)-1] == '\'' {
		s = s[1 : len(s)-1]
	}
	return strings.ReplaceAll(s, `''`, `'`)
}

func TrimDoubleQuotes(s string) string {
	if len(s) < 2 {
		return s
	}
	if s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	return strings.ReplaceAll(s, `""`, `"`)
}

func GetTimestampInfo(timestampContext cql.ITimestampContext, params *types.QueryParameterBuilder) (types.DynamicValue, error) {
	if timestampContext == nil {
		return nil, nil
	}
	if timestampContext.Marker() != nil {
		return ParseMarker(timestampContext.Marker(), types.TypeBigInt, params, nil)
	}
	literal := timestampContext.DecimalLiteral()
	if literal == nil {
		return nil, nil
	}
	value, err := ExtractDecimalLiteral(literal, types.TypeBigInt)
	if err != nil {
		return nil, err
	}
	return value, nil
}

func ValidateRequiredPrimaryKeys(tableConfig *schemaMapping.TableSchema, assignments []types.Assignment) error {
	// primary key counts are very small for legitimate use cases so greedy iterations are fine
	for _, wantKey := range tableConfig.PrimaryKeys {
		found := false
		for _, assignment := range assignments {
			if assignment.Column().Name == wantKey.Name {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("missing primary key: '%s'", wantKey.Name)
		}
	}
	return nil
}

func ParseListValue(l cql.IValueListContext, dt types.CqlDataType) ([]types.GoValue, error) {
	lt, ok := dt.(*types.ListType)
	if !ok {
		return nil, fmt.Errorf("cannot parse list value for non-list type: %s", dt.String())
	}
	var result []types.GoValue
	for _, c := range l.AllConstant() {
		val, err := ParseCqlConstant(c, lt.ElementType())
		if err != nil {
			return nil, err
		}
		result = append(result, val)
	}
	return result, nil
}

func ParseCqlMapAssignment(m cql.IValueMapContext, dt types.CqlDataType) (map[types.GoValue]types.GoValue, error) {
	mt, ok := dt.(*types.MapType)
	if !ok {
		return nil, fmt.Errorf("cannot parse map assignment for column type `%s`", dt.String())
	}
	result := make(map[types.GoValue]types.GoValue)
	all := m.AllConstant()
	for i := 0; i+1 < len(all); i += 2 {
		key, err := ParseCqlConstant(all[i], mt.KeyType())
		if err != nil {
			return nil, err
		}
		val, err := ParseCqlConstant(all[i+1], mt.ValueType())
		if err != nil {
			return nil, err
		}
		result[key] = val
	}
	return result, nil
}

func ParseCqlSetAssignment(s cql.IValueSetContext, dt types.CqlDataType) ([]types.GoValue, error) {
	st, ok := dt.(*types.SetType)
	if !ok {
		return nil, fmt.Errorf("cannot perform set assignment on column of type %s", dt.String())
	}
	var result []types.GoValue
	all := s.AllConstant()
	if len(all) == 0 {
		return result, nil
	}
	for _, c := range all {
		if c == nil {
			continue
		}
		val, err := ParseCqlConstant(c, st.ElementType())
		if err != nil {
			return nil, err
		}
		result = append(result, val)
	}
	return result, nil
}

// ParseCqlConstant parses a CQL constant value.
// Converts CQL constant values to their corresponding Go types with validation.
// Returns error if constant is invalid or conversion fails.
func ParseCqlConstant(c cql.IConstantContext, dt types.CqlDataType) (types.GoValue, error) {
	if c.Marker() != nil {
		return nil, fmt.Errorf("cannot get constant from prepared query")
	}

	if c.KwNull() != nil {
		return nil, nil
	}

	switch dt.Code() {
	case types.TIMESTAMP:
		if c.StringLiteral() != nil {
			return parseStringLiteral(c.StringLiteral(), dt)
		}
		if c.DecimalLiteral() != nil {
			return utilities.StringToGo(c.DecimalLiteral().GetText(), dt)
		}
	case types.VARCHAR, types.TEXT, types.ASCII:
		if c.StringLiteral() != nil {
			return parseStringLiteral(c.StringLiteral(), dt)
		}
	case types.INT, types.BIGINT, types.DECIMAL, types.FLOAT, types.COUNTER:
		if c.DecimalLiteral() != nil {
			return utilities.StringToGo(c.DecimalLiteral().GetText(), dt)
		}
		if c.FloatLiteral() != nil {
			return utilities.StringToGo(c.FloatLiteral().GetText(), dt)
		}
	case types.BOOLEAN:
		if c.BooleanLiteral() != nil {
			return utilities.StringToGo(c.BooleanLiteral().GetText(), dt)
		}
	case types.BLOB:
		if c.HexadecimalLiteral() != nil {
			return utilities.StringToGo(c.HexadecimalLiteral().GetText(), dt)
		}
	case types.UUID, types.TIMEUUID:
		if c.UUID() != nil {
			return utilities.StringToGo(c.UUID().GetText(), dt)
		}
	}

	return nil, fmt.Errorf("invalid literal for type %s: '%s'", dt.String(), c.GetText())
}

func parseStringLiteral(s cql.IStringLiteralContext, dt types.CqlDataType) (types.GoValue, error) {
	return utilities.StringToGo(TrimQuotes(s.GetText()), dt)
}

const (
	bigtableEncodingVersion = primitive.ProtocolVersion4
	referenceTime           = int64(1262304000000)
	maxNanos                = int32(9999)
)

// EncodeScalarForBigtable converts a value to its byte representation based on CQL type.
// Handles type conversion and encoding according to the protocol version.
// Returns error if value type is invalid or encoding fails.
func EncodeScalarForBigtable(value types.GoValue, cqlType types.CqlDataType) (types.BigtableValue, error) {
	if value == nil {
		return nil, nil
	}

	switch cqlType.DataType() {
	case datatype.Int, datatype.Bigint:
		return encodeBigIntForBigtable(value)
	case datatype.Float:
		return encodeFloat32ForBigtable(value)
	case datatype.Double:
		return encodeFloat64ForBigtable(value)
	case datatype.Boolean:
		return encodeBoolForBigtable(value)
	case datatype.Timestamp:
		return encodeTimestampForBigtable(value)
	case datatype.Blob:
		return encodeBlobForBigtable(value)
	case datatype.Varchar, datatype.Ascii:
		return encodeStringForBigtable(value)
	case datatype.Uuid, datatype.Timeuuid:
		return encodeUuidForBigtable(value, cqlType.DataType())
	default:
		return nil, fmt.Errorf("unsupported CQL type: %s", cqlType.String())
	}
}

func encodeUuidForBigtable(value interface{}, dt datatype.DataType) (types.BigtableValue, error) {
	var valToEncode interface{} = value
	switch v := value.(type) {
	case uuid.UUID:
		valToEncode = primitive.UUID(v)
	case *uuid.UUID:
		if v != nil {
			valToEncode = primitive.UUID(*v)
		}
	case []byte:
		if len(v) == 16 {
			var u primitive.UUID
			copy(u[:], v)
			valToEncode = u
		}
	}
	result, err := proxycore.EncodeType(dt, bigtableEncodingVersion, valToEncode)
	if err != nil {
		return nil, fmt.Errorf("failed to encode uuid/timeuuid: %w", err)
	}
	return result, err
}

func encodeBlobForBigtable(value types.GoValue) (types.BigtableValue, error) {
	bd, err := proxycore.EncodeType(datatype.Blob, bigtableEncodingVersion, value)
	if err != nil {
		return nil, fmt.Errorf("error encoding blob: %w", err)
	}
	return bd, nil
}

func encodeStringForBigtable(value types.GoValue) (types.BigtableValue, error) {
	bd, err := proxycore.EncodeType(datatype.Varchar, bigtableEncodingVersion, value)
	if err != nil {
		return nil, fmt.Errorf("error encoding string: %w", err)
	}
	return bd, nil
}

func cassandraValueToGoValue(dt types.CqlDataType, value *primitive.Value, pv primitive.ProtocolVersion) (types.GoValue, error) {
	goValue, err := proxycore.DecodeType(dt.DataType(), pv, value.Contents)
	if err != nil {
		return nil, err
	}

	// the proxy codec returns []*string/whatever type which isn't quite what we're expecting downstream
	if dt.Code() == types.LIST || dt.Code() == types.SET {
		goValue = dereferenceSlice(goValue)
	}

	err = utilities.ValidateData(goValue, dt)
	if err != nil {
		return nil, err
	}

	return goValue, nil
}

func dereferenceSlice(goValue types.GoValue) types.GoValue {
	switch v := goValue.(type) {
	case []*string:
		var s = make([]string, len(v))
		for i, ps := range v {
			s[i] = *ps
		}
		return s
	case []*int32:
		var s = make([]int32, len(v))
		for i, ps := range v {
			s[i] = *ps
		}
		return s
	case []*int64:
		var s = make([]int64, len(v))
		for i, ps := range v {
			s[i] = *ps
		}
		return s
	case []*bool:
		var s = make([]bool, len(v))
		for i, ps := range v {
			s[i] = *ps
		}
		return s
	case []*float32:
		var s = make([]float32, len(v))
		for i, ps := range v {
			s[i] = *ps
		}
		return s
	case []*float64:
		var s = make([]float64, len(v))
		for i, ps := range v {
			s[i] = *ps
		}
		return s
	case []*primitive.UUID:
		var s = make([]primitive.UUID, len(v))
		for i, ps := range v {
			s[i] = *ps
		}
		return s
	}
	return goValue
}

// encodeBigIntForBigtable encodes bigint values to bytes.
// Converts bigint values to byte representation with validation.
// Returns error if value is invalid or encoding fails.
func encodeBigIntForBigtable(value interface{}) ([]byte, error) {
	var intVal int64
	switch v := value.(type) {
	case int32:
		intVal = int64(v)
	case *int32:
		intVal = int64(*v)
	case int64:
		intVal = v
	case *int64:
		intVal = *v
	default:
		return nil, fmt.Errorf("unsupported type for bigint: %T", value)
	}
	result, err := proxycore.EncodeType(datatype.Bigint, bigtableEncodingVersion, intVal)
	if err != nil {
		return nil, fmt.Errorf("failed to encode bigint: %w", err)
	}
	return result, err
}

func encodeFloat32ForBigtable(value interface{}) (types.BigtableValue, error) {
	var floatVal float32
	var err error
	switch v := value.(type) {
	case float32:
		floatVal = v
	case *float32:
		floatVal = *v
	default:
		return nil, fmt.Errorf("unsupported type for float: %T", value)
	}
	if err != nil {
		return nil, err
	}
	result, err := proxycore.EncodeType(datatype.Float, bigtableEncodingVersion, floatVal)
	if err != nil {
		return nil, fmt.Errorf("failed to encode float: %w", err)
	}
	return result, err
}

func encodeFloat64ForBigtable(value interface{}) (types.BigtableValue, error) {
	var floatVal float64
	var err error
	switch v := value.(type) {
	case float64:
		floatVal = v
	case *float64:
		floatVal = *v
	default:
		return nil, fmt.Errorf("unsupported type for double: %T", value)
	}
	if err != nil {
		return nil, err
	}
	result, err := proxycore.EncodeType(datatype.Double, bigtableEncodingVersion, floatVal)
	if err != nil {
		return nil, fmt.Errorf("failed to encode double: %w", err)
	}
	return result, err
}

// encodeBoolForBigtable encodes boolean values to bytes.
// Converts boolean values to byte representation with validation.
// Returns error if value is invalid or encoding fails.
func encodeBoolForBigtable(value interface{}) (types.BigtableValue, error) {
	var intVal int64
	switch v := value.(type) {
	case bool:
		if v {
			intVal = 1
		} else {
			intVal = 0
		}
	case *bool:
		if *v {
			intVal = 1
		} else {
			intVal = 0
		}
	default:
		return nil, fmt.Errorf("unsupported type: %T", value)
	}
	bd, err := proxycore.EncodeType(datatype.Bigint, bigtableEncodingVersion, intVal)
	if err != nil {
		return nil, err
	}
	return bd, nil
}

func encodeTimestampForBigtable(value interface{}) (types.BigtableValue, error) {
	var t time.Time
	switch v := value.(type) {
	case time.Time:
		t = v
	case *time.Time:
		t = *v
	default:
		return nil, fmt.Errorf("unsupported timestamp type: %T", value)
	}
	return proxycore.EncodeType(datatype.Timestamp, bigtableEncodingVersion, t)
}

func ParseTableSpec(tableSpec cql.ITableSpecContext, sessionKeyspace types.Keyspace) (types.Keyspace, types.TableName, error) {
	if tableSpec == nil {
		return "", "", errors.New("failed to parse table name")
	}

	keyspace, err := ParseKeyspace(tableSpec.Keyspace(), sessionKeyspace)
	if err != nil {
		return "", "", err
	}
	table, err := ParseTable(tableSpec.Table())
	if err != nil {
		return "", "", err
	}
	return keyspace, table, nil
}

func ParseTable(t cql.ITableContext) (types.TableName, error) {
	if t == nil {
		return "", errors.New("failed to parse table name")
	}

	// some system tables use keywords, so we need to handle them explicitly
	if t.K_TABLES() != nil {
		return "tables", nil
	} else if t.K_KEYSPACES() != nil {
		return "keyspaces", nil
	} else if t.K_FUNCTIONS() != nil {
		return "functions", nil
	}

	if t.OBJECT_NAME() == nil {
		return "", errors.New("failed to parse table name")
	}

	name := t.OBJECT_NAME().GetText()
	if name == "" {
		return "", errors.New("failed to parse table name")
	}
	if !IsValidIdentifier(name) {
		return "", fmt.Errorf("invalid table name: `%s`", name)
	}

	tableName := types.TableName(name)
	return tableName, nil
}

func ParseKeyspace(k cql.IKeyspaceContext, sessionKeyspace types.Keyspace) (types.Keyspace, error) {
	keyspaceString := string(sessionKeyspace)
	if k != nil && k.OBJECT_NAME() != nil {
		keyspaceString = TrimDoubleQuotes(k.OBJECT_NAME().GetText())
	}
	if keyspaceString == "" {
		return "", errors.New("no keyspace specified")
	}
	if !IsValidIdentifier(keyspaceString) {
		return "", fmt.Errorf("invalid keyspace name: `%s`", keyspaceString)
	}
	return types.Keyspace(keyspaceString), nil
}

func IsValidIdentifier(i string) bool {
	return validTableName.MatchString(i)
}

func ParseSelectIndex(si cql.ISelectIndexContext, alias string, table *schemaMapping.TableSchema) (types.SelectedColumn, error) {
	col, err := ParseColumnContext(table, si.Column())
	if err != nil {
		return types.SelectedColumn{}, err
	}
	if col.CQLType.Code() == types.MAP {
		mt := col.CQLType.(*types.MapType)
		mapKey, err := ParseCqlConstant(si.Constant(), mt.KeyType())
		if err != nil {
			return types.SelectedColumn{}, err
		}
		colQualifier, err := scalarToColumnQualifier(mapKey)
		if err != nil {
			return types.SelectedColumn{}, err
		}
		return *types.NewSelectedColumnMapElement(si.GetText(), col.Name, alias, mt.ValueType(), colQualifier), nil
	} else if col.CQLType.Code() == types.LIST {
		index, err := ParseBigInt(si.Constant().DecimalLiteral())
		if err != nil {
			return types.SelectedColumn{}, err
		}
		lt := col.CQLType.(*types.ListType)
		return *types.NewSelectedColumnListElement(si.GetText(), col.Name, alias, lt.ElementType(), index), nil
	} else {
		return types.SelectedColumn{}, fmt.Errorf("cannot access index/key of column type %s", col.CQLType.String())
	}
}

func ParseSelectColumn(si cql.ISelectColumnContext, alias string, table *schemaMapping.TableSchema) (types.SelectedColumn, error) {
	col, err := ParseColumnContext(table, si.Column())
	if err != nil {
		return types.SelectedColumn{}, err
	}
	return *types.NewSelectedColumn(string(col.Name), col.Name, alias, col.CQLType), nil
}

func ParseSelectFunction(sf cql.ISelectFunctionContext, alias string, table *schemaMapping.TableSchema) (types.SelectedColumn, error) {
	f, err := ParseCqlFunc(sf.FunctionCall())
	if err != nil {
		return types.SelectedColumn{}, err
	}
	switch f {
	case types.FuncCodeWriteTime:
		col, err := parseSingleColumnFunctionArg(sf.FunctionCall(), table)
		if err != nil {
			return types.SelectedColumn{}, err
		}
		return *types.NewSelectedColumnFunction(sf.FunctionCall().GetText(), col.Name, alias, types.TypeBigInt, f), nil
	case types.FuncCodeCount:
		value, err := parseStarOrColumnFunctionArg(sf.FunctionCall(), table)
		if err != nil {
			return types.SelectedColumn{}, err
		}
		sql := fmt.Sprintf("system.count(%s)", value)
		return *types.NewSelectedColumnFunction(sql, types.ColumnName(value), alias, types.TypeBigInt, f), nil
	case types.FuncCodeAvg, types.FuncCodeSum, types.FuncCodeMin, types.FuncCodeMax:
		col, err := parseSingleColumnFunctionArg(sf.FunctionCall(), table)
		if err != nil {
			return types.SelectedColumn{}, err
		}

		if !isNumericType(col.CQLType) {
			return types.SelectedColumn{}, fmt.Errorf("invalid aggregate type: %s", col.CQLType.String())
		}

		resultType := col.CQLType
		// integer type columns need to have result a result type that captures decimal positionalValues
		if f == types.FuncCodeAvg && (col.CQLType.Code() == types.BIGINT || col.CQLType.Code() == types.INT || col.CQLType.Code() == types.COUNTER) {
			resultType = types.TypeDouble
		}

		// example: system.avg(price)
		sql := fmt.Sprintf("system.%s(%s)", strings.ToLower(f.String()), col.Name)
		return *types.NewSelectedColumnFunction(sql, col.Name, alias, resultType, f), nil
	case types.FuncCodeNow:
		return *types.NewSelectedColumnFunction("now()", "", alias, types.TypeTimeuuid, f), nil
	case types.FuncCodeToTimestamp:
		col, err := parseSingleColumnFunctionArg(sf.FunctionCall(), table)
		if err != nil {
			return types.SelectedColumn{}, err
		}
		if col.CQLType.Code() != types.TIMEUUID {
			return types.SelectedColumn{}, fmt.Errorf("totimestamp() argument must be a timeuuid, not %s", col.CQLType.String())
		}
		sql := string(col.Name)
		return *types.NewSelectedColumnFunction(sql, col.Name, alias, types.TypeTimestamp, f), nil
	default:
		return types.SelectedColumn{}, fmt.Errorf("unhandled function type `%s`", sf.FunctionCall().GetText())
	}
}

func isNumericType(t types.CqlDataType) bool {
	return t.Code() == types.BIGINT || t.Code() == types.INT || t.Code() == types.FLOAT || t.Code() == types.DOUBLE || t.Code() == types.COUNTER
}

func parseSingleColumnFunctionArg(fn cql.IFunctionCallContext, table *schemaMapping.TableSchema) (*types.Column, error) {
	if fn.FunctionArgs() == nil || len(fn.FunctionArgs().AllOBJECT_NAME()) != 1 {
		return nil, fmt.Errorf("expected 1 column argument for `%s`", fn.GetText())
	}
	if len(fn.FunctionArgs().AllFunctionCall()) != 0 {
		return nil, fmt.Errorf("expected 1 column argument for `%s`", fn.GetText())
	}
	arg := fn.FunctionArgs().OBJECT_NAME(0).GetText()
	col, err := table.GetColumn(types.ColumnName(arg))
	if err != nil {
		return nil, fmt.Errorf("expected valid column in function arguments `%s`", fn.GetText())
	}
	return col, nil
}

func parseStarOrColumnFunctionArg(fn cql.IFunctionCallContext, table *schemaMapping.TableSchema) (string, error) {
	if fn.STAR() != nil {
		return "*", nil
	}
	col, err := parseSingleColumnFunctionArg(fn, table)
	if err != nil {
		return "", err
	}
	return string(col.Name), nil
}

func ParseAs(a cql.IAsSpecContext) (string, error) {
	if a == nil || a.OBJECT_NAME() == nil {
		return "", nil
	}

	alias := a.OBJECT_NAME().GetText()
	if utilities.IsReservedCqlKeyword(alias) {
		return "", fmt.Errorf("cannot use reserved word as alias: '%s'", alias)
	}

	return alias, nil
}

func ConvertStrictConditionsToRowKeyValues(table *schemaMapping.TableSchema, conditions []types.Condition) ([]types.DynamicValue, error) {
	if len(conditions) > len(table.PrimaryKeys) {
		return nil, fmt.Errorf("only primary keys supported in where clause")
	}
	var results []types.DynamicValue
	for _, key := range table.PrimaryKeys {
		var value types.DynamicValue
		for _, con := range conditions {
			if con.Column.Name == key.Name {
				value = con.Value
				break
			}
		}
		if value == nil {
			return nil, fmt.Errorf("all primary keys must be included in the where clause. missing `%s`", key.Name)
		}
		results = append(results, value)
	}
	return results, nil
}
