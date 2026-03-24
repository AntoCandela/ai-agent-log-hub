# Contributing to AI Agent Log Hub

Thanks for your interest in contributing! This guide will help you get started.

## Development Setup

### Prerequisites

- Go 1.25+
- Docker & Docker Compose
- [golangci-lint](https://golangci-lint.run/welcome/install/)
- [govulncheck](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck) (`go install golang.org/x/vuln/cmd/govulncheck@latest`)

### Getting Started

```bash
git clone https://github.com/AntoCandela/ai-agent-log-hub.git
cd ai-agent-log-hub
cp .env.example .env       # Set POSTGRES_PASSWORD
docker compose up db -d     # Start PostgreSQL
cd backend
export DATABASE_URL="postgres://loghub:yourpassword@localhost:5432/loghub?sslmode=disable"
go build ./cmd/loghub
./loghub
```

### Running Quality Checks

```bash
make fmt      # Check formatting
make lint     # Run golangci-lint
make vet      # Run go vet
make test     # Run tests with race detection
make cover    # Generate coverage report
make vuln     # Check for known vulnerabilities
make ci       # Run all checks (what CI runs)
```

Always run `make ci` before pushing.

## Code Style

- Follow standard Go conventions (`gofmt`, `goimports`)
- Use `slog` for structured logging
- Use `pgx` for database access — no ORMs
- Keep files focused — one responsibility per file
- Prefer returning errors over panicking
- Write tests for new functionality

## Pull Request Process

1. Fork the repository and create a feature branch
2. Make your changes with clear, atomic commits
3. Ensure `make ci` passes locally
4. Open a PR against `main` with a clear description
5. All CI checks must pass before merge

### Commit Messages

Use [conventional commits](https://www.conventionalcommits.org/):

```
feat: add session comparison endpoint
fix: handle nil pointer in agent lookup
chore: update golangci-lint config
docs: add API authentication guide
test: add coverage for OTLP log handler
refactor: extract event validation to separate file
```

### PR Description

Include:
- What changed and why
- How to test it
- Any breaking changes

## Project Structure

```
backend/
├── cmd/loghub/       # Entry point — keep thin
├── internal/
│   ├── config/       # Environment configuration
│   ├── handler/      # HTTP handlers — validation + delegation
│   ├── middleware/    # Request middleware chain
│   ├── model/        # Domain types — no business logic
│   ├── otlp/         # OpenTelemetry receivers
│   ├── repository/   # Database access — SQL only
│   └── service/      # Business logic
└── migrations/       # SQL migrations (golang-migrate)
```

**Layering rules:**
- Handlers call services, never repositories directly
- Services call repositories
- Repositories contain SQL only, no business logic
- Models are plain structs, shared across layers

## Reporting Issues

Use [GitHub Issues](https://github.com/AntoCandela/ai-agent-log-hub/issues). Include:
- What you expected vs what happened
- Steps to reproduce
- Go version, OS, and Docker version

## License

By contributing, you agree that your contributions will be licensed under the Apache License 2.0.
