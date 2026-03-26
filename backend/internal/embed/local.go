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

const defaultLocalURL = "http://localhost:8090/embed"

// LocalEmbedder calls a sidecar HTTP embedding service (e.g. sentence-transformers).
type LocalEmbedder struct {
	url    string
	model  string
	dims   int
	client *http.Client
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

type localEmbedRequest struct {
	Text  string `json:"text,omitempty"`
	Texts []string `json:"texts,omitempty"`
	Model string `json:"model"`
}

type localEmbedResponse struct {
	Embedding  []float32   `json:"embedding"`
	Embeddings [][]float32 `json:"embeddings"`
}

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

func (e *LocalEmbedder) Dimensions() int {
	return e.dims
}
