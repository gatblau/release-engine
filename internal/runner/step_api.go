package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gatblau/release-engine/internal/db"
	"github.com/jackc/pgx/v4"
)

// ApprovalOutcome defines the result of a human approval decision.
type ApprovalOutcome struct {
	Decision      string    `json:"decision"` // "approved" or "rejected"
	Approver      string    `json:"approver"`
	Justification string    `json:"justification"`
	DecidedAt     time.Time `json:"decided_at"`
}

// ApprovalRequest defines the context payload for approval gates.
type ApprovalRequest struct {
	Summary     string            `json:"summary"`
	Detail      string            `json:"detail"`
	BlastRadius string            `json:"blast_radius"`
	PolicyRef   string            `json:"policy_ref"`
	Metadata    map[string]string `json:"metadata"`
}

// ConnectorRequest is the connector invocation payload.
type ConnectorRequest struct {
	Connector string         `json:"connector"`
	Operation string         `json:"operation"`
	Input     map[string]any `json:"input"`
}

// ConnectorResult is the normalized connector call result.
type ConnectorResult struct {
	Status string         `json:"status"`
	Output map[string]any `json:"output,omitempty"`
	Error  *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// StepAPI defines the module-facing execution interface.
type StepAPI interface {
	BeginStep(stepKey string) error
	EndStepOK(stepKey string, output map[string]any) error
	EndStepErr(stepKey, code, msg string) error
	CallConnector(ctx context.Context, req ConnectorRequest) (*ConnectorResult, error)
	SetContext(key string, value any) error
	GetContext(key string) (any, bool)
	IsCancelled() bool
	// WaitForApproval parks the execution until an approval decision is recorded.
	WaitForApproval(ctx context.Context, req ApprovalRequest) (ApprovalOutcome, error)
}

type stepAPIAdapter struct {
	pool         db.Pool
	jobID        string
	runID        string
	attempt      int
	currentStep  string
	contextStore map[string]any
	pollInterval time.Duration
}

// NewStepAPIAdapter creates the module-facing runtime API for a specific job run.
func NewStepAPIAdapter(pool db.Pool, jobID, runID string, attempt int) StepAPI {
	return &stepAPIAdapter{
		pool:         pool,
		jobID:        jobID,
		runID:        runID,
		attempt:      attempt,
		contextStore: make(map[string]any),
		pollInterval: 500 * time.Millisecond,
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

func (a *stepAPIAdapter) CallConnector(ctx context.Context, req ConnectorRequest) (*ConnectorResult, error) {
	return nil, fmt.Errorf("connector calls are not implemented yet")
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

func (a *stepAPIAdapter) WaitForApproval(ctx context.Context, req ApprovalRequest) (ApprovalOutcome, error) {
	if a.currentStep == "" {
		return ApprovalOutcome{}, fmt.Errorf("BeginStep must be called before WaitForApproval")
	}

	conn, err := a.pool.Acquire(ctx)
	if err != nil {
		return ApprovalOutcome{}, fmt.Errorf("failed to acquire db connection: %w", err)
	}
	defer conn.Release()

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return ApprovalOutcome{}, fmt.Errorf("failed to marshal approval request: %w", err)
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
	if err != nil {
		return ApprovalOutcome{}, fmt.Errorf("failed to park step for approval: %w", err)
	}

	ticker := time.NewTicker(a.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ApprovalOutcome{}, ctx.Err()
		case <-ticker.C:
			var outcome ApprovalOutcome
			err := conn.QueryRow(
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
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					continue
				}
				return ApprovalOutcome{}, fmt.Errorf("failed reading approval decision: %w", err)
			}
			return outcome, nil
		}
	}
}
