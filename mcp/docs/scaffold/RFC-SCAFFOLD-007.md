# RFC-SCAFFOLD-007 — API Surface and Transport Bindings

---

## 1. Summary

This RFC defines the public **API surface** and **transport bindings** for scaffolding operations.

It standardizes how clients:

- submit scaffold requests,
- validate requests before submission,
- retrieve execution status,
- list executions,
- cancel executions,
- and consume scaffold lifecycle events.

This RFC binds the logical contracts defined in prior scaffold RFCs to concrete API behavior while preserving the core architectural model:

- **clients talk to MCP-facing APIs/tools**
- **MCP validates, compiles, and submits**
- **Release Engine executes and owns runtime truth**
- **MCP exposes normalized read/write contracts to consumers**

---

## 2. Related RFCs

This RFC builds on:

- **RFC-SCAFFOLD-001** — Scaffolding intent model and MCP contract
- **RFC-SCAFFOLD-002** — Template catalog and validation rules
- **RFC-SCAFFOLD-003** — Compile-to-Release-Engine mapping
- **RFC-SCAFFOLD-004** — Fixtures and Golden Tests
- **RFC-SCAFFOLD-005** — Approval Context and Release Engine Enforcement
- **RFC-SCAFFOLD-006** — Execution Status and Outcome Model

---

## 3. Problem Statement

The scaffold RFC line now defines:

- request semantics,
- compile semantics,
- approval boundaries,
- and execution status semantics.

However, without a concrete API surface, different consumers may invent incompatible integrations:

- portal uses one request shape,
- CLI uses a different shape,
- agents use ad hoc tool contracts,
- event subscribers depend on unstable payloads,
- cancellation semantics vary,
- idempotency behavior becomes inconsistent.

We need a transport-level contract that makes scaffold execution accessible in a consistent and implementation-safe way.

---

## 4. Goals

This RFC aims to:

1. define stable submit/read/cancel/list operations,
2. support both synchronous validation and asynchronous execution,
3. preserve idempotency for submit operations,
4. define event delivery bindings at a contract level,
5. support portal, CLI, and agent consumers uniformly,
6. separate API-visible resources from internal Release Engine resources,
7. provide a clean path for MCP tool exposure.

---

## 5. Non-Goals

This RFC does **not** define:

- UI flows,
- concrete gateway product selection,
- queue implementation,
- event bus vendor choice,
- Release Engine internal APIs,
- or per-language SDK design.

This RFC defines the **logical API contract** and its transport behavior.

---

## 6. Design Principles

### 6.1 Asynchronous by Default

Scaffolding is a long-running operation. Submission should create an execution resource rather than block until completion.

### 6.2 One Logical Model, Multiple Bindings

The same logical operations should be representable through:

- HTTP/JSON APIs,
- MCP tools,
- and event streams.

### 6.3 Read-Your-Write Friendly

Submit responses should immediately provide identifiers and enough metadata for follow-up polling.

### 6.4 Stable External IDs

Consumers should interact with stable scaffold execution identifiers, not internal Release Engine IDs.

### 6.5 Idempotent Submission

Repeated submits with the same idempotency key and materially identical payload should not create duplicate executions.

### 6.6 Cancellation Is Best-Effort but Explicit

Cancellation should have a clear contract even if underlying execution cannot halt instantly.

---

## 7. Logical Resource Model

This RFC defines the following public logical resources:

| Resource | Description |
|---|---|
| `ScaffoldRequest` | A submitted scaffold intent payload |
| `ScaffoldExecution` | The long-running execution record created from a request |
| `ScaffoldValidationResult` | Result of validating a request without execution |
| `ScaffoldExecutionEvent` | Emitted lifecycle event for an execution |

The primary durable externally visible resource is **`ScaffoldExecution`**.

---

## 8. Public Operations

The public API surface consists of:

1. **Validate scaffold request**
2. **Submit scaffold request**
3. **Get execution by ID**
4. **List executions**
5. **Cancel execution**
6. **Subscribe or consume events** (transport-dependent)

Optional future operations may include:

- retry,
- resubmit from prior execution,
- dry-run compile inspection,
- approval action forwarding.

Those are out of scope for v1 unless explicitly added later.

---

## 9. Operation 1 — Validate Scaffold Request

### 9.1 Purpose

Allows callers to validate a request before execution.

This is useful for:

- portals building guided forms,
- CLIs offering preflight checks,
- agents testing request validity,
- policy and approval preview.

### 9.2 Behavior

Validation should perform, as applicable:

- schema validation,
- template resolution,
- target normalization,
- policy evaluation,
- approval requirement derivation,
- compile-time feasibility checks that do not cause side effects.

Validation must not create external artifacts.

### 9.3 Canonical Response Shape

```json
{
  "valid": true,
  "contractVersion": "scaffold-validation-result/v1",
  "normalizedRequest": {
    "templateId": "service-node",
    "templateVersion": "3.4.1",
    "serviceName": "billing-api",
    "owner": "team-payments",
    "environment": "prod"
  },
  "policy": {
    "outcome": "allow_with_approval",
    "approvalRequired": true,
    "reasonCodes": [
      "production_target",
      "public_visibility"
    ]
  },
  "issues": [],
  "warnings": [
    {
      "code": "default_branch_not_specified",
      "message": "Default branch will be set to main"
    }
  ]
}
```

### 9.4 Validation Result Semantics

Validation may return:

- valid and executable,
- valid but approval-required,
- invalid due to user-correctable issues,
- denied due to policy,
- indeterminate due to transient backend dependencies.

Validation does not guarantee ultimate execution success.

---

## 10. Operation 2 — Submit Scaffold Request

### 10.1 Purpose

Creates a new scaffold execution from an intent payload.

### 10.2 Submission Semantics

Submission should:

1. validate the request,
2. normalize it,
3. compile it,
4. submit the compiled plan to Release Engine,
5. create and return an execution resource reference.

### 10.3 Request Shape

```json
{
  "idempotencyKey": "c7eb4f40-ef9d-4c2f-97d2-d8290fb3d16d",
  "request": {
    "templateId": "service-node",
    "templateVersion": "3.4.1",
    "serviceName": "billing-api",
    "owner": "team-payments",
    "visibility": "public",
    "environment": "prod",
    "parameters": {
      "language": "nodejs",
      "ci": true,
      "catalogRegistration": true
    }
  },
  "requestMetadata": {
    "requestedBy": "user_12345",
    "source": "portal"
  }
}
```

### 10.4 Submit Response Shape

```json
{
  "executionId": "scafexec_01HRQ3S2YJ9J8Q4R6P0T1M2N3A",
  "requestId": "req_01HRQ3QHAX2J3E4M5N6P7R8S9T",
  "state": "submitted",
  "outcome": null,
  "contractVersion": "scaffold-submit-response/v1",
  "statusRef": {
    "executionId": "scafexec_01HRQ3S2YJ9J8Q4R6P0T1M2N3A"
  },
  "approval": {
    "required": true,
    "state": "pending",
    "reasonCodes": [
      "production_target",
      "public_visibility"
    ]
  },
  "submittedAt": "2026-03-15T10:14:06Z"
}
```

### 10.5 Submit Response Guarantees

The submit response must include:

- stable `executionId`,
- `requestId`,
- initial normalized state,
- enough information to poll immediately.

It may also include initial approval and policy metadata.

---

## 11. Submission Idempotency

### 11.1 Requirement

Clients should supply an `idempotencyKey` for all submit operations.

### 11.2 Idempotency Rule

If the same caller submits:

- the same `idempotencyKey`,
- within the active idempotency window,
- with a materially identical request payload,

then the API must return the original execution reference rather than create a new one.

### 11.3 Conflict Rule

If the same `idempotencyKey` is reused with a materially different request, the API must reject the request as an idempotency conflict.

### 11.4 Recommended Material Equality Basis

Material identity should include at least:

- template identity,
- resolved version,
- normalized target,
- significant parameters,
- caller tenant/org scope.

Metadata like tracing headers should not affect material identity.

---

## 12. Operation 3 — Get Execution by ID

### 12.1 Purpose

Fetches the current normalized execution status.

### 12.2 Response Contract

The response must use the status model defined in **RFC-SCAFFOLD-006**.

### 12.3 Example

```json
{
  "executionId": "scafexec_01HRQ3S2YJ9J8Q4R6P0T1M2N3A",
  "requestId": "req_01HRQ3QHAX2J3E4M5N6P7R8S9T",
  "domain": "scaffolding",
  "contractVersion": "scaffold-execution-status/v1",
  "state": "running",
  "outcome": null,
  "submittedAt": "2026-03-15T10:14:06Z",
  "startedAt": "2026-03-15T10:14:17Z",
  "updatedAt": "2026-03-15T10:15:02Z",
  "completedAt": null,
  "progress": {
    "phase": "repository",
    "message": "Creating repository and default branch",
    "percent": 31
  },
  "outputs": null,
  "issues": []
}
```

### 12.4 Read Semantics

This operation must be:

- safe,
- idempotent,
- durable for terminal executions,
- authorization-scoped.

---

## 13. Operation 4 — List Executions

### 13.1 Purpose

Allows clients to view historical or active scaffold executions within an authorized scope.

### 13.2 Typical Use Cases

- portal execution history,
- CLI `list` commands,
- audit and operational review,
- automation reconciliation.

### 13.3 Filter Dimensions

Recommended filters include:

- `state`
- `outcome`
- `templateId`
- `owner`
- `requestedBy`
- `createdAfter`
- `createdBefore`
- `updatedAfter`
- `updatedBefore`

### 13.4 Example Response Shape

```json
{
  "items": [
    {
      "executionId": "scafexec_01",
      "requestId": "req_01",
      "state": "succeeded",
      "outcome": "success",
      "template": {
        "templateId": "service-node",
        "templateVersion": "3.4.1"
      },
      "target": {
        "serviceName": "billing-api"
      },
      "submittedAt": "2026-03-14T14:11:20Z",
      "completedAt": "2026-03-14T14:15:45Z"
    },
    {
      "executionId": "scafexec_02",
      "requestId": "req_02",
      "state": "awaiting_approval",
      "outcome": null,
      "template": {
        "templateId": "service-java",
        "templateVersion": "2.8.0"
      },
      "target": {
        "serviceName": "identity-api"
      },
      "submittedAt": "2026-03-15T08:02:03Z",
      "completedAt": null
    }
  ],
  "pageInfo": {
    "nextCursor": "opaque_cursor_123",
    "hasNextPage": true
  }
}
```

### 13.5 Pagination

List operations should use cursor-based pagination rather than offset pagination for stability.

---

## 14. Operation 5 — Cancel Execution

### 14.1 Purpose

Requests cancellation of an in-flight scaffold execution.

### 14.2 Cancellation Contract

Cancellation is a **request**, not a guarantee of immediate stop.

The operation should:

- accept cancellation of non-terminal executions,
- reject cancellation of terminal executions,
- return the latest known execution state.

### 14.3 Example Request

```json
{
  "reason": "User requested cancellation from portal"
}
```

### 14.4 Example Response

```json
{
  "executionId": "scafexec_01HRQ3S2YJ9J8Q4R6P0T1M2N3A",
  "state": "cancelling",
  "outcome": null,
  "updatedAt": "2026-03-15T10:19:17Z",
  "issues": []
}
```

### 14.5 Cancellation Semantics

If cancellation occurs after partial artifact creation:

- the final execution may become `cancelled`,
- outputs may still include created artifacts,
- issues may explain required cleanup or remediation.

---

## 15. Event Consumption

### 15.1 Purpose

Provides push-based visibility into scaffold execution changes.

### 15.2 Event Model

The canonical event types are defined in **RFC-SCAFFOLD-006** and include:

- `scaffold.execution.accepted`
- `scaffold.execution.submitted`
- `scaffold.execution.queued`
- `scaffold.execution.running`
- `scaffold.execution.awaiting_approval`
- `scaffold.execution.approved`
- `scaffold.execution.rejected`
- `scaffold.execution.succeeded`
- `scaffold.execution.failed`
- `scaffold.execution.cancelled`
- `scaffold.execution.expired`

### 15.3 Canonical Event Shape

```json
{
  "eventId": "evt_01HRQ5NQW84T2M2W7CX7V4Q8KJ",
  "eventType": "scaffold.execution.running",
  "occurredAt": "2026-03-15T10:15:02Z",
  "contractVersion": "scaffold-execution-event/v1",
  "executionId": "scafexec_01HRQ3S2YJ9J8Q4R6P0T1M2N3A",
  "requestId": "req_01HRQ3QHAX2J3E4M5N6P7R8S9T",
  "state": "running",
  "outcome": null,
  "summary": {
    "phase": "repository",
    "message": "Creating repository and default branch"
  }
}
```

### 15.4 Reconciliation Rule

Events are advisory for real-time responsiveness. The authoritative current state should still be retrieved through `Get Execution by ID`.

---

## 16. HTTP/JSON Binding

This section defines a recommended HTTP-style binding. Exact URI layout may vary by platform, but semantics should remain equivalent.

---

## 17. Recommended Endpoints

### 17.1 Validate

`POST /scaffolds:validate`

### 17.2 Submit

`POST /scaffolds:submit`

### 17.3 Get Execution

`GET /scaffold-executions/{executionId}`

### 17.4 List Executions

`GET /scaffold-executions`

### 17.5 Cancel Execution

`POST /scaffold-executions/{executionId}:cancel`

These action-style endpoints are recommended to make asynchronous command semantics explicit.

---

## 18. HTTP Status Guidance

Recommended HTTP status behavior:

| Operation | Condition | Recommended Status |
|---|---|---:|
| Validate | valid or invalid request | `200` |
| Submit | execution created or idempotently reused | `202` |
| Submit | malformed request | `400` |
| Submit | unauthorized | `401` / `403` |
| Submit | idempotency conflict | `409` |
| Submit | policy denied before execution creation | `422` or domain-specific refusal code |
| Get | found | `200` |
| Get | unknown execution | `404` |
| List | success | `200` |
| Cancel | cancellation accepted | `202` |
| Cancel | already terminal | `409` |
| Cancel | unknown execution | `404` |

### 18.1 Why `202` for Submit

`202 Accepted` reflects the asynchronous nature of scaffolding and aligns with the execution resource model.

---

## 19. Error Response Contract

All API operations should return structured error payloads.

### 19.1 Canonical Error Shape

```json
{
  "error": {
    "code": "IDEMPOTENCY_CONFLICT",
    "message": "The idempotency key was already used with a different request payload",
    "retryable": false,
    "details": {
      "idempotencyKey": "c7eb4f40-ef9d-4c2f-97d2-d8290fb3d16d"
    }
  }
}
```

### 19.2 Error Taxonomy

A fuller code taxonomy should be defined in a later RFC, but errors should already distinguish:

- invalid request,
- unknown template,
- policy denied,
- approval-required preview,
- idempotency conflict,
- unauthorized access,
- execution not found,
- execution not cancellable,
- backend unavailable.

---

## 20. MCP Tool Binding

In addition to HTTP, the same operations should be available via MCP tools or equivalent agent-facing RPC surfaces.

### 20.1 Recommended Tool Set

- `scaffold.validate`
- `scaffold.submit`
- `scaffold.get_status`
- `scaffold.list_executions`
- `scaffold.cancel`

### 20.2 Tool Contract Principle

Tool input/output should mirror the logical resource schema as closely as possible to avoid transport divergence.

---

## 21. MCP Tool Examples

### 21.1 `scaffold.validate`

**Input**
```json
{
  "request": {
    "templateId": "service-node",
    "serviceName": "billing-api",
    "owner": "team-payments",
    "environment": "prod"
  }
}
```

**Output**
```json
{
  "valid": true,
  "policy": {
    "outcome": "allow_with_approval",
    "approvalRequired": true
  },
  "issues": []
}
```

### 21.2 `scaffold.submit`

**Input**
```json
{
  "idempotencyKey": "c7eb4f40-ef9d-4c2f-97d2-d8290fb3d16d",
  "request": {
    "templateId": "service-node",
    "serviceName": "billing-api",
    "owner": "team-payments",
    "environment": "prod"
  }
}
```

**Output**
```json
{
  "executionId": "scafexec_01HRQ3S2YJ9J8Q4R6P0T1M2N3A",
  "state": "submitted"
}
```

---

## 22. Authentication and Authorization

### 22.1 Authentication

All operations must require authenticated callers unless explicitly exposed through a trusted internal channel.

### 22.2 Authorization

Authorization must be enforced for:

- submit rights by template and target scope,
- read access to execution records,
- cancel rights,
- event subscription scope.

### 22.3 Multi-Tenant Rule

Execution records must be tenant- and org-scoped. Callers must not be able to enumerate or infer executions outside their authorized boundary.

---

## 23. Correlation and Traceability

All operations should support correlation metadata.

### 23.1 Recommended Metadata

- `requestId`
- caller principal or service identity
- source channel (`portal`, `cli`, `agent`)
- trace or span correlation IDs
- idempotency key on submit

### 23.2 Audit Rule

Submit, cancel, and possibly validation requests should be auditable with sufficient actor and context metadata.

---

## 24. Read Consistency and Caching

### 24.1 Get by ID

`Get Execution by ID` should prioritize freshness over aggressive caching.

### 24.2 Terminal Results

Terminal execution records may be cached more aggressively as long as returned content remains consistent.

### 24.3 List Results

List operations may be eventually consistent, but individual `Get` on a known execution should converge quickly to the latest known state.

---

## 25. API Versioning

### 25.1 Versioning Approach

This RFC recommends explicit contract versioning in payloads, with transport versioning optionally added at the route or header level.

### 25.2 Versioned Contracts

Initial contract versions:

- `scaffold-validation-result/v1`
- `scaffold-submit-response/v1`
- `scaffold-execution-status/v1`
- `scaffold-execution-event/v1`

### 25.3 Compatibility Rules

Backward-compatible changes include:

- adding optional fields,
- adding issue codes,
- adding filter parameters,
- adding event summary fields.

Breaking changes include:

- renaming states,
- changing outcome semantics,
- changing idempotency behavior materially,
- changing required fields incompatibly.

---

## 26. Submission Flow Summary

The canonical submit flow is:

1. client sends scaffold intent,
2. MCP authenticates and authorizes,
3. MCP validates and normalizes,
4. MCP compiles to Release Engine module input,
5. MCP submits execution,
6. MCP returns `executionId`,
7. client polls or subscribes for status,
8. Release Engine updates runtime state,
9. MCP normalizes and serves status until terminal.

---

## 27. Validation vs Submission Policy Behavior

### 27.1 Validation

Validation may return:

- allowed,
- allowed with approval,
- denied,
- invalid,
- temporarily indeterminate.

### 27.2 Submission

Submission may:

- create an execution that later enters `awaiting_approval`,
- fail before execution creation if request is invalid,
- fail before execution creation if policy denies immediate submission.

### 27.3 Recommendation

If policy behavior supports approval-managed workflows, submission should generally still create an execution rather than fail outright, so long as the request is otherwise valid and eligible for approval handling.

---

## 28. Approval-Related API Behavior

### 28.1 Approval Visibility

Submit and get-status responses may include approval metadata where relevant.

### 28.2 Approval Actions

This RFC does **not** define approval action endpoints such as approve/reject.

Those remain either:

- in Release Engine-native APIs,
- or in a future cross-domain approval RFC.

### 28.3 Principle

Scaffold APIs surface approval context; they do not become the approval authority.

---

## 29. List and Search Semantics

### 29.1 List Scope

List operations should be authorization-scoped and default to recent executions.

### 29.2 Sorting

Default sort should be descending by most recently updated or submitted time.

### 29.3 Search Safety

Search/filter parameters must not reveal hidden executions through timing, count leakage, or detailed validation differences across unauthorized scopes.

---

## 30. Event Delivery Bindings

This RFC intentionally keeps event transport abstract, but the following bindings are expected to be supportable:

- webhook delivery,
- message bus/topic subscription,
- server-sent events,
- portal-internal push channels.

### 30.1 Common Rules

Regardless of transport:

- events must carry stable IDs,
- duplicate delivery must be tolerated,
- consumers must support at-least-once delivery,
- ordering must not be assumed unless explicitly documented by that transport.

---

## 31. Retry Guidance for Clients

### 31.1 Safe Retries

Clients may safely retry:

- validation requests,
- get-status requests,
- list requests.

### 31.2 Conditional Retry

Clients may retry submit only when using the same `idempotencyKey`.

### 31.3 Cancellation Retry

Clients may retry cancellation requests if the previous response was ambiguous, but must tolerate terminal conflict responses.

---

## 32. Security and Privacy Considerations

### 32.1 Sensitive Input Handling

Request parameters may contain sensitive operational intent, even if they do not contain secrets. Logging must be controlled accordingly.

### 32.2 No Secret Echoing

Responses must never echo secrets, credentials, tokens, or generated secure material.

### 32.3 Event Minimization

Events should carry enough data for routing and UI updates without leaking unnecessary request internals.

### 32.4 Access Separation

Read access, submit access, and cancel access may differ by role and must be enforced independently.

---

## 33. Observability Considerations

The API surface should support:

- submission rate metrics,
- validation failure rate,
- idempotency reuse rate,
- queue latency,
- approval wait distribution,
- terminal outcome distribution,
- cancellation rate,
- read latency for status endpoints.

Recommended high-cardinality identifiers should be handled carefully to avoid telemetry cost blow-up.

---

## 34. Testing Strategy

Tests should cover:

1. validate success
2. validate denial
3. submit success with new execution
4. submit replay with same idempotency key
5. submit idempotency conflict
6. get active execution
7. get terminal execution
8. list with filters and pagination
9. cancel active execution
10. cancel terminal execution rejection
11. authorization failures across operations
12. event payload conformance
13. transport consistency between HTTP and MCP tool bindings

---

## 35. Open Questions

1. Should validation expose a fully compiled dry-run plan hash, or remain higher level?
2. Should submit optionally support `validateOnly = true`, or should validation remain a separate operation only?
3. Should list responses include lightweight approval metadata by default?
4. Should approval action forwarding ever be exposed through scaffold APIs, or stay separate?
5. Should event subscription registration be standardized in this RFC line, or left transport-specific?

---

## 36. Decision

This RFC proposes a stable scaffold API surface with five core operations:

- validate,
- submit,
- get execution,
- list executions,
- cancel execution,

plus transport-neutral event consumption.

It preserves the platform architecture:

- **MCP exposes the public contract**
- **Release Engine remains the execution authority**
- **clients interact through stable execution resources**
- **approval context is visible, but approval control remains external**

---