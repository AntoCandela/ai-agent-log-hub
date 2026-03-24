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
