/*
 * Copyright (C) 2025 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not
 * use this file except in compliance with the License. You may obtain a copy of
 * the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
 * WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
 * License for the specific language governing permissions and limitations under
 * the License.
 */

package translator

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	methods "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/methods"
	types "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	schemaMapping "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/schema-mapping"
	cql "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/third_party/cqlparser"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/utilities"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
)

const (
	customWriteTime = "writetime_"
)

// parseColumnsFromSelect() parse Columns from the Select Query
//
// Parameters:
//   - input: The Select Element context from the antlr Parser.
//
// Returns: Column Meta and an error if any.
func parseColumnsFromSelect(input cql.ISelectElementsContext) (ColumnMeta, error) {
	var response ColumnMeta
	var funcName, argument string
	if input == nil {
		// todo - parser support for writetime - parser fails to get the selectItemList if first Item is writetime function
		return response, errors.New("no Input parameters found for columns")
	}

	if input.STAR() != nil {
		response.Star = true
	} else {
		columns := input.AllSelectElement()
		if len(columns) == 0 {
			// Todo - Code is not reachable
			return response, errors.New("no column parameters found in the query")
		}
		for _, val := range columns {
			var selectedColumns types.SelectedColumns
			if val == nil {
				return response, errors.New("error while parsing the values")
			}
			funcCall := val.FunctionCall() // TODO: improve this to parse writetime function
			if funcCall == nil {
				selectedColumns.Name = val.GetText()
				selectedColumns.ColumnName = val.GetText()
				// Handle map access
				mapAccess := val.MapAccess()

				if mapAccess != nil {
					objectName := mapAccess.OBJECT_NAME(0).GetText()
					if mapAccess.KwAs() != nil && len(mapAccess.AllOBJECT_NAME()) > 1 {
						selectedColumns.IsAs = true
						selectedColumns.Alias = mapAccess.OBJECT_NAME(1).GetText()
					}
					mapKey := mapAccess.Constant().GetText()

					// Remove surrounding quotes from mapKey
					mapKey = strings.Trim(mapKey, "'")

					// Populate the selected column
					selectedColumns.Name = fmt.Sprintf("%s['%s']", objectName, mapKey)

					selectedColumns.ColumnName = objectName
					selectedColumns.MapKey = mapKey

					// Append the parsed column to the response
					response.Column = append(response.Column, selectedColumns)
					continue
				}

			} else {
				selectedColumns.IsFunc = true
				if funcCall.OBJECT_NAME() == nil {
					return response, errors.New("function call object is nil")
				}
				funcName = strings.ToLower(funcCall.OBJECT_NAME().GetText())

				if funcCall.STAR() != nil {
					argument = funcCall.STAR().GetText()
				} else {
					if funcCall.FunctionArgs() == nil {
						return response, errors.New("function call argument object is nil")
					}
					argument = funcCall.FunctionArgs().GetText()
				}
				selectedColumns.Name = fmt.Sprintf("system.%s(%s)", funcName, argument)
				selectedColumns.FuncName = funcName
				selectedColumns.ColumnName = argument
				asOperator := val.KwAs()
				if asOperator != nil {
					selectedColumns.IsAs = true
					selectedColumns.Alias = val.AllOBJECT_NAME()[0].GetText()
				}
				response.Column = append(response.Column, selectedColumns)
				continue
			}
			//todo: fix this flow below
			allObject := val.AllOBJECT_NAME()
			asOperator := val.KwAs()
			if asOperator != nil && len(allObject) != 2 {
				return response, errors.New("unknown flow with as operator")
			}
			if asOperator != nil {
				selectedColumns.IsAs = true
				selectedColumns.Name = allObject[0].GetText()
				selectedColumns.Alias = allObject[1].GetText()
				selectedColumns.ColumnName = allObject[0].GetText()
			} else if len(allObject) == 2 {
				columnName := allObject[0].GetText()
				s := strings.ToLower(selectedColumns.Name)
				if strings.Contains(s, "writetime("+columnName+")") {
					selectedColumns.Name = "writetime(" + columnName + ")"
					selectedColumns.Alias = allObject[1].GetText()
					selectedColumns.IsAs = true
					selectedColumns.ColumnName = columnName
					selectedColumns.IsWriteTimeColumn = true
				}
			}
			selectedColumns.Name = strings.ReplaceAll(selectedColumns.Name, literalPlaceholder, "")
			writetimeValue, isExist := ExtractWritetimeValue(selectedColumns.Name)
			if isExist {
				selectedColumns.IsWriteTimeColumn = true
				selectedColumns.ColumnName = writetimeValue
			}
			response.Column = append(response.Column, selectedColumns)
		}
	}
	return response, nil
}

// parseTableFromSelect() parse Table Name from the Select Query
//
// Parameters:
//   - input: The From Spec context from the antlr Parser.
//
// Returns: Table Name and an error if any.
func parseTableFromSelect(input cql.IFromSpecContext) (*TableObj, error) {
	if input == nil {
		return nil, errors.New("no input parameters found for table and keyspace")
	}

	var response TableObj
	fromSpec, err := getFromSpecElement(input)
	if err != nil {
		return nil, err
	}

	allObj, err := getAllObjectNames(fromSpec)
	if err != nil {
		return nil, err
	}

	keyspaceName, tableName, err := getTableAndKeyspaceObjects(allObj)
	if err != nil {
		return nil, err
	}

	response = TableObj{
		TableName:    tableName,
		KeyspaceName: keyspaceName,
	}

	return &response, nil
}

// parseOrderByFromSelect() parse Order By from the Select Query
//
// Parameters:
//   - input: The Order Spec context from the antlr Parser.
//
// Returns: OrderBy struct
func parseOrderByFromSelect(input cql.IOrderSpecContext) (OrderBy, error) {
	var response OrderBy

	if input == nil {
		response.IsOrderBy = false
		return response, nil
	}

	orderSpecElements := input.AllOrderSpecElement()
	if len(orderSpecElements) == 0 {
		return OrderBy{}, fmt.Errorf("Order_by section not have proper values")
	}

	response.IsOrderBy = true
	response.Columns = make([]OrderByColumn, 0, len(orderSpecElements))

	for _, element := range orderSpecElements {
		object := element.OBJECT_NAME()
		if object == nil {
			return OrderBy{}, fmt.Errorf("Order_by section not have proper values")
		}

		colName := strings.TrimSpace(object.GetText())
		if colName == "" {
			return OrderBy{}, fmt.Errorf("Order_by section has empty column name")
		}
		if strings.Contains(colName, missingUndefined) {
			return OrderBy{}, fmt.Errorf("In order by, column name not provided correctly")
		}

		colName = strings.ReplaceAll(colName, literalPlaceholder, "")
		orderByCol := OrderByColumn{
			Column:    colName,
			Operation: Asc,
		}

		if element.KwDesc() != nil {
			orderByCol.Operation = Desc
		}

		response.Columns = append(response.Columns, orderByCol)
	}

	return response, nil
}

// parseLimitFromSelect() parse Limit from the Select Query
//
// Parameters:
//   - input: The Limit Spec context from the antlr Parser.
//
// Returns: Limit struct
func parseLimitFromSelect(input cql.ILimitSpecContext) (Limit, error) {
	var response Limit

	if input == nil {
		return response, nil
	}

	limitVal := input.DecimalLiteral().GetText()
	if limitVal == "" {
		return response, fmt.Errorf("LIMIT must be strictly positive")
	} else if strings.Contains(limitVal, missingUndefined) {
		return response, nil
	}
	// Check if the limit value is numeric and non-zero.
	if !strings.Contains(limitVal, "?") {
		// Attempt to convert count to an integer.
		numericCount, err := strconv.Atoi(limitVal)

		// Check if there was an error in conversion or if the numeric count is zero or negative.
		if err != nil || numericCount <= 0 {
			return response, fmt.Errorf("no viable alternative at input '%s'", limitVal)
		}
	}

	response.IsLimit = true
	response.Count = limitVal
	return response, nil
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
		if strings.Contains(colName, missingUndefined) || strings.Contains(colName, missing) {
			// If any group by element is malformed, treat as malformed and return nil
			return nil
		}

		colName = strings.ReplaceAll(colName, literalPlaceholder, "")
		columns = append(columns, colName)
	}

	return columns
}

// processSetStrings() processes the selected columns, formats them, and returns a map of aliases to metadata and a slice of formatted columns.
// Parameters:
//   - t : Translator instance
//   - selectedColumns: []types
//   - tableName : table name on which query is being executes
//   - keySpace : keyspace name on which query is being executed
//
// Returns:
//   - []string column containing formatted selected columns for bigtable query
//   - error if any
func processSetStrings(t *Translator, tableConfig *schemaMapping.TableConfig, selectedColumns []types.SelectedColumns, isGroupBy bool) ([]string, error) {
	var columns = make([]string, 0)
	columnFamily := t.SchemaMappingConfig.SystemColumnFamily
	var err error
	for _, columnMetadata := range selectedColumns {
		if columnMetadata.IsFunc {
			//todo: implement genereralized handling of writetime with rest of the aggregate functions
			columns, err = processFunctionColumn(t, columnMetadata, tableConfig, columns)
			if err != nil {
				return nil, err
			}
			continue
		} else {
			columns, err = processNonFunctionColumn(t, tableConfig, columnMetadata, columnFamily, columns, isGroupBy)
			if err != nil {
				return nil, err
			}
		}
	}

	return columns, nil
}

// processFunctionColumn processes columns that have aggregate functions applied to them in a SELECT query.
// It handles special cases like COUNT(*) and validates if the column and function types are allowed in aggregates.
//
// Parameters:
//   - t: Translator instance containing schema mapping configuration
//   - columnMetadata: Contains information about the selected column including function name and aliases
//   - tableName: Name of the table being queried
//   - keySpace: Keyspace name where the table exists
//   - columns: Slice of strings containing the processed column expressions
//
// Returns:
//   - []string: Updated slice of columns with the processed function column
//   - string: The data type of the column after function application
//   - error: Error if column metadata is not found, or if column/function type is not supported for aggregation
//
// The function performs the following operations:
//  1. Handles COUNT(*) as a special case
//  2. Validates column existence in schema mapping
//  3. Checks if column data type is allowed in aggregates
//  4. Validates if the aggregate function is supported
//  5. Applies any necessary type casting (converting cql select columns to respective bigtable columns)
//  6. Formats the column expression with function and alias if specified
func processFunctionColumn(t *Translator, columnMetadata types.SelectedColumns, tableConfig *schemaMapping.TableConfig, columns []string) ([]string, error) {

	if columnMetadata.ColumnName == STAR && strings.ToLower(columnMetadata.FuncName) == "count" {
		if columnMetadata.Alias != "" {
			return append(columns, "count(*) as "+columnMetadata.Alias), nil
		}
		columns = append(columns, "count(*)")
		return columns, nil

	}
	colMeta, found := tableConfig.Columns[columnMetadata.ColumnName]
	if !found {
		// Check if the column is an alias
		if aliasMeta, aliasFound := tableConfig.Columns[columnMetadata.Alias]; aliasFound {
			colMeta = aliasMeta
		} else {
			return nil, fmt.Errorf("column metadata not found for column '%s' in table '%s' and keyspace '%s'", columnMetadata.ColumnName, tableConfig.Name, tableConfig.Keyspace)
		}
	}
	colFamily := t.SchemaMappingConfig.SystemColumnFamily
	column := ""

	if !funcAllowedInAggregate(columnMetadata.FuncName) {
		return nil, fmt.Errorf("unknown function '%s'", columnMetadata.FuncName)
	}
	if columnMetadata.FuncName != "count" {
		colType, _ := methods.ConvertCQLDataTypeToString(colMeta.CQLType)
		if !dtAllowedInAggregate(colType) {
			return nil, fmt.Errorf("column not supported for aggregate")
		}
	}
	castValue, castErr := castColumns(colMeta, colFamily, t.SchemaMappingConfig.CounterColumnFamily)
	if castErr != nil {
		return nil, castErr
	}
	column = fmt.Sprintf("%s(%s)", columnMetadata.FuncName, castValue)

	if columnMetadata.IsAs {
		column = column + " as " + columnMetadata.Alias
	}
	columns = append(columns, column)
	return columns, nil
}

// dtAllowedInAggregate checks whether the provided data type is allowed in aggregate functions.
// It returns true if dataType is one of the supported numeric types (i.e., "int", "bigint", "float", or "double"),
// ensuring that only appropriate types are used for aggregate operations.
func dtAllowedInAggregate(dataType string) bool {
	allowedDataTypes := map[string]bool{
		"int":     true,
		"bigint":  true,
		"float":   true,
		"double":  true,
		"counter": true,
	}
	return allowedDataTypes[dataType]
}

// funcAllowedInAggregate checks if a given function name is allowed within an aggregate function.
// It converts the input string to lowercase and checks if it exists as a key in the allowedFunctions map.
// The allowed functions are "avg", "sum", "min", "max", and "count".
// It returns true if the function is allowed, and false otherwise.
func funcAllowedInAggregate(s string) bool {
	s = strings.ToLower(s)
	allowedFunctions := map[string]bool{
		"avg":   true,
		"sum":   true,
		"min":   true,
		"max":   true,
		"count": true,
	}

	return allowedFunctions[s]
}

func processNonFunctionColumn(t *Translator, tableConfig *schemaMapping.TableConfig, columnMetadata types.SelectedColumns, columnFamily string, columns []string, isGroupBy bool) ([]string, error) {
	colName := columnMetadata.Name
	if columnMetadata.IsWriteTimeColumn || columnMetadata.MapKey != "" {
		colName = columnMetadata.ColumnName
	}
	if columnMetadata.MapKey != "" {
		colName = columnMetadata.ColumnName
	}
	colMeta, err := tableConfig.GetColumn(colName)
	if err != nil {
		return nil, err
	}
	if columnMetadata.IsWriteTimeColumn {
		columns = processWriteTimeColumn(tableConfig, columnMetadata, columnFamily, columns)
	} else if columnMetadata.IsAs {
		columns = processAsColumn(columnMetadata, columnFamily,t.SchemaMappingConfig.CounterColumnFamily, colMeta, columns, isGroupBy)
	} else {
		columns = processRegularColumn(columnMetadata, tableConfig.Name, t.SchemaMappingConfig.CounterColumnFamily, columnFamily, colMeta, columns, isGroupBy)
	}
	return columns, nil
}

func processWriteTimeColumn(tableConfig *schemaMapping.TableConfig, columnMetadata types.SelectedColumns, columnFamily string, columns []string) []string {
	colFamily := tableConfig.GetColumnFamily(columnMetadata.ColumnName)
	aliasColumnName := customWriteTime + columnMetadata.ColumnName + ""
	wtColumn := ""
	if columnMetadata.Alias == customWriteTime+columnMetadata.ColumnName {
		wtColumn = fmt.Sprintf("WRITE_TIMESTAMP(%s, '%s') as %s", colFamily, columnMetadata.ColumnName, aliasColumnName)
	} else if columnMetadata.Alias != "" {
		wtColumn = fmt.Sprintf("WRITE_TIMESTAMP(%s, '%s') as %s", columnFamily, columnMetadata.ColumnName, columnMetadata.Alias)
	} else {
		wtColumn = fmt.Sprintf("WRITE_TIMESTAMP(%s, '%s')", columnFamily, columnMetadata.ColumnName)
	}
	columns = append(columns, wtColumn)
	return columns
}

func processAsColumn(columnMetadata types.SelectedColumns, counterColumnFamily string, columnFamily string, colMeta *types.Column, columns []string, isGroupBy bool) []string {
	var columnSelected string
	if !utilities.IsCollection(colMeta.CQLType) {
		var columnName = columnMetadata.Name
		if colMeta.CQLType == datatype.Counter {
			// override column family and name because counters are stored in the same column family
			columnFamily = counterColumnFamily
		}
		if isGroupBy {
			castedCol, _ := castColumns(colMeta, columnFamily, counterColumnFamily)
			columnSelected = castedCol + " as " + columnMetadata.Alias
		} else if colMeta.IsPrimaryKey {
			columnSelected = fmt.Sprintf("%s as %s", columnName, columnMetadata.Alias)
		} else {
			columnSelected = fmt.Sprintf("%s['%s'] as %s", columnFamily, columnName, columnMetadata.Alias)
		}
	} else {
		if colMeta.CQLType.GetDataTypeCode() == primitive.DataTypeCodeList {
			columnSelected = fmt.Sprintf("MAP_VALUES(%s) as %s", columnMetadata.Name, columnMetadata.Alias)
		} else {
			columnSelected = fmt.Sprintf("`%s` as %s", columnMetadata.Name, columnMetadata.Alias)
			if columnMetadata.MapKey != "" {
				columnSelected = fmt.Sprintf("%s['%s'] as %s", columnMetadata.ColumnName, columnMetadata.MapKey, columnMetadata.Alias)
			}
		}

	}
	columns = append(columns, columnSelected)
	return columns
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
func processRegularColumn(columnMetadata types.SelectedColumns, tableName string, counterColumnFamily string, columnFamily string, colMeta *types.Column, columns []string, isGroupBy bool) []string {
	if !utilities.IsCollection(colMeta.CQLType) {
		var columnName = columnMetadata.Name
		if colMeta.CQLType == datatype.Counter {
			// override column family and name because counters are stored in the same column family
			columnFamily = counterColumnFamily
		}
		if isGroupBy {
			castedCol, _ := castColumns(colMeta, columnFamily, counterColumnFamily)
			columns = append(columns, castedCol)
		} else if colMeta.IsPrimaryKey {
			columns = append(columns, columnName)
		} else {
			columns = append(columns, fmt.Sprintf("%s['%s']", columnFamily, columnName))
		}
	} else {
		var collectionColumn string
		colType, _ := methods.ConvertCQLDataTypeToString(colMeta.CQLType)
		if strings.Contains(colType, "list") {
			collectionColumn = fmt.Sprintf("MAP_VALUES(%s)", columnMetadata.Name)
		} else {
			collectionColumn = fmt.Sprintf("`%s`", columnMetadata.Name)
			if columnMetadata.MapKey != "" {
				collectionColumn = fmt.Sprintf("%s['%s']", columnMetadata.ColumnName, columnMetadata.MapKey)
			}
		}
		columns = append(columns, collectionColumn)
	}
	return columns
}

// inferDataType() returns the data type based on the name of a function.
//
// Parameters:
//   - methodName: Name of aggregate function
//
// Returns: Returns datatype of aggregate function.
func inferDataType(methodName string) (string, error) {
	switch methodName {
	case "count":
		return "bigint", nil
	case "round":
		return "float", nil
	default:
		return "", fmt.Errorf("unknown function '%s'", methodName)
	}
}

// getBigtableSelectQuery() Returns Bigtable Select query using Parsed information.
//
// Parameters:
//   - data: SelectQueryMap struct with all select query info from CQL query
func getBigtableSelectQuery(t *Translator, data *SelectQueryMap) (string, error) {
	column := ""
	var columns []string
	var err error

	tableConfig, err := t.SchemaMappingConfig.GetTableConfig(data.Keyspace, data.Table)
	if err != nil {
		return "", err
	}

	if data.ColumnMeta.Star {
		column = STAR
	} else {
		isGroupBy := false
		if len(data.GroupByColumns) > 0 {
			isGroupBy = true
		}
		columns, err = processSetStrings(t, tableConfig, data.ColumnMeta.Column, isGroupBy)
		if err != nil {
			return "nil", err
		}
		column = strings.Join(columns, ",")
	}

	if column == "" && data.Table == "" {
		return "", errors.New("could not prepare the select query due to incomplete information")
	}
	btQuery := fmt.Sprintf("SELECT %s FROM %s", column, data.Table)
	whereCondition, err := buildWhereClause(data.Clauses, tableConfig, data.Table, data.Keyspace)
	if err != nil {
		return "nil", err
	}
	if whereCondition != "" {
		btQuery += whereCondition
	}

	// Build alias-to-column map
	aliasToColumn := make(map[string]string)
	for _, col := range data.ColumnMeta.Column {
		if col.IsAs && col.Alias != "" {
			aliasToColumn[col.Alias] = col.ColumnName
		}
	}

	if len(data.GroupByColumns) > 0 {
		btQuery = btQuery + " GROUP BY "
		groupBykeys := []string{}
		for _, col := range data.GroupByColumns {
			lookupCol := col
			if _, ok := aliasToColumn[col]; ok {
				//lookupCol = realCol
				groupBykeys = append(groupBykeys, col)
			} else {
				if colMeta, ok := tableConfig.Columns[lookupCol]; ok {
					if !utilities.IsCollection(colMeta.CQLType) {
						col, err := castColumns(colMeta, t.SchemaMappingConfig.SystemColumnFamily, t.SchemaMappingConfig.CounterColumnFamily)
						if err != nil {
							return "", err
						}
						groupBykeys = append(groupBykeys, col)
					} else {
						return "", errors.New("group by on collection data type is not supported")
					}
				}
			}
		}
		btQuery = btQuery + strings.Join(groupBykeys, ",")
	}

	if data.OrderBy.IsOrderBy {
		orderByClauses := make([]string, 0, len(data.OrderBy.Columns))
		for _, orderByCol := range data.OrderBy.Columns {
			lookupCol := orderByCol.Column
			if _, ok := aliasToColumn[orderByCol.Column]; ok {
				orderByClauses = append(orderByClauses, orderByCol.Column+" "+string(orderByCol.Operation))
			} else {
				if colMeta, ok := tableConfig.Columns[lookupCol]; ok {
					if colMeta.IsPrimaryKey {
						orderByClauses = append(orderByClauses, orderByCol.Column+" "+string(orderByCol.Operation))
					} else if !utilities.IsCollection(colMeta.CQLType) {
						orderByKey, err := castColumns(colMeta, t.SchemaMappingConfig.SystemColumnFamily, t.SchemaMappingConfig.CounterColumnFamily)
						if err != nil {
							return "", err
						}
						orderByClauses = append(orderByClauses, orderByKey+" "+string(orderByCol.Operation))
					} else {
						return "", errors.New("order by on collection data type is not supported")
					}
				} else {
					return "", errors.New("Undefined column name " + orderByCol.Column + " in table " + data.Keyspace + "." + data.Table)
				}
			}
		}
		btQuery = btQuery + " ORDER BY " + strings.Join(orderByClauses, ", ")
	}

	if data.Limit.IsLimit {
		val := data.Limit.Count
		if val == questionMark || strings.Contains(val, questionMark) {
			val = "@" + limitPlaceholder
		}
		btQuery = btQuery + " LIMIT " + val
	}
	btQuery += ";"
	return btQuery, nil
}

// TranslateSelectQuerytoBigtable() Translates Cassandra select statement into a compatible Cloud Bigtable select query.
//
// Parameters:
//   - query: CQL Select statement
//
// Returns: SelectQueryMap struct and error if any
func (t *Translator) TranslateSelectQuerytoBigtable(originalQuery, sessionKeyspace string) (*SelectQueryMap, error) {
	lowerQuery := strings.ToLower(originalQuery)

	//Create copy of cassandra query where literals are substituted with a suffix
	query := renameLiterals(originalQuery)

	p, err := NewCqlParser(query, false)
	if err != nil {
		return nil, err
	}
	selectObj := p.Select_()
	if selectObj == nil || selectObj.KwSelect() == nil {
		// Todo - Code is not reachable
		return nil, errors.New("ToBigtableSelect: Could not parse select object")
	}

	kwSelectObj := selectObj.KwSelect()

	queryType := kwSelectObj.GetText()
	columns, err := parseColumnsFromSelect(selectObj.SelectElements())
	if err != nil {
		return nil, err
	}

	tableSpec, err := parseTableFromSelect(selectObj.FromSpec())
	if err != nil {
		return nil, err
	}

	keyspaceName := tableSpec.KeyspaceName
	tableName := tableSpec.TableName

	if keyspaceName == "" {
		if sessionKeyspace != "" {
			keyspaceName = sessionKeyspace
		} else {
			return nil, fmt.Errorf("invalid input paramaters found for keyspace")
		}
	}

	tableConfig, err := t.SchemaMappingConfig.GetTableConfig(keyspaceName, tableName)
	if err != nil {
		return nil, err
	}

	var QueryClauses QueryClauses

	if hasWhere(lowerQuery) {
		resp, err := parseWhereByClause(selectObj.WhereSpec(), tableConfig)
		if err != nil {
			return nil, err
		}
		QueryClauses = *resp
	}

	var groupBy []string
	if hasGroupBy(lowerQuery) {
		groupBy = parseGroupByColumn(selectObj.GroupSpec())
	}
	var orderBy OrderBy
	if hasOrderBy(lowerQuery) {
		orderBy, err = parseOrderByFromSelect(selectObj.OrderSpec())
		if err != nil {
			// pass the original error to provide proper root cause of error.
			return nil, err
		}
	} else {
		orderBy.IsOrderBy = false
	}

	var limit Limit
	if hasLimit(lowerQuery) {
		limit, err = parseLimitFromSelect(selectObj.LimitSpec())
		if err != nil {
			return nil, err
		}
		if limit.Count == questionMark || strings.Contains(limit.Count, questionMark) {
			QueryClauses.Params[limitPlaceholder] = int64(0) //placeholder type setting
			QueryClauses.ParamKeys = append(QueryClauses.ParamKeys, limitPlaceholder)
		}
	} else {
		limit.IsLimit = false
	}

	if limit.IsLimit && !orderBy.IsOrderBy {
		// Todo:- Add this logic in the parser
		limitPosition := strings.Index(strings.ToUpper(query), "LIMIT")
		orderByPosition := strings.Index(strings.ToUpper(query), "ORDER BY")
		if orderByPosition > limitPosition {
			return nil, errors.New("mismatched input 'Order' expecting EOF (...age = ? LIMIT ? [Order]...)")
		}
	}

	pmkNames := tableConfig.GetPrimaryKeys()

	selectQueryData := &SelectQueryMap{
		Query:           query,
		TranslatedQuery: "",
		QueryType:       queryType,
		Table:           tableName,
		Keyspace:        keyspaceName,
		ColumnMeta:      columns,
		Clauses:         QueryClauses.Clauses,
		PrimaryKeys:     pmkNames,
		Limit:           limit,
		OrderBy:         orderBy,
		GroupByColumns:  groupBy,
		Params:          QueryClauses.Params,
		ParamKeys:       QueryClauses.ParamKeys,
	}

	translatedResult, err := getBigtableSelectQuery(t, selectQueryData)
	if err != nil {
		return nil, err
	}

	selectQueryData.TranslatedQuery = translatedResult
	return selectQueryData, nil
}
