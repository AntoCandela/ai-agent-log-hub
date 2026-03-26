package tools

import (
	"context"
	"fmt"
	"net/url"

	"github.com/AntoCandela/ai-agent-log-hub/mcp/internal/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerLogErrors(s *server.MCPServer, backend *client.BackendClient) {
	tool := mcp.NewTool("log.errors",
		mcp.WithDescription("Search for error log events"),
		mcp.WithString("session_id",
			mcp.Description("Filter errors by session ID"),
		),
		mcp.WithString("agent_id",
			mcp.Description("Filter errors by agent ID"),
		),
		mcp.WithString("since",
			mcp.Description("Filter errors since this RFC3339 timestamp"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of errors to return"),
		),
	)

	s.AddTool(tool, handleLogErrors(backend))
}

func handleLogErrors(backend *client.BackendClient) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := args(request)
		params := url.Values{}

		if v, ok := args["session_id"].(string); ok && v != "" {
			params.Set("session_id", v)
		}
		if v, ok := args["agent_id"].(string); ok && v != "" {
			params.Set("agent_id", v)
		}
		if v, ok := args["since"].(string); ok && v != "" {
			params.Set("since", v)
		}
		if v, ok := args["limit"].(float64); ok {
			params.Set("limit", fmt.Sprintf("%d", int(v)))
		}

		result, err := backend.Get(ctx, "/api/v1/logs/errors", params)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to search errors: %v", err)), nil
		}

		return mcp.NewToolResultText(string(result)), nil
	}
}
