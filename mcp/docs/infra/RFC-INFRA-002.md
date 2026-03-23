# RFC-INFRA-002: Infrastructure Intent Specification v1

---

## 1. Executive Summary

This RFC defines **Infrastructure Intent Specification v1**, a structured YAML contract that allows users and agentic clients to request infrastructure in a way that is:

- **expressive enough** for real platform use,
- **simple enough** for humans and agents to author,
- **safe enough** to preserve a golden-path operating model,
- **compatible with Release Engine** as it exists today.

The specification is intentionally **intent-based**, not provider-resource based. It is designed to be **validated and compiled outside Release Engine**, then submitted as a standard Release Engine job. That preserves Release Engine’s architecture, where modules are orchestration-only, side effects must go through `StepAPI`, and job submission remains a compact, idempotent API operation. ([raw.githubusercontent.com](https://raw.githubusercontent.com/gatblau/release-engine/main/docs/design/d02.md))

---

## 2. Background and Design Constraints

Release Engine is designed around a strict execution model:

- modules are compiled Go code,
- modules are **golden-path orchestration units**,
- modules must use `StepAPI` for all side effects,
- external interaction is done through connectors,
- the HTTP intake is idempotent and payload-bounded,
- approval is a first-class engine concern. ([raw.githubusercontent.com](https://raw.githubusercontent.com/gatblau/release-engine/main/docs/design/d02.md))

That means this spec must **not** become an alternative execution language. Instead, it must be:

1. **authored by humans/agents**,
2. **validated by a compiler/validator layer**,
3. **compiled into module params**,
4. **submitted to `POST /v1/jobs`** with a normal `path_key` and `params`. ([raw.githubusercontent.com](https://raw.githubusercontent.com/gatblau/release-engine/main/docs/design/d04.md))

---

## 3. Goals

### 3.1 Primary Goals

This spec must:

- let users describe infrastructure **by intent and constraints**,
- support simple and moderately complex requests,
- be machine-validatable,
- compile deterministically to `infra/provision-crossplane`,
- support approval, cost, and policy workflows,
- remain small enough for Release Engine intake constraints. ([raw.githubusercontent.com](https://raw.githubusercontent.com/gatblau/release-engine/main/docs/design/d04.md))

### 3.2 Non-Goals

This spec does **not** aim to:

- model arbitrary cloud resources,
- expose raw AWS/GCP/Azure primitives directly,
- embed imperative logic,
- replace Terraform, Crossplane, or Pulumi,
- move policy enforcement into the agent.

---

## 4. Design Principles

### 4.1 Intent over primitives

Users should say:

- “I need a production internal API platform with HA database and private ingress”

not:

- “Create 3 subnets, 2 route tables, 1 RDS instance, 1 ALB...”

### 4.2 Bounded expressiveness

The spec should be powerful enough to support real use cases, but constrained enough to prevent the platform from becoming a custom IaC language.

### 4.3 Deterministic compilation

The same valid spec plus the same capability catalog and policy set must compile to the same execution payload.

### 4.4 Explicit governance

Approval triggers, blast radius, residency constraints, and cost limits should be first-class, not inferred only from prose.

### 4.5 Versioned contract

Every request must be explicitly versioned so the platform can evolve without breaking older workflows.

---

## 5. Top-Level Resource Model

The v1 schema defines one top-level document kind:

```yaml
apiVersion: platform.gatblau.io/v1alpha1
kind: InfrastructureRequest
metadata: {}
spec: {}
```

### 5.1 Required Top-Level Fields

- `apiVersion`
- `kind`
- `metadata.name`
- `metadata.owner`
- `metadata.tenant`
- `spec.environment`
- `spec.workload`
- `spec.capabilities`

---

## 6. Full YAML Shape

## 6.1 Canonical Example

```yaml
apiVersion: platform.gatblau.io/v1alpha1
kind: InfrastructureRequest

metadata:
  name: analytics-prod-eu
  owner: team-data
  tenant: acme
  labels:
    system: analytics
    costCenter: fin-001
  annotations:
    requestor: alice@example.com

spec:
  environment: production

  workload:
    type: analytics-platform
    criticality: high
    exposure: internal
    lifecycle: long-lived

  capabilities:
    kubernetes:
      enabled: true
      profile: standard
      scale: medium
      addons:
        ingress: true
        externalSecrets: true
        serviceMesh: false

    database:
      enabled: true
      engine: postgres
      tier: highly-available
      size: medium
      storage: 500Gi

    objectStorage:
      enabled: true
      class: standard

    messaging:
      enabled: false

    networking:
      ingress: private
      egress: controlled
      connectivity:
        - corporate
        - shared-services

  requirements:
    availability: multi-az
    encryption: required
    backup:
      enabled: true
      retention: 35d
    observability:
      logs: standard
      metrics: standard
      tracing: optional
    residency: eu
    compliance:
      - gdpr

  constraints:
    approvedTemplatesOnly: true
    allowedRegions:
      - eu-west-1
      - eu-central-1
    maxMonthlyCost: 3000

  delivery:
    gitStrategy: pull-request
    approvalClass: high-blast-radius
    targetDate: 2026-03-31
    verifyCloud: true

  outputs:
    exportConnectionDetails: true
    notify:
      - platform-team@example.com
      - team-data@example.com
```

---

## 7. Field Semantics

## 7.1 `apiVersion`

String. Required.

For v1:

```yaml
apiVersion: platform.gatblau.io/v1alpha1
```

### Rules
- must exactly match a supported schema version,
- compiler rejects unsupported versions,
- future versions may add fields but must preserve compatibility where possible.

---

## 7.2 `kind`

String. Required.

For v1:

```yaml
kind: InfrastructureRequest
```

### Rules
- must exactly equal `InfrastructureRequest`.

---

## 7.3 `metadata`

Object. Required.

### Fields

#### `metadata.name`
Human-readable request identifier.

Rules:
- required,
- DNS-like slug preferred,
- max 63 chars recommended,
- regex:
  `^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`

#### `metadata.owner`
Logical owning team or group.

Rules:
- required,
- maps to namespace/team ownership,
- used in audit, approval, and compilation.

#### `metadata.tenant`
Release Engine tenant identifier.

Rules:
- required,
- must match submitting tenant context,
- compiler must reject tenant mismatch before submission.

#### `metadata.labels`
Optional key-value metadata.

Use for:
- cost center,
- application grouping,
- business unit.

#### `metadata.annotations`
Optional free-form metadata.

Use sparingly for:
- requestor identity,
- ticket references,
- planning notes.

---

## 7.4 `spec.environment`

Enum. Required.

Allowed values:

- `development`
- `test`
- `staging`
- `production`

### Semantics
Used to drive:
- template resolution,
- approval policy,
- default sizing,
- compliance policy,
- git destination/path conventions.

---

## 7.5 `spec.workload`

Object. Required.

### Fields

#### `type`
Required string or enum from capability catalog.

Examples:
- `web-api`
- `analytics-platform`
- `batch-processing`
- `internal-tooling`
- `data-pipeline`

#### `criticality`
Required enum:
- `low`
- `medium`
- `high`

#### `exposure`
Required enum:
- `internal`
- `partner`
- `public`

#### `lifecycle`
Optional enum:
- `ephemeral`
- `temporary`
- `long-lived`

### Semantics
This section describes **what the system is**, not how it is built.

---

## 7.6 `spec.capabilities`

Object. Required.

This is the heart of the request. Each sub-object is an **optional platform capability** with bounded parameters.

### General rule
A capability may either be:
- omitted,
- present with `enabled: false`,
- present with `enabled: true` and capability-specific settings.

---

# 8. Capability Definitions

## 8.1 `capabilities.kubernetes`

```yaml
kubernetes:
  enabled: true
  profile: standard
  scale: medium
  addons:
    ingress: true
    externalSecrets: true
    serviceMesh: false
```

### Fields

- `enabled`: boolean, required if object present
- `profile`: enum
    - `lite`
    - `standard`
    - `hardened`
- `scale`: enum
    - `small`
    - `medium`
    - `large`
- `addons.ingress`: boolean
- `addons.externalSecrets`: boolean
- `addons.serviceMesh`: boolean

### Semantics
Represents a managed Kubernetes capability, not raw cluster settings.

### Validation examples
- if `enabled = false`, no other fields allowed
- `serviceMesh = true` may require `profile != lite`
- `production` + `public` may require `profile = hardened`

---

## 8.2 `capabilities.database`

```yaml
database:
  enabled: true
  engine: postgres
  tier: highly-available
  size: medium
  storage: 500Gi
```

### Fields

- `enabled`: boolean
- `engine`: enum
    - `postgres`
    - `mysql`
    - `mongodb`
- `tier`: enum
    - `dev`
    - `standard`
    - `highly-available`
- `size`: enum
    - `small`
    - `medium`
    - `large`
- `storage`: quantity string

### Validation rules
- storage format: `^[0-9]+(Gi|Ti)$`
- `production` should not allow `tier = dev`
- `mongodb` support must depend on platform catalog
- unsupported `engine` values must fail validation

---

## 8.3 `capabilities.objectStorage`

```yaml
objectStorage:
  enabled: true
  class: standard
```

### Fields
- `enabled`: boolean
- `class`: enum
    - `standard`
    - `infrequent-access`
    - `archive`

### Validation
- archive may be incompatible with some workload types
- some environments may disallow archive as primary storage class

---

## 8.4 `capabilities.messaging`

```yaml
messaging:
  enabled: true
  type: queue
  tier: standard
```

### Fields
- `enabled`: boolean
- `type`: enum
    - `queue`
    - `stream`
- `tier`: enum
    - `dev`
    - `standard`
    - `highly-available`

### Notes
This stays capability-level; no topic count, broker count, shard definitions, etc. in v1.

---

## 8.5 `capabilities.networking`

```yaml
networking:
  ingress: private
  egress: controlled
  connectivity:
    - corporate
    - shared-services
```

### Fields
- `ingress`: enum
    - `none`
    - `private`
    - `public`
- `egress`: enum
    - `restricted`
    - `controlled`
    - `open`
- `connectivity`: array of enum/string catalog entries

### Semantics
Describes desired network posture.

### Validation
- `public` ingress with `criticality = high` likely requires approval escalation
- `open` egress may be disallowed by policy
- connectivity targets must be known to the platform catalog

---

# 9. Requirements Section

## 9.1 `spec.requirements`

Object. Optional but strongly recommended.

```yaml
requirements:
  availability: multi-az
  encryption: required
  backup:
    enabled: true
    retention: 35d
  observability:
    logs: standard
    metrics: standard
    tracing: optional
  residency: eu
  compliance:
    - gdpr
```

### Fields

#### `availability`
Enum:
- `single-zone`
- `multi-az`

#### `encryption`
Enum:
- `required`
- `platform-default`

#### `backup.enabled`
Boolean

#### `backup.retention`
Duration-like string, e.g.:
- `7d`
- `35d`
- `90d`

Regex:
`^[0-9]+d$`

#### `observability.logs`
Enum:
- `none`
- `standard`
- `enhanced`

#### `observability.metrics`
Enum:
- `none`
- `standard`
- `enhanced`

#### `observability.tracing`
Enum:
- `none`
- `optional`
- `required`

#### `residency`
Enum or catalog value, e.g.:
- `eu`
- `uk`
- `us`

#### `compliance`
Array of enum/catalog values, e.g.:
- `gdpr`
- `pci`
- `sox`

### Semantics
These are cross-cutting constraints that influence template selection and approval.

---

# 10. Constraints Section

## 10.1 `spec.constraints`

Object. Optional.

```yaml
constraints:
  approvedTemplatesOnly: true
  allowedRegions:
    - eu-west-1
    - eu-central-1
  maxMonthlyCost: 3000
```

### Fields

#### `approvedTemplatesOnly`
Boolean. Default `true`.

This should remain `true` for normal platform operation.

#### `allowedRegions`
Array of platform-approved region identifiers.

#### `maxMonthlyCost`
Integer or number.

### Compiler behavior
- if estimated cost exceeds limit, compilation fails or routes to approval workflow, depending on policy mode
- region selection must stay inside approved regions

---

# 11. Delivery Section

## 11.1 `spec.delivery`

Object. Optional.

```yaml
delivery:
  gitStrategy: pull-request
  approvalClass: high-blast-radius
  targetDate: 2026-03-31
  verifyCloud: true
```

### Fields

#### `gitStrategy`
Enum:
- `direct`
- `pull-request`

Maps to module `git_strategy`.

#### `approvalClass`
Enum:
- `auto`
- `low-blast-radius`
- `high-blast-radius`

This is a hint, not the final authority. Final approval behavior is computed by compiler + policy.

#### `targetDate`
Date string in `YYYY-MM-DD`.

#### `verifyCloud`
Boolean.

Maps to module `verify_cloud`.

---

# 12. Outputs Section

## 12.1 `spec.outputs`

Object. Optional.

```yaml
outputs:
  exportConnectionDetails: true
  notify:
    - platform-team@example.com
    - team-data@example.com
```

### Fields

#### `exportConnectionDetails`
Boolean.

Controls whether the platform should surface connection metadata, subject to secrets policy.

#### `notify`
Array of recipients.

### Note
Release Engine itself handles callbacks and outbox events; approval and callback events are already first-class outbox contracts in the engine design. ([raw.githubusercontent.com](https://raw.githubusercontent.com/gatblau/release-engine/main/docs/design/d04.md))

---

# 13. Formal Validation Rules

## 13.1 Structural Validation

The validator must enforce:

- correct `apiVersion`
- correct `kind`
- required top-level fields present
- known fields only, unless explicitly allowed by schema mode
- correct scalar types
- enum membership
- regex rules
- numeric and length bounds

---

## 13.2 Semantic Validation

The validator/compiler must enforce cross-field rules such as:

1. `production` + `database.tier = dev` → invalid
2. `public` ingress + `criticality = high` → requires high approval class
3. `backup.retention` provided while `backup.enabled = false` → invalid
4. `residency = eu` with only non-EU regions allowed → invalid
5. `enabled = false` with extra capability fields present → invalid
6. unsupported capability for workload type → invalid
7. estimated cost > `maxMonthlyCost` → policy failure or requires approval
8. tenant mismatch between metadata and caller context → invalid

---

## 13.3 Policy Outcome Categories

Validation should return one of:

- `allow`
- `allow_with_approval`
- `deny`

### Examples
- unsupported compliance profile → `deny`
- normal request → `allow`
- over-budget but waivable → `allow_with_approval`

---

# 14. Compilation Rules to `infra/provision-crossplane`

The compiled result must produce standard Release Engine job params.

Because Release Engine job intake expects `tenant_id`, `path_key`, `params`, `idempotency_key`, and optional callback/scheduling fields, the compiler should emit a compact payload suitable for `POST /v1/jobs`. ([raw.githubusercontent.com](https://raw.githubusercontent.com/gatblau/release-engine/main/docs/design/d04.md))

## 14.1 Target Job Envelope

```yaml
tenant_id: acme
path_key: golden-path/infra/provision-crossplane
idempotency_key: infrareq-analytics-prod-eu-v1
params: {...}
callback_url: https://example.internal/release-engine/callback
```

---

## 14.2 Core Mapping

### Top-level mapping

| Intent field | Compiled module param |
|---|---|
| `metadata.tenant` | job `tenant_id` |
| `spec.delivery.gitStrategy` | `git_strategy` |
| `spec.delivery.verifyCloud` | `verify_cloud` |
| derived template | `template_name` |
| derived composition | `composition_ref` |
| derived namespace | `namespace` |
| compiled settings | `parameters` |
| derived approval payload | `approvalContext` |

---

## 14.3 Example Compilation

### Input

```yaml
apiVersion: platform.gatblau.io/v1alpha1
kind: InfrastructureRequest
metadata:
  name: analytics-prod-eu
  owner: team-data
  tenant: acme
spec:
  environment: production
  workload:
    type: analytics-platform
    criticality: high
    exposure: internal
  capabilities:
    kubernetes:
      enabled: true
      profile: standard
      scale: medium
    database:
      enabled: true
      engine: postgres
      tier: highly-available
      storage: 500Gi
    objectStorage:
      enabled: true
      class: standard
    networking:
      ingress: private
      egress: controlled
  requirements:
    availability: multi-az
    encryption: required
    backup:
      enabled: true
      retention: 35d
    residency: eu
    compliance: [gdpr]
  delivery:
    gitStrategy: pull-request
    verifyCloud: true
```

### Compiled Plan

```yaml
kind: CompiledProvisioningPlan
version: v1

summary:
  requestName: analytics-prod-eu
  template: analytics-platform-prod
  blastRadius: high
  estimatedMonthlyCost: 2475

job:
  tenant_id: acme
  path_key: golden-path/infra/provision-crossplane
  idempotency_key: infrareq-analytics-prod-eu-v1

  params:
    template_name: analytics-platform-prod
    composition_ref: xrd.analytics.platform/v1
    namespace: team-data

    git_repo: acme/infra-live
    git_branch: main
    git_strategy: pull-request

    verify_cloud: true
    cloud_resource_type: composite

    approvalContext:
      required: true
      decisionBasis:
        policyOutcome: allow_with_approval
        reasonCodes:
          - high_blast_radius
      riskSummary:
        blastRadius: high
        estimatedCostBand: medium
      reviewContext:
        requestedBy: team-data
        targetScope: acme
        changeSummary: "analytics-prod-eu"
      suggestedApproverRoles:
        - techops-lead
      ttl:
        expiresAt: "2026-04-01T12:00:00Z"

    parameters:
      environment: production
      cluster_profile: standard
      cluster_scale: medium
      database_engine: postgres
      database_tier: highly-available
      database_storage: 500Gi
      object_storage_class: standard
      ingress_mode: private
      egress_mode: controlled
      availability: multi-az
      encryption: required
      backup_retention: 35d
      residency: eu
      compliance_tags:
        - gdpr
      owner: team-data
      request_name: analytics-prod-eu
```

---

# 15. Template Resolution Model

The compiler should resolve a request into a **vetted template** plus parameters.

## 15.1 Example Resolution Strategy

Inputs considered:

- `spec.environment`
- `spec.workload.type`
- requested capabilities
- criticality
- residency/compliance
- policy rules

### Example
A request for:

- `workload.type = analytics-platform`
- `environment = production`
- `kubernetes enabled`
- `database postgres HA`
- `objectStorage enabled`

may resolve to:

```yaml
template_name: analytics-platform-prod
composition_ref: xrd.analytics.platform/v1
```

## 15.2 Rule
The compiler may only resolve to templates/compositions in the approved platform catalog.

---

# 16. Blast Radius and Approval Derivation

Release Engine’s approval model already supports role allow-lists, self-approval restriction, tenant scope, optional budget authority, and multi-approver progression. ([raw.githubusercontent.com](https://raw.githubusercontent.com/gatblau/release-engine/main/docs/design/d04.md))

This spec should therefore derive approval metadata, not replace engine approval semantics.

## 16.1 Suggested Blast Radius Heuristics

### Low
- development or test
- no public ingress
- no HA database
- low criticality

### Medium
- staging
- internal production services
- moderate cost
- shared network connectivity

### High
- production
- public ingress
- HA data services
- regulated workload
- estimated cost above threshold

## 16.2 Compiler Output
Compiler should emit:

- `approvalContext` wrapper, containing:
  - `required` flag
  - `decisionBasis` (policy outcome and reason codes)
  - `riskSummary`
  - `reviewContext`
  - `suggestedApproverRoles`
  - `ttl`

The actual waiting step and decisions remain inside Release Engine. ([raw.githubusercontent.com](https://raw.githubusercontent.com/gatblau/release-engine/main/docs/design/d02.md))

---

# 17. Cost Estimation Contract

Cost in v1 should be **approximate**, not bill-grade.

## 17.1 Purpose
- early feedback,
- approval support,
- budget policy checks,
- request comparison.

## 17.2 Output Shape

```yaml
costEstimate:
  monthly: 2475
  currency: GBP
  confidence: low
  drivers:
    - kubernetes/standard/medium
    - postgres/ha/500Gi
    - objectStorage/standard
```

## 17.3 Rule
If cost cannot be reliably estimated:
- return warning,
- do not silently assume zero.

---

# 18. Error Model

## 18.1 Validation Errors

```yaml
status: invalid
errors:
  - field: spec.capabilities.database.tier
    code: UNSUPPORTED_VALUE
    message: "tier 'dev' is not permitted for environment 'production'"
```

## 18.2 Approval Required Example

```yaml
status: allow_with_approval
warnings:
  - field: spec.constraints.maxMonthlyCost
    code: ESTIMATED_COST_EXCEEDED
    message: "Estimated monthly cost 4200 exceeds maxMonthlyCost 3000"
```

## 18.3 Compilation Errors
Compilation fails when:
- no template mapping exists,
- policy blocks request,
- required derived fields cannot be resolved,
- resulting job payload would exceed engine constraints. Release Engine’s documented intake cap is 256 KB. ([raw.githubusercontent.com](https://raw.githubusercontent.com/gatblau/release-engine/main/docs/design/d04.md))

---

# 19. JSON Schema Recommendation

The user-facing format should remain YAML, but the authoritative validation contract should be JSON Schema.

## 19.1 Why JSON Schema
- easy to validate in MCP server,
- tool-friendly,
- editor support,
- can drive forms and docs,
- versionable.

## 19.2 Schema Mode Recommendation
Use:

```json
"additionalProperties": false
```

for most objects in v1.

Reason:
- limits agent hallucination,
- improves consistency,
- keeps compilation deterministic.

---

# 20. Suggested JSON Schema Skeleton

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "platform.gatblau.io/infrastructure-request-v1alpha1.schema.json",
  "type": "object",
  "required": ["apiVersion", "kind", "metadata", "spec"],
  "properties": {
    "apiVersion": {
      "const": "platform.gatblau.io/v1alpha1"
    },
    "kind": {
      "const": "InfrastructureRequest"
    },
    "metadata": {
      "type": "object",
      "required": ["name", "owner", "tenant"],
      "additionalProperties": false,
      "properties": {
        "name": {
          "type": "string",
          "pattern": "^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$"
        },
        "owner": { "type": "string", "minLength": 1 },
        "tenant": { "type": "string", "minLength": 1 },
        "labels": {
          "type": "object",
          "additionalProperties": { "type": "string" }
        },
        "annotations": {
          "type": "object",
          "additionalProperties": { "type": "string" }
        }
      }
    },
    "spec": {
      "type": "object"
    }
  },
  "additionalProperties": false
}
```

If you want, I can generate the **full JSON Schema** next.

---

# 21. MCP Tool Contract Implications

This spec supports the following MCP workflow cleanly:

1. `draft_infra_request`
2. `validate_infra_request`
3. `estimate_infra_cost`
4. `compile_infra_request`
5. `submit_infra_request`
6. `get_infra_request_status`

That fits the architecture proposed earlier and keeps Release Engine usage aligned with its intended API and module model. Release Engine remains responsible for durable job handling, approvals, and execution, while the MCP layer owns request shaping and compilation. ([raw.githubusercontent.com](https://raw.githubusercontent.com/gatblau/release-engine/main/docs/design/d02.md))

---

# 22. Compatibility with `infra/provision-crossplane`

This spec is intentionally designed to compile into the current module model you shared:

- render XR manifests from params,
- optional approval gate,
- commit manifests to infra repo,
- wait for Crossplane readiness,
- optional cloud verification,
- completion callback/outbox.

### Result
Minimal architectural disruption.

### Recommended enhancement
Allow compiled provenance metadata such as:
- request ID,
- intent spec version,
- compiler version,
- cost estimate,
- blast radius

to be included in:
- commit message,
- PR body,
- step outputs,
- callback payload metadata.

---

# 23. Open Design Decisions for v1 Finalization

Before implementation, these should be fixed:

1. **Capability catalog source of truth**  
   Static YAML? DB-backed catalog? Git-managed registry?

2. **Region vocabulary**  
   Platform regions or provider-native regions?

3. **Compliance vocabulary**  
   Closed enum or policy-managed catalog?

4. **Cost estimation source**  
   Static heuristics first, or integration with cloud pricing APIs?

5. **Failure mode for unsupported requests**  
   `deny` (hard reject) vs `allow_with_approval`

6. **Compiled plan persistence**  
   Store artifact separately, or only generate transiently before job submission?

---

# 24. Final Recommendation

Adopt this spec as **v1alpha1** with the following posture:

- **strict schema**
- **bounded capability catalog**
- **deterministic compilation**
- **approval-aware**
- **template-resolving, not resource-interpreting**

That gives you a practical middle ground:

- more flexible than raw template selection,
- much safer than arbitrary IaC,
- fully compatible with Release Engine’s current design constraints and HTTP contract. ([raw.githubusercontent.com](https://raw.githubusercontent.com/gatblau/release-engine/main/docs/design/d02.md))

---
