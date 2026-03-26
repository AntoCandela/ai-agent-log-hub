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
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ── mock SessionLister ──

type mockSessionLister struct {
	sessions    []*model.Session
	total       int
	session     *model.Session
	err         error
	lastFilters repository.SessionFilters
	lastLimit   int
	lastOffset  int
}

func (m *mockSessionLister) List(_ context.Context, filters repository.SessionFilters, limit, offset int) (*repository.SessionListResult, error) {
	m.lastFilters = filters
	m.lastLimit = limit
	m.lastOffset = offset
	if m.err != nil {
		return nil, m.err
	}
	return &repository.SessionListResult{Sessions: m.sessions, Total: m.total}, nil
}

func (m *mockSessionLister) GetByID(_ context.Context, _ uuid.UUID) (*model.Session, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.session, nil
}

// ── helpers ──

func newTestSession() *model.Session {
	return &model.Session{
		SessionID:   uuid.New(),
		AgentID:     "test-agent",
		Status:      "active",
		StartedAt:   time.Now().UTC(),
		LastEventAt: time.Now().UTC(),
		EventCount:  5,
	}
}

// doGetWithChi sends a GET through a chi router so URL params are populated.
func doGetWithChi(pattern, url string, handlerFn http.HandlerFunc) *httptest.ResponseRecorder {
	r := chi.NewRouter()
	r.Get(pattern, handlerFn)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	return rr
}

// ── tests ──

func TestListSessions(t *testing.T) {
	s1 := newTestSession()
	s2 := newTestSession()
	s2.AgentID = "agent-2"

	mock := &mockSessionLister{
		sessions: []*model.Session{s1, s2},
		total:    2,
	}
	h := NewSessionHandler(mock, &mockLogQuerier{})

	rr := doGet(h.ListSessions, "/api/v1/sessions")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var body map[string]any
	json.Unmarshal(rr.Body.Bytes(), &body)
	data := body["data"].([]any)
	if len(data) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(data))
	}
	if int(body["total_count"].(float64)) != 2 {
		t.Fatalf("expected total_count=2, got %v", body["total_count"])
	}
}

func TestListSessions_WithFilters(t *testing.T) {
	mock := &mockSessionLister{
		sessions: []*model.Session{},
		total:    0,
	}
	h := NewSessionHandler(mock, &mockLogQuerier{})

	rr := doGet(h.ListSessions, "/api/v1/sessions?agent_id=my-agent&status=active&pinned=true&limit=10&offset=5")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	if mock.lastFilters.AgentID != "my-agent" {
		t.Fatalf("expected agent_id=my-agent, got %q", mock.lastFilters.AgentID)
	}
	if mock.lastFilters.Status != "active" {
		t.Fatalf("expected status=active, got %q", mock.lastFilters.Status)
	}
	if mock.lastFilters.Pinned == nil || *mock.lastFilters.Pinned != true {
		t.Fatal("expected pinned=true")
	}
	if mock.lastLimit != 10 {
		t.Fatalf("expected limit=10, got %d", mock.lastLimit)
	}
	if mock.lastOffset != 5 {
		t.Fatalf("expected offset=5, got %d", mock.lastOffset)
	}
}

func TestGetSession(t *testing.T) {
	sess := newTestSession()
	mock := &mockSessionLister{session: sess}
	h := NewSessionHandler(mock, &mockLogQuerier{})

	rr := doGetWithChi(
		"/api/v1/sessions/{sessionID}",
		"/api/v1/sessions/"+sess.SessionID.String(),
		h.GetSession,
	)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var body map[string]any
	json.Unmarshal(rr.Body.Bytes(), &body)
	data := body["data"].(map[string]any)
	if data["session_id"] != sess.SessionID.String() {
		t.Fatalf("expected session_id=%s, got %v", sess.SessionID, data["session_id"])
	}
}

func TestGetSession_BadUUID(t *testing.T) {
	mock := &mockSessionLister{}
	h := NewSessionHandler(mock, &mockLogQuerier{})

	rr := doGetWithChi(
		"/api/v1/sessions/{sessionID}",
		"/api/v1/sessions/not-a-uuid",
		h.GetSession,
	)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestGetSession_NotFound(t *testing.T) {
	mock := &mockSessionLister{session: nil}
	h := NewSessionHandler(mock, &mockLogQuerier{})

	rr := doGetWithChi(
		"/api/v1/sessions/{sessionID}",
		"/api/v1/sessions/"+uuid.New().String(),
		h.GetSession,
	)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestGetSessionSummary(t *testing.T) {
	sess := newTestSession()
	mock := &mockSessionLister{session: sess}
	h := NewSessionHandler(mock, &mockLogQuerier{})

	rr := doGetWithChi(
		"/api/v1/sessions/{sessionID}/summary",
		"/api/v1/sessions/"+sess.SessionID.String()+"/summary",
		h.GetSessionSummary,
	)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var body map[string]any
	json.Unmarshal(rr.Body.Bytes(), &body)
	data := body["data"].(map[string]any)
	if int(data["event_count"].(float64)) != sess.EventCount {
		t.Fatalf("expected event_count=%d, got %v", sess.EventCount, data["event_count"])
	}
}

func TestGetSessionFiles(t *testing.T) {
	sess := newTestSession()
	sessionMock := &mockSessionLister{session: sess}
	eventMock := &mockLogQuerier{
		events: []model.AgentEvent{
			{
				EventID:   uuid.New(),
				EventType: "tool_call",
				Params:    json.RawMessage(`{"file_path":"src/main.go"}`),
			},
			{
				EventID:   uuid.New(),
				EventType: "tool_call",
				Params:    json.RawMessage(`{"file_path":"src/main.go"}`),
			},
			{
				EventID:   uuid.New(),
				EventType: "tool_call",
				Params:    json.RawMessage(`{"file_path":"src/util.go"}`),
			},
		},
		total: 3,
	}
	h := NewSessionHandler(sessionMock, eventMock)

	rr := doGetWithChi(
		"/api/v1/sessions/{sessionID}/files",
		"/api/v1/sessions/"+sess.SessionID.String()+"/files",
		h.GetSessionFiles,
	)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var body map[string]any
	json.Unmarshal(rr.Body.Bytes(), &body)
	data := body["data"].([]any)
	if len(data) != 2 {
		t.Fatalf("expected 2 file entries, got %d", len(data))
	}

	// Check that touches are correct.
	fileMap := make(map[string]int)
	for _, item := range data {
		f := item.(map[string]any)
		fileMap[f["file_path"].(string)] = int(f["touches"].(float64))
	}
	if fileMap["src/main.go"] != 2 {
		t.Fatalf("expected src/main.go touches=2, got %d", fileMap["src/main.go"])
	}
	if fileMap["src/util.go"] != 1 {
		t.Fatalf("expected src/util.go touches=1, got %d", fileMap["src/util.go"])
	}
}
