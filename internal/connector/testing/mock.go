package testing

import (
	"context"
	"errors"

	"github.com/gatblau/release-engine/internal/connector"
)

// MockConnector is a flexible mock for connector testing.
type MockConnector struct {
	connector.BaseConnector
	ValidateFunc   func(operation string, input map[string]interface{}) error
	ExecuteFunc    func(ctx context.Context, operation string, input map[string]interface{}) (*connector.ConnectorResult, error)
	OperationsFunc func() []connector.OperationMeta
	CloseFunc      func() error
}

func (m *MockConnector) Validate(operation string, input map[string]interface{}) error {
	if m.ValidateFunc != nil {
		return m.ValidateFunc(operation, input)
	}
	if operation == "unknown_op" {
		return errors.New("unknown operation")
	}
	return nil
}

func (m *MockConnector) Execute(ctx context.Context, operation string, input map[string]interface{}) (*connector.ConnectorResult, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if operation == "invalid_operation_name" {
		return nil, errors.New("invalid operation name")
	}
	if m.ExecuteFunc != nil {
		return m.ExecuteFunc(ctx, operation, input)
	}
	return &connector.ConnectorResult{Status: connector.StatusSuccess}, nil
}

func (m *MockConnector) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

func (m *MockConnector) Operations() []connector.OperationMeta {
	if m.OperationsFunc != nil {
		return m.OperationsFunc()
	}
	return nil
}
