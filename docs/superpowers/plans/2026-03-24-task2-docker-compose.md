# TASK-2: Docker Compose Stack — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Dockerize the backend and PostgreSQL so `docker compose up` boots the full stack with DB connectivity.

**Architecture:** Multi-stage Docker build for the Go backend. PostgreSQL 16 with pgvector extension. Backend connects via pgxpool, healthz endpoint reports DB status.

**Tech Stack:** Docker Compose, PostgreSQL 16 + pgvector, pgx/v5 (pgxpool), multi-stage Dockerfile

**Spec:** TASK-2 from autok graph + `docs/superpowers/specs/2026-03-24-task1-init-repo-design.md` (base)

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `docker-compose.yml` | Modify | Define db and backend services |
| `backend/Dockerfile` | Create | Multi-stage Go build |
| `backend/cmd/loghub/main.go` | Modify | Add pgxpool connection, pass pool to healthz |
| `backend/cmd/loghub/main_test.go` | Modify | Update test for new healthz signature |
| `backend/go.mod` | Modified by go get | Add pgx/v5 dependency |
| `backend/go.sum` | Modified by go get | Updated checksums |

---

### Task 1: Add pgx dependency and update healthz test (red)

**Files:**
- Modify: `backend/go.mod` (via go get)
- Modify: `backend/cmd/loghub/main_test.go`

- [ ] **Step 1: Add pgx dependency**

```bash
cd /Users/tobias/Documenti/personale_LOCALE/Auto-k-Frontend/Issue_155/testImplementations/ai-agent-log-hub/backend
go get github.com/jackc/pgx/v5
```

- [ ] **Step 2: Replace test file for new healthz response shape**

Replace the entire content of `backend/cmd/loghub/main_test.go` — the healthz handler will now accept a `*pgxpool.Pool` parameter (nil-safe for tests). The test verifies the new response shape `{"status":"ok","db":"disconnected"}` when pool is nil, and keeps testing status code + content-type:

```go
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func newTestRouter() *chi.Mux {
	r := chi.NewRouter()
	r.Get("/healthz", makeHealthzHandler(nil))
	return r
}

func TestHealthz_ReturnsOKStatus(t *testing.T) {
	srv := httptest.NewServer(newTestRouter())
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

	if body["db"] != "disconnected" {
		t.Errorf("expected db \"disconnected\" when pool is nil, got %q", body["db"])
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

```bash
cd /Users/tobias/Documenti/personale_LOCALE/Auto-k-Frontend/Issue_155/testImplementations/ai-agent-log-hub/backend
go test ./cmd/loghub/ -v
```

Expected: FAIL — `makeHealthzHandler` is not defined (old `healthzHandler` still exists).

- [ ] **Step 4: Commit failing test**

```bash
cd /Users/tobias/Documenti/personale_LOCALE/Auto-k-Frontend/Issue_155/testImplementations/ai-agent-log-hub
git add backend/cmd/loghub/main_test.go backend/go.mod backend/go.sum
git commit -m "test: update healthz test for DB-aware response (red)"
```

---

### Task 2: Update main.go with pgxpool and new healthz (green)

**Files:**
- Modify: `backend/cmd/loghub/main.go`

- [ ] **Step 1: Update main.go**

Replace `backend/cmd/loghub/main.go` with:

```go
package main

import (
	"context"
	"encoding/json"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func makeHealthzHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		dbStatus := "disconnected"
		if pool != nil {
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			if err := pool.Ping(ctx); err == nil {
				dbStatus = "connected"
			}
		}

		json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
			"db":     dbStatus,
		})
	}
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	var pool *pgxpool.Pool
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL != "" {
		var err error
		pool, err = pgxpool.New(context.Background(), dbURL)
		if err != nil {
			slog.Error("failed to create connection pool", "error", err)
		} else {
			defer pool.Close()
			slog.Info("database connection pool created")
		}
	} else {
		slog.Warn("DATABASE_URL not set, running without database")
	}

	r := chi.NewRouter()
	r.Get("/healthz", makeHealthzHandler(pool))

	port := os.Getenv("API_PORT")
	if port == "" {
		port = "4800"
	}

	slog.Info("starting server", "port", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
```

- [ ] **Step 2: Run test to verify it passes**

```bash
cd /Users/tobias/Documenti/personale_LOCALE/Auto-k-Frontend/Issue_155/testImplementations/ai-agent-log-hub/backend
go test ./cmd/loghub/ -v
```

Expected: PASS — `TestHealthz_ReturnsOKStatus` passes (nil pool → db=disconnected).

- [ ] **Step 3: Verify build**

```bash
cd /Users/tobias/Documenti/personale_LOCALE/Auto-k-Frontend/Issue_155/testImplementations/ai-agent-log-hub/backend
go build ./cmd/loghub
```

Expected: Builds successfully.

- [ ] **Step 4: Commit**

```bash
cd /Users/tobias/Documenti/personale_LOCALE/Auto-k-Frontend/Issue_155/testImplementations/ai-agent-log-hub
git add backend/cmd/loghub/main.go backend/go.mod backend/go.sum
git commit -m "feat: add pgxpool connection and DB-aware healthz"
```

---

### Task 3: Create Dockerfile

**Files:**
- Create: `backend/Dockerfile`

- [ ] **Step 1: Create multi-stage Dockerfile**

Create `backend/Dockerfile`:

```dockerfile
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /loghub ./cmd/loghub

FROM alpine:3.19
COPY --from=builder /loghub /usr/local/bin/loghub
COPY migrations/ /migrations/
EXPOSE 4800
CMD ["loghub"]
```

- [ ] **Step 2: Commit**

```bash
cd /Users/tobias/Documenti/personale_LOCALE/Auto-k-Frontend/Issue_155/testImplementations/ai-agent-log-hub
git add backend/Dockerfile
git commit -m "chore: add multi-stage Dockerfile for backend"
```

---

### Task 4: Update docker-compose.yml

**Files:**
- Modify: `docker-compose.yml`

- [ ] **Step 1: Overwrite the entire docker-compose.yml with full config**

```yaml
services:
  db:
    image: pgvector/pgvector:pg16
    environment:
      POSTGRES_USER: loghub
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-loghub_dev}
      POSTGRES_DB: loghub
    volumes:
      - pgdata:/var/lib/postgresql/data
    ports:
      - '5432:5432'
    healthcheck:
      test: pg_isready -U loghub
      interval: 5s
      retries: 5

  backend:
    build:
      context: ./backend
      dockerfile: Dockerfile
    depends_on:
      db:
        condition: service_healthy
    environment:
      DATABASE_URL: postgres://loghub:${POSTGRES_PASSWORD:-loghub_dev}@db:5432/loghub?sslmode=disable
      API_PORT: '4800'
    ports:
      - '4800:4800'

volumes:
  pgdata:
```

- [ ] **Step 2: Commit**

```bash
cd /Users/tobias/Documenti/personale_LOCALE/Auto-k-Frontend/Issue_155/testImplementations/ai-agent-log-hub
git add docker-compose.yml
git commit -m "feat: add Docker Compose stack with PostgreSQL and backend"
```

---

### Task 5: Integration test with Docker Compose

- [ ] **Step 1: Build and start the stack**

```bash
cd /Users/tobias/Documenti/personale_LOCALE/Auto-k-Frontend/Issue_155/testImplementations/ai-agent-log-hub
docker compose up --build -d
```

Expected: Both `db` and `backend` services start. Backend waits for DB healthcheck.

- [ ] **Step 2: Wait for services and test healthz**

```bash
sleep 10
curl -s http://localhost:4800/healthz | jq .
```

Expected:
```json
{
  "status": "ok",
  "db": "connected"
}
```

- [ ] **Step 3: Verify clean shutdown**

```bash
docker compose down
```

Expected: Both services stop cleanly, no errors.

- [ ] **Step 4: Verify volume cleanup**

```bash
docker compose down -v
```

Expected: Volumes removed cleanly.

- [ ] **Step 5: Commit docs**

```bash
cd /Users/tobias/Documenti/personale_LOCALE/Auto-k-Frontend/Issue_155/testImplementations/ai-agent-log-hub
git add docs/superpowers/plans/2026-03-24-task2-docker-compose.md
git commit -m "docs: add TASK-2 implementation plan"
```

---

## Acceptance Checklist

- [ ] `docker compose up` starts both db and backend services
- [ ] `curl localhost:4800/healthz` returns `{"status":"ok","db":"connected"}`
- [ ] `docker compose down` stops cleanly
- [ ] `docker compose down -v` removes volumes
- [ ] `go test ./cmd/loghub/ -v` still passes (without DB → db=disconnected)
- [ ] Backend builds successfully in Docker
