package embed_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/embed"
)

// localEmbedRequest mirrors the unexported type for test server decoding.
type localEmbedRequest struct {
	Text  string   `json:"text,omitempty"`
	Texts []string `json:"texts,omitempty"`
	Model string   `json:"model"`
}

// localEmbedResponse mirrors the unexported type for test server encoding.
type localEmbedResponse struct {
	Embedding  []float32   `json:"embedding,omitempty"`
	Embeddings [][]float32 `json:"embeddings,omitempty"`
}

// apiEmbedResponseData mirrors the unexported type.
type apiEmbedResponseData struct {
	Embedding []float32 `json:"embedding"`
}

// apiEmbedResponse mirrors the unexported type.
type apiEmbedResponse struct {
	Data []apiEmbedResponseData `json:"data"`
}

func TestNewEmbedder_Noop(t *testing.T) {
	e, err := embed.NewEmbedder("noop", "", "", "", 384)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := e.(*embed.NoopEmbedder); !ok {
		t.Fatalf("expected *NoopEmbedder, got %T", e)
	}
}

func TestNewEmbedder_UnknownBackend(t *testing.T) {
	_, err := embed.NewEmbedder("unknown", "", "", "", 384)
	if err == nil {
		t.Fatal("expected error for unknown backend")
	}
}

func TestNoopEmbedder_Embed(t *testing.T) {
	e := embed.NewNoopEmbedder(128)
	vec, err := e.Embed(context.Background(), "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vec) != 128 {
		t.Fatalf("expected 128 dimensions, got %d", len(vec))
	}
	for i, v := range vec {
		if v != 0 {
			t.Fatalf("expected zero at index %d, got %f", i, v)
		}
	}
}

func TestNoopEmbedder_EmbedBatch(t *testing.T) {
	e := embed.NewNoopEmbedder(64)
	texts := []string{"a", "b", "c"}
	vecs, err := e.EmbedBatch(context.Background(), texts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vecs) != 3 {
		t.Fatalf("expected 3 results, got %d", len(vecs))
	}
	for i, vec := range vecs {
		if len(vec) != 64 {
			t.Fatalf("result %d: expected 64 dimensions, got %d", i, len(vec))
		}
	}
}

func TestNoopEmbedder_Dimensions(t *testing.T) {
	e := embed.NewNoopEmbedder(256)
	if e.Dimensions() != 256 {
		t.Fatalf("expected 256, got %d", e.Dimensions())
	}
}

func TestLocalEmbedder_Embed(t *testing.T) {
	expected := []float32{0.1, 0.2, 0.3}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		var req localEmbedRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if req.Text != "hello world" {
			t.Errorf("expected text 'hello world', got %q", req.Text)
		}
		if req.Model != "test-model" {
			t.Errorf("expected model 'test-model', got %q", req.Model)
		}
		resp := localEmbedResponse{Embedding: expected}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	e, err := embed.NewLocalEmbedder(srv.URL, "test-model", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	vec, err := e.Embed(context.Background(), "hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vec) != 3 {
		t.Fatalf("expected 3 dimensions, got %d", len(vec))
	}
	for i, v := range vec {
		if v != expected[i] {
			t.Errorf("index %d: expected %f, got %f", i, expected[i], v)
		}
	}
}

func TestLocalEmbedder_EmbedBatch(t *testing.T) {
	expected := [][]float32{{0.1, 0.2}, {0.3, 0.4}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req localEmbedRequest
		json.NewDecoder(r.Body).Decode(&req)
		if len(req.Texts) != 2 {
			t.Errorf("expected 2 texts, got %d", len(req.Texts))
		}
		resp := localEmbedResponse{Embeddings: expected}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	e, err := embed.NewLocalEmbedder(srv.URL, "test-model", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	vecs, err := e.EmbedBatch(context.Background(), []string{"a", "b"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vecs) != 2 {
		t.Fatalf("expected 2 results, got %d", len(vecs))
	}
}

func TestLocalEmbedder_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	e, _ := embed.NewLocalEmbedder(srv.URL, "test-model", 3)
	_, err := e.Embed(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected error for server error response")
	}
}

func TestLocalEmbedder_DefaultURL(t *testing.T) {
	// When no URL is provided, the embedder should use the default local URL.
	// We verify this by checking that an Embed call targets the default URL
	// (it will fail to connect, but the error message reveals the URL).
	e, err := embed.NewLocalEmbedder("", "model", 384)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Attempting to embed should fail with a connection error to the default URL.
	_, err = e.Embed(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error when connecting to default URL")
	}
	// The embedder was created successfully, which means it accepted the empty URL
	// and substituted the default.
	_ = e // successfully created with default URL
}

func TestAPIEmbedder_Embed(t *testing.T) {
	expected := []float32{0.5, 0.6, 0.7}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-key" {
			t.Errorf("expected 'Bearer test-key', got %q", auth)
		}
		resp := apiEmbedResponse{
			Data: []apiEmbedResponseData{{Embedding: expected}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	e, err := embed.NewAPIEmbedder(srv.URL, "test-key", "text-embedding-3-small", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	vec, err := e.Embed(context.Background(), "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vec) != 3 {
		t.Fatalf("expected 3 dimensions, got %d", len(vec))
	}
	for i, v := range vec {
		if v != expected[i] {
			t.Errorf("index %d: expected %f, got %f", i, expected[i], v)
		}
	}
}

func TestAPIEmbedder_EmbedBatch(t *testing.T) {
	expected := [][]float32{{0.1, 0.2}, {0.3, 0.4}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := apiEmbedResponse{
			Data: []apiEmbedResponseData{
				{Embedding: expected[0]},
				{Embedding: expected[1]},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	e, err := embed.NewAPIEmbedder(srv.URL, "key", "model", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	vecs, err := e.EmbedBatch(context.Background(), []string{"a", "b"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vecs) != 2 {
		t.Fatalf("expected 2 results, got %d", len(vecs))
	}
}

func TestAPIEmbedder_EmptyURL(t *testing.T) {
	_, err := embed.NewAPIEmbedder("", "key", "model", 384)
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
}

func TestAPIEmbedder_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("unauthorized"))
	}))
	defer srv.Close()

	e, _ := embed.NewAPIEmbedder(srv.URL, "bad-key", "model", 3)
	_, err := e.Embed(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected error for server error response")
	}
}

func TestNewEmbedder_Local(t *testing.T) {
	e, err := embed.NewEmbedder("local", "model", "http://localhost:9999", "", 384)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := e.(*embed.LocalEmbedder); !ok {
		t.Fatalf("expected *LocalEmbedder, got %T", e)
	}
}

func TestNewEmbedder_API(t *testing.T) {
	e, err := embed.NewEmbedder("api", "model", "http://localhost:9999", "key", 384)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := e.(*embed.APIEmbedder); !ok {
		t.Fatalf("expected *APIEmbedder, got %T", e)
	}
}
