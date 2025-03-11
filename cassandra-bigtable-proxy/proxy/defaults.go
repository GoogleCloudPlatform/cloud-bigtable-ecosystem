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
	BigTableGrpcChannels = 1
	BigTableMinSession   = 100
	BigTableMaxSession   = 400
	SchemaMappingTable   = "schema_mapping"
	ErrorAuditTable      = "error_audit"
	DefaultColumnFamily  = "cf1"
	DefaultProfileId     = "default"
	TimestampColumnName  = "ts_column"
)

// ApplyDefaults applies default values to the configuration after it is loaded
func ValidateAndApplyDefaults(cfg *UserConfig) error {
	if len(cfg.Listeners) == 0 {
		return fmt.Errorf("listener configuration is missing in `config.yaml`")
	}
	for i := range cfg.Listeners {
		if cfg.Listeners[i].BigTable.Session.GrpcChannels == 0 {
			cfg.Listeners[i].BigTable.Session.GrpcChannels = BigTableGrpcChannels
		}
		if cfg.Listeners[i].BigTable.DefaultColumnFamily == "" {
			cfg.Listeners[i].BigTable.DefaultColumnFamily = DefaultColumnFamily
		}
		if cfg.Listeners[i].BigTable.AppProfileID == "" {
			cfg.Listeners[i].BigTable.AppProfileID = DefaultProfileId
		}
		if cfg.Listeners[i].BigTable.SchemaMappingTable == "" {
			if cfg.CassandraToBigTableConfigs.SchemaMappingTable == "" {
				cfg.Listeners[i].BigTable.SchemaMappingTable = SchemaMappingTable
			} else {
				cfg.Listeners[i].BigTable.SchemaMappingTable = cfg.CassandraToBigTableConfigs.SchemaMappingTable
			}
		}

		if cfg.Listeners[i].BigTable.ProjectID == "" {
			if cfg.CassandraToBigTableConfigs.ProjectID == "" {
				return fmt.Errorf("project id is not defined for listener %s %d", cfg.Listeners[i].Name, cfg.Listeners[i].Port)
			}
			cfg.Listeners[i].BigTable.ProjectID = cfg.CassandraToBigTableConfigs.ProjectID
		}
		if cfg.Listeners[i].BigTable.InstanceIDs == "" {
			return fmt.Errorf("instance id is not defined for listener %s %d", cfg.Listeners[i].Name, cfg.Listeners[i].Port)
		}
	}
	return nil
}
