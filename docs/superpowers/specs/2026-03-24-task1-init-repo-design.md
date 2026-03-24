# TASK-1: Initialize Go Module and Repo Structure

## Purpose

Create the monorepo skeleton so all subsequent MS-1 tasks have a home. This is the foundation for the AI Agent Log Hub backend.

## Repo Structure

```
ai-agent-log-hub/
├── backend/
│   ├── cmd/loghub/main.go
│   ├── internal/
│   │   ├── config/
│   │   ├── handler/
│   │   ├── service/
│   │   ├── repository/
│   │   ├── middleware/
│   │   ├── model/
│   │   ├── embed/
│   │   ├── otlp/
│   │   └── ws/
│   ├── migrations/
│   ├── go.mod
│   └── go.sum
├── frontend/
│   └── .gitkeep
├── mcp/
│   └── .gitkeep
├── docker-compose.yml
├── .gitignore
├── .env.example
└── README.md
```

## Go Module

- **Path**: `github.com/tobias/ai-agent-log-hub/backend`
- **Go version**: 1.23
- **Dependencies**: `github.com/go-chi/chi/v5` (router only; pgx and migrate added in later tasks when needed)

## main.go

Minimal chi server on port 4800 with a `/healthz` endpoint:

```go
func main() {
    logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
    slog.SetDefault(logger)

    r := chi.NewRouter()
    r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
    })

    slog.Info("starting server", "port", 4800)
    log.Fatal(http.ListenAndServe(":4800", r))
}
```

## .gitignore

Go binaries, `node_modules`, `.env`, `*.sqlite`, `pgdata/`, IDE files (`.idea/`, `.vscode/`), OS files (`.DS_Store`), test coverage output.

## .env.example

All env vars from the config spec (TASK-3) with defaults and comments:
- `API_PORT`, `LOG_LEVEL`, `DATABASE_URL`, `MAX_DB_CONNS`
- `AUTH_ENABLED`, `SESSION_TIMEOUT`, `TLS_ENABLED`, `TLS_CERT_PATH`, `TLS_KEY_PATH`
- `AGENT_SESSION_TIMEOUT`, `RETENTION_DAYS`
- `EMBEDDING_BACKEND`, `EMBEDDING_MODEL`, `EMBEDDING_API_URL`, `EMBEDDING_API_KEY`, `EMBEDDING_DIMENSIONS`
- `FRONTEND_URL`

## docker-compose.yml

Placeholder with a comment indicating TASK-2 will fill it.

## README.md

Project name, one-line description, quick start (`docker compose up`).

## Done When

1. `cd backend && go build ./cmd/loghub` succeeds
2. Binary starts and listens on :4800
3. `curl localhost:4800/healthz` returns `{"status":"ok"}`
4. Git repo initialized with initial commit
