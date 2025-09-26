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
	"github.com/datastax/go-cassandra-native-protocol/primitive"
)

type Column struct {
	Name         string
	ColumnFamily string
	TypeInfo     *CqlTypeInfo
	// todo remove this field because it's redundant - you can use PkPrecedence or KeyType to infer this
	IsPrimaryKey bool
	PkPrecedence int
	KeyType      string
	Metadata     message.ColumnMetadata
}

type CqlTypeInfo struct {
	// used to track the exact type given by the user
	RawType string
	// describes the datatype, which is a subset of all possible types (i.e. this type only supports varchar and not text)
	DataType datatype.DataType
	// is the datatype frozen
	IsFrozen bool
}

func NewCqlTypeInfoFromType(dt datatype.DataType) *CqlTypeInfo {
	return NewCqlTypeInfo(dt.String(), dt, false)
}
func NewCqlTypeInfo(rawType string, dataType datatype.DataType, isFrozen bool) *CqlTypeInfo {
	return &CqlTypeInfo{RawType: rawType, DataType: dataType, IsFrozen: isFrozen}
}

func (t *CqlTypeInfo) GetDataTypeCode() primitive.DataTypeCode {
	return t.DataType.GetDataTypeCode()
}
func (t *CqlTypeInfo) IsCounter() bool {
	return t.DataType == datatype.Counter
}
func (t *CqlTypeInfo) IsCollection() bool {
	switch t.DataType.GetDataTypeCode() {
	case primitive.DataTypeCodeList, primitive.DataTypeCodeSet, primitive.DataTypeCodeMap:
		return true
	default:
		return false
	}
}

type CreateColumn struct {
	Name     string
	Index    int32
	TypeInfo *CqlTypeInfo
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
