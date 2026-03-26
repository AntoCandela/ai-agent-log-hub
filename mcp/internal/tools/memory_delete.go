package tools

import (
	"context"
	"fmt"

	"github.com/AntoCandela/ai-agent-log-hub/mcp/internal/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// registerMemoryDelete creates the "memory.delete" tool.
// Translates a key into DELETE /api/v1/memory/<key>.
func registerMemoryDelete(s *server.MCPServer, backend *client.BackendClient) {
	tool := mcp.NewTool("memory.delete",
		mcp.WithDescription("Delete a memory entry by key"),
		mcp.WithString("key",
			mcp.Required(),
			mcp.Description("Key of the memory to delete"),
		),
	)

	s.AddTool(tool, handleMemoryDelete(backend))
}

// handleMemoryDelete removes a memory entry by its key.
func handleMemoryDelete(backend *client.BackendClient) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		key, _ := args(request)["key"].(string)
		if key == "" {
			return mcp.NewToolResultError("key is required"), nil
		}

		result, err := backend.Delete(ctx, "/api/v1/memory/"+key)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to delete memory: %v", err)), nil
		}

		return mcp.NewToolResultText(string(result)), nil
	}
}
