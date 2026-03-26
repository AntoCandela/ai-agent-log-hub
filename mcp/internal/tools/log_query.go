package tools

import (
	"context"
	"fmt"
	"net/url"

	"github.com/AntoCandela/ai-agent-log-hub/mcp/internal/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// registerLogQuery creates the "log.query" tool.
// Translates MCP arguments into GET /api/v1/logs with query-string filters.
func registerLogQuery(s *server.MCPServer, backend *client.BackendClient) {
	tool := mcp.NewTool("log.query",
		mcp.WithDescription("Query log events with optional filters"),
		mcp.WithString("severity",
			mcp.Description("Filter by severity level"),
			mcp.Enum("debug", "info", "warn", "error"),
		),
		mcp.WithString("session_id",
			mcp.Description("Filter by session ID"),
		),
		mcp.WithString("agent_id",
			mcp.Description("Filter by agent ID"),
		),
		mcp.WithString("since",
			mcp.Description("Filter logs since this RFC3339 timestamp"),
		),
		mcp.WithString("until",
			mcp.Description("Filter logs until this RFC3339 timestamp"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of results to return"),
		),
	)

	s.AddTool(tool, handleLogQuery(backend))
}

// handleLogQuery maps optional MCP arguments to URL query parameters and
// returns the backend's filtered log list.
func handleLogQuery(backend *client.BackendClient) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := args(request)
		params := url.Values{}

		if v, ok := args["severity"].(string); ok && v != "" {
			params.Set("severity", v)
		}
		if v, ok := args["session_id"].(string); ok && v != "" {
			params.Set("session_id", v)
		}
		if v, ok := args["agent_id"].(string); ok && v != "" {
			params.Set("agent_id", v)
		}
		if v, ok := args["since"].(string); ok && v != "" {
			params.Set("since", v)
		}
		if v, ok := args["until"].(string); ok && v != "" {
			params.Set("until", v)
		}
		if v, ok := args["limit"].(float64); ok {
			params.Set("limit", fmt.Sprintf("%d", int(v)))
		}

		result, err := backend.Get(ctx, "/api/v1/logs", params)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to query logs: %v", err)), nil
		}

		return mcp.NewToolResultText(string(result)), nil
	}
}
