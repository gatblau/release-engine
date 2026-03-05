package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/gatblau/release-engine/internal/db"
	"github.com/google/uuid"
)

// LeaseManager manages job lease acquisition and fenced updates.
type LeaseManager interface {
	AcquireJobLease(ctx context.Context, jobID, ownerID string, ttl time.Duration) (runID string, ok bool, err error)
	FinaliseWithFence(ctx context.Context, jobID, runID, state string) (rows int64, err error)
}

type leaseManager struct {
	pool db.Pool
}

// NewLeaseManager creates a new lease manager.
func NewLeaseManager(pool db.Pool) LeaseManager {
	return &leaseManager{pool: pool}
}

func (l *leaseManager) AcquireJobLease(ctx context.Context, jobID, ownerID string, ttl time.Duration) (string, bool, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return "", false, err
	}
	defer conn.Release()

	runID := uuid.New().String()
	// SQL: Update jobs set run_id=$1, lease_expires_at=now()+$2 where job_id=$3 and (lease_expires_at is null or lease_expires_at < now())
	tag, err := conn.Exec(ctx, "UPDATE jobs SET run_id=$1, owner_id=$2, lease_expires_at=now() + $3::interval WHERE job_id=$4 AND (lease_expires_at IS NULL OR lease_expires_at < NOW())",
		runID, ownerID, fmt.Sprintf("%d seconds", int(ttl.Seconds())), jobID)
	if err != nil {
		return "", false, err
	}

	if tag.RowsAffected() == 0 {
		return "", false, &LeaseError{Err: ErrLeaseAcquireConflict, Code: "LEASE_ACQUIRE_CONFLICT"}
	}

	return runID, true, nil
}

func (l *leaseManager) FinaliseWithFence(ctx context.Context, jobID, runID, state string) (int64, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return 0, err
	}
	defer conn.Release()

	// SQL: Update jobs set state=$1, lease_expires_at=null where job_id=$2 and run_id=$3
	tag, err := conn.Exec(ctx, "UPDATE jobs SET state=$1, lease_expires_at=NULL WHERE job_id=$2 AND run_id=$3", state, jobID, runID)
	if err != nil {
		return 0, err
	}

	if tag.RowsAffected() == 0 {
		return 0, &LeaseError{Err: ErrFencedConflict, Code: "ERR_FENCED_CONFLICT"}
	}
	return tag.RowsAffected(), nil
}
