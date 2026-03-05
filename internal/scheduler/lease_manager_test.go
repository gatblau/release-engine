package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/gatblau/release-engine/internal/db"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockPool struct {
	mock.Mock
}

func (m *MockPool) Acquire(ctx context.Context) (db.Conn, error) {
	args := m.Called(ctx)
	return args.Get(0).(db.Conn), args.Error(1)
}

func (m *MockPool) Ping(ctx context.Context) error {
	return m.Called(ctx).Error(0)
}

func (m *MockPool) Close() {
	m.Called()
}

type MockConn struct {
	mock.Mock
}

func (m *MockConn) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	a := m.Called(ctx, sql, args)
	return a.Get(0).(pgx.Rows), a.Error(1)
}

func (m *MockConn) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	a := m.Called(ctx, sql, args)
	return a.Get(0).(pgx.Row)
}

func (m *MockConn) Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error) {
	a := m.Called(ctx, sql, arguments)
	return a.Get(0).(pgconn.CommandTag), a.Error(1)
}

func (m *MockConn) Release() {
	m.Called()
}

func TestAcquireJobLeaseSuccess(t *testing.T) {
	mPool := new(MockPool)
	mConn := new(MockConn)
	leaseManager := NewLeaseManager(mPool)

	ctx := context.Background()
	mPool.On("Acquire", ctx).Return(mConn, nil)
	mConn.On("Exec", ctx, mock.Anything, mock.Anything).Return(pgconn.CommandTag("UPDATE 1"), nil)
	mConn.On("Release").Return()

	runID, ok, err := leaseManager.AcquireJobLease(ctx, "job1", "owner1", time.Minute)

	assert.NoError(t, err)
	assert.True(t, ok)
	assert.NotEmpty(t, runID)
	mPool.AssertExpectations(t)
	mConn.AssertExpectations(t)
}

func TestAcquireJobLeaseConflict(t *testing.T) {
	mPool := new(MockPool)
	mConn := new(MockConn)
	leaseManager := NewLeaseManager(mPool)

	ctx := context.Background()
	mPool.On("Acquire", ctx).Return(mConn, nil)
	mConn.On("Exec", ctx, mock.Anything, mock.Anything).Return(pgconn.CommandTag("UPDATE 0"), nil)
	mConn.On("Release").Return()

	_, ok, err := leaseManager.AcquireJobLease(ctx, "job1", "owner1", time.Minute)

	assert.Error(t, err)
	assert.False(t, ok)
	assert.Contains(t, err.Error(), "LEASE_ACQUIRE_CONFLICT")
	mPool.AssertExpectations(t)
	mConn.AssertExpectations(t)
}
