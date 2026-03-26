package capture

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/AntoCandela/ai-agent-log-hub/hooks/internal/sanitize"
)

// HookInput represents the JSON payload Claude Code passes via stdin.
type HookInput struct {
	ToolName   string                 `json:"tool_name"`
	ToolInput  map[string]interface{} `json:"tool_input"`
	ToolOutput map[string]interface{} `json:"tool_output"`
	DurationMs int                    `json:"duration_ms"`
	SessionID  string                 `json:"session_id"`
	ProjectDir string                 `json:"project_dir"`
	GitBranch  string                 `json:"git_branch"`
}

// LogHubEvent is the payload sent to the Log Hub backend.
type LogHubEvent struct {
	AgentID      string                 `json:"agent_id"`
	SessionToken string                 `json:"session_token"`
	Timestamp    string                 `json:"timestamp"`
	EventType    string                 `json:"event_type"`
	Severity     string                 `json:"severity"`
	ToolName     string                 `json:"tool_name"`
	ToolType     string                 `json:"tool_type"`
	Message      string                 `json:"message"`
	Params       map[string]interface{} `json:"params"`
	Result       map[string]interface{} `json:"result"`
	Context      map[string]interface{} `json:"context"`
	DurationMs   int                    `json:"duration_ms"`
}

// ParseHookInput deserializes the raw JSON from Claude Code into a HookInput.
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

// BuildEvent converts a HookInput into a LogHubEvent ready for the backend API.
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

func determineEventType(toolName string) string {
	lower := strings.ToLower(toolName)
	if strings.Contains(lower, "commit") {
		return "git_commit"
	}
	return "tool_call"
}

func determineToolType(toolName string) string {
	if strings.HasPrefix(toolName, "mcp__") {
		return "mcp"
	}
	return "builtin"
}

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
