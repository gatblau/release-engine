// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package connector

import (
	"fmt"
	"strings"
	"sync"
)

// familyRegistry implements FamilyRegistry by wrapping a ConnectorRegistry.
type familyRegistry struct {
	mu       sync.RWMutex
	registry ConnectorRegistry          // underlying concrete registry
	families map[string]ConnectorFamily // family name → contract
	bindings map[string]string          // family name → connector key
}

// NewFamilyRegistry creates a new FamilyRegistry wrapping the given ConnectorRegistry.
func NewFamilyRegistry(registry ConnectorRegistry) FamilyRegistry {
	return &familyRegistry{
		registry: registry,
		families: make(map[string]ConnectorFamily),
		bindings: make(map[string]string),
	}
}

// RegisterFamily registers a family contract.
func (r *familyRegistry) RegisterFamily(family ConnectorFamily) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if family.Name == "" {
		return fmt.Errorf("family name cannot be empty")
	}
	if _, exists := r.families[family.Name]; exists {
		return fmt.Errorf("family already registered: %s", family.Name)
	}
	if family.Operations == nil {
		family.Operations = make(map[string]OperationContract)
	}
	r.families[family.Name] = family
	return nil
}

// BindImplementation binds a concrete connector to a family.
func (r *familyRegistry) BindImplementation(familyName, connectorKey string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.families[familyName]; !exists {
		return fmt.Errorf("family not registered: %s", familyName)
	}

	// Verify connector exists
	_, ok := r.registry.Lookup(connectorKey)
	if !ok {
		return fmt.Errorf("connector not found: %s", connectorKey)
	}

	r.bindings[familyName] = connectorKey
	return nil
}

// Resolve returns the concrete connector for a family.
func (r *familyRegistry) Resolve(familyName string) (Connector, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	connectorKey, bound := r.bindings[familyName]
	if !bound {
		return nil, fmt.Errorf("no implementation bound for family: %s", familyName)
	}

	conn, ok := r.registry.Lookup(connectorKey)
	if !ok {
		return nil, fmt.Errorf("bound connector not found: %s (for family %s)", connectorKey, familyName)
	}

	return conn, nil
}

// ValidateBindings validates all family bindings and their contracts.
func (r *familyRegistry) ValidateBindings() error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var errors []string

	// Check all families have bindings
	for familyName := range r.families {
		if _, bound := r.bindings[familyName]; !bound {
			errors = append(errors, fmt.Sprintf("family %s has no bound implementation", familyName))
			continue
		}

		connectorKey := r.bindings[familyName]
		conn, ok := r.registry.Lookup(connectorKey)
		if !ok {
			errors = append(errors, fmt.Sprintf("family %s bound to non-existent connector: %s", familyName, connectorKey))
			continue
		}

		// Check operation contract compliance if connector implements OperationDescriber
		if describer, ok := conn.(OperationDescriber); ok {
			family := r.families[familyName]
			if err := r.validateContractCompliance(family, describer); err != nil {
				errors = append(errors, fmt.Sprintf("family %s contract violation: %v", familyName, err))
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("validation failed:\n  %s", strings.Join(errors, "\n  "))
	}
	return nil
}

// validateContractCompliance checks that a connector supports all required operations.
func (r *familyRegistry) validateContractCompliance(family ConnectorFamily, describer OperationDescriber) error {
	supportedOps := make(map[string]bool)
	for _, op := range describer.Operations() {
		supportedOps[op.Name] = true
	}

	for opName := range family.Operations {
		if !supportedOps[opName] {
			return fmt.Errorf("operation %s not supported by connector", opName)
		}
	}

	return nil
}

// GetFamilies returns all registered families (for testing/inspection).
func (r *familyRegistry) GetFamilies() map[string]ConnectorFamily {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]ConnectorFamily)
	for k, v := range r.families {
		result[k] = v
	}
	return result
}

// GetBindings returns all family bindings (for testing/inspection).
func (r *familyRegistry) GetBindings() map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]string)
	for k, v := range r.bindings {
		result[k] = v
	}
	return result
}
