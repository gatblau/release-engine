package http

import (
	"context"
	"fmt"
	"net/http"

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
	e      *echo.Echo
	logger *zap.Logger
	port   int
}

// NewServer creates a new HTTP server.
func NewServer(port int, logger *zap.Logger) Server {
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
		e:      e,
		logger: logger,
		port:   port,
	}
}

func (s *server) RegisterRoutes() {
	jobHandler := NewJobHandler()

	s.e.GET("/healthz", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})
	s.e.GET("/readyz", func(c echo.Context) error {
		// TODO: Implement readiness check
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	api := s.e.Group("/v1", AuthMiddleware, RateLimiter)
	api.POST("/jobs", jobHandler.CreateJob)
	api.GET("/jobs/:id", jobHandler.GetJob)
	api.POST("/jobs/:id/cancel", jobHandler.CancelJob)
}

func (s *server) Start(ctx context.Context) error {
	addr := fmt.Sprintf(":%d", s.port)
	if err := s.e.Start(addr); err != nil && err != http.ErrServerClosed {
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
