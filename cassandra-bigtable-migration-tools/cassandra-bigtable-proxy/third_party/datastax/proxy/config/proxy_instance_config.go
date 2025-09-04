package config

import (
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
)

func NewProxyInstanceConfig(cliArgs *types.CliArgs, port int, otel *types.OtelConfig, bigtableConfig *types.BigtableConfig) *types.ProxyInstanceConfig {
	config := &types.ProxyInstanceConfig{
		Port:           port,
		Bind:           buildBindAndPort(cliArgs.TcpBindPort, port),
		Options:        cliArgs,
		NumConns:       cliArgs.NumConns,
		RPCAddr:        cliArgs.RpcAddress,
		DC:             cliArgs.DataCenter,
		Tokens:         cliArgs.Tokens,
		OtelConfig:     otel,
		BigtableConfig: bigtableConfig,
	}
	return config
}
