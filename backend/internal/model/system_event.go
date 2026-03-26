package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// SystemEvent represents a telemetry event originating from
// infrastructure or external services rather than from an AI agent
// directly. Examples include HTTP server spans, database query traces,
// and container runtime logs received via the OTLP endpoints.
//
// System events can be correlated with agent sessions through their
// TraceID — a background worker periodically matches system events to
// the session whose agent events share the same trace.
type SystemEvent struct {
	// EventID is the primary key (UUID).
	EventID uuid.UUID `json:"event_id"`
	// Timestamp is when the event was recorded by the source system.
	Timestamp time.Time `json:"timestamp"`
	// TraceID is the distributed-tracing trace identifier, used to link
	// this system event to agent events in the same trace.
	TraceID *string `json:"trace_id"`
	// SpanID identifies this specific span in the trace.
	SpanID *string `json:"span_id"`
	// ParentSpanID links to the parent span in the trace tree.
	ParentSpanID *string `json:"parent_span_id"`
	// SourceType describes the kind of source (e.g. "trace", "log", "metric").
	SourceType string `json:"source_type"`
	// SourceService is the name of the service that emitted this event
	// (e.g. "nginx", "postgres", "redis").
	SourceService string `json:"source_service"`
	// Severity is the log level: "info", "warn", "error", etc.
	Severity string `json:"severity"`
	// EventName is an optional label describing the event
	// (e.g. "HTTP GET /api/v1/logs").
	EventName *string `json:"event_name"`
	// Message is a human-readable description of the event.
	Message *string `json:"message"`
	// Attributes holds OTLP span/log attributes as raw JSON. Using
	// json.RawMessage keeps the schema flexible since different sources
	// send different attribute sets.
	Attributes json.RawMessage `json:"attributes"`
	// Resource holds OTLP resource attributes as raw JSON (e.g. host name,
	// container ID, SDK version).
	Resource json.RawMessage `json:"resource"`
	// DurationMs is the span duration in milliseconds, if applicable.
	DurationMs *int `json:"duration_ms"`
	// SessionID links this system event to an agent session, once the
	// background correlation worker has matched them via TraceID.
	// Nil until correlation has run.
	SessionID *uuid.UUID `json:"session_id"`
	// CreatedAt is when this row was inserted into the database.
	CreatedAt time.Time `json:"created_at"`
}
