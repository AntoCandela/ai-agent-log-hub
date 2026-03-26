package tools

import "github.com/mark3labs/mcp-go/mcp"

// args extracts the arguments map from a CallToolRequest.
//
// When the MCP host calls a tool, it sends the arguments as a generic
// interface{} value inside the request. In practice this is always a
// map[string]any produced by JSON deserialization. This helper performs
// the type assertion and returns an empty map if the arguments are nil
// or of an unexpected type, so callers never need nil checks.
func args(request mcp.CallToolRequest) map[string]any {
	if m, ok := request.Params.Arguments.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}
