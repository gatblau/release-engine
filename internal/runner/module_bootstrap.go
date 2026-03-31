package runner

import (
	"context"
	"fmt"
	"os"

	"github.com/gatblau/release-engine/internal/connector"
	"github.com/gatblau/release-engine/internal/module/config"
	"github.com/gatblau/release-engine/internal/module/infra"
	"github.com/gatblau/release-engine/internal/registry"
)

// NewDefaultModuleRegistry builds a module registry with built-in modules.
// This is the legacy assembly path that will be used for non-config-managed modules.
func NewDefaultModuleRegistry() (registry.ModuleRegistry, error) {
	reg := registry.NewModuleRegistry()
	if err := infra.Register(reg); err != nil {
		return nil, err
	}
	return reg, nil
}

// NewConfigAwareModuleResolver creates a module resolver that supports both
// config-managed and legacy module assembly paths.
func NewConfigAwareModuleResolver(connRegistry connector.TypedConnectorRegistry, familyReg connector.FamilyRegistry) (*Resolver, error) {
	// Create config loader with default base path
	defaultBasePath := "."
	if cfgRoot := os.Getenv("CFG_ROOT"); cfgRoot != "" {
		defaultBasePath = cfgRoot
	}
	configLoader := config.NewLoader(defaultBasePath)

	// Create legacy registry for backward compatibility
	legacyRegistry, err := NewDefaultModuleRegistry()
	if err != nil {
		return nil, err
	}

	return NewResolver(configLoader, connRegistry, familyReg, legacyRegistry), nil
}

// BootstrapWithConfig loads and assembles modules using config-aware resolution.
// This is the new entry point for framework bootstrap that supports config-managed modules.
func BootstrapWithConfig(ctx context.Context, connRegistry connector.TypedConnectorRegistry, familyReg connector.FamilyRegistry) (registry.ModuleRegistry, error) {
	resolver, err := NewConfigAwareModuleResolver(connRegistry, familyReg)
	if err != nil {
		return nil, err
	}

	// Create a dynamic registry that uses the resolver
	return &dynamicModuleRegistry{
		resolver: resolver,
		modules:  make(map[string]registry.Module),
	}, nil
}

// dynamicModuleRegistry implements registry.ModuleRegistry but resolves modules
// on-demand using the config-aware resolver.
type dynamicModuleRegistry struct {
	resolver *Resolver
	modules  map[string]registry.Module
}

func (r *dynamicModuleRegistry) Register(m registry.Module) error {
	// For dynamic registry, we don't support direct registration
	// Modules are resolved on-demand via ResolveModule
	return nil
}

func (r *dynamicModuleRegistry) Lookup(key, version string) (registry.Module, bool) {
	// Try to resolve the module using the resolver
	module, err := r.resolver.ResolveModule(context.Background(), key, version)
	if err != nil {
		fmt.Printf("[ERROR] failed to resolve module %s:%s: %v\n", key, version, err)
		return nil, false
	}

	// Cache the resolved module
	cacheKey := key + ":" + version
	r.modules[cacheKey] = module

	return module, true
}

func (r *dynamicModuleRegistry) ListModules() []registry.ModuleDescriptor {
	// For dynamic registry, we need to list modules that can be resolved
	// This is a simplified implementation that returns descriptors for known modules
	descriptors := []registry.ModuleDescriptor{}

	// Try to resolve known modules and get their descriptors
	knownModules := []struct {
		key     string
		version string
	}{
		{"infra.provision", "latest"},
		{"scaffolding/create-service", "1.0.0"},
	}

	for _, km := range knownModules {
		if module, ok := r.Lookup(km.key, km.version); ok {
			descriptors = append(descriptors, module.Describe())
		}
	}

	return descriptors
}
