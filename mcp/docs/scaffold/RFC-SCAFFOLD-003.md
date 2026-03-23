# RFC-SCAFFOLD-003 — Compile-to-Release-Engine Mapping

---

## 1. Summary

This RFC defines how a validated **`ServiceScaffoldRequest`** is compiled into a deterministic Release Engine job targeting the module:

```text
scaffolding/create-service
```

It standardizes:

- the **compiled plan shape**,
- the mapping from intent fields to module params,
- default and derived value handling,
- idempotency key derivation,
- request hashing,
- submission payload structure,
- and the boundary between **MCP compilation** and **Release Engine execution**.

This RFC builds on:

- **RFC-SCAFFOLD-001** — Scaffolding intent model and MCP contract
- **RFC-SCAFFOLD-002** — Template catalog and validation rules

---

## 2. Problem Statement

By the time a request reaches execution, the platform needs more than a user-supplied scaffold intent.

It needs a payload that is:

- fully validated,
- normalized,
- deterministic,
- stable for retries,
- traceable for audit,
- and directly consumable by Release Engine.

The Release Engine module contract is intentionally narrow:

```go
type Module interface {
    Key() string
    Version() string
    Execute(ctx context.Context, api any, params map[string]any) error
}
```

For `scaffolding/create-service`, the raw module params are roughly:

- `template`
- `service_name`
- `owner`
- `org`
- `parameters`
- `visibility`
- `callback_url`

However, the intent request contains richer data and semantics:

- metadata and tenant context,
- service metadata,
- repo metadata,
- catalog metadata,
- delivery preferences,
- template defaults and policy outcomes.

A compile phase is therefore required to convert validated intent into an execution-ready payload.

---

## 3. Goals

### 3.1 Primary Goals

This RFC aims to:

1. define a **canonical compiled plan structure**,
2. define deterministic mapping from request → module params,
3. define **derived fields** and precedence rules,
4. define **idempotency key derivation**,
5. define **request hashing** for replay safety and auditability,
6. define the Release Engine submission envelope,
7. preserve a clean separation between **compile** and **execute**.

### 3.2 Non-Goals

This RFC does **not** define:

- template rendering implementation details,
- Release Engine internal persistence schema,
- connector retry strategy,
- approval workflow orchestration,
- or the exact event schema for downstream notifications.

---

## 4. Design Principles

### 4.1 Deterministic Compilation

The same normalized input must always compile to the same plan.

### 4.2 Explicit Over Implicit

All derived values should be materialized in compiled output where useful.

### 4.3 Stable Contract Boundary

The Release Engine module should receive a stable param contract, independent of how rich the intent model becomes.

### 4.4 Auditability

Compilation should produce enough metadata to explain:
- what was submitted,
- why defaults were applied,
- which template was resolved,
- and which exact input produced the job.

### 4.5 Idempotent Submission

Equivalent requests should be safe to deduplicate.

---

## 5. Compilation Overview

Compilation happens **after validation** and **before submission**.

### Pipeline

1. receive `ServiceScaffoldRequest`
2. validate request shape and semantics
3. resolve template from catalog
4. apply defaults and normalization
5. derive missing values
6. build compiled plan
7. compute request hash
8. compute idempotency key
9. optionally submit compiled plan to Release Engine

---

## 6. Inputs to Compilation

The compiler consumes:

- the original request,
- the normalized request,
- the resolved template definition,
- effective policy outcomes,
- platform submission settings.

### Required effective inputs

- `request`
- `normalizedRequest`
- `template`
- `contractVersion`
- `submissionContext`

### Example submission context

```yaml
submissionContext:
  environment: production
  releaseEngineEndpoint: internal
  idempotencyNamespace: scaffold-service
  defaultCallbackMode: webhook
```

---

## 7. Outputs of Compilation

Compilation returns a **Compiled Service Scaffold Plan**.

This plan is the canonical bridge between MCP and Release Engine.

### High-level contents

- plan metadata
- source request reference
- normalized intent snapshot
- derived values
- module target
- module params
- idempotency metadata
- request hash
- submission payload preview

---

## 8. Compiled Plan Schema

Recommended logical shape:

```yaml
apiVersion: platform.gatblau.io/v1alpha1
kind: CompiledServiceScaffoldPlan
metadata:
  name: payments-api
spec:
  source:
    requestName: payments-api
    requestUID: 2a1b6c0d
    tenant: acme
  template:
    id: go-grpc
    version: 1.0.0
  contract:
    moduleKey: scaffolding/create-service
    contractVersion: scaffolding-create-service/v1
  derived:
    serviceName: payments-api
    repoName: payments-api
    catalogName: payments-api
    effectiveVisibility: internal
  moduleParams:
    template: go-grpc
    service_name: payments-api
    owner: team-payments
    org: acme-platform
    visibility: internal
    parameters:
      goVersion: "1.24"
      enableDocker: true
      enableOpenAPI: false
    callback_url: https://example.internal/hooks/scaffold/123
  idempotency:
    namespace: scaffold-service
    key: scaffold-service:acme:acme-platform:payments-api:go-grpc:2fd2a1d4
  hashes:
    normalizedRequestSha256: 2fd2a1d4...
    compiledPlanSha256: a81e9c11...
```

---

## 9. Plan Sections

---

## 9.1 `source`

Captures where the plan came from.

### Recommended fields

- `requestName`
- `requestUID` if available
- `tenant`
- `submittedBy` if available
- `compileTimestamp`
- `requestVersion`

### Example

```yaml
source:
  requestName: payments-api
  requestUID: req-01JABCXYZ
  tenant: acme
  submittedBy: agent/backstage
  requestVersion: v1alpha1
```

This section is informational and audit-oriented.

---

## 9.2 `template`

Captures the resolved template identity.

### Recommended fields

- `id`
- `version`
- `status`

### Example

```yaml
template:
  id: go-grpc
  version: 1.0.0
  status: active
```

This ensures downstream systems know exactly which template definition was used at compile time.

---

## 9.3 `contract`

Declares the execution target.

### Required fields

- `moduleKey`
- `contractVersion`

### Example

```yaml
contract:
  moduleKey: scaffolding/create-service
  contractVersion: scaffolding-create-service/v1
```

These values are typically sourced from template compile hints or platform defaults.

---

## 9.4 `derived`

Captures values computed by MCP during compilation.

### Recommended fields

- `serviceName`
- `repoName`
- `catalogName`
- `effectiveVisibility`
- `callbackUrlResolved`
- `defaultedFields`

### Example

```yaml
derived:
  serviceName: payments-api
  repoName: payments-api
  catalogName: payments-api
  effectiveVisibility: internal
  defaultedFields:
    - spec.visibility
    - spec.parameters.enableDocker
    - spec.parameters.enableOpenAPI
```

This section improves traceability and explainability.

---

## 9.5 `moduleParams`

This is the exact parameter map passed to Release Engine module execution.

This section is normative.

---

## 10. Module Param Contract

The v1 compiled contract for `scaffolding/create-service` is:

```yaml
template: string
service_name: string
owner: string
org: string
parameters: map[string, any]
visibility: string
callback_url: string? 
```

### Required params

- `template`
- `service_name`
- `owner`
- `org`
- `parameters`

### Optional params

- `visibility`
- `callback_url`

If omitted, optional params must be handled by module defaults or upstream normalization rules.

---

## 11. Canonical Mapping Rules

This section defines the normative mapping from normalized request to module params.

---

## 11.1 `template`

### Source
`normalizedRequest.spec.template`

### Rule
Pass through unchanged.

### Example

```yaml
spec:
  template: go-grpc
```

becomes:

```yaml
moduleParams:
  template: go-grpc
```

---

## 11.2 `service_name`

### Source
Derived from effective service name.

### Derivation order

1. `normalizedRequest.spec.service.name`
2. `normalizedRequest.metadata.name`

### Rule
The derived name must already have passed naming validation.

### Example

```yaml
metadata:
  name: payments-api
spec:
  service: {}
```

becomes:

```yaml
moduleParams:
  service_name: payments-api
```

---

## 11.3 `owner`

### Source
`normalizedRequest.spec.owner`

### Rule
Pass through unchanged after validation and normalization.

---

## 11.4 `org`

### Source
`normalizedRequest.spec.org`

### Rule
Pass through unchanged after validation and normalization.

---

## 11.5 `parameters`

### Source
`normalizedRequest.spec.parameters`

### Rule
Pass the normalized parameter object after:
- defaulting,
- schema validation,
- stable key ordering for hashing.

The param map contents are template-specific.

### Example

```yaml
spec:
  parameters:
    goVersion: "1.24"
```

with defaults:

```yaml
enableDocker: true
enableOpenAPI: false
```

becomes:

```yaml
moduleParams:
  parameters:
    enableDocker: true
    enableOpenAPI: false
    goVersion: "1.24"
```

---

## 11.6 `visibility`

### Source
Effective visibility after defaulting and policy evaluation.

### Derivation order

1. `normalizedRequest.spec.visibility`
2. template default
3. tenant/platform default

### Rule
Pass final effective visibility.

### Example

```yaml
moduleParams:
  visibility: internal
```

---

## 11.7 `callback_url`

### Source
Resolved delivery callback URL.

### Derivation order

1. `normalizedRequest.spec.delivery.callbackUrl`
2. platform-generated callback relay URL, if enabled
3. omitted

### Rule
Only include if allowed by policy.

### Example

```yaml
moduleParams:
  callback_url: https://example.internal/hooks/scaffold/123
```

---

## 12. Fields Not Passed to the Module

Not all intent fields should become module params.

The following are typically **consumed upstream** by MCP and not passed directly:

- `metadata.tenant`
- request labels/annotations used only for policy
- `spec.repo.topics`
- `spec.catalog.*`
- approval metadata
- warning state
- request diagnostics
- request hash / idempotency metadata

These may still influence compilation, naming, or future module evolution, but are not part of the v1 module contract.

---

## 13. Derived Value Rules

Compilation may derive several execution values from validated input.

---

## 13.1 Effective Service Name

```text
effectiveServiceName =
  spec.service.name
  else metadata.name
```

If both are present and differ, the platform should choose one canonical rule and enforce it during validation.

**Recommended v1 rule:** they must match if both are provided.

---

## 13.2 Effective Repo Name

```text
effectiveRepoName = effectiveServiceName
```

The current module does not expose repo name separately. The assumption in v1 is that repository name equals service name.

If that changes later, a new contract version should make `repo_name` explicit.

---

## 13.3 Effective Catalog Name

Usually derived from service name unless explicitly modeled.

```text
effectiveCatalogName = effectiveServiceName
```

This is primarily informational in v1.

---

## 13.4 Effective Visibility

Resolved during validation/defaulting and surfaced into compiled output.

---

## 13.5 Effective Callback URL

If callback relay mode exists, the compiler may resolve a platform-owned URL rather than pass through the caller value directly.

---

## 14. Contract Versioning

The compiler must stamp the compiled output with an explicit contract version.

### Recommended initial value

```text
scaffolding-create-service/v1
```

This version controls:

- module param names,
- required fields,
- mapping behavior,
- omission rules,
- derived field assumptions.

Any breaking change to the compiled param shape must create a new contract version.

---

## 15. Deterministic Serialization

To safely hash and deduplicate requests, compilation must use deterministic serialization rules.

### Recommended rules

- stable field ordering
- stable map key ordering
- no random values in compiled output prior to hashing
- omit ephemeral timestamps from hash input
- canonical representation of null/empty fields
- normalized string casing where policy allows

### Suggested approach

Hash the **normalized request projection**, not raw user input.

That means two semantically equivalent requests should produce the same normalized request hash even if:

- field ordering differs,
- optional defaults were omitted in one request but explicitly set in another,
- harmless whitespace differs.

---

## 16. Request Hashing

### 16.1 Purpose

Request hashes support:

- deduplication,
- drift detection,
- audit traceability,
- reproducible compile testing.

### 16.2 Hash Input

The recommended hash input is the normalized request plus relevant template/contract identifiers.

### Example hash projection

```yaml
requestHashInput:
  request:
    metadata:
      name: payments-api
      tenant: acme
    spec:
      template: go-grpc
      owner: team-payments
      org: acme-platform
      visibility: internal
      service:
        name: payments-api
      parameters:
        enableDocker: true
        enableOpenAPI: false
        goVersion: "1.24"
  template:
    id: go-grpc
    version: 1.0.0
  contractVersion: scaffolding-create-service/v1
```

### 16.3 Algorithm

**Recommended:** SHA-256 over canonical JSON serialization.

### Example

```text
normalizedRequestSha256 = sha256(canonical_json(hash_input))
```

---

## 17. Idempotency Key Derivation

### 17.1 Purpose

The idempotency key is used to prevent accidental duplicate submissions.

### 17.2 Requirements

An idempotency key should be:

- deterministic for equivalent requests,
- scoped to tenant/org/module where relevant,
- safe to log,
- not excessively long.

### 17.3 Recommended format

```text
<namespace>:<tenant>:<org>:<service_name>:<template>:<hash_prefix>
```

### Example

```text
scaffold-service:acme:acme-platform:payments-api:go-grpc:2fd2a1d4
```

### 17.4 Hash prefix

Use a short prefix from the normalized request hash, e.g. first 8–16 hex chars.

### 17.5 Collision guidance

A short prefix is acceptable operationally when used alongside namespace and logical identifiers, but the full hash should still be retained in plan metadata.

---

## 18. Submission Payload to Release Engine

The compiler should produce or be able to produce the exact submission envelope for Release Engine.

### Recommended envelope

```yaml
module: scaffolding/create-service
params:
  contract_version: scaffolding-create-service/v1
  template: go-grpc
  service_name: payments-api
  owner: team-payments
  org: acme-platform
  visibility: internal
  parameters:
    goVersion: "1.24"
    enableDocker: true
    enableOpenAPI: false
  callback_url: https://example.internal/hooks/scaffold/123
metadata:
  tenant: acme
  idempotency_key: scaffold-service:acme:acme-platform:payments-api:go-grpc:2fd2a1d4
  request_hash: 2fd2a1d4...
```

### Notes

- `contract_version` should be included in the submitted params or envelope metadata.
- submission metadata may be platform-specific and need not be passed into the module itself.
- the normative module execution payload remains the `params` map.

---

## 19. MCP Tool Contract Implications

This RFC affects the behavior of two main MCP tools:

- `compile_service_scaffold_request`
- `submit_service_scaffold_request`

---

## 19.1 `compile_service_scaffold_request`

Should return:

- normalized request
- validation outcome
- compiled plan
- request hash
- idempotency key
- submission preview

### Example response shape

```json
{
  "valid": true,
  "normalizedRequest": {
    "metadata": {
      "name": "payments-api",
      "tenant": "acme"
    },
    "spec": {
      "template": "go-grpc",
      "owner": "team-payments",
      "org": "acme-platform",
      "visibility": "internal",
      "parameters": {
        "goVersion": "1.24",
        "enableDocker": true,
        "enableOpenAPI": false
      }
    }
  },
  "compiledPlan": {
    "contract": {
      "moduleKey": "scaffolding/create-service",
      "contractVersion": "scaffolding-create-service/v1"
    },
    "moduleParams": {
      "template": "go-grpc",
      "service_name": "payments-api",
      "owner": "team-payments",
      "org": "acme-platform",
      "visibility": "internal",
      "parameters": {
        "goVersion": "1.24",
        "enableDocker": true,
        "enableOpenAPI": false
      }
    }
  },
  "idempotency": {
    "key": "scaffold-service:acme:acme-platform:payments-api:go-grpc:2fd2a1d4"
  }
}
```

---

## 19.2 `submit_service_scaffold_request`

Should:

1. validate and compile the request,
2. compute hash and idempotency key,
3. submit to Release Engine,
4. return a job handle and plan summary.

### Example response shape

```json
{
  "job": {
    "id": "job-01JABD123",
    "module": "scaffolding/create-service",
    "status": "queued"
  },
  "contract": {
    "version": "scaffolding-create-service/v1"
  },
  "idempotency": {
    "key": "scaffold-service:acme:acme-platform:payments-api:go-grpc:2fd2a1d4"
  },
  "hashes": {
    "normalizedRequestSha256": "2fd2a1d4..."
  }
}
```

---

## 20. Example End-to-End Compile

### Input request

```yaml
apiVersion: platform.gatblau.io/v1alpha1
kind: ServiceScaffoldRequest
metadata:
  name: payments-api
  tenant: acme
spec:
  template: go-grpc
  owner: team-payments
  org: acme-platform
  parameters:
    goVersion: "1.24"
  delivery:
    callbackUrl: https://example.internal/hooks/scaffold/123
```

### Resolved template defaults

```yaml
defaults:
  visibility: internal
  parameters:
    enableDocker: true
    enableOpenAPI: false
```

### Normalized request

```yaml
metadata:
  name: payments-api
  tenant: acme
spec:
  template: go-grpc
  owner: team-payments
  org: acme-platform
  visibility: internal
  service:
    name: payments-api
  parameters:
    goVersion: "1.24"
    enableDocker: true
    enableOpenAPI: false
  delivery:
    callbackUrl: https://example.internal/hooks/scaffold/123
```

### Compiled plan

```yaml
apiVersion: platform.gatblau.io/v1alpha1
kind: CompiledServiceScaffoldPlan
metadata:
  name: payments-api
spec:
  source:
    requestName: payments-api
    tenant: acme
  template:
    id: go-grpc
    version: 1.0.0
    status: active
  contract:
    moduleKey: scaffolding/create-service
    contractVersion: scaffolding-create-service/v1
  derived:
    serviceName: payments-api
    repoName: payments-api
    effectiveVisibility: internal
  moduleParams:
    template: go-grpc
    service_name: payments-api
    owner: team-payments
    org: acme-platform
    visibility: internal
    parameters:
      goVersion: "1.24"
      enableDocker: true
      enableOpenAPI: false
    callback_url: https://example.internal/hooks/scaffold/123
  idempotency:
    namespace: scaffold-service
    key: scaffold-service:acme:acme-platform:payments-api:go-grpc:2fd2a1d4
  hashes:
    normalizedRequestSha256: 2fd2a1d4...
```

### Submission envelope

```yaml
module: scaffolding/create-service
params:
  contract_version: scaffolding-create-service/v1
  template: go-grpc
  service_name: payments-api
  owner: team-payments
  org: acme-platform
  visibility: internal
  parameters:
    goVersion: "1.24"
    enableDocker: true
    enableOpenAPI: false
  callback_url: https://example.internal/hooks/scaffold/123
metadata:
  tenant: acme
  idempotency_key: scaffold-service:acme:acme-platform:payments-api:go-grpc:2fd2a1d4
  request_hash: 2fd2a1d4...
```

---

## 21. Error Handling in Compilation

Compilation should fail fast if any required precondition is missing.

### Recommended compilation error codes

- `COMPILE_INVALID_REQUEST`
- `COMPILE_TEMPLATE_UNRESOLVED`
- `COMPILE_CONTRACT_UNSUPPORTED`
- `COMPILE_DERIVATION_FAILED`
- `COMPILE_CALLBACK_NOT_ALLOWED`
- `COMPILE_HASH_FAILED`
- `COMPILE_IDEMPOTENCY_FAILED`

### Example diagnostic

```json
{
  "code": "COMPILE_DERIVATION_FAILED",
  "message": "Unable to derive service_name from request",
  "field": "spec.service.name",
  "severity": "error"
}
```

---

## 22. Backward Compatibility and Evolution

This RFC defines **contract version `scaffolding-create-service/v1`**.

Breaking changes requiring a new version include:

- renaming module params,
- changing source mapping semantics,
- introducing a distinct `repo_name`,
- changing omission behavior for `visibility`,
- changing callback handling incompatibly.

Non-breaking changes may include:

- adding compiled metadata fields,
- adding advisory derived fields,
- adding extra submission metadata outside module params.

### Example future version triggers

#### v2 candidates

- explicit `repo_name`
- explicit `catalog_descriptor`
- explicit `catalog_metadata`
- module-level support for enriched repo configuration

---

## 23. Implementation Guidance

A practical Go implementation may be structured like:

```text
internal/scaffolding/
  compile/
    compiler.go
    mapping.go
    derive.go
    hash.go
    idempotency.go
    contract_v1.go
```

### Suggested interfaces

```go
type Compiler interface {
    Compile(ctx context.Context, req ServiceScaffoldRequest) (*CompiledServiceScaffoldPlan, error)
}

type Hasher interface {
    HashNormalizedRequest(req NormalizedServiceScaffoldRequest, template ResolvedTemplate, contractVersion string) (string, error)
}

type IdempotencyKeyBuilder interface {
    Build(plan *CompiledServiceScaffoldPlan) (string, error)
}
```

### Suggested compiler stages

1. assert validated request
2. derive effective names
3. construct `moduleParams`
4. build hash input projection
5. compute request hash
6. compute idempotency key
7. assemble compiled plan

---

## 24. Testing Requirements

This RFC implies golden and unit tests for:

- normalized request → compiled plan mapping
- stable hash generation
- stable idempotency key generation
- callback inclusion/omission rules
- explicit vs defaulted visibility
- equivalent requests producing same hash
- non-equivalent requests producing different hashes
- compile failure when service name cannot be derived

### Recommended golden test categories

- minimal valid request
- fully populated request
- defaulted parameter request
- callback-enabled request
- deprecated template request
- invalid compile contract request

---

## 25. Security Considerations

### 25.1 No Unsafe Raw Pass-Through
Only validated and normalized template parameters may flow into `moduleParams.parameters`.

### 25.2 Callback URL Governance
Caller-supplied callback URLs must be policy-checked before being compiled into the submission payload.

### 25.3 Hash Safety
Do not include secrets or tokens in cleartext hash projections if logs may expose them.

### 25.4 Idempotency Scope
Idempotency keys should be scoped enough to avoid accidental cross-tenant collisions.

### 25.5 Auditability
Compilation metadata should support post-incident reconstruction of what was submitted.

---

## 26. Open Questions

1. Should `callback_url` live in params or Release Engine metadata only?
2. Should `repo_name` become explicit in v1 despite the current module shape?
3. Should catalog descriptor content eventually be pre-rendered upstream and passed into the module?
4. Should approval state be embedded in compiled plan metadata?
5. Should idempotency keys be caller-overridable in special workflows?
6. Should the compiler hash only normalized request content, or also selected policy version identifiers?

---

## 27. Decision

This RFC proposes that scaffolding requests be compiled into a deterministic **CompiledServiceScaffoldPlan** that:

- targets module `scaffolding/create-service`,
- uses contract version `scaffolding-create-service/v1`,
- maps normalized intent into a stable module param payload,
- and includes hashing and idempotency metadata for safe submission.

This preserves a clean architecture:

- **intent and policy in MCP**
- **execution in Release Engine**

---