package config

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigError_Error(t *testing.T) {
	innerErr := errors.New("missing value")
	err := &ConfigError{
		Err:  innerErr,
		Code: "MISSING_CONFIG",
	}

	result := err.Error()
	assert.Contains(t, result, "MISSING_CONFIG")
	assert.Contains(t, result, "missing value")
}

func TestConfigError_Unwrap(t *testing.T) {
	innerErr := errors.New("inner error")
	err := &ConfigError{
		Err:  innerErr,
		Code: "TEST_CODE",
	}

	unwrapped := err.Unwrap()
	assert.Equal(t, innerErr, unwrapped)
}

func TestConfigError_WithDetail(t *testing.T) {
	innerErr := errors.New("invalid value")
	err := &ConfigError{
		Err:  innerErr,
		Code: "INVALID_CONFIG",
		Detail: map[string]string{
			"field": "timeout",
			"value": "negative",
		},
	}

	assert.Contains(t, err.Error(), "INVALID_CONFIG")
	assert.Contains(t, err.Error(), "invalid value")
}

func TestConfigError_WrappedError(t *testing.T) {
	wrappedErr := fmt.Errorf("wrapped: %w", errors.New("original"))
	err := &ConfigError{
		Err:  wrappedErr,
		Code: "WRAPPED_CODE",
	}

	assert.Contains(t, err.Error(), "WRAPPED_CODE")
	assert.Equal(t, wrappedErr, err.Unwrap())
}
