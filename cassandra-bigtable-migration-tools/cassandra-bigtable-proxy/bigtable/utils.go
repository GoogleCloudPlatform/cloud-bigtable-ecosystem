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
package bigtableclient

import (
	"encoding/base64"

	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/message"
)

const (
	DefaultProfileId = "default"
)

// GetProfileId returns the provided profile ID if it is not empty.
// If the provided profile ID is empty, it returns a default profile ID.
func GetProfileId(profileId string) string {
	if profileId != "" {
		return profileId
	}
	return DefaultProfileId
}

// GenerateAppliedRowsResult creates a RowsResult message to indicate whether a database operation was applied.
// It generates a single column row result with a boolean indicating the application status.
// it is specifically for if exists and if not exists queries
//
// Parameters:
//   - keyspace: A string representing the name of the keyspace in which the table resides.
//   - tableName: A string representing the name of the table where the operation was attempted.
//   - applied: A boolean flag indicating whether the operation was successfully applied.
//
// Returns: A pointer to a RowsResult object that contains metadata for a single boolean column denoting
//
//	the application status ([applied]) and the corresponding row data indicating true or false.
func GenerateAppliedRowsResult(keyspace string, tableName string, applied bool) *message.RowsResult {
	row := message.Column{0x00}
	if applied {
		row = message.Column{0x01}
	}
	return &message.RowsResult{
		Metadata: &message.RowsMetadata{
			ColumnCount: 1,
			Columns: []*message.ColumnMetadata{
				{
					Keyspace: keyspace,
					Table:    tableName,
					Name:     "[applied]",
					Type:     datatype.Boolean,
				},
			},
		},
		Data: message.RowSet{message.Row{row}},
	}
}

func decodeBase64(k string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(k)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}
