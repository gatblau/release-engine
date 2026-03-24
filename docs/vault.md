# Secret Retrieval Proposal

## The Problem

Volta's `UseSecret` pattern exists for a reason:

```go
vault.UseSecret("github-token", func(data []byte) error {
    // secret exists only within this scope
    // vault handles zeroing memory after fn returns
    return doSomethingWith(data)
})
```

If you resolve secrets externally and pass them as strings:

```go
// Secret now lives in connector memory indefinitely
connector = NewGitHubConnector(token)
```

You've **defeated the secure disposal mechanism.** The token sits in heap memory, subject to GC timing, potentially copied across goroutines, visible in memory dumps.

## The Real Question

You're choosing between:
1. **Clean architecture** — connector doesn't know about vault
2. **Secure secret handling** — vault controls secret lifecycle

**You need both.**

## Proposed Solution: Scoped Execution

Don't pass the secret to the connector. Pass the connector to the secret.

```go
type ConnectorSecretRequirements struct {
    Keys []string
}

func (c *GitHubConnector) RequiredSecrets() ConnectorSecretRequirements {
    return ConnectorSecretRequirements{
        Keys: []string{"github-token"},
    }
}

// Runtime orchestrates scoped execution
func (r *Runtime) ExecuteConnectorOperation(op Operation) error {
    reqs := op.Connector.RequiredSecrets()

    return r.vault.UseSecret(reqs.Keys[0], func(data []byte) error {
        // Connector receives secret, uses it, returns
        // Vault cleans up after this scope exits
        return op.Connector.ExecuteWithCredentials(data, op.Params)
    })
}
```

The connector **declares** what it needs and **receives** credentials within a scoped callback. The vault still controls the lifecycle.

## What This Gives You

- Connector doesn't depend on vault
- Vault still manages memory disposal
- Runtime orchestrates the binding
- Connector is still testable with raw byte slices
- Secret never persists in connector state

## The Tradeoff

The connector can't hold a long-lived authenticated client. Each operation gets scoped credentials. If that's a performance concern, you'd need to evaluate whether Volta supports leased/renewable references — but for GitHub API calls, per-operation authentication is perfectly fine.

**This is the pattern your architecture is pointing toward.** The connector defines requirements, the runtime resolves them, and the vault controls the secret lifecycle throughout.