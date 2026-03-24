package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tobias/ai-agent-log-hub/backend/internal/model"
)

// AgentRepo provides database access for agent records.
type AgentRepo struct {
	db *pgxpool.Pool
}

// NewAgentRepo creates a new AgentRepo backed by the given pool.
func NewAgentRepo(db *pgxpool.Pool) *AgentRepo {
	return &AgentRepo{db: db}
}

// FindByID retrieves an agent by its ID. Returns nil, nil when not found.
func (r *AgentRepo) FindByID(ctx context.Context, agentID string) (*model.Agent, error) {
	const q = `
		SELECT agent_id, display_name, status, first_seen, last_seen,
		       total_sessions, total_events, metadata, created_at, updated_at
		FROM agents
		WHERE agent_id = $1`

	row := r.db.QueryRow(ctx, q, agentID)
	a := &model.Agent{}
	err := row.Scan(
		&a.AgentID, &a.DisplayName, &a.Status,
		&a.FirstSeen, &a.LastSeen,
		&a.TotalSessions, &a.TotalEvents,
		&a.Metadata, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		// pgx returns pgx.ErrNoRows; check by message to avoid importing pgx directly.
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("AgentRepo.FindByID: %w", err)
	}
	return a, nil
}

// Upsert inserts an agent or updates last_seen if a record already exists.
func (r *AgentRepo) Upsert(ctx context.Context, agentID string) error {
	const q = `
		INSERT INTO agents (agent_id)
		VALUES ($1)
		ON CONFLICT (agent_id) DO UPDATE
		  SET last_seen  = NOW(),
		      updated_at = NOW()`

	_, err := r.db.Exec(ctx, q, agentID)
	if err != nil {
		return fmt.Errorf("AgentRepo.Upsert: %w", err)
	}
	return nil
}

// UpdateLastSeen touches last_seen and updated_at for the given agent.
func (r *AgentRepo) UpdateLastSeen(ctx context.Context, agentID string) error {
	const q = `
		UPDATE agents
		SET last_seen  = NOW(),
		    updated_at = NOW()
		WHERE agent_id = $1`

	_, err := r.db.Exec(ctx, q, agentID)
	if err != nil {
		return fmt.Errorf("AgentRepo.UpdateLastSeen: %w", err)
	}
	return nil
}

// UpdateDisplayName sets a human-readable name for the agent.
func (r *AgentRepo) UpdateDisplayName(ctx context.Context, agentID, displayName string) error {
	const q = `
		UPDATE agents
		SET display_name = $2,
		    updated_at   = NOW()
		WHERE agent_id = $1`

	_, err := r.db.Exec(ctx, q, agentID, displayName)
	if err != nil {
		return fmt.Errorf("AgentRepo.UpdateDisplayName: %w", err)
	}
	return nil
}

// ListOptions controls filtering and pagination for List.
type ListOptions struct {
	// Status filters by agent status. Empty string means all statuses.
	Status string
	Limit  int
	Offset int
}

// ListResult holds a page of agents together with the total un-paginated count.
type ListResult struct {
	Agents []*model.Agent
	Total  int
}

// List returns a paginated list of agents and the total count matching the filter.
func (r *AgentRepo) List(ctx context.Context, opts ListOptions) (*ListResult, error) {
	var (
		countQ string
		listQ  string
		args   []any
	)

	if opts.Status != "" {
		countQ = `SELECT COUNT(*) FROM agents WHERE status = $1`
		listQ = `
			SELECT agent_id, display_name, status, first_seen, last_seen,
			       total_sessions, total_events, metadata, created_at, updated_at
			FROM agents
			WHERE status = $1
			ORDER BY last_seen DESC
			LIMIT $2 OFFSET $3`
		args = []any{opts.Status, opts.Limit, opts.Offset}
	} else {
		countQ = `SELECT COUNT(*) FROM agents`
		listQ = `
			SELECT agent_id, display_name, status, first_seen, last_seen,
			       total_sessions, total_events, metadata, created_at, updated_at
			FROM agents
			ORDER BY last_seen DESC
			LIMIT $1 OFFSET $2`
		args = []any{opts.Limit, opts.Offset}
	}

	// Get total count.
	var total int
	var countRow interface{ Scan(...any) error }
	if opts.Status != "" {
		countRow = r.db.QueryRow(ctx, countQ, opts.Status)
	} else {
		countRow = r.db.QueryRow(ctx, countQ)
	}
	if err := countRow.Scan(&total); err != nil {
		return nil, fmt.Errorf("AgentRepo.List count: %w", err)
	}

	// Fetch page.
	rows, err := r.db.Query(ctx, listQ, args...)
	if err != nil {
		return nil, fmt.Errorf("AgentRepo.List query: %w", err)
	}
	defer rows.Close()

	var agents []*model.Agent
	for rows.Next() {
		a := &model.Agent{}
		if err := rows.Scan(
			&a.AgentID, &a.DisplayName, &a.Status,
			&a.FirstSeen, &a.LastSeen,
			&a.TotalSessions, &a.TotalEvents,
			&a.Metadata, &a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("AgentRepo.List scan: %w", err)
		}
		agents = append(agents, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("AgentRepo.List rows: %w", err)
	}

	return &ListResult{Agents: agents, Total: total}, nil
}

// SetStatus updates the status field for the given agent.
func (r *AgentRepo) SetStatus(ctx context.Context, agentID, status string) error {
	const q = `
		UPDATE agents
		SET status     = $2,
		    updated_at = NOW()
		WHERE agent_id = $1`

	_, err := r.db.Exec(ctx, q, agentID, status)
	if err != nil {
		return fmt.Errorf("AgentRepo.SetStatus: %w", err)
	}
	return nil
}

// IncrementSessions atomically increments total_sessions for the given agent.
func (r *AgentRepo) IncrementSessions(ctx context.Context, agentID string) error {
	const q = `
		UPDATE agents
		SET total_sessions = total_sessions + 1,
		    updated_at     = NOW()
		WHERE agent_id = $1`

	_, err := r.db.Exec(ctx, q, agentID)
	if err != nil {
		return fmt.Errorf("AgentRepo.IncrementSessions: %w", err)
	}
	return nil
}
