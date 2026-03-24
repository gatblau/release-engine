

# Volta Integration Plan for Secure Secret Handling

## Executive Summary

This document outlines a plan to integrate Volta secret management with the Release Engine connector framework. The goal is to enable secure, scoped secret retrieval for connector operations while maintaining backward compatibility and performance.

**Key Outcomes:**
- Connectors receive secrets at operation execution time, not construction time
- Volta controls secret lifecycle with secure memory disposal
- Single HTTP client reuse with per-operation authentication
- Unified connector interface with optional secret declaration
- Module-owned tenant resolution via `SecretContext` interface
- Gradual migration path with minimal breaking changes

## Problem Statement

The current connector architecture creates security risks:

1. **Secret Persistence**: GitHub connector stores tokens in heap memory indefinitely
2. **Defeated Secure Disposal**: Volta's `UseSecret` callback pattern is bypassed
3. **Hidden Dependencies**: No clear declaration of secret requirements
4. **Architecture Mismatch**: Clean separation vs. secure lifecycle trade-off
5. **No Tenant Awareness**: Secret resolution has no concept of which tenant context applies

As outlined in `vault.md`, passing secrets as strings to connectors defeats Volta's secure disposal mechanism. Tokens remain in memory subject to GC timing, potentially visible in memory dumps.

## Current Architecture Analysis

### Connector Interface
```go
type Connector interface {
    Execute(ctx context.Context, operation string, input map[string]interface{}) (*ConnectorResult, error)
}
```

### GitHub Connector Pattern
```go
// Token baked into connector instance
func NewGitHubConnector(cfg connector.ConnectorConfig, token string) (*GitHubConnector, error)
```

### Execution Flow
1. StepAPI adapter calls `CallConnector`
2. Connector resolved from family registry
3. Direct `Execute()` with no secret injection
4. GitHub connector uses pre-baked token

### Volta Manager
- Provides `UseSecret(secretKey string, fn func(data []byte) error) error`
- Provides `UseSecrets(secretKeys []string, fn func(secrets map[string][]byte) error) error`
- Supports AWS Secrets Manager and file-based storage
- Handles secure memory zeroing after callback
- Not integrated with connector execution

## Proposed Solution

### Core Principles

**1. Scoped Execution**: Don't pass secrets to connectors. Pass connectors to secrets within Volta's scoped callback.

**2. Module-Owned Tenant Context**: The module — not the runtime, not the connector, not external config — determines which tenant's secrets to use. The module is the only component that understands *why* it needs a particular tenant's secrets.

**3. Connector-Declared Requirements**: Connectors declare *what* secrets they need. Modules declare *whose* secrets to use. The runtime combines both.

### Tenant Resolution Architecture

#### The Problem

Different modules need secrets from different tenant contexts:

- **Infra module**: Connects to Git for ArgoCD as part of the platform → tenant `platform`
- **Scaffold module**: Connects to Git for a customer's repository → tenant `customer-x`

This is module-domain knowledge. The runtime has no basis to determine it.

#### The Solution: Module SecretContext Interface

Modules expose an API for the runtime to query:

```go
type SecretContextProvider interface {
    SecretContext() SecretContext
}

type SecretContext struct {
    TenantID string
}
```

Each module implementation owns its tenant resolution logic:

```go
// Infra module — always platform
func (m *InfraModule) SecretContext() SecretContext {
    return SecretContext{
        TenantID: "platform",
    }
}

// Scaffold module — derived from its own inputs
func (m *ScaffoldModule) SecretContext() SecretContext {
    return SecretContext{
        TenantID: m.input.CustomerID,
    }
}
```

The module validates its own tenant context. If a scaffold module receives an empty or invalid `CustomerID`, the module rejects it — the runtime doesn't need to know the validation rules.

#### Resolution Flow

```
Module.SecretContext() → TenantID
Connector.RequiredSecrets(op) → ["github-token"]
Runtime combines → "tenants/platform/github-token"
Volta resolves → secret bytes
Connector.Execute() receives → secrets map with logical keys
```

The physical-to-logical key remapping is critical: connectors always receive logical keys regardless of how Volta stores them.

```go
// Runtime resolves physical keys but delivers logical keys
physicalToLogical := make(map[string]string)
physicalKeys := make([]string, len(requiredSecrets))
for i, logicalKey := range requiredSecrets {
    physical := fmt.Sprintf("tenants/%s/%s", tenantID, logicalKey)
    physicalKeys[i] = physical
    physicalToLogical[physical] = logicalKey
}

err := volta.UseSecrets(physicalKeys, func(secrets map[string][]byte) error {
    logicalSecrets := make(map[string][]byte, len(secrets))
    for physical, value := range secrets {
        logicalSecrets[physicalToLogical[physical]] = value
    }
    return conn.Execute(ctx, operation, input, logicalSecrets)
})
```

### Revised Connector Interface

```go
type Connector interface {
    Execute(ctx context.Context, operation string, input map[string]interface{}, secrets map[string][]byte) (*ConnectorResult, error)
}

// Optional interface for secret declaration
type SecretRequirer interface {
    RequiredSecrets(operation string) []string
}
```

Key design decisions:
- `secrets` parameter is `map[string][]byte` — logical keys only, connectors never see tenant paths
- `RequiredSecrets` is a separate optional interface, not part of the main `Connector` interface, allowing gradual adoption
- Connectors that don't need secrets receive an empty map

### Revised GitHub Connector

```go
type GitHubConnector struct {
    client *http.Client // reusable, no auth baked in
    apiURL string
}

func NewGitHubConnector(cfg connector.ConnectorConfig) (*GitHubConnector, error) {
    return &GitHubConnector{
        client: &http.Client{Timeout: 30 * time.Second},
        apiURL: cfg.Settings["api_url"].(string),
    }, nil
}

func (g *GitHubConnector) RequiredSecrets(operation string) []string {
    return []string{"github-token"}
}

func (g *GitHubConnector) Execute(ctx context.Context, operation string, input map[string]interface{}, secrets map[string][]byte) (*ConnectorResult, error) {
    token := secrets["github-token"]
    if token == nil {
        return nil, fmt.Errorf("missing required secret: github-token")
    }

    req, err := http.NewRequestWithContext(ctx, "GET", g.apiURL+"/some/endpoint", nil)
    if err != nil {
        return nil, err
    }
    req.Header.Set("Authorization", "Bearer "+string(token))

    resp, err := g.client.Do(req)
    // ... handle response
}
```

### Revised CallConnector Flow

```go
func (a *StepAPIAdapter) CallConnector(ctx context.Context, req ConnectorRequest) (*connector.ConnectorResult, error) {
    conn, err := a.connectorRegistry.GetConnector(req.ConnectorFamily, req.ConnectorType)
    if err != nil {
        return nil, fmt.Errorf("connector resolution failed: %w", err)
    }

    // Determine required secrets
    var requiredSecrets []string
    if secretReq, ok := conn.(connector.SecretRequirer); ok {
        requiredSecrets = secretReq.RequiredSecrets(req.Operation)
    }

    // No secrets needed — execute directly
    if len(requiredSecrets) == 0 {
        return conn.Execute(ctx, req.Operation, req.Input, nil)
    }

    // Resolve tenant context from module
    secretCtx := a.module.SecretContext()

    // Build physical keys, maintain logical mapping
    physicalToLogical := make(map[string]string, len(requiredSecrets))
    physicalKeys := make([]string, len(requiredSecrets))
    for i, logicalKey := range requiredSecrets {
        physical := fmt.Sprintf("tenants/%s/%s", secretCtx.TenantID, logicalKey)
        physicalKeys[i] = physical
        physicalToLogical[physical] = logicalKey
    }

    // Execute within Volta's secure scope
    var result *connector.ConnectorResult
    err = a.vaultManager.UseSecrets(physicalKeys, func(secrets map[string][]byte) error {
        // Remap to logical keys for connector
        logicalSecrets := make(map[string][]byte, len(secrets))
        for physical, value := range secrets {
            logicalSecrets[physicalToLogical[physical]] = value
        }
        var execErr error
        result, execErr = conn.Execute(ctx, req.Operation, req.Input, logicalSecrets)
        return execErr
    })

    return result, err
}
```

### Security Gap Acknowledgment

The `string(token)` conversion in connector implementations creates a copy that escapes Volta's secure disposal. This is a known Go limitation — `http.Header.Set` requires a string. Mitigations:

1. Volta zeroes the original `[]byte` after callback return
2. The string copy is short-lived (eligible for GC after request completes)
3. Future: custom `http.Transport` that works with `[]byte` directly

This is an acceptable trade-off. The current architecture stores tokens indefinitely in struct fields. Scoped execution reduces exposure from "lifetime of process" to "duration of single operation."

## Implementation Plan

### Phase 1: Foundation (Week 1)

**Objective**: Core interfaces and tenant resolution.

**Tasks:**
1. Define `SecretContextProvider` interface
2. Update `Connector` interface with `secrets` parameter
3. Define `SecretRequirer` optional interface
4. Implement physical/logical key resolution in `StepAPIAdapter`
5. Implement `SecretContext()` on existing modules (infra → `platform`)
6. Update all existing connectors to accept new `Execute` signature
7. Update all tests and mocks

**Deliverables:**
- Updated interfaces
- All existing code compiles and tests pass
- Connectors receive empty secret maps (no Volta wiring yet)

### Phase 2: Volta Integration (Week 2)

**Objective**: Wire Volta into the connector execution path.

**Tasks:**
1. Integrate `UseSecrets` call in `CallConnector`
2. Implement physical-to-logical key remapping
3. Update GitHub connector to use injected secrets
4. Remove token parameter from `NewGitHubConnector`
5. Add integration tests with file-based Volta backend
6. Benchmark latency impact

**Deliverables:**
- GitHub connector operating through Volta scoped execution
- No secrets stored in connector struct fields
- Latency benchmarks establishing baseline

### Phase 3: Multi-Tenant Validation (Week 3)

**Objective**: Validate tenant resolution across module types.

**Tasks:**
1. Implement `SecretContext()` on scaffold module with dynamic tenant
2. Add tenant validation within modules
3. Test cross-tenant isolation (module A cannot access module B's tenant secrets)
4. Test error paths (missing secrets, invalid tenant, Volta failures)
5. Add audit logging for secret access (tenant, keys accessed, timestamp)
6. Security review

**Deliverables:**
- Multi-tenant secret resolution working end-to-end
- Audit trail for secret access
- Security review sign-off

### Phase 4: Secrets Management API (Week 4)

**Objective**: Enable secret provisioning through the release engine.

**Tasks:**
1. Implement secrets API endpoints:
    - `PUT /api/v1/tenants/{tenant}/secrets/{key}` — set secret
    - `GET /api/v1/tenants/{tenant}/secrets` — list keys (never values)
    - `DELETE /api/v1/tenants/{tenant}/secrets/{key}` — remove secret
2. Implement tenant-scoped authorization (platform secrets require elevated permissions)
3. Add audit logging for all write/delete operations
4. Validate secret keys against known connector declarations
5. Documentation and runbook

**Deliverables:**
- Self-service secret management for tenant teams
- Platform secret management for infrastructure team
- Complete audit trail
- Operational documentation

## Risk Assessment

| Risk | Impact | Mitigation |
|------|--------|------------|
| Breaking interface change | High (guaranteed break) | Few existing connectors; update all simultaneously in Phase 1 |
| Performance degradation | Medium | Connection pooling on HTTP clients, benchmark in Phase 2 |
| Secret scope mismatch | High | Explicit testing of multi-call operations within single Volta scope |
| Memory leakage via string conversion | Medium | Accepted trade-off; document; scope limits exposure window |
| Invalid tenant resolution | High | Module-owned validation; modules reject bad inputs before runtime acts |
| Volta availability | High | Error propagation; connectors fail clearly on missing secrets |
| Complex error handling | Medium | Comprehensive error scenarios in Phase 3 |

## Success Metrics

1. **Security**: No secrets stored in long-lived struct fields after operation completion (verifiable via code audit and heap analysis)
2. **Performance**: <10% latency increase for GitHub operations (measured via benchmarking in Phase 2)
3. **Reliability**: 99.9% secret retrieval success rate (measured via monitoring)
4. **Adoption**: All new connectors use secret injection pattern (verifiable via code review)
5. **Tenant Isolation**: No cross-tenant secret access possible (verifiable via integration tests)
6. **Operational**: Secrets provisioned via API, not manual side-channels (verifiable via audit logs)

## Appendix

### A. Secret Key Naming Convention

Connectors declare logical keys. The runtime prefixes with tenant path. This convention is enforced by the resolution flow — connectors cannot bypass it.

```
Logical (connector declares):  github-token, gitlab-ssh-key, aws-access-key
Physical (Volta resolves):     tenants/{tenant-id}/github-token
```

### B. Multi-Secret Operations

Some operations require multiple secrets:

```go
func (c *SomeConnector) RequiredSecrets(operation string) []string {
    switch operation {
    case "deploy-encrypted":
        return []string{"api-token", "encryption-key"}
    default:
        return []string{"api-token"}
    }
}
```

All declared secrets are fetched in a single `UseSecrets` call — one Volta scope per operation.

### C. Volta Scope Lifetime

Critical requirement: Volta's `UseSecrets` callback must encompass the entire connector operation, not individual API calls within an operation. The current design satisfies this — the `Execute` call happens entirely within the callback.

### D. Module SecretContext Examples

```go
// Static tenant — infra module always uses platform secrets
func (m *InfraModule) SecretContext() SecretContext {
    return SecretContext{TenantID: "platform"}
}

// Dynamic tenant — scaffold module resolves from its inputs
func (m *ScaffoldModule) SecretContext() SecretContext {
    // Module validates its own inputs
    if m.input.CustomerID == "" {
        // Module rejects invalid state at construction, not here
        panic("scaffold module constructed without customer ID")
    }
    return SecretContext{TenantID: m.input.CustomerID}
}

// No secrets — module doesn't implement SecretContextProvider
// Runtime skips tenant resolution entirely
```

### E. Secrets API Authorization Model

| Operation | Platform Tenant | Customer Tenant |
|-----------|----------------|-----------------|
| Set secret | Platform admin only | Tenant admin or platform admin |
| List keys | Platform admin | Tenant member or platform admin |
| Delete secret | Platform admin only | Tenant admin or platform admin |
