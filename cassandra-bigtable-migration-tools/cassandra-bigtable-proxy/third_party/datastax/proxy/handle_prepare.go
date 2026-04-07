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
	"crypto/md5"
	"fmt"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/parser"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/responsehandler"
	cql "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/third_party/cqlparser"
	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"go.uber.org/zap"
	"time"
)

func (c *client) handlePrepare(ctx context.Context, raw *frame.RawFrame, msg *message.Prepare) (message.Message, types.QueryType, error) {
	startTime := time.Now()

	var otelErr error
	queryType := types.QueryTypeUnknown.String()
	defer func() {
		c.proxy.otelInst.RecordMetrics(ctx, handlePrepare, startTime, queryType, c.sessionKeyspace, otelErr)
	}()

	id := c.getQueryId(msg)
	if preparedQuery, found := c.proxy.preparedQueryCache.Load(id); found {
		return responsehandler.BuildPreparedResultResponse(id, preparedQuery), preparedQuery.QueryType(), nil
	}

	c.proxy.logger.Debug("preparing query", zap.String(Query, msg.Query), zap.Int16("stream", raw.Header.StreamId))

	keyspace := c.sessionKeyspace
	if len(msg.Keyspace) != 0 {
		keyspace = types.Keyspace(msg.Keyspace)
	}

	p := parser.GetParser(msg.Query)
	qt, err := parseQueryType(p)
	if err != nil {
		return &message.Invalid{}, qt, err
	}

	rawQuery := types.NewRawQuery(raw.Header, keyspace, msg.Query, p, qt)
	return c.handleServerPreparedQuery(ctx, rawQuery, id)
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
// Returns: error if any error occurs during preparation
func (c *client) handleServerPreparedQuery(ctx context.Context, query *types.RawQuery, id [16]byte) (message.Message, types.QueryType, error) {
	preparedQuery, err := c.prepareQuery(ctx, query)
	if err != nil {
		return &message.Invalid{ErrorMessage: err.Error()}, query.QueryType(), err
	}

	response := responsehandler.BuildPreparedResultResponse(id, preparedQuery)

	// update cache
	c.proxy.preparedQueryCache.Store(id, preparedQuery)

	return response, query.QueryType(), nil
}

func (c *client) prepareQuery(ctx context.Context, query *types.RawQuery) (types.IPreparedQuery, error) {
	preparedQuery, err := c.proxy.translator.TranslateQuery(ctx, query, c.sessionKeyspace)
	if err != nil {
		return nil, err
	}

	btPreparedQuery, err := c.proxy.bigtableClient.PrepareStatement(ctx, preparedQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare bigtable statement `%s`: %w", preparedQuery.BigtableQuery(), err)
	}
	preparedQuery.SetBigtablePreparedQuery(btPreparedQuery)

	return preparedQuery, err
}
