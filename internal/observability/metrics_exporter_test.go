package observability

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// Test handleMetrics function - tests the /metrics HTTP endpoint
func TestMetricsExporter_HandleMetrics_Success(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	exporter := NewMetricsExporter(logger, 8080)
	e := echo.New()

	// Register routes
	exporter.RegisterRoutes(e)

	// Create a test request
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Call handleMetrics
	err := exporter.handleMetrics(c)

	// Assertions
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NotEmpty(t, rec.Body.String())
}

func TestMetricsExporter_HandleMetrics_ResponseContainsMetrics(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	exporter := NewMetricsExporter(logger, 8080)

	// Register some collectors to have metrics in the output
	err := exporter.RegisterCollectors()
	require.NoError(t, err)

	e := echo.New()
	exporter.RegisterRoutes(e)

	// Create a test request
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Call handleMetrics
	err = exporter.handleMetrics(c)
	require.NoError(t, err)

	// Verify response contains Prometheus metrics format
	body := rec.Body.String()
	assert.True(t, strings.Contains(body, "# HELP"), "Response should contain metric help text")
	assert.True(t, strings.Contains(body, "# TYPE"), "Response should contain metric type text")
}

func TestMetricsExporter_HandleMetrics_RecordsScrapeDuration(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	exporter := NewMetricsExporter(logger, 8080)
	e := echo.New()
	exporter.RegisterRoutes(e)

	// Get initial count
	initialCount := testutil.ToFloat64(exporter.totalRequests.WithLabelValues("success"))

	// Create a test request
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Call handleMetrics
	err := exporter.handleMetrics(c)
	require.NoError(t, err)

	// Verify totalRequests counter was incremented
	finalCount := testutil.ToFloat64(exporter.totalRequests.WithLabelValues("success"))
	assert.Equal(t, initialCount+1, finalCount, "totalRequests should be incremented")
}

func TestMetricsExporter_HandleMetrics_RecordsScrapeDurationHistogram(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	exporter := NewMetricsExporter(logger, 8080)
	e := echo.New()
	exporter.RegisterRoutes(e)

	// Get initial count by gathering metrics
	initialMetrics, err := exporter.registry.Gather()
	require.NoError(t, err)
	var initialCount uint64
	for _, m := range initialMetrics {
		if strings.Contains(m.GetName(), "release_engine_metrics_scrape_duration") {
			initialCount = m.GetMetric()[0].GetHistogram().GetSampleCount()
			break
		}
	}

	// Create a test request
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Call handleMetrics
	err = exporter.handleMetrics(c)
	require.NoError(t, err)

	// Verify scrapeDuration histogram was observed
	finalMetrics, err := exporter.registry.Gather()
	require.NoError(t, err)
	var finalCount uint64
	for _, m := range finalMetrics {
		if strings.Contains(m.GetName(), "release_engine_metrics_scrape_duration") {
			finalCount = m.GetMetric()[0].GetHistogram().GetSampleCount()
			break
		}
	}
	assert.Greater(t, finalCount, initialCount, "scrapeDuration should have observations")
}

// Test scheduler metrics registration
func TestMetricsExporter_SchedulerClaimsRegistered(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	exporter := NewMetricsExporter(logger, 8080)

	err := exporter.RegisterCollectors()
	require.NoError(t, err)

	// Verify scheduler_claims collector is registered in the collectors map
	collector, exists := exporter.collectors["scheduler_claims"]
	assert.True(t, exists, "scheduler_claims collector should be registered")
	assert.NotNil(t, collector, "scheduler_claims collector should not be nil")
}

func TestMetricsExporter_SchedulerClaimDurationRegistered(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	exporter := NewMetricsExporter(logger, 8080)

	err := exporter.RegisterCollectors()
	require.NoError(t, err)

	// Verify scheduler_claim_duration collector is registered in the collectors map
	collector, exists := exporter.collectors["scheduler_claim_duration"]
	assert.True(t, exists, "scheduler_claim_duration collector should be registered")
	assert.NotNil(t, collector, "scheduler_claim_duration collector should not be nil")
}

// Test that RecordScrape properly records metrics with assertions
func TestMetricsExporter_RecordScrape_WithAssertions(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	exporter := NewMetricsExporter(logger, 8080)
	ctx := context.Background()

	// Record a scrape
	exporter.RecordScrape(ctx, "success", 100*time.Millisecond)

	// Verify the metric was recorded - check via registry
	metrics, err := exporter.registry.Gather()
	require.NoError(t, err)

	// Find the scrape duration and total requests metrics
	var foundScrapeDuration, foundTotalRequests bool
	for _, m := range metrics {
		name := m.GetName()
		if strings.Contains(name, "release_engine_metrics_scrape_duration") {
			foundScrapeDuration = true
		}
		if strings.Contains(name, "release_engine_metricsexporter_total") {
			foundTotalRequests = true
		}
	}
	assert.True(t, foundScrapeDuration, "scrape duration metric should exist")
	assert.True(t, foundTotalRequests, "total requests metric should exist")
}

// Test RegisterRoutes actually registers a working route
func TestMetricsExporter_RegisterRoutes_ActuallyWorks(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	exporter := NewMetricsExporter(logger, 8080)
	e := echo.New()

	exporter.RegisterRoutes(e)

	// Create a test request using the Echo instance
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NotEmpty(t, rec.Body.String())
}
