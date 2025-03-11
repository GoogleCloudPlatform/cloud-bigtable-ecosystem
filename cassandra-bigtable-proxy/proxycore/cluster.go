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
	"time"

	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"go.uber.org/zap"
)

const (
	DefaultRefreshWindow  = 10 * time.Second
	DefaultRefreshTimeout = 5 * time.Second
)

type Event interface {
	isEvent() // Marker method for the event interface
}

type AddEvent struct {
	Host *Host
}

func (a AddEvent) isEvent() {
	panic("do not call")
}

type RemoveEvent struct {
	Host *Host
}

func (r RemoveEvent) isEvent() {
	panic("do not call")
}

type UpEvent struct {
	Host *Host
}

func (a UpEvent) isEvent() {
	panic("do not call")
}

type BootstrapEvent struct {
	Hosts []*Host
}

func (b BootstrapEvent) isEvent() {
	panic("do not call")
}

type SchemaChangeEvent struct {
	Message *message.SchemaChangeEvent
}

func (s SchemaChangeEvent) isEvent() {
	panic("do not call")
}

type ReconnectEvent struct {
	Endpoint
}

func (r ReconnectEvent) isEvent() {
	panic("do not call")
}

type ClusterListener interface {
	OnEvent(event Event)
}

type ClusterListenerFunc func(event Event)

func (f ClusterListenerFunc) OnEvent(event Event) {
	f(event)
}

type ClusterConfig struct {
	Version           primitive.ProtocolVersion
	Auth              Authenticator
	Resolver          EndpointResolver
	RefreshWindow     time.Duration
	HeartBeatInterval time.Duration
	ConnectTimeout    time.Duration
	RefreshTimeout    time.Duration
	IdleTimeout       time.Duration
	Logger            *zap.Logger
}

type ClusterInfo struct {
	Partitioner    string
	ReleaseVersion string
	CQLVersion     string
	LocalDC        string
	DSEVersion     string
}

// Cluster defines a downstream cluster that is being proxied to.
type Cluster struct {
	ctx               context.Context
	config            ClusterConfig
	logger            *zap.Logger
	controlConn       *ClientConn
	hosts             []*Host
	currentHostIndex  int
	listeners         []ClusterListener
	addListener       chan ClusterListener
	events            chan *frame.Frame
	NegotiatedVersion primitive.ProtocolVersion
	Info              ClusterInfo
}

// ConnectCluster establishes control connections to each of the endpoints within a downstream cluster that is being proxied to.
func ConnectCluster(ctx context.Context, config ClusterConfig) (*Cluster, error) {
	c := &Cluster{
		ctx:              ctx,
		config:           config,
		logger:           GetOrCreateNopLogger(config.Logger),
		controlConn:      nil,
		hosts:            nil,
		currentHostIndex: 0,
		events:           make(chan *frame.Frame),
		addListener:      make(chan ClusterListener),
		listeners:        make([]ClusterListener, 0),
	}
	return c, nil
}
