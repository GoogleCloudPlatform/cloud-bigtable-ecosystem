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
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/alecthomas/kong"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	bigtableModule "github.com/ollionorg/cassandra-to-bigtable-proxy/bigtable"
	"github.com/ollionorg/cassandra-to-bigtable-proxy/utilities"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
)

var (
	clusterPartitioner    = "org.apache.cassandra.dht.Murmur3Partitioner"
	clusterReleaseversion = "4.0.0.6816"
	defaultCqlVersion     = "3.4.5"
	TCP_BIND_PORT         = "0.0.0.0:%s"
	proxyReleaseVersion   = "v1.0.1"
)

var readFile = os.ReadFile

const defaultConfigFile = "config.yaml"

// Config holds all the configuration data
type UserConfig struct {
	CassandraToBigtableConfigs CassandraToBigtableConfigs `yaml:"cassandra_to_bigtable_configs"`
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
	Enabled                  bool   `yaml:"enabled"`
	EnabledClientSideMetrics bool   `yaml:"enabledClientSideMetrics"`
	ServiceName              string `yaml:"serviceName"`
	HealthCheck              struct {
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
type Bigtable struct {
	ProjectID           string  `yaml:"projectId"`
	InstanceIDs         string  `yaml:"instanceIds"`
	SchemaMappingTable  string  `yaml:"schemaMappingTable"`
	Session             Session `yaml:"Session"`
	DefaultColumnFamily string  `yaml:"defaultColumnFamily"`
	AppProfileID        string  `yaml:"appProfileID"`
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

	for _, listener := range UserConfig.Listeners {
		bigtableConfig := bigtableModule.BigtableConfig{
			NumOfChannels:       listener.Bigtable.Session.GrpcChannels,
			SchemaMappingTable:  listener.Bigtable.SchemaMappingTable,
			InstanceID:          listener.Bigtable.InstanceIDs,
			GCPProjectID:        listener.Bigtable.ProjectID,
			DefaultColumnFamily: listener.Bigtable.DefaultColumnFamily,
			AppProfileID:        listener.Bigtable.AppProfileID,
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
			UserAgent:      "cassandra-adapter/" + proxyReleaseVersion,
		})

		if err1 != nil {
			logger.Error(err1.Error())
			return 1
		}
		cfgloop := cfg
		cfgloop.Bind = fmt.Sprintf(TCP_BIND_PORT, strconv.Itoa(listener.Port))
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
	var wg sync.WaitGroup
	ch := make(chan error)
	numServers := 1 // Without the HTTP server

	// Connect and listen is called first to set up the listening server connection and establish initial client
	// connections to the backend cluster so that when the readiness check is hit the proxy is actually ready.

	err = p.Connect()
	if err != nil {
		return err
	}

	proxyListener, err := resolveAndListen(c.Bind)
	if err != nil {
		return err
	}

	logger.Info("proxy is listening", zap.Stringer("address", proxyListener.Addr()))

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

	go func() {
		defer wg.Done()
		err := p.Serve(proxyListener)
		if err != nil && err != ErrProxyClosed {
			ch <- err
		}
	}()

	for err = range ch {
		if err != nil {
			return err
		}
	}

	return err
}

// resolveAndListen creates and returns a TCP listener
func resolveAndListen(address string) (net.Listener, error) {
	return net.Listen("tcp", address)
}
