package service

import (
	"context"
	"fmt"

	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/model"
)

// agentRepo is the minimal interface AgentService needs from the repository layer.
type agentRepo interface {
	Upsert(ctx context.Context, agentID string) error
}

// AgentService handles business logic for agent lifecycle.
type AgentService struct {
	repo agentRepo
}

// NewAgentService creates an AgentService that delegates persistence to repo.
func NewAgentService(repo agentRepo) *AgentService {
	return &AgentService{repo: repo}
}

// EnsureExists validates the agent ID and upserts the agent record so that it
// is always present before any session or event is processed.
func (s *AgentService) EnsureExists(ctx context.Context, agentID string) error {
	if err := model.ValidateAgentID(agentID); err != nil {
		return fmt.Errorf("AgentService.EnsureExists: %w", err)
	}
	if err := s.repo.Upsert(ctx, agentID); err != nil {
		return fmt.Errorf("AgentService.EnsureExists: %w", err)
	}
	return nil
}
