.PHONY: all build test lint fmt vet cover clean

# Default target
all: fmt lint vet test build

# Build the backend binary
build:
	cd backend && go build -o loghub ./cmd/loghub

# Run all tests with race detection
test:
	cd backend && go test -race ./... -v

# Run golangci-lint
lint:
	cd backend && golangci-lint run ./...

# Check formatting (fails if files need formatting)
fmt:
	@test -z "$$(cd backend && gofmt -l .)" || (echo "Files need formatting:" && cd backend && gofmt -l . && exit 1)

# Run go vet
vet:
	cd backend && go vet ./...

# Run tests with coverage report
cover:
	cd backend && go test -race -coverprofile=coverage.out ./...
	cd backend && go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: backend/coverage.html"

# Check for known vulnerabilities
vuln:
	cd backend && govulncheck ./...

# Run all quality checks (what CI runs)
ci: fmt lint vet test vuln

# Clean build artifacts
clean:
	rm -f backend/loghub backend/coverage.out backend/coverage.html

# Start the stack locally
up:
	docker compose up --build -d

# Stop the stack
down:
	docker compose down

# Stop and remove volumes
down-v:
	docker compose down -v
