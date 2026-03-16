# Infra Module — Design Overview

## Purpose

The Infra module translates a canonical infrastructure request into a deterministic Crossplane manifest.  
Its role is to enforce platform policy, validate request intent, and compose capability-specific sections into a single renderable output.

This document describes **architecture and design intent only**. Implementation details and code-level specifics belong in source files and tests.

---

## Design Goals

1. **Deterministic rendering**  
   The same input must always produce the same manifest structure.

2. **Capability composition**  
   Infrastructure features are modeled as independent capabilities that can be combined safely.

3. **Catalogue-governed behavior**  
   Template constraints and defaults are defined declaratively in catalogue definitions.

4. **Validation-first flow**  
   Invalid requests fail early with actionable errors.

5. **Policy by design**  
   Security, compliance, and operational guardrails are enforced as first-class behavior.

6. **Testable architecture**  
   Fragments, catalog rules, and orchestration can be verified in isolation and in composition.

---

## Architectural Model

The module follows a pipeline:

1. **Input ingestion** from module parameters into canonical provision parameters.
2. **Catalog resolution** by template name.
3. **Default application** from catalog and policy baselines.
4. **Constraint validation** (global, catalog-level, cross-capability).
5. **Capability rendering** via fragment composition.
6. **Manifest assembly** into a Crossplane XR shape.
7. **Context publication** for downstream steps.

This model keeps business policy centralized while allowing capabilities to evolve independently.

---

## Capability Fragment Strategy

Each capability is represented as a fragment with clear responsibilities:

- decide if it applies for the current request,
- validate its own parameter domain,
- contribute its section to the final manifest.

This enables additive growth of infrastructure capabilities without creating monolithic conditional logic.

Typical capability families:

- Compute (e.g., Kubernetes, VM)
- Data (database, object/block/file storage, cache)
- Network & edge (VPC, load balancing, DNS, CDN)
- Platform services (messaging, identity, secrets, observability)
- Always-on policy sections (tags/compliance baseline)

---

## Catalog Design

Catalog definitions are the declarative contract for template behavior. They provide:

- template identity and composition reference,
- required/optional/forbidden capabilities,
- allowed operational dimensions (environment, profile, availability, residency),
- default values for omitted fields.

### Naming and Stability

Catalog names are treated as stable identifiers and must be centralized in a single source of truth so tests and module behavior do not drift when definitions are renamed.

---

## Validation Layers

Validation is intentionally multi-layered:

1. **Global validity** — required fields, baseline semantics.
2. **Catalogue constraints** — template-specific guardrails.
3. **Cross-capability rules** — mutual exclusions and dependencies.
4. **Fragment validation** — capability-local correctness.

This layering gives better error locality and clearer remediation guidance.

---

## Cross-Cutting Guardrails

Examples of invariant policy patterns the design enforces:

- Mutually exclusive compute modes when platform policy requires it.
- Dependency rules (e.g., one capability requiring another).
- Availability tier expectations (e.g., observability/backup/DR requirements).
- Data-classification-driven controls.
- Forbidden capabilities per catalog.

The intent is to prevent invalid infrastructure combinations before manifest generation.

---

## Determinism and Reproducibility

Rendered output must be stable across runs.  
Determinism is achieved through fixed orchestration order, explicit defaults, and stable map/key handling during serialization.

This supports reliable testing, consistent diffs, and predictable automation behaviour.

---

## Testing Strategy (Design-Level)

The design assumes coverage at three levels:

1. **Fragment tests** for applicability, validation, and section rendering.
2. **Engine orchestration tests** for catalog application, validation ordering, and combined rendering.
3. **Module tests** for end-to-end execution behavior and context publication.

Test suites should assert both success paths and policy failures, including catalog name consistency.

---

## Evolution Guidelines

When extending the module:

- prefer adding or extending fragments over embedding branching logic in orchestrators,
- keep catalogue definitions as the policy source for template-level behaviour,
- centralise identifiers used across runtime and tests,
- preserve deterministic output guarantees,
- update tests at fragment, engine, and module levels together.

---

## Scope Boundaries

This design document defines **what the module is and how it is intended to behave**.  
It does not prescribe exact implementation syntax, concrete structs, or code snippets.

Operational runbooks, API reference details, and implementation examples should live in their respective documentation artefacts.
