package tools

import (
	"context"
	"fmt"

	"github.com/AntoCandela/ai-agent-log-hub/mcp/internal/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerLogSummary(s *server.MCPServer, backend *client.BackendClient) {
	tool := mcp.NewTool("log.summary",
		mcp.WithDescription("Get a summary of a session including event counts and timeline"),
		mcp.WithString("session_id",
			mcp.Required(),
			mcp.Description("The session ID to summarize"),
		),
	)

	s.AddTool(tool, handleLogSummary(backend))
}

func handleLogSummary(backend *client.BackendClient) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sessionID, _ := args(request)["session_id"].(string)
		if sessionID == "" {
			return mcp.NewToolResultError("session_id is required"), nil
		}

		result, err := backend.Get(ctx, "/api/v1/sessions/"+sessionID+"/summary", nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get session summary: %v", err)), nil
		}

		return mcp.NewToolResultText(string(result)), nil
	}
}
