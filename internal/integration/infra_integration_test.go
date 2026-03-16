//go:build integration

// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	connectortesting "github.com/gatblau/release-engine/internal/connector/testing"
	"github.com/gatblau/release-engine/internal/module/infra"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestInfraIntegration_FullLifecycleAndCrossplaneValidation(t *testing.T) {
	harness := NewIntegrationTestHarness(t)
	harness.RegisterModule(infra.NewModule())

	fileGit, err := connectortesting.NewFileGitConnector(harness.TempDir)
	require.NoError(t, err)
	harness.RegisterConnector(fileGit)

	jobID := harness.CreateJob(JobOptions{
		TenantID:       "tenant-integration",
		PathKey:        infra.ModuleKey,
		IdempotencyKey: "infra-it-lifecycle-1",
		Params:         loadParamsFixture(t, "infra-k8s-app"),
	})

	require.NoError(t, harness.RunSchedulerCycle())
	require.NoError(t, harness.WaitForJobState(jobID, "succeeded", 5*time.Second))

	state, attempt, err := harness.JobState(jobID)
	require.NoError(t, err)
	require.Equal(t, "succeeded", state)
	require.Equal(t, 1, attempt)

	manifest := fetchLatestManifestYAML(t, harness, jobID)
	require.NotEmpty(t, manifest)
	exportManifestForVisualValidation(t, "infra-k8s-app.manifest.yaml", manifest)

	validator := NewCrossplaneValidator(
		FieldPresenceRule{fields: []string{"apiVersion", "kind", "metadata", "spec"}},
		PlatformTenantLabelRule{},
		SpecRule{},
	)
	require.NoError(t, validator.ValidateYAML([]byte(manifest)))

	require.NoError(t, fileGit.Validate("create_repository", map[string]interface{}{"repo_path": "tenant-integration/checkout-prod"}))
	_, err = fileGit.Execute(context.Background(), "create_repository", map[string]interface{}{"repo_path": "tenant-integration/checkout-prod"})
	require.NoError(t, err)

	_, err = fileGit.Execute(context.Background(), "create_or_update_file", map[string]interface{}{
		"repo_path": "tenant-integration/checkout-prod",
		"path":      "infra/manifest.yaml",
		"content":   manifest,
	})
	require.NoError(t, err)

	res, err := fileGit.Execute(context.Background(), "get_file", map[string]interface{}{
		"repo_path": "tenant-integration/checkout-prod",
		"path":      "infra/manifest.yaml",
	})
	require.NoError(t, err)
	require.Equal(t, manifest, res.Output["content"])
}

func TestInfraIntegration_IdempotencyOnRetry(t *testing.T) {
	harness := NewIntegrationTestHarness(t)
	harness.RegisterModule(infra.NewModule())

	fileGit, err := connectortesting.NewFileGitConnector(harness.TempDir)
	require.NoError(t, err)
	harness.RegisterConnector(fileGit)

	jobID := harness.CreateJob(JobOptions{
		TenantID:       "tenant-retry",
		PathKey:        infra.ModuleKey,
		IdempotencyKey: "infra-it-retry-1",
		Params:         loadParamsFixture(t, "infra-k8s-app"),
	})

	require.NoError(t, harness.RunSchedulerCycle())
	require.NoError(t, harness.WaitForJobState(jobID, "succeeded", 5*time.Second))
	firstManifest := fetchLatestManifestYAML(t, harness, jobID)

	requeueJobForRetry(t, harness, jobID)
	require.NoError(t, harness.RunSchedulerCycle())
	require.NoError(t, harness.WaitForJobState(jobID, "succeeded", 5*time.Second))

	state, attempt, err := harness.JobState(jobID)
	require.NoError(t, err)
	require.Equal(t, "succeeded", state)
	require.Equal(t, 2, attempt)

	secondManifest := fetchLatestManifestYAML(t, harness, jobID)
	require.Equal(t, firstManifest, secondManifest, "manifest must stay stable across retries")

	_, err = fileGit.Execute(context.Background(), "create_repository", map[string]interface{}{"repo_path": "tenant-retry/checkout-prod"})
	require.NoError(t, err)

	_, err = fileGit.Execute(context.Background(), "create_or_update_file", map[string]interface{}{
		"repo_path": "tenant-retry/checkout-prod",
		"path":      "infra/manifest.yaml",
		"content":   firstManifest,
	})
	require.NoError(t, err)

	_, err = fileGit.Execute(context.Background(), "create_or_update_file", map[string]interface{}{
		"repo_path": "tenant-retry/checkout-prod",
		"path":      "infra/manifest.yaml",
		"content":   secondManifest,
	})
	require.NoError(t, err)

	res, err := fileGit.Execute(context.Background(), "get_file", map[string]interface{}{
		"repo_path": "tenant-retry/checkout-prod",
		"path":      "infra/manifest.yaml",
	})
	require.NoError(t, err)
	require.Equal(t, secondManifest, res.Output["content"])
}

func TestInfraIntegration_DataProcTemplate(t *testing.T) {
	harness := NewIntegrationTestHarness(t)
	harness.RegisterModule(infra.NewModule())

	fileGit, err := connectortesting.NewFileGitConnector(harness.TempDir)
	require.NoError(t, err)
	harness.RegisterConnector(fileGit)

	jobID := harness.CreateJob(JobOptions{
		TenantID:       "tenant-data-proc",
		PathKey:        infra.ModuleKey,
		IdempotencyKey: "infra-it-data-proc-1",
		Params:         loadParamsFixture(t, "infra-data-proc"),
	})

	require.NoError(t, harness.RunSchedulerCycle())
	require.NoError(t, harness.WaitForJobState(jobID, "succeeded", 5*time.Second))

	manifest := fetchLatestManifestYAML(t, harness, jobID)
	exportManifestForVisualValidation(t, "infra-data-proc.manifest.yaml", manifest)

	validator := NewCrossplaneValidator(
		FieldPresenceRule{fields: []string{"apiVersion", "kind", "metadata", "spec"}},
		PlatformTenantLabelRule{},
		SpecRule{},
	)
	require.NoError(t, validator.ValidateYAML([]byte(manifest)))
	require.Contains(t, manifest, "composition-data-proc-v1")
	require.Contains(t, manifest, "messaging")
	require.Contains(t, manifest, "objectStorage")

	_, err = fileGit.Execute(context.Background(), "create_repository", map[string]interface{}{"repo_path": "tenant-data-proc/pipeline"})
	require.NoError(t, err)
	_, err = fileGit.Execute(context.Background(), "create_or_update_file", map[string]interface{}{
		"repo_path": "tenant-data-proc/pipeline",
		"path":      "infra/manifest.yaml",
		"content":   manifest,
	})
	require.NoError(t, err)
}

func TestInfraIntegration_VMAppTemplate(t *testing.T) {
	harness := NewIntegrationTestHarness(t)
	harness.RegisterModule(infra.NewModule())

	fileGit, err := connectortesting.NewFileGitConnector(harness.TempDir)
	require.NoError(t, err)
	harness.RegisterConnector(fileGit)

	jobID := harness.CreateJob(JobOptions{
		TenantID:       "tenant-vm-app",
		PathKey:        infra.ModuleKey,
		IdempotencyKey: "infra-it-vm-app-1",
		Params:         loadParamsFixture(t, "infra-vm-app"),
	})

	require.NoError(t, harness.RunSchedulerCycle())
	require.NoError(t, harness.WaitForJobState(jobID, "succeeded", 5*time.Second))

	manifest := fetchLatestManifestYAML(t, harness, jobID)
	exportManifestForVisualValidation(t, "infra-vm-app.manifest.yaml", manifest)

	validator := NewCrossplaneValidator(
		FieldPresenceRule{fields: []string{"apiVersion", "kind", "metadata", "spec"}},
		PlatformTenantLabelRule{},
		SpecRule{},
	)
	require.NoError(t, validator.ValidateYAML([]byte(manifest)))
	require.Contains(t, manifest, "composition-vm-app-v1")
	require.Contains(t, manifest, "vm:")
	require.NotContains(t, manifest, "kubernetes:")

	_, err = fileGit.Execute(context.Background(), "create_repository", map[string]interface{}{"repo_path": "tenant-vm-app/workload"})
	require.NoError(t, err)
	_, err = fileGit.Execute(context.Background(), "create_or_update_file", map[string]interface{}{
		"repo_path": "tenant-vm-app/workload",
		"path":      "infra/manifest.yaml",
		"content":   manifest,
	})
	require.NoError(t, err)
}

func loadParamsFixture(t *testing.T, name string) map[string]any {
	t.Helper()
	path := filepath.Join("testdata", "payloads", fmt.Sprintf("%s.yaml", name))
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	params := map[string]any{}
	require.NoError(t, yaml.Unmarshal(data, &params))
	return params
}

func exportManifestForVisualValidation(t *testing.T, fileName, manifest string) {
	t.Helper()
	outDir := filepath.Join("testdata", "output")
	require.NoError(t, os.MkdirAll(outDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(outDir, fileName), []byte(manifest), 0o644))
}

func fetchLatestManifestYAML(t *testing.T, harness *IntegrationTestHarness, jobID string) string {
	t.Helper()
	conn, err := harness.Pool.Acquire(context.Background())
	require.NoError(t, err)
	defer conn.Release()

	var manifest string
	err = conn.QueryRow(context.Background(), `
		SELECT output_json->>'manifest_yaml'
		FROM steps
		WHERE job_id = $1::uuid
		  AND step_key = 'infra.render'
		  AND status = 'ok'
		ORDER BY attempt DESC, id DESC
		LIMIT 1
	`, jobID).Scan(&manifest)
	require.NoError(t, err)
	return manifest
}

func requeueJobForRetry(t *testing.T, harness *IntegrationTestHarness, jobID string) {
	t.Helper()
	conn, err := harness.Pool.Acquire(context.Background())
	require.NoError(t, err)
	defer conn.Release()

	_, err = conn.Exec(context.Background(), `
		UPDATE jobs
		SET state = 'queued',
			owner_id = NULL,
			lease_expires_at = NULL,
			next_run_at = now() - interval '1 second',
			updated_at = now()
		WHERE id = $1::uuid
	`, jobID)
	require.NoError(t, err)
}

// PlatformTenantLabelRule validates labels used by infra XR manifests.
type PlatformTenantLabelRule struct{}

func (PlatformTenantLabelRule) Name() string { return "PlatformTenantLabel" }

func (PlatformTenantLabelRule) Validate(obj map[string]any) error {
	metadata, ok := obj["metadata"].(map[string]any)
	if !ok {
		return fmt.Errorf("metadata must be an object")
	}
	labels, ok := metadata["labels"].(map[string]any)
	if !ok {
		return fmt.Errorf("metadata.labels must be defined")
	}
	if _, ok := labels["platform.io/tenant"]; !ok {
		return fmt.Errorf("metadata.labels must include platform.io/tenant")
	}
	return nil
}
