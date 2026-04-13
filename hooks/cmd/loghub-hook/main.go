// TOCHANGE: Enhanced hook system for stack migration
// - Add: parent_span_id, trace_id, span_id for causality tracking (REQ-22)
// - Add: sub-agent detection when tool_name is "Agent" (CON-21 multi-agent topology)
// - Add: severity inference from tool output (detect errors, not just hardcode "info")
// - Add: batch sending (queue locally, flush every 10 events or 5 seconds)
// - Add: MCP server identification from mcp__ tool name prefix
// - Keep: sanitization, basic event capture, POST to backend
// - See autok design fragment DES-7 (Enhanced Hook System)
//
// Package main is the entry point for the loghub-hook binary.
//
// Claude Code Hooks
// -----------------
// Claude Code supports "hooks" -- small executables that run automatically at
// specific points in the agent's lifecycle (e.g., after a tool call finishes).
// Claude Code invokes the hook binary, passes a JSON payload on **stdin**
// describing what just happened (tool name, input, output, duration, etc.),
// and expects the process to exit quickly without producing output on stdout.
//
// Fire-and-Forget Pattern
// -----------------------
// This binary is designed to never interfere with Claude Code's normal
// operation. Every error path calls os.Exit(0) silently -- no stderr output,
// no non-zero exit codes. A 5-second HTTP timeout ensures the hook does not
// hang. If the backend is down or the payload is malformed, the hook simply
// exits without reporting the failure. This is intentional: a logging
// side-channel must never break the primary tool.
//
// Configuration (environment variables):
//   - LOGHUB_URL      -- backend base URL (default http://localhost:4800)
//   - LOGHUB_API_KEY  -- optional Bearer token
//   - LOGHUB_AGENT_ID -- agent identifier (default "claude-code-<hostname>")
package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/AntoCandela/ai-agent-log-hub/hooks/internal/capture"
)

func main() {
	// --- Configuration from environment ---
	loghubURL := os.Getenv("LOGHUB_URL")
	if loghubURL == "" {
		loghubURL = "http://localhost:4800"
	}
	apiKey := os.Getenv("LOGHUB_API_KEY")
	agentID := os.Getenv("LOGHUB_AGENT_ID")
	if agentID == "" {
		hostname, _ := os.Hostname()
		agentID = "claude-code-" + hostname
	}

	// --- Read the hook payload from stdin ---
	// Claude Code pipes a JSON object describing the tool call into stdin.
	input, err := io.ReadAll(os.Stdin)
	if err != nil || len(input) == 0 {
		os.Exit(0) // Silent exit -- never break Claude Code.
	}

	// --- Parse the JSON and build a LogHubEvent ---
	hookInput, err := capture.ParseHookInput(input)
	if err != nil {
		os.Exit(0) // Silent exit -- malformed payload is not fatal.
	}

	event := capture.BuildEvent(hookInput, agentID)

	// --- POST the event to the Log Hub backend ---
	// This is fire-and-forget: a short timeout prevents blocking, and any
	// error is swallowed so Claude Code is never affected.
	body, err := json.Marshal(event)
	if err != nil {
		os.Exit(0)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("POST", loghubURL+"/api/v1/events", bytes.NewReader(body))
	if err != nil {
		os.Exit(0)
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		os.Exit(0) // Silent -- backend might be unreachable.
	}
	resp.Body.Close()
}
