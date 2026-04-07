package executors

import (
	"context"
	"fmt"
	bigtableModule "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/bigtable"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/mem_table"
	schemaMapping "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/metadata"
	otelgo "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/otel"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"go.uber.org/zap"
	"strings"
)

type IQueryExecutor interface {
	CanRun(q types.IExecutableQuery) bool
	Execute(ctx context.Context, c types.ICassandraClient, q types.IExecutableQuery) (message.Message, error)
}

type QueryExecutorManager struct {
	logger    *zap.Logger
	executors []IQueryExecutor
	otelInst  *otelgo.OpenTelemetry
}

func NewQueryExecutorManager(logger *zap.Logger, s *schemaMapping.SchemaMetadata, bt *bigtableModule.BigtableAdapter, systemTables *mem_table.InMemEngine, otelInst *otelgo.OpenTelemetry) *QueryExecutorManager {
	return &QueryExecutorManager{
		logger: logger,
		executors: []IQueryExecutor{
			newDescribeExecutor(s),
			newUseExecutor(s),
			newSelectSystemTableExecutor(s, systemTables),
			newBigtableExecutor(bt),
		},
		otelInst: otelInst,
	}
}

func (m *QueryExecutorManager) getExecutor(q types.IExecutableQuery) (IQueryExecutor, error) {
	for _, e := range m.executors {
		if e.CanRun(q) {
			return e, nil
		}
	}
	return nil, fmt.Errorf("no executor found for query %s on keyspace %s", strings.ToUpper(q.QueryType().String()), q.Keyspace())
}

func (m *QueryExecutorManager) Execute(ctx context.Context, client types.ICassandraClient, q types.IExecutableQuery) (message.Message, error) {
	executor, err := m.getExecutor(q)
	if err != nil {
		return nil, err
	}

	otelCtx, childSpan := m.otelInst.StartSpan(ctx, "execute", nil)
	defer childSpan.End()
	msg, err := executor.Execute(otelCtx, client, q)
	childSpan.RecordError(err)
	return msg, err
}
