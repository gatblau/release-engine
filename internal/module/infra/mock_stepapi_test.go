// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package infra

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gatblau/release-engine/internal/stepapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// MockStepAPI implements stepAPI for testing
type MockStepAPI struct {
	steps          []string
	stepOutputs    map[string]map[string]any
	stepErrors     map[string]string
	connectorCalls []stepapi.ConnectorRequest
	approvalResult *stepapi.ApprovalOutcome
	callConnector  func(ctx context.Context, req stepapi.ConnectorRequest) (*stepapi.ConnectorResult, error)
}

func (m *MockStepAPI) BeginStep(stepKey string) error {
	m.steps = append(m.steps, stepKey)
	return nil
}

func (m *MockStepAPI) EndStepOK(stepKey string, output map[string]any) error {
	if m.stepOutputs == nil {
		m.stepOutputs = make(map[string]map[string]any)
	}
	m.stepOutputs[stepKey] = output
	return nil
}

func (m *MockStepAPI) EndStepErr(stepKey, code, msg string) error {
	if m.stepErrors == nil {
		m.stepErrors = make(map[string]string)
	}
	m.stepErrors[stepKey] = msg
	return nil
}

func (m *MockStepAPI) SetContext(key string, value any) error {
	return nil
}

func (m *MockStepAPI) CallConnector(ctx context.Context, req stepapi.ConnectorRequest) (*stepapi.ConnectorResult, error) {
	m.connectorCalls = append(m.connectorCalls, req)

	if m.callConnector != nil {
		return m.callConnector(ctx, req)
	}

	// Default mock responses based on connector type
	switch req.Connector {
	case "policy":
		switch req.Operation {
		case "evaluate":
			return &stepapi.ConnectorResult{
				Status: "success",
				Output: map[string]any{
					"allowed":    true,
					"violations": []interface{}{},
				},
			}, nil
		}
	case "git":
		switch req.Operation {
		case "commit_files":
			return &stepapi.ConnectorResult{
				Status: "success",
				Output: map[string]any{
					"commit_sha": "abc123",
					"changed":    true,
				},
			}, nil
		case "read_file":
			return &stepapi.ConnectorResult{
				Status: "success",
				Output: map[string]any{
					"content": "status: healthy\ncommit_sha: abc123\ntimestamp: 2026-03-19T14:40:45Z",
					"sha":     "def456",
				},
			}, nil
		}
	case "webhook":
		switch req.Operation {
		case "post_callback":
			return &stepapi.ConnectorResult{
				Status: "success",
				Output: map[string]any{
					"status_code":   200,
					"response_body": `{"status":"success"}`,
				},
			}, nil
		}
	}

	return &stepapi.ConnectorResult{
		Status: "success",
		Output: map[string]any{},
	}, nil
}

func (m *MockStepAPI) WaitForApproval(ctx context.Context, req stepapi.ApprovalRequest) (stepapi.ApprovalOutcome, error) {
	if m.approvalResult != nil {
		return *m.approvalResult, nil
	}
	return stepapi.ApprovalOutcome{
		Decision: "approved",
		Approver: "test-approver",
	}, nil
}

func (m *MockStepAPI) GetContext(key string) (any, bool) {
	return nil, false
}

func (m *MockStepAPI) IsCancelled() bool {
	return false
}

func (m *MockStepAPI) Logger() *zap.Logger {
	return zap.NewNop()
}

func TestModule_Phase2_HappyPath(t *testing.T) {
	module := NewLegacyModule()
	mockStep := &MockStepAPI{}

	params := map[string]any{
		"idempotency_key":     "test-job-123",
		"job_id":              "job-123",
		"callback_url":        "https://example.com/callback",
		"infra_repo":          "org/infra-manifests",
		"contract_version":    "v1",
		"request_name":        "test-cluster-provision",
		"tenant":              "tenant-a",
		"owner":               "team-infra",
		"environment":         "production",
		"catalogue_item":      "k8s-app",
		"namespace":           "tenant-a-production",
		"workload_profile":    "medium",
		"availability":        "standard",
		"residency":           "us",
		"data_classification": "internal",
		"ingress_mode":        "private",
		"egress_mode":         "restricted",
		"cost_centre":         "team-infra-001",
		"kubernetes": map[string]any{
			"enabled":  true,
			"provider": "aws",
			"size":     "medium",
		},
		"primary_region": "us-west-2",
		"cluster_name":   "test-cluster",
		"cloud_provider": "aws",
		"region":         "us-west-2",
		"node_pools": []any{
			map[string]any{
				"name":          "default",
				"instance_type": "t3.medium",
				"min_count":     2,
				"max_count":     5,
			},
		},
		"networking": map[string]any{
			"vpc_cidr":     "10.0.0.0/16",
			"subnet_cidrs": []string{"10.0.1.0/24", "10.2.0.0/24"},
		},
	}

	ctx := context.Background()
	err := module.Execute(ctx, mockStep, params)

	require.NoError(t, err)

	// Verify steps were called in order
	expectedSteps := []string{
		"infra.render",
		"infra.policy_evaluate",
		"infra.approval_gate",
		"infra.phase1_complete",
		"infra.git_commit",
		"infra.health_poll",
		"infra.callback",
	}

	assert.Equal(t, expectedSteps, mockStep.steps)

	// Verify connector calls
	assert.GreaterOrEqual(t, len(mockStep.connectorCalls), 3)

	// First call should be policy.evaluate
	assert.Equal(t, "policy", mockStep.connectorCalls[0].Connector)
	assert.Equal(t, "evaluate", mockStep.connectorCalls[0].Operation)

	// Second call should be git.commit_files
	assert.Equal(t, "git", mockStep.connectorCalls[1].Connector)
	assert.Equal(t, "commit_files", mockStep.connectorCalls[1].Operation)

	// Third call should be git.read_file for health polling
	assert.Equal(t, "git", mockStep.connectorCalls[2].Connector)
	assert.Equal(t, "read_file", mockStep.connectorCalls[2].Operation)

	// Last call should be webhook.post_callback
	lastCall := mockStep.connectorCalls[len(mockStep.connectorCalls)-1]
	assert.Equal(t, "webhook", lastCall.Connector)
	assert.Equal(t, "post_callback", lastCall.Operation)
}

func TestModule_Phase2_IdempotentCommit(t *testing.T) {
	module := NewLegacyModule()
	mockStep := &MockStepAPI{
		approvalResult: &stepapi.ApprovalOutcome{
			Decision: "approved",
			Approver: "test-approver",
		},
	}

	// Set custom callConnector function
	mockStep.callConnector = func(ctx context.Context, req stepapi.ConnectorRequest) (*stepapi.ConnectorResult, error) {
		mockStep.connectorCalls = append(mockStep.connectorCalls, req)

		switch {
		case req.Connector == "git" && req.Operation == "commit_files":
			return &stepapi.ConnectorResult{
				Status: "success",
				Output: map[string]any{
					"commit_sha": "abc123",
					"changed":    false, // Idempotent commit - no changes
				},
			}, nil
		case req.Connector == "policy" && req.Operation == "evaluate":
			return &stepapi.ConnectorResult{
				Status: "success",
				Output: map[string]any{
					"allowed":    true,
					"violations": []interface{}{},
				},
			}, nil
		case req.Connector == "git" && req.Operation == "read_file":
			return &stepapi.ConnectorResult{
				Status: "success",
				Output: map[string]any{
					"content": "status: healthy\ncommit_sha: abc123\ntimestamp: 2026-03-19T14:40:45Z",
					"sha":     "def456",
				},
			}, nil
		case req.Connector == "webhook" && req.Operation == "post_callback":
			return &stepapi.ConnectorResult{
				Status: "success",
				Output: map[string]any{
					"status_code":   200,
					"response_body": `{"status":"success"}`,
				},
			}, nil
		default:
			return &stepapi.ConnectorResult{
				Status: "success",
				Output: map[string]any{},
			}, nil
		}
	}

	params := map[string]any{
		"idempotency_key":     "test-job-456",
		"job_id":              "job-456",
		"callback_url":        "https://example.com/callback",
		"infra_repo":          "org/infra-manifests",
		"contract_version":    "v1",
		"request_name":        "test-db-provision",
		"tenant":              "tenant-b",
		"owner":               "team-db",
		"environment":         "staging",
		"catalogue_item":      "vm-app",
		"namespace":           "tenant-b-staging",
		"workload_profile":    "small",
		"availability":        "standard",
		"residency":           "eu",
		"data_classification": "internal",
		"ingress_mode":        "private",
		"egress_mode":         "restricted",
		"cost_centre":         "team-db-001",
		"vm": map[string]any{
			"enabled":         true,
			"provider":        "aws",
			"count":           2,
			"instance_family": "general",
			"size":            "medium",
			"os_family":       "linux",
		},
		"primary_region": "eu-west-1",
		"cluster_name":   "test-db",
		"cloud_provider": "aws",
		"region":         "eu-west-1",
		"node_pools":     []any{},
		"networking": map[string]any{
			"vpc_cidr":     "10.1.0.0/16",
			"subnet_cidrs": []string{"10.1.1.0/24"},
		},
	}

	ctx := context.Background()
	err := module.Execute(ctx, mockStep, params)

	require.NoError(t, err)

	// Verify steps - should skip health polling when changed == false
	expectedSteps := []string{
		"infra.render",
		"infra.policy_evaluate",
		"infra.approval_gate",
		"infra.phase1_complete",
		"infra.git_commit",
		"infra.callback", // Should go directly to callback, skipping health_poll
	}

	assert.Equal(t, expectedSteps, mockStep.steps)

	// Verify no git.read_file call (health polling skipped)
	for _, call := range mockStep.connectorCalls {
		if call.Connector == "git" && call.Operation == "read_file" {
			t.Error("git.read_file should not be called for idempotent commit")
		}
	}
}

func TestModule_Phase2_HealthTimeoutRemediation(t *testing.T) {
	module := NewLegacyModule()
	mockStep := &MockStepAPI{
		approvalResult: &stepapi.ApprovalOutcome{
			Decision: "approved",
			Approver: "test-approver",
		},
	}

	readFileCallCount := 0
	// Set custom callConnector function
	mockStep.callConnector = func(ctx context.Context, req stepapi.ConnectorRequest) (*stepapi.ConnectorResult, error) {
		mockStep.connectorCalls = append(mockStep.connectorCalls, req)

		switch {
		case req.Connector == "git" && req.Operation == "read_file":
			readFileCallCount++
			// First call returns error (file not found), second call returns healthy status
			if readFileCallCount == 1 {
				return &stepapi.ConnectorResult{
					Status: "error",
					Error: &struct {
						Code    string `json:"code"`
						Message string `json:"message"`
					}{
						Code:    "FILE_NOT_FOUND",
						Message: "file not found",
					},
				}, nil
			}
			// Second call returns healthy status
			return &stepapi.ConnectorResult{
				Status: "success",
				Output: map[string]any{
					"content": "status: healthy\ncommit_sha: remediated-commit\ntimestamp: 2026-03-19T14:40:45Z",
					"sha":     "remediated-sha",
				},
			}, nil
		case req.Connector == "policy" && req.Operation == "evaluate":
			return &stepapi.ConnectorResult{
				Status: "success",
				Output: map[string]any{
					"allowed":    true,
					"violations": []interface{}{},
				},
			}, nil
		case req.Connector == "git" && req.Operation == "commit_files":
			return &stepapi.ConnectorResult{
				Status: "success",
				Output: map[string]any{
					"commit_sha": "original-commit",
					"changed":    true,
				},
			}, nil
		case req.Connector == "webhook" && req.Operation == "post_callback":
			return &stepapi.ConnectorResult{
				Status: "success",
				Output: map[string]any{
					"status_code":   200,
					"response_body": `{"status":"success"}`,
				},
			}, nil
		default:
			return &stepapi.ConnectorResult{
				Status: "success",
				Output: map[string]any{},
			}, nil
		}
	}

	params := map[string]any{
		"idempotency_key":     "test-job-789",
		"job_id":              "job-789",
		"callback_url":        "https://example.com/callback",
		"infra_repo":          "org/infra-manifests",
		"contract_version":    "v1",
		"request_name":        "test-vpc-provision",
		"tenant":              "tenant-c",
		"owner":               "team-networking",
		"environment":         "production",
		"catalogue_item":      "data-proc",
		"namespace":           "tenant-c-production",
		"workload_profile":    "large",
		"availability":        "high",
		"residency":           "us",
		"data_classification": "internal",
		"ingress_mode":        "private",
		"egress_mode":         "restricted",
		"cost_centre":         "team-networking-001",
		"kubernetes": map[string]any{
			"enabled":  true,
			"provider": "aws",
			"size":     "large",
		},
		"object_store": map[string]any{
			"enabled":  true,
			"provider": "aws",
			"class":    "standard",
		},
		"messaging": map[string]any{
			"enabled":     true,
			"provider":    "aws",
			"tier":        "standard",
			"queue_count": 1,
		},
		"primary_region": "us-east-1",
		"cluster_name":   "test-vpc",
		"cloud_provider": "aws",
		"region":         "us-east-1",
		"node_pools":     []any{},
		"networking": map[string]any{
			"vpc_cidr":     "10.2.0.0/16",
			"subnet_cidrs": []string{"10.2.1.0/24", "10.2.2.0/24", "10.2.3.0/24"},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	err := module.Execute(ctx, mockStep, params)

	// Should fail due to health timeout or cancellation
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "health polling")

	// Verify remediation was attempted
	var remediationFound bool
	for _, call := range mockStep.connectorCalls {
		if call.Connector == "git" && call.Operation == "commit_files" {
			if message, ok := call.Input["message"].(string); ok && strings.Contains(message, "[remediation]") {
				remediationFound = true
			}
		}
	}
	assert.True(t, remediationFound, "Remediation commit should have been attempted")
}
