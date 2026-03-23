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
	"github.com/gatblau/release-engine/internal/stepapi"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/minio"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/zap"
)

// IntegrationTestHarness wires together containers, registries, and services so integration
// tests can focus on observability and behaviour verification.
type IntegrationTestHarness struct {
	*testing.T
	ctx            context.Context
	cancel         context.CancelFunc
	Pool           db.Pool
	ConnectorReg   connector.ConnectorRegistry
	FamilyReg      connector.FamilyRegistry
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

	// Create family registry with default families (skip validation for tests)
	familyReg := connector.NewFamilyRegistry(h.ConnectorReg)
	for _, family := range connector.DefaultFamilies() {
		if err := familyReg.RegisterFamily(family); err != nil {
			panic(fmt.Sprintf("failed to register family %s: %v", family.Name, err))
		}
	}
	h.FamilyReg = familyReg

	// Create a simple test StepAPI that auto-approves any approval requests
	// We'll create a wrapper runner that injects job context into the StepAPI
	// Since runner.runnerService is not exported, we'll handle this differently
	// We'll create a custom test StepAPI that stores steps in memory for tests
	testStepAPI := &testStepAPI{
		pool:           pool,
		familyRegistry: familyReg,
		// In-memory storage for test steps
		steps: make(map[string]*testStep),
	}

	roller := runner.NewRunnerService(pool, familyReg, testStepAPI, h.ModuleReg)
	leaseMgr := scheduler.NewLeaseManager(pool)
	svc := scheduler.NewSchedulerService(pool, h.ModuleReg, leaseMgr, 25*time.Millisecond, roller)
	h.Runner = roller
	h.Scheduler = svc
	minioContainer := h.setupMinio(ctx)
	h.minioContainer = minioContainer
	return h
}

// testStep is an in-memory representation of a step for testing
type testStep struct {
	stepKey string
	output  map[string]any
	errCode string
	errMsg  string
	status  string // "ok", "error"
}

// testStepAPI is a test implementation of stepapi.StepAPI that wraps the real StepAPIAdapter
// for persistence, while providing test-specific behavior (auto-approval).
type testStepAPI struct {
	pool           db.Pool
	familyRegistry connector.FamilyRegistry
	real           stepapi.StepAPI // Real adapter for persistence
	jobID          string
	runID          string
	attempt        int
	currentStep    string
	contextStore   map[string]any
	steps          map[string]*testStep // stepKey -> testStep
}

// SetJobContext sets the job context for this StepAPI instance and creates the real adapter
func (t *testStepAPI) SetJobContext(jobID, runID string, attempt int) {
	t.jobID = jobID
	t.runID = runID
	t.attempt = attempt
	// Create real adapter with the correct job context
	t.real = runner.NewStepAPIAdapter(t.pool, t.familyRegistry, jobID, runID, attempt)
}

func (t *testStepAPI) BeginStep(stepKey string) error {
	fmt.Printf("[TEST StepAPI] BeginStep: %s (job: %s, run: %s, attempt: %d)\n", stepKey, t.jobID, t.runID, t.attempt)
	t.currentStep = stepKey
	// Delegate to real adapter if it exists
	if t.real != nil {
		return t.real.BeginStep(stepKey)
	}
	return nil
}

func (t *testStepAPI) EndStepOK(stepKey string, output map[string]any) error {
	fmt.Printf("[TEST StepAPI] EndStepOK: %s (job: %s)\n", stepKey, t.jobID)
	// Store step in memory for test queries
	t.steps[stepKey] = &testStep{
		stepKey: stepKey,
		output:  output,
		status:  "ok",
	}
	// Delegate to real adapter for persistence
	if t.real != nil {
		return t.real.EndStepOK(stepKey, output)
	}
	// Fallback: create adapter with current job context (should not happen if SetJobContext was called)
	if t.jobID == "" || t.runID == "" || t.attempt == 0 {
		return fmt.Errorf("testStepAPI not initialized with job context")
	}
	adapter := runner.NewStepAPIAdapter(t.pool, t.familyRegistry, t.jobID, t.runID, t.attempt)
	return adapter.EndStepOK(stepKey, output)
}

func (t *testStepAPI) EndStepErr(stepKey, code, msg string) error {
	fmt.Printf("[TEST StepAPI] EndStepErr: %s, code: %s, msg: %s (job: %s)\n", stepKey, code, msg, t.jobID)
	// Store step in memory for test queries
	t.steps[stepKey] = &testStep{
		stepKey: stepKey,
		errCode: code,
		errMsg:  msg,
		status:  "error",
	}
	// Delegate to real adapter for persistence
	if t.real != nil {
		return t.real.EndStepErr(stepKey, code, msg)
	}
	// Fallback: create adapter with current job context (should not happen if SetJobContext was called)
	if t.jobID == "" || t.runID == "" || t.attempt == 0 {
		return fmt.Errorf("testStepAPI not initialized with job context")
	}
	adapter := runner.NewStepAPIAdapter(t.pool, t.familyRegistry, t.jobID, t.runID, t.attempt)
	return adapter.EndStepErr(stepKey, code, msg)
}

func (t *testStepAPI) CallConnector(ctx context.Context, req stepapi.ConnectorRequest) (*stepapi.ConnectorResult, error) {
	fmt.Printf("[TEST StepAPI] CallConnector: %s.%s (job: %s)\n", req.Connector, req.Operation, t.jobID)
	// Delegate to real adapter for connector calls (which will use fake connectors from family registry)
	if t.real != nil {
		return t.real.CallConnector(ctx, req)
	}
	// Fallback: create adapter with current job context (should not happen if SetJobContext was called)
	if t.jobID == "" || t.runID == "" || t.attempt == 0 {
		return &stepapi.ConnectorResult{
			Status: "error",
			Error: &struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			}{
				Code:    "TEST_STEPAPI_NOT_INITIALIZED",
				Message: "testStepAPI not initialized with job context",
			},
		}, nil
	}
	adapter := runner.NewStepAPIAdapter(t.pool, t.familyRegistry, t.jobID, t.runID, t.attempt)
	return adapter.CallConnector(ctx, req)
}

func (t *testStepAPI) SetContext(key string, value any) error {
	fmt.Printf("[TEST StepAPI] SetContext: %s = %v (job: %s)\n", key, value, t.jobID)
	// Delegate to real adapter if it exists
	if t.real != nil {
		return t.real.SetContext(key, value)
	}
	if t.contextStore == nil {
		t.contextStore = make(map[string]any)
	}
	t.contextStore[key] = value
	return nil
}

func (t *testStepAPI) GetContext(key string) (any, bool) {
	// Delegate to real adapter if it exists
	if t.real != nil {
		return t.real.GetContext(key)
	}
	if t.contextStore == nil {
		return nil, false
	}
	v, ok := t.contextStore[key]
	return v, ok
}

func (t *testStepAPI) IsCancelled() bool {
	// Delegate to real adapter if it exists
	if t.real != nil {
		return t.real.IsCancelled()
	}
	return false
}

func (t *testStepAPI) Logger() *zap.Logger {
	// Delegate to real adapter if it exists
	if t.real != nil {
		return t.real.Logger()
	}
	// Create a real logger for debugging test failures
	logger, _ := zap.NewDevelopment()
	return logger
}

func (t *testStepAPI) WaitForApproval(ctx context.Context, req stepapi.ApprovalRequest) (stepapi.ApprovalOutcome, error) {
	fmt.Printf("[TEST StepAPI] WaitForApproval: %s (job: %s)\n", req.Summary, t.jobID)
	// Immediately return an approved decision without polling the database
	return stepapi.ApprovalOutcome{
		Decision:      "approved",
		Approver:      "test-auto-approver",
		Justification: "Auto-approved in test",
		DecidedAt:     time.Now(),
	}, nil
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

// BindFamily binds a connector to a family in the family registry.
func (h *IntegrationTestHarness) BindFamily(familyName string, connectorKey string) {
	require.NoError(h, h.FamilyReg.BindImplementation(familyName, connectorKey))
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
		// Auto-approve any waiting approval steps
		h.AutoApproveWaitingSteps(jobID)
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("job %s did not reach state %s in time", jobID, expected)
}

// AutoApproveWaitingSteps automatically approves any steps in waiting_approval status.
func (h *IntegrationTestHarness) AutoApproveWaitingSteps(jobID string) {
	// Try up to 3 times to handle "conn busy" errors
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			// Small delay between retries
			time.Sleep(10 * time.Millisecond)
		}

		conn, err := h.Pool.Acquire(h.ctx)
		if err != nil {
			h.Logf("AutoApproveWaitingSteps: failed to acquire connection (attempt %d): %v", attempt+1, err)
			continue
		}

		// Find steps waiting for approval
		rows, err := conn.Query(h.ctx, `
			SELECT id, run_id, attempt, step_key
			FROM steps
			WHERE job_id = $1::uuid
			  AND status = 'waiting_approval'
			  AND NOT EXISTS (
				SELECT 1 FROM approval_decisions ad WHERE ad.step_id = steps.id
			  )
		`, jobID)
		if err != nil {
			h.Logf("AutoApproveWaitingSteps: failed to query steps (attempt %d): %v", attempt+1, err)
			conn.Release()
			continue
		}

		approvedCount := 0
		hasRows := false
		for rows.Next() {
			hasRows = true
			var stepID int64
			var runID string
			var attempt int
			var stepKey string
			if err := rows.Scan(&stepID, &runID, &attempt, &stepKey); err != nil {
				h.Logf("AutoApproveWaitingSteps: failed to scan step (attempt %d): %v", attempt+1, err)
				continue
			}

			h.Logf("AutoApproveWaitingSteps: approving step %d (key: %s) for job %s", stepID, stepKey, jobID)

			// Insert approval decision
			_, err = conn.Exec(h.ctx, `
				INSERT INTO approval_decisions (
					job_id, step_id, run_id, decision, approver, justification,
					policy_snapshot, idempotency_key, created_at
				) VALUES (
					$1::uuid, $2, $3::uuid, 'approved', 'test-auto-approver',
					'Auto-approved in integration test',
					'{}'::jsonb, $4, now()
				)
			`, jobID, stepID, runID, fmt.Sprintf("auto-approve-%s-%d", jobID, stepID))
			if err != nil {
				// Check if it's a "conn busy" error
				if strings.Contains(err.Error(), "conn busy") {
					h.Logf("AutoApproveWaitingSteps: conn busy for step %d (attempt %d), will retry", stepID, attempt+1)
					rows.Close()
					conn.Release()
					break // Break out of rows loop, will retry outer loop
				}
				h.Logf("AutoApproveWaitingSteps: failed to insert approval decision for step %d (attempt %d): %v", stepID, attempt+1, err)
			} else {
				approvedCount++
			}
		}
		rows.Close()
		conn.Release()

		// If we processed rows without "conn busy" errors, we're done
		if hasRows && approvedCount > 0 {
			h.Logf("AutoApproveWaitingSteps: approved %d steps for job %s", approvedCount, jobID)
			return
		}

		// If we had rows but got "conn busy", continue to next attempt
		if hasRows {
			continue
		}

		// No rows found, nothing to approve
		return
	}
}
