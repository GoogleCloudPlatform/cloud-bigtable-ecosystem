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

package schema_setup

// TableSchema represents the schema for multiple Cassandra tables
type TableSchema struct {
	Tables []Table `json:"tables"`
}

// Table represents the schema definition for a single table
type Table struct {
	TableName      string    `json:"table_name"`
	ColumnFamilies []string  `json:"column_families"`
	Columns        []Columns `json:"columns"`
}

// Columns represents the definition of a single column in the table schema
type Columns struct {
	ColumnName   string `json:"ColumnName"`
	ColumnType   string `json:"ColumnType"`
	IsCollection string `json:"IsCollection"`
	IsPrimaryKey string `json:"IsPrimaryKey"`
	PKPrecedence string `json:"PK_Precedence"`
	KeyType      string `json:"KeyType"`
	TableName    string `json:"TableName"`
}
