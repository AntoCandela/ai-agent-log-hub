package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/model"
	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/repository"
)

// BlameQuerier abstracts the event query method for blame lookups.
// Blame needs the same Query signature as other handlers, but only uses
// the FilePath filter to find events that touched a specific file.
type BlameQuerier interface {
	Query(ctx context.Context, filters repository.EventFilters) ([]model.AgentEvent, int, error)
}

// BlameHandler serves the GET /api/v1/logs/blame endpoint.
// "Blame" answers the question: "which agents (and sessions) recently
// modified this file?" -- similar in spirit to "git blame" but for
// AI agent activity.
type BlameHandler struct {
	eventRepo BlameQuerier
}

// NewBlameHandler creates a BlameHandler with the given dependency.
func NewBlameHandler(eventRepo BlameQuerier) *BlameHandler {
	return &BlameHandler{eventRepo: eventRepo}
}

// blameEntry represents a single file modification entry.
// Each entry records which agent touched the file, in which session,
// at what time, and (optionally) which tool was used and what message
// was logged.
type blameEntry struct {
	AgentID   string  `json:"agent_id"`
	SessionID string  `json:"session_id"`
	Timestamp string  `json:"timestamp"`
	ToolName  *string `json:"tool_name"`
	Message   *string `json:"message"`
}

// GetBlame handles GET /api/v1/logs/blame.
//
// Query parameters:
//   - file (required): the file path to look up (e.g. "src/main.go").
//   - depth (optional): how many recent modifications to return.
//     Defaults to 5, maximum 100.
//
// The handler queries events that reference the given file path, orders
// them newest-first, and returns a simplified list of blame entries.
func (h *BlameHandler) GetBlame(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	file := q.Get("file")
	if file == "" {
		writeError(w, http.StatusBadRequest, "file query parameter is required")
		return
	}

	// Parse the "depth" parameter (how many recent modifications to return).
	depth := 5
	if v := q.Get("depth"); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil && n > 0 {
			depth = n
		}
	}
	if depth > 100 {
		depth = 100
	}

	// Query events that reference this file, newest first.
	filters := repository.EventFilters{
		FilePath: &file,
		Limit:    depth,
		Order:    "desc",
	}

	events, _, err := h.eventRepo.Query(r.Context(), filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed: "+err.Error())
		return
	}

	// Convert full event objects to lightweight blame entries.
	entries := make([]blameEntry, 0, len(events))
	for _, e := range events {
		entries = append(entries, blameEntry{
			AgentID:   e.AgentID,
			SessionID: e.SessionID.String(),
			Timestamp: e.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
			ToolName:  e.ToolName,
			Message:   e.Message,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"data": entries,
		"file": file,
	})
}
