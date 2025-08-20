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
package types

import (
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/message"
)

type Column struct {
	Name         string
	ColumnFamily string
	CQLType      datatype.DataType
	// todo remove this field because it's redundant - you can use PkPrecedence or KeyType to infer this
	IsPrimaryKey bool
	PkPrecedence int
	KeyType      string
	Metadata     message.ColumnMetadata
}

type Clause struct {
	Column       string
	Operator     string
	Value        string
	IsPrimaryKey bool
}

type SelectedColumns struct {
	Name              string
	IsFunc            bool
	IsAs              bool
	FuncName          string
	Alias             string
	MapKey            string
	ListIndex         string
	ColumnName        string //this will be the column name for writetime function,aggregate function and Map key access
	KeyType           string
	IsWriteTimeColumn bool
}
