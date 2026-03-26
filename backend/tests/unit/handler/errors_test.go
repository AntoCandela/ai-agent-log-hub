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
	"github.com/google/uuid"
)

// ── mock ──

type mockErrorSearcher struct {
	events []model.AgentEvent
	total  int
	err    error
	calls  []repository.EventFilters
}

func (m *mockErrorSearcher) Query(_ context.Context, filters repository.EventFilters) ([]model.AgentEvent, int, error) {
	m.calls = append(m.calls, filters)
	if m.err != nil {
		return nil, 0, m.err
	}
	return m.events, m.total, nil
}

// ── tests ──

func TestSearchErrors_MissingPattern(t *testing.T) {
	h := handler.NewErrorHandler(&mockErrorSearcher{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/errors", nil)
	rr := httptest.NewRecorder()
	h.SearchErrors(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestSearchErrors_ReturnsResults(t *testing.T) {
	sessionID := uuid.New()
	now := time.Now().UTC()
	errorEvent := model.AgentEvent{
		EventID:   uuid.New(),
		SessionID: sessionID,
		AgentID:   "agent-1",
		Timestamp: now,
		EventType: "error",
		Severity:  "error",
	}

	// Context events: 3 before, the error, 3 after.
	contextEvents := []model.AgentEvent{
		{EventID: uuid.New(), SessionID: sessionID, AgentID: "agent-1", Timestamp: now.Add(-3 * time.Second), EventType: "tool_call", Severity: "info"},
		{EventID: uuid.New(), SessionID: sessionID, AgentID: "agent-1", Timestamp: now.Add(-2 * time.Second), EventType: "tool_call", Severity: "info"},
		{EventID: uuid.New(), SessionID: sessionID, AgentID: "agent-1", Timestamp: now.Add(-1 * time.Second), EventType: "tool_call", Severity: "info"},
		errorEvent,
		{EventID: uuid.New(), SessionID: sessionID, AgentID: "agent-1", Timestamp: now.Add(1 * time.Second), EventType: "tool_call", Severity: "info"},
		{EventID: uuid.New(), SessionID: sessionID, AgentID: "agent-1", Timestamp: now.Add(2 * time.Second), EventType: "tool_call", Severity: "info"},
		{EventID: uuid.New(), SessionID: sessionID, AgentID: "agent-1", Timestamp: now.Add(3 * time.Second), EventType: "tool_call", Severity: "info"},
	}

	callCount := 0

	// We need a smarter mock that returns different results based on filter.
	smartMock := &smartErrorMock{
		errorEvents: []model.AgentEvent{errorEvent},
		contextEvts: contextEvents,
		callCount:   &callCount,
	}
	h := handler.NewErrorHandler(smartMock)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/errors?pattern=crash", nil)
	rr := httptest.NewRecorder()
	h.SearchErrors(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(rr.Body.Bytes(), &resp)

	data := resp["data"].([]any)
	if len(data) != 1 {
		t.Fatalf("expected 1 error result, got %d", len(data))
	}

	entry := data[0].(map[string]any)
	before := entry["before"].([]any)
	after := entry["after"].([]any)
	if len(before) != 3 {
		t.Fatalf("expected 3 before events, got %d", len(before))
	}
	if len(after) != 3 {
		t.Fatalf("expected 3 after events, got %d", len(after))
	}
}

// smartErrorMock returns error events on the first call (severity=error filter)
// and context events on subsequent calls.
type smartErrorMock struct {
	errorEvents []model.AgentEvent
	contextEvts []model.AgentEvent
	callCount   *int
}

func (m *smartErrorMock) Query(_ context.Context, filters repository.EventFilters) ([]model.AgentEvent, int, error) {
	*m.callCount++
	if filters.Severity != nil && *filters.Severity == "error" {
		return m.errorEvents, len(m.errorEvents), nil
	}
	// Context query.
	return m.contextEvts, len(m.contextEvts), nil
}

func TestSearchErrors_WithAgentFilter(t *testing.T) {
	mock := &mockErrorSearcher{
		events: []model.AgentEvent{},
		total:  0,
	}
	h := handler.NewErrorHandler(mock)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/errors?pattern=timeout&agent_id=agent-x", nil)
	rr := httptest.NewRecorder()
	h.SearchErrors(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify the agent_id filter was passed.
	if len(mock.calls) == 0 {
		t.Fatal("expected at least one query call")
	}
	if mock.calls[0].AgentID == nil || *mock.calls[0].AgentID != "agent-x" {
		t.Fatalf("expected agent_id filter 'agent-x', got %v", mock.calls[0].AgentID)
	}
}
