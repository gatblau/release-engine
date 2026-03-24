package connector

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockConnector struct {
	BaseConnector
}

func (m *mockConnector) Validate(operation string, input map[string]interface{}) error { return nil }
func (m *mockConnector) Execute(ctx context.Context, operation string, input map[string]interface{}, secrets map[string][]byte) (*ConnectorResult, error) {
	return &ConnectorResult{Status: StatusSuccess}, nil
}
func (m *mockConnector) Close() error { return nil }

func TestRegistry(t *testing.T) {
	reg := NewConnectorRegistry()
	base, err := NewBaseConnector(ConnectorTypeGit, "github")
	require.NoError(t, err)
	conn := &mockConnector{BaseConnector: base}

	if err := reg.Register(conn); err != nil {
		t.Fatalf("failed to register connector: %v", err)
	}

	c, ok := reg.Lookup(conn.Key())
	if !ok || c != conn {
		t.Fatal("failed to lookup connector")
	}

	if err := reg.Close(); err != nil {
		t.Fatalf("failed to close registry: %v", err)
	}
}

func TestTypedConnectorRegistry_ResolveGit(t *testing.T) {
	reg := NewTypedConnectorRegistry()

	// Create a git connector
	base, err := NewBaseConnector(ConnectorTypeGit, "github")
	require.NoError(t, err)
	conn := &mockConnector{BaseConnector: base}

	// Register the connector
	require.NoError(t, reg.Register(conn))

	// Test resolving by name "github" (should map to "git-github")
	gitConn, err := reg.ResolveGit("github")
	require.NoError(t, err)
	assert.Equal(t, conn.Key(), gitConn.Key())

	// Test resolving non-existent connector
	_, err = reg.ResolveGit("nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git connector not found")

	// Test resolving wrong type connector - trying to resolve "policy" as git
	// when only a policy connector (other-policy) exists
	baseOther, err := NewBaseConnector(ConnectorTypeOther, "policy")
	require.NoError(t, err)
	otherConn := &mockConnector{BaseConnector: baseOther}
	require.NoError(t, reg.Register(otherConn))

	// Try to resolve "policy" as git connector (should fail with not found)
	_, err = reg.ResolveGit("policy")
	require.Error(t, err)
	// Should fail because "git-policy" doesn't exist (we have "other-policy")
	assert.Contains(t, err.Error(), "git connector not found")
}

func TestTypedConnectorRegistry_ResolvePolicy(t *testing.T) {
	reg := NewTypedConnectorRegistry()

	// Create a policy connector (ConnectorTypeOther)
	base, err := NewBaseConnector(ConnectorTypeOther, "policy")
	require.NoError(t, err)
	conn := &mockConnector{BaseConnector: base}

	// Register the connector
	require.NoError(t, reg.Register(conn))

	// Test resolving by name "policy" (should map to "other-policy")
	policyConn, err := reg.ResolvePolicy("policy")
	require.NoError(t, err)
	assert.Equal(t, conn.Key(), policyConn.Key())

	// Test resolving non-existent connector
	_, err = reg.ResolvePolicy("nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "policy connector not found")
}

func TestTypedConnectorRegistry_ResolveWebhook(t *testing.T) {
	reg := NewTypedConnectorRegistry()

	// Create a webhook connector (ConnectorTypeOther)
	base, err := NewBaseConnector(ConnectorTypeOther, "webhook")
	require.NoError(t, err)
	conn := &mockConnector{BaseConnector: base}

	// Register the connector
	require.NoError(t, reg.Register(conn))

	// Test resolving by name "webhook" (should map to "other-webhook")
	webhookConn, err := reg.ResolveWebhook("webhook")
	require.NoError(t, err)
	assert.Equal(t, conn.Key(), webhookConn.Key())

	// Test resolving non-existent connector
	_, err = reg.ResolveWebhook("nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "webhook connector not found")
}

func TestTypedConnectorRegistry_InterfaceCompatibility(t *testing.T) {
	// Verify that NewTypedConnectorRegistry returns a TypedConnectorRegistry
	reg := NewTypedConnectorRegistry()

	// Should implement TypedConnectorRegistry interface
	var typedReg = reg
	assert.NotNil(t, typedReg)

	// Should also implement ConnectorRegistry interface
	var baseReg ConnectorRegistry = reg
	assert.NotNil(t, baseReg)

	// Verify methods are available
	base, err := NewBaseConnector(ConnectorTypeGit, "github")
	require.NoError(t, err)
	conn := &mockConnector{BaseConnector: base}

	// Register the connector once (both interfaces point to same registry)
	require.NoError(t, typedReg.Register(conn))

	// Both interfaces should work with the same underlying registry
	git1, err := typedReg.ResolveGit("github")
	require.NoError(t, err)
	assert.NotNil(t, git1)

	conn2, ok := baseReg.Lookup("git-github")
	require.True(t, ok)
	assert.Equal(t, conn, conn2)
}
