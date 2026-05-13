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
	"cloud.google.com/go/bigtable"
	"encoding/base64"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
)

const (
	DefaultProfileId = "default"
)

// Events
const (
	applyingBigtableMutation = "Applying Insert/Update Mutation"
	bigtableMutationApplied  = "Insert/Update Mutation Applied"
	applyingDeleteMutation   = "Applying Delete Mutation"
	deleteMutationApplied    = "Delete Mutation Applied"
	applyingBulkMutation     = "Applying Bulk Mutation"
	bulkMutationApplied      = "Bulk Mutation Applied"
	// Cassandra doesn't have a time dimension to their counters, so we need to
	// use the same time for all counters
	counterTimestamp = 0
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
func GenerateAppliedRowsResult(keyspace types.Keyspace, table types.TableName, applied bool) *message.RowsResult {
	row := message.Column{0x00}
	if applied {
		row = message.Column{0x01}
	}
	return &message.RowsResult{
		Metadata: &message.RowsMetadata{
			ColumnCount: 1,
			Columns: []*message.ColumnMetadata{
				{
					Keyspace: string(keyspace),
					Table:    string(table),
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

func BuildBigtableParams(b *types.ExecutableSelectQuery) (map[string]any, error) {
	var err error
	result := make(map[string]any)
	for _, condition := range b.Conditions {
		err = addBindValueIfNeeded(condition.Value, b.Values, needsByteConversion(condition), result)
		if err != nil {
			return nil, err
		}
		err = addBindValueIfNeeded(condition.Value2, b.Values, false, result)
		if err != nil {
			return nil, err
		}
	}

	err = addBindValueIfNeeded(b.Limit, b.Values, false, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func addBindValueIfNeeded(dynamicValue types.DynamicValue, values *types.QueryParameterValues, needsByteConversion bool, result map[string]any) error {
	if dynamicValue == nil {
		return nil
	}
	param, ok := dynamicValue.(*types.ParameterizedValue)
	if !ok {
		return nil
	}

	var value any
	if needsByteConversion {
		var err error
		value, err = param.GetValue(values)
		if err != nil {
			return err
		}
		// special case where bigtable expects bytes
		switch v := value.(type) {
		case string:
			value = []byte(v)
		}
	} else {
		var err error
		value, err = param.GetValue(values)
		if err != nil {
			return err
		}
		// ensure UUIDs are converted to []byte for Bigtable SQL
		switch v := value.(type) {
		case primitive.UUID:
			value = v[:]
		case [16]byte:
			value = v[:]
		}
	}
	result[string(param.Parameter)] = value
	return nil
}

func needsByteConversion(condition types.Condition) bool {
	if condition.Operator == types.CONTAINS {
		return true
	}

	// we're checking the column qualifier, so we need to convert to bytes
	if condition.Operator == types.CONTAINS_KEY {
		return true
	}
	return false
}

func BuildParamTypes(b *types.PreparedSelectQuery) (map[string]bigtable.SQLType, error) {
	var err error
	result := make(map[string]bigtable.SQLType)
	for _, condition := range b.Conditions {
		err = addParamIfNeeded(condition.Value, b.Params, needsByteConversion(condition), result)
		if err != nil {
			return nil, err
		}
		err = addParamIfNeeded(condition.Value2, b.Params, needsByteConversion(condition), result)
		if err != nil {
			return nil, err
		}
	}

	err = addParamIfNeeded(b.LimitValue, b.Params, false, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func addParamIfNeeded(value types.DynamicValue, params *types.QueryParameters, needsByteConversion bool, result map[string]bigtable.SQLType) error {
	if value == nil {
		return nil
	}

	param, ok := value.(*types.ParameterizedValue)
	if !ok {
		return nil
	}

	md, err := params.GetMetadata(param.Parameter)
	if err != nil {
		return err
	}

	bigtableType := md.Type.BigtableSqlType()
	if needsByteConversion {
		bigtableType = bigtable.BytesSQLType{}
	}
	result[string(param.Parameter)] = bigtableType
	return nil
}
