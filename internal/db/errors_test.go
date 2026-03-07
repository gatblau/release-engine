package db

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDBError_Error(t *testing.T) {
	innerErr := errors.New("connection refused")
	err := &DBError{
		Err:  innerErr,
		Code: "DB_UNAVAILABLE",
	}

	result := err.Error()
	assert.Contains(t, result, "DB_UNAVAILABLE")
	assert.Contains(t, result, "connection refused")
}

func TestDBError_Unwrap(t *testing.T) {
	innerErr := errors.New("inner error")
	err := &DBError{
		Err:  innerErr,
		Code: "TEST_CODE",
	}

	unwrapped := err.Unwrap()
	assert.Equal(t, innerErr, unwrapped)
}

func TestDBError_WithDetail(t *testing.T) {
	innerErr := errors.New("query failed")
	err := &DBError{
		Err:  innerErr,
		Code: "DB_ERROR",
		Detail: map[string]string{
			"query": "SELECT * FROM jobs",
			"db":    "postgres",
		},
	}

	assert.Contains(t, err.Error(), "DB_ERROR")
	assert.Contains(t, err.Error(), "query failed")
}

func TestDBError_WrappedError(t *testing.T) {
	wrappedErr := fmt.Errorf("wrapped: %w", errors.New("original"))
	err := &DBError{
		Err:  wrappedErr,
		Code: "WRAPPED_CODE",
	}

	assert.Contains(t, err.Error(), "WRAPPED_CODE")
	assert.Equal(t, wrappedErr, err.Unwrap())
}
