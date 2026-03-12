//go:build integration

package scheduler

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gatblau/release-engine/internal/db"
	"github.com/gatblau/release-engine/internal/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingRunner struct {
	calls []runnerCall
}

type runnerCall struct {
	jobID string
	runID string
}

func (r *recordingRunner) RunJob(_ context.Context, jobID string, runID string) error {
	r.calls = append(r.calls, runnerCall{jobID: jobID, runID: runID})
	return nil
}

func setupSchedulerTestSchema(ctx context.Context, t *testing.T, pool db.Pool) {
	t.Helper()

	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()

	_, err = conn.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS pgcrypto`)
	require.NoError(t, err)

	_, err = conn.Exec(ctx, `CREATE TABLE jobs (
		id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
		state text NOT NULL,
		schedule text,
		attempt int NOT NULL DEFAULT 0,
		owner_id text,
		run_id uuid NOT NULL DEFAULT gen_random_uuid(),
		lease_expires_at timestamptz,
		next_run_at timestamptz,
		started_at timestamptz,
		updated_at timestamptz NOT NULL DEFAULT now()
	)`)
	require.NoError(t, err)

	_, err = conn.Exec(ctx, `CREATE TABLE steps (
		id bigserial PRIMARY KEY,
		job_id uuid NOT NULL,
		status text NOT NULL
	)`)
	require.NoError(t, err)
}

func insertJob(ctx context.Context, t *testing.T, pool db.Pool, state, schedule, timingCol string, timing time.Time, attempt int) (id string, runID string) {
	t.Helper()

	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()

	query := fmt.Sprintf(`INSERT INTO jobs (state, schedule, %s, attempt)
		VALUES ($1, $2, $3, $4)
		RETURNING id::text, run_id::text`, timingCol)
	require.NoError(t, conn.QueryRow(ctx, query, state, schedule, timing, attempt).Scan(&id, &runID))
	return id, runID
}

func getJobState(ctx context.Context, t *testing.T, pool db.Pool, id string) (state string, attempt int, ownerID string, runID string) {
	t.Helper()

	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()

	err = conn.QueryRow(ctx, `
		SELECT state, attempt, COALESCE(owner_id, ''), run_id::text
		FROM jobs
		WHERE id = $1::uuid
	`, id).Scan(&state, &attempt, &ownerID, &runID)
	require.NoError(t, err)
	return state, attempt, ownerID, runID
}

func TestSchedulerService_Integration_ClaimAndDispatch(t *testing.T) {
	ctx := context.Background()
	connStr := db.SetupTestPostgres(ctx, t)

	pool, err := db.NewPool(connStr)
	require.NoError(t, err)
	defer pool.Close()

	setupSchedulerTestSchema(ctx, t, pool)

	now := time.Now().UTC()
	dueQueuedID, dueQueuedRunBefore := insertJob(ctx, t, pool, "queued", "*/5 * * * *", "next_run_at", now.Add(-2*time.Minute), 1)
	blockedQueuedID, _ := insertJob(ctx, t, pool, "queued", "", "next_run_at", now.Add(-1*time.Minute), 0)
	futureQueuedID, _ := insertJob(ctx, t, pool, "queued", "", "next_run_at", now.Add(10*time.Minute), 0)
	expiredRunningID, expiredRunBefore := insertJob(ctx, t, pool, "running", "", "lease_expires_at", now.Add(-1*time.Minute), 5)
	activeRunningID, _ := insertJob(ctx, t, pool, "running", "", "lease_expires_at", now.Add(10*time.Minute), 5)

	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	_, err = conn.Exec(ctx, `INSERT INTO steps (job_id, status) VALUES ($1::uuid, 'waiting_approval')`, blockedQueuedID)
	require.NoError(t, err)
	conn.Release()

	reg := registry.NewModuleRegistry()
	runner := &recordingRunner{}

	svc := NewSchedulerService(pool, reg, nil, 100*time.Millisecond, runner)
	impl := svc.(*schedulerService)
	impl.claimAndDispatch(ctx)

	assert.Len(t, runner.calls, 2, "only due queued and expired running jobs should be dispatched")

	dueState, dueAttempt, dueOwner, dueRunAfter := getJobState(ctx, t, pool, dueQueuedID)
	assert.Equal(t, "running", dueState)
	assert.Equal(t, 2, dueAttempt, "queued claim should increment attempt")
	assert.Equal(t, "scheduler-1", dueOwner)
	assert.NotEqual(t, dueQueuedRunBefore, dueRunAfter, "claim should issue new run_id")

	expiredState, expiredAttempt, expiredOwner, expiredRunAfter := getJobState(ctx, t, pool, expiredRunningID)
	assert.Equal(t, "running", expiredState)
	assert.Equal(t, 5, expiredAttempt, "reclaimed running job should not increment attempt")
	assert.Equal(t, "scheduler-1", expiredOwner)
	assert.NotEqual(t, expiredRunBefore, expiredRunAfter, "reclaim should issue new run_id")

	blockedState, blockedAttempt, blockedOwner, _ := getJobState(ctx, t, pool, blockedQueuedID)
	assert.Equal(t, "queued", blockedState)
	assert.Equal(t, 0, blockedAttempt)
	assert.Equal(t, "", blockedOwner)

	futureState, futureAttempt, futureOwner, _ := getJobState(ctx, t, pool, futureQueuedID)
	assert.Equal(t, "queued", futureState)
	assert.Equal(t, 0, futureAttempt)
	assert.Equal(t, "", futureOwner)

	activeState, activeAttempt, activeOwner, _ := getJobState(ctx, t, pool, activeRunningID)
	assert.Equal(t, "running", activeState)
	assert.Equal(t, 5, activeAttempt)
	assert.Equal(t, "", activeOwner)

	actualRunByJob := map[string]string{}
	for _, c := range runner.calls {
		actualRunByJob[c.jobID] = c.runID
	}
	assert.Equal(t, dueRunAfter, actualRunByJob[dueQueuedID], "dispatched run_id should match claimed row")
	assert.Equal(t, expiredRunAfter, actualRunByJob[expiredRunningID], "dispatched run_id should match claimed row")
}
