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

import com.google.auto.value.AutoValue;
import com.google.common.base.Preconditions;

import java.util.Optional;
import java.util.OptionalInt;

/**
 * Configuration for connecting to Cloud Bigtable using CQL.
 *
 * <p>This class provides a builder to configure various connection parameters.
 */
@AutoValue
public abstract class BigtableCqlConfiguration {

  private static final String DEFAULT_SCHEMA_MAPPING_TABLE = "schema_mapping";
  private static final String DEFAULT_COLUMN_FAMILY = "cf1";
  private static final int DEFAULT_BIGTABLE_CHANNEL_POOL_SIZE = 4;
  private static final String DEFAULT_APP_PROFILE_ID = "default";

  /**
   * @return The Google Cloud project ID.
   * @see BigtableCqlConfiguration.Builder#setProjectId(String)
   */
  public abstract String getProjectId();

  /**
   * @return The Bigtable instance ID.
   * @see BigtableCqlConfiguration.Builder#setInstanceId(String)
   */
  public abstract String getInstanceId();

  /**
   * @return An {@link Optional} containing the Bigtable app profile ID.
   * @see BigtableCqlConfiguration.Builder#setAppProfileId(String)
   */
  public abstract Optional<String> getAppProfileId();

  /**
   * @return An {@link Optional} containing the schema mapping table name.
   * @see BigtableCqlConfiguration.Builder#setSchemaMappingTable
   */
  public abstract Optional<String> getSchemaMappingTable();

  /**
   * @return An {@link Optional} containing the default column family name.
   * @see BigtableCqlConfiguration.Builder#setDefaultColumnFamily(String)
   */
  public abstract Optional<String> getDefaultColumnFamily();

  /**
   * @return An {@link OptionalInt} containing the Bigtable channel pool size.
   * @see BigtableCqlConfiguration.Builder#setBigtableChannelPoolSize(int)
   */
  public abstract OptionalInt getBigtableChannelPoolSize();

  /**
   * @return An {@link Optional} containing the {@link OpenTelemetryConfiguration}.
   * @see BigtableCqlConfiguration.Builder#enableOpenTelemetry(OpenTelemetryConfiguration)
   */
  public abstract Optional<OpenTelemetryConfiguration> getOpenTelemetryConfiguration();

  /**
   * @return An {@link Optional} containing the default keyspace for the session.
   * @see BigtableCqlConfiguration.Builder#setDefaultKeyspace
   */
  public abstract Optional<String> getDefaultKeyspace();

  /**
   * Creates a new {@link Builder} for {@link BigtableCqlConfiguration}.
   * <p>
   * A new builder is created with default values for some properties.
   *
   * @return A new {@link Builder}.
   */
  public static Builder builder() {
    return new AutoValue_BigtableCqlConfiguration.Builder()
        // Set defaults
        .setSchemaMappingTable(DEFAULT_SCHEMA_MAPPING_TABLE)
        .setDefaultColumnFamily(DEFAULT_COLUMN_FAMILY)
        .setBigtableChannelPoolSize(DEFAULT_BIGTABLE_CHANNEL_POOL_SIZE)
        .setAppProfileId(DEFAULT_APP_PROFILE_ID)
        .setDefaultKeyspace(Optional.empty());
  }

  /**
   * Builder for {@link BigtableCqlConfiguration}.
   */
  @AutoValue.Builder
  public abstract static class Builder {

    /**
     * Sets the Google Cloud Project ID.
     * This should be the Google Cloud project ID that contains the targeted Bigtable Instance.
     * This should be the bare project identifier (i.e. my-project) rather than the fully qualified path (e.g. projects/my-project).
     *
     * @param projectId The project ID.
     * @return This {@link Builder} instance.
     */
    public abstract Builder setProjectId(String projectId);

    /**
     * Sets the Bigtable instance ID.
     * This should be the bare instance ID (e.g. my-instance) rather than the full qualified path
     * (e.g. projects/my-project/instances/my-instance). See <a
     * href="https://cloud.google.com/bigtable/docs/instances-clusters-nodes#instances">documentation for details</a>.
     *
     * @param instanceId The instance ID.
     * @return This {@link Builder} instance.
     */
    public abstract Builder setInstanceId(String instanceId);

    abstract Builder setAppProfileId(Optional<String> appProfileId);

    /**
     * Sets the Bigtable App Profile ID to use.
     * See <a href="https://cloud.google.com/bigtable/docs/app-profiles">documentation for details</a>
     *
     * @param appProfileId The app profile ID.
     * @return This {@link Builder} instance.
     */
    public Builder setAppProfileId(String appProfileId) {
      this.setAppProfileId(Optional.of(appProfileId));
      return this;
    }

    abstract Builder setSchemaMappingTable(Optional<String> schemaMappingTable);

    /**
     * Sets the schema mapping table name.
     * This is the schema mapping table created during the <a
     * href="https://github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/tree/main/cassandra-bigtable-proxy/docs/schema-mapping-creation/README.md">
     * Schema Setup process</a>. The schema_mapping table acts as a metadata repository, holding the
     * schema configuration for your Cassandra-like tables in Bigtable. It stores details such as
     * column names, data types, and primary key information.
     *
     * @param schemaMappingTable The schema mapping table name.
     * @return This {@link Builder} instance.
     */
    public Builder setSchemaMappingTable(String schemaMappingTable) {
      return this.setSchemaMappingTable(Optional.of(schemaMappingTable));
    }

    abstract Builder setDefaultColumnFamily(Optional<String> defaultColumnFamily);

    /**
     * Sets the default column family name.
     * This is the name of the Bigtable column family to target for columns with 'primitive' data
     * types when the column family is not set explicitly set.
     *
     * @param defaultColumnFamily The default column family name.
     * @return This {@link Builder} instance.
     */
    public Builder setDefaultColumnFamily(String defaultColumnFamily) {
      this.setDefaultColumnFamily(Optional.of(defaultColumnFamily));
      return this;
    }

    abstract Builder setDefaultKeyspace(Optional<String> keyspace);

    /**
     * Sets the default keyspace for the session.
     *
     * @param keyspace The default keyspace to use.
     * @return This {@link Builder} instance.
     */
    public Builder setDefaultKeyspace(String keyspace) {
      this.setDefaultKeyspace(Optional.of(keyspace));
      return this;
    }

    abstract Builder setBigtableChannelPoolSize(OptionalInt bigtableChannelPoolSize);

    /**
     * Sets the Bigtable channel pool size.
     * The number of gRPC channels that the bigtable client will use. The Bigtable client uses an
     * underlying round-robin channel pool that allows it to spread load across shared load
     * balancers. A good rule of thumb is to limit each channel to at most 50 qps.
     *
     * @param bigtableChannelPoolSize The Bigtable channel pool size.
     * @return This {@link Builder} instance.
     */
    public Builder setBigtableChannelPoolSize(int bigtableChannelPoolSize) {
      this.setBigtableChannelPoolSize(OptionalInt.of(bigtableChannelPoolSize));
      return this;
    }

    abstract Builder setOpenTelemetryConfiguration(
        Optional<OpenTelemetryConfiguration> openTelemetryConfiguration);

    /**
     * Enables Opentelemetry health-checks, metrics and tracing instrumentation.
     * Currently only `{@link OpenTelemetryCollectorConfiguration } is supported.
     * This only affects monitoring of the proxy itself. Please note, Bigtable client's builtin
     * metrics are separate and are not affected by this configuration.
     *
     * @param config The {@link OpenTelemetryConfiguration}.
     * @return This {@link Builder} instance.
     */
    public Builder enableOpenTelemetry(OpenTelemetryConfiguration config) {
      Preconditions.checkArgument(config instanceof OpenTelemetryCollectorConfiguration,
          "Only OpenTelemetryCollectorConfiguration is currently supported");
      this.setOpenTelemetryConfiguration(Optional.of(config));
      return this;
    }

    /**
     * Disables OpenTelemetry.
     *
     * @return This {@link Builder} instance.
     */
    public Builder disableOpenTelemetry() {
      this.setOpenTelemetryConfiguration(Optional.empty());
      return this;
    }

    /**
     * Builds the {@link BigtableCqlConfiguration}.
     *
     * @return The built {@link BigtableCqlConfiguration}.
     */
    public abstract BigtableCqlConfiguration build();
  }

}
