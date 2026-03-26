package model

import (
	"time"

	"github.com/google/uuid"
)

// Memory represents a key-value memory entry stored by an agent.
type Memory struct {
	MemoryID  uuid.UUID  `json:"memory_id"`
	AgentID   string     `json:"agent_id"`
	SessionID *uuid.UUID `json:"session_id"`
	Key       string     `json:"key"`
	Value     string     `json:"value"`
	Tags      []string   `json:"tags"`
	Shared    bool       `json:"shared"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}
