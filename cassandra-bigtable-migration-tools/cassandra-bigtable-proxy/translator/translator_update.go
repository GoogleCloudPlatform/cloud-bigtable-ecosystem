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
	"slices"
	"strconv"
	"strings"

	methods "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/methods"
	types "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	schemaMapping "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/schema-mapping"
	cql "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/third_party/cqlparser"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/utilities"
	"github.com/antlr4-go/antlr/v4"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
)

// Helper function to check if a node is a column reference
func isColumn(node antlr.Tree, colName string) bool {
	if tn, ok := node.(antlr.TerminalNode); ok {
		return tn.GetText() == colName
	}
	return false
}

// Helper function to check if a node is a collection type
func isCollection(node antlr.Tree) bool {
	switch node.(type) {
	case cql.IAssignmentListContext, cql.IAssignmentSetContext, cql.IAssignmentMapContext:
		return true
	}
	return false
}

// Helper function to get text from a node
func getNodeText(node antlr.Tree, parser antlr.Parser) string {
	if tn, ok := node.(antlr.TerminalNode); ok {
		return tn.GetText()
	}
	return antlr.TreesGetNodeText(node, nil, parser)
}

// Helper function to get value from a node
func getNodeValue(node antlr.Tree, parent antlr.ParserRuleContext) interface{} {
	switch ctx := node.(type) {
	case cql.IAssignmentListContext:
		val, _ := parseCqlValue(ctx)
		return val
	case cql.IAssignmentSetContext:
		val, _ := parseCqlValue(ctx)
		return val
	case cql.IAssignmentMapContext:
		val, _ := parseCqlValue(ctx)
		return val
	case antlr.TerminalNode:
		return ctx.GetText()
	default:
		if parser, ok := parent.(interface{ GetParser() antlr.Parser }); ok {
			return antlr.TreesGetNodeText(node, nil, parser.GetParser())
		}
		return antlr.TreesGetNodeText(node, nil, nil)
	}

}

// parseAssignments() processes a list of assignment elements from a CQL update statement,
// generating a structured response that includes the assignments' details and their corresponding
// placeholder values for parameterized queries.
//
// Parameters:
//   - assignments: A slice of IAssignmentElementContext, each representing an assignment in the CQL query.
//   - tableName: The name of the table being updated.
//   - schemaMapping: A pointer to SchemaMappingConfig holding schema information for the table.
//   - keyspace: The name of the keyspace containing the table.
//
// Returns:
//   - A pointer to an UpdateSetResponse which contains structured information about the assignments,
//     including updated values, parameter keys, and parameters map.
//   - An error if invalid input is detected, such as an empty assignment list or issues with assignment syntax,
//     or if a column type cannot be retrieved from the schema mapping table, or if an attempt is made to assign
//     a value to a primary key.
func parseAssignments(assignments []cql.IAssignmentElementContext, tableConfig *schemaMapping.TableConfig, prependColumn *[]string, isPreparedQuery bool) (*UpdateSetResponse, error) {
	if len(assignments) == 0 {
		return nil, errors.New("invalid input")
	}
	var setResp []UpdateSetValue
	var paramKeys []string
	params := make(map[string]interface{})

	for i, setVal := range assignments {
		colObj := setVal.OBJECT_NAME(0)
		if colObj == nil {
			return nil, errors.New("error parsing column for assignments")
		}
		columnName := colObj.GetText()
		if columnName == "" {
			return nil, errors.New("no columnName found for assignments")
		}
		columnName = strings.ReplaceAll(columnName, literalPlaceholder, "")

		var value interface{}
		var err error

		// Handle append, prepend, remove as ComplexAssignment
		if setVal.PLUS() != nil || setVal.MINUS() != nil {
			var op string = "+"
			if setVal.MINUS() != nil {
				op = "-"
			}
			// Find the operator index
			var opIndex int
			for i := 0; i < setVal.GetChildCount(); i++ {
				child := setVal.GetChild(i)
				if token, ok := child.(antlr.TerminalNode); ok && token.GetText() == op {
					opIndex = i
					break
				}
			}
			left := setVal.GetChild(opIndex - 1)
			right := setVal.GetChild(opIndex + 1)
			var leftVal, rightVal interface{}
			if isColumn(left, columnName) && isCollection(right) && op == "+" {
				// Append: col = col + [values]
				leftVal = getNodeText(left, setVal.GetParser())
				rightVal = getNodeValue(right, setVal)
			} else if isCollection(left) && isColumn(right, columnName) && op == "+" {
				// Prepend: col = [values] + col
				leftVal = getNodeValue(left, setVal)
				rightVal = getNodeText(right, setVal.GetParser())
				*prependColumn = append(*prependColumn, columnName)
			} else {
				// fallback: handle error or other cases
				leftVal = getNodeText(left, setVal.GetParser())
				rightVal = getNodeValue(right, setVal)
			}
			complexOp := ComplexAssignment{
				Column:    columnName,
				Operation: op,
				Left:      leftVal,
				Right:     rightVal,
			}
			value = complexOp
		} else if setVal.AssignmentList() != nil {
			// i.e. marks = [ ... ] or marks = []
			value, err = parseCqlValue(setVal.AssignmentList())
			if err != nil {
				return nil, err
			}
			var val interface{}
			column, err := tableConfig.GetColumn(columnName)
			if err != nil {
				return nil, err
			}
			if column.IsPrimaryKey {
				return nil, fmt.Errorf("primary key not allowed to assignments")
			}
			if value != questionMark {
				if utilities.IsCollectionColumn(column) {
					val = value
				} else {
					val, err = formatValues(fmt.Sprintf("%v", value), column.CQLType, 4)
					if err != nil {
						return nil, err
					}
				}
				params["set"+strconv.Itoa(i+1)] = val
			}
			paramKeys = append(paramKeys, "set"+strconv.Itoa(i+1))
			cqlTypeStr, err := methods.ConvertCQLDataTypeToString(column.CQLType)
			if err != nil {
				return nil, err
			}
			setResp = append(setResp, UpdateSetValue{
				Column:    columnName,
				Value:     "@set" + strconv.Itoa(i+1),
				Encrypted: val,
				CQLType:   cqlTypeStr,
			})
			continue // Prevent falling through to the rest of the loop
		} else if setVal.SyntaxBracketLs() != nil && setVal.DecimalLiteral() != nil && setVal.SyntaxBracketRs() != nil && setVal.Constant() != nil {
			// i.e. marks[1] = 99 (index update)
			index := strings.Trim(setVal.DecimalLiteral().GetText(), "'")
			valueRaw, err := parseCqlConstant(setVal.Constant())
			if err != nil {
				return nil, err
			}
			complexOp := ComplexAssignment{
				Column:    columnName,
				Operation: "update_index",
				Left:      index,
				Right:     valueRaw,
			}
			value = complexOp
		} else if setVal.Constant() != nil {
			value, err = parseCqlConstant(setVal.Constant())
		} else if setVal.AssignmentMap() != nil {
			value, err = parseCqlValue(setVal.AssignmentMap())
		} else if setVal.AssignmentSet() != nil {
			value, err = parseCqlValue(setVal.AssignmentSet())
		} else if setVal.QUESTION_MARK() != nil {
			value = questionMark
		} else {
			value = setVal.GetText()
		}
		if err != nil {
			return nil, err
		}

		var val interface{} // encrypted val
		column, err := tableConfig.GetColumn(columnName)
		if err != nil {
			return nil, err
		}
		if column.IsPrimaryKey {
			return nil, fmt.Errorf("primary key not allowed to assignments")
		}
		if !isPreparedQuery {
			if utilities.IsCollectionColumn(column) {
				val = value
			} else {
				val, err = formatValues(fmt.Sprintf("%v", value), column.CQLType, 4)
				if err != nil {
					return nil, err
				}
			}
			params["set"+strconv.Itoa(i+1)] = val
		} else {
			val = value
		}
		paramKeys = append(paramKeys, "set"+strconv.Itoa(i+1))
		cqlTypeStr, err := methods.ConvertCQLDataTypeToString(column.CQLType)
		if err != nil {
			return nil, err
		}
		setResp = append(setResp, UpdateSetValue{
			Column:    columnName,
			Value:     "@set" + strconv.Itoa(i+1),
			Encrypted: val,
			CQLType:   cqlTypeStr,
		})
	}
	return &UpdateSetResponse{
		UpdateSetValues: setResp,
		ParamKeys:       paramKeys,
		Params:          params,
	}, nil
}

// TranslateUpdateQuerytoBigtable() frames UpdateQueryMapping which translates the update to bigtable mutation
//
// Parameters:
//   - query: CQL Update query
//
// Returns: UpdateQueryMapping struct and error if any
func (t *Translator) TranslateUpdateQuerytoBigtable(queryStr string, isPreparedQuery bool, sessionKeyspace string) (*UpdateQueryMapping, error) {
	lowerQuery := strings.ToLower(queryStr)
	query := renameLiterals(queryStr)
	lexer := cql.NewCqlLexer(antlr.NewInputStream(query))
	stream := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)
	p := cql.NewCqlParser(stream)
	updateObj := p.Update()
	var PrependColumns []string

	if updateObj == nil || updateObj.KwUpdate() == nil {
		return nil, errors.New("error parsing the update object")
	}

	keyspace := updateObj.Keyspace()
	updateObj.DOT()
	table := updateObj.Table()

	var keyspaceName, tableName string

	if table != nil && table.OBJECT_NAME() != nil && table.OBJECT_NAME().GetText() != "" {
		tableName = table.OBJECT_NAME().GetText()
	} else {
		return nil, fmt.Errorf("invalid input paramaters found for table")
	}

	if keyspace != nil && keyspace.OBJECT_NAME() != nil && keyspace.OBJECT_NAME().GetText() != "" {
		keyspaceName = keyspace.OBJECT_NAME().GetText()
	} else if sessionKeyspace != "" {
		keyspaceName = sessionKeyspace
	} else {
		return nil, fmt.Errorf("invalid input paramaters found for keyspace")
	}

	tableConfig, err := t.SchemaMappingConfig.GetTableConfig(keyspaceName, tableName)
	if err != nil {
		return nil, err
	}

	timestampInfo, err := GetTimestampInfoByUpdate(queryStr, updateObj)
	if err != nil {
		return nil, err
	}
	updateObj.KwSet()
	if updateObj.Assignments() == nil || updateObj.Assignments().AllAssignmentElement() == nil {
		return nil, errors.New("error parsing the assignment object")
	}

	assignmentObj := updateObj.Assignments()
	allAssignmentObj := assignmentObj.AllAssignmentElement()
	if allAssignmentObj == nil {
		return nil, errors.New("error parsing all the assignment object")
	}
	setValues, err := parseAssignments(allAssignmentObj, tableConfig, &PrependColumns, isPreparedQuery)
	if err != nil {
		return nil, err
	}

	var QueryClauses QueryClauses

	if hasWhere(lowerQuery) {
		resp, err := parseWhereByClause(updateObj.WhereSpec(), tableConfig)
		if err != nil {
			return nil, err
		}

		QueryClauses = *resp

		for k, v := range QueryClauses.Params {
			setValues.Params[k] = v
		}

		setValues.ParamKeys = append(setValues.ParamKeys, QueryClauses.ParamKeys...)
	}
	ifExistObj := updateObj.IfExist()
	var ifExist bool = false
	if ifExistObj != nil {
		val := strings.ToLower(ifExistObj.GetText())
		if val == ifExists {
			ifExist = true
		}
	}

	primaryKeys := tableConfig.GetPrimaryKeys()
	var actualPrimaryKeys []string
	for _, val := range QueryClauses.Clauses {
		actualPrimaryKeys = append(actualPrimaryKeys, val.Column)
	}
	if !ValidateRequiredPrimaryKeys(primaryKeys, actualPrimaryKeys) {
		missingPrime := findFirstMissingKey(primaryKeys, actualPrimaryKeys)
		missingPkColumnType, err := tableConfig.GetPkKeyType(missingPrime)
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("some %s key parts are missing: %s", missingPkColumnType, missingPrime)
	}
	if err != nil {
		return nil, err
	}
	var primkeyvalues []string
	var rowKey string
	var values []interface{}
	var columns []types.Column
	for _, val := range setValues.UpdateSetValues {
		values = append(values, val.Encrypted)
		cqlType, err := methods.GetCassandraColumnType(val.CQLType)
		if err != nil {
			return nil, err
		}
		columns = append(columns, types.Column{Name: val.Column, ColumnFamily: t.SchemaMappingConfig.SystemColumnFamily, CQLType: cqlType})
	}
	var newValues []interface{} = values
	var newColumns []types.Column = columns
	var delColumns []types.Column
	var delColumnFamily []string
	var complexMeta map[string]*ComplexOperation
	var rawOutput *ProcessRawCollectionsOutput // Declare rawOutput here

	if !isPreparedQuery {
		pkValues := make(map[string]interface{})

		for _, key := range primaryKeys {
			for _, clause := range QueryClauses.Clauses {
				if !clause.IsPrimaryKey {
					return nil, errors.New("any value other then primary key is not accepted in where clause")
				}
				if clause.IsPrimaryKey && clause.Operator == "=" && key == clause.Column {
					pv := clause.Value
					if strings.HasPrefix(clause.Value, "@") {
						pv = clause.Value[1:]
					}
					pkValues[clause.Column] = QueryClauses.Params[pv]
					primkeyvalues = append(primkeyvalues, fmt.Sprintf("%v", QueryClauses.Params[pv]))
				}
			}
		}
		if len(primkeyvalues) != len(primaryKeys) {
			return nil, errors.New("missing primary key values in where clause")
		}
		pmks := tableConfig.GetPkByTableNameWithFilter(primaryKeys)
		rowKeyBytes, err := createOrderedCodeKey(pmks, pkValues, t.EncodeIntValuesWithBigEndian)
		if err != nil {
			return nil, err
		}
		rowKey = string(rowKeyBytes)

		// Building new colum family, qualifier and values for collection type of data.
		rawInput := ProcessRawCollectionsInput{
			Columns:        columns,
			Values:         values,
			TableName:      tableName,
			Translator:     t,
			KeySpace:       keyspaceName,
			PrependColumns: PrependColumns,
		}
		rawOutput, err = processCollectionColumnsForRawQueries(tableConfig, rawInput)
		if err != nil {
			return nil, fmt.Errorf("error processing raw collection columns: %w", err)
		}
		newColumns = rawOutput.NewColumns
		newValues = rawOutput.NewValues
		delColumnFamily = rawOutput.DelColumnFamily
		delColumns = rawOutput.DelColumns
		complexMeta = rawOutput.ComplexMeta // Assign complexMeta from output

		for _, val := range QueryClauses.Clauses {
			var column types.Column
			if columns, exists := tableConfig.Columns[val.Column]; exists {
				column = types.Column{Name: columns.Name, ColumnFamily: tableConfig.SystemColumnFamily, CQLType: columns.CQLType}
			}
			newColumns = append(newColumns, column)

			pv := val.Value
			if strings.HasPrefix(val.Value, "@") {
				pv = val.Value[1:]
			}
			value := fmt.Sprintf("%v", QueryClauses.Params[pv])
			encryVal, err := formatValues(value, column.CQLType, 4)
			if err != nil {
				return nil, err
			}
			newValues = append(newValues, encryVal)
		}
	} else {
		complexMeta, err = t.ProcessComplexUpdate(columns, values, tableName, keyspaceName, PrependColumns)
		if err != nil {
			return nil, err
		}
	}

	updateQueryData := &UpdateQueryMapping{
		Query:                 query,
		QueryType:             UPDATE,
		Table:                 tableName,
		RowKey:                rowKey,
		Columns:               newColumns,
		Values:                newValues,
		DeleteColumnFamilies:  delColumnFamily,
		DeleteColumQualifires: delColumns,
		IfExists:              ifExist,
		Keyspace:              keyspaceName,
		Clauses:               QueryClauses.Clauses,
		Params:                setValues.Params,
		ParamKeys:             setValues.ParamKeys,
		UpdateSetValues:       setValues.UpdateSetValues,
		PrimaryKeys:           primaryKeys,
		TimestampInfo:         timestampInfo,
		ComplexOperation:      complexMeta,
	}

	return updateQueryData, nil
}

// BuildUpdatePrepareQuery() constructs an UpdateQueryMapping for an update operation, preparing the necessary
// components such as columns, values, row keys, and primary keys. It also processes any collection types
// and manages timestamp information for the update query.
//
// Parameters:
//   - columnsResponse: A slice of Column structs representing metadata of columns involved in the update.
//   - values: A slice of pointers to primitive.Value, representing the values for the update operation.
//   - st: A pointer to an UpdateQueryMapping that contains existing query data and metadata.
//   - protocolV: The Cassandra protocol version used for encoding and decoding operations.
//
// Returns:
//   - A pointer to an UpdateQueryMapping populated with the new query components required for the update.
//   - An error if any issues arise during processing, such as failure to fetch primary keys or errors
//     handling column data or timestamp.
func (t *Translator) BuildUpdatePrepareQuery(columnsResponse []types.Column, values []*primitive.Value, st *UpdateQueryMapping, protocolV primitive.ProtocolVersion) (*UpdateQueryMapping, error) {
	var newColumns []types.Column
	var newValues []interface{}
	var primaryKeys []string = st.PrimaryKeys
	var err error
	var unencrypted map[string]interface{}
	var delColumnFamily []string
	var delColumns []types.Column                      // Added missing declaration
	var prepareOutput *ProcessPrepareCollectionsOutput // Declare prepareOutput

	tableConfig, err := t.SchemaMappingConfig.GetTableConfig(st.Keyspace, st.Table)
	if err != nil {
		return nil, err
	}

	if len(primaryKeys) == 0 {
		primaryKeys = tableConfig.GetPrimaryKeys()
	}
	timestampInfo, values, variableMetadata, err := ProcessTimestampByUpdate(st, values)

	if err != nil {
		return nil, err
	}

	prepareInput := ProcessPrepareCollectionsInput{
		ColumnsResponse: columnsResponse,
		Values:          values,
		TableName:       st.Table,
		ProtocolV:       protocolV,
		PrimaryKeys:     primaryKeys,
		Translator:      t,
		KeySpace:        st.Keyspace,
		ComplexMeta:     st.ComplexOperation,
	}

	prepareOutput, err = processCollectionColumnsForPrepareQueries(tableConfig, prepareInput)
	if err != nil {
		return nil, err
	}
	newColumns = prepareOutput.NewColumns
	newValues = prepareOutput.NewValues
	unencrypted = prepareOutput.Unencrypted
	indexEnd := prepareOutput.IndexEnd
	delColumnFamily = prepareOutput.DelColumnFamily
	delColumns = prepareOutput.DelColumns

	for i, clause := range st.Clauses {
		var column types.Column
		if columns, exists := tableConfig.Columns[clause.Column]; exists {
			column = types.Column{Name: columns.Name}
		}

		if slices.Contains(primaryKeys, column.Name) {

			val, _ := utilities.DecodeBytesToCassandraColumnType(values[i+indexEnd+1].Contents, variableMetadata[i+indexEnd+1].Type, protocolV)
			unencrypted[column.Name] = val
		}
	}

	pmks := tableConfig.GetPkByTableNameWithFilter(primaryKeys)
	rowKeyBytes, err := createOrderedCodeKey(pmks, unencrypted, t.EncodeIntValuesWithBigEndian)
	if err != nil {
		return nil, err
	}
	rowKey := string(rowKeyBytes)

	UpdateQueryData := &UpdateQueryMapping{
		Query:                 st.Query,
		QueryType:             st.QueryType,
		Keyspace:              st.Keyspace,
		Columns:               newColumns,
		Values:                newValues,
		PrimaryKeys:           primaryKeys,
		RowKey:                rowKey,
		Table:                 st.Table,
		DeleteColumnFamilies:  delColumnFamily,
		DeleteColumQualifires: delColumns,
		Clauses:               st.Clauses,
		TimestampInfo:         timestampInfo,
		IfExists:              st.IfExists,
		ComplexOperation:      st.ComplexOperation,
	}

	return UpdateQueryData, nil
}
