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

	"cloud.google.com/go/bigtable"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
)

// CreateBigtableClient initializes and returns a Bigtable client for a specified instance.
// It sets up a gRPC connection pool with custom options and parameters.
//
// Parameters:
//   - ctx: The context to use for managing the lifecycle of the client and request cancellations.
//   - config: A ConnConfig struct containing configuration parameters for the connection, such as GCP project ID,
//     number of channels, app profile ID, and metrics provider.
//   - instanceID: A string identifying the Bigtable instance to connect to.
//
// Returns:
//   - A pointer to an initialized bigtable.Client for the specified instance.
//   - An error if the client setup process encounters any issues.
func CreateBigtableClient(ctx context.Context, config ConnConfig, instanceConfig InstanceConfig) (*bigtable.Client, error) {
	instanceID := instanceConfig.BigtableInstance
	// Create a gRPC connection pool with the specified number of channels
	pool := grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(1024 * 1024 * 10)) // 10 MB max message size

	// Specify gRPC connection options, including your custom number of channels
	opts := []option.ClientOption{
		option.WithGRPCDialOption(pool),
		option.WithGRPCConnectionPool(config.NumOfChannels),
		option.WithUserAgent(config.UserAgent),
	}

	client, err := bigtable.NewClientWithConfig(ctx, config.GCPProjectID, instanceID, bigtable.ClientConfig{
		AppProfile: instanceConfig.AppProfileId,
	}, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Bigtable client for instance %s: %v", instanceID, err)
	}
	return client, nil
}

func CreateBigtableAdminClient(ctx context.Context, config ConnConfig, instanceID string) (*bigtable.AdminClient, error) {
	client, err := bigtable.NewAdminClient(ctx, config.GCPProjectID, instanceID, option.WithUserAgent(config.UserAgent))
	if err != nil {
		return nil, fmt.Errorf("failed to create Bigtable admin client for instance %s: %v", instanceID, err)
	}
	return client, nil
}

// CreateClientsForInstances creates and configures Bigtable clients for multiple instances.
// It initializes a map with Bigtable clients keyed by their instance IDs.
//
// Parameters:
//   - ctx: A context for managing the lifecycle of all Bigtable clients and request cancellations.
//   - config: A ConnConfig struct that holds configuration details such as instance IDs, and other client settings.
//
// Returns:
//   - A map[string]*bigtable.Client, where each key is an instance ID and the value is the corresponding Bigtable client.
//   - An error if the client creation fails for any of the specified instances.
func CreateClientsForInstances(ctx context.Context, config ConnConfig) (map[string]*bigtable.Client, map[string]*bigtable.AdminClient, error) {
	clients := make(map[string]*bigtable.Client)
	adminClients := make(map[string]*bigtable.AdminClient)
	for _, instanceConfig := range config.InstancesMap {
		instanceID := strings.TrimSpace(instanceConfig.BigtableInstance)
		client, err := CreateBigtableClient(ctx, config, instanceConfig)
		if err != nil {
			return nil, nil, err
		}
		clients[instanceID] = client

		adminClient, err := CreateBigtableAdminClient(ctx, config, instanceID)
		if err != nil {
			return nil, nil, err
		}
		adminClients[instanceID] = adminClient
	}
	return clients, adminClients, nil
}
