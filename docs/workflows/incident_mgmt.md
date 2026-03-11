# Incident Management

**Audience:** Dev, Ops

## Overview

AI-coordinated incident response system with a three-level hierarchy — coordinator overview, triage workflow, and five specialised child workflows (rollback, scale-out, config revert, failover, escalate). Each level is self-contained and traceable via parent job IDs.

## Purpose

What this workflow accomplishes: AI-coordinated incident response that automates triage, hypothesis scoring, and remediation dispatch across a hierarchy of specialised workflows.

## Rationale

Why this workflow exists: To reduce Mean Time to Recovery (MTTR) by automating the investigative and remediation phases of incident response while preserving human oversight through approval gates.

## Benefit

What value it delivers:
- Automated triage and hypothesis scoring shortens the time from alert to resolution
- Human approval gates ensure no automated action runs without consent
- Coordinator pattern intelligently dispatches the right child workflow based on alert type
- Full traceability with parent-child job relationships enables thorough post-mortem analysis
- Self-healing workflows (rollback, scale-out, config revert, failover) provide automated remediation for common patterns

## Value — TechOps as a Product

| Value Dimension | T-Shirt Size  | Notes |
|---|:-------------:|---|
| Speed at Scale |      XL       | Coordinates response across the entire estate; parallelises triage and remediation. |
| Consistency & Reduced Risk |      XL       | Same triage logic and remediation paths applied to every incident; no ad-hoc response. |
| Governance Through Code |       L       | Human approval gates, job chaining, and audit trails ensure governance without manual oversight. |
| Developer Experience (DX) |       M       | Developers receive notifications and approve actions; the platform handles the heavy lifting. |
| Clear Ownership / Fewer Hand-offs |      XL       | Clear separation between platform (automation) and product teams (approval); no ambiguity. |

**Combined Value Score (Velocity 1):** 32/40 (XL + XL + L + M + XL = 8 + 8 + 5 + 3 + 8)

---

# AI-Assisted Release and Incident Management

## Architecture and Workflow Reference

---

### Three levels

**Level 1 — Overview diagram**
Shows the coordinator pattern end to end. No internal details. Humans and LLMs use this to understand the full shape of the system.

**Level 2 — Per workflow diagram**
One sequence diagram per workflow. Shows the steps, participants, and handoffs for that workflow only. Each Level 2 workflow may dispatch child workflows — those are Level 3.

**Level 3 — Specialised child workflow diagram**
One sequence diagram per child workflow, dispatched by a Level 2 workflow. Self-contained. References its parent by job ID. Also used for module internals when branching logic is complex enough to warrant it.

---

### Summary

| Level | Format | Purpose |
|---|---|---|
| 1 | flowchart | System shape — coordinator routes to workflows |
| 2 | sequenceDiagram | Per workflow — steps, participants, handoffs, dispatches child jobs |
| 3 | sequenceDiagram or flowchart | Child workflows dispatched by Level 2, or module internals |

---

### Rules that make diagrams digestible by both humans and LLMs

**For humans**
- Colour bands group logical phases — detection, submission, execution, approval, dispatch
- Step numbers are sequential across the full diagram
- Title includes the workflow name as registered in the Release Engine
- Level and parent reference are explicit in every Level 3 title

**For LLMs**
- Each diagram is self-contained with explicit participant names
- Notes inside `rect` blocks name the phase — LLMs use these as section anchors
- Workflow names in titles match exactly what is registered in the module registry
- Parent and child job references are explicit — LLMs can trace the chain across diagrams without ambiguity
- Level is declared in the title so an LLM knows where in the hierarchy it is reading

---

## Level 1 — Coordinator Overview

```mermaid
---
title: "Level 1 — Incident Response System — Coordinator Pattern Overview"
---
flowchart TD
    Alert[Observability Alert]
    Coordinator[AI Coordinator Agent]
    Backstage[Backstage — Job Record]
    Triage[incident/triage-and-respond]
    Approval[Human Approval Gate]

    Rollback[incident/rollback-with-ai-assist]
    ScaleOut[incident/scale-out]
    ConfigRevert[incident/config-revert]
    Failover[incident/failover]
    Escalate[incident/notify-and-escalate]

    Alert --> Coordinator
    Coordinator --> Backstage
    Backstage --> Triage
    Triage --> Approval
    Approval -->|bad deploy detected| Rollback
    Approval -->|traffic spike| ScaleOut
    Approval -->|config drift| ConfigRevert
    Approval -->|regional failure| Failover
    Approval -->|no safe automation| Escalate
```

---

## Level 2 — Triage Workflow

Entry point for all incidents. Enriches the alert, scores hypotheses, gates on human approval, and dispatches one Level 3 child workflow.

```mermaid
---
title: "Level 2 — incident/triage-and-respond"
---
sequenceDiagram
    actor AIAgent as AI Coordinator Agent
    participant Backstage
    participant ReleaseEngine as Release Engine
    participant Observability
    participant GitHub
    actor OnCall as On-Call Engineer

    rect rgb(255, 228, 230)
        Note over AIAgent,Observability: Phase 1 — Detection
        Observability-->>AIAgent: 1. alert (service, error_rate, threshold)
    end

    rect rgb(224, 242, 254)
        Note over AIAgent,ReleaseEngine: Phase 2 — Job Submission
        AIAgent->>Backstage: 2. submit triage job (incident_ref, service, alert_payload)
        Backstage->>Backstage: 3. persist job record (job_id, status=pending)
        Backstage->>ReleaseEngine: 4. forward job (idempotency_key, params, callback_url)
        ReleaseEngine-->>Backstage: 5. 202 Accepted (job_id)
        Backstage-->>AIAgent: 6. job accepted (job_id)
    end

    rect rgb(243, 232, 255)
        Note over ReleaseEngine,GitHub: Phase 3 — Enrichment and Hypothesis Scoring
        ReleaseEngine->>Observability: 7. fetch metrics, logs, traces (time window)
        Observability-->>ReleaseEngine: 8. enriched signals
        ReleaseEngine->>GitHub: 9. fetch recent commits and deploy history
        GitHub-->>ReleaseEngine: 10. commit log and last deploy sha
        Note over ReleaseEngine: invoke AI agent with structured enriched context
        ReleaseEngine->>AIAgent: 11. score hypotheses (signals, commit log, deploy sha)
        AIAgent-->>ReleaseEngine: 12. hypothesis scores (type, confidence, proposed_workflow)
        Note over ReleaseEngine: Deterministic Validation Gate — engine validates AI output before proceeding
        Note over ReleaseEngine: check: confidence >= threshold, workflow name in registry, hypothesis type valid
        alt validation passes
            Note over ReleaseEngine: proceed with proposed workflow
        else confidence too low or workflow not in registry
            Note over ReleaseEngine: default to incident/notify-and-escalate (safe fallback)
        end
    end

    rect rgb(254, 243, 199)
        Note over ReleaseEngine,OnCall: Phase 4 — Human Approval Gate
        ReleaseEngine->>Backstage: 13. post diagnosis (hypothesis, confidence, proposed_workflow)
        Backstage->>OnCall: 14. notify — approval required
        OnCall-->>Backstage: 15. approved (or override with alternate workflow)
    end

    rect rgb(220, 252, 231)
        Note over Backstage,ReleaseEngine: Phase 5 — Dispatch Level 3 Child Workflow
        Backstage->>ReleaseEngine: 16. submit child job (approved_workflow, parent_job_id)
        ReleaseEngine-->>Backstage: 17. child job accepted (child_job_id)
        Backstage->>Backstage: 18. link child_job_id to parent triage job
        Backstage-->>AIAgent: 19. triage complete — child job dispatched (child_job_id, workflow)
    end
```

---

## Level 3 — Child Workflows

Each workflow below is dispatched by `incident/triage-and-respond`. Each is self-contained and references its parent via `parent_job_id`.

---

### incident/rollback-with-ai-assist

```mermaid
---
title: "Level 3 — incident/rollback-with-ai-assist — parent: incident/triage-and-respond"
---
sequenceDiagram
    participant Backstage
    participant ReleaseEngine as Release Engine
    participant GitHub
    participant ArgoCD as Argo CD
    participant Observability
    actor OnCall as On-Call Engineer

    rect rgb(224, 242, 254)
        Note over Backstage,ReleaseEngine: Phase 1 — Job Received
        Backstage->>ReleaseEngine: 1. child job (parent_job_id, target_service, bad_sha)
        ReleaseEngine-->>Backstage: 2. 202 Accepted (child_job_id)
    end

    rect rgb(243, 232, 255)
        Note over ReleaseEngine,GitHub: Phase 2 — Identify Safe Rollback Target
        ReleaseEngine->>GitHub: 3. fetch commit history for service
        GitHub-->>ReleaseEngine: 4. ordered commit log
        Note over ReleaseEngine: AI selects last known good sha
    end

    rect rgb(254, 243, 199)
        Note over ReleaseEngine,OnCall: Phase 3 — Confirm Rollback Target
        ReleaseEngine->>Backstage: 5. post rollback plan (bad_sha to good_sha, diff summary)
        Backstage->>OnCall: 6. notify — confirm rollback target
        OnCall-->>Backstage: 7. confirmed
    end

    rect rgb(255, 228, 230)
        Note over ReleaseEngine,ArgoCD: Phase 4 — Execute Rollback via Git Revert
        Note over ReleaseEngine: rollback is a Git revert commit — ArgoCD reconciles, preserving audit trail
        ReleaseEngine->>GitHub: 8. push revert commit (good_sha) to service repo
        GitHub-->>ReleaseEngine: 9. revert sha confirmed
        GitHub-->>ArgoCD: 10. repo change detected
        ArgoCD->>ArgoCD: 11. sync — apply reverted manifest
        ArgoCD-->>GitHub: 12. observed healthy
        ReleaseEngine->>ArgoCD: 13. poll app health status
        ArgoCD-->>ReleaseEngine: 14. healthy confirmed
    end

    rect rgb(220, 252, 231)
        Note over ReleaseEngine,Observability: Phase 5 — Verify and Close
        ReleaseEngine->>Observability: 15. poll health metrics (service, time window)
        Observability-->>ReleaseEngine: 16. metrics (error_rate, latency, saturation)
        Note over ReleaseEngine: AI evaluates — healthy or escalate
        ReleaseEngine->>Backstage: 17. post resolution (status, revert_sha, metrics snapshot)
        Backstage->>Backstage: 18. mark child_job_id resolved, update parent triage job
        Backstage->>OnCall: 19. notify — rollback complete (or escalation required)
    end
```

---

### incident/scale-out

```mermaid
---
title: "Level 3 — incident/scale-out — parent: incident/triage-and-respond"
---
sequenceDiagram
    participant Backstage
    participant ReleaseEngine as Release Engine
    participant InfraRepo as Infra Repo
    participant ArgoCD as Argo CD
    participant Kubernetes
    participant Observability
    actor OnCall as On-Call Engineer

    rect rgb(224, 242, 254)
        Note over Backstage,ReleaseEngine: Phase 1 — Job Received
        Backstage->>ReleaseEngine: 1. child job (parent_job_id, target_service, load_signal)
        ReleaseEngine-->>Backstage: 2. 202 Accepted (child_job_id)
    end

    rect rgb(243, 232, 255)
        Note over ReleaseEngine,Observability: Phase 2 — Load Analysis
        ReleaseEngine->>Observability: 3. fetch load metrics (cpu, rps, queue depth)
        Observability-->>ReleaseEngine: 4. current and projected load
        Note over ReleaseEngine: AI calculates target replica count
    end

    rect rgb(254, 243, 199)
        Note over ReleaseEngine,OnCall: Phase 3 — Approval Gate
        ReleaseEngine->>Backstage: 5. post scale plan (current_replicas to target_replicas)
        Backstage->>OnCall: 6. notify — approve scale-out
        OnCall-->>Backstage: 7. approved
    end

    rect rgb(255, 228, 230)
        Note over ReleaseEngine,ArgoCD: Phase 4 — Execute Scale-Out via GitOps
        Note over ReleaseEngine: scale-out is a Git commit — preserves audit trail and enables rollback via revert
        ReleaseEngine->>InfraRepo: 8. commit updated Deployment manifest (target_replicas)
        InfraRepo-->>ReleaseEngine: 9. commit sha confirmed
        InfraRepo-->>ArgoCD: 10. repo change detected
        ArgoCD->>Kubernetes: 11. apply updated replica count
        Kubernetes-->>ArgoCD: 12. pods ready and healthy
        ArgoCD-->>InfraRepo: 13. observed healthy
        ReleaseEngine->>ArgoCD: 14. poll app health status
        ArgoCD-->>ReleaseEngine: 15. healthy confirmed
    end

    rect rgb(220, 252, 231)
        Note over ReleaseEngine,Observability: Phase 5 — Verify and Close
        ReleaseEngine->>Observability: 16. poll health metrics (service, time window)
        Observability-->>ReleaseEngine: 17. metrics (cpu, rps, error_rate)
        Note over ReleaseEngine: AI evaluates — stable or escalate
        ReleaseEngine->>Backstage: 18. post resolution (status, replica_count, metrics snapshot)
        Backstage->>Backstage: 19. mark child_job_id resolved, update parent triage job
        Backstage->>OnCall: 20. notify — scale-out complete (or escalation required)
    end
```

---

### incident/config-revert

```mermaid
---
title: "Level 3 — incident/config-revert — parent: incident/triage-and-respond"
---
sequenceDiagram
    participant Backstage
    participant ReleaseEngine as Release Engine
    participant GitHub
    participant ArgoCD as Argo CD
    participant Kubernetes
    participant Observability
    actor OnCall as On-Call Engineer

    rect rgb(224, 242, 254)
        Note over Backstage,ReleaseEngine: Phase 1 — Job Received
        Backstage->>ReleaseEngine: 1. child job (parent_job_id, target_service, config_ref)
        ReleaseEngine-->>Backstage: 2. 202 Accepted (child_job_id)
    end

    rect rgb(243, 232, 255)
        Note over ReleaseEngine,GitHub: Phase 2 — Identify Config Drift
        ReleaseEngine->>GitHub: 3. fetch config change history
        GitHub-->>ReleaseEngine: 4. recent config commits and diffs
        Note over ReleaseEngine: AI identifies offending change
    end

    rect rgb(254, 243, 199)
        Note over ReleaseEngine,OnCall: Phase 3 — Approval Gate
        ReleaseEngine->>Backstage: 5. post revert plan (bad_config to previous_config, diff)
        Backstage->>OnCall: 6. notify — approve config revert
        OnCall-->>Backstage: 7. approved
    end

    rect rgb(255, 228, 230)
        Note over ReleaseEngine,ArgoCD: Phase 4 — Execute Revert via Git Revert Commit
        Note over ReleaseEngine: config revert is a Git revert commit — ArgoCD reconciles, no direct ConfigStore call
        ReleaseEngine->>GitHub: 8. push revert commit (previous_config) to config repo
        GitHub-->>ReleaseEngine: 9. revert sha confirmed
        GitHub-->>ArgoCD: 10. config repo change detected
        ArgoCD->>Kubernetes: 11. apply reverted config
        Kubernetes-->>ArgoCD: 12. config applied and healthy
        ArgoCD-->>GitHub: 13. observed healthy
        ReleaseEngine->>ArgoCD: 14. poll app health status
        ArgoCD-->>ReleaseEngine: 15. healthy confirmed
    end

    rect rgb(220, 252, 231)
        Note over ReleaseEngine,Observability: Phase 5 — Verify and Close
        ReleaseEngine->>Observability: 16. poll health metrics (service, time window)
        Observability-->>ReleaseEngine: 17. metrics snapshot
        Note over ReleaseEngine: AI evaluates — healthy or escalate
        ReleaseEngine->>Backstage: 18. post resolution (status, revert_sha, metrics snapshot)
        Backstage->>Backstage: 19. mark child_job_id resolved, update parent triage job
        Backstage->>OnCall: 20. notify — config revert complete (or escalation required)
    end
```

---

### incident/failover

```mermaid
---
title: "Level 3 — incident/failover — parent: incident/triage-and-respond"
---
sequenceDiagram
    participant Backstage
    participant ReleaseEngine as Release Engine
    participant Observability
    participant Kubernetes
    participant DNS
    actor OnCall as On-Call Engineer

    rect rgb(224, 242, 254)
        Note over Backstage,ReleaseEngine: Phase 1 — Job Received
        Backstage->>ReleaseEngine: 1. child job (parent_job_id, target_service, failed_region)
        ReleaseEngine-->>Backstage: 2. 202 Accepted (child_job_id)
    end

    rect rgb(243, 232, 255)
        Note over ReleaseEngine,Observability: Phase 2 — Region Health Assessment
        ReleaseEngine->>Observability: 3. fetch regional health signals
        Observability-->>ReleaseEngine: 4. region status (primary=degraded, secondary=healthy)
        Note over ReleaseEngine: AI confirms failover target region
    end

    rect rgb(254, 243, 199)
        Note over ReleaseEngine,OnCall: Phase 3 — Approval Gate
        ReleaseEngine->>Backstage: 5. post failover plan (primary to failover region)
        Backstage->>OnCall: 6. notify — approve failover
        OnCall-->>Backstage: 7. approved
    end

    rect rgb(255, 228, 230)
        Note over ReleaseEngine,DNS: Phase 4 — Execute Failover (imperative mode)
        Note over ReleaseEngine: Tradeoff: direct calls for RTO, reconciliation commit after
        ReleaseEngine->>Kubernetes: 8. scale up workloads in failover region
        Kubernetes-->>ReleaseEngine: 9. workloads ready
        ReleaseEngine->>DNS: 10. update routing to failover region
        DNS-->>ReleaseEngine: 11. routing updated
    end

    rect rgb(220, 252, 231)
        Note over ReleaseEngine,Observability: Phase 5 — Verify and Close
        ReleaseEngine->>Observability: 12. poll health metrics (failover region)
        Observability-->>ReleaseEngine: 13. metrics (error_rate, latency, saturation)
        Note over ReleaseEngine: AI evaluates — stable or escalate
        ReleaseEngine->>Backstage: 14. post resolution (status, active_region, metrics)
        Backstage->>Backstage: 15. mark child_job_id resolved, update parent job
        Backstage->>OnCall: 16. notify — failover complete, reconciliation commit required
    end
```

---

### incident/notify-and-escalate

```mermaid
---
title: "Level 3 — incident/notify-and-escalate — parent: incident/triage-and-respond"
---
sequenceDiagram
    participant Backstage
    participant ReleaseEngine as Release Engine
    participant PagerDuty
    participant Slack
    actor OnCall as On-Call Engineer
    actor IncidentCommander as Incident Commander

    rect rgb(224, 242, 254)
        Note over Backstage,ReleaseEngine: Phase 1 — Job Received
        Backstage->>ReleaseEngine: 1. child job (parent_job_id, diagnosis, reason=no_safe_automation)
        ReleaseEngine-->>Backstage: 2. 202 Accepted (child_job_id)
    end

    rect rgb(255, 228, 230)
        Note over ReleaseEngine,Slack: Phase 2 — Notify and Escalate
        ReleaseEngine->>PagerDuty: 3. trigger incident (severity, service, diagnosis_summary)
        PagerDuty-->>IncidentCommander: 4. page incident commander
        ReleaseEngine->>Slack: 5. post incident summary (diagnosis, triage_job_id, child_job_id)
        Slack-->>OnCall: 6. incident thread created
    end

    rect rgb(220, 252, 231)
        Note over ReleaseEngine,Backstage: Phase 3 — Record and Await
        ReleaseEngine->>Backstage: 7. post escalation record (status=escalated, pagerduty_incident_id)
        Backstage->>Backstage: 8. mark child_job_id escalated, update parent triage job
        Note over Backstage: job remains open until incident commander closes manually
    end
```

---

## Hierarchy reference

```
Level 1 — Coordinator Overview
  └── Level 2 — incident/triage-and-respond
        ├── Level 3 — incident/rollback-with-ai-assist
        ├── Level 3 — incident/scale-out
        ├── Level 3 — incident/config-revert
        ├── Level 3 — incident/failover
        └── Level 3 — incident/notify-and-escalate
```

Each Level 3 workflow is self-contained. The only shared state between levels is `parent_job_id` recorded in Backstage, which allows any human or LLM to trace the full chain from alert to resolution across all three levels.