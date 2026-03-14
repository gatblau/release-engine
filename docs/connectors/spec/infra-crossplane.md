## SPEC: CrossplaneConnector

- **File:** `internal/connector/crossplane/crossplane.go`
- **Package:** `crossplane`
- **Build Phase:** 2
- **Dependencies:** BaseConnector, ConnectorConfig, VoltaManager, client-go dynamic client

#### Purpose

Implements Crossplane-specific connector for managing composite resources via the Kubernetes API.

#### Public Interface

```go
type CrossplaneConnector struct {
connector.BaseConnector
client dynamic.Interface // k8s.io/client-go/dynamic
config connector.ConnectorConfig
}

func NewCrossplaneConnector(cfg connector.ConnectorConfig, client dynamic.Interface) (*CrossplaneConnector, error)
func (c *CrossplaneConnector) Validate(operation string, input map[string]interface{}) error
func (c *CrossplaneConnector) Execute(ctx context.Context, operation string, input map[string]interface{}) (*connector.ConnectorResult, error)
func (c *CrossplaneConnector) Close() error

// Implements connector.OperationDescriber
func (c *CrossplaneConnector) Operations() []connector.OperationMeta
```

Supported operations:
- `create_composite_resource` (async — returns immediately after resource creation)
- `get_resource_status` (polling operation)
- `delete_resource`

#### Internal Logic

1. Validates Crossplane composition reference and parameters using `Validate()`.
2. Extracts `call_id` from context via `CallIDFromContext(ctx)` and sets it as an annotation on the resource for idempotency tracking.
3. **Kubernetes client construction** (at connector creation time, not per-request):
- If `CROSSPLANE_KUBECONFIG_SECRET_NAME` is set: fetch kubeconfig from Volta, build rest.Config from it, create dynamic client.
- Else if `CROSSPLANE_TOKEN_SECRET_NAME` is set: fetch token from Volta, build rest.Config with bearer token auth.
- Else: use `rest.InClusterConfig()` (assumes in-cluster deployment).
4. Uses Kubernetes dynamic client to create/get/delete unstructured resources.
5. `create_composite_resource` constructs an `unstructured.Unstructured` object with the specified API group, version, kind, and spec, then calls `client.Resource(gvr).Create(ctx, obj, metav1.CreateOptions{})`. Returns immediately with the resource name and UID.
6. `get_resource_status` fetches the resource and extracts `.status.conditions` to determine readiness.
7. Maps Kubernetes API errors to `ConnectorResult`:
- `StatusReasonNotFound` → `terminal_error` / `NOT_FOUND`
- `StatusReasonAlreadyExists` → `terminal_error` / `CONFLICT`
- `StatusReasonForbidden` → `terminal_error` / `AUTH_FAILED`
- `StatusReasonInvalid` → `terminal_error` / `INVALID_COMPOSITION`
- `StatusReasonServerTimeout` / `StatusReasonServiceUnavailable` → `retryable_error` / `CROSSPLANE_ERROR`
8. Transport-level retries handled internally per `ConnectorConfig.TransportRetries` for transient API server errors.
9. **Return contract:** Provider/business errors return `(*ConnectorResult{Status: error}, nil)`. Infrastructure failures return `(nil, error)`.
10. Goroutine-safe: Kubernetes dynamic client is safe for concurrent use.
11. `Close()` is idempotent — no persistent state to release beyond the HTTP transport, which is shared.

#### Error Table

| Condition | Status | Code | Response |
|-----------|--------|------|----------|
| Invalid composition reference | terminal_error | INVALID_COMPOSITION | Crossplane composition is invalid |
| Resource already exists | terminal_error | CONFLICT | Resource already exists |
| Forbidden / RBAC failure | terminal_error | AUTH_FAILED | Insufficient Kubernetes RBAC permissions |
| Resource not found | terminal_error | NOT_FOUND | Resource does not exist |
| API server timeout / unavailable | retryable_error | CROSSPLANE_ERROR | Kubernetes API server error |
| Invalid resource spec | terminal_error | VALIDATION_FAILED | Resource spec rejected by API server |

#### Output Schema

| Operation | Field | Type | Description |
|-----------|-------|------|-------------|
| `create_composite_resource` | resource_name | string | Name of the created composite resource |
| `create_composite_resource` | resource_uid | string | Kubernetes UID of the resource |
| `create_composite_resource` | async | bool | Always true — poll with get_resource_status |
| `get_resource_status` | resource_name | string | Name of the resource |
| `get_resource_status` | ready | bool | Whether all conditions report ready |
| `get_resource_status` | conditions | []map | Raw status conditions from the resource |
| `delete_resource` | deleted | bool | Whether the delete was accepted |

#### Acceptance Criteria

```gherkin
Feature: CrossplaneConnector

Scenario: Create composite resource
Given valid Kubernetes credentials and composition
When executing create_composite_resource
Then it returns success with resource_name, resource_uid, and async=true

Scenario: Poll resource status
Given a Crossplane resource in provisioning state
When executing get_resource_status
Then it returns success with ready=false and conditions

Scenario: Resource becomes ready
Given a Crossplane resource that has completed provisioning
When executing get_resource_status
Then it returns success with ready=true

Scenario: Delete resource
Given an existing Crossplane resource
When executing delete_resource
Then it returns success with deleted=true

Scenario: Invalid composition
Given an invalid composition reference
When executing create_composite_resource
Then it returns terminal_error with code INVALID_COMPOSITION

Scenario: RBAC failure
Given insufficient Kubernetes RBAC permissions
When executing any operation
Then it returns terminal_error with code AUTH_FAILED

Scenario: Unknown operation
When executing operation "terraform_apply"
Then Validate returns error

Scenario: Concurrent execution
Given valid credentials
When 50 goroutines execute operations concurrently
Then all return without race conditions
```
