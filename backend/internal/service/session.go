package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/model"
	"github.com/google/uuid"
)

// SummaryGenerator generates a summary for a closed session.
// This interface is implemented by SummaryService and injected into
// SessionService so that a summary is automatically created when a session closes.
type SummaryGenerator interface {
	GenerateForSession(ctx context.Context, session *model.Session) error
}

// sessionRepo is the minimal interface SessionService needs from the repository
// layer for session CRUD operations.
type sessionRepo interface {
	FindActiveByToken(ctx context.Context, agentID, sessionToken string) (*model.Session, error)
	Create(ctx context.Context, s *model.Session) error
	Close(ctx context.Context, sessionID uuid.UUID) error
}

// sessionAgentRepo is the slice of AgentRepo that SessionService needs to keep
// the agent's session counter in sync.
type sessionAgentRepo interface {
	IncrementSessions(ctx context.Context, agentID string) error
}

// sessionGetter extends the session repo interface with GetByID, which is
// needed to reload the session after closing it so that the summary generator
// receives the full session data (including ended_at).
type sessionGetter interface {
	GetByID(ctx context.Context, sessionID uuid.UUID) (*model.Session, error)
}

// SessionService handles business logic for session lifecycle, including:
//   - Session resolution: finding or creating sessions for incoming events.
//   - Session closing: marking sessions as closed and triggering summary generation.
//   - Timeout detection: used by a background worker to close idle sessions.
type SessionService struct {
	sessions  sessionRepo      // CRUD for session records.
	agents    sessionAgentRepo // For incrementing the agent's session counter.
	getter    sessionGetter    // For reloading session data after close (used by summarizer).
	summarize SummaryGenerator // nil = skip summary generation on close.
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
//
// Session resolution flow:
//   1. Look up an existing active session by (agentID, sessionToken).
//   2. If found, return it — all subsequent events will be associated with this
//      session.
//   3. If not found, create a new session row in the database with status
//      "active", and increment the agent's total_sessions counter.
//
// This design means the client does not need to explicitly "start" a session.
// It just sends events with a session token, and the backend lazily creates the
// session on the first event. A background "timeout worker" (see
// SessionRepo.FindTimedOut) periodically closes sessions that have been idle
// for too long.
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
//
// Summary generation is best-effort: if it fails, the session is still
// successfully closed. This prevents summary bugs from blocking the normal
// session lifecycle. Errors are logged but not propagated to the caller.
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
