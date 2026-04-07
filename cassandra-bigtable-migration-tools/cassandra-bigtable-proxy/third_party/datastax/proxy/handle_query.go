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
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	otelgo "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/otel"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/parser"
	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"time"
)

func (c *client) handleQuery(ctx context.Context, raw *frame.RawFrame, msg *partialQuery) (message.Message, types.QueryType, error) {
	startTime := time.Now()
	c.proxy.logger.Debug("handling query", zap.String("encodedQuery", msg.query), zap.Int16("stream", raw.Header.StreamId))

	var otelErr error
	queryType := types.QueryTypeUnknown.String()
	defer func() {
		c.proxy.otelInst.RecordMetrics(ctx, handleQuery, startTime, queryType, c.sessionKeyspace, otelErr)
	}()

	p := parser.GetParser(msg.query)
	qt, err := parseQueryType(p)
	if err != nil {
		return &message.Invalid{ErrorMessage: err.Error()}, types.QueryTypeUnknown, err
	}
	queryType = qt.String()
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.SetAttributes(attribute.String(QueryType, queryType))
	}

	rawQuery := types.NewRawQuery(raw.Header, c.sessionKeyspace, msg.query, p, qt)

	query, err := c.prepareQuery(ctx, rawQuery)
	if err != nil {
		return &message.ServerError{ErrorMessage: err.Error()}, qt, err
	}

	values := types.NewQueryParameterValues(query.Parameters(), time.Now())
	executableQuery, err := c.proxy.translator.BindQueryParameters(query, values, raw.Header.Version)
	if err != nil {
		return &message.ConfigError{ErrorMessage: err.Error()}, qt, err
	}

	otelgo.AddAnnotation(ctx, executingBigtableSQLAPIRequestEvent)
	selectResult, err := c.proxy.executor.Execute(ctx, c, executableQuery)
	otelgo.AddAnnotation(ctx, bigtableExecutionDoneEvent)

	if err != nil {
		return nil, qt, err
	}

	if rawQuery.QueryType().IsDDLType() {
		c.handlePostDDLEvent(query.QueryType(), query.Keyspace(), query.Table())
	}

	return selectResult, qt, nil
}
