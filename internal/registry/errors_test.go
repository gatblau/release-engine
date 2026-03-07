package registry

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRegistryError_Error(t *testing.T) {
	innerErr := errors.New("module not found")
	err := &RegistryError{
		Err:  innerErr,
		Code: "MODULE_NOT_FOUND",
	}

	result := err.Error()
	assert.Contains(t, result, "MODULE_NOT_FOUND")
	assert.Contains(t, result, "module not found")
}

func TestRegistryError_Unwrap(t *testing.T) {
	innerErr := errors.New("inner error")
	err := &RegistryError{
		Err:  innerErr,
		Code: "TEST_CODE",
	}

	unwrapped := err.Unwrap()
	assert.Equal(t, innerErr, unwrapped)
}

func TestRegistryError_WithDetail(t *testing.T) {
	innerErr := errors.New("duplicate registration")
	err := &RegistryError{
		Err:  innerErr,
		Code: "DUPLICATE_MODULE",
		Detail: map[string]string{
			"module":  "my-module",
			"version": "1.0.0",
		},
	}

	assert.Contains(t, err.Error(), "DUPLICATE_MODULE")
	assert.Contains(t, err.Error(), "duplicate registration")
}

func TestRegistryError_WrappedError(t *testing.T) {
	wrappedErr := fmt.Errorf("wrapped: %w", errors.New("original"))
	err := &RegistryError{
		Err:  wrappedErr,
		Code: "WRAPPED_CODE",
	}

	assert.Contains(t, err.Error(), "WRAPPED_CODE")
	assert.Equal(t, wrappedErr, err.Unwrap())
}
