package types

import (
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"time"
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

type BigtableInstance string

type InstanceMapping struct {
	InstanceId   BigtableInstance
	Keyspace     Keyspace
	AppProfileID string
}

type BigtableConfig struct {
	ProjectID                string
	Instances                map[Keyspace]*InstanceMapping
	SchemaMappingTable       TableName
	Session                  *Session
	DefaultColumnFamily      ColumnFamily
	DefaultIntRowKeyEncoding IntRowKeyEncodingType
	MetadataRefreshInterval  *time.Duration
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
