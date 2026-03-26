package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/model"
	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/repository"
)

// ErrorSearcher abstracts the event query method for error search.
type ErrorSearcher interface {
	Query(ctx context.Context, filters repository.EventFilters) ([]model.AgentEvent, int, error)
}

// ErrorHandler serves the GET /api/v1/logs/errors endpoint.
type ErrorHandler struct {
	eventRepo ErrorSearcher
}

// NewErrorHandler creates an ErrorHandler with the given dependency.
func NewErrorHandler(eventRepo ErrorSearcher) *ErrorHandler {
	return &ErrorHandler{eventRepo: eventRepo}
}

// errorWithContext pairs an error event with surrounding context events.
type errorWithContext struct {
	Error   model.AgentEvent   `json:"error"`
	Before  []model.AgentEvent `json:"before"`
	After   []model.AgentEvent `json:"after"`
}

// SearchErrors handles GET /api/v1/logs/errors.
func (h *ErrorHandler) SearchErrors(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	pattern := q.Get("pattern")
	if pattern == "" {
		writeError(w, http.StatusBadRequest, "pattern query parameter is required")
		return
	}

	// Build filters for error events.
	errorSeverity := "error"
	filters := repository.EventFilters{
		Severity: &errorSeverity,
		Text:     &pattern,
		Order:    "desc",
	}

	if v := q.Get("agent_id"); v != "" {
		filters.AgentID = &v
	}
	if v := q.Get("since"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid since: "+err.Error())
			return
		}
		filters.Since = &t
	}

	limit := 20
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 100 {
		limit = 100
	}
	filters.Limit = limit

	errors, total, err := h.eventRepo.Query(r.Context(), filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed: "+err.Error())
		return
	}

	// For each error, fetch context events from the same session.
	results := make([]errorWithContext, 0, len(errors))
	for _, errEvt := range errors {
		ctx := r.Context()

		// Query events in the same session, around the error timestamp.
		contextWindow := 3
		beforeTime := errEvt.Timestamp.Add(-5 * time.Minute)
		afterTime := errEvt.Timestamp.Add(5 * time.Minute)
		sessionID := errEvt.SessionID

		contextFilters := repository.EventFilters{
			SessionID: &sessionID,
			Since:     &beforeTime,
			Until:     &afterTime,
			Limit:     contextWindow*2 + 1 + 10, // fetch enough to find context
			Order:     "asc",
		}

		contextEvents, _, err := h.eventRepo.Query(ctx, contextFilters)
		if err != nil {
			// If context fetch fails, still return the error without context.
			results = append(results, errorWithContext{
				Error:  errEvt,
				Before: []model.AgentEvent{},
				After:  []model.AgentEvent{},
			})
			continue
		}

		// Sort by timestamp ascending.
		sort.Slice(contextEvents, func(i, j int) bool {
			return contextEvents[i].Timestamp.Before(contextEvents[j].Timestamp)
		})

		// Find the error event in context and extract before/after.
		var before, after []model.AgentEvent
		errorIdx := -1
		for i, e := range contextEvents {
			if e.EventID == errEvt.EventID {
				errorIdx = i
				break
			}
		}

		if errorIdx >= 0 {
			// Get up to 3 events before.
			start := errorIdx - contextWindow
			if start < 0 {
				start = 0
			}
			before = contextEvents[start:errorIdx]

			// Get up to 3 events after.
			end := errorIdx + 1 + contextWindow
			if end > len(contextEvents) {
				end = len(contextEvents)
			}
			if errorIdx+1 < len(contextEvents) {
				after = contextEvents[errorIdx+1 : end]
			}
		}

		if before == nil {
			before = []model.AgentEvent{}
		}
		if after == nil {
			after = []model.AgentEvent{}
		}

		results = append(results, errorWithContext{
			Error:  errEvt,
			Before: before,
			After:  after,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"data":        results,
		"total_count": total,
	})
}
