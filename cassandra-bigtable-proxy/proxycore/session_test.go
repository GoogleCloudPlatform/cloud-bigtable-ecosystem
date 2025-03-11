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

	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestConnectSession(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const supported = primitive.ProtocolVersion4

	c := NewMockCluster(net.ParseIP("127.0.0.0"), 9042)

	err := c.Add(ctx, 1)
	require.NoError(t, err)

	err = c.Add(ctx, 2)
	require.NoError(t, err)

	err = c.Add(ctx, 3)
	require.NoError(t, err)

	cluster, err := ConnectCluster(ctx, ClusterConfig{
		Version:           supported,
		Resolver:          NewResolver("127.0.0.1:9042"),
		RefreshWindow:     100 * time.Millisecond,
		ConnectTimeout:    10 * time.Second,
		Logger:            logger,
		HeartBeatInterval: 30 * time.Second,
		IdleTimeout:       60 * time.Second,
	})
	require.NoError(t, err)

	_, err = ConnectSession(ctx, cluster, SessionConfig{
		Version:           supported,
		ConnectTimeout:    10 * time.Second,
		HeartBeatInterval: 30 * time.Second,
		IdleTimeout:       60 * time.Second,
	})
	require.NoError(t, err)
}
