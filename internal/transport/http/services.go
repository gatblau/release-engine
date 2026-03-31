package http

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gatblau/release-engine/internal/outbox"
)

type PolicyEngine struct{}

func NewPolicyEngine() *PolicyEngine {
	return &PolicyEngine{}
}

func (p *PolicyEngine) Evaluate(requestID string) bool {
	// TODO: Go-native evaluator
	return true
}

// EvaluateApproval evaluates approval policy constraints for a decision request.
func (p *PolicyEngine) EvaluateApproval(input ApprovalPolicyInput) ApprovalPolicyResult {
	if input.Approver == "" {
		return ApprovalPolicyResult{Allowed: false, Reason: "missing approver", Violations: []string{"approver_missing"}}
	}
	if !input.SelfApproval && input.JobOwner != "" && input.Approver == input.JobOwner {
		return ApprovalPolicyResult{Allowed: false, Reason: "self approval blocked", Violations: []string{"self_approval_blocked"}}
	}
	if input.JobTenantID != "" {
		if input.ApproverTenantID == "" {
			return ApprovalPolicyResult{Allowed: false, Reason: "approver tenant missing", Violations: []string{"approver_tenant_missing"}}
		}
		if input.ApproverTenantID != input.JobTenantID {
			return ApprovalPolicyResult{Allowed: false, Reason: "tenant scope mismatch", Violations: []string{"tenant_scope_mismatch"}}
		}
	}
	if len(input.AllowedRoles) > 0 {
		allowed := false
		for _, role := range input.AllowedRoles {
			if strings.EqualFold(role, input.ApproverRole) {
				allowed = true
				break
			}
		}
		if !allowed {
			return ApprovalPolicyResult{Allowed: false, Reason: "role not authorised", Violations: []string{"role_not_allowed"}}
		}
	}
	if estimated, ok := parseFloat(input.Metadata, "estimated_cost"); ok {
		if limit, hasLimit := parseFloat(input.Metadata, "approver_limit"); hasLimit && estimated > limit {
			return ApprovalPolicyResult{Allowed: false, Reason: "budget authority exceeded", Violations: []string{"budget_exceeded"}}
		}
	}
	return ApprovalPolicyResult{Allowed: true}
}

type IdempotencyService struct{}

func NewIdempotencyService() *IdempotencyService {
	return &IdempotencyService{}
}

func (i *IdempotencyService) Proccess(transactionID string) bool {
	// TODO: Deterministic intake transaction
	return true
}

type ApprovalPolicyInput struct {
	Approver         string
	ApproverRole     string
	ApproverTenantID string
	JobOwner         string
	JobTenantID      string
	PathKey          string
	StepKey          string
	BlastRadius      string
	Metadata         map[string]string
	AllowedRoles     []string
	SelfApproval     bool
	MinApprovers     int
}

type ApprovalPolicyResult struct {
	Allowed    bool
	Reason     string
	Violations []string
}

type ApprovalContext struct {
	Summary     string            `json:"summary"`
	Detail      string            `json:"detail"`
	BlastRadius string            `json:"blast_radius"`
	PolicyRef   string            `json:"policy_ref"`
	Metadata    map[string]string `json:"metadata"`
}

type ApprovalTally struct {
	Approved           int `json:"approved"`
	Rejected           int `json:"rejected"`
	RemainingToApprove int `json:"remaining_to_approve"`
}

type ApprovalContextResponse struct {
	JobID     string           `json:"job_id"`
	StepID    string           `json:"step_id"`
	Status    string           `json:"status"`
	Context   ApprovalContext  `json:"context"`
	Policy    ApprovalPolicy   `json:"policy"`
	Tally     ApprovalTally    `json:"tally"`
	Decisions []DecisionRecord `json:"decisions"`
	ExpiresAt *time.Time       `json:"expires_at,omitempty"`
}

type ApprovalPolicy struct {
	Required     bool     `json:"required"`
	MinApprovers int      `json:"min_approvers"`
	AllowedRoles []string `json:"allowed_roles"`
	SelfApproval bool     `json:"self_approval"`
}

type DecisionRecord struct {
	Decision      string    `json:"decision"`
	Approver      string    `json:"approver"`
	Justification string    `json:"justification,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

type ApprovalEvent struct {
	Type       string         `json:"type"`
	JobID      string         `json:"job_id"`
	StepID     string         `json:"step_id"`
	OccurredAt time.Time      `json:"occurred_at"`
	Payload    map[string]any `json:"payload,omitempty"`
}

type PendingApprovalJob struct {
	JobID      string `json:"job_id"`
	TenantID   string `json:"tenant_id"`
	PathKey    string `json:"path_key"`
	StepID     string `json:"step_id"`
	StepStatus string `json:"step_status"`
}

type DecisionInput struct {
	JobID            string
	StepID           string
	Decision         string
	Justification    string
	IdempotencyKey   string
	Approver         string
	ApproverRole     string
	ApproverTenantID string
}

type DecisionOutput struct {
	JobID              string    `json:"job_id"`
	StepID             string    `json:"step_id"`
	Decision           string    `json:"decision"`
	State              string    `json:"state"`
	IdempotencyKey     string    `json:"idempotency_key"`
	IdempotentReplay   bool      `json:"idempotent_replay"`
	RemainingApprovals int       `json:"remaining_approvals"`
	RecordedAt         time.Time `json:"recorded_at"`
	Violations         []string  `json:"violations,omitempty"`
}

var (
	ErrApprovalStepNotFound        = errors.New("approval step not found")
	ErrApprovalNotWaiting          = errors.New("step is not waiting approval")
	ErrApprovalIdempotencyConflict = errors.New("idempotency key conflict")
	ErrApprovalDecisionConflict    = errors.New("approver has already decided")
	ErrApprovalForbidden           = errors.New("approver not authorised")
	ErrApprovalPolicyViolation     = errors.New("approval policy violation")
)

type approvalStepState struct {
	jobID       string
	stepID      string
	tenantID    string
	pathKey     string
	status      string
	jobOwner    string
	context     ApprovalContext
	policy      ApprovalPolicy
	requestedAt time.Time
	expiresAt   *time.Time
	escalated   bool
	decisions   map[string]DecisionRecord
}

type idempotencyRecord struct {
	payloadHash string
	output      DecisionOutput
}

type ApprovalService struct {
	mu           sync.Mutex
	steps        map[string]*approvalStepState
	idem         map[string]idempotencyRecord
	events       []ApprovalEvent
	outbox       outbox.Dispatcher
	policyEngine *PolicyEngine
	metrics      ApprovalMetricsRecorder
	nowFn        func() time.Time
	defaultTTL   time.Duration
	escalationAt float64
}

// ApprovalMetricsRecorder captures approval lifecycle telemetry.
type ApprovalMetricsRecorder interface {
	RecordApprovalRequest(tenantID, pathKey, stepKey string)
	RecordApprovalDecision(tenantID, pathKey, stepKey, decision string)
	RecordApprovalLatency(tenantID, pathKey string, latency time.Duration)
	RecordApprovalEscalation(tenantID, pathKey string)
	RecordApprovalTimeout(tenantID, pathKey string)
	RecordApprovalWorkerTick(status string, duration time.Duration)
}

func NewApprovalService(policyEngine *PolicyEngine) *ApprovalService {
	return NewApprovalServiceWithOutbox(policyEngine, nil)
}

func NewApprovalServiceWithOutbox(policyEngine *PolicyEngine, dispatcher outbox.Dispatcher) *ApprovalService {
	if policyEngine == nil {
		policyEngine = NewPolicyEngine()
	}
	service := &ApprovalService{
		steps:        make(map[string]*approvalStepState),
		idem:         make(map[string]idempotencyRecord),
		events:       make([]ApprovalEvent, 0),
		outbox:       dispatcher,
		policyEngine: policyEngine,
		nowFn:        time.Now,
		defaultTTL:   48 * time.Hour,
		escalationAt: 0.8,
	}
	now := service.nowFn().UTC()
	expires := now.Add(service.defaultTTL)
	service.SeedStep(approvalStepState{
		jobID:       "job-123",
		stepID:      "step-456",
		tenantID:    "acme-prod",
		pathKey:     "deploy-production",
		status:      "waiting_approval",
		jobOwner:    "owner-1",
		requestedAt: now,
		expiresAt:   &expires,
		context: ApprovalContext{
			Summary:     "Approve production deployment",
			Detail:      "Deploy release 2026.03.11 to production",
			BlastRadius: "high",
			PolicyRef:   "paths.deploy-production.approval",
			Metadata:    map[string]string{"change_id": "CHANGE-1234"},
		},
		policy: ApprovalPolicy{Required: true, MinApprovers: 2, AllowedRoles: []string{"release-manager", "team-lead"}, SelfApproval: false},
	})
	// Keep constructor deterministic for tests by clearing seeded bootstrap event(s).
	service.DrainEvents()
	return service
}

func stepMapKey(jobID, stepID string) string {
	return fmt.Sprintf("%s:%s", jobID, stepID)
}

// AttachMetrics configures a telemetry recorder for approval lifecycle instrumentation.
func (s *ApprovalService) AttachMetrics(recorder ApprovalMetricsRecorder) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.metrics = recorder
}

func (s *ApprovalService) SeedStep(step approvalStepState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.nowFn().UTC()
	if step.status == "" {
		step.status = "waiting_approval"
	}
	if step.requestedAt.IsZero() {
		step.requestedAt = now
	}
	if step.expiresAt == nil && step.status == "waiting_approval" {
		expires := step.requestedAt.Add(s.defaultTTL)
		step.expiresAt = &expires
	}
	if step.decisions == nil {
		step.decisions = make(map[string]DecisionRecord)
	}
	copy := step
	s.steps[stepMapKey(step.jobID, step.stepID)] = &copy
	if copy.status == "waiting_approval" {
		if s.metrics != nil {
			s.metrics.RecordApprovalRequest(copy.tenantID, copy.pathKey, copy.stepID)
		}
		s.recordEventLocked(ApprovalEvent{
			Type:       outbox.EventApprovalRequested,
			JobID:      copy.jobID,
			StepID:     copy.stepID,
			OccurredAt: now,
			Payload: map[string]any{
				"approval_request": copy.context,
				"policy_ref":       copy.context.PolicyRef,
				"expires_at":       copy.expiresAt,
			},
		})
	}
}

func (s *ApprovalService) TickApprovals(_ context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.nowFn().UTC()
	for _, step := range s.steps {
		if step.status != "waiting_approval" {
			continue
		}
		if step.expiresAt == nil {
			expires := now.Add(s.defaultTTL)
			step.expiresAt = &expires
		}

		total := step.expiresAt.Sub(step.requestedAt)
		if total <= 0 {
			total = s.defaultTTL
		}
		escalationPoint := step.requestedAt.Add(time.Duration(float64(total) * s.escalationAt))
		if !step.escalated && !now.Before(escalationPoint) && now.Before(*step.expiresAt) {
			step.escalated = true
			if s.metrics != nil {
				s.metrics.RecordApprovalEscalation(step.tenantID, step.pathKey)
			}
			s.recordEventLocked(ApprovalEvent{
				Type:       outbox.EventApprovalEscalated,
				JobID:      step.jobID,
				StepID:     step.stepID,
				OccurredAt: now,
				Payload: map[string]any{
					"elapsed_seconds":   int(now.Sub(step.requestedAt).Seconds()),
					"remaining_seconds": int(step.expiresAt.Sub(now).Seconds()),
					"approval_request":  step.context,
				},
			})
		}

		if !now.Before(*step.expiresAt) {
			step.status = "error"
			if s.metrics != nil {
				s.metrics.RecordApprovalTimeout(step.tenantID, step.pathKey)
			}
			if _, exists := step.decisions["system"]; !exists {
				step.decisions["system"] = DecisionRecord{
					Decision:      "expired",
					Approver:      "system",
					Justification: "approval_timeout",
					CreatedAt:     now,
				}
			}
			s.recordEventLocked(ApprovalEvent{
				Type:       outbox.EventApprovalExpired,
				JobID:      step.jobID,
				StepID:     step.stepID,
				OccurredAt: now,
				Payload: map[string]any{
					"reason":           "approval_timeout",
					"original_request": step.context,
					"ttl_seconds":      int(step.expiresAt.Sub(step.requestedAt).Seconds()),
				},
			})
		}
	}
}

func (s *ApprovalService) DrainEvents() []ApprovalEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.events) == 0 {
		return nil
	}
	events := make([]ApprovalEvent, len(s.events))
	copy(events, s.events)
	s.events = s.events[:0]
	return events
}

func (s *ApprovalService) EnsureTerminal(jobID, stepID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	step, exists := s.steps[stepMapKey(jobID, stepID)]
	if !exists {
		return false
	}
	if step.status == "waiting_approval" {
		step.status = "running"
		return true
	}
	return step.status == "running" || step.status == "succeeded" || step.status == "failed" || step.status == "jobs_exhausted"
}

func (s *ApprovalService) SubmitDecision(_ context.Context, input DecisionInput) (DecisionOutput, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	step, exists := s.steps[stepMapKey(input.JobID, input.StepID)]
	if !exists {
		return DecisionOutput{}, ErrApprovalStepNotFound
	}

	payloadHash := strings.Join([]string{input.JobID, input.StepID, input.Decision, input.Justification, input.Approver}, "|")
	if existing, ok := s.idem[input.IdempotencyKey]; ok {
		if existing.payloadHash != payloadHash {
			return DecisionOutput{}, ErrApprovalIdempotencyConflict
		}
		existing.output.IdempotentReplay = true
		return existing.output, nil
	}

	if step.status != "waiting_approval" {
		return DecisionOutput{}, ErrApprovalNotWaiting
	}

	if _, exists = step.decisions[input.Approver]; exists {
		return DecisionOutput{}, ErrApprovalDecisionConflict
	}

	policyResult := s.policyEngine.EvaluateApproval(ApprovalPolicyInput{
		Approver:         input.Approver,
		ApproverRole:     input.ApproverRole,
		ApproverTenantID: input.ApproverTenantID,
		JobOwner:         step.jobOwner,
		JobTenantID:      step.tenantID,
		PathKey:          step.pathKey,
		StepKey:          step.stepID,
		BlastRadius:      step.context.BlastRadius,
		Metadata:         step.context.Metadata,
		AllowedRoles:     step.policy.AllowedRoles,
		SelfApproval:     step.policy.SelfApproval,
		MinApprovers:     step.policy.MinApprovers,
	})
	if !policyResult.Allowed {
		if contains(policyResult.Violations, "self_approval_blocked") || contains(policyResult.Violations, "budget_exceeded") {
			return DecisionOutput{}, ErrApprovalPolicyViolation
		}
		return DecisionOutput{}, ErrApprovalForbidden
	}

	recordedAt := s.nowFn().UTC()
	recorded := DecisionRecord{
		Decision:      input.Decision,
		Approver:      input.Approver,
		Justification: input.Justification,
		CreatedAt:     recordedAt,
	}
	step.decisions[input.Approver] = recorded
	if s.metrics != nil {
		s.metrics.RecordApprovalDecision(step.tenantID, step.pathKey, step.stepID, input.Decision)
		s.metrics.RecordApprovalLatency(step.tenantID, step.pathKey, recordedAt.Sub(step.requestedAt))
	}

	approved := 0
	for _, d := range step.decisions {
		if d.Decision == "approved" {
			approved++
		}
	}

	requiredApprovals := step.policy.MinApprovers
	if requiredApprovals < 1 {
		requiredApprovals = 1
	}
	remaining := requiredApprovals - approved
	if remaining < 0 {
		remaining = 0
	}
	if input.Decision == "rejected" {
		step.status = "error"
	} else if remaining == 0 {
		step.status = "running"
	}

	output := DecisionOutput{
		JobID:              input.JobID,
		StepID:             input.StepID,
		Decision:           input.Decision,
		State:              step.status,
		IdempotencyKey:     input.IdempotencyKey,
		IdempotentReplay:   false,
		RemainingApprovals: remaining,
		RecordedAt:         recorded.CreatedAt,
	}
	s.recordEventLocked(ApprovalEvent{
		Type:       outbox.EventApprovalDecided,
		JobID:      input.JobID,
		StepID:     input.StepID,
		OccurredAt: recorded.CreatedAt,
		Payload: map[string]any{
			"decision":      input.Decision,
			"approver":      input.Approver,
			"justification": input.Justification,
		},
	})
	s.idem[input.IdempotencyKey] = idempotencyRecord{payloadHash: payloadHash, output: output}
	return output, nil
}

func (s *ApprovalService) recordEventLocked(event ApprovalEvent) {
	s.events = append(s.events, event)
	if s.outbox != nil {
		_ = s.outbox.Emit(context.Background(), outbox.Event{
			Type:       event.Type,
			JobID:      event.JobID,
			StepID:     event.StepID,
			OccurredAt: event.OccurredAt,
			Payload:    event.Payload,
		})
	}
}

func (s *ApprovalService) GetApprovalContext(_ context.Context, jobID, stepID string) (ApprovalContextResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	step, exists := s.steps[stepMapKey(jobID, stepID)]
	if !exists {
		return ApprovalContextResponse{}, ErrApprovalStepNotFound
	}

	approved, rejected := 0, 0
	decisions := make([]DecisionRecord, 0, len(step.decisions))
	for _, decision := range step.decisions {
		switch decision.Decision {
		case "approved":
			approved++
		case "rejected":
			rejected++
		}
		decisions = append(decisions, decision)
	}
	sort.Slice(decisions, func(i, j int) bool { return decisions[i].CreatedAt.Before(decisions[j].CreatedAt) })

	requiredApprovals := step.policy.MinApprovers
	if requiredApprovals < 1 {
		requiredApprovals = 1
	}
	remaining := requiredApprovals - approved
	if remaining < 0 {
		remaining = 0
	}

	return ApprovalContextResponse{
		JobID:     step.jobID,
		StepID:    step.stepID,
		Status:    step.status,
		Context:   step.context,
		Policy:    step.policy,
		Tally:     ApprovalTally{Approved: approved, Rejected: rejected, RemainingToApprove: remaining},
		Decisions: decisions,
		ExpiresAt: step.expiresAt,
	}, nil
}

func (s *ApprovalService) ListPendingJobs(_ context.Context, tenantID, approverRole string) []PendingApprovalJob {
	s.mu.Lock()
	defer s.mu.Unlock()

	jobs := make([]PendingApprovalJob, 0)
	for _, step := range s.steps {
		if step.status != "waiting_approval" {
			continue
		}
		if tenantID != "" && step.tenantID != tenantID {
			continue
		}
		if approverRole != "" && len(step.policy.AllowedRoles) > 0 {
			matched := false
			for _, role := range step.policy.AllowedRoles {
				if role == approverRole {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		jobs = append(jobs, PendingApprovalJob{
			JobID:      step.jobID,
			TenantID:   step.tenantID,
			PathKey:    step.pathKey,
			StepID:     step.stepID,
			StepStatus: step.status,
		})
	}

	sort.Slice(jobs, func(i, j int) bool {
		if jobs[i].TenantID == jobs[j].TenantID {
			if jobs[i].JobID == jobs[j].JobID {
				return jobs[i].StepID < jobs[j].StepID
			}
			return jobs[i].JobID < jobs[j].JobID
		}
		return jobs[i].TenantID < jobs[j].TenantID
	})
	return jobs
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func parseFloat(values map[string]string, key string) (float64, bool) {
	if values == nil {
		return 0, false
	}
	raw, ok := values[key]
	if !ok || strings.TrimSpace(raw) == "" {
		return 0, false
	}
	parsed, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, false
	}
	return parsed, true
}
