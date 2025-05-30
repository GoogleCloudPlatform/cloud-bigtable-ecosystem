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

import com.google.bigtable.cassandra.internal.ProxyConfig.Bigtable;
import com.google.bigtable.cassandra.internal.ProxyConfig.CassandraToBigTableConfig;
import com.google.bigtable.cassandra.internal.ProxyConfig.HealthCheck;
import com.google.bigtable.cassandra.internal.ProxyConfig.Listener;
import com.google.bigtable.cassandra.internal.ProxyConfig.ListenerOtel;
import com.google.bigtable.cassandra.internal.ProxyConfig.LoggerConfig;
import com.google.bigtable.cassandra.internal.ProxyConfig.Metrics;
import com.google.bigtable.cassandra.internal.ProxyConfig.OtelConfig;
import com.google.bigtable.cassandra.internal.ProxyConfig.Session;
import com.google.bigtable.cassandra.internal.ProxyConfig.Traces;
import java.util.ArrayList;
import java.util.List;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.assertEquals;

/**
 * Internal use only.
 */
public class ProxyConfigTest {

  @Test
  public void testToYaml() {
    CassandraToBigTableConfig cassandraToBigTableConfigs = CassandraToBigTableConfig.builder()
        .setProjectId("some-project-id")
        .setSchemaMappingTable("some-schema-mapping-table")
        .setDefaultColumnFamily("cf1")
        .build();

    LoggerConfig loggerConfig = LoggerConfig.builder()
        .outputType("stdout")
        .filename("output/output.log")
        .maxSize(10)
        .maxBackups(2)
        .maxAge(1)
        .compress(true)
        .build();

    OtelConfig otelConfig = OtelConfig.builder()
        .enabled(false)
        .enabledClientSideMetrics(true)
        .serviceName("otel-service")
        .healthCheck(HealthCheck.builder()
            .enabled(false)
            .endpoint("localhost:12345").build())
        .metrics(new Metrics("localhost:1234"))
        .traces(Traces.builder()
            .endpoint("localhost:1234")
            .samplingRatio(1.0).build())
        .build();

    List<Listener> listeners = new ArrayList<>();
    listeners.add(Listener.builder()
        .name("cluster1")
        .bigtable(Bigtable.builder()
            .projectId("someProjectId")
            .instanceIds("SomeInstanceId")
            .schemaMappingTable("someSchemaMappingTable")
            .appProfileId("someAppProfileId")
            .defaultColumnFamily("cf1")
            .session(new Session(4))
            .build())
        .otel(new ListenerOtel(false))
        .build());

    ProxyConfig config = ProxyConfig.builder()
        .setCassandraToBigtableConfigs(cassandraToBigTableConfigs)
        .setListeners(listeners)
        .setOtel(otelConfig)
        .setLoggerConfig(loggerConfig)
        .build();

    String actualYaml = config.toYaml();

    String expectedYaml = "cassandraToBigtableConfigs:\n"
        + "  defaultColumnFamily: cf1\n"
        + "  projectId: some-project-id\n"
        + "  schemaMappingTable: some-schema-mapping-table\n"
        + "listeners:\n"
        + "- bigtable:\n"
        + "    appProfileId: someAppProfileId\n"
        + "    defaultColumnFamily: cf1\n"
        + "    instanceIds: SomeInstanceId\n"
        + "    projectId: someProjectId\n"
        + "    schemaMappingTable: someSchemaMappingTable\n"
        + "    session:\n"
        + "      grpcChannels: 4\n"
        + "  name: cluster1\n"
        + "  otel:\n"
        + "    disabled: false\n"
        + "loggerConfig:\n"
        + "  compress: true\n"
        + "  filename: output/output.log\n"
        + "  maxAge: 1\n"
        + "  maxBackups: 2\n"
        + "  maxSize: 10\n"
        + "  outputType: stdout\n"
        + "otel:\n"
        + "  enabled: false\n"
        + "  enabledClientSideMetrics: true\n"
        + "  healthCheck:\n"
        + "    enabled: false\n"
        + "    endpoint: localhost:12345\n"
        + "  metrics:\n"
        + "    endpoint: localhost:1234\n"
        + "  serviceName: otel-service\n"
        + "  traces:\n"
        + "    endpoint: localhost:1234\n"
        + "    samplingRatio: 1.0\n";

    assertEquals(expectedYaml, actualYaml);
  }

  @Test
  public void testToYamlNullFieldsOmitted() {
    CassandraToBigTableConfig cassandraToBigTableConfigs = CassandraToBigTableConfig.builder()
        .setProjectId("some-project-id")
        .setSchemaMappingTable("some-schema-mapping-table")
        .setDefaultColumnFamily("cf1")
        .build();

    // omitted maxAge
    LoggerConfig loggerConfig = LoggerConfig.builder()
        .outputType("stdout")
        .filename("output/output.log")
        .maxBackups(2)
        .compress(true)
        .build();

    OtelConfig otelConfig = OtelConfig.builder()
        .enabled(false)
        .enabledClientSideMetrics(true)
        .serviceName("otel-service")
        .healthCheck(HealthCheck.builder().endpoint("localhost:12345").build())
        .metrics(new Metrics("localhost:1234"))
        .traces(Traces.builder()
            .endpoint("localhost:1234")
            .samplingRatio(1.0).build())
        .build();

    List<Listener> listeners = new ArrayList<>();
    // omitted port
    listeners.add(Listener.builder()
        .name("cluster1")
        .bigtable(Bigtable.builder()
            .projectId("someProjectId")
            .instanceIds("SomeInstanceId")
            .schemaMappingTable("someSchemaMappingTable")
            .appProfileId("someAppProfileId")
            .defaultColumnFamily("cf1")
            .session(new Session(4))
            .build())
        .otel(new ListenerOtel(false))
        .build());

    ProxyConfig config = ProxyConfig.builder()
        .setCassandraToBigtableConfigs(cassandraToBigTableConfigs)
        .setListeners(listeners)
        .setOtel(otelConfig)
        .setLoggerConfig(loggerConfig)
        .build();

    String actualYaml = config.toYaml();

    String expectedYaml = "cassandraToBigtableConfigs:\n"
        + "  defaultColumnFamily: cf1\n"
        + "  projectId: some-project-id\n"
        + "  schemaMappingTable: some-schema-mapping-table\n"
        + "listeners:\n"
        + "- bigtable:\n"
        + "    appProfileId: someAppProfileId\n"
        + "    defaultColumnFamily: cf1\n"
        + "    instanceIds: SomeInstanceId\n"
        + "    projectId: someProjectId\n"
        + "    schemaMappingTable: someSchemaMappingTable\n"
        + "    session:\n"
        + "      grpcChannels: 4\n"
        + "  name: cluster1\n"
        + "  otel:\n"
        + "    disabled: false\n"
        + "loggerConfig:\n"
        + "  compress: true\n"
        + "  filename: output/output.log\n"
        + "  maxBackups: 2\n"
        + "  outputType: stdout\n"
        + "otel:\n"
        + "  enabled: false\n"
        + "  enabledClientSideMetrics: true\n"
        + "  healthCheck:\n"
        + "    endpoint: localhost:12345\n"
        + "  metrics:\n"
        + "    endpoint: localhost:1234\n"
        + "  serviceName: otel-service\n"
        + "  traces:\n"
        + "    endpoint: localhost:1234\n"
        + "    samplingRatio: 1.0\n";

    assertEquals(expectedYaml, actualYaml);
  }

}
