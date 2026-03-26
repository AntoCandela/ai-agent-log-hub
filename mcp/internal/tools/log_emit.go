package tools

// -----------------------------------------------------------------------
// Tool-to-REST Translation Pattern (applies to every tool file)
// -----------------------------------------------------------------------
//
// Each file in this package follows the same two-function pattern:
//
//   registerXxx(s, backend)
//       Creates the MCP tool schema (name, description, typed parameters)
//       using mcp.NewTool / mcp.WithString / mcp.WithNumber / etc., then
//       calls s.AddTool to register it on the MCP server together with a
//       handler function.
//
//   handleXxx(backend) -> server.ToolHandlerFunc
//       Returns a closure that the MCP server calls when the AI agent
//       invokes the tool. Inside the closure:
//         1. Extract arguments from the MCP request (via the args helper).
//         2. Validate required fields.
//         3. Translate the arguments into an HTTP request to the backend
//            REST API (GET with query params, or POST with a JSON body).
//         4. Return the backend's JSON response as MCP tool result text,
//            or return an MCP error result on failure.
//
// This pattern means every MCP tool is a thin adapter: MCP request in,
// REST call out, JSON response back. No business logic lives here.
// -----------------------------------------------------------------------

import (
	"context"
	"fmt"

	"github.com/AntoCandela/ai-agent-log-hub/mcp/internal/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// registerLogEmit creates the "log.emit" MCP tool and binds it to the server.
// This tool lets the AI agent send a structured log event to the backend.
func registerLogEmit(s *server.MCPServer, backend *client.BackendClient) {
	// mcp.NewTool defines the tool's name and, via With* helpers, its
	// typed parameter schema. The MCP host uses this schema to validate
	// arguments before calling the handler.
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

	// s.AddTool binds the schema to its handler so the MCP server knows
	// how to dispatch incoming "log.emit" calls.
	s.AddTool(tool, handleLogEmit(backend))
}

// handleLogEmit returns the handler closure for "log.emit".
// It translates MCP arguments into a POST /api/v1/events request.
func handleLogEmit(backend *client.BackendClient) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Step 1: Extract arguments from the MCP request.
		args := args(request)

		// Step 2: Validate required fields.
		message, _ := args["message"].(string)
		if message == "" {
			return mcp.NewToolResultError("message is required"), nil
		}

		severity, _ := args["severity"].(string)
		if severity == "" {
			severity = "info"
		}

		// Step 3: Build the JSON body expected by the backend REST API.
		// The agent_id is injected automatically from the client config.
		event := map[string]any{
			"agent_id": backend.AgentID(),
			"message":  message,
			"severity": severity,
		}

		// Optional fields are included only when the caller provided them.
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

		// The backend expects events wrapped in an "events" array.
		payload := map[string]any{
			"events": []any{event},
		}

		// Step 4: POST to the backend and return the JSON response as text.
		result, err := backend.Post(ctx, "/api/v1/events", payload)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to emit log: %v", err)), nil
		}

		return mcp.NewToolResultText(string(result)), nil
	}
}
