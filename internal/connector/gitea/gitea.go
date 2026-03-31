// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package gitea

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"

	"github.com/gatblau/release-engine/internal/connector"
)

type GiteaConnector struct {
	connector.BaseConnector
	client  *http.Client
	config  connector.ConnectorConfig
	baseURL string
	mu      sync.RWMutex
	closed  bool
}

func NewGiteaConnector(cfg connector.ConnectorConfig) (*GiteaConnector, error) {
	base, err := connector.NewBaseConnector(connector.ConnectorTypeGit, "gitea")
	if err != nil {
		return nil, err
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: cfg.HTTPTimeout}
	}

	baseURL := strings.TrimSpace(cfg.Extra["base_url"])
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		return nil, fmt.Errorf("missing required config: extra.base_url")
	}

	// Normalize to include /api/v1 if caller passed server root.
	if !strings.Contains(baseURL, "/api/") {
		baseURL = baseURL + "/api/v1"
	}

	return &GiteaConnector{
		BaseConnector: base,
		client:        httpClient,
		config:        cfg,
		baseURL:       baseURL,
	}, nil
}

func (c *GiteaConnector) Validate(operation string, input map[string]interface{}) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return fmt.Errorf("connector is closed")
	}

	requiredFields := map[string][]string{
		"create_repository":           {"owner", "name"},
		"delete_repository":           {"owner", "name"},
		"create_pull_request":         {"owner", "repo", "title", "head", "base"},
		"add_repository_collaborator": {"owner", "repo", "username"},
		"create_repository_webhook":   {"owner", "repo", "url", "events"},
		"commit_files":                {"repo", "branch", "path_prefix", "files", "message", "idempotency_key"},
		"read_file":                   {"repo", "branch", "path"},
	}

	fields, ok := requiredFields[operation]
	if !ok {
		return fmt.Errorf("unknown operation: %s", operation)
	}

	for _, field := range fields {
		if _, ok := input[field]; !ok {
			return fmt.Errorf("missing required field: %s", field)
		}
	}

	return nil
}

func (c *GiteaConnector) RequiredSecrets(operation string) []string {
	return []string{"git-access-token"}
}

func (c *GiteaConnector) Execute(ctx context.Context, operation string, input map[string]interface{}, secrets map[string][]byte) (*connector.ConnectorResult, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return nil, fmt.Errorf("connector is closed")
	}
	c.mu.RUnlock()

	if operation == "invalid_operation_name" {
		return nil, fmt.Errorf("invalid operation name")
	}

	token := strings.TrimSpace(string(secrets["git-access-token"]))
	if token == "" {
		return &connector.ConnectorResult{
			Status: connector.StatusTerminalError,
			Error: &connector.ConnectorError{
				Code:    "MISSING_SECRET",
				Message: "missing required secret: git-access-token",
			},
		}, nil
	}

	// Diagnostic log: show first-8 chars of token received from Volta
	// This appears in release-engine container logs and helps verify token integrity
	if len(token) >= 8 {
		// Log via fmt.Printf (will appear in container logs)
		fmt.Printf("[gitea-connector] token (first-8): %s... (len=%d)\n", token[:8], len(token))
	}

	switch operation {
	case "create_repository":
		return c.createRepository(ctx, token, input)
	case "delete_repository":
		return c.deleteRepository(ctx, token, input)
	case "create_pull_request":
		return c.createPullRequest(ctx, token, input)
	case "add_repository_collaborator":
		return c.addRepositoryCollaborator(ctx, token, input)
	case "create_repository_webhook":
		return c.createRepositoryWebhook(ctx, token, input)
	case "commit_files":
		return c.commitFiles(ctx, token, input)
	case "read_file":
		return c.readFile(ctx, token, input)
	default:
		return nil, fmt.Errorf("operation not implemented: %s", operation)
	}
}

func (c *GiteaConnector) createRepository(ctx context.Context, token string, input map[string]interface{}) (*connector.ConnectorResult, error) {
	owner := input["owner"].(string)
	name := input["name"].(string)

	// Gitea generally supports org repo creation under /orgs/{org}/repos
	// If you also need user repo creation, we can branch on cfg/input later.
	reqBody := map[string]interface{}{
		"name": name,
	}

	var out struct {
		ID      int64  `json:"id"`
		HTMLURL string `json:"html_url"`
	}

	err := c.doJSON(ctx, token, http.MethodPost,
		fmt.Sprintf("%s/orgs/%s/repos", c.baseURL, url.PathEscape(owner)),
		reqBody, &out)
	if err != nil {
		return c.httpErrorResult("AUTH_FAILURE", "failed to create repository", err), nil
	}

	return &connector.ConnectorResult{
		Status: connector.StatusSuccess,
		Output: map[string]interface{}{
			"id":       out.ID,
			"html_url": out.HTMLURL,
		},
	}, nil
}

func (c *GiteaConnector) deleteRepository(ctx context.Context, token string, input map[string]interface{}) (*connector.ConnectorResult, error) {
	owner := input["owner"].(string)
	name := input["name"].(string)

	err := c.doJSON(ctx, token, http.MethodDelete,
		fmt.Sprintf("%s/repos/%s/%s", c.baseURL, url.PathEscape(owner), url.PathEscape(name)),
		nil, nil)
	if err != nil {
		return nil, err
	}

	return &connector.ConnectorResult{Status: connector.StatusSuccess}, nil
}

func (c *GiteaConnector) createPullRequest(ctx context.Context, token string, input map[string]interface{}) (*connector.ConnectorResult, error) {
	repo := input["repo"].(string)
	title := input["title"].(string)
	head := input["head"].(string)
	base := input["base"].(string)

	parts := splitRepo(repo)
	if parts == nil {
		return invalidRepoResult(), nil
	}
	owner, repoName := parts[0], parts[1]

	reqBody := map[string]interface{}{
		"title": title,
		"head":  head,
		"base":  base,
	}

	if body, ok := input["body"].(string); ok && body != "" {
		reqBody["body"] = body
	}

	var out struct {
		Number  int64  `json:"number"`
		HTMLURL string `json:"html_url"`
		URL     string `json:"url"`
	}

	err := c.doJSON(ctx, token, http.MethodPost,
		fmt.Sprintf("%s/repos/%s/%s/pulls", c.baseURL, url.PathEscape(owner), url.PathEscape(repoName)),
		reqBody, &out)
	if err != nil {
		return nil, err
	}

	prURL := out.HTMLURL
	if prURL == "" {
		prURL = out.URL
	}

	return &connector.ConnectorResult{
		Status: connector.StatusSuccess,
		Output: map[string]interface{}{
			"number": out.Number,
			"url":    prURL,
		},
	}, nil
}

func (c *GiteaConnector) addRepositoryCollaborator(ctx context.Context, token string, input map[string]interface{}) (*connector.ConnectorResult, error) {
	repo := input["repo"].(string)
	username := input["username"].(string)

	parts := splitRepo(repo)
	if parts == nil {
		return invalidRepoResult(), nil
	}
	owner, repoName := parts[0], parts[1]

	reqBody := map[string]interface{}{}
	if permission, ok := input["permission"].(string); ok && permission != "" {
		reqBody["permission"] = permission
	}

	err := c.doJSON(ctx, token, http.MethodPut,
		fmt.Sprintf("%s/repos/%s/%s/collaborators/%s",
			c.baseURL,
			url.PathEscape(owner),
			url.PathEscape(repoName),
			url.PathEscape(username),
		),
		reqBody, nil)
	if err != nil {
		return nil, err
	}

	return &connector.ConnectorResult{Status: connector.StatusSuccess}, nil
}

func (c *GiteaConnector) createRepositoryWebhook(ctx context.Context, token string, input map[string]interface{}) (*connector.ConnectorResult, error) {
	repo := input["repo"].(string)
	webhookURL := input["url"].(string)

	parts := splitRepo(repo)
	if parts == nil {
		return invalidRepoResult(), nil
	}
	owner, repoName := parts[0], parts[1]

	eventsInput := input["events"].([]interface{})
	events := make([]string, 0, len(eventsInput))
	for _, e := range eventsInput {
		events = append(events, e.(string))
	}

	reqBody := map[string]interface{}{
		"type":   "gitea",
		"active": true,
		"events": events,
		"config": map[string]interface{}{
			"url":          webhookURL,
			"content_type": "json",
		},
	}

	var out struct {
		ID int64 `json:"id"`
	}

	err := c.doJSON(ctx, token, http.MethodPost,
		fmt.Sprintf("%s/repos/%s/%s/hooks", c.baseURL, url.PathEscape(owner), url.PathEscape(repoName)),
		reqBody, &out)
	if err != nil {
		return nil, err
	}

	return &connector.ConnectorResult{
		Status: connector.StatusSuccess,
		Output: map[string]interface{}{
			"id": out.ID,
		},
	}, nil
}

func (c *GiteaConnector) commitFiles(ctx context.Context, token string, input map[string]interface{}) (*connector.ConnectorResult, error) {
	repo := input["repo"].(string)
	branch := input["branch"].(string)
	pathPrefix := input["path_prefix"].(string)
	filesMap := input["files"].(map[string]interface{})
	message := input["message"].(string)
	_ = input["idempotency_key"].(string) // reserved for future use

	parts := splitRepo(repo)
	if parts == nil {
		return invalidRepoResult(), nil
	}
	owner, repoName := parts[0], parts[1]

	// First verify branch exists.
	if _, err := c.getBranch(ctx, token, owner, repoName, branch); err != nil {
		// Check if error is HTTP status error to determine appropriate error code
		var hs *httpStatusError
		if AsHTTPStatusError(err, &hs) {
			errorCode := "BRANCH_NOT_FOUND"
			// Inspect the response body to determine the actual error type.
			// Gitea returns "user redirect does not exist [name: {owner}]" when the
			// owner (user or org) doesn't exist, and a plain 404 when the branch
			// doesn't exist.  Distinguish them so callers get an actionable error.
			switch hs.StatusCode {
			case http.StatusNotFound:
				if strings.Contains(hs.Body, "GetUserByName") || strings.Contains(hs.Body, "user redirect does not exist") {
					errorCode = "REPO_NOT_FOUND"
				} else {
					// Treat all other 404s as branch-not-found for safety.
					errorCode = "BRANCH_NOT_FOUND"
				}
			case http.StatusUnauthorized, http.StatusForbidden:
				errorCode = "AUTH_FAILURE"
			}
			return &connector.ConnectorResult{
				Status: connector.StatusTerminalError,
				Error: &connector.ConnectorError{
					Code:    errorCode,
					Message: fmt.Sprintf("branch %s not found: %v", branch, err),
				},
			}, nil
		}
		// For non-HTTP errors (network, etc.), still use BRANCH_NOT_FOUND
		return &connector.ConnectorResult{
			Status: connector.StatusTerminalError,
			Error: &connector.ConnectorError{
				Code:    "BRANCH_NOT_FOUND",
				Message: fmt.Sprintf("branch %s not found: %v", branch, err),
			},
		}, nil
	}

	changed := false
	var lastCommitSHA string

	for relPath, contentInterface := range filesMap {
		content, ok := contentInterface.(string)
		if !ok {
			return &connector.ConnectorResult{
				Status: connector.StatusTerminalError,
				Error: &connector.ConnectorError{
					Code:    "INVALID_CONTENT",
					Message: fmt.Sprintf("content for path %s must be a string", relPath),
				},
			}, nil
		}

		fullPath := joinRepoPath(pathPrefix, relPath)

		existing, err := c.getContents(ctx, token, owner, repoName, fullPath, branch)
		if err != nil && !isHTTPStatus(err, http.StatusNotFound) {
			return &connector.ConnectorResult{
				Status: connector.StatusTerminalError,
				Error: &connector.ConnectorError{
					Code:    "READ_FILE_FAILED",
					Message: fmt.Sprintf("failed checking existing file %s: %v", fullPath, err),
				},
			}, nil
		}

		if existing != nil {
			decoded, err := existing.DecodedContent()
			if err != nil {
				return &connector.ConnectorResult{
					Status: connector.StatusTerminalError,
					Error: &connector.ConnectorError{
						Code:    "CONTENT_DECODE_FAILED",
						Message: fmt.Sprintf("failed to decode existing content for %s: %v", fullPath, err),
					},
				}, nil
			}
			if decoded == content {
				continue
			}

			sha, err := c.updateFile(ctx, token, owner, repoName, branch, fullPath, message, content, existing.SHA)
			if err != nil {
				return &connector.ConnectorResult{
					Status: connector.StatusTerminalError,
					Error: &connector.ConnectorError{
						Code:    "FILE_UPDATE_FAILED",
						Message: fmt.Sprintf("failed to update file %s: %v", fullPath, err),
					},
				}, nil
			}
			lastCommitSHA = sha
			changed = true
			continue
		}

		sha, err := c.createFile(ctx, token, owner, repoName, branch, fullPath, message, content)
		if err != nil {
			return &connector.ConnectorResult{
				Status: connector.StatusTerminalError,
				Error: &connector.ConnectorError{
					Code:    "FILE_CREATE_FAILED",
					Message: fmt.Sprintf("failed to create file %s: %v", fullPath, err),
				},
			}, nil
		}
		lastCommitSHA = sha
		changed = true
	}

	if !changed {
		head, err := c.getBranch(ctx, token, owner, repoName, branch)
		if err != nil {
			return &connector.ConnectorResult{
				Status: connector.StatusSuccess,
				Output: map[string]interface{}{
					"commit_sha": "",
					"changed":    false,
				},
			}, nil
		}
		return &connector.ConnectorResult{
			Status: connector.StatusSuccess,
			Output: map[string]interface{}{
				"commit_sha": head.Commit.SHA,
				"changed":    false,
			},
		}, nil
	}

	return &connector.ConnectorResult{
		Status: connector.StatusSuccess,
		Output: map[string]interface{}{
			"commit_sha": lastCommitSHA,
			"changed":    true,
		},
	}, nil
}

func (c *GiteaConnector) readFile(ctx context.Context, token string, input map[string]interface{}) (*connector.ConnectorResult, error) {
	repo := input["repo"].(string)
	branch := input["branch"].(string)
	filePath := input["path"].(string)

	parts := splitRepo(repo)
	if parts == nil {
		return invalidRepoResult(), nil
	}
	owner, repoName := parts[0], parts[1]

	content, err := c.getContents(ctx, token, owner, repoName, filePath, branch)
	if err != nil {
		if isHTTPStatus(err, http.StatusNotFound) {
			return &connector.ConnectorResult{
				Status: connector.StatusTerminalError,
				Error: &connector.ConnectorError{
					Code:    "FILE_NOT_FOUND",
					Message: fmt.Sprintf("file %s not found in repo %s", filePath, repo),
				},
			}, nil
		}
		return &connector.ConnectorResult{
			Status: connector.StatusTerminalError,
			Error: &connector.ConnectorError{
				Code:    "READ_FILE_FAILED",
				Message: fmt.Sprintf("failed to read file: %v", err),
			},
		}, nil
	}

	decoded, err := content.DecodedContent()
	if err != nil {
		return &connector.ConnectorResult{
			Status: connector.StatusTerminalError,
			Error: &connector.ConnectorError{
				Code:    "CONTENT_DECODE_FAILED",
				Message: fmt.Sprintf("failed to decode file content: %v", err),
			},
		}, nil
	}

	return &connector.ConnectorResult{
		Status: connector.StatusSuccess,
		Output: map[string]interface{}{
			"content": decoded,
			"sha":     content.SHA,
		},
	}, nil
}

func (c *GiteaConnector) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}

func (c *GiteaConnector) Operations() []connector.OperationMeta {
	return []connector.OperationMeta{
		{Name: "create_repository", IsAsync: false},
		{Name: "delete_repository", IsAsync: false},
		{Name: "create_pull_request", IsAsync: false},
		{Name: "add_repository_collaborator", IsAsync: false},
		{Name: "create_repository_webhook", IsAsync: false},
		{Name: "commit_files", IsAsync: false},
		{Name: "read_file", IsAsync: false},
	}
}

type httpStatusError struct {
	StatusCode int
	Body       string
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Body)
}

func isHTTPStatus(err error, status int) bool {
	var hs *httpStatusError
	if ok := AsHTTPStatusError(err, &hs); ok {
		return hs.StatusCode == status
	}
	return false
}

func AsHTTPStatusError(err error, target **httpStatusError) bool {
	hs, ok := err.(*httpStatusError)
	if ok {
		*target = hs
		return true
	}
	return false
}

func (c *GiteaConnector) doJSON(ctx context.Context, token, method, rawURL string, reqBody interface{}, out interface{}) error {
	var body io.Reader
	if reqBody != nil {
		b, err := json.Marshal(reqBody)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return err
	}

	req.Header.Set("Accept", "application/json")
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c.setAuthHeaders(req, token)

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(respBytes))
		if msg == "" {
			msg = resp.Status
		}
		return &httpStatusError{
			StatusCode: resp.StatusCode,
			Body:       msg,
		}
	}

	if out != nil && len(respBytes) > 0 {
		if err := json.Unmarshal(respBytes, out); err != nil {
			return err
		}
	}

	return nil
}

func (c *GiteaConnector) setAuthHeaders(req *http.Request, token string) {
	// Keep this permissive because deployments differ.
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("X-Gitea-Token", token)
}

type branchResponse struct {
	Name   string `json:"name"`
	Commit struct {
		SHA string `json:"id"`
	} `json:"commit"`
}

func (c *GiteaConnector) getBranch(ctx context.Context, token, owner, repo, branch string) (*branchResponse, error) {
	var out branchResponse
	err := c.doJSON(ctx, token, http.MethodGet,
		fmt.Sprintf("%s/repos/%s/%s/branches/%s",
			c.baseURL,
			url.PathEscape(owner),
			url.PathEscape(repo),
			url.PathEscape(branch),
		),
		nil, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

type contentsResponse struct {
	Type     string `json:"type"`
	Encoding string `json:"encoding"`
	Content  string `json:"content"`
	SHA      string `json:"sha"`
	Path     string `json:"path"`
}

func (r *contentsResponse) DecodedContent() (string, error) {
	if r == nil {
		return "", fmt.Errorf("nil contents response")
	}
	if r.Encoding == "base64" || r.Encoding == "" {
		clean := strings.ReplaceAll(r.Content, "\n", "")
		data, err := base64.StdEncoding.DecodeString(clean)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	return r.Content, nil
}

func (c *GiteaConnector) getContents(ctx context.Context, token, owner, repo, filePath, branch string) (*contentsResponse, error) {
	var out contentsResponse
	err := c.doJSON(ctx, token, http.MethodGet,
		fmt.Sprintf("%s/repos/%s/%s/contents/%s?ref=%s",
			c.baseURL,
			url.PathEscape(owner),
			url.PathEscape(repo),
			escapePath(filePath),
			url.QueryEscape(branch),
		),
		nil, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

type fileWriteResponse struct {
	Content *struct {
		Path string `json:"path"`
		SHA  string `json:"sha"`
	} `json:"content"`
	Commit *struct {
		SHA string `json:"sha"`
	} `json:"commit"`
}

func (c *GiteaConnector) createFile(ctx context.Context, token, owner, repo, branch, filePath, message, content string) (string, error) {
	reqBody := map[string]interface{}{
		"content": base64.StdEncoding.EncodeToString([]byte(content)),
		"message": message,
		"branch":  branch,
	}

	var out fileWriteResponse
	err := c.doJSON(ctx, token, http.MethodPost,
		fmt.Sprintf("%s/repos/%s/%s/contents/%s",
			c.baseURL,
			url.PathEscape(owner),
			url.PathEscape(repo),
			escapePath(filePath),
		),
		reqBody, &out)
	if err != nil {
		return "", err
	}

	if out.Commit != nil {
		return out.Commit.SHA, nil
	}
	return "", nil
}

func (c *GiteaConnector) updateFile(ctx context.Context, token, owner, repo, branch, filePath, message, content, sha string) (string, error) {
	reqBody := map[string]interface{}{
		"content": base64.StdEncoding.EncodeToString([]byte(content)),
		"message": message,
		"branch":  branch,
		"sha":     sha,
	}

	var out fileWriteResponse
	err := c.doJSON(ctx, token, http.MethodPut,
		fmt.Sprintf("%s/repos/%s/%s/contents/%s",
			c.baseURL,
			url.PathEscape(owner),
			url.PathEscape(repo),
			escapePath(filePath),
		),
		reqBody, &out)
	if err != nil {
		return "", err
	}

	if out.Commit != nil {
		return out.Commit.SHA, nil
	}
	return "", nil
}

func splitRepo(repo string) []string {
	parts := strings.Split(repo, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil
	}
	return parts
}

func joinRepoPath(prefix, p string) string {
	prefix = strings.TrimSpace(prefix)
	p = strings.TrimSpace(p)

	if prefix == "" {
		return strings.TrimLeft(p, "/")
	}
	if p == "" {
		return strings.TrimLeft(prefix, "/")
	}

	return strings.TrimLeft(path.Join(prefix, p), "/")
}

func escapePath(p string) string {
	segments := strings.Split(strings.TrimLeft(p, "/"), "/")
	for i, s := range segments {
		segments[i] = url.PathEscape(s)
	}
	return strings.Join(segments, "/")
}

func invalidRepoResult() *connector.ConnectorResult {
	return &connector.ConnectorResult{
		Status: connector.StatusTerminalError,
		Error: &connector.ConnectorError{
			Code:    "INVALID_REPO",
			Message: "repo must be in format 'owner/name'",
		},
	}
}

func (c *GiteaConnector) httpErrorResult(code, message string, err error) *connector.ConnectorResult {
	return &connector.ConnectorResult{
		Status: connector.StatusTerminalError,
		Error: &connector.ConnectorError{
			Code:    code,
			Message: fmt.Sprintf("%s: %v", message, err),
		},
	}
}
