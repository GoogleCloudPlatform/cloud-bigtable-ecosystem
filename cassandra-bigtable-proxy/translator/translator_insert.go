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
	"strings"

	"github.com/antlr4-go/antlr/v4"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	schemaMapping "github.com/ollionorg/cassandra-to-bigtable-proxy/schema-mapping"
	cql "github.com/ollionorg/cassandra-to-bigtable-proxy/third_party/cqlparser"
)

// IsCollection() Function to determine if provided colunm if type of collection or not
// It returns true if given column is of collection type or else return false
func (t *Translator) IsCollection(keySpace, tableName, columnName string) bool {
	if colType, err := t.SchemaMappingConfig.GetColumnType(keySpace, tableName, columnName); err == nil {
		return colType.IsCollection
	}
	return false
}

// parseColumnsAndValuesFromInsert() parses columns and values from the Insert query
//
// Parameters:
//   - input: Insert Column Spec Context from antlr parser.
//   - tableName: Table Name
//   - schemaMapping: JSON Config which maintains column and its datatypes info.
//
// Returns: ColumnsResponse struct and error if any
func parseColumnsAndValuesFromInsert(input cql.IInsertColumnSpecContext, tableName string, schemaMapping *schemaMapping.SchemaMappingConfig, keyspace string) (*ColumnsResponse, error) {

	if input == nil {
		return nil, errors.New("parseColumnsAndValuesFromInsert: No Input paramaters found for columns")
	}

	columnListObj := input.ColumnList()
	if columnListObj == nil {
		return nil, errors.New("parseColumnsAndValuesFromInsert: error while parsing columns")
	}
	columns := columnListObj.AllColumn()
	if columns == nil {
		return nil, errors.New("parseColumnsAndValuesFromInsert: error while parsing columns")
	}

	if len(columns) <= 0 {
		return nil, errors.New("parseColumnsAndValuesFromInsert: No Columns found in the Insert Query")
	}
	var columnArr []Column
	var paramKeys []string
	var primaryColumns []string

	for _, val := range columns {
		columnName := val.GetText()
		if columnName == "" {
			return nil, errors.New("parseColumnsAndValuesFromInsert: No Columns found in the Insert Query")
		}
		columnName = strings.ReplaceAll(columnName, literalPlaceholder, "")
		columnType, err := schemaMapping.GetColumnType(keyspace, tableName, columnName)
		if err != nil {
			return nil, fmt.Errorf("undefined column name %s in table %s.%s", columnName, keyspace, tableName)

		}
		isPrimaryKey := false
		if columnType.IsPrimaryKey {
			isPrimaryKey = true
		}
		column := Column{
			Name:         columnName,
			ColumnFamily: schemaMapping.SystemColumnFamily,
			CQLType:      columnType.CQLType,
			IsPrimaryKey: isPrimaryKey,
		}
		if columnType.IsPrimaryKey {
			primaryColumns = append(primaryColumns, columnName)
		}
		columnArr = append(columnArr, column)
		paramKeys = append(paramKeys, columnName)

	}

	response := &ColumnsResponse{
		Columns:       columnArr,
		ParamKeys:     paramKeys,
		PrimayColumns: primaryColumns,
	}
	return response, nil
}

// setParamsFromValues() parses Values from the Insert Query
//
// Parameters:
//   - input: Insert Valye Spec Context from antlr parser.
//   - columns: Array of Column Names
//
// Returns: Map Interface for param name as key and its value and error if any
func setParamsFromValues(input cql.IInsertValuesSpecContext, columns []Column, schemaMapping *schemaMapping.SchemaMappingConfig, protocolV primitive.ProtocolVersion, tableName string, keySpace string, isPreparedQuery bool) (map[string]interface{}, []interface{}, map[string]interface{}, error) {
	if input != nil {
		valuesExpressionList := input.ExpressionList()
		if valuesExpressionList == nil {
			return nil, nil, nil, errors.New("setParamsFromValues: error while parsing values")
		}

		values := valuesExpressionList.AllExpression()
		if values == nil {
			return nil, nil, nil, errors.New("setParamsFromValues: error while parsing values")
		}
		var valuesArr []string
		var respValue []interface{}
		for _, val := range values {
			valueName := val.GetText()
			if valueName != "" {
				valuesArr = append(valuesArr, valueName)
			} else {
				return nil, nil, nil, errors.New("setParamsFromValues: Invalid Value paramaters")
			}
		}
		response := make(map[string]interface{})
		unencrypted := make(map[string]interface{})
		for i, col := range columns {
			value := strings.ReplaceAll(valuesArr[i], "'", "")
			colName := col.Name
			if !isPreparedQuery {
				var val interface{}
				var unenVal interface{}
				var err error
				columnType, er := schemaMapping.GetColumnType(keySpace, tableName, col.Name)
				if er != nil {
					return nil, nil, nil, er
				}
				if columnType.IsCollection {
					val = value
					unenVal = value
				} else {
					unenVal = value
					val, err = formatValues(value, col.CQLType, protocolV)
					if err != nil {
						return nil, nil, nil, err
					}
				}
				response[colName] = val
				unencrypted[colName] = unenVal
				respValue = append(respValue, val)
			}
		}

		return response, respValue, unencrypted, nil
	}
	return nil, nil, nil, errors.New("setParamsFromValues: No Value paramaters found")
}

// TranslateInsertQuerytoBigtable() parses Values from the Insert Query
//
// Parameters:
//   - queryStr: Read the query, parse its columns, values, table name, type of query and keyspaces etc.
//   - protocolV: Array of Column Names
//
// Returns: InsertQueryMapping, build the InsertQueryMapping and return it with nil value of error. In case of error
// InsertQueryMapping will return as nil and error will contains the error object

func (t *Translator) TranslateInsertQuerytoBigtable(queryStr string, protocolV primitive.ProtocolVersion, isPreparedQuery bool) (*InsertQueryMapping, error) {
	query := renameLiterals(queryStr)
	lexer := cql.NewCqlLexer(antlr.NewInputStream(query))
	stream := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)
	p := cql.NewCqlParser(stream)

	insertObj := p.Insert()
	if insertObj == nil {
		return nil, errors.New("could not parse insert object")
	}
	kwInsertObj := insertObj.KwInsert()
	if kwInsertObj == nil {
		return nil, errors.New("could not parse insert object")
	}
	insertObj.KwInto()
	keyspace := insertObj.Keyspace()
	table := insertObj.Table()

	var columnsResponse ColumnsResponse

	if keyspace == nil {
		return nil, errors.New("invalid input paramaters found for keyspace")
	}
	keyspaceObj := keyspace.OBJECT_NAME()
	if keyspaceObj == nil {
		return nil, errors.New("invalid input paramaters found for keyspace")
	}

	if table == nil {
		return nil, errors.New("invalid input paramaters found for table")
	}
	tableobj := table.OBJECT_NAME()
	if tableobj == nil {
		return nil, errors.New("invalid input paramaters found for table")
	}
	tableName := tableobj.GetText()
	keyspaceName := keyspaceObj.GetText()

	if keyspaceName == "" {
		return nil, errors.New("keyspace is null")
	}
	if !t.SchemaMappingConfig.InstanceExists(keyspaceName) {
		return nil, fmt.Errorf("keyspace %s does not exist", keyspaceName)
	}
	if !t.SchemaMappingConfig.TableExist(keyspaceName, tableName) {
		return nil, fmt.Errorf("table %s does not exist", tableName)
	}

	ifNotExistObj := insertObj.IfNotExist()
	var ifNotExists bool = false
	if ifNotExistObj != nil {
		val := strings.ToLower(ifNotExistObj.GetText())
		if val == "ifnotexists" {
			ifNotExists = true
		}
	}
	resp, err := parseColumnsAndValuesFromInsert(insertObj.InsertColumnSpec(), tableName, t.SchemaMappingConfig, keyspaceName)
	if err != nil {
		return nil, err
	}

	if resp != nil {
		columnsResponse = *resp
	}
	params, values, unencrypted, err := setParamsFromValues(insertObj.InsertValuesSpec(), columnsResponse.Columns, t.SchemaMappingConfig, protocolV, tableName, keyspaceName, isPreparedQuery)
	if err != nil {
		return nil, err
	}

	timestampInfo, err := GetTimestampInfo(queryStr, insertObj, int32(len(columnsResponse.Columns)))
	if err != nil {
		return nil, err
	}
	var delColumnFamily []string
	var rowKey string
	var newValues []interface{} = values
	var newColumns []Column = columnsResponse.Columns
	var rawOutput *ProcessRawCollectionsOutput // Declare rawOutput

	primaryKeys, err := getPrimaryKeys(t.SchemaMappingConfig, tableName, keyspaceName)
	if err != nil {
		return nil, err
	}

	if !ValidateRequiredPrimaryKeys(primaryKeys, columnsResponse.PrimayColumns) {
		missingKey := findFirstMissingKey(primaryKeys, columnsResponse.PrimayColumns)
		missingPkColumnType, err := t.SchemaMappingConfig.GetPkKeyType(tableName, keyspaceName, missingKey)
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("some %s key parts are missing: %s", missingPkColumnType, missingKey)
	}

	if !isPreparedQuery {
		pmks, err := t.SchemaMappingConfig.GetPkByTableNameWithFilter(tableName, keyspaceName, primaryKeys)
		if err != nil {
			return nil, err
		}
		rowKeyBytes, err := createOrderedCodeKey(pmks, unencrypted, t.EncodeIntValuesWithBigEndian)
		if err != nil {
			return nil, err
		}
		rowKey = string(rowKeyBytes)

		// Building new colum family, qualifier and values for collection type of data.
		rawInput := ProcessRawCollectionsInput{
			Columns:    columnsResponse.Columns,
			Values:     values,
			TableName:  tableName,
			Translator: t,
			KeySpace:   keyspaceName,
		}
		rawOutput, err = processCollectionColumnsForRawQueries(rawInput)
		if err != nil {
			return nil, fmt.Errorf("error processing raw collection columns: %w", err)
		}
		newColumns = rawOutput.NewColumns
		newValues = rawOutput.NewValues
		delColumnFamily = rawOutput.DelColumnFamily

	}

	insertQueryData := &InsertQueryMapping{
		Query:                query,
		QueryType:            INSERT,
		Table:                tableName,
		Keyspace:             keyspaceName,
		Columns:              newColumns,
		Values:               newValues,
		Params:               params,
		ParamKeys:            columnsResponse.ParamKeys,
		PrimaryKeys:          resp.PrimayColumns,
		RowKey:               rowKey,
		DeleteColumnFamilies: delColumnFamily,
		TimestampInfo:        timestampInfo,
		IfNotExists:          ifNotExists,
	}
	return insertQueryData, nil
}

// BuildInsertPrepareQuery() reads the insert query from cache, forms the columns and its value for collection type of data.
//
// Parameters:
//   - columnsResponse: List of all the columns
//   - values: Array of values for all columns
//   - tableName: tablename on which operation has to be performed
//   - protocolV: cassandra protocol version
//
// Returns: InsertQueryMapping, build the InsertQueryMapping and return it with nil value of error. In case of error
// InsertQueryMapping will return as nil and error will contains the error object
func (t *Translator) BuildInsertPrepareQuery(columnsResponse []Column, values []*primitive.Value, st *InsertQueryMapping, protocolV primitive.ProtocolVersion) (*InsertQueryMapping, error) {
	var newColumns []Column
	var newValues []interface{}
	var primaryKeys []string = st.PrimaryKeys
	var delColumnFamily []string
	var err error
	var unencrypted map[string]interface{}
	var prepareOutput *ProcessPrepareCollectionsOutput // Declare prepareOutput

	if len(primaryKeys) == 0 {
		primaryKeys, _ = getPrimaryKeys(t.SchemaMappingConfig, st.Table, st.Keyspace)
	}
	timestampInfo, err := ProcessTimestamp(st, values)
	if err != nil {
		return nil, err
	}
	// Building new colum family, qualifier and values for collection type of data.
	prepareInput := ProcessPrepareCollectionsInput{
		ColumnsResponse: columnsResponse,
		Values:          values,
		TableName:       st.Table,
		ProtocolV:       protocolV,
		PrimaryKeys:     primaryKeys,
		Translator:      t,
		KeySpace:        st.Keyspace,
		ComplexMeta:     nil, // Assuming nil for insert
	}
	prepareOutput, err = processCollectionColumnsForPrepareQueries(prepareInput)
	if err != nil {
		fmt.Println("Error processing prepared collection columns:", err)
		return nil, err
	}
	newColumns = prepareOutput.NewColumns
	newValues = prepareOutput.NewValues
	unencrypted = prepareOutput.Unencrypted
	delColumnFamily = prepareOutput.DelColumnFamily

	pmks, err := t.SchemaMappingConfig.GetPkByTableNameWithFilter(st.Table, st.Keyspace, primaryKeys)
	if err != nil {
		return nil, err
	}

	rowKeyBytes, err := createOrderedCodeKey(pmks, unencrypted, t.EncodeIntValuesWithBigEndian)
	if err != nil {
		return nil, err
	}
	rowKey := string(rowKeyBytes)

	insertQueryData := &InsertQueryMapping{
		Columns:              newColumns,
		Values:               newValues,
		PrimaryKeys:          primaryKeys,
		RowKey:               rowKey,
		DeleteColumnFamilies: delColumnFamily,
		TimestampInfo:        timestampInfo,
		Query:                st.Query,
		QueryType:            st.QueryType,
		Table:                st.Table,
		Keyspace:             st.Keyspace,
		Params:               st.Params,
		ParamKeys:            st.PrimaryKeys,
		IfNotExists:          st.IfNotExists,
	}

	return insertQueryData, nil
}
