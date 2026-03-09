# Backstage Connector — Pseudo Code

The connector encapsulates all interactions with the Backstage API surface. It is used by modules that need to register software catalog entities, manage templates, trigger scaffolder tasks, and read or write TechDocs, annotations, and ownership data. All operations are idempotent by contract and return one of `Success`, `RetryableError`, or `TerminalError`.

---

## Interface

```
CONNECTOR: BackstageConnector
implements Connector interface

registered name: "backstage"

func Call(ctx, op, params, credential) -> ConnectorResult:
  client = resolve_client(credential)
  // credential: { base_url, token, token_type (static|oidc) }

  switch op:
    case "register_entity":               return register_entity(ctx, client, params)
    case "unregister_entity":             return unregister_entity(ctx, client, params)
    case "get_entity":                    return get_entity(ctx, client, params)
    case "list_entities":                 return list_entities(ctx, client, params)
    case "refresh_entity":                return refresh_entity(ctx, client, params)
    case "validate_entity":               return validate_entity(ctx, client, params)
    case "apply_entity_annotation":       return apply_entity_annotation(ctx, client, params)
    case "get_catalog_location":          return get_catalog_location(ctx, client, params)
    case "list_catalog_locations":        return list_catalog_locations(ctx, client, params)
    case "delete_catalog_location":       return delete_catalog_location(ctx, client, params)
    case "trigger_scaffolder_task":       return trigger_scaffolder_task(ctx, client, params)
    case "get_scaffolder_task":           return get_scaffolder_task(ctx, client, params)
    case "list_scaffolder_tasks":         return list_scaffolder_tasks(ctx, client, params)
    case "cancel_scaffolder_task":        return cancel_scaffolder_task(ctx, client, params)
    case "list_scaffolder_templates":     return list_scaffolder_templates(ctx, client, params)
    case "get_scaffolder_template":       return get_scaffolder_template(ctx, client, params)
    case "get_techdocs_site":             return get_techdocs_site(ctx, client, params)
    case "trigger_techdocs_build":        return trigger_techdocs_build(ctx, client, params)
    case "get_user":                      return get_user(ctx, client, params)
    case "get_group":                     return get_group(ctx, client, params)
    case "list_group_members":            return list_group_members(ctx, client, params)
    case "search_entities":               return search_entities(ctx, client, params)
    case "get_entity_ancestry":           return get_entity_ancestry(ctx, client, params)
    case "get_entity_facets":             return get_entity_facets(ctx, client, params)
    case "create_or_update_proxy_config": return create_or_update_proxy_config(ctx, client, params)

    default:
      return TerminalError("unknown operation: " + op)
```

---

## Supported Operations

### Catalog Entity Operations

---

#### `register_entity`
> Registers a catalog entity by submitting a location URL pointing to a `catalog-info.yaml` file. Used in service scaffolding workflows immediately after a repository is created and the manifest has been pushed.

```
func register_entity(ctx, client, params):
  // params: location_url, presence (optional: required|optional)

  existing = find_location_by_url(client, params.location_url)

  if existing is not null:
    return Success({
      location_id: existing.id,
      location_url: params.location_url,
      idempotent: true,
    })

  resp = client.POST("/api/catalog/locations", {
    type:   "url",
    target: params.location_url,
  })

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({
    location_id:  resp.body.location.id,
    location_url: params.location_url,
  })


func find_location_by_url(client, url) -> Location | null:
  resp = client.GET("/api/catalog/locations/by-url",
                    query={ url: url })
  if resp.status == 200: return resp.body
  return null
```

---

#### `unregister_entity`
> Removes a catalog entity and its associated location registration. Used in service decommissioning workflows to remove stale entries from the software catalog.

```
func unregister_entity(ctx, client, params):
  // params: entity_ref (kind:namespace/name)  OR  location_id

  if params.location_id is not null:
    location_id = params.location_id
  else:
    location = find_location_by_entity_ref(client, params.entity_ref)
    if location is null:
      return Success({ idempotent: true })    // entity does not exist
    location_id = location.id

  resp = client.DELETE("/api/catalog/locations/{location_id}")

  if resp.status == 404:
    return Success({ idempotent: true })

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({ location_id: location_id })
```

---

#### `get_entity`
> Fetches the full entity record for a single catalog entry by its entity reference. Used in validation, orchestration gating, and read-only inspection steps.

```
func get_entity(ctx, client, params):
  // params: kind, namespace, name

  namespace = params.namespace ?? "default"

  resp = client.GET(
    "/api/catalog/entities/by-name/{params.kind}/{namespace}/{params.name}"
  )

  if resp.status == 404:
    return TerminalError("entity_not_found")

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({
    entity_ref:   to_entity_ref(resp.body),
    kind:         resp.body.kind,
    name:         resp.body.metadata.name,
    namespace:    resp.body.metadata.namespace,
    annotations:  resp.body.metadata.annotations,
    labels:       resp.body.metadata.labels,
    owner:        resp.body.spec.owner,
    lifecycle:    resp.body.spec.lifecycle,
    raw:          resp.body,
  })
```

---

#### `list_entities`
> Lists catalog entities with optional filter, field selection, and pagination. Used in audit, bulk-operation, and reporting workflows.

```
func list_entities(ctx, client, params):
  // params: filter[](field=value expressions), fields[], order_by,
  //         after (cursor), limit

  // filter examples:
  //   "kind=Component"
  //   "spec.type=service"
  //   "metadata.annotations.backstage.io/owner=team-a"

  resp = client.GET("/api/catalog/entities", query={
    filter:   params.filter,           // repeated query param
    fields:   params.fields,           // repeated query param; sparse fieldset
    orderField: params.order_by,
    after:    params.after,
    limit:    params.limit ?? 100,
  })

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({
    entities:    resp.body,
    next_cursor: parse_next_cursor(resp.headers["link"]),
  })
```

---

#### `refresh_entity`
> Schedules an immediate refresh of a catalog entity from its source location. Used after automated changes to a repository to ensure the catalog reflects the latest state without waiting for the scheduled polling interval.

```
func refresh_entity(ctx, client, params):
  // params: entity_ref (kind:namespace/name)

  resp = client.POST("/api/catalog/refresh", {
    entityRef: params.entity_ref,
  })

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({ entity_ref: params.entity_ref, scheduled: true })
```

---

#### `validate_entity`
> Validates a catalog entity definition without persisting it. Used as a pre-flight check in scaffolding workflows to assert that a generated `catalog-info.yaml` is well-formed before committing it to source control.

```
func validate_entity(ctx, client, params):
  // params: entity (raw entity object matching Backstage schema)

  resp = client.POST("/api/catalog/validate-entity", {
    entity: params.entity,
  })

  if resp.status == 200:
    return Success({ valid: true })

  if resp.status == 400:
    return TerminalError({
      reason:  "entity_validation_failed",
      errors:  resp.body.errors,
    })

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({ valid: true })
```

---

#### `apply_entity_annotation`
> Adds or updates one or more annotations on an existing catalog entity. Used in post-provisioning workflows to attach runtime metadata such as cloud resource ARNs, deployment targets, or pipeline URLs to a service record.

```
func apply_entity_annotation(ctx, client, params):
  // params: kind, namespace, name, annotations{}
  // Note: Backstage catalog API does not expose a PATCH endpoint for
  // metadata mutations on server-managed entities. The standard approach
  // is to write the annotation into the source catalog-info.yaml and
  // trigger a refresh. This operation encapsulates that pattern.

  namespace = params.namespace ?? "default"

  // Step 1: fetch current entity to resolve source location
  entity_result = get_entity(ctx, client, {
    kind:      params.kind,
    namespace: namespace,
    name:      params.name,
  })
  if entity_result is error: return entity_result

  source_location = entity_result.data.annotations["backstage.io/managed-by-location"]
  if source_location is null:
    return TerminalError("entity_has_no_managed_source_location")

  // Step 2: caller is responsible for patching the yaml at source_location
  // and calling refresh_entity. This operation returns the source location
  // so the module can delegate to the git connector for the file update.

  return Success({
    entity_ref:      to_entity_ref(entity_result.data),
    source_location: source_location,
    current_annotations: entity_result.data.annotations,
  })
```

---

#### `get_entity_ancestry`
> Returns the full ancestry chain for an entity — all parent entities through the ownership and part-of graph. Used in impact analysis and dependency mapping workflows.

```
func get_entity_ancestry(ctx, client, params):
  // params: entity_ref (kind:namespace/name)

  resp = client.GET("/api/catalog/entities/by-ref/{params.entity_ref}/ancestry")

  if resp.status == 404:
    return TerminalError("entity_not_found")

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({
    root_entity_ref: params.entity_ref,
    items:           resp.body.items.map(i => {
      entity_ref: to_entity_ref(i.entity),
      parents:    i.parentEntityRefs,
    }),
  })
```

---

#### `get_entity_facets`
> Returns aggregated facet counts for catalog fields — useful for dashboards and filtering UIs. Used in reporting and compliance summary workflows.

```
func get_entity_facets(ctx, client, params):
  // params: facets[] (field paths), filter[] (field=value expressions)

  resp = client.GET("/api/catalog/entity-facets", query={
    facet:  params.facets,     // repeated: e.g. ["kind", "spec.type", "spec.lifecycle"]
    filter: params.filter,
  })

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({
    facets: resp.body.facets,
    // shape: { "kind": [{ value: "Component", count: 42 }, ...], ... }
  })
```

---

### Catalog Location Operations

---

#### `get_catalog_location`
> Fetches metadata for a registered catalog location by its ID. Used to inspect the status of a location and verify it was correctly ingested after registration.

```
func get_catalog_location(ctx, client, params):
  // params: location_id

  resp = client.GET("/api/catalog/locations/{params.location_id}")

  if resp.status == 404:
    return TerminalError("location_not_found")

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({
    location_id:  resp.body.data.id,
    type:         resp.body.data.type,
    target:       resp.body.data.target,
  })
```

---

#### `list_catalog_locations`
> Lists all registered catalog locations. Used in audit and deduplication workflows to identify stale or duplicate location registrations before performing bulk cleanup.

```
func list_catalog_locations(ctx, client, params):
  // params: (none required)

  resp = client.GET("/api/catalog/locations")

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({
    locations: resp.body.map(l => {
      location_id: l.data.id,
      type:        l.data.type,
      target:      l.data.target,
    }),
  })
```

---

#### `delete_catalog_location`
> Deletes a catalog location registration by ID, which causes all entities ingested exclusively from that location to be removed from the catalog. Used in bulk decommissioning and cleanup workflows.

```
func delete_catalog_location(ctx, client, params):
  // params: location_id

  resp = client.DELETE("/api/catalog/locations/{params.location_id}")

  if resp.status == 404:
    return Success({ idempotent: true })

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({ location_id: params.location_id })
```

---

### Search Operations

---

#### `search_entities`
> Executes a full-text search across all indexed catalog entities and TechDocs content. Used in discovery workflows and pre-provisioning checks to detect existing services before creating duplicates.

```
func search_entities(ctx, client, params):
  // params: query, types[] (software-catalog|techdocs),
  //         filter{}, page_cursor, page_limit

  resp = client.GET("/api/search/query", query={
    term:       params.query,
    types:      params.types ?? ["software-catalog"],
    filters:    params.filter,
    pageCursor: params.page_cursor,
    pageLimit:  params.page_limit ?? 25,
  })

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({
    results:     resp.body.results.map(r => {
      type:      r.type,
      document:  r.document,
      highlight: r.highlight,
    }),
    next_cursor: resp.body.nextPageCursor,
  })
```

---

### Scaffolder Operations

---

#### `trigger_scaffolder_task`
> Submits a scaffolder task for execution using a named template and parameter set. Used in meta-orchestration workflows where the Release Engine drives Backstage to provision a resource through a Backstage template.

```
func trigger_scaffolder_task(ctx, client, params):
  // params: template_ref (template:default/name), values{},
  //         secrets{} (sensitive inputs — not logged)

  resp = client.POST("/api/scaffolder/v2/tasks", {
    templateRef: params.template_ref,
    values:      params.values,
    secrets:     params.secrets ?? {},
  })

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({
    task_id:    resp.body.id,
    task_url:   "{client.base_url}/api/scaffolder/v2/tasks/{resp.body.id}",
  })
```

---

#### `get_scaffolder_task`
> Fetches the current status and output of a scaffolder task by task ID. Used in polling loops within orchestration workflows that must gate on scaffolder completion before proceeding.

```
func get_scaffolder_task(ctx, client, params):
  // params: task_id

  resp = client.GET("/api/scaffolder/v2/tasks/{params.task_id}")

  if resp.status == 404:
    return TerminalError("scaffolder_task_not_found")

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({
    task_id:    resp.body.id,
    status:     resp.body.status,        // open | processing | failed | cancelled | completed
    created_by: resp.body.createdBy,
    output:     resp.body.spec.output,
    created_at: resp.body.createdAt,
    last_heartbeat_at: resp.body.lastHeartbeatAt,
  })
```

---

#### `list_scaffolder_tasks`
> Lists scaffolder tasks with optional status and creator filters. Used in audit, monitoring, and stuck-task detection workflows.

```
func list_scaffolder_tasks(ctx, client, params):
  // params: status (open|processing|failed|cancelled|completed),
  //         created_by, order_by, page_size, page_token

  resp = client.GET("/api/scaffolder/v2/tasks", query={
    status:     params.status,
    createdBy:  params.created_by,
    orderBy:    params.order_by,
    pageSize:   params.page_size ?? 50,
    pageToken:  params.page_token,
  })

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({
    tasks: resp.body.tasks.map(t => {
      task_id:   t.id,
      status:    t.status,
      created_by: t.createdBy,
    }),
    next_page_token: resp.body.nextPageToken,
  })
```

---

#### `cancel_scaffolder_task`
> Cancels an in-progress scaffolder task. Used in abort and rollback workflows when a parent orchestration step has failed and dependent scaffolder activity must be halted.

```
func cancel_scaffolder_task(ctx, client, params):
  // params: task_id

  resp = client.POST("/api/scaffolder/v2/tasks/{params.task_id}/cancel")

  if resp.status == 404:
    return TerminalError("scaffolder_task_not_found")

  if resp.status == 409:
    return Success({ idempotent: true })   // already completed or cancelled

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({ task_id: params.task_id, cancelled: true })
```

---

#### `list_scaffolder_templates`
> Lists all available scaffolder templates registered in the catalog. Used in discovery and validation workflows to assert that a required template exists before submitting a task.

```
func list_scaffolder_templates(ctx, client, params):
  // params: filter[] (metadata.name=..., metadata.tags has value, etc.)

  // Templates are standard catalog entities of kind=Template.
  // Delegate to list_entities with a fixed kind filter.

  return list_entities(ctx, client, {
    filter: ["kind=Template"] + (params.filter ?? []),
    fields: [
      "metadata.name",
      "metadata.namespace",
      "metadata.title",
      "metadata.description",
      "metadata.tags",
      "spec.type",
      "spec.owner",
    ],
    limit: params.limit ?? 100,
  })
```

---

#### `get_scaffolder_template`
> Fetches the full schema and metadata for a single scaffolder template. Used to validate parameter sets before submitting a task, avoiding a round-trip failure from a malformed submission.

```
func get_scaffolder_template(ctx, client, params):
  // params: name, namespace

  return get_entity(ctx, client, {
    kind:      "Template",
    namespace: params.namespace ?? "default",
    name:      params.name,
  })
```

---

### TechDocs Operations

---

#### `get_techdocs_site`
> Fetches TechDocs site metadata for an entity. Used to verify that documentation has been published for a service, or to retrieve the docs URL for inclusion in a portal notification.

```
func get_techdocs_site(ctx, client, params):
  // params: kind, namespace, name

  namespace = params.namespace ?? "default"

  resp = client.GET(
    "/api/techdocs/metadata/techdocs/{params.kind}/{namespace}/{params.name}"
  )

  if resp.status == 404:
    return TerminalError("techdocs_site_not_found")

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({
    entity_ref:  "{params.kind}:{namespace}/{params.name}",
    site_name:   resp.body.site_name,
    site_description: resp.body.site_description,
    build_timestamp: resp.body.build_timestamp,
  })
```

---

#### `trigger_techdocs_build`
> Initiates an on-demand TechDocs build for an entity. Used after documentation source files have been updated in a repository to force immediate publication without waiting for the next scheduled sync.

```
func trigger_techdocs_build(ctx, client, params):
  // params: kind, namespace, name

  namespace = params.namespace ?? "default"

  resp = client.POST(
    "/api/techdocs/sync/{params.kind}/{namespace}/{params.name}"
  )

  if resp.status == 404:
    return TerminalError("entity_not_found_for_techdocs_build")

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({
    entity_ref: "{params.kind}:{namespace}/{params.name}",
    triggered:  true,
  })
```

---

### User and Group Operations

---

#### `get_user`
> Fetches a User entity from the catalog by name. Used in access provisioning and ownership-validation workflows to resolve a human identity to a catalog entity before assigning ownership.

```
func get_user(ctx, client, params):
  // params: name, namespace

  return get_entity(ctx, client, {
    kind:      "User",
    namespace: params.namespace ?? "default",
    name:      params.name,
  })
```

---

#### `get_group`
> Fetches a Group entity from the catalog by name. Used in provisioning workflows to validate team ownership references before assigning a group as the owner of a new service.

```
func get_group(ctx, client, params):
  // params: name, namespace

  return get_entity(ctx, client, {
    kind:      "Group",
    namespace: params.namespace ?? "default",
    name:      params.name,
  })
```

---

#### `list_group_members`
> Lists all User entities that are direct members of a Group. Used in access provisioning, notification, and on-call routing workflows.

```
func list_group_members(ctx, client, params):
  // params: name, namespace

  namespace = params.namespace ?? "default"

  // Group membership is modelled as catalog relations. Fetch member entities
  // via the entity list endpoint filtered by the memberOf relation.

  resp = list_entities(ctx, client, {
    filter: [
      "kind=User",
      "relations.memberof=group:{namespace}/{params.name}",
    ],
    fields: [
      "metadata.name",
      "metadata.namespace",
      "spec.profile",
    ],
  })

  if resp is error: return resp

  return Success({
    group:   "{namespace}/{params.name}",
    members: resp.data.entities.map(u => {
      name:      u.metadata.name,
      namespace: u.metadata.namespace,
      email:     u.spec.profile.email,
      display_name: u.spec.profile.displayName,
    }),
  })
```

---

### Proxy Configuration Operations

---

#### `create_or_update_proxy_config`
> Creates or updates a Backstage proxy route configuration entry. Used in provisioning workflows that expose a new backend service through the Backstage proxy endpoint, enabling frontend plugins to reach it without direct cross-origin calls.

```
func create_or_update_proxy_config(ctx, client, params):
  // params: route, target, allowed_methods[], headers{},
  //         change_origin, secure, credentials
  // Note: Backstage proxy config is managed via the app-config rather
  // than a live API. This operation writes to the configuration store
  // used by the deployment pipeline and triggers a config reload.
  // The exact mechanism is deployment-topology-dependent.

  config_entry = {
    target:        params.target,
    changeOrigin:  params.change_origin ?? true,
    secure:        params.secure ?? true,
    allowedMethods: params.allowed_methods ?? ["GET"],
    headers:       params.headers ?? {},
    credentials:   params.credentials ?? "require",
  }

  resp = client.PUT("/api/proxy-config/routes/{url_encode(params.route)}", {
    route:  params.route,
    config: config_entry,
  })

  if resp.status in [200, 201]:
    return Success({
      route:      params.route,
      target:     params.target,
      idempotent: resp.status == 200,
    })

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({ route: params.route, target: params.target })
```

---

## Error Classification Reference

| HTTP Status | Classification | Engine Behaviour |
|---|---|---|
| `2xx` | Success | Advance to next step |
| `404` on delete/unregister | Success (idempotent) | Already absent — treat as done |
| `404` on get/fetch | TerminalError | Resource does not exist — fail the step |
| `409` on cancel | Success (idempotent) | Already completed or cancelled |
| `400` validation failure | TerminalError | Malformed request — fail the step with error detail |
| `401` | TerminalError | Credential invalid or expired — fail the step |
| `403` | TerminalError | Insufficient permissions — fail the step |
| `429` rate limited | RetryableError | Re-enqueue with backoff |
| `5xx` | RetryableError | Re-enqueue with exponential backoff |
| Network timeout | RetryableError | Re-enqueue with backoff |

---

## Notes

**Authentication.** The credential's `token_type` field controls how the `Authorization` header is constructed. A `static` token is passed directly as a `Bearer` token. An `oidc` token is fetched from the configured OIDC provider at call time and is scoped to the duration of the connector invocation. The plaintext token is scrubbed from guarded memory immediately after the HTTP response is received.

**Entity References.** All entity refs follow the Backstage canonical form `kind:namespace/name`. Helper `to_entity_ref(entity)` constructs this from a raw entity response object. Helper `url_encode(ref)` percent-encodes the ref for use in path segments.

**Annotation Mutations.** The Backstage catalog API does not expose a direct PATCH endpoint for mutating metadata on server-managed entities. The `apply_entity_annotation` operation resolves the source location and returns it to the calling module, which is then responsible for delegating a file update to the Git connector followed by a `refresh_entity` call. This two-step pattern is the canonical approach.

**Scaffolder Task Polling.** The `get_scaffolder_task` operation is designed to be called repeatedly inside a module polling loop. The module is responsible for implementing the polling interval and timeout; the connector makes no assumptions about call frequency.