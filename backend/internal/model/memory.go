package model

import (
	"time"

	"github.com/google/uuid"
)

// Memory represents a key-value memory entry stored by an agent.
// Agents can persist facts, preferences, or context across sessions
// using the memory API. Each memory has a unique key scoped to an
// agent, and can optionally be tied to a specific session or shared
// across all agents.
type Memory struct {
	// MemoryID is the primary key (UUID).
	MemoryID uuid.UUID `json:"memory_id"`
	// AgentID identifies which agent created this memory entry.
	AgentID string `json:"agent_id"`
	// SessionID optionally scopes this memory to a particular session.
	// Nil means the memory is session-independent.
	SessionID *uuid.UUID `json:"session_id"`
	// Key is the lookup name for this memory (e.g. "preferred_language",
	// "project_context"). Must be unique per agent.
	Key string `json:"key"`
	// Value is the stored content — typically a short string or
	// serialized JSON.
	Value string `json:"value"`
	// Tags is a list of free-form labels for organizing and filtering
	// memories (e.g. ["config", "user-pref"]).
	Tags []string `json:"tags"`
	// Shared indicates whether other agents can read this memory.
	// When false, only the owning agent can access it.
	Shared bool `json:"shared"`
	// CreatedAt is when this memory was first stored.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is when this memory was last modified.
	UpdatedAt time.Time `json:"updated_at"`
}
