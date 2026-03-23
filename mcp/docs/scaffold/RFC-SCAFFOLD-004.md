# RFC-SCAFFOLD-004 — Fixtures and Golden Tests

**Status:** Draft  
**Authors:** Platform Engineering  
**Audience:** Platform team, Release Engine maintainers, MCP server implementers, QA / test automation maintainers  
**Last Updated:** March 15, 2026

---

## 1. Summary

This RFC defines the **fixtures, golden tests, and regression strategy** for the scaffolding flow introduced in:

- **RFC-SCAFFOLD-001** — Scaffolding intent model and MCP contract
- **RFC-SCAFFOLD-002** — Template catalog and validation rules
- **RFC-SCAFFOLD-003** — Compile-to-Release-Engine mapping

The purpose of this RFC is to make the scaffolding system:

- deterministic,
- testable,
- reviewable,
- resistant to accidental contract drift,
- and safe to evolve over time.

This RFC standardizes:

- fixture categories,
- directory layout,
- fixture naming rules,
- expected outputs,
- golden file generation strategy,
- regression review rules,
- and CI expectations.

---

## 2. Problem Statement

The scaffolding flow now includes several layers of behavior:

1. request parsing,
2. request normalization,
3. catalog resolution,
4. validation,
5. defaulting,
6. compilation,
7. idempotency derivation,
8. submission envelope generation.

That means behavior can drift in subtle ways even when individual units appear correct.

Examples of risky drift include:

- a new template default changing compiled output unexpectedly,
- hash inputs changing due to field ordering changes,
- validation diagnostics changing shape,
- optional fields being emitted differently,
- idempotency keys changing unintentionally,
- callback handling changing without review.

Unit tests alone are not sufficient to catch these integration-level changes.

We therefore need a fixture and golden strategy that captures the **observable contract** of the system.

---

## 3. Goals

### 3.1 Primary Goals

This RFC aims to:

1. define a **canonical fixture structure**,
2. define **golden outputs** for normalization, validation, and compilation,
3. support deterministic regression testing,
4. make behavior changes visible in code review,
5. enable template-by-template and request-by-request test coverage,
6. support future contract versioning with minimal ambiguity.

### 3.2 Non-Goals

This RFC does **not** define:

- end-to-end live integration tests against real SCM providers,
- connector acceptance tests,
- performance benchmarking,
- approval workflow simulation,
- or UI snapshot testing.

---

## 4. Design Principles

### 4.1 Determinism First

Tests should validate stable outputs from stable inputs.

### 4.2 Contract-Focused

Goldens should capture externally meaningful behavior, not arbitrary internal implementation details.

### 4.3 Minimal Surprise

Any change to validation or compilation behavior should be explicit in diffs.

### 4.4 Layered Testing

Use both:
- focused unit tests for logic, and
- golden tests for end-to-end transformation behavior.

### 4.5 Human Reviewability

Fixture and golden files should be easy to read, diff, and update.

---

## 5. Test Scope

This RFC covers tests for the following layers:

### 5.1 Request Layer
- parsing request fixtures,
- required field presence,
- shape validation,
- type validation.

### 5.2 Catalog Layer
- template resolution,
- template eligibility,
- default application,
- schema validation,
- restricted visibility / owner / org checks.

### 5.3 Normalization Layer
- derived defaults,
- canonical field population,
- normalized output determinism.

### 5.4 Validation Layer
- warnings,
- errors,
- policy outcomes,
- deprecation behavior,
- manual review indicators if supported.

### 5.5 Compilation Layer
- module param mapping,
- derived fields,
- request hash generation,
- idempotency key generation,
- submission envelope generation.

---

## 6. What Counts as Contract Surface

The following outputs are considered **contract surfaces** and should be covered by goldens where applicable:

- normalized request,
- validation diagnostics,
- compiled plan,
- submission preview,
- request hash projection,
- idempotency key,
- effective template resolution metadata.

Internal helper function outputs are **not** golden-worthy unless they are directly part of a stable public or inter-component contract.

---

## 7. Fixture Model

A fixture represents a complete test scenario.

Each fixture should describe:

- input request,
- catalog context,
- policy context,
- expected validation result,
- expected normalized request,
- expected compiled output when valid.

### Fixture philosophy

A fixture should answer:

> “Given this request and this catalog/policy state, what exact outcome should the platform produce?”

---

## 8. Recommended Directory Layout

```text
testdata/
  scaffolding/
    catalogs/
      base/
        catalog.yaml
      deprecated-template/
        catalog.yaml
      restricted-owners/
        catalog.yaml
      visibility-policy/
        catalog.yaml

    requests/
      valid/
        minimal-go-grpc.yaml
        full-go-grpc.yaml
        internal-defaulted.yaml
        callback-enabled.yaml
      invalid/
        missing-owner.yaml
        invalid-template.yaml
        forbidden-visibility.yaml
        owner-not-allowed.yaml
        bad-parameter-type.yaml
        conflicting-service-name.yaml

    fixtures/
      minimal-go-grpc/
        input.request.yaml
        context.catalog-ref.txt
        expected.normalized.yaml
        expected.validation.json
        expected.compiled.yaml
        expected.submission.yaml
      full-go-grpc/
        input.request.yaml
        context.catalog-ref.txt
        expected.normalized.yaml
        expected.validation.json
        expected.compiled.yaml
        expected.submission.yaml
      missing-owner/
        input.request.yaml
        context.catalog-ref.txt
        expected.validation.json
      forbidden-visibility/
        input.request.yaml
        context.catalog-ref.txt
        expected.validation.json
```

This structure is intentionally simple and file-system friendly.

---

## 9. Fixture Types

There are two good patterns. Either may be used.

### Option A — Multi-file scenario directories
Each scenario has one directory with input and expected outputs.

### Option B — Paired request/golden files
Requests and expected outputs are stored in parallel directories.

### Recommendation

Use **Option A** for readability and scenario isolation.

---

## 10. Standard Fixture Files

Each fixture directory should support the following files.

### Required

- `input.request.yaml`
- `context.catalog-ref.txt`
- `expected.validation.json`

### Optional, depending on validity

- `expected.normalized.yaml`
- `expected.compiled.yaml`
- `expected.submission.yaml`
- `expected.hash.json`

### Optional future extensions

- `context.policy.yaml`
- `expected.approval.json`
- `notes.md`

---

## 11. Fixture Context

A request does not exist in isolation. Results depend on context.

### Minimum context dimensions

- catalog definition/version,
- tenant,
- policy flags,
- contract version,
- platform defaults.

### Recommended context representation

Use a small explicit file when context varies materially.

Example:

```yaml
tenant: acme
contractVersion: scaffolding-create-service/v1
platformDefaults:
  visibility: internal
callbackPolicy:
  mode: allow-listed
  relayEnabled: true
```

If a shared default context exists, it may be implicit for most fixtures.

---

## 12. Golden Output Types

---

## 12.1 Validation Golden

Captures the full validation outcome.

### Recommended fields

- `valid`
- `errors`
- `warnings`
- `template`
- `effectiveDefaults`
- `manualReviewRequired` if supported

### Example

```json
{
  "valid": true,
  "errors": [],
  "warnings": [],
  "template": {
    "id": "go-grpc",
    "version": "1.0.0",
    "status": "active"
  },
  "effectiveDefaults": {
    "visibility": "internal",
    "parameters": {
      "enableDocker": true,
      "enableOpenAPI": false
    }
  }
}
```

---

## 12.2 Normalized Request Golden

Captures the exact normalized request after:
- defaulting,
- canonicalization,
- and derivation.

### Example

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
```

---

## 12.3 Compiled Plan Golden

Captures the exact compiled plan contract.

### Example

```yaml
apiVersion: platform.gatblau.io/v1alpha1
kind: CompiledServiceScaffoldPlan
metadata:
  name: payments-api
spec:
  contract:
    moduleKey: scaffolding/create-service
    contractVersion: scaffolding-create-service/v1
  template:
    id: go-grpc
    version: 1.0.0
  derived:
    serviceName: payments-api
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
```

---

## 12.4 Submission Golden

Captures the exact Release Engine submission preview.

### Example

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
metadata:
  tenant: acme
  idempotency_key: scaffold-service:acme:acme-platform:payments-api:go-grpc:2fd2a1d4
```

---

## 12.5 Hash Golden

Captures deterministic hashes and idempotency values.

### Example

```json
{
  "normalizedRequestSha256": "2fd2a1d4d9c1d9d10ab3c2d1...",
  "compiledPlanSha256": "a81e9c1191f83d2d87a9ab12...",
  "idempotencyKey": "scaffold-service:acme:acme-platform:payments-api:go-grpc:2fd2a1d4"
}
```

This is especially useful for ensuring changes to canonical serialization are reviewed intentionally.

---

## 13. Normative Serialization Rules for Goldens

Golden tests are only useful if serialization is stable.

### Required rules

- stable object key ordering,
- stable map key ordering,
- omit non-deterministic timestamps,
- omit runtime-generated IDs unless specifically under test,
- use consistent YAML/JSON formatting,
- preserve null/empty handling rules consistently.

### Recommendation

- Use **YAML** for structural outputs intended for humans.
- Use **JSON** for diagnostics and hashes where strict shape matters.

---

## 14. Scenario Matrix

The test suite should cover a representative scenario matrix.

---

## 14.1 Valid Scenarios

At minimum:

1. **minimal valid request**
2. **fully populated request**
3. **defaults-only request**
4. **callback-enabled request**
5. **explicit visibility override**
6. **deprecated template with warning**
7. **equivalent request representations produce same hash**
8. **owner/org allowed by template restriction**
9. **template with enum parameter**
10. **template with optional parameter omitted**

---

## 14.2 Invalid Scenarios

At minimum:

1. missing `spec.template`
2. missing `spec.owner`
3. missing `spec.org`
4. unknown template
5. disabled template
6. forbidden owner
7. forbidden org
8. forbidden visibility
9. bad parameter type
10. missing required parameter
11. enum violation
12. pattern violation
13. additional unknown parameter if not allowed
14. mismatched `metadata.name` and `spec.service.name`
15. callback URL not permitted
16. compile contract unsupported

---

## 14.3 Edge Scenarios

Strongly recommended:

1. empty parameters map
2. parameters supplied in different order
3. explicit default value vs omitted value
4. deprecated but still allowed template
5. tenant-specific template access
6. service name with boundary-valid characters
7. longest allowed service name
8. callback omitted when relay disabled
9. same request across different tenants yields different idempotency key
10. same request with different template version yields different hash if version is included

---

## 15. Validation Diagnostic Contract

Validation outputs should be testable as structured data.

### Recommended diagnostic shape

```json
{
  "code": "VALIDATION_OWNER_NOT_ALLOWED",
  "message": "Owner 'team-unknown' is not allowed for template 'go-grpc'",
  "field": "spec.owner",
  "severity": "error"
}
```

### Diagnostic fields to stabilize

- `code`
- `field`
- `severity`

### Diagnostic fields that may be more flexible

- `message` wording, if desired

### Recommendation

Treat `code`, `field`, and `severity` as strict-match contract fields.  
Treat `message` as strict-match too unless there is a strong reason to avoid it.

---

## 16. Golden Comparison Rules

Different outputs may require different matching strictness.

### Strict comparison recommended for:
- normalized request,
- compiled plan,
- submission envelope,
- hashes,
- idempotency key,
- validation codes and fields.

### Flexible comparison may be acceptable for:
- ordering of warning arrays, if explicitly declared unordered,
- human-readable message wording, if kept out of strict contract.

### Recommendation

Prefer **strict comparison everywhere practical**.

It is easier to intentionally update a golden than to debug vague matching rules.

---

## 17. Golden Update Workflow

Golden files will need updates when behavior intentionally changes.

### Recommended workflow

1. make implementation change,
2. run golden tests,
3. inspect diffs,
4. update goldens with explicit command,
5. include rationale in PR description.

### Example command

```bash
go test ./... -update-goldens
```

or

```bash
make test-goldens-update
```

### Requirement

Updating goldens should be a conscious action, never the default.

---

## 18. CI Expectations

The CI pipeline should run:

1. unit tests,
2. fixture/golden tests,
3. schema validation on fixture files,
4. optional linting for fixture consistency.

### CI should fail when:
- fixture inputs are malformed,
- expected outputs are missing,
- actual outputs differ from goldens,
- serialization is not deterministic,
- or stale fixtures reference missing catalogs.

---

## 19. Recommended Go Test Structure

```text
internal/scaffolding/
  normalize/
    normalize_test.go
  validate/
    validate_test.go
  compile/
    compile_test.go
  golden/
    fixture_runner_test.go
```

### Suggested test responsibilities

- `normalize_test.go`: unit tests for defaulting/canonicalization
- `validate_test.go`: rule-specific validation tests
- `compile_test.go`: direct compile contract tests
- `fixture_runner_test.go`: end-to-end fixture execution and golden comparison

---

## 20. Fixture Runner Behavior

A fixture runner should:

1. load scenario files,
2. load referenced catalog/context,
3. parse request,
4. normalize and validate,
5. compare validation output,
6. if valid, compile,
7. compare normalized, compiled, submission, and hash outputs.

### Pseudocode

```go
for _, fx := range fixtures {
    req := loadRequest(fx)
    ctx := loadContext(fx)

    result := RunScaffoldingPipeline(ctx, req)

    assertEqualGolden(fx.Validation, result.Validation)

    if result.Validation.Valid {
        assertEqualGolden(fx.Normalized, result.NormalizedRequest)
        assertEqualGolden(fx.Compiled, result.CompiledPlan)
        assertEqualGolden(fx.Submission, result.SubmissionPreview)
        assertEqualGolden(fx.Hash, result.Hashes)
    }
}
```

---

## 21. Example Fixture

### Directory

```text
testdata/scaffolding/fixtures/minimal-go-grpc/
```

### `input.request.yaml`

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
```

### `context.catalog-ref.txt`

```text
catalogs/base/catalog.yaml
```

### `expected.validation.json`

```json
{
  "valid": true,
  "errors": [],
  "warnings": [],
  "template": {
    "id": "go-grpc",
    "version": "1.0.0",
    "status": "active"
  }
}
```

### `expected.normalized.yaml`

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
```

### `expected.compiled.yaml`

```yaml
apiVersion: platform.gatblau.io/v1alpha1
kind: CompiledServiceScaffoldPlan
metadata:
  name: payments-api
spec:
  template:
    id: go-grpc
    version: 1.0.0
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
```

### `expected.hash.json`

```json
{
  "idempotencyKey": "scaffold-service:acme:acme-platform:payments-api:go-grpc:2fd2a1d4"
}
```

---

## 22. Catalog Fixture Strategy

Catalog fixtures should also be versioned and reviewed.

### Recommended catalog fixture categories

- `base`
- `deprecated-template`
- `disabled-template`
- `restricted-owner`
- `restricted-org`
- `enum-heavy`
- `pattern-validation`
- `callback-policy-sensitive`

### Rule

Avoid a single mega-catalog for all tests.  
Use smaller targeted catalogs where possible to reduce ambiguity.

---

## 23. Hash and Idempotency Testing Strategy

Hashing and idempotency are particularly prone to accidental drift.

### Required tests

- same semantic request → same normalized hash,
- reordered parameters → same normalized hash,
- explicit defaults vs omitted defaults → same normalized hash,
- different tenant → different idempotency key,
- different template version → hash changes if version participates,
- different contract version → hash changes.

### Important note

If hashes are included in goldens, the canonicalization algorithm becomes part of the reviewed contract.  
That is desirable, but only if the team accepts that such changes will require intentional migration.

---

## 24. Backward Compatibility Testing

When new contract versions are introduced, goldens should be organized by version.

### Recommended layout

```text
testdata/scaffolding/contracts/
  scaffolding-create-service-v1/
    fixtures/...
  scaffolding-create-service-v2/
    fixtures/...
```

This prevents accidental mixing of expectations between contract versions.

---

## 25. Review Guidance for Golden Diffs

Golden diffs should be treated as meaningful product changes.

### Reviewers should ask:

- Did validation behavior change intentionally?
- Did defaults change?
- Did template version resolution change?
- Did compiled param names change?
- Did callback behavior change?
- Did hashes or idempotency keys drift?
- Is this a bug fix, policy change, or contract break?

### Recommendation

PRs that modify goldens should include a short section:

```text
Golden diff rationale:
- changed default visibility from private to internal
- added enableOpenAPI default=false to go-grpc template
- hash changed because normalized request projection now includes contractVersion
```

---

## 26. Security Considerations

### 26.1 No Secrets in Goldens
Fixture inputs and expected outputs must not contain real tokens, credentials, or internal secret URLs.

### 26.2 Safe Callback Examples
Use obviously fake or internal-example callback URLs only.

### 26.3 Tenant Isolation
Fixtures should cover cross-tenant separation logic, but with synthetic tenant names.

### 26.4 Policy Visibility
Security-relevant validation rules should have explicit fixtures so accidental weakening is caught by CI.

---

## 27. Open Questions

1. Should validation messages be strict golden content, or only codes/fields?
2. Should compiled timestamps be entirely omitted, or normalized to placeholders?
3. Should hash goldens include full SHA-256 values or shortened prefixes only?
4. Should fixtures support matrix expansion to reduce duplication?
5. Should policy context be fully explicit per fixture, or inherited from suite defaults?
6. Should end-to-end submission previews include platform metadata fields that may vary by environment?

---

## 28. Decision

This RFC proposes that the scaffolding implementation include a **fixture-driven golden test suite** covering:

- validation,
- normalization,
- compilation,
- submission preview,
- hashing,
- and idempotency behavior.

This ensures that the scaffolding contract remains:

- deterministic,
- reviewable,
- and safe to evolve.

---

## 29. Next Steps

Recommended implementation sequence:

1. create `testdata/scaffolding/fixtures/`
2. add 5–10 baseline fixtures
3. implement fixture runner
4. add golden update command
5. enforce CI golden checks
6. expand scenario matrix as templates and policies grow

---

## 30. Next RFCs

Suggested follow-ons:

### RFC-SCAFFOLD-005 — Approval and Policy Gate Model
Defines:
- approval-required states,
- policy outcomes,
- manual review contracts,
- MCP approval tool behavior.

### RFC-SCAFFOLD-006 — Execution Status and Outcome Model
Defines:
- scaffold job lifecycle,
- outcome normalization,
- polling and event contracts,
- catalog/repo creation result reporting.

---
