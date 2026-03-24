## SPEC: AWSConnector

- **File:** `internal/connector/aws/aws.go`
- **Package:** `aws`
- **Build Phase:** 2
- **Dependencies:** BaseConnector, ConnectorConfig, VoltaManager, AWS SDK v2

#### Purpose

Implements AWS-specific connector for cloud infrastructure operations including S3, IAM, and EKS management.

#### Public Interface

```go
type AWSConnector struct {
    connector.BaseConnector
    cfg    aws.Config // AWS SDK v2 config
    config connector.ConnectorConfig
}

func NewAWSConnector(cfg connector.ConnectorConfig, awsCfg aws.Config) (*AWSConnector, error)
func (c *AWSConnector) Validate(operation string, input map[string]interface{}) error
func (c *AWSConnector) Execute(ctx context.Context, operation string, input map[string]interface{}, secrets map[string][]byte) (*connector.ConnectorResult, error)
func (c *AWSConnector) Close() error

// Implements connector.OperationDescriber
func (c *AWSConnector) Operations() []connector.OperationMeta
```

Supported operations:
- `create_s3_bucket`
- `create_iam_role`
- `create_eks_cluster` (async — returns immediately)
- `get_cluster_status` (polling operation for EKS)

#### Internal Logic

1. Validates input fields per operation using `Validate()`.
2. Extracts `call_id` from context via `CallIDFromContext(ctx)` and uses it as a client request token for idempotent AWS API calls.
3. Uses AWS SDK v2 service clients (`s3.Client`, `iam.Client`, `eks.Client`) constructed from the injected `aws.Config`.
4. `create_eks_cluster` returns immediately with cluster name and ARN. Status is `success` with output field `async=true`. Runner polls via `get_cluster_status`.
5. Maps AWS API errors to `ConnectorResult`:
- `InvalidParameterException` → `terminal_error` / `VALIDATION_FAILED`
- `AccessDeniedException` → `terminal_error` / `AUTH_FAILED`
- `ResourceNotFoundException` → `terminal_error` / `NOT_FOUND`
- `ResourceInUseException` → `terminal_error` / `CONFLICT`
- `ThrottlingException` / `TooManyRequestsException` → `retryable_error` / `THROTTLED`
- `ServiceException` (5xx) → `retryable_error` / `API_ERROR`
6. Transport-level retries handled by AWS SDK v2 retry configuration, configured via `ConnectorConfig.TransportRetries`.
7. **Return contract:** Provider/business errors return `(*ConnectorResult{Status: error}, nil)`. Infrastructure failures return `(nil, error)`.
8. Goroutine-safe: SDK v2 clients are safe for concurrent use.
9. `Close()` is idempotent — no persistent connections to clean up in SDK v2.

#### Error Table

| Condition | Status | Code | Response |
|-----------|--------|------|----------|
| Invalid input parameters | terminal_error | VALIDATION_FAILED | Input validation error details |
| AWS access denied | terminal_error | AUTH_FAILED | IAM permissions insufficient |
| Resource not found | terminal_error | NOT_FOUND | AWS resource does not exist |
| Resource already exists | terminal_error | CONFLICT | Resource already exists |
| AWS throttling | retryable_error | THROTTLED | API rate limit exceeded |
| AWS service error (5xx) | retryable_error | API_ERROR | AWS service error |

#### Output Schema

| Operation | Field | Type | Description |
|-----------|-------|------|-------------|
| `create_s3_bucket` | bucket_name | string | Name of created bucket |
| `create_s3_bucket` | bucket_arn | string | ARN of created bucket |
| `create_iam_role` | role_name | string | Name of created role |
| `create_iam_role` | role_arn | string | ARN of created role |
| `create_eks_cluster` | cluster_name | string | Name of the cluster |
| `create_eks_cluster` | cluster_arn | string | ARN of the cluster |
| `create_eks_cluster` | async | bool | Always true — poll with get_cluster_status |
| `get_cluster_status` | cluster_name | string | Name of the cluster |
| `get_cluster_status` | status | string | CREATING, ACTIVE, DELETING, FAILED |
| `get_cluster_status` | endpoint | string | Cluster API endpoint (populated when ACTIVE) |

#### Acceptance Criteria

```gherkin
Feature: AWSConnector

  Scenario: Create S3 bucket
    Given valid AWS credentials
    When executing create_s3_bucket with bucket name
    Then it returns success with bucket_name and bucket_arn

  Scenario: Create EKS cluster (async)
    Given valid AWS credentials
    When executing create_eks_cluster
    Then it returns immediately with cluster_name, cluster_arn, and async=true

  Scenario: Poll EKS cluster status
    Given an EKS cluster in CREATING state
    When executing get_cluster_status
    Then it returns success with status "CREATING"

  Scenario: AWS throttling
    Given AWS returns ThrottlingException
    When executing any operation
    Then it returns retryable_error with code THROTTLED

  Scenario: Access denied
    Given insufficient IAM permissions
    When executing any operation
    Then it returns terminal_error with code AUTH_FAILED

  Scenario: Unknown operation
    When executing operation "launch_satellite"
    Then Validate returns error

  Scenario: Concurrent execution
    Given valid credentials
    When 50 goroutines execute operations concurrently
    Then all return without race conditions
```