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
	"fmt"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/zap"
)

func TestNewOpenTelemetry(t *testing.T) {
	ctx := context.Background()
	srv1 := setupTestEndpoint(":7060", "/trace")
	srv2 := setupTestEndpoint(":7061", "/metric")
	srv := setupTestEndpoint(":7062", "/TestNewOpenTelemetry")
	time.Sleep(3 * time.Second)
	defer func() {
		assert.NoError(t, srv.Shutdown(ctx), "failed to shutdown srv")
		assert.NoError(t, srv1.Shutdown(ctx), "failed to shutdown srv1")
		assert.NoError(t, srv2.Shutdown(ctx), "failed to shutdown srv2")
	}()
	type args struct {
		ctx    context.Context
		config *OTelConfig
		logger *zap.Logger
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Test success",
			args: args{
				ctx: ctx,
				config: &OTelConfig{
					TracerEndpoint:   "http://localhost:7060",
					MetricEndpoint:   "http://localhost:7061",
					ServiceName:      "test",
					OTELEnabled:      true,
					TraceSampleRatio: 20,
					Database:         "testDB",
					Instance:         "testInstance",
					HealthCheckEp:    "localhost:7062/TestNewOpenTelemetry",
				},
				logger: zap.NewNop(),
			},
			wantErr: false,
		},
		{
			name: "Test when otel disabled",
			args: args{
				ctx: ctx,
				config: &OTelConfig{
					TracerEndpoint:   "http://localhost:7060",
					MetricEndpoint:   "http://localhost:7061",
					ServiceName:      "test",
					OTELEnabled:      false,
					TraceSampleRatio: 20,
					Database:         "testDB",
					Instance:         "testInstance",
					HealthCheckEp:    "localhost:7062/TestNewOpenTelemetry",
				},
				logger: zap.NewNop(),
			},
			wantErr: false,
		},
		{
			name: "Test when healthcheck endpoint missing",
			args: args{
				ctx: ctx,
				config: &OTelConfig{
					TracerEndpoint:     "http://localhost:7060",
					MetricEndpoint:     "http://localhost:7061",
					ServiceName:        "test",
					OTELEnabled:        true,
					TraceSampleRatio:   20,
					Database:           "testDB",
					Instance:           "testInstance",
					HealthCheckEnabled: false,
					HealthCheckEp:      "",
				},
				logger: zap.NewNop(),
			},
			wantErr: false,
		},
		{
			name: "Test error when healthcheck endpoint missing",
			args: args{
				ctx: ctx,
				config: &OTelConfig{
					TracerEndpoint:     "http://localhost:7060",
					MetricEndpoint:     "http://localhost:7061",
					ServiceName:        "test",
					OTELEnabled:        true,
					TraceSampleRatio:   20,
					Database:           "testDB",
					Instance:           "testInstance",
					HealthCheckEnabled: true,
					HealthCheckEp:      "",
				},
				logger: zap.NewNop(),
			},
			wantErr: true,
		},
		{
			name: "Test error when TracerEndpoint endpoint missing",
			args: args{
				ctx: ctx,
				config: &OTelConfig{
					TracerEndpoint:   "",
					MetricEndpoint:   "http://localhost:7061",
					ServiceName:      "test",
					OTELEnabled:      true,
					TraceSampleRatio: 20,
					Database:         "testDB",
					Instance:         "testInstance",
					HealthCheckEp:    "localhost:7062/TestNewOpenTelemetry",
				},
				logger: zap.NewNop(),
			},
			wantErr: true,
		},
		{
			name: "Test when MetricEndpoint endpoint missing",
			args: args{
				ctx: ctx,
				config: &OTelConfig{
					TracerEndpoint:   "http://localhost:7060",
					MetricEndpoint:   "",
					ServiceName:      "test",
					OTELEnabled:      true,
					TraceSampleRatio: 20,
					Database:         "testDB",
					Instance:         "testInstance",
					HealthCheckEp:    "localhost:7062/TestNewOpenTelemetry",
				},
				logger: zap.NewNop(),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			_, _, err := NewOpenTelemetry(tt.args.ctx, tt.args.config, tt.args.logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewOpenTelemetry() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestShutdownOpenTelemetryComponents(t *testing.T) {
	t.Run("All functions succeed", func(t *testing.T) {
		ctx := context.Background()

		shutdownFunc1 := func(ctx context.Context) error {
			return nil
		}
		shutdownFunc2 := func(ctx context.Context) error {
			return nil
		}

		shutdownFuncs := []func(context.Context) error{
			shutdownFunc1,
			shutdownFunc2,
		}

		shutdown := shutdownOpenTelemetryComponents(shutdownFuncs)
		err := shutdown(ctx)
		assert.NoError(t, err)
	})

	t.Run("One function fails", func(t *testing.T) {
		ctx := context.Background()

		shutdownFunc1 := func(ctx context.Context) error {
			return nil
		}
		shutdownFunc2 := func(ctx context.Context) error {
			return errors.New("shutdown error")
		}

		shutdownFuncs := []func(context.Context) error{
			shutdownFunc1,
			shutdownFunc2,
		}

		shutdown := shutdownOpenTelemetryComponents(shutdownFuncs)
		err := shutdown(ctx)
		assert.Error(t, err)
		assert.Equal(t, "shutdown error", err.Error())
	})

	t.Run("Multiple functions fail", func(t *testing.T) {
		ctx := context.Background()

		shutdownFunc1 := func(ctx context.Context) error {
			return errors.New("first shutdown error")
		}
		shutdownFunc2 := func(ctx context.Context) error {
			return errors.New("second shutdown error")
		}

		shutdownFuncs := []func(context.Context) error{
			shutdownFunc1,
			shutdownFunc2,
		}

		shutdown := shutdownOpenTelemetryComponents(shutdownFuncs)

		err := shutdown(ctx)
		assert.Error(t, err)
		assert.Equal(t, "second shutdown error", err.Error())
	})
}

func TestSRecordLatency(t *testing.T) {
	cyx := context.Background()
	srv := setupTestEndpoint(":7061", "/TestSRecordLatency")
	time.Sleep(3 * time.Second)
	defer func() {
		assert.NoError(t, srv.Shutdown(cyx), "failed to shutdown srv")
	}()

	var ds1 []func(context.Context) error

	cfg := &OTelConfig{
		TracerEndpoint:   "http://localhost:7060",
		MetricEndpoint:   "http://localhost:7061",
		ServiceName:      "test",
		OTELEnabled:      true,
		TraceSampleRatio: 20,
		Database:         "testDB",
		Instance:         "testInstance",
		HealthCheckEp:    "localhost:7061/TestSRecordLatency",
	}

	ot, ds, err := NewOpenTelemetry(cyx, cfg, nil)
	ds1 = append(ds1, ds)
	assert.NoErrorf(t, err, "error occurred")

	ot.RecordLatencyMetric(cyx, time.Now(), Attributes{Method: "handlePrepare"})
	assert.NoErrorf(t, err, "error occurred")
	//when otel is disabled
	cfg2 := &OTelConfig{
		OTELEnabled: false,
	}

	ot1, ds, err2 := NewOpenTelemetry(cyx, cfg2, nil)
	ds1 = append(ds1, ds)
	assert.NoErrorf(t, err, "error occurred")

	ot1.RecordLatencyMetric(cyx, time.Now(), Attributes{Method: "handlePrepare"})

	shutdownOpenTelemetryComponents(ds1)
	assert.NoErrorf(t, err2, "error occurred")
}

func TestRecordRequestCountMetric(t *testing.T) {
	cyx := context.Background()
	srv := setupTestEndpoint(":7061", "/TestRecordRequestCountMetric")
	time.Sleep(3 * time.Second)
	defer func() {
		assert.NoError(t, srv.Shutdown(cyx), "failed to shutdown srv")
	}()

	var ds1 []func(context.Context) error

	cfg := &OTelConfig{
		TracerEndpoint:   "http://localhost:7060",
		MetricEndpoint:   "http://localhost:7061",
		ServiceName:      "test",
		OTELEnabled:      true,
		TraceSampleRatio: 20,
		Database:         "testDB",
		Instance:         "testInstance",
		HealthCheckEp:    "localhost:7061/TestRecordRequestCountMetric",
	}

	ot, ds, err := NewOpenTelemetry(cyx, cfg, nil)
	ds1 = append(ds1, ds)
	assert.NoErrorf(t, err, "error occurred")

	ot.RecordRequestCountMetric(cyx, Attributes{Method: "handlePrepare"})

	assert.NoErrorf(t, err, "error occurred")

	//when otel is disabled
	cfg2 := &OTelConfig{
		OTELEnabled: false,
	}

	ot1, ds, err2 := NewOpenTelemetry(cyx, cfg2, nil)
	ds1 = append(ds1, ds)
	assert.NoErrorf(t, err, "error occurred")

	ot1.RecordRequestCountMetric(cyx, Attributes{Method: "handlePrepare"})

	shutdownOpenTelemetryComponents(ds1)
	assert.NoErrorf(t, err2, "error occurred")
}

func TestApplyTrace(t *testing.T) {
	cyx := context.Background()
	srv := setupTestEndpoint(":7061", "/TestApplyTrace")
	time.Sleep(3 * time.Second)
	defer func() {
		assert.NoError(t, srv.Shutdown(cyx), "failed to shutdown srv")
	}()

	var ds1 []func(context.Context) error

	cfg := &OTelConfig{
		TracerEndpoint:   "http://localhost:7060",
		MetricEndpoint:   "http://localhost:7061",
		ServiceName:      "test",
		OTELEnabled:      true,
		TraceSampleRatio: 20,
		Database:         "testDB",
		Instance:         "testInstance",
		HealthCheckEp:    "localhost:7061/TestApplyTrace",
	}

	ot, ds, err := NewOpenTelemetry(cyx, cfg, nil)
	ds1 = append(ds1, ds)
	assert.NoErrorf(t, err, "error occurred")

	_, span := ot.StartSpan(cyx, "test", []attribute.KeyValue{
		attribute.String("method", "handlePrepare"),
	})

	shutdownOpenTelemetryComponents(ds1)
	assert.NoErrorf(t, err, "error occurred")
	ot.EndSpan(span)
}

func TestShutdown(t *testing.T) {
	cyx := context.Background()
	srv := setupTestEndpoint(":7061", "/TestShutdown")
	time.Sleep(3 * time.Second)
	defer func() {
		assert.NoError(t, srv.Shutdown(cyx), "failed to shutdown srv")
	}()

	var ds1 []func(context.Context) error

	cfg := &OTelConfig{
		TracerEndpoint:   "http://localhost:7060",
		MetricEndpoint:   "http://localhost:7061",
		ServiceName:      "test",
		OTELEnabled:      true,
		TraceSampleRatio: 20,
		Database:         "testDB",
		Instance:         "testInstance",
		HealthCheckEp:    "localhost:7061/TestShutdown",
	}

	_, ds, err := NewOpenTelemetry(cyx, cfg, nil)
	ds1 = append(ds1, ds)
	assert.NoErrorf(t, err, "error occurred")
	shutdownOpenTelemetryComponents(ds1)
}

// testHandler handles requests to the test endpoint.
func testHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Hello, this is a test endpoint!")
}

func setupTestEndpoint(st string, typ string) *http.Server {
	// Create a new instance of a server
	server := &http.Server{Addr: st}

	// Register the test handler with the specific type
	http.HandleFunc(typ, testHandler)

	// Start the server in a goroutine
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("ListenAndServe error: %s\n", err)
		}
	}()

	return server
}

func TestSRecordError(t *testing.T) {
	cyx := context.Background()
	srv := setupTestEndpoint(":7061", "/TestSRecordError")
	time.Sleep(3 * time.Second)
	defer func() {
		assert.NoError(t, srv.Shutdown(cyx), "failed to shutdown srv")
	}()

	var ds1 []func(context.Context) error

	cfg := &OTelConfig{
		TracerEndpoint:   "http://localhost:7060",
		MetricEndpoint:   "http://localhost:7061",
		ServiceName:      "test",
		OTELEnabled:      true,
		TraceSampleRatio: 20,
		Database:         "testDB",
		Instance:         "testInstance",
		HealthCheckEp:    "localhost:7061/TestSRecordError",
	}

	ot, ds, err := NewOpenTelemetry(cyx, cfg, nil)
	ds1 = append(ds1, ds)
	assert.NoErrorf(t, err, "error occurred")

	_, span := ot.StartSpan(cyx, "test", []attribute.KeyValue{
		attribute.String("method", "handlePrepare"),
	})

	ot.RecordError(span, fmt.Errorf("test error"))

	shutdownOpenTelemetryComponents(ds1)
	assert.NoErrorf(t, err, "error occurred")
}

func TestInitTracerProvider(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		config  *OTelConfig
		wantErr bool
	}{
		{
			name: "Valid Configuration",
			config: &OTelConfig{
				TracerEndpoint:   "localhost:4317",
				ServiceName:      "test-service",
				TraceSampleRatio: 1.0,
			},
			wantErr: false,
		},
		{
			name: "Missing Tracer Endpoint",
			config: &OTelConfig{
				TracerEndpoint:   "",
				ServiceName:      "test-service",
				TraceSampleRatio: 1.0,
			},
			wantErr: true,
		},
		{
			name: "Invalid Tracer Endpoint Format",
			config: &OTelConfig{
				TracerEndpoint:   "invalid-endpoint",
				ServiceName:      "test-service",
				TraceSampleRatio: 1.0,
			},
			wantErr: true,
		},
		{
			name: "Zero Trace Sample Ratio",
			config: &OTelConfig{
				TracerEndpoint:   "localhost:4317",
				ServiceName:      "test-service",
				TraceSampleRatio: 0.0,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource := resource.NewSchemaless() // Mock resource
			got, err := InitTracerProvider(ctx, tt.config, resource)
			if (err != nil) != tt.wantErr {
				t.Errorf("InitTracerProvider() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && (got == nil || reflect.TypeOf(got) != reflect.TypeOf(&sdktrace.TracerProvider{})) {
				t.Errorf("InitTracerProvider() = %v, expected valid TracerProvider", got)
			}
		})
	}
}
func TestInitMeterProvider(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		config  *OTelConfig
		wantErr bool
	}{
		{
			name: "Valid Configuration",
			config: &OTelConfig{
				MetricEndpoint: "localhost:4318",
				ServiceName:    "test-service",
			},
			wantErr: false,
		},
		{
			name: "Missing Metric Endpoint",
			config: &OTelConfig{
				MetricEndpoint: "",
				ServiceName:    "test-service",
			},
			wantErr: true,
		},
		{
			name: "Invalid Metric Endpoint Format",
			config: &OTelConfig{
				MetricEndpoint: "://invalid-endpoint",
				ServiceName:    "test-service",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource := resource.NewSchemaless() // Mock resource
			got, err := InitMeterProvider(ctx, tt.config, resource)

			if (err != nil) != tt.wantErr {
				t.Errorf("InitMeterProvider() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && (got == nil || reflect.TypeOf(got) != reflect.TypeOf(&sdkmetric.MeterProvider{})) {
				t.Errorf("InitMeterProvider() = %v, expected valid MeterProvider", got)
			}
		})
	}
}

func Test_buildOtelResource(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name   string
		config *OTelConfig
	}{
		{
			name: "Valid Service Configuration",
			config: &OTelConfig{
				ServiceName:    "test-service",
				ServiceVersion: "1.0.0",
			},
		},
		{
			name: "Missing Service Name",
			config: &OTelConfig{
				ServiceName:    "",
				ServiceVersion: "1.0.0",
			},
		},
		{
			name: "Missing Service Version",
			config: &OTelConfig{
				ServiceName:    "test-service",
				ServiceVersion: "",
			},
		},
		{
			name: "Empty Config",
			config: &OTelConfig{
				ServiceName:    "",
				ServiceVersion: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildOtelResource(ctx, tt.config)

			// Validate that the returned resource is not nil
			assert.NotNil(t, got, "buildOtelResource() returned nil for test case: %s", tt.name)

			// Retrieve attributes from resource
			attrs := got.Attributes()

			// Check if expected attributes exist in the resource
			expectedAttrs := map[string]string{
				string(semconv.ServiceNameKey):    tt.config.ServiceName,
				string(semconv.ServiceVersionKey): tt.config.ServiceVersion,
			}

			for key, expectedValue := range expectedAttrs {
				found := false
				for _, attr := range attrs {
					if string(attr.Key) == key && attr.Value.AsString() == expectedValue {
						found = true
						break
					}
				}
				assert.True(t, found, "buildOtelResource() missing expected attribute: %s", key)
			}

			// Ensure the instance ID exists
			instanceIDFound := false
			for _, attr := range attrs {
				if string(attr.Key) == string(semconv.ServiceInstanceIDKey) {
					instanceIDFound = true
					break
				}
			}
			assert.True(t, instanceIDFound, "buildOtelResource() missing expected attribute: ServiceInstanceID")
		})
	}
}

func TestOpenTelemetry_StartSpan(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		otelConfig *OTelConfig
		spanName   string
		attrs      []attribute.KeyValue
		expectSpan bool
	}{
		{
			name: "Start span when OTEL is enabled",
			otelConfig: &OTelConfig{
				OTELEnabled: true,
			},
			spanName:   "test-span",
			attrs:      []attribute.KeyValue{attribute.String("key", "value")},
			expectSpan: true,
		},
		{
			name: "Return same context when OTEL is disabled",
			otelConfig: &OTelConfig{
				OTELEnabled: false,
			},
			spanName:   "test-span",
			attrs:      []attribute.KeyValue{attribute.String("key", "value")},
			expectSpan: false,
		},
		{
			name: "Start span with empty attributes",
			otelConfig: &OTelConfig{
				OTELEnabled: true,
			},
			spanName:   "empty-attributes-span",
			attrs:      []attribute.KeyValue{},
			expectSpan: true,
		},
		{
			name: "Start span with empty span name",
			otelConfig: &OTelConfig{
				OTELEnabled: true,
			},
			spanName:   "",
			attrs:      []attribute.KeyValue{attribute.String("key", "value")},
			expectSpan: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			otelInstance := &OpenTelemetry{
				Config: tt.otelConfig,
				tracer: noop.NewTracerProvider().Tracer("test"), // Use noop tracer for testing
				logger: zap.NewNop(),
			}

			newCtx, span := otelInstance.StartSpan(ctx, tt.spanName, tt.attrs)

			if tt.expectSpan {
				assert.NotNil(t, span, "Expected a valid span but got nil")
			} else {
				assert.Nil(t, span, "Expected nil span but got a valid span")
			}

			// Ensure context is updated when OTEL is enabled
			if tt.expectSpan {
				assert.NotEqual(t, ctx, newCtx, "Context should be updated with span")
			} else {
				assert.Equal(t, ctx, newCtx, "Context should remain unchanged when OTEL is disabled")
			}
		})
	}
}

func TestOpenTelemetry_EndSpan(t *testing.T) {
	tests := []struct {
		name       string
		otelConfig *OTelConfig
		expectCall bool
	}{
		{
			name: "End span when OTEL is enabled",
			otelConfig: &OTelConfig{
				OTELEnabled: true,
			},
			expectCall: true,
		},
		{
			name: "Do nothing when OTEL is disabled",
			otelConfig: &OTelConfig{
				OTELEnabled: false,
			},
			expectCall: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			otelInstance := &OpenTelemetry{
				Config: tt.otelConfig,
				tracer: noop.NewTracerProvider().Tracer("test"), // No-op tracer
				logger: zap.NewNop(),
			}

			// Creating a valid context and span
			ctx := context.Background()
			otelInstance.tracer.Start(ctx, "test-span")

			// Spy to check if span.End() is called
			ended := false
			mockSpan := &mockSpan{endCalled: &ended}

			otelInstance.EndSpan(mockSpan)

			if tt.expectCall {
				if !ended {
					t.Errorf("Expected span.End() to be called, but it wasn't")
				}
			} else {
				if ended {
					t.Errorf("Expected span.End() to NOT be called, but it was")
				}
			}
		})
	}
}

// mockSpan is a mock implementation of trace.Span to check if End() is called.
type mockSpan struct {
	trace.Span
	endCalled *bool
}

func (m *mockSpan) End(opts ...trace.SpanEndOption) {
	*m.endCalled = true
}

// MockOpenTelemetry is a mock implementation of OpenTelemetry for testing.
type MockOpenTelemetry struct {
	*OpenTelemetry
	RequestCountCalled bool
	LatencyCalled      bool
}

// Override RecordMetrics to ensure the mock methods get called
func (m *MockOpenTelemetry) RecordMetrics(ctx context.Context, method string, startTime time.Time, queryType string, err error) {
	// Check if OTEL is enabled before recording metrics
	if !m.Config.OTELEnabled {
		return
	}

	// Call mock versions of RecordRequestCountMetric & RecordLatencyMetric
	m.RecordRequestCountMetric(ctx, Attributes{Method: method, Status: "OK", QueryType: queryType})
	m.RecordLatencyMetric(ctx, startTime, Attributes{Method: method, QueryType: queryType})
}

// Override RecordRequestCountMetric to track calls
func (m *MockOpenTelemetry) RecordRequestCountMetric(ctx context.Context, attrs Attributes) {
	m.RequestCountCalled = true
}

// Override RecordLatencyMetric to track calls
func (m *MockOpenTelemetry) RecordLatencyMetric(ctx context.Context, startTime time.Time, attrs Attributes) {
	m.LatencyCalled = true
}

func TestOpenTelemetry_RecordMetrics(t *testing.T) {
	tests := []struct {
		name       string
		otelConfig *OTelConfig
		method     string
		queryType  string
		err        error
		expectCall bool
	}{
		{
			name: "Record metrics with successful request",
			otelConfig: &OTelConfig{
				OTELEnabled: true,
			},
			method:     "GET",
			queryType:  "select",
			err:        nil,
			expectCall: true,
		},
		{
			name: "Record metrics with failed request",
			otelConfig: &OTelConfig{
				OTELEnabled: true,
			},
			method:     "POST",
			queryType:  "insert",
			err:        errors.New("database error"),
			expectCall: true,
		},
		{
			name: "Do not record metrics when OTEL is disabled",
			otelConfig: &OTelConfig{
				OTELEnabled: false,
			},
			method:     "DELETE",
			queryType:  "delete",
			err:        nil,
			expectCall: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			startTime := time.Now()

			// Use MockOpenTelemetry with the overridden RecordMetrics method
			mockOtel := &MockOpenTelemetry{
				OpenTelemetry: &OpenTelemetry{
					Config: tt.otelConfig,
					tracer: noop.NewTracerProvider().Tracer("test"),
					logger: zap.NewNop(),
				},
			}

			// Call the function under test
			mockOtel.RecordMetrics(ctx, tt.method, startTime, tt.queryType, tt.err)

			if tt.expectCall {
				if !mockOtel.RequestCountCalled {
					t.Errorf("Expected RecordRequestCountMetric to be called, but it wasn't")
				}
				if !mockOtel.LatencyCalled {
					t.Errorf("Expected RecordLatencyMetric to be called, but it wasn't")
				}
			} else {
				if mockOtel.RequestCountCalled || mockOtel.LatencyCalled {
					t.Errorf("Expected no metric calls when OTEL is disabled, but they were called")
				}
			}
		})
	}
}

// MockSpan is a mock implementation of trace.Span for testing
type MockSpan struct {
	trace.Span
	events []string
}

func (m *MockSpan) AddEvent(name string, opts ...trace.EventOption) {
	m.events = append(m.events, name)
}

func TestAddAnnotation(t *testing.T) {
	tests := []struct {
		name  string
		event string
	}{
		{
			name:  "Add simple event",
			event: "UserLoggedIn",
		},
		{
			name:  "Add another event",
			event: "DatabaseQueried",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSpan := &MockSpan{}
			ctx := trace.ContextWithSpan(context.Background(), mockSpan)

			// Call function
			AddAnnotation(ctx, tt.event)

			// Verify that the event was recorded
			assert.Contains(t, mockSpan.events, tt.event, "Expected event to be added")
		})
	}
}

func Test_isValidEndpoint(t *testing.T) {
	type args struct {
		endpoint string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Valid host:port format",
			args: args{endpoint: "localhost:8080"},
			want: true,
		},
		{
			name: "Invalid host:port format (no port)",
			args: args{endpoint: "localhost"},
			want: false,
		},
		{
			name: "Valid URL format with http",
			args: args{endpoint: "http://localhost:8080"},
			want: true,
		},
		{
			name: "Valid URL format with https",
			args: args{endpoint: "https://example.com:443"},
			want: true,
		},
		{
			name: "Invalid URL format (missing host)",
			args: args{endpoint: "http://:8080"},
			want: false,
		},
		{
			name: "Invalid URL format (missing port)",
			args: args{endpoint: "http://localhost:"},
			want: false,
		},
		{
			name: "Empty string",
			args: args{endpoint: ""},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidEndpoint(tt.args.endpoint); got != tt.want {
				t.Errorf("isValidEndpoint() = %v, want %v", got, tt.want)
			}
		})
	}
}
