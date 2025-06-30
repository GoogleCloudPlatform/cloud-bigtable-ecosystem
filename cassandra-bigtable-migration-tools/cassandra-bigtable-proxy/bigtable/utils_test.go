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
	"reflect"
	"testing"

	types "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	"github.com/stretchr/testify/assert"
)

func TestGetProfileId(t *testing.T) {
	tests := []struct {
		name       string
		profileId  string
		expectedId string
	}{
		{
			name:       "Non-empty profileId",
			profileId:  "user-profile-id",
			expectedId: "user-profile-id",
		},
		{
			name:       "Empty profileId",
			profileId:  "",
			expectedId: DefaultProfileId,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetProfileId(tt.profileId)
			assert.Equal(t, tt.expectedId, result)
		})
	}
}

func Test_sortPkData(t *testing.T) {
	type args struct {
		pkMetadata map[string][]types.Column
	}
	tests := []struct {
		name string
		args args
		want map[string][]types.Column
	}{
		{
			name: "Basic Sorting",
			args: args{
				pkMetadata: map[string][]types.Column{
					"users": {
						{ColumnName: "id", PkPrecedence: 2},
						{ColumnName: "email", PkPrecedence: 1},
					},
				},
			},
			want: map[string][]types.Column{
				"users": {
					{ColumnName: "email", PkPrecedence: 1},
					{ColumnName: "id", PkPrecedence: 2},
				},
			},
		},
		{
			name: "Already Sorted Data",
			args: args{
				pkMetadata: map[string][]types.Column{
					"orders": {
						{ColumnName: "order_id", PkPrecedence: 1},
						{ColumnName: "customer_id", PkPrecedence: 2},
					},
				},
			},
			want: map[string][]types.Column{
				"orders": {
					{ColumnName: "order_id", PkPrecedence: 1},
					{ColumnName: "customer_id", PkPrecedence: 2},
				},
			},
		},
		{
			name: "Multiple Tables",
			args: args{
				pkMetadata: map[string][]types.Column{
					"users": {
						{ColumnName: "id", PkPrecedence: 2},
						{ColumnName: "email", PkPrecedence: 1},
					},
					"orders": {
						{ColumnName: "order_id", PkPrecedence: 1},
						{ColumnName: "customer_id", PkPrecedence: 3},
						{ColumnName: "date", PkPrecedence: 2},
					},
				},
			},
			want: map[string][]types.Column{
				"users": {
					{ColumnName: "email", PkPrecedence: 1},
					{ColumnName: "id", PkPrecedence: 2},
				},
				"orders": {
					{ColumnName: "order_id", PkPrecedence: 1},
					{ColumnName: "date", PkPrecedence: 2},
					{ColumnName: "customer_id", PkPrecedence: 3},
				},
			},
		},
		{
			name: "Same Precedence Values (Unchanged Order)",
			args: args{
				pkMetadata: map[string][]types.Column{
					"products": {
						{ColumnName: "product_id", PkPrecedence: 1},
						{ColumnName: "category_id", PkPrecedence: 1},
					},
				},
			},
			want: map[string][]types.Column{
				"products": {
					{ColumnName: "product_id", PkPrecedence: 1},
					{ColumnName: "category_id", PkPrecedence: 1}, // Order should remain the same
				},
			},
		},
		{
			name: "Single types.Column (No Sorting Needed)",
			args: args{
				pkMetadata: map[string][]types.Column{
					"categories": {
						{ColumnName: "category_id", PkPrecedence: 1},
					},
				},
			},
			want: map[string][]types.Column{
				"categories": {
					{ColumnName: "category_id", PkPrecedence: 1},
				},
			},
		},
		{
			name: "Empty Map (No Operation)",
			args: args{
				pkMetadata: map[string][]types.Column{},
			},
			want: map[string][]types.Column{},
		},
		{
			name: "Empty Table Entries (No Sorting Needed)",
			args: args{
				pkMetadata: map[string][]types.Column{
					"empty_table": {},
				},
			},
			want: map[string][]types.Column{
				"empty_table": {},
			},
		},
		{
			name: "Table With Nil Columns (Should Not Panic)",
			args: args{
				pkMetadata: map[string][]types.Column{
					"table_nil": nil,
				},
			},
			want: map[string][]types.Column{
				"table_nil": nil,
			},
		},
		{
			name: "Negative Precedence Values (Still Sorted Correctly)",
			args: args{
				pkMetadata: map[string][]types.Column{
					"test_table": {
						{ColumnName: "col1", PkPrecedence: -1},
						{ColumnName: "col2", PkPrecedence: -3},
						{ColumnName: "col3", PkPrecedence: -2},
					},
				},
			},
			want: map[string][]types.Column{
				"test_table": {
					{ColumnName: "col2", PkPrecedence: -3},
					{ColumnName: "col3", PkPrecedence: -2},
					{ColumnName: "col1", PkPrecedence: -1},
				},
			},
		},
		{
			name: "Zero Precedence Values (Sorted Normally)",
			args: args{
				pkMetadata: map[string][]types.Column{
					"zero_precedence": {
						{ColumnName: "colA", PkPrecedence: 0},
						{ColumnName: "colB", PkPrecedence: 2},
						{ColumnName: "colC", PkPrecedence: 1},
					},
				},
			},
			want: map[string][]types.Column{
				"zero_precedence": {
					{ColumnName: "colA", PkPrecedence: 0},
					{ColumnName: "colC", PkPrecedence: 1},
					{ColumnName: "colB", PkPrecedence: 2},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sortPkData(tt.args.pkMetadata)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("sortPkData() = %v, want %v", got, tt.want)
			}
		})
	}
}
