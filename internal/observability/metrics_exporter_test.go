package observability

import (
	"context"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestMetricsExporter_NewMetricsExporter(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	exporter := NewMetricsExporter(logger, 8080)

	assert.NotNil(t, exporter)
	assert.NotNil(t, exporter.registry)
	assert.NotNil(t, exporter.collectors)
}

func TestMetricsExporter_RegisterCollectors(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	exporter := NewMetricsExporter(logger, 8080)

	err := exporter.RegisterCollectors()

	assert.NoError(t, err)
	assert.Greater(t, len(exporter.collectors), 0)
}

func TestMetricsExporter_RegisterCollectors_Duplicate(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	exporter := NewMetricsExporter(logger, 8080)

	// First call should succeed
	err := exporter.RegisterCollectors()
	assert.NoError(t, err)

	// Second call should fail with conflict
	err = exporter.RegisterCollectors()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "METRICS_COLLECTOR_CONFLICT")
}

func TestMetricsExporter_GetRegistry(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	exporter := NewMetricsExporter(logger, 8080)

	registry := exporter.GetRegistry()

	assert.NotNil(t, registry)
}

func TestMetricsExporter_RegisterRoutes(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	exporter := NewMetricsExporter(logger, 8080)
	e := echo.New()

	exporter.RegisterRoutes(e)

	assert.NotNil(t, e)
}

func TestMetricsExporter_RecordScrape(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	exporter := NewMetricsExporter(logger, 8080)
	ctx := context.Background()

	exporter.RecordScrape(ctx, "success", 100)
}

func TestNewObservabilityError(t *testing.T) {
	err := NewObservabilityError(ErrMetricsCollectorConflict, "METRICS_COLLECTOR_CONFLICT", map[string]string{"test": "value"})

	assert.Equal(t, "METRICS_COLLECTOR_CONFLICT: metrics collector conflict", err.Error())
	assert.Equal(t, ErrMetricsCollectorConflict, err.Unwrap())
	assert.Equal(t, "value", err.Detail["test"])
}
