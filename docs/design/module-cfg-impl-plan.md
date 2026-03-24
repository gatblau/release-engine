# Module Configuration Implementation Plan v2

## Overview
This document outlines the revised implementation plan for the configuration-driven module assembly system described in `module-cfg.md`.

This version incorporates the following architectural decisions:

- **configuration is loaded during normal module resolution**
- **module assembly is explicit and constructor-driven**
- **raw YAML config is distinct from module-specific typed config**
- **module-specific defaults and validation are owned by the module**
- **reflection-based DI and post-construction `SetConfig` mutation are out of scope**
- **config-managed modules fail fast on missing or invalid configuration**
- **assembly tests are distinct from true outside-in workflow tests**

The plan remains incremental, with each phase including clear deliverables, compilation checks, and test validation.

---

## Phase 1: Foundation - Raw Configuration Schema and Loading

### Objectives
1. Define a framework-level raw YAML schema for module configuration
2. Create a configuration loader with environment variable overrides
3. Add framework-level schema validation
4. Unit test raw config loading in isolation

### Deliverables
- [x] **File**: `internal/module/config/schema.go` - Raw config struct definitions
- [x] **File**: `internal/module/config/loader.go` - Config loader with env var resolution
- [x] **File**: `internal/module/config/errors.go` - Error types for config loading/validation
- [x] **File**: `cfg_infra.yaml` - Example raw config for infra module
- [x] **Tests**: `internal/module/config/loader_test.go` - Unit tests for raw config loading

### Implementation Details

#### Raw Schema Definition
This schema is the **framework-level parse representation**, not the final runtime config injected into a module.

```go
// internal/module/config/schema.go
package config

type ModuleConfigFile struct {
    APIVersion string           `yaml:"apiVersion"`
    Module     string           `yaml:"module"`
    Vars       map[string]any   `yaml:"vars,omitempty"`
    Connectors ConnectorsConfig `yaml:"connectors"`
}

type ConnectorsConfig struct {
    Families map[string]string `yaml:"families"`
}
```

### Loader Interface
```go
// internal/module/config/loader.go
type Loader interface {
    Load(ctx context.Context, moduleName string) (*ModuleConfigFile, error)
}

type loader struct {
    defaultBasePath string
}

func NewLoader(defaultBasePath string) Loader {
    return &loader{defaultBasePath: defaultBasePath}
}
```

### Path Resolution Rules
Config path resolution order:

1. `CFG_PATH_<MODULE_UPPER>`
2. `${CFG_ROOT}/cfg_<module>.yaml` if `CFG_ROOT` is set
3. `<defaultBasePath>/cfg_<module>.yaml`

Examples:

- `CFG_PATH_INFRA=/custom/path/cfg_infra.yaml`
- `CFG_ROOT=/workspace/test-configs`

### Framework-Level Validation
The raw loader validates only framework concerns:

- file exists
- YAML parses successfully
- unknown top-level fields are rejected where feasible
- `apiVersion` is present and supported
- `module` is present
- `module` matches the requested module name
- `connectors.families` is present for config-managed modules

It does **not** validate module-specific vars beyond basic structure.

### Example Config
```yaml
# cfg_infra.yaml
apiVersion: module.config/v1
module: infra

vars:
  health_timeout: 10s
  poll_interval: 200ms

connectors:
  families:
    git: git-file
    policy: policy-mock
    webhook: webhook-mock
```

### Compilation Validation
```bash
go build ./internal/module/config/...
go test ./internal/module/config/...
```

### Test Validation
1. **Unit Tests**:
    - load config from default path
    - load config via `CFG_PATH_INFRA`
    - load config via `CFG_ROOT`
    - validate schema parsing
    - fail on missing required fields
    - fail on malformed YAML
    - fail on unsupported API version
    - fail on requested module / file module mismatch
    - fail on missing connector family mapping block
    - reject unknown top-level fields if strict YAML decoding is enabled

2. **Example Config Validation**:
    - infra example config parses successfully
    - env-var path precedence is correct

---

## Phase 2: Module-Owned Typed Config Parsing

### Objectives
1. Introduce module-specific typed config parsing
2. Move defaults and var validation into each module
3. Define required connector families per module
4. Unit test typed parsing independently from module resolution

### Deliverables
- [x] **File**: `internal/module/infra/config.go` - Infra typed config, defaults, validation
- [x] **File**: `internal/module/infra/config_test.go` - Infra config parser tests
- [x] **Optional Pattern Doc**: `internal/module/README.md` or authoring guide update
- [x] **Tests**: typed parsing tests for infra

### Implementation Details

#### Typed Module Config
Each module owns its typed config model.

```go
// internal/module/infra/config.go
package infra

type Vars struct {
    HealthTimeout time.Duration
    PollInterval  time.Duration
}

type ConnectorSelection struct {
    Git     string
    Policy  string
    Webhook string
}

type Config struct {
    Vars       Vars
    Connectors ConnectorSelection
}
```

#### Typed Parsing Contract
Each config-managed module provides logic to:

- convert `config.ModuleConfigFile` into typed config
- apply defaults for omitted vars
- validate var types and ranges
- declare required connector families

Example:

```go
func ParseConfig(raw *config.ModuleConfigFile) (*Config, error)
func RequiredConnectorFamilies() []string
```

#### Validation Ownership
**Framework validation** remains responsible for:
- file loading
- API version
- module identity
- top-level schema integrity

**Module validation** is responsible for:
- var parsing
- defaulting
- semantic validation
- required connector family declarations

#### Defaults Behavior
- missing optional vars are defaulted by the module parser
- missing required connector family mappings are errors
- invalid duration/string formats are errors
- unknown connector family names in `vars` are ignored because they are outside module vars scope

### Compilation Validation
```bash
go build ./internal/module/infra/...
go test ./internal/module/infra/... -run TestParseConfig
```

### Test Validation
1. **Typed Config Tests**:
    - valid raw config parses into typed infra config
    - default values applied when vars omitted
    - missing required connector family fails
    - invalid duration values fail
    - unknown connector implementation names are not checked here if registry validation happens later
    - semantic validation errors are actionable

2. **Module Ownership Tests**:
    - infra required connector families are explicitly declared
    - infra parser does not rely on framework switch statements

---

## Phase 3: Explicit Connector Resolution and Constructor Assembly

### Objectives
1. Add family-based connector resolution in the connector registry
2. Resolve connector implementations from typed module config
3. Assemble modules using explicit constructor injection
4. Keep reflection-based field injection out of scope

### Deliverables
- [x] **File**: `internal/connector/registry.go` - Family-aware connector registry updates
- [x] **File**: `internal/module/infra/module.go` - Explicit constructor-based assembly
- [x] **Tests**: `internal/connector/registry_test.go` - Family lookup and compatibility tests
- [x] **Tests**: `internal/module/infra/module_test.go` - Constructor assembly tests

### Implementation Details

#### Registry Model
The registry must support:

- lookup by implementation name
- validation of connector family membership
- construction of typed connector dependencies

Example conceptual model:

```go
type Descriptor struct {
    Name   string
    Family string
}

type Registry interface {
    ResolveGit(name string) (GitConnector, error)
    ResolvePolicy(name string) (PolicyConnector, error)
    ResolveWebhook(name string) (WebhookConnector, error)
}
```

This is preferred over a single untyped `connector.Connector` where possible.

#### Constructor Injection
Modules are assembled only through explicit constructors.

```go
func NewModule(
    vars Vars,
    git GitConnector,
    policy PolicyConnector,
    webhook WebhookConnector,
) (*Module, error)
```

Out of scope:

- reflection-based field injection
- `SetConfig`
- partially initialized module instances

#### Connector Resolution Flow
1. module typed config declares selected implementations
2. registry resolves each implementation by family
3. framework passes typed connectors into constructor
4. constructor validates invariant completeness

### Compilation Validation
```bash
go build ./internal/connector/...
go build ./internal/module/infra/...
go test ./internal/connector/...
go test ./internal/module/infra/...
```

### Test Validation
1. **Registry Tests**:
    - valid implementation resolves for expected family
    - unknown implementation fails
    - implementation in wrong family fails
    - registry returns typed connector interface

2. **Module Constructor Tests**:
    - constructor succeeds with valid typed config and connectors
    - constructor fails on nil/missing dependencies
    - constructor stores config and dependencies immutably

---

## Phase 4: Resolver Integration - Framework-Driven Module Assembly

### Objectives
1. Integrate raw config loading into module resolution
2. Invoke module-owned typed parsing during resolution
3. Resolve typed connectors from registry
4. Assemble config-managed modules during normal framework bootstrap
5. Fail fast for migrated modules with missing or invalid configuration

### Deliverables
- [x] **File**: `internal/module/factory.go` - Config-aware assembly entry points
- [x] **File**: `internal/runner/module_resolver.go` - Resolver integration
- [x] **Modify**: `internal/runner/module_bootstrap.go` - Bootstrap loads and assembles modules
- [x] **Tests**: `internal/runner/module_resolver_test.go` - Resolver integration tests
- [x] **Tests**: `internal/runner/module_bootstrap_test.go` - Bootstrap behavior tests

### Implementation Details

#### Resolution Flow
For a config-managed module:

1. look up module registration
2. load raw config file
3. validate framework-level schema
4. invoke module-owned typed parser
5. validate required connector families are present
6. resolve selected connector implementations from registry
7. call module constructor with typed config and typed connectors
8. return assembled module or fail

#### Migration Strategy
During transition, the framework may support two categories:

1. **legacy modules**
    - continue using old assembly path temporarily

2. **config-managed modules**
    - must load valid config
    - must assemble through the new path
    - missing config is a hard failure

This avoids silent fallback for modules already migrated.

#### Fail-Fast Rules
For config-managed modules, module load must fail if:

- config file is missing
- config is malformed
- unsupported `apiVersion`
- module mismatch
- required connector family mapping missing
- selected connector implementation unknown
- selected connector implementation belongs to wrong family
- typed var parsing fails
- constructor dependency validation fails

### Compilation Validation
```bash
go build ./internal/runner/...
go test ./internal/runner/...
```

### Test Validation
1. **Resolver Tests**:
    - resolver loads raw config and assembles infra module
    - env var path override is honored
    - `CFG_ROOT` override is honored
    - typed parser is invoked
    - registry resolution occurs by family
    - constructor is called with expected dependencies

2. **Migration Tests**:
    - legacy module still uses legacy path
    - config-managed module does not silently fall back
    - missing config for migrated module fails loudly

3. **Assembly Boundary Tests**:
    - assembly tests verify config loading/wiring behavior
    - these tests are distinct from end-to-end workflow tests

---

## Phase 5: Validation, Outside-In Testing, and Migration Rollout

### Objectives
1. Add environment-specific config sets for real assembly scenarios
2. separate assembly integration tests from outside-in workflow tests
3. migrate modules incrementally to config-managed assembly
4. update docs and examples
5. benchmark and verify no meaningful performance regression

### Deliverables
- [x] **Config Files**: Environment-specific configs for dev, test, staging, prod
- [x] **Migration**: Updated constructors and assembly paths for migrated modules
- [x] **Documentation**: Updated module authoring and config guide (see updated plan and examples)
- [x] **Performance Tests**: Benchmark config loading and module assembly
- [x] **Integration Tests**: Assembly tests and outside-in workflow tests

### Implementation Details

#### Environment-Specific Config Layout
```text
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

#### Test Layers

### 5.1 Assembly Integration Tests
These validate the real framework loading/wiring path.

Assertions may include:

- correct config file selected
- typed parsing occurred
- correct connector implementations selected
- constructor received expected dependencies
- module assembled successfully

These tests may use test doubles or capture connectors because their purpose is assembly validation.

### 5.2 Outside-In Workflow Tests
These validate end-to-end behavior through public interfaces.

Assertions should primarily use **externally visible outcomes**, such as:

- API response
- job state transitions
- persisted outputs
- file system state in test repositories
- callback/webhook receipt through a capture service
- observable policy outcomes

These tests should avoid depending primarily on module internals.

#### Module Migration Checklist
1. add typed config parser to module
2. define required connector families
3. add constructor accepting typed config and typed dependencies
4. remove hardcoded connector references
5. move defaults into module-owned parser
6. update tests to provide config and/or config files
7. mark module as config-managed in framework registration

#### Removal Criteria for Legacy Path
Legacy assembly can be removed when:

- all target modules are migrated
- config-managed assembly is used in CI
- outside-in workflow tests cover critical paths
- no production environment depends on legacy wiring

### Compilation Validation
```bash
go build ./...

go test ./... -v

go test ./internal/integration/... -v
```

### Test Validation
1. **Assembly Integration Tests**:
    - real framework assembly works with environment-specific config
    - all migrated modules load via config path
    - connector selection changes behavior by environment as intended

2. **Outside-In Workflow Tests**:
    - production-like execution path works in CI
    - public outcomes are correct
    - no dependence on hidden test-mode branches

3. **Performance Tests**:
    - config loading overhead is acceptable
    - module assembly occurs only at startup or resolution time as designed
    - no material regression in job execution path

4. **Migration Tests**:
    - all existing functionality preserved
    - migrated modules no longer use hardcoded connector selection
    - legacy path only remains for unmigrated modules

---

## Risk Mitigation

### Technical Risks
1. **Breaking changes during constructor migration**
    - mitigated by incremental module-by-module rollout

2. **Architecture drift toward dynamic/untyped runtime config**
    - mitigated by explicit raw-vs-typed config separation

3. **Hidden fallback weakening fail-fast assembly**
    - mitigated by hard-fail rules for config-managed modules

4. **DI complexity**
    - mitigated by explicit constructor injection and typed connector interfaces

### Migration Strategy
1. **Phase 1-2**: establish raw loading and typed module parsing
2. **Phase 3-4**: enable config-managed assembly for selected modules
3. **Phase 5**: migrate remaining modules incrementally
4. remove legacy path only after critical modules and CI paths are migrated

### Rollback Plan
1. keep legacy assembly for unmigrated modules during transition
2. migrate modules individually rather than all at once
3. revert module registration from config-managed to legacy if needed during rollout
4. preserve test coverage at both assembly and outside-in levels

---

## Success Criteria

### Phase Completion Criteria
1. all tests pass for the phase
2. code compiles cleanly
3. docs are updated where interfaces or authoring expectations change
4. migrated modules use explicit constructor assembly
5. fail-fast behavior is verified for config-managed modules

### Overall Success Criteria
1. all migrated modules configurable via YAML
2. environment-specific assembly works through normal framework resolution
3. typed module config parsing is module-owned
4. outside-in integration tests pass without hidden test branches
5. no material performance regression
6. developer workflow is clearer than the prior hardcoded assembly model

---

## Timeline Estimates

- **Phase 1**: 3-5 days  
  raw schema, loader, env resolution, unit tests

- **Phase 2**: 3-5 days  
  typed config parsing for infra, defaults, validation tests

- **Phase 3**: 4-6 days  
  connector family resolution, explicit constructor assembly

- **Phase 4**: 4-6 days  
  resolver/bootstrap integration, fail-fast behavior, migration boundary

- **Phase 5**: 7-12 days  
  migration rollout, environment configs, assembly tests, outside-in tests, docs

**Total**: 21-34 development days

Note: total effort may increase depending on the number of existing modules and the extent of constructor/signature changes required.

---

## Dependencies

1. **Go 1.21+**
    - suitable for current implementation patterns and testing support

2. **YAML v3**
    - used for config parsing

3. **Connector Registry**
    - must support family-aware typed resolution

4. **Testing Framework**
    - existing test framework should support unit, integration, and outside-in test layers

---

## Operational and Design Constraints

1. **No reflection-based field injection**
2. **No `SetConfig` post-construction mutation**
3. **No silent fallback for config-managed modules**
4. **No hot reload in the initial implementation**
5. **Config is loaded at startup or module assembly time**
6. **`map[string]any` is allowed only as raw parse representation, not injected runtime config**


