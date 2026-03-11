# Scaffold Service

**Audience:** Dev

## Overview

Golden Path workflow for creating new services. A developer selects a template in Backstage; the engine creates a GitHub repository with starter code and CI pre-configured, then registers the component in the Service Catalog — all in a single automated operation.

## Purpose

What this workflow accomplishes: Automated service scaffolding that creates a GitHub repository with starter code, CI pipelines, and Service Catalog registration from a single Backstage request.

## Rationale

Why this workflow exists: To eliminate inconsistency at birth — ensuring every new service starts with vetted defaults, compliant CI, and proper catalog registration rather than a blank slate.

## Benefit

What value it delivers:
- Every new service starts with the right CI, structure, and defaults — no snowflakes
- Developers spin up fully compliant services in minutes, not days
- New services are automatically registered in the Service Catalog for discoverability and observability
- Templates include linting, testing, security scanning, and deployment pipelines by default
- Developers create services without any hand-offs to TechOps or platform teams

## Value — TechOps as a Product

| Value Dimension | T-Shirt Size  | Notes |
|---|:-------------:|---|
| Speed at Scale |      XL       | New services can be created in minutes; scales to any number of teams. |
| Consistency & Reduced Risk |      XL       | Every service starts from a vetted template; no snowflakes or missing defaults. |
| Governance Through Code |       L       | Templates are version-controlled; changes propagate to all new services. |
| Developer Experience (DX) |      XL       | Developers create services from Backstage with one click; immediate productivity. |
| Clear Ownership / Fewer Hand-offs |       L       | Platform owns templates; developers consume self-service; no ticket required. |

**Combined Value Score (Velocity 1):** 34/40 (XL + XL + L + XL + L = 8 + 8 + 5 + 8 + 5)

---

```mermaid
---
title: "Release Engine — Scaffold Service Workflow (scaffolding/create-service)"
---
sequenceDiagram
    actor Developer
    participant Backstage
    participant ReleaseEngine as Release Engine
    participant GitHub
    participant ServiceCatalog as Service Catalog

    rect rgb(224, 242, 254)
        Note over Developer,Backstage: User Interaction
        Developer->>Backstage: 1. choose template and params
    end

    rect rgb(220, 252, 231)
        Note over Backstage,ReleaseEngine: Job Submission
        Backstage->>ReleaseEngine: 2. submit job (idempotency_key, params, callback_url)
        ReleaseEngine-->>Backstage: 3. 202 Accepted (job_id)
    end

    rect rgb(254, 243, 199)
        Note over ReleaseEngine: Internal Scheduling
        Note over ReleaseEngine: Scheduler claims job via SKIP LOCKED
        Note over ReleaseEngine: Runner acquires lease + run_id
    end

    rect rgb(243, 232, 255)
        Note over ReleaseEngine,ServiceCatalog: Module Execution — ScaffoldServiceModule
        ReleaseEngine->>GitHub: 4. create repository with starter code and CI
        alt repository creation fails
            GitHub-->>ReleaseEngine: 5a. creation failed (reason)
            ReleaseEngine->>Backstage: 5b. webhook callback (job failed, reason=repo_creation_failed)
            Backstage-->>Developer: 5c. notify — repository creation failed, no resources created
        else repository created successfully
            GitHub-->>ReleaseEngine: 5. repository created (repo_url)
            ReleaseEngine->>ServiceCatalog: 6. register component yaml
            alt catalog registration fails
                ServiceCatalog-->>ReleaseEngine: 7a. registration failed (reason)
                Note over ReleaseEngine: orphaned repo risk — delete repo to prevent dangling resource
                ReleaseEngine->>GitHub: 7b. delete repository (repo_url)
                GitHub-->>ReleaseEngine: 7c. repository deleted
                ReleaseEngine->>Backstage: 7d. webhook callback (job failed, reason=catalog_registration_failed)
                Backstage-->>Developer: 7e. notify — scaffold failed and repo cleaned up
            else catalog registration succeeds
                ServiceCatalog-->>ReleaseEngine: 7. registration confirmed
            end
        end
    end

    rect rgb(255, 228, 230)
        Note over ReleaseEngine,Developer: Completion Callback
        ReleaseEngine->>Backstage: 8. webhook callback (job complete, repo_url)
        Backstage-->>Developer: 9. service visible in portal with catalog entry and CI pre-configured
    end
```
