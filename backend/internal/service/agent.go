// Package service implements the business-logic layer for the ai-agent-log-hub
// backend. Services sit between the HTTP handlers and the repository layer:
// handlers parse requests and call services, services enforce business rules
// and orchestrate repository calls, and repositories talk to the database.
//
// This separation means that database details (SQL, pgx) do not leak into HTTP
// handlers, and HTTP details (request/response) do not leak into repositories.
package service

import (
	"context"
	"fmt"

	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/model"
)

// agentRepo is the minimal interface AgentService needs from the repository layer.
// Defining a small interface (rather than depending on the concrete AgentRepo
// struct) makes the service easier to test with mock implementations.
type agentRepo interface {
	Upsert(ctx context.Context, agentID string) error
}

// AgentService handles business logic for agent lifecycle.
// Its primary job is "auto-registration": ensuring that an agent record exists
// in the database before any events or sessions reference it.
type AgentService struct {
	repo agentRepo
}

// NewAgentService creates an AgentService that delegates persistence to repo.
func NewAgentService(repo agentRepo) *AgentService {
	return &AgentService{repo: repo}
}

// EnsureExists validates the agent ID and upserts the agent record so that it
// is always present before any session or event is processed.
//
// This is the "auto-registration" pattern: the first time an agent sends any
// data, EnsureExists is called. The Upsert in the repository will INSERT the
// agent if it has never been seen, or just refresh its last_seen timestamp if
// it already exists. This means agents never need an explicit registration
// step — they are registered on first contact.
func (s *AgentService) EnsureExists(ctx context.Context, agentID string) error {
	if err := model.ValidateAgentID(agentID); err != nil {
		return fmt.Errorf("AgentService.EnsureExists: %w", err)
	}
	if err := s.repo.Upsert(ctx, agentID); err != nil {
		return fmt.Errorf("AgentService.EnsureExists: %w", err)
	}
	return nil
}
