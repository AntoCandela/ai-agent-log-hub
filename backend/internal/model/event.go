// TOCHANGE: Stack migration — model structs evolve for SurrealDB
// - Add: graph-related fields (record links instead of foreign key UUIDs)
// - Add: File, Commit, MCPServer models (new record types)
// - Add: Relation types (Touched, CausedBy, FollowedBy, CoAccessed, etc.)
// - Keep: core field semantics (event_type, severity, tool_name, params, result)
// - See autok design fragment DES-2 for full SurrealDB record + relation schema
package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// AgentEvent represents a single action or observation recorded by an
// AI agent during a session — for example, a tool call, a chat message,
// or an error. It is the most granular unit of data in the log hub.
//
// Several fields use json.RawMessage instead of a typed struct. This
// means the JSON is stored and forwarded as-is without parsing it into
// Go types. The advantage is flexibility: different agents can send
// different shapes of data in "params", "result", or "context" without
// requiring schema changes on the backend.
type AgentEvent struct {
	// EventID is the primary key (UUID).
	EventID uuid.UUID `json:"event_id"`
	// SessionID links this event to the session it belongs to.
	SessionID uuid.UUID `json:"session_id"`
	// AgentID identifies which agent emitted this event.
	AgentID string `json:"agent_id"`
	// TraceID is the distributed-tracing trace identifier, used to
	// correlate this event with system-level spans (e.g. from OTLP).
	// Nil if the agent did not supply one.
	TraceID *string `json:"trace_id"`
	// SpanID is the distributed-tracing span identifier for this event.
	SpanID *string `json:"span_id"`
	// ParentSpanID links this span to its parent in the trace tree.
	ParentSpanID *string `json:"parent_span_id"`
	// Timestamp is the wall-clock time the event occurred (set by the agent).
	Timestamp time.Time `json:"timestamp"`
	// EventType categorizes the event, e.g. "tool_call", "chat", "error".
	EventType string `json:"event_type"`
	// Severity is the log level: "info", "warn", "error", etc.
	Severity string `json:"severity"`
	// ToolName is the name of the tool that was called, if this is a tool event.
	ToolName *string `json:"tool_name"`
	// ToolType classifies the tool (e.g. "file", "shell", "search").
	ToolType *string `json:"tool_type"`
	// MCPServer is the MCP server that handled the tool call, if applicable.
	MCPServer *string `json:"mcp_server"`
	// Message is a human-readable description of the event.
	Message *string `json:"message"`
	// Params holds the input parameters of the event as raw JSON.
	// Using json.RawMessage avoids deserializing agent-specific payloads.
	Params json.RawMessage `json:"params"`
	// Result holds the output/result of the event as raw JSON.
	Result json.RawMessage `json:"result"`
	// Context holds additional contextual data as raw JSON (e.g. file paths,
	// conversation state, environment info).
	Context json.RawMessage `json:"context"`
	// DurationMs is how long the event took in milliseconds (e.g. tool execution time).
	DurationMs *int `json:"duration_ms"`
	// Tags is a list of free-form labels for filtering (e.g. ["refactor", "test"]).
	Tags []string `json:"tags"`
	// SpawnedBy is the EventID of a parent event that spawned this one
	// (used for sub-agent or nested-tool relationships).
	SpawnedBy *uuid.UUID `json:"spawned_by"`
	// CreatedAt is when this row was inserted into the database.
	CreatedAt time.Time `json:"created_at"`
}
