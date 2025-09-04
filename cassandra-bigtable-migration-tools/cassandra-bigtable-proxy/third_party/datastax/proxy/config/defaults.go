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
package config

import (
	"errors"
	"fmt"

	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
)

var (
	// todo ensure this is a reasonable default
	DefaultBigtableGrpcChannels   = 1
	BigtableMinSession            = 100
	BigtableMaxSession            = 400
	DefaultSchemaMappingTableName = "schema_mapping"
	ErrorAuditTable               = "error_audit"
	DefaultColumnFamily           = "cf1"
	DefaultAppProfileId           = "default"
	TimestampColumnName           = "ts_column"
)

func validateCliArgs(args *types.CliArgs) error {
	if args.NumConns < 1 {
		return fmt.Errorf("invalid number of connections, must be greater than 0 (provided: %d)", args.NumConns)
	}

	if args.ProtocolVersion > args.MaxProtocolVersion {
		return fmt.Errorf("default protocol version is greater than max protocol version")
	}

	return nil
}

// ApplyDefaults applies default values to the configuration after it is loaded
func validateAndApplyDefaults(cfg *yamlProxyConfig) error {
	if cfg.Otel == nil {
		cfg.Otel = &yamlOtelConfig{
			Enabled: false,
		}
	} else {
		if cfg.Otel.Enabled {
			if cfg.Otel.Traces.SamplingRatio < 0 || cfg.Otel.Traces.SamplingRatio > 1 {
				return errors.New("Sampling Ratio for Otel Traces should be between 0 and 1]")
			}
		}
	}

	for i := range cfg.Listeners {
		if cfg.Listeners[i].Bigtable.Session.GrpcChannels == 0 {
			cfg.Listeners[i].Bigtable.Session.GrpcChannels = DefaultBigtableGrpcChannels
		}
		if cfg.Listeners[i].Bigtable.DefaultColumnFamily == "" {
			cfg.Listeners[i].Bigtable.DefaultColumnFamily = DefaultColumnFamily
		}

		if cfg.Listeners[i].Bigtable.SchemaMappingTable == "" {
			if cfg.CassandraToBigtableConfigs.SchemaMappingTable == "" {
				cfg.Listeners[i].Bigtable.SchemaMappingTable = DefaultSchemaMappingTableName
			} else {
				cfg.Listeners[i].Bigtable.SchemaMappingTable = cfg.CassandraToBigtableConfigs.SchemaMappingTable
			}
		}

		if cfg.Listeners[i].Bigtable.ProjectID == "" {
			if cfg.CassandraToBigtableConfigs.ProjectID == "" {
				return fmt.Errorf("project id is not defined for listener %s %d", cfg.Listeners[i].Name, cfg.Listeners[i].Port)
			}
			cfg.Listeners[i].Bigtable.ProjectID = cfg.CassandraToBigtableConfigs.ProjectID
		}
		if len(cfg.Listeners[i].Bigtable.Instances) == 0 && cfg.Listeners[i].Bigtable.InstanceIDs == "" {
			return fmt.Errorf("either 'instances' or 'instance_ids' must be defined for listener %s on port %d", cfg.Listeners[i].Name, cfg.Listeners[i].Port)
		}
		if len(cfg.Listeners[i].Bigtable.Instances) != 0 && cfg.Listeners[i].Bigtable.InstanceIDs != "" {
			return fmt.Errorf("only one of 'instances' or 'instance_ids' should be set for listener %s on port %d", cfg.Listeners[i].Name, cfg.Listeners[i].Port)
		}
	}
	return nil
}

func validateInstanceConfig(c *types.ProxyInstanceConfig) error {
	if c.BigtableConfig.ProjectID == "" {
		return fmt.Errorf("missing project id for listener with port %d", c.Port)
	}
	if len(c.BigtableConfig.Instances) == 0 {
		return fmt.Errorf("missing instances for listener with port %d", c.Port)
	}
	for key, mapping := range c.BigtableConfig.Instances {
		if mapping.BigtableInstance == "" {
			return fmt.Errorf("missing an instance id for listener with port %d", c.Port)
		}
		if key == "" || mapping.Keyspace == "" {
			return fmt.Errorf("missing a keyspace for listener with port %d", c.Port)
		}
	}

	return nil
}
