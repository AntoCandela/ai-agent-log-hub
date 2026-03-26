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
type SystemQuerier interface {
	Query(ctx context.Context, filters repository.SystemEventFilters) ([]model.SystemEvent, int, error)
}

// SystemHandler serves the GET /api/v1/system endpoint.
type SystemHandler struct {
	systemRepo SystemQuerier
}

// NewSystemHandler creates a SystemHandler with the given dependency.
func NewSystemHandler(systemRepo SystemQuerier) *SystemHandler {
	return &SystemHandler{systemRepo: systemRepo}
}

// QuerySystem handles GET /api/v1/system.
func (h *SystemHandler) QuerySystem(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	filters := repository.SystemEventFilters{
		Order: "desc",
	}

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
