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
	"github.com/datastax/go-cassandra-native-protocol/message"
)

type Column struct {
	Name         string
	ColumnFamily string
	CQLType      CqlDataType
	// todo remove this field because it's redundant - you can use PkPrecedence or KeyType to infer this
	IsPrimaryKey bool
	PkPrecedence int
	KeyType      string
	Metadata     message.ColumnMetadata
}

type CreateColumn struct {
	Name     string
	Index    int32
	TypeInfo CqlDataType
}

type Clause struct {
	Column       string
	Operator     string
	Value        string
	IsPrimaryKey bool
}

// SelectedColumn describes a column that was selected as part of a query. It's
// an output of query translating, and is also used for response construction.
type SelectedColumn struct {
	// Name is the original value of the selected column, including functions. It
	// does not include the alias. e.g. "region" or "count(*)"
	Name   string
	IsFunc bool
	// IsAs is true if an alias is used
	IsAs      bool
	FuncName  string
	Alias     string
	MapKey    string
	ListIndex string
	// ColumnName is the name of the underlying column in a function, or map key
	// access. e.g. the column name of "max(price)" is "price"
	ColumnName        string
	KeyType           string
	IsWriteTimeColumn bool
}

type IntRowKeyEncodingType int

const (
	// BigEndianEncoding should not be used for new tables - this is only included for backwards compatability
	BigEndianEncoding IntRowKeyEncodingType = iota
	OrderedCodeEncoding
)
