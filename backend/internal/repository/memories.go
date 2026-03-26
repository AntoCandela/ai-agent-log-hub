package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MemoryRepo handles persistence for agent memories.
// A "memory" is a key-value pair scoped to an agent, allowing agents to store
// and retrieve persistent state (e.g. user preferences, learned patterns).
// Memories can optionally be shared across agents.
type MemoryRepo struct {
	pool *pgxpool.Pool
}

// NewMemoryRepo creates a new MemoryRepo backed by the given connection pool.
func NewMemoryRepo(pool *pgxpool.Pool) *MemoryRepo {
	return &MemoryRepo{pool: pool}
}

// Upsert inserts a new memory or updates an existing one keyed by (agent_id, key).
// On return the memory's MemoryID, CreatedAt and UpdatedAt fields are populated.
//
// This uses the UPSERT pattern with ON CONFLICT (agent_id, key):
//   - The table has a unique constraint on the (agent_id, key) pair, meaning
//     each agent can have at most one memory with a given key.
//   - If no row exists for this agent+key, a new row is inserted.
//   - If a row already exists, the DO UPDATE clause overwrites value, tags,
//     shared, and refreshes updated_at.
//   - RETURNING gives back the memory_id and timestamps so the caller has the
//     full record without a second query.
func (r *MemoryRepo) Upsert(ctx context.Context, m *model.Memory) error {
	tags := m.Tags
	if tags == nil {
		tags = []string{}
	}

	err := r.pool.QueryRow(ctx,
		// Query: insert or update a memory keyed by (agent_id, key).
		`INSERT INTO memories (agent_id, session_id, key, value, tags, shared)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (agent_id, key) DO UPDATE
		   SET value = $4, tags = $5, shared = $6, updated_at = NOW()
		 RETURNING memory_id, created_at, updated_at`,
		m.AgentID, m.SessionID, m.Key, m.Value, tags, m.Shared,
	).Scan(&m.MemoryID, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert memory: %w", err)
	}
	return nil
}

// GetByKey retrieves a single memory by agent_id and key.
func (r *MemoryRepo) GetByKey(ctx context.Context, agentID, key string) (*model.Memory, error) {
	var m model.Memory
	err := r.pool.QueryRow(ctx,
		`SELECT memory_id, agent_id, session_id, key, value, tags, shared, created_at, updated_at
		 FROM memories
		 WHERE agent_id = $1 AND key = $2`,
		agentID, key,
	).Scan(&m.MemoryID, &m.AgentID, &m.SessionID, &m.Key, &m.Value, &m.Tags, &m.Shared, &m.CreatedAt, &m.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get memory by key: %w", err)
	}
	return &m, nil
}

// List returns memories for an agent, optionally filtered by tags (ANY overlap).
// Results are ordered by updated_at DESC. Returns the slice and total count.
// The tags filter uses the PostgreSQL array overlap operator (&&): it matches
// rows whose tags array shares at least one element with the provided tags.
func (r *MemoryRepo) List(ctx context.Context, agentID string, tags []string, limit, offset int) ([]model.Memory, int, error) {
	var conditions []string
	var args []any
	argIdx := 1

	conditions = append(conditions, fmt.Sprintf("agent_id = $%d", argIdx))
	args = append(args, agentID)
	argIdx++

	if len(tags) > 0 {
		conditions = append(conditions, fmt.Sprintf("tags && $%d", argIdx))
		args = append(args, tags)
		argIdx++
	}

	where := " WHERE " + strings.Join(conditions, " AND ")

	// Count total matching rows.
	var total int
	err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM memories"+where, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count memories: %w", err)
	}

	// Defaults.
	if limit <= 0 {
		limit = 50
	}
	if limit > 1000 {
		limit = 1000
	}

	listArgs := append(args, limit, offset)
	query := fmt.Sprintf(
		`SELECT memory_id, agent_id, session_id, key, value, tags, shared, created_at, updated_at
		 FROM memories%s ORDER BY updated_at DESC LIMIT $%d OFFSET $%d`,
		where, argIdx, argIdx+1)

	rows, err := r.pool.Query(ctx, query, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list memories: %w", err)
	}
	defer rows.Close()

	var memories []model.Memory
	for rows.Next() {
		var m model.Memory
		if err := rows.Scan(&m.MemoryID, &m.AgentID, &m.SessionID, &m.Key, &m.Value, &m.Tags, &m.Shared, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan memory: %w", err)
		}
		memories = append(memories, m)
	}
	return memories, total, nil
}

// Delete removes a memory by agent_id and key.
func (r *MemoryRepo) Delete(ctx context.Context, agentID, key string) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM memories WHERE agent_id = $1 AND key = $2`,
		agentID, key,
	)
	if err != nil {
		return fmt.Errorf("delete memory: %w", err)
	}
	return nil
}
