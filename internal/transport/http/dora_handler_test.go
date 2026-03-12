package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gatblau/release-engine/internal/db"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestDoraHandler_GetSummary_BrandAuthorized_NoData(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/v1/dora/summary?tenant_id=t-1&service_ref=svc-a&brand_id=b-1", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(string(authClaimsKey), AuthClaims{TenantID: "t-1", BrandIDs: []string{"b-1"}})

	h := NewDoraHandler(nil)
	err := h.GetSummary(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	current := body["current_period"].(map[string]any)
	assert.Equal(t, "no_data", current["deployment_frequency_data_quality"])
	assert.Equal(t, "no_data", current["lead_time_data_quality"])
	_, hasLeadTimeP95 := current["lead_time_p95_seconds"]
	assert.True(t, hasLeadTimeP95)
	assert.Equal(t, "no_data", current["change_failure_rate_data_quality"])
	assert.Equal(t, "no_data", current["mttr_data_quality"])
}

func TestDoraHandler_GetSummary_ForbiddenWithoutBrandOrGroup(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/v1/dora/summary?tenant_id=t-1&service_ref=svc-a", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(string(authClaimsKey), AuthClaims{TenantID: "t-1"})

	h := NewDoraHandler(nil)
	err := h.GetSummary(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestDoraHandler_GetSummary_GroupAuthorizationFailClosedWhenMapUnavailable(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/v1/dora/summary?tenant_id=t-1&service_ref=svc-a&group_id=g-1", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(string(authClaimsKey), AuthClaims{TenantID: "t-1", GroupIDs: []string{"g-1"}})

	h := NewDoraHandler(nil)
	err := h.GetSummary(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Contains(t, rec.Body.String(), "group_map_stale")
}

func TestDoraHandler_GetGroupSummary_RequiresFields(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/v1/dora/group/summary?tenant_id=t-1&group_id=g-1", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(string(authClaimsKey), AuthClaims{TenantID: "t-1", GroupIDs: []string{"g-1"}})

	h := NewDoraHandler(nil)
	err := h.GetGroupSummary(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestDoraHandler_GetGroupSummary_ClassificationVersionMismatchReturns422(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/v1/dora/group/summary?tenant_id=t-1&service_ref=svc-a&group_id=g-1", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(string(authClaimsKey), AuthClaims{TenantID: "t-1", GroupIDs: []string{"g-1"}})

	pool := new(db.MockPool)
	conn := new(db.MockConn)
	pool.On("Acquire", mock.Anything).Return(conn, nil).Twice()
	conn.On("Release").Return().Twice()
	conn.On("QueryRow", mock.Anything, mock.Anything, mock.Anything).Return(&scanRow{values: []any{time.Now().UTC()}}).Once()
	conn.On("QueryRow", mock.Anything, mock.Anything, mock.Anything).Return(&scanRow{values: []any{int64(2), []byte(`[{"brand_id":"b-1","classification_version":"v1"},{"brand_id":"b-2","classification_version":"v2"}]`)}}).Once()

	h := NewDoraHandler(pool)
	err := h.GetGroupSummary(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	assert.Contains(t, rec.Body.String(), "classification_version_mismatch")
}

func TestDoraHandler_GetDeployments_ForbiddenForUnauthorizedBrand(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/v1/dora/deployments?tenant_id=t-1&service_ref=svc-a&brand_id=b-2", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(string(authClaimsKey), AuthClaims{TenantID: "t-1", BrandIDs: []string{"b-1"}})

	h := NewDoraHandler(nil)
	err := h.GetDeployments(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestDoraHandler_ListDeadLetter_ValidatesTenantID(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/v1/internal/dora/dead-letter?brand_id=b-1", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(string(authClaimsKey), AuthClaims{TenantID: "t-1", BrandIDs: []string{"b-1"}})

	h := NewDoraHandler(nil)
	err := h.ListDeadLetter(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestDoraHandler_IngestWebhook_UnknownNormalizer_ReturnsStructuredFailure(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/dora/unknown?tenant_id=t-1&service_ref=svc-a", strings.NewReader(`{"hello":"world"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("provider")
	c.SetParamValues("unknown")

	h := NewDoraHandler(nil)
	err := h.IngestWebhook(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, true, body["accepted"])
	assert.Equal(t, false, body["processed"])
	assert.Equal(t, "store_error", body["error_code"])
}

func TestDoraHandler_IngestWebhook_GitHubIrrelevantEvent_AcceptsZeroEvents(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/dora/github?tenant_id=t-1&service_ref=svc-a", strings.NewReader(`{"zen":"keep it logically awesome"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set("X-GitHub-Event", "ping")
	req.Header.Set("X-GitHub-Delivery", "delivery-1")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("provider")
	c.SetParamValues("github")

	h := NewDoraHandler(nil)
	err := h.IngestWebhook(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, true, body["accepted"])
	assert.Equal(t, true, body["processed"])
	assert.EqualValues(t, 0, body["events_written"])
}

func TestDoraHandler_ReplayDeadLetter_ValidatesFields(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/v1/internal/dora/dead-letter/123/replay?brand_id=b-1", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("123")
	c.Set(string(authClaimsKey), AuthClaims{TenantID: "t-1", BrandIDs: []string{"b-1"}})

	h := NewDoraHandler(nil)
	err := h.ReplayDeadLetter(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestDoraHandler_ReplayDeadLetter_NotFoundWhenPoolMissing(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/v1/internal/dora/dead-letter/123/replay?tenant_id=t-1&brand_id=b-1", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("123")
	c.Set(string(authClaimsKey), AuthClaims{TenantID: "t-1", BrandIDs: []string{"b-1"}})

	h := NewDoraHandler(nil)
	err := h.ReplayDeadLetter(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestCFRQuality(t *testing.T) {
	assert.Equal(t, "no_data", cfrQuality(cfrStats{}))

	proxy := 12.3
	assert.Equal(t, "proxy", cfrQuality(cfrStats{DeploymentCount: 5, ProxyFailureRatePercent: &proxy}))

	assert.Equal(t, "partial", cfrQuality(cfrStats{DeploymentCount: 5, IncidentOpenedCount: 2, HeuristicCorrelations: 1, CorrelatedDeployments: 1}))
	assert.Equal(t, "complete", cfrQuality(cfrStats{DeploymentCount: 5, IncidentOpenedCount: 2, ExplicitCorrelations: 2, CorrelatedDeployments: 2}))
}

func TestMTTRQuality(t *testing.T) {
	assert.Equal(t, "no_data", mttrQuality(mttrStats{}))
	assert.Equal(t, "partial", mttrQuality(mttrStats{OpenedCount: 3}))

	v := 900.0
	assert.Equal(t, "partial", mttrQuality(mttrStats{OpenedCount: 3, ResolvedPairCount: 2, P50: &v}))
	assert.Equal(t, "complete", mttrQuality(mttrStats{OpenedCount: 2, ResolvedPairCount: 2, P50: &v}))
}

func TestCFRProxyDescription(t *testing.T) {
	assert.Nil(t, cfrProxyDescription(cfrStats{}))
	proxy := 5.0
	assert.NotNil(t, cfrProxyDescription(cfrStats{ProxyFailureRatePercent: &proxy}))
}

type scanRow struct {
	values []any
	err    error
}

func (r *scanRow) Scan(dest ...interface{}) error {
	if r.err != nil {
		return r.err
	}
	for i := range dest {
		switch d := dest[i].(type) {
		case *time.Time:
			*d = r.values[i].(time.Time)
		case *int64:
			*d = r.values[i].(int64)
		case *[]byte:
			*d = r.values[i].([]byte)
		default:
			return nil
		}
	}
	return nil
}
