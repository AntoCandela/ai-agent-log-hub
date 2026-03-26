// Package capture builds structured log events from raw Claude Code hook
// payloads.
//
// When Claude Code fires a hook, it sends a JSON object on stdin describing
// the tool call that just completed. This package:
//  1. Parses that JSON into a HookInput struct.
//  2. Enriches it with metadata (agent ID, timestamp, event/tool type).
//  3. Sanitizes sensitive fields (via the sanitize package).
//  4. Produces a LogHubEvent ready to POST to the backend REST API.
package capture

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/AntoCandela/ai-agent-log-hub/hooks/internal/sanitize"
)

// HookInput represents the JSON payload Claude Code passes via stdin when a
// hook fires. Fields map directly to the keys in that JSON object.
type HookInput struct {
	ToolName   string                 `json:"tool_name"`   // e.g. "Edit", "Bash", "mcp__loghub__log.emit"
	ToolInput  map[string]interface{} `json:"tool_input"`  // arguments the agent passed to the tool
	ToolOutput map[string]interface{} `json:"tool_output"` // result the tool returned
	DurationMs int                    `json:"duration_ms"` // wall-clock time the tool call took
	SessionID  string                 `json:"session_id"`  // unique ID for the current Claude Code session
	ProjectDir string                 `json:"project_dir"` // working directory of the session
	GitBranch  string                 `json:"git_branch"`  // current git branch (if available)
}

// LogHubEvent is the payload sent to the Log Hub backend's POST /api/v1/events
// endpoint. It combines data from the hook input with server-side enrichment
// (agent ID, timestamp, sanitized params/results).
type LogHubEvent struct {
	AgentID      string                 `json:"agent_id"`
	SessionToken string                 `json:"session_token"`
	Timestamp    string                 `json:"timestamp"`
	EventType    string                 `json:"event_type"` // "tool_call" or "git_commit"
	Severity     string                 `json:"severity"`
	ToolName     string                 `json:"tool_name"`
	ToolType     string                 `json:"tool_type"` // "builtin" or "mcp"
	Message      string                 `json:"message"`
	Params       map[string]interface{} `json:"params"`  // sanitized tool input
	Result       map[string]interface{} `json:"result"`  // sanitized tool output
	Context      map[string]interface{} `json:"context"` // project_dir, git_branch
	DurationMs   int                    `json:"duration_ms"`
}

// ParseHookInput deserializes the raw JSON from Claude Code into a HookInput.
// Returns an error if the JSON is malformed or the required tool_name field is
// missing.
func ParseHookInput(data []byte) (*HookInput, error) {
	var h HookInput
	if err := json.Unmarshal(data, &h); err != nil {
		return nil, fmt.Errorf("parse hook input: %w", err)
	}
	if h.ToolName == "" {
		return nil, fmt.Errorf("parse hook input: tool_name is required")
	}
	return &h, nil
}

// BuildEvent converts a HookInput into a LogHubEvent ready for the backend
// API. It determines the event type and tool type from the tool name,
// generates a human-readable message, and runs the sanitizer over tool
// input/output to redact secrets before they leave the machine.
func BuildEvent(input *HookInput, agentID string) *LogHubEvent {
	eventType := determineEventType(input.ToolName)
	toolType := determineToolType(input.ToolName)
	message := buildMessage(input.ToolName)

	return &LogHubEvent{
		AgentID:      agentID,
		SessionToken: input.SessionID,
		Timestamp:    time.Now().UTC().Format(time.RFC3339),
		EventType:    eventType,
		Severity:     "info",
		ToolName:     input.ToolName,
		ToolType:     toolType,
		Message:      message,
		Params:       sanitize.RedactJSON(input.ToolInput),
		Result:       sanitize.RedactJSON(input.ToolOutput),
		Context: map[string]interface{}{
			"project_dir": input.ProjectDir,
			"git_branch":  input.GitBranch,
		},
		DurationMs: input.DurationMs,
	}
}

// determineEventType classifies the tool call. Commit-related tools get the
// special "git_commit" type; everything else is a generic "tool_call".
func determineEventType(toolName string) string {
	lower := strings.ToLower(toolName)
	if strings.Contains(lower, "commit") {
		return "git_commit"
	}
	return "tool_call"
}

// determineToolType distinguishes MCP tools (whose names start with "mcp__")
// from Claude Code's built-in tools (Edit, Bash, Read, etc.).
func determineToolType(toolName string) string {
	if strings.HasPrefix(toolName, "mcp__") {
		return "mcp"
	}
	return "builtin"
}

// buildMessage creates a short, human-readable summary of the tool call for
// display in the log hub dashboard.
func buildMessage(toolName string) string {
	lower := strings.ToLower(toolName)
	switch {
	case strings.Contains(lower, "commit"):
		return fmt.Sprintf("Git commit via %s", toolName)
	case strings.Contains(lower, "edit"):
		return fmt.Sprintf("Edited file via %s", toolName)
	case strings.Contains(lower, "read"):
		return fmt.Sprintf("Read file via %s", toolName)
	case strings.Contains(lower, "bash"):
		return fmt.Sprintf("Executed command via %s", toolName)
	default:
		return fmt.Sprintf("Used %s", toolName)
	}
}
