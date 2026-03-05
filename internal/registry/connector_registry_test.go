package registry

import (
	"testing"
)

type mockConnector struct {
	key string
}

func (c *mockConnector) Key() string { return c.key }

func TestConnectorRegistry(t *testing.T) {
	reg := NewConnectorRegistry()
	c := &mockConnector{key: "test-connector"}

	if err := reg.Register(c); err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	found, ok := reg.Lookup("test-connector")
	if !ok || found.Key() != "test-connector" {
		t.Errorf("expected connector found, got %v", ok)
	}
}
