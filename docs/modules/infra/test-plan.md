## Concrete architecture/test plan for building true outside-in integration capability

Below is a practical plan aimed at one specific outcome:

> **Prove that the product, as assembled from orchestrator + plugins + connectors + external systems, behaves correctly through its public interfaces and produces the expected final outcomes.**

This is not just “make tests pass.”  
It is about creating a **systematic validation capability**.

---

# 1. Target outcome

We want a test capability that can answer, with evidence:

- Does a user request travel through the real orchestration path?
- Are the right plugins/connectors invoked in the right order?
- Do retries, async waiting, callbacks, and idempotency work correctly?
- Does the final external state match the design intent?
- Can we detect regression when any connector/plugin changes?

---

# 2. Core principles

## Principle A — Test from the outside
Tests should interact through **public interfaces only**, such as:

- API endpoint
- job submission endpoint
- CLI command
- webhook entrypoint
- public status/read model

Tests should **not**:
- call internal methods directly
- set hidden phase flags
- inspect private in-memory state as primary evidence

---

## Principle B — Same workflow, different environment
Do **not** skip production phases for tests.

Instead:
- run the same workflow
- against **controlled test dependencies**
- with **faster timing parameters**
- and **deterministic downstream behavior**

Good:
- same orchestration, fast polling, ephemeral repo, local webhook receiver

Bad:
- `test_mode = true` causing git/health/callback to be skipped

---

## Principle C — Observe externally visible behavior
Assertions should be based on:

- job state transitions
- committed artifacts
- created resources
- callback payloads
- audit events
- public status APIs
- emitted messages

This is what makes the test truly outside-in.

---

## Principle D — Separate test layers cleanly
You need multiple test layers, each with a clear purpose.

---

# 3. Test architecture overview

Use a **three-layer test pyramid**, but with stronger integration capability than a classic unit-heavy pyramid.

---

## Layer 1 — Contract and component integration tests
Purpose:
- validate each plugin/connector contract independently
- catch payload and schema mismatches early

Examples:
- git connector accepts commit request and writes expected path/content
- callback connector sends required headers/body
- policy plugin returns expected decision schema
- health poller interprets external status correctly

Characteristics:
- fast
- isolated
- high signal
- can use stubs/emulators

---

## Layer 2 — Workflow integration tests
Purpose:
- validate full workflow execution through the orchestrator
- use real module logic
- use controlled dependencies
- assert job completes correctly

Examples:
- request -> render -> policy -> approval -> git -> health -> callback -> succeeded
- request -> policy denied -> terminal failure
- request retried with same idempotency key -> no duplicate git/callback

Characteristics:
- medium speed
- deterministic
- close to production behavior

---

## Layer 3 — End-to-end acceptance tests
Purpose:
- validate business-critical user journeys in a realistic environment
- prove final product quality

Examples:
- provision resource in disposable environment and verify target resource healthy
- verify callback reaches external receiver
- verify audit trail and repository state

Characteristics:
- slowest
- fewer in number
- highest confidence

---

# 4. Reference architecture for the test environment

The key is to create a **deterministic integration sandbox**.

## 4.1 Test harness components

Build a reusable test harness containing:

### A. System under test
The real assembled service:
- orchestrator
- workflow engine
- plugin manager
- connector runtime
- persistence layer

### B. Controlled external dependencies
Replace uncontrolled third parties with test-grade equivalents:

- **Git service**: ephemeral git repo or local git server
- **Webhook receiver**: local HTTP capture service
- **Policy service**: deterministic service or fixed decision engine
- **Approval service**: deterministic auto-approval/rejection service
- **Target infra/cluster**: disposable environment or emulator
- **Message broker**: test instance if your system is async/event-driven
- **Database**: real test DB, resettable per run

### C. Observability plane
You need first-class observability in tests:

- structured logs
- job event stream
- correlation IDs
- trace IDs
- metrics
- callback capture
- repo commit capture
- resource state snapshots

Without this, failures will be expensive to diagnose.

---

## 4.2 Deployment topology

Use one of these:

### Option 1 — Docker Compose / local stack
Best for:
- developer workflows
- fast CI smoke integration
- deterministic connector behavior

Contains:
- app
- db
- broker
- local git service
- webhook listener
- mock policy/approval services

### Option 2 — Ephemeral Kubernetes namespace
Best for:
- systems already designed around Kubernetes/Crossplane
- more realistic lifecycle validation

Contains:
- system under test
- disposable namespace
- test CRDs/resources
- webhook capture service
- test repo secret/config

### Option 3 — Hybrid
Use Compose for workflow integration, and ephemeral cluster for acceptance tests.

This is often the most practical.

---

# 5. Critical design changes needed in the product

To support true outside-in testing, the product usually needs some refactoring.

---

## 5.1 Make runtime parameters configurable
The following should be runtime configuration, not hardcoded:

- health polling timeout
- polling interval
- retry count
- retry backoff
- callback timeout
- connector endpoints
- repo target
- approval provider endpoint
- policy provider endpoint

### Example
Production:
- polling interval = 15s
- timeout = 15m

Test:
- polling interval = 100ms
- timeout = 10s

This preserves logic while making tests practical.

---

## 5.2 Preserve workflow semantics
Do not change:
- the phase sequence
- state transition logic
- retry rules
- idempotency logic
- callback triggering rules

Only change:
- environment
- timing
- dependency endpoints

---

## 5.3 Add explicit state model
Your system needs a public or queryable state machine, e.g.:

- `accepted`
- `rendered`
- `policy_passed`
- `approval_granted`
- `committed`
- `provisioning`
- `healthy`
- `callback_sent`
- `succeeded`
- `failed`

This helps both:
- product observability
- robust test assertions

If internal phases exist but are not externally observable, expose them through a status API or event log.

---

## 5.4 Add correlation and auditability
Every request should carry:

- `job_id`
- `correlation_id`
- `idempotency_key`

These must appear in:
- logs
- events
- callback payload
- git commit metadata where relevant
- status records

This makes outside-in assertions possible.

---

# 6. Dependency strategy: real, fake, stub, or emulator?

Not all dependencies should be treated the same.

---

## 6.1 Use real implementations when they are core to quality
Use real where failure risk is high and interaction is important.

Examples:
- real database
- real message broker
- real git operations if commit semantics matter
- real local HTTP callback transmission

---

## 6.2 Use deterministic emulators where external systems are slow or costly
Examples:
- policy service emulator
- approval service emulator
- cloud provider emulator where possible
- Crossplane test target with fast-converging resources

---

## 6.3 Avoid simplistic mocks in outside-in tests
A mock that merely returns success immediately often hides integration risk.

If using a fake, make it **behavioral**, not just static.  
It should model:
- delays
- failures
- retries
- duplicates
- malformed responses when needed

---

# 7. Test scenario catalog

To claim integral quality, define a canonical set of end-to-end scenarios.

---

## 7.1 Golden path scenarios
These prove the happy path works.

### Scenario G1 — Full success
- submit valid job
- policy passes
- approval granted
- git commit succeeds
- target resource becomes healthy
- callback sent
- job reaches `succeeded`

Assertions:
- correct status transitions
- one commit created
- expected manifest committed
- callback received once
- final resource healthy

---

## 7.2 Failure path scenarios
These prove the system fails correctly.

### Scenario F1 — Policy denial
Assertions:
- no git commit
- no target provisioning
- no success callback
- job reaches expected rejected/failed state

### Scenario F2 — Approval timeout/rejection
Assertions:
- workflow stops at approval
- no downstream side effects

### Scenario F3 — Git failure
Assertions:
- retry behavior correct
- final failure diagnosable
- no false success

### Scenario F4 — Health never reaches healthy
Assertions:
- timeout behavior correct
- terminal failure state correct
- callback/final event reflects failure

### Scenario F5 — Callback failure
Assertions:
- retry or dead-letter behavior correct
- provisioning success is not misreported if callback fails, depending on design

---

## 7.3 Resilience scenarios
These are essential in plugin-heavy systems.

### Scenario R1 — Duplicate request with same idempotency key
Assertions:
- no duplicate commit
- no duplicate provisioning
- no duplicate callback

### Scenario R2 — Connector transient failure then recovery
Assertions:
- retries happen as designed
- eventual success without duplicate side effects

### Scenario R3 — Orchestrator restart mid-flight
Assertions:
- job resumes or recovers correctly
- no orphaned operations

### Scenario R4 — Slow downstream response
Assertions:
- system remains consistent
- timeout and retry rules honored

---

## 7.4 Contract drift scenarios
These catch ecosystem breakage.

### Scenario C1 — Plugin returns unexpected but schema-valid optional fields
### Scenario C2 — Connector response missing required field
### Scenario C3 — Version mismatch between plugin and orchestrator

These may not all be full E2E tests, but they should exist in workflow integration or contract suites.

---

# 8. Test data and fixture strategy

## 8.1 Standardized request fixtures
Create reusable request fixtures:

- minimal valid request
- full valid request
- invalid policy request
- invalid approval request
- duplicate/idempotent request
- slow resource request

Each fixture should include:
- explicit `job_id`
- explicit `idempotency_key`
- deterministic environment parameters

---

## 8.2 Deterministic resource templates
Use resource definitions that converge quickly and predictably.

For Crossplane-like systems, define:
- a lightweight test composition/resource
- a fast “healthy” path
- a known “never healthy” variant
- a known “malformed” variant

---

## 8.3 Seeded test repos and capture sinks
Use:
- ephemeral repo initialized at startup
- webhook sink that records all requests
- resettable DB fixtures

This ensures repeatability.

---

# 9. Observability requirements for testability

This is often the missing piece.

---

## 9.1 Structured lifecycle events
Emit events such as:

- `job.accepted`
- `job.rendered`
- `job.policy_passed`
- `job.approval_granted`
- `job.git_committed`
- `job.health_check_started`
- `job.healthy`
- `job.callback_sent`
- `job.succeeded`
- `job.failed`

Each event should include:
- timestamp
- job_id
- correlation_id
- phase
- outcome
- error_code if any

This makes assertions and diagnosis straightforward.

---

## 9.2 Public status read model
Expose a status endpoint or query surface:
```json
{
  "job_id": "...",
  "state": "provisioning",
  "phase": "health_polling",
  "attempts": 2,
  "last_error": null,
  "updated_at": "...",
  "artifacts": {
    "commit_sha": "...",
    "callback_attempts": 1
  }
}
```

Tests should assert on this, not on internal memory.

---

## 9.3 Callback capture service
Provide a simple test service that records:
- request body
- headers
- received timestamp
- number of deliveries

This is essential for outside-in verification.

---

# 10. CI/CD execution model

Use multiple lanes.

---

## 10.1 Lane A — Fast PR validation
Runs on every PR:
- contract tests
- component integration tests
- selected workflow smoke tests

Budget:
- a few minutes

Goal:
- catch obvious integration regressions quickly

---

## 10.2 Lane B — Full workflow integration
Runs on merge/main and maybe selective PRs:
- full lifecycle tests in controlled environment
- failure path tests
- idempotency tests

Budget:
- moderate

Goal:
- validate real orchestration behavior

---

## 10.3 Lane C — Acceptance / release validation
Runs before release or nightly:
- end-to-end scenarios
- restart/resilience cases
- environment realism
- plugin matrix where relevant

Goal:
- release confidence

---

# 11. Plugin/connector matrix strategy

For many plugins/connectors, exhaustive permutation testing becomes impossible.

So build a matrix based on risk.

---

## 11.1 Classify connectors by criticality
For each connector, score:
- business criticality
- change frequency
- historical instability
- external dependency volatility
- blast radius

Then define:
- **core set**: always tested in full workflow
- **secondary set**: tested on rotation or nightly
- **long tail**: covered by contract tests plus scheduled workflows

---

## 11.2 Use pairwise or risk-based combinations
Do not test every combination.

Test:
- the most common production combinations
- the highest-risk connector interactions
- at least one representative from each class

---

# 12. Acceptance criteria for “true outside-in capability”

You can say you have this capability when all of the following are true:

### Capability checklist
- [ ] Tests enter only through public interfaces
- [ ] Full workflow executes without hidden test-only shortcuts
- [ ] External dependencies are controllable and deterministic
- [ ] Job status is externally queryable
- [ ] Side effects are externally observable
- [ ] Assertions verify final product behavior, not internals
- [ ] Failure causes are diagnosable from logs/events/status
- [ ] Idempotency and retry semantics are tested
- [ ] At least one realistic full-lifecycle path passes in CI
- [ ] The suite catches regressions in connectors/plugins

---

# 13. Concrete implementation plan

Here is a practical phased rollout.

---

## Phase 0 — Clarify test taxonomy

Define and document these categories with clear examples:

### 1. Contract Tests
**Purpose**: Validate each plugin/connector contract independently, catching payload and schema mismatches early.
**Characteristics**: Fast, isolated, high signal, can use stubs/emulators.
**Examples**: 
- Git connector accepts commit request and writes expected path/content
- Callback connector sends required headers/body  
- Policy plugin returns expected decision schema
- Health poller interprets external status correctly
**Naming pattern**: `Test<Component>_Contract_<Scenario>` or `Test<Component>Contract_<Scenario>`

### 2. Workflow Integration Tests  
**Purpose**: Validate full workflow execution through the orchestrator using real module logic and controlled dependencies.
**Characteristics**: Medium speed, deterministic, close to production behavior.
**Examples**:
- Request → render → policy → approval → git → health → callback → succeeded
- Request → policy denied → terminal failure  
- Request retried with same idempotency key → no duplicate git/callback
**Naming pattern**: `Test<Module>Integration_<Scenario>` or `Test<Workflow>_Integration`

### 3. Acceptance Tests
**Purpose**: Validate business-critical user journeys in a realistic environment; prove final product quality.
**Characteristics**: Slowest, fewer in number, highest confidence.
**Examples**:
- Provision resource in disposable environment and verify target resource healthy
- Verify callback reaches external receiver  
- Verify audit trail and repository state
**Naming pattern**: `Test<Scenario>_Acceptance` or `Test<Component>_E2E`

Rename existing tests accordingly.
A test called `FullLifecycle` must truly be full lifecycle.

---

## Phase 1 — Introduce runtime configurability
Refactor the module/system so these are config-driven:

- polling intervals
- timeouts
- retry/backoff
- dependency endpoints
- repo targets
- callback targets

### Deliverable
A config profile set:

- `prod`
- `ci-integration`
- `local-dev`
- `acceptance`

---

## Phase 2 — Build the integration harness
Create reusable test infrastructure with:

- ephemeral DB
- broker
- local git target
- webhook receiver
- policy emulator
- approval emulator
- disposable target infra

### Deliverable
Single command to boot environment, e.g.:
```bash
make test-integration-env
```

---

## Phase 3 — Add public observability surfaces
Implement:
- status endpoint
- lifecycle events
- callback capture inspection
- commit/artifact inspection helpers

### Deliverable
Tests can assert externally without peeking inside process memory.

---

## Phase 4 — Create canonical workflow tests
Implement the first 5 must-have scenarios:

1. full success
2. policy denied
3. git failure
4. health timeout
5. duplicate idempotent request

### Deliverable
A stable workflow integration suite.

---

## Phase 5 — Add acceptance suite
Build a smaller but more realistic suite that runs:
- nightly
- before release
- on critical branches

### Deliverable
Evidence of final assembled product quality.

---

## Phase 6 — Expand connector/plugin coverage by risk
Add:
- matrix selection
- connector certification tests
- compatibility checks for plugin versions

### Deliverable
Scalable confidence as ecosystem grows.

---

# 14. Example architecture of one full-lifecycle test

## Test name
`TestProvisioning_FullLifecycle_Succeeds_OutsideIn`

## Setup
- start system under test
- start test DB
- start local git repo service
- start webhook capture service
- start deterministic policy + approval services
- start disposable target infra

## Input
Submit request through public API:

```json
{
  "job_id": "job-123",
  "idempotency_key": "idem-123",
  "infra_repo": "test/repo",
  "callback_url": "http://webhook-capture/callback",
  "resource_spec": {
    "name": "sample-a"
  }
}
```

## Test flow
1. POST request
2. Poll public status endpoint until terminal state or timeout
3. Query git repo for expected commit
4. Query target infra for expected resource state
5. Query webhook capture for received callback
6. Assert final status

## Assertions
- final job state = `succeeded`
- commit count = 1
- expected manifest path/content exists
- target resource = healthy
- callback deliveries = 1
- callback payload includes `job_id = job-123`
- no duplicate side effects

That is a true outside-in test.

---

# 15. What to change in the current failing test

For your current situation specifically:

## Do this
- decide whether the current test is really a rendering test or a full lifecycle test
- if rendering-only, rename and scope it narrowly
- if full lifecycle, remove the 5-second assumption and provide a real deterministic environment
- make health timing configurable
- provide all required params
- verify external outcomes, not just internal manifest generation

## Do not do this as the main approach
- add `test_mode` that skips git/polling/callback and still call it “full lifecycle”

---

# 16. Governance and ownership

This capability usually fails unless ownership is explicit.

Assign owners for:

- **test platform/harness**
- **connector certification**
- **workflow scenario catalog**
- **environment reliability**
- **observability standards**

Also require that every new connector/plugin provides:

- contract tests
- at least one workflow scenario contribution
- observability fields
- failure-mode definitions

---

# 17. Metrics for whether the capability is working

Track:

- workflow test pass rate
- flaky test rate
- mean time to diagnose failures
- escaped integration defects
- connector regression detection rate
- number of scenarios with outside-in coverage
- percentage of critical workflows covered

A testing capability is only real if it stays reliable and useful.

---


