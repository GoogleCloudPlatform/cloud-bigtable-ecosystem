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
	"bytes"
	"testing"
)

func TestEvaluateChallenge(t *testing.T) {
	// Create a passwordAuth instance for testing
	auth := &passwordAuth{
		authId:   "authId",
		username: "username",
		password: "password",
	}

	tests := []struct {
		name          string
		token         []byte
		expectedToken []byte
		expectError   bool
	}{
		{
			name:          "Correct token",
			token:         []byte("PLAIN-START"),
			expectedToken: []byte("authId\x00username\x00password"),
			expectError:   false,
		},
		{
			name:        "Nil token",
			token:       nil,
			expectError: true,
		},
		{
			name:        "Incorrect token",
			token:       []byte("WRONG-START"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := auth.EvaluateChallenge(tt.token)
			if (err != nil) != tt.expectError {
				t.Fatalf("expected error: %v, got: %v", tt.expectError, err)
			}
			if !tt.expectError && !bytes.Equal(token, tt.expectedToken) {
				t.Fatalf("expected token: %v, got: %v", tt.expectedToken, token)
			}
		})
	}
}
