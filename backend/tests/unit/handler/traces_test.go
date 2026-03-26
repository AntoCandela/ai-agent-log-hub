package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/handler"
	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/model"
	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ── mocks ──

type mockTraceQuerier struct {
	events []model.AgentEvent
	total  int
	err    error
}

func (m *mockTraceQuerier) Query(_ context.Context, filters repository.EventFilters) ([]model.AgentEvent, int, error) {
	_ = filters
	if m.err != nil {
		return nil, 0, m.err
	}
	return m.events, m.total, nil
}

type mockSystemTraceQuerier struct {
	events []model.SystemEvent
	err    error
}

func (m *mockSystemTraceQuerier) FindByTraceID(_ context.Context, traceID string) ([]model.SystemEvent, error) {
	_ = traceID
	if m.err != nil {
		return nil, m.err
	}
	return m.events, nil
}

// ── helpers ──

func traceRequest(traceID string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/traces/"+traceID, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("traceID", traceID)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

// ── tests ──

func TestGetTrace_MergesEvents(t *testing.T) {
	traceID := "trace-abc-123"
	now := time.Now().UTC()

	agentMock := &mockTraceQuerier{
		events: []model.AgentEvent{
			{
				EventID:   uuid.New(),
				SessionID: uuid.New(),
				AgentID:   "agent-1",
				TraceID:   &traceID,
				Timestamp: now,
				EventType: "tool_call",
				Severity:  "info",
			},
			{
				EventID:   uuid.New(),
				SessionID: uuid.New(),
				AgentID:   "agent-1",
				TraceID:   &traceID,
				Timestamp: now.Add(2 * time.Second),
				EventType: "tool_call",
				Severity:  "info",
			},
		},
		total: 2,
	}

	eventName := "db.query"
	systemMock := &mockSystemTraceQuerier{
		events: []model.SystemEvent{
			{
				EventID:       uuid.New(),
				Timestamp:     now.Add(1 * time.Second),
				TraceID:       &traceID,
				SourceType:    "span",
				SourceService: "postgres",
				Severity:      "info",
				EventName:     &eventName,
			},
		},
	}

	h := handler.NewTraceHandler(agentMock, systemMock)

	req := traceRequest(traceID)
	rr := httptest.NewRecorder()
	h.GetTrace(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(rr.Body.Bytes(), &resp)

	if resp["trace_id"] != traceID {
		t.Fatalf("expected trace_id=%q, got %v", traceID, resp["trace_id"])
	}

	spanCount := int(resp["span_count"].(float64))
	if spanCount != 3 {
		t.Fatalf("expected 3 spans, got %d", spanCount)
	}

	spans := resp["spans"].([]any)
	// Verify ordering: agent(t0), system(t0+1s), agent(t0+2s).
	first := spans[0].(map[string]any)
	second := spans[1].(map[string]any)
	third := spans[2].(map[string]any)

	if first["source"] != "agent" {
		t.Fatalf("expected first span source=agent, got %v", first["source"])
	}
	if second["source"] != "system" {
		t.Fatalf("expected second span source=system, got %v", second["source"])
	}
	if third["source"] != "agent" {
		t.Fatalf("expected third span source=agent, got %v", third["source"])
	}
}

func TestGetTrace_EmptyTrace(t *testing.T) {
	agentMock := &mockTraceQuerier{events: []model.AgentEvent{}, total: 0}
	systemMock := &mockSystemTraceQuerier{events: []model.SystemEvent{}}

	h := handler.NewTraceHandler(agentMock, systemMock)
	req := traceRequest("trace-empty")
	rr := httptest.NewRecorder()
	h.GetTrace(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(rr.Body.Bytes(), &resp)

	spanCount := int(resp["span_count"].(float64))
	if spanCount != 0 {
		t.Fatalf("expected 0 spans, got %d", spanCount)
	}
}

func TestGetTrace_MissingTraceID(t *testing.T) {
	agentMock := &mockTraceQuerier{}
	systemMock := &mockSystemTraceQuerier{}

	h := handler.NewTraceHandler(agentMock, systemMock)

	// Request without chi route context (empty traceID).
	req := httptest.NewRequest(http.MethodGet, "/api/v1/traces/", nil)
	rctx := chi.NewRouteContext()
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	h.GetTrace(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}
