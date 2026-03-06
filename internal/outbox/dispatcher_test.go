package outbox

import (
	"context"
	"testing"
	"time"

	"github.com/gatblau/release-engine/internal/db"
	"github.com/stretchr/testify/assert"
)

func TestNewOutboxDispatcher(t *testing.T) {
	mockPool := new(db.MockPool)
	dispatcher := NewOutboxDispatcher(mockPool)
	assert.NotNil(t, dispatcher)
}

func TestOutboxDispatcher_Start(t *testing.T) {
	mockPool := new(db.MockPool)

	dispatcher := NewOutboxDispatcher(mockPool)

	// Create a context that will cancel after a short time
	ctx, cancel := context.WithCancel(context.Background())

	// Start the dispatcher in a goroutine
	go func() {
		err := dispatcher.Start(ctx)
		assert.NoError(t, err)
	}()

	// Let it run for a bit
	time.Sleep(100 * time.Millisecond)

	// Cancel the context to stop
	cancel()

	// Wait for clean shutdown
	time.Sleep(50 * time.Millisecond)
}

func TestOutboxDispatcher_Stop(t *testing.T) {
	mockPool := new(db.MockPool)
	dispatcher := NewOutboxDispatcher(mockPool)

	// Stop should return without error
	err := dispatcher.Stop(context.Background())
	assert.NoError(t, err)
}

func TestOutboxDispatcher_StartAndStop(t *testing.T) {
	mockPool := new(db.MockPool)

	dispatcher := NewOutboxDispatcher(mockPool)

	// Create a cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	// Start in goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- dispatcher.Start(ctx)
	}()

	// Let it run briefly
	time.Sleep(50 * time.Millisecond)

	// Stop - this should cause Start to return
	err := dispatcher.Stop(context.Background())
	assert.NoError(t, err)

	// Cancel the context (already stopped but doesn't hurt)
	cancel()

	// Wait for goroutine to finish
	select {
	case err := <-errCh:
		assert.NoError(t, err)
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for dispatcher to stop")
	}
}
