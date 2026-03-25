// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package module

import (
	"context"
	"fmt"

	"github.com/gatblau/release-engine/internal/connector"
	"github.com/gatblau/release-engine/internal/module/config"
	"github.com/gatblau/release-engine/internal/module/infra"
	"github.com/gatblau/release-engine/internal/registry"
)

// Factory provides config-aware assembly entry points for modules.
type Factory struct {
	configLoader config.Loader
	connRegistry connector.TypedConnectorRegistry
}

// NewFactory creates a new module factory.
func NewFactory(configLoader config.Loader, connRegistry connector.TypedConnectorRegistry) *Factory {
	return &Factory{
		configLoader: configLoader,
		connRegistry: connRegistry,
	}
}

// AssembleConfigManagedModule assembles a config-managed module using the framework-driven path.
// This implements the resolution flow described in phase 4 of module-cfg-impl-plan.md.
func (f *Factory) AssembleConfigManagedModule(ctx context.Context, moduleName string) (registry.Module, error) {
	// 1. Load raw config file
	rawConfig, err := f.configLoader.Load(ctx, moduleName)
	if err != nil {
		return nil, fmt.Errorf("failed to load config for module %s: %w", moduleName, err)
	}

	// 2. Module-specific assembly based on module name
	switch moduleName {
	case "infra":
		return f.assembleInfraModule(ctx, rawConfig)
	// Add other config-managed modules here as they are migrated
	default:
		return nil, fmt.Errorf("module %s is not config-managed or not supported", moduleName)
	}
}

// assembleInfraModule assembles the infra module using config-managed assembly.
func (f *Factory) assembleInfraModule(ctx context.Context, rawConfig *config.ModuleConfigFile) (registry.Module, error) {
	// 3. Invoke module-owned typed parser
	typedConfig, err := infra.ParseConfig(rawConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to parse infra module config: %w", err)
	}

	// 4. Validate required connector families are present (handled by ParseConfig)

	// 5. Resolve selected connector implementations from registry
	gitConn, err := f.connRegistry.ResolveGit(typedConfig.Connectors.Git)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve git connector %s: %w", typedConfig.Connectors.Git, err)
	}

	// Crossplane connectors use the generic Connector interface
	// Crossplane connectors are registered with "infra-" prefix (ConnectorTypeInfra)
	crossplaneKey := typedConfig.Connectors.Crossplane
	if !isPrefixedWithInfra(crossplaneKey) {
		crossplaneKey = "infra-" + crossplaneKey
	}
	crossplaneConn, ok := f.connRegistry.Lookup(crossplaneKey)
	if !ok {
		return nil, fmt.Errorf("failed to resolve crossplane connector %s (looked up as %s): not found", typedConfig.Connectors.Crossplane, crossplaneKey)
	}

	policyConn, err := f.connRegistry.ResolvePolicy(typedConfig.Connectors.Policy)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve policy connector %s: %w", typedConfig.Connectors.Policy, err)
	}

	webhookConn, err := f.connRegistry.ResolveWebhook(typedConfig.Connectors.Webhook)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve webhook connector %s: %w", typedConfig.Connectors.Webhook, err)
	}

	// 6. Call module constructor with typed config and typed connectors
	module, err := infra.NewModule(typedConfig.Vars, gitConn, crossplaneConn, policyConn, webhookConn)
	if err != nil {
		return nil, fmt.Errorf("failed to create infra module: %w", err)
	}

	return module, nil
}

// IsConfigManagedModule returns true if the module is expected to be config-managed.
// This helps the framework decide which assembly path to use during migration.
func IsConfigManagedModule(moduleName string) bool {
	configManagedModules := map[string]bool{
		"infra": true,
		// Add other config-managed modules here as they are migrated
	}
	return configManagedModules[moduleName]
}

// isPrefixedWithInfra checks if a key already has the "infra-" prefix
func isPrefixedWithInfra(key string) bool {
	return len(key) > 6 && key[:6] == "infra-"
}
