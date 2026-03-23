# RFC-SCAFFOLD-013 — Template Capability and Policy Classification

---

## 1. Summary

This RFC defines the **capability classification** and **policy-relevant metadata model** for scaffold templates.

Its purpose is to make template behavior understandable and governable **before execution** without moving runtime policy authority out of the Release Engine.

Specifically, this RFC standardizes how templates declare:

- what they are capable of provisioning,
- what kinds of risk they introduce,
- what environments they are suitable for,
- whether they imply elevated approvals or review,
- and what rollout or visibility restrictions should apply.

This classification model supports:

- safer template review,
- clearer catalog behavior,
- better operator understanding,
- stronger pre-submit validation,
- and more predictable policy integration.

It does **not** make template metadata the final execution authority. Template classification is **advisory and governance-enabling**; **Release Engine remains authoritative** for execution-time approval, policy, and compliance decisions.

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
- **RFC-SCAFFOLD-012** — Template Authoring and Lifecycle Model

---

## 3. Problem Statement

Without a consistent classification model, template governance becomes fragmented:

- one team labels a template “high risk,” another does not,
- catalog visibility rules become ad hoc,
- review depth varies unpredictably,
- approval hints are buried in docs,
- environment restrictions are enforced inconsistently,
- operators cannot quickly assess what a template is allowed to do,
- and consumers may unknowingly select templates with very different operational or security consequences.

The platform needs a standard way to answer questions like:

- Does this template create externally exposed resources?
- Does it manipulate privileged identity or access?
- Is it safe for self-service in production?
- What approval intensity should reviewers expect?
- Should it be limited to certain tenants, regions, or operators?
- Does it have destructive or irreversible semantics?
- Does it handle regulated or sensitive data patterns?

These questions should be answerable from template metadata in a consistent, reviewable way.

---

## 4. Goals

This RFC aims to define:

1. a standard classification vocabulary for templates,
2. required capability and risk metadata,
3. how that metadata influences catalog visibility and review workflows,
4. how classification informs pre-submit validation and operator UX,
5. how classification maps into advisory approval hints for Release Engine integration,
6. how classification is reviewed and changed over time.

---

## 5. Non-Goals

This RFC does **not**:

- replace Release Engine policy enforcement,
- define every organization-specific policy rule,
- create a fully automated risk engine,
- guarantee that metadata alone captures all runtime risk,
- require one universal classification taxonomy for every enterprise forever.

This RFC defines a **platform baseline model** that can be extended carefully.

---

## 6. Design Principles

### 6.1 Classification must be explicit

Policy-relevant behavior should not be inferred only from documentation or author intent.

### 6.2 Classification is declarative metadata, not hidden code behavior

A reviewer should not need to read compile internals just to understand the broad risk shape of a template.

### 6.3 Classification must be stable enough for automation

If classification is too vague, it cannot support review routing, visibility controls, or validation.

### 6.4 Classification informs but does not authorize

A template may declare “requires elevated approval,” but final approval logic still belongs to Release Engine.

### 6.5 Conservative classification is preferred

When uncertain, classify toward greater scrutiny rather than less.

### 6.6 Capability and risk are related but distinct

A template’s **capabilities** describe what it can do. Its **risk class** describes how much governance scrutiny it should receive.

---

## 7. Definitions

### 7.1 Capability

A **capability** is a normalized statement about what kinds of resources, permissions, or operational behaviors a template can produce.

Examples:
- creates network-accessible service,
- provisions persistent data store,
- manages identity/permissions,
- performs destructive replacement,
- exposes public endpoint.

### 7.2 Risk class

A **risk class** is the platform’s coarse-grained governance tier for a template.

Examples:
- low,
- medium,
- high,
- critical.

### 7.3 Approval hint

An **approval hint** is non-authoritative metadata indicating the likely approval intensity or review path associated with the template.

### 7.4 Environment suitability

A declaration of where a template may be used, such as:
- dev/test only,
- non-prod and prod,
- restricted prod only,
- operator-mediated only.

### 7.5 Policy classification record

The structured metadata object attached to each published template version that captures capabilities, risk, restrictions, and review-relevant attributes.

---

## 8. Why Classification Exists

The classification system exists to support several platform behaviors consistently.

### 8.1 Review routing

High-risk templates should automatically require stronger review.

### 8.2 Catalog controls

Some templates should not be broadly discoverable or self-service eligible.

### 8.3 Validation and warnings

Requests can be rejected or warned earlier when they target templates not allowed for a given environment, tenant, or access level.

### 8.4 Operational readiness

Operators need quick context on whether a template is likely to involve sensitive actions, elevated approvals, or careful rollout.

### 8.5 Consumer clarity

Users should understand, at a high level, whether they are requesting a safe routine operation or a sensitive platform change.

---

## 9. Classification Model Overview

Each published template version must include a structured **policy classification record** with, at minimum, the following conceptual sections:

1. **resource capability classes**
2. **risk classification**
3. **approval and review hints**
4. **environment restrictions**
5. **visibility and self-service eligibility**
6. **destructive/irreversible behavior indicators**
7. **data sensitivity indicators**
8. **network and exposure indicators**
9. **identity/privilege indicators**
10. **operational sensitivity metadata**

---

## 10. Capability Taxonomy

The platform should use a bounded taxonomy rather than free-form prose wherever possible.

### 10.1 Core capability classes

A template may declare one or more of the following:

- `compute`
- `service`
- `database`
- `storage`
- `messaging`
- `network`
- `identity`
- `secrets`
- `observability`
- `delivery`
- `configuration`
- `integration`
- `tenant-foundation`
- `security-control`

These are intentionally broad and can be extended carefully later.

### 10.2 Behavioral capability flags

In addition to resource classes, templates should declare relevant behavioral flags such as:

- `createsPersistentState`
- `modifiesAccessControls`
- `createsExternalExposure`
- `handlesSensitiveConfiguration`
- `mayCauseServiceInterruption`
- `supportsDestructiveChange`
- `createsSharedInfrastructure`
- `requiresGlobalUniqueness`
- `createsCrossEnvironmentDependency`
- `createsCrossTenantImpact`
- `operatorMediatedActionRequired`

### 10.3 Example

A managed database template might classify as:

- capability classes:
    - `database`
    - `storage`
- behavioral flags:
    - `createsPersistentState`
    - `mayCauseServiceInterruption`
    - `handlesSensitiveConfiguration`

A public ingress template might classify as:

- capability classes:
    - `network`
    - `service`
- behavioral flags:
    - `createsExternalExposure`
    - `createsSharedInfrastructure`
    - `mayCauseServiceInterruption`

---

## 11. Risk Classification Model

### 11.1 Purpose of risk class

Risk class determines the **governance intensity** expected for a template version.

It may affect:

- review requirements,
- publication workflow,
- self-service eligibility,
- rollout strategy,
- warning surfaces,
- operational onboarding expectations.

### 11.2 Standard risk classes

The platform should define, at minimum:

| Risk Class | Meaning |
|---|---|
| `low` | Limited blast radius, routine, low-sensitivity changes |
| `medium` | Moderate operational impact or moderate governance concern |
| `high` | Significant security, operational, data, or production impact |
| `critical` | Highly sensitive, privileged, regulated, or potentially broad-impact actions |

### 11.3 Risk class guidance

#### Low
Typical characteristics:
- no public exposure,
- limited scope,
- low privilege,
- easily reversible,
- no regulated data implications,
- usually safe for broad self-service in non-prod.

#### Medium
Typical characteristics:
- common production-capable resources,
- moderate blast radius,
- some operational dependencies,
- possible downtime or persistence concerns,
- may need stronger review but still suitable for self-service in controlled cases.

#### High
Typical characteristics:
- production-sensitive infra,
- public exposure or access-control impact,
- persistent state with significant recovery implications,
- privileged integrations,
- destructive operations with meaningful service risk.

#### Critical
Typical characteristics:
- identity/privilege boundary changes,
- security controls with broad blast radius,
- highly regulated or sensitive data patterns,
- organization-wide shared infrastructure,
- changes that can materially affect many tenants, services, or compliance posture.

### 11.4 Risk class is not inferred solely from one flag

For example:
- a database is not automatically `high`,
- a network-related template is not always `critical`,
- a non-prod-only template may still be `high` if it changes identity or shared controls.

Risk class must be reviewed as a holistic classification.

---

## 12. Approval and Review Hints

### 12.1 Advisory approval metadata

Templates may include advisory fields such as:

- `approvalHint = none | standard | elevated | restricted`
- `reviewProfile = basic | platform | security | ops | multi-party`

These fields do **not** grant or deny execution.

They exist to:
- set expectations,
- help route review,
- support UI messaging,
- and inform mapping into Release Engine request metadata.

### 12.2 Mapping guidance

A typical mapping might be:

| Approval Hint | Expected Meaning |
|---|---|
| `none` | routine low-risk action, often auto-approvable in some contexts |
| `standard` | common approval path |
| `elevated` | stronger approval or additional reviewer class expected |
| `restricted` | limited use, often operator-mediated or tightly scoped |

### 12.3 Release Engine remains authoritative

If template metadata says `standard` but Release Engine policy decides stronger approval is required, Release Engine wins.

If template metadata says `elevated` and Release Engine policy would allow less, the platform may still choose to preserve the stricter path as a governance practice.

---

## 13. Environment Restriction Model

### 13.1 Why environment restrictions matter

Not every template should be valid in every environment.

Examples:
- some templates are safe only in dev/test,
- some are production-only and operator-mediated,
- some should be blocked in shared sandboxes,
- some should be disabled in regulated environments without extra controls.

### 13.2 Required environment metadata

Templates should declare:

- allowed environments,
- disallowed environments,
- any extra conditions for production use,
- whether self-service is permitted by environment.

### 13.3 Example categories

A template might declare:

- `allowedEnvironments = [dev, test, staging]`
- `prodEligibility = restricted`
- `selfServiceByEnvironment = { dev: true, test: true, staging: false, prod: false }`

### 13.4 Validation behavior

MCP should reject requests that target clearly disallowed environments based on template metadata before reaching Release Engine.

This is a **front-door validation responsibility**, not a replacement for downstream policy.

---

## 14. Visibility and Self-Service Eligibility

### 14.1 Visibility is separate from publication

A template can be published but still restricted from:
- broad catalog listing,
- self-service use,
- certain tenants,
- certain personas,
- certain environments.

### 14.2 Self-service eligibility categories

A template version should declare one of:

- `self-service`
- `self-service-with-warning`
- `operator-mediated`
- `hidden-internal`

### 14.3 Meaning

| Eligibility | Meaning |
|---|---|
| `self-service` | can be requested directly by authorized consumers |
| `self-service-with-warning` | direct use allowed, but UX should surface notable caution |
| `operator-mediated` | not intended for normal direct self-service |
| `hidden-internal` | internal-only; not shown in public consumer discovery |

### 14.4 Common uses

- Highly sensitive networking changes may be `operator-mediated`
- Experimental templates may be `hidden-internal`
- Common service templates may be `self-service`
- Templates with persistence or downtime risk may be `self-service-with-warning`

---

## 15. Destructive and Irreversible Semantics

Some templates or template versions imply operations that are difficult to reverse safely.

### 15.1 Required indicators

Templates should explicitly declare whether they can involve:

- destructive replacement,
- data loss risk,
- irreversible identifiers,
- forced recreation,
- shared resource mutation,
- downtime-prone transitions.

### 15.2 Why this matters

These indicators help:
- review depth,
- user messaging,
- migration planning,
- rollout caution,
- support readiness.

### 15.3 Example

A storage template with immutable naming constraints and replacement-on-change behavior should not look equivalent to a harmless metadata template.

---

## 16. Data Sensitivity Classification

### 16.1 Purpose

Some templates are more sensitive because of the kinds of data they store, reference, or expose.

### 16.2 Recommended sensitivity fields

Templates may declare:

- `dataSensitivity = none | internal | confidential | regulated`
- `storesCustomerData = true | false`
- `storesCredentialsOrSecrets = true | false`
- `supportsRegulatedWorkloads = true | false`

### 16.3 Notes

This classification is about **template suitability and governance**, not a guarantee of what every individual request will contain.

For example:
- a database template may be suitable for regulated workloads,
- but a specific request may still require stricter downstream controls.

---

## 17. Network and Exposure Classification

### 17.1 Exposure indicators

Templates should declare whether they may:

- create private-only endpoints,
- create internal shared endpoints,
- create internet-reachable endpoints,
- modify ingress/egress controls,
- bridge network boundaries.

### 17.2 Why this matters

Public exposure often raises:
- security review intensity,
- production rollout caution,
- incident response implications,
- approval expectations.

### 17.3 Example fields

- `networkExposure = none | private | internal-shared | public`
- `modifiesNetworkControls = true | false`
- `crossBoundaryConnectivity = true | false`

---

## 18. Identity and Privilege Classification

### 18.1 High-sensitivity area

Templates that create or modify identities, roles, permissions, trust relationships, or secret distribution paths are especially sensitive.

### 18.2 Required identity-related indicators

Templates should declare whether they:

- create principals,
- grant permissions,
- modify existing access controls,
- establish trust relationships,
- distribute sensitive credentials,
- operate with elevated platform privilege.

### 18.3 Governance implications

Identity-related templates will often default to:
- higher risk class,
- stronger review profile,
- reduced self-service eligibility,
- elevated audit expectations.

---

## 19. Operational Sensitivity Metadata

### 19.1 Purpose

Some templates are risky not because of security alone but because of operational consequences.

### 19.2 Suggested indicators

Templates may declare:

- expected blast radius (`local`, `service`, `shared`, `platform`)
- rollback complexity (`easy`, `moderate`, `hard`)
- downtime sensitivity (`none`, `possible`, `likely`)
- dependency sensitivity (`isolated`, `shared-control-plane`, `external-integration`)
- support tier (`standard`, `enhanced`, `specialist`)

### 19.3 Usage

This metadata supports:
- release planning,
- operator messaging,
- incident triage,
- change review expectations,
- onboarding decisions.

---

## 20. Recommended Classification Record Shape

An example conceptual model:

```yaml
classification:
  capabilityClasses:
    - database
    - storage

  behavioralFlags:
    - createsPersistentState
    - mayCauseServiceInterruption
    - handlesSensitiveConfiguration

  riskClass: high

  approval:
    hint: elevated
    reviewProfile: multi-party

  environment:
    allowed:
      - dev
      - test
      - staging
      - prod
    selfServiceByEnvironment:
      dev: true
      test: true
      staging: true
      prod: false

  visibility:
    catalog: visible
    selfServiceEligibility: self-service-with-warning

  destructiveBehavior:
    replacementPossible: true
    dataLossPossible: true
    irreversibleIdentifiers: true

  dataSensitivity:
    level: confidential
    storesCustomerData: true
    storesCredentialsOrSecrets: false
    supportsRegulatedWorkloads: true

  network:
    exposure: private
    modifiesNetworkControls: false
    crossBoundaryConnectivity: false

  identity:
    createsPrincipals: false
    grantsPermissions: false
    modifiesAccessControls: false
    distributesSensitiveCredentials: false

  operational:
    blastRadius: service
    rollbackComplexity: hard
    downtimeSensitivity: possible
    supportTier: enhanced
```

This is illustrative, not a mandated syntax.

---

## 21. How Classification Is Used

### 21.1 During authoring

Classification is authored alongside the template and reviewed with it.

### 21.2 During publication

Publication checks should ensure classification:
- is present,
- uses allowed vocabulary,
- is internally consistent,
- meets required review rules for its declared risk.

### 21.3 During catalog exposure

Catalog systems may use classification to:
- filter discoverability,
- display warnings,
- restrict self-service visibility,
- label template sensitivity.

### 21.4 During submit validation

MCP may use classification to:
- reject impossible environment selections,
- warn on sensitive templates,
- require explicit acknowledgment for certain risk classes,
- attach advisory metadata to Release Engine requests.

### 21.5 During operations and support

Operators may use classification to:
- triage tickets,
- prioritize review,
- route incidents,
- understand likely blast radius.

---

## 22. Validation Rules for Classification

### 22.1 Required completeness

Every published template must include required classification fields.

### 22.2 Vocabulary validation

Fields like risk class, exposure type, and self-service eligibility must use controlled enum values.

### 22.3 Consistency validation

The platform should detect obvious contradictions.

Examples:
- `networkExposure = public` with `createsExternalExposure = false`
- `selfServiceEligibility = self-service` combined with `approvalHint = restricted`
- `allowedEnvironments = [dev]` but `prod` marked self-service true
- `riskClass = low` while identity modification flags are true

These checks need not be perfect, but they should catch clearly inconsistent records.

### 22.4 Human review remains necessary

No consistency checker will replace reviewer judgment for nuanced cases.

---

## 23. Review and Governance Model

### 23.1 Classification changes are meaningful changes

Changing classification metadata can materially alter:
- catalog visibility,
- operator expectations,
- approval routing,
- consumer trust.

Therefore, classification changes must be reviewed, not treated as doc-only edits.

### 23.2 Stronger review for downward reclassification

Changes like:
- `high` to `medium`,
- `operator-mediated` to `self-service`,
- `public` exposure to `private`,
- `elevated` to `standard`

should receive particularly careful review, because they reduce scrutiny.

### 23.3 Risk review triggers

Templates should automatically trigger security/platform review when classification includes traits like:
- public exposure,
- identity modification,
- regulated data suitability,
- shared infrastructure mutation,
- platform-wide blast radius.

---

## 24. Interaction with Release Engine

### 24.1 Classification is upstream metadata

MCP may propagate selected classification metadata into Release Engine submission context.

Examples:
- template risk class,
- approval hint,
- operational sensitivity indicators,
- exposure classification.

### 24.2 Release Engine is authoritative

Release Engine may:
- agree with the advisory metadata,
- override it with stronger requirements,
- ignore non-authoritative hints if policy dictates.

### 24.3 No policy duplication requirement

MCP should not attempt to mirror all Release Engine rules from classification metadata.

Classification exists to improve governance and UX, not to create dual policy engines.

---

## 25. Interaction with Error and Status Models

Classification may affect presentation, but not the fundamental execution outcome model.

### 25.1 Error messaging

A disallowed environment or restricted visibility condition may produce structured validation errors before execution submission.

### 25.2 Status display

Status responses may include safe classification summaries such as:
- risk label,
- template sensitivity badge,
- operator-mediated marker.

### 25.3 Redaction and safety

Internal classification details that could reveal sensitive control topology should not necessarily be exposed to all consumers.

For example, highly specific internal control annotations may remain internal-only.

---

## 26. Example Template Profiles

### 26.1 Low-risk internal metadata template

- capability classes: `configuration`
- risk: `low`
- self-service: `self-service`
- exposure: `none`
- identity change: false
- data sensitivity: `internal`

### 26.2 Standard application service template

- capability classes: `service`, `compute`
- risk: `medium`
- self-service: `self-service-with-warning`
- exposure: `private` or `internal-shared`
- persistent state: false
- downtime sensitivity: `possible`

### 26.3 Managed production database template

- capability classes: `database`, `storage`
- risk: `high`
- self-service: `operator-mediated` or tightly restricted
- persistent state: true
- data sensitivity: `confidential` or `regulated`
- downtime sensitivity: `possible`
- rollback complexity: `hard`

### 26.4 IAM role or trust policy template

- capability classes: `identity`, `security-control`
- risk: often `high` or `critical`
- self-service: usually not broad self-service
- identity changes: true
- elevated privilege indicators: likely true
- review profile: `security` or `multi-party`

---

## 27. Anti-Patterns

### 27.1 Free-form prose without normalized fields
If classification is only described in narrative docs, automation and validation break down.

### 27.2 Under-classifying risky templates
Calling a public ingress or privilege-changing template “medium” just to simplify rollout is governance debt.

### 27.3 Overloading risk class to mean everything
Risk class is useful, but it should not replace specific flags like public exposure or identity change.

### 27.4 Hiding sensitive behavior in compile logic
If a template creates public endpoints or mutates permissions, that should be reflected explicitly in classification.

### 27.5 Treating approval hints as policy decisions
Hints are advisory. Runtime authority stays with Release Engine.

---

## 28. Minimum MVP vs Hardened Scope

### 28.1 MVP

For MVP, the platform should support at least:

- required `riskClass`,
- capability classes,
- self-service eligibility,
- environment restrictions,
- basic network/identity sensitivity indicators,
- validation of required fields,
- review routing based on declared risk.

### 28.2 Production-hardened scope

Later maturity should add:

- broader behavioral flag taxonomy,
- stronger consistency validation,
- richer operational sensitivity metadata,
- UI badges and warnings,
- automated policy-review routing,
- tenant-specific visibility overlays,
- audit history of classification changes.

---

## 29. Open Questions

1. Should the platform allow custom organization-specific capability extensions, or only core enums initially?
2. Should `critical` templates always be `operator-mediated`, or can there be carefully controlled exceptions?
3. How much of classification should be visible to end users versus only operators and reviewers?
4. Should certain behavioral flags automatically imply minimum risk classes?
5. Should classification be versioned only with templates, or also have independently trackable governance revisions?

---

## 30. Decision

This RFC adopts the following model:

- every published template version must declare a **structured policy classification record**,
- classification includes **capabilities, risk, restrictions, and sensitivity indicators**,
- classification is used for **governance, visibility, validation, and review routing**,
- classification is **advisory to execution policy**,
- **Release Engine remains authoritative** for runtime approval and compliance enforcement,
- classification changes are **meaningful governed changes**, not casual metadata edits.

This provides a consistent foundation for safer template review, clearer catalog semantics, and more predictable platform governance.

---
