## Test Framework Plan 

### 1. **Decoupled Test Framework Architecture**

Looking at `smoke_test.go`, I see excellent patterns we can follow:
- **Testcontainers** for database and storage dependencies
- **Full lifecycle testing** with job scheduling, execution, and state transitions
- **Modular setup** with reusable test helpers

We'll create a framework that:
1. **Is reusable** across different modules (infra, scaffold, etc.)
2. **Tests full job lifecycle** from API intake → scheduler → runner → connector
3. **Validates Crossplane definitions** with field-level validation
4. **Uses FileGitConnector** for deterministic testing

### 2. **Framework Components**

#### **A. FileGitConnector Implementation**
- Implements the `git` connector interface but writes to filesystem
- Supports operations needed for infra module testing:
    - `push_files` → writes YAML files to test directory
    - `create_repository` → creates directory structure
    - `get_file` → reads from test directory
    - `create_or_update_file` → writes/updates files
- Maintains idempotency semantics
- Configurable output directory (temp dir for tests)

#### **B. Integration Test Harness**
Inspired by `smoke_test.go` patterns:
```go
type IntegrationTestHarness struct {
    DBContainer    *postgres.PostgresContainer
    MinioContainer *minio.MinioContainer
    TempDir        string
    ConnectorReg   *connector.Registry
    ModuleReg      *registry.ModuleRegistry
    Runner         *runner.RunnerService
    Scheduler      *scheduler.SchedulerService
    // ... other components
}

func NewIntegrationTestHarness(t *testing.T) *IntegrationTestHarness {
    // Setup testcontainers, temp dirs, registries
    // Register FileGitConnector and infra module
}
```

#### **C. Crossplane Validator**
- Parses generated YAML/JSON
- Validates:
    - Required fields (`apiVersion`, `kind`, `metadata`, `spec`)
    - Field relationships and constraints
    - Policy compliance (encryption, tags, etc.)
    - Golden file comparison for regression testing

#### **D. Test Scenarios**
Based on your requirements:
1. **Minimal Web App**: Basic Kubernetes + Database
2. **Production Web App**: Full stack with CDN, LB, observability
3. **VM Workload**: Legacy lift-and-shift scenario
4. **Validation Failures**: Invalid parameters should fail gracefully

### 3. **Implementation Steps**

#### **Phase 1: FileGitConnector**
1. Create `internal/connector/testing/filegit.go`
2. Implement core operations (`push_files`, `create_repository`, etc.)
3. Add contract tests using existing `ConnectorContractTestSuite`
4. Create unit tests for file operations

#### **Phase 2: Test Harness Infrastructure**
1. Create `internal/integration/harness.go` with reusable setup/teardown
2. Port patterns from `smoke_test.go`:
    - Database container setup
    - Volta/S3 container setup (if needed)
    - Registry initialization
3. Add helper methods for:
    - Creating test jobs
    - Running scheduler cycles
    - Validating job state transitions

#### **Phase 3: Crossplane Validator**
1. Create `internal/integration/validator.go`
2. Implement YAML parsing and validation logic
3. Add golden file comparison with `-update-golden` flag
4. Create validation rules based on infra module design

#### **Phase 4: Integration Tests**
1. Create `internal/integration/infra_integration_test.go`
2. Test scenarios:
    - Full job lifecycle with infra module
    - FileGitConnector file output validation
    - Crossplane definition validation
    - Idempotency on retry
3. Add to CI/CD pipeline

### 4. **Key Design Decisions**

#### **Decoupling Strategy**
- **Harness** is module-agnostic: accepts any module + connector combo
- **FileGitConnector** is connector-agnostic: can be used with any module
- **Validator** is output-agnostic: validates any YAML/JSON structure

#### **Full Lifecycle Testing**
We'll test:
```
API Intake → Job Creation → Scheduler Claim → Runner Execution → 
Module Processing → Connector Call → File Output → Validation
```

#### **Crossplane Validation Depth**
We'll validate:
- **Structure**: Required fields, proper nesting
- **Values**: Field-specific validation (e.g., `encrypted: true` for storage)
- **Relationships**: Cross-field dependencies
- **Policy**: Compliance with security policies

### 5. **Example Test Structure**

```go
func TestInfraModule_EndToEnd(t *testing.T) {
    harness := integration.NewTestHarness(t)
    defer harness.Cleanup()
    
    // Register infra module and filegit connector
    harness.RegisterModule(infra.NewModule())
    harness.RegisterConnector("git", &filegit.FileGitConnector{
        OutputDir: harness.TempDir,
    })
    
    // Create test job
    jobID := harness.CreateJob(map[string]any{
        "contract_version": "v1",
        "request_name": "test-app",
        "tenant": "test-tenant",
        // ... full parameters
    })
    
    // Run scheduler to process job
    harness.RunSchedulerCycle()
    
    // Verify job completed successfully
    state := harness.GetJobState(jobID)
    require.Equal(t, "succeeded", state)
    
    // Validate Crossplane definitions were created
    files, err := os.ReadDir(filepath.Join(harness.TempDir, "test-tenant", "test-app"))
    require.NoError(t, err)
    require.NotEmpty(t, files)
    
    // Parse and validate YAML
    for _, file := range files {
        if strings.HasSuffix(file.Name(), ".yaml") {
            data, err := os.ReadFile(filepath.Join(harness.TempDir, "test-tenant", "test-app", file.Name()))
            require.NoError(t, err)
            
            validator := integration.NewCrossplaneValidator()
            err = validator.Validate(data)
            require.NoError(t, err)
            
            // Golden file comparison
            integration.AssertGoldenYAML(t, "webapp-minimal", data)
        }
    }
}
```

### 6. **Questions Before Implementation**

1. **Connector Registration**: Should `FileGitConnector` be in `internal/connector/testing/` or `internal/integration/`?

2. **Test Data**: Should we use the existing test fixtures from `internal/module/infra/template/testdata/` or create new ones?

3. **Validation Scope**: Should we validate against actual Crossplane schemas (using Kubernetes client-go) or implement our own validation logic?

4. **CI/CD Integration**: Should these be `integration` tagged tests (like smoke tests) or regular unit tests?

Once you approve this plan, I'll start implementation. The framework will be reusable for testing other modules (scaffold, etc.) by simply registering different modules and connectors.