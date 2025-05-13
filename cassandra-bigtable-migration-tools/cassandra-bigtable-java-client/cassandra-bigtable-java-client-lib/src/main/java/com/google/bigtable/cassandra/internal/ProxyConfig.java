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

import com.google.auto.value.AutoValue;
import java.io.StringWriter;
import java.util.List;
import org.yaml.snakeyaml.DumperOptions;
import org.yaml.snakeyaml.Yaml;
import org.yaml.snakeyaml.introspector.Property;
import org.yaml.snakeyaml.introspector.BeanAccess;
import org.yaml.snakeyaml.nodes.NodeTuple;
import org.yaml.snakeyaml.nodes.Tag;
import org.yaml.snakeyaml.representer.Representer;

/**
 * Internal use only. This is the object model for the cassandra-to-bigtable-proxy `config.yaml`
 * configuration file.
 */
@AutoValue
abstract class ProxyConfig {

  private static final int YAML_INDENT = 2;

  abstract CassandraToBigTableConfig getCassandraToBigtableConfigs();
  abstract List<Listener> getListeners();
  abstract OtelConfig getOtel();
  abstract LoggerConfig getLoggerConfig();

  protected static Builder builder() {
    return new AutoValue_ProxyConfig.Builder();
  }

  @AutoValue.Builder
  protected abstract static class Builder {
    abstract Builder setCassandraToBigtableConfigs(CassandraToBigTableConfig value);
    abstract Builder setListeners(List<Listener> value);
    abstract Builder setOtel(OtelConfig value);
    abstract Builder setLoggerConfig(LoggerConfig value);
    abstract ProxyConfig build();
  }

  @AutoValue
  static abstract class CassandraToBigTableConfig {
    abstract String getProjectId();
    abstract String getSchemaMappingTable();
    abstract String getDefaultColumnFamily();

    protected static Builder builder() {
      return new AutoValue_ProxyConfig_CassandraToBigTableConfig.Builder();
    }

    @AutoValue.Builder
    protected abstract static class Builder {
      abstract Builder setProjectId(String projectId);
      abstract Builder setSchemaMappingTable(String schemaMappingTable);
      abstract Builder setDefaultColumnFamily(String defaultColumnFamily);
      abstract CassandraToBigTableConfig build();
    }
  }

  static class Listener {
    protected String name;
    protected Integer port;
    protected Bigtable bigtable;
    protected ListenerOtel otel;

    protected static class Builder {
      private String name;
      private Integer port;
      private Bigtable bigtable;
      private ListenerOtel otel;

      protected Builder name(String name) {
        this.name = name;
        return this;
      }

      protected Builder port (Integer port) {
        this.port = port;
        return this;
      }

      protected Builder bigtable(Bigtable bigtable) {
        this.bigtable = bigtable;
        return this;
      }

      protected Builder otel(ListenerOtel otel) {
        this.otel = otel;
        return this;
      }

      protected Listener build() {
        Listener listener = new Listener();
        listener.name = this.name;
        listener.port = this.port;
        listener.bigtable = this.bigtable;
        listener.otel = this.otel;
        return listener;
      }
    }

    protected static Builder builder() {
      return new Builder();
    }
  }

  protected static class Bigtable {
    protected String projectId;
    protected String instanceIds;
    protected String schemaMappingTable;
    protected Session session;
    protected String defaultColumnFamily;
    protected String appProfileId;

    protected static class Builder {
      private String projectId;
      private String instanceIds;
      private String schemaMappingTable;
      private Session session;
      private String defaultColumnFamily;
      private String appProfileId;

      protected Builder projectId(String projectId) {
        this.projectId = projectId;
        return this;
      }

      protected Builder instanceIds(String instanceIds) {
        this.instanceIds = instanceIds;
        return this;
      }

      protected Builder schemaMappingTable(String schemaMappingTable) {
        this.schemaMappingTable = schemaMappingTable;
        return this;
      }

      protected Builder session(Session session) {
        this.session = session;
        return this;
      }

      protected Builder defaultColumnFamily(String defaultColumnFamily) {
        this.defaultColumnFamily = defaultColumnFamily;
        return this;
      }

      protected Builder appProfileId(String appProfileId) {
        this.appProfileId = appProfileId;
        return this;
      }

      protected Bigtable build() {
        Bigtable bigtable = new Bigtable();
        bigtable.projectId = this.projectId;
        bigtable.instanceIds = this.instanceIds;
        bigtable.schemaMappingTable = this.schemaMappingTable;
        bigtable.session = this.session;
        bigtable.defaultColumnFamily = this.defaultColumnFamily;
        bigtable.appProfileId = this.appProfileId;
        return bigtable;
      }

    }

    protected static Builder builder() { return new Builder(); }
  }

  protected static class Session {
    protected Integer grpcChannels;

    protected Session(Integer grpcChannels) {
      this.grpcChannels = grpcChannels;
    }
  }

  protected static class ListenerOtel {
    protected Boolean disabled;

    protected ListenerOtel(Boolean disabled) {
      this.disabled = disabled;
    }
  }

  protected static class OtelConfig {
    protected Boolean enabled;
    protected Boolean enabledClientSideMetrics;
    protected String serviceName;
    protected HealthCheck healthCheck;
    protected Metrics metrics;
    protected Traces traces;

    protected static class Builder {
      private Boolean enabled;
      private Boolean enabledClientSideMetrics;
      private String serviceName;
      private HealthCheck healthCheck;
      private Metrics metrics;
      private Traces traces;

      protected Builder enabled(Boolean enabled) {
        this.enabled = enabled;
        return this;
      }

      protected Builder enabledClientSideMetrics(Boolean enabledClientSideMetrics) {
        this.enabledClientSideMetrics = enabledClientSideMetrics;
        return this;
      }

      protected Builder serviceName(String serviceName) {
        this.serviceName = serviceName;
        return this;
      }

      protected Builder healthCheck(HealthCheck healthCheck) {
        this.healthCheck = healthCheck;
        return this;
      }

      protected Builder metrics(Metrics metrics) {
        this.metrics = metrics;
        return this;
      }

      protected Builder traces(Traces traces) {
        this.traces = traces;
        return this;
      }

      protected OtelConfig build() {
        OtelConfig config = new OtelConfig();
        config.enabled = this.enabled;
        config.enabledClientSideMetrics = this.enabledClientSideMetrics;
        config.serviceName = this.serviceName;
        config.healthCheck = this.healthCheck;
        config.metrics = this.metrics;
        config.traces = this.traces;
        return config;
      }
    }

    protected static Builder builder() {
      return new Builder();
    }
  }

  protected static class HealthCheck {
    protected Boolean enabled;
    protected String endpoint;

    protected static class Builder {
      private Boolean enabled;
      private String endpoint;

      protected Builder enabled(Boolean enabled) {
        this.enabled = enabled;
        return this;
      }

      protected Builder endpoint(String endpoint) {
        this.endpoint = endpoint;
        return this;
      }

      protected HealthCheck build() {
        HealthCheck healthCheck = new HealthCheck();
        healthCheck.enabled = this.enabled;
        healthCheck.endpoint = this.endpoint;
        return healthCheck;
      }
    }

    protected static Builder builder() {
      return new Builder();
    }
  }

  protected static class Metrics {
    protected String endpoint;

    protected Metrics(String endpoint) {
      this.endpoint = endpoint;
    }
  }

  protected static class Traces {
    protected String endpoint;
    protected Double samplingRatio;

    protected static class Builder {
      private String endpoint;
      private Double samplingRatio;

      protected Builder endpoint(String endpoint) {
        this.endpoint = endpoint;
        return this;
      }

      protected Builder samplingRatio(Double samplingRatio) {
        this.samplingRatio = samplingRatio;
        return this;
      }

      protected Traces build() {
        Traces traces = new Traces();
        traces.endpoint = this.endpoint;
        traces.samplingRatio = this.samplingRatio;
        return traces;
      }
    }

    public static Builder builder() {
      return new Builder();
    }
  }

  static class LoggerConfig {
    protected String outputType;
    protected String filename;
    protected Integer maxSize;
    protected Integer maxBackups;
    protected Integer maxAge;
    protected Boolean compress;

    protected static class Builder {
      private String outputType;
      private String filename;
      private Integer maxSize;
      private Integer maxBackups;
      private Integer maxAge;
      private Boolean compress;

      protected Builder outputType(String outputType) {
        this.outputType = outputType;
        return this;
      }

      protected Builder filename(String filename) {
        this.filename = filename;
        return this;
      }

      protected Builder maxSize (Integer maxSize) {
        this.maxSize = maxSize;
        return this;
      }

      protected Builder maxBackups (Integer maxBackups) {
        this.maxBackups = maxBackups;
        return this;
      }

      protected Builder maxAge (Integer maxAge) {
        this.maxAge = maxAge;
        return this;
      }

      protected Builder compress(Boolean compress) {
        this.compress = compress;
        return this;
      }

      protected LoggerConfig build() {
        LoggerConfig config = new LoggerConfig();
        config.outputType = outputType;
        config.filename = filename;
        config.maxSize = maxSize;
        config.maxBackups = maxBackups;
        config.maxAge = maxAge;
        config.compress = compress;
        return config;
      }
    }

    private LoggerConfig() {}

    protected LoggerConfig(String outputType) {
      this.outputType = outputType;
    }

    protected static Builder builder() {
      return new Builder();
    }
  }

  private static class OmitNullRepresenter extends Representer {

    public OmitNullRepresenter(DumperOptions options) {
      super(options);
    }

    @Override
    protected NodeTuple representJavaBeanProperty(Object javaBean, Property property, Object propertyValue, Tag customTag) {
      if (propertyValue == null) {
        return null;
      } else {
        return super.representJavaBeanProperty(javaBean, property, propertyValue, customTag);
      }
    }
  }

  protected String toYaml() {
    DumperOptions options = new DumperOptions();
    options.setIndent(YAML_INDENT);
    options.setDefaultFlowStyle(DumperOptions.FlowStyle.BLOCK);

    Representer representer = new OmitNullRepresenter(options);
    representer.addClassTag(AutoValue_ProxyConfig.class, Tag.MAP);
    representer.addClassTag(AutoValue_ProxyConfig_CassandraToBigTableConfig.class, Tag.MAP);

    Yaml yaml = new Yaml(representer, options);
    yaml.setBeanAccess(BeanAccess.FIELD);
    StringWriter stringWriter = new StringWriter();
    yaml.dump(this, stringWriter);
    return stringWriter.toString();
  }

}
