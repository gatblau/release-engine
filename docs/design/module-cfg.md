# Design Document: Configuration-Driven Module Assembly for Outside-In Integration Testing

## 1. Purpose

This document proposes a configuration-driven module assembly model to replace hardcoded module configuration.

The goal is to:

- enable **true outside-in integration testing**
- preserve **production-faithful workflow execution**
- allow **environment-specific connector selection**
- support **deterministic test environments**
- validate the **real module loading and wiring path** used by the system

---

## 2. Problem Statement

Currently, module configuration is hardcoded in code. This creates several issues:

- module behavior is difficult to vary by environment
- integration tests cannot assemble modules realistically
- test-specific behavior tends to be implemented through code shortcuts
- connector selection is not externally visible or controllable
- assembly-time errors are discovered late or not tested at all

This limits confidence in the final integrated product, especially for systems composed of multiple modules, plugins, and connectors.

---

## 3. Design Goals

The proposed design must:

1. allow each module to be configured externally
2. support selection of connector implementations by connector family
3. allow optional module variables with defaults
4. fail fast if required connectors are not configured
5. support config path override through environment variables
6. load configuration during normal module resolution
7. inject resolved configuration into the module through constructor injection
8. support both test and production assembly without changing module logic

---

## 4. Proposed Approach

Each module will have a YAML configuration file, for example:

- `cfg_infra.yaml`
- `cfg_billing.yaml`
- `cfg_notifications.yaml`

A module configuration contains:

- **vars**: module runtime settings
- **connectors.families**: connector implementation selection by family

Example:

```yaml
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

---

## 5. Configuration Semantics

### 5.1 Vars
`vars` contains module-specific configuration values.

Rules:

- vars may be omitted if the module defines defaults
- missing optional vars are defaulted during config loading
- invalid or incompatible values cause load failure

Examples:

- polling interval
- timeout values
- retry settings
- module feature tuning parameters

### 5.2 Connectors
`connectors.families` defines which implementation to use for each required connector family.

Rules:

- all required connector families must be present
- missing required connector families cause module load failure
- unknown connector implementation names cause module load failure
- connector compatibility is validated at load time

Examples:

- `git -> git-file`
- `policy -> policy-mock`
- `webhook -> webhook-capture`

---

## 6. Configuration Discovery

The framework resolves the module configuration file when a module is loaded.

Configuration path resolution:

1. check environment variable override:
    - `CFG_PATH_INFRA`
    - `CFG_PATH_BILLING`
    - etc.
2. otherwise use the framework default config location

This allows the same module to be assembled differently in:

- local development
- CI integration testing
- acceptance testing
- production

without code changes.

---

## 7. Module Resolution and Injection

When the framework resolves a module, it will:

1. identify the module name
2. locate the config file
3. parse YAML
4. validate schema and module identity
5. apply defaults for optional vars
6. validate required connectors
7. resolve connector implementations from the connector registry
8. build a typed configuration object
9. inject config and connectors into the module constructor

This ensures modules receive validated runtime configuration and resolved dependencies through the normal framework path.

---

## 8. Runtime Model

This design does **not** introduce test-only code paths.

Instead:

- the same module logic runs in all environments
- only configuration and connector selection differ

For example:

- production may use:
    - `git: git-remote`
    - `policy: policy-live`
    - `webhook: webhook-http`
- integration tests may use:
    - `git: git-file`
    - `policy: policy-mock`
    - `webhook: webhook-capture`

This preserves the real workflow while allowing deterministic testing.

---

## 9. Benefits

### 9.1 Better testability
Modules can be tested in different environments by changing config, not code.

### 9.2 Realistic outside-in integration
Full integration tests can validate:

- config loading
- module resolution
- connector wiring
- runtime orchestration
- externally visible outcomes

### 9.3 Fail-fast assembly validation
Misconfiguration is detected when the module is loaded, not during late execution.

### 9.4 Cleaner separation of concerns
- module logic handles workflow behavior
- configuration defines environment-specific assembly
- framework handles loading, validation, and injection

---

## 10. Failure Behavior

### Module load should fail if:
- config file is missing
- config file is malformed
- schema version is invalid
- module name does not match
- required connector family is missing
- selected connector implementation is unknown
- a config value is invalid

### Module load should succeed if:
- optional vars are missing and defaults exist

### Request execution should fail at runtime if:
- request-specific inputs are missing
- external systems fail during execution
- downstream dependencies are unavailable

---

## 11. Testing Impact

This design enables three important classes of tests:

### 11.1 Config loader tests
Validate parsing, defaulting, and validation rules.

### 11.2 Module assembly tests
Validate env var override, connector resolution, and constructor injection.

### 11.3 Outside-in workflow tests
Start the real engine with test config, call public APIs, and verify public outcomes.

This gives confidence not only in module logic, but also in the real assembly behavior of the system.

---

## 12. Non-Goals

This design does not aim to:

- move request-specific business inputs into static config
- create hidden test modes
- bypass production phases for testing
- replace connector contract tests or unit tests

It is intended to improve assembly, configurability, and integration realism.

---

## 13. Implementation Summary

Initial implementation should include:

1. per-module YAML config files
2. env-var-based config path override
3. schema validation and default application
4. connector family resolution from registry
5. constructor injection of typed config and resolved connectors
6. integration tests using real framework-driven config loading

---

## 14. Summary

This design externalizes module behavior and connector selection into per-module configuration files loaded by the framework during normal module resolution.

It enables:

- realistic environment-specific module assembly
- deterministic integration testing
- validation of the actual production loading path
- strong support for black-box, outside-in quality assurance

The key principle is:

> **preserve the real workflow, vary the assembly through configuration.**