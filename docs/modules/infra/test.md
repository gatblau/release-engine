# Infra Module Testing Guide

## Overview

This document describes how to run tests for the infra module, understand the test architecture, and work with golden files for regression testing.

## Table of Contents

1. [Test Types and Purposes](#test-types-and-purposes)
2. [Running Tests](#running-tests)
3. [Integration Test Architecture](#integration-test-architecture)
4. [Golden File Testing](#golden-file-testing)
5. [Test Data Organization](#test-data-organization)
6. [Crossplane Validation](#crossplane-validation)
7. [Debugging and Maintenance](#debugging-and-maintenance)

## Test Types and Purposes

### Unit Tests
- **Location**: `internal/module/infra/*_test.go`
- **Purpose**: Test individual functions, validation logic, and template rendering
- **Run with**: `go test ./internal/module/infra/...`

### Integration Tests
- **Location**: `internal/integration/*_test.go`
- **Purpose**: Test full job lifecycle, database interactions, and container dependencies
- **Requires**: Docker (for test containers)
- **Tag**: `//go:build integration`
- **Run with**: `make test-infra-integration` or `go test -tags=integration ./internal/integration/...`

### Smoke Tests
- **Location**: `internal/smoke/*_test.go`
- **Purpose**: Verify container setup and basic system functionality
- **Run with**: `make test-smoke`

## Running Tests

### Makefile Commands

The project includes several convenient Makefile targets:

```bash
# Run all tests (excluding smoke tests)
make test

# Run only smoke tests (container verification)
make test-smoke

# Run only infra integration tests
make test-infra-integration

# Run tests with race detection
make test-race

# Run security checks
make security

# Run linter
make lint
```

### Direct Go Test Commands

```bash
# Run all tests with coverage
go test -tags=integration -coverprofile=coverage.out ./...

# Run specific integration test
go test -tags=integration -v ./internal/integration -run TestInfraIntegration_FullLifecycle

# Run tests without containers (skip integration tests)
go test ./...
```

### Prerequisites

1. **Docker**: Required for integration tests (Postgres, MinIO containers)
2. **Go 1.21+**: Required for module support
3. **Environment**: Tests use temporary directories and containerized dependencies

## Integration Test Architecture

### Test Harness

The `IntegrationTestHarness` (`internal/integration/harness.go`) provides a complete test environment:

```go
harness := integration.NewIntegrationTestHarness(t)
defer harness.Cleanup()

// Register module and connectors
harness.RegisterModule(infra.NewModule())
harness.RegisterConnector(fileGitConnector)

// Create test job
jobID := harness.CreateJob(integration.JobOptions{
    TenantID:       "test-tenant",
    PathKey:        infra.ModuleKey,
    IdempotencyKey: "test-job-1",
    Params:         testParams,
})

// Run scheduler and validate results
require.NoError(t, harness.RunSchedulerCycle())
require.NoError(t, harness.WaitForJobState(jobID, "succeeded", 5*time.Second))
```

### Key Components

1. **Test Containers**:
   - PostgreSQL for job state persistence
   - MinIO for S3-compatible storage (Volta secrets)

2. **Registries**:
   - Module registry for infra module registration
   - Connector registry for test connectors (FileGitConnector)

3. **Services**:
   - Scheduler service for job processing
   - Runner service for step execution

### FileGitConnector

A testing connector that writes to filesystem instead of real Git:
- Implements full git connector interface
- Writes files to test directory for validation
- Supports deterministic testing without external dependencies

## Golden File Testing

### What are Golden Files?

Golden files store the **expected output** of operations. During tests, actual output is compared against these "golden" files to detect regressions.

### Location

```
internal/integration/testdata/golden/
├── example.yaml          # Simple test case
└── (more golden files for integration tests)
```

### How They Work

The `AssertGoldenYAML` function in `validator.go`:

1. **Compares output**: During normal test runs, compares actual YAML against golden file
2. **Updates on demand**: Use `-update-golden` flag to refresh golden files when outputs intentionally change
3. **Provides clear diffs**: Shows exact differences when outputs don't match

### Using Golden Files

```go
func TestExample(t *testing.T) {
    // Generate some YAML output
    actualYAML := generateYAML()
    
    // Compare against golden file
    integration.AssertGoldenYAML(t, "test-case-name", []byte(actualYAML))
}
```

### Updating Golden Files

When you've intentionally changed output format:

```bash
# Run tests with update flag
go test -tags=integration -v ./internal/integration -update-golden

# Or use Makefile with flag
go test -tags=integration -v ./internal/integration -run TestInfraIntegration -update-golden
```

**Important**: Only update golden files when you intentionally change output behavior. CI/CD should fail if golden files differ without this flag.

### Benefits

1. **Regression protection**: Catches unintended output changes
2. **Clear expectations**: Golden files document expected output format
3. **Easy review**: Diffs show exactly what changed
4. **Deterministic testing**: Same inputs produce same verified outputs

## Test Data Organization

### Input Payloads

```
testdata/payloads/
├── infra-k8s-app.yaml      # Kubernetes application template
├── infra-vm-app.yaml       # VM application template  
└── infra-data-proc.yaml    # Data processing template
```

These YAML files define test scenarios with different parameter combinations.

### Output Directory

```
testdata/output/
├── infra-k8s-app.manifest.yaml     # Generated manifests for visual review
├── infra-vm-app.manifest.yaml
└── infra-data-proc.manifest.yaml
```

**Note**: Output files are for visual review during development. Golden files are the authoritative expected outputs.

## Crossplane Validation

### CrossplaneValidator

The `CrossplaneValidator` (`internal/integration/validator.go`) validates generated Crossplane manifests:

```go
validator := integration.NewCrossplaneValidator()
err := validator.ValidateYAML([]byte(manifest))
```

### Validation Rules

1. **Field Presence**: Ensures required fields (`apiVersion`, `kind`, `metadata`, `spec`)
2. **Metadata Structure**: Validates labels, annotations, naming
3. **Spec Structure**: Ensures spec is non-empty object
4. **XRD Contracts**: Validates against known Crossplane XRD parameter schemas

### Supported XRD Kinds

The validator knows about these Crossplane XRD kinds:
- `XCache`, `XDatabase`, `XDNSZone`, `XKubernetesCluster`
- `XLoadBalancer`, `XMessaging`, `XObjectStore`, `XObservability`
- `XSecretsStore`, `XVPCNetwork`, `XVirtualMachine`

Each kind has:
- **Required parameters**: Must be present in `spec.parameters`
- **Allowed parameters**: Only documented parameters are allowed

## Debugging and Maintenance

### Common Issues

1. **Test Container Failures**:
   ```bash
   # Ensure Docker is running
   docker ps
   
   # Clean up old containers
   docker system prune -f
   ```

2. **Golden File Mismatches**:
   - Review the diff output in test failure
   - Determine if change is intentional
   - Update with `-update-golden` if intentional

3. **Database Schema Issues**:
   - Tests automatically apply migrations
   - Check `internal/db/schema.go` for current schema

### Adding New Test Scenarios

1. **Create input payload** in `testdata/payloads/`
2. **Add integration test** in `infra_integration_test.go`
3. **Generate golden file** with `-update-golden` flag
4. **Verify** output matches expectations

### Best Practices

1. **Keep tests deterministic**: Use fixed inputs, mock time dependencies
2. **Clean up resources**: Tests use temporary directories cleaned automatically
3. **Validate outputs**: Use both programmatic validation and golden file comparison
4. **Document changes**: When updating golden files, document why in commit messages

## Related Documentation

- [API Contract](api.md): Infra module input/output specifications
- [Design](design.md): Module architecture and design decisions
- [Test Framework Plan](../test.md): Overall testing strategy and framework design