package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type SystemEvent struct {
	EventID       uuid.UUID       `json:"event_id"`
	Timestamp     time.Time       `json:"timestamp"`
	TraceID       *string         `json:"trace_id"`
	SpanID        *string         `json:"span_id"`
	ParentSpanID  *string         `json:"parent_span_id"`
	SourceType    string          `json:"source_type"`
	SourceService string          `json:"source_service"`
	Severity      string          `json:"severity"`
	EventName     *string         `json:"event_name"`
	Message       *string         `json:"message"`
	Attributes    json.RawMessage `json:"attributes"`
	Resource      json.RawMessage `json:"resource"`
	DurationMs    *int            `json:"duration_ms"`
	SessionID     *uuid.UUID      `json:"session_id"`
	CreatedAt     time.Time       `json:"created_at"`
}
