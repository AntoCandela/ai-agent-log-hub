package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/model"
	"github.com/google/uuid"
)

// ── mocks ──

type mockAgentEnsurer struct {
	called map[string]int
	err    error
}

func (m *mockAgentEnsurer) EnsureExists(_ context.Context, agentID string) error {
	if m.called == nil {
		m.called = make(map[string]int)
	}
	m.called[agentID]++
	return m.err
}

type mockSessionResolver struct {
	sessions map[string]*model.Session
	err      error
}

func (m *mockSessionResolver) ResolveSession(_ context.Context, agentID, sessionToken string, _ *string, _ *string) (*model.Session, error) {
	if m.err != nil {
		return nil, m.err
	}
	key := agentID + "|" + sessionToken
	if s, ok := m.sessions[key]; ok {
		return s, nil
	}
	s := &model.Session{
		SessionID:    uuid.New(),
		SessionToken: sessionToken,
		AgentID:      agentID,
		Status:       "active",
	}
	if m.sessions == nil {
		m.sessions = make(map[string]*model.Session)
	}
	m.sessions[key] = s
	return s, nil
}

type mockEventInserter struct {
	inserted []model.AgentEvent
	err      error
}

func (m *mockEventInserter) InsertBatch(_ context.Context, events []model.AgentEvent) (int, int, error) {
	if m.err != nil {
		return 0, 0, m.err
	}
	m.inserted = append(m.inserted, events...)
	return len(events), 0, nil
}

// ── helpers ──

func newHandler() (*EventHandler, *mockAgentEnsurer, *mockSessionResolver, *mockEventInserter) {
	agents := &mockAgentEnsurer{}
	sessions := &mockSessionResolver{}
	events := &mockEventInserter{}
	h := NewEventHandler(agents, sessions, events)
	return h, agents, sessions, events
}

func doPost(h *EventHandler, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/events", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.IngestEvents(rr, req)
	return rr
}

func validEvent() map[string]any {
	return map[string]any{
		"agent_id":      "test-agent",
		"session_token": "tok-1",
		"timestamp":     time.Now().UTC().Format(time.RFC3339Nano),
		"event_type":    "tool_call",
		"severity":      "info",
	}
}

func toJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// ── tests ──

func TestSingleEventAccepted(t *testing.T) {
	h, _, _, _ := newHandler()
	rr := doPost(h, toJSON(validEvent()))

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(rr.Body.Bytes(), &resp)
	data := resp["data"].(map[string]any)
	if int(data["accepted"].(float64)) != 1 {
		t.Fatalf("expected accepted=1, got %v", data["accepted"])
	}
	if int(data["duplicates"].(float64)) != 0 {
		t.Fatalf("expected duplicates=0, got %v", data["duplicates"])
	}
	if _, err := uuid.Parse(data["session_id"].(string)); err != nil {
		t.Fatalf("invalid session_id in response: %v", err)
	}
}

func TestBatchAccepted(t *testing.T) {
	h, _, _, _ := newHandler()

	batch := []map[string]any{validEvent(), validEvent()}
	batch[1]["agent_id"] = "agent-two"
	rr := doPost(h, toJSON(batch))

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(rr.Body.Bytes(), &resp)
	data := resp["data"].(map[string]any)
	if int(data["accepted"].(float64)) != 2 {
		t.Fatalf("expected accepted=2, got %v", data["accepted"])
	}
}

func TestMissingAgentID(t *testing.T) {
	h, _, _, _ := newHandler()
	e := validEvent()
	delete(e, "agent_id")
	rr := doPost(h, toJSON(e))

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestInvalidEventType(t *testing.T) {
	h, _, _, _ := newHandler()
	e := validEvent()
	e["event_type"] = "invalid_type"
	rr := doPost(h, toJSON(e))

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestEmptyBody(t *testing.T) {
	h, _, _, _ := newHandler()
	rr := doPost(h, "")

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestBatchOver100(t *testing.T) {
	h, _, _, _ := newHandler()

	batch := make([]map[string]any, 101)
	for i := range batch {
		e := validEvent()
		e["agent_id"] = fmt.Sprintf("agent-%d", i)
		batch[i] = e
	}
	rr := doPost(h, toJSON(batch))

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestDefaultSeverity(t *testing.T) {
	h, _, _, evts := newHandler()
	e := validEvent()
	delete(e, "severity")
	rr := doPost(h, toJSON(e))

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(evts.inserted) != 1 {
		t.Fatal("expected 1 event inserted")
	}
	if evts.inserted[0].Severity != "info" {
		t.Fatalf("expected default severity 'info', got %q", evts.inserted[0].Severity)
	}
}

func TestGeneratesEventID(t *testing.T) {
	h, _, _, evts := newHandler()
	e := validEvent()
	delete(e, "event_id")
	rr := doPost(h, toJSON(e))

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	if evts.inserted[0].EventID == uuid.Nil {
		t.Fatal("expected a generated event_id")
	}
}

func TestInvalidTimestamp(t *testing.T) {
	h, _, _, _ := newHandler()
	e := validEvent()
	e["timestamp"] = "not-a-time"
	rr := doPost(h, toJSON(e))

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestFutureTimestamp(t *testing.T) {
	h, _, _, _ := newHandler()
	e := validEvent()
	e["timestamp"] = time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339Nano)
	rr := doPost(h, toJSON(e))

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}
