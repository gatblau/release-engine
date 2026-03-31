// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package policy

import (
	"context"
	"fmt"
	"sync"

	"github.com/gatblau/release-engine/internal/connector"
)

// PolicyConnector implements the policy connector contract from docs/plan.md
type PolicyConnector struct {
	connector.BaseConnector
	config connector.ConnectorConfig
	mu     sync.RWMutex
	closed bool
}

// NewPolicyConnector creates a new policy connector.
func NewPolicyConnector(cfg connector.ConnectorConfig) (*PolicyConnector, error) {
	base, err := connector.NewBaseConnector(connector.ConnectorTypePolicy, "embedded")
	if err != nil {
		return nil, err
	}
	return &PolicyConnector{
		BaseConnector: base,
		config:        cfg,
	}, nil
}

// Validate validates operation input.
func (c *PolicyConnector) Validate(operation string, input map[string]interface{}) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
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

	// resource must be a map or object
	if resource, ok := input["resource"]; ok {
		if _, isMap := resource.(map[string]interface{}); !isMap {
			// Could also be any JSON-like object; we'll accept any for now
			fmt.Printf("resource is not a map: %v", resource)
		}
	}

	return nil
}

// RequiredSecrets returns the secrets required for policy operations.
func (c *PolicyConnector) RequiredSecrets(operation string) []string {
	// Policy connector doesn't require any secrets
	return []string{}
}

// Execute performs policy evaluation.
func (c *PolicyConnector) Execute(ctx context.Context, operation string, input map[string]interface{}, secrets map[string][]byte) (*connector.ConnectorResult, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return nil, fmt.Errorf("connector is closed")
	}
	c.mu.RUnlock()

	if operation != "evaluate" {
		return nil, fmt.Errorf("operation not implemented: %s", operation)
	}

	// This is a base implementation that always allows.
	// MockPolicyConnector will override this behavior for testing.
	return c.evaluate(ctx, input)
}

// evaluate performs the actual policy evaluation.
func (c *PolicyConnector) evaluate(ctx context.Context, input map[string]interface{}) (*connector.ConnectorResult, error) {
	// Base implementation: always allow
	return &connector.ConnectorResult{
		Status: connector.StatusSuccess,
		Output: map[string]interface{}{
			"allowed":    true,
			"violations": []interface{}{},
		},
	}, nil
}

// Close closes the connector.
func (c *PolicyConnector) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}

// Operations returns the list of supported operations.
func (c *PolicyConnector) Operations() []connector.OperationMeta {
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
