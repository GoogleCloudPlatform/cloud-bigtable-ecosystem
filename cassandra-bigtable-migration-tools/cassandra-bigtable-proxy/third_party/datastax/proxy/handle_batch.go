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
	bigtableModule "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/bigtable"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	otelgo "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/otel"
	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"go.opentelemetry.io/otel/attribute"
	"strings"
	"time"
)

// handle batch queries
func (c *client) handleBatch(ctx context.Context, raw *frame.RawFrame, msg *partialBatch) (message.Message, error) {
	startTime := time.Now()
	otelCtx, span := c.proxy.otelInst.StartSpan(c.proxy.ctx, handleBatch, []attribute.KeyValue{
		attribute.Int("Batch Size", len(msg.queryOrIds)),
	})
	defer c.proxy.otelInst.EndSpan(span)
	var otelErr error
	defer c.proxy.otelInst.RecordMetrics(otelCtx, handleBatch, startTime, handleBatch, c.sessionKeyspace, otelErr)
	bulkMutations, keyspace, err := c.bindBulkOperations(msg, raw.Header.Version)
	if err != nil {
		return &message.ConfigError{}, err
	}
	otelgo.AddAnnotation(otelCtx, sendingBulkApplyMutation)
	var errs []string
	for tableName, mutations := range bulkMutations.Mutations() {
		res, err := c.proxy.bigtableClient.ApplyBulkMutation(otelCtx, keyspace, tableName, mutations)
		if err != nil {
			c.proxy.otelInst.RecordError(span, err)
			errs = append(errs, err.Error())
		} else if res.FailedRows != "" {
			err = fmt.Errorf("failed rows for table %s: %s", tableName, res.FailedRows)
			c.proxy.otelInst.RecordError(span, err)
			errs = append(errs, res.FailedRows)
		}
	}
	otelgo.AddAnnotation(otelCtx, gotBulkApplyResp)
	if len(errs) > 0 {
		return nil, errors.New(strings.Join(errs, "\n"))
	}

	return &message.VoidResult{}, nil
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
