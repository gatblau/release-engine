package model

import (
	"time"
)

type ApprovalDecision string

const (
	DecisionApproved ApprovalDecision = "approved"
	DecisionRejected ApprovalDecision = "rejected"
	DecisionExpired  ApprovalDecision = "expired"
)

type ApprovalDecisionRecord struct {
	ID             string                 `json:"id"`
	JobID          string                 `json:"job_id"`
	StepID         int64                  `json:"step_id"`
	RunID          string                 `json:"run_id"`
	Decision       ApprovalDecision       `json:"decision"`
	Approver       string                 `json:"approver"`
	Justification  string                 `json:"justification"`
	PolicySnapshot map[string]interface{} `json:"policy_snapshot"`
	IdempotencyKey string                 `json:"idempotency_key"`
	CreatedAt      time.Time              `json:"created_at"`
}

type StepRecord struct {
	ID                int64                  `json:"id"`
	JobID             string                 `json:"job_id"`
	RunID             string                 `json:"run_id"`
	Attempt           int                    `json:"attempt"`
	StepKey           string                 `json:"step_key"`
	Status            string                 `json:"status"`
	OutputJSON        map[string]interface{} `json:"output_json"`
	ErrorCode         string                 `json:"error_code"`
	ErrorMessage      string                 `json:"error_message"`
	StartedAt         time.Time              `json:"started_at"`
	FinishedAt        *time.Time             `json:"finished_at"`
	DurationMS        int                    `json:"duration_ms"`
	ApprovalRequest   map[string]interface{} `json:"approval_request"`
	ApprovalTTL       string                 `json:"approval_ttl"`
	ApprovalExpiresAt *time.Time             `json:"approval_expires_at"`
}
