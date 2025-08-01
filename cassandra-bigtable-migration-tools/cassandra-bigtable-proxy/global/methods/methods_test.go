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
package methods

import (
	"reflect"
	"testing"

	"github.com/datastax/go-cassandra-native-protocol/datatype"
)

func TestGetCassandraColumnType(t *testing.T) {
	testCases := []struct {
		input    string
		wantType datatype.DataType
		wantErr  bool
	}{
		{"text", datatype.Varchar, false},
		{"blob", datatype.Blob, false},
		{"timestamp", datatype.Timestamp, false},
		{"int", datatype.Int, false},
		{"float", datatype.Float, false},
		{"double", datatype.Double, false},
		{"bigint", datatype.Bigint, false},
		{"boolean", datatype.Boolean, false},
		{"uuid", datatype.Uuid, false},
		{"map<text, boolean>", datatype.NewMapType(datatype.Varchar, datatype.Boolean), false},
		{"map<varchar, boolean>", datatype.NewMapType(datatype.Varchar, datatype.Boolean), false},
		{"map<text, text>", datatype.NewMapType(datatype.Varchar, datatype.Varchar), false},
		{"map<text, varchar>", datatype.NewMapType(datatype.Varchar, datatype.Varchar), false},
		{"map<varchar, text>", datatype.NewMapType(datatype.Varchar, datatype.Varchar), false},
		{"map<varchar, varchar>", datatype.NewMapType(datatype.Varchar, datatype.Varchar), false},
		{"map<varchar>", nil, true},
		{"list<text>", datatype.NewListType(datatype.Varchar), false},
		{"list<varchar>", datatype.NewListType(datatype.Varchar), false},
		{"frozen<list<text>>", datatype.NewListType(datatype.Varchar), false},
		{"frozen<list<varchar>>", datatype.NewListType(datatype.Varchar), false},
		{"set<text>", datatype.NewSetType(datatype.Varchar), false},
		{"set<text", nil, true},
		{"set<", nil, true},
		{"set<varchar>", datatype.NewSetType(datatype.Varchar), false},
		{"frozen<set<text>>", datatype.NewSetType(datatype.Varchar), false},
		{"frozen<set<varchar>>", datatype.NewSetType(datatype.Varchar), false},
		{"unknown", nil, true},
		// Future scope items below:
		{"map<text, int>", datatype.NewMapType(datatype.Varchar, datatype.Int), false},
		{"map<varchar, int>", datatype.NewMapType(datatype.Varchar, datatype.Int), false},
		{"map<text, bigint>", datatype.NewMapType(datatype.Varchar, datatype.Bigint), false},
		{"map<varchar, bigint>", datatype.NewMapType(datatype.Varchar, datatype.Bigint), false},
		{"map<text, float>", datatype.NewMapType(datatype.Varchar, datatype.Float), false},
		{"map<varchar, float>", datatype.NewMapType(datatype.Varchar, datatype.Float), false},
		{"map<text, double>", datatype.NewMapType(datatype.Varchar, datatype.Double), false},
		{"map<varchar, double>", datatype.NewMapType(datatype.Varchar, datatype.Double), false},
		{"map<text, timestamp>", datatype.NewMapType(datatype.Varchar, datatype.Timestamp), false},
		{"map<varchar, timestamp>", datatype.NewMapType(datatype.Varchar, datatype.Timestamp), false},
		{"map<timestamp, text>", datatype.NewMapType(datatype.Timestamp, datatype.Varchar), false},
		{"map<timestamp, varchar>", datatype.NewMapType(datatype.Timestamp, datatype.Varchar), false},
		{"map<timestamp, boolean>", datatype.NewMapType(datatype.Timestamp, datatype.Boolean), false},
		{"map<timestamp, int>", datatype.NewMapType(datatype.Timestamp, datatype.Int), false},
		{"map<timestamp, bigint>", datatype.NewMapType(datatype.Timestamp, datatype.Bigint), false},
		{"map<timestamp, float>", datatype.NewMapType(datatype.Timestamp, datatype.Float), false},
		{"map<timestamp, double>", datatype.NewMapType(datatype.Timestamp, datatype.Double), false},
		{"map<timestamp, timestamp>", datatype.NewMapType(datatype.Timestamp, datatype.Timestamp), false},
		{"set<int>", datatype.NewSetType(datatype.Int), false},
		{"set<bigint>", datatype.NewSetType(datatype.Bigint), false},
		{"set<float>", datatype.NewSetType(datatype.Float), false},
		{"set<double>", datatype.NewSetType(datatype.Double), false},
		{"set<boolean>", datatype.NewSetType(datatype.Boolean), false},
		{"set<timestamp>", datatype.NewSetType(datatype.Timestamp), false},
		{"set<text>", datatype.NewSetType(datatype.Varchar), false},
		{"set<varchar>", datatype.NewSetType(datatype.Varchar), false},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			gotType, err := GetCassandraColumnType(tc.input)
			if (err != nil) != tc.wantErr {
				t.Errorf("getCassandraColumnType(%s) error = %v, wantErr %v", tc.input, err, tc.wantErr)
				return
			}

			if err == nil && !reflect.DeepEqual(gotType, tc.wantType) {
				t.Errorf("getCassandraColumnType(%s) = %v, want %v", tc.input, gotType, tc.wantType)
			}
		})
	}
}
