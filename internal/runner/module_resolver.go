// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package runner

import (
	"context"
	"fmt"

	"github.com/gatblau/release-engine/internal/connector"
	"github.com/gatblau/release-engine/internal/module"
	"github.com/gatblau/release-engine/internal/module/config"
	"github.com/gatblau/release-engine/internal/registry"
)

// Resolver integrates config-aware module assembly into the framework bootstrap.
type Resolver struct {
	factory        *module.Factory
	legacyRegistry registry.ModuleRegistry
}

// NewResolver creates a new module resolver.
func NewResolver(configLoader config.Loader, connRegistry connector.TypedConnectorRegistry, legacyRegistry registry.ModuleRegistry) *Resolver {
	factory := module.NewFactory(configLoader, connRegistry)
	return &Resolver{
		factory:        factory,
		legacyRegistry: legacyRegistry,
	}
}

// ResolveModule resolves a module using the appropriate assembly path.
// This implements the migration strategy described in phase 4 of module-cfg-impl-plan.md.
func (r *Resolver) ResolveModule(ctx context.Context, moduleKey, moduleVersion string) (registry.Module, error) {
	// Extract module name from module key (e.g., "infra.provision" -> "infra")
	moduleName := extractModuleName(moduleKey)

	// Determine which assembly path to use
	if module.IsConfigManagedModule(moduleName) {
		// Config-managed module: use new assembly path with fail-fast behavior
		return r.resolveConfigManagedModule(ctx, moduleName)
	} else {
		// Legacy module: use old assembly path
		return r.resolveLegacyModule(moduleKey, moduleVersion)
	}
}

// resolveConfigManagedModule resolves a config-managed module using the framework-driven path.
func (r *Resolver) resolveConfigManagedModule(ctx context.Context, moduleName string) (registry.Module, error) {
	module, err := r.factory.AssembleConfigManagedModule(ctx, moduleName)
	if err != nil {
		return nil, fmt.Errorf("failed to assemble config-managed module %s: %w", moduleName, err)
	}
	return module, nil
}

// resolveLegacyModule resolves a legacy module using the existing registry.
func (r *Resolver) resolveLegacyModule(moduleKey, moduleVersion string) (registry.Module, error) {
	module, ok := r.legacyRegistry.Lookup(moduleKey, moduleVersion)
	if !ok {
		return nil, fmt.Errorf("legacy module not found: %s:%s", moduleKey, moduleVersion)
	}
	return module, nil
}

// extractModuleName extracts the module name from a module key.
// Example: "infra.provision" -> "infra", "billing.calculate" -> "billing"
func extractModuleName(moduleKey string) string {
	// Split by dot and take the first part
	for i, ch := range moduleKey {
		if ch == '.' {
			return moduleKey[:i]
		}
	}
	return moduleKey
}
