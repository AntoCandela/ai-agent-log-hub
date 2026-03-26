package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/model"
	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/repository"
	"github.com/google/uuid"
)

// ── mock ──

type mockBlameQuerier struct {
	events []model.AgentEvent
	total  int
	err    error
	calls  []repository.EventFilters
}

func (m *mockBlameQuerier) Query(_ context.Context, filters repository.EventFilters) ([]model.AgentEvent, int, error) {
	m.calls = append(m.calls, filters)
	if m.err != nil {
		return nil, 0, m.err
	}
	return m.events, m.total, nil
}

// ── tests ──

func TestGetBlame_MissingFile(t *testing.T) {
	h := NewBlameHandler(&mockBlameQuerier{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/blame", nil)
	rr := httptest.NewRecorder()
	h.GetBlame(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestGetBlame_ReturnsModifications(t *testing.T) {
	toolName := "write_file"
	msg := "Updated main.go"
	now := time.Now().UTC()

	mock := &mockBlameQuerier{
		events: []model.AgentEvent{
			{
				EventID:   uuid.New(),
				SessionID: uuid.New(),
				AgentID:   "agent-1",
				Timestamp: now,
				EventType: "file_change",
				Severity:  "info",
				ToolName:  &toolName,
				Message:   &msg,
			},
			{
				EventID:   uuid.New(),
				SessionID: uuid.New(),
				AgentID:   "agent-2",
				Timestamp: now.Add(-time.Hour),
				EventType: "file_change",
				Severity:  "info",
				ToolName:  &toolName,
				Message:   &msg,
			},
		},
		total: 2,
	}

	h := NewBlameHandler(mock)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/blame?file=src/main.go", nil)
	rr := httptest.NewRecorder()
	h.GetBlame(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(rr.Body.Bytes(), &resp)

	data := resp["data"].([]any)
	if len(data) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(data))
	}

	// Verify file_path filter was set.
	if len(mock.calls) == 0 {
		t.Fatal("expected at least one query call")
	}
	if mock.calls[0].FilePath == nil || *mock.calls[0].FilePath != "src/main.go" {
		t.Fatalf("expected file_path filter 'src/main.go', got %v", mock.calls[0].FilePath)
	}

	// Verify order is desc.
	if mock.calls[0].Order != "desc" {
		t.Fatalf("expected order 'desc', got %q", mock.calls[0].Order)
	}

	// Verify response contains file field.
	if resp["file"] != "src/main.go" {
		t.Fatalf("expected file='src/main.go', got %v", resp["file"])
	}

	// Check entry fields.
	entry := data[0].(map[string]any)
	if entry["agent_id"] != "agent-1" {
		t.Fatalf("expected agent_id='agent-1', got %v", entry["agent_id"])
	}
}

func TestGetBlame_CustomDepth(t *testing.T) {
	mock := &mockBlameQuerier{
		events: []model.AgentEvent{},
		total:  0,
	}

	h := NewBlameHandler(mock)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/blame?file=src/main.go&depth=3", nil)
	rr := httptest.NewRecorder()
	h.GetBlame(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	if len(mock.calls) == 0 {
		t.Fatal("expected at least one query call")
	}
	if mock.calls[0].Limit != 3 {
		t.Fatalf("expected limit=3, got %d", mock.calls[0].Limit)
	}
}
