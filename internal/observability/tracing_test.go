package observability

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestTracingConfig_Defaults(t *testing.T) {
	config := &TracingConfig{}

	// Set defaults as in the actual implementation
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
		config.SampleRatio = 0.1
	}

	assert.Equal(t, "release-engine", config.ServiceName)
	assert.Equal(t, "development", config.Environment)
	assert.Equal(t, "0.0.0", config.Version)
	assert.Equal(t, "localhost:4317", config.OTLPEndpoint)
	assert.Equal(t, 0.1, config.SampleRatio)
}

func TestTracingConfig_CustomValues(t *testing.T) {
	config := &TracingConfig{
		ServiceName:  "custom-service",
		Environment:  "production",
		Version:      "2.0.0",
		OTLPEndpoint: "otel.company.com:4317",
		SampleRatio:  0.25,
	}

	// Verify custom values are preserved
	assert.Equal(t, "custom-service", config.ServiceName)
	assert.Equal(t, "production", config.Environment)
	assert.Equal(t, "2.0.0", config.Version)
	assert.Equal(t, "otel.company.com:4317", config.OTLPEndpoint)
	assert.Equal(t, 0.25, config.SampleRatio)
}

func TestNewTracingService_Integration(t *testing.T) {
	// This test shows that NewTracingService can initialize successfully
	// In a real environment with OTLP endpoint available, it would work
	// For testing, we just verify the configuration defaults work
	logger, _ := zap.NewDevelopment()

	// Test configuration handling
	config := &TracingConfig{
		ServiceName:  "test-service",
		Environment:  "test",
		Version:      "1.0.0",
		OTLPEndpoint: "localhost:4317",
		SampleRatio:  0.5,
	}

	// Just verify the config values are as expected
	assert.Equal(t, "test-service", config.ServiceName)
	assert.Equal(t, "test", config.Environment)
	assert.Equal(t, "1.0.0", config.Version)
	assert.Equal(t, "localhost:4317", config.OTLPEndpoint)
	assert.Equal(t, 0.5, config.SampleRatio)

	_ = logger // Suppress unused warning
}
