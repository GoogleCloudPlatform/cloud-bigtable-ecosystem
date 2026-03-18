package mem_table

import (
	"fmt"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	"strings"
)

func (e *InMemEngine) Execute(query *types.ExecutableSelectQuery) ([]types.GoRow, error) {
	data, err := e.GetTable(query.Keyspace(), query.Table())
	if err != nil {
		return nil, err
	}

	// filter with where clause
	var filtered []types.GoRow
	for _, row := range data {
		matches, err := matchesConditions(row, query.Conditions, query.Values)
		if err != nil {
			return nil, err
		}
		if matches {
			filtered = append(filtered, row)
		}
	}

	// apply limit
	if query.Limit != nil {
		limit, err := query.Values.GetValueInt32(query.Limit)
		if err != nil {
			return nil, err
		}
		if limit < int32(len(filtered)) {
			filtered = filtered[:limit]
		}
	}

	// todo move transformation logic to CqlFunc type
	// handle aggregates if present
	for _, col := range query.SelectClause.Columns {
		if f, ok := col.Value.(*types.FunctionValue); ok {
			if f.Func.GetCode() == types.FuncCodeCount {
				key := col.Cql
				if col.Alias != "" {
					key = col.Alias
				}
				return []types.GoRow{
					{key: int64(len(filtered))},
				}, nil
			}
		}
	}

	// projection (SELECT specific columns)
	if query.SelectClause.IsStar {
		return filtered, nil
	}

	projectedRows := make([]types.GoRow, len(filtered))
	for i, row := range filtered {
		selectedRow, err := projectRow(row, query.SelectClause.Columns)
		if err != nil {
			return nil, err
		}
		projectedRows[i] = selectedRow
	}

	return projectedRows, nil
}

func matchesConditions(row types.GoRow, conditions []types.Condition, values *types.QueryParameterValues) (bool, error) {
	for _, cond := range conditions {
		col := string(cond.Column.Name)
		rowValue, ok := row[col]
		if !ok {
			return false, fmt.Errorf("unknown column '%s' on table", col)
		}

		queryValue, err := cond.Value.GetValue(values)
		if err != nil {
			return false, err
		}

		comparison, err := compareAny(rowValue, queryValue)
		if err != nil {
			return false, err
		}

		switch cond.Operator {
		case types.EQ:
			return comparison == 0, nil
		case types.GT:
			return comparison > 0, nil
		case types.LT:
			return comparison < 0, nil
		case types.GTE:
			return comparison >= 0, nil
		case types.LTE:
			return comparison <= 0, nil
		default:
			return false, fmt.Errorf("unhandled where clause operator: '%s'", cond.Operator)
		}
	}
	return true, nil
}

func compareAny(v1, v2 any) (int, error) {
	s1, ok1 := v1.(string)
	s2, ok2 := v2.(string)
	if ok1 && ok2 {
		return strings.Compare(s1, s2), nil
	}

	i1, ok1 := v1.(int32)
	i2, ok2 := v2.(int32)
	if ok1 && ok2 {
		if i1 == i2 {
			return 0, nil
		} else if i1 < i2 {
			return -1, nil
		} else {
			return 1, nil
		}
	}
	bigInt1, ok1 := v1.(int64)
	bigInt2, ok2 := v2.(int64)
	if ok1 && ok2 {
		if bigInt1 == bigInt2 {
			return 0, nil
		} else if bigInt1 < bigInt2 {
			return -1, nil
		} else {
			return 1, nil
		}
	}

	return 0, fmt.Errorf("unhandled comparison types: %T and %T", v1, v2)
}

func projectRow(source types.GoRow, columns []types.SelectedColumn) (types.GoRow, error) {
	newRow := make(types.GoRow)
	for _, col := range columns {
		selectedCol, ok := col.Value.(*types.ColumnValue)
		if !ok {
			return nil, fmt.Errorf("unhandled select column operation: %T", col.Value)
		}
		val, exists := source[string(selectedCol.Column.Name)]
		if exists {
			// Use Alias if present, otherwise use original Family Name
			targetKey := col.Alias
			if targetKey == "" {
				targetKey = string(selectedCol.Column.Name)
			}
			newRow[targetKey] = val
		}
	}
	return newRow, nil
}
