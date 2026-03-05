package registry

import (
	"testing"
)

type mockModule struct {
	key     string
	version string
}

func (m *mockModule) Key() string     { return m.key }
func (m *mockModule) Version() string { return m.version }

func TestModuleRegistry(t *testing.T) {
	reg := NewModuleRegistry()
	m := &mockModule{key: "test", version: "1.0"}

	if err := reg.Register(m); err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	found, ok := reg.Lookup("test", "1.0")
	if !ok || found.Key() != "test" {
		t.Errorf("expected module found, got %v", ok)
	}
}
