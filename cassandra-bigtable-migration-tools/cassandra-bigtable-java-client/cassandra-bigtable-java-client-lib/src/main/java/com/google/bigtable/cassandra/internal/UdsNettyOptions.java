// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package com.google.bigtable.cassandra.internal;

import com.datastax.oss.driver.api.core.config.DefaultDriverOption;
import com.datastax.oss.driver.api.core.config.DriverExecutionProfile;
import com.datastax.oss.driver.internal.core.context.DefaultNettyOptions;
import com.datastax.oss.driver.internal.core.context.InternalDriverContext;
import io.netty.bootstrap.Bootstrap;
import io.netty.channel.ChannelOption;
import io.netty.channel.FixedRecvByteBufAllocator;
import io.netty.util.concurrent.DefaultThreadFactory;
import io.netty.util.concurrent.MultithreadEventExecutorGroup;
import java.time.Duration;

abstract class UdsNettyOptions extends DefaultNettyOptions {

  private final DriverExecutionProfile config;

  UdsNettyOptions(InternalDriverContext context) {
    super(context);
    this.config = context.getConfig().getDefaultProfile();
  }

  @Override
  public void afterBootstrapInitialized(Bootstrap bootstrap) {
    int sendBufferSize;
    if (this.config.isDefined(DefaultDriverOption.SOCKET_LINGER_INTERVAL)) {
      sendBufferSize = this.config.getInt(DefaultDriverOption.SOCKET_LINGER_INTERVAL);
      bootstrap.option(ChannelOption.SO_LINGER, sendBufferSize);
    }

    if (this.config.isDefined(DefaultDriverOption.SOCKET_RECEIVE_BUFFER_SIZE)) {
      sendBufferSize = this.config.getInt(DefaultDriverOption.SOCKET_RECEIVE_BUFFER_SIZE);
      bootstrap.option(ChannelOption.SO_RCVBUF, sendBufferSize)
          .option(ChannelOption.RCVBUF_ALLOCATOR, new FixedRecvByteBufAllocator(sendBufferSize));
    }

    if (this.config.isDefined(DefaultDriverOption.SOCKET_SEND_BUFFER_SIZE)) {
      sendBufferSize = this.config.getInt(DefaultDriverOption.SOCKET_SEND_BUFFER_SIZE);
      bootstrap.option(ChannelOption.SO_SNDBUF, sendBufferSize);
    }

    if (this.config.isDefined(DefaultDriverOption.CONNECTION_CONNECT_TIMEOUT)) {
      Duration connectTimeout = this.config.getDuration(DefaultDriverOption.CONNECTION_CONNECT_TIMEOUT);
      bootstrap.option(ChannelOption.CONNECT_TIMEOUT_MILLIS, Long.valueOf(connectTimeout.toMillis()).intValue());
    }
  }

  protected DefaultThreadFactory createThreadFactory() {
    return new DefaultThreadFactory(MultithreadEventExecutorGroup.class, true);
  }

}
