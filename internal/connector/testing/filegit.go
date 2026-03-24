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
	base, err := connector.NewBaseConnector(connector.ConnectorTypeGit, "file")
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
		"commit_files":          {"repo", "branch", "path_prefix", "files", "message", "idempotency_key"},
		"read_file":             {"repo", "branch", "path"},
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
			switch operation {
			case "commit_files":
				if _, ok := val.(map[string]interface{}); !ok {
					return fmt.Errorf("field files must be a map for commit_files operation")
				}
			case "push_files":
				if _, ok := val.([]interface{}); !ok {
					return fmt.Errorf("field files must be an array for push_files operation")
				}
			default:
				if _, ok := val.([]interface{}); !ok {
					return fmt.Errorf("field files must be an array")
				}
			}
		}
	}
	return nil
}

// Execute handles the declared operations.
func (c *FileGitConnector) Execute(ctx context.Context, operation string, input map[string]interface{}, secrets map[string][]byte) (*connector.ConnectorResult, error) {
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
	case "commit_files":
		return c.commitFiles(input)
	case "read_file":
		return c.readFile(input)
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
		{Name: "commit_files", IsAsync: false},
		{Name: "read_file", IsAsync: false},
	}
}

func (c *FileGitConnector) createRepository(input map[string]interface{}) (*connector.ConnectorResult, error) {
	repoDir, err := c.repoDir(input)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(repoDir, 0o750); err != nil {
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
	// Open a scoped root to prevent any path from escaping repoDir.
	root, err := os.OpenRoot(repoDir)
	if err != nil {
		return nil, fmt.Errorf("failed to open scoped root %s: %w", repoDir, err)
	}
	defer func(root *os.Root) {
		if err = root.Close(); err != nil {
			fmt.Printf("failed to close root %s: %v\n", repoDir, err)
		}
	}(root)

	for _, entry := range entries {
		if err = writeFileSafe(root, repoDir, entry.path, entry.content); err != nil {
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
	root, err := os.OpenRoot(repoDir)
	if err != nil {
		return nil, fmt.Errorf("failed to open scoped root %s: %w", repoDir, err)
	}
	defer func(root *os.Root) {
		if err = root.Close(); err != nil {
			fmt.Printf("failed to close root %s: %v\n", repoDir, err)
		}
	}(root)

	if err := writeFileSafe(root, repoDir, pathValue, content); err != nil {
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

	// os.OpenRoot scopes all operations to repoDir; symlinks that escape are rejected.
	root, err := os.OpenRoot(repoDir)
	if err != nil {
		return nil, fmt.Errorf("failed to open scoped root %s: %w", repoDir, err)
	}
	defer func(root *os.Root) {
		if err = root.Close(); err != nil {
			fmt.Printf("failed to close root %s: %v\n", repoDir, err)
		}
	}(root)

	content, err := readFileSafe(root, pathValue)
	if err != nil {
		return nil, err
	}
	return &connector.ConnectorResult{
		Status: connector.StatusSuccess,
		Output: map[string]interface{}{"content": string(content)},
	}, nil
}

func (c *FileGitConnector) commitFiles(input map[string]interface{}) (*connector.ConnectorResult, error) {
	repo := input["repo"].(string)
	branch := input["branch"].(string)
	pathPrefix := input["path_prefix"].(string)
	filesMap := input["files"].(map[string]interface{})
	message := input["message"].(string)
	idempotencyKey := input["idempotency_key"].(string)

	inputWithRepoPath := map[string]interface{}{
		"repo_path": repo,
	}
	repoDir, err := c.repoDir(inputWithRepoPath)
	if err != nil {
		return nil, err
	}

	// Open a single scoped root for the entire commit operation.
	root, err := os.OpenRoot(repoDir)
	if err != nil {
		return nil, fmt.Errorf("failed to open scoped root %s: %w", repoDir, err)
	}
	defer func(root *os.Root) {
		if err = root.Close(); err != nil {
			fmt.Printf("failed to close root %s: %v\n", repoDir, err)
		}
	}(root)

	changed := false
	filesWritten := 0

	for path, contentInterface := range filesMap {
		content, ok := contentInterface.(string)
		if !ok {
			return &connector.ConnectorResult{
				Status: connector.StatusTerminalError,
				Error: &connector.ConnectorError{
					Code:    "INVALID_CONTENT",
					Message: fmt.Sprintf("content for path %s must be a string", path),
				},
			}, nil
		}

		fullPath := pathPrefix + path

		// Use root.Stat so the check is scoped — no escape via symlinks or traversal.
		cleanRel := filepath.Clean(fullPath)
		if _, statErr := root.Stat(cleanRel); statErr == nil {
			// File exists inside the root; read and compare safely.
			existing, readErr := readFileSafe(root, cleanRel)
			if readErr == nil && string(existing) == content {
				continue // identical content, skip
			}
		}

		changed = true
		filesWritten++

		if err := writeFileSafe(root, repoDir, fullPath, content); err != nil {
			return &connector.ConnectorResult{
				Status: connector.StatusTerminalError,
				Error: &connector.ConnectorError{
					Code:    "FILE_WRITE_FAILED",
					Message: fmt.Sprintf("failed to write file %s: %v", fullPath, err),
				},
			}, nil
		}
	}

	mockCommitSHA := fmt.Sprintf("%s-%s-%d", repo, branch, filesWritten)
	_ = message
	_ = idempotencyKey

	return &connector.ConnectorResult{
		Status: connector.StatusSuccess,
		Output: map[string]interface{}{
			"commit_sha":    mockCommitSHA,
			"changed":       changed,
			"files_written": filesWritten,
		},
	}, nil
}

func (c *FileGitConnector) readFile(input map[string]interface{}) (*connector.ConnectorResult, error) {
	repo := input["repo"].(string)
	branch := input["branch"].(string)
	path := input["path"].(string)

	inputWithRepoPath := map[string]interface{}{
		"repo_path": repo,
		"path":      path,
	}
	_ = branch
	return c.getFile(inputWithRepoPath)
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
	// After Clean, any traversal attempt surfaces as a leading "..".
	if repoPath == ".." || strings.HasPrefix(repoPath, ".."+string(os.PathSeparator)) {
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

// readFileSafe reads a file through an os.Root, ensuring the path cannot
// escape the root directory via traversal sequences or symlinks.
func readFileSafe(root *os.Root, relPath string) ([]byte, error) {
	cleanRel := filepath.Clean(relPath)
	f, err := root.Open(cleanRel)
	if err != nil {
		return nil, fmt.Errorf("readFileSafe: %w", err)
	}
	defer func(root *os.Root) {
		if err = root.Close(); err != nil {
			fmt.Printf("failed to close file %s: %v\n", cleanRel, err)
		}
	}(root)

	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("readFileSafe stat: %w", err)
	}
	buf := make([]byte, info.Size())
	if _, err := f.Read(buf); err != nil {
		return nil, fmt.Errorf("readFileSafe read: %w", err)
	}
	return buf, nil
}

// writeFileSafe writes content to a file through an os.Root, ensuring the
// path cannot escape the root directory. Parent directories are created via
// the normal os package using a path already validated to be inside repoDir.
func writeFileSafe(root *os.Root, repoDir, relPath, content string) error {
	cleanRel := filepath.Clean(relPath)

	// Ensure the cleaned relative path does not escape root before we create
	// parent directories with the un-scoped os.MkdirAll (needed because
	// os.Root does not expose MkdirAll yet).
	absPath := filepath.Join(repoDir, cleanRel)
	cleanRoot := filepath.Clean(repoDir)
	if !strings.HasPrefix(absPath, cleanRoot+string(os.PathSeparator)) {
		return fmt.Errorf("writeFileSafe: path %q escapes root %q", relPath, repoDir)
	}

	if err := os.MkdirAll(filepath.Dir(absPath), 0o750); err != nil {
		return fmt.Errorf("writeFileSafe mkdir: %w", err)
	}

	// The actual write goes through the scoped root.
	f, err := root.OpenFile(cleanRel, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("writeFileSafe open: %w", err)
	}
	defer func(f *os.File) {
		if err = f.Close(); err != nil {
			fmt.Printf("WARNING: failed to close file %s: %v", cleanRel, err)
		}
	}(f)

	if _, err = f.Write([]byte(content)); err != nil {
		return fmt.Errorf("writeFileSafe write: %w", err)
	}
	return nil
}
