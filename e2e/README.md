# End-to-End (E2E) Testing Infrastructure

This directory contains the infrastructure and test code for running end-to-end integration tests against the Release Engine.

## Overview

The E2E test validates the complete flow from authentication through job execution, verifying:
- OIDC authentication via Dex
- Gitea integration (PAT, org, repo creation)
- Platform secret storage
- Job creation and execution
- Approval step automation
- Callback handling
- Git commit verification

## Architecture

```mermaid
graph TB
    subgraph e2e-network["Docker Network (e2e)"]
        subgraph databases["Data Layer"]
            PG[(PostgreSQL<br/>:5432)]
            PGB(["PgBouncer<br/>:6432"])
        end
        
        subgraph storage["Storage Layer"]
            MINIO[/"MinIO<br/>:9000<br/>:9001"/]
        end
        
        subgraph identity["Identity & Access"]
            DEX(["Dex (OIDC)<br/>:5556"])
        end
        
        subgraph code["Code & Collaboration"]
            GITEA(["Gitea<br/>:3000<br/>:22"])
        end
        
        subgraph application["Application"]
            RE{{"Release Engine<br/>:8080"}}
        end
        
        subgraph testing["Test Infrastructure"]
            CB([Callback Sink<br/>:9090])
            E2E[["E2E Test Runner<br/>(Go test)"]]
        end
    end
    
    subgraph external["External / Host"]
        TESTER[Test User<br/>password grant]
    end
    
    %% Connections
    E2E -->|Password Grant| DEX
    E2E -->|Admin API| RE
    E2E -->|PAT| GITEA
    E2E -->|JWT Token| RE
    E2E -->|GET /callback| CB
    
    RE -->|DB URL| PGB
    PGB -->|Connection Pool| PG
    RE -->|S3 API| MINIO
    RE -->|Token Validation| DEX
    RE -->|Webhook/CB| CB
    
    DEX -->|Password Auth| TESTER
    GITEA -->|gitea db| PG
    
    %% Styling
    classDef database fill:#f9f,stroke:#333,stroke-width:2px
    classDef storage fill:#bfb,stroke:#333,stroke-width:2px
    classDef identity fill:#fb9,stroke:#333,stroke-width:2px
    classDef code fill:#9ff,stroke:#333,stroke-width:2px
    classDef app fill:#ff9,stroke:#333,stroke-width:2px
    classDef test fill:#9f9,stroke:#333,stroke-width:2px
    
    class PG,PGB database
    class MINIO storage
    class DEX identity
    class GITEA code
    class RE app
    class CB,E2E test
```

## Services

| Service | Image | Ports | Purpose                                           |
|---------|-------|-------|---------------------------------------------------|
| **postgres** | postgres:15-alpine | 5432 | Primary database for Release Engine               |
| **pgbouncer** | edoburu/pgbouncer:latest | 6432 | Connection pooler for database                    |
| **minio** | minio/minio:latest | 9000, 9001 | S3-compatible object storage for secrets vault    |
| **gitea** | gitea/gitea:1.21 | 3000, 2222 | Git server (Code repositories for infrastructure) |
| **dex** | ghcr.io/dexidp/dex:v2.38.0 | 5556 | OIDC provider for authentication                  |
| **release-engine** | (builds from Dockerfile) | 8080 | The application being tested                      |
| **callback-sink** | python:3.12-alpine | 9090 | HTTP server to capture webhook callbacks          |

## Bootstrap Sequence Diagram

The following diagram shows the step-by-step flow of the E2E bootstrap process:

### Step-by-Step Explanation

| Step | Name | Participants | Description |
|------|------|--------------|-------------|
| **0** | Authentication | E2E Test Runner → Dex | The E2E test runner authenticates with Dex (OIDC provider) using password grant flow to obtain a JWT ID token. This token is used for subsequent Release Engine API calls. |
| **1** | Gitea Bootstrap | E2E → CLI → Gitea | Creates the initial Gitea infrastructure: (1) Runs `gitea-init.sh` to create an admin user, (2) Creates a bootstrap token with `write:user` scope, (3) Creates a Personal Access Token (PAT) with `read:user` and `repo` scopes, (4) Creates the `test-org` organization, (5) Creates the `test-repo` repository with `auto_init=true`, (6) Polls until the main branch exists (max 30s). |
| **1b** | Pre-flight Check | E2E → Gitea | Verifies the repository is accessible by fetching the main branch using the PAT. Confirms all Gitea resources are properly initialized before proceeding. |
| **2** | Store Secret | E2E → Release Engine → MinIO | The E2E test stores the Gitea PAT securely in the Release Engine's secrets vault. The API call `PUT /internal/v1/platform/secrets/git-access-token` triggers encryption and storage in MinIO. |
| **3** | Setup Callback | E2E → Callback Sink | Verifies the callback-sink service is running and accessible at port 9090. This service will capture webhook callbacks from the Release Engine when jobs complete. |
| **4** | Create Job | E2E → Release Engine → DB | Creates a new job using the `infra.provision` module, referencing the stored secret (`secret_ref`) and providing the callback URL (`http://callback-sink:9090`). Returns 202 Accepted with a job ID. |
| **4b** | Auto-Approve | E2E ↔ Release Engine | Runs in parallel with job execution: (1) An auto-approve watcher polls every 200ms for steps in `waiting_approval` state and approves them, (2) The job runner clones the repo, executes module steps, pushes commits, and sends callbacks. |
| **5** | Verification | E2E → RE → DB → CB → Gitea | Final validation: (1) Polls job status until completion, (2) Queries PostgreSQL directly to verify job state, (3) Fetches callback payload from sink, (4) Verifies the commit SHA in Gitea matches the callback payload. |



```mermaid
sequenceDiagram
    autonumber
    participant E2E as E2E Test Runner
    participant DEX as Dex (OIDC)
    participant CLI as Gitea CLI
    participant GITEA as Gitea API
    participant RE as Release Engine
    participant DB as PostgreSQL
    participant CB as Callback Sink
    participant MINIO as MinIO

    Note over E2E: Test starts with empty state
    Note over E2E: All services must be healthy

    rect rgb(200, 230, 255)
        Note over E2E: Step 0 - Authentication
        E2E->>DEX: POST /dex/token (password grant)
        DEX-->>E2E: JWT Token (ID Token)
    end

    rect rgb(255, 230, 200)
        Note over E2E,GITEA: Step 1 - Gitea Bootstrap (CLI + API)

        E2E->>CLI: Run gitea-init.sh
        CLI->>GITEA: docker exec gitea admin user create
        GITEA-->>CLI: Admin user created
        CLI-->>E2E: Admin user ready

        E2E->>GITEA: POST /api/v1/users/{admin}/tokens
        GITEA-->>E2E: Bootstrap Token (write:user)

        E2E->>GITEA: POST /api/v1/users/{admin}/tokens<br/>(Basic Auth, scopes: read:user, repo)
        GITEA-->>E2E: Personal Access Token (PAT)

        E2E->>GITEA: POST /api/v1/orgs (test-org)
        GITEA-->>E2E: Organization created

        E2E->>GITEA: POST /api/v1/orgs/test-org/repos (test-repo, auto_init)
        GITEA-->>E2E: Repository created

        loop Poll until main exists (max 30s)
            E2E->>GITEA: GET /api/v1/repos/test-org/test-repo/branches/main
            GITEA-->>E2E: Branch exists
        end
    end

    rect rgb(255, 200, 255)
        Note over E2E,GITEA: Step 1b - Pre-flight Check
        E2E->>GITEA: GET /api/v1/repos/test-org/test-repo/branches/main (with PAT)
        GITEA-->>E2E: 200 OK (resources confirmed)
    end

    rect rgb(200, 255, 200)
        Note over E2E,MINIO: Step 2 - Store Secret
        E2E->>RE: PUT /internal/v1/platform/secrets/git-access-token
        RE->>MINIO: Store encrypted secret
        MINIO-->>RE: Secret stored
        RE-->>E2E: 200 OK
    end

    rect rgb(255, 200, 200)
        Note over E2E,CB: Step 3 - Setup Callback
        Note over E2E: callback-sink at http://callback-sink:9090
        Note over E2E: verified via http://localhost:9090
    end

    rect rgb(200, 200, 255)
        Note over E2E,DB: Step 4 - Create Job
        E2E->>RE: POST /v1/jobs (infra.provision, secret_ref, callback_url)
        RE->>DB: Insert job record
        RE-->>E2E: 202 Accepted (job_id)
    end

    rect rgb(255, 255, 200)
        Note over E2E,CB: Step 4b - Auto-Approve
        par Auto-Approve Watcher (goroutine)
            loop Every 200ms until complete
                E2E->>RE: GET /v1/jobs (step_status=waiting_approval)
                RE-->>E2E: Steps with waiting_approval

                alt Step is for our job
                    E2E->>RE: POST /v1/jobs/{id}/steps/{step_id}/decisions<br/>{"decision": "approved"}
                    RE-->>E2E: 200 OK
                end
            end
        and Job Execution (parallel)
            RE->>GITEA: Clone test-org/test-repo
            RE->>RE: Execute infra module steps
            RE->>GITEA: Push commit
            RE->>CB: POST callback (job_id, status, commit_sha)
            CB-->>RE: 200 OK
        end
    end

    rect rgb(220, 220, 220)
        Note over E2E,GITEA: Step 5 - Verification

        loop Poll for job completion
            E2E->>RE: GET /v1/jobs/{job_id}
            alt Job succeeded
                RE-->>E2E: state=succeeded
            else Job failed
                RE-->>E2E: state=failed, error details
                Note over E2E: FAIL test
            end
        end

        E2E->>DB: SELECT state FROM jobs WHERE id={job_id}
        DB-->>E2E: state=succeeded (DB verification)

        E2E->>CB: GET / (verify callback received)
        CB-->>E2E: job_id, status=succeeded, commit_sha

        E2E->>GITEA: GET /api/v1/repos/test-org/test-repo/commits?sha=main
        GITEA-->>E2E: Commits list
        Note over E2E: Verify commit SHA matches callback payload
    end

    Note over E2E: All verifications passed
    Note over E2E: Test completes successfully
```

## Data Flow Diagram

```mermaid
flowchart LR
    subgraph auth["Authentication Flow"]
        A1[Test User<br/>Credentials] --> A2[Password Grant<br/>to Dex]
        A2 --> A3[JWT Token]
    end

    subgraph gitea_setup["Gitea Bootstrap"]
        G1[Admin User<br/>Creation] --> G2[PAT<br/>Generation]
        G2 --> G3[Organization<br/>test-org]
        G3 --> G4[Repository<br/>test-repo]
        G4 --> G5[Main Branch<br/>Initialized]
    end

    subgraph secrets["Secret Management"]
        S1[Gitea PAT] --> S2[Admin API<br/>PUT secret]
        S2 --> S3[Release Engine]
        S3 --> S4[MinIO<br/>Encrypted Storage]
    end

    subgraph job_exec["Job Execution"]
        J1[Create Job<br/>with secret_ref] --> J2[Runner fetches<br/>secret from vault]
        J2 --> J3[Git Clone<br/>with PAT]
        J3 --> J4[Execute Steps]
        J4 --> J5[Push Commit]
        J5 --> J6[Send Callback]
    end

    subgraph verification["Verification"]
        V1[Job State API] --> V2[Database Query]
        V2 --> V3[Callback Payload]
        V3 --> V4[Git Commit<br/>Verification]
    end

    auth --> gitea_setup
    gitea_setup --> secrets
    secrets --> job_exec
    job_exec --> verification
```

## Environment Variables

The E2E tests are configured via environment variables with sensible defaults:

| Variable | Default | Description |
|----------|---------|-------------|
| `RELEASE_ENGINE_URL` | http://localhost:8080 | Release Engine API URL |
| `GITEA_URL` | http://localhost:3000 | Gitea API URL |
| `DEX_URL` | http://localhost:5556 | Dex OIDC URL |
| `TENANT_ID` | test-tenant | Tenant identifier |
| `OIDC_CLIENT_ID` | release-engine | OIDC client ID |
| `OIDC_CLIENT_SECRET` | example-secret | OIDC client secret |
| `TEST_USERNAME` | test-user@example.com | Test user for authentication |
| `TEST_PASSWORD` | password | Test user password |
| `GITEA_ADMIN_USER` | gitadmin | Gitea admin username |
| `GITEA_ADMIN_PASSWORD` | admin-password | Gitea admin password |
| `TEST_TIMEOUT` | 5m | Overall test timeout |
| `JOB_EXECUTION_TIMEOUT` | 45s | Maximum time for job execution |
| `API_CLIENT_TIMEOUT` | 30s | HTTP client timeout for API calls |

## Running the Tests

### Prerequisites

1. **Docker and Docker Compose** must be installed and running
2. **Go 1.21+** must be installed for running the tests
3. **Make** must be available

### Run the E2E Tests

Use the `test-e2e` Makefile target from the project root. This target handles:
- Tearing down any previous Docker Compose services
- Building the Release Engine Linux binary
- Building and starting all Docker Compose services
- Running the E2E tests
- Cleaning up Docker Compose services

```bash
# Run E2E tests
make test-e2e

# Run with coverage (generates coverage report in coverage/e2e.cover.out)
make test-e2e COVER=1

# Run configuration tests (no services required)
go test -tags=e2e ./e2e/bootstrap -run TestE2EConfigDefaults -v
go test -tags=e2e ./e2e/bootstrap -run TestE2EConfigOverrides -v
```

## Test Structure

```
e2e/
├── README.md                    # This file
├── docker-compose.yml           # Service definitions
├── .env                         # Environment defaults
├── .gitignore                   # Ignore test artifacts
├── bootstrap/                   # Go test code
│   ├── e2e.go                   # Main bootstrap orchestration
│   ├── e2e_test.go             # Test definitions
│   ├── gitea.go                # Gitea client and bootstrap
│   ├── gitea_cli.go            # Gitea CLI wrapper
│   ├── gitea_test.go           # Gitea client tests
│   └── oidc.go                 # OIDC client (Dex)
├── configs/                     # Service configurations
│   ├── dex/config.yaml         # Dex OIDC configuration
│   ├── gitea/app.ini           # Gitea configuration
│   ├── minio/                   # MinIO configuration
│   ├── pgbouncer/              # PgBouncer configuration
│   └── postgres/               # PostgreSQL init scripts
└── scripts/                     # Utility scripts
    └── gitea-init.sh           # Gitea admin user creation
```

## Key Test Assertions

The E2E test verifies:

1. **Authentication**: JWT token obtained successfully from Dex
2. **Gitea Bootstrap**: Admin user, PAT, org, and repo created
3. **Secret Storage**: PAT stored securely via Admin API
4. **Job Creation**: Job created with secret reference
5. **Auto-Approval**: Approval steps automatically approved
6. **Job Execution**: Job reaches "succeeded" state
7. **Database State**: Job state persisted correctly
8. **Callback Received**: Webhook callback captured by sink
9. **Git Verification**: Commit made to repository with correct SHA

## Troubleshooting

### Services not healthy

```bash
# Check service logs
docker compose logs postgres
docker compose logs gitea
docker compose logs dex

# Restart specific service
docker compose restart release-engine
```

### Test times out

```bash
# Increase timeouts
export TEST_TIMEOUT=15m
export JOB_EXECUTION_TIMEOUT=3m

# Or check if services are under heavy load
docker stats
```

### Gitea admin user creation fails

```bash
# Check if Gitea is fully initialized
docker compose exec gitea bash -c "gitea admin user list --config /data/gitea/conf/app.ini"

# Manually create admin via API
curl -X POST http://localhost:3000/api/v1/admin/users \
  -H "Content-Type: application/json" \
  -d '{"username":"gitadmin","email":"gitadmin@local.dev","password":"admin-password"}'
```

### Callback not received

```bash
# Check callback-sink is running
docker compose logs callback-sink

# Manually test callback sink
curl -X POST http://localhost:9090 -d '{"test":"data"}'
curl http://localhost:9090