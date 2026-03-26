package model

import (
	"time"

	"github.com/google/uuid"
)

// Session represents a single interaction session of an AI agent.
// A session starts when an agent begins working on a task and ends
// when the task is complete or the session times out due to inactivity.
// Multiple events belong to one session.
type Session struct {
	// SessionID is the primary key — a UUID generated when the session is created.
	SessionID uuid.UUID `json:"session_id"`
	// SessionToken is a short opaque token that agents use to associate
	// subsequent events with this session (acts like a bearer token).
	SessionToken string `json:"session_token"`
	// AgentID links this session to the agent that owns it.
	AgentID string `json:"agent_id"`
	// Status is the session's lifecycle state: "active", "completed", or "timed_out".
	Status string `json:"status"`
	// ProjectDir is the filesystem path the agent was working in, if known.
	// Pointer because it may be NULL (not every agent reports this).
	ProjectDir *string `json:"project_dir,omitempty"`
	// GitBranch is the git branch the agent was on, if known.
	// Pointer because it may be NULL.
	GitBranch *string `json:"git_branch,omitempty"`
	// StartedAt is when the session was created.
	StartedAt time.Time `json:"started_at"`
	// EndedAt is when the session was closed. Nil while the session is still active.
	EndedAt *time.Time `json:"ended_at,omitempty"`
	// LastEventAt is the timestamp of the most recent event in this session,
	// used to detect idle sessions that should be timed out.
	LastEventAt time.Time `json:"last_event_at"`
	// EventCount is the running total of events recorded in this session.
	EventCount int `json:"event_count"`
	// Pinned indicates whether the user has pinned this session in the UI
	// so it stays visible regardless of filters.
	Pinned bool `json:"pinned"`
}
