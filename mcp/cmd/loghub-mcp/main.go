package main

import (
	"fmt"
	"os"

	"github.com/AntoCandela/ai-agent-log-hub/mcp/internal/client"
	"github.com/AntoCandela/ai-agent-log-hub/mcp/internal/tools"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	baseURL := os.Getenv("LOGHUB_URL")
	if baseURL == "" {
		baseURL = "http://localhost:4800"
	}

	apiKey := os.Getenv("LOGHUB_API_KEY")

	agentID := os.Getenv("LOGHUB_AGENT_ID")
	if agentID == "" {
		agentID = "claude-code"
	}

	backend := client.NewBackendClient(baseURL, apiKey, agentID)

	s := server.NewMCPServer(
		"loghub",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	tools.RegisterLogTools(s, backend)
	tools.RegisterMemoryTools(s, backend)

	fmt.Fprintf(os.Stderr, "loghub-mcp: tools registered, serving on stdio\n")

	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "loghub-mcp: server error: %v\n", err)
		os.Exit(1)
	}
}
