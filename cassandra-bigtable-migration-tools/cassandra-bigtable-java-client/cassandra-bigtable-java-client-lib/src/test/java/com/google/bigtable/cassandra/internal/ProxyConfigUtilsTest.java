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

import static org.junit.jupiter.api.Assertions.assertEquals;

import com.google.bigtable.cassandra.BigtableCqlConfiguration;
import com.google.bigtable.cassandra.OpenTelemetryCollectorConfiguration;
import org.junit.jupiter.api.Test;

/**
 * Internal use only.
 */
public class ProxyConfigUtilsTest {

  @Test
  public void testCreateProxyConfig_Minimal() {
    BigtableCqlConfiguration bigtableCqlConfiguration = BigtableCqlConfiguration.builder()
        .setProjectId("someProjectId")
        .setInstanceId("someInstanceId")
        .setSchemaMappingTable("someSchemaMappingTable")
        .build();

    ProxyConfig proxyConfig = ProxyConfigUtils.createProxyConfig(bigtableCqlConfiguration);

    String expectedProxyConfigYaml = "cassandraToBigtableConfigs:\n"
        + "  defaultColumnFamily: cf1\n"
        + "  projectId: someProjectId\n"
        + "  schemaMappingTable: someSchemaMappingTable\n"
        + "listeners:\n"
        + "- bigtable:\n"
        + "    appProfileId: default\n"
        + "    defaultColumnFamily: cf1\n"
        + "    instanceIds: someInstanceId\n"
        + "    projectId: someProjectId\n"
        + "    schemaMappingTable: someSchemaMappingTable\n"
        + "    session:\n"
        + "      grpcChannels: 4\n"
        + "  name: cluster1\n"
        + "  otel:\n"
        + "    disabled: true\n"
        + "loggerConfig:\n"
        + "  outputType: stdout\n"
        + "otel:\n"
        + "  enabled: false\n";

    assertEquals(expectedProxyConfigYaml, proxyConfig.toYaml());
  }

  @Test
  public void testCreateProxyConfig_Detailed() {
    BigtableCqlConfiguration bigtableCqlConfiguration = BigtableCqlConfiguration.builder()
        .setProjectId("someProjectId")
        .setInstanceId("someInstanceId")
        .setSchemaMappingTable("someSchemaMappingTable")
        .setAppProfileId("someAppProfile")
        .setDefaultColumnFamily("someDefaultColumnFamily")
        .setBigtableChannelPoolSize(3)
        .enableOpenTelemetry(OpenTelemetryCollectorConfiguration.builder()
            .setServiceName("someOtelServiceName")
            .setHealthcheckEndpoint("someOtelHealthcheckEndpoint")
            .enableMetrics("someOtelMetricsEndpoint")
            .enableTracing("someOtelTracingEndpoint", 1.0)
            .build())
        .build();

    ProxyConfig proxyConfig = ProxyConfigUtils.createProxyConfig(bigtableCqlConfiguration);

    String expectedProxyConfigYaml = "cassandraToBigtableConfigs:\n"
        + "  defaultColumnFamily: someDefaultColumnFamily\n"
        + "  projectId: someProjectId\n"
        + "  schemaMappingTable: someSchemaMappingTable\n"
        + "listeners:\n"
        + "- bigtable:\n"
        + "    appProfileId: someAppProfile\n"
        + "    defaultColumnFamily: someDefaultColumnFamily\n"
        + "    instanceIds: someInstanceId\n"
        + "    projectId: someProjectId\n"
        + "    schemaMappingTable: someSchemaMappingTable\n"
        + "    session:\n"
        + "      grpcChannels: 3\n"
        + "  name: cluster1\n"
        + "  otel:\n"
        + "    disabled: false\n"
        + "loggerConfig:\n"
        + "  outputType: stdout\n"
        + "otel:\n"
        + "  enabled: true\n"
        + "  enabledClientSideMetrics: true\n"
        + "  healthCheck:\n"
        + "    enabled: true\n"
        + "    endpoint: someOtelHealthcheckEndpoint\n"
        + "  metrics:\n"
        + "    endpoint: someOtelMetricsEndpoint\n"
        + "  serviceName: someOtelServiceName\n"
        + "  traces:\n"
        + "    endpoint: someOtelTracingEndpoint\n"
        + "    samplingRatio: 1.0\n";

    assertEquals(expectedProxyConfigYaml, proxyConfig.toYaml());
  }

}
