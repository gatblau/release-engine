# Module Configuration and Assembly

This directory contains the implementation of the configuration-driven module assembly system for the release engine.

## Overview

The module configuration system allows modules to be assembled at runtime using YAML configuration files. This replaces hardcoded connector selection with explicit configuration while retaining explicit constructor dependency injection, enabling environment-specific assembly and easier testing.

## Key Components

### 1. Raw Configuration (`internal/module/config/`)
- **schema.go**: Framework-level raw YAML schema definitions
- **loader.go**: Config loader with environment variable overrides
- **errors.go**: Error types for config loading/validation
- **loader_test.go**: Unit tests for raw config loading

### 2. Module-Owned Typed Configuration (`internal/module/<module>/config.go`)
Each module is responsible for parsing and validating its own typed configuration from the raw framework schema:

```go
package infra

import "github.com/gatblau/release-engine/internal/module/config"

// Config represents the typed configuration for the infra module.
type Config struct {
    Vars       Vars
    Connectors ConnectorSelection
}

// ConnectorSelection specifies which connector implementations to use.
// Each module defines its own required families.
type ConnectorSelection struct {
    Git     string
    Policy  string
    Webhook string
}
```

Each module implements:
- `ParseConfig(raw *config.ModuleConfigFile) (*Config, error)` - converts raw config to typed config
- `RequiredConnectorFamilies() []string` - declares required connector families

### 3. Explicit Constructor Assembly (`internal/module/<module>/module.go`)
Modules are assembled through explicit constructors:

```go
func NewModule(
    vars Vars,
    git connector.GitConnector,
    policy connector.PolicyConnector,
    webhook connector.WebhookConnector,
) (*Module, error)
```

### 4. Factory and Resolver Integration
- **factory.go**: Config-aware assembly entry points
- **module_resolver.go**: Framework-driven module resolution
- **module_bootstrap.go**: Bootstrap integration with config loading

## Configuration File Format

### Basic Structure
```yaml
apiVersion: module.config/v1
module: <module-name>

vars:
  # Module-specific variables
  health_timeout: 30s
  poll_interval: 500ms

connectors:
  families:
    git: git-file
    policy: policy-mock
    webhook: webhook-mock
```

### Environment-Specific Configuration

The system supports environment-specific configurations through the `CFG_ROOT` environment variable:

```
config/
├── dev/
│   ├── cfg_infra.yaml
│   └── cfg_billing.yaml
├── test/
│   ├── cfg_infra.yaml
│   └── cfg_billing.yaml
├── staging/
│   ├── cfg_infra.yaml
│   └── cfg_billing.yaml
└── prod/
    ├── cfg_infra.yaml
    └── cfg_billing.yaml
```

## Path Resolution Order

Configuration files are resolved in the following order:

1. `CFG_PATH_<MODULE_UPPER>`
   Direct file path override for a specific module.
   Example: `CFG_PATH_INFRA=/custom/path/cfg_infra.yaml`

2. `CFG_ROOT`
   Root directory for the active environment's configuration files.
   The loader resolves `${CFG_ROOT}/cfg_<module>.yaml`.
   Example: `CFG_ROOT=/workspace/config/dev`

3. Default base path
   If neither environment variable is set, the loader resolves
   `<basePath>/cfg_<module>.yaml`.

`basePath` defaults to the process working directory unless explicitly set by the caller.

## Migration Checklist for Module Authors

To migrate a module to config-managed assembly:

1. **Add typed config parser**
   - Create `config.go` in your module directory
   - Implement `ParseConfig()` and `RequiredConnectorFamilies()`
   - Define your module's typed configuration struct

2. **Update module constructor**
   - Add explicit constructor accepting typed config and typed connectors
   - Example: `NewModule(vars Vars, git GitConnector, policy PolicyConnector, webhook WebhookConnector)`
   - Keep legacy constructor for backward compatibility

3. **Update tests**
   - Add tests for config parsing
   - Update assembly tests to use config-managed path
   - Ensure both legacy and config-managed paths are tested

4. **Create configuration files**
   - Add `cfg_<module>.yaml` to appropriate environment directories
   - Define module variables and connector selections

5. **Mark module as config-managed**
   - Update `IsConfigManagedModule()` in `internal/module/factory.go`

## Testing

### Unit Tests
- Test config parsing in isolation
- Test constructor assembly with mock connectors
- Test validation and error cases

### Assembly Integration Tests
- Test real framework loading/wiring path
- Test environment-specific config loading
- Test fail-fast behavior for missing/invalid config

### Outside-In Workflow Tests
- Test end-to-end behavior through public interfaces
- Validate externally visible outcomes
- Avoid depending on module internals

## Performance

The system has been benchmarked for performance:

- Normal config loading: ~14μs per operation
- Config with env var overrides: ~14μs per operation  
- Large config parsing (100+ vars): ~75μs per operation
- Missing config detection: ~1.7μs per operation

## Fail-Fast Behavior

Config-managed modules fail fast when:

- Configuration file is missing
- Configuration is malformed or has unsupported API version
- Module name mismatch
- Required connector **families** are missing from the configuration
- Selected connector **implementations** are unknown or unavailable
- Typed variable parsing fails
- Constructor dependency validation fails

**Important**: If a module is marked as config-managed, failure to load its configuration is fatal. The framework does not auto-fallback from the config-managed path to the legacy constructor path. Legacy constructors exist for backward compatibility and testing, but are not used as a fallback for config-managed modules.

## Best Practices

1. **Keep typed config minimal**: Only include variables the module actually uses
2. **Provide sensible defaults**: Missing optional vars should be defaulted
3. **Validate early**: Fail fast with clear error messages
4. **Maintain backward compatibility**: Keep legacy constructors during migration
5. **Use environment-specific configs**: Different connectors for dev/test/staging/prod

## Example: Infra Module

See `internal/module/infra/` for a complete example:

- `config.go`: Typed config parsing for infra module
- `module.go`: Explicit constructor with injected connectors
- `config_test.go`: Config parsing tests
- `module_test.go`: Constructor assembly tests

## API Versioning

The `apiVersion` field uses a major-versioned API identifier:

- `module.config/v1`: initial version

Backward-compatible additions should remain within the same API version where possible.
Breaking changes should be introduced through a new major version, such as `module.config/v2`.

## Operational Considerations

- Configuration is loaded at module assembly time (startup)
- No hot reload in initial implementation
- Reflection-based field injection is intentionally not supported
- No `SetConfig` post-construction mutation
- No silent fallback for config-managed modules