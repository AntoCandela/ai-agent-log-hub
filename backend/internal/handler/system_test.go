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

type mockSystemQuerier struct {
	events []model.SystemEvent
	total  int
	err    error
	calls  []repository.SystemEventFilters
}

func (m *mockSystemQuerier) Query(_ context.Context, filters repository.SystemEventFilters) ([]model.SystemEvent, int, error) {
	m.calls = append(m.calls, filters)
	if m.err != nil {
		return nil, 0, m.err
	}
	return m.events, m.total, nil
}

// ── tests ──

func TestQuerySystem_DefaultParams(t *testing.T) {
	mock := &mockSystemQuerier{
		events: []model.SystemEvent{},
		total:  0,
	}
	h := NewSystemHandler(mock)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/system", nil)
	rr := httptest.NewRecorder()
	h.QuerySystem(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["total_count"].(float64) != 0 {
		t.Fatalf("expected total_count=0, got %v", resp["total_count"])
	}
}

func TestQuerySystem_WithFilters(t *testing.T) {
	eventName := "db.query"
	now := time.Now().UTC()

	mock := &mockSystemQuerier{
		events: []model.SystemEvent{
			{
				EventID:       uuid.New(),
				Timestamp:     now,
				SourceType:    "span",
				SourceService: "postgres",
				Severity:      "info",
				EventName:     &eventName,
			},
		},
		total: 1,
	}
	h := NewSystemHandler(mock)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/system?severity=info&source_service=postgres&event_name=db.query&order=asc&limit=10", nil)
	rr := httptest.NewRecorder()
	h.QuerySystem(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	if len(mock.calls) == 0 {
		t.Fatal("expected at least one query call")
	}
	f := mock.calls[0]
	if f.Severity == nil || *f.Severity != "info" {
		t.Fatalf("expected severity=info filter")
	}
	if f.SourceService == nil || *f.SourceService != "postgres" {
		t.Fatalf("expected source_service=postgres filter")
	}
	if f.EventName == nil || *f.EventName != "db.query" {
		t.Fatalf("expected event_name=db.query filter")
	}
	if f.Order != "asc" {
		t.Fatalf("expected order=asc, got %q", f.Order)
	}
	if f.Limit != 10 {
		t.Fatalf("expected limit=10, got %d", f.Limit)
	}

	var resp map[string]any
	json.Unmarshal(rr.Body.Bytes(), &resp)
	data := resp["data"].([]any)
	if len(data) != 1 {
		t.Fatalf("expected 1 event, got %d", len(data))
	}
}

func TestQuerySystem_InvalidSessionID(t *testing.T) {
	mock := &mockSystemQuerier{}
	h := NewSystemHandler(mock)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/system?session_id=not-a-uuid", nil)
	rr := httptest.NewRecorder()
	h.QuerySystem(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestQuerySystem_HasMore(t *testing.T) {
	mock := &mockSystemQuerier{
		events: []model.SystemEvent{
			{EventID: uuid.New(), Timestamp: time.Now(), SourceType: "span", SourceService: "svc", Severity: "info"},
		},
		total: 10,
	}
	h := NewSystemHandler(mock)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/system?limit=1", nil)
	rr := httptest.NewRecorder()
	h.QuerySystem(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["has_more"] != true {
		t.Fatalf("expected has_more=true, got %v", resp["has_more"])
	}
}
