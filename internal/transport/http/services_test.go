package http

import (
	"context"
	"testing"
	"time"

	"github.com/gatblau/release-engine/internal/outbox"
	"github.com/stretchr/testify/assert"
)

type approvalMetricsStub struct {
	requests    int
	decisions   int
	latencies   int
	escalations int
	timeouts    int
	workerTicks int
}

func (m *approvalMetricsStub) RecordApprovalRequest(_, _, _ string) {
	m.requests++
}

func (m *approvalMetricsStub) RecordApprovalDecision(_, _, _, _ string) {
	m.decisions++
}

func (m *approvalMetricsStub) RecordApprovalLatency(_, _ string, _ time.Duration) {
	m.latencies++
}

func (m *approvalMetricsStub) RecordApprovalEscalation(_, _ string) {
	m.escalations++
}

func (m *approvalMetricsStub) RecordApprovalTimeout(_, _ string) {
	m.timeouts++
}

func (m *approvalMetricsStub) RecordApprovalWorkerTick(_ string, _ time.Duration) {
	m.workerTicks++
}

func TestNewPolicyEngine(t *testing.T) {
	pe := NewPolicyEngine()
	assert.NotNil(t, pe)
}

func TestPolicyEngine_Evaluate(t *testing.T) {
	pe := NewPolicyEngine()

	result := pe.Evaluate("test-request-id")
	assert.True(t, result)
}

func TestPolicyEngine_Evaluate_MultipleIDs(t *testing.T) {
	pe := NewPolicyEngine()

	// Test with different request IDs
	assert.True(t, pe.Evaluate("request-1"))
	assert.True(t, pe.Evaluate("request-2"))
	assert.True(t, pe.Evaluate(""))
}

func TestNewIdempotencyService(t *testing.T) {
	is := NewIdempotencyService()
	assert.NotNil(t, is)
}

func TestIdempotencyService_Proccess(t *testing.T) {
	is := NewIdempotencyService()

	result := is.Proccess("test-transaction-id")
	assert.True(t, result)
}

func TestIdempotencyService_Proccess_MultipleIDs(t *testing.T) {
	is := NewIdempotencyService()

	// Test with different transaction IDs
	assert.True(t, is.Proccess("tx-001"))
	assert.True(t, is.Proccess("tx-002"))
	assert.True(t, is.Proccess(""))
}

func TestPolicyEngine_EvaluateApproval_Allows_WhenConstraintsSatisfied(t *testing.T) {
	pe := NewPolicyEngine()

	result := pe.EvaluateApproval(ApprovalPolicyInput{
		Approver:         "approver-1",
		ApproverRole:     "release-manager",
		ApproverTenantID: "acme-prod",
		JobOwner:         "owner-1",
		JobTenantID:      "acme-prod",
		AllowedRoles:     []string{"release-manager", "team-lead"},
		SelfApproval:     false,
	})

	assert.True(t, result.Allowed)
	assert.Empty(t, result.Violations)
}

func TestPolicyEngine_EvaluateApproval_Denies_TenantMismatch(t *testing.T) {
	pe := NewPolicyEngine()

	result := pe.EvaluateApproval(ApprovalPolicyInput{
		Approver:         "approver-1",
		ApproverRole:     "release-manager",
		ApproverTenantID: "acme-dev",
		JobOwner:         "owner-1",
		JobTenantID:      "acme-prod",
		AllowedRoles:     []string{"release-manager", "team-lead"},
		SelfApproval:     false,
	})

	assert.False(t, result.Allowed)
	assert.Contains(t, result.Violations, "tenant_scope_mismatch")
}

func TestPolicyEngine_EvaluateApproval_Denies_BudgetExceeded(t *testing.T) {
	pe := NewPolicyEngine()

	result := pe.EvaluateApproval(ApprovalPolicyInput{
		Approver:         "approver-1",
		ApproverRole:     "release-manager",
		ApproverTenantID: "acme-prod",
		JobOwner:         "owner-1",
		JobTenantID:      "acme-prod",
		AllowedRoles:     []string{"release-manager", "team-lead"},
		SelfApproval:     false,
		Metadata: map[string]string{
			"estimated_cost": "2000",
			"approver_limit": "1500",
		},
	})

	assert.False(t, result.Allowed)
	assert.Contains(t, result.Violations, "budget_exceeded")
}

func TestPolicyEngine_EvaluateApproval_Allows_SelfApproval_WhenEnabled(t *testing.T) {
	pe := NewPolicyEngine()

	result := pe.EvaluateApproval(ApprovalPolicyInput{
		Approver:         "owner-1",
		ApproverRole:     "release-manager",
		ApproverTenantID: "acme-prod",
		JobOwner:         "owner-1",
		JobTenantID:      "acme-prod",
		AllowedRoles:     []string{"release-manager", "team-lead"},
		SelfApproval:     true,
	})

	assert.True(t, result.Allowed)
}

func TestPolicyEngine_EvaluateApproval_Denies_SelfApproval_WhenDisabled(t *testing.T) {
	pe := NewPolicyEngine()

	result := pe.EvaluateApproval(ApprovalPolicyInput{
		Approver:         "owner-1",
		ApproverRole:     "release-manager",
		ApproverTenantID: "acme-prod",
		JobOwner:         "owner-1",
		JobTenantID:      "acme-prod",
		AllowedRoles:     []string{"release-manager", "team-lead"},
		SelfApproval:     false,
	})

	assert.False(t, result.Allowed)
	assert.Contains(t, result.Violations, "self_approval_blocked")
}

func TestPolicyEngine_EvaluateApproval_Allows_RoleCaseInsensitive(t *testing.T) {
	pe := NewPolicyEngine()

	result := pe.EvaluateApproval(ApprovalPolicyInput{
		Approver:         "approver-1",
		ApproverRole:     "release-manager",
		ApproverTenantID: "acme-prod",
		JobOwner:         "owner-1",
		JobTenantID:      "acme-prod",
		AllowedRoles:     []string{"Release-Manager", "Team-Lead"},
		SelfApproval:     false,
	})

	assert.True(t, result.Allowed)
}

func TestApprovalService_SubmitDecision_FourEyesProgression(t *testing.T) {
	service := NewApprovalService(NewPolicyEngine())
	service.SeedStep(approvalStepState{
		jobID:    "job-4eyes",
		stepID:   "step-1",
		tenantID: "acme-prod",
		pathKey:  "deploy-production",
		status:   "waiting_approval",
		jobOwner: "owner-1",
		context: ApprovalContext{
			Summary:     "Approve prod deployment",
			Detail:      "Release 1.2.3",
			BlastRadius: "high",
			PolicyRef:   "paths.deploy-production.approval",
		},
		policy: ApprovalPolicy{Required: true, MinApprovers: 2, AllowedRoles: []string{"release-manager", "team-lead"}, SelfApproval: false},
	})

	first, err := service.SubmitDecision(context.TODO(), DecisionInput{
		JobID:            "job-4eyes",
		StepID:           "step-1",
		Decision:         "approved",
		IdempotencyKey:   "k-4eyes-1",
		Approver:         "approver-1",
		ApproverRole:     "release-manager",
		ApproverTenantID: "acme-prod",
	})
	assert.NoError(t, err)
	assert.Equal(t, "waiting_approval", first.State)
	assert.Equal(t, 1, first.RemainingApprovals)

	second, err := service.SubmitDecision(context.TODO(), DecisionInput{
		JobID:            "job-4eyes",
		StepID:           "step-1",
		Decision:         "approved",
		IdempotencyKey:   "k-4eyes-2",
		Approver:         "approver-2",
		ApproverRole:     "team-lead",
		ApproverTenantID: "acme-prod",
	})
	assert.NoError(t, err)
	assert.Equal(t, "running", second.State)
	assert.Equal(t, 0, second.RemainingApprovals)
}

func TestApprovalService_TickApprovals_EmitsEscalation(t *testing.T) {
	base := time.Date(2026, 3, 11, 10, 0, 0, 0, time.UTC)
	service := NewApprovalService(NewPolicyEngine())
	service.defaultTTL = 10 * time.Minute
	service.escalationAt = 0.8
	service.nowFn = func() time.Time { return base }

	requested := base.Add(-9 * time.Minute)
	expires := base.Add(1 * time.Minute)
	service.SeedStep(approvalStepState{
		jobID:       "job-escalate",
		stepID:      "step-1",
		tenantID:    "acme-prod",
		pathKey:     "deploy-production",
		status:      "waiting_approval",
		jobOwner:    "owner-1",
		requestedAt: requested,
		expiresAt:   &expires,
		context: ApprovalContext{
			Summary: "Approve prod deployment",
			Detail:  "Release 1.2.3",
		},
		policy: ApprovalPolicy{Required: true, MinApprovers: 2},
	})

	service.TickApprovals(context.Background())
	events := service.DrainEvents()

	assert.GreaterOrEqual(t, len(events), 1)
	assert.Contains(t, eventTypes(events), outbox.EventApprovalEscalated)

	ctx, err := service.GetApprovalContext(context.Background(), "job-escalate", "step-1")
	assert.NoError(t, err)
	assert.Equal(t, "waiting_approval", ctx.Status)
}

func TestApprovalService_TickApprovals_ExpiresStep(t *testing.T) {
	base := time.Date(2026, 3, 11, 10, 0, 0, 0, time.UTC)
	service := NewApprovalService(NewPolicyEngine())
	service.defaultTTL = 10 * time.Minute
	service.nowFn = func() time.Time { return base }

	requested := base.Add(-11 * time.Minute)
	expires := base.Add(-1 * time.Minute)
	service.SeedStep(approvalStepState{
		jobID:       "job-expire",
		stepID:      "step-1",
		tenantID:    "acme-prod",
		pathKey:     "deploy-production",
		status:      "waiting_approval",
		jobOwner:    "owner-1",
		requestedAt: requested,
		expiresAt:   &expires,
		policy:      ApprovalPolicy{Required: true, MinApprovers: 1},
	})

	service.TickApprovals(context.Background())
	events := service.DrainEvents()

	assert.GreaterOrEqual(t, len(events), 1)
	assert.Contains(t, eventTypes(events), outbox.EventApprovalExpired)

	ctx, err := service.GetApprovalContext(context.Background(), "job-expire", "step-1")
	assert.NoError(t, err)
	assert.Equal(t, "error", ctx.Status)
	assert.Equal(t, "expired", ctx.Decisions[len(ctx.Decisions)-1].Decision)
	assert.Equal(t, "system", ctx.Decisions[len(ctx.Decisions)-1].Approver)
}

func TestApprovalService_SubmitDecision_EmitsApprovalDecidedEvent(t *testing.T) {
	service := NewApprovalService(NewPolicyEngine())
	service.SeedStep(approvalStepState{
		jobID:    "job-decide",
		stepID:   "step-1",
		tenantID: "acme-prod",
		pathKey:  "deploy-production",
		status:   "waiting_approval",
		jobOwner: "owner-1",
		policy:   ApprovalPolicy{Required: true, MinApprovers: 1, AllowedRoles: []string{"release-manager"}, SelfApproval: false},
	})

	_, err := service.SubmitDecision(context.TODO(), DecisionInput{
		JobID:            "job-decide",
		StepID:           "step-1",
		Decision:         "approved",
		IdempotencyKey:   "idem-decide",
		Approver:         "approver-1",
		ApproverRole:     "release-manager",
		ApproverTenantID: "acme-prod",
	})
	assert.NoError(t, err)

	events := service.DrainEvents()
	assert.Contains(t, eventTypes(events), outbox.EventApprovalDecided)
}

func TestApprovalService_MetricsEmission(t *testing.T) {
	metrics := &approvalMetricsStub{}
	service := NewApprovalService(NewPolicyEngine())
	service.AttachMetrics(metrics)

	service.SeedStep(approvalStepState{
		jobID:    "job-metrics",
		stepID:   "step-1",
		tenantID: "acme-prod",
		pathKey:  "deploy-production",
		status:   "waiting_approval",
		jobOwner: "owner-1",
		policy:   ApprovalPolicy{Required: true, MinApprovers: 1, AllowedRoles: []string{"release-manager"}, SelfApproval: false},
	})

	_, err := service.SubmitDecision(context.TODO(), DecisionInput{
		JobID:            "job-metrics",
		StepID:           "step-1",
		Decision:         "approved",
		IdempotencyKey:   "idem-metrics",
		Approver:         "approver-1",
		ApproverRole:     "release-manager",
		ApproverTenantID: "acme-prod",
	})
	assert.NoError(t, err)

	assert.Equal(t, 1, metrics.requests)
	assert.Equal(t, 1, metrics.decisions)
	assert.Equal(t, 1, metrics.latencies)
}

func eventTypes(events []ApprovalEvent) []string {
	types := make([]string, 0, len(events))
	for _, event := range events {
		types = append(types, event.Type)
	}
	return types
}
