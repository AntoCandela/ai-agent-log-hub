package tools

import (
	"context"
	"fmt"

	"github.com/AntoCandela/ai-agent-log-hub/mcp/internal/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerMemoryRecall(s *server.MCPServer, backend *client.BackendClient) {
	tool := mcp.NewTool("memory.recall",
		mcp.WithDescription("Recall a specific memory by key"),
		mcp.WithString("key",
			mcp.Required(),
			mcp.Description("Key of the memory to recall"),
		),
		mcp.WithString("namespace",
			mcp.Description("Namespace the memory belongs to"),
		),
	)

	s.AddTool(tool, handleMemoryRecall(backend))
}

func handleMemoryRecall(backend *client.BackendClient) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := args(request)

		key, _ := args["key"].(string)
		if key == "" {
			return mcp.NewToolResultError("key is required"), nil
		}

		body := map[string]any{
			"key": key,
		}
		if ns, ok := args["namespace"].(string); ok && ns != "" {
			body["namespace"] = ns
		}

		result, err := backend.Post(ctx, "/api/v1/memory/recall", body)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to recall memory: %v", err)), nil
		}

		return mcp.NewToolResultText(string(result)), nil
	}
}
