// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package observability

import (
	"context"
	"strconv"
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
	registry                   *prometheus.Registry
	doraRegistry               *prometheus.Registry
	logger                     *zap.Logger
	collectors                 map[string]prometheus.Collector
	doraCollectors             map[string]prometheus.Collector
	mu                         sync.RWMutex
	httpPort                   int
	doraPrometheusEnabled      bool
	scrapeDuration             *prometheus.HistogramVec
	totalRequests              *prometheus.CounterVec
	approvalRequestsTotal      *prometheus.CounterVec
	approvalDecisionsTotal     *prometheus.CounterVec
	approvalLatencySeconds     *prometheus.HistogramVec
	approvalEscalationsTotal   *prometheus.CounterVec
	approvalTimeoutsTotal      *prometheus.CounterVec
	approvalWorkerTickDuration *prometheus.HistogramVec
	doraDeploymentsTotal       *prometheus.CounterVec
	doraLeadTimeSeconds        *prometheus.HistogramVec
	doraCFRPercent             *prometheus.GaugeVec
	doraMTTRSeconds            *prometheus.GaugeVec
	doraDeadLetterTotal        *prometheus.CounterVec
	doraTenantsAboveCFRTotal   *prometheus.GaugeVec
	doraTenantsWithNoDataTotal *prometheus.GaugeVec

	connectorCallTotal      *prometheus.CounterVec
	connectorCallDuration   *prometheus.HistogramVec
	connectorTransportRetry *prometheus.CounterVec
}

type MetricsExporterOption func(*MetricsExporter)

func WithDoraPrometheusEnabled(enabled bool) MetricsExporterOption {
	return func(m *MetricsExporter) {
		m.doraPrometheusEnabled = enabled
	}
}

// NewMetricsExporter creates a new MetricsExporter.
func NewMetricsExporter(logger *zap.Logger, httpPort int, opts ...MetricsExporterOption) *MetricsExporter {
	registry := prometheus.NewRegistry()
	doraRegistry := prometheus.NewRegistry()

	// Create standard collectors
	registry.MustRegister(collectors.NewGoCollector())
	registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	exporter := &MetricsExporter{
		registry:              registry,
		doraRegistry:          doraRegistry,
		logger:                logger,
		collectors:            make(map[string]prometheus.Collector),
		doraCollectors:        make(map[string]prometheus.Collector),
		httpPort:              httpPort,
		doraPrometheusEnabled: true,
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
	for _, opt := range opts {
		opt(exporter)
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

	// Register approval lifecycle metrics (Phase 8)
	m.approvalRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "re_approval_requests_total",
			Help: "Total number of approval requests created",
		},
		[]string{"tenant_id", "path_key", "step_key"},
	)
	if err := m.registerCollector("approval_requests_total", m.approvalRequestsTotal); err != nil {
		return err
	}

	m.approvalDecisionsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "re_approval_decisions_total",
			Help: "Total number of approval decisions recorded",
		},
		[]string{"tenant_id", "path_key", "step_key", "decision"},
	)
	if err := m.registerCollector("approval_decisions_total", m.approvalDecisionsTotal); err != nil {
		return err
	}

	m.approvalLatencySeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "re_approval_latency_seconds",
			Help:    "Approval decision latency in seconds",
			Buckets: []float64{1, 5, 10, 30, 60, 120, 300, 600, 1800, 3600},
		},
		[]string{"tenant_id", "path_key"},
	)
	if err := m.registerCollector("approval_latency_seconds", m.approvalLatencySeconds); err != nil {
		return err
	}

	m.approvalEscalationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "re_approval_escalations_total",
			Help: "Total number of approval escalations",
		},
		[]string{"tenant_id", "path_key"},
	)
	if err := m.registerCollector("approval_escalations_total", m.approvalEscalationsTotal); err != nil {
		return err
	}

	m.approvalTimeoutsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "re_approval_timeouts_total",
			Help: "Total number of approval timeouts",
		},
		[]string{"tenant_id", "path_key"},
	)
	if err := m.registerCollector("approval_timeouts_total", m.approvalTimeoutsTotal); err != nil {
		return err
	}

	m.approvalWorkerTickDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "re_approval_worker_tick_duration_seconds",
			Help:    "Duration of approval worker tick execution",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0},
		},
		[]string{"status"},
	)
	if err := m.registerCollector("approval_worker_tick_duration_seconds", m.approvalWorkerTickDuration); err != nil {
		return err
	}

	m.doraDeploymentsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "release_engine_dora_deployments_total",
			Help: "Total number of DORA deployment events written",
		},
		[]string{"tenant_id", "service_ref", "environment", "status"},
	)
	if m.doraPrometheusEnabled {
		if err := m.registerDoraCollector("dora_deployments_total", m.doraDeploymentsTotal); err != nil {
			return err
		}
	}

	m.doraLeadTimeSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "release_engine_dora_lead_time_seconds",
			Help:    "Lead time for changes in seconds for correlated commit to deployment pairs",
			Buckets: []float64{60, 300, 900, 1800, 3600, 14400, 86400, 604800},
		},
		[]string{"tenant_id", "service_ref"},
	)
	if m.doraPrometheusEnabled {
		if err := m.registerDoraCollector("dora_lead_time_seconds", m.doraLeadTimeSeconds); err != nil {
			return err
		}
	}

	m.doraCFRPercent = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "release_engine_dora_cfr_percent",
			Help: "Most recently computed change failure rate percentage",
		},
		[]string{"tenant_id", "service_ref", "window_days"},
	)
	if m.doraPrometheusEnabled {
		if err := m.registerDoraCollector("dora_cfr_percent", m.doraCFRPercent); err != nil {
			return err
		}
	}

	m.doraMTTRSeconds = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "release_engine_dora_mttr_seconds",
			Help: "Most recently computed mean time to restore in seconds",
		},
		[]string{"tenant_id", "service_ref", "window_days"},
	)
	if m.doraPrometheusEnabled {
		if err := m.registerDoraCollector("dora_mttr_seconds", m.doraMTTRSeconds); err != nil {
			return err
		}
	}

	m.doraDeadLetterTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "release_engine_dora_dead_letter_total",
			Help: "Total number of DORA webhook dead-letter writes",
		},
		[]string{"tenant_id", "provider", "error_code"},
	)
	if m.doraPrometheusEnabled {
		if err := m.registerDoraCollector("dora_dead_letter_total", m.doraDeadLetterTotal); err != nil {
			return err
		}
	}

	m.doraTenantsAboveCFRTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "release_engine_dora_tenants_above_cfr_threshold_total",
			Help: "Aggregate number of tenants above configured CFR threshold",
		},
		[]string{"window", "threshold"},
	)
	if err := m.registerCollector("dora_tenants_above_cfr_threshold_total", m.doraTenantsAboveCFRTotal); err != nil {
		return err
	}

	m.doraTenantsWithNoDataTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "release_engine_dora_tenants_with_no_dora_data_total",
			Help: "Aggregate number of tenants with no DORA data by metric",
		},
		[]string{"metric"},
	)
	if err := m.registerCollector("dora_tenants_with_no_dora_data_total", m.doraTenantsWithNoDataTotal); err != nil {
		return err
	}

	m.connectorCallTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "release_engine_connector_call_total",
			Help: "Total number of connector calls",
		},
		[]string{"connector_key", "operation", "status"},
	)
	if err := m.registerCollector("connector_call_total", m.connectorCallTotal); err != nil {
		return err
	}

	m.connectorCallDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "release_engine_connector_call_duration_seconds",
			Help:    "Duration of connector calls in seconds",
			Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1.0, 5.0, 10.0, 30.0},
		},
		[]string{"connector_key", "operation", "status"},
	)
	if err := m.registerCollector("connector_call_duration_seconds", m.connectorCallDuration); err != nil {
		return err
	}

	m.connectorTransportRetry = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "release_engine_connector_transport_retries_total",
			Help: "Total number of connector transport retries",
		},
		[]string{"connector_key"},
	)
	if err := m.registerCollector("connector_transport_retries_total", m.connectorTransportRetry); err != nil {
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

func (m *MetricsExporter) registerDoraCollector(name string, collector prometheus.Collector) error {
	if _, exists := m.doraCollectors[name]; exists {
		m.logger.Error("metricsexporter.failure",
			zap.String("component", "MetricsExporter"),
			zap.String("error", "dora collector already registered"))
		return NewObservabilityError(
			ErrMetricsCollectorConflict,
			"METRICS_COLLECTOR_CONFLICT",
			map[string]string{"collector": name},
		)
	}

	if err := m.doraRegistry.Register(collector); err != nil {
		m.logger.Error("metricsexporter.failure",
			zap.String("component", "MetricsExporter"),
			zap.String("error", err.Error()))
		return NewObservabilityError(
			ErrMetricsCollectorConflict,
			"METRICS_COLLECTOR_CONFLICT",
			map[string]string{"collector": name, "error": err.Error()},
		)
	}

	m.doraCollectors[name] = collector
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

	handler := promhttp.HandlerFor(prometheus.Gatherers{m.registry, m.doraRegistry}, promhttp.HandlerOpts{})
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

// RecordApprovalRequest increments approval request count.
func (m *MetricsExporter) RecordApprovalRequest(tenantID, pathKey, stepKey string) {
	if m.approvalRequestsTotal == nil {
		return
	}
	m.approvalRequestsTotal.WithLabelValues(tenantID, pathKey, stepKey).Inc()
}

// RecordApprovalDecision increments approval decision count.
func (m *MetricsExporter) RecordApprovalDecision(tenantID, pathKey, stepKey, decision string) {
	if m.approvalDecisionsTotal == nil {
		return
	}
	m.approvalDecisionsTotal.WithLabelValues(tenantID, pathKey, stepKey, decision).Inc()
}

// RecordApprovalLatency observes approval decision latency.
func (m *MetricsExporter) RecordApprovalLatency(tenantID, pathKey string, latency time.Duration) {
	if m.approvalLatencySeconds == nil {
		return
	}
	m.approvalLatencySeconds.WithLabelValues(tenantID, pathKey).Observe(latency.Seconds())
}

// RecordApprovalEscalation increments approval escalation count.
func (m *MetricsExporter) RecordApprovalEscalation(tenantID, pathKey string) {
	if m.approvalEscalationsTotal == nil {
		return
	}
	m.approvalEscalationsTotal.WithLabelValues(tenantID, pathKey).Inc()
}

// RecordApprovalTimeout increments approval timeout count.
func (m *MetricsExporter) RecordApprovalTimeout(tenantID, pathKey string) {
	if m.approvalTimeoutsTotal == nil {
		return
	}
	m.approvalTimeoutsTotal.WithLabelValues(tenantID, pathKey).Inc()
}

// RecordApprovalWorkerTick records approval worker tick duration.
func (m *MetricsExporter) RecordApprovalWorkerTick(status string, duration time.Duration) {
	if m.approvalWorkerTickDuration == nil {
		return
	}
	m.approvalWorkerTickDuration.WithLabelValues(status).Observe(duration.Seconds())
}

// RecordDoraLeadTime observes lead time in seconds.
func (m *MetricsExporter) RecordDoraLeadTime(tenantID, serviceRef string, leadTimeSeconds float64) {
	if !m.doraPrometheusEnabled || m.doraLeadTimeSeconds == nil {
		return
	}
	m.doraLeadTimeSeconds.WithLabelValues(tenantID, serviceRef).Observe(leadTimeSeconds)
}

// RecordDoraCFR sets the latest CFR percentage.
func (m *MetricsExporter) RecordDoraCFR(tenantID, serviceRef string, windowDays int, percent float64) {
	if !m.doraPrometheusEnabled || m.doraCFRPercent == nil {
		return
	}
	m.doraCFRPercent.WithLabelValues(tenantID, serviceRef, strconv.Itoa(windowDays)).Set(percent)
}

// RecordDoraMTTR sets the latest MTTR in seconds.
func (m *MetricsExporter) RecordDoraMTTR(tenantID, serviceRef string, windowDays int, seconds float64) {
	if !m.doraPrometheusEnabled || m.doraMTTRSeconds == nil {
		return
	}
	m.doraMTTRSeconds.WithLabelValues(tenantID, serviceRef, strconv.Itoa(windowDays)).Set(seconds)
}

// RecordDoraDeadLetter increments DORA dead-letter counter.
func (m *MetricsExporter) RecordDoraDeadLetter(tenantID, provider, errorCode string) {
	if !m.doraPrometheusEnabled || m.doraDeadLetterTotal == nil {
		return
	}
	m.doraDeadLetterTotal.WithLabelValues(tenantID, provider, errorCode).Inc()
}

func (m *MetricsExporter) RecordDoraTenantsAboveCFRThreshold(windowDays int, threshold float64, total float64) {
	if m.doraTenantsAboveCFRTotal == nil {
		return
	}
	m.doraTenantsAboveCFRTotal.WithLabelValues(strconv.Itoa(windowDays), strconv.FormatFloat(threshold, 'f', -1, 64)).Set(total)
}

func (m *MetricsExporter) RecordDoraTenantsWithNoDoraData(metric string, total float64) {
	if m.doraTenantsWithNoDataTotal == nil {
		return
	}
	m.doraTenantsWithNoDataTotal.WithLabelValues(metric).Set(total)
}

func (m *MetricsExporter) RecordConnectorCall(connectorKey, operation, status string, duration time.Duration) {
	if m.connectorCallTotal != nil {
		m.connectorCallTotal.WithLabelValues(connectorKey, operation, status).Inc()
	}
	if m.connectorCallDuration != nil {
		m.connectorCallDuration.WithLabelValues(connectorKey, operation, status).Observe(duration.Seconds())
	}
}

func (m *MetricsExporter) RecordConnectorRetry(connectorKey string) {
	if m.connectorTransportRetry != nil {
		m.connectorTransportRetry.WithLabelValues(connectorKey).Inc()
	}
}
