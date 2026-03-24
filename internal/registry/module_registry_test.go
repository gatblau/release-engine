package registry

import (
	"context"
	"testing"
)

type mockModule struct {
	key     string
	version string
}

func (m *mockModule) Key() string     { return m.key }
func (m *mockModule) Version() string { return m.version }
func (m *mockModule) Execute(ctx context.Context, api any, params map[string]any) error {
	return nil
}
func (m *mockModule) Query(ctx context.Context, api any, req QueryRequest) (QueryResult, error) {
	return QueryResult{
		Status: "error",
		Error:  "queries not implemented for mock module",
	}, nil
}
func (m *mockModule) Describe() ModuleDescriptor {
	return ModuleDescriptor{
		Name:   "test",
		Domain: "testing",
	}
}

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

func TestModuleRegistry_ListModules(t *testing.T) {
	reg := NewModuleRegistry()

	// Register multiple modules
	m1 := &mockModule{key: "test1", version: "1.0"}
	m2 := &mockModule{key: "test2", version: "2.0"}

	if err := reg.Register(m1); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if err := reg.Register(m2); err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	descriptors := reg.ListModules()
	if len(descriptors) != 2 {
		t.Errorf("expected 2 descriptors, got %d", len(descriptors))
	}

	// Check that descriptors contain expected names
	foundTest := false
	for _, desc := range descriptors {
		if desc.Name == "test" && desc.Domain == "testing" {
			foundTest = true
			break
		}
	}

	if !foundTest {
		t.Error("expected to find test module descriptor")
	}
}
