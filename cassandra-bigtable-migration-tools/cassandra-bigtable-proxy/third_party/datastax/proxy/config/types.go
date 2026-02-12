package config

type yamlProxyConfig struct {
	CassandraToBigtableConfigs yamlCassandraToBigtableConfigs `yaml:"cassandraToBigtableConfigs"`
	Listeners                  []yamlListener                 `yaml:"listeners"`
	Otel                       *yamlOtelConfig                `yaml:"otel"`
	LoggerConfig               *yamlLoggerConfig              `yaml:"loggerConfig"`
}

type yamlLoggerConfig struct {
	OutputType string `yaml:"outputType"`
	Filename   string `yaml:"fileName"`
	MaxSize    int    `yaml:"maxSize"`    // megabytes
	MaxBackups int    `yaml:"maxBackups"` // The value of MaxBackups determines how many previous log files are kept after a new log file is created due to the MaxSize or MaxAge limits.
	MaxAge     int    `yaml:"maxAge"`     // days
	Compress   bool   `yaml:"compress"`   // the rotated log files to be compressed to save disk space.
}

type yamlCassandraToBigtableConfigs struct {
	ProjectID          string `yaml:"projectId"`
	SchemaMappingTable string `yaml:"DefaultSchemaMappingTableName"`
}
type yamlOtelConfig struct {
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

type yamlListener struct {
	Name     string       `yaml:"name"`
	Port     int          `yaml:"port"`
	Bigtable yamlBigtable `yaml:"bigtable"`
	Otel     yamlOtel     `yaml:"otel"`
}

type yamlInstancesMap struct {
	BigtableInstance string `yaml:"bigtableInstance"`
	Keyspace         string `yaml:"keyspace"`
	AppProfileID     string `yaml:"appProfileID"`
}

type yamlBigtable struct {
	ProjectID                     string             `yaml:"projectId"`
	Instances                     []yamlInstancesMap `yaml:"instances"`
	InstanceIDs                   string             `yaml:"instanceIds"`
	SchemaMappingTable            string             `yaml:"schemaMappingTable"`
	Session                       yamlSession        `yaml:"Session"`
	DefaultColumnFamily           string             `yaml:"defaultColumnFamily"`
	AppProfileID                  string             `yaml:"appProfileID"`
	EncodeIntRowKeysWithBigEndian bool               `yaml:"encodeIntRowKeysWithBigEndian"`
	MetadataRefreshInterval       int                `yaml:"metadataRefreshInterval"`
}

type yamlSession struct {
	GrpcChannels int `yaml:"grpcChannels"`
}

// yamlOtel configures OpenTelemetry features
type yamlOtel struct {
	Disabled bool `yaml:"disabled"`
}
