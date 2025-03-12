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

	"github.com/antlr4-go/antlr/v4"
	schemaMapping "github.com/ollionorg/cassandra-to-bigtable-proxy/schema-mapping"
	cql "github.com/ollionorg/cassandra-to-bigtable-proxy/translator/cqlparser"
	util "github.com/ollionorg/cassandra-to-bigtable-proxy/utilities"
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
			var selectedColumns schemaMapping.SelectedColumns
			if val == nil {
				return response, errors.New("error while parsing the values")
			}
			funcCall := val.FunctionCall()
			if funcCall == nil {
				selectedColumns.Name = val.GetText()
				// Handle map access
				mapAccess := val.MapAccess()
				if mapAccess != nil {
					objectName := mapAccess.OBJECT_NAME().GetText()
					mapKey := mapAccess.Constant().GetText()

					// Remove surrounding quotes from mapKey
					mapKey = strings.Trim(mapKey, "'")

					// Populate the selected column
					selectedColumns.Name = objectName
					selectedColumns.MapKey = mapKey
					selectedColumns.Alias = objectName + "_" + mapKey

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
					selectedColumns.Alias = funcName
				} else {
					if funcCall.FunctionArgs() == nil {
						return response, errors.New("function call argument object is nil")
					}
					argument = funcCall.FunctionArgs().GetText()
					selectedColumns.Alias = funcName + "_" + argument
				}
				selectedColumns.Name = argument
				selectedColumns.FuncName = funcName
			}
			if strings.Contains(selectedColumns.Name, missingUndefined) {
				return response, errors.New("one or more undefined fields for column name")
			}
			allObject := val.AllOBJECT_NAME()
			asOperator := val.KwAs()
			if asOperator != nil && len(allObject) != 2 {
				return response, errors.New("unknown flow with as operator")
			}
			if asOperator != nil {
				selectedColumns.IsAs = true
				selectedColumns.Name = allObject[0].GetText()
				selectedColumns.Alias = allObject[1].GetText()
			} else if len(allObject) == 2 {
				columnName := allObject[0].GetText()
				s := strings.ToLower(selectedColumns.Name)
				if strings.Contains(s, "writetime("+columnName+")") {
					selectedColumns.Name = "writetime(" + columnName + ")"
					selectedColumns.Alias = allObject[1].GetText()
					selectedColumns.IsAs = true
					selectedColumns.WriteTime_Column = columnName
					selectedColumns.Is_WriteTime_Column = true
				}
			}
			selectedColumns.Name = strings.ReplaceAll(selectedColumns.Name, literalPlaceholder, "")
			writetime_value, isExist := ExtractWritetimeValue(selectedColumns.Name)
			if isExist {
				selectedColumns.Is_WriteTime_Column = true
				selectedColumns.WriteTime_Column = writetime_value
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
	fromSpec := input.FromSpecElement()
	if fromSpec == nil {
		return nil, errors.New("error while parsing fromSpec")
	}
	allObj := fromSpec.AllOBJECT_NAME()
	if allObj == nil {
		return nil, errors.New("error while parsing all objects from the fromSpec")
	}

	if len(allObj) == 0 {
		return nil, errors.New("could not find table and keyspace name")
	}

	var tableObj antlr.TerminalNode
	var keyspaceObj antlr.TerminalNode
	if len(allObj) == 2 {
		keyspaceObj = fromSpec.OBJECT_NAME(0)
		tableObj = fromSpec.OBJECT_NAME(1)
		response.TableName = tableObj.GetText()
		response.KeyspaceName = keyspaceObj.GetText()
	} else {
		return nil, errors.New("could not find table or keyspace name or some extra parameter provided")
	}
	return &response, nil
}

// parseOrderByFromSelect() parse Order By from the Select Query
//
// Parameters:
//   - input: The Order Spec context from the antlr Parser.
//
// Returns: OrderBy struct
func parseOrderByFromSelect(input cql.IOrderSpecContext) OrderBy {
	var response OrderBy

	if input == nil {
		response.IsOrderBy = false
		return response
	}
	response.IsOrderBy = true
	element := input.OrderSpecElement()
	object := element.OBJECT_NAME()
	colName := object.GetText()
	if strings.Contains(colName, missingUndefined) {
		response.IsOrderBy = false
		return response
	}
	colName = strings.ReplaceAll(colName, literalPlaceholder, "")
	response.Column = colName

	response.Operation = "asc"
	if input.OrderSpecElement().KwDesc() != nil {
		response.Operation = "desc"
	}

	return response
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

	count := input.DecimalLiteral().GetText()
	if count == "" {
		return response, fmt.Errorf("LIMIT must be strictly positive")
	} else if strings.Contains(count, missingUndefined) {
		return response, nil
	}
	// Check if the count is numeric and non-zero.
	if !strings.Contains(count, "?") {
		// Attempt to convert count to an integer.
		numericCount, err := strconv.Atoi(count)

		// Check if there was an error in conversion or if the numeric count is zero or negative.
		if err != nil || numericCount <= 0 {
			return response, fmt.Errorf("no viable alternative at input '%s'", count)
		}
	}

	response.IsLimit = true
	response.Count = count
	return response, nil
}

// processStrings() processes the selected columns, formats them, and returns a map of aliases to metadata and a slice of formatted columns.
// Parameters:
//   - t : Translator instance
//   - selectedColumns: []schemaMapping.SelectedColumns
//   - tableName : table name on which query is being executes
//
// Returns: []string column containing formatted selected columns for bigtable query
//
//	and a map of aliases to AsKeywordMeta structs
func processStrings(t *Translator, selectedColumns []schemaMapping.SelectedColumns, tableName string, keySpace string) (map[string]AsKeywordMeta, []string, error) {
	lenInputString := len(selectedColumns)
	var columns = make([]string, 0)
	structs := make([]AsKeywordMeta, 0, lenInputString)
	columnFamily := t.SchemaMappingConfig.SystemColumnFamily

	for _, columnMetadata := range selectedColumns {
		dataType := ""
		isStructUpdated := false
		if columnMetadata.IsFunc {
			dd, err := inferDataType(columnMetadata.FuncName)
			if err != nil {
				return nil, nil, err
			}
			dataType = dd
			if columnMetadata.Name == STAR {
				columns = append(columns, fmt.Sprintf("%s(%s) as %s", columnMetadata.FuncName, columnMetadata.Name, columnMetadata.Alias))
			} else {
				if tableName == columnMetadata.Name {
					columns = append(columns, fmt.Sprintf("%s(`%s`.`%s`) as %s", columnMetadata.FuncName, columnMetadata.Name, columnMetadata.Name, columnMetadata.Alias))
				} else {
					columns = append(columns, fmt.Sprintf("%s(`%s`) as %s", columnMetadata.FuncName, columnMetadata.Name, columnMetadata.Alias))
				}
			}
		} else {
			var colMeta *schemaMapping.Column
			var ok bool
			colName := columnMetadata.Name
			if columnMetadata.Is_WriteTime_Column {
				colName = columnMetadata.WriteTime_Column
				dataType = "timestamp"
			}
			colMeta, ok = t.SchemaMappingConfig.TablesMetaData[keySpace][tableName][colName]
			if !ok {
				return nil, nil, fmt.Errorf("Column metadata not found")
			}
			if !columnMetadata.Is_WriteTime_Column {
				dataType = colMeta.CQLType
			}
			if columnMetadata.Is_WriteTime_Column {
				colFamily := t.GetColumnFamily(tableName, columnMetadata.WriteTime_Column, keySpace)
				aliasColumnName := util.CustomWritetime + columnMetadata.WriteTime_Column + ""
				if columnMetadata.Alias == util.CustomWritetime+columnMetadata.WriteTime_Column {
					columnMetadata.FormattedColumn = fmt.Sprintf("WRITE_TIMESTAMP(%s, '%s') as %s", colFamily, columnMetadata.WriteTime_Column, aliasColumnName)
				} else if columnMetadata.Alias != "" {
					columnMetadata.FormattedColumn = fmt.Sprintf("WRITE_TIMESTAMP(%s, '%s') as %s", columnFamily, columnMetadata.WriteTime_Column, columnMetadata.Alias)
					aliasColumnName = columnMetadata.Alias
				} else {
					columnMetadata.FormattedColumn = fmt.Sprintf("WRITE_TIMESTAMP(%s, '%s')", columnFamily, columnMetadata.WriteTime_Column)
					aliasColumnName = ""
				}
				structs = append(structs, AsKeywordMeta{
					IsFunc:   columnMetadata.IsFunc,
					Name:     columnMetadata.WriteTime_Column,
					Alias:    aliasColumnName,
					DataType: "timestamp",
				})
				isStructUpdated = true
				columns = append(columns, columnMetadata.FormattedColumn)
			} else if columnMetadata.IsAs {
				if tableName == columnMetadata.Name && !columnMetadata.Is_WriteTime_Column {
					columnMetadata.FormattedColumn = fmt.Sprintf("`%s`.`%s` as %s", columnMetadata.Name, columnMetadata.Name, columnMetadata.Alias)
				} else {
					if !colMeta.IsCollection {
						columnMetadata.FormattedColumn = fmt.Sprintf("%s['%s'] as %s", columnFamily, columnMetadata.Name, columnMetadata.Alias)
					} else {
						columnMetadata.FormattedColumn = fmt.Sprintf("`%s` as %s", columnMetadata.Name, columnMetadata.Alias)
					}
				}
				columns = append(columns, columnMetadata.FormattedColumn)
			} else {
				if tableName == columnMetadata.Name {
					columnMetadata.FormattedColumn = "`" + columnMetadata.Name + "`.`" + columnMetadata.Name + "`"
				} else {
					columnMetadata.FormattedColumn = "`" + columnMetadata.Name + "`"
				}
				if !colMeta.IsCollection {
					columns = append(columns, fmt.Sprintf("%s['%s']", columnFamily, columnMetadata.Name))
				} else {
					columns = append(columns, columnMetadata.FormattedColumn)
				}
			}
		}
		if !isStructUpdated {
			structs = append(structs, AsKeywordMeta{
				IsFunc:   columnMetadata.IsFunc,
				Name:     columnMetadata.Name,
				Alias:    columnMetadata.Alias,
				DataType: dataType,
			})
		}
	}
	var aliasMap = make(map[string]AsKeywordMeta)
	for _, meta := range structs {
		if meta.Alias != "" {
			aliasMap[meta.Alias] = meta
		}
	}
	return aliasMap, columns, nil
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
	var aliasMap map[string]AsKeywordMeta

	if data.ColumnMeta.Star {
		column = STAR
	} else {
		aliasMap, columns, err = processStrings(t, data.ColumnMeta.Column, data.Table, data.Keyspace)
		if err != nil {
			return "nil", err
		}
		data.AliasMap = aliasMap
		column = strings.Join(columns, ",")
	}

	if column != "" && data.Table != "" {
		btQuery := fmt.Sprintf("SELECT %s FROM %s", column, data.Table)
		whereCondition, err := buildWhereClause(data.Clauses, t, data.Table, data.Keyspace)
		if err != nil {
			return "nil", err
		}
		if whereCondition != "" {
			btQuery += whereCondition
		}

		if data.OrderBy.IsOrderBy {
			if colMeta, ok := t.SchemaMappingConfig.TablesMetaData[data.Keyspace][data.Table][data.OrderBy.Column]; ok {
				if !colMeta.IsCollection {
					orderByKey := t.SchemaMappingConfig.SystemColumnFamily + "['" + data.OrderBy.Column + "']"
					btQuery = btQuery + " ORDER BY " + orderByKey + " " + data.OrderBy.Operation
				} else {
					return "", errors.New("order by on collection data type is not supported")
				}
			} else {
				return "", errors.New("order by column is not correct")
			}

		}

		if data.Limit.IsLimit {
			val := data.Limit.Count
			if val == questionMark || strings.Contains(val, questionMark) {
				val = "@" + limitPlaceholder
			}
			btQuery = btQuery + " " + "LIMIT" + " " + val
		}

		btQuery += ";"
		return btQuery, nil
	}
	return "", errors.New("could not prepare the select query due to incomplete information")

}

// TranslateSelectQuerytoBigtable() Translates Cassandra select statement into a compatible Cloud Bigtable select query.
//
// Parameters:
//   - query: CQL Select statement
//
// Returns: SelectQueryMap struct and error if any
func (t *Translator) TranslateSelectQuerytoBigtable(originalQuery string) (*SelectQueryMap, error) {
	lowerQuery := strings.ToLower(originalQuery)

	//Create copy of cassandra query where literals are substituted with a suffix
	query := renameLiterals(originalQuery)

	p, err := NewCqlParser(query, false)
	if err != nil {
		return nil, err
	}
	selectObj := p.Select_()
	if selectObj == nil {
		// Todo - Code is not reachable
		return nil, errors.New("ToBigtableSelect: Could not parse select object")
	}

	kwSelectObj := selectObj.KwSelect()
	if kwSelectObj == nil {
		// Todo - Code is not reachable
		return nil, errors.New("ToBigtableSelect: Could not parse select object")
	}

	queryType := kwSelectObj.GetText()
	columns, err := parseColumnsFromSelect(selectObj.SelectElements())
	if err != nil {
		return nil, err
	}

	tableSpec, err := parseTableFromSelect(selectObj.FromSpec())
	if err != nil {
		return nil, err
	}
	if !t.SchemaMappingConfig.InstanceExist(tableSpec.KeyspaceName) {
		return nil, fmt.Errorf("keyspace %s does not exist", tableSpec.KeyspaceName)
	}
	if !t.SchemaMappingConfig.TableExist(tableSpec.KeyspaceName, tableSpec.TableName) {
		return nil, fmt.Errorf("table %s does not exist", tableSpec.TableName)
	}
	var clauseResponse ClauseResponse

	if hasWhere(lowerQuery) {
		resp, err := parseWhereByClause(selectObj.WhereSpec(), tableSpec.TableName, t.SchemaMappingConfig, tableSpec.KeyspaceName)
		if err != nil {
			return nil, err
		}
		clauseResponse = *resp
	}

	var orderBy OrderBy
	if hasOrderBy(lowerQuery) {
		orderBy = parseOrderByFromSelect(selectObj.OrderSpec())
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
			clauseResponse.ParamKeys = append(clauseResponse.ParamKeys, limitPlaceholder)
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

	selectQueryData := &SelectQueryMap{
		Query:           query,
		TranslatedQuery: "",
		QueryType:       queryType,
		Table:           tableSpec.TableName,
		Keyspace:        tableSpec.KeyspaceName,
		ColumnMeta:      columns,
		Clauses:         clauseResponse.Clauses,
		Limit:           limit,
		OrderBy:         orderBy,
		Params:          clauseResponse.Params,
		ParamKeys:       clauseResponse.ParamKeys,
	}

	translatedResult, err := getBigtableSelectQuery(t, selectQueryData)
	if err != nil {
		return nil, err
	}

	selectQueryData.TranslatedQuery = translatedResult
	return selectQueryData, nil
}
