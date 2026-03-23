// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package stepapi

import (
	"context"
	"time"

	"go.uber.org/zap"
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
// This interface is used by both runner and modules.
type StepAPI interface {
	BeginStep(stepKey string) error
	EndStepOK(stepKey string, output map[string]any) error
	EndStepErr(stepKey, code, msg string) error
	CallConnector(ctx context.Context, req ConnectorRequest) (*ConnectorResult, error)
	WaitForApproval(ctx context.Context, req ApprovalRequest) (ApprovalOutcome, error)
	SetContext(key string, value any) error
	GetContext(key string) (any, bool)
	IsCancelled() bool
	// Logger returns a component-scoped logger for the module.
	// The logger is configured with the appropriate log level from module configuration.
	Logger() *zap.Logger
}
