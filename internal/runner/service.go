// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package runner

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gatblau/release-engine/internal/connector"
	"github.com/gatblau/release-engine/internal/db"
	"github.com/gatblau/release-engine/internal/registry"
	"github.com/gorhill/cronexpr"
	"github.com/jackc/pgx/v4"
)

// RunnerService executes claimed jobs, drives step lifecycle, and finalises jobs.
type RunnerService interface {
	RunJob(ctx context.Context, jobID string, runID string) error
}

type runnerService struct {
	pool              db.Pool
	connectorRegistry connector.ConnectorRegistry
	stepAPI           StepAPI
	registry          registry.ModuleRegistry
	stepAPIFac        func(jobID, runID string, attempt int) StepAPI
}

func NewRunnerService(pool db.Pool, familyRegistry connector.FamilyRegistry, stepAPI StepAPI, registry registry.ModuleRegistry) RunnerService {
	return &runnerService{
		pool:              pool,
		connectorRegistry: nil, // Not used anymore
		stepAPI:           stepAPI,
		registry:          registry,
		stepAPIFac: func(jobID, runID string, attempt int) StepAPI {
			// Return the StepAPI adapter directly - no need for DualStepAPI wrapper
			return NewStepAPIAdapter(pool, familyRegistry, jobID, runID, attempt)
		},
	}
}

func (s *runnerService) RunJob(ctx context.Context, jobID string, runID string) error {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("failed to acquire db connection: %w", err)
	}
	defer conn.Release()

	// 1. Fetch job definition
	var pathKey, version string
	var paramsRaw []byte
	var schedule sql.NullString
	var attempt int
	err = conn.QueryRow(ctx, "SELECT path_key, COALESCE(params_json->>'version', 'latest'), params_json, schedule, attempt FROM jobs WHERE id = $1::uuid AND run_id = $2::uuid AND state = 'running'", jobID, runID).Scan(&pathKey, &version, &paramsRaw, &schedule, &attempt)
	if err != nil {
		return fmt.Errorf("failed to fetch job: %w", err)
	}

	params := map[string]any{}
	if len(paramsRaw) > 0 {
		if err := json.Unmarshal(paramsRaw, &params); err != nil {
			return fmt.Errorf("failed to decode job params: %w", err)
		}
	}

	// 2. Resolve module from registry
	if version == "" {
		version = "latest"
	}
	module, ok := s.registry.Lookup(pathKey, version)
	if !ok {
		return fmt.Errorf("module %s:%s not found", pathKey, version)
	}

	stepAPI := s.stepAPI
	if stepAPI == nil && s.stepAPIFac != nil {
		stepAPI = s.stepAPIFac(jobID, runID, attempt)
	}

	// If stepAPI supports job context setting, set it
	if setter, ok := stepAPI.(interface {
		SetJobContext(jobID, runID string, attempt int)
	}); ok {
		setter.SetJobContext(jobID, runID, attempt)
	}

	// 3. Execute module workflow (modules orchestrate via StepAPI).
	if err := module.Execute(ctx, stepAPI, params); err != nil {
		_ = s.finaliseFailure(ctx, jobID, runID, "RUNNER_EXEC_FAILED", err.Error())
		return fmt.Errorf("module execution failed for %s:%s: %w", pathKey, version, err)
	}

	if err := s.finaliseSuccess(ctx, jobID, runID); err != nil {
		return err
	}

	fmt.Printf("Job %s run %s processed successfully\n", jobID, runID)
	return nil
}

func (s *runnerService) finaliseSuccess(ctx context.Context, jobID, runID string) error {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("failed to acquire db connection: %w", err)
	}
	defer conn.Release()

	var schedule sql.NullString
	var dbNow time.Time
	err = conn.QueryRow(ctx,
		"SELECT schedule, now() FROM jobs WHERE id = $1::uuid AND run_id = $2::uuid AND state = 'running'",
		jobID,
		runID,
	).Scan(&schedule, &dbNow)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("fenced conflict while finalising success")
		}
		return fmt.Errorf("failed to fetch schedule for success finalization: %w", err)
	}

	nextRunAt, shouldRequeue, err := computeNextRunAt(schedule, dbNow)
	if err != nil {
		return err
	}

	var tag interface{ RowsAffected() int64 }
	if shouldRequeue {
		tag, err = conn.Exec(ctx, `
			UPDATE jobs
			SET state = 'queued',
				owner_id = NULL,
				lease_expires_at = NULL,
				next_run_at = $3,
				last_error_code = NULL,
				last_error_message = NULL,
				finished_at = NULL,
				updated_at = now()
			WHERE id = $1::uuid AND run_id = $2::uuid AND state = 'running'`, jobID, runID, nextRunAt)
	} else {
		tag, err = conn.Exec(ctx, `
			UPDATE jobs
			SET state = 'succeeded',
				owner_id = NULL,
				lease_expires_at = NULL,
				next_run_at = NULL,
				last_error_code = NULL,
				last_error_message = NULL,
				finished_at = now(),
				updated_at = now()
			WHERE id = $1::uuid AND run_id = $2::uuid AND state = 'running'`, jobID, runID)
	}
	if err != nil {
		return fmt.Errorf("failed to finalise success: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("fenced conflict while finalising success")
	}

	_, _ = conn.Exec(ctx, `
		INSERT INTO jobs_read AS r (id, tenant_id, path_key, state, attempt, max_attempts, schedule, owner_id, run_id,
			lease_expires_at, next_run_at, accepted_at, last_error_code, last_error_message, started_at, finished_at, created_at, updated_at)
		SELECT id, tenant_id, path_key, state, attempt, max_attempts, schedule, owner_id, run_id,
			lease_expires_at, next_run_at, accepted_at, last_error_code, last_error_message, started_at, finished_at, created_at, updated_at
		FROM jobs WHERE id = $1::uuid
		ON CONFLICT (id) DO UPDATE
		SET state = EXCLUDED.state,
			attempt = EXCLUDED.attempt,
			max_attempts = EXCLUDED.max_attempts,
			schedule = EXCLUDED.schedule,
			owner_id = EXCLUDED.owner_id,
			run_id = EXCLUDED.run_id,
			lease_expires_at = EXCLUDED.lease_expires_at,
			next_run_at = EXCLUDED.next_run_at,
			accepted_at = EXCLUDED.accepted_at,
			last_error_code = EXCLUDED.last_error_code,
			last_error_message = EXCLUDED.last_error_message,
			started_at = EXCLUDED.started_at,
			finished_at = EXCLUDED.finished_at,
			created_at = EXCLUDED.created_at,
			updated_at = EXCLUDED.updated_at`, jobID)

	_, _ = conn.Exec(ctx, `
		INSERT INTO outbox (tenant_id, job_id, kind, payload_json, next_run_at)
		SELECT tenant_id, id, 'event', jsonb_build_object('type','job.succeeded','job_id',id), now()
		FROM jobs WHERE id = $1::uuid`, jobID)

	return nil
}

func (s *runnerService) finaliseFailure(ctx context.Context, jobID, runID, code, msg string) error {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("failed to acquire db connection: %w", err)
	}
	defer conn.Release()

	tag, err := conn.Exec(ctx, `
		UPDATE jobs
		SET state = CASE WHEN attempt >= max_attempts THEN 'jobs_exhausted' ELSE 'queued' END,
			owner_id = NULL,
			lease_expires_at = NULL,
			next_run_at = CASE
				WHEN attempt >= max_attempts THEN NULL
				ELSE now() + backoff_interval(attempt, backoff_policy)
			END,
			last_error_code = $3,
			last_error_message = $4,
			updated_at = now(),
			finished_at = CASE WHEN attempt >= max_attempts THEN now() ELSE NULL END
		WHERE id = $1::uuid AND run_id = $2::uuid AND state = 'running'`, jobID, runID, code, msg)
	if err != nil {
		return fmt.Errorf("failed to finalise failure: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("fenced conflict while finalising failure")
	}

	_, _ = conn.Exec(ctx, `
		INSERT INTO jobs_read AS r (id, tenant_id, path_key, state, attempt, max_attempts, schedule, owner_id, run_id,
			lease_expires_at, next_run_at, accepted_at, last_error_code, last_error_message, started_at, finished_at, created_at, updated_at)
		SELECT id, tenant_id, path_key, state, attempt, max_attempts, schedule, owner_id, run_id,
			lease_expires_at, next_run_at, accepted_at, last_error_code, last_error_message, started_at, finished_at, created_at, updated_at
		FROM jobs WHERE id = $1::uuid
		ON CONFLICT (id) DO UPDATE
		SET state = EXCLUDED.state,
			attempt = EXCLUDED.attempt,
			max_attempts = EXCLUDED.max_attempts,
			schedule = EXCLUDED.schedule,
			owner_id = EXCLUDED.owner_id,
			run_id = EXCLUDED.run_id,
			lease_expires_at = EXCLUDED.lease_expires_at,
			next_run_at = EXCLUDED.next_run_at,
			accepted_at = EXCLUDED.accepted_at,
			last_error_code = EXCLUDED.last_error_code,
			last_error_message = EXCLUDED.last_error_message,
			started_at = EXCLUDED.started_at,
			finished_at = EXCLUDED.finished_at,
			created_at = EXCLUDED.created_at,
			updated_at = EXCLUDED.updated_at`, jobID)

	return nil
}

func computeNextRunAt(schedule sql.NullString, base time.Time) (time.Time, bool, error) {
	if !schedule.Valid || strings.TrimSpace(schedule.String) == "" {
		return time.Time{}, false, nil
	}

	expr, err := cronexpr.Parse(schedule.String)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("failed to parse job schedule during success finalization: %w", err)
	}

	next := expr.Next(base)
	if next.IsZero() {
		return time.Time{}, false, fmt.Errorf("failed to compute next run for schedule %q", schedule.String)
	}

	return next, true, nil
}
