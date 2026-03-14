package connector

import (
	"context"
	"testing"
)

type mockConnector struct {
	BaseConnector
}

func (m *mockConnector) Validate(operation string, input map[string]interface{}) error { return nil }
func (m *mockConnector) Execute(ctx context.Context, operation string, input map[string]interface{}) (*ConnectorResult, error) {
	return &ConnectorResult{Status: StatusSuccess}, nil
}
func (m *mockConnector) Close() error { return nil }

func TestRegistry(t *testing.T) {
	reg := NewConnectorRegistry()
	base, _ := NewBaseConnector(ConnectorTypeGit, "github")
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
