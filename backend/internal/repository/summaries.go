package repository

import (
	"context"
	"fmt"

	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SummaryRepo provides database access for session_summaries.
type SummaryRepo struct {
	pool *pgxpool.Pool
}

// NewSummaryRepo creates a SummaryRepo backed by the given pool.
func NewSummaryRepo(pool *pgxpool.Pool) *SummaryRepo {
	return &SummaryRepo{pool: pool}
}

// Create inserts a new session summary row, populating ID and CreatedAt from
// the database RETURNING clause.
func (r *SummaryRepo) Create(ctx context.Context, summary *model.SessionSummary) error {
	const q = `
		INSERT INTO session_summaries
			(session_id, agent_id, duration_seconds, event_count,
			 files_modified, tools_used, commits, errors, timeline, summary_text)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at`

	row := r.pool.QueryRow(ctx, q,
		summary.SessionID,
		summary.AgentID,
		summary.DurationSeconds,
		summary.EventCount,
		summary.FilesModified,
		summary.ToolsUsed,
		summary.Commits,
		summary.Errors,
		summary.Timeline,
		summary.SummaryText,
	)
	if err := row.Scan(&summary.ID, &summary.CreatedAt); err != nil {
		return fmt.Errorf("SummaryRepo.Create: %w", err)
	}
	return nil
}

// GetBySessionID retrieves the summary for a specific session.
// Returns nil, nil when no summary exists.
func (r *SummaryRepo) GetBySessionID(ctx context.Context, sessionID uuid.UUID) (*model.SessionSummary, error) {
	const q = `
		SELECT id, session_id, agent_id, duration_seconds, event_count,
		       files_modified, tools_used, commits, errors, timeline,
		       summary_text, created_at
		FROM session_summaries
		WHERE session_id = $1`

	s := &model.SessionSummary{}
	err := r.pool.QueryRow(ctx, q, sessionID).Scan(
		&s.ID, &s.SessionID, &s.AgentID, &s.DurationSeconds, &s.EventCount,
		&s.FilesModified, &s.ToolsUsed, &s.Commits, &s.Errors, &s.Timeline,
		&s.SummaryText, &s.CreatedAt,
	)
	if err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("SummaryRepo.GetBySessionID: %w", err)
	}
	return s, nil
}

// GetByAgentID returns summaries for an agent, ordered by creation time descending,
// along with the total count (for pagination).
func (r *SummaryRepo) GetByAgentID(ctx context.Context, agentID string, limit, offset int) ([]model.SessionSummary, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM session_summaries WHERE agent_id = $1", agentID,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("SummaryRepo.GetByAgentID count: %w", err)
	}

	const q = `
		SELECT id, session_id, agent_id, duration_seconds, event_count,
		       files_modified, tools_used, commits, errors, timeline,
		       summary_text, created_at
		FROM session_summaries
		WHERE agent_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.pool.Query(ctx, q, agentID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("SummaryRepo.GetByAgentID query: %w", err)
	}
	defer rows.Close()

	var summaries []model.SessionSummary
	for rows.Next() {
		var s model.SessionSummary
		if err := rows.Scan(
			&s.ID, &s.SessionID, &s.AgentID, &s.DurationSeconds, &s.EventCount,
			&s.FilesModified, &s.ToolsUsed, &s.Commits, &s.Errors, &s.Timeline,
			&s.SummaryText, &s.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("SummaryRepo.GetByAgentID scan: %w", err)
		}
		summaries = append(summaries, s)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("SummaryRepo.GetByAgentID rows: %w", err)
	}
	return summaries, total, nil
}
