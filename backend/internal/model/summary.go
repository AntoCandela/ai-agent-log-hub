package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// SessionSummary holds aggregated metrics and a textual summary for a
// completed session. It is generated after a session ends and gives a
// quick overview of what happened: how long the session lasted, which
// files were changed, which tools were used, and any errors that
// occurred.
type SessionSummary struct {
	// ID is the primary key for the summary row itself (UUID).
	ID uuid.UUID `json:"id"`
	// SessionID links this summary to the session it describes.
	SessionID uuid.UUID `json:"session_id"`
	// AgentID identifies which agent ran the session.
	AgentID string `json:"agent_id"`
	// DurationSeconds is the total wall-clock duration of the session.
	DurationSeconds int `json:"duration_seconds"`
	// EventCount is the total number of events recorded during the session.
	EventCount int `json:"event_count"`
	// FilesModified is a JSON array listing the file paths that were
	// created or changed during the session. Stored as json.RawMessage
	// because the exact structure is flexible (list of strings or objects).
	FilesModified json.RawMessage `json:"files_modified"`
	// ToolsUsed is a JSON array or object summarizing which tools were
	// invoked and how many times.
	ToolsUsed json.RawMessage `json:"tools_used"`
	// Commits is a JSON array of git commits made during the session
	// (hashes, messages, etc.).
	Commits json.RawMessage `json:"commits"`
	// Errors is a JSON array of error events that occurred during the session.
	Errors json.RawMessage `json:"errors"`
	// Timeline is a JSON array of key moments in the session, ordered
	// chronologically, used to render a visual timeline in the UI.
	Timeline json.RawMessage `json:"timeline"`
	// SummaryText is an optional natural-language summary of what the
	// agent accomplished. Nil if no summary has been generated yet.
	SummaryText *string `json:"summary_text"`
	// CreatedAt is when this summary was generated.
	CreatedAt time.Time `json:"created_at"`
}
