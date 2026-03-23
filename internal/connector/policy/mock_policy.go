// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package policy

import (
	"context"
	"fmt"
	"sync"

	"github.com/gatblau/release-engine/internal/connector"
)

// MockPolicyConnector is a configurable mock for policy connector testing.
type MockPolicyConnector struct {
	connector.BaseConnector
	config     connector.ConnectorConfig
	mu         sync.RWMutex
	closed     bool
	allowAll   bool // if true, all evaluations pass
	denyAll    bool // if true, all evaluations fail
	customEval func(ctx context.Context, input map[string]interface{}) (*connector.ConnectorResult, error)
}

// NewMockPolicyConnector creates a new mock policy connector.
func NewMockPolicyConnector(cfg connector.ConnectorConfig) (*MockPolicyConnector, error) {
	base, err := connector.NewBaseConnector(connector.ConnectorTypeOther, "pmock")
	if err != nil {
		return nil, err
	}
	return &MockPolicyConnector{
		BaseConnector: base,
		config:        cfg,
		allowAll:      true, // default to allowing
	}, nil
}

// WithAllowAll configures the mock to allow all evaluations.
func (m *MockPolicyConnector) WithAllowAll() *MockPolicyConnector {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.allowAll = true
	m.denyAll = false
	return m
}

// WithDenyAll configures the mock to deny all evaluations.
func (m *MockPolicyConnector) WithDenyAll() *MockPolicyConnector {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.allowAll = false
	m.denyAll = true
	return m
}

// WithCustomEval configures a custom evaluation function.
func (m *MockPolicyConnector) WithCustomEval(fn func(ctx context.Context, input map[string]interface{}) (*connector.ConnectorResult, error)) *MockPolicyConnector {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.customEval = fn
	m.allowAll = false
	m.denyAll = false
	return m
}

// Validate validates operation input.
func (m *MockPolicyConnector) Validate(operation string, input map[string]interface{}) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.closed {
		return fmt.Errorf("connector is closed")
	}

	if operation != "evaluate" {
		return fmt.Errorf("unknown operation: %s", operation)
	}

	// Required fields per docs/plan.md
	requiredFields := []string{"policy_bundle", "resource"}
	for _, field := range requiredFields {
		if _, ok := input[field]; !ok {
			return fmt.Errorf("missing required field: %s", field)
		}
	}

	return nil
}

// Execute performs policy evaluation.
func (m *MockPolicyConnector) Execute(ctx context.Context, operation string, input map[string]interface{}) (*connector.ConnectorResult, error) {
	m.mu.RLock()
	if m.closed {
		m.mu.RUnlock()
		return nil, fmt.Errorf("connector is closed")
	}
	m.mu.RUnlock()

	if operation != "evaluate" {
		return nil, fmt.Errorf("operation not implemented: %s", operation)
	}

	if m.customEval != nil {
		return m.customEval(ctx, input)
	}

	if m.denyAll {
		return &connector.ConnectorResult{
			Status: connector.StatusSuccess,
			Output: map[string]interface{}{
				"allowed": false,
				"violations": []interface{}{
					map[string]interface{}{
						"rule":     "mock-deny-rule",
						"message":  "Mock policy connector configured to deny all",
						"severity": "error",
					},
				},
			},
		}, nil
	}

	// Default: allow
	return &connector.ConnectorResult{
		Status: connector.StatusSuccess,
		Output: map[string]interface{}{
			"allowed":    true,
			"violations": []interface{}{},
		},
	}, nil
}

// Close closes the connector.
func (m *MockPolicyConnector) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

// Operations returns the list of supported operations.
func (m *MockPolicyConnector) Operations() []connector.OperationMeta {
	return []connector.OperationMeta{
		{
			Name:           "evaluate",
			Description:    "Evaluate resources against policy bundle",
			RequiredFields: []string{"policy_bundle", "resource"},
			OptionalFields: []string{"context"},
			IsAsync:        false,
		},
	}
}
