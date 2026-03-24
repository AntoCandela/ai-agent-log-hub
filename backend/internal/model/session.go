package model

import (
	"time"

	"github.com/google/uuid"
)

// Session represents a single interaction session of an AI agent.
type Session struct {
	SessionID    uuid.UUID  `json:"session_id"`
	SessionToken string     `json:"session_token"`
	AgentID      string     `json:"agent_id"`
	Status       string     `json:"status"`
	ProjectDir   *string    `json:"project_dir,omitempty"`
	GitBranch    *string    `json:"git_branch,omitempty"`
	StartedAt    time.Time  `json:"started_at"`
	EndedAt      *time.Time `json:"ended_at,omitempty"`
	LastEventAt  time.Time  `json:"last_event_at"`
	EventCount   int        `json:"event_count"`
	Pinned       bool       `json:"pinned"`
}
