// TOCHANGE: Stack migration to SurrealDB + Loom + loom-mcp
// - Aggregation logic stays (files, tools, commits, errors, timeline)
// - Embedding stored as native vector field on session record (not separate table)
// - Add: relationship extraction during summary (file co-modification edges)
// - Add: tool call composition pattern detection
// - See autok design fragment DES-5 (Background Workers) for embedding generator spec
//
// NOTE: This file is temporarily stubbed. The full aggregation logic was in the
// previous version (see git history). It will be rewritten with SurrealDB queries
// that leverage native graph traversal for richer aggregation.
package service
