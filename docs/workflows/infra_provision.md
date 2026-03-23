# Infrastructure Provisioning Module — Implementation Design

## Overview

Self-service infrastructure provisioning via a Backstage template → Release Engine → Crossplane GitOps pipeline. Developers choose a template; the engine commits XR manifests; Argo CD reconciles; Crossplane provisions against Cloud APIs. Health is verified before completion.

## Purpose

Self-service infrastructure provisioning that allows developers to provision cloud resources from vetted templates without manual ops involvement.

## Rationale

To deliver the "Golden Path" for infrastructure — a pre-defined, safe, compliant way to provision resources that enforces architectural standards by default.

## Benefit

- Every provisioning request follows a pre-defined, safe path defined by TechOps
- No more tickets to TechOps for bespoke infrastructure requests
- Crossplane Compositions embed security, networking, and tagging policies automatically
- Any developer can provision infrastructure in minutes without waiting for an ops engineer
- GitOps ensures every change is version-controlled, reviewed, and traceable

## Release Engine Capability Mapping

- **Human in the Loop (optional):** for high-blast-radius templates, insert an explicit `waiting_approval` step before committing manifests.
- **Recurrent jobs (optional):** generally on-demand, but can run with `schedule` for periodic drift-probe or reconciliation workflows.

## Value — TechOps as a Product

| Value Dimension | T-Shirt Size | Notes |
|---|:---:|---|
| Speed at Scale | XL | Self-service eliminates queue time; provisioning happens in minutes, not days. |
| Consistency & Reduced Risk | XL | Every resource is provisioned from the same Compositions; no snowflakes. |
| Governance Through Code | XL | Policy-as-code in Compositions enforces compliance before resources are created. |
| Developer Experience (DX) | XL | Developers provision what they need from Backstage without engaging TechOps. |
| Clear Ownership / Fewer Hand-offs | XL | Platform owns the Compositions; developers consume self-service; clear boundary. |

**Combined Value Score (Velocity 1):** 40/40

---

## Workflow Sequence

```mermaid
sequenceDiagram
    actor Developer
    participant Backstage
    participant ReleaseEngine as Release Engine
    participant OPA as Policy Engine
    participant Approver
    participant InfraRepo as Infra Repo (main)
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
        Note over ReleaseEngine,OPA: Module Execution — Render + Policy Check
        Note over ReleaseEngine: Render Crossplane XR manifests from params
        ReleaseEngine->>OPA: 4. evaluate rendered manifests against policies
        OPA-->>ReleaseEngine: 5. policy decision (allow / deny + reasons)
        Note over ReleaseEngine: if denied → job terminated with policy violation
    end

    rect rgb(255, 228, 230)
        Note over ReleaseEngine,Approver: Approval Gate (d09 State Machine)
        ReleaseEngine->>Backstage: 6. approval request (structured summary, resource list, cost estimate)
        Backstage->>Approver: 7. approval prompt in portal
        Note over ReleaseEngine: job state → AWAITING_APPROVAL
        Note over ReleaseEngine: timeout clock starts (configurable per tenant)
        alt Approved
            Approver->>Backstage: 8a. APPROVE
            Backstage->>ReleaseEngine: 8b. approval callback
        else Rejected
            Approver->>Backstage: 8a. REJECT with reason
            Backstage->>ReleaseEngine: 8b. rejection callback
            Note over ReleaseEngine: job → TERMINATED (rejected)
        else Timeout
            Note over ReleaseEngine: escalation policy triggers
        end
    end

    rect rgb(220, 252, 231)
        Note over ReleaseEngine,InfraRepo: Commit to Main
        ReleaseEngine->>InfraRepo: 9. commit XR manifests to main
        InfraRepo-->>ReleaseEngine: 10. commit_sha confirmed
    end

    rect rgb(255, 237, 213)
        Note over InfraRepo,CloudAPIs: GitOps Reconciliation (ArgoCD-driven)
        InfraRepo-->>ArgoCD: 11. change detected on main
        ArgoCD->>Crossplane: 12. apply CRs and XRs
        Crossplane->>CloudAPIs: 13. create or update resources
        CloudAPIs-->>Crossplane: 14. provisioned ok
        Crossplane-->>ArgoCD: 15. ready status
    end

    rect rgb(254, 243, 199)
        Note over ReleaseEngine,CloudAPIs: Health Verification
        ReleaseEngine->>InfraRepo: 16. poll ArgoCD Application status via git connector (health annotation)
        InfraRepo-->>ReleaseEngine: 17. health status
        Note over ReleaseEngine: if not healthy within threshold → remediation
        ReleaseEngine->>InfraRepo: 18. recommit or annotate for force-sync
        InfraRepo-->>ReleaseEngine: 19. commit_sha confirmed
    end

    rect rgb(220, 252, 231)
        Note over ReleaseEngine,Developer: Completion Callback
        ReleaseEngine->>Backstage: 20. webhook callback (job complete, infra status, resource refs)
        Backstage-->>Developer: 21. infrastructure visible and healthy in portal
    end
```

## Failure Handling

```mermaid
sequenceDiagram
  participant RE as Release Engine (infra/provision-crossplane)
  participant InfraRepo as Infra Repo
  participant ArgoCD as Argo CD
  participant Crossplane
  participant CloudAPIs as AWS

  RE->>RE: step state → RUNNING
  RE->>InfraRepo: commit XR manifests
  InfraRepo-->>ArgoCD: change detected
  ArgoCD->>Crossplane: apply CRs
  Crossplane->>CloudAPIs: create resources
  CloudAPIs-->>Crossplane: ❌ failure
  Crossplane-->>ArgoCD: degraded / error status
  ArgoCD-->>InfraRepo: writes .status.yaml (state: failed, reason: ...)

  RE->>InfraRepo: poll .status.yaml
  RE->>RE: step state → REMEDIATION (attempt=1)

  RE->>InfraRepo: recommit with force-sync annotation
  ArgoCD->>Crossplane: re-apply
  Crossplane->>CloudAPIs: retry provision

  alt Success
    CloudAPIs-->>Crossplane: ✅ provisioned
    ArgoCD-->>InfraRepo: .status.yaml (state: healthy)
    RE->>RE: step state → COMPLETED
  else Fails again
    ArgoCD-->>InfraRepo: .status.yaml (state: failed)
    RE->>RE: step state → FAILED (cap reached, no further commits)
  end

  Note over RE: External systems query RE API for job status
``` 
---

## Design Decisions

| Decision | Rationale |
|---|---|
| No PRs or branches | Infra repo is engine-managed; PRs add untracked state outside the job state machine |
| Approval via d09 gate, not GitHub reviewers | Enforces timeout, escalation, policy, and full audit inside the engine |
| Policy evaluation before approval | Approvers see a pre-validated request; policy violations never reach a human |
| Health verification via git, not direct Crossplane API | Maintains the principle that the engine's only interface to the GitOps layer is Git; ArgoCD writes health status back to the repo as annotations or status files |
| Remediation is recommit or force-sync annotation | The engine never talks to Crossplane or ArgoCD directly; it nudges the GitOps loop through Git |
