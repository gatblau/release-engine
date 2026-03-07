package db

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMigrationError_Error(t *testing.T) {
	innerErr := errors.New("schema out of date")
	err := &MigrationError{
		Err:  innerErr,
		Code: "MIGRATION_OUTDATED",
	}

	result := err.Error()
	assert.Contains(t, result, "MIGRATION_OUTDATED")
	assert.Contains(t, result, "schema out of date")
}

func TestMigrationError_Unwrap(t *testing.T) {
	innerErr := errors.New("inner error")
	err := &MigrationError{
		Err:  innerErr,
		Code: "TEST_CODE",
	}

	unwrapped := err.Unwrap()
	assert.Equal(t, innerErr, unwrapped)
}

func TestMigrationError_WithDetail(t *testing.T) {
	innerErr := errors.New("metadata missing")
	err := &MigrationError{
		Err:  innerErr,
		Code: "MIGRATION_METADATA",
		Detail: map[string]string{
			"table":  "schema_migrations",
			"column": "version",
		},
	}

	assert.Contains(t, err.Error(), "MIGRATION_METADATA")
	assert.Contains(t, err.Error(), "metadata missing")
}

func TestMigrationError_WrappedError(t *testing.T) {
	wrappedErr := fmt.Errorf("wrapped: %w", errors.New("original"))
	err := &MigrationError{
		Err:  wrappedErr,
		Code: "WRAPPED_CODE",
	}

	assert.Contains(t, err.Error(), "WRAPPED_CODE")
	assert.Equal(t, wrappedErr, err.Unwrap())
}
