// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package connector

import (
	"context"
	"fmt"
	"time"
)

// --- Connector Types ---

// ConnectorType represents the category of connector.
type ConnectorType string

const (
	ConnectorTypeGit    ConnectorType = "git"
	ConnectorTypeCloud  ConnectorType = "cloud"
	ConnectorTypeCD     ConnectorType = "cd"
	ConnectorTypeInfra  ConnectorType = "infra"
	ConnectorTypeDevOps ConnectorType = "devops"
	ConnectorTypeOther  ConnectorType = "other"
)

// ValidConnectorTypes is the set of known types for registration validation.
var ValidConnectorTypes = map[ConnectorType]bool{
	ConnectorTypeGit:    true,
	ConnectorTypeCloud:  true,
	ConnectorTypeCD:     true,
	ConnectorTypeInfra:  true,
	ConnectorTypeDevOps: true,
	ConnectorTypeOther:  true,
}

// --- Result Types ---

const (
	StatusSuccess        = "success"
	StatusRetryableError = "retryable_error"
	StatusTerminalError  = "terminal_error"
)

// ConnectorResult is the standard return type for all connector operations.
type ConnectorResult struct {
	Status string
	Output map[string]interface{}
	Error  *ConnectorError
}

// ConnectorError provides structured error information from a connector call.
type ConnectorError struct {
	Code    string
	Message string
	Details map[string]interface{}
}

func (e *ConnectorError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// --- Configuration ---

// ConnectorConfig holds configuration common to all connectors.
type ConnectorConfig struct {
	HTTPTimeout      time.Duration
	TransportRetries int
	Extra            map[string]string
}

// DefaultConnectorConfig returns a ConnectorConfig with sensible defaults.
func DefaultConnectorConfig() ConnectorConfig {
	return ConnectorConfig{
		HTTPTimeout:      30 * time.Second,
		TransportRetries: 3,
		Extra:            make(map[string]string),
	}
}

// --- Context Helpers ---

type contextKey string

const callIDKey contextKey = "call_id"

// WithCallID returns a new context with the call ID set.
func WithCallID(ctx context.Context, callID string) context.Context {
	return context.WithValue(ctx, callIDKey, callID)
}

// CallIDFromContext extracts the call ID from the context.
func CallIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(callIDKey).(string); ok {
		return v
	}
	return ""
}

// --- Connector Interface ---

// Connector is the core interface that all connector implementations must satisfy.
type Connector interface {
	Key() string
	Validate(operation string, input map[string]interface{}) error
	Execute(ctx context.Context, operation string, input map[string]interface{}) (*ConnectorResult, error)
	Close() error
}

// OperationDescriber is an optional interface that connectors may implement
// to provide introspection into supported operations.
type OperationDescriber interface {
	Operations() []OperationMeta
}

// OperationMeta describes a single operation supported by a connector.
type OperationMeta struct {
	Name           string
	Description    string
	RequiredFields []string
	OptionalFields []string
	IsAsync        bool
}

// --- Registry Interface ---

// ConnectorRegistry manages connector registration and lookup.
type ConnectorRegistry interface {
	Register(connector Connector) error
	Replace(connector Connector) error
	Lookup(key string) (Connector, bool)
	ListByType(connectorType ConnectorType) []Connector
	Close() error
}
