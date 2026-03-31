# Makefile for release-engine

.PHONY: lint test test-smoke test-infra-integration security

# Check if golangci-lint is installed, if not install it
lint-check:
	@echo "Checking if golangci-lint is installed..."
	@which golangci-lint || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)

# Run golangci-lint
lint: lint-check
	@echo "Running linter..."
	@golangci-lint run ./...

# Run tests with coverage (excluding smoke tests - they have no production code to cover)
test:
	@echo "Running tests with coverage..."
	@go test -tags=integration -coverprofile=coverage.out $$(go list ./... | grep -v /internal/smoke)
	@go tool cover -func=coverage.out

# Run smoke tests (containers verification - no production code to cover)
test-smoke:
	@echo "Running smoke tests (container verification)..."
	@go test -tags=integration -v ./internal/smoke/...

# Run infra integration tests (phase 4 scenarios)
test-infra-integration:
	@echo "Running infra integration tests..."
	@go test -tags=integration -v ./internal/integration -run TestInfraIntegration

test-race:
	@echo "Running tests with race detection..."
	@go test -race -count=1 ./...

# Run security checks (go install github.com/securego/gosec/v2/cmd/gosec@latest)
security:
	@echo "Running security checks..."
	@go run github.com/securego/gosec/v2/cmd/gosec@latest -exclude=G101 -quiet ./...

# Run E2E tests (requires Docker Compose)
# Usage: make test-e2e [COVER=1] [KEEP_UP=1]
test-e2e:
	@echo "=== Tearing down any previous Docker Compose services ==="
	@docker compose -f e2e/docker-compose.yml down -v --remove-orphans || true
	@rm -f e2e/gitea_pat.txt
	@rm -f release-engine
	@echo "=== Building Linux release-engine binary ==="
	@CGO_ENABLED=0 GOOS=linux go build -o release-engine ./cmd/release-engine/
	@docker compose -f e2e/docker-compose.yml build --no-cache
	@echo "=== Starting Docker Compose services ==="
	@docker compose -f e2e/docker-compose.yml up --force-recreate -d --wait
	@echo "=== Running E2E tests ==="
ifdef COVER
	@echo "Coverage collection enabled"
	@mkdir -p coverage
	@TEST_TIMEOUT=10m \
	RELEASE_ENGINE_URL=http://localhost:8080 \
	GITEA_URL=http://localhost:3000 \
	DEX_URL=http://localhost:5556 \
	TENANT_ID=test-tenant \
	OIDC_CLIENT_ID=release-engine \
	OIDC_CLIENT_SECRET=example-secret \
	TEST_USERNAME=test-user@example.com \
	TEST_PASSWORD=password \
	GITEA_ADMIN_USER=gitadmin \
	GITEA_ADMIN_PASSWORD=admin-password \
	go test -tags=e2e ./e2e/bootstrap -run TestRunE2E \
		-coverprofile=coverage/e2e.cover.out \
		-covermode=atomic \
		-v
	@echo "=== Coverage report ==="
	@go tool cover -func=coverage/e2e.cover.out | grep -E "TOTAL|e2e/bootstrap"
else
	@TEST_TIMEOUT=10m \
	RELEASE_ENGINE_URL=http://localhost:8080 \
	GITEA_URL=http://localhost:3000 \
	DEX_URL=http://localhost:5556 \
	TENANT_ID=test-tenant \
	OIDC_CLIENT_ID=release-engine \
	OIDC_CLIENT_SECRET=example-secret \
	TEST_USERNAME=test-user@example.com \
	TEST_PASSWORD=password \
	GITEA_ADMIN_USER=gitadmin \
	GITEA_ADMIN_PASSWORD=admin-password \
	go test -tags=e2e ./e2e/bootstrap -run TestRunE2E -v
endif
ifdef KEEP_UP
	@echo "=== Keeping Docker Compose services up (skip teardown) ==="
else
	@echo "=== Cleaning up Docker Compose services ==="
	@docker compose -f e2e/docker-compose.yml down
	@rm -f release-engine
endif
