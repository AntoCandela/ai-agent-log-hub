package tools

import (
	"github.com/AntoCandela/ai-agent-log-hub/mcp/internal/client"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterLogTools registers all log-related MCP tools.
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

// RegisterMemoryTools registers all memory-related MCP tools.
func RegisterMemoryTools(s *server.MCPServer, backend *client.BackendClient) {
	registerMemoryStore(s, backend)
	registerMemorySearch(s, backend)
	registerMemoryRecall(s, backend)
	registerMemoryList(s, backend)
	registerMemoryDelete(s, backend)
}
