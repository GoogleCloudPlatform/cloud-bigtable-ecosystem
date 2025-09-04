package types

import (
	"github.com/datastax/go-cassandra-native-protocol/primitive"
)

type CliArgs struct {
	Version            bool
	RpcAddress         string
	ProtocolVersion    primitive.ProtocolVersion
	MaxProtocolVersion primitive.ProtocolVersion
	DataCenter         string
	ConfigFilePath     string
	NumConns           int
	ReleaseVersion     string
	Partitioner        string
	Tokens             []string
	CQLVersion         string
	LogLevel           string
	TcpBindPort        string
	UseUnixSocket      bool
	UnixSocketPath     string
	ProxyCertFile      string
	ProxyKeyFile       string
	UserAgent          string
	ClientPid          int32
	ClientUid          uint32
	// quick start config
	QuickStartPort                int
	QuickStartProjectId           string
	QuickStartInstanceId          string
	QuickStartKeyspaceId          string
	QuickStartAppProfile          string
	QuickStartSchemaMappingTable  string
	QuickStartDefaultColumnFamily string
}

type OtelConfig struct {
	Enabled     bool
	ServiceName string
	HealthCheck struct {
		Enabled  bool
		Endpoint string
	}
	Metrics struct {
		Endpoint string
	}
	Traces struct {
		Endpoint      string
		SamplingRatio float64
	}
}

type InstancesMapping struct {
	BigtableInstance string
	Keyspace         string
	AppProfileID     string
}

type BigtableConfig struct {
	ProjectID                string
	Instances                map[string]*InstancesMapping
	SchemaMappingTable       string
	Session                  *Session
	DefaultColumnFamily      string
	DefaultIntRowKeyEncoding IntRowKeyEncodingType
}

type Session struct {
	GrpcChannels int
}

type ProxyInstanceConfig struct {
	Port           int
	Options        *CliArgs
	Bind           string
	NumConns       int
	RPCAddr        string
	DC             string
	Tokens         []string
	BigtableConfig *BigtableConfig
	OtelConfig     *OtelConfig
}
