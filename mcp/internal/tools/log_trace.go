package tools

import (
	"context"
	"fmt"

	"github.com/AntoCandela/ai-agent-log-hub/mcp/internal/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerLogTrace(s *server.MCPServer, backend *client.BackendClient) {
	tool := mcp.NewTool("log.trace",
		mcp.WithDescription("Get all events associated with a trace ID"),
		mcp.WithString("trace_id",
			mcp.Required(),
			mcp.Description("The trace ID to look up"),
		),
	)

	s.AddTool(tool, handleLogTrace(backend))
}

func handleLogTrace(backend *client.BackendClient) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		traceID, _ := args(request)["trace_id"].(string)
		if traceID == "" {
			return mcp.NewToolResultError("trace_id is required"), nil
		}

		result, err := backend.Get(ctx, "/api/v1/traces/"+traceID, nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get trace: %v", err)), nil
		}

		return mcp.NewToolResultText(string(result)), nil
	}
}
