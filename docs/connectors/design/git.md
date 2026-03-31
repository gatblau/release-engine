# Git Connector — Pseudo Code

The connector is protocol-agnostic at the engine boundary. Provider-specific behaviour (GitHub, GitLab, Bitbucket, Gitea) is resolved internally via the credential's `provider` field. All operations are idempotent by contract and return one of `Success`, `RetryableError`, or `TerminalError`.

---

## Interface

```
CONNECTOR: GitConnector
implements Connector interface

registered name: "git"

func Call(ctx, op, params, credential) -> ConnectorResult:
  provider = credential.provider          // "github" | "gitlab" | "bitbucket" | "gitea"
  client   = resolve_client(provider, credential)
  
  switch op:
    case "create_repository":           return create_repository(ctx, client, params)
    case "delete_repository":           return delete_repository(ctx, client, params)
    case "archive_repository":          return archive_repository(ctx, client, params)
    case "get_repository":              return get_repository(ctx, client, params)
    case "list_repositories":           return list_repositories(ctx, client, params)
    case "update_repository":           return update_repository(ctx, client, params)
    case "push_files":                  return push_files(ctx, client, params)
    case "create_branch":               return create_branch(ctx, client, params)
    case "delete_branch":               return delete_branch(ctx, client, params)
    case "protect_branch":              return protect_branch(ctx, client, params)
    case "unprotect_branch":            return unprotect_branch(ctx, client, params)
    case "merge_branch":                return merge_branch(ctx, client, params)
    case "create_pull_request":         return create_pull_request(ctx, client, params)
    case "merge_pull_request":          return merge_pull_request(ctx, client, params)
    case "close_pull_request":          return close_pull_request(ctx, client, params)
    case "get_pull_request":            return get_pull_request(ctx, client, params)
    case "list_pull_requests":          return list_pull_requests(ctx, client, params)
    case "create_tag":                  return create_tag(ctx, client, params)
    case "delete_tag":                  return delete_tag(ctx, client, params)
    case "create_release":              return create_release(ctx, client, params)
    case "update_release":              return update_release(ctx, client, params)
    case "delete_release":              return delete_release(ctx, client, params)
    case "get_release":                 return get_release(ctx, client, params)
    case "add_collaborator":            return add_collaborator(ctx, client, params)
    case "remove_collaborator":         return remove_collaborator(ctx, client, params)
    case "list_collaborators":          return list_collaborators(ctx, client, params)
    case "add_deploy_key":              return add_deploy_key(ctx, client, params)
    case "remove_deploy_key":           return remove_deploy_key(ctx, client, params)
    case "create_webhook":              return create_webhook(ctx, client, params)
    case "delete_webhook":              return delete_webhook(ctx, client, params)
    case "list_webhooks":               return list_webhooks(ctx, client, params)
    case "get_file":                    return get_file(ctx, client, params)
    case "create_or_update_file":       return create_or_update_file(ctx, client, params)
    case "delete_file":                 return delete_file(ctx, client, params)
    case "get_commit":                  return get_commit(ctx, client, params)
    case "list_commits":                return list_commits(ctx, client, params)
    case "compare_commits":             return compare_commits(ctx, client, params)
    case "trigger_workflow":            return trigger_workflow(ctx, client, params)
    case "get_workflow_run":            return get_workflow_run(ctx, client, params)
    case "cancel_workflow_run":         return cancel_workflow_run(ctx, client, params)
    case "list_workflow_runs":          return list_workflow_runs(ctx, client, params)

    default:
      return TerminalError("unknown operation: " + op)
```

---

## Supported Operations

### Repository Management

---

#### `create_repository`
> Creates a new repository for an organisation or user, optionally initialised from a template. Used in new service scaffolding and project provisioning workflows.

```
func create_repository(ctx, client, params):
  // params: org, name, description, visibility, template, auto_init,
  //         default_branch, gitignore_template, license_template,
  //         has_issues, has_wiki, has_projects, team_id

  resp = client.POST("/orgs/{params.org}/repos", {
    name:                params.name,
    description:         params.description,
    private:             params.visibility == "private",
    auto_init:           params.auto_init ?? true,
    gitignore_template:  params.gitignore_template,
    license_template:    params.license_template,
    has_issues:          params.has_issues ?? true,
    has_wiki:            params.has_wiki ?? false,
    has_projects:        params.has_projects ?? false,
    team_id:             params.team_id,
  })

  if resp.status == 422 and error_is("name_already_exists", resp):
    existing = client.GET("/repos/{params.org}/{params.name}")
    return Success({ repo_url: existing.html_url, repo_id: existing.id, idempotent: true })

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  if params.template is not null:
    push_result = push_template_contents(ctx, client, resp.full_name, params.template)
    if push_result is error: return push_result

  return Success({ repo_url: resp.html_url, repo_id: resp.id })
```

---

#### `delete_repository`
> Permanently deletes a repository. Used in decommissioning and cleanup workflows. Treat as terminal — cannot be retried after success.

```
func delete_repository(ctx, client, params):
  // params: org, name

  resp = client.DELETE("/repos/{params.org}/{params.name}")

  if resp.status == 404:
    return Success({ idempotent: true })   // already gone

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({})
```

---

#### `archive_repository`
> Sets a repository to archived (read-only) state. Used in service retirement workflows where history must be preserved but active development halted.

```
func archive_repository(ctx, client, params):
  // params: org, name

  resp = client.PATCH("/repos/{params.org}/{params.name}", {
    archived: true,
  })

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({ repo_url: resp.html_url })
```

---

#### `get_repository`
> Fetches metadata for a single repository. Used for existence checks, validation steps, and read-only inspection within orchestration workflows.

```
func get_repository(ctx, client, params):
  // params: org, name

  resp = client.GET("/repos/{params.org}/{params.name}")

  if resp.status == 404:
    return TerminalError("repository_not_found")

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({
    repo_id:          resp.id,
    repo_url:         resp.html_url,
    clone_url:        resp.clone_url,
    default_branch:   resp.default_branch,
    visibility:       resp.private ? "private" : "public",
    archived:         resp.archived,
  })
```

---

#### `list_repositories`
> Lists repositories for an organisation or user with optional filtering. Used in audit, migration, and bulk-operation orchestration workflows.

```
func list_repositories(ctx, client, params):
  // params: org, type (all|public|private|forks|sources|member),
  //         sort, direction, per_page, page

  resp = client.GET("/orgs/{params.org}/repos", query={
    type:      params.type ?? "all",
    sort:      params.sort ?? "updated",
    direction: params.direction ?? "desc",
    per_page:  params.per_page ?? 100,
    page:      params.page ?? 1,
  })

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({
    repositories: resp.body.map(r => { name: r.name, repo_url: r.html_url }),
    total_count:  resp.headers["x-total-count"],
  })
```

---

#### `update_repository`
> Updates repository metadata such as description, visibility, or feature flags. Used in governance enforcement and bulk-settings workflows.

```
func update_repository(ctx, client, params):
  // params: org, name, description, visibility, has_issues,
  //         has_wiki, has_projects, default_branch

  resp = client.PATCH("/repos/{params.org}/{params.name}", {
    description:  params.description,
    private:      params.visibility == "private",
    has_issues:   params.has_issues,
    has_wiki:     params.has_wiki,
    has_projects: params.has_projects,
    default_branch: params.default_branch,
  })

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({ repo_url: resp.html_url })
```

---

### File and Content Operations

---

#### `push_files`
> Commits one or more files to a branch in a single operation. Used in scaffolding workflows to write starter code, CI configurations, and policy files into a new repository.

```
func push_files(ctx, client, params):
  // params: org, name, branch, files[{path, content}],
  //         commit_message, author_name, author_email

  for file in params.files:
    existing     = client.GET("/repos/{params.org}/{params.name}/contents/{file.path}",
                              query={ ref: params.branch })
    existing_sha = existing.status == 200 ? existing.body.sha : null

    resp = client.PUT("/repos/{params.org}/{params.name}/contents/{file.path}", {
      message:  params.commit_message,
      content:  base64_encode(file.content),
      branch:   params.branch,
      sha:      existing_sha,             // required for update; omit for create
      author: {
        name:   params.author_name,
        email:  params.author_email,
      },
    })

    if resp.status == 409 and error_is("sha_mismatch", resp):
      return RetryableError("sha_conflict — re-fetch and retry")

    if resp.status in [500, 502, 503, 429]:
      return RetryableError(resp.error)

    if resp.status >= 400:
      return TerminalError(resp.error)

  return Success({ branch: params.branch })
```

---

#### `get_file`
> Retrieves the content and metadata of a single file at a given ref. Used in validation, templating, and inspection workflows.

```
func get_file(ctx, client, params):
  // params: org, name, path, ref (branch, tag, or commit sha)

  resp = client.GET("/repos/{params.org}/{params.name}/contents/{params.path}",
                    query={ ref: params.ref })

  if resp.status == 404:
    return TerminalError("file_not_found")

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({
    path:    resp.body.path,
    sha:     resp.body.sha,
    content: base64_decode(resp.body.content),
  })
```

---

#### `create_or_update_file`
> Creates or updates a single file at an explicit path. Used when individual file writes are needed with explicit SHA control, such as updating a manifest or config file.

```
func create_or_update_file(ctx, client, params):
  // params: org, name, path, content, branch,
  //         commit_message, author_name, author_email

  existing     = client.GET("/repos/{params.org}/{params.name}/contents/{params.path}",
                            query={ ref: params.branch })
  existing_sha = existing.status == 200 ? existing.body.sha : null

  resp = client.PUT("/repos/{params.org}/{params.name}/contents/{params.path}", {
    message:  params.commit_message,
    content:  base64_encode(params.content),
    branch:   params.branch,
    sha:      existing_sha,
    author: {
      name:   params.author_name,
      email:  params.author_email,
    },
  })

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({ path: params.path, commit_sha: resp.body.commit.sha })
```

---

#### `delete_file`
> Deletes a single file from a branch. Used in cleanup, deprecation, and automated housekeeping workflows.

```
func delete_file(ctx, client, params):
  // params: org, name, path, branch, commit_message, author_name, author_email

  existing = client.GET("/repos/{params.org}/{params.name}/contents/{params.path}",
                        query={ ref: params.branch })

  if existing.status == 404:
    return Success({ idempotent: true })   // already absent

  resp = client.DELETE("/repos/{params.org}/{params.name}/contents/{params.path}", {
    message: params.commit_message,
    sha:     existing.body.sha,
    branch:  params.branch,
    author: {
      name:  params.author_name,
      email: params.author_email,
    },
  })

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({ commit_sha: resp.body.commit.sha })
```

---

### Branch Operations

---

#### `create_branch`
> Creates a new branch from a given base ref. Used in release preparation, feature scaffolding, and automated change workflows.

**Note:** GitHub and Gitea use different URL shapes for single ref lookups:
- GitHub: `/repos/{owner}/{repo}/git/ref/heads/{ref}`
- Gitea: `/repos/{owner}/{repo}/git/refs/heads/{ref}`

The connector implements automatic fallback: try GitHub's path first; if 404, retry with Gitea's path.

```
func create_branch(ctx, client, params):
  // params: org, name, branch, base_ref

  // GitHub path first; if 404, fallback to Gitea path
  base = client.GET("/repos/{params.org}/{params.name}/git/ref/heads/{params.base_ref}")
  if base.status == 404:
    base = client.GET("/repos/{params.org}/{params.name}/git/refs/heads/{params.base_ref}")
  if base.status >= 400:
    return TerminalError(base.error)
  base_sha = base.body.object.sha

  resp = client.POST("/repos/{params.org}/{params.name}/git/refs", {
    ref: "refs/heads/{params.branch}",
    sha: base_sha,
  })

  if resp.status == 422 and error_is("reference_already_exists", resp):
    return Success({ branch: params.branch, idempotent: true })

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({ branch: params.branch, sha: base_sha })
```

---

#### `delete_branch`
> Deletes a branch by name. Used in cleanup workflows after a pull request merge or a release cut.

```
func delete_branch(ctx, client, params):
  // params: org, name, branch

  resp = client.DELETE("/repos/{params.org}/{params.name}/git/refs/heads/{params.branch}")

  if resp.status == 422:
    return Success({ idempotent: true })   // already deleted

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({})
```

---

#### `protect_branch`
> Applies branch protection rules including required reviews, status checks, and push restrictions. Used in governance workflows after repository creation or policy enforcement runs.

```
func protect_branch(ctx, client, params):
  // params: org, name, branch, required_approvals,
  //         required_status_checks[], dismiss_stale_reviews,
  //         enforce_admins, restrict_push_to_teams[]

  resp = client.PUT("/repos/{params.org}/{params.name}/branches/{params.branch}/protection", {
    required_pull_request_reviews: {
      required_approving_review_count: params.required_approvals ?? 1,
      dismiss_stale_reviews:           params.dismiss_stale_reviews ?? true,
    },
    required_status_checks: {
      strict:   true,
      contexts: params.required_status_checks ?? [],
    },
    enforce_admins:                params.enforce_admins ?? true,
    restrictions: params.restrict_push_to_teams ? {
      teams: params.restrict_push_to_teams,
      users: [],
    } : null,
  })

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({ branch: params.branch })
```

---

#### `unprotect_branch`
> Removes all branch protection rules. Used in emergency break-glass or repository decommissioning workflows.

```
func unprotect_branch(ctx, client, params):
  // params: org, name, branch

  resp = client.DELETE("/repos/{params.org}/{params.name}/branches/{params.branch}/protection")

  if resp.status == 404:
    return Success({ idempotent: true })

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({})
```

---

#### `merge_branch`
> Merges a head branch into a base branch directly via the API. Used in automated release promotion and environment sync workflows.

```
func merge_branch(ctx, client, params):
  // params: org, name, base, head, commit_message

  resp = client.POST("/repos/{params.org}/{params.name}/merges", {
    base:           params.base,
    head:           params.head,
    commit_message: params.commit_message,
  })

  if resp.status == 204:
    return Success({ idempotent: true })   // already up to date

  if resp.status == 409:
    return TerminalError("merge_conflict")

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({ merge_commit_sha: resp.body.sha })
```

---

### Pull Request Operations

---

#### `create_pull_request`
> Opens a pull request between two branches. Used in automated change delivery workflows such as dependency updates, config propagation, and scaffolding amendments.

```
func create_pull_request(ctx, client, params):
  // params: org, name, title, body, head, base, draft, labels[]

  resp = client.POST("/repos/{params.org}/{params.name}/pulls", {
    title:  params.title,
    body:   params.body,
    head:   params.head,
    base:   params.base,
    draft:  params.draft ?? false,
  })

  if resp.status == 422 and error_is("pull_request_already_exists", resp):
    existing = find_open_pull_request(client, params.org, params.name, params.head, params.base)
    return Success({ pr_number: existing.number, pr_url: existing.html_url, idempotent: true })

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  if params.labels:
    client.POST("/repos/{params.org}/{params.name}/issues/{resp.body.number}/labels",
                { labels: params.labels })

  return Success({ pr_number: resp.body.number, pr_url: resp.body.html_url })
```

---

#### `merge_pull_request`
> Merges an open pull request using a specified merge strategy. Used in release automation and change delivery workflows after required checks have passed.

```
func merge_pull_request(ctx, client, params):
  // params: org, name, pr_number, merge_method (merge|squash|rebase),
  //         commit_title, commit_message

  resp = client.PUT("/repos/{params.org}/{params.name}/pulls/{params.pr_number}/merge", {
    merge_method:   params.merge_method ?? "squash",
    commit_title:   params.commit_title,
    commit_message: params.commit_message,
  })

  if resp.status == 405:
    return TerminalError("pull_request_not_mergeable")

  if resp.status == 409:
    return TerminalError("pull_request_already_merged")

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({ merge_commit_sha: resp.body.sha })
```

---

#### `close_pull_request`
> Closes an open pull request without merging. Used in rollback, cancellation, and cleanup workflows.

```
func close_pull_request(ctx, client, params):
  // params: org, name, pr_number

  resp = client.PATCH("/repos/{params.org}/{params.name}/pulls/{params.pr_number}", {
    state: "closed",
  })

  if resp.status == 422 and error_is("already_closed", resp):
    return Success({ idempotent: true })

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({ pr_number: params.pr_number })
```

---

#### `get_pull_request`
> Fetches the current state and metadata of a pull request. Used in orchestration workflows that gate on PR status before proceeding to subsequent steps.

```
func get_pull_request(ctx, client, params):
  // params: org, name, pr_number

  resp = client.GET("/repos/{params.org}/{params.name}/pulls/{params.pr_number}")

  if resp.status == 404:
    return TerminalError("pull_request_not_found")

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({
    pr_number:   resp.body.number,
    state:       resp.body.state,
    merged:      resp.body.merged,
    mergeable:   resp.body.mergeable,
    head_sha:    resp.body.head.sha,
    pr_url:      resp.body.html_url,
  })
```

---

#### `list_pull_requests`
> Lists pull requests for a repository with optional state and branch filters. Used in audit, reporting, and bulk-operation workflows.

```
func list_pull_requests(ctx, client, params):
  // params: org, name, state (open|closed|all), head, base, per_page, page

  resp = client.GET("/repos/{params.org}/{params.name}/pulls", query={
    state:    params.state ?? "open",
    head:     params.head,
    base:     params.base,
    per_page: params.per_page ?? 100,
    page:     params.page ?? 1,
  })

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({
    pull_requests: resp.body.map(pr => {
      number: pr.number, state: pr.state, pr_url: pr.html_url
    }),
  })
```

---

### Tag and Release Operations

---

#### `create_tag`
> Creates a lightweight or annotated tag at a given commit SHA. Used in release workflows to mark stable points in history.

```
func create_tag(ctx, client, params):
  // params: org, name, tag, sha, message (annotated if present), tagger_name, tagger_email

  if params.message is not null:
    tag_resp = client.POST("/repos/{params.org}/{params.name}/git/tags", {
      tag:     params.tag,
      message: params.message,
      object:  params.sha,
      type:    "commit",
      tagger: {
        name:  params.tagger_name,
        email: params.tagger_email,
        date:  now_iso8601(),
      },
    })
    tag_sha = tag_resp.body.sha

    ref_resp = client.POST("/repos/{params.org}/{params.name}/git/refs", {
      ref: "refs/tags/{params.tag}",
      sha: tag_sha,
    })
  else:
    ref_resp = client.POST("/repos/{params.org}/{params.name}/git/refs", {
      ref: "refs/tags/{params.tag}",
      sha: params.sha,
    })

  if ref_resp.status == 422 and error_is("reference_already_exists", ref_resp):
    return Success({ tag: params.tag, idempotent: true })

  if ref_resp.status in [500, 502, 503, 429]:
    return RetryableError(ref_resp.error)

  if ref_resp.status >= 400:
    return TerminalError(ref_resp.error)

  return Success({ tag: params.tag })
```

---

#### `delete_tag`
> Deletes a tag by name. Used in release rollback and cleanup workflows.

```
func delete_tag(ctx, client, params):
  // params: org, name, tag

  resp = client.DELETE("/repos/{params.org}/{params.name}/git/refs/tags/{params.tag}")

  if resp.status == 422:
    return Success({ idempotent: true })

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({})
```

---

#### `create_release`
> Creates a release record associated with a tag, with a changelog body and pre-release flag. Used as the final step in release publication workflows.

```
func create_release(ctx, client, params):
  // params: org, name, tag, target_commitish, release_name,
  //         body, draft, prerelease

  resp = client.POST("/repos/{params.org}/{params.name}/releases", {
    tag_name:         params.tag,
    target_commitish: params.target_commitish ?? "main",
    name:             params.release_name,
    body:             params.body,
    draft:            params.draft ?? false,
    prerelease:       params.prerelease ?? false,
  })

  if resp.status == 422 and error_is("already_exists", resp):
    existing = find_release_by_tag(client, params.org, params.name, params.tag)
    return Success({ release_id: existing.id, release_url: existing.html_url, idempotent: true })

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({ release_id: resp.body.id, release_url: resp.body.html_url })
```

---

#### `update_release`
> Updates the body, name, or published state of an existing release. Used to attach generated changelogs or promote a draft release after asset upload.

```
func update_release(ctx, client, params):
  // params: org, name, release_id, release_name, body, draft, prerelease

  resp = client.PATCH("/repos/{params.org}/{params.name}/releases/{params.release_id}", {
    name:       params.release_name,
    body:       params.body,
    draft:      params.draft,
    prerelease: params.prerelease,
  })

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({ release_id: resp.body.id, release_url: resp.body.html_url })
```

---

#### `delete_release`
> Deletes a release record without deleting the associated tag. Used in release rollback workflows.

```
func delete_release(ctx, client, params):
  // params: org, name, release_id

  resp = client.DELETE("/repos/{params.org}/{params.name}/releases/{params.release_id}")

  if resp.status == 404:
    return Success({ idempotent: true })

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({})
```

---

#### `get_release`
> Fetches the metadata of a release by ID or by tag. Used in workflows that gate on release state before proceeding, such as asset attachment or promotion.

```
func get_release(ctx, client, params):
  // params: org, name, release_id OR tag

  if params.tag is not null:
    resp = client.GET("/repos/{params.org}/{params.name}/releases/tags/{params.tag}")
  else:
    resp = client.GET("/repos/{params.org}/{params.name}/releases/{params.release_id}")

  if resp.status == 404:
    return TerminalError("release_not_found")

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({
    release_id:  resp.body.id,
    tag:         resp.body.tag_name,
    draft:       resp.body.draft,
    prerelease:  resp.body.prerelease,
    release_url: resp.body.html_url,
  })
```

---

### Collaborator and Access Operations

---

#### `add_collaborator`
> Grants a user or team access to a repository at a specified permission level. Used in provisioning workflows when a new service is created for a team.

```
func add_collaborator(ctx, client, params):
  // params: org, name, username, permission (pull|triage|push|maintain|admin)

  resp = client.PUT("/repos/{params.org}/{params.name}/collaborators/{params.username}", {
    permission: params.permission ?? "push",
  })

  if resp.status in [201, 204]:
    return Success({ username: params.username, idempotent: resp.status == 204 })

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({ username: params.username })
```

---

#### `remove_collaborator`
> Revokes a user's access to a repository. Used in offboarding and access review workflows.

```
func remove_collaborator(ctx, client, params):
  // params: org, name, username

  resp = client.DELETE("/repos/{params.org}/{params.name}/collaborators/{params.username}")

  if resp.status == 404:
    return Success({ idempotent: true })

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({})
```

---

#### `list_collaborators`
> Lists all collaborators and their permission levels for a repository. Used in access audit and compliance reporting workflows.

```
func list_collaborators(ctx, client, params):
  // params: org, name, affiliation (outside|direct|all), per_page, page

  resp = client.GET("/repos/{params.org}/{params.name}/collaborators", query={
    affiliation: params.affiliation ?? "all",
    per_page:    params.per_page ?? 100,
    page:        params.page ?? 1,
  })

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({
    collaborators: resp.body.map(c => { username: c.login, permission: c.role_name }),
  })
```

---

#### `add_deploy_key`
> Adds a read-only or read-write SSH deploy key to a repository. Used in CI/CD provisioning workflows to grant deployment pipelines access without using personal credentials.

```
func add_deploy_key(ctx, client, params):
  // params: org, name, title, key (public SSH key), read_only

  resp = client.POST("/repos/{params.org}/{params.name}/keys", {
    title:     params.title,
    key:       params.key,
    read_only: params.read_only ?? true,
  })

  if resp.status == 422 and error_is("key_already_exists", resp):
    return Success({ idempotent: true })

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({ key_id: resp.body.id })
```

---

#### `remove_deploy_key`
> Removes a deploy key from a repository by key ID. Used in key rotation and decommissioning workflows.

```
func remove_deploy_key(ctx, client, params):
  // params: org, name, key_id

  resp = client.DELETE("/repos/{params.org}/{params.name}/keys/{params.key_id}")

  if resp.status == 404:
    return Success({ idempotent: true })

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({})
```

---

### Webhook Operations

---

#### `create_webhook`
> Registers a webhook on a repository to deliver event payloads to an external endpoint. Used in provisioning workflows to wire up CI, deployment, and notification pipelines.

```
func create_webhook(ctx, client, params):
  // params: org, name, url, secret, events[], active, content_type

  resp = client.POST("/repos/{params.org}/{params.name}/hooks", {
    name:   "web",
    active: params.active ?? true,
    events: params.events ?? ["push", "pull_request", "release"],
    config: {
      url:          params.url,
      secret:       params.secret,
      content_type: params.content_type ?? "json",
      insecure_ssl: "0",
    },
  })

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({ hook_id: resp.body.id })
```

---

#### `delete_webhook`
> Removes a webhook from a repository by hook ID. Used in cleanup and decommissioning workflows.

```
func delete_webhook(ctx, client, params):
  // params: org, name, hook_id

  resp = client.DELETE("/repos/{params.org}/{params.name}/hooks/{params.hook_id}")

  if resp.status == 404:
    return Success({ idempotent: true })

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({})
```

---

#### `list_webhooks`
> Lists all webhooks configured on a repository. Used in audit and idempotency-check workflows before registering new hooks.

```
func list_webhooks(ctx, client, params):
  // params: org, name

  resp = client.GET("/repos/{params.org}/{params.name}/hooks")

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({
    webhooks: resp.body.map(h => { hook_id: h.id, url: h.config.url, events: h.events }),
  })
```

---

### Commit Operations

---

#### `get_commit`
> Fetches the metadata and diff summary for a single commit by SHA. Used in change validation, audit, and changelog generation workflows.

```
func get_commit(ctx, client, params):
  // params: org, name, sha

  resp = client.GET("/repos/{params.org}/{params.name}/commits/{params.sha}")

  if resp.status == 404:
    return TerminalError("commit_not_found")

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({
    sha:       resp.body.sha,
    message:   resp.body.commit.message,
    author:    resp.body.commit.author.name,
    timestamp: resp.body.commit.author.date,
    url:       resp.body.html_url,
  })
```

---

#### `list_commits`
> Lists commits on a branch with optional author and path filters. Used in changelog generation, audit, and drift-detection workflows.

```
func list_commits(ctx, client, params):
  // params: org, name, sha (branch or sha), path, author, since, until, per_page, page

  resp = client.GET("/repos/{params.org}/{params.name}/commits", query={
    sha:      params.sha,
    path:     params.path,
    author:   params.author,
    since:    params.since,
    until:    params.until,
    per_page: params.per_page ?? 100,
    page:     params.page ?? 1,
  })

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({
    commits: resp.body.map(c => { sha: c.sha, message: c.commit.message }),
  })
```

---

#### `compare_commits`
> Computes the diff and commit list between two refs. Used in release note generation and change impact assessment workflows.

```
func compare_commits(ctx, client, params):
  // params: org, name, base, head

  resp = client.GET("/repos/{params.org}/{params.name}/compare/{params.base}...{params.head}")

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({
    status:        resp.body.status,      // ahead | behind | diverged | identical
    ahead_by:      resp.body.ahead_by,
    behind_by:     resp.body.behind_by,
    commits:       resp.body.commits.map(c => { sha: c.sha, message: c.commit.message }),
    diff_url:      resp.body.diff_url,
  })
```

---

### CI / Workflow Operations

---

#### `trigger_workflow`
> Dispatches a CI workflow run via the `workflow_dispatch` event. Used in release promotion, environment deployment, and on-demand testing workflows.

```
func trigger_workflow(ctx, client, params):
  // params: org, name, workflow_id (filename or numeric id),
  //         ref, inputs{}

  resp = client.POST(
    "/repos/{params.org}/{params.name}/actions/workflows/{params.workflow_id}/dispatches", {
      ref:    params.ref,
      inputs: params.inputs ?? {},
  })

  if resp.status == 204:
    return Success({ triggered: true })

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({ triggered: true })
```

---

#### `get_workflow_run`
> Fetches the status and conclusion of a specific workflow run by run ID. Used in orchestration workflows that must gate on CI completion before proceeding.

```
func get_workflow_run(ctx, client, params):
  // params: org, name, run_id

  resp = client.GET("/repos/{params.org}/{params.name}/actions/runs/{params.run_id}")

  if resp.status == 404:
    return TerminalError("workflow_run_not_found")

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({
    run_id:     resp.body.id,
    status:     resp.body.status,        // queued | in_progress | completed
    conclusion: resp.body.conclusion,    // success | failure | cancelled | null
    run_url:    resp.body.html_url,
  })
```

---

#### `cancel_workflow_run`
> Cancels an in-progress workflow run. Used in rollback and abort workflows when a downstream step has already failed.

```
func cancel_workflow_run(ctx, client, params):
  // params: org, name, run_id

  resp = client.POST(
    "/repos/{params.org}/{params.name}/actions/runs/{params.run_id}/cancel")

  if resp.status == 409:
    return Success({ idempotent: true })   // already completed or cancelled

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({ run_id: params.run_id })
```

---

#### `list_workflow_runs`
> Lists workflow runs for a repository or specific workflow with status and branch filters. Used in monitoring, audit, and retry-orchestration workflows.

```
func list_workflow_runs(ctx, client, params):
  // params: org, name, workflow_id (optional), branch, status,
  //         created, per_page, page

  base_path = params.workflow_id
    ? "/repos/{params.org}/{params.name}/actions/workflows/{params.workflow_id}/runs"
    : "/repos/{params.org}/{params.name}/actions/runs"

  resp = client.GET(base_path, query={
    branch:   params.branch,
    status:   params.status,    // queued | in_progress | completed | success | failure
    created:  params.created,
    per_page: params.per_page ?? 100,
    page:     params.page ?? 1,
  })

  if resp.status in [500, 502, 503, 429]:
    return RetryableError(resp.error)

  if resp.status >= 400:
    return TerminalError(resp.error)

  return Success({
    runs: resp.body.workflow_runs.map(r => {
      run_id:     r.id,
      status:     r.status,
      conclusion: r.conclusion,
      run_url:    r.html_url,
    }),
    total_count: resp.body.total_count,
  })
```

---

## Error Classification Reference

| HTTP Status | Classification | Engine Behaviour |
|---|---|---|
| `2xx` | Success | Advance to next step |
| `304` | Success (not modified) | Treat as idempotent success |
| `404` on delete/get | Success (idempotent) | Already absent — treat as done |
| `409` conflict | Context-dependent | Merge conflict → Terminal; already merged → Success |
| `422` already exists | Success (idempotent) | Re-fetch and return existing resource |
| `429` rate limited | RetryableError | Re-enqueue with backoff |
| `4xx` other | TerminalError | Do not retry — fail the job step |
| `5xx` | RetryableError | Re-enqueue with exponential backoff |
| Network timeout | RetryableError | Re-enqueue with backoff |