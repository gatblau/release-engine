package testing

import (
	"context"
	"testing"
	"time"

	"github.com/gatblau/release-engine/internal/connector"
	"github.com/gatblau/release-engine/internal/observability"
	"github.com/gatblau/release-engine/internal/runner"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// E2EComponentHarness tests the full StepExecutor pipeline including observability.
func TestE2EComponentHarness(t *testing.T) {
	// 1. Arrange
	registry := connector.NewConnectorRegistry()
	mock, _ := connector.NewBaseConnector(connector.ConnectorTypeOther, "mock")
	m := &MockConnector{
		BaseConnector: mock,
		ExecuteFunc: func(ctx context.Context, operation string, input map[string]interface{}, secrets map[string][]byte) (*connector.ConnectorResult, error) {
			return &connector.ConnectorResult{Status: connector.StatusSuccess, Output: map[string]interface{}{"result": "done"}}, nil
		},
	}
	_ = registry.Register(m)

	logger := zap.NewNop()
	// Create minimal metrics exporter for test
	metrics := observability.NewMetricsExporter(logger, 9091)

	executor := runner.NewStepExecutor(registry, logger, metrics, nil)

	// 2. Act
	ctx := connector.WithCallID(context.Background(), "e2e-test-123")
	result, err := executor.Execute(ctx, "other-mock", "test_op", map[string]interface{}{"foo": "bar"}, 5*time.Second)

	// 3. Assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
	if result != nil {
		assert.Equal(t, connector.StatusSuccess, result.Status)
		assert.Equal(t, "done", result.Output["result"])
	}

	// 4. Teardown
	_ = m.Close()
}
