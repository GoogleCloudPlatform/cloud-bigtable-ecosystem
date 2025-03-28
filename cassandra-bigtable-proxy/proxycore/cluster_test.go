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

package proxycore

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/stretchr/testify/require"
	"github.com/tj/assert"
	"go.uber.org/zap"
)

func TestConnectCluster(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := NewMockCluster(net.ParseIP("127.0.0.1"), 9042)

	err := c.Add(ctx, 1)
	require.NoError(t, err)

	err = c.Add(ctx, 2)
	require.NoError(t, err)

	err = c.Add(ctx, 3)
	require.NoError(t, err)

	_, err = ConnectCluster(ctx, ClusterConfig{
		Version:           primitive.ProtocolVersion4,
		Resolver:          NewResolver("127.0.0.1:9042"),
		ConnectTimeout:    10 * time.Second,
		Logger:            logger,
		HeartBeatInterval: 30 * time.Second,
		IdleTimeout:       60 * time.Second,
	})
	require.NoError(t, err)
}

func TestClusterListenerFunc(t *testing.T) {
	// Create a ClusterListenerFunc that collects received events.
	var receivedEvents []Event
	listener := ClusterListenerFunc(func(event Event) {
		receivedEvents = append(receivedEvents, event)
	})

	// Define some test events.
	addEvent := AddEvent{Host: &Host{DC: "datacenter1"}}
	removeEvent := RemoveEvent{Host: &Host{DC: "datacenter2"}}
	upEvent := UpEvent{Host: &Host{DC: "datacenter3"}}
	bootstrapEvent := BootstrapEvent{Hosts: []*Host{{DC: "datacenter4"}, {DC: "datacenter5"}}}
	schemaChangeEvent := SchemaChangeEvent{Message: &message.SchemaChangeEvent{}}

	// Dispatch events to the listener.
	listener.OnEvent(addEvent)
	listener.OnEvent(removeEvent)
	listener.OnEvent(upEvent)
	listener.OnEvent(bootstrapEvent)
	listener.OnEvent(schemaChangeEvent)

	// Assert that the received events match the dispatched events.
	expectedEvents := []Event{addEvent, removeEvent, upEvent, bootstrapEvent, schemaChangeEvent}
	assert.Equal(t, expectedEvents, receivedEvents)
}

func TestEventTypes(t *testing.T) {
	// Test that the isEvent method is implemented for each event type.
	assert.Panics(t, func() { AddEvent{}.isEvent() })
	assert.Panics(t, func() { RemoveEvent{}.isEvent() })
	assert.Panics(t, func() { UpEvent{}.isEvent() })
	assert.Panics(t, func() { BootstrapEvent{}.isEvent() })
	assert.Panics(t, func() { SchemaChangeEvent{}.isEvent() })
	assert.Panics(t, func() { ReconnectEvent{}.isEvent() })
}
