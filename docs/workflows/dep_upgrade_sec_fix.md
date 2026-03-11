# Dependency Upgrade and Security Fix

**Audience:** Dev, Ops

## Overview

Automated dependency upgrade and CVE remediation workflow. An AI agent monitors vulnerability feeds and proposes fixes; a deterministic engine rewrites the lockfile, opens a PR, and gates on CI. AI Code Agent is only invoked on escalation.

## Purpose

What this workflow accomplishes: Automated dependency upgrades and CVE remediation that scans the entire service estate, proposes fixes, and merges them through CI.

## Rationale

Why this workflow exists: To make security patches a default, automated process rather than a manual, error-prone task that accumulates technical debt.

## Benefit

What value it delivers:
- Every CVE is automatically triaged and fixed without waiting for developer awareness
- Eliminates vulnerability backlog on multi-service estates
- Deterministic engine handles routine upgrades; AI is reserved for complex escalations
- Full audit trail with CVE references, diff, and merge status
- Safe escalation: AI Code Agent only invoked when CI fails

## Value — TechOps as a Product

| Value Dimension | T-Shirt Size  | Notes |
|---|:-------------:|---|
| Speed at Scale |      XL       | Monitors all services simultaneously; upgrades are parallelised across the fleet. |
| Consistency & Reduced Risk |      XL       | Same upgrade logic applied everywhere; no service is left behind on outdated dependencies. |
| Governance Through Code |       L       | All changes go through PR and CI; CVEs are linked in the PR description for traceability. |
| Developer Experience (DX) |       L       | Developers are notified of upgrades; they can review but don't need to drive the process. |
| Clear Ownership / Fewer Hand-offs |       M       | TechOps owns the platform; developers receive ready-to-merge PRs instead of filing tickets. |

**Combined Value Score (Velocity 1):** 29/40 (XL + XL + L + L + M = 8 + 8 + 5 + 5 + 3)

---

```mermaid
---
title: "Release Engine — Dependency Upgrade Workflow (dependency/upgrade-dependencies)"
---
sequenceDiagram
    autonumber
    actor Developer
    participant AIAgent as DevOps AI Agent
    participant Backstage
    participant ReleaseEngine as Release Engine
    participant GitHubRepo as GitHub Repo
    participant CIPipeline as CI Pipeline
    participant SonarQube

    rect rgb(254, 243, 199)
        Note over Developer,Backstage: PATH A — human-driven discovery via Backstage
        Developer->>Backstage: manually select service, dependency, target version
    end

    rect rgb(220, 252, 231)
        Note over AIAgent,Backstage: PATH B — AI-assisted discovery with human approval
        AIAgent->>AIAgent: watch CVE feeds, OSV, Dependabot advisories
        AIAgent->>AIAgent: reason — which services affected, safe target version
        AIAgent->>Backstage: surface recommendation (service, dep, target version, CVE refs, risk)
        Developer->>Backstage: review recommendation and approve or reject
    end

    rect rgb(224, 242, 254)
        Note over Developer,ReleaseEngine: Both paths converge — Backstage submits to Release Engine
        Backstage->>ReleaseEngine: submit job (idempotency_key, intent, callback_url)
        ReleaseEngine-->>Backstage: 202 Accepted (job_id)
    end

    rect rgb(224, 242, 254)
        Note over ReleaseEngine: Internal Scheduling
        Note over ReleaseEngine: Scheduler claims job via SKIP LOCKED
        Note over ReleaseEngine: Runner acquires lease + run_id
    end

    rect rgb(243, 232, 255)
        Note over ReleaseEngine,GitHubRepo: Module Execution — DependencyUpgradeModule (deterministic)
        ReleaseEngine->>ReleaseEngine: detect package manager (npm, poetry, maven, go mod)
        ReleaseEngine->>ReleaseEngine: shell out to native tool to rewrite version
        Note over ReleaseEngine: npm install / poetry add / mvn versions:use-dep-version
        ReleaseEngine->>ReleaseEngine: verify lockfile updated, no unintended transitive changes
        ReleaseEngine->>GitHubRepo: open PR (branch, diff, CVE refs, run_id)
    end

    rect rgb(255, 237, 213)
        Note over GitHubRepo,SonarQube: CI and Quality Gate
        GitHubRepo-->>CIPipeline: PR triggers CI pipeline
        CIPipeline->>SonarQube: analyze quality and security posture
        SonarQube-->>CIPipeline: quality gate result
        CIPipeline-->>GitHubRepo: checks complete
    end

    rect rgb(243, 232, 255)
        Note over ReleaseEngine,GitHubRepo: Happy Path — all checks green
        ReleaseEngine->>GitHubRepo: poll PR status and check runs
        GitHubRepo-->>ReleaseEngine: all checks green
        ReleaseEngine->>GitHubRepo: enable auto-merge
        GitHubRepo-->>ReleaseEngine: merged (merge_sha)
    end

    rect rgb(254, 226, 226)
        Note over ReleaseEngine,AIAgent: Escalation Path — CI failed (breaking change or transitive conflict)
        CIPipeline-->>ReleaseEngine: checks failed (type errors, broken tests)
        ReleaseEngine->>Backstage: notify failure (pr_url, failed checks, diff context)
        Backstage-->>Developer: alert — escalation options presented
        Developer->>Backstage: approve AI Code Agent remediation
        Backstage->>ReleaseEngine: submit follow-up job (AI Code Agent step enabled)
        Note over ReleaseEngine: AI Code Agent invoked only on escalation — not on happy path
        ReleaseEngine->>AIAgent: invoke AI Code Agent (failed diff, error context, constraints)
        AIAgent-->>ReleaseEngine: remediation patch (updated dependency version or code fix)
        Note over ReleaseEngine: Deterministic Validation Gate — engine validates AI output before applying
        Note over ReleaseEngine: check: patch is syntactically valid, no forbidden imports, scope within target files
        alt validation passes
            ReleaseEngine->>ReleaseEngine: apply patch — rewrite version and lockfile
            ReleaseEngine->>GitHubRepo: open revised PR (updated branch, remediated diff, escalation_ref)
            GitHubRepo-->>ReleaseEngine: revised PR url confirmed
            GitHubRepo-->>CIPipeline: revised PR triggers CI pipeline
            CIPipeline-->>GitHubRepo: revised checks complete
            ReleaseEngine->>GitHubRepo: poll revised PR status
            GitHubRepo-->>ReleaseEngine: all checks green
            ReleaseEngine->>GitHubRepo: enable auto-merge on revised PR
            GitHubRepo-->>ReleaseEngine: merged (revised_merge_sha)
        else validation fails
            ReleaseEngine->>Backstage: notify escalation failure — manual intervention required
            Backstage-->>Developer: alert — AI Code Agent output invalid, manual fix needed
        end
    end

    rect rgb(220, 252, 231)
        Note over ReleaseEngine,AIAgent: Completion — callback to Backstage
        ReleaseEngine->>Backstage: webhook (job complete, pr_url, merge_sha, CVE refs closed)
        Backstage-->>Developer: upgrade visible in portal with full audit trail
        AIAgent->>AIAgent: consume result — chain next action or close advisory
    end
```