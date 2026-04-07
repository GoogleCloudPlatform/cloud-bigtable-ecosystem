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
	"errors"
	"fmt"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/executors"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/system_tables"
	"net"
	"sync"

	bigtableModule "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/bigtable"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	schemaMapping "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/metadata"
	otelgo "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/otel"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/third_party/datastax/proxycore"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/translators"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	lru "github.com/hashicorp/golang-lru"
	"go.uber.org/zap"
)

var ErrProxyClosed = errors.New("proxy closed")
var ErrProxyAlreadyConnected = errors.New("proxy already connected")
var ErrProxyNotConnected = errors.New("proxy not connected")

const preparedIdSize = 16
const Query = "CqlQuery"
const QueryType = "QueryType"
const translatorErrorMessage = "Error occurred at translators"
const errorAtBigtable = "Error occurred at bigtable - "
const errorWhileDecoding = "Error while decoding bytes - "
const unhandledScenario = "Unhandled execution Scenario for prepared CqlQuery"
const errQueryNotPrepared = "query is not prepared"

const (
	traceNamespace = "cassandra.bigtable.proxy"
	handleQuery    = traceNamespace + "/HandleQuery"
	handleBatch    = traceNamespace + "/ExecuteBatch"
	handlePrepare  = traceNamespace + "/PrepareQuery"
	handleExecute  = traceNamespace + "/ExecuteQuery"
	handleRegister = traceNamespace + "/ExecuteRegister"
	handleOptions  = traceNamespace + "/ExecuteOptions"
)

// Events
const (
	executingBigtableRequestEvent       = "Executing Bigtable Mutation Request"
	executingBigtableSQLAPIRequestEvent = "Executing Bigtable SQL API Request"
	bigtableExecutionDoneEvent          = "bigtable Execution Done"
	gotBulkApplyResp                    = "Got the response for bulk apply"
	sendingBulkApplyMutation            = "Sending Mutation For Bulk Apply"
)

type Proxy struct {
	ctx                context.Context
	config             *types.ProxyInstanceConfig
	logger             *zap.Logger
	cluster            *proxycore.Cluster
	sessions           [primitive.ProtocolVersionDse2 + 1]sync.Map // Cache sessions per protocol version
	mu                 sync.Mutex
	isConnected        bool
	isClosing          bool
	clients            map[*client]struct{}
	listeners          map[*net.Listener]struct{}
	eventClients       sync.Map
	preparedQueryCache proxycore.PreparedCache[types.IPreparedQuery]
	systemLocalValues  map[string]message.Column
	closed             chan struct{}
	localNode          *node
	nodes              []*node
	clientManager      *types.BigtableClientManager
	systemTableManager *system_tables.SystemTableManager
	metadataStore      *schemaMapping.MetadataStore
	bigtableClient     *bigtableModule.BigtableAdapter
	translator         *translators.TranslatorManager
	executor           *executors.QueryExecutorManager
	otelInst           *otelgo.OpenTelemetry
	otelShutdown       func(context.Context) error
}

type node struct {
	addr   *net.IPAddr
	dc     string
	tokens []string
}

func (p *Proxy) OnEvent(event proxycore.Event) {
	switch evt := event.(type) {
	case *proxycore.SchemaChangeEvent:
		p.logger.Debug("Schema change event detected", zap.String("SchemaChangeEvent", evt.Message.String()))
	}
}

func NewProxy(ctx context.Context, logger *zap.Logger, config *types.ProxyInstanceConfig) (*Proxy, error) {
	clientManager, err := types.CreateBigtableClientManager(ctx, config)
	if err != nil {
		return nil, err
	}

	metadataStore := schemaMapping.NewMetadataStore(logger, clientManager, config.BigtableConfig)
	err = metadataStore.Initialize(ctx)
	if err != nil {
		return nil, err
	}

	// Enable OpenTelemetry traces by setting environment variable GOOGLE_API_GO_EXPERIMENTAL_TELEMETRY_PLATFORM_TRACING to the case-insensitive value "opentelemetry" before loading the client library.
	otelConfig := &otelgo.OTelConfig{}

	var shutdownOTel func(context.Context) error
	var otelInst *otelgo.OpenTelemetry
	// Initialize OpenTelemetry
	if config.OtelConfig.Enabled {
		otelConfig = &otelgo.OTelConfig{
			TracerEndpoint:     config.OtelConfig.Traces.Endpoint,
			ProjectId:          config.OtelConfig.Traces.ProjectId,
			MetricEndpoint:     config.OtelConfig.Metrics.Endpoint,
			ServiceName:        config.OtelConfig.ServiceName,
			OTELEnabled:        config.OtelConfig.Enabled,
			TraceSampleRatio:   config.OtelConfig.Traces.SamplingRatio,
			HealthCheckEnabled: config.OtelConfig.HealthCheck.Enabled,
			HealthCheckEp:      config.OtelConfig.HealthCheck.Endpoint,
			ServiceVersion:     config.Options.ProtocolVersion.String(),
		}
	} else {
		otelConfig = &otelgo.OTelConfig{OTELEnabled: false}
	}
	otelInst, shutdownOTel, err = otelgo.NewOpenTelemetry(ctx, otelConfig, logger)
	if err != nil {
		logger.Error("Failed to enable the OTEL: " + err.Error())
		return nil, err
	}

	bigtableClient := bigtableModule.NewBigtableClient(clientManager, logger, config.BigtableConfig, metadataStore)

	translator := translators.NewTranslatorManager(logger, metadataStore.Schemas(), config.BigtableConfig, otelInst)

	systemTables := system_tables.NewSystemTableManager(metadataStore, logger)

	proxy := &Proxy{
		ctx:                ctx,
		config:             config,
		logger:             logger,
		clients:            make(map[*client]struct{}),
		listeners:          make(map[*net.Listener]struct{}),
		closed:             make(chan struct{}),
		clientManager:      clientManager,
		systemTableManager: systemTables,
		metadataStore:      metadataStore,
		bigtableClient:     bigtableClient,
		translator:         translator,
		executor:           executors.NewQueryExecutorManager(logger, metadataStore.Schemas(), bigtableClient, systemTables.Db(), otelInst),
		otelInst:           otelInst,
		otelShutdown:       shutdownOTel,
	}

	err = systemTables.Initialize(proxy)
	if err != nil {
		logger.Error("Failed to initialize system table manager: " + err.Error())
		return nil, err
	}

	return proxy, nil
}

func (p *Proxy) Connect() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.isConnected {
		return ErrProxyAlreadyConnected
	}

	var err error
	const cacheSize = 1e8 / 256 // ~100MB with an average query size of 256 bytes
	p.preparedQueryCache, err = NewDefaultPreparedCache[types.IPreparedQuery](cacheSize)
	if err != nil {
		return fmt.Errorf("unable to create cache: %w", err)
	}

	//  connecting to cassandra cluster
	p.cluster, err = proxycore.ConnectCluster(p.ctx, proxycore.ClusterConfig{
		Version: p.config.Options.ProtocolVersion,
		Logger:  p.logger,
	})

	if err != nil {
		return fmt.Errorf("unable to connect to cluster %w", err)
	}

	err = p.buildNodes()
	if err != nil {
		return fmt.Errorf("unable to build node information: %w", err)
	}

	// Create cassandra session
	sess, err := proxycore.ConnectSession(p.ctx, proxycore.SessionConfig{
		Version: p.cluster.NegotiatedVersion,
		Logger:  p.logger,
	})

	if err != nil {
		return fmt.Errorf("unable to connect session: %w", err)
	}

	p.sessions[p.cluster.NegotiatedVersion].Store("", sess)

	err = p.systemTableManager.ReloadSystemTables()
	if err != nil {
		p.logger.Error("failed to update system tables", zap.Error(err))
	}

	p.isConnected = true
	return nil
}

// Serve the proxy using the specified listener. It can be called multiple times with different listeners allowing
// them to share the same backend clusters.
func (p *Proxy) Serve(l net.Listener) (err error) {
	l = &closeOnceListener{Listener: l}
	defer l.Close()

	if err = p.addListener(&l); err != nil {
		return err
	}
	defer p.removeListener(&l)

	for {
		conn, err := l.Accept()
		if err != nil {
			select {
			case <-p.closed:
				return ErrProxyClosed
			default:
				return err
			}
		}
		p.handle(conn)
	}
}

func (p *Proxy) GetSystemTableConfig() system_tables.SystemTableConfig {
	var peers []system_tables.PeerConfig
	for _, n := range p.nodes {
		peers = append(peers, system_tables.PeerConfig{
			Addr:   n.addr.String(),
			Dc:     n.dc,
			Tokens: n.tokens,
		})
	}

	systemTableConfig := system_tables.SystemTableConfig{
		RpcAddress:            p.config.RPCAddr,
		Datacenter:            p.config.DC,
		ReleaseVersion:        p.config.Options.ReleaseVersion,
		Partitioner:           p.config.Options.Partitioner,
		CqlVersion:            p.config.Options.CQLVersion,
		NativeProtocolVersion: p.config.Options.ProtocolVersion.String(),
		Peers:                 peers,
	}
	return systemTableConfig
}

func (p *Proxy) addListener(l *net.Listener) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.isClosing {
		return ErrProxyClosed
	}
	if !p.isConnected {
		return ErrProxyNotConnected
	}
	p.listeners[l] = struct{}{}
	return nil
}

func (p *Proxy) removeListener(l *net.Listener) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.listeners, l)
}

func (p *Proxy) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	select {
	case <-p.closed:
	default:
		close(p.closed)
	}
	var err error
	for l := range p.listeners {
		if closeErr := (*l).Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}
	for cl := range p.clients {
		_ = cl.conn.Close()
		p.eventClients.Delete(cl)
		delete(p.clients, cl)
	}

	p.clientManager.Close()
	if p.otelShutdown != nil {
		err = p.otelShutdown(p.ctx)
	}
	return err
}

func (p *Proxy) Ready() bool {
	return true
}

type UCred struct {
	Pid int32
	Uid uint32
}

func getUdsPeerCredentials(conn *net.UnixConn) (UCred, error) {
	return getUdsPeerCredentialsOS(conn)
}

func (p *Proxy) handle(conn net.Conn) {
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		if err := tcpConn.SetKeepAlive(false); err != nil {
			p.logger.Warn("failed to disable keepalive on connection", zap.Error(err))
		}
		if err := tcpConn.SetNoDelay(true); err != nil {
			p.logger.Warn("failed to set TCP_NODELAY on connection", zap.Error(err))
		}
	}

	if unixConn, ok := conn.(*net.UnixConn); ok {
		UCred, err := getUdsPeerCredentials(unixConn)
		if err != nil || p.config.Options.ClientPid != UCred.Pid || p.config.Options.ClientUid != UCred.Uid {
			_ = conn.Close()
			p.logger.Error("failed to authenticate connection")
		}
	}

	cl := &client{
		ctx:   p.ctx,
		proxy: p,
	}
	cl.sender = cl
	p.addClient(cl)
	cl.conn = proxycore.NewConn(conn, cl)
	cl.conn.Start()
}

func (p *Proxy) buildNodes() (err error) {
	localDC := p.config.DC
	if len(localDC) == 0 {
		localDC = p.cluster.Info.LocalDC
		p.logger.Info("no local DC configured using DC from the first successful contact point",
			zap.String("dc", localDC))
	}

	p.localNode = &node{
		dc: localDC,
	}

	return nil
}

func (p *Proxy) addClient(cl *client) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.clients[cl] = struct{}{}
}

func (p *Proxy) registerForEvents(cl *client) {
	p.eventClients.Store(cl, struct{}{})
}

func (p *Proxy) removeClient(cl *client) {
	p.eventClients.Delete(cl)

	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.clients, cl)

}

// NewDefaultPreparedCache creates a new default prepared cache capping the max item capacity to `size`.
func NewDefaultPreparedCache[T any](size int) (proxycore.PreparedCache[T], error) {
	cache, err := lru.New(size)
	if err != nil {
		return nil, err
	}
	return &defaultPreparedCache[T]{cache}, nil
}

type defaultPreparedCache[T any] struct {
	cache *lru.Cache
}

func (d defaultPreparedCache[T]) Store(id [16]byte, entry T) {
	d.cache.Add(id, entry)
}

func (d defaultPreparedCache[T]) Load(id [16]byte) (entry T, ok bool) {
	if val, ok := d.cache.Get(id); ok {
		return val.(T), true
	}
	return *new(T), false
}

func preparedIdKey(bytes []byte) [preparedIdSize]byte {
	var buf [preparedIdSize]byte
	copy(buf[:], bytes)
	return buf
}

// Wrap the listener so that if it's closed in the serve loop it doesn't race with proxy Close()
type closeOnceListener struct {
	net.Listener
	once     sync.Once
	closeErr error
}

func (oc *closeOnceListener) Close() error {
	oc.once.Do(oc.close)
	return oc.closeErr
}

func (oc *closeOnceListener) close() { oc.closeErr = oc.Listener.Close() }
