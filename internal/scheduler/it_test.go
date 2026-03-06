//go:build integration

package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/gatblau/release-engine/internal/db"
	"github.com/gatblau/release-engine/internal/registry"
	"github.com/stretchr/testify/assert"
)

func TestSchedulerService_Integration(t *testing.T) {
	ctx := context.Background()
	connStr := db.SetupTestPostgres(ctx, t)

	pool, err := db.NewPool(connStr)
	assert.NoError(t, err)
	defer pool.Close()

	// Initialise table
	conn, err := pool.Acquire(ctx)
	assert.NoError(t, err)
	_, err = conn.Exec(ctx, `CREATE TABLE jobs (
		job_id TEXT PRIMARY KEY,
		tenant_id TEXT,
		run_id TEXT,
		owner_id TEXT,
		state TEXT,
		lease_expires_at TIMESTAMP
	)`)
	assert.NoError(t, err)
	conn.Release()

	reg := registry.NewModuleRegistry()
	lm := NewLeaseManager(pool)

	svc := NewSchedulerService(pool, reg, lm, 100*time.Millisecond)

	// Test the claim method with potential job entries
	// (requires proper data setup and testing of claimAndDispatch)
	_ = svc
}
