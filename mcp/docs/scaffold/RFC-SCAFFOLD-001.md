# RFC-SCAFFOLD-001 — Scaffolding Intent Model and MCP Contract

**Status:** Draft  
**Authors:** Gatblau 
**Audience:** Platform team, Release Engine maintainers, MCP server implementers, Backstage / agent workflow integrators  
**Last Updated:** March 15, 2026

---

## 1. Summary

This RFC defines the **intent-facing request model** and **MCP contract** for service scaffolding workflows executed through Release Engine.

It introduces a structured, agent-friendly API for requesting creation of a new service repository and registration in the service catalog, without exposing agents directly to low-level module parameters or connector-specific execution details.

The RFC standardizes:

- the **`ServiceScaffoldRequest`** intent model,
- the **MCP tools** used to draft, validate, compile, submit, and inspect requests,
- the separation between **intent resolution** and **durable execution**,
- the **response envelope** used by MCP tools,
- and the contract boundary between the MCP layer and the Release Engine module `scaffolding/create-service`.

This follows the same architectural approach used for infrastructure provisioning:
- **MCP handles intent, validation, defaults, policy, and compilation**
- **Release Engine handles execution, retries, compensation, and lifecycle tracking**

---

## 2. Problem Statement

The `scaffolding/create-service` Release Engine module is a strong execution primitive, but it is not an ideal **agent-facing contract** on its own.

Its raw parameters are operational and low-level:

- `template`
- `service_name`
- `owner`
- `org`
- `parameters`
- `visibility`
- `callback_url`

That parameter set is enough for a module invocation, but not enough to safely support higher-level workflows where an agent or UI must:

- discover available templates,
- validate whether a team may use a template,
- apply defaults,
- validate naming and ownership rules,
- preview expected outcomes,
- compile a stable request into an execution payload,
- and submit work deterministically with traceable diagnostics.

Without an intent layer, the execution module becomes responsible for interpreting ambiguous user input, which leads to:

- duplicated validation logic,
- poor previewability,
- unstable API behavior,
- harder testing,
- weak policy enforcement,
- and poor ergonomics for MCP-based agents.

---

## 3. Goals

### 3.1 Primary Goals

This RFC aims to:

1. define a **canonical intent model** for service scaffolding requests,
2. define the **MCP tool surface** for scaffolding workflows,
3. preserve a clean boundary between:
    - **request interpretation**, and
    - **workflow execution**,
4. support deterministic compilation into Release Engine job parameters,
5. provide a foundation for:
    - template catalogs,
    - validation engines,
    - policy evaluation,
    - preview tooling,
    - and golden tests.

### 3.2 Non-Goals

This RFC does **not** define:

- the complete template catalog schema,
- the full policy engine,
- the full compiled parameter mapping spec,
- GitHub connector details,
- catalog connector implementation details,
- or the Release Engine step graph.

Those belong in later RFCs.

---

## 4. Design Principles

### 4.1 Intent First

Agents should express **what service they want**, not how to invoke connector operations.

### 4.2 Deterministic Compilation

A validated request must compile into a stable, explicit execution payload.

### 4.3 Release Engine as Executor

Release Engine remains responsible for:

- step sequencing,
- retries,
- compensation,
- connector calls,
- execution durability,
- status tracking.

It should not be the primary place where ambiguous user intent is interpreted.

### 4.4 Policy Before Execution

Ownership, naming, visibility, and template eligibility should be checked before job submission whenever possible.

### 4.5 Domain-Specific, Not Module-Specific

The MCP interface should model the **service scaffolding domain**, not merely mirror raw module params.

---

## 5. Scope

This RFC applies to workflows that create a new service scaffold by:

1. rendering a supported service template,
2. creating a source repository,
3. registering a component in the service catalog,
4. and emitting completion notifications.

The initial execution target is the Release Engine module:

```text
scaffolding/create-service
```

---

## 6. Terminology

### 6.1 Service Scaffold Request
A high-level request describing a service to be created from a supported platform template.

### 6.2 Template Catalog
A platform-managed list of scaffolding templates, defaults, constraints, and metadata.

### 6.3 Compilation
Transformation of an intent request into a deterministic Release Engine job payload.

### 6.4 Resolved Request
A request after defaults, normalization, and policy-relevant derivations have been applied.

### 6.5 Compiled Scaffold Plan
A machine-readable plan produced by the compiler that contains the normalized request, selected template, derived fields, validation context, and the job submission payload.

---

## 7. Architecture

The architecture is split into two layers.

### 7.1 MCP Layer Responsibilities

The MCP layer is responsible for:

- drafting requests from user intent,
- validating structure and semantics,
- looking up templates and constraints,
- applying defaults,
- normalizing fields,
- checking policy,
- compiling to module params,
- submitting jobs to Release Engine,
- fetching request/job status.

### 7.2 Release Engine Responsibilities

Release Engine is responsible for:

- executing the compiled workflow,
- rendering templates,
- invoking connectors,
- performing compensating cleanup,
- persisting step state,
- and reporting execution outcomes.

### 7.3 Boundary

The MCP layer must submit an already-validated, already-compiled parameter payload.

The Release Engine module should receive a stable contract such as:

```yaml
contract_version: scaffolding-create-service/v1
template: go-grpc
service_name: payments-api
owner: team-payments
org: acme-platform
visibility: internal
parameters:
  goVersion: "1.24"
  enableDocker: true
callback_url: https://backstage.example/callback
```

---

## 8. High-Level Workflow

The intended workflow is:

1. User or agent expresses desire to create a service.
2. MCP drafts or accepts a `ServiceScaffoldRequest`.
3. MCP validates schema and semantics.
4. MCP resolves template and defaults.
5. MCP compiles a `CompiledScaffoldPlan`.
6. MCP submits the compiled plan to Release Engine.
7. Release Engine executes `scaffolding/create-service`.
8. MCP or caller polls status and receives the outcome.

---

## 9. Intent Model

## 9.1 Resource Shape

The canonical request resource is:

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
  visibility: internal
  service:
    name: payments-api
    description: Payments API
    tier: backend
    lifecycle: experimental
  repo:
    defaultBranch: main
    topics:
      - go
      - grpc
      - payments
  catalog:
    system: payments
    domain: commerce
  parameters:
    goVersion: "1.24"
    enableDocker: true
    enableOpenAPI: true
  delivery:
    callbackUrl: https://backstage.example/callback
```

---

## 9.2 Top-Level Fields

### `apiVersion`
Version of the intent schema.

Initial value:

```text
platform.gatblau.io/v1alpha1
```

### `kind`
Must be:

```text
ServiceScaffoldRequest
```

### `metadata`
Envelope metadata for identity and tenancy.

### `spec`
Requested service scaffold details.

---

## 10. `metadata` Contract

### 10.1 Required Fields

- `name`
- `tenant`

### 10.2 Recommended Fields

- `labels`
- `annotations`
- `requester`

### 10.3 Semantics

#### `metadata.name`
A stable request name or resource name used for tracking.  
Should be DNS-label compatible where possible.

#### `metadata.tenant`
Logical tenant or platform boundary under which the request is evaluated.

#### `metadata.requester`
Optional identity metadata describing who initiated the request.

Example:

```yaml
metadata:
  name: payments-api
  tenant: acme
  requester:
    subject: user:alice
    displayName: Alice Smith
```

---

## 11. `spec` Contract

## 11.1 Required Fields

- `owner`
- `template`
- `org`

## 11.2 Common Optional Fields

- `visibility`
- `service`
- `repo`
- `catalog`
- `parameters`
- `delivery`

---

## 12. Field Semantics

## 12.1 `spec.owner`

Owning team, group, or platform-recognized owner reference.

Examples:

```yaml
owner: team-payments
```

Constraints:
- must resolve to a known owner if owner validation is enabled,
- may be subject to tenant restrictions,
- may be written into generated catalog metadata.

---

## 12.2 `spec.template`

Identifier of the platform-supported service template.

Examples:

```yaml
template: go-grpc
template: node-express
template: java-spring
```

Constraints:
- must exist in the template catalog,
- may impose required `parameters`,
- may constrain `visibility`,
- may imply language/runtime metadata.

---

## 12.3 `spec.org`

Target GitHub organization or equivalent SCM organization.

Example:

```yaml
org: acme-platform
```

Constraints:
- must be allowed for the tenant,
- may be restricted by owner/team,
- may have naming, visibility, or branch policy defaults.

---

## 12.4 `spec.visibility`

Repository visibility.

Allowed values:

- `public`
- `internal`
- `private`

Default:
- `internal`, unless overridden by template or tenant policy

Notes:
- some templates may forbid `public`,
- some tenants may force `private` or `internal`,
- public repositories may require approval.

---

## 12.5 `spec.service`

Logical service metadata.

Example:

```yaml
service:
  name: payments-api
  description: Payments API
  tier: backend
  lifecycle: experimental
  language: go
```

### Recommended child fields

- `name`
- `description`
- `tier`
- `lifecycle`
- `language`
- `runtime`

### Semantics

#### `service.name`
Preferred service name if different from `metadata.name`.  
If omitted, compilers may derive it from `metadata.name`.

#### `service.description`
Human-readable description for repository or catalog metadata.

#### `service.tier`
Optional classification such as:
- `frontend`
- `backend`
- `worker`
- `library`
- `cli`

#### `service.lifecycle`
Optional lifecycle such as:
- `experimental`
- `production`
- `deprecated`

---

## 12.6 `spec.repo`

Repository-specific preferences.

Example:

```yaml
repo:
  defaultBranch: main
  topics:
    - go
    - grpc
    - payments
  homepage: https://example.internal/services/payments-api
```

### Recommended child fields

- `defaultBranch`
- `topics`
- `homepage`
- `description`

Constraints:
- topic count and format may be limited by provider policy,
- default branch may be fixed by org policy,
- some fields may be informational only.

---

## 12.7 `spec.catalog`

Service catalog metadata.

Example:

```yaml
catalog:
  system: payments
  domain: commerce
  type: service
  lifecycle: experimental
```

This section is used to populate generated `catalog-info` content or equivalent descriptor metadata.

Possible fields:
- `system`
- `domain`
- `type`
- `lifecycle`
- `tags`
- `annotations`

Constraints:
- may be required for some templates,
- may be defaulted from tenant or template metadata.

---

## 12.8 `spec.parameters`

Template-specific parameters.

Example:

```yaml
parameters:
  goVersion: "1.24"
  enableDocker: true
  enableOpenAPI: true
```

This field is intentionally open-ended and validated against the chosen template’s parameter schema.

Important properties:
- keys are template-defined,
- types are template-defined,
- unknown keys may be rejected,
- defaults may be applied from the catalog.

---

## 12.9 `spec.delivery`

Notification or callback preferences.

Example:

```yaml
delivery:
  callbackUrl: https://backstage.example/callback
```

### Common child fields

- `callbackUrl`
- `notifyOn`
- `correlationId`

Constraints:
- callback URL may need to match an allowlist,
- callback URL may be optional if the platform uses internal event routing,
- correlation ID may be preserved through submission/status flows.

---

## 13. Minimal Valid Request

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
```

---

## 14. Rich Example

```yaml
apiVersion: platform.gatblau.io/v1alpha1
kind: ServiceScaffoldRequest
metadata:
  name: payments-api
  tenant: acme
  labels:
    environment: shared
    platform.gatblau.io/request-source: backstage
  requester:
    subject: user:alice
    displayName: Alice Smith
spec:
  owner: team-payments
  template: go-grpc
  org: acme-platform
  visibility: internal
  service:
    name: payments-api
    description: Payments API for internal transaction processing
    tier: backend
    lifecycle: experimental
    language: go
    runtime: grpc
  repo:
    defaultBranch: main
    topics:
      - go
      - grpc
      - payments
  catalog:
    system: payments
    domain: commerce
    type: service
    lifecycle: experimental
    tags:
      - payments
      - backend
  parameters:
    goVersion: "1.24"
    enableDocker: true
    enableOpenAPI: true
  delivery:
    callbackUrl: https://backstage.example/callback
    correlationId: req-12345
```

---

## 15. Validation Model

Validation is multi-phase.

## 15.1 Phase 1 — Schema Validation

Checks include:

- required fields present,
- string/enum/type correctness,
- URI format validation,
- map/list type correctness,
- base naming rules.

Examples:
- missing `spec.template`,
- invalid `visibility`,
- malformed `callbackUrl`.

---

## 15.2 Phase 2 — Semantic Validation

Checks include:

- template exists,
- owner is recognized,
- org is allowed for tenant,
- service name conforms to policy,
- selected visibility is allowed,
- template parameters match template schema,
- catalog metadata is sufficient for registration,
- reserved names are not used.

---

## 15.3 Phase 3 — Policy Validation

Checks include:

- whether public repo creation requires approval,
- whether this owner may use this template,
- whether certain orgs are restricted,
- whether callback URL domain is permitted,
- whether template/runtime combinations require additional metadata.

---

## 15.4 Validation Outcomes

Validation may result in:

- `valid`
- `invalid`
- `valid_with_warnings`
- `manual_review_required`

Examples:

- unknown template → `invalid`
- public repo in restricted tenant → `manual_review_required` or `approval_required`
- missing recommended catalog metadata → `valid_with_warnings`

---

## 16. MCP Tool Contract

This RFC recommends the following MCP tools for scaffolding workflows.

---

## 16.1 `draft_service_scaffold_request`

### Purpose
Draft a structured request from conversational or partial input.

### Input
Free-form or partial structured input.

### Output
A partial or complete `ServiceScaffoldRequest`.

### Notes
This tool is optional but useful for agent workflows.

---

## 16.2 `validate_service_scaffold_request`

### Purpose
Validate a `ServiceScaffoldRequest` before compilation or submission.

### Input

```json
{
  "request": { "...": "ServiceScaffoldRequest" }
}
```

### Output

```json
{
  "ok": true,
  "status": "valid_with_warnings",
  "errors": [],
  "warnings": [
    {
      "code": "MISSING_CATALOG_DOMAIN",
      "message": "catalog.domain was not set; default may be applied"
    }
  ],
  "normalizedRequest": {},
  "derived": {
    "serviceName": "payments-api",
    "repoName": "payments-api",
    "defaultVisibility": "internal"
  }
}
```

### Responsibilities
- schema validation,
- semantic validation,
- policy validation,
- return normalized/defaulted form when possible.

---

## 16.3 `list_service_templates`

### Purpose
List available scaffolding templates visible to the caller or tenant.

### Input

Optional filters such as:
- tenant
- owner
- language
- visibility

### Output

A list of template summaries.

Example:

```json
{
  "templates": [
    {
      "id": "go-grpc",
      "displayName": "Go gRPC Service",
      "allowedVisibilities": ["internal", "private"],
      "requiredParameters": ["goVersion"]
    }
  ]
}
```

---

## 16.4 `get_service_template`

### Purpose
Return detailed metadata for one template.

### Input

```json
{
  "templateId": "go-grpc"
}
```

### Output
Detailed template metadata, defaults, parameter schema references, and policy notes.

---

## 16.5 `compile_service_scaffold_request`

### Purpose
Compile a validated request into a deterministic execution plan.

### Input

```json
{
  "request": { "...": "ServiceScaffoldRequest" }
}
```

### Output

A `CompiledScaffoldPlan` containing:
- normalized request,
- selected template,
- derived values,
- approval/policy signals,
- Release Engine job payload.

Example:

```json
{
  "ok": true,
  "plan": {
    "requestHash": "sha256:...",
    "normalizedRequest": {},
    "resolvedTemplate": {
      "id": "go-grpc"
    },
    "derived": {
      "serviceName": "payments-api",
      "repoName": "payments-api"
    },
    "job": {
      "pathKey": "platform/scaffolding/create-service",
      "params": {
        "contract_version": "scaffolding-create-service/v1",
        "template": "go-grpc",
        "service_name": "payments-api",
        "owner": "team-payments",
        "org": "acme-platform",
        "visibility": "internal",
        "parameters": {
          "goVersion": "1.24",
          "enableDocker": true
        }
      }
    }
  }
}
```

---

## 16.6 `submit_service_scaffold_request`

### Purpose
Submit a compiled or raw request to Release Engine.

### Input

Either:
- a raw `ServiceScaffoldRequest`, or
- a previously compiled plan.

Recommended contract:
- if raw request is submitted, the server must validate and compile before submission.

### Output

```json
{
  "ok": true,
  "requestId": "scaf_01HXYZ...",
  "jobId": "job_12345",
  "status": "submitted"
}
```

---

## 16.7 `get_service_scaffold_request_status`

### Purpose
Return current execution state for a submitted request.

### Input

```json
{
  "requestId": "scaf_01HXYZ..."
}
```

### Output

```json
{
  "ok": true,
  "requestId": "scaf_01HXYZ...",
  "jobId": "job_12345",
  "status": "running",
  "step": "register-component",
  "result": null
}
```

Possible statuses:
- `draft`
- `validated`
- `compiled`
- `submitted`
- `running`
- `succeeded`
- `failed`
- `cancelled`
- `manual_review_required`

---

## 17. Shared MCP Response Envelope

All scaffolding MCP tools should use a consistent envelope.

Recommended shape:

```json
{
  "ok": true,
  "status": "valid",
  "errors": [],
  "warnings": [],
  "data": {}
}
```

### 17.1 Fields

#### `ok`
Boolean indicating whether the operation succeeded.

#### `status`
Operation-specific status value.

#### `errors`
Structured diagnostics.

#### `warnings`
Structured non-fatal diagnostics.

#### `data`
Operation payload.

### 17.2 Diagnostic Shape

```json
{
  "code": "INVALID_TEMPLATE",
  "message": "Template 'go-unknown' is not supported",
  "field": "spec.template",
  "severity": "error"
}
```

Recommended diagnostic fields:
- `code`
- `message`
- `field`
- `severity`
- `hint`

---

## 18. Compilation Contract

The compiler transforms a `ServiceScaffoldRequest` into a Release Engine job request.

## 18.1 Output Requirements

Compilation must produce:

- deterministic normalized request,
- stable request hash,
- stable idempotency key,
- explicit path key,
- explicit param map,
- compiler metadata.

## 18.2 Recommended Job Shape

```yaml
tenant_id: acme
path_key: platform/scaffolding/create-service
idempotency_key: scaffold-payments-api-3f67a2b1
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
    enableOpenAPI: true
  callback_url: https://backstage.example/callback
```

---

## 19. Normalization Rules

Recommended normalization behavior:

- trim surrounding whitespace from scalar strings,
- canonicalize enum casing,
- default `visibility` when absent,
- derive `service.name` from `metadata.name` when absent,
- derive repository name from normalized service name,
- inject template default parameters where absent,
- preserve explicitly provided values over defaults,
- sort map keys for stable hashing where applicable.

---

## 20. Idempotency

Submissions should be idempotent for equivalent compiled intent.

Recommended sources for idempotency key:
- tenant
- org
- template
- normalized service name
- normalized owner
- normalized effective parameters

Equivalent requests should compile to equivalent idempotency keys.

This allows safe retries and prevents accidental duplicate repository creation.

---

## 21. Error Model

Recommended error categories:

- `INVALID_REQUEST`
- `INVALID_SCHEMA`
- `INVALID_TEMPLATE`
- `INVALID_OWNER`
- `INVALID_ORG`
- `INVALID_VISIBILITY`
- `INVALID_TEMPLATE_PARAMETERS`
- `POLICY_VIOLATION`
- `APPROVAL_REQUIRED`
- `COMPILATION_FAILED`
- `SUBMISSION_FAILED`
- `STATUS_NOT_FOUND`

Errors should clearly distinguish:

- malformed requests,
- unsupported requests,
- policy-blocked requests,
- transient backend failures.

---

## 22. Security and Policy Considerations

### 22.1 Template Allowlisting
Only platform-approved templates may be used.

### 22.2 Owner Validation
Owner references should be checked against authoritative sources where configured.

### 22.3 Callback URL Controls
Callback targets may need an allowlist to prevent data exfiltration.

### 22.4 Visibility Controls
Public repositories may require stricter review or approval.

### 22.5 Tenant Isolation
Templates, orgs, and policies may vary by tenant and must be evaluated in tenant context.

---

## 23. Backward Compatibility

The request contract is versioned by:

- `apiVersion` for the intent resource,
- `contract_version` for the compiled module param dialect.

This allows:
- intent evolution without breaking old clients,
- module param evolution without silently breaking compilers.

Initial compiled contract version:

```text
scaffolding-create-service/v1
```

---

## 24. Testing Implications

This RFC implies the need for:

- request fixtures,
- schema fixtures,
- valid/invalid test cases,
- normalization tests,
- compile golden tests,
- submission envelope tests.

At minimum, tests should prove:
- deterministic compilation,
- stable idempotency keys,
- correct defaults,
- clear diagnostics.

---

## 25. Example End-to-End Flow

### Step 1 — draft

Agent drafts:

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
```

### Step 2 — validate

MCP returns:
- valid,
- visibility defaults to `internal`,
- service name resolves to `payments-api`.

### Step 3 — compile

MCP returns:

```yaml
path_key: platform/scaffolding/create-service
params:
  contract_version: scaffolding-create-service/v1
  template: go-grpc
  service_name: payments-api
  owner: team-payments
  org: acme-platform
  visibility: internal
  parameters: {}
```

### Step 4 — submit

MCP submits Release Engine job.

### Step 5 — status

Caller receives:
- running → succeeded,
- plus output such as repository URL and catalog entity ref.

---

## 26. Recommended MCP Tool Set

The recommended baseline scaffolding tools are:

1. `draft_service_scaffold_request`
2. `validate_service_scaffold_request`
3. `list_service_templates`
4. `get_service_template`
5. `compile_service_scaffold_request`
6. `submit_service_scaffold_request`
7. `get_service_scaffold_request_status`

Optional future tools:

8. `estimate_service_scaffold_outcome`
9. `approve_service_scaffold_request`
10. `cancel_service_scaffold_request`

---

## 27. Open Questions

The following are intentionally deferred:

1. Should owner validation be hard-fail or warning-only in some tenants?
2. Should `metadata.name` and `spec.service.name` both remain, or should one be canonical?
3. Should public repository requests always require approval?
4. Should callback URLs remain caller-specified, or be abstracted behind internal event routing?
5. Should template parameter schemas be embedded or referenced externally?
6. Should repo existence checks happen during validation or at execution time only?

---

## 28. Decision

This RFC proposes that service scaffolding requests be modeled as a first-class intent resource, handled through MCP tools that validate and compile requests into the Release Engine module `scaffolding/create-service`.

This preserves:
- a clean agent-facing API,
- deterministic workflow execution,
- strong validation boundaries,
- and a scalable pattern parallel to infrastructure provisioning.

---
