# RFC-SCAFFOLD-006: Execution Status and Outcome Model

---

## 1. Summary

This RFC defines the canonical **execution status**, **state transitions**, and **terminal outcome model** for scaffolding requests submitted through MCP and executed by Release Engine.

It standardizes:

- the lifecycle states exposed for scaffold execution,
- the difference between execution state and business outcome,
- the shape of normalized status responses,
- terminal success and failure payloads,
- emitted outputs for created artifacts,
- polling and event semantics,
- and how approval-related waits are represented externally.

This RFC preserves the architectural split established in prior RFCs:

- **MCP accepts intent and compiles plans**
- **Release Engine executes workflows and owns runtime state**
- **MCP reflects normalized execution status and outcome to callers**

---

## 2. Related RFCs

This RFC builds on:

- **RFC-SCAFFOLD-001** — Scaffolding intent model and MCP contract
- **RFC-SCAFFOLD-002** — Template catalog and validation rules
- **RFC-SCAFFOLD-003** — Compile-to-Release-Engine mapping
- **RFC-SCAFFOLD-004** — Fixtures and Golden Tests
- **RFC-SCAFFOLD-005** — Approval Context and Release Engine Enforcement

---

## 3. Problem Statement

Scaffolding requests are not instantaneous. A single request may involve:

- template resolution,
- repository creation,
- branch initialization,
- CI/bootstrap registration,
- catalog registration,
- secret or variable setup,
- approval wait states,
- retries,
- partial progress,
- and terminal outputs.

Without a canonical status model, consumers will invent inconsistent interpretations of workflow state, leading to:

- ambiguous user messaging,
- poor polling behavior,
- inconsistent UX across CLI and portal,
- weak automation hooks,
- unclear handling of approval waits,
- and fragmented audit/reporting semantics.

We need a stable contract that answers:

- what state is the scaffold job in?
- is it still progressing?
- is it waiting on approval or user action?
- did it succeed, partially succeed, or fail?
- what was created?
- what follow-up actions should the caller take?

---

## 4. Goals

This RFC aims to:

1. define a canonical lifecycle for scaffold execution,
2. separate **runtime status** from **terminal business outcome**,
3. normalize Release Engine state for external consumers,
4. make approval waits first-class in status reporting,
5. define stable success, failure, and partial-result payloads,
6. support polling, eventing, and auditability,
7. provide a consistent cross-domain pattern reusable for INFRA.

---

## 5. Non-Goals

This RFC does **not** define:

- the internal Release Engine step graph,
- template authoring semantics,
- approval policy derivation logic,
- retry algorithms internal to specific tasks,
- UI design for status rendering,
- or low-level transport choices for event delivery.

This RFC only defines the **contractual model** exposed to platform consumers.

---

## 6. Design Principles

### 6.1 Release Engine Owns Runtime Truth

Release Engine is the source of truth for execution status and transitions.

### 6.2 MCP Normalizes, Not Reinterprets

MCP may reshape or normalize Release Engine status into the public contract, but it must not invent contradictory state.

### 6.3 States Must Be Few and Stable

Consumers should rely on a compact status vocabulary that remains stable even as internal implementations change.

### 6.4 Outcome Must Be Explicit

A job reaching a terminal runtime state does not by itself explain business outcome. The contract must separately describe the outcome.

### 6.5 Partial Success Must Be Representable

Scaffolding workflows may create some artifacts before failing later. The contract must expose that reality safely.

### 6.6 Approval Waits Are First-Class

Approval is not an implementation detail; it must appear as a normal externally visible state.

---

## 7. Conceptual Model

Each scaffold execution is represented as a **ScaffoldExecutionRecord** containing:

- immutable identity,
- request metadata,
- current lifecycle state,
- timestamps,
- approval wait context if relevant,
- progress metadata,
- terminal outcome if finished,
- and discovered outputs.

The model separates:

### 7.1 Lifecycle State

Answers: **“What is happening now?”**

Examples:

- queued
- running
- awaiting_approval
- succeeded
- failed

### 7.2 Outcome

Answers: **“What was the result?”**

Examples:

- success
- rejected
- failed
- partially_completed
- cancelled
- expired

A job may have no outcome yet if it is still active.

---

## 8. Canonical Lifecycle States

The public lifecycle states are:

| State | Terminal | Meaning |
|---|---:|---|
| `accepted` | No | Request accepted by MCP or API, before execution has started |
| `submitted` | No | Compiled plan submitted to Release Engine |
| `queued` | No | Waiting for execution capacity or dependency |
| `running` | No | Execution in progress |
| `awaiting_approval` | No | Execution paused pending Release Engine approval |
| `cancelling` | No | Cancellation requested, not yet terminal |
| `succeeded` | Yes | Execution completed successfully |
| `failed` | Yes | Execution completed unsuccessfully |
| `cancelled` | Yes | Execution was cancelled |
| `rejected` | Yes | Execution was rejected by approval or policy gate |
| `expired` | Yes | Execution or approval wait expired |

---

## 9. State Semantics

### 9.1 `accepted`

The request has passed intake-level validation and has been assigned an external execution identifier, but the compiled plan may not yet have been handed to Release Engine.

### 9.2 `submitted`

The compiled plan has been created and successfully handed off to Release Engine.

### 9.3 `queued`

Release Engine has acknowledged the job, but active execution has not started.

### 9.4 `running`

At least one execution step is actively progressing, retrying, or completing.

### 9.5 `awaiting_approval`

Release Engine has paused the job pending one or more approvals. No further protected steps may proceed until approval is resolved.

### 9.6 `cancelling`

A cancellation request has been accepted but final cleanup or interruption has not completed.

### 9.7 Terminal States

- `succeeded`
- `failed`
- `cancelled`
- `rejected`
- `expired`

Terminal means no further lifecycle transitions are allowed except archival metadata updates.

---

## 10. Allowed State Transitions

The following transitions are allowed:

- `accepted -> submitted`
- `submitted -> queued`
- `submitted -> running`
- `queued -> running`
- `running -> awaiting_approval`
- `awaiting_approval -> queued`
- `awaiting_approval -> running`
- `running -> succeeded`
- `running -> failed`
- `running -> cancelling`
- `queued -> cancelling`
- `awaiting_approval -> cancelling`
- `cancelling -> cancelled`
- `awaiting_approval -> rejected`
- `awaiting_approval -> expired`
- `queued -> failed`
- `submitted -> failed`

Transitions not listed above should be treated as invalid unless explicitly versioned later.

---

## 11. Why `awaiting_approval -> queued` Is Allowed

Some Release Engine implementations may requeue a job after approval before resuming execution. Others may resume directly into `running`.

To preserve compatibility, both are allowed.

---

## 12. Outcome Model

The normalized outcome values are:

| Outcome | Terminal | Meaning |
|---|---:|---|
| `success` | Yes | All required scaffold actions completed successfully |
| `partial_success` | Yes | Some externally relevant artifacts were created, but not all intended actions completed |
| `failure` | Yes | Execution failed without usable completion |
| `rejected` | Yes | Approval or policy rejected the request |
| `cancelled` | Yes | Request was cancelled before completion |
| `expired` | Yes | Pending approval or execution window expired |

### 12.1 Mapping: State vs Outcome

| Final State | Outcome |
|---|---|
| `succeeded` | `success` or `partial_success` |
| `failed` | `failure` or `partial_success` |
| `rejected` | `rejected` |
| `cancelled` | `cancelled` |
| `expired` | `expired` |

This means terminal **state** and terminal **outcome** are related but not identical.

Example:

- runtime state: `failed`
- outcome: `partial_success`
- interpretation: execution failed overall, but some artifacts were already created and must be surfaced.

---

## 13. Partial Success Semantics

`partial_success` should be used when:

- at least one externally relevant artifact was successfully created or mutated,
- the overall requested scaffold operation did not fully complete,
- and the caller must understand both the usable outputs and the incompleteness.

Examples:

- repository created, but catalog registration failed,
- service skeleton generated, but CI bootstrap failed,
- repo and default branch created, but secrets setup failed.

`partial_success` must not be used for trivial internal progress that leaves no user-visible result.

---

## 14. Execution Status Contract

### 14.1 Canonical Schema

```json
{
  "executionId": "scafexec_01HRQ3S2YJ9J8Q4R6P0T1M2N3A",
  "requestId": "req_01HRQ3QHAX2J3E4M5N6P7R8S9T",
  "domain": "scaffolding",
  "contractVersion": "scaffold-execution-status/v1",
  "state": "awaiting_approval",
  "outcome": null,
  "submittedAt": "2026-03-15T09:41:12Z",
  "startedAt": "2026-03-15T09:41:25Z",
  "updatedAt": "2026-03-15T09:43:03Z",
  "completedAt": null,
  "planRef": {
    "module": "scaffolding/create-service",
    "moduleContractVersion": "scaffolding-create-service/v1",
    "planHash": "sha256:2c2d7e..."
  },
  "template": {
    "templateId": "service-node",
    "templateVersion": "3.4.1"
  },
  "target": {
    "org": "acme",
    "serviceName": "billing-api",
    "environment": "prod"
  },
  "approval": {
    "required": true,
    "state": "pending",
    "policyRef": "approval-policy/prod-public-service",
    "reasonCodes": [
      "production_target",
      "public_visibility"
    ],
    "pendingSince": "2026-03-15T09:43:03Z",
    "expiresAt": "2026-03-16T09:43:03Z"
  },
  "progress": {
    "phase": "approval",
    "message": "Waiting for Release Engine approval",
    "percent": 42
  },
  "outputs": null,
  "issues": [],
  "links": {
    "releaseEngineRunId": "rel_01HRQ3TTQ3ZZH1DVFZ8S6G4K5Y"
  }
}
```

---

## 15. Field Definitions

### 15.1 Identity Fields

- `executionId`: stable external identifier for the scaffold execution
- `requestId`: caller-visible request correlation id
- `domain`: always `scaffolding`
- `contractVersion`: public status contract version

### 15.2 State Fields

- `state`: current lifecycle state
- `outcome`: terminal business outcome or `null` while active

### 15.3 Time Fields

- `submittedAt`: when the request was handed to execution flow
- `startedAt`: when active execution began
- `updatedAt`: most recent state-affecting update
- `completedAt`: terminal completion time, otherwise `null`

### 15.4 Plan Reference

Identifies the compiled plan and contract used for execution.

### 15.5 Template Metadata

Captures the resolved template identity used for the job.

### 15.6 Target Metadata

Contains normalized high-level target information safe for external consumers.

### 15.7 Approval Object

Populated when approval is relevant. May remain present after terminal resolution for audit visibility.

### 15.8 Progress Object

Optional human-friendly and machine-friendly progress hints.

### 15.9 Outputs

Terminal or late-bound generated resources and identifiers.

### 15.10 Issues

Structured warnings, errors, or remediation notices.

### 15.11 Links

Opaque references to backing systems or follow-up actions.

---

## 16. Approval Status Submodel

When approval applies, the `approval` object should use this shape:

```json
{
  "required": true,
  "state": "pending",
  "policyRef": "approval-policy/prod-public-service",
  "reasonCodes": [
    "production_target",
    "public_visibility"
  ],
  "suggestedApproverRoles": [
    "platform-admin",
    "service-owner-manager"
  ],
  "pendingSince": "2026-03-15T09:43:03Z",
  "expiresAt": "2026-03-16T09:43:03Z",
  "resolvedAt": null,
  "resolution": null
}
```

### 16.1 Approval States

| Approval State | Meaning |
|---|---|
| `not_required` | Approval not needed |
| `pending` | Awaiting decision |
| `approved` | Approved by Release Engine |
| `rejected` | Rejected by Release Engine |
| `expired` | Approval window expired |

### 16.2 Resolution Values

| Resolution | Meaning |
|---|---|
| `approved` | Approval granted |
| `rejected` | Approval denied |
| `expired` | Approval timed out |

### 16.3 Source of Truth

Approval data is informational from the perspective of MCP consumers. Release Engine remains the system of record.

---

## 17. Progress Model

The public contract may expose lightweight progress information:

```json
{
  "phase": "bootstrap",
  "message": "Configuring CI and repository settings",
  "percent": 67
}
```

### 17.1 Recommended Progress Phases

- `validation`
- `submission`
- `queue`
- `bootstrap`
- `repository`
- `catalog`
- `configuration`
- `approval`
- `finalization`

### 17.2 Progress Rules

- `percent` is optional and advisory
- progress percentages need not be exact
- phases should be stable, coarse-grained, and user-friendly
- consumers must not depend on exact step ordering from phase alone

---

## 18. Outputs Model

Outputs represent created artifacts or externally relevant identifiers.

### 18.1 Canonical Outputs Shape

```json
{
  "repository": {
    "provider": "github",
    "owner": "acme",
    "name": "billing-api",
    "defaultBranch": "main",
    "url": "opaque"
  },
  "catalog": {
    "entityRef": "component:default/billing-api",
    "registered": true
  },
  "service": {
    "name": "billing-api",
    "slug": "billing-api",
    "language": "nodejs"
  },
  "ci": {
    "enabled": true,
    "provider": "github-actions"
  },
  "artifacts": [
    {
      "type": "readme",
      "status": "created"
    }
  ]
}
```

### 18.2 Output Design Rules

- outputs should be stable and domain-oriented
- outputs should avoid leaking internal Release Engine details
- outputs may be partially populated in `partial_success`
- absent outputs should be `null` or omitted consistently per field contract
- sensitive values must never be included

---

## 19. Issues Model

`issues` captures structured problems, warnings, or remediation hints.

### 19.1 Canonical Issue Shape

```json
{
  "severity": "warning",
  "code": "catalog_registration_failed",
  "message": "Catalog registration did not complete",
  "retryable": true,
  "scope": "catalog",
  "details": {
    "entityRef": "component:default/billing-api"
  }
}
```

### 19.2 Severities

- `info`
- `warning`
- `error`

### 19.3 Use Cases

Issues may communicate:

- non-fatal warnings in successful executions,
- causes of failure,
- partial-success explanations,
- remediation guidance,
- transient retry suggestions.

---

## 20. Success Contract

A successful terminal response should look like:

```json
{
  "executionId": "scafexec_01",
  "state": "succeeded",
  "outcome": "success",
  "completedAt": "2026-03-15T09:49:11Z",
  "outputs": {
    "repository": {
      "provider": "github",
      "owner": "acme",
      "name": "billing-api",
      "defaultBranch": "main"
    },
    "catalog": {
      "entityRef": "component:default/billing-api",
      "registered": true
    },
    "service": {
      "name": "billing-api",
      "slug": "billing-api"
    }
  },
  "issues": []
}
```

A success response must indicate that all required scaffold actions completed.

---

## 21. Partial Success Contract

A partial success example:

```json
{
  "executionId": "scafexec_02",
  "state": "failed",
  "outcome": "partial_success",
  "completedAt": "2026-03-15T09:52:48Z",
  "outputs": {
    "repository": {
      "provider": "github",
      "owner": "acme",
      "name": "billing-api",
      "defaultBranch": "main"
    },
    "catalog": null
  },
  "issues": [
    {
      "severity": "error",
      "code": "catalog_registration_failed",
      "message": "Catalog registration failed after repository creation",
      "retryable": true,
      "scope": "catalog"
    }
  ]
}
```

This explicitly tells consumers:

- something useful exists,
- the overall workflow did not fully complete,
- and follow-up may be required.

---

## 22. Failure Contract

A failure example:

```json
{
  "executionId": "scafexec_03",
  "state": "failed",
  "outcome": "failure",
  "completedAt": "2026-03-15T09:46:09Z",
  "outputs": null,
  "issues": [
    {
      "severity": "error",
      "code": "repository_creation_failed",
      "message": "Repository provider rejected the create request",
      "retryable": false,
      "scope": "repository"
    }
  ]
}
```

Use `failure` when no meaningful scaffold result is available to the caller.

---

## 23. Rejected Contract

A rejection example:

```json
{
  "executionId": "scafexec_04",
  "state": "rejected",
  "outcome": "rejected",
  "completedAt": "2026-03-15T10:10:00Z",
  "approval": {
    "required": true,
    "state": "rejected",
    "policyRef": "approval-policy/prod-public-service",
    "reasonCodes": [
      "production_target",
      "public_visibility"
    ],
    "resolvedAt": "2026-03-15T10:10:00Z",
    "resolution": "rejected"
  },
  "outputs": null,
  "issues": [
    {
      "severity": "error",
      "code": "approval_rejected",
      "message": "Request was rejected during approval review",
      "retryable": false,
      "scope": "approval"
    }
  ]
}
```

---

## 24. Cancelled Contract

A cancellation example:

```json
{
  "executionId": "scafexec_05",
  "state": "cancelled",
  "outcome": "cancelled",
  "completedAt": "2026-03-15T10:02:32Z",
  "outputs": {
    "repository": {
      "provider": "github",
      "owner": "acme",
      "name": "billing-api"
    }
  },
  "issues": [
    {
      "severity": "warning",
      "code": "execution_cancelled_after_partial_creation",
      "message": "Execution was cancelled after repository creation",
      "retryable": false,
      "scope": "execution"
    }
  ]
}
```

If cancellation occurs after visible artifacts were created, outputs should still be surfaced.

---

## 25. Expired Contract

An expiry example:

```json
{
  "executionId": "scafexec_06",
  "state": "expired",
  "outcome": "expired",
  "completedAt": "2026-03-16T09:43:03Z",
  "approval": {
    "required": true,
    "state": "expired",
    "policyRef": "approval-policy/prod-public-service",
    "reasonCodes": [
      "production_target"
    ],
    "pendingSince": "2026-03-15T09:43:03Z",
    "expiresAt": "2026-03-16T09:43:03Z",
    "resolvedAt": "2026-03-16T09:43:03Z",
    "resolution": "expired"
  },
  "outputs": null,
  "issues": [
    {
      "severity": "error",
      "code": "approval_expired",
      "message": "Approval was not granted before expiry",
      "retryable": true,
      "scope": "approval"
    }
  ]
}
```

---

## 26. Polling Contract

Consumers should be able to retrieve execution status through a stable read API.

### 26.1 Polling Requirements

Status reads must:

- be idempotent,
- return the latest normalized state known to MCP,
- preserve terminal results,
- and support safe repeated reads.

### 26.2 Polling Behavior

Recommended behavior:

- poll more frequently in `accepted`, `submitted`, `queued`
- poll moderately in `running`
- poll slowly in `awaiting_approval`
- stop polling once terminal

### 26.3 Terminal Cacheability

Terminal responses should be durable and stable for later retrieval and audit.

---

## 27. Event Model

In addition to polling, platforms may emit execution events.

### 27.1 Canonical Event Types

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

### 27.2 Event Payload Rule

Each event should include:

- `executionId`
- `state`
- `outcome` if terminal
- `occurredAt`
- minimal correlation metadata
- optional diff or summary fields

### 27.3 Event Ordering

Consumers must not assume perfect delivery or total ordering unless explicitly guaranteed by the transport. Polling remains the source of reconciliation.

---

## 28. Normalization from Release Engine

Release Engine internal statuses may be richer or more granular. MCP should normalize them into the public scaffold model.

### 28.1 Example Mapping

| Release Engine Internal Status | Public State |
|---|---|
| `PENDING_SUBMISSION` | `accepted` |
| `SUBMITTED` | `submitted` |
| `READY` | `queued` |
| `IN_PROGRESS` | `running` |
| `WAITING_APPROVAL` | `awaiting_approval` |
| `CANCEL_REQUESTED` | `cancelling` |
| `COMPLETED` | `succeeded` or `failed` |
| `REJECTED` | `rejected` |
| `CANCELLED` | `cancelled` |
| `EXPIRED` | `expired` |

### 28.2 Rule

MCP must preserve semantic truth while reducing implementation-specific variance.

---

## 29. Idempotency and Read Consistency

Repeated status reads for the same `executionId` must be stable and monotonic in lifecycle progression.

### 29.1 Monotonicity

Consumers must not observe a state regression such as:

- `running -> queued`
- `succeeded -> running`

unless an explicitly versioned exceptional replay mode exists, which is out of scope here.

### 29.2 Output Stability

Once a terminal output is published, it should not be silently removed. Corrections should be additive and auditable.

---

## 30. Deletion and Retention

Execution records should remain retrievable for a defined retention period.

### 30.1 Minimum Expectation

At minimum, retained records should preserve:

- identifiers,
- final state,
- final outcome,
- timestamps,
- outputs,
- issues,
- approval resolution if applicable.

### 30.2 Archival

Implementations may archive older executions, but the external retrieval contract should remain stable.

---

## 31. Error Semantics for Status Reads

Status read failures should be distinguishable from execution failures.

### 31.1 Examples

- `404` or equivalent: unknown `executionId`
- `403` or equivalent: caller lacks access
- transient backend read failure: infrastructure/API error, not execution failure
- malformed record: contract integrity issue requiring operator attention

These errors do not change the scaffold execution state itself.

---

## 32. Security and Privacy Considerations

### 32.1 No Secrets in Outputs

Outputs must never include credentials, access tokens, secret material, or raw secret references that should remain internal.

### 32.2 Safe Error Messages

Issues should be informative without exposing sensitive internals.

### 32.3 Access Control

Execution status visibility must respect tenant, org, and request authorization boundaries.

### 32.4 Approval Privacy

Approval metadata should expose policy-relevant context without leaking private reviewer identity data unless policy explicitly permits it.

---

## 33. Observability Considerations

The model should support platform operations by enabling:

- request-to-run correlation,
- state transition timing,
- approval latency measurement,
- execution duration analysis,
- partial-success rate tracking,
- and failure scope classification.

Recommended derived metrics:

- time to start
- time in queue
- time awaiting approval
- total execution duration
- success rate
- partial success rate
- failure rate by scope
- cancellation rate

---

## 34. Testing Strategy

Fixtures and golden tests should include:

1. simple success
2. queued then running then success
3. approval-required then approved then success
4. approval-required then rejected
5. approval-required then expired
6. repository-created then downstream failure
7. cancellation before start
8. cancellation after partial artifact creation
9. malformed output normalization rejection
10. terminal-state stability across repeated reads

---

## 35. Migration and Versioning

The public contract version for this RFC is:

- `scaffold-execution-status/v1`

Future versions may add fields, but should avoid changing meaning of existing states.

### 35.1 Compatibility Rules

- adding optional fields is backward-compatible
- adding new issue codes is backward-compatible
- changing state names is breaking
- changing outcome semantics is breaking
- changing partial-success criteria materially is breaking

---

## 36. Open Questions

1. Should `accepted` and `submitted` both remain public, or should they collapse into one externally visible pre-execution state?
2. Should progress percentages be omitted entirely if they cannot be estimated reliably?
3. Should `partial_success` be allowed under terminal state `cancelled`, or remain only an interpretation through outputs plus `cancelled` outcome?
4. Should follow-up actions be a first-class field, such as `recommendedActions`?
5. Should portal-facing status include richer human-readable summaries while API remains minimal?

---

## 37. Decision

This RFC proposes that scaffolding executions expose a stable normalized status model with:

- a compact lifecycle state machine,
- a separate explicit outcome model,
- first-class approval wait visibility,
- structured outputs and issues,
- and durable terminal results for polling and event consumers.

This preserves architectural clarity:

- **Release Engine owns execution truth**
- **MCP normalizes and exposes**
- **clients consume a stable contract**

---