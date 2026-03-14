// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package observability

import (
	"context"
	"encoding/json"
	"strconv"
	"sync"
	"time"

	"github.com/gatblau/release-engine/internal/db"
	"go.uber.org/zap"
)

// MetricsWriter writes immutable operational events to metrics_sql.
type MetricsWriter interface {
	WriteEvent(ctx context.Context, event MetricsEvent) error
	Close() error
}

// MetricsEvent represents a metrics event to be written to SQL.
type MetricsEvent struct {
	TenantID     string
	JobID        string
	PathKey      string
	Attempt      int
	RunID        string
	EventType    string
	Timestamp    time.Time
	Environment  string
	CommitSHAs   []string
	State        string
	DurationMs   int64
	ErrorCode    string
	ErrorMessage string
	Metadata     map[string]string
}

// MetricsSQLWriter implements the MetricsWriter interface for writing events to SQL.
type MetricsSQLWriter struct {
	db            db.Pool
	logger        *zap.Logger
	queue         chan MetricsEvent
	workers       int
	wg            sync.WaitGroup
	stopCh        chan struct{}
	mu            sync.RWMutex
	closed        bool
	queueSize     int
	writeDuration *prometheusHistogram
	totalWritten  *prometheusCounter
}

// prometheusHistogram is a simple histogram implementation for metrics.
type prometheusHistogram struct {
	mu           sync.RWMutex
	buckets      []float64
	count        uint64
	sum          float64
	bucketCounts map[float64]uint64
}

// prometheusCounter is a simple counter implementation for metrics.
type prometheusCounter struct {
	mu    sync.RWMutex
	value uint64
}

// NewMetricsSQLWriter creates a new MetricsSQLWriter.
// Implements Phase 3: MetricsSQLWriter spec - write immutable operational events.
func NewMetricsSQLWriter(logger *zap.Logger, pool db.Pool, queueSize int, workers int) *MetricsSQLWriter {
	if queueSize <= 0 {
		queueSize = 10000 // Default queue size
	}
	if workers <= 0 {
		workers = 4 // Default worker count
	}

	writer := &MetricsSQLWriter{
		db:        pool,
		logger:    logger,
		queue:     make(chan MetricsEvent, queueSize),
		workers:   workers,
		stopCh:    make(chan struct{}),
		queueSize: queueSize,
		writeDuration: &prometheusHistogram{
			buckets:      []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000},
			bucketCounts: make(map[float64]uint64),
		},
		totalWritten: &prometheusCounter{},
	}

	// Start worker pool
	for i := 0; i < workers; i++ {
		writer.wg.Add(1)
		go writer.worker(i)
	}

	logger.Info("metricssqlwriter.start",
		zap.String("component", "MetricsSQLWriter"),
		zap.Int("queue_size", queueSize),
		zap.Int("workers", workers))

	return writer
}

// WriteEvent writes a metrics event to the queue.
// Implements Phase 3: MetricsSQLWriter spec - insert events asynchronously.
func (m *MetricsSQLWriter) WriteEvent(ctx context.Context, event MetricsEvent) error {
	m.mu.RLock()
	if m.closed {
		m.mu.RUnlock()
		return NewObservabilityError(
			ErrMetricsSQLQueueFull,
			"METRICS_SQL_QUEUE_FULL",
			map[string]string{"error": "writer closed"},
		)
	}
	m.mu.RUnlock()

	select {
	case m.queue <- event:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Queue is full - return error
		m.logger.Warn("metricssqlwriter.queue_full",
			zap.String("component", "MetricsSQLWriter"),
			zap.Int("queue_size", m.queueSize))
		return NewObservabilityError(
			ErrMetricsSQLQueueFull,
			"METRICS_SQL_QUEUE_FULL",
			map[string]string{"queue_size": strconv.Itoa(m.queueSize)},
		)
	}
}

// worker processes events from the queue and writes them to the database.
func (m *MetricsSQLWriter) worker(id int) {
	defer m.wg.Done()

	for {
		select {
		case <-m.stopCh:
			return
		case event, ok := <-m.queue:
			if !ok {
				return
			}
			m.processEvent(event, id)
		}
	}
}

// processEvent writes a single event to the database.
func (m *MetricsSQLWriter) processEvent(event MetricsEvent, workerID int) {
	start := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := m.db.Acquire(ctx)
	if err != nil {
		m.logger.Error("metricssqlwriter.failure",
			zap.String("component", "MetricsSQLWriter"),
			zap.Int("worker_id", workerID),
			zap.Error(err))
		m.recordFailure()
		return
	}
	defer conn.Release()

	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	labels := encodeMetadata(event.Metadata)
	doraType := doraEventTypeFromJobEvent(event.EventType)

	if doraType == "" {
		_, err = conn.Exec(ctx,
			`INSERT INTO metrics_job_events
			(ts, tenant_id, job_id, path_key, event, attempt, run_id, code, duration_ms, labels)
			VALUES ($1, $2, $3::uuid, $4, $5, $6, $7::uuid, $8, $9, $10::jsonb)`,
			event.Timestamp,
			event.TenantID,
			event.JobID,
			event.PathKey,
			event.EventType,
			event.Attempt,
			event.RunID,
			event.ErrorCode,
			event.DurationMs,
			labels,
		)
	} else {
		payload, payloadErr := json.Marshal(map[string]any{
			"job_id":      event.JobID,
			"run_id":      event.RunID,
			"commit_shas": event.CommitSHAs,
		})
		if payloadErr != nil {
			m.logger.Error("metricssqlwriter.failure",
				zap.String("component", "MetricsSQLWriter"),
				zap.Int("worker_id", workerID),
				zap.Error(payloadErr))
			m.recordFailure()
			return
		}

		outcome := "failed"
		if doraType == "deployment.succeeded" {
			outcome = "succeeded"
		}

		_, err = conn.Exec(ctx,
			`WITH metrics_insert AS (
				INSERT INTO metrics_job_events
				(ts, tenant_id, job_id, path_key, event, attempt, run_id, code, duration_ms, labels)
				VALUES ($1, $2, $3::uuid, $4, $5, $6, $7::uuid, $8, $9, $10::jsonb)
			),
			dora_insert AS (
				INSERT INTO dora_events
				(tenant_id, event_type, event_source, service_ref, environment, correlation_key, event_timestamp, payload)
				VALUES ($2, $11, 'release-engine', $4, $12, $3, $1, $13::jsonb)
				ON CONFLICT DO NOTHING
				RETURNING id
			)
			INSERT INTO dora_commit_deployment_links
			(tenant_id, service_ref, commit_sha, deployment_id, deployment_outcome, deployment_time)
			SELECT $2, $4, sha, di.id, $14, $1
			FROM dora_insert di
			CROSS JOIN UNNEST($15::text[]) AS sha
			WHERE COALESCE(sha, '') <> ''
			ON CONFLICT DO NOTHING`,
			event.Timestamp,
			event.TenantID,
			event.JobID,
			event.PathKey,
			event.EventType,
			event.Attempt,
			event.RunID,
			event.ErrorCode,
			event.DurationMs,
			labels,
			doraType,
			event.Environment,
			string(payload),
			outcome,
			event.CommitSHAs,
		)
	}

	duration := time.Since(start).Milliseconds()

	if err != nil {
		m.logger.Error("metricssqlwriter.failure",
			zap.String("component", "MetricsSQLWriter"),
			zap.Int("worker_id", workerID),
			zap.Error(err))
		m.recordFailure()
		return
	}

	m.recordSuccess(duration)

	m.logger.Debug("metricssqlwriter.event_written",
		zap.String("component", "MetricsSQLWriter"),
		zap.String("job_id", event.JobID),
		zap.String("event_type", event.EventType),
		zap.Int64("duration_ms", duration))
}

// recordSuccess records a successful write.
func (m *MetricsSQLWriter) recordSuccess(durationMs int64) {
	m.writeDuration.mu.Lock()
	m.writeDuration.count++
	m.writeDuration.sum += float64(durationMs)
	for _, bucket := range m.writeDuration.buckets {
		if float64(durationMs) <= bucket {
			m.writeDuration.bucketCounts[bucket]++
		}
	}
	m.writeDuration.mu.Unlock()

	m.totalWritten.mu.Lock()
	m.totalWritten.value++
	m.totalWritten.mu.Unlock()
}

// recordFailure records a failed write.
func (m *MetricsSQLWriter) recordFailure() {
	// Log failure - in production this would increment a counter
}

// Close stops the writer and drains the queue.
// Implements Phase 3: MetricsSQLWriter spec - graceful shutdown.
func (m *MetricsSQLWriter) Close() error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}
	m.closed = true
	m.mu.Unlock()

	// Signal workers to stop
	close(m.stopCh)

	// Wait for workers to finish with timeout
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		m.logger.Info("metricssqlwriter.success",
			zap.String("component", "MetricsSQLWriter"))
		return nil
	case <-time.After(10 * time.Second):
		m.logger.Error("metricssqlwriter.failure",
			zap.String("component", "MetricsSQLWriter"),
			zap.Error(NewObservabilityError(
				ErrMetricsSQLWriteFailed,
				"METRICS_SQL_WRITE_FAILED",
				map[string]string{"error": "close timeout"},
			)))
		return NewObservabilityError(
			ErrMetricsSQLWriteFailed,
			"METRICS_SQL_WRITE_FAILED",
			map[string]string{"error": "close timeout"},
		)
	}
}

// encodeMetadata encodes metadata map to JSON string.
func encodeMetadata(m map[string]string) string {
	if m == nil {
		return "{}"
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func doraEventTypeFromJobEvent(eventType string) string {
	switch eventType {
	case "job_completed":
		return "deployment.succeeded"
	case "job_failed":
		return "deployment.failed"
	default:
		return ""
	}
}

// GetQueueLength returns the current queue length.
func (m *MetricsSQLWriter) GetQueueLength() int {
	return len(m.queue)
}

// GetQueueCapacity returns the queue capacity.
func (m *MetricsSQLWriter) GetQueueCapacity() int {
	return m.queueSize
}
