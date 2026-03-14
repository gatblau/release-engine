// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package github

import (
	"context"
	"fmt"
	"net/http"
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
	}
}
