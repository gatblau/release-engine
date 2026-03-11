# Phase 5 — Generation Playbook

## 0. Project Scaffolding

- [ ] Initialise repository and module metadata.
- [ ] Configure Atlas migration directory and baseline schema.
- [ ] Create folder structure under `cmd/`, `internal/`, `docs/spec/`, and `migrations/`.
- [ ] Add `.env.example` with all required variables from Phase 2D.

## 1. Foundation Components (Phase 0)

1. [ ] Implement **ConfigLoader**  
       Spec: `SPEC: ConfigLoader`  
       Verify: startup fails with explicit `CONFIG_MISSING` for missing required variable.

2. [ ] Implement **LoggerFactory**  
       Spec: `SPEC: LoggerFactory`  
       Verify: JSON logs contain `service`, `component`, and `request_id` when present.

3. [ ] Implement **DBPool**  
       Spec: `SPEC: DBPool`  
       Verify: `Ping()` succeeds and isolation check returns `read committed`.

4. [ ] Implement **MigrationChecker**  
       Spec: `SPEC: MigrationChecker`  
       Verify: readiness fails when schema version is behind expected version.

5. [ ] Implement **HTTPServer**  
       Spec: `SPEC: HTTPServer`  
       Verify: server starts with configured timeouts and request-size limit.

## 2. API Control Plane (Phase 1)

6. [ ] Implement **AuthMiddleware** and JWT validation pipeline.
7. [ ] Implement **RateLimiter** (token bucket per tenant).
8. [ ] Implement **PolicyEngine** (Go-native evaluator, cache bypass for `job:cancel`, and `EvaluateApproval` guardrails for role, self-approval, tenant scope, and optional budget authority).
9. [ ] Implement **IdempotencyService** with deterministic intake transaction.
10. [ ] Implement **HealthHandler** (`/healthz`, `/readyz`).
11. [ ] Implement **JobsAPIHandler** routes: `POST /v1/jobs`, `GET /v1/jobs/{id}`, `POST /v1/jobs/{id}/cancel`, `POST /v1/jobs/{job_id}/steps/{step_id}/decisions`, `GET /v1/jobs/{job_id}/steps/{step_id}/approval-context`, and pending-approval query via `GET /v1/jobs?step_status=waiting_approval`.

## 3. Runtime Execution Plane (Phase 1–2)

12. [ ] Implement **ModuleRegistry** and **ConnectorRegistry** with startup registration.
13. [ ] Implement **LeaseManager** fenced write helpers.
14. [ ] Implement **SchedulerService** with DWRR fairness and SKIP LOCKED claiming.
15. [ ] Implement **StepAPIAdapter** for durable step/effect/context operations.
16. [ ] Implement **RunnerService** job execution and fenced finalisation.
17. [ ] Implement **ReconcilerService** unknown-outcome resolution and DLQ escalation.

## 4. Secrets and Outbound Delivery (Phase 2)

18. [ ] Implement **VoltaManager** bootstrap from AWS Secrets Manager and S3-backed vaults.
19. [ ] Implement **CallbackSigner** dual-key HMAC signing.
20. [ ] Implement **OutboxDispatcher** retry, timeout, and DLQ workflow.

## 5. Observability and Audit (Phase 2)

21. [ ] Implement **MetricsExporter** and expose `/metrics`.
22. [ ] Implement **MetricsSQLWriter** for immutable SQL event stream.
23. [ ] Implement **TracingService** OTLP exporter and sampling policy.
24. [ ] Implement **AuditService** immutable audit log persistence.

## 6. Integration and Verification

- [ ] Run unit tests: `go test ./... -v -race -count=1`
- [ ] Run SQL and migration checks: `atlas migrate lint --dir file://migrations`
- [ ] Run integration suite against PostgreSQL, PgBouncer, NATS, and mocked providers.
- [ ] Execute critical smoke tests:
  1. Deterministic idempotent intake and replay.
  2. Claim/execute/fence with forced lease loss.
  3. Outbox delivery retries and DLQ promotion.
  4. Unknown-outcome reconciliation to succeeded/failed/dlq.
  5. Volta secret bootstrap and scoped UseSecret decryption.
- [ ] Verify dashboards and alerts for claim latency, fenced conflicts, outbox DLQ, and effect DLQ.
- [ ] Run security and vulnerability checks: `gosec ./...` and `govulncheck ./...`.
- [ ] Run linter: `golangci-lint run ./...`.

## 7. Approval Lifecycle (Phase 6)

25. [ ] Implement **ApprovalWorker** lifecycle loop (`internal/transport/http/approval_worker.go`).
        Verify: worker ticks every 30 seconds by default and can be configured.

26. [ ] Implement TTL persistence in **StepAPIAdapter.WaitForApproval** using `approval_ttl` and `approval_expires_at`.
        Verify: step records include TTL and computed expiry timestamp.

27. [ ] Implement escalation and expiry transitions in **ApprovalService** / worker integration.
        Verify: emit `approval_escalated` at threshold and `approval_expired` at timeout with `approval_timeout` reason.

28. [ ] Add expiry decision recording.
        Verify: system decision (`decision=expired`, `approver=system`) is inserted exactly once per expired step.

29. [ ] Add and run tests for escalation timing and expiry handling.
        Verify: tests cover pre-threshold, threshold crossing, and post-expiry transitions.

## 8. Outbox Events (Phase 7)

30. [ ] Register approval event types in **OutboxDispatcher**.
        Verify: dispatcher registers `approval_requested`, `approval_decided`, `approval_escalated`, `approval_expired` at startup.

31. [ ] Enforce dispatcher event-type contract.
        Verify: unregistered event types are rejected and not queued.

32. [ ] Emit approval lifecycle outbox events from **ApprovalService**.
        Verify: `approval_requested` on wait entry and `approval_decided` on accepted decision.

33. [ ] Emit timeout/escalation events from **ApprovalWorker** integration.
        Verify: `approval_escalated` and `approval_expired` are emitted with required payload fields.

34. [ ] Add and run outbox emission tests.
        Verify: coverage includes event registration, accepted emissions, and rejection of unknown event types.

## 9. Metrics and Observability (Phase 8)

35. [ ] Register approval lifecycle metrics in **MetricsExporter**.
        Verify: exporter registers `re_approval_requests_total`, `re_approval_decisions_total`, `re_approval_latency_seconds`, `re_approval_escalations_total`, `re_approval_timeouts_total`, and `re_approval_worker_tick_duration_seconds`.

36. [ ] Instrument **ApprovalService** with approval telemetry hooks.
        Verify: request, decision, latency, escalation, and timeout metrics are emitted at lifecycle transition points.

37. [ ] Instrument **ApprovalWorker** tick loop with duration telemetry.
        Verify: each worker tick records `re_approval_worker_tick_duration_seconds{status}`.

38. [ ] Add and run observability-focused tests for approval metrics.
        Verify: unit tests cover collector registration and metric emission from service/worker paths.
