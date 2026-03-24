package model

import (
	"fmt"
	"regexp"
	"time"
)

// agentIDRegex matches valid agent IDs: 1-128 chars, alphanumeric + hyphens + underscores.
var agentIDRegex = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

// Agent represents a registered AI agent in the system.
type Agent struct {
	AgentID       string    `json:"agent_id"`
	DisplayName   string    `json:"display_name"`
	Status        string    `json:"status"`
	FirstSeen     time.Time `json:"first_seen"`
	LastSeen      time.Time `json:"last_seen"`
	TotalSessions int       `json:"total_sessions"`
	TotalEvents   int64     `json:"total_events"`
	Metadata      []byte    `json:"metadata"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// ValidateAgentID returns an error if the given agent ID is not valid.
// Valid agent IDs are 1-128 characters and may only contain alphanumeric
// characters, hyphens (-), and underscores (_).
func ValidateAgentID(id string) error {
	if !agentIDRegex.MatchString(id) {
		return fmt.Errorf("invalid agent_id %q: must be 1-128 characters, alphanumeric, hyphens, or underscores", id)
	}
	return nil
}
