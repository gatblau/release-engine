package runner

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/gatblau/release-engine/internal/db"
	"github.com/gorhill/cronexpr"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type successFinaliseRow struct {
	schedule sql.NullString
	now      time.Time
	err      error
}

func (r *successFinaliseRow) Scan(dest ...interface{}) error {
	if r.err != nil {
		return r.err
	}
	*dest[0].(*sql.NullString) = r.schedule
	*dest[1].(*time.Time) = r.now
	return nil
}

func TestComputeNextRunAt_WithoutSchedule(t *testing.T) {
	next, requeue, err := computeNextRunAt(sql.NullString{}, time.Now())
	require.NoError(t, err)
	assert.False(t, requeue)
	assert.True(t, next.IsZero())
}

func TestComputeNextRunAt_InvalidSchedule(t *testing.T) {
	_, _, err := computeNextRunAt(sql.NullString{Valid: true, String: "not-a-cron"}, time.Now())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse job schedule")
}

func TestRunnerFinaliseSuccess_UnscheduledToSucceeded(t *testing.T) {
	ctx := context.Background()
	pool := new(db.MockPool)
	conn := new(db.MockConn)

	svc := &runnerService{pool: pool}

	pool.On("Acquire", ctx).Return(conn, nil).Once()
	conn.On("Release").Return().Once()
	conn.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).
		Return(&successFinaliseRow{
			schedule: sql.NullString{Valid: false},
			now:      time.Date(2026, 3, 12, 11, 0, 0, 0, time.UTC),
		}).Once()

	conn.On("Exec", ctx,
		mock.MatchedBy(func(q string) bool {
			return strings.Contains(q, "SET state = 'succeeded'")
		}),
		mock.Anything,
	).Return(pgconn.CommandTag("UPDATE 1"), nil).Once()

	conn.On("Exec", ctx,
		mock.MatchedBy(func(q string) bool {
			return strings.Contains(q, "INSERT INTO jobs_read AS r")
		}),
		mock.Anything,
	).Return(pgconn.CommandTag("INSERT 0 1"), nil).Once()

	conn.On("Exec", ctx,
		mock.MatchedBy(func(q string) bool {
			return strings.Contains(q, "INSERT INTO outbox")
		}),
		mock.Anything,
	).Return(pgconn.CommandTag("INSERT 0 1"), nil).Once()

	err := svc.finaliseSuccess(ctx, "job-1", "run-1")
	require.NoError(t, err)

	pool.AssertExpectations(t)
	conn.AssertExpectations(t)
}

func TestRunnerFinaliseSuccess_ScheduledRequeues(t *testing.T) {
	ctx := context.Background()
	pool := new(db.MockPool)
	conn := new(db.MockConn)

	svc := &runnerService{pool: pool}
	base := time.Date(2026, 3, 12, 11, 0, 0, 0, time.UTC)
	expr := cronexpr.MustParse("*/5 * * * *")
	expectedNext := expr.Next(base)

	pool.On("Acquire", ctx).Return(conn, nil).Once()
	conn.On("Release").Return().Once()
	conn.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).
		Return(&successFinaliseRow{
			schedule: sql.NullString{Valid: true, String: "*/5 * * * *"},
			now:      base,
		}).Once()

	conn.On("Exec", ctx,
		mock.MatchedBy(func(q string) bool {
			return strings.Contains(q, "SET state = 'queued'")
		}),
		mock.MatchedBy(func(args []interface{}) bool {
			if len(args) != 3 {
				return false
			}
			next, ok := args[2].(time.Time)
			if !ok {
				return false
			}
			return next.Equal(expectedNext)
		}),
	).Return(pgconn.CommandTag("UPDATE 1"), nil).Once()

	conn.On("Exec", ctx,
		mock.MatchedBy(func(q string) bool {
			return strings.Contains(q, "INSERT INTO jobs_read AS r")
		}),
		mock.Anything,
	).Return(pgconn.CommandTag("INSERT 0 1"), nil).Once()

	conn.On("Exec", ctx,
		mock.MatchedBy(func(q string) bool {
			return strings.Contains(q, "INSERT INTO outbox")
		}),
		mock.Anything,
	).Return(pgconn.CommandTag("INSERT 0 1"), nil).Once()

	err := svc.finaliseSuccess(ctx, "job-1", "run-1")
	require.NoError(t, err)

	pool.AssertExpectations(t)
	conn.AssertExpectations(t)
}

func TestRunnerFinaliseSuccess_FencedConflictOnUpdate(t *testing.T) {
	ctx := context.Background()
	pool := new(db.MockPool)
	conn := new(db.MockConn)

	svc := &runnerService{pool: pool}

	pool.On("Acquire", ctx).Return(conn, nil).Once()
	conn.On("Release").Return().Once()
	conn.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).
		Return(&successFinaliseRow{schedule: sql.NullString{Valid: false}, now: time.Now()}).Once()

	conn.On("Exec", ctx,
		mock.MatchedBy(func(q string) bool {
			return strings.Contains(q, "SET state = 'succeeded'")
		}),
		mock.Anything,
	).Return(pgconn.CommandTag("UPDATE 0"), nil).Once()

	err := svc.finaliseSuccess(ctx, "job-1", "run-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fenced conflict while finalising success")

	pool.AssertExpectations(t)
	conn.AssertExpectations(t)
}

func TestRunnerFinaliseSuccess_FencedConflictOnMissingRow(t *testing.T) {
	ctx := context.Background()
	pool := new(db.MockPool)
	conn := new(db.MockConn)

	svc := &runnerService{pool: pool}

	pool.On("Acquire", ctx).Return(conn, nil).Once()
	conn.On("Release").Return().Once()
	conn.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).
		Return(&successFinaliseRow{err: pgx.ErrNoRows}).Once()

	err := svc.finaliseSuccess(ctx, "job-1", "run-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fenced conflict while finalising success")

	pool.AssertExpectations(t)
	conn.AssertExpectations(t)
}
