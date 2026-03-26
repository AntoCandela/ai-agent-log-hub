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

// SystemEventRepo provides database access for system event records.
// System events come from infrastructure (e.g. OpenTelemetry traces/logs from
// services that the AI agent interacts with) rather than from the agent itself.
// They are stored separately from agent events but can be correlated to agent
// sessions via shared trace IDs (see LinkToSessions).
type SystemEventRepo struct {
	pool *pgxpool.Pool
}

// NewSystemEventRepo creates a new SystemEventRepo backed by the given pool.
func NewSystemEventRepo(pool *pgxpool.Pool) *SystemEventRepo {
	return &SystemEventRepo{pool: pool}
}

// InsertBatch inserts multiple system events using pgx.Batch for efficiency.
// Like AgentEventRepo.InsertBatch, it uses ON CONFLICT (event_id) DO NOTHING
// for idempotent deduplication. Returns the count of newly inserted rows.
func (r *SystemEventRepo) InsertBatch(ctx context.Context, events []model.SystemEvent) (int, error) {
	batch := &pgx.Batch{}
	for _, e := range events {
		// Default nullable JSON fields to empty objects so PostgreSQL receives
		// valid JSON rather than NULL.
		attrs := e.Attributes
		if attrs == nil {
			attrs = []byte("{}")
		}
		res := e.Resource
		if res == nil {
			res = []byte("{}")
		}

		// Queue an INSERT with duplicate-skipping via ON CONFLICT DO NOTHING.
		batch.Queue(
			`INSERT INTO system_events (event_id, timestamp, trace_id, span_id, parent_span_id,
			 source_type, source_service, severity, event_name, message,
			 attributes, resource, duration_ms, session_id)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
			 ON CONFLICT (event_id) DO NOTHING`,
			e.EventID, e.Timestamp, e.TraceID, e.SpanID, e.ParentSpanID,
			e.SourceType, e.SourceService, e.Severity, e.EventName, e.Message,
			attrs, res, e.DurationMs, e.SessionID,
		)
	}

	results := r.pool.SendBatch(ctx, batch)
	defer results.Close()

	accepted := 0
	for range events {
		ct, err := results.Exec()
		if err != nil {
			return 0, fmt.Errorf("insert system event: %w", err)
		}
		if ct.RowsAffected() > 0 {
			accepted++
		}
	}
	return accepted, nil
}

// SystemEventFilters holds optional filter criteria for querying system events.
// Uses the same composable pattern as EventFilters — nil pointer = skip filter.
type SystemEventFilters struct {
	Severity      *string    // Filter by severity level (e.g. "error", "warn").
	SourceService *string    // Filter by originating service name.
	EventName     *string    // Filter by event/span name.
	TraceID       *string    // Filter by distributed trace ID.
	SessionID     *uuid.UUID // Filter to events linked to this agent session.
	Text          *string    // Full-text search across message and event_name.
	Since         *time.Time // Only events at or after this timestamp.
	Until         *time.Time // Only events at or before this timestamp.
	Limit         int        // Maximum number of results (default 50, max 1000).
	Offset        int        // Number of results to skip for pagination.
	Order         string     // Sort direction: "asc" or "desc" (default "desc").
}

// Query returns system events matching the given filters, plus total count.
// Uses the same composable WHERE builder as AgentEventRepo.Query.
func (r *SystemEventRepo) Query(ctx context.Context, filters SystemEventFilters) ([]model.SystemEvent, int, error) {
	var conditions []string
	var args []any
	argIdx := 1

	if filters.Severity != nil {
		conditions = append(conditions, fmt.Sprintf("severity = $%d", argIdx))
		args = append(args, *filters.Severity)
		argIdx++
	}
	if filters.SourceService != nil {
		conditions = append(conditions, fmt.Sprintf("source_service = $%d", argIdx))
		args = append(args, *filters.SourceService)
		argIdx++
	}
	if filters.EventName != nil {
		conditions = append(conditions, fmt.Sprintf("event_name = $%d", argIdx))
		args = append(args, *filters.EventName)
		argIdx++
	}
	if filters.TraceID != nil {
		conditions = append(conditions, fmt.Sprintf("trace_id = $%d", argIdx))
		args = append(args, *filters.TraceID)
		argIdx++
	}
	if filters.SessionID != nil {
		conditions = append(conditions, fmt.Sprintf("session_id = $%d", argIdx))
		args = append(args, *filters.SessionID)
		argIdx++
	}
	if filters.Text != nil {
		conditions = append(conditions, fmt.Sprintf(
			"to_tsvector('english', COALESCE(message,'') || ' ' || COALESCE(event_name,'')) @@ plainto_tsquery('english', $%d)", argIdx))
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

	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	// Count
	var total int
	err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM system_events"+where, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count system events: %w", err)
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

	listArgs := append(args, limit, filters.Offset)
	query := fmt.Sprintf(
		`SELECT event_id, timestamp, trace_id, span_id, parent_span_id,
		        source_type, source_service, severity, event_name, message,
		        attributes, resource, duration_ms, session_id, created_at
		 FROM system_events%s ORDER BY timestamp %s LIMIT $%d OFFSET $%d`,
		where, order, argIdx, argIdx+1)

	rows, err := r.pool.Query(ctx, query, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("query system events: %w", err)
	}
	defer rows.Close()

	var events []model.SystemEvent
	for rows.Next() {
		var e model.SystemEvent
		if err := rows.Scan(
			&e.EventID, &e.Timestamp, &e.TraceID, &e.SpanID, &e.ParentSpanID,
			&e.SourceType, &e.SourceService, &e.Severity, &e.EventName, &e.Message,
			&e.Attributes, &e.Resource, &e.DurationMs, &e.SessionID, &e.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan system event: %w", err)
		}
		events = append(events, e)
	}
	return events, total, nil
}

// LinkToSessions is the core of the "trace correlation engine". It links
// system events to agent sessions by matching trace IDs.
//
// How it works:
//  1. Find system_events rows that have a trace_id but no session_id yet.
//  2. JOIN them against agent_events that share the same trace_id.
//  3. Copy the agent_event's session_id into the system_event.
//
// This lets the UI show infrastructure-level telemetry (HTTP spans, database
// queries, etc.) alongside the agent events that triggered them, because they
// share the same distributed trace. Returns the number of rows updated.
func (r *SystemEventRepo) LinkToSessions(ctx context.Context) (int, error) {
	// Query: for every system event that has a trace_id but no session_id,
	// find an agent event with the same trace_id and copy its session_id.
	ct, err := r.pool.Exec(ctx,
		`UPDATE system_events SET session_id = ae.session_id
		 FROM agent_events ae
		 WHERE system_events.trace_id = ae.trace_id
		   AND system_events.session_id IS NULL
		   AND system_events.trace_id IS NOT NULL`,
	)
	if err != nil {
		return 0, fmt.Errorf("link system events to sessions: %w", err)
	}
	return int(ct.RowsAffected()), nil
}

// FindByTraceID returns all system events sharing a given distributed trace ID,
// ordered chronologically. This is used to reconstruct the full trace timeline
// for a specific request or operation.
func (r *SystemEventRepo) FindByTraceID(ctx context.Context, traceID string) ([]model.SystemEvent, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT event_id, timestamp, trace_id, span_id, parent_span_id,
		        source_type, source_service, severity, event_name, message,
		        attributes, resource, duration_ms, session_id, created_at
		 FROM system_events
		 WHERE trace_id = $1
		 ORDER BY timestamp ASC`,
		traceID,
	)
	if err != nil {
		return nil, fmt.Errorf("find system events by trace_id: %w", err)
	}
	defer rows.Close()

	var events []model.SystemEvent
	for rows.Next() {
		var e model.SystemEvent
		if err := rows.Scan(
			&e.EventID, &e.Timestamp, &e.TraceID, &e.SpanID, &e.ParentSpanID,
			&e.SourceType, &e.SourceService, &e.Severity, &e.EventName, &e.Message,
			&e.Attributes, &e.Resource, &e.DurationMs, &e.SessionID, &e.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan system event: %w", err)
		}
		events = append(events, e)
	}
	return events, nil
}
