package tools

import (
	"context"
	"fmt"

	"github.com/AntoCandela/ai-agent-log-hub/mcp/internal/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// registerMemorySearch creates the "memory.search" tool.
// Translates a search query into POST /api/v1/memory/search.
func registerMemorySearch(s *server.MCPServer, backend *client.BackendClient) {
	tool := mcp.NewTool("memory.search",
		mcp.WithDescription("Search memories by semantic similarity"),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Search query for semantic matching"),
		),
		mcp.WithString("namespace",
			mcp.Description("Namespace to search within"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of results to return"),
		),
	)

	s.AddTool(tool, handleMemorySearch(backend))
}

// handleMemorySearch performs a semantic similarity search over stored memories.
func handleMemorySearch(backend *client.BackendClient) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := args(request)

		query, _ := args["query"].(string)
		if query == "" {
			return mcp.NewToolResultError("query is required"), nil
		}

		body := map[string]any{
			"query": query,
		}
		if ns, ok := args["namespace"].(string); ok && ns != "" {
			body["namespace"] = ns
		}
		if limit, ok := args["limit"].(float64); ok {
			body["limit"] = int(limit)
		}

		result, err := backend.Post(ctx, "/api/v1/memory/search", body)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to search memory: %v", err)), nil
		}

		return mcp.NewToolResultText(string(result)), nil
	}
}
