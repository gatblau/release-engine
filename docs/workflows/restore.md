# Restore Workflow

**Audience:** Ops

## Overview

Incident-driven service restore workflow. An AI agent locates the latest valid snapshot; the engine orchestrates a GitOps sequence to scale down writers, apply the restore job, scale services back up, and verify data integrity before reopening traffic.

## Purpose

What this workflow accomplishes: Automated service restore that locates the latest valid snapshot, orchestrates the full restore sequence, and verifies data integrity before reopening traffic.

## Rationale

Why this workflow exists: To reduce Recovery Time Objective (RTO) by eliminating manual restore steps during incident response and ensuring data integrity before traffic is reopened.

## Benefit

What value it delivers:
- Automated orchestration eliminates manual steps during high-pressure incidents
- AI-powered snapshot selection guarantees the best available restore point
- Traffic gate prevents reopening until data integrity is confirmed
- GitOps-driven sequence provides complete audit trail and easy rollback
- Deterministic process removes the chance of human error during incident response

## Value — TechOps as a Product

| Value Dimension | T-Shirt Size  | Notes |
|---|:-------------:|---|
| Speed at Scale |       M       | Restore is inherently sequential; scaling comes from automated orchestration, not parallelism. |
| Consistency & Reduced Risk |      XL       | Same restore sequence every time; integrity checks prevent partial or corrupted restores. |
| Governance Through Code |       M       | GitOps ensures every restore step is version-controlled and reviewable. |
| Developer Experience (DX) |       M       | Developers can trigger restores via Backstage; less reliance on on-call ops experts. |
| Clear Ownership / Fewer Hand-offs |       M       | Platform owns the restore automation; on-call engineers trigger, not execute manually. |

**Combined Value Score (Velocity 1):** 20/40 (M + XL + M + M + M = 3 + 8 + 3 + 3 + 3)

---

```mermaid
---
title: "Release Engine — Restore Workflow (ops/restore-service)"
---
sequenceDiagram
    autonumber
    actor Oncall as On-Call Engineer
    participant Backstage
    participant ReleaseEngine as Release Engine
    participant AIAgent as AI IT Ops Agent
    participant BackupSystem as Backup System
    participant InfraRepo as Infra Repo
    participant ArgoCD as Argo CD
    participant Kubernetes

    rect rgb(224, 242, 254)
        Note over Oncall,Backstage: Incident Trigger
        Oncall->>Backstage: request restore for service (service_id, target_env)
    end

    rect rgb(220, 252, 231)
        Note over Backstage,ReleaseEngine: Job Submission
        Backstage->>ReleaseEngine: submit job (idempotency_key, params, callback_url)
        ReleaseEngine-->>Backstage: 202 Accepted (job_id)
    end

    rect rgb(254, 243, 199)
        Note over ReleaseEngine: Internal Scheduling
        Note over ReleaseEngine: Scheduler claims job via SKIP LOCKED
        Note over ReleaseEngine: Runner acquires lease + run_id
    end

    rect rgb(243, 232, 255)
        Note over ReleaseEngine,AIAgent: Module Execution — RestoreServiceModule (snapshot selection)
        Note over ReleaseEngine: Delegate investigation to AI IT Ops Agent
        ReleaseEngine->>AIAgent: instruct agent — locate latest good snapshot
        AIAgent->>BackupSystem: query snapshots for service
        BackupSystem-->>AIAgent: snapshot list with integrity metadata
        AIAgent-->>ReleaseEngine: latest good snapshot confirmed (snapshot_id, timestamp)
    end

    rect rgb(255, 237, 213)
        Note over ReleaseEngine,Kubernetes: GitOps Restore Sequence — scale down writers
        Note over ReleaseEngine: Render scale-down manifest (replicas=0 for writers)
        ReleaseEngine->>InfraRepo: commit scale-down manifest (branch + PR)
        InfraRepo-->>ReleaseEngine: commit sha confirmed
        InfraRepo-->>ArgoCD: config repo change detected
        ArgoCD->>Kubernetes: apply scale-down — writers replicas set to 0
        Kubernetes-->>ArgoCD: scale-down healthy
        ArgoCD-->>InfraRepo: observed healthy
    end

    rect rgb(220, 252, 231)
        Note over ReleaseEngine,Kubernetes: GitOps Restore Sequence — apply restore job
        Note over ReleaseEngine: Render restore job manifest (snapshot_id, target PVC refs)
        ReleaseEngine->>InfraRepo: commit restore job manifest (branch + PR)
        InfraRepo-->>ReleaseEngine: commit sha confirmed
        InfraRepo-->>ArgoCD: config repo change detected
        ArgoCD->>Kubernetes: apply restore job
        Kubernetes-->>ArgoCD: job succeeded — restore complete
        ArgoCD-->>InfraRepo: observed healthy
        Note over ReleaseEngine: poll ArgoCD for observed-healthy — engine reads health from ArgoCD, not Kubernetes directly
        ReleaseEngine->>ArgoCD: poll app health status (restore job)
        ArgoCD-->>ReleaseEngine: observed healthy — restore job complete
    end

    rect rgb(243, 232, 255)
        Note over ReleaseEngine,Kubernetes: GitOps Restore Sequence — scale up service
        Note over ReleaseEngine: Render scale-up manifest (replicas restored to desired state)
        ReleaseEngine->>InfraRepo: commit scale-up manifest (branch + PR)
        InfraRepo-->>ReleaseEngine: commit sha confirmed
        InfraRepo-->>ArgoCD: config repo change detected
        ArgoCD->>Kubernetes: apply scale-up — service replicas restored
        Kubernetes-->>ArgoCD: pods ready and healthy
        ArgoCD-->>InfraRepo: observed healthy
    end

    rect rgb(255, 243, 199)
        Note over ReleaseEngine,AIAgent: Data Integrity Verification and Traffic Gate (inside module)
        Note over ReleaseEngine: Engine owns the traffic gate — deterministic polling loop against ArgoCD health
        ReleaseEngine->>ArgoCD: poll service health (error rate, readiness probes)
        ArgoCD-->>ReleaseEngine: service healthy and pods ready
        Note over ReleaseEngine: evaluate integrity result against policy threshold (deterministic)
        alt integrity probes pass policy threshold
            Note over ReleaseEngine: traffic gate open — service healthy
            ReleaseEngine->>AIAgent: report restore metrics for anomaly interpretation (optional)
            AIAgent-->>ReleaseEngine: no anomalies detected
        else integrity probes fail or timeout
            Note over ReleaseEngine: traffic gate remains closed — escalate
            ReleaseEngine->>AIAgent: analyse integrity failure — recommend next action
            AIAgent-->>ReleaseEngine: anomaly analysis and recommendation
        end
    end

    rect rgb(255, 228, 230)
        Note over ReleaseEngine,Oncall: Completion Callback
        ReleaseEngine->>Backstage: webhook callback (job complete, snapshot used, integrity status)
        Backstage-->>Oncall: restore confirmed — service visible and healthy in portal
    end
```