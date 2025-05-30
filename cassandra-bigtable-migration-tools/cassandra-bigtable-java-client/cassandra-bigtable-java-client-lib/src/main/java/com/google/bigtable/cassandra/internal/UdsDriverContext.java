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

import com.datastax.oss.driver.api.core.config.DriverConfigLoader;
import com.datastax.oss.driver.api.core.session.ProgrammaticArguments;
import com.datastax.oss.driver.internal.core.context.DefaultDriverContext;
import com.datastax.oss.driver.internal.core.context.NettyOptions;

/**
 * Internal use only
 */
class UdsDriverContext extends DefaultDriverContext {

  public UdsDriverContext(DriverConfigLoader configLoader,
      ProgrammaticArguments programmaticArguments) {
    super(configLoader, programmaticArguments);
  }

  @Override
  protected NettyOptions buildNettyOptions() {
    if (OsUtils.isWindows()) {
      throw new IllegalStateException("Windows is not supported");
    }
    // Mac
    if (OsUtils.isMac()) {
      return new UdsKqueueNettyOptions(this);
    }
    // Linux
    return new UdsEpollNettyOptions(this);
  }

}
