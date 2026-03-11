# Phase 2 — Architecture Artefacts

## Table of Contents

- [2A — System Context Diagram (Mermaid)](#2a--system-context-diagram-mermaid)
- [2B — Component Inventory](#2b--component-inventory)
- [2C — Shared Types Catalogue](#2c--shared-types-catalogue)
- [2D — Configuration & Environment Variables](#2d--configuration--environment-variables)
- [2E — State Machine Extension](#2e--state-machine-extension)

## 2A — System Context Diagram (Mermaid)

```mermaid
flowchart LR
  subgraph Clients["Clients"]
    Portal["Client Portal and Backstage"]
    AIOps["TechOps Assistant and Agents"]
  end

  subgraph RE["Release Engine"]
    API["API Layer - Echo"]
    RL[Rate Limiter]
    PE["Policy Engine and RBAC"]
    IDEM[Idempotency Service]
    SCH[Scheduler]
    RUN[Runner]
    REG[Registry]
    MOD["Modules compiled in"]
    CONN["Connectors compiled in"]
    VOLTA[Volta VaultManager]
    OUTBOX[Outbox Worker]
    OBS["Metrics and Tracing Exporters"]
    APPROVAL[Approval Service]
  end

  subgraph Data["Data"]
    PG[(PostgreSQL 16)]
  end

  subgraph AWS["AWS"]
    SM[AWS Secrets Manager]
    S3[AWS S3]
  end

  subgraph External["External"]
    PROVIDER[External Providers]
    CALLBACK[Callback Endpoint]
    METRICS["Prometheus and AMP"]
    TRACE["Trace Collector and X-Ray"]
  end

  Portal -->|HTTPS TLS13 Bearer JWT OIDC| API
  AIOps -->|HTTPS TLS13 Bearer JWT OIDC| API

  API -->|in-process| RL
  API -->|in-process| PE
  API -->|in-process| IDEM
  API -->|in-process| APPROVAL
  API -->|PostgreSQL over TLS SCRAM via PgBouncer tx pool| PG

  SCH -->|SELECT FOR UPDATE SKIP LOCKED with run_id fencing| PG
  SCH -->|in process dispatch with context run_id lease| RUN
  SCH -->|check pending approvals| APPROVAL

  RUN -->|in-process lookup| REG
  RUN -->|in-process StepAPI| MOD
  RUN -->|wait for approval| APPROVAL
  MOD -->|in-process connector call request| CONN
  RUN -->|VaultService UseSecret in guarded scope| VOLTA
  CONN -->|HTTPS TLS12 provider auth call_id idempotency token| PROVIDER
  RUN -->|SQL writes jobs jobs_read steps external_effects outbox| PG

  VOLTA -->|AWS SigV4 HTTPS TLS12 IAM role auth GetSecretValue startup| SM
  VOLTA -->|S3 API HTTPS TLS12 IAM role auth encrypted read write| S3

  OUTBOX -->|SQL claim/update| PG
  OUTBOX -->|HTTPS TLS12 HMAC SHA256 delivery_id| CALLBACK

  OBS -->|Prometheus metrics endpoint over HTTPS| METRICS
  OBS -->|OTLP gRPC or HTTP over TLS with OpenTelemetry context| TRACE
```

## 2B — Component Inventory

| Component | Type | Phase | Dependencies | Estimated Complexity |
|---|---|---|---|---|
| ConfigLoader | pkg | 0 | none | low |
| LoggerFactory | pkg | 0 | ConfigLoader | low |
| DBPool (pgx + PgBouncer) | pkg | 0 | ConfigLoader | low |
| HTTPServer (Echo bootstrap) | transport | 0 | ConfigLoader, LoggerFactory | low |
| AuthMiddleware (OIDC JWT) | middleware | 1 | ConfigLoader, HTTPServer | medium |
| RateLimiter | middleware | 1 | ConfigLoader | medium |
| PolicyEngine | service | 1 | ConfigLoader, DBPool | medium |
| IdempotencyService | service | 1 | DBPool, PolicyEngine | high |
| JobsAPIHandler | transport | 1 | AuthMiddleware, RateLimiter, PolicyEngine, IdempotencyService, DBPool | high |
| HealthHandler (`/healthz`, `/readyz`) | transport | 1 | DBPool, SchedulerService, MigrationChecker | low |
| SchedulerService | service | 1 | DBPool, Registry, MetricsService | high |
| LeaseManager | pkg | 1 | DBPool | medium |
| RunnerService | service | 2 | DBPool, Registry, VoltaManager, MetricsService, TracingService | high |
| StepAPIAdapter | service | 2 | DBPool, RunnerService | medium |
| ModuleRegistry | service | 2 | ConfigLoader | low |
| ConnectorRegistry | service | 2 | ConfigLoader | low |
| VoltaManager | service | 2 | ConfigLoader, LoggerFactory | high |
| ReconcilerService | service | 2 | DBPool, ConnectorRegistry, MetricsService | high |
| OutboxDispatcher | service | 2 | DBPool, CallbackSigner, MetricsService | high |
| ApprovalService | service | 2 | DBPool, PolicyEngine, OutboxDispatcher | high |
| CallbackSigner (HMAC rotation) | pkg | 2 | ConfigLoader | medium |
| MetricsExporter (Prometheus) | observability | 2 | HTTPServer | medium |
| MetricsSQLWriter | observability | 2 | DBPool | medium |
| TracingService (OpenTelemetry) | observability | 2 | ConfigLoader | medium |
| AuditService | observability | 2 | DBPool, LoggerFactory | medium |
| MigrationChecker | pkg | 0 | DBPool | low |

## 2C — Shared Types Catalogue

```go
package shared

import "time"

// (Keep existing shared types and add approval-related types)

// ApprovalRecord models a human decision for a step.
// Used by: ApprovalService, AuditService
type ApprovalRecord struct {
	ID             string    `json:"id"`
	JobID          string    `json:"job_id"`
	StepID         int64     `json:"step_id"`
	Decision       string    `json:"decision"`       // approved|rejected|expired
	Approver       string    `json:"approver"`
	Justification  string    `json:"justification"`
	PolicySnapshot any       `json:"policy_snapshot"`
	CreatedAt      time.Time `json:"created_at"`
}

// ApprovalRequest is the record of an approval gate defined in a step.
// Used by: Module runtime via StepAPI
type ApprovalRequest struct {
	Summary     string            `json:"summary"`
	Detail      string            `json:"detail"`
	BlastRadius string            `json:"blast_radius"`
	PolicyRef   string            `json:"policy_ref"`
	Metadata    map[string]string `json:"metadata"`
}
```

## 2D — Configuration & Environment Variables

(Keep existing, add for ApprovalService if needed)

## 2E — State Machine Extension

The step state machine now includes `waiting_approval`.

```mermaid
stateDiagram-v2
  [*] --> queued: submit
  queued --> running: claim
  running --> ok: finalise success
  running --> error: finalise failure
  running --> jobs_exhausted: retry exhausted
  running --> queued: retry scheduling
  running --> canceled: cancel requested

  running --> waiting_approval: step needs approval
  waiting_approval --> running: step approved
  waiting_approval --> error: step rejected
  waiting_approval --> canceled: job canceled
  waiting_approval --> error: approval timeout
```

- **`waiting_approval`**: A special state where a step is parked awaiting human intervention. The scheduler does not claim subsequent steps in the job until this step is resolved.
- **Triggered when a module encounters a point requiring approval.**
- **Transitions to `running` on approval, `error` on rejection or timeout.**
