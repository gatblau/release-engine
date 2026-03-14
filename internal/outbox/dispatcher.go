// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package outbox

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/gatblau/release-engine/internal/db"
)

const (
	EventApprovalRequested = "approval_requested"
	EventApprovalDecided   = "approval_decided"
	EventApprovalEscalated = "approval_escalated"
	EventApprovalExpired   = "approval_expired"
)

// Event models an outbox event payload queued by services.
type Event struct {
	Type       string         `json:"type"`
	JobID      string         `json:"job_id"`
	StepID     string         `json:"step_id"`
	OccurredAt time.Time      `json:"occurred_at"`
	Payload    map[string]any `json:"payload,omitempty"`
}

// Dispatcher defines the interface for the outbox worker.
type Dispatcher interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	RegisterEventType(eventType string)
	RegisteredEventTypes() []string
	Emit(ctx context.Context, event Event) error
}

type outboxDispatcher struct {
	db         db.Pool
	mu         sync.Mutex
	eventTypes map[string]struct{}
	pending    []Event
}

// NewOutboxDispatcher creates a new dispatcher.
func NewOutboxDispatcher(db db.Pool) Dispatcher {
	d := &outboxDispatcher{
		db:         db,
		eventTypes: make(map[string]struct{}),
		pending:    make([]Event, 0),
	}
	d.RegisterEventType(EventApprovalRequested)
	d.RegisterEventType(EventApprovalDecided)
	d.RegisterEventType(EventApprovalEscalated)
	d.RegisterEventType(EventApprovalExpired)
	return d
}

func (d *outboxDispatcher) RegisterEventType(eventType string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if eventType == "" {
		return
	}
	d.eventTypes[eventType] = struct{}{}
}

func (d *outboxDispatcher) RegisteredEventTypes() []string {
	d.mu.Lock()
	defer d.mu.Unlock()

	types := make([]string, 0, len(d.eventTypes))
	for eventType := range d.eventTypes {
		types = append(types, eventType)
	}
	sort.Strings(types)
	return types
}

func (d *outboxDispatcher) Emit(_ context.Context, event Event) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, ok := d.eventTypes[event.Type]; !ok {
		return fmt.Errorf("outbox event type not registered: %s", event.Type)
	}
	d.pending = append(d.pending, event)
	return nil
}

func (d *outboxDispatcher) Start(ctx context.Context) error {
	ticker := time.NewTicker(time.Second * 5)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			d.mu.Lock()
			if len(d.pending) > 0 {
				d.pending = d.pending[:0]
			}
			d.mu.Unlock()
		}
	}
}

func (d *outboxDispatcher) Stop(ctx context.Context) error {
	return nil
}
