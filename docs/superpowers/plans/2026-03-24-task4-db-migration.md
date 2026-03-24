# TASK-4: Initial Database Migration — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create the full database schema as a versioned migration that runs automatically on startup.

**Architecture:** Single up/down migration file pair using golang-migrate. Migration runner executes on startup before the HTTP server starts. All 9 tables created with indexes, triggers, and constraints per DES-35 through DES-44.

**Tech Stack:** PostgreSQL 16, pgvector, pg_trgm, golang-migrate/v4

**Spec:** TASK-4 from autok graph, DES-35 through DES-44

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `backend/migrations/001_initial_schema.up.sql` | Create | Full schema: extensions, 9 tables, indexes, triggers |
| `backend/migrations/001_initial_schema.down.sql` | Create | Drop all in reverse dependency order |
| `backend/cmd/loghub/main.go` | Modify | Add migration runner before pool creation |
| `backend/go.mod` | Modified by go get | Add golang-migrate/v4 |
| `backend/go.sum` | Modified by go get | Updated checksums |

---

### Task 1: Add golang-migrate dependency

**Files:**
- Modify: `backend/go.mod` (via go get)

- [ ] **Step 1: Add migrate dependency**

```bash
cd /Users/tobias/Documenti/personale_LOCALE/Auto-k-Frontend/Issue_155/testImplementations/ai-agent-log-hub/backend
go get -u github.com/golang-migrate/migrate/v4
go get github.com/golang-migrate/migrate/v4/database/postgres
go get github.com/golang-migrate/migrate/v4/source/file
```

- [ ] **Step 2: Commit**

```bash
cd /Users/tobias/Documenti/personale_LOCALE/Auto-k-Frontend/Issue_155/testImplementations/ai-agent-log-hub
git add backend/go.mod backend/go.sum
git commit -m "chore: add golang-migrate dependency"
```

---

### Task 2: Create up migration

**Files:**
- Create: `backend/migrations/001_initial_schema.up.sql`
- Remove: `backend/migrations/.gitkeep`

- [ ] **Step 1: Remove .gitkeep and create the up migration**

Create `backend/migrations/001_initial_schema.up.sql`:

```sql
-- 001_initial_schema.up.sql
-- Full database schema for AI Agent Log Hub
-- Ref: DES-35 through DES-44

-- ============================================================
-- Extensions
-- ============================================================
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "vector";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";

-- ============================================================
-- 1. agents (DES-36)
-- ============================================================
CREATE TABLE agents (
  agent_id        TEXT PRIMARY KEY,
  display_name    TEXT NOT NULL DEFAULT '',
  status          TEXT NOT NULL DEFAULT 'active'
    CHECK (status IN ('active', 'inactive')),
  first_seen      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_seen       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  total_sessions  INTEGER NOT NULL DEFAULT 0,
  total_events    BIGINT NOT NULL DEFAULT 0,
  metadata        JSONB NOT NULL DEFAULT '{}',
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE OR REPLACE FUNCTION set_agent_display_name()
RETURNS TRIGGER AS $$
BEGIN
  IF NEW.display_name = '' THEN
    NEW.display_name := NEW.agent_id;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_agent_display_name
  BEFORE INSERT ON agents
  FOR EACH ROW EXECUTE FUNCTION set_agent_display_name();

CREATE INDEX idx_agents_status ON agents (status);
CREATE INDEX idx_agents_last_seen ON agents (last_seen DESC);

-- ============================================================
-- 2. sessions (DES-37)
-- ============================================================
CREATE TABLE sessions (
  session_id      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  session_token   TEXT NOT NULL,
  agent_id        TEXT NOT NULL REFERENCES agents(agent_id),
  status          TEXT NOT NULL DEFAULT 'active'
    CHECK (status IN ('active', 'closed')),
  project_dir     TEXT,
  git_branch      TEXT,
  started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  ended_at        TIMESTAMPTZ,
  last_event_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  event_count     INTEGER NOT NULL DEFAULT 0,
  pinned          BOOLEAN NOT NULL DEFAULT FALSE,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_sessions_token ON sessions (agent_id, session_token) WHERE status = 'active';
CREATE INDEX idx_sessions_agent ON sessions (agent_id, started_at DESC);
CREATE INDEX idx_sessions_status ON sessions (status) WHERE status = 'active';
CREATE INDEX idx_sessions_project ON sessions (project_dir, started_at DESC);
CREATE INDEX idx_sessions_started ON sessions USING BRIN (started_at);
CREATE INDEX idx_sessions_pinned ON sessions (pinned) WHERE pinned = TRUE;

-- ============================================================
-- 3. agent_events (DES-38)
-- ============================================================
CREATE TABLE agent_events (
  event_id        UUID PRIMARY KEY,
  session_id      UUID NOT NULL REFERENCES sessions(session_id) ON DELETE CASCADE,
  agent_id        TEXT NOT NULL REFERENCES agents(agent_id),
  trace_id        TEXT,
  span_id         TEXT,
  parent_span_id  TEXT,
  timestamp       TIMESTAMPTZ NOT NULL,
  event_type      TEXT NOT NULL
    CHECK (event_type IN ('tool_call', 'explicit_log', 'git_commit', 'file_change', 'error')),
  severity        TEXT NOT NULL DEFAULT 'info'
    CHECK (severity IN ('debug', 'info', 'warn', 'error')),
  tool_name       TEXT,
  tool_type       TEXT
    CHECK (tool_type IS NULL OR tool_type IN ('builtin', 'mcp')),
  mcp_server      TEXT,
  message         TEXT,
  params          JSONB NOT NULL DEFAULT '{}',
  result          JSONB NOT NULL DEFAULT '{}',
  context         JSONB NOT NULL DEFAULT '{}',
  duration_ms     INTEGER,
  tags            TEXT[] DEFAULT '{}',
  spawned_by      UUID,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_agent_events_timestamp ON agent_events USING BRIN (timestamp);
CREATE INDEX idx_agent_events_session ON agent_events (session_id, timestamp);
CREATE INDEX idx_agent_events_agent ON agent_events (agent_id, timestamp DESC);
CREATE INDEX idx_agent_events_trace ON agent_events (trace_id) WHERE trace_id IS NOT NULL;
CREATE INDEX idx_agent_events_tool ON agent_events (tool_name);
CREATE INDEX idx_agent_events_type ON agent_events (event_type);
CREATE INDEX idx_agent_events_severity ON agent_events (severity) WHERE severity IN ('warn', 'error');
CREATE INDEX idx_agent_events_file ON agent_events USING GIN ((params->>'file_path') gin_trgm_ops)
  WHERE params->>'file_path' IS NOT NULL;
CREATE INDEX idx_agent_events_params ON agent_events USING GIN (params jsonb_path_ops);
CREATE INDEX idx_agent_events_result ON agent_events USING GIN (result jsonb_path_ops);
CREATE INDEX idx_agent_events_tags ON agent_events USING GIN (tags);
CREATE INDEX idx_agent_events_fts ON agent_events USING GIN (
  to_tsvector('english', COALESCE(message, '') || ' ' || COALESCE(params->>'file_path', '') || ' ' || COALESCE(params->>'command', ''))
);

-- ============================================================
-- 4. system_events (DES-39)
-- ============================================================
CREATE TABLE system_events (
  event_id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  timestamp         TIMESTAMPTZ NOT NULL,
  trace_id          TEXT,
  span_id           TEXT,
  parent_span_id    TEXT,
  source_type       TEXT NOT NULL
    CHECK (source_type IN ('otlp', 'http_api')),
  source_service    TEXT NOT NULL,
  severity          TEXT NOT NULL DEFAULT 'info'
    CHECK (severity IN ('debug', 'info', 'warn', 'error', 'fatal')),
  event_name        TEXT,
  message           TEXT,
  attributes        JSONB NOT NULL DEFAULT '{}',
  resource          JSONB NOT NULL DEFAULT '{}',
  duration_ms       INTEGER,
  session_id        UUID REFERENCES sessions(session_id),
  created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_system_events_timestamp ON system_events USING BRIN (timestamp);
CREATE INDEX idx_system_events_trace ON system_events (trace_id) WHERE trace_id IS NOT NULL;
CREATE INDEX idx_system_events_session ON system_events (session_id) WHERE session_id IS NOT NULL;
CREATE INDEX idx_system_events_severity ON system_events (severity) WHERE severity IN ('warn', 'error', 'fatal');
CREATE INDEX idx_system_events_service ON system_events (source_service, timestamp DESC);
CREATE INDEX idx_system_events_name ON system_events (event_name) WHERE event_name IS NOT NULL;
CREATE INDEX idx_system_events_attrs ON system_events USING GIN (attributes jsonb_path_ops);
CREATE INDEX idx_system_events_fts ON system_events USING GIN (
  to_tsvector('english', COALESCE(message, '') || ' ' || COALESCE(event_name, ''))
);

-- ============================================================
-- 5. session_summaries (DES-40)
-- ============================================================
CREATE TABLE session_summaries (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  session_id      UUID NOT NULL UNIQUE REFERENCES sessions(session_id) ON DELETE CASCADE,
  agent_id        TEXT NOT NULL REFERENCES agents(agent_id),
  duration_seconds INTEGER NOT NULL,
  event_count     INTEGER NOT NULL,
  files_modified  JSONB NOT NULL DEFAULT '[]',
  tools_used      JSONB NOT NULL DEFAULT '[]',
  commits         JSONB NOT NULL DEFAULT '[]',
  errors          JSONB NOT NULL DEFAULT '{"count": 0}',
  timeline        JSONB NOT NULL DEFAULT '[]',
  summary_text    TEXT,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_summaries_agent ON session_summaries (agent_id, created_at DESC);
CREATE INDEX idx_summaries_session ON session_summaries (session_id);

-- ============================================================
-- 6. memories (DES-41)
-- ============================================================
CREATE TABLE memories (
  memory_id       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  agent_id        TEXT NOT NULL REFERENCES agents(agent_id),
  session_id      UUID REFERENCES sessions(session_id),
  key             TEXT NOT NULL,
  value           TEXT NOT NULL,
  tags            TEXT[] DEFAULT '{}',
  shared          BOOLEAN NOT NULL DEFAULT FALSE,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT uq_memories_agent_key UNIQUE (agent_id, key)
);

CREATE INDEX idx_memories_agent ON memories (agent_id, updated_at DESC);
CREATE INDEX idx_memories_tags ON memories USING GIN (tags);
CREATE INDEX idx_memories_shared ON memories (shared) WHERE shared = TRUE;
CREATE INDEX idx_memories_key ON memories (agent_id, key);

-- ============================================================
-- 7. embeddings (DES-42)
-- ============================================================
CREATE TABLE embeddings (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  source_type     TEXT NOT NULL
    CHECK (source_type IN ('session_summary', 'explicit_log', 'error', 'memory', 'commit')),
  source_id       UUID NOT NULL,
  agent_id        TEXT NOT NULL REFERENCES agents(agent_id),
  content         TEXT NOT NULL,
  embedding       vector(384),
  shared          BOOLEAN NOT NULL DEFAULT FALSE,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_embeddings_hnsw ON embeddings
  USING hnsw (embedding vector_cosine_ops)
  WITH (m = 16, ef_construction = 64);
CREATE INDEX idx_embeddings_source ON embeddings (source_type, source_id);
CREATE INDEX idx_embeddings_agent ON embeddings (agent_id);
CREATE INDEX idx_embeddings_shared ON embeddings (shared) WHERE shared = TRUE;

-- ============================================================
-- 8. api_keys (DES-43)
-- ============================================================
CREATE TABLE api_keys (
  key_id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  key_hash        TEXT NOT NULL UNIQUE,
  key_prefix      TEXT NOT NULL,
  label           TEXT NOT NULL,
  scope           TEXT NOT NULL DEFAULT 'read_write'
    CHECK (scope IN ('read_only', 'read_write')),
  status          TEXT NOT NULL DEFAULT 'active'
    CHECK (status IN ('active', 'revoked')),
  last_used_at    TIMESTAMPTZ,
  revoked_at      TIMESTAMPTZ,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_by      TEXT
);

CREATE INDEX idx_api_keys_hash ON api_keys (key_hash) WHERE status = 'active';
CREATE INDEX idx_api_keys_label ON api_keys (label);
CREATE INDEX idx_api_keys_status ON api_keys (status);

-- ============================================================
-- 9. audit_log (DES-44)
-- ============================================================
CREATE TABLE audit_log (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  timestamp       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  action          TEXT NOT NULL,
  actor           TEXT NOT NULL,
  target_type     TEXT,
  target_id       TEXT,
  old_value       JSONB,
  new_value       JSONB,
  metadata        JSONB NOT NULL DEFAULT '{}',
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_timestamp ON audit_log USING BRIN (timestamp);
CREATE INDEX idx_audit_action ON audit_log (action);
CREATE INDEX idx_audit_target ON audit_log (target_type, target_id);
```

- [ ] **Step 2: Commit**

```bash
cd /Users/tobias/Documenti/personale_LOCALE/Auto-k-Frontend/Issue_155/testImplementations/ai-agent-log-hub
rm -f backend/migrations/.gitkeep
git add backend/migrations/001_initial_schema.up.sql
git rm -f backend/migrations/.gitkeep 2>/dev/null || true
git commit -m "feat: add initial database schema migration (up)"
```

---

### Task 3: Create down migration

**Files:**
- Create: `backend/migrations/001_initial_schema.down.sql`

- [ ] **Step 1: Create the down migration**

Create `backend/migrations/001_initial_schema.down.sql`:

```sql
-- 001_initial_schema.down.sql
-- Drop all tables in reverse dependency order

DROP TABLE IF EXISTS audit_log CASCADE;
DROP TABLE IF EXISTS api_keys CASCADE;
DROP TABLE IF EXISTS embeddings CASCADE;
DROP TABLE IF EXISTS memories CASCADE;
DROP TABLE IF EXISTS session_summaries CASCADE;
DROP TABLE IF EXISTS system_events CASCADE;
DROP TABLE IF EXISTS agent_events CASCADE;
DROP TABLE IF EXISTS sessions CASCADE;

DROP TRIGGER IF EXISTS trg_agent_display_name ON agents;
DROP FUNCTION IF EXISTS set_agent_display_name();

DROP TABLE IF EXISTS agents CASCADE;

DROP EXTENSION IF EXISTS "pg_trgm";
DROP EXTENSION IF EXISTS "vector";
DROP EXTENSION IF EXISTS "uuid-ossp";
```

- [ ] **Step 2: Commit**

```bash
cd /Users/tobias/Documenti/personale_LOCALE/Auto-k-Frontend/Issue_155/testImplementations/ai-agent-log-hub
git add backend/migrations/001_initial_schema.down.sql
git commit -m "feat: add initial database schema migration (down)"
```

---

### Task 4: Add migration runner to main.go

**Files:**
- Modify: `backend/cmd/loghub/main.go`

- [ ] **Step 1: Replace the entire content of `backend/cmd/loghub/main.go`**

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tobias/ai-agent-log-hub/backend/internal/config"
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

func runMigrations(dbURL string) error {
	migrationsPath := os.Getenv("MIGRATIONS_PATH")
	if migrationsPath == "" {
		migrationsPath = "migrations"
	}
	m, err := migrate.New("file://"+migrationsPath, dbURL)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("run migrations: %w", err)
	}

	version, dirty, _ := m.Version()
	slog.Info("migrations complete", "version", version, "dirty", dirty)
	return nil
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	for _, entry := range cfg.Describe() {
		if entry.Value != "" && entry.Value != entry.Default {
			slog.Info("config", "key", entry.Name, "value", entry.Value)
		}
	}

	if err := runMigrations(cfg.DatabaseURL); err != nil {
		slog.Error("migration failed", "error", err)
		os.Exit(1)
	}

	var pool *pgxpool.Pool
	pool, err = pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to create connection pool", "error", err)
	} else {
		defer pool.Close()
		slog.Info("database connection pool created")
	}

	r := chi.NewRouter()
	r.Get("/healthz", makeHealthzHandler(pool))

	slog.Info("starting server", "port", cfg.APIPort)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", cfg.APIPort), r))
}
```

- [ ] **Step 2: Run unit tests (no DB needed)**

```bash
cd /Users/tobias/Documenti/personale_LOCALE/Auto-k-Frontend/Issue_155/testImplementations/ai-agent-log-hub/backend
go test ./... -v
```

Expected: All existing tests pass (healthz test uses nil pool, config tests mock env vars).

- [ ] **Step 3: Verify build**

```bash
cd /Users/tobias/Documenti/personale_LOCALE/Auto-k-Frontend/Issue_155/testImplementations/ai-agent-log-hub/backend
go build ./cmd/loghub && rm loghub
```

- [ ] **Step 4: Update docker-compose.yml to set MIGRATIONS_PATH**

Add `MIGRATIONS_PATH: /migrations` to the backend environment in `docker-compose.yml` (the Dockerfile copies migrations to `/migrations/` but locally they're at `./migrations` relative to the binary).

- [ ] **Step 5: Commit**

```bash
cd /Users/tobias/Documenti/personale_LOCALE/Auto-k-Frontend/Issue_155/testImplementations/ai-agent-log-hub
git add backend/cmd/loghub/main.go backend/go.mod backend/go.sum docker-compose.yml
git commit -m "feat: add migration runner to startup"
```

---

### Task 5: Integration test with Docker Compose

- [ ] **Step 1: Start the stack with alternate ports**

```bash
cd /Users/tobias/Documenti/personale_LOCALE/Auto-k-Frontend/Issue_155/testImplementations/ai-agent-log-hub

cat > /tmp/loghub-override.yml <<'EOF'
services:
  db:
    ports:
      !override
      - '15432:5432'
  backend:
    ports:
      !override
      - '14800:4800'
EOF

docker compose -p loghub-test -f docker-compose.yml -f /tmp/loghub-override.yml up --build -d
```

Expected: Both services start, backend runs migrations on startup.

- [ ] **Step 2: Wait and test healthz**

```bash
sleep 15
curl -s http://localhost:14800/healthz | jq .
```

Expected:
```json
{
  "status": "ok",
  "db": "connected"
}
```

- [ ] **Step 3: Verify all 9 tables exist**

```bash
docker exec loghub-test-db-1 psql -U loghub -d loghub -c "\dt" | grep -c "public"
```

Expected: 10 rows (9 tables + 1 schema_migrations table from golang-migrate).

- [ ] **Step 4: Verify key tables have correct structure**

```bash
docker exec loghub-test-db-1 psql -U loghub -d loghub -c "\d agents"
docker exec loghub-test-db-1 psql -U loghub -d loghub -c "\d agent_events"
docker exec loghub-test-db-1 psql -U loghub -d loghub -c "\d embeddings"
```

- [ ] **Step 5: Verify extensions**

```bash
docker exec loghub-test-db-1 psql -U loghub -d loghub -c "\dx"
```

Expected: `uuid-ossp`, `vector`, `pg_trgm` listed.

- [ ] **Step 6: Test clean recreate**

```bash
docker compose -p loghub-test -f docker-compose.yml -f /tmp/loghub-override.yml down -v
docker compose -p loghub-test -f docker-compose.yml -f /tmp/loghub-override.yml up --build -d
sleep 15
curl -s http://localhost:14800/healthz | jq .
```

Expected: Full stack recreates cleanly, healthz returns connected.

- [ ] **Step 7: Tear down and clean up**

```bash
docker compose -p loghub-test -f docker-compose.yml -f /tmp/loghub-override.yml down -v
rm /tmp/loghub-override.yml
```

- [ ] **Step 8: Commit docs**

```bash
cd /Users/tobias/Documenti/personale_LOCALE/Auto-k-Frontend/Issue_155/testImplementations/ai-agent-log-hub
git add docs/superpowers/plans/2026-03-24-task4-db-migration.md
git commit -m "docs: add TASK-4 implementation plan"
```

---

## Acceptance Checklist

- [ ] `docker compose up` creates all 9 tables with all indexes and constraints
- [ ] `\dt` in psql shows all tables (agents, sessions, agent_events, system_events, session_summaries, memories, embeddings, api_keys, audit_log)
- [ ] Extensions uuid-ossp, vector, pg_trgm are installed
- [ ] `docker compose down -v && docker compose up` recreates cleanly from scratch
- [ ] Agent display_name trigger works (INSERT with empty display_name sets it to agent_id)
- [ ] `go test ./... -v` still passes (no DB-dependent tests broken)
- [ ] Migration runner logs version on startup
