# Architecture

AI Agent Log Hub is undergoing a stack migration. The full product specification,
including personas, user stories, concepts, design fragments, and requirements,
lives in the **autok knowledge graph** (project: "AI Agent Log Hub").

## New Stack

| Layer | Technology | Purpose |
|-------|-----------|---------|
| Database | SurrealDB | Multi-model: document + graph + vector |
| API Framework | [Loom](https://github.com/CaliLuke/loom) | Design-first Go API generation |
| MCP Server | [loom-mcp](https://github.com/CaliLuke/loom-mcp) | MCP tool generation from same DSL |
| Business Logic | Go services | Service -> Repository layering |
| Embeddings | HuggingFace TEI sidecar or API | Vector generation |
| Hooks | Enhanced Go binary | PostToolUse with causality tracking |
| Infra | Docker Compose | SurrealDB + backend + optional sidecar |

## Four-Phase Event Pipeline

Every piece of data flows through:

1. **Ingestion** (sync) — Event stored, session resolved, agent upserted
2. **Relationship Extraction** (sync) — Graph edges created from each event
3. **Background Enrichment** (async) — Trust promotion, git ingestion, embedding
4. **Serving Intelligence** (on query) — Hybrid SurrealQL: document + graph + vector

## Key Design Fragments (in autok)

- DES-1: System Architecture (Four-Phase Pipeline)
- DES-2: SurrealDB Graph Schema (8 records, 11 relations)
- DES-3: Technology Stack Decision
- DES-4: Loom API Design Structure
- DES-5: Background Workers Architecture
- DES-6: Docker Compose Infrastructure
- DES-7: Enhanced Hook System

## Directory Structure (target)

```
design/           <- Loom DSL (source of truth for API + MCP)
gen/              <- Generated code (HTTP, MCP, OpenAPI, types)
internal/
  service/        <- Business logic (hand-written)
  repository/     <- SurrealDB queries (hand-written)
  embed/          <- Embedding pipeline (hand-written)
  otlp/           <- OTLP receiver (hand-written, raw HTTP)
hooks/            <- Claude Code PostToolUse hook
```

## Replaced (removed)

- `backend/migrations/` — SurrealDB is schema-flexible
- `backend/internal/handler/` — Loom generates HTTP handlers
- `backend/internal/middleware/` — Loom generates middleware
- `backend/internal/repository/` — Rewriting for SurrealDB SDK
- `mcp/` — loom-mcp generates MCP server from same DSL
