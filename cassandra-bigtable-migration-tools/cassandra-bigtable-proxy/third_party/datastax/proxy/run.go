// Copyright (c) DataStax, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package proxy

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	bigtableModule "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/bigtable"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/utilities"
	"github.com/alecthomas/kong"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
)

var (
	clusterPartitioner    = "org.apache.cassandra.dht.Murmur3Partitioner"
	clusterReleaseversion = "4.0.0.6816"
	defaultCqlVersion     = "3.4.5"
	defaultTcpBindPort    = "0.0.0.0:%s"
	proxyReleaseVersion   = "v1.0.1"
)

var readFile = os.ReadFile

const defaultConfigFile = "config.yaml"

// Config holds all the configuration data
type UserConfig struct {
	CassandraToBigtableConfigs CassandraToBigtableConfigs `yaml:"cassandraToBigtableConfigs"`
	Listeners                  []Listener                 `yaml:"listeners"`
	Otel                       *OtelConfig                `yaml:"otel"`
	LoggerConfig               *utilities.LoggerConfig    `yaml:"loggerConfig"`
}

// CassandraToBigtableConfigs contains configurations for Cassandra to bigtable proxy
type CassandraToBigtableConfigs struct {
	ProjectID          string `yaml:"projectId"`
	SchemaMappingTable string `yaml:"SchemaMappingTable"`
}
type OtelConfig struct {
	Enabled     bool   `yaml:"enabled"`
	ServiceName string `yaml:"serviceName"`
	HealthCheck struct {
		Enabled  bool   `yaml:"enabled"`
		Endpoint string `yaml:"endpoint"`
	} `yaml:"healthcheck"`
	Metrics struct {
		Endpoint string `yaml:"endpoint"`
	} `yaml:"metrics"`
	Traces struct {
		Endpoint      string  `yaml:"endpoint"`
		SamplingRatio float64 `yaml:"samplingRatio"`
	} `yaml:"traces"`
}

// Listener represents each listener configuration
type Listener struct {
	Name     string   `yaml:"name"`
	Port     int      `yaml:"port"`
	Bigtable Bigtable `yaml:"bigtable"`
	Otel     Otel     `yaml:"otel"`
}

// Bigtable holds the Bigtable database configuration

type InstancesMap struct {
	BigtableInstance string `yaml:"bigtableInstance"`
	Keyspace         string `yaml:"keyspace"`
	AppProfileID     string `yaml:"appProfileID"`
}
type Bigtable struct {
	ProjectID           string         `yaml:"projectId"`
	Instances           []InstancesMap `yaml:"instances"`
	InstanceIDs         string         `yaml:"instanceIds"`
	SchemaMappingTable  string         `yaml:"schemaMappingTable"`
	Session             Session        `yaml:"Session"`
	DefaultColumnFamily string         `yaml:"defaultColumnFamily"`
	CounterColumnFamily string         `yaml:"counterColumnFamily"`
	AppProfileID        string         `yaml:"appProfileID"`
}

// Session describes the settings for Bigtable sessions
type Session struct {
	GrpcChannels int `yaml:"grpcChannels"`
}

// Otel configures OpenTelemetry features
type Otel struct {
	Disabled bool `yaml:"disabled"`
}

type runConfig struct {
	Version            bool     `yaml:"version" help:"Show current proxy version" short:"v" default:"false" env:"PROXY_VERSION"`
	RpcAddress         string   `yaml:"rpc-address" help:"Address to advertise in the 'system.local' table for 'rpc_address'. It must be set if configuring peer proxies" env:"RPC_ADDRESS"`
	ProtocolVersion    string   `yaml:"protocol-version" help:"Initial protocol version to use when connecting to the backend cluster (default: v4, options: v3, v4, v5, DSEv1, DSEv2)" default:"v4" short:"n" env:"PROTOCOL_VERSION"`
	MaxProtocolVersion string   `yaml:"max-protocol-version" help:"Max protocol version supported by the backend cluster (default: v4, options: v3, v4, v5, DSEv1, DSEv2)" default:"v4" short:"m" env:"MAX_PROTOCOL_VERSION"`
	DataCenter         string   `yaml:"data-center" help:"Data center to use in system tables" default:"datacenter1"  env:"DATA_CENTER"`
	Bind               string   `yaml:"bind" help:"Address to use to bind server" short:"a" default:":9042" env:"BIND"`
	Config             *os.File `yaml:"-" help:"YAML configuration file" short:"f" env:"CONFIG_FILE"` // Not available in the configuration file
	NumConns           int      `yaml:"num-conns" help:"Number of connection to create to each node of the backend cluster" default:"20" env:"NUM_CONNS"`
	ReleaseVersion     string   `yaml:"release-version" help:"Cluster Release version" default:"4.0.0.6816"  env:"RELEASE_VERSION"`
	Partitioner        string   `yaml:"partitioner" help:"Partitioner partitioner" default:"org.apache.cassandra.dht.Murmur3Partitioner"  env:"PARTITIONER"`
	Tokens             []string `yaml:"tokens" help:"Tokens to use in the system tables. It's not recommended" env:"TOKENS"`
	CQLVersion         string   `yaml:"cql-version" help:"CQL version" default:"3.4.5"  env:"CQLVERSION"`
	LogLevel           string   `yaml:"log-level" help:"Log level configuration." default:"info" env:"LOG_LEVEL"`
	TcpBindPort        string   `yaml:"-" help:"YAML configuration file" short:"t" env:"TCP_BIND_PORT"`
	UseUnixSocket      bool     `help:"Use Unix Domain Socket instead of TCP." default:"false"`
	UnixSocketPath     string   `help:"Path for the Unix Domain Socket file." default:"/tmp/cassandra-proxy.sock"`
	ProxyCertFile      string   `yaml:"proxy-cert-file" help:"Path to a PEM encoded certificate file with its intermediate certificate chain. This is used to encrypt traffic for proxy clients" env:"PROXY_CERT_FILE"`
	ProxyKeyFile       string   `yaml:"proxy-key-file" help:"Path to a PEM encoded private key file. This is used to encrypt traffic for proxy clients" env:"PROXY_KEY_FILE"`
	// hidden because we only intend the java session wrapper to use this flag
	UserAgentOverride string `yaml:"-" help:"" hidden:"" optional:"" default:"" short:"u"`
	ClientPid         int32  `yaml:"client-pid" help:"" hidden:"" optional:"" default:"" short:""`
	ClientUid         uint32 `yaml:"client-uid" help:"" hidden:"" optional:"" default:"" short:""`
}

// Run starts the proxy command. 'args' shouldn't include the executable (i.e. os.Args[1:]). It returns the exit code
// for the proxy.
func Run(ctx context.Context, args []string) int {
	var cfg runConfig
	var err error

	configFile := defaultConfigFile
	if configFileEnv := os.Getenv("CONFIG_FILE"); len(configFileEnv) != 0 {
		configFile = configFileEnv
	}

	parser, err := kong.New(&cfg)
	if err != nil {
		panic(err)
	}

	var cliCtx *kong.Context
	if cliCtx, err = parser.Parse(args); err != nil {
		parser.Errorf("error parsing flags: %v", err)
		return 1
	}

	if cfg.Config != nil {
		bytes, err := io.ReadAll(cfg.Config)
		if err != nil {
			cliCtx.Errorf("unable to read contents of configuration file '%s': %v", cfg.Config.Name(), err)
			return 1
		}
		err = yaml.Unmarshal(bytes, &cfg)
		if err != nil {
			cliCtx.Errorf("invalid YAML in configuration file '%s': %v", cfg.Config.Name(), err)
		}
		configFile = cfg.Config.Name()
	}

	UserConfig, err := LoadConfig(configFile)
	if err != nil {
		log.Fatalf("error while loading config.yaml: %v", err)
	}

	if cfg.NumConns < 1 {
		cliCtx.Errorf("invalid number of connections, must be greater than 0 (provided: %d)", cfg.NumConns)
		return 1
	}

	var ok bool
	var version primitive.ProtocolVersion
	if version, ok = parseProtocolVersion(cfg.ProtocolVersion); !ok {
		cliCtx.Errorf("unsupported protocol version: %s", cfg.ProtocolVersion)
		return 1
	}

	var maxVersion primitive.ProtocolVersion
	if maxVersion, ok = parseProtocolVersion(cfg.MaxProtocolVersion); !ok {
		cliCtx.Errorf("unsupported max protocol version: %s", cfg.ProtocolVersion)
		return 1
	}

	if version > maxVersion {
		cliCtx.Errorf("default protocol version is greater than max protocol version")
		return 1
	}

	var partitioner string
	if cfg.Partitioner != "" {
		partitioner = cfg.Partitioner
	} else {
		partitioner = clusterPartitioner
	}

	var releaseVersion string
	if cfg.ReleaseVersion != "" {
		releaseVersion = cfg.ReleaseVersion
	} else {
		releaseVersion = clusterReleaseversion
	}

	var cqlVersion string
	if cfg.CQLVersion != "" {
		cqlVersion = cfg.CQLVersion
	} else {
		cqlVersion = defaultCqlVersion
	}

	flag := false
	supportedLogLevels := []string{"info", "debug", "error", "warn"}
	for _, level := range supportedLogLevels {
		if cfg.LogLevel == level {
			flag = true
		}
	}
	if !flag {
		cliCtx.Errorf("Invalid log-level should be [info/debug/error/warn]")
		return 1
	}

	logger, err := utilities.SetupLogger(cfg.LogLevel, UserConfig.LoggerConfig)
	if err != nil {
		cliCtx.Errorf("unable to create logger")
		return 1
	}
	defer logger.Sync()
	if cfg.Version {
		cliCtx.Printf("Version - " + proxyReleaseVersion)
		return 0
	}

	if UserConfig.Otel == nil {
		UserConfig.Otel = &OtelConfig{
			Enabled: false,
		}
	} else {
		if UserConfig.Otel.Enabled {
			if UserConfig.Otel.Traces.SamplingRatio < 0 || UserConfig.Otel.Traces.SamplingRatio > 1 {
				cliCtx.Errorf("Sampling Ratio for Otel Traces should be between 0 and 1]")
				return 1
			}
		}
	}

	// config logs.
	logger.Info("Protocol Version:" + version.String())
	logger.Info("CQL Version:" + cqlVersion)
	logger.Info("Release Version:" + releaseVersion)
	logger.Info("Partitioner:" + partitioner)
	logger.Info("Data Center:" + cfg.DataCenter)
	logger.Debug("Configuration - ", zap.Any("UserConfig", UserConfig))
	var wg sync.WaitGroup

	userAgent := "cassandra-adapter/" + proxyReleaseVersion
	if cfg.UserAgentOverride != "" {
		userAgent = cfg.UserAgentOverride
	}

	for _, listener := range UserConfig.Listeners {
		InstanceMap := make(map[string]bigtableModule.InstanceConfig)
		if listener.Bigtable.InstanceIDs != "" {
			instances := strings.Split(listener.Bigtable.InstanceIDs, ",")
			for _, v := range instances {
				InstanceMap[v] = bigtableModule.InstanceConfig{
					BigtableInstance: v,
					AppProfileId:     listener.Bigtable.AppProfileID,
				}
			}
		} else {
			for _, v := range listener.Bigtable.Instances {
				AppProfileId := listener.Bigtable.AppProfileID
				if v.AppProfileID != "" {
					AppProfileId = v.AppProfileID
				}
				InstanceMap[v.Keyspace] = bigtableModule.InstanceConfig{
					BigtableInstance: v.BigtableInstance,
					AppProfileId:     AppProfileId,
				}
			}
		}

		bigtableConfig := bigtableModule.BigtableConfig{
			NumOfChannels:       listener.Bigtable.Session.GrpcChannels,
			SchemaMappingTable:  listener.Bigtable.SchemaMappingTable,
			InstancesMap:        InstanceMap,
			GCPProjectID:        listener.Bigtable.ProjectID,
			DefaultColumnFamily: listener.Bigtable.DefaultColumnFamily,
			CounterColumnFamily: listener.Bigtable.CounterColumnFamily,
			// todo remove once we support ordered code ints
			EncodeIntValuesWithBigEndian: encodeIntValuesWithBigEndian,
		}

		p, err1 := NewProxy(ctx, Config{
			Version:        version,
			MaxVersion:     maxVersion,
			NumConns:       cfg.NumConns,
			Logger:         logger,
			RPCAddr:        cfg.RpcAddress,
			DC:             cfg.DataCenter,
			Tokens:         cfg.Tokens,
			BigtableConfig: bigtableConfig,
			Partitioner:    partitioner,
			ReleaseVersion: releaseVersion,
			CQLVersion:     cqlVersion,
			OtelConfig:     UserConfig.Otel,
			UserAgent:      userAgent,
			ClientPid:      cfg.ClientPid,
			ClientUid:      cfg.ClientUid,
		})

		if err1 != nil {
			logger.Error(err1.Error())
			return 1
		}
		tcpPort := defaultTcpBindPort
		if cfg.TcpBindPort != "" {
			tcpPort = cfg.TcpBindPort
		}
		cfgloop := cfg
		cfgloop.Bind = fmt.Sprintf(tcpPort, strconv.Itoa(listener.Port))
		cfgloop.Bind = maybeAddPort(cfgloop.Bind, "9042")

		var mux http.ServeMux
		wg.Add(1)
		go func(cfg runConfig, p *Proxy, mux *http.ServeMux) {
			defer wg.Done()
			err := cfg.listenAndServe(p, mux, ctx, logger) // Use cfg2 or other instances as needed
			if err != nil {
				logger.Fatal("Error while serving - ", zap.Error(err))
			}
		}(cfgloop, p, &mux)

	}
	wg.Wait() // Wait for all servers to finish
	logger.Debug("\n>>>>>>>>>>>>> Closed All listeners <<<<<<<<<\n")

	return 0
}

// LoadConfig reads and parses the configuration from a YAML file
func LoadConfig(filename string) (*UserConfig, error) {
	data, err := readFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config UserConfig
	if err = yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	if err = ValidateAndApplyDefaults(&config); err != nil {
		return nil, err
	}
	return &config, nil
}

func parseProtocolVersion(s string) (version primitive.ProtocolVersion, ok bool) {
	ok = true
	lowered := strings.ToLower(s)
	if lowered == "3" || lowered == "v3" {
		version = primitive.ProtocolVersion3
	} else if lowered == "4" || lowered == "v4" {
		version = primitive.ProtocolVersion4
	} else if lowered == "5" || lowered == "v5" {
		version = primitive.ProtocolVersion5
	} else if lowered == "65" || lowered == "dsev1" {
		version = primitive.ProtocolVersionDse1
	} else if lowered == "66" || lowered == "dsev2" {
		version = primitive.ProtocolVersionDse1
	} else {
		ok = false
	}
	return version, ok
}

// maybeAddPort adds the default port to an IP; otherwise, it returns the original address.
func maybeAddPort(addr string, defaultPort string) string {
	if net.ParseIP(addr) != nil {
		return net.JoinHostPort(addr, defaultPort)
	}
	return addr
}

// listenAndServe correctly handles serving both the proxy and an HTTP server simultaneously.
func (c *runConfig) listenAndServe(p *Proxy, mux *http.ServeMux, ctx context.Context, logger *zap.Logger) (err error) {
	logger.Info("Starting proxy with configuration:\n")
	logger.Info(fmt.Sprintf("  Bind: %s\n", c.Bind))
	logger.Info(fmt.Sprintf("  Use Unix Socket: %v\n", c.UseUnixSocket))
	logger.Info(fmt.Sprintf("  Unix Socket Path: %s\n", c.UnixSocketPath))
	logger.Info(fmt.Sprintf("  Use TLS: %v\n", c.ProxyCertFile != "" && c.ProxyKeyFile != ""))

	var listeners []net.Listener

	// Set up listener based on configuration
	if c.UseUnixSocket {
		// Use Unix Domain Socket
		unixListener, err := resolveAndListen("", true, c.UnixSocketPath, "", "", logger)
		if err != nil {
			return fmt.Errorf("failed to create Unix socket listener: %v", err)
		}
		listeners = append(listeners, unixListener)
		logger.Info(fmt.Sprintf("Unix socket listener created successfully at %s\n", c.UnixSocketPath))
	} else {
		// Use TCP
		tcpListener, err := resolveAndListen(c.Bind, false, "", c.ProxyCertFile, c.ProxyKeyFile, logger)
		if err != nil {
			return fmt.Errorf("failed to create TCP listener: %v", err)
		}
		listeners = append(listeners, tcpListener)
		logger.Info(fmt.Sprintf("TCP listener created successfully on %s\n", c.Bind))
	}

	// Set up Bigtable client
	logger.Info("Initializing Bigtable client...\n")
	err = p.Connect()
	if err != nil {
		for _, l := range listeners {
			l.Close()
		}
		return err
	}
	logger.Info("Bigtable client initialized successfully\n")

	var wg sync.WaitGroup
	ch := make(chan error)
	numServers := len(listeners)

	wg.Add(numServers)

	go func() {
		wg.Wait()
		close(ch)
	}()

	go func() {
		select {
		case <-ctx.Done():
			logger.Debug("proxy interrupted/killed")
			_ = p.Close()
		}
	}()

	// Serve on all listeners
	for _, listener := range listeners {
		go func(l net.Listener) {
			defer wg.Done()
			// WARNING: Do NOT change this log - the cassandra-bigtable-java-client-lib and compliance tests use the "Starting to serve on listener" log message to check for start up.
			logger.Info(fmt.Sprintf("Starting to serve on listener: %v\n", l.Addr()))
			err := p.Serve(l)
			if err != nil && err != ErrProxyClosed {
				ch <- err
			}
		}(listener)
	}

	for err = range ch {
		if err != nil {
			return err
		}
	}

	return err
}

// resolveAndListen creates and returns a TCP, TLS, or Unix Domain Socket listener
func resolveAndListen(bind string, useUnixSocket bool, unixSocketPath, certFile, keyFile string, logger *zap.Logger) (net.Listener, error) {
	if useUnixSocket {
		// Remove existing socket file if it exists
		if err := os.RemoveAll(unixSocketPath); err != nil {
			return nil, fmt.Errorf("failed to remove existing socket file: %v", err)
		}
		logger.Debug(fmt.Sprintf("Creating Unix Domain Socket"))
		listener, err := net.Listen("unix", unixSocketPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create Unix Domain Socket: %v", err)
		}
		logger.Debug("Successfully created Unix Domain Socket listener\n")

		// Set socket permissions
		if err := os.Chmod(unixSocketPath, 0666); err != nil {
			return nil, fmt.Errorf("failed to set socket permissions: %v", err)
		}
		logger.Debug("Set socket permissions\n")

		return listener, nil
	}

	if certFile != "" && keyFile != "" {
		logger.Info(fmt.Sprintf("Setting up TLS listener with cert: %s and key: %s\n", certFile, keyFile))
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("unable to load TLS certificate pair: %v", err)
		}
		config := &tls.Config{
			Certificates: []tls.Certificate{cert},
		}
		listener, err := tls.Listen("tcp", bind, config)
		if err != nil {
			return nil, fmt.Errorf("failed to create TLS listener: %v", err)
		}
		logger.Info(fmt.Sprintf("Successfully created TLS listener on %s\n", bind))
		return listener, nil
	}

	logger.Info(fmt.Sprintf("Setting up TCP listener on %s\n", bind))
	listener, err := net.Listen("tcp", bind)

	if err != nil {
		return nil, fmt.Errorf("failed to create TCP listener: %v", err)
	}

	logger.Info("Successfully created TCP listener\n")
	return listener, nil

}
