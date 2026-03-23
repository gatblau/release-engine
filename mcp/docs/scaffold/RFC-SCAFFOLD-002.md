# RFC-SCAFFOLD-002 — Template Catalog and Validation Rules

---

## 1. Summary

This RFC defines the **template catalog model** and **validation rules** for service scaffolding requests handled through MCP and executed by the Release Engine module:

```text
scaffolding/create-service
```

It standardizes:

- how service templates are described,
- how template parameters are declared and validated,
- how defaults and constraints are represented,
- how tenant / owner / org / visibility restrictions are encoded,
- and how validation should be applied consistently before compilation and submission.

This RFC builds directly on **RFC-SCAFFOLD-001**, which defined the intent model and MCP contract.

---

## 2. Problem Statement

The `ServiceScaffoldRequest` intent model includes:

- `spec.template`
- `spec.parameters`
- `spec.owner`
- `spec.org`
- `spec.visibility`
- related service, repo, and catalog metadata

However, that intent model requires a **catalog of platform-supported templates** to answer questions such as:

- Which templates are available?
- What parameters does each template accept?
- Which parameters are required?
- What are the parameter types and allowed values?
- Which owners or tenants may use a template?
- Which orgs and visibilities are permitted?
- What defaults should be applied?
- What service and catalog metadata are required by a template?

Without a formal catalog, template validation becomes:

- hard-coded,
- duplicated across services,
- difficult to evolve,
- difficult to test,
- and difficult for agents/UIs to introspect.

This RFC solves that by introducing a **declarative template catalog**.

---

## 3. Goals

### 3.1 Primary Goals

This RFC aims to:

1. define a **canonical template catalog schema**,
2. support **declarative validation** of template-specific parameters,
3. support **defaults and constraints** at template level,
4. allow tenant-aware and owner-aware template visibility,
5. make templates discoverable to MCP tools,
6. support deterministic normalization and compilation.

### 3.2 Non-Goals

This RFC does **not** define:

- the final compiled Release Engine param mapping,
- the runtime implementation of template rendering,
- the connector contracts,
- the persistence model for the catalog,
- or the full approval workflow model.

Those belong to later RFCs.

---

## 4. Design Principles

### 4.1 Declarative Over Hard-Coded

Templates and validation rules should live in data, not scattered through imperative code.

### 4.2 Introspectable by MCP

MCP tools should be able to list templates, explain parameters, and validate requests from catalog metadata alone plus platform policy.

### 4.3 Deterministic Defaults

Defaults must be explicit and consistently applied.

### 4.4 Layered Validation

Validation should combine:

- schema validation,
- template validation,
- policy validation,
- and environmental/platform validation.

### 4.5 Safe Extensibility

Different templates may support different parameter sets without breaking the common request model.

---

## 5. Scope

This RFC covers:

- template metadata,
- parameter schema declaration,
- defaulting rules,
- template eligibility constraints,
- validation rule ordering,
- warning vs error behavior.

It applies to service templates such as:

- `go-grpc`
- `node-express`
- `java-spring`

and future templates exposed through scaffolding MCP tools.

---

## 6. Template Catalog Overview

The template catalog is a platform-managed collection of template definitions.

Each template definition describes:

- template identity,
- display metadata,
- lifecycle state,
- input constraints,
- parameter schema,
- defaults,
- policy restrictions,
- metadata derivation hints,
- compilation metadata.

The catalog is consumed by the MCP scaffolding domain during:

- `list_service_templates`
- `get_service_template`
- `validate_service_scaffold_request`
- `compile_service_scaffold_request`

---

## 7. Catalog Resource Shape

A catalog may be represented as a single YAML document containing many templates, or as multiple template files loaded into one in-memory registry.

A conceptual top-level shape:

```yaml
apiVersion: platform.gatblau.io/v1alpha1
kind: ServiceTemplateCatalog
metadata:
  name: base-service-templates
spec:
  templates:
    - id: go-grpc
      version: 1.0.0
      displayName: Go gRPC Service
      status: active
      description: Scaffold a Go-based gRPC microservice with CI and catalog metadata
      owners:
        - platform-engineering
      allowedTenants:
        - acme
      allowedOrgs:
        - acme-platform
      allowedVisibilities:
        - internal
        - private
      defaults:
        visibility: internal
        repo:
          defaultBranch: main
      parameterSchema:
        type: object
        properties:
          goVersion:
            type: string
            enum: ["1.23", "1.24"]
            default: "1.24"
          enableDocker:
            type: boolean
            default: true
          enableOpenAPI:
            type: boolean
            default: false
        required:
          - goVersion
```

This RFC does not require this exact top-level storage form, but it **does** define the expected logical contents.

---

## 8. Template Definition Schema

Each template in the catalog should include the following sections.

---

## 8.1 Identity Fields

### Required

- `id`
- `version`
- `displayName`
- `status`

### Recommended

- `description`
- `tags`
- `maintainers`

### Example

```yaml
id: go-grpc
version: 1.0.0
displayName: Go gRPC Service
status: active
description: Scaffold a Go gRPC service with standard CI and service catalog registration
tags:
  - go
  - grpc
  - backend
maintainers:
  - team-platform
```

### Semantics

#### `id`
Stable template identifier used in `spec.template`.

#### `version`
Template definition version, not necessarily identical to generated starter code version.

#### `displayName`
Human-friendly name for UIs and agents.

#### `status`
Lifecycle state of the template.

Allowed values:
- `active`
- `deprecated`
- `disabled`
- `preview`

Validation behavior:
- `active` → valid for use
- `preview` → valid, may emit warnings
- `deprecated` → valid, should emit warnings
- `disabled` → invalid for new requests

---

## 8.2 Visibility and Eligibility Constraints

Templates may constrain who can use them and where.

### Recommended fields

- `allowedTenants`
- `deniedTenants`
- `allowedOwners`
- `deniedOwners`
- `allowedOrgs`
- `allowedVisibilities`

### Example

```yaml
allowedTenants:
  - acme
allowedOwners:
  - team-payments
  - team-orders
allowedOrgs:
  - acme-platform
allowedVisibilities:
  - internal
  - private
```

### Validation semantics

- if `allowedTenants` is set, tenant must be included
- if `deniedTenants` contains the tenant, request is invalid
- if `allowedOwners` is set, `spec.owner` must be included
- if `allowedOrgs` is set, `spec.org` must be included
- if `allowedVisibilities` is set, requested/effective visibility must be included

When both allow and deny lists exist, **deny wins**.

---

## 8.3 Template Defaults

Templates may define defaults for request fields and parameters.

### Example

```yaml
defaults:
  visibility: internal
  service:
    lifecycle: experimental
    tier: backend
  repo:
    defaultBranch: main
  catalog:
    type: service
    lifecycle: experimental
  parameters:
    goVersion: "1.24"
    enableDocker: true
    enableOpenAPI: false
```

### Defaulting order

Defaults should be applied in this order:

1. platform/global defaults
2. tenant defaults
3. template defaults
4. caller-provided explicit values

Caller-provided values always win unless a policy explicitly forbids them.

---

## 8.4 Parameter Schema

Each template defines the schema of `spec.parameters`.

A JSON-Schema-like subset is recommended for portability and familiarity.

### Recommended supported keywords

- `type`
- `properties`
- `required`
- `enum`
- `default`
- `minimum`
- `maximum`
- `minLength`
- `maxLength`
- `pattern`
- `items`
- `additionalProperties`
- `description`

### Example

```yaml
parameterSchema:
  type: object
  additionalProperties: false
  required:
    - goVersion
  properties:
    goVersion:
      type: string
      description: Go toolchain version
      enum:
        - "1.23"
        - "1.24"
      default: "1.24"
    enableDocker:
      type: boolean
      description: Include Dockerfile and docker build CI
      default: true
    enableOpenAPI:
      type: boolean
      description: Generate OpenAPI bridge assets
      default: false
```

### Validation behavior

- unknown parameter keys are rejected when `additionalProperties: false`
- missing required parameters are invalid unless a default fills them
- values must match declared type and constraints
- defaulted parameters become part of the normalized request

---

## 8.5 Service Metadata Requirements

Templates may require or default parts of `spec.service`.

### Example

```yaml
serviceRules:
  requireName: false
  requireDescription: true
  allowedTiers:
    - backend
    - worker
  defaultTier: backend
  allowedLifecycles:
    - experimental
    - production
  defaultLifecycle: experimental
```

### Semantics

- `requireName: false` means service name may be derived
- `requireDescription: true` means request must include `spec.service.description`
- `allowedTiers` constrains `spec.service.tier`
- `defaultTier` applies if omitted
- lifecycle constraints work the same way

---

## 8.6 Repo Metadata Rules

Templates may constrain repository metadata.

### Example

```yaml
repoRules:
  defaultBranch:
    value: main
    mutable: false
  topics:
    minItems: 0
    maxItems: 10
    pattern: "^[a-z0-9-]+$"
  requireDescription: false
```

### Semantics

- fixed branch policies can be declared
- topic format and count may be enforced
- some repo metadata may be optional informational hints only

---

## 8.7 Catalog Metadata Rules

Templates may require service catalog metadata for registration.

### Example

```yaml
catalogRules:
  requiredFields:
    - system
    - domain
  defaults:
    type: service
    lifecycle: experimental
  allowedTypes:
    - service
    - library
```

### Validation semantics

- required catalog fields must be present after defaulting
- defaults may be injected
- invalid catalog types are rejected

---

## 8.8 Policy and Approval Hints

Templates may carry signals used by policy logic.

### Example

```yaml
policyHints:
  approval:
    publicVisibilityRequiresApproval: true
    productionLifecycleRequiresApproval: false
  riskLevel: medium
  dataClassification: internal
```

These fields do not directly execute policy decisions on their own, but they provide structured inputs for policy evaluation.

---

## 8.9 Compilation Hints

Templates may include metadata used later by the compiler.

### Example

```yaml
compileHints:
  moduleKey: scaffolding/create-service
  contractVersion: scaffolding-create-service/v1
  parameterPassThrough: true
  derivedTopics:
    - from: spec.template
    - from: spec.service.tier
```

These hints help keep compilation deterministic and template-aware.

---

## 9. Recommended Full Template Example

```yaml
id: go-grpc
version: 1.0.0
displayName: Go gRPC Service
status: active
description: Scaffold a Go gRPC service with CI, Docker, and service catalog metadata
tags:
  - go
  - grpc
  - backend
maintainers:
  - team-platform

allowedTenants:
  - acme

allowedOrgs:
  - acme-platform

allowedVisibilities:
  - internal
  - private

defaults:
  visibility: internal
  service:
    tier: backend
    lifecycle: experimental
  repo:
    defaultBranch: main
  catalog:
    type: service
    lifecycle: experimental
  parameters:
    goVersion: "1.24"
    enableDocker: true
    enableOpenAPI: false

serviceRules:
  requireDescription: true
  allowedTiers:
    - backend
    - worker
  defaultTier: backend
  allowedLifecycles:
    - experimental
    - production
  defaultLifecycle: experimental

repoRules:
  defaultBranch:
    value: main
    mutable: false
  topics:
    maxItems: 10
    pattern: "^[a-z0-9-]+$"

catalogRules:
  requiredFields:
    - system
    - domain
  defaults:
    type: service
    lifecycle: experimental
  allowedTypes:
    - service

parameterSchema:
  type: object
  additionalProperties: false
  required:
    - goVersion
  properties:
    goVersion:
      type: string
      enum: ["1.23", "1.24"]
      default: "1.24"
      description: Go toolchain version
    enableDocker:
      type: boolean
      default: true
      description: Include Docker support
    enableOpenAPI:
      type: boolean
      default: false
      description: Generate OpenAPI assets

policyHints:
  approval:
    publicVisibilityRequiresApproval: true
  riskLevel: medium

compileHints:
  moduleKey: scaffolding/create-service
  contractVersion: scaffolding-create-service/v1
```

---

## 10. Validation Pipeline

Validation should occur in ordered phases.

---

## 10.1 Phase 1 — Base Request Schema Validation

Validate the request against `ServiceScaffoldRequest` shape from RFC-SCAFFOLD-001.

Examples:
- missing `spec.owner`
- malformed `delivery.callbackUrl`
- invalid `spec.visibility`

Failures here return `invalid`.

---

## 10.2 Phase 2 — Template Resolution

Resolve `spec.template` against the catalog.

Checks:
- template exists
- template status is not `disabled`
- tenant can see template if tenancy restrictions exist

Failures:
- unknown template → `INVALID_TEMPLATE`
- disabled template → `TEMPLATE_DISABLED`
- tenant not allowed → `TEMPLATE_NOT_ALLOWED`

---

## 10.3 Phase 3 — Defaulting

Apply effective defaults.

Sources:
1. platform defaults
2. tenant defaults
3. template defaults
4. parameter schema defaults

Defaults may apply to:
- `spec.visibility`
- `spec.service.*`
- `spec.repo.*`
- `spec.catalog.*`
- `spec.parameters.*`

The output of this phase is the **normalized request draft**.

---

## 10.4 Phase 4 — Eligibility and Constraint Validation

Validate request against template restrictions.

Checks include:
- tenant allowed
- owner allowed
- org allowed
- visibility allowed
- service tier allowed
- lifecycle allowed
- repo branch policy honored
- catalog type allowed

Failures here are usually `invalid`, unless platform policy chooses to escalate some cases to manual review.

---

## 10.5 Phase 5 — Parameter Validation

Validate `spec.parameters` against the template parameter schema.

Checks include:
- required parameters present after defaulting
- no unknown keys when forbidden
- types match
- strings satisfy pattern / enum / min / max rules
- arrays satisfy item constraints

---

## 10.6 Phase 6 — Cross-Field Semantic Validation

Validate combinations across request sections.

Examples:
- public visibility not allowed with template
- production lifecycle requires stronger metadata
- service description required by template
- catalog `system` required for this template
- repo name derived from service name must meet naming policy

This phase is where the template rules and broader platform rules meet.

---

## 10.7 Phase 7 — Policy Evaluation

Evaluate external or platform-level policy signals.

Examples:
- public repos require approval
- owner may only create repos in certain orgs
- callback URL must be allowlisted
- template may only be used by a specific group

Outcomes may be:
- `valid`
- `valid_with_warnings`
- `manual_review_required`
- `approval_required`
- `invalid`

---

## 11. Validation Outcomes

### 11.1 Valid

All required checks passed.

### 11.2 Valid With Warnings

Request can proceed, but issues should be surfaced.

Examples:
- deprecated template
- missing recommended tags
- preview template selected

### 11.3 Manual Review Required

The request is structurally valid but cannot proceed automatically.

Examples:
- public repo request in restricted tenant
- template allowed only by exception process
- callback URL outside normal allowlist but permitted for review path

### 11.4 Invalid

The request violates schema, template constraints, or hard policy.

Examples:
- unknown template
- unsupported visibility
- invalid parameter type
- owner not allowed

---

## 12. Error and Warning Model

Diagnostics should be structured.

### Example diagnostic

```json
{
  "code": "INVALID_TEMPLATE_PARAMETER",
  "message": "Parameter 'goVersion' must be one of: 1.23, 1.24",
  "field": "spec.parameters.goVersion",
  "severity": "error",
  "hint": "Use one of the supported Go versions for template 'go-grpc'"
}
```

### Recommended diagnostic codes

- `INVALID_TEMPLATE`
- `TEMPLATE_DISABLED`
- `TEMPLATE_DEPRECATED`
- `TEMPLATE_PREVIEW`
- `TEMPLATE_NOT_ALLOWED`
- `OWNER_NOT_ALLOWED`
- `ORG_NOT_ALLOWED`
- `VISIBILITY_NOT_ALLOWED`
- `INVALID_TEMPLATE_PARAMETER`
- `UNKNOWN_TEMPLATE_PARAMETER`
- `MISSING_REQUIRED_TEMPLATE_PARAMETER`
- `INVALID_SERVICE_TIER`
- `INVALID_SERVICE_LIFECYCLE`
- `MISSING_REQUIRED_SERVICE_DESCRIPTION`
- `MISSING_REQUIRED_CATALOG_FIELD`
- `INVALID_CATALOG_TYPE`
- `APPROVAL_REQUIRED`
- `MANUAL_REVIEW_REQUIRED`

---

## 13. Naming Validation Rules

The template catalog may not fully own all naming rules, but it must integrate with them.

### Recommended normalized service name policy

- lowercase
- alphanumeric plus hyphen
- must start with a letter
- max length platform-defined
- no consecutive hyphens
- no reserved names

Example regex:

```text
^[a-z][a-z0-9-]{1,62}$
```

This may be a platform-level rule, but templates may add further restrictions.

### Repo name derivation

By default:
- repo name derives from normalized `spec.service.name`
- if `spec.service.name` absent, derive from `metadata.name`

Derived names must still pass org/repo naming policy validation.

---

## 14. Parameter Schema Rules

A minimum supported parameter schema dialect should be documented and stable.

---

## 14.1 Supported Primitive Types

- `string`
- `boolean`
- `integer`
- `number`
- `array`
- `object`

---

## 14.2 String Constraints

Supported:
- `enum`
- `pattern`
- `minLength`
- `maxLength`

Example:

```yaml
runtimeMode:
  type: string
  enum: [grpc, http]
```

---

## 14.3 Numeric Constraints

Supported:
- `minimum`
- `maximum`

Example:

```yaml
replicaCount:
  type: integer
  minimum: 1
  maximum: 5
```

---

## 14.4 Array Constraints

Supported:
- `items`
- `minItems`
- `maxItems`

Example:

```yaml
extraTopics:
  type: array
  items:
    type: string
    pattern: "^[a-z0-9-]+$"
  maxItems: 5
```

---

## 14.5 Object Constraints

Supported:
- `properties`
- `required`
- `additionalProperties`

Nested object parameters may be supported, but initial implementation may reasonably restrict depth for simplicity.

---

## 15. MCP Contract Implications

This catalog supports the following MCP operations.

---

## 15.1 `list_service_templates`

Must return template summaries filtered by:
- tenant
- owner
- org
- visibility
- status

Recommended summary fields:
- `id`
- `displayName`
- `description`
- `status`
- `tags`
- `allowedVisibilities`
- high-level parameter summary

---

## 15.2 `get_service_template`

Must return:
- template metadata
- parameter schema
- defaults
- eligibility constraints
- warnings for deprecated or preview templates

---

## 15.3 `validate_service_scaffold_request`

Must use the catalog to:
- resolve template
- apply defaults
- validate `spec.parameters`
- produce diagnostics
- return normalized request

---

## 15.4 `compile_service_scaffold_request`

Must use the catalog to:
- ensure request is valid,
- determine contract and module hints,
- apply final effective defaults,
- and produce deterministic compiled output.

---

## 16. Template States and Lifecycle Policy

Templates should support lifecycle state management.

### `active`
Fully supported.

### `preview`
Available, but may produce warning:
- `TEMPLATE_PREVIEW`

### `deprecated`
Still available for compatibility, but warning required:
- `TEMPLATE_DEPRECATED`

### `disabled`
Not valid for new requests.

This lets the platform evolve template offerings without immediate hard breaks.

---

## 17. Catalog Composition and Overrides

The effective catalog may be built from multiple layers.

Recommended layering:

1. base platform catalog
2. tenant-specific overlays
3. environment-specific restrictions
4. emergency denylist/disable overrides

### Example use cases

- tenant A can use `go-grpc`
- tenant B cannot
- org `oss-public` only allows templates flagged as open-source compatible
- deprecated template disabled in one tenant earlier than others

The implementation may merge these layers into one effective catalog at load time.

---

## 18. Example Multi-Template Catalog

```yaml
apiVersion: platform.gatblau.io/v1alpha1
kind: ServiceTemplateCatalog
metadata:
  name: platform-base
spec:
  templates:
    - id: go-grpc
      version: 1.0.0
      displayName: Go gRPC Service
      status: active
      allowedTenants: [acme]
      allowedOrgs: [acme-platform]
      allowedVisibilities: [internal, private]
      defaults:
        visibility: internal
        parameters:
          goVersion: "1.24"
          enableDocker: true
      parameterSchema:
        type: object
        additionalProperties: false
        required: [goVersion]
        properties:
          goVersion:
            type: string
            enum: ["1.23", "1.24"]
          enableDocker:
            type: boolean
            default: true

    - id: node-express
      version: 1.0.0
      displayName: Node Express Service
      status: active
      allowedTenants: [acme]
      allowedOrgs: [acme-platform]
      allowedVisibilities: [internal, private, public]
      defaults:
        visibility: internal
        parameters:
          nodeVersion: "22"
          enableDocker: true
      parameterSchema:
        type: object
        additionalProperties: false
        required: [nodeVersion]
        properties:
          nodeVersion:
            type: string
            enum: ["20", "22"]
          enableDocker:
            type: boolean
            default: true

    - id: java-spring
      version: 0.9.0
      displayName: Java Spring Service
      status: preview
      allowedTenants: [acme]
      allowedOrgs: [acme-platform]
      allowedVisibilities: [internal]
      defaults:
        visibility: internal
        parameters:
          javaVersion: "21"
          buildTool: gradle
      parameterSchema:
        type: object
        additionalProperties: false
        required: [javaVersion, buildTool]
        properties:
          javaVersion:
            type: string
            enum: ["17", "21"]
          buildTool:
            type: string
            enum: [gradle, maven]
```

---

## 19. Validation Examples

---

## 19.1 Valid Request

```yaml
apiVersion: platform.gatblau.io/v1alpha1
kind: ServiceScaffoldRequest
metadata:
  name: payments-api
  tenant: acme
spec:
  owner: team-payments
  template: go-grpc
  org: acme-platform
  parameters:
    goVersion: "1.24"
```

Result:
- valid
- `visibility` defaults to `internal`
- `enableDocker` defaults to `true`

---

## 19.2 Invalid Unknown Parameter

```yaml
spec:
  template: go-grpc
  parameters:
    goVersion: "1.24"
    unsupportedThing: true
```

If `additionalProperties: false`:
- invalid
- `UNKNOWN_TEMPLATE_PARAMETER`

---

## 19.3 Invalid Visibility

```yaml
spec:
  template: go-grpc
  visibility: public
```

If template allows only `internal` and `private`:
- invalid or manual review depending on platform policy
- recommended default: invalid

---

## 19.4 Warning for Deprecated Template

```yaml
spec:
  template: old-node-service
```

If template state is `deprecated`:
- valid_with_warnings
- warning code `TEMPLATE_DEPRECATED`

---

## 20. Normalization Rules

During validation, the following normalization should be applied consistently:

- trim scalar strings
- canonicalize enum values where case-insensitive matching is supported
- apply template defaults
- apply parameter defaults
- derive absent service name from `metadata.name`
- preserve caller-provided values unless forbidden
- sort parameter keys for stable hashing
- omit purely empty optional sections from normalized form if desired, but do so consistently

Normalization behavior must be deterministic because later compilation depends on it.

---

## 21. Implementation Guidance

A practical Go implementation may separate concerns like this:

```text
internal/scaffolding/
  catalog/
    loader.go
    model.go
    merge.go
  validate/
    schema.go
    template_rules.go
    parameters.go
    policy.go
  normalize/
    defaults.go
    canonicalize.go
```

Recommended interfaces:

- `CatalogProvider`
- `TemplateResolver`
- `RequestNormalizer`
- `RequestValidator`

Example conceptual interfaces:

```go
type CatalogProvider interface {
    EffectiveCatalog(ctx context.Context, tenant string) (*ServiceTemplateCatalog, error)
}

type TemplateResolver interface {
    ResolveTemplate(ctx context.Context, tenant, templateID string) (*TemplateDefinition, error)
}

type RequestValidator interface {
    Validate(ctx context.Context, req ServiceScaffoldRequest) ValidationResult
}
```

---

## 22. Testing Requirements

This RFC implies fixture and unit test coverage for:

- catalog loading
- catalog merge/overlay behavior
- template resolution
- parameter validation
- defaulting
- warnings for preview/deprecated templates
- deny-overrides-allow behavior
- deterministic normalization

### Recommended fixture classes

- valid templates
- malformed templates
- deprecated templates
- disabled templates
- tenant-restricted templates
- owner-restricted templates
- invalid parameter combinations

---

## 23. Backward Compatibility and Versioning

Catalog evolution must be controlled.

### Versioning surfaces

- catalog schema version
- template definition version
- compiled contract version

### Compatibility guidance

- adding optional parameters is backward-compatible
- adding new defaults is potentially behavior-changing and must be reviewed
- removing parameters or tightening enums is breaking
- changing template IDs is breaking
- changing status to `deprecated` is non-breaking
- changing status to `disabled` is breaking for new requests

Template changes should be reviewed with golden tests to detect drift.

---

## 24. Security Considerations

### 24.1 Safe Parameter Exposure
Only approved parameter schemas should be exposed to callers.

### 24.2 Restrict Dangerous Options
Templates should not expose arbitrary pass-through configuration that bypasses platform policy.

### 24.3 Visibility Controls
Public repository support should be explicit and reviewable.

### 24.4 Callback and Metadata Hygiene
Catalog-defined defaults must not silently inject unsafe endpoints or metadata.

### 24.5 Tenant Isolation
Template access must be evaluated in tenant context.

---

## 25. Open Questions

1. Should nested object parameters be fully supported in v1, or restricted?
2. Should template state `preview` require explicit opt-in by tenants?
3. Should `allowedOwners` be exact-match only, or support groups/patterns?
4. Should parameter schemas support conditional rules such as "if X, require Y"?
5. Should catalog overlays be file-based, DB-backed, or both?
6. Should some validation outcomes return `manual_review_required` instead of `invalid` by template flag?

---

## 26. Decision

This RFC proposes a declarative template catalog that acts as the source of truth for:

- template discovery,
- template eligibility,
- template-specific defaults,
- parameter validation,
- and request normalization inputs.

This keeps the MCP scaffolding flow:

- introspectable,
- deterministic,
- policy-aware,
- and testable.

---
