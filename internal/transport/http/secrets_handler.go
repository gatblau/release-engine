// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gatblau/release-engine/internal/secrets"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// SecretsHandler handles secrets management API endpoints.
type SecretsHandler struct {
	voltaManager *secrets.Manager
	logger       *zap.Logger
}

// NewSecretsHandler creates a new SecretsHandler.
func NewSecretsHandler(voltaManager *secrets.Manager, logger *zap.Logger) *SecretsHandler {
	return &SecretsHandler{
		voltaManager: voltaManager,
		logger:       logger,
	}
}

// SetSecretRequest is the request body for setting a secret.
type SetSecretRequest struct {
	Value string `json:"value"`
}

// SetSecretResponse is the response for setting a secret.
type SetSecretResponse struct {
	Key      string `json:"key"`
	TenantID string `json:"tenant_id"`
	Status   string `json:"status"`
}

// ListSecretsResponse is the response for listing secret keys.
type ListSecretsResponse struct {
	TenantID string   `json:"tenant_id"`
	Keys     []string `json:"keys"`
}

// DeleteSecretResponse is the response for deleting a secret.
type DeleteSecretResponse struct {
	Key      string `json:"key"`
	TenantID string `json:"tenant_id"`
	Status   string `json:"status"`
}

// SetSecret handles PUT /api/v1/tenants/{tenant}/secrets/{key}
func (h *SecretsHandler) SetSecret(c echo.Context) error {
	ctx := c.Request().Context()
	tenantID := c.Param("tenant")
	key := c.Param("key")

	// Validate tenant and key
	if err := validateTenantAndKey(tenantID, key); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   err.Error(),
			Code:    "INVALID_REQUEST",
			Details: nil,
		})
	}

	// Check authorization
	claims, ok := GetAuthClaims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "authentication required",
			Code:    "AUTH_REQUIRED",
			Details: nil,
		})
	}

	if !h.isAuthorizedToSetSecret(claims, tenantID) {
		return c.JSON(http.StatusForbidden, ErrorResponse{
			Error:   "not authorized to set secret for this tenant",
			Code:    "FORBIDDEN",
			Details: nil,
		})
	}

	// Parse request body
	var req SetSecretRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid request body",
			Code:    "INVALID_REQUEST",
			Details: nil,
		})
	}

	if req.Value == "" {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "secret value cannot be empty",
			Code:    "INVALID_REQUEST",
			Details: nil,
		})
	}

	// Validate secret key against known connector declarations (if implemented)
	// This would check if the key matches patterns like "github-token", "aws-access-key", etc.
	// For now, we'll accept any key.

	// Store the secret
	if err := h.storeSecret(ctx, tenantID, key, []byte(req.Value)); err != nil {
		h.logger.Error("failed to store secret",
			zap.String("tenant", tenantID),
			zap.String("key", key),
			zap.Error(err))
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "failed to store secret",
			Code:    "INTERNAL_ERROR",
			Details: nil,
		})
	}

	// Audit logging
	h.logger.Info("secret stored",
		zap.String("tenant", tenantID),
		zap.String("key", key),
		zap.String("requester", claims.Subject))

	return c.JSON(http.StatusOK, SetSecretResponse{
		Key:      key,
		TenantID: tenantID,
		Status:   "stored",
	})
}

// ListSecrets handles GET /api/v1/tenants/{tenant}/secrets
func (h *SecretsHandler) ListSecrets(c echo.Context) error {
	ctx := c.Request().Context()
	tenantID := c.Param("tenant")

	// Validate tenant
	if tenantID == "" {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "tenant ID is required",
			Code:    "INVALID_REQUEST",
			Details: nil,
		})
	}

	// Check authorization
	claims, ok := GetAuthClaims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "authentication required",
			Code:    "AUTH_REQUIRED",
			Details: nil,
		})
	}

	if !h.isAuthorizedToListSecrets(claims, tenantID) {
		return c.JSON(http.StatusForbidden, ErrorResponse{
			Error:   "not authorized to list secrets for this tenant",
			Code:    "FORBIDDEN",
			Details: nil,
		})
	}

	// List secret keys
	keys, err := h.listSecretKeys(ctx, tenantID)
	if err != nil {
		h.logger.Error("failed to list secret keys",
			zap.String("tenant", tenantID),
			zap.Error(err))
		// For now, return empty list if tenant vault doesn't exist
		// In a real implementation, we'd differentiate between "not found" and other errors
		keys = []string{}
	}

	return c.JSON(http.StatusOK, ListSecretsResponse{
		TenantID: tenantID,
		Keys:     keys,
	})
}

// DeleteSecret handles DELETE /api/v1/tenants/{tenant}/secrets/{key}
func (h *SecretsHandler) DeleteSecret(c echo.Context) error {
	ctx := c.Request().Context()
	tenantID := c.Param("tenant")
	key := c.Param("key")

	// Validate tenant and key
	if err := validateTenantAndKey(tenantID, key); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   err.Error(),
			Code:    "INVALID_REQUEST",
			Details: nil,
		})
	}

	// Check authorization
	claims, ok := GetAuthClaims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "authentication required",
			Code:    "AUTH_REQUIRED",
			Details: nil,
		})
	}

	if !h.isAuthorizedToDeleteSecret(claims, tenantID) {
		return c.JSON(http.StatusForbidden, ErrorResponse{
			Error:   "not authorized to delete secret for this tenant",
			Code:    "FORBIDDEN",
			Details: nil,
		})
	}

	// Delete the secret
	if err := h.deleteSecret(ctx, tenantID, key); err != nil {
		h.logger.Error("failed to delete secret",
			zap.String("tenant", tenantID),
			zap.String("key", key),
			zap.Error(err))
		// Check if secret doesn't exist
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "does not exist") {
			return c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "secret not found",
				Code:    "NOT_FOUND",
				Details: nil,
			})
		}
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "failed to delete secret",
			Code:    "INTERNAL_ERROR",
			Details: nil,
		})
	}

	// Audit logging
	h.logger.Info("secret deleted",
		zap.String("tenant", tenantID),
		zap.String("key", key),
		zap.String("requester", claims.Subject))

	return c.JSON(http.StatusOK, DeleteSecretResponse{
		Key:      key,
		TenantID: tenantID,
		Status:   "deleted",
	})
}

// Helper functions

func validateTenantAndKey(tenantID, key string) error {
	if tenantID == "" {
		return errors.New("tenant ID is required")
	}
	if key == "" {
		return errors.New("secret key is required")
	}
	// Basic validation - could be more restrictive
	if strings.Contains(key, "..") || strings.Contains(key, "/") {
		return errors.New("invalid secret key")
	}
	return nil
}

func (h *SecretsHandler) isAuthorizedToSetSecret(claims AuthClaims, tenantID string) bool {
	// Platform tenant requires platform admin role
	if tenantID == "platform" {
		return claims.Role == "platform-admin" || containsGroup(claims.GroupIDs, "platform-admin")
	}
	// Non-platform tenants: any authenticated user may manage secrets.
	// Dex password-grant tokens do not carry group claims, so we rely on JWT authentication
	// and reserve explicit role enforcement only for the platform tenant.
	return claims.Subject != ""
}

func (h *SecretsHandler) isAuthorizedToListSecrets(claims AuthClaims, tenantID string) bool {
	// Platform tenant requires platform admin role
	if tenantID == "platform" {
		return claims.Role == "platform-admin" || containsGroup(claims.GroupIDs, "platform-admin")
	}
	// Non-platform tenants: any authenticated user may list secrets.
	return claims.Subject != ""
}

func (h *SecretsHandler) isAuthorizedToDeleteSecret(claims AuthClaims, tenantID string) bool {
	// Same authorization as set secret
	return h.isAuthorizedToSetSecret(claims, tenantID)
}

func containsGroup(groups []string, target string) bool {
	for _, group := range groups {
		if group == target {
			return true
		}
	}
	return false
}

// storeSecret stores a secret using the volta manager.
func (h *SecretsHandler) storeSecret(ctx context.Context, tenantID, key string, value []byte) error {
	// Get vault for tenant
	vault, err := h.voltaManager.GetVault(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("failed to get vault for tenant %s: %w", tenantID, err)
	}

	// Store the secret
	if err = vault.StoreSecret(key, value); err != nil {
		return fmt.Errorf("failed to store secret: %w", err)
	}
	return nil
}

// listSecretKeys lists all secret keys for a tenant.
func (h *SecretsHandler) listSecretKeys(ctx context.Context, tenantID string) ([]string, error) {
	// Get vault for tenant
	vault, err := h.voltaManager.GetVault(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get vault for tenant %s: %w", tenantID, err)
	}

	// List secrets
	keys, err := vault.ListSecrets()
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets: %w", err)
	}
	return keys, nil
}

// deleteSecret deletes a secret for a tenant.
func (h *SecretsHandler) deleteSecret(ctx context.Context, tenantID, key string) error {
	// Get vault for tenant
	vault, err := h.voltaManager.GetVault(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("failed to get vault for tenant %s: %w", tenantID, err)
	}

	// Delete the secret
	if err = vault.DeleteSecret(key); err != nil {
		return fmt.Errorf("failed to delete secret: %w", err)
	}
	return nil
}
