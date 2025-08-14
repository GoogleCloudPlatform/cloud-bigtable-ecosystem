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
		pkMetadata []*types.Column
	}
	tests := []struct {
		name string
		args args
		want []*types.Column
	}{
		{
			name: "Basic Sorting",
			args: args{
				pkMetadata: []*types.Column{
					{Name: "id", PkPrecedence: 2},
					{Name: "email", PkPrecedence: 1},
				},
			},
			want: []*types.Column{
				{Name: "email", PkPrecedence: 1},
				{Name: "id", PkPrecedence: 2},
			},
		},
		{
			name: "Already Sorted Data",
			args: args{
				pkMetadata: []*types.Column{
					{Name: "order_id", PkPrecedence: 1},
					{Name: "customer_id", PkPrecedence: 2},
				},
			},
			want: []*types.Column{
				{Name: "order_id", PkPrecedence: 1},
				{Name: "customer_id", PkPrecedence: 2},
			},
		},
		{
			name: "Same Precedence Values (Unchanged Order)",
			args: args{
				pkMetadata: []*types.Column{
					{Name: "product_id", PkPrecedence: 1},
					{Name: "category_id", PkPrecedence: 1},
				},
			},
			want: []*types.Column{
				{Name: "product_id", PkPrecedence: 1},
				{Name: "category_id", PkPrecedence: 1}, // Order should remain the same
			},
		},
		{
			name: "Single types.Column (No Sorting Needed)",
			args: args{
				pkMetadata: []*types.Column{
					{Name: "category_id", PkPrecedence: 1},
				},
			},
			want: []*types.Column{
				{Name: "category_id", PkPrecedence: 1},
			},
		},
		{
			name: "Empty Slice (No Operation)",
			args: args{
				pkMetadata: []*types.Column{},
			},
			want: []*types.Column{},
		},
		{
			name: "Table With Nil Columns (Should Not Panic)",
			args: args{
				pkMetadata: nil,
			},
			want: nil,
		},
		{
			name: "Negative Precedence Values (Still Sorted Correctly)",
			args: args{
				pkMetadata: []*types.Column{
					{Name: "col1", PkPrecedence: -1},
					{Name: "col2", PkPrecedence: -3},
					{Name: "col3", PkPrecedence: -2},
				},
			},
			want: []*types.Column{
				{Name: "col2", PkPrecedence: -3},
				{Name: "col3", PkPrecedence: -2},
				{Name: "col1", PkPrecedence: -1},
			},
		},
		{
			name: "Zero Precedence Values (Sorted Normally)",
			args: args{
				pkMetadata: []*types.Column{
					{Name: "colA", PkPrecedence: 0},
					{Name: "colB", PkPrecedence: 2},
					{Name: "colC", PkPrecedence: 1},
				},
			},
			want: []*types.Column{
				{Name: "colA", PkPrecedence: 0},
				{Name: "colC", PkPrecedence: 1},
				{Name: "colB", PkPrecedence: 2},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sortPrimaryKeys(tt.args.pkMetadata)
			if !reflect.DeepEqual(tt.args.pkMetadata, tt.want) {
				t.Errorf("sortPrimaryKeys() = %v, want %v", tt.args.pkMetadata, tt.want)
			}
		})
	}
}
