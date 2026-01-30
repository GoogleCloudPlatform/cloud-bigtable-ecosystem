package update_translator

import (
	"errors"
	"fmt"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	schemaMapping "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/metadata"
	cql "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/third_party/cqlparser"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/translators/common"
)

func parseUpdateValues(assignments []cql.IAssignmentElementContext, tableConfig *schemaMapping.TableSchema, params *types.QueryParameterBuilder) ([]types.Assignment, error) {
	if len(assignments) == 0 {
		return nil, errors.New("invalid input")
	}
	var parsed []types.Assignment

	for _, a := range assignments {
		if a.AssignmentEquals() != nil {
			assignment := a.AssignmentEquals()
			col, err := common.ParseColumnContext(tableConfig, assignment.Column())
			if err != nil {
				return nil, err
			}
			value, err := common.ParseValueAny(assignment.ValueAny(), col.CQLType, params, col)
			if err != nil {
				return nil, err
			}
			parsed = append(parsed, types.NewComplexAssignmentSet(col, value))
		} else if a.AssignmentAppend() != nil {
			assignment := a.AssignmentAppend()
			append_, err := ParseAppend(assignment.Column(), assignment.ArithmeticOperator(), assignment.ValueAny(), false, tableConfig, params)
			if err != nil {
				return nil, err
			}
			parsed = append(parsed, append_)
		} else if a.AssignmentPrepend() != nil {
			assignment := a.AssignmentPrepend()
			prepend, err := ParseAppend(assignment.Column(), assignment.ArithmeticOperator(), assignment.ValueAny(), true, tableConfig, params)
			if err != nil {
				return nil, err
			}
			parsed = append(parsed, prepend)
		} else if a.AssignmentIndex() != nil {
			assignment := a.AssignmentIndex()
			col, err := common.ParseColumnContext(tableConfig, assignment.Column())
			if err != nil {
				return nil, err
			}
			if col.CQLType.Code() == types.MAP {
				mt := col.CQLType.(*types.MapType)
				keyValue, err := common.ParseCqlConstant(assignment.IndexOrKeyAccess().Constant(), mt.KeyType())
				if err != nil {
					return nil, fmt.Errorf("failed to parse map key: %w", err)
				}
				value, err := common.ParseConstantValue(assignment.Constant(), mt.ValueType(), params, col)
				if err != nil {
					return nil, err
				}
				parsed = append(parsed, types.NewComplexAssignmentUpdateMapValue(col, keyValue, value))
			} else if col.CQLType.Code() == types.LIST {
				lt := col.CQLType.(*types.ListType)
				// cql doesn't allow parameterized index access so no need to handle that here
				index, err := common.ParseBigInt(assignment.IndexOrKeyAccess().Constant().DecimalLiteral())
				if err != nil {
					return nil, fmt.Errorf("failed to parse list collection index: %w", err)
				}
				value, err := common.ParseConstantValue(assignment.Constant(), lt.ElementType(), params, col)
				if err != nil {
					return nil, err
				}
				parsed = append(parsed, types.NewComplexAssignmentUpdateListIndex(col, index, value))
			} else {
				return nil, fmt.Errorf("cannot do key or index based access on type %s", col.CQLType.String())
			}
		}
	}
	return parsed, nil
}

func ParseAppend(columnContext cql.IColumnContext, op cql.IArithmeticOperatorContext, valueAny cql.IValueAnyContext, isPrepend bool, tableConfig *schemaMapping.TableSchema, params *types.QueryParameterBuilder) (types.Assignment, error) {
	col, err := common.ParseColumnContext(tableConfig, columnContext)
	if err != nil {
		return nil, err
	}

	operator, err := common.ParseArithmeticOperator(op)

	// append value type might be different from the column type, depending on the operator and column type.
	var valueType types.CqlDataType
	if col.CQLType.Code() == types.MAP {
		if operator == types.PLUS {
			valueType = col.CQLType
		} else if operator == types.MINUS {
			// removing elements can only be keys for maps
			mt := col.CQLType.(*types.MapType)
			valueType = types.NewListType(mt.KeyType())
		} else {
			return nil, fmt.Errorf("unsupported map append operator: %s", operator)
		}
	} else if col.CQLType.Code() == types.LIST {
		valueType = col.CQLType
	} else if col.CQLType.Code() == types.SET {
		valueType = col.CQLType
	} else if col.CQLType.Code() == types.COUNTER {
		valueType = col.CQLType
	} else {
		return nil, fmt.Errorf("cannot append on column type: %s", col.CQLType.String())
	}

	value, err := common.ParseValueAny(valueAny, valueType, params, col)
	if err != nil {
		return nil, err
	}
	if col.CQLType.Code() == types.COUNTER {
		return types.NewAssignmentCounterIncrement(col, operator, value), nil
	} else {
		return types.NewComplexAssignmentAppend(col, operator, value, isPrepend), nil
	}
}
