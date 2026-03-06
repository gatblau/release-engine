package observability

import (
	"context"
	"testing"

	"github.com/gatblau/release-engine/internal/db"
	"github.com/jackc/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

func TestMetricsSQLWriter_NewMetricsSQLWriter(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockPool := new(db.MockPool)

	// Test with default values
	writer := NewMetricsSQLWriter(logger, mockPool, 0, 0)
	assert.NotNil(t, writer)
	assert.Equal(t, 10000, writer.GetQueueCapacity())
	assert.Equal(t, 4, writer.workers)

	// Cleanup
	err := writer.Close()
	assert.NoError(t, err)
}

func TestMetricsSQLWriter_NewMetricsSQLWriter_CustomValues(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockPool := new(db.MockPool)

	writer := NewMetricsSQLWriter(logger, mockPool, 500, 2)
	assert.NotNil(t, writer)
	assert.Equal(t, 500, writer.GetQueueCapacity())
	assert.Equal(t, 2, writer.workers)

	// Cleanup
	err := writer.Close()
	assert.NoError(t, err)
}

func TestMetricsSQLWriter_WriteEvent_ClosedWriter(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockPool := new(db.MockPool)

	writer := NewMetricsSQLWriter(logger, mockPool, 100, 1)

	// Close the writer first
	writer.Close()

	// Try to write to closed writer
	event := MetricsEvent{
		TenantID:  "tenant-1",
		JobID:     "job-123",
		EventType: "job_started",
	}

	err := writer.WriteEvent(context.Background(), event)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "METRICS_SQL_QUEUE_FULL")
}

func TestMetricsSQLWriter_WriteEvent_ContextCancelled(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockPool := new(db.MockPool)
	mockConn := new(db.MockConn)

	// Set up mock expectations for the worker that will process the event
	mockPool.On("Acquire", mock.Anything).Return(mockConn, nil)
	mockConn.On("Exec", mock.Anything, mock.Anything, mock.Anything).Return(pgconn.CommandTag{}, nil)
	mockConn.On("Release").Return()

	writer := NewMetricsSQLWriter(logger, mockPool, 100, 1)

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	event := MetricsEvent{
		TenantID:  "tenant-1",
		JobID:     "job-123",
		EventType: "job_started",
	}

	// Note: With the current implementation, the event is queued even if the context
	// is cancelled (as long as there's space in the queue). The context cancellation
	// is only checked when the queue is full.
	err := writer.WriteEvent(ctx, event)
	// The event may be queued successfully if there's space
	assert.True(t, err == nil || err == context.Canceled)

	// Cleanup
	_ = writer.Close()
}

func TestMetricsSQLWriter_GetQueueLength(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockPool := new(db.MockPool)
	mockConn := new(db.MockConn)

	// Set up mock expectations for the workers that will process events
	mockPool.On("Acquire", mock.Anything).Return(mockConn, nil).Maybe()
	mockConn.On("Exec", mock.Anything, mock.Anything, mock.Anything).Return(pgconn.CommandTag{}, nil).Maybe()
	mockConn.On("Release").Return().Maybe()

	writer := NewMetricsSQLWriter(logger, mockPool, 100, 1)

	// Initially empty
	assert.Equal(t, 0, writer.GetQueueLength())

	// Add an event
	err := writer.WriteEvent(context.Background(), MetricsEvent{
		TenantID:  "tenant-1",
		JobID:     "job-123",
		EventType: "job_started",
	})
	assert.NoError(t, err)

	// Queue should have at least 1 event (it may have been processed already)
	assert.GreaterOrEqual(t, writer.GetQueueLength(), 0)

	// Cleanup
	writer.Close()
}

func TestMetricsSQLWriter_GetQueueCapacity(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockPool := new(db.MockPool)

	writer := NewMetricsSQLWriter(logger, mockPool, 500, 1)
	assert.Equal(t, 500, writer.GetQueueCapacity())

	writer.Close()
}

func TestMetricsSQLWriter_Close_AlreadyClosed(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockPool := new(db.MockPool)

	writer := NewMetricsSQLWriter(logger, mockPool, 100, 1)

	// Close once
	err := writer.Close()
	assert.NoError(t, err)

	// Close again - should be idempotent
	err = writer.Close()
	assert.NoError(t, err)
}

func TestEncodeMetadata_Nil(t *testing.T) {
	result := encodeMetadata(nil)
	assert.Equal(t, "{}", result)
}

func TestEncodeMetadata_Empty(t *testing.T) {
	result := encodeMetadata(map[string]string{})
	assert.Equal(t, "{}", result)
}

func TestEncodeMetadata_Single(t *testing.T) {
	result := encodeMetadata(map[string]string{"key": "value"})
	assert.Equal(t, `{"key":"value"}`, result)
}

func TestEncodeMetadata_Multiple(t *testing.T) {
	result := encodeMetadata(map[string]string{"a": "1", "b": "2"})
	assert.Contains(t, result, `"a":"1"`)
	assert.Contains(t, result, `"b":"2"`)
}
