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
