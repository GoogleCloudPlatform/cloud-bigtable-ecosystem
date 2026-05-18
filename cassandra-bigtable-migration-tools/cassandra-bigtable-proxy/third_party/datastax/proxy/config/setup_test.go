package config

import (
	"log"
	"os"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/constants"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCliArgs(t *testing.T) {

	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("could not get current working directory: %v", err)
	}
	const NO_ERROR_EXPECTED = ""

	tests := []struct {
		name    string
		args    []string
		want    *types.CliArgs
		wantErr string
	}{
		{
			name: "Valid config file",
			args: []string{"-f", "testdata/valid_config.yaml", "--num-conns", "19", "--protocol-version", "3"},
			want: &types.CliArgs{
				Version:                       false,
				RpcAddress:                    "",
				ProtocolVersion:               primitive.ProtocolVersion3,
				MaxProtocolVersion:            primitive.ProtocolVersion4,
				DataCenter:                    "datacenter1",
				ConfigFilePath:                wd + "/testdata/valid_config.yaml",
				NumConns:                      19,
				ReleaseVersion:                "4.0.0.6816",
				Partitioner:                   "org.apache.cassandra.dht.Murmur3Partitioner",
				Tokens:                        nil,
				CQLVersion:                    "3.4.5",
				LogLevel:                      "info",
				TcpBindPort:                   "0.0.0.0:%s",
				UseUnixSocket:                 false,
				UnixSocketPath:                "/tmp/cassandra-proxy.sock",
				ProxyCertFile:                 "",
				ProxyKeyFile:                  "",
				UserAgent:                     "cassandra-adapter/" + constants.ProxyReleaseVersion,
				ClientPid:                     0,
				ClientUid:                     0,
				QuickStartProjectId:           "",
				QuickStartInstanceId:          "",
				QuickStartAppProfile:          "default",
				QuickStartPort:                9042,
				QuickStartDefaultColumnFamily: "cf1",
				QuickStartSchemaMappingTable:  "schema_mapping",
			},
			wantErr: NO_ERROR_EXPECTED,
		},
		{
			name: "Version flag",
			args: []string{"-v"},
			want: &types.CliArgs{
				Version:                       true,
				RpcAddress:                    "",
				ProtocolVersion:               primitive.ProtocolVersion4,
				MaxProtocolVersion:            primitive.ProtocolVersion4,
				DataCenter:                    "datacenter1",
				ConfigFilePath:                "",
				NumConns:                      20,
				ReleaseVersion:                "4.0.0.6816",
				Partitioner:                   "org.apache.cassandra.dht.Murmur3Partitioner",
				Tokens:                        nil,
				CQLVersion:                    "3.4.5",
				LogLevel:                      "info",
				TcpBindPort:                   "0.0.0.0:%s",
				UseUnixSocket:                 false,
				UnixSocketPath:                "/tmp/cassandra-proxy.sock",
				ProxyCertFile:                 "",
				ProxyKeyFile:                  "",
				UserAgent:                     "cassandra-adapter/" + constants.ProxyReleaseVersion,
				ClientPid:                     0,
				ClientUid:                     0,
				QuickStartProjectId:           "",
				QuickStartInstanceId:          "",
				QuickStartAppProfile:          "default",
				QuickStartPort:                9042,
				QuickStartDefaultColumnFamily: "cf1",
				QuickStartSchemaMappingTable:  "schema_mapping",
			},
			wantErr: NO_ERROR_EXPECTED,
		},
		{
			name: "Quick start args",
			args: []string{
				"-f", "testdata/valid_config.yaml",
				"--project-id", "my-project-id",
				"--instance-id", "my-instance-id",
				"--app-profile", "my-app-profile",
				"--port", "9041",
				"--default-column-family", "my-default-column-family",
				"--schema-mapping-table", "my-schema-mapping-table",
			},
			want: &types.CliArgs{
				Version:                       false,
				RpcAddress:                    "",
				ProtocolVersion:               primitive.ProtocolVersion4,
				MaxProtocolVersion:            primitive.ProtocolVersion4,
				DataCenter:                    "datacenter1",
				ConfigFilePath:                wd + "/testdata/valid_config.yaml",
				NumConns:                      20,
				ReleaseVersion:                "4.0.0.6816",
				Partitioner:                   "org.apache.cassandra.dht.Murmur3Partitioner",
				Tokens:                        nil,
				CQLVersion:                    "3.4.5",
				LogLevel:                      "info",
				TcpBindPort:                   "0.0.0.0:%s",
				UseUnixSocket:                 false,
				UnixSocketPath:                "/tmp/cassandra-proxy.sock",
				ProxyCertFile:                 "",
				ProxyKeyFile:                  "",
				UserAgent:                     "cassandra-adapter/" + constants.ProxyReleaseVersion,
				ClientUid:                     0,
				QuickStartProjectId:           "my-project-id",
				QuickStartInstanceId:          "my-instance-id",
				QuickStartAppProfile:          "my-app-profile",
				QuickStartPort:                9041,
				QuickStartDefaultColumnFamily: "my-default-column-family",
				QuickStartSchemaMappingTable:  "my-schema-mapping-table",
			},
			wantErr: NO_ERROR_EXPECTED,
		},
		{
			name:    "unrecognized flag",
			args:    []string{"--blah"},
			want:    nil,
			wantErr: "unknown flag --blah",
		},
		{
			name:    "invalid num connections",
			args:    []string{"--num-conns", "0"},
			want:    nil,
			wantErr: "invalid number of connections",
		},
		{
			name:    "invalid protocol",
			args:    []string{"--protocol-version", "foo"},
			want:    nil,
			wantErr: "unsupported protocol version: foo",
		},
		{
			name:    "invalid max protocol",
			args:    []string{"--max-protocol-version", "foo"},
			want:    nil,
			wantErr: "unsupported max protocol version: foo",
		},
		{
			name:    "invalid protocol config",
			args:    []string{"--max-protocol-version", "3", "--protocol-version", "4"},
			want:    nil,
			wantErr: "default protocol version is greater than max protocol version",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseCliArgs(tt.args)
			if tt.wantErr != NO_ERROR_EXPECTED {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseProxyConfig(t *testing.T) {
	const NO_ERROR_EXPECTED = ""

	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("could not get current working directory: %v", err)
	}

	tests := []struct {
		name    string
		args    []string
		want    []*types.ProxyInstanceConfig
		wantErr string
	}{
		{
			name: "Valid config file",
			args: []string{"-f", wd + "/testdata/valid_config.yaml", "--num-conns=19"},
			want: []*types.ProxyInstanceConfig{
				{
					Port:     9092,
					Bind:     "0.0.0.0:9092",
					Options:  nil, // reference fixed by test fixture
					NumConns: 19,
					RPCAddr:  "",
					DC:       "datacenter1",
					Tokens:   nil,
					BigtableConfig: &types.BigtableConfig{
						ProjectID: "cassandra-prod-789",
						Instances: map[types.Keyspace]*types.InstanceMapping{
							"prodinstance001": {
								InstanceId:   "prod-instance-001",
								Keyspace:     "prodinstance001",
								AppProfileID: "prod-profile-123",
							},
						},
						SchemaMappingTable: "prod_table_config",
						Session: &types.Session{
							GrpcChannels: 3,
						},
						DefaultColumnFamily:      "cf_default",
						DefaultIntRowKeyEncoding: types.OrderedCodeEncoding,
						MetadataRefreshInterval:  (*time.Duration)(&DefaultMetadataRefreshInterval),
					},
					OtelConfig: &types.OtelConfig{
						Enabled: false,
					},
				},
			},
			wantErr: NO_ERROR_EXPECTED,
		},
		{
			name:    "invalid protocol config",
			args:    []string{"-f", wd + "/testdata/invalid_config.yaml"},
			want:    nil,
			wantErr: "failed to unmarshal config",
		},
		{
			name:    "no listeners provided",
			args:    []string{"-f", wd + "/testdata/no_listeners_config.yaml"},
			want:    nil,
			wantErr: "no listeners provided",
		},
		{
			name: "no listeners in yaml but quick start args given",
			args: []string{"-f", wd + "/testdata/no_listeners_config.yaml", "--port=9042", "--project-id=my-project", "--instance-id=my-instance"},
			want: []*types.ProxyInstanceConfig{
				{
					Port:     9042,
					Bind:     "0.0.0.0:9042",
					Options:  nil, // reference fixed by test fixture
					DC:       "datacenter1",
					NumConns: 20,
					BigtableConfig: &types.BigtableConfig{
						ProjectID: "my-project",
						Instances: map[types.Keyspace]*types.InstanceMapping{
							"my-instance": {
								InstanceId:   "my-instance",
								Keyspace:     "my-instance",
								AppProfileID: "default",
							},
						},
						SchemaMappingTable: "schema_mapping",
						Session: &types.Session{
							GrpcChannels: 1,
						},
						DefaultColumnFamily:      "cf1",
						DefaultIntRowKeyEncoding: types.OrderedCodeEncoding,
						MetadataRefreshInterval:  (*time.Duration)(&DefaultMetadataRefreshInterval),
					},
					OtelConfig: &types.OtelConfig{
						Enabled: false,
					},
				},
			},
			wantErr: NO_ERROR_EXPECTED,
		},
		{
			name: "quick start",
			args: []string{"-f", wd + "/testdata/no_listeners_config.yaml", "--port=1234", "--project-id=my-project", "--instance-id=my-instance", "--schema-mapping-table=sm", "--default-column-family=df", "--app-profile=cql-proxy"},
			want: []*types.ProxyInstanceConfig{
				{
					Port:     1234,
					Bind:     "0.0.0.0:1234",
					Options:  nil, // reference fixed by test fixture
					DC:       "datacenter1",
					NumConns: 20,
					BigtableConfig: &types.BigtableConfig{
						ProjectID: "my-project",
						Instances: map[types.Keyspace]*types.InstanceMapping{
							"my-instance": {
								InstanceId:   "my-instance",
								Keyspace:     "my-instance",
								AppProfileID: "cql-proxy",
							},
						},
						SchemaMappingTable: "sm",
						Session: &types.Session{
							GrpcChannels: 1,
						},
						DefaultColumnFamily:      "df",
						DefaultIntRowKeyEncoding: types.OrderedCodeEncoding,
						MetadataRefreshInterval:  (*time.Duration)(&DefaultMetadataRefreshInterval),
					},
					OtelConfig: &types.OtelConfig{
						Enabled: false,
					},
				},
			},
			wantErr: NO_ERROR_EXPECTED,
		},
		{
			name: "quick start with keyspace id",
			args: []string{"-f", wd + "/testdata/no_listeners_config.yaml", "--port=1234", "--project-id=my-project", "--instance-id=my-instance", "--keyspace-id=my-keyspace", "--schema-mapping-table=sm", "--default-column-family=df", "--app-profile=cql-proxy"},
			want: []*types.ProxyInstanceConfig{
				{
					Port:     1234,
					Bind:     "0.0.0.0:1234",
					Options:  nil, // reference fixed by test fixture
					DC:       "datacenter1",
					NumConns: 20,
					BigtableConfig: &types.BigtableConfig{
						ProjectID: "my-project",
						Instances: map[types.Keyspace]*types.InstanceMapping{
							"my-keyspace": {
								InstanceId:   "my-instance",
								Keyspace:     "my-keyspace",
								AppProfileID: "cql-proxy",
							},
						},
						SchemaMappingTable: "sm",
						Session: &types.Session{
							GrpcChannels: 1,
						},
						DefaultColumnFamily:      "df",
						DefaultIntRowKeyEncoding: types.OrderedCodeEncoding,
						MetadataRefreshInterval:  (*time.Duration)(&DefaultMetadataRefreshInterval),
					},
					OtelConfig: &types.OtelConfig{
						Enabled: false,
					},
				},
			},
			wantErr: NO_ERROR_EXPECTED,
		},
		{
			name:    "missing quickstart arg",
			args:    []string{"--instance-id=my-instance"},
			want:    nil,
			wantErr: "invalid cli configuration: missing project id for listener",
		},
		{
			name:    "missing quickstart arg 2",
			args:    []string{"--project-id=my-instance"},
			want:    nil,
			wantErr: "invalid cli configuration: missing an instance id for",
		},
		{
			name:    "quick start and yaml have same port",
			args:    []string{"-f", wd + "/testdata/valid_config.yaml", "--port=9092", "--project-id=my-project", "--instance-id=my-instance", "--schema-mapping-table=sm", "--default-column-family=df", "--app-profile=cql-proxy"},
			want:    nil,
			wantErr: "multiple listeners configured for port 9092",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args, err := ParseCliArgs(tt.args)
			require.NoError(t, err, "test args should be parsable")

			got, err := ParseProxyConfig(args)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, len(tt.want), len(got))
			// loop over each element because this is a clearer diff than assert.ElementsMatch
			for i := range tt.want {
				tt.want[i].Options = args // add a reference to the cli args
				assert.Equal(t, tt.want[i], got[i])
			}
		})
	}
}

func TestValidateInstanceConfigs(t *testing.T) {
	const NO_ERROR_EXPECTED = ""

	tests := []struct {
		name    string
		args    []*types.ProxyInstanceConfig
		wantErr string
	}{
		{
			name: "Duplicate ports",
			args: []*types.ProxyInstanceConfig{
				{
					Port:           9042,
					Tokens:         nil,
					BigtableConfig: nil,
					OtelConfig:     nil,
				},
				{
					Port:           9042,
					Tokens:         nil,
					BigtableConfig: nil,
					OtelConfig:     nil,
				},
			},
			wantErr: "multiple listeners configured for port 9042",
		},
		{
			name:    "Empty",
			args:    []*types.ProxyInstanceConfig{},
			wantErr: "no listeners provided",
		},
		{
			name: "Duplicate keyspaces",
			args: []*types.ProxyInstanceConfig{
				{
					Port: 1,
					BigtableConfig: &types.BigtableConfig{
						ProjectID: "my-project",
						Instances: map[types.Keyspace]*types.InstanceMapping{
							"foo": {
								InstanceId:   "",
								Keyspace:     "",
								AppProfileID: "",
							},
						},
						SchemaMappingTable:       "",
						Session:                  nil,
						DefaultColumnFamily:      "",
						DefaultIntRowKeyEncoding: 0,
					},
				},
			},
			wantErr: NO_ERROR_EXPECTED,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateInstanceConfigs(tt.args)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestValidateInstanceConfig(t *testing.T) {
	const NO_ERROR_EXPECTED = ""
	tests := []struct {
		name    string
		args    *types.ProxyInstanceConfig
		wantErr string
	}{
		{
			name: "valid config",
			args: &types.ProxyInstanceConfig{
				Port:   9042,
				Tokens: nil,
				BigtableConfig: &types.BigtableConfig{
					ProjectID: "project-123",
					Instances: map[types.Keyspace]*types.InstanceMapping{
						"k": {
							InstanceId:   "i",
							Keyspace:     "k",
							AppProfileID: "a",
						},
					},
					SchemaMappingTable: "table",
					Session: &types.Session{
						GrpcChannels: 1,
					},
					DefaultColumnFamily:      "cf1",
					DefaultIntRowKeyEncoding: types.OrderedCodeEncoding,
				},
				OtelConfig: nil,
			},
			wantErr: NO_ERROR_EXPECTED,
		},
		{
			name: "missing project id",
			args: &types.ProxyInstanceConfig{
				Port:   9042,
				Tokens: nil,
				BigtableConfig: &types.BigtableConfig{
					ProjectID:          "",
					Instances:          nil,
					SchemaMappingTable: "table",
					Session: &types.Session{
						GrpcChannels: 1,
					},
					DefaultColumnFamily:      "cf1",
					DefaultIntRowKeyEncoding: types.OrderedCodeEncoding,
				},
				OtelConfig: nil,
			},
			wantErr: "missing project id",
		},
		{
			name: "missing instance id",
			args: &types.ProxyInstanceConfig{
				Port:   9042,
				Tokens: nil,
				BigtableConfig: &types.BigtableConfig{
					ProjectID:          "",
					Instances:          nil,
					SchemaMappingTable: "table",
					Session: &types.Session{
						GrpcChannels: 1,
					},
					DefaultColumnFamily:      "cf1",
					DefaultIntRowKeyEncoding: types.OrderedCodeEncoding,
				},
				OtelConfig: nil,
			},
			wantErr: "missing project id",
		},
		{
			name: "missing keyspace id",
			args: &types.ProxyInstanceConfig{
				Port:   9042,
				Tokens: nil,
				BigtableConfig: &types.BigtableConfig{
					ProjectID: "my-project",
					Instances: map[types.Keyspace]*types.InstanceMapping{
						"": {
							InstanceId:   "my-instance",
							Keyspace:     "",
							AppProfileID: "default",
						},
					},
					SchemaMappingTable: "table",
					Session: &types.Session{
						GrpcChannels: 1,
					},
					DefaultColumnFamily:      "cf1",
					DefaultIntRowKeyEncoding: types.OrderedCodeEncoding,
				},
				OtelConfig: nil,
			},
			wantErr: "missing a keyspace",
		},
		{
			name: "no instances",
			args: &types.ProxyInstanceConfig{
				Port:   9042,
				Tokens: nil,
				BigtableConfig: &types.BigtableConfig{
					ProjectID:          "my-project",
					Instances:          nil,
					SchemaMappingTable: "table",
					Session: &types.Session{
						GrpcChannels: 1,
					},
					DefaultColumnFamily:      "cf1",
					DefaultIntRowKeyEncoding: types.OrderedCodeEncoding,
				},
				OtelConfig: nil,
			},
			wantErr: "missing instances for listener with port 9042",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateInstanceConfig(tt.args)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSetupLogger(t *testing.T) {
	tests := []struct {
		name    string
		args    *types.CliArgs
		wantErr bool
	}{
		{
			name:    "info log level",
			args:    &types.CliArgs{LogLevel: "info"},
			wantErr: false,
		},
		{
			name:    "debug log level",
			args:    &types.CliArgs{LogLevel: "debug"},
			wantErr: false,
		},
		{
			name:    "error log level",
			args:    &types.CliArgs{LogLevel: "error"},
			wantErr: false,
		},
		{
			name:    "warn log level",
			args:    &types.CliArgs{LogLevel: "warn"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseLoggerConfig(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetupLogger() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got == nil {
				t.Errorf("SetupLogger() = %v", got)
			}
		})
	}
}

func TestReadProxyConfig(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		wantErr string
		check   func(t *testing.T, cfg *yamlProxyConfig)
	}{
		{
			name: "Valid config with minimal settings",
			data: `
listeners:
  - name: "test-listener"
    port: 9042
    bigtable:
      projectId: "test-project"
      instances:
        - bigtableInstance: "test-instance"
          keyspace: "test-keyspace"
`,
			wantErr: "",
		},
		{
			name: "Invalid YAML",
			data: `
listeners:
  - name: "test-listener"
    port: 9042
    bigtable:
      projectId: "test-project"
      instances
        - bigtableInstance: "test-instance"
`,
			wantErr: "failed to unmarshal config",
		},
		{
			name: "Missing project ID and no global project ID",
			data: `
listeners:
  - name: "test-listener"
    port: 9042
    bigtable:
      instances:
        - bigtableInstance: "test-instance"
          keyspace: "test-keyspace"
`,
			wantErr: "project id is not defined",
		},
		{
			name: "Valid config with global project ID",
			data: `
cassandraToBigtableConfigs:
  projectId: "global-project"
listeners:
  - name: "test-listener"
    port: 9042
    bigtable:
      instances:
        - bigtableInstance: "test-instance"
          keyspace: "test-keyspace"
`,
			wantErr: "",
			check: func(t *testing.T, cfg *yamlProxyConfig) {
				assert.Equal(t, "global-project", cfg.Listeners[0].Bigtable.ProjectID)
			},
		},
		{
			name: "Default values applied",
			data: `
listeners:
  - name: "test-listener"
    port: 9042
    bigtable:
      projectId: "test-project"
      instances:
        - bigtableInstance: "test-instance"
          keyspace: "test-keyspace"
`,
			wantErr: "",
			check: func(t *testing.T, cfg *yamlProxyConfig) {
				assert.Equal(t, DefaultBigtableGrpcChannels, cfg.Listeners[0].Bigtable.Session.GrpcChannels)
				assert.Equal(t, DefaultColumnFamily, cfg.Listeners[0].Bigtable.DefaultColumnFamily)
				assert.Equal(t, DefaultSchemaMappingTableName, cfg.Listeners[0].Bigtable.SchemaMappingTable)
				assert.NotNil(t, cfg.Listeners[0].Bigtable.MetadataRefreshInterval)
				assert.Equal(t, DefaultMetadataRefreshInterval, *cfg.Listeners[0].Bigtable.MetadataRefreshInterval)
			},
		},
		{
			name: "Both instances and instance_ids set",
			data: `
listeners:
  - name: "test-listener"
    port: 9042
    bigtable:
      projectId: "test-project"
      instances:
        - bigtableInstance: "test-instance"
          keyspace: "test-keyspace"
      instanceIds: "inst1,inst2"
`,
			wantErr: "only one of 'instances' or 'instance_ids' should be set",
		},
		{
			name: "Neither instances nor instance_ids set",
			data: `
listeners:
  - name: "test-listener"
    port: 9042
    bigtable:
      projectId: "test-project"
`,
			wantErr: "either 'instances' or 'instance_ids' must be defined",
		},
		{
			name: "Valid MetadataRefreshInterval",
			data: `
listeners:
  - name: "test-listener"
    port: 9042
    bigtable:
      projectId: "test-project"
      instances:
        - bigtableInstance: "test-instance"
          keyspace: "test-keyspace"
      metadataRefreshInterval: "1m"
`,
			wantErr: "",
			check: func(t *testing.T, cfg *yamlProxyConfig) {
				assert.Equal(t, Duration(1*time.Minute), *cfg.Listeners[0].Bigtable.MetadataRefreshInterval)
			},
		},
		{
			name: "Invalid MetadataRefreshInterval",
			data: `
listeners:
  - name: "test-listener"
    port: 9042
    bigtable:
      projectId: "test-project"
      instances:
        - bigtableInstance: "test-instance"
          keyspace: "test-keyspace"
      metadataRefreshInterval: "invalid"
`,
			wantErr: "invalid duration \"invalid\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readProxyConfig([]byte(tt.data))
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, got)
				if tt.check != nil {
					tt.check(t, got)
				}
			}
		})
	}
}
