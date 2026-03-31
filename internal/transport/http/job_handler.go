package http

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gatblau/release-engine/internal/db"
	"github.com/google/uuid"
	"github.com/gorhill/cronexpr"
	"github.com/jackc/pgx/v4"
	"github.com/labstack/echo/v4"
)

type JobHandler struct {
	approvalService       *ApprovalService
	pool                  db.Pool
	nowFn                 func() time.Time
	allowPrivateCallbacks bool
	mu                    sync.Mutex
	idem                  map[string]createJobRecord
}

type jobRecord struct {
	ID               string    `json:"id"`
	TenantID         string    `json:"tenant_id"`
	PathKey          string    `json:"path_key"`
	State            string    `json:"state"`
	Attempt          int       `json:"attempt"`
	RunID            string    `json:"run_id"`
	LastErrorCode    string    `json:"last_error_code,omitempty"`
	LastErrorMessage string    `json:"last_error_message,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

type pendingApprovalJobRecord struct {
	JobID      string `json:"job_id"`
	TenantID   string `json:"tenant_id"`
	PathKey    string `json:"path_key"`
	StepID     string `json:"step_id"`
	StepStatus string `json:"step_status"`
}

func NewJobHandler() *JobHandler {
	return newJobHandler(nil, nil, false)
}

func NewJobHandlerWithPool(pool db.Pool) *JobHandler {
	return newJobHandler(pool, nil, false)
}

func NewJobHandlerWithApprovalService(approvalService *ApprovalService) *JobHandler {
	return newJobHandler(nil, approvalService, false)
}

func NewJobHandlerWithPrivateCallbacksAllowed(allow bool) *JobHandler {
	return newJobHandler(nil, nil, allow)
}

func newJobHandler(pool db.Pool, approvalService *ApprovalService, allowPrivateCallbacks bool) *JobHandler {
	if approvalService == nil {
		approvalService = NewApprovalService(NewPolicyEngine())
	}
	return &JobHandler{
		approvalService:       approvalService,
		pool:                  pool,
		nowFn:                 time.Now,
		allowPrivateCallbacks: allowPrivateCallbacks,
		idem:                  make(map[string]createJobRecord),
	}
}

type createJobRequest struct {
	TenantID       string                 `json:"tenant_id"`
	PathKey        string                 `json:"path_key"`
	Params         map[string]any         `json:"params"`
	IdempotencyKey string                 `json:"idempotency_key"`
	CallbackURL    string                 `json:"callback_url,omitempty"`
	MaxAttempts    int                    `json:"max_attempts,omitempty"`
	FirstRunAt     *time.Time             `json:"first_run_at,omitempty"`
	Schedule       string                 `json:"schedule,omitempty"`
	Raw            map[string]interface{} `json:"-"`
}

type createJobEnvelope struct {
	JobID       string     `json:"job_id"`
	TenantID    string     `json:"tenant_id"`
	PathKey     string     `json:"path_key"`
	State       string     `json:"state"`
	Attempt     int        `json:"attempt"`
	MaxAttempts int        `json:"max_attempts"`
	NextRunAt   *time.Time `json:"next_run_at"`
	AcceptedAt  time.Time  `json:"accepted_at"`
}

type createJobRecord struct {
	fingerprint string
	statusCode  int
	envelope    createJobEnvelope
}

const maxCreateJobBodyBytes = 256 * 1024

var idemKeyRegex = regexp.MustCompile(`^[a-zA-Z0-9\-_.]+$`)

func (h *JobHandler) CreateJob(c echo.Context) error {
	body, err := io.ReadAll(io.LimitReader(c.Request().Body, maxCreateJobBodyBytes+1))
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request body", Code: "INVALID_REQUEST"})
	}
	if len(body) > maxCreateJobBodyBytes {
		return c.JSON(http.StatusRequestEntityTooLarge, ErrorResponse{Error: "payload too large", Code: "ERR_PAYLOAD_TOO_LARGE"})
	}

	var req createJobRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request body", Code: "INVALID_REQUEST"})
	}

	req.TenantID = strings.TrimSpace(req.TenantID)
	req.PathKey = strings.TrimSpace(req.PathKey)
	if req.TenantID == "" || req.PathKey == "" {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "tenant_id and path_key are required", Code: "INVALID_REQUEST"})
	}
	if !validIdempotencyKey(req.IdempotencyKey) {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid idempotency key", Code: "ERR_INVALID_IDEMPOTENCY_KEY"})
	}
	if req.CallbackURL != "" {
		if err := validateCallbackURL(req.CallbackURL, h.allowPrivateCallbacks); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid callback url", Code: "ERR_INVALID_CALLBACK_URL"})
		}
	}

	req.Schedule = strings.TrimSpace(req.Schedule)
	if req.MaxAttempts < 1 {
		req.MaxAttempts = 3
	}
	nextRunAt := h.nextRunAt(req)
	if req.Schedule != "" {
		if err := validateSchedule(req.Schedule); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid schedule", Code: "ERR_INVALID_SCHEDULE"})
		}
	}

	fingerprint, err := createJobFingerprint(req)
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request", Code: "INVALID_REQUEST"})
	}

	idemKey := fmt.Sprintf("%s|%s|%s", req.TenantID, req.PathKey, req.IdempotencyKey)

	ctx := c.Request().Context()
	var rec createJobRecord
	var replayed bool
	var conflict bool
	if h.pool != nil {
		rec, replayed, conflict, err = h.createJobWithDB(ctx, req, fingerprint, nextRunAt)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "could not persist job", Code: "INTERNAL_ERROR"})
		}
	} else {
		env := createJobEnvelope{
			JobID:       uuid.NewString(),
			TenantID:    req.TenantID,
			PathKey:     req.PathKey,
			State:       "queued",
			Attempt:     0,
			MaxAttempts: req.MaxAttempts,
			NextRunAt:   &nextRunAt,
			AcceptedAt:  h.nowFn().UTC(),
		}
		h.mu.Lock()
		rec, replayed, conflict = h.handleInMemory(idemKey, fingerprint, env)
		h.mu.Unlock()
	}

	c.Response().Header().Set("Idempotency-Key", req.IdempotencyKey)
	if conflict {
		c.Response().Header().Set("Idempotency-Replayed", "false")
		c.Response().Header().Set("Location", "/v1/jobs/"+rec.envelope.JobID)
		return c.JSON(http.StatusConflict, map[string]any{
			"error":      "idempotency key reused with different payload",
			"error_code": "ERR_IDEM_CONFLICT",
			"conflict":   true,
			"canonical":  rec.envelope,
			"message":    "idempotency key reused with different payload",
		})
	}
	c.Response().Header().Set("Idempotency-Replayed", fmt.Sprintf("%t", replayed))
	c.Response().Header().Set("Location", "/v1/jobs/"+rec.envelope.JobID)
	return c.JSON(rec.statusCode, rec.envelope)
}

func validIdempotencyKey(key string) bool {
	if len(key) == 0 || len(key) > 128 {
		return false
	}
	return idemKeyRegex.MatchString(key)
}

func createJobFingerprint(req createJobRequest) (string, error) {
	payload := map[string]any{
		"path_key": req.PathKey,
		"params":   req.Params,
	}
	if req.CallbackURL != "" {
		payload["callback_url"] = req.CallbackURL
	}
	if req.Schedule != "" {
		payload["schedule"] = req.Schedule
	}
	if req.FirstRunAt != nil {
		payload["first_run_at"] = req.FirstRunAt.UTC().Format(time.RFC3339Nano)
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func (h *JobHandler) nextRunAt(req createJobRequest) time.Time {
	if req.FirstRunAt != nil {
		return req.FirstRunAt.UTC()
	}
	return h.nowFn().UTC()
}

func validateSchedule(schedule string) error {
	_, err := cronexpr.Parse(schedule)
	return err
}

func (h *JobHandler) handleInMemory(idemKey, fingerprint string, env createJobEnvelope) (createJobRecord, bool, bool) {
	if rec, ok := h.idem[idemKey]; ok {
		if rec.fingerprint == fingerprint {
			return rec, true, false
		}
		return rec, false, true
	}
	rec := createJobRecord{fingerprint: fingerprint, statusCode: http.StatusAccepted, envelope: env}
	h.idem[idemKey] = rec
	return rec, false, false
}

func (h *JobHandler) createJobWithDB(ctx context.Context, req createJobRequest, fingerprint string, nextRunAt time.Time) (createJobRecord, bool, bool, error) {
	conn, err := h.pool.Acquire(ctx)
	if err != nil {
		return createJobRecord{}, false, false, err
	}
	defer conn.Release()

	fingerprintBytes, err := hex.DecodeString(fingerprint)
	if err != nil {
		return createJobRecord{}, false, false, err
	}

	acceptedAt := h.nowFn().UTC()
	env := createJobEnvelope{
		JobID:       uuid.NewString(),
		TenantID:    req.TenantID,
		PathKey:     req.PathKey,
		State:       "queued",
		Attempt:     0,
		MaxAttempts: req.MaxAttempts,
		NextRunAt:   &nextRunAt,
		AcceptedAt:  acceptedAt,
	}
	envJSON, err := json.Marshal(env)
	if err != nil {
		return createJobRecord{}, false, false, err
	}

	insertIdemResult, err := conn.Exec(ctx, `
		INSERT INTO idempotency_keys (tenant_id, path_key, idempotency_key, payload_fingerprint, status_code, response_body_json, job_id, accepted_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7::uuid, $8, $9)
		ON CONFLICT (tenant_id, path_key, idempotency_key) DO NOTHING
	`, req.TenantID, req.PathKey, req.IdempotencyKey, fingerprintBytes, http.StatusAccepted, envJSON, env.JobID, acceptedAt, acceptedAt.Add(48*time.Hour))
	if err != nil {
		return createJobRecord{}, false, false, err
	}

	if insertIdemResult.RowsAffected() == 0 {
		rec, replayed, conflict, fetchErr := h.fetchExistingIdempotencyRecord(ctx, conn, req, fingerprintBytes)
		return rec, replayed, conflict, fetchErr
	}

	paramsJSON, err := json.Marshal(req.Params)
	if err != nil {
		return createJobRecord{}, false, false, err
	}

	var runID string
	err = conn.QueryRow(ctx, `
		INSERT INTO jobs (id, tenant_id, path_key, idempotency_key, params_json, schedule, state, attempt, max_attempts, next_run_at, accepted_at, created_at, updated_at)
		VALUES ($1::uuid, $2, $3, $4, $5::jsonb, $6, 'queued', 0, $7, $8, $9, $9, $9)
		RETURNING run_id::text
	`, env.JobID, req.TenantID, req.PathKey, req.IdempotencyKey, paramsJSON, nullableSchedule(req.Schedule), req.MaxAttempts, nextRunAt, acceptedAt).Scan(&runID)
	if err != nil {
		return createJobRecord{}, false, false, err
	}

	_, err = conn.Exec(ctx, `
		INSERT INTO jobs_read (
			id, tenant_id, path_key, state, attempt, max_attempts, schedule, owner_id, run_id,
			lease_expires_at, next_run_at, accepted_at, last_error_code, last_error_message,
			started_at, finished_at, created_at, updated_at
		) VALUES (
			$1::uuid, $2, $3, 'queued', 0, $4, $5, NULL, $6::uuid,
			NULL, $7, $8, NULL, NULL,
			NULL, NULL, $8, $8
		)
	`, env.JobID, req.TenantID, req.PathKey, req.MaxAttempts, nullableSchedule(req.Schedule), runID, nextRunAt, acceptedAt)
	if err != nil {
		return createJobRecord{}, false, false, err
	}

	return createJobRecord{fingerprint: fingerprint, statusCode: http.StatusAccepted, envelope: env}, false, false, nil
}

type queryConn interface {
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
}

func (h *JobHandler) fetchExistingIdempotencyRecord(ctx context.Context, conn queryConn, req createJobRequest, fingerprintBytes []byte) (createJobRecord, bool, bool, error) {
	var storedFingerprint []byte
	var statusCode int
	var responseBody []byte
	err := conn.QueryRow(ctx, `
		SELECT payload_fingerprint, status_code, response_body_json
		FROM idempotency_keys
		WHERE tenant_id = $1 AND path_key = $2 AND idempotency_key = $3
	`, req.TenantID, req.PathKey, req.IdempotencyKey).Scan(&storedFingerprint, &statusCode, &responseBody)
	if err != nil {
		return createJobRecord{}, false, false, err
	}

	var env createJobEnvelope
	if err := json.Unmarshal(responseBody, &env); err != nil {
		return createJobRecord{}, false, false, err
	}
	rec := createJobRecord{fingerprint: hex.EncodeToString(storedFingerprint), statusCode: statusCode, envelope: env}
	if bytes.Equal(storedFingerprint, fingerprintBytes) {
		return rec, true, false, nil
	}
	return rec, false, true, nil
}

func nullableSchedule(schedule string) any {
	if strings.TrimSpace(schedule) == "" {
		return nil
	}
	return strings.TrimSpace(schedule)
}

func validateCallbackURL(raw string, allowPrivateCallbacks bool) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return errors.New("invalid callback url")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("invalid callback url scheme")
	}
	if allowPrivateCallbacks {
		return nil
	}
	host := u.Hostname()
	if host == "localhost" {
		return errors.New("localhost is not allowed")
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return nil
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return errors.New("private callback ip is not allowed")
	}
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 10 ||
			(ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31) ||
			(ip4[0] == 192 && ip4[1] == 168) ||
			(ip4[0] == 169 && ip4[1] == 254) {
			return errors.New("private callback ip is not allowed")
		}
	}
	return nil
}

func (h *JobHandler) GetJob(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		id = c.Param("job_id")
	}
	if h.pool == nil {
		return c.JSON(http.StatusOK, map[string]string{"id": id, "status": "pending"})
	}

	conn, err := h.pool.Acquire(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "could not query job", Code: "INTERNAL_ERROR"})
	}
	defer conn.Release()

	var job jobRecord
	err = conn.QueryRow(c.Request().Context(), `
		SELECT id::text, tenant_id, path_key, state::text, attempt, run_id::text, COALESCE(last_error_code,''), COALESCE(last_error_message,''), created_at
		FROM jobs
		WHERE id = $1::uuid
	`, id).Scan(&job.ID, &job.TenantID, &job.PathKey, &job.State, &job.Attempt, &job.RunID, &job.LastErrorCode, &job.LastErrorMessage, &job.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return c.JSON(http.StatusNotFound, ErrorResponse{Error: "job not found", Code: "NOT_FOUND"})
		}
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "could not query job", Code: "INTERNAL_ERROR"})
	}

	return c.JSON(http.StatusOK, job)
}

func (h *JobHandler) CancelJob(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		id = c.Param("job_id")
	}
	return c.JSON(http.StatusOK, map[string]string{"id": id, "status": "cancelled"})
}

type decisionRequest struct {
	Decision       string `json:"decision"`
	Justification  string `json:"justification"`
	IdempotencyKey string `json:"idempotency_key"`
	RunID          string `json:"run_id,omitempty"`
}

func (h *JobHandler) SubmitDecision(c echo.Context) error {
	jobID := c.Param("job_id")
	stepID := c.Param("step_id")
	if jobID == "" || stepID == "" {
		return c.JSON(http.StatusNotFound, ErrorResponse{Error: "job or step not found", Code: "NOT_FOUND"})
	}

	var request decisionRequest
	if err := c.Bind(&request); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request body", Code: "INVALID_REQUEST"})
	}

	request.Decision = strings.ToLower(strings.TrimSpace(request.Decision))
	if request.Decision != "approved" && request.Decision != "rejected" {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid decision", Code: "INVALID_DECISION"})
	}

	claims, _ := GetAuthClaims(c)
	approver := claims.Subject
	if approver == "" {
		approver = c.Request().Header.Get("X-Approver")
	}
	approverRole := claims.Role
	if approverRole == "" {
		approverRole = c.Request().Header.Get("X-Approver-Role")
	}
	approverTenantID := claims.TenantID
	if approverTenantID == "" {
		approverTenantID = c.Request().Header.Get("X-Approver-Tenant")
	}
	if approver == "" {
		approver = "anonymous"
	}

	if request.IdempotencyKey == "" {
		runID := request.RunID
		if runID == "" {
			runID = c.Request().Header.Get("X-Run-ID")
		}
		request.IdempotencyKey = deriveDecisionIdempotencyKey(approver, stepID, runID)
	}

	if h.pool == nil {
		result, err := h.approvalService.SubmitDecision(c.Request().Context(), DecisionInput{
			JobID:            jobID,
			StepID:           stepID,
			Decision:         request.Decision,
			Justification:    request.Justification,
			IdempotencyKey:   request.IdempotencyKey,
			Approver:         approver,
			ApproverRole:     approverRole,
			ApproverTenantID: approverTenantID,
		})
		if err != nil {
			switch {
			case errors.Is(err, ErrApprovalStepNotFound):
				return c.JSON(http.StatusNotFound, ErrorResponse{Error: "job or step not found", Code: "NOT_FOUND"})
			case errors.Is(err, ErrApprovalNotWaiting), errors.Is(err, ErrApprovalDecisionConflict), errors.Is(err, ErrApprovalIdempotencyConflict):
				return c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error(), Code: "CONFLICT"})
			case errors.Is(err, ErrApprovalForbidden):
				return c.JSON(http.StatusForbidden, ErrorResponse{Error: "approver not authorised", Code: "FORBIDDEN"})
			case errors.Is(err, ErrApprovalPolicyViolation):
				return c.JSON(http.StatusUnprocessableEntity, ErrorResponse{Error: "policy violation", Code: "POLICY_VIOLATION"})
			default:
				return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal error", Code: "INTERNAL_ERROR"})
			}
		}
		return c.JSON(http.StatusOK, result)
	}

	// SQL strings below are resolved at runtime against the configured database.

	conn, err := h.pool.Acquire(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal error", Code: "INTERNAL_ERROR"})
	}
	defer conn.Release()

	var stepStatus string
	var runID string
	var tenantID string
	var pathKey string
	var ownerID string
	err = conn.QueryRow(c.Request().Context(), `
		SELECT s.status::text, s.run_id::text, j.tenant_id, j.path_key, COALESCE(j.created_by, '')
		FROM steps s
		JOIN jobs j ON j.id = s.job_id
		WHERE s.job_id = $1::uuid AND s.id = $2::bigint
	`, jobID, stepID).Scan(&stepStatus, &runID, &tenantID, &pathKey, &ownerID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return c.JSON(http.StatusNotFound, ErrorResponse{Error: "job or step not found", Code: "NOT_FOUND"})
		}
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal error", Code: "INTERNAL_ERROR"})
	}
	if stepStatus != "waiting_approval" {
		return c.JSON(http.StatusConflict, ErrorResponse{Error: ErrApprovalNotWaiting.Error(), Code: "CONFLICT"})
	}

	policyResult := h.approvalService.policyEngine.EvaluateApproval(ApprovalPolicyInput{
		Approver:         approver,
		ApproverRole:     approverRole,
		ApproverTenantID: approverTenantID,
		JobOwner:         ownerID,
		JobTenantID:      tenantID,
		PathKey:          pathKey,
		StepKey:          stepID,
		AllowedRoles:     []string{"release-manager", "team-lead"},
		SelfApproval:     false,
		MinApprovers:     1,
	})
	if !policyResult.Allowed {
		if contains(policyResult.Violations, "self_approval_blocked") || contains(policyResult.Violations, "budget_exceeded") {
			return c.JSON(http.StatusUnprocessableEntity, ErrorResponse{Error: "policy violation", Code: "POLICY_VIOLATION"})
		}
		return c.JSON(http.StatusForbidden, ErrorResponse{Error: "approver not authorised", Code: "FORBIDDEN"})
	}

	if request.IdempotencyKey == "" {
		request.IdempotencyKey = deriveDecisionIdempotencyKey(approver, stepID, runID)
	}

	var stepPK int64
	err = conn.QueryRow(c.Request().Context(), `SELECT id FROM steps WHERE job_id = $1::uuid AND id = $2::bigint`, jobID, stepID).Scan(&stepPK)
	if err != nil {
		return c.JSON(http.StatusNotFound, ErrorResponse{Error: "job or step not found", Code: "NOT_FOUND"})
	}

	var insertedID string
	err = conn.QueryRow(c.Request().Context(), `
		INSERT INTO approval_decisions (job_id, step_id, run_id, decision, approver, justification, policy_snapshot, idempotency_key)
		VALUES ($1::uuid, $2::bigint, $3::uuid, $4::approval_decision, $5, $6, '{}'::jsonb, $7)
		ON CONFLICT (idempotency_key) DO UPDATE SET job_id = EXCLUDED.job_id
		RETURNING id::text
	`, jobID, stepPK, runID, request.Decision, approver, request.Justification, request.IdempotencyKey).Scan(&insertedID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal error", Code: "INTERNAL_ERROR"})
	}

	return c.JSON(http.StatusOK, DecisionOutput{
		JobID:              jobID,
		StepID:             stepID,
		Decision:           request.Decision,
		State:              "running",
		IdempotencyKey:     request.IdempotencyKey,
		IdempotentReplay:   false,
		RemainingApprovals: 0,
		RecordedAt:         time.Now().UTC(),
	})
}

func (h *JobHandler) ListJobs(c echo.Context) error {
	stepStatus := c.QueryParam("step_status")
	if stepStatus == "waiting_approval" {
		tenantID := c.QueryParam("tenant")
		if h.pool == nil {
			approverRole := c.QueryParam("approver_role")
			return c.JSON(http.StatusOK, map[string]any{
				"jobs": h.approvalService.ListPendingJobs(c.Request().Context(), tenantID, approverRole),
			})
		}

		conn, err := h.pool.Acquire(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "could not query pending approvals", Code: "INTERNAL_ERROR"})
		}
		defer conn.Release()

		rows, err := conn.Query(c.Request().Context(), `
			SELECT j.id::text, j.tenant_id, j.path_key, s.id::text, s.status::text
			FROM jobs j
			JOIN steps s ON s.job_id = j.id
			WHERE s.status = 'waiting_approval'
			  AND ($1 = '' OR j.tenant_id = $1)
			ORDER BY j.id, s.id
		`, tenantID)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "could not query pending approvals", Code: "INTERNAL_ERROR"})
		}
		defer rows.Close()

		jobs := make([]pendingApprovalJobRecord, 0)
		for rows.Next() {
			var rec pendingApprovalJobRecord
			if err := rows.Scan(&rec.JobID, &rec.TenantID, &rec.PathKey, &rec.StepID, &rec.StepStatus); err != nil {
				return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "could not query pending approvals", Code: "INTERNAL_ERROR"})
			}
			jobs = append(jobs, rec)
		}
		return c.JSON(http.StatusOK, map[string]any{"jobs": jobs})
	}

	return c.JSON(http.StatusOK, map[string]any{"jobs": []string{}})
}

func (h *JobHandler) GetApprovalContext(c echo.Context) error {
	jobID := c.Param("job_id")
	stepID := c.Param("step_id")
	if h.pool == nil {
		ctx, err := h.approvalService.GetApprovalContext(c.Request().Context(), jobID, stepID)
		if err != nil {
			if errors.Is(err, ErrApprovalStepNotFound) {
				return c.JSON(http.StatusNotFound, ErrorResponse{Error: "job or step not found", Code: "NOT_FOUND"})
			}
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal error", Code: "INTERNAL_ERROR"})
		}
		return c.JSON(http.StatusOK, ctx)
	}

	conn, err := h.pool.Acquire(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal error", Code: "INTERNAL_ERROR"})
	}
	defer conn.Release()

	var stepStatus string
	var approvalRequest []byte
	var approvalTTL any
	var expiresAt *time.Time
	var tenantID, pathKey, runID string
	err = conn.QueryRow(c.Request().Context(), `
		SELECT s.status::text, s.approval_request, s.approval_ttl, s.approval_expires_at, j.tenant_id, j.path_key, s.run_id::text
		FROM steps s
		JOIN jobs j ON j.id = s.job_id
		WHERE s.job_id = $1::uuid AND s.id = $2::bigint
	`, jobID, stepID).Scan(&stepStatus, &approvalRequest, &approvalTTL, &expiresAt, &tenantID, &pathKey, &runID)
	if err != nil {
		if errors.Is(err, ErrApprovalStepNotFound) {
			return c.JSON(http.StatusNotFound, ErrorResponse{Error: "job or step not found", Code: "NOT_FOUND"})
		}
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal error", Code: "INTERNAL_ERROR"})
	}

	var ctxResp ApprovalContextResponse
	_ = json.Unmarshal(approvalRequest, &ctxResp.Context)
	ctxResp.JobID = jobID
	ctxResp.StepID = stepID
	ctxResp.Status = stepStatus
	ctxResp.ExpiresAt = expiresAt
	ctxResp.Policy = ApprovalPolicy{Required: true, MinApprovers: 1, AllowedRoles: []string{"release-manager", "team-lead"}, SelfApproval: false}
	ctxResp.Tally = ApprovalTally{RemainingToApprove: 1}
	_ = tenantID
	_ = pathKey
	_ = runID
	return c.JSON(http.StatusOK, ctxResp)
}

func deriveDecisionIdempotencyKey(approver, stepID, runID string) string {
	payload := approver + ":" + stepID + ":" + runID
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}
