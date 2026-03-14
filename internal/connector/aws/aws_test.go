package aws

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/gatblau/release-engine/internal/connector"
	conntesting "github.com/gatblau/release-engine/internal/connector/testing"
	"github.com/stretchr/testify/suite"
)

type mockHTTPClient struct{}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader([]byte(`<CreateBucketResponse xmlns="http://s3.amazonaws.com/doc/2006-03-01/"></CreateBucketResponse>`))),
		Header:     make(http.Header),
	}, nil
}

type AWSContractTestSuite struct {
	conntesting.ConnectorContractTestSuite
}

func (suite *AWSContractTestSuite) SetupTest() {
	cfg := connector.DefaultConnectorConfig()
	awsCfg := aws.Config{
		Region: "us-east-1",
		Credentials: aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
			return aws.Credentials{AccessKeyID: "mock", SecretAccessKey: "mock"}, nil
		}),
		HTTPClient: &mockHTTPClient{},
	}
	conn, err := NewAWSConnector(cfg, awsCfg)
	suite.Require().NoError(err)

	suite.Connector = conn
	suite.ValidOperation = "create_s3_bucket"
	suite.ValidInput = map[string]interface{}{
		"name": "test-bucket",
	}
	suite.ConnectorContractTestSuite.SetupTest()
}

func TestAWSConnectorContract(t *testing.T) {
	suite.Run(t, new(AWSContractTestSuite))
}
