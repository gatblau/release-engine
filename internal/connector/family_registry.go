// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package connector

import (
	"fmt"
	"sync"
)

// familyRegistry implements FamilyRegistry by wrapping a ConnectorRegistry.
type familyRegistry struct {
	mu       sync.RWMutex
	registry ConnectorRegistry          // underlying concrete registry
	families map[string]ConnectorFamily // family name → contract
}

// NewFamilyRegistry creates a new FamilyRegistry wrapping the given ConnectorRegistry.
func NewFamilyRegistry(registry ConnectorRegistry) FamilyRegistry {
	return &familyRegistry{
		registry: registry,
		families: make(map[string]ConnectorFamily),
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

// Resolve returns the concrete connector for a family using the specified implementation key.
// It validates that the implementation key is a valid member of the family before returning.
// The implKey can be either the full connector key (e.g., "git-github") or a simple name (e.g., "github").
// If a simple name is provided, it will be prefixed with "family-" (e.g., "git-" for the git family).
func (r *familyRegistry) Resolve(family, implKey string) (Connector, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	f, ok := r.families[family]
	if !ok {
		return nil, fmt.Errorf("unknown family: %s", family)
	}

	// Try the provided key first
	if f.HasMember(implKey) {
		conn, ok := r.registry.Lookup(implKey)
		if !ok {
			return nil, fmt.Errorf("connector not found: %s", implKey)
		}
		return conn, nil
	}

	// If the key is not a member, try prefixing it with "family-"
	prefixedKey := family + "-" + implKey
	if f.HasMember(prefixedKey) {
		conn, ok := r.registry.Lookup(prefixedKey)
		if !ok {
			return nil, fmt.Errorf("connector not found: %s (prefixed as %s)", implKey, prefixedKey)
		}
		return conn, nil
	}

	return nil, fmt.Errorf("%s is not a member of family %s (tried %s and %s)", implKey, family, implKey, prefixedKey)
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
