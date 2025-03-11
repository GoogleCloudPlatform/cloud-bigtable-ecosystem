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
package proxycore

import (
	"net"
	"testing"

	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/stretchr/testify/assert"
)

func TestResultSet(t *testing.T) {
	// Example column metadata
	columns := []*message.ColumnMetadata{
		{Name: "column1", Type: datatype.Varchar},
		{Name: "column2", Type: datatype.Inet},
		{Name: "column3", Type: datatype.Uuid},
	}

	// Create a [][]byte to hold the row
	var row [][]byte
	uuid := primitive.UUID{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	for _, col := range columns {
		// Convert each value to []byte based on its type (example conversion)
		var valueBytes []byte
		switch col.Type {
		case datatype.Varchar:
			valueBytes = []byte("value1")
		case datatype.Inet:
			valueBytes = []byte(net.IPv4(127, 0, 0, 1))
		case datatype.Uuid:
			valueBytes = []byte(uuid.Bytes())
		default:
			valueBytes = nil // Handle unsupported types or add more cases as needed
		}

		// Append the valueBytes to the row
		row = append(row, valueBytes)
	}

	// Example RowsResult
	rowsResult := &message.RowsResult{
		Metadata: &message.RowsMetadata{Columns: columns},
		Data:     message.RowSet{row},
	}

	// Example ProtocolVersion
	version := primitive.ProtocolVersion4

	// Create ResultSet
	rs := NewResultSet(rowsResult, version)

	t.Run("RowCount", func(t *testing.T) {
		assert.Equal(t, 1, rs.RowCount())
	})

	t.Run("ByPos", func(t *testing.T) {
		row := rs.Row(0)
		val, err := row.ByPos(0)
		assert.NoError(t, err)
		assert.Equal(t, "value1", val)
	})

	t.Run("ByName", func(t *testing.T) {
		row := rs.Row(0)
		val, err := row.ByName("column1")
		assert.NoError(t, err)
		assert.Equal(t, "value1", val)

		_, err = row.ByName("nonexistent")
		assert.Error(t, err)
		assert.Equal(t, ColumnNameNotFound, err)
	})

	t.Run("StringByName", func(t *testing.T) {
		row := rs.Row(0)
		val, err := row.StringByName("column1")
		assert.NoError(t, err)
		assert.Equal(t, "value1", val)

		_, err = row.StringByName("column2")
		assert.Error(t, err)
	})

	t.Run("InetByName", func(t *testing.T) {
		row := rs.Row(0)
		val, err := row.InetByName("column2")
		assert.NoError(t, err)
		assert.Equal(t, net.IPv4(127, 0, 0, 1), val)

		_, err = row.InetByName("column1")
		assert.Error(t, err)
	})

	t.Run("UUIDByName", func(t *testing.T) {
		row := rs.Row(0)
		val, err := row.UUIDByName("column3")
		assert.NoError(t, err)
		assert.Equal(t, primitive.UUID{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}, val)

		_, err = row.UUIDByName("column1")
		assert.Error(t, err)
	})
}
