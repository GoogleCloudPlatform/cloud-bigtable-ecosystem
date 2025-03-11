
# Integrating OpenTelemetry (OTEL) with Your Application

## Overview

OpenTelemetry (OTEL) provides a set of tools, APIs, and SDKs to instrument, generate, collect, and export telemetry data (metrics, logs, and traces) for monitoring and troubleshooting your applications. This guide will help you integrate OTEL into your application, enabling you to capture and export metrics and traces.


# Setting up the OTEL Collector Service on GCP

This guide provides step-by-step instructions to set up the OpenTelemetry (OTEL) Collector Service on Google Cloud Platform (GCP) using the provided configuration file.

## Prerequisites

Before enabling OTEL in your application, ensure that the collector service is up and running. The collector service is responsible for capturing and exporting the telemetry data. By default, OTEL is disabled in the configuration.

1. **GCP Account**: Ensure you have a GCP account and have access to a GCP project.
2. **Google Cloud SDK**: Install the [Google Cloud SDK](https://cloud.google.com/sdk/docs/install).
3. **Docker**: Install Docker to run the OTEL Collector in a container.
4. **Kubernetes Cluster (GKE)**: Set up a Google Kubernetes Engine (GKE) cluster if deploying in a Kubernetes environment.

## Steps to Set Up and Configure the OTEL Collector Service with Proxy Adaptor.

### Step 1: Use the below config.yaml (or set up the `CONFIG_FILE` environment variable) file for OTEL collector service

```yaml
  # Enable the health check in the proxy application config only if the
  # "health_check" extension is added to this OTEL config for the collector service.
  #
  # Recommendation:
  # Enable the OTEL health check if you need to verify the collector's availability
  # at the start of the application. For development or testing environments, it can
  # be safely disabled to reduce complexity.
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: "0.0.0.0:4317"
      http:
        endpoint: "0.0.0.0:55681"

processors:
  batch:
    send_batch_max_size: 0
    send_batch_size: 8192
    timeout: 5s

  memory_limiter:
    # drop metrics if memory usage gets too high
    check_interval: 5s
    limit_percentage: 65
    spike_limit_percentage: 20

  resourcedetection:
    detectors: [gcp]
    timeout: 10s

exporters:
  googlecloud:
    metric:
      instrumentation_library_labels: true
      service_resource_labels: true

extensions:
  health_check:
    endpoint: "0.0.0.0:13133"

service:
  extensions: [health_check]
  pipelines:
    metrics:
      receivers: [otlp]
      processors: [batch, memory_limiter, resourcedetection]
      exporters: [googlecloud]
    traces:
      receivers: [otlp]
      processors: [batch, memory_limiter, resourcedetection]
      exporters: [googlecloud]
```

### Step 2: Configure Proxy Adaptor

Follow these steps to enable and configure OTEL in your application:

1. **Edit the `config.yaml` (or set up the `CONFIG_FILE` environment variable) File:**
   - Set the `enabled` field to `True` to enable metrics and traces.
   - Configure the endpoints for the collector service to export the metrics and traces.
   - Set the healthcheck endpoint, which is configured in the collector service. Avoid including `http://` in the endpoints. Refer to the `example_config.yaml` file for guidance.

2. **Example Configuration Block for OTEL:**
   ```yaml
      # Enable the health check in this proxy application config only if the
      # "health_check" extension is added to the OTEL collector service configuration.
      #
      # Recommendation:
      # Enable the OTEL health check if you need to verify the collector's availability
      # at the start of the application. For development or testing environments, it can
      # be safely disabled to reduce complexity.
   otel:
       # Set enabled to true or false for OTEL metrics and traces
       enabled: True
       # Name of the collector service to be setup as a sidecar
       serviceName: YOUR_OTEL_COLLECTOR_SERVICE_NAME
       healthcheck:
           # Enable/Disable Health Check for OTEL, Default 'False'.
           enabled: False
           # Health check endpoint for the OTEL collector service
           endpoint: YOUR_OTEL_COLLECTOR_HEALTHCHECK_ENDPOINT
       metrics:
           # Collector service endpoint
           endpoint: YOUR_OTEL_COLLECTOR_SERVICE_ENDPOINT
       traces:
           # Collector service endpoint
           endpoint: YOUR_OTEL_COLLECTOR_SERVICE_ENDPOINT
           # Sampling ratio should be between 0 and 1.
           samplingRatio: YOUR_SAMPLING_RATIO


### Step 3: Setup Proxy Adaptor & OTEL Collector service as a sidecar on GKE.

1. Use the `proxy-adapter-application-as-sidecar.yaml` file from the deployment/sidecar-k8 to setup the otel collector service along with proxy adaptor as a sidecar. You can find step by step instructions in the `/deployment/sidecar-k8/README.md` file.


### Step 4: Verify OTEL changes

1. Verify the Health Check:

Run a curl command from the k8 pod to http://collector-ip:13133/health_check. You should see a health status message.

2. Check OTEL traces on GCP

- Login to your GCP project and open Monitoring service.
- Open the sidebar on the left and click on Trace Explorer
- You should be able to see the traces as shown below:

![Alt text](./img/traces-execute.png)
![Alt text](./img/traces-batch.png)

3. Check OTEL metrics on GCP

- Login to your GCP project and open Monitoring service.
- Open the sidebar on the left and click on Metrics Explorer
- You should be able to see the categories as shown below:

![Alt text](./img/metrics-category.png)

- Select options `Prometheus Target/Bigtable`
- To view the metrics for total number of requests, select option
`prometheus/bigtable_cassandra_adapter_request_count_total/counter`

![Alt text](./img/metrics_total_requests.png)

- To view the metrics for latency, select option 
`prometheus/bigtable_cassandra_adapter_roundtrip_latencies_milliseconds/histogram`

![Alt text](./img/metrics-latency.png)

- You can also view other metrics related to Bigtable library under the `Prometheus Target/Bigtable` category.

