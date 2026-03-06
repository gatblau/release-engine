package outbox

import (
	"context"
	"time"

	"github.com/gatblau/release-engine/internal/db"
)

// Dispatcher defines the interface for the outbox worker.
type Dispatcher interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

type outboxDispatcher struct {
	db db.Pool
}

// NewOutboxDispatcher creates a new dispatcher.
func NewOutboxDispatcher(db db.Pool) Dispatcher {
	return &outboxDispatcher{db: db}
}

func (d *outboxDispatcher) Start(ctx context.Context) error {
	ticker := time.NewTicker(time.Second * 5)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			// Process outbox rows
		}
	}
}

func (d *outboxDispatcher) Stop(ctx context.Context) error {
	return nil
}
