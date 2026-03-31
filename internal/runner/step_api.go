// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gatblau/release-engine/internal/audit"
	"github.com/gatblau/release-engine/internal/connector"
	"github.com/gatblau/release-engine/internal/db"
	"github.com/gatblau/release-engine/internal/logger"
	"github.com/gatblau/release-engine/internal/secrets"
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
	vaultManager   *secrets.Manager
	jobID          string
	runID          string
	attempt        int
	currentStep    string
	contextStore   map[string]any
	pollInterval   time.Duration
	logger         *zap.Logger
	auditService   *audit.AuditService
	tenantID       string
	// module is the module that this StepAPI is serving
	module any
	// resolvedConnectors stores connectors resolved at module assembly time
	// Key: family name, Value: resolved connector
	resolvedConnectors map[string]connector.Connector
}

func runnerPollInterval() time.Duration {
	if raw := strings.TrimSpace(os.Getenv("RUNNER_POLL_INTERVAL")); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil && d > 0 {
			return d
		}
	}
	return 500 * time.Millisecond
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
		pollInterval:   runnerPollInterval(),
		logger:         log,
	}
}

// NewStepAPIAdapterWithVault creates the module-facing runtime API for a specific job run with Volta integration.
func NewStepAPIAdapterWithVault(pool db.Pool, familyRegistry connector.FamilyRegistry, vaultManager *secrets.Manager, jobID, runID string, attempt int) StepAPI {
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
		vaultManager:   vaultManager,
		jobID:          jobID,
		runID:          runID,
		attempt:        attempt,
		contextStore:   make(map[string]any),
		pollInterval:   runnerPollInterval(),
		logger:         log,
	}
}

// NewStepAPIAdapterWithVaultAndAudit creates the module-facing runtime API for a specific job run with Volta integration and audit logging.
func NewStepAPIAdapterWithVaultAndAudit(pool db.Pool, familyRegistry connector.FamilyRegistry, vaultManager *secrets.Manager, auditService *audit.AuditService, jobID, runID string, attempt int) StepAPI {
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
		vaultManager:   vaultManager,
		auditService:   auditService,
		jobID:          jobID,
		runID:          runID,
		attempt:        attempt,
		contextStore:   make(map[string]any),
		pollInterval:   runnerPollInterval(),
		logger:         log,
	}
}

// NewStepAPIAdapterWithModule creates the module-facing runtime API for a specific job run with module reference.
func NewStepAPIAdapterWithModule(pool db.Pool, familyRegistry connector.FamilyRegistry, jobID, runID string, attempt int, module any) StepAPI {
	adapter := NewStepAPIAdapter(pool, familyRegistry, jobID, runID, attempt).(*stepAPIAdapter)
	adapter.module = module
	return adapter
}

// NewStepAPIAdapterWithConnectors creates the module-facing runtime API with pre-resolved connectors.
// This is used by the config-managed module assembly path where connectors are resolved at assembly time.
func NewStepAPIAdapterWithConnectors(pool db.Pool, jobID, runID string, attempt int, module any, connectors map[string]connector.Connector) StepAPI {
	// Create a logger for the module execution
	loggerFactory, err := logger.NewFactory("info", "console")
	var log *zap.Logger
	if err != nil {
		log = zap.NewNop()
	} else {
		log = loggerFactory.New(fmt.Sprintf("module.job.%s", jobID))
	}

	return &stepAPIAdapter{
		pool:               pool,
		jobID:              jobID,
		runID:              runID,
		attempt:            attempt,
		module:             module,
		contextStore:       make(map[string]any),
		pollInterval:       runnerPollInterval(),
		logger:             log,
		resolvedConnectors: connectors,
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
	a.logger.Error("step failed", zap.String("step", stepKey), zap.String("code", code), zap.String("message", msg))
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
	// Look up connector from pre-resolved connectors (set at module assembly time)
	var conn connector.Connector
	var err error

	if a.resolvedConnectors != nil {
		var ok bool
		conn, ok = a.resolvedConnectors[req.Connector]
		if !ok {
			return &stepapi.ConnectorResult{
				Status: "error",
				Error: &struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				}{
					Code:    "CONNECTOR_NOT_FOUND",
					Message: fmt.Sprintf("no connector resolved for family %q", req.Connector),
				},
			}, nil
		}
	} else if a.familyRegistry != nil {
		// Fallback to family registry resolution for legacy modules
		conn, err = a.familyRegistry.Resolve(req.Connector, req.ImplKey)
		if err != nil {
			return &stepapi.ConnectorResult{
				Status: "error",
				Error: &struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				}{
					Code:    "CONNECTOR_NOT_FOUND",
					Message: fmt.Sprintf("connector family %s with impl %s not resolved: %v", req.Connector, req.ImplKey, err),
				},
			}, nil
		}
	} else {
		return &stepapi.ConnectorResult{
			Status: "error",
			Error: &struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			}{
				Code:    "CONNECTOR_NOT_FOUND",
				Message: fmt.Sprintf("no connector available for family %q", req.Connector),
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

	// Determine required secrets
	var requiredSecrets []string
	if secretReq, ok := conn.(connector.SecretRequirer); ok {
		requiredSecrets = secretReq.RequiredSecrets(req.Operation)
	}

	// No secrets needed — execute directly with empty secrets map
	if len(requiredSecrets) == 0 {
		result, err := conn.Execute(ctx, req.Operation, req.Input, nil)
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

	// Phase 2: Volta integration - fetch secrets via Volta and execute connector
	// Check if module provides tenant context
	var secretCtx connector.SecretContext
	if a.module != nil {
		if provider, ok := a.module.(connector.SecretContextProvider); ok {
			secretCtx = provider.SecretContext()
		} else {
			// Module doesn't implement SecretContextProvider but connector requires secrets
			return &stepapi.ConnectorResult{
				Status: "error",
				Error: &struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				}{
					Code:    "TENANT_CONTEXT_MISSING",
					Message: "connector requires secrets but module doesn't provide tenant context",
				},
			}, nil
		}
	} else {
		if a.tenantID != "" {
			secretCtx = connector.SecretContext{TenantID: a.tenantID}
		} else {
			return &stepapi.ConnectorResult{
				Status: "error",
				Error: &struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				}{
					Code:    "MODULE_MISSING",
					Message: "module reference not available for secret resolution",
				},
			}, nil
		}
	}

	// Check if vault manager is available
	if a.vaultManager == nil {
		return &stepapi.ConnectorResult{
			Status: "error",
			Error: &struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			}{
				Code:    "VOLTA_NOT_CONFIGURED",
				Message: "Volta secret manager not configured",
			},
		}, nil
	}

	// Build physical keys, maintain logical mapping
	physicalToLogical := make(map[string]string, len(requiredSecrets))
	physicalKeys := make([]string, len(requiredSecrets))
	for i, logicalKey := range requiredSecrets {
		physical := logicalKey
		physicalKeys[i] = physical
		physicalToLogical[physical] = logicalKey
	}

	// Get vault for the tenant
	vault, err := a.vaultManager.GetVault(ctx, secretCtx.TenantID)
	if err != nil {
		return &stepapi.ConnectorResult{
			Status: "error",
			Error: &struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			}{
				Code:    "VAULT_UNAVAILABLE",
				Message: fmt.Sprintf("failed to get vault for tenant %s: %v", secretCtx.TenantID, err),
			},
		}, nil
	}

	// Log audit event for secret access attempt
	if a.auditService != nil {
		auditEvent := audit.AuditEvent{
			TenantID:  secretCtx.TenantID,
			Principal: "system", // System-initiated access
			Action:    "secret.access",
			Target:    strings.Join(physicalKeys, ","),
			Decision:  "granted",
			Reason:    fmt.Sprintf("Connector %s operation %s requires secrets", req.Connector, req.Operation),
			Metadata: map[string]string{
				"connector": req.Connector,
				"operation": req.Operation,
				"job_id":    a.jobID,
				"run_id":    a.runID,
				"attempt":   fmt.Sprintf("%d", a.attempt),
			},
			Timestamp: time.Now().UTC(),
			JobID:     a.jobID,
			RunID:     a.runID,
		}
		// Log the audit event in background - we don't want audit failure to block execution
		go func() {
			auditCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			if err = a.auditService.Record(auditCtx, auditEvent); err != nil {
				a.logger.Error("failed to record audit event for secret access",
					zap.String("tenant", secretCtx.TenantID),
					zap.String("connector", req.Connector),
					zap.Error(err))
			}
		}()
	}

	// Execute within Volta's secure scope
	var result *connector.ConnectorResult
	err = vault.UseSecrets(physicalKeys, func(secrets map[string][]byte) error {
		// Remap to logical keys for connector
		logicalSecrets := make(map[string][]byte, len(secrets))
		for physical, value := range secrets {
			logicalKey := physicalToLogical[physical]
			logicalSecrets[logicalKey] = value
		}

		var execErr error
		result, execErr = conn.Execute(ctx, req.Operation, req.Input, logicalSecrets)
		return execErr
	})

	if err != nil {
		// Log failed audit event if audit service is available
		if a.auditService != nil {
			auditEvent := audit.AuditEvent{
				TenantID:  secretCtx.TenantID,
				Principal: "system",
				Action:    "secret.access",
				Target:    strings.Join(physicalKeys, ","),
				Decision:  "denied",
				Reason:    fmt.Sprintf("Secret execution failed: %v", err),
				Metadata: map[string]string{
					"connector": req.Connector,
					"operation": req.Operation,
					"job_id":    a.jobID,
					"run_id":    a.runID,
					"attempt":   fmt.Sprintf("%d", a.attempt),
					"error":     err.Error(),
				},
				Timestamp: time.Now().UTC(),
				JobID:     a.jobID,
				RunID:     a.runID,
			}
			go func() {
				auditCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				defer cancel()
				if err := a.auditService.Record(auditCtx, auditEvent); err != nil {
					a.logger.Error("failed to record audit event for secret access failure",
						zap.String("tenant", secretCtx.TenantID),
						zap.String("connector", req.Connector),
						zap.Error(err))
				}
			}()
		}
		return &stepapi.ConnectorResult{
			Status: "error",
			Error: &struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			}{
				Code:    "SECRET_EXECUTION_FAILED",
				Message: fmt.Sprintf("failed to execute with secrets: %v", err),
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

func (a *stepAPIAdapter) ResolveSecret(ctx context.Context, tenantID, key string) (string, error) {
	return "", fmt.Errorf("secret resolution not configured")
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
