package tools

import (
	"context"
	"fmt"
	"net/url"

	"github.com/AntoCandela/ai-agent-log-hub/mcp/internal/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerMemoryList(s *server.MCPServer, backend *client.BackendClient) {
	tool := mcp.NewTool("memory.list",
		mcp.WithDescription("List all stored memory entries"),
		mcp.WithString("namespace",
			mcp.Description("Filter memories by namespace"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of memories to return"),
		),
	)

	s.AddTool(tool, handleMemoryList(backend))
}

func handleMemoryList(backend *client.BackendClient) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := args(request)
		params := url.Values{}

		if v, ok := args["namespace"].(string); ok && v != "" {
			params.Set("namespace", v)
		}
		if v, ok := args["limit"].(float64); ok {
			params.Set("limit", fmt.Sprintf("%d", int(v)))
		}

		result, err := backend.Get(ctx, "/api/v1/memory", params)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to list memories: %v", err)), nil
		}

		return mcp.NewToolResultText(string(result)), nil
	}
}
