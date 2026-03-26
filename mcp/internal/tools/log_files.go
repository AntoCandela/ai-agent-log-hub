package tools

import (
	"context"
	"fmt"

	"github.com/AntoCandela/ai-agent-log-hub/mcp/internal/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerLogFiles(s *server.MCPServer, backend *client.BackendClient) {
	tool := mcp.NewTool("log.files",
		mcp.WithDescription("Get files touched during a session"),
		mcp.WithString("session_id",
			mcp.Required(),
			mcp.Description("The session ID to retrieve files for"),
		),
	)

	s.AddTool(tool, handleLogFiles(backend))
}

func handleLogFiles(backend *client.BackendClient) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sessionID, _ := args(request)["session_id"].(string)
		if sessionID == "" {
			return mcp.NewToolResultError("session_id is required"), nil
		}

		result, err := backend.Get(ctx, "/api/v1/sessions/"+sessionID+"/files", nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get session files: %v", err)), nil
		}

		return mcp.NewToolResultText(string(result)), nil
	}
}
