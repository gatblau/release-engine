// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package http

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gatblau/release-engine/internal/db"
	"github.com/gatblau/release-engine/internal/dora"
	"github.com/labstack/echo/v4"
)

type doraMetricsRecorder interface {
	RecordDoraLeadTime(tenantID, serviceRef string, leadTimeSeconds float64)
	RecordDoraCFR(tenantID, serviceRef string, windowDays int, percent float64)
	RecordDoraMTTR(tenantID, serviceRef string, windowDays int, seconds float64)
	RecordDoraDeadLetter(tenantID, provider, errorCode string)
	RecordDoraTenantsAboveCFRThreshold(windowDays int, threshold float64, total float64)
	RecordDoraTenantsWithNoDoraData(metric string, total float64)
}

type DoraHandler struct {
	pool         db.Pool
	registry     *dora.Registry
	metrics      doraMetricsRecorder
	groupMapTTL  time.Duration
	nowFn        func() time.Time
	defaultLimit int
}

type leadTimeStats struct {
	P50                   *float64
	P95                   *float64
	SuccessfulDeployments int64
	CorrelatedDeployments int64
}

type cfrStats struct {
	Percent                 *float64
	DeploymentCount         int64
	IncidentOpenedCount     int64
	CorrelatedDeployments   int64
	ExplicitCorrelations    int64
	HeuristicCorrelations   int64
	ProxyFailureRatePercent *float64
}

type mttrStats struct {
	P50               *float64
	OpenedCount       int64
	ResolvedPairCount int64
}

func NewDoraHandler(pool db.Pool) *DoraHandler {
	registry := dora.NewRegistry()
	registry.Register(dora.NewGitHubNormalizer())
	registry.Register(dora.NewGitLabNormalizer())
	registry.Register(dora.NewOpsgenieNormalizer())
	registry.Register(dora.NewDatadogNormalizer())

	return &DoraHandler{
		pool:         pool,
		registry:     registry,
		groupMapTTL:  15 * time.Minute,
		nowFn:        time.Now,
		defaultLimit: 50,
	}
}

func (h *DoraHandler) WithMetricsRecorder(recorder doraMetricsRecorder) *DoraHandler {
	h.metrics = recorder
	return h
}

func (h *DoraHandler) GetSummary(c echo.Context) error {
	tenantID := strings.TrimSpace(c.QueryParam("tenant_id"))
	serviceRef := strings.TrimSpace(c.QueryParam("service_ref"))
	if tenantID == "" || serviceRef == "" {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "tenant_id and service_ref are required", Code: "INVALID_REQUEST"})
	}

	if err := h.authorize(c, tenantID); err != nil {
		return err
	}

	windowDays := 30
	if raw := strings.TrimSpace(c.QueryParam("window_days")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "window_days must be a positive integer", Code: "INVALID_REQUEST"})
		}
		windowDays = parsed
	}

	classificationVersion := strings.TrimSpace(c.QueryParam("classification_version"))
	if classificationVersion == "" {
		classificationVersion = "dora-2023-default+gates-included"
	}

	now := h.nowFn().UTC()
	start := now.AddDate(0, 0, -windowDays)
	prevStart := start.AddDate(0, 0, -windowDays)

	currentDailyAvg := 0.0
	previousDailyAvg := 0.0
	deploymentDataQuality := "no_data"
	currentLeadTime := leadTimeStats{}
	previousLeadTime := leadTimeStats{}
	leadTimeDataQuality := "no_data"
	currentCFR := cfrStats{}
	previousCFR := cfrStats{}
	cfrDataQuality := "no_data"
	currentMTTR := mttrStats{}
	previousMTTR := mttrStats{}
	mttrDataQuality := "no_data"

	if h.pool != nil {
		conn, err := h.pool.Acquire(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "database unavailable", Code: "INTERNAL_ERROR"})
		}
		defer conn.Release()

		var currentSuccess int64
		err = conn.QueryRow(c.Request().Context(), `
			SELECT COALESCE(SUM(success_count), 0)
			FROM dora_deployment_frequency_daily
			WHERE tenant_id = $1
			  AND service_ref = $2
			  AND bucket >= $3
			  AND bucket < $4
		`, tenantID, serviceRef, start, now).Scan(&currentSuccess)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "could not query deployment frequency", Code: "INTERNAL_ERROR"})
		}

		var previousSuccess int64
		err = conn.QueryRow(c.Request().Context(), `
			SELECT COALESCE(SUM(success_count), 0)
			FROM dora_deployment_frequency_daily
			WHERE tenant_id = $1
			  AND service_ref = $2
			  AND bucket >= $3
			  AND bucket < $4
		`, tenantID, serviceRef, prevStart, start).Scan(&previousSuccess)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "could not query previous period frequency", Code: "INTERNAL_ERROR"})
		}

		currentDailyAvg = float64(currentSuccess) / float64(windowDays)
		previousDailyAvg = float64(previousSuccess) / float64(windowDays)
		if currentSuccess > 0 {
			deploymentDataQuality = "complete"
		}

		currentLeadTime, err = h.queryLeadTimeStats(c, conn, tenantID, serviceRef, start, now)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "could not query lead time", Code: "INTERNAL_ERROR"})
		}

		previousLeadTime, err = h.queryLeadTimeStats(c, conn, tenantID, serviceRef, prevStart, start)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "could not query previous lead time", Code: "INTERNAL_ERROR"})
		}

		leadTimeDataQuality = "no_data"
		if currentLeadTime.SuccessfulDeployments > 0 && currentLeadTime.CorrelatedDeployments > 0 && currentLeadTime.P50 != nil {
			coverage := float64(currentLeadTime.CorrelatedDeployments) / float64(currentLeadTime.SuccessfulDeployments)
			if coverage >= 0.80 {
				leadTimeDataQuality = "complete"
			} else {
				leadTimeDataQuality = "partial"
			}
			if h.metrics != nil {
				h.metrics.RecordDoraLeadTime(tenantID, serviceRef, *currentLeadTime.P50)
			}
		}

		currentCFR, err = h.queryCFRStats(c, conn, tenantID, serviceRef, start, now)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "could not query change failure rate", Code: "INTERNAL_ERROR"})
		}
		previousCFR, err = h.queryCFRStats(c, conn, tenantID, serviceRef, prevStart, start)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "could not query previous change failure rate", Code: "INTERNAL_ERROR"})
		}
		cfrDataQuality = cfrQuality(currentCFR)

		currentMTTR, err = h.queryMTTRStats(c, conn, tenantID, serviceRef, start, now)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "could not query mttr", Code: "INTERNAL_ERROR"})
		}
		previousMTTR, err = h.queryMTTRStats(c, conn, tenantID, serviceRef, prevStart, start)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "could not query previous mttr", Code: "INTERNAL_ERROR"})
		}
		mttrDataQuality = mttrQuality(currentMTTR)

		if h.metrics != nil {
			if currentCFR.Percent != nil {
				h.metrics.RecordDoraCFR(tenantID, serviceRef, windowDays, *currentCFR.Percent)
				above := 0.0
				if *currentCFR.Percent >= 15.0 {
					above = 1.0
				}
				h.metrics.RecordDoraTenantsAboveCFRThreshold(windowDays, 15.0, above)
			}
			if currentMTTR.P50 != nil {
				h.metrics.RecordDoraMTTR(tenantID, serviceRef, windowDays, *currentMTTR.P50)
			}
			h.metrics.RecordDoraTenantsWithNoDoraData("deployment_frequency", boolToFloat(deploymentDataQuality == "no_data"))
			h.metrics.RecordDoraTenantsWithNoDoraData("lead_time", boolToFloat(leadTimeDataQuality == "no_data"))
			h.metrics.RecordDoraTenantsWithNoDoraData("change_failure_rate", boolToFloat(cfrDataQuality == "no_data"))
			h.metrics.RecordDoraTenantsWithNoDoraData("mttr", boolToFloat(mttrDataQuality == "no_data"))
		}
	}

	resp := map[string]any{
		"tenant_id":              tenantID,
		"service_ref":            serviceRef,
		"classification_version": classificationVersion,
		"current_period": map[string]any{
			"start":                                 start,
			"end":                                   now,
			"deployment_frequency_daily_avg":        currentDailyAvg,
			"deployment_frequency_data_quality":     deploymentDataQuality,
			"lead_time_p50_seconds":                 currentLeadTime.P50,
			"lead_time_p95_seconds":                 currentLeadTime.P95,
			"lead_time_data_quality":                leadTimeDataQuality,
			"change_failure_rate_percent":           currentCFR.Percent,
			"change_failure_rate_data_quality":      cfrDataQuality,
			"change_failure_rate_proxy_description": cfrProxyDescription(currentCFR),
			"mttr_p50_seconds":                      currentMTTR.P50,
			"mttr_data_quality":                     mttrDataQuality,
			"dora_level":                            nil,
		},
		"previous_period": map[string]any{
			"start":                          prevStart,
			"end":                            start,
			"deployment_frequency_daily_avg": previousDailyAvg,
			"lead_time_p50_seconds":          previousLeadTime.P50,
			"lead_time_p95_seconds":          previousLeadTime.P95,
			"change_failure_rate_percent":    currentOrNil(previousCFR.Percent),
			"mttr_p50_seconds":               currentOrNil(previousMTTR.P50),
			"dora_level":                     nil,
		},
		"deltas": map[string]any{
			"deployment_frequency_percent": percentDelta(previousDailyAvg, currentDailyAvg),
			"lead_time_percent":            leadTimeDelta(previousLeadTime.P50, currentLeadTime.P50),
			"change_failure_rate_percent":  leadTimeDelta(previousCFR.Percent, currentCFR.Percent),
			"mttr_percent":                 leadTimeDelta(previousMTTR.P50, currentMTTR.P50),
		},
	}

	return c.JSON(http.StatusOK, resp)
}

func (h *DoraHandler) GetGroupSummary(c echo.Context) error {
	tenantID := strings.TrimSpace(c.QueryParam("tenant_id"))
	serviceRef := strings.TrimSpace(c.QueryParam("service_ref"))
	groupID := strings.TrimSpace(c.QueryParam("group_id"))
	if tenantID == "" || serviceRef == "" || groupID == "" {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "tenant_id, service_ref and group_id are required", Code: "INVALID_REQUEST"})
	}

	if err := h.authorize(c, tenantID); err != nil {
		return err
	}

	if h.pool == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]any{"error": "group map stale or unavailable", "code": "group_map_stale"})
	}

	conn, err := h.pool.Acquire(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]any{"error": "group map stale or unavailable", "code": "group_map_stale"})
	}
	defer conn.Release()

	var distinctVersions int64
	var mismatchDetails []byte
	err = conn.QueryRow(c.Request().Context(), `
		SELECT
			COUNT(DISTINCT classification_version),
			COALESCE(
				json_agg(
					json_build_object('brand_id', brand_id, 'classification_version', classification_version)
					ORDER BY brand_id
				),
				'[]'::json
			)
		FROM dora_group_brand_map
		WHERE tenant_id = $1
		  AND group_id = $2
	`, tenantID, groupID).Scan(&distinctVersions, &mismatchDetails)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "could not query group classification versions", Code: "INTERNAL_ERROR"})
	}

	if distinctVersions > 1 {
		var breakdown any = []any{}
		_ = json.Unmarshal(mismatchDetails, &breakdown)
		return c.JSON(http.StatusUnprocessableEntity, map[string]any{
			"error": "group brands have mismatched classification versions",
			"code":  "classification_version_mismatch",
			"details": map[string]any{
				"group_id":                 groupID,
				"classification_breakdown": breakdown,
			},
		})
	}

	return h.GetSummary(c)
}

func (h *DoraHandler) GetDeployments(c echo.Context) error {
	tenantID := strings.TrimSpace(c.QueryParam("tenant_id"))
	serviceRef := strings.TrimSpace(c.QueryParam("service_ref"))
	if tenantID == "" || serviceRef == "" {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "tenant_id and service_ref are required", Code: "INVALID_REQUEST"})
	}

	if err := h.authorize(c, tenantID); err != nil {
		return err
	}

	limit := h.defaultLimit
	if raw := strings.TrimSpace(c.QueryParam("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 || parsed > 200 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "limit must be between 1 and 200", Code: "INVALID_REQUEST"})
		}
		limit = parsed
	}

	deployments := make([]map[string]any, 0)
	if h.pool != nil {
		conn, err := h.pool.Acquire(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "database unavailable", Code: "INTERNAL_ERROR"})
		}
		defer conn.Release()

		rows, err := conn.Query(c.Request().Context(), `
			SELECT id::text, event_type, event_source, environment, correlation_key, event_timestamp, payload
			FROM dora_events
			WHERE tenant_id = $1
			  AND service_ref = $2
			  AND event_type IN ('deployment.succeeded', 'deployment.failed')
			ORDER BY event_timestamp DESC
			LIMIT $3
		`, tenantID, serviceRef, limit)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "could not query deployments", Code: "INTERNAL_ERROR"})
		}
		defer rows.Close()

		for rows.Next() {
			var id, eventType, eventSource, environment, correlationKey string
			var eventTimestamp time.Time
			var payload []byte
			if err := rows.Scan(&id, &eventType, &eventSource, &environment, &correlationKey, &eventTimestamp, &payload); err != nil {
				return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "could not scan deployment row", Code: "INTERNAL_ERROR"})
			}
			deployments = append(deployments, map[string]any{
				"id":              id,
				"event_type":      eventType,
				"event_source":    eventSource,
				"environment":     environment,
				"correlation_key": correlationKey,
				"event_timestamp": eventTimestamp,
				"payload":         string(payload),
			})
		}
	}

	return c.JSON(http.StatusOK, map[string]any{
		"tenant_id":   tenantID,
		"service_ref": serviceRef,
		"deployments": deployments,
		"count":       len(deployments),
	})
}

func (h *DoraHandler) ListDeadLetter(c echo.Context) error {
	tenantID := strings.TrimSpace(c.QueryParam("tenant_id"))
	if tenantID == "" {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "tenant_id is required", Code: "INVALID_REQUEST"})
	}

	if err := h.authorize(c, tenantID); err != nil {
		return err
	}

	limit := h.defaultLimit
	if raw := strings.TrimSpace(c.QueryParam("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 || parsed > 200 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "limit must be between 1 and 200", Code: "INVALID_REQUEST"})
		}
		limit = parsed
	}

	provider := strings.TrimSpace(c.QueryParam("provider"))
	items := make([]map[string]any, 0)
	if h.pool != nil {
		conn, err := h.pool.Acquire(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "database unavailable", Code: "INTERNAL_ERROR"})
		}
		defer conn.Release()

		query := `
			SELECT id::text, provider, source_event_id, failure_reason, created_at, replayed_at
			FROM dora_webhook_dead_letter
			WHERE tenant_id = $1`
		args := []any{tenantID}
		if provider != "" {
			query += ` AND provider = $2`
			args = append(args, provider)
			query += ` ORDER BY created_at DESC LIMIT $3`
			args = append(args, limit)
		} else {
			query += ` ORDER BY created_at DESC LIMIT $2`
			args = append(args, limit)
		}

		rows, err := conn.Query(c.Request().Context(), query, args...)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "could not query dead-letter rows", Code: "INTERNAL_ERROR"})
		}
		defer rows.Close()

		for rows.Next() {
			var id, rowProvider string
			var sourceEventID *string
			var failureReason string
			var createdAt time.Time
			var replayedAt *time.Time
			if err := rows.Scan(&id, &rowProvider, &sourceEventID, &failureReason, &createdAt, &replayedAt); err != nil {
				return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "could not scan dead-letter row", Code: "INTERNAL_ERROR"})
			}
			items = append(items, map[string]any{
				"id":              id,
				"provider":        rowProvider,
				"source_event_id": sourceEventID,
				"failure_reason":  failureReason,
				"created_at":      createdAt,
				"replayed_at":     replayedAt,
			})
		}
	}

	return c.JSON(http.StatusOK, map[string]any{"items": items, "count": len(items)})
}

func (h *DoraHandler) GetDeadLetter(c echo.Context) error {
	id := strings.TrimSpace(c.Param("id"))
	tenantID := strings.TrimSpace(c.QueryParam("tenant_id"))
	if id == "" || tenantID == "" {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "id and tenant_id are required", Code: "INVALID_REQUEST"})
	}

	if err := h.authorize(c, tenantID); err != nil {
		return err
	}

	if h.pool == nil {
		return c.JSON(http.StatusNotFound, ErrorResponse{Error: "dead-letter row not found", Code: "NOT_FOUND"})
	}

	conn, err := h.pool.Acquire(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "database unavailable", Code: "INTERNAL_ERROR"})
	}
	defer conn.Release()

	var provider string
	var sourceEventID *string
	var headers []byte
	var body []byte
	var failureReason string
	var createdAt time.Time
	var replayedAt *time.Time
	err = conn.QueryRow(c.Request().Context(), `
		SELECT provider, source_event_id, headers, body, failure_reason, created_at, replayed_at
		FROM dora_webhook_dead_letter
		WHERE id::text = $1 AND tenant_id = $2
	`, id, tenantID).Scan(&provider, &sourceEventID, &headers, &body, &failureReason, &createdAt, &replayedAt)
	if err != nil {
		return c.JSON(http.StatusNotFound, ErrorResponse{Error: "dead-letter row not found", Code: "NOT_FOUND"})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"id":              id,
		"provider":        provider,
		"source_event_id": sourceEventID,
		"headers":         string(headers),
		"body":            string(body),
		"failure_reason":  failureReason,
		"created_at":      createdAt,
		"replayed_at":     replayedAt,
	})
}

func (h *DoraHandler) ReplayDeadLetter(c echo.Context) error {
	id := strings.TrimSpace(c.Param("id"))
	tenantID := strings.TrimSpace(c.QueryParam("tenant_id"))
	if id == "" || tenantID == "" {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "id and tenant_id are required", Code: "INVALID_REQUEST"})
	}

	if err := h.authorize(c, tenantID); err != nil {
		return err
	}

	if h.pool == nil {
		return c.JSON(http.StatusNotFound, ErrorResponse{Error: "dead-letter row not found", Code: "NOT_FOUND"})
	}

	conn, err := h.pool.Acquire(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "database unavailable", Code: "INTERNAL_ERROR"})
	}
	defer conn.Release()

	var provider string
	var headersJSON []byte
	var body []byte
	var replayedAt *time.Time
	err = conn.QueryRow(c.Request().Context(), `
		SELECT provider, headers, body, replayed_at
		FROM dora_webhook_dead_letter
		WHERE id::text = $1 AND tenant_id = $2
	`, id, tenantID).Scan(&provider, &headersJSON, &body, &replayedAt)
	if err != nil {
		return c.JSON(http.StatusNotFound, ErrorResponse{Error: "dead-letter row not found", Code: "NOT_FOUND"})
	}

	if replayedAt != nil {
		return c.JSON(http.StatusConflict, ErrorResponse{Error: "dead-letter row already replayed", Code: "ALREADY_REPLAYED"})
	}

	headerMap, err := parseStoredHeaderMap(headersJSON)
	if err != nil {
		return c.JSON(http.StatusUnprocessableEntity, ErrorResponse{Error: "invalid stored dead-letter headers", Code: "INVALID_REQUEST"})
	}

	normalizer := h.registry.Resolve(provider)
	if normalizer == nil {
		return c.JSON(http.StatusUnprocessableEntity, ErrorResponse{Error: "no normalizer registered for provider", Code: "NORMALIZER_NOT_FOUND"})
	}

	serviceRef := strings.TrimSpace(c.QueryParam("service_ref"))
	if serviceRef == "" {
		serviceRef = "unknown"
	}

	events, err := normalizer.Normalize(c.Request().Context(), tenantID, serviceRef, headerMap, body)
	if err != nil {
		return c.JSON(http.StatusUnprocessableEntity, ErrorResponse{Error: "normalizer replay failed", Code: "NORMALIZER_ERROR", Details: err.Error()})
	}

	for _, ev := range events {
		if err := dora.ValidateEvent(ev); err != nil {
			return c.JSON(http.StatusUnprocessableEntity, ErrorResponse{Error: "normalizer replay validation failed", Code: "VALIDATION_ERROR", Details: err.Error()})
		}
	}

	written := 0
	for _, ev := range events {
		payload := "{}"
		if ev.Payload != nil {
			b, mErr := json.Marshal(ev.Payload)
			if mErr != nil {
				return c.JSON(http.StatusUnprocessableEntity, ErrorResponse{Error: "normalizer replay validation failed", Code: "VALIDATION_ERROR", Details: mErr.Error()})
			}
			payload = string(b)
		}

		_, err = conn.Exec(c.Request().Context(), `
			INSERT INTO dora_events
			(tenant_id, event_type, event_source, service_ref, environment, correlation_key, source_event_id, event_timestamp, payload)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb)
			ON CONFLICT DO NOTHING
		`, ev.TenantID, ev.EventType, ev.EventSource, ev.ServiceRef, ev.Environment, ev.CorrelationKey, nullableString(ev.SourceEventID), ev.EventTimestamp, payload)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "could not replay dead-letter row", Code: "INTERNAL_ERROR"})
		}
		written++
	}

	var replayJobID string
	err = conn.QueryRow(c.Request().Context(), `
		UPDATE dora_webhook_dead_letter
		SET replayed_at = now(),
		    replay_job_id = COALESCE(replay_job_id, gen_random_uuid())
		WHERE id::text = $1 AND tenant_id = $2 AND replayed_at IS NULL
		RETURNING replay_job_id::text
	`, id, tenantID).Scan(&replayJobID)
	if err != nil {
		return c.JSON(http.StatusConflict, ErrorResponse{Error: "dead-letter row already replayed", Code: "ALREADY_REPLAYED"})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"accepted":       true,
		"processed":      true,
		"dead_letter_id": id,
		"replay_job_id":  replayJobID,
		"events_written": written,
	})
}

func (h *DoraHandler) IngestWebhook(c echo.Context) error {
	provider := strings.ToLower(strings.TrimSpace(c.Param("provider")))
	tenantID := strings.TrimSpace(c.QueryParam("tenant_id"))
	serviceRef := strings.TrimSpace(c.QueryParam("service_ref"))
	if provider == "" || tenantID == "" || serviceRef == "" {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "provider, tenant_id and service_ref are required", Code: "INVALID_REQUEST"})
	}

	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "could not read request body", Code: "INVALID_REQUEST"})
	}
	headersJSON, headerMap := headerMaps(c)

	normalizer := h.registry.Resolve(provider)
	if normalizer == nil {
		return c.JSON(http.StatusOK, h.failWithDeadLetter(c, tenantID, provider, "normalizer_not_found", "no normalizer registered for provider", headersJSON, body))
	}

	events, err := normalizer.Normalize(c.Request().Context(), tenantID, serviceRef, headerMap, body)
	if err != nil {
		return c.JSON(http.StatusOK, h.failWithDeadLetter(c, tenantID, provider, "normalizer_error", err.Error(), headersJSON, body))
	}

	for _, ev := range events {
		if err := dora.ValidateEvent(ev); err != nil {
			return c.JSON(http.StatusOK, h.failWithDeadLetter(c, tenantID, provider, "validation_error", err.Error(), headersJSON, body))
		}
	}

	if len(events) == 0 {
		return c.JSON(http.StatusOK, map[string]any{"accepted": true, "processed": true, "events_written": 0})
	}

	if h.pool == nil {
		return c.JSON(http.StatusOK, h.failWithDeadLetter(c, tenantID, provider, "store_error", "database unavailable", headersJSON, body))
	}

	conn, err := h.pool.Acquire(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusOK, h.failWithDeadLetter(c, tenantID, provider, "store_error", "database unavailable", headersJSON, body))
	}
	defer conn.Release()

	written := 0
	for _, ev := range events {
		payload := "{}"
		if ev.Payload != nil {
			b, mErr := json.Marshal(ev.Payload)
			if mErr != nil {
				return c.JSON(http.StatusOK, h.failWithDeadLetter(c, tenantID, provider, "validation_error", mErr.Error(), headersJSON, body))
			}
			payload = string(b)
		}

		_, err = conn.Exec(c.Request().Context(), `
			INSERT INTO dora_events
			(tenant_id, event_type, event_source, service_ref, environment, correlation_key, source_event_id, event_timestamp, payload)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb)
			ON CONFLICT DO NOTHING
		`, ev.TenantID, ev.EventType, ev.EventSource, ev.ServiceRef, ev.Environment, ev.CorrelationKey, nullableString(ev.SourceEventID), ev.EventTimestamp, payload)
		if err != nil {
			return c.JSON(http.StatusOK, h.failWithDeadLetter(c, tenantID, provider, "store_error", err.Error(), headersJSON, body))
		}
		written++
	}

	return c.JSON(http.StatusOK, map[string]any{
		"accepted":       true,
		"processed":      true,
		"events_written": written,
	})
}

func (h *DoraHandler) authorize(c echo.Context, tenantID string) error {
	claims, ok := GetAuthClaims(c)
	if !ok {
		return c.JSON(http.StatusForbidden, ErrorResponse{Error: "missing authorization claims", Code: "FORBIDDEN"})
	}
	if claims.TenantID != "" && claims.TenantID != tenantID {
		return c.JSON(http.StatusForbidden, ErrorResponse{Error: "tenant mismatch", Code: "FORBIDDEN"})
	}

	brandID := strings.TrimSpace(c.QueryParam("brand_id"))
	groupID := strings.TrimSpace(c.QueryParam("group_id"))
	if (brandID == "" && groupID == "") || (brandID != "" && groupID != "") {
		return c.JSON(http.StatusForbidden, ErrorResponse{Error: "exactly one of brand_id or group_id is required", Code: "FORBIDDEN"})
	}

	if brandID != "" {
		if !contains(claims.BrandIDs, brandID) {
			return c.JSON(http.StatusForbidden, ErrorResponse{Error: "brand not permitted", Code: "FORBIDDEN"})
		}
		return nil
	}

	if !contains(claims.GroupIDs, groupID) {
		return c.JSON(http.StatusForbidden, ErrorResponse{Error: "group not permitted", Code: "FORBIDDEN"})
	}

	if h.pool == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]any{"error": "group map stale or unavailable", "code": "group_map_stale"})
	}

	conn, err := h.pool.Acquire(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]any{"error": "group map stale or unavailable", "code": "group_map_stale"})
	}
	defer conn.Release()

	var lastSyncedAt time.Time
	err = conn.QueryRow(c.Request().Context(), `
		SELECT MAX(last_synced_at)
		FROM dora_group_brand_map
		WHERE tenant_id = $1 AND group_id = $2
	`, tenantID, groupID).Scan(&lastSyncedAt)
	if err != nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]any{"error": "group map stale or unavailable", "code": "group_map_stale"})
	}

	if h.nowFn().UTC().Sub(lastSyncedAt) > h.groupMapTTL {
		return c.JSON(http.StatusServiceUnavailable, map[string]any{"error": "group map stale or unavailable", "code": "group_map_stale"})
	}

	return nil
}

func percentDelta(previous, current float64) any {
	if previous == 0 {
		if current == 0 {
			return 0.0
		}
		return nil
	}
	return ((current - previous) / previous) * 100.0
}

func leadTimeDelta(previous, current *float64) any {
	if previous == nil || current == nil {
		return nil
	}
	return percentDelta(*previous, *current)
}

func currentOrNil(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}

func nullableString(v string) any {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	return v
}

func boolToFloat(v bool) float64 {
	if v {
		return 1.0
	}
	return 0.0
}

func parseStoredHeaderMap(headersJSON []byte) (map[string]string, error) {
	if len(headersJSON) == 0 {
		return map[string]string{}, nil
	}
	var generic map[string]any
	if err := json.Unmarshal(headersJSON, &generic); err != nil {
		return nil, err
	}
	var multi map[string][]string
	if err := json.Unmarshal(headersJSON, &multi); err == nil {
		flat := make(map[string]string, len(multi))
		for k, values := range multi {
			if len(values) > 0 {
				flat[strings.ToLower(strings.TrimSpace(k))] = values[0]
			}
		}
		return flat, nil
	}

	var flat map[string]string
	if err := json.Unmarshal(headersJSON, &flat); err == nil {
		out := make(map[string]string, len(flat))
		for k, v := range flat {
			out[strings.ToLower(strings.TrimSpace(k))] = v
		}
		return out, nil
	}

	return nil, fmt.Errorf("unsupported stored header format")
}

func (h *DoraHandler) queryLeadTimeStats(c echo.Context, conn db.Conn, tenantID, serviceRef string, start, end time.Time) (leadTimeStats, error) {
	stats := leadTimeStats{}

	if err := conn.QueryRow(c.Request().Context(), `
		SELECT COUNT(*)
		FROM dora_events
		WHERE tenant_id = $1
		  AND service_ref = $2
		  AND event_type = 'deployment.succeeded'
		  AND event_timestamp >= $3
		  AND event_timestamp < $4
	`, tenantID, serviceRef, start, end).Scan(&stats.SuccessfulDeployments); err != nil {
		return leadTimeStats{}, err
	}

	if err := conn.QueryRow(c.Request().Context(), `
		SELECT COUNT(DISTINCT deployment_id)
		FROM dora_commit_deployment_links
		WHERE tenant_id = $1
		  AND service_ref = $2
		  AND deployment_outcome = 'succeeded'
		  AND deployment_time >= $3
		  AND deployment_time < $4
	`, tenantID, serviceRef, start, end).Scan(&stats.CorrelatedDeployments); err != nil {
		return leadTimeStats{}, err
	}

	var p50, p95 *float64
	if err := conn.QueryRow(c.Request().Context(), `
		WITH deployments AS (
			SELECT id, event_timestamp AS deployment_time
			FROM dora_events
			WHERE tenant_id = $1
			  AND service_ref = $2
			  AND event_type = 'deployment.succeeded'
			  AND event_timestamp >= $3
			  AND event_timestamp < $4
		),
		first_commit AS (
			SELECT tenant_id, service_ref, correlation_key AS commit_sha, MIN(event_timestamp) AS first_commit_time
			FROM dora_events
			WHERE tenant_id = $1
			  AND service_ref = $2
			  AND event_type IN ('commit.pushed', 'commit.merged')
			  AND correlation_key IS NOT NULL
			  AND event_timestamp < $4
			GROUP BY tenant_id, service_ref, correlation_key
		),
		per_deployment AS (
			SELECT
				d.id AS deployment_id,
				EXTRACT(EPOCH FROM (d.deployment_time - MIN(fc.first_commit_time))) AS lead_seconds
			FROM deployments d
			JOIN dora_commit_deployment_links l
			  ON l.deployment_id = d.id
			 AND l.tenant_id = $1
			 AND l.service_ref = $2
			 AND l.deployment_outcome = 'succeeded'
			JOIN first_commit fc
			  ON fc.tenant_id = l.tenant_id
			 AND fc.service_ref = l.service_ref
			 AND fc.commit_sha = l.commit_sha
			GROUP BY d.id, d.deployment_time
		)
		SELECT
			percentile_cont(0.5) WITHIN GROUP (ORDER BY lead_seconds),
			percentile_cont(0.95) WITHIN GROUP (ORDER BY lead_seconds)
		FROM per_deployment
	`, tenantID, serviceRef, start, end).Scan(&p50, &p95); err != nil {
		return leadTimeStats{}, err
	}

	stats.P50 = p50
	stats.P95 = p95
	return stats, nil
}

func (h *DoraHandler) queryCFRStats(c echo.Context, conn db.Conn, tenantID, serviceRef string, start, end time.Time) (cfrStats, error) {
	stats := cfrStats{}

	if err := conn.QueryRow(c.Request().Context(), `
		SELECT COUNT(*)
		FROM dora_events
		WHERE tenant_id = $1
		  AND service_ref = $2
		  AND event_type IN ('deployment.succeeded', 'deployment.failed')
		  AND event_timestamp >= $3
		  AND event_timestamp < $4
	`, tenantID, serviceRef, start, end).Scan(&stats.DeploymentCount); err != nil {
		return cfrStats{}, err
	}

	if err := conn.QueryRow(c.Request().Context(), `
		SELECT COUNT(*)
		FROM dora_events
		WHERE tenant_id = $1
		  AND service_ref = $2
		  AND event_type = 'incident.opened'
		  AND event_timestamp >= $3
		  AND event_timestamp < $4
	`, tenantID, serviceRef, start, end).Scan(&stats.IncidentOpenedCount); err != nil {
		return cfrStats{}, err
	}

	if stats.DeploymentCount == 0 {
		return stats, nil
	}

	if err := conn.QueryRow(c.Request().Context(), `
		WITH deployments AS (
			SELECT id, event_timestamp, correlation_key
			FROM dora_events
			WHERE tenant_id = $1
			  AND service_ref = $2
			  AND event_type IN ('deployment.succeeded', 'deployment.failed')
			  AND event_timestamp >= $3
			  AND event_timestamp < $4
		),
		explicit_match AS (
			SELECT DISTINCT d.id
			FROM deployments d
			JOIN dora_events i
			  ON i.tenant_id = $1
			 AND i.service_ref = $2
			 AND i.event_type = 'incident.opened'
			 AND i.event_timestamp >= d.event_timestamp
			 AND i.event_timestamp < $4
			 AND i.correlation_key IS NOT NULL
			 AND d.correlation_key IS NOT NULL
			 AND i.correlation_key = d.correlation_key
		),
		heuristic_match AS (
			SELECT DISTINCT d.id
			FROM deployments d
			JOIN dora_events i
			  ON i.tenant_id = $1
			 AND i.service_ref = $2
			 AND i.event_type = 'incident.opened'
			 AND i.event_timestamp >= d.event_timestamp
			 AND i.event_timestamp <= d.event_timestamp + interval '1 hour'
			 AND i.event_timestamp < $4
			LEFT JOIN explicit_match em ON em.id = d.id
			WHERE em.id IS NULL
		)
		SELECT
			(SELECT COUNT(*) FROM explicit_match),
			(SELECT COUNT(*) FROM heuristic_match)
	`, tenantID, serviceRef, start, end).Scan(&stats.ExplicitCorrelations, &stats.HeuristicCorrelations); err != nil {
		return cfrStats{}, err
	}

	stats.CorrelatedDeployments = stats.ExplicitCorrelations + stats.HeuristicCorrelations
	v := (float64(stats.CorrelatedDeployments) / float64(stats.DeploymentCount)) * 100.0
	stats.Percent = &v

	if stats.IncidentOpenedCount == 0 {
		var failedCount int64
		if err := conn.QueryRow(c.Request().Context(), `
			SELECT COUNT(*)
			FROM dora_events
			WHERE tenant_id = $1
			  AND service_ref = $2
			  AND event_type = 'deployment.failed'
			  AND event_timestamp >= $3
			  AND event_timestamp < $4
		`, tenantID, serviceRef, start, end).Scan(&failedCount); err != nil {
			return cfrStats{}, err
		}
		proxy := (float64(failedCount) / float64(stats.DeploymentCount)) * 100.0
		stats.ProxyFailureRatePercent = &proxy
		stats.Percent = &proxy
	}

	return stats, nil
}

func (h *DoraHandler) queryMTTRStats(c echo.Context, conn db.Conn, tenantID, serviceRef string, start, end time.Time) (mttrStats, error) {
	stats := mttrStats{}

	if err := conn.QueryRow(c.Request().Context(), `
		SELECT COUNT(*)
		FROM dora_events
		WHERE tenant_id = $1
		  AND service_ref = $2
		  AND event_type = 'incident.opened'
		  AND event_timestamp >= $3
		  AND event_timestamp < $4
	`, tenantID, serviceRef, start, end).Scan(&stats.OpenedCount); err != nil {
		return mttrStats{}, err
	}

	if err := conn.QueryRow(c.Request().Context(), `
		SELECT COALESCE(SUM(resolved_incident_count), 0),
		       percentile_cont(0.5) WITHIN GROUP (ORDER BY p50_restore_seconds)
		FROM dora_incidents_daily
		WHERE tenant_id = $1
		  AND service_ref = $2
		  AND bucket >= date_trunc('day', $3)
		  AND bucket < date_trunc('day', $4)
	`, tenantID, serviceRef, start, end).Scan(&stats.ResolvedPairCount, &stats.P50); err != nil {
		return mttrStats{}, err
	}

	return stats, nil
}

func cfrQuality(stats cfrStats) string {
	if stats.DeploymentCount == 0 {
		return "no_data"
	}
	if stats.IncidentOpenedCount == 0 {
		if stats.ProxyFailureRatePercent != nil {
			return "proxy"
		}
		return "no_data"
	}
	if stats.CorrelatedDeployments == 0 {
		return "partial"
	}
	if stats.HeuristicCorrelations > 0 {
		return "partial"
	}
	if stats.ExplicitCorrelations > 0 {
		return "complete"
	}
	return "partial"
}

func mttrQuality(stats mttrStats) string {
	if stats.OpenedCount == 0 {
		return "no_data"
	}
	if stats.ResolvedPairCount == 0 || stats.P50 == nil {
		return "partial"
	}
	if stats.ResolvedPairCount < stats.OpenedCount {
		return "partial"
	}
	return "complete"
}

func cfrProxyDescription(stats cfrStats) any {
	if stats.ProxyFailureRatePercent == nil {
		return nil
	}
	return "computed from deployment failure rate because no incident.opened events were available"
}

func normalizeFailureResponse(errorCode, detail, deadLetterID string) map[string]any {
	resp := map[string]any{
		"accepted":     true,
		"processed":    false,
		"error_code":   errorCode,
		"error_detail": detail,
	}
	if deadLetterID != "" {
		resp["dead_letter_id"] = deadLetterID
	}
	return resp
}

func (h *DoraHandler) failWithDeadLetter(c echo.Context, tenantID, provider, requestedCode, detail, headersJSON string, body []byte) map[string]any {
	deadLetterID := h.writeDeadLetter(c, tenantID, provider, requestedCode, headersJSON, body, detail)
	if deadLetterID == "" {
		return normalizeFailureResponse("store_error", detail, "")
	}
	return normalizeFailureResponse(requestedCode, detail, deadLetterID)
}

func headerMaps(c echo.Context) (string, map[string]string) {
	headersMulti := map[string][]string{}
	headersFlat := map[string]string{}
	for k, values := range c.Request().Header {
		key := strings.ToLower(k)
		headersMulti[key] = append([]string(nil), values...)
		if len(values) > 0 {
			headersFlat[key] = values[0]
		}
	}
	b, _ := json.Marshal(headersMulti)
	return string(b), headersFlat
}

func (h *DoraHandler) writeDeadLetter(c echo.Context, tenantID, provider, errorCode, headersJSON string, body []byte, detail string) string {
	if h.metrics != nil {
		h.metrics.RecordDoraDeadLetter(tenantID, provider, errorCode)
	}
	if h.pool == nil {
		return ""
	}
	conn, err := h.pool.Acquire(c.Request().Context())
	if err != nil {
		return ""
	}
	defer conn.Release()

	failureReason := map[string]string{"error_code": errorCode, "error_detail": detail}
	failureReasonJSON, _ := json.Marshal(failureReason)

	var id string
	err = conn.QueryRow(c.Request().Context(), `
		INSERT INTO dora_webhook_dead_letter (tenant_id, provider, source_event_id, headers, body, failure_reason)
		VALUES ($1, $2, NULL, $3::jsonb, $4, $5)
		RETURNING id::text
	`, tenantID, provider, headersJSON, body, string(failureReasonJSON)).Scan(&id)
	if err != nil {
		return ""
	}
	return id
}
