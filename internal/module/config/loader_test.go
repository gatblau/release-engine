// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package config

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLoader_Load_DefaultPath(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cfg_infra.yaml")

	// Write a valid config file
	configContent := `apiVersion: module.config/v1
module: infra
vars:
  health_timeout: 10s
  poll_interval: 200ms
connectors:
  families:
    git: git-file
    policy: policy-mock
    webhook: webhook-mock`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// Create loader with temp directory as default base path
	loader := NewLoader(tmpDir)

	// Load the config
	cfg, err := loader.Load(context.Background(), "infra")
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Verify the loaded config
	if cfg.APIVersion != "module.config/v1" {
		t.Errorf("expected APIVersion 'module.config/v1', got %s", cfg.APIVersion)
	}
	if cfg.Module != "infra" {
		t.Errorf("expected module 'infra', got %s", cfg.Module)
	}
	if cfg.Vars == nil {
		t.Error("expected vars to be non-nil")
	}
	if cfg.Connectors.Families == nil {
		t.Error("expected connectors.families to be non-nil")
	}
	if len(cfg.Connectors.Families) != 3 {
		t.Errorf("expected 3 connector families, got %d", len(cfg.Connectors.Families))
	}
}

func TestLoader_Load_CFG_PATH_Override(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "custom_cfg_infra.yaml")

	// Write a valid config file
	configContent := `apiVersion: module.config/v1
module: infra
connectors:
  families:
    git: git-file`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// Set CFG_PATH_INFRA environment variable
	t.Setenv("CFG_PATH_INFRA", configPath)

	// Create loader with a different default path (should be ignored)
	loader := NewLoader("/some/other/path")

	// Load the config
	cfg, err := loader.Load(context.Background(), "infra")
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Verify the loaded config
	if cfg.APIVersion != "module.config/v1" {
		t.Errorf("expected APIVersion 'module.config/v1', got %s", cfg.APIVersion)
	}
	if cfg.Module != "infra" {
		t.Errorf("expected module 'infra', got %s", cfg.Module)
	}
	if cfg.Connectors.Families["git"] != "git-file" {
		t.Errorf("expected git connector 'git-file', got %s", cfg.Connectors.Families["git"])
	}
}

func TestLoader_Load_CFG_ROOT_Override(t *testing.T) {
	// Create a temporary directory for CFG_ROOT
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cfg_infra.yaml")

	// Write a valid config file
	configContent := `apiVersion: module.config/v1
module: infra
connectors:
  families:
    git: git-file`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// Set CFG_ROOT environment variable
	t.Setenv("CFG_ROOT", tmpDir)

	// Create loader with a different default path (should be ignored since file exists in CFG_ROOT)
	loader := NewLoader("/some/other/path")

	// Load the config
	cfg, err := loader.Load(context.Background(), "infra")
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Verify the loaded config
	if cfg.APIVersion != "module.config/v1" {
		t.Errorf("expected APIVersion 'module.config/v1', got %s", cfg.APIVersion)
	}
	if cfg.Module != "infra" {
		t.Errorf("expected module 'infra', got %s", cfg.Module)
	}
	if cfg.Connectors.Families["git"] != "git-file" {
		t.Errorf("expected git connector 'git-file', got %s", cfg.Connectors.Families["git"])
	}
}

func TestLoader_Load_CFG_ROOT_FallbackToDefault(t *testing.T) {
	// Create a temporary directory for default path
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cfg_infra.yaml")

	// Write a valid config file
	configContent := `apiVersion: module.config/v1
module: infra
connectors:
  families:
    git: git-file`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// Set CFG_ROOT to a directory that doesn't contain the config file
	t.Setenv("CFG_ROOT", "/nonexistent/path")

	// Create loader with the temp directory as default path
	loader := NewLoader(tmpDir)

	// Load the config
	cfg, err := loader.Load(context.Background(), "infra")
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Verify the loaded config
	if cfg.APIVersion != "module.config/v1" {
		t.Errorf("expected APIVersion 'module.config/v1', got %s", cfg.APIVersion)
	}
	if cfg.Module != "infra" {
		t.Errorf("expected module 'infra', got %s", cfg.Module)
	}
	if cfg.Connectors.Families["git"] != "git-file" {
		t.Errorf("expected git connector 'git-file', got %s", cfg.Connectors.Families["git"])
	}
}

func TestLoader_Load_FileNotFound(t *testing.T) {
	// Create loader with a temp directory
	tmpDir := t.TempDir()
	loader := NewLoader(tmpDir)

	// Try to load config that doesn't exist
	_, err := loader.Load(context.Background(), "infra")

	// Should get a ConfigError with CONFIG_FILE_NOT_FOUND
	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("expected ConfigError, got %T: %v", err, err)
	}
	if cfgErr.Code != "CONFIG_FILE_NOT_FOUND" {
		t.Errorf("expected code CONFIG_FILE_NOT_FOUND, got %s", cfgErr.Code)
	}
}

func TestLoader_Load_InvalidYAML(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cfg_infra.yaml")

	// Write invalid YAML
	configContent := `apiVersion: module.config/v1
module: infra
connectors:
  families:
    git: git-file
  invalid: yaml: here`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// Create loader
	loader := NewLoader(tmpDir)

	// Load the config
	_, err := loader.Load(context.Background(), "infra")

	// Should get a ConfigError with CONFIG_INVALID_YAML
	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("expected ConfigError, got %T: %v", err, err)
	}
	if cfgErr.Code != "CONFIG_INVALID_YAML" {
		t.Errorf("expected code CONFIG_INVALID_YAML, got %s", cfgErr.Code)
	}
}

func TestLoader_Load_MissingAPIVersion(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cfg_infra.yaml")

	// Write config missing apiVersion
	configContent := `module: infra
connectors:
  families:
    git: git-file`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// Create loader
	loader := NewLoader(tmpDir)

	// Load the config
	_, err := loader.Load(context.Background(), "infra")

	// Should get a ConfigError with CONFIG_MISSING_FIELD
	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("expected ConfigError, got %T: %v", err, err)
	}
	if cfgErr.Code != "CONFIG_MISSING_FIELD" {
		t.Errorf("expected code CONFIG_MISSING_FIELD, got %s", cfgErr.Code)
	}
	if cfgErr.Detail["field"] != "apiVersion" {
		t.Errorf("expected field 'apiVersion', got %s", cfgErr.Detail["field"])
	}
}

func TestLoader_Load_UnsupportedAPIVersion(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cfg_infra.yaml")

	// Write config with unsupported apiVersion
	configContent := `apiVersion: module.config/v2
module: infra
connectors:
  families:
    git: git-file`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// Create loader
	loader := NewLoader(tmpDir)

	// Load the config
	_, err := loader.Load(context.Background(), "infra")

	// Should get a ConfigError with CONFIG_UNSUPPORTED_API_VERSION
	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("expected ConfigError, got %T: %v", err, err)
	}
	if cfgErr.Code != "CONFIG_UNSUPPORTED_API_VERSION" {
		t.Errorf("expected code CONFIG_UNSUPPORTED_API_VERSION, got %s", cfgErr.Code)
	}
}

func TestLoader_Load_MissingModuleField(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cfg_infra.yaml")

	// Write config missing module field
	configContent := `apiVersion: module.config/v1
connectors:
  families:
    git: git-file`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// Create loader
	loader := NewLoader(tmpDir)

	// Load the config
	_, err := loader.Load(context.Background(), "infra")

	// Should get a ConfigError with CONFIG_MISSING_FIELD
	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("expected ConfigError, got %T: %v", err, err)
	}
	if cfgErr.Code != "CONFIG_MISSING_FIELD" {
		t.Errorf("expected code CONFIG_MISSING_FIELD, got %s", cfgErr.Code)
	}
	if cfgErr.Detail["field"] != "module" {
		t.Errorf("expected field 'module', got %s", cfgErr.Detail["field"])
	}
}

func TestLoader_Load_ModuleMismatch(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cfg_infra.yaml")

	// Write config with wrong module name
	configContent := `apiVersion: module.config/v1
module: billing
connectors:
  families:
    git: git-file`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// Create loader
	loader := NewLoader(tmpDir)

	// Load the config (requesting infra but file says billing)
	_, err := loader.Load(context.Background(), "infra")

	// Should get a ConfigError with CONFIG_MODULE_MISMATCH
	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("expected ConfigError, got %T: %v", err, err)
	}
	if cfgErr.Code != "CONFIG_MODULE_MISMATCH" {
		t.Errorf("expected code CONFIG_MODULE_MISMATCH, got %s", cfgErr.Code)
	}
}

func TestLoader_Load_MissingConnectorFamilies(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cfg_infra.yaml")

	// Write config missing connectors.families
	configContent := `apiVersion: module.config/v1
module: infra`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// Create loader
	loader := NewLoader(tmpDir)

	// Load the config
	_, err := loader.Load(context.Background(), "infra")

	// Should get a ConfigError with CONFIG_MISSING_CONNECTOR_FAMILIES
	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("expected ConfigError, got %T: %v", err, err)
	}
	if cfgErr.Code != "CONFIG_MISSING_CONNECTOR_FAMILIES" {
		t.Errorf("expected code CONFIG_MISSING_CONNECTOR_FAMILIES, got %s", cfgErr.Code)
	}
}

func TestLoader_Load_EmptyDefaultBasePath(t *testing.T) {
	// Create loader with empty default base path
	loader := NewLoader("")

	// Try to load config
	_, err := loader.Load(context.Background(), "infra")

	// Should get a ConfigError with CONFIG_PATH_RESOLUTION_FAILED
	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("expected ConfigError, got %T: %v", err, err)
	}
	if cfgErr.Code != "CONFIG_PATH_RESOLUTION_FAILED" {
		t.Errorf("expected code CONFIG_PATH_RESOLUTION_FAILED, got %s", cfgErr.Code)
	}
}

func TestExampleConfig_ParsesSuccessfully(t *testing.T) {
	// This test verifies that the example config in the config/dev directory parses successfully
	// Note: The dev config is at ../../../config/dev/cfg_infra.yaml relative to this package
	loader := NewLoader("../../../config/dev")

	// Try to load the infra config from the dev config directory
	cfg, err := loader.Load(context.Background(), "infra")
	if err != nil {
		// If the file doesn't exist, skip the test
		var cfgErr *ConfigError
		if errors.As(err, &cfgErr) && cfgErr.Code == "CONFIG_FILE_NOT_FOUND" {
			t.Skip("dev config file not found in config/dev directory")
		}
		t.Fatalf("failed to load dev config: %v", err)
	}

	// Verify the loaded config has the expected structure
	if cfg.APIVersion != "module.config/v1" {
		t.Errorf("expected APIVersion 'module.config/v1', got %s", cfg.APIVersion)
	}
	if cfg.Module != "infra" {
		t.Errorf("expected module 'infra', got %s", cfg.Module)
	}
	if cfg.Connectors.Families == nil {
		t.Error("expected connectors.families to be non-nil")
	}
}

func TestLoader_Load_InvalidModuleName(t *testing.T) {
	// Create loader with a temp directory
	tmpDir := t.TempDir()
	loader := NewLoader(tmpDir)

	// Test various invalid module names
	testCases := []struct {
		name       string
		moduleName string
		expectCode string
	}{
		{
			name:       "contains slash",
			moduleName: "../infra",
			expectCode: "CONFIG_INVALID_MODULE_NAME",
		},
		{
			name:       "contains dot dot",
			moduleName: "..",
			expectCode: "CONFIG_INVALID_MODULE_NAME",
		},
		{
			name:       "contains backslash",
			moduleName: "..\\infra",
			expectCode: "CONFIG_INVALID_MODULE_NAME",
		},
		{
			name:       "empty string",
			moduleName: "",
			expectCode: "CONFIG_INVALID_MODULE_NAME",
		},
		{
			name:       "contains space",
			moduleName: "infra test",
			expectCode: "CONFIG_INVALID_MODULE_NAME",
		},
		{
			name:       "contains special characters",
			moduleName: "infra@test",
			expectCode: "CONFIG_INVALID_MODULE_NAME",
		},
		{
			name:       "contains parentheses",
			moduleName: "infra(test)",
			expectCode: "CONFIG_INVALID_MODULE_NAME",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := loader.Load(context.Background(), tc.moduleName)

			// Should get a ConfigError with CONFIG_INVALID_MODULE_NAME
			var cfgErr *ConfigError
			if !errors.As(err, &cfgErr) {
				t.Fatalf("expected ConfigError, got %T: %v", err, err)
			}
			if cfgErr.Code != tc.expectCode {
				t.Errorf("expected code %s, got %s", tc.expectCode, cfgErr.Code)
			}
		})
	}
}
