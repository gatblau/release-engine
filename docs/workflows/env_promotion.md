# Environment Promotion

**Audience:** Dev

## Overview

Automated multi-environment promotion pipeline (Dev → Staging → Production) driven via GitOps. Each stage raises a PR, waits for health confirmation from Argo CD, applies an approval gate, and promotes only when the previous environment is healthy.

## Purpose

What this workflow accomplishes: Automated multi-environment promotion pipeline that moves code from Dev through Staging to Production with approval gates and health checks at each stage.

## Rationale

Why this workflow exists: To eliminate ad-hoc, inconsistent deployments and replace them with a deterministic, auditable promotion process that ensures stability at each stage.

## Benefit

What value it delivers:
- No more manual deployments using inconsistent commands across teams
- Staging and Production require explicit approval before promotion
- Full traceability with commit, environment, approver, and health status
- Argo CD health checks verify stability before promotion proceeds
- GitOps-driven approach enables instant rollback via commit revert

## Release Engine Capability Mapping

- **Human in the Loop (engine-native):** staging and production gates map to `waiting_approval` steps and resume through the decisions API.
- **Recurrent jobs (optional):** promotion checks can be scheduled (for example nightly candidate promotion windows) via `schedule`.

## Value — TechOps as a Product

| Value Dimension | T-Shirt Size  | Notes |
|---|:-------------:|---|
| Speed at Scale |       L       | Automates the full promotion chain; scales to any number of services and environments. |
| Consistency & Reduced Risk |      XL       | Same promotion flow for every service; no environmental drift between stages. |
| Governance Through Code |      XL       | GitOps + approval gates ensure every promotion is auditable and controlled. |
| Developer Experience (DX) |       L       | Developers see promotion status in Backstage; approvals can be handled via PR or Backstage. |
| Clear Ownership / Fewer Hand-offs |       M       | Platform owns the promotion logic; developers trigger and approve, not execute. |

**Combined Value Score (Velocity 1):** 29/40 (L + XL + XL + L + M = 3 + 8 + 8 + 5 + 3)

---

```mermaid
---
title: "Release Engine — Environment Promotion Workflow (deployment/promote-service)"
---
sequenceDiagram
    actor Developer
    participant Backstage
    participant ReleaseEngine as Release Engine
    participant EnvConfigRepo as Env Config Repo
    participant ArgoDev as Argo Dev
    participant ArgoStg as Argo Stg
    participant ArgoProd as Argo Prod

    rect rgb(224, 242, 254)
        Note over Developer,Backstage: User Interaction
        Developer->>Backstage: 1. choose service, image tag and target environment
    end

    rect rgb(220, 252, 231)
        Note over Backstage,ReleaseEngine: Job Submission
        Backstage->>ReleaseEngine: 2. submit job (idempotency_key, params, callback_url, schedule?)
        ReleaseEngine-->>Backstage: 3. 202 Accepted (job_id)
    end

    rect rgb(254, 243, 199)
        Note over ReleaseEngine: Internal Scheduling
        Note over ReleaseEngine: Scheduler claims job via SKIP LOCKED
        Note over ReleaseEngine: Runner acquires lease + run_id
    end

    rect rgb(243, 232, 255)
        Note over ReleaseEngine,EnvConfigRepo: Module Execution — PromoteServiceModule (Dev)
        ReleaseEngine->>EnvConfigRepo: 4. raise PR to bump image tag on dev branch
        EnvConfigRepo-->>ReleaseEngine: 5. PR url confirmed
        Note over ReleaseEngine: await PR auto-merge or approval gate
        ReleaseEngine->>EnvConfigRepo: 6. merge PR to dev branch
        EnvConfigRepo-->>ReleaseEngine: 7. merge sha confirmed
    end

    rect rgb(255, 237, 213)
        Note over EnvConfigRepo,ArgoDev: GitOps Reconciliation — Dev (outside module)
        EnvConfigRepo-->>ArgoDev: 8. dev branch change detected
        ArgoDev->>ArgoDev: 9. sync dev apps
    end

    rect rgb(243, 232, 255)
        Note over ReleaseEngine,EnvConfigRepo: Module Execution — PromoteServiceModule (Stg)
        Note over ReleaseEngine: poll Argo Dev health — ready or timeout + remediation
        ReleaseEngine->>ArgoDev: 10. poll app health status
        ArgoDev-->>ReleaseEngine: 11. healthy confirmed
        ReleaseEngine->>EnvConfigRepo: 12. raise PR to promote to stg branch
        EnvConfigRepo-->>ReleaseEngine: 13. PR url confirmed
        ReleaseEngine->>Backstage: 14. create approval task (pr_url, target_env=stg, job_id)
        Note over ReleaseEngine: job step is parked in `waiting_approval` until decision is recorded
        Backstage->>Developer: 15. notify — approval required for stg promotion
        Developer->>Backstage: 16. approved
        Backstage->>ReleaseEngine: 17. approval confirmed (job_id, step_id)
        ReleaseEngine->>EnvConfigRepo: 18. merge PR to stg branch
        EnvConfigRepo-->>ReleaseEngine: 19. merge sha confirmed
    end

    rect rgb(255, 237, 213)
        Note over EnvConfigRepo,ArgoStg: GitOps Reconciliation — Stg (outside module)
        EnvConfigRepo-->>ArgoStg: 20. stg branch change detected
        ArgoStg->>ArgoStg: 21. sync stg apps
    end

    rect rgb(243, 232, 255)
        Note over ReleaseEngine,EnvConfigRepo: Module Execution — PromoteServiceModule (Prod)
        Note over ReleaseEngine: poll Argo Stg health — ready or timeout + remediation
        ReleaseEngine->>ArgoStg: 22. poll app health status
        ArgoStg-->>ReleaseEngine: 23. healthy confirmed
        ReleaseEngine->>EnvConfigRepo: 24. raise PR to promote to prod branch
        EnvConfigRepo-->>ReleaseEngine: 25. PR url confirmed
        ReleaseEngine->>Backstage: 26. create approval task (pr_url, target_env=prod, job_id)
        Note over ReleaseEngine: same `waiting_approval` gate for production promotion
        Backstage->>Developer: 27. notify — mandatory approval required for prod promotion
        Developer->>Backstage: 28. approved
        Backstage->>ReleaseEngine: 29. approval confirmed (job_id, step_id)
        ReleaseEngine->>EnvConfigRepo: 30. merge PR to prod branch
        EnvConfigRepo-->>ReleaseEngine: 31. merge sha confirmed
    end

    rect rgb(255, 237, 213)
        Note over EnvConfigRepo,ArgoProd: GitOps Reconciliation — Prod (outside module)
        EnvConfigRepo-->>ArgoProd: 32. prod branch change detected
        ArgoProd->>ArgoProd: 33. sync prod apps
    end

    rect rgb(254, 243, 199)
        Note over ReleaseEngine,ArgoProd: Final Health Verification
        ReleaseEngine->>ArgoProd: 34. poll app health status
        ArgoProd-->>ReleaseEngine: 35. healthy confirmed
        Note over ReleaseEngine: if not healthy — trigger rollback remediation
    end

    rect rgb(255, 228, 230)
        Note over ReleaseEngine,Developer: Completion Callback
        ReleaseEngine->>Backstage: 36. webhook callback (job complete, promotion status, env refs)
        Backstage-->>Developer: 37. deployment visible and healthy across all environments
    end
```