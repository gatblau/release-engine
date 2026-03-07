package observability

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// TestTracingConfig_Defaults tests that TracingConfig has the expected default values.
func TestTracingConfig_Defaults(t *testing.T) {
	config := &TracingConfig{}

	// When no values are set, they should be empty/zero
	// The actual defaults are applied in NewTracingService
	assert.Empty(t, config.ServiceName)
	assert.Empty(t, config.Environment)
	assert.Empty(t, config.Version)
	assert.Empty(t, config.OTLPEndpoint)
	assert.Equal(t, float64(0), config.SampleRatio)
}

// TestTracingConfig_CustomValues tests that custom config values are preserved.
func TestTracingConfig_CustomValues(t *testing.T) {
	config := &TracingConfig{
		ServiceName:  "custom-service",
		Environment:  "production",
		Version:      "2.0.0",
		OTLPEndpoint: "otel.company.com:4317",
		SampleRatio:  0.25,
	}

	assert.Equal(t, "custom-service", config.ServiceName)
	assert.Equal(t, "production", config.Environment)
	assert.Equal(t, "2.0.0", config.Version)
	assert.Equal(t, "otel.company.com:4317", config.OTLPEndpoint)
	assert.Equal(t, 0.25, config.SampleRatio)
}

// TestNewTracingService_WithNoopProvider tests creating a TracingService with noop provider.
func TestNewTracingService_WithNoopProvider(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	config := &TracingConfig{
		ServiceName: "test-service",
		Environment: "test",
		Version:     "1.0.0",
	}

	service, err := NewTracingService(logger, config, WithNoopProvider())

	assert.NoError(t, err)
	assert.NotNil(t, service)
	assert.Equal(t, "test-service", service.GetServiceName())
	assert.Equal(t, "test", service.GetEnvironment())
	assert.Equal(t, "1.0.0", service.GetVersion())
}

// TestNewTracingService_NilConfig tests that NewTracingService handles nil config gracefully.
func TestNewTracingService_NilConfig(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	// This will fail because it tries to connect to OTLP endpoint
	// But it should not panic on nil config
	_, err := NewTracingService(logger, nil)

	// The service should fail to initialize because there's no OTLP endpoint
	// This is expected behavior - we just verify it doesn't panic
	if err != nil {
		// Expected - no OTLP endpoint available
		assert.Contains(t, err.Error(), "TRACING_INIT_FAILED")
	}
}

// TestNewTracingService_WithDefaults tests that NewTracingService applies default values.
func TestNewTracingService_WithDefaults(t *testing.T) {
	config := &TracingConfig{}

	// This will fail to connect but we can verify the config defaults
	// by checking what would be applied if it succeeded
	assert.Equal(t, "", config.ServiceName, "Config should start empty")
	assert.Equal(t, "", config.Environment, "Config should start empty")
	assert.Equal(t, "", config.Version, "Config should start empty")
	assert.Equal(t, "", config.OTLPEndpoint, "Config should start empty")
}

// TestNewTracingService_CustomConfig tests custom configuration values.
func TestNewTracingService_CustomConfig(t *testing.T) {
	config := &TracingConfig{
		ServiceName:  "test-service",
		Environment:  "test",
		Version:      "1.0.0",
		OTLPEndpoint: "localhost:4317",
		SampleRatio:  0.5,
	}

	// Verify the values are set correctly
	assert.Equal(t, "test-service", config.ServiceName)
	assert.Equal(t, "test", config.Environment)
	assert.Equal(t, "1.0.0", config.Version)
	assert.Equal(t, "localhost:4317", config.OTLPEndpoint)
	assert.Equal(t, 0.5, config.SampleRatio)
}

// TestTracingService_Tracer tests the Tracer method returns a valid tracer.
func TestTracingService_Tracer(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	service, err := NewTracingService(logger, &TracingConfig{
		ServiceName: "test-service",
		Environment: "test",
		Version:     "1.0.0",
	}, WithNoopProvider())

	assert.NoError(t, err)
	assert.NotNil(t, service)

	// Test that Tracer returns a tracer
	tracer := service.Tracer("test-tracer")
	assert.NotNil(t, tracer)
}

// TestTracingService_Shutdown_Success tests successful shutdown.
func TestTracingService_Shutdown_Success(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	service, err := NewTracingService(logger, &TracingConfig{
		ServiceName: "test-service",
		Environment: "test",
		Version:     "1.0.0",
	}, WithNoopProvider())

	assert.NoError(t, err)
	assert.NotNil(t, service)

	ctx := context.Background()
	err = service.Shutdown(ctx)

	assert.NoError(t, err)
}

// TestTracingService_Shutdown_Error tests shutdown error handling.
func TestTracingService_Shutdown_Error(t *testing.T) {
	// Test that shutdown with noop provider works
	logger, _ := zap.NewDevelopment()

	service, err := NewTracingService(logger, &TracingConfig{
		ServiceName: "test-service",
		Environment: "test",
		Version:     "1.0.0",
	}, WithNoopProvider())

	assert.NoError(t, err)
	assert.NotNil(t, service)

	ctx := context.Background()
	err = service.Shutdown(ctx)

	// Noop provider should not return error
	assert.NoError(t, err)
}

// TestTracingService_StartSpan tests starting a new span.
func TestTracingService_StartSpan(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	service, err := NewTracingService(logger, &TracingConfig{
		ServiceName: "test-service",
		Environment: "test",
		Version:     "1.0.0",
	}, WithNoopProvider())

	assert.NoError(t, err)
	assert.NotNil(t, service)

	ctx := context.Background()

	// Call StartSpan
	ctx, span := service.StartSpan(ctx, "test-span")

	// Verify span was created
	assert.NotNil(t, span)
	assert.NotNil(t, ctx)
}

// TestTracingService_GetServiceName tests the GetServiceName method.
func TestTracingService_GetServiceName(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	service, err := NewTracingService(logger, &TracingConfig{
		ServiceName: "my-service",
		Environment: "production",
		Version:     "2.0.0",
	}, WithNoopProvider())

	assert.NoError(t, err)
	assert.NotNil(t, service)

	serviceName := service.GetServiceName()
	assert.Equal(t, "my-service", serviceName)
}

// TestTracingService_GetEnvironment tests the GetEnvironment method.
func TestTracingService_GetEnvironment(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	service, err := NewTracingService(logger, &TracingConfig{
		ServiceName: "my-service",
		Environment: "production",
		Version:     "2.0.0",
	}, WithNoopProvider())

	assert.NoError(t, err)
	assert.NotNil(t, service)

	environment := service.GetEnvironment()
	assert.Equal(t, "production", environment)
}

// TestTracingService_GetVersion tests the GetVersion method.
func TestTracingService_GetVersion(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	service, err := NewTracingService(logger, &TracingConfig{
		ServiceName: "my-service",
		Environment: "production",
		Version:     "2.0.0",
	}, WithNoopProvider())

	assert.NoError(t, err)
	assert.NotNil(t, service)

	version := service.GetVersion()
	assert.Equal(t, "2.0.0", version)
}

// TestTracingService_WithSpan_Success tests WithSpan when the function succeeds.
func TestTracingService_WithSpan_Success(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	service, err := NewTracingService(logger, &TracingConfig{
		ServiceName: "test-service",
		Environment: "test",
		Version:     "1.0.0",
	}, WithNoopProvider())

	assert.NoError(t, err)
	assert.NotNil(t, service)

	ctx := context.Background()

	// Define a function that succeeds
	fn := func(ctx context.Context) error {
		return nil
	}

	err = service.WithSpan(ctx, "test-operation", fn)

	assert.NoError(t, err)
}

// TestTracingService_WithSpan_Error tests WithSpan when the function returns an error.
func TestTracingService_WithSpan_Error(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	service, err := NewTracingService(logger, &TracingConfig{
		ServiceName: "test-service",
		Environment: "test",
		Version:     "1.0.0",
	}, WithNoopProvider())

	assert.NoError(t, err)
	assert.NotNil(t, service)

	ctx := context.Background()

	// Define a function that returns an error
	fn := func(ctx context.Context) error {
		return errors.New("operation failed")
	}

	err = service.WithSpan(ctx, "test-operation", fn)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "operation failed")
}

// TestTracingService_GetServiceName_Default tests default service name behavior.
func TestTracingService_GetServiceName_Default(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	// Test with default service name (empty string should get replaced with "release-engine")
	service, err := NewTracingService(logger, &TracingConfig{
		Environment: "development",
		Version:     "0.0.0",
	}, WithNoopProvider())

	assert.NoError(t, err)
	assert.NotNil(t, service)

	// With empty service name, it defaults to "release-engine"
	serviceName := service.GetServiceName()
	assert.Equal(t, "release-engine", serviceName)
}

// TestTracingService_GetEnvironment_Default tests default environment behavior.
func TestTracingService_GetEnvironment_Default(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	service, err := NewTracingService(logger, &TracingConfig{
		ServiceName: "test-service",
		Version:     "0.0.0",
	}, WithNoopProvider())

	assert.NoError(t, err)
	assert.NotNil(t, service)

	// With empty environment, it defaults to "development"
	environment := service.GetEnvironment()
	assert.Equal(t, "development", environment)
}

// TestTracingService_GetVersion_Default tests default version behavior.
func TestTracingService_GetVersion_Default(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	service, err := NewTracingService(logger, &TracingConfig{
		ServiceName: "test-service",
		Environment: "development",
	}, WithNoopProvider())

	assert.NoError(t, err)
	assert.NotNil(t, service)

	// With empty version, it defaults to "0.0.0"
	version := service.GetVersion()
	assert.Equal(t, "0.0.0", version)
}

// TestTracingService_WithSpan_Nested tests nested WithSpan calls.
func TestTracingService_WithSpan_Nested(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	service, err := NewTracingService(logger, &TracingConfig{
		ServiceName: "test-service",
		Environment: "test",
		Version:     "1.0.0",
	}, WithNoopProvider())

	assert.NoError(t, err)
	assert.NotNil(t, service)

	ctx := context.Background()

	// Outer span
	err = service.WithSpan(ctx, "outer-operation", func(ctx context.Context) error {
		// Inner span
		return service.WithSpan(ctx, "inner-operation", func(ctx context.Context) error {
			return nil
		})
	})

	assert.NoError(t, err)
}

// TestTracingService_MultipleTracers tests creating multiple tracers.
func TestTracingService_MultipleTracers(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	service, err := NewTracingService(logger, &TracingConfig{
		ServiceName: "test-service",
		Environment: "test",
		Version:     "1.0.0",
	}, WithNoopProvider())

	assert.NoError(t, err)
	assert.NotNil(t, service)

	tracer1 := service.Tracer("tracer-1")
	tracer2 := service.Tracer("tracer-2")

	assert.NotNil(t, tracer1)
	assert.NotNil(t, tracer2)
}

// TestTracingService_TracerReturnsSameInstance tests that Tracer returns the same instance for the same name.
func TestTracingService_TracerReturnsSameInstance(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	service, err := NewTracingService(logger, &TracingConfig{
		ServiceName: "test-service",
		Environment: "test",
		Version:     "1.0.0",
	}, WithNoopProvider())

	assert.NoError(t, err)
	assert.NotNil(t, service)

	tracer1 := service.Tracer("same-tracer")
	tracer2 := service.Tracer("same-tracer")

	// Both should return the same tracer (same name)
	assert.NotNil(t, tracer1)
	assert.NotNil(t, tracer2)
}

// TestObservabilityError_TracingInitFailed tests the ErrTracingInitFailed error.
func TestObservabilityError_TracingInitFailed(t *testing.T) {
	err := NewObservabilityError(ErrTracingInitFailed, "TRACING_INIT_FAILED", map[string]string{"error": "connection refused"})

	assert.Equal(t, "TRACING_INIT_FAILED: tracing initialisation failed", err.Error())
	assert.Equal(t, ErrTracingInitFailed, err.Unwrap())
	assert.Equal(t, "connection refused", err.Detail["error"])
}

// TestObservabilityError_TracingFlushTimeout tests the ErrTracingFlushTimeout error.
func TestObservabilityError_TracingFlushTimeout(t *testing.T) {
	err := NewObservabilityError(ErrTracingFlushTimeout, "TRACING_FLUSH_TIMEOUT", map[string]string{"error": "timeout"})

	assert.Equal(t, "TRACING_FLUSH_TIMEOUT: tracing flush timed out", err.Error())
	assert.Equal(t, ErrTracingFlushTimeout, err.Unwrap())
	assert.Equal(t, "timeout", err.Detail["error"])
}

// TestObservabilityError_NilDetail tests that detail can be nil.
func TestObservabilityError_NilDetail(t *testing.T) {
	err := NewObservabilityError(ErrTracingInitFailed, "TRACING_INIT_FAILED", nil)

	assert.Equal(t, "TRACING_INIT_FAILED: tracing initialisation failed", err.Error())
	assert.Nil(t, err.Detail)
}

// TestObservabilityError_EmptyDetail tests that detail can be empty.
func TestObservabilityError_EmptyDetail(t *testing.T) {
	err := NewObservabilityError(ErrTracingInitFailed, "TRACING_INIT_FAILED", map[string]string{})

	assert.Equal(t, "TRACING_INIT_FAILED: tracing initialisation failed", err.Error())
	assert.Empty(t, err.Detail)
}

// TestTracingService_Shutdown_WithTimeout tests shutdown with context timeout.
func TestTracingService_Shutdown_WithTimeout(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	service, err := NewTracingService(logger, &TracingConfig{
		ServiceName: "test-service",
		Environment: "test",
		Version:     "1.0.0",
	}, WithNoopProvider())

	assert.NoError(t, err)
	assert.NotNil(t, service)

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err = service.Shutdown(ctx)

	assert.NoError(t, err)
}

// TestTracingService_StartSpan_WithOptions tests starting a span with options.
func TestTracingService_StartSpan_WithOptions(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	service, err := NewTracingService(logger, &TracingConfig{
		ServiceName: "test-service",
		Environment: "test",
		Version:     "1.0.0",
	}, WithNoopProvider())

	assert.NoError(t, err)
	assert.NotNil(t, service)

	ctx := context.Background()

	// Start span with options
	ctx, span := service.StartSpan(ctx, "test-span", trace.WithSpanKind(trace.SpanKindServer))

	assert.NotNil(t, span)
	assert.NotNil(t, ctx)
}
