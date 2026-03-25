// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package testing

import (
	"context"
	"fmt"
	"sync"

	"github.com/gatblau/release-engine/internal/connector"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

// CrossplaneMockConnector is a mock implementation of the crossplane connector for testing.
type CrossplaneMockConnector struct {
	connector.BaseConnector
	mu     sync.RWMutex
	closed bool
	// In-memory store for mock resources
	resources map[string]*unstructured.Unstructured
}

// NewCrossplaneMockConnector creates a new mock crossplane connector.
func NewCrossplaneMockConnector() (*CrossplaneMockConnector, error) {
	base, err := connector.NewBaseConnector(connector.ConnectorTypeInfra, "crossplaneMock")
	if err != nil {
		return nil, err
	}
	return &CrossplaneMockConnector{
		BaseConnector: base,
		resources:     make(map[string]*unstructured.Unstructured),
	}, nil
}

func (c *CrossplaneMockConnector) Validate(operation string, input map[string]interface{}) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
		return fmt.Errorf("connector is closed")
	}

	requiredFields := map[string][]string{
		"create_composite_resource": {"kind", "name", "manifest"},
		"get_resource_status":       {"kind", "name"},
		"delete_resource":           {"kind", "name"},
	}

	fields, ok := requiredFields[operation]
	if !ok {
		return fmt.Errorf("unknown operation: %s", operation)
	}

	for _, field := range fields {
		if _, ok := input[field]; !ok {
			return fmt.Errorf("missing required field: %s", field)
		}
	}

	return nil
}

func (c *CrossplaneMockConnector) RequiredSecrets(operation string) []string {
	// Crossplane mock doesn't require any secrets
	return []string{}
}

func (c *CrossplaneMockConnector) Execute(ctx context.Context, operation string, input map[string]interface{}, secrets map[string][]byte) (*connector.ConnectorResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return nil, fmt.Errorf("connector is closed")
	}
	c.mu.RUnlock()

	switch operation {
	case "create_composite_resource":
		return c.createCompositeResource(ctx, input)
	case "get_resource_status":
		return c.getResourceStatus(ctx, input)
	case "delete_resource":
		return c.deleteResource(ctx, input)
	default:
		return nil, fmt.Errorf("operation not implemented: %s", operation)
	}
}

func (c *CrossplaneMockConnector) createCompositeResource(ctx context.Context, input map[string]interface{}) (*connector.ConnectorResult, error) {
	kind := input["kind"].(string)
	name := input["name"].(string)

	// Parse manifest
	var obj map[string]interface{}
	switch v := input["manifest"].(type) {
	case string:
		// For simplicity, we'll just create a basic object
		obj = map[string]interface{}{
			"apiVersion": "pkg.crossplane.io/v1",
			"kind":       kind,
			"metadata": map[string]interface{}{
				"name": name,
			},
			"spec": map[string]interface{}{
				"parameters": input,
			},
		}
	case map[string]interface{}:
		obj = v
	default:
		return nil, fmt.Errorf("manifest must be a string or map[string]interface{}")
	}

	unstr := &unstructured.Unstructured{Object: obj}
	unstr.SetName(name)

	// Store in memory
	key := fmt.Sprintf("%s/%s", kind, name)
	c.mu.Lock()
	c.resources[key] = unstr
	c.mu.Unlock()

	return &connector.ConnectorResult{
		Status: connector.StatusSuccess,
		Output: map[string]interface{}{
			"name": name,
			"uid":  fmt.Sprintf("mock-uid-%s", name),
		},
	}, nil
}

func (c *CrossplaneMockConnector) getResourceStatus(ctx context.Context, input map[string]interface{}) (*connector.ConnectorResult, error) {
	kind := input["kind"].(string)
	name := input["name"].(string)

	key := fmt.Sprintf("%s/%s", kind, name)
	c.mu.RLock()
	_, exists := c.resources[key]
	c.mu.RUnlock()

	if !exists {
		return &connector.ConnectorResult{
			Status: connector.StatusSuccess,
			Output: map[string]interface{}{
				"status": map[string]interface{}{
					"health": "not-found",
				},
			},
		}, nil
	}

	// Return mock status
	return &connector.ConnectorResult{
		Status: connector.StatusSuccess,
		Output: map[string]interface{}{
			"status": map[string]interface{}{
				"health": "healthy",
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Ready",
						"status": "True",
						"reason": "ResourceAvailable",
					},
				},
			},
		},
	}, nil
}

func (c *CrossplaneMockConnector) deleteResource(ctx context.Context, input map[string]interface{}) (*connector.ConnectorResult, error) {
	kind := input["kind"].(string)
	name := input["name"].(string)

	key := fmt.Sprintf("%s/%s", kind, name)
	c.mu.Lock()
	delete(c.resources, key)
	c.mu.Unlock()

	return &connector.ConnectorResult{
		Status: connector.StatusSuccess,
	}, nil
}

func (c *CrossplaneMockConnector) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	c.resources = make(map[string]*unstructured.Unstructured)
	return nil
}

func (c *CrossplaneMockConnector) Operations() []connector.OperationMeta {
	return []connector.OperationMeta{
		{Name: "create_composite_resource", IsAsync: true},
		{Name: "get_resource_status", IsAsync: false},
		{Name: "delete_resource", IsAsync: false},
	}
}

// GetDynamicClient returns a mock dynamic client (not used by mock, but required for interface compatibility)
func (c *CrossplaneMockConnector) GetDynamicClient(kubeconfig []byte) (dynamic.Interface, error) {
	return nil, fmt.Errorf("mock does not implement dynamic client")
}
