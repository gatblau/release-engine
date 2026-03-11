<p style="text-align: right">
  <img src="docs/logo/re.png" alt="Release Engine Logo" width="200" />
</p>

# Release Engine

![Tests](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/gatblau/release-engine/main/.github/badges/tests.json&cacheSeconds=0)
![Coverage](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/gatblau/release-engine/main/.github/badges/coverage.json&cacheSeconds=0)
![Lint](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/gatblau/release-engine/main/.github/badges/lint.json&cacheSeconds=0)
![Security](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/gatblau/release-engine/main/.github/badges/security.json&cacheSeconds=0)

Release Engine is a durable, multi-tenant (modular-monolith) service designed to orchestrate asynchronous release and platform automation workflows. 

It provides idempotent job intake, policy-governed execution, connector-based integrations (e.g., GitHub/Backstage/AWS), and strong operational guarantees around retries, reconciliation, observability, and auditability.

## Documentation

### Specifications
- [Phase 1 — Analysis & Ambiguity Resolution](docs/spec/phase-1-analysis.md)
- [Phase 2 — Architecture Artefacts](docs/spec/phase-2-architecture.md)
- [Phase 3 — Components](docs/spec/phase-3-components.md)
- [Phase 4 — Cross-Cutting Concerns](docs/spec/phase-4-cross-cutting.md)
- [Phase 5 — Playbook](docs/spec/phase-5-playbook.md)
- [Phase 6 — Audit](docs/spec/phase-6-audit.md)

### Design Documents
- [Design Index & Strategic Context](docs/design/d00.md) — entry point to the design set, goals, and document map.
- [System Context & Component Architecture](docs/design/d01.md) — high-level system boundaries, major components, and architecture overview.
- [Job State Machine & Module Runtime Contract](docs/design/d02.md) — execution model, module interfaces, registry, and runtime behavior.
- [Scheduling, Claiming, and Idempotency](docs/design/d03.md) — scheduler mechanics, transaction behavior, and duplicate-prevention semantics.
- [Design Contracts, API Surface, and Data Model](docs/design/d04.md) — API contracts, extension points, and core PostgreSQL schema.
- [Lifecycle SQL Flows, Retries, and Recovery](docs/design/d05.md) — end-to-end lifecycle flows including fencing, cancellation, outbox, and lease recovery.
- [Scalability, SLOs, and Security/Compliance](docs/design/d06.md) — performance targets, fairness, capacity, and operational security controls.
- [Volta Secret Management & AWS Integration](docs/design/d07.md) — tenant secret encryption model, key hierarchy, and secure runtime secret usage.

### Prompts
- [Code Generation Prompt](docs/prompts/codegen.md)
- [Specification Generation Prompt](docs/prompts/specgen.md)