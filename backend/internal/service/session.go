package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/tobias/ai-agent-log-hub/backend/internal/model"
)

// sessionRepo is the minimal interface SessionService needs from the repository layer.
type sessionRepo interface {
	FindActiveByToken(ctx context.Context, agentID, sessionToken string) (*model.Session, error)
	Create(ctx context.Context, s *model.Session) error
	Close(ctx context.Context, sessionID uuid.UUID) error
}

// sessionAgentRepo is the slice of AgentRepo that SessionService needs.
type sessionAgentRepo interface {
	IncrementSessions(ctx context.Context, agentID string) error
}

// SessionService handles business logic for session lifecycle.
type SessionService struct {
	sessions sessionRepo
	agents   sessionAgentRepo
}

// NewSessionService creates a SessionService that delegates persistence to the
// provided session and agent repositories.
func NewSessionService(sessions sessionRepo, agents sessionAgentRepo) *SessionService {
	return &SessionService{sessions: sessions, agents: agents}
}

// ResolveSession finds the active session matching agentID + sessionToken, or
// creates a new one if none exists. When a new session is created the agent's
// total_sessions counter is atomically incremented.
func (s *SessionService) ResolveSession(
	ctx context.Context,
	agentID, sessionToken string,
	projectDir, gitBranch *string,
) (*model.Session, error) {
	existing, err := s.sessions.FindActiveByToken(ctx, agentID, sessionToken)
	if err != nil {
		return nil, fmt.Errorf("SessionService.ResolveSession: %w", err)
	}
	if existing != nil {
		return existing, nil
	}

	// No active session found — create a new one.
	sess := &model.Session{
		SessionToken: sessionToken,
		AgentID:      agentID,
		Status:       "active",
		ProjectDir:   projectDir,
		GitBranch:    gitBranch,
	}
	if err := s.sessions.Create(ctx, sess); err != nil {
		return nil, fmt.Errorf("SessionService.ResolveSession create: %w", err)
	}

	// Keep the agent's counter in sync; log but don't abort on failure.
	if err := s.agents.IncrementSessions(ctx, agentID); err != nil {
		return nil, fmt.Errorf("SessionService.ResolveSession increment: %w", err)
	}

	return sess, nil
}

// CloseSession marks the session as closed.
// TODO: trigger async summary generation once the summariser is implemented.
func (s *SessionService) CloseSession(ctx context.Context, sessionID uuid.UUID) error {
	if err := s.sessions.Close(ctx, sessionID); err != nil {
		return fmt.Errorf("SessionService.CloseSession: %w", err)
	}
	// TODO: enqueue summary generation job for sessionID.
	return nil
}
