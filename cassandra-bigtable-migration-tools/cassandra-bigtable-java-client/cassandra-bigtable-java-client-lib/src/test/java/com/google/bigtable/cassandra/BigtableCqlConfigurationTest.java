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

package com.google.bigtable.cassandra;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

import org.junit.jupiter.api.Test;

import java.util.Optional;

public class BigtableCqlConfigurationTest {

  @Test
  public void testBigtableCqlConfig() {
    BigtableCqlConfiguration bigtableCqlConfiguration = BigtableCqlConfiguration.builder()
        .setProjectId("someProjectId")
        .setInstanceId("someInstanceId")
        .setAppProfileId("someAppProfileId")
        .setDefaultColumnFamily("someColumnFamily")
        .setSchemaMappingTable("someSchemaMappingTable")
        .setBigtableChannelPoolSize(4)
        .setDefaultKeyspace("someKeyspace")
        .enableOpenTelemetry(OpenTelemetryCollectorConfiguration.builder()
            .setServiceName("someServiceName")
            .enableMetrics("someMetricsEndpoint")
            .setHealthcheckEndpoint("someHealthcheckEndpoint")
            .enableTracing("someTraceEndpoint", 1.0)
            .build())
        .build();

    assertEquals("someProjectId", bigtableCqlConfiguration.getProjectId());
    assertEquals("someInstanceId", bigtableCqlConfiguration.getInstanceId());
    assertEquals("someAppProfileId", bigtableCqlConfiguration.getAppProfileId().get());
    assertEquals("someColumnFamily", bigtableCqlConfiguration.getDefaultColumnFamily().get());
    assertEquals("someSchemaMappingTable", bigtableCqlConfiguration.getSchemaMappingTable().get());
    assertEquals("someKeyspace", bigtableCqlConfiguration.getDefaultKeyspace().get());
    assertEquals(4, bigtableCqlConfiguration.getBigtableChannelPoolSize().getAsInt());
    assertTrue(bigtableCqlConfiguration.getOpenTelemetryConfiguration().isPresent());

    OpenTelemetryCollectorConfiguration otel = (OpenTelemetryCollectorConfiguration) bigtableCqlConfiguration.getOpenTelemetryConfiguration().get();
    assertEquals("someServiceName", otel.serviceName());
    assertEquals("someMetricsEndpoint", otel.metricsEndpoint().get());
    assertEquals("someHealthcheckEndpoint", otel.healthcheckEndpoint().get());
    assertEquals("someTraceEndpoint", otel.tracesEndpoint().get());
    assertEquals(1.0, otel.tracesSamplingRatio().get());
  }

  @Test
  public void testBigtableCqlConfigDefaultsForOptionalFields() {
    BigtableCqlConfiguration bigtableCqlConfiguration = BigtableCqlConfiguration.builder()
        .setProjectId("someProjectId")
        .setInstanceId("someInstanceId")
        .setSchemaMappingTable("someSchemaMappingTable")
        .build();

    assertEquals("someProjectId", bigtableCqlConfiguration.getProjectId());
    assertEquals("someInstanceId", bigtableCqlConfiguration.getInstanceId());
    assertEquals("default", bigtableCqlConfiguration.getAppProfileId().get());
    assertEquals("cf1", bigtableCqlConfiguration.getDefaultColumnFamily().get());
    assertEquals("someSchemaMappingTable", bigtableCqlConfiguration.getSchemaMappingTable().get());
    assertEquals(Optional.empty(), bigtableCqlConfiguration.getDefaultKeyspace());
    assertEquals(4, bigtableCqlConfiguration.getBigtableChannelPoolSize().getAsInt());
  }

  @Test
  public void testBigtableCqlConfigDisableOtel() {
    BigtableCqlConfiguration bigtableCqlConfiguration = BigtableCqlConfiguration.builder()
        .setProjectId("someProjectId")
        .setInstanceId("someInstanceId")
        .setSchemaMappingTable("someSchemaMappingTable")
        .disableOpenTelemetry()
        .build();

    assertEquals("someProjectId", bigtableCqlConfiguration.getProjectId());
    assertEquals("someInstanceId", bigtableCqlConfiguration.getInstanceId());
    assertEquals("default", bigtableCqlConfiguration.getAppProfileId().get());
    assertEquals("cf1", bigtableCqlConfiguration.getDefaultColumnFamily().get());
    assertEquals("someSchemaMappingTable", bigtableCqlConfiguration.getSchemaMappingTable().get());
    assertEquals(4, bigtableCqlConfiguration.getBigtableChannelPoolSize().getAsInt());
  }

}
