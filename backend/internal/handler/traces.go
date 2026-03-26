package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"time"

	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/model"
	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// TraceQuerier abstracts agent event querying for trace lookups.
type TraceQuerier interface {
	Query(ctx context.Context, filters repository.EventFilters) ([]model.AgentEvent, int, error)
}

// SystemTraceQuerier abstracts system event querying by trace ID.
type SystemTraceQuerier interface {
	FindByTraceID(ctx context.Context, traceID string) ([]model.SystemEvent, error)
}

// TraceHandler serves the GET /api/v1/traces/{traceID} endpoint.
type TraceHandler struct {
	agentRepo  TraceQuerier
	systemRepo SystemTraceQuerier
}

// NewTraceHandler creates a TraceHandler with the given dependencies.
func NewTraceHandler(agentRepo TraceQuerier, systemRepo SystemTraceQuerier) *TraceHandler {
	return &TraceHandler{
		agentRepo:  agentRepo,
		systemRepo: systemRepo,
	}
}

// traceSpan represents a unified span in the trace timeline.
type traceSpan struct {
	Source       string          `json:"source"`
	EventID      uuid.UUID       `json:"event_id"`
	Timestamp    time.Time       `json:"timestamp"`
	SpanID       *string         `json:"span_id,omitempty"`
	ParentSpanID *string         `json:"parent_span_id,omitempty"`
	EventType    string          `json:"event_type,omitempty"`
	EventName    *string         `json:"event_name,omitempty"`
	Severity     string          `json:"severity"`
	Message      *string         `json:"message,omitempty"`
	ToolName     *string         `json:"tool_name,omitempty"`
	DurationMs   *int            `json:"duration_ms,omitempty"`
	Details      json.RawMessage `json:"details,omitempty"`
}

// GetTrace handles GET /api/v1/traces/{traceID}.
func (h *TraceHandler) GetTrace(w http.ResponseWriter, r *http.Request) {
	traceID := chi.URLParam(r, "traceID")
	if traceID == "" {
		writeError(w, http.StatusBadRequest, "traceID path parameter is required")
		return
	}

	ctx := r.Context()

	// Query agent events by trace_id.
	agentFilters := repository.EventFilters{
		TraceID: &traceID,
		Limit:   1000,
		Order:   "asc",
	}
	agentEvents, _, err := h.agentRepo.Query(ctx, agentFilters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query agent events: "+err.Error())
		return
	}

	// Query system events by trace_id.
	systemEvents, err := h.systemRepo.FindByTraceID(ctx, traceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query system events: "+err.Error())
		return
	}

	// Merge into unified timeline.
	spans := make([]traceSpan, 0, len(agentEvents)+len(systemEvents))

	for _, e := range agentEvents {
		spans = append(spans, traceSpan{
			Source:       "agent",
			EventID:      e.EventID,
			Timestamp:    e.Timestamp,
			SpanID:       e.SpanID,
			ParentSpanID: e.ParentSpanID,
			EventType:    e.EventType,
			Severity:     e.Severity,
			Message:      e.Message,
			ToolName:     e.ToolName,
			DurationMs:   e.DurationMs,
			Details:      e.Params,
		})
	}

	for _, e := range systemEvents {
		spans = append(spans, traceSpan{
			Source:       "system",
			EventID:      e.EventID,
			Timestamp:    e.Timestamp,
			SpanID:       e.SpanID,
			ParentSpanID: e.ParentSpanID,
			EventName:    e.EventName,
			Severity:     e.Severity,
			Message:      e.Message,
			DurationMs:   e.DurationMs,
			Details:      e.Attributes,
		})
	}

	// Sort by timestamp ascending.
	sort.Slice(spans, func(i, j int) bool {
		return spans[i].Timestamp.Before(spans[j].Timestamp)
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"trace_id":   traceID,
		"span_count": len(spans),
		"spans":      spans,
	})
}
