# Infrastructure Provisioning

**Audience:** Dev

## Overview

Self-service infrastructure provisioning via a Backstage template → Release Engine → Crossplane GitOps pipeline. Developers choose a template; the engine commits XR manifests; Argo CD reconciles; Crossplane provisions against Cloud APIs. Health is verified before completion.

## Purpose

What this workflow accomplishes: Self-service infrastructure provisioning that allows developers to provision cloud resources from vetted templates without manual ops involvement.

## Rationale

Why this workflow exists: To deliver the "Golden Path" for infrastructure — a pre-defined, safe, compliant way to provision resources that enforces architectural standards by default.

## Benefit

What value it delivers:
- Every provisioning request follows a pre-defined, safe path defined by TechOps
- No more tickets to TechOps for bespoke infrastructure requests
- Crossplane Compositions embed security, networking, and tagging policies automatically
- Any developer can provision infrastructure in minutes without waiting for an ops engineer
- GitOps ensures every change is version-controlled, reviewed, and traceable

## Value — TechOps as a Product

| Value Dimension | T-Shirt Size  | Notes |
|---|:-------------:|---|
| Speed at Scale |      XL       | Self-service eliminates queue time; provisioning happens in minutes, not days. |
| Consistency & Reduced Risk |      XL       | Every resource is provisioned from the same Compositions; no snowflakes. |
| Governance Through Code |      XL       | Policy-as-code in Compositions enforces compliance before resources are created. |
| Developer Experience (DX) |      XL       | Developers provision what they need from Backstage without engaging TechOps. |
| Clear Ownership / Fewer Hand-offs |      XL       | Platform owns the Compositions; developers consume self-service; clear boundary. |

**Combined Value Score (Velocity 1):** 40/40 (XL + XL + XL + XL + XL = 8 + 8 + 8 + 8 + 8)

---

```mermaid
---
title: "Release Engine — Infrastructure Provisioning Workflow (infra/provision-crossplane)"
---
sequenceDiagram
    actor Developer
    participant Backstage
    participant ReleaseEngine as Release Engine
    participant InfraRepo as Infra Repo
    participant ArgoCD as Argo CD
    participant Crossplane
    participant CloudAPIs as Cloud APIs

    rect rgb(224, 242, 254)
        Note over Developer,Backstage: User Interaction
        Developer->>Backstage: 1. choose infra template and params
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
        Note over ReleaseEngine,InfraRepo: Module Execution — ProvisionInfraModule
        Note over ReleaseEngine: Render Crossplane XR and Composition refs from params
        ReleaseEngine->>InfraRepo: 4. commit XR manifests and composition refs (branch + PR)
        InfraRepo-->>ReleaseEngine: 5. commit sha confirmed
    end

    rect rgb(255, 237, 213)
        Note over InfraRepo,CloudAPIs: GitOps Reconciliation (outside module — driven by Argo CD)
        InfraRepo-->>ArgoCD: 6. config repo change detected
        ArgoCD->>Crossplane: 7. apply CRs and XRs
        Crossplane->>CloudAPIs: 8. create or update resources
        CloudAPIs-->>Crossplane: 9. provisioned ok
        Crossplane-->>ArgoCD: 10. ready status
        ArgoCD-->>InfraRepo: 11. observed healthy
    end

    rect rgb(254, 243, 199)
        Note over ReleaseEngine,Crossplane: Module Health Verification (back in module — polling loop with timeout)
        ReleaseEngine->>Crossplane: 12. poll XR status (Ready condition)
        Crossplane-->>ReleaseEngine: 13. XR status response
        Note over ReleaseEngine: if not Ready within threshold → remediation
        ReleaseEngine->>Crossplane: 14. trigger remediation (re-apply or alert)
        Crossplane-->>ReleaseEngine: 15. remediation acknowledged
        ReleaseEngine->>CloudAPIs: 16. optional — verify resource state via Cloud API
        CloudAPIs-->>ReleaseEngine: 17. resource state confirmed
    end

    rect rgb(255, 228, 230)
        Note over ReleaseEngine,Developer: Completion Callback
        ReleaseEngine->>Backstage: 18. webhook callback (job complete, infra status, resource refs)
        Backstage-->>Developer: 19. infrastructure visible and healthy in portal
    end
```