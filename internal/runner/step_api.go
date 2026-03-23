// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gatblau/release-engine/internal/connector"
	"github.com/gatblau/release-engine/internal/db"
	"github.com/gatblau/release-engine/internal/logger"
	"github.com/gatblau/release-engine/internal/stepapi"
	"github.com/jackc/pgx/v4"
	"go.uber.org/zap"
)

// StepAPI defines the module-facing execution interface.
// This is an alias to the shared stepapi.StepAPI interface.
type StepAPI = stepapi.StepAPI

type stepAPIAdapter struct {
	pool           db.Pool
	familyRegistry connector.FamilyRegistry
	jobID          string
	runID          string
	attempt        int
	currentStep    string
	contextStore   map[string]any
	pollInterval   time.Duration
	logger         *zap.Logger
}

// NewStepAPIAdapter creates the module-facing runtime API for a specific job run.
func NewStepAPIAdapter(pool db.Pool, familyRegistry connector.FamilyRegistry, jobID, runID string, attempt int) StepAPI {
	// Create a logger for the module execution
	loggerFactory, err := logger.NewFactory("info", "console")
	var log *zap.Logger
	if err != nil {
		// Fallback to no-op logger if factory creation fails
		log = zap.NewNop()
	} else {
		log = loggerFactory.New(fmt.Sprintf("module.job.%s", jobID))
	}

	return &stepAPIAdapter{
		pool:           pool,
		familyRegistry: familyRegistry,
		jobID:          jobID,
		runID:          runID,
		attempt:        attempt,
		contextStore:   make(map[string]any),
		pollInterval:   500 * time.Millisecond,
		logger:         log,
	}
}

func (a *stepAPIAdapter) BeginStep(stepKey string) error {
	a.currentStep = stepKey
	return nil
}

func (a *stepAPIAdapter) EndStepOK(stepKey string, output map[string]any) error {
	conn, err := a.pool.Acquire(context.Background())
	if err != nil {
		return fmt.Errorf("failed to acquire db connection: %w", err)
	}
	defer conn.Release()

	var out any
	if output != nil {
		out = output
	}

	_, err = conn.Exec(
		context.Background(),
		`INSERT INTO steps (job_id, run_id, attempt, step_key, status, output_json, started_at, finished_at)
		 VALUES ($1,$2,$3,$4,'ok',$5,now(),now())
		 ON CONFLICT (job_id, attempt, step_key)
		 DO UPDATE SET status='ok', output_json=$5, finished_at=now()`,
		a.jobID,
		a.runID,
		a.attempt,
		stepKey,
		out,
	)
	if err != nil {
		return fmt.Errorf("failed to persist step success: %w", err)
	}
	return nil
}

func (a *stepAPIAdapter) EndStepErr(stepKey, code, msg string) error {
	conn, err := a.pool.Acquire(context.Background())
	if err != nil {
		return fmt.Errorf("failed to acquire db connection: %w", err)
	}
	defer conn.Release()

	_, err = conn.Exec(
		context.Background(),
		`INSERT INTO steps (job_id, run_id, attempt, step_key, status, error_code, error_message, started_at, finished_at)
		 VALUES ($1,$2,$3,$4,'error',$5,$6,now(),now())
		 ON CONFLICT (job_id, attempt, step_key)
		 DO UPDATE SET status='error', error_code=$5, error_message=$6, finished_at=now()`,
		a.jobID,
		a.runID,
		a.attempt,
		stepKey,
		code,
		msg,
	)
	if err != nil {
		return fmt.Errorf("failed to persist step error: %w", err)
	}
	return nil
}

func (a *stepAPIAdapter) CallConnector(ctx context.Context, req stepapi.ConnectorRequest) (*stepapi.ConnectorResult, error) {
	// Look up connector via family registry
	conn, err := a.familyRegistry.Resolve(req.Connector)
	if err != nil {
		return &stepapi.ConnectorResult{
			Status: "error",
			Error: &struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			}{
				Code:    "CONNECTOR_NOT_FOUND",
				Message: fmt.Sprintf("connector family not resolved: %s", req.Connector),
			},
		}, nil
	}

	// Validate input
	if err := conn.Validate(req.Operation, req.Input); err != nil {
		return &stepapi.ConnectorResult{
			Status: "error",
			Error: &struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			}{
				Code:    "VALIDATION_FAILED",
				Message: fmt.Sprintf("validation failed: %v", err),
			},
		}, nil
	}

	// Execute connector
	result, err := conn.Execute(ctx, req.Operation, req.Input)
	if err != nil {
		return &stepapi.ConnectorResult{
			Status: "error",
			Error: &struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			}{
				Code:    "EXECUTION_FAILED",
				Message: fmt.Sprintf("execution failed: %v", err),
			},
		}, nil
	}

	// Convert connector.ConnectorResult to stepapi.ConnectorResult
	runnerResult := &stepapi.ConnectorResult{
		Status: result.Status,
		Output: result.Output,
	}
	if result.Error != nil {
		runnerResult.Error = &struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}{
			Code:    result.Error.Code,
			Message: result.Error.Message,
		}
	}
	return runnerResult, nil
}

func (a *stepAPIAdapter) SetContext(key string, value any) error {
	a.contextStore[key] = value
	return nil
}

func (a *stepAPIAdapter) GetContext(key string) (any, bool) {
	v, ok := a.contextStore[key]
	return v, ok
}

func (a *stepAPIAdapter) IsCancelled() bool {
	return false
}

func (a *stepAPIAdapter) Logger() *zap.Logger {
	return a.logger
}

func (a *stepAPIAdapter) WaitForApproval(ctx context.Context, req stepapi.ApprovalRequest) (stepapi.ApprovalOutcome, error) {
	if a.currentStep == "" {
		return stepapi.ApprovalOutcome{}, fmt.Errorf("BeginStep must be called before WaitForApproval")
	}

	// First, acquire a connection to insert the approval request
	conn, err := a.pool.Acquire(ctx)
	if err != nil {
		return stepapi.ApprovalOutcome{}, fmt.Errorf("failed to acquire db connection: %w", err)
	}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		conn.Release()
		return stepapi.ApprovalOutcome{}, fmt.Errorf("failed to marshal approval request: %w", err)
	}

	approvalTTL := "48 hours"
	if req.Metadata != nil {
		if rawTTL, ok := req.Metadata["approval_ttl"]; ok && rawTTL != "" {
			if parsed, parseErr := time.ParseDuration(rawTTL); parseErr == nil && parsed > 0 {
				approvalTTL = fmt.Sprintf("%f seconds", parsed.Seconds())
			}
		}
	}

	_, err = conn.Exec(
		ctx,
		`INSERT INTO steps (job_id, run_id, attempt, step_key, status, approval_request, approval_ttl, approval_expires_at, started_at)
		 VALUES ($1,$2,$3,$4,'waiting_approval',$5,$6::interval, now() + $6::interval, now())
		 ON CONFLICT (job_id, attempt, step_key)
		 DO UPDATE SET status='waiting_approval', approval_request=$5, approval_ttl=$6::interval, approval_expires_at=now() + $6::interval`,
		a.jobID,
		a.runID,
		a.attempt,
		a.currentStep,
		reqBytes,
		approvalTTL,
	)
	conn.Release() // Release connection immediately after insert

	if err != nil {
		return stepapi.ApprovalOutcome{}, fmt.Errorf("failed to park step for approval: %w", err)
	}

	ticker := time.NewTicker(a.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return stepapi.ApprovalOutcome{}, ctx.Err()
		case <-ticker.C:
			// Acquire a new connection for each poll attempt
			pollConn, err := a.pool.Acquire(ctx)
			if err != nil {
				// If we can't acquire a connection, continue polling
				continue
			}

			var outcome stepapi.ApprovalOutcome
			err = pollConn.QueryRow(
				ctx,
				`SELECT decision, approver, COALESCE(justification,''), created_at
				 FROM approval_decisions
				 WHERE job_id = $1
				   AND run_id = $2
				   AND step_id = (
					 SELECT id
					 FROM steps
					 WHERE job_id = $1 AND run_id = $2 AND attempt = $3 AND step_key = $4
					 ORDER BY id DESC
					 LIMIT 1
				   )
				 ORDER BY created_at DESC
				 LIMIT 1`,
				a.jobID,
				a.runID,
				a.attempt,
				a.currentStep,
			).Scan(&outcome.Decision, &outcome.Approver, &outcome.Justification, &outcome.DecidedAt)

			pollConn.Release() // Release connection immediately after query

			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					continue
				}
				return stepapi.ApprovalOutcome{}, fmt.Errorf("failed reading approval decision: %w", err)
			}
			return outcome, nil
		}
	}
}
