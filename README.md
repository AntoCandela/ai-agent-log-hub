# AI Agent Log Hub

Unified observability platform for AI agent behavior traces and application logs.

## Quick Start

```bash
docker compose up
```

The backend will be available at `http://localhost:4800`.

## Project Structure

- `backend/` — Go backend (API, ingestion, storage)
- `frontend/` — Web UI (React, coming soon)
- `mcp/` — MCP server for agent tools (coming soon)

## Development

```bash
cd backend
go build ./cmd/loghub
./loghub
```

Health check: `curl http://localhost:4800/healthz`
