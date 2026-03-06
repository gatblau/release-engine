//go:build integration

package audit

import (
	"context"
	"testing"

	"github.com/gatblau/release-engine/internal/db"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestAuditService_Integration(t *testing.T) {
	ctx := context.Background()
	connStr := db.SetupTestPostgres(ctx, t)

	pool, err := db.NewPool(connStr)
	assert.NoError(t, err)
	defer pool.Close()

	// Initialise table
	conn, err := pool.Acquire(ctx)
	assert.NoError(t, err)
	_, err = conn.Exec(ctx, `CREATE TABLE audit_log (
		tenant_id TEXT, principal TEXT, action TEXT, target TEXT, 
		decision TEXT, reason TEXT, metadata TEXT, timestamp TIMESTAMP,
		request_id TEXT, job_id TEXT, run_id TEXT, created_at TIMESTAMP
	)`)
	assert.NoError(t, err)
	conn.Release()

	logger := zap.NewNop()
	svc := NewAuditService(logger, pool)

	event := AuditEvent{
		TenantID:  "tenant-1",
		Principal: "user-1",
		Action:    "job:create",
		Target:    "/v1/jobs",
	}

	err = svc.Record(ctx, event)
	assert.NoError(t, err)
}
