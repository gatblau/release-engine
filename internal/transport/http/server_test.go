package http

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestHTTPServer_Healthz(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	srv := NewServer(8080, logger)
	srv.RegisterRoutes()

	// Direct access to Echo instance to test handler
	e := srv.(*server).e
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `{"status":"ok"}`)
}

func TestHTTPServer_StartAndShutdown(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	// Use port 0 to get an available port
	srv := NewServer(0, logger)
	srv.RegisterRoutes()

	// Start server in a goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start(context.Background())
	}()

	// Give the server time to start
	time.Sleep(100 * time.Millisecond)

	// Verify server is running by making a health check request
	resp, err := http.Get("http://localhost:" + getPort(srv) + "/healthz")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, resp.Body.Close())

	// Now shutdown the server gracefully
	err = srv.Shutdown(context.Background())
	assert.NoError(t, err)

	// Wait a bit for the Start goroutine to complete
	time.Sleep(100 * time.Millisecond)

	// When server is gracefully shutdown, Start returns nil (http.ErrServerClosed is filtered in Start)
	select {
	case err := <-errCh:
		// Start returns nil when gracefully shutdown (http.ErrServerClosed is filtered out)
		assert.Nil(t, err, "Start should return nil when gracefully shutdown")
	default:
		// Channel may be empty if the server already closed
	}
}

func TestHTTPServer_ShutdownGraceful(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	srv := NewServer(0, logger)
	srv.RegisterRoutes()

	// Start server in a goroutine
	go func() {
		_ = srv.Start(context.Background())
	}()

	// Give the server time to start
	time.Sleep(100 * time.Millisecond)

	// Graceful shutdown should work
	err := srv.Shutdown(context.Background())
	assert.NoError(t, err)
}

// Helper function to get the port the server is listening on
func getPort(srv Server) string {
	// Use a default if we can't determine the port
	ln := srv.(*server).e.Listener
	if ln == nil {
		// Find the listener by attempting to connect
		conn, err := net.Dial("tcp", "localhost:0")
		if err == nil {
			_ = conn.Close()
			return "0"
		}
		return "0"
	}
	return fmt.Sprintf("%d", ln.Addr().(*net.TCPAddr).Port)
}
