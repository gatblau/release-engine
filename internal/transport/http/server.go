// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gatblau/release-engine/internal/db"
	"github.com/gatblau/release-engine/internal/registry"
	"github.com/gatblau/release-engine/internal/secrets"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"
)

// Server defines the HTTP server interface.
type Server interface {
	RegisterRoutes()
	Start(ctx context.Context) error
	Shutdown(ctx context.Context) error
}

type server struct {
	e                     *echo.Echo
	logger                *zap.Logger
	port                  int
	pool                  db.Pool
	secretsHandler        *SecretsHandler
	moduleRegistry        registry.ModuleRegistry
	oidcIssuer            string
	oidcAudience          string
	adminToken            string
	allowPrivateCallbacks bool
}

// NewServer creates a new HTTP server.
func NewServer(port int, logger *zap.Logger, moduleRegistry registry.ModuleRegistry, oidcIssuer, oidcAudience string) Server {
	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Recover())
	e.Use(middleware.RequestID())
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogStatus: true,
		LogMethod: true,
		LogURI:    true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			return nil
		},
	}))

	return &server{
		e:                     e,
		logger:                logger,
		port:                  port,
		pool:                  nil,
		moduleRegistry:        moduleRegistry,
		oidcIssuer:            oidcIssuer,
		oidcAudience:          oidcAudience,
		allowPrivateCallbacks: parseBoolEnv("RE_ALLOW_PRIVATE_CALLBACKS"),
	}
}

// NewServerWithSecrets creates a new HTTP server with secrets management support.
func resolveAdminToken() string {
	if token := os.Getenv("ADMIN_TOKEN"); token != "" {
		return token
	}
	if secretID := os.Getenv("ADMIN_TOKEN_SM_SECRET_ID"); secretID != "" {
		// Fallback to AWS secret if an env token is not provided.
		// This keeps prod secure while allowing e2e to inject a plain env token.
		if strings.TrimSpace(secretID) != "" {
			// TODO: hook up AWS Secrets Manager when the secrets client abstraction is available here.
			return ""
		}
	}
	return ""
}

func NewServerWithSecrets(port int, logger *zap.Logger, pool db.Pool, voltaManager *secrets.Manager, moduleRegistry registry.ModuleRegistry, oidcIssuer, oidcAudience string) Server {
	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Recover())
	e.Use(middleware.RequestID())
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogStatus: true,
		LogMethod: true,
		LogURI:    true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			return nil
		},
	}))

	return &server{
		e:                     e,
		logger:                logger,
		port:                  port,
		pool:                  pool,
		secretsHandler:        NewSecretsHandler(voltaManager, logger),
		moduleRegistry:        moduleRegistry,
		oidcIssuer:            oidcIssuer,
		oidcAudience:          oidcAudience,
		adminToken:            resolveAdminToken(),
		allowPrivateCallbacks: parseBoolEnv("RE_ALLOW_PRIVATE_CALLBACKS"),
	}
}

func (s *server) RegisterRoutes() {
	jobHandler := newJobHandler(s.pool, NewApprovalService(NewPolicyEngine()), s.allowPrivateCallbacks)
	doraHandler := NewDoraHandler(nil)

	// Create query handler if module registry is available
	var queryHandler *QueryHandler
	if s.moduleRegistry != nil {
		queryHandler = NewQueryHandler(s.moduleRegistry, s.logger)
	}

	s.e.GET("/healthz", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})
	s.e.GET("/readyz", func(c echo.Context) error {
		// TODO: Implement readiness check
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	// Rate limiting configuration: 100 requests per second, burst of 150
	rateLimiterConfig := 100.0
	rateLimiterRefill := 100.0

	api := s.e.Group("/v1",
		NewAuthMiddleware(s.oidcIssuer, s.oidcAudience, s.logger),
		NewRateLimiterMiddleware(rateLimiterConfig, rateLimiterRefill, s.logger),
	)
	api.POST("/jobs", jobHandler.CreateJob)
	api.GET("/jobs", jobHandler.ListJobs)
	api.GET("/jobs/:id", jobHandler.GetJob)
	api.POST("/jobs/:id/cancel", jobHandler.CancelJob)
	api.POST("/jobs/:job_id/steps/:step_id/decisions", jobHandler.SubmitDecision)
	api.GET("/jobs/:job_id/steps/:step_id/approval-context", jobHandler.GetApprovalContext)
	api.GET("/dora/summary", doraHandler.GetSummary)
	api.GET("/dora/group/summary", doraHandler.GetGroupSummary)
	api.GET("/dora/deployments", doraHandler.GetDeployments)
	api.GET("/internal/dora/dead-letter", doraHandler.ListDeadLetter)
	api.GET("/internal/dora/dead-letter/:id", doraHandler.GetDeadLetter)
	api.POST("/internal/dora/dead-letter/:id/replay", doraHandler.ReplayDeadLetter)

	// secrets API
	api.PUT("/tenants/:tenant/secrets/:key", s.secretsHandler.SetSecret)
	api.GET("/tenants/:tenant/secrets", s.secretsHandler.ListSecrets)
	api.DELETE("/tenants/:tenant/secrets/:key", s.secretsHandler.DeleteSecret)
	if s.adminToken != "" {
		s.e.PUT("/internal/v1/platform/secrets/:key", func(c echo.Context) error {
			if !strings.HasPrefix(c.Request().Header.Get("Authorization"), "Bearer "+s.adminToken) {
				return c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized", Code: "AUTH_REQUIRED"})
			}
			key := c.Param("key")
			var req SetSecretRequest
			if err := c.Bind(&req); err != nil || req.Value == "" {
				return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request body", Code: "INVALID_REQUEST"})
			}
			if err := s.secretsHandler.storeSecret(c.Request().Context(), "platform", key, []byte(req.Value)); err != nil {
				return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error(), Code: "INTERNAL_ERROR"})
			}
			return c.JSON(http.StatusOK, SetSecretResponse{Key: key, TenantID: "platform", Status: "stored"})
		})
	}

	// Query API (only if module registry is available)
	if queryHandler != nil {
		api.GET("/modules", queryHandler.ListModules)
		api.GET("/modules/:module/describe", queryHandler.DescribeModule)
		api.GET("/query/:module/:query", queryHandler.ExecuteQuery)
	}

	// Inbound provider webhooks use provider-native authentication and are not
	// JWT-protected interactive API routes.
	s.e.POST("/v1/webhooks/dora/:provider", doraHandler.IngestWebhook)
}

func (s *server) Start(ctx context.Context) error {
	addr := fmt.Sprintf(":%d", s.port)
	if err := s.e.Start(addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return &HTTPError{Err: ErrHTTPBindFailed, Code: "HTTP_BIND_FAILED", Detail: map[string]string{"addr": addr, "error": err.Error()}}
	}
	return nil
}

func (s *server) Shutdown(ctx context.Context) error {
	if err := s.e.Shutdown(ctx); err != nil {
		return &HTTPError{Err: ErrHTTPShutdownTimeout, Code: "HTTP_SHUTDOWN_TIMEOUT", Detail: map[string]string{"error": err.Error()}}
	}
	return nil
}

func parseBoolEnv(key string) bool {
	v := os.Getenv(key)
	b, _ := strconv.ParseBool(v)
	return b
}
