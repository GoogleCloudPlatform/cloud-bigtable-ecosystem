package config

import (
	"errors"
	"fmt"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/constants"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	"github.com/alecthomas/kong"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/yaml.v2"
	"net"
	"os"
	"strconv"
	"time"
)

const (
	defaultClusterPartitioner = "org.apache.cassandra.dht.Murmur3Partitioner"
	clusterReleaseVersion     = "4.0.0.6816"
	defaultCqlVersion         = "3.4.5"
	defaultTcpBindPort        = "0.0.0.0:%s"
)

type rawCliArgs struct {
	Version            bool     `yaml:"version" help:"Show current proxy version" short:"v" default:"false"`
	RpcAddress         string   `yaml:"rpc-address" help:"Address to advertise in the 'system.local' table for 'rpc_address'. It must be set if configuring peer proxies" env:"RPC_ADDRESS"`
	ProtocolVersion    string   `yaml:"protocol-version" help:"Initial protocol version to use when connecting to the backend cluster (default: v4, options: v3, v4, v5, DSEv1, DSEv2)" default:"v4" short:"n" env:"PROTOCOL_VERSION"`
	MaxProtocolVersion string   `yaml:"max-protocol-version" help:"Max protocol version supported by the backend cluster (default: v4, options: v3, v4, v5, DSEv1, DSEv2)" default:"v4" short:"m" env:"MAX_PROTOCOL_VERSION"`
	DataCenter         string   `yaml:"data-center" help:"Data center to use in system tables" default:"datacenter1"  env:"DATA_CENTER"`
	Config             *os.File `yaml:"-" help:"YAML configuration file" short:"f" env:"CONFIG_FILE"` // Not available in the configuration file
	NumConns           int      `yaml:"num-conns" help:"Number of connection to create to each node of the backend cluster" default:"20" env:"NUM_CONNS"`
	ReleaseVersion     string   `yaml:"release-version" help:"Cluster Release version" default:"4.0.0.6816"  env:"RELEASE_VERSION"`
	Partitioner        string   `yaml:"partitioner" help:"Partitioner partitioner" default:"org.apache.cassandra.dht.Murmur3Partitioner"  env:"PARTITIONER"`
	Tokens             []string `yaml:"tokens" help:"Tokens to use in the system tables. It's not recommended" env:"TOKENS"`
	CQLVersion         string   `yaml:"cql-version" help:"CQL version" default:"3.4.5"  env:"CQLVERSION"`
	LogLevel           string   `yaml:"log-level" help:"Log level configuration." default:"info" env:"LOG_LEVEL"`
	TcpBindPort        string   `yaml:"-" help:"YAML configuration file" short:"t" env:"TCP_BIND_PORT" default:"0.0.0.0:%s"`
	UseUnixSocket      bool     `help:"Use Unix Domain Socket instead of TCP." default:"false"`
	UnixSocketPath     string   `help:"Path for the Unix Domain Socket file." default:"/tmp/cassandra-proxy.sock"`
	ProxyCertFile      string   `yaml:"proxy-cert-file" help:"Path to a PEM encoded certificate file with its intermediate certificate chain. This is used to encrypt traffic for proxy clients" env:"PROXY_CERT_FILE"`
	ProxyKeyFile       string   `yaml:"proxy-key-file" help:"Path to a PEM encoded private key file. This is used to encrypt traffic for proxy clients" env:"PROXY_KEY_FILE"`
	// hidden because we only intend the java session wrapper to use this flag
	UserAgentOverride string `yaml:"-" help:"" hidden:"" optional:"" default:"" short:"u"`
	ClientPid         int32  `yaml:"client-pid" help:"" hidden:"" optional:"" default:"" short:""`
	ClientUid         uint32 `yaml:"client-uid" help:"" hidden:"" optional:"" default:"" short:""`
	// quick start config - used for running the proxy without a yaml config file
	ProjectId           string `yaml:"project-id" help:"Google Cloud Project Id to use."`
	InstanceId          string `yaml:"instance-id" help:"Bigtable Instance Id to use."`
	KeyspaceId          string `yaml:"keyspace-id" help:"Cassandra Keyspace which will map to the instance-id option."`
	AppProfile          string `yaml:"app-profile" help:"Bigtable App Profile to use." default:"default"`
	Port                int    `yaml:"port" help:"Port to serve CQL traffic on." default:"9042"`
	DefaultColumnFamily string `yaml:"default-column-family" help:"The Bigtable column family used for storing scalar values." default:"cf1"`
	SchemaMappingTable  string `yaml:"metadata-table" help:"The Bigtable table name used for storing schema information (automatically created by the proxy)." default:"schema_mapping"`
}

func ParseCliArgs(args []string) (*types.CliArgs, error) {
	var parsed rawCliArgs

	parser, err := kong.New(&parsed)
	if err != nil {
		return nil, err
	}

	if _, err = parser.Parse(args); err != nil {
		return nil, fmt.Errorf("error parsing flags: %w", err)
	}

	if parsed.Partitioner == "" {
		parsed.Partitioner = defaultClusterPartitioner
	}

	if parsed.ReleaseVersion == "" {
		parsed.ReleaseVersion = clusterReleaseVersion
	}

	if parsed.CQLVersion == "" {
		parsed.CQLVersion = defaultCqlVersion
	}

	if parsed.TcpBindPort == "" {
		parsed.TcpBindPort = defaultTcpBindPort
	}

	var ok bool
	var version primitive.ProtocolVersion
	if version, ok = parseProtocolVersion(parsed.ProtocolVersion); !ok {
		return nil, fmt.Errorf("unsupported protocol version: %s", parsed.ProtocolVersion)
	}

	var maxVersion primitive.ProtocolVersion
	if maxVersion, ok = parseProtocolVersion(parsed.MaxProtocolVersion); !ok {
		return nil, fmt.Errorf("unsupported max protocol version: %s", parsed.MaxProtocolVersion)
	}

	configFilePath := ""
	if parsed.Config != nil {
		configFilePath = parsed.Config.Name()
	}

	userAgent := "cassandra-adapter/" + constants.ProxyReleaseVersion
	if parsed.UserAgentOverride != "" {
		userAgent = parsed.UserAgentOverride
	}

	result := types.CliArgs{
		Version:                       parsed.Version,
		RpcAddress:                    parsed.RpcAddress,
		ProtocolVersion:               version,
		MaxProtocolVersion:            maxVersion,
		DataCenter:                    parsed.DataCenter,
		ConfigFilePath:                configFilePath,
		NumConns:                      parsed.NumConns,
		ReleaseVersion:                parsed.ReleaseVersion,
		Partitioner:                   parsed.Partitioner,
		Tokens:                        parsed.Tokens,
		CQLVersion:                    parsed.CQLVersion,
		LogLevel:                      parsed.LogLevel,
		TcpBindPort:                   parsed.TcpBindPort,
		UseUnixSocket:                 parsed.UseUnixSocket,
		UnixSocketPath:                parsed.UnixSocketPath,
		ProxyCertFile:                 parsed.ProxyCertFile,
		ProxyKeyFile:                  parsed.ProxyKeyFile,
		UserAgent:                     userAgent,
		ClientPid:                     parsed.ClientPid,
		ClientUid:                     parsed.ClientUid,
		QuickStartProjectId:           parsed.ProjectId,
		QuickStartInstanceId:          parsed.InstanceId,
		QuickStartKeyspaceId:          parsed.KeyspaceId,
		QuickStartAppProfile:          parsed.AppProfile,
		QuickStartPort:                parsed.Port,
		QuickStartDefaultColumnFamily: parsed.DefaultColumnFamily,
		QuickStartSchemaMappingTable:  parsed.SchemaMappingTable,
	}

	err = validateCliArgs(&result)
	if err != nil {
		return nil, err
	}

	return &result, err
}

func maybeParseQuickStartArgs(args *types.CliArgs) (*types.ProxyInstanceConfig, error) {
	if args.QuickStartProjectId == "" && args.QuickStartInstanceId == "" {
		return nil, nil
	}

	// use keyspace-id but fallback to the bigtable instance id.
	keyspace := types.Keyspace(assignWithFallbacks(args.QuickStartKeyspaceId, args.QuickStartInstanceId))

	bigtableConfig := &types.BigtableConfig{
		ProjectID: args.QuickStartProjectId,
		Instances: map[types.Keyspace]*types.InstanceMapping{
			keyspace: {
				InstanceId:   types.BigtableInstance(args.QuickStartInstanceId),
				Keyspace:     keyspace,
				AppProfileID: assignWithFallbacks(args.QuickStartAppProfile, DefaultAppProfileId),
			},
		},
		SchemaMappingTable: types.TableName(assignWithFallbacks(args.QuickStartSchemaMappingTable, DefaultSchemaMappingTableName)),
		Session: &types.Session{
			GrpcChannels: DefaultBigtableGrpcChannels,
		},
		DefaultColumnFamily:      types.ColumnFamily(args.QuickStartDefaultColumnFamily),
		DefaultIntRowKeyEncoding: types.OrderedCodeEncoding,
		MetadataRefreshInterval:  (*time.Duration)(&DefaultMetadataRefreshInterval),
	}

	// quick start instances don't have a way to configure otel, so just disable it
	otel := &types.OtelConfig{
		Enabled: false,
	}

	instanceConfig := NewProxyInstanceConfig(args, args.QuickStartPort, otel, bigtableConfig)

	err := validateInstanceConfig(instanceConfig)
	if err != nil {
		return nil, fmt.Errorf("invalid cli configuration: %w", err)
	}

	return instanceConfig, nil
}

// maybeAddPort adds the default port to an IP; otherwise, it returns the original address.
func maybeAddPort(addr string, defaultPort string) string {
	if net.ParseIP(addr) != nil {
		return net.JoinHostPort(addr, defaultPort)
	}
	return addr
}

func ParseLoggerConfig(args *types.CliArgs) (*zap.Logger, error) {
	level := zap.NewAtomicLevel()
	err := level.UnmarshalText([]byte(args.LogLevel))
	if err != nil {
		return nil, err
	}

	var loggerConfig *yamlLoggerConfig = nil
	if args.ConfigFilePath != "" {
		config, err := readProxyConfigFile(args.ConfigFilePath)
		if err != nil {
			return nil, err
		}
		loggerConfig = config.LoggerConfig
	}

	if loggerConfig != nil && loggerConfig.OutputType == "file" {
		return setupFileLogger(level, loggerConfig)
	}

	return setupConsoleLogger(level)
}

// setupFileLogger() configures a zap.Logger for file output using a lumberjack.Logger for log rotation.
// Accepts a zap.AtomicLevel and a yamlLoggerConfig struct to customize log output and rotation settings.
// Returns the configured zap.Logger or an error if setup fails.
func setupFileLogger(level zap.AtomicLevel, loggerConfig *yamlLoggerConfig) (*zap.Logger, error) {
	filename := loggerConfig.Filename
	if filename == "" {
		filename = "/var/log/cassandra-to-spanner-proxy/output.log"
	}
	maxAge := loggerConfig.MaxAge
	if maxAge == 0 {
		maxAge = 3 // setting default value to 3 days
	}
	maxBackups := loggerConfig.MaxBackups
	if maxBackups == 0 {
		maxBackups = 10 // setting default max backups to 10 files
	}
	rotationalLogger := &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    loggerConfig.MaxSize, // megabytes, default 100MB
		MaxAge:     maxAge,
		MaxBackups: maxBackups,
		Compress:   loggerConfig.Compress,
	}
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
		zapcore.AddSync(rotationalLogger),
		level,
	)
	return zap.New(core), nil
}

// setupConsoleLogger() configures a zap.Logger for console output.
// Accepts a zap.AtomicLevel to set the logging level.
// Returns the configured zap.Logger or an error if setup fails.
func setupConsoleLogger(level zap.AtomicLevel) (*zap.Logger, error) {
	config := zap.Config{
		Encoding:         "json", // or "console"
		Level:            level,  // default log level
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "time",
			CallerKey:      "caller",
			LevelKey:       "level",
			NameKey:        "logger",
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.StringDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
	}

	return config.Build()
}

func readProxyConfigFile(path string) (*yamlProxyConfig, error) {
	fileData, err := readFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	return readProxyConfig(fileData)
}

func readProxyConfig(fileData []byte) (*yamlProxyConfig, error) {
	var err error
	var config yamlProxyConfig
	if err = yaml.Unmarshal(fileData, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	err = validateAndApplyDefaults(&config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func ParseProxyConfig(args *types.CliArgs) ([]*types.ProxyInstanceConfig, error) {
	var instanceConfigs []*types.ProxyInstanceConfig = nil
	if args.ConfigFilePath != "" {
		config, err := readProxyConfigFile(args.ConfigFilePath)
		if err != nil {
			return nil, err
		}
		instanceConfigs, err = loadProxyConfigFile(config, args)
		if err != nil {
			return nil, fmt.Errorf("error while loading config.yaml: %w", err)
		}
	}

	quickStartInstanceConfig, err := maybeParseQuickStartArgs(args)
	if err != nil {
		return nil, err
	}

	if quickStartInstanceConfig != nil {
		instanceConfigs = append(instanceConfigs, quickStartInstanceConfig)
	}

	err = validateInstanceConfigs(instanceConfigs)
	if err != nil {
		return nil, err
	}

	return instanceConfigs, nil
}

func validateInstanceConfigs(configs []*types.ProxyInstanceConfig) error {
	if len(configs) == 0 {
		return errors.New("no listeners provided. please provide at least one listener via config.yaml or cli options")
	}

	found, duplicatePort := findFirstDuplicateValue(configs, func(config *types.ProxyInstanceConfig) string { return strconv.Itoa(config.Port) })
	if found {
		return fmt.Errorf("multiple listeners configured for port %s", duplicatePort)
	}
	return nil
}
