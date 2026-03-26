package tools

import (
	"context"
	"fmt"
	"net/url"

	"github.com/AntoCandela/ai-agent-log-hub/mcp/internal/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerLogSystem(s *server.MCPServer, backend *client.BackendClient) {
	tool := mcp.NewTool("log.system",
		mcp.WithDescription("Query system-level events (build, test, deploy)"),
		mcp.WithString("kind",
			mcp.Description("Filter by event kind"),
		),
		mcp.WithString("since",
			mcp.Description("Filter events since this RFC3339 timestamp"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of results to return"),
		),
	)

	s.AddTool(tool, handleLogSystem(backend))
}

func handleLogSystem(backend *client.BackendClient) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := args(request)
		params := url.Values{}

		if v, ok := args["kind"].(string); ok && v != "" {
			params.Set("kind", v)
		}
		if v, ok := args["since"].(string); ok && v != "" {
			params.Set("since", v)
		}
		if v, ok := args["limit"].(float64); ok {
			params.Set("limit", fmt.Sprintf("%d", int(v)))
		}

		result, err := backend.Get(ctx, "/api/v1/system", params)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to query system events: %v", err)), nil
		}

		return mcp.NewToolResultText(string(result)), nil
	}
}
