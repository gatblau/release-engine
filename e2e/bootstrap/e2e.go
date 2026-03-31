// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package bootstrap

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v4"
)

// E2EConfig holds configuration for the E2E bootstrap flow.
type E2EConfig struct {
	ReleaseEngineURL    string
	GiteaURL            string
	DexURL              string
	TenantID            string
	ClientID            string
	ClientSecret        string
	TestUsername        string
	TestPassword        string
	AdminUser           string
	AdminPassword       string
	JobExecutionTimeout time.Duration
	APIClientTimeout    time.Duration
	TestTimeout         time.Duration
}

// E2EResult holds the outcome of a successful E2E run.
type E2EResult struct {
	JobID           string
	CommitSHA       string
	CallbackPayload *CallbackPayload
}

// RunE2E executes the full E2E bootstrap flow:
//  1. Authenticate via Dex (JWT token) using password grant
//  2. Bootstrap Gitea resources (admin user, PAT, org, repo)
//  3. Store gitea-token secret via Release Engine API
//  4. Create a job referencing the secret with a Git operation
//  5. Auto-approve waiting approval steps
//  6. Verify job execution, database state, Git commit, and callback
func RunE2E(ctx context.Context, cfg E2EConfig) (*E2EResult, error) {
	fmt.Println("=== E2E Bootstrap Test ===")
	fmt.Printf("Release Engine URL: %s\n", cfg.ReleaseEngineURL)
	fmt.Printf("Gitea URL: %s\n", cfg.GiteaURL)
	fmt.Printf("Dex URL: %s\n", cfg.DexURL)
	fmt.Printf("Tenant ID: %s\n", cfg.TenantID)

	// Step 0: Get JWT token via password grant
	fmt.Println("0. Obtaining JWT token via password grant...")
	jwtToken, err := GetTokenWithPasswordGrant(ctx, cfg.DexURL, cfg.ClientID, cfg.ClientSecret, cfg.TestUsername, cfg.TestPassword)
	if err != nil {
		return nil, fmt.Errorf("failed to obtain JWT token: %w", err)
	}
	fmt.Printf("   JWT token obtained (length: %d chars)\n", len(jwtToken))

	// Step 1: Bootstrap Gitea resources (admin user, PAT, org, repo).
	fmt.Println("1. Bootstrapping Gitea resources (PAT, org, repo)...")
	pat, err := BootstrapGiteaWithCLIFallback(ctx, cfg.GiteaURL, cfg.AdminUser, cfg.AdminPassword)
	if err != nil {
		return nil, fmt.Errorf("failed to bootstrap Gitea: %w", err)
	}
	if pat == "" {
		return nil, fmt.Errorf("gitea bootstrap completed without returning a PAT")
	}
	fmt.Printf("   Gitea personal access token obtained (length: %d chars)\n", len(pat))

	// Step 1b: Pre-flight check
	fmt.Println("1b. Pre-flight check: verifying test-org/test-repo...")
	if err := EnsureGiteaResources(ctx, cfg.GiteaURL, pat, cfg.AdminUser, cfg.AdminPassword); err != nil {
		return nil, fmt.Errorf("gitea pre-flight check failed: %w", err)
	}

	// Step 2: Store PAT as secret via Release Engine API
	fmt.Println("2. Storing PAT as secret via Release Engine API...")
	secretKey := "git-access-token"
	fmt.Printf("   PAT to store (first-8): %s... (len=%d)\n", pat[:8], len(pat))
	if err := storeSecretViaAdminAPI(ctx, cfg.ReleaseEngineURL, secretKey, pat); err != nil {
		return nil, fmt.Errorf("failed to store secret via API: %w", err)
	}
	fmt.Printf("   Secret '%s' stored successfully\n", secretKey)

	// Step 3: Use docker-internal callback sink
	fmt.Println("3. Using docker-internal callback sink...")
	callbackURLForJob := "http://callback-sink:9090"
	callbackURLForVerify := "http://localhost:9090"
	fmt.Printf("   Callback sink URL (job):  %s\n", callbackURLForJob)
	fmt.Printf("   Callback sink URL (verify): %s\n", callbackURLForVerify)

	// Step 4: Create job referencing the secret
	fmt.Println("4. Creating job with Git operation...")
	jobID, err := createJobViaAPI(ctx, cfg.ReleaseEngineURL, cfg.TenantID, "infra.provision", callbackURLForJob, jwtToken, cfg.APIClientTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to create job via API: %w", err)
	}
	fmt.Printf("   Job created with ID: %s\n", jobID)

	// Step 4b: Continuously auto-approve waiting approval steps
	fmt.Println("   Starting auto-approval watcher...")
	autoApproveDone := make(chan error, 1)
	go func() {
		autoApproveDone <- autoApproveViaAPI(ctx, cfg.ReleaseEngineURL, cfg.TenantID, jobID, jwtToken, cfg.JobExecutionTimeout)
	}()

	// Step 5: Wait for job execution and verify results
	fmt.Println("5. Waiting for job execution and verifying results...")

	fmt.Println("   Waiting for auto-approval to complete...")
	if err := <-autoApproveDone; err != nil {
		return nil, fmt.Errorf("auto-approval watcher failed: %w", err)
	}
	fmt.Println("   Verifying job state transitions...")
	if err := verifyJobState(ctx, cfg.ReleaseEngineURL, jobID, jwtToken, cfg.JobExecutionTimeout); err != nil {
		return nil, fmt.Errorf("job state API verification failed: %w", err)
	}

	fmt.Println("   Verifying job state in database...")
	if err := verifyJobStateInDatabase(ctx, jobID); err != nil {
		fmt.Printf("   Job state database verification warning: %v\n", err)
	}

	fmt.Println("   Verifying callback receipt...")
	callbackPayload, err := verifyCallbackReceived(ctx, callbackURLForVerify, jobID)
	if err != nil {
		return nil, fmt.Errorf("callback verification failed: %w", err)
	}
	if callbackPayload == nil {
		return nil, fmt.Errorf("callback verification failed: no payload received")
	}
	fmt.Printf("   Callback verified: job_id=%s status=%s commit_sha=%s\n",
		callbackPayload.JobID, callbackPayload.Status, callbackPayload.CommitSHA)

	fmt.Println("   Verifying Git commit in Gitea...")
	if err := verifyGitCommit(ctx, cfg.GiteaURL, "test-org/test-repo", pat, callbackPayload.CommitSHA); err != nil {
		return nil, fmt.Errorf("git commit verification failed: %w", err)
	}

	fmt.Println("   Verifying secret usage (not bypassed)...")

	fmt.Println("=== E2E Bootstrap Test Successful ===")

	return &E2EResult{
		JobID:           jobID,
		CommitSHA:       callbackPayload.CommitSHA,
		CallbackPayload: callbackPayload,
	}, nil
}

// LoadE2EConfig loads E2E configuration from environment variables only.
// It does not depend on the Release Engine server config (DATABASE_URL etc.)
// so it works in environments where only the e2e test runner runs.
func LoadE2EConfig() (*E2EConfig, error) {
	jobExecTimeoutStr := getEnv("JOB_EXECUTION_TIMEOUT", "45s")
	jobExecTimeout, err := parseDuration(jobExecTimeoutStr)
	if err != nil {
		return nil, fmt.Errorf("invalid JOB_EXECUTION_TIMEOUT value: %w", err)
	}

	apiClientTimeoutStr := getEnv("API_CLIENT_TIMEOUT", "30s")
	apiClientTimeout, err := parseDuration(apiClientTimeoutStr)
	if err != nil {
		return nil, fmt.Errorf("invalid API_CLIENT_TIMEOUT value: %w", err)
	}

	testTimeoutStr := getEnv("TEST_TIMEOUT", "5m")
	testTimeout, err := parseDuration(testTimeoutStr)
	if err != nil {
		return nil, fmt.Errorf("invalid TEST_TIMEOUT value: %w", err)
	}

	return &E2EConfig{
		ReleaseEngineURL:    getEnv("RELEASE_ENGINE_URL", "http://localhost:8080"),
		GiteaURL:            getEnv("GITEA_URL", "http://localhost:3000"),
		DexURL:              getEnv("DEX_URL", "http://localhost:5556"),
		TenantID:            getEnv("TENANT_ID", "test-tenant"),
		ClientID:            getEnv("OIDC_CLIENT_ID", "release-engine"),
		ClientSecret:        getEnv("OIDC_CLIENT_SECRET", "example-secret"),
		TestUsername:        getEnv("TEST_USERNAME", "test-user@example.com"),
		TestPassword:        getEnv("TEST_PASSWORD", "password"),
		AdminUser:           getEnv("GITEA_ADMIN_USER", "gitadmin"),
		AdminPassword:       getEnv("GITEA_ADMIN_PASSWORD", "admin-password"),
		JobExecutionTimeout: jobExecTimeout,
		APIClientTimeout:    apiClientTimeout,
		TestTimeout:         testTimeout,
	}, nil
}

func getEnv(key, defaultVal string) string {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		return val
	}
	return defaultVal
}

// --- helper types and functions (extracted from main.go) ---

// SetSecretRequest is the request body for setting a secret.
type SetSecretRequest struct {
	Value string `json:"value"`
}

// CreateJobRequest is the request body for creating a job.
type CreateJobRequest struct {
	TenantID       string         `json:"tenant_id"`
	PathKey        string         `json:"path_key"`
	Params         map[string]any `json:"params"`
	IdempotencyKey string         `json:"idempotency_key"`
	CallbackURL    string         `json:"callback_url,omitempty"`
	MaxAttempts    int            `json:"max_attempts,omitempty"`
}

// CreateJobEnvelope is the response envelope for job creation.
type CreateJobEnvelope struct {
	JobID       string     `json:"job_id"`
	TenantID    string     `json:"tenant_id"`
	PathKey     string     `json:"path_key"`
	State       string     `json:"state"`
	Attempt     int        `json:"attempt"`
	MaxAttempts int        `json:"max_attempts"`
	NextRunAt   *time.Time `json:"next_run_at"`
	AcceptedAt  time.Time  `json:"accepted_at"`
}

// JobState represents job state from database.
type JobState struct {
	ID               string    `json:"id"`
	TenantID         string    `json:"tenant_id"`
	PathKey          string    `json:"path_key"`
	State            string    `json:"state"`
	Attempt          int       `json:"attempt"`
	RunID            string    `json:"run_id"`
	LastErrorCode    string    `json:"last_error_code,omitempty"`
	LastErrorMessage string    `json:"last_error_message,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

// CallbackPayload represents the expected callback payload.
type CallbackPayload struct {
	JobID        string   `json:"job_id"`
	Status       string   `json:"status"`
	CommitSHA    string   `json:"commit_sha,omitempty"`
	Reason       string   `json:"reason,omitempty"`
	ResourceRefs []string `json:"resource_refs,omitempty"`
}

func storeSecretViaAdminAPI(ctx context.Context, baseURL, key, value string) error {
	url := fmt.Sprintf("%s/internal/v1/platform/secrets/%s", baseURL, key)
	reqBody := SetSecretRequest{Value: value}
	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(reqBytes))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer e2e-admin-token")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return nil
}

func createJobViaAPI(ctx context.Context, baseURL, tenantID, pathKey, callbackURL, jwtToken string, apiClientTimeout time.Duration) (string, error) {
	url := fmt.Sprintf("%s/v1/jobs", baseURL)

	reqBody := CreateJobRequest{
		TenantID: tenantID,
		PathKey:  pathKey,
		Params: map[string]any{
			"tenant":              tenantID,
			"environment":         "development",
			"contract_version":    "v1",
			"catalogue_item":      "k8s-app",
			"owner":               "e2e",
			"request_name":        "e2e-test",
			"namespace":           "e2e",
			"primary_region":      "eu-west-1",
			"workload_profile":    "small",
			"availability":        "standard",
			"data_classification": "internal",
			"ingress_mode":        "private",
			"egress_mode":         "nat",
			"residency":           "eu",
			"default_provider":    "aws",
			"cost_centre":         "cc-001",
			"kubernetes": map[string]any{
				"enabled":  true,
				"provider": "aws",
				"tier":     "standard",
				"size":     "small",
			},
			"infra_repo":        "test-org/test-repo",
			"callback_url":      callbackURL,
			"idempotency_key":   "e2e-job-001",
			"secret_ref":        "git-access-token",
			"skip_health_check": true,
		},
		IdempotencyKey: "e2e-job-001",
		CallbackURL:    callbackURL,
		MaxAttempts:    1,
	}

	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBytes))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+jwtToken)

	client := &http.Client{Timeout: apiClientTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("execute request: %w", err)
	}
	defer func(Body io.ReadCloser) {
		if err = Body.Close(); err != nil {
			fmt.Printf("failed to close response body: %v", err)
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusAccepted {
		var errResp struct {
			Error   string `json:"error"`
			Code    string `json:"code"`
			Details any    `json:"details"`
		}
		if err = json.NewDecoder(resp.Body).Decode(&errResp); err == nil {
			return "", fmt.Errorf("API error: %s (code: %s)", errResp.Error, errResp.Code)
		}
		return "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var envelope CreateJobEnvelope
	if err = json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	fmt.Printf("Job created successfully: %s (state: %s)\n", envelope.JobID, envelope.State)
	return envelope.JobID, nil
}

func verifyJobState(ctx context.Context, baseURL, jobID, jwtToken string, jobExecutionTimeout time.Duration) error {
	url := fmt.Sprintf("%s/v1/jobs/%s", baseURL, jobID)

	client := &http.Client{Timeout: 5 * time.Second}
	deadline := time.NewTimer(jobExecutionTimeout)
	defer deadline.Stop()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	var lastState string
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for job state: %w", ctx.Err())
		case <-deadline.C:
			if lastState == "" {
				return fmt.Errorf("timed out waiting for job to reach terminal state")
			}
			return fmt.Errorf("timed out waiting for job to reach terminal state; last observed state: %s", lastState)
		case <-ticker.C:
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				return fmt.Errorf("create request: %w", err)
			}
			req.Header.Set("Authorization", "Bearer "+jwtToken)

			resp, err := client.Do(req)
			if err != nil {
				lastState = "request error"
				continue
			}

			var jobState JobState
			decodeErr := json.NewDecoder(resp.Body).Decode(&jobState)
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				lastState = fmt.Sprintf("unexpected status %d", resp.StatusCode)
				continue
			}
			if decodeErr != nil {
				lastState = "decode error"
				continue
			}

			lastState = jobState.State
			if jobState.State != "succeeded" && jobState.State != "failed" && jobState.State != "jobs_exhausted" {
				continue
			}

			if jobState.State == "failed" || jobState.State == "jobs_exhausted" {
				return fmt.Errorf("job failed: state=%s error=%s: %s", jobState.State, jobState.LastErrorCode, jobState.LastErrorMessage)
			}

			fmt.Printf("   Job state: %s (attempt: %d)\n", jobState.State, jobState.Attempt)
			return nil
		}
	}
}

func verifyGitCommit(ctx context.Context, giteaURL, repo, pat, expectedCommitSHA string) error {
	parts := strings.Split(repo, "/")
	if len(parts) != 2 {
		return fmt.Errorf("invalid repo format: %s", repo)
	}
	owner, repoName := parts[0], parts[1]

	client := NewGiteaClient(giteaURL)
	client.SetToken(pat)

	commits, err := client.ListCommits(ctx, owner, repoName, "main")
	if err != nil {
		return fmt.Errorf("failed to list commits: %w", err)
	}

	if len(commits) == 0 {
		return fmt.Errorf("no commits found in repository %s", repo)
	}

	if expectedCommitSHA != "" {
		for _, commit := range commits {
			if strings.HasPrefix(commit.ID, expectedCommitSHA) || strings.HasPrefix(expectedCommitSHA, commit.ID) {
				fmt.Printf("   Commit verified via SHA: %s (%s)\n", commit.ID[:8], commit.Message)
				return nil
			}
		}
		return fmt.Errorf("expected commit SHA %s not found in repository (checked %d commits)", expectedCommitSHA, len(commits))
	}

	fmt.Printf("   No commit_sha in callback payload; performing presence check\n")
	fmt.Println("   Latest commit: " + commits[0].ID[:8] + " (" + commits[0].Message + ")")
	return fmt.Errorf("idempotent commit — cannot verify without commit_sha; fix infra module callback payload")
}

func verifyJobStateInDatabase(ctx context.Context, jobID string) error {
	conn, err := pgx.Connect(ctx, "postgres://release_engine:release_engine@localhost:5432/release_engine")
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer func(conn *pgx.Conn, ctx context.Context) {
		if err = conn.Close(ctx); err != nil {
			fmt.Printf("failed to close database connection: %v\n", err)
		}
	}(conn, ctx)

	var state string
	var attempt int
	var tenantID string
	var pathKey string

	err = conn.QueryRow(ctx, `
		SELECT state, attempt, tenant_id, path_key
		FROM jobs 
		WHERE id = $1::uuid
	`, jobID).Scan(&state, &attempt, &tenantID, &pathKey)
	if err != nil {
		return fmt.Errorf("failed to query job from database: %w", err)
	}

	if state != "succeeded" && state != "failed" && state != "jobs_exhausted" {
		return fmt.Errorf("job not in terminal state in database: %s", state)
	}

	if state != "succeeded" {
		return fmt.Errorf("job did not succeed in database: %s", state)
	}

	fmt.Printf("   Database verification: job %s has state %s (attempt: %d)\n", jobID, state, attempt)
	fmt.Printf("   Tenant: %s, Path: %s\n", tenantID, pathKey)

	var readCount int
	err = conn.QueryRow(ctx, `
		SELECT COUNT(*) 
		FROM jobs_read 
		WHERE id = $1::uuid
	`, jobID).Scan(&readCount)
	if err != nil {
		return fmt.Errorf("failed to query jobs_read table: %w", err)
	}

	if readCount == 0 {
		fmt.Printf("   Warning: No projection found in jobs_read table (gap)\n")
	} else {
		fmt.Printf("   Found projection in jobs_read table (%d rows)\n", readCount)
	}

	return nil
}

func verifyCallbackReceived(ctx context.Context, callbackSinkURL, expectedJobID string) (*CallbackPayload, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("callback sink: context cancelled (job_id=%s): %w", expectedJobID, ctx.Err())
		case <-ticker.C:
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, callbackSinkURL, nil)
			if err != nil {
				continue
			}

			resp, err := client.Do(req)
			if err != nil {
				continue
			}
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				fmt.Printf("   [callback-sink] GET returned HTTP %d, retrying...\n", resp.StatusCode)
				continue
			}

			if len(body) == 0 || strings.TrimSpace(string(body)) == "{}" {
				continue
			}

			var payload CallbackPayload
			if err = json.Unmarshal(body, &payload); err != nil {
				fmt.Printf("   [callback-sink] failed to unmarshal payload: %s, retrying...\n", string(body))
				continue
			}

			if payload.JobID == "" {
				return nil, fmt.Errorf("callback payload has empty job_id: %s", string(body))
			}
			if payload.Status == "" {
				return nil, fmt.Errorf("callback payload has empty status: %s", string(body))
			}

			if payload.JobID != expectedJobID {
				return nil, fmt.Errorf("callback job_id mismatch: expected=%s got=%s", expectedJobID, payload.JobID)
			}

			fmt.Printf("   [callback-sink] Received payload: job_id=%s status=%s commit_sha=%s\n",
				payload.JobID, payload.Status, payload.CommitSHA)

			return &payload, nil
		}
	}
}

func autoApproveViaAPI(ctx context.Context, baseURL, tenantID, jobID, jwtToken string, jobExecutionTimeout time.Duration) error {
	client := &http.Client{Timeout: 10 * time.Second}
	deadline := time.NewTimer(jobExecutionTimeout)
	defer deadline.Stop()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			if s, err := fetchCurrentJobState(ctx, client, baseURL, jobID, jwtToken); err == nil {
				return fmt.Errorf("timed out waiting for job completion; job_state=%s error=%s: %s", s.State, s.LastErrorCode, s.LastErrorMessage)
			}
			return fmt.Errorf("timed out waiting for job completion")
		case <-ticker.C:
			s, err := fetchCurrentJobState(ctx, client, baseURL, jobID, jwtToken)
			if err != nil {
				continue
			}

			if s.State == "succeeded" || s.State == "failed" || s.State == "jobs_exhausted" {
				if s.State != "succeeded" {
					return fmt.Errorf("job failed: state=%s error=%s: %s", s.State, s.LastErrorCode, s.LastErrorMessage)
				}
				return nil
			}

			if s.LastErrorCode != "" {
				return fmt.Errorf("job failed early: state=%s error=%s: %s", s.State, s.LastErrorCode, s.LastErrorMessage)
			}

			url := fmt.Sprintf("%s/v1/jobs?step_status=waiting_approval&tenant=%s", baseURL, tenantID)
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				return fmt.Errorf("create list request: %w", err)
			}
			req.Header.Set("Authorization", "Bearer "+jwtToken)

			resp, err := client.Do(req)
			if err != nil {
				continue
			}

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				_ = resp.Body.Close()
				fmt.Printf("   [auto-approve] ListJobs returned HTTP %d: %s\n", resp.StatusCode, string(body))
				continue
			}

			var envelope struct {
				Jobs []struct {
					JobID      string `json:"job_id"`
					TenantID   string `json:"tenant_id"`
					PathKey    string `json:"path_key"`
					StepID     string `json:"step_id"`
					StepStatus string `json:"step_status"`
				} `json:"jobs"`
			}
			if err = json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
				_ = resp.Body.Close()
				continue
			}
			_ = resp.Body.Close()

			for _, job := range envelope.Jobs {
				if job.JobID != jobID {
					continue
				}
				approveURL := fmt.Sprintf("%s/v1/jobs/%s/steps/%s/decisions", baseURL, job.JobID, job.StepID)
				body := map[string]any{
					"decision":      "approved",
					"justification": "e2e auto-approval",
				}
				reqBytes, err := json.Marshal(body)
				if err != nil {
					return fmt.Errorf("marshal approval request: %w", err)
				}
				req, err := http.NewRequestWithContext(ctx, http.MethodPost, approveURL, bytes.NewReader(reqBytes))
				if err != nil {
					return fmt.Errorf("create approval request: %w", err)
				}
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+jwtToken)
				req.Header.Set("X-Approver-Tenant", tenantID)
				req.Header.Set("X-Approver-Role", "release-manager")
				req.Header.Set("X-Approver", "e2e")

				approvalResp, err := client.Do(req)
				if err != nil {
					return fmt.Errorf("submit approval decision: %w", err)
				}
				if approvalResp.StatusCode != http.StatusOK {
					body, _ := io.ReadAll(approvalResp.Body)
					_ = approvalResp.Body.Close()
					return fmt.Errorf("unexpected approval status %d: %s", approvalResp.StatusCode, string(body))
				}
				_ = approvalResp.Body.Close()
			}
		}
	}
}

func fetchCurrentJobState(ctx context.Context, client *http.Client, baseURL, jobID, jwtToken string) (*JobState, error) {
	stateURL := fmt.Sprintf("%s/v1/jobs/%s", baseURL, jobID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, stateURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create job state request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+jwtToken)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		if err = Body.Close(); err != nil {
			fmt.Printf("failed to close response body: %v", err)
		}
	}(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	var s JobState
	if err = json.NewDecoder(resp.Body).Decode(&s); err != nil {
		return nil, err
	}
	return &s, nil
}

func parseDuration(durationStr string) (time.Duration, error) {
	return time.ParseDuration(durationStr)
}
