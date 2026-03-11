package http

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

type JobHandler struct {
	approvalService *ApprovalService
}

func NewJobHandler() *JobHandler {
	return &JobHandler{
		approvalService: NewApprovalService(NewPolicyEngine()),
	}
}

func NewJobHandlerWithApprovalService(approvalService *ApprovalService) *JobHandler {
	if approvalService == nil {
		approvalService = NewApprovalService(NewPolicyEngine())
	}
	return &JobHandler{approvalService: approvalService}
}

func (h *JobHandler) CreateJob(c echo.Context) error {
	return c.JSON(http.StatusAccepted, map[string]string{"message": "job accepted"})
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
