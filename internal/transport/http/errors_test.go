package http

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHTTPError_Error(t *testing.T) {
	innerErr := errors.New("bind failed")
	err := &HTTPError{
		Err:  innerErr,
		Code: "HTTP_BIND_FAILED",
	}

	result := err.Error()
	assert.Contains(t, result, "HTTP_BIND_FAILED")
	assert.Contains(t, result, "bind failed")
}

func TestHTTPError_Unwrap(t *testing.T) {
	innerErr := errors.New("inner error")
	err := &HTTPError{
		Err:  innerErr,
		Code: "TEST_CODE",
	}

	unwrapped := err.Unwrap()
	assert.Equal(t, innerErr, unwrapped)
}

func TestHTTPError_WithDetail(t *testing.T) {
	innerErr := errors.New("timeout")
	err := &HTTPError{
		Err:  innerErr,
		Code: "SHUTDOWN_TIMEOUT",
		Detail: map[string]string{
			"timeout": "30s",
			"port":    "8080",
		},
	}

	assert.Contains(t, err.Error(), "SHUTDOWN_TIMEOUT")
	assert.Contains(t, err.Error(), "timeout")
}

func TestHTTPError_WrappedError(t *testing.T) {
	wrappedErr := fmt.Errorf("wrapped: %w", errors.New("original"))
	err := &HTTPError{
		Err:  wrappedErr,
		Code: "WRAPPED_CODE",
	}

	assert.Contains(t, err.Error(), "WRAPPED_CODE")
	assert.Equal(t, wrappedErr, err.Unwrap())
}

func TestErrorResponse_JSON(t *testing.T) {
	resp := ErrorResponse{
		Error:   "not found",
		Code:    "NOT_FOUND",
		Details: map[string]string{"id": "123"},
	}

	assert.Equal(t, "not found", resp.Error)
	assert.Equal(t, "NOT_FOUND", resp.Code)
	assert.NotNil(t, resp.Details)
}

func TestErrorResponse_NoDetails(t *testing.T) {
	resp := ErrorResponse{
		Error: "internal error",
		Code:  "INTERNAL_ERROR",
	}

	assert.Equal(t, "internal error", resp.Error)
	assert.Equal(t, "INTERNAL_ERROR", resp.Code)
	assert.Nil(t, resp.Details)
}
