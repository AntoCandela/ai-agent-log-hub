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
	// Read configuration from environment.
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

	// Read hook payload from stdin.
	input, err := io.ReadAll(os.Stdin)
	if err != nil || len(input) == 0 {
		os.Exit(0) // Silent exit — don't break Claude Code.
	}

	// Parse and build the event.
	hookInput, err := capture.ParseHookInput(input)
	if err != nil {
		os.Exit(0) // Silent exit.
	}

	event := capture.BuildEvent(hookInput, agentID)

	// POST to the Log Hub backend (fire-and-forget with short timeout).
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
		os.Exit(0) // Silent — don't break Claude Code.
	}
	resp.Body.Close()
}
