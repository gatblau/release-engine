# RFC-SCAFFOLD-009: Reference Implementation Plan

---

## 1. Summary

This RFC defines the **reference implementation plan** for the scaffolding platform described in RFC-SCAFFOLD-001 through RFC-SCAFFOLD-008.

It translates the prior conceptual RFCs into an implementable architecture covering:

- MCP server module boundaries,
- request validation and compilation flow,
- Release Engine integration,
- persistence responsibilities,
- event emission,
- read models,
- rollout phases,
- testing strategy,
- and operational guardrails.

This RFC is intentionally practical. It does not redefine the scaffold contracts; it specifies **how to build them** in a maintainable, incrementally deployable way.

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

---

## 3. Problem Statement

The scaffold RFC line now defines:

- request shape,
- template resolution,
- approval context,
- execution status,
- public APIs,
- and normalized errors.

However, without a reference implementation plan, teams may still diverge on critical engineering choices:

- where validation lives,
- whether MCP stores execution truth or only a projection,
- how idempotency is enforced,
- how events are emitted and reconciled,
- how adapters isolate external systems,
- and how rollout occurs safely.

We need an implementation plan that preserves the intended architecture while reducing integration ambiguity.

---

## 4. Goals

This RFC aims to:

1. define a reference service decomposition for scaffolding,
2. establish clear ownership boundaries between MCP and Release Engine,
3. describe the end-to-end request lifecycle in implementation terms,
4. define required persistence and read-model responsibilities,
5. standardize event emission and reconciliation patterns,
6. enable phased rollout with low migration risk,
7. support testability, observability, and operational safety.

---

## 5. Non-Goals

This RFC does **not** define:

- exact programming language selection,
- vendor-specific infrastructure products,
- exact database schema DDL,
- internal org chart or team staffing,
- UI implementation details,
- approval system implementation internals,
- every workflow step for every scaffold template.

This RFC defines the **reference architecture and implementation approach**, not all low-level code.

---

## 6. Design Principles

### 6.1 MCP Is the Public Contract Boundary

MCP owns:

- input normalization,
- contract validation,
- compilation,
- public API/tool exposure,
- idempotent submit behavior,
- normalized status projection,
- and public error shaping.

### 6.2 Release Engine Owns Runtime Truth

Release Engine owns:

- workflow execution,
- task retries,
- approval waits,
- runtime state transitions,
- and execution completion truth.

MCP should not reimplement workflow orchestration.

### 6.3 Read and Write Concerns Should Be Separated

Write-path submission logic and read-path status projection should be separable so that:

- submissions remain simple and reliable,
- reads remain fast,
- and event-driven updates can evolve independently.

### 6.4 External Integrations Must Be Adapter-Isolated

Git providers, catalogs, secret stores, CI systems, and approval systems must be wrapped in adapters to prevent contract leakage.

### 6.5 Public Models Must Be Stable Even if Internals Evolve

Internal workflow or dependency changes must not force breaking changes on clients.

### 6.6 Failures Must Be Recoverable and Traceable

Every submission and execution should be traceable through:

- execution IDs,
- correlation IDs,
- idempotency keys,
- event sequence handling,
- and diagnostics references.

---

## 7. Reference Architecture

At a high level, the implementation consists of five logical layers:

1. **Public Contract Layer**
2. **Scaffold Domain Layer**
3. **Integration/Adapter Layer**
4. **Execution Backend Layer**
5. **Projection and Eventing Layer**

### 7.1 Logical Diagram

```text
Clients (Portal / CLI / Agents)
            |
            v
   MCP Public Contract Layer
   - HTTP endpoints / MCP tools
   - AuthN/AuthZ
   - Idempotency
   - Request validation
   - Error normalization
            |
            v
   Scaffold Domain Layer
   - Template resolution
   - Parameter normalization
   - Policy checks
   - Approval context assembly
   - Compile to execution spec
            |
            v
   Integration / Adapter Layer
   - Template catalog adapter
   - Policy adapter
   - Approval adapter
   - Release Engine submit adapter
   - Event ingestion adapter
            |
            v
      Release Engine
   - Workflow execution
   - Runtime state
   - Retries
   - Approval waits
   - Task outputs
            |
            v
 Projection / Read Model Layer
 - Execution projection store
 - Event consumers
 - Normalized status materializer
 - List/search indexes
```

---

## 8. Major Components

## 8.1 MCP API/Tool Front Door

Responsibilities:

- expose submit/validate/get/list/cancel interfaces,
- authenticate callers,
- authorize operations,
- accept idempotency keys,
- attach correlation metadata,
- and return normalized public payloads.

This layer should be thin and delegate business logic downward.

---

## 8.2 Scaffold Application Service

This is the main orchestration entrypoint inside MCP for scaffold-specific behavior.

Responsibilities:

- receive normalized requests from transport layer,
- run validation,
- invoke template and policy services,
- compile request to execution spec,
- submit to Release Engine,
- write submission records,
- return normalized execution resource.

Suggested methods:

- `validateScaffold(request, context)`
- `submitScaffold(request, context, idempotencyKey)`
- `getExecution(executionId, context)`
- `listExecutions(filter, context)`
- `cancelExecution(executionId, context)`

---

## 8.3 Template Resolver Service

Responsibilities:

- locate requested template,
- resolve effective version,
- load parameter schema,
- apply defaults,
- validate template-specific constraints,
- provide render/compile metadata.

This service should depend on a template catalog adapter rather than raw storage.

---

## 8.4 Policy and Approval Context Service

Responsibilities:

- evaluate policy-relevant attributes,
- determine hard deny vs approval-required vs allowed,
- assemble approval context payload,
- generate policy decision summaries safe for public responses.

This service must keep public-safe summaries distinct from internal policy traces.

---

## 8.5 Compiler Service

Responsibilities:

- transform normalized scaffold intent into Release Engine execution input,
- derive stable plan fingerprints,
- map fields into workflow variables/inputs,
- determine workflow/template selection,
- attach execution metadata for traceability.

Outputs should be deterministic for the same normalized request and template version.

---

## 8.6 Submission Repository

Responsibilities:

- persist scaffold submission records,
- bind idempotency key to canonical request fingerprint,
- map public execution ID to backend execution reference,
- store submission-time metadata and audit-relevant attributes,
- support replay-safe submit responses.

This repository is **not** the source of runtime truth; it is the source of MCP-side submission truth.

---

## 8.7 Execution Projection Service

Responsibilities:

- consume Release Engine events and/or poll backend state,
- normalize backend states into RFC-SCAFFOLD-006 states,
- attach issues/outcomes/outputs,
- materialize read-friendly execution records,
- support list and get operations efficiently.

This is the primary MCP read model for execution status.

---

## 8.8 Event Publisher

Responsibilities:

- publish MCP-normalized scaffold lifecycle events,
- ensure event payloads conform to public contracts,
- preserve ordering semantics where possible,
- avoid leaking backend-only details.

This may publish from the projection layer rather than directly from submit path.

---

## 8.9 Cancel Service

Responsibilities:

- authorize cancel requests,
- determine whether state is cancellable,
- forward cancel intent to Release Engine,
- update MCP-side projection when cancel request is accepted,
- expose consistent terminal/non-terminal behavior to clients.

---

## 9. Proposed Module Structure

A reference codebase might be structured as follows:

```text
/scaffold
  /transport
    http/
    mcp_tools/
  /application
    scaffold_service/
    cancel_service/
    query_service/
  /domain
    intent/
    validation/
    template_resolution/
    policy/
    approval/
    compile/
    status_normalization/
    errors/
  /adapters
    template_catalog/
    release_engine/
    approval_system/
    policy_engine/
    git_provider/
    ci_provider/
    secret_store/
  /persistence
    submission_repo/
    projection_repo/
    idempotency_repo/
    event_outbox/
  /events
    consumers/
    publishers/
    schemas/
  /testing
    fixtures/
    golden/
    integration/
    conformance/
```

This is illustrative, not mandatory.

---

## 10. Ownership Boundaries

### 10.1 MCP Owns

- public request contracts,
- validation and normalization,
- template resolution,
- policy decision exposure,
- approval context assembly,
- compile output generation,
- idempotency enforcement,
- public execution projection,
- normalized events.

### 10.2 Release Engine Owns

- workflow scheduling,
- runtime execution,
- retries and backoff,
- approval pause/resume semantics,
- step-level outputs,
- runtime timestamps,
- terminal execution truth.

### 10.3 Shared but Separated Concerns

Both systems may reference:

- execution identifiers,
- approval references,
- correlation metadata,
- issue codes,
- and outputs.

But they should do so via stable contracts, not shared internal tables.

---

## 11. Data Model Overview

The implementation should maintain at least three logical record types.

### 11.1 Submission Record

Represents the accepted scaffold request at submit time.

Suggested fields:

- `submissionId`
- `executionId`
- `backendExecutionRef`
- `idempotencyKey`
- `requestFingerprint`
- `requestSnapshot`
- `compiledPlanHash`
- `templateId`
- `templateVersion`
- `requester`
- `tenant`
- `createdAt`
- `correlationId`

Purpose:

- idempotency,
- submit replay,
- audit linkage,
- backend mapping.

---

### 11.2 Execution Projection Record

Represents the current normalized public state.

Suggested fields:

- `executionId`
- `state`
- `outcome`
- `statusSummary`
- `approval`
- `issues`
- `outputs`
- `submittedAt`
- `startedAt`
- `updatedAt`
- `completedAt`
- `requester`
- `templateId`
- `templateVersion`
- `tenant`
- `labels`
- `searchTokens`
- `version`

Purpose:

- fast reads,
- list/filter queries,
- event publishing basis.

---

### 11.3 Event Offset / Reconciliation Record

Tracks event ingestion progress and deduplication.

Suggested fields:

- `consumerName`
- `backendEventId`
- `backendExecutionRef`
- `receivedAt`
- `appliedAt`
- `projectionVersion`
- `checksum`

Purpose:

- exactly-once-ish projection handling,
- replay safety,
- debugging.

---

## 12. End-to-End Lifecycle

## 12.1 Validate

Flow:

1. client submits candidate scaffold request,
2. MCP normalizes request shape,
3. template is resolved,
4. parameters are validated,
5. policy is evaluated,
6. approval context is derived if needed,
7. normalized validation result is returned.

No execution should be created in this path.

---

## 12.2 Submit

Flow:

1. client sends scaffold request with idempotency key,
2. MCP authenticates and authorizes submit,
3. request is normalized and fingerprinted,
4. idempotency store is checked,
5. template resolution and policy evaluation run,
6. compiler generates execution input,
7. submission adapter calls Release Engine,
8. submission record is written,
9. initial execution projection is written,
10. normalized response is returned.

Recommended initial public state:

- `accepted` or `submitted`, depending on final choice from RFC-SCAFFOLD-006.

---

## 12.3 Execution Updates

Flow:

1. Release Engine emits execution events,
2. MCP event consumer receives them,
3. backend state is normalized,
4. projection is updated if event is newer,
5. MCP-normalized event may be published,
6. get/list APIs serve updated projection.

---

## 12.4 Cancel

Flow:

1. client requests cancellation,
2. MCP verifies authorization,
3. MCP checks projection/backend state for cancellability,
4. cancel intent is sent to Release Engine,
5. MCP returns accepted/rejected result,
6. terminal cancellation state arrives asynchronously through normal status updates.

---

## 12.5 Read

Flow:

1. client requests execution or list,
2. MCP authorizes read,
3. execution projection is fetched,
4. normalized response is returned.

Read paths should avoid blocking on direct Release Engine calls except for explicit reconciliation fallback.

---

## 13. Idempotency Design

Idempotency is essential for reliable submit behavior.

### 13.1 Key Principles

- the same idempotency key with the same material request returns the original result,
- the same key with a different material request returns conflict,
- idempotency decisions happen before backend submission,
- uncertain submit outcomes must support safe recovery.

### 13.2 Material Fingerprint

The request fingerprint should be derived from:

- normalized request content,
- effective template version,
- tenant context,
- materially relevant submission metadata.

It should exclude non-material values such as tracing headers.

### 13.3 Failure Recovery

If MCP cannot determine whether backend submission succeeded, it should:

- preserve the idempotency record in a pending/uncertain state,
- attempt backend reconciliation using correlation metadata,
- return a retry-safe response strategy.

This avoids double-creation under partial failure.

---

## 14. Persistence Strategy

### 14.1 Recommended Split

Use separate logical stores or schemas for:

- idempotency/submission records,
- execution projection records,
- event ingestion offsets,
- outbox/publish records if event publishing is transactional.

### 14.2 Why Split

Because these concerns have different characteristics:

| Concern | Access Pattern | Consistency Need |
|---|---|---|
| submission/idempotency | point lookup, write-heavy at submit | strong |
| execution projection | read-heavy, filter/list | high but eventually consistent is acceptable |
| event offsets | append/update small records | strong enough for dedupe |
| outbox | asynchronous publish | durable |

---

## 15. Read Model and Projection Semantics

### 15.1 Projection Is the Public Read Source

`Get Execution` and `List Executions` should primarily read from MCP’s normalized projection, not directly from Release Engine.

### 15.2 Eventual Consistency

Clients must tolerate slight lag between backend truth and MCP projection.

This lag should be:

- measured,
- bounded operationally,
- and surfaced in monitoring.

### 15.3 Reconciliation Fallback

If a projection is missing or stale beyond an operational threshold, MCP may perform a backend reconciliation read and then repair the projection.

This should be exceptional, not the default path.

---

## 16. Eventing Model

### 16.1 Event Sources

Two event streams may exist:

1. **backend execution events** from Release Engine
2. **public normalized scaffold events** from MCP

These should not be conflated.

### 16.2 Backend Event Consumer

Responsibilities:

- ingest backend events,
- deduplicate,
- map backend states to normalized states,
- upsert execution projection,
- trigger normalized event publish if projection materially changed.

### 16.3 Normalized Event Publisher

Suggested event types:

- `scaffold.execution.accepted`
- `scaffold.execution.started`
- `scaffold.execution.awaiting_approval`
- `scaffold.execution.completed`
- `scaffold.execution.failed`
- `scaffold.execution.cancelled`
- `scaffold.execution.rejected`
- `scaffold.execution.expired`

### 16.4 Material Change Rule

Do not publish a new normalized event for every internal heartbeat. Publish when one of the following changes:

- public `state`
- public `outcome`
- approval wait metadata
- primary issue
- terminal outputs
- terminal timestamps

---

## 17. Adapter Design

Adapters isolate MCP from dependency-specific details.

## 17.1 Template Catalog Adapter

Responsibilities:

- fetch template metadata,
- fetch versioned schema,
- fetch compile/render configuration.

Must not leak storage-specific details into domain logic.

---

## 17.2 Policy Adapter

Responsibilities:

- evaluate policy inputs,
- return normalized allow/deny/approval-required decision,
- provide sanitized reasons and internal diagnostics handles.

---

## 17.3 Approval Adapter

Responsibilities:

- format approval context,
- link execution to approval reference,
- ingest approval-related events if needed.

Approval action handling may remain outside scaffold APIs, but the adapter must still support status visibility.

---

## 17.4 Release Engine Adapter

Responsibilities:

- submit compiled execution request,
- cancel backend execution,
- map backend IDs and states,
- retrieve execution details for reconciliation.

This adapter is one of the most important contract boundaries.

### Suggested interface

```ts
interface ReleaseEngineAdapter {
  submitExecution(input: CompiledScaffoldExecution): Promise<SubmitResult>;
  cancelExecution(backendExecutionRef: string): Promise<CancelResult>;
  getExecution(backendExecutionRef: string): Promise<BackendExecutionSnapshot>;
  mapEvent(event: BackendExecutionEvent): NormalizationInput;
}
```

---

## 18. Compilation Pipeline

The submit path should use a clear sequence.

### 18.1 Pipeline

1. request normalization
2. base schema validation
3. template resolution
4. parameter validation/defaulting
5. policy evaluation
6. approval context derivation
7. compile to backend execution spec
8. plan hash generation
9. backend submission

### 18.2 Why Explicit Ordering Matters

This improves:

- determinism,
- debuggability,
- idempotency correctness,
- golden test coverage,
- and policy clarity.

---

## 19. Status Normalization Pipeline

Backend execution details should be transformed into public status using a dedicated normalizer.

### 19.1 Inputs

- backend runtime state,
- backend timestamps,
- backend task outputs,
- approval status,
- backend errors/issues,
- submission metadata.

### 19.2 Outputs

- public `state`
- public `outcome`
- normalized `issues`
- normalized `outputs`
- approval block
- timestamps
- status summary

### 19.3 Rule

The normalization logic should be centralized in one module, not reimplemented in multiple API handlers.

---

## 20. Error Mapping Implementation

RFC-SCAFFOLD-008 defined the public error contract. This RFC recommends implementing error mapping through:

- typed internal domain errors,
- adapter error translators,
- a central public error mapper.

### 20.1 Recommended Layers

| Layer | Error Form |
|---|---|
| domain | typed domain error |
| adapter | dependency/provider error |
| application | normalized scaffold error |
| transport | public API/tool error payload |

### 20.2 Rule

Transport handlers should not invent codes ad hoc. All public scaffold errors should pass through the same mapper.

---

## 21. Security Boundaries

The implementation must enforce separation between:

- public execution data,
- internal workflow details,
- sensitive provider responses,
- approval-private information,
- and operator-only diagnostics.

### 21.1 Sensitive Data Rules

Do not persist in public projection:

- raw secrets,
- token values,
- private approval comments unless explicitly allowed,
- raw dependency payloads,
- stack traces.

### 21.2 Public Projection Rule

The projection store should contain only data safe for authorized public/API readers within tenant scope.

---

## 22. Authorization Model in Implementation

Authorization should be evaluated per operation:

- validate
- submit
- read
- list
- cancel

The implementation should not assume submit permission implies read or cancel permission.

### 22.1 Recommended Pattern

Use a dedicated authorization service with methods like:

- `canValidate(context, request)`
- `canSubmit(context, request)`
- `canReadExecution(context, execution)`
- `canCancelExecution(context, execution)`

This prevents ad hoc policy drift.

---

## 23. Observability Design

The reference implementation should emit:

### 23.1 Metrics

- submit count
- validate count
- validation failure rate
- idempotency replay rate
- submit latency
- backend submission failure rate
- projection lag
- approval wait duration
- execution terminal outcomes
- cancel acceptance rate
- event processing retry rate

### 23.2 Logs

Structured logs should include:

- `correlationId`
- `executionId`
- `backendExecutionRef`
- `idempotencyKeyHash` or safe derivative
- `tenant`
- `templateId`
- `state`
- `outcome`
- `errorCode`

### 23.3 Traces

Trace across:

- public request handler,
- template resolution,
- policy evaluation,
- backend submission,
- event consumption,
- projection update.

---

## 24. Rollout Strategy

A phased rollout reduces risk.

## 24.1 Phase 0 — Offline Contract Validation

Deliver:

- schemas,
- fixtures,
- golden tests,
- compiler snapshots,
- state normalization tests.

No production traffic yet.

### Exit Criteria

- contract fixtures stable,
- compile mappings reviewed,
- error/status examples approved.

---

## 24.2 Phase 1 — Validate-Only Path

Deliver:

- validate endpoint/tool,
- template resolution,
- policy checks,
- approval-required signaling,
- error normalization.

No backend execution submission yet.

### Benefits

- proves request model,
- exercises catalog and policy integration,
- supports portal UX early.

### Exit Criteria

- validation success/error behavior stable,
- template catalog trustworthy,
- policy decision visibility approved.

---

## 24.3 Phase 2 — Submit with Minimal Execution Type

Deliver:

- one low-risk scaffold template,
- idempotent submit,
- backend submission adapter,
- execution get endpoint,
- minimal projection ingestion.

### Candidate Scope

A simple repository bootstrap or low-privilege service type is ideal.

### Exit Criteria

- submit replay safe,
- end-to-end execution works,
- projection correctness verified,
- cancellation semantics proven for selected workflow.

---

## 24.4 Phase 3 — Full Read Model and Events

Deliver:

- list executions,
- normalized public events,
- projection repair tooling,
- richer outputs and issues,
- approval wait visibility.

### Exit Criteria

- event consumers stable,
- list/search performant,
- projection lag within target SLO.

---

## 24.5 Phase 4 — Multi-Template Expansion

Deliver:

- broader template coverage,
- provider-specific adapters,
- partial-success handling,
- richer remediation payloads.

### Exit Criteria

- template onboarding playbook exists,
- no contract drift across template types.

---

## 24.6 Phase 5 — Hardening and Migration

Deliver:

- operational dashboards,
- replay/reconciliation tooling,
- retention jobs,
- advanced audit integrations,
- deprecation of legacy scaffold flows if applicable.

---

## 25. Delivery Workstreams

A practical delivery plan likely needs parallel tracks.

### 25.1 Contract and Domain Workstream

- schemas
- normalization logic
- compiler
- error mapper
- fixture coverage

### 25.2 Backend Integration Workstream

- Release Engine adapter
- event ingestion
- cancel flow
- reconciliation logic

### 25.3 Persistence and Query Workstream

- submission repo
- projection store
- list filters
- outbox/offset tracking

### 25.4 Consumer Integration Workstream

- portal integration
- CLI support
- agent tool bindings
- event subscriber examples

### 25.5 Operations Workstream

- logging
- metrics
- dashboards
- alerting
- replay tools

---

## 26. Testing Strategy

The implementation should be tested at four levels.

## 26.1 Unit Tests

Cover:

- request normalization,
- template validation,
- policy decision mapping,
- error mapping,
- state normalization,
- idempotency fingerprinting.

---

## 26.2 Golden Tests

Reuse RFC-SCAFFOLD-004 fixture style for:

- compile outputs,
- validation results,
- error payloads,
- terminal status payloads,
- event payloads.

These are especially valuable for preventing contract drift.

---

## 26.3 Integration Tests

Cover:

- catalog adapter,
- policy adapter,
- backend submit/cancel adapter,
- event ingestion pipeline,
- projection updates.

Use deterministic fake adapters where possible.

---

## 26.4 End-to-End Tests

Cover:

1. validate success
2. validate approval required
3. validate denial
4. submit success
5. submit idempotent replay
6. submit conflict on changed request
7. execution transitions to running
8. approval wait visible
9. approval rejected
10. terminal success outputs
11. terminal failure issues
12. cancel accepted
13. cancel rejected due to terminal state
14. projection repair after missed event

---

## 27. Operational Tooling

The reference implementation should include internal tools for:

- reprocessing backend events,
- rebuilding execution projections,
- locating execution by correlation ID,
- diagnosing idempotency disputes,
- comparing backend truth vs MCP projection,
- replaying normalization against stored backend snapshots.

These tools are essential for real operations and should not be treated as optional.

---

## 28. Failure Modes and Recovery

### 28.1 Submit Succeeds but MCP Crashes Before Record Write

Mitigation:

- include correlation metadata in backend submission,
- reconcile on retry using idempotency key and correlation ID,
- use durable idempotency states.

---

### 28.2 Event Received Twice

Mitigation:

- event dedupe using backend event ID/version,
- projection version checks.

---

### 28.3 Event Missed Entirely

Mitigation:

- reconciliation job polls backend for non-terminal or stale executions,
- repairs projection.

---

### 28.4 Projection Corruption

Mitigation:

- rebuild projection from backend history or latest backend snapshot plus submission record.

---

### 28.5 Backend Temporarily Unavailable

Mitigation:

- surface dependency error on submit,
- retry event ingestion,
- use reconciliation after recovery.

---

## 29. SLO-Oriented Considerations

The implementation should define internal targets for:

- submit latency,
- validate latency,
- projection freshness,
- event processing delay,
- reconciliation completion time,
- list query latency.

Example categories to monitor:

| Capability | Example Objective |
|---|---|
| validate | low-latency synchronous response |
| submit | predictable p95 with strong idempotency |
| get execution | fast read from projection |
| projection freshness | near-real-time for active executions |
| reconciliation | bounded recovery for missed updates |

This RFC intentionally avoids freezing numerical SLOs too early, but the implementation must be instrumented to support them.

---

## 30. Migration from Legacy Flows

If existing scaffold mechanisms already exist, migration should follow these principles:

1. preserve public identifiers where feasible,
2. do not expose backend workflow details newly if old systems hid them,
3. front legacy flows behind the same normalized read model if necessary,
4. migrate one template family at a time,
5. use shadow-mode validation or projection before cutover,
6. maintain compatibility for portal/CLI consumers.

---

## 31. Suggested Milestones

### Milestone A — Contract Foundation

- schemas finalized
- compiler skeleton
- validation service
- fixture suite

### Milestone B — First Submit Path

- idempotency repo
- backend submit adapter
- minimal status projection
- get execution

### Milestone C — Event-Driven Read Model

- backend event consumer
- projection normalizer
- list endpoint/tool
- normalized events

### Milestone D — Approval and Cancellation

- approval visibility
- cancel flow
- rejected/expired semantics
- remediation enrichment

### Milestone E — Production Hardening

- replay tools
- dashboards
- alarms
- reconciliation jobs
- migration support

---

## 32. Implementation Recommendations

### 32.1 Prefer Deterministic Compilation

Compilation should be a pure or near-pure function over normalized inputs plus resolved template metadata.

### 32.2 Prefer Append-Friendly Event Processing

Projection logic should tolerate replays and out-of-order delivery safely.

### 32.3 Keep Public Projection Small and Safe

Do not turn the projection store into a dump of backend runtime internals.

### 32.4 Centralize Contract Mapping

Status, errors, and outputs should be normalized in one place each.

### 32.5 Build Reconciliation Early

Do not wait for production incidents to add projection repair and backend reconciliation.

---

## 33. Open Questions

1. Should projection updates be purely event-driven, or should submit path also synchronously seed richer initial state?
2. Should MCP retain a copy of compiled execution input for replay/debug, or only a plan hash plus request snapshot?
3. Should public normalized events be published directly from projection writes via outbox, or via a separate event derivation worker?
4. How much backend execution history should MCP retain versus only latest normalized state?
5. Should list/search support arbitrary metadata filters in v1, or a constrained indexed subset only?

---

## 34. Decision

This RFC proposes a reference implementation with:

- **MCP as public contract and normalization boundary**
- **Release Engine as execution authority**
- **separate submission and projection persistence**
- **adapter-isolated integrations**
- **event-driven status materialization with reconciliation**
- **phased rollout from validation to full production hardening**

This implementation plan preserves the architecture established across RFC-SCAFFOLD-001 through RFC-SCAFFOLD-008 while making delivery concrete and incremental.

---