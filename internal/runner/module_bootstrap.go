package runner

import (
	"github.com/gatblau/release-engine/internal/module/infra"
	"github.com/gatblau/release-engine/internal/registry"
)

// NewDefaultModuleRegistry builds a module registry with built-in modules.
func NewDefaultModuleRegistry() (registry.ModuleRegistry, error) {
	reg := registry.NewModuleRegistry()
	if err := infra.Register(reg); err != nil {
		return nil, err
	}
	return reg, nil
}
