// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package connector

import (
	"fmt"
)

// SetupFamilyRegistry creates and configures a complete family registry.
// It registers default families, loads configuration, applies bindings, and validates.
func SetupFamilyRegistry(concreteRegistry ConnectorRegistry, configPath string) (FamilyRegistry, error) {
	// Create family registry
	familyReg := NewFamilyRegistry(concreteRegistry)

	// Register default families
	for _, family := range DefaultFamilies() {
		if err := familyReg.RegisterFamily(family); err != nil {
			return nil, fmt.Errorf("failed to register family %s: %w", family.Name, err)
		}
	}

	// Load configuration
	config, err := LoadFamilyConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load family config: %w", err)
	}

	// Also load from environment (env vars take precedence)
	envConfig, err := LoadFamilyConfigFromEnv()
	if err != nil {
		return nil, fmt.Errorf("failed to load family config from env: %w", err)
	}

	// Merge configs (environment overrides file)
	for family, connectorKey := range envConfig.Families {
		config.Families[family] = connectorKey
	}

	// Apply bindings
	if err := config.ApplyBindings(familyReg); err != nil {
		return nil, fmt.Errorf("failed to apply family bindings: %w", err)
	}

	// Validate bindings
	if err := familyReg.ValidateBindings(); err != nil {
		return nil, fmt.Errorf("family binding validation failed: %w", err)
	}

	return familyReg, nil
}

// MustSetupFamilyRegistry calls SetupFamilyRegistry and panics on error.
// Use this in application startup where configuration errors should be fatal.
func MustSetupFamilyRegistry(concreteRegistry ConnectorRegistry, configPath string) FamilyRegistry {
	registry, err := SetupFamilyRegistry(concreteRegistry, configPath)
	if err != nil {
		panic(fmt.Sprintf("failed to setup family registry: %v", err))
	}
	return registry
}
