package insert_translator

import (
	"errors"
	"fmt"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	schemaMapping "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/metadata"
	cql "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/third_party/cqlparser"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/translators/common"
)

// parseInsertColumns() parses columns and values from the Insert query
//
// Parameters:
//   - input: Insert Columns Spec Context from antlr parser.
//   - tableName: Table Columns
//   - schemaMapping: JSON Config which maintains column and its datatypes info.
//
// Returns: ColumnsResponse struct and error if any
func parseInsertColumns(input cql.IInsertColumnSpecContext, tableConfig *schemaMapping.TableSchema, params *types.QueryParameterBuilder) ([]*types.Column, error) {

	if input == nil {
		return nil, errors.New("parseInsertColumns: No Input paramaters found for columns")
	}

	columnListObj := input.ColumnList()
	if columnListObj == nil {
		return nil, errors.New("parseInsertColumns: error while parsing columns")
	}
	columns := columnListObj.AllColumn()
	if columns == nil {
		return nil, errors.New("parseInsertColumns: error while parsing columns")
	}

	if len(columns) <= 0 {
		return nil, errors.New("parseInsertColumns: No Columns found in the Insert CqlQuery")
	}

	var result []*types.Column
	for _, val := range columns {
		columnName := types.ColumnName(val.GetText())
		col, err := tableConfig.GetColumn(columnName)
		if err != nil {
			return nil, err
		}
		result = append(result, col)
	}

	return result, nil
}

func parseInsertValues(input cql.IInsertValuesSpecContext, columns []*types.Column, params *types.QueryParameterBuilder) ([]types.Assignment, error) {
	if input == nil {
		return nil, errors.New("insert values clause missing or malformed")
	}

	valuesExpressionList := input.ValueListSpec()
	if valuesExpressionList == nil {
		return nil, errors.New("setParamsFromValues: error while parsing values")
	}

	allValues := valuesExpressionList.AllValueAny()
	if allValues == nil {
		return nil, errors.New("setParamsFromValues: error while parsing values")
	}

	if len(allValues) != len(columns) {
		return nil, fmt.Errorf("found mismatch between column count (%d) value count (%d)", len(columns), len(allValues))
	}

	var assignments []types.Assignment
	for i, v := range allValues {
		column := columns[i]
		value, err := common.ParseValueAny(v, column.CQLType, params, column)
		if err != nil {
			return nil, err
		}
		assignments = append(assignments, types.NewComplexAssignmentSet(column, value))
	}
	return assignments, nil
}
