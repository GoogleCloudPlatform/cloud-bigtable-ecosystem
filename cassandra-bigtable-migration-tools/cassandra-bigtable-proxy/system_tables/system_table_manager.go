package system_tables

import (
	"crypto"
	"fmt"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/constants"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/mem_table"
	schemaMapping "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/metadata"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"go.uber.org/zap"
	"io"
	"net"
)

const (
	dseVersion = "5.1.21"
)

type SystemTableManager struct {
	metadataStore  *schemaMapping.MetadataStore
	db             *mem_table.InMemEngine
	logger         *zap.Logger
	configProvider SystemTableConfigProvider
	schemaVersion  primitive.UUID
}

type SystemTableConfigProvider interface {
	GetSystemTableConfig() SystemTableConfig
}

type SystemTableConfig struct {
	RpcAddress            string
	Datacenter            string
	ReleaseVersion        string
	Partitioner           string
	CqlVersion            string
	NativeProtocolVersion string
	Peers                 []PeerConfig
}

type PeerConfig struct {
	Addr   string
	Dc     string
	Tokens []string
}

func (s *SystemTableManager) Db() *mem_table.InMemEngine {
	return s.db
}

func NewSystemTableManager(metadataStore *schemaMapping.MetadataStore, logger *zap.Logger) *SystemTableManager {
	db := mem_table.NewInMemEngine()
	s := &SystemTableManager{
		metadataStore: metadataStore,
		db:            db,
		logger:        logger,
	}
	return s
}

func (s *SystemTableManager) Initialize(configProvider SystemTableConfigProvider) error {
	s.configProvider = configProvider

	// add system tables
	s.metadataStore.Schemas().AddTables(getSystemTableConfigs())
	// subscribe for changes, so we can rebuild the system table data
	s.metadataStore.Schemas().Subscribe(s)

	err := s.initializeSystemTables()
	if err != nil {
		return err
	}
	return nil
}

func (s *SystemTableManager) OnEvent(event schemaMapping.MetadataEvent) {
	s.logger.Info("received schema update event", zap.Any("event", event))
	err := s.ReloadSystemTables()
	if err != nil {
		s.logger.Error("failed to update system tables", zap.Error(err))
	}
}

func (s *SystemTableManager) initializeSystemTables() error {
	// add all system tables with empty data, so they're at least queryable
	for _, table := range getSystemTableConfigs() {
		err := s.db.SetData(table, nil)
		if err != nil {
			return err
		}
	}

	err := s.ReloadSystemTables()
	if err != nil {
		return err
	}

	return nil
}

func (s *SystemTableManager) ReloadSystemTables() error {
	var err error

	schemaVersion, err := s.metadataStore.Schemas().CalculateSchemaVersion()
	if err != nil {
		return err
	}
	s.schemaVersion = schemaVersion

	config := s.configProvider.GetSystemTableConfig()
	err = s.db.SetData(SystemSchemaTableKeyspace, s.getKeyspaceMetadata())
	if err != nil {
		return err
	}

	err = s.db.SetData(SystemSchemaTableTables, s.getTableMetadata())
	if err != nil {
		return err
	}

	err = s.db.SetData(SystemSchemaTableColumns, s.getColumnMetadata())
	if err != nil {
		return err
	}

	err = s.db.SetData(SystemTableLocal, s.getLocalMetadata(config))
	if err != nil {
		return err
	}

	err = s.db.SetData(SystemTablePeers, s.getPeerMetadata(config))
	if err != nil {
		return err
	}

	err = s.db.SetData(SystemTablePeersV2, s.getPeerV2Metadata(config))
	if err != nil {
		return err
	}

	return nil
}

func (s *SystemTableManager) getKeyspaceMetadata() []types.GoRow {
	var rows []types.GoRow
	for _, keyspace := range s.metadataStore.Schemas().Keyspaces() {
		md := keyspace.GetMetadata()
		row := types.GoRow{
			"keyspace_name":  md.KeyspaceName,
			"durable_writes": md.DurableWrites,
			"replication":    md.Replication,
		}
		rows = append(rows, row)
	}
	return rows
}

// getTableMetadata converts table metadata into table metadata rows
func (s *SystemTableManager) getTableMetadata() []types.GoRow {
	var rows []types.GoRow
	for _, t := range s.metadataStore.Schemas().Tables() {
		md := t.CreateTableMetadata()
		rows = append(rows, types.GoRow{
			"keyspace_name":           md.KeyspaceName,
			"table_name":              md.TableName,
			"additional_write_policy": md.AdditionalWritePolicy,
			"bloom_filter_fp_chance":  md.BloomFilterFpChance,
			"caching":                 md.Caching,
			"flags":                   md.Flags,
		})
	}
	return rows
}

// getColumnMetadata converts table metadata into column metadata rows
func (s *SystemTableManager) getColumnMetadata() []types.GoRow {
	var rows []types.GoRow
	for _, table := range s.metadataStore.Schemas().Tables() {
		for columnName, column := range table.Columns {
			position := table.GetCassandraPositionForColumn(columnName)
			md := column.SystemMetadata(position)
			rows = append(rows, types.GoRow{
				"keyspace_name":    md.KeyspaceName,
				"table_name":       md.TableName,
				"column_name":      md.ColumnName,
				"clustering_order": md.ClusteringOrder,
				"kind":             md.Kind,
				"position":         md.Position,
				"type":             md.Type,
			})
		}
	}
	return rows
}

func (s *SystemTableManager) getLocalMetadata(config SystemTableConfig) []types.GoRow {
	return []types.GoRow{
		{
			"rpc_address":             localIP(config.RpcAddress),
			"host_id":                 nameBasedUUID(localIP(config.RpcAddress).String()),
			"key":                     "local",
			"data_center":             config.Datacenter,
			"rack":                    "rack1",
			"tokens":                  []string{"-9223372036854775808"},
			"release_version":         config.ReleaseVersion,
			"partitioner":             config.Partitioner,
			"cluster_name":            fmt.Sprintf("cassandra-bigtable-proxy-%s", constants.ProxyReleaseVersion),
			"cql_version":             config.CqlVersion,
			"schema_version":          s.schemaVersion,
			"native_protocol_version": config.NativeProtocolVersion,
			"dse_version":             dseVersion,
		},
	}
}

func (s *SystemTableManager) getPeerMetadata(config SystemTableConfig) []types.GoRow {
	var rows []types.GoRow
	for _, peer := range config.Peers {
		rows = append(rows, types.GoRow{
			"peer":            peer.Addr,
			"rpc_address":     peer.Addr,
			"data_center":     peer.Dc,
			"dse_version":     dseVersion,
			"rack":            "rack1",
			"tokens":          peer.Tokens,
			"release_version": config.ReleaseVersion,
			"schema_version":  s.schemaVersion,
			"host_id":         nameBasedUUID(peer.Addr),
		})
	}
	return rows
}

func (s *SystemTableManager) getPeerV2Metadata(config SystemTableConfig) []types.GoRow {
	var rows []types.GoRow
	for _, peer := range config.Peers {
		rows = append(rows, types.GoRow{
			"peer":            peer.Addr,
			"peer_port":       9042,
			"rpc_address":     peer.Addr,
			"data_center":     peer.Dc,
			"dse_version":     dseVersion,
			"rack":            "rack1",
			"tokens":          peer.Tokens,
			"release_version": config.ReleaseVersion,
			"schema_version":  s.schemaVersion,
			"host_id":         nameBasedUUID(peer.Addr),
			"native_address":  peer.Addr,
			"native_port":     9042,
			"preferred_ip":    peer.Addr,
			"preferred_port":  9042,
		})
	}
	return rows
}

func localIP(address string) net.IP {
	if address != "" {
		return net.ParseIP(address)
	}
	return net.ParseIP("127.0.0.1")
}

func nameBasedUUID(name string) primitive.UUID {
	var uuid primitive.UUID
	m := crypto.MD5.New()
	_, _ = io.WriteString(m, name)
	hash := m.Sum(nil)
	for i := 0; i < len(uuid); i++ {
		uuid[i] = hash[i]
	}
	uuid[6] &= 0x0F
	uuid[6] |= 0x30
	uuid[8] &= 0x3F
	uuid[8] |= 0x80
	return uuid
}
