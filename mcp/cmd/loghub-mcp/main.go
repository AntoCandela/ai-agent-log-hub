// Package main is the entry point for the loghub-mcp server binary.
//
// MCP (Model Context Protocol) is a standard that lets AI assistants (like
// Claude Code) discover and call "tools" exposed by external servers. This
// binary starts an MCP server that exposes logging and memory tools, allowing
// the AI agent to read/write structured logs and key-value memories via the
// ai-agent-log-hub REST backend.
//
// Communication happens over **stdio** (standard input/output): the AI host
// process launches this binary, then sends JSON-RPC requests on stdin and
// reads JSON-RPC responses on stdout. This is the simplest MCP transport and
// requires no network ports.
//
// Configuration is done entirely through environment variables:
//   - LOGHUB_URL      — base URL of the log-hub backend (default http://localhost:4800)
//   - LOGHUB_API_KEY  — optional Bearer token for backend authentication
//   - LOGHUB_AGENT_ID — identifier for this agent instance (default "claude-code")
package main

import (
	"fmt"
	"os"

	"github.com/AntoCandela/ai-agent-log-hub/mcp/internal/client"
	"github.com/AntoCandela/ai-agent-log-hub/mcp/internal/tools"
	"github.com/mark3labs/mcp-go/server"
)

// main reads environment-based configuration, creates an HTTP client for the
// backend, registers all MCP tools, and starts serving on stdio.
func main() {
	// LOGHUB_URL points to the ai-agent-log-hub REST backend.
	// Default to localhost for local development.
	baseURL := os.Getenv("LOGHUB_URL")
	if baseURL == "" {
		baseURL = "http://localhost:4800"
	}

	// LOGHUB_API_KEY is an optional Bearer token attached to every backend request.
	apiKey := os.Getenv("LOGHUB_API_KEY")

	// LOGHUB_AGENT_ID identifies which AI agent is producing the logs.
	agentID := os.Getenv("LOGHUB_AGENT_ID")
	if agentID == "" {
		agentID = "claude-code"
	}

	// BackendClient wraps all HTTP calls to the REST API so that individual
	// tool handlers never deal with raw HTTP themselves.
	backend := client.NewBackendClient(baseURL, apiKey, agentID)

	// Create the MCP server instance. "loghub" is the server name that the AI
	// host will see during the MCP handshake. WithToolCapabilities(true) tells
	// the host that this server exposes callable tools.
	s := server.NewMCPServer(
		"loghub",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	// Register every tool the AI agent can invoke. Each tool maps to one
	// REST endpoint on the backend (see the tools package for details).
	tools.RegisterLogTools(s, backend)
	tools.RegisterMemoryTools(s, backend)

	fmt.Fprintf(os.Stderr, "loghub-mcp: tools registered, serving on stdio\n")

	// ServeStdio blocks forever, reading JSON-RPC requests from stdin and
	// writing responses to stdout. If it returns, something went wrong.
	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "loghub-mcp: server error: %v\n", err)
		os.Exit(1)
	}
}
