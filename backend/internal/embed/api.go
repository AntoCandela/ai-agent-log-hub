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

// APIEmbedder calls an external embedding API (e.g. OpenAI, Voyage).
type APIEmbedder struct {
	url    string
	apiKey string
	model  string
	dims   int
	client *http.Client
}

// NewAPIEmbedder creates an APIEmbedder for an external embedding API.
func NewAPIEmbedder(url, apiKey, model string, dims int) (*APIEmbedder, error) {
	if url == "" {
		return nil, fmt.Errorf("API embedding URL is required")
	}
	return &APIEmbedder{
		url:    url,
		apiKey: apiKey,
		model:  model,
		dims:   dims,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

type apiEmbedRequest struct {
	Input any    `json:"input"`
	Model string `json:"model"`
}

type apiEmbedResponseData struct {
	Embedding []float32 `json:"embedding"`
}

type apiEmbedResponse struct {
	Data []apiEmbedResponseData `json:"data"`
}

func (e *APIEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	reqBody := apiEmbedRequest{
		Input: text,
		Model: e.model,
	}

	result, err := e.doRequest(ctx, reqBody)
	if err != nil {
		return nil, err
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("embedding API returned no data")
	}

	return result.Data[0].Embedding, nil
}

func (e *APIEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	reqBody := apiEmbedRequest{
		Input: texts,
		Model: e.model,
	}

	result, err := e.doRequest(ctx, reqBody)
	if err != nil {
		return nil, err
	}

	embeddings := make([][]float32, len(result.Data))
	for i, d := range result.Data {
		embeddings[i] = d.Embedding
	}

	return embeddings, nil
}

func (e *APIEmbedder) Dimensions() int {
	return e.dims
}

func (e *APIEmbedder) doRequest(ctx context.Context, reqBody apiEmbedRequest) (*apiEmbedResponse, error) {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal API embed request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create API embed request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if e.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.apiKey)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embedding API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result apiEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode API embed response: %w", err)
	}

	return &result, nil
}
