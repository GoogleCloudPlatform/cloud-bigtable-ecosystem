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
	"testing"

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
