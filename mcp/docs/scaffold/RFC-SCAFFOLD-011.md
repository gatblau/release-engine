# RFC-SCAFFOLD-011: Delivery Roadmap and Milestone Plan

---

## 1. Summary

This RFC defines the **delivery roadmap** for the scaffolding platform across RFC-SCAFFOLD-001 through RFC-SCAFFOLD-010.

It translates the architecture into a practical rollout plan with:

- milestone sequencing,
- dependency ordering,
- MVP scope,
- production-hardening scope,
- migration checkpoints,
- test gates,
- operational readiness criteria,
- and exit criteria for each phase.

The goal is to avoid building the platform as a single large release. Instead, we deliver it in controlled stages that progressively add:

1. contract correctness,
2. template determinism,
3. execution integration,
4. status visibility,
5. security boundary clarity,
6. and operational reliability.

---

## 2. Related RFCs

This roadmap depends on:

- **RFC-SCAFFOLD-001** — Scaffolding Intent Model and MCP Contract
- **RFC-SCAFFOLD-002** — Template Catalog and Validation Rules
- **RFC-SCAFFOLD-003** — Compile-to-Release-Engine Mapping
- **RFC-SCAFFOLD-004** — Fixtures and Golden Tests
- **RFC-SCAFFOLD-005** — Approval Context and Release Engine Enforcement
- **RFC-SCAFFOLD-006** — Execution Status and Outcome Model
- **RFC-SCAFFOLD-007** — API Surface and Transport Bindings
- **RFC-SCAFFOLD-008** — Error Taxonomy and Remediation Model
- **RFC-SCAFFOLD-009** — Reference Implementation Plan
- **RFC-SCAFFOLD-010** — Security Boundary and Audit Integration

---

## 3. Problem Statement

The scaffold platform has enough architectural definition to build, but without a staged roadmap teams risk:

- implementing pieces in the wrong order,
- integrating with Release Engine before contracts are stable,
- exposing APIs before status/error models are mature,
- hard-coding template behavior without test fixtures,
- duplicating approval logic,
- or delaying operational hardening until after rollout.

We need a roadmap that answers:

- what comes first,
- what can be deferred,
- what constitutes MVP,
- what blocks production use,
- and how to move from prototype to hardened service.

---

## 4. Goals

This RFC aims to:

1. define implementation phases in dependency order,
2. identify MVP scope,
3. distinguish MVP from production-hardened scope,
4. define quality gates for each milestone,
5. establish rollout readiness criteria,
6. minimize rework from premature integration choices,
7. provide a migration path from proof-of-concept to production service.

---

## 5. Non-Goals

This RFC does **not**:

- redefine technical contracts already specified in prior RFCs,
- prescribe exact staffing or team structure,
- define sprint-level project plans,
- set organization-wide deadlines,
- replace backlog grooming or delivery management.

---

## 6. Delivery Principles

### 6.1 Build contract-first

The public request, status, and error models must stabilize before broad client adoption.

### 6.2 Prove determinism before scale

Template compilation must be deterministic and testable before production execution volume increases.

### 6.3 Integrate execution behind stable abstractions

Release Engine integration should be introduced behind adapters, not directly into API surfaces.

### 6.4 Ship observability before relying on it

Do not make the service operationally important before status, logging, correlation, and recovery tools exist.

### 6.5 Security boundaries must be explicit before production rollout

Approval/compliance ownership must be clear before handling sensitive or protected workflows.

### 6.6 Prefer thin vertical slices

Each milestone should produce an end-to-end usable slice, not just isolated components.

---

## 7. Phase Overview

The recommended roadmap is:

| Phase | Name | Primary Outcome |
|---|---|---|
| 0 | Foundations | RFC alignment, repo/module structure, basic domain skeleton |
| 1 | Contract MVP | Stable intent schema, validation, template catalog basics |
| 2 | Compile MVP | Deterministic compile path and golden fixtures |
| 3 | Submit MVP | Release Engine submission adapter and execution creation |
| 4 | Read/Status MVP | Public status/read APIs and normalized outcome projection |
| 5 | Security Alignment | Identity propagation, redaction, Release Engine authority alignment |
| 6 | Operational Hardening | Reconciliation, retries, observability, failure recovery |
| 7 | Controlled Production Rollout | Tenant/template-limited production use |
| 8 | General Availability Hardening | Broader scale, policy maturity, performance, support readiness |

---

## 8. MVP Definition

For this program, **MVP** means:

> A limited-production-capable scaffold service that can validate a request, compile it deterministically, submit it to Release Engine, and expose normalized status/outcome through stable APIs for a constrained set of templates and tenants.

MVP does **not** require:

- broad template coverage,
- arbitrary search/filter richness,
- advanced analytics,
- large-scale migration tooling,
- extensive multi-region support,
- fully self-service template authoring UX.

---

## 9. Phase 0 — Foundations

### 9.1 Objective

Create the delivery base so implementation can proceed without structural churn.

### 9.2 Scope

- establish repository/module boundaries,
- create domain package layout,
- define canonical schema/version directories,
- create initial persistence interfaces,
- define adapter interfaces for Release Engine,
- define event/projection package boundaries,
- establish coding standards and testing harness conventions.

### 9.3 Deliverables

- service/module skeleton,
- schema folder structure,
- interface definitions for compiler, validator, submitter, projector,
- baseline CI pipeline,
- test fixture directory conventions,
- RFC traceability matrix.

### 9.4 Exit criteria

- all major components have defined ownership boundaries,
- CI runs schema and unit test jobs,
- developers can implement features without changing top-level architecture.

---

## 10. Phase 1 — Contract MVP

### 10.1 Objective

Implement the scaffold request contract and validation behavior from RFC-001, RFC-002, RFC-007, and RFC-008.

### 10.2 Scope

- request schema implementation,
- template catalog loading/resolution,
- field validation,
- normalization logic,
- idempotency behavior,
- normalized error responses,
- submit/read API stubs.

### 10.3 Deliverables

- `submit` request validator,
- template registry/catalog resolver,
- normalized validation error mapper,
- basic API transport bindings,
- request persistence for accepted submissions,
- template/version selection logic.

### 10.4 Not yet required

- actual Release Engine submission,
- live execution polling,
- full projection worker,
- approval enforcement logic,
- sophisticated search/list features.

### 10.5 Exit criteria

- invalid requests fail deterministically,
- template selection is stable and testable,
- public API contract is usable by early clients,
- error model is normalized and documented.

---

## 11. Phase 2 — Compile MVP

### 11.1 Objective

Make request-to-plan compilation deterministic and regression-testable.

### 11.2 Scope

- compile pipeline implementation,
- canonical intermediate representation if defined,
- compile-time defaults,
- compile diagnostics,
- plan hashing/version stamping,
- golden fixture support.

### 11.3 Deliverables

- compiler implementation,
- fixture runner,
- golden output tests,
- compile artifact metadata,
- deterministic plan hash generation.

### 11.4 Why this phase matters

Before Release Engine integration, we must know that the same request and template version produce the same compiled output.

### 11.5 Exit criteria

- fixture coverage exists for representative templates,
- plan generation is deterministic,
- compile diffs are reviewable,
- regressions are caught in CI.

---

## 12. Phase 3 — Submit MVP

### 12.1 Objective

Connect the scaffold platform to Release Engine for execution creation.

### 12.2 Scope

- submission adapter,
- request-to-execution mapping,
- identity/tenant propagation,
- execution ID capture,
- idempotent submit semantics,
- submit-path persistence.

### 12.3 Deliverables

- Release Engine client adapter,
- execution creation integration tests,
- submission records with correlation identifiers,
- retry-safe submit path,
- failure classification for pre-submit vs submit-time errors.

### 12.4 Important constraints

- Release Engine remains execution authority,
- MCP does not implement approval decisions,
- service-to-service auth must be in place before real environment use.

### 12.5 Exit criteria

- accepted requests can create Release Engine executions,
- execution IDs are persisted and queryable,
- duplicate client retries do not create unsafe duplicate executions,
- submit errors are normalized correctly.

---

## 13. Phase 4 — Read/Status MVP

### 13.1 Objective

Expose stable requester-facing execution status and outcomes.

### 13.2 Scope

- status projection model,
- Release Engine state mapping,
- polling or event-driven update flow,
- `get`/`list` read APIs,
- normalized terminal outcomes,
- user-safe remediation messages.

### 13.3 Deliverables

- projection updater,
- state mapper aligned to RFC-006,
- read API implementation,
- status history basics if in scope,
- normalized outcome surfaces.

### 13.4 Exit criteria

- users can submit and later retrieve status,
- pending/running/completed/failed/cancelled states map consistently,
- requester-safe failures are exposed without leaking internal detail,
- stale status behavior is understood and documented.

---

## 14. Phase 5 — Security Alignment

### 14.1 Objective

Bring implementation into conformance with RFC-SCAFFOLD-010.

### 14.2 Scope

- front-door authentication/authorization enforcement,
- tenant resolution hardening,
- structured identity propagation,
- redaction rules,
- public/internal boundary reviews,
- audit correlation fields,
- removal of any shadow approval logic in MCP.

### 14.3 Deliverables

- authN/authZ middleware,
- propagated identity contract,
- redaction policy implementation,
- support tooling using correlation IDs,
- security review checklist completion.

### 14.4 Exit criteria

- MCP clearly owns request-boundary security,
- Release Engine remains authoritative for approval/compliance,
- no sensitive internal execution data leaks through public APIs,
- end-to-end traceability fields are present.

---

## 15. Phase 6 — Operational Hardening

### 15.1 Objective

Prepare the platform to survive real-world failures, drift, and support load.

### 15.2 Scope

- projection reconciliation jobs,
- retry/backoff policies,
- dead-letter handling,
- duplicate event tolerance,
- metrics and alerting,
- dashboards,
- runbooks,
- backfill/repair utilities,
- rate limiting and resource protection.

### 15.3 Deliverables

- reconciliation worker,
- projection repair tooling,
- observability dashboards,
- alert definitions,
- incident runbooks,
- SLO draft,
- support playbooks.

### 15.4 Exit criteria

- system can recover from missed events or transient backend failures,
- operators can investigate using documented tools,
- alerts exist for major failure modes,
- read models can be repaired without data loss.

---

## 16. Phase 7 — Controlled Production Rollout

### 16.1 Objective

Enable real production use with constrained blast radius.

### 16.2 Scope

- limited tenant rollout,
- limited template rollout,
- explicit allowlists,
- operational review gates,
- shadow/support monitoring,
- incident response drills.

### 16.3 Rollout strategy

Recommended rollout order:

1. internal/non-critical templates,
2. low-risk infrastructure types,
3. a small set of trusted tenants/projects,
4. workflows without complex approval dependencies,
5. progressively more sensitive templates.

### 16.4 Exit criteria

- at least one full vertical slice runs in production reliably,
- support teams can triage real incidents,
- failure/retry/reconciliation paths have been exercised,
- stakeholder confidence exists for broader exposure.

---

## 17. Phase 8 — General Availability Hardening

### 17.1 Objective

Move from controlled rollout to broad platform capability.

### 17.2 Scope

- scale/performance tuning,
- broader template catalog onboarding,
- pagination/search improvements,
- stronger tenancy controls if needed,
- quota/rate-limit maturity,
- capacity planning,
- multi-team onboarding materials,
- lifecycle/retention policy finalization.

### 17.3 Exit criteria

- platform operates at expected volume,
- onboarding of new templates is repeatable,
- documentation is complete for consumers and operators,
- support burden is sustainable,
- known architectural gaps are either resolved or accepted explicitly.

---

## 18. Dependency Ordering

Some capabilities should not be started before others are sufficiently mature.

### 18.1 Hard dependencies

| Capability | Depends on |
|---|---|
| Release Engine submission | request validation, compile determinism |
| Public status API | execution ID linkage, state mapping |
| Approval-related UX | Release Engine authority alignment |
| Broad rollout | observability, redaction, reconciliation |
| Sensitive template support | security alignment, audit correlation |
| General availability | operational hardening, support readiness |

### 18.2 Soft dependencies

| Capability | Benefits from |
|---|---|
| Search/list richness | stable projections |
| Template self-service onboarding | strong fixtures/golden tests |
| Advanced remediation messaging | mature error taxonomy + runtime mapping |
| Analytics/reporting | durable projection/event model |

---

## 19. Recommended Team Workstreams

The roadmap can proceed in parallel via a small number of coordinated tracks.

### 19.1 Contract and Compiler track
Owns:
- request schema,
- catalog,
- compiler,
- fixtures,
- deterministic outputs.

### 19.2 Integration and Runtime track
Owns:
- Release Engine adapter,
- submission path,
- status mapping,
- projection ingestion,
- reconciliation.

### 19.3 API and UX track
Owns:
- transport bindings,
- normalized responses,
- read/list surfaces,
- user-facing remediation,
- pagination/filtering.

### 19.4 Security and Operations track
Owns:
- authN/authZ,
- identity propagation,
- redaction,
- metrics/alerts,
- runbooks,
- incident readiness.

---

## 20. Testing Strategy by Phase

### 20.1 Phase 1
- schema validation tests,
- template resolution tests,
- API contract tests,
- normalized error mapping tests.

### 20.2 Phase 2
- compile golden tests,
- fixture coverage tests,
- plan hash determinism tests,
- backward compatibility checks for template version updates.

### 20.3 Phase 3
- adapter integration tests,
- idempotent submit tests,
- failure injection for submit-time errors,
- service auth tests.

### 20.4 Phase 4
- state mapping tests,
- projection update tests,
- eventual consistency behavior tests,
- terminal outcome normalization tests.

### 20.5 Phase 5
- authorization tests,
- tenancy isolation tests,
- redaction tests,
- correlation propagation tests.

### 20.6 Phase 6+
- reconciliation tests,
- replay/backfill tests,
- load tests,
- chaos/fault injection tests,
- operational drill exercises.

---

## 21. Milestone Gates

Each milestone should have explicit review gates.

### Gate A — Design completeness
- implementation scope matches RFC boundaries,
- unresolved questions are documented.

### Gate B — Test completeness
- required automated coverage exists,
- golden fixtures are updated intentionally.

### Gate C — Operational visibility
- logs/metrics/traces exist for the new path,
- failure modes are observable.

### Gate D — Security review
- data exposure reviewed,
- tenant behavior validated,
- service trust path reviewed.

### Gate E — Release readiness
- rollback plan exists,
- feature flags/allowlists are ready,
- support documentation is available.

---

## 22. MVP vs Production-Hardened Scope

| Capability | MVP | Production-Hardened |
|---|---|---|
| Submit API | Yes | Yes |
| Get status API | Yes | Yes |
| List/search basic | Optional/basic | Yes |
| Template catalog | Small curated set | Broad managed catalog |
| Compiler determinism | Yes | Yes |
| Golden fixtures | Core templates | Broad and policy-sensitive coverage |
| Release Engine integration | Yes | Yes |
| Approval display | Basic | Full polished UX |
| Reconciliation | Minimal/manual fallback acceptable | Automated and robust |
| Redaction/security boundary | Required | Required + reviewed continuously |
| Dashboards/alerts | Basic | Mature |
| Multi-tenant scale tooling | Limited | Mature |
| Advanced support workflows | Minimal | Strong |

---

## 23. Rollout Controls

The rollout should be feature-flagged and template-gated.

### 23.1 Recommended controls

- template allowlist,
- tenant/project allowlist,
- environment allowlist,
- channel allowlist,
- read API visibility flags,
- projection backend kill switch,
- submission adapter circuit breaker.

### 23.2 Why this matters

If execution integration or status mapping behaves unexpectedly, the platform must be able to:

- stop new submissions,
- preserve read access,
- restrict risky templates,
- and recover without full platform shutdown.

---

## 24. Migration from Prototype to Production

If a prototype already exists, migrate in structured steps.

### 24.1 Stabilize external contracts first
Do not expose prototype-specific payloads to consumers if they will be replaced.

### 24.2 Wrap direct backend calls in adapters
Any prototype logic that talks directly to Release Engine should be moved behind formal interfaces.

### 24.3 Replace ad hoc statuses with normalized statuses
Consumers should not depend on backend-native terminology.

### 24.4 Remove duplicated approval logic
Prototype convenience behavior must not become long-term architecture.

### 24.5 Add fixtures before broad template growth
Without fixtures, every new template increases regression risk.

---

## 25. Risks and Mitigations

### 25.1 Risk: premature Release Engine coupling
**Mitigation:** finish compiler and contract boundaries before broad integration.

### 25.2 Risk: unstable status semantics
**Mitigation:** normalize backend states before exposing broad read APIs.

### 25.3 Risk: template drift and regressions
**Mitigation:** invest early in golden fixtures and determinism tests.

### 25.4 Risk: duplicated security/compliance logic
**Mitigation:** enforce RFC-010 ownership during implementation review.

### 25.5 Risk: operational blind spots
**Mitigation:** do not expand rollout before reconciliation and dashboards exist.

### 25.6 Risk: support overload during rollout
**Mitigation:** use controlled onboarding, allowlists, and strong correlation tooling.

---

## 26. Success Metrics

The roadmap is succeeding if, phase by phase, we can show:

- high validation correctness,
- deterministic compile outputs,
- safe idempotent submission behavior,
- stable status retrieval,
- low ambiguity in error handling,
- clear security ownership,
- operational recovery from drift/failures,
- predictable onboarding of new templates.

Examples of useful metrics:

- validation rejection rate by category,
- compile determinism regression count,
- duplicate submission prevention rate,
- status projection lag,
- reconciliation repair volume,
- redaction/security defect count,
- template onboarding lead time.

---

## 27. Exit Criteria for Production Confidence

Before broad adoption, the platform should demonstrate:

1. stable external API semantics,
2. deterministic template compilation,
3. reliable Release Engine execution creation,
4. normalized status/outcome correctness,
5. security boundary compliance with RFC-010,
6. reconciliation and repair capability,
7. operator support readiness,
8. successful limited-production usage.

---

## 28. Open Questions

1. Should the first production rollout target one infrastructure type only, or a small curated set?
2. How much list/search capability is required before broader consumer adoption?
3. Should event-driven projection updates be mandatory before MVP, or can polling suffice initially?
4. What SLOs are required before general availability?
5. Should template onboarding be centralized initially, or open to multiple owning teams from the start?

---

## 29. Decision

This RFC adopts a phased delivery model:

- **contract first,**
- **deterministic compile second,**
- **execution integration third,**
- **public read/status next,**
- **security boundary hardening before broad rollout,**
- **operational resilience before GA.**

This sequencing minimizes architectural rework while providing useful end-to-end value early.

---