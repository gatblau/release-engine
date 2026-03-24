// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package webhook

import (
	"context"
	"fmt"
	"sync"

	"github.com/gatblau/release-engine/internal/connector"
)

// MockWebhookConnector is a configurable mock for webhook connector testing.
type MockWebhookConnector struct {
	connector.BaseConnector
	config         connector.ConnectorConfig
	mu             sync.RWMutex
	closed         bool
	successAll     bool // if true, all callbacks succeed
	failAll        bool // if true, all callbacks fail
	customCallback func(ctx context.Context, input map[string]interface{}, secrets map[string][]byte) (*connector.ConnectorResult, error)
	callHistory    []map[string]interface{} // track calls for verification
}

// NewMockWebhookConnector creates a new mock webhook connector.
func NewMockWebhookConnector(cfg connector.ConnectorConfig) (*MockWebhookConnector, error) {
	base, err := connector.NewBaseConnector(connector.ConnectorTypeOther, "wmock")
	if err != nil {
		return nil, err
	}
	return &MockWebhookConnector{
		BaseConnector: base,
		config:        cfg,
		successAll:    true, // default to success
		callHistory:   []map[string]interface{}{},
	}, nil
}

// WithSuccessAll configures the mock to succeed all callbacks.
func (m *MockWebhookConnector) WithSuccessAll() *MockWebhookConnector {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.successAll = true
	m.failAll = false
	return m
}

// WithFailAll configures the mock to fail all callbacks.
func (m *MockWebhookConnector) WithFailAll() *MockWebhookConnector {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.successAll = false
	m.failAll = true
	return m
}

// WithCustomCallback configures a custom callback function.
func (m *MockWebhookConnector) WithCustomCallback(fn func(ctx context.Context, input map[string]interface{}, secrets map[string][]byte) (*connector.ConnectorResult, error)) *MockWebhookConnector {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.customCallback = fn
	m.successAll = false
	m.failAll = false
	return m
}

// GetCallHistory returns the history of calls made to the connector.
func (m *MockWebhookConnector) GetCallHistory() []map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.callHistory
}

// ClearCallHistory clears the call history.
func (m *MockWebhookConnector) ClearCallHistory() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callHistory = []map[string]interface{}{}
}

// Validate validates operation input.
func (m *MockWebhookConnector) Validate(operation string, input map[string]interface{}) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.closed {
		return fmt.Errorf("connector is closed")
	}

	if operation != "post_callback" {
		return fmt.Errorf("unknown operation: %s", operation)
	}

	// Required fields per docs/plan.md
	requiredFields := []string{"url", "headers", "body", "idempotency_key"}
	for _, field := range requiredFields {
		if _, ok := input[field]; !ok {
			return fmt.Errorf("missing required field: %s", field)
		}
	}

	// headers must be a map[string]string
	if headers, ok := input["headers"].(map[string]interface{}); ok {
		// Validate each header value is string
		for k, v := range headers {
			if _, ok := v.(string); !ok {
				return fmt.Errorf("header value for %s must be a string", k)
			}
		}
	} else {
		return fmt.Errorf("headers must be a map[string]string")
	}

	// body can be any object
	if _, ok := input["body"]; !ok {
		return fmt.Errorf("body is required")
	}

	return nil
}

// Execute performs webhook callback.
func (m *MockWebhookConnector) Execute(ctx context.Context, operation string, input map[string]interface{}, secrets map[string][]byte) (*connector.ConnectorResult, error) {
	m.mu.RLock()
	if m.closed {
		m.mu.RUnlock()
		return nil, fmt.Errorf("connector is closed")
	}
	m.mu.RUnlock()

	if operation != "post_callback" {
		return nil, fmt.Errorf("operation not implemented: %s", operation)
	}

	// Record the call
	m.mu.Lock()
	m.callHistory = append(m.callHistory, input)
	m.mu.Unlock()

	if m.customCallback != nil {
		return m.customCallback(ctx, input, secrets)
	}

	if m.failAll {
		return &connector.ConnectorResult{
			Status: connector.StatusTerminalError,
			Error: &connector.ConnectorError{
				Code:    "MOCK_FAILURE",
				Message: "Mock webhook connector configured to fail all",
			},
		}, nil
	}

	// Default: success
	return &connector.ConnectorResult{
		Status: connector.StatusSuccess,
		Output: map[string]interface{}{
			"status_code":   200,
			"response_body": `{"status":"success","mock":true}`,
		},
	}, nil
}

// Close closes the connector.
func (m *MockWebhookConnector) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

// Operations returns the list of supported operations.
func (m *MockWebhookConnector) Operations() []connector.OperationMeta {
	return []connector.OperationMeta{
		{
			Name:           "post_callback",
			Description:    "Post callback to webhook URL",
			RequiredFields: []string{"url", "headers", "body", "idempotency_key"},
			OptionalFields: []string{},
			IsAsync:        false,
		},
	}
}
