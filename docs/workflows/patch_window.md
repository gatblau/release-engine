# Patch Window Orchestration

**Audience:** Ops

## Overview

Automated node patching workflow with full change-management integration. Renders updated XR manifests for AMI/k8s version upgrades, gates on a Jira CHG approval, commits via GitOps, monitors SLIs during rollout, and auto-reverts on breach.

## Purpose

What this workflow accomplishes: Automated node patching that orchestrates the full patch lifecycle from approval through execution to verification, with automatic rollback on SLI breach.

## Rationale

Why this workflow exists: To transform high-risk, frequent node patching from a manual, error-prone task into a deterministic, auditable, self-healing process.

## Benefit

What value it delivers:
- Node patching becomes deterministic automation instead of manual, high-risk intervention
- Jira CHG approval gates ensure compliance with change management policy
- Automatic rollback preserves service stability if SLIs degrade during patching
- Full audit trail with Jira ticket, commit SHA, and SLI snapshots
- Zero manual intervention after approval — the workflow runs to completion autonomously

## Release Engine Capability Mapping

- **Approval model:** this workflow uses an **external** approval source (Jira CHG) rather than engine-native `waiting_approval`.
- **Recurrent jobs (optional):** patch windows can be submitted with `schedule` for pre-approved recurring maintenance windows.

## Value — TechOps as a Product

| Value Dimension | T-Shirt Size  | Notes |
|---|:-------------:|---|
| Speed at Scale |       L       | Parallel node patching across clusters; scales with cluster count, not manual effort. |
| Consistency & Reduced Risk |       L       | Same patch process every time; auto-rollback reduces blast radius. |
| Governance Through Code |      XL       | Jira integration + GitOps + SLI monitoring ensures change governance without manual oversight. |
| Developer Experience (DX) |       M       | Limited direct DX impact; primarily an ops workflow, but operators benefit from automation. |
| Clear Ownership / Fewer Hand-offs |       L       | Platform owns the automation; operators approve via Jira, not execute patches manually. |

**Combined Value Score (Velocity 1):** 26/40 (L + L + XL + M + L = 5 + 5 + 8 + 3 + 5)

---

```mermaid
---
title: "Release Engine — Patch Window Orchestration via GitOps (ops/patch-nodes)"
---
sequenceDiagram
    autonumber
    actor Operator
    participant Backstage
    participant ReleaseEngine as Release Engine
    participant Jira
    participant InfraRepo as Git Infra Repo
    participant ArgoCD as Argo CD
    participant Crossplane
    participant Observability

    rect rgb(224, 242, 254)
        Note over Operator,Backstage: User Interaction
        Operator->>Backstage: request patch window<br/>(cluster, node group, target AMI / k8s version, schedule)
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
        Note over ReleaseEngine,Observability: Module Execution — PatchNodesModule

        Note over ReleaseEngine: Deterministic manifest rendering<br/>inject AMI ID / k8s version into known XR template<br/>no coding agent required — pure param substitution

        ReleaseEngine->>Jira: create Change Request (CHG-XXXX)<br/>attach rendered diff as comment
        Jira-->>ReleaseEngine: CHG-XXXX status=Open

        ReleaseEngine->>Jira: poll — await Approved transition
        Jira-->>ReleaseEngine: CHG-XXXX status=Approved
        Note over ReleaseEngine: Jira CHG approval is the sole consent gate — no second Backstage approval required
        Note over ReleaseEngine: engine proceeds automatically on CHG Approved status

        ReleaseEngine->>Jira: transition to In Progress
        Jira-->>ReleaseEngine: CHG-XXXX status=In Progress

        ReleaseEngine->>InfraRepo: commit updated XR manifests<br/>(nodePool version, AMI ID, batch annotation)
        InfraRepo-->>ReleaseEngine: commit sha confirmed

        Note over InfraRepo,Crossplane: GitOps reconciliation — outside module
        InfraRepo-->>ArgoCD: repo change detected
        ArgoCD->>Crossplane: apply updated XRs
        Crossplane->>Crossplane: cordon, drain, patch and reboot nodes via managed resources

        ReleaseEngine->>Crossplane: poll XR Ready condition (read-only)
        Crossplane-->>ReleaseEngine: XR status response

        ReleaseEngine->>Observability: verify SLIs are normal (read-only)
        Observability-->>ReleaseEngine: SLI response

        alt SLIs within threshold and XR Ready
            ReleaseEngine->>Jira: add comment (patch summary, commit sha, nodes ready)
            ReleaseEngine->>Jira: transition to Done
            Jira-->>ReleaseEngine: CHG-XXXX status=Done
        else SLIs breached or XR stuck
            Note over ReleaseEngine: remediation — revert commit in Git<br/>Argo CD reconciles back to previous XR state
            ReleaseEngine->>InfraRepo: revert commit (previous XR manifest)
            InfraRepo-->>ReleaseEngine: revert sha confirmed
            ReleaseEngine->>Jira: add comment (breach details, revert sha)
            ReleaseEngine->>Jira: transition to Failed
            Jira-->>ReleaseEngine: CHG-XXXX status=Failed
        end
    end

    rect rgb(255, 228, 230)
        Note over ReleaseEngine,Operator: Completion Callback
        ReleaseEngine->>Backstage: webhook callback (job result, CHG-XXXX, commit sha)
        Backstage-->>Operator: patch result and Jira ticket visible in portal
    end
```