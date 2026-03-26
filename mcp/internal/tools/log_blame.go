package tools

import (
	"context"
	"fmt"
	"net/url"

	"github.com/AntoCandela/ai-agent-log-hub/mcp/internal/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// registerLogBlame creates the "log.blame" tool.
// Translates optional filters into GET /api/v1/logs/blame.
func registerLogBlame(s *server.MCPServer, backend *client.BackendClient) {
	tool := mcp.NewTool("log.blame",
		mcp.WithDescription("Get blame information showing which agent modified which files"),
		mcp.WithString("file_path",
			mcp.Description("Filter blame by file path"),
		),
		mcp.WithString("session_id",
			mcp.Description("Filter blame by session ID"),
		),
		mcp.WithString("agent_id",
			mcp.Description("Filter blame by agent ID"),
		),
	)

	s.AddTool(tool, handleLogBlame(backend))
}

// handleLogBlame returns blame info (which agent modified which files).
func handleLogBlame(backend *client.BackendClient) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := args(request)
		params := url.Values{}

		if v, ok := args["file_path"].(string); ok && v != "" {
			params.Set("file_path", v)
		}
		if v, ok := args["session_id"].(string); ok && v != "" {
			params.Set("session_id", v)
		}
		if v, ok := args["agent_id"].(string); ok && v != "" {
			params.Set("agent_id", v)
		}

		result, err := backend.Get(ctx, "/api/v1/logs/blame", params)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get blame: %v", err)), nil
		}

		return mcp.NewToolResultText(string(result)), nil
	}
}
