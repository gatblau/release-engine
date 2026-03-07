package scheduler

import (
	"context"
	"fmt"
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

// Note: claimAndDispatch tests require proper pgx.Rows mocking which is complex.
// The function is tested via integration tests (it_test.go).
// Below we test the error types which are simpler to test.

// Test scheduler error types
func TestSchedulerError(t *testing.T) {
	// Create a scheduler error
	innerErr := fmt.Errorf("inner error")
	err := &SchedulerError{
		Err:    innerErr,
		Code:   "TEST_CODE",
		Detail: map[string]string{"key": "value"},
	}

	// Test Error() method - should contain the code and error message
	assert.Contains(t, err.Error(), "TEST_CODE")
	assert.Contains(t, err.Error(), "inner error")

	// Test Unwrap() method
	assert.Equal(t, innerErr, err.Unwrap())
}

func TestSchedulerError_WithWrappedError(t *testing.T) {
	// Create a wrapped error
	wrappedErr := fmt.Errorf("wrapped: %w", assert.AnError)
	err := &SchedulerError{
		Err:  wrappedErr,
		Code: "WRAPPED_CODE",
	}

	// Test Error() method - should contain the code
	assert.Contains(t, err.Error(), "WRAPPED_CODE")

	// Test Unwrap() method - should return the wrapped error
	assert.Equal(t, wrappedErr, err.Unwrap())
}
