# RFC-INFRA-006: Detailed Module Param Mapping Spec

**Target module:** `infra/provision-crossplane`  
**Purpose:** define deterministic mapping from `InfrastructureRequest` to:

1. **Release Engine `job.params`**
2. **module input params**
3. **rendered XR/composition input fields**

This is the critical contract between the compiler and the provisioning golden path.

---
## 1. Mapping principles

### 1.1 Compiler owns interpretation

The compiler must:

- resolve defaults
- flatten intent into explicit params
- decide the effective template
- derive approval metadata
- derive namespace
- derive idempotency key

The module should receive an already-compiled, execution-ready parameter set.

### 1.2 Module owns orchestration only

The module should:

- take compiled params
- render manifests / values
- execute GitOps workflow
- request approval if needed
- verify outcome if enabled

It should **not** reinterpret high-level intent.

### 1.3 Effective values beat requested values

If a request says:

- delivery strategy = `direct-commit`

but policy/catalog says:

- only `pull-request` is allowed

then compiled params must contain only:

```yaml
git_strategy: pull-request
```

and optionally preserve requested value in metadata/audit only.

---

## 2. Canonical `job.params` structure

Recommended canonical structure submitted to Release Engine:

```yaml
template_name: analytics-platform-prod
composition_ref: xrd.analytics.platform/v1
namespace: team-data

git_repo: acme/infra-live
git_branch: main
git_strategy: pull-request

    verify_cloud: true
    cloud_resource_type: composite

    approvalContext:
      required: true
      decisionBasis:
        policyOutcome: allow_with_approval
        reasonCodes:
          - high_blast_radius
      riskSummary:
        blastRadius: high
        estimatedCostBand: medium
      reviewContext:
        request_name: analytics-prod-eu
        tenant: acme
        owner: team-data
        environment: production
        template: analytics-platform-prod
        composition_ref: xrd.analytics.platform/v1
        estimated_cost: "2475"
        currency: GBP
        compiler_version: "1.0.0"
        catalog_version: "2026-03-15"
        policy_version: "2026-03-15.1"
      suggestedApproverRoles:
        - techops-lead
      ttl:
        expiresAt: "2026-04-01T12:00:00Z"

    parameters:
  request_name: analytics-prod-eu
  tenant: acme
  owner: team-data
  environment: production

  workload_type: analytics-platform
  workload_profile: medium
  workload_exposure: internal

  residency: eu
  primary_region: eu-west-1
  secondary_region: eu-central-1

  kubernetes_enabled: true
  kubernetes_tier: standard
  kubernetes_size: medium
  kubernetes_multi_az: true

  database_enabled: true
  database_engine: postgres
  database_tier: highly-available
  database_storage_gib: 500
  database_backup_enabled: true
  database_backup_retention_days: 30

  object_storage_enabled: true
  object_storage_class: standard
  object_storage_versioning: true

  messaging_enabled: false
  messaging_tier: ""

  compliance: ["gdpr"]
  ingress_mode: internal
  egress_mode: controlled
  data_classification: confidential

  availability: high
  backup_required: true
  dr_required: true
  monitoring: enhanced

  cost_max_monthly: 3000
  cost_currency: GBP
```

---

## 3. Mapping model

There are **three layers**:

| Layer | Purpose |
|---|---|
| Intent fields | User-facing request |
| Compiled module params | Flat deterministic execution inputs |
| XR fields | Composition/XR-specific rendered values |

---

## 4. Detailed mapping table

## 4.1 Identity and tenancy

| Intent Field | Required | Transform | Module Param | XR Field / Use |
|---|---:|---|---|---|
| `metadata.name` | yes | copy | `parameters.request_name` | `metadata.name`, labels, annotations |
| `metadata.tenant` | yes | copy, auth-checked | `parameters.tenant` | namespace/repo path/labels |
| `spec.owner` | yes | sanitize for namespace-safe if needed | `parameters.owner` | `spec.owner`, namespace derivation |
| `spec.environment` | yes | copy enum | `parameters.environment` | `spec.environment`, labels |

### Notes
- `metadata.tenant` must be validated against auth context.
- `metadata.name` should be DNS-safe already by schema.
- `spec.owner` may need a slugified form for namespace purposes, but preserve original in metadata if needed.

---

## 4.2 Workload mapping

| Intent Field | Required | Defaulting | Module Param | XR Field |
|---|---:|---|---|---|
| `spec.workload.type` | yes | none | `parameters.workload_type` | `spec.workload.type` |
| `spec.workload.profile` | no | catalog default or `medium` | `parameters.workload_profile` | sizing preset |
| `spec.workload.exposure` | no | derive from `security.ingress` or catalog default | `parameters.workload_exposure` | `spec.network.exposure` |

### Recommended derivation
If `workload.exposure` is omitted:

- `security.ingress = private` → `workload_exposure = private`
- `security.ingress = internal` → `workload_exposure = internal`
- `security.ingress = public` → `workload_exposure = public`

---

## 4.3 Location mapping

| Intent Field | Required | Defaulting | Module Param | XR Field |
|---|---:|---|---|---|
| `spec.location.residency` | no | catalog default | `parameters.residency` | placement policy |
| `spec.location.primaryRegion` | no | catalog default or selected region | `parameters.primary_region` | `spec.region.primary` |
| `spec.location.secondaryRegion` | no | required if DR true and template supports DR | `parameters.secondary_region` | `spec.region.secondary` |

### Notes
- `primaryRegion` and `secondaryRegion` should be validated against catalog allowed regions.
- If `drRequired = true` and no secondary region is provided, compiler may:
    - reject,
    - auto-derive if catalog has paired-region policy,
    - or mark `allow_with_approval`.

---

## 4.4 Kubernetes capability mapping

| Intent Field | Required | Defaulting | Module Param | XR Field |
|---|---:|---|---|---|
| `spec.capabilities.kubernetes.enabled` | no | infer from matched template requirements | `parameters.kubernetes_enabled` | whether cluster resources are rendered |
| `spec.capabilities.kubernetes.tier` | no | catalog default | `parameters.kubernetes_tier` | cluster class/security baseline |
| `spec.capabilities.kubernetes.size` | no | derive from workload profile | `parameters.kubernetes_size` | node pool preset |
| `spec.capabilities.kubernetes.multiAz` | no | derive from availability/environment | `parameters.kubernetes_multi_az` | HA placement setting |

### Suggested derivation rules

```text
if environment = production => multiAz default true
if operations.availability = high => multiAz default true
if workload.profile = small and environment != production => size default small
if workload.profile = medium => size default medium
if workload.profile = large => size default large
```

### Example XR mapping

```yaml
spec:
  kubernetes:
    enabled: true
    tier: standard
    size: medium
    multiAz: true
```

---

## 4.5 Database capability mapping

| Intent Field | Required | Defaulting | Module Param | XR Field |
|---|---:|---|---|---|
| `spec.capabilities.database.enabled` | no | infer from template | `parameters.database_enabled` | whether DB rendered |
| `spec.capabilities.database.engine` | no | catalog default | `parameters.database_engine` | `spec.database.engine` |
| `spec.capabilities.database.tier` | no | derive from env/availability | `parameters.database_tier` | `spec.database.tier` |
| `spec.capabilities.database.storageGiB` | no | workload default | `parameters.database_storage_gib` | `spec.database.storageGiB` |
| `spec.capabilities.database.backup.enabled` | no | true if backup required or prod | `parameters.database_backup_enabled` | `spec.database.backup.enabled` |
| `spec.capabilities.database.backup.retentionDays` | no | policy/catalog default | `parameters.database_backup_retention_days` | `spec.database.backup.retentionDays` |

### Suggested derivation rules

| Condition | Derived DB Tier |
|---|---|
| `environment = development` | `dev` |
| `environment = test` | `standard` |
| `environment = staging` | `standard` |
| `environment = production` and `availability != high` | `standard` |
| `environment = production` and `availability = high` | `highly-available` |

### Storage default suggestions

| Workload Profile | Default DB Storage |
|---|---:|
| `small` | 50 GiB |
| `medium` | 200 GiB |
| `large` | 500 GiB |

### Example XR mapping

```yaml
spec:
  database:
    enabled: true
    engine: postgres
    tier: highly-available
    storageGiB: 500
    backup:
      enabled: true
      retentionDays: 30
```

---

## 4.6 Object storage mapping

| Intent Field | Required | Defaulting | Module Param | XR Field |
|---|---:|---|---|---|
| `spec.capabilities.objectStorage.enabled` | no | infer from template/workload | `parameters.object_storage_enabled` | render bucket/object store |
| `spec.capabilities.objectStorage.class` | no | `standard` | `parameters.object_storage_class` | storage class |
| `spec.capabilities.objectStorage.versioning` | no | true for production/confidential+ | `parameters.object_storage_versioning` | bucket versioning |

### Suggested derivation
- if `environment = production` → versioning default `true`
- if `dataClassification in [confidential, restricted]` → versioning default `true`

---

## 4.7 Messaging mapping

| Intent Field | Required | Defaulting | Module Param | XR Field |
|---|---:|---|---|---|
| `spec.capabilities.messaging.enabled` | no | false | `parameters.messaging_enabled` | render queue/bus |
| `spec.capabilities.messaging.tier` | no | `standard` if enabled | `parameters.messaging_tier` | messaging class |

---

## 4.8 Security mapping

| Intent Field | Required | Defaulting | Module Param | XR Field |
|---|---:|---|---|---|
| `spec.security.compliance[]` | no | `[]` | `parameters.compliance` | labels/policies/compliance controls |
| `spec.security.ingress` | no | derive from workload exposure | `parameters.ingress_mode` | networking/ingress config |
| `spec.security.egress` | no | catalog default `controlled` | `parameters.egress_mode` | network policy / firewall mode |
| `spec.security.dataClassification` | no | `internal` | `parameters.data_classification` | policy labels / encryption / restrictions |

### Suggested derivation rules

| Condition | Derived Value |
|---|---|
| `workload.exposure = private` | `ingress = private` |
| `workload.exposure = internal` | `ingress = internal` |
| `workload.exposure = public` | `ingress = public` |
| `dataClassification = restricted` | `egress != open` enforced |

### XR example

```yaml
spec:
  security:
    compliance:
      - gdpr
    ingress: internal
    egress: controlled
    dataClassification: confidential
```

---

## 4.9 Operations mapping

| Intent Field | Required | Defaulting | Module Param | XR Field |
|---|---:|---|---|---|
| `spec.operations.availability` | no | derive from environment | `parameters.availability` | HA/replica/multi-AZ posture |
| `spec.operations.backupRequired` | no | true in production | `parameters.backup_required` | backup policy requirement |
| `spec.operations.drRequired` | no | false | `parameters.dr_required` | DR topology requirement |
| `spec.operations.monitoring` | no | derive from environment | `parameters.monitoring` | monitoring profile |

### Suggested defaults

| Environment | Availability | Backup | Monitoring |
|---|---|---|---|
| development | best-effort | false | basic |
| test | best-effort | false | basic |
| staging | standard | true | standard |
| production | high | true | enhanced |

---

## 4.10 Cost mapping

| Intent Field | Required | Defaulting | Module Param | Use |
|---|---:|---|---|---|
| `spec.cost.maxMonthly` | no | none | `parameters.cost_max_monthly` | budget enforcement / metadata |
| `spec.cost.currency` | no | tenant default or `GBP` | `parameters.cost_currency` | estimate display |
| `spec.cost.approvalRequiredAbove` | no | policy default | not necessarily passed to XR | approval derivation |

### Note
`approvalRequiredAbove` is generally a **compiler/policy concern**, not an XR concern.

---

## 4.11 Delivery mapping

| Intent Field | Required | Defaulting | Module Param / Top-level Param | Use |
|---|---:|---|---|---|
| `spec.delivery.strategy` | no | catalog default | `git_strategy` | GitOps execution mode |
| `spec.delivery.verifyCloud` | no | catalog default | `verify_cloud` | post-apply verification |
| `spec.delivery.callbackUrl` | no | none | `callback_url` in submit request, not XR | async callback |

### Important distinction
- `callbackUrl` belongs to submission/adaptor behavior.
- It should normally **not** be passed through `parameters`.

---

## 5. Top-level compiled params mapping

## 5.1 Template resolution

| Source | Output Param |
|---|---|
| catalog `resolvesTo.templateName` | `template_name` |
| catalog `resolvesTo.compositionRef` | `composition_ref` |
| resolved namespace | `namespace` |

## 5.2 Git execution

| Source | Output Param |
|---|---|
| platform config | `git_repo` |
| platform config | `git_branch` |
| effective delivery strategy | `git_strategy` |

## 5.3 Verification

| Source | Output Param |
|---|---|
| effective delivery.verifyCloud | `verify_cloud` |
| module default or catalog | `cloud_resource_type` |

---

## 6. Approval mapping spec

Approval is not a direct user field; it is **derived** from policy.

## 6.1 Derived fields

| Derived Input | Output Param |
|---|---|
| `policyOutcome == allow_with_approval` | `approvalContext.required = true` |
| policy default TTL | `approvalContext.ttl.expiresAt` |
| computed summary data | `approvalContext.reviewContext` |

## 6.2 Recommended `approvalContext` keys

```yaml
approvalContext:
  required: true
  decisionBasis:
    policyOutcome: allow_with_approval
    reasonCodes:
      - high_blast_radius
  riskSummary:
    blastRadius: high
    estimatedCostBand: medium
  reviewContext:
    request_name: analytics-prod-eu
    tenant: acme
    owner: team-data
    environment: production
    workload_type: analytics-platform
    template: analytics-platform-prod
    composition_ref: xrd.analytics.platform/v1
    estimated_cost: "2475"
    currency: GBP
    compiler_version: "1.0.0"
    catalog_version: "2026-03-15"
    policy_version: "2026-03-15.1"
  suggestedApproverRoles:
    - techops-lead
  ttl:
    expiresAt: "2026-04-01T12:00:00Z"
```

## 6.3 Approval summary recommendation

A concise approval prompt summary should be derivable as:

```text
Provision analytics-prod-eu for tenant acme in production using template analytics-platform-prod (cost ~2475 GBP/month, blast radius high)
```

---

## 7. XR rendering contract

The module should translate flat `parameters.*` into XR fields predictably.

## 7.1 Example rendered XR

```yaml
apiVersion: platform.example.io/v1alpha1
kind: AnalyticsPlatform
metadata:
  name: analytics-prod-eu
  namespace: team-data
  labels:
    tenant: acme
    owner: team-data
    environment: production
spec:
  owner: team-data
  environment: production

  workload:
    type: analytics-platform
    profile: medium
    exposure: internal

  location:
    residency: eu
    primaryRegion: eu-west-1
    secondaryRegion: eu-central-1

  kubernetes:
    enabled: true
    tier: standard
    size: medium
    multiAz: true

  database:
    enabled: true
    engine: postgres
    tier: highly-available
    storageGiB: 500
    backup:
      enabled: true
      retentionDays: 30

  objectStorage:
    enabled: true
    class: standard
    versioning: true

  security:
    compliance:
      - gdpr
    ingress: internal
    egress: controlled
    dataClassification: confidential

  operations:
    availability: high
    backupRequired: true
    drRequired: true
    monitoring: enhanced
```

---

## 8. Defaulting matrix

Below is a recommended compiler defaulting matrix.

| Field | Rule |
|---|---|
| `workload.profile` | default `medium` |
| `workload.exposure` | derive from ingress or default `internal` |
| `kubernetes.enabled` | infer from template requirements |
| `kubernetes.tier` | default `standard` |
| `kubernetes.size` | derive from workload profile |
| `kubernetes.multiAz` | true for prod/high availability |
| `database.enabled` | infer from template requirements |
| `database.engine` | catalog default, usually `postgres` |
| `database.tier` | derive from env + availability |
| `database.storageGiB` | derive from profile |
| `database.backup.enabled` | true in prod or if backupRequired |
| `database.backup.retentionDays` | policy default, e.g. 30 |
| `objectStorage.enabled` | infer from template |
| `objectStorage.class` | `standard` |
| `objectStorage.versioning` | true in prod/confidential+ |
| `messaging.enabled` | false |
| `messaging.tier` | `standard` if enabled |
| `security.ingress` | derive from exposure |
| `security.egress` | default `controlled` |
| `security.dataClassification` | default `internal` |
| `operations.availability` | derive from env |
| `operations.backupRequired` | true in prod |
| `operations.drRequired` | false |
| `operations.monitoring` | derive from env |
| `cost.currency` | tenant default or `GBP` |
| `delivery.strategy` | catalog/platform default |
| `delivery.verifyCloud` | catalog/platform default |

---

## 9. Unsupported / non-mappable fields

If a request includes semantically meaningful intent that the module cannot express, the compiler should **not silently drop it**.

Allowed outcomes:

- `deny`
- `allow_with_approval`

Examples:

- unsupported compliance combination
- public production ingress when policy forbids it
- requested region outside approved list
- request for messaging type not modeled by template
- specific backup topology unsupported by target XR

---

## 10. Deterministic mapping rules

To keep idempotency stable, the compiler must:

1. normalize omitted values to explicit effective values
2. sort arrays where ordering is not semantically relevant
3. serialize booleans and numerics consistently
4. emit all effective execution params in a stable order before hashing

For example, this:

```yaml
compliance:
  - gdpr
  - pci
```

and this:

```yaml
compliance:
  - pci
  - gdpr
```

should canonicalize identically if order is semantically irrelevant.

---

## 11. Compiler pseudocode for mapping

```go
func MapRequestToParams(req *InfrastructureRequest, match *CatalogMatch, policy *PolicyDecision, estimate *CostEstimate) map[string]any {
    effective := NormalizeAndDefault(req, match, policy)

    params := map[string]any{
        "template_name":    match.TemplateName,
        "composition_ref":  match.CompositionRef,
        "namespace":        match.Namespace,
        "git_repo":         PlatformGitRepo(req.Metadata.Tenant),
        "git_branch":       PlatformGitBranch(req.Metadata.Tenant),
        "git_strategy":     effective.DeliveryStrategy,
        "verify_cloud":     effective.VerifyCloud,
        "cloud_resource_type": "composite",
        "approvalContext": map[string]any{
            "required": policy.Outcome == PolicyOutcomeAllowWithApproval,
            "decisionBasis": map[string]any{
                "policyOutcome": string(policy.Outcome),
                "reasonCodes":   policy.ReasonCodes,
            },
            "riskSummary": map[string]any{
                "blastRadius":       effective.BlastRadius,
                "estimatedCostBand": effective.CostBand,
            },
            "reviewContext": map[string]any{
                "request_name":      req.Metadata.Name,
                "tenant":            req.Metadata.Tenant,
                "owner":             effective.Owner,
                "environment":       effective.Environment,
                "template":          match.TemplateName,
                "composition_ref":   match.CompositionRef,
                "estimated_cost":    fmt.Sprintf("%.0f", estimate.Monthly),
                "currency":          estimate.Currency,
                "compiler_version":  CompilerVersion,
                "catalog_version":   match.CatalogVersion,
                "policy_version":    policy.Version,
            },
            "suggestedApproverRoles": policy.AllowedRoles,
            "ttl": map[string]any{
                "expiresAt": PolicyApprovalTTL(policy),
            },
        },
        "parameters": map[string]any{
            "request_name":                    req.Metadata.Name,
            "tenant":                          req.Metadata.Tenant,
            "owner":                           effective.Owner,
            "environment":                     effective.Environment,
            "workload_type":                   effective.WorkloadType,
            "workload_profile":                effective.WorkloadProfile,
            "workload_exposure":               effective.WorkloadExposure,
            "residency":                       effective.Residency,
            "primary_region":                  effective.PrimaryRegion,
            "secondary_region":                effective.SecondaryRegion,
            "kubernetes_enabled":              effective.KubernetesEnabled,
            "kubernetes_tier":                 effective.KubernetesTier,
            "kubernetes_size":                 effective.KubernetesSize,
            "kubernetes_multi_az":             effective.KubernetesMultiAZ,
            "database_enabled":                effective.DatabaseEnabled,
            "database_engine":                 effective.DatabaseEngine,
            "database_tier":                   effective.DatabaseTier,
            "database_storage_gib":            effective.DatabaseStorageGiB,
            "database_backup_enabled":         effective.DatabaseBackupEnabled,
            "database_backup_retention_days":  effective.DatabaseBackupRetentionDays,
            "object_storage_enabled":          effective.ObjectStorageEnabled,
            "object_storage_class":            effective.ObjectStorageClass,
            "object_storage_versioning":       effective.ObjectStorageVersioning,
            "messaging_enabled":               effective.MessagingEnabled,
            "messaging_tier":                  effective.MessagingTier,
            "compliance":                      effective.Compliance,
            "ingress_mode":                    effective.IngressMode,
            "egress_mode":                     effective.EgressMode,
            "data_classification":             effective.DataClassification,
            "availability":                    effective.Availability,
            "backup_required":                 effective.BackupRequired,
            "dr_required":                     effective.DRRequired,
            "monitoring":                      effective.Monitoring,
            "cost_max_monthly":                effective.CostMaxMonthly,
            "cost_currency":                   effective.CostCurrency,
        },
    }

    return params
}
```

---

## 12. Example compiled output

For the sample request, the compiler should produce something like:

```yaml
tenant_id: acme
path_key: golden-path/infra/provision-crossplane
idempotency_key: infrareq-analytics-prod-eu-v1-a13f29c2
params:
  template_name: analytics-platform-prod
  composition_ref: xrd.analytics.platform/v1
  namespace: team-data
  git_repo: acme/infra-live
  git_branch: main
  git_strategy: pull-request
  verify_cloud: true
  cloud_resource_type: composite
  approvalContext:
    required: true
    decisionBasis:
      policyOutcome: allow_with_approval
      reasonCodes:
        - high_blast_radius
    riskSummary:
      blastRadius: high
      estimatedCostBand: medium
    reviewContext:
      request_name: analytics-prod-eu
      tenant: acme
      owner: team-data
      environment: production
      template: analytics-platform-prod
      composition_ref: xrd.analytics.platform/v1
      estimated_cost: "2475"
      currency: GBP
      compiler_version: "1.0.0"
      catalog_version: "2026-03-15"
      policy_version: "2026-03-15.1"
    suggestedApproverRoles:
      - techops-lead
    ttl:
      expiresAt: "2026-04-01T12:00:00Z"
  parameters:
    request_name: analytics-prod-eu
    tenant: acme
    owner: team-data
    environment: production
    workload_type: analytics-platform
    workload_profile: medium
    workload_exposure: internal
    residency: eu
    primary_region: eu-west-1
    secondary_region: eu-central-1
    kubernetes_enabled: true
    kubernetes_tier: standard
    kubernetes_size: medium
    kubernetes_multi_az: true
    database_enabled: true
    database_engine: postgres
    database_tier: highly-available
    database_storage_gib: 500
    database_backup_enabled: true
    database_backup_retention_days: 30
    object_storage_enabled: true
    object_storage_class: standard
    object_storage_versioning: true
    messaging_enabled: false
    messaging_tier: ""
    compliance:
      - gdpr
    ingress_mode: internal
    egress_mode: controlled
    data_classification: confidential
    availability: high
    backup_required: true
    dr_required: true
    monitoring: enhanced
    cost_max_monthly: 3000
    cost_currency: GBP
```

---

## 13. Strong recommendation on module contract versioning

Introduce an explicit param:

```yaml
contract_version: infra-provision-crossplane/v1
```

### Why
This gives you a clean boundary for future evolution.

Recommended top-level addition:

```yaml
params:
  contract_version: infra-provision-crossplane/v1
  template_name: ...
```

That lets the module validate whether the compiler is speaking a supported param dialect.

---
