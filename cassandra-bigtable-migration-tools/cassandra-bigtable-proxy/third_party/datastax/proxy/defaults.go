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
package proxy

import "fmt"

var (
	BigtableGrpcChannels       = 1
	BigtableMinSession         = 100
	BigtableMaxSession         = 400
	SchemaMappingTable         = "schema_mapping"
	ErrorAuditTable            = "error_audit"
	DefaultColumnFamily        = "cf1"
	DefaultCounterColumnFamily = "ccf"
	DefaultProfileId           = "default"
	TimestampColumnName        = "ts_column"
)

// ApplyDefaults applies default values to the configuration after it is loaded
func ValidateAndApplyDefaults(cfg *UserConfig) error {
	if len(cfg.Listeners) == 0 {
		return fmt.Errorf("listener configuration is missing in `config.yaml`")
	}
	for i := range cfg.Listeners {
		if cfg.Listeners[i].Bigtable.Session.GrpcChannels == 0 {
			cfg.Listeners[i].Bigtable.Session.GrpcChannels = BigtableGrpcChannels
		}
		if cfg.Listeners[i].Bigtable.DefaultColumnFamily == "" {
			cfg.Listeners[i].Bigtable.DefaultColumnFamily = DefaultColumnFamily
		}
		if cfg.Listeners[i].Bigtable.CounterColumnFamily == "" {
			cfg.Listeners[i].Bigtable.CounterColumnFamily = DefaultCounterColumnFamily
		}
		if cfg.Listeners[i].Bigtable.AppProfileID == "" {
			cfg.Listeners[i].Bigtable.AppProfileID = DefaultProfileId
		}
		if cfg.Listeners[i].Bigtable.SchemaMappingTable == "" {
			if cfg.CassandraToBigtableConfigs.SchemaMappingTable == "" {
				cfg.Listeners[i].Bigtable.SchemaMappingTable = SchemaMappingTable
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
		if cfg.Listeners[i].Bigtable.DefaultColumnFamily == cfg.Listeners[i].Bigtable.CounterColumnFamily {
			return fmt.Errorf("default column family is the same as counter column family for listener %s but must be different", cfg.Listeners[i].Name)
		}
	}
	return nil
}
