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
	"bytes"
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/executors"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/parser"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/responsehandler"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/system_tables"
	cql "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/third_party/cqlparser"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	bigtableModule "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/bigtable"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	schemaMapping "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/metadata"
	otelgo "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/otel"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/third_party/datastax/proxycore"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/translators"
	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	lru "github.com/hashicorp/golang-lru"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

var ErrProxyClosed = errors.New("proxy closed")
var ErrProxyAlreadyConnected = errors.New("proxy already connected")
var ErrProxyNotConnected = errors.New("proxy not connected")

const preparedIdSize = 16
const Query = "CqlQuery"
const translatorErrorMessage = "Error occurred at translators"
const errorAtBigtable = "Error occurred at bigtable - "
const errorWhileDecoding = "Error while decoding bytes - "
const unhandledScenario = "Unhandled execution Scenario for prepared CqlQuery"
const errQueryNotPrepared = "query is not prepared"
const (
	handleQuery = "handleQuery"
	handleBatch = "Batch"
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
	queryMetadataCache proxycore.PreparedCache[*message.PreparedResult]
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

	bigtableClient := bigtableModule.NewBigtableClient(clientManager, logger, config.BigtableConfig, metadataStore)

	translator := translators.NewTranslatorManager(logger, metadataStore.Schemas(), config.BigtableConfig)

	// Enable OpenTelemetry traces by setting environment variable GOOGLE_API_GO_EXPERIMENTAL_TELEMETRY_PLATFORM_TRACING to the case-insensitive value "opentelemetry" before loading the client library.
	otelConfig := &otelgo.OTelConfig{}
	otelInst := &otelgo.OpenTelemetry{Config: &otelgo.OTelConfig{OTELEnabled: false}}

	var shutdownOTel func(context.Context) error
	// Initialize OpenTelemetry
	if config.OtelConfig.Enabled {
		otelConfig = &otelgo.OTelConfig{
			TracerEndpoint:     config.OtelConfig.Traces.Endpoint,
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
		executor:           executors.NewQueryExecutorManager(logger, metadataStore.Schemas(), bigtableClient, systemTables.Db()),
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
	p.queryMetadataCache, err = NewDefaultPreparedCache[*message.PreparedResult](cacheSize)
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

type Sender interface {
	Send(hdr *frame.Header, msg message.Message)
}

type client struct {
	sessionKeyspace types.Keyspace
	ctx             context.Context
	proxy           *Proxy
	conn            *proxycore.Conn
	sender          Sender
}

func (c *client) SetSessionKeyspace(k types.Keyspace) {
	c.sessionKeyspace = k
}

func (c *client) Receive(reader io.Reader) error {
	raw, err := codec.DecodeRawFrame(reader)
	if err != nil {
		if !errors.Is(err, io.EOF) {
			c.proxy.logger.Error("unable to decode frame", zap.Error(err))
		}
		return err
	}

	if raw.Header.Version > c.proxy.config.Options.MaxProtocolVersion || raw.Header.Version < primitive.ProtocolVersion3 {
		c.sender.Send(raw.Header, &message.ProtocolError{
			ErrorMessage: fmt.Sprintf("Invalid or unsupported protocol version %d", raw.Header.Version),
		})
		return nil
	}

	body, err := codec.DecodeBody(raw.Header, bytes.NewReader(raw.Body))
	if err != nil {
		c.proxy.logger.Error("unable to decode body", zap.Error(err))
		return err
	}

	switch msg := body.Message.(type) {
	case *message.Options:
		// CC - responding with status READY
		c.sender.Send(raw.Header, &message.Supported{Options: map[string][]string{
			"CQL_VERSION": {c.proxy.config.Options.CQLVersion},
			"COMPRESSION": {},
		}})
	case *message.Startup:
		// CC -  register for Event types and respond READY
		c.sender.Send(raw.Header, &message.Ready{})
	case *message.Register:
		c.proxy.logger.Info("Client registered for events", zap.Any("event_types", msg.EventTypes))
		for _, t := range msg.EventTypes {
			if t == primitive.EventTypeSchemaChange {
				c.proxy.registerForEvents(c)
			}
		}
		c.sender.Send(raw.Header, &message.Ready{})
	case *message.Prepare:
		c.handlePrepare(raw, msg)
	case *partialExecute:
		c.handleExecute(raw, msg)
	case *partialQuery:
		c.handleQuery(raw, msg)
	case *partialBatch:
		c.handleBatch(raw, msg)
	default:
		c.sender.Send(raw.Header, &message.ProtocolError{ErrorMessage: "Unsupported operation"})
	}
	return nil
}

func (c *client) handlePrepare(raw *frame.RawFrame, msg *message.Prepare) {
	id := c.getQueryId(msg)
	if response, found := c.proxy.queryMetadataCache.Load(id); found {
		c.sender.Send(raw.Header, response)
		return
	}

	c.proxy.logger.Debug("preparing query", zap.String(Query, msg.Query), zap.Int16("stream", raw.Header.StreamId))

	keyspace := c.sessionKeyspace
	if len(msg.Keyspace) != 0 {
		keyspace = types.Keyspace(msg.Keyspace)
	}

	p := parser.NewParser(msg.Query)
	qt, err := parseQueryType(p)
	if err != nil {
		c.proxy.logger.Error("error parsing query to see if it's handled", zap.String(Query, msg.Query), zap.Error(err))
		c.sender.Send(raw.Header, &message.Invalid{ErrorMessage: err.Error()})
		return
	}
	rawQuery := types.NewRawQuery(raw.Header, keyspace, msg.Query, p, qt)

	c.handleServerPreparedQuery(rawQuery, id)
}

func parseQueryType(p *parser.ProxyCqlParser) (types.QueryType, error) {
	tok := p.GetFirstToken()
	t := tok.GetTokenType()
	switch t {
	case cql.CqlLexerK_SELECT:
		return types.QueryTypeSelect, nil
	case cql.CqlLexerK_INSERT:
		return types.QueryTypeInsert, nil
	case cql.CqlLexerK_UPDATE:
		return types.QueryTypeUpdate, nil
	case cql.CqlLexerK_DELETE:
		return types.QueryTypeDelete, nil
	case cql.CqlLexerK_CREATE:
		return types.QueryTypeCreate, nil
	case cql.CqlLexerK_ALTER:
		return types.QueryTypeAlter, nil
	case cql.CqlLexerK_DROP:
		return types.QueryTypeDrop, nil
	case cql.CqlLexerK_TRUNCATE:
		return types.QueryTypeTruncate, nil
	case cql.CqlLexerK_USE:
		return types.QueryTypeUse, nil
	case cql.CqlLexerK_DESCRIBE, cql.CqlLexerK_DESC:
		return types.QueryTypeDescribe, nil
	default:
		return types.QueryTypeUnknown, fmt.Errorf("unsupported query type: %s", tok.String())
	}
}

func (c *client) getQueryId(msg *message.Prepare) [16]byte {
	// Generating unique prepared query_id
	return md5.Sum([]byte(msg.Query + string(c.sessionKeyspace)))
}

// handleServerPreparedQuery handle prepared query that was supposed to run on cassandra server
// This method will keep track of prepared query in a map and send hashed query_id with result
// metadata and variable column metadata to the client
//
// Parameters:
//   - raw: *frame.RawFrame
//   - msg: *message.Prepare
//
// Returns: nil
func (c *client) handleServerPreparedQuery(query *types.RawQuery, id [16]byte) {
	preparedQuery, err := c.prepareQuery(query)
	if err != nil {
		c.proxy.logger.Error(translatorErrorMessage, zap.String(Query, query.RawCql()), zap.Error(err))
		c.sender.Send(query.Header(), &message.Invalid{ErrorMessage: err.Error()})
		return
	}

	response, err := responsehandler.BuildPreparedResultResponse(id, preparedQuery)

	// update caches
	c.proxy.preparedQueryCache.Store(id, preparedQuery)
	c.proxy.queryMetadataCache.Store(id, response)

	c.sender.Send(query.Header(), response)
}

func (c *client) prepareQuery(query *types.RawQuery) (types.IPreparedQuery, error) {
	preparedQuery, err := c.proxy.translator.TranslateQuery(query, c.sessionKeyspace)
	if err != nil {
		return nil, err
	}

	btPreparedQuery, err := c.proxy.bigtableClient.PrepareStatement(c.ctx, preparedQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare bigtable statement `%s`: %w", preparedQuery.BigtableQuery(), err)
	}
	preparedQuery.SetBigtablePreparedQuery(btPreparedQuery)

	return preparedQuery, err
}

// handleExecute for prepared query
func (c *client) handleExecute(raw *frame.RawFrame, msg *partialExecute) {
	ctx := context.Background()
	id := preparedIdKey(msg.queryId)

	preparedStmt, ok := c.proxy.preparedQueryCache.Load(id)
	if !ok {
		c.proxy.logger.Error(errQueryNotPrepared)
		c.sender.Send(raw.Header, &message.ServerError{ErrorMessage: errQueryNotPrepared})
		return
	}

	boundQuery, err := c.proxy.translator.BindQuery(preparedStmt, msg.PositionalValues, msg.NamedValues, raw.Header.Version)
	if err != nil {
		c.proxy.logger.Error(errorWhileDecoding, zap.String(Query, preparedStmt.CqlQuery()), zap.Error(err))
		c.sender.Send(raw.Header, &message.ConfigError{ErrorMessage: err.Error()})
		return
	}

	results, err := c.proxy.executor.Execute(ctx, c, boundQuery)
	if err != nil {
		c.proxy.logger.Error(errorAtBigtable, zap.String(Query, preparedStmt.CqlQuery()), zap.String(Query, preparedStmt.BigtableQuery()), zap.Error(err))
		c.sender.Send(raw.Header, &message.ConfigError{ErrorMessage: err.Error()})
		return
	}
	if preparedStmt.QueryType().IsDDLType() {
		c.handlePostDDLEvent(preparedStmt.QueryType(), preparedStmt.Keyspace(), preparedStmt.Table())
	}
	c.sender.Send(raw.Header, results)
}

// handle batch queries
func (c *client) handleBatch(raw *frame.RawFrame, msg *partialBatch) {
	startTime := time.Now()
	otelCtx, span := c.proxy.otelInst.StartSpan(c.proxy.ctx, handleBatch, []attribute.KeyValue{
		attribute.Int("Batch Size", len(msg.queryOrIds)),
	})
	defer c.proxy.otelInst.EndSpan(span)
	var otelErr error
	defer c.proxy.otelInst.RecordMetrics(otelCtx, handleBatch, startTime, handleBatch, c.sessionKeyspace, otelErr)
	bulkMutations, keyspace, err := c.bindBulkOperations(msg, raw.Header.Version)
	if err != nil {
		c.proxy.logger.Error("Error preparing batch query metadata", zap.Error(err))
		c.sender.Send(raw.Header, &message.ConfigError{ErrorMessage: err.Error()})
		otelErr = err
		c.proxy.otelInst.RecordError(span, otelErr)
		return
	}
	otelgo.AddAnnotation(otelCtx, sendingBulkApplyMutation)
	var errs []string
	for tableName, mutations := range bulkMutations.Mutations() {
		res, err := c.proxy.bigtableClient.ApplyBulkMutation(otelCtx, keyspace, tableName, mutations)
		if err != nil || res.FailedRows != "" {
			c.proxy.otelInst.RecordError(span, err)
			errs = append(errs, res.FailedRows)
		}
	}
	otelgo.AddAnnotation(otelCtx, gotBulkApplyResp)
	if len(errs) > 0 {
		c.sender.Send(raw.Header, &message.ServerError{ErrorMessage: strings.Join(errs, "\n")})
		return
	}
	c.sender.Send(raw.Header, &message.VoidResult{})
}

func (c *client) bindBulkOperations(msg *partialBatch, pv primitive.ProtocolVersion) (*bigtableModule.BigtableBulkMutation, types.Keyspace, error) {
	var keyspace types.Keyspace
	tableMutationsMap := bigtableModule.NewBigtableBulkMutation()
	for index, queryId := range msg.queryOrIds {
		queryOrId, ok := queryId.([]byte)
		if !ok {
			return nil, "", fmt.Errorf("batch query id malformed")
		}
		id := preparedIdKey(queryOrId)
		preparedStmt, ok := c.proxy.preparedQueryCache.Load(id)
		if !ok {
			return nil, "", fmt.Errorf("prepared query not found in cache")
		}

		if preparedStmt.Keyspace() != "" {
			keyspace = preparedStmt.Keyspace()
		}

		// note: we don't support batch named queries at this time
		executableQuery, err := c.proxy.translator.BindQuery(preparedStmt, msg.BatchPositionalValues[index], nil, pv)
		if err != nil {
			return nil, "", err
		}
		mutation, ok := executableQuery.AsBulkMutation()
		if !ok {
			return nil, "", fmt.Errorf("query type '%s' not compatible with bulk", executableQuery.QueryType().String())
		}
		tableMutationsMap.AddMutation(mutation)
	}
	if keyspace == "" {
		keyspace = c.sessionKeyspace
	}
	return tableMutationsMap, keyspace, nil
}

func (c *client) handleQuery(raw *frame.RawFrame, msg *partialQuery) {
	startTime := time.Now()
	c.proxy.logger.Debug("handling query", zap.String("encodedQuery", msg.query), zap.Int16("stream", raw.Header.StreamId))
	otelCtx, span := c.proxy.otelInst.StartSpan(c.proxy.ctx, handleQuery, []attribute.KeyValue{
		attribute.String("CqlQuery", msg.query),
	})

	p := parser.NewParser(msg.query)
	qt, err := parseQueryType(p)
	if err != nil {
		c.proxy.logger.Error("error parsing query to see if it's handled", zap.String(Query, msg.query), zap.Error(err))
		c.sender.Send(raw.Header, &message.Invalid{ErrorMessage: err.Error()})
		return
	}

	rawQuery := types.NewRawQuery(raw.Header, c.sessionKeyspace, msg.query, p, qt)
	defer c.proxy.otelInst.EndSpan(span)

	var otelErr error
	defer c.proxy.otelInst.RecordMetrics(otelCtx, handleQuery, startTime, rawQuery.QueryType().String(), c.sessionKeyspace, otelErr)

	query, err := c.prepareQuery(rawQuery)
	if err != nil {
		c.proxy.logger.Error(translatorErrorMessage, zap.String(Query, msg.query), zap.Error(err))
		c.sender.Send(raw.Header, &message.Invalid{ErrorMessage: err.Error()})
		return
	}
	values := types.NewQueryParameterValues(query.Parameters(), time.Now())
	executableQuery, err := c.proxy.translator.BindQueryParameters(query, values, raw.Header.Version)
	if err != nil {
		c.proxy.logger.Error(translatorErrorMessage, zap.String(Query, msg.query), zap.Error(err))
		c.sender.Send(raw.Header, &message.Invalid{ErrorMessage: err.Error()})
		return
	}

	otelgo.AddAnnotation(otelCtx, executingBigtableSQLAPIRequestEvent)
	selectResult, err := c.proxy.executor.Execute(otelCtx, c, executableQuery)
	otelgo.AddAnnotation(otelCtx, bigtableExecutionDoneEvent)

	if err != nil {
		c.proxy.logger.Error(errorAtBigtable, zap.String(Query, msg.query), zap.Error(err))
		otelErr = err
		c.proxy.otelInst.RecordError(span, otelErr)
		c.sender.Send(raw.Header, &message.Invalid{ErrorMessage: err.Error()})
		return
	}

	if rawQuery.QueryType().IsDDLType() {
		c.handlePostDDLEvent(query.QueryType(), query.Keyspace(), query.Table())
	}

	c.sender.Send(raw.Header, selectResult)
}

func (c *client) Send(hdr *frame.Header, msg message.Message) {
	_ = c.conn.Write(proxycore.SenderFunc(func(writer io.Writer) error {
		return codec.EncodeFrame(frame.NewFrame(hdr.Version, hdr.StreamId, msg), writer)
	}))
}

func (c *client) Closing(_ error) {
	c.proxy.removeClient(c)
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

// handleEvent handles events from the proxy core
// It sends the event message to all connected clients.
func (c *client) handleEvent(event proxycore.Event) {
	switch evt := event.(type) {
	case *proxycore.SchemaChangeEvent:
		c.sender.Send(&frame.Header{
			Version:  c.proxy.config.Options.ProtocolVersion,
			StreamId: -1, // -1 for events
			OpCode:   primitive.OpCodeEvent,
		}, evt.Message)
	}
}

// handlePostDDLEvent handles common operations after DDL statements (CREATE, ALTER, DROP)
func (c *client) handlePostDDLEvent(queryType types.QueryType, keyspace types.Keyspace, table types.TableName) {
	var changeType primitive.SchemaChangeType
	switch queryType {
	case types.QueryTypeCreate:
		changeType = primitive.SchemaChangeTypeCreated
	case types.QueryTypeAlter:
		changeType = primitive.SchemaChangeTypeUpdated
	case types.QueryTypeDrop:
		changeType = primitive.SchemaChangeTypeDropped
	default:
		c.proxy.logger.Warn("unhandled ddl event type", zap.String("queryType", queryType.String()))
		return
	}

	// SendEvent all clients of schema change
	event := &proxycore.SchemaChangeEvent{
		Message: &message.SchemaChangeEvent{
			ChangeType: changeType,
			Target:     primitive.SchemaChangeTargetTable,
			Keyspace:   string(keyspace),
			Object:     string(table),
		},
	}
	c.proxy.eventClients.Range(func(key, _ interface{}) bool {
		if client, ok := key.(*client); ok {
			client.handleEvent(event)
		}
		return true
	})
}
