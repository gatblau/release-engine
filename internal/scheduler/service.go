package scheduler

import (
	"context"
	"time"

	"github.com/gatblau/release-engine/internal/db"
	"github.com/gatblau/release-engine/internal/registry"
)

// SchedulerService claims runnable jobs and dispatches to runner.
type SchedulerService interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// JobRunner is the dispatch target used by the scheduler.
type JobRunner interface {
	RunJob(ctx context.Context, jobID string, runID string) error
}

type schedulerService struct {
	pool           db.Pool
	moduleRegistry registry.ModuleRegistry
	leaseManager   LeaseManager
	runner         JobRunner
	done           chan struct{}
	interval       time.Duration
}

// NewSchedulerService creates a new scheduler service.

func NewSchedulerService(pool db.Pool, reg registry.ModuleRegistry, lm LeaseManager, interval time.Duration, runners ...JobRunner) SchedulerService {
	var r JobRunner
	if len(runners) > 0 {
		r = runners[0]
	}
	return &schedulerService{
		pool:           pool,
		moduleRegistry: reg,
		leaseManager:   lm,
		runner:         r,
		done:           make(chan struct{}),
		interval:       interval,
	}
}

func (s *schedulerService) Start(ctx context.Context) error {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-s.done:
			return nil
		case <-ticker.C:
			s.claimAndDispatch(ctx)
		}
	}
}

func (s *schedulerService) Stop(ctx context.Context) error {
	close(s.done)
	return nil
}

func (s *schedulerService) claimAndDispatch(ctx context.Context) {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return
	}
	defer conn.Release()

	rows, err := conn.Query(ctx, `
		WITH picked AS (
			SELECT j.id
			FROM jobs j
			WHERE (
				(j.state = 'queued'  AND j.next_run_at      <= now())
				OR
				(j.state = 'running' AND j.lease_expires_at <= now())
			)
			AND NOT EXISTS (
				SELECT 1
				FROM steps s
				WHERE s.job_id = j.id
				  AND s.status = 'waiting_approval'
			)
			ORDER BY
				CASE WHEN j.state = 'running' THEN 0 ELSE 1 END,
				j.next_run_at NULLS FIRST,
				j.lease_expires_at NULLS FIRST
			FOR UPDATE SKIP LOCKED
			LIMIT 10
		)
		UPDATE jobs j
		SET
			state = 'running',
			owner_id = $1,
			run_id = gen_random_uuid(),
			lease_expires_at = now() + $2::interval,
			attempt = CASE WHEN j.state = 'queued' THEN j.attempt + 1 ELSE j.attempt END,
			started_at = COALESCE(j.started_at, now()),
			updated_at = now()
		FROM picked p
		WHERE j.id = p.id
		RETURNING j.id::text, j.run_id::text;
	`, "scheduler-1", "60 seconds")
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var jobID, runID string
		if err := rows.Scan(&jobID, &runID); err != nil {
			continue
		}

		if s.runner != nil {
			_ = s.runner.RunJob(ctx, jobID, runID)
		}
	}
}
