package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/model"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ── mocks ──

type mockMemoryStorer struct {
	memories map[string]*model.Memory // keyed by "agentID|key"
	err      error
}

func newMockMemoryStorer() *mockMemoryStorer {
	return &mockMemoryStorer{memories: make(map[string]*model.Memory)}
}

func (m *mockMemoryStorer) Upsert(_ context.Context, mem *model.Memory) error {
	if m.err != nil {
		return m.err
	}
	mem.MemoryID = uuid.New()
	mem.CreatedAt = time.Now()
	mem.UpdatedAt = time.Now()
	m.memories[mem.AgentID+"|"+mem.Key] = mem
	return nil
}

func (m *mockMemoryStorer) GetByKey(_ context.Context, agentID, key string) (*model.Memory, error) {
	if m.err != nil {
		return nil, m.err
	}
	mem, ok := m.memories[agentID+"|"+key]
	if !ok {
		return nil, nil
	}
	return mem, nil
}

func (m *mockMemoryStorer) List(_ context.Context, agentID string, tags []string, limit, offset int) ([]model.Memory, int, error) {
	if m.err != nil {
		return nil, 0, m.err
	}
	var result []model.Memory
	for _, mem := range m.memories {
		if mem.AgentID != agentID {
			continue
		}
		if len(tags) > 0 {
			match := false
			for _, t := range tags {
				for _, mt := range mem.Tags {
					if t == mt {
						match = true
						break
					}
				}
				if match {
					break
				}
			}
			if !match {
				continue
			}
		}
		result = append(result, *mem)
	}
	total := len(result)
	if offset >= len(result) {
		return []model.Memory{}, total, nil
	}
	end := offset + limit
	if end > len(result) {
		end = len(result)
	}
	return result[offset:end], total, nil
}

func (m *mockMemoryStorer) Delete(_ context.Context, agentID, key string) error {
	if m.err != nil {
		return m.err
	}
	delete(m.memories, agentID+"|"+key)
	return nil
}

type mockMemoryEmbedder struct {
	vec []float32
	err error
}

func (m *mockMemoryEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.vec, nil
}

type mockEmbeddingStorer struct {
	stored  int
	deleted int
	results []EmbeddingSearchResult
	err     error
}

func (m *mockEmbeddingStorer) Store(_ context.Context, _ string, _ uuid.UUID, _, _ string, _ []float32, _ bool) error {
	if m.err != nil {
		return m.err
	}
	m.stored++
	return nil
}

func (m *mockEmbeddingStorer) DeleteBySource(_ context.Context, _ string, _ uuid.UUID) error {
	if m.err != nil {
		return m.err
	}
	m.deleted++
	return nil
}

func (m *mockEmbeddingStorer) Search(_ context.Context, _ []float32, _ string, _ []string, _ int, _ float64) ([]EmbeddingSearchResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.results, nil
}

// ── tests ──

func TestStoreMemory(t *testing.T) {
	store := newMockMemoryStorer()
	embedder := &mockMemoryEmbedder{vec: []float32{0.1, 0.2, 0.3}}
	embStore := &mockEmbeddingStorer{}
	h := NewMemoryHandler(store, embedder, embStore)

	body := `{"agent_id":"agent-1","key":"user_pref","value":"dark mode","tags":["ui","pref"],"shared":false}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/memory", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.StoreMemory(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// Verify embedding was stored.
	if embStore.stored != 1 {
		t.Fatalf("expected 1 embedding stored, got %d", embStore.stored)
	}

	// Verify memory in store.
	mem, _ := store.GetByKey(nil, "agent-1", "user_pref")
	if mem == nil {
		t.Fatal("memory not found in store")
	}
	if mem.Value != "dark mode" {
		t.Fatalf("expected value 'dark mode', got %q", mem.Value)
	}
}

func TestStoreMemory_MissingAgentID(t *testing.T) {
	h := NewMemoryHandler(newMockMemoryStorer(), nil, nil)

	body := `{"key":"k","value":"v"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/memory", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.StoreMemory(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestStoreMemory_MissingKey(t *testing.T) {
	h := NewMemoryHandler(newMockMemoryStorer(), nil, nil)

	body := `{"agent_id":"a","value":"v"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/memory", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.StoreMemory(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSearchMemory(t *testing.T) {
	store := newMockMemoryStorer()
	embedder := &mockMemoryEmbedder{vec: []float32{0.1, 0.2}}
	results := []EmbeddingSearchResult{
		{SourceType: "memory", SourceID: uuid.New(), AgentID: "agent-1", Content: "found it", Similarity: 0.95},
		{SourceType: "memory", SourceID: uuid.New(), AgentID: "agent-1", Content: "also this", Similarity: 0.80},
	}
	embStore := &mockEmbeddingStorer{results: results}
	h := NewMemoryHandler(store, embedder, embStore)

	body := `{"query":"dark mode","agent_id":"agent-1","top_k":5}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/memory/search", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.SearchMemory(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	data, ok := resp["data"].([]any)
	if !ok {
		t.Fatal("expected data array in response")
	}
	if len(data) != 2 {
		t.Fatalf("expected 2 results, got %d", len(data))
	}
}

func TestSearchMemory_MissingAgentID(t *testing.T) {
	h := NewMemoryHandler(newMockMemoryStorer(), &mockMemoryEmbedder{}, &mockEmbeddingStorer{})

	body := `{"query":"something"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/memory/search", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.SearchMemory(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestListMemories(t *testing.T) {
	store := newMockMemoryStorer()
	// Seed some memories.
	store.memories["agent-1|k1"] = &model.Memory{
		MemoryID: uuid.New(), AgentID: "agent-1", Key: "k1", Value: "v1",
		Tags: []string{"tag1"}, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	store.memories["agent-1|k2"] = &model.Memory{
		MemoryID: uuid.New(), AgentID: "agent-1", Key: "k2", Value: "v2",
		Tags: []string{"tag2"}, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	h := NewMemoryHandler(store, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/memory?agent_id=agent-1&limit=10", nil)
	w := httptest.NewRecorder()

	h.ListMemories(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	data, ok := resp["data"].([]any)
	if !ok {
		t.Fatal("expected data array")
	}
	if len(data) != 2 {
		t.Fatalf("expected 2 memories, got %d", len(data))
	}
	if resp["total"].(float64) != 2 {
		t.Fatalf("expected total=2, got %v", resp["total"])
	}
}

func TestListMemories_MissingAgentID(t *testing.T) {
	h := NewMemoryHandler(newMockMemoryStorer(), nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/memory", nil)
	w := httptest.NewRecorder()

	h.ListMemories(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestDeleteMemory(t *testing.T) {
	store := newMockMemoryStorer()
	memID := uuid.New()
	store.memories["agent-1|mykey"] = &model.Memory{
		MemoryID: memID, AgentID: "agent-1", Key: "mykey", Value: "val",
	}
	embStore := &mockEmbeddingStorer{}
	h := NewMemoryHandler(store, nil, embStore)

	// Use chi router to extract URL param.
	r := chi.NewRouter()
	r.Delete("/api/v1/memory/{key}", h.DeleteMemory)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/memory/mykey?agent_id=agent-1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}

	// Verify memory was removed.
	if _, ok := store.memories["agent-1|mykey"]; ok {
		t.Fatal("memory should have been deleted")
	}

	// Verify embedding was cleaned up.
	if embStore.deleted != 1 {
		t.Fatalf("expected 1 embedding deleted, got %d", embStore.deleted)
	}
}

func TestDeleteMemory_MissingAgentID(t *testing.T) {
	h := NewMemoryHandler(newMockMemoryStorer(), nil, nil)

	r := chi.NewRouter()
	r.Delete("/api/v1/memory/{key}", h.DeleteMemory)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/memory/mykey", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
