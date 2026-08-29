package select_translator

import (
	"errors"
	"fmt"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	sm "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/metadata"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/utilities"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"strings"
)

func createBtqlSelectClause(s *types.SelectClause, isGroupBy bool) (string, error) {
	if s.IsStar {
		return "*", nil
	}
	var columns []string
	for _, col := range s.Columns {
		c, err := toBtql(col.Value)
		if err != nil {
			return "", err
		}
		if col.Alias != "" {
			c = fmt.Sprintf("%s AS %s", c, col.Alias)
		}
		columns = append(columns, c)
	}

	return strings.Join(columns, ", "), nil
}

func toBtql(value types.DynamicValue) (string, error) {
	switch v := value.(type) {
	case *types.SelectStarValue:
		return "*", nil
	case *types.ColumnValue:
		return processRegularColumn(v)
	case *types.MapAccessValue:
		return fmt.Sprintf("`%s`['%s']", v.Column.Name, v.MapKey), nil
	case *types.FunctionValue:
		return createBtqlFunc(v)
	case *types.LiteralValue:
		return utilities.GoToQueryString(v.Value)
	case *types.ParameterizedValue:
		return "@" + string(v.GetParameter()), nil
	default:
		return "", fmt.Errorf("unhandled dynamic value type: %T", v)
	}
}

func createBtqlFunc(f *types.FunctionValue) (string, error) {
	if f.GetParameter() != "" {
		return "@" + string(f.GetParameter()), nil
	}

	switch f.Func.GetCode() {
	case types.FuncCodeCount:
		arg0, err := toBtql(f.Args[0])
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("COUNT(%s)", arg0), nil
	case types.FuncCodeWriteTime:
		col, err := getColumnArg(0, f.Args)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("UNIX_MICROS(WRITE_TIMESTAMP(%s, '%s'))", col.ColumnFamily, col.Name), nil
	case types.FuncCodeAvg:
		return createSimpleBtqlFunc(f)
	case types.FuncCodeSum:
		return createSimpleBtqlFunc(f)
	case types.FuncCodeMin:
		return createSimpleBtqlFunc(f)
	case types.FuncCodeMax:
		return createSimpleBtqlFunc(f)
	default:
		return "", fmt.Errorf("unhandled function translation: %s", f.Func.GetName())
	}
}

func createSimpleBtqlFunc(f *types.FunctionValue) (string, error) {
	col, err := getColumnArg(0, f.Args)
	if err != nil {
		return "", err
	}
	castValue, err := castScalarColumn(col)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s(%s)", f.Func.GetName(), castValue), nil
}

func getColumnArgOrStar(index int, args []types.DynamicValue) (types.DynamicValue, error) {
	value := args[index]
	star, ok := value.(types.SelectStarValue)
	if ok {
		return star, nil
	}
	col, ok := value.(types.ColumnValue)
	if ok {
		return col, nil
	}
	return nil, fmt.Errorf("invalid argument: %T expected column or *", value)
}

func getColumnArg(index int, args []types.DynamicValue) (*types.Column, error) {
	value := args[index]
	col, ok := value.(*types.ColumnValue)
	if !ok {
		return nil, fmt.Errorf("invalid argument: %T expected a column", value)
	}
	return col.Column, nil
}

func processRegularColumn(col *types.ColumnValue) (string, error) {
	if col.Column.CQLType.Code() == types.LIST {
		return fmt.Sprintf("MAP_VALUES(`%s`)", col.Column.Name), nil
	} else if col.Column.CQLType.IsCollection() {
		return fmt.Sprintf("`%s`", col.Column.Name), nil
	} else {
		return castScalarColumn(col.Column)
	}
}

// createBigtableSql() Returns Bigtable Select query using Parsed information.
//
// Parameters:
//   - data: PreparedSelectQuery struct with all select query info from CQL query
func createBigtableSql(t *SelectTranslator, st *types.PreparedSelectQuery) (string, error) {
	column := ""

	tableConfig, err := t.schemaMappingConfig.GetTableSchema(st.Keyspace(), st.Table())
	if err != nil {
		return "", err
	}

	isGroupBy := false
	if len(st.GroupByColumns) > 0 {
		isGroupBy = true
	}
	column, err = createBtqlSelectClause(st.SelectClause, isGroupBy)
	if err != nil {
		return "", err
	}

	btQuery := fmt.Sprintf("SELECT %s FROM %s", column, st.Table())
	whereCondition, err := createBtqlWhereClause(st.Conditions, tableConfig)
	if err != nil {
		return "nil", err
	}
	if whereCondition != "" {
		btQuery += whereCondition
	}

	// Build alias-to-column map
	aliasToColumn := make(map[string]bool)
	for _, col := range st.SelectClause.Columns {
		if col.Alias != "" {
			aliasToColumn[col.Alias] = true
		}
	}

	if len(st.GroupByColumns) > 0 {
		btQuery = btQuery + " GROUP BY "
		var groupByKeys []string
		for _, col := range st.GroupByColumns {
			lookupCol := col
			if _, ok := aliasToColumn[col]; ok {
				groupByKeys = append(groupByKeys, col)
			} else {
				if colMeta, ok := tableConfig.Columns[types.ColumnName(lookupCol)]; ok {
					if !colMeta.CQLType.IsCollection() {
						col, err := castScalarColumn(colMeta)
						if err != nil {
							return "", err
						}
						groupByKeys = append(groupByKeys, col)
					} else {
						return "", errors.New("group by on collection st type is not supported")
					}
				}
			}
		}
		btQuery = btQuery + strings.Join(groupByKeys, ",")
	}

	if st.OrderBy.IsOrderBy {
		orderByClauses := make([]string, 0, len(st.OrderBy.Columns))
		for _, orderByCol := range st.OrderBy.Columns {
			lookupCol := orderByCol.Column
			if _, ok := aliasToColumn[orderByCol.Column]; ok {
				orderByClauses = append(orderByClauses, orderByCol.Column+" "+string(orderByCol.Operation))
			} else {
				if colMeta, ok := tableConfig.Columns[types.ColumnName(lookupCol)]; ok {
					if colMeta.IsPrimaryKey {
						orderByClauses = append(orderByClauses, orderByCol.Column+" "+string(orderByCol.Operation))
					} else if !colMeta.CQLType.IsCollection() {
						orderByKey, err := castScalarColumn(colMeta)
						if err != nil {
							return "", err
						}
						orderByClauses = append(orderByClauses, orderByKey+" "+string(orderByCol.Operation))
					} else {
						return "", errors.New("order by on collection st type is not supported")
					}
				} else {
					return "", fmt.Errorf("unknown column name '%s' in table %s.%s", orderByCol.Column, st.Keyspace(), st.Table())
				}
			}
		}
		btQuery = btQuery + " ORDER BY " + strings.Join(orderByClauses, ", ")
	}

	if st.LimitValue != nil {
		limitString, err := toBtql(st.LimitValue)
		if err != nil {
			return "", err
		}
		btQuery = btQuery + " LIMIT " + limitString
	}
	btQuery += ";"
	return btQuery, nil
}

// createBtqlWhereClause(): takes a slice of Condition structs and returns a string representing the WHERE clause of a bigtable SQL query.
// It iterates over the clauses and constructs the WHERE clause by combining the column name, operator, and value of each clause.
// If the operator is "IN", the value is wrapped with the UNNEST function.
// The constructed WHERE clause is returned as a string.
func createBtqlWhereClause(conditions []types.Condition, tableConfig *sm.TableSchema) (string, error) {
	var btqlConditions []string
	for _, condition := range conditions {
		column := "`" + string(condition.Column.Name) + "`"
		if col, ok := tableConfig.Columns[condition.Column.Name]; ok {
			// Check if the column is a primitive type and prepend the column family
			if !col.CQLType.IsCollection() {
				var castErr error
				column, castErr = castScalarColumn(col)
				if castErr != nil {
					return "", castErr
				}
			}
		}

		var btql string
		if condition.Operator == types.BETWEEN {
			v1, err := toBtql(condition.Value)
			if err != nil {
				return "", err
			}
			v2, err := toBtql(condition.Value2)
			if err != nil {
				return "", err
			}

			btql = fmt.Sprintf("%s BETWEEN %s AND %s", column, v1, v2)
		} else if condition.Operator == types.IN {
			v, err := toBtql(condition.Value)
			if err != nil {
				return "", err
			}
			btql = fmt.Sprintf("%s IN UNNEST(%s)", column, v)
		} else if condition.Operator == types.CONTAINS || condition.Operator == types.CONTAINS_KEY {
			v, err := toBtql(condition.Value)
			if err != nil {
				return "", err
			}
			if condition.Column.CQLType.Code() == types.SET || condition.Column.CQLType.Code() == types.MAP {
				btql = fmt.Sprintf("MAP_CONTAINS_KEY(%s, %s)", column, v)
			} else {
				btql = fmt.Sprintf("ARRAY_INCLUDES(MAP_VALUES(%s), %s)", column, v)
			}
		} else {
			v, err := toBtql(condition.Value)
			if err != nil {
				return "", err
			}
			btql = fmt.Sprintf("%s %s %s", column, condition.Operator, v)
		}

		btqlConditions = append(btqlConditions, btql)
	}

	if len(btqlConditions) == 0 {
		return "", nil
	}

	return " WHERE " + strings.Join(btqlConditions, " AND "), nil
}

// castScalarColumn handles column type casting in queries.
// Manages type conversion for column values with validation.
// Returns error if column type is invalid or conversion fails.
func castScalarColumn(colMeta *types.Column) (string, error) {
	if colMeta.CQLType.IsCollection() {
		return "", fmt.Errorf("cannot cast collection type column '%s'", colMeta.Name)
	}
	if colMeta.IsPrimaryKey {
		// primary keys are stored in structured row keys, not column families, and have type information already, so no need to case
		return string(colMeta.Name), nil
	}

	switch colMeta.CQLType.DataType() {
	case datatype.Int:
		return fmt.Sprintf("TO_INT64(`%s`['%s'])", colMeta.ColumnFamily, colMeta.Name), nil
	case datatype.Bigint:
		return fmt.Sprintf("TO_INT64(`%s`['%s'])", colMeta.ColumnFamily, colMeta.Name), nil
	case datatype.Float:
		return fmt.Sprintf("TO_FLOAT32(`%s`['%s'])", colMeta.ColumnFamily, colMeta.Name), nil
	case datatype.Double:
		return fmt.Sprintf("TO_FLOAT64(`%s`['%s'])", colMeta.ColumnFamily, colMeta.Name), nil
	case datatype.Boolean:
		return fmt.Sprintf("TO_INT64(`%s`['%s'])", colMeta.ColumnFamily, colMeta.Name), nil
	case datatype.Timestamp:
		return fmt.Sprintf("TIMESTAMP_FROM_UNIX_MILLIS(TO_INT64(`%s`['%s']))", colMeta.ColumnFamily, colMeta.Name), nil
	case datatype.Counter:
		return fmt.Sprintf("`%s`['']", colMeta.Name), nil
	case datatype.Blob, datatype.Uuid, datatype.Timeuuid:
		return fmt.Sprintf("`%s`['%s']", colMeta.ColumnFamily, colMeta.Name), nil
	case datatype.Varchar, datatype.Ascii:
		return fmt.Sprintf("`%s`['%s']", colMeta.ColumnFamily, colMeta.Name), nil
	default:
		return "", fmt.Errorf("unsupported CQL type: %s", colMeta.CQLType.DataType().String())
	}
}
