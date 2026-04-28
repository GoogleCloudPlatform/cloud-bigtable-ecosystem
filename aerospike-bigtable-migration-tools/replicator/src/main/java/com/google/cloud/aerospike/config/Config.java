/*
 * Copyright 2025 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package com.google.cloud.aerospike.config;

import java.util.Map;
import org.apache.kafka.common.config.AbstractConfig;
import org.apache.kafka.common.config.ConfigDef;

public class Config extends AbstractConfig {
  public static final String START_LUT_MILLIS = "start.lut.millis";

  public static ConfigDef getDefinition() {
    return new ConfigDef()
        .define(
            START_LUT_MILLIS,
            ConfigDef.Type.LONG,
            ConfigDef.NO_DEFAULT_VALUE,
            ConfigDef.Range.atLeast(0),
            ConfigDef.Importance.HIGH,
            "Records with Aerospike LUT (last update timestamp) lower than this value will be"
                + " dropped by this SMT. Expected to be a number of milliseconds since UNIX"
                + " epoch.");
  }

  public Config(final Map<?, ?> configs) {
    super(getDefinition(), configs);
  }
}
