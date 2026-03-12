# Release Engine Workflows

This document provides an overview of all workflows available in the Release Engine.

| Workflow Name                                                 | Purpose | Value Score | Value % | Audience | Approval Mode | Recurrent Mode |
|---------------------------------------------------------------|---|:---:|:---:|---|---|---|
| [Infrastructure Provisioning](infra_provision.md)             | Self-service infrastructure provisioning via Backstage template → Release Engine → Crossplane GitOps | 40/40 | 100% | Dev | Optional engine-native (`waiting_approval`) | Optional (`schedule`) |
| [Scaffold Service](scaffold_service.md)                       | Golden Path workflow for creating new services from templates | 34/40 | 85% | Dev | Optional engine-native (`waiting_approval`) | Optional (`schedule`) |
| [Incident Management](incident_mgmt.md)                       | AI-coordinated incident response with triage and remediation workflows | 32/40 | 80% | Dev, Ops | Engine-native (`waiting_approval`) | Optional (`schedule`) |
| [Dependency Upgrade and Security Fix](dep_upgrade_sec_fix.md) | Automated dependency upgrade and CVE remediation workflow | 29/40 | 72.5% | Dev, Ops | Hybrid: Backstage review + optional engine-native gate | Recommended (`schedule`) for continuous CVE sweeps |
| [Environment Promotion](env_promotion.md)                     | Automated multi-environment promotion pipeline (Dev → Staging → Production) | 29/40 | 72.5% | Dev | Engine-native (`waiting_approval`) | Optional (`schedule`) |
| [Cost and Capacity Optimisation](cost_optimisation.md)        | AI-driven analysis of cloud cost and Kubernetes resource usage to generate right-sizing recommendations | 26/40 | 65% | Dev, Ops | Engine-native (`waiting_approval`) | Recommended (`schedule`) |
| [Patch Window Orchestration](patch_window.md)                 | Automated node patching workflow with Jira CHG approval and SLI monitoring | 26/40 | 65% | Ops | External (Jira CHG) | Optional (`schedule`) |
| [Flaky Test Triage](flaky_test.md)                            | AI-assisted triage of flaky test failures in CI | 22/40 | 55% | Dev | Human PR review + optional engine-native gate | Optional (`schedule`) |
| [Restore](restore.md)                                         | Incident-driven service restore workflow with AI-powered snapshot selection | 20/40 | 50% | Ops | Optional engine-native (`waiting_approval`) | Typically on-demand (no recurrence) |
| [Backup](backup.md)                                           | Automated, GitOps-driven backup orchestration for Kubernetes services | 16/40 | 40% | Ops | Optional engine-native (`waiting_approval`) | Primary mode (`schedule`) |

### Capability Notes

- **Engine-native approval** means workflow steps can enter `waiting_approval` and resume through `POST /v1/jobs/{job_id}/steps/{step_id}/decisions`.
- **Recurrent mode** means intake can include `schedule` (cron expression). Scheduled jobs re-queue after successful runs with the next occurrence time.
- **External approval** (for example Jira CHG) is intentionally outside the Release Engine approval step model.

## Value Score Breakdown

The Value Score is calculated using five dimensions:
- **Speed at Scale** - How well the workflow scales with the number of services
- **Consistency & Reduced Risk** - How the workflow reduces variability and risk
- **Governance Through Code** - How the workflow ensures governance through automation
- **Developer Experience (DX)** - How the workflow improves developer productivity
- **Clear Ownership / Fewer Hand-offs** - How the workflow reduces friction between teams

Each dimension is rated with T-shirt sizes: S (2), M (3), L (5), XL (8), giving a maximum possible score of 40/40.

## Gap Areas for New Workflows

New workflows that could add significant value:

| Gap Area | Description | Potential Value | Target Audience |
|---|---|:---:|---|
| Security Scanning & Compliance | Automated security scanning, vulnerability assessment, and compliance checking for deployed services | High | Dev, Ops |
| Database Migration | Automated database schema migration with rollback capabilities and data validation | High | Dev, Ops |
| Secret Rotation | Automated secret and credential rotation with zero downtime | High | Ops |
| Disaster Recovery Drill | Automated disaster recovery testing and failover drills | High | Ops |
| Chaos Engineering | Automated chaos experiments to test system resilience and fault tolerance | Medium | Dev, Ops |
| Performance Testing | Automated load testing and performance benchmarking | Medium | Dev |
| Configuration Drift Detection | Automated detection and remediation of configuration drift across environments | Medium | Ops |
| Service Decommission | Automated service deprecation and cleanup workflow | Medium | Dev, Ops |
| Log Aggregation & Analysis | Automated log collection, parsing, and anomaly detection | Medium | Dev |
| API Versioning & Deprecation | Automated API versioning management and deprecation scheduling | Medium | Dev |

### Value Opportunity Analysis

- **High Priority Gaps**: Security Scanning & Compliance, Database Migration, Secret Rotation, and Disaster Recovery Drill represent the highest value opportunities as they address critical operational concerns with limited current coverage.
- **Medium Priority Gaps**: Chaos Engineering, Performance Testing, and Configuration Drift Detection would enhance operational resilience and reliability.
- **Efficiency Gaps**: Log Aggregation & Analysis, Service Decommission, and API Versioning & Deprecation would improve developer productivity and reduce technical debt.