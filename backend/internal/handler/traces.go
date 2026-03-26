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
// Traces cut across sessions -- a single trace_id can appear in events
// from multiple agents, so this query is not scoped to a single session.
type TraceQuerier interface {
	Query(ctx context.Context, filters repository.EventFilters) ([]model.AgentEvent, int, error)
}

// SystemTraceQuerier abstracts system event querying by trace ID.
// System events live in a separate table, so they need their own
// lookup method rather than sharing the agent event query.
type SystemTraceQuerier interface {
	FindByTraceID(ctx context.Context, traceID string) ([]model.SystemEvent, error)
}

// TraceHandler serves the GET /api/v1/traces/{traceID} endpoint.
// It merges events from two different "layers" (agent and system) into
// a single unified timeline, sorted by timestamp.
type TraceHandler struct {
	agentRepo  TraceQuerier
	systemRepo SystemTraceQuerier
}

// NewTraceHandler creates a TraceHandler with the given dependencies.
// Both repositories are required to build the full cross-layer trace view.
func NewTraceHandler(agentRepo TraceQuerier, systemRepo SystemTraceQuerier) *TraceHandler {
	return &TraceHandler{
		agentRepo:  agentRepo,
		systemRepo: systemRepo,
	}
}

// traceSpan represents a unified span in the trace timeline.
// It normalizes both agent events and system events into one shape so the
// frontend can render a single timeline without caring about the source.
// The "Source" field ("agent" or "system") tells the frontend which layer
// the span came from.
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
//
// Cross-layer trace merging:
//  1. Fetch all agent events that share this trace_id (up to 1000).
//  2. Fetch all system events that share this trace_id.
//  3. Convert both sets into a common traceSpan struct.
//     - Agent events populate EventType, ToolName, and use Params as Details.
//     - System events populate EventName and use Attributes as Details.
//  4. Merge the two slices and sort by timestamp ascending so the frontend
//     receives a chronologically ordered timeline.
//  5. Return the merged list along with the trace_id and total span count.
func (h *TraceHandler) GetTrace(w http.ResponseWriter, r *http.Request) {
	traceID := chi.URLParam(r, "traceID")
	if traceID == "" {
		writeError(w, http.StatusBadRequest, "traceID path parameter is required")
		return
	}

	ctx := r.Context()

	// Step 1: Query agent events by trace_id.
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

	// Step 2: Query system events by trace_id.
	systemEvents, err := h.systemRepo.FindByTraceID(ctx, traceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query system events: "+err.Error())
		return
	}

	// Step 3: Merge into a unified timeline by converting both types to traceSpan.
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

	// Step 4: Sort by timestamp ascending for chronological order.
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
