# ArgoCD Connector — Pseudo Code

The connector encapsulates all interactions with the ArgoCD API surface, with a focus on supporting troubleshooting, diagnostics, and remediation workflows in addition to standard deployment operations. All operations are idempotent by contract and return one of `Success`, `RetryableError`, or `TerminalError`.

---

## Interface

```
CONNECTOR: ArgoCDConnector
implements Connector interface

registered name: "argocd"

func Call(ctx, op, params, credential) -> ConnectorResult:
  client = resolve_client(credential)
  // credential: {
  //   base_url,
  //   token,                        // argocd API token (static or oidc)
  //   token_type (static|oidc),
  //   insecure_skip_verify (bool)   // for self-signed certs in internal clusters
  // }

  switch op:
    // Applications
    case "get_application":               return get_application(ctx, client, params)
    case "list_applications":             return list_applications(ctx, client, params)
    case "create_application":            return create_application(ctx, client, params)
    case "update_application":            return update_application(ctx, client, params)
    case "delete_application":            return delete_application(ctx, client, params)
    case "sync_application":              return sync_application(ctx, client, params)
    case "rollback_application":          return rollback_application(ctx, client, params)
    case "terminate_operation":           return terminate_operation(ctx, client, params)
    case "patch_application":             return patch_application(ctx, client, params)

    // Health and Status
    case "get_application_status":        return get_application_status(ctx, client, params)
    case "get_resource_tree":             return get_resource_tree(ctx, client, params)
    case "get_managed_resources":         return get_managed_resources(ctx, client, params)
    case "get_resource_events":           return get_resource_events(ctx, client, params)
    case "get_application_events":        return get_application_events(ctx, client, params)
    case "watch_application":             return watch_application(ctx, client, params)

    // Troubleshooting
    case "get_resource_diff":             return get_resource_diff(ctx, client, params)
    case "get_pod_logs":                  return get_pod_logs(ctx, client, params)
    case "exec_pod":                      return exec_pod(ctx, client, params)
    case "get_sync_windows":              return get_sync_windows(ctx, client, params)
    case "get_operation_state":           return get_operation_state(ctx, client, params)
    case "get_degraded_resources":        return get_degraded_resources(ctx, client, params)
    case "get_out_of_sync_resources":     return get_out_of_sync_resources(ctx, client, params)
    case "get_orphaned_resources":        return get_orphaned_resources(ctx, client, params)
    case "get_hook_status":               return get_hook_status(ctx, client, params)
    case "retry_operation":               return retry_operation(ctx, client, params)

    // AppProject
    case "get_project":                   return get_project(ctx, client, params)
    case "list_projects":                 return list_projects(ctx, client, params)
    case "create_project":                return create_project(ctx, client, params)
    case "update_project":                return update_project(ctx, client, params)
    case "delete_project":                return delete_project(ctx, client, params)
    case "get_project_events":            return get_project_events(ctx, client, params)
    case "list_project_links":            return list_project_links(ctx, client, params)

    // Repositories
    case "list_repositories":             return list_repositories(ctx, client, params)
    case "get_repository":                return get_repository(ctx, client, params)
    case "create_repository":             return create_repository(ctx, client, params)
    case "delete_repository":             return delete_repository(ctx, client, params)
    case "validate_repository_access":    return validate_repository_access(ctx, client, params)

    // Clusters
    case "list_clusters":                 return list_clusters(ctx, client, params)
    case "get_cluster":                   return get_cluster(ctx, client, params)
    case "create_cluster":                return create_cluster(ctx, client, params)
    case "delete_cluster":                return delete_cluster(ctx, client, params)
    case "rotate_cluster_auth":           return rotate_cluster_auth(ctx, client, params)
    case "invalidate_cluster_cache":      return invalidate_cluster_cache(ctx, client, params)

    // Certificates and Secrets
    case "list_certificates":             return list_certificates(ctx, client, params)
    case "create_certificate":            return create_certificate(ctx, client, params)
    case "delete_certificate":            return delete_certificate(ctx, client, params)
    case "list_repository_credentials":   return list_repository_credentials(ctx, client, params)
    case "create_repository_credential":  return create_repository_credential(ctx, client, params)
    case "delete_repository_credential":  return delete_repository_credential(ctx, client, params)

    // RBAC and Accounts
    case "list_accounts":                 return list_accounts(ctx, client, params)
    case "get_account":                   return get_account(ctx, client, params)
    case "can_i":                         return can_i(ctx, client, params)

    // Settings and Server
    case "get_server_info":               return get_server_info(ctx, client, params)
    case "get_settings":                  return get_settings(ctx, client, params)

    default:
      return TerminalError("unknown operation: " + op)
```

---

## Supported Operations

### Application Operations

---

#### `get_application`
> Fetches the full application spec and current state for a single ArgoCD application. Used as the entry point for most troubleshooting workflows to capture a complete snapshot of the application at a point in time.

```
func get_application(ctx, client, params):
  // params: name, project (optional — used to disambiguate if app names
  //         are not globally unique across projects), refresh (bool)

  query = {}
  if params.refresh:
    query.refresh = "hard"    // forces a git and cluster state refresh
                              // before returning — use with care in hot paths

  resp = client.GET("/api/v1/applications/{params.name}", query=query)

  if resp.status == 404:
    return TerminalError("application_not_found")

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  app = resp.body
  return Success({
    name:             app.metadata.name,
    project:          app.spec.project,
    namespace:        app.spec.destination.namespace,
    cluster:          app.spec.destination.server,
    repo_url:         app.spec.source.repoURL,
    target_revision:  app.spec.source.targetRevision,
    path:             app.spec.source.path,
    helm_chart:       app.spec.source.chart,
    sync_policy:      app.spec.syncPolicy,
    health_status:    app.status.health.status,
    health_message:   app.status.health.message,
    sync_status:      app.status.sync.status,
    operation_state:  app.status.operationState,
    conditions:       app.status.conditions,
    images:           app.status.summary.images,
    revision:         app.status.sync.revision,
    raw:              app,
  })
```

---

#### `list_applications`
> Lists ArgoCD applications with optional label selector, project, and sync/health status filters. Used in fleet-wide troubleshooting workflows to identify all applications in a degraded or out-of-sync state across one or more clusters.

```
func list_applications(ctx, client, params):
  // params: project, repo, labels{}, sync_status, health_status,
  //         cluster, namespace, limit, continue_token

  resp = client.GET("/api/v1/applications", query={
    projects:     params.project,
    repo:         params.repo,
    labels:       format_label_selector(params.labels),
    syncStates:   params.sync_status,      // Synced | OutOfSync | Unknown
    healthStates: params.health_status,    // Healthy | Progressing | Degraded |
                                           // Suspended | Missing | Unknown
    cluster:      params.cluster,
    namespace:    params.namespace,
    limit:        params.limit ?? 100,
    continue:     params.continue_token,
  })

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({
    applications: resp.body.items.map(app => {
      name:          app.metadata.name,
      project:       app.spec.project,
      health_status: app.status.health.status,
      sync_status:   app.status.sync.status,
      cluster:       app.spec.destination.server,
      namespace:     app.spec.destination.namespace,
    }),
    continue_token: resp.body.metadata.continue,
  })
```

---

#### `create_application`
> Creates a new ArgoCD application definition. Used in service provisioning workflows to register a newly created repository as a managed application and begin its deployment lifecycle.

```
func create_application(ctx, client, params):
  // params: name, project, repo_url, target_revision, path,
  //         destination_server, destination_namespace,
  //         sync_policy{automated, prune, self_heal, sync_options[]},
  //         helm{values_files[], values{}, release_name, version},
  //         kustomize{version, name_prefix, name_suffix, images[]},
  //         labels{}, annotations{}

  existing = get_application(ctx, client, { name: params.name })
  if existing is Success:
    return Success({ idempotent: true, name: params.name })

  app_manifest = {
    metadata: {
      name:        params.name,
      namespace:   "argocd",
      labels:      params.labels ?? {},
      annotations: params.annotations ?? {},
    },
    spec: {
      project: params.project ?? "default",
      source: {
        repoURL:        params.repo_url,
        targetRevision: params.target_revision ?? "HEAD",
        path:           params.path,
        helm:           params.helm,
        kustomize:      params.kustomize,
      },
      destination: {
        server:    params.destination_server,
        namespace: params.destination_namespace,
      },
      syncPolicy: build_sync_policy(params.sync_policy),
    },
  }

  resp = client.POST("/api/v1/applications", app_manifest)

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({
    name:    resp.body.metadata.name,
    project: resp.body.spec.project,
  })
```

---

#### `update_application`
> Performs a full replacement update of an application spec. Used in change workflows where source revision, destination, or sync policy must be modified. Prefer `patch_application` for targeted field updates.

```
func update_application(ctx, client, params):
  // params: name, spec (full application spec object)

  resp = client.PUT("/api/v1/applications/{params.name}", {
    metadata: { name: params.name },
    spec:     params.spec,
  })

  if resp.status == 404:
    return TerminalError("application_not_found")

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({ name: params.name })
```

---

#### `delete_application`
> Deletes an ArgoCD application and optionally cascades deletion to all managed Kubernetes resources. Used in decommissioning workflows. Cascade must be explicitly opted into to prevent accidental resource destruction.

```
func delete_application(ctx, client, params):
  // params: name, cascade (bool, default false), propagation_policy
  //         (foreground|background|orphan)

  if params.cascade == true and params.propagation_policy is null:
    return TerminalError("cascade_requires_propagation_policy")

  resp = client.DELETE("/api/v1/applications/{params.name}", query={
    cascade:            params.cascade ?? false,
    propagationPolicy:  params.propagation_policy,
  })

  if resp.status == 404:
    return Success({ idempotent: true })

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({ name: params.name, cascade: params.cascade ?? false })
```

---

#### `sync_application`
> Triggers a sync operation for an application. Used in deployment and remediation workflows to converge actual cluster state with the desired git state. Supports selective resource sync, dry-run, and forced replacement.

```
func sync_application(ctx, client, params):
  // params: name, revision, resources[]{group, kind, name, namespace},
  //         dry_run (bool), force (bool — triggers replace, not apply),
  //         prune (bool), apply_only (bool — skip hooks),
  //         retry_strategy{limit, backoff{duration, factor, max_duration}}

  resp = client.POST("/api/v1/applications/{params.name}/sync", {
    revision:  params.revision ?? "HEAD",
    dryRun:    params.dry_run ?? false,
    force:     params.force ?? false,
    prune:     params.prune ?? false,
    resources: params.resources,
    syncOptions: build_sync_options(params),
    retryStrategy: params.retry_strategy,
  })

  if resp.status == 404:
    return TerminalError("application_not_found")

  if resp.status == 409:
    // another operation already in progress
    return RetryableError("operation_in_progress — terminate first or wait")

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({
    name:           params.name,
    operation_state: resp.body.status.operationState,
    dry_run:        params.dry_run ?? false,
  })
```

---

#### `rollback_application`
> Rolls back an application to a specific history ID. Used in incident response workflows when the current revision is confirmed as the cause of a degradation and a previous known-good revision must be restored immediately.

```
func rollback_application(ctx, client, params):
  // params: name, history_id (integer — from app.status.history),
  //         dry_run (bool), prune (bool)
  // Note: rollback disables auto-sync if it is currently enabled.
  //       The calling module must re-enable auto-sync after confirming
  //       the rollback is stable if that is the desired end state.

  resp = client.POST("/api/v1/applications/{params.name}/rollback", {
    id:     params.history_id,
    dryRun: params.dry_run ?? false,
    prune:  params.prune ?? false,
  })

  if resp.status == 404:
    return TerminalError("application_not_found")

  if resp.status == 400:
    return TerminalError({
      reason: "invalid_history_id_or_rollback_not_supported",
      detail: resp.body.message,
    })

  if resp.status == 409:
    return RetryableError("operation_in_progress")

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({
    name:        params.name,
    history_id:  params.history_id,
    dry_run:     params.dry_run ?? false,
  })
```

---

#### `terminate_operation`
> Terminates a currently running sync or rollback operation. Used in troubleshooting workflows when a sync has stalled, is taking too long, or must be aborted to allow a corrective operation to proceed.

```
func terminate_operation(ctx, client, params):
  // params: name

  resp = client.DELETE("/api/v1/applications/{params.name}/operation")

  if resp.status == 404:
    return Success({ idempotent: true })   // no operation in progress

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({ name: params.name, terminated: true })
```

---

#### `patch_application`
> Applies a JSON patch or merge patch to an application spec. Used in targeted remediation workflows where only a specific field — such as target revision, a single helm value, or a sync option — must be changed without touching the rest of the spec.

```
func patch_application(ctx, client, params):
  // params: name, patch_type (json|merge|strategic),
  //         patch (raw patch object or array)

  resp = client.PATCH("/api/v1/applications/{params.name}", {
    name:      params.name,
    patch:     JSON.stringify(params.patch),
    patchType: params.patch_type ?? "merge",
  })

  if resp.status == 404:
    return TerminalError("application_not_found")

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({ name: params.name, patched: true })
```

---

### Health and Status Operations

---

#### `get_application_status`
> Returns a focused health and sync summary for an application. Used as a lightweight polling probe in deployment gating and incident triage workflows where only status fields are needed rather than the full application record.

```
func get_application_status(ctx, client, params):
  // params: name

  result = get_application(ctx, client, { name: params.name })
  if result is error: return result

  app = result.data
  return Success({
    name:            app.name,
    health_status:   app.health_status,    // Healthy | Progressing | Degraded |
                                           // Suspended | Missing | Unknown
    health_message:  app.health_message,
    sync_status:     app.sync_status,      // Synced | OutOfSync | Unknown
    operation_state: app.operation_state,  // nil if no operation running
    conditions:      app.conditions,       // degradation root causes if present
    revision:        app.revision,
  })
```

---

#### `get_resource_tree`
> Returns the full Kubernetes resource tree for an application — all managed and child resources with their individual health and sync status. Used in troubleshooting workflows to map exactly which resources are degraded and trace failure through parent-child relationships.

```
func get_resource_tree(ctx, client, params):
  // params: name

  resp = client.GET("/api/v1/applications/{params.name}/resource-tree")

  if resp.status == 404:
    return TerminalError("application_not_found")

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  nodes = resp.body.nodes ?? []

  return Success({
    nodes: nodes.map(n => {
      group:          n.group,
      version:        n.version,
      kind:           n.kind,
      namespace:      n.namespace,
      name:           n.name,
      uid:            n.uid,
      parent_refs:    n.parentRefs,
      health_status:  n.health?.status,
      health_message: n.health?.message,
      created_at:     n.createdAt,
    }),
    degraded_nodes: nodes
      .filter(n => n.health?.status == "Degraded")
      .map(n => format_node_ref(n)),
    missing_nodes: nodes
      .filter(n => n.health?.status == "Missing")
      .map(n => format_node_ref(n)),
    orphaned_nodes: resp.body.orphanedNodes?.map(n => format_node_ref(n)) ?? [],
  })
```

---

#### `get_managed_resources`
> Returns the list of Kubernetes resources currently managed by an application with their sync state. Used in drift-detection workflows to identify which specific resources are out of sync before initiating a targeted remediation sync.

```
func get_managed_resources(ctx, client, params):
  // params: name, group, kind, namespace, resource_name, version

  resp = client.GET("/api/v1/applications/{params.name}/managed-resources", query={
    group:        params.group,
    kind:         params.kind,
    namespace:    params.namespace,
    resourceName: params.resource_name,
    version:      params.version,
  })

  if resp.status == 404:
    return TerminalError("application_not_found")

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({
    resources: resp.body.items.map(r => {
      group:        r.group,
      version:      r.version,
      kind:         r.kind,
      namespace:    r.namespace,
      name:         r.name,
      status:       r.status,         // Synced | OutOfSync | Unknown
      health:       r.health,
      live_state:   JSON.parse(r.liveState ?? "{}"),
      target_state: JSON.parse(r.targetState ?? "{}"),
      requires_pruning: r.requiresPruning,
    }),
  })
```

---

#### `get_resource_events`
> Fetches Kubernetes events for a specific resource managed by an application. Used in troubleshooting workflows to surface recent Kubernetes-level events such as failed scheduling, image pull errors, OOM kills, and probe failures without requiring direct cluster access.

```
func get_resource_events(ctx, client, params):
  // params: name (app name), resource_namespace, resource_name,
  //         resource_uid, resource_kind

  resp = client.GET("/api/v1/applications/{params.name}/events", query={
    resourceNamespace: params.resource_namespace,
    resourceName:      params.resource_name,
    resourceUID:       params.resource_uid,
  })

  if resp.status == 404:
    return TerminalError("application_or_resource_not_found")

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  events = resp.body.items ?? []

  return Success({
    events: events.map(e => {
      type:            e.type,             // Normal | Warning
      reason:          e.reason,
      message:         e.message,
      count:           e.count,
      first_timestamp: e.firstTimestamp,
      last_timestamp:  e.lastTimestamp,
      involved_object: {
        kind:      e.involvedObject.kind,
        name:      e.involvedObject.name,
        namespace: e.involvedObject.namespace,
      },
      source:          e.source.component,
    }),
    warning_count: events.filter(e => e.type == "Warning").length,
  })
```

---

#### `get_application_events`
> Fetches ArgoCD-level events for an application — sync started, sync succeeded, sync failed, health degraded. Used in post-incident timeline reconstruction and audit workflows.

```
func get_application_events(ctx, client, params):
  // params: name

  resp = client.GET("/api/v1/applications/{params.name}/events")

  if resp.status == 404:
    return TerminalError("application_not_found")

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  events = resp.body.items ?? []

  return Success({
    events: events.map(e => {
      type:      e.type,
      reason:    e.reason,
      message:   e.message,
      count:     e.count,
      first_at:  e.firstTimestamp,
      last_at:   e.lastTimestamp,
    }),
  })
```

---

#### `watch_application`
> Opens a server-sent event stream for real-time application state changes. Used in synchronous deployment workflows where the module must observe live status transitions — Progressing → Healthy or Progressing → Degraded — without polling.

```
func watch_application(ctx, client, params):
  // params: name, timeout_seconds

  // Note: this operation establishes a streaming HTTP connection.
  // The module receives a channel and must consume events or cancel the
  // context to close the stream. The connector manages reconnection
  // transparently for transient disconnects.

  stream = client.GET_STREAM(
    "/api/v1/stream/applications",
    query={ name: params.name },
    timeout: params.timeout_seconds ?? 300,
  )

  if stream.error is not null:
    if is_transient(stream.error):
      return RetryableError(stream.error)
    return TerminalError(stream.error)

  return Success({
    event_channel: stream.channel,
    // Event shape per message:
    // {
    //   type:        "ADDED" | "MODIFIED" | "DELETED",
    //   application: { health_status, sync_status, operation_state, ... }
    // }
  })
```

---

### Troubleshooting Operations

---

#### `get_resource_diff`
> Returns the computed diff between the desired state from git and the live state in the cluster for a specific resource. Used as the primary diagnostic tool when a resource is out of sync — reveals exactly what has drifted and why ArgoCD considers it non-compliant.

```
func get_resource_diff(ctx, client, params):
  // params: name (app name), resource_name, resource_namespace,
  //         kind, group, version

  resources_result = get_managed_resources(ctx, client, {
    name:            params.name,
    kind:            params.kind,
    group:           params.group,
    version:         params.version,
    namespace:       params.resource_namespace,
    resource_name:   params.resource_name,
  })

  if resources_result is error: return resources_result

  resource = resources_result.data.resources[0]
  if resource is null:
    return TerminalError("resource_not_found_in_managed_resources")

  live   = resource.live_state
  target = resource.target_state

  diff = compute_diff(target, live)
  // compute_diff produces a structured field-level diff:
  // [{ path, desired, actual, change_type: added|removed|modified }]

  return Success({
    resource_ref: {
      kind:      params.kind,
      name:      params.resource_name,
      namespace: params.resource_namespace,
    },
    sync_status:    resource.status,
    diff:           diff,
    diff_summary: {
      added_fields:    diff.filter(d => d.change_type == "added").length,
      removed_fields:  diff.filter(d => d.change_type == "removed").length,
      modified_fields: diff.filter(d => d.change_type == "modified").length,
    },
    live_state:   live,
    target_state: target,
  })
```

---

#### `get_pod_logs`
> Streams or fetches pod logs for a resource managed by an application. Used in incident response workflows to retrieve application-level error output without requiring direct cluster access or kubectl credentials.

```
func get_pod_logs(ctx, client, params):
  // params: name (app name), pod_name, namespace, container,
  //         tail_lines, since_seconds, since_time (ISO8601),
  //         follow (bool), previous (bool — fetch logs from crashed container)

  resp = client.GET(
    "/api/v1/applications/{params.name}/pods/{params.pod_name}/logs",
    query={
      namespace:    params.namespace,
      container:    params.container,
      tailLines:    params.tail_lines ?? 100,
      sinceSeconds: params.since_seconds,
      sinceTime:    params.since_time,
      follow:       params.follow ?? false,
      previous:     params.previous ?? false,  // critical for crash-loop diagnosis
    }
  )

  if resp.status == 404:
    return TerminalError("pod_not_found")

  if resp.status == 400:
    return TerminalError({
      reason: "invalid_log_request",
      detail: resp.body.message,
      // common causes:
      //   - container name not found in pod spec
      //   - previous=true but no terminated container exists
      //   - pod in Pending state — no logs available yet
    })

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({
    pod_name:   params.pod_name,
    container:  params.container,
    namespace:  params.namespace,
    previous:   params.previous ?? false,
    lines:      resp.body.split("\n").filter(l => l != ""),
  })
```

---

#### `exec_pod`
> Executes a command inside a running container managed by an application and returns stdout, stderr, and exit code. Used in advanced troubleshooting workflows for interactive diagnosis — checking config mounts, testing connectivity, or inspecting process state. Must be explicitly permitted by ArgoCD RBAC and is audit-logged.

```
func exec_pod(ctx, client, params):
  // params: name (app name), pod_name, namespace, container,
  //         command[] (argv), timeout_seconds

  if params.command is null or params.command.length == 0:
    return TerminalError("command_is_required")

  resp = client.POST(
    "/api/v1/applications/{params.name}/pods/{params.pod_name}/exec",
    {
      namespace:  params.namespace,
      container:  params.container,
      command:    params.command,
    },
    timeout: params.timeout_seconds ?? 30,
  )

  if resp.status == 403:
    return TerminalError("exec_not_permitted — check argocd rbac policy")

  if resp.status == 404:
    return TerminalError("pod_or_container_not_found")

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({
    pod_name:   params.pod_name,
    container:  params.container,
    command:    params.command,
    stdout:     resp.body.stdout,
    stderr:     resp.body.stderr,
    exit_code:  resp.body.exitCode,
    succeeded:  resp.body.exitCode == 0,
  })
```

---

#### `get_sync_windows`
> Returns all active and upcoming sync windows that apply to an application. Used in troubleshooting workflows when a sync is not being triggered automatically — identifies whether a deny window is blocking automated sync and when the window will lift.

```
func get_sync_windows(ctx, client, params):
  // params: name (app name)

  resp = client.GET("/api/v1/applications/{params.name}/syncwindows")

  if resp.status == 404:
    return TerminalError("application_not_found")

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  windows = resp.body.activeWindows ?? []

  return Success({
    active_windows: windows.map(w => {
      kind:       w.kind,             // allow | deny
      schedule:   w.schedule,         // cron expression
      duration:   w.duration,
      namespaces: w.namespaces,
      clusters:   w.clusters,
      manual_sync: w.manualSync,      // if true, manual sync is still permitted
    }),
    sync_allowed: not windows.any(w => w.kind == "deny" and not w.manualSync),
    // if sync_allowed == false, automated sync is currently blocked
  })
```

---

#### `get_operation_state`
> Returns the detailed state of the most recent sync or rollback operation. Used in troubleshooting workflows to diagnose exactly which sync phase failed, which resource caused the failure, and the precise error message from the Kubernetes API server.

```
func get_operation_state(ctx, client, params):
  // params: name

  result = get_application(ctx, client, { name: params.name })
  if result is error: return result

  op = result.data.operation_state
  if op is null:
    return Success({ name: params.name, no_operation: true })

  return Success({
    name:           params.name,
    phase:          op.phase,             // Running | Failed | Error | Succeeded | Terminating
    message:        op.message,
    started_at:     op.startedAt,
    finished_at:    op.finishedAt,
    sync_result: {
      revision:   op.syncResult?.revision,
      source:     op.syncResult?.source,
      resources:  op.syncResult?.resources?.map(r => {
        group:      r.group,
        kind:       r.kind,
        namespace:  r.namespace,
        name:       r.name,
        status:     r.status,        // Synced | SyncFailed | Pruned | PruneSkipped | etc.
        message:    r.message,       // Kubernetes API error detail if failed
        hook_type:  r.hookType,
        hook_phase: r.hookPhase,
      }),
      failed_resources: op.syncResult?.resources
        ?.filter(r => r.status == "SyncFailed")
        ?.map(r => ({
          ref:     "{r.kind}/{r.namespace}/{r.name}",
          message: r.message,
        })),
    },
  })
```

---

#### `get_degraded_resources`
> Returns all resources in the application resource tree whose health status is Degraded. Used as a focused triage operation to surface the specific set of Kubernetes resources that need attention without processing the full tree.

```
func get_degraded_resources(ctx, client, params):
  // params: name

  tree_result = get_resource_tree(ctx, client, { name: params.name })
  if tree_result is error: return tree_result

  return Success({
    name:              params.name,
    degraded_resources: tree_result.data.degraded_nodes,
    count:             tree_result.data.degraded_nodes.length,
    healthy:           tree_result.data.degraded_nodes.length == 0,
  })
```

---

#### `get_out_of_sync_resources`
> Returns all resources whose sync status is OutOfSync. Used in drift-detection workflows to enumerate which Kubernetes objects have diverged from the declared git state and need to be reconciled.

```
func get_out_of_sync_resources(ctx, client, params):
  // params: name, group, kind, namespace

  resources_result = get_managed_resources(ctx, client, {
    name:      params.name,
    group:     params.group,
    kind:      params.kind,
    namespace: params.namespace,
  })

  if resources_result is error: return resources_result

  out_of_sync = resources_result.data.resources
    .filter(r => r.status == "OutOfSync")

  return Success({
    name:                params.name,
    out_of_sync_resources: out_of_sync.map(r => ({
      kind:      r.kind,
      name:      r.name,
      namespace: r.namespace,
      requires_pruning: r.requires_pruning,
    })),
    count:    out_of_sync.length,
    in_sync:  out_of_sync.length == 0,
  })
```

---

#### `get_orphaned_resources`
> Returns resources that exist in the cluster but are no longer defined in the git source — i.e., resources ArgoCD would prune on the next sync with prune enabled. Used in cleanup and compliance workflows to identify stale cluster objects before enabling automated pruning.

```
func get_orphaned_resources(ctx, client, params):
  // params: name

  tree_result = get_resource_tree(ctx, client, { name: params.name })
  if tree_result is error: return tree_result

  return Success({
    name:              params.name,
    orphaned_resources: tree_result.data.orphaned_nodes,
    count:             tree_result.data.orphaned_nodes.length,
    // Caller should review this list before enabling prune to avoid
    // accidental deletion of resources intentionally created out-of-band.
  })
```

---

#### `get_hook_status`
> Returns the status of all sync hooks (PreSync, Sync, PostSync, SyncFail) from the most recent operation. Used in troubleshooting workflows when a sync appears to have completed but the application is not reaching Healthy status — commonly caused by a failing PostSync hook.

```
func get_hook_status(ctx, client, params):
  // params: name, hook_type (PreSync|Sync|PostSync|SyncFail — optional filter)

  op_result = get_operation_state(ctx, client, { name: params.name })
  if op_result is error: return op_result
  if op_result.data.no_operation:
    return Success({ name: params.name, no_operation: true, hooks: [] })

  hooks = op_result.data.sync_result.resources
    ?.filter(r => r.hook_type is not null)
    ?.filter(r => params.hook_type is null or r.hook_type == params.hook_type)
    ?? []

  return Success({
    name:  params.name,
    hooks: hooks.map(h => ({
      kind:       h.kind,
      name:       h.name,
      namespace:  h.namespace,
      hook_type:  h.hook_type,       // PreSync | Sync | PostSync | SyncFail
      hook_phase: h.hook_phase,      // Running | Succeeded | Failed | Error | Terminating
      message:    h.message,
    })),
    failed_hooks: hooks.filter(h => h.hook_phase in ["Failed", "Error"]),
  })
```

---

#### `retry_operation`
> Retries the last failed sync operation. Used in automated remediation workflows when a sync has failed due to a transient error — cluster unavailability, webhook timeout, rate limit — and the fix is to simply re-attempt without modifying the spec.

```
func retry_operation(ctx, client, params):
  // params: name

  // Guard: only retry if the current operation phase is Failed or Error.
  op_result = get_operation_state(ctx, client, { name: params.name })
  if op_result is error: return op_result

  phase = op_result.data.phase
  if phase not in ["Failed", "Error"]:
    return TerminalError({
      reason: "cannot_retry",
      current_phase: phase,
      message: "retry is only valid for Failed or Error operations",
    })

  resp = client.POST("/api/v1/applications/{params.name}/retry")

  if resp.status == 409:
    return RetryableError("operation_already_in_progress")

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({ name: params.name, retried: true })
```

---

### AppProject Operations

---

#### `get_project`
> Fetches an ArgoCD AppProject definition. Used to inspect source repo allow-lists, destination cluster restrictions, and RBAC rules before provisioning a new application into a project.

```
func get_project(ctx, client, params):
  // params: name

  resp = client.GET("/api/v1/projects/{params.name}")

  if resp.status == 404:
    return TerminalError("project_not_found")

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  p = resp.body.project
  return Success({
    name:               p.metadata.name,
    source_repos:       p.spec.sourceRepos,
    destinations:       p.spec.destinations,
    cluster_resources:  p.spec.clusterResourceWhitelist,
    namespace_resources: p.spec.namespaceResourceWhitelist,
    roles:              p.spec.roles,
    sync_windows:       p.spec.syncWindows,
    orphaned_resources: p.spec.orphanedResources,
  })
```

---

#### `list_projects`
> Lists all ArgoCD projects. Used in compliance and onboarding workflows to enumerate available projects before assigning a new application.

```
func list_projects(ctx, client, params):
  // params: (none required)

  resp = client.GET("/api/v1/projects")

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({
    projects: resp.body.items.map(p => ({
      name:         p.metadata.name,
      source_repos: p.spec.sourceRepos,
      destinations: p.spec.destinations,
    })),
  })
```

---

#### `create_project`
> Creates an ArgoCD AppProject with defined source repo allow-lists, destination constraints, and RBAC roles. Used in tenant onboarding workflows to establish an isolation boundary before provisioning applications for a team.

```
func create_project(ctx, client, params):
  // params: name, description, source_repos[], destinations[]{server,namespace},
  //         cluster_resource_whitelist[]{group, kind},
  //         namespace_resource_blacklist[]{group, kind},
  //         roles[]{name, description, policies[], groups[]},
  //         sync_windows[], orphaned_resources{warn}

  existing = get_project(ctx, client, { name: params.name })
  if existing is Success:
    return Success({ idempotent: true, name: params.name })

  resp = client.POST("/api/v1/projects", {
    project: {
      metadata: { name: params.name },
      spec: {
        description:               params.description,
        sourceRepos:               params.source_repos ?? ["*"],
        destinations:              params.destinations ?? [],
        clusterResourceWhitelist:  params.cluster_resource_whitelist ?? [],
        namespaceResourceBlacklist: params.namespace_resource_blacklist ?? [],
        roles:                     params.roles ?? [],
        syncWindows:               params.sync_windows ?? [],
        orphanedResources:         params.orphaned_resources,
      },
    },
  })

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({ name: params.name })
```

---

#### `delete_project`
> Deletes an ArgoCD AppProject. Will fail if any applications are still assigned to the project. Used in tenant offboarding workflows after all applications have been removed.

```
func delete_project(ctx, client, params):
  // params: name

  resp = client.DELETE("/api/v1/projects/{params.name}")

  if resp.status == 404:
    return Success({ idempotent: true })

  if resp.status == 400:
    return TerminalError({
      reason: "project_not_empty",
      detail: resp.body.message,
      // Applications must be deleted before the project can be removed.
    })

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({ name: params.name })
```

---

#### `get_project_events`
> Returns recent ArgoCD events scoped to a project. Used in audit and incident reconstruction workflows to surface all sync, degradation, and access events across all applications within a project boundary.

```
func get_project_events(ctx, client, params):
  // params: name

  resp = client.GET("/api/v1/projects/{params.name}/events")

  if resp.status == 404:
    return TerminalError("project_not_found")

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({
    events: resp.body.items.map(e => ({
      type:     e.type,
      reason:   e.reason,
      message:  e.message,
      last_at:  e.lastTimestamp,
    })),
  })
```

---

### Repository Operations

---

#### `validate_repository_access`
> Tests whether ArgoCD can connect to and read a repository using the stored credentials. Used as a pre-flight check in provisioning workflows before creating an application, and as a diagnostic in troubleshooting workflows when sync errors suggest a credential or network issue.

```
func validate_repository_access(ctx, client, params):
  // params: repo_url, username (optional), password (optional),
  //         ssh_private_key (optional), tls_client_cert (optional),
  //         insecure (bool), enable_lfs (bool)

  resp = client.POST("/api/v1/repositories", {
    repo:          params.repo_url,
    username:      params.username,
    password:      params.password,
    sshPrivateKey: params.ssh_private_key,
    tlsClientCertData: params.tls_client_cert,
    insecure:      params.insecure ?? false,
    enableLfs:     params.enable_lfs ?? false,
    connectionState: true,     // dry-run validation only
  })

  if resp.status == 200:
    return Success({
      repo_url:      params.repo_url,
      reachable:     resp.body.connectionState?.status == "Successful",
      message:       resp.body.connectionState?.message,
    })

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError({
      reason:  "repository_access_failed",
      detail:  resp.body.message,
      // common causes:
      //   - SSH host key not trusted — add certificate first
      //   - token expired or has insufficient scopes
      //   - repository URL typo
      //   - network policy blocking ArgoCD repo-server egress
    })

  return Success({ repo_url: params.repo_url, reachable: true })
```

---

### Cluster Operations

---

#### `invalidate_cluster_cache`
> Forces ArgoCD to discard its cached view of a cluster and re-list all resources from the Kubernetes API. Used in troubleshooting workflows when the resource tree appears stale — a resource shows as Missing but exists in the cluster, or a deleted resource still appears in the tree.

```
func invalidate_cluster_cache(ctx, client, params):
  // params: cluster_server (URL of the cluster API server)

  resp = client.POST(
    "/api/v1/clusters/{url_encode(params.cluster_server)}/invalidate-cache"
  )

  if resp.status == 404:
    return TerminalError("cluster_not_found")

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({
    cluster_server:   params.cluster_server,
    cache_invalidated: true,
    // Cache rebuild is asynchronous. Allow 30-60 seconds before
    // re-querying the resource tree.
  })
```

---

### RBAC Operations

---

#### `can_i`
> Checks whether the current ArgoCD token has permission to perform a specific action on a resource. Used in pre-flight checks within provisioning and remediation workflows to fail fast with a descriptive error rather than discovering a permissions issue mid-execution.

```
func can_i(ctx, client, params):
  // params: resource (applications|projects|clusters|repositories|...),
  //         action (get|create|update|delete|sync|override|action),
  //         subresource (optional: logs|exec),
  //         object (resource name or *)

  resp = client.GET("/api/v1/account/can-i/{params.action}/{params.resource}/{params.object}",
    query={ subresource: params.subresource }
  )

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  allowed = resp.body == "yes"

  return Success({
    resource:    params.resource,
    action:      params.action,
    object:      params.object,
    allowed:     allowed,
  })
```

---

## Error Classification Reference

| HTTP Status | Classification | Engine Behaviour |
|---|---|---|
| `2xx` | Success | Advance to next step |
| `404` on delete/terminate | Success (idempotent) | Already absent — treat as done |
| `404` on get/sync/rollback | TerminalError | Resource does not exist — fail the step |
| `400` invalid request | TerminalError | Malformed params — fail with error detail |
| `400` project not empty | TerminalError | Applications must be removed first |
| `401` | TerminalError | Token invalid or expired |
| `403` | TerminalError | Insufficient RBAC permissions — check ArgoCD policy |
| `409` operation in progress | RetryableError | Wait and re-attempt or terminate first |
| `429` rate limited | RetryableError | Re-enqueue with backoff |
| `5xx` | RetryableError | Re-enqueue with exponential backoff |
| Network timeout | RetryableError | Re-enqueue with backoff |

---

## Notes

**Authentication.** ArgoCD API tokens are Bearer tokens issued by ArgoCD itself or passed through OIDC. The connector resolves the token from the credential vault at call time and scrubs it from guarded memory after the HTTP response is received. For OIDC-backed tokens, expiry is checked before each call and a refresh is attempted if the token is within 60 seconds of expiry.

**Stale Cache Diagnosis.** When a troubleshooting workflow observes a mismatch between `get_resource_tree` output and known cluster state, the canonical remediation sequence is: `invalidate_cluster_cache` → wait 30-60 seconds → `get_application` with `refresh=true` → `get_resource_tree`. This forces a full reconciliation cycle.

**Rollback and Auto-Sync.** The `rollback_application` operation implicitly disables automated sync on the application. If the calling module intends to re-enable auto-sync after confirming the rolled-back state is stable, it must explicitly call `patch_application` to restore the `syncPolicy.automated` field. Failing to do so leaves the application in a manually managed state indefinitely.

**Hook Failures.** The most common cause of an application that is Synced but not Healthy is a failing PostSync hook. The canonical troubleshooting sequence is: `get_hook_status` → identify failing hook → `get_pod_logs` with `previous=true` on the hook pod → inspect stderr → `retry_operation` or fix the source and re-sync.

**Exec Safety.** The `exec_pod` operation requires explicit RBAC permission in the ArgoCD policy (`p, role:operator, exec, create, */*, allow`). It must not be enabled by default. Modules that use it must document the justification and the operation is always audit-logged with the full command vector.

**Sync Window Blocking.** When automated sync is not triggering, the canonical diagnostic sequence is: `get_sync_windows` → check `sync_allowed` field → if `false`, inspect the active deny window schedule and duration. Manual sync (`sync_application`) is still available if the window's `manualSync` flag is true.