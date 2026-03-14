## SPEC: GitHubConnector

- **File:** `internal/connector/github/github.go`
- **Package:** `github`
- **Build Phase:** 2
- **Dependencies:** BaseConnector, ConnectorConfig, VoltaManager

#### Purpose

Implements GitHub-specific connector for Git operations including repository management, PR creation, and webhook handling.

#### Public Interface

```go
type GitHubConnector struct {
    connector.BaseConnector
    client *github.Client // google/go-github client
    config connector.ConnectorConfig
}

func NewGitHubConnector(cfg connector.ConnectorConfig, token string) (*GitHubConnector, error)
func (c *GitHubConnector) Validate(operation string, input map[string]interface{}) error
func (c *GitHubConnector) Execute(ctx context.Context, operation string, input map[string]interface{}) (*connector.ConnectorResult, error)
func (c *GitHubConnector) Close() error

// Implements connector.OperationDescriber
func (c *GitHubConnector) Operations() []connector.OperationMeta
```

Supported operations:
- `create_repository`
- `delete_repository`
- `create_pull_request`
- `add_repository_collaborator`
- `create_repository_webhook`

#### Internal Logic

1. Validates input fields per operation using `Validate()`.
2. Extracts `call_id` from context via `CallIDFromContext(ctx)` and sets it as an idempotency header where supported.
3. Executes GitHub API call using the injected `*github.Client`.
4. Handles GitHub-specific rate limiting: detects `403` with `Retry-After` header and returns `retryable_error`.
5. Maps GitHub API errors to `ConnectorResult`:
- 401/403 (non-rate-limit) → `terminal_error` / `AUTH_FAILED`
- 404 → `terminal_error` / `NOT_FOUND`
- 422 → `terminal_error` / `VALIDATION_FAILED`
- 429/403-rate-limit → `retryable_error` / `RATE_LIMITED`
- 500/502/503 → `retryable_error` / `API_ERROR`
6. Transport-level retries (TCP reset, 503) handled internally per `ConnectorConfig.TransportRetries`.
7. **Return contract:** Provider/business errors return `(*ConnectorResult{Status: error}, nil)`. Infrastructure failures (context cancelled, JSON serialization) return `(nil, error)`.
8. Goroutine-safe: no request-specific state stored on the struct.
9. `Close()` is idempotent — closes the underlying HTTP client transport if applicable.

#### Error Table

| Condition | Status | Code | Response |
|-----------|--------|------|----------|
| Invalid input parameters | terminal_error | VALIDATION_FAILED | Input validation error details |
| GitHub API rate limit (429, 403+Retry-After) | retryable_error | RATE_LIMITED | Rate limit exceeded, retry after |
| Authentication failure (401, 403) | terminal_error | AUTH_FAILED | Authentication failed |
| Repository not found (404) | terminal_error | NOT_FOUND | Repository does not exist |
| Unprocessable entity (422) | terminal_error | VALIDATION_FAILED | GitHub rejected the request |
| GitHub server error (500/502/503) | retryable_error | API_ERROR | API error from GitHub |

#### Output Schema

| Operation | Field | Type | Description |
|-----------|-------|------|-------------|
| `create_repository` | repo_url | string | HTML URL of the created repository |
| `create_repository` | repo_id | int | GitHub repository ID |
| `create_repository` | clone_url | string | Git clone URL |
| `delete_repository` | deleted | bool | Whether the repository was deleted |
| `create_pull_request` | pr_url | string | HTML URL of the created pull request |
| `create_pull_request` | pr_id | int | GitHub pull request ID |
| `create_pull_request` | pr_number | int | GitHub pull request number |
| `add_repository_collaborator` | user_added | string | Username of the added collaborator |
| `add_repository_collaborator` | permission | string | Permission level granted |
| `create_repository_webhook` | webhook_url | string | URL of the created webhook |
| `create_repository_webhook` | webhook_id | int | GitHub webhook ID |

#### Acceptance Criteria

```gherkin
Feature: GitHubConnector

  Scenario: Create repository
    Given valid GitHub credentials
    When executing create_repository with owner and name
    Then it returns success with repo_url, repo_id, and clone_url

  Scenario: Create pull request
    Given valid GitHub credentials and existing repository
    When executing create_pull_request with required fields
    Then it returns success with pr_url, pr_id, and pr_number

  Scenario: Delete repository
    Given valid GitHub credentials and existing repository
    When executing delete_repository
    Then it returns success with deleted=true

  Scenario: Rate limiting
    Given GitHub returns 429
    When executing any operation
    Then it returns retryable_error with code RATE_LIMITED

  Scenario: Authentication failure
    Given invalid GitHub credentials
    When executing any operation
    Then it returns terminal_error with code AUTH_FAILED

  Scenario: Unknown operation
    When executing operation "unknown_op"
    Then Validate returns error

  Scenario: Concurrent execution
    Given valid credentials
    When 50 goroutines execute operations concurrently
    Then all return without race conditions (verified with -race flag)

  Scenario: Close then execute
    Given a closed GitHubConnector
    When Execute is called
    Then it returns (nil, error) indicating the connector is closed
```