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
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/message"
)

// handleExecute for prepared query
func (c *client) handleExecute(ctx context.Context, raw *frame.RawFrame, msg *partialExecute) (message.Message, types.QueryType, error) {
	id := preparedIdKey(msg.queryId)

	preparedStmt, ok := c.proxy.preparedQueryCache.Load(id)
	if !ok {
		return &message.ServerError{ErrorMessage: errQueryNotPrepared}, types.QueryTypeUnknown, errors.New(errQueryNotPrepared)
	}

	boundQuery, err := c.proxy.translator.BindQuery(preparedStmt, msg.PositionalValues, msg.NamedValues, raw.Header.Version)
	if err != nil {
		return &message.ConfigError{ErrorMessage: err.Error()}, preparedStmt.QueryType(), err
	}

	results, err := c.proxy.executor.Execute(ctx, c, boundQuery)
	if err != nil {
		return &message.ConfigError{ErrorMessage: err.Error()}, preparedStmt.QueryType(), err
	}
	if preparedStmt.QueryType().IsDDLType() {
		c.handlePostDDLEvent(preparedStmt.QueryType(), preparedStmt.Keyspace(), preparedStmt.Table())
	}
	return results, preparedStmt.QueryType(), nil
}
