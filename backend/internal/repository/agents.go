// Package repository implements the data-access layer for the ai-agent-log-hub
// backend. Each "*Repo" struct wraps a pgxpool.Pool — a thread-safe PostgreSQL
// connection pool provided by the pgx library (github.com/jackc/pgx/v5/pgxpool).
//
// pgxpool.Pool manages a pool of database connections behind the scenes: when a
// query is executed, the pool lends an idle connection (or opens a new one) and
// returns it when the query completes. This avoids the overhead of opening a
// fresh TCP connection for every single database call.
//
// A common pattern you will see throughout this package is the SQL "UPSERT"
// (INSERT ... ON CONFLICT ... DO UPDATE). This is a single atomic statement
// that inserts a row if it does not exist, or updates it if it already does,
// avoiding the need for a separate "check then insert" round-trip.
package repository

import (
	"context"
	"fmt"

	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AgentRepo provides database access for agent records.
// It holds a reference to a pgxpool.Pool — a connection pool that safely shares
// database connections across many goroutines (concurrent tasks).
type AgentRepo struct {
	// db is the PostgreSQL connection pool used for all queries in this repo.
	db *pgxpool.Pool
}

// NewAgentRepo creates a new AgentRepo backed by the given pool.
func NewAgentRepo(db *pgxpool.Pool) *AgentRepo {
	return &AgentRepo{db: db}
}

// FindByID retrieves an agent by its ID. Returns nil, nil when not found.
//
// The SQL selects all columns from the agents table where agent_id matches the
// provided value ($1 is a placeholder that pgx fills with agentID safely,
// preventing SQL injection).
func (r *AgentRepo) FindByID(ctx context.Context, agentID string) (*model.Agent, error) {
	// Query: look up a single agent row by its primary key (agent_id).
	const q = `
		SELECT agent_id, display_name, status, first_seen, last_seen,
		       total_sessions, total_events, metadata, created_at, updated_at
		FROM agents
		WHERE agent_id = $1`

	// QueryRow returns at most one row. Scan reads the column values into the
	// struct fields in the same order they appear in the SELECT list.
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
//
// This uses the PostgreSQL "UPSERT" pattern:
//
//	INSERT ... ON CONFLICT (agent_id) DO UPDATE ...
//
// How it works:
//  1. Try to INSERT a new row with the given agent_id.
//  2. If a row with that agent_id already exists (a conflict on the unique
//     primary key), instead of failing, run the DO UPDATE clause which
//     refreshes the last_seen and updated_at timestamps to the current time.
//
// This is done in a single SQL statement, so it is atomic — no race conditions
// can occur even if two requests try to register the same agent at once.
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
	// Status filters by agent status (e.g. "active", "idle"). Empty string means all statuses.
	Status string
	// Limit is the maximum number of agents to return in one page.
	Limit int
	// Offset is the number of rows to skip (for pagination: page 2 with limit 20 uses offset 20).
	Offset int
}

// ListResult holds a page of agents together with the total un-paginated count.
type ListResult struct {
	Agents []*model.Agent
	Total  int
}

// List returns a paginated list of agents and the total count matching the filter.
//
// The method runs two queries:
//  1. A COUNT(*) query to find out how many agents match the filter in total
//     (ignoring pagination). This total is returned alongside the page so that
//     the frontend can show "page 3 of 12" style navigation.
//  2. A SELECT query with LIMIT/OFFSET to fetch just the current page of agents,
//     ordered by most-recently-seen first.
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
//
// The SQL "total_sessions = total_sessions + 1" is atomic — PostgreSQL
// guarantees that concurrent increments will not lose updates because the read
// and write happen within the same statement.
func (r *AgentRepo) IncrementSessions(ctx context.Context, agentID string) error {
	// Query: add 1 to the agent's session counter and refresh updated_at.
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
