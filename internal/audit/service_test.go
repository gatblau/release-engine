package audit

import (
	"context"
	"testing"
	"time"

	"github.com/gatblau/release-engine/internal/db"
	"github.com/jackc/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

func TestNewAuditService(t *testing.T) {
	logger := zap.NewNop()
	mockPool := new(db.MockPool)

	service := NewAuditService(logger, mockPool)
	assert.NotNil(t, service)
}

func TestAuditService_Record(t *testing.T) {
	logger := zap.NewNop()
	mockPool := new(db.MockPool)
	mockConn := new(db.MockConn)

	service := NewAuditService(logger, mockPool)

	ctx := context.Background()
	event := AuditEvent{
		TenantID:  "tenant-1",
		Principal: "user-1",
		Action:    "job:create",
		Target:    "/v1/jobs",
	}

	// Expectations
	mockPool.On("Acquire", ctx).Return(mockConn, nil)
	mockConn.On("Exec", ctx, mock.Anything, mock.Anything).Return(pgconn.CommandTag([]byte{}), nil)
	mockConn.On("Release").Return()

	err := service.Record(ctx, event)

	assert.NoError(t, err)
	mockPool.AssertExpectations(t)
	mockConn.AssertExpectations(t)
}

func TestNewAuditError(t *testing.T) {
	err := NewAuditError(ErrAuditWriteFailed, "AUDIT_WRITE_FAILED", map[string]string{"detail": "test"})

	assert.Equal(t, "AUDIT_WRITE_FAILED: audit write failed", err.Error())
	assert.Equal(t, ErrAuditWriteFailed, err.Unwrap())
	assert.Equal(t, "test", err.Detail["detail"])
}

func TestAuditEvent_Timestamp(t *testing.T) {
	event := AuditEvent{
		TenantID: "test-tenant",
		Action:   "test:action",
	}

	assert.True(t, event.Timestamp.IsZero())

	event.Timestamp = time.Now()
	assert.False(t, event.Timestamp.IsZero())
}

func TestAuditEvent_Fields(t *testing.T) {
	event := AuditEvent{
		TenantID:  "tenant-1",
		Principal: "user-1",
		Action:    "job:create",
		Target:    "/v1/jobs",
		Decision:  "allow",
		Reason:    "authorised",
		JobID:     "job-123",
		RequestID: "req-456",
		RunID:     "run-789",
		Metadata:  map[string]string{"key": "value"},
	}

	assert.Equal(t, "tenant-1", event.TenantID)
	assert.Equal(t, "user-1", event.Principal)
	assert.Equal(t, "job:create", event.Action)
	assert.Equal(t, "/v1/jobs", event.Target)
	assert.Equal(t, "allow", event.Decision)
	assert.Equal(t, "authorised", event.Reason)
	assert.Equal(t, "job-123", event.JobID)
	assert.Equal(t, "req-456", event.RequestID)
	assert.Equal(t, "run-789", event.RunID)
	assert.Equal(t, "value", event.Metadata["key"])
}

func TestAuditService_ConvenienceMethods(t *testing.T) {
	logger := zap.NewNop()
	mockPool := new(db.MockPool)
	mockConn := new(db.MockConn)
	service := NewAuditService(logger, mockPool)
	ctx := context.Background()

	mockPool.On("Acquire", ctx).Return(mockConn, nil)
	mockConn.On("Exec", ctx, mock.Anything, mock.Anything).Return(pgconn.CommandTag([]byte{}), nil)
	mockConn.On("Release").Return()
	mockPool.On("Acquire", ctx).Return(mockConn, nil)
	mockConn.On("Exec", ctx, mock.Anything, mock.Anything).Return(pgconn.CommandTag([]byte{}), nil)
	mockConn.On("Release").Return()
	mockPool.On("Acquire", ctx).Return(mockConn, nil)
	mockConn.On("Exec", ctx, mock.Anything, mock.Anything).Return(pgconn.CommandTag([]byte{}), nil)
	mockConn.On("Release").Return()

	assert.NoError(t, service.RecordPolicyDecision(ctx, "t", "p", "a", "t", "d", "r"))
	assert.NoError(t, service.RecordJobAction(ctx, "t", "p", "a", "j", "d", "r"))
	assert.NoError(t, service.RecordAdminAction(ctx, "t", "p", "a", "t", "d", "r"))
	assert.NoError(t, service.Close())
}
