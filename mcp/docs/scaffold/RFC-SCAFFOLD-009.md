# RFC-SCAFFOLD-008 — Error Taxonomy and Remediation Model

**Status:** Draft  
**Authors:** Platform Engineering  
**Audience:** Platform team, MCP server implementers, Release Engine maintainers, portal/CLI consumers, security, support, and operations stakeholders  
**Last Updated:** March 15, 2026

---

## 1. Summary

This RFC defines the canonical **error taxonomy**, **retryability model**, and **remediation contract** for scaffold validation, submission, execution, and status retrieval.

It standardizes:

- how scaffold-related errors are classified,
- which errors are safe to expose to end users,
- how retryability is represented,
- how remediation guidance is attached,
- how user-facing and operator-facing diagnostics are separated,
- and how errors map across validation, submit, runtime, and terminal outcome flows.

This RFC completes the scaffold API/status line by ensuring clients can interpret failures consistently across:

- **MCP validation**
- **MCP submit**
- **Release Engine execution**
- **approval waits and expirations**
- **status polling**
- **event-driven consumption**

---

## 2. Related RFCs

This RFC builds on:

- **RFC-SCAFFOLD-001** — Scaffolding intent model and MCP contract
- **RFC-SCAFFOLD-002** — Template catalog and validation rules
- **RFC-SCAFFOLD-003** — Compile-to-Release-Engine mapping
- **RFC-SCAFFOLD-004** — Fixtures and Golden Tests
- **RFC-SCAFFOLD-005** — Approval Context and Release Engine Enforcement
- **RFC-SCAFFOLD-006** — Execution Status and Outcome Model
- **RFC-SCAFFOLD-007** — API Surface and Transport Bindings

---

## 3. Problem Statement

Without a canonical error model, scaffold clients and operators will face inconsistent failure handling:

- portals may show raw backend messages,
- CLIs may retry unsafe failures,
- agents may not know whether a failure is user-fixable,
- support teams may lack correlation points,
- policy denials may be confused with transient outages,
- approval expiration may be treated as an infrastructure failure,
- partial artifact creation may be hidden from consumers.

We need a single model that answers:

- **what kind of failure occurred?**
- **is it retryable?**
- **who should act next?**
- **what is safe to display?**
- **what remediation is recommended?**
- **does the execution need cleanup or manual follow-up?**

---

## 4. Goals

This RFC aims to:

1. define a stable scaffold error code taxonomy,
2. distinguish validation, submission, execution, approval, and access failures,
3. standardize retryability semantics,
4. support user-safe remediation messages,
5. preserve operator diagnostics without overexposure,
6. enable consistent portal, CLI, and agent behavior,
7. support analytics and auditability around failure classes.

---

## 5. Non-Goals

This RFC does **not** define:

- implementation-specific exception hierarchies,
- logging vendor selection,
- exact internal stack trace formats,
- incident response processes,
- template-authoring lint rules beyond their surfaced errors.

This RFC defines the **public and normalized contract**, not every internal implementation detail.

---

## 6. Design Principles

### 6.1 Errors Must Be Actionable

An error without guidance creates repeat failures and support burden.

### 6.2 Public Errors Must Be Safe

Public payloads must not expose secrets, internal hostnames, raw credentials, or sensitive policy internals.

### 6.3 Retryability Must Be Explicit

Clients should not infer retryability from text messages.

### 6.4 User Responsibility Must Be Distinguishable

Errors should make it clear whether the next action belongs to:

- the requester,
- an approver,
- the platform operator,
- or the system itself via retry.

### 6.5 Terminal Failures and API Errors Are Different

A `400` on submit is not the same kind of failure as an execution that enters terminal `failed`.

### 6.6 Partial Progress Must Not Be Hidden

If artifacts were created before failure or cancellation, the error/remediation model must help clients and operators respond appropriately.

---

## 7. Error Model Layers

This RFC defines error handling across four layers:

| Layer | Description | Example |
|---|---|---|
| **Request Error** | Error before execution creation | invalid payload |
| **Submission Error** | Error during compile/submit boundary | idempotency conflict |
| **Execution Issue** | Problem encountered while running | repository creation failed |
| **Terminal Failure Outcome** | Final failure interpretation for execution | scaffold failed after partial artifact creation |

These layers may use related codes, but they are not identical in behavior.

---

## 8. Canonical Error Dimensions

Every normalized error or issue should be interpretable along these dimensions:

- **code** — stable machine-readable identifier
- **category** — broad error family
- **message** — safe human-readable description
- **retryable** — whether retry is recommended or safe
- **actor** — who should act next
- **severity** — informational, warning, error, critical
- **stage** — where the error occurred
- **remediation** — structured guidance
- **details** — bounded structured metadata
- **diagnosticsRef** — operator correlation reference where applicable

---

## 9. Error Categories

The canonical top-level categories are:

1. **VALIDATION**
2. **POLICY**
3. **AUTH**
4. **IDEMPOTENCY**
5. **DEPENDENCY**
6. **EXECUTION**
7. **APPROVAL**
8. **CANCELLATION**
9. **TIMEOUT**
10. **INTERNAL**
11. **NOT_FOUND**
12. **CONFLICT**

These categories are intentionally stable and broad.

---

## 10. Actors for Remediation

The `actor` field indicates who should respond next.

Allowed values:

- `requester`
- `approver`
- `operator`
- `system`
- `unknown`

### 10.1 Examples

- malformed service name → `requester`
- approval pending → `approver`
- Git provider outage → `operator` or `system`
- background retry in progress → `system`
- unexpected internal invariant failure → `operator`

---

## 11. Stages

The `stage` field indicates where the issue occurred.

Recommended values:

- `validation`
- `normalization`
- `compile`
- `submission`
- `queue`
- `approval`
- `execution`
- `finalization`
- `status_read`

This helps clients and operators localize failure origin without exposing implementation internals.

---

## 12. Retryability Model

Retryability must be explicit and structured.

### 12.1 Allowed Values

This RFC recommends a richer retry model than a simple boolean:

- `never`
- `safe_now`
- `safe_later`
- `unsafe_without_change`
- `unknown`

### 12.2 Interpretation

| Value | Meaning |
|---|---|
| `never` | Retrying will not help |
| `safe_now` | Retry can be attempted immediately |
| `safe_later` | Retry may succeed after delay or dependency recovery |
| `unsafe_without_change` | Retry only after request, policy, or state change |
| `unknown` | Retryability cannot be determined reliably |

### 12.3 Backward-Compatible Bool Mapping

Where a simple boolean is needed:

- `safe_now`, `safe_later` → `retryable = true`
- `never`, `unsafe_without_change`, `unknown` → `retryable = false`

---

## 13. Remediation Model

### 13.1 Purpose

Remediation gives consumers actionable next steps instead of forcing them to parse free-text messages.

### 13.2 Canonical Remediation Shape

```json
{
  "actor": "requester",
  "retryability": "unsafe_without_change",
  "summary": "Update the request to use a valid service name",
  "steps": [
    "Use only lowercase letters, digits, and hyphens",
    "Ensure the service name is unique within the target organization",
    "Resubmit the scaffold request after correction"
  ],
  "docsRef": "scaffold-errors/invalid-service-name",
  "supportHint": "If the name should already be reserved for your team, contact platform support"
}
```

### 13.3 Remediation Rules

- `summary` should be concise and action-oriented
- `steps` should be ordered
- `docsRef` should be a stable internal documentation key or route, not a raw implementation URL
- `supportHint` is optional and should be safe for end users

---

## 14. Public Error Shape

### 14.1 Canonical API Error

```json
{
  "error": {
    "code": "SCAFFOLD_VALIDATION_INVALID_SERVICE_NAME",
    "category": "VALIDATION",
    "message": "The service name is not valid",
    "severity": "error",
    "stage": "validation",
    "actor": "requester",
    "retryability": "unsafe_without_change",
    "details": {
      "field": "serviceName",
      "constraint": "dns_label"
    },
    "remediation": {
      "actor": "requester",
      "retryability": "unsafe_without_change",
      "summary": "Choose a valid DNS-compatible service name",
      "steps": [
        "Use lowercase letters, digits, and hyphens only",
        "Start and end with an alphanumeric character",
        "Resubmit after updating the request"
      ]
    },
    "diagnosticsRef": "diag_01HS123ABC456DEF"
  }
}
```

### 14.2 Canonical Runtime Issue Shape

Runtime issues inside execution status should use a sibling shape:

```json
{
  "code": "SCAFFOLD_DEPENDENCY_GIT_PROVIDER_UNAVAILABLE",
  "category": "DEPENDENCY",
  "message": "The repository provider was temporarily unavailable",
  "severity": "error",
  "stage": "execution",
  "actor": "system",
  "retryability": "safe_later",
  "details": {
    "dependency": "git-provider"
  },
  "remediation": {
    "actor": "system",
    "retryability": "safe_later",
    "summary": "Retry after the dependency recovers",
    "steps": [
      "Wait for platform retry or resubmit later if the failure persists"
    ]
  },
  "diagnosticsRef": "diag_01HS123XYZ789LMN"
}
```

---

## 15. Separation of Public and Internal Diagnostics

### 15.1 Public Contract

Public responses may include:

- safe messages,
- stable codes,
- bounded structured details,
- remediation guidance,
- opaque correlation IDs.

### 15.2 Internal Diagnostics

Internal systems may retain:

- stack traces,
- raw vendor responses,
- internal hostnames,
- token fingerprints,
- deeper policy traces,
- internal release/workflow identifiers.

These must not be exposed by default to untrusted consumers.

### 15.3 Operator Correlation

`diagnosticsRef` should allow operators to correlate public errors with richer internal diagnostics.

---

## 16. Canonical Error Code Structure

### 16.1 Naming Pattern

Recommended pattern:

`SCAFFOLD_<CATEGORY>_<SPECIFIC_CONDITION>`

Examples:

- `SCAFFOLD_VALIDATION_MISSING_FIELD`
- `SCAFFOLD_POLICY_DENIED`
- `SCAFFOLD_AUTH_FORBIDDEN_TEMPLATE`
- `SCAFFOLD_IDEMPOTENCY_CONFLICT`
- `SCAFFOLD_DEPENDENCY_GIT_PROVIDER_UNAVAILABLE`
- `SCAFFOLD_EXECUTION_TEMPLATE_RENDER_FAILED`
- `SCAFFOLD_APPROVAL_REJECTED`
- `SCAFFOLD_TIMEOUT_APPROVAL_EXPIRED`
- `SCAFFOLD_INTERNAL_UNEXPECTED_STATE`

### 16.2 Stability Rule

Codes are part of the public contract and must remain stable once released.

Messages may improve; meanings must not drift.

---

## 17. Taxonomy by Category

---

### 17.1 Validation Errors

These occur before execution begins, usually during validate or submit.

Common codes:

- `SCAFFOLD_VALIDATION_MISSING_FIELD`
- `SCAFFOLD_VALIDATION_UNKNOWN_TEMPLATE`
- `SCAFFOLD_VALIDATION_UNSUPPORTED_TEMPLATE_VERSION`
- `SCAFFOLD_VALIDATION_INVALID_SERVICE_NAME`
- `SCAFFOLD_VALIDATION_INVALID_OWNER`
- `SCAFFOLD_VALIDATION_INVALID_PARAMETER`
- `SCAFFOLD_VALIDATION_PARAMETER_CONFLICT`
- `SCAFFOLD_VALIDATION_TARGET_ALREADY_EXISTS`

Typical actor:

- `requester`

Typical retryability:

- `unsafe_without_change`

---

### 17.2 Policy Errors

These represent governance or rule enforcement outcomes.

Common codes:

- `SCAFFOLD_POLICY_DENIED`
- `SCAFFOLD_POLICY_TEMPLATE_NOT_ALLOWED`
- `SCAFFOLD_POLICY_TARGET_SCOPE_DENIED`
- `SCAFFOLD_POLICY_VISIBILITY_DENIED`
- `SCAFFOLD_POLICY_ENVIRONMENT_DENIED`

Typical actor:

- `requester` or `operator`

Typical retryability:

- `unsafe_without_change`

Notes:

- if approval is available, the result may not be an error; it may instead produce `approval.required = true`
- hard denials should be clearly distinguished from approval-managed cases

---

### 17.3 Auth Errors

These represent access control failures.

Common codes:

- `SCAFFOLD_AUTH_UNAUTHENTICATED`
- `SCAFFOLD_AUTH_FORBIDDEN`
- `SCAFFOLD_AUTH_FORBIDDEN_TEMPLATE`
- `SCAFFOLD_AUTH_FORBIDDEN_EXECUTION_READ`
- `SCAFFOLD_AUTH_FORBIDDEN_EXECUTION_CANCEL`

Typical actor:

- `requester`

Typical retryability:

- `unsafe_without_change` or `never`

---

### 17.4 Idempotency Errors

These arise during submit handling.

Common codes:

- `SCAFFOLD_IDEMPOTENCY_CONFLICT`
- `SCAFFOLD_IDEMPOTENCY_KEY_REQUIRED`
- `SCAFFOLD_IDEMPOTENCY_WINDOW_EXPIRED`

Typical actor:

- `requester`

Typical retryability:

- `unsafe_without_change`

---

### 17.5 Dependency Errors

These arise when an external or internal dependency is unavailable or returns an invalid result.

Common codes:

- `SCAFFOLD_DEPENDENCY_GIT_PROVIDER_UNAVAILABLE`
- `SCAFFOLD_DEPENDENCY_CATALOG_UNAVAILABLE`
- `SCAFFOLD_DEPENDENCY_SECRET_STORE_UNAVAILABLE`
- `SCAFFOLD_DEPENDENCY_RELEASE_ENGINE_UNAVAILABLE`
- `SCAFFOLD_DEPENDENCY_TIMEOUT`
- `SCAFFOLD_DEPENDENCY_INVALID_RESPONSE`

Typical actor:

- `system` or `operator`

Typical retryability:

- `safe_now` or `safe_later`

---

### 17.6 Execution Errors

These occur after execution begins.

Common codes:

- `SCAFFOLD_EXECUTION_TEMPLATE_RENDER_FAILED`
- `SCAFFOLD_EXECUTION_REPOSITORY_CREATE_FAILED`
- `SCAFFOLD_EXECUTION_BRANCH_INIT_FAILED`
- `SCAFFOLD_EXECUTION_BOOTSTRAP_FAILED`
- `SCAFFOLD_EXECUTION_CATALOG_REGISTRATION_FAILED`
- `SCAFFOLD_EXECUTION_OUTPUT_NORMALIZATION_FAILED`
- `SCAFFOLD_EXECUTION_CLEANUP_FAILED`

Typical actor:

- `operator`, `system`, or `requester` depending on cause

Typical retryability:

- variable; must be explicit per code

---

### 17.7 Approval Errors and Wait Outcomes

These cover approval-specific conditions.

Common codes:

- `SCAFFOLD_APPROVAL_REQUIRED`
- `SCAFFOLD_APPROVAL_REJECTED`
- `SCAFFOLD_APPROVAL_EXPIRED`
- `SCAFFOLD_APPROVAL_CONTEXT_INVALID`

Typical actor:

- `approver`, `requester`, or `operator`

Typical retryability:

- `unsafe_without_change` for rejected/expired unless resubmission or policy change occurs

Important distinction:

- `SCAFFOLD_APPROVAL_REQUIRED` is often **not an error**
- it may appear as a status issue, warning, or informational state during `awaiting_approval`

---

### 17.8 Cancellation Errors

Common codes:

- `SCAFFOLD_CANCELLATION_NOT_ALLOWED`
- `SCAFFOLD_CANCELLATION_ALREADY_TERMINAL`
- `SCAFFOLD_CANCELLATION_FAILED`

Typical actor:

- `requester` or `operator`

Typical retryability:

- depends on execution state

---

### 17.9 Timeout Errors

Timeouts should be distinguished from generic execution errors.

Common codes:

- `SCAFFOLD_TIMEOUT_QUEUE_EXCEEDED`
- `SCAFFOLD_TIMEOUT_APPROVAL_EXPIRED`
- `SCAFFOLD_TIMEOUT_EXECUTION_EXCEEDED`
- `SCAFFOLD_TIMEOUT_DEPENDENCY_RESPONSE`

Typical actor:

- `system`, `approver`, or `operator`

Typical retryability:

- variable

---

### 17.10 Internal Errors

These reflect bugs, invariant violations, or unexpected conditions.

Common codes:

- `SCAFFOLD_INTERNAL_UNEXPECTED_STATE`
- `SCAFFOLD_INTERNAL_MAPPING_FAILURE`
- `SCAFFOLD_INTERNAL_RESULT_CORRUPTION`

Typical actor:

- `operator`

Typical retryability:

- `unknown` or `safe_later`

---

### 17.11 Not Found and Conflict Errors

Common codes:

- `SCAFFOLD_NOT_FOUND_EXECUTION`
- `SCAFFOLD_NOT_FOUND_TEMPLATE`
- `SCAFFOLD_CONFLICT_TARGET_EXISTS`
- `SCAFFOLD_CONFLICT_NON_CANCELLABLE_STATE`

These should be used carefully to avoid information leakage across auth boundaries.

---

## 18. Validation Response Behavior

Validation may return:

- `valid = true` with no blocking issues,
- `valid = true` with warnings,
- `valid = true` with approval-required context,
- `valid = false` with one or more errors,
- a transport/API failure if validation infrastructure itself failed.

### 18.1 Example Invalid Validation Result

```json
{
  "valid": false,
  "contractVersion": "scaffold-validation-result/v1",
  "issues": [
    {
      "code": "SCAFFOLD_VALIDATION_INVALID_SERVICE_NAME",
      "category": "VALIDATION",
      "message": "The service name is not valid",
      "severity": "error",
      "stage": "validation",
      "actor": "requester",
      "retryability": "unsafe_without_change",
      "details": {
        "field": "serviceName"
      },
      "remediation": {
        "actor": "requester",
        "retryability": "unsafe_without_change",
        "summary": "Use a DNS-compatible service name",
        "steps": [
          "Use lowercase letters, digits, and hyphens only",
          "Resubmit after correcting the name"
        ]
      }
    }
  ]
}
```

---

## 19. Submit Error Behavior

Submit may fail before execution creation due to:

- malformed request,
- denied policy,
- idempotency conflict,
- authentication/authorization failure,
- dependency outage at submit boundary.

### 19.1 Example Submit Failure

```json
{
  "error": {
    "code": "SCAFFOLD_IDEMPOTENCY_CONFLICT",
    "category": "IDEMPOTENCY",
    "message": "The idempotency key was already used with a different scaffold request",
    "severity": "error",
    "stage": "submission",
    "actor": "requester",
    "retryability": "unsafe_without_change",
    "remediation": {
      "actor": "requester",
      "retryability": "unsafe_without_change",
      "summary": "Use a new idempotency key for a materially different request",
      "steps": [
        "Generate a new idempotency key",
        "Resubmit the intended scaffold request"
      ]
    },
    "diagnosticsRef": "diag_01HSIDEMP0001"
  }
}
```

---

## 20. Runtime Failure Behavior

Once an execution exists, failures should generally be represented in execution status rather than only as transport errors.

### 20.1 Principle

A scaffold execution that reaches runtime and fails should be observed as:

- `state = failed` or another terminal state,
- `outcome = failure` or related terminal outcome,
- with structured `issues`.

### 20.2 Example Terminal Failure Status

```json
{
  "executionId": "scafexec_01HRQ3S2YJ9J8Q4R6P0T1M2N3A",
  "state": "failed",
  "outcome": "failure",
  "completedAt": "2026-03-15T10:23:51Z",
  "issues": [
    {
      "code": "SCAFFOLD_EXECUTION_REPOSITORY_CREATE_FAILED",
      "category": "EXECUTION",
      "message": "The repository could not be created",
      "severity": "error",
      "stage": "execution",
      "actor": "operator",
      "retryability": "safe_later",
      "details": {
        "step": "repository_create"
      },
      "remediation": {
        "actor": "operator",
        "retryability": "safe_later",
        "summary": "Investigate repository provider failure and retry",
        "steps": [
          "Check repository provider availability",
          "Verify permissions and organization quota",
          "Retry the scaffold request after remediation"
        ]
      },
      "diagnosticsRef": "diag_01HSREPOFAIL001"
    }
  ],
  "outputs": null
}
```

---

## 21. Warnings vs Errors vs Informational Issues

Not every issue is fatal.

### 21.1 Severity Levels

- `info`
- `warning`
- `error`
- `critical`

### 21.2 Guidance

Use `warning` for cases such as:

- default values applied,
- optional registration skipped,
- non-blocking metadata normalization.

Use `info` for cases such as:

- approval pending,
- fallback behavior applied.

Use `error` for blocking failures.

Use `critical` sparingly for severe internal or data integrity conditions.

---

## 22. Approval State Representation

Approval-related conditions should not always be surfaced as failures.

### 22.1 Approval Pending

If the execution is waiting on approval:

- `state = awaiting_approval`
- `outcome = null`
- issues may include an informational entry such as `SCAFFOLD_APPROVAL_REQUIRED`

### 22.2 Approval Rejected

If rejected:

- terminal state may become `rejected`
- outcome may become `rejected`
- issue code should be `SCAFFOLD_APPROVAL_REJECTED`

### 22.3 Approval Expired

If approval times out:

- terminal state may become `expired`
- outcome may become `expired`
- issue code should be `SCAFFOLD_TIMEOUT_APPROVAL_EXPIRED` or `SCAFFOLD_APPROVAL_EXPIRED`

Recommendation:
- use `SCAFFOLD_TIMEOUT_APPROVAL_EXPIRED` when TTL-driven expiry is the primary interpretation

---

## 23. Partial Success and Cleanup Guidance

If a scaffold execution creates some artifacts before failing or being cancelled, remediation must reflect that reality.

### 23.1 Required Signaling

When partial artifacts exist:

- final outputs should include what was created,
- issues should include cleanup or follow-up guidance,
- remediation actor should be explicit.

### 23.2 Example

```json
{
  "code": "SCAFFOLD_EXECUTION_BOOTSTRAP_FAILED",
  "category": "EXECUTION",
  "message": "Repository creation succeeded, but CI bootstrap failed",
  "severity": "error",
  "stage": "execution",
  "actor": "operator",
  "retryability": "safe_later",
  "remediation": {
    "actor": "operator",
    "retryability": "safe_later",
    "summary": "Repair CI bootstrap or clean up the partially created repository",
    "steps": [
      "Check whether the repository should be retained",
      "Re-run bootstrap if supported",
      "Delete the repository if the scaffold must be abandoned"
    ]
  }
}
```

---

## 24. Mapping Errors to HTTP Status

This RFC does not make HTTP status the primary semantic carrier, but recommends consistency.

| Category / Condition | Recommended HTTP Status |
|---|---:|
| validation failure | `400` |
| unauthenticated | `401` |
| forbidden | `403` |
| not found | `404` |
| idempotency conflict | `409` |
| non-cancellable conflict | `409` |
| policy denial | `422` |
| dependency unavailable | `503` |
| internal unexpected failure | `500` |

Important:
- transport status should not replace structured error codes
- terminal execution failure is usually still retrieved via `200` on `Get Execution`, with failure details in the payload

---

## 25. Mapping Errors to Client Behavior

### 25.1 Portal

Portal should:

- display safe summaries,
- highlight who must act next,
- provide remediation steps,
- suppress raw backend details,
- show support/correlation references when needed.

### 25.2 CLI

CLI should:

- print stable code and short message,
- indicate retryability,
- optionally print structured remediation steps,
- support machine-readable output formats.

### 25.3 Agents

Agents should:

- branch on `code`, `category`, `actor`, and `retryability`,
- avoid interpreting free text alone,
- not auto-retry `unsafe_without_change` errors,
- escalate to human or operator when required.

---

## 26. Error Handling for Events

Events may include summaries of failure, but should not be the sole source of error truth.

### 26.1 Event Payload Guidance

Failure events may include:

- `executionId`
- terminal `state`
- `outcome`
- primary error code
- short summary
- `diagnosticsRef` if appropriate

### 26.2 Reconciliation Rule

Clients should reconcile detailed error information by retrieving the execution resource after receiving a failure-related event.

---

## 27. Safe Exposure Rules

The following must **not** be exposed in public error messages unless explicitly authorized and sanitized:

- secrets or tokens
- raw git/provider credentials
- internal endpoint URLs
- stack traces
- policy engine implementation details that aid bypass
- tenant-internal identifiers outside authorized scope
- approval comments marked private

---

## 28. Error Prioritization Rules

When multiple issues exist, clients need a primary interpretation.

### 28.1 Primary Issue Rule

Responses should identify or imply a primary issue based on:

1. blocking severity
2. first causal failure
3. user actionability
4. terminal outcome relevance

### 28.2 Multi-Issue Rule

It is valid to return multiple issues, but clients should be able to identify the principal failure without guessing.

A future contract may add `primary = true`; for v1 this may be inferred from ordering if documented.

---

## 29. Internationalization and Human Messaging

Stable codes are language-independent. Human-readable `message` and remediation text may later be localized.

Recommendation:

- clients should use `code` for automation,
- text is primarily for human assistance,
- text changes must not alter semantics.

---

## 30. Observability and Analytics

The normalized taxonomy should support metrics such as:

- validation error rate by code,
- policy denial rate by template,
- dependency failure rate by provider,
- approval rejection rate,
- approval expiration rate,
- partial-success/failure cleanup incidence,
- internal error frequency.

This taxonomy should also support support workflows and incident triage.

---

## 31. Contract Examples

---

### 31.1 Unknown Template

```json
{
  "error": {
    "code": "SCAFFOLD_VALIDATION_UNKNOWN_TEMPLATE",
    "category": "VALIDATION",
    "message": "The requested template does not exist",
    "severity": "error",
    "stage": "validation",
    "actor": "requester",
    "retryability": "unsafe_without_change",
    "details": {
      "field": "templateId",
      "value": "service-rust"
    },
    "remediation": {
      "actor": "requester",
      "retryability": "unsafe_without_change",
      "summary": "Choose a valid template identifier",
      "steps": [
        "List available templates",
        "Select a supported template",
        "Resubmit the request"
      ]
    }
  }
}
```

---

### 31.2 Policy Denial

```json
{
  "error": {
    "code": "SCAFFOLD_POLICY_DENIED",
    "category": "POLICY",
    "message": "This scaffold request is not permitted by policy",
    "severity": "error",
    "stage": "validation",
    "actor": "requester",
    "retryability": "unsafe_without_change",
    "details": {
      "policyDecision": "deny"
    },
    "remediation": {
      "actor": "requester",
      "retryability": "unsafe_without_change",
      "summary": "Modify the request to comply with policy or use an approved target",
      "steps": [
        "Review the requested visibility, environment, and ownership values",
        "Submit a policy-compliant request or contact platform governance if an exception is needed"
      ]
    }
  }
}
```

---

### 31.3 Approval Rejected

```json
{
  "executionId": "scafexec_01HRQ3S2YJ9J8Q4R6P0T1M2N3A",
  "state": "rejected",
  "outcome": "rejected",
  "issues": [
    {
      "code": "SCAFFOLD_APPROVAL_REJECTED",
      "category": "APPROVAL",
      "message": "The scaffold request was rejected during approval",
      "severity": "error",
      "stage": "approval",
      "actor": "requester",
      "retryability": "unsafe_without_change",
      "remediation": {
        "actor": "requester",
        "retryability": "unsafe_without_change",
        "summary": "Review the rejection reason and resubmit only after addressing it",
        "steps": [
          "Check the approval feedback if available",
          "Update the request or seek clarification from the approver",
          "Resubmit if appropriate"
        ]
      }
    }
  ]
}
```

---

### 31.4 Dependency Unavailable at Submit Time

```json
{
  "error": {
    "code": "SCAFFOLD_DEPENDENCY_RELEASE_ENGINE_UNAVAILABLE",
    "category": "DEPENDENCY",
    "message": "The execution backend is temporarily unavailable",
    "severity": "error",
    "stage": "submission",
    "actor": "system",
    "retryability": "safe_later",
    "remediation": {
      "actor": "system",
      "retryability": "safe_later",
      "summary": "Retry the submission after the backend recovers",
      "steps": [
        "Wait briefly and retry with the same idempotency key if submission status is uncertain"
      ]
    },
    "diagnosticsRef": "diag_01HSRELBACKEND01"
  }
}
```

---

### 31.5 Status Read Forbidden

```json
{
  "error": {
    "code": "SCAFFOLD_AUTH_FORBIDDEN_EXECUTION_READ",
    "category": "AUTH",
    "message": "You are not authorized to view this scaffold execution",
    "severity": "error",
    "stage": "status_read",
    "actor": "requester",
    "retryability": "never",
    "remediation": {
      "actor": "requester",
      "retryability": "never",
      "summary": "Request access from the appropriate administrator if needed",
      "steps": [
        "Verify that you are operating in the correct organization or tenant context",
        "Contact an administrator if access is expected"
      ]
    }
  }
}
```

---

## 32. Default Client Behaviors

### 32.1 For `unsafe_without_change`

Clients should not auto-retry.

### 32.2 For `safe_later`

Clients may retry with exponential backoff or advise user to retry later.

### 32.3 For `never`

Clients should stop retrying and present remediation.

### 32.4 For `unknown`

Clients should avoid blind retries and surface operator escalation guidance.

---

## 33. Testing Strategy

Tests should cover:

1. stable code emission for all core validation failures
2. policy denial vs approval-required distinction
3. idempotency conflict behavior
4. dependency outage mapping at submit time
5. runtime failure issue normalization
6. approval rejected and expired mappings
7. cancellation conflict behavior
8. partial artifact failure remediation
9. forbidden status read behavior
10. safe message redaction
11. diagnostics reference propagation
12. retryability consistency across transport bindings

---

## 34. Migration and Compatibility

The public contract version introduced by this RFC is:

- `scaffold-error-model/v1`

Compatibility rules:

- adding optional detail fields is backward-compatible
- adding new error codes is backward-compatible
- changing meaning of existing codes is breaking
- changing retryability semantics for existing codes is breaking
- changing actor semantics materially is breaking

---

## 35. Open Questions

1. Should `primaryIssue` become an explicit field in v1, or remain implied by ordering?
2. Should `docsRef` be mandatory for requester-fixable errors?
3. Should remediation include structured `canAutoRetry` or is `retryability` sufficient?
4. Should approval rejection support a separate sanitized `approvalFeedbackSummary` field?
5. Should partial artifact cleanup guidance become a dedicated top-level field in terminal outcomes?

---

## 36. Decision

This RFC proposes a normalized scaffold error and remediation model with:

- stable machine-readable error codes,
- explicit category, actor, stage, and retryability semantics,
- safe public messaging,
- structured remediation guidance,
- operator correlation through diagnostics references,
- and consistent behavior across validation, submit, execution, and terminal outcomes.

This gives portal, CLI, and agent consumers a shared way to answer:

- **what failed**
- **who acts next**
- **whether retry is safe**
- **what to do now**

---