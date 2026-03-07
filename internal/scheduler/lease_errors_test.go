package scheduler

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLeaseError_Error(t *testing.T) {
	innerErr := errors.New("lease already held")
	err := &LeaseError{
		Err:  innerErr,
		Code: "LEASE_ACQUIRE_CONFLICT",
	}

	result := err.Error()
	assert.Contains(t, result, "LEASE_ACQUIRE_CONFLICT")
	assert.Contains(t, result, "lease already held")
}

func TestLeaseError_Unwrap(t *testing.T) {
	innerErr := errors.New("inner error")
	err := &LeaseError{
		Err:  innerErr,
		Code: "TEST_CODE",
	}

	unwrapped := err.Unwrap()
	assert.Equal(t, innerErr, unwrapped)
}

func TestLeaseError_WithDetail(t *testing.T) {
	innerErr := errors.New("lease lost")
	err := &LeaseError{
		Err:  innerErr,
		Code: "FENCED_CONFLICT",
		Detail: map[string]string{
			"job_id": "job-123",
			"run_id": "run-456",
		},
	}

	assert.Contains(t, err.Error(), "FENCED_CONFLICT")
	assert.Contains(t, err.Error(), "lease lost")
}

func TestLeaseError_WrappedError(t *testing.T) {
	wrappedErr := fmt.Errorf("wrapped: %w", errors.New("original"))
	err := &LeaseError{
		Err:  wrappedErr,
		Code: "WRAPPED_CODE",
	}

	assert.Contains(t, err.Error(), "WRAPPED_CODE")
	assert.Equal(t, wrappedErr, err.Unwrap())
}
