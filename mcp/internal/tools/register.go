// Package tools implements MCP tool definitions and their handler functions.
//
// In the MCP protocol, a "tool" is a named capability that the AI agent can
// invoke. Each tool has:
//   - A schema describing its name, description, and expected arguments
//     (built with mcp.NewTool and helpers like mcp.WithString).
//   - A handler function that runs when the tool is called. The handler
//     receives the parsed arguments, calls the backend REST API, and
//     returns the result as text.
//
// The tool registration pattern used here is:
//  1. Each tool lives in its own file (e.g., log_emit.go).
//  2. A package-private registerXxx function creates the tool schema and
//     binds a handler via s.AddTool.
//  3. This file (register.go) groups those registrations into two public
//     entry points: RegisterLogTools and RegisterMemoryTools.
package tools

import (
	"github.com/AntoCandela/ai-agent-log-hub/mcp/internal/client"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterLogTools registers all log-related MCP tools on the given server.
// Each tool maps one-to-one to a REST endpoint on the backend
// (e.g., log.emit -> POST /api/v1/events).
func RegisterLogTools(s *server.MCPServer, backend *client.BackendClient) {
	registerLogEmit(s, backend)
	registerLogQuery(s, backend)
	registerLogSession(s, backend)
	registerLogSessions(s, backend)
	registerLogFiles(s, backend)
	registerLogErrors(s, backend)
	registerLogSummary(s, backend)
	registerLogBlame(s, backend)
	registerLogSystem(s, backend)
	registerLogTrace(s, backend)
}

// RegisterMemoryTools registers all memory-related MCP tools on the given
// server. Memory tools let the AI agent store, search, recall, list, and
// delete key-value entries persisted by the backend.
func RegisterMemoryTools(s *server.MCPServer, backend *client.BackendClient) {
	registerMemoryStore(s, backend)
	registerMemorySearch(s, backend)
	registerMemoryRecall(s, backend)
	registerMemoryList(s, backend)
	registerMemoryDelete(s, backend)
}
