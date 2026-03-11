package http

import (
	"bytes"
	"encoding/json"
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
	assert.Equal(t, http.StatusAccepted, rec.Code)
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

func TestJobHandler_SubmitDecision_HappyPath(t *testing.T) {
	e := echo.New()
	body := bytes.NewBufferString(`{"decision":"approved","justification":"looks good","idempotency_key":"k-1"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/jobs/job-123/steps/step-456/decisions", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("job_id", "step_id")
	c.SetParamValues("job-123", "step-456")
	c.Set(string(authClaimsKey), AuthClaims{Subject: "approver-1", Role: "release-manager", TenantID: "acme-prod"})

	h := NewJobHandler()
	err := h.SubmitDecision(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestJobHandler_SubmitDecision_IdempotentReplay(t *testing.T) {
	e := echo.New()
	h := NewJobHandler()

	for i := 0; i < 2; i++ {
		body := bytes.NewBufferString(`{"decision":"approved","justification":"looks good","idempotency_key":"k-replay"}`)
		req := httptest.NewRequest(http.MethodPost, "/v1/jobs/job-123/steps/step-456/decisions", body)
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("job_id", "step_id")
		c.SetParamValues("job-123", "step-456")
		c.Set(string(authClaimsKey), AuthClaims{Subject: "approver-1", Role: "release-manager", TenantID: "acme-prod"})

		err := h.SubmitDecision(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		if i == 1 {
			var payload map[string]any
			_ = json.Unmarshal(rec.Body.Bytes(), &payload)
			assert.Equal(t, true, payload["idempotent_replay"])
		}
	}
}

func TestJobHandler_SubmitDecision_Conflict(t *testing.T) {
	e := echo.New()
	h := NewJobHandler()

	first := bytes.NewBufferString(`{"decision":"approved","idempotency_key":"k-conflict"}`)
	req1 := httptest.NewRequest(http.MethodPost, "/v1/jobs/job-123/steps/step-456/decisions", first)
	req1.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec1 := httptest.NewRecorder()
	c1 := e.NewContext(req1, rec1)
	c1.SetParamNames("job_id", "step_id")
	c1.SetParamValues("job-123", "step-456")
	c1.Set(string(authClaimsKey), AuthClaims{Subject: "approver-1", Role: "release-manager", TenantID: "acme-prod"})
	_ = h.SubmitDecision(c1)

	second := bytes.NewBufferString(`{"decision":"rejected","idempotency_key":"k-conflict"}`)
	req2 := httptest.NewRequest(http.MethodPost, "/v1/jobs/job-123/steps/step-456/decisions", second)
	req2.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec2 := httptest.NewRecorder()
	c2 := e.NewContext(req2, rec2)
	c2.SetParamNames("job_id", "step_id")
	c2.SetParamValues("job-123", "step-456")
	c2.Set(string(authClaimsKey), AuthClaims{Subject: "approver-1", Role: "release-manager", TenantID: "acme-prod"})

	err := h.SubmitDecision(c2)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusConflict, rec2.Code)
}

func TestJobHandler_SubmitDecision_Forbidden(t *testing.T) {
	e := echo.New()
	body := bytes.NewBufferString(`{"decision":"approved","idempotency_key":"k-2"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/jobs/job-123/steps/step-456/decisions", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("job_id", "step_id")
	c.SetParamValues("job-123", "step-456")
	c.Set(string(authClaimsKey), AuthClaims{Subject: "approver-2", Role: "developer", TenantID: "acme-prod"})

	h := NewJobHandler()
	err := h.SubmitDecision(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestJobHandler_SubmitDecision_PolicyViolation(t *testing.T) {
	e := echo.New()
	body := bytes.NewBufferString(`{"decision":"approved","idempotency_key":"k-3"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/jobs/job-123/steps/step-456/decisions", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("job_id", "step_id")
	c.SetParamValues("job-123", "step-456")
	c.Set(string(authClaimsKey), AuthClaims{Subject: "owner-1", Role: "release-manager", TenantID: "acme-prod"})

	h := NewJobHandler()
	err := h.SubmitDecision(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestJobHandler_GetApprovalContext(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/v1/jobs/job-123/steps/step-456/approval-context", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("job_id", "step_id")
	c.SetParamValues("job-123", "step-456")

	h := NewJobHandler()
	err := h.GetApprovalContext(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestJobHandler_ListJobs_WaitingApproval(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/v1/jobs?step_status=waiting_approval&tenant=acme-prod&approver_role=release-manager", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := NewJobHandler()
	err := h.ListJobs(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "job-123")
}
