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
// It uses the same Query signature as other event interfaces, but this
// handler always filters by severity="error" and requires a text pattern.
type ErrorSearcher interface {
	Query(ctx context.Context, filters repository.EventFilters) ([]model.AgentEvent, int, error)
}

// ErrorHandler serves the GET /api/v1/logs/errors endpoint.
// It searches for error-severity events and returns each one with
// surrounding "context" events so the developer can see what happened
// immediately before and after the error.
type ErrorHandler struct {
	eventRepo ErrorSearcher
}

// NewErrorHandler creates an ErrorHandler with the given dependency.
func NewErrorHandler(eventRepo ErrorSearcher) *ErrorHandler {
	return &ErrorHandler{eventRepo: eventRepo}
}

// errorWithContext pairs an error event with surrounding context events.
// "Before" contains up to 3 events that occurred just before the error,
// and "After" contains up to 3 events that occurred just after it, all
// within the same session. This gives developers a mini timeline around
// each failure.
type errorWithContext struct {
	Error  model.AgentEvent   `json:"error"`
	Before []model.AgentEvent `json:"before"`
	After  []model.AgentEvent `json:"after"`
}

// SearchErrors handles GET /api/v1/logs/errors.
//
// The context-window approach:
//  1. Query error-severity events matching the caller's text pattern.
//  2. For each error found, make a second query to fetch events from the
//     same session within a +/-5 minute window around the error timestamp.
//  3. Sort those context events chronologically.
//  4. Locate the error event in the list and extract up to 3 events before
//     and 3 events after it (the "context window").
//  5. If the context fetch fails, the error is still returned but with
//     empty before/after arrays (graceful degradation).
//
// Query parameters:
//   - pattern (required): text to search for in error messages.
//   - agent_id (optional): restrict to a single agent.
//   - since (optional): only errors after this RFC 3339 timestamp.
//   - limit (optional): max errors to return, default 20, max 100.
func (h *ErrorHandler) SearchErrors(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	pattern := q.Get("pattern")
	if pattern == "" {
		writeError(w, http.StatusBadRequest, "pattern query parameter is required")
		return
	}

	// Build filters for error events: always severity="error" + text pattern.
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

	// Step 1: Fetch error events matching the pattern.
	errors, total, err := h.eventRepo.Query(r.Context(), filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed: "+err.Error())
		return
	}

	// Steps 2-5: For each error, fetch surrounding context events.
	results := make([]errorWithContext, 0, len(errors))
	for _, errEvt := range errors {
		ctx := r.Context()

		// contextWindow is how many events to show before and after the error.
		contextWindow := 3
		// Query a 10-minute window (5 min each side) in the same session.
		beforeTime := errEvt.Timestamp.Add(-5 * time.Minute)
		afterTime := errEvt.Timestamp.Add(5 * time.Minute)
		sessionID := errEvt.SessionID

		contextFilters := repository.EventFilters{
			SessionID: &sessionID,
			Since:     &beforeTime,
			Until:     &afterTime,
			Limit:     contextWindow*2 + 1 + 10, // fetch extra rows to ensure we capture enough context
			Order:     "asc",
		}

		contextEvents, _, err := h.eventRepo.Query(ctx, contextFilters)
		if err != nil {
			// Graceful degradation: return the error without context if the
			// second query fails.
			results = append(results, errorWithContext{
				Error:  errEvt,
				Before: []model.AgentEvent{},
				After:  []model.AgentEvent{},
			})
			continue
		}

		// Sort by timestamp ascending to establish chronological order.
		sort.Slice(contextEvents, func(i, j int) bool {
			return contextEvents[i].Timestamp.Before(contextEvents[j].Timestamp)
		})

		// Find the error event in the context list by matching its EventID.
		var before, after []model.AgentEvent
		errorIdx := -1
		for i, e := range contextEvents {
			if e.EventID == errEvt.EventID {
				errorIdx = i
				break
			}
		}

		if errorIdx >= 0 {
			// Extract up to 3 events before the error.
			start := errorIdx - contextWindow
			if start < 0 {
				start = 0
			}
			before = contextEvents[start:errorIdx]

			// Extract up to 3 events after the error.
			end := errorIdx + 1 + contextWindow
			if end > len(contextEvents) {
				end = len(contextEvents)
			}
			if errorIdx+1 < len(contextEvents) {
				after = contextEvents[errorIdx+1 : end]
			}
		}

		// Ensure JSON arrays are never null.
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
