// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package testing

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gatblau/release-engine/internal/connector"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type FileGitContractTestSuite struct {
	ConnectorContractTestSuite
}

func (suite *FileGitContractTestSuite) SetupTest() {
	tempDir := suite.T().TempDir()
	conn, err := NewFileGitConnector(tempDir)
	suite.Require().NoError(err)
	suite.Connector = conn

	// Create the repository first (git‑like semantics: push requires an existing repo)
	_, err = conn.Execute(context.Background(), "create_repository", map[string]interface{}{
		"repo_path": "tenant/test-app",
	}, nil)
	suite.Require().NoError(err)

	// Now push_files can be tested as a valid operation
	suite.ValidOperation = "push_files"
	suite.ValidInput = map[string]interface{}{
		"repo_path": "tenant/test-app",
		"files": []interface{}{
			map[string]interface{}{"path": "infra/main.yaml", "content": "apiVersion: v1"},
		},
	}
	suite.ConnectorContractTestSuite.SetupTest()
}

func TestFileGitContractSuite(t *testing.T) {
	suite.Run(t, new(FileGitContractTestSuite))
}

func TestFileGitConnectorOperations(t *testing.T) {
	tempDir := t.TempDir()
	conn, err := NewFileGitConnector(tempDir)
	require.NoError(t, err)
	require.NoError(t, conn.Validate("create_repository", map[string]interface{}{"repo_path": "tenant/ops"}))
	res, err := conn.Execute(context.Background(), "create_repository", map[string]interface{}{"repo_path": "tenant/ops"}, nil)
	require.NoError(t, err)
	require.Equal(t, connector.StatusSuccess, res.Status)
	info, err := os.Stat(filepath.Join(tempDir, "tenant", "ops"))
	require.NoError(t, err)
	require.True(t, info.IsDir())
	filesPayload := map[string]interface{}{
		"repo_path": "tenant/ops",
		"files": []interface{}{
			map[string]interface{}{"path": "infra/main.yaml", "content": "apiVersion: v1"},
			map[string]interface{}{"path": "infra/vars.yaml", "content": "vars:"},
		},
	}
	res, err = conn.Execute(context.Background(), "push_files", filesPayload, nil)
	require.NoError(t, err)
	require.Equal(t, connector.StatusSuccess, res.Status)
	require.Equal(t, 2, res.Output["files_written"])
	getRes, err := conn.Execute(context.Background(), "get_file", map[string]interface{}{"repo_path": "tenant/ops", "path": "infra/main.yaml"}, nil)
	require.NoError(t, err)
	require.Equal(t, connector.StatusSuccess, getRes.Status)
	require.Equal(t, "apiVersion: v1", getRes.Output["content"])
	updatePayload := map[string]interface{}{
		"repo_path": "tenant/ops",
		"path":      "infra/main.yaml",
		"content":   "apiVersion: v2",
	}
	res, err = conn.Execute(context.Background(), "create_or_update_file", updatePayload, nil)
	require.NoError(t, err)
	require.Equal(t, connector.StatusSuccess, res.Status)
	getRes, err = conn.Execute(context.Background(), "get_file", map[string]interface{}{"repo_path": "tenant/ops", "path": "infra/main.yaml"}, nil)
	require.NoError(t, err)
	require.Equal(t, "apiVersion: v2", getRes.Output["content"])
}
