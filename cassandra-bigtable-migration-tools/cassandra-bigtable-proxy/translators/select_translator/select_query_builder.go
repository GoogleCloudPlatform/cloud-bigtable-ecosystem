package select_translator

import (
	"errors"
	"fmt"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	sm "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/metadata"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/utilities"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"strings"
)

func createBtqlSelectClause(tableConfig *sm.TableSchema, s *types.SelectClause, isGroupBy bool) (string, error) {
	if s.IsStar {
		return "*", nil
	}
	var columns []string
	for _, col := range s.Columns {
		if col.Func != types.FuncCodeUnknown {
			c, err := createBtqlFunc(col, tableConfig)
			if err != nil {
				return "", err
			}
			columns = append(columns, c)
		} else {
			c, err := createBtqlSelectCol(tableConfig, col, isGroupBy)
			if err != nil {
				return "", err
			}
			columns = append(columns, c)
		}
	}

	return strings.Join(columns, ", "), nil
}

func createBtqlFunc(col types.SelectedColumn, tableConfig *sm.TableSchema) (string, error) {
	if col.Func == types.FuncCodeCount && col.ColumnName == "*" {
		if col.Alias != "" {
			return "count(*) as " + col.Alias, nil
		}
		return "count(*)", nil
	}

	colMeta, found := tableConfig.Columns[col.ColumnName]
	if !found {
		// Check if the column is an alias
		if aliasMeta, aliasFound := tableConfig.Columns[types.ColumnName(col.Alias)]; aliasFound {
			colMeta = aliasMeta
		} else {
			return "", fmt.Errorf("column metadata not found for column '%s' in table '%s' and keyspace '%s'", col.ColumnName, tableConfig.Name, tableConfig.Keyspace)
		}
	}

	if col.Func == types.FuncCodeWriteTime {
		// todo what about collections?
		if col.Alias != "" {
			return fmt.Sprintf("UNIX_MICROS(WRITE_TIMESTAMP(%s, '%s')) AS %s", colMeta.ColumnFamily, colMeta.Name, col.Alias), nil
		}
		return fmt.Sprintf("UNIX_MICROS(WRITE_TIMESTAMP(%s, '%s'))", colMeta.ColumnFamily, colMeta.Name), nil
	}

	if col.Func == types.FuncCodeToTimestamp {
		castValue, err := castScalarColumn(colMeta)
		if err != nil {
			return "", err
		}
		if col.Alias != "" {
			return fmt.Sprintf("%s AS %s", castValue, col.Alias), nil
		}
		return castValue, nil
	}

	if col.Func == types.FuncCodeNow {
		if col.Alias != "" {
			return "CURRENT_TIMESTAMP() AS " + col.Alias, nil // CURRENT_TIMESTAMP returns a timestamp, we'll convert it to UUID in result processor if needed, but it's better to return raw bits if we can.
		}
		return "CURRENT_TIMESTAMP()", nil
	}

	castValue, castErr := castScalarColumn(colMeta)
	if castErr != nil {
		return "", castErr
	}
	column := fmt.Sprintf("%s(%s)", col.Func.String(), castValue)

	if col.Alias != "" {
		column = column + " as " + col.Alias
	}

	return column, nil
}

func createBtqlSelectCol(tableConfig *sm.TableSchema, selectedColumn types.SelectedColumn, isGroupBy bool) (string, error) {
	colName := selectedColumn.Sql
	if selectedColumn.MapKey != "" {
		colName = string(selectedColumn.ColumnName)
	}
	col, err := tableConfig.GetColumn(types.ColumnName(colName))
	if err != nil {
		return "", err
	}
	var sql string
	if isGroupBy {
		sql, err = castScalarColumn(col)
		if err != nil {
			return "", err
		}
	} else {
		sql, err = processRegularColumn(selectedColumn, col)
		if err != nil {
			return "", err
		}
	}

	if selectedColumn.Alias != "" {
		sql = fmt.Sprintf("%s as %s", sql, selectedColumn.Alias)
	}
	return sql, nil
}

/*
processRegularColumn processes the given column based on its metadata and table context.

It formats the column name differently depending on whether the current table name matches the column metadata's name.
If there is a match, the function qualifies the column name using the table name; otherwise, it formats the column name as standalone.
Additionally, if the column is not a collection, it builds a formatted reference that includes accessing the column within the specified column family;
if the column is a collection, it uses the preformatted column name from the metadata.

Parameters:
  - columnMetadata: Specifies metadata for the selected column, including its name and a field for storing the formatted version.
  - tableName: The name of the current table to determine if table-qualified formatting is needed.
  - columnFamily: The column family identifier used when constructing the column reference for non-collection types.
  - colMeta: A pointer to column configuration containing additional details such as whether the column is a collection.
  - columns: A slice of strings that accumulates the formatted column references.

Returns:

	An updated slice of strings with the new formatted column reference appended.
*/
func processRegularColumn(selectedColumn types.SelectedColumn, col *types.Column) (string, error) {
	if col.CQLType.DataType().GetDataTypeCode() == primitive.DataTypeCodeList {
		return fmt.Sprintf("MAP_VALUES(`%s`)", selectedColumn.Sql), nil
	} else if selectedColumn.MapKey != "" {
		return fmt.Sprintf("`%s`['%s']", selectedColumn.ColumnName, selectedColumn.MapKey), nil
	} else if col.CQLType.IsCollection() {
		return fmt.Sprintf("`%s`", selectedColumn.Sql), nil
	} else {
		return castScalarColumn(col)
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
	column, err = createBtqlSelectClause(tableConfig, st.SelectClause, isGroupBy)
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
	aliasToColumn := make(map[string]string)
	for _, col := range st.SelectClause.Columns {
		if col.Alias != "" {
			aliasToColumn[col.Alias] = string(col.ColumnName)
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
		limitString, err := ToBtql(st.LimitValue)
		if err != nil {
			return "", err
		}
		btQuery = btQuery + " LIMIT " + limitString
	}
	btQuery += ";"
	return btQuery, nil
}

func ToBtql(dynamicValue types.DynamicValue) (string, error) {
	switch v := dynamicValue.(type) {
	case *types.LiteralValue:
		return utilities.GoToString(v.Value)
	case *types.ParameterizedValue:
		return "@" + string(v.Parameter), nil
	case *types.TimestampNowValue:
		return "CURRENT_TIMESTAMP()", nil
	default:
		return "", fmt.Errorf("unhandled value type %T", v)
	}
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
			v1, err := ToBtql(condition.Value)
			if err != nil {
				return "", err
			}
			v2, err := ToBtql(condition.Value2)
			if err != nil {
				return "", err
			}

			btql = fmt.Sprintf("%s BETWEEN %s AND %s", column, v1, v2)
		} else if condition.Operator == types.IN {
			v, err := ToBtql(condition.Value)
			if err != nil {
				return "", err
			}
			btql = fmt.Sprintf("%s IN UNNEST(%s)", column, v)
		} else if condition.Operator == types.CONTAINS || condition.Operator == types.CONTAINS_KEY {
			v, err := ToBtql(condition.Value)
			if err != nil {
				return "", err
			}
			if condition.Column.CQLType.Code() == types.SET || condition.Column.CQLType.Code() == types.MAP {
				btql = fmt.Sprintf("MAP_CONTAINS_KEY(%s, %s)", column, v)
			} else {
				btql = fmt.Sprintf("ARRAY_INCLUDES(MAP_VALUES(%s), %s)", column, v)
			}
		} else {
			v, err := ToBtql(condition.Value)
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
