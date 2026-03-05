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

type schedulerService struct {
	pool           db.Pool
	moduleRegistry registry.ModuleRegistry
	leaseManager   LeaseManager
	done           chan struct{}
	interval       time.Duration
}

// NewSchedulerService creates a new scheduler service.
func NewSchedulerService(pool db.Pool, reg registry.ModuleRegistry, lm LeaseManager, interval time.Duration) SchedulerService {
	return &schedulerService{
		pool:           pool,
		moduleRegistry: reg,
		leaseManager:   lm,
		done:           make(chan struct{}),
		interval:       interval,
	}
}

func (s *schedulerService) Start(ctx context.Context) error {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
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

	// Query: SELECT job_id FROM jobs ...
	rows, err := conn.Query(ctx, "SELECT job_id FROM jobs WHERE lease_expires_at IS NULL OR lease_expires_at < NOW() ORDER BY job_id LIMIT 10 FOR UPDATE SKIP LOCKED")
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var jobID string
		if err := rows.Scan(&jobID); err != nil {
			continue
		}

		_, _, err = s.leaseManager.AcquireJobLease(ctx, jobID, "scheduler-1", 60*time.Second)
		if err == nil {
			// Trigger RunnerService here
			_ = jobID
		}
	}
}
