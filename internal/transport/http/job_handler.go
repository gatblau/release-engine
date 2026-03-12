package http

import (
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

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type JobHandler struct {
	approvalService *ApprovalService
	mu              sync.Mutex
	idem            map[string]createJobRecord
}

func NewJobHandler() *JobHandler {
	return &JobHandler{
		approvalService: NewApprovalService(NewPolicyEngine()),
		idem:            make(map[string]createJobRecord),
	}
}

func NewJobHandlerWithApprovalService(approvalService *ApprovalService) *JobHandler {
	if approvalService == nil {
		approvalService = NewApprovalService(NewPolicyEngine())
	}
	return &JobHandler{approvalService: approvalService, idem: make(map[string]createJobRecord)}
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
		return c.JSON(http.StatusRequestEntityTooLarge, map[string]string{"error_code": "ERR_PAYLOAD_TOO_LARGE"})
	}

	var req createJobRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request body", Code: "INVALID_REQUEST"})
	}

	if strings.TrimSpace(req.TenantID) == "" || strings.TrimSpace(req.PathKey) == "" {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "tenant_id and path_key are required", Code: "INVALID_REQUEST"})
	}
	if !validIdempotencyKey(req.IdempotencyKey) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error_code": "ERR_INVALID_IDEMPOTENCY_KEY"})
	}
	if req.CallbackURL != "" {
		if err := validateCallbackURL(req.CallbackURL); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error_code": "ERR_INVALID_CALLBACK_URL"})
		}
	}

	if req.MaxAttempts < 1 {
		req.MaxAttempts = 3
	}
	nextRunAt := time.Now().UTC()
	if req.FirstRunAt != nil {
		nextRunAt = req.FirstRunAt.UTC()
	}

	fingerprint, err := createJobFingerprint(req)
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request", Code: "INVALID_REQUEST"})
	}

	idemKey := fmt.Sprintf("%s|%s|%s", req.TenantID, req.PathKey, req.IdempotencyKey)

	h.mu.Lock()
	defer h.mu.Unlock()

	if rec, ok := h.idem[idemKey]; ok {
		c.Response().Header().Set("Idempotency-Key", req.IdempotencyKey)
		if rec.fingerprint == fingerprint {
			c.Response().Header().Set("Idempotency-Replayed", "true")
			c.Response().Header().Set("Location", "/v1/jobs/"+rec.envelope.JobID)
			return c.JSON(rec.statusCode, rec.envelope)
		}
		return c.JSON(http.StatusConflict, map[string]any{
			"error_code": "ERR_IDEM_CONFLICT",
			"conflict":   true,
			"canonical":  rec.envelope,
			"message":    "idempotency key reused with different payload",
		})
	}

	env := createJobEnvelope{
		JobID:       uuid.NewString(),
		TenantID:    req.TenantID,
		PathKey:     req.PathKey,
		State:       "queued",
		Attempt:     0,
		MaxAttempts: req.MaxAttempts,
		NextRunAt:   &nextRunAt,
		AcceptedAt:  time.Now().UTC(),
	}

	h.idem[idemKey] = createJobRecord{fingerprint: fingerprint, statusCode: http.StatusAccepted, envelope: env}

	c.Response().Header().Set("Idempotency-Key", req.IdempotencyKey)
	c.Response().Header().Set("Idempotency-Replayed", "false")
	c.Response().Header().Set("Location", "/v1/jobs/"+env.JobID)
	return c.JSON(http.StatusAccepted, env)
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

func validateCallbackURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return errors.New("invalid callback url")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("invalid callback url scheme")
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
	return c.JSON(http.StatusOK, map[string]string{"id": id, "status": "pending"})
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

func (h *JobHandler) ListJobs(c echo.Context) error {
	stepStatus := c.QueryParam("step_status")
	if stepStatus == "waiting_approval" {
		tenantID := c.QueryParam("tenant")
		approverRole := c.QueryParam("approver_role")
		return c.JSON(http.StatusOK, map[string]any{
			"jobs": h.approvalService.ListPendingJobs(c.Request().Context(), tenantID, approverRole),
		})
	}

	return c.JSON(http.StatusOK, map[string]any{"jobs": []string{}})
}

func (h *JobHandler) GetApprovalContext(c echo.Context) error {
	jobID := c.Param("job_id")
	stepID := c.Param("step_id")

	ctx, err := h.approvalService.GetApprovalContext(c.Request().Context(), jobID, stepID)
	if err != nil {
		if errors.Is(err, ErrApprovalStepNotFound) {
			return c.JSON(http.StatusNotFound, ErrorResponse{Error: "job or step not found", Code: "NOT_FOUND"})
		}
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal error", Code: "INTERNAL_ERROR"})
	}

	return c.JSON(http.StatusOK, ctx)
}

func deriveDecisionIdempotencyKey(approver, stepID, runID string) string {
	payload := approver + ":" + stepID + ":" + runID
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}
