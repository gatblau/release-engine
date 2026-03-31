// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package connector

import (
	"fmt"
)

// SetupFamilyRegistry creates and configures a complete family registry.
// In the new design, connector resolution happens per-module at assembly time,
// so this function only registers the families with their valid members.
// Configuration is applied by each module's config file (connectors.families map).
func SetupFamilyRegistry(concreteRegistry ConnectorRegistry, configPath string) (FamilyRegistry, error) {
	// Create family registry
	familyReg := NewFamilyRegistry(concreteRegistry)

	// Register default families with their valid members
	for _, family := range DefaultFamilies() {
		if err := familyReg.RegisterFamily(family); err != nil {
			return nil, fmt.Errorf("failed to register family %s: %w", family.Name, err)
		}
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
