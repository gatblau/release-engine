// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package registry

import (
	"context"
	"fmt"
	"sync"
)

// Module defines the interface for an executable module.
type Module interface {
	Key() string
	Version() string
	Execute(ctx context.Context, api any, params map[string]any) error
	Query(ctx context.Context, api any, req QueryRequest) (QueryResult, error)
	Describe() ModuleDescriptor
}

// ModuleRegistry handles module registration and lookup.
type ModuleRegistry interface {
	Register(m Module) error
	Lookup(key, version string) (Module, bool)
	ListModules() []ModuleDescriptor
}

type moduleRegistry struct {
	mu      sync.RWMutex
	modules map[string]Module
}

// NewModuleRegistry creates a new module registry.
func NewModuleRegistry() ModuleRegistry {
	return &moduleRegistry{
		modules: make(map[string]Module),
	}
}

func (r *moduleRegistry) Register(m Module) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := fmt.Sprintf("%s:%s", m.Key(), m.Version())
	if _, exists := r.modules[id]; exists {
		return &RegistryError{Err: ErrModuleDuplicate, Code: "MODULE_DUPLICATE", Detail: map[string]string{"id": id}}
	}
	r.modules[id] = m
	return nil
}

func (r *moduleRegistry) Lookup(key, version string) (Module, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.modules[fmt.Sprintf("%s:%s", key, version)]
	return m, ok
}

func (r *moduleRegistry) ListModules() []ModuleDescriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()

	descriptors := make([]ModuleDescriptor, 0, len(r.modules))
	for _, module := range r.modules {
		descriptors = append(descriptors, module.Describe())
	}
	return descriptors
}
