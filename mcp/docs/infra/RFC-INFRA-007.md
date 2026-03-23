# RFC-INFRA-007: Fixture Pack + Golden Test Cases

Below is an implementation-ready **test fixture pack** and a **Go golden-test skeleton** for the schema/compile pipeline.

This is designed to validate:

- schema correctness
- semantic validation
- defaulting behavior
- catalog resolution
- deterministic compilation
- idempotency stability
- approval derivation
- rejection/approval paths

---

# 1. Goals

The fixture pack should prove these invariants:

1. **Valid requests compile deterministically**
2. **Equivalent input normalizes to identical compiled output**
3. **Invalid requests fail early with precise diagnostics**
4. **Unsupported but well-formed requests become `allow_with_approval` or `deny`**
5. **Approval and cost policy decisions are reproducible**
6. **Compiled params remain backward-compatible across refactors**

---

# 2. Recommended test directory layout

```text
test/
├── fixtures/
│   ├── catalog/
│   │   ├── capability-catalog.base.yaml
│   │   └── capability-catalog.restricted.yaml
│   ├── policies/
│   │   ├── policy-bundle.base.yaml
│   │   └── policy-bundle.strict.yaml
│   ├── requests/
│   │   ├── valid/
│   │   │   ├── analytics-prod-eu.yaml
│   │   │   ├── web-dev-private.yaml
│   │   │   ├── api-staging-internal.yaml
│   │   │   └── equivalent-ordering-a.yaml
│   │   │   └── equivalent-ordering-b.yaml
│   │   ├── invalid/
│   │   │   ├── missing-owner.yaml
│   │   │   ├── invalid-environment.yaml
│   │   │   ├── invalid-name.yaml
│   │   │   ├── invalid-db-engine.yaml
│   │   │   └── invalid-callback-url.yaml
│   │   └── review/
│   │       ├── dr-no-secondary-region.yaml
│   │       ├── unsupported-compliance-combo.yaml
│   │       └── region-not-in-allow-list.yaml
│   ├── expected/
│   │   ├── validation/
│   │   │   ├── analytics-prod-eu.validation.yaml
│   │   │   ├── missing-owner.validation.yaml
│   │   │   └── dr-no-secondary-region.validation.yaml
│   │   ├── compiled/
│   │   │   ├── analytics-prod-eu.compiled.yaml
│   │   │   ├── web-dev-private.compiled.yaml
│   │   │   ├── api-staging-internal.compiled.yaml
│   │   │   ├── equivalent-ordering-a.compiled.yaml
│   │   │   └── equivalent-ordering-b.compiled.yaml
│   │   └── submit/
│   │       └── analytics-prod-eu.job.yaml
│   └── snapshots/
│       └── compiler-version.txt
└── golden/
    ├── validate_test.go
    ├── compile_test.go
    ├── canonicalize_test.go
    └── submit_test.go
```

---

# 3. Shared fixture data

## 3.1 Base capability catalog

**File:** `test/fixtures/catalog/capability-catalog.base.yaml`

```yaml
apiVersion: platform.gatblau.io/v1
kind: CapabilityCatalog
metadata:
  name: base-catalog
  version: "2026-03-15"
spec:
  templates:
    - id: analytics-platform-prod
      displayName: Analytics Platform Production
      match:
        workloadType: analytics-platform
        environment: production
        exposure: internal
      requires:
        capabilities:
          kubernetes: true
          database: true
          objectStorage: true
          messaging: false
      allows:
        regions:
          - eu-west-1
          - eu-central-1
          - uk-west-2
        databaseEngines:
          - postgres
        compliance:
          - gdpr
          - sox
      resolvesTo:
        templateName: analytics-platform-prod
        compositionRef: xrd.analytics.platform/v1
        namespaceStrategy: owner
      defaults:
        gitStrategy: pull-request
        verifyCloud: true
        approvalClass: high-blast-radius
      costModel:
        kubernetesBase: 1200
        databaseHA: 900
        objectStorage: 150
        monitoringEnhanced: 225

    - id: web-service-dev
      displayName: Web Service Development
      match:
        workloadType: web-service
        environment: development
        exposure: private
      requires:
        capabilities:
          kubernetes: true
          database: false
          objectStorage: false
          messaging: false
      allows:
        regions:
          - eu-west-1
          - uk-west-2
        databaseEngines:
          - postgres
          - mysql
        compliance:
          - none
          - gdpr
      resolvesTo:
        templateName: web-service-dev
        compositionRef: xrd.web.service/v1
        namespaceStrategy: owner
      defaults:
        gitStrategy: direct-commit
        verifyCloud: false
        approvalClass: low-blast-radius
      costModel:
        kubernetesBase: 150
        monitoringBasic: 20

    - id: api-service-staging
      displayName: API Service Staging
      match:
        workloadType: api-service
        environment: staging
        exposure: internal
      requires:
        capabilities:
          kubernetes: true
          database: true
          objectStorage: false
          messaging: false
      allows:
        regions:
          - eu-west-1
          - eu-central-1
        databaseEngines:
          - postgres
          - mysql
        compliance:
          - gdpr
          - none
      resolvesTo:
        templateName: api-service-staging
        compositionRef: xrd.api.service/v1
        namespaceStrategy: owner
      defaults:
        gitStrategy: pull-request
        verifyCloud: true
        approvalClass: standard
      costModel:
        kubernetesBase: 400
        databaseStandard: 250
        monitoringStandard: 80
```

---

## 3.2 Restricted catalog

**File:** `test/fixtures/catalog/capability-catalog.restricted.yaml`

```yaml
apiVersion: platform.gatblau.io/v1
kind: CapabilityCatalog
metadata:
  name: restricted-catalog
  version: "2026-03-15-restricted"
spec:
  templates:
    - id: analytics-platform-prod
      displayName: Analytics Platform Production
      match:
        workloadType: analytics-platform
        environment: production
        exposure: internal
      requires:
        capabilities:
          kubernetes: true
          database: true
          objectStorage: true
          messaging: false
      allows:
        regions:
          - eu-west-1
        databaseEngines:
          - postgres
        compliance:
          - gdpr
      resolvesTo:
        templateName: analytics-platform-prod
        compositionRef: xrd.analytics.platform/v1
        namespaceStrategy: owner
      defaults:
        gitStrategy: pull-request
        verifyCloud: true
        approvalClass: high-blast-radius
```

---

## 3.3 Base policy bundle

**File:** `test/fixtures/policies/policy-bundle.base.yaml`

```yaml
apiVersion: platform.gatblau.io/v1
kind: PolicyBundle
metadata:
  name: base-policy
  version: "2026-03-15.1"
spec:
  defaults:
    currency: GBP
    approvalTTL: 24h
    databaseBackupRetentionDays: 30
  rules:
    - id: require-approval-production
      when:
        environmentIn: [production]
      decision:
        requireApproval: true
        approvalClass: high-blast-radius

    - id: require-approval-cost-threshold
      when:
        estimatedMonthlyCostGte: 1000
      decision:
        requireApproval: true
        approvalClass: cost-threshold

    - id: restricted-data-requires-controlled-egress
      when:
        dataClassificationIn: [restricted]
      decision:
        enforce:
          egress: controlled
```

---

## 3.4 Strict policy bundle

**File:** `test/fixtures/policies/policy-bundle.strict.yaml`

```yaml
apiVersion: platform.gatblau.io/v1
kind: PolicyBundle
metadata:
  name: strict-policy
  version: "2026-03-15.2"
spec:
  defaults:
    currency: GBP
    approvalTTL: 24h
    databaseBackupRetentionDays: 35
  rules:
    - id: public-prod-forbidden
      when:
        environmentIn: [production]
        ingressIn: [public]
      decision:
        reject: true
        reason: public ingress is forbidden in production

    - id: dr-requires-secondary-region
      when:
        drRequired: true
      decision:
        requireSecondaryRegion: true

    - id: pci-requires-approval
      when:
        complianceIncludes: [pci]
      decision:
        requireApproval: true
        reason: pci workloads require platform review
```

---

# 4. Request fixtures

---

## 4.1 Valid: analytics production EU

**File:** `test/fixtures/requests/valid/analytics-prod-eu.yaml`

```yaml
apiVersion: platform.gatblau.io/v1alpha1
kind: InfrastructureRequest
metadata:
  name: analytics-prod-eu
  tenant: acme
  labels:
    app: analytics
spec:
  owner: team-data
  environment: production
  workload:
    type: analytics-platform
    profile: medium
    exposure: internal
  capabilities:
    kubernetes:
      enabled: true
      tier: standard
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
  location:
    residency: eu
    primaryRegion: eu-west-1
    secondaryRegion: eu-central-1
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
  cost:
    maxMonthly: 3000
    currency: GBP
    approvalRequiredAbove: 2000
  delivery:
    strategy: pull-request
    verifyCloud: true
    callbackUrl: https://example.org/callbacks/infra/acme
```

---

## 4.2 Valid: web dev private

**File:** `test/fixtures/requests/valid/web-dev-private.yaml`

```yaml
apiVersion: platform.gatblau.io/v1alpha1
kind: InfrastructureRequest
metadata:
  name: web-dev-private
  tenant: acme
spec:
  owner: team-web
  environment: development
  workload:
    type: web-service
    profile: small
    exposure: private
  capabilities:
    kubernetes:
      enabled: true
  location:
    residency: uk
    primaryRegion: uk-west-2
  security:
    dataClassification: internal
  operations:
    monitoring: basic
```

---

## 4.3 Valid: api staging internal

**File:** `test/fixtures/requests/valid/api-staging-internal.yaml`

```yaml
apiVersion: platform.gatblau.io/v1alpha1
kind: InfrastructureRequest
metadata:
  name: api-staging-internal
  tenant: acme
spec:
  owner: team-api
  environment: staging
  workload:
    type: api-service
    profile: medium
    exposure: internal
  capabilities:
    kubernetes:
      enabled: true
    database:
      enabled: true
      engine: postgres
  location:
    residency: eu
    primaryRegion: eu-west-1
  security:
    compliance:
      - gdpr
    ingress: internal
    dataClassification: internal
  operations:
    availability: standard
    backupRequired: true
    monitoring: standard
```

---

## 4.4 Equivalent canonicalization A

**File:** `test/fixtures/requests/valid/equivalent-ordering-a.yaml`

```yaml
apiVersion: platform.gatblau.io/v1alpha1
kind: InfrastructureRequest
metadata:
  name: eq-order-a
  tenant: acme
spec:
  owner: team-data
  environment: production
  workload:
    type: analytics-platform
    exposure: internal
    profile: medium
  security:
    compliance:
      - gdpr
      - sox
    ingress: internal
    egress: controlled
    dataClassification: confidential
  location:
    residency: eu
    primaryRegion: eu-west-1
    secondaryRegion: eu-central-1
  operations:
    drRequired: true
    availability: high
    monitoring: enhanced
  capabilities:
    database:
      enabled: true
      engine: postgres
    kubernetes:
      enabled: true
    objectStorage:
      enabled: true
      versioning: true
```

---

## 4.5 Equivalent canonicalization B

**File:** `test/fixtures/requests/valid/equivalent-ordering-b.yaml`

```yaml
apiVersion: platform.gatblau.io/v1alpha1
kind: InfrastructureRequest
metadata:
  tenant: acme
  name: eq-order-a
spec:
  environment: production
  owner: team-data
  workload:
    profile: medium
    type: analytics-platform
    exposure: internal
  capabilities:
    objectStorage:
      versioning: true
      enabled: true
    kubernetes:
      enabled: true
    database:
      engine: postgres
      enabled: true
  operations:
    monitoring: enhanced
    availability: high
    drRequired: true
  location:
    secondaryRegion: eu-central-1
    primaryRegion: eu-west-1
    residency: eu
  security:
    dataClassification: confidential
    egress: controlled
    ingress: internal
    compliance:
      - sox
      - gdpr
```

---

## 4.6 Invalid: missing owner

**File:** `test/fixtures/requests/invalid/missing-owner.yaml`

```yaml
apiVersion: platform.gatblau.io/v1alpha1
kind: InfrastructureRequest
metadata:
  name: missing-owner
  tenant: acme
spec:
  environment: production
  workload:
    type: analytics-platform
```

---

## 4.7 Invalid: invalid environment

**File:** `test/fixtures/requests/invalid/invalid-environment.yaml`

```yaml
apiVersion: platform.gatblau.io/v1alpha1
kind: InfrastructureRequest
metadata:
  name: invalid-environment
  tenant: acme
spec:
  owner: team-x
  environment: prod
  workload:
    type: web-service
```

---

## 4.8 Invalid: invalid name

**File:** `test/fixtures/requests/invalid/invalid-name.yaml`

```yaml
apiVersion: platform.gatblau.io/v1alpha1
kind: InfrastructureRequest
metadata:
  name: Invalid_Name
  tenant: acme
spec:
  owner: team-x
  environment: development
  workload:
    type: web-service
```

---

## 4.9 Invalid: invalid DB engine

**File:** `test/fixtures/requests/invalid/invalid-db-engine.yaml`

```yaml
apiVersion: platform.gatblau.io/v1alpha1
kind: InfrastructureRequest
metadata:
  name: invalid-db-engine
  tenant: acme
spec:
  owner: team-db
  environment: staging
  workload:
    type: api-service
    exposure: internal
  capabilities:
    database:
      enabled: true
      engine: oracle
```

---

## 4.10 Invalid: callback URL malformed

**File:** `test/fixtures/requests/invalid/invalid-callback-url.yaml`

```yaml
apiVersion: platform.gatblau.io/v1alpha1
kind: InfrastructureRequest
metadata:
  name: invalid-callback-url
  tenant: acme
spec:
  owner: team-web
  environment: development
  workload:
    type: web-service
  delivery:
    callbackUrl: not-a-uri
```

---

## 4.11 Review: DR without secondary region

**File:** `test/fixtures/requests/review/dr-no-secondary-region.yaml`

```yaml
apiVersion: platform.gatblau.io/v1alpha1
kind: InfrastructureRequest
metadata:
  name: dr-no-secondary-region
  tenant: acme
spec:
  owner: team-data
  environment: production
  workload:
    type: analytics-platform
    profile: medium
    exposure: internal
  capabilities:
    kubernetes:
      enabled: true
    database:
      enabled: true
      engine: postgres
    objectStorage:
      enabled: true
  location:
    residency: eu
    primaryRegion: eu-west-1
  operations:
    availability: high
    drRequired: true
```

---

## 4.12 Review: unsupported compliance combo

**File:** `test/fixtures/requests/review/unsupported-compliance-combo.yaml`

```yaml
apiVersion: platform.gatblau.io/v1alpha1
kind: InfrastructureRequest
metadata:
  name: unsupported-compliance-combo
  tenant: acme
spec:
  owner: team-finance
  environment: production
  workload:
    type: analytics-platform
    exposure: internal
  capabilities:
    kubernetes:
      enabled: true
    database:
      enabled: true
      engine: postgres
    objectStorage:
      enabled: true
  location:
    residency: eu
    primaryRegion: eu-west-1
  security:
    compliance:
      - gdpr
      - pci
```

---

## 4.13 Review: region outside allow list

**File:** `test/fixtures/requests/review/region-not-in-allow-list.yaml`

```yaml
apiVersion: platform.gatblau.io/v1alpha1
kind: InfrastructureRequest
metadata:
  name: region-not-in-allow-list
  tenant: acme
spec:
  owner: team-data
  environment: production
  workload:
    type: analytics-platform
    exposure: internal
  capabilities:
    kubernetes:
      enabled: true
    database:
      enabled: true
      engine: postgres
    objectStorage:
      enabled: true
  location:
    residency: eu
    primaryRegion: us-east-1
  security:
    compliance:
      - gdpr
```

---

# 5. Expected validation outputs

---

## 5.1 Expected validation: analytics-prod-eu

**File:** `test/fixtures/expected/validation/analytics-prod-eu.validation.yaml`

```yaml
apiVersion: platform.gatblau.io/v1
kind: ValidationResult
result:
  valid: true
  status: valid
  errors: []
  warnings: []
  derived:
    blastRadius: high
    approvalRequired: true
    matchedTemplate: analytics-platform-prod
```

---

## 5.2 Expected validation: missing-owner

**File:** `test/fixtures/expected/validation/missing-owner.validation.yaml`

```yaml
apiVersion: platform.gatblau.io/v1
kind: ValidationResult
result:
  valid: false
  status: invalid
  errors:
    - code: REQUIRED_FIELD_MISSING
      message: spec.owner is required
      field: spec.owner
      severity: error
  warnings: []
```

---

## 5.3 Expected validation: dr-no-secondary-region

**File:** `test/fixtures/expected/validation/dr-no-secondary-region.validation.yaml`

```yaml
apiVersion: platform.gatblau.io/v1
kind: ValidationResult
result:
  valid: false
  status: allow_with_approval
  errors: []
  warnings:
    - code: SECONDARY_REGION_REQUIRED
      message: secondary region is required when DR is enabled
      field: spec.location.secondaryRegion
      severity: warning
  derived:
    blastRadius: high
    approvalRequired: true
    matchedTemplate: analytics-platform-prod
```

---

# 6. Expected compiled plans

---

## 6.1 Expected compiled: analytics-prod-eu

**File:** `test/fixtures/expected/compiled/analytics-prod-eu.compiled.yaml`

```yaml
apiVersion: platform.gatblau.io/v1
kind: CompiledProvisioningPlan
metadata:
  name: analytics-prod-eu
  tenant: acme
summary:
  template: analytics-platform-prod
  blastRadius: high
  estimatedMonthlyCost: 2475
  currency: GBP
source:
  requestHash: sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
  catalogVersion: "2026-03-15"
  policyVersion: "2026-03-15.1"
  compilerVersion: "1.0.0"
resolution:
  namespace: team-data
  compositionRef: xrd.analytics.platform/v1
  gitStrategy: pull-request
  verifyCloud: true
job:
  tenant_id: acme
  path_key: golden-path/infra/provision-crossplane
  idempotency_key: infrareq-analytics-prod-eu-v1-aaaaaaaa
  params:
    contract_version: infra-provision-crossplane/v1
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
        request_hash: sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
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

## 6.2 Expected compiled: web-dev-private

**File:** `test/fixtures/expected/compiled/web-dev-private.compiled.yaml`

```yaml
apiVersion: platform.gatblau.io/v1
kind: CompiledProvisioningPlan
metadata:
  name: web-dev-private
  tenant: acme
summary:
  template: web-service-dev
  blastRadius: low
  estimatedMonthlyCost: 170
  currency: GBP
source:
  requestHash: sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb
  catalogVersion: "2026-03-15"
  policyVersion: "2026-03-15.1"
  compilerVersion: "1.0.0"
resolution:
  namespace: team-web
  compositionRef: xrd.web.service/v1
  gitStrategy: direct-commit
  verifyCloud: false
job:
  tenant_id: acme
  path_key: golden-path/infra/provision-crossplane
  idempotency_key: infrareq-web-dev-private-v1-bbbbbbbb
  params:
    contract_version: infra-provision-crossplane/v1
    template_name: web-service-dev
    composition_ref: xrd.web.service/v1
    namespace: team-web
    git_repo: acme/infra-live
    git_branch: main
    git_strategy: direct-commit
    verify_cloud: false
    cloud_resource_type: composite
    approvalContext:
      required: false
      decisionBasis:
        policyOutcome: allow
      riskSummary:
        blastRadius: low
        estimatedCostBand: low
      reviewContext:
        request_name: web-dev-private
        tenant: acme
        owner: team-web
        environment: development
        template: web-service-dev
        composition_ref: xrd.web.service/v1
        estimated_cost: "170"
        currency: GBP
        compiler_version: "1.0.0"
        catalog_version: "2026-03-15"
        policy_version: "2026-03-15.1"
        request_hash: sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb
    parameters:
      request_name: web-dev-private
      tenant: acme
      owner: team-web
      environment: development
      workload_type: web-service
      workload_profile: small
      workload_exposure: private
      residency: uk
      primary_region: uk-west-2
      secondary_region: ""
      kubernetes_enabled: true
      kubernetes_tier: standard
      kubernetes_size: small
      kubernetes_multi_az: false
      database_enabled: false
      database_engine: ""
      database_tier: ""
      database_storage_gib: 0
      database_backup_enabled: false
      database_backup_retention_days: 0
      object_storage_enabled: false
      object_storage_class: ""
      object_storage_versioning: false
      messaging_enabled: false
      messaging_tier: ""
      compliance: []
      ingress_mode: private
      egress_mode: controlled
      data_classification: internal
      availability: best-effort
      backup_required: false
      dr_required: false
      monitoring: basic
      cost_max_monthly: 0
      cost_currency: GBP
```

---

## 6.3 Expected compiled: api-staging-internal

**File:** `test/fixtures/expected/compiled/api-staging-internal.compiled.yaml`

```yaml
apiVersion: platform.gatblau.io/v1
kind: CompiledProvisioningPlan
metadata:
  name: api-staging-internal
  tenant: acme
summary:
  template: api-service-staging
  blastRadius: medium
  estimatedMonthlyCost: 730
  currency: GBP
source:
  requestHash: sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc
  catalogVersion: "2026-03-15"
  policyVersion: "2026-03-15.1"
  compilerVersion: "1.0.0"
resolution:
  namespace: team-api
  compositionRef: xrd.api.service/v1
  gitStrategy: pull-request
  verifyCloud: true
job:
  tenant_id: acme
  path_key: golden-path/infra/provision-crossplane
  idempotency_key: infrareq-api-staging-internal-v1-cccccccc
  params:
    contract_version: infra-provision-crossplane/v1
    template_name: api-service-staging
    composition_ref: xrd.api.service/v1
    namespace: team-api
    git_repo: acme/infra-live
    git_branch: main
    git_strategy: pull-request
    verify_cloud: true
    cloud_resource_type: composite
    approvalContext:
      required: false
      decisionBasis:
        policyOutcome: allow
      riskSummary:
        blastRadius: medium
        estimatedCostBand: low
      reviewContext:
        request_name: api-staging-internal
        tenant: acme
        owner: team-api
        environment: staging
        template: api-service-staging
        composition_ref: xrd.api.service/v1
        estimated_cost: "730"
        currency: GBP
        compiler_version: "1.0.0"
        catalog_version: "2026-03-15"
        policy_version: "2026-03-15.1"
        request_hash: sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc
    parameters:
      request_name: api-staging-internal
      tenant: acme
      owner: team-api
      environment: staging
      workload_type: api-service
      workload_profile: medium
      workload_exposure: internal
      residency: eu
      primary_region: eu-west-1
      secondary_region: ""
      kubernetes_enabled: true
      kubernetes_tier: standard
      kubernetes_size: medium
      kubernetes_multi_az: false
      database_enabled: true
      database_engine: postgres
      database_tier: standard
      database_storage_gib: 200
      database_backup_enabled: true
      database_backup_retention_days: 30
      object_storage_enabled: false
      object_storage_class: ""
      object_storage_versioning: false
      messaging_enabled: false
      messaging_tier: ""
      compliance:
        - gdpr
      ingress_mode: internal
      egress_mode: controlled
      data_classification: internal
      availability: standard
      backup_required: true
      dr_required: false
      monitoring: standard
      cost_max_monthly: 0
      cost_currency: GBP
```

---

## 6.4 Expected compiled equivalence

For `equivalent-ordering-a.yaml` and `equivalent-ordering-b.yaml`, the expected compiled output should be **identical except for source file name**.

A practical way to test this:

- do **not** keep two different golden output files
- instead compare:
    - canonical normalized requests
    - compiled plan structs
    - idempotency keys
    - request hashes

---

# 7. Expected submission payload

## 7.1 Example submit payload

**File:** `test/fixtures/expected/submit/analytics-prod-eu.job.yaml`

```yaml
tenant_id: acme
path_key: golden-path/infra/provision-crossplane
idempotency_key: infrareq-analytics-prod-eu-v1-aaaaaaaa
params:
  contract_version: infra-provision-crossplane/v1
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
      request_hash: sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
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

# 8. Golden-test plan

---

## 8.1 Schema validation tests

**Purpose:** ensure malformed requests fail with stable diagnostics.

Cases:

- missing required field
- enum violation
- pattern violation
- URI format failure
- unsupported property

### Assertions
- `valid == false`
- exact error codes
- exact field paths
- no unexpected warnings

---

## 8.2 Semantic validation tests

**Purpose:** test rules beyond JSON Schema.

Cases:

- DR without secondary region
- primary region not in allow list
- DB engine not permitted by matched template
- production public ingress forbidden by strict policy
- compliance requiring approval

### Assertions
- exact status:
    - `allow`
    - `allow_with_approval`
    - `deny`
- exact diagnostic code
- matched template if available

---

## 8.3 Compiler golden tests

**Purpose:** detect output drift.

Cases:

- compile each valid request fixture
- serialize to canonical YAML or JSON
- compare to expected golden file

### Assertions
- entire compiled plan matches golden
- source metadata populated
- summary cost and blast radius stable
- all expected params present

---

## 8.4 Canonicalization tests

**Purpose:** ensure logically equivalent input produces identical output.

Cases:

- `equivalent-ordering-a.yaml`
- `equivalent-ordering-b.yaml`

### Assertions
- normalized request hash equal
- compiled plan deep-equal
- idempotency key equal
- submit payload equal

---

## 8.5 Submission payload tests

**Purpose:** verify adapter contract to Release Engine.

Cases:

- compile valid request
- convert to submit job payload
- compare with `expected/submit/*.job.yaml`

### Assertions
- `tenant_id`
- `path_key`
- `idempotency_key`
- complete `params`
- callback behavior if applicable

---

## 8.6 Approval derivation tests

**Purpose:** ensure policy logic is stable.

Cases:

- production request => approval required
- dev request low cost => no approval
- high estimated cost => approval required
- allow_with_approval and approval required both true if policy demands it

### Assertions
- `approvalContext.required`
- `approvalContext.ttl.expiresAt`
- `approvalContext.suggestedApproverRoles`
- `approvalContext.reviewContext` content

---

# 9. Canonical serialization rules for testing

To make golden files reliable, standardize output before compare.

## 9.1 Rules

1. sort all map keys recursively
2. sort semantically unordered arrays:
    - compliance lists
    - labels if represented as list
3. preserve semantically ordered arrays if any exist later
4. normalize empty optional strings to `""`
5. normalize absent booleans to explicit `false` in compiled output
6. normalize absent numerics to `0` in compiled output when contract expects explicit values
7. use stable YAML marshaling or canonical JSON

## 9.2 Preferred compare format

Best practice:

- internal compare: **canonical JSON bytes**
- fixture authoring: **YAML**

That gives humans readable files and machines deterministic comparisons.

---

# 10. Go test skeleton

Below is a practical starter skeleton.

---

## 10.1 Test helper structure

**File:** `test/golden/helpers_test.go`

```go
package golden

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %s: %v", path, err)
	}
	return b
}

func mustLoadYAML[T any](t *testing.T, path string) T {
	t.Helper()
	var v T
	if err := yaml.Unmarshal(mustReadFile(t, path), &v); err != nil {
		t.Fatalf("unmarshal yaml %s: %v", path, err)
	}
	return v
}

func mustCanonicalJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshal canonical json: %v", err)
	}
	return b
}

func fixturePath(parts ...string) string {
	all := append([]string{"..", "fixtures"}, parts...)
	return filepath.Join(all...)
}
```

---

## 10.2 Validation golden test

**File:** `test/golden/validate_test.go`

```go
package golden

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	infra "github.com/gatblau/release-engine/infra-intent/pkg/api"
)

func TestValidateGolden(t *testing.T) {
	type tc struct {
		name         string
		requestFile  string
		expectedFile string
		catalogFile  string
		policyFile   string
	}

	tests := []tc{
		{
			name:         "analytics-prod-eu",
			requestFile:  fixturePath("requests", "valid", "analytics-prod-eu.yaml"),
			expectedFile: fixturePath("expected", "validation", "analytics-prod-eu.validation.yaml"),
			catalogFile:  fixturePath("catalog", "capability-catalog.base.yaml"),
			policyFile:   fixturePath("policies", "policy-bundle.base.yaml"),
		},
		{
			name:         "missing-owner",
			requestFile:  fixturePath("requests", "invalid", "missing-owner.yaml"),
			expectedFile: fixturePath("expected", "validation", "missing-owner.validation.yaml"),
			catalogFile:  fixturePath("catalog", "capability-catalog.base.yaml"),
			policyFile:   fixturePath("policies", "policy-bundle.base.yaml"),
		},
		{
			name:         "dr-no-secondary-region",
			requestFile:  fixturePath("requests", "review", "dr-no-secondary-region.yaml"),
			expectedFile: fixturePath("expected", "validation", "dr-no-secondary-region.validation.yaml"),
			catalogFile:  fixturePath("catalog", "capability-catalog.base.yaml"),
			policyFile:   fixturePath("policies", "policy-bundle.strict.yaml"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := mustLoadYAML[infra.InfrastructureRequest](t, tt.requestFile)
			catalog := mustLoadYAML[infra.CapabilityCatalog](t, tt.catalogFile)
			policy := mustLoadYAML[infra.PolicyBundle](t, tt.policyFile)
			expected := mustLoadYAML[infra.ValidationResult](t, tt.expectedFile)

			got, err := infra.Validate(req, catalog, policy)
			if err != nil {
				t.Fatalf("validate: %v", err)
			}

			if diff := cmp.Diff(expected, got); diff != "" {
				t.Fatalf("validation mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
```

---

## 10.3 Compile golden test

**File:** `test/golden/compile_test.go`

```go
package golden

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	infra "github.com/gatblau/release-engine/infra-intent/pkg/api"
)

func TestCompileGolden(t *testing.T) {
	type tc struct {
		name         string
		requestFile  string
		expectedFile string
		catalogFile  string
		policyFile   string
	}

	tests := []tc{
		{
			name:         "analytics-prod-eu",
			requestFile:  fixturePath("requests", "valid", "analytics-prod-eu.yaml"),
			expectedFile: fixturePath("expected", "compiled", "analytics-prod-eu.compiled.yaml"),
			catalogFile:  fixturePath("catalog", "capability-catalog.base.yaml"),
			policyFile:   fixturePath("policies", "policy-bundle.base.yaml"),
		},
		{
			name:         "web-dev-private",
			requestFile:  fixturePath("requests", "valid", "web-dev-private.yaml"),
			expectedFile: fixturePath("expected", "compiled", "web-dev-private.compiled.yaml"),
			catalogFile:  fixturePath("catalog", "capability-catalog.base.yaml"),
			policyFile:   fixturePath("policies", "policy-bundle.base.yaml"),
		},
		{
			name:         "api-staging-internal",
			requestFile:  fixturePath("requests", "valid", "api-staging-internal.yaml"),
			expectedFile: fixturePath("expected", "compiled", "api-staging-internal.compiled.yaml"),
			catalogFile:  fixturePath("catalog", "capability-catalog.base.yaml"),
			policyFile:   fixturePath("policies", "policy-bundle.base.yaml"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := mustLoadYAML[infra.InfrastructureRequest](t, tt.requestFile)
			catalog := mustLoadYAML[infra.CapabilityCatalog](t, tt.catalogFile)
			policy := mustLoadYAML[infra.PolicyBundle](t, tt.policyFile)
			expected := mustLoadYAML[infra.CompiledProvisioningPlan](t, tt.expectedFile)

			opts := infra.CompilerOptions{
				CompilerVersion: "1.0.0",
				PathKey:         "golden-path/infra/provision-crossplane",
				GitRepoResolver: func(tenant string) string { return tenant + "/infra-live" },
				GitBranchResolver: func(tenant string) string { return "main" },
				HashOverride: func(name string) string {
					switch name {
					case "analytics-prod-eu":
						return "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					case "web-dev-private":
						return "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
					case "api-staging-internal":
						return "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
					default:
						return "sha256:deadbeef"
					}
				},
				IdempotencyKeyOverride: func(name string) string {
					switch name {
					case "analytics-prod-eu":
						return "infrareq-analytics-prod-eu-v1-aaaaaaaa"
					case "web-dev-private":
						return "infrareq-web-dev-private-v1-bbbbbbbb"
					case "api-staging-internal":
						return "infrareq-api-staging-internal-v1-cccccccc"
					default:
						return "infrareq-unknown"
					}
				},
			}

			got, err := infra.Compile(req, catalog, policy, opts)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}

			if diff := cmp.Diff(expected, got); diff != "" {
				t.Fatalf("compiled mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
```

---

## 10.4 Canonicalization equivalence test

**File:** `test/golden/canonicalize_test.go`

```go
package golden

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	infra "github.com/gatblau/release-engine/infra-intent/pkg/api"
)

func TestEquivalentInputsCompileIdentically(t *testing.T) {
	reqA := mustLoadYAML[infra.InfrastructureRequest](t, fixturePath("requests", "valid", "equivalent-ordering-a.yaml"))
	reqB := mustLoadYAML[infra.InfrastructureRequest](t, fixturePath("requests", "valid", "equivalent-ordering-b.yaml"))
	catalog := mustLoadYAML[infra.CapabilityCatalog](t, fixturePath("catalog", "capability-catalog.base.yaml"))
	policy := mustLoadYAML[infra.PolicyBundle](t, fixturePath("policies", "policy-bundle.base.yaml"))

	opts := infra.CompilerOptions{
		CompilerVersion: "1.0.0",
		PathKey:         "golden-path/infra/provision-crossplane",
		GitRepoResolver: func(tenant string) string { return tenant + "/infra-live" },
		GitBranchResolver: func(tenant string) string { return "main" },
	}

	normA, err := infra.Normalize(reqA)
	if err != nil {
		t.Fatalf("normalize A: %v", err)
	}
	normB, err := infra.Normalize(reqB)
	if err != nil {
		t.Fatalf("normalize B: %v", err)
	}

	if diff := cmp.Diff(normA, normB); diff != "" {
		t.Fatalf("normalized requests differ (-a +b):\n%s", diff)
	}

	planA, err := infra.Compile(reqA, catalog, policy, opts)
	if err != nil {
		t.Fatalf("compile A: %v", err)
	}
	planB, err := infra.Compile(reqB, catalog, policy, opts)
	if err != nil {
		t.Fatalf("compile B: %v", err)
	}

	if diff := cmp.Diff(planA, planB); diff != "" {
		t.Fatalf("compiled plans differ (-a +b):\n%s", diff)
	}

	if planA.Job.IdempotencyKey != planB.Job.IdempotencyKey {
		t.Fatalf("idempotency keys differ: %s != %s", planA.Job.IdempotencyKey, planB.Job.IdempotencyKey)
	}

	if planA.Source.RequestHash != planB.Source.RequestHash {
		t.Fatalf("request hashes differ: %s != %s", planA.Source.RequestHash, planB.Source.RequestHash)
	}
}
```

---

## 10.5 Submission payload golden test

**File:** `test/golden/submit_test.go`

```go
package golden

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	infra "github.com/gatblau/release-engine/infra-intent/pkg/api"
)

func TestSubmitPayloadGolden(t *testing.T) {
	req := mustLoadYAML[infra.InfrastructureRequest](t, fixturePath("requests", "valid", "analytics-prod-eu.yaml"))
	catalog := mustLoadYAML[infra.CapabilityCatalog](t, fixturePath("catalog", "capability-catalog.base.yaml"))
	policy := mustLoadYAML[infra.PolicyBundle](t, fixturePath("policies", "policy-bundle.base.yaml"))
	expected := mustLoadYAML[infra.ReleaseEngineJobRequest](t, fixturePath("expected", "submit", "analytics-prod-eu.job.yaml"))

	opts := infra.CompilerOptions{
		CompilerVersion: "1.0.0",
		PathKey:         "golden-path/infra/provision-crossplane",
		GitRepoResolver: func(tenant string) string { return tenant + "/infra-live" },
		GitBranchResolver: func(tenant string) string { return "main" },
		HashOverride: func(name string) string {
			return "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		},
		IdempotencyKeyOverride: func(name string) string {
			return "infrareq-analytics-prod-eu-v1-aaaaaaaa"
		},
	}

	plan, err := infra.Compile(req, catalog, policy, opts)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	got := infra.ToReleaseEngineJobRequest(plan)

	if diff := cmp.Diff(expected, got); diff != "" {
		t.Fatalf("submit payload mismatch (-want +got):\n%s", diff)
	}
}
```

---

# 11. Suggested API types for the test harness

These are the minimal structs your tests expect.

```go
type InfrastructureRequest struct { /* ... */ }
type CapabilityCatalog struct { /* ... */ }
type PolicyBundle struct { /* ... */ }
type ValidationResult struct { /* ... */ }
type CompiledProvisioningPlan struct { /* ... */ }

type ReleaseEngineJobRequest struct {
	TenantID       string         `yaml:"tenant_id" json:"tenant_id"`
	PathKey        string         `yaml:"path_key" json:"path_key"`
	IdempotencyKey string         `yaml:"idempotency_key" json:"idempotency_key"`
	Params         map[string]any `yaml:"params" json:"params"`
}

type CompilerOptions struct {
	CompilerVersion        string
	PathKey                string
	GitRepoResolver        func(tenant string) string
	GitBranchResolver      func(tenant string) string
	HashOverride           func(name string) string
	IdempotencyKeyOverride func(name string) string
}

func Normalize(req InfrastructureRequest) (InfrastructureRequest, error)
func Validate(req InfrastructureRequest, catalog CapabilityCatalog, policy PolicyBundle) (ValidationResult, error)
func Compile(req InfrastructureRequest, catalog CapabilityCatalog, policy PolicyBundle, opts CompilerOptions) (CompiledProvisioningPlan, error)
func ToReleaseEngineJobRequest(plan CompiledProvisioningPlan) ReleaseEngineJobRequest
```

---

# 12. Regression checklist

Use the golden tests to catch these common regressions:

- accidental param rename
- changing default values silently
- dropping explicit false/zero fields
- approval logic drift
- cost estimate drift
- map ordering instability
- idempotency key instability
- request hash instability
- namespace resolution drift
- template match ambiguity

---

# 13. Recommended CI stages

## Stage 1 — Fast validation
Run on every PR:

- unit tests
- schema tests
- semantic validation tests
- canonicalization tests

## Stage 2 — Golden compile tests
Run on every PR:

- compiled plan golden files
- submit payload golden files

## Stage 3 — Contract compatibility
Run on protected branches:

- compare produced params against module contract
- ensure required keys remain present
- ensure `contract_version` still supported

## Stage 4 — End-to-end smoke
Run nightly or pre-release:

- mock Release Engine submission
- dry-run compile/submit flow
- optional ephemeral test environment

---

# 14. Strong recommendation on golden-file maintenance

When behavior changes intentionally:

1. update compiler/version notes
2. regenerate goldens
3. review diffs manually
4. require approver sign-off on:
    - contract changes
    - approval logic changes
    - cost logic changes

This prevents "golden drift" from hiding breaking changes.
