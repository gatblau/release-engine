# Phase 1 — Analysis & Ambiguity Resolution

## Table of Contents

- [1A — Assumptions Register](#1a--assumptions-register)
- [1B — Open Questions](#1b--open-questions)
- [1C — Glossary](#1c--glossary)

## 1A — Assumptions Register

| ID | Area | Assumption | Rationale | Impact if wrong |
|---|---|---|---|---|
| A-01 | Runtime | Release Engine is deployed as a stateless Go service on Kubernetes with multiple replicas behind an L7 load balancer. | Design defines horizontal scaling, pod IAM role use, and rolling restart semantics. | Deployment and HA model change; readiness, shutdown, and rollout specs must be rewritten. |
| A-02 | Transport Security | All inbound API traffic uses HTTPS with TLS 1.2+; production baseline is TLS 1.3 where supported by ingress. | Design requires JWT bearer auth and HSTS; secure transport is mandatory for token confidentiality. | Tokens can be exposed in transit; compliance and threat model fail. |
| A-03 | Authentication | JWT validation uses OIDC discovery + JWKS cache with issuer pinning, audience validation, and max 60-second clock skew. | Explicitly stated in design security requirements. | Authentication failures, bypass risk, or incompatibility with portal tokens. |
| A-04 | Authorization | RBAC decisions are evaluated in-process from policy/binding data; cache TTL is 60s for non-destructive actions and bypassed for `job:cancel`. | Explicitly stated; supports low-latency decisions and strong cancel safety. | Unauthorised actions or excessive auth latency depending on replacement model. |
| A-05 | Database | PostgreSQL 16 is the primary OLTP store; all business writes use `READ COMMITTED` isolation. | `SKIP LOCKED` and fencing semantics rely on `READ COMMITTED`. | Scheduler correctness and lease fencing are broken under stricter isolation defaults. |
| A-06 | Connection Pooling | PgBouncer runs in transaction pooling mode for all app DB connections. | Design includes transaction-mode constraints and sizing guidance. | Session-scoped features may fail; connection pressure may increase without pooling. |
| A-07 | Scheduler | Scheduler uses only `SELECT ... FOR UPDATE SKIP LOCKED` with no scheduler leader election. | Explicit architecture decision documented repeatedly. | Coordination logic, failure modes, and claim metrics contract change. |
| A-08 | Time Source | All lease and scheduling comparisons use database `now()`; application clocks are not trusted for correctness decisions. | Explicit invariant in lifecycle SQL section. | Clock skew can produce duplicate processing or lease starvation. |
| A-09 | Idempotency Scope | Job intake idempotency uniqueness key is `(tenant_id, path_key, idempotency_key)` with TTL 48 hours. | Explicitly documented in API semantics and table design. | Replay behavior and conflict detection semantics change for clients. |
| A-10 | Payload Limits | `POST /v1/jobs` enforces max body size 256 KB before any DB write. | Explicit API contract and security control. | Resource exhaustion risk and API behavior inconsistency. |
| A-11 | Outbox Delivery | Outbox uses at-least-once delivery with per-entry max attempts default `12`, capped backoff 600s, and `dlq` escalation. | Explicitly specified under webhooks and SQL flow. | Callback reliability and alerting behavior materially change. |
| A-12 | Provider Outcomes | Connector responses map to normalised outcomes: `ok`, `retryable`, `terminal`, `unknown`; unknown outcomes enter reconciler flow. | Explicit connector contract and effect lifecycle design. | Retry/reconcile behavior and audit trail taxonomy change. |
| A-13 | Reconciliation | Reconciler scan interval is 30s, max reconcile attempts default 5, and `unknown_outcome` P99 resolution target is ≤5 minutes. | Explicit in reconciler sections and SLO table. | Incident response and DLQ volume may significantly change. |
| A-14 | Module Runtime | Modules and connectors are compiled-in Go implementations; no plugin/WASM/script runtime. | Explicit decision record in design. | Build/deploy lifecycle and extensibility model change. |
| A-15 | Secrets Root-of-Trust | Volta master passphrase is fetched from AWS Secrets Manager once per process startup, then retained in memguard-protected memory. | Explicit security model in Volta section. | Secret bootstrap and runtime threat posture change. |
| A-16 | Secret Storage | Volta encrypted vault objects are stored in S3 with key prefix `volta/`, SSE-S3 enabled, and per-tenant key hierarchy KEK→DEK. | Explicit storage model and table in Volta design. | Secret management interfaces, IAM policy, and rotation process change. |
| A-17 | Observability Stack | Metrics are exposed via Prometheus-compatible endpoint; tracing via OpenTelemetry spans; structured logs are JSON. | Explicitly documented across observability sections. | Existing dashboards, alert rules, and incident workflows become incompatible. |
| A-18 | CI/CD | Delivery pipeline uses GitHub Actions for build/test and ArgoCD for runtime rollout and rollback decisions. | ArgoCD rollout behavior is explicit; CI tool not explicitly named, so GitHub Actions is a project constant assumption. | Automation scripts and release gates must be redesigned. |
| A-19 | Region & Compliance | Primary production footprint is EU region(s), with retention and erasure controls enforced per compliance requirements. | Explicit EU compliance section. | Legal/compliance controls and data residency guarantees change. |
| A-20 | Alert Routing | PagerDuty + Slack are available and configured for SLO/canary alerts. | Explicit canary rollback notification path in design. | Automated rollback may lack human escalation and response SLAs. |
| A-21 | Human-in-the-Loop | Approval gates implemented as a native engine feature using `approval_decisions` table and `steps` table extension. | Explicitly architected for native gate support, eliminating external dependency. | Decision tracking and audit trails for approvals would require external system delegation. |

## 1B — Open Questions

| ID | Question | Options | Impact | Blocking? |
|---|---|---|---|---|
| Q-01 | What exact API versioning strategy is required after `v1` (URI-only, header-based, or hybrid)? | URI `/v{n}` only; custom media type header; hybrid URI+header | Determines long-term compatibility strategy and client SDK design. | No — does not block Phase 2 artefacts. |
| Q-02 | Should SSE endpoint `GET /v1/jobs/{job_id}/events` be mandatory for GA or optional feature-flagged? | Mandatory at GA; optional behind feature flag; remove endpoint | Changes transport surface and component inventory for streaming. | No — callback outbox path still provides completion events. |
| Q-03 | Is policy evaluation implemented as native Go policy engine only, or must OPA-compatible bundles be supported at launch? | Go-native only; OPA-compatible in-process; dual-mode | Affects policy component interfaces, bundle loaders, and operational dependencies. | Yes — blocks detailed Phase 3 policy component spec. |
| Q-04 | Which schema migration tool is authoritative (`golang-migrate`, Atlas, Flyway, or custom SQL runner)? | golang-migrate; Atlas; custom migration runner | Impacts migration CI gates, rollback procedures, and bootstrap automation. | Yes — blocks implementation-level migration spec. |
| Q-05 | Should jobs and metrics_sql share one PostgreSQL cluster with Timescale extension, or use separate clusters? | Single cluster with extension; separate metrics cluster; managed timeseries backend | Affects capacity planning, backup/restore scope, and failure domain boundaries. | Yes — blocks final production topology spec. |
| Q-06 | What tenant fairness algorithm is required in claimer: strict round-robin or weighted fair sharing? | Strict round-robin; weighted fair queue by tenant quota; FIFO with caps | Affects scheduling implementation and starvation guarantees under load. | Yes — blocks detailed scheduler algorithm spec. |
| Q-07 | Which provider connectors are in MVP scope? | GitHub + Backstage only; add AWS baseline; full listed set | Impacts connector inventory, IAM setup, and end-to-end acceptance criteria. | Yes — blocks final component decomposition for Phase 3. |
| Q-08 | Should callback allowlist be global-only, tenant-specific, or both with precedence rules? | Global allowlist; tenant allowlist override; combined global+tenant rules | Affects callback URL validation logic and tenant onboarding model. | No — default safe implementation can proceed with both enforced. |

# Design Decisions for Release Engine

## Summary

Converting all 8 open questions into binding design decisions, optimizing for: reliability (single failure domains where safe, proven tools), operational simplicity (fewer moving parts at launch), and incremental extensibility (decisions that don't paint us into corners).

---

## Decisions

### DD-01: API Versioning Strategy (addresses Q-01)

| Attribute | Value |
|-----------|-------|
| **Decision** | **URI-path only (`/v{n}`)** |
| **Rationale** | Simplest for clients, load-balancer routing, documentation, and observability (version visible in access logs and metrics labels). Header-based versioning adds complexity with marginal benefit at our scale. |
| **Extension path** | If a future consumer requires fine-grained content negotiation, add `Accept` header versioning *within* a major URI version (e.g., `/v2` with `Accept: application/vnd.re.v2.1+json`). This is additive, not breaking. |
| **Status** | **Decided — non-blocking.** |

---

### DD-02: SSE Events Endpoint (addresses Q-02)

| Attribute | Value |
|-----------|-------|
| **Decision** | **Feature-flagged, off by default at GA.** |
| **Rationale** | The outbox/webhook path already delivers completion events reliably. SSE adds connection management, load-balancer tuning (long-lived connections), and an additional failure mode. Ship it behind `FF_SSE_EVENTS=true` so early adopters can opt in; promote to default-on in a subsequent release after production validation. |
| **Flag** | `features.sse_events.enabled` (config / env `FF_SSE_EVENTS`) |
| **Implication** | GA acceptance criteria do **not** include SSE. Client SDKs document webhook-first, SSE as experimental. |
| **Status** | **Decided — non-blocking.** |

---

### DD-03: Policy Engine Implementation (addresses Q-03)

| Attribute | Value |
|-----------|-------|
| **Decision** | **Go-native policy engine at launch, with OPA-compatible bundle interface designed in but not implemented.** |
| **Rationale** | A native Go engine eliminates the OPA sidecar/library dependency, reduces memory overhead, and keeps the blast radius small. However, enterprise customers will expect OPA/Rego compatibility. Design the `PolicyEvaluator` interface now with an `Evaluate(ctx, input) → Decision` contract that a future `OPAEvaluator` adapter can implement without changing callers. |
| **Extension path** | Phase 4+: add `OPAEvaluator` that loads Rego bundles from S3/Git. Feature-flagged per tenant (`policy.engine=native\|opa`). |
| **Status** | **Decided — unblocks Phase 3 policy spec.** |

**Interface sketch:**

```go
type PolicyEvaluator interface {
    Evaluate(ctx context.Context, input PolicyInput) (PolicyDecision, error)
    Reload(ctx context.Context) error
}
```

---

### DD-04: Schema Migration Tool (addresses Q-04)

| Attribute | Value |
|-----------|-------|
| **Decision** | **Atlas** (`ariga.io/atlas`) |
| **Rationale** | Atlas provides declarative schema-as-code with a powerful diff engine, versioned migration directory, CI linting (`atlas migrate lint`), and native Go integration. Compared to `golang-migrate` (manual SQL only, no drift detection) and Flyway (JVM dependency), Atlas gives us: (1) migration linting in CI, (2) automatic drift detection in production, (3) single binary, no JVM. |
| **CI gate** | `atlas migrate lint` runs on every PR touching `migrations/`. Fails on destructive changes without explicit `-- atlas:nolint` annotation + reviewer approval. |
| **Rollback** | Each migration directory entry includes a corresponding `down` file. `atlas migrate down` in runbook. Tested in staging before every prod apply. |
| **Status** | **Decided — unblocks migration spec.** |

---

### DD-05: PostgreSQL Topology for Jobs + Metrics (addresses Q-05)

| Attribute | Value |
|-----------|-------|
| **Decision** | **Single PostgreSQL (Aurora) cluster with TimescaleDB extension for `metrics_sql`.** |
| **Rationale** | At launch volumes (≤500 jobs/sec, ≤10K metric rows/sec), a single cluster is operationally simpler: one backup strategy, one connection pool, one failure domain to monitor. TimescaleDB hypertables handle time-series compression and retention on the metrics tables without affecting OLTP job tables. |
| **Extension path** | When metric ingest exceeds 50K rows/sec or metrics queries impact job SLO p99, split to a dedicated TimescaleDB/Managed Timestream cluster. The `MetricsWriter` interface already abstracts the backend. |
| **Status** | **Decided — unblocks production topology spec.** |

**Guardrails:**

1. `metrics_sql` tables on a separate tablespace with independent IOPS budget (Aurora I/O-Optimised).
2. Connection pool split: `jobs` pool (80%) / `metrics` pool (20%).
3. Alert on p99 query latency per pool — if metrics queries degrade job latency, that's the trigger to split.

**Breakpoint criteria:** Split when metrics query p99 > 50ms **AND** job claim latency p99 degrades >10% from baseline.

---

### DD-06: Tenant Fairness Algorithm in Claimer (addresses Q-06)

| Attribute | Value |
|-----------|-------|
| **Decision** | **Weighted fair sharing with per-tenant quotas.** |
| **Rationale** | Strict round-robin doesn't account for tenant tier differences (enterprise vs. free). FIFO-with-caps risks starvation under burst. Weighted fair queuing (WFQ) assigns each tenant a `weight` (from their plan/quota config) and the claimer distributes claim slots proportionally per scheduling cycle. |
| **Algorithm** | Deficit-weighted round-robin (DWRR): each tenant accumulates a deficit counter. Per claim cycle, tenants with highest deficit (= most underserved relative to weight) are served first. Simple, O(n) per cycle where n = active tenants. |
| **Starvation guarantee** | Every tenant with pending jobs is guaranteed ≥1 claim per `fairness_window` (default 10s), regardless of weight. |
| **Status** | **Decided — unblocks scheduler algorithm spec.** |

**Configuration example:**

```yaml
scheduling:
  fairness:
    algorithm: weighted_fair
    default_weight: 1
    fairness_window: 10s
  tenant_overrides:
    tenant_abc:
      weight: 10
      max_concurrent: 50
```

**Metrics:**

- `re_claimer_tenant_deficit`
- `re_claimer_tenant_claims_total`
- `re_claimer_starvation_events_total`

---

### DD-07: MVP Provider Connectors (addresses Q-07)

| Attribute | Value |
|-----------|-------|
| **Decision** | **GitHub + Backstage + AWS baseline (IAM, S3, EKS).** |
| **Rationale** | GitHub and Backstage are mandatory for the core GitOps and IDP flows. AWS baseline (IAM role management, S3 operations, EKS cluster interaction) is required because the platform runs on AWS and infrastructure jobs need these primitives. Adding AWS baseline at MVP avoids a gap where infra jobs would have no connector. |
| **Extension path** | Additional connectors (Terraform Cloud, PagerDuty, Datadog, etc.) added via the `ConnectorRegistry` in subsequent phases. Each connector implements the `Connector` interface and is registered at compile time. |
| **Status** | **Decided — unblocks Phase 3 component decomposition.** |

**Connector inventory (MVP):**

| Connector | Scope | Acceptance Criteria |
|-----------|-------|---------------------|
| `github` | Repo CRUD, PR creation, branch protection, webhook management | E2E: create repo, open PR, merge, verify via API |
| `backstage` | Catalog entity registration, scaffolder trigger, webhook receipt | E2E: register component, trigger scaffolder, receive callback |
| `aws-iam` | Role/policy CRUD, assume-role for tenant isolation | E2E: create role, attach policy, assume role, verify access |
| `aws-s3` | Bucket operations (used by Volta and artifact storage) | E2E: create bucket, put/get object, verify encryption |
| `aws-eks` | Cluster auth, namespace provisioning, ArgoCD Application resource creation | E2E: provision namespace, write ArgoCD `Application` manifest to Git (via `github` connector), verify ArgoCD syncs and namespace exists in cluster |

---

### DD-08: Callback Allowlist Model (addresses Q-08)

| Attribute | Value |
|-----------|-------|
| **Decision** | **Combined global + tenant-scoped allowlist with global-takes-precedence.** |
| **Rationale** | Global allowlist acts as a security ceiling (blocklist dangerous destinations, enforce HTTPS-only, block internal IPs). Tenant allowlist further restricts to their own callback URLs. A tenant cannot add a URL that the global list blocks. This is defense-in-depth without operational complexity. |
| **Config** | Global list managed via platform config (IaC). Tenant list managed via API (`PUT /v1/tenants/{id}/callback-allowlist`) with admin RBAC. |
| **Status** | **Decided — non-blocking, confirms safe default.** |

**Precedence rules:**

1. Global denylist checked first → reject if matched.
2. Global allowlist checked → reject if not matched (when global allowlist is non-empty).
3. Tenant allowlist checked → reject if not matched.
4. URL must be HTTPS with valid TLS (no self-signed in prod).

**Validation pseudocode:**

```
validateCallback(url, tenantID):
  if globalDenylist.matches(url)                              → REJECT
  if globalAllowlist.isActive() && !globalAllowlist.matches(url) → REJECT
  if !tenantAllowlist(tenantID).matches(url)                  → REJECT
  if !isHTTPS(url)                                            → REJECT
                                                              → ALLOW
```

---

## Decision Summary Table

| ID | Decision | Blocking Resolved? | Phase Impact |
|----|----------|-------------------|--------------|
| DD-01 | URI-path versioning (`/v{n}`) | N/A (was non-blocking) | All phases |
| DD-02 | SSE feature-flagged, off at GA | N/A (was non-blocking) | GA scope reduced |
| DD-03 | Go-native policy engine; OPA interface designed, not built | ✅ Unblocks Phase 3 | Phase 3 policy spec |
| DD-04 | Atlas for schema migrations | ✅ Unblocks migration spec | Phase 2–3 |
| DD-05 | Single Aurora cluster + TimescaleDB; split criteria defined | ✅ Unblocks topology spec | Phase 3 prod topology |
| DD-06 | Weighted fair sharing (DWRR) with starvation guarantee | ✅ Unblocks scheduler spec | Phase 3 scheduler |
| DD-07 | GitHub + Backstage + AWS baseline (IAM, S3, EKS) | ✅ Unblocks component decomposition | Phase 3 connectors |
| DD-08 | Global + tenant allowlist, global precedence | N/A (was non-blocking) | Phase 2–3 |

**All 4 blocking questions are now resolved.** Phase 3 detailed specs can proceed.


## 1C — Glossary

| Term | Definition | Example |
|---|---|---|
| Release Engine | The core durable, idempotent job scheduler/executor service in the platform. | `release-engine` deployment running 4 replicas. |
| Tenant | Logical isolation boundary for data, policy, rate limits, and secrets. | `tenant_id = "acme-prod"` |
| Job | Durable unit of work submitted to the API and executed asynchronously. | `job_id = "8f2d..."` |
| Path Key | External workflow identifier used by clients and RBAC/policy checks. | `path_key = "golden-path/scaffold-service"` |
| Module | Deterministic orchestration code implementing workflow steps with no direct I/O. | `scaffold-service:1.0.0` module. |
| Connector | External boundary component that performs protocol-specific provider calls. | `connector_key = "github"` |
| Registry | Resolution layer mapping path keys to module versions and connector bindings. | `binding_module[path_key] -> module_key/version` |
| Idempotency Key | Client-supplied key that deduplicates job creation within a fixed scope and TTL. | `idempotency_key = "req-20260304-001"` |
| Payload Fingerprint | SHA-256 over canonical JSON payload used to detect idempotency conflicts. | `fingerprint = sha256(canonical_request)` |
| Canonical Envelope | Stable response body stored for an idempotency key and replayed bit-for-bit. | `{"job_id":"...","state":"queued"}` |
| Lease | Time-bounded ownership marker for a running job/effect. | `lease_expires_at = 2026-03-04T19:30:00Z` |
| Run ID | UUID fencing token identifying the current execution owner of a job attempt. | `run_id = "d9bd..."` |
| Fencing | Conditional write strategy rejecting stale owners via `WHERE run_id = ...`. | 0-row update indicates lost lease. |
| SKIP LOCKED | PostgreSQL row-lock strategy allowing concurrent claimers to avoid locked rows. | `FOR UPDATE SKIP LOCKED LIMIT 10` |
| Outbox | Transactional event table for webhook/callback delivery with retries and DLQ. | `outbox.delivery_state = 'pending'` |
| DLQ | Dead-letter state for items that exhausted retry/reconciliation attempts. | `external_effects.status = 'dlq'` |
| External Effect | Durable record of outbound connector call lifecycle and reconciliation status. | `effect_id`, `call_id`, `status='unknown_outcome'` |
| Call ID | Stable idempotency identifier for provider-facing effect execution. | Sent as provider idempotency token/header. |
| Reconciler | Worker that resolves `unknown_outcome` effects by probing provider state. | Poll every 30s; escalate after 5 attempts. |
| Jobs Read Projection (`jobs_read`) | Read-optimised table updated in same transaction as `jobs`. | `GET /v1/jobs/{id}` reads from `jobs_read`. |
| Backoff Policy | Parameterised exponential delay strategy for retries. | `base_seconds=2`, `max_seconds=300` |
| Cooperative Cancellation | Cancellation model where runner observes `cancel_requested_at` between steps. | `POST /v1/jobs/{id}/cancel` while running. |
| Volta | Embedded multi-tenant encryption library for connector credentials. | `VaultService.UseSecret(...)` |
| KEK | Key Encryption Key derived per tenant from master passphrase. | KEK decrypts tenant DEK blob. |
| DEK | Data Encryption Key used to encrypt tenant secrets. | DEK encrypts `connector:github:token`. |
| Secrets Bootstrap | Startup retrieval of Volta master passphrase from AWS Secrets Manager. | `GetSecretValue(release-engine/volta/master-passphrase)` |
| SSRF Validation | Callback URL validation rejecting private/link-local/metadata endpoints. | Reject `http://169.254.169.254/...` |
| Metrics SQL | Durable SQL event stream for audits and long-term trend analysis. | `metrics_job_events` hypertable rows. |
| Metrics Exporter | Prometheus scrape endpoint for real-time counters/histograms. | `/metrics` endpoint scraped every 15s. |
| SLO | Service-level objective with target and measurement window. | Intake success `99.99% / 30 days`. |
| Canary Rollback | Automated or manual rollback triggered by canary SLI breach. | Error rate >1% for 15-minute window. |
