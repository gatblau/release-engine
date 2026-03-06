//go:build integration

package observability

import (
	"context"
	"testing"
	"time"

	"github.com/gatblau/release-engine/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestMetricsSQLWriter_Integration(t *testing.T) {
	ctx := context.Background()
	logger, _ := zap.NewDevelopment()

	// Start PostgreSQL container
	connStr := db.SetupTestPostgres(ctx, t)
	require.NotEmpty(t, connStr, "connection string should not be empty")

	// Create a real pool
	pool, err := db.NewPool(connStr)
	require.NoError(t, err, "failed to create pool")
	require.NotNil(t, pool)
	defer pool.Close()

	// Create the metrics_job_events table
	conn, err := pool.Acquire(ctx)
	require.NoError(t, err, "failed to acquire connection")
	_, err = conn.Exec(ctx, db.SchemaMetricsJobEvents)
	conn.Release()
	require.NoError(t, err, "failed to create schema")

	// Create the MetricsSQLWriter with real pool
	writer := NewMetricsSQLWriter(logger, pool, 100, 1)
	require.NotNil(t, writer)

	// Write an event
	event := MetricsEvent{
		TenantID:   "tenant-1",
		JobID:      "job-123",
		RunID:      "run-456",
		EventType:  "job_started",
		Timestamp:  time.Now(),
		State:      "running",
		DurationMs: 100,
	}

	err = writer.WriteEvent(ctx, event)
	assert.NoError(t, err)

	// Wait for the event to be processed
	time.Sleep(500 * time.Millisecond)

	// Verify queue is empty (event was processed)
	assert.Equal(t, 0, writer.GetQueueLength())

	// Cleanup
	err = writer.Close()
	assert.NoError(t, err)
}

func TestMetricsSQLWriter_Integration_MultipleEvents(t *testing.T) {
	ctx := context.Background()
	logger, _ := zap.NewDevelopment()

	// Start PostgreSQL container
	connStr := db.SetupTestPostgres(ctx, t)
	require.NotEmpty(t, connStr, "connection string should not be empty")

	// Create a real pool
	pool, err := db.NewPool(connStr)
	require.NoError(t, err, "failed to create pool")
	require.NotNil(t, pool)
	defer pool.Close()

	// Create the metrics_job_events table
	conn, err := pool.Acquire(ctx)
	require.NoError(t, err, "failed to acquire connection")
	_, err = conn.Exec(ctx, db.SchemaMetricsJobEvents)
	conn.Release()
	require.NoError(t, err, "failed to create schema")

	// Create the MetricsSQLWriter with real pool
	writer := NewMetricsSQLWriter(logger, pool, 100, 2)
	require.NotNil(t, writer)

	// Write multiple events
	for i := 0; i < 5; i++ {
		event := MetricsEvent{
			TenantID:   "tenant-1",
			JobID:      "job-123",
			RunID:      "run-456",
			EventType:  "job_started",
			Timestamp:  time.Now(),
			State:      "running",
			DurationMs: int64(i * 100),
		}

		err := writer.WriteEvent(ctx, event)
		assert.NoError(t, err)
	}

	// Wait for events to be processed
	time.Sleep(1 * time.Second)

	// Verify queue is empty (all events processed)
	assert.Equal(t, 0, writer.GetQueueLength())

	// Cleanup
	err = writer.Close()
	assert.NoError(t, err)
}
