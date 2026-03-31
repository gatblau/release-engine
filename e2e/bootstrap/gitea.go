// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// GiteaClient provides a client for interacting with Gitea API.
type GiteaClient struct {
	baseURL    string
	httpClient *http.Client
	token      string
}

// NewGiteaClient creates a new Gitea client.
func NewGiteaClient(baseURL string) *GiteaClient {
	return &GiteaClient{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// SetToken sets the authentication token for the client.
func (c *GiteaClient) SetToken(token string) {
	c.token = token
}

// Login authenticates with user credentials and returns a session token (PAT).
// The scopes slice defines the required OAuth2 scopes for the token.
// If a token with the given name already exists (HTTP 400), it deletes the
// existing one and retries, so callers need not track pre-existing tokens.
func (c *GiteaClient) Login(ctx context.Context, username, password string, scopes []string) (string, error) {
	tokenName := "e2e-bootstrap"

	// Helper to create a token and return the response body bytes.
	createToken := func() ([]byte, int, error) {
		reqBody := map[string]any{
			"name":   tokenName,
			"scopes": scopes,
		}
		reqBytes, err := json.Marshal(reqBody)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal login request: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, "POST",
			c.baseURL+"/api/v1/users/"+username+"/tokens",
			strings.NewReader(string(reqBytes)))
		if err != nil {
			return nil, 0, fmt.Errorf("create login request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.SetBasicAuth(username, password)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, 0, fmt.Errorf("execute login request: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()

		body, _ := io.ReadAll(resp.Body)
		return body, resp.StatusCode, nil
	}

	body, status, err := createToken()
	if err != nil {
		return "", fmt.Errorf("login failed: %w", err)
	}

	// Defensively handle "token name already used" by deleting and retrying.
	// This covers the case where a previous run created the token but the
	// process was interrupted before it could be used.
	if status == http.StatusBadRequest {
		if strings.Contains(string(body), "access token name has been used") ||
			strings.Contains(string(body), "token name already used") {
			fmt.Printf("[gitea-bootstrap] Token '%s' already exists, deleting and recreating...\n", tokenName)
			if err := c.deleteToken(ctx, username, password, tokenName); err != nil {
				return "", fmt.Errorf("failed to delete stale token: %w", err)
			}
			body, status, err = createToken()
			if err != nil {
				return "", fmt.Errorf("login failed after cleanup: %w", err)
			}
		}
	}

	if status != http.StatusCreated {
		return "", fmt.Errorf("login failed with status %d: %s", status, string(body))
	}

	var tokenResp struct {
		SHA1 string `json:"sha1"` // when scopes are provided, Gitea returns the full token as sha1
	}
	if err := json.NewDecoder(strings.NewReader(string(body))).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}

	c.token = tokenResp.SHA1
	return tokenResp.SHA1, nil
}

// deleteToken deletes a personal access token by name using the user's credentials.
func (c *GiteaClient) deleteToken(ctx context.Context, username, password, tokenName string) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE",
		c.baseURL+"/api/v1/users/"+username+"/tokens/"+tokenName, nil)
	if err != nil {
		return fmt.Errorf("create delete token request: %w", err)
	}
	req.SetBasicAuth(username, password)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute delete token request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// 204 = deleted, 404 = already gone — both are fine for cleanup.
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete token failed with status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// RequiredScopes is the canonical list of scopes every PAT must have for use
// with the release-engine Gitea connector.  "read:user" is required so that
// Gitea can resolve org owners via GetUserByName when the connector calls
// scoped repo APIs (e.g. /repos/{owner}/{repo}/branches).  "write:organization"
// is required to create organizations via the API.
var RequiredScopes = []string{"read:user", "read:repository", "write:repository", "write:organization"}

// bootstrapScopes is used internally by the bootstrap logic to create a token
// capable of calling the token management API.  It extends RequiredScopes with
// "write:user" because Gitea requires that scope on the authenticating token
// when creating new tokens via POST /api/v1/users/{username}/tokens.
var bootstrapScopes = []string{"write:user", "read:user", "read:repository", "write:repository", "write:organization"}

// CreatePersonalAccessToken creates a new personal access token for a user.
// Requires the client to already have a valid admin token.
// After successful creation, stores the new token on the client.
func (c *GiteaClient) CreatePersonalAccessToken(ctx context.Context, username, tokenName string, scopes []string) (string, error) {
	if c.token == "" {
		return "", fmt.Errorf("authentication token required")
	}

	reqBody := map[string]interface{}{
		"name":   tokenName,
		"scopes": scopes,
	}
	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal create token request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		c.baseURL+"/api/v1/users/"+username+"/tokens",
		strings.NewReader(string(reqBytes)))
	if err != nil {
		return "", fmt.Errorf("create create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "token "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("execute create token request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("create token failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		SHA1 string `json:"sha1"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}

	// Store the PAT on the client so subsequent operations (org/repo creation)
	// use the fully-scoped token rather than any intermediate bootstrap token.
	c.token = tokenResp.SHA1

	return tokenResp.SHA1, nil
}

// CreatePersonalAccessTokenWithBasicAuth creates a new personal access token using
// Basic Auth (username + password) instead of a bearer token.  This is used for
// creating the canonical PAT directly from admin credentials without needing to
// parse/reuse an intermediate bootstrap token.
// After successful creation, stores the new token on the client.
func (c *GiteaClient) CreatePersonalAccessTokenWithBasicAuth(ctx context.Context, username, password, tokenName string, scopes []string) (string, error) {
	reqBody := map[string]interface{}{
		"name":   tokenName,
		"scopes": scopes,
	}
	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal create token request: %w", err)
	}

	makeReq := func() (*http.Request, error) {
		return http.NewRequestWithContext(ctx, "POST",
			c.baseURL+"/api/v1/users/"+username+"/tokens",
			strings.NewReader(string(reqBytes)))
	}

	req, err := makeReq()
	if err != nil {
		return "", fmt.Errorf("create create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(username, password)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("execute create token request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Defensively handle "token name already used" by deleting and retrying.
	if resp.StatusCode == http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		if strings.Contains(string(body), "access token name has been used") ||
			strings.Contains(string(body), "token name already used") {
			fmt.Printf("[gitea-bootstrap] Token '%s' already exists (via Basic Auth path), deleting and recreating...\n", tokenName)
			if err := c.deleteToken(ctx, username, password, tokenName); err != nil {
				return "", fmt.Errorf("failed to delete stale token: %w", err)
			}
			// Retry once
			req, err = makeReq()
			if err != nil {
				return "", fmt.Errorf("recreate create token request: %w", err)
			}
			req.Header.Set("Content-Type", "application/json")
			req.SetBasicAuth(username, password)
			resp, err = c.httpClient.Do(req)
			if err != nil {
				return "", fmt.Errorf("execute recreate token request: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()
		}
	}

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("create token failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		SHA1 string `json:"sha1"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}

	// Store the PAT on the client so subsequent operations (org/repo creation)
	// use the fully-scoped token rather than any intermediate bootstrap token.
	c.token = tokenResp.SHA1

	return tokenResp.SHA1, nil
}

// CreateOrganization creates a new organization.  HTTP 409 is treated as
// "already exists" (success) rather than an error.
func (c *GiteaClient) CreateOrganization(ctx context.Context, orgName, description string) error {
	if c.token == "" {
		return fmt.Errorf("authentication token required")
	}

	reqBody := map[string]interface{}{
		"username":    orgName,
		"full_name":   orgName,
		"description": description,
		"visibility":  "public",
	}
	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal create org request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		c.baseURL+"/api/v1/orgs",
		strings.NewReader(string(reqBytes)))
	if err != nil {
		return fmt.Errorf("create org request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "token "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute create org request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// 201 = created, 409 = already exists — both are success for bootstrap
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusConflict {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create organization failed with status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// CreateRepository creates a new repository under an organization.
// HTTP 409 is treated as "already exists" (success).
func (c *GiteaClient) CreateRepository(ctx context.Context, orgName, repoName, description string, autoInit bool) error {
	if c.token == "" {
		return fmt.Errorf("authentication token required")
	}

	reqBody := map[string]interface{}{
		"name":           repoName,
		"description":    description,
		"private":        false,
		"auto_init":      autoInit,
		"default_branch": "main",
	}
	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal create repo request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		c.baseURL+"/api/v1/orgs/"+orgName+"/repos",
		strings.NewReader(string(reqBytes)))
	if err != nil {
		return fmt.Errorf("create repo request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "token "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute create repo request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// 201 = created, 409 = already exists — both are success for bootstrap
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusConflict {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create repository failed with status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// RepositoryExists checks whether a repository exists by attempting to fetch it.
func (c *GiteaClient) RepositoryExists(ctx context.Context, owner, repo string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		c.baseURL+"/api/v1/repos/"+owner+"/"+repo, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "token "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("unexpected status %d checking repo existence", resp.StatusCode)
	}
	return true, nil
}

// BranchExists checks whether a branch exists in the given repository.
func (c *GiteaClient) BranchExists(ctx context.Context, owner, repo, branch string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		c.baseURL+"/api/v1/repos/"+owner+"/"+repo+"/branches/"+branch, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "token "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("unexpected status %d checking branch existence", resp.StatusCode)
	}
	return true, nil
}

// Commit represents a Gitea commit.
type Commit struct {
	ID      string `json:"sha"`
	Message string `json:"message"`
	Author  struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"author"`
	Committer struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"committer"`
	Created time.Time `json:"created"`
}

// ListCommits lists commits for a repository.
func (c *GiteaClient) ListCommits(ctx context.Context, owner, repo, branch string) ([]Commit, error) {
	if c.token == "" {
		return nil, fmt.Errorf("authentication token required")
	}

	req, err := http.NewRequestWithContext(ctx, "GET",
		c.baseURL+"/api/v1/repos/"+owner+"/"+repo+"/commits?sha="+branch, nil)
	if err != nil {
		return nil, fmt.Errorf("create list commits request: %w", err)
	}
	req.Header.Set("Authorization", "token "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute list commits request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list commits failed with status %d", resp.StatusCode)
	}

	var commits []Commit
	if err = json.NewDecoder(resp.Body).Decode(&commits); err != nil {
		return nil, fmt.Errorf("decode commits response: %w", err)
	}

	return commits, nil
}

// BootstrapGitea is the single entry point for all Gitea bootstrap operations.
// It creates the canonical PAT (with RequiredScopes), ensures test-org and
// test-org/test-repo exist, and verifies the main branch is present.
// Returns the PAT token for use as the git-access-token platform secret.
func BootstrapGitea(ctx context.Context, baseURL, adminUser, adminPassword string) (string, error) {
	client := NewGiteaClient(baseURL)

	// Step 1: Authenticate as admin to get a bootstrap token.
	// This token is only used to create the canonical PAT — it is not returned.
	// bootstrapScopes includes write:user so the bootstrap token can in turn
	// create the canonical PAT (Gitea requires write:user to call the tokens API).
	fmt.Println("[gitea-bootstrap] Authenticating as admin to obtain bootstrap token...")
	_, err := client.Login(ctx, adminUser, adminPassword, bootstrapScopes)
	if err != nil {
		return "", fmt.Errorf("admin login failed: %w", err)
	}

	// Step 2: Create the canonical PAT for release-engine with RequiredScopes.
	// Use Basic Auth directly so we don't depend on the bootstrap token from
	// Step 1.  This avoids the 401 "auth required" error caused by Gitea not
	// accepting the intermediate bootstrap token for this endpoint.
	// CreatePersonalAccessTokenWithBasicAuth stores the new PAT on client.token
	// so subsequent operations use the fully-scoped token.
	fmt.Println("[gitea-bootstrap] Creating canonical PAT with required scopes:", RequiredScopes)
	pat, err := client.CreatePersonalAccessTokenWithBasicAuth(ctx, adminUser, adminPassword, "release-engine-e2e", RequiredScopes)
	if err != nil {
		return "", fmt.Errorf("failed to create PAT: %w", err)
	}
	fmt.Printf("[gitea-bootstrap] PAT created (first-8: %s...)\n", pat[:8])

	// Step 3: Create test-org if it doesn't exist.
	fmt.Println("[gitea-bootstrap] Ensuring test-org exists...")
	if err := client.CreateOrganization(ctx, "test-org", "Test organization for release-engine"); err != nil {
		return "", fmt.Errorf("failed to create test-org: %w", err)
	}
	fmt.Println("[gitea-bootstrap] test-org ready")

	// Step 4: Create test-org/test-repo with auto_init=true so main branch exists.
	fmt.Println("[gitea-bootstrap] Ensuring test-org/test-repo exists (auto_init=true)...")
	if err := client.CreateRepository(ctx, "test-org", "test-repo", "Test repository for release-engine", true); err != nil {
		return "", fmt.Errorf("failed to create test-repo: %w", err)
	}
	fmt.Println("[gitea-bootstrap] test-org/test-repo ready")

	// Step 5: Verify main branch exists (auto_init should have created it).
	// Poll for up to 30 seconds to allow Gitea to finish initialization.
	fmt.Println("[gitea-bootstrap] Verifying main branch exists...")
	branchExists := false
	for i := 0; i < 15; i++ {
		exists, err := client.BranchExists(ctx, "test-org", "test-repo", "main")
		if err == nil && exists {
			branchExists = true
			break
		}
		if i < 14 {
			fmt.Printf("[gitea-bootstrap] main branch not yet visible, retrying... (%d/15)\n", i+1)
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(2 * time.Second):
			}
		}
	}
	if !branchExists {
		return "", fmt.Errorf("branch 'main' not found in test-org/test-repo after 30 seconds")
	}
	fmt.Println("[gitea-bootstrap] main branch confirmed")

	// Step 6: Verify the PAT works by checking we can reach the repo with it.
	fmt.Println("[gitea-bootstrap] Verifying PAT is functional...")
	ok, err := client.BranchExists(ctx, "test-org", "test-repo", "main")
	if err != nil {
		return "", fmt.Errorf("PAT verification failed: %w", err)
	}
	if !ok {
		return "", fmt.Errorf("PAT verification failed: branch disappeared between checks")
	}
	fmt.Println("[gitea-bootstrap] PAT verified successfully")

	return pat, nil
}

// EnsureGiteaResources verifies that test-org/test-repo and the main branch
// exist using the provided PAT.  If they don't exist it attempts to create them
// using admin credentials (if supplied).  This is a pre-flight check to be
// called before creating a job, ensuring the infra module's git connector
// won't fail due to missing resources.
func EnsureGiteaResources(ctx context.Context, baseURL, pat, adminUser, adminPassword string) error {
	client := NewGiteaClient(baseURL)
	client.SetToken(pat)

	// Quick path: check if repo + main branch already exist with the PAT.
	exists, err := client.BranchExists(ctx, "test-org", "test-repo", "main")
	if err != nil {
		return fmt.Errorf("failed to check branch existence: %w", err)
	}
	if exists {
		fmt.Println("[gitea-preflight] test-org/test-repo:main already exists, no action needed")
		return nil
	}

	// Branch doesn't exist (or repo doesn't exist).  Use admin credentials to fix.
	fmt.Println("[gitea-preflight] test-org/test-repo or main branch missing, attempting to create...")
	if adminUser == "" || adminPassword == "" {
		return fmt.Errorf("cannot create missing Gitea resources: admin credentials not provided")
	}

	adminClient := NewGiteaClient(baseURL)
	// Use CreatePersonalAccessTokenWithBasicAuth so the client stores the
	// fully-scoped PAT and subsequent org/repo calls succeed without needing
	// to explicitly set the token again.
	if _, err := adminClient.CreatePersonalAccessTokenWithBasicAuth(ctx, adminUser, adminPassword, "e2e-preflight", bootstrapScopes); err != nil {
		return fmt.Errorf("admin token creation failed during preflight: %w", err)
	}

	if err := adminClient.CreateOrganization(ctx, "test-org", "Test organization for release-engine"); err != nil {
		return fmt.Errorf("failed to create test-org: %w", err)
	}

	if err := adminClient.CreateRepository(ctx, "test-org", "test-repo", "Test repository for release-engine", true); err != nil {
		return fmt.Errorf("failed to create test-repo: %w", err)
	}

	// Re-verify with the original PAT.
	client.SetToken(pat)
	for i := 0; i < 15; i++ {
		exists, err := client.BranchExists(ctx, "test-org", "test-repo", "main")
		if err == nil && exists {
			fmt.Println("[gitea-preflight] test-org/test-repo:main created and verified")
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}

	return fmt.Errorf("branch 'main' still not visible after creation attempts")
}
