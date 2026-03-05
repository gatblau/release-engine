package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
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
