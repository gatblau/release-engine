package runner

import (
	"context"
	"testing"
	"time"

	"github.com/gatblau/release-engine/internal/db"
	"github.com/jackc/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type decisionRow struct{}

func (r *decisionRow) Scan(dest ...interface{}) error {
	*dest[0].(*string) = "approved"
	*dest[1].(*string) = "alice"
	*dest[2].(*string) = "looks good"
	*dest[3].(*time.Time) = time.Now()
	return nil
}

func TestWaitForApprovalRequiresBeginStep(t *testing.T) {
	pool := new(db.MockPool)
	api := NewStepAPIAdapter(pool, "job-1", "run-1", 1)

	_, err := api.WaitForApproval(context.Background(), ApprovalRequest{Summary: "approve"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "BeginStep must be called")
}

func TestWaitForApprovalReturnsDecision(t *testing.T) {
	pool := new(db.MockPool)
	conn := new(db.MockConn)

	api := NewStepAPIAdapter(pool, "job-1", "run-1", 1)
	adapter := api.(*stepAPIAdapter)
	adapter.pollInterval = 1 * time.Millisecond

	assert.NoError(t, api.BeginStep("manual-approval"))

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	pool.On("Acquire", ctx).Return(conn, nil).Once()
	conn.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.CommandTag("INSERT 0 1"), nil).Once()
	conn.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).
		Return(&decisionRow{}).Once()
	conn.On("Release").Return().Once()

	outcome, err := api.WaitForApproval(ctx, ApprovalRequest{
		Summary: "Release to production",
		Detail:  "Requires a human check",
	})

	assert.NoError(t, err)
	assert.Equal(t, "approved", outcome.Decision)
	assert.Equal(t, "alice", outcome.Approver)
	assert.Equal(t, "looks good", outcome.Justification)

	pool.AssertExpectations(t)
	conn.AssertExpectations(t)
}
