package http

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

type JobHandler struct{}

func NewJobHandler() *JobHandler {
	return &JobHandler{}
}

func (h *JobHandler) CreateJob(c echo.Context) error {
	return c.JSON(http.StatusCreated, map[string]string{"message": "job created"})
}

func (h *JobHandler) GetJob(c echo.Context) error {
	id := c.Param("id")
	return c.JSON(http.StatusOK, map[string]string{"id": id, "status": "pending"})
}

func (h *JobHandler) CancelJob(c echo.Context) error {
	id := c.Param("id")
	return c.JSON(http.StatusOK, map[string]string{"id": id, "status": "cancelled"})
}
