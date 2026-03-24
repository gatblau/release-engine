// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gatblau/release-engine/internal/connector"
	"github.com/gatblau/release-engine/internal/module/config"
	"github.com/gatblau/release-engine/internal/module/infra"
	"github.com/stretchr/testify/require"
)

// mockTypedConnectorRegistry implements connector.TypedConnectorRegistry for testing
type mockTypedConnectorRegistry struct {
	gitConnector     connector.GitConnector
	policyConnector  connector.PolicyConnector
	webhookConnector connector.WebhookConnector
}

func (m *mockTypedConnectorRegistry) Register(connector connector.Connector) error { return nil }
func (m *mockTypedConnectorRegistry) Replace(connector connector.Connector) error  { return nil }
func (m *mockTypedConnectorRegistry) Lookup(key string) (connector.Connector, bool) {
	return nil, false
}
func (m *mockTypedConnectorRegistry) ListByType(connectorType connector.ConnectorType) []connector.Connector {
	return nil
}
func (m *mockTypedConnectorRegistry) Close() error { return nil }
func (m *mockTypedConnectorRegistry) ResolveGit(name string) (connector.GitConnector, error) {
	return m.gitConnector, nil
}
func (m *mockTypedConnectorRegistry) ResolvePolicy(name string) (connector.PolicyConnector, error) {
	return m.policyConnector, nil
}
func (m *mockTypedConnectorRegistry) ResolveWebhook(name string) (connector.WebhookConnector, error) {
	return m.webhookConnector, nil
}

// mockConnector implements connector.Connector for testing
type mockConnector struct {
	key string
}

func (m *mockConnector) Key() string                                                   { return m.key }
func (m *mockConnector) Validate(operation string, input map[string]interface{}) error { return nil }
func (m *mockConnector) Execute(ctx context.Context, operation string, input map[string]interface{}, secrets map[string][]byte) (*connector.ConnectorResult, error) {
	return &connector.ConnectorResult{Status: connector.StatusSuccess}, nil
}
func (m *mockConnector) Close() error { return nil }

func TestResolver_ResolveModule_ConfigManagedInfra(t *testing.T) {
	// Create a temporary directory for config files
	tempDir := t.TempDir()

	// Create cfg_infra.yaml in temp directory
	configContent := `apiVersion: module.config/v1
module: infra

vars:
  health_timeout: 5s
  poll_interval: 100ms

connectors:
  families:
    git: git-mock
    policy: policy-mock
    webhook: webhook-mock
`
	configPath := filepath.Join(tempDir, "cfg_infra.yaml")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Set CFG_ROOT to temp directory
	if err = os.Setenv("CFG_ROOT", tempDir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err = os.Unsetenv("CFG_ROOT"); err != nil {
			t.Fatal(err)
		}
	}()

	// Create mock connectors
	mockGit := &mockConnector{key: "git-mock"}
	mockPolicy := &mockConnector{key: "policy-mock"}
	mockWebhook := &mockConnector{key: "webhook-mock"}

	// Create mock registry
	mockRegistry := &mockTypedConnectorRegistry{
		gitConnector:     mockGit,
		policyConnector:  mockPolicy,
		webhookConnector: mockWebhook,
	}

	// Create config loader
	configLoader := config.NewLoader(tempDir)

	// Create legacy registry
	legacyRegistry, err := NewDefaultModuleRegistry()
	require.NoError(t, err)

	// Create resolver
	resolver := NewResolver(configLoader, mockRegistry, legacyRegistry)

	// Test resolving infra module
	ctx := context.Background()
	module, err := resolver.ResolveModule(ctx, infra.ModuleKey, infra.ModuleVersion)
	require.NoError(t, err)
	require.NotNil(t, module)
	require.Equal(t, infra.ModuleKey, module.Key())
	require.Equal(t, infra.ModuleVersion, module.Version())

	// Verify it's a config-managed module (not legacy)
	// The module should have connectors injected via NewModule constructor
}

func TestResolver_ResolveModule_LegacyModule(t *testing.T) {
	// Create mock registry
	mockRegistry := &mockTypedConnectorRegistry{}

	// Create config loader with default path
	configLoader := config.NewLoader(".")

	// Create legacy registry
	legacyRegistry, err := NewDefaultModuleRegistry()
	require.NoError(t, err)

	// Create resolver
	resolver := NewResolver(configLoader, mockRegistry, legacyRegistry)

	// Test resolving infra module (should fall back to legacy since no config file)
	ctx := context.Background()
	_, err = resolver.ResolveModule(ctx, infra.ModuleKey, infra.ModuleVersion)
	// This should fail because infra is marked as config-managed but config is missing
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to assemble config-managed module")
}

func TestResolver_ResolveModule_NonExistentModule(t *testing.T) {
	// Create mock registry
	mockRegistry := &mockTypedConnectorRegistry{}

	// Create config loader
	configLoader := config.NewLoader(".")

	// Create legacy registry
	legacyRegistry, err := NewDefaultModuleRegistry()
	require.NoError(t, err)

	// Create resolver
	resolver := NewResolver(configLoader, mockRegistry, legacyRegistry)

	// Test resolving non-existent module
	ctx := context.Background()
	_, err = resolver.ResolveModule(ctx, "nonexistent.module", "latest")
	require.Error(t, err)
	// Should fail because module is not config-managed and not in legacy registry
	require.Contains(t, err.Error(), "legacy module not found")
}

func TestExtractModuleName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"infra.provision", "infra"},
		{"billing.calculate", "billing"},
		{"audit.report", "audit"},
		{"simple", "simple"},
		{"multi.part.key", "multi"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := extractModuleName(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestNewConfigAwareModuleResolver(t *testing.T) {
	// Create a temporary directory for config files
	tempDir := t.TempDir()
	if err := os.Setenv("CFG_ROOT", tempDir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Unsetenv("CFG_ROOT"); err != nil {
			t.Fatal(err)
		}
	}()

	// Create mock registry
	mockRegistry := &mockTypedConnectorRegistry{}

	// Test creating resolver
	resolver, err := NewConfigAwareModuleResolver(mockRegistry)
	require.NoError(t, err)
	require.NotNil(t, resolver)
}

func TestBootstrapWithConfig(t *testing.T) {
	// Create a temporary directory for config files
	tempDir := t.TempDir()

	// Create cfg_infra.yaml in temp directory
	configContent := `apiVersion: module.config/v1
module: infra

vars:
  health_timeout: 5s
  poll_interval: 100ms

connectors:
  families:
    git: git-mock
    policy: policy-mock
    webhook: webhook-mock
`
	configPath := filepath.Join(tempDir, "cfg_infra.yaml")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Set CFG_ROOT to temp directory
	if err = os.Setenv("CFG_ROOT", tempDir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err = os.Unsetenv("CFG_ROOT"); err != nil {
			t.Fatal(err)
		}
	}()

	// Create mock connectors
	mockGit := &mockConnector{key: "git-mock"}
	mockPolicy := &mockConnector{key: "policy-mock"}
	mockWebhook := &mockConnector{key: "webhook-mock"}

	// Create mock registry
	mockRegistry := &mockTypedConnectorRegistry{
		gitConnector:     mockGit,
		policyConnector:  mockPolicy,
		webhookConnector: mockWebhook,
	}

	// Test bootstrap
	ctx := context.Background()
	registry, err := BootstrapWithConfig(ctx, mockRegistry)
	require.NoError(t, err)
	require.NotNil(t, registry)

	// Lookup infra module
	module, ok := registry.Lookup(infra.ModuleKey, infra.ModuleVersion)
	require.True(t, ok)
	require.NotNil(t, module)
	require.Equal(t, infra.ModuleKey, module.Key())
	require.Equal(t, infra.ModuleVersion, module.Version())
}
