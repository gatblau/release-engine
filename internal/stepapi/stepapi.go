// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package stepapi

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// TerminalError is a sentinel error type that signals an unrecoverable failure.
// When a module returns a TerminalError, the runner will immediately transition
// the job to jobs_exhausted state without retrying.
type TerminalError struct {
	Code    string
	Message string
}

func (e *TerminalError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return e.Message
}

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
// In the per-module design, Connector specifies the family name and ImplKey
// specifies the implementation key within that family.
type ConnectorRequest struct {
	Connector string         `json:"connector"` // family name, e.g., "git"
	ImplKey   string         `json:"impl_key"`  // implementation key, e.g., "git-github"
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
// This interface is used by both runner and modules.
type StepAPI interface {
	BeginStep(stepKey string) error
	EndStepOK(stepKey string, output map[string]any) error
	EndStepErr(stepKey, code, msg string) error
	CallConnector(ctx context.Context, req ConnectorRequest) (*ConnectorResult, error)
	ResolveSecret(ctx context.Context, tenantID, key string) (string, error)
	WaitForApproval(ctx context.Context, req ApprovalRequest) (ApprovalOutcome, error)
	SetContext(key string, value any) error
	GetContext(key string) (any, bool)
	IsCancelled() bool
	// Logger returns a component-scoped logger for the module.
	// The logger is configured with the appropriate log level from module configuration.
	Logger() *zap.Logger
}
