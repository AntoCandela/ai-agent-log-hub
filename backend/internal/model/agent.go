// Package model defines the core data structures (domain objects) used
// throughout the ai-agent-log-hub backend. Each struct maps to a
// database table and carries JSON tags so it can be serialized directly
// into API responses. Pointer fields (e.g. *string) represent columns
// that may be NULL in the database — Go uses nil to express "no value".
package model

import (
	"fmt"
	"regexp"
	"time"
)

// agentIDRegex matches valid agent IDs: 1-128 characters that are
// alphanumeric, hyphens, or underscores.  This is compiled once at
// package initialization time for performance.
var agentIDRegex = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

// Agent represents a registered AI agent in the system. Each distinct
// agent (e.g. "claude-code", "copilot") gets one row in the agents
// table. The struct tracks lifetime statistics like how many sessions
// the agent has had and when it was last active.
type Agent struct {
	// AgentID is the unique, human-readable identifier for the agent
	// (e.g. "claude-code-1"). Must match agentIDRegex.
	AgentID string `json:"agent_id"`
	// DisplayName is a friendly label shown in the UI (e.g. "Claude Code").
	DisplayName string `json:"display_name"`
	// Status is the agent's current state: "active", "idle", or "offline".
	Status string `json:"status"`
	// FirstSeen is the timestamp of the agent's very first event.
	FirstSeen time.Time `json:"first_seen"`
	// LastSeen is the timestamp of the agent's most recent event.
	LastSeen time.Time `json:"last_seen"`
	// TotalSessions is the lifetime count of sessions this agent has opened.
	TotalSessions int `json:"total_sessions"`
	// TotalEvents is the lifetime count of events emitted by this agent.
	TotalEvents int64 `json:"total_events"`
	// Metadata stores arbitrary agent-specific data as raw JSON bytes
	// (e.g. version info, capabilities). Stored as JSONB in PostgreSQL.
	Metadata []byte `json:"metadata"`
	// CreatedAt is when this agent row was first inserted.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is when this agent row was last modified.
	UpdatedAt time.Time `json:"updated_at"`
}

// ValidateAgentID returns an error if the given agent ID does not match
// the allowed pattern. Valid agent IDs are 1-128 characters and may
// only contain letters (A-Z, a-z), digits (0-9), hyphens (-), and
// underscores (_).
func ValidateAgentID(id string) error {
	if !agentIDRegex.MatchString(id) {
		return fmt.Errorf("invalid agent_id %q: must be 1-128 characters, alphanumeric, hyphens, or underscores", id)
	}
	return nil
}
