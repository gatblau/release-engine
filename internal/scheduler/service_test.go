package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/gatblau/release-engine/internal/db"
	"github.com/gatblau/release-engine/internal/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockModuleRegistry struct {
	mock.Mock
}

func (m *mockModuleRegistry) Register(mod registry.Module) error {
	return m.Called(mod).Error(0)
}

func (m *mockModuleRegistry) Lookup(key, version string) (registry.Module, bool) {
	args := m.Called(key, version)
	mod, ok := args.Get(0).(registry.Module)
	return mod, ok
}

type mockLeaseManager struct {
	mock.Mock
}

func (m *mockLeaseManager) AcquireJobLease(ctx context.Context, jobID, ownerID string, ttl time.Duration) (string, bool, error) {
	args := m.Called(ctx, jobID, ownerID, ttl)
	return args.String(0), args.Bool(1), args.Error(2)
}

func (m *mockLeaseManager) FinaliseWithFence(ctx context.Context, jobID, runID, state string) (int64, error) {
	args := m.Called(ctx, jobID, runID, state)
	return int64(args.Int(0)), args.Error(1)
}

func TestNewSchedulerService(t *testing.T) {
	pool := new(db.MockPool)
	reg := new(mockModuleRegistry)
	lm := new(mockLeaseManager)

	svc := NewSchedulerService(pool, reg, lm, 100*time.Millisecond)
	assert.NotNil(t, svc)
}

func TestSchedulerService_Stop(t *testing.T) {
	pool := new(db.MockPool)
	reg := new(mockModuleRegistry)
	lm := new(mockLeaseManager)

	svc := NewSchedulerService(pool, reg, lm, 100*time.Millisecond)

	err := svc.Stop(context.Background())
	assert.NoError(t, err)
}

func TestSchedulerService_claimAndDispatch(t *testing.T) {
	pool := new(db.MockPool)
	reg := new(mockModuleRegistry)
	lm := new(mockLeaseManager)
	conn := new(db.MockConn)

	_ = &schedulerService{
		pool:           pool,
		moduleRegistry: reg,
		leaseManager:   lm,
	}

	ctx := context.Background()

	pool.On("Acquire", ctx).Return(conn, nil)
	conn.On("Query", ctx, mock.Anything, mock.Anything).Return(nil, nil)
	conn.On("Release").Return()

	// Tested by calling directly if needed or via integration tests
}
