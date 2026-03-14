package testing

import (
	"testing"

	"github.com/gatblau/release-engine/internal/connector"
	"github.com/stretchr/testify/suite"
)

func TestMockConnectorContract(t *testing.T) {
	mock, _ := connector.NewBaseConnector(connector.ConnectorTypeOther, "mock")
	suite.Run(t, &ConnectorContractTestSuite{
		Connector: &MockConnector{
			BaseConnector: mock,
		},
		ValidOperation: "test_op",
		ValidInput:     map[string]interface{}{"key": "value"},
	})
}
