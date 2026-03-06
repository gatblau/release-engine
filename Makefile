# Makefile for release-engine

.PHONY: lint test security

# Check if golangci-lint is installed, if not install it
lint-check:
	@echo "Checking if golangci-lint is installed..."
	@which golangci-lint || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.5)

# Run golangci-lint
lint: lint-check
	@echo "Running linter..."
	@golangci-lint run ./...

# Run tests with coverage
test:
	@echo "Running tests with coverage..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out

# Run security checks (go install github.com/securego/gosec/v2/cmd/gosec@latest)
security:
	@echo "Running security checks..."
	@go run github.com/securego/gosec/v2/cmd/gosec@latest -exclude=G101 -quiet ./...
