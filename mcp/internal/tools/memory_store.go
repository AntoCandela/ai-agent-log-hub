package tools

import (
	"context"
	"fmt"

	"github.com/AntoCandela/ai-agent-log-hub/mcp/internal/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerMemoryStore(s *server.MCPServer, backend *client.BackendClient) {
	tool := mcp.NewTool("memory.store",
		mcp.WithDescription("Store a key-value memory entry for later recall"),
		mcp.WithString("key",
			mcp.Required(),
			mcp.Description("Unique key for the memory entry"),
		),
		mcp.WithString("value",
			mcp.Required(),
			mcp.Description("Value to store"),
		),
		mcp.WithString("namespace",
			mcp.Description("Namespace to organize memories"),
		),
		mcp.WithObject("metadata",
			mcp.Description("Additional metadata to store with the memory"),
		),
	)

	s.AddTool(tool, handleMemoryStore(backend))
}

func handleMemoryStore(backend *client.BackendClient) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := args(request)

		key, _ := args["key"].(string)
		value, _ := args["value"].(string)
		if key == "" || value == "" {
			return mcp.NewToolResultError("key and value are required"), nil
		}

		body := map[string]any{
			"key":   key,
			"value": value,
		}
		if ns, ok := args["namespace"].(string); ok && ns != "" {
			body["namespace"] = ns
		}
		if meta, ok := args["metadata"].(map[string]any); ok {
			body["metadata"] = meta
		}

		result, err := backend.Post(ctx, "/api/v1/memory", body)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to store memory: %v", err)), nil
		}

		return mcp.NewToolResultText(string(result)), nil
	}
}
