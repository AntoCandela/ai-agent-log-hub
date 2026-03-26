package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/model"
	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// SessionLister abstracts the session query methods for testability.
type SessionLister interface {
	List(ctx context.Context, filters repository.SessionFilters, limit, offset int) (*repository.SessionListResult, error)
	GetByID(ctx context.Context, sessionID uuid.UUID) (*model.Session, error)
}

// SessionHandler serves the /api/v1/sessions endpoints.
type SessionHandler struct {
	sessionRepo SessionLister
	eventRepo   LogQuerier
}

// NewSessionHandler creates a SessionHandler with the given dependencies.
func NewSessionHandler(sessionRepo SessionLister, eventRepo LogQuerier) *SessionHandler {
	return &SessionHandler{
		sessionRepo: sessionRepo,
		eventRepo:   eventRepo,
	}
}

// ListSessions handles GET /api/v1/sessions.
func (h *SessionHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	filters := repository.SessionFilters{}
	if v := q.Get("agent_id"); v != "" {
		filters.AgentID = v
	}
	if v := q.Get("status"); v != "" {
		filters.Status = v
	}
	if v := q.Get("project_dir"); v != "" {
		filters.ProjectDir = &v
	}
	if v := q.Get("pinned"); v != "" {
		b, err := strconv.ParseBool(v)
		if err == nil {
			filters.Pinned = &b
		}
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

	offset := 0
	if v := q.Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil && n >= 0 {
			offset = n
		}
	}

	result, err := h.sessionRepo.List(r.Context(), filters, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed: "+err.Error())
		return
	}

	sessions := result.Sessions
	if sessions == nil {
		sessions = []*model.Session{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"data":        sessions,
		"total_count": result.Total,
	})
}

// GetSession handles GET /api/v1/sessions/{sessionID}.
func (h *SessionHandler) GetSession(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "sessionID")
	sessionID, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid session ID: "+err.Error())
		return
	}

	session, err := h.sessionRepo.GetByID(r.Context(), sessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed: "+err.Error())
		return
	}
	if session == nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"data": session,
	})
}

// FileTouchInfo aggregates how many times each file was touched in a session.
type FileTouchInfo struct {
	FilePath string `json:"file_path"`
	Touches  int    `json:"touches"`
}

// GetSessionFiles handles GET /api/v1/sessions/{sessionID}/files.
func (h *SessionHandler) GetSessionFiles(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "sessionID")
	sessionID, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid session ID: "+err.Error())
		return
	}

	// Query tool_call events for this session.
	toolCall := "tool_call"
	filters := repository.EventFilters{
		SessionID: &sessionID,
		EventType: &toolCall,
		Limit:     1000,
	}

	if fp := r.URL.Query().Get("file_path"); fp != "" {
		filters.FilePath = &fp
	}

	events, _, err := h.eventRepo.Query(r.Context(), filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed: "+err.Error())
		return
	}

	// Aggregate by file_path from params.
	touchMap := make(map[string]int)
	for _, e := range events {
		if e.Params == nil {
			continue
		}
		var params map[string]any
		if err := json.Unmarshal(e.Params, &params); err != nil {
			continue
		}
		fp, ok := params["file_path"].(string)
		if !ok || fp == "" {
			continue
		}
		touchMap[fp]++
	}

	files := make([]FileTouchInfo, 0, len(touchMap))
	for fp, count := range touchMap {
		files = append(files, FileTouchInfo{FilePath: fp, Touches: count})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"data": files,
	})
}

// GetSessionSummary handles GET /api/v1/sessions/{sessionID}/summary.
// This is a placeholder; full summary generation comes in TASK-19.
func (h *SessionHandler) GetSessionSummary(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "sessionID")
	sessionID, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid session ID: "+err.Error())
		return
	}

	session, err := h.sessionRepo.GetByID(r.Context(), sessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed: "+err.Error())
		return
	}
	if session == nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"data": map[string]any{
			"session_id":  session.SessionID,
			"agent_id":    session.AgentID,
			"status":      session.Status,
			"started_at":  session.StartedAt,
			"ended_at":    session.EndedAt,
			"event_count": session.EventCount,
		},
	})
}
