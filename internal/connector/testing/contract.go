// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package testing

import (
	"context"
	"sync"
	"time"

	"github.com/gatblau/release-engine/internal/connector"
	"github.com/stretchr/testify/suite"
)

// ConnectorContractTestSuite defines the required tests for any Connector implementation.
type ConnectorContractTestSuite struct {
	suite.Suite
	Connector      connector.Connector
	ValidOperation string
	ValidInput     map[string]interface{}
}

func (suite *ConnectorContractTestSuite) SetupTest() {
	if suite.Connector == nil {
		suite.Fail("Connector not initialized")
	}
}

// 1. Validation Tests
func (suite *ConnectorContractTestSuite) TestValidation() {
	err := suite.Connector.Validate("unknown_op", nil)
	suite.Error(err, "should return error for unknown operation")

	err = suite.Connector.Validate(suite.ValidOperation, suite.ValidInput)
	suite.NoError(err, "should be valid for correct input")
}

// 2. Execution Contract
func (suite *ConnectorContractTestSuite) TestExecutionContract() {
	res, err := suite.Connector.Execute(context.Background(), suite.ValidOperation, suite.ValidInput)
	suite.NoError(err)
	suite.NotNil(res)
}

// 3. Context Sensitivity
func (suite *ConnectorContractTestSuite) TestContextSensitivity() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := suite.Connector.Execute(ctx, suite.ValidOperation, suite.ValidInput)
	// Connector should handle context cancellation by either returning (nil, error) or (ConnectorResult{terminal}, nil)
	// If context is cancelled, error is usually non-nil.
	suite.Error(err, "should return error when context is cancelled")
}

// 4. Call ID Propagation
func (suite *ConnectorContractTestSuite) TestCallIDPropagation() {
	callID := "test-call-id"
	ctx := connector.WithCallID(context.Background(), callID)
	suite.Equal(callID, connector.CallIDFromContext(ctx))
}

// 5. Idempotency/Close
func (suite *ConnectorContractTestSuite) TestIdempotencyAndClose() {
	err := suite.Connector.Close()
	suite.NoError(err)
	err = suite.Connector.Close() // Should be idempotent
	suite.NoError(err)
}

// 6. Error Mapping test example
func (suite *ConnectorContractTestSuite) TestErrorMapping() {
	// Let's test that executing an invalid operation cleanly returns an error
	_, err := suite.Connector.Execute(context.Background(), "invalid_operation_name", nil)
	suite.Error(err)
}

// 7. Race Condition
func (suite *ConnectorContractTestSuite) TestConcurrentExecution() {
	var wg sync.WaitGroup
	ctx := context.Background()
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = suite.Connector.Execute(ctx, suite.ValidOperation, suite.ValidInput)
		}()
	}
	wg.Wait()
}

// 8. Async Polling
func (suite *ConnectorContractTestSuite) TestAsyncPolling() {
	if describer, ok := suite.Connector.(connector.OperationDescriber); ok {
		for _, op := range describer.Operations() {
			if op.Name == suite.ValidOperation && op.IsAsync {
				start := time.Now()
				_, _ = suite.Connector.Execute(context.Background(), suite.ValidOperation, suite.ValidInput)
				duration := time.Since(start)
				suite.Less(duration, 500*time.Millisecond, "Async operation should return immediately")
			}
		}
	}
}
