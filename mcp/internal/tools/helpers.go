package tools

import "github.com/mark3labs/mcp-go/mcp"

// args extracts the arguments map from a CallToolRequest.
func args(request mcp.CallToolRequest) map[string]any {
	if m, ok := request.Params.Arguments.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}
