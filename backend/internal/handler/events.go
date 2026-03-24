package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/model"
)

// AgentEnsurer validates and upserts an agent record.
type AgentEnsurer interface {
	EnsureExists(ctx context.Context, agentID string) error
}

// SessionResolver finds or creates the active session for a given agent + token.
type SessionResolver interface {
	ResolveSession(ctx context.Context, agentID, sessionToken string, projectDir, gitBranch *string) (*model.Session, error)
}

// EventInserter persists a batch of events.
type EventInserter interface {
	InsertBatch(ctx context.Context, events []model.AgentEvent) (accepted, duplicates int, err error)
}

// EventHandler serves the POST /api/v1/events endpoint.
type EventHandler struct {
	agentService   AgentEnsurer
	sessionService SessionResolver
	eventRepo      EventInserter
}

// NewEventHandler creates an EventHandler with the given dependencies.
func NewEventHandler(agents AgentEnsurer, sessions SessionResolver, events EventInserter) *EventHandler {
	return &EventHandler{
		agentService:   agents,
		sessionService: sessions,
		eventRepo:      events,
	}
}

// EventInput is the JSON shape accepted from callers.
type EventInput struct {
	EventID      string          `json:"event_id"`
	AgentID      string          `json:"agent_id"`
	SessionToken string          `json:"session_token"`
	ProjectDir   *string         `json:"project_dir"`
	GitBranch    *string         `json:"git_branch"`
	TraceID      *string         `json:"trace_id"`
	SpanID       *string         `json:"span_id"`
	ParentSpanID *string         `json:"parent_span_id"`
	Timestamp    string          `json:"timestamp"`
	EventType    string          `json:"event_type"`
	Severity     string          `json:"severity"`
	ToolName     *string         `json:"tool_name"`
	ToolType     *string         `json:"tool_type"`
	MCPServer    *string         `json:"mcp_server"`
	Message      *string         `json:"message"`
	Params       json.RawMessage `json:"params"`
	Result       json.RawMessage `json:"result"`
	Context      json.RawMessage `json:"context"`
	DurationMs   *int            `json:"duration_ms"`
	Tags         []string        `json:"tags"`
	SpawnedBy    *string         `json:"spawned_by"`
}

var validEventTypes = map[string]bool{
	"tool_call":    true,
	"explicit_log": true,
	"git_commit":  true,
	"file_change": true,
	"error":       true,
}

var validSeverities = map[string]bool{
	"debug": true,
	"info":  true,
	"warn":  true,
	"error": true,
}

const maxBatchSize = 100

// IngestEvents handles POST /api/v1/events.
func (h *EventHandler) IngestEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		writeError(w, http.StatusBadRequest, "empty request body")
		return
	}

	// Detect single object vs array.
	var inputs []EventInput
	if body[0] == '{' {
		var single EventInput
		if err := json.Unmarshal(body, &single); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		inputs = []EventInput{single}
	} else if body[0] == '[' {
		if err := json.Unmarshal(body, &inputs); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
	} else {
		writeError(w, http.StatusBadRequest, "body must be a JSON object or array")
		return
	}

	if len(inputs) == 0 {
		writeError(w, http.StatusBadRequest, "empty event list")
		return
	}
	if len(inputs) > maxBatchSize {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("batch size %d exceeds maximum of %d", len(inputs), maxBatchSize))
		return
	}

	now := time.Now()

	// Validate each event.
	for i := range inputs {
		inp := &inputs[i]

		if inp.AgentID == "" {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("event[%d]: agent_id is required", i))
			return
		}
		if err := model.ValidateAgentID(inp.AgentID); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("event[%d]: %s", i, err.Error()))
			return
		}

		if inp.Timestamp == "" {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("event[%d]: timestamp is required", i))
			return
		}
		ts, err := time.Parse(time.RFC3339Nano, inp.Timestamp)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("event[%d]: invalid timestamp: %s", i, err.Error()))
			return
		}
		if ts.After(now.Add(time.Hour)) {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("event[%d]: timestamp is more than 1 hour in the future", i))
			return
		}

		if !validEventTypes[inp.EventType] {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("event[%d]: invalid event_type %q", i, inp.EventType))
			return
		}

		if inp.Severity == "" {
			inp.Severity = "info"
		}
		if !validSeverities[inp.Severity] {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("event[%d]: invalid severity %q", i, inp.Severity))
			return
		}

		// Generate event_id if missing.
		if inp.EventID == "" {
			inp.EventID = uuid.New().String()
		} else {
			if _, err := uuid.Parse(inp.EventID); err != nil {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("event[%d]: invalid event_id: %s", i, err.Error()))
				return
			}
		}

		// Default session_token to agent_id if missing.
		if inp.SessionToken == "" {
			inp.SessionToken = inp.AgentID
		}
	}

	// EnsureExists for each unique agent_id.
	agentsSeen := make(map[string]bool)
	for _, inp := range inputs {
		if !agentsSeen[inp.AgentID] {
			agentsSeen[inp.AgentID] = true
			if err := h.agentService.EnsureExists(ctx, inp.AgentID); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to ensure agent: "+err.Error())
				return
			}
		}
	}

	// ResolveSession for each unique (agent_id, session_token).
	type sessionKey struct{ agentID, token string }
	sessionMap := make(map[sessionKey]*model.Session)
	for _, inp := range inputs {
		key := sessionKey{inp.AgentID, inp.SessionToken}
		if _, ok := sessionMap[key]; !ok {
			sess, err := h.sessionService.ResolveSession(ctx, inp.AgentID, inp.SessionToken, inp.ProjectDir, inp.GitBranch)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to resolve session: "+err.Error())
				return
			}
			sessionMap[key] = sess
		}
	}

	// Map inputs to model.AgentEvent.
	events := make([]model.AgentEvent, len(inputs))
	var lastSessionID uuid.UUID
	for i, inp := range inputs {
		key := sessionKey{inp.AgentID, inp.SessionToken}
		sess := sessionMap[key]
		lastSessionID = sess.SessionID

		eventID := uuid.MustParse(inp.EventID)
		ts, _ := time.Parse(time.RFC3339Nano, inp.Timestamp)

		var spawnedBy *uuid.UUID
		if inp.SpawnedBy != nil {
			parsed := uuid.MustParse(*inp.SpawnedBy)
			spawnedBy = &parsed
		}

		events[i] = model.AgentEvent{
			EventID:      eventID,
			SessionID:    sess.SessionID,
			AgentID:      inp.AgentID,
			TraceID:      inp.TraceID,
			SpanID:       inp.SpanID,
			ParentSpanID: inp.ParentSpanID,
			Timestamp:    ts,
			EventType:    inp.EventType,
			Severity:     inp.Severity,
			ToolName:     inp.ToolName,
			ToolType:     inp.ToolType,
			MCPServer:    inp.MCPServer,
			Message:      inp.Message,
			Params:       inp.Params,
			Result:       inp.Result,
			Context:      inp.Context,
			DurationMs:   inp.DurationMs,
			Tags:         inp.Tags,
			SpawnedBy:    spawnedBy,
		}
	}

	accepted, duplicates, err := h.eventRepo.InsertBatch(ctx, events)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to insert events: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"data": map[string]any{
			"accepted":   accepted,
			"duplicates": duplicates,
			"session_id": lastSessionID.String(),
		},
	})
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{
		"error": msg,
	})
}
