// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package testing

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gatblau/release-engine/internal/connector"
)

// FileGitConnector is a filesystem-backed stand-in for the git connector interface.
type FileGitConnector struct {
	connector.BaseConnector
	OutputDir string
	mu        sync.RWMutex
	closed    bool
}

// NewFileGitConnector creates a new FileGitConnector that uses the provided directory.
// If outputDir is empty the system temp directory is used.
func NewFileGitConnector(outputDir string) (*FileGitConnector, error) {
	base, err := connector.NewBaseConnector(connector.ConnectorTypeGit, "filegit")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(outputDir) == "" {
		outputDir = os.TempDir()
	}
	return &FileGitConnector{BaseConnector: base, OutputDir: outputDir}, nil
}

// Validate ensures the operation payload has the required fields.
func (c *FileGitConnector) Validate(operation string, input map[string]interface{}) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
		return fmt.Errorf("connector is closed")
	}
	required := map[string][]string{
		"create_repository":     {"repo_path"},
		"push_files":            {"repo_path", "files"},
		"create_or_update_file": {"repo_path", "path", "content"},
		"get_file":              {"repo_path", "path"},
	}
	fields, ok := required[operation]
	if !ok {
		return fmt.Errorf("unknown operation: %s", operation)
	}
	for _, field := range fields {
		val, ok := input[field]
		if !ok {
			return fmt.Errorf("missing required field: %s", field)
		}
		if str, ok := val.(string); ok && strings.TrimSpace(str) == "" {
			return fmt.Errorf("field %s must not be empty", field)
		}
		if field == "files" {
			if _, ok := val.([]interface{}); !ok {
				return fmt.Errorf("field files must be an array")
			}
		}
	}
	return nil
}

// Execute handles the declared operations.
func (c *FileGitConnector) Execute(ctx context.Context, operation string, input map[string]interface{}) (*connector.ConnectorResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return nil, fmt.Errorf("connector is closed")
	}
	c.mu.RUnlock()
	switch operation {
	case "create_repository":
		return c.createRepository(input)
	case "push_files":
		return c.pushFiles(input)
	case "create_or_update_file":
		return c.createOrUpdateFile(input)
	case "get_file":
		return c.getFile(input)
	default:
		return nil, fmt.Errorf("operation not implemented: %s", operation)
	}
}

// Close marks the connector as closed.
func (c *FileGitConnector) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}

// Operations returns the supported operation metadata.
func (c *FileGitConnector) Operations() []connector.OperationMeta {
	return []connector.OperationMeta{
		{Name: "create_repository", IsAsync: false},
		{Name: "push_files", IsAsync: false},
		{Name: "create_or_update_file", IsAsync: false},
		{Name: "get_file", IsAsync: false},
	}
}

func (c *FileGitConnector) createRepository(input map[string]interface{}) (*connector.ConnectorResult, error) {
	repoDir, err := c.repoDir(input)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		return nil, err
	}
	return &connector.ConnectorResult{Status: connector.StatusSuccess}, nil
}

func (c *FileGitConnector) pushFiles(input map[string]interface{}) (*connector.ConnectorResult, error) {
	repoDir, err := c.repoDir(input)
	if err != nil {
		return nil, err
	}
	entries, err := c.parseFiles(input["files"])
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if err := writeFile(repoDir, entry.path, entry.content); err != nil {
			return nil, err
		}
	}
	return &connector.ConnectorResult{
		Status: connector.StatusSuccess,
		Output: map[string]interface{}{"files_written": len(entries)},
	}, nil
}

func (c *FileGitConnector) createOrUpdateFile(input map[string]interface{}) (*connector.ConnectorResult, error) {
	repoDir, err := c.repoDir(input)
	if err != nil {
		return nil, err
	}
	content, ok := input["content"].(string)
	if !ok {
		return nil, fmt.Errorf("content must be a string")
	}
	pathValue, ok := input["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path must be a string")
	}
	if err := writeFile(repoDir, pathValue, content); err != nil {
		return nil, err
	}
	return &connector.ConnectorResult{Status: connector.StatusSuccess}, nil
}

func (c *FileGitConnector) getFile(input map[string]interface{}) (*connector.ConnectorResult, error) {
	repoDir, err := c.repoDir(input)
	if err != nil {
		return nil, err
	}
	pathValue, ok := input["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path must be a string")
	}
	content, err := os.ReadFile(resolvePath(repoDir, pathValue))
	if err != nil {
		return nil, err
	}
	return &connector.ConnectorResult{
		Status: connector.StatusSuccess,
		Output: map[string]interface{}{"content": string(content)},
	}, nil
}

func (c *FileGitConnector) repoDir(input map[string]interface{}) (string, error) {
	repoPath, ok := input["repo_path"].(string)
	if !ok || strings.TrimSpace(repoPath) == "" {
		return "", fmt.Errorf("repo_path must be a non-empty string")
	}
	repoPath = filepath.Clean(repoPath)
	if filepath.IsAbs(repoPath) {
		return "", fmt.Errorf("repo_path must be relative")
	}
	if strings.HasPrefix(repoPath, "../") || strings.HasPrefix(repoPath, "..\\") {
		return "", fmt.Errorf("repo_path must not escape the output dir")
	}
	return filepath.Join(c.OutputDir, repoPath), nil
}

type fileEntry struct {
	path    string
	content string
}

func (c *FileGitConnector) parseFiles(raw interface{}) ([]fileEntry, error) {
	list, ok := raw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("files must be an array of objects")
	}
	var entries []fileEntry
	for _, item := range list {
		m, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("each file descriptor must be a map")
		}
		pathValue, ok := m["path"].(string)
		if !ok || strings.TrimSpace(pathValue) == "" {
			return nil, fmt.Errorf("file path must be a non-empty string")
		}
		content, ok := m["content"].(string)
		if !ok {
			return nil, fmt.Errorf("file content must be a string")
		}
		entries = append(entries, fileEntry{path: pathValue, content: content})
	}
	return entries, nil
}

func writeFile(repoDir, relPath, content string) error {
	fullPath := resolvePath(repoDir, relPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(fullPath, []byte(content), 0o644)
}

func resolvePath(repoDir, relPath string) string {
	relPath = filepath.Clean(relPath)
	if filepath.IsAbs(relPath) {
		relPath = relPath[1:]
	}
	return filepath.Join(repoDir, relPath)
}
