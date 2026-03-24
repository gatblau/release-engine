# Phase 2 — Architecture Artefacts

## Table of Contents

- [2A — System Context Diagram (Mermaid)](#2a--system-context-diagram-mermaid)
- [2A.1 — Connector Context Diagram (Mermaid)](#2a1--connector-context-diagram-mermaid)
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
    DORAAPI["DORA API Handler"]
    DORABRIDGE["DORA Internal Event Bridge"]
    DORAREG["DORA Normalizer Registry"]
    GHNORM["GitHub Normalizer"]
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
  API -->|in-process| DORAAPI
  API -->|provider dispatch| DORAREG
  DORAREG -->|github| GHNORM
  API -->|PostgreSQL over TLS SCRAM via PgBouncer tx pool| PG

  DORABRIDGE -->|atomic write metrics_job_events + dora_events + dora_commit_deployment_links| PG

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

## 2A.1 — Connector Context Diagram (Mermaid)

```mermaid
flowchart LR
  subgraph Modules["Modules"]
    M1["Module 1: scaffold-service"]
    M2["Module 2: deploy-eks"]
  end

  subgraph Connectors["Connectors"]
    CG["GitHub Git"]
    CA["AWS Cloud"]
    CK["Crossplane Infra"]
  end

  subgraph Providers["External Providers"]
    PG[GitHub API]
    PA[AWS API]
    PC[Crossplane API]
  end

  subgraph Registry["Connector Registry"]
    CR[Connector Registry]
  end

  subgraph Runner["Release Engine Runner"]
    RS[Runner Service]
    SA[Step Executor]
  end

  subgraph Engine["Release Engine Core"]
    SE[Scheduler]
    JE[Jobs Engine]
  end

  M1 -->|connector: git-github| SA
  M2 -->|connector: cloud-aws| SA
  M2 -->|connector: infra-crossplane| SA

  SA -->|lookup| CR
  CR -->|returns| CG
  CR -->|returns| CA
  CR -->|returns| CK

  CG -->|HTTPS + call_id| PG
  CA -->|HTTPS + call_id| PA
  CK -->|HTTPS + call_id| PC

  JE -->|schedule| SE
  SE -->|dispatch| RS
  RS -->|execute| SA
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
| Runner Step Executor | service | 2 | ConnectorRegistry, LoggerFactory, MetricsExporter | medium |
| SecretContextProvider | interface | 2 | Module runtime | low |
| SecretRequirer | interface | 2 | Connector runtime | low |
| StepAPIAdapter (Secret Orchestration) | service | 2 | DBPool, RunnerService, VoltaManager, ModuleRegistry, ConnectorRegistry | high |
| BaseConnector | pkg | 2 | Shared Types | low |
| GitHubConnector | service | 2 | BaseConnector, ConnectorConfig, VoltaManager | high |
| CrossplaneConnector | service | 2 | BaseConnector, ConnectorConfig, VoltaManager, Kubernetes client | high |
| AWSConnector | service | 2 | BaseConnector, ConnectorConfig, VoltaManager, AWS SDK v2 | high |
| Connector Testing Framework | pkg | 3 | Shared Types, ConnectorRegistry | medium |
| Startup Wiring | pkg | 1 | ConfigLoader, ConnectorRegistry, VoltaManager | medium |
| VoltaManager | service | 2 | ConfigLoader, LoggerFactory | high |
| ReconcilerService | service | 2 | DBPool, ConnectorRegistry, MetricsService | high |
| OutboxDispatcher | service | 2 | DBPool, CallbackSigner, MetricsService | high |
| ApprovalService | service | 2 | DBPool, PolicyEngine, OutboxDispatcher | high |
| DoraAPIHandler | transport | 1 | AuthMiddleware, RateLimiter, DBPool | medium |
| DoraEventBridge (in MetricsSQLWriter) | observability | 1 | DBPool | medium |
| DoraNormalizerRegistry | service | 2 | DoraAPIHandler | medium |
| GitHubDoraNormalizer | service | 2 | DoraNormalizerRegistry | medium |
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

### Connector Shared Types Catalogue

```go
package connector

import (
    "context"
    "fmt"
    "strings"
    "time"
)

// --- Connector Types ---

type ConnectorType string

const (
    ConnectorTypeGit    ConnectorType = "git"
    ConnectorTypeCloud  ConnectorType = "cloud"
    ConnectorTypeCD     ConnectorType = "cd"
    ConnectorTypeInfra  ConnectorType = "infra"
    ConnectorTypeDevOps ConnectorType = "devops"
    ConnectorTypeOther  ConnectorType = "other"
)

var ValidConnectorTypes = map[ConnectorType]bool{
    ConnectorTypeGit:    true,
    ConnectorTypeCloud:  true,
    ConnectorTypeCD:     true,
    ConnectorTypeInfra:  true,
    ConnectorTypeDevOps: true,
    ConnectorTypeOther:  true,
}

// --- Result Types ---

const (
    StatusSuccess        = "success"
    StatusRetryableError = "retryable_error"
    StatusTerminalError  = "terminal_error"
)

type ConnectorResult struct {
    Status string
    Output map[string]interface{}
    Error  *ConnectorError
}

type ConnectorError struct {
    Code    string
    Message string
    Details map[string]interface{}
}

func (e *ConnectorError) Error() string {
    return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

type ConnectorConfig struct {
    HTTPTimeout      time.Duration
    TransportRetries int
    Extra            map[string]string
}

func DefaultConnectorConfig() ConnectorConfig {
    return ConnectorConfig{
        HTTPTimeout:      30 * time.Second,
        TransportRetries: 3,
        Extra:            make(map[string]string),
    }
}

type contextKey string

const callIDKey contextKey = "call_id"

func WithCallID(ctx context.Context, callID string) context.Context {
    return context.WithValue(ctx, callIDKey, callID)
}

func CallIDFromContext(ctx context.Context) string {
    if v, ok := ctx.Value(callIDKey).(string); ok {
        return v
    }
    return ""
}

type Connector interface {
    Key() string
    Validate(operation string, input map[string]interface{}) error
    Execute(ctx context.Context, operation string, input map[string]interface{}) (*ConnectorResult, error)
    Close() error
}

type OperationDescriber interface {
    Operations() []OperationMeta
}

type OperationMeta struct {
    Name           string
    Description    string
    RequiredFields []string
    OptionalFields []string
    IsAsync        bool
}

type ConnectorRegistry interface {
    Register(conn Connector) error
    Replace(conn Connector) error
    Lookup(key string) (Connector, bool)
    ListByType(connectorType ConnectorType) []Connector
    Close() error
}

type BaseConnector struct {
    connectorType ConnectorType
    technology    string
}

func NewBaseConnector(ctype ConnectorType, tech string) (BaseConnector, error) {
    if !ValidConnectorTypes[ctype] {
        return BaseConnector{}, fmt.Errorf("unknown connector type: %s", ctype)
    }
    if tech == "" {
        return BaseConnector{}, fmt.Errorf("technology must not be empty")
    }
    if strings.Contains(tech, "-") {
        return BaseConnector{}, fmt.Errorf("technology must not contain hyphens: %s", tech)
    }
    return BaseConnector{connectorType: ctype, technology: tech}, nil
}

func (b *BaseConnector) Type() ConnectorType { return b.connectorType }
func (b *BaseConnector) Technology() string   { return b.technology }
func (b *BaseConnector) Key() string          { return fmt.Sprintf("%s-%s", b.connectorType, b.technology) }
```

## 2D — Configuration & Environment Variables

| Variable | Default | Purpose |
|---|---|---|
| `DORA_GROUP_MAP_TTL` | `15m` | Maximum staleness tolerated for `dora_group_brand_map` during group-scoped reads. |
| `DORA_LEAD_TIME_COVERAGE_THRESHOLD` | `0.8` | Minimum correlated deployment coverage required for Lead Time `data_quality=complete`. |
| `DORA_CLASSIFICATION_VERSION` | `dora-2023-default+gates-included` | Default classification profile when request does not provide an override. |

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
