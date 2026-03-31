// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

//go:build e2e

package bootstrap

import (
	"context"
	"testing"
	"time"
)

// TestRunE2E is the end-to-end integration test for the bootstrap flow.
// It requires a running infrastructure stack (Dex, Gitea, Postgres, Release Engine).
//
// To run:
//   - With coverage:
//     go test -tags=e2e ./e2e/bootstrap -run TestRunE2E -coverprofile=e2e.cover.out -covermode=atomic
//   - Without coverage:
//     TEST_TIMEOUT=10m go test -tags=e2e ./e2e/bootstrap -run TestRunE2E -v
//
// Environment variables (all optional; defaults match docker-compose setup):
//
//	TEST_TIMEOUT        overall test context deadline (default: 5m)
//	RELEASE_ENGINE_URL Release Engine API URL (default: http://localhost:8080)
//	GITEA_URL          Gitea base URL (default: http://localhost:3000)
//	DEX_URL            Dex OIDC URL (default: http://localhost:5556)
//	TENANT_ID          Tenant ID (default: test-tenant)
//	OIDC_CLIENT_ID     OIDC client ID (default: release-engine)
//	OIDC_CLIENT_SECRET OIDC client secret (default: example-secret)
//	TEST_USERNAME      Test user email (default: test-user@example.com)
//	TEST_PASSWORD      Test user password (default: password)
//	GITEA_ADMIN_USER   Gitea admin username (default: gitadmin)
//	GITEA_ADMIN_PASSWORD Gitea admin password (default: admin-password)
func TestRunE2E(t *testing.T) {
	cfg, err := LoadE2EConfig()
	if err != nil {
		t.Fatalf("LoadE2EConfig failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.TestTimeout)
	defer cancel()

	result, err := RunE2E(ctx, *cfg)
	if err != nil {
		t.Fatalf("E2E flow failed: %v", err)
	}

	// Assertions
	if result.JobID == "" {
		t.Error("expected non-empty JobID")
	}

	// Print a clean summary for CI logs
	t.Logf("=== E2E PASSED ===")
	t.Logf("Job ID:    %s", result.JobID)
	t.Logf("Commit SHA: %s", result.CommitSHA)
}

// TestE2EConfigDefaults verifies that LoadE2EConfig returns valid defaults
// when no environment is set. This test does NOT require running services.
func TestE2EConfigDefaults(t *testing.T) {
	// Clear relevant env vars so we test the defaults path
	envVars := []string{
		"TEST_TIMEOUT", "RELEASE_ENGINE_URL", "GITEA_URL", "DEX_URL",
		"TENANT_ID", "OIDC_CLIENT_ID", "OIDC_CLIENT_SECRET",
		"TEST_USERNAME", "TEST_PASSWORD", "GITEA_ADMIN_USER", "GITEA_ADMIN_PASSWORD",
		"JOB_EXECUTION_TIMEOUT", "API_CLIENT_TIMEOUT",
	}
	for _, k := range envVars {
		t.Setenv(k, "")
	}

	cfg, err := LoadE2EConfig()
	if err != nil {
		t.Fatalf("LoadE2EConfig failed: %v", err)
	}

	// Spot-check defaults
	if cfg.ReleaseEngineURL != "http://localhost:8080" {
		t.Errorf("expected default ReleaseEngineURL, got %s", cfg.ReleaseEngineURL)
	}
	if cfg.GiteaURL != "http://localhost:3000" {
		t.Errorf("expected default GiteaURL, got %s", cfg.GiteaURL)
	}
	if cfg.TenantID != "test-tenant" {
		t.Errorf("expected default TenantID, got %s", cfg.TenantID)
	}
	if cfg.TestTimeout <= 0 {
		t.Errorf("expected positive TestTimeout, got %v", cfg.TestTimeout)
	}
	if cfg.JobExecutionTimeout <= 0 {
		t.Errorf("expected positive JobExecutionTimeout, got %v", cfg.JobExecutionTimeout)
	}
	if cfg.APIClientTimeout <= 0 {
		t.Errorf("expected positive APIClientTimeout, got %v", cfg.APIClientTimeout)
	}
}

// TestE2EConfigOverrides verifies that environment variables override defaults.
func TestE2EConfigOverrides(t *testing.T) {
	t.Setenv("RELEASE_ENGINE_URL", "http://custom:9999")
	t.Setenv("GITEA_URL", "http://custom-gitea:3001")
	t.Setenv("TENANT_ID", "my-tenant")
	t.Setenv("TEST_TIMEOUT", "15m")
	t.Setenv("JOB_EXECUTION_TIMEOUT", "20m")
	t.Setenv("API_CLIENT_TIMEOUT", "30s")

	cfg, err := LoadE2EConfig()
	if err != nil {
		t.Fatalf("LoadE2EConfig failed: %v", err)
	}

	if cfg.ReleaseEngineURL != "http://custom:9999" {
		t.Errorf("expected overridden ReleaseEngineURL, got %s", cfg.ReleaseEngineURL)
	}
	if cfg.GiteaURL != "http://custom-gitea:3001" {
		t.Errorf("expected overridden GiteaURL, got %s", cfg.GiteaURL)
	}
	if cfg.TenantID != "my-tenant" {
		t.Errorf("expected overridden TenantID, got %s", cfg.TenantID)
	}
	if cfg.TestTimeout != 15*time.Minute {
		t.Errorf("expected 15m TestTimeout, got %v", cfg.TestTimeout)
	}
	if cfg.JobExecutionTimeout != 20*time.Minute {
		t.Errorf("expected 20m JobExecutionTimeout, got %v", cfg.JobExecutionTimeout)
	}
	if cfg.APIClientTimeout != 30*time.Second {
		t.Errorf("expected 30s APIClientTimeout, got %v", cfg.APIClientTimeout)
	}
}
