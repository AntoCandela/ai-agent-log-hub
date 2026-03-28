package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AgentEventRepo provides database access for agent event records.
// Agent events are the primary telemetry emitted by AI agents (tool calls,
// messages, errors, etc.).
type AgentEventRepo struct {
	pool *pgxpool.Pool
}

// NewAgentEventRepo creates a new AgentEventRepo backed by the given pool.
func NewAgentEventRepo(pool *pgxpool.Pool) *AgentEventRepo {
	return &AgentEventRepo{pool: pool}
}

// InsertBatch inserts multiple agent events in a single database round-trip
// using pgx's Batch API. Each event uses "ON CONFLICT (event_id) DO NOTHING"
// to silently skip duplicates — if a client retries a request, the same
// event_id will not produce a second row. This is called "idempotent insert
// with deduplication".
//
// How pgx.Batch works: instead of sending N separate INSERT statements over
// the network, Batch queues them all up and sends them to PostgreSQL in one
// network round-trip. The database executes each statement and returns N
// results. This is much faster than N individual queries.
//
// Returns the number of newly accepted rows and the number of duplicates
// (events that were already in the table).
func (r *AgentEventRepo) InsertBatch(ctx context.Context, events []model.AgentEvent) (accepted, duplicates int, err error) {
	batch := &pgx.Batch{}
	for _, e := range events {
		// Ensure nullable JSON/array fields default to valid empty values so that
		// PostgreSQL does not receive NULL where it expects a JSON object or array.
		tags := e.Tags
		if tags == nil {
			tags = []string{}
		}
		params := e.Params
		if params == nil {
			params = []byte("{}")
		}
		result := e.Result
		if result == nil {
			result = []byte("{}")
		}
		evtCtx := e.Context
		if evtCtx == nil {
			evtCtx = []byte("{}")
		}

		// Queue an INSERT for this event. ON CONFLICT (event_id) DO NOTHING
		// means: if this event_id already exists in the table, silently skip
		// the insert rather than returning an error.
		batch.Queue(
			`INSERT INTO agent_events (event_id, session_id, agent_id, trace_id, span_id, parent_span_id,
			 timestamp, event_type, severity, tool_name, tool_type, mcp_server, message,
			 params, result, context, duration_ms, tags, spawned_by)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)
			 ON CONFLICT (event_id) DO NOTHING`,
			e.EventID, e.SessionID, e.AgentID, e.TraceID, e.SpanID, e.ParentSpanID,
			e.Timestamp, e.EventType, e.Severity, e.ToolName, e.ToolType, e.MCPServer, e.Message,
			params, result, evtCtx, e.DurationMs, tags, e.SpawnedBy,
		)
	}

	// SendBatch sends all queued statements to PostgreSQL in one network call.
	results := r.pool.SendBatch(ctx, batch)
	defer results.Close() //nolint:errcheck // batch close error is not actionable

	// Read back the result of each INSERT. RowsAffected() returns 1 for a new
	// insert and 0 when the ON CONFLICT clause skipped a duplicate.
	accepted = 0
	for range events {
		ct, err := results.Exec()
		if err != nil {
			return 0, 0, fmt.Errorf("insert event: %w", err)
		}
		if ct.RowsAffected() > 0 {
			accepted++
		}
	}
	duplicates = len(events) - accepted
	return accepted, duplicates, nil
}

// EventFilters holds optional filter criteria for querying agent events.
// Every pointer field is optional: nil means "don't filter on this field".
// This struct is used with the composable WHERE builder in Query().
type EventFilters struct {
	SessionID *uuid.UUID // Filter to events belonging to this session.
	AgentID   *string    // Filter to events from this agent.
	ToolName  *string    // Filter to events involving this tool (e.g. "Read", "Edit").
	EventType *string    // Filter by event type (e.g. "tool_call", "git_commit").
	Severity  *string    // Filter by severity level (e.g. "error", "info").
	FilePath  *string    // Filter events whose params.file_path matches this glob pattern.
	TraceID   *string    // Filter to events sharing this distributed trace ID.
	Text      *string    // Full-text search across message, file_path, and command fields.
	Since     *time.Time // Only events at or after this timestamp.
	Until     *time.Time // Only events at or before this timestamp.
	Tags      []string   // Filter events whose tags overlap with any of these (PostgreSQL "&&" array overlap operator).
	Limit     int        // Maximum number of results (default 50, max 1000).
	Offset    int        // Number of results to skip for pagination.
	Order     string     // Sort direction: "asc" or "desc" (default "desc").
}

// Query returns agent events matching the given filters, plus the total count
// of matching rows (for pagination). It uses the same composable WHERE builder
// pattern as SessionRepo.List — each non-nil filter field appends a SQL
// condition and a parameter value, then all conditions are joined with AND.
//
// Notable filter implementations:
//   - FilePath uses PostgreSQL LIKE with glob-to-LIKE conversion (* -> %, ** -> %).
//   - Text uses PostgreSQL full-text search: to_tsvector / plainto_tsquery.
//     This converts the message and key params fields into a searchable "text
//     vector" and matches the user's search query against it, supporting
//     stemming and language-aware word matching.
//   - Tags uses the PostgreSQL array overlap operator (&&) which returns true
//     when the stored tags array shares at least one element with the filter.
func (r *AgentEventRepo) Query(ctx context.Context, filters EventFilters) ([]model.AgentEvent, int, error) {
	// Build WHERE clause dynamically from filters (composable WHERE builder).
	var conditions []string
	var args []any
	argIdx := 1

	if filters.SessionID != nil {
		conditions = append(conditions, fmt.Sprintf("session_id = $%d", argIdx))
		args = append(args, *filters.SessionID)
		argIdx++
	}
	if filters.AgentID != nil {
		conditions = append(conditions, fmt.Sprintf("agent_id = $%d", argIdx))
		args = append(args, *filters.AgentID)
		argIdx++
	}
	if filters.ToolName != nil {
		conditions = append(conditions, fmt.Sprintf("tool_name = $%d", argIdx))
		args = append(args, *filters.ToolName)
		argIdx++
	}
	if filters.EventType != nil {
		conditions = append(conditions, fmt.Sprintf("event_type = $%d", argIdx))
		args = append(args, *filters.EventType)
		argIdx++
	}
	if filters.Severity != nil {
		conditions = append(conditions, fmt.Sprintf("severity = $%d", argIdx))
		args = append(args, *filters.Severity)
		argIdx++
	}
	if filters.TraceID != nil {
		conditions = append(conditions, fmt.Sprintf("trace_id = $%d", argIdx))
		args = append(args, *filters.TraceID)
		argIdx++
	}
	if filters.FilePath != nil {
		// Convert glob to LIKE pattern
		like := strings.ReplaceAll(*filters.FilePath, "**", "%")
		like = strings.ReplaceAll(like, "*", "%")
		conditions = append(conditions, fmt.Sprintf("params->>'file_path' LIKE $%d", argIdx))
		args = append(args, like)
		argIdx++
	}
	if filters.Text != nil {
		conditions = append(conditions, fmt.Sprintf(
			"to_tsvector('english', COALESCE(message,'') || ' ' || COALESCE(params->>'file_path','') || ' ' || COALESCE(params->>'command','')) @@ plainto_tsquery('english', $%d)", argIdx))
		args = append(args, *filters.Text)
		argIdx++
	}
	if filters.Since != nil {
		conditions = append(conditions, fmt.Sprintf("timestamp >= $%d", argIdx))
		args = append(args, *filters.Since)
		argIdx++
	}
	if filters.Until != nil {
		conditions = append(conditions, fmt.Sprintf("timestamp <= $%d", argIdx))
		args = append(args, *filters.Until)
		argIdx++
	}
	if len(filters.Tags) > 0 {
		conditions = append(conditions, fmt.Sprintf("tags && $%d", argIdx))
		args = append(args, filters.Tags)
		argIdx++
	}

	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	// Count
	var total int
	err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM agent_events"+where, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count events: %w", err)
	}

	// Defaults
	limit := filters.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 1000 {
		limit = 1000
	}
	order := "DESC"
	if filters.Order == "asc" {
		order = "ASC"
	}

	listArgs := append(append([]any{}, args...), limit, filters.Offset)
	query := fmt.Sprintf(
		`SELECT event_id, session_id, agent_id, trace_id, span_id, parent_span_id,
		        timestamp, event_type, severity, tool_name, tool_type, mcp_server, message,
		        params, result, context, duration_ms, tags, spawned_by, created_at
		 FROM agent_events%s ORDER BY timestamp %s LIMIT $%d OFFSET $%d`,
		where, order, argIdx, argIdx+1)

	rows, err := r.pool.Query(ctx, query, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()

	var events []model.AgentEvent
	for rows.Next() {
		var e model.AgentEvent
		if err := rows.Scan(&e.EventID, &e.SessionID, &e.AgentID, &e.TraceID, &e.SpanID, &e.ParentSpanID,
			&e.Timestamp, &e.EventType, &e.Severity, &e.ToolName, &e.ToolType, &e.MCPServer, &e.Message,
			&e.Params, &e.Result, &e.Context, &e.DurationMs, &e.Tags, &e.SpawnedBy, &e.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan event: %w", err)
		}
		events = append(events, e)
	}
	return events, total, nil
}

// CountBySession returns the total number of agent events for a given session.
func (r *AgentEventRepo) CountBySession(ctx context.Context, sessionID uuid.UUID) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM agent_events WHERE session_id = $1", sessionID).Scan(&count)
	return count, err
}
