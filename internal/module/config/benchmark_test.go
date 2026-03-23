// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkLoader_Load(b *testing.B) {
	// Setup: create a temporary config file
	tempDir := b.TempDir()
	configContent := `apiVersion: module.config/v1
module: infra

vars:
  health_timeout: 30s
  poll_interval: 500ms

connectors:
  families:
    git: git-file
    policy: policy-mock
    webhook: webhook-mock
`
	configPath := filepath.Join(tempDir, "cfg_infra.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		b.Fatal(err)
	}

	// Create loader
	loader := NewLoader(tempDir)
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := loader.Load(ctx, "infra")
			if err != nil {
				b.Fatalf("Failed to load config: %v", err)
			}
		}
	})
}

func BenchmarkLoader_Load_WithEnvVarOverride(b *testing.B) {
	// Setup: create two config files to test env var override
	tempDir := b.TempDir()
	configContent := `apiVersion: module.config/v1
module: infra

vars:
  health_timeout: 30s
  poll_interval: 500ms

connectors:
  families:
    git: git-file
    policy: policy-mock
    webhook: webhook-mock
`
	configPath := filepath.Join(tempDir, "cfg_infra.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		b.Fatal(err)
	}

	// Create another config file in a different directory for CFG_PATH override
	overrideDir := filepath.Join(b.TempDir(), "override")
	if err := os.MkdirAll(overrideDir, 0755); err != nil {
		b.Fatal(err)
	}
	overrideConfigPath := filepath.Join(overrideDir, "cfg_infra.yaml")
	if err := os.WriteFile(overrideConfigPath, []byte(configContent), 0644); err != nil {
		b.Fatal(err)
	}

	// Set CFG_PATH_INFRA env var
	if err := os.Setenv("CFG_PATH_INFRA", overrideConfigPath); err != nil {
		b.Fatal(err)
	}

	defer func() {
		if err := os.Unsetenv("CFG_PATH_INFRA"); err != nil {
			b.Fatal(err)
		}
	}()

	// Create loader with default path to tempDir (should be overridden by env var)
	loader := NewLoader(tempDir)
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := loader.Load(ctx, "infra")
			if err != nil {
				b.Fatalf("Failed to load config: %v", err)
			}
		}
	})
}

func BenchmarkLoader_Load_WithCFGRoot(b *testing.B) {
	// Setup: create config directory structure with CFG_ROOT
	cfgRootDir := b.TempDir()
	configContent := `apiVersion: module.config/v1
module: infra

vars:
  health_timeout: 30s
  poll_interval: 500ms

connectors:
  families:
    git: git-file
    policy: policy-mock
    webhook: webhook-mock
`
	configPath := filepath.Join(cfgRootDir, "cfg_infra.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		b.Fatal(err)
	}

	// Set CFG_ROOT env var
	if err := os.Setenv("CFG_ROOT", cfgRootDir); err != nil {
		b.Fatal(err)
	}
	defer func() {
		if err := os.Unsetenv("CFG_ROOT"); err != nil {
			b.Fatal(err)
		}
	}()

	// Create loader with a different default base path
	loader := NewLoader("/some/other/path")
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := loader.Load(ctx, "infra")
			if err != nil {
				b.Fatalf("Failed to load config: %v", err)
			}
		}
	})
}

func BenchmarkLoader_Load_MissingConfig(b *testing.B) {
	// Setup: create loader pointing to empty directory
	tempDir := b.TempDir()
	loader := NewLoader(tempDir)
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := loader.Load(ctx, "nonexistent")
			// Expect error, just benchmark the path
			_ = err
		}
	})
}

func BenchmarkLoader_Load_ParseLargeConfig(b *testing.B) {
	// Setup: create a config with many variables
	tempDir := b.TempDir()

	// Build a large config with many variables
	var configContent = `apiVersion: module.config/v1
module: infra

vars:
  health_timeout: 30s
  poll_interval: 500ms
`
	// Add many additional variables
	for i := 0; i < 100; i++ {
		configContent += "  var_" + string(rune('a'+(i%26))) + "_" + string(rune('0'+(i%10))) + ": value" + string(rune('0'+(i%10))) + "\n"
	}

	configContent += `
connectors:
  families:
    git: git-file
    policy: policy-mock
    webhook: webhook-mock
`

	configPath := filepath.Join(tempDir, "cfg_infra.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		b.Fatal(err)
	}

	loader := NewLoader(tempDir)
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := loader.Load(ctx, "infra")
			if err != nil {
				b.Fatalf("Failed to load config: %v", err)
			}
		}
	})
}
