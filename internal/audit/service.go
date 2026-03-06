package audit

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gatblau/release-engine/internal/db"
	"go.uber.org/zap"
)

// Auditor persists and emits audit events.
type Auditor interface {
	Record(ctx context.Context, event AuditEvent) error
	Close() error
}

// AuditEvent represents an audit event for policy decisions, administrative actions, and sensitive runtime transitions.
type AuditEvent struct {
	TenantID  string            `json:"tenant_id"`
	Principal string            `json:"principal"`
	Action    string            `json:"action"`
	Target    string            `json:"target"`
	Decision  string            `json:"decision"`
	Reason    string            `json:"reason"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
	RequestID string            `json:"request_id,omitempty"`
	JobID     string            `json:"job_id,omitempty"`
	RunID     string            `json:"run_id,omitempty"`
}

// AuditService implements the Auditor interface.
type AuditService struct {
	db     db.Pool
	logger *zap.Logger
}

// NewAuditService creates a new AuditService.
// Implements Phase 3: AuditService spec - persist and emit audit events.
func NewAuditService(logger *zap.Logger, pool db.Pool) *AuditService {
	return &AuditService{
		db:     pool,
		logger: logger,
	}
}

// Record records an audit event to the audit_log table and emits a structured log.
// Implements Phase 3: AuditService spec - write immutable event to audit_log table.
func (s *AuditService) Record(ctx context.Context, event AuditEvent) error {
	// Validate event
	if event.TenantID == "" {
		s.logger.Error("audit.failure",
			zap.String("component", "AuditService"),
			zap.Error(NewAuditError(
				ErrAuditEventInvalid,
				"AUDIT_EVENT_INVALID",
				map[string]string{"error": "tenant_id is required"},
			)))
		return NewAuditError(
			ErrAuditEventInvalid,
			"AUDIT_EVENT_INVALID",
			map[string]string{"error": "tenant_id is required"},
		)
	}

	if event.Action == "" {
		s.logger.Error("audit.failure",
			zap.String("component", "AuditService"),
			zap.Error(NewAuditError(
				ErrAuditEventInvalid,
				"AUDIT_EVENT_INVALID",
				map[string]string{"error": "action is required"},
			)))
		return NewAuditError(
			ErrAuditEventInvalid,
			"AUDIT_EVENT_INVALID",
			map[string]string{"error": "action is required"},
		)
	}

	// Set timestamp if not provided
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	// Acquire connection from pool
	conn, err := s.db.Acquire(ctx)
	if err != nil {
		s.logger.Error("audit.failure",
			zap.String("component", "AuditService"),
			zap.String("tenant_id", event.TenantID),
			zap.Error(err))
		return NewAuditError(
			ErrAuditWriteFailed,
			"AUDIT_WRITE_FAILED",
			map[string]string{"error": err.Error()},
		)
	}
	defer conn.Release()

	// Insert audit event into audit_log table
	metadataJSON, err := json.Marshal(event.Metadata)
	if err != nil {
		metadataJSON = []byte("{}")
	}

	_, err = conn.Exec(ctx,
		`INSERT INTO audit_log 
		(tenant_id, principal, action, target, decision, reason, metadata, timestamp, request_id, job_id, run_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW())`,
		event.TenantID,
		event.Principal,
		event.Action,
		event.Target,
		event.Decision,
		event.Reason,
		metadataJSON,
		event.Timestamp,
		event.RequestID,
		event.JobID,
		event.RunID,
	)

	if err != nil {
		s.logger.Error("audit.failure",
			zap.String("component", "AuditService"),
			zap.String("tenant_id", event.TenantID),
			zap.String("action", event.Action),
			zap.Error(err))
		return NewAuditError(
			ErrAuditWriteFailed,
			"AUDIT_WRITE_FAILED",
			map[string]string{"error": err.Error()},
		)
	}

	// Emit structured log for SIEM ingestion
	s.logger.Info("audit.event",
		zap.String("component", "AuditService"),
		zap.String("tenant_id", event.TenantID),
		zap.String("principal", event.Principal),
		zap.String("action", event.Action),
		zap.String("target", event.Target),
		zap.String("decision", event.Decision),
		zap.String("reason", event.Reason),
		zap.String("request_id", event.RequestID),
		zap.String("job_id", event.JobID),
		zap.String("run_id", event.RunID),
	)

	return nil
}

// RecordPolicyDecision records a policy decision audit event.
func (s *AuditService) RecordPolicyDecision(ctx context.Context, tenantID, principal, action, target, decision, reason string) error {
	return s.Record(ctx, AuditEvent{
		TenantID:  tenantID,
		Principal: principal,
		Action:    action,
		Target:    target,
		Decision:  decision,
		Reason:    reason,
	})
}

// RecordJobAction records a job-related audit event.
func (s *AuditService) RecordJobAction(ctx context.Context, tenantID, principal, action, jobID, decision, reason string) error {
	return s.Record(ctx, AuditEvent{
		TenantID:  tenantID,
		Principal: principal,
		Action:    action,
		JobID:     jobID,
		Decision:  decision,
		Reason:    reason,
	})
}

// RecordAdminAction records an administrative action audit event.
func (s *AuditService) RecordAdminAction(ctx context.Context, tenantID, principal, action, target, decision, reason string) error {
	return s.Record(ctx, AuditEvent{
		TenantID:  tenantID,
		Principal: principal,
		Action:    action,
		Target:    target,
		Decision:  decision,
		Reason:    reason,
	})
}

// Close closes the audit service (no-op for current implementation).
func (s *AuditService) Close() error {
	s.logger.Info("audit.success",
		zap.String("component", "AuditService"))
	return nil
}
