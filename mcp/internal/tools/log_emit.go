package tools

import (
	"context"
	"fmt"

	"github.com/AntoCandela/ai-agent-log-hub/mcp/internal/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerLogEmit(s *server.MCPServer, backend *client.BackendClient) {
	tool := mcp.NewTool("log.emit",
		mcp.WithDescription("Emit a structured log event to the log hub"),
		mcp.WithString("message",
			mcp.Required(),
			mcp.Description("Log message"),
		),
		mcp.WithString("severity",
			mcp.Description("Log severity level"),
			mcp.Enum("debug", "info", "warn", "error"),
		),
		mcp.WithArray("tags",
			mcp.Description("Tags for filtering"),
		),
		mcp.WithString("session_id",
			mcp.Description("Session ID to associate the log with"),
		),
		mcp.WithString("trace_id",
			mcp.Description("Trace ID for distributed tracing"),
		),
		mcp.WithString("file_path",
			mcp.Description("File path related to this log event"),
		),
	)

	s.AddTool(tool, handleLogEmit(backend))
}

func handleLogEmit(backend *client.BackendClient) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := args(request)

		message, _ := args["message"].(string)
		if message == "" {
			return mcp.NewToolResultError("message is required"), nil
		}

		severity, _ := args["severity"].(string)
		if severity == "" {
			severity = "info"
		}

		event := map[string]any{
			"agent_id": backend.AgentID(),
			"message":  message,
			"severity": severity,
		}

		if tags, ok := args["tags"].([]any); ok {
			event["tags"] = tags
		}
		if sid, ok := args["session_id"].(string); ok && sid != "" {
			event["session_id"] = sid
		}
		if tid, ok := args["trace_id"].(string); ok && tid != "" {
			event["trace_id"] = tid
		}
		if fp, ok := args["file_path"].(string); ok && fp != "" {
			event["file_path"] = fp
		}

		payload := map[string]any{
			"events": []any{event},
		}

		result, err := backend.Post(ctx, "/api/v1/events", payload)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to emit log: %v", err)), nil
		}

		return mcp.NewToolResultText(string(result)), nil
	}
}
