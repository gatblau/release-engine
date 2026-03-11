package model

import (
	"testing"
)

func TestApprovalModels(t *testing.T) {
	decision := ApprovalDecisionRecord{
		ID:       "123",
		Decision: DecisionApproved,
	}
	if decision.Decision != "approved" {
		t.Errorf("Expected approved, got %s", decision.Decision)
	}

	step := StepRecord{
		ID:     1,
		Status: "waiting_approval",
	}
	if step.Status != "waiting_approval" {
		t.Errorf("Expected waiting_approval, got %s", step.Status)
	}
}
