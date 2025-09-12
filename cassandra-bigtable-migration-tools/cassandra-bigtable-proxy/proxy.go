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

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/third_party/datastax/proxy"
)

func main() {
	ctx, cancel := signalContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	err := proxy.Run(ctx, os.Args[1:])
	if err != nil {
		println(fmt.Sprintf("error: %s", err.Error()))
		os.Exit(1)
		return
	}
	os.Exit(0)
}

// signalContext is a simplified version of `signal.NotifyContext()` for  golang 1.15 and earlier
func signalContext(parent context.Context, sig ...os.Signal) (context.Context, func()) {
	ctx, cancel := context.WithCancel(parent)
	ch := make(chan os.Signal)
	signal.Notify(ch, sig...)
	if ctx.Err() == nil {
		go func() {
			select {
			case <-ch:
				cancel()
			case <-ctx.Done():
			}
		}()
	}
	return ctx, func() {
		cancel()
		signal.Stop(ch)
	}
}
