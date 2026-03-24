package crossplane

import (
	"context"
	"testing"

	"github.com/gatblau/release-engine/internal/connector"
	conntesting "github.com/gatblau/release-engine/internal/connector/testing"
	"github.com/stretchr/testify/suite"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"
)

type CrossplaneContractTestSuite struct {
	conntesting.ConnectorContractTestSuite
}

func (suite *CrossplaneContractTestSuite) SetupTest() {
	scheme := runtime.NewScheme()
	fakeClient := fake.NewSimpleDynamicClient(scheme)
	cfg := connector.DefaultConnectorConfig()
	conn, err := NewCrossplaneConnector(cfg, fakeClient)
	suite.Require().NoError(err)

	suite.Connector = conn
	suite.ValidOperation = "create_composite_resource"
	suite.ValidInput = map[string]interface{}{
		"kind":       "Database",
		"name":       "my-db",
		"manifest":   map[string]interface{}{"spec": map[string]interface{}{"size": "large"}},
		"apiVersion": "test.crossplane.io/v1alpha1",
	}
	suite.ConnectorContractTestSuite.SetupTest()
}

func TestCrossplaneConnectorContract(t *testing.T) {
	suite.Run(t, new(CrossplaneContractTestSuite))
}

func (suite *CrossplaneContractTestSuite) TestErrorMapping() {
	_, err := suite.Connector.Execute(context.Background(), "get_resource_status", map[string]interface{}{
		"kind":       "UnknownResource",
		"name":       "not-found",
		"apiVersion": "test.crossplane.io/v1alpha1",
	}, nil)
	suite.Error(err) // Should probably return a terminal error for not found instead of go error
}
