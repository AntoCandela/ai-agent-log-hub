package tools

import (
	"context"
	"fmt"
	"net/url"

	"github.com/AntoCandela/ai-agent-log-hub/mcp/internal/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerLogSessions(s *server.MCPServer, backend *client.BackendClient) {
	tool := mcp.NewTool("log.sessions",
		mcp.WithDescription("List all sessions with optional filters"),
		mcp.WithString("agent_id",
			mcp.Description("Filter by agent ID"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of sessions to return"),
		),
	)

	s.AddTool(tool, handleLogSessions(backend))
}

func handleLogSessions(backend *client.BackendClient) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := args(request)
		params := url.Values{}

		if v, ok := args["agent_id"].(string); ok && v != "" {
			params.Set("agent_id", v)
		}
		if v, ok := args["limit"].(float64); ok {
			params.Set("limit", fmt.Sprintf("%d", int(v)))
		}

		result, err := backend.Get(ctx, "/api/v1/sessions", params)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to list sessions: %v", err)), nil
		}

		return mcp.NewToolResultText(string(result)), nil
	}
}
