// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package observability

import (
	"errors"
	"fmt"
)

var (
	// ErrMetricsCollectorConflict is returned when a metric collector conflicts with an existing one.
	ErrMetricsCollectorConflict = errors.New("metrics collector conflict")
	// ErrMetricsEndpointUnavailable is returned when the metrics endpoint is unavailable.
	ErrMetricsEndpointUnavailable = errors.New("metrics endpoint unavailable")
	// ErrMetricsSQLWriteFailed is returned when writing to metrics SQL fails.
	ErrMetricsSQLWriteFailed = errors.New("metrics sql write failed")
	// ErrMetricsSQLQueueFull is returned when the metrics SQL queue is full.
	ErrMetricsSQLQueueFull = errors.New("metrics sql queue full")
	// ErrTracingInitFailed is returned when tracing initialisation fails.
	ErrTracingInitFailed = errors.New("tracing initialisation failed")
	// ErrTracingFlushTimeout is returned when tracing flush times out.
	ErrTracingFlushTimeout = errors.New("tracing flush timed out")
)

// ObservabilityError represents an observability-specific error.
type ObservabilityError struct {
	Err    error
	Code   string
	Detail map[string]string
}

// Error returns the error message.
func (e *ObservabilityError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Err.Error())
}

// Unwrap returns the underlying error.
func (e *ObservabilityError) Unwrap() error {
	return e.Err
}

// NewObservabilityError creates a new ObservabilityError.
func NewObservabilityError(err error, code string, detail map[string]string) *ObservabilityError {
	return &ObservabilityError{Err: err, Code: code, Detail: detail}
}
