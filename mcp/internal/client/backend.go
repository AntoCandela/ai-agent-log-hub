// Package client provides an HTTP client for communicating with the
// ai-agent-log-hub REST backend.
//
// Every MCP tool handler delegates its actual work to this client rather
// than building HTTP requests directly. This keeps network concerns (base
// URL, authentication, timeouts, error handling) in one place.
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
// It holds the base URL, an optional API key for Bearer-token authentication,
// and the agent identifier that is included in log events.
type BackendClient struct {
	baseURL string       // e.g. "http://localhost:4800"
	apiKey  string       // sent as "Authorization: Bearer <apiKey>" when non-empty
	agentID string       // identifies the AI agent producing logs
	client  *http.Client // shared HTTP client with a 30-second timeout
}

// NewBackendClient creates a BackendClient configured with the given backend
// URL, optional API key, and agent ID. A single http.Client with a 30-second
// timeout is reused across all requests.
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

// AgentID returns the configured agent identifier so that tool handlers can
// include it in log events without knowing how the client was configured.
func (c *BackendClient) AgentID() string { return c.agentID }

// Get performs an HTTP GET request against the backend. Query parameters are
// appended to the URL from the provided url.Values. The raw JSON response
// body is returned on success.
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

// Post performs an HTTP POST request against the backend. The body parameter
// is JSON-encoded automatically. The raw JSON response body is returned on
// success.
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

// Delete performs an HTTP DELETE request against the backend. The raw JSON
// response body is returned on success.
func (c *BackendClient) Delete(ctx context.Context, path string) (json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("creating DELETE request: %w", err)
	}

	return c.do(req)
}

// do is the shared request executor. It attaches the Bearer token (if
// configured), sends the request, reads the full response, and returns an
// error for any non-2xx status code. All public methods (Get, Post, Delete)
// funnel through here.
func (c *BackendClient) do(req *http.Request) (json.RawMessage, error) {
	// Attach Bearer auth header when an API key has been configured.
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

	// Treat anything outside the 2xx range as an error, including the
	// response body in the message for easier debugging.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("backend returned %d for %s %s: %s", resp.StatusCode, req.Method, req.URL.Path, string(body))
	}

	return json.RawMessage(body), nil
}
