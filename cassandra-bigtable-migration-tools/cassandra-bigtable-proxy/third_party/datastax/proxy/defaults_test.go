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

import "testing"

func TestValidateAndApplyDefaults(t *testing.T) {
	type args struct {
		cfg *UserConfig
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Empty Listeners - expect error",
			args: args{
				cfg: &UserConfig{
					Listeners: []Listener{},
				},
			},
			wantErr: true,
		},
		{
			name: "Valid Config with Defaults Applied",
			args: args{
				cfg: &UserConfig{
					Listeners: []Listener{
						{
							Name: "listener1",
							Port: 8080,
							Bigtable: Bigtable{
								Session:   Session{},
								Instances: []InstancesMap{{BigtableInstance: "instance1", Keyspace: "instance1"}},
								ProjectID: "project1",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Missing ProjectID - expect error",
			args: args{
				cfg: &UserConfig{
					Listeners: []Listener{
						{
							Name: "listener1",
							Port: 8080,
							Bigtable: Bigtable{
								Session:   Session{},
								Instances: []InstancesMap{{BigtableInstance: "instance1", Keyspace: "instance1"}},
							},
						},
					},
					CassandraToBigtableConfigs: CassandraToBigtableConfigs{
						ProjectID: "",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Valid Config with Custom SchemaMappingTable",
			args: args{
				cfg: &UserConfig{
					Listeners: []Listener{
						{
							Name: "listener1",
							Port: 8080,
							Bigtable: Bigtable{
								Session:            Session{},
								Instances:          []InstancesMap{{BigtableInstance: "instance1", Keyspace: "instance1"}},
								ProjectID:          "project1",
								SchemaMappingTable: "custom_table",
							},
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateAndApplyDefaults(tt.args.cfg); (err != nil) != tt.wantErr {
				t.Errorf("ValidateAndApplyDefaults() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if tt.args.cfg.Listeners[0].Bigtable.Session.GrpcChannels != BigtableGrpcChannels {
					t.Errorf("Expected GrpcChannels to be %d, got %d", BigtableGrpcChannels, tt.args.cfg.Listeners[0].Bigtable.Session.GrpcChannels)
				}
				if tt.args.cfg.Listeners[0].Bigtable.DefaultColumnFamily != DefaultColumnFamily {
					t.Errorf("Expected ColumnFamily to be %s, got %s", DefaultColumnFamily, tt.args.cfg.Listeners[0].Bigtable.DefaultColumnFamily)
				}
				if tt.args.cfg.Listeners[0].Bigtable.SchemaMappingTable == "" {
					t.Errorf("Expected SchemaMappingTable to be set, got empty string")
				}
			}
		})
	}
}
