package tools

import (
	"context"
	"fmt"

	"github.com/AntoCandela/ai-agent-log-hub/mcp/internal/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// registerLogSession creates the "log.session" tool.
// Translates a session_id argument into GET /api/v1/sessions/<id>.
func registerLogSession(s *server.MCPServer, backend *client.BackendClient) {
	tool := mcp.NewTool("log.session",
		mcp.WithDescription("Get details for a specific session"),
		mcp.WithString("session_id",
			mcp.Required(),
			mcp.Description("The session ID to retrieve"),
		),
	)

	s.AddTool(tool, handleLogSession(backend))
}

// handleLogSession fetches details for a single session by ID.
func handleLogSession(backend *client.BackendClient) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sessionID, _ := args(request)["session_id"].(string)
		if sessionID == "" {
			return mcp.NewToolResultError("session_id is required"), nil
		}

		result, err := backend.Get(ctx, "/api/v1/sessions/"+sessionID, nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get session: %v", err)), nil
		}

		return mcp.NewToolResultText(string(result)), nil
	}
}
