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

// MemoryStorer abstracts memory persistence (the key-value store).
// Each memory entry is keyed by (agent_id, key) and can carry arbitrary
// string values plus tags. This interface is used for CRUD operations.
type MemoryStorer interface {
	Upsert(ctx context.Context, m *model.Memory) error
	GetByKey(ctx context.Context, agentID, key string) (*model.Memory, error)
	List(ctx context.Context, agentID string, tags []string, limit, offset int) ([]model.Memory, int, error)
	Delete(ctx context.Context, agentID, key string) error
}

// MemoryEmbedder generates vector embeddings from text.
// Embeddings are fixed-length float32 slices that represent the semantic
// meaning of the input text. They are used for similarity search: texts
// with similar meanings produce vectors that are close together in
// high-dimensional space.
//
// This dependency is optional -- if nil, semantic search endpoints will
// return a "service unavailable" error, but basic CRUD still works.
type MemoryEmbedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// EmbeddingSearchResult represents a single result from a semantic search.
// Similarity is a 0-to-1 score where 1 means identical meaning.
type EmbeddingSearchResult struct {
	SourceType string    `json:"source_type"`
	SourceID   uuid.UUID `json:"source_id"`
	AgentID    string    `json:"agent_id"`
	Content    string    `json:"content"`
	Similarity float64   `json:"similarity"`
	Shared     bool      `json:"shared"`
}

// EmbeddingStorer abstracts embedding persistence and search.
// It stores vector embeddings alongside metadata (source type, agent, etc.)
// and provides a Search method that finds the closest vectors to a query
// embedding using cosine similarity.
type EmbeddingStorer interface {
	Store(ctx context.Context, sourceType string, sourceID uuid.UUID, agentID, content string, embedding []float32, shared bool) error
	DeleteBySource(ctx context.Context, sourceType string, sourceID uuid.UUID) error
	Search(ctx context.Context, queryEmbedding []float32, agentID string, sourceTypes []string, topK int, minSimilarity float64) ([]EmbeddingSearchResult, error)
}

// MemoryHandler serves the memory REST endpoints:
//   - POST   /api/v1/memory         -> StoreMemory  (create/update)
//   - GET    /api/v1/memory         -> ListMemories (list with filters)
//   - DELETE /api/v1/memory/{key}   -> DeleteMemory
//   - POST   /api/v1/memory/search  -> SearchMemory (semantic search)
//   - POST   /api/v1/memory/recall  -> RecallMemory (semantic recall)
//
// The embedder and embStore fields are optional. When nil, CRUD operations
// still work but semantic search/recall endpoints return 503.
type MemoryHandler struct {
	store    MemoryStorer
	embedder MemoryEmbedder
	embStore EmbeddingStorer
}

// NewMemoryHandler creates a MemoryHandler with the given dependencies.
// Pass nil for embedder and embStore if semantic search is not available.
func NewMemoryHandler(store MemoryStorer, embedder MemoryEmbedder, embStore EmbeddingStorer) *MemoryHandler {
	return &MemoryHandler{
		store:    store,
		embedder: embedder,
		embStore: embStore,
	}
}

// ── request / response types ──

// storeMemoryRequest is the JSON body for POST /api/v1/memory.
type storeMemoryRequest struct {
	AgentID   string     `json:"agent_id"`
	SessionID *uuid.UUID `json:"session_id"`
	Key       string     `json:"key"`
	Value     string     `json:"value"`
	Tags      []string   `json:"tags"`
	Shared    bool       `json:"shared"`
}

// searchMemoryRequest is the JSON body for POST /api/v1/memory/search.
type searchMemoryRequest struct {
	Query         string   `json:"query"`
	AgentID       string   `json:"agent_id"`
	SourceTypes   []string `json:"source_types"`
	TopK          int      `json:"top_k"`
	MinSimilarity float64  `json:"min_similarity"`
}

// recallMemoryRequest is the JSON body for POST /api/v1/memory/recall.
type recallMemoryRequest struct {
	Context string `json:"context"`
	AgentID string `json:"agent_id"`
	TopK    int    `json:"top_k"`
}

// StoreMemory handles POST /api/v1/memory.
// It creates or updates a memory entry (upsert by agent_id + key).
// If an embedder and embedding store are configured, it also generates a
// vector embedding of the value and stores it for later semantic search.
// Embedding failures are silently ignored so that the primary write always
// succeeds.
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

	// Embed and store the embedding if both services are available.
	// Errors are ignored because embedding is a best-effort enhancement.
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
// It performs a semantic similarity search: the caller's "query" text is
// converted to a vector embedding, then compared against stored embeddings
// to find the most semantically similar entries.
//
// Defaults: top_k=10, min_similarity=0.5, source_types=["memory"].
// Returns 503 if the embedding services are not configured.
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
	// Apply sensible defaults for optional parameters.
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

	// Convert the query text into a vector embedding.
	vec, err := h.embedder.Embed(ctx, req.Query)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to embed query: "+err.Error())
		return
	}

	// Find stored embeddings closest to the query vector.
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
// Similar to SearchMemory but broader: it searches across memories, session
// summaries, and error records to give the agent a comprehensive "recall"
// of relevant past context. The minimum similarity threshold is lower (0.3)
// to surface more potentially relevant results.
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

	// Recall searches across multiple source types for broad context retrieval.
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
// Returns a paginated list of memory entries for a given agent, optionally
// filtered by comma-separated tags.
//
// Query parameters:
//   - agent_id (required): which agent's memories to list.
//   - tags (optional): comma-separated tag filter (e.g. "config,preference").
//   - limit (optional): max results, default 50.
//   - offset (optional): pagination offset, default 0.
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
// Removes a memory entry by its key for a given agent. If an embedding store
// is configured, the associated embedding is also cleaned up. The embedding
// cleanup is best-effort: if it fails, the memory is still deleted.
//
// Query parameters:
//   - agent_id (required): identifies which agent's memory to delete.
//
// Path parameters:
//   - key: the memory key to delete (from the URL path).
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
	// This must happen before the delete, because after deletion the memory is gone.
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
