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

	types "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	schemaMapping "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/schema-mapping"
	cql "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/third_party/cqlparser"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/utilities"
	"github.com/antlr4-go/antlr/v4"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
)

// parseTableFromDelete() extracts the table and keyspace information from the input context of a DELETE query.
//
// Parameters:
//   - input: A context interface representing the FROM specification of a CQL DELETE query.
//
// Returns:
//   - A pointer to a TableObj containing the extracted table and keyspace names.
//   - An error if the input is nil, or if there are issues in parsing the table or keyspace names.
func parseTableFromDelete(input cql.IFromSpecContext) (*TableObj, error) {
	if input == nil {
		return nil, errors.New("no input parameters found for table and keyspace")
	}

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

	response := TableObj{
		TableName:    tableName,
		KeyspaceName: keyspaceName,
	}

	return &response, nil
}

// parseClauseFromDelete() parse Clauses from the Delete Query
//
// Parameters:
//   - input: The Where Spec context from the antlr Parser.
//   - tableName - Table Name
//   - schemaMapping - JSON Config which maintains column and its datatypes info.
//
// Returns: QueryClauses and an error if any.
func parseClauseFromDelete(input cql.IWhereSpecContext, tableConfig *schemaMapping.TableConfig) (*QueryClauses, error) {
	if input == nil {
		return nil, errors.New("no input parameters found for clauses")
	}

	elements, err := getRelationElements(input)
	if err != nil {
		return nil, err
	}

	if len(elements) == 0 {
		return &QueryClauses{}, nil
	}

	clauses, params, paramKeys, err := processElements(tableConfig, elements)
	if err != nil {
		return nil, err
	}

	return &QueryClauses{
		Clauses:   clauses,
		Params:    params,
		ParamKeys: paramKeys,
	}, nil
}

// getRelationElements() retrieves all relation elements from the WHERE clause of a CQL query.
//
// Parameters:
//   - input: A context interface representing the WHERE specification of a CQL query.
//
// Returns:
//   - A slice of IRelationElementContext, each representing a clause in the WHERE condition.
//   - An error if no relation elements are found in the input.
func getRelationElements(input cql.IWhereSpecContext) ([]cql.IRelationElementContext, error) {
	elements := input.RelationElements().AllRelationElement()
	if elements == nil {
		return nil, errors.New("no input parameters found for clauses")
	}
	return elements, nil
}

// processElements() processes a list of relation elements and generates corresponding clauses,
// parameters, and parameter keys. It retrieves column types to handle values appropriately.
//
// Parameters:
//   - elements: A slice of IRelationElementContext representing WHERE clause elements.
//   - tableName: The name of the table involved in the query.
//   - schemaMapping: A pointer to SchemaMappingConfig for retrieving schema information.
//   - keyspace: The name of the keyspace containing the table.
//
// Returns:
//   - A slice of Clause structs each representing a WHERE condition.
//   - A map of parameters to use for prepared statements.
//   - A slice of strings representing parameter keys.
//   - An error if parsing column names, values, or column types fails.
func processElements(tableConfig *schemaMapping.TableConfig, elements []cql.IRelationElementContext) ([]types.Clause, map[string]interface{}, []string, error) {
	var clauses []types.Clause
	params := make(map[string]interface{})
	var paramKeys []string

	for i, val := range elements {
		if val == nil {
			return nil, nil, nil, errors.New("could not parse column object")
		}

		placeholder := "value" + strconv.Itoa(i+1)
		paramKeys = append(paramKeys, placeholder)

		colName, operator, err := parseColumnAndOperator(val)
		if err != nil {
			return nil, nil, nil, err
		}

		column, err := tableConfig.GetColumn(colName)
		if err != nil {
			return nil, nil, nil, err
		}

		actualVal, err := handleColumnType(val, column, placeholder, params)
		if err != nil {
			return nil, nil, nil, err
		}

		clause := types.Clause{
			Column:       colName,
			Operator:     operator,
			Value:        actualVal,
			IsPrimaryKey: column.IsPrimaryKey,
		}
		clauses = append(clauses, clause)
	}

	return clauses, params, paramKeys, nil
}

// parseColumnAndOperator() extracts the column name and operator from a relation element.
//
// Parameters:
//   - val: A relation element context from which the column and operator are parsed.
//
// Returns:
//   - A string representing the column name.
//   - A string representing the operator used in the relation.
//   - An error if parsing the column object or operator fails.
func parseColumnAndOperator(val cql.IRelationElementContext) (string, string, error) {
	colObj := val.OBJECT_NAME(0)
	if colObj == nil {
		return "", "", errors.New("could not parse column object")
	}

	operator, err := getOperator(val)
	if err != nil {
		return "", "", err
	}

	colName := strings.ReplaceAll(colObj.GetText(), literalPlaceholder, "")
	if colName == "" {
		return "", "", errors.New("could not parse column name")
	}

	return colName, operator, nil
}

// getOperator() determines the operator used in a relation element.
//
// Parameters:
//   - val: A relation element context from which the operator is extracted.
//
// Returns:
//   - A string representing the operator.
//   - An error if no supported operator is found.
func getOperator(val cql.IRelationElementContext) (string, error) {
	switch {
	case val.OPERATOR_EQ() != nil:
		return val.OPERATOR_EQ().GetText(), nil
	//
	default:
		return "", errors.New("no supported operator found")
	}
}

// handleColumnType() processes the value associated with a column, formats it if necessary,
// and updates the parameters map with the formatted value.
//
// Parameters:
//   - val: The relation element context containing the value to process.
//   - columnType: A pointer to ColumnType providing type information for the column.
//   - placeholder: A string used as the key in the parameters map.
//   - params: A map for storing formatted parameter values.
//
// Returns:
//   - A string representing the actual value.
//   - An error if parsing or formatting the value fails.
func handleColumnType(val cql.IRelationElementContext, columnType *types.Column, placeholder string, params map[string]interface{}) (string, error) {
	if columnType == nil || columnType.CQLType == nil {
		return "", nil
	}

	valConst := val.Constant(0)
	if valConst == nil {
		return "", errors.New("could not parse value from query for one of the clauses")
	}

	value := strings.ReplaceAll(valConst.GetText(), "'", "")
	if value == "" {
		return "", errors.New("could not parse value from query for one of the clauses")
	}

	acctualVal := value
	if value != "?" {
		formattedVal, err := formatValues(value, columnType.CQLType, 4)
		if err != nil {
			return "", err
		}
		params[placeholder] = formattedVal
	}

	return acctualVal, nil
}

// TranslateDeleteQuerytoBigtable() translate the CQL Delete Query into bigtable mutation api equivalent.
//
// Parameters:
//   - queryStr: CQL delete query with condition
//
// Returns: QueryClauses and an error if any.
func (t *Translator) TranslateDeleteQuerytoBigtable(queryStr string, isPreparedQuery bool, sessionKeyspace string) (*DeleteQueryMapping, error) {
	lowerQuery := strings.ToLower(queryStr)
	query := renameLiterals(queryStr)
	lexer := cql.NewCqlLexer(antlr.NewInputStream(query))
	stream := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)
	p := cql.NewCqlParser(stream)

	deleteObj := p.Delete_()
	if deleteObj == nil || deleteObj.KwDelete() == nil {
		return nil, errors.New("error while parsing delete object")
	}

	tableSpec, err := parseTableFromDelete(deleteObj.FromSpec())
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

	selectedColumns, err := parseDeleteColumns(deleteObj.DeleteColumnList(), tableConfig)
	if err != nil {
		return nil, err
	}
	timestampInfo, err := GetTimestampInfoForRawDelete(lowerQuery, deleteObj)
	if err != nil {
		return nil, err
	}
	ifExistObj := deleteObj.IfExist()
	var ifExist bool = false
	if ifExistObj != nil {
		val := strings.ToLower(ifExistObj.GetText())
		if val == ifExists {
			ifExist = true
		}
	}

	var QueryClauses QueryClauses

	if hasWhere(lowerQuery) {
		resp, err := parseClauseFromDelete(deleteObj.WhereSpec(), tableConfig)
		if err != nil {
			return nil, errors.New("TranslateDeletetQuerytoBigtable: Invalid Where clause condition")
		}
		QueryClauses = *resp
	}

	primaryKeys := tableConfig.GetPrimaryKeys()
	var primaryKeysFound []string
	pkValues := make(map[string]interface{})

	for _, key := range primaryKeys {
		for _, clause := range QueryClauses.Clauses {
			if !clause.IsPrimaryKey {
				return nil, fmt.Errorf("non PRIMARY KEY columns found in where clause: %s", clause.Column)
			}
			if clause.IsPrimaryKey && clause.Operator == "=" && key == clause.Column {
				pkValues[clause.Column] = clause.Value
				primaryKeysFound = append(primaryKeysFound, fmt.Sprintf("%v", clause.Value))
			}
		}
	}
	// The below code checking the required primary keys and actual primary keys when we are having clause statements
	if len(primaryKeysFound) != len(primaryKeys) && len(QueryClauses.Clauses) > 0 {
		missingPrime := findFirstMissingKey(primaryKeys, primaryKeysFound)
		missingPkColumnType, err := tableConfig.GetPkKeyType(missingPrime)
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("some %s key parts are missing: %s", missingPkColumnType, missingPrime)
	}

	var rowKey string
	if !isPreparedQuery {
		rowKeyBytes, err := createOrderedCodeKey(tableConfig, pkValues)
		if err != nil {
			return nil, fmt.Errorf("key encoding failed. %w", err)
		}
		rowKey = string(rowKeyBytes)
	}

	deleteQueryData := &DeleteQueryMapping{
		Query:           query,
		QueryType:       DELETE,
		Table:           tableName,
		Keyspace:        keyspaceName,
		Clauses:         QueryClauses.Clauses,
		Params:          QueryClauses.Params,
		ParamKeys:       QueryClauses.ParamKeys,
		PrimaryKeys:     primaryKeys,
		RowKey:          rowKey,
		TimestampInfo:   timestampInfo,
		IfExists:        ifExist,
		SelectedColumns: selectedColumns,
	}
	return deleteQueryData, nil
}

// BuildDeletePrepareQuery() Function to accept the values clause columns and form the rowKey and return the same
func (t *Translator) BuildDeletePrepareQuery(values []*primitive.Value, st *DeleteQueryMapping, variableColumnMetadata []*message.ColumnMetadata, protocolV primitive.ProtocolVersion) (string, TimestampInfo, error) {

	timestamp, values, err := ProcessTimestampByDelete(st, values)
	if err != nil {
		return "", TimestampInfo{}, fmt.Errorf("error while getting timestamp value")
	}

	valueMap := make(map[string]interface{})
	for i, col := range variableColumnMetadata {
		val, _ := utilities.DecodeBytesToCassandraColumnType(values[i].Contents, variableColumnMetadata[i].Type, protocolV)
		valueMap[col.Name] = val
	}

	tableConfig, err := t.SchemaMappingConfig.GetTableConfig(st.Keyspace, st.Table)
	if err != nil {
		return "", TimestampInfo{}, err
	}
	rowKeyBytes, err := createOrderedCodeKey(tableConfig, valueMap)
	if err != nil {
		return "", timestamp, fmt.Errorf("key encoding failed. %w", err)
	}
	rowKey := string(rowKeyBytes)
	return rowKey, timestamp, nil
}

// Parses the delete columns from a CQL DELETE statement and returns the selected columns with their associated map keys or list indices.
func parseDeleteColumns(deleteColumns cql.IDeleteColumnListContext, tableConfig *schemaMapping.TableConfig) ([]types.SelectedColumns, error) {
	if deleteColumns == nil {
		return nil, nil
	}
	cols := deleteColumns.AllDeleteColumnItem()
	var Columns []types.SelectedColumns
	var decimalLiteral, stringLiteral string
	for _, v := range cols {
		var Column types.SelectedColumns
		Column.Name = v.OBJECT_NAME().GetText()
		if v.LS_BRACKET() != nil {
			if v.DecimalLiteral() != nil { // for list index
				decimalLiteral = v.DecimalLiteral().GetText()
				Column.ListIndex = decimalLiteral
			}
			if v.StringLiteral() != nil { //for map Key
				stringLiteral = v.StringLiteral().GetText()
				stringLiteral = strings.Trim(stringLiteral, "'")
				Column.MapKey = stringLiteral
			}
		}
		_, err := tableConfig.GetColumnType(Column.Name)
		if err != nil {
			return nil, err
		}
		Columns = append(Columns, Column)
	}
	return Columns, nil
}

// findFirstMissingKey() finds the first primary key that's missing from primaryKeysFound
func findFirstMissingKey(primaryKeys []string, primaryKeysFound []string) string {
	foundMap := make(map[string]bool)
	for _, key := range primaryKeysFound {
		foundMap[key] = true
	}

	for _, key := range primaryKeys {
		if !foundMap[key] {
			return key
		}
	}
	return ""
}
