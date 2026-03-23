# RFC-SCAFFOLD-010 — Security Boundary and Audit Integration

**Status:** Draft  
**Authors:** Platform Engineering  
**Audience:** Platform team, MCP server implementers, Release Engine maintainers, security reviewers, compliance stakeholders, SRE/operations, portal/CLI owners  
**Last Updated:** March 15, 2026

---

## 1. Summary

This RFC defines the **security boundary**, **identity propagation model**, and **audit integration contract** for the scaffolding platform.

It formalizes the architectural decision that:

- **Release Engine is the authoritative system for execution security, approval enforcement, runtime audit, and compliance evidence**
- **MCP is responsible for front-door authentication, authorization, validation, redaction, and identity propagation**
- **MCP must not become a second compliance or approval authority**
- **End-to-end traceability must be preserved across MCP and Release Engine**

This RFC intentionally does **not** create a standalone compliance engine inside the scaffold layer. Instead, it ensures the scaffold layer integrates cleanly with the Release Engine’s authoritative controls and audit record.

---

## 2. Related RFCs

This RFC builds on:

- **RFC-SCAFFOLD-001** — Scaffolding Intent Model and MCP Contract
- **RFC-SCAFFOLD-002** — Template Catalog and Validation Rules
- **RFC-SCAFFOLD-003** — Compile-to-Release-Engine Mapping
- **RFC-SCAFFOLD-004** — Fixtures and Golden Tests
- **RFC-SCAFFOLD-005** — Approval Context and Release Engine Enforcement
- **RFC-SCAFFOLD-006** — Execution Status and Outcome Model
- **RFC-SCAFFOLD-007** — API Surface and Transport Bindings
- **RFC-SCAFFOLD-008** — Error Taxonomy and Remediation Model
- **RFC-SCAFFOLD-009** — Reference Implementation Plan

---

## 3. Problem Statement

The scaffold architecture spans at least two distinct layers:

1. **MCP / scaffold-facing layer**
    - receives requests,
    - validates and normalizes them,
    - compiles them to execution inputs,
    - exposes public APIs and projections.

2. **Release Engine**
    - enforces approvals,
    - applies execution-time policy,
    - performs or orchestrates actual work,
    - records workflow actions and outcomes.

Without an explicit boundary, teams may accidentally create one or more anti-patterns:

- duplicate approval logic in MCP and Release Engine,
- split audit trails with unclear authority,
- inconsistent identity propagation,
- incomplete compliance evidence,
- redaction mistakes between public and internal records,
- unclear ownership for tenant isolation and runtime authorization.

We need a clear contract stating **what MCP must secure**, **what Release Engine must enforce**, and **how traceability must be maintained across both layers**.

---

## 4. Goals

This RFC aims to:

1. define the security responsibility split between MCP and Release Engine,
2. establish Release Engine as the authoritative execution audit and compliance system,
3. define MCP’s front-door security obligations,
4. standardize identity, tenant, and correlation propagation,
5. define public vs internal visibility boundaries,
6. ensure end-to-end traceability for reviews, investigations, and compliance evidence,
7. prevent duplication of approval/compliance logic across layers.

---

## 5. Non-Goals

This RFC does **not**:

- redefine Release Engine’s internal approval workflow design,
- prescribe enterprise-wide IAM architecture,
- define cryptographic implementation specifics,
- replace Release Engine’s native audit/event system,
- require MCP to store full compliance evidence,
- make MCP responsible for runtime policy decisions,
- define every regulatory mapping for every jurisdiction.

---

## 6. Core Decision

### 6.1 Authoritative ownership

The platform adopts the following ownership model:

- **MCP owns request-boundary security**
- **Release Engine owns execution security and compliance authority**

Concretely:

### MCP owns
- caller authentication at the scaffold API boundary,
- caller authorization to invoke scaffold capabilities,
- request schema validation,
- template/input validation,
- tenant and identity resolution,
- correlation metadata generation,
- redaction and safe public response shaping,
- propagation of identity and trace metadata to Release Engine.

### Release Engine owns
- approval enforcement,
- runtime authorization and policy gates,
- separation-of-duties enforcement at execution time,
- execution-time tenancy enforcement,
- authoritative workflow audit records,
- compliance evidence for what actually occurred,
- execution decision records,
- action-level traceability for approvals, starts, pauses, failures, cancels, and completions.

---

## 7. Architectural Principles

### 7.1 Enforce where the action happens

Security and compliance controls should be enforced at the point where the consequential action occurs.

That means:

- request validity is enforced by MCP,
- execution legitimacy is enforced by Release Engine.

### 7.2 One authoritative audit trail for execution

There must be a single authoritative execution audit system.  
That system is **Release Engine**, because it is closest to the actual approvals, gates, and execution outcomes.

### 7.3 No duplicated approval engines

MCP may **collect**, **validate**, and **forward** approval-related metadata, but must not act as an approval decision authority for execution.

### 7.4 Public and internal views must remain separate

The scaffold API may expose normalized, user-safe summaries.  
It must not expose raw operator diagnostics, sensitive execution metadata, or internal policy details unless explicitly allowed.

### 7.5 Traceability must be continuous

Every scaffold request must be traceable from:

- requester identity,
- to normalized scaffold request,
- to compiled plan,
- to Release Engine execution,
- to approval and runtime events,
- to final outcome and resulting artifacts/resources.

---

## 8. Responsibility Model

## 8.1 High-level split

| Concern | MCP | Release Engine |
|---|---|---|
| API authentication | **Owns** | Consumes propagated identity |
| API authorization | **Owns** | May perform secondary checks |
| Request schema validation | **Owns** | No |
| Template validation | **Owns** | No |
| Compile-time normalization | **Owns** | No |
| Identity propagation | **Owns** | Consumes and records |
| Tenant propagation | **Owns** | Enforces at runtime |
| Approval policy hints | **Owns / forwards** | Consumes |
| Approval enforcement | No | **Owns** |
| Runtime policy enforcement | No | **Owns** |
| Execution authorization | No | **Owns** |
| Runtime separation of duties | No | **Owns** |
| Workflow audit records | Support only | **Owns** |
| Compliance evidence | Support only | **Owns** |
| Public-safe execution projection | **Owns** | Source input |
| Redacted user-facing errors | **Owns** | Source input / internal detail |
| Execution truth | Projection only | **Owns** |

---

## 8.2 Rationale

This split avoids three major failure modes:

### A. Split authority
If both systems decide execution approval independently, there is no clear source of truth.

### B. Broken evidence chain
If approvals and execution steps are recorded outside the execution system, evidence becomes weaker and harder to trust.

### C. Policy drift
If MCP and Release Engine separately encode similar controls, they will eventually diverge.

---

## 9. MCP Security Responsibilities

MCP is the **front door** to the scaffold platform and must enforce the controls appropriate to that role.

## 9.1 Authentication

MCP must authenticate the caller before accepting scaffold actions.

Examples include:

- human users via portal,
- CLI users,
- service principals,
- automation agents.

MCP must bind the authenticated principal to the scaffold request context.

---

## 9.2 Authorization

MCP must authorize whether the caller may:

- use scaffolding at all,
- use a given template,
- target a given tenant/environment/account/project,
- request a given capability class,
- perform read actions on execution status and outputs.

MCP authorization is about **access to the scaffold capability**, not final permission to execute runtime actions inside Release Engine.

---

## 9.3 Input validation and sanitization

MCP must validate:

- schema shape,
- required fields,
- enum values,
- field formats,
- template constraints,
- prohibited combinations,
- size limits,
- safe formatting for strings likely to be displayed or logged.

MCP should reject malformed or obviously unsafe inputs before submission to Release Engine.

---

## 9.4 Tenant and scope resolution

MCP must determine and attach:

- tenant identifier,
- workspace/project/account context,
- caller scope,
- originating client/tool,
- request correlation identifiers.

These must be explicit and not inferred opaquely downstream where possible.

---

## 9.5 Safe public error handling

MCP must prevent leakage of:

- raw backend stack traces,
- internal policy logic,
- secret values,
- sensitive operator notes,
- internal infrastructure details not meant for requesters.

MCP is responsible for returning the normalized public error model defined in prior RFCs.

---

## 9.6 Redaction before persistence or display

Where MCP stores request snapshots, it must avoid or minimize persistence of sensitive data not required for public workflow operation.

MCP should prefer:

- references over secrets,
- hashes over raw sensitive values where practical,
- redacted projections over raw backend payloads.

---

## 9.7 Correlation metadata generation

MCP must generate and preserve identifiers that allow stitching together the full lifecycle, including:

- request ID,
- idempotency key,
- trace ID,
- correlation ID,
- submission ID,
- compile artifact hash/version.

---

## 10. Release Engine Security and Compliance Responsibilities

Release Engine is the authoritative system for **execution control** and **execution evidence**.

## 10.1 Approval enforcement

Release Engine must authoritatively enforce:

- whether approval is required,
- who may approve,
- whether sufficient approvals have been granted,
- whether approval windows have expired,
- whether separation-of-duties constraints are satisfied,
- whether execution may resume after approval.

MCP may display approval status, but it must not independently authorize execution continuation.

---

## 10.2 Runtime policy enforcement

Release Engine must enforce runtime policies such as:

- target environment restrictions,
- forbidden deployment windows,
- tenancy/account boundaries,
- required gate checks,
- policy-as-code decisions,
- protected resource restrictions,
- concurrency/locking constraints where relevant.

---

## 10.3 Runtime authorization

Release Engine must validate that the execution is permitted in the target context using the propagated identity and any platform-specific execution credentials.

This includes ensuring:

- the tenant context is valid,
- the execution principal is authorized,
- runtime privilege assumptions are consistent with policy,
- cross-tenant or cross-boundary actions are not allowed unless explicitly sanctioned.

---

## 10.4 Audit records

Release Engine must maintain the authoritative audit trail for:

- execution creation,
- approval requests,
- approval grants/rejections,
- execution start,
- task transitions,
- policy denials,
- retries,
- cancellation requests,
- cancellation completion,
- terminal outcomes.

This audit trail is the compliance record for execution behavior.

---

## 10.5 Compliance evidence

Release Engine must be able to support evidence generation for questions such as:

- who requested the action,
- who approved it,
- what policy checks ran,
- when execution started and ended,
- what targets were affected,
- what outcome occurred,
- what controls prevented or blocked unsafe execution.

MCP may retain references to this evidence, but should not be treated as the canonical compliance store.

---

## 11. Identity Propagation Contract

To preserve accountability, MCP must pass identity and request context to Release Engine in a structured form.

## 11.1 Required propagated fields

At minimum, the following fields should be propagated:

```json
{
  "requestId": "req_123",
  "submissionId": "sub_123",
  "idempotencyKey": "idem_123",
  "traceId": "trace_123",
  "tenantId": "tenant_abc",
  "requester": {
    "subjectId": "user_456",
    "subjectType": "human",
    "displayName": "Jane Doe",
    "authProvider": "sso"
  },
  "origin": {
    "channel": "portal",
    "clientId": "portal-web",
    "sessionId": "sess_789"
  },
  "template": {
    "templateId": "service-template",
    "templateVersion": "3.2.1"
  },
  "compile": {
    "compilerVersion": "scaffold-compiler/1.8.0",
    "planHash": "sha256:..."
  }
}
```

---

## 11.2 Semantics

### `requestId`
Stable identifier for the inbound scaffold request.

### `submissionId`
Identifier for the MCP-side submission record.

### `idempotencyKey`
Used to deduplicate repeated submit attempts safely.

### `traceId`
Used for distributed tracing and investigation.

### `tenantId`
Authoritative tenant or organizational boundary for the request.

### `requester`
Identity of the initiating subject as known by MCP.

### `origin`
Client/channel context to support audit and abuse analysis.

### `template`
Identifies the requested scaffold definition.

### `compile`
Binds the Release Engine execution back to the exact compiled plan/version.

---

## 11.3 Integrity expectations

Release Engine should treat propagated identity/context as **trusted input only from trusted MCP callers**, not from arbitrary end-user fields.

Where possible:

- MCP-to-Release-Engine calls should be authenticated service-to-service,
- propagated identity should be structured, not free-form,
- security-sensitive fields should not be overridable by user payload content.

---

## 12. Tenant Isolation Model

Tenant isolation is a shared concern with different responsibilities at each layer.

## 12.1 MCP responsibilities

MCP must:

- resolve the tenant from authenticated context,
- ensure the caller can act within that tenant,
- prevent requests from ambiguously spanning tenants unless explicitly supported,
- pass the tenant context explicitly downstream.

## 12.2 Release Engine responsibilities

Release Engine must:

- enforce tenant/account/workspace boundaries at runtime,
- ensure approvals and execution operate within the correct tenant scope,
- prevent execution from escaping the authorized tenant context,
- record tenant identity in execution audit records.

## 12.3 Rule

A request must not become *less constrained* when it moves from MCP to Release Engine.

---

## 13. Public vs Internal Data Boundaries

A core security requirement is the separation of **public-safe operational state** from **internal execution detail**.

## 13.1 MCP public projection may include

- execution status,
- normalized stage,
- approval-required / approval-pending summaries,
- requester-facing remediation,
- user-safe timestamps,
- safe output references,
- normalized error codes and categories.

## 13.2 MCP public projection must not include by default

- raw stack traces,
- backend exception messages,
- internal policy rule source text,
- sensitive operator comments,
- credentials or tokens,
- internal infrastructure topology details,
- unredacted execution payloads,
- backend-only runbook links not intended for requesters.

## 13.3 Release Engine internal records may include

- detailed execution logs,
- policy evaluation results,
- approver identities,
- task-level transitions,
- system actor actions,
- operator diagnostics,
- backend references and remediation notes.

The fact that Release Engine may contain this information does **not** imply MCP should mirror or expose it.

---

## 14. Audit Integration Model

MCP and Release Engine both produce records, but they are not equal in authority.

## 14.1 MCP records are integration records

MCP may record:

- request received,
- request normalized,
- template resolved,
- compile completed,
- submission sent,
- execution ID linked,
- projection updated,
- user-facing status served,
- cancellation request forwarded.

These are useful for debugging and product analytics, but they are not the compliance authority for execution.

## 14.2 Release Engine records are authoritative execution records

Release Engine must record:

- job created,
- approval requested,
- approval decided,
- policy gate evaluated,
- execution started,
- execution transitioned,
- execution completed/failed/cancelled,
- runtime actor/system actions.

These records are authoritative for operational audit and compliance evidence.

## 14.3 Correlation requirement

Every MCP record associated with execution should include enough linkage to correlate with Release Engine records, especially:

- `submissionId`
- `requestId`
- `traceId`
- `executionId`
- `tenantId`
- `planHash`

---

## 15. End-to-End Traceability Requirements

The platform must support reconstructing the lifecycle of a scaffold request.

## 15.1 Trace chain

At minimum, investigators should be able to traverse:

1. authenticated requester
2. scaffold request
3. normalized request snapshot
4. template version
5. compile version and plan hash
6. MCP submission record
7. Release Engine execution ID
8. approval and policy events
9. execution outcome
10. resulting artifacts/resources/change references

---

## 15.2 Required join keys

The following identifiers should be stable enough for investigation:

- `requestId`
- `submissionId`
- `executionId`
- `traceId`
- `tenantId`
- `templateId`
- `templateVersion`
- `planHash`

---

## 15.3 Investigation outcomes supported

The model should support answering questions like:

- Who initiated this scaffold?
- Which template version was used?
- What exact compiled plan was executed?
- Which tenant did it target?
- Was approval required?
- Who approved or rejected it?
- Which policy blocked it?
- What happened at runtime?
- What user-safe result was shown to the requester?
- Which resources or changes were created?

---

## 16. Approval and Separation-of-Duties Model

This RFC aligns with RFC-SCAFFOLD-005 while clarifying ownership.

## 16.1 MCP role

MCP may:

- collect approval context inputs,
- validate that required approval-related fields are present,
- display approval status,
- expose approval wait states in normalized status,
- return requester-safe remediation if approval is pending or rejected.

## 16.2 MCP must not

MCP must not:

- independently decide that approval is sufficient for execution,
- treat approval metadata supplied by the requester as authoritative,
- bypass Release Engine approval gates,
- maintain a separate approval source of truth.

## 16.3 Release Engine role

Release Engine must:

- determine whether approvals are required under policy,
- enforce approval before protected execution proceeds,
- verify approver identity and authorization,
- enforce separation-of-duties constraints,
- record the approval chain in the audit trail.

---

## 17. Error and Remediation Security Rules

The normalized error model from RFC-SCAFFOLD-008 must be applied with security boundaries in mind.

## 17.1 User-facing errors should be safe and actionable

Errors returned by MCP should explain:

- what failed,
- what the requester can do next,
- whether retry is appropriate,
- whether another actor must intervene.

But they should avoid revealing sensitive internal details.

## 17.2 Internal diagnostics remain internal

Detailed backend diagnostics should remain in:

- Release Engine logs,
- operator-only dashboards,
- restricted internal records.

## 17.3 Dual-message pattern

Where useful, the platform may maintain:

- a **public message** for requesters,
- an **internal diagnostic message** for operators.

MCP should expose only the public form by default.

---

## 18. Data Retention and Redaction Principles

This RFC does not define exact retention durations, but it defines ownership principles.

## 18.1 MCP retention guidance

MCP should retain only the minimum data needed for:

- request lifecycle support,
- user-facing status retrieval,
- debugging of compile/submit behavior,
- correlation to Release Engine records.

MCP should avoid long-term storage of sensitive execution detail that already belongs in Release Engine.

## 18.2 Release Engine retention guidance

Release Engine should retain execution audit and compliance evidence according to enterprise requirements because it is the authoritative execution record.

## 18.3 Redaction rule

If a field is not needed for requester experience, correlation, or compile determinism, MCP should strongly prefer not to persist it.

---

## 19. Logging and Observability

## 19.1 MCP logs should include

- request ID,
- submission ID,
- trace ID,
- tenant ID,
- template ID/version,
- execution ID once known,
- normalized error code,
- major state transitions.

## 19.2 MCP logs should exclude or redact

- secrets,
- access tokens,
- unbounded user input where unsafe,
- raw backend payloads containing internal-only data.

## 19.3 Release Engine observability

Release Engine should emit logs, metrics, and events sufficient to support:

- audit investigation,
- policy denial analysis,
- runtime debugging,
- SLA/SLO monitoring,
- approval flow investigation.

---

## 20. Service-to-Service Trust Model

MCP and Release Engine must communicate using authenticated service-to-service channels.

## 20.1 Requirements

- mutual trust must be established between services,
- caller identity at the service level must be authenticated,
- propagated end-user identity must be structured and attributable,
- security-sensitive fields must not rely on caller-supplied free text.

## 20.2 Principle

Release Engine should trust **MCP as a submitting service**, not arbitrary fields from an end user.

---

## 21. Canonical Security Invariants

The following invariants must hold:

1. **No unauthenticated scaffold submission is accepted by MCP.**
2. **No execution bypasses Release Engine approval enforcement when approval is required.**
3. **No execution is considered authoritative based only on MCP records.**
4. **No public status surface exposes internal-only diagnostics by default.**
5. **Every execution can be linked back to a requester, tenant, and compile plan.**
6. **Tenant constraints must not be weakened across system boundaries.**
7. **MCP must not become a shadow approval or compliance engine.**
8. **Release Engine must remain the system of record for execution audit.**

---

## 22. Example Record Shapes

## 22.1 MCP integration/audit-support record

```json
{
  "requestId": "req_123",
  "submissionId": "sub_123",
  "tenantId": "tenant_abc",
  "requesterSubjectId": "user_456",
  "templateId": "service-template",
  "templateVersion": "3.2.1",
  "planHash": "sha256:abc123",
  "status": "submitted",
  "executionId": "exec_789",
  "traceId": "trace_xyz",
  "createdAt": "2026-03-15T09:12:10Z"
}
```

## 22.2 Release Engine authoritative audit record

```json
{
  "executionId": "exec_789",
  "tenantId": "tenant_abc",
  "requestId": "req_123",
  "submissionId": "sub_123",
  "traceId": "trace_xyz",
  "actor": {
    "type": "human",
    "subjectId": "user_456"
  },
  "eventType": "approval_granted",
  "eventTime": "2026-03-15T09:15:42Z",
  "approval": {
    "approverSubjectId": "user_999",
    "policyRef": "policy/prod-change"
  }
}
```

---

## 23. Failure Scenarios and Expected Ownership

## 23.1 Invalid schema submitted

**Owner:** MCP  
**Reason:** request-boundary validation failure

**Expected behavior:**
- MCP rejects before submit
- no Release Engine execution created
- user gets normalized validation error

---

## 23.2 Caller lacks permission to use template

**Owner:** MCP  
**Reason:** capability authorization failure

**Expected behavior:**
- MCP denies request
- no execution created
- denial may be logged as access-control event at MCP

---

## 23.3 Approval required before runtime

**Owner:** Release Engine  
**Reason:** approval enforcement is execution-time control

**Expected behavior:**
- execution enters approval-pending state
- Release Engine records approval requirement and later decision
- MCP projects safe approval-pending status

---

## 23.4 Policy gate denies production execution

**Owner:** Release Engine  
**Reason:** runtime policy enforcement

**Expected behavior:**
- Release Engine blocks execution
- authoritative audit captures denial
- MCP shows normalized denied/failed outcome without exposing sensitive policy internals

---

## 23.5 Status API returns failure details to requester

**Owner:** MCP  
**Reason:** public response shaping is MCP responsibility

**Expected behavior:**
- sensitive diagnostics are redacted
- safe remediation is included if available
- correlation ID may be returned for support escalation

---

## 24. Migration Guidance

If current implementations blur the boundary, migrate toward this model in phases.

### Phase 1 — Clarify authority
- document Release Engine as execution authority
- remove ambiguous wording implying MCP owns execution compliance

### Phase 2 — Standardize propagation
- require request/tenant/trace identity fields on submissions

### Phase 3 — Reduce duplication
- remove MCP-side approval decisions that affect execution permission
- keep only validation and display logic in MCP

### Phase 4 — Tighten redaction
- ensure status and error APIs expose only requester-safe detail

### Phase 5 — Audit correlation hardening
- ensure MCP records and Release Engine records can be joined reliably

---

## 25. Operational Recommendations

### 25.1 Build support tooling around correlation IDs
Support staff should be able to start with an MCP-visible request or execution summary and pivot to Release Engine audit records.

### 25.2 Treat Release Engine evidence as authoritative in incidents
When MCP and Release Engine appear inconsistent, Release Engine execution audit wins for what actually occurred.

### 25.3 Keep projections lightweight
Do not copy all execution internals into MCP merely for convenience.

### 25.4 Prefer explicit metadata propagation
Avoid hidden coupling or implied context wherever possible.

---

## 26. Open Questions

1. Should MCP persist a redacted request snapshot, or only normalized references plus plan hash?
2. Should approver identity ever be exposed in requester-facing status, or remain internal by default?
3. Should some policy denial categories expose more structured requester-safe detail?
4. Should Release Engine return explicit audit reference IDs for support workflows?
5. What minimum retention window should MCP keep for correlation records after execution completion?

---

## 27. Decision

This RFC adopts the following model:

- **MCP is the scaffold request security boundary**
- **Release Engine is the authoritative execution security, audit, and compliance authority**
- **MCP must not duplicate approval or compliance authority**
- **Identity, tenant, and trace metadata must be propagated end-to-end**
- **Public-safe projections and internal audit records must remain clearly separated**
- **Execution investigations must be able to traverse from request to runtime evidence**

This keeps authority aligned with where actions actually occur while preserving a clean public scaffold API.

---


