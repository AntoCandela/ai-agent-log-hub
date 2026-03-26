package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/model"
	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/repository"
	"github.com/google/uuid"
)

// ── mocks ──

type mockSummaryEventRepo struct {
	events []model.AgentEvent
	total  int
	err    error
}

func (m *mockSummaryEventRepo) Query(_ context.Context, _ repository.EventFilters) ([]model.AgentEvent, int, error) {
	return m.events, m.total, m.err
}

type mockSummaryStore struct {
	created *model.SessionSummary
	err     error
}

func (m *mockSummaryStore) Create(_ context.Context, s *model.SessionSummary) error {
	if m.err != nil {
		return m.err
	}
	s.ID = uuid.New()
	s.CreatedAt = time.Now()
	m.created = s
	return nil
}

type mockEmbeddingStore struct {
	stored     bool
	sourceType string
	content    string
	err        error
}

func (m *mockEmbeddingStore) Store(_ context.Context, sourceType string, _ uuid.UUID, _, content string, _ []float32, _ bool) error {
	m.stored = true
	m.sourceType = sourceType
	m.content = content
	return m.err
}

type mockEmbedder struct {
	vec []float32
	err error
}

func (m *mockEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
	return m.vec, m.err
}

func (m *mockEmbedder) EmbedBatch(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = m.vec
	}
	return out, m.err
}

func (m *mockEmbedder) Dimensions() int { return len(m.vec) }

// ── helpers ──

func strPtr(s string) *string { return &s }

func makeEvents() []model.AgentEvent {
	sid := uuid.New()
	now := time.Now().UTC()

	return []model.AgentEvent{
		{
			EventID:   uuid.New(),
			SessionID: sid,
			AgentID:   "test-agent",
			Timestamp: now,
			EventType: "tool_call",
			Severity:  "info",
			ToolName:  strPtr("file_edit"),
			Params:    json.RawMessage(`{"file_path":"src/main.go","command":"edit"}`),
		},
		{
			EventID:   uuid.New(),
			SessionID: sid,
			AgentID:   "test-agent",
			Timestamp: now.Add(10 * time.Second),
			EventType: "tool_call",
			Severity:  "info",
			ToolName:  strPtr("file_edit"),
			Params:    json.RawMessage(`{"file_path":"src/main.go"}`),
		},
		{
			EventID:   uuid.New(),
			SessionID: sid,
			AgentID:   "test-agent",
			Timestamp: now.Add(20 * time.Second),
			EventType: "tool_call",
			Severity:  "info",
			ToolName:  strPtr("bash"),
			Params:    json.RawMessage(`{"command":"go test ./..."}`),
		},
		{
			EventID:   uuid.New(),
			SessionID: sid,
			AgentID:   "test-agent",
			Timestamp: now.Add(30 * time.Second),
			EventType: "git_commit",
			Severity:  "info",
			Params:    json.RawMessage(`{"hash":"abc123","message":"fix bug"}`),
		},
		{
			EventID:   uuid.New(),
			SessionID: sid,
			AgentID:   "test-agent",
			Timestamp: now.Add(40 * time.Second),
			EventType: "tool_call",
			Severity:  "error",
			ToolName:  strPtr("bash"),
			Message:   strPtr("command failed: exit 1"),
			Params:    json.RawMessage(`{}`),
		},
		{
			EventID:   uuid.New(),
			SessionID: sid,
			AgentID:   "test-agent",
			Timestamp: now.Add(50 * time.Second),
			EventType: "tool_call",
			Severity:  "info",
			ToolName:  strPtr("file_read"),
			Params:    json.RawMessage(`{"file_path":"src/util.go"}`),
		},
	}
}

// ── tests ──

func TestGenerateForSession_Aggregation(t *testing.T) {
	events := makeEvents()
	now := time.Now().UTC()
	endedAt := now.Add(5 * time.Minute)

	session := &model.Session{
		SessionID: events[0].SessionID,
		AgentID:   "test-agent",
		Status:    "closed",
		StartedAt: now,
		EndedAt:   &endedAt,
	}

	eventRepo := &mockSummaryEventRepo{events: events, total: len(events)}
	summaryStore := &mockSummaryStore{}
	embedStore := &mockEmbeddingStore{}
	embedder := &mockEmbedder{vec: []float32{0.1, 0.2, 0.3}}

	svc := NewSummaryService(summaryStore, eventRepo, embedStore, embedder)
	err := svc.GenerateForSession(context.Background(), session)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify summary was stored.
	s := summaryStore.created
	if s == nil {
		t.Fatal("expected summary to be created")
	}

	// Check event count.
	if s.EventCount != len(events) {
		t.Errorf("expected EventCount=%d, got %d", len(events), s.EventCount)
	}

	// Check duration (should be 5 minutes = 300 seconds).
	if s.DurationSeconds != 300 {
		t.Errorf("expected DurationSeconds=300, got %d", s.DurationSeconds)
	}

	// Check files modified.
	var files []fileInfo
	if err := json.Unmarshal(s.FilesModified, &files); err != nil {
		t.Fatalf("failed to unmarshal files_modified: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("expected 2 files modified, got %d", len(files))
	}
	// src/main.go should have 2 changes (highest first due to sorting).
	if len(files) > 0 && files[0].FilePath != "src/main.go" {
		t.Errorf("expected first file to be src/main.go, got %s", files[0].FilePath)
	}
	if len(files) > 0 && files[0].Changes != 2 {
		t.Errorf("expected src/main.go changes=2, got %d", files[0].Changes)
	}

	// Check tools used.
	var tools []toolInfo
	if err := json.Unmarshal(s.ToolsUsed, &tools); err != nil {
		t.Fatalf("failed to unmarshal tools_used: %v", err)
	}
	if len(tools) != 3 {
		t.Errorf("expected 3 tools, got %d", len(tools))
	}
	// file_edit should be first (count=2).
	toolMap := make(map[string]int)
	for _, tool := range tools {
		toolMap[tool.ToolName] = tool.Count
	}
	if toolMap["file_edit"] != 2 {
		t.Errorf("expected file_edit count=2, got %d", toolMap["file_edit"])
	}
	if toolMap["bash"] != 2 {
		t.Errorf("expected bash count=2, got %d", toolMap["bash"])
	}
	if toolMap["file_read"] != 1 {
		t.Errorf("expected file_read count=1, got %d", toolMap["file_read"])
	}

	// Check commits.
	var commits []commitInfo
	if err := json.Unmarshal(s.Commits, &commits); err != nil {
		t.Fatalf("failed to unmarshal commits: %v", err)
	}
	if len(commits) != 1 {
		t.Errorf("expected 1 commit, got %d", len(commits))
	}
	if len(commits) > 0 {
		if commits[0].Hash != "abc123" {
			t.Errorf("expected commit hash=abc123, got %s", commits[0].Hash)
		}
		if commits[0].Message != "fix bug" {
			t.Errorf("expected commit message='fix bug', got %s", commits[0].Message)
		}
	}

	// Check errors.
	var errs []errorInfo
	if err := json.Unmarshal(s.Errors, &errs); err != nil {
		t.Fatalf("failed to unmarshal errors: %v", err)
	}
	if len(errs) != 1 {
		t.Errorf("expected 1 error, got %d", len(errs))
	}
	if len(errs) > 0 && errs[0].Message != "command failed: exit 1" {
		t.Errorf("expected error message='command failed: exit 1', got %s", errs[0].Message)
	}

	// Check summary text.
	if s.SummaryText == nil {
		t.Fatal("expected non-nil SummaryText")
	}
	if *s.SummaryText != "Session lasted 5m. Modified 2 files, used 3 tools. 1 commits. 1 errors." {
		t.Errorf("unexpected summary text: %s", *s.SummaryText)
	}

	// Check timeline.
	var timeline []timelineEntry
	if err := json.Unmarshal(s.Timeline, &timeline); err != nil {
		t.Fatalf("failed to unmarshal timeline: %v", err)
	}
	if len(timeline) < 3 {
		t.Errorf("expected at least 3 timeline entries, got %d", len(timeline))
	}
	// First entry should be session_start.
	if len(timeline) > 0 && timeline[0].EventType != "session_start" {
		t.Errorf("expected first timeline entry to be session_start, got %s", timeline[0].EventType)
	}

	// Check embedding was stored.
	if !embedStore.stored {
		t.Error("expected embedding to be stored")
	}
	if embedStore.sourceType != "session_summary" {
		t.Errorf("expected sourceType=session_summary, got %s", embedStore.sourceType)
	}
}

func TestGenerateForSession_NoEvents(t *testing.T) {
	session := &model.Session{
		SessionID: uuid.New(),
		AgentID:   "test-agent",
		Status:    "closed",
		StartedAt: time.Now().UTC(),
	}

	eventRepo := &mockSummaryEventRepo{events: nil, total: 0}
	summaryStore := &mockSummaryStore{}

	svc := NewSummaryService(summaryStore, eventRepo, nil, nil)
	err := svc.GenerateForSession(context.Background(), session)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s := summaryStore.created
	if s == nil {
		t.Fatal("expected summary to be created even with no events")
	}
	if s.EventCount != 0 {
		t.Errorf("expected EventCount=0, got %d", s.EventCount)
	}
	if *s.SummaryText != "Session lasted 0m. Modified 0 files, used 0 tools. 0 commits. 0 errors." {
		t.Errorf("unexpected summary text: %s", *s.SummaryText)
	}
}

func TestGenerateForSession_NilEmbedder(t *testing.T) {
	events := makeEvents()
	session := &model.Session{
		SessionID: events[0].SessionID,
		AgentID:   "test-agent",
		Status:    "closed",
		StartedAt: time.Now().UTC(),
	}

	eventRepo := &mockSummaryEventRepo{events: events, total: len(events)}
	summaryStore := &mockSummaryStore{}
	embedStore := &mockEmbeddingStore{}

	// embedder is nil — should not try to embed.
	svc := NewSummaryService(summaryStore, eventRepo, embedStore, nil)
	err := svc.GenerateForSession(context.Background(), session)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if embedStore.stored {
		t.Error("expected no embedding to be stored when embedder is nil")
	}
}
