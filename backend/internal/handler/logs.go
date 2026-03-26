package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/model"
	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/repository"
	"github.com/google/uuid"
)

// LogQuerier abstracts the event query method for testability.
// By depending on this small interface instead of the full repository struct,
// the handler only sees the one method it needs, and tests can provide a
// lightweight fake that returns canned data.
type LogQuerier interface {
	Query(ctx context.Context, filters repository.EventFilters) ([]model.AgentEvent, int, error)
}

// LogHandler serves the GET /api/v1/logs endpoint.
// It translates HTTP query-string parameters into a repository.EventFilters
// struct and delegates the actual database work to the LogQuerier dependency.
type LogHandler struct {
	eventRepo LogQuerier
}

// NewLogHandler creates a LogHandler with the given dependency.
func NewLogHandler(eventRepo LogQuerier) *LogHandler {
	return &LogHandler{eventRepo: eventRepo}
}

// QueryLogs handles GET /api/v1/logs.
//
// Filter parsing:
// Each supported query-string parameter is read individually from the URL.
// Optional filters (session_id, agent_id, tool_name, etc.) are only set on
// the EventFilters struct when the caller actually provides them. This means
// an omitted parameter results in "no filter on that field" (the pointer
// stays nil).
//
// Pagination:
//   - "limit" defaults to 50, max 1000. Invalid values fall back to the default.
//   - "offset" skips that many rows. Default 0.
//   - The response includes "total_count" and "has_more" so the frontend can
//     build pagination controls.
//
// Time range:
//   - "since" and "until" accept RFC 3339 timestamps (e.g. "2025-01-15T00:00:00Z").
//
// Tags:
//   - Comma-separated list, e.g. "tags=deploy,hotfix".
func (h *LogHandler) QueryLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	// Start with default ordering (newest first).
	filters := repository.EventFilters{
		Order: "desc",
	}

	// --- Optional filters: each block reads one query parameter ---

	if v := q.Get("session_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid session_id: "+err.Error())
			return
		}
		filters.SessionID = &id
	}
	if v := q.Get("agent_id"); v != "" {
		filters.AgentID = &v
	}
	if v := q.Get("tool_name"); v != "" {
		filters.ToolName = &v
	}
	if v := q.Get("event_type"); v != "" {
		filters.EventType = &v
	}
	if v := q.Get("severity"); v != "" {
		filters.Severity = &v
	}
	if v := q.Get("file_path"); v != "" {
		filters.FilePath = &v
	}
	if v := q.Get("trace_id"); v != "" {
		filters.TraceID = &v
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
	if v := q.Get("tags"); v != "" {
		// Tags arrive as a comma-separated string and are split into a slice.
		filters.Tags = strings.Split(v, ",")
	}
	if v := q.Get("order"); v == "asc" || v == "desc" {
		filters.Order = v
	}

	// Parse limit with default 50 and max 1000.
	limit := 50
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			// Use default
		} else {
			limit = n
		}
	}
	if limit > 1000 {
		limit = 1000
	}
	filters.Limit = limit

	// Parse offset (number of rows to skip for pagination).
	if v := q.Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil && n >= 0 {
			filters.Offset = n
		}
	}

	// Execute the query against the repository.
	events, total, err := h.eventRepo.Query(r.Context(), filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed: "+err.Error())
		return
	}

	// Ensure the JSON response contains an empty array [] instead of null
	// when there are no matching events.
	if events == nil {
		events = []model.AgentEvent{}
	}

	// has_more tells the frontend whether additional pages exist.
	hasMore := filters.Offset+len(events) < total

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"data":        events,
		"total_count": total,
		"has_more":    hasMore,
	})
}
