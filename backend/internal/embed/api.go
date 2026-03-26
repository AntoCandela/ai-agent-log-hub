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

// APIEmbedder calls an external embedding API over the internet (e.g. OpenAI's
// /v1/embeddings, Voyage AI, or any service that follows the same request/response
// format). This is the "external API pattern": the embedding computation
// happens on a remote server, and this client just sends HTTP requests.
//
// The request format follows the OpenAI convention:
//   { "input": "text or array of texts", "model": "model-name" }
// and the response contains an array of { "embedding": [float, ...] } objects.
type APIEmbedder struct {
	url    string       // The embedding API endpoint URL.
	apiKey string       // Bearer token for authentication (may be empty if not required).
	model  string       // Model name to request (e.g. "text-embedding-ada-002").
	dims   int          // Expected dimensionality of the output vectors.
	client *http.Client // HTTP client with a 30-second timeout.
}

// NewAPIEmbedder creates an APIEmbedder for an external embedding API.
// The url parameter is required; apiKey is optional but recommended for
// authenticated services.
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

// apiEmbedRequest is the JSON body sent to the external embedding API.
type apiEmbedRequest struct {
	Input any    `json:"input"` // A single string or an array of strings.
	Model string `json:"model"` // Which model to use for embedding.
}

// apiEmbedResponseData holds one embedding vector from the API response.
type apiEmbedResponseData struct {
	Embedding []float32 `json:"embedding"`
}

// apiEmbedResponse is the JSON body returned by the external embedding API.
type apiEmbedResponse struct {
	Data []apiEmbedResponseData `json:"data"` // One entry per input text.
}

// Embed sends a single text to the external API and returns its embedding vector.
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

// EmbedBatch sends multiple texts to the external API in one HTTP call.
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

// Dimensions returns the configured vector dimensionality.
func (e *APIEmbedder) Dimensions() int {
	return e.dims
}

// doRequest is the shared HTTP call logic for Embed and EmbedBatch. It
// marshals the request, sends it with optional Bearer auth, and decodes the
// JSON response.
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
