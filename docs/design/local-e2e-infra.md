# Local End-to-End (E2E) Infrastructure for Release Engine

## Context

The Release Engine is a durable orchestration system with:

- PostgreSQL persistence (`jobs`, `jobs_read`, `outbox`)
- PgBouncer transaction pooling
- OIDC/JWT-based authentication
- External connectors (Git, cloud providers, etc.)
- Callback delivery via outbox pattern
- Crossplane integration for infrastructure composition

To properly validate system behavior, we need a **local environment** that:

1. Tests true end-to-end execution paths
2. Exposes side effects (DB, Git, callbacks)
3. Validates Crossplane XRDs and Compositions
4. Provides a safe learning/debugging platform

Pure mocks are insufficient because they do not exercise real boundaries like:
- transactional DB guarantees
- OIDC validation
- Git protocol behavior
- Kubernetes reconciliation

---

## Decision

We will implement a **hybrid local environment** consisting of:

### Docker Compose Layer
- PostgreSQL
- PgBouncer
- MinIO (S3-compatible storage)
- Dex (OIDC provider)
- Gitea (Git service)
- Release Engine
- Callback sink (custom lightweight service)

### Kubernetes Layer (kind)
- kind cluster
- Crossplane core
- XRDs and Compositions under test
- Optional composition functions / providers

### Go Test Harness
A dedicated CLI tool (`re-harness`) to:
- bootstrap the environment
- seed test data
- run E2E tests
- inspect outputs
- tear down or preserve state

---

## Architecture Overview

```
Go Harness
   |
   +--> Docker Compose
   |       - Postgres + PgBouncer
   |       - MinIO
   |       - Dex
   |       - Gitea
   |       - Release Engine
   |       - Callback Sink
   |
   +--> kind Cluster
           - Crossplane
           - XRDs / Compositions
           - Composite Resources
```

---

## Design Details

### 1. Database Layer

- PostgreSQL is the system of record
- PgBouncer is required to reflect production behavior
- Tests will directly inspect:
  - `jobs`
  - `jobs_read`
  - `outbox`

---

### 2. Authentication (OIDC)

- Dex will act as the OIDC provider
- JWT tokens used to call Release Engine APIs
- Pre-seeded users:
  - platform-admin
  - tenant-operator
  - tenant-reader
  - approver

---

### 3. Object Storage

- MinIO used as S3-compatible storage
- Initial mode:
  - `VOLTA_ENV=dev`
  - `VOLTA_STORAGE=file`
- Future option:
  - switch to S3-compatible vault backend

---

### 4. Git Backend

- Gitea provides:
  - HTTP/SSH Git access
  - UI for inspection
- Seeded repositories:
  - platform-config
  - service-catalog
  - infra-live

---

### 5. Crossplane

Two validation modes:

#### Offline
- `crossplane beta validate`
- `crossplane render`

#### Live
- Apply XRDs and Compositions to kind cluster
- Create Composite Resources (XR)
- Observe reconciliation and outputs

---

### 6. Callback System

A lightweight callback sink will:
- capture webhook payloads
- simulate success/failure responses
- allow assertions on retry behavior

---

### 7. Go Harness CLI

Command structure:

```
re-harness up
re-harness seed
re-harness run e2e
re-harness inspect
re-harness down
```

#### Responsibilities

**up**
- start Compose services
- create kind cluster
- install Crossplane
- wait for readiness

**seed**
- configure Dex, Gitea, MinIO
- apply XRDs/Compositions
- create test fixtures

**run e2e**
- execute API flows
- validate DB, Git, callbacks, Crossplane

**inspect**
- dump system state for debugging

**down**
- teardown or preserve environment

---

## Test Scenarios

### Core Flows

- Job submission (happy path)
- Idempotency replay
- Idempotency conflict
- Job cancellation
- Approval workflow

### Side Effects

- Database state validation
- Git repository changes
- Callback delivery + retries

### Crossplane

- Offline schema validation
- Live reconciliation in cluster

---

## Environment Modes

### Developer Mode
- Full stack
- Persistent state
- Debug-friendly

### CI Mode
- Ephemeral
- Deterministic
- Automated validation

### Advanced Mode (Future)
- S3-backed secrets
- AWS emulation if needed

---

## Alternatives Considered

### 1. Fully Mocked Environment
Rejected:
- does not validate real system guarantees

### 2. Docker Compose Only
Rejected:
- cannot properly validate Crossplane reconciliation

### 3. Full Cloud Deployment
Rejected:
- too heavy for local development
- slow feedback loop

---

## Consequences

### Positive

- High-fidelity testing
- Realistic failure modes
- Improved developer understanding
- Direct inspection of system state

### Negative

- Increased setup complexity
- Higher resource usage
- Longer startup time

---

## Risks and Mitigations

### Complexity
Mitigation:
- provide profiles (fast vs full)

### Crossplane provider overhead
Mitigation:
- use lightweight or Kubernetes-native providers

### Secrets handling differences
Mitigation:
- start with local file mode, evolve later

---

## Implementation Plan

### Phase 1
- Core services (Postgres, Dex, Gitea)
- Go harness
- Basic E2E tests
- Offline Crossplane validation

### Phase 2
- Add MinIO
- Add kind + Crossplane
- Live reconciliation tests

### Phase 3
- Advanced workflows (approvals, retries)
- Failure injection
- Multi-tenant scenarios

---

## Notes

- NATS is present in configuration but not clearly required by design → treat as optional
- Prefer real boundaries over mocks
- Optimize for learning and observability

---

## Decision Summary

To build a **hybrid Docker Compose + kind + Go harness environment** to achieve realistic, inspectable, and reproducible end-to-end testing of the Release Engine infrastructure module.
