# Cost and Capacity Optimisation

**Audience:** Dev, Ops

## Overview

AI-driven analysis of cloud cost and Kubernetes resource usage to generate right-sizing recommendations. Requires human approval before raising a PR with updated resource configs. Post-apply verification confirms no regression.

## Purpose

What this workflow accomplishes: Automated cost and capacity analysis that produces actionable right-sizing recommendations for Kubernetes workloads.

## Rationale

Why this workflow exists: To replace manual, spreadsheet-based cost reviews with a repeatable, scalable process that continuously aligns cloud spend with actual resource needs.

## Benefit

What value it delivers:
- Eliminates ad-hoc cost analysis with automated, consistent recommendations
- Prevents both over-provisioning (waste) and under-resourcing (risk) at scale
- Scales linearly without adding headcount as the number of services grows
- Human approval gates ensure operator intent is respected
- Post-change verification confirms no performance regression

## Value — TechOps as a Product

| Value Dimension | T-Shirt Size  | Notes |
|---|:-------------:|---|
| Speed at Scale |      XL       | Automated analysis eliminates manual effort entirely; scales to any number of services. |
| Consistency & Reduced Risk |       L       | Same analysis logic applied across all services; recommendations follow known patterns. |
| Governance Through Code |       L       | Approval gates and PR workflows ensure every cost change is reviewed and traceable. |
| Developer Experience (DX) |       M       | Developers see recommendations and can approve/reject via Backstage; self-service but not fully automated. |
| Clear Ownership / Fewer Hand-offs |       L       | TechOps defines the analysis logic; developers consume recommendations without ops tickets. |

**Combined Value Score (Velocity 1):** 26/40 (XL + L + L + M + L = 8 + 5 + 5 + 3 + 5)

---

```mermaid
---
title: "Release Engine — Cost and Capacity Optimisation Workflow (infra/cost-capacity-optimisation)"
---
sequenceDiagram
    autonumber
    actor Developer
    participant UI as Backstage / Conv. Agent
    participant ReleaseEngine as Release Engine
    participant AWSFinOps as AWS FinOps Connector
    participant AWSEKS as AWS EKS Connector
    participant DevOpsAgent as DevOps AI Agent
    participant ConfigRepo as Config Repo
    participant ArgoCD as Argo CD

    rect rgb(224, 242, 254)
        Note over Developer,UI: User Interaction
        Developer->>UI: 1. request cost and capacity optimisation for service
    end

    rect rgb(220, 252, 231)
        Note over UI,ReleaseEngine: Job Submission
        UI->>ReleaseEngine: 2. submit job (idempotency_key, service_ref, scope, callback_url)
        ReleaseEngine-->>UI: 3. 202 Accepted (job_id)
    end

    rect rgb(254, 243, 199)
        Note over ReleaseEngine: Internal Scheduling
        Note over ReleaseEngine: Scheduler claims job via SKIP LOCKED
        Note over ReleaseEngine: Runner acquires lease + run_id
    end

    rect rgb(243, 232, 255)
        Note over ReleaseEngine,DevOpsAgent: Module Execution — CostCapacityOptimisationModule

        Note over ReleaseEngine: Phase 1 — Data Collection (Release Engine owned)
        ReleaseEngine->>AWSFinOps: 4. fetch cost profile for service (tag-based)
        AWSFinOps-->>ReleaseEngine: 5. cost breakdown and trends returned
        ReleaseEngine->>AWSEKS: 6. fetch CPU, memory requests, limits and actuals
        AWSEKS-->>ReleaseEngine: 7. resource usage profile returned

        Note over ReleaseEngine: Phase 2 — Agent Reasoning
        ReleaseEngine->>DevOpsAgent: 8. invoke agent with structured context (cost profile, usage profile, service_ref)
        DevOpsAgent-->>ReleaseEngine: 9. recommendation returned (new requests, limits, autoscaling params, savings estimate, rationale)
    end

    rect rgb(255, 237, 213)
        Note over ReleaseEngine,UI: Human in the Loop — Review Gate
        ReleaseEngine->>UI: 10. create review task (recommendation, savings estimate, rationale, approve/reject)
        Note over UI: task visible in Backstage or conversational thread
        Developer->>UI: 11a. approve recommendation
        UI->>ReleaseEngine: 12a. approval confirmed (job_id)
    end

    rect rgb(220, 252, 231)
        Note over ReleaseEngine,ConfigRepo: Post-Approval — PR Creation and Merge (Release Engine owned)
        ReleaseEngine->>ConfigRepo: 13. open PR (updated requests, limits, HPA config, savings estimate in description)
        ConfigRepo-->>ReleaseEngine: 14. PR created (pr_url)
        ReleaseEngine->>UI: 15. notify developer (pr_url)
        ReleaseEngine->>ConfigRepo: 16. poll PR status and checks
        ConfigRepo-->>ReleaseEngine: 17. checks passed
        ReleaseEngine->>ConfigRepo: 18. enable auto-merge
        ConfigRepo-->>ReleaseEngine: 19. PR merged (merge_sha)
    end

    rect rgb(255, 237, 213)
        Note over ConfigRepo,ArgoCD: GitOps Reconciliation (outside module)
        ConfigRepo-->>ArgoCD: 20. merge detected — config change detected
        ArgoCD->>ArgoCD: 21. apply updated resource config
        ArgoCD-->>ConfigRepo: 22. observed healthy
    end

    rect rgb(254, 243, 199)
        Note over ReleaseEngine,AWSEKS: Post-Apply Verification (back in module — polling loop)
        Note over ReleaseEngine: wait for ArgoCD observed-healthy before polling actuals
        ReleaseEngine->>ArgoCD: 23. poll app health status
        ArgoCD-->>ReleaseEngine: 24. observed healthy confirmed
        ReleaseEngine->>AWSEKS: 25. poll post-rollout CPU and memory actuals
        AWSEKS-->>ReleaseEngine: 26. updated usage profile returned
        Note over ReleaseEngine: if regression detected — flag remediation task
    end

    rect rgb(255, 228, 230)
        Note over ReleaseEngine,Developer: Completion Callback
        ReleaseEngine->>UI: 27. webhook callback (job complete, pr_url, savings_estimate, post_rollout_status)
        UI-->>Developer: 28. optimisation summary visible in portal or conversation
    end

    rect rgb(254, 226, 226)
        Note over UI,ReleaseEngine: Rejection Path (alternative to step 11a)
        Developer->>UI: 11b. reject with feedback comment
        UI->>ReleaseEngine: 12b. rejection (job_id, feedback)
        ReleaseEngine->>DevOpsAgent: 12c. re-invoke agent with original context and reviewer feedback
        DevOpsAgent-->>ReleaseEngine: 12d. revised recommendation returned
        ReleaseEngine->>UI: 12e. create new review task (revised recommendation)
        Note over UI: loop continues until approval or job expires
    end
```