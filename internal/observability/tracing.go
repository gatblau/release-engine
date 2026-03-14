// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package observability

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// Tracing provides OpenTelemetry tracing capabilities.
type Tracing interface {
	Tracer(name string) trace.Tracer
	Shutdown(ctx context.Context) error
}

// TracingService implements the Tracing interface for OpenTelemetry.
type TracingService struct {
	logger      *zap.Logger
	tracer      trace.Tracer
	provider    *sdktrace.TracerProvider
	serviceName string
	environment string
	version     string
	sampleRatio float64
	shutdownFn  func(ctx context.Context) error
}

// TracingConfig holds configuration for the tracing service.
type TracingConfig struct {
	ServiceName  string
	Environment  string
	Version      string
	OTLPEndpoint string
	SampleRatio  float64
	Insecure     bool
}

// Option is a functional option for configuring TracingService.
type Option func(*TracingService) error

// WithTracerProvider sets a custom tracer provider for testing.
func WithTracerProvider(provider *sdktrace.TracerProvider) Option {
	return func(ts *TracingService) error {
		ts.provider = provider
		ts.tracer = provider.Tracer(ts.serviceName)
		ts.shutdownFn = provider.Shutdown
		return nil
	}
}

// WithNoopProvider creates a no-op tracer provider for testing without OTLP endpoint.
func WithNoopProvider() Option {
	return func(ts *TracingService) error {
		provider := sdktrace.NewTracerProvider()
		ts.provider = provider
		ts.tracer = provider.Tracer(ts.serviceName)
		ts.shutdownFn = func(ctx context.Context) error {
			return nil // Noop shutdown
		}
		return nil
	}
}

// NewTracingService creates a new TracingService.
// Implements Phase 3: TracingService spec - initialise OpenTelemetry tracing.
// Use WithNoopProvider() option for testing without an OTLP endpoint.
func NewTracingService(logger *zap.Logger, config *TracingConfig, opts ...Option) (*TracingService, error) {
	if config == nil {
		config = &TracingConfig{}
	}

	if config.ServiceName == "" {
		config.ServiceName = "release-engine"
	}
	if config.Environment == "" {
		config.Environment = "development"
	}
	if config.Version == "" {
		config.Version = "0.0.0"
	}
	if config.OTLPEndpoint == "" {
		config.OTLPEndpoint = "localhost:4317"
	}
	if config.SampleRatio <= 0 {
		config.SampleRatio = 0.1 // 10% steady-state sampling
	}

	// Create a basic service first
	ts := &TracingService{
		logger:      logger,
		serviceName: config.ServiceName,
		environment: config.Environment,
		version:     config.Version,
		sampleRatio: config.SampleRatio,
	}

	// Apply options first - if WithNoopProvider or WithTracerProvider is used,
	// we skip OTLP initialization
	for _, opt := range opts {
		if err := opt(ts); err != nil {
			return nil, err
		}
	}

	// If provider is already set by options, we're done
	if ts.provider != nil {
		logger.Info("tracing.start",
			zap.String("component", "TracingService"),
			zap.String("service", config.ServiceName),
			zap.String("environment", config.Environment),
			zap.Float64("sample_ratio", config.SampleRatio),
			zap.String("endpoint", "noop"))
		return ts, nil
	}

	// Create OTLP exporter
	ctx := context.Background()

	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(config.OTLPEndpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		logger.Error("tracing.init.failure",
			zap.String("component", "TracingService"),
			zap.Error(err))
		return nil, NewObservabilityError(
			ErrTracingInitFailed,
			"TRACING_INIT_FAILED",
			map[string]string{"error": err.Error()},
		)
	}

	// Create resource with service metadata
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(config.ServiceName),
			semconv.ServiceVersionKey.String(config.Version),
			semconv.DeploymentEnvironmentKey.String(config.Environment),
		),
	)
	if err != nil {
		logger.Warn("tracing.resource.warning",
			zap.String("component", "TracingService"),
			zap.Error(err))
		// Continue with default resource
		res = resource.Default()
	}

	// Create tracer provider with sampling
	// 100% sampling during bootstrap, then configured ratio
	sampler := sdktrace.ParentBased(
		sdktrace.TraceIDRatioBased(config.SampleRatio),
	)

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	// Set global tracer provider
	otel.SetTracerProvider(provider)

	// Set text map propagator for W3C trace context
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	ts.provider = provider
	ts.tracer = provider.Tracer(config.ServiceName)
	ts.shutdownFn = provider.Shutdown

	logger.Info("tracing.start",
		zap.String("component", "TracingService"),
		zap.String("service", config.ServiceName),
		zap.String("environment", config.Environment),
		zap.Float64("sample_ratio", config.SampleRatio),
		zap.String("endpoint", config.OTLPEndpoint))

	return ts, nil
}

// Tracer returns a tracer for the given name.
// Implements Phase 3: TracingService spec - provide tracer instances to components.
func (t *TracingService) Tracer(name string) trace.Tracer {
	return t.provider.Tracer(name)
}

// Shutdown gracefully shuts down the tracing service.
// Implements Phase 3: TracingService spec - ensure trace context propagation.
func (t *TracingService) Shutdown(ctx context.Context) error {
	// Create a timeout context for shutdown
	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Flush any pending spans
	if err := t.provider.ForceFlush(shutdownCtx); err != nil {
		t.logger.Warn("tracing.flush.warning",
			zap.String("component", "TracingService"),
			zap.Error(err))
	}

	// Shutdown the provider
	if err := t.shutdownFn(shutdownCtx); err != nil {
		t.logger.Error("tracing.failure",
			zap.String("component", "TracingService"),
			zap.Error(err))
		return NewObservabilityError(
			ErrTracingFlushTimeout,
			"TRACING_FLUSH_TIMEOUT",
			map[string]string{"error": err.Error()},
		)
	}

	t.logger.Info("tracing.success",
		zap.String("component", "TracingService"))

	return nil
}

// StartSpan starts a new span with the given name and options.
func (t *TracingService) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, name, opts...)
}

// GetServiceName returns the service name.
func (t *TracingService) GetServiceName() string {
	return t.serviceName
}

// GetEnvironment returns the environment.
func (t *TracingService) GetEnvironment() string {
	return t.environment
}

// GetVersion returns the version.
func (t *TracingService) GetVersion() string {
	return t.version
}

// WithSpan runs the provided function within a new span.
func (t *TracingService) WithSpan(ctx context.Context, name string, fn func(ctx context.Context) error) error {
	ctx, span := t.StartSpan(ctx, name)
	defer span.End()

	if err := fn(ctx); err != nil {
		span.RecordError(err)
		return fmt.Errorf("span %s failed: %w", name, err)
	}

	return nil
}
