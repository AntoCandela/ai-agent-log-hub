package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// SessionSummary holds aggregated metrics and a textual summary for a closed session.
type SessionSummary struct {
	ID              uuid.UUID       `json:"id"`
	SessionID       uuid.UUID       `json:"session_id"`
	AgentID         string          `json:"agent_id"`
	DurationSeconds int             `json:"duration_seconds"`
	EventCount      int             `json:"event_count"`
	FilesModified   json.RawMessage `json:"files_modified"`
	ToolsUsed       json.RawMessage `json:"tools_used"`
	Commits         json.RawMessage `json:"commits"`
	Errors          json.RawMessage `json:"errors"`
	Timeline        json.RawMessage `json:"timeline"`
	SummaryText     *string         `json:"summary_text"`
	CreatedAt       time.Time       `json:"created_at"`
}
