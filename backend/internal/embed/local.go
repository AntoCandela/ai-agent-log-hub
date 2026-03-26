package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// defaultLocalURL is the default endpoint for the local sidecar embedding service.
const defaultLocalURL = "http://localhost:8090/embed"

// LocalEmbedder calls a sidecar HTTP embedding service running on the same
// machine or in a companion container. A "sidecar" is a helper process that
// runs alongside the main application — in this case a lightweight HTTP server
// that loads a machine-learning model (e.g. sentence-transformers) and exposes
// an /embed endpoint.
//
// The sidecar model approach keeps the Go backend simple (no ML dependencies)
// while allowing any Python/Rust model to be plugged in behind a standard
// HTTP/JSON interface.
type LocalEmbedder struct {
	url    string       // The sidecar's /embed endpoint URL.
	model  string       // Model name to pass in the request (e.g. "all-MiniLM-L6-v2").
	dims   int          // Expected dimensionality of the output vectors.
	client *http.Client // HTTP client with a 30-second timeout.
}

// NewLocalEmbedder creates a LocalEmbedder targeting the given URL.
// If url is empty, it defaults to http://localhost:8090/embed.
func NewLocalEmbedder(url, model string, dims int) (*LocalEmbedder, error) {
	if url == "" {
		url = defaultLocalURL
	}
	return &LocalEmbedder{
		url:   url,
		model: model,
		dims:  dims,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// localEmbedRequest is the JSON body sent to the sidecar's /embed endpoint.
type localEmbedRequest struct {
	Text  string   `json:"text,omitempty"`  // Single text (used by Embed).
	Texts []string `json:"texts,omitempty"` // Multiple texts (used by EmbedBatch).
	Model string   `json:"model"`           // Which model the sidecar should use.
}

// localEmbedResponse is the JSON body returned by the sidecar's /embed endpoint.
type localEmbedResponse struct {
	Embedding  []float32   `json:"embedding"`  // Single vector (returned for single text).
	Embeddings [][]float32 `json:"embeddings"` // Multiple vectors (returned for batch).
}

// Embed sends a single text to the sidecar and returns its embedding vector.
func (e *LocalEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	reqBody := localEmbedRequest{
		Text:  text,
		Model: e.model,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal embed request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create embed request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding sidecar request failed (is the sidecar running at %s?): %w", e.url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embedding sidecar returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result localEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode embed response: %w", err)
	}

	return result.Embedding, nil
}

// EmbedBatch sends multiple texts to the sidecar in one HTTP call and returns
// their embedding vectors.
func (e *LocalEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	reqBody := localEmbedRequest{
		Texts: texts,
		Model: e.model,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal embed batch request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create embed batch request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding sidecar request failed (is the sidecar running at %s?): %w", e.url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embedding sidecar returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result localEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode embed batch response: %w", err)
	}

	return result.Embeddings, nil
}

// Dimensions returns the configured vector dimensionality.
func (e *LocalEmbedder) Dimensions() int {
	return e.dims
}
