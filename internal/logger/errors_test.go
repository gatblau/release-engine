package logger

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoggerError_Error(t *testing.T) {
	innerErr := errors.New("init failed")
	err := &LoggerError{
		Err:  innerErr,
		Code: "LOGGER_INIT_FAILED",
	}

	result := err.Error()
	assert.Contains(t, result, "LOGGER_INIT_FAILED")
	assert.Contains(t, result, "init failed")
}

func TestLoggerError_Unwrap(t *testing.T) {
	innerErr := errors.New("inner error")
	err := &LoggerError{
		Err:  innerErr,
		Code: "TEST_CODE",
	}

	unwrapped := err.Unwrap()
	assert.Equal(t, innerErr, unwrapped)
}

func TestLoggerError_WithDetail(t *testing.T) {
	innerErr := errors.New("invalid level")
	err := &LoggerError{
		Err:  innerErr,
		Code: "INVALID_LEVEL",
		Detail: map[string]string{
			"level": "TRACE",
			"valid": "DEBUG, INFO, WARN, ERROR",
		},
	}

	assert.Contains(t, err.Error(), "INVALID_LEVEL")
	assert.Contains(t, err.Error(), "invalid level")
}

func TestLoggerError_WrappedError(t *testing.T) {
	wrappedErr := fmt.Errorf("wrapped: %w", errors.New("original"))
	err := &LoggerError{
		Err:  wrappedErr,
		Code: "WRAPPED_CODE",
	}

	assert.Contains(t, err.Error(), "WRAPPED_CODE")
	assert.Equal(t, wrappedErr, err.Unwrap())
}
