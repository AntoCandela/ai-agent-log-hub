package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// BackendClient wraps HTTP calls to the ai-agent-log-hub backend REST API.
type BackendClient struct {
	baseURL string
	apiKey  string
	agentID string
	client  *http.Client
}

// NewBackendClient creates a new BackendClient.
func NewBackendClient(baseURL, apiKey, agentID string) *BackendClient {
	return &BackendClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		agentID: agentID,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// AgentID returns the configured agent identifier.
func (c *BackendClient) AgentID() string { return c.agentID }

// Get performs an HTTP GET request against the backend.
func (c *BackendClient) Get(ctx context.Context, path string, params url.Values) (json.RawMessage, error) {
	u := c.baseURL + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("creating GET request: %w", err)
	}

	return c.do(req)
}

// Post performs an HTTP POST request against the backend.
func (c *BackendClient) Post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshalling request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("creating POST request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	return c.do(req)
}

// Delete performs an HTTP DELETE request against the backend.
func (c *BackendClient) Delete(ctx context.Context, path string) (json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("creating DELETE request: %w", err)
	}

	return c.do(req)
}

func (c *BackendClient) do(req *http.Request) (json.RawMessage, error) {
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request %s %s: %w", req.Method, req.URL.Path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("backend returned %d for %s %s: %s", resp.StatusCode, req.Method, req.URL.Path, string(body))
	}

	return json.RawMessage(body), nil
}
