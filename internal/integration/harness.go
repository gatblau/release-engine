//go:build integration

// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gatblau/release-engine/internal/connector"
	"github.com/gatblau/release-engine/internal/db"
	"github.com/gatblau/release-engine/internal/registry"
	"github.com/gatblau/release-engine/internal/runner"
	"github.com/gatblau/release-engine/internal/scheduler"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/minio"
	"github.com/testcontainers/testcontainers-go/wait"
)

// IntegrationTestHarness wires together containers, registries, and services so integration
// tests can focus on observability and behaviour verification.
type IntegrationTestHarness struct {
	*testing.T
	ctx            context.Context
	cancel         context.CancelFunc
	Pool           db.Pool
	ConnectorReg   connector.ConnectorRegistry
	ModuleReg      registry.ModuleRegistry
	Runner         runner.RunnerService
	Scheduler      scheduler.SchedulerService
	TempDir        string
	minioContainer testcontainers.Container
}

// JobOptions describe the properties of a job to insert via the harness.
type JobOptions struct {
	TenantID       string
	PathKey        string
	IdempotencyKey string
	Params         map[string]any
	Schedule       string
	NextRunAt      time.Time
	MaxAttempts    int
	State          string
}

// NewIntegrationTestHarness sets up test containers, registries, and services, and registers
// a cleanup handler with the provided testing.TB.
func NewIntegrationTestHarness(t *testing.T) *IntegrationTestHarness {
	ctx, cancel := context.WithCancel(context.Background())
	h := &IntegrationTestHarness{
		T:      t,
		ctx:    ctx,
		cancel: cancel,
	}
	t.Cleanup(h.Cleanup)
	connStr := db.SetupTestPostgres(ctx, t)
	pool, err := db.NewPool(connStr)
	require.NoError(t, err)
	h.Pool = pool
	h.TempDir = t.TempDir()
	logConn := func() {
		conn, err := pool.Acquire(ctx)
		require.NoError(t, err)
		_, err = conn.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS pgcrypto`)
		require.NoError(t, err)
		for _, schema := range db.SchemaAll {
			_, err := conn.Exec(ctx, schema)
			require.NoError(t, err)
		}
		conn.Release()
	}
	logConn()
	h.ConnectorReg = connector.NewConnectorRegistry()
	h.ModuleReg = registry.NewModuleRegistry()
	roller := runner.NewRunnerService(pool, nil, h.ModuleReg)
	leaseMgr := scheduler.NewLeaseManager(pool)
	svc := scheduler.NewSchedulerService(pool, h.ModuleReg, leaseMgr, 25*time.Millisecond, roller)
	h.Runner = roller
	h.Scheduler = svc
	minioContainer := h.setupMinio(ctx)
	h.minioContainer = minioContainer
	return h
}

// setupMinio creates MinIO for vault/backing-store emulation and wires AWS-style env vars.
func (h *IntegrationTestHarness) setupMinio(ctx context.Context) testcontainers.Container {
	container, err := minio.RunContainer(ctx,
		testcontainers.WithImage("minio/minio:latest"),
		testcontainers.WithEnv(map[string]string{
			"MINIO_ROOT_USER":     "minioadmin",
			"MINIO_ROOT_PASSWORD": "minioadmin",
		}),
		testcontainers.WithCmd("server", "/data"),
		testcontainers.WithWaitStrategy(wait.ForListeningPort("9000")),
	)
	require.NoError(h, err)
	endpoint, err := container.Endpoint(ctx, "http")
	require.NoError(h, err)
	h.Setenv("AWS_ACCESS_KEY_ID", "minioadmin")
	h.Setenv("AWS_SECRET_ACCESS_KEY", "minioadmin")
	h.Setenv("AWS_REGION", "us-east-1")
	h.Setenv("AWS_ENDPOINT", strings.TrimPrefix(endpoint, "http://"))
	h.Setenv("VOLTA_STORAGE", "s3")
	h.Setenv("VOLTA_MASTER_PASSPHRASE", "test-passphrase-123")
	h.Setenv("VOLTA_S3_BUCKET", "test-volta-bucket")
	h.Setenv("VOLTA_FILE_PATH", filepath.Join(h.TempDir, "volta"))
	return container
}

// Cleanup shuts down services and containers.
func (h *IntegrationTestHarness) Cleanup() {
	if h.cancel != nil {
		h.cancel()
	}
	if h.Scheduler != nil {
		_ = h.Scheduler.Stop(context.Background())
	}
	if h.minioContainer != nil {
		_ = h.minioContainer.Terminate(context.Background())
	}
	if h.ConnectorReg != nil {
		_ = h.ConnectorReg.Close()
	}
	if h.Pool != nil {
		h.Pool.Close()
	}
}

// RegisterConnector exposes the connector registry.
func (h *IntegrationTestHarness) RegisterConnector(conn connector.Connector) {
	require.NoError(h, h.ConnectorReg.Register(conn))
}

// RegisterModule exposes the module registry.
func (h *IntegrationTestHarness) RegisterModule(mod registry.Module) {
	require.NoError(h, h.ModuleReg.Register(mod))
}

// CreateJob inserts a job with defaults and returns the generated ID.
func (h *IntegrationTestHarness) CreateJob(opts JobOptions) string {
	ctx := h.ctx
	conn, err := h.Pool.Acquire(ctx)
	require.NoError(h, err)
	defer conn.Release()
	if opts.TenantID == "" {
		opts.TenantID = "test-tenant"
	}
	require.NotEmpty(h, opts.PathKey, "path key is required")
	if opts.IdempotencyKey == "" {
		opts.IdempotencyKey = fmt.Sprintf("idem-%s-%d", opts.PathKey, time.Now().UnixNano())
	}
	params := map[string]any{}
	if opts.Params != nil {
		params = opts.Params
	}
	paramsJSON, err := json.Marshal(params)
	require.NoError(h, err)
	jobID := uuid.NewString()
	state := opts.State
	if state == "" {
		state = "queued"
	}
	nextRun := interface{}(nil)
	if opts.NextRunAt.IsZero() {
		nextRun = time.Now().UTC()
	} else {
		nextRun = opts.NextRunAt
	}
	maxAttempts := opts.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	_, err = conn.Exec(ctx, `INSERT INTO jobs (id, tenant_id, path_key, idempotency_key, params_json, schedule, state, attempt, max_attempts, next_run_at, created_at, updated_at)
		VALUES ($1::uuid, $2, $3, $4, $5::jsonb, $6, $7, 0, $8, $9, now(), now())`,
		jobID, opts.TenantID, opts.PathKey, opts.IdempotencyKey, string(paramsJSON), opts.Schedule, state, maxAttempts, nextRun)
	require.NoError(h, err)
	return jobID
}

// RunSchedulerCycle runs the scheduler for a short burst so queued jobs can be claimed.
func (h *IntegrationTestHarness) RunSchedulerCycle() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- h.Scheduler.Start(ctx)
	}()
	<-ctx.Done()
	return <-errCh
}

// JobState returns the current state and attempt count for a job.
func (h *IntegrationTestHarness) JobState(jobID string) (string, int, error) {
	conn, err := h.Pool.Acquire(h.ctx)
	if err != nil {
		return "", 0, err
	}
	defer conn.Release()
	var state string
	var attempt int
	err = conn.QueryRow(h.ctx, `SELECT state, attempt FROM jobs WHERE id = $1::uuid`, jobID).Scan(&state, &attempt)
	return state, attempt, err
}

// WaitForJobState polls until the job reaches the expected state or the timeout elapses.
func (h *IntegrationTestHarness) WaitForJobState(jobID, expected string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		state, _, err := h.JobState(jobID)
		if err != nil {
			return err
		}
		if state == expected {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("job %s did not reach state %s in time", jobID, expected)
}
