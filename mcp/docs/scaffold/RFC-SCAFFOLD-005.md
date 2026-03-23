# RFC-SCAFFOLD-005 — Approval Context and Release Engine Enforcement

**Status:** Draft  
**Authors:** Platform Engineering  
**Audience:** Platform team, Release Engine maintainers, MCP server implementers, security and governance stakeholders  
**Last Updated:** March 15, 2026

---

## 1. Summary

This RFC defines how scaffolding requests express **approval-related policy outcomes** while preserving **Release Engine** as the sole authority for approval enforcement and lifecycle management.

This RFC standardizes:

- how approval requirements are derived before execution,
- how approval context is attached to compiled scaffold plans,
- how Release Engine interprets and enforces approval requirements,
- how approval-related state is exposed externally,
- and what responsibilities are explicitly **out of scope** for MCP.

This RFC builds on:

- **RFC-SCAFFOLD-001** — Scaffolding intent model and MCP contract
- **RFC-SCAFFOLD-002** — Template catalog and validation rules
- **RFC-SCAFFOLD-003** — Compile-to-Release-Engine mapping
- **RFC-SCAFFOLD-004** — Fixtures and Golden Tests

---

## 2. Core Decision

### 2.1 Decision Statement

**Approval authority and approval state MUST remain in Release Engine.**

The scaffolding compiler and MCP layer:

- **MAY** evaluate policy,
- **MAY** determine that approval is required,
- **MAY** attach approval context and risk metadata,
- but **MUST NOT** become the system of record for approval decisions or approval workflow state.

Release Engine:

- **MUST** enforce approval requirements,
- **MUST** own approval lifecycle state,
- **MUST** record approval decisions,
- and **MUST** control execution resume behavior after approval.

---

## 3. Problem Statement

Scaffolding requests may vary significantly in risk and governance impact.

Examples:

- creating a public repository,
- provisioning a production-bound service,
- using a privileged infrastructure template,
- targeting a restricted tenant or org,
- requesting elevated capabilities,
- or generating resources with significant cost implications.

These requests may require:

- human review,
- role-based authorization,
- separation-of-duties controls,
- multi-stage approval,
- or escalation and expiry handling.

If approval were split across MCP and Release Engine, the platform would risk:

- duplicate approval state,
- inconsistent authorization logic,
- fragmented audit trails,
- race conditions between submission and execution,
- weak enforcement,
- and unclear ownership of resume logic.

Therefore, approval must be modeled as:

- **policy signal upstream**, and
- **workflow enforcement downstream**.

---

## 4. Goals

### 4.1 Primary Goals

This RFC aims to:

1. preserve a single source of truth for approval state,
2. allow pre-execution policy evaluation to signal approval need,
3. standardize the approval metadata attached to compiled plans,
4. define the Release Engine handoff contract,
5. support auditability and deterministic execution behavior,
6. support future expansion to multi-step approval models.

### 4.2 Non-Goals

This RFC does **not** define:

- the full internal Release Engine approval implementation,
- UI design for approval prompts,
- enterprise identity provider integration details,
- notification delivery mechanics,
- or organization-specific approval policies.

---

## 5. Architectural Boundary

---

### 5.1 MCP / Compiler Responsibilities

The MCP and compiler layers are responsible for:

- receiving scaffold intent,
- validating request structure and policy inputs,
- evaluating rules that determine whether approval is required,
- deriving approval-related metadata,
- compiling that metadata into the Release Engine submission payload,
- surfacing approval status returned by Release Engine to callers.

They are **not** responsible for:

- authorizing approvers,
- storing authoritative approval state,
- executing approval transitions,
- handling approval expiry,
- or resuming jobs after approval.

---

### 5.2 Release Engine Responsibilities

Release Engine is responsible for:

- evaluating the submitted approval requirement and context,
- entering approval wait states,
- authorizing approvers,
- applying separation-of-duties rules,
- preventing self-approval when configured,
- recording approval and rejection decisions,
- handling escalation or timeout,
- and resuming or terminating execution accordingly.

---

## 6. Approval Model Overview

The scaffold flow should follow this sequence:

1. user submits `ServiceScaffoldRequest`,
2. MCP validates and normalizes the request,
3. policy evaluation determines whether approval is needed,
4. compiler attaches approval context to the compiled plan,
5. plan is submitted to Release Engine,
6. Release Engine decides whether to:
    - start execution immediately,
    - pause pending approval,
    - or reject due to policy or malformed approval metadata,
7. Release Engine owns all subsequent approval state transitions.

---

## 7. Policy Outcome Categories

Policy evaluation before submission may produce one of three broad outcomes.

### 7.1 Allow Without Approval

The request is policy-compliant and does not require manual review.

Example:

```json
{
  "policyOutcome": "allow",
  "approvalRequired": false
}
```

### 7.2 Allow With Approval

The request is policy-compliant only if approved.

Example:

```json
{
  "policyOutcome": "allow_with_approval",
  "approvalRequired": true
}
```

### 7.3 Deny

The request is not permitted and must not be submitted for execution.

Example reasons:

- forbidden tenant,
- prohibited visibility,
- disallowed template,
- policy violation with no approval override path.

Example:

```json
{
  "policyOutcome": "deny",
  "approvalRequired": false,
  "denialReasonCodes": ["public_repo_forbidden"]
}
```

If the outcome is `deny`, the compiler must not create a runnable Release Engine submission.

---

## 8. Approval Context Data Model

The compiler should attach a stable approval context to the compiled plan.

### 8.1 Proposed Shape

```json
{
  "approval": {
    "required": true,
    "policyRef": "scaffold-policy/v3",
    "reasonCodes": [
      "public_repo",
      "prod_target",
      "privileged_template"
    ],
    "riskSummary": {
      "level": "high",
      "blastRadius": "team",
      "estimatedMonthlyCostBand": "medium",
      "sensitiveCapabilities": [
        "internet_ingress",
        "cloud_iam_write"
      ]
    },
    "reviewContext": {
      "service": "payments-api",
      "template": "go-service",
      "environment": "prod",
      "owner": "team-payments",
      "repoVisibility": "public",
      "tenant": "acme-prod"
    },
    "suggestedApproverRoles": [
      "platform-admin",
      "engineering-manager"
    ],
    "ttlSeconds": 172800
  }
}
```

---

### 8.2 Semantics

#### `approval.required`
Whether policy evaluation determined that approval is required prior to execution.

#### `approval.policyRef`
Identifier for the policy bundle, rule set, or policy version that produced the outcome.

#### `approval.reasonCodes`
Stable machine-readable reason identifiers explaining why approval is needed.

#### `approval.riskSummary`
A normalized risk description for approvers and audit systems.

#### `approval.reviewContext`
Human-meaningful execution context to support review.

#### `approval.suggestedApproverRoles`
Advisory roles that may be appropriate reviewers. These are hints, not final authority grants.

#### `approval.ttlSeconds`
Requested review lifetime. Release Engine may honor, cap, or override it according to its own policy.

---

## 9. Normative Rules

### 9.1 Upstream Policy Rules

The compiler layer:

- **MUST** compute `approval.required` deterministically from validated input and policy context.
- **MUST** include machine-readable reason codes when approval is required.
- **MUST NOT** submit a plan marked as both `deny` and executable.
- **SHOULD** include a policy reference for traceability.
- **SHOULD** include a bounded review context useful for human approval.

### 9.2 Downstream Enforcement Rules

Release Engine:

- **MUST** treat approval lifecycle state as authoritative.
- **MUST** enforce approval before execution when `approval.required = true`.
- **MUST** record approval decisions and approver identity.
- **MUST NOT** rely solely on client-supplied suggested approver roles for authorization.
- **MAY** re-evaluate policy or approval requirements if configured.
- **MAY** reject malformed or incomplete approval metadata.

### 9.3 Separation of Responsibilities

MCP:

- **MUST NOT** mark a job as approved.
- **MUST NOT** store canonical approval decision state.
- **MUST NOT** resume paused execution after approval.

Release Engine:

- **MUST** own those actions.

---

## 10. Compilation Contract Changes

This RFC extends the compiled scaffold plan from RFC-SCAFFOLD-003 with approval metadata.

### 10.1 Compiled Plan Fragment

```json
{
  "module": {
    "key": "scaffolding/create-service",
    "version": "v1"
  },
  "contractVersion": "scaffolding-create-service/v1",
  "inputs": {
    "template": "go-service",
    "service_name": "payments-api",
    "owner": "team-payments",
    "org": "acme",
    "visibility": "public",
    "parameters": {
      "language": "go",
      "runtime": "1.24"
    }
  },
  "policy": {
    "outcome": "allow_with_approval"
  },
  "approval": {
    "required": true,
    "policyRef": "scaffold-policy/v3",
    "reasonCodes": [
      "public_repo",
      "prod_target"
    ],
    "riskSummary": {
      "level": "high"
    },
    "reviewContext": {
      "service": "payments-api",
      "environment": "prod"
    },
    "suggestedApproverRoles": [
      "platform-admin"
    ],
    "ttlSeconds": 172800
  }
}
```

---

## 11. Submission Envelope to Release Engine

The submission payload should preserve a clear distinction between:

- module execution inputs,
- policy outcome,
- and approval context.

### 11.1 Example Submission Envelope

```json
{
  "jobType": "module",
  "moduleKey": "scaffolding/create-service",
  "moduleVersion": "v1",
  "params": {
    "template": "go-service",
    "service_name": "payments-api",
    "owner": "team-payments",
    "org": "acme",
    "visibility": "public",
    "parameters": {
      "language": "go",
      "runtime": "1.24"
    }
  },
  "metadata": {
    "contractVersion": "scaffolding-create-service/v1",
    "requestHash": "sha256:abc123...",
    "idempotencyKey": "scaffold:tenant/acme-prod:9f8a...",
    "policyOutcome": "allow_with_approval",
    "approval": {
      "required": true,
      "policyRef": "scaffold-policy/v3",
      "reasonCodes": [
        "public_repo",
        "prod_target"
      ],
      "riskSummary": {
        "level": "high"
      },
      "reviewContext": {
        "service": "payments-api",
        "template": "go-service",
        "environment": "prod"
      },
      "suggestedApproverRoles": [
        "platform-admin"
      ],
      "ttlSeconds": 172800
    }
  }
}
```

---

## 12. Status Model Exposed Back to Clients

MCP should expose approval-related state only as reflected from Release Engine.

### 12.1 External Status States

Recommended normalized states:

- `queued`
- `running`
- `awaiting_approval`
- `approved`
- `rejected`
- `succeeded`
- `failed`
- `cancelled`
- `expired`

### 12.2 Important Rule

`approved` in client-visible status means:

- Release Engine has recorded that approval requirements were satisfied,

not:

- MCP decided approval had happened.

---

## 13. Example Flows

## 13.1 Low-Risk Request

Request:

- internal repo,
- approved template,
- dev tenant,
- no sensitive capabilities.

Policy result:

```json
{
  "policyOutcome": "allow",
  "approval": {
    "required": false
  }
}
```

Release Engine behavior:

- starts execution immediately.

---

## 13.2 Public Repository Request

Request:

- `visibility = public`.

Policy result:

```json
{
  "policyOutcome": "allow_with_approval",
  "approval": {
    "required": true,
    "reasonCodes": ["public_repo"]
  }
}
```

Release Engine behavior:

- places job in `awaiting_approval`,
- requires an authorized reviewer,
- resumes only after approval is granted.

---

## 13.3 Denied Template Usage

Request:

- restricted template,
- unapproved tenant.

Policy result:

```json
{
  "policyOutcome": "deny",
  "denialReasonCodes": ["restricted_template_not_allowed"]
}
```

Compiler behavior:

- returns a denied outcome to the caller,
- does not submit a Release Engine job.

---

## 14. Suggested Schema Definitions

### 14.1 Approval Context

```ts
type ApprovalContext = {
  required: boolean;
  policyRef?: string;
  reasonCodes?: string[];
  riskSummary?: {
    level?: "low" | "medium" | "high" | "critical";
    blastRadius?: "service" | "team" | "department" | "org";
    estimatedMonthlyCostBand?: "low" | "medium" | "high";
    sensitiveCapabilities?: string[];
  };
  reviewContext?: Record<string, string | number | boolean>;
  suggestedApproverRoles?: string[];
  ttlSeconds?: number;
};
```

### 14.2 Policy Outcome

```ts
type PolicyOutcome =
  | { outcome: "allow"; approvalRequired: false }
  | { outcome: "allow_with_approval"; approvalRequired: true; approval: ApprovalContext }
  | { outcome: "deny"; approvalRequired: false; denialReasonCodes: string[] };
```

---

## 15. Determinism and Idempotency

Approval metadata affects execution semantics and therefore must be handled carefully.

### 15.1 Request Hashing

If approval context is part of the execution contract, then the request hash or compiled-plan hash should include:

- policy outcome,
- approval required flag,
- approval reason codes,
- policy reference,
- and any approval fields that alter execution behavior.

Otherwise, the same scaffold request might produce different execution outcomes without changing the derived hash.

### 15.2 Recommended Rule

The hash projection **SHOULD** include all approval fields that materially affect workflow behavior.

It **MAY** exclude purely advisory text fields if they are non-semantic and unstable.

---

## 16. Audit Model

Approval-related auditability should include:

- who submitted the request,
- what policy outcome was computed,
- why approval was required,
- which policy version was used,
- what approval state transitions occurred in Release Engine,
- who approved or rejected,
- and when execution resumed.

The authoritative audit timeline should be reconstructable by combining:

- compiler submission metadata,
- Release Engine job metadata,
- Release Engine approval events.

---

## 17. Failure Modes

### 17.1 Missing Approval Metadata

If a job claims `policyOutcome = allow_with_approval` but omits required approval context, Release Engine should reject the submission or place it in a policy-error state.

### 17.2 Stale Policy Reference

If policy bundles evolve, old jobs may still reference prior policy versions. This is acceptable if the reference is audit metadata rather than a runtime dependency.

### 17.3 Suggested Roles Mismatch

If `suggestedApproverRoles` contains roles not recognized by Release Engine, Release Engine should ignore them or reject them according to local policy, but must not treat them as authoritative grants.

### 17.4 TTL Mismatch

If the compiler requests `ttlSeconds = 172800` and Release Engine caps pending approvals at 86400, Release Engine should apply its own cap and record the effective TTL.

---

## 18. Security Considerations

### 18.1 No Trust in Caller-Supplied Approval Decisions

Approval decisions must never be accepted from MCP or client-submitted metadata.

### 18.2 Approver Authorization

Suggested approver roles are hints only. Release Engine must validate approvers against its own identity and authorization systems.

### 18.3 Separation of Duties

If self-approval is prohibited, Release Engine must enforce it regardless of upstream metadata.

### 18.4 Tamper Resistance

Approval metadata included in hashing and job metadata improves traceability and reduces ambiguity when investigating workflow discrepancies.

---

## 19. Testing Strategy

Fixtures and golden tests from RFC-SCAFFOLD-004 should include:

- allow-without-approval cases,
- allow-with-approval cases,
- denial cases,
- public visibility approval cases,
- privileged template approval cases,
- TTL override cases,
- malformed approval metadata rejection cases,
- hash changes caused by approval requirement differences.

---

## 20. Open Questions

1. Should Release Engine always re-evaluate approval policy, or trust the compiled result by default?
2. Should `suggestedApproverRoles` be retained if different business units use different authorization models?
3. Should approval reason codes be standardized globally or scoped per domain?
4. Should denial outcomes ever be optionally submitted for audit-only recording?
5. Should approval review context be schema-bound or intentionally flexible?

---

## 21. Decision

This RFC proposes that:

- **approval requirement derivation may happen upstream,**
- **approval context may be attached during compilation,**
- but **approval enforcement and lifecycle management must remain in Release Engine.**

This preserves:

- a clean separation of concerns,
- a single approval source of truth,
- strong auditability,
- and resumable workflow execution.

---

