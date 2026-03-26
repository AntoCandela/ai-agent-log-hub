# AI Agent Log Hub

Unified observability platform for **any AI coding agent** — Claude Code, Cursor, Aider, LangChain agents, or your own custom tools. Capture, correlate, and query tool calls, file modifications, git commits, and system telemetry with end-to-end traceability.

**Not just another Claude Code extension.** While most AI observability tools lock you into a single agent runtime, Log Hub works with any agent that can make HTTP calls or emit OpenTelemetry traces. Ship events from multiple agents, correlate their actions with your backend telemetry, and build up cross-agent knowledge over time.

## Why This Exists

AI coding agents are powerful but opaque. When an agent modifies your codebase across a 30-minute session, you're left with a git diff and no context. Log Hub gives you:

- **Full session replay** — every tool call, file edit, command, and decision, in order
- **Cross-layer tracing** — follow an agent's curl command through your backend, into the database, and back
- **Agent memory** — agents store and recall knowledge across sessions via semantic search
- **Multi-agent visibility** — see what all your agents did across all your projects, in one place

## Supported Agents

| Agent | Integration | How |
|-------|-----------|-----|
| **Claude Code** | Native hooks + MCP | Automatic capture via PostToolUse hook + 15 MCP query tools |
| **Cursor** | HTTP API | POST events to `/api/v1/events` |
| **Aider** | HTTP API | POST events to `/api/v1/events` |
| **LangChain / LangGraph** | HTTP API + OTLP | Event API for agent actions + OTLP for chain traces |
| **Custom agents** | HTTP API + OTLP | Any agent that can make HTTP calls or emit OpenTelemetry |
| **Any backend** | OTLP | Standard OpenTelemetry traces and logs |

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

### Connect Claude Code (MCP)

```jsonc
// .claude/settings.json or project .mcp.json
{
  "mcpServers": {
    "loghub": {
      "command": "loghub-mcp",
      "env": {
        "LOGHUB_URL": "http://localhost:4800",
        "LOGHUB_AGENT_ID": "claude-code"
      }
    }
  }
}
```

## Architecture

```
┌──────────────┐
│ Claude Code   │──hook──▶┐
├──────────────┤         │
│ Cursor/Aider  │──HTTP──▶│    ┌──────────────┐    ┌──────────────────┐
├──────────────┤         ├───▶│   Backend    │───▶│   PostgreSQL 16  │
│ LangChain     │──OTLP──▶│    │   (Go/Chi)   │    │   + pgvector     │
├──────────────┤         │    │  :4800       │    │                  │
│ Your backend  │──OTLP──▶│    └──────────────┘    │  9 tables        │
├──────────────┤         │                         │  Vector search   │
│ Browser       │──HTTP──▶┘                         │  Full-text index │
└──────────────┘                                    └──────────────────┘
```

## API Endpoints

### Ingestion
| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/events` | Ingest agent events (single or batch) |
| POST | `/v1/traces` | OTLP trace ingestion (JSON) |
| POST | `/v1/logs` | OTLP log ingestion (JSON) |

### Query
| Method | Path | Description |
|--------|------|-------------|
| GET | `/healthz` | Health check with DB status |
| GET | `/api/v1/logs` | Search logs with composable filters |
| GET | `/api/v1/sessions` | List sessions with filters |
| GET | `/api/v1/sessions/{id}` | Session detail |
| GET | `/api/v1/sessions/{id}/files` | Files touched in session |
| GET | `/api/v1/sessions/{id}/summary` | Auto-generated session summary |
| GET | `/api/v1/logs/errors` | Error search with surrounding context |
| GET | `/api/v1/logs/blame` | Which agent last modified a file |
| GET | `/api/v1/system` | System events (OTLP data) |
| GET | `/api/v1/traces/{traceID}` | Cross-layer trace waterfall |

### Memory
| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/memory` | Store agent memory (key-value + embedding) |
| POST | `/api/v1/memory/search` | Semantic search over memories |
| POST | `/api/v1/memory/recall` | Contextual recall (find similar past situations) |
| GET | `/api/v1/memory` | List stored memories |
| DELETE | `/api/v1/memory/{key}` | Delete a memory |

### MCP Tools (for Claude Code)

15 tools available when connected via MCP: `log.emit`, `log.query`, `log.session`, `log.sessions`, `log.files`, `log.errors`, `log.summary`, `log.blame`, `log.system`, `log.trace`, `memory.store`, `memory.search`, `memory.recall`, `memory.list`, `memory.delete`

## Project Structure

```
ai-agent-log-hub/
├── backend/              # Go backend (API, ingestion, storage)
│   ├── cmd/loghub/       # Entry point
│   ├── internal/
│   │   ├── config/       # Environment configuration
│   │   ├── handler/      # HTTP handlers (events, logs, sessions, memory, traces)
│   │   ├── middleware/    # Request ID, logging, CORS, auth, rate limiting
│   │   ├── model/        # Domain types (Agent, Session, Event, Memory, Summary)
│   │   ├── otlp/         # OpenTelemetry trace and log receivers
│   │   ├── embed/        # Embedding service client (local + API backends)
│   │   ├── repository/   # Database access (SQL, pgvector search)
│   │   └── service/      # Business logic (session lifecycle, summaries)
│   ├── migrations/       # SQL schema (9 tables, indexes, triggers)
│   └── tests/            # Unit and integration tests
├── mcp/                  # MCP server binary (15 tools, stdio transport)
├── hooks/                # Claude Code hook binary (event capture + sanitization)
├── frontend/             # Web UI (coming soon)
└── docker-compose.yml    # Full stack: PostgreSQL + backend + embedding service
```

## Development

### Prerequisites

- Go 1.24+
- Docker & Docker Compose

### Run locally

```bash
# Start PostgreSQL and embedding service
docker compose up db embedding -d

# Build and run the backend
cd backend
cp ../.env.example ../.env  # set POSTGRES_PASSWORD
export DATABASE_URL="postgres://loghub:yourpassword@localhost:5432/loghub?sslmode=disable"
go build ./cmd/loghub
./loghub
```

### Run quality checks

```bash
make ci    # fmt + lint + vet + test + vuln
```

### Run tests

```bash
cd backend
go test ./... -v
```

## Configuration

All configuration via environment variables. See [`.env.example`](.env.example) for the full list.

| Variable | Default | Description |
|----------|---------|-------------|
| `API_PORT` | `4800` | HTTP server port |
| `DATABASE_URL` | (required) | PostgreSQL connection string |
| `LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |
| `AUTH_ENABLED` | `false` | Enable API key authentication |
| `AGENT_SESSION_TIMEOUT` | `30m` | Session inactivity timeout |
| `RETENTION_DAYS` | `90` | Days to retain event data |
| `EMBEDDING_BACKEND` | `local` | Embedding backend (`local` or `api`) |

## Roadmap

- [x] **MS-1 Foundation** — Event ingestion, sessions, OTLP, PostgreSQL schema
- [x] **MS-2 Agent Query & MCP** — REST query endpoints, MCP server, Claude Code hook, memory system
- [x] **MS-4 CI/CD** — GitHub Actions, linting, branch protection
- [ ] **MS-3 Observability UI** — Web dashboard, live tail, session explorer, log search
- [ ] **MS-5 Knowledge Graph** — TypeDB integration for causal reasoning and cross-agent learning

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for setup instructions, code style, and PR process.

## License

Licensed under the [Apache License, Version 2.0](LICENSE).
