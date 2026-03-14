// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package connector

import (
	"fmt"
	"strings"
	"sync"
)

type connectorRegistry struct {
	mu         sync.RWMutex
	connectors map[string]Connector
	closed     bool
}

// NewConnectorRegistry creates a new ConnectorRegistry.
func NewConnectorRegistry() ConnectorRegistry {
	return &connectorRegistry{
		connectors: make(map[string]Connector),
	}
}

func (r *connectorRegistry) Register(connector Connector) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return fmt.Errorf("registry closed")
	}
	if _, exists := r.connectors[connector.Key()]; exists {
		return fmt.Errorf("connector already registered: %s", connector.Key())
	}
	r.connectors[connector.Key()] = connector
	return nil
}

func (r *connectorRegistry) Replace(connector Connector) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return fmt.Errorf("registry closed")
	}
	if _, exists := r.connectors[connector.Key()]; !exists {
		return fmt.Errorf("connector not found: %s", connector.Key())
	}
	r.connectors[connector.Key()] = connector
	return nil
}

func (r *connectorRegistry) Lookup(key string) (Connector, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.connectors[key]
	return c, ok
}

func (r *connectorRegistry) ListByType(t ConnectorType) []Connector {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var res []Connector
	for _, c := range r.connectors {
		if ct, ok := c.(interface{ Type() ConnectorType }); ok {
			if ct.Type() == t {
				res = append(res, c)
			}
		}
	}
	return res
}

func (r *connectorRegistry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil
	}
	r.closed = true
	var errs []string
	for _, c := range r.connectors {
		if err := c.Close(); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors closing connectors: %s", strings.Join(errs, "; "))
	}
	return nil
}
