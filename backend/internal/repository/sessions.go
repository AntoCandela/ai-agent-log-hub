package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/model"
)

// SessionFilters holds optional filter criteria for listing sessions.
type SessionFilters struct {
	// AgentID, when non-empty, restricts results to this agent.
	AgentID string
	// Status, when non-empty, restricts results to sessions with this status.
	Status string
	// ProjectDir, when non-nil, restricts results to sessions with this project dir.
	ProjectDir *string
	// Pinned, when non-nil, restricts results to pinned/unpinned sessions.
	Pinned *bool
}

// SessionRepo provides database access for session records.
type SessionRepo struct {
	db *pgxpool.Pool
}

// NewSessionRepo creates a new SessionRepo backed by the given pool.
func NewSessionRepo(db *pgxpool.Pool) *SessionRepo {
	return &SessionRepo{db: db}
}

// FindActiveByToken returns the active session for the given agent and session
// token, or nil if none exists.
func (r *SessionRepo) FindActiveByToken(ctx context.Context, agentID, sessionToken string) (*model.Session, error) {
	const q = `
		SELECT session_id, session_token, agent_id, status,
		       project_dir, git_branch,
		       started_at, ended_at, last_event_at,
		       event_count, pinned
		FROM sessions
		WHERE agent_id      = $1
		  AND session_token = $2
		  AND status        = 'active'`

	row := r.db.QueryRow(ctx, q, agentID, sessionToken)
	s, err := scanSession(row)
	if err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("SessionRepo.FindActiveByToken: %w", err)
	}
	return s, nil
}

// Create inserts a new session row and populates session_id, started_at, and
// last_event_at from the database RETURNING clause.
func (r *SessionRepo) Create(ctx context.Context, s *model.Session) error {
	const q = `
		INSERT INTO sessions (session_token, agent_id, status, project_dir, git_branch)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING session_id, started_at, last_event_at`

	row := r.db.QueryRow(ctx, q,
		s.SessionToken,
		s.AgentID,
		s.Status,
		s.ProjectDir,
		s.GitBranch,
	)
	if err := row.Scan(&s.SessionID, &s.StartedAt, &s.LastEventAt); err != nil {
		return fmt.Errorf("SessionRepo.Create: %w", err)
	}
	return nil
}

// Close marks a session as closed and records the ended_at timestamp.
func (r *SessionRepo) Close(ctx context.Context, sessionID uuid.UUID) error {
	const q = `
		UPDATE sessions
		SET status   = 'closed',
		    ended_at = NOW()
		WHERE session_id = $1`

	_, err := r.db.Exec(ctx, q, sessionID)
	if err != nil {
		return fmt.Errorf("SessionRepo.Close: %w", err)
	}
	return nil
}

// UpdateLastEvent sets last_event_at to NOW() and increments event_count by one.
func (r *SessionRepo) UpdateLastEvent(ctx context.Context, sessionID uuid.UUID) error {
	const q = `
		UPDATE sessions
		SET last_event_at = NOW(),
		    event_count   = event_count + 1
		WHERE session_id = $1`

	_, err := r.db.Exec(ctx, q, sessionID)
	if err != nil {
		return fmt.Errorf("SessionRepo.UpdateLastEvent: %w", err)
	}
	return nil
}

// FindTimedOut returns active sessions whose last_event_at is older than the
// given timeout duration.
func (r *SessionRepo) FindTimedOut(ctx context.Context, timeout time.Duration) ([]*model.Session, error) {
	const q = `
		SELECT session_id, session_token, agent_id, status,
		       project_dir, git_branch,
		       started_at, ended_at, last_event_at,
		       event_count, pinned
		FROM sessions
		WHERE status = 'active'
		  AND NOW() - last_event_at > $1::interval`

	rows, err := r.db.Query(ctx, q, timeout.String())
	if err != nil {
		return nil, fmt.Errorf("SessionRepo.FindTimedOut: %w", err)
	}
	defer rows.Close()

	var sessions []*model.Session
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			return nil, fmt.Errorf("SessionRepo.FindTimedOut scan: %w", err)
		}
		sessions = append(sessions, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("SessionRepo.FindTimedOut rows: %w", err)
	}
	return sessions, nil
}

// GetByID retrieves a session by its UUID. Returns nil, nil when not found.
func (r *SessionRepo) GetByID(ctx context.Context, sessionID uuid.UUID) (*model.Session, error) {
	const q = `
		SELECT session_id, session_token, agent_id, status,
		       project_dir, git_branch,
		       started_at, ended_at, last_event_at,
		       event_count, pinned
		FROM sessions
		WHERE session_id = $1`

	row := r.db.QueryRow(ctx, q, sessionID)
	s, err := scanSession(row)
	if err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("SessionRepo.GetByID: %w", err)
	}
	return s, nil
}

// SessionListResult holds a page of sessions together with the total
// un-paginated count.
type SessionListResult struct {
	Sessions []*model.Session
	Total    int
}

// List returns a paginated slice of sessions matching the given filters, along
// with the total count of matching rows (ignoring limit/offset).
func (r *SessionRepo) List(ctx context.Context, filters SessionFilters, limit, offset int) (*SessionListResult, error) {
	// Build the WHERE clause dynamically.
	var (
		conds []string
		args  []any
	)
	argN := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	if filters.AgentID != "" {
		conds = append(conds, "agent_id = "+argN(filters.AgentID))
	}
	if filters.Status != "" {
		conds = append(conds, "status = "+argN(filters.Status))
	}
	if filters.ProjectDir != nil {
		conds = append(conds, "project_dir = "+argN(*filters.ProjectDir))
	}
	if filters.Pinned != nil {
		conds = append(conds, "pinned = "+argN(*filters.Pinned))
	}

	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}

	countQ := "SELECT COUNT(*) FROM sessions " + where

	var total int
	if err := r.db.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("SessionRepo.List count: %w", err)
	}

	listQ := fmt.Sprintf(`
		SELECT session_id, session_token, agent_id, status,
		       project_dir, git_branch,
		       started_at, ended_at, last_event_at,
		       event_count, pinned
		FROM sessions
		%s
		ORDER BY started_at DESC
		LIMIT %s OFFSET %s`,
		where,
		argN(limit),
		argN(offset),
	)

	rows, err := r.db.Query(ctx, listQ, args...)
	if err != nil {
		return nil, fmt.Errorf("SessionRepo.List query: %w", err)
	}
	defer rows.Close()

	var sessions []*model.Session
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			return nil, fmt.Errorf("SessionRepo.List scan: %w", err)
		}
		sessions = append(sessions, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("SessionRepo.List rows: %w", err)
	}

	return &SessionListResult{Sessions: sessions, Total: total}, nil
}

// scanner is the common interface shared by pgx.Row and pgx.Rows.
type scanner interface {
	Scan(dest ...any) error
}

// scanSession scans one session row into a model.Session.
func scanSession(row scanner) (*model.Session, error) {
	s := &model.Session{}
	err := row.Scan(
		&s.SessionID, &s.SessionToken, &s.AgentID, &s.Status,
		&s.ProjectDir, &s.GitBranch,
		&s.StartedAt, &s.EndedAt, &s.LastEventAt,
		&s.EventCount, &s.Pinned,
	)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// isNoRows reports whether err is the pgx "no rows" sentinel.
func isNoRows(err error) bool {
	return err != nil && err.Error() == "no rows in result set"
}
