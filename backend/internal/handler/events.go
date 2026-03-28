// Package handler provides HTTP request handlers for the ai-agent-log-hub REST API.
//
// Each handler struct owns a set of interface dependencies (repositories, services)
// that are injected at construction time. This design lets us swap real implementations
// for test doubles (fakes/mocks) in unit tests without changing handler logic.
//
// Every handler method follows the standard Go HTTP signature:
//
//	func(w http.ResponseWriter, r *http.Request)
//
// so it plugs directly into any router that accepts http.HandlerFunc (e.g. chi).
package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/model"
	"github.com/google/uuid"
)

// AgentEnsurer validates and upserts an agent record.
// If the agent already exists in the database, this is a no-op.
// If not, a new agent row is created so that foreign-key constraints
// on events and sessions are satisfied.
//
// This is an interface (rather than a concrete type) so tests can
// provide a lightweight stub instead of a real database connection.
type AgentEnsurer interface {
	EnsureExists(ctx context.Context, agentID string) error
}

// SessionResolver finds or creates the active session for a given agent + token.
// A "session" groups related events together (e.g. one coding task).
// The session_token acts as a client-supplied correlation key; if a session
// with the same (agent_id, session_token) already exists and is still active,
// it is returned. Otherwise a new session is created.
//
// projectDir and gitBranch are optional metadata attached to the session.
type SessionResolver interface {
	ResolveSession(ctx context.Context, agentID, sessionToken string, projectDir, gitBranch *string) (*model.Session, error)
}

// EventInserter persists a batch of events to the database.
// It returns how many events were accepted (inserted) and how many were
// duplicates (skipped because their event_id already existed).
type EventInserter interface {
	InsertBatch(ctx context.Context, events []model.AgentEvent) (accepted, duplicates int, err error)
}

// EventHandler serves the POST /api/v1/events endpoint.
// It orchestrates the full ingestion pipeline: validate input, ensure the
// agent exists, resolve the session, and finally insert the events.
type EventHandler struct {
	agentService   AgentEnsurer
	sessionService SessionResolver
	eventRepo      EventInserter
}

// NewEventHandler creates an EventHandler with the given dependencies.
// All three dependencies are required; passing nil will cause panics at runtime.
func NewEventHandler(agents AgentEnsurer, sessions SessionResolver, events EventInserter) *EventHandler {
	return &EventHandler{
		agentService:   agents,
		sessionService: sessions,
		eventRepo:      events,
	}
}

// EventInput is the JSON shape accepted from callers.
// Pointer fields (e.g. *string) are optional: they will be nil when the
// caller omits them from the JSON payload. json.RawMessage fields (Params,
// Result, Context) store arbitrary nested JSON without parsing it into a
// Go struct -- the backend just passes them through to the database.
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

// validEventTypes is a look-up set of allowed event_type values.
// Using a map[string]bool lets us check membership with a single map access
// (e.g. validEventTypes["tool_call"] == true).
var validEventTypes = map[string]bool{
	"tool_call":    true,
	"explicit_log": true,
	"git_commit":   true,
	"file_change":  true,
	"error":        true,
}

// validSeverities is a look-up set of allowed severity levels.
var validSeverities = map[string]bool{
	"debug": true,
	"info":  true,
	"warn":  true,
	"error": true,
}

// maxBatchSize caps the number of events that can be sent in a single request
// to prevent excessively large payloads from overwhelming the server.
const maxBatchSize = 100

// IngestEvents handles POST /api/v1/events.
//
// The validation flow proceeds in several phases:
//  1. Read and trim the raw request body.
//  2. Detect whether the body is a single JSON object or an array, and
//     unmarshal accordingly (this lets callers send one event or many).
//  3. Validate each event: required fields, timestamp format, enum values.
//     Any validation failure rejects the entire batch (no partial inserts).
//  4. Auto-generate missing event_id (UUID) and default severity to "info".
//  5. Ensure each unique agent_id exists in the agents table.
//  6. Resolve each unique (agent_id, session_token) pair to a session row.
//  7. Map the validated inputs to model.AgentEvent structs and insert them.
//  8. Return a 201 response with accepted/duplicate counts and the session_id.
func (h *EventHandler) IngestEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Phase 1: Read the raw body bytes so we can inspect the first character.
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

	// Phase 2: Detect single object vs array by peeking at the first byte.
	// '{' means a single event; '[' means an array of events.
	var inputs []EventInput
	switch body[0] {
	case '{':
		var single EventInput
		if err := json.Unmarshal(body, &single); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		inputs = []EventInput{single}
	case '[':
		if err := json.Unmarshal(body, &inputs); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
	default:
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

	// Phase 3 & 4: Validate each event and apply defaults.
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
		// Reject timestamps too far in the future to guard against clock skew.
		if ts.After(now.Add(time.Hour)) {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("event[%d]: timestamp is more than 1 hour in the future", i))
			return
		}

		if !validEventTypes[inp.EventType] {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("event[%d]: invalid event_type %q", i, inp.EventType))
			return
		}

		// Default severity to "info" when the caller omits it.
		if inp.Severity == "" {
			inp.Severity = "info"
		}
		if !validSeverities[inp.Severity] {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("event[%d]: invalid severity %q", i, inp.Severity))
			return
		}

		// Generate event_id if missing, so callers don't have to create UUIDs.
		if inp.EventID == "" {
			inp.EventID = uuid.New().String()
		} else {
			if _, err := uuid.Parse(inp.EventID); err != nil {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("event[%d]: invalid event_id: %s", i, err.Error()))
				return
			}
		}

		// Default session_token to agent_id if missing, creating a 1:1 mapping.
		if inp.SessionToken == "" {
			inp.SessionToken = inp.AgentID
		}
	}

	// Phase 5: EnsureExists for each unique agent_id.
	// We track which agents we've already checked to avoid duplicate DB calls.
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

	// Phase 6: ResolveSession for each unique (agent_id, session_token) pair.
	// sessionKey is a local struct used as a map key for deduplication.
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

	// Phase 7: Map validated inputs to model.AgentEvent structs.
	events := make([]model.AgentEvent, len(inputs))
	var lastSessionID uuid.UUID
	for i, inp := range inputs {
		key := sessionKey{inp.AgentID, inp.SessionToken}
		sess := sessionMap[key]
		lastSessionID = sess.SessionID

		eventID := uuid.MustParse(inp.EventID)
		ts, _ := time.Parse(time.RFC3339Nano, inp.Timestamp)

		// Convert the optional spawned_by string to a *uuid.UUID pointer.
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

	// Phase 8: Persist events and return the result.
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

// writeError is a shared helper that sends a JSON error response.
// It sets Content-Type, writes the HTTP status code, and encodes an
// {"error": "..."} JSON body. Every handler in this package uses it
// for consistent error formatting.
func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{
		"error": msg,
	})
}
