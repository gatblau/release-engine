// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package dora

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Event is the canonical normalized DORA event emitted by provider normalizers.
type Event struct {
	TenantID       string
	EventType      string
	EventSource    string
	ServiceRef     string
	Environment    string
	CorrelationKey string
	SourceEventID  string
	EventTimestamp time.Time
	Payload        map[string]any
}

// Normalizer transforms a provider webhook payload into zero or more DORA events.
type Normalizer interface {
	Provider() string
	Normalize(ctx context.Context, tenantID string, serviceRef string, headers map[string]string, body []byte) ([]Event, error)
}

// Registry stores provider -> normalizer mappings.
type Registry struct {
	mu          sync.RWMutex
	normalizers map[string]Normalizer
}

func NewRegistry() *Registry {
	return &Registry{normalizers: make(map[string]Normalizer)}
}

func (r *Registry) Resolve(provider string) Normalizer {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.normalizers[strings.ToLower(strings.TrimSpace(provider))]
}

func (r *Registry) Register(n Normalizer) {
	provider := strings.ToLower(strings.TrimSpace(n.Provider()))
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.normalizers[provider]; exists {
		panic("duplicate DORA normalizer for provider: " + provider)
	}
	r.normalizers[provider] = n
}

func (r *Registry) Providers() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.normalizers))
	for provider := range r.normalizers {
		out = append(out, provider)
	}
	sort.Strings(out)
	return out
}

func ValidateEvent(e Event) error {
	if strings.TrimSpace(e.TenantID) == "" {
		return fmt.Errorf("tenant_id is required")
	}
	if strings.TrimSpace(e.ServiceRef) == "" {
		return fmt.Errorf("service_ref is required")
	}
	if strings.TrimSpace(e.EventSource) == "" {
		return fmt.Errorf("event_source is required")
	}
	if e.EventTimestamp.IsZero() {
		return fmt.Errorf("event_timestamp is required")
	}
	switch e.EventType {
	case "deployment.succeeded", "deployment.failed", "commit.pushed", "commit.merged", "incident.opened", "incident.resolved", "rollback.completed":
		return nil
	default:
		return fmt.Errorf("unsupported event_type: %s", e.EventType)
	}
}
