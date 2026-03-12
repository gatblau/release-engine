# Backup Workflow

**Audience:** Ops

## Overview

Automated, GitOps-driven backup orchestration for services running on Kubernetes. Triggered on-demand via Backstage or by schedule, this workflow quiesces writers, creates a validated volume snapshot, resumes writers, and enforces retention policy — all with full audit trail.

## Purpose

What this workflow accomplishes: Automated backup orchestration that eliminates manual, error-prone backup steps.

## Rationale

Why this workflow exists: To ensure backups are consistently executed, validated, and retained according to policy — without requiring manual intervention from on-call engineers.

## Benefit

What value it delivers:
- On-call engineers no longer execute error-prone, multi-step manual procedures
- AI-powered pre-flight checks guarantee snapshot usability
- Stale snapshots are pruned automatically, preventing storage bloat
- Every backup is recorded with full audit trail (timestamp, snapshot ID, integrity status)
- Developers can request backups via Backstage without filing tickets to TechOps

## Release Engine Capability Mapping

- **Recurrent jobs (primary mode):** backup runs can be submitted with `schedule` (cron expression). On successful completion, the job is re-queued for the next occurrence.
- **Human in the Loop (optional):** for production backups, an explicit approval step can be inserted before quiescing writers (`waiting_approval` → decision API).

## Value — TechOps as a Product

| Value Dimension | T-Shirt Size | Notes |
|---|:------------:|---|
| Speed at Scale |      M       | Backup orchestration is mostly sequential; scaling comes from self-service, not parallelism. |
| Consistency & Reduced Risk |      L       | Quiesce, snapshot, resume, validate — every backup follows the exact same safe sequence. |
| Governance Through Code |      M       | GitOps ensures all backup manifests are version-controlled and reviewable. |
| Developer Experience (DX) |      S       | Limited self-service value — backups are primarily an ops concern, not a daily dev workflow. |
| Clear Ownership / Fewer Hand-offs |      M       | TechOps owns the platform; developers consume backup capability without needing ops involvement. |

**Combined Value Score (Velocity 1):** 16/40 (M + L + M + S + M = 3 + 5 + 3 + 2 + 3)

---

```mermaid
---
title: "Release Engine — Backup Workflow (ops/backup-service)"
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
        Note over Oncall,Backstage: Backup Trigger
        Oncall->>Backstage: request backup for service (service_id, target_env, reason)
        Note over Backstage: trigger may also come from scheduled cron job
    end

    rect rgb(220, 252, 231)
        Note over Backstage,ReleaseEngine: Job Submission
        Backstage->>ReleaseEngine: submit job (idempotency_key, params, callback_url, schedule?)
        ReleaseEngine-->>Backstage: 202 Accepted (job_id)
    end

    rect rgb(254, 243, 199)
        Note over ReleaseEngine: Internal Scheduling
        Note over ReleaseEngine: Scheduler claims job via SKIP LOCKED
        Note over ReleaseEngine: Runner acquires lease + run_id
    end

    rect rgb(243, 232, 255)
        Note over ReleaseEngine,AIAgent: Module Execution — BackupServiceModule (pre-flight checks)
        ReleaseEngine->>AIAgent: assess service state before backup
        AIAgent->>Kubernetes: check service health and replication lag
        Kubernetes-->>AIAgent: service healthy — replication lag within threshold
        AIAgent-->>ReleaseEngine: pre-flight passed — safe to proceed with backup
        Note over ReleaseEngine: optional production guardrail — wait in `waiting_approval` before quiesce
    end

    rect rgb(255, 237, 213)
        Note over ReleaseEngine,Kubernetes: GitOps — quiesce writes (optional, low-risk window)
        Note over ReleaseEngine: Render quiesce annotation manifest (pause non-critical writers)
        ReleaseEngine->>InfraRepo: commit quiesce manifest (branch + PR)
        InfraRepo-->>ReleaseEngine: commit sha confirmed
        InfraRepo-->>ArgoCD: config repo change detected
        ArgoCD->>Kubernetes: apply quiesce annotation — writers paused
        Kubernetes-->>ArgoCD: quiesce applied
        ArgoCD-->>InfraRepo: observed healthy
    end

    rect rgb(220, 252, 231)
        Note over ReleaseEngine,Kubernetes: GitOps — create snapshot job
        Note over ReleaseEngine: Render snapshot job manifest (VolumeSnapshot or backup CRD)
        ReleaseEngine->>InfraRepo: commit snapshot job manifest (branch + PR)
        InfraRepo-->>ReleaseEngine: commit sha confirmed
        InfraRepo-->>ArgoCD: config repo change detected
        ArgoCD->>Kubernetes: apply snapshot job
        Kubernetes-->>ArgoCD: snapshot job running
        ArgoCD-->>InfraRepo: observed in progress
        ReleaseEngine->>ArgoCD: poll snapshot job health status
        alt snapshot job succeeds
            ArgoCD-->>ReleaseEngine: observed healthy — snapshot complete (snapshot_id, size, timestamp)
        else snapshot job fails or timeout
            Note over ReleaseEngine: snapshot failed — writers still paused — must resume before aborting
            ReleaseEngine->>InfraRepo: commit resume manifest (remove quiesce annotation — emergency resume)
            InfraRepo-->>ReleaseEngine: commit sha confirmed
            InfraRepo-->>ArgoCD: config repo change detected
            ArgoCD->>Kubernetes: apply resume — writers restarted
            Kubernetes-->>ArgoCD: writers healthy and accepting writes
            ArgoCD-->>InfraRepo: observed healthy
            ReleaseEngine->>Backstage: webhook callback (job failed, snapshot_id=none, reason=snapshot_job_failed)
        end
    end

    rect rgb(243, 232, 255)
        Note over ReleaseEngine,Kubernetes: GitOps — resume writers
        Note over ReleaseEngine: Render resume manifest (remove quiesce annotation)
        ReleaseEngine->>InfraRepo: commit resume manifest (branch + PR)
        InfraRepo-->>ReleaseEngine: commit sha confirmed
        InfraRepo-->>ArgoCD: config repo change detected
        ArgoCD->>Kubernetes: apply resume — writers restarted
        Kubernetes-->>ArgoCD: writers healthy and accepting writes
        ArgoCD-->>InfraRepo: observed healthy
    end

    rect rgb(255, 243, 199)
        Note over ReleaseEngine,AIAgent: Snapshot Validation and Metadata Registration
        ReleaseEngine->>AIAgent: validate snapshot integrity (snapshot_id)
        AIAgent->>BackupSystem: run integrity probe on snapshot
        BackupSystem-->>AIAgent: integrity check passed — snapshot valid
        AIAgent->>BackupSystem: register snapshot metadata (snapshot_id, timestamp, service, size, status=good)
        BackupSystem-->>AIAgent: metadata stored
        AIAgent-->>ReleaseEngine: snapshot validated and registered
    end

    rect rgb(220, 252, 231)
        Note over ReleaseEngine,AIAgent: Retention Policy Enforcement
        ReleaseEngine->>AIAgent: apply retention policy for service
        AIAgent->>BackupSystem: list snapshots beyond retention window
        BackupSystem-->>AIAgent: stale snapshot list
        AIAgent->>BackupSystem: delete stale snapshots
        BackupSystem-->>AIAgent: stale snapshots pruned
        AIAgent-->>ReleaseEngine: retention policy enforced
    end

    rect rgb(255, 228, 230)
        Note over ReleaseEngine,Oncall: Completion Callback
        ReleaseEngine->>Backstage: webhook callback (job complete, snapshot_id, integrity status, retention summary)
        Backstage-->>Oncall: backup confirmed — snapshot registered and available for restore
    end
```