package otlp

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tobias/ai-agent-log-hub/backend/internal/model"
)

// mockInserter records calls to InsertBatch for assertions.
type mockInserter struct {
	events []model.SystemEvent
	err    error
}

func (m *mockInserter) InsertBatch(_ context.Context, events []model.SystemEvent) (int, error) {
	m.events = append(m.events, events...)
	if m.err != nil {
		return 0, m.err
	}
	return len(events), nil
}

func TestTracesHandler_ValidPayload(t *testing.T) {
	mock := &mockInserter{}
	h := NewOTLPHandler(mock)

	payload := `{
		"resourceSpans": [{
			"resource": {"attributes": [{"key": "service.name", "value": {"stringValue": "my-agent"}}]},
			"scopeSpans": [{
				"spans": [{
					"traceId": "0af7651916cd43dd8448eb211c80319c",
					"spanId": "00f067aa0ba902b7",
					"parentSpanId": "b7ad6b7169203331",
					"name": "HTTP POST /api/nodes",
					"startTimeUnixNano": "1234567890000000000",
					"endTimeUnixNano":   "1234567890100000000",
					"status": {"code": 1},
					"attributes": [{"key": "http.method", "value": {"stringValue": "POST"}}]
				}]
			}]
		}]
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/traces", bytes.NewBufferString(payload))
	w := httptest.NewRecorder()

	h.TracesHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	if len(mock.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(mock.events))
	}

	ev := mock.events[0]

	if ev.SourceService != "my-agent" {
		t.Errorf("expected source_service=my-agent, got %s", ev.SourceService)
	}
	if ev.SourceType != "otlp" {
		t.Errorf("expected source_type=otlp, got %s", ev.SourceType)
	}
	if ev.TraceID == nil || *ev.TraceID != "0af7651916cd43dd8448eb211c80319c" {
		t.Errorf("unexpected trace_id: %v", ev.TraceID)
	}
	if ev.SpanID == nil || *ev.SpanID != "00f067aa0ba902b7" {
		t.Errorf("unexpected span_id: %v", ev.SpanID)
	}
	if ev.ParentSpanID == nil || *ev.ParentSpanID != "b7ad6b7169203331" {
		t.Errorf("unexpected parent_span_id: %v", ev.ParentSpanID)
	}
	if ev.EventName == nil || *ev.EventName != "HTTP POST /api/nodes" {
		t.Errorf("unexpected event_name: %v", ev.EventName)
	}
	if ev.Severity != "info" {
		t.Errorf("expected severity=info for status code 1, got %s", ev.Severity)
	}
	if ev.DurationMs == nil || *ev.DurationMs != 100 {
		t.Errorf("expected duration_ms=100, got %v", ev.DurationMs)
	}
}

func TestTracesHandler_ErrorStatus(t *testing.T) {
	mock := &mockInserter{}
	h := NewOTLPHandler(mock)

	payload := `{
		"resourceSpans": [{
			"resource": {"attributes": []},
			"scopeSpans": [{
				"spans": [{
					"traceId": "0af7651916cd43dd8448eb211c80319c",
					"spanId": "00f067aa0ba902b7",
					"name": "failing-span",
					"startTimeUnixNano": "1000000000000000000",
					"endTimeUnixNano":   "1000000000500000000",
					"status": {"code": 2}
				}]
			}]
		}]
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/traces", bytes.NewBufferString(payload))
	w := httptest.NewRecorder()
	h.TracesHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if len(mock.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(mock.events))
	}
	if mock.events[0].Severity != "error" {
		t.Errorf("expected severity=error for status code 2, got %s", mock.events[0].Severity)
	}
	if mock.events[0].SourceService != "unknown" {
		t.Errorf("expected source_service=unknown, got %s", mock.events[0].SourceService)
	}
	if mock.events[0].DurationMs == nil || *mock.events[0].DurationMs != 500 {
		t.Errorf("expected duration_ms=500, got %v", mock.events[0].DurationMs)
	}
}

func TestLogsHandler_ValidPayload(t *testing.T) {
	mock := &mockInserter{}
	h := NewOTLPHandler(mock)

	payload := `{
		"resourceLogs": [{
			"resource": {"attributes": [{"key": "service.name", "value": {"stringValue": "log-service"}}]},
			"scopeLogs": [{
				"logRecords": [{
					"timeUnixNano": "1234567890000000000",
					"severityNumber": 9,
					"severityText": "WARN",
					"body": {"stringValue": "something happened"},
					"traceId": "0af7651916cd43dd8448eb211c80319c",
					"spanId": "00f067aa0ba902b7",
					"attributes": [{"key": "component", "value": {"stringValue": "auth"}}]
				}]
			}]
		}]
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/logs", bytes.NewBufferString(payload))
	w := httptest.NewRecorder()
	h.LogsHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	if len(mock.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(mock.events))
	}

	ev := mock.events[0]

	if ev.SourceService != "log-service" {
		t.Errorf("expected source_service=log-service, got %s", ev.SourceService)
	}
	if ev.Severity != "warn" {
		t.Errorf("expected severity=warn for severityNumber 9, got %s", ev.Severity)
	}
	if ev.Message == nil || *ev.Message != "something happened" {
		t.Errorf("unexpected message: %v", ev.Message)
	}
	if ev.TraceID == nil || *ev.TraceID != "0af7651916cd43dd8448eb211c80319c" {
		t.Errorf("unexpected trace_id: %v", ev.TraceID)
	}
}

func TestLogsHandler_SeverityMapping(t *testing.T) {
	tests := []struct {
		severityNumber int
		expected       string
	}{
		{1, "debug"},
		{4, "debug"},
		{5, "info"},
		{8, "info"},
		{9, "warn"},
		{12, "warn"},
		{13, "error"},
		{16, "error"},
		{17, "fatal"},
		{21, "fatal"},
		{0, "info"}, // unset defaults to info
	}

	for _, tt := range tests {
		got := logSeverityNumberToString(tt.severityNumber)
		if got != tt.expected {
			t.Errorf("severityNumber %d: expected %s, got %s", tt.severityNumber, tt.expected, got)
		}
	}
}

func TestTracesHandler_EmptyBody(t *testing.T) {
	mock := &mockInserter{}
	h := NewOTLPHandler(mock)

	req := httptest.NewRequest(http.MethodPost, "/v1/traces", bytes.NewBufferString(""))
	w := httptest.NewRecorder()
	h.TracesHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for empty body, got %d", w.Code)
	}
	if len(mock.events) != 0 {
		t.Errorf("expected 0 events for empty body, got %d", len(mock.events))
	}
}

func TestLogsHandler_EmptyBody(t *testing.T) {
	mock := &mockInserter{}
	h := NewOTLPHandler(mock)

	req := httptest.NewRequest(http.MethodPost, "/v1/logs", bytes.NewBufferString(""))
	w := httptest.NewRecorder()
	h.LogsHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for empty body, got %d", w.Code)
	}
	if len(mock.events) != 0 {
		t.Errorf("expected 0 events for empty body, got %d", len(mock.events))
	}
}

func TestTracesHandler_InvalidJSON(t *testing.T) {
	mock := &mockInserter{}
	h := NewOTLPHandler(mock)

	req := httptest.NewRequest(http.MethodPost, "/v1/traces", bytes.NewBufferString("{not json"))
	w := httptest.NewRecorder()
	h.TracesHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid JSON, got %d", w.Code)
	}
}

func TestLogsHandler_InvalidJSON(t *testing.T) {
	mock := &mockInserter{}
	h := NewOTLPHandler(mock)

	req := httptest.NewRequest(http.MethodPost, "/v1/logs", bytes.NewBufferString("{not json"))
	w := httptest.NewRecorder()
	h.LogsHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid JSON, got %d", w.Code)
	}
}

func TestTracesHandler_EmptyResourceSpans(t *testing.T) {
	mock := &mockInserter{}
	h := NewOTLPHandler(mock)

	req := httptest.NewRequest(http.MethodPost, "/v1/traces", bytes.NewBufferString(`{"resourceSpans":[]}`))
	w := httptest.NewRecorder()
	h.TracesHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if len(mock.events) != 0 {
		t.Errorf("expected 0 events, got %d", len(mock.events))
	}
}
