// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package registry

import (
	"sync"
)

// Connector defines the interface for an external connector.
type Connector interface {
	Key() string
}

// ConnectorRegistry handles connector registration and lookup.
type ConnectorRegistry interface {
	Register(c Connector) error
	Lookup(key string) (Connector, bool)
}

type connectorRegistry struct {
	mu         sync.RWMutex
	connectors map[string]Connector
}

// NewConnectorRegistry creates a new connector registry.
func NewConnectorRegistry() ConnectorRegistry {
	return &connectorRegistry{
		connectors: make(map[string]Connector),
	}
}

func (r *connectorRegistry) Register(c Connector) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.connectors[c.Key()]; exists {
		return &RegistryError{Err: ErrConnectorDuplicate, Code: "CONNECTOR_DUPLICATE", Detail: map[string]string{"key": c.Key()}}
	}
	r.connectors[c.Key()] = c
	return nil
}

func (r *connectorRegistry) Lookup(key string) (Connector, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.connectors[key]
	return c, ok
}
