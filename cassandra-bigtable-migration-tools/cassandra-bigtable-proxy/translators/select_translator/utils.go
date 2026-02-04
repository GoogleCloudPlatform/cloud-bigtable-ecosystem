package select_translator

import (
	"errors"
	"fmt"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	sm "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/metadata"
	cql "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/third_party/cqlparser"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/translators/common"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"strings"
)

// parseSelectClause() parse Columns from the Select CqlQuery
//
// Parameters:
//   - input: The Select Element context from the antlr Parser.
//
// Returns: Columns Meta and an error if any.
func parseSelectClause(input cql.ISelectElementsContext, table *sm.TableSchema) (*types.SelectClause, error) {
	if input == nil {
		return nil, errors.New("select clause empty")
	}

	if input.STAR() != nil {
		return &types.SelectClause{
			IsStar: true,
		}, nil
	}

	if len(input.AllSelectElement()) == 0 {
		return nil, errors.New("select clause empty")
	}

	var selectedColumns []types.SelectedColumn
	for _, val := range input.AllSelectElement() {
		alias, err := common.ParseAs(val.AsSpec())
		if err != nil {
			return nil, err
		}
		var selected types.SelectedColumn
		if val.SelectFunction() != nil {
			selected, err = common.ParseSelectFunction(val.SelectFunction(), alias, table)
		} else if val.SelectColumn() != nil {
			selected, err = common.ParseSelectColumn(val.SelectColumn(), alias, table)
		} else if val.SelectIndex() != nil {
			selected, err = common.ParseSelectIndex(val.SelectIndex(), alias, table)
		} else {
			return nil, fmt.Errorf("unhandled select column `%s`", val.GetText())
		}
		if err != nil {
			return nil, err
		}
		if selected.ResultType == nil {
			return nil, fmt.Errorf("unhandled result type")
		}
		selectedColumns = append(selectedColumns, selected)
	}

	return &types.SelectClause{Columns: selectedColumns}, nil
}

// parseOrderByFromSelect() parse Order By from the Select CqlQuery
//
// Parameters:
//   - input: The Order Spec context from the antlr Parser.
//
// Returns: OrderBy struct
func parseOrderByFromSelect(input cql.IOrderSpecContext) (types.OrderBy, error) {
	var response types.OrderBy

	if input == nil {
		response.IsOrderBy = false
		return response, nil
	}

	orderSpecElements := input.AllOrderSpecElement()
	if len(orderSpecElements) == 0 {
		return types.OrderBy{}, fmt.Errorf("order_by section not have proper values")
	}

	response.IsOrderBy = true
	response.Columns = make([]types.OrderByColumn, 0, len(orderSpecElements))

	for _, element := range orderSpecElements {
		object := element.OBJECT_NAME()
		if object == nil {
			return types.OrderBy{}, fmt.Errorf("order_by section not have proper values")
		}

		colName := strings.TrimSpace(object.GetText())
		if colName == "" {
			return types.OrderBy{}, fmt.Errorf("order_by section has empty column name")
		}

		orderByCol := types.OrderByColumn{
			Column:    colName,
			Operation: types.Asc,
		}

		if element.KwDesc() != nil {
			orderByCol.Operation = types.Desc
		}

		response.Columns = append(response.Columns, orderByCol)
	}

	return response, nil
}

func parseLimitClause(input cql.ILimitSpecContext, params *types.QueryParameterBuilder) (types.DynamicValue, error) {
	if input == nil {
		return nil, nil
	}
	if input.Marker() != nil {
		return common.ParseMarker(input.Marker(), types.TypeInt, params, nil)
	}
	return common.ExtractDecimalLiteral(input.DecimalLiteral(), types.TypeInt)
}

func parseGroupByColumn(input cql.IGroupSpecContext) []string {
	if input == nil {
		return nil
	}

	groupSpecElements := input.AllGroupSpecElement()
	if len(groupSpecElements) == 0 {
		return nil
	}

	var columns []string
	for _, element := range groupSpecElements {
		object := element.OBJECT_NAME()
		if object == nil {
			// If any group by element is missing, treat as malformed and return nil
			return nil
		}

		colName := object.GetText()
		columns = append(columns, colName)
	}

	return columns
}

func selectedColumnsToMetadata(table *sm.TableSchema, selectClause *types.SelectClause) []*message.ColumnMetadata {
	if selectClause.IsStar {
		return table.GetMetadata()
	}
	var resultColumns []*message.ColumnMetadata
	for i, c := range selectClause.Columns {
		name := c.Sql
		if c.Alias != "" {
			name = c.Alias
		}
		var col = message.ColumnMetadata{
			Keyspace: string(table.Keyspace),
			Table:    string(table.Name),
			Name:     name,
			Index:    int32(i),
			Type:     c.ResultType.DataType(),
		}
		resultColumns = append(resultColumns, &col)
	}
	return resultColumns
}
