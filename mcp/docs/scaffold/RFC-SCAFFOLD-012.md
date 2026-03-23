# RFC-SCAFFOLD-012 — Template Authoring and Lifecycle Model

---

## 1. Summary

This RFC defines the **template authoring, versioning, review, publication, and deprecation model** for the scaffolding platform.

Templates are a core control point in the system. They determine:

- what kinds of infrastructure or platform resources may be requested,
- which inputs are allowed,
- how requests are validated,
- how requests compile into execution plans,
- and how changes evolve over time without breaking consumers.

This RFC establishes a lifecycle that makes templates:

- **deterministic**
- **reviewable**
- **testable**
- **versioned**
- **safe to evolve**
- **governable across teams**

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
- **RFC-SCAFFOLD-010** — Security Boundary and Audit Integration
- **RFC-SCAFFOLD-011** — Delivery Roadmap and Milestone Plan

---

## 3. Problem Statement

Without a clear template lifecycle, platforms usually drift into one or more failure modes:

- templates become ad hoc code paths instead of governed artifacts,
- authors change validation or defaults without compatibility review,
- consumers are broken by silent template updates,
- execution behavior diverges from documented behavior,
- ownership becomes unclear,
- risky templates reach production without security or operational review,
- old templates accumulate with no deprecation strategy.

Since templates effectively define **what the platform is allowed to create and how**, they need the same rigor as APIs and deployment workflows.

---

## 4. Goals

This RFC aims to define:

1. what a template is,
2. how templates are structured,
3. who owns them,
4. how they are versioned,
5. how they are reviewed and tested,
6. how they are published to the catalog,
7. how they are deprecated and retired,
8. how compatibility is managed over time.

---

## 5. Non-Goals

This RFC does **not**:

- define the exact DSL or file format syntax in full detail,
- replace domain-specific implementation docs for individual template types,
- define end-user portal UX,
- define all policy rules enforced by Release Engine,
- mandate a single org-wide staffing model for template ownership.

---

## 6. Definitions

### 6.1 Template

A **template** is a versioned scaffold definition that describes:

- supported input fields,
- validation rules,
- defaults,
- compile behavior,
- generated plan structure,
- metadata such as ownership, risk, and availability.

### 6.2 Template family

A **template family** is the stable logical product or resource type exposed to users.

Examples:

- `service`
- `postgres-database`
- `object-storage-bucket`
- `kafka-topic`
- `network-segment`

A family may have multiple versions.

### 6.3 Template version

A **template version** is a published immutable revision of a template family.

A version is what gets selected during request handling and compile.

### 6.4 Catalog

The **catalog** is the discoverable registry of published templates and associated metadata.

### 6.5 Author

A **template author** creates or updates the template definition and its tests.

### 6.6 Owner

A **template owner** is the team accountable for the template in production, including lifecycle, incident response, and consumer communication.

### 6.7 Publisher

A **publisher** is the controlled process that promotes reviewed template versions into the active catalog.

---

## 7. Design Principles

### 7.1 Templates are products, not snippets

A template must be treated as a maintained platform artifact, not a loose convenience definition.

### 7.2 Published versions are immutable

Once published, a version must not be changed in place.

### 7.3 Compatibility must be explicit

Breaking changes must result in a new version, not silent mutation.

### 7.4 Determinism is required

The same template version and normalized input must compile to the same plan.

### 7.5 Review depth should scale with risk

A low-risk internal metadata template may need lighter review than a privileged infrastructure template, but all published templates need governance.

### 7.6 Runtime policy is not replaced by authoring controls

Template review improves safety but does not replace Release Engine approval or runtime enforcement.

---

## 8. Template Model

At minimum, each template should define the following conceptual sections.

### 8.1 Identity metadata

- `family`
- `version`
- `displayName`
- `description`
- `ownerTeam`
- `contact`
- `documentationRef`

### 8.2 Availability metadata

- supported environments,
- tenant visibility rules,
- lifecycle state,
- publication date,
- deprecation date if applicable.

### 8.3 Input schema

Defines:

- fields,
- types,
- required vs optional status,
- allowed values,
- formats,
- nested structures,
- list/map behavior.

### 8.4 Validation rules

Defines:

- field constraints,
- cross-field validation,
- environment-specific rules,
- naming rules,
- policy-relevant compile-time checks.

### 8.5 Defaults and normalization

Defines:

- default values,
- derived values,
- normalization behavior,
- canonical formatting rules.

### 8.6 Compile rules

Defines:

- how validated input becomes a compiled plan,
- generated identifiers,
- implicit resources or dependencies,
- output bindings into the Release Engine plan.

### 8.7 Operational metadata

Defines:

- risk classification,
- expected approval class,
- observability tags,
- support escalation metadata,
- rollback/deprecation notes if relevant.

---

## 9. Required Template Metadata

Every published template version must carry required governance metadata.

| Field | Purpose |
|---|---|
| `family` | Stable template family identifier |
| `version` | Immutable published version identifier |
| `ownerTeam` | Accountable team |
| `reviewers` | Required review group or policy |
| `riskClass` | Risk tier for review and rollout |
| `lifecycleState` | Draft, review, published, deprecated, retired |
| `compatibilityLevel` | Backward-compatible or breaking |
| `documentationRef` | Human-readable docs reference |
| `testSuiteRef` | Link/reference to fixture and test set |
| `changeSummary` | What changed in this version |
| `releaseNotes` | Consumer-facing implications |

---

## 10. Lifecycle States

Templates move through explicit lifecycle states.

### 10.1 Draft

The template is under development and not available to consumers.

Properties:
- may change freely,
- not visible in production catalog,
- may fail compatibility expectations,
- must not be selected by normal requests.

### 10.2 Review

The template is frozen for approval and testing review.

Properties:
- changes require renewed review,
- fixture and validation results must be available,
- publication blockers are assessed.

### 10.3 Published

The template is active in the catalog and may be selected for request handling.

Properties:
- version is immutable,
- eligible for tenant/environment visibility according to metadata,
- supported by the owning team.

### 10.4 Deprecated

The template remains usable under defined conditions but should no longer be chosen for new usage by default.

Properties:
- catalog visibility may be reduced,
- warnings should be surfaced to operators or clients,
- successor version should be identified when possible.

### 10.5 Retired

The template is no longer available for new submissions.

Properties:
- existing historical records remain resolvable,
- new requests must be rejected,
- read/status of past executions must still remain interpretable.

---

## 11. Authoring Workflow

The recommended workflow is:

1. create or update template in source control,
2. add/update schema and compile logic,
3. add/update fixtures and golden tests,
4. run validation and deterministic compile tests,
5. complete review requirements,
6. publish immutable version,
7. expose in catalog according to allowlists/policy,
8. monitor usage and incidents,
9. deprecate or retire when superseded.

---

## 12. Versioning Model

### 12.1 Versioning goals

Versioning should let the platform:

- evolve templates safely,
- preserve deterministic historical behavior,
- communicate compatibility clearly,
- avoid breaking existing automation.

### 12.2 Recommended version shape

A semver-like model is recommended:

- **major** — breaking changes,
- **minor** — backward-compatible capability additions,
- **patch** — non-breaking fixes or metadata corrections.

Example:

- `service@1.0.0`
- `service@1.1.0`
- `service@2.0.0`

### 12.3 Immutability requirement

Once a version is published:

- its schema must not change,
- its compile behavior must not change,
- its defaults must not change,
- its validation semantics must not change.

Any such change requires a new version.

---

## 13. Compatibility Rules

### 13.1 Backward-compatible changes

Typically acceptable as minor or patch, depending on impact:

- adding optional fields,
- clarifying metadata or docs,
- widening allowed values where safe,
- improving non-semantic diagnostics,
- fixing internal metadata that does not affect request meaning or compile output.

### 13.2 Breaking changes

Require a new major version:

- removing fields,
- making optional fields required,
- changing field meaning,
- narrowing accepted values incompatibly,
- changing generated plan structure materially,
- changing defaults in a way that alters behavior for unchanged input,
- changing identifiers or naming behavior materially.

### 13.3 Gray-area changes

Some changes may appear compatible but still alter user expectations.

Examples:
- adding an optional field with a non-obvious implicit default,
- changing a derived naming convention,
- changing environment-specific behavior,
- adding a hidden dependency in compile output.

These should be reviewed conservatively and often treated as breaking if they affect runtime meaning.

---

## 14. Template Selection Rules

### 14.1 Explicit version selection preferred

Where practical, requests should either:

- specify an explicit template version, or
- resolve through a clearly documented version-selection policy.

### 14.2 Stable resolution policy required

If the platform supports “latest compatible” selection, the resolution rule must be explicit and testable.

### 14.3 No silent re-binding of historical requests

Once a request is accepted and compiled, its selected template version must be persisted and remain stable for that execution.

---

## 15. Review Model

### 15.1 Required review dimensions

Each publishable template version should be reviewed across relevant dimensions:

- **functional correctness**
- **schema/validation correctness**
- **compile determinism**
- **security**
- **operational supportability**
- **consumer impact / compatibility**

### 15.2 Risk-tiered review

Review depth should vary by risk class.

| Risk Class | Example | Typical Review Depth |
|---|---|---|
| Low | low-impact internal metadata resources | author + owner review |
| Medium | common application resources | owner + platform review |
| High | security-sensitive infra or privileged workflows | owner + platform + security + operations review |

### 15.3 Required sign-offs

At minimum, publication should require:

- template owner approval,
- platform/schema approval,
- test/fixture pass confirmation.

High-risk templates should also require:

- security review,
- operational readiness review,
- possibly domain-team approval.

---

## 16. Testing Requirements

No template version should be publishable without tests.

### 16.1 Required tests

- input validation tests,
- normalization tests,
- compile golden tests,
- compatibility checks against prior versions where applicable,
- negative tests for invalid input,
- metadata completeness checks.

### 16.2 Recommended tests

- environment matrix tests,
- edge-case naming tests,
- idempotent compile tests,
- policy-hint tests,
- deprecation behavior tests.

### 16.3 Golden fixture expectation

Every non-trivial template should include representative fixtures that cover:

- happy path,
- minimal valid input,
- common optional combinations,
- edge-case values,
- invalid combinations,
- migration-sensitive behaviors.

---

## 17. Publication Model

### 17.1 Publication must be controlled

Publishing a template version should happen via a controlled process, not manual mutation of the live catalog.

### 17.2 Publication outputs

Publishing should produce:

- immutable version record,
- catalog entry,
- metadata snapshot,
- test result snapshot,
- changelog/release notes,
- visibility configuration.

### 17.3 Promotion path

Recommended promotion path:

1. draft in source,
2. reviewed candidate,
3. published to non-prod catalog,
4. validated in staging/test flows,
5. promoted to production catalog,
6. enabled for target tenants/environments.

---

## 18. Catalog Behavior

### 18.1 Catalog must expose lifecycle state

Consumers and internal tools should be able to distinguish:

- draft,
- published,
- deprecated,
- retired.

### 18.2 Catalog visibility may be scoped

A published template version may still be restricted by:

- environment,
- tenant,
- platform region,
- operator allowlist,
- feature flags.

### 18.3 Catalog should preserve history

Historical versions should remain discoverable internally even if they are no longer available for new requests.

---

## 19. Ownership and Accountability

### 19.1 Every template needs a clear owner

No template should be publishable without an accountable owning team.

### 19.2 Owner responsibilities

The owning team is responsible for:

- correctness,
- documentation,
- support escalation,
- compatibility communication,
- deprecation planning,
- incident response for template-caused defects.

### 19.3 Shared platform responsibilities

The platform team is responsible for:

- authoring framework,
- catalog infrastructure,
- test harnesses,
- publication pipeline,
- governance rules,
- version resolution behavior.

---

## 20. Security and Compliance Considerations

### 20.1 Templates are policy-adjacent, not policy-authoritative

Templates may declare metadata relevant to policy, such as risk or approval class hints, but **Release Engine remains authoritative** for execution-time approval and compliance enforcement.

### 20.2 High-risk templates require stronger scrutiny

Templates that can provision privileged, externally exposed, destructive, or data-sensitive resources should require elevated review and stronger fixture coverage.

### 20.3 Sensitive defaults must be explicit

A template must not hide risky behavior in unobvious defaults.

Examples of risky hidden behavior:
- public network exposure,
- broad IAM permissions,
- destructive replacement semantics,
- production-environment assumptions.

### 20.4 Auditability of publication

The platform should retain an internal audit trail of:

- who authored changes,
- who approved publication,
- what tests ran,
- what version was published,
- when visibility changed.

---

## 21. Deprecation Model

### 21.1 Why deprecate

Deprecation exists to:

- guide consumers off old behavior,
- reduce maintenance burden,
- remove unsafe or outdated definitions,
- introduce improved versions safely.

### 21.2 Deprecation requirements

When a template version is deprecated, the platform should record:

- deprecation date,
- reason,
- recommended replacement,
- support end date,
- submission behavior after support end.

### 21.3 Deprecation behavior

Deprecation may include:
- warning in catalog,
- warning on submit,
- hidden from default discovery,
- deny new usage after a cutoff date,
- continued support for historical read/status.

### 21.4 Deprecation should be staged

Recommended progression:

1. **announce deprecation**
2. **warn on new usage**
3. **restrict default selection**
4. **deny new submissions**
5. **retire version**

---

## 22. Retirement Model

### 22.1 Retirement scope

Retired versions are unavailable for new submissions, but historical references must remain interpretable.

### 22.2 Historical stability requirement

The system must still be able to explain:
- which version was used,
- what inputs were accepted,
- how compile behavior was defined at the time.

This is important for:
- troubleshooting,
- audits,
- incident reviews,
- historical replay analysis.

---

## 23. Change Management

### 23.1 Every published change needs a summary

Each new version must include a concise change summary:
- what changed,
- whether it is breaking,
- why it changed,
- who is affected,
- what migration is recommended.

### 23.2 Consumer communication expectations

For significant template changes, especially breaking ones, consumers should receive:
- release notes,
- migration guidance,
- deprecation timelines,
- examples if needed.

### 23.3 Migrations should be documented

If a major version supersedes an older one, the owner should document:
- field mappings,
- changed defaults,
- behavior differences,
- common migration pitfalls.

---

## 24. Recommended Repository Structure

A logical structure might look like:

```text
templates/
  service/
    1.0.0/
      template.yaml
      schema.yaml
      compile/
      fixtures/
      docs.md
      metadata.yaml
    1.1.0/
      ...
  postgres-database/
    2.0.0/
      ...
```

Supporting systems may also keep:
- generated catalog artifacts,
- release notes,
- compatibility manifests,
- publication records.

The exact structure may vary, but the principles should remain:
- family grouping,
- version immutability,
- co-located tests and metadata.

---

## 25. Guardrails and Anti-Patterns

### 25.1 Anti-pattern: mutable published template
Changing a published version in place destroys historical determinism.

### 25.2 Anti-pattern: hidden breaking change
Changing defaults or compile behavior without versioning is effectively an undocumented API break.

### 25.3 Anti-pattern: ownerless template
If no team owns a template, support and deprecation become impossible.

### 25.4 Anti-pattern: shipping without fixtures
Without fixture coverage, template growth becomes regression-prone and unsafe.

### 25.5 Anti-pattern: encoding runtime authorization in template alone
Template metadata can inform review, but execution-time authorization must still be enforced by Release Engine and surrounding platform controls.

### 25.6 Anti-pattern: immediate hard retirement without warning
Consumers need a staged path off deprecated templates unless there is an urgent security issue.

---

## 26. Operational Recommendations

### 26.1 Keep template docs near template source
This reduces drift between implementation and documentation.

### 26.2 Require release notes at publish time
If a change matters enough to publish, it matters enough to describe.

### 26.3 Track usage by template version
You cannot deprecate responsibly if you do not know who still depends on a version.

### 26.4 Prefer additive evolution where possible
Minor-version growth is usually easier for consumers than repeated breaking changes.

### 26.5 Use canary visibility for risky templates
A newly published template version can be production-published but initially visible only to a controlled set of tenants or operators.

---

## 27. Implementation Guidance

### 27.1 Minimum viable lifecycle support

For MVP, the platform should support at least:

- draft vs published distinction,
- immutable published versions,
- required ownership metadata,
- fixture-backed publication checks,
- deprecation marker support,
- catalog lookup by family/version.

### 27.2 Production-hardened lifecycle support

Later maturity should add:

- richer compatibility manifests,
- staged promotion workflows,
- automated deprecation warnings,
- usage-based retirement gating,
- audit-ready publication history,
- policy-based review workflows by risk tier.

---

## 28. Open Questions

1. Should template version selection always be explicit, or may some clients use a policy like “latest supported minor”?
2. How much compile logic should live declaratively versus in reviewed code extensions?
3. Should publication be centralized through a platform team initially, or delegated to domain teams with policy controls?
4. What minimum historical retention is required for retired template metadata and fixtures?
5. Should some high-risk template families require mandatory staging soak time before production publication?

---

## 29. Decision

This RFC adopts the following model:

- templates are **versioned, governed platform artifacts**,
- published versions are **immutable**,
- breaking changes require **new versions**,
- every template must have **clear ownership**,
- publication requires **review and tests**,
- deprecation and retirement must be **explicit and staged**,
- historical template behavior must remain **auditable and interpretable**.

This ensures templates can evolve safely without sacrificing determinism, supportability, or consumer trust.

---