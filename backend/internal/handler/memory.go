package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/model"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// MemoryStorer abstracts memory persistence.
type MemoryStorer interface {
	Upsert(ctx context.Context, m *model.Memory) error
	GetByKey(ctx context.Context, agentID, key string) (*model.Memory, error)
	List(ctx context.Context, agentID string, tags []string, limit, offset int) ([]model.Memory, int, error)
	Delete(ctx context.Context, agentID, key string) error
}

// MemoryEmbedder generates vector embeddings from text.
type MemoryEmbedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// EmbeddingSearchResult represents a single result from a semantic search.
type EmbeddingSearchResult struct {
	SourceType string    `json:"source_type"`
	SourceID   uuid.UUID `json:"source_id"`
	AgentID    string    `json:"agent_id"`
	Content    string    `json:"content"`
	Similarity float64   `json:"similarity"`
	Shared     bool      `json:"shared"`
}

// EmbeddingStorer abstracts embedding persistence and search.
type EmbeddingStorer interface {
	Store(ctx context.Context, sourceType string, sourceID uuid.UUID, agentID, content string, embedding []float32, shared bool) error
	DeleteBySource(ctx context.Context, sourceType string, sourceID uuid.UUID) error
	Search(ctx context.Context, queryEmbedding []float32, agentID string, sourceTypes []string, topK int, minSimilarity float64) ([]EmbeddingSearchResult, error)
}

// MemoryHandler serves the memory REST endpoints.
type MemoryHandler struct {
	store    MemoryStorer
	embedder MemoryEmbedder
	embStore EmbeddingStorer
}

// NewMemoryHandler creates a MemoryHandler with the given dependencies.
func NewMemoryHandler(store MemoryStorer, embedder MemoryEmbedder, embStore EmbeddingStorer) *MemoryHandler {
	return &MemoryHandler{
		store:    store,
		embedder: embedder,
		embStore: embStore,
	}
}

// ── request / response types ──

type storeMemoryRequest struct {
	AgentID   string     `json:"agent_id"`
	SessionID *uuid.UUID `json:"session_id"`
	Key       string     `json:"key"`
	Value     string     `json:"value"`
	Tags      []string   `json:"tags"`
	Shared    bool       `json:"shared"`
}

type searchMemoryRequest struct {
	Query         string   `json:"query"`
	AgentID       string   `json:"agent_id"`
	SourceTypes   []string `json:"source_types"`
	TopK          int      `json:"top_k"`
	MinSimilarity float64  `json:"min_similarity"`
}

type recallMemoryRequest struct {
	Context string `json:"context"`
	AgentID string `json:"agent_id"`
	TopK    int    `json:"top_k"`
}

// StoreMemory handles POST /api/v1/memory.
func (h *MemoryHandler) StoreMemory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req storeMemoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if req.AgentID == "" {
		writeError(w, http.StatusBadRequest, "agent_id is required")
		return
	}
	if req.Key == "" {
		writeError(w, http.StatusBadRequest, "key is required")
		return
	}

	m := &model.Memory{
		AgentID:   req.AgentID,
		SessionID: req.SessionID,
		Key:       req.Key,
		Value:     req.Value,
		Tags:      req.Tags,
		Shared:    req.Shared,
	}

	if err := h.store.Upsert(ctx, m); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store memory: "+err.Error())
		return
	}

	// Embed and store the embedding if embedder and embedding store are available.
	if h.embedder != nil && h.embStore != nil {
		vec, err := h.embedder.Embed(ctx, req.Value)
		if err == nil && vec != nil {
			_ = h.embStore.Store(ctx, "memory", m.MemoryID, m.AgentID, req.Value, vec, req.Shared)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"data": m,
	})
}

// SearchMemory handles POST /api/v1/memory/search.
func (h *MemoryHandler) SearchMemory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req searchMemoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if req.AgentID == "" {
		writeError(w, http.StatusBadRequest, "agent_id is required")
		return
	}
	if req.Query == "" {
		writeError(w, http.StatusBadRequest, "query is required")
		return
	}
	if req.TopK <= 0 {
		req.TopK = 10
	}
	if req.MinSimilarity <= 0 {
		req.MinSimilarity = 0.5
	}
	if len(req.SourceTypes) == 0 {
		req.SourceTypes = []string{"memory"}
	}

	if h.embedder == nil || h.embStore == nil {
		writeError(w, http.StatusServiceUnavailable, "embedding service not configured")
		return
	}

	vec, err := h.embedder.Embed(ctx, req.Query)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to embed query: "+err.Error())
		return
	}

	results, err := h.embStore.Search(ctx, vec, req.AgentID, req.SourceTypes, req.TopK, req.MinSimilarity)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "search failed: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"data": results,
	})
}

// RecallMemory handles POST /api/v1/memory/recall.
func (h *MemoryHandler) RecallMemory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req recallMemoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if req.AgentID == "" {
		writeError(w, http.StatusBadRequest, "agent_id is required")
		return
	}
	if req.Context == "" {
		writeError(w, http.StatusBadRequest, "context is required")
		return
	}
	if req.TopK <= 0 {
		req.TopK = 10
	}

	if h.embedder == nil || h.embStore == nil {
		writeError(w, http.StatusServiceUnavailable, "embedding service not configured")
		return
	}

	vec, err := h.embedder.Embed(ctx, req.Context)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to embed context: "+err.Error())
		return
	}

	// Recall focuses on session summaries and errors in addition to memories.
	sourceTypes := []string{"memory", "summary", "error"}

	results, err := h.embStore.Search(ctx, vec, req.AgentID, sourceTypes, req.TopK, 0.3)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "recall failed: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"data": results,
	})
}

// ListMemories handles GET /api/v1/memory.
func (h *MemoryHandler) ListMemories(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	agentID := r.URL.Query().Get("agent_id")
	if agentID == "" {
		writeError(w, http.StatusBadRequest, "agent_id is required")
		return
	}

	var tags []string
	if t := r.URL.Query().Get("tags"); t != "" {
		tags = strings.Split(t, ",")
	}

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}

	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	memories, total, err := h.store.List(ctx, agentID, tags, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list memories: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"data":   memories,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// DeleteMemory handles DELETE /api/v1/memory/{key}.
func (h *MemoryHandler) DeleteMemory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	key := chi.URLParam(r, "key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "key is required")
		return
	}

	agentID := r.URL.Query().Get("agent_id")
	if agentID == "" {
		writeError(w, http.StatusBadRequest, "agent_id is required")
		return
	}

	// If embedding store is available, look up the memory to get its ID for cleanup.
	if h.embStore != nil {
		m, _ := h.store.GetByKey(ctx, agentID, key)
		if m != nil {
			_ = h.embStore.DeleteBySource(ctx, "memory", m.MemoryID)
		}
	}

	if err := h.store.Delete(ctx, agentID, key); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete memory: "+err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
