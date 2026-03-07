package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
)

func TestAuthMiddleware_MissingTokenAPI(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	logger := zaptest.NewLogger(t)
	h := NewAuthMiddleware("https://issuer.example.com", "test-audience", logger)(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	err := h(c)
	assert.Error(t, err)
	assert.Equal(t, http.StatusUnauthorized, err.(*echo.HTTPError).Code)
}

func TestJobHandler_CreateJob(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/v1/jobs", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := NewJobHandler()
	err := h.CreateJob(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestJobHandler_GetJob(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/v1/jobs/1", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")

	h := NewJobHandler()
	err := h.GetJob(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestJobHandler_CancelJob(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/v1/jobs/1/cancel", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")

	h := NewJobHandler()
	err := h.CancelJob(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRateLimiter(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	logger := zaptest.NewLogger(t)
	h := NewRateLimiterMiddleware(100, 100, logger)(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	err := h(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}
