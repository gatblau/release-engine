# Phase 4 — Cross-Cutting Concern Specifications

## Table of Contents

- [SPEC: Authentication and Authorisation](#spec-authentication-and-authorisation)
- [SPEC: Error Handling](#spec-error-handling)
- [SPEC: Logging](#spec-logging)
- [SPEC: Metrics](#spec-metrics)
- [SPEC: Tracing](#spec-tracing)
- [SPEC: Configuration](#spec-configuration)
- [SPEC: Database Migrations](#spec-database-migrations)
- [SPEC: Health Checks](#spec-health-checks)
- [SPEC: Rate Limiting](#spec-rate-limiting)
- [SPEC: Pagination](#spec-pagination)
- [SPEC: CORS](#spec-cors)
- [SPEC: Input Validation](#spec-input-validation)
- [SPEC: Graceful Shutdown](#spec-graceful-shutdown)

### SPEC: Authentication and Authorisation

**File:** `internal/crosscut/authentication-and-authorisation.md`
**Package:** `crosscut`
**Phase:** 4
**Dependencies:** ConfigLoader, HTTPServer, DBPool, observability stack

---

#### Purpose

Token format, validation flow, RBAC model, and permission checks for `job:create`, `job:read`, and `job:cancel` actions.

---

#### Shared Context (duplicated for self-containment)

- Error envelope: `{error, code, details}`
- Tenant isolation key: `tenant_id`
- Correlation key: `request_id`
- Ownership fence keys: `run_id`, `effect_id`

---

#### Public Interface

```text
Policy is applied across all relevant components through middleware, service wrappers, and shared helper packages.
```

##### Example — Authentication and Authorisation

**Request:**
```json
{"tenant_id":"acme-prod","request_id":"r-123"}
```

**Response (compliant):**
```json
{"status":"compliant"}
```

**Response (violation):**
```json
{"error":"policy violation","code":"POLICY_VIOLATION","details":null}
```

---

#### Internal Logic (step-by-step)

1. Evaluate incoming request or runtime event against this concern's policy.
2. Enforce deterministic action (allow, deny, retry, or terminate).
3. Persist required state transitions and audit evidence.
4. Emit logs, metrics, and traces with standard fields.
5. Approval lifecycle metrics MUST be emitted at runtime hooks:
   - request created (`re_approval_requests_total`)
   - decision recorded (`re_approval_decisions_total`)
   - decision latency observed (`re_approval_latency_seconds`)
   - escalation (`re_approval_escalations_total`)
   - timeout (`re_approval_timeouts_total`)
   - worker poll duration (`re_approval_worker_tick_duration_seconds`)

Approval-decision guardrails are enforced for `POST /v1/jobs/{job_id}/steps/{step_id}/decisions`:

- Role allow-list (`allowed_roles`) is matched case-insensitively.
- Self-approval is denied when `self_approval=false` and `approver == job_owner`.
- Tenant scope is denied when `approver_tenant_id != job_tenant_id`.
- Optional budget authority is denied when `estimated_cost > approver_limit`.
- Four-eyes progression is enforced after authorisation: the step remains `waiting_approval` until `approved_count >= min_approvers`.
- Approval wait TTL is mandatory and resolved as step override → path policy default → system default (`48h`).
- At `escalation_at * ttl` (default 80%), emit `approval_escalated` without changing step state.
- At expiry (`now() >= approval_expires_at`), transition step to `error` with `approval_timeout`, record system decision `expired`, and emit `approval_expired`.
- Outbox emission contract for approval lifecycle events:
  - Allowed types: `approval_requested`, `approval_decided`, `approval_escalated`, `approval_expired`.
  - Event type must be registered in `OutboxDispatcher` before enqueue.
  - Unregistered event types are rejected and not persisted.
  - Event payloads are immutable once accepted.

---

#### Data Model (if this component owns a table)

Uses shared tables (`jobs`, `jobs_read`, `outbox`, `external_effects`, `idempotency_keys`, `audit_log`) with no additional table required in this phase.

---

#### Error Table

| Condition | HTTP Status | Error Code | Response Body |
|---|---|---|---|
| Policy or validation violation | 400 | `POLICY_VIOLATION` | `{"error":"policy violation","code":"POLICY_VIOLATION","details":null}` |
| Runtime dependency unavailable | 503 | `SERVICE_UNAVAILABLE` | `{"error":"service unavailable","code":"SERVICE_UNAVAILABLE","details":null}` |

---

#### Acceptance Criteria (Gherkin)

```gherkin
Feature: Authentication and Authorisation

  Scenario: Happy path
    Given valid request and dependencies
    When policy is evaluated
    Then compliant behaviour is enforced

  Scenario: Edge case
    Given replayed or duplicated input
    When policy is evaluated again
    Then output remains deterministic

  Scenario: Error
    Given runtime dependency failure
    When policy is evaluated
    Then structured error response is returned
```

---

#### Performance Targets

| Metric | Target |
|---|---|
| p95 evaluation latency | <20ms |
| policy enforcement overhead | <5% of request latency budget |

---

#### Security Considerations

- No secrets in logs, traces, or error bodies.
- Explicit tenant scoping for all checks.
- Deny by default when policy engine state is unavailable.

---

#### Observability

- **Log events:** `crosscut.authentication-and-authorisation.applied`, `crosscut.authentication-and-authorisation.rejected`
- **Metrics:** `crosscut_authentication_and_authorisation_total{status}`
- **Trace span:** `crosscut.authentication-and-authorisation`

---

#### Cross-Component Interactions

| Direction | Component | Mechanism | Data Exchanged |
|---|---|---|---|
| This → HTTP middleware | in-process | request interception | headers, claims, request metadata |
| This → DBPool | SQL | policy/audit persistence | decision, timestamps, tenant_id |
| This → Observability | in-process | metrics/traces/logs | event labels, durations, outcome |

### SPEC: Error Handling

**File:** `internal/crosscut/error-handling.md`
**Package:** `crosscut`
**Phase:** 4
**Dependencies:** ConfigLoader, HTTPServer, DBPool, observability stack

---

#### Purpose

Standard error envelope, explicit error code registry, and panic recovery behaviour.

---

#### Shared Context (duplicated for self-containment)

- Error envelope: `{error, code, details}`
- Tenant isolation key: `tenant_id`
- Correlation key: `request_id`
- Ownership fence keys: `run_id`, `effect_id`

---

#### Public Interface

```text
Policy is applied across all relevant components through middleware, service wrappers, and shared helper packages.
```

##### Example — Error Handling

**Request:**
```json
{"tenant_id":"acme-prod","request_id":"r-123"}
```

**Response (compliant):**
```json
{"status":"compliant"}
```

**Response (violation):**
```json
{"error":"policy violation","code":"POLICY_VIOLATION","details":null}
```

---

#### Internal Logic (step-by-step)

1. Evaluate incoming request or runtime event against this concern's policy.
2. Enforce deterministic action (allow, deny, retry, or terminate).
3. Persist required state transitions and audit evidence.
4. Emit logs, metrics, and traces with standard fields.

---

#### Data Model (if this component owns a table)

Uses shared tables (`jobs`, `jobs_read`, `outbox`, `external_effects`, `idempotency_keys`, `audit_log`) with no additional table required in this phase.

---

#### Error Table

| Condition | HTTP Status | Error Code | Response Body |
|---|---|---|---|
| Policy or validation violation | 400 | `POLICY_VIOLATION` | `{"error":"policy violation","code":"POLICY_VIOLATION","details":null}` |
| Runtime dependency unavailable | 503 | `SERVICE_UNAVAILABLE` | `{"error":"service unavailable","code":"SERVICE_UNAVAILABLE","details":null}` |

---

#### Acceptance Criteria (Gherkin)

```gherkin
Feature: Error Handling

  Scenario: Happy path
    Given valid request and dependencies
    When policy is evaluated
    Then compliant behaviour is enforced

  Scenario: Edge case
    Given replayed or duplicated input
    When policy is evaluated again
    Then output remains deterministic

  Scenario: Error
    Given runtime dependency failure
    When policy is evaluated
    Then structured error response is returned
```

---

#### Performance Targets

| Metric | Target |
|---|---|
| p95 evaluation latency | <20ms |
| policy enforcement overhead | <5% of request latency budget |

---

#### Security Considerations

- No secrets in logs, traces, or error bodies.
- Explicit tenant scoping for all checks.
- Deny by default when policy engine state is unavailable.

---

#### Observability

- **Log events:** `crosscut.error-handling.applied`, `crosscut.error-handling.rejected`
- **Metrics:** `crosscut_error_handling_total{status}`
- **Trace span:** `crosscut.error-handling`

---

#### Cross-Component Interactions

| Direction | Component | Mechanism | Data Exchanged |
|---|---|---|---|
| This → HTTP middleware | in-process | request interception | headers, claims, request metadata |
| This → DBPool | SQL | policy/audit persistence | decision, timestamps, tenant_id |
| This → Observability | in-process | metrics/traces/logs | event labels, durations, outcome |

### SPEC: Logging

**File:** `internal/crosscut/logging.md`
**Package:** `crosscut`
**Phase:** 4
**Dependencies:** ConfigLoader, HTTPServer, DBPool, observability stack

---

#### Purpose

Structured JSON format, required fields, correlation ID propagation, and log-level policy.

---

#### Shared Context (duplicated for self-containment)

- Error envelope: `{error, code, details}`
- Tenant isolation key: `tenant_id`
- Correlation key: `request_id`
- Ownership fence keys: `run_id`, `effect_id`

---

#### Public Interface

```text
Policy is applied across all relevant components through middleware, service wrappers, and shared helper packages.
```

##### Example — Logging

**Request:**
```json
{"tenant_id":"acme-prod","request_id":"r-123"}
```

**Response (compliant):**
```json
{"status":"compliant"}
```

**Response (violation):**
```json
{"error":"policy violation","code":"POLICY_VIOLATION","details":null}
```

---

#### Internal Logic (step-by-step)

1. Evaluate incoming request or runtime event against this concern's policy.
2. Enforce deterministic action (allow, deny, retry, or terminate).
3. Persist required state transitions and audit evidence.
4. Emit logs, metrics, and traces with standard fields.

---

#### Data Model (if this component owns a table)

Uses shared tables (`jobs`, `jobs_read`, `outbox`, `external_effects`, `idempotency_keys`, `audit_log`) with no additional table required in this phase.

---

#### Error Table

| Condition | HTTP Status | Error Code | Response Body |
|---|---|---|---|
| Policy or validation violation | 400 | `POLICY_VIOLATION` | `{"error":"policy violation","code":"POLICY_VIOLATION","details":null}` |
| Runtime dependency unavailable | 503 | `SERVICE_UNAVAILABLE` | `{"error":"service unavailable","code":"SERVICE_UNAVAILABLE","details":null}` |

---

#### Acceptance Criteria (Gherkin)

```gherkin
Feature: Logging

  Scenario: Happy path
    Given valid request and dependencies
    When policy is evaluated
    Then compliant behaviour is enforced

  Scenario: Edge case
    Given replayed or duplicated input
    When policy is evaluated again
    Then output remains deterministic

  Scenario: Error
    Given runtime dependency failure
    When policy is evaluated
    Then structured error response is returned
```

---

#### Performance Targets

| Metric | Target |
|---|---|
| p95 evaluation latency | <20ms |
| policy enforcement overhead | <5% of request latency budget |

---

#### Security Considerations

- No secrets in logs, traces, or error bodies.
- Explicit tenant scoping for all checks.
- Deny by default when policy engine state is unavailable.

---

#### Observability

- **Log events:** `crosscut.logging.applied`, `crosscut.logging.rejected`
- **Metrics:** `crosscut_logging_total{status}`
- **Trace span:** `crosscut.logging`

---

#### Cross-Component Interactions

| Direction | Component | Mechanism | Data Exchanged |
|---|---|---|---|
| This → HTTP middleware | in-process | request interception | headers, claims, request metadata |
| This → DBPool | SQL | policy/audit persistence | decision, timestamps, tenant_id |
| This → Observability | in-process | metrics/traces/logs | event labels, durations, outcome |

### SPEC: Metrics

**File:** `internal/crosscut/metrics.md`
**Package:** `crosscut`
**Phase:** 4
**Dependencies:** ConfigLoader, HTTPServer, DBPool, observability stack

---

#### Purpose

Prometheus metric names, bounded label conventions, and histogram buckets.

---

#### Shared Context (duplicated for self-containment)

- Error envelope: `{error, code, details}`
- Tenant isolation key: `tenant_id`
- Correlation key: `request_id`
- Ownership fence keys: `run_id`, `effect_id`

---

#### Public Interface

```text
Policy is applied across all relevant components through middleware, service wrappers, and shared helper packages.
```

##### Example — Metrics

**Request:**
```json
{"tenant_id":"acme-prod","request_id":"r-123"}
```

**Response (compliant):**
```json
{"status":"compliant"}
```

**Response (violation):**
```json
{"error":"policy violation","code":"POLICY_VIOLATION","details":null}
```

---

#### Internal Logic (step-by-step)

1. Evaluate incoming request or runtime event against this concern's policy.
2. Enforce deterministic action (allow, deny, retry, or terminate).
3. Persist required state transitions and audit evidence.
4. Emit logs, metrics, and traces with standard fields.

---

#### Data Model (if this component owns a table)

Uses shared tables (`jobs`, `jobs_read`, `outbox`, `external_effects`, `idempotency_keys`, `audit_log`) with no additional table required in this phase.

---

#### Error Table

| Condition | HTTP Status | Error Code | Response Body |
|---|---|---|---|
| Policy or validation violation | 400 | `POLICY_VIOLATION` | `{"error":"policy violation","code":"POLICY_VIOLATION","details":null}` |
| Runtime dependency unavailable | 503 | `SERVICE_UNAVAILABLE` | `{"error":"service unavailable","code":"SERVICE_UNAVAILABLE","details":null}` |

---

#### Acceptance Criteria (Gherkin)

```gherkin
Feature: Metrics

  Scenario: Happy path
    Given valid request and dependencies
    When policy is evaluated
    Then compliant behaviour is enforced

  Scenario: Edge case
    Given replayed or duplicated input
    When policy is evaluated again
    Then output remains deterministic

  Scenario: Error
    Given runtime dependency failure
    When policy is evaluated
    Then structured error response is returned
```

---

#### Performance Targets

| Metric | Target |
|---|---|
| p95 evaluation latency | <20ms |
| policy enforcement overhead | <5% of request latency budget |

---

#### Security Considerations

- No secrets in logs, traces, or error bodies.
- Explicit tenant scoping for all checks.
- Deny by default when policy engine state is unavailable.

---

#### Observability

- **Log events:** `crosscut.metrics.applied`, `crosscut.metrics.rejected`
- **Metrics:** `crosscut_metrics_total{status}`
- **Trace span:** `crosscut.metrics`

Approval-specific metrics contract:
- `re_approval_requests_total{tenant_id,path_key,step_key}`
- `re_approval_decisions_total{tenant_id,path_key,step_key,decision}`
- `re_approval_latency_seconds{tenant_id,path_key}`
- `re_approval_escalations_total{tenant_id,path_key}`
- `re_approval_timeouts_total{tenant_id,path_key}`
- `re_approval_worker_tick_duration_seconds{status}`

---

#### Cross-Component Interactions

| Direction | Component | Mechanism | Data Exchanged |
|---|---|---|---|
| This → HTTP middleware | in-process | request interception | headers, claims, request metadata |
| This → DBPool | SQL | policy/audit persistence | decision, timestamps, tenant_id |
| This → Observability | in-process | metrics/traces/logs | event labels, durations, outcome |

### SPEC: Tracing

**File:** `internal/crosscut/tracing.md`
**Package:** `crosscut`
**Phase:** 4
**Dependencies:** ConfigLoader, HTTPServer, DBPool, observability stack

---

#### Purpose

OpenTelemetry span naming, context propagation, and sampling policy.

---

#### Shared Context (duplicated for self-containment)

- Error envelope: `{error, code, details}`
- Tenant isolation key: `tenant_id`
- Correlation key: `request_id`
- Ownership fence keys: `run_id`, `effect_id`

---

#### Public Interface

```text
Policy is applied across all relevant components through middleware, service wrappers, and shared helper packages.
```

##### Example — Tracing

**Request:**
```json
{"tenant_id":"acme-prod","request_id":"r-123"}
```

**Response (compliant):**
```json
{"status":"compliant"}
```

**Response (violation):**
```json
{"error":"policy violation","code":"POLICY_VIOLATION","details":null}
```

---

#### Internal Logic (step-by-step)

1. Evaluate incoming request or runtime event against this concern's policy.
2. Enforce deterministic action (allow, deny, retry, or terminate).
3. Persist required state transitions and audit evidence.
4. Emit logs, metrics, and traces with standard fields.

---

#### Data Model (if this component owns a table)

Uses shared tables (`jobs`, `jobs_read`, `outbox`, `external_effects`, `idempotency_keys`, `audit_log`) with no additional table required in this phase.

---

#### Error Table

| Condition | HTTP Status | Error Code | Response Body |
|---|---|---|---|
| Policy or validation violation | 400 | `POLICY_VIOLATION` | `{"error":"policy violation","code":"POLICY_VIOLATION","details":null}` |
| Runtime dependency unavailable | 503 | `SERVICE_UNAVAILABLE` | `{"error":"service unavailable","code":"SERVICE_UNAVAILABLE","details":null}` |

---

#### Acceptance Criteria (Gherkin)

```gherkin
Feature: Tracing

  Scenario: Happy path
    Given valid request and dependencies
    When policy is evaluated
    Then compliant behaviour is enforced

  Scenario: Edge case
    Given replayed or duplicated input
    When policy is evaluated again
    Then output remains deterministic

  Scenario: Error
    Given runtime dependency failure
    When policy is evaluated
    Then structured error response is returned
```

---

#### Performance Targets

| Metric | Target |
|---|---|
| p95 evaluation latency | <20ms |
| policy enforcement overhead | <5% of request latency budget |

---

#### Security Considerations

- No secrets in logs, traces, or error bodies.
- Explicit tenant scoping for all checks.
- Deny by default when policy engine state is unavailable.

---

#### Observability

- **Log events:** `crosscut.tracing.applied`, `crosscut.tracing.rejected`
- **Metrics:** `crosscut_tracing_total{status}`
- **Trace span:** `crosscut.tracing`

---

#### Cross-Component Interactions

| Direction | Component | Mechanism | Data Exchanged |
|---|---|---|---|
| This → HTTP middleware | in-process | request interception | headers, claims, request metadata |
| This → DBPool | SQL | policy/audit persistence | decision, timestamps, tenant_id |
| This → Observability | in-process | metrics/traces/logs | event labels, durations, outcome |

### SPEC: Configuration

**File:** `internal/crosscut/configuration.md`
**Package:** `crosscut`
**Phase:** 4
**Dependencies:** ConfigLoader, HTTPServer, DBPool, observability stack

---

#### Purpose

Loading order, startup validation, required and optional variables, and immutable runtime config.

---

#### Shared Context (duplicated for self-containment)

- Error envelope: `{error, code, details}`
- Tenant isolation key: `tenant_id`
- Correlation key: `request_id`
- Ownership fence keys: `run_id`, `effect_id`

---

#### Public Interface

```text
Policy is applied across all relevant components through middleware, service wrappers, and shared helper packages.
```

##### Example — Configuration

**Request:**
```json
{"tenant_id":"acme-prod","request_id":"r-123"}
```

**Response (compliant):**
```json
{"status":"compliant"}
```

**Response (violation):**
```json
{"error":"policy violation","code":"POLICY_VIOLATION","details":null}
```

---

#### Internal Logic (step-by-step)

1. Evaluate incoming request or runtime event against this concern's policy.
2. Enforce deterministic action (allow, deny, retry, or terminate).
3. Persist required state transitions and audit evidence.
4. Emit logs, metrics, and traces with standard fields.

---

#### Data Model (if this component owns a table)

Uses shared tables (`jobs`, `jobs_read`, `outbox`, `external_effects`, `idempotency_keys`, `audit_log`) with no additional table required in this phase.

---

#### Error Table

| Condition | HTTP Status | Error Code | Response Body |
|---|---|---|---|
| Policy or validation violation | 400 | `POLICY_VIOLATION` | `{"error":"policy violation","code":"POLICY_VIOLATION","details":null}` |
| Runtime dependency unavailable | 503 | `SERVICE_UNAVAILABLE` | `{"error":"service unavailable","code":"SERVICE_UNAVAILABLE","details":null}` |

---

#### Acceptance Criteria (Gherkin)

```gherkin
Feature: Configuration

  Scenario: Happy path
    Given valid request and dependencies
    When policy is evaluated
    Then compliant behaviour is enforced

  Scenario: Edge case
    Given replayed or duplicated input
    When policy is evaluated again
    Then output remains deterministic

  Scenario: Error
    Given runtime dependency failure
    When policy is evaluated
    Then structured error response is returned
```

---

#### Performance Targets

| Metric | Target |
|---|---|
| p95 evaluation latency | <20ms |
| policy enforcement overhead | <5% of request latency budget |

---

#### Security Considerations

- No secrets in logs, traces, or error bodies.
- Explicit tenant scoping for all checks.
- Deny by default when policy engine state is unavailable.

---

#### Observability

- **Log events:** `crosscut.configuration.applied`, `crosscut.configuration.rejected`
- **Metrics:** `crosscut_configuration_total{status}`
- **Trace span:** `crosscut.configuration`

---

#### Cross-Component Interactions

| Direction | Component | Mechanism | Data Exchanged |
|---|---|---|---|
| This → HTTP middleware | in-process | request interception | headers, claims, request metadata |
| This → DBPool | SQL | policy/audit persistence | decision, timestamps, tenant_id |
| This → Observability | in-process | metrics/traces/logs | event labels, durations, outcome |

### SPEC: Database Migrations

**File:** `internal/crosscut/database-migrations.md`
**Package:** `crosscut`
**Phase:** 4
**Dependencies:** ConfigLoader, HTTPServer, DBPool, observability stack

---

#### Purpose

Atlas workflow, naming convention, CI linting, and controlled apply procedure.

---

#### Shared Context (duplicated for self-containment)

- Error envelope: `{error, code, details}`
- Tenant isolation key: `tenant_id`
- Correlation key: `request_id`
- Ownership fence keys: `run_id`, `effect_id`

---

#### Public Interface

```text
Policy is applied across all relevant components through middleware, service wrappers, and shared helper packages.
```

##### Example — Database Migrations

**Request:**
```json
{"tenant_id":"acme-prod","request_id":"r-123"}
```

**Response (compliant):**
```json
{"status":"compliant"}
```

**Response (violation):**
```json
{"error":"policy violation","code":"POLICY_VIOLATION","details":null}
```

---

#### Internal Logic (step-by-step)

1. Evaluate incoming request or runtime event against this concern's policy.
2. Enforce deterministic action (allow, deny, retry, or terminate).
3. Persist required state transitions and audit evidence.
4. Emit logs, metrics, and traces with standard fields.

---

#### Data Model (if this component owns a table)

Uses shared tables (`jobs`, `jobs_read`, `outbox`, `external_effects`, `idempotency_keys`, `audit_log`) with no additional table required in this phase.

---

#### Error Table

| Condition | HTTP Status | Error Code | Response Body |
|---|---|---|---|
| Policy or validation violation | 400 | `POLICY_VIOLATION` | `{"error":"policy violation","code":"POLICY_VIOLATION","details":null}` |
| Runtime dependency unavailable | 503 | `SERVICE_UNAVAILABLE` | `{"error":"service unavailable","code":"SERVICE_UNAVAILABLE","details":null}` |

---

#### Acceptance Criteria (Gherkin)

```gherkin
Feature: Database Migrations

  Scenario: Happy path
    Given valid request and dependencies
    When policy is evaluated
    Then compliant behaviour is enforced

  Scenario: Edge case
    Given replayed or duplicated input
    When policy is evaluated again
    Then output remains deterministic

  Scenario: Error
    Given runtime dependency failure
    When policy is evaluated
    Then structured error response is returned
```

---

#### Performance Targets

| Metric | Target |
|---|---|
| p95 evaluation latency | <20ms |
| policy enforcement overhead | <5% of request latency budget |

---

#### Security Considerations

- No secrets in logs, traces, or error bodies.
- Explicit tenant scoping for all checks.
- Deny by default when policy engine state is unavailable.

---

#### Observability

- **Log events:** `crosscut.database-migrations.applied`, `crosscut.database-migrations.rejected`
- **Metrics:** `crosscut_database_migrations_total{status}`
- **Trace span:** `crosscut.database-migrations`

---

#### Cross-Component Interactions

| Direction | Component | Mechanism | Data Exchanged |
|---|---|---|---|
| This → HTTP middleware | in-process | request interception | headers, claims, request metadata |
| This → DBPool | SQL | policy/audit persistence | decision, timestamps, tenant_id |
| This → Observability | in-process | metrics/traces/logs | event labels, durations, outcome |

### SPEC: Health Checks

**File:** `internal/crosscut/health-checks.md`
**Package:** `crosscut`
**Phase:** 4
**Dependencies:** ConfigLoader, HTTPServer, DBPool, observability stack

---

#### Purpose

`/healthz` and `/readyz` semantics, readiness dependencies, and failure response schema.

---

#### Shared Context (duplicated for self-containment)

- Error envelope: `{error, code, details}`
- Tenant isolation key: `tenant_id`
- Correlation key: `request_id`
- Ownership fence keys: `run_id`, `effect_id`

---

#### Public Interface

```text
Policy is applied across all relevant components through middleware, service wrappers, and shared helper packages.
```

##### Example — Health Checks

**Request:**
```json
{"tenant_id":"acme-prod","request_id":"r-123"}
```

**Response (compliant):**
```json
{"status":"compliant"}
```

**Response (violation):**
```json
{"error":"policy violation","code":"POLICY_VIOLATION","details":null}
```

---

#### Internal Logic (step-by-step)

1. Evaluate incoming request or runtime event against this concern's policy.
2. Enforce deterministic action (allow, deny, retry, or terminate).
3. Persist required state transitions and audit evidence.
4. Emit logs, metrics, and traces with standard fields.

---

#### Data Model (if this component owns a table)

Uses shared tables (`jobs`, `jobs_read`, `outbox`, `external_effects`, `idempotency_keys`, `audit_log`) with no additional table required in this phase.

---

#### Error Table

| Condition | HTTP Status | Error Code | Response Body |
|---|---|---|---|
| Policy or validation violation | 400 | `POLICY_VIOLATION` | `{"error":"policy violation","code":"POLICY_VIOLATION","details":null}` |
| Runtime dependency unavailable | 503 | `SERVICE_UNAVAILABLE` | `{"error":"service unavailable","code":"SERVICE_UNAVAILABLE","details":null}` |

---

#### Acceptance Criteria (Gherkin)

```gherkin
Feature: Health Checks

  Scenario: Happy path
    Given valid request and dependencies
    When policy is evaluated
    Then compliant behaviour is enforced

  Scenario: Edge case
    Given replayed or duplicated input
    When policy is evaluated again
    Then output remains deterministic

  Scenario: Error
    Given runtime dependency failure
    When policy is evaluated
    Then structured error response is returned
```

---

#### Performance Targets

| Metric | Target |
|---|---|
| p95 evaluation latency | <20ms |
| policy enforcement overhead | <5% of request latency budget |

---

#### Security Considerations

- No secrets in logs, traces, or error bodies.
- Explicit tenant scoping for all checks.
- Deny by default when policy engine state is unavailable.

---

#### Observability

- **Log events:** `crosscut.health-checks.applied`, `crosscut.health-checks.rejected`
- **Metrics:** `crosscut_health_checks_total{status}`
- **Trace span:** `crosscut.health-checks`

---

#### Cross-Component Interactions

| Direction | Component | Mechanism | Data Exchanged |
|---|---|---|---|
| This → HTTP middleware | in-process | request interception | headers, claims, request metadata |
| This → DBPool | SQL | policy/audit persistence | decision, timestamps, tenant_id |
| This → Observability | in-process | metrics/traces/logs | event labels, durations, outcome |

### SPEC: Rate Limiting

**File:** `internal/crosscut/rate-limiting.md`
**Package:** `crosscut`
**Phase:** 4
**Dependencies:** ConfigLoader, HTTPServer, DBPool, observability stack

---

#### Purpose

Token-bucket algorithm, per-tenant defaults, Retry-After semantics, and exhaustion metrics.

---

#### Shared Context (duplicated for self-containment)

- Error envelope: `{error, code, details}`
- Tenant isolation key: `tenant_id`
- Correlation key: `request_id`
- Ownership fence keys: `run_id`, `effect_id`

---

#### Public Interface

```text
Policy is applied across all relevant components through middleware, service wrappers, and shared helper packages.
```

##### Example — Rate Limiting

**Request:**
```json
{"tenant_id":"acme-prod","request_id":"r-123"}
```

**Response (compliant):**
```json
{"status":"compliant"}
```

**Response (violation):**
```json
{"error":"policy violation","code":"POLICY_VIOLATION","details":null}
```

---

#### Internal Logic (step-by-step)

1. Evaluate incoming request or runtime event against this concern's policy.
2. Enforce deterministic action (allow, deny, retry, or terminate).
3. Persist required state transitions and audit evidence.
4. Emit logs, metrics, and traces with standard fields.

---

#### Data Model (if this component owns a table)

Uses shared tables (`jobs`, `jobs_read`, `outbox`, `external_effects`, `idempotency_keys`, `audit_log`) with no additional table required in this phase.

---

#### Error Table

| Condition | HTTP Status | Error Code | Response Body |
|---|---|---|---|
| Policy or validation violation | 400 | `POLICY_VIOLATION` | `{"error":"policy violation","code":"POLICY_VIOLATION","details":null}` |
| Runtime dependency unavailable | 503 | `SERVICE_UNAVAILABLE` | `{"error":"service unavailable","code":"SERVICE_UNAVAILABLE","details":null}` |

---

#### Acceptance Criteria (Gherkin)

```gherkin
Feature: Rate Limiting

  Scenario: Happy path
    Given valid request and dependencies
    When policy is evaluated
    Then compliant behaviour is enforced

  Scenario: Edge case
    Given replayed or duplicated input
    When policy is evaluated again
    Then output remains deterministic

  Scenario: Error
    Given runtime dependency failure
    When policy is evaluated
    Then structured error response is returned
```

---

#### Performance Targets

| Metric | Target |
|---|---|
| p95 evaluation latency | <20ms |
| policy enforcement overhead | <5% of request latency budget |

---

#### Security Considerations

- No secrets in logs, traces, or error bodies.
- Explicit tenant scoping for all checks.
- Deny by default when policy engine state is unavailable.

---

#### Observability

- **Log events:** `crosscut.rate-limiting.applied`, `crosscut.rate-limiting.rejected`
- **Metrics:** `crosscut_rate_limiting_total{status}`
- **Trace span:** `crosscut.rate-limiting`

---

#### Cross-Component Interactions

| Direction | Component | Mechanism | Data Exchanged |
|---|---|---|---|
| This → HTTP middleware | in-process | request interception | headers, claims, request metadata |
| This → DBPool | SQL | policy/audit persistence | decision, timestamps, tenant_id |
| This → Observability | in-process | metrics/traces/logs | event labels, durations, outcome |

### SPEC: Pagination

**File:** `internal/crosscut/pagination.md`
**Package:** `crosscut`
**Phase:** 4
**Dependencies:** ConfigLoader, HTTPServer, DBPool, observability stack

---

#### Purpose

Cursor schema for future list APIs; no current GA list endpoint requires pagination.

---

#### Shared Context (duplicated for self-containment)

- Error envelope: `{error, code, details}`
- Tenant isolation key: `tenant_id`
- Correlation key: `request_id`
- Ownership fence keys: `run_id`, `effect_id`

---

#### Public Interface

```text
Policy is applied across all relevant components through middleware, service wrappers, and shared helper packages.
```

##### Example — Pagination

**Request:**
```json
{"tenant_id":"acme-prod","request_id":"r-123"}
```

**Response (compliant):**
```json
{"status":"compliant"}
```

**Response (violation):**
```json
{"error":"policy violation","code":"POLICY_VIOLATION","details":null}
```

---

#### Internal Logic (step-by-step)

1. Evaluate incoming request or runtime event against this concern's policy.
2. Enforce deterministic action (allow, deny, retry, or terminate).
3. Persist required state transitions and audit evidence.
4. Emit logs, metrics, and traces with standard fields.

---

#### Data Model (if this component owns a table)

Uses shared tables (`jobs`, `jobs_read`, `outbox`, `external_effects`, `idempotency_keys`, `audit_log`) with no additional table required in this phase.

---

#### Error Table

| Condition | HTTP Status | Error Code | Response Body |
|---|---|---|---|
| Policy or validation violation | 400 | `POLICY_VIOLATION` | `{"error":"policy violation","code":"POLICY_VIOLATION","details":null}` |
| Runtime dependency unavailable | 503 | `SERVICE_UNAVAILABLE` | `{"error":"service unavailable","code":"SERVICE_UNAVAILABLE","details":null}` |

---

#### Acceptance Criteria (Gherkin)

```gherkin
Feature: Pagination

  Scenario: Happy path
    Given valid request and dependencies
    When policy is evaluated
    Then compliant behaviour is enforced

  Scenario: Edge case
    Given replayed or duplicated input
    When policy is evaluated again
    Then output remains deterministic

  Scenario: Error
    Given runtime dependency failure
    When policy is evaluated
    Then structured error response is returned
```

---

#### Performance Targets

| Metric | Target |
|---|---|
| p95 evaluation latency | <20ms |
| policy enforcement overhead | <5% of request latency budget |

---

#### Security Considerations

- No secrets in logs, traces, or error bodies.
- Explicit tenant scoping for all checks.
- Deny by default when policy engine state is unavailable.

---

#### Observability

- **Log events:** `crosscut.pagination.applied`, `crosscut.pagination.rejected`
- **Metrics:** `crosscut_pagination_total{status}`
- **Trace span:** `crosscut.pagination`

---

#### Cross-Component Interactions

| Direction | Component | Mechanism | Data Exchanged |
|---|---|---|---|
| This → HTTP middleware | in-process | request interception | headers, claims, request metadata |
| This → DBPool | SQL | policy/audit persistence | decision, timestamps, tenant_id |
| This → Observability | in-process | metrics/traces/logs | event labels, durations, outcome |

### SPEC: CORS

**File:** `internal/crosscut/cors.md`
**Package:** `crosscut`
**Phase:** 4
**Dependencies:** ConfigLoader, HTTPServer, DBPool, observability stack

---

#### Purpose

Allowed origins/methods/headers policy for browser-based clients.

---

#### Shared Context (duplicated for self-containment)

- Error envelope: `{error, code, details}`
- Tenant isolation key: `tenant_id`
- Correlation key: `request_id`
- Ownership fence keys: `run_id`, `effect_id`

---

#### Public Interface

```text
Policy is applied across all relevant components through middleware, service wrappers, and shared helper packages.
```

##### Example — CORS

**Request:**
```json
{"tenant_id":"acme-prod","request_id":"r-123"}
```

**Response (compliant):**
```json
{"status":"compliant"}
```

**Response (violation):**
```json
{"error":"policy violation","code":"POLICY_VIOLATION","details":null}
```

---

#### Internal Logic (step-by-step)

1. Evaluate incoming request or runtime event against this concern's policy.
2. Enforce deterministic action (allow, deny, retry, or terminate).
3. Persist required state transitions and audit evidence.
4. Emit logs, metrics, and traces with standard fields.

---

#### Data Model (if this component owns a table)

Uses shared tables (`jobs`, `jobs_read`, `outbox`, `external_effects`, `idempotency_keys`, `audit_log`) with no additional table required in this phase.

---

#### Error Table

| Condition | HTTP Status | Error Code | Response Body |
|---|---|---|---|
| Policy or validation violation | 400 | `POLICY_VIOLATION` | `{"error":"policy violation","code":"POLICY_VIOLATION","details":null}` |
| Runtime dependency unavailable | 503 | `SERVICE_UNAVAILABLE` | `{"error":"service unavailable","code":"SERVICE_UNAVAILABLE","details":null}` |

---

#### Acceptance Criteria (Gherkin)

```gherkin
Feature: CORS

  Scenario: Happy path
    Given valid request and dependencies
    When policy is evaluated
    Then compliant behaviour is enforced

  Scenario: Edge case
    Given replayed or duplicated input
    When policy is evaluated again
    Then output remains deterministic

  Scenario: Error
    Given runtime dependency failure
    When policy is evaluated
    Then structured error response is returned
```

---

#### Performance Targets

| Metric | Target |
|---|---|
| p95 evaluation latency | <20ms |
| policy enforcement overhead | <5% of request latency budget |

---

#### Security Considerations

- No secrets in logs, traces, or error bodies.
- Explicit tenant scoping for all checks.
- Deny by default when policy engine state is unavailable.

---

#### Observability

- **Log events:** `crosscut.cors.applied`, `crosscut.cors.rejected`
- **Metrics:** `crosscut_cors_total{status}`
- **Trace span:** `crosscut.cors`

---

#### Cross-Component Interactions

| Direction | Component | Mechanism | Data Exchanged |
|---|---|---|---|
| This → HTTP middleware | in-process | request interception | headers, claims, request metadata |
| This → DBPool | SQL | policy/audit persistence | decision, timestamps, tenant_id |
| This → Observability | in-process | metrics/traces/logs | event labels, durations, outcome |

### SPEC: Input Validation

**File:** `internal/crosscut/input-validation.md`
**Package:** `crosscut`
**Phase:** 4
**Dependencies:** ConfigLoader, HTTPServer, DBPool, observability stack

---

#### Purpose

Schema validation, SSRF prevention, payload-size cap, and canonicalisation for hashing.

---

#### Shared Context (duplicated for self-containment)

- Error envelope: `{error, code, details}`
- Tenant isolation key: `tenant_id`
- Correlation key: `request_id`
- Ownership fence keys: `run_id`, `effect_id`

---

#### Public Interface

```text
Policy is applied across all relevant components through middleware, service wrappers, and shared helper packages.
```

##### Example — Input Validation

**Request:**
```json
{"tenant_id":"acme-prod","request_id":"r-123"}
```

**Response (compliant):**
```json
{"status":"compliant"}
```

**Response (violation):**
```json
{"error":"policy violation","code":"POLICY_VIOLATION","details":null}
```

---

#### Internal Logic (step-by-step)

1. Evaluate incoming request or runtime event against this concern's policy.
2. Enforce deterministic action (allow, deny, retry, or terminate).
3. Persist required state transitions and audit evidence.
4. Emit logs, metrics, and traces with standard fields.

---

#### Data Model (if this component owns a table)

Uses shared tables (`jobs`, `jobs_read`, `outbox`, `external_effects`, `idempotency_keys`, `audit_log`) with no additional table required in this phase.

---

#### Error Table

| Condition | HTTP Status | Error Code | Response Body |
|---|---|---|---|
| Policy or validation violation | 400 | `POLICY_VIOLATION` | `{"error":"policy violation","code":"POLICY_VIOLATION","details":null}` |
| Runtime dependency unavailable | 503 | `SERVICE_UNAVAILABLE` | `{"error":"service unavailable","code":"SERVICE_UNAVAILABLE","details":null}` |

---

#### Acceptance Criteria (Gherkin)

```gherkin
Feature: Input Validation

  Scenario: Happy path
    Given valid request and dependencies
    When policy is evaluated
    Then compliant behaviour is enforced

  Scenario: Edge case
    Given replayed or duplicated input
    When policy is evaluated again
    Then output remains deterministic

  Scenario: Error
    Given runtime dependency failure
    When policy is evaluated
    Then structured error response is returned
```

---

#### Performance Targets

| Metric | Target |
|---|---|
| p95 evaluation latency | <20ms |
| policy enforcement overhead | <5% of request latency budget |

---

#### Security Considerations

- No secrets in logs, traces, or error bodies.
- Explicit tenant scoping for all checks.
- Deny by default when policy engine state is unavailable.

---

#### Observability

- **Log events:** `crosscut.input-validation.applied`, `crosscut.input-validation.rejected`
- **Metrics:** `crosscut_input_validation_total{status}`
- **Trace span:** `crosscut.input-validation`

---

#### Cross-Component Interactions

| Direction | Component | Mechanism | Data Exchanged |
|---|---|---|---|
| This → HTTP middleware | in-process | request interception | headers, claims, request metadata |
| This → DBPool | SQL | policy/audit persistence | decision, timestamps, tenant_id |
| This → Observability | in-process | metrics/traces/logs | event labels, durations, outcome |

### SPEC: Graceful Shutdown

**File:** `internal/crosscut/graceful-shutdown.md`
**Package:** `crosscut`
**Phase:** 4
**Dependencies:** ConfigLoader, HTTPServer, DBPool, observability stack

---

#### Purpose

Signal handling, claim stop, in-flight drain timeout, and connection close order.

---

#### Shared Context (duplicated for self-containment)

- Error envelope: `{error, code, details}`
- Tenant isolation key: `tenant_id`
- Correlation key: `request_id`
- Ownership fence keys: `run_id`, `effect_id`

---

#### Public Interface

```text
Policy is applied across all relevant components through middleware, service wrappers, and shared helper packages.
```

##### Example — Graceful Shutdown

**Request:**
```json
{"tenant_id":"acme-prod","request_id":"r-123"}
```

**Response (compliant):**
```json
{"status":"compliant"}
```

**Response (violation):**
```json
{"error":"policy violation","code":"POLICY_VIOLATION","details":null}
```

---

#### Internal Logic (step-by-step)

1. Evaluate incoming request or runtime event against this concern's policy.
2. Enforce deterministic action (allow, deny, retry, or terminate).
3. Persist required state transitions and audit evidence.
4. Emit logs, metrics, and traces with standard fields.

---

#### Data Model (if this component owns a table)

Uses shared tables (`jobs`, `jobs_read`, `outbox`, `external_effects`, `idempotency_keys`, `audit_log`) with no additional table required in this phase.

---

#### Error Table

| Condition | HTTP Status | Error Code | Response Body |
|---|---|---|---|
| Policy or validation violation | 400 | `POLICY_VIOLATION` | `{"error":"policy violation","code":"POLICY_VIOLATION","details":null}` |
| Runtime dependency unavailable | 503 | `SERVICE_UNAVAILABLE` | `{"error":"service unavailable","code":"SERVICE_UNAVAILABLE","details":null}` |

---

#### Acceptance Criteria (Gherkin)

```gherkin
Feature: Graceful Shutdown

  Scenario: Happy path
    Given valid request and dependencies
    When policy is evaluated
    Then compliant behaviour is enforced

  Scenario: Edge case
    Given replayed or duplicated input
    When policy is evaluated again
    Then output remains deterministic

  Scenario: Error
    Given runtime dependency failure
    When policy is evaluated
    Then structured error response is returned
```

---

#### Performance Targets

| Metric | Target |
|---|---|
| p95 evaluation latency | <20ms |
| policy enforcement overhead | <5% of request latency budget |

---

#### Security Considerations

- No secrets in logs, traces, or error bodies.
- Explicit tenant scoping for all checks.
- Deny by default when policy engine state is unavailable.

---

#### Observability

- **Log events:** `crosscut.graceful-shutdown.applied`, `crosscut.graceful-shutdown.rejected`
- **Metrics:** `crosscut_graceful_shutdown_total{status}`
- **Trace span:** `crosscut.graceful-shutdown`

---

#### Cross-Component Interactions

| Direction | Component | Mechanism | Data Exchanged |
|---|---|---|---|
| This → HTTP middleware | in-process | request interception | headers, claims, request metadata |
| This → DBPool | SQL | policy/audit persistence | decision, timestamps, tenant_id |
| This → Observability | in-process | metrics/traces/logs | event labels, durations, outcome |
