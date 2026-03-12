# Phase 3 — Detailed Component Specifications

## Table of Contents

- [SPEC: ConfigLoader](#spec-configloader)
- [SPEC: LoggerFactory](#spec-loggerfactory)
- [SPEC: DBPool](#spec-dbpool)
- [SPEC: HTTPServer](#spec-httpserver)
- [SPEC: AuthMiddleware](#spec-authmiddleware)
- [SPEC: RateLimiter](#spec-ratelimiter)
- [SPEC: PolicyEngine](#spec-policyengine)
- [SPEC: IdempotencyService](#spec-idempotencyservice)
- [SPEC: JobsAPIHandler](#spec-jobsapihandler)
- [SPEC: HealthHandler](#spec-healthhandler)
- [SPEC: SchedulerService](#spec-schedulerservice)
- [SPEC: LeaseManager](#spec-leasemanager)
- [SPEC: RunnerService](#spec-runnerservice)
- [SPEC: StepAPIAdapter](#spec-stepapiadapter)
- [SPEC: ApprovalWorker](#spec-approvalworker)
- [SPEC: ModuleRegistry](#spec-moduleregistry)
- [SPEC: ConnectorRegistry](#spec-connectorregistry)
- [SPEC: VoltaManager](#spec-voltamanager)
- [SPEC: ReconcilerService](#spec-reconcilerservice)
- [SPEC: OutboxDispatcher](#spec-outboxdispatcher)
- [SPEC: CallbackSigner](#spec-callbacksigner)
- [SPEC: MetricsExporter](#spec-metricsexporter)
- [SPEC: MetricsSQLWriter](#spec-metricssqlwriter)
- [SPEC: TracingService](#spec-tracingservice)
- [SPEC: AuditService](#spec-auditservice)
- [SPEC: MigrationChecker](#spec-migrationchecker)

### SPEC: ConfigLoader

**File:** `internal/config/loader.go`
**Package:** `config`
**Phase:** 0
**Dependencies:** none

---

#### Purpose

Loads environment variables and static defaults, validates required values at startup, and exposes immutable runtime configuration.

---

#### Shared Context (duplicated for self-containment)

```go
// ErrorResponse is the standard API error envelope.
type ErrorResponse struct {
    Error   string `json:"error"`
    Code    string `json:"code"`
    Details any    `json:"details,omitempty"`
}

// JobCreateRequest is the POST /v1/jobs request schema.
type JobCreateRequest struct {
    TenantID       string         `json:"tenant_id"`
    PathKey        string         `json:"path_key"`
    Params         map[string]any `json:"params"`
    IdempotencyKey string         `json:"idempotency_key"`
    CallbackURL    *string        `json:"callback_url,omitempty"`
}

// JobStatusResponse is the GET /v1/jobs/{job_id} schema.
type JobStatusResponse struct {
    JobID            string     `json:"job_id"`
    TenantID         string     `json:"tenant_id"`
    State            string     `json:"state"`
    Attempt          int        `json:"attempt"`
    LeaseExpiresAt   *time.Time `json:"lease_expires_at,omitempty"`
    LastErrorCode    *string    `json:"last_error_code,omitempty"`
    LastErrorMessage *string    `json:"last_error_message,omitempty"`
}
```

Validation rules used by this component:
- `tenant_id` regex: `^[a-z0-9][a-z0-9-]{1,62}$`
- `idempotency_key` regex: `^[a-zA-Z0-9\-_.]{1,128}$`
- Optional `schedule` must be a valid cron expression.
- Optional `first_run_at` must be a valid RFC3339 timestamp.
- Optional `max_attempts` defaults to `3` when omitted or invalid.
- Maximum request body size: `262144` bytes
- Callback URL must use HTTPS and must not resolve to blocked private/link-local/metadata ranges.

---

#### Public Interface

```go
type Loader interface {
    Load(ctx context.Context) (Config, error)
}

type Config struct {
    HTTPPort int
    DatabaseURL string
    OIDCIssuerURL string
    OIDCAudience string
}
```

##### Example — ConfigLoader

**Request:**
```json
{
  "tenant_id": "acme-prod",
  "job_id": "6c8bb1de-ef20-45cb-9e13-a57f44667d88",
  "run_id": "8a4cfa4d-f89c-4f0e-a809-f7d9a2c8f8ad"
}
```

**Response (200):**
```json
{
  "status": "ok",
  "component": "ConfigLoader"
}
```

**Response (error):**
```json
{
  "error": "operation failed",
  "code": "INTERNAL_ERROR",
  "details": null
}
```

---

#### Internal Logic (step-by-step)

1. Read environment variables in precedence order: environment -> static defaults.
2. Validate required fields: DATABASE_URL, OIDC_ISSUER_URL, OIDC_AUDIENCE, VOLTA_SM_SECRET_ID, VOLTA_S3_BUCKET.
3. Parse typed values (durations, integers, booleans) and fail fast on invalid values.
4. Return immutable Config; do not allow runtime mutation.

---

#### Data Model (if this component owns a table)

This component uses shared Release Engine tables and does not introduce a new table in this phase.

---

#### Error Table

| Condition | HTTP Status | Error Code | Response Body |
|---|---|---|---|
| Missing required variable | 500 | `CONFIG_MISSING` | `{"error":"missing required configuration","code":"CONFIG_MISSING","details":{"var":"DATABASE_URL"}}` |
| Invalid typed value | 500 | `CONFIG_INVALID` | `{"error":"invalid configuration value","code":"CONFIG_INVALID","details":{"var":"HTTP_PORT"}}` |

---

#### Acceptance Criteria (Gherkin)

```gherkin
Feature: ConfigLoader

  Scenario: Happy path
    Given valid input and healthy dependencies
    When the component executes
    Then it returns success and emits logs, metrics, and traces

  Scenario: Edge case
    Given replayed state or partial prior progress
    When the component executes again
    Then it behaves deterministically without duplicate side effects

  Scenario: Error
    Given dependency failure or stale run ownership
    When the component executes
    Then it returns a structured error with explicit code
```

---

#### Performance Targets

| Metric | Target |
|---|---|
| Startup config load | <150ms |
| Validation | <20ms |

---

#### Security Considerations

- Enforce tenant isolation on every SQL read and write.
- Redact or avoid secret values in logs, traces, and error payloads.
- Use fenced writes (`run_id`/`effect_id`) for ownership-sensitive state transitions.

---

#### Observability

- **Log events:** `configloader.start`, `configloader.success`, `configloader.failure`
- **Metrics:** `release_engine_configloader_total{status}`, `release_engine_configloader_duration_seconds`
- **Trace span:** `config.load`

---

#### Cross-Component Interactions

| Direction | Component | Mechanism | Data Exchanged |
|---|---|---|---|
| This → DBPool | outbound call | pgx query/exec | SQL statements, parameters, transaction result |
| This → MetricsExporter | outbound call | in-process API | counters, histograms, labels |
| This → TracingService | outbound call | OpenTelemetry | span start/end, attributes, errors |
| Upstream runtime → This | inbound call | in-process call | context, tenant_id, job_id, run_id |

### SPEC: LoggerFactory

**File:** `internal/logging/factory.go`
**Package:** `logging`
**Phase:** 0
**Dependencies:** ConfigLoader

---

#### Purpose

Initialises structured JSON logging with stable fields and level filtering for all components.

---

#### Shared Context (duplicated for self-containment)

```go
// ErrorResponse is the standard API error envelope.
type ErrorResponse struct {
    Error   string `json:"error"`
    Code    string `json:"code"`
    Details any    `json:"details,omitempty"`
}

// JobCreateRequest is the POST /v1/jobs request schema.
type JobCreateRequest struct {
    TenantID       string         `json:"tenant_id"`
    PathKey        string         `json:"path_key"`
    Params         map[string]any `json:"params"`
    IdempotencyKey string         `json:"idempotency_key"`
    CallbackURL    *string        `json:"callback_url,omitempty"`
}

// JobStatusResponse is the GET /v1/jobs/{job_id} schema.
type JobStatusResponse struct {
    JobID            string     `json:"job_id"`
    TenantID         string     `json:"tenant_id"`
    State            string     `json:"state"`
    Attempt          int        `json:"attempt"`
    LeaseExpiresAt   *time.Time `json:"lease_expires_at,omitempty"`
    LastErrorCode    *string    `json:"last_error_code,omitempty"`
    LastErrorMessage *string    `json:"last_error_message,omitempty"`
}
```

Validation rules used by this component:
- `tenant_id` regex: `^[a-z0-9][a-z0-9-]{1,62}$`
- `idempotency_key` regex: `^[a-zA-Z0-9\-_.]{1,128}$`
- Maximum request body size: `262144` bytes
- Callback URL must use HTTPS and must not resolve to blocked private/link-local/metadata ranges.

---

#### Public Interface

```go
type Factory interface {
    New(component string) *zap.Logger
}
```

##### Example — LoggerFactory

**Request:**
```json
{
  "tenant_id": "acme-prod",
  "job_id": "6c8bb1de-ef20-45cb-9e13-a57f44667d88",
  "run_id": "8a4cfa4d-f89c-4f0e-a809-f7d9a2c8f8ad"
}
```

**Response (200):**
```json
{
  "status": "ok",
  "component": "LoggerFactory"
}
```

**Response (error):**
```json
{
  "error": "operation failed",
  "code": "INTERNAL_ERROR",
  "details": null
}
```

---

#### Internal Logic (step-by-step)

1. Read LOG_LEVEL and LOG_FORMAT from config.
2. Initialise zap logger in JSON mode.
3. Attach default fields: service, component, environment, version.
4. Return component-scoped logger instances.

---

#### Data Model (if this component owns a table)

This component uses shared Release Engine tables and does not introduce a new table in this phase.

---

#### Error Table

| Condition | HTTP Status | Error Code | Response Body |
|---|---|---|---|
| Unsupported log level | 500 | `LOG_LEVEL_INVALID` | `{"error":"unsupported log level","code":"LOG_LEVEL_INVALID","details":null}` |
| Logger initialisation failure | 500 | `LOG_INIT_FAILED` | `{"error":"logger initialisation failed","code":"LOG_INIT_FAILED","details":null}` |

---

#### Acceptance Criteria (Gherkin)

```gherkin
Feature: LoggerFactory

  Scenario: Happy path
    Given valid input and healthy dependencies
    When the component executes
    Then it returns success and emits logs, metrics, and traces

  Scenario: Edge case
    Given replayed state or partial prior progress
    When the component executes again
    Then it behaves deterministically without duplicate side effects

  Scenario: Error
    Given dependency failure or stale run ownership
    When the component executes
    Then it returns a structured error with explicit code
```

---

#### Performance Targets

| Metric | Target |
|---|---|
| Logger creation | <5ms |
| Per-event overhead | <1ms |

---

#### Security Considerations

- Enforce tenant isolation on every SQL read and write.
- Redact or avoid secret values in logs, traces, and error payloads.
- Use fenced writes (`run_id`/`effect_id`) for ownership-sensitive state transitions.

---

#### Observability

- **Log events:** `loggerfactory.start`, `loggerfactory.success`, `loggerfactory.failure`
- **Metrics:** `release_engine_loggerfactory_total{status}`, `release_engine_loggerfactory_duration_seconds`
- **Trace span:** `logging.factory.new`

---

#### Cross-Component Interactions

| Direction | Component | Mechanism | Data Exchanged |
|---|---|---|---|
| This → DBPool | outbound call | pgx query/exec | SQL statements, parameters, transaction result |
| This → MetricsExporter | outbound call | in-process API | counters, histograms, labels |
| This → TracingService | outbound call | OpenTelemetry | span start/end, attributes, errors |
| Upstream runtime → This | inbound call | in-process call | context, tenant_id, job_id, run_id |

### SPEC: DBPool

**File:** `internal/db/pool.go`
**Package:** `db`
**Phase:** 0
**Dependencies:** ConfigLoader

---

#### Purpose

Creates and manages the PostgreSQL connection pool through PgBouncer transaction mode and verifies database readiness.

---

#### Shared Context (duplicated for self-containment)

```go
// ErrorResponse is the standard API error envelope.
type ErrorResponse struct {
    Error   string `json:"error"`
    Code    string `json:"code"`
    Details any    `json:"details,omitempty"`
}

// JobCreateRequest is the POST /v1/jobs request schema.
type JobCreateRequest struct {
    TenantID       string         `json:"tenant_id"`
    PathKey        string         `json:"path_key"`
    Params         map[string]any `json:"params"`
    IdempotencyKey string         `json:"idempotency_key"`
    CallbackURL    *string        `json:"callback_url,omitempty"`
}

// JobStatusResponse is the GET /v1/jobs/{job_id} schema.
type JobStatusResponse struct {
    JobID            string     `json:"job_id"`
    TenantID         string     `json:"tenant_id"`
    State            string     `json:"state"`
    Attempt          int        `json:"attempt"`
    LeaseExpiresAt   *time.Time `json:"lease_expires_at,omitempty"`
    LastErrorCode    *string    `json:"last_error_code,omitempty"`
    LastErrorMessage *string    `json:"last_error_message,omitempty"`
}
```

Validation rules used by this component:
- `tenant_id` regex: `^[a-z0-9][a-z0-9-]{1,62}$`
- `idempotency_key` regex: `^[a-zA-Z0-9\-_.]{1,128}$`
- Maximum request body size: `262144` bytes
- Callback URL must use HTTPS and must not resolve to blocked private/link-local/metadata ranges.

---

#### Public Interface

```go
type Pool interface {
    Acquire(ctx context.Context) (*pgxpool.Conn, error)
    Ping(ctx context.Context) error
    Close()
}
```

##### Example — DBPool

**Request:**
```json
{
  "tenant_id": "acme-prod",
  "job_id": "6c8bb1de-ef20-45cb-9e13-a57f44667d88",
  "run_id": "8a4cfa4d-f89c-4f0e-a809-f7d9a2c8f8ad"
}
```

**Response (200):**
```json
{
  "status": "ok",
  "component": "DBPool"
}
```

**Response (error):**
```json
{
  "error": "operation failed",
  "code": "INTERNAL_ERROR",
  "details": null
}
```

---

#### Internal Logic (step-by-step)

1. Create pgxpool with DB_MAX_CONNS and DB_MIN_CONNS.
2. Apply statement timeout and connection lifetime settings.
3. Run startup ping and isolation-level check (`read committed`).
4. Expose Acquire and Close methods for dependent components.

---

#### Data Model (if this component owns a table)

This component uses shared Release Engine tables and does not introduce a new table in this phase.

---

#### Error Table

| Condition | HTTP Status | Error Code | Response Body |
|---|---|---|---|
| Database unreachable | 503 | `DB_UNAVAILABLE` | `{"error":"database unavailable","code":"DB_UNAVAILABLE","details":null}` |
| Isolation mismatch | 500 | `DB_ISOLATION_INVALID` | `{"error":"invalid isolation level","code":"DB_ISOLATION_INVALID","details":{"expected":"read committed"}}` |

---

#### Acceptance Criteria (Gherkin)

```gherkin
Feature: DBPool

  Scenario: Happy path
    Given valid input and healthy dependencies
    When the component executes
    Then it returns success and emits logs, metrics, and traces

  Scenario: Edge case
    Given replayed state or partial prior progress
    When the component executes again
    Then it behaves deterministically without duplicate side effects

  Scenario: Error
    Given dependency failure or stale run ownership
    When the component executes
    Then it returns a structured error with explicit code
```

---

#### Performance Targets

| Metric | Target |
|---|---|
| Acquire connection p95 | <10ms |
| Ping | <50ms |

---

#### Security Considerations

- Enforce tenant isolation on every SQL read and write.
- Redact or avoid secret values in logs, traces, and error payloads.
- Use fenced writes (`run_id`/`effect_id`) for ownership-sensitive state transitions.

---

#### Observability

- **Log events:** `dbpool.start`, `dbpool.success`, `dbpool.failure`
- **Metrics:** `release_engine_dbpool_total{status}`, `release_engine_dbpool_duration_seconds`
- **Trace span:** `db.pool`

---

#### Cross-Component Interactions

| Direction | Component | Mechanism | Data Exchanged |
|---|---|---|---|
| This → DBPool | outbound call | pgx query/exec | SQL statements, parameters, transaction result |
| This → MetricsExporter | outbound call | in-process API | counters, histograms, labels |
| This → TracingService | outbound call | OpenTelemetry | span start/end, attributes, errors |
| Upstream runtime → This | inbound call | in-process call | context, tenant_id, job_id, run_id |

### SPEC: HTTPServer

**File:** `internal/transport/http/server.go`
**Package:** `http`
**Phase:** 0
**Dependencies:** ConfigLoader, LoggerFactory

---

#### Purpose

Bootstraps Echo HTTP server, registers middleware and routes, and controls graceful shutdown semantics.

---

#### Shared Context (duplicated for self-containment)

```go
// ErrorResponse is the standard API error envelope.
type ErrorResponse struct {
    Error   string `json:"error"`
    Code    string `json:"code"`
    Details any    `json:"details,omitempty"`
}

// JobCreateRequest is the POST /v1/jobs request schema.
type JobCreateRequest struct {
    TenantID       string         `json:"tenant_id"`
    PathKey        string         `json:"path_key"`
    Params         map[string]any `json:"params"`
    IdempotencyKey string         `json:"idempotency_key"`
    CallbackURL    *string        `json:"callback_url,omitempty"`
}

// JobStatusResponse is the GET /v1/jobs/{job_id} schema.
type JobStatusResponse struct {
    JobID            string     `json:"job_id"`
    TenantID         string     `json:"tenant_id"`
    State            string     `json:"state"`
    Attempt          int        `json:"attempt"`
    LeaseExpiresAt   *time.Time `json:"lease_expires_at,omitempty"`
    LastErrorCode    *string    `json:"last_error_code,omitempty"`
    LastErrorMessage *string    `json:"last_error_message,omitempty"`
}
```

Validation rules used by this component:
- `tenant_id` regex: `^[a-z0-9][a-z0-9-]{1,62}$`
- `idempotency_key` regex: `^[a-zA-Z0-9\-_.]{1,128}$`
- Maximum request body size: `262144` bytes
- Callback URL must use HTTPS and must not resolve to blocked private/link-local/metadata ranges.

---

#### Public Interface

```go
type Server interface {
    RegisterRoutes()
    Start(ctx context.Context) error
    Shutdown(ctx context.Context) error
}
```

##### Example — HTTPServer

**Request:**
```json
{
  "tenant_id": "acme-prod",
  "job_id": "6c8bb1de-ef20-45cb-9e13-a57f44667d88",
  "run_id": "8a4cfa4d-f89c-4f0e-a809-f7d9a2c8f8ad"
}
```

**Response (200):**
```json
{
  "status": "ok",
  "component": "HTTPServer"
}
```

**Response (error):**
```json
{
  "error": "operation failed",
  "code": "INTERNAL_ERROR",
  "details": null
}
```

---

#### Internal Logic (step-by-step)

1. Initialise Echo with request size limit and timeouts.
2. Register health and job routes.
3. Install panic recovery, request ID propagation, and access logging middleware.
4. On shutdown, stop accepting new connections and wait up to RUNNER_DRAIN_TIMEOUT.

---

#### Data Model (if this component owns a table)

This component uses shared Release Engine tables and does not introduce a new table in this phase.

---

#### Error Table

| Condition | HTTP Status | Error Code | Response Body |
|---|---|---|---|
| Port bind failure | 500 | `HTTP_BIND_FAILED` | `{"error":"http bind failed","code":"HTTP_BIND_FAILED","details":null}` |
| Graceful shutdown timeout | 503 | `HTTP_SHUTDOWN_TIMEOUT` | `{"error":"shutdown timed out","code":"HTTP_SHUTDOWN_TIMEOUT","details":null}` |

---

#### Acceptance Criteria (Gherkin)

```gherkin
Feature: HTTPServer

  Scenario: Happy path
    Given valid input and healthy dependencies
    When the component executes
    Then it returns success and emits logs, metrics, and traces

  Scenario: Edge case
    Given replayed state or partial prior progress
    When the component executes again
    Then it behaves deterministically without duplicate side effects

  Scenario: Error
    Given dependency failure or stale run ownership
    When the component executes
    Then it returns a structured error with explicit code
```

---

#### Performance Targets

| Metric | Target |
|---|---|
| Request routing overhead | <2ms |
| Shutdown drain | <=30s |

---

#### Security Considerations

- Enforce tenant isolation on every SQL read and write.
- Redact or avoid secret values in logs, traces, and error payloads.
- Use fenced writes (`run_id`/`effect_id`) for ownership-sensitive state transitions.

---

#### Observability

- **Log events:** `httpserver.start`, `httpserver.success`, `httpserver.failure`
- **Metrics:** `release_engine_httpserver_total{status}`, `release_engine_httpserver_duration_seconds`
- **Trace span:** `http.server`

---

#### Cross-Component Interactions

| Direction | Component | Mechanism | Data Exchanged |
|---|---|---|---|
| This → DBPool | outbound call | pgx query/exec | SQL statements, parameters, transaction result |
| This → MetricsExporter | outbound call | in-process API | counters, histograms, labels |
| This → TracingService | outbound call | OpenTelemetry | span start/end, attributes, errors |
| Upstream runtime → This | inbound call | in-process call | context, tenant_id, job_id, run_id |

### SPEC: AuthMiddleware

**File:** `internal/transport/http/middleware/auth.go`
**Package:** `middleware`
**Phase:** 1
**Dependencies:** ConfigLoader, HTTPServer

---

#### Purpose

Validates OIDC JWT bearer tokens and injects verified claims into request context.

---

#### Shared Context (duplicated for self-containment)

```go
// ErrorResponse is the standard API error envelope.
type ErrorResponse struct {
    Error   string `json:"error"`
    Code    string `json:"code"`
    Details any    `json:"details,omitempty"`
}

// JobCreateRequest is the POST /v1/jobs request schema.
type JobCreateRequest struct {
    TenantID       string         `json:"tenant_id"`
    PathKey        string         `json:"path_key"`
    Params         map[string]any `json:"params"`
    IdempotencyKey string         `json:"idempotency_key"`
    CallbackURL    *string        `json:"callback_url,omitempty"`
}

// JobStatusResponse is the GET /v1/jobs/{job_id} schema.
type JobStatusResponse struct {
    JobID            string     `json:"job_id"`
    TenantID         string     `json:"tenant_id"`
    State            string     `json:"state"`
    Attempt          int        `json:"attempt"`
    LeaseExpiresAt   *time.Time `json:"lease_expires_at,omitempty"`
    LastErrorCode    *string    `json:"last_error_code,omitempty"`
    LastErrorMessage *string    `json:"last_error_message,omitempty"`
}
```

Validation rules used by this component:
- `tenant_id` regex: `^[a-z0-9][a-z0-9-]{1,62}$`
- `idempotency_key` regex: `^[a-zA-Z0-9\-_.]{1,128}$`
- Maximum request body size: `262144` bytes
- Callback URL must use HTTPS and must not resolve to blocked private/link-local/metadata ranges.

---

#### Public Interface

```go
func Auth(next echo.HandlerFunc) echo.HandlerFunc
```

##### Example — AuthMiddleware

**Request:**
```json
{
  "tenant_id": "acme-prod",
  "job_id": "6c8bb1de-ef20-45cb-9e13-a57f44667d88",
  "run_id": "8a4cfa4d-f89c-4f0e-a809-f7d9a2c8f8ad"
}
```

**Response (200):**
```json
{
  "status": "ok",
  "component": "AuthMiddleware"
}
```

**Response (error):**
```json
{
  "error": "operation failed",
  "code": "INTERNAL_ERROR",
  "details": null
}
```

---

#### Internal Logic (step-by-step)

1. Extract `Authorization: Bearer` token.
2. Validate signature using JWKS cache; refresh every 5 minutes.
3. Validate issuer, audience, expiry, and maximum 60-second clock skew.
4. Inject AuthClaims into context for downstream handlers.

---

#### Data Model (if this component owns a table)

This component uses shared Release Engine tables and does not introduce a new table in this phase.

---

#### Error Table

| Condition | HTTP Status | Error Code | Response Body |
|---|---|---|---|
| Missing token | 401 | `AUTH_MISSING` | `{"error":"missing bearer token","code":"AUTH_MISSING","details":null}` |
| Invalid token | 401 | `AUTH_INVALID` | `{"error":"invalid token","code":"AUTH_INVALID","details":null}` |

---

#### Acceptance Criteria (Gherkin)

```gherkin
Feature: AuthMiddleware

  Scenario: Happy path
    Given valid input and healthy dependencies
    When the component executes
    Then it returns success and emits logs, metrics, and traces

  Scenario: Edge case
    Given replayed state or partial prior progress
    When the component executes again
    Then it behaves deterministically without duplicate side effects

  Scenario: Error
    Given dependency failure or stale run ownership
    When the component executes
    Then it returns a structured error with explicit code
```

---

#### Performance Targets

| Metric | Target |
|---|---|
| Token validation p95 | <15ms |
| JWKS cache hit | >99% |

---

#### Security Considerations

- Enforce tenant isolation on every SQL read and write.
- Redact or avoid secret values in logs, traces, and error payloads.
- Use fenced writes (`run_id`/`effect_id`) for ownership-sensitive state transitions.

---

#### Observability

- **Log events:** `auth.start`, `auth.success`, `auth.failure`
- **Metrics:** `release_engine_auth_total{status}`, `release_engine_auth_duration_seconds`
- **Trace span:** `http.auth`

---

#### Cross-Component Interactions

| Direction | Component | Mechanism | Data Exchanged |
|---|---|---|---|
| This → DBPool | outbound call | pgx query/exec | SQL statements, parameters, transaction result |
| This → MetricsExporter | outbound call | in-process API | counters, histograms, labels |
| This → TracingService | outbound call | OpenTelemetry | span start/end, attributes, errors |
| Upstream runtime → This | inbound call | in-process call | context, tenant_id, job_id, run_id |

### SPEC: RateLimiter

**File:** `internal/transport/http/middleware/rate_limiter.go`
**Package:** `middleware`
**Phase:** 1
**Dependencies:** ConfigLoader

---

#### Purpose

Applies per-tenant token-bucket rate limiting for intake endpoints.

---

#### Shared Context (duplicated for self-containment)

```go
// ErrorResponse is the standard API error envelope.
type ErrorResponse struct {
    Error   string `json:"error"`
    Code    string `json:"code"`
    Details any    `json:"details,omitempty"`
}

// JobCreateRequest is the POST /v1/jobs request schema.
type JobCreateRequest struct {
    TenantID       string         `json:"tenant_id"`
    PathKey        string         `json:"path_key"`
    Params         map[string]any `json:"params"`
    IdempotencyKey string         `json:"idempotency_key"`
    CallbackURL    *string        `json:"callback_url,omitempty"`
}

// JobStatusResponse is the GET /v1/jobs/{job_id} schema.
type JobStatusResponse struct {
    JobID            string     `json:"job_id"`
    TenantID         string     `json:"tenant_id"`
    State            string     `json:"state"`
    Attempt          int        `json:"attempt"`
    LeaseExpiresAt   *time.Time `json:"lease_expires_at,omitempty"`
    LastErrorCode    *string    `json:"last_error_code,omitempty"`
    LastErrorMessage *string    `json:"last_error_message,omitempty"`
}
```

Validation rules used by this component:
- `tenant_id` regex: `^[a-z0-9][a-z0-9-]{1,62}$`
- `idempotency_key` regex: `^[a-zA-Z0-9\-_.]{1,128}$`
- Maximum request body size: `262144` bytes
- Callback URL must use HTTPS and must not resolve to blocked private/link-local/metadata ranges.

---

#### Public Interface

```go
func RateLimit(next echo.HandlerFunc) echo.HandlerFunc
```

##### Example — RateLimiter

**Request:**
```json
{
  "tenant_id": "acme-prod",
  "job_id": "6c8bb1de-ef20-45cb-9e13-a57f44667d88",
  "run_id": "8a4cfa4d-f89c-4f0e-a809-f7d9a2c8f8ad"
}
```

**Response (200):**
```json
{
  "status": "ok",
  "component": "RateLimiter"
}
```

**Response (error):**
```json
{
  "error": "operation failed",
  "code": "INTERNAL_ERROR",
  "details": null
}
```

---

#### Internal Logic (step-by-step)

1. Resolve tenant_id from claims.
2. Consume one token from tenant bucket.
3. If bucket empty, return HTTP 429 with Retry-After header.
4. Emit per-tenant rate-limited metric.

---

#### Data Model (if this component owns a table)

This component uses shared Release Engine tables and does not introduce a new table in this phase.

---

#### Error Table

| Condition | HTTP Status | Error Code | Response Body |
|---|---|---|---|
| Rate limit exceeded | 429 | `ERR_RATE_LIMITED` | `{"error":"rate limit exceeded","code":"ERR_RATE_LIMITED","details":null}` |
| Limiter state unavailable | 503 | `RATE_LIMIT_UNAVAILABLE` | `{"error":"rate limiter unavailable","code":"RATE_LIMIT_UNAVAILABLE","details":null}` |

---

#### Acceptance Criteria (Gherkin)

```gherkin
Feature: RateLimiter

  Scenario: Happy path
    Given valid input and healthy dependencies
    When the component executes
    Then it returns success and emits logs, metrics, and traces

  Scenario: Edge case
    Given replayed state or partial prior progress
    When the component executes again
    Then it behaves deterministically without duplicate side effects

  Scenario: Error
    Given dependency failure or stale run ownership
    When the component executes
    Then it returns a structured error with explicit code
```

---

#### Performance Targets

| Metric | Target |
|---|---|
| Limiter decision | <1ms |
| Tenant bucket lookup | <1ms |

---

#### Security Considerations

- Enforce tenant isolation on every SQL read and write.
- Redact or avoid secret values in logs, traces, and error payloads.
- Use fenced writes (`run_id`/`effect_id`) for ownership-sensitive state transitions.

---

#### Observability

- **Log events:** `ratelimiter.start`, `ratelimiter.success`, `ratelimiter.failure`
- **Metrics:** `release_engine_ratelimiter_total{status}`, `release_engine_ratelimiter_duration_seconds`
- **Trace span:** `http.rate_limit`

---

#### Cross-Component Interactions

| Direction | Component | Mechanism | Data Exchanged |
|---|---|---|---|
| This → DBPool | outbound call | pgx query/exec | SQL statements, parameters, transaction result |
| This → MetricsExporter | outbound call | in-process API | counters, histograms, labels |
| This → TracingService | outbound call | OpenTelemetry | span start/end, attributes, errors |
| Upstream runtime → This | inbound call | in-process call | context, tenant_id, job_id, run_id |

### SPEC: PolicyEngine

**File:** `internal/policy/engine.go`
**Package:** `policy`
**Phase:** 1
**Dependencies:** ConfigLoader, DBPool

---

#### Purpose

Evaluates RBAC and policy decisions in-process using the Go-native evaluator defined in DD-03.

---

#### Shared Context (duplicated for self-containment)

```go
// ErrorResponse is the standard API error envelope.
type ErrorResponse struct {
    Error   string `json:"error"`
    Code    string `json:"code"`
    Details any    `json:"details,omitempty"`
}

// JobCreateRequest is the POST /v1/jobs request schema.
type JobCreateRequest struct {
    TenantID       string         `json:"tenant_id"`
    PathKey        string         `json:"path_key"`
    Params         map[string]any `json:"params"`
    IdempotencyKey string         `json:"idempotency_key"`
    CallbackURL    *string        `json:"callback_url,omitempty"`
}

// JobStatusResponse is the GET /v1/jobs/{job_id} schema.
type JobStatusResponse struct {
    JobID            string     `json:"job_id"`
    TenantID         string     `json:"tenant_id"`
    State            string     `json:"state"`
    Attempt          int        `json:"attempt"`
    LeaseExpiresAt   *time.Time `json:"lease_expires_at,omitempty"`
    LastErrorCode    *string    `json:"last_error_code,omitempty"`
    LastErrorMessage *string    `json:"last_error_message,omitempty"`
}
```

Validation rules used by this component:
- `tenant_id` regex: `^[a-z0-9][a-z0-9-]{1,62}$`
- `idempotency_key` regex: `^[a-zA-Z0-9\-_.]{1,128}$`
- Maximum request body size: `262144` bytes
- Callback URL must use HTTPS and must not resolve to blocked private/link-local/metadata ranges.

---

#### Public Interface

```go
type Evaluator interface {
    Evaluate(ctx context.Context, input PolicyInput) (PolicyDecision, error)
    EvaluateApproval(input ApprovalPolicyInput) ApprovalPolicyResult
    Reload(ctx context.Context) error
}
```

##### Example — PolicyEngine

**Request:**
```json
{
  "tenant_id": "acme-prod",
  "job_id": "6c8bb1de-ef20-45cb-9e13-a57f44667d88",
  "run_id": "8a4cfa4d-f89c-4f0e-a809-f7d9a2c8f8ad"
}
```

**Response (200):**
```json
{
  "status": "ok",
  "component": "PolicyEngine"
}
```

**Response (error):**
```json
{
  "error": "operation failed",
  "code": "INTERNAL_ERROR",
  "details": null
}
```

---

#### Internal Logic (step-by-step)

1. Build PolicyInput from claims, action, tenant, and resource path.
2. Evaluate policy rules in-memory with cached bindings.
3. Bypass cache for destructive action `job:cancel`.
4. For approval decisions, evaluate `ApprovalPolicyInput` with guardrails:
   - self-approval block when policy sets `self_approval=false`
   - role allow-list match against `allowed_roles`
   - tenant scope match (`approver_tenant_id == job_tenant_id`)
   - optional budget authority check (`estimated_cost <= approver_limit` when both metadata values are present)
   - role matching is case-insensitive.
5. `EvaluateApproval()` returns a pure authorisation decision for one submitted decision; it does not apply four-eyes progression transitions.
6. Four-eyes progression (`approved_count >= min_approvers`) is enforced by `ApprovalService` after successful decision persistence.
7. Write audit event for allow/deny decision.

---

#### Data Model (if this component owns a table)

This component uses shared Release Engine tables and does not introduce a new table in this phase.

---

#### Error Table

| Condition | HTTP Status | Error Code | Response Body |
|---|---|---|---|
| Policy denied | 403 | `ERR_POLICY_DENIED` | `{"error":"policy denied","code":"ERR_POLICY_DENIED","details":null}` |
| Approval policy violation | 422 | `POLICY_VIOLATION` | `{"error":"policy violation","code":"POLICY_VIOLATION","details":null}` |
| Policy evaluation error | 503 | `POLICY_EVAL_FAILED` | `{"error":"policy evaluation failed","code":"POLICY_EVAL_FAILED","details":null}` |

---

#### Acceptance Criteria (Gherkin)

```gherkin
Feature: PolicyEngine

  Scenario: Happy path
    Given valid input and healthy dependencies
    When the component executes
    Then it returns success and emits logs, metrics, and traces

  Scenario: Edge case
    Given replayed state or partial prior progress
    When the component executes again
    Then it behaves deterministically without duplicate side effects

  Scenario: Error
    Given dependency failure or stale run ownership
    When the component executes
    Then it returns a structured error with explicit code
```

---

#### Performance Targets

| Metric | Target |
|---|---|
| Policy decision p95 | <8ms |
| Cache hit ratio | >95% for non-destructive actions |

---

#### Security Considerations

- Enforce tenant isolation on every SQL read and write.
- Redact or avoid secret values in logs, traces, and error payloads.
- Use fenced writes (`run_id`/`effect_id`) for ownership-sensitive state transitions.

---

#### Observability

- **Log events:** `policyengine.start`, `policyengine.success`, `policyengine.failure`
- **Metrics:** `release_engine_policyengine_total{status}`, `release_engine_policyengine_duration_seconds`
- **Trace span:** `policy.evaluate`

---

#### Cross-Component Interactions

| Direction | Component | Mechanism | Data Exchanged |
|---|---|---|---|
| This → DBPool | outbound call | pgx query/exec | SQL statements, parameters, transaction result |
| This → MetricsExporter | outbound call | in-process API | counters, histograms, labels |
| This → TracingService | outbound call | OpenTelemetry | span start/end, attributes, errors |
| Upstream runtime → This | inbound call | in-process call | context, tenant_id, job_id, run_id |

### SPEC: IdempotencyService

**File:** `internal/idempotency/service.go`
**Package:** `idempotency`
**Phase:** 1
**Dependencies:** DBPool, PolicyEngine

---

#### Purpose

Implements deterministic job intake using `(tenant_id, path_key, idempotency_key)` uniqueness and payload fingerprint conflict detection.

---

#### Shared Context (duplicated for self-containment)

```go
// ErrorResponse is the standard API error envelope.
type ErrorResponse struct {
    Error   string `json:"error"`
    Code    string `json:"code"`
    Details any    `json:"details,omitempty"`
}

// JobCreateRequest is the POST /v1/jobs request schema.
type JobCreateRequest struct {
    TenantID       string         `json:"tenant_id"`
    PathKey        string         `json:"path_key"`
    Params         map[string]any `json:"params"`
    IdempotencyKey string         `json:"idempotency_key"`
    CallbackURL    *string        `json:"callback_url,omitempty"`
}

// JobStatusResponse is the GET /v1/jobs/{job_id} schema.
type JobStatusResponse struct {
    JobID            string     `json:"job_id"`
    TenantID         string     `json:"tenant_id"`
    State            string     `json:"state"`
    Attempt          int        `json:"attempt"`
    LeaseExpiresAt   *time.Time `json:"lease_expires_at,omitempty"`
    LastErrorCode    *string    `json:"last_error_code,omitempty"`
    LastErrorMessage *string    `json:"last_error_message,omitempty"`
}
```

Validation rules used by this component:
- `tenant_id` regex: `^[a-z0-9][a-z0-9-]{1,62}$`
- `idempotency_key` regex: `^[a-zA-Z0-9\-_.]{1,128}$`
- Maximum request body size: `262144` bytes
- Callback URL must use HTTPS and must not resolve to blocked private/link-local/metadata ranges.

---

#### Public Interface

```go
type Service interface {
    ReserveOrReplay(ctx context.Context, req JobCreateRequest) (status int, envelope any, replayed bool, err error)
}
```

##### Example — IdempotencyService

**Request:**
```json
{
  "tenant_id": "acme-prod",
  "job_id": "6c8bb1de-ef20-45cb-9e13-a57f44667d88",
  "run_id": "8a4cfa4d-f89c-4f0e-a809-f7d9a2c8f8ad"
}
```

**Response (200):**
```json
{
  "status": "ok",
  "component": "IdempotencyService"
}
```

**Response (error):**
```json
{
  "error": "operation failed",
  "code": "INTERNAL_ERROR",
  "details": null
}
```

---

#### Internal Logic (step-by-step)

1. Normalise request payload and compute SHA-256 fingerprint.
   - Canonical fingerprint inputs include `path_key`, `params`, optional `callback_url`, optional `schedule`, and optional `first_run_at`.
2. Reserve idempotency key in `idempotency_keys` with 48-hour expiry inside intake transaction.
3. On first-write path, create `jobs` and `jobs_read` in the same transaction and persist canonical intake envelope.
4. On replay path, compare fingerprints: exact match replays the stored canonical envelope (status fixed at first write); mismatch returns `409 ERR_IDEM_CONFLICT`.

---

#### Data Model (if this component owns a table)

This component uses shared Release Engine tables and does not introduce a new table in this phase.

---

#### Error Table

| Condition | HTTP Status | Error Code | Response Body |
|---|---|---|---|
| Payload mismatch for existing key | 409 | `ERR_IDEM_CONFLICT` | `{"error":"idempotency key reused with different payload","code":"ERR_IDEM_CONFLICT","details":null}` |
| Transaction failure | 503 | `IDEM_TX_FAILED` | `{"error":"idempotency transaction failed","code":"IDEM_TX_FAILED","details":null}` |

---

#### Acceptance Criteria (Gherkin)

```gherkin
Feature: IdempotencyService

  Scenario: Happy path
    Given valid input and healthy dependencies
    When the component executes
    Then it returns success and emits logs, metrics, and traces

  Scenario: Edge case
    Given replayed state or partial prior progress
    When the component executes again
    Then it behaves deterministically without duplicate side effects

  Scenario: Error
    Given dependency failure or stale run ownership
    When the component executes
    Then it returns a structured error with explicit code
```

---

#### Performance Targets

| Metric | Target |
|---|---|
| First-write intake p95 | <120ms |
| Replay response p95 | <40ms |

---

#### Security Considerations

- Enforce tenant isolation on every SQL read and write.
- Redact or avoid secret values in logs, traces, and error payloads.
- Use fenced writes (`run_id`/`effect_id`) for ownership-sensitive state transitions.

---

#### Observability

- **Log events:** `idempotency.start`, `idempotency.success`, `idempotency.failure`
- **Metrics:** `release_engine_idempotency_total{status}`, `release_engine_idempotency_duration_seconds`
- **Trace span:** `idempotency.reserve_or_replay`

---

#### Cross-Component Interactions

| Direction | Component | Mechanism | Data Exchanged |
|---|---|---|---|
| This → DBPool | outbound call | pgx query/exec | SQL statements, parameters, transaction result |
| This → MetricsExporter | outbound call | in-process API | counters, histograms, labels |
| This → TracingService | outbound call | OpenTelemetry | span start/end, attributes, errors |
| Upstream runtime → This | inbound call | in-process call | context, tenant_id, job_id, run_id |

### SPEC: JobsAPIHandler

**File:** `internal/transport/http/handler/jobs.go`
**Package:** `handler`
**Phase:** 1
**Dependencies:** AuthMiddleware, RateLimiter, PolicyEngine, IdempotencyService, DBPool

---

#### Purpose

Exposes the public Jobs HTTP API: create, read status, and cancel endpoints with strict validation and deterministic responses.

---

#### Shared Context (duplicated for self-containment)

```go
// ErrorResponse is the standard API error envelope.
type ErrorResponse struct {
    Error   string `json:"error"`
    Code    string `json:"code"`
    Details any    `json:"details,omitempty"`
}

// JobCreateRequest is the POST /v1/jobs request schema.
type JobCreateRequest struct {
    TenantID       string         `json:"tenant_id"`
    PathKey        string         `json:"path_key"`
    Params         map[string]any `json:"params"`
    IdempotencyKey string         `json:"idempotency_key"`
    CallbackURL    *string        `json:"callback_url,omitempty"`
    MaxAttempts    int            `json:"max_attempts,omitempty"`
    FirstRunAt     *time.Time     `json:"first_run_at,omitempty"`
    Schedule       string         `json:"schedule,omitempty"`
}

// JobStatusResponse is the GET /v1/jobs/{job_id} schema.
type JobStatusResponse struct {
    JobID            string     `json:"job_id"`
    TenantID         string     `json:"tenant_id"`
    State            string     `json:"state"`
    Attempt          int        `json:"attempt"`
    LeaseExpiresAt   *time.Time `json:"lease_expires_at,omitempty"`
    LastErrorCode    *string    `json:"last_error_code,omitempty"`
    LastErrorMessage *string    `json:"last_error_message,omitempty"`
}
```

Validation rules used by this component:
- `tenant_id` regex: `^[a-z0-9][a-z0-9-]{1,62}$`
- `idempotency_key` regex: `^[a-zA-Z0-9\-_.]{1,128}$`
- Maximum request body size: `262144` bytes
- Callback URL must use HTTPS and must not resolve to blocked private/link-local/metadata ranges.

---

#### Public Interface

```go
POST   /v1/jobs
GET    /v1/jobs
GET    /v1/jobs/:job_id
POST   /v1/jobs/:job_id/cancel
POST   /v1/jobs/:job_id/steps/:step_id/decisions
GET    /v1/jobs/:job_id/steps/:step_id/approval-context
```

##### Example — JobsAPIHandler

**Request:**
```json
{
  "tenant_id": "acme-prod",
  "job_id": "6c8bb1de-ef20-45cb-9e13-a57f44667d88",
  "run_id": "8a4cfa4d-f89c-4f0e-a809-f7d9a2c8f8ad"
}
```

**Response (200):**
```json
{
  "status": "ok",
  "component": "JobsAPIHandler"
}
```

**Response (error):**
```json
{
  "error": "operation failed",
  "code": "INTERNAL_ERROR",
  "details": null
}
```

---

#### Internal Logic (step-by-step)

1. Validate request body schema and size limit (256 KB).
2. Validate callback URL against SSRF and allowlist rules.
3. Validate optional `schedule` as cron expression; reject invalid format with `ERR_INVALID_SCHEDULE`.
4. Normalise optional `max_attempts` (default `3`) and `first_run_at` (default acceptance time).
5. Authorise action using PolicyEngine (`job:create`, `job:read`, `job:cancel`).
6. Delegate intake to IdempotencyService and return stable headers (`Idempotency-Key`, `Idempotency-Replayed`, `Location`).
7. Accept decision submissions for `waiting_approval` steps with idempotency support.
8. Expose pending approval job queries and step approval context read API.
9. Enforce decision outcomes:
   - `rejected` transitions step state to `error` immediately.
   - `approved` keeps step in `waiting_approval` until policy threshold is met.
   - transition to `running` only when `approved_count >= min_approvers`.

---

#### Data Model (if this component owns a table)

This component uses shared Release Engine tables and does not introduce a new table in this phase.

---

#### Error Table

| Condition | HTTP Status | Error Code | Response Body |
|---|---|---|---|
| Invalid callback URL | 400 | `ERR_INVALID_CALLBACK_URL` | `{"error":"invalid callback URL","code":"ERR_INVALID_CALLBACK_URL","details":null}` |
| Invalid schedule cron expression | 400 | `ERR_INVALID_SCHEDULE` | `{"error":"invalid schedule","code":"ERR_INVALID_SCHEDULE","details":null}` |
| Payload too large | 413 | `ERR_PAYLOAD_TOO_LARGE` | `{"error":"payload too large","code":"ERR_PAYLOAD_TOO_LARGE","details":null}` |
| Step not waiting approval / decision conflict | 409 | `CONFLICT` | `{"error":"step is not waiting approval","code":"CONFLICT","details":null}` |
| Approver role forbidden | 403 | `FORBIDDEN` | `{"error":"approver not authorised","code":"FORBIDDEN","details":null}` |
| Approval policy violation (self-approval) | 422 | `POLICY_VIOLATION` | `{"error":"policy violation","code":"POLICY_VIOLATION","details":null}` |

---

#### Acceptance Criteria (Gherkin)

```gherkin
Feature: JobsAPIHandler

  Scenario: Happy path
    Given valid input and healthy dependencies
    When the component executes
    Then it returns success and emits logs, metrics, and traces

  Scenario: Edge case
    Given replayed state or partial prior progress
    When the component executes again
    Then it behaves deterministically without duplicate side effects

  Scenario: Error
    Given dependency failure or stale run ownership
    When the component executes
    Then it returns a structured error with explicit code
```

---

#### Performance Targets

| Metric | Target |
|---|---|
| POST /v1/jobs p95 | <200ms |
| GET /v1/jobs/{id} p95 | <50ms |

---

#### Security Considerations

- Enforce tenant isolation on every SQL read and write.
- Redact or avoid secret values in logs, traces, and error payloads.
- Use fenced writes (`run_id`/`effect_id`) for ownership-sensitive state transitions.

---

#### Observability

- **Log events:** `jobsapi.start`, `jobsapi.success`, `jobsapi.failure`
- **Metrics:** `release_engine_jobsapi_total{status}`, `release_engine_jobsapi_duration_seconds`
- **Trace span:** `http.jobs`

---

#### Cross-Component Interactions

| Direction | Component | Mechanism | Data Exchanged |
|---|---|---|---|
| This → DBPool | outbound call | pgx query/exec | SQL statements, parameters, transaction result |
| This → MetricsExporter | outbound call | in-process API | counters, histograms, labels |
| This → TracingService | outbound call | OpenTelemetry | span start/end, attributes, errors |
| Upstream runtime → This | inbound call | in-process call | context, tenant_id, job_id, run_id |

### SPEC: HealthHandler

**File:** `internal/transport/http/handler/health.go`
**Package:** `handler`
**Phase:** 1
**Dependencies:** DBPool, SchedulerService, MigrationChecker

---

#### Purpose

Provides liveness and readiness endpoints for orchestration and load balancer health checks.

---

#### Shared Context (duplicated for self-containment)

```go
// ErrorResponse is the standard API error envelope.
type ErrorResponse struct {
    Error   string `json:"error"`
    Code    string `json:"code"`
    Details any    `json:"details,omitempty"`
}

// JobCreateRequest is the POST /v1/jobs request schema.
type JobCreateRequest struct {
    TenantID       string         `json:"tenant_id"`
    PathKey        string         `json:"path_key"`
    Params         map[string]any `json:"params"`
    IdempotencyKey string         `json:"idempotency_key"`
    CallbackURL    *string        `json:"callback_url,omitempty"`
}

// JobStatusResponse is the GET /v1/jobs/{job_id} schema.
type JobStatusResponse struct {
    JobID            string     `json:"job_id"`
    TenantID         string     `json:"tenant_id"`
    State            string     `json:"state"`
    Attempt          int        `json:"attempt"`
    LeaseExpiresAt   *time.Time `json:"lease_expires_at,omitempty"`
    LastErrorCode    *string    `json:"last_error_code,omitempty"`
    LastErrorMessage *string    `json:"last_error_message,omitempty"`
}
```

Validation rules used by this component:
- `tenant_id` regex: `^[a-z0-9][a-z0-9-]{1,62}$`
- `idempotency_key` regex: `^[a-zA-Z0-9\-_.]{1,128}$`
- Maximum request body size: `262144` bytes
- Callback URL must use HTTPS and must not resolve to blocked private/link-local/metadata ranges.

---

#### Public Interface

```go
GET /healthz
GET /readyz
```

##### Example — HealthHandler

**Request:**
```json
{
  "tenant_id": "acme-prod",
  "job_id": "6c8bb1de-ef20-45cb-9e13-a57f44667d88",
  "run_id": "8a4cfa4d-f89c-4f0e-a809-f7d9a2c8f8ad"
}
```

**Response (200):**
```json
{
  "status": "ok",
  "component": "HealthHandler"
}
```

**Response (error):**
```json
{
  "error": "operation failed",
  "code": "INTERNAL_ERROR",
  "details": null
}
```

---

#### Internal Logic (step-by-step)

1. `/healthz` returns 200 when process loop is alive.
2. `/readyz` checks DB connectivity, schema migration level, and scheduler loop state.
3. Return 503 with explicit reasons when not ready.

---

#### Data Model (if this component owns a table)

This component uses shared Release Engine tables and does not introduce a new table in this phase.

---

#### Error Table

| Condition | HTTP Status | Error Code | Response Body |
|---|---|---|---|
| Dependency not ready | 503 | `NOT_READY` | `{"error":"service not ready","code":"NOT_READY","details":{"db":false}}` |
| Internal health check error | 500 | `HEALTH_CHECK_FAILED` | `{"error":"health check failed","code":"HEALTH_CHECK_FAILED","details":null}` |

---

#### Acceptance Criteria (Gherkin)

```gherkin
Feature: HealthHandler

  Scenario: Happy path
    Given valid input and healthy dependencies
    When the component executes
    Then it returns success and emits logs, metrics, and traces

  Scenario: Edge case
    Given replayed state or partial prior progress
    When the component executes again
    Then it behaves deterministically without duplicate side effects

  Scenario: Error
    Given dependency failure or stale run ownership
    When the component executes
    Then it returns a structured error with explicit code
```

---

#### Performance Targets

| Metric | Target |
|---|---|
| Health check p95 | <20ms |
| Readiness check p95 | <80ms |

---

#### Security Considerations

- Enforce tenant isolation on every SQL read and write.
- Redact or avoid secret values in logs, traces, and error payloads.
- Use fenced writes (`run_id`/`effect_id`) for ownership-sensitive state transitions.

---

#### Observability

- **Log events:** `health.start`, `health.success`, `health.failure`
- **Metrics:** `release_engine_health_total{status}`, `release_engine_health_duration_seconds`
- **Trace span:** `http.health`

---

#### Cross-Component Interactions

| Direction | Component | Mechanism | Data Exchanged |
|---|---|---|---|
| This → DBPool | outbound call | pgx query/exec | SQL statements, parameters, transaction result |
| This → MetricsExporter | outbound call | in-process API | counters, histograms, labels |
| This → TracingService | outbound call | OpenTelemetry | span start/end, attributes, errors |
| Upstream runtime → This | inbound call | in-process call | context, tenant_id, job_id, run_id |

### SPEC: SchedulerService

**File:** `internal/scheduler/service.go`
**Package:** `scheduler`
**Phase:** 1
**Dependencies:** DBPool, ModuleRegistry, MetricsExporter

---

#### Purpose

Claims runnable jobs using SKIP LOCKED and weighted fair sharing (DD-06), then dispatches them to the runner. Claim semantics include due queued jobs (`next_run_at <= now()`) and expired running jobs (`lease_expires_at <= now()`) in one atomic query.

---

#### Shared Context (duplicated for self-containment)

```go
// ErrorResponse is the standard API error envelope.
type ErrorResponse struct {
    Error   string `json:"error"`
    Code    string `json:"code"`
    Details any    `json:"details,omitempty"`
}

// JobCreateRequest is the POST /v1/jobs request schema.
type JobCreateRequest struct {
    TenantID       string         `json:"tenant_id"`
    PathKey        string         `json:"path_key"`
    Params         map[string]any `json:"params"`
    IdempotencyKey string         `json:"idempotency_key"`
    CallbackURL    *string        `json:"callback_url,omitempty"`
}

// JobStatusResponse is the GET /v1/jobs/{job_id} schema.
type JobStatusResponse struct {
    JobID            string     `json:"job_id"`
    TenantID         string     `json:"tenant_id"`
    State            string     `json:"state"`
    Attempt          int        `json:"attempt"`
    LeaseExpiresAt   *time.Time `json:"lease_expires_at,omitempty"`
    LastErrorCode    *string    `json:"last_error_code,omitempty"`
    LastErrorMessage *string    `json:"last_error_message,omitempty"`
}
```

Validation rules used by this component:
- `tenant_id` regex: `^[a-z0-9][a-z0-9-]{1,62}$`
- `idempotency_key` regex: `^[a-zA-Z0-9\-_.]{1,128}$`
- Maximum request body size: `262144` bytes
- Callback URL must use HTTPS and must not resolve to blocked private/link-local/metadata ranges.

---

#### Public Interface

```go
type Scheduler interface {
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
}
```

##### Example — SchedulerService

**Request:**
```json
{
  "tenant_id": "acme-prod",
  "job_id": "6c8bb1de-ef20-45cb-9e13-a57f44667d88",
  "run_id": "8a4cfa4d-f89c-4f0e-a809-f7d9a2c8f8ad"
}
```

**Response (200):**
```json
{
  "status": "ok",
  "component": "SchedulerService"
}
```

**Response (error):**
```json
{
  "error": "operation failed",
  "code": "INTERNAL_ERROR",
  "details": null
}
```

---

#### Internal Logic (step-by-step)

1. Poll every SCHEDULER_POLL_INTERVAL.
2. Claim up to SCHEDULER_CLAIM_BATCH_SIZE jobs using one atomic SQL statement that selects:
   - queued jobs due by database time (`next_run_at <= now()`), and
   - running jobs with expired leases (`lease_expires_at <= now()`) for recovery.
3. Fence ownership by rotating `run_id` on each successful claim.
3. Apply DWRR fairness by tenant weights and starvation window 10 seconds.
4. Dispatch claimed jobs to runner queue with lease metadata.
5. Do not claim steps blocked by unresolved approval gates (`waiting_approval`).
6. Periodic jobs do not require special claim logic; they re-enter the same due-queue path after runner requeue updates `next_run_at`.

---

#### Data Model (if this component owns a table)

This component uses shared Release Engine tables and does not introduce a new table in this phase.

---

#### Error Table

| Condition | HTTP Status | Error Code | Response Body |
|---|---|---|---|
| Claim query failure | 503 | `SCHED_CLAIM_FAILED` | `{"error":"claim query failed","code":"SCHED_CLAIM_FAILED","details":null}` |
| Dispatch queue saturated | 503 | `SCHED_DISPATCH_SATURATED` | `{"error":"dispatch queue saturated","code":"SCHED_DISPATCH_SATURATED","details":null}` |

---

#### Acceptance Criteria (Gherkin)

```gherkin
Feature: SchedulerService

  Scenario: Happy path
    Given valid input and healthy dependencies
    When the component executes
    Then it returns success and emits logs, metrics, and traces

  Scenario: Edge case
    Given replayed state or partial prior progress
    When the component executes again
    Then it behaves deterministically without duplicate side effects

  Scenario: Error
    Given dependency failure or stale run ownership
    When the component executes
    Then it returns a structured error with explicit code
```

---

#### Performance Targets

| Metric | Target |
|---|---|
| Claim latency p99 | <= TTL/4 |
| Claim throughput | 50-200 claims/s per instance |

---

#### Security Considerations

- Enforce tenant isolation on every SQL read and write.
- Redact or avoid secret values in logs, traces, and error payloads.
- Use fenced writes (`run_id`/`effect_id`) for ownership-sensitive state transitions.

---

#### Observability

- **Log events:** `scheduler.start`, `scheduler.success`, `scheduler.failure`
- **Metrics:** `release_engine_scheduler_total{status}`, `release_engine_scheduler_duration_seconds`
- **Trace span:** `scheduler.claim`

---

#### Cross-Component Interactions

| Direction | Component | Mechanism | Data Exchanged |
|---|---|---|---|
| This → DBPool | outbound call | pgx query/exec | SQL statements, parameters, transaction result |
| This → MetricsExporter | outbound call | in-process API | counters, histograms, labels |
| This → TracingService | outbound call | OpenTelemetry | span start/end, attributes, errors |
| Upstream runtime → This | inbound call | in-process call | context, tenant_id, job_id, run_id |

### SPEC: LeaseManager

**File:** `internal/scheduler/lease_manager.go`
**Package:** `scheduler`
**Phase:** 1
**Dependencies:** DBPool

---

#### Purpose

Manages job lease acquisition, expiry checks, and fenced update primitives using database time only.

---

#### Shared Context (duplicated for self-containment)

```go
// ErrorResponse is the standard API error envelope.
type ErrorResponse struct {
    Error   string `json:"error"`
    Code    string `json:"code"`
    Details any    `json:"details,omitempty"`
}

// JobCreateRequest is the POST /v1/jobs request schema.
type JobCreateRequest struct {
    TenantID       string         `json:"tenant_id"`
    PathKey        string         `json:"path_key"`
    Params         map[string]any `json:"params"`
    IdempotencyKey string         `json:"idempotency_key"`
    CallbackURL    *string        `json:"callback_url,omitempty"`
}

// JobStatusResponse is the GET /v1/jobs/{job_id} schema.
type JobStatusResponse struct {
    JobID            string     `json:"job_id"`
    TenantID         string     `json:"tenant_id"`
    State            string     `json:"state"`
    Attempt          int        `json:"attempt"`
    LeaseExpiresAt   *time.Time `json:"lease_expires_at,omitempty"`
    LastErrorCode    *string    `json:"last_error_code,omitempty"`
    LastErrorMessage *string    `json:"last_error_message,omitempty"`
}
```

Validation rules used by this component:
- `tenant_id` regex: `^[a-z0-9][a-z0-9-]{1,62}$`
- `idempotency_key` regex: `^[a-zA-Z0-9\-_.]{1,128}$`
- Maximum request body size: `262144` bytes
- Callback URL must use HTTPS and must not resolve to blocked private/link-local/metadata ranges.

---

#### Public Interface

```go
type LeaseManager interface {
    AcquireJobLease(ctx context.Context, jobID string, ownerID string, ttl time.Duration) (runID string, ok bool, err error)
    FinaliseWithFence(ctx context.Context, jobID string, runID string, state string) (rows int64, err error)
}
```

##### Example — LeaseManager

**Request:**
```json
{
  "tenant_id": "acme-prod",
  "job_id": "6c8bb1de-ef20-45cb-9e13-a57f44667d88",
  "run_id": "8a4cfa4d-f89c-4f0e-a809-f7d9a2c8f8ad"
}
```

**Response (200):**
```json
{
  "status": "ok",
  "component": "LeaseManager"
}
```

**Response (error):**
```json
{
  "error": "operation failed",
  "code": "INTERNAL_ERROR",
  "details": null
}
```

---

#### Internal Logic (step-by-step)

1. Acquire leases via SQL updates using `now()`.
2. Reject stale finalisation attempts with 0-row update signal.
3. Expose helpers for runner and scheduler fenced transitions.

---

#### Data Model (if this component owns a table)

This component uses shared Release Engine tables and does not introduce a new table in this phase.

---

#### Error Table

| Condition | HTTP Status | Error Code | Response Body |
|---|---|---|---|
| Lease acquisition failed | 409 | `LEASE_ACQUIRE_CONFLICT` | `{"error":"lease already held","code":"LEASE_ACQUIRE_CONFLICT","details":null}` |
| Lost lease on finalise | 409 | `ERR_FENCED_CONFLICT` | `{"error":"lease lost","code":"ERR_FENCED_CONFLICT","details":null}` |

---

#### Acceptance Criteria (Gherkin)

```gherkin
Feature: LeaseManager

  Scenario: Happy path
    Given valid input and healthy dependencies
    When the component executes
    Then it returns success and emits logs, metrics, and traces

  Scenario: Edge case
    Given replayed state or partial prior progress
    When the component executes again
    Then it behaves deterministically without duplicate side effects

  Scenario: Error
    Given dependency failure or stale run ownership
    When the component executes
    Then it returns a structured error with explicit code
```

---

#### Performance Targets

| Metric | Target |
|---|---|
| Lease acquire p95 | <15ms |
| Fenced finalise p95 | <20ms |

---

#### Security Considerations

- Enforce tenant isolation on every SQL read and write.
- Redact or avoid secret values in logs, traces, and error payloads.
- Use fenced writes (`run_id`/`effect_id`) for ownership-sensitive state transitions.

---

#### Observability

- **Log events:** `leasemanager.start`, `leasemanager.success`, `leasemanager.failure`
- **Metrics:** `release_engine_leasemanager_total{status}`, `release_engine_leasemanager_duration_seconds`
- **Trace span:** `lease.manager`

---

#### Cross-Component Interactions

| Direction | Component | Mechanism | Data Exchanged |
|---|---|---|---|
| This → DBPool | outbound call | pgx query/exec | SQL statements, parameters, transaction result |
| This → MetricsExporter | outbound call | in-process API | counters, histograms, labels |
| This → TracingService | outbound call | OpenTelemetry | span start/end, attributes, errors |
| Upstream runtime → This | inbound call | in-process call | context, tenant_id, job_id, run_id |

### SPEC: RunnerService

**File:** `internal/runner/service.go`
**Package:** `runner`
**Phase:** 2
**Dependencies:** DBPool, ModuleRegistry, ConnectorRegistry, VoltaManager

---

#### Purpose

Executes claimed jobs, drives step lifecycle, invokes connectors through fenced external effect records, and finalises jobs. On successful completion, unscheduled jobs are terminally completed while scheduled jobs are re-queued for their next cron occurrence.

---

#### Shared Context (duplicated for self-containment)

```go
// ErrorResponse is the standard API error envelope.
type ErrorResponse struct {
    Error   string `json:"error"`
    Code    string `json:"code"`
    Details any    `json:"details,omitempty"`
}

// JobCreateRequest is the POST /v1/jobs request schema.
type JobCreateRequest struct {
    TenantID       string         `json:"tenant_id"`
    PathKey        string         `json:"path_key"`
    Params         map[string]any `json:"params"`
    IdempotencyKey string         `json:"idempotency_key"`
    CallbackURL    *string        `json:"callback_url,omitempty"`
}

// JobStatusResponse is the GET /v1/jobs/{job_id} schema.
type JobStatusResponse struct {
    JobID            string     `json:"job_id"`
    TenantID         string     `json:"tenant_id"`
    State            string     `json:"state"`
    Attempt          int        `json:"attempt"`
    LeaseExpiresAt   *time.Time `json:"lease_expires_at,omitempty"`
    LastErrorCode    *string    `json:"last_error_code,omitempty"`
    LastErrorMessage *string    `json:"last_error_message,omitempty"`
}
```

Validation rules used by this component:
- `tenant_id` regex: `^[a-z0-9][a-z0-9-]{1,62}$`
- `idempotency_key` regex: `^[a-zA-Z0-9\-_.]{1,128}$`
- Maximum request body size: `262144` bytes
- Callback URL must use HTTPS and must not resolve to blocked private/link-local/metadata ranges.

---

#### Public Interface

```go
type Runner interface {
    RunJob(ctx context.Context, jobID string, runID string) error
}
```

##### Example — RunnerService

**Request:**
```json
{
  "tenant_id": "acme-prod",
  "job_id": "6c8bb1de-ef20-45cb-9e13-a57f44667d88",
  "run_id": "8a4cfa4d-f89c-4f0e-a809-f7d9a2c8f8ad"
}
```

**Response (200):**
```json
{
  "status": "ok",
  "component": "RunnerService"
}
```

**Response (error):**
```json
{
  "error": "operation failed",
  "code": "INTERNAL_ERROR",
  "details": null
}
```

---

#### Internal Logic (step-by-step)

1. Load `path_key`, `version`, `params_json`, and `schedule` for `(job_id, run_id)`.
2. Resolve module binding from path and version in `ModuleRegistry`.
3. Decode `params_json` to `map[string]any` and invoke `module.Execute(ctx, stepAPI, params)`.
4. Module execution drives step lifecycle through `StepAPI`, including approval gating via `WaitForApproval`.
5. Finalise with fenced writes (`WHERE id = $id AND run_id = $run_id AND state = 'running'`):
   - success + no schedule: set `state='succeeded'`, clear lease fields, set `finished_at=now()`.
   - success + schedule: compute next cron occurrence and set `state='queued'` with `next_run_at=<next occurrence>`.
   - execution failure: apply bounded backoff (`queued` or `jobs_exhausted`).
6. Synchronise `jobs_read` projection on every finalisation path and enqueue `job.succeeded` outbox event after successful module execution.
7. Treat 0-row finalisation updates as fenced conflicts (lost lease) and stop local processing.

---

#### Data Model (if this component owns a table)

This component uses shared Release Engine tables and does not introduce a new table in this phase.

---

#### Error Table

| Condition | HTTP Status | Error Code | Response Body |
|---|---|---|---|
| Module resolution failure | 500 | `RUNNER_MODULE_NOT_FOUND` | `{"error":"module not found","code":"RUNNER_MODULE_NOT_FOUND","details":null}` |
| Connector unknown outcome | 503 | `ERR_PROVIDER_TIMEOUT` | `{"error":"provider timeout","code":"ERR_PROVIDER_TIMEOUT","details":null}` |

---

#### Acceptance Criteria (Gherkin)

```gherkin
Feature: RunnerService

  Scenario: Happy path
    Given valid input and healthy dependencies
    When the component executes
    Then it returns success and emits logs, metrics, and traces

  Scenario: Edge case
    Given replayed state or partial prior progress
    When the component executes again
    Then it behaves deterministically without duplicate side effects

  Scenario: Error
    Given dependency failure or stale run ownership
    When the component executes
    Then it returns a structured error with explicit code
```

---

#### Performance Targets

| Metric | Target |
|---|---|
| Step execution overhead | <10ms excluding provider call |
| Finalise write p95 | <30ms |

---

#### Security Considerations

- Enforce tenant isolation on every SQL read and write.
- Redact or avoid secret values in logs, traces, and error payloads.
- Use fenced writes (`run_id`/`effect_id`) for ownership-sensitive state transitions.

---

#### Observability

- **Log events:** `runner.start`, `runner.success`, `runner.failure`
- **Metrics:** `release_engine_runner_total{status}`, `release_engine_runner_duration_seconds`
- **Trace span:** `runner.job`

---

#### Cross-Component Interactions

| Direction | Component | Mechanism | Data Exchanged |
|---|---|---|---|
| This → DBPool | outbound call | pgx query/exec | SQL statements, parameters, transaction result |
| This → MetricsExporter | outbound call | in-process API | counters, histograms, labels |
| This → TracingService | outbound call | OpenTelemetry | span start/end, attributes, errors |
| Upstream runtime → This | inbound call | in-process call | context, tenant_id, job_id, run_id |

### SPEC: StepAPIAdapter

**File:** `internal/runner/step_api.go`
**Package:** `runner`
**Phase:** 2
**Dependencies:** DBPool, RunnerService

---

#### Purpose

Provides module-facing StepAPI methods with durable step persistence, context store support, and effect reservation.

---

#### Shared Context (duplicated for self-containment)

```go
// ErrorResponse is the standard API error envelope.
type ErrorResponse struct {
    Error   string `json:"error"`
    Code    string `json:"code"`
    Details any    `json:"details,omitempty"`
}

// JobCreateRequest is the POST /v1/jobs request schema.
type JobCreateRequest struct {
    TenantID       string         `json:"tenant_id"`
    PathKey        string         `json:"path_key"`
    Params         map[string]any `json:"params"`
    IdempotencyKey string         `json:"idempotency_key"`
    CallbackURL    *string        `json:"callback_url,omitempty"`
}

// JobStatusResponse is the GET /v1/jobs/{job_id} schema.
type JobStatusResponse struct {
    JobID            string     `json:"job_id"`
    TenantID         string     `json:"tenant_id"`
    State            string     `json:"state"`
    Attempt          int        `json:"attempt"`
    LeaseExpiresAt   *time.Time `json:"lease_expires_at,omitempty"`
    LastErrorCode    *string    `json:"last_error_code,omitempty"`
    LastErrorMessage *string    `json:"last_error_message,omitempty"`
}
```

Validation rules used by this component:
- `tenant_id` regex: `^[a-z0-9][a-z0-9-]{1,62}$`
- `idempotency_key` regex: `^[a-zA-Z0-9\-_.]{1,128}$`
- Maximum request body size: `262144` bytes
- Callback URL must use HTTPS and must not resolve to blocked private/link-local/metadata ranges.

---

#### Public Interface

```go
type StepAPI interface {
    BeginStep(stepKey string) error
    EndStepOK(stepKey string, output map[string]any) error
    EndStepErr(stepKey, code, msg string) error
    CallConnector(ctx context.Context, req ConnectorRequest) (*ConnectorResult, error)
    WaitForApproval(ctx context.Context, req ApprovalRequest) (ApprovalOutcome, error)
    SetContext(key string, value any) error
    GetContext(key string) (any, bool)
    IsCancelled() bool
}
```

##### Example — StepAPIAdapter

**Request:**
```json
{
  "tenant_id": "acme-prod",
  "job_id": "6c8bb1de-ef20-45cb-9e13-a57f44667d88",
  "run_id": "8a4cfa4d-f89c-4f0e-a809-f7d9a2c8f8ad"
}
```

**Response (200):**
```json
{
  "status": "ok",
  "component": "StepAPIAdapter"
}
```

**Response (error):**
```json
{
  "error": "operation failed",
  "code": "INTERNAL_ERROR",
  "details": null
}
```

---

#### Internal Logic (step-by-step)

1. BeginStep tracks the active step key for subsequent API calls.
2. EndStepOK/EndStepErr persist terminal step state in `steps` via upsert semantics.
3. WaitForApproval persists `approval_request`, computes and stores `approval_ttl` and `approval_expires_at`, transitions step to `waiting_approval`, and polls `approval_decisions` until a decision is available.
4. SetContext/GetContext provide in-memory context storage for module execution scope.

---

#### Data Model (if this component owns a table)

This component uses shared Release Engine tables and does not introduce a new table in this phase.

---

#### Error Table

| Condition | HTTP Status | Error Code | Response Body |
|---|---|---|---|
| Step persistence failure | 503 | `STEP_PERSIST_FAILED` | `{"error":"step persistence failed","code":"STEP_PERSIST_FAILED","details":null}` |
| Context write fenced | 409 | `ERR_FENCED_CONFLICT` | `{"error":"stale run_id context write","code":"ERR_FENCED_CONFLICT","details":null}` |

---

#### Acceptance Criteria (Gherkin)

```gherkin
Feature: StepAPIAdapter

  Scenario: Happy path
    Given valid input and healthy dependencies
    When the component executes
    Then it returns success and emits logs, metrics, and traces

  Scenario: Edge case
    Given replayed state or partial prior progress
    When the component executes again
    Then it behaves deterministically without duplicate side effects

  Scenario: Error
    Given dependency failure or stale run ownership
    When the component executes
    Then it returns a structured error with explicit code
```

---

#### Performance Targets

| Metric | Target |
|---|---|
| BeginStep p95 | <15ms |
| SetContext/GetContext p95 | <10ms |

---

#### Security Considerations

- Enforce tenant isolation on every SQL read and write.
- Redact or avoid secret values in logs, traces, and error payloads.
- Use fenced writes (`run_id`/`effect_id`) for ownership-sensitive state transitions.

---

#### Observability

- **Log events:** `stepapiadapter.start`, `stepapiadapter.success`, `stepapiadapter.failure`
- **Metrics:** `release_engine_stepapiadapter_total{status}`, `release_engine_stepapiadapter_duration_seconds`
- **Trace span:** `runner.step_api`

---

#### Cross-Component Interactions

| Direction | Component | Mechanism | Data Exchanged |
|---|---|---|---|
| This → DBPool | outbound call | pgx query/exec | SQL statements, parameters, transaction result |
| This → MetricsExporter | outbound call | in-process API | counters, histograms, labels |
| This → TracingService | outbound call | OpenTelemetry | span start/end, attributes, errors |
| Upstream runtime → This | inbound call | in-process call | context, tenant_id, job_id, run_id |

### SPEC: ApprovalWorker

**File:** `internal/transport/http/approval_worker.go`
**Package:** `http`
**Phase:** 2
**Dependencies:** PolicyEngine, JobsAPIHandler, OutboxDispatcher

---

#### Purpose

Applies approval timeout lifecycle rules for `waiting_approval` steps, including escalation and automatic expiry.

---

#### Public Interface

```go
type ApprovalWorker struct {
    // Tick applies escalation and expiry decisions once.
    Tick(ctx context.Context)
    // Run executes Tick on a fixed poll interval.
    Run(ctx context.Context)
}
```

---

#### Internal Logic (step-by-step)

1. Poll every 30 seconds by default.
2. Identify approval steps in `waiting_approval` with computed `approval_expires_at`.
3. Emit `approval_escalated` once when elapsed time reaches `escalation_at * ttl`.
4. Transition expired steps to `error` with `approval_timeout` reason.
5. Record system decision (`expired`, approver=`system`) and emit `approval_expired`.
6. Emit outbox contracts through registered event types only (`approval_escalated`, `approval_expired`).

---

#### Data Model (if this component owns a table)

This component uses shared Release Engine tables and does not introduce a new table in this phase.

---

#### Error Table

| Condition | HTTP Status | Error Code | Response Body |
|---|---|---|---|
| Approval timeout transition failed | 503 | `APPROVAL_TIMEOUT_WRITE_FAILED` | `{"error":"failed to persist approval timeout","code":"APPROVAL_TIMEOUT_WRITE_FAILED","details":null}` |
| Escalation event emission failed | 503 | `APPROVAL_ESCALATION_EMIT_FAILED` | `{"error":"failed to emit escalation","code":"APPROVAL_ESCALATION_EMIT_FAILED","details":null}` |

---

#### Acceptance Criteria (Gherkin)

```gherkin
Feature: ApprovalWorker

  Scenario: Happy path
    Given waiting approvals below expiry with elapsed time crossing escalation threshold
    When the worker ticks
    Then it emits a single escalation event and leaves step state unchanged

  Scenario: Edge case
    Given an already escalated waiting approval
    When the worker ticks again before expiry
    Then it does not emit duplicate escalation events

  Scenario: Error
    Given a waiting approval past expiry
    When the worker ticks
    Then it records an expired system decision and transitions the step to error
```

---

#### Performance Targets

| Metric | Target |
|---|---|
| Tick execution p95 | <50ms for 1,000 waiting approvals |
| Escalation/expiry detection lag | <= poll interval + 5s |

---

#### Security Considerations

- Enforce tenant isolation on all queried steps.
- Use database time for TTL comparisons to avoid host clock skew.
- Ensure system-generated expiry decisions are immutable and auditable.

---

#### Observability

- **Log events:** `approvalworker.tick`, `approvalworker.escalated`, `approvalworker.expired`
- **Metrics:** `release_engine_approval_escalations_total`, `release_engine_approval_timeouts_total`
- **Trace span:** `approval.worker.tick`

---

#### Cross-Component Interactions

| Direction | Component | Mechanism | Data Exchanged |
|---|---|---|---|
| This → Steps/ApprovalDecisions storage | outbound call | SQL read/write | waiting approval rows, expiry decisions |
| This → OutboxDispatcher | outbound call | outbox enqueue | `approval_escalated`, `approval_expired` events |
| This → MetricsExporter | outbound call | in-process API | escalation and timeout counters |

### SPEC: ModuleRegistry

**File:** `internal/registry/module_registry.go`
**Package:** `registry`
**Phase:** 2
**Dependencies:** ConfigLoader

---

#### Purpose

Stores compiled-in module registrations and resolves module versions for execution.

---

#### Shared Context (duplicated for self-containment)

```go
// ErrorResponse is the standard API error envelope.
type ErrorResponse struct {
    Error   string `json:"error"`
    Code    string `json:"code"`
    Details any    `json:"details,omitempty"`
}

// JobCreateRequest is the POST /v1/jobs request schema.
type JobCreateRequest struct {
    TenantID       string         `json:"tenant_id"`
    PathKey        string         `json:"path_key"`
    Params         map[string]any `json:"params"`
    IdempotencyKey string         `json:"idempotency_key"`
    CallbackURL    *string        `json:"callback_url,omitempty"`
}

// JobStatusResponse is the GET /v1/jobs/{job_id} schema.
type JobStatusResponse struct {
    JobID            string     `json:"job_id"`
    TenantID         string     `json:"tenant_id"`
    State            string     `json:"state"`
    Attempt          int        `json:"attempt"`
    LeaseExpiresAt   *time.Time `json:"lease_expires_at,omitempty"`
    LastErrorCode    *string    `json:"last_error_code,omitempty"`
    LastErrorMessage *string    `json:"last_error_message,omitempty"`
}
```

Validation rules used by this component:
- `tenant_id` regex: `^[a-z0-9][a-z0-9-]{1,62}$`
- `idempotency_key` regex: `^[a-zA-Z0-9\-_.]{1,128}$`
- Maximum request body size: `262144` bytes
- Callback URL must use HTTPS and must not resolve to blocked private/link-local/metadata ranges.

---

#### Public Interface

```go
type ModuleRegistry interface {
    Register(module Module) error
    Lookup(moduleKey string, version string) (Module, bool)
}
```

##### Example — ModuleRegistry

**Request:**
```json
{
  "tenant_id": "acme-prod",
  "job_id": "6c8bb1de-ef20-45cb-9e13-a57f44667d88",
  "run_id": "8a4cfa4d-f89c-4f0e-a809-f7d9a2c8f8ad"
}
```

**Response (200):**
```json
{
  "status": "ok",
  "component": "ModuleRegistry"
}
```

**Response (error):**
```json
{
  "error": "operation failed",
  "code": "INTERNAL_ERROR",
  "details": null
}
```

---

#### Internal Logic (step-by-step)

1. Register modules at startup with key:version uniqueness.
2. Resolve module lookup requests from runner.
3. Return deterministic not-found results for missing bindings.

---

#### Data Model (if this component owns a table)

This component uses shared Release Engine tables and does not introduce a new table in this phase.

---

#### Error Table

| Condition | HTTP Status | Error Code | Response Body |
|---|---|---|---|
| Duplicate registration | 500 | `MODULE_DUPLICATE` | `{"error":"duplicate module registration","code":"MODULE_DUPLICATE","details":null}` |
| Lookup miss | 404 | `MODULE_NOT_FOUND` | `{"error":"module not found","code":"MODULE_NOT_FOUND","details":null}` |

---

#### Acceptance Criteria (Gherkin)

```gherkin
Feature: ModuleRegistry

  Scenario: Happy path
    Given valid input and healthy dependencies
    When the component executes
    Then it returns success and emits logs, metrics, and traces

  Scenario: Edge case
    Given replayed state or partial prior progress
    When the component executes again
    Then it behaves deterministically without duplicate side effects

  Scenario: Error
    Given dependency failure or stale run ownership
    When the component executes
    Then it returns a structured error with explicit code
```

---

#### Performance Targets

| Metric | Target |
|---|---|
| Lookup p95 | <1ms |
| Startup registration | <100ms |

---

#### Security Considerations

- Enforce tenant isolation on every SQL read and write.
- Redact or avoid secret values in logs, traces, and error payloads.
- Use fenced writes (`run_id`/`effect_id`) for ownership-sensitive state transitions.

---

#### Observability

- **Log events:** `moduleregistry.start`, `moduleregistry.success`, `moduleregistry.failure`
- **Metrics:** `release_engine_moduleregistry_total{status}`, `release_engine_moduleregistry_duration_seconds`
- **Trace span:** `registry.module`

---

#### Cross-Component Interactions

| Direction | Component | Mechanism | Data Exchanged |
|---|---|---|---|
| This → DBPool | outbound call | pgx query/exec | SQL statements, parameters, transaction result |
| This → MetricsExporter | outbound call | in-process API | counters, histograms, labels |
| This → TracingService | outbound call | OpenTelemetry | span start/end, attributes, errors |
| Upstream runtime → This | inbound call | in-process call | context, tenant_id, job_id, run_id |

### SPEC: ConnectorRegistry

**File:** `internal/registry/connector_registry.go`
**Package:** `registry`
**Phase:** 2
**Dependencies:** ConfigLoader

---

#### Purpose

Registers compiled-in connectors and resolves connector keys for effect execution and reconciliation.

---

#### Shared Context (duplicated for self-containment)

```go
// ErrorResponse is the standard API error envelope.
type ErrorResponse struct {
    Error   string `json:"error"`
    Code    string `json:"code"`
    Details any    `json:"details,omitempty"`
}

// JobCreateRequest is the POST /v1/jobs request schema.
type JobCreateRequest struct {
    TenantID       string         `json:"tenant_id"`
    PathKey        string         `json:"path_key"`
    Params         map[string]any `json:"params"`
    IdempotencyKey string         `json:"idempotency_key"`
    CallbackURL    *string        `json:"callback_url,omitempty"`
}

// JobStatusResponse is the GET /v1/jobs/{job_id} schema.
type JobStatusResponse struct {
    JobID            string     `json:"job_id"`
    TenantID         string     `json:"tenant_id"`
    State            string     `json:"state"`
    Attempt          int        `json:"attempt"`
    LeaseExpiresAt   *time.Time `json:"lease_expires_at,omitempty"`
    LastErrorCode    *string    `json:"last_error_code,omitempty"`
    LastErrorMessage *string    `json:"last_error_message,omitempty"`
}
```

Validation rules used by this component:
- `tenant_id` regex: `^[a-z0-9][a-z0-9-]{1,62}$`
- `idempotency_key` regex: `^[a-zA-Z0-9\-_.]{1,128}$`
- Maximum request body size: `262144` bytes
- Callback URL must use HTTPS and must not resolve to blocked private/link-local/metadata ranges.

---

#### Public Interface

```go
type ConnectorRegistry interface {
    Register(conn Connector)
    Lookup(key string) (Connector, bool)
}
```

##### Example — ConnectorRegistry

**Request:**
```json
{
  "tenant_id": "acme-prod",
  "job_id": "6c8bb1de-ef20-45cb-9e13-a57f44667d88",
  "run_id": "8a4cfa4d-f89c-4f0e-a809-f7d9a2c8f8ad"
}
```

**Response (200):**
```json
{
  "status": "ok",
  "component": "ConnectorRegistry"
}
```

**Response (error):**
```json
{
  "error": "operation failed",
  "code": "INTERNAL_ERROR",
  "details": null
}
```

---

#### Internal Logic (step-by-step)

1. Register connector implementations at startup.
2. Lookup connectors by connector_key and optional policy constraints.
3. Expose deterministic missing-connector errors.

---

#### Data Model (if this component owns a table)

This component uses shared Release Engine tables and does not introduce a new table in this phase.

---

#### Error Table

| Condition | HTTP Status | Error Code | Response Body |
|---|---|---|---|
| Duplicate connector registration | 500 | `CONNECTOR_DUPLICATE` | `{"error":"duplicate connector registration","code":"CONNECTOR_DUPLICATE","details":null}` |
| Connector not found | 404 | `CONNECTOR_NOT_FOUND` | `{"error":"connector not found","code":"CONNECTOR_NOT_FOUND","details":null}` |

---

#### Acceptance Criteria (Gherkin)

```gherkin
Feature: ConnectorRegistry

  Scenario: Happy path
    Given valid input and healthy dependencies
    When the component executes
    Then it returns success and emits logs, metrics, and traces

  Scenario: Edge case
    Given replayed state or partial prior progress
    When the component executes again
    Then it behaves deterministically without duplicate side effects

  Scenario: Error
    Given dependency failure or stale run ownership
    When the component executes
    Then it returns a structured error with explicit code
```

---

#### Performance Targets

| Metric | Target |
|---|---|
| Lookup p95 | <1ms |
| Startup registration | <100ms |

---

#### Security Considerations

- Enforce tenant isolation on every SQL read and write.
- Redact or avoid secret values in logs, traces, and error payloads.
- Use fenced writes (`run_id`/`effect_id`) for ownership-sensitive state transitions.

---

#### Observability

- **Log events:** `connectorregistry.start`, `connectorregistry.success`, `connectorregistry.failure`
- **Metrics:** `release_engine_connectorregistry_total{status}`, `release_engine_connectorregistry_duration_seconds`
- **Trace span:** `registry.connector`

---

#### Cross-Component Interactions

| Direction | Component | Mechanism | Data Exchanged |
|---|---|---|---|
| This → DBPool | outbound call | pgx query/exec | SQL statements, parameters, transaction result |
| This → MetricsExporter | outbound call | in-process API | counters, histograms, labels |
| This → TracingService | outbound call | OpenTelemetry | span start/end, attributes, errors |
| Upstream runtime → This | inbound call | in-process call | context, tenant_id, job_id, run_id |

### SPEC: VoltaManager

**File:** `internal/secrets/volta_manager.go`
**Package:** `secrets`
**Phase:** 2
**Dependencies:** ConfigLoader, LoggerFactory

---

#### Purpose

Bootstraps Volta with master passphrase from AWS Secrets Manager and manages tenant vault sessions backed by S3.

---

#### Shared Context (duplicated for self-containment)

```go
// ErrorResponse is the standard API error envelope.
type ErrorResponse struct {
    Error   string `json:"error"`
    Code    string `json:"code"`
    Details any    `json:"details,omitempty"`
}

// JobCreateRequest is the POST /v1/jobs request schema.
type JobCreateRequest struct {
    TenantID       string         `json:"tenant_id"`
    PathKey        string         `json:"path_key"`
    Params         map[string]any `json:"params"`
    IdempotencyKey string         `json:"idempotency_key"`
    CallbackURL    *string        `json:"callback_url,omitempty"`
}

// JobStatusResponse is the GET /v1/jobs/{job_id} schema.
type JobStatusResponse struct {
    JobID            string     `json:"job_id"`
    TenantID         string     `json:"tenant_id"`
    State            string     `json:"state"`
    Attempt          int        `json:"attempt"`
    LeaseExpiresAt   *time.Time `json:"lease_expires_at,omitempty"`
    LastErrorCode    *string    `json:"last_error_code,omitempty"`
    LastErrorMessage *string    `json:"last_error_message,omitempty"`
}
```

Validation rules used by this component:
- `tenant_id` regex: `^[a-z0-9][a-z0-9-]{1,62}$`
- `idempotency_key` regex: `^[a-zA-Z0-9\-_.]{1,128}$`
- Maximum request body size: `262144` bytes
- Callback URL must use HTTPS and must not resolve to blocked private/link-local/metadata ranges.

---

#### Public Interface

```go
type Manager interface {
    Init(ctx context.Context) error
    GetVault(ctx context.Context, tenantID string) (VaultService, error)
    CloseAll(ctx context.Context) error
}
```

##### Example — VoltaManager

**Request:**
```json
{
  "tenant_id": "acme-prod",
  "job_id": "6c8bb1de-ef20-45cb-9e13-a57f44667d88",
  "run_id": "8a4cfa4d-f89c-4f0e-a809-f7d9a2c8f8ad"
}
```

**Response (200):**
```json
{
  "status": "ok",
  "component": "VoltaManager"
}
```

**Response (error):**
```json
{
  "error": "operation failed",
  "code": "INTERNAL_ERROR",
  "details": null
}
```

---

#### Internal Logic (step-by-step)

1. Fetch master passphrase once on startup from Secrets Manager via IAM role.
2. Initialise Volta manager with memguard-protected key material.
3. Load tenant vault data from S3 on demand and cache for VOLTA_SESSION_TTL.
4. Expose UseSecret scoped decryption for runner.

---

#### Data Model (if this component owns a table)

This component uses shared Release Engine tables and does not introduce a new table in this phase.

---

#### Error Table

| Condition | HTTP Status | Error Code | Response Body |
|---|---|---|---|
| Secrets Manager bootstrap failure | 503 | `VOLTA_BOOTSTRAP_FAILED` | `{"error":"volta bootstrap failed","code":"VOLTA_BOOTSTRAP_FAILED","details":null}` |
| Vault retrieval failure | 503 | `VOLTA_VAULT_UNAVAILABLE` | `{"error":"tenant vault unavailable","code":"VOLTA_VAULT_UNAVAILABLE","details":null}` |

---

#### Acceptance Criteria (Gherkin)

```gherkin
Feature: VoltaManager

  Scenario: Happy path
    Given valid input and healthy dependencies
    When the component executes
    Then it returns success and emits logs, metrics, and traces

  Scenario: Edge case
    Given replayed state or partial prior progress
    When the component executes again
    Then it behaves deterministically without duplicate side effects

  Scenario: Error
    Given dependency failure or stale run ownership
    When the component executes
    Then it returns a structured error with explicit code
```

---

#### Performance Targets

| Metric | Target |
|---|---|
| Bootstrap passphrase fetch | <500ms |
| Cached UseSecret overhead | <5ms excluding provider call |

---

#### Security Considerations

- Enforce tenant isolation on every SQL read and write.
- Redact or avoid secret values in logs, traces, and error payloads.
- Use fenced writes (`run_id`/`effect_id`) for ownership-sensitive state transitions.

---

#### Observability

- **Log events:** `voltamanager.start`, `voltamanager.success`, `voltamanager.failure`
- **Metrics:** `release_engine_voltamanager_total{status}`, `release_engine_voltamanager_duration_seconds`
- **Trace span:** `secrets.volta`

---

#### Cross-Component Interactions

| Direction | Component | Mechanism | Data Exchanged |
|---|---|---|---|
| This → DBPool | outbound call | pgx query/exec | SQL statements, parameters, transaction result |
| This → MetricsExporter | outbound call | in-process API | counters, histograms, labels |
| This → TracingService | outbound call | OpenTelemetry | span start/end, attributes, errors |
| Upstream runtime → This | inbound call | in-process call | context, tenant_id, job_id, run_id |

### SPEC: ReconcilerService

**File:** `internal/reconciler/service.go`
**Package:** `reconciler`
**Phase:** 2
**Dependencies:** DBPool, ConnectorRegistry

---

#### Purpose

Resolves `unknown_outcome` effects by querying provider state and finalising to succeeded, failed, retry, or DLQ.

---

#### Shared Context (duplicated for self-containment)

```go
// ErrorResponse is the standard API error envelope.
type ErrorResponse struct {
    Error   string `json:"error"`
    Code    string `json:"code"`
    Details any    `json:"details,omitempty"`
}

// JobCreateRequest is the POST /v1/jobs request schema.
type JobCreateRequest struct {
    TenantID       string         `json:"tenant_id"`
    PathKey        string         `json:"path_key"`
    Params         map[string]any `json:"params"`
    IdempotencyKey string         `json:"idempotency_key"`
    CallbackURL    *string        `json:"callback_url,omitempty"`
}

// JobStatusResponse is the GET /v1/jobs/{job_id} schema.
type JobStatusResponse struct {
    JobID            string     `json:"job_id"`
    TenantID         string     `json:"tenant_id"`
    State            string     `json:"state"`
    Attempt          int        `json:"attempt"`
    LeaseExpiresAt   *time.Time `json:"lease_expires_at,omitempty"`
    LastErrorCode    *string    `json:"last_error_code,omitempty"`
    LastErrorMessage *string    `json:"last_error_message,omitempty"`
}
```

Validation rules used by this component:
- `tenant_id` regex: `^[a-z0-9][a-z0-9-]{1,62}$`
- `idempotency_key` regex: `^[a-zA-Z0-9\-_.]{1,128}$`
- Maximum request body size: `262144` bytes
- Callback URL must use HTTPS and must not resolve to blocked private/link-local/metadata ranges.

---

#### Public Interface

```go
type Reconciler interface {
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
}
```

##### Example — ReconcilerService

**Request:**
```json
{
  "tenant_id": "acme-prod",
  "job_id": "6c8bb1de-ef20-45cb-9e13-a57f44667d88",
  "run_id": "8a4cfa4d-f89c-4f0e-a809-f7d9a2c8f8ad"
}
```

**Response (200):**
```json
{
  "status": "ok",
  "component": "ReconcilerService"
}
```

**Response (error):**
```json
{
  "error": "operation failed",
  "code": "INTERNAL_ERROR",
  "details": null
}
```

---

#### Internal Logic (step-by-step)

1. Scan `external_effects` every 30 seconds where status is unknown_outcome and next_run_at <= now().
2. Probe provider by call_id if supported; otherwise inspect resource state.
3. Finalise effect status or schedule retry with next_run_at plus 5 minutes.
4. Escalate to dlq after max reconcile attempts (default 5).

---

#### Data Model (if this component owns a table)

This component uses shared Release Engine tables and does not introduce a new table in this phase.

---

#### Error Table

| Condition | HTTP Status | Error Code | Response Body |
|---|---|---|---|
| Provider probe failed | 503 | `RECONCILE_PROBE_FAILED` | `{"error":"reconciliation probe failed","code":"RECONCILE_PROBE_FAILED","details":null}` |
| Max attempts reached | 503 | `RECONCILE_DLQ` | `{"error":"effect escalated to dlq","code":"RECONCILE_DLQ","details":null}` |

---

#### Acceptance Criteria (Gherkin)

```gherkin
Feature: ReconcilerService

  Scenario: Happy path
    Given valid input and healthy dependencies
    When the component executes
    Then it returns success and emits logs, metrics, and traces

  Scenario: Edge case
    Given replayed state or partial prior progress
    When the component executes again
    Then it behaves deterministically without duplicate side effects

  Scenario: Error
    Given dependency failure or stale run ownership
    When the component executes
    Then it returns a structured error with explicit code
```

---

#### Performance Targets

| Metric | Target |
|---|---|
| Reconciliation P99 | <=5m |
| Scan query p95 | <50ms |

---

#### Security Considerations

- Enforce tenant isolation on every SQL read and write.
- Redact or avoid secret values in logs, traces, and error payloads.
- Use fenced writes (`run_id`/`effect_id`) for ownership-sensitive state transitions.

---

#### Observability

- **Log events:** `reconciler.start`, `reconciler.success`, `reconciler.failure`
- **Metrics:** `release_engine_reconciler_total{status}`, `release_engine_reconciler_duration_seconds`
- **Trace span:** `reconciler.scan`

---

#### Cross-Component Interactions

| Direction | Component | Mechanism | Data Exchanged |
|---|---|---|---|
| This → DBPool | outbound call | pgx query/exec | SQL statements, parameters, transaction result |
| This → MetricsExporter | outbound call | in-process API | counters, histograms, labels |
| This → TracingService | outbound call | OpenTelemetry | span start/end, attributes, errors |
| Upstream runtime → This | inbound call | in-process call | context, tenant_id, job_id, run_id |

### SPEC: OutboxDispatcher

**File:** `internal/outbox/dispatcher.go`
**Package:** `outbox`
**Phase:** 2
**Dependencies:** DBPool, CallbackSigner

---

#### Purpose

Delivers callback webhooks from outbox with signed payloads, retries, and DLQ escalation.

---

#### Shared Context (duplicated for self-containment)

```go
// ErrorResponse is the standard API error envelope.
type ErrorResponse struct {
    Error   string `json:"error"`
    Code    string `json:"code"`
    Details any    `json:"details,omitempty"`
}

// JobCreateRequest is the POST /v1/jobs request schema.
type JobCreateRequest struct {
    TenantID       string         `json:"tenant_id"`
    PathKey        string         `json:"path_key"`
    Params         map[string]any `json:"params"`
    IdempotencyKey string         `json:"idempotency_key"`
    CallbackURL    *string        `json:"callback_url,omitempty"`
}

// JobStatusResponse is the GET /v1/jobs/{job_id} schema.
type JobStatusResponse struct {
    JobID            string     `json:"job_id"`
    TenantID         string     `json:"tenant_id"`
    State            string     `json:"state"`
    Attempt          int        `json:"attempt"`
    LeaseExpiresAt   *time.Time `json:"lease_expires_at,omitempty"`
    LastErrorCode    *string    `json:"last_error_code,omitempty"`
    LastErrorMessage *string    `json:"last_error_message,omitempty"`
}
```

Validation rules used by this component:
- `tenant_id` regex: `^[a-z0-9][a-z0-9-]{1,62}$`
- `idempotency_key` regex: `^[a-zA-Z0-9\-_.]{1,128}$`
- Maximum request body size: `262144` bytes
- Callback URL must use HTTPS and must not resolve to blocked private/link-local/metadata ranges.

---

#### Public Interface

```go
type Dispatcher interface {
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
}
```

##### Example — OutboxDispatcher

**Request:**
```json
{
  "tenant_id": "acme-prod",
  "job_id": "6c8bb1de-ef20-45cb-9e13-a57f44667d88",
  "run_id": "8a4cfa4d-f89c-4f0e-a809-f7d9a2c8f8ad"
}
```

**Response (200):**
```json
{
  "status": "ok",
  "component": "OutboxDispatcher"
}
```

**Response (error):**
```json
{
  "error": "operation failed",
  "code": "INTERNAL_ERROR",
  "details": null
}
```

---

#### Internal Logic (step-by-step)

1. Claim pending outbox rows with SKIP LOCKED.
2. Validate event type against dispatcher registry (`approval_requested`, `approval_decided`, `approval_escalated`, `approval_expired`, and other configured types).
2. Sign payload with active HMAC key and attach signature headers.
3. POST to callback_url with 10-second timeout.
4. Mark delivered on 2xx, retry on 5xx/network error, and move to dlq when attempts exhausted.

#### Approval Event Registration Contract

- Dispatcher MUST register these approval lifecycle types at startup:
  - `approval_requested`
  - `approval_decided`
  - `approval_escalated`
  - `approval_expired`
- Enqueue attempts for unknown event types MUST fail fast.
- Approval event payloads are immutable once queued.

---

#### Data Model (if this component owns a table)

This component uses shared Release Engine tables and does not introduce a new table in this phase.

---

#### Error Table

| Condition | HTTP Status | Error Code | Response Body |
|---|---|---|---|
| Callback timeout | 503 | `OUTBOX_DELIVERY_TIMEOUT` | `{"error":"callback timeout","code":"OUTBOX_DELIVERY_TIMEOUT","details":null}` |
| Max attempts exhausted | 503 | `OUTBOX_DLQ` | `{"error":"outbox entry moved to dlq","code":"OUTBOX_DLQ","details":null}` |

---

#### Acceptance Criteria (Gherkin)

```gherkin
Feature: OutboxDispatcher

  Scenario: Happy path
    Given valid input and healthy dependencies
    When the component executes
    Then it returns success and emits logs, metrics, and traces

  Scenario: Edge case
    Given replayed state or partial prior progress
    When the component executes again
    Then it behaves deterministically without duplicate side effects

  Scenario: Error
    Given dependency failure or stale run ownership
    When the component executes
    Then it returns a structured error with explicit code
```

---

#### Performance Targets

| Metric | Target |
|---|---|
| Delivery attempt overhead | <20ms excluding network |
| Throughput | 1,000-5,000 deliveries/minute per worker pool |

---

#### Security Considerations

- Enforce tenant isolation on every SQL read and write.
- Redact or avoid secret values in logs, traces, and error payloads.
- Use fenced writes (`run_id`/`effect_id`) for ownership-sensitive state transitions.

---

#### Observability

- **Log events:** `outboxdispatcher.start`, `outboxdispatcher.success`, `outboxdispatcher.failure`
- **Metrics:** `release_engine_outboxdispatcher_total{status}`, `release_engine_outboxdispatcher_duration_seconds`
- **Trace span:** `outbox.dispatch`

---

#### Cross-Component Interactions

| Direction | Component | Mechanism | Data Exchanged |
|---|---|---|---|
| This → DBPool | outbound call | pgx query/exec | SQL statements, parameters, transaction result |
| This → MetricsExporter | outbound call | in-process API | counters, histograms, labels |
| This → TracingService | outbound call | OpenTelemetry | span start/end, attributes, errors |
| Upstream runtime → This | inbound call | in-process call | context, tenant_id, job_id, run_id |

### SPEC: CallbackSigner

**File:** `internal/outbox/signer.go`
**Package:** `outbox`
**Phase:** 2
**Dependencies:** ConfigLoader

---

#### Purpose

Signs callback payloads with HMAC-SHA256 and supports dual-key verification during key rotation.

---

#### Shared Context (duplicated for self-containment)

```go
// ErrorResponse is the standard API error envelope.
type ErrorResponse struct {
    Error   string `json:"error"`
    Code    string `json:"code"`
    Details any    `json:"details,omitempty"`
}

// JobCreateRequest is the POST /v1/jobs request schema.
type JobCreateRequest struct {
    TenantID       string         `json:"tenant_id"`
    PathKey        string         `json:"path_key"`
    Params         map[string]any `json:"params"`
    IdempotencyKey string         `json:"idempotency_key"`
    CallbackURL    *string        `json:"callback_url,omitempty"`
}

// JobStatusResponse is the GET /v1/jobs/{job_id} schema.
type JobStatusResponse struct {
    JobID            string     `json:"job_id"`
    TenantID         string     `json:"tenant_id"`
    State            string     `json:"state"`
    Attempt          int        `json:"attempt"`
    LeaseExpiresAt   *time.Time `json:"lease_expires_at,omitempty"`
    LastErrorCode    *string    `json:"last_error_code,omitempty"`
    LastErrorMessage *string    `json:"last_error_message,omitempty"`
}
```

Validation rules used by this component:
- `tenant_id` regex: `^[a-z0-9][a-z0-9-]{1,62}$`
- `idempotency_key` regex: `^[a-zA-Z0-9\-_.]{1,128}$`
- Maximum request body size: `262144` bytes
- Callback URL must use HTTPS and must not resolve to blocked private/link-local/metadata ranges.

---

#### Public Interface

```go
type Signer interface {
    Sign(payload []byte) (signature string, keyID string, err error)
    Verify(payload []byte, signature string, keyID string) bool
}
```

##### Example — CallbackSigner

**Request:**
```json
{
  "tenant_id": "acme-prod",
  "job_id": "6c8bb1de-ef20-45cb-9e13-a57f44667d88",
  "run_id": "8a4cfa4d-f89c-4f0e-a809-f7d9a2c8f8ad"
}
```

**Response (200):**
```json
{
  "status": "ok",
  "component": "CallbackSigner"
}
```

**Response (error):**
```json
{
  "error": "operation failed",
  "code": "INTERNAL_ERROR",
  "details": null
}
```

---

#### Internal Logic (step-by-step)

1. Load active and optional secondary keys from configuration.
2. Generate HMAC-SHA256 signature over raw payload bytes.
3. Return `X-Signature-Version` key identifier for receiver verification.
4. Support verification against active and secondary keys during rotation window.

---

#### Data Model (if this component owns a table)

This component uses shared Release Engine tables and does not introduce a new table in this phase.

---

#### Error Table

| Condition | HTTP Status | Error Code | Response Body |
|---|---|---|---|
| Active key missing | 500 | `SIGNING_KEY_MISSING` | `{"error":"signing key missing","code":"SIGNING_KEY_MISSING","details":null}` |
| Invalid signature format | 400 | `SIGNATURE_INVALID` | `{"error":"invalid signature","code":"SIGNATURE_INVALID","details":null}` |

---

#### Acceptance Criteria (Gherkin)

```gherkin
Feature: CallbackSigner

  Scenario: Happy path
    Given valid input and healthy dependencies
    When the component executes
    Then it returns success and emits logs, metrics, and traces

  Scenario: Edge case
    Given replayed state or partial prior progress
    When the component executes again
    Then it behaves deterministically without duplicate side effects

  Scenario: Error
    Given dependency failure or stale run ownership
    When the component executes
    Then it returns a structured error with explicit code
```

---

#### Performance Targets

| Metric | Target |
|---|---|
| Sign operation p95 | <1ms |
| Verify operation p95 | <1ms |

---

#### Security Considerations

- Enforce tenant isolation on every SQL read and write.
- Redact or avoid secret values in logs, traces, and error payloads.
- Use fenced writes (`run_id`/`effect_id`) for ownership-sensitive state transitions.

---

#### Observability

- **Log events:** `callbacksigner.start`, `callbacksigner.success`, `callbacksigner.failure`
- **Metrics:** `release_engine_callbacksigner_total{status}`, `release_engine_callbacksigner_duration_seconds`
- **Trace span:** `outbox.signer`

---

#### Cross-Component Interactions

| Direction | Component | Mechanism | Data Exchanged |
|---|---|---|---|
| This → DBPool | outbound call | pgx query/exec | SQL statements, parameters, transaction result |
| This → MetricsExporter | outbound call | in-process API | counters, histograms, labels |
| This → TracingService | outbound call | OpenTelemetry | span start/end, attributes, errors |
| Upstream runtime → This | inbound call | in-process call | context, tenant_id, job_id, run_id |

### SPEC: MetricsExporter

**File:** `internal/observability/metrics_exporter.go`
**Package:** `observability`
**Phase:** 2
**Dependencies:** HTTPServer

---

#### Purpose

Publishes Prometheus-compatible counters, gauges, and histograms on `/metrics`.

---

#### Shared Context (duplicated for self-containment)

```go
// ErrorResponse is the standard API error envelope.
type ErrorResponse struct {
    Error   string `json:"error"`
    Code    string `json:"code"`
    Details any    `json:"details,omitempty"`
}

// JobCreateRequest is the POST /v1/jobs request schema.
type JobCreateRequest struct {
    TenantID       string         `json:"tenant_id"`
    PathKey        string         `json:"path_key"`
    Params         map[string]any `json:"params"`
    IdempotencyKey string         `json:"idempotency_key"`
    CallbackURL    *string        `json:"callback_url,omitempty"`
}

// JobStatusResponse is the GET /v1/jobs/{job_id} schema.
type JobStatusResponse struct {
    JobID            string     `json:"job_id"`
    TenantID         string     `json:"tenant_id"`
    State            string     `json:"state"`
    Attempt          int        `json:"attempt"`
    LeaseExpiresAt   *time.Time `json:"lease_expires_at,omitempty"`
    LastErrorCode    *string    `json:"last_error_code,omitempty"`
    LastErrorMessage *string    `json:"last_error_message,omitempty"`
}
```

Validation rules used by this component:
- `tenant_id` regex: `^[a-z0-9][a-z0-9-]{1,62}$`
- `idempotency_key` regex: `^[a-zA-Z0-9\-_.]{1,128}$`
- Maximum request body size: `262144` bytes
- Callback URL must use HTTPS and must not resolve to blocked private/link-local/metadata ranges.

---

#### Public Interface

```go
type Exporter interface {
    RegisterCollectors() error
}
```

##### Example — MetricsExporter

**Request:**
```json
{
  "tenant_id": "acme-prod",
  "job_id": "6c8bb1de-ef20-45cb-9e13-a57f44667d88",
  "run_id": "8a4cfa4d-f89c-4f0e-a809-f7d9a2c8f8ad"
}
```

**Response (200):**
```json
{
  "status": "ok",
  "component": "MetricsExporter"
}
```

**Response (error):**
```json
{
  "error": "operation failed",
  "code": "INTERNAL_ERROR",
  "details": null
}
```

---

#### Internal Logic (step-by-step)

1. Register metric families used by API, scheduler, runner, outbox, and reconciler.
2. Register approval lifecycle metrics:
   - `re_approval_requests_total{tenant_id,path_key,step_key}`
   - `re_approval_decisions_total{tenant_id,path_key,step_key,decision}`
   - `re_approval_latency_seconds{tenant_id,path_key}`
   - `re_approval_escalations_total{tenant_id,path_key}`
   - `re_approval_timeouts_total{tenant_id,path_key}`
   - `re_approval_worker_tick_duration_seconds{status}`
3. Expose metrics endpoint through HTTP server.
4. Provide in-process recording helpers consumed by `ApprovalService` and `ApprovalWorker`.
5. Ensure bounded label cardinality for tenant and component labels.

---

#### Data Model (if this component owns a table)

This component uses shared Release Engine tables and does not introduce a new table in this phase.

---

#### Error Table

| Condition | HTTP Status | Error Code | Response Body |
|---|---|---|---|
| Collector registration conflict | 500 | `METRICS_COLLECTOR_CONFLICT` | `{"error":"metrics collector conflict","code":"METRICS_COLLECTOR_CONFLICT","details":null}` |
| Metrics endpoint unavailable | 503 | `METRICS_ENDPOINT_UNAVAILABLE` | `{"error":"metrics endpoint unavailable","code":"METRICS_ENDPOINT_UNAVAILABLE","details":null}` |

---

#### Acceptance Criteria (Gherkin)

```gherkin
Feature: MetricsExporter

  Scenario: Happy path
    Given valid input and healthy dependencies
    When the component executes
    Then it returns success and emits logs, metrics, and traces

  Scenario: Edge case
    Given replayed state or partial prior progress
    When the component executes again
    Then it behaves deterministically without duplicate side effects

  Scenario: Error
    Given dependency failure or stale run ownership
    When the component executes
    Then it returns a structured error with explicit code
```

---

#### Performance Targets

| Metric | Target |
|---|---|
| Metrics scrape latency p95 | <100ms |
| Exporter overhead | <2% CPU |

---

#### Security Considerations

- Enforce tenant isolation on every SQL read and write.
- Redact or avoid secret values in logs, traces, and error payloads.
- Use fenced writes (`run_id`/`effect_id`) for ownership-sensitive state transitions.

---

#### Observability

- **Log events:** `metricsexporter.start`, `metricsexporter.success`, `metricsexporter.failure`
- **Metrics:** `release_engine_metricsexporter_total{status}`, `release_engine_metricsexporter_duration_seconds`
- **Trace span:** `observability.metrics`

Approval-specific telemetry emitted through this component:
- `re_approval_requests_total`
- `re_approval_decisions_total`
- `re_approval_latency_seconds`
- `re_approval_escalations_total`
- `re_approval_timeouts_total`
- `re_approval_worker_tick_duration_seconds`

---

#### Cross-Component Interactions

| Direction | Component | Mechanism | Data Exchanged |
|---|---|---|---|
| This → DBPool | outbound call | pgx query/exec | SQL statements, parameters, transaction result |
| This → MetricsExporter | outbound call | in-process API | counters, histograms, labels |
| This → TracingService | outbound call | OpenTelemetry | span start/end, attributes, errors |
| Upstream runtime → This | inbound call | in-process call | context, tenant_id, job_id, run_id |

### SPEC: MetricsSQLWriter

**File:** `internal/observability/metrics_sql_writer.go`
**Package:** `observability`
**Phase:** 2
**Dependencies:** DBPool

---

#### Purpose

Writes immutable operational events into `metrics_job_events` for audit and long-term analysis.

---

#### Shared Context (duplicated for self-containment)

```go
// ErrorResponse is the standard API error envelope.
type ErrorResponse struct {
    Error   string `json:"error"`
    Code    string `json:"code"`
    Details any    `json:"details,omitempty"`
}

// JobCreateRequest is the POST /v1/jobs request schema.
type JobCreateRequest struct {
    TenantID       string         `json:"tenant_id"`
    PathKey        string         `json:"path_key"`
    Params         map[string]any `json:"params"`
    IdempotencyKey string         `json:"idempotency_key"`
    CallbackURL    *string        `json:"callback_url,omitempty"`
}

// JobStatusResponse is the GET /v1/jobs/{job_id} schema.
type JobStatusResponse struct {
    JobID            string     `json:"job_id"`
    TenantID         string     `json:"tenant_id"`
    State            string     `json:"state"`
    Attempt          int        `json:"attempt"`
    LeaseExpiresAt   *time.Time `json:"lease_expires_at,omitempty"`
    LastErrorCode    *string    `json:"last_error_code,omitempty"`
    LastErrorMessage *string    `json:"last_error_message,omitempty"`
}
```

Validation rules used by this component:
- `tenant_id` regex: `^[a-z0-9][a-z0-9-]{1,62}$`
- `idempotency_key` regex: `^[a-zA-Z0-9\-_.]{1,128}$`
- Maximum request body size: `262144` bytes
- Callback URL must use HTTPS and must not resolve to blocked private/link-local/metadata ranges.

---

#### Public Interface

```go
type MetricsWriter interface {
    WriteEvent(ctx context.Context, event MetricsEvent) error
}
```

##### Example — MetricsSQLWriter

**Request:**
```json
{
  "tenant_id": "acme-prod",
  "job_id": "6c8bb1de-ef20-45cb-9e13-a57f44667d88",
  "run_id": "8a4cfa4d-f89c-4f0e-a809-f7d9a2c8f8ad"
}
```

**Response (200):**
```json
{
  "status": "ok",
  "component": "MetricsSQLWriter"
}
```

**Response (error):**
```json
{
  "error": "operation failed",
  "code": "INTERNAL_ERROR",
  "details": null
}
```

---

#### Internal Logic (step-by-step)

1. Map runtime events to metrics_sql row schema.
2. Insert events asynchronously with bounded worker pool.
3. Drop or backpressure according to configured queue limits to avoid unbounded memory growth.

---

#### Data Model (if this component owns a table)

This component uses shared Release Engine tables and does not introduce a new table in this phase.

---

#### Error Table

| Condition | HTTP Status | Error Code | Response Body |
|---|---|---|---|
| Insert failure | 503 | `METRICS_SQL_WRITE_FAILED` | `{"error":"metrics sql write failed","code":"METRICS_SQL_WRITE_FAILED","details":null}` |
| Queue overflow | 503 | `METRICS_SQL_QUEUE_FULL` | `{"error":"metrics sql queue full","code":"METRICS_SQL_QUEUE_FULL","details":null}` |

---

#### Acceptance Criteria (Gherkin)

```gherkin
Feature: MetricsSQLWriter

  Scenario: Happy path
    Given valid input and healthy dependencies
    When the component executes
    Then it returns success and emits logs, metrics, and traces

  Scenario: Edge case
    Given replayed state or partial prior progress
    When the component executes again
    Then it behaves deterministically without duplicate side effects

  Scenario: Error
    Given dependency failure or stale run ownership
    When the component executes
    Then it returns a structured error with explicit code
```

---

#### Performance Targets

| Metric | Target |
|---|---|
| Event write p95 | <25ms |
| Sustained ingest | >10,000 events/minute per instance |

---

#### Security Considerations

- Enforce tenant isolation on every SQL read and write.
- Redact or avoid secret values in logs, traces, and error payloads.
- Use fenced writes (`run_id`/`effect_id`) for ownership-sensitive state transitions.

---

#### Observability

- **Log events:** `metricssqlwriter.start`, `metricssqlwriter.success`, `metricssqlwriter.failure`
- **Metrics:** `release_engine_metricssqlwriter_total{status}`, `release_engine_metricssqlwriter_duration_seconds`
- **Trace span:** `observability.metrics_sql`

---

#### Cross-Component Interactions

| Direction | Component | Mechanism | Data Exchanged |
|---|---|---|---|
| This → DBPool | outbound call | pgx query/exec | SQL statements, parameters, transaction result |
| This → MetricsExporter | outbound call | in-process API | counters, histograms, labels |
| This → TracingService | outbound call | OpenTelemetry | span start/end, attributes, errors |
| Upstream runtime → This | inbound call | in-process call | context, tenant_id, job_id, run_id |

### SPEC: TracingService

**File:** `internal/observability/tracing.go`
**Package:** `observability`
**Phase:** 2
**Dependencies:** ConfigLoader

---

#### Purpose

Initialises OpenTelemetry tracing, applies sampling policy, and provides tracer instances to components.

---

#### Shared Context (duplicated for self-containment)

```go
// ErrorResponse is the standard API error envelope.
type ErrorResponse struct {
    Error   string `json:"error"`
    Code    string `json:"code"`
    Details any    `json:"details,omitempty"`
}

// JobCreateRequest is the POST /v1/jobs request schema.
type JobCreateRequest struct {
    TenantID       string         `json:"tenant_id"`
    PathKey        string         `json:"path_key"`
    Params         map[string]any `json:"params"`
    IdempotencyKey string         `json:"idempotency_key"`
    CallbackURL    *string        `json:"callback_url,omitempty"`
}

// JobStatusResponse is the GET /v1/jobs/{job_id} schema.
type JobStatusResponse struct {
    JobID            string     `json:"job_id"`
    TenantID         string     `json:"tenant_id"`
    State            string     `json:"state"`
    Attempt          int        `json:"attempt"`
    LeaseExpiresAt   *time.Time `json:"lease_expires_at,omitempty"`
    LastErrorCode    *string    `json:"last_error_code,omitempty"`
    LastErrorMessage *string    `json:"last_error_message,omitempty"`
}
```

Validation rules used by this component:
- `tenant_id` regex: `^[a-z0-9][a-z0-9-]{1,62}$`
- `idempotency_key` regex: `^[a-zA-Z0-9\-_.]{1,128}$`
- Maximum request body size: `262144` bytes
- Callback URL must use HTTPS and must not resolve to blocked private/link-local/metadata ranges.

---

#### Public Interface

```go
type Tracing interface {
    Tracer(name string) trace.Tracer
    Shutdown(ctx context.Context) error
}
```

##### Example — TracingService

**Request:**
```json
{
  "tenant_id": "acme-prod",
  "job_id": "6c8bb1de-ef20-45cb-9e13-a57f44667d88",
  "run_id": "8a4cfa4d-f89c-4f0e-a809-f7d9a2c8f8ad"
}
```

**Response (200):**
```json
{
  "status": "ok",
  "component": "TracingService"
}
```

**Response (error):**
```json
{
  "error": "operation failed",
  "code": "INTERNAL_ERROR",
  "details": null
}
```

---

#### Internal Logic (step-by-step)

1. Initialise OTLP exporter and service resource attributes.
2. Apply 100% sampling during bootstrap period, then 10% steady-state sampling.
3. Ensure trace context propagation across API, scheduler, runner, connector, and outbox flows.

---

#### Data Model (if this component owns a table)

This component uses shared Release Engine tables and does not introduce a new table in this phase.

---

#### Error Table

| Condition | HTTP Status | Error Code | Response Body |
|---|---|---|---|
| Exporter initialisation failure | 503 | `TRACING_INIT_FAILED` | `{"error":"tracing initialisation failed","code":"TRACING_INIT_FAILED","details":null}` |
| Exporter flush timeout | 503 | `TRACING_FLUSH_TIMEOUT` | `{"error":"tracing flush timed out","code":"TRACING_FLUSH_TIMEOUT","details":null}` |

---

#### Acceptance Criteria (Gherkin)

```gherkin
Feature: TracingService

  Scenario: Happy path
    Given valid input and healthy dependencies
    When the component executes
    Then it returns success and emits logs, metrics, and traces

  Scenario: Edge case
    Given replayed state or partial prior progress
    When the component executes again
    Then it behaves deterministically without duplicate side effects

  Scenario: Error
    Given dependency failure or stale run ownership
    When the component executes
    Then it returns a structured error with explicit code
```

---

#### Performance Targets

| Metric | Target |
|---|---|
| Span creation overhead | <1ms |
| Export batch flush | <2s |

---

#### Security Considerations

- Enforce tenant isolation on every SQL read and write.
- Redact or avoid secret values in logs, traces, and error payloads.
- Use fenced writes (`run_id`/`effect_id`) for ownership-sensitive state transitions.

---

#### Observability

- **Log events:** `tracing.start`, `tracing.success`, `tracing.failure`
- **Metrics:** `release_engine_tracing_total{status}`, `release_engine_tracing_duration_seconds`
- **Trace span:** `observability.tracing`

---

#### Cross-Component Interactions

| Direction | Component | Mechanism | Data Exchanged |
|---|---|---|---|
| This → DBPool | outbound call | pgx query/exec | SQL statements, parameters, transaction result |
| This → MetricsExporter | outbound call | in-process API | counters, histograms, labels |
| This → TracingService | outbound call | OpenTelemetry | span start/end, attributes, errors |
| Upstream runtime → This | inbound call | in-process call | context, tenant_id, job_id, run_id |

### SPEC: AuditService

**File:** `internal/audit/service.go`
**Package:** `audit`
**Phase:** 2
**Dependencies:** DBPool, LoggerFactory

---

#### Purpose

Persists and emits audit events for policy decisions, administrative actions, and sensitive runtime transitions.

---

#### Shared Context (duplicated for self-containment)

```go
// ErrorResponse is the standard API error envelope.
type ErrorResponse struct {
    Error   string `json:"error"`
    Code    string `json:"code"`
    Details any    `json:"details,omitempty"`
}

// JobCreateRequest is the POST /v1/jobs request schema.
type JobCreateRequest struct {
    TenantID       string         `json:"tenant_id"`
    PathKey        string         `json:"path_key"`
    Params         map[string]any `json:"params"`
    IdempotencyKey string         `json:"idempotency_key"`
    CallbackURL    *string        `json:"callback_url,omitempty"`
}

// JobStatusResponse is the GET /v1/jobs/{job_id} schema.
type JobStatusResponse struct {
    JobID            string     `json:"job_id"`
    TenantID         string     `json:"tenant_id"`
    State            string     `json:"state"`
    Attempt          int        `json:"attempt"`
    LeaseExpiresAt   *time.Time `json:"lease_expires_at,omitempty"`
    LastErrorCode    *string    `json:"last_error_code,omitempty"`
    LastErrorMessage *string    `json:"last_error_message,omitempty"`
}
```

Validation rules used by this component:
- `tenant_id` regex: `^[a-z0-9][a-z0-9-]{1,62}$`
- `idempotency_key` regex: `^[a-zA-Z0-9\-_.]{1,128}$`
- Maximum request body size: `262144` bytes
- Callback URL must use HTTPS and must not resolve to blocked private/link-local/metadata ranges.

---

#### Public Interface

```go
type Auditor interface {
    Record(ctx context.Context, event AuditEvent) error
}
```

##### Example — AuditService

**Request:**
```json
{
  "tenant_id": "acme-prod",
  "job_id": "6c8bb1de-ef20-45cb-9e13-a57f44667d88",
  "run_id": "8a4cfa4d-f89c-4f0e-a809-f7d9a2c8f8ad"
}
```

**Response (200):**
```json
{
  "status": "ok",
  "component": "AuditService"
}
```

**Response (error):**
```json
{
  "error": "operation failed",
  "code": "INTERNAL_ERROR",
  "details": null
}
```

---

#### Internal Logic (step-by-step)

1. Construct audit event with tenant, principal, action, target, decision, and reason.
2. Write immutable event to `audit_log` table.
3. Emit parallel structured log for SIEM ingestion.

---

#### Data Model (if this component owns a table)

This component uses shared Release Engine tables and does not introduce a new table in this phase.

---

#### Error Table

| Condition | HTTP Status | Error Code | Response Body |
|---|---|---|---|
| Audit write failure | 503 | `AUDIT_WRITE_FAILED` | `{"error":"audit write failed","code":"AUDIT_WRITE_FAILED","details":null}` |
| Malformed audit event | 400 | `AUDIT_EVENT_INVALID` | `{"error":"invalid audit event","code":"AUDIT_EVENT_INVALID","details":null}` |

---

#### Acceptance Criteria (Gherkin)

```gherkin
Feature: AuditService

  Scenario: Happy path
    Given valid input and healthy dependencies
    When the component executes
    Then it returns success and emits logs, metrics, and traces

  Scenario: Edge case
    Given replayed state or partial prior progress
    When the component executes again
    Then it behaves deterministically without duplicate side effects

  Scenario: Error
    Given dependency failure or stale run ownership
    When the component executes
    Then it returns a structured error with explicit code
```

---

#### Performance Targets

| Metric | Target |
|---|---|
| Audit write p95 | <20ms |
| Event serialisation | <2ms |

---

#### Security Considerations

- Enforce tenant isolation on every SQL read and write.
- Redact or avoid secret values in logs, traces, and error payloads.
- Use fenced writes (`run_id`/`effect_id`) for ownership-sensitive state transitions.

---

#### Observability

- **Log events:** `audit.start`, `audit.success`, `audit.failure`
- **Metrics:** `release_engine_audit_total{status}`, `release_engine_audit_duration_seconds`
- **Trace span:** `audit.record`

---

#### Cross-Component Interactions

| Direction | Component | Mechanism | Data Exchanged |
|---|---|---|---|
| This → DBPool | outbound call | pgx query/exec | SQL statements, parameters, transaction result |
| This → MetricsExporter | outbound call | in-process API | counters, histograms, labels |
| This → TracingService | outbound call | OpenTelemetry | span start/end, attributes, errors |
| Upstream runtime → This | inbound call | in-process call | context, tenant_id, job_id, run_id |

### SPEC: MigrationChecker

**File:** `internal/db/migration_checker.go`
**Package:** `db`
**Phase:** 0
**Dependencies:** DBPool

---

#### Purpose

Verifies schema version and migration integrity at startup and exposes readiness status.

---

#### Shared Context (duplicated for self-containment)

```go
// ErrorResponse is the standard API error envelope.
type ErrorResponse struct {
    Error   string `json:"error"`
    Code    string `json:"code"`
    Details any    `json:"details,omitempty"`
}

// JobCreateRequest is the POST /v1/jobs request schema.
type JobCreateRequest struct {
    TenantID       string         `json:"tenant_id"`
    PathKey        string         `json:"path_key"`
    Params         map[string]any `json:"params"`
    IdempotencyKey string         `json:"idempotency_key"`
    CallbackURL    *string        `json:"callback_url,omitempty"`
}

// JobStatusResponse is the GET /v1/jobs/{job_id} schema.
type JobStatusResponse struct {
    JobID            string     `json:"job_id"`
    TenantID         string     `json:"tenant_id"`
    State            string     `json:"state"`
    Attempt          int        `json:"attempt"`
    LeaseExpiresAt   *time.Time `json:"lease_expires_at,omitempty"`
    LastErrorCode    *string    `json:"last_error_code,omitempty"`
    LastErrorMessage *string    `json:"last_error_message,omitempty"`
}
```

Validation rules used by this component:
- `tenant_id` regex: `^[a-z0-9][a-z0-9-]{1,62}$`
- `idempotency_key` regex: `^[a-zA-Z0-9\-_.]{1,128}$`
- Maximum request body size: `262144` bytes
- Callback URL must use HTTPS and must not resolve to blocked private/link-local/metadata ranges.

---

#### Public Interface

```go
type Checker interface {
    CurrentVersion(ctx context.Context) (string, error)
    IsUpToDate(ctx context.Context) (bool, error)
}
```

##### Example — MigrationChecker

**Request:**
```json
{
  "tenant_id": "acme-prod",
  "job_id": "6c8bb1de-ef20-45cb-9e13-a57f44667d88",
  "run_id": "8a4cfa4d-f89c-4f0e-a809-f7d9a2c8f8ad"
}
```

**Response (200):**
```json
{
  "status": "ok",
  "component": "MigrationChecker"
}
```

**Response (error):**
```json
{
  "error": "operation failed",
  "code": "INTERNAL_ERROR",
  "details": null
}
```

---

#### Internal Logic (step-by-step)

1. Read applied migration version from schema_migrations table.
2. Compare with application expected version.
3. Return readiness failure when schema is behind or drift is detected.

---

#### Data Model (if this component owns a table)

This component uses shared Release Engine tables and does not introduce a new table in this phase.

---

#### Error Table

| Condition | HTTP Status | Error Code | Response Body |
|---|---|---|---|
| Schema out of date | 503 | `MIGRATION_OUTDATED` | `{"error":"database schema out of date","code":"MIGRATION_OUTDATED","details":null}` |
| Migration table missing | 500 | `MIGRATION_METADATA_MISSING` | `{"error":"migration metadata missing","code":"MIGRATION_METADATA_MISSING","details":null}` |

---

#### Acceptance Criteria (Gherkin)

```gherkin
Feature: MigrationChecker

  Scenario: Happy path
    Given valid input and healthy dependencies
    When the component executes
    Then it returns success and emits logs, metrics, and traces

  Scenario: Edge case
    Given replayed state or partial prior progress
    When the component executes again
    Then it behaves deterministically without duplicate side effects

  Scenario: Error
    Given dependency failure or stale run ownership
    When the component executes
    Then it returns a structured error with explicit code
```

---

#### Performance Targets

| Metric | Target |
|---|---|
| Version check p95 | <20ms |
| Startup verification | <100ms |

---

#### Security Considerations

- Enforce tenant isolation on every SQL read and write.
- Redact or avoid secret values in logs, traces, and error payloads.
- Use fenced writes (`run_id`/`effect_id`) for ownership-sensitive state transitions.

---

#### Observability

- **Log events:** `migrationchecker.start`, `migrationchecker.success`, `migrationchecker.failure`
- **Metrics:** `release_engine_migrationchecker_total{status}`, `release_engine_migrationchecker_duration_seconds`
- **Trace span:** `db.migration_check`

---

#### Cross-Component Interactions

| Direction | Component | Mechanism | Data Exchanged |
|---|---|---|---|
| This → DBPool | outbound call | pgx query/exec | SQL statements, parameters, transaction result |
| This → MetricsExporter | outbound call | in-process API | counters, histograms, labels |
| This → TracingService | outbound call | OpenTelemetry | span start/end, attributes, errors |
| Upstream runtime → This | inbound call | in-process call | context, tenant_id, job_id, run_id |
