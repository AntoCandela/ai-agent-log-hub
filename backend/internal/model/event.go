package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type AgentEvent struct {
	EventID      uuid.UUID       `json:"event_id"`
	SessionID    uuid.UUID       `json:"session_id"`
	AgentID      string          `json:"agent_id"`
	TraceID      *string         `json:"trace_id"`
	SpanID       *string         `json:"span_id"`
	ParentSpanID *string         `json:"parent_span_id"`
	Timestamp    time.Time       `json:"timestamp"`
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
	SpawnedBy    *uuid.UUID      `json:"spawned_by"`
	CreatedAt    time.Time       `json:"created_at"`
}
