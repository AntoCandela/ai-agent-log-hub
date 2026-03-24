# TASK-1: Initialize Go Module and Repo Structure — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create the monorepo skeleton with a working Go backend that serves `/healthz`.

**Architecture:** Single Go binary using chi router, structured logging via slog. Monorepo with `backend/`, `frontend/`, `mcp/` top-level dirs. Backend follows standard Go layout with `cmd/` for entrypoints and `internal/` for private packages.

**Tech Stack:** Go 1.23, chi/v5 router, slog (stdlib)

**Spec:** `docs/superpowers/specs/2026-03-24-task1-init-repo-design.md`

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `.gitignore` | Create | Exclude binaries, env files, IDE files, OS files |
| `.env.example` | Create | Document all env vars with defaults |
| `README.md` | Create | Project overview and quick start |
| `docker-compose.yml` | Create | Placeholder for TASK-2 |
| `frontend/.gitkeep` | Create | Placeholder dir |
| `mcp/.gitkeep` | Create | Placeholder dir |
| `backend/go.mod` | Create | Go module definition |
| `backend/go.sum` | Generated | Dependency checksums |
| `backend/cmd/loghub/main.go` | Create | HTTP server entrypoint with `/healthz` |
| `backend/internal/config/.gitkeep` | Create | Placeholder for TASK-3 |
| `backend/internal/handler/.gitkeep` | Create | Placeholder for TASK-8 |
| `backend/internal/service/.gitkeep` | Create | Placeholder for TASK-6/7 |
| `backend/internal/repository/.gitkeep` | Create | Placeholder for TASK-6/7/9/11 |
| `backend/internal/middleware/.gitkeep` | Create | Placeholder for TASK-5 |
| `backend/internal/model/.gitkeep` | Create | Placeholder for TASK-6/7 |
| `backend/internal/embed/.gitkeep` | Create | Placeholder for embedding service |
| `backend/internal/otlp/.gitkeep` | Create | Placeholder for TASK-10 |
| `backend/internal/ws/.gitkeep` | Create | Placeholder for WebSocket service |
| `backend/migrations/.gitkeep` | Create | Placeholder for TASK-4 |

---

### Task 1: Initialize git and create root project files

**Files:**
- Create: `.gitignore`
- Create: `.env.example`
- Create: `README.md`
- Create: `docker-compose.yml`
- Create: `frontend/.gitkeep`
- Create: `mcp/.gitkeep`

- [ ] **Step 1: Initialize git repo**

```bash
cd /Users/tobias/Documenti/personale_LOCALE/Auto-k-Frontend/Issue_155/testImplementations/ai-agent-log-hub
git init
```

- [ ] **Step 2: Create .gitignore**

```gitignore
# Go
*.exe
*.exe~
*.dll
*.so
*.dylib
/backend/cmd/loghub/loghub
/backend/loghub

# Test
*.test
*.out
coverage.html

# Environment
.env
.env.local
.env.*.local

# Database
*.sqlite
*.sqlite3
pgdata/

# Dependencies
node_modules/
vendor/

# IDE
.idea/
.vscode/
*.swp
*.swo
*~

# OS
.DS_Store
Thumbs.db
```

- [ ] **Step 3: Create .env.example**

```env
# Server
API_PORT=4800
LOG_LEVEL=info

# Database
DATABASE_URL=postgres://loghub:loghub_dev@localhost:5432/loghub?sslmode=disable
MAX_DB_CONNS=20

# Auth (disabled by default for local dev)
AUTH_ENABLED=false
SESSION_TIMEOUT=24h

# TLS (disabled by default for local dev)
TLS_ENABLED=false
TLS_CERT_PATH=
TLS_KEY_PATH=

# Agent Sessions
AGENT_SESSION_TIMEOUT=30m

# Retention
RETENTION_DAYS=90

# Embedding
EMBEDDING_BACKEND=local
EMBEDDING_MODEL=all-MiniLM-L6-v2
EMBEDDING_API_URL=
EMBEDDING_API_KEY=
EMBEDDING_DIMENSIONS=384

# Frontend
FRONTEND_URL=http://localhost:4801
```

- [ ] **Step 4: Create README.md**

```markdown
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
```

- [ ] **Step 5: Create docker-compose.yml placeholder**

```yaml
# AI Agent Log Hub — Docker Compose
# TASK-2 will populate this with db and backend services.
# Quick start: docker compose up
```

- [ ] **Step 6: Create placeholder directories**

```bash
mkdir -p frontend mcp
touch frontend/.gitkeep mcp/.gitkeep
```

- [ ] **Step 7: Commit**

```bash
git add .gitignore .env.example README.md docker-compose.yml frontend/.gitkeep mcp/.gitkeep
git commit -m "chore: add root project files — gitignore, env example, readme, placeholders"
```

---

### Task 2: Initialize Go module and create backend skeleton

**Files:**
- Create: `backend/go.mod`
- Create: `backend/internal/config/.gitkeep`
- Create: `backend/internal/handler/.gitkeep`
- Create: `backend/internal/service/.gitkeep`
- Create: `backend/internal/repository/.gitkeep`
- Create: `backend/internal/middleware/.gitkeep`
- Create: `backend/internal/model/.gitkeep`
- Create: `backend/internal/embed/.gitkeep`
- Create: `backend/internal/otlp/.gitkeep`
- Create: `backend/internal/ws/.gitkeep`
- Create: `backend/migrations/.gitkeep`

- [ ] **Step 1: Initialize Go module**

```bash
cd /Users/tobias/Documenti/personale_LOCALE/Auto-k-Frontend/Issue_155/testImplementations/ai-agent-log-hub/backend
go mod init github.com/tobias/ai-agent-log-hub/backend
```

- [ ] **Step 2: Add chi dependency**

```bash
cd /Users/tobias/Documenti/personale_LOCALE/Auto-k-Frontend/Issue_155/testImplementations/ai-agent-log-hub/backend
go get github.com/go-chi/chi/v5
```

- [ ] **Step 3: Create internal package directories and migrations**

```bash
cd /Users/tobias/Documenti/personale_LOCALE/Auto-k-Frontend/Issue_155/testImplementations/ai-agent-log-hub/backend
mkdir -p internal/config internal/handler internal/service internal/repository internal/middleware internal/model internal/embed internal/otlp internal/ws migrations
touch internal/config/.gitkeep internal/handler/.gitkeep internal/service/.gitkeep internal/repository/.gitkeep internal/middleware/.gitkeep internal/model/.gitkeep internal/embed/.gitkeep internal/otlp/.gitkeep internal/ws/.gitkeep migrations/.gitkeep
```

- [ ] **Step 4: Commit**

```bash
cd /Users/tobias/Documenti/personale_LOCALE/Auto-k-Frontend/Issue_155/testImplementations/ai-agent-log-hub
git add backend/go.mod backend/go.sum backend/internal/ backend/migrations/
git commit -m "chore: initialize Go module and backend skeleton with internal packages"
```

---

### Task 3: Write healthz test

**Files:**
- Create: `backend/cmd/loghub/main_test.go`

- [ ] **Step 1: Write the failing test**

Create `backend/cmd/loghub/main_test.go`:

```go
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func newRouter() *chi.Mux {
	r := chi.NewRouter()
	r.Get("/healthz", healthzHandler)
	return r
}

func TestHealthz_ReturnsOKStatus(t *testing.T) {
	srv := httptest.NewServer(newRouter())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if body["status"] != "ok" {
		t.Errorf("expected status \"ok\", got %q", body["status"])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/tobias/Documenti/personale_LOCALE/Auto-k-Frontend/Issue_155/testImplementations/ai-agent-log-hub/backend
go test ./cmd/loghub/ -v
```

Expected: FAIL — `healthzHandler` is not defined.

- [ ] **Step 3: Commit failing test**

```bash
cd /Users/tobias/Documenti/personale_LOCALE/Auto-k-Frontend/Issue_155/testImplementations/ai-agent-log-hub
git add backend/cmd/loghub/main_test.go
git commit -m "test: add healthz endpoint test (red)"
```

---

### Task 4: Implement main.go to pass the test

**Files:**
- Create: `backend/cmd/loghub/main.go`

- [ ] **Step 1: Create main.go**

```go
package main

import (
	"encoding/json"
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
)

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	r := chi.NewRouter()
	r.Get("/healthz", healthzHandler)

	slog.Info("starting server", "port", 4800)
	log.Fatal(http.ListenAndServe(":4800", r))
}
```

- [ ] **Step 2: Run test to verify it passes**

```bash
cd /Users/tobias/Documenti/personale_LOCALE/Auto-k-Frontend/Issue_155/testImplementations/ai-agent-log-hub/backend
go test ./cmd/loghub/ -v
```

Expected: PASS — `TestHealthz_ReturnsOKStatus` passes.

- [ ] **Step 3: Verify build**

```bash
cd /Users/tobias/Documenti/personale_LOCALE/Auto-k-Frontend/Issue_155/testImplementations/ai-agent-log-hub/backend
go build ./cmd/loghub
```

Expected: Produces `loghub` binary with no errors.

- [ ] **Step 4: Commit**

```bash
cd /Users/tobias/Documenti/personale_LOCALE/Auto-k-Frontend/Issue_155/testImplementations/ai-agent-log-hub
git add backend/cmd/loghub/main.go
git commit -m "feat: add healthz endpoint with chi router and slog"
```

---

### Task 5: Manual smoke test and cleanup

- [ ] **Step 1: Start server**

```bash
cd /Users/tobias/Documenti/personale_LOCALE/Auto-k-Frontend/Issue_155/testImplementations/ai-agent-log-hub/backend
./loghub &
```

Expected: JSON log line `{"level":"INFO","msg":"starting server","port":4800}`

- [ ] **Step 2: Curl healthz**

```bash
curl -s http://localhost:4800/healthz | jq .
```

Expected:
```json
{
  "status": "ok"
}
```

- [ ] **Step 3: Stop server and clean up binary**

```bash
kill %1
rm /Users/tobias/Documenti/personale_LOCALE/Auto-k-Frontend/Issue_155/testImplementations/ai-agent-log-hub/backend/loghub
```

- [ ] **Step 4: Final commit with docs**

```bash
cd /Users/tobias/Documenti/personale_LOCALE/Auto-k-Frontend/Issue_155/testImplementations/ai-agent-log-hub
git add docs/
git commit -m "docs: add TASK-1 spec and implementation plan"
```

---

## Acceptance Checklist

- [ ] `cd backend && go build ./cmd/loghub` succeeds
- [ ] Binary starts and listens on :4800
- [ ] `curl localhost:4800/healthz` returns `{"status":"ok"}`
- [ ] `go test ./cmd/loghub/ -v` passes
- [ ] Git repo initialized with clean commit history
