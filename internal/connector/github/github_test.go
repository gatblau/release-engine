package github

import (
	"context"
	"net/http"
	"testing"

	"github.com/gatblau/release-engine/internal/connector"
	conntesting "github.com/gatblau/release-engine/internal/connector/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"gopkg.in/h2non/gock.v1"
)

type GitHubContractTestSuite struct {
	conntesting.ConnectorContractTestSuite
}

func (suite *GitHubContractTestSuite) SetupTest() {
	cfg := connector.DefaultConnectorConfig()

	httpClient := &http.Client{}
	gock.InterceptClient(httpClient)

	conn, err := NewGitHubConnectorWithClient(cfg, "", httpClient)
	suite.Require().NoError(err)

	gock.New("https://api.github.com").
		Post("/user/repos").
		Persist().
		Reply(201).
		JSON(map[string]interface{}{"id": 123, "html_url": "https://github.com/user/test"})

	suite.Connector = conn
	suite.ValidOperation = "create_repository"
	suite.ValidInput = map[string]interface{}{
		"owner": "user",
		"name":  "test",
	}
	suite.ConnectorContractTestSuite.SetupTest()
}

func (suite *GitHubContractTestSuite) TearDownTest() {
	gock.Off()
}

func TestGitHubConnectorContract(t *testing.T) {
	suite.Run(t, new(GitHubContractTestSuite))
}

func TestGitHubConnector_CreateRepository(t *testing.T) {
	defer gock.Off()

	httpClient := &http.Client{}
	gock.InterceptClient(httpClient)

	gock.New("https://api.github.com").
		Post("/user/repos").
		Reply(201).
		JSON(map[string]interface{}{"id": 123, "html_url": "https://github.com/user/test"})

	cfg := connector.DefaultConnectorConfig()
	conn, _ := NewGitHubConnectorWithClient(cfg, "fake-token", httpClient)

	input := map[string]interface{}{
		"owner": "user",
		"name":  "test",
	}

	result, err := conn.Execute(context.Background(), "create_repository", input, nil)
	assert.NoError(t, err)
	assert.Equal(t, connector.StatusSuccess, result.Status)
	assert.Equal(t, int64(123), result.Output["id"])
	assert.Equal(t, "https://github.com/user/test", result.Output["html_url"])
}
