package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EmbeddingResult holds a single search result from the embeddings table.
type EmbeddingResult struct {
	SourceType string
	SourceID   uuid.UUID
	AgentID    string
	Content    string
	Similarity float64
	Shared     bool
}

// EmbeddingRepo provides database access for the embeddings table.
type EmbeddingRepo struct {
	pool *pgxpool.Pool
}

// NewEmbeddingRepo creates an EmbeddingRepo backed by the given pool.
func NewEmbeddingRepo(pool *pgxpool.Pool) *EmbeddingRepo {
	return &EmbeddingRepo{pool: pool}
}

// Store inserts an embedding row. It first deletes any existing embedding for
// the same (source_type, source_id) pair to allow updates.
func (r *EmbeddingRepo) Store(ctx context.Context, sourceType string, sourceID uuid.UUID, agentID, content string, embedding []float32, shared bool) error {
	// Remove any previous embedding for this source to allow re-generation.
	if err := r.DeleteBySource(ctx, sourceType, sourceID); err != nil {
		return err
	}

	const q = `
		INSERT INTO embeddings (source_type, source_id, agent_id, content, embedding, shared)
		VALUES ($1, $2, $3, $4, $5, $6)`

	if _, err := r.pool.Exec(ctx, q, sourceType, sourceID, agentID, content, embedding, shared); err != nil {
		return fmt.Errorf("EmbeddingRepo.Store: %w", err)
	}
	return nil
}

// DeleteBySource removes embeddings matching the given source_type and source_id.
func (r *EmbeddingRepo) DeleteBySource(ctx context.Context, sourceType string, sourceID uuid.UUID) error {
	const q = `DELETE FROM embeddings WHERE source_type = $1 AND source_id = $2`
	if _, err := r.pool.Exec(ctx, q, sourceType, sourceID); err != nil {
		return fmt.Errorf("EmbeddingRepo.DeleteBySource: %w", err)
	}
	return nil
}

// Search performs a vector similarity search on the embeddings table.
// Results are filtered by agent visibility (own agent or shared) and source types.
func (r *EmbeddingRepo) Search(ctx context.Context, queryEmbedding []float32, agentID string, sourceTypes []string, topK int, minSimilarity float64) ([]EmbeddingResult, error) {
	const q = `
		SELECT source_type, source_id, agent_id, content, shared,
		       1 - (embedding <=> $1) AS similarity
		FROM embeddings
		WHERE (agent_id = $2 OR shared = TRUE)
		  AND source_type = ANY($3)
		  AND 1 - (embedding <=> $1) > $4
		ORDER BY embedding <=> $1
		LIMIT $5`

	rows, err := r.pool.Query(ctx, q, queryEmbedding, agentID, sourceTypes, minSimilarity, topK)
	if err != nil {
		return nil, fmt.Errorf("EmbeddingRepo.Search: %w", err)
	}
	defer rows.Close()

	var results []EmbeddingResult
	for rows.Next() {
		var r EmbeddingResult
		if err := rows.Scan(&r.SourceType, &r.SourceID, &r.AgentID, &r.Content, &r.Shared, &r.Similarity); err != nil {
			return nil, fmt.Errorf("EmbeddingRepo.Search scan: %w", err)
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("EmbeddingRepo.Search rows: %w", err)
	}
	return results, nil
}
