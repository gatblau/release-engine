# Scaffold Service Module — Design Document

## Module Identity

| Field | Value |
|-------|-------|
| **Module Key** | `scaffolding/create-service` |
| **Version** | `1.0.0` |
| **Interface** | `registry.Module` |
| **Package** | `internal/module/scaffoldservice` |

## Architecture

The module implements the `registry.Module` interface as defined in `internal/registry/module_registry.go`:

```go
type Module interface {
    Key() string
    Version() string
    Execute(ctx context.Context, api any, params map[string]any) error
}
```

The runner calls `module.Execute(ctx, stepAPI, params)`. The module uses `StepAPI` to manage step lifecycle and call connectors. This follows the procedural module pattern established in `d02.md`.

## Connectors

| Connector | Package | Operations Used |
|-----------|---------|-----------------|
| **GitHub** | `internal/connector/github/` | `create-repository`, `delete-repository` |
| **Service Catalog** | `internal/connector/catalog/` | `register-component` |

## Workflow Steps

### Step 1: Render Starter Code and CI Configuration

Generates repository contents from the selected template — source scaffolding, CI pipeline definitions, and catalog-info descriptor.

- **Inputs**: `template`, `service_name`, `owner`, `parameters`
- **Outputs**: `repo_files` (rendered file tree stored in job context)
- **Failure mode**: Terminal. Invalid template or parameters are not retryable.

### Step 2: Create GitHub Repository

Creates a new repository with the rendered starter code and CI configuration via the GitHub connector.

- **Connector**: `github`
- **Operation**: `create-repository`
- **Inputs**: `org`, `service_name`, `repo_files` (from Step 1), `visibility`
- **Outputs**: `repo_url`
- **Failure mode**: Retryable on transient GitHub API errors. Terminal on auth, naming conflicts, or validation failures. On terminal failure, no cleanup needed — nothing was created.

### Step 3: Register Component in Service Catalog

Registers the new service as a component in the Service Catalog using the catalog-info descriptor.

- **Connector**: `catalog`
- **Operation**: `register-component`
- **Inputs**: `repo_url` (from Step 2), `service_name`, `owner`, `catalog_descriptor` (from Step 1)
- **Outputs**: `catalog_entity_ref`
- **Failure mode**: On failure, triggers compensating cleanup (Step 3a) before failing the job. This prevents orphaned repositories.

### Step 3a: Compensating Cleanup — Delete Repository

Executes only when Step 3 fails. Deletes the repository created in Step 2 to prevent dangling resources.

- **Connector**: `github`
- **Operation**: `delete-repository`
- **Inputs**: `repo_url` (from Step 2)
- **Outputs**: None
- **Failure mode**: Best-effort. If deletion fails, the job fails with error code `CLEANUP_FAILED` and the orphaned repository is logged for manual intervention.

### Step 4: Completion Notification

Emits a completion event through the outbox system back to Backstage.

- **Inputs**: `repo_url` (from Step 2), `catalog_entity_ref` (from Step 3)
- **Outputs**: Webhook delivery via outbox
- **Failure mode**: Outbox handles delivery guarantees. Step itself is terminal on context write failure.

## Execute Implementation

```go
func (m *ScaffoldServiceModule) Execute(ctx context.Context, api any, params map[string]any) error {
    stepAPI := api.(runner.StepAPI)

    // Step 1: Render starter code and CI configuration
    stepAPI.BeginStep("render-template")
    repoFiles, catalogDescriptor, err := renderTemplate(params)
    if err != nil {
        return stepAPI.FailStep(err)
    }
    stepAPI.CompleteStep(map[string]any{
        "repo_files":         repoFiles,
        "catalog_descriptor": catalogDescriptor,
    })

    // Step 2: Create GitHub repository
    stepAPI.BeginStep("create-repository")
    repoResult, err := stepAPI.CallConnector("github", "create-repository", map[string]any{
        "org":        params["org"],
        "name":       params["service_name"],
        "files":      repoFiles,
        "visibility": stringOrDefault(params["visibility"], "internal"),
    })
    if err != nil {
        return stepAPI.FailStep(err)
    }
    stepAPI.CompleteStep(repoResult)

    // Step 3: Register component in Service Catalog
    stepAPI.BeginStep("register-component")
    catalogResult, err := stepAPI.CallConnector("catalog", "register-component", map[string]any{
        "repo_url":    repoResult["repo_url"],
        "name":        params["service_name"],
        "owner":       params["owner"],
        "descriptor":  catalogDescriptor,
    })
    if err != nil {
        // Step 3a: Compensating cleanup — delete orphaned repository
        stepAPI.FailStep(err)

        stepAPI.BeginStep("cleanup-repository")
        _, cleanupErr := stepAPI.CallConnector("github", "delete-repository", map[string]any{
            "repo_url": repoResult["repo_url"],
        })
        if cleanupErr != nil {
            return stepAPI.FailStep(fmt.Errorf(
                "catalog registration failed (%w) and cleanup failed (%v) — orphaned repo: %s",
                err, cleanupErr, repoResult["repo_url"],
            ))
        }
        stepAPI.CompleteStep(nil)

        return fmt.Errorf("catalog registration failed, repo cleaned up: %w", err)
    }
    stepAPI.CompleteStep(catalogResult)

    // Step 4: Completion notification
    stepAPI.BeginStep("notify-completion")
    _, err = stepAPI.CallConnector("outbox", "emit-event", map[string]any{
        "event_type":        "scaffolding.service.created",
        "repo_url":          repoResult["repo_url"],
        "catalog_entity_ref": catalogResult["entity_ref"],
    })
    if err != nil {
        return stepAPI.FailStep(err)
    }
    stepAPI.CompleteStep(nil)

    return nil
}
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `template` | string | yes | — | Template name (e.g. `go-grpc`, `node-express`, `java-spring`) |
| `service_name` | string | yes | — | Name for the new service and repository |
| `owner` | string | yes | — | Owning team identifier |
| `org` | string | yes | — | GitHub organisation |
| `parameters` | map | no | `{}` | Template-specific parameter values |
| `visibility` | string | no | `internal` | Repository visibility: `public`, `internal`, `private` |
| `callback_url` | string | no | — | Backstage webhook URL for completion notification |

## Error Handling

| Error Type | Behaviour |
|------------|-----------|
| Parameter validation failure | Terminal. `FailStep` with error code `INVALID_PARAMS`. |
| Template rendering failure | Terminal. `FailStep` with error code `RENDER_FAILED`. |
| GitHub create — transient error | Retryable by connector. Module sees final result. |
| GitHub create — naming conflict | Terminal. `FailStep` with error code `REPO_EXISTS`. |
| Catalog registration failure | Terminal. Triggers compensating cleanup (delete repo), then `FailStep` with error code `CATALOG_REGISTRATION_FAILED`. |
| Compensating cleanup failure | Terminal. `FailStep` with error code `CLEANUP_FAILED`. Orphaned repo logged for manual intervention. |
| Context cancellation | Terminal. Propagates `ctx.Err()`. |

Compensating cleanup is explicit and scoped. The module deletes the repository created in Step 2 if and only if catalog registration in Step 3 fails. This directly mirrors the workflow diagram's `7a → 7b → 7c → 7d` path.

## File Structure

```
internal/module/scaffoldservice/
├── module.go          # Module struct, Key(), Version(), Execute()
├── render.go          # renderTemplate(), starter code generation
├── helpers.go         # stringOrDefault(), formatters
└── module_test.go     # Tests with mock StepAPI
```

## Registration

In `cmd/release-engine/main.go`:

```go
registry.Register(&scaffoldservice.ScaffoldServiceModule{})
```

## Dependencies

| Dependency | Source | Usage |
|------------|--------|-------|
| `runner.StepAPI` | `internal/runner/` | Step lifecycle, connector calls |
| `text/template` | Go stdlib | Starter code and CI rendering |
| `gopkg.in/yaml.v3` | `go.mod` (existing) | catalog-info.yaml generation |

No new external dependencies required.