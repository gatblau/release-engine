# Infra Module API Contract (`infra.provision`)

## Overview

This document describes the **actual current API contract** for the Infra module implemented in `internal/module/infra`.

It focuses on:

- module identity and runtime behavior,
- expected input parameter shape,
- supported infrastructure capabilities,
- validation and error model,
- output/context produced by the module.

---

## 1) Module Identity

- **Module key:** `infra.provision`
- **Module version:** `latest`
- **Primary execution step:** `infra.render`

Execution flow:

1. Decode `map[string]any` input into `template.ProvisionParams`.
2. Load embedded catalog definitions.
3. Apply catalog defaults and constraints.
4. Run global + cross-cutting + fragment validations.
5. Render deterministic YAML Crossplane XR manifest.
6. Publish output manifest to step context.

---

## 2) Input Contract

The module accepts a map payload decoded by YAML tags into `template.ProvisionParams`.

### 2.1 Top-Level Parameters

| Field | Type | Required | Notes |
|---|---|---:|---|
| `contract_version` | string | Yes | Added to XR annotation |
| `request_name` | string | Yes | Used as XR metadata name |
| `tenant` | string | Yes | Label + tagging context |
| `owner` | string | Yes | Tagging context |
| `environment` | string | Yes | Validated against catalog constraints |
| `workload_profile` | string | No* | Required for some catalog constraints |
| `template_name` | string | Yes | Must match an embedded catalog name |
| `composition_ref` | string | Yes* | Can be defaulted from catalog |
| `namespace` | string | Yes | Used as XR namespace |
| `residency` | string | No* | Required for some catalog constraints |
| `primary_region` | string | Yes | Global validation |
| `secondary_region` | string | Conditional | Required when `dr_required=true` |
| `availability` | string | Yes | e.g. `standard`, `high`, `critical` |
| `data_classification` | string | Yes | e.g. `confidential`, `restricted` |
| `compliance` | []string | No | Consumed by compliance fragment |
| `workload_type` | string | No | Used by some fragments |
| `workload_exposure` | string | No | Used by CDN validation |
| `ingress_mode` | string | Yes | Used by load balancer validation |
| `egress_mode` | string | Yes | Global validation |
| `dr_required` | bool | No | Cross-field validation |
| `backup_required` | bool | No | Cross-field validation |
| `extra_tags` | map[string]string | No | Additional tag enrichment |

\* Conditionally required depending on catalog/rule paths.

---

## 3) Supported Capability Blocks

Each capability uses nested objects with `enabled` and capability-specific fields.

- `kubernetes`
- `vm`
- `database`
- `object_store`
- `block_store`
- `file_store`
- `vpc`
- `messaging`
- `cache`
- `dns`
- `load_balancer`
- `cdn`
- `identity`
- `secrets`
- `observability`

Always-on policy fragments also contribute:

- `tags`
- `compliance`

### 3.1 Capability Presence Rule

At least one capability above must be enabled, otherwise render fails with:

`at least one infrastructure capability must be enabled`

---

## 4) Catalog Contract (Embedded)

Catalog definitions are embedded under `internal/module/infra/template/catalog/definitions`.

Current catalog names:

- `k8s-app`
- `vm-app`
- `data-proc`

Catalog provides:

- `composition_ref`
- required / optional / forbidden capabilities
- constraints (`allowed_environments`, `allowed_workload_profiles`, `allowed_availabilities`, `allowed_residencies`)
- defaults (e.g., `kubernetes_tier`, `database_engine`, `cache_engine`, `object_storage_versioning`)

If `template_name` does not match a loaded catalog, render fails with:

`catalog "<name>" not found`

---

## 5) Validation and Policy Rules

Validation happens in four stages:

1. **Catalog resolution/defaulting**
2. **Global validation**
3. **Cross-cutting validation**
4. **Fragment-level validation**

### 5.1 Key Global / Cross-Cutting Rules

- `secondary_region` required when `dr_required=true`
- `dr_required=true` when `availability=critical`
- `backup_required=true` when `availability=critical`
- `backup_required=true` when `data_classification=restricted`
- `vm.enabled` and `kubernetes.enabled` cannot both be true
- `block_store.enabled` requires `vm.enabled=true`
- `availability=critical` requires `observability.enabled=true`
- Catalog forbidden/required capabilities are enforced

### 5.2 Fragment-Level Validation Examples

- Kubernetes requires `kubernetes.tier` and `kubernetes.size` when enabled
- VM requires `vm.count`, `vm.instance_family`, `vm.size`, `vm.os_family`
- Database requires `database.engine`, `database.tier`, `database.storage_gib`
- DNS requires `dns.zone_name` when enabled
- Load balancer requires `load_balancer.type` and `load_balancer.scheme`

---

## 6) Output Contract

Successful render produces deterministic YAML for a Crossplane XR with:

- `apiVersion: infrastructure.platform.io/v1alpha1`
- `kind: InfrastructureRequest`

Structure:

- `metadata.name = request_name`
- `metadata.namespace = namespace`
- `metadata.labels` include tenant/environment/template
- `metadata.annotations` include contract/composition refs
- `spec.compositionRef.name = composition_ref`
- `spec.parameters` merged from applicable fragments

---

## 7) Step API Integration

During module execution:

- `BeginStep("infra.render")`
- on success: `EndStepOK("infra.render", {"manifest_yaml": "..."})`
- on failure: `EndStepErr("infra.render", <code>, <message>)`
- context key set on success: `infra.manifest` (string YAML)

Error codes emitted by module:

- `INFRA_PARAMS_INVALID` (decode/unmarshal failure)
- `INFRA_RENDER_FAILED` (catalog/validation/fragment/render errors)

---

## 8) Example Minimal Valid Payload

```yaml
contract_version: v1
request_name: checkout-prod
tenant: payments
owner: platform-team
environment: production
workload_profile: medium
template_name: k8s-app
composition_ref: ""
namespace: platform-system
residency: eu
primary_region: eu-west-1
secondary_region: eu-central-1
availability: high
data_classification: confidential
ingress_mode: public
egress_mode: nat
dr_required: true
backup_required: true
kubernetes:
  enabled: true
  size: medium
```

Notes:

- `composition_ref` may be empty and default from catalog.
- `kubernetes.tier` may default from catalog for `k8s-app`.

---

## 9) Compatibility Notes

- Parameter decoding is permissive via YAML unmarshalling from map payload.
- Unknown fields may be ignored unless consumed by future struct fields.
- Catalog names are centralized and must remain stable identifiers across code/tests.

---

## 10) Source of Truth

For exact behavior, refer to:

- `internal/module/infra/module.go`
- `internal/module/infra/render.go`
- `internal/module/infra/template/types.go`
- `internal/module/infra/template/validate.go`
- `internal/module/infra/template/engine.go`
- `internal/module/infra/template/catalog/*`
- `internal/module/infra/template/fragments/*`
