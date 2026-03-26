package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/model"
	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/repository"
	"github.com/google/uuid"
)

// ── mock LogQuerier ──

type mockLogQuerier struct {
	events       []model.AgentEvent
	total        int
	err          error
	lastFilters  repository.EventFilters
	queryCalled  bool
}

func (m *mockLogQuerier) Query(_ context.Context, filters repository.EventFilters) ([]model.AgentEvent, int, error) {
	m.queryCalled = true
	m.lastFilters = filters
	return m.events, m.total, m.err
}

// ── helpers ──

func doGet(handler http.HandlerFunc, url string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	return rr
}

func decodeBody(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	return body
}

// ── tests ──

func TestQueryLogs_NoFilters(t *testing.T) {
	mock := &mockLogQuerier{
		events: []model.AgentEvent{
			{EventID: uuid.New(), AgentID: "agent-1"},
			{EventID: uuid.New(), AgentID: "agent-2"},
		},
		total: 2,
	}
	h := NewLogHandler(mock)
	rr := doGet(h.QueryLogs, "/api/v1/logs")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	body := decodeBody(t, rr)
	data := body["data"].([]any)
	if len(data) != 2 {
		t.Fatalf("expected 2 events, got %d", len(data))
	}
	if int(body["total_count"].(float64)) != 2 {
		t.Fatalf("expected total_count=2, got %v", body["total_count"])
	}

	// Defaults should be applied.
	if mock.lastFilters.Limit != 50 {
		t.Fatalf("expected default limit=50, got %d", mock.lastFilters.Limit)
	}
	if mock.lastFilters.Order != "desc" {
		t.Fatalf("expected default order=desc, got %q", mock.lastFilters.Order)
	}
}

func TestQueryLogs_WithFilters(t *testing.T) {
	mock := &mockLogQuerier{
		events: []model.AgentEvent{},
		total:  0,
	}
	h := NewLogHandler(mock)

	sid := uuid.New()
	url := "/api/v1/logs?session_id=" + sid.String() +
		"&agent_id=my-agent&event_type=tool_call&severity=error" +
		"&tool_name=read_file&trace_id=abc&text=hello" +
		"&tags=a,b,c&order=asc&limit=10&offset=5"
	rr := doGet(h.QueryLogs, url)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	f := mock.lastFilters
	if f.SessionID == nil || *f.SessionID != sid {
		t.Fatal("session_id not parsed")
	}
	if f.AgentID == nil || *f.AgentID != "my-agent" {
		t.Fatal("agent_id not parsed")
	}
	if f.EventType == nil || *f.EventType != "tool_call" {
		t.Fatal("event_type not parsed")
	}
	if f.Severity == nil || *f.Severity != "error" {
		t.Fatal("severity not parsed")
	}
	if f.ToolName == nil || *f.ToolName != "read_file" {
		t.Fatal("tool_name not parsed")
	}
	if f.TraceID == nil || *f.TraceID != "abc" {
		t.Fatal("trace_id not parsed")
	}
	if f.Text == nil || *f.Text != "hello" {
		t.Fatal("text not parsed")
	}
	if len(f.Tags) != 3 || f.Tags[0] != "a" || f.Tags[1] != "b" || f.Tags[2] != "c" {
		t.Fatalf("tags not parsed correctly: %v", f.Tags)
	}
	if f.Order != "asc" {
		t.Fatalf("expected order=asc, got %q", f.Order)
	}
	if f.Limit != 10 {
		t.Fatalf("expected limit=10, got %d", f.Limit)
	}
	if f.Offset != 5 {
		t.Fatalf("expected offset=5, got %d", f.Offset)
	}
}

func TestQueryLogs_InvalidLimit(t *testing.T) {
	mock := &mockLogQuerier{events: []model.AgentEvent{}, total: 0}
	h := NewLogHandler(mock)

	rr := doGet(h.QueryLogs, "/api/v1/logs?limit=notanumber")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 with default limit, got %d", rr.Code)
	}
	if mock.lastFilters.Limit != 50 {
		t.Fatalf("expected default limit=50 for invalid input, got %d", mock.lastFilters.Limit)
	}
}

func TestQueryLogs_LimitCapped(t *testing.T) {
	mock := &mockLogQuerier{events: []model.AgentEvent{}, total: 0}
	h := NewLogHandler(mock)

	rr := doGet(h.QueryLogs, "/api/v1/logs?limit=5000")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if mock.lastFilters.Limit != 1000 {
		t.Fatalf("expected limit capped to 1000, got %d", mock.lastFilters.Limit)
	}
}

func TestQueryLogs_HasMore(t *testing.T) {
	mock := &mockLogQuerier{
		events: []model.AgentEvent{{EventID: uuid.New()}},
		total:  100,
	}
	h := NewLogHandler(mock)

	rr := doGet(h.QueryLogs, "/api/v1/logs?limit=1")

	body := decodeBody(t, rr)
	if body["has_more"] != true {
		t.Fatal("expected has_more=true")
	}
}

func TestQueryLogs_InvalidSessionID(t *testing.T) {
	mock := &mockLogQuerier{}
	h := NewLogHandler(mock)

	rr := doGet(h.QueryLogs, "/api/v1/logs?session_id=not-a-uuid")

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}
