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
type BlameQuerier interface {
	Query(ctx context.Context, filters repository.EventFilters) ([]model.AgentEvent, int, error)
}

// BlameHandler serves the GET /api/v1/logs/blame endpoint.
type BlameHandler struct {
	eventRepo BlameQuerier
}

// NewBlameHandler creates a BlameHandler with the given dependency.
func NewBlameHandler(eventRepo BlameQuerier) *BlameHandler {
	return &BlameHandler{eventRepo: eventRepo}
}

// blameEntry represents a single file modification entry.
type blameEntry struct {
	AgentID   string  `json:"agent_id"`
	SessionID string  `json:"session_id"`
	Timestamp string  `json:"timestamp"`
	ToolName  *string `json:"tool_name"`
	Message   *string `json:"message"`
}

// GetBlame handles GET /api/v1/logs/blame.
func (h *BlameHandler) GetBlame(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	file := q.Get("file")
	if file == "" {
		writeError(w, http.StatusBadRequest, "file query parameter is required")
		return
	}

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
