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

// NewTypedConnectorRegistry creates a new TypedConnectorRegistry.
func NewTypedConnectorRegistry() TypedConnectorRegistry {
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

// ResolveGit resolves a git connector by implementation name.
func (r *connectorRegistry) ResolveGit(name string) (GitConnector, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Handle both "github" and "git-github" format
	// If name already has "git-" prefix, use it as is, otherwise add prefix
	var key string
	if strings.HasPrefix(name, string(ConnectorTypeGit)+"-") {
		key = name
	} else {
		key = string(ConnectorTypeGit) + "-" + name
	}

	conn, ok := r.connectors[key]
	if !ok {
		return nil, fmt.Errorf("git connector not found: %s", key)
	}

	// Verify it's a git connector
	if typed, ok := conn.(GitConnector); ok {
		return typed, nil
	}

	// Check connector type via Type() method
	if ct, ok := conn.(interface{ Type() ConnectorType }); ok {
		if ct.Type() != ConnectorTypeGit {
			return nil, fmt.Errorf("connector %s is not a git connector (type: %s)", key, ct.Type())
		}
	}

	// If connector doesn't implement Type() but implements Connector, assume it's GitConnector
	// (all Connectors implement the interface, GitConnector is just Connector)
	return conn, nil
}

// ResolvePolicy resolves a policy connector by implementation name.
func (r *connectorRegistry) ResolvePolicy(name string) (PolicyConnector, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Handle both "pmock" and "policy-pmock" format
	// If name already has "policy-" prefix, we need to extract technology part
	// Config provides "policy-mock", but connectors are registered as "other-pmock"
	// Actually config should provide just "pmock" (technology), not "policy-mock"
	var key string
	if strings.HasPrefix(name, "policy-") {
		// Extract technology part after "policy-"
		tech := strings.TrimPrefix(name, "policy-")
		key = string(ConnectorTypeOther) + "-" + tech
	} else {
		key = string(ConnectorTypeOther) + "-" + name
	}

	conn, ok := r.connectors[key]
	if !ok {
		return nil, fmt.Errorf("policy connector not found: %s", key)
	}

	// Verify it's a policy connector
	if typed, ok := conn.(PolicyConnector); ok {
		return typed, nil
	}

	// For policy connectors, we can't rely on Type() since they're ConnectorTypeOther
	// But we can check if the name matches expected pattern
	// For now, just return the connector
	return conn, nil
}

// ResolveWebhook resolves a webhook connector by implementation name.
func (r *connectorRegistry) ResolveWebhook(name string) (WebhookConnector, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Handle both "wmock" and "webhook-wmock" format
	// If name already has "webhook-" prefix, we need to extract technology part
	// Config provides "webhook-mock", but connectors are registered as "other-wmock"
	var key string
	if strings.HasPrefix(name, "webhook-") {
		// Extract technology part after "webhook-"
		tech := strings.TrimPrefix(name, "webhook-")
		key = string(ConnectorTypeOther) + "-" + tech
	} else {
		key = string(ConnectorTypeOther) + "-" + name
	}

	conn, ok := r.connectors[key]
	if !ok {
		return nil, fmt.Errorf("webhook connector not found: %s", key)
	}

	// Verify it's a webhook connector
	if typed, ok := conn.(WebhookConnector); ok {
		return typed, nil
	}

	// For webhook connectors, similar to policy
	return conn, nil
}
