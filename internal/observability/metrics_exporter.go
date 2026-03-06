package observability

import (
	"context"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// Exporter publishes Prometheus-compatible metrics.
type Exporter interface {
	RegisterCollectors() error
}

// MetricsExporter implements the Exporter interface for Prometheus metrics.
type MetricsExporter struct {
	registry       *prometheus.Registry
	logger         *zap.Logger
	collectors     map[string]prometheus.Collector
	mu             sync.RWMutex
	httpPort       int
	scrapeDuration *prometheus.HistogramVec
	totalRequests  *prometheus.CounterVec
}

// NewMetricsExporter creates a new MetricsExporter.
func NewMetricsExporter(logger *zap.Logger, httpPort int) *MetricsExporter {
	registry := prometheus.NewRegistry()

	// Create standard collectors
	registry.MustRegister(collectors.NewGoCollector())
	registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	exporter := &MetricsExporter{
		registry:   registry,
		logger:     logger,
		collectors: make(map[string]prometheus.Collector),
		httpPort:   httpPort,
		scrapeDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "release_engine_metrics_scrape_duration_seconds",
				Help:    "Duration of metrics scrape operations in seconds",
				Buckets: []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0},
			},
			[]string{"status"},
		),
		totalRequests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "release_engine_metricsexporter_total",
				Help: "Total number of metrics exporter requests",
			},
			[]string{"status"},
		),
	}

	registry.MustRegister(exporter.scrapeDuration)
	registry.MustRegister(exporter.totalRequests)

	return exporter
}

// RegisterCollectors registers metric collectors for the API, scheduler, runner, outbox, and reconciler.
// Implements Phase 3: MetricsExporter spec - register metric families.
func (m *MetricsExporter) RegisterCollectors() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Register API metrics
	apiDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "release_engine_api_request_duration_seconds",
			Help:    "Duration of API requests in seconds",
			Buckets: []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0},
		},
		[]string{"method", "path", "status"},
	)
	if err := m.registerCollector("api_requests", apiDuration); err != nil {
		return err
	}

	// Register scheduler metrics
	schedulerClaims := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "release_engine_scheduler_claims_total",
			Help: "Total number of job claims by the scheduler",
		},
		[]string{"status"},
	)
	if err := m.registerCollector("scheduler_claims", schedulerClaims); err != nil {
		return err
	}

	schedulerClaimDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "release_engine_scheduler_claim_duration_seconds",
			Help:    "Duration of job claim operations in seconds",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25},
		},
		[]string{"status"},
	)
	if err := m.registerCollector("scheduler_claim_duration", schedulerClaimDuration); err != nil {
		return err
	}

	// Register runner metrics
	runnerExecutions := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "release_engine_runner_executions_total",
			Help: "Total number of job executions by the runner",
		},
		[]string{"status"},
	)
	if err := m.registerCollector("runner_executions", runnerExecutions); err != nil {
		return err
	}

	runnerExecutionDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "release_engine_runner_execution_duration_seconds",
			Help:    "Duration of job execution in seconds",
			Buckets: []float64{0.1, 0.5, 1.0, 5.0, 10.0, 30.0, 60.0},
		},
		[]string{"status"},
	)
	if err := m.registerCollector("runner_execution_duration", runnerExecutionDuration); err != nil {
		return err
	}

	// Register outbox metrics
	outboxDeliveries := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "release_engine_outbox_deliveries_total",
			Help: "Total number of outbox deliveries",
		},
		[]string{"status"},
	)
	if err := m.registerCollector("outbox_deliveries", outboxDeliveries); err != nil {
		return err
	}

	outboxDeliveryDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "release_engine_outbox_delivery_duration_seconds",
			Help:    "Duration of outbox delivery in seconds",
			Buckets: []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0},
		},
		[]string{"status"},
	)
	if err := m.registerCollector("outbox_delivery_duration", outboxDeliveryDuration); err != nil {
		return err
	}

	// Register reconciler metrics
	reconcilerReconciliations := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "release_engine_reconciler_reconciliations_total",
			Help: "Total number of effect reconciliations",
		},
		[]string{"status"},
	)
	if err := m.registerCollector("reconciler_reconciliations", reconcilerReconciliations); err != nil {
		return err
	}

	reconcilerReconciliationDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "release_engine_reconciler_reconciliation_duration_seconds",
			Help:    "Duration of reconciliation in seconds",
			Buckets: []float64{0.1, 0.5, 1.0, 5.0, 10.0, 30.0},
		},
		[]string{"status"},
	)
	if err := m.registerCollector("reconciler_reconciliation_duration", reconcilerReconciliationDuration); err != nil {
		return err
	}

	// Register job metrics
	jobStateTransitions := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "release_engine_job_state_transitions_total",
			Help: "Total number of job state transitions",
		},
		[]string{"from_state", "to_state"},
	)
	if err := m.registerCollector("job_state_transitions", jobStateTransitions); err != nil {
		return err
	}

	// Register tenant-specific metrics with bounded cardinality
	tenantJobs := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "release_engine_tenant_jobs",
			Help: "Current number of jobs per tenant and state",
		},
		[]string{"tenant_id", "state"},
	)
	if err := m.registerCollector("tenant_jobs", tenantJobs); err != nil {
		return err
	}

	m.logger.Info("metricsexporter.start", zap.String("component", "MetricsExporter"))
	m.logger.Info("metricsexporter.success", zap.String("component", "MetricsExporter"))

	return nil
}

// registerCollector registers a collector with the registry.
func (m *MetricsExporter) registerCollector(name string, collector prometheus.Collector) error {
	if _, exists := m.collectors[name]; exists {
		m.logger.Error("metricsexporter.failure",
			zap.String("component", "MetricsExporter"),
			zap.String("error", "collector already registered"))
		return NewObservabilityError(
			ErrMetricsCollectorConflict,
			"METRICS_COLLECTOR_CONFLICT",
			map[string]string{"collector": name},
		)
	}

	if err := m.registry.Register(collector); err != nil {
		m.logger.Error("metricsexporter.failure",
			zap.String("component", "MetricsExporter"),
			zap.String("error", err.Error()))
		return NewObservabilityError(
			ErrMetricsCollectorConflict,
			"METRICS_COLLECTOR_CONFLICT",
			map[string]string{"collector": name, "error": err.Error()},
		)
	}

	m.collectors[name] = collector
	return nil
}

// RegisterRoutes registers the metrics endpoint with the Echo server.
// Implements Phase 3: MetricsExporter spec - expose metrics endpoint through HTTP.
func (m *MetricsExporter) RegisterRoutes(e *echo.Echo) {
	e.GET("/metrics", m.handleMetrics)
}

// handleMetrics handles the /metrics endpoint.
func (m *MetricsExporter) handleMetrics(c echo.Context) error {
	start := time.Now()

	handler := promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
	handler.ServeHTTP(c.Response().Writer, c.Request())

	duration := time.Since(start).Seconds()
	status := "success"

	m.scrapeDuration.WithLabelValues(status).Observe(duration)
	m.totalRequests.WithLabelValues(status).Inc()

	m.logger.Info("metrics scrape",
		zap.String("component", "MetricsExporter"),
		zap.Float64("duration_ms", duration*1000),
		zap.String("status", status))

	return nil
}

// GetRegistry returns the Prometheus registry for external use.
func (m *MetricsExporter) GetRegistry() *prometheus.Registry {
	return m.registry
}

// RecordScrape records a metrics scrape operation.
func (m *MetricsExporter) RecordScrape(ctx context.Context, status string, duration time.Duration) {
	m.scrapeDuration.WithLabelValues(status).Observe(duration.Seconds())
	m.totalRequests.WithLabelValues(status).Inc()
}
