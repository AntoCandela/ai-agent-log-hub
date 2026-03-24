# AI Agent Log Hub

Unified observability platform for AI agent behavior traces and application logs. Capture, correlate, and query tool calls, file modifications, git commits, and system telemetry with end-to-end traceability.

Think "Datadog for AI agents" — where agents can also query their own logs as a knowledge base.

## Features

- **Agent event capture** — tool calls, explicit logs, git commits, file changes, errors
- **OTLP ingestion** — accept OpenTelemetry traces and logs from backend services
- **Auto-registration** — agents are registered on first event, sessions created automatically
- **Cross-layer correlation** — link agent actions to system traces via trace IDs
- **Dual ingestion** — HTTP API for agent events + OTLP for system telemetry
- **Session lifecycle** — automatic session creation, timeout, and closure
- **Composable queries** — filter events by agent, session, tool, severity, time range, full-text search

## Quick Start

```bash
cp .env.example .env   # then edit .env to set POSTGRES_PASSWORD
docker compose up
```

The backend API will be available at `http://localhost:4800`.

### Verify it's running

```bash
curl http://localhost:4800/healthz
# {"status":"ok","db":"connected"}
```

### Send your first event

```bash
curl -X POST http://localhost:4800/api/v1/events \
  -H "Content-Type: application/json" \
  -d '{
    "agent_id": "my-agent",
    "session_token": "session-001",
    "timestamp": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'",
    "event_type": "tool_call",
    "tool_name": "Bash",
    "message": "Ran tests"
  }'
```

## Architecture

```
┌─────────────┐    ┌──────────────┐    ┌──────────────────┐
│ Claude Code  │───▶│              │    │                  │
│ Hooks        │    │   Backend    │───▶│   PostgreSQL 16  │
│              │    │   (Go/Chi)   │    │   + pgvector     │
├─────────────┤    │              │    │                  │
│ OTLP Sources │───▶│  :4800       │    │  9 tables        │
│ (traces/logs)│    │              │    │  Full-text search │
├─────────────┤    │              │    │  Vector embeddings│
│ HTTP API     │───▶│              │    │                  │
└─────────────┘    └──────────────┘    └──────────────────┘
```

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/healthz` | Health check with DB status |
| POST | `/api/v1/events` | Ingest agent events (single or batch) |
| POST | `/v1/traces` | OTLP trace ingestion (JSON) |
| POST | `/v1/logs` | OTLP log ingestion (JSON) |

## Project Structure

```
ai-agent-log-hub/
├── backend/              # Go backend
│   ├── cmd/loghub/       # Entry point
│   ├── internal/
│   │   ├── config/       # Environment configuration
│   │   ├── handler/      # HTTP handlers
│   │   ├── middleware/    # Request ID, logging, CORS, auth, rate limiting
│   │   ├── model/        # Domain types
│   │   ├── otlp/         # OpenTelemetry receivers
│   │   ├── repository/   # Database access layer
│   │   └── service/      # Business logic
│   └── migrations/       # SQL schema migrations
├── frontend/             # Web UI (coming soon)
├── mcp/                  # MCP server for agent tools (coming soon)
└── docker-compose.yml
```

## Development

### Prerequisites

- Go 1.25+
- Docker & Docker Compose
- PostgreSQL 16 with pgvector (or use Docker Compose)

### Run locally

```bash
# Start PostgreSQL
docker compose up db -d

# Build and run the backend
cd backend
export DATABASE_URL="postgres://loghub:loghub_dev@localhost:5432/loghub?sslmode=disable"
go build ./cmd/loghub
./loghub
```

### Run tests

```bash
cd backend
go test ./... -v
```

## Configuration

All configuration is via environment variables. See [`.env.example`](.env.example) for the full list with defaults.

| Variable | Default | Description |
|----------|---------|-------------|
| `API_PORT` | `4800` | HTTP server port |
| `DATABASE_URL` | (required) | PostgreSQL connection string |
| `LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |
| `AUTH_ENABLED` | `false` | Enable API key authentication |
| `AGENT_SESSION_TIMEOUT` | `30m` | Session inactivity timeout |
| `RETENTION_DAYS` | `90` | Days to retain event data |

## Roadmap

- **MS-1 Foundation** — Log capture, event ingestion, sessions, OTLP (done)
- **MS-2 Agent Query & MCP** — MCP tools for agents to query their own history and store memories
- **MS-3 Observability UI** — Web dashboard with live tail, session explorer, log search

## License

Licensed under the [Apache License, Version 2.0](LICENSE).
