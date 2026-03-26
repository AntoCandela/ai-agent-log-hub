package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/model"
	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/repository"
	"github.com/google/uuid"
)

// SystemQuerier abstracts the system event query method.
// System events are distinct from agent events -- they represent
// infrastructure-level occurrences (e.g. service restarts, DB migrations)
// rather than individual AI agent actions.
type SystemQuerier interface {
	Query(ctx context.Context, filters repository.SystemEventFilters) ([]model.SystemEvent, int, error)
}

// SystemHandler serves the GET /api/v1/system endpoint.
// It works very similarly to LogHandler but operates on the system_events
// table with its own filter struct (SystemEventFilters).
type SystemHandler struct {
	systemRepo SystemQuerier
}

// NewSystemHandler creates a SystemHandler with the given dependency.
func NewSystemHandler(systemRepo SystemQuerier) *SystemHandler {
	return &SystemHandler{systemRepo: systemRepo}
}

// QuerySystem handles GET /api/v1/system.
//
// Supports the following query-string filters:
//   - severity: filter by severity level (debug, info, warn, error).
//   - source_service: filter by the service that emitted the event.
//   - event_name: filter by event name (e.g. "db_migration_complete").
//   - trace_id: filter by distributed trace ID.
//   - session_id: filter by associated session (UUID).
//   - text: free-text search across message and attributes.
//   - since / until: time range (RFC 3339 format).
//   - order: "asc" or "desc" (default "desc").
//   - limit: max rows, default 50, max 1000.
//   - offset: pagination offset, default 0.
func (h *SystemHandler) QuerySystem(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	// Default ordering is newest-first.
	filters := repository.SystemEventFilters{
		Order: "desc",
	}

	// Parse each optional filter from the query string.
	if v := q.Get("severity"); v != "" {
		filters.Severity = &v
	}
	if v := q.Get("source_service"); v != "" {
		filters.SourceService = &v
	}
	if v := q.Get("event_name"); v != "" {
		filters.EventName = &v
	}
	if v := q.Get("trace_id"); v != "" {
		filters.TraceID = &v
	}
	if v := q.Get("session_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid session_id: "+err.Error())
			return
		}
		filters.SessionID = &id
	}
	if v := q.Get("text"); v != "" {
		filters.Text = &v
	}
	if v := q.Get("since"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid since: "+err.Error())
			return
		}
		filters.Since = &t
	}
	if v := q.Get("until"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid until: "+err.Error())
			return
		}
		filters.Until = &t
	}
	if v := q.Get("order"); v == "asc" || v == "desc" {
		filters.Order = v
	}

	// Parse limit with default 50 and max 1000.
	limit := 50
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 1000 {
		limit = 1000
	}
	filters.Limit = limit

	// Parse offset for pagination.
	if v := q.Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil && n >= 0 {
			filters.Offset = n
		}
	}

	events, total, err := h.systemRepo.Query(r.Context(), filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed: "+err.Error())
		return
	}

	// Guarantee a JSON array, never null.
	if events == nil {
		events = []model.SystemEvent{}
	}

	hasMore := filters.Offset+len(events) < total

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"data":        events,
		"total_count": total,
		"has_more":    hasMore,
	})
}
