# Crossplane Connector — Pseudo Code

The connector encapsulates all interactions with Crossplane running inside a Kubernetes cluster. It is used by modules that need to provision, observe, and tear down managed infrastructure resources through Crossplane's Composition and Claims API surface. Given the asynchronous and eventually-consistent nature of Crossplane reconciliation, all operations are designed with deep observability in mind — surfacing condition arrays, sync/ready states, events, and health checks as first-class outputs. All operations return one of `Success`, `RetryableError`, or `TerminalError`.

---

## Interface

```
CONNECTOR: CrossplaneConnector
implements Connector interface

registered name: "crossplane"

func Call(ctx, op, params, credential) -> ConnectorResult:
  client = resolve_client(credential)
  // credential: {
  //   kubeconfig (base64) | in_cluster (bool),
  //   context (optional),
  //   namespace (optional, default: "default")
  // }

  switch op:
    // Claims
    case "apply_claim":                   return apply_claim(ctx, client, params)
    case "get_claim":                     return get_claim(ctx, client, params)
    case "delete_claim":                  return delete_claim(ctx, client, params)
    case "list_claims":                   return list_claims(ctx, client, params)
    case "wait_for_claim_ready":          return wait_for_claim_ready(ctx, client, params)

    // Composite Resources (XR)
    case "get_composite":                 return get_composite(ctx, client, params)
    case "list_composites":               return list_composites(ctx, client, params)
    case "delete_composite":              return delete_composite(ctx, client, params)

    // Managed Resources (MR)
    case "get_managed_resource":          return get_managed_resource(ctx, client, params)
    case "list_managed_resources":        return list_managed_resources(ctx, client, params)
    case "annotate_managed_resource":     return annotate_managed_resource(ctx, client, params)

    // Compositions
    case "get_composition":               return get_composition(ctx, client, params)
    case "list_compositions":             return list_compositions(ctx, client, params)
    case "apply_composition":             return apply_composition(ctx, client, params)
    case "delete_composition":            return delete_composition(ctx, client, params)

    // XRDs
    case "get_xrd":                       return get_xrd(ctx, client, params)
    case "list_xrds":                     return list_xrds(ctx, client, params)
    case "apply_xrd":                     return apply_xrd(ctx, client, params)
    case "delete_xrd":                    return delete_xrd(ctx, client, params)
    case "wait_for_xrd_established":      return wait_for_xrd_established(ctx, client, params)

    // Providers
    case "get_provider":                  return get_provider(ctx, client, params)
    case "list_providers":                return list_providers(ctx, client, params)
    case "apply_provider":                return apply_provider(ctx, client, params)
    case "delete_provider":               return delete_provider(ctx, client, params)
    case "get_provider_config":           return get_provider_config(ctx, client, params)
    case "apply_provider_config":         return apply_provider_config(ctx, client, params)
    case "delete_provider_config":        return delete_provider_config(ctx, client, params)
    case "wait_for_provider_healthy":     return wait_for_provider_healthy(ctx, client, params)

    // Packages
    case "get_function":                  return get_function(ctx, client, params)
    case "list_functions":                return list_functions(ctx, client, params)
    case "apply_function":                return apply_function(ctx, client, params)
    case "delete_function":               return delete_function(ctx, client, params)

    // Observability and Troubleshooting
    case "get_resource_health":           return get_resource_health(ctx, client, params)
    case "list_resource_events":          return list_resource_events(ctx, client, params)
    case "get_resource_tree":             return get_resource_tree(ctx, client, params)
    case "list_provider_pod_logs":        return list_provider_pod_logs(ctx, client, params)
    case "describe_resource":             return describe_resource(ctx, client, params)
    case "check_readiness_conditions":    return check_readiness_conditions(ctx, client, params)
    case "list_failed_managed_resources": return list_failed_managed_resources(ctx, client, params)
    case "get_last_reconcile_error":      return get_last_reconcile_error(ctx, client, params)
    case "force_reconcile":               return force_reconcile(ctx, client, params)

    default:
      return TerminalError("unknown operation: " + op)
```

---

## Internal Helpers

```
// Shared condition extraction — used by all operations that return
// Crossplane or Kubernetes standard conditions.

func extract_conditions(conditions[]) -> ConditionSummary:
  return conditions.map(c => {
    type:               c.type,          // Ready | Synced | Healthy | Offered | Established
    status:             c.status,        // True | False | Unknown
    reason:             c.reason,
    message:            c.message,
    last_transition:    c.lastTransitionTime,
  })

func is_ready(conditions[]) -> bool:
  ready = conditions.find(c => c.type == "Ready")
  return ready is not null and ready.status == "True"

func is_synced(conditions[]) -> bool:
  synced = conditions.find(c => c.type == "Synced")
  return synced is not null and synced.status == "True"

func extract_sync_error(conditions[]) -> string | null:
  synced = conditions.find(c => c.type == "Synced" and c.status == "False")
  if synced is null: return null
  return "{synced.reason}: {synced.message}"

func resource_ref(group, version, resource, namespace, name) -> string:
  if namespace is not null:
    return "{group}/{version}/{resource}/{namespace}/{name}"
  return "{group}/{version}/{resource}/{name}"  // cluster-scoped
```

---

## Supported Operations

### Claim Operations

---

#### `apply_claim`
> Creates or updates a Crossplane Claim (namespaced XRC). This is the primary entry point for provisioning infrastructure through a Composition. Used in provisioning workflows where a developer or platform team requests a managed resource such as a database, bucket, or network without needing to know the underlying provider details.

```
func apply_claim(ctx, client, params):
  // params: api_version, kind, namespace, name, spec{}, labels{},
  //         annotations{}, composition_ref (optional),
  //         composition_selector (optional)

  manifest = {
    apiVersion: params.api_version,
    kind:       params.kind,
    metadata: {
      name:        params.name,
      namespace:   params.namespace,
      labels:      params.labels ?? {},
      annotations: params.annotations ?? {},
    },
    spec: params.spec,
  }

  if params.composition_ref is not null:
    manifest.spec.compositionRef = { name: params.composition_ref }

  if params.composition_selector is not null:
    manifest.spec.compositionSelector = {
      matchLabels: params.composition_selector,
    }

  resp = client.APPLY(
    "/apis/{params.api_version}/namespaces/{params.namespace}/{kind_plural(params.kind)}/{params.name}",
    manifest,
    strategy: "server-side-apply",
    field_manager: "release-engine",
  )

  if resp.status in [200, 201]:
    return Success({
      name:        params.name,
      namespace:   params.namespace,
      uid:         resp.body.metadata.uid,
      resource_version: resp.body.metadata.resourceVersion,
      created:     resp.status == 201,
    })

  if resp.status == 409:
    return RetryableError("conflict_during_apply — resourceVersion mismatch, retry")

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)
```

---

#### `get_claim`
> Fetches the current state of a Claim including its conditions, composite resource reference, and connection secret reference. The primary read operation for troubleshooting — surfaces Ready and Synced conditions along with any error messages from the Crossplane reconciler.

```
func get_claim(ctx, client, params):
  // params: api_version, kind, namespace, name

  resp = client.GET(
    "/apis/{params.api_version}/namespaces/{params.namespace}/{kind_plural(params.kind)}/{params.name}"
  )

  if resp.status == 404:
    return TerminalError("claim_not_found")

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  conditions = extract_conditions(resp.body.status.conditions ?? [])
  sync_error = extract_sync_error(resp.body.status.conditions ?? [])

  return Success({
    name:                params.name,
    namespace:           params.namespace,
    uid:                 resp.body.metadata.uid,
    resource_version:    resp.body.metadata.resourceVersion,
    ready:               is_ready(resp.body.status.conditions ?? []),
    synced:              is_synced(resp.body.status.conditions ?? []),
    sync_error:          sync_error,                 // null if clean
    conditions:          conditions,
    composite_ref:       resp.body.spec.resourceRef, // -> XR name
    connection_secret:   resp.body.spec.writeConnectionSecretToRef,
    observed_generation: resp.body.status.observedGeneration,
    labels:              resp.body.metadata.labels,
    annotations:         resp.body.metadata.annotations,
    raw:                 resp.body,
  })
```

---

#### `delete_claim`
> Deletes a Claim and propagates deletion to the composite resource and all managed resources via Crossplane's cascading garbage collection. Used in decommissioning workflows. Returns immediately — callers must poll `get_claim` until 404 to confirm full teardown.

```
func delete_claim(ctx, client, params):
  // params: api_version, kind, namespace, name,
  //         propagation_policy (Foreground|Background|Orphan) default: Foreground

  resp = client.DELETE(
    "/apis/{params.api_version}/namespaces/{params.namespace}/{kind_plural(params.kind)}/{params.name}",
    body: {
      propagationPolicy: params.propagation_policy ?? "Foreground",
    }
  )

  if resp.status == 404:
    return Success({ idempotent: true })

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({
    name:      params.name,
    namespace: params.namespace,
    deleted:   true,
  })
```

---

#### `list_claims`
> Lists all Claims of a given kind in a namespace with optional label selector. Used in audit, bulk teardown, and inventory workflows.

```
func list_claims(ctx, client, params):
  // params: api_version, kind, namespace, label_selector,
  //         field_selector, limit, continue_token

  resp = client.GET(
    "/apis/{params.api_version}/namespaces/{params.namespace}/{kind_plural(params.kind)}",
    query={
      labelSelector:  params.label_selector,
      fieldSelector:  params.field_selector,
      limit:          params.limit ?? 100,
      continue:       params.continue_token,
    }
  )

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({
    claims: resp.body.items.map(c => {
      name:          c.metadata.name,
      namespace:     c.metadata.namespace,
      ready:         is_ready(c.status.conditions ?? []),
      synced:        is_synced(c.status.conditions ?? []),
      sync_error:    extract_sync_error(c.status.conditions ?? []),
      composite_ref: c.spec.resourceRef,
    }),
    continue_token: resp.body.metadata.continue,
    total:          resp.body.metadata.remainingItemCount,
  })
```

---

#### `wait_for_claim_ready`
> Polls a Claim until its Ready condition is True, a terminal failure condition is detected, or the timeout is exceeded. The central gating operation in provisioning workflows — all downstream steps that depend on a live infrastructure resource must wait on this before proceeding. Surfaces the exact failure condition and message on timeout or error to aid troubleshooting.

```
func wait_for_claim_ready(ctx, client, params):
  // params: api_version, kind, namespace, name,
  //         timeout_seconds (default: 600),
  //         poll_interval_seconds (default: 15)

  deadline = now() + (params.timeout_seconds ?? 600)
  interval = params.poll_interval_seconds ?? 15

  while now() < deadline:
    result = get_claim(ctx, client, {
      api_version: params.api_version,
      kind:        params.kind,
      namespace:   params.namespace,
      name:        params.name,
    })

    if result is error: return result    // propagate RetryableError / TerminalError

    claim = result.data

    if claim.ready and claim.synced:
      return Success({
        name:          params.name,
        namespace:     params.namespace,
        ready:         true,
        conditions:    claim.conditions,
        composite_ref: claim.composite_ref,
      })

    // Terminal failure — do not keep polling
    if claim.synced == false and claim.sync_error is not null:
      if is_terminal_crossplane_reason(claim.conditions):
        return TerminalError({
          reason:     "claim_sync_failed_terminal",
          sync_error: claim.sync_error,
          conditions: claim.conditions,
        })

    sleep(interval)

  // Timeout — fetch final state snapshot for diagnostics
  final = get_claim(ctx, client, params)

  return RetryableError({
    reason:     "wait_for_claim_ready_timeout",
    elapsed_s:  params.timeout_seconds ?? 600,
    conditions: final.data.conditions,
    sync_error: final.data.sync_error,
    hint:       "run get_resource_tree and list_resource_events for diagnosis",
  })


func is_terminal_crossplane_reason(conditions[]) -> bool:
  terminal_reasons = [
    "ReconcileError",
    "CompositionNotFound",
    "CompositionRevisionNotFound",
    "CompositionInvalid",
    "FunctionResultInvalid",
    "ProviderConfigNotFound",
    "ManagedResourceCreationFailed",
    "CredentialsNotFound",
    "TooManyRetries",
  ]
  return conditions.any(c =>
    c.status == "False" and terminal_reasons.contains(c.reason)
  )
```

---

### Composite Resource (XR) Operations

---

#### `get_composite`
> Fetches the composite resource (cluster-scoped XR) that was created by a Claim. Contains the full spec as resolved by the Composition, the list of managed resource references, and the pipeline function results. The primary intermediate layer to inspect when a Claim is not becoming ready and the error message is insufficient.

```
func get_composite(ctx, client, params):
  // params: api_version, kind, name

  resp = client.GET(
    "/apis/{params.api_version}/{kind_plural(params.kind)}/{params.name}"
  )

  if resp.status == 404:
    return TerminalError("composite_not_found")

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  conditions   = extract_conditions(resp.body.status.conditions ?? [])
  sync_error   = extract_sync_error(resp.body.status.conditions ?? [])

  return Success({
    name:              params.name,
    uid:               resp.body.metadata.uid,
    composition_ref:   resp.body.spec.compositionRef,
    ready:             is_ready(resp.body.status.conditions ?? []),
    synced:            is_synced(resp.body.status.conditions ?? []),
    sync_error:        sync_error,
    conditions:        conditions,
    resource_refs:     resp.body.spec.resourceRefs ?? [],   // -> managed resources
    connection_details_last_published: resp.body.status.connectionDetails?.lastPublishedTime,
    pipeline_results:  resp.body.status.pipelineResults ?? [],
    raw:               resp.body,
  })
```

---

#### `list_composites`
> Lists composite resources of a given kind cluster-wide with optional label selectors. Used in bulk audit, ownership mapping, and orphan-detection workflows.

```
func list_composites(ctx, client, params):
  // params: api_version, kind, label_selector, limit, continue_token

  resp = client.GET(
    "/apis/{params.api_version}/{kind_plural(params.kind)}",
    query={
      labelSelector: params.label_selector,
      limit:         params.limit ?? 100,
      continue:      params.continue_token,
    }
  )

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({
    composites: resp.body.items.map(x => {
      name:          x.metadata.name,
      ready:         is_ready(x.status.conditions ?? []),
      synced:        is_synced(x.status.conditions ?? []),
      sync_error:    extract_sync_error(x.status.conditions ?? []),
      resource_refs: x.spec.resourceRefs ?? [],
      claim_ref:     x.spec.claimRef,
    }),
    continue_token: resp.body.metadata.continue,
  })
```

---

#### `delete_composite`
> Deletes a composite resource directly (bypasses the Claim layer). Used only in recovery workflows where a Claim has already been deleted but the composite resource is orphaned and must be manually removed.

```
func delete_composite(ctx, client, params):
  // params: api_version, kind, name, propagation_policy

  resp = client.DELETE(
    "/apis/{params.api_version}/{kind_plural(params.kind)}/{params.name}",
    body: { propagationPolicy: params.propagation_policy ?? "Foreground" }
  )

  if resp.status == 404:
    return Success({ idempotent: true })

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({ name: params.name, deleted: true })
```

---

### Managed Resource (MR) Operations

---

#### `get_managed_resource`
> Fetches a single managed resource by group/version/resource and name. The lowest level of the Crossplane object hierarchy — this is where provider-level errors surface. Used in troubleshooting workflows when a Claim or XR is stuck and the root cause must be traced to the actual cloud API call failure.

```
func get_managed_resource(ctx, client, params):
  // params: group, version, resource (plural), name

  resp = client.GET(
    "/apis/{params.group}/{params.version}/{params.resource}/{params.name}"
  )

  if resp.status == 404:
    return TerminalError("managed_resource_not_found")

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  conditions = extract_conditions(resp.body.status.conditions ?? [])

  return Success({
    name:                params.name,
    uid:                 resp.body.metadata.uid,
    provider_config_ref: resp.body.spec.providerConfigRef,
    deletion_policy:     resp.body.spec.deletionPolicy,    // Delete | Orphan
    ready:               is_ready(resp.body.status.conditions ?? []),
    synced:              is_synced(resp.body.status.conditions ?? []),
    sync_error:          extract_sync_error(resp.body.status.conditions ?? []),
    conditions:          conditions,
    at_provider:         resp.body.status.atProvider ?? {},  // cloud-side state
    external_name:       resp.body.metadata.annotations["crossplane.io/external-name"],
    last_reconcile_time: resp.body.status.conditions
                           .find(c => c.type == "Synced")
                           ?.lastTransitionTime,
    raw:                 resp.body,
  })
```

---

#### `list_managed_resources`
> Lists managed resources of a given type cluster-wide. Used in inventory, drift-detection, and orphan-cleanup workflows. Particularly useful when a composite resource has been deleted but provider-side resources remain due to a `deletionPolicy: Orphan` setting.

```
func list_managed_resources(ctx, client, params):
  // params: group, version, resource (plural),
  //         label_selector, field_selector, limit, continue_token

  resp = client.GET(
    "/apis/{params.group}/{params.version}/{params.resource}",
    query={
      labelSelector: params.label_selector,
      fieldSelector: params.field_selector,
      limit:         params.limit ?? 100,
      continue:      params.continue_token,
    }
  )

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({
    resources: resp.body.items.map(r => {
      name:          r.metadata.name,
      external_name: r.metadata.annotations["crossplane.io/external-name"],
      ready:         is_ready(r.status.conditions ?? []),
      synced:        is_synced(r.status.conditions ?? []),
      sync_error:    extract_sync_error(r.status.conditions ?? []),
      deletion_policy: r.spec.deletionPolicy,
      provider_config: r.spec.providerConfigRef?.name,
    }),
    continue_token: resp.body.metadata.continue,
  })
```

---

#### `annotate_managed_resource`
> Adds or updates annotations on a managed resource. Used in recovery workflows to set `crossplane.io/paused: "true"` to halt reconciliation, or to override `crossplane.io/external-name` to re-attach an orphaned cloud resource.

```
func annotate_managed_resource(ctx, client, params):
  // params: group, version, resource, name, annotations{}
  // Common annotation keys:
  //   "crossplane.io/paused"         -> "true" | "false"
  //   "crossplane.io/external-name"  -> cloud resource ID
  //   "crossplane.io/external-create-pending" -> RFC3339 timestamp

  patch = {
    metadata: {
      annotations: params.annotations,
    },
  }

  resp = client.PATCH(
    "/apis/{params.group}/{params.version}/{params.resource}/{params.name}",
    patch,
    content_type: "application/merge-patch+json",
  )

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({
    name:        params.name,
    annotations: resp.body.metadata.annotations,
  })
```

---

### Composition Operations

---

#### `get_composition`
> Fetches a Composition definition including its pipeline steps and patch-and-transform rules. Used in troubleshooting workflows to inspect which function pipeline or P&T rules are applied to a composite resource, and to detect version mismatches between the Composition and the XRD.

```
func get_composition(ctx, client, params):
  // params: name

  resp = client.GET("/apis/apiextensions.crossplane.io/v1/compositions/{params.name}")

  if resp.status == 404:
    return TerminalError("composition_not_found")

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({
    name:              params.name,
    composite_kind:    resp.body.spec.compositeTypeRef,
    mode:              resp.body.spec.mode,          // Resources | Pipeline
    pipeline_steps:    resp.body.spec.pipeline ?? [],
    resources:         resp.body.spec.resources ?? [],
    write_connection_secrets_to_namespace: resp.body.spec.writeConnectionSecretsToNamespace,
    raw:               resp.body,
  })
```

---

#### `list_compositions`
> Lists all Composition resources in the cluster with optional label filtering. Used in governance workflows to audit which Compositions are available and to detect unused or superseded Compositions before performing a cleanup.

```
func list_compositions(ctx, client, params):
  // params: label_selector, limit, continue_token

  resp = client.GET(
    "/apis/apiextensions.crossplane.io/v1/compositions",
    query={
      labelSelector: params.label_selector,
      limit:         params.limit ?? 100,
      continue:      params.continue_token,
    }
  )

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({
    compositions: resp.body.items.map(c => {
      name:           c.metadata.name,
      composite_kind: c.spec.compositeTypeRef,
      mode:           c.spec.mode,
    }),
    continue_token: resp.body.metadata.continue,
  })
```

---

#### `apply_composition`
> Creates or updates a Composition using server-side apply. Used in platform engineering workflows to publish new or revised Compositions as part of a GitOps-driven infrastructure release.

```
func apply_composition(ctx, client, params):
  // params: manifest (full Composition object)

  resp = client.APPLY(
    "/apis/apiextensions.crossplane.io/v1/compositions/{params.manifest.metadata.name}",
    params.manifest,
    strategy:      "server-side-apply",
    field_manager: "release-engine",
  )

  if resp.status == 409:
    return RetryableError("apply_conflict — retry with latest resourceVersion")

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({
    name:    resp.body.metadata.name,
    uid:     resp.body.metadata.uid,
    created: resp.status == 201,
  })
```

---

#### `delete_composition`
> Deletes a Composition resource. Existing composite resources that reference this Composition will enter a `Synced=False` state until reassigned. Used in cleanup workflows after migrating all consumers to a new Composition version.

```
func delete_composition(ctx, client, params):
  // params: name

  resp = client.DELETE(
    "/apis/apiextensions.crossplane.io/v1/compositions/{params.name}"
  )

  if resp.status == 404:
    return Success({ idempotent: true })

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({ name: params.name, deleted: true })
```

---

### Composite Resource Definition (XRD) Operations

---

#### `get_xrd`
> Fetches an XRD and its establishment status. Used in pre-flight checks before submitting a Claim to confirm that the XRD is fully established and its CRDs have been installed into the cluster API surface.

```
func get_xrd(ctx, client, params):
  // params: name

  resp = client.GET(
    "/apis/apiextensions.crossplane.io/v1/compositeresourcedefinitions/{params.name}"
  )

  if resp.status == 404:
    return TerminalError("xrd_not_found")

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  conditions = extract_conditions(resp.body.status.conditions ?? [])

  established = conditions.find(c => c.type == "Established")
  offered     = conditions.find(c => c.type == "Offered")

  return Success({
    name:              params.name,
    group:             resp.body.spec.group,
    claim_kind:        resp.body.spec.claimNames?.kind,
    composite_kind:    resp.body.spec.names.kind,
    versions:          resp.body.spec.versions.map(v => v.name),
    established:       established?.status == "True",
    offered:           offered?.status == "True",
    conditions:        conditions,
    established_error: established?.status != "True" ? established?.message : null,
    raw:               resp.body,
  })
```

---

#### `wait_for_xrd_established`
> Polls an XRD until its Established condition is True or the timeout is exceeded. Used after applying a new or updated XRD to gate subsequent Claim submissions on full CRD installation. A common source of timing failures in automated provisioning pipelines.

```
func wait_for_xrd_established(ctx, client, params):
  // params: name,
  //         timeout_seconds (default: 120),
  //         poll_interval_seconds (default: 10)

  deadline = now() + (params.timeout_seconds ?? 120)
  interval = params.poll_interval_seconds ?? 10

  while now() < deadline:
    result = get_xrd(ctx, client, { name: params.name })
    if result is TerminalError: return result

    if result.data.established:
      return Success({
        name:        params.name,
        established: true,
        conditions:  result.data.conditions,
      })

    sleep(interval)

  final = get_xrd(ctx, client, { name: params.name })

  return RetryableError({
    reason:     "wait_for_xrd_established_timeout",
    elapsed_s:  params.timeout_seconds ?? 120,
    conditions: final.data.conditions,
    error:      final.data.established_error,
    hint:       "check that the XRD schema is valid and that crossplane core is healthy",
  })
```

---

### Provider Operations

---

#### `get_provider`
> Fetches a Provider package resource and its installation and health status. Used in pre-flight and troubleshooting workflows to confirm that the provider responsible for a given managed resource type is installed, healthy, and running the expected version.

```
func get_provider(ctx, client, params):
  // params: name

  resp = client.GET(
    "/apis/pkg.crossplane.io/v1/providers/{params.name}"
  )

  if resp.status == 404:
    return TerminalError("provider_not_found")

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  conditions = extract_conditions(resp.body.status.conditions ?? [])

  healthy   = conditions.find(c => c.type == "Healthy")
  installed = conditions.find(c => c.type == "Installed")

  return Success({
    name:             params.name,
    package:          resp.body.spec.package,
    revision_history: resp.body.spec.revisionHistoryLimit,
    healthy:          healthy?.status == "True",
    installed:        installed?.status == "True",
    conditions:       conditions,
    health_error:     healthy?.status != "True" ? healthy?.message : null,
    install_error:    installed?.status != "True" ? installed?.message : null,
    current_revision: resp.body.status.currentRevision,
    raw:              resp.body,
  })
```

---

#### `wait_for_provider_healthy`
> Polls a Provider until its Healthy condition is True or the timeout is exceeded. Used after installing or upgrading a provider to gate resource provisioning on a confirmed healthy provider pod. Surfaces pod-level log hints on timeout.

```
func wait_for_provider_healthy(ctx, client, params):
  // params: name,
  //         timeout_seconds (default: 180),
  //         poll_interval_seconds (default: 15)

  deadline = now() + (params.timeout_seconds ?? 180)
  interval = params.poll_interval_seconds ?? 15

  while now() < deadline:
    result = get_provider(ctx, client, { name: params.name })
    if result is TerminalError: return result

    if result.data.healthy and result.data.installed:
      return Success({
        name:       params.name,
        healthy:    true,
        conditions: result.data.conditions,
      })

    sleep(interval)

  final = get_provider(ctx, client, { name: params.name })

  return RetryableError({
    reason:        "wait_for_provider_healthy_timeout",
    elapsed_s:     params.timeout_seconds ?? 180,
    health_error:  final.data.health_error,
    install_error: final.data.install_error,
    conditions:    final.data.conditions,
    hint:          "run list_provider_pod_logs to inspect provider container output",
  })
```

---

#### `apply_provider`
> Creates or updates a Provider package resource. Used in platform bootstrapping and provider upgrade workflows.

```
func apply_provider(ctx, client, params):
  // params: name, package (OCI image ref), revision_history_limit,
  //         install_mode (Automatic|Manual), skip_dependency_resolution

  manifest = {
    apiVersion: "pkg.crossplane.io/v1",
    kind:       "Provider",
    metadata: { name: params.name },
    spec: {
      package:                   params.package,
      revisionHistoryLimit:      params.revision_history_limit ?? 1,
      installationPolicy:        params.install_mode ?? "Automatic",
      skipDependencyResolution:  params.skip_dependency_resolution ?? false,
    },
  }

  resp = client.APPLY(
    "/apis/pkg.crossplane.io/v1/providers/{params.name}",
    manifest,
    strategy:      "server-side-apply",
    field_manager: "release-engine",
  )

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({
    name:    params.name,
    package: params.package,
    created: resp.status == 201,
  })
```

---

#### `get_provider_config`
> Fetches a ProviderConfig resource and its usage status. Used in troubleshooting workflows to verify that a ProviderConfig exists and is correctly referencing the expected credential secret or IAM role before a managed resource reconcile is attempted.

```
func get_provider_config(ctx, client, params):
  // params: group (e.g. aws.upbound.io), version, name

  resp = client.GET(
    "/apis/{params.group}/{params.version}/providerconfigs/{params.name}"
  )

  if resp.status == 404:
    return TerminalError("provider_config_not_found")

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  conditions = extract_conditions(resp.body.status.conditions ?? [])

  return Success({
    name:        params.name,
    credentials: resp.body.spec.credentials,
    conditions:  conditions,
    users:       resp.body.status.users ?? 0,   // number of MRs referencing this config
    ready:       is_ready(resp.body.status.conditions ?? []),
    raw:         resp.body,
  })
```

---

### Observability and Troubleshooting Operations

These operations form the diagnostic layer of the connector. They are designed to be called from troubleshooting modules and automated remediation workflows that need to understand why a resource is stuck.

---

#### `get_resource_health`
> Returns a consolidated health snapshot for a resource at any level of the hierarchy — Claim, XR, or Managed Resource. Aggregates Ready, Synced, and Healthy conditions into a single structured summary with a derived top-level status. The recommended first call in any automated troubleshooting workflow.

```
func get_resource_health(ctx, client, params):
  // params: api_version, kind, name, namespace (null for cluster-scoped)

  if params.namespace is not null:
    resp = client.GET(
      "/apis/{params.api_version}/namespaces/{params.namespace}/{kind_plural(params.kind)}/{params.name}"
    )
  else:
    resp = client.GET(
      "/apis/{params.api_version}/{kind_plural(params.kind)}/{params.name}"
    )

  if resp.status == 404:
    return TerminalError("resource_not_found")

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  conditions = extract_conditions(resp.body.status.conditions ?? [])
  ready      = is_ready(resp.body.status.conditions ?? [])
  synced     = is_synced(resp.body.status.conditions ?? [])
  sync_error = extract_sync_error(resp.body.status.conditions ?? [])

  // Derive a single top-level health status for workflow routing
  if ready and synced:
    health_status = "healthy"
  else if not synced and sync_error is not null:
    health_status = "sync_failed"
  else if not ready and synced:
    health_status = "not_ready"
  else:
    health_status = "unknown"

  return Success({
    resource_ref:    resource_ref(params.api_version, "", params.kind,
                                  params.namespace, params.name),
    health_status:   health_status,         // healthy | sync_failed | not_ready | unknown
    ready:           ready,
    synced:          synced,
    sync_error:      sync_error,
    conditions:      conditions,
    generation:      resp.body.metadata.generation,
    observed_generation: resp.body.status.observedGeneration,
    generation_lag:  resp.body.metadata.generation
                       - (resp.body.status.observedGeneration ?? 0),
    // generation_lag > 0 means the reconciler has not yet processed
    // the latest spec change — not necessarily an error
    paused:          resp.body.metadata.annotations
                       ["crossplane.io/paused"] == "true",
  })
```

---

#### `list_resource_events`
> Lists Kubernetes Events associated with a specific resource. Events contain human-readable messages from the Crossplane reconciler and the provider controller, including API call failures, quota errors, and permission denials. The most direct source of root-cause information for stuck or failed resources.

```
func list_resource_events(ctx, client, params):
  // params: namespace, name, uid, kind,
  //         limit (default: 50), warn_only (default: false)

  namespace = params.namespace ?? "default"

  field_selector = [
    "involvedObject.name={params.name}",
    "involvedObject.kind={params.kind}",
  ]

  if params.uid is not null:
    field_selector.push("involvedObject.uid={params.uid}")

  if params.warn_only:
    field_selector.push("type=Warning")

  resp = client.GET(
    "/api/v1/namespaces/{namespace}/events",
    query={
      fieldSelector: field_selector.join(","),
      limit:         params.limit ?? 50,
    }
  )

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({
    events: resp.body.items
      .sort_by(e => e.lastTimestamp, descending: true)
      .map(e => {
        type:           e.type,            // Normal | Warning
        reason:         e.reason,
        message:        e.message,
        source:         e.source.component,
        count:          e.count,
        first_time:     e.firstTimestamp,
        last_time:      e.lastTimestamp,
      }),
    warning_count: resp.body.items.filter(e => e.type == "Warning").length,
  })
```

---

#### `get_resource_tree`
> Walks the full ownership tree from a Claim down through its XR to all managed resources, returning health status at each level. The canonical troubleshooting operation for locating exactly which resource in the hierarchy is failing. Equivalent to running `crossplane beta trace` from the CLI.

```
func get_resource_tree(ctx, client, params):
  // params: claim_api_version, claim_kind, claim_namespace, claim_name

  // Level 1: Claim
  claim_result = get_claim(ctx, client, {
    api_version: params.claim_api_version,
    kind:        params.claim_kind,
    namespace:   params.claim_namespace,
    name:        params.claim_name,
  })
  if claim_result is error: return claim_result
  claim = claim_result.data

  tree = {
    claim: {
      name:          claim.name,
      namespace:     claim.namespace,
      ready:         claim.ready,
      synced:        claim.synced,
      sync_error:    claim.sync_error,
      conditions:    claim.conditions,
    },
    composite:        null,
    managed_resources: [],
  }

  if claim.composite_ref is null:
    tree.diagnosis = "claim_has_no_composite_ref — " +
      "composition may not have matched or XRD is not established"
    return Success(tree)

  // Level 2: Composite Resource
  xr_result = get_composite(ctx, client, {
    api_version: claim.composite_ref.apiVersion,
    kind:        claim.composite_ref.kind,
    name:        claim.composite_ref.name,
  })
  if xr_result is error: return xr_result
  xr = xr_result.data

  tree.composite = {
    name:          xr.name,
    ready:         xr.ready,
    synced:        xr.synced,
    sync_error:    xr.sync_error,
    conditions:    xr.conditions,
    resource_refs: xr.resource_refs,
    pipeline_results: xr.pipeline_results,
  }

  if xr.resource_refs is empty:
    tree.diagnosis = "composite_has_no_resource_refs — " +
      "composition pipeline may have failed before producing resources"
    return Success(tree)

  // Level 3: Managed Resources
  for ref in xr.resource_refs:
    mr_result = get_managed_resource(ctx, client, {
      group:    parse_group(ref.apiVersion),
      version:  parse_version(ref.apiVersion),
      resource: kind_plural(ref.kind),
      name:     ref.name,
    })

    if mr_result is error:
      tree.managed_resources.push({
        name:   ref.name,
        kind:   ref.kind,
        error:  mr_result.error,
      })
      continue

    mr = mr_result.data
    tree.managed_resources.push({
      name:          mr.name,
      kind:          ref.kind,
      external_name: mr.external_name,
      ready:         mr.ready,
      synced:        mr.synced,
      sync_error:    mr.sync_error,
      conditions:    mr.conditions,
      at_provider:   mr.at_provider,
    })

  // Derive a diagnosis hint
  tree.diagnosis = derive_tree_diagnosis(tree)
  return Success(tree)


func derive_tree_diagnosis(tree) -> string:
  if not tree.claim.synced:
    return "claim_sync_failed: " + tree.claim.sync_error

  if tree.composite is null:
    return "no_composite_created"

  if not tree.composite.synced:
    return "composite_sync_failed: " + tree.composite.sync_error

  failed_mrs = tree.managed_resources.filter(mr => not mr.synced or not mr.ready)
  if failed_mrs is not empty:
    summary = failed_mrs.map(mr => "{mr.name}: {mr.sync_error}").join("; ")
    return "managed_resource_failures: " + summary

  if tree.claim.ready and tree.composite.ready
      and tree.managed_resources.all(mr => mr.ready):
    return "fully_healthy"

  return "partial_readiness — some resources not yet ready"
```

---

#### `list_provider_pod_logs`
> Fetches recent log lines from the provider controller pod. Provider pods emit structured log lines containing the actual error returned by the cloud API — credential failures, missing permissions, quota exhaustion, invalid field values — that do not always surface in Kubernetes Events or resource conditions. The deepest troubleshooting layer available without direct cluster access.

```
func list_provider_pod_logs(ctx, client, params):
  // params: provider_name, tail_lines (default: 200),
  //         since_seconds (default: 300),
  //         filter_level (error|warn|info — optional, applied client-side),
  //         managed_resource_name (optional, used for client-side grep)

  // Step 1: resolve provider pod via ProviderRevision -> Deployment -> Pod
  revision_resp = client.GET(
    "/apis/pkg.crossplane.io/v1/providers/{params.provider_name}"
  )
  if revision_resp.status != 200:
    return TerminalError("provider_not_found_for_log_fetch")

  current_revision = revision_resp.body.status.currentRevision
  if current_revision is null:
    return TerminalError("provider_has_no_current_revision")

  // Step 2: find pods owned by the current revision
  pod_resp = client.GET(
    "/api/v1/namespaces/crossplane-system/pods",
    query={ labelSelector: "pkg.crossplane.io/revision={current_revision}" }
  )

  if pod_resp.status != 200:
    return RetryableError("failed_to_list_provider_pods")

  pods = pod_resp.body.items
  if pods is empty:
    return TerminalError("no_pods_found_for_provider_revision: " + current_revision)

  pod_name = pods[0].metadata.name    // take first running pod

  // Step 3: fetch logs
  log_resp = client.GET(
    "/api/v1/namespaces/crossplane-system/pods/{pod_name}/log",
    query={
      tailLines:    params.tail_lines ?? 200,
      sinceSeconds: params.since_seconds ?? 300,
    }
  )

  if log_resp.status in [500, 502, 503, 429]:
    return RetryableError(log_resp.error)

  if log_resp.status >= 400:
    return TerminalError(log_resp.error)

  lines = log_resp.body.split("\n")
    .filter(l => l is not empty)
    .map(l => try_parse_json(l) ?? { raw: l })

  // Client-side filtering
  if params.filter_level is not null:
    lines = lines.filter(l => l.level == params.filter_level or
                               l.severity == params.filter_level)

  if params.managed_resource_name is not null:
    lines = lines.filter(l =>
      l.raw?.contains(params.managed_resource_name) or
      l["managed-resource"]?.contains(params.managed_resource_name)
    )

  return Success({
    provider_name:    params.provider_name,
    pod_name:         pod_name,
    revision:         current_revision,
    line_count:       lines.length,
    lines:            lines,
    error_lines:      lines.filter(l => l.level == "error" or l.severity == "error"),
  })
```

---

#### `describe_resource`
> Returns a comprehensive diagnostic snapshot for a resource combining its full spec and status, all associated Kubernetes Events, and a health summary. Designed to be the single call in an automated incident triage workflow that needs to hand off a complete picture of a failed resource to an on-call engineer or an LLM-based remediation system.

```
func describe_resource(ctx, client, params):
  // params: api_version, kind, name, namespace,
  //         include_events (default: true),
  //         include_managed_resources (default: false)

  health = get_resource_health(ctx, client, {
    api_version: params.api_version,
    kind:        params.kind,
    name:        params.name,
    namespace:   params.namespace,
  })
  if health is error: return health

  result = {
    health:  health.data,
    events:  [],
    managed_resources: [],
  }

  if params.include_events ?? true:
    events = list_resource_events(ctx, client, {
      namespace: params.namespace ?? "crossplane-system",
      name:      params.name,
      kind:      params.kind,
    })
    if events is Success:
      result.events = events.data.events

  if params.include_managed_resources ?? false:
    tree = get_resource_tree(ctx, client, {
      claim_api_version: params.api_version,
      claim_kind:        params.kind,
      claim_namespace:   params.namespace,
      claim_name:        params.name,
    })
    if tree is Success:
      result.managed_resources = tree.data.managed_resources
      result.diagnosis          = tree.data.diagnosis

  return Success(result)
```

---

#### `check_readiness_conditions`
> Evaluates all conditions on a resource against a caller-supplied acceptance policy. Returns a structured pass/fail report rather than a simple boolean. Used in gating workflows where different condition combinations represent different acceptable states depending on the pipeline stage.

```
func check_readiness_conditions(ctx, client, params):
  // params: api_version, kind, name, namespace,
  //         required_conditions[]: [{ type, status, reason (optional) }]
  //         forbidden_conditions[]: [{ type, status }]

  health = get_resource_health(ctx, client, params)
  if health is error: return health

  conditions = health.data.conditions
  report     = { passed: [], failed: [], forbidden_matched: [] }
  all_passed = true

  for req in (params.required_conditions ?? []):
    actual = conditions.find(c => c.type == req.type)
    match  = actual is not null and
             actual.status == req.status and
             (req.reason is null or actual.reason == req.reason)

    if match:
      report.passed.push({ type: req.type, actual: actual })
    else:
      report.failed.push({ type: req.type, required: req, actual: actual })
      all_passed = false

  for forb in (params.forbidden_conditions ?? []):
    actual = conditions.find(c => c.type == forb.type and c.status == forb.status)
    if actual is not null:
      report.forbidden_matched.push({ type: forb.type, actual: actual })
      all_passed = false

  return Success({
    passed:    all_passed,
    report:    report,
    conditions: conditions,
  })
```

---

#### `list_failed_managed_resources`
> Scans all managed resources of one or more types and returns only those that are in a failed or not-synced state. Used in scheduled health-check workflows and alerting pipelines to detect drifted or broken resources across the entire cluster without walking individual resource trees.

```
func list_failed_managed_resources(ctx, client, params):
  // params: resources[]: [{ group, version, resource }],
  //         include_paused (default: false)

  failed = []

  for r in params.resources:
    result = list_managed_resources(ctx, client, {
      group:    r.group,
      version:  r.version,
      resource: r.resource,
    })

    if result is error: continue      // log and proceed to next type

    for mr in result.data.resources:
      is_paused = mr.annotations?["crossplane.io/paused"] == "true"

      if is_paused and not (params.include_paused ?? false):
        continue

      if not mr.ready or not mr.synced:
        failed.push({
          resource_type: r.resource,
          name:          mr.name,
          external_name: mr.external_name,
          ready:         mr.ready,
          synced:        mr.synced,
          sync_error:    mr.sync_error,
          provider_config: mr.provider_config,
          paused:        is_paused,
        })

  return Success({
    failed_count: failed.length,
    failed:       failed,
  })
```

---

#### `get_last_reconcile_error`
> Extracts the most recent reconciliation error from a resource's Synced condition message and enriches it with the last Kubernetes Event warning. Produces a single, consolidated error string suitable for inclusion in a notification, incident ticket, or automated remediation prompt.

```
func get_last_reconcile_error(ctx, client, params):
  // params: api_version, kind, name, namespace

  health = get_resource_health(ctx, client, params)
  if health is error: return health

  events = list_resource_events(ctx, client, {
    namespace:  params.namespace ?? "crossplane-system",
    name:       params.name,
    kind:       params.kind,
    warn_only:  true,
    limit:      5,
  })

  synced_condition = health.data.conditions.find(c => c.type == "Synced")
  ready_condition  = health.data.conditions.find(c => c.type == "Ready")

  last_event_message = events.data.events[0]?.message ?? null

  reconcile_error = health.data.sync_error
                    ?? last_event_message
                    ?? "no_error_detected"

  return Success({
    resource_ref:       resource_ref(params.api_version, "", params.kind,
                                     params.namespace, params.name),
    health_status:      health.data.health_status,
    reconcile_error:    reconcile_error,
    synced_condition:   synced_condition,
    ready_condition:    ready_condition,
    last_warning_event: events.data.events[0] ?? null,
    paused:             health.data.paused,
    generation_lag:     health.data.generation_lag,
  })
```

---

#### `force_reconcile`
> Triggers an immediate reconciliation of a managed resource by bumping an annotation. Crossplane does not expose a direct reconcile endpoint — the standard workaround is to modify a no-op annotation to increment the resource's generation and force the controller to requeue it. Used in recovery workflows after a transient provider error has been resolved externally.

```
func force_reconcile(ctx, client, params):
  // params: group, version, resource, name

  timestamp = now().iso8601()

  result = annotate_managed_resource(ctx, client, {
    group:    params.group,
    version:  params.version,
    resource: params.resource,
    name:     params.name,
    annotations: {
      "release-engine.io/force-reconcile": timestamp,
    },
  })

  if result is error: return result

  return Success({
    name:              params.name,
    reconcile_trigger: timestamp,
    note: "annotation bumped — crossplane will requeue on next watch event",
  })
```

---

## Error Classification Reference

| HTTP Status | Classification | Engine Behaviour |
|---|---|---|
| `2xx` | Success | Advance to next step |
| `404` on delete | Success (idempotent) | Already absent — treat as done |
| `404` on get | TerminalError | Resource does not exist — fail the step |
| `409` conflict on apply | RetryableError | resourceVersion mismatch — retry with backoff |
| `422` unprocessable | TerminalError | Schema violation — manifest is malformed |
| `429` rate limited | RetryableError | Re-enqueue with backoff |
| `4xx` other | TerminalError | Do not retry — fail the step |
| `5xx` | RetryableError | Re-enqueue with exponential backoff |
| Network timeout | RetryableError | Re-enqueue with backoff |

---

## Crossplane-Specific Condition Reference

| Condition | Type | Meaning |
|---|---|---|
| `Ready=True` | All resources | Resource exists on the provider and is operational |
| `Ready=False` | All resources | Resource creation failed or cloud-side health check failed |
| `Synced=True` | All resources | Last reconcile loop completed without error |
| `Synced=False` | All resources | Reconciler encountered an error — check `message` for root cause |
| `Established=True` | XRD | CRDs derived from the XRD have been installed into the cluster |
| `Offered=True` | XRD | Claim CRD has been offered — Claims of this type can now be submitted |
| `Healthy=True` | Provider | Provider pod is running and its package constraints are satisfied |
| `Installed=True` | Provider | Provider package has been unpacked and its CRDs installed |

---

## Troubleshooting Decision Tree

```
Claim not ready
│
├── claim.synced == false
│     └── read claim.sync_error
│           ├── "CompositionNotFound"      → apply_composition or fix compositionRef
│           ├── "CompositionInvalid"       → inspect composition pipeline/resources
│           ├── "FunctionResultInvalid"    → get_composition → check pipeline steps
│           └── "ReconcileError"           → go deeper ↓
│
├── claim.composite_ref == null
│     └── XRD may not be established      → wait_for_xrd_established
│
├── composite.synced == false
│     └── read composite.sync_error
│           └── "ReconcileError"           → go deeper ↓
│
├── managed_resource.synced == false
│     ├── read mr.sync_error              → provider-level error
│     ├── list_resource_events            → check Warning events for API message
│     ├── list_provider_pod_logs          → grep for mr.name — find raw cloud error
│     └── get_provider_config             → verify credentials reference is correct
│
├── managed_resource.ready == false
│     └── resource created but not healthy
│           ├── check mr.at_provider      → cloud-side state fields
│           └── list_resource_events      → look for health check failures
│
├── generation_lag > 0
│     └── spec changed but reconciler has not caught up
│           └── wait or force_reconcile
│
└── resource paused
      └── annotate_managed_resource       → remove crossplane.io/paused annotation
```

---

## Notes

**Server-Side Apply.** All `apply_*` operations use Kubernetes Server-Side Apply with `fieldManager: release-engine`. This ensures that the engine owns only the fields it explicitly sets, allowing other controllers (such as Crossplane itself) to manage the remaining fields without conflict.

**Cluster-Scoped vs Namespaced.** Claims are namespaced resources. Composite resources, Compositions, XRDs, Providers, and Managed Resources are all cluster-scoped. The `namespace` parameter is omitted for cluster-scoped resources and the URL construction differs accordingly.

**Polling Architecture.** `wait_for_*` operations implement polling internally as a convenience for simple workflows. For long-running provisioning operations — such as a managed RDS instance that may take 15 minutes — the calling module should use the scheduler's native retry and lease renewal mechanisms rather than blocking inside a single `wait_for_claim_ready` call with a long timeout.

**Pause-Before-Delete Pattern.** In destructive workflows, annotate a managed resource with `crossplane.io/paused: "true"` before deletion to prevent the reconciler from re-creating it during the deletion window. Follow with `delete_claim` and then remove the annotation from the managed resource if orphaning is required.

**External Name Re-Attachment.** When an orphaned cloud resource must be re-adopted by a new managed resource, use `annotate_managed_resource` to set `crossplane.io/external-name` to the existing cloud resource ID before triggering reconciliation. This prevents Crossplane from creating a duplicate resource.