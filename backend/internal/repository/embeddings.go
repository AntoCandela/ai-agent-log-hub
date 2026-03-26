package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EmbeddingResult holds a single search result from the embeddings table.
type EmbeddingResult struct {
	SourceType string    // What kind of content was embedded (e.g. "session_summary", "memory").
	SourceID   uuid.UUID // The ID of the source record (e.g. the session summary UUID).
	AgentID    string    // The agent that owns this embedding.
	Content    string    // The original text that was embedded.
	Similarity float64   // Cosine similarity score (0.0 to 1.0, higher = more similar).
	Shared     bool      // Whether this embedding is visible to all agents.
}

// EmbeddingRepo provides database access for the embeddings table, which stores
// vector embeddings (arrays of floating-point numbers) alongside the text they
// represent. These embeddings are used for semantic similarity search — finding
// content that is conceptually related to a query, even if it uses different
// words.
//
// The table uses the pgvector extension, which adds a special "vector" column
// type and distance operators to PostgreSQL. An HNSW (Hierarchical Navigable
// Small World) index on the embedding column makes similarity searches fast
// even with millions of rows, by building a graph-based approximate nearest
// neighbor index.
type EmbeddingRepo struct {
	pool *pgxpool.Pool
}

// NewEmbeddingRepo creates an EmbeddingRepo backed by the given pool.
func NewEmbeddingRepo(pool *pgxpool.Pool) *EmbeddingRepo {
	return &EmbeddingRepo{pool: pool}
}

// Store inserts an embedding row. It first deletes any existing embedding for
// the same (source_type, source_id) pair to allow updates (delete-then-insert
// instead of upsert, because pgvector columns cannot be used in ON CONFLICT
// UPDATE expressions in all versions).
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

// Search performs a vector similarity search on the embeddings table using
// pgvector's cosine distance operator.
//
// How the SQL works:
//   - "embedding <=> $1" computes the cosine distance between the stored
//     embedding and the query vector. The <=> operator is provided by pgvector.
//     Cosine distance ranges from 0 (identical) to 2 (opposite).
//   - "1 - (embedding <=> $1)" converts cosine distance to cosine similarity
//     (1.0 = identical, 0.0 = unrelated). This is the "similarity" score
//     returned to the caller.
//   - The WHERE clause enforces visibility: an agent can only see its own
//     embeddings OR embeddings that are marked as shared.
//   - "source_type = ANY($3)" filters to only the requested content types
//     (e.g. "session_summary", "memory").
//   - "ORDER BY embedding <=> $1" sorts by distance ascending (most similar
//     first). The HNSW index on the embedding column accelerates this sort so
//     that PostgreSQL does not need to scan every row.
//   - LIMIT $5 caps results at topK.
func (r *EmbeddingRepo) Search(ctx context.Context, queryEmbedding []float32, agentID string, sourceTypes []string, topK int, minSimilarity float64) ([]EmbeddingResult, error) {
	// Query: find the topK most similar embeddings using pgvector cosine distance.
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
