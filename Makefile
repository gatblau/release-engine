# Makefile for release-engine

.PHONY: lint test security

# Run golangci-lint
lint:
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
