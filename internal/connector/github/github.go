// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package github

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/gatblau/release-engine/internal/connector"
	"github.com/google/go-github/v60/github"
)

type GitHubConnector struct {
	connector.BaseConnector
	client *github.Client
	config connector.ConnectorConfig
	mu     sync.RWMutex
	closed bool
}

func NewGitHubConnector(cfg connector.ConnectorConfig, token string) (*GitHubConnector, error) {
	base, err := connector.NewBaseConnector(connector.ConnectorTypeGit, "github")
	if err != nil {
		return nil, err
	}
	// Use explicit HTTP client to allow gock interception in tests if needed
	httpClient := &http.Client{}
	client := github.NewClient(httpClient)
	if token != "" {
		client = client.WithAuthToken(token)
	}
	return &GitHubConnector{
		BaseConnector: base,
		client:        client,
		config:        cfg,
	}, nil
}

// NewGitHubConnectorWithClient allows passing a custom HTTP client (for gock interception)
func NewGitHubConnectorWithClient(cfg connector.ConnectorConfig, token string, httpClient *http.Client) (*GitHubConnector, error) {
	base, err := connector.NewBaseConnector(connector.ConnectorTypeGit, "github")
	if err != nil {
		return nil, err
	}
	client := github.NewClient(httpClient)
	if token != "" {
		client = client.WithAuthToken(token)
	}
	return &GitHubConnector{
		BaseConnector: base,
		client:        client,
		config:        cfg,
	}, nil
}

func (c *GitHubConnector) Validate(operation string, input map[string]interface{}) error {
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

func (c *GitHubConnector) Execute(ctx context.Context, operation string, input map[string]interface{}) (*connector.ConnectorResult, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return nil, fmt.Errorf("connector is closed")
	}
	c.mu.RUnlock()

	switch operation {
	case "create_repository":
		return c.createRepository(ctx, input)
	case "delete_repository":
		return c.deleteRepository(ctx, input)
	case "create_pull_request":
		return c.createPullRequest(ctx, input)
	case "add_repository_collaborator":
		return c.addRepositoryCollaborator(ctx, input)
	case "create_repository_webhook":
		return c.createRepositoryWebhook(ctx, input)
	case "commit_files":
		return c.commitFiles(ctx, input)
	case "read_file":
		return c.readFile(ctx, input)
	default:
		return nil, fmt.Errorf("operation not implemented: %s", operation)
	}
}

func (c *GitHubConnector) createRepository(ctx context.Context, input map[string]interface{}) (*connector.ConnectorResult, error) {
	repo := &github.Repository{
		Name: github.String(input["name"].(string)),
	}

	// Assuming personal repository creation for brevity if owner is empty or omit owner logic for now
	owner := input["owner"].(string)

	result, resp, err := c.client.Repositories.Create(ctx, owner, repo)

	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusForbidden {
			return &connector.ConnectorResult{
				Status: connector.StatusTerminalError,
				Error:  &connector.ConnectorError{Code: "AUTH_FAILURE", Message: "failed to create repository"},
			}, nil
		}
		return nil, err
	}

	return &connector.ConnectorResult{
		Status: connector.StatusSuccess,
		Output: map[string]interface{}{
			"id":       result.GetID(),
			"html_url": result.GetHTMLURL(),
		},
	}, nil
}

func (c *GitHubConnector) deleteRepository(ctx context.Context, input map[string]interface{}) (*connector.ConnectorResult, error) {
	owner := input["owner"].(string)
	name := input["name"].(string)
	_, err := c.client.Repositories.Delete(ctx, owner, name)
	if err != nil {
		return nil, err
	}
	return &connector.ConnectorResult{Status: connector.StatusSuccess}, nil
}

func (c *GitHubConnector) createPullRequest(ctx context.Context, input map[string]interface{}) (*connector.ConnectorResult, error) {
	owner := input["owner"].(string)
	repo := input["repo"].(string)
	newPR := &github.NewPullRequest{
		Title: github.String(input["title"].(string)),
		Head:  github.String(input["head"].(string)),
		Base:  github.String(input["base"].(string)),
	}
	pr, _, err := c.client.PullRequests.Create(ctx, owner, repo, newPR)
	if err != nil {
		return nil, err
	}
	return &connector.ConnectorResult{
		Status: connector.StatusSuccess,
		Output: map[string]interface{}{
			"number": pr.GetNumber(),
			"url":    pr.GetHTMLURL(),
		},
	}, nil
}

func (c *GitHubConnector) addRepositoryCollaborator(ctx context.Context, input map[string]interface{}) (*connector.ConnectorResult, error) {
	owner := input["owner"].(string)
	repo := input["repo"].(string)
	username := input["username"].(string)

	opts := &github.RepositoryAddCollaboratorOptions{}
	_, _, err := c.client.Repositories.AddCollaborator(ctx, owner, repo, username, opts)
	if err != nil {
		return nil, err
	}
	return &connector.ConnectorResult{Status: connector.StatusSuccess}, nil
}

func (c *GitHubConnector) createRepositoryWebhook(ctx context.Context, input map[string]interface{}) (*connector.ConnectorResult, error) {
	owner := input["owner"].(string)
	repo := input["repo"].(string)

	eventsInput := input["events"].([]interface{})
	var events []string
	for _, e := range eventsInput {
		events = append(events, e.(string))
	}

	hook := &github.Hook{
		Config: &github.HookConfig{
			URL:         github.String(input["url"].(string)),
			ContentType: github.String("json"),
		},
		Events: events,
		Active: github.Bool(true),
	}

	createdHook, _, err := c.client.Repositories.CreateHook(ctx, owner, repo, hook)
	if err != nil {
		return nil, err
	}

	return &connector.ConnectorResult{
		Status: connector.StatusSuccess,
		Output: map[string]interface{}{
			"id": createdHook.GetID(),
		},
	}, nil
}

func (c *GitHubConnector) commitFiles(ctx context.Context, input map[string]interface{}) (*connector.ConnectorResult, error) {
	// Parse repo as "owner/name"
	repo := input["repo"].(string)
	branch := input["branch"].(string)
	pathPrefix := input["path_prefix"].(string)
	filesMap := input["files"].(map[string]interface{})
	message := input["message"].(string)
	idempotencyKey := input["idempotency_key"].(string)
	_ = idempotencyKey // Mark as used for now

	// Split repo into owner and name
	parts := splitRepo(repo)
	if parts == nil {
		return &connector.ConnectorResult{
			Status: connector.StatusTerminalError,
			Error: &connector.ConnectorError{
				Code:    "INVALID_REPO",
				Message: "repo must be in format 'owner/name'",
			},
		}, nil
	}
	owner, repoName := parts[0], parts[1]

	// Get the current commit SHA for the branch
	ref, _, err := c.client.Git.GetRef(ctx, owner, repoName, "refs/heads/"+branch)
	if err != nil {
		return &connector.ConnectorResult{
			Status: connector.StatusTerminalError,
			Error: &connector.ConnectorError{
				Code:    "BRANCH_NOT_FOUND",
				Message: fmt.Sprintf("branch %s not found: %v", branch, err),
			},
		}, nil
	}
	commitSHA := ref.GetObject().GetSHA()

	// Get the tree SHA for that commit
	commit, _, err := c.client.Git.GetCommit(ctx, owner, repoName, commitSHA)
	if err != nil {
		return &connector.ConnectorResult{
			Status: connector.StatusTerminalError,
			Error: &connector.ConnectorError{
				Code:    "COMMIT_NOT_FOUND",
				Message: fmt.Sprintf("commit %s not found: %v", commitSHA, err),
			},
		}, nil
	}
	baseTreeSHA := commit.GetTree().GetSHA()

	// Create tree entries for each file
	var treeEntries []*github.TreeEntry
	changed := false
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
		// Check if file exists and content differs
		fileContent, _, _, err := c.client.Repositories.GetContents(ctx, owner, repoName, fullPath, &github.RepositoryContentGetOptions{Ref: branch})
		if err == nil {
			// File exists, compare content
			existingContent, err := fileContent.GetContent()
			if err == nil && existingContent == content {
				// Content identical, skip
				continue
			}
		}
		changed = true
		blob := &github.Blob{
			Content:  github.String(content),
			Encoding: github.String("utf-8"),
		}
		createdBlob, _, err := c.client.Git.CreateBlob(ctx, owner, repoName, blob)
		if err != nil {
			return &connector.ConnectorResult{
				Status: connector.StatusTerminalError,
				Error: &connector.ConnectorError{
					Code:    "BLOB_CREATE_FAILED",
					Message: fmt.Sprintf("failed to create blob for %s: %v", fullPath, err),
				},
			}, nil
		}
		treeEntries = append(treeEntries, &github.TreeEntry{
			Path: github.String(fullPath),
			Mode: github.String("100644"),
			Type: github.String("blob"),
			SHA:  github.String(createdBlob.GetSHA()),
		})
	}

	// If no changes, return early with changed=false
	if !changed {
		return &connector.ConnectorResult{
			Status: connector.StatusSuccess,
			Output: map[string]interface{}{
				"commit_sha": commitSHA,
				"changed":    false,
			},
		}, nil
	}

	// Create new tree
	tree, _, err := c.client.Git.CreateTree(ctx, owner, repoName, baseTreeSHA, treeEntries)
	if err != nil {
		return &connector.ConnectorResult{
			Status: connector.StatusTerminalError,
			Error: &connector.ConnectorError{
				Code:    "TREE_CREATE_FAILED",
				Message: fmt.Sprintf("failed to create tree: %v", err),
			},
		}, nil
	}

	// Create new commit
	newCommit := &github.Commit{
		Message: github.String(message),
		Tree:    tree,
		Parents: []*github.Commit{{SHA: github.String(commitSHA)}},
	}
	createdCommit, _, err := c.client.Git.CreateCommit(ctx, owner, repoName, newCommit, nil)
	if err != nil {
		return &connector.ConnectorResult{
			Status: connector.StatusTerminalError,
			Error: &connector.ConnectorError{
				Code:    "COMMIT_CREATE_FAILED",
				Message: fmt.Sprintf("failed to create commit: %v", err),
			},
		}, nil
	}

	// Update branch reference
	_, _, err = c.client.Git.UpdateRef(ctx, owner, repoName, &github.Reference{
		Ref: github.String("refs/heads/" + branch),
		Object: &github.GitObject{
			SHA: github.String(createdCommit.GetSHA()),
		},
	}, false)
	if err != nil {
		return &connector.ConnectorResult{
			Status: connector.StatusTerminalError,
			Error: &connector.ConnectorError{
				Code:    "REF_UPDATE_FAILED",
				Message: fmt.Sprintf("failed to update branch: %v", err),
			},
		}, nil
	}

	return &connector.ConnectorResult{
		Status: connector.StatusSuccess,
		Output: map[string]interface{}{
			"commit_sha": createdCommit.GetSHA(),
			"changed":    true,
		},
	}, nil
}

func (c *GitHubConnector) readFile(ctx context.Context, input map[string]interface{}) (*connector.ConnectorResult, error) {
	repo := input["repo"].(string)
	branch := input["branch"].(string)
	path := input["path"].(string)

	parts := splitRepo(repo)
	if parts == nil {
		return &connector.ConnectorResult{
			Status: connector.StatusTerminalError,
			Error: &connector.ConnectorError{
				Code:    "INVALID_REPO",
				Message: "repo must be in format 'owner/name'",
			},
		}, nil
	}
	owner, repoName := parts[0], parts[1]

	fileContent, _, resp, err := c.client.Repositories.GetContents(ctx, owner, repoName, path, &github.RepositoryContentGetOptions{Ref: branch})
	if err != nil {
		if resp != nil && resp.StatusCode == 404 {
			return &connector.ConnectorResult{
				Status: connector.StatusTerminalError,
				Error: &connector.ConnectorError{
					Code:    "FILE_NOT_FOUND",
					Message: fmt.Sprintf("file %s not found in repo %s", path, repo),
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

	content, err := fileContent.GetContent()
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
			"content": content,
			"sha":     fileContent.GetSHA(),
		},
	}, nil
}

// Helper function to split repo string "owner/name" into [owner, name]
func splitRepo(repo string) []string {
	parts := strings.Split(repo, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil
	}
	return parts
}

func (c *GitHubConnector) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}

func (c *GitHubConnector) Operations() []connector.OperationMeta {
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
