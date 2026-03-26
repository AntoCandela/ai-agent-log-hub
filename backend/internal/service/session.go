package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/model"
	"github.com/google/uuid"
)

// SummaryGenerator generates a summary for a closed session.
type SummaryGenerator interface {
	GenerateForSession(ctx context.Context, session *model.Session) error
}

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

// sessionGetter extends the session repo interface with GetByID for summary
// generation after close.
type sessionGetter interface {
	GetByID(ctx context.Context, sessionID uuid.UUID) (*model.Session, error)
}

// SessionService handles business logic for session lifecycle.
type SessionService struct {
	sessions  sessionRepo
	agents    sessionAgentRepo
	getter    sessionGetter
	summarize SummaryGenerator // nil = skip summary generation
}

// NewSessionService creates a SessionService that delegates persistence to the
// provided session and agent repositories.
func NewSessionService(sessions sessionRepo, agents sessionAgentRepo) *SessionService {
	return &SessionService{sessions: sessions, agents: agents}
}

// SetSummaryGenerator attaches a summary generator to be called on session close.
// Passing nil disables summary generation (the default for backward compat).
func (s *SessionService) SetSummaryGenerator(gen SummaryGenerator, getter sessionGetter) {
	s.summarize = gen
	s.getter = getter
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

// CloseSession marks the session as closed and triggers summary generation
// if a SummaryGenerator has been configured.
func (s *SessionService) CloseSession(ctx context.Context, sessionID uuid.UUID) error {
	if err := s.sessions.Close(ctx, sessionID); err != nil {
		return fmt.Errorf("SessionService.CloseSession: %w", err)
	}

	// Trigger summary generation if configured.
	if s.summarize != nil && s.getter != nil {
		session, err := s.getter.GetByID(ctx, sessionID)
		if err != nil {
			slog.Warn("CloseSession: failed to fetch session for summary", "error", err, "session_id", sessionID)
			return nil // don't fail the close
		}
		if session != nil {
			if err := s.summarize.GenerateForSession(ctx, session); err != nil {
				slog.Warn("CloseSession: summary generation failed", "error", err, "session_id", sessionID)
				// don't fail the close for summary errors
			}
		}
	}

	return nil
}
