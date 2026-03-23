package runner

import (
	"context"
	"testing"
	"time"

	"github.com/gatblau/release-engine/internal/connector"
	"github.com/gatblau/release-engine/internal/db"
	"github.com/gatblau/release-engine/internal/stepapi"
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

type mockConnectorRegistry struct {
	mock.Mock
}

func (m *mockConnectorRegistry) Register(conn connector.Connector) error {
	args := m.Called(conn)
	return args.Error(0)
}

func (m *mockConnectorRegistry) Replace(conn connector.Connector) error {
	args := m.Called(conn)
	return args.Error(0)
}

func (m *mockConnectorRegistry) Lookup(key string) (connector.Connector, bool) {
	args := m.Called(key)
	if args.Get(0) == nil {
		return nil, args.Bool(1)
	}
	return args.Get(0).(connector.Connector), args.Bool(1)
}

func (m *mockConnectorRegistry) ListByType(t connector.ConnectorType) []connector.Connector {
	args := m.Called(t)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).([]connector.Connector)
}

func (m *mockConnectorRegistry) Close() error {
	args := m.Called()
	return args.Error(0)
}

// FamilyRegistry methods
func (m *mockConnectorRegistry) RegisterFamily(family connector.ConnectorFamily) error {
	args := m.Called(family)
	return args.Error(0)
}

func (m *mockConnectorRegistry) BindImplementation(familyName, connectorKey string) error {
	args := m.Called(familyName, connectorKey)
	return args.Error(0)
}

func (m *mockConnectorRegistry) Resolve(familyName string) (connector.Connector, error) {
	args := m.Called(familyName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(connector.Connector), args.Error(1)
}

func (m *mockConnectorRegistry) ValidateBindings() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockConnectorRegistry) GetFamilies() map[string]connector.ConnectorFamily {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(map[string]connector.ConnectorFamily)
}

func (m *mockConnectorRegistry) GetBindings() map[string]string {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(map[string]string)
}

func TestWaitForApprovalRequiresBeginStep(t *testing.T) {
	pool := new(db.MockPool)
	registry := new(mockConnectorRegistry)
	api := NewStepAPIAdapter(pool, registry, "job-1", "run-1", 1)

	_, err := api.WaitForApproval(context.Background(), stepapi.ApprovalRequest{Summary: "approve"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "BeginStep must be called")
}

func TestWaitForApprovalReturnsDecision(t *testing.T) {
	pool := new(db.MockPool)
	conn := new(db.MockConn)
	registry := new(mockConnectorRegistry)

	api := NewStepAPIAdapter(pool, registry, "job-1", "run-1", 1)
	adapter := api.(*stepAPIAdapter)
	adapter.pollInterval = 1 * time.Millisecond

	assert.NoError(t, api.BeginStep("manual-approval"))

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	pool.On("Acquire", ctx).Return(conn, nil).Twice()
	conn.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.CommandTag("INSERT 0 1"), nil).Once()
	conn.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).
		Return(&decisionRow{}).Once()
	conn.On("Release").Return().Twice()

	outcome, err := api.WaitForApproval(ctx, stepapi.ApprovalRequest{
		Summary: "Release to production",
		Detail:  "Requires a human check",
	})

	assert.NoError(t, err)
	assert.Equal(t, "approved", outcome.Decision)
	assert.Equal(t, "alice", outcome.Approver)
	assert.Equal(t, "looks good", outcome.Justification)

	pool.AssertExpectations(t)
	conn.AssertExpectations(t)
	registry.AssertExpectations(t)
}
