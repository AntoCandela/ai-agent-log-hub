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

type AgentEventRepo struct {
	pool *pgxpool.Pool
}

func NewAgentEventRepo(pool *pgxpool.Pool) *AgentEventRepo {
	return &AgentEventRepo{pool: pool}
}

func (r *AgentEventRepo) InsertBatch(ctx context.Context, events []model.AgentEvent) (accepted, duplicates int, err error) {
	batch := &pgx.Batch{}
	for _, e := range events {
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

	results := r.pool.SendBatch(ctx, batch)
	defer results.Close()

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

type EventFilters struct {
	SessionID *uuid.UUID
	AgentID   *string
	ToolName  *string
	EventType *string
	Severity  *string
	FilePath  *string
	TraceID   *string
	Text      *string
	Since     *time.Time
	Until     *time.Time
	Tags      []string
	Limit     int
	Offset    int
	Order     string // "asc" or "desc"
}

func (r *AgentEventRepo) Query(ctx context.Context, filters EventFilters) ([]model.AgentEvent, int, error) {
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

	listArgs := append(args, limit, filters.Offset)
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

func (r *AgentEventRepo) CountBySession(ctx context.Context, sessionID uuid.UUID) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM agent_events WHERE session_id = $1", sessionID).Scan(&count)
	return count, err
}
