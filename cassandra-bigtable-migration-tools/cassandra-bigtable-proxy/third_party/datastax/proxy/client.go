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
	"errors"
	"fmt"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/third_party/datastax/proxycore"
	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.uber.org/zap"
	"io"
	"time"
)

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
	startTime := time.Now()
	otelCtx, span := c.proxy.otelInst.StartSpan(c.proxy.ctx, "Receive", nil)
	defer c.proxy.otelInst.EndSpan(span)

	raw, err := codec.DecodeRawFrame(reader)
	if err != nil {
		if !errors.Is(err, io.EOF) {
			c.proxy.logger.Error("unable to decode frame", zap.Error(err))
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	if raw.Header.Version > c.proxy.config.Options.MaxProtocolVersion || raw.Header.Version < primitive.ProtocolVersion3 {
		c.sender.Send(raw.Header, &message.ProtocolError{
			ErrorMessage: fmt.Sprintf("Invalid or unsupported protocol version %d", raw.Header.Version),
		})
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil
	}

	body, err := codec.DecodeBody(raw.Header, bytes.NewReader(raw.Body))
	if err != nil {
		c.proxy.logger.Error("unable to decode body", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	var response message.Message
	queryType := types.QueryTypeUnknown
	switch msg := body.Message.(type) {
	case *message.Options:
		span.SetName(handleOptions)
		response, err = c.handleOptions(otelCtx, raw, msg)
	case *message.Startup:
		span.SetName("Startup")
		response, err = c.handleStartup(otelCtx, raw, msg)
	case *message.Register:
		span.SetName(handleRegister)
		response, err = c.handleRegister(otelCtx, raw, msg)
	case *message.Prepare:
		span.SetName(handlePrepare)
		response, queryType, err = c.handlePrepare(otelCtx, raw, msg)
	case *partialExecute:
		span.SetName(handleExecute)
		response, queryType, err = c.handleExecute(otelCtx, raw, msg)
	case *partialQuery:
		span.SetName(handleQuery)
		response, queryType, err = c.handleQuery(otelCtx, raw, msg)
	case *partialBatch:
		span.SetName(handleBatch)
		response, err = c.handleBatch(otelCtx, raw, msg)
	default:
		response = &message.ServerError{ErrorMessage: "unsupported operation"}
		err = errors.New("unsupported operation")
	}

	if queryType != types.QueryTypeUnknown {
		span.SetAttributes(attribute.String(QueryType, queryType.String()))
	}

	c.proxy.otelInst.RecordMetrics(otelCtx, handleExecute, startTime, queryType.String(), c.sessionKeyspace, err)

	if response == nil {
		c.proxy.logger.Error("nil response")
		// fix the response
		response = &message.ServerError{ErrorMessage: "unhandled response"}
	}

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		c.sender.Send(raw.Header, response)
		return nil
	}

	span.SetStatus(codes.Ok, "")
	c.sender.Send(raw.Header, response)
	return nil
}

func (c *client) Send(hdr *frame.Header, msg message.Message) {
	_ = c.conn.Write(proxycore.SenderFunc(func(writer io.Writer) error {
		return codec.EncodeFrame(frame.NewFrame(hdr.Version, hdr.StreamId, msg), writer)
	}))
}

func (c *client) Closing(_ error) {
	c.proxy.removeClient(c)
}

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
