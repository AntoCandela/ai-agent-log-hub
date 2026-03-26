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
// It exposes listing (with filters and pagination) and single-session lookup.
type SessionLister interface {
	List(ctx context.Context, filters repository.SessionFilters, limit, offset int) (*repository.SessionListResult, error)
	GetByID(ctx context.Context, sessionID uuid.UUID) (*model.Session, error)
}

// SummaryLister abstracts the summary query methods for testability.
// Session summaries are generated asynchronously, so they may not exist yet
// for every session -- callers must handle a nil return gracefully.
type SummaryLister interface {
	GetBySessionID(ctx context.Context, sessionID uuid.UUID) (*model.SessionSummary, error)
}

// SessionHandler serves the /api/v1/sessions endpoints.
// It aggregates data from sessions, events, and (optionally) summaries.
type SessionHandler struct {
	sessionRepo SessionLister
	eventRepo   LogQuerier
	summaryRepo SummaryLister // nil = return placeholder summary
}

// NewSessionHandler creates a SessionHandler with the given dependencies.
// The summary repository is optional and can be attached later via SetSummaryRepo.
func NewSessionHandler(sessionRepo SessionLister, eventRepo LogQuerier) *SessionHandler {
	return &SessionHandler{
		sessionRepo: sessionRepo,
		eventRepo:   eventRepo,
	}
}

// SetSummaryRepo attaches a summary repository for real summary lookups.
// Call this after construction when the summary subsystem is available.
func (h *SessionHandler) SetSummaryRepo(repo SummaryLister) {
	h.summaryRepo = repo
}

// ListSessions handles GET /api/v1/sessions.
// Supports filtering by agent_id, status, project_dir, and pinned flag,
// with limit/offset pagination (defaults: limit=50, max 1000).
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

	// Guarantee a JSON array in the response, never null.
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
// Returns a single session by its UUID, or 404 if not found.
func (h *SessionHandler) GetSession(w http.ResponseWriter, r *http.Request) {
	// chi.URLParam extracts the {sessionID} path variable from the route.
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
// A "touch" is any tool_call event whose params JSON contains a "file_path" key.
type FileTouchInfo struct {
	FilePath string `json:"file_path"`
	Touches  int    `json:"touches"`
}

// GetSessionFiles handles GET /api/v1/sessions/{sessionID}/files.
//
// File aggregation logic:
//  1. Query all tool_call events for the given session (up to 1000).
//  2. For each event, unmarshal its params JSON and extract the "file_path" field.
//  3. Count how many events reference each unique file path.
//  4. Return the aggregated list of {file_path, touches} objects.
//
// An optional "file_path" query parameter narrows the event query to a single file.
func (h *SessionHandler) GetSessionFiles(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "sessionID")
	sessionID, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid session ID: "+err.Error())
		return
	}

	// Step 1: Query tool_call events for this session.
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

	// Steps 2-3: Aggregate by file_path extracted from the params JSON blob.
	// Events whose params don't contain a file_path are silently skipped.
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

	// Step 4: Build the response slice.
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
// Returns the full generated summary if available, otherwise falls back to
// basic session metadata.
//
// The two-tier approach works as follows:
//  1. If a summaryRepo is configured, try to load the AI-generated summary.
//  2. If the summary exists, return it directly.
//  3. Otherwise, fall back to returning basic session fields (id, status, etc.)
//     so the frontend always gets a usable response.
func (h *SessionHandler) GetSessionSummary(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "sessionID")
	sessionID, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid session ID: "+err.Error())
		return
	}

	// Try to return the real summary if the repo is configured.
	if h.summaryRepo != nil {
		summary, err := h.summaryRepo.GetBySessionID(r.Context(), sessionID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "query failed: "+err.Error())
			return
		}
		if summary != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"data": summary,
			})
			return
		}
	}

	// Fallback: return basic session info when no summary exists yet.
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
