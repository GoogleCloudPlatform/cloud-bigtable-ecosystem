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
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateClientsForInstances(t *testing.T) {
	ctx := context.Background()

	config := ConnConfig{
		GCPProjectID:  "test-project",
		InstancesMap:  map[string]InstanceConfig{"test-instance1": {BigtableInstance: "test-instance1"}, "test-instance2": {BigtableInstance: "test-instance2"}},
		NumOfChannels: 1,
		AppProfileID:  "test-app",
		UserAgent:     "cassandra-proxy",
	}

	clients, _, err := CreateClientsForInstances(ctx, config)
	assert.NoError(t, err, "CreateClientsForInstances should not return an error")

	for _, instanceConfig := range config.InstancesMap {
		instanceID := strings.TrimSpace(instanceConfig.BigtableInstance)
		client, ok := clients[instanceID]
		assert.True(t, ok, fmt.Sprintf("Client for instance %s should be created", instanceID))
		assert.NotNil(t, client, fmt.Sprintf("Client for instance %s should not be nil", instanceID))
	}

	// Close clients after test
	for _, client := range clients {
		client.Close()
	}
}
