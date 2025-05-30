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

import com.datastax.oss.driver.api.core.metadata.EndPoint;
import io.netty.channel.unix.DomainSocketAddress;
import java.net.SocketAddress;

/**
 * Internal use only
 */
class UdsEndpoint implements EndPoint {

  private final DomainSocketAddress unixDomainSocketAddress;

  UdsEndpoint(DomainSocketAddress unixDomainSocketAddress) {
    this.unixDomainSocketAddress = unixDomainSocketAddress;
  }

  @Override
  public SocketAddress resolve() {
    return this.unixDomainSocketAddress;
  };

  @Override
  public String asMetricPrefix() {
    return this.unixDomainSocketAddress.toString()
        .replace("/", "-")
        .replace(".", "-");
  };

}
