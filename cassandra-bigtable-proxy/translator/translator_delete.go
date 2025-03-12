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
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	schemaMapping "github.com/ollionorg/cassandra-to-bigtable-proxy/schema-mapping"
	cql "github.com/ollionorg/cassandra-to-bigtable-proxy/translator/cqlparser"
	"github.com/ollionorg/cassandra-to-bigtable-proxy/utilities"
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

	tableObj, keyspaceObj, err := getTableAndKeyspaceObjects(allObj)
	if err != nil {
		return nil, err
	}

	response := TableObj{
		TableName:    tableObj.GetText(),
		KeyspaceName: keyspaceObj.GetText(),
	}

	return &response, nil
}

// getFromSpecElement() retrieves the first FROM specification element from the given context.
//
// Parameters:
//   - input: A context interface of a FROM specification in a CQL query.
//
// Returns:
//   - An element context containing FROM specification details.
//   - An error if the element context is not present.
func getFromSpecElement(input cql.IFromSpecContext) (cql.IFromSpecElementContext, error) {
	fromSpec := input.FromSpecElement()
	if fromSpec == nil {
		return nil, errors.New("error while parsing fromSpec")
	}
	return fromSpec, nil
}

// getAllObjectNames() retrieves all object names (such as tables and keyspaces) from the FROM specification element.
//
// Parameters:
//   - fromSpec: A context interface representing an element of a FROM specification in a CQL query.
//
// Returns:
//   - A slice of TerminalNode containing all OBJECT_NAME entries.
//   - An error if no object names are found or if parsing fails.
func getAllObjectNames(fromSpec cql.IFromSpecElementContext) ([]antlr.TerminalNode, error) {
	allObj := fromSpec.AllOBJECT_NAME()
	if allObj == nil {
		return nil, errors.New("error while parsing all objects from the fromSpec")
	}
	if len(allObj) == 0 {
		return nil, errors.New("could not find table and keyspace name")
	}
	return allObj, nil
}

// getTableAndKeyspaceObjects() extracts the table and keyspace tokens from a list of object names.
//
// Parameters:
//   - allObj: A slice of TerminalNode representing object names parsed from a FROM specification.
//
// Returns:
//   - tableObj: The TerminalNode representing the table name.
//   - keyspaceObj: The TerminalNode representing the keyspace name.
//   - An error if the expected two tokens (table and keyspace) are not found.
func getTableAndKeyspaceObjects(allObj []antlr.TerminalNode) (tableObj, keyspaceObj antlr.TerminalNode, err error) {
	if len(allObj) == 2 {
		return allObj[1], allObj[0], nil
	}
	return nil, nil, errors.New("could not find table or keyspace name")
}

// parseClauseFromDelete() parse Clauses from the Delete Query
//
// Parameters:
//   - input: The Where Spec context from the antlr Parser.
//   - tableName - Table Name
//   - schemaMapping - JSON Config which maintains column and its datatypes info.
//
// Returns: ClauseResponse and an error if any.
func parseClauseFromDelete(input cql.IWhereSpecContext, tableName string, schemaMapping *schemaMapping.SchemaMappingConfig, keyspace string) (*ClauseResponse, error) {
	if input == nil {
		return nil, errors.New("no input parameters found for clauses")
	}

	elements, err := getRelationElements(input)
	if err != nil {
		return nil, err
	}

	if len(elements) == 0 {
		return &ClauseResponse{}, nil
	}

	clauses, params, paramKeys, err := processElements(elements, tableName, schemaMapping, keyspace)
	if err != nil {
		return nil, err
	}

	return &ClauseResponse{
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
func processElements(elements []cql.IRelationElementContext, tableName string, schemaMapping *schemaMapping.SchemaMappingConfig, keyspace string) ([]Clause, map[string]interface{}, []string, error) {
	var clauses []Clause
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

		columnType, err := schemaMapping.GetColumnType(tableName, colName, keyspace)
		if err != nil {
			return nil, nil, nil, err
		}

		acctualVal, err := handleColumnType(val, columnType, placeholder, params)
		if err != nil {
			return nil, nil, nil, err
		}

		clause := Clause{
			Column:       colName,
			Operator:     operator,
			Value:        acctualVal,
			IsPrimaryKey: columnType.IsPrimaryKey,
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
func handleColumnType(val cql.IRelationElementContext, columnType *schemaMapping.ColumnType, placeholder string, params map[string]interface{}) (string, error) {
	if columnType == nil || columnType.CQLType == "" {
		return "", nil
	}

	valConst := val.Constant()
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
// Returns: ClauseResponse and an error if any.
func (t *Translator) TranslateDeleteQuerytoBigtable(queryStr string) (*DeleteQueryMap, error) {
	lowerQuery := strings.ToLower(queryStr)
	query := renameLiterals(queryStr)
	lexer := cql.NewCqlLexer(antlr.NewInputStream(query))
	stream := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)
	p := cql.NewCqlParser(stream)

	deleteObj := p.Delete_()
	if deleteObj == nil {
		return nil, errors.New("error while parsing delete object")
	}
	kwDeleteObj := deleteObj.KwDelete()
	if kwDeleteObj == nil {
		return nil, errors.New("error while parsing delete object")
	}
	queryType := kwDeleteObj.GetText()
	tableSpec, err := parseTableFromDelete(deleteObj.FromSpec())
	if err != nil {
		return nil, err
	} else if tableSpec.TableName == "" || tableSpec.KeyspaceName == "" {
		return nil, errors.New("TranslateDeletetQuerytoBigtable: No table or keyspace name found in the query")
	}
	if !t.SchemaMappingConfig.InstanceExist(tableSpec.KeyspaceName) {
		return nil, fmt.Errorf("keyspace %s does not exist", tableSpec.KeyspaceName)
	}
	if !t.SchemaMappingConfig.TableExist(tableSpec.KeyspaceName, tableSpec.TableName) {
		return nil, fmt.Errorf("table %s does not exist", tableSpec.TableName)
	}
	selectedColumns, err := parseDeleteColumns(deleteObj.DeleteColumnList(), tableSpec.TableName, t.SchemaMappingConfig, tableSpec.KeyspaceName)
	if err != nil {
		return nil, err
	}
	timestampInfo, err := GetTimestampInfoForRawDelete(lowerQuery, deleteObj)
	if err != nil {
		return nil, err
	}
	ifExistObj := deleteObj.IfExist()
	var ifExists bool = false
	if ifExistObj != nil {
		val := strings.ToLower(ifExistObj.GetText())
		if val == "ifexists" {
			ifExists = true
		}
	}

	var clauseResponse ClauseResponse

	if hasWhere(lowerQuery) {
		resp, err := parseClauseFromDelete(deleteObj.WhereSpec(), tableSpec.TableName, t.SchemaMappingConfig, tableSpec.KeyspaceName)
		if err != nil {
			return nil, errors.New("TranslateDeletetQuerytoBigtable: Invalid Where clause condition")
		}
		clauseResponse = *resp
	}

	primaryKeys, err := getPrimaryKeys(t.SchemaMappingConfig, tableSpec.TableName, tableSpec.KeyspaceName)
	if err != nil {
		return nil, err
	}
	var primaryKeysFound []string

	for _, key := range primaryKeys {
		for _, clause := range clauseResponse.Clauses {
			if !clause.IsPrimaryKey {
				return nil, fmt.Errorf("non PRIMARY KEY columns found in where clause: %s", clause.Column)
			}
			if clause.IsPrimaryKey && clause.Operator == "=" && key == clause.Column {
				primaryKeysFound = append(primaryKeysFound, fmt.Sprintf("%v", clause.Value))
			}
		}
	}
	if len(primaryKeysFound) != len(primaryKeys) && len(clauseResponse.Clauses) > 0 {

		missingPrime := findFirstMissingKey(primaryKeys, primaryKeysFound)
		missingPkColumnType, err := t.SchemaMappingConfig.GetPkKeyType(tableSpec.TableName, tableSpec.KeyspaceName, missingPrime)
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("some %s key parts are missing: %s", missingPkColumnType, missingPrime)
	}
	rowKey := strings.Join(primaryKeysFound[:], "#")
	deleteQueryData := &DeleteQueryMap{
		Query:           query,
		QueryType:       queryType,
		Table:           tableSpec.TableName,
		Keyspace:        tableSpec.KeyspaceName,
		Clauses:         clauseResponse.Clauses,
		Params:          clauseResponse.Params,
		ParamKeys:       clauseResponse.ParamKeys,
		PrimaryKeys:     primaryKeys,
		RowKey:          rowKey,
		TimestampInfo:   timestampInfo,
		IfExists:        ifExists,
		SelectedColumns: selectedColumns,
	}
	return deleteQueryData, nil
}

// BuildDeletePrepareQuery() Function to accept the values clause columns and form the rowKey and return the same
func (t *Translator) BuildDeletePrepareQuery(values []*primitive.Value, st *DeleteQueryMap, variableColumnMetadata []*message.ColumnMetadata, protocolV primitive.ProtocolVersion) (string, TimestampInfo, error) {
	parts := strings.Split(st.RowKey, "#")
	timestamp, values, err := ProcessTimestampByDelete(st, values)
	if err != nil {
		return "", TimestampInfo{}, fmt.Errorf("error while getting timestamp value")
	}
	if len(parts) != len(values) && len(parts)-1 != len(values) {
		return "", TimestampInfo{}, fmt.Errorf("number of placeholders in RowKey does not match number of values")
	}

	valueMap := make(map[string]interface{})
	for i, col := range variableColumnMetadata {
		columnType, _ := t.SchemaMappingConfig.GetColumnType(st.Table, col.Name, st.Keyspace)
		val, _ := utilities.DecodeEncodedValues(values[i].Contents, columnType.CQLType, protocolV)
		valueMap[col.Name] = val
	}

	for i, key := range st.PrimaryKeys {
		val, ok := valueMap[key]
		if !ok {
			return "", TimestampInfo{}, fmt.Errorf("key %s not found in value map", key)
		}
		convertedVal, _ := primitivesToString(val)
		parts[i] = convertedVal
	}

	rowKey := strings.Join(parts, "#")
	return rowKey, timestamp, nil
}

// Parses the delete columns from a CQL DELETE statement and returns the selected columns with their associated map keys or list indices.
func parseDeleteColumns(deleteColumns cql.IDeleteColumnListContext, tableName string, tableConf *schemaMapping.SchemaMappingConfig, keySpace string) ([]schemaMapping.SelectedColumns, error) {
	if deleteColumns == nil {
		return nil, nil
	}
	cols := deleteColumns.AllDeleteColumnItem()
	var Columns []schemaMapping.SelectedColumns
	var decimalLiteral, stringLiteral string
	for _, v := range cols {
		var Column schemaMapping.SelectedColumns
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
		_, err := tableConf.GetColumnType(tableName, Column.Name, keySpace)
		if err != nil {
			return nil, fmt.Errorf("undefined column name %s in table %s.%s", Column.Name, keySpace, tableName)
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
