// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package http

import (
	"net/http"

	"github.com/gatblau/release-engine/internal/registry"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// QueryHandler handles module query API requests.
type QueryHandler struct {
	moduleRegistry registry.ModuleRegistry
	logger         *zap.Logger
}

// NewQueryHandler creates a new query handler.
func NewQueryHandler(moduleRegistry registry.ModuleRegistry, logger *zap.Logger) *QueryHandler {
	return &QueryHandler{
		moduleRegistry: moduleRegistry,
		logger:         logger,
	}
}

// ExecuteQuery handles GET /query/:module/:query requests.
func (h *QueryHandler) ExecuteQuery(c echo.Context) error {
	moduleName := c.Param("module")
	queryName := c.Param("query")

	// Look up the module by name (using "latest" version for now)
	module, ok := h.moduleRegistry.Lookup(moduleName, "latest")
	if !ok {
		return c.JSON(http.StatusNotFound, ErrorResponse{
			Error: "module not found",
			Code:  "MODULE_NOT_FOUND",
		})
	}

	// Parse query parameters
	params := make(map[string]any)
	for k, v := range c.QueryParams() {
		if len(v) > 0 {
			params[k] = v[0]
		}
	}

	// Execute the query
	result, err := module.Query(c.Request().Context(), nil, registry.QueryRequest{
		Name:   queryName,
		Params: params,
	})

	if err != nil {
		h.logger.Error("failed to execute query", zap.Error(err), zap.String("module", moduleName), zap.String("query", queryName))
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: err.Error(),
			Code:  "QUERY_EXECUTION_FAILED",
		})
	}

	// Return the query result
	return c.JSON(http.StatusOK, result)
}

// ListModules handles GET /modules requests.
func (h *QueryHandler) ListModules(c echo.Context) error {
	descriptors := h.moduleRegistry.ListModules()
	return c.JSON(http.StatusOK, descriptors)
}

// DescribeModule handles GET /modules/:module/describe requests.
func (h *QueryHandler) DescribeModule(c echo.Context) error {
	moduleName := c.Param("module")

	// Look up the module by name (using "latest" version for now)
	module, ok := h.moduleRegistry.Lookup(moduleName, "latest")
	if !ok {
		return c.JSON(http.StatusNotFound, ErrorResponse{
			Error: "module not found",
			Code:  "MODULE_NOT_FOUND",
		})
	}

	desc := module.Describe()
	return c.JSON(http.StatusOK, desc)
}
