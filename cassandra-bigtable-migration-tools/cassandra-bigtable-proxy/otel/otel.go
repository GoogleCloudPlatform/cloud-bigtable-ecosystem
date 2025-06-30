/*
 * Copyright (C) 2025 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not
 * use this file except in compliance with the License. You may obtain a copy of
 * the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
 * WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
 * License for the specific language governing permissions and limitations under
 * the License.
 */

package otelgo

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/contrib/detectors/gcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type Attributes struct {
	Method    string
	Status    string
	QueryType string
	Keyspace  string
}

var (
	attributeKeyDatabase  = attribute.Key("database")
	attributeKeyMethod    = attribute.Key("method")
	attributeKeyStatus    = attribute.Key("status")
	attributeKeyInstance  = attribute.Key("instance")
	attributeKeyQueryType = attribute.Key("query_type")
)

// OTelConfig holds configuration for OpenTelemetry.
type OTelConfig struct {
	TracerEndpoint     string
	MetricEndpoint     string
	ServiceName        string
	TraceSampleRatio   float64
	OTELEnabled        bool
	Database           string
	Instance           string
	HealthCheckEnabled bool
	HealthCheckEp      string
	ServiceVersion     string
}

const (
	requestCountMetric = "bigtable/cassandra_adapter/request_count"
	latencyMetric      = "bigtable/cassandra_adapter/roundtrip_latencies"
)

// OpenTelemetry provides methods to setup tracing and metrics.
type OpenTelemetry struct {
	Config         *OTelConfig
	tracer         trace.Tracer
	requestCount   metric.Int64Counter
	requestLatency metric.Int64Histogram
	logger         *zap.Logger
}

// NewOpenTelemetry() initializes OpenTelemetry tracing and metrics components.
// It sets up the tracer and meter providers, configures health checks (if enabled),
// and returns an OpenTelemetry instance along with a shutdown function.
//
// Parameters:
//   - ctx: Context for managing OpenTelemetry lifecycle.
//   - config: Configuration struct for OpenTelemetry settings.
//   - logger: Logger instance for capturing OpenTelemetry logs.
//
// Returns:
//   - *OpenTelemetry: A configured instance of OpenTelemetry.
//   - func(context.Context) error: A shutdown function to clean up resources.
//   - error: An error if initialization fails.
func NewOpenTelemetry(ctx context.Context, config *OTelConfig, logger *zap.Logger) (*OpenTelemetry, func(context.Context) error, error) {
	otelInst := &OpenTelemetry{Config: config, logger: logger}
	var err error
	otelInst.Config.OTELEnabled = config.OTELEnabled
	if !config.OTELEnabled {
		return otelInst, nil, nil
	}

	if config.HealthCheckEnabled {
		resp, err := http.Get("http://" + config.HealthCheckEp)
		if err != nil {
			return nil, nil, err
		}
		if resp.StatusCode != 200 {
			return nil, nil, errors.New("OTEL collector service is not up and running")
		}
		logger.Info("OTEL health check complete")
	}
	var shutdownFuncs []func(context.Context) error
	otelResource := buildOtelResource(ctx, config)

	// Initialize tracerProvider
	tracerProvider, err := InitTracerProvider(ctx, config, otelResource)
	if err != nil {
		logger.Error("error while initializing the tracer provider", zap.Error(err))
		return nil, nil, err
	}
	otel.SetTracerProvider(tracerProvider)
	otelInst.tracer = tracerProvider.Tracer(config.ServiceName)
	shutdownFuncs = append(shutdownFuncs, tracerProvider.Shutdown)

	// Initialize MeterProvider
	meterProvider, err := InitMeterProvider(ctx, config, otelResource)
	if err != nil {
		logger.Error("error while initializing the meter provider", zap.Error(err))
		return nil, nil, err
	}
	otel.SetMeterProvider(meterProvider)
	meter := meterProvider.Meter(config.ServiceName)
	shutdownFuncs = append(shutdownFuncs, meterProvider.Shutdown)
	shutdown := shutdownOpenTelemetryComponents(shutdownFuncs)
	otelInst.requestCount, err = meter.Int64Counter(requestCountMetric, metric.WithDescription("Records metric for number of query requests coming in"), metric.WithUnit("1"))
	if err != nil {
		logger.Error("error during registering instrument for metric bigtable/cassandra_adapter/request_count", zap.Error(err))
		return nil, nil, err
	}
	otelInst.requestLatency, err = meter.Int64Histogram(latencyMetric,
		metric.WithDescription("Records latency for all query operations"),
		metric.WithExplicitBucketBoundaries(0.0, 0.0010, 0.0013, 0.0016, 0.0020, 0.0024, 0.0031, 0.0038, 0.0048, 0.0060,
			0.0075, 0.0093, 0.0116, 0.0146, 0.0182, 0.0227, 0.0284, 0.0355, 0.0444, 0.0555, 0.0694, 0.0867,
			0.1084, 0.1355, 0.1694, 0.2118, 0.2647, 0.3309, 0.4136, 0.5170, 0.6462, 0.8078, 1.0097, 1.2622,
			1.5777, 1.9722, 2.4652, 3.0815, 3.8519, 4.8148, 6.0185, 7.5232, 9.4040, 11.7549, 14.6937, 18.3671,
			22.9589, 28.6986, 35.8732, 44.8416, 56.0519, 70.0649, 87.5812, 109.4764, 136.8456, 171.0569, 213.8212,
			267.2765, 334.0956, 417.6195, 522.0244, 652.5304),
		metric.WithUnit("ms"))
	if err != nil {
		logger.Error("error during registering instrument for metric bigtable/cassandra_adapter/roundtrip_latencies", zap.Error(err))
		return nil, nil, err
	}
	return otelInst, shutdown, nil
}

// shutdownOpenTelemetryComponents() aggregates multiple shutdown functions into a single callable function.
// It iterates over all shutdown functions, executing them sequentially.
//
// Parameters:
//   - shutdownFuncs: A slice of shutdown functions for OpenTelemetry components.
//
// Returns:
//   - func(context.Context) error: A single shutdown function that cleans up all initialized components.
func shutdownOpenTelemetryComponents(shutdownFuncs []func(context.Context) error) func(context.Context) error {
	return func(ctx context.Context) error {
		var shutdownErr error
		for _, shutdownFunc := range shutdownFuncs {
			if err := shutdownFunc(ctx); err != nil {
				shutdownErr = err
			}
		}
		return shutdownErr
	}
}

// InitTracerProvider() configures and initializes an OpenTelemetry TracerProvider.
// It sets up a gRPC-based OTLP trace exporter and applies the sampling strategy.
//
// Parameters:
//   - ctx: Context for managing initialization.
//   - config: OpenTelemetry configuration settings.
//   - resource: OpenTelemetry resource with metadata.
//
// Returns:
//   - *sdktrace.TracerProvider: A configured TracerProvider instance.
//   - error: An error if initialization fails.
func InitTracerProvider(ctx context.Context, config *OTelConfig, resource *resource.Resource) (*sdktrace.TracerProvider, error) {
	sampler := sdktrace.TraceIDRatioBased(config.TraceSampleRatio)
	if config.TracerEndpoint == "" {
		return nil, errors.New("tracer endpoint cannot be empty")
	}
	// Basic validation for incorrect endpoint format
	if !isValidEndpoint(config.TracerEndpoint) {
		return nil, errors.New("invalid tracer endpoint format")
	}

	traceExporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(config.TracerEndpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(resource),
		sdktrace.WithSampler(sdktrace.ParentBased(sampler)),
	)
	return tp, nil
}

// InitMeterProvider() initializes an OpenTelemetry MeterProvider for collecting application metrics.
// It configures a gRPC exporter to send metrics data and applies filtering to exclude unnecessary gRPC metrics.
//
// Parameters:
//   - ctx: Context for managing initialization.
//   - config: OpenTelemetry configuration settings.
//   - resource: OpenTelemetry resource with metadata.
//
// Returns:
//   - *sdkmetric.MeterProvider: A configured MeterProvider instance.
//   - error: An error if initialization fails.
func InitMeterProvider(ctx context.Context, config *OTelConfig, resource *resource.Resource) (*sdkmetric.MeterProvider, error) {
	if config.MetricEndpoint == "" {
		return nil, errors.New("metric endpoint cannot be empty")
	}

	// Basic validation for incorrect endpoint format
	if !isValidEndpoint(config.MetricEndpoint) {
		return nil, errors.New("invalid tracer endpoint format")
	}
	var views []sdkmetric.View
	me, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithEndpoint(config.MetricEndpoint),
		otlpmetricgrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	// Define views to filter out unwanted gRPC metrics
	views = []sdkmetric.View{
		sdkmetric.NewView(
			sdkmetric.Instrument{Name: "rpc.client.*"},                 // Wildcard pattern to match gRPC client metrics
			sdkmetric.Stream{Aggregation: sdkmetric.AggregationDrop{}}, // Drop these metrics
		)}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(me)),
		sdkmetric.WithResource(resource),
		sdkmetric.WithView(views...),
	)
	return mp, nil
}

// buildOtelResource() creates an OpenTelemetry resource containing metadata about the service.
// It uses GCP resource detectors and falls back to manually provided attributes if necessary.
//
// Parameters:
//   - ctx: Context for managing initialization.
//   - config: OpenTelemetry configuration settings.
//
// Returns:
//   - *resource.Resource: A configured OpenTelemetry resource containing metadata.
func buildOtelResource(ctx context.Context, config *OTelConfig) *resource.Resource {
	res, err := resource.New(ctx,
		resource.WithSchemaURL(semconv.SchemaURL),
		// Use the GCP resource detector!
		resource.WithDetectors(gcp.NewDetector()),
		// Keep the default detectors
		resource.WithTelemetrySDK(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(config.ServiceName),
			semconv.ServiceInstanceIDKey.String(uuid.New().String()),
			semconv.ServiceVersionKey.String(config.ServiceVersion),
		),
	)

	if err != nil {
		// Default resource
		return resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(config.ServiceName),
			semconv.ServiceInstanceIDKey.String(uuid.New().String()),
			semconv.ServiceVersionKey.String(config.ServiceVersion),
		)
	}

	return res

}

// StartSpan() creates and starts a new trace span in OpenTelemetry.
// If OpenTelemetry is disabled, it returns the original context.
//
// Parameters:
//   - ctx: The current execution context.
//   - name: The name of the span to be created.
//   - attrs: A list of attributes to associate with the span.
//
// Returns:
//   - context.Context: The updated context containing the new span.
//   - trace.Span: The created span instance.
func (o *OpenTelemetry) StartSpan(ctx context.Context, name string, attrs []attribute.KeyValue) (context.Context, trace.Span) {
	if !o.Config.OTELEnabled {
		return ctx, nil
	}

	ctx, span := o.tracer.Start(ctx, name, trace.WithAttributes(attrs...))
	return ctx, span
}

// RecordError() logs an error inside an active trace span in OpenTelemetry.
// It updates the span's status to indicate an error has occurred.
//
// Parameters:
//   - span: The active trace span where the error should be recorded.
//   - err: The error to be recorded in the span. If nil, the span is marked as OK.
func (o *OpenTelemetry) RecordError(span trace.Span, err error) {
	if !o.Config.OTELEnabled {
		return
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}
}

// EndSpan() finalizes the current span in OpenTelemetry.
// If OpenTelemetry is disabled, this function does nothing.
//
// Parameters:
//   - span: The span to be ended.
func (o *OpenTelemetry) EndSpan(span trace.Span) {
	if !o.Config.OTELEnabled {
		return
	}

	span.End()
}

// RecordMetrics() records request count and latency metrics in OpenTelemetry.
// It determines whether the request was successful or failed based on the error parameter.
//
// Parameters:
//   - ctx: The execution context for OpenTelemetry.
//   - method: The name of the method being recorded.
//   - startTime: The start time of the request for latency calculation.
//   - queryType: The type of query being executed (e.g., "select", "insert").
//   - err: The error encountered, if any. Used to determine success/failure status.
func (o *OpenTelemetry) RecordMetrics(ctx context.Context, method string, startTime time.Time, queryType string, keyspace string, err error) {
	status := "OK"
	if err != nil {
		status = "failure"
	}
	o.RecordRequestCountMetric(ctx, Attributes{
		Method:    method,
		Status:    status,
		QueryType: queryType,
		Keyspace:  keyspace,
	})
	o.RecordLatencyMetric(ctx, startTime, Attributes{
		Method:    method,
		QueryType: queryType,
		Keyspace:  keyspace,
	})
}

// RecordLatencyMetric() records the latency of an operation in OpenTelemetry.
// It dynamically builds metric attributes before sending the recorded value.
//
// Parameters:
//   - ctx: The execution context.
//   - startTime: The time when the operation started, used for latency calculation.
//   - attrs: Additional attributes to associate with the latency metric.
func (o *OpenTelemetry) RecordLatencyMetric(ctx context.Context, startTime time.Time, attrs Attributes) {
	if !o.Config.OTELEnabled {
		return
	}

	// Build attributes dynamically
	attr := []attribute.KeyValue{
		attributeKeyInstance.String(attrs.Keyspace),
		attributeKeyDatabase.String(o.Config.Database),
		attributeKeyMethod.String(attrs.Method),
		attributeKeyQueryType.String(attrs.QueryType),
	}
	attr = append(attr, attributeKeyMethod.String(attrs.Method))
	attr = append(attr, attributeKeyQueryType.String(attrs.QueryType))
	o.requestLatency.Record(ctx, int64(time.Since(startTime).Milliseconds()), metric.WithAttributes(attr...))
}

// RecordRequestCountMetric() increments the request count metric in OpenTelemetry.
// It dynamically builds metric attributes before sending the recorded value.
//
// Parameters:
//   - ctx: The execution context.
//   - attrs: Attributes associated with the request (e.g., method, status).
func (o *OpenTelemetry) RecordRequestCountMetric(ctx context.Context, attrs Attributes) {
	if !o.Config.OTELEnabled {
		return
	}

	// Build attributes dynamically
	attr := []attribute.KeyValue{
		attributeKeyInstance.String(attrs.Keyspace),
		attributeKeyDatabase.String(o.Config.Database),
		attributeKeyMethod.String(attrs.Method),
		attributeKeyQueryType.String(attrs.QueryType),
		attributeKeyStatus.String(attrs.Status),
	}
	attr = append(attr, attributeKeyMethod.String(attrs.Method))
	attr = append(attr, attributeKeyQueryType.String(attrs.QueryType))
	attr = append(attr, attributeKeyStatus.String(attrs.Status))
	o.requestCount.Add(ctx, 1, metric.WithAttributes(attr...))
}

// AddAnnotation() adds an event annotation to the active span in the given context.
//
// Parameters:
//   - ctx: The execution context containing the span.
//   - event: The event name to be added as an annotation.
func AddAnnotation(ctx context.Context, event string) {
	span := trace.SpanFromContext(ctx)
	span.AddEvent(event)
}

// AddAnnotationWithAttr() adds an event annotation with attributes to the active span.
//
// Parameters:
//   - ctx: The execution context containing the span.
//   - event: The event name to be added as an annotation.
//   - attr: A list of attributes to attach to the annotation.
func AddAnnotationWithAttr(ctx context.Context, event string, attr []attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.AddEvent(event, trace.WithAttributes(attr...))
}

// isValidEndpoint checks if the given endpoint is a valid host:port format
func isValidEndpoint(endpoint string) bool {
	if strings.Contains(endpoint, "://") {
		parsedURL, err := url.Parse(endpoint)
		if err != nil {
			return false
		}
		// Check if the original endpoint string had an empty host.
		if strings.HasPrefix(endpoint, parsedURL.Scheme+"://:") {
			return false
		}
		if parsedURL.Host == "" || parsedURL.Port() == "" {
			return false
		}
		return true
	}

	parts := strings.Split(endpoint, ":")
	return len(parts) == 2 && parts[0] != "" && parts[1] != ""
}
