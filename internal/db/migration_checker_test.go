package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestMigrationChecker_CurrentVersion(t *testing.T) {
	mockPool := new(MockPool)
	mockConn := new(MockConn)
	checker := NewMigrationChecker(mockPool)

	ctx := context.Background()

	// Expectations
	mockRow := new(mockRow)
	mockRow.On("Scan", mock.Anything).Return(nil)

	mockPool.On("Acquire", ctx).Return(mockConn, nil)
	mockConn.On("QueryRow", ctx, "SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 1", mock.Anything).Return(mockRow)
	mockConn.On("Release").Return()

	version, err := checker.CurrentVersion(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "", version)
}

// Mock for pgx.Row
type mockRow struct {
	mock.Mock
}

func (m *mockRow) Scan(dest ...interface{}) error {
	*dest[0].(*string) = ""
	return m.Called(dest...).Error(0)
}
